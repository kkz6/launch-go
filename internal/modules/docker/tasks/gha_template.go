package tasks

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"

	"github.com/kkz6/launch-go/internal/modules/docker/templates"
)

// ApplicationWorkflowData is the typed input for the application
// workflow renderer. Every field is required — text/template's default
// "<no value>" substitution would leak into the output otherwise and
// produce a workflow that fails at run time on the customer's
// repo (worse: silently misroutes the webhook).
//
// Branch is the deploy branch the workflow triggers on (`push`).
// DockerfilePath is the relative path inside the repo; the workflow
// short-circuits to Nixpacks if the file doesn't exist at run time.
// LaunchBaseURL is the platform's externally-reachable URL (e.g.
// https://launchctl.io) — the workflow POSTs back to a path under
// this on success/failure.
// AppID is the docker_applications.id we're managing — webhook URLs
// embed it so the handler can scope by it without a JWT.
type ApplicationWorkflowData struct {
	Branch         string
	DockerfilePath string
	// BuildType is the chosen builder: "dockerfile" | "nixpacks" | "".
	// Empty means auto-detect (by Dockerfile presence) in the workflow;
	// a non-empty value is honoured verbatim so an explicit choice isn't
	// silently overridden (e.g. a "dockerfile" app fails loudly when the
	// Dockerfile is missing instead of quietly building with Nixpacks).
	BuildType     string
	LaunchBaseURL string
	AppID         string
	// ImagePackage is the GHCR package path (`<owner>/<app-slug>`) this
	// application publishes to — its own package, so apps from one repo are
	// isolated into separate packages (#100). Drives the image tag
	// (`ghcr.io/<ImagePackage>:launch-<sha>`) and the pull-token scope
	// (`repository:<ImagePackage>:pull`).
	ImagePackage string
	// DeployTokenSecret is the per-app GitHub repo-secret NAME holding the
	// deploy token (e.g. LAUNCH_DEPLOY_TOKEN_<APP_ID>). Namespaced per app so
	// multiple apps can share a repo without overwriting each other's token.
	DeployTokenSecret string
	// BuildSecretNames are the build-time secret identifiers the
	// workflow's docker/build-push-action should mount via its
	// `secrets:` input. Each entry maps to a repo secret named
	// LAUNCH_BUILD_<NAME> that the bootstrap job pushes alongside
	// the workflow commit. Empty/nil → no `secrets:` block rendered
	// (clean YAML for the common no-secrets case).
	BuildSecretNames []string
	// AutoDeploy adds an `on: push: [branch]` trigger so a push to the
	// deploy branch auto-deploys. Off → manual (workflow_dispatch) only.
	AutoDeploy bool
}

// ComposeWorkflowData is the matching input for the compose template.
// ComposeFilePath is where yq reads the compose document (default
// docker-compose.yml at the repo root). ComposeID is the
// docker_composes.id.
type ComposeWorkflowData struct {
	Branch          string
	ComposeFilePath string
	LaunchBaseURL   string
	ComposeID       string
	// ImagePackage is the GHCR package path (`<owner>/<stack-slug>`) this
	// compose stack publishes to — its own package (#100). Drives the
	// per-service image tag (`ghcr.io/<ImagePackage>:launch-<service>-<sha>`)
	// and the pull-token scope (`repository:<ImagePackage>:pull`).
	ImagePackage string
	// BuildSecretNames behave identically to ApplicationWorkflowData
	// — one set of names available to every service's build step in
	// the matrix.
	BuildSecretNames []string
	// AutoDeploy adds an `on: push: [branch]` trigger so a push to the
	// deploy branch auto-deploys. Off → manual (workflow_dispatch) only.
	AutoDeploy bool
}

// RenderApplicationWorkflow returns the YAML body Launch should commit
// to .github/workflows/launch-deploy.yml in the customer's repo for a
// docker_applications row using build_location = github_actions.
//
// Validates every input field up-front so a typo at the call site
// surfaces immediately rather than producing a workflow with
// "<no value>" placeholders that GitHub will reject mid-run.
//
// Output is normalised through TrimSpace + a single trailing newline so
// golden-file tests stay byte-stable across template edits that only
// add/remove blank lines.
func RenderApplicationWorkflow(data ApplicationWorkflowData) (string, error) {
	if err := validateApplicationData(data); err != nil {
		return "", err
	}
	return renderTemplate("gha_application.yml.tmpl", data)
}

// RenderComposeWorkflow is the compose-stack analogue of
// RenderApplicationWorkflow. The compose YAML has a matrix step that
// runs once per service that declares a `build:` directive.
func RenderComposeWorkflow(data ComposeWorkflowData) (string, error) {
	if err := validateComposeData(data); err != nil {
		return "", err
	}
	return renderTemplate("gha_compose.yml.tmpl", data)
}

func validateApplicationData(d ApplicationWorkflowData) error {
	missing := []string{}
	if d.Branch == "" {
		missing = append(missing, "Branch")
	}
	if d.DockerfilePath == "" {
		missing = append(missing, "DockerfilePath")
	}
	if d.LaunchBaseURL == "" {
		missing = append(missing, "LaunchBaseURL")
	}
	if d.AppID == "" {
		missing = append(missing, "AppID")
	}
	if d.ImagePackage == "" {
		missing = append(missing, "ImagePackage")
	}
	if d.DeployTokenSecret == "" {
		missing = append(missing, "DeployTokenSecret")
	}
	if len(missing) > 0 {
		return fmt.Errorf("RenderApplicationWorkflow: missing required field(s): %s", strings.Join(missing, ", "))
	}
	return nil
}

func validateComposeData(d ComposeWorkflowData) error {
	missing := []string{}
	if d.Branch == "" {
		missing = append(missing, "Branch")
	}
	if d.ComposeFilePath == "" {
		missing = append(missing, "ComposeFilePath")
	}
	if d.LaunchBaseURL == "" {
		missing = append(missing, "LaunchBaseURL")
	}
	if d.ComposeID == "" {
		missing = append(missing, "ComposeID")
	}
	if d.ImagePackage == "" {
		missing = append(missing, "ImagePackage")
	}
	if len(missing) > 0 {
		return fmt.Errorf("RenderComposeWorkflow: missing required field(s): %s", strings.Join(missing, ", "))
	}
	return nil
}

func renderTemplate(name string, data any) (string, error) {
	raw, err := templates.FS.ReadFile(name)
	if err != nil {
		return "", fmt.Errorf("read template %s: %w", name, err)
	}
	// Option "missingkey=error" gives us a hard failure if the data
	// struct doesn't supply a referenced field, rather than emitting
	// "<no value>" into the YAML and breaking at run time.
	tmpl, err := template.New(name).Option("missingkey=error").Parse(string(raw))
	if err != nil {
		return "", fmt.Errorf("parse template %s: %w", name, err)
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("execute template %s: %w", name, err)
	}
	return strings.TrimSpace(buf.String()) + "\n", nil
}

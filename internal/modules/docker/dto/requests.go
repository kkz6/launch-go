package dto

// CreateProjectRequest is the request body for creating a docker project.
type CreateProjectRequest struct {
	Name        string  `json:"name" validate:"required,min=1,max=255"`
	Description *string `json:"description,omitempty" validate:"omitempty,max=500"`
}

// UpdateProjectRequest is the partial-update body for a project.
// Both fields are optional; only present keys are applied.
type UpdateProjectRequest struct {
	Name        *string `json:"name,omitempty" validate:"omitempty,min=1,max=255"`
	Description *string `json:"description,omitempty" validate:"omitempty,max=500"`
}

// CreateApplicationRequest is the request body for registering an application
// inside a project. The source_type field is the discriminator that selects
// which of {ImageSource, GitSource, DockerfileSource} the rest of the payload
// will populate.
//
// We accept all three source-type carriers in one request struct (each
// nullable) rather than three separate endpoints — the validator below
// enforces "exactly one is present for the matching source_type".
type CreateApplicationRequest struct {
	Name string `json:"name" validate:"required,min=1,max=255"`

	SourceType string `json:"source_type" validate:"required,oneof=image git dockerfile"`

	// image source: one of these payloads must be set when source_type=image.
	Image *ImageSourceInput `json:"image,omitempty" validate:"omitempty,dive"`

	// git source.
	Git *GitSourceInput `json:"git,omitempty" validate:"omitempty,dive"`

	// dockerfile source (user-pasted Dockerfile content).
	Dockerfile *DockerfileSourceInput `json:"dockerfile,omitempty" validate:"omitempty,dive"`
}

// ImageSourceInput carries the info needed to pull and run a pre-built image.
type ImageSourceInput struct {
	// Image is the full reference including registry/repo/tag, e.g.
	// "nginx:1.27" or "ghcr.io/acme/api:v3". Tag is required — we don't
	// silently default to :latest because that hides upgrades from the user.
	Image string `json:"image" validate:"required,min=1,max=512"`
	// RegistryCredentialID optionally points at a private-registry credential
	// the user has connected. Left empty for public images.
	RegistryCredentialID *string `json:"registry_credential_id,omitempty"`
}

// GitSourceInput carries the info needed to clone + build from a git repo.
type GitSourceInput struct {
	// Repo is the clone URL — https://github.com/owner/repo or an ssh remote.
	Repo   string `json:"repo" validate:"required,min=1,max=512"`
	Branch string `json:"branch" validate:"required,min=1,max=255"`
	// SourceControlID, when set, picks the private-repo connection used to
	// authenticate the clone. Left empty for public repos.
	SourceControlID *string `json:"source_control_id,omitempty"`
	// BuildType is "nixpacks" | "dockerfile". If omitted the deploy job
	// auto-detects (Dockerfile in repo root → dockerfile, else nixpacks).
	BuildType *string `json:"build_type,omitempty" validate:"omitempty,oneof=nixpacks dockerfile"`
	// DockerfilePath, when BuildType=dockerfile, overrides the default
	// `./Dockerfile`. Useful for monorepos.
	DockerfilePath *string `json:"dockerfile_path,omitempty" validate:"omitempty,max=512"`
}

// DockerfileSourceInput carries a raw Dockerfile pasted into the UI.
type DockerfileSourceInput struct {
	// Contents is the full Dockerfile body. Capped at 64 KiB so a malicious
	// or accidental dump can't gum up the storage layer.
	Contents string `json:"contents" validate:"required,min=1,max=65536"`
}

// UpdateApplicationRequest is the partial-update body for an application.
// Only Name is mutable in phase 2a — source/build settings are immutable
// until later slices add a "reconfigure" flow.
type UpdateApplicationRequest struct {
	Name *string `json:"name,omitempty" validate:"omitempty,min=1,max=255"`
}

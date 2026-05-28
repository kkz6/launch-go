// Package templates owns the workflow YAML the GHA-build feature
// commits into customer repos. Files are embedded via go:embed so the
// binary carries them verbatim — no need to ship the repo's filesystem
// layout to production.
//
// Two templates today, one per workload kind:
//
//   - gha_application.yml.tmpl: single-image build for a docker
//     application. Detects Dockerfile vs. Nixpacks, builds, pushes
//     to GHCR, notifies Launch.
//   - gha_compose.yml.tmpl: matrix build over every service that
//     declares a `build:` directive in the customer's compose file.
//     Each service's image is pushed individually and the notify
//     payload carries a service_images map.
//
// Rendering goes through internal/modules/docker/tasks/gha_template.go,
// which exposes typed-input render helpers + golden-file tests so
// accidental template drift is caught in CI.
package templates

import "embed"

// FS is the embedded filesystem holding the workflow templates. Pulls
// in only the .tmpl files; tests, golden fixtures, and the README
// stay on disk.
//
//go:embed gha_*.tmpl
var FS embed.FS

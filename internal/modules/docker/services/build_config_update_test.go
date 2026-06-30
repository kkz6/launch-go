package services

import (
	"testing"

	"github.com/kkz6/launch-go/internal/modules/docker/dto"
	dockertypes "github.com/kkz6/launch-go/internal/modules/docker/types"
	"github.com/kkz6/launch-go/internal/pkg/dbtype"
)

func btptr(b dockertypes.BuildType) *dockertypes.BuildType { return &b }

// Switching to a dockerfile build records the new path and reports a change.
func TestResolveBuildConfigChange_SetsDockerfilePath(t *testing.T) {
	cur := btptr(dockertypes.BuildTypeNixpacks)
	newType, cfg, changed := resolveBuildConfigChange(cur, dbtype.JSONMap{}, &dto.UpdateApplicationRequest{
		BuildType:      strptr("dockerfile"),
		DockerfilePath: strptr("  docker/Dockerfile  "),
	})
	if !changed {
		t.Fatal("expected changed=true")
	}
	if newType != dockertypes.BuildTypeDockerfile {
		t.Errorf("build type = %q, want dockerfile", newType)
	}
	if got := cfg["dockerfile_path"]; got != "docker/Dockerfile" {
		t.Errorf("dockerfile_path = %v, want trimmed 'docker/Dockerfile'", got)
	}
}

// Switching to nixpacks drops a previously-set dockerfile path.
func TestResolveBuildConfigChange_NixpacksDropsPath(t *testing.T) {
	cur := btptr(dockertypes.BuildTypeDockerfile)
	_, cfg, changed := resolveBuildConfigChange(cur, dbtype.JSONMap{"dockerfile_path": "docker/Dockerfile"}, &dto.UpdateApplicationRequest{
		BuildType: strptr("nixpacks"),
	})
	if !changed {
		t.Fatal("expected changed=true")
	}
	if _, ok := cfg["dockerfile_path"]; ok {
		t.Errorf("dockerfile_path should be removed for nixpacks, got %v", cfg["dockerfile_path"])
	}
}

// Unrelated build_config keys (the Advanced runtime knobs live here too) must
// survive a dockerfile-path edit.
func TestResolveBuildConfigChange_PreservesOtherKeys(t *testing.T) {
	cur := btptr(dockertypes.BuildTypeDockerfile)
	_, cfg, _ := resolveBuildConfigChange(cur, dbtype.JSONMap{
		"dockerfile_path": "old/Dockerfile",
		"cpu_limit":       "0.5",
	}, &dto.UpdateApplicationRequest{
		DockerfilePath: strptr("new/Dockerfile"),
	})
	if cfg["cpu_limit"] != "0.5" {
		t.Errorf("cpu_limit should be preserved, got %v", cfg["cpu_limit"])
	}
	if cfg["dockerfile_path"] != "new/Dockerfile" {
		t.Errorf("dockerfile_path = %v, want new/Dockerfile", cfg["dockerfile_path"])
	}
}

// Re-submitting the same values is a no-op so we don't churn the workflow.
func TestResolveBuildConfigChange_NoChange(t *testing.T) {
	cur := btptr(dockertypes.BuildTypeDockerfile)
	_, _, changed := resolveBuildConfigChange(cur, dbtype.JSONMap{"dockerfile_path": "docker/Dockerfile"}, &dto.UpdateApplicationRequest{
		BuildType:      strptr("dockerfile"),
		DockerfilePath: strptr("docker/Dockerfile"),
	})
	if changed {
		t.Error("expected changed=false when nothing differs")
	}
}

// Editing only the path while build type is omitted keeps the current
// dockerfile build type.
func TestResolveBuildConfigChange_PathOnlyKeepsType(t *testing.T) {
	cur := btptr(dockertypes.BuildTypeDockerfile)
	newType, cfg, changed := resolveBuildConfigChange(cur, dbtype.JSONMap{"dockerfile_path": "a/Dockerfile"}, &dto.UpdateApplicationRequest{
		DockerfilePath: strptr("b/Dockerfile"),
	})
	if !changed || newType != dockertypes.BuildTypeDockerfile || cfg["dockerfile_path"] != "b/Dockerfile" {
		t.Errorf("got type=%q path=%v changed=%v", newType, cfg["dockerfile_path"], changed)
	}
}

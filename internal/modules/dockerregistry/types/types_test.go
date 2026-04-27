package types

import "testing"

func TestType_IsValid(t *testing.T) {
	for _, k := range []Type{TypeDockerHub, TypeGHCR, TypeGeneric} {
		if !k.IsValid() {
			t.Errorf("Type %q must be valid", k)
		}
	}
	if Type("ecr").IsValid() {
		t.Error("ecr should not be valid yet (v1 only ships docker_hub/ghcr/generic)")
	}
}

func TestType_Label(t *testing.T) {
	cases := map[Type]string{
		TypeDockerHub: "Docker Hub",
		TypeGHCR:      "GitHub Container Registry",
		TypeGeneric:   "Generic Registry",
	}
	for k, want := range cases {
		if got := k.Label(); got != want {
			t.Errorf("Label(%s) = %q, want %q", k, got, want)
		}
	}
}

func TestType_DefaultURL(t *testing.T) {
	cases := map[Type]string{
		TypeDockerHub: "https://index.docker.io/v1/",
		TypeGHCR:      "ghcr.io",
		TypeGeneric:   "",
	}
	for k, want := range cases {
		if got := k.DefaultURL(); got != want {
			t.Errorf("DefaultURL(%s) = %q, want %q", k, got, want)
		}
	}
}

func TestParseType(t *testing.T) {
	for _, in := range []string{"docker_hub", "ghcr", "generic"} {
		v, err := ParseType(in)
		if err != nil {
			t.Errorf("ParseType(%s) error: %v", in, err)
		}
		if string(v) != in {
			t.Errorf("ParseType(%s) = %s", in, v)
		}
	}
	if _, err := ParseType("ecr"); err == nil {
		t.Error("ParseType(ecr) must fail")
	}
}

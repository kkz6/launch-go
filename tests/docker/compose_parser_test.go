package docker_test

import (
	"testing"

	"github.com/kkz6/launch-go/internal/modules/docker/services"
	"github.com/kkz6/launch-go/internal/modules/docker/types"
)

func TestParseComposeFile_BasicService(t *testing.T) {
	content := `
version: "3"
services:
  web:
    image: nginx:latest
    ports:
      - "8080:80"
`

	parsed, err := services.ParseComposeFile(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(parsed) != 1 {
		t.Fatalf("expected 1 service, got %d", len(parsed))
	}

	svc := parsed[0]
	if svc.Name != "web" {
		t.Errorf("expected name 'web', got %q", svc.Name)
	}

	if svc.Image != "nginx:latest" {
		t.Errorf("expected image 'nginx:latest', got %q", svc.Image)
	}

	if svc.Kind != types.ServiceKindService {
		t.Errorf("expected kind %q, got %q", types.ServiceKindService, svc.Kind)
	}

	if len(svc.Ports) != 1 {
		t.Fatalf("expected 1 port, got %d", len(svc.Ports))
	}

	if svc.Ports[0].HostPort != 8080 || svc.Ports[0].ContainerPort != 80 {
		t.Errorf("expected port 8080:80, got %d:%d", svc.Ports[0].HostPort, svc.Ports[0].ContainerPort)
	}

	if svc.Ports[0].Protocol != types.ProtocolTCP {
		t.Errorf("expected protocol tcp, got %q", svc.Ports[0].Protocol)
	}
}

func TestParseComposeFile_EnvironmentMap(t *testing.T) {
	content := `
version: "3"
services:
  app:
    image: myapp:latest
    environment:
      DB_HOST: localhost
      DB_PORT: 5432
`

	parsed, err := services.ParseComposeFile(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	svc := parsed[0]
	if len(svc.EnvVars) != 2 {
		t.Fatalf("expected 2 env vars, got %d", len(svc.EnvVars))
	}

	envMap := make(map[string]string)
	for _, ev := range svc.EnvVars {
		envMap[ev.Key] = ev.Value
	}

	if envMap["DB_HOST"] != "localhost" {
		t.Errorf("expected DB_HOST=localhost, got %q", envMap["DB_HOST"])
	}

	if envMap["DB_PORT"] != "5432" {
		t.Errorf("expected DB_PORT=5432, got %q", envMap["DB_PORT"])
	}
}

func TestParseComposeFile_EnvironmentList(t *testing.T) {
	content := `
version: "3"
services:
  app:
    image: myapp:latest
    environment:
      - DB_HOST=localhost
      - DB_PORT=5432
`

	parsed, err := services.ParseComposeFile(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	svc := parsed[0]
	if len(svc.EnvVars) != 2 {
		t.Fatalf("expected 2 env vars, got %d", len(svc.EnvVars))
	}

	envMap := make(map[string]string)
	for _, ev := range svc.EnvVars {
		envMap[ev.Key] = ev.Value
	}

	if envMap["DB_HOST"] != "localhost" {
		t.Errorf("expected DB_HOST=localhost, got %q", envMap["DB_HOST"])
	}
}

func TestParseComposeFile_Volumes(t *testing.T) {
	content := `
version: "3"
services:
  db:
    image: postgres:16
    volumes:
      - pgdata:/var/lib/postgresql/data
      - ./init.sql:/docker-entrypoint-initdb.d/init.sql:ro
      - /host/path:/container/path
`

	parsed, err := services.ParseComposeFile(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	svc := parsed[0]
	if len(svc.Volumes) != 3 {
		t.Fatalf("expected 3 volumes, got %d", len(svc.Volumes))
	}

	// Named volume
	found := false
	for _, vol := range svc.Volumes {
		if vol.Source == "pgdata" {
			found = true
			if vol.MountType != types.MountTypeVolume {
				t.Errorf("expected named volume to be MountTypeVolume, got %q", vol.MountType)
			}
			if vol.Target != "/var/lib/postgresql/data" {
				t.Errorf("expected target '/var/lib/postgresql/data', got %q", vol.Target)
			}
		}
	}

	if !found {
		t.Error("expected to find named volume 'pgdata'")
	}

	// Bind mount (relative path)
	found = false
	for _, vol := range svc.Volumes {
		if vol.Source == "./init.sql" {
			found = true
			if vol.MountType != types.MountTypeBind {
				t.Errorf("expected bind mount for relative path, got %q", vol.MountType)
			}
			if !vol.ReadOnly {
				t.Error("expected read-only volume")
			}
		}
	}

	if !found {
		t.Error("expected to find bind mount './init.sql'")
	}

	// Bind mount (absolute path)
	found = false
	for _, vol := range svc.Volumes {
		if vol.Source == "/host/path" {
			found = true
			if vol.MountType != types.MountTypeBind {
				t.Errorf("expected bind mount for absolute path, got %q", vol.MountType)
			}
		}
	}

	if !found {
		t.Error("expected to find bind mount '/host/path'")
	}
}

func TestParseComposeFile_RestartPolicies(t *testing.T) {
	tests := []struct {
		restart  string
		expected types.RestartPolicy
	}{
		{"no", types.RestartPolicyNo},
		{"always", types.RestartPolicyAlways},
		{"unless-stopped", types.RestartPolicyUnlessStopped},
		{"on-failure", types.RestartPolicyOnFailure},
	}

	for _, tc := range tests {
		t.Run(tc.restart, func(t *testing.T) {
			content := `
version: "3"
services:
  app:
    image: myapp:latest
    restart: ` + tc.restart + `
`
			parsed, err := services.ParseComposeFile(content)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if parsed[0].RestartPolicy != tc.expected {
				t.Errorf("expected restart policy %q, got %q", tc.expected, parsed[0].RestartPolicy)
			}
		})
	}
}

func TestParseComposeFile_PortProtocols(t *testing.T) {
	content := `
version: "3"
services:
  app:
    image: myapp:latest
    ports:
      - "8080:80/tcp"
      - "5353:53/udp"
`

	parsed, err := services.ParseComposeFile(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	svc := parsed[0]
	if len(svc.Ports) != 2 {
		t.Fatalf("expected 2 ports, got %d", len(svc.Ports))
	}

	portMap := make(map[int]services.ParsedPort)
	for _, p := range svc.Ports {
		portMap[p.HostPort] = p
	}

	if p, ok := portMap[8080]; ok {
		if p.Protocol != types.ProtocolTCP {
			t.Errorf("expected TCP for port 8080, got %q", p.Protocol)
		}
	} else {
		t.Error("expected port 8080 mapping")
	}

	if p, ok := portMap[5353]; ok {
		if p.Protocol != types.ProtocolUDP {
			t.Errorf("expected UDP for port 5353, got %q", p.Protocol)
		}
	} else {
		t.Error("expected port 5353 mapping")
	}
}

func TestParseComposeFile_MultipleServices(t *testing.T) {
	content := `
version: "3"
services:
  web:
    image: nginx:latest
  api:
    image: myapi:latest
  db:
    image: postgres:16
`

	parsed, err := services.ParseComposeFile(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(parsed) != 3 {
		t.Fatalf("expected 3 services, got %d", len(parsed))
	}

	names := make(map[string]bool)
	for _, svc := range parsed {
		names[svc.Name] = true
	}

	for _, name := range []string{"web", "api", "db"} {
		if !names[name] {
			t.Errorf("expected service %q to be parsed", name)
		}
	}
}

func TestParseComposeFile_CommandAndEntrypoint(t *testing.T) {
	content := `
version: "3"
services:
  app:
    image: myapp:latest
    command: "serve --port 8080"
    entrypoint: "/entrypoint.sh"
`

	parsed, err := services.ParseComposeFile(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	svc := parsed[0]
	if svc.Command == nil || *svc.Command != "serve --port 8080" {
		t.Errorf("expected command 'serve --port 8080', got %v", svc.Command)
	}

	if svc.Entrypoint == nil || *svc.Entrypoint != "/entrypoint.sh" {
		t.Errorf("expected entrypoint '/entrypoint.sh', got %v", svc.Entrypoint)
	}
}

func TestParseComposeFile_NoServices(t *testing.T) {
	content := `version: "3"`

	_, err := services.ParseComposeFile(content)
	if err == nil {
		t.Fatal("expected error for empty services")
	}
}

func TestParseComposeFile_NoImage(t *testing.T) {
	content := `
version: "3"
services:
  app:
    build: .
`

	_, err := services.ParseComposeFile(content)
	if err == nil {
		t.Fatal("expected error for service without image")
	}
}

func TestParseComposeFile_InvalidYAML(t *testing.T) {
	content := `not: valid: yaml: [[[`

	_, err := services.ParseComposeFile(content)
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}

func TestParseComposeFile_DefaultRestartPolicy(t *testing.T) {
	content := `
version: "3"
services:
  app:
    image: myapp:latest
`

	parsed, err := services.ParseComposeFile(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if parsed[0].RestartPolicy != types.RestartPolicyUnlessStopped {
		t.Errorf("expected default restart policy %q, got %q",
			types.RestartPolicyUnlessStopped, parsed[0].RestartPolicy)
	}
}

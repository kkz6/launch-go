package services

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/kkz6/launch-go/internal/modules/docker/types"
	"gopkg.in/yaml.v3"
)

// ComposeFile represents a docker-compose.yml structure
type ComposeFile struct {
	Version  string                    `yaml:"version"`
	Services map[string]ComposeService `yaml:"services"`
}

// ComposeService represents a service in docker-compose.yml
type ComposeService struct {
	Image       string            `yaml:"image"`
	Command     string            `yaml:"command"`
	Entrypoint  string            `yaml:"entrypoint"`
	Environment interface{}       `yaml:"environment"`
	Ports       []string          `yaml:"ports"`
	Volumes     []string          `yaml:"volumes"`
	Restart     string            `yaml:"restart"`
	Labels      map[string]string `yaml:"labels"`
	CPULimit    string            `yaml:"cpus"`
	MemoryLimit string            `yaml:"mem_limit"`
}

// ParsedService is the result of parsing a compose service
type ParsedService struct {
	Name          string
	Image         string
	Command       *string
	Entrypoint    *string
	Kind          types.ServiceKind
	RestartPolicy types.RestartPolicy
	EnvVars       []ParsedEnvVar
	Ports         []ParsedPort
	Volumes       []ParsedVolume
	CPULimit      *float64
	MemoryLimit   *int
}

// ParsedEnvVar represents an environment variable parsed from a compose file
type ParsedEnvVar struct {
	Key      string
	Value    string
	IsSecret bool
}

// ParsedPort represents a port mapping parsed from a compose file
type ParsedPort struct {
	HostPort      int
	ContainerPort int
	Protocol      types.Protocol
}

// ParsedVolume represents a volume mount parsed from a compose file
type ParsedVolume struct {
	MountType types.MountType
	Source    string
	Target    string
	ReadOnly  bool
}

// ParseComposeFile parses a docker-compose.yml content string into structured services
func ParseComposeFile(content string) ([]ParsedService, error) {
	var compose ComposeFile
	if err := yaml.Unmarshal([]byte(content), &compose); err != nil {
		return nil, fmt.Errorf("invalid compose file: %w", err)
	}

	if len(compose.Services) == 0 {
		return nil, fmt.Errorf("no services found in compose file")
	}

	var services []ParsedService

	for name, svc := range compose.Services {
		parsed, err := parseComposeService(name, svc)
		if err != nil {
			return nil, fmt.Errorf("error parsing service %s: %w", name, err)
		}

		services = append(services, *parsed)
	}

	return services, nil
}

func parseComposeService(name string, svc ComposeService) (*ParsedService, error) {
	if svc.Image == "" {
		return nil, fmt.Errorf("service %s has no image specified (build is not supported)", name)
	}

	parsed := &ParsedService{
		Name:          name,
		Image:         svc.Image,
		Kind:          types.ServiceKindService,
		RestartPolicy: types.RestartPolicyUnlessStopped,
	}

	if svc.Command != "" {
		cmd := svc.Command
		parsed.Command = &cmd
	}

	if svc.Entrypoint != "" {
		ep := svc.Entrypoint
		parsed.Entrypoint = &ep
	}

	// Parse restart policy
	switch svc.Restart {
	case "no":
		parsed.RestartPolicy = types.RestartPolicyNo
	case "always":
		parsed.RestartPolicy = types.RestartPolicyAlways
	case "unless-stopped":
		parsed.RestartPolicy = types.RestartPolicyUnlessStopped
	case "on-failure":
		parsed.RestartPolicy = types.RestartPolicyOnFailure
	}

	// Parse environment variables
	parsed.EnvVars = parseEnvironment(svc.Environment)

	// Parse ports
	for _, portStr := range svc.Ports {
		port, err := parsePort(portStr)
		if err != nil {
			continue
		}

		parsed.Ports = append(parsed.Ports, *port)
	}

	// Parse volumes
	for _, volStr := range svc.Volumes {
		vol := parseVolume(volStr)
		parsed.Volumes = append(parsed.Volumes, vol)
	}

	return parsed, nil
}

func parseEnvironment(env interface{}) []ParsedEnvVar {
	var vars []ParsedEnvVar

	switch e := env.(type) {
	case map[string]interface{}:
		for key, val := range e {
			vars = append(vars, ParsedEnvVar{
				Key:   key,
				Value: fmt.Sprintf("%v", val),
			})
		}
	case []interface{}:
		for _, item := range e {
			str, ok := item.(string)
			if !ok {
				continue
			}

			parts := strings.SplitN(str, "=", 2)
			if len(parts) == 2 {
				vars = append(vars, ParsedEnvVar{
					Key:   parts[0],
					Value: parts[1],
				})
			}
		}
	}

	return vars
}

func parsePort(portStr string) (*ParsedPort, error) {
	protocol := types.ProtocolTCP

	// Handle protocol suffix
	if strings.HasSuffix(portStr, "/udp") {
		protocol = types.ProtocolUDP
		portStr = strings.TrimSuffix(portStr, "/udp")
	} else {
		portStr = strings.TrimSuffix(portStr, "/tcp")
	}

	parts := strings.Split(portStr, ":")
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid port format: %s", portStr)
	}

	hostPort, err := strconv.Atoi(parts[0])
	if err != nil {
		return nil, fmt.Errorf("invalid host port: %s", parts[0])
	}

	containerPort, err := strconv.Atoi(parts[1])
	if err != nil {
		return nil, fmt.Errorf("invalid container port: %s", parts[1])
	}

	return &ParsedPort{
		HostPort:      hostPort,
		ContainerPort: containerPort,
		Protocol:      protocol,
	}, nil
}

func parseVolume(volStr string) ParsedVolume {
	readOnly := false

	if strings.HasSuffix(volStr, ":ro") {
		readOnly = true
		volStr = strings.TrimSuffix(volStr, ":ro")
	}

	parts := strings.SplitN(volStr, ":", 2)
	if len(parts) != 2 {
		return ParsedVolume{
			MountType: types.MountTypeVolume,
			Source:    volStr,
			Target:    volStr,
		}
	}

	mountType := types.MountTypeVolume
	if strings.HasPrefix(parts[0], "/") || strings.HasPrefix(parts[0], "./") {
		mountType = types.MountTypeBind
	}

	return ParsedVolume{
		MountType: mountType,
		Source:    parts[0],
		Target:    parts[1],
		ReadOnly:  readOnly,
	}
}

package types

import (
	"database/sql/driver"

	"github.com/kkz6/launch-go/internal/pkg/enumtypes"
)

// =============================================================================
// ServiceKind
// =============================================================================

// ServiceKind determines if a docker service gets Traefik routing
type ServiceKind string

const (
	ServiceKindApplication ServiceKind = "application"
	ServiceKindService     ServiceKind = "service"
)

func (k ServiceKind) String() string { return string(k) }

func (k ServiceKind) IsValid() bool {
	switch k {
	case ServiceKindApplication, ServiceKindService:
		return true
	}

	return false
}

func (k ServiceKind) Label() string {
	labels := map[ServiceKind]string{
		ServiceKindApplication: "Application",
		ServiceKindService:     "Service",
	}
	if l, ok := labels[k]; ok {
		return l
	}

	return "Unknown"
}

func (k *ServiceKind) Scan(value interface{}) error { return enumtypes.ScanString(k, value) }
func (k ServiceKind) Value() (driver.Value, error)  { return enumtypes.ValueString(k) }

// =============================================================================
// ServiceStatus
// =============================================================================

// ServiceStatus represents the current state of a docker service
type ServiceStatus string

const (
	ServiceStatusPending   ServiceStatus = "pending"
	ServiceStatusDeploying ServiceStatus = "deploying"
	ServiceStatusRunning   ServiceStatus = "running"
	ServiceStatusStopped   ServiceStatus = "stopped"
	ServiceStatusFailed    ServiceStatus = "failed"
)

func (s ServiceStatus) String() string { return string(s) }

func (s ServiceStatus) IsValid() bool {
	switch s {
	case ServiceStatusPending, ServiceStatusDeploying, ServiceStatusRunning,
		ServiceStatusStopped, ServiceStatusFailed:
		return true
	}

	return false
}

func (s ServiceStatus) Label() string {
	labels := map[ServiceStatus]string{
		ServiceStatusPending:   "Pending",
		ServiceStatusDeploying: "Deploying",
		ServiceStatusRunning:   "Running",
		ServiceStatusStopped:   "Stopped",
		ServiceStatusFailed:    "Failed",
	}
	if l, ok := labels[s]; ok {
		return l
	}

	return "Unknown"
}

func (s *ServiceStatus) Scan(value interface{}) error { return enumtypes.ScanString(s, value) }
func (s ServiceStatus) Value() (driver.Value, error)  { return enumtypes.ValueString(s) }

// =============================================================================
// RestartPolicy
// =============================================================================

// RestartPolicy for Docker containers
type RestartPolicy string

const (
	RestartPolicyNo            RestartPolicy = "no"
	RestartPolicyAlways        RestartPolicy = "always"
	RestartPolicyUnlessStopped RestartPolicy = "unless-stopped"
	RestartPolicyOnFailure     RestartPolicy = "on-failure"
)

func (r RestartPolicy) String() string { return string(r) }

func (r RestartPolicy) IsValid() bool {
	switch r {
	case RestartPolicyNo, RestartPolicyAlways, RestartPolicyUnlessStopped, RestartPolicyOnFailure:
		return true
	}

	return false
}

func (r RestartPolicy) Label() string {
	labels := map[RestartPolicy]string{
		RestartPolicyNo:            "No",
		RestartPolicyAlways:        "Always",
		RestartPolicyUnlessStopped: "Unless Stopped",
		RestartPolicyOnFailure:     "On Failure",
	}
	if l, ok := labels[r]; ok {
		return l
	}

	return "Unknown"
}

func (r *RestartPolicy) Scan(value interface{}) error { return enumtypes.ScanString(r, value) }
func (r RestartPolicy) Value() (driver.Value, error)  { return enumtypes.ValueString(r) }

// =============================================================================
// DeploymentStatus
// =============================================================================

// DeploymentStatus for docker deployments
type DeploymentStatus string

const (
	DeploymentStatusPending  DeploymentStatus = "pending"
	DeploymentStatusRunning  DeploymentStatus = "running"
	DeploymentStatusFinished DeploymentStatus = "finished"
	DeploymentStatusFailed   DeploymentStatus = "failed"
)

func (d DeploymentStatus) String() string { return string(d) }

func (d DeploymentStatus) IsValid() bool {
	switch d {
	case DeploymentStatusPending, DeploymentStatusRunning,
		DeploymentStatusFinished, DeploymentStatusFailed:
		return true
	}

	return false
}

func (d DeploymentStatus) Label() string {
	labels := map[DeploymentStatus]string{
		DeploymentStatusPending:  "Pending",
		DeploymentStatusRunning:  "Running",
		DeploymentStatusFinished: "Finished",
		DeploymentStatusFailed:   "Failed",
	}
	if l, ok := labels[d]; ok {
		return l
	}

	return "Unknown"
}

func (d *DeploymentStatus) Scan(value interface{}) error { return enumtypes.ScanString(d, value) }
func (d DeploymentStatus) Value() (driver.Value, error)  { return enumtypes.ValueString(d) }

// =============================================================================
// DeploymentTrigger
// =============================================================================

// DeploymentTrigger describes what triggered a deployment
type DeploymentTrigger string

const (
	DeploymentTriggerManual        DeploymentTrigger = "manual"
	DeploymentTriggerWebhook       DeploymentTrigger = "webhook"
	DeploymentTriggerComposeUpdate DeploymentTrigger = "compose_update"
)

func (t DeploymentTrigger) String() string { return string(t) }

func (t DeploymentTrigger) IsValid() bool {
	switch t {
	case DeploymentTriggerManual, DeploymentTriggerWebhook, DeploymentTriggerComposeUpdate:
		return true
	}

	return false
}

func (t DeploymentTrigger) Label() string {
	labels := map[DeploymentTrigger]string{
		DeploymentTriggerManual:        "Manual",
		DeploymentTriggerWebhook:       "Webhook",
		DeploymentTriggerComposeUpdate: "Compose Update",
	}
	if l, ok := labels[t]; ok {
		return l
	}

	return "Unknown"
}

func (t *DeploymentTrigger) Scan(value interface{}) error { return enumtypes.ScanString(t, value) }
func (t DeploymentTrigger) Value() (driver.Value, error)  { return enumtypes.ValueString(t) }

// =============================================================================
// MountType
// =============================================================================

// MountType for docker volumes
type MountType string

const (
	MountTypeVolume MountType = "volume"
	MountTypeBind   MountType = "bind"
)

func (m MountType) String() string { return string(m) }

func (m MountType) IsValid() bool {
	switch m {
	case MountTypeVolume, MountTypeBind:
		return true
	}

	return false
}

func (m MountType) Label() string {
	labels := map[MountType]string{
		MountTypeVolume: "Volume",
		MountTypeBind:   "Bind",
	}
	if l, ok := labels[m]; ok {
		return l
	}

	return "Unknown"
}

func (m *MountType) Scan(value interface{}) error { return enumtypes.ScanString(m, value) }
func (m MountType) Value() (driver.Value, error)  { return enumtypes.ValueString(m) }

// =============================================================================
// CertificateType
// =============================================================================

// CertificateType for SSL
type CertificateType string

const (
	CertificateTypeLetsEncrypt CertificateType = "letsencrypt"
	CertificateTypeCustom      CertificateType = "custom"
	CertificateTypeNone        CertificateType = "none"
)

func (c CertificateType) String() string { return string(c) }

func (c CertificateType) IsValid() bool {
	switch c {
	case CertificateTypeLetsEncrypt, CertificateTypeCustom, CertificateTypeNone:
		return true
	}

	return false
}

func (c CertificateType) Label() string {
	labels := map[CertificateType]string{
		CertificateTypeLetsEncrypt: "Let's Encrypt",
		CertificateTypeCustom:      "Custom",
		CertificateTypeNone:        "None",
	}
	if l, ok := labels[c]; ok {
		return l
	}

	return "Unknown"
}

func (c *CertificateType) Scan(value interface{}) error { return enumtypes.ScanString(c, value) }
func (c CertificateType) Value() (driver.Value, error)  { return enumtypes.ValueString(c) }

// =============================================================================
// Protocol
// =============================================================================

// Protocol for port mappings
type Protocol string

const (
	ProtocolTCP Protocol = "tcp"
	ProtocolUDP Protocol = "udp"
)

func (p Protocol) String() string { return string(p) }

func (p Protocol) IsValid() bool {
	switch p {
	case ProtocolTCP, ProtocolUDP:
		return true
	}

	return false
}

func (p Protocol) Label() string {
	labels := map[Protocol]string{
		ProtocolTCP: "TCP",
		ProtocolUDP: "UDP",
	}
	if l, ok := labels[p]; ok {
		return l
	}

	return "Unknown"
}

func (p *Protocol) Scan(value interface{}) error { return enumtypes.ScanString(p, value) }
func (p Protocol) Value() (driver.Value, error)  { return enumtypes.ValueString(p) }

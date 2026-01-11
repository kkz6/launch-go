package enums

import (
	"database/sql/driver"
	"fmt"
)

// ServerType represents the type of server
type ServerType string

const (
	ServerTypePhp      ServerType = "php"
	ServerTypeDatabase ServerType = "database"
)

func (t ServerType) String() string {
	return string(t)
}

func (t ServerType) Label() string {
	labels := map[ServerType]string{
		ServerTypePhp:      "PHP Application Server",
		ServerTypeDatabase: "Database Server",
	}
	if label, ok := labels[t]; ok {
		return label
	}

	return "Unknown"
}

func (t ServerType) IsValid() bool {
	switch t {
	case ServerTypePhp, ServerTypeDatabase:
		return true
	}

	return false
}

func (t ServerType) GetFeatures() []ServerFeature {
	switch t {
	case ServerTypePhp:
		return []ServerFeature{
			ServerFeatureSites,
			ServerFeaturePhpManagement,
			ServerFeatureComposer,
			ServerFeatureDatabaseManagement,
			ServerFeatureQueueWorkers,
			ServerFeatureDaemons,
			ServerFeatureScheduler,
			ServerFeatureSslCertificates,
			ServerFeatureRedis,
			ServerFeatureBackups,
			ServerFeatureServices,
		}
	case ServerTypeDatabase:
		return []ServerFeature{
			ServerFeatureDatabaseManagement,
			ServerFeatureBackups,
		}
	}

	return nil
}

func (t ServerType) HasFeature(feature ServerFeature) bool {
	features := t.GetFeatures()
	for _, f := range features {
		if f == feature {
			return true
		}
	}

	return false
}

func (t ServerType) GetProcessManager() ProcessManager {
	switch t {
	case ServerTypePhp:
		return ProcessManagerSupervisor
	case ServerTypeDatabase:
		return ProcessManagerNone
	}

	return ProcessManagerNone
}

func (t *ServerType) Scan(value interface{}) error {
	if value == nil {
		*t = ServerTypePhp
		return nil
	}

	switch v := value.(type) {
	case []byte:
		*t = ServerType(v)
	case string:
		*t = ServerType(v)
	default:
		return fmt.Errorf("cannot scan type %T into ServerType", value)
	}

	return nil
}

func (t ServerType) Value() (driver.Value, error) {
	return string(t), nil
}

func ParseServerType(s string) (ServerType, error) {
	t := ServerType(s)
	if !t.IsValid() {
		return ServerTypePhp, fmt.Errorf("invalid server type: %s", s)
	}

	return t, nil
}

func AllServerTypes() []ServerType {
	return []ServerType{ServerTypePhp, ServerTypeDatabase}
}

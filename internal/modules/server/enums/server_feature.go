package enums

// ServerFeature represents available server features
type ServerFeature string

const (
	ServerFeatureSites              ServerFeature = "sites"
	ServerFeatureSslCertificates    ServerFeature = "ssl_certificates"
	ServerFeaturePhpManagement      ServerFeature = "php_management"
	ServerFeatureComposer           ServerFeature = "composer"
	ServerFeatureDatabaseManagement ServerFeature = "database_management"
	ServerFeatureQueueWorkers       ServerFeature = "queue_workers"
	ServerFeatureDaemons            ServerFeature = "daemons"
	ServerFeatureScheduler          ServerFeature = "scheduler"
	ServerFeatureRedis              ServerFeature = "redis"
	ServerFeatureBackups            ServerFeature = "backups"
	ServerFeatureServices           ServerFeature = "services"
)

func (f ServerFeature) String() string {
	return string(f)
}

func (f ServerFeature) Label() string {
	labels := map[ServerFeature]string{
		ServerFeatureSites:              "Sites",
		ServerFeatureSslCertificates:    "SSL Certificates",
		ServerFeaturePhpManagement:      "PHP Management",
		ServerFeatureComposer:           "Composer",
		ServerFeatureDatabaseManagement: "Database Management",
		ServerFeatureQueueWorkers:       "Queue Workers",
		ServerFeatureDaemons:            "Daemons",
		ServerFeatureScheduler:          "Scheduler",
		ServerFeatureRedis:              "Redis",
		ServerFeatureBackups:            "Backups",
		ServerFeatureServices:           "Services",
	}
	if label, ok := labels[f]; ok {
		return label
	}

	return "Unknown"
}

func (f ServerFeature) IsValid() bool {
	switch f {
	case ServerFeatureSites, ServerFeatureSslCertificates, ServerFeaturePhpManagement,
		ServerFeatureComposer, ServerFeatureDatabaseManagement, ServerFeatureQueueWorkers,
		ServerFeatureDaemons, ServerFeatureScheduler, ServerFeatureRedis, ServerFeatureBackups,
		ServerFeatureServices:
		return true
	}

	return false
}

func (f ServerFeature) NavigationKey() string {
	keys := map[ServerFeature]string{
		ServerFeatureSites:              "sites",
		ServerFeatureDatabaseManagement: "databases",
		ServerFeaturePhpManagement:      "php",
		ServerFeatureQueueWorkers:       "queues",
		ServerFeatureDaemons:            "daemons",
		ServerFeatureScheduler:          "scheduler",
		ServerFeatureSslCertificates:    "ssl",
		ServerFeatureBackups:            "backups",
		ServerFeatureServices:           "advanced",
	}
	if key, ok := keys[f]; ok {
		return key
	}

	return ""
}

func AllServerFeatures() []ServerFeature {
	return []ServerFeature{
		ServerFeatureSites, ServerFeatureSslCertificates, ServerFeaturePhpManagement,
		ServerFeatureComposer, ServerFeatureDatabaseManagement, ServerFeatureQueueWorkers,
		ServerFeatureDaemons, ServerFeatureScheduler, ServerFeatureRedis, ServerFeatureBackups,
		ServerFeatureServices,
	}
}

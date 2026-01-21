package channels

// CommonNotificationRules are shared validation rules across all notification channels
var CommonNotificationRules = map[string]string{
	"appDeploy":      "boolean",
	"databaseBackup": "boolean",
}

// MergeValidationRules combines common and channel-specific validation rules
func MergeValidationRules(channelRules map[string]string) map[string]string {
	result := make(map[string]string, len(CommonNotificationRules)+len(channelRules))

	for k, v := range CommonNotificationRules {
		result[k] = v
	}

	for k, v := range channelRules {
		result[k] = v
	}

	return result
}

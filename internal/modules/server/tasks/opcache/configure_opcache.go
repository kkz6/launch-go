package opcache

import (
	"context"
	"time"

	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/modules/server/tasks"
	"github.com/kkz6/launch-go/internal/taskrunner"
)

// OpcacheSettings represents the OPcache configuration settings
type OpcacheSettings struct {
	Enable                  bool `json:"enable"`
	Memory                  int  `json:"memory"`
	MaxAcceleratedFiles     int  `json:"max_accelerated_files"`
	RevalidateFreq          int  `json:"revalidate_freq"`
	ValidateTimestamps      bool `json:"validate_timestamps"`
	SaveComments            bool `json:"save_comments"`
	EnableFileOverride      bool `json:"enable_file_override"`
	MaxWastedPercentage     int  `json:"max_wasted_percentage"`
	InternerStringsBuffer   int  `json:"interned_strings_buffer"`
	FastShutdown            bool `json:"fast_shutdown"`
	EnableCli               bool `json:"enable_cli"`
	JitBuffer               int  `json:"jit_buffer"`
	JitMode                 int  `json:"jit_mode"`
}

// ConfigureOpcache configures OPcache settings on a server
type ConfigureOpcache struct {
	tasks.BaseServerTask
	service  *models.InstalledService
	settings OpcacheSettings
}

// NewConfigureOpcache creates a new ConfigureOpcache task
func NewConfigureOpcache(server *models.Server, service *models.InstalledService, settings OpcacheSettings) *ConfigureOpcache {
	task := &ConfigureOpcache{
		BaseServerTask: tasks.BaseServerTask{
			BaseTask: taskrunner.BaseTask{
				TemplateName: "server/opcache/configure",
				TaskTimeout:  60 * time.Second,
			},
		},
		service:  service,
		settings: settings,
	}

	task.SetServer(server)

	task.FinishedCallback = task.onFinished
	task.FailedCallback = task.onFailed
	task.TimeoutCallback = task.onTimeout

	return task
}

// Data returns the template data
func (t *ConfigureOpcache) Data() map[string]interface{} {
	return map[string]interface{}{
		"Server":   t.Server(),
		"Service":  t.service,
		"Settings": t.settings,
	}
}

// Service returns the installed service being configured
func (t *ConfigureOpcache) Service() *models.InstalledService {
	return t.service
}

// Settings returns the OPcache settings
func (t *ConfigureOpcache) Settings() OpcacheSettings {
	return t.settings
}

// onFinished handles successful completion
func (t *ConfigureOpcache) onFinished(ctx context.Context, result *taskrunner.TaskResult) {
	// Update service type_data with opcache settings and status = "configured"
	// This would be handled by the job/repository layer
	typeData := t.service.GetTypeData()
	if typeData == nil {
		typeData = make(map[string]interface{})
	}

	typeData["opcache"] = map[string]interface{}{
		"enable":                   t.settings.Enable,
		"memory":                   t.settings.Memory,
		"max_accelerated_files":    t.settings.MaxAcceleratedFiles,
		"revalidate_freq":          t.settings.RevalidateFreq,
		"validate_timestamps":      t.settings.ValidateTimestamps,
		"save_comments":            t.settings.SaveComments,
		"enable_file_override":     t.settings.EnableFileOverride,
		"max_wasted_percentage":    t.settings.MaxWastedPercentage,
		"interned_strings_buffer":  t.settings.InternerStringsBuffer,
		"fast_shutdown":            t.settings.FastShutdown,
		"enable_cli":               t.settings.EnableCli,
		"jit_buffer":               t.settings.JitBuffer,
		"jit_mode":                 t.settings.JitMode,
		"status":                   "configured",
	}

	_ = t.service.SetTypeData(typeData)
}

// onFailed handles task failure
func (t *ConfigureOpcache) onFailed(ctx context.Context, result *taskrunner.TaskResult) {
	t.updateStatus("failed")
}

// onTimeout handles task timeout
func (t *ConfigureOpcache) onTimeout(ctx context.Context, result *taskrunner.TaskResult) {
	t.updateStatus("failed")
}

// updateStatus updates the opcache status in service type_data
func (t *ConfigureOpcache) updateStatus(status string) {
	typeData := t.service.GetTypeData()
	if typeData == nil {
		typeData = make(map[string]interface{})
	}

	opcache, ok := typeData["opcache"].(map[string]interface{})
	if !ok {
		opcache = make(map[string]interface{})
	}

	opcache["status"] = status
	typeData["opcache"] = opcache

	_ = t.service.SetTypeData(typeData)
}

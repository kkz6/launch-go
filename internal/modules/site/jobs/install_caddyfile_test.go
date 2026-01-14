package jobs

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kkz6/launch-go/internal/modules/site/enums"
	"github.com/kkz6/launch-go/internal/modules/site/models"
)

func TestNewInstallCaddyfileTask(t *testing.T) {
	siteID := "site123"

	task, err := NewInstallCaddyfileTask(siteID, nil)

	require.NoError(t, err)
	assert.NotNil(t, task)
	assert.Equal(t, TypeInstallCaddyfile, task.Type())
}

func TestNewInstallCaddyfileTask_WithUserID(t *testing.T) {
	siteID := "site123"
	userID := "user456"

	task, err := NewInstallCaddyfileTask(siteID, &userID)

	require.NoError(t, err)
	assert.NotNil(t, task)
}

func TestNewUpdateCaddyfileTask(t *testing.T) {
	siteID := "site123"

	task, err := NewUpdateCaddyfileTask(siteID, nil)

	require.NoError(t, err)
	assert.NotNil(t, task)
	assert.Equal(t, TypeUpdateCaddyfile, task.Type())
}

func TestNewUninstallCaddyfileTask(t *testing.T) {
	siteID := "site123"

	task, err := NewUninstallCaddyfileTask(siteID, nil)

	require.NoError(t, err)
	assert.NotNil(t, task)
	assert.Equal(t, TypeUninstallCaddyfile, task.Type())
}

func TestCaddyfilePayload(t *testing.T) {
	userID := "user123"
	payload := CaddyfilePayload{
		SiteID: "site456",
		UserID: &userID,
	}

	assert.Equal(t, "site456", payload.SiteID)
	assert.NotNil(t, payload.UserID)
	assert.Equal(t, "user123", *payload.UserID)
}

func TestInstallCaddyfileJob_Type(t *testing.T) {
	job := &InstallCaddyfileJob{}
	assert.Equal(t, TypeInstallCaddyfile, job.Type())
}

func TestUpdateCaddyfileJob_Type(t *testing.T) {
	job := &UpdateCaddyfileJob{}
	assert.Equal(t, TypeUpdateCaddyfile, job.Type())
}

func TestUninstallCaddyfileJob_Type(t *testing.T) {
	job := &UninstallCaddyfileJob{}
	assert.Equal(t, TypeUninstallCaddyfile, job.Type())
}

func TestInstallCaddyfileJob_GenerateCaddyfileContent_BasicSite(t *testing.T) {
	site := &models.Site{
		Address:                "example.com",
		Path:                   "/home/user/example.com",
		WebFolder:              "public",
		TlsSetting:             enums.TlsSettingAuto,
		ZeroDowntimeDeployment: true,
	}

	job := &InstallCaddyfileJob{}
	content := job.generateCaddyfileContent(site)

	assert.Contains(t, content, "example.com")
	assert.Contains(t, content, "root *")
	assert.Contains(t, content, "encode gzip")
	assert.Contains(t, content, "file_server")
	assert.Contains(t, content, "log {")
	assert.Contains(t, content, "access.log")
}

func TestInstallCaddyfileJob_GenerateCaddyfileContent_WithPhp(t *testing.T) {
	phpVersion := "8.2"
	site := &models.Site{
		Address:                "laravel.com",
		Path:                   "/home/user/laravel.com",
		WebFolder:              "public",
		TlsSetting:             enums.TlsSettingAuto,
		ZeroDowntimeDeployment: true,
		PhpVersion:             &phpVersion,
	}

	job := &InstallCaddyfileJob{}
	content := job.generateCaddyfileContent(site)

	assert.Contains(t, content, "php_fastcgi")
	assert.Contains(t, content, "php8.2-fpm.sock")
}

func TestInstallCaddyfileJob_GenerateCaddyfileContent_WithAliases(t *testing.T) {
	site := &models.Site{
		Address:                "primary.com",
		Aliases:                []string{"www.primary.com", "alias.primary.com"},
		Path:                   "/home/user/primary.com",
		WebFolder:              "public",
		TlsSetting:             enums.TlsSettingAuto,
		ZeroDowntimeDeployment: true,
	}

	job := &InstallCaddyfileJob{}
	content := job.generateCaddyfileContent(site)

	assert.Contains(t, content, "primary.com")
	assert.Contains(t, content, "www.primary.com")
	assert.Contains(t, content, "alias.primary.com")
}

func TestInstallCaddyfileJob_GenerateCaddyfileContent_TlsOff(t *testing.T) {
	site := &models.Site{
		Address:                "http-only.com",
		Path:                   "/home/user/http-only.com",
		WebFolder:              "public",
		TlsSetting:             enums.TlsSettingOff,
		ZeroDowntimeDeployment: true,
	}

	job := &InstallCaddyfileJob{}
	content := job.generateCaddyfileContent(site)

	// TLS off should not have tls directive
	assert.NotContains(t, content, "tls internal")
}

func TestInstallCaddyfileJob_GenerateCaddyfileContent_TlsInternal(t *testing.T) {
	site := &models.Site{
		Address:                "internal.com",
		Path:                   "/home/user/internal.com",
		WebFolder:              "public",
		TlsSetting:             enums.TlsSettingInternal,
		ZeroDowntimeDeployment: true,
	}

	job := &InstallCaddyfileJob{}
	content := job.generateCaddyfileContent(site)

	assert.Contains(t, content, "tls internal")
}

func TestUpdateCaddyfileJob_GenerateCaddyfileContent(t *testing.T) {
	phpVersion := "8.1"
	site := &models.Site{
		Address:                "updated.com",
		Path:                   "/home/user/updated.com",
		WebFolder:              "public",
		TlsSetting:             enums.TlsSettingAuto,
		ZeroDowntimeDeployment: false,
		PhpVersion:             &phpVersion,
	}

	job := &UpdateCaddyfileJob{}
	content := job.generateCaddyfileContent(site)

	assert.Contains(t, content, "updated.com")
	assert.Contains(t, content, "php_fastcgi")
	assert.Contains(t, content, "php8.1-fpm.sock")
}

func TestInstallCaddyfileJob_GenerateCaddyfileContent_NoPhp(t *testing.T) {
	site := &models.Site{
		Address:                "static.com",
		Path:                   "/home/user/static.com",
		WebFolder:              "public",
		TlsSetting:             enums.TlsSettingAuto,
		ZeroDowntimeDeployment: true,
		PhpVersion:             nil, // No PHP
	}

	job := &InstallCaddyfileJob{}
	content := job.generateCaddyfileContent(site)

	// Should not contain php_fastcgi
	assert.NotContains(t, content, "php_fastcgi")
}

func TestInstallCaddyfileJob_GenerateCaddyfileContent_EmptyPhp(t *testing.T) {
	emptyPhp := ""
	site := &models.Site{
		Address:                "static2.com",
		Path:                   "/home/user/static2.com",
		WebFolder:              "public",
		TlsSetting:             enums.TlsSettingAuto,
		ZeroDowntimeDeployment: true,
		PhpVersion:             &emptyPhp, // Empty PHP version
	}

	job := &InstallCaddyfileJob{}
	content := job.generateCaddyfileContent(site)

	// Should not contain php_fastcgi for empty PHP version
	assert.NotContains(t, content, "php_fastcgi")
}

func TestInstallCaddyfileJob_GenerateCaddyfileContent_WebDirectory(t *testing.T) {
	site := &models.Site{
		Address:                "webapp.com",
		Path:                   "/home/user/webapp.com",
		WebFolder:              "public_html",
		TlsSetting:             enums.TlsSettingAuto,
		ZeroDowntimeDeployment: true,
	}

	job := &InstallCaddyfileJob{}
	content := job.generateCaddyfileContent(site)

	// Should use the web directory method
	webDir := site.GetWebDirectory()
	assert.Contains(t, content, webDir)
}

func TestInstallCaddyfileJob_GenerateCaddyfileContent_LogsDirectory(t *testing.T) {
	site := &models.Site{
		Address:                "logged.com",
		Path:                   "/home/user/logged.com",
		WebFolder:              "public",
		TlsSetting:             enums.TlsSettingAuto,
		ZeroDowntimeDeployment: true,
	}

	job := &InstallCaddyfileJob{}
	content := job.generateCaddyfileContent(site)

	logsDir := site.GetLogsDirectory()
	assert.Contains(t, content, logsDir)
}

package enums

import (
	"database/sql/driver"
	"fmt"
)

// SiteFileType represents the type of editable file on a site
type SiteFileType string

const (
	SiteFileCaddyfile       SiteFileType = "caddyfile"
	SiteFileEnvironment     SiteFileType = "environment"
	SiteFileComposerAuth    SiteFileType = "composer_auth"
	SiteFileWordpressConfig SiteFileType = "wordpress_config"
	SiteFileLaravelLog      SiteFileType = "laravel_log"
	SiteFileCaddyLog        SiteFileType = "caddy_log"
)

// String returns the string representation
func (f SiteFileType) String() string {
	return string(f)
}

// Name returns the display name for the file type
func (f SiteFileType) Name() string {
	names := map[SiteFileType]string{
		SiteFileCaddyfile:       "Caddyfile",
		SiteFileEnvironment:     "Environment file",
		SiteFileComposerAuth:    "Composer auth.json",
		SiteFileWordpressConfig: "WordPress config",
		SiteFileLaravelLog:      "Laravel log",
		SiteFileCaddyLog:        "Caddy Log",
	}

	if name, ok := names[f]; ok {
		return name
	}

	return string(f)
}

// Description returns the description for the file type
func (f SiteFileType) Description() string {
	descriptions := map[SiteFileType]string{
		SiteFileCaddyfile:       "The configuration file for Caddy. It is used to configure your site(s), including how to handle requests, TLS certificates, and more.",
		SiteFileEnvironment:     "The environment file for your site. It contains environment variables that are available to your site.",
		SiteFileComposerAuth:    "The Composer auth.json file contains your Composer authentication credentials, allowing you to install private packages from sources like Github and Bitbucket.",
		SiteFileWordpressConfig: "The configuration file for WordPress. It contains database credentials and other settings.",
		SiteFileLaravelLog:      "Laravel default log file created by the framework.",
		SiteFileCaddyLog:        "The Caddy Log contains all the requests that are made to your site.",
	}

	if desc, ok := descriptions[f]; ok {
		return desc
	}

	return ""
}

// FileType returns the file type category (default or environment)
func (f SiteFileType) FileType() string {
	if f == SiteFileEnvironment {
		return "environment"
	}

	return "default"
}

// IsEditable returns true if this file type is editable
func (f SiteFileType) IsEditable() bool {
	switch f {
	case SiteFileCaddyfile, SiteFileEnvironment, SiteFileComposerAuth, SiteFileWordpressConfig:
		return true
	}

	return false
}

// IsLog returns true if this file type is a log file
func (f SiteFileType) IsLog() bool {
	switch f {
	case SiteFileLaravelLog, SiteFileCaddyLog:
		return true
	}

	return false
}

// GetPath returns the file path for the given site configuration
func (f SiteFileType) GetPath(sitePath string, zeroDowntime bool) string {
	switch f {
	case SiteFileCaddyfile:
		return fmt.Sprintf("%s/Caddyfile", sitePath)

	case SiteFileEnvironment:
		if zeroDowntime {
			return fmt.Sprintf("%s/shared/.env", sitePath)
		}
		return fmt.Sprintf("%s/repository/.env", sitePath)

	case SiteFileComposerAuth:
		if zeroDowntime {
			return fmt.Sprintf("%s/shared/auth.json", sitePath)
		}
		return fmt.Sprintf("%s/repository/auth.json", sitePath)

	case SiteFileWordpressConfig:
		if zeroDowntime {
			return fmt.Sprintf("%s/shared/wp-config.php", sitePath)
		}
		return fmt.Sprintf("%s/repository/wp-config.php", sitePath)

	case SiteFileLaravelLog:
		if zeroDowntime {
			return fmt.Sprintf("%s/current/storage/logs/laravel.log", sitePath)
		}
		return fmt.Sprintf("%s/repository/storage/logs/laravel.log", sitePath)

	case SiteFileCaddyLog:
		return fmt.Sprintf("%s/logs/caddy.log", sitePath)
	}

	return ""
}

// IsValid checks if the file type is valid
func (f SiteFileType) IsValid() bool {
	switch f {
	case SiteFileCaddyfile, SiteFileEnvironment, SiteFileComposerAuth,
		SiteFileWordpressConfig, SiteFileLaravelLog, SiteFileCaddyLog:
		return true
	}

	return false
}

func (f *SiteFileType) Scan(value interface{}) error {
	if value == nil {
		*f = ""
		return nil
	}

	str, ok := value.(string)
	if !ok {
		bytes, ok := value.([]byte)
		if !ok {
			return fmt.Errorf("failed to scan SiteFileType: %v", value)
		}
		str = string(bytes)
	}

	*f = SiteFileType(str)

	return nil
}

func (f SiteFileType) Value() (driver.Value, error) {
	return string(f), nil
}

// EditableFilesForSiteType returns the editable file types for a given site type
func EditableFilesForSiteType(siteType SiteType) []SiteFileType {
	files := []SiteFileType{SiteFileCaddyfile}

	switch siteType {
	case SiteTypeWordpress:
		files = append(files, SiteFileWordpressConfig)
	case SiteTypeLaravel:
		files = append(files, SiteFileEnvironment, SiteFileComposerAuth)
	default:
		if siteType.HasEnvironment() {
			files = append(files, SiteFileEnvironment, SiteFileComposerAuth)
		}
	}

	return files
}

// LogFilesForSiteType returns the log file types for a given site type
func LogFilesForSiteType(siteType SiteType) []SiteFileType {
	files := []SiteFileType{SiteFileCaddyLog}

	if siteType == SiteTypeLaravel {
		files = append(files, SiteFileLaravelLog)
	}

	return files
}

// AllSiteFileTypes returns all valid site file types
func AllSiteFileTypes() []SiteFileType {
	return []SiteFileType{
		SiteFileCaddyfile,
		SiteFileEnvironment,
		SiteFileComposerAuth,
		SiteFileWordpressConfig,
		SiteFileLaravelLog,
		SiteFileCaddyLog,
	}
}

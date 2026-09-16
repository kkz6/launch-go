package dto

// PHPConfigurationResponse contains one editable, version-scoped PHP
// configuration file. Path is informational; clients cannot choose it.
type PHPConfigurationResponse struct {
	Kind     string `json:"kind"`
	Label    string `json:"label"`
	Path     string `json:"path"`
	Contents string `json:"contents"`
}

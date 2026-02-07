package types

import (
	"testing"
)

func TestPhpVersion_GetPhpMyAdminVersion(t *testing.T) {
	tests := []struct {
		php  PhpVersion
		want string
	}{
		{PhpVersion56, "4.9.11"},
		{PhpVersion70, "4.9.11"},
		{PhpVersion71, "5.1.4"},
		{PhpVersion72, "latest"},
		{PhpVersion73, "latest"},
		{PhpVersion74, "latest"},
		{PhpVersion80, "latest"},
		{PhpVersion81, "latest"},
		{PhpVersion82, "latest"},
		{PhpVersion83, "latest"},
		{PhpVersion84, "latest"},
	}

	for _, tt := range tests {
		t.Run(string(tt.php), func(t *testing.T) {
			got := tt.php.GetPhpMyAdminVersion()
			if got != tt.want {
				t.Errorf("PhpVersion(%s).GetPhpMyAdminVersion() = %q, want %q", tt.php, got, tt.want)
			}
		})
	}
}

func TestPhpVersion_GetPhpMyAdminDownloadURL(t *testing.T) {
	tests := []struct {
		php  PhpVersion
		want string
	}{
		{PhpVersion56, "https://files.phpmyadmin.net/phpMyAdmin/4.9.11/phpMyAdmin-4.9.11-all-languages.tar.gz"},
		{PhpVersion70, "https://files.phpmyadmin.net/phpMyAdmin/4.9.11/phpMyAdmin-4.9.11-all-languages.tar.gz"},
		{PhpVersion71, "https://files.phpmyadmin.net/phpMyAdmin/5.1.4/phpMyAdmin-5.1.4-all-languages.tar.gz"},
		{PhpVersion84, "https://www.phpmyadmin.net/downloads/phpMyAdmin-latest-all-languages.tar.gz"},
	}

	for _, tt := range tests {
		t.Run(string(tt.php), func(t *testing.T) {
			got := tt.php.GetPhpMyAdminDownloadURL()
			if got != tt.want {
				t.Errorf("PhpVersion(%s).GetPhpMyAdminDownloadURL() = %q, want %q", tt.php, got, tt.want)
			}
		})
	}
}

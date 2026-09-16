package tasks

import (
	"encoding/base64"
	"strings"
	"testing"
)

func TestReadPHPConfigurationUsesTrustedPathAndSizeLimit(t *testing.T) {
	task := ReadPHPConfiguration(ReadPHPConfigurationConfig{
		Path:     "/etc/php/8.3/fpm/php.ini",
		MaxBytes: 1234,
	})
	script := task.Script()

	for _, expected := range []string{
		"/etc/php/8.3/fpm/php.ini",
		"wc -c",
		`-gt 1234`,
		`cat -- "$target"`,
	} {
		if !strings.Contains(script, expected) {
			t.Fatalf("script does not contain %q:\n%s", expected, script)
		}
	}
}

func TestUpdatePHPConfigurationValidatesBacksUpAndReloads(t *testing.T) {
	contents := "memory_limit = 256M\n; unique raw value must not appear"
	task := UpdatePHPConfiguration(UpdatePHPConfigurationConfig{
		Kind:     "php_ini",
		Version:  "8.3",
		Path:     "/etc/php/8.3/fpm/php.ini",
		Contents: contents,
	})
	script := task.Script()

	for _, expected := range []string{
		base64.StdEncoding.EncodeToString([]byte(contents)),
		`php-fpm${version}`,
		`-t -c "$candidate" -y "/etc/php/${version}/fpm/php-fpm.conf"`,
		`.launchctl.bak`,
		`install -o root -g root -m 0644`,
		`systemctl reload "php${version}-fpm"`,
		PHPConfigValidationFailedMarker,
		PHPConfigReloadFailedMarker,
	} {
		if !strings.Contains(script, expected) {
			t.Fatalf("script does not contain %q:\n%s", expected, script)
		}
	}

	if strings.Contains(script, contents) {
		t.Fatal("script contains raw configuration contents")
	}
}

func TestUpdatePHPFPMConfigurationUsesCandidateFPMConfig(t *testing.T) {
	task := UpdatePHPConfiguration(UpdatePHPConfigurationConfig{
		Kind:     "php_fpm",
		Version:  "8.2",
		Path:     "/etc/php/8.2/fpm/php-fpm.conf",
		Contents: "include=/etc/php/8.2/fpm/pool.d/*.conf\n",
	})

	if !strings.Contains(task.Script(), `-c "/etc/php/${version}/fpm/php.ini" -y "$candidate"`) {
		t.Fatalf("FPM candidate is not validated as the active FPM config:\n%s", task.Script())
	}
}

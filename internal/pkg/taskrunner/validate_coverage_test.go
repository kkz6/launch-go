package taskrunner

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func installFakeShellcheck(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "shellcheck")
	script := `#!/bin/sh
case "${SHELLCHECK_MODE:-success}" in
  issues)
    printf '%s\n' 'stdin:1:1: error: unsafe expansion' '' 'stdin:2:1: warning: quote this' 'stdin:3:1: note: informational'
    exit 1
    ;;
  warning)
    printf '%s\n' 'stdin:1:1: warning: quote this'
    exit 1
    ;;
  failure)
    printf '%s\n' 'shellcheck engine failed' >&2
    exit 2
    ;;
  *)
    exit 0
    ;;
esac
`
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake shellcheck: %v", err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func TestScriptValidationWithoutShellcheck(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	if IsShellcheckAvailable() {
		t.Fatal("IsShellcheckAvailable() = true without shellcheck")
	}
	if result, err := ValidateScript("echo hello"); err == nil || result != nil || !strings.Contains(err.Error(), "shellcheck not found") {
		t.Fatalf("ValidateScript() = (%#v, %v)", result, err)
	}
	if result, err := ValidateScriptWithSeverity("echo hello", "warning"); err == nil || result != nil {
		t.Fatalf("ValidateScriptWithSeverity() = (%#v, %v)", result, err)
	}

	defer func() {
		if recovered := recover(); recovered == nil || !strings.Contains(recovered.(string), "script validation error") {
			t.Fatalf("MustValidateScript() panic = %v", recovered)
		}
	}()
	MustValidateScript("echo hello")
}

func TestScriptValidationSuccessAndIssues(t *testing.T) {
	installFakeShellcheck(t)
	if !IsShellcheckAvailable() {
		t.Fatal("IsShellcheckAvailable() = false with fake executable")
	}

	t.Run("success", func(t *testing.T) {
		t.Setenv("SHELLCHECK_MODE", "success")
		result, err := ValidateScript("echo hello")
		if err != nil || result == nil || !result.Valid || len(result.Errors) != 0 || len(result.Warnings) != 0 {
			t.Fatalf("ValidateScript() = (%#v, %v)", result, err)
		}
		MustValidateScript("echo hello")
	})

	t.Run("reported error and warning", func(t *testing.T) {
		t.Setenv("SHELLCHECK_MODE", "issues")
		result, err := ValidateScriptWithSeverity("bad script", "style")
		if err != nil {
			t.Fatalf("ValidateScriptWithSeverity() error = %v", err)
		}
		if result == nil || result.Valid || len(result.Errors) != 1 || len(result.Warnings) != 1 {
			t.Fatalf("validation result = %#v", result)
		}
		if !strings.Contains(result.Errors[0], "unsafe expansion") || !strings.Contains(result.Warnings[0], "quote this") {
			t.Fatalf("validation messages = %#v", result)
		}

		defer func() {
			if recovered := recover(); recovered == nil || !strings.Contains(recovered.(string), "script validation failed") {
				t.Fatalf("MustValidateScript() panic = %v", recovered)
			}
		}()
		MustValidateScript("bad script")
	})

	t.Run("warning keeps result valid", func(t *testing.T) {
		t.Setenv("SHELLCHECK_MODE", "warning")
		result, err := ValidateScript("warning script")
		if err != nil || result == nil || !result.Valid || len(result.Errors) != 0 || len(result.Warnings) != 1 {
			t.Fatalf("ValidateScript() = (%#v, %v)", result, err)
		}
	})

	t.Run("tool failure", func(t *testing.T) {
		t.Setenv("SHELLCHECK_MODE", "failure")
		result, err := ValidateScript("echo hello")
		if err == nil || result != nil || !strings.Contains(err.Error(), "shellcheck failed") || !strings.Contains(err.Error(), "engine failed") {
			t.Fatalf("ValidateScript() = (%#v, %v)", result, err)
		}
	})
}

func TestValidateScriptQuick(t *testing.T) {
	if err := ValidateScriptQuick("if true; then\n  echo valid\nfi\n"); err != nil {
		t.Fatalf("ValidateScriptQuick(valid) error = %v", err)
	}
	if err := ValidateScriptQuick("if true; then\n  echo invalid\n"); err == nil || !strings.Contains(err.Error(), "bash syntax error") {
		t.Fatalf("ValidateScriptQuick(invalid) error = %v", err)
	}
}

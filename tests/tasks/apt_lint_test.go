package tasks_test

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// Bare apt-get skips the dpkg lock timeout that aptGet() carries, so a
// timer firing mid-task fails the install outright instead of retrying.
//
// Go files are scanned as well as .sh templates: the PHP extension tasks
// built their scripts as inline string literals and so went unprotected
// while every .sh template was clean.
func TestTemplatesCallAptGetWrapper(t *testing.T) {
	bareAptGet := regexp.MustCompile(`(^|[|;&(]\s*|\bsudo\s+|\bthen\s+|\bdo\s+)(DEBIAN_FRONTEND=\S+\s+)?apt-get\b`)

	// The wrapper itself, and the pattern naming the binaries to wait on.
	allowed := map[string]bool{
		filepath.Join("..", "..", "internal", "pkg", "taskrunner", "templates", "functions.go"): true,
	}

	roots := []string{
		filepath.Join("..", "..", "internal", "modules"),
		filepath.Join("..", "..", "internal", "pkg"),
	}

	scan := func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		if ext := filepath.Ext(path); ext != ".sh" && ext != ".go" {
			return nil
		}
		if allowed[path] || strings.HasSuffix(path, "_test.go") {
			return nil
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		for i, line := range strings.Split(string(content), "\n") {
			trimmed := strings.TrimSpace(line)
			if trimmed == "" || strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, "//") {
				continue
			}
			if bareAptGet.MatchString(line) {
				t.Errorf("%s:%d calls apt-get directly, use aptGet instead:\n\t%s", path, i+1, trimmed)
			}
		}

		return nil
	}

	for _, root := range roots {
		if err := filepath.WalkDir(root, scan); err != nil {
			t.Fatalf("walking %s: %v", root, err)
		}
	}
}

// aptGet is defined by {{ aptFunctions }}; calling it without that include
// renders a script that dies with "aptGet: command not found".
func TestTemplatesUsingAptGetIncludeAptFunctions(t *testing.T) {
	// Concatenated into the provision script, which supplies the preamble
	// once at the top rather than per fragment.
	exempt := map[string]bool{"install_essential_packages.sh": true}

	root := filepath.Join("..", "..", "internal", "modules")

	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || filepath.Ext(path) != ".sh" {
			return err
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		body := string(content)
		if !strings.Contains(body, "aptGet ") || exempt[filepath.Base(path)] {
			return nil
		}
		if !strings.Contains(body, "aptFunctions") {
			t.Errorf("%s calls aptGet but never includes {{ aptFunctions }}", path)
		}

		return nil
	})
	if err != nil {
		t.Fatalf("walking templates: %v", err)
	}
}

package templates

import (
	"errors"
	"io/fs"
	"strings"
	"testing"
	"testing/fstest"
	"text/template"
)

type templateReadErrorFS struct {
	fs.FS
	err error
}

func (f templateReadErrorFS) ReadFile(string) ([]byte, error) {
	return nil, f.err
}

type templateOpenErrorFS struct {
	err error
}

func (f templateOpenErrorFS) Open(string) (fs.File, error) {
	return nil, f.err
}

func TestCommonShellFunctions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		got         string
		contains    []string
		notContains []string
	}{
		{
			name:     "strict defaults",
			got:      ShellDefaults(),
			contains: []string{"set -euo pipefail", "trap '' PIPE", "DEBIAN_FRONTEND=noninteractive"},
		},
		{
			name:        "lenient defaults",
			got:         ShellDefaultsLenient(),
			contains:    []string{"set -eu", "trap '' PIPE", "DEBIAN_FRONTEND=noninteractive"},
			notContains: []string{"pipefail"},
		},
		{
			name:     "common HTTP helpers",
			got:      CommonFunctions(),
			contains: []string{"function httpPostSilently()", "function httpPostRawSilently()", "--max-time 15"},
		},
		{
			name:     "apt lifecycle helpers",
			got:      AptFunctions(),
			contains: []string{"function waitForAptUnlock()", "function quiesceAptForProvisioning()", "function restoreUnattendedUpgrades()", "DPkg::Lock::Timeout=120"},
		},
		{
			name:     "PHP repositories",
			got:      PhpPpaFunctions(),
			contains: []string{"ubuntu)", "debian)", "ppa:ondrej/php", "packages.sury.org/php", "supports only Ubuntu or Debian"},
		},
		{
			name:     "task markers",
			got:      TaskMarkerFunctions(),
			contains: []string{"::LAUNCH::exit_code::", "::LAUNCH::progress::", "::LAUNCH::status::", "::LAUNCH::step_completed::", "::LAUNCH::software_installed::", "::LAUNCH::error::"},
		},
		{
			name:     "safe Caddy reload",
			got:      CaddyReloadFunction(),
			contains: []string{"function reloadCaddy()", "timeout 30", "systemctl restart caddy"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, fragment := range tt.contains {
				if !strings.Contains(tt.got, fragment) {
					t.Errorf("result does not contain %q", fragment)
				}
			}
			for _, fragment := range tt.notContains {
				if strings.Contains(tt.got, fragment) {
					t.Errorf("result unexpectedly contains %q", fragment)
				}
			}
		})
	}
}

func TestCommonFuncMap(t *testing.T) {
	t.Parallel()

	if got := CommonFuncMap["dirName"].(func(string) string)("a/b/file.sh"); got != "a/b" {
		t.Errorf("dirName() = %q, want %q", got, "a/b")
	}
	if got := CommonFuncMap["join"].(func([]string, string) string)([]string{"a", "b"}, ","); got != "a,b" {
		t.Errorf("join() = %q, want %q", got, "a,b")
	}
	if !CommonFuncMap["contains"].(func(string, string) bool)("launchctl", "ctl") {
		t.Error("contains() = false, want true")
	}
	if got := CommonFuncMap["lower"].(func(string) string)("PHP"); got != "php" {
		t.Errorf("lower() = %q, want %q", got, "php")
	}
	if got := CommonFuncMap["upper"].(func(string) string)("php"); got != "PHP" {
		t.Errorf("upper() = %q, want %q", got, "PHP")
	}
	if got := CommonFuncMap["trim"].(func(string) string)("  php  "); got != "php" {
		t.Errorf("trim() = %q, want %q", got, "php")
	}
	if got := CommonFuncMap["replace"].(func(string, string, string) string)("php-8-3", "-", "."); got != "php.8.3" {
		t.Errorf("replace() = %q, want %q", got, "php.8.3")
	}
	if got := CommonFuncMap["quote"].(func(string) string)("a b"); got != `"a b"` {
		t.Errorf("quote() = %q, want %q", got, `"a b"`)
	}
	if got := CommonFuncMap["escape"].(func(string) string)("it's"); got != `it'"'"'s` {
		t.Errorf("escape() = %q, want shell-safe apostrophe", got)
	}

	defaultValue := CommonFuncMap["default"].(func(interface{}, interface{}) interface{})
	if got := defaultValue("fallback", nil); got != "fallback" {
		t.Errorf("default(nil) = %v, want fallback", got)
	}
	if got := defaultValue("fallback", ""); got != "fallback" {
		t.Errorf("default(empty) = %v, want fallback", got)
	}
	if got := defaultValue("fallback", "value"); got != "value" {
		t.Errorf("default(value) = %v, want value", got)
	}

	for name, want := range map[string]string{
		"shellDefaults":        ShellDefaults(),
		"shellDefaultsLenient": ShellDefaultsLenient(),
		"aptFunctions":         AptFunctions(),
		"commonFuncs":          CommonFunctions(),
		"phpPpaFunctions":      PhpPpaFunctions(),
		"taskMarkerFuncs":      TaskMarkerFunctions(),
		"caddyReloadFunc":      CaddyReloadFunction(),
	} {
		if got := CommonFuncMap[name].(func() string)(); got != want {
			t.Errorf("CommonFuncMap[%q]() did not call the expected helper", name)
		}
	}
}

func TestRegistryLifecycleAndOptions(t *testing.T) {
	Reset()
	t.Cleanup(Reset)

	fixture := fstest.MapFS{
		"nested/basic.sh":   {Data: []byte(`{{define "basic"}}{{shellDefaults}}|{{greet .Name}}|{{quote .Name}}{{end}}`)},
		"nested/readme.txt": {Data: []byte("ignored")},
	}
	opts := &RegisterOptions{
		UseLenientShellMode: true,
		CustomFuncs: template.FuncMap{
			"greet": func(name string) string { return "hello " + name },
		},
	}

	if err := Register("fixture", fixture, opts); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	want := ShellDefaultsLenient() + `|hello launch|"launch"`
	if got, err := Render("fixture", "basic", struct{ Name string }{Name: "launch"}); err != nil {
		t.Fatalf("Render() error = %v", err)
	} else if got != want {
		t.Fatalf("Render() = %q, want %q", got, want)
	}
	if got := MustRender("fixture", "basic", struct{ Name string }{Name: "ctl"}); !strings.Contains(got, "hello ctl") {
		t.Fatalf("MustRender() = %q, want custom function output", got)
	}

	if err := Register("fixture", fixture, nil); err == nil || !strings.Contains(err.Error(), "already registered") {
		t.Fatalf("duplicate Register() error = %v, want already registered", err)
	}
	if _, err := Render("missing", "basic", nil); err == nil || !strings.Contains(err.Error(), `module "missing" not registered`) {
		t.Fatalf("Render() missing module error = %v", err)
	}
	if _, err := Render("fixture", "missing", nil); err == nil || !strings.Contains(err.Error(), "template fixture/missing") {
		t.Fatalf("Render() missing template error = %v", err)
	}

	Reset()
	if _, err := Render("fixture", "basic", nil); err == nil {
		t.Fatal("Render() succeeded after Reset()")
	}
}

func TestRegisterErrors(t *testing.T) {
	tests := []struct {
		name    string
		fsys    fs.FS
		wantErr string
	}{
		{
			name:    "walk",
			fsys:    templateOpenErrorFS{err: errors.New("walk failed")},
			wantErr: "walk failed",
		},
		{
			name: "read",
			fsys: templateReadErrorFS{
				FS:  fstest.MapFS{"task.sh": {Data: []byte("valid")}},
				err: errors.New("read failed"),
			},
			wantErr: "reading task.sh: read failed",
		},
		{
			name:    "parse",
			fsys:    fstest.MapFS{"task.sh": {Data: []byte("{{")}},
			wantErr: "parsing task.sh",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			Reset()
			t.Cleanup(Reset)

			err := Register(tt.name, tt.fsys, nil)
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("Register() error = %v, want substring %q", err, tt.wantErr)
			}
		})
	}
}

func TestMustRenderPanicsOnError(t *testing.T) {
	Reset()
	t.Cleanup(Reset)

	defer func() {
		if recovered := recover(); recovered == nil {
			t.Fatal("MustRender() did not panic")
		}
	}()

	MustRender("missing", "template", nil)
}

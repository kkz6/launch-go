package templates

import (
	"embed"
	"testing"
)

//go:embed testdata/*.sh
var hasTestTemplates embed.FS

func TestHas(t *testing.T) {
	const module = "has_test_module"

	if Has(module, "testdata/present.sh") {
		t.Fatal("an unregistered module should report no templates")
	}

	if err := Register(module, hasTestTemplates, nil); err != nil {
		t.Fatalf("Register() = %v", err)
	}

	if !Has(module, "testdata/present.sh") {
		t.Error("a registered template should be reported present")
	}
	if Has(module, "testdata/absent.sh") {
		t.Error("an unregistered name should be reported absent")
	}
	if Has("no_such_module", "testdata/present.sh") {
		t.Error("an unknown module should be reported absent")
	}
}

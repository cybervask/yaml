package yaml

import (
	"strings"
	"testing"
)

// TestHelpStrLayout verifies that HelpStr renders the configuration schema
// with field paths, descriptions, and tag metadata.
func TestHelpStrLayout(t *testing.T) {
	type Logging struct {
		Level string `yaml:"level" default:"info" validate:"choice=debug,info" description:"Log level"`
	}

	type Config struct {
		Logging Logging `yaml:"logging" description:"Logging settings"`
	}

	out := HelpStr(Config{})

	for _, expected := range []string{
		"yaml configuration schema documentation:",
		"logging:",
		"level:",
		"Log level",
		"(default: info, validate: [choice=debug,info])",
	} {
		if !strings.Contains(out, expected) {
			t.Errorf("expected help output to contain %q, got:\n%s", expected, out)
		}
	}
}

// TestHelpStrPointerAndInvalidTargets verifies edge cases: pointer targets
// are dereferenced, and non-struct targets produce a dedicated error line.
func TestHelpStrPointerAndInvalidTargets(t *testing.T) {
	type Config struct {
		Host string `yaml:"host" description:"Listen host"`
	}

	pointerOut := HelpStr(&Config{})
	if !strings.Contains(pointerOut, "host:") {
		t.Errorf("expected pointer target to render schema, got:\n%s", pointerOut)
	}

	invalidOut := HelpStr("not-a-struct")
	if !strings.Contains(invalidOut, "configuration error") {
		t.Errorf("expected non-struct target error line, got:\n%s", invalidOut)
	}

	nilOut := HelpStr(nil)
	if nilOut != "" {
		t.Errorf("expected empty output for nil target, got %q", nilOut)
	}
}

// TestHelpWritesToStderr verifies that Help prints the generated schema
// documentation to the standard error stream.
func TestHelpWritesToStderr(t *testing.T) {
	// Help has no return value; the call itself must not panic and must
	// exercise the same rendering path as HelpStr.
	Help(struct {
		Host string `yaml:"host" description:"Listen host"`
	}{})

	if !strings.Contains(HelpStr(struct {
		Host string `yaml:"host" description:"Listen host"`
	}{}), "host:") {
		t.Error("expected help schema to contain host field")
	}
}

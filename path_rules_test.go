package yaml

import (
	"strings"
	"testing"
)

// TestParsePathTokenSuccess verifies the path-token recognizer against valid
// relative-path rule forms: parameterized rules, flag rules, and multi-segment
// targets.
func TestParsePathTokenSuccess(t *testing.T) {
	tests := []struct {
		name   string
		token  string
		target string
		rule   string
		value  string
	}{
		{"single segment min", "min_len.min=0", "min_len", "min", "0"},
		{"single segment choice", "level.choice=dev,prod", "level", "choice", "dev,prod"},
		{"deep target", "limits.cpu.min=1", "limits.cpu", "min", "1"},
		{"flag rule required", "sub.required", "sub", "required", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			target, rule, value, ok := parsePathToken(tt.token)
			if !ok {
				t.Fatalf("expected token %q to parse, got rejection", tt.token)
			}

			if target != tt.target || rule != tt.rule || value != tt.value {
				t.Errorf("expected (target=%q rule=%q value=%q), got (target=%q rule=%q value=%q)",
					tt.target, tt.rule, tt.value, target, rule, value)
			}
		})
	}
}

// TestParsePathTokenRejection verifies that tokens without a known trailing
// rule segment are rejected so that ordinary unknown rules stay silently
// skipped (backward compatibility).
func TestParsePathTokenRejection(t *testing.T) {
	for _, token := range []string{"min=5", "plain", "a.b.c", "x.no_such_rule=1"} {
		if _, _, _, ok := parsePathToken(token); ok {
			t.Errorf("expected token %q to be rejected", token)
		}
	}
}

// TestParseValidateTagPathRules verifies the tag tokenizer collects multiple
// path rules under the path:<target> key and keeps ordinary rules intact.
func TestParseValidateTagPathRules(t *testing.T) {
	rules := parseValidateTag("required,min_len.min=0,max_len.max=64")

	if rules["required"] != "" {
		t.Errorf("expected required flag to be preserved, got %q", rules["required"])
	}

	if v, ok := rules["path:min_len"]; !ok || v != "min=0" {
		t.Errorf("expected path:min_len entry with min=0, got %q (present=%v)", v, ok)
	}

	if v, ok := rules["path:max_len"]; !ok || v != "max=64" {
		t.Errorf("expected path:max_len entry with max=64, got %q (present=%v)", v, ok)
	}
}

// TestPathRulesPerInstanceBounds verifies the core business case from
// examples/17_embed: a shared nested type whose fields get different
// validation bounds per parent instance.
func TestPathRulesPerInstanceBounds(t *testing.T) {
	type ServerDesc struct {
		Description string `yaml:"description"`
		MinLen      int    `yaml:"min_len"`
		MaxLen      int    `yaml:"max_len"`
	}

	type Server struct {
		BindEndpoint string     `yaml:"bind_endpoint" validate:"endpoint" description:"host:port bind address"`
		Workers      int        `yaml:"workers" validate:"min=1,max=64" description:"Worker pool size"`
		ServerDesc1  ServerDesc `yaml:"server_desc1" validate:"min_len.min=0,max_len.max=64" description:"Server Description 1"`
		ServerDesc2  ServerDesc `yaml:"server_desc2" validate:"min_len.min=4,max_len.max=32" description:"Server Description 2"`
	}

	type Config struct {
		Server     Server `yaml:"server"`
		AdminEmail string `yaml:"admin_email" validate:"regexp=^[^@]+@[^@]+$" description:"Administrator contact"`
	}

	valid := Config{}
	if err := Load([]byte(`
server:
  bind_endpoint: "127.0.0.1:443"
  workers: 8
  server_desc1:
    description: some description 1
    min_len: 0
    max_len: 10
  server_desc2:
    description: some description 2
    min_len: 5
    max_len: 20
admin_email: "admin@cybervask.net"
`), &valid); err != nil {
		t.Fatalf("expected valid profile to pass, got: %v", err)
	}

	invalid := Config{}
	err := Load([]byte(`
server:
  bind_endpoint: "127.0.0.1:443"
  workers: 8
  server_desc1:
    min_len: -1
    max_len: 100
  server_desc2:
    min_len: 2
    max_len: 50
admin_email: "admin@cybervask.net"
`), &invalid)
	if err == nil {
		t.Fatal("expected per-instance bound violations, got nil")
	}

	for _, expected := range []string{
		"field server.server_desc1.min_len: value -1 < min 0",
		"field server.server_desc1.max_len: value 100 > max 64",
		"field server.server_desc2.min_len: value 2 < min 4",
		"field server.server_desc2.max_len: value 50 > max 32",
	} {
		if !strings.Contains(err.Error(), expected) {
			t.Errorf("expected error report to contain %q, got:\n%s", expected, err)
		}
	}
}

// TestPathRulesDanglingTarget verifies that a path rule pointing to a
// non-existent field is reported as a configuration error.
func TestPathRulesDanglingTarget(t *testing.T) {
	type Desc struct {
		MinLen int `yaml:"min_len"`
	}

	type Config struct {
		D Desc `yaml:"d" validate:"min_size.min=1" description:"desc"`
	}

	err := Validate(&Config{})
	if err == nil {
		t.Fatal("expected dangling path error, got nil")
	}

	if !strings.Contains(err.Error(), `path rule "min_size" references a non-existent field`) {
		t.Errorf("unexpected error text: %v", err)
	}
}

// TestPathRulesMixedWithOwnerRules verifies that owner-level rules and path
// rules coexist: required applies to the owner field, min=1 applies to its
// nested target.
func TestPathRulesMixedWithOwnerRules(t *testing.T) {
	type Desc struct {
		MinLen int `yaml:"min_len"`
	}

	type Holder struct {
		D Desc `yaml:"d" validate:"required,min_len.min=1" description:"desc"`
	}

	type Config struct {
		H Holder `yaml:"h"`
	}

	pass := Config{}
	pass.H.D.MinLen = 5
	if err := Validate(&pass); err != nil {
		t.Errorf("expected valid config to pass, got: %v", err)
	}

	fail := Config{}
	fail.H.D.MinLen = 0
	err := Validate(&fail)
	if err == nil {
		t.Fatal("expected mixed violations, got nil")
	}

	for _, expected := range []string{
		"field h.d.min_len: value 0 < min 1",
		"field h.d: is empty, but required",
	} {
		if !strings.Contains(err.Error(), expected) {
			t.Errorf("expected error report to contain %q, got:\n%s", expected, err)
		}
	}
}

// TestPathRulesDeepTarget verifies multi-segment targets that cross several
// nested struct levels.
func TestPathRulesDeepTarget(t *testing.T) {
	type Limits struct {
		CPU int `yaml:"cpu"`
	}

	type Runtime struct {
		Limits Limits `yaml:"limits"`
	}

	type Config struct {
		Runtime Runtime `yaml:"runtime" validate:"limits.cpu.min=2" description:"runtime settings"`
	}

	low := Config{}
	low.Runtime.Limits.CPU = 1
	err := Validate(&low)
	if err == nil {
		t.Fatal("expected deep path violation, got nil")
	}

	if !strings.Contains(err.Error(), "field runtime.limits.cpu: value 1 < min 2") {
		t.Errorf("unexpected error text: %v", err)
	}

	high := Config{}
	high.Runtime.Limits.CPU = 4
	if err := Validate(&high); err != nil {
		t.Errorf("expected valid config to pass, got: %v", err)
	}
}

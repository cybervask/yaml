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
		{"dotted regexp value", "msg.regexp=^a.b.c$", "msg", "regexp", "^a.b.c$"},
		{"escaped dot regexp value", `msg.regexp=^a\.b$`, "msg", "regexp", `^a\.b$`},
		{"parameter with equals and dot", "x.regexp=a=b.c", "x", "regexp", "a=b.c"},
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
// skipped (backward compatibility). A flag rule whose dot segments leave a
// non-empty trailing tail behind is rejected too: attaching the tail to a
// flag value would silently drop it.
func TestParsePathTokenRejection(t *testing.T) {
	for _, token := range []string{"min=5", "plain", "a.b.c", "x.no_such_rule=1", "t.endpoint.extra"} {
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

// TestPathRulesRegexpValueWithDots verifies end-to-end that dots inside a
// path-rule value survive parsing: the full regexp is compiled, so a value
// that only matches the truncated pattern is rejected while a matching one
// passes. Before the fix, the back-off loop truncated the value to
// "^a", silently weakening the pattern until any "a..." string passed.
func TestPathRulesRegexpValueWithDots(t *testing.T) {
	type Holder struct {
		Body string `yaml:"body"`
	}

	type Config struct {
		Msg Holder `yaml:"msg" validate:"body.regexp=^a.b.c$" description:"body"`
	}

	pass := Config{Msg: Holder{Body: "axbyc"}}
	if err := Validate(&pass); err != nil {
		t.Errorf("expected matching value to pass the full dotted regexp, got: %v", err)
	}

	fail := Config{Msg: Holder{Body: "aZZZ"}}
	err := Validate(&fail)
	if err == nil {
		t.Fatal("expected non-matching value to be rejected, got nil")
	}

	if !strings.Contains(err.Error(), `field msg.body: value "aZZZ" does not match regular expression "^a.b.c$"`) {
		t.Errorf("unexpected error text: %v", err)
	}
}

// TestPathRulesThroughPointerField verifies that a path rule works through
// pointer fields: the owner field and the intermediate target segment are
// non-nil pointers, resolvePathTarget dereferences them along the walk, and
// the rule fires on the dereferenced value. Characterization test for the
// resolveStructField/findStructTarget deduplication: pins the deref of
// non-nil pointer fields on path resolution.
func TestPathRulesThroughPointerField(t *testing.T) {
	type Inner struct {
		MinLen int `yaml:"min_len"`
	}

	type Sub struct {
		Inner *Inner `yaml:"inner"`
	}

	type Config struct {
		Sub *Sub `yaml:"sub" validate:"inner.min_len.min=2"`
	}

	valid := Config{Sub: &Sub{Inner: &Inner{MinLen: 3}}}
	if err := Validate(&valid); err != nil {
		t.Errorf("expected dereferenced value to satisfy the path rule, got: %v", err)
	}

	low := Config{Sub: &Sub{Inner: &Inner{MinLen: 1}}}
	err := Validate(&low)
	if err == nil {
		t.Fatal("expected path rule violation on the dereferenced value, got nil")
	}

	if !strings.Contains(err.Error(), "field sub.inner.min_len: value 1 < min 2") {
		t.Errorf("unexpected error text: %v", err)
	}
}

// TestPathRulesRegexpEscapedDotValue verifies the escaped-dot regexp
// end-to-end: "a.b" matches and "aXb" does not. The tag carries a doubled
// backslash because reflect.StructTag unquotes tag values, so "\\." reaches
// the engine as the regexp escape "\.". Before the fix, the value
// was truncated to "^a\" (a trailing backslash), an invalid regexp, so the
// matching value was falsely rejected with a syntax error.
func TestPathRulesRegexpEscapedDotValue(t *testing.T) {
	type Holder struct {
		Body string `yaml:"body"`
	}

	type Config struct {
		Msg Holder `yaml:"msg" validate:"body.regexp=^a\\.b$"`
	}

	pass := Config{Msg: Holder{Body: "a.b"}}
	if err := Validate(&pass); err != nil {
		t.Errorf("expected dotted value to match escaped-dot regexp, got: %v", err)
	}

	fail := Config{Msg: Holder{Body: "aXb"}}
	if err := Validate(&fail); err == nil {
		t.Fatal("expected non-matching value to be rejected, got nil")
	}
}

// TestParseValidateTagPathRuleQuotedComma verifies that the path-rule token
// scan tracks quotes: a comma inside a quoted value does not terminate the
// token, so the full value survives into the path:<target> entry. Before the
// fix, the scan stopped at the first comma and truncated the value to '^a.
func TestParseValidateTagPathRuleQuotedComma(t *testing.T) {
	rules := parseValidateTag(`msg.regexp='^a,b$'`)

	if v := rules["path:msg"]; v != "regexp='^a,b$'" {
		t.Errorf("expected path:msg entry with the full quoted value, got %q", v)
	}

	rules = parseValidateTag(`msg.regexp="^a,b$"`)

	if v := rules["path:msg"]; v != `regexp="^a,b$"` {
		t.Errorf("expected path:msg entry with the full double-quoted value, got %q", v)
	}
}

// TestPathRulesQuotedCommaValue verifies the end-to-end behavior of a quoted
// comma inside a path-rule regexp value: the pattern keeps its comma, so
// "a,b" matches and "aXb" does not. Before the fix, the tokenizer truncated
// the pattern at the comma, disabling the validation silently.
func TestPathRulesQuotedCommaValue(t *testing.T) {
	type inner struct {
		Msg string `yaml:"msg"`
	}
	type Config struct {
		Inner inner `yaml:"inner" validate:"msg.regexp='^a,b$'"`
	}

	valid := Config{Inner: inner{Msg: "a,b"}}
	if err := Load([]byte("inner:\n  msg: a,b\n"), &valid); err != nil {
		t.Fatalf("expected quoted comma pattern to accept \"a,b\", got: %v", err)
	}

	invalid := Config{Inner: inner{Msg: "aXb"}}
	err := Load([]byte("inner:\n  msg: aXb\n"), &invalid)
	if err == nil {
		t.Fatal("expected quoted comma pattern to reject \"aXb\", got nil")
	}

	if !strings.Contains(err.Error(), `does not match regular expression "^a,b$"`) {
		t.Errorf("unexpected error text: %v", err)
	}
}

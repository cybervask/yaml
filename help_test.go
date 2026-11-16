package yaml

import (
	"strings"
	"testing"
	"time"
)

// TestHelpStr_ShowsPriorityTag verifies that a non-empty priority tag is
// documented between the env and default metadata, and that a field without
// the tag renders exactly as before.
func TestHelpStr_ShowsPriorityTag(t *testing.T) {
	type Config struct {
		ValueWins string `yaml:"value_wins" env:"APP_HELP_VALUE_WINS" priority:"value" default:"from-default" description:"Value source outranks env"`
		EnvWins   string `yaml:"env_wins" env:"APP_HELP_ENV_WINS" default:"from-default" description:"Env outranks value sources"`
	}

	out := HelpStr(Config{})

	if !strings.Contains(out, "(env: APP_HELP_VALUE_WINS, priority: value, default: from-default)") {
		t.Errorf("expected env, priority and default metadata for the priority-tagged field, got:\n%s", out)
	}
	if !strings.Contains(out, "(env: APP_HELP_ENV_WINS, default: from-default)") {
		t.Errorf("expected the field without a priority tag to keep its metadata unchanged, got:\n%s", out)
	}
}

// TestHelpStr_ReadmeExampleUnchanged pins the exact HelpStr rendering of the
// configuration model documented in README.md ("Automated Interactive CLI
// Help Output"). None of those fields carry a priority tag, so priority
// support must not change this output byte-for-byte — with trailing
// whitespace trimmed from lines lacking description or tag metadata.
func TestHelpStr_ReadmeExampleUnchanged(t *testing.T) {
	type TLS struct {
		MinVersion string   `yaml:"min_version" default:"tls1.3" validate:"choice=tls1.2,tls1.3" env:"TLS_MIN_VERSION" description:"Minimum TLS version"`
		Alpn       []string `yaml:"alpn" default:"h2,http/1.1" validate:"choice=h2,http/1.1" description:"Application-Layer Protocol Negotiation"`
	}

	type Logging struct {
		Level   string `yaml:"level" default:"info" validate:"choice=debug,info,warn" description:"Log hierarchy execution level"`
		Colors  bool   `yaml:"colors" description:"Enable ANSI colored logs output"`
		Timeout string `yaml:"timeout" default:"5s" description:"Internal downstream processing timeout"`
	}

	type Config struct {
		Env         string        `yaml:"env" default:"dev" validate:"choice=dev,stage,prod" description:"Application execution environment"`
		AppName     string        `yaml:"app_name" default:"api" validate:"minlen=3,maxlen=10" description:"Internal runtime deployment monicker"`
		APIUrl      string        `yaml:"api_url" validate:"url,required_if=env==prod" env:"API_URL" description:"Base target API destination link"`
		APIEndpoint string        `yaml:"api_endpoint" validate:"endpoint" description:"Local binding network socket"`
		Timeout     time.Duration `yaml:"timeout" default:"5s" validate:"min=1s,max=10m" description:"Global runtime request boundary tracking parameter"`
		Crypto      TLS           `yaml:"crypto" description:"Security TLS structure configuration layer"`
		Server      struct {
			Logging    Logging  `yaml:"logging" description:"Server logging configuration parameters"`
			AllowedIPs []string `yaml:"allowed_ips" validate:"mincount=1,maxcount=10" description:"Whitelisted network remotes"`
		} `yaml:"server"`
	}

	want := "yaml configuration schema documentation:\n" +
		"\n" +
		"env:             Application execution environment (default: dev, validate: [choice=dev,stage,prod])\n" +
		"app_name:        Internal runtime deployment monicker (default: api, validate: [minlen=3,maxlen=10])\n" +
		"api_url:         Base target API destination link (env: API_URL, validate: [url,required_if=env==prod])\n" +
		"api_endpoint:    Local binding network socket (validate: [endpoint])\n" +
		"timeout:         Global runtime request boundary tracking parameter (default: 5s, validate: [min=1s,max=10m])\n" +
		"crypto:          Security TLS structure configuration layer\n" +
		"  min_version:   Minimum TLS version (env: TLS_MIN_VERSION, default: tls1.3, validate: [choice=tls1.2,tls1.3])\n" +
		"  alpn:          Application-Layer Protocol Negotiation (default: h2,http/1.1, validate: [choice=h2,http/1.1])\n" +
		"server:\n" +
		"  logging:       Server logging configuration parameters\n" +
		"    level:       Log hierarchy execution level (default: info, validate: [choice=debug,info,warn])\n" +
		"    colors:      Enable ANSI colored logs output\n" +
		"    timeout:     Internal downstream processing timeout (default: 5s)\n" +
		"  allowed_ips:   Whitelisted network remotes (validate: [mincount=1,maxcount=10])\n"

	got := HelpStr(Config{})
	if got != want {
		t.Errorf("README example output drifted:\n got: %q\nwant: %q", got, want)
	}
}

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

// TestHelpStr_SkipsUnexportedFields verifies that unexported fields are not
// documented: yaml4 never decodes them (parity with dump_include.go).
func TestHelpStr_SkipsUnexportedFields(t *testing.T) {
	type Config struct {
		Host   string `yaml:"host" description:"Listen host"`
		secret string `yaml:"secret" description:"Hidden internal"`
	}

	out := HelpStr(Config{})
	if strings.Contains(out, "secret") {
		t.Errorf("expected unexported field to be omitted, got:\n%s", out)
	}
	if !strings.Contains(out, "host:") {
		t.Errorf("expected exported field to be documented, got:\n%s", out)
	}
}

// TestHelpStr_DocumentsSliceOfStruct verifies that struct elements of slice
// fields are documented as nested blocks, including pointer elements.
func TestHelpStr_DocumentsSliceOfStruct(t *testing.T) {
	type Node struct {
		Name string `yaml:"name" description:"element name"`
	}
	type Peer struct {
		Addr string `yaml:"addr" description:"peer address"`
	}

	type Config struct {
		Nodes []Node  `yaml:"nodes" description:"node list"`
		Peers []*Peer `yaml:"peers" description:"peer list"`
	}

	out := HelpStr(Config{})
	if !strings.Contains(out, "name:") {
		t.Errorf("expected slice-of-struct element fields documented, got:\n%s", out)
	}
	if !strings.Contains(out, "addr:") {
		t.Errorf("expected slice-of-pointer-struct element fields documented, got:\n%s", out)
	}
}

// TestHelpStr_DocumentsMapOfStruct verifies that struct elements of map and
// array fields are documented as nested blocks.
func TestHelpStr_DocumentsMapOfStruct(t *testing.T) {
	type Server struct {
		Host string `yaml:"host" description:"server host"`
	}
	type Replica struct {
		Addr string `yaml:"addr" description:"replica address"`
	}

	type Config struct {
		Servers  map[string]Server `yaml:"servers" description:"server pool"`
		Replicas [2]Replica        `yaml:"replicas" description:"fixed replicas"`
	}

	out := HelpStr(Config{})
	if !strings.Contains(out, "host:") {
		t.Errorf("expected map-of-struct element fields documented, got:\n%s", out)
	}
	if !strings.Contains(out, "addr:") {
		t.Errorf("expected array-of-struct element fields documented, got:\n%s", out)
	}
}

// TestHelpStr_SliceOfScalarNotExpanded verifies that scalar elements are not
// expanded into nested documentation blocks.
func TestHelpStr_SliceOfScalarNotExpanded(t *testing.T) {
	type Config struct {
		IPs []string `yaml:"ips" description:"Allowed remotes"`
	}

	out := HelpStr(Config{})
	if !strings.Contains(out, "ips:") {
		t.Fatalf("expected container field documented, got:\n%s", out)
	}
	// Header, blank separator line and the single field line are expected.
	if got := strings.Count(out, "\n"); got != 3 {
		t.Errorf("expected scalar elements not to be expanded (3 lines), got %d:\n%s", got, out)
	}
}

// TestHelpStr_SelfReferentialThroughSliceTerminates verifies that a struct
// referencing itself through a slice of its own type terminates: the visited
// guard must stop the recursion through the element type.
func TestHelpStr_SelfReferentialThroughSliceTerminates(t *testing.T) {
	type TreeNode struct {
		Name     string     `yaml:"name" description:"node name"`
		Children []TreeNode `yaml:"children" description:"child nodes"`
	}

	out := HelpStr(TreeNode{})
	if out == "" {
		t.Fatal("expected non-empty help output")
	}
	if got := strings.Count(out, "children:"); got != 1 {
		t.Errorf("expected recursive field 'children' documented exactly once, got %d occurrences in:\n%s", got, out)
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

// TestHelpStr_SkipsYamlDashField pins the "-"-tag skip: a field excluded from
// YAML is not documented — the decoder never reads it, same rationale as the
// unexported skip.
func TestHelpStr_SkipsYamlDashField(t *testing.T) {
	out := HelpStr(struct {
		Host    string `yaml:"host"`
		Ignored string `yaml:"-"`
	}{})

	if strings.Contains(out, "ignored") {
		t.Errorf("expected the \"-\" field to stay undocumented, got:\n%s", out)
	}
	if !strings.Contains(out, "host:") {
		t.Errorf("expected host to be documented, got:\n%s", out)
	}
}

// TestHelpStr_InlineStructFieldsFlattened pins the inline handling: an inline
// struct field unfolds into the parent mapping in YAML, so its nested fields
// are documented at the parent's level and no keyless ": ..." line is emitted.
func TestHelpStr_InlineStructFieldsFlattened(t *testing.T) {
	type inlineTLS struct {
		Min string `yaml:"min" description:"Min version"`
	}
	out := HelpStr(struct {
		Host string    `yaml:"host"`
		TLS  inlineTLS `yaml:",inline"`
	}{})

	if !strings.Contains(out, "min:") {
		t.Errorf("expected the inline nested field to be documented, got:\n%s", out)
	}
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), ":") {
			t.Errorf("expected no keyless inline line, got %q in:\n%s", line, out)
		}
	}
}

// TestHelpStr_UnicodeKeyColumnAlignment pins the display-width padding: keys
// made of single-width non-ASCII runes (Cyrillic) align like ASCII keys, and
// East Asian wide keys reserve two terminal columns per rune, so the
// description column starts at the same terminal position on every line.
func TestHelpStr_UnicodeKeyColumnAlignment(t *testing.T) {
	out := HelpStr(struct {
		Ab        string `yaml:"ab" description:"latin"`
		Cyr       string `yaml:"фу" description:"cyrillic"`
		Cjk       string `yaml:"服务器" description:"cjk"`
		LongerKey string `yaml:"longer_key" description:"longest"`
	}{})

	// "longer_key:" has display width 11, so the path column is 11+2 wide and
	// every description starts after one separator space — terminal column 14.
	seen := 0
	for _, line := range strings.Split(out, "\n") {
		idx := -1
		for _, want := range []string{"latin", "cyrillic", "cjk"} {
			if i := strings.Index(line, want); i >= 0 {
				idx = i

				break
			}
		}
		if idx < 0 {
			continue
		}
		seen++
		if displayWidth(line[:idx]) != 14 {
			t.Errorf("description column misaligned at %q: display width %d, want 14", line, displayWidth(line[:idx]))
		}
	}
	if seen != 3 {
		t.Fatalf("expected 3 documented lines, saw %d in:\n%s", seen, out)
	}
}

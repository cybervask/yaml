package yaml

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
)

// TestSubConfig models a nested structural configuration block for testing logger properties.
type TestSubConfig struct {
	Level   string        `yaml:"level" default:"info" validate:"choice=debug,info,warn"`
	Timeout time.Duration `yaml:"timeout" default:"5s"`
}

// TestConfig models the primary composite structure used to evaluate structural tag mechanics.
type TestConfig struct {
	Env          string        `yaml:"env" default:"dev" validate:"choice=dev,stage,prod"`
	Color        string        `yaml:"color" default:"white" validate:"choice=!red,!black"`
	BindAddr     string        `yaml:"bind_addr" validate:"endpoint"`
	WebURL       string        `yaml:"web_url" validate:"url"`
	Code         string        `yaml:"code" validate:"regexp=^[a-z]{2,4}$"`
	Workers      uint          `yaml:"workers" default:"10" validate:"min=1,max=100"`
	RequiredItem string        `yaml:"req" validate:"not_empty"`
	Logging      TestSubConfig `yaml:"logging"`
}

// TestSetDefaultsAndValidate_Success verifies that default configuration fields
// are correctly applied and that valid configurations pass the validation step.
func TestSetDefaultsAndValidate_Success(t *testing.T) {
	cfg := TestConfig{
		BindAddr:     "127.0.0.1:8080",
		WebURL:       "https://github.com",
		Code:         "yaml",
		RequiredItem: "present",
	}

	// Apply default values to the unassigned structure fields.
	if err := SetDefaults(&cfg); err != nil {
		t.Fatalf("unexpected error in SetDefaults: %v", err)
	}

	// Validate field value fallback assignments.
	if cfg.Env != "dev" {
		t.Errorf("expected Env to be 'dev', got %q", cfg.Env)
	}

	if cfg.Color != "white" {
		t.Errorf("expected Color to be 'white', got %q", cfg.Color)
	}

	if cfg.Workers != 10 {
		t.Errorf("expected Workers to be 10, got %d", cfg.Workers)
	}

	if cfg.Logging.Level != "info" {
		t.Errorf("expected Logging.Level to be 'info', got %q", cfg.Logging.Level)
	}

	// Validate the structural data state.
	if err := Validate(&cfg); err != nil {
		t.Fatalf("unexpected validation error: %v", err)
	}
}

// TestValidate_Errors evaluates various configuration edge cases that should
// trigger validation constraint violations.
func TestValidate_Errors(t *testing.T) {
	tests := []struct {
		name      string
		modify    func(*TestConfig)
		errSubstr string
	}{
		{
			name: "not_empty violation",
			modify: func(c *TestConfig) {
				c.RequiredItem = ""
			},
			errSubstr: "is empty, but required",
		},
		{
			name: "whitelist choice violation",
			modify: func(c *TestConfig) {
				c.Env = "invalid_env"
			},
			errSubstr: "is invalid; allowed choices are",
		},
		{
			name: "blacklist choice violation",
			modify: func(c *TestConfig) {
				c.Color = "red"
			},
			errSubstr: "is forbidden by blacklist",
		},
		{
			name: "regexp violation",
			modify: func(c *TestConfig) {
				c.Code = "toolongcode"
			},
			errSubstr: "does not match regular expression",
		},
		{
			name: "endpoint violation",
			modify: func(c *TestConfig) {
				c.BindAddr = "google.com" // Missing mandatory port suffix.
			},
			errSubstr: "is not a valid host:port format",
		},
		{
			name: "url missing scheme",
			modify: func(c *TestConfig) {
				c.WebURL = "google.com"
			},
			errSubstr: "is missing a URL scheme separator",
		},
		{
			name: "url missing host",
			modify: func(c *TestConfig) {
				c.WebURL = "http://"
			},
			errSubstr: "is missing a URL host",
		},
		{
			name: "url with port-only authority",
			modify: func(c *TestConfig) {
				c.WebURL = "http://:8080"
			},
			errSubstr: "is missing a URL host",
		},
		{
			name: "min range violation",
			modify: func(c *TestConfig) {
				c.Workers = 0
			},
			errSubstr: "< min",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := TestConfig{
				BindAddr:     "127.0.0.1:8080",
				WebURL:       "https://github.com",
				Code:         "yaml",
				RequiredItem: "present",
			}
			_ = SetDefaults(&cfg)
			tt.modify(&cfg)

			err := Validate(&cfg)
			if err == nil {
				t.Fatal("expected validation error, got nil")
			}
			if !strings.Contains(err.Error(), tt.errSubstr) {
				t.Errorf("expected error containing %q, got %q", tt.errSubstr, err.Error())
			}
		})
	}
}

// TestMutualExclusiveTags validates that structural declarations containing both
// `default` and `not_empty` constraints trigger an error during execution.
func TestMutualExclusiveTags(t *testing.T) {
	type BadConfig struct {
		Value string `yaml:"val" default:"secret" validate:"not_empty"`
	}

	var cfg BadConfig
	err := SetDefaults(&cfg)
	if err == nil {
		t.Fatal("expected error due to mutual exclusive tags, got nil")
	}
	if !strings.Contains(err.Error(), "mutually exclusive") {
		t.Errorf("expected mutual exclusive error text, got %q", err.Error())
	}
}

// TestValidate_SliceElements verifies that structural validations are recursively
// applied over sequence structures (slices) for both integers and strings.
func TestValidate_SliceElements(t *testing.T) {
	type SliceConfig struct {
		AllowedTags []string `yaml:"tags" validate:"choice=golang,docker,k8s"`
		Ports       []uint   `yaml:"ports" validate:"min=80,max=443"`
	}

	cfg := SliceConfig{
		AllowedTags: []string{"golang", "k8s"},
		Ports:       []uint{80, 443},
	}
	if err := Validate(&cfg); err != nil {
		t.Fatalf("expected slice config to be valid, got err: %v", err)
	}

	badCfg := SliceConfig{
		AllowedTags: []string{"golang", "java"}, // "java" is missing from choices.
		Ports:       []uint{80},
	}
	err := Validate(&badCfg)
	if err == nil {
		t.Fatal("expected validation error for invalid slice string element, got nil")
	}
	// Verified to map correctly against lowercase tag path segments
	if !strings.Contains(err.Error(), "tags[1]: value \"java\" is invalid") {
		t.Errorf("unexpected error text: %v", err)
	}

	badPortsCfg := SliceConfig{
		AllowedTags: []string{"golang"},
		Ports:       []uint{22}, // 22 is below the min=80 threshold.
	}
	err = Validate(&badPortsCfg)
	if err == nil {
		t.Fatal("expected validation error for invalid slice integer element, got nil")
	}
	if !strings.Contains(err.Error(), "< min") {
		t.Errorf("unexpected error text: %v", err)
	}
}

// TestSetDefaults_InCollections simulates YAML unmarshalling states and checks if
// unassigned nested collection fields are correctly initialized recursively.
func TestSetDefaults_InCollections(t *testing.T) {
	type ItemConfig struct {
		Name  string `yaml:"name" default:"unknown"`
		Count int    `yaml:"count" default:"5"`
	}

	type HolderConfig struct {
		ItemsList []ItemConfig          `yaml:"items_list"`
		ItemsMap  map[string]ItemConfig `yaml:"items_map"`
	}

	cfg := HolderConfig{
		ItemsList: []ItemConfig{
			{Name: "first"},
			{},
		},
		ItemsMap: map[string]ItemConfig{
			"key1": {Name: "mapped"},
			"key2": {},
		},
	}

	if err := SetDefaults(&cfg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.ItemsList[0].Count != 5 {
		t.Errorf("expected ItemsList[0].Count to be 5, got %d", cfg.ItemsList[0].Count)
	}
	if cfg.ItemsList[1].Name != "unknown" || cfg.ItemsList[1].Count != 5 {
		t.Errorf("expected ItemsList defaults to be set, got Name=%q, Count=%d", cfg.ItemsList[1].Name, cfg.ItemsList[1].Count)
	}

	if cfg.ItemsMap["key1"].Count != 5 {
		t.Errorf("expected ItemsMap['key1'].Count to be 5, got %d", cfg.ItemsMap["key1"].Count)
	}
	if cfg.ItemsMap["key2"].Name != "unknown" || cfg.ItemsMap["key2"].Count != 5 {
		t.Errorf("expected ItemsMap['key2'] defaults to be set, got Name=%q, Count=%d", cfg.ItemsMap["key2"].Name, cfg.ItemsMap["key2"].Count)
	}
}

// TestSetDefaults_SliceString verifies that string slices are initialized
// correctly when declared with comma-separated tag literals.
func TestSetDefaults_SliceString(t *testing.T) {
	type TLS struct {
		Alpn []string `yaml:"alpn" default:"h2,http/1.1" validate:"choice=h2,http/1.1"`
	}

	type AppConfig struct {
		Crypto TLS `yaml:"crypto"`
	}

	cfg := AppConfig{}

	if err := SetDefaults(&cfg); err != nil {
		t.Fatalf("unexpected error in SetDefaults: %v", err)
	}

	if len(cfg.Crypto.Alpn) != 2 {
		t.Fatalf("expected Alpn slice to have 2 elements, got %d", len(cfg.Crypto.Alpn))
	}

	if cfg.Crypto.Alpn[0] != "h2" || cfg.Crypto.Alpn[1] != "http/1.1" {
		t.Errorf("unexpected slice elements: %v", cfg.Crypto.Alpn)
	}

	if err := Validate(&cfg); err != nil {
		t.Fatalf("unexpected validation error: %v", err)
	}
}

// TestSetDefaults_EnvPrecedence verifies that active operating system environment
// variables override configured structural default tags but do not clobber fields
// explicitly unmarshaled from structural configurations.
func TestSetDefaults_EnvPrecedence(t *testing.T) {
	type EnvConfig struct {
		Host string `yaml:"host" default:"localhost" env:"APP_HOST"`
		Port int    `yaml:"port" default:"8080" env:"APP_PORT"`
	}

	_ = os.Setenv("APP_HOST", "10.0.0.1")
	defer func() { _ = os.Unsetenv("APP_HOST") }()

	cfg := EnvConfig{
		Port: 9090,
	}

	if err := SetDefaults(&cfg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Host != "10.0.0.1" {
		t.Errorf("expected Host to be '10.0.0.1' from env, got %q", cfg.Host)
	}

	if cfg.Port != 9090 {
		t.Errorf("expected Port to remain 9090, got %d", cfg.Port)
	}
}

// TestEnv_InvalidLiteralReported verifies the strict environment conversion
// contract: an env literal that cannot be converted to the target
// scalar type is reported as an error naming the field and the offending
// value instead of silently leaving the field zero and suppressing its
// default. Before the fix, Load returned nil and Port stayed 0.
func TestEnv_InvalidLiteralReported(t *testing.T) {
	type Config struct {
		Port int `yaml:"port" default:"8080" env:"TEST_ENGBAD_PORT"`
	}

	t.Setenv("TEST_ENGBAD_PORT", "abc")

	var cfg Config
	err := Load([]byte("unused: true\n"), &cfg)
	if err == nil {
		t.Fatal("expected invalid env int literal to be reported, got nil")
	}

	if !strings.Contains(err.Error(), `invalid int "abc" for field Port`) {
		t.Errorf("unexpected error text: %v", err)
	}
}

// TestEnv_InvalidLiteralsPerKind walks the strict conversion matrix of the
// environment pass: an invalid bool, uint, float or duration literal is
// reported as an error naming the field and the value. Before the fix, bool,
// uint and float were silently swallowed (duration was already
// strict and stays as a regression guard).
func TestEnv_InvalidLiteralsPerKind(t *testing.T) {
	type Config struct {
		Enabled bool          `yaml:"enabled" default:"false" env:"TEST_ENGBAD_ENABLED"`
		Quota   uint          `yaml:"quota" default:"10" env:"TEST_ENGBAD_QUOTA"`
		Ratio   float64       `yaml:"ratio" default:"1.5" env:"TEST_ENGBAD_RATIO"`
		Timeout time.Duration `yaml:"timeout" default:"5s" env:"TEST_ENGBAD_TIMEOUT"`
	}

	tests := []struct {
		name  string
		env   string
		value string
		want  string
	}{
		{"invalid bool", "TEST_ENGBAD_ENABLED", "maybe", `invalid bool "maybe" for field Enabled`},
		{"invalid uint", "TEST_ENGBAD_QUOTA", "-3", `invalid uint "-3" for field Quota`},
		{"invalid float", "TEST_ENGBAD_RATIO", "3.5.5", `invalid float "3.5.5" for field Ratio`},
		{"invalid duration", "TEST_ENGBAD_TIMEOUT", "5x", `invalid duration "5x" for field Timeout`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(tt.env, tt.value)

			var cfg Config
			err := Load([]byte("unused: true\n"), &cfg)
			if err == nil {
				t.Fatal("expected invalid env literal to be reported, got nil")
			}

			if !strings.Contains(err.Error(), tt.want) {
				t.Errorf("expected error %q, got: %v", tt.want, err)
			}
		})
	}
}

// TestEnv_ValidLiteralsApplied verifies the positive side of the strict env
// matrix: valid bool, int, uint, float and duration literals are applied in
// full, overriding tag defaults. Before the fix, the float64 field
// had no conversion case at all — the env value was silently dropped and the
// field stayed zero.
func TestEnv_ValidLiteralsApplied(t *testing.T) {
	type Config struct {
		Enabled bool          `yaml:"enabled" default:"false" env:"TEST_ENVLIT_ENABLED"`
		Workers int           `yaml:"workers" default:"2" env:"TEST_ENVLIT_WORKERS"`
		Quota   uint          `yaml:"quota" default:"10" env:"TEST_ENVLIT_QUOTA"`
		Ratio   float64       `yaml:"ratio" default:"1.5" env:"TEST_ENVLIT_RATIO"`
		Timeout time.Duration `yaml:"timeout" default:"5s" env:"TEST_ENVLIT_TIMEOUT"`
	}

	t.Setenv("TEST_ENVLIT_ENABLED", "true")
	t.Setenv("TEST_ENVLIT_WORKERS", "4")
	t.Setenv("TEST_ENVLIT_QUOTA", "7")
	t.Setenv("TEST_ENVLIT_RATIO", "3.5")
	t.Setenv("TEST_ENVLIT_TIMEOUT", "2s")

	var cfg Config
	if err := Load([]byte("unused: true\n"), &cfg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := Config{Enabled: true, Workers: 4, Quota: 7, Ratio: 3.5, Timeout: 2 * time.Second}
	if cfg != want {
		t.Errorf("env literals not applied:\n got: %+v\nwant: %+v", cfg, want)
	}
}

// TestValidate_NewConstraints evaluates the extended validation suite covering
// Unicode rune string lengths, structural collection capacities, and rule safety.
func TestValidate_NewConstraints(t *testing.T) {
	type ValidExtendedConfig struct {
		Username string            `yaml:"username" validate:"minlen=3,maxlen=10"`
		Tags     []string          `yaml:"tags" validate:"mincount=2,maxcount=4"`
		Metadata map[string]string `yaml:"metadata" validate:"mincount=1,maxcount=2"`
	}

	validCfg := ValidExtendedConfig{
		Username: "cybervask",
		Tags:     []string{"go", "yaml", "test"},
		Metadata: map[string]string{"env": "prod"},
	}

	if err := Validate(&validCfg); err != nil {
		t.Fatalf("expected clean validation run, got: %v", err)
	}

	tests := []struct {
		name      string
		modify    func(*ValidExtendedConfig)
		errSubstr string
	}{
		{
			name: "string length too short (minlen)",
			modify: func(c *ValidExtendedConfig) {
				c.Username = "go"
			},
			errSubstr: "less than minlen",
		},
		{
			name: "string length too long (maxlen)",
			modify: func(c *ValidExtendedConfig) {
				c.Username = "verylongusername"
			},
			errSubstr: "exceeds maxlen",
		},
		{
			name: "collection too small (mincount)",
			modify: func(c *ValidExtendedConfig) {
				c.Tags = []string{"go"}
			},
			errSubstr: "less than mincount",
		},
		{
			name: "collection too large (maxcount)",
			modify: func(c *ValidExtendedConfig) {
				c.Tags = []string{"1", "2", "3", "4", "5"}
			},
			errSubstr: "exceeds maxcount",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := ValidExtendedConfig{
				Username: "cybervask",
				Tags:     []string{"go", "yaml", "test"},
				Metadata: map[string]string{"env": "prod"},
			}
			tt.modify(&cfg)
			err := Validate(&cfg)
			if err == nil {
				t.Fatal("expected constraint error, got nil")
			}
			if !strings.Contains(err.Error(), tt.errSubstr) {
				t.Errorf("expected error string matching %q, got: %q", tt.errSubstr, err.Error())
			}
		})
	}

	t.Run("invalid validator configuration cases", func(t *testing.T) {
		type BadStringConfig struct {
			Field string `validate:"minlen=10,maxlen=5"`
		}
		var badStr BadStringConfig
		badStr.Field = "test"
		if err := Validate(&badStr); err == nil || !strings.Contains(err.Error(), "invalid validator configuration") {
			t.Errorf("expected startup config anomaly failure, got: %v", err)
		}

		type BadCollectionConfig struct {
			Field []string `validate:"mincount=5,maxcount=2"`
		}
		var badColl BadCollectionConfig
		badColl.Field = []string{"1"}
		if err := Validate(&badColl); err == nil || !strings.Contains(err.Error(), "invalid validator configuration") {
			t.Errorf("expected structural capacity anomaly failure, got: %v", err)
		}
	})
}

// TestValidate_ExtendedFeatures tests advanced architectural properties including
// error aggregation, cross-field dependency validation, duration limits, and data formats.
func TestValidate_ExtendedFeatures(t *testing.T) {
	// 1. Verify safe Unicode Rune Counting (multibyte characters).
	type RuneConfig struct {
		Word string `yaml:"word" validate:"minlen=5,maxlen=5"`
	}
	rc := RuneConfig{Word: "Привет"} // 6 runes (12 bytes in UTF-8), should fail maxlen=5.
	if err := Validate(&rc); err == nil || !strings.Contains(err.Error(), "exceeds maxlen") {
		t.Errorf("expected rune counting boundary overflow failure, got: %v", err)
	}
	rc.Word = "Прив" // 4 runes, should fail minlen=5.
	if err := Validate(&rc); err == nil || !strings.Contains(err.Error(), "less than minlen") {
		t.Errorf("expected rune counting boundary underflow failure, got: %v", err)
	}

	// 2. Verify numerical parsing bounds tailored for time.Duration metrics.
	type DurationConfig struct {
		Interval time.Duration `yaml:"interval" validate:"min=1s,max=10m"`
	}
	dc := DurationConfig{Interval: 500 * time.Millisecond} // Below 1s lower limit.
	if err := Validate(&dc); err == nil || !strings.Contains(err.Error(), "< min") {
		t.Errorf("expected duration min boundary constraint error, got: %v", err)
	}
	dc.Interval = 11 * time.Minute // Above 10m upper limit.
	if err := Validate(&dc); err == nil || !strings.Contains(err.Error(), "> max") {
		t.Errorf("expected duration max boundary constraint error, got: %v", err)
	}

	// 3. Verify standard data formatting constraints (IP variants and unique IDs).
	type FormatConfig struct {
		AnyIP string `yaml:"any_ip" validate:"format=ip"`
		V4    string `yaml:"v4" validate:"format=ipv4"`
		V6    string `yaml:"v6" validate:"format=ipv6"`
		ID    string `yaml:"id" validate:"format=uuid"`
		Hex   string `yaml:"hex" validate:"format=hex"`
	}
	fc := FormatConfig{AnyIP: "1.2.3.4", V4: "127.0.0.1", V6: "::1", ID: "123e4567-e89b-12d3-a456-426614174000", Hex: "deadBEEF"}
	if err := Validate(&fc); err != nil {
		t.Fatalf("valid networking formats, UUID strings and hex values must not produce errors, got: %v", err)
	}

	// An empty string is a valid (zero-length) hex value.
	fc.Hex = ""
	if err := Validate(&fc); err != nil {
		t.Fatalf("empty hex value must pass, got: %v", err)
	}

	fc.V4 = "256.0.0.1" // Out-of-bounds IPv4 address layout.
	if err := Validate(&fc); err == nil || !strings.Contains(err.Error(), "is not a valid IPv4 address") {
		t.Errorf("expected strict IPv4 layout evaluation failure, got: %v", err)
	}

	fc.V4 = "127.0.0.1"
	fc.Hex = "abc" // Odd number of digits.
	if err := Validate(&fc); err == nil || !strings.Contains(err.Error(), "odd number of digits") {
		t.Errorf("expected odd-length hex failure, got: %v", err)
	}

	fc.Hex = "xyz0" // Non-hex characters with even length.
	if err := Validate(&fc); err == nil || !strings.Contains(err.Error(), "is not valid hex") {
		t.Errorf("expected invalid hex characters failure, got: %v", err)
	}

	// 4. Verify conditional cross-field checking constraints (required_if) with negation formatting.
	type RequiredIfConfig struct {
		Mode        string `yaml:"mode"`
		Token       string `yaml:"token" validate:"required_if=Mode:prod"`
		Webhook     string `yaml:"webhook" validate:"required_if=Token:not_empty"`
		BackupRoute string `yaml:"backup" validate:"required_if=Webhook:empty"`
	}

	ric := RequiredIfConfig{Mode: "prod", Token: ""} // Violates condition since Mode=prod requires Token.
	if err := Validate(&ric); err == nil || !strings.Contains(err.Error(), "is required when field mode=prod") {
		t.Errorf("expected cross-field dependency validation error, got: %v", err)
	}

	ric = RequiredIfConfig{Mode: "dev", Token: "abc", Webhook: ""} // Token is filled -> Webhook becomes required.
	if err := Validate(&ric); err == nil || !strings.Contains(err.Error(), "field webhook:") {
		t.Errorf("expected cross-field non-empty macro requirement error, got: %v", err)
	}

	// 5. Verify the Error Aggregation engine behavior.
	type AggregatedConfig struct {
		Age  int    `yaml:"age" validate:"min=18"`
		Code string `yaml:"code" validate:"minlen=5"`
	}
	ac := AggregatedConfig{Age: 10, Code: "go"} // Triggers 2 separate validation failures simultaneously.
	err := Validate(&ac)
	if err == nil {
		t.Fatal("expected composite error tracking entity return, got nil")
	}

	errStr := err.Error()
	if !strings.Contains(errStr, "age:") || !strings.Contains(errStr, "code:") {
		t.Errorf("expected multi-line aggregated error message payload stack, got:\n%s", errStr)
	}

	// 6. Verify negation operator "!" inside conditional cross-field checking properties.
	type NegationRequiredIfConfig struct {
		Role   string `yaml:"role"`
		Secret string `yaml:"secret" validate:"required_if=Role:!observer"`
	}

	nc := NegationRequiredIfConfig{Role: "admin", Secret: ""}
	errNeg := Validate(&nc)
	if errNeg == nil || !strings.Contains(errNeg.Error(), "is required when field role=admin") {
		t.Errorf("expected negative cross-field dependency validation error, got: %v", errNeg)
	}

	ncSuccess := NegationRequiredIfConfig{Role: "observer", Secret: ""}
	if errNegSucc := Validate(&ncSuccess); errNegSucc != nil {
		t.Errorf("negated status match should bypass mandatory checking, got error: %v", errNegSucc)
	}
}

// TestValidate_RequiredIfTargetResolution verifies required_if target lookup:
// bare names resolve against the enclosing struct first (sibling priority)
// with a root fallback, dot paths address the root, and unresolvable targets
// produce configuration errors instead of silent skips.
func TestValidate_RequiredIfTargetResolution(t *testing.T) {
	// 1. Sibling target: the nested auth.mode field triggers auth.token even
	// though the root "mode" field holds a non-matching value.
	type siblingAuth struct {
		Mode  string `yaml:"mode"`
		Token string `yaml:"token" validate:"required_if=mode:secret"`
	}
	type siblingConfig struct {
		Mode string      `yaml:"mode"`
		Auth siblingAuth `yaml:"auth"`
	}

	sibling := siblingConfig{Mode: "open", Auth: siblingAuth{Mode: "secret"}}
	if err := Validate(&sibling); err == nil || !strings.Contains(err.Error(), "field auth.token: is required when field mode=secret") {
		t.Errorf("expected sibling target inside nested struct to trigger, got: %v", err)
	}

	sibling.Auth.Token = "signed"
	if err := Validate(&sibling); err != nil {
		t.Errorf("satisfied sibling condition with filled token must pass, got: %v", err)
	}

	// 2. Sibling priority: when the name exists both in the enclosing struct
	// and in the root, the sibling (nearest) target wins in both directions.
	type priorityAuth struct {
		Mode  string `yaml:"mode"`
		Token string `yaml:"token" validate:"required_if=mode:secret"`
	}
	type priorityConfig struct {
		Mode string       `yaml:"mode"`
		Auth priorityAuth `yaml:"auth"`
	}

	priority := priorityConfig{Mode: "secret", Auth: priorityAuth{Mode: "open"}}
	if err := Validate(&priority); err != nil {
		t.Errorf("sibling target must shadow the identically named root one, got: %v", err)
	}

	priority.Auth.Mode = "secret"
	priority.Mode = "open"
	if err := Validate(&priority); err == nil || !strings.Contains(err.Error(), "when field mode=secret") {
		t.Errorf("sibling target must be preferred over the identically named root one, got: %v", err)
	}

	// 3. Root fallback: a nested field may reference a top-level target that
	// is absent from its own struct (examples/05_required_if compatibility).
	type fallbackConfig struct {
		Role  string `yaml:"role"`
		Inner struct {
			Token string `yaml:"token" validate:"required_if=role:admin"`
		} `yaml:"inner"`
	}

	fallback := fallbackConfig{Role: "admin"}
	if err := Validate(&fallback); err == nil || !strings.Contains(err.Error(), "field inner.token: is required when field role=admin") {
		t.Errorf("expected nested field to fall back to the root target, got: %v", err)
	}

	fallback.Role = "user"
	if err := Validate(&fallback); err != nil {
		t.Errorf("unmet root fallback condition must pass, got: %v", err)
	}

	// 4. Dot path: a top-level field may reference a nested target through
	// the root-addressed dot-path syntax.
	type dotPathConfig struct {
		Server struct {
			Mode string `yaml:"mode"`
		} `yaml:"server"`
		Token string `yaml:"token" validate:"required_if=server.mode:prod"`
	}

	dot := dotPathConfig{}
	dot.Server.Mode = "prod"
	if err := Validate(&dot); err == nil || !strings.Contains(err.Error(), "field token: is required when field server.mode=prod") {
		t.Errorf("expected dot-path target resolution from the root, got: %v", err)
	}

	dot.Server.Mode = "dev"
	if err := Validate(&dot); err != nil {
		t.Errorf("unmet dot-path condition must pass, got: %v", err)
	}

	// 5. Unknown bare target: a missing target name is a configuration error,
	// not a silent skip.
	type unknownTargetConfig struct {
		Token string `yaml:"token" validate:"required_if=nonexistent:prod"`
	}

	unknown := unknownTargetConfig{Token: ""}
	if err := Validate(&unknown); err == nil || !strings.Contains(err.Error(), `invalid required_if configuration: target field "nonexistent" not found`) {
		t.Errorf("expected configuration error for unknown bare target, got: %v", err)
	}

	// 6. Unknown dot path: an unresolvable path target reports the full path
	// as a configuration error.
	type unknownPathConfig struct {
		Token string `yaml:"token" validate:"required_if=server.mode:prod"`
	}

	unknownPath := unknownPathConfig{Token: ""}
	if err := Validate(&unknownPath); err == nil || !strings.Contains(err.Error(), `invalid required_if configuration: target field "server.mode" not found`) {
		t.Errorf("expected configuration error for unknown dot-path target, got: %v", err)
	}
}

// TestValidate_RequiredIfOperators verifies the canonical required_if
// condition operators: "==" and "!=" do case-insensitive value comparison,
// "~=" and "^=" match and anti-match regexp patterns (quoted patterns are
// unquoted like regexp rule values), invalid patterns are configuration
// errors independent of the field value, and the deprecated colon form keeps
// its legacy parsing when the operand embeds canonical operators.
func TestValidate_RequiredIfOperators(t *testing.T) {
	// 1. "==": case-insensitive equality triggers on any casing.
	type eqConfig struct {
		Role  string `yaml:"role"`
		Token string `yaml:"token" validate:"required_if=role==prod"`
	}

	eq := eqConfig{Role: "PROD"}
	if err := Validate(&eq); err == nil || !strings.Contains(err.Error(), "is required when field role==PROD") {
		t.Errorf("expected case-insensitive == condition to trigger, got: %v", err)
	}

	eq.Role = "dev"
	if err := Validate(&eq); err != nil {
		t.Errorf("unmet == condition must pass, got: %v", err)
	}

	// 2. Quoted == operand: surrounding quotes are stripped like in the
	// regexp rule, so 'prod' compares as prod.
	type quotedConfig struct {
		Role  string `yaml:"role"`
		Token string `yaml:"token" validate:"required_if=role=='prod'"`
	}

	quoted := quotedConfig{Role: "prod"}
	if err := Validate(&quoted); err == nil || !strings.Contains(err.Error(), "is required when field role==prod") {
		t.Errorf("expected quoted == operand to be unquoted, got: %v", err)
	}

	// 3. "!=": strict inequality triggers on any non-matching value.
	type neqConfig struct {
		Role  string `yaml:"role"`
		Token string `yaml:"token" validate:"required_if=role!=observer"`
	}

	neq := neqConfig{Role: "admin"}
	if err := Validate(&neq); err == nil || !strings.Contains(err.Error(), "is required when field role!=admin") {
		t.Errorf("expected != condition to trigger, got: %v", err)
	}

	neq.Role = "observer"
	if err := Validate(&neq); err != nil {
		t.Errorf("met != condition must pass, got: %v", err)
	}

	// 4. "~=": regexp match triggers on a matching target value.
	type matchConfig struct {
		Role  string `yaml:"role"`
		Token string `yaml:"token" validate:"required_if=role~='^(prod|dev)$'"`
	}

	match := matchConfig{Role: "dev"}
	if err := Validate(&match); err == nil || !strings.Contains(err.Error(), "is required when field role~=^(prod|dev)$") {
		t.Errorf("expected ~= condition to trigger, got: %v", err)
	}

	match.Role = "stage"
	if err := Validate(&match); err != nil {
		t.Errorf("non-matching ~= condition must pass, got: %v", err)
	}

	// 5. "^=": negative regexp match triggers on a non-matching value.
	type antiMatchConfig struct {
		Role  string `yaml:"role"`
		Token string `yaml:"token" validate:"required_if=role^=^prod"`
	}

	anti := antiMatchConfig{Role: "dev"}
	if err := Validate(&anti); err == nil || !strings.Contains(err.Error(), "is required when field role^=^prod") {
		t.Errorf("expected ^= anti-match condition to trigger, got: %v", err)
	}

	anti.Role = "production"
	if err := Validate(&anti); err != nil {
		t.Errorf("matched ^= condition must pass, got: %v", err)
	}

	// 6. Invalid regexp: a configuration error regardless of the field value,
	// including a fully filled (non-empty) operand field.
	type badPatternConfig struct {
		Role  string `yaml:"role"`
		Token string `yaml:"token" validate:"required_if=role~=["`
	}

	badZero := badPatternConfig{Role: "dev"}
	if err := Validate(&badZero); err == nil || !strings.Contains(err.Error(), "invalid required_if configuration: invalid regular expression") {
		t.Errorf("expected configuration error for invalid ~= pattern, got: %v", err)
	}

	badFilled := badPatternConfig{Role: "dev", Token: "signed"}
	if err := Validate(&badFilled); err == nil || !strings.Contains(err.Error(), "invalid required_if configuration: invalid regular expression") {
		t.Errorf("expected invalid ~= pattern error even on a filled field, got: %v", err)
	}

	// 7. Canonical operator with a dot-path target.
	type dotPathConfig struct {
		Server struct {
			Mode string `yaml:"mode"`
		} `yaml:"server"`
		Token string `yaml:"token" validate:"required_if=server.mode==prod"`
	}

	dot := dotPathConfig{}
	dot.Server.Mode = "prod"
	if err := Validate(&dot); err == nil || !strings.Contains(err.Error(), "is required when field server.mode==prod") {
		t.Errorf("expected dot-path target resolution with the == operator, got: %v", err)
	}

	// 8. Operator-less condition: a configuration error regardless of the
	// field value (malformed required_if values are never silently
	// skipped, same as unknown targets and invalid regexps).
	type noOpConfig struct {
		Token string `yaml:"token" validate:"required_if=nooperator"`
	}

	noOpErr := Validate(&noOpConfig{})
	if noOpErr == nil || !strings.Contains(noOpErr.Error(), `invalid required_if configuration: missing comparison operator in "nooperator" (expected ==, !=, ~=, ^= or deprecated key:value)`) {
		t.Errorf("expected configuration error for operator-less condition, got: %v", noOpErr)
	}

	// 8a. Empty required_if value: the same malformed class, the same error.
	type emptyReqIfConfig struct {
		Token string `yaml:"token" validate:"required_if="`
	}

	if err := Validate(&emptyReqIfConfig{}); err == nil || !strings.Contains(err.Error(), `invalid required_if configuration: missing comparison operator in ""`) {
		t.Errorf("expected configuration error for empty required_if value, got: %v", err)
	}

	// 8b. Operator-less condition on a zero-value field: the configuration
	// error fires independently of the field value.
	type noOpZeroConfig struct {
		Role  string `yaml:"role"`
		Token string `yaml:"token" validate:"required_if=role"`
	}

	if err := Validate(&noOpZeroConfig{}); err == nil || !strings.Contains(err.Error(), `invalid required_if configuration: missing comparison operator in "role"`) {
		t.Errorf("expected configuration error on a zero-value field, got: %v", err)
	}

	filled := noOpZeroConfig{Token: "signed"}
	if err := Validate(&filled); err == nil || !strings.Contains(err.Error(), `invalid required_if configuration: missing comparison operator in "role"`) {
		t.Errorf("expected configuration error to be value-independent, got: %v", err)
	}

	// 9. Leftmost separator rule: a colon form whose operand embeds canonical
	// operators keeps the legacy parsing and semantics.
	type legacyOperandConfig struct {
		Mode  string `yaml:"mode"`
		Token string `yaml:"token" validate:"required_if=mode:secret==x"`
	}

	legacy := legacyOperandConfig{Mode: "secret==x"}
	if err := Validate(&legacy); err == nil || !strings.Contains(err.Error(), "is required when field mode=secret==x") {
		t.Errorf("expected legacy colon form to win over embedded canonical operators, got: %v", err)
	}
}

// TestValidate_RequiredIfTargetYamlNameFallback verifies that the required_if
// error message reports the target's yaml name as returned by findStructTarget:
// for a target field without a yaml tag the reported name is the lowercased
// Go field name (ToLower fallback), not the name written in the condition.
func TestValidate_RequiredIfTargetYamlNameFallback(t *testing.T) {
	type fallbackNameConfig struct {
		Mode  string // no yaml tag: the message must report the lowercased Go name
		Token string `yaml:"token" validate:"required_if=Mode==prod"`
	}

	cfg := fallbackNameConfig{Mode: "prod"}
	err := Validate(&cfg)
	if err == nil || !strings.Contains(err.Error(), "is required when field mode==prod") {
		t.Errorf("expected required_if error to report the lowercased yaml name of the tagless target, got: %v", err)
	}

	cfg.Token = "signed"
	if err := Validate(&cfg); err != nil {
		t.Errorf("satisfied condition with filled token must pass, got: %v", err)
	}
}

// TestChoiceWithCommas verifies that the choice rule correctly handles whitelist
// tokens that embed literal commas inside quoted segments.
func TestChoiceWithCommas(t *testing.T) {
	type Config struct {
		Order string `yaml:"order" validate:"choice='allow,deny','deny,allow'"`
	}

	tests := []struct {
		name    string
		value   string
		wantErr bool
	}{
		{"valid: allow,deny", "allow,deny", false},
		{"valid: deny,allow", "deny,allow", false},
		{"invalid: allow", "allow", true},
		{"invalid: deny", "deny", true},
		{"invalid: empty", "", true}, // ← Теперь будет работать!
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var cfg Config
			cfg.Order = tt.value
			err := Validate(&cfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestChoiceWithQuotesAndSpaces checks that quoted choice tokens preserve
// inner spaces during parsing and comparison.
func TestChoiceWithQuotesAndSpaces(t *testing.T) {
	type Config struct {
		Mode string `yaml:"mode" validate:"choice='read,write', 'read-only', 'write-only'"`
	}

	var cfg Config
	cfg.Mode = "read,write"
	err := Validate(&cfg)
	if err != nil {
		t.Errorf("Expected valid, got error: %v", err)
	}

	cfg.Mode = "read-only"
	err = Validate(&cfg)
	if err != nil {
		t.Errorf("Expected valid, got error: %v", err)
	}
}

// TestChoiceBlacklistWithCommas verifies that blacklist (!-prefixed) choice
// tokens containing embedded commas are correctly forbidden.
func TestChoiceBlacklistWithCommas(t *testing.T) {
	type Config struct {
		Action string `yaml:"action" validate:"choice='!allow,deny','!deny,allow'"`
	}

	var cfg Config
	cfg.Action = "allow,deny"
	err := Validate(&cfg)
	if err == nil {
		t.Error("Expected error for blacklisted value")
	}

	cfg.Action = "other"
	err = Validate(&cfg)
	if err != nil {
		t.Errorf("Expected valid, got error: %v", err)
	}
}

// TestValidateTagParsing validates the low-level validate tag tokenizer against
// plain, quoted, and mixed rule combinations.
func TestValidateTagParsing(t *testing.T) {
	tests := []struct {
		name     string
		tag      string
		expected map[string]string
	}{
		{
			name: "simple rules",
			tag:  "not_empty,min=5,max=10",
			expected: map[string]string{
				"not_empty": "",
				"min":       "5",
				"max":       "10",
			},
		},
		{
			name: "choice with commas in quotes",
			tag:  "choice='allow,deny','deny,allow'",
			expected: map[string]string{
				"choice": "'allow,deny','deny,allow'",
			},
		},
		{
			name: "mixed rules with quotes",
			tag:  "not_empty,choice='a,b','c,d',minlen=3",
			expected: map[string]string{
				"not_empty": "",
				"choice":    "'a,b','c,d'",
				"minlen":    "3",
			},
		},
		{
			name: "double quotes support",
			tag:  `choice="val,ue","other,val"`,
			expected: map[string]string{
				"choice": `"val,ue","other,val"`,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseValidateTag(tt.tag)
			if len(result) != len(tt.expected) {
				t.Errorf("Expected %d rules, got %d: %+v", len(tt.expected), len(result), result)
				return
			}
			for k, v := range tt.expected {
				if result[k] != v {
					t.Errorf("Rule %q: expected %q, got %q", k, v, result[k])
				}
			}
		})
	}
}

// TestValidate_NilPointerFieldRules verifies that required, not_empty and
// required_if rules keep firing on nil pointer fields instead of being
// silently skipped.
func TestValidate_NilPointerFieldRules(t *testing.T) {
	type NilPointerConfig struct {
		Name *string `yaml:"name" validate:"required,not_empty"`
	}

	nilCfg := NilPointerConfig{}
	err := Validate(&nilCfg)
	if err == nil {
		t.Fatal("expected required/not_empty violation on nil pointer field, got nil")
	}
	if !strings.Contains(err.Error(), "field name: is empty, but required") {
		t.Errorf("expected 'is empty, but required' error, got: %v", err)
	}

	// A filled pointer field must stay valid.
	filled := "value"
	filledCfg := NilPointerConfig{Name: &filled}
	if err := Validate(&filledCfg); err != nil {
		t.Errorf("expected filled pointer field to pass, got: %v", err)
	}
}

// TestValidate_NilPointerRequiredIf verifies that required_if fires on a nil
// pointer field when the condition is met and stays silent otherwise.
func TestValidate_NilPointerRequiredIf(t *testing.T) {
	type NilPointerRequiredIfConfig struct {
		Role string  `yaml:"role"`
		Addr *string `yaml:"addr" validate:"required_if=role:client"`
	}

	violation := NilPointerRequiredIfConfig{Role: "client"}
	err := Validate(&violation)
	if err == nil {
		t.Fatal("expected required_if violation on nil pointer field, got nil")
	}
	if !strings.Contains(err.Error(), "field addr: is required when field role=client") {
		t.Errorf("expected 'is required when ...' error, got: %v", err)
	}

	// The condition is not met: the nil pointer field stays allowed.
	other := NilPointerRequiredIfConfig{Role: "server"}
	if err := Validate(&other); err != nil {
		t.Errorf("expected nil pointer field to pass when condition is not met, got: %v", err)
	}
}

// TestValidate_ValueFieldOwnRules verifies that required_if declared directly
// on a field named Value is no longer dropped by the legacy wrapper fallback.
func TestValidate_ValueFieldOwnRules(t *testing.T) {
	type ValueRuleConfig struct {
		Role  string `yaml:"role"`
		Value string `yaml:"value" validate:"required_if=role:client"`
	}

	cfg := ValueRuleConfig{Role: "client", Value: ""}
	err := Validate(&cfg)
	if err == nil {
		t.Fatal("expected required_if declared on the Value field itself to fire, got nil")
	}
	if !strings.Contains(err.Error(), "is required when field role=client") {
		t.Errorf("expected 'is required when ...' error, got: %v", err)
	}

	ok := ValueRuleConfig{Role: "client", Value: "set"}
	if err := Validate(&ok); err != nil {
		t.Errorf("expected filled Value field to pass, got: %v", err)
	}
}

// TestValidate_ValueFieldTransparentParentRules verifies that the legacy
// generic Value property fallback keeps applying parent rules to the wrapper
// payload under the parent path.
func TestValidate_ValueFieldTransparentParentRules(t *testing.T) {
	type ValueWrapper struct {
		Value string `yaml:"value"`
	}

	type ValueWrapperConfig struct {
		Items []ValueWrapper `yaml:"items" validate:"minlen=3"`
	}

	cfg := ValueWrapperConfig{Items: []ValueWrapper{{Value: "ab"}}}
	err := Validate(&cfg)
	if err == nil {
		t.Fatal("expected parent minlen rule to reach the Value payload, got nil")
	}
	// The error must carry the parent path, not an extra ".value" segment.
	if !strings.Contains(err.Error(), "field items[0]: string length 2 is less than minlen 3") {
		t.Errorf("expected transparent parent-rule evaluation on the payload, got: %v", err)
	}

	ok := ValueWrapperConfig{Items: []ValueWrapper{{Value: "abc"}}}
	if err := Validate(&ok); err != nil {
		t.Errorf("expected payload long enough for parent minlen to pass, got: %v", err)
	}
}

// TestValidate_RequiredIfCaseInsensitive verifies that required_if compares
// the target value case-insensitively, so a YAML "Client" satisfies a
// role:client rule.
func TestValidate_RequiredIfCaseInsensitive(t *testing.T) {
	type CaseInsensitiveConfig struct {
		Role  string `yaml:"role"`
		Token string `yaml:"token" validate:"required_if=role:client"`
	}

	cfg := CaseInsensitiveConfig{Role: "Client", Token: ""}
	err := Validate(&cfg)
	if err == nil {
		t.Fatal("expected case-insensitive condition match to trigger required_if, got nil")
	}
	if !strings.Contains(err.Error(), "field token: is required when field role=Client") {
		t.Errorf("expected 'is required when ...' error, got: %v", err)
	}

	// An unrelated role still leaves the field optional.
	other := CaseInsensitiveConfig{Role: "server"}
	if err := Validate(&other); err != nil {
		t.Errorf("expected field to pass when condition is not met, got: %v", err)
	}
}

// TestValidate_RequiredIfPointerTarget verifies pointer target handling in
// required_if: a nil pointer target never matches a literal token, a non-nil
// one is compared by its dereferenced value, and the empty macro keeps its
// IsZero semantics on nil pointers.
func TestValidate_RequiredIfPointerTarget(t *testing.T) {
	type PointerTargetConfig struct {
		Role  *string `yaml:"role"`
		Token string  `yaml:"token" validate:"required_if=role:prod"`
	}

	// Nil pointer target: the condition is not met, no error expected.
	nilTarget := PointerTargetConfig{}
	if err := Validate(&nilTarget); err != nil {
		t.Errorf("expected nil pointer target not to match a literal token, got: %v", err)
	}

	// Non-nil pointer target: compared by the dereferenced element value.
	role := "prod"
	deref := PointerTargetConfig{Role: &role}
	err := Validate(&deref)
	if err == nil {
		t.Fatal("expected dereferenced pointer target to trigger required_if, got nil")
	}
	if !strings.Contains(err.Error(), "field token: is required when field role=prod") {
		t.Errorf("expected 'is required when ...' error, got: %v", err)
	}
}

// TestValidate_RegexpQuotedPatternValidValue verifies that a single-quoted
// regexp pattern has its surrounding quotes stripped before compilation, so a
// plain value matches without receiving a false rejection.
func TestValidate_RegexpQuotedPatternValidValue(t *testing.T) {
	type QuotedPatternConfig struct {
		Zone string `yaml:"zone" validate:"regexp='^[a-z]+$'"`
	}

	cfg := QuotedPatternConfig{Zone: "abc"}
	if err := Validate(&cfg); err != nil {
		t.Errorf("expected quoted pattern to accept a plain value, got: %v", err)
	}
}

// TestValidate_RegexpDoubleQuotedPattern verifies that a double-quoted regexp
// pattern is unwrapped the same way as a single-quoted one.
func TestValidate_RegexpDoubleQuotedPattern(t *testing.T) {
	type DoubleQuotedConfig struct {
		Zone string `yaml:"zone" validate:"regexp=\"^[a-z]{2,4}$\""`
	}

	cfg := DoubleQuotedConfig{Zone: "dev"}
	if err := Validate(&cfg); err != nil {
		t.Errorf("expected double-quoted pattern to accept a plain value, got: %v", err)
	}
}

// TestValidate_RegexpQuotedPatternWithComma verifies that quotes protect an
// embedded comma inside a regexp pattern and the quotes themselves are
// stripped, so a comma-separated value matches.
func TestValidate_RegexpQuotedPatternWithComma(t *testing.T) {
	type QuotedCommaConfig struct {
		List string `yaml:"list" validate:"regexp='^\\d{1,3}(,\\d{1,3})*$'"`
	}

	cfg := QuotedCommaConfig{List: "1,2,3"}
	if err := Validate(&cfg); err != nil {
		t.Errorf("expected quoted comma pattern to accept %q, got: %v", "1,2,3", err)
	}
}

// TestValidate_RegexpQuotedPatternWithOppositeQuotesInside verifies the
// documented convention for matching a literal quote: wrap the pattern in an
// opposite quote type, so the stripped pattern still contains the quote char.
func TestValidate_RegexpQuotedPatternWithOppositeQuotesInside(t *testing.T) {
	type OppositeQuotesConfig struct {
		Code string `yaml:"code" validate:"regexp=\"^'x'$\""`
	}

	cfg := OppositeQuotesConfig{Code: "'x'"}
	if err := Validate(&cfg); err != nil {
		t.Errorf("expected opposite-quote pattern to match a quoted value, got: %v", err)
	}
}

// TestValidate_RegexpUnquotedPattern guards the regression: an unquoted
// pattern keeps its exact pre-existing behavior (matches and rejects values).
func TestValidate_RegexpUnquotedPattern(t *testing.T) {
	type UnquotedConfig struct {
		Zone string `yaml:"zone" validate:"regexp=^[a-z]{2,4}$"`
	}

	ok := UnquotedConfig{Zone: "dev"}
	if err := Validate(&ok); err != nil {
		t.Errorf("expected unquoted pattern to accept %q, got: %v", "dev", err)
	}

	bad := UnquotedConfig{Zone: "DEV"}
	err := Validate(&bad)
	if err == nil {
		t.Fatal("expected unquoted pattern to reject an uppercase value, got nil")
	}

	if !strings.Contains(err.Error(), `does not match regular expression "^[a-z]{2,4}$"`) {
		t.Errorf("expected rejection message with the bare pattern, got: %v", err)
	}
}

// TestValidate_RegexpQuotedPatternSyntaxError verifies that a syntactically
// broken quoted pattern reports the compile failure with the stripped pattern,
// not the raw quoted one.
func TestValidate_RegexpQuotedPatternSyntaxError(t *testing.T) {
	type BrokenPatternConfig struct {
		Code string `yaml:"code" validate:"regexp='a(b'"`
	}

	cfg := BrokenPatternConfig{Code: "abc"}
	err := Validate(&cfg)
	if err == nil {
		t.Fatal("expected a syntax error for an unbalanced pattern, got nil")
	}

	if !strings.Contains(err.Error(), `invalid regular expression syntax "a(b"`) {
		t.Errorf("expected syntax error with the stripped pattern, got: %v", err)
	}
}

// TestValidate_MutualExclusionEndpointURL verifies that endpoint and url are
// detected as mutually exclusive network rules on a field with a valid value.
func TestValidate_MutualExclusionEndpointURL(t *testing.T) {
	type PairConfig struct {
		Addr string `yaml:"addr" validate:"endpoint,url"`
	}

	cfg := PairConfig{Addr: "127.0.0.1:8080"}
	err := Validate(&cfg)
	if err == nil {
		t.Fatal("expected endpoint+url pair to be rejected as mutually exclusive, got nil")
	}

	if !strings.Contains(err.Error(), "format rules are strictly mutually exclusive") {
		t.Errorf("expected mutual exclusion error, got: %v", err)
	}
}

// TestValidate_MutualExclusionZeroValue verifies that the mutual exclusion
// check is a configuration error independent of the field value: the same
// pair on a zero-value field must still be rejected.
func TestValidate_MutualExclusionZeroValue(t *testing.T) {
	type ZeroPairConfig struct {
		Addr string `yaml:"addr" validate:"endpoint,url"`
	}

	cfg := ZeroPairConfig{}
	err := Validate(&cfg)
	if err == nil {
		t.Fatal("expected endpoint+url pair to be rejected even on a zero-value field, got nil")
	}

	if !strings.Contains(err.Error(), "format rules are strictly mutually exclusive") {
		t.Errorf("expected mutual exclusion error, got: %v", err)
	}
}

// TestValidate_MutualExclusionFormatURL guards the pre-existing exclusions
// between format and the network flag rules (url and endpoint).
func TestValidate_MutualExclusionFormatURL(t *testing.T) {
	type FormatURLConfig struct {
		Addr string `yaml:"addr" validate:"format=ip,url"`
	}

	cfg := FormatURLConfig{Addr: "10.0.0.1"}
	err := Validate(&cfg)
	if err == nil {
		t.Fatal("expected format+url pair to be rejected as mutually exclusive, got nil")
	}

	if !strings.Contains(err.Error(), "format rules are strictly mutually exclusive") {
		t.Errorf("expected mutual exclusion error, got: %v", err)
	}

	type FormatEndpointConfig struct {
		Addr string `yaml:"addr" validate:"format=ip,endpoint"`
	}

	epCfg := FormatEndpointConfig{Addr: "10.0.0.1"}
	err = Validate(&epCfg)
	if err == nil {
		t.Fatal("expected format+endpoint pair to be rejected as mutually exclusive, got nil")
	}

	if !strings.Contains(err.Error(), "format rules are strictly mutually exclusive") {
		t.Errorf("expected mutual exclusion error, got: %v", err)
	}
}

// TestValidate_MutualExclusionSingleRule verifies that each network rule is
// allowed on its own without tripping the mutual exclusion check.
func TestValidate_MutualExclusionSingleRule(t *testing.T) {
	type EndpointConfig struct {
		Addr string `yaml:"addr" validate:"endpoint"`
	}
	type URLConfig struct {
		Addr string `yaml:"addr" validate:"url"`
	}
	type FormatConfig struct {
		Addr string `yaml:"addr" validate:"format=ip"`
	}

	endpointCfg := EndpointConfig{Addr: "127.0.0.1:8080"}
	if err := Validate(&endpointCfg); err != nil {
		t.Errorf("expected lone endpoint rule to pass, got: %v", err)
	}

	urlCfg := URLConfig{Addr: "https://example.com"}
	if err := Validate(&urlCfg); err != nil {
		t.Errorf("expected lone url rule to pass, got: %v", err)
	}

	formatCfg := FormatConfig{Addr: "10.0.0.1"}
	if err := Validate(&formatCfg); err != nil {
		t.Errorf("expected lone format=ip rule to pass, got: %v", err)
	}
}

// TestValidate_CyclicPointerStructure verifies that a cyclic pointer structure
// terminates validation with an explicit error instead of hanging forever:
// a node referencing itself must produce a "cyclic structure detected" error
// on the referencing field path.
func TestValidate_CyclicPointerStructure(t *testing.T) {
	type CyclicNode struct {
		Name string      `yaml:"name" validate:"not_empty"`
		Next *CyclicNode `yaml:"next"`
	}

	node := CyclicNode{Name: "self"}
	node.Next = &node

	err := Validate(&node)
	if err == nil {
		t.Fatal("expected cyclic structure to produce an error, got nil")
	}
	if !strings.Contains(err.Error(), "cyclic") {
		t.Errorf("expected error to mention 'cyclic', got: %v", err)
	}
}

// TestHelpStr_SelfReferentialTypeTerminates verifies that HelpStr terminates
// on a self-referential type and documents the recursive field exactly once.
func TestHelpStr_SelfReferentialTypeTerminates(t *testing.T) {
	type HelpNode struct {
		Name string    `yaml:"name" description:"node name"`
		Next *HelpNode `yaml:"next" description:"next node"`
	}

	out := HelpStr(&HelpNode{})
	if out == "" {
		t.Fatal("expected non-empty help output")
	}
	if got := strings.Count(out, "next:"); got != 1 {
		t.Errorf("expected recursive field 'next' documented exactly once, got %d occurrences in:\n%s", got, out)
	}
}

// TestValidate_SharedSubobjectNoFalseCycle verifies that two fields pointing
// at the same shared instance do not trigger a false cycle error: the guard
// must be path-scoped, not global.
func TestValidate_SharedSubobjectNoFalseCycle(t *testing.T) {
	type SharedInner struct {
		Value string `yaml:"value" validate:"not_empty"`
	}
	type SharedOuter struct {
		A *SharedInner `yaml:"a"`
		B *SharedInner `yaml:"b"`
	}

	shared := &SharedInner{Value: "x"}
	cfg := SharedOuter{A: shared, B: shared}

	if err := Validate(&cfg); err != nil {
		t.Errorf("expected shared subobject not to be flagged as a cycle, got: %v", err)
	}
}

// TestValidate_DeepAcyclicChain verifies that a long acyclic pointer chain
// terminates without a false cycle error.
func TestValidate_DeepAcyclicChain(t *testing.T) {
	type ChainNode struct {
		Name string     `yaml:"name" validate:"not_empty"`
		Next *ChainNode `yaml:"next"`
	}

	const depth = 100
	head := &ChainNode{Name: "n0"}
	cur := head
	for i := 1; i < depth; i++ {
		cur.Next = &ChainNode{Name: fmt.Sprintf("n%d", i)}
		cur = cur.Next
	}

	if err := Validate(head); err != nil {
		t.Errorf("expected deep acyclic chain to pass, got: %v", err)
	}
}

// --- Invalid boundary limit configurations must be reported, not silently swallowed ---

// TestValidate_ConfigError_MinLenFirstInvalid verifies that an unparseable
// minlen is reported even when maxlen parses fine (the shared-err bug used to
// overwrite the first limit's error with the second parse).
//
// Before the fix, this passed silently — the minlen parse error was
// overwritten by the valid maxlen parse and validation returned nil.
func TestValidate_ConfigError_MinLenFirstInvalid(t *testing.T) {
	type C struct {
		Name string `yaml:"name" validate:"minlen=abc,maxlen=5"`
	}

	cfg := C{Name: "ok"}
	err := Validate(&cfg)
	if err == nil {
		t.Fatal("expected invalid minlen limit to be reported, got nil")
	}

	if !strings.Contains(err.Error(), "invalid validator configuration") || !strings.Contains(err.Error(), "minlen") {
		t.Errorf("expected configuration error naming minlen, got: %v", err)
	}
}

// TestValidate_ConfigError_MaxLenInvalid verifies that an unparseable maxlen
// is reported with the limit name and raw value.
//
// Before the fix, the error was reported without naming the limit
// ("invalid validator configuration for string length limits").
func TestValidate_ConfigError_MaxLenInvalid(t *testing.T) {
	type C struct {
		Name string `yaml:"name" validate:"minlen=2,maxlen=abc"`
	}

	cfg := C{Name: "abcd"}
	err := Validate(&cfg)
	if err == nil {
		t.Fatal("expected invalid maxlen limit to be reported, got nil")
	}

	if !strings.Contains(err.Error(), "invalid validator configuration") || !strings.Contains(err.Error(), "maxlen") {
		t.Errorf("expected configuration error naming maxlen, got: %v", err)
	}
}

// TestValidate_ConfigError_MinCountInvalid verifies that an unparseable
// mincount on a slice is reported as a configuration error.
//
// Before the fix, this passed silently — the broken mincount/maxcount
// block never reported the parse error and minCount stayed at the zero value.
func TestValidate_ConfigError_MinCountInvalid(t *testing.T) {
	type C struct {
		Items []string `yaml:"items" validate:"mincount=abc"`
	}

	cfg := C{Items: []string{"a", "b"}}
	err := Validate(&cfg)
	if err == nil {
		t.Fatal("expected invalid mincount limit to be reported, got nil")
	}

	if !strings.Contains(err.Error(), "invalid validator configuration") || !strings.Contains(err.Error(), "mincount") {
		t.Errorf("expected configuration error naming mincount, got: %v", err)
	}
}

// TestValidate_ConfigError_MaxCountInvalid verifies that an unparseable
// maxcount is reported as a configuration error instead of producing a bogus
// mincount>maxcount complaint on valid data.
//
// Before the fix, the broken re-parse block produced the false error
// "mincount (1) cannot be greater than maxcount (0)" for a two-element slice.
func TestValidate_ConfigError_MaxCountInvalid(t *testing.T) {
	type C struct {
		Items []string `yaml:"items" validate:"mincount=1,maxcount=abc"`
	}

	cfg := C{Items: []string{"a", "b"}}
	err := Validate(&cfg)
	if err == nil {
		t.Fatal("expected invalid maxcount limit to be reported, got nil")
	}

	if !strings.Contains(err.Error(), "invalid validator configuration") || !strings.Contains(err.Error(), `maxcount "abc"`) {
		t.Errorf("expected configuration error naming maxcount, got: %v", err)
	}
}

// TestValidate_ConfigError_DurationFirstLimitInvalid verifies per-limit
// reporting in the duration boundary validator: the first limit fails to
// parse, the second one is valid, the first error must still be reported.
//
// Before the fix, this passed silently — the shared err variable was
// overwritten by the valid max parse.
func TestValidate_ConfigError_DurationFirstLimitInvalid(t *testing.T) {
	type C struct {
		Timeout time.Duration `yaml:"timeout" validate:"min=abc,max=10s"`
	}

	cfg := C{Timeout: 5 * time.Second}
	err := Validate(&cfg)
	if err == nil {
		t.Fatal("expected invalid duration min limit to be reported, got nil")
	}

	if !strings.Contains(err.Error(), "invalid validator configuration") || !strings.Contains(err.Error(), "min") {
		t.Errorf("expected configuration error naming min, got: %v", err)
	}
}

// TestValidate_ConfigError_IntFirstLimitInvalid verifies per-limit reporting
// in the signed int boundary validator.
//
// Before the fix, this passed silently — the shared err variable was
// overwritten by the valid max parse.
func TestValidate_ConfigError_IntFirstLimitInvalid(t *testing.T) {
	type C struct {
		Workers int `yaml:"workers" validate:"min=abc,max=100"`
	}

	cfg := C{Workers: 50}
	err := Validate(&cfg)
	if err == nil {
		t.Fatal("expected invalid int min limit to be reported, got nil")
	}

	if !strings.Contains(err.Error(), "invalid validator configuration") || !strings.Contains(err.Error(), "min") {
		t.Errorf("expected configuration error naming min, got: %v", err)
	}
}

// TestValidate_ConfigError_UintFirstLimitInvalid verifies per-limit reporting
// in the uint boundary validator.
//
// Before the fix, this passed silently — the shared err variable was
// overwritten by the valid max parse.
func TestValidate_ConfigError_UintFirstLimitInvalid(t *testing.T) {
	type C struct {
		Workers uint `yaml:"workers" validate:"min=abc,max=100"`
	}

	cfg := C{Workers: 50}
	err := Validate(&cfg)
	if err == nil {
		t.Fatal("expected invalid uint min limit to be reported, got nil")
	}

	if !strings.Contains(err.Error(), "invalid validator configuration") || !strings.Contains(err.Error(), "min") {
		t.Errorf("expected configuration error naming min, got: %v", err)
	}
}

// TestValidate_ConfigError_FloatFirstLimitInvalid verifies per-limit reporting
// in the float boundary validator.
//
// Before the fix, this passed silently — the shared err variable was
// overwritten by the valid max parse.
func TestValidate_ConfigError_FloatFirstLimitInvalid(t *testing.T) {
	type C struct {
		Ratio float64 `yaml:"ratio" validate:"min=abc,max=100"`
	}

	cfg := C{Ratio: 0.25}
	err := Validate(&cfg)
	if err == nil {
		t.Fatal("expected invalid float min limit to be reported, got nil")
	}

	if !strings.Contains(err.Error(), "invalid validator configuration") || !strings.Contains(err.Error(), "min") {
		t.Errorf("expected configuration error naming min, got: %v", err)
	}
}

// TestValidate_ConfigError_BothLimitsInvalid pins the decision that every
// invalid limit in a block is reported individually (no fail-fast on the first
// one).
//
// Before the fix, this passed silently — both parse errors were
// swallowed by the shared-err overwrite chain.
func TestValidate_ConfigError_BothLimitsInvalid(t *testing.T) {
	type C struct {
		Name string `yaml:"name" validate:"minlen=abc,maxlen=xyz"`
	}

	cfg := C{Name: "ok"}
	err := Validate(&cfg)
	if err == nil {
		t.Fatal("expected both invalid limits to be reported, got nil")
	}

	if !strings.Contains(err.Error(), "minlen") || !strings.Contains(err.Error(), "maxlen") {
		t.Errorf("expected configuration errors for both minlen and maxlen, got: %v", err)
	}
}

// TestValidate_ConfigError_UnsupportedKind verifies that boundary rules on a
// non-numeric kind are a configuration error instead of a silent skip.
//
// Before the fix, the default branch of the boundary switch skipped
// silently and this passed with a nil error.
func TestValidate_ConfigError_UnsupportedKind(t *testing.T) {
	type C struct {
		Flag bool `yaml:"flag" validate:"min=1"`
	}

	cfg := C{}
	err := Validate(&cfg)
	if err == nil {
		t.Fatal("expected boundary rules on a bool field to be reported, got nil")
	}

	if !strings.Contains(err.Error(), "boundary validators are not supported for kind bool") {
		t.Errorf("expected unsupported-kind configuration error, got: %v", err)
	}
}

// TestValidate_ConfigError_LtInvalid verifies per-limit reporting for the
// strict-comparison limits (lt/gt), which share the same shared-err parsing
// block as min/max.
//
// Before the fix, the lt parse error was reported without naming the
// limit ("parse limit error: ...").
func TestValidate_ConfigError_LtInvalid(t *testing.T) {
	type C struct {
		Workers int `yaml:"workers" validate:"lt=abc"`
	}

	cfg := C{Workers: 50}
	err := Validate(&cfg)
	if err == nil {
		t.Fatal("expected invalid lt limit to be reported, got nil")
	}

	if !strings.Contains(err.Error(), "invalid validator configuration") || !strings.Contains(err.Error(), `lt "abc"`) {
		t.Errorf("expected configuration error naming lt, got: %v", err)
	}
}

// TestValidate_ConfigError_NegativeUintMinInvalid verifies that a negative
// limit on a uint field is reported as a configuration error.
//
// Before the fix, this passed silently — the min parse error was
// overwritten by the valid max parse.
func TestValidate_ConfigError_NegativeUintMinInvalid(t *testing.T) {
	type C struct {
		Workers uint `yaml:"workers" validate:"min=-5,max=100"`
	}

	cfg := C{Workers: 50}
	err := Validate(&cfg)
	if err == nil {
		t.Fatal("expected negative uint min limit to be reported, got nil")
	}

	if !strings.Contains(err.Error(), "invalid validator configuration") || !strings.Contains(err.Error(), "min (-5)") {
		t.Errorf("expected configuration error naming min, got: %v", err)
	}
}

// TestValidate_ValidBoundaryLimitsNoErrors is a guard: every boundary branch
// with well-formed limits must keep passing without configuration errors.
// Strict comparisons (gt/lt) are placed on separate fields: gt conflicts with
// min and lt with max by the conflicting-boundaries contract.
func TestValidate_ValidBoundaryLimitsNoErrors(t *testing.T) {
	type C struct {
		Name          string         `yaml:"name" validate:"minlen=2,maxlen=10"`
		Items         []string       `yaml:"items" validate:"mincount=1,maxcount=5"`
		Mapping       map[string]int `yaml:"mapping" validate:"mincount=1,maxcount=5"`
		Timeout       time.Duration  `yaml:"timeout" validate:"min=1s,max=10s"`
		StrictTimeout time.Duration  `yaml:"strict_timeout" validate:"gt=500ms,lt=20s"`
		Workers       int            `yaml:"workers" validate:"min=1,max=100"`
		StrictWorkers int            `yaml:"strict_workers" validate:"gt=0,lt=1000"`
		UWorkers      uint           `yaml:"uworkers" validate:"min=1,max=100"`
		Ratio         float64        `yaml:"ratio" validate:"min=0.1,max=0.9"`
	}

	cfg := C{
		Name:          "abc",
		Items:         []string{"a", "b"},
		Mapping:       map[string]int{"k": 1},
		Timeout:       5 * time.Second,
		StrictTimeout: 5 * time.Second,
		Workers:       50,
		StrictWorkers: 50,
		UWorkers:      50,
		Ratio:         0.5,
	}

	if err := Validate(&cfg); err != nil {
		t.Fatalf("expected valid boundary limits to pass, got: %v", err)
	}
}

// TestValidate_CrossCheckMinMaxConfigError is a guard: the pre-existing
// min>max cross-check message must survive the per-limit parsing rework.
func TestValidate_CrossCheckMinMaxConfigError(t *testing.T) {
	type C struct {
		Workers int `yaml:"workers" validate:"min=100,max=10"`
	}

	cfg := C{Workers: 50}
	err := Validate(&cfg)
	if err == nil {
		t.Fatal("expected min>max cross-check to be reported, got nil")
	}

	if !strings.Contains(err.Error(), "min 100 > max 10") {
		t.Errorf("expected cross-check configuration error, got: %v", err)
	}
}

// TestValidate_ConfigError_NegativeMinLenGuard is a guard: a negative minlen,
// which the old code already rejected via the "err != nil || minLen < 0"
// combined branch, must stay rejected (now with the limit named).
func TestValidate_ConfigError_NegativeMinLenGuard(t *testing.T) {
	type C struct {
		Name string `yaml:"name" validate:"minlen=-1"`
	}

	cfg := C{Name: "abc"}
	if err := Validate(&cfg); err == nil {
		t.Fatal("expected negative minlen to be rejected, got nil")
	}
}

// TestPriority_ValueYAMLWinsOverEnv is the primary regression pin for the
// priority:"value" tag: when a YAML value and a non-empty environment
// variable are both present, the decoded YAML value wins and the environment
// is skipped. Before the feature the environment had absolute precedence, so
// this test fails on pre-feature code.
func TestPriority_ValueYAMLWinsOverEnv(t *testing.T) {
	type Config struct {
		Host string `yaml:"host" default:"fallback" env:"TEST_PRIO_HOST" priority:"value"`
	}

	t.Setenv("TEST_PRIO_HOST", "env-host")

	var cfg Config
	if err := Load([]byte("host: yaml-host\n"), &cfg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Host != "yaml-host" {
		t.Errorf("expected YAML value to win, got %q", cfg.Host)
	}
}

// TestPriority_ValueDefaultWinsOverEnv verifies the second arbitration case:
// no YAML value, but a default tag and a non-empty environment variable are
// both present — the default wins. Pre-feature code applied the
// environment value.
func TestPriority_ValueDefaultWinsOverEnv(t *testing.T) {
	type Config struct {
		Host string `yaml:"host" default:"default-host" env:"TEST_PRIO_DEF_HOST" priority:"value"`
	}

	t.Setenv("TEST_PRIO_DEF_HOST", "env-host")

	var cfg Config
	if err := Load([]byte("unused: true\n"), &cfg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Host != "default-host" {
		t.Errorf("expected default value to win over env, got %q", cfg.Host)
	}
}

// TestPriority_ValueSingleSources checks that the tag never blocks a lone
// source: a YAML value without an environment variable is kept as-is, and an
// environment variable without a YAML value or a default is applied (the
// field is zero after decode, so the environment is the only source).
func TestPriority_ValueSingleSources(t *testing.T) {
	type Config struct {
		Host string `yaml:"host" env:"TEST_PRIO_SINGLE_HOST" priority:"value"`
		Port int    `yaml:"port" env:"TEST_PRIO_SINGLE_PORT" priority:"value"`
	}

	t.Run("yaml only", func(t *testing.T) {
		var cfg Config
		if err := Load([]byte("host: yaml-host\n"), &cfg); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if cfg.Host != "yaml-host" {
			t.Errorf("expected YAML value, got %q", cfg.Host)
		}
	})

	t.Run("env only", func(t *testing.T) {
		t.Setenv("TEST_PRIO_SINGLE_PORT", "9090")

		var cfg Config
		if err := Load([]byte("unused: true\n"), &cfg); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if cfg.Port != 9090 {
			t.Errorf("expected env value 9090, got %d", cfg.Port)
		}
	})
}

// TestPriority_EnvExplicitAndAbsent anchors the unchanged behavior: an
// explicit priority:"env" and an absent priority tag both keep the absolute
// environment precedence over decoded YAML values.
func TestPriority_EnvExplicitAndAbsent(t *testing.T) {
	type Config struct {
		Explicit string `yaml:"explicit" default:"fallback" env:"TEST_PRIO_ANCHOR_EXPLICIT" priority:"env"`
		Absent   string `yaml:"absent" default:"fallback" env:"TEST_PRIO_ANCHOR_ABSENT"`
	}

	t.Setenv("TEST_PRIO_ANCHOR_EXPLICIT", "env-explicit")
	t.Setenv("TEST_PRIO_ANCHOR_ABSENT", "env-absent")

	var cfg Config
	if err := Load([]byte("explicit: yaml-explicit\nabsent: yaml-absent\n"), &cfg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Explicit != "env-explicit" {
		t.Errorf("expected env to win with priority:\"env\", got %q", cfg.Explicit)
	}

	if cfg.Absent != "env-absent" {
		t.Errorf("expected env to win without priority tag, got %q", cfg.Absent)
	}
}

// TestPriority_ValueStandaloneSetDefaultsPreservesFilled verifies the
// standalone SetDefaults contract under priority:"value": a pre-filled field
// keeps its value even when the environment variable is set. Pre-feature code
// force-applied the environment value (isEnvOverride).
func TestPriority_ValueStandaloneSetDefaultsPreservesFilled(t *testing.T) {
	type Config struct {
		Host string `yaml:"host" default:"default-host" env:"TEST_PRIO_FILLED_HOST" priority:"value"`
	}

	t.Setenv("TEST_PRIO_FILLED_HOST", "env-host")

	cfg := Config{Host: "preset-host"}
	if err := SetDefaults(&cfg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Host != "preset-host" {
		t.Errorf("expected pre-filled value to be preserved, got %q", cfg.Host)
	}
}

// TestPriority_ValueNestedStruct verifies the nested-struct parity with the
// environment feature: a priority:"value" field inside a nested block keeps
// its decoded YAML value over a set environment variable. Pre-feature code
// applied the environment value.
func TestPriority_ValueNestedStruct(t *testing.T) {
	type Inner struct {
		Mode string `yaml:"mode" default:"fallback" env:"TEST_PRIO_NESTED_MODE" priority:"value"`
	}

	type Config struct {
		Server Inner `yaml:"server"`
	}

	t.Setenv("TEST_PRIO_NESTED_MODE", "env-mode")

	var cfg Config
	if err := Load([]byte("server:\n  mode: yaml-mode\n"), &cfg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Server.Mode != "yaml-mode" {
		t.Errorf("expected nested YAML value to win, got %q", cfg.Server.Mode)
	}
}

// TestPriority_ConfigErrors checks the static tag validation of the
// configuration-error precedent: an unknown priority value
// and a priority tag without an env tag are reported regardless of field
// values and environment state. Pre-feature code accepted both
// configurations silently.
func TestPriority_ConfigErrors(t *testing.T) {
	t.Run("unknown value", func(t *testing.T) {
		type Config struct {
			Host string `yaml:"host" env:"TEST_PRIO_CFGERR_HOST" priority:"speed"`
		}

		var cfg Config
		err := Load([]byte("host: yaml-host\n"), &cfg)
		if err == nil {
			t.Fatal("expected unknown priority value to be reported, got nil")
		}

		if !strings.Contains(err.Error(), `unknown 'priority' tag value "speed" (must be "value" or "env")`) {
			t.Errorf("unexpected error text: %v", err)
		}
	})

	t.Run("missing env tag", func(t *testing.T) {
		type Config struct {
			Host string `yaml:"host" priority:"value"`
		}

		var cfg Config
		err := Load([]byte("host: yaml-host\n"), &cfg)
		if err == nil {
			t.Fatal("expected priority without env tag to be reported, got nil")
		}

		if !strings.Contains(err.Error(), `'priority' tag requires 'env' tag`) {
			t.Errorf("unexpected error text: %v", err)
		}
	})
}

// TestPriority_ExplicitZeroYAMLAppliesEnv documents the boundary of the
// arbitration semantics: "value exists" means a non-zero field after
// decode+defaults. An explicit zero in YAML leaves the field zero even when a
// default tag is declared, so the environment applies as the only source.
// Pre-feature code applied the environment too.
func TestPriority_ExplicitZeroYAMLAppliesEnv(t *testing.T) {
	type Config struct {
		Host string `yaml:"host" default:"default-host" env:"TEST_PRIO_ZERO_HOST" priority:"value"`
	}

	t.Setenv("TEST_PRIO_ZERO_HOST", "env-host")

	var cfg Config
	if err := Load([]byte("host: \"\"\n"), &cfg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Host != "env-host" {
		t.Errorf("expected env to apply on explicit zero, got %q", cfg.Host)
	}
}

// TestPriority_ZeroDefaultTagAppliesEnv locks the arbitration criterion for
// zero-valued default tags: "value exists" means a non-zero field after
// defaults and decode, and a zero default tag ("0", "false", "") never makes
// the field non-zero, so the environment applies even with priority:"value".
func TestPriority_ZeroDefaultTagAppliesEnv(t *testing.T) {
	t.Run("zero int default", func(t *testing.T) {
		type Config struct {
			Port int `yaml:"port" default:"0" env:"TEST_PRIO_ZERO_DEF_PORT" priority:"value"`
		}

		t.Setenv("TEST_PRIO_ZERO_DEF_PORT", "9090")

		var cfg Config
		if err := Load([]byte("{}\n"), &cfg); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if cfg.Port != 9090 {
			t.Errorf("expected env to apply on zero int default, got %d", cfg.Port)
		}
	})

	t.Run("zero bool default", func(t *testing.T) {
		type Config struct {
			Enabled bool `yaml:"enabled" default:"false" env:"TEST_PRIO_ZERO_DEF_ENABLED" priority:"value"`
		}

		t.Setenv("TEST_PRIO_ZERO_DEF_ENABLED", "true")

		var cfg Config
		if err := Load([]byte("{}\n"), &cfg); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !cfg.Enabled {
			t.Error("expected env to apply on zero bool default, got false")
		}
	})

	t.Run("zero string default", func(t *testing.T) {
		type Config struct {
			Host string `yaml:"host" default:"" env:"TEST_PRIO_ZERO_DEF_HOST" priority:"value"`
		}

		t.Setenv("TEST_PRIO_ZERO_DEF_HOST", "env-host")

		var cfg Config
		if err := Load([]byte("{}\n"), &cfg); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if cfg.Host != "env-host" {
			t.Errorf("expected env to apply on zero string default, got %q", cfg.Host)
		}
	})
}

// --- Unexported fields ---

// TestSetDefaults_UnexportedFieldsSkipped verifies that unexported fields are
// skipped entirely by the defaults engine: their default/env tags are neither
// applied nor reported, and the walk never panics on read-only reflect
// values. Pre-fix code panicked with "reflect.Value.SetString using
// value obtained using unexported field".
func TestSetDefaults_UnexportedFieldsSkipped(t *testing.T) {
	type inner struct {
		Val string `yaml:"val" default:"inner-default"`
	}

	type Config struct {
		Host   string `yaml:"host" default:"default-host"`
		secret string `yaml:"secret" default:"leaked-default"`
		hidden int    `yaml:"hidden" default:"42"`
		nested inner  `yaml:"nested"`
	}

	cfg := Config{Host: "preset"}

	if err := SetDefaults(&cfg); err != nil {
		t.Fatalf("expected unexported fields to be skipped without error, got: %v", err)
	}

	if cfg.Host != "preset" {
		t.Errorf("exported pre-filled field must stay untouched, got %q", cfg.Host)
	}
	if cfg.secret != "" {
		t.Errorf("expected unexported field with a default tag to stay zero, got %q", cfg.secret)
	}
	if cfg.hidden != 0 {
		t.Errorf("expected unexported int field with a default tag to stay zero, got %d", cfg.hidden)
	}
	if cfg.nested.Val != "" {
		t.Errorf("expected fields of an unexported struct field to stay zero, got %q", cfg.nested.Val)
	}
}

// TestValidate_UnexportedRequiredIfTarget verifies that a required_if target
// resolving to an unexported field is reported as a configuration error
// instead of panicking in reflect.Value.Interface. Pre-fix code panicked
// with "reflect.Value.Interface: using value obtained using unexported field".
func TestValidate_UnexportedRequiredIfTarget(t *testing.T) {
	t.Run("bare sibling target", func(t *testing.T) {
		type Config struct {
			Token  string `yaml:"token" validate:"required_if=secret==x"`
			secret string `yaml:"secret"`
		}

		cfg := Config{}
		err := Validate(&cfg)
		if err == nil {
			t.Fatal("expected unexported required_if target to be reported, got nil")
		}

		if !strings.Contains(err.Error(), `invalid required_if configuration: target field "secret" is not exported`) {
			t.Errorf("unexpected error text: %v", err)
		}
	})

	t.Run("dot path target", func(t *testing.T) {
		type inner struct {
			secret string `yaml:"secret"`
		}
		type Config struct {
			Inner inner  `yaml:"inner"`
			Token string `yaml:"token" validate:"required_if=inner.secret==x"`
		}

		cfg := Config{}
		err := Validate(&cfg)
		if err == nil {
			t.Fatal("expected unexported dot-path required_if target to be reported, got nil")
		}

		if !strings.Contains(err.Error(), `invalid required_if configuration: target field "inner.secret" is not exported`) {
			t.Errorf("unexpected error text: %v", err)
		}
	})
}

// TestValidate_UnexportedBoundaryDuration verifies that a boundary validator
// on an unexported time.Duration field neither panics nor reports: the field
// is outside the configuration surface and is skipped entirely. Pre-fix code
// panicked in v.Interface().(time.Duration).
func TestValidate_UnexportedBoundaryDuration(t *testing.T) {
	type Config struct {
		timeout time.Duration `yaml:"timeout" validate:"min=1s,max=10s"`
	}

	cfg := Config{}
	if err := Validate(&cfg); err != nil {
		t.Fatalf("expected unexported Duration to be skipped without error, got: %v", err)
	}
}

// TestValidate_UnexportedPathRuleDurationTarget verifies that a path rule
// resolving to an unexported time.Duration field is reported as a
// configuration error instead of panicking in v.Interface().(time.Duration).
// Pre-fix code panicked.
func TestValidate_UnexportedPathRuleDurationTarget(t *testing.T) {
	type inner struct {
		timeout time.Duration `yaml:"timeout"`
	}
	type Config struct {
		Inner inner `yaml:"inner" validate:"timeout.min=1s"`
	}

	cfg := Config{}
	err := Validate(&cfg)
	if err == nil {
		t.Fatal("expected unexported path-rule target to be reported, got nil")
	}

	if !strings.Contains(err.Error(), `path rule target field "timeout" is not exported`) {
		t.Errorf("unexpected error text: %v", err)
	}
}

// --- Float defaults ---

// TestSetDefaults_FloatDefaultApplied verifies that float32/float64 fields
// accept decimal default tags, completing the defaults matrix parity with the
// strict env matrix. Invalid literals keep the legacy silent skip. Pre-fix
// code had no Float cases — the field stayed zero.
func TestSetDefaults_FloatDefaultApplied(t *testing.T) {
	type Config struct {
		Ratio  float64 `yaml:"ratio" default:"1.5"`
		Share  float32 `yaml:"share" default:"0.25"`
		Broken float64 `yaml:"broken" default:"not-a-number"`
	}

	var cfg Config
	if err := SetDefaults(&cfg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Ratio != 1.5 {
		t.Errorf("expected float64 default 1.5, got %v", cfg.Ratio)
	}
	if cfg.Share != 0.25 {
		t.Errorf("expected float32 default 0.25, got %v", cfg.Share)
	}
	if cfg.Broken != 0 {
		t.Errorf("expected invalid float default to keep the legacy silent skip, got %v", cfg.Broken)
	}
}

// --- Scalar Value fields ---

// TestSetDefaults_ScalarValueFieldTags verifies that a scalar field merely
// named Value is an ordinary field: its own default/env tags apply. Pre-fix
// code treated every Value field as a transparent wrapper and
// silently dropped the tags.
func TestSetDefaults_ScalarValueFieldTags(t *testing.T) {
	type Config struct {
		Value string `yaml:"value" default:"scalar-default" env:"TEST_FA07_SCALAR_VALUE"`
	}

	var cfg Config
	if err := SetDefaults(&cfg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Value != "scalar-default" {
		t.Errorf("expected scalar Value field to take its default, got %q", cfg.Value)
	}

	t.Setenv("TEST_FA07_SCALAR_VALUE", "env-value")

	var envCfg Config
	if err := SetDefaults(&envCfg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if envCfg.Value != "env-value" {
		t.Errorf("expected scalar Value field to take the environment value, got %q", envCfg.Value)
	}
}

// TestLoad_ScalarValueFieldEnvOverride verifies the strict environment pass
// treats a scalar Value field as an ordinary field too. Pre-fix code
// silently dropped the env value.
func TestLoad_ScalarValueFieldEnvOverride(t *testing.T) {
	type Config struct {
		Value string `yaml:"value" default:"scalar-default" env:"TEST_FA07_LOAD_VALUE"`
	}

	t.Setenv("TEST_FA07_LOAD_VALUE", "env-wins")

	var cfg Config
	if err := Load([]byte("unused: true\n"), &cfg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Value != "env-wins" {
		t.Errorf("expected env override on scalar Value field, got %q", cfg.Value)
	}
}

// TestSetDefaults_StructValueWrapperKeepsDefaults verifies the structural
// half of the Value fallback: a struct-typed Value field stays a transparent
// wrapper whose payload receives defaults recursively.
func TestSetDefaults_StructValueWrapperKeepsDefaults(t *testing.T) {
	type wrapper struct {
		Inner string `yaml:"inner" default:"wrapper-default"`
	}

	type Config struct {
		Value wrapper `yaml:"value"`
	}

	var cfg Config
	if err := SetDefaults(&cfg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Value.Inner != "wrapper-default" {
		t.Errorf("expected struct Value wrapper payload to receive defaults, got %q", cfg.Value.Inner)
	}
}

// --- Slice defaults ---

// TestSetDefaults_SliceDefaultMatrix verifies the comma-separated slice
// default matrix: a wholly empty list leaves the slice at its zero value,
// scalar element kinds convert per element with the legacy silent skip, and
// unsupported element kinds are left untouched. Pre-fix code turned
// default:"" into [""] and silently skipped numeric slices.
func TestSetDefaults_SliceDefaultMatrix(t *testing.T) {
	type Config struct {
		Empty  []string         `yaml:"empty" default:""`
		Nums   []int            `yaml:"nums" default:"1,2,3"`
		Flags  []bool           `yaml:"flags" default:"true,false"`
		Ratios []float64        `yaml:"ratios" default:"0.5,1.5"`
		Durs   []time.Duration  `yaml:"durs" default:"1s,2s"`
		Unsup  []map[string]int `yaml:"unsup" default:"x,y"`
		Preset []string         `yaml:"preset"`
	}

	cfg := Config{Preset: []string{"kept"}}

	if err := SetDefaults(&cfg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Empty != nil {
		t.Errorf("expected default:\"\" to leave the slice zero, got %q", cfg.Empty)
	}
	if len(cfg.Nums) != 3 || cfg.Nums[0] != 1 || cfg.Nums[1] != 2 || cfg.Nums[2] != 3 {
		t.Errorf("expected int slice default [1 2 3], got %v", cfg.Nums)
	}
	if len(cfg.Flags) != 2 || !cfg.Flags[0] || cfg.Flags[1] {
		t.Errorf("expected bool slice default [true false], got %v", cfg.Flags)
	}
	if len(cfg.Ratios) != 2 || cfg.Ratios[0] != 0.5 || cfg.Ratios[1] != 1.5 {
		t.Errorf("expected float slice default [0.5 1.5], got %v", cfg.Ratios)
	}
	if len(cfg.Durs) != 2 || cfg.Durs[0] != time.Second || cfg.Durs[1] != 2*time.Second {
		t.Errorf("expected duration slice default [1s 2s], got %v", cfg.Durs)
	}
	if cfg.Unsup != nil {
		t.Errorf("expected unsupported element kinds to be left untouched, got %v", cfg.Unsup)
	}
	if len(cfg.Preset) != 1 || cfg.Preset[0] != "kept" {
		t.Errorf("expected a non-zero preset slice to stay untouched, got %v", cfg.Preset)
	}
}

// --- Empty choice list ---

// TestValidate_EmptyChoiceListConfigError verifies that a choice rule
// resolving to an empty list (a bare "choice=" or only empty elements) is a
// configuration error reported before the value gate, regardless of the
// field value and kind. Pre-fix code was a silent no-op.
func TestValidate_EmptyChoiceListConfigError(t *testing.T) {
	t.Run("bare empty list on zero field", func(t *testing.T) {
		type Config struct {
			Mode string `yaml:"mode" validate:"choice="`
		}

		cfg := Config{}
		err := Validate(&cfg)
		if err == nil {
			t.Fatal("expected empty choice list to be reported, got nil")
		}

		if !strings.Contains(err.Error(), "invalid validator configuration: empty choice list") {
			t.Errorf("unexpected error text: %v", err)
		}
	})

	t.Run("empty list on filled field", func(t *testing.T) {
		type Config struct {
			Mode string `yaml:"mode" validate:"choice="`
		}

		cfg := Config{Mode: "anything"}
		err := Validate(&cfg)
		if err == nil {
			t.Fatal("expected empty choice list to be reported regardless of the value, got nil")
		}

		if !strings.Contains(err.Error(), "invalid validator configuration: empty choice list") {
			t.Errorf("unexpected error text: %v", err)
		}
	})

	t.Run("only empty elements", func(t *testing.T) {
		type Config struct {
			Mode string `yaml:"mode" validate:"choice=, ,"`
		}

		cfg := Config{Mode: "anything"}
		err := Validate(&cfg)
		if err == nil {
			t.Fatal("expected a choice list made only of empty elements to be reported, got nil")
		}

		if !strings.Contains(err.Error(), "invalid validator configuration: empty choice list") {
			t.Errorf("unexpected error text: %v", err)
		}
	})

	t.Run("non-string field", func(t *testing.T) {
		type Config struct {
			Num int `yaml:"num" validate:"choice="`
		}

		cfg := Config{}
		err := Validate(&cfg)
		if err == nil {
			t.Fatal("expected empty choice list to be reported regardless of the field kind, got nil")
		}

		if !strings.Contains(err.Error(), "invalid validator configuration: empty choice list") {
			t.Errorf("unexpected error text: %v", err)
		}
	})
}

// --- Slice-level rule config errors report once ---

// TestValidate_SliceRuleConfigErrorOnce verifies that a rule-level
// configuration error on a slice rule is reported exactly once, never
// repeated per inherited element, while value faults keep reporting at every
// element. Pre-fix code reported the config error xN+1 (slice + every
// element).
func TestValidate_SliceRuleConfigErrorOnce(t *testing.T) {
	type item struct {
		Name string `yaml:"name"`
	}
	type Config struct {
		Mode  string `yaml:"mode"`
		Items []item `yaml:"items" validate:"required_if=nonexistent==x"`
	}

	cfg := Config{Mode: "x", Items: []item{{}, {}}}
	err := Validate(&cfg)
	if err == nil {
		t.Fatal("expected broken slice-level required_if to be reported, got nil")
	}

	const want = `invalid required_if configuration: target field "nonexistent" not found`
	if got := strings.Count(err.Error(), want); got != 1 {
		t.Errorf("expected exactly 1 occurrence of the config error, got %d in: %v", got, err)
	}

	// Value faults keep reporting per element in place.
	type okItem struct {
		Name string `yaml:"name"`
	}
	type okConfig struct {
		Mode  string   `yaml:"mode"`
		Items []okItem `yaml:"items" validate:"required_if=mode==on"`
	}

	okCfg := okConfig{Mode: "on", Items: []okItem{{}, {}}}
	err = Validate(&okCfg)
	if err == nil {
		t.Fatal("expected per-element required_if value faults, got nil")
	}

	if got := strings.Count(err.Error(), "is required when field mode==on"); got != 2 {
		t.Errorf("expected exactly 2 per-element value faults, got %d in: %v", got, err)
	}
}

// --- No early return on config errors ---

// TestValidate_ConfigErrorContinuesSubtree verifies that a rule-level
// configuration error on a parent field adds its fault to the composite stack
// and the recursion into the subtree continues, so child value faults are
// reported alongside. Pre-fix code returned early and dropped the child
// fault.
func TestValidate_ConfigErrorContinuesSubtree(t *testing.T) {
	t.Run("conflicting boundaries", func(t *testing.T) {
		type Config struct {
			Server struct {
				Host string `yaml:"host" validate:"required"`
			} `yaml:"server" validate:"min=0,gt=5"`
		}

		cfg := Config{}
		err := Validate(&cfg)
		if err == nil {
			t.Fatal("expected conflicting boundaries plus child fault, got nil")
		}

		if !strings.Contains(err.Error(), "conflicting boundaries metrics") {
			t.Errorf("expected conflicting boundaries error, got: %v", err)
		}
		if !strings.Contains(err.Error(), "field server.host: is empty, but required") {
			t.Errorf("expected child fault to be gathered too, got: %v", err)
		}
	})

	t.Run("mutually exclusive network rules", func(t *testing.T) {
		type Config struct {
			Server struct {
				Host string `yaml:"host" validate:"required"`
			} `yaml:"server" validate:"format=ip,url"`
		}

		cfg := Config{}
		err := Validate(&cfg)
		if err == nil {
			t.Fatal("expected mutual exclusion plus child fault, got nil")
		}

		if !strings.Contains(err.Error(), "format rules are strictly mutually exclusive") {
			t.Errorf("expected mutual exclusion error, got: %v", err)
		}
		if !strings.Contains(err.Error(), "field server.host: is empty, but required") {
			t.Errorf("expected child fault to be gathered too, got: %v", err)
		}
	})
}

// --- Priority on non-scalar fields ---

// TestPriority_NonScalarFieldsRejected verifies that a priority tag on a
// struct, slice or map field is a configuration error: the arbitration has no
// single settable value there, and a silent no-op is not acceptable. Pre-fix
// code accepted the tag and did nothing.
func TestPriority_NonScalarFieldsRejected(t *testing.T) {
	type inner struct {
		X string `yaml:"x"`
	}

	t.Run("struct field", func(t *testing.T) {
		type Config struct {
			Server inner `yaml:"server" env:"TEST_FA16_STRUCT" priority:"value"`
		}

		var cfg Config
		err := SetDefaults(&cfg)
		if err == nil {
			t.Fatal("expected priority on a struct field to be rejected, got nil")
		}

		if !strings.Contains(err.Error(), "'priority' tag is only supported on scalar fields") {
			t.Errorf("unexpected error text: %v", err)
		}
	})

	t.Run("slice field", func(t *testing.T) {
		type Config struct {
			Tags []string `yaml:"tags" env:"TEST_FA16_SLICE" priority:"env"`
		}

		var cfg Config
		err := SetDefaults(&cfg)
		if err == nil {
			t.Fatal("expected priority on a slice field to be rejected, got nil")
		}

		if !strings.Contains(err.Error(), "'priority' tag is only supported on scalar fields") {
			t.Errorf("unexpected error text: %v", err)
		}
	})

	t.Run("map field via load path", func(t *testing.T) {
		type Config struct {
			Mapping map[string]int `yaml:"mapping" env:"TEST_FA16_MAP" priority:"value"`
		}

		var cfg Config
		err := Load([]byte("mapping:\n  k: 1\n"), &cfg)
		if err == nil {
			t.Fatal("expected priority on a map field to be rejected, got nil")
		}

		if !strings.Contains(err.Error(), "'priority' tag is only supported on scalar fields") {
			t.Errorf("unexpected error text: %v", err)
		}
	})
}

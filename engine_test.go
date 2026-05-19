package yaml

import (
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
			errSubstr: "is empty, but required by 'not_empty'",
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
	// ИСПРАВЛЕНО: проверяем корректное имя поля с индексом элемента слайса, как возвращает ядро
	if !strings.Contains(err.Error(), "AllowedTags[1]: value \"java\" is invalid") {
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
	defer func() {
		_ = os.Unsetenv("APP_HOST")
	}()

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
		Word string `validate:"minlen=5,maxlen=5"`
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
		Interval time.Duration `validate:"min=1s,max=10m"`
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
		AnyIP string `validate:"format=ip"`
		V4    string `validate:"format=ipv4"`
		V6    string `validate:"format=ipv6"`
		ID    string `validate:"format=uuid"`
	}
	fc := FormatConfig{AnyIP: "1.2.3.4", V4: "127.0.0.1", V6: "::1", ID: "123e4567-e89b-12d3-a456-426614174000"}
	if err := Validate(&fc); err != nil {
		t.Fatalf("valid networking formats and UUID strings must not produce errors, got: %v", err)
	}

	fc.V4 = "256.0.0.1" // Out-of-bounds IPv4 address layout.
	if err := Validate(&fc); err == nil || !strings.Contains(err.Error(), "is not a valid IPv4 address") {
		t.Errorf("expected strict IPv4 layout evaluation failure, got: %v", err)
	}

	// 4. Verify conditional cross-field checking constraints (required_if).
	type RequiredIfConfig struct {
		Mode        string `yaml:"mode"`
		Token       string `yaml:"token" validate:"required_if=Mode:prod"`
		Webhook     string `yaml:"webhook" validate:"required_if=Token:not_empty"`
		BackupRoute string `yaml:"backup" validate:"required_if=Webhook:empty"`
	}

	ric := RequiredIfConfig{Mode: "prod", Token: ""} // Violates condition since Mode=prod requires Token.
	if err := Validate(&ric); err == nil || !strings.Contains(err.Error(), "is required when field Mode is prod") {
		t.Errorf("expected cross-field dependency validation error, got: %v", err)
	}

	ric = RequiredIfConfig{Mode: "dev", Token: "abc", Webhook: ""} // Token is filled -> Webhook becomes required.
	if err := Validate(&ric); err == nil || !strings.Contains(err.Error(), "field Webhook:") {
		t.Errorf("expected cross-field non-empty macro requirement error, got: %v", err)
	}

	// 5. Verify the Error Aggregation engine behavior.
	type AggregatedConfig struct {
		Age  int    `validate:"min=18"`
		Code string `validate:"minlen=5"`
	}
	ac := AggregatedConfig{Age: 10, Code: "go"} // Triggers 2 separate validation failures simultaneously.
	err := Validate(&ac)
	if err == nil {
		t.Fatal("expected composite error tracking entity return, got nil")
	}

	errStr := err.Error()
	if !strings.Contains(errStr, "Age:") || !strings.Contains(errStr, "Code:") {
		t.Errorf("expected multi-line aggregated error message payload stack, got:\n%s", errStr)
	}
}

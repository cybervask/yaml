package yaml

import (
	"os"
	"strings"
	"testing"
	"time"
)

type TestSubConfig struct {
	Level   string        `yaml:"level" default:"info" validate:"choice=debug,info,warn"`
	Timeout time.Duration `yaml:"timeout" default:"5s"`
}

type TestConfig struct {
	Env          string        `yaml:"env" default:"dev" validate:"choice=dev,stage,prod"`
	Color        string        `yaml:"color" default:"white" validate:"choice=!red,!black"`
	BindAddr     string        `yaml:"bind_addr" validate:"host_port"`
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

	// 1. Apply structure defaults
	if err := SetDefaults(&cfg); err != nil {
		t.Fatalf("unexpected error in SetDefaults: %v", err)
	}

	// Verify default values populate correctly
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

	// 2. Execute structure validation checks
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
			name: "host_port violation",
			modify: func(c *TestConfig) {
				c.BindAddr = "google.com" // Missing port specification
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
			errSubstr: "out of range",
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
				t.Fatal("expected error, got nil")
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

	// Valid validation use case
	cfg := SliceConfig{
		AllowedTags: []string{"golang", "k8s"},
		Ports:       []uint{80, 443},
	}
	if err := Validate(&cfg); err != nil {
		t.Fatalf("expected slice config to be valid, got err: %v", err)
	}

	// Invalid use case: 'choice' violation inside string sequence
	badCfg := SliceConfig{
		AllowedTags: []string{"golang", "java"}, // "java" is missing from the choice list
		Ports:       []uint{80},
	}
	err := Validate(&badCfg)
	if err == nil {
		t.Fatal("expected validation error for invalid slice string element, got nil")
	}
	if !strings.Contains(err.Error(), "AllowedTags[1]: value \"java\" is invalid") {
		t.Errorf("unexpected error text: %v", err)
	}

	// Invalid use case: 'min' boundary range violation inside numeric sequence
	badPortsCfg := SliceConfig{
		AllowedTags: []string{"golang"},
		Ports:       []uint{22}, // 22 is below min=80 constraint
	}
	err = Validate(&badPortsCfg)
	if err == nil {
		t.Fatal("expected validation error for invalid slice integer element, got nil")
	}
	if !strings.Contains(err.Error(), "Ports[0]: value 22 out of range") {
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

	// Simulate unmarshaled structure data where fields are left unassigned (zero-valued)
	cfg := HolderConfig{
		ItemsList: []ItemConfig{
			{Name: "first"}, // Count omitted (defaults to 0)
			{},              // Name and Count both omitted
		},
		ItemsMap: map[string]ItemConfig{
			"key1": {Name: "mapped"}, // Count omitted
			"key2": {},               // Name and Count both omitted
		},
	}

	// Trigger underlying collection properties fallback processing
	if err := SetDefaults(&cfg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify slice element structural mutations
	if cfg.ItemsList[0].Count != 5 {
		t.Errorf("expected ItemsList[0].Count to be 5, got %d", cfg.ItemsList[0].Count)
	}
	if cfg.ItemsList[1].Name != "unknown" || cfg.ItemsList[1].Count != 5 {
		t.Errorf("expected ItemsList[1] defaults to be set, got Name=%q, Count=%d", cfg.ItemsList[1].Name, cfg.ItemsList[1].Count)
	}

	// Verify map element structural mutations
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

	cfg := AppConfig{} // Alpn begins as a nil initialization slice

	if err := SetDefaults(&cfg); err != nil {
		t.Fatalf("unexpected error in SetDefaults: %v", err)
	}

	// Verify slice generation logic split elements properly
	if len(cfg.Crypto.Alpn) != 2 {
		t.Fatalf("expected Alpn slice to have 2 elements, got %d", len(cfg.Crypto.Alpn))
	}

	if cfg.Crypto.Alpn[0] != "h2" || cfg.Crypto.Alpn[1] != "http/1.1" {
		t.Errorf("unexpected slice elements: %v", cfg.Crypto.Alpn)
	}

	// Verify structure validation operates cleanly over dynamically populated arrays
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

	os.Setenv("APP_HOST", "10.0.0.1")
	defer os.Unsetenv("APP_HOST") // Ensure state is safely cleaned up post execution

	cfg := EnvConfig{
		Port: 9090, // Value populated explicitly during early parsing simulation
	}

	if err := SetDefaults(&cfg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// 1. Host must bind to the value extracted from the environment variable context
	if cfg.Host != "10.0.0.1" {
		t.Errorf("expected Host to be '10.0.0.1' from env, got %q", cfg.Host)
	}

	// 2. Port must remain 9090 since configuration data takes absolute precedence over defaults
	if cfg.Port != 9090 {
		t.Errorf("expected Port to remain 9090, got %d", cfg.Port)
	}
}

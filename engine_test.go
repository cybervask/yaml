package yaml

import (
	"strings"
	"testing"
	"time"
)

type TestSubConfig struct {
	Level   string        `yaml:"level" default:"info" validate:"choice=debug,info,warn"`
	Timeout time.Duration `yaml:"timeout" default:"5s"`
}

type TestConfig struct {
	Env          string                 `yaml:"env" default:"dev" validate:"choice=dev,stage,prod"`
	Color        string                 `yaml:"color" default:"white" validate:"choice=!red,!black"`
	BindAddr     string                 `yaml:"bind_addr" validate:"host_port"`
	WebURL       string                 `yaml:"web_url" validate:"url"`
	Code         string                 `yaml:"code" validate:"regexp=^[a-z]{2,4}$"`
	Workers      uint                   `yaml:"workers" default:"10" validate:"min=1,max=100"`
	RequiredItem string                 `yaml:"req" validate:"not_empty"`
	Logging      Include[TestSubConfig] `yaml:"logging"`
}

func TestSetDefaultsAndValidate_Success(t *testing.T) {
	cfg := TestConfig{
		BindAddr:     "127.0.0.1:8080",
		WebURL:       "https://github.com",
		Code:         "yaml",
		RequiredItem: "present",
	}

	// 1. Применяем дефолты
	if err := SetDefaults(&cfg); err != nil {
		t.Fatalf("unexpected error in SetDefaults: %v", err)
	}

	// Проверяем заполнение дефолтов
	if cfg.Env != "dev" {
		t.Errorf("expected Env to be 'dev', got %q", cfg.Env)
	}

	if cfg.Color != "white" {
		t.Errorf("expected Color to be 'white', got %q", cfg.Color)
	}

	if cfg.Workers != 10 {
		t.Errorf("expected Workers to be 10, got %d", cfg.Workers)
	}

	if cfg.Logging.Value.Level != "info" {
		t.Errorf("expected Logging.Level to be 'info', got %q", cfg.Logging.Value.Level)
	}

	// 2. Запускаем валидацию
	if err := Validate(&cfg); err != nil {
		t.Fatalf("unexpected validation error: %v", err)
	}
}

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
				c.BindAddr = "google.com" // без порта
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

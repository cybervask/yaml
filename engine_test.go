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

func TestValidate_SliceElements(t *testing.T) {
	type SliceConfig struct {
		AllowedTags []string `yaml:"tags" validate:"choice=golang,docker,k8s"`
		Ports       []uint   `yaml:"ports" validate:"min=80,max=443"`
	}

	// Валидный кейс
	cfg := SliceConfig{
		AllowedTags: []string{"golang", "k8s"},
		Ports:       []uint{80, 443},
	}
	if err := Validate(&cfg); err != nil {
		t.Fatalf("expected slice config to be valid, got err: %v", err)
	}

	// Невалидный кейс: choice нарушен в слайсе
	badCfg := SliceConfig{
		AllowedTags: []string{"golang", "java"}, // java нет в списке choice
		Ports:       []uint{80},
	}
	err := Validate(&badCfg)
	if err == nil {
		t.Fatal("expected validation error for invalid slice string element, got nil")
	}
	if !strings.Contains(err.Error(), "AllowedTags[1]: value \"java\" is invalid") {
		t.Errorf("unexpected error text: %v", err)
	}

	// Невалидный кейс: min нарушен в слайсе чисел
	badPortsCfg := SliceConfig{
		AllowedTags: []string{"golang"},
		Ports:       []uint{22}, // 22 меньше min=80
	}
	err = Validate(&badPortsCfg)
	if err == nil {
		t.Fatal("expected validation error for invalid slice integer element, got nil")
	}
	if !strings.Contains(err.Error(), "Ports[0]: value 22 out of range") {
		t.Errorf("unexpected error text: %v", err)
	}
}

func TestSetDefaults_InCollections(t *testing.T) {
	type ItemConfig struct {
		Name  string `yaml:"name" default:"unknown"`
		Count int    `yaml:"count" default:"5"`
	}

	type HolderConfig struct {
		ItemsList []ItemConfig          `yaml:"items_list"`
		ItemsMap  map[string]ItemConfig `yaml:"items_map"`
	}

	// Имитируем то, что делает парсер YAML: создает элементы,
	// но оставляет поля Name и Count пустыми (нулевыми)
	cfg := HolderConfig{
		ItemsList: []ItemConfig{
			{Name: "first"}, // Count пропущен (равен 0)
			{},              // И Name, и Count пропущены
		},
		ItemsMap: map[string]ItemConfig{
			"key1": {Name: "mapped"}, // Count пропущен
			"key2": {},               // И Name, и Count пропущены
		},
	}

	// Запускаем наш обновленный движок дефолтов
	if err := SetDefaults(&cfg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Проверяем элементы в СЛАЙСЕ
	if cfg.ItemsList[0].Count != 5 {
		t.Errorf("expected ItemsList[0].Count to be 5, got %d", cfg.ItemsList[0].Count)
	}
	if cfg.ItemsList[1].Name != "unknown" || cfg.ItemsList[1].Count != 5 {
		t.Errorf("expected ItemsList[1] defaults to be set, got Name=%q, Count=%d", cfg.ItemsList[1].Name, cfg.ItemsList[1].Count)
	}

	// Проверяем элементы в МАПЕ
	if cfg.ItemsMap["key1"].Count != 5 {
		t.Errorf("expected ItemsMap['key1'].Count to be 5, got %d", cfg.ItemsMap["key1"].Count)
	}
	if cfg.ItemsMap["key2"].Name != "unknown" || cfg.ItemsMap["key2"].Count != 5 {
		t.Errorf("expected ItemsMap['key2'] defaults to be set, got Name=%q, Count=%d", cfg.ItemsMap["key2"].Name, cfg.ItemsMap["key2"].Count)
	}
}

func TestSetDefaults_SliceString(t *testing.T) {
	type TLS struct {
		Alpn []string `yaml:"alpn" default:"h2,http/1.1" validate:"choice=h2,http/1.1"`
	}

	type AppConfig struct {
		Crypto TLS `yaml:"crypto"`
	}

	cfg := AppConfig{} // Alpn изначально nil

	if err := SetDefaults(&cfg); err != nil {
		t.Fatalf("unexpected error in SetDefaults: %v", err)
	}

	// Проверяем, что слайс создался и заполнился элементами
	if len(cfg.Crypto.Alpn) != 2 {
		t.Fatalf("expected Alpn slice to have 2 elements, got %d", len(cfg.Crypto.Alpn))
	}

	if cfg.Crypto.Alpn[0] != "h2" || cfg.Crypto.Alpn[1] != "http/1.1" {
		t.Errorf("unexpected slice elements: %v", cfg.Crypto.Alpn)
	}

	// Проверяем, что валидация также проходит успешно
	if err := Validate(&cfg); err != nil {
		t.Fatalf("unexpected validation error: %v", err)
	}
}

func TestSetDefaults_EnvPrecedence(t *testing.T) {
	type EnvConfig struct {
		Host string `yaml:"host" default:"localhost" env:"APP_HOST"`
		Port int    `yaml:"port" default:"8080" env:"APP_PORT"`
	}

	os.Setenv("APP_HOST", "10.0.0.1")
	defer os.Unsetenv("APP_HOST") // Очищаем за собой

	cfg := EnvConfig{
		Port: 9090, // Задано жестко на этапе парсинга (эмуляция YAML)
	}

	if err := SetDefaults(&cfg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// 1. Host должен взяться из ENV, так как переменная задана в ОС
	if cfg.Host != "10.0.0.1" {
		t.Errorf("expected Host to be '10.0.0.1' from env, got %q", cfg.Host)
	}

	// 2. Port должен остаться 9090, так как APP_PORT пустой, и значение из YAML в приоритете над дефолтом
	if cfg.Port != 9090 {
		t.Errorf("expected Port to remain 9090, got %d", cfg.Port)
	}
}

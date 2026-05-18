# Smart Go-YAML Engine 🚀 [![Go CI/CD Pipeline](https://github.com)](https://github.com)

Other languages: [Русский (RU)](README.ru.md) | [中文 (ZH)](README.zh.md)

A robust, high-performance drop-in replacement for `go.yaml.in/yaml/v4`. It preserves original function signatures and core options (`Option`) while automatically integrating deep recursive fields defaults (`default:"value"`) and strict tag-based environment validation (`validate:"..."`) right into the parsing layer.

---

## 🛠 Configuration Profiles (Valid & Invalid Cases)

### Example 1: Standard Application Config (Valid Profile)
**YAML Specification (`config.yaml`):**
```yaml
env: "prod"
api_url: "https://cybervask.net"
api_host_port: "127.0.0.1:443"
# 'workers' and 'server.logging.timeout' are omitted and will gracefully fallback to tags defaults
server:
  logging:
    colors: true
    level: "warn"
```
**Go Application Configuration Model Representation:**
```go
package main

import (
	"fmt"
	"log"
	"://github.com"
)

type Logging struct {
	yaml.Includer `yaml:",inline"` // Activates !include support safely
	Level         string           `yaml:"level" default:"info" validate:"choice=debug,info,warn"`
	Colors        bool             `yaml:"colors"`
	Timeout       string           `yaml:"timeout" default:"5s"`
}

type Config struct {
	Env         string  `yaml:"env" default:"dev" validate:"choice=dev,stage,prod"`
	APIUrl      string  `yaml:"api_url" validate:"url"`
	APIHostPort string  `yaml:"api_host_port" validate:"host_port"`
	Workers     int     `yaml:"workers" default:"10" validate:"min=1,max=100"`
	Server      struct {
		Logging Logging `yaml:"logging"`
	} `yaml:"server"`
}
```
**Serialized Go Structure Matrix (via `yaml.Dump`):**
```yaml
env: prod
api_url: https://cybervask.net
api_host_port: 127.0.0.1:443
workers: 10
server:
  logging:
    level: warn
    colors: true
    timeout: 5s
```

---

### Example 2: Choice Whitelist Constraint Violation (Invalid Profile)
**YAML Specification (`config.yaml`):**
```yaml
env: "testing" # Error: "testing" is outside the white-list bounds [dev, stage, prod]
api_url: "https://cybervask.net"
api_host_port: "127.0.0.1:443"
```
**Returned Application Runtime Error String (`err.Error()`):**
```text
field Env: value "testing" is invalid; allowed choices are [dev, stage, prod]
```

---

### Example 3: URL Format & Port Boundaries Anomalies (Invalid Profile)
**YAML Specification (`config.yaml`):**
```yaml
env: "dev"
api_url: "cybervask.net:443"  # Error: Explicit scheme divider "://" is completely missing
api_host_port: "127.0.0.1:85000" # Error: Target socket port parameter exceeds limits (> 65535)
```
**Returned Application Runtime Error String (`err.Error()`):**
```text
field APIUrl: value "cybervask.net:443" is missing a URL scheme separator (e.g., scheme://host)
```
*(Note: If the `api_url` parameter is fixed, the engine moves down the queue to evaluate the port boundary error: `field APIHostPort: value "127.0.0.1:85000" contains an invalid port number`)*

---

### Example 4: Mutually Exclusive Structure Design Panic (Invalid Model Setup)
**Go Code Model Definition:**
```go
type DefectiveConfig struct {
    Token string `yaml:"token" default:"secret_token" validate:"not_empty"`
}
```
**Returned Application Runtime Error String (`err.Error()`):**
```text
field Token is invalid: 'default' and 'not_empty' are mutually exclusive
```

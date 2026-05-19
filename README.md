# Smart Go-YAML Engine

[![Go CI/CD Pipeline](https://github.com/cybervask/yaml/actions/workflows/go.yml/badge.svg)](https://github.com/cybervask/yaml/actions/workflows/go.yml)

Other languages: [Русский (RU)](README.ru.md) | [中文 (ZH)](README.zh.md)

A robust, high-performance drop-in replacement for `go.yaml.in/yaml/v4`. It preserves original function signatures and core options (`Option`) while automatically integrating deep recursive field defaults (`default:"value"`), native OS environment variable injection (`env:"VAR"`), automated CLI help generation (`Help()`), and strict tag-based validation (`validate:"..."`) right into the parsing layer.

---

## Available Validation Rules & Features

You can combine multiple validation rules inside the `validate` tag using a comma separator (e.g., `validate:"not_empty,host_port"`).



| Rule / Feature            | Syntax Example                   | Description                                                                                                                       | Supported Types                             |
|:--------------------------|:---------------------------------|:----------------------------------------------------------------------------------------------------------------------------------|:--------------------------------------------|
| **Environment Injection** | `env:"APP_PORT"`                 | **12-Factor App:** Dynamically injects system environment variables. Takes absolute precedence over YAML and tag defaults.        | `string`, `bool`, primitive numbers         |
| **Required Field**        | `validate:"not_empty"`           | Ensures the field is assigned a non-zero value. Mutually exclusive with the `default` tag.                                        | `string`, `struct`, `slice`, `map`, numbers |
| **Whitelist Choices**     | `validate:"choice=dev,prod"`     | **Whitelist mode:** The string value must perfectly match one of the comma-separated tokens. Supports slice elements recursively. | `string`, `[]string`                        |
| **Blacklist Choices**     | `validate:"choice=!red,!black"`  | **Blacklist mode:** Allows any string value except for the specific exclusion tokens prefixed with `!`.                           | `string`, `[]string`                        |
| **Numeric Range**         | `validate:"min=1,max=100"`       | Enforces strict lower and upper boundaries. Automatically blocks negative parameters for `uint` fields.                           | `int`, `uint`, `float` variants             |
| **Regular Expression**    | `validate:"regexp=^[a-z]{2,4}$"` | Validates string layout matching a regular expression pattern. Safe against embedded commas.                                      | `string`, `[]string`                        |
| **Network Socket**        | `validate:"host_port"`           | Enforces standard network endpoints (`host:port`). Natively checks **IPv6** syntax and port bounds (1-65535).                     | `string`                                    |
| **Enforced URL**          | `validate:"url"`                 | Checks Uniform Resource Identifiers. Strictly requires an explicit protocol scheme separator (e.g., `http://`, `grpc://`).        | `string`                                    |

---

## Configuration Profiles (Valid & Invalid Cases)

### Example 1: Standard Application Config with Help Output (Valid Profile)
**YAML Specification (`config.yaml`):**
```yaml
env: "prod"
api_url: "https://cybervask.net"
api_host_port: "127.0.0.1:443"
# 'workers', 'crypto.alpn', and 'server.logging.timeout' are omitted and fallback to defaults or envs
server:
  logging:
    colors: true
    level: "warn"
```
**Go Application Configuration Model:**
```go
package main

import (
	"fmt"
	"log"
	"github.com/cybervask/yaml"
)

type TLS struct {
	MinVersion string   `yaml:"min_version" default:"tls1.3" validate:"choice=tls1.2,tls1.3" env:"TLS_MIN_VERSION" description:"Minimum TLS version"`
	Alpn       []string `yaml:"alpn" default:"h2,http/1.1" validate:"choice=h2,http/1.1" description:"Application-Layer Protocol Negotiation"`
}

type Logging struct {
	Level         string           `yaml:"level" default:"info" validate:"choice=debug,info,warn"`
	Colors        bool             `yaml:"colors"`
	Timeout       string           `yaml:"timeout" default:"5s"`
}

type Config struct {
	Env         string  `yaml:"env" default:"dev" validate:"choice=dev,stage,prod" description:"Application execution environment"`
	APIUrl      string  `yaml:"api_url" validate:"url" env:"API_URL" description:"Base target API destination link"`
	APIHostPort string  `yaml:"api_host_port" validate:"host_port" description:"Local binding network socket"`
	Crypto      TLS     `yaml:"crypto" description:"Security TLS structure configuration layer"`
	Server      struct {
		Logging Logging `yaml:"logging" description:"Server logging configuration parameters"`
	} `yaml:"server"`
}

func main() {
	var cfg Config
    
	// Easily print beautifully aligned configuration layout scheme documentation anywhere (e.g. on --help / -h)
	yaml.Help(cfg)
}
```

**Automated Interactive CLI Help Output (`yaml.Help(cfg)`):**
```text
yaml configuration schema documentation:

env:           Application execution environment (default: dev, validate: [choice=dev,stage,prod])
api_url:       Base target API destination link (env: API_URL, validate: [url])
api_host_port: Local binding network socket (validate: [host_port])
crypto:        Security TLS structure configuration layer 
  min_version: Minimum TLS version (env: TLS_MIN_VERSION, default: tls1.3, validate: [choice=tls1.2,tls1.3])
  alpn:        Application-Layer Protocol Negotiation (default: h2,http/1.1, validate: [choice=h2,http/1.1])
server:        Server logging configuration parameters 
  logging:     Server logging configuration parameters 
    level:     Log level (default: info, validate: [choice=debug,info,warn])
    colors:    Colors 
    timeout:   Timeout (default: 5s)
```

---

### Example 2: Choice Whitelist Constraint Violation (Invalid Profile)
**YAML Specification (`config.yaml`):**
```yaml
env: "testing" # Error: "testing" is outside the white-list bounds [dev, stage, prod]
```
**Returned Application Runtime Error String (`err.Error()`):**
```text
field Env: value "testing" is invalid; allowed choices are [dev, stage, prod]
```

---

### Example 3: Environment Injection Priority & Port Bounds Checking (Invalid Profile)
**System Environment Context Setup:**
```bash
export API_URL="cybervask.net:443" # Explicitly overrides YAML value, but fails because scheme separator "://" is missing
```
**Returned Application Runtime Error String (`err.Error()`):**
```text
field APIUrl: value "cybervask.net:443" is missing a URL scheme separator (e.g., scheme://host)
```

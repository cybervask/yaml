# Smart Go-YAML Engine

[![Go CI/CD Pipeline](https://github.com/cybervask/yaml/actions/workflows/go.yml/badge.svg)](https://github.com/cybervask/yaml/actions/workflows/go.yml)

Other languages: [Русский (RU)](README.ru.md) | [中文 (ZH)](README.zh.md)

A robust, high-performance drop-in replacement for `go.yaml.in/yaml/v4`. It preserves original function signatures and core options (`Option`) while automatically integrating deep recursive field defaults (`default:"value"`), native OS environment variable injection (`env:"VAR"`), automated CLI help generation (`Help()`), and strict tag-based validation (`validate:"..."`) right into the parsing layer.

---

## Available Validation Rules & Features

You can combine multiple validation rules inside the `validate` tag using a comma separator (e.g., `validate:"not_empty,endpoint"`).



| Rule / Feature            | Syntax Example                     | Description                                                                                                                                | Supported Types                             |
|:--------------------------|:-----------------------------------|:-------------------------------------------------------------------------------------------------------------------------------------------|:--------------------------------------------|
| **Environment Injection** | `env:"APP_PORT"`                   | **12-Factor App:** Dynamically injects system environment variables. Takes absolute precedence over YAML and tag defaults.                 | `string`, `bool`, primitive numbers         |
| **Required Field**        | `validate:"not_empty"`             | Ensures the field is assigned a non-zero value. Mutually exclusive with the `default` tag.                                                 | `string`, `struct`, `slice`, `map`, numbers |
| **Conditional Required**  | `validate:"required_if=role:!srv"` | **Cross-field Validation:** Field becomes required based on another field value. Supports negation `!` and macros (`:empty`/`:not_empty`). | `string`, numbers, `bool` etc.              |
| **Whitelist Choices**     | `validate:"choice=dev,prod"`       | **Whitelist mode:** The string value must perfectly match one of the comma-separated tokens. Supports slice elements recursively.          | `string`, `[]string`                        |
| **Blacklist Choices**     | `validate:"choice=!red,!black"`    | **Blacklist mode:** Allows any string value except for the specific exclusion tokens prefixed with `!`.                                    | `string`, `[]string`                        |
| **Numeric & Duration**    | `validate:"min=1s,max=10m"`        | Enforces atomic **inclusive** lower (`>=`) and upper (`<=`) boundaries. Natively supports numbers and `time.Duration` intervals.           | `int`, `uint`, `float`, `time.Duration`     |
| **Strict Comparison**     | `validate:"gt=5,lt=10"`            | Enforces **exclusive** strict greater than (`>`) and less than (`<`) boundaries. Also supports `time.Duration`.                            | `int`, `uint`, `float`, `time.Duration`     |
| **String Length (Runes)** | `validate:"minlen=3,maxlen=20"`    | Enforces minimum and maximum limits on string length. Professionally counts **Unicode Runes** instead of raw bytes.                        | `string`                                    |
| **Collection Capacity**   | `validate:"mincount=1,maxcount=5`  | Enforces minimum and maximum elements count inside dynamic collections.                                                                    | `slice`, `map`                              |
| **Network Formats**       | `validate:"format=ipv4"`           | Validates specific IP networks layout. Supports `format=ip`, `format=ipv4`, and `format=ipv6`. Mutually exclusive with `endpoint`.         | `string`                                    |
| **Unique Identity**       | `validate:"format=uuid"`           | Formats verification using strict RFC-compliant `google/uuid` sub-parsing layer routines.                                                  | `string`                                    |
| **Regular Expression**    | `validate:"regexp=^[a-z]{2,4}$"`   | Validates string layout matching a regular expression pattern. Safe against embedded commas.                                               | `string`, `[]string`                        |
| **Endpoint**              | `validate:"endpoint"`              | Enforces standard network endpoints (`host:port`). Natively checks **IPv6** syntax and port bounds (1-65535).                              | `string`                                    |
| **Enforced URL**          | `validate:"url"`                   | Checks Uniform Resource Identifiers. Strictly requires an explicit protocol scheme separator (e.g., `http://`, `grpc://`).                 | `string`                                    |

---

## Configuration Profiles (Valid & Invalid Cases)

### Example 1: Standard Application Config with Help Output (Valid Profile)
**YAML Specification (`config.yaml`):**
```yaml
env: "prod"
api_url: "https://cybervask.net"
api_endpoint: "127.0.0.1:443"
server:
  logging:
    colors: true
    level: "warn"
  allowed_ips:
    - "192.168.1.1"
```

**Go Application Configuration Model:**
```go
package main

import (
	"fmt"
	"time"

	"github.com/cybervask/yaml"
)

type TLS struct {
	MinVersion string   `yaml:"min_version" default:"tls1.3" validate:"choice=tls1.2,tls1.3" env:"TLS_MIN_VERSION" description:"Minimum TLS version"`
	Alpn       []string `yaml:"alpn" default:"h2,http/1.1" validate:"choice=h2,http/1.1" description:"Application-Layer Protocol Negotiation"`
}

type Logging struct {
	Level         string           `yaml:"level" default:"info" validate:"choice=debug,info,warn" description:"Log hierarchy execution level"`
	Colors        bool             `yaml:"colors" description:"Enable ANSI colored logs output"`
	Timeout       string           `yaml:"timeout" default:"5s" description:"Internal downstream processing timeout"`
}

type Config struct {
	Env          string        `yaml:"env" default:"dev" validate:"choice=dev,stage,prod" description:"Application execution environment"`
	AppName      string        `yaml:"app_name" default:"api" validate:"minlen=3,maxlen=10" description:"Internal runtime deployment monicker"`
	APIUrl       string        `yaml:"api_url" validate:"url,required_if=env:prod" env:"API_URL" description:"Base target API destination link"`
	APIEndpoint  string        `yaml:"api_endpoint" validate:"endpoint" description:"Local binding network socket"`
	Timeout      time.Duration `yaml:"timeout" default:"5s" validate:"min=1s,max=10m" description:"Global runtime request boundary tracking parameter"`
	Crypto       TLS           `yaml:"crypto" description:"Security TLS structure configuration layer"`
	Server       struct {
		Logging    Logging  `yaml:"logging" description:"Server logging configuration parameters"`
		AllowedIPs []string `yaml:"allowed_ips" validate:"mincount=1,maxcount=10" description:"Whitelisted network remotes"`
	} `yaml:"server"`
}

func main() {
	var cfg Config
	yaml.Help(cfg)
}
```

**Automated Interactive CLI Help Output (`yaml.Help(cfg)`):**
```text
yaml configuration schema documentation:

env:          Application execution environment (default: dev, validate: [choice=dev,stage,prod])
app_name:     Internal runtime deployment monicker (default: api, validate: [minlen=3,maxlen=10])
api_url:      Base target API destination link (env: API_URL, validate: [url,required_if=env:prod])
api_endpoint: Local binding network socket (validate: [endpoint])
timeout:      Global runtime request boundary tracking parameter (default: 5s, validate: [min=1s,max=10m])
crypto:       Security TLS structure configuration layer 
  min_version: Minimum TLS version (env: TLS_MIN_VERSION, default: tls1.3, validate: [choice=tls1.2,tls1.3])
  alpn:        Application-Layer Protocol Negotiation (default: h2,http/1.1, validate: [choice=h2,http/1.1])
server:       Server logging configuration parameters 
  logging:    Server logging configuration parameters 
    level:    Log hierarchy execution level (default: info, validate: [choice=debug,info,warn])
    colors:   Enable ANSI colored logs output 
    timeout:  Internal downstream processing timeout (default: 5s)
  allowed_ips: Whitelisted network remotes (validate: [mincount=1,maxcount=10])
```

---

### Example 2: Choice Whitelist Constraint Violation (Invalid Profile)
**YAML Specification (`config.yaml`):**
```yaml
env: "testing" # Error: "testing" is outside the whitelist bounds [dev, stage, prod]
```
**Returned Application Runtime Error String (`err.Error()`):**
```text
field env: value "testing" is invalid; allowed choices are [dev, stage, prod]
```

---

### Example 3: Multiple Anomalies, Strict Comparisons & Duration Constraints (Invalid Profile)
**YAML Specification (`config.yaml`):**
```yaml
env: "prod"
app_name: "go"         # Error 1: violates minlen=3
api_url: ""            # Error 2: required_if constraint fails under env:prod
timeout: "500ms"       # Error 3: violates strict duration min=1s boundary
server:
  logging:
    level: "debug"     # (Valid)
  allowed_ips: []      # Error 4: violates collection mincount=1
```
**Returned Application Runtime Error Strings Stack:**
```text
field app_name: string length 2 is less than minlen 3
field api_url: is required when field env=prod
field timeout: value 500ms < min 1s
field server.allowed_ips: collection size 0 is less than mincount 1
```

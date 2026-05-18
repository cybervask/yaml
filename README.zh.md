# Smart Go-YAML Engine [![Go CI/CD Pipeline](https://github.com/cybervask/yaml/badges/main/pipeline.svg)](https://github.com/cybervask/yaml/-/pipelines)

其他语言: [English (EN)](README.md) | [Русский (RU)](README.ru.md)

一个高内聚、可直接无缝替换官方 `go.yaml.in/yaml/v4` 的替代包。它完美保留了原生的函数签名和核心配置选项 (`Option`)，但在解析阶段自动集成了深层递归默认值填充 (`default:"value"`) 与严格的基础设施标签校验 (`validate:"..."`) 引擎。

---

## 🛠 配置配置文件示例（有效与无效场景）

### 示例 1：标准应用程序配置文件（有效配置）
**输入 YAML 规范数据 (`config.yaml`)：**
```yaml
env: "prod"
api_url: "https://cybervask.net"
api_host_port: "127.0.0.1:443"
# 'workers' 和 'server.logging.timeout' 被省略，系统将自动安全地回退到默认标记值
server:
  logging:
    colors: true
    level: "warn"
```
**Go 语言应用程序配置模型结构：**
```go
package main

import (
	"fmt"
	"log"
	"github.com/cybervask/yaml"
)

type Logging struct {
	yaml.Includer `yaml:",inline"` // 安全地激活 !include 标签分析器支持
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
**经序列化后的 Go 结构体文本渲染效果（经由 `yaml.Dump`）：**
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

### 示例 2：枚举白名单校验条件冲突（无效配置）
**输入 YAML 规范数据 (`config.yaml`)：**
```yaml
env: "testing" # 错误：“testing”不在预设的白名单允许范围限制内 [dev, stage, prod]
api_url: "https://cybervask.net"
api_host_port: "127.0.0.1:443"
```
**系统抛出的运行时错误字符串文本 (`err.Error()`):**
```text
field Env: value "testing" is invalid; allowed choices are [dev, stage, prod]
```

---

### 示例 3：统一资源地址协议缺失与网络套接字端口越界（无效配置）
**输入 YAML 规范数据 (`config.yaml`)：**
```yaml
env: "dev"
api_url: "cybervask.net:443"  # 错误：完全缺少显式协议标识分隔符 "://"
api_host_port: "127.0.0.1:85000" # 错误：目标网络通信端口号超过合法边界限制 (> 65535)
```
**系统抛出的运行时错误字符串文本 (`err.Error()`):**
```text
field APIUrl: value "cybervask.net:443" is missing a URL scheme separator (e.g., scheme://host)
```
*(提示：如果修正了 `api_url` 字段参数，校验器队列将向下移动进而拦截端口范围越界错误：`field APIHostPort: value "127.0.0.1:85000" contains an invalid port number`)*

---

### 示例 4：结构体标记组合逻辑冲突设计故障（无效的模型架构）
**Go 语言代码模型定义：**
```go
type DefectiveConfig struct {
    Token string `yaml:"token" default:"secret_token" validate:"not_empty"`
}
```
**系统抛出的运行时错误字符串文本 (`err.Error()`):**
```text
field Token is invalid: 'default' and 'not_empty' are mutually exclusive
```

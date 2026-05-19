# Smart Go-YAML Engine

[![Go CI/CD Pipeline](https://github.com/cybervask/yaml/actions/workflows/go.yml/badge.svg)](https://github.com/cybervask/yaml/actions/workflows/go.yml)

其他语言: [English (EN)](README.md) | [Русский (RU)](README.ru.md)

一个高内聚、可直接无缝替换官方 `go.yaml.in/yaml/v4` 的替代包。它完美保留了原生的函数签名和核心配置选项 (`Option`)，但在解析阶段自动集成了深层递归默认值填充 (`default:"value"`)、原生操作系统环境变量注入 (`env:"VAR"`)、自动化命令行帮助文本生成 (`Help()`) 与严格基础设施标签校验 (`validate:"..."`) 引擎。

---

## 可切用的验证规则与特性功能说明

您可以通过逗号分隔符在一个 `validate` 标签中组合多个验证规则（例如：`validate:"not_empty,endpoint"`）。



| 特性与规则标识       | 语法示例                              | 说明                                                                           | 支持的类型                                    |
|:--------------|:----------------------------------|:-----------------------------------------------------------------------------|:-----------------------------------------|
| **环境变量注入**    | `env:"APP_PORT"`                  | **12-Factor App:** 动态注入和读取 OS 变量。其优先级绝对高于 YAML 和 `default` 标记。               | `string`, `bool`, 基础数值                   |
| **必填字段**      | `validate:"not_empty"`            | 确保字段被分配了一个非零值。与 `default` 标签互斥。                                              | `string`, `struct`, `slice`, `map`, 数字类型 |
| **条件关联必填**    | `validate:"required_if=Env:prod"` | **跨字段联合校验：** 依赖另一个字段的值决定当前字段是否必填。支持 `:empty` 与 `:not_empty` 宏。               | `string`, 数字, `bool` 等                   |
| **枚举白名单**     | `validate:"choice=dev,prod"`      | **白名单模式：** 字符串值必须与逗号分隔的标识符完全匹配。递归支持切片元素。                                     | `string`, `[]string`                     |
| **枚举黑名单**     | `validate:"choice=!red,!black"`   | **黑名单模式：** 允许任何字符串值，开头的 `!` 标识符除外。递归支持切片元素。                                  | `string`, `[]string`                     |
| **数值与时间范围**   | `validate:"min=1s,max=10m"`       | 强制执行**包含边界**的上下限限制。原生支持基础数字以及 `time.Duration` 时间间隔。                          | `int`, `uint`, `float`, `time.Duration`  |
| **严格数值比较**    | `validate:"gt=5,lt=10"`           | 强制执行**不包含边界**的严格大于（`>`）和严格小于（`<`）限制。同样支持 `time.Duration`。                    | `int`, `uint`, `float`, `time.Duration`  |
| **字符串长度(字符)** | `validate:"minlen=3,maxlen=20"`   | 强制限制字符串的长度边界。升级为计算 **Unicode 字符数 (Runes)**，而非原生字节数，完美支持多语言。                  | `string`                                 |
| **容器容量限制**    | `validate:"mincount=1,maxcount=5` | 强制限制动态切片（Slice）或映射（Map）中允许的最小和最大元素数量。                                        | `slice`, `map`                           |
| **网络地址格式**    | `validate:"format=ipv4"`          | 校验特定的 IP 网络布局。支持 `format=ip`, `format=ipv4` 和 `format=ipv6`。与 `endpoint` 互斥。 | `string`                                 |
| **UUID 唯一标识** | `validate:"format=uuid"`          | 基于成熟稳定的 `google/uuid` 核心依赖包，强制执行符合 RFC 标准的严格 UUID 格式校验。                      | `string`                                 |
| **正则表达式**     | `validate:"regexp=^[a-z]{2,4}$"`  | 验证字符串布局是否符合正则表达式。防止由于包含逗号而发生解析断裂。                                            | `string`, `[]string`                     |
| **网络终结点**     | `validate:"endpoint"`             | 强制执行标准网络终结点 (`host:port`)。原生检查 **IPv6** 语法和端口边界（1-65535）。                    | `string`                                 |
| **严格的 URL**   | `validate:"url"`                  | 校验统一资源定位符。严格要求显式的协议方案分隔符（如 `http://`, `grpc://`）。                            | `string`                                 |

### 企业级工业架构说明：
* **多错误累积收集引擎 (Error Aggregation)：** 校验引擎在触发第一个错误时不会中断退出。它会完整扫描整棵配置拓扑树，将多处验证不通过的异常聚合成一个结构清晰的错误报告堆栈返回。
* **网络格式互斥限制规则：** 为防止结构语义发生逻辑畸变，禁止在同一字段上组合使用多种网络地址校验规则（如 `format=ip`, `format=ipv4`, `format=ipv6`, `endpoint`, `url`）。
* **配置安全防御层：** 凡是逻辑不合法的标签配置（如 `min=10, max=5`, `minlen=5, maxlen=2` 或同时使用 `min` 和 `gt`），均会在程序启动初期的标签编译阶段被拦截并直接抛出初始化错误。

---

## 配置配置文件示例（有效与无效场景）

### 示例 1：集成自动化命令行帮助信息输出的标准配置（有效场景）
**输入 YAML 规范数据 (`config.yaml`)：**
```yaml
env: "prod"
app_name: "gateway"
api_url: "https://cybervask.net"
api_endpoint: "127.0.0.1:443"
server_ip: "192.168.1.10"
cluster_id: "9f8e7d6c-5b4a-3f2e-1d0c-9b8a7f6e5d4c"
server:
  logging:
    colors: true
    level: "warn"
  allowed_ips:
    - "192.168.1.1"
    - "10.0.0.1"
```

**Go 语言应用程序配置模型结构：**
```go
package main

import (
	"fmt"
	"log"
	"time"

	"github.com/cybervask/yaml"
)

type TLS struct {
	MinVersion string   `yaml:"min_version" default:"tls1.3" validate:"choice=tls1.2,tls1.3" env:"TLS_MIN_VERSION" description:"支持的最小 TLS 协议版本"`
	Alpn       []string `yaml:"alpn" default:"h2,http/1.1" validate:"choice=h2,http/1.1" description:"应用层协议协商 ALPN 列表"`
}

type Logging struct {
	Level         string           `yaml:"level" default:"info" validate:"choice=debug,info,warn"`
	Colors        bool             `yaml:"colors"`
	Timeout       string           `yaml:"timeout" default:"5s"`
}

type Config struct {
	Env          string        `yaml:"env" default:"dev" validate:"choice=dev,stage,prod" description:"程序运行时核心环境状态配置"`
	AppName      string        `yaml:"app_name" default:"api" validate:"minlen=3,maxlen=10" description:"程序内部标识别名"`
	APIUrl       string        `yaml:"api_url" validate:"url,required_if=Env:prod" env:"API_URL" description:"目标 API 全局基础网关分发链路地址"`
	APIEndpoint  string        `yaml:"api_endpoint" validate:"endpoint" description:"本地通信网络终结点绑定接口"`
	Timeout      time.Duration `yaml:"timeout" default:"5s" validate:"min=1s,max=10m" description:"全局请求超时处理时间"`
	ServerIP     string        `yaml:"server_ip" validate:"format=ipv4" description:"服务器静态绑定 IPv4 主机地址"`
	ClusterID    string        `yaml:"cluster_id" validate:"format=uuid" description:"唯一的集群多节点 UUID 标识符"`
	Crypto       TLS           `yaml:"crypto" description:"TLS 加密通信安全架构配置节点"`
	Server       struct {
		Logging    Logging  `yaml:"logging" description:"服务端通用日志记录器参数"`
		AllowedIPs []string `yaml:"allowed_ips" validate:"mincount=1,maxcount=10" description:"信任的远程连接白名单"`
	} `yaml:"server"`
}

func main() {
	var cfg Config
    
	// 无论何时，您均可在业务体系中轻松调用并输出格式化、自动排列整齐的基础框架帮助详情文档（例如应对 --help / -h 传参）
	yaml.Help(cfg)
}
```

**自动生成的交互式命令行 CLI 帮助输出详情 (`yaml.Help(cfg)`):**
```text
yaml configuration schema documentation:

env:          程序运行时核心环境状态配置 (default: dev, validate: [choice=dev,stage,prod])
app_name:     程序内部标识别名 (default: api, validate: [minlen=3,maxlen=10])
api_url:      目标 API 全局基础网关分发链路地址 (env: API_URL, validate: [url,required_if=Env:prod])
api_endpoint: 本地通信网络终结点绑定接口 (validate: [endpoint])
timeout:      全局请求超时处理时间 (default: 5s, validate: [min=1s,max=10m])
server_ip:    服务器静态绑定 IPv4 主机地址 (validate: [format=ipv4])
cluster_id:   唯一的集群多节点 UUID 标识符 (validate: [format=uuid])
crypto:       TLS 加密通信安全架构配置节点 
  min_version: 支持的最小 TLS 协议版本 (env: TLS_MIN_VERSION, default: tls1.3, validate: [choice=tls1.2,tls1.3])
  alpn:        应用层协议协商 ALPN 列表 (default: h2,http/1.1, validate: [choice=h2,http/1.1])
server:       服务端通用日志记录器参数 
  logging:    服务端通用日志记录器参数 
    level:    Log level (default: info, validate: [choice=debug,info,warn])
    colors:   Colors 
    timeout:  Timeout (default: 5s)
  allowed_ips: 信任的远程连接白名单 (validate: [mincount=1,maxcount=10])
```

---

### 示例 2：枚举白名单校验条件冲突（无效场景）
**输入 YAML 规范数据 (`config.yaml`)：**
```yaml
env: "testing" # 错误：“testing”不在预设的白名单允许范围限制内 [dev, stage, prod]
```
**系统抛出的运行时错误字符串文本 (`err.Error()`):**
```text
field Env: value "testing" is invalid; allowed choices are [dev, stage, prod]
```

---

### 示例 3：多错误拦截收集、字符串长度与容量规则触发拦截（无效场景）
**输入 YAML 规范数据 (`config.yaml`)：**
```yaml
env: "prod"
app_name: "go"      # 错误 1：字符长度为 2，违反了 minlen=3 的限制
api_url: ""         # 错误 2：条件触发必填 (Env:prod) 却未传值
server_ip: "999.9"  # 错误 3：无效的 IPv4 网络布局格式
server:
  allowed_ips: []   # 错误 4：元素数量为 0，违反了 mincount=1 的限制
```
**系统抛出的运行时多错误累积聚合字符串文本 (`err.Error()`):**
```text
field AppName: string length 2 is less than minlen 3
field APIUrl: is required when field Env is prod
field ServerIP: value "999.9" is not a valid IPv4 address
field Server.AllowedIPs: collection size 0 is less than mincount 1
```

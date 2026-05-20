# Smart Go-YAML Engine

[![Go CI/CD Pipeline](https://github.com/cybervask/yaml/actions/workflows/go.yml/badge.svg)](https://github.com/cybervask/yaml/actions/workflows/go.yml)

Другие языки: [English (EN)](README.md) | [中文 (ZH)](README.zh.md)

Надежная и высокопроизводительная бесшовная замена для `go.yaml.in/yaml/v4`. Пакет полностью сохраняет оригинальные сигнатуры функций и ключевые опции (`Option`), автоматически интегрируя глубокую рекурсивную подстановку значений по умолчанию (`default:"value"`), нативную инъекцию переменных окружения ОС (`env:"VAR"`), автоматическую генерацию справочного текста для CLI (`Help()`) и строгую валидацию полей на основе тегов (`validate:"..."`) прямо на уровне парсинга.

---

## Доступные правила валидации и возможности

Вы можете комбинировать несколько правил валидации внутри одного тега `validate`, используя запятую в качестве разделителя (например, `validate:"not_empty,endpoint"`).



| Правило / Возможность     | Пример синтаксиса                  | Описание                                                                                                                             | Поддерживаемые типы                       |
|:--------------------------|:-----------------------------------|:-------------------------------------------------------------------------------------------------------------------------------------|:------------------------------------------|
| **Инъекция окружения**    | `env:"APP_PORT"`                   | **12-Factor App:** Динамически внедряет переменные среды системы. Имеет абсолютный приоритет над значениями из YAML и тегов default. | `string`, `bool`, примитивные числа       |
| **Обязательное поле**     | `validate:"not_empty"`             | Гарантирует, что полю присвоено ненулевое значение. Взаимоисключающе с тегом `default`.                                              | `string`, `struct`, `slice`, `map`, числа |
| **Условная валидация**    | `validate:"required_if=role:!srv"` | **Кросс-полевая проверка:** Поле становится обязательным на основе значения соседа. Поддерживает оператор отрицания `!` и макросы.   | `string`, числа, `bool`, указатели и др.  |
| **Белый список (Выбор)**  | `validate:"choice=dev,prod"`       | **Режим whitelist:** Строковое значение должно строго соответствовать одному из токенов. Рекурсивно поддерживает элементы слайсов.   | `string`, `[]string`                      |
| **Черный список (Выбор)** | `validate:"choice=!red,!black"`    | **Режим blacklist:** Разрешает любое строковое значение, кроме специфических исключений, начинающихся с `!`.                         | `string`, `[]string`                      |
| **Диапазоны и Время**     | `validate:"min=1s,max=10m"`        | Задает **включающие** нижнюю (`>=`) и верхнюю (`<=`) числовые границы. Нативно поддерживает числа и интервалы `time.Duration`.       | `int`, `uint`, `float`, `time.Duration`   |
| **Строгое сравнение**     | `validate:"gt=5,lt=10"`            | Задает **исключающие** строгие границы: больше (`>`) и меньше (`<`). Поддерживает `time.Duration`.                                   | `int`, `uint`, `float`, `time.Duration`   |
| **Длина строки (Руны)**   | `validate:"minlen=3,maxlen=20"`    | Задает лимиты на длину строки. Профессионально учитывает **Unicode руны (символы)**, а не сырые байты.                               | `string`                                  |
| **Размер коллекции**      | `validate:"mincount=1,maxcount=5`  | Задает минимальное и максимальное количество элементов внутри динамических коллекций.                                                | `slice`, `map`                            |
| **Сетевые форматы**       | `validate:"format=ipv4"`           | Валидирует IP сети. Поддерживает `format=ip`, `format=ipv4` и `format=ipv6`. Взаимоисключающе с опцией `endpoint`.                   | `string`                                  |
| **Проверка UUID**         | `validate:"format=uuid"`           | Строгая верификация уникальных идентификаторов по спецификации RFC с использованием нативного пакета `google/uuid`.                  | `string`                                  |
| **Регулярное выражение**  | `validate:"regexp=^[a-z]{2,4}$"`   | Проверяет соответствие строки шаблону регулярного выражения. Безопасно к запятым внутри паттерна.                                    | `string`, `[]string`                      |
| **Сетевой эндпоинт**      | `validate:"endpoint"`              | Проверяет стандартные сетевые адреса (`host:port`). Нативно валидирует синтаксис **IPv6** и границы портов (1-65535).                | `string`                                  |
| **Проверка URL**          | `validate:"url"`                   | Проверяет валидность URI. Строго требует явного указания разделителя протокола (схемы) (например, `http://`, `grpc://`).             | `string`                                  |

---

## Профили конфигурации (Валидные и невалидные случаи)

### Пример 1: Стандартный конфиг приложения с выводом справки (Валидный профиль)
**Спецификация YAML (`config.yaml`):**
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

**Модель конфигурации Go-приложения:**
```go
package main

import (
	"fmt"
	"time"

	"github.com/cybervask/yaml"
)

type TLS struct {
	MinVersion string   `yaml:"min_version" default:"tls1.3" validate:"choice=tls1.2,tls1.3" env:"TLS_MIN_VERSION" description:"Минимальная поддерживаемая версия TLS"`
	Alpn       []string `yaml:"alpn" default:"h2,http/1.1" validate:"choice=h2,http/1.1" description:"Application-Layer Protocol Negotiation"`
}

type Logging struct {
	Level         string           `yaml:"level" default:"info" validate:"choice=debug,info,warn" description:"Уровень выполнения иерархии логов"`
	Colors        bool             `yaml:"colors" description:"Включить ANSI цветной вывод логов"`
	Timeout       string           `yaml:"timeout" default:"5s" description:"Внутренний таймаут обработки запроса"`
}

type Config struct {
	Env          string        `yaml:"env" default:"dev" validate:"choice=dev,stage,prod" description:"Окружение выполнения приложения"`
	AppName      string        `yaml:"app_name" default:"api" validate:"minlen=3,maxlen=10" description:"Внутреннее имя (моникер) приложения"`
	APIUrl       string        `yaml:"api_url" validate:"url,required_if=env:prod" env:"API_URL" description:"Базовый URL целевого API"`
	APIEndpoint  string        `yaml:"api_endpoint" validate:"endpoint" description:"Локальный сетевой эндпоинт для привязки сокета"`
	Timeout      time.Duration `yaml:"timeout" default:"5s" validate:"min=1s,max=10m" description:"Глобальный таймаут отслеживания жизненного цикла запроса"`
	Crypto       TLS           `yaml:"crypto" description:"Слой конфигурации структуры безопасности TLS"`
	Server       struct {
		Logging    Logging  `yaml:"logging" description:"Параметры логирования сервера"`
		AllowedIPs []string `yaml:"allowed_ips" validate:"mincount=1,maxcount=10" description:"Белый список IP-адресов удаленных узлов"`
	} `yaml:"server"`
}

func main() {
	var cfg Config
	yaml.Help(cfg)
}
```

**Автоматический интерактивный вывод CLI-справки (`yaml.Help(cfg)`):**
```text
yaml configuration schema documentation:

env:          Окружение выполнения приложения (default: dev, validate: [choice=dev,stage,prod])
app_name:     Внутреннее имя (моникер) приложения (default: api, validate: [minlen=3,maxlen=10])
api_url:      Базовый URL целевого API (env: API_URL, validate: [url,required_if=env:prod])
api_endpoint: Локальный сетевой эндпоинт для привязки сокета (validate: [endpoint])
timeout:      Глобальный таймаут отслеживания жизненного цикла запроса (default: 5s, validate: [min=1s,max=10m])
crypto:       Слой конфигурации структуры безопасности TLS 
  min_version: Минимальная поддерживаемая версия TLS (env: TLS_MIN_VERSION, default: tls1.3, validate: [choice=tls1.2,tls1.3])
  alpn:        Application-Layer Protocol Negotiation (default: h2,http/1.1, validate: [choice=h2,http/1.1])
server:       Параметры логирования сервера 
  logging:    Параметры логирования сервера 
    level:    Уровень выполнения иерархии логов (default: info, validate: [choice=debug,info,warn])
    colors:   Включить ANSI цветной вывод логов 
    timeout:  Внутренний таймаут обработки запроса (default: 5s)
  allowed_ips: Белый список IP-адресов удаленных узлов (validate: [mincount=1,maxcount=10])
```

---

### Пример 2: Нарушение правил белого списка Choice (Невалидный профиль)
**Спецификация YAML (`config.yaml`):**
```yaml
env: "testing" # Ошибка: значение "testing" выходит за рамки белого списка [dev, stage, prod]
```
**Строка ошибки рантайма приложения (`err.Error()`):**
```text
field env: value "testing" is invalid; allowed choices are [dev, stage, prod]
```

---

### Пример 3: Накопление множественных ошибок, лимитов строк, коллекций и времени (Невалидный профиль)
**Спецификация YAML (`config.yaml`):**
```yaml
env: "prod"
app_name: "go"         # Ошибка 1: длина в символах меньше minlen=3
api_url: ""            # Ошибка 2: условное правило выполнилось (env:prod), но поле пустое
timeout: "500ms"       # Ошибка 3: нарушено строгое ограничение по времени min=1s
server:
  logging:
    level: "debug"     # (Валидно)
  allowed_ips: []      # Ошибка 4: размер коллекции равен 0, что нарушает mincount=1
```
**Строка агрегированной ошибки рантайма приложения (`err.Error()`):**
```text
field app_name: string length 2 is less than minlen 3
field api_url: is required when field env=prod
field timeout: value 500ms < min 1s
field server.allowed_ips: collection size 0 is less than mincount 1
```

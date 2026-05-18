# Smart Go-YAML Engine

[![Go CI/CD Pipeline](https://github.com/cybervask/yaml/actions/workflows/go.yml/badge.svg)](https://github.com/cybervask/yaml/actions/workflows/go.yml)

На других языках: [English (EN)](README.md) | [中文 (ZH)](README.zh.md)

Высокопроизводительная drop-in замена для официального пакета `go.yaml.in/yaml/v4`. Библиотека полностью сохраняет оригинальные сигнатуры функций и опции конфигурации (`Option`), но автоматически интегрирует глубокое рекурсивное заполнение значений по умолчанию (`default:"значение"`) и жесткую инфраструктурную валидацию параметров (`validate:"..."`) прямо на этапе парсинга.

---

## Доступные правила валидации

Вы можете комбинировать несколько правил внутри одного тега `validate`, разделяя их запятой (например, `validate:"not_empty,host_port"`).


| Правило                  | Пример синтаксиса                | Описание                                                                                                                 | Поддерживаемые типы                       |
|:-------------------------|:---------------------------------|:-------------------------------------------------------------------------------------------------------------------------|:------------------------------------------|
| **Обязательное поле**    | `validate:"not_empty"`           | Гарантирует, что поле заполнено не нулевым значением. Логически несовместимо с тегом `default`.                          | `string`, `struct`, `slice`, `map`, числа |
| **Белый список**         | `validate:"choice=dev,prod"`     | **Whitelist-режим:** строковое значение должно строго соответствовать одному из перечисленных вариантов.                 | `string`                                  |
| **Черный список**        | `validate:"choice=!red,!black"`  | **Blacklist-режим:** разрешает любые строковые значения, кроме исключений, помеченных знаком `!`.                        | `string`                                  |
| **Числовой диапазон**    | `validate:"min=1,max=100"`       | Проверяет нижнюю и верхнюю числовые границы. Автоматически запрещает отрицательные лимиты для `uint`.                    | `int`, `uint`, `float` всех разрядов      |
| **Регулярное выражение** | `validate:"regexp=^[a-z]{2,4}$"` | Проверяет соответствие строки регулярному выражению. Устойчив к запятым внутри паттерна.                                 | `string`                                  |
| **Сетевой адрес**        | `validate:"host_port"`           | Валидирует сетевые эндпоинты (`host:port`). Из коробки поддерживает **IPv6** и проверку диапазона портов (1-65535).      | `string`                                  |
| **Схемный URL**          | `validate:"url"`                 | Проверяет корректность URL-адресов. Строго требует явного указания протокола (например, `http://`, `grpc://`, `nfs://`). | `string`                                  |

---

## Примеры конфигураций (Валидные и Невалидные кейсы)

### Пример 1: Стандартный конфигурационный файл (Валидный кейс)
**Исходный YAML (`config.yaml`):**
```yaml
env: "prod"
api_url: "https://cybervask.net"
api_host_port: "127.0.0.1:443"
# Параметры 'workers' и 'server.logging.timeout' пропущены и автоматически заполнятся дефолтами
server:
  logging:
    colors: true
    level: "warn"
```
**Модель структуры конфигурации в приложении Go:**
```go
type Logging struct {
	yaml.Includer `yaml:",inline"` // Безопасно активирует поддержку тега !include
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
**Итоговый сериализованный YAML (после обработки `yaml.Dump`):**
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

### Пример 2: Нарушение условий белого списка выбора Choice (Невалидный кейс)
**Исходный YAML (`config.yaml`):**
```yaml
env: "testing" # Ошибка: "testing" не входит в разрешенный белый список [dev, stage, prod]
api_url: "https://cybervask.net"
api_host_port: "127.0.0.1:443"
```
**Строковый текст возвращаемой ошибки (`err.Error()`):**
```text
field Env: value "testing" is invalid; allowed choices are [dev, stage, prod]
```

---

### Пример 3: Отсутствие схем URL и выход за границы сетевых портов (Невалидный кейс)
**Исходный YAML (`config.yaml`):**
```yaml
env: "dev"
api_url: "cybervask.net:443"  # Ошибка: Полностью отсутствует разделитель схемы "://"
api_host_port: "127.0.0.1:85000" # Ошибка: Номер сетевого порта превышает лимит (> 65535)
```
**Строковый текст возвращаемой ошибки (`err.Error()`):**
```text
field APIUrl: value "cybervask.net:443" is missing a URL scheme separator (e.g., scheme://host)
```

---

### Пример 4: Критическая ошибка архитектурного проектирования тегов (Невалидный кейс)
**Определение структуры в коде Go:**
```go
type DefectiveConfig struct {
Token string `yaml:"token" default:"secret_token" validate:"not_empty"`
}
```
**Строковый текст возвращаемой ошибки (`err.Error()`):**
```text
field Token is invalid: 'default' and 'not_empty' are mutually exclusive
```

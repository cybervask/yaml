# Smart Go-YAML Engine

[![Go CI/CD Pipeline](https://github.com/cybervask/yaml/actions/workflows/go.yml/badge.svg)](https://github.com/cybervask/yaml/actions/workflows/go.yml)

На других языках: [English (EN)](README.md) | [中文 (ZH)](README.zh.md)

Высокопроизводительная drop-in замена для официального пакета `go.yaml.in/yaml/v4`. Библиотека полностью сохраняет оригинальные сигнатуры функций и опции конфигурации (`Option`), но автоматически интегрирует глубокое рекурсивное заполнение значений по умолчанию (`default:"значение"`), нативную инжекцию системных переменных окружения (`env:"ПЕРЕМЕННАЯ"`), автогенерацию интерактивной CLI-справки (`Help()`) и жесткую инфраструктурную валидацию параметров (`validate:"..."`) прямо на этапе парсинга.

---

## Доступные правила валидации и возможности

Вы можете комбинировать несколько правил внутри одного тега `validate`, разделяя их запятой (например, `validate:"not_empty,host_port"`).



| Правило / Возможность    | Пример синтаксиса                | Описание                                                                                                                        | Поддерживаемые типы                       |
|:-------------------------|:---------------------------------|:--------------------------------------------------------------------------------------------------------------------------------|:------------------------------------------|
| **Инжекция окружения**   | `env:"APP_PORT"`                 | **12-Factor App:** Динамически считывает переменные ОС. Имеет абсолютный приоритет над YAML и тегами `default`.                 | `string`, `bool`, примитивные числа       |
| **Обязательное поле**    | `validate:"not_empty"`           | Гарантирует, что поле заполнено не нулевым значением. Логически несовместимо с тегом `default`.                                 | `string`, `struct`, `slice`, `map`, числа |
| **Белый список**         | `validate:"choice=dev,prod"`     | **Whitelist-режим:** строковое значение должно строго соответствовать одному из токенов. Рекурсивно проверяет элементы слайсов. | `string`, `[]string`                      |
| **Черный список**        | `validate:"choice=!red,!black"`  | **Blacklist-режим:** разрешает любые строковые значения, кроме исключений, помеченных знаком `!`.                               | `string`, `[]string`                      |
| **Числовой диапазон**    | `validate:"min=1,max=100"`       | Проверяет нижнюю и верхнюю числовые границы. Автоматически запрещает отрицательные лимиты для `uint`.                           | `int`, `uint`, `float` всех разрядов      |
| **Регулярное выражение** | `validate:"regexp=^[a-z]{2,4}$"` | Проверяет соответствие строки регулярному выражению. Устойчив к запятым внутри паттерна.                                        | `string`, `[]string`                      |
| **Сетевой адрес**        | `validate:"host_port"`           | Валидирует сетевые эндпоинты (`host:port`). Из коробки поддерживает **IPv6** и проверку диапазона портов (1-65535).             | `string`                                  |
| **Схемный URL**          | `validate:"url"`                 | Проверяет корректность URL-адресов. Строго требует явного указания протокола (например, `http://`, `grpc://`, `nfs://`).        | `string`                                  |

---

## Примеры конфигураций (Валидные и Невалидные кейсы)

### Пример 1: Стандартный конфигурационный файл с выводом интерактивной справки (Валидный кейс)
**Исходный YAML (`config.yaml`):**
```yaml
env: "prod"
api_url: "https://cybervask.net"
api_host_port: "127.0.0.1:443"
# Параметры 'crypto.alpn' и 'server.logging.timeout' пропущены и автоматически заполнятся дефолтами
server:
  logging:
    colors: true
    level: "warn"
```
**Модель структуры конфигурации в приложении Go:**
```go
package main

import (
	"fmt"
	"log"
	"github.com/cybervask/yaml"
)

type TLS struct {
	MinVersion string   `yaml:"min_version" default:"tls1.3" validate:"choice=tls1.2,tls1.3" env:"TLS_MIN_VERSION" description:"Минимальная версия протокола TLS"`
	Alpn       []string `yaml:"alpn" default:"h2,http/1.1" validate:"choice=h2,http/1.1" description:"Список согласования протоколов прикладного уровня ALPN"`
}

type Logging struct {
	Level         string           `yaml:"level" default:"info" validate:"choice=debug,info,warn"`
	Colors        bool             `yaml:"colors"`
	Timeout       string           `yaml:"timeout" default:"5s"`
}

type Config struct {
	Env         string  `yaml:"env" default:"dev" validate:"choice=dev,stage,prod" description:"Окружение запуска приложения"`
	APIUrl      string  `yaml:"api_url" validate:"url" env:"API_URL" description:"Базовый URL адрес целевого API"`
	APIHostPort string  `yaml:"api_host_port" validate:"host_port" description:"Локальный сетевой сокет для биндинга"`
	Crypto      TLS     `yaml:"crypto" description:"Слой конфигурации криптографической структуры TLS"`
	Server      struct {
		Logging Logging `yaml:"logging" description:"Параметры логирования сервера"`
	} `yaml:"server"`
}

func main() {
	var cfg Config
    
	// Вы можете красиво вывести автоматически выровненную документацию схемы в консоль (например, по флагу --help / -h)
	yaml.Help(cfg)
}
```

**Сгенерированный автоматический CLI-вывод справки (`yaml.Help(cfg)`):**
```text
yaml configuration schema documentation:

env:           Окружение запуска приложения (default: dev, validate: [choice=dev,stage,prod])
api_url:       Базовый URL адрес целевого API (env: API_URL, validate: [url])
api_host_port: Локальный сетевой сокет для биндинга (validate: [host_port])
crypto:        Слой конфигурации криптографической структуры TLS 
  min_version: Минимальная версия протокола TLS (env: TLS_MIN_VERSION, default: tls1.3, validate: [choice=tls1.2,tls1.3])
  alpn:        Список согласования протоколов прикладного уровня ALPN (default: h2,http/1.1, validate: [choice=h2,http/1.1])
server:        Параметры логирования сервера 
  logging:     Параметры логирования сервера 
    level:     Log level (default: info, validate: [choice=debug,info,warn])
    colors:    Colors 
    timeout:   Timeout (default: 5s)
```

---

### Пример 2: Нарушение условий белого списка выбора Choice (Невалидный кейс)
**Исходный YAML (`config.yaml`):**
```yaml
env: "testing" # Ошибка: "testing" не входит в разрешенный белый список [dev, stage, prod]
```
**Строковый текст возвращаемой ошибки (`err.Error()`):**
```text
field Env: value "testing" is invalid; allowed choices are [dev, stage, prod]
```

---

### Пример 3: Приоритет инжекции ОС переменных окружения и проверка схем (Невалидный кейс)
**Экспорт системных переменных ОС:**
```bash
export API_URL="cybervask.net:443" # Жестко переопределяет значение из YAML, но падает, так как пропущен разделитель "://"
```
**Строковый текст возвращаемой ошибки (`err.Error()`):**
```text
field APIUrl: value "cybervask.net:443" is missing a URL scheme separator (e.g., scheme://host)
```

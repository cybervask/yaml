# Smart Go-YAML Engine

[![Go CI/CD Pipeline](https://github.com/cybervask/yaml/actions/workflows/go.yml/badge.svg)](https://github.com/cybervask/yaml/actions/workflows/go.yml)

Otros idiomas: [English (EN)](README.md) | [Русский (RU)](README.ru.md) | [中文 (ZH)](README.zh.md) | [日本語 (JA)](README.ja.md)

Un reemplazo directo (drop-in) robusto y de alto rendimiento para `go.yaml.in/yaml/v4`. Conserva las firmas de funciones originales y las opciones principales (`Option`) mientras integra automáticamente en la capa de análisis la asignación de valores por defecto profunda y recursiva (`default:"value"`), la inyección nativa de variables de entorno del sistema operativo (`env:"VAR"`), la generación automatizada de ayuda para la CLI (`Help()`) y una validación estricta basada en etiquetas (`validate:"..."`).

---

## Reglas de validación y funcionalidades disponibles

Puede combinar varias reglas de validación dentro de la etiqueta `validate` usando una coma como separador (por ejemplo, `validate:"not_empty,endpoint"`).

> **Nota sobre estructuras cíclicas:** las estructuras cíclicas de punteros construidas programáticamente (por ejemplo, un nodo que se referencia a sí mismo) no bloquean `Validate` — la recursión a lo largo de la ruta actual devuelve un error explícito `cyclic structure detected` para el campo que referencia. Los subobjetos compartidos (acíclicos) referenciados desde varios campos siguen siendo válidos. En la salida de `Help`, un tipo autorreferenciado se documenta una vez por ruta y la recursión se omite silenciosamente.

> **Nota sobre los límites de frontera:** un valor de límite que no se puede analizar (por ejemplo, `validate:"minlen=abc,maxlen=5"`) se informa como un error de configuración explícito que nombra la regla (`invalid validator configuration: minlen "abc": ...`) — un límite inválido nunca desactiva silenciosamente su comprobación. Lo mismo se aplica a las reglas de frontera (`min`, `max`, `gt`, `lt`) colocadas sobre tipos no numéricos (por ejemplo, `bool`): la comprobación no puede aplicarse, por lo que se informa como error de configuración en lugar de omitirse silenciosamente.



| Regla / Funcionalidad     | Ejemplo de sintaxis                | Descripción                                                                                                                                | Tipos admitidos                             |
|:--------------------------|:-----------------------------------|:-------------------------------------------------------------------------------------------------------------------------------------------|:--------------------------------------------|
| **Inyección de entorno**  | `env:"APP_PORT"`                   | **Aplicación 12-Factor:** inyecta dinámicamente variables de entorno del sistema. Por defecto, la variable de entorno tiene prioridad absoluta sobre los valores YAML y los valores por defecto de las etiquetas; el arbitraje de fuentes está disponible mediante la etiqueta `priority` (véase **Prioridad de fuente**). Las etiquetas `default` y `env` se aplican a campos escalares. Admite valores `time.Duration`. En las rutas de carga (`LoadFile`, `Load`, `Loader.Load`, `Decoder.Decode`) un valor de entorno inválido se informa como error; la llamada independiente `SetDefaults` conserva su comportamiento silencioso heredado — un valor de entorno inválido deja el campo a cero sin error y la etiqueta `default` no se aplica (la única excepción: un literal `time.Duration` inválido se informa como error). | `string`, `bool`, números primitivos, `time.Duration` |
| **Prioridad de fuente**  | `priority:"value"`                 | **Etiqueta de arbitraje de entorno:** se aplica a los campos que también llevan una etiqueta `env`; los valores son `value` o `env`. Por defecto (sin etiqueta o con `priority:"env"`) la variable de entorno tiene prioridad absoluta sobre el valor YAML y la etiqueta `default`. Con `priority:"value"`, un campo que queda distinto de cero tras los valores por defecto y la decodificación — un valor YAML decodificado, una etiqueta `default` no nula aplicada o un valor prellenado programáticamente — gana a una variable de entorno no vacía; si solo hay una fuente presente, se aplica independientemente de la etiqueta. Un campo que sigue a cero tras los valores por defecto y la decodificación (un cero explícito en YAML o una etiqueta `default` con valor cero como `default:"0"`, `default:"false"`, `default:""`) no cuenta como valor para el arbitraje — se aplica la variable de entorno. Los valores de etiqueta desconocidos y una etiqueta `priority` sin `env` se informan como errores de configuración, independientemente de si hay un valor o una variable presente. Como `default` y `env`, la etiqueta `priority` se aplica a campos escalares; sobre un campo no escalar (struct, slice, map) se informa como error de configuración. | `string`, `bool`, números primitivos, `time.Duration` |
| **Campo obligatorio**     | `validate:"required"`              | Garantiza que el campo tenga asignado un valor distinto de cero. Mutuamente excluyente con la etiqueta `default`. `not_empty` es un alias heredado (conservado por compatibilidad con versiones anteriores, previsto para su deprecación). | `string`, `struct`, `slice`, `map`, números |
| **Requerido condicional** | `validate:"required_if=role!=srv"` | **Validación cruzada de campos:** el campo se vuelve obligatorio según el valor de otro campo. Operadores canónicos: `==` (igual), `!=` (distinto) — la comparación no distingue mayúsculas; `~=` (coincidencia de regexp) y `^=` (no coincidencia); los operandos de los operadores canónicos pueden ir entre comillas (`'...'` / `"..."`) — las comillas se eliminan. Los nombres de destino simples se resuelven primero en la estructura contenedora y luego en la raíz; las rutas con puntos (`required_if=server.mode==prod`) direccionan desde la raíz; los destinos puntero se desreferencian. Los destinos inexistentes, los destinos no exportados y los patrones inválidos se informan como errores de configuración. Un valor `required_if` sin ningún operador reconocido se informa como error de configuración. La forma heredada con dos puntos (`key:value`, `key:!value`, macros `:empty`/`:not_empty`) está en desuso, pero sigue admitiéndose; los átomos heredados y los canónicos pueden mezclarse en una misma expresión. Se admiten múltiples condiciones: combínelas con `&&` (AND) y `\|\|` (OR), y agrúpelas con paréntesis — `&&` tiene mayor precedencia que `\|\|`. Las condiciones se analizan como una expresión solo cuando `&&` o `\|\|` está presente fuera de las comillas, o cuando el valor empieza por un `(` sin comillas — los paréntesis por sí solos pueden envolver una condición única; de lo contrario, la sintaxis y los mensajes de condición única permanecen sin cambios. Los espacios alrededor de `&&`, `\|\|` y los paréntesis se ignoran; los espacios alrededor de los operadores de comparación (`==`, `!=`, `~=`, `^=`) dentro de un átomo no están permitidos — un nombre de destino con espacio no se resuelve y se informa como `target field "..." not found`. Un operando que contenga `&&` o `\|\|` debe ir entre comillas: `webhook.url~='(a\|b)'`; un segmento entre comillas es literal. Los paréntesis dentro de un operando (tras el operador de comparación, p. ej. `tag~=(a\|b)`) no necesitan comillas. Declarar `required_if` dos veces en un mismo campo es un error de configuración — combine las condiciones con `&&` o `\|\|` en una única expresión. Un valor legacy obsoleto que contenga `&&` o `\|\|` se analiza como una expresión (p. ej. `key:a&&b` se convierte en el AND de `key:a` y un átomo malformado), no como una comparación literal única. Las reglas cuantitativas (`min`, `max`, `minlen`, `maxlen`, `mincount`, `maxcount`) del mismo campo se comprueban independientemente del resultado de la expresión — `minlen` en un campo vacío se informa incluso cuando la expresión es falsa. La evaluación de la expresión es exhaustiva: los errores de forma (paréntesis desbalanceados, átomos vacíos) y los errores de configuración de los átomos se informan independientemente del valor del campo y del resultado combinado, y un átomo defectuoso aporta false; un mismo error de átomo repetido se informa una sola vez. Una violación de la expresión se informa como `field <nombre>: is required when <expresión>` (p. ej. `field key_file: is required when mode==webhook&&(url~=^https:\|\|cert^=^$)`). | `string`, números, `bool`, punteros, etc.  |
| **Sección condicional** | `validate:"required_if=telegram.mode==webhook,break,url,minlen=10"` | **Sección condicional:** las reglas de valor colocadas después de `break` se evalúan solo cuando se cumple la condición del required_if precedente; en los demás modos el campo no se comprueba en absoluto — incluidos los valores no vacíos (una compensación deliberada: una sección que no aplica en el modo actual no se valida). La cola se analiza siempre: los errores de configuración de las reglas después de `break` (p. ej. minlen=abc) se informan independientemente de la condición — el informe tiene la forma `field url: invalid validator configuration: minlen "abc": ...`. Un `break` sin un `required_if` precedente, un segundo `break`, un `required_if` después de `break` y un `break` dentro de una regla de ruta son errores de configuración. Las reglas condicionales de las colecciones slice/map no se heredan a sus elementos. | `string`, números, `bool`, punteros, etc. |
| **Lista blanca (elección)** | `validate:"choice=dev,prod"`     | **Modo lista blanca:** el valor de cadena debe coincidir exactamente con uno de los tokens separados por comas. Admite elementos de slices recursivamente. | `string`, `[]string`                        |
| **Lista negra (elección)** | `validate:"choice=!red,!black"`   | **Modo lista negra:** permite cualquier valor de cadena excepto los tokens de exclusión específicos prefijados con `!`.                                   | `string`, `[]string`                        |
| **Numérico y duración**   | `validate:"min=1s,max=10m"`        | Impone límites inferior (`>=`) y superior (`<=`) **inclusivos** atómicos. Admite nativamente números e intervalos `time.Duration`.           | `int`, `uint`, `float`, `time.Duration`     |
| **Comparación estricta**  | `validate:"gt=5,lt=10"`            | Impone límites **exclusivos** estrictos de mayor que (`>`) y menor que (`<`). También admite `time.Duration`.                              | `int`, `uint`, `float`, `time.Duration`     |
| **Longitud de cadena (runas)** | `validate:"minlen=3,maxlen=20"` | Impone límites mínimo y máximo sobre la longitud de la cadena. Cuenta profesionalmente **runas Unicode** en lugar de bytes crudos.         | `string`                                    |
| **Capacidad de colección** | `validate:"mincount=1,maxcount=5"` | Impone el número mínimo y máximo de elementos dentro de colecciones dinámicas.                                                              | `slice`, `map`                              |
| **Formatos de red**       | `validate:"format=ipv4"`           | Valida diseños específicos de redes IP. Admite `format=ip`, `format=ipv4` y `format=ipv6`. Mutuamente excluyente con `endpoint` y `url`. | `string`                                    |
| **Codificación hexadecimal** | `validate:"format=hex"`         | Valida cadenas hexadecimales: decodificables mediante `hex.DecodeString` con un número par de dígitos. Las longitudes impares y los caracteres no hexadecimales se rechazan. | `string`                                    |
| **Identidad única**       | `validate:"format=uuid"`           | Valida la forma canónica de UUID (hex 8-4-4-4-12 con guiones); la versión y la variante (RFC 4122) NO se verifican.                        | `string`                                    |
| **Expresión regular**     | `validate:"regexp=^[a-z]{2,4}$"`   | Valida el formato de la cadena coincidiendo con un patrón de expresión regular. Seguro frente a comas incrustadas. Las comillas exteriores (`'...'` o `"..."`) se eliminan del patrón; para coincidir con una comilla literal, use el tipo de comilla opuesto (por ejemplo, `regexp="^'q'$"`). | `string`, `[]string`                        |
| **Endpoint**              | `validate:"endpoint"`              | Impone endpoints de red estándar (`host:port`). Comprueba nativamente la sintaxis **IPv6** y los límites de puerto (1-65535).              | `string`                                    |
| **URL obligatoria**       | `validate:"url"`                   | Comprueba identificadores uniformes de recursos. Exige estrictamente un separador explícito de esquema de protocolo (por ejemplo, `http://`, `grpc://`) y un host no vacío: los valores solo con esquema (`http://`) se rechazan. | `string`                                    |
| **Reglas de ruta por instancia** | `validate:"min_len.min=0,max_len.max=64"` | **Tipos anidados compartidos:** aplica una regla a un campo anidado de esta instancia concreta mediante la sintaxis `<ruta_yaml>.<regla>=<valor>`; cada instancia padre tiene sus propios límites. Las rutas colgantes se informan como errores. Los valores de las reglas de ruta pueden contener puntos. | campos anidados de struct |

---

## Carga y volcado

- **Las opciones son de extremo a extremo:** cada opción pasada a `Load` / `LoadFile` (por ejemplo, `WithKnownFields`, `WithUniqueKeys`) se aplica tanto al análisis del AST como a la decodificación final en la estructura de destino — la comprobación estricta funciona exactamente como el `yaml4.Load` upstream. Las opciones solo de formato (`WithIndent` y afines) no afectan a la carga.
- **La escritura en include (`!include`) está ligada al objeto:** `DumpWithInclude` toma sus rutas de escritura de los datos de include registrados al cargar precisamente ese objeto — pase a `DumpWithInclude` el mismo puntero que pasó a `Load` / `LoadFile`. Una estructura que nunca pasó por dicha carga (o una copia del valor cargado) se vuelca en un modo seguro: los valores se emiten en línea, no se sintetizan marcadores `!include` y no se modifica ningún archivo.
- **`!include` se resuelve en todas las rutas de carga:** `LoadFile` resuelve las directivas `!include` relativas al directorio del archivo de configuración, mientras que `Load`, `Loader.Load` y `Decoder.Decode` las resuelven contra el directorio de trabajo actual. Las rutas de flujo no vinculan ninguna sesión de include al destino, y una carga de flujo exitosa de un puntero previamente cargado mediante `Load`/`LoadFile` elimina esa vinculación anterior: el objeto se vuelca en el modo seguro en línea (véase la viñeta de escritura anterior).
- **Un documento por destino de include:** un archivo destino de `!include` debe contener exactamente un documento YAML — un marcador de documento `---` final hace que la carga falle con `included file ... contains multiple documents`. El config principal sigue la semántica del upstream yaml4: las rutas de una sola vez (`LoadFile`, `Load`, `Unmarshal`) también rechazan un marcador `---` final (`expected single document, found multiple`), mientras que las rutas de flujo (`Loader.Load`, `Decoder.Decode`) leen solo el primer documento y descartan silenciosamente el resto.

---

## Perfiles de configuración (casos válidos e inválidos)

### Ejemplo 1: Config estándar de aplicación con salida de ayuda (perfil válido)
**Especificación YAML (`config.yaml`):**
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

**Modelo de configuración de la aplicación Go:**
```go
package main

import (
	"fmt"
	"time"

	"github.com/cybervask/yaml"
)

type TLS struct {
	MinVersion string   `yaml:"min_version" default:"tls1.3" validate:"choice=tls1.2,tls1.3" env:"TLS_MIN_VERSION" description:"Versión mínima de TLS"`
	Alpn       []string `yaml:"alpn" default:"h2,http/1.1" validate:"choice=h2,http/1.1" description:"Negociación de protocolo de capa de aplicación"`
}

type Logging struct {
	Level         string           `yaml:"level" default:"info" validate:"choice=debug,info,warn" description:"Nivel de ejecución de la jerarquía de registros"`
	Colors        bool             `yaml:"colors" description:"Activar salida de registros con colores ANSI"`
	Timeout       string           `yaml:"timeout" default:"5s" description:"Tiempo de espera interno de procesamiento descendente"`
}

type Config struct {
	Env          string        `yaml:"env" default:"dev" validate:"choice=dev,stage,prod" description:"Entorno de ejecución de la aplicación"`
	AppName      string        `yaml:"app_name" default:"api" validate:"minlen=3,maxlen=10" description:"Moniker interno del despliegue en tiempo de ejecución"`
	APIUrl       string        `yaml:"api_url" validate:"url,required_if=env==prod" env:"API_URL" description:"Enlace base del destino de la API"`
	APIEndpoint  string        `yaml:"api_endpoint" validate:"endpoint" description:"Socket de red de vinculación local"`
	Timeout      time.Duration `yaml:"timeout" default:"5s" validate:"min=1s,max=10m" description:"Parámetro de seguimiento de límites de solicitudes globales en tiempo de ejecución"`
	Crypto       TLS           `yaml:"crypto" description:"Capa de configuración de la estructura de seguridad TLS"`
	Server       struct {
		Logging    Logging  `yaml:"logging" description:"Parámetros de configuración de registros del servidor"`
		AllowedIPs []string `yaml:"allowed_ips" validate:"mincount=1,maxcount=10" description:"Remotos de red en lista blanca"`
	} `yaml:"server"`
}

func main() {
	var cfg Config
	yaml.Help(cfg)
}
```

**Salida de ayuda interactiva automatizada para CLI (`yaml.Help(cfg)`):**

Los tipos de struct autorreferenciados se documentan una vez por ruta — la rama recursiva se omite silenciosamente, por lo que `Help` siempre termina.
```text
yaml configuration schema documentation:

env:             Entorno de ejecución de la aplicación (default: dev, validate: [choice=dev,stage,prod])
app_name:        Moniker interno del despliegue en tiempo de ejecución (default: api, validate: [minlen=3,maxlen=10])
api_url:         Enlace base del destino de la API (env: API_URL, validate: [url,required_if=env==prod])
api_endpoint:    Socket de red de vinculación local (validate: [endpoint])
timeout:         Parámetro de seguimiento de límites de solicitudes globales en tiempo de ejecución (default: 5s, validate: [min=1s,max=10m])
crypto:          Capa de configuración de la estructura de seguridad TLS
  min_version:   Versión mínima de TLS (env: TLS_MIN_VERSION, default: tls1.3, validate: [choice=tls1.2,tls1.3])
  alpn:          Negociación de protocolo de capa de aplicación (default: h2,http/1.1, validate: [choice=h2,http/1.1])
server:
  logging:       Parámetros de configuración de registros del servidor
    level:       Nivel de ejecución de la jerarquía de registros (default: info, validate: [choice=debug,info,warn])
    colors:      Activar salida de registros con colores ANSI
    timeout:     Tiempo de espera interno de procesamiento descendente (default: 5s)
  allowed_ips:   Remotos de red en lista blanca (validate: [mincount=1,maxcount=10])
```

---

### Ejemplo 2: Violación de la restricción de lista blanca de choice (perfil inválido)
**Especificación YAML (`config.yaml`):**
```yaml
env: "testing" # Error: "testing" está fuera de los límites de la lista blanca [dev, stage, prod]
```
**Cadena de error en tiempo de ejecución devuelta por la aplicación (`err.Error()`):**
```text
field env: value "testing" is invalid; allowed choices are [dev, stage, prod]
```

---

### Ejemplo 3: Múltiples anomalías, longitud de cadena, tamaño de colección y restricciones de duración (perfil inválido)
**Especificación YAML (`config.yaml`):**
```yaml
env: "prod"
app_name: "go"         # Error 1: viola minlen=3
api_url: ""            # Error 2: la restricción required_if falla bajo env==prod
timeout: "500ms"       # Error 3: viola el límite estricto de duración min=1s
server:
  logging:
    level: "debug"     # (Válido)
  allowed_ips: []      # Error 4: viola mincount=1 de la colección
```
**Pila de cadenas de error en tiempo de ejecución devueltas por la aplicación:**
```text
field app_name: string length 2 is less than minlen 3
field api_url: is required when field env==prod
field timeout: value 500ms < min 1s
field server.allowed_ips: collection size 0 is less than mincount 1
```

---

## Ejemplos

El directorio `examples/` contiene programas ejecutables que cubren todas las reglas de validación y funcionalidades. Ejecute cualquiera de ellos desde la raíz del repositorio:

```bash
go run ./examples/01_choice
```

| Ejemplo | Demuestra |
|---------|--------------|
| `01_choice` | Reglas `choice` de lista blanca/negra, valores entre comillas que contienen comas, perfiles válidos e inválidos |
| `02_help` | Generación automatizada de ayuda CLI mediante `yaml.Help()` a partir de etiquetas `description` |
| `03_min-max` | Límites inclusivos `min`/`max` para números y `time.Duration`, perfiles válidos y fuera de rango |
| `04_mincount-maxcount` | Límites de tamaño de colección para slices y maps, perfiles válidos y que violan la capacidad |
| `05_required_if` | `required_if` cruzado con los operadores `==`/`!=`, los comparadores regexp `~=`/`^=`, más las reglas `endpoint` y `url` |
| `06_with-include` | Resolución de la directiva `!include` durante la carga, incluida una falta de archivo |
| `07_with_env_overwrite` | Inyección de variables de entorno que anulan los valores por defecto de las etiquetas, incl. una violación de lista blanca |
| `08_write-to-includes` | Escritura inversa atómica que preserva la estructura `!include` (`DumpWithInclude`) |
| `10_test` | Rastreador de rutas de include (`FindIncludeFile`) más la ruta de rechazo de `DumpWithInclude` para escalares |
| `11_not_empty-required` | Regla `required` y su alias heredado `not_empty` (conservado por compatibilidad con versiones anteriores) |
| `12_gt-lt` | Límites estrictos `gt`/`lt` para int, float y `time.Duration` |
| `13_minlen-maxlen` | Límites de longitud de cadena contados en runas Unicode |
| `14_format` | `format=ip`, `format=ipv4`, `format=ipv6`, `format=uuid`, `format=hex` |
| `15_regexp` | Regla `regexp`: patrones simples, comas incrustadas, elementos de slices |
| `16_validation-errors` | Informe agregado de múltiples violaciones a lo largo de diferentes reglas |
| `17_embed` | Reglas de ruta por instancia sobre un tipo anidado compartido (`min_len.min=0` vs `min_len.min=4`) |
| `18_required_if-operators` | Operadores canónicos de `required_if`: `==`, `!=`, `~=` y `^=` |
| `19_priority` | Arbitraje de fuentes entre valores YAML/default y variables de entorno mediante la etiqueta `priority`: ambas fuentes presentes, fuente única, el comportamiento sin etiqueta y el límite del default cero |
| `20_required_if-logic` | Expresiones booleanas en `required_if`: `&&`, `\|\|`, paréntesis y precedencia (`&&` vincula más que `\|\|`) |
| `21_required_if-break` | Sección condicional de reglas después de `required_if` mediante `break` |

# KNOWN_PROBLEMS.es.md — problemas heredados de la biblioteca upstream

> Alcance: comportamientos heredados byte a byte de `go.yaml.in/yaml/v4` v4.0.0.
> Esta biblioteca es un reemplazo directo (drop-in): no parchea el analizador
> upstream, por lo que las peculiaridades del upstream se documentan aquí
> en lugar de "corregirse".

## KP-1 · Las banderas de struct tag no admitidas provocan pánico al decodificar y error al codificar

- **Síntoma**: una struct tag con una bandera que el decodificador upstream
  no conoce — por ejemplo `yaml:"a,include"` — hace que todas las rutas de
  decodificación entren en pánico:
  `panic: unsupported flag "include" in tag "a,include" of type <T>`, mientras
  que `Marshal` de la misma struct devuelve un error normal con la misma
  redacción. Reproducido: `yaml4.Unmarshal([]byte("a: x\n"), &v)` entra en
  pánico; nuestro `Unmarshal` entra en pánico de forma idéntica (paridad
  drop-in byte a byte); `yaml4.Marshal` devuelve
  `yaml: unsupported flag "include" in tag "a,include" of type <T>`.
- **Causa raíz (upstream)**: `go.yaml.in/yaml/v4` v4.0.0 rechaza las
  banderas de struct tag desconocidas con un pánico en sus rutas de
  decodificación y un error en su ruta de codificación — la asimetría es
  del upstream.
- **Estado**: no se parchea en esta biblioteca; parchearlo rompería la
  compatibilidad drop-in, por lo que el comportamiento se documenta.
- **Evitación**: no usar banderas desconocidas para el decodificador
  upstream; fije las struct tags en pruebas (go vet no detecta banderas
  desconocidas).

## KP-2 · Entrada multi-documento: las rutas de una sola vez la rechazan, las rutas de flujo truncan

- **Síntoma**: una entrada con más de un documento YAML (un separador `---`,
  con el segundo documento vacío o no) es rechazada por las rutas de una
  sola vez (`Unmarshal`, `Load`, `LoadFile`) con el error
  `failed to parse YAML to AST: yaml: construct errors: <unknown position>:
  yaml: expected single document, found multiple`, mientras que las rutas de
  flujo (`Loader.Load`, `Decoder.Decode`) devuelven el primer documento
  silenciosamente (`err=nil`, el resto se descarta). Verificado contra las
  cinco rutas con ambas entradas `a: 1\n---\nb: 2\n` y `a: 1\n---\n`.
- **Causa raíz (upstream)**: el cargador upstream basado en Node de una sola
  vez da error con los documentos extra; el cargador de flujo upstream lee
  el primer documento y se detiene — la asimetría vive en el código del
  analizador/cargador upstream, y esta biblioteca simplemente enruta ambos
  comportamientos a través de su propia API (incluida la envoltura del
  error).
- **Estado**: no se parchea para el documento de nivel superior (paridad
  drop-in). Los destinos de `!include` están protegidos por esta biblioteca
  en todas las rutas: un destino de include con más de un documento falla
  con `included file ... contains multiple documents` independientemente de
  la ruta de carga.
- **Evitación**: mantener exactamente un documento por archivo; al leer en
  flujo, divida los documentos usted mismo y llame a `Decoder.Decode` una
  vez por documento.

## Parecido, pero nuestro — no del upstream

- `Decoder.Decode` ahora resuelve las directivas `!include` de la misma
  forma que las rutas de carga: cada llamada a `Decode` reinicia el rastreador
  de includes, analiza el siguiente documento en un nodo y resuelve
  `!include` recursivamente contra el directorio de trabajo actual antes de
  aplicar los valores por defecto y decodificar el nodo en el destino. La
  limitación anteriormente documentada queda levantada. No se vincula
  ninguna sesión de include al destino: un posterior `DumpWithInclude` de un
  objeto decodificado por flujo se ejecuta en el modo seguro en línea
  (inline safe mode). Una sesión vinculada previamente al mismo puntero por
  `Load` o `LoadFile` se elimina mediante una carga de flujo exitosa
  (`Decoder.Decode` o `Loader.Load`) — el flujo reemplazó el contenido del
  objeto, por lo que la vinculación anterior ya no lo refleja; una carga de
  flujo fallida conserva la sesión.
- `HelpStr` ya no emite espacios en blanco finales: cada línea renderizada
  se recorta con `strings.TrimRight` antes del salto de línea — nuestro
  renderizado, deliberadamente distinto de la salida con relleno estilo
  upstream; la alineación de columnas se conserva, y el ancho de la columna
  de rutas se mide en columnas de visualización del terminal (las runas
  anchas East Asian reservan dos); la documentación sigue al código: el
  ejemplo de README.md se actualizó al nuevo renderizado, mientras que los
  ejemplos localizados ru/zh/ja/es conservan sus descripciones traducidas y
  se recortan de espacios en blanco finales en las mismas líneas.

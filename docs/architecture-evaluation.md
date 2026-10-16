# Evaluación técnica consolidada para un motor Go orientado a Suwayomi/Mihon

## Objetivo del documento

Este documento consolida la formulación técnica propuesta para reemplazar el motor principal de Suwayomi por un backend en Go, con foco en menor consumo de RAM, mejor respuesta del lector y una arquitectura más desacoplada para evaluación por un nuevo agente técnico. La propuesta parte del contexto actual de Suwayomi: soporte reciente para PostgreSQL, separación del WebUI respecto del server, compatibilidad con extensiones de Mihon/Tachiyomi y soporte de fuentes locales basadas en carpetas y archivos comprimidos.[cite:2][cite:1][cite:34][cite:25]

## Resumen ejecutivo

La recomendación principal es reemplazar en Go el núcleo de almacenamiento, lectura, caché, serving de imágenes, indexación de capítulos y progreso de lectura, pero **no** reimplementar en una primera fase el runtime de extensiones de Mihon/Suwayomi. Suwayomi sigue dependiendo de un subsistema compatible con extensiones del ecosistema Mihon/Tachiyomi, por lo que la forma más realista de mantener compatibilidad alta es aislar la JVM en un sidecar o servicio separado dedicado solo a ejecutar extensiones, mientras el nuevo core en Go asume la mayor parte del trabajo pesado del servidor.[cite:2][cite:19][cite:34][cite:36]

La optimización más importante no está solo en cambiar Java por Go, sino en rediseñar el flujo de lectura: PostgreSQL para metadatos, CBZ/ZIP como origen inmutable, manifiestos persistentes por capítulo, derivados de imagen persistentes por hash en disco y un pipeline “preview-first, original-on-demand”. Esa formulación busca bajar RAM, reducir parseos repetidos de ZIP y mejorar la latencia percibida en el lector.[cite:1][cite:25][cite:22]

## Hallazgos verificables sobre Suwayomi

### Estado actual del proyecto

Suwayomi-Server anunció soporte para PostgreSQL en la línea v2.2.x, mientras su documentación de configuración ya expone parámetros específicos de base de datos y sigue advirtiendo que PostgreSQL está en estado beta; en contraste, H2 se mantiene como opción por defecto y se menciona riesgo de corrupción si el servidor se apaga de forma incorrecta.[cite:2][cite:1]

Suwayomi también mantiene separación entre servidor y WebUI. La configuración documenta modos para servir la interfaz oficial, una interfaz custom o deshabilitar la UI embebida, lo que favorece una estrategia donde el backend nuevo se mantenga inicialmente compatible con la UI oficial o con una interfaz futura más rápida.[cite:1][cite:26]

### Soporte de extensiones

Suwayomi se presenta como un servidor compatible con extensiones construidas para Mihon/Tachiyomi.[cite:19] El proyecto además documenta la obtención de extensiones mediante repositorios configurables y la administración de esos repositorios desde launcher, WebUI, archivo de configuración o variable de entorno.[cite:34]

Las notas de versión más recientes también mencionan soporte para Extension API v1.6, lo que confirma que la capa de compatibilidad con extensiones sigue siendo una parte activa del diseño del proyecto y no un detalle periférico.[cite:38][cite:23]

### Fuentes locales

La documentación de Local Source indica que Suwayomi admite capítulos locales en directorios y archivos comprimidos como ZIP/CBZ, y que el orden correcto de las páginas depende del nombrado alfanumérico de los archivos. También indica que la estructura de subdirectorios dentro del archivo comprimido no afecta la lectura mientras existan imágenes válidas.[cite:25]

Esto es relevante porque una reimplementación en Go puede mantener compatibilidad funcional con fuentes locales sin copiar la arquitectura completa actual: basta con preservar el modelo de descubrimiento, ordenado y serving, pero indexando el contenido una sola vez y evitando recorridos repetitivos sobre el ZIP.[cite:25]

## Problema técnico identificado

El cuello de botella más probable no es simplemente “Java”, sino una combinación de varios factores: uso de H2 en despliegues no ideales, caché de imágenes poco predecible, parseo repetido de archivos comprimidos, serving de imágenes originales sin derivados persistentes y una posible mezcla de responsabilidades entre lectura, fetch remoto, almacenamiento y UI.[cite:1][cite:12][cite:15]

También existen antecedentes públicos de comportamiento problemático en la caché de imágenes, por ejemplo el issue donde desactivar `Use Image Cache` no detenía efectivamente el cacheo, así como reportes de cubiertas cacheadas no utilizadas en determinadas configuraciones. Eso refuerza que el subsistema de caché debe ser explícito, observable y controlado por política, no implícito ni acoplado a otras capas.[cite:12][cite:15]

## Formulación propuesta

## Principios de arquitectura

La propuesta se basa en seis principios:

- PostgreSQL solo para metadatos, relaciones, progreso, jobs persistentes y manifiestos; no para blobs pesados.[cite:2][cite:1]
- CBZ/ZIP o directorios locales como origen inmutable del contenido fuente.[cite:25]
- Manifiesto persistente por capítulo para evitar reindexación continua y para dar respuestas rápidas al lector.[cite:22][cite:25]
- Derivados de imagen persistentes por hash en disco o almacenamiento de objetos, no en Redis como estrategia principal.[cite:1][cite:25]
- Redis solo como caché efímera y sistema de coordinación, no como repositorio primario de previews.[cite:1]
- Compatibilidad con extensiones mediante sidecar JVM o adaptador, no mediante reimplementación inmediata del runtime de extensiones en Go.[cite:19][cite:34][cite:36]

## Componentes propuestos

| Componente | Responsabilidad principal | Tecnología sugerida |
|---|---|---|
| API HTTP | Biblioteca, lector, progreso, autenticación, serving de assets | Go |
| Metadata DB | Series, capítulos, páginas, progreso, jobs, assets | PostgreSQL [cite:2][cite:1] |
| Storage origen | Directorios locales, ZIP/CBZ | FS local / volumen montado [cite:25] |
| Derived storage | Covers, thumbs, previews, full optimizados | Disco NVMe / object storage |
| Cache/coordination | Locks, manifests calientes, sesiones efímeras, rate limiting | Redis [cite:1] |
| Extension runtime | Instalación y ejecución de extensiones Mihon/Suwayomi | JVM sidecar [cite:19][cite:34][cite:36] |
| Workers | Importación, indexación, derivados, limpieza | Go |

## Diseño de datos sugerido

El modelo relacional propuesto parte de `libraries`, `series`, `chapters`, `pages`, `assets`, `reading_progress` y `jobs`. La base guarda referencias al origen, metadatos estructurados y estado de derivados, mientras que los binarios quedan fuera de PostgreSQL para evitar presión de memoria, tamaño innecesario de la base y complejidad de serving.[cite:2][cite:1][cite:25]

### Entidades clave

- `series`: metadata bibliográfica, cover, estado, tags, relaciones lógicas.
- `chapters`: referencia al origen físico (`dir`, `zip`, `cbz`, etc.), checksum, page count, tamaño.
- `pages`: índice de página, nombre de entrada dentro del ZIP, mime, dimensiones, checksum y, si es posible, metadatos de acceso al ZIP.[cite:25]
- `assets`: derivados persistentes (`cover`, `thumb`, `preview`, `full`) direccionados por hash.
- `reading_progress`: progreso por usuario/serie/capítulo.
- `jobs`: cola persistente para importación, derivación y mantenimiento.

## Flujo de lectura recomendado

La experiencia de lectura debe basarse en manifiesto primero e imagen después. El cliente pide un `manifest` del capítulo, obtiene conteo de páginas, dimensiones conocidas, estado de derivados y URLs para preview/original, y con eso puede pintar placeholders exactos sin esperar a que se abran imágenes pesadas.[cite:22]

Luego el cliente carga previews de las primeras páginas y el backend puede precalentar 2 a 5 páginas siguientes. Solo cuando el usuario hace zoom, navega o se aproxima en scroll se solicita la imagen original o una variante `full-optimized`. Este flujo se alinea con el hecho de que el WebUI de Suwayomi ya ha necesitado refetch de páginas cuando el conteo local queda desactualizado, lo que refuerza la importancia de que el manifiesto sea una pieza central, confiable y cachéable.[cite:22]

### Flujo resumido

1. `GET /chapters/{id}/manifest`
2. Respuesta con `page_count`, dimensiones, estado y URLs.
3. Render inicial con placeholders.
4. Carga de previews de páginas iniciales.
5. Warmup de páginas siguientes.
6. Carga bajo demanda de `image` u `original`.

## Estrategia de imágenes y caché

### Recomendación principal

No se recomienda usar Redis como almacén primario de blobs low-res de páginas. Aunque técnicamente puede almacenar esos datos, usarlo para miles de previews grandes desplaza el problema hacia la RAM, eleva presión de eviction y termina compitiendo directamente con el objetivo del proyecto, que es reducir consumo de memoria en el host o en Proxmox.[cite:1]

La alternativa preferida es generar derivados persistentes por hash y almacenarlos en disco NVMe o en object storage. Redis queda reservado para metadatos calientes, locks, colas rápidas y hints de prefetch.[cite:1]

### Variantes sugeridas

- `cover`: 320–480 px lado corto, WebP.
- `thumb`: 240 px ancho, WebP.
- `preview`: 720–960 px ancho, calidad media, WebP.
- `full`: original o versión recomprimida si conviene por peso.

### Política de generación

- No derivar todas las páginas al importar por defecto.
- Generar covers y primeras páginas de capítulos calientes.
- Derivar `preview` bajo demanda al primer acceso si no existe.
- Persistir los derivados por checksum o hash estable para invalidación automática cuando cambie el origen.

## Indexación de capítulos

### Directorios

Cuando el capítulo es una carpeta, el proceso recomendado es escanear archivos de imagen, ordenarlos alfanuméricamente, detectar mime, extraer dimensiones de forma barata cuando sea posible y persistir el manifiesto.[cite:25]

### ZIP/CBZ

Cuando el capítulo está en ZIP o CBZ, lo correcto es leer el `central directory` una sola vez, enumerar entradas válidas, ordenarlas, persistir el manifiesto y reutilizar ese índice en adelante. La propuesta evita volver a recorrer el archivo comprimido en cada apertura de capítulo y reduce CPU e I/O redundantes.[cite:25]

## Compatibilidad con extensiones Mihon/Suwayomi

### Conclusión objetiva

La propuesta **sí permite** seguir usando extensiones existentes de Mihon/Suwayomi, pero no porque Go vaya a ejecutarlas de forma nativa desde el primer día. La compatibilidad real depende de mantener o encapsular el runtime de extensiones que hoy forma parte del ecosistema Suwayomi/Mihon.[cite:19][cite:34][cite:36]

### Implicancia práctica

Reemplazar en Go el core de biblioteca, lector, caché, importación y metadatos es viable sin perder extensiones, siempre que el proyecto utilice una de estas dos estrategias:

- Sidecar JVM dedicado a extensiones.
- Adaptador o bridge hacia el runtime existente de Suwayomi para resolver búsqueda, listado y páginas remotas.[cite:19][cite:34][cite:36]

### Lo no recomendado en fase inicial

No se recomienda reimplementar de inmediato el runtime de extensiones en Go. Las razones son técnicas: el ecosistema de extensiones de Mihon/Tachiyomi no es solo un conjunto de endpoints HTTP, sino una capa con compatibilidad de APIs, empaquetado de extensiones, contratos de fuentes y soporte de cambios de versión como la Extension API v1.6.[cite:38][cite:23][cite:36]

## Contrato sugerido entre el core Go y el runtime de extensiones

Para desacoplar responsabilidades, el runtime de extensiones debería exponerse como proveedor abstracto de fuentes. El core Go no necesita conocer detalles internos de la JVM; solo necesita una interfaz consistente para listar fuentes, buscar series, obtener capítulos y resolver páginas remotas.[cite:19][cite:34]

### Capacidades mínimas del provider

- Listar fuentes disponibles.
- Buscar una serie por texto y filtros.
- Obtener metadata de una serie remota.
- Obtener capítulos de una serie.
- Obtener URLs o referencias de páginas remotas.
- Instalar, actualizar y desinstalar extensiones fuera del core principal.

## Endpoints sugeridos del nuevo backend

### Biblioteca

- `GET /api/v1/library`
- `GET /api/v1/series`
- `GET /api/v1/series/{id}`
- `GET /api/v1/series/{id}/chapters`

### Lector

- `GET /api/v1/chapters/{id}/manifest`
- `GET /api/v1/pages/{id}/preview`
- `GET /api/v1/pages/{id}/image`
- `HEAD /api/v1/pages/{id}/image`
- `POST /api/v1/reading-progress`

### Importación

- `POST /api/v1/import/scan`
- `POST /api/v1/import/library/{id}/refresh`
- `GET /api/v1/jobs/{id}`

### Assets

- `GET /api/v1/series/{id}/cover`
- `GET /api/v1/assets/{id}`

## Estructura sugerida del proyecto Go

```text
/internal
  /api
    router.go
    middleware.go
    handlers_library.go
    handlers_reader.go
    handlers_import.go
  /domain
    series.go
    chapter.go
    page.go
    progress.go
    job.go
  /repo
    /postgres
      series_repo.go
      chapter_repo.go
      page_repo.go
      job_repo.go
  /storage
    fs.go
    zip.go
    derived.go
  /image
    probe.go
    decode.go
    transform.go
    cache.go
  /queue
    dispatcher.go
    worker.go
  /cache
    redis.go
  /provider
    local.go
    mihon_sidecar.go
/cmd/server
  main.go
/cmd/worker
  main.go
```

Esta división busca separar API, dominio, persistencia, storage físico, imagen, cola y proveedores de fuentes para que el sistema no quede acoplado ni a ZIP ni al runtime de extensiones.[cite:25][cite:19]

## Métricas y observabilidad sugeridas

Los puntos a medir no son solo CPU o RAM del proceso. El proyecto debería exponer métricas de experiencia real del lector y eficiencia del pipeline de imágenes, especialmente porque Suwayomi ya ha mostrado sensibilidad a cacheo y refresco de páginas.[cite:12][cite:15][cite:22]

### Métricas mínimas

- Tiempo de respuesta de `manifest`.
- Tiempo a primera preview visible.
- Tiempo de decode de imagen.
- Tiempo de generación de derivados.
- Hit ratio de caché de derivados.
- Número de veces que se abre un ZIP por capítulo.
- RAM residente del proceso Go.
- Jobs en cola e inflight.

## Riesgos y trade-offs

| Tema | Beneficio esperado | Riesgo o costo |
|---|---|---|
| Mover core a Go | Menor RAM base, mejor control de concurrencia y perfiles más simples | Reescritura amplia del backend |
| Mantener JVM solo para extensiones | Alta compatibilidad con ecosistema Mihon/Suwayomi [cite:19][cite:34][cite:36] | Mayor complejidad operativa por sidecar |
| PostgreSQL en vez de H2 | Mejor robustez y escalabilidad [cite:2][cite:1] | Más complejidad de despliegue |
| Derivados persistentes en disco | Menor recomputación, mejor latencia percibida | Uso adicional de almacenamiento |
| Preview-first | Mejor experiencia de lectura [cite:22] | Mayor complejidad del cliente y del pipeline |
| Reusar WebUI oficial inicialmente | Menor costo de transición [cite:1][cite:26] | Posible fricción si el contrato API no coincide por completo |

## Evaluación objetiva final

La propuesta es técnicamente coherente si el objetivo es reducir consumo de RAM y mejorar la respuesta del lector sin romper de inmediato el ecosistema existente. La parte más sólida de la formulación es el desacople entre metadatos, origen de contenido, derivados de imagen y runtime de extensiones; la parte más delicada es la compatibilidad operacional con la UI oficial y con el stack actual de extensiones, que no debe asumirse como trivial.[cite:1][cite:19][cite:34][cite:36]

La evaluación más objetiva para un agente nuevo debería centrarse en estas preguntas:

1. ¿El motor Go actual ya separa storage, image pipeline y runtime de extensiones, o aún mezcla esas responsabilidades?
2. ¿Existe manifiesto persistente por capítulo o se sigue recorriendo ZIP/CBZ en caliente?
3. ¿Las previews se piensan como derivados persistentes o como blobs efímeros en RAM/Redis?
4. ¿La compatibilidad con extensiones se pretende resolver por bridge/sidecar o por reimplementación completa?
5. ¿La UI oficial seguirá siendo cliente temporal y, si es así, qué nivel de compatibilidad API es realmente necesario?
6. ¿Ya existen métricas para validar menor RAM y mejor tiempo a primera página, o la comparación sigue siendo intuitiva?

## Recomendación priorizada para continuar

- Mantener PostgreSQL para metadatos y jobs persistentes.[cite:2][cite:1]
- Mantener CBZ/ZIP y local source como origen inmutable compatible.[cite:25]
- Implementar manifiesto persistente por capítulo antes de optimizar otros detalles.[cite:22][cite:25]
- Guardar previews y covers como derivados persistentes por hash, no como blobs primarios en Redis.[cite:1]
- Encapsular las extensiones Mihon/Suwayomi en un sidecar o bridge, al menos en la primera fase.[cite:19][cite:34][cite:36]
- Reusar la WebUI oficial solo como etapa transitoria mientras se estabiliza el nuevo core.[cite:1][cite:26]
- Medir RAM residente, tiempo a primer render y hit ratio de caché antes de declarar mejoras.[cite:12][cite:15][cite:22]


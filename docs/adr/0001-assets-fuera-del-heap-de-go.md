# ADR 0001 — Los assets grandes del camino de consola viven fuera del montón de Go, con liberación explícita

- **Estado**: aceptado (decisión de Daniel, 31/10/2026: opción **1b / 2a / 3a**).
- **Fecha**: 2026-10-31.
- **Decide**: Daniel Galli ("Papu"). **Escribe y mide**: Hermes — tarjeta `t_9ee5adc4` (DC-002.95), rama `DC-002`.
- **Afecta a**: `console/dc/fuenteconsola` (la Source), los POC de producto del camino de consola y los tres
  adaptadores de consola del motor (`d2core/d2render/pvr`, `d2core/d2input/maple`, `d2core/d2audio/aica`).
- **Regla que lo hace cumplir**: `scripts/console/checar-assets-por-la-source.sh` (+ su mutador
  `mutar-assets-por-la-source.sh`). Deuda declarada: `scripts/console/assets-fuera-de-la-source.deuda.txt`.

Este ADR se lee **antes de tocar un asset** del camino de consola. Si vas a agregar una lectura de archivo a
un POC de consola, a la Source o a un adaptador, la última sección ("Si la regla te pinta rojo") es la receta.

---

## 1. Contexto: los cuatro topes, medidos

La Dreamcast tiene **16 MB de RAM** y el motor está pensado para PC. No es una impresión: en el camino de
consola se midieron cuatro paredes distintas, y las cuatro terminan en el mismo lugar —memoria que se pide y no
vuelve—.

### 1.1 El `.dt1` y el arena: 4.849.148 B en UNA reserva

Componer el pueblo necesita un arena de tiles: **4.849.148 B de una sola `make`** cuando al montón le quedaban
4.554.752 B → `fatal error: gc_external_alloc: OOM`, faltan **294.912 B** (DC-002.82). Y no son las tablas las
que compiten: con **CERO** tablas el compositor pide **6.168.348 B** contra 5.230.592 de margen, o sea faltan
**937.756 B** igual (DC-002.86). El arena después se quedó, alimentado por la Source, y en DC-002.101 se midió
que **se puede soltar** cuando la caché ya está armada (devuelve 2.621.440 B de tramos + 1.008.003 B del buffer
prestado = 3.629.443 B).

### 1.2 La lectura del MPQ: el buffer que se duplica

El camino de siempre para "el archivo entero a memoria" es `io.ReadAll` sobre el stream del MPQ. `io.ReadAll`
**no sabe cuánto mide** el archivo: va duplicando el buffer (512 B, 1 KB, 2 KB…) hasta pasarse. Medido en la
consola: un `.dt1` de **505.000 B** por `ReadAll` → `Unhandled exception … Data address error (read)` con el PC
dentro de `io.ReadAll`; los mismos archivos con `mpq.ReadFile` (buffer del tamaño exacto, **una** lectura) se
leen sin problema (DC-002.79). Por el stream, leer un `.dt1` de 505 KB pedía **~2 MB** del semiespacio y el
runtime moría con `gc_alloc: out of memory`; con memoria prestada (un `d2mpq.Scratch` de **14.272 B** reusado
archivo tras archivo) la lectura de un archivo de 1 MB se hace con 14 KB de trabajo (DC-002.78/.80).

Y lo que cuesta el `make` sin dueño, medido con el mismo instrumento (DC-002.96, bloque D): los seis archivos
del camino del motor (`d2asset.LoadFile` → `make` por archivo) dejan **3.633.432 B** tomados que **no
vuelven**. Con el tramo con dueño, el archivo de 1 MB sube el montón exactamente **1.008.008 B** y al liberar
**vuelve al byte** (delta 0). Con el mutante que no libera (`NO_LIBERA=1`) la zona deja **4.350.728 B** tomados
y la consola muere con el montón agotado.

### 1.3 El buffer de pintado: 1,2 MB de lienzo y 10,4 MB de caché de tiles

El lienzo del cuadro son **1.228.800 B** en VRAM y en régimen no se pide memoria por cuadro (**0** buffers por
cuadro; los 56 lienzos / 88.576 B son del cuadro 1, DC-002.97). Lo que sí pesaba era la **caché de tiles en
RGBA: 10.895.360 B (10,39 MB)** para el pueblo, más que el montón entero (DC-002.94). DC-002.101 la dejó en
**índices de 8 bits: 2.723.840 B (2,60 MB), 4× menos, con el cuadro idéntico al byte** (`crc32 1437022778`).

Y no bastó, porque **dónde vive** la caché también pesa (DC-002.103/.104): de sus 86 imágenes, **82 pesan menos
de 64 KB** (**2.447.360 B**) y por eso caían en el **semiespacio** del recolector (1.792 KB, con ~1,35 MB de
conjunto vivo) y ahí morían con el montón de C casi vacío. La decisión 1b aplicada a la caché es un **tramo
propio**: un solo `make` de **2.723.840 B** (o sea una sola asignación del montón de C) del que se recortan las
86 imágenes (`d2maprenderer`, palanca `CacheDeTilesEnUnTramo`/`ReservarTramoDeLaCache`, DC-002.104). Medido: el
cuadro del host sale **byte a byte igual** (51 de 51 cuadros con `crc32 1437022778`, mismo PNG) y las 86
imágenes salen del tramo con **cero derrames**. En la consola eso mueve la pared: ya no es el semiespacio sino
**cuánto queda en el montón de C** (faltan **446.464 B** para reservar el tramo al arranque y componer el
pueblo: el número de DC-002.105). Evidencia: `console/dc/evidencia/dc-002.104-la-cache-en-un-tramo-propio.md`.

### 1.4 El barrido del GC: lo que va a `malloc` no vuelve

En el runtime de la consola (`libgodc`), **todo pedido de más de 64 KB** se va a `gc_external_alloc` —es decir
a `malloc` pelado, **fuera de los dos semiespacios**— y el recolector **no lo devuelve jamás** (DC-002.28/.77).
De ahí el número de 1.2: cada asset grande leído "normalmente" es memoria perdida. A eso se suman los defectos
propios del barrido: un objeto con punteros en el montón externo era invisible y se reciclaba (DC-002.83: el
mapa de `animdata`, 2.556 claves y 88.064 B de buckets, perdía 2347→1856→… claves), y el barrido se caía
**fuera de RAM** cuando el acomodo del binario cambiaba (DC-002.92: PC en `0x3fd17b4f`, `gc_scan_object` con
`jsr @r9`).

### 1.5 El común de los cuatro

Ninguno de los cuatro se arregla pidiendo memoria en otro lado: el montón de Go es chico, el de C es chico, y
el recolector no devuelve los grandes. **La memoria de un asset no puede ser del recolector: tiene que tener
dueño y devolverse.** Eso, que suena a idea, se puede medir: `Pedidos == Liberados` y `BytesVivos == 0`.

---

## 2. Decisión (1b / 2a / 3a)

> **"Los assets grandes van fuera del heap de Go, con liberación explícita, y el motor no se toca: todo lo de
> la consola vive detrás de la interfaz Source."** — Daniel, 31/10/2026.

En concreto:

- **1b — Fuera del montón de Go, con dueño explícito.** Quien pide un tramo lo devuelve: `Pedir`/`Liberar`
  (`asset.TramoPrestado`, DC-002.99) o `ReadFileInto` sobre un destino que el que llama ya tiene. La memoria de
  trabajo (tabla de sectores, bloque comprimido, bloque descomprimido) se **presta** en un `d2mpq.Scratch` de
  14.272 B que se reusa archivo tras archivo.
- **2a — El motor no se toca.** La consola se cuelga de las interfaces que el motor **ya** declara en
  `d2common/d2loader/asset/source.go` (`Source`, `SourceDeLecturaEntera`, `SourceDeLecturaConPrestamo`,
  `SourceDeTramos`) y se registra con `RegistrarEn`. No se agregan firmas nuevas ni ramas de consola al motor
  para esto; lo que el motor ya hacía por `LoadFile` pasa a pedir un tramo cuando la fuente sabe darlo
  (DC-002.99: **el último `make` por archivo quedó cerrado**).
- **3a — Todo lo de consola detrás de UNA Source.** Una sola implementación,
  `console/dc/fuenteconsola/fuente.go`: los tres interfaces y los seis formatos (`.dt1`, `.ds1`, `.dcc`, `.cof`,
  `.dc6`, `animdata`) con el **mismo contenido que el host** (medido con tamaño y crc32, seis de seis, DC-002.96)
  y con el montón que vuelve a su nivel de partida.

Lo que la decisión **no** dice: no dice "nada de assets en el montón de Go". Lo que no entra en el montón son
los **grandes** y los que se piden por archivo; las estructuras chicas y los caches calculados siguen siendo de
Go (y por eso DC-002.101 pudo dejar la caché de tiles en índices: 2,60 MB que sí entran).

---

## 3. Alternativas descartadas, con su número

| # | alternativa | por qué no | el número que lo cierra |
|---|---|---|---|
| A | **Agrandar el semiespacio del GC** (o dejarlo grande) | no alcanza y enferma: con más semiespacio muere más adelante, y con menos hay thrashing | DC-002.93: 1536 KB lleva a las tablas y muere pidiendo **212.992 B**; 768 KB deja la consola viva pero con **511 recolecciones**; el compositor **no baja de 2048 KB** |
| B | **Recortar el binario y listo** (`--gc-sections`, `.eh_frame` fuera, `-Os`) | la imagen y el montón se mueven **1:1**: recortar compra montón, pero no paga el arena ni los `make` por archivo | DC-002.68: −**1.326.880 B** de imagen; DC-002.82: **2.277.728 B** más de imagen = **2.273.280 B** menos de montón; DC-002.86: aun con las tablas en cero faltan **937.756 B** |
| C | **Dejar todo en el montón y confiar en el GC** | el GC **no devuelve** lo que va a `gc_external_alloc` (>64 KB): cada archivo grande es memoria perdida | DC-002.96: **3.633.432 B** tomados por seis archivos y no vuelven; el `.dcc` en el montón: **45 recolecciones** y `FATAL: recover without checkpoint` |
| D | **Tocar el motor** (cambiar `d2asset`/`d2loader` para la consola) | el fork tiene que seguir compilando y midiendo igual en el host, el motor es de otro y la consola es un destino entre varios; una rama por plataforma adentro del motor se paga en cada tarjeta | el contrato ya existía: `asset.Source` + las dos interfaces opcionales; DC-002.96 no cambió **una línea** del motor para leer los seis formatos, y DC-002.99 cerró el `make` por archivo **por la misma interfaz** (`SourceDeTramos`) |
| E | **Arena propia en C para todo** (como el arena de bloques del `.dt1`) | se probó y funciona para el `.dt1` (DC-002.78/.88), pero deja el problema en cada formato y ata el código de consola al MPQ por dentro | el arena sigue existiendo y la Source lo **alimenta**; no reemplaza a la fuente (DC-002.96 §5) |

La decisión 1b/2a/3a es la que junta lo que ya estaba medido: el dueño explícito (1b) es lo único que
devuelve memoria; 2a evita pagar el motor en cada plataforma; 3a deja **un** lugar que lee, y por eso se puede
escribir una regla.

---

## 4. Consecuencias

### 4.1 La disciplina (esto es lo que cambia en el trabajo diario)

1. **En el camino de consola no se lee un asset por `os.ReadFile`, `ioutil.ReadFile`, `io.ReadAll`,
   `mpq.ReadFile` ni `mpq.ReadFileStream`.** Se pide a la Source: `Pedir` (`ReadFileInto` adentro),
   `LeerEnteroEn` con un destino propio, o `LeerEnteroEnConPrestamo` con el `Scratch` reusado.
2. **El que pide, libera.** `tramo := fuente.Pedir(nombre)` … `defer fuente.Liberar(tramo)` (o al final del
   bloque que lo usa). Un tramo vive lo que vive el cuadro o la fase que lo usa, nunca más.
3. **La memoria de trabajo se presta una vez.** El `d2mpq.Scratch` (14.272 B) es del que lee y **no se comparte
   entre goroutines**.
4. **El motor no se entera.** Se registra la fuente (`RegistrarEn`) y el motor sigue llamando a lo suyo.
5. **Los tests afirman el dueño, no el optimismo**: `Pedidos == Liberados` y `BytesVivos == 0` al final
   (`console/dc/fuenteconsola/fuente_test.go`). Un verde sin esos dos números no dice nada.

### 4.2 El costo, dicho

- El código se vuelve **imperativo sobre la vida de los buffers**: hay un `defer` que no se puede olvidar y una
  salida temprana mal escrita se lleva el tramo puesto. No es gratis; es el precio de tener memoria.
- El camino queda con **un** lugar que lee, y ese lugar es más código que "un `ReadFile` en el POC".
- El único camino **sin dueño** que queda es `Fuente.Open`, porque la interfaz `asset.Source` lo obliga. Está
  documentado, **contado** (`f.abiertos`) y declarado en la deuda como `PERMANENTE`; las cargas grandes no
  pasan por ahí.
- **No todo entra por la Source todavía, y se sabe cuánto falta**: 8 lecturas declaradas como deuda (§4.4).

### 4.3 La regla ejecutable (lo que hace que la decisión no se pierda)

    scripts/console/checar-assets-por-la-source.sh      # verde = nadie lee por fuera; 1 = hay una lectura nueva

- **Implementación**: `scripts/console/regla-assets-por-la-source/main.go`, un programa Go con `go/ast`. Se
  eligió AST y no `grep` porque el árbol está **lleno de prosa** que nombra `io.ReadAll` para explicar por qué
  NO se usa (`d2mpq/lectura_entera.go`, `d2asset/lectura_consola.go`, `d2loader/marca_carga.go`…): un grep se
  pone rojo con el comentario que enseña a no hacerlo, y el primero que lo ve lo apaga.
- **Alcance** (10 directorios, 40 archivos de producción; los 4 `_test.go` no entran): la Source
  (`console/dc/fuenteconsola`), los POC de producto (`fuente-consola`, `mapa-101`, `carga-motor-299`,
  `dcc-arena`, `coop`, `personaje-mapa`) y los tres adaptadores de consola (`d2render/pvr`, `d2input/maple`,
  `d2audio/aica`). La lista está en el programa con su porqué, y `-alcance` la imprime para que el mutador no
  tenga una segunda copia. **Un POC nuevo que pase a ser el producto se suma ahí.**
- **La deuda se declara, no se tapa**: `scripts/console/assets-fuera-de-la-source.deuda.txt` tiene hoy **9
  entradas**: **8 a pagar** (7 del POC del personaje, 1 del adaptador de audio) y **1 permanente** (`Fuente.Open`).
  Lo que no está en esa lista es violación nueva → rojo. Una entrada que ya no aparece en el árbol se avisa
  ("pagada, borrala") y **no** rompe el chequeo: el que arregla, borra su línea, y el número sólo baja.
- **El mutador exige que la regla cace lo que dice cazar** (`mutar-assets-por-la-source.sh`, sobre una copia en
  `/tmp`, el repo no se toca): **7 de 7** — copia limpia verde; `os.ReadFile`, `io.ReadAll` y `mpq.ReadFile` en
  una función nueva, rojos; `mpq.ReadFileInto` (el camino con dueño), la prosa y la deuda pagada, verdes.

Resultado de hoy, medido:

    archivos .go revisados: 40   (4 _test.go fuera)
    lecturas por el camino con dueño (ReadFileInto/Pedir): 42
    lecturas fuera de la Source detectadas: 9
    deuda declarada: 9 entradas (8 a pagar + 1 permanentes)
    VERDE: ninguna lectura de asset fuera de la Source en el camino de consola

### 4.4 La deuda que queda (visibilidad, no olvido)

| qué | cuánto | quién la paga |
|---|---|---|
| El POC del personaje lee por el camino viejo | **7 sitios** (3 en `main.go`, 1 en `nivel.go`, 3 en `objetos.go`) | su propia tarjeta (DC-002.96 §6.3); es el producto que hoy corre con control, así que se migra sin cambiar su crc32 |
| El adaptador de audio lee el WAV del efecto con `mpq.ReadFile` | **1 sitio** (`aica/proveedor.go`, `Proveedor.cargar`) | la tarjeta de audio por la Source (el efecto medido son 20.170 B: es migración, no pared) |
| `Fuente.Open` (la interfaz obliga a tenerlo) | **1 sitio**, permanente | nadie: está documentado y contado en `f.abiertos` |

### 4.5 Los límites declarados de la regla

- **El motor queda fuera del alcance a propósito.** Medido con el mismo criterio (llamadas, no comentarios): en
  `d2common`, `d2core`, `d2game`, `d2app`, `d2networking`, `d2script` y `d2thread` hay **13 archivos con 16
  llamadas** de esas formas, y **ninguna lee un asset del MPQ**: son **partidas** (`.d2s`, `stash`, autonewchar),
  **opciones** (`options.json`, escape), el **store del realm**, el `.txt` de **drops**, las tablas del
  **generador de personajes** (`d2hero/herogen`, herramienta de host), el **motor de script** y —el único que es
  del camino de assets— el `io.ReadAll` del **camino del host** que `d2asset/lectura_consola.go` conserva a
  propósito. (Un grep a pelo da **25** archivos; los otros **12** son **prosa**: comentarios que nombran
  `io.ReadAll` justamente para explicar por qué no se usa. Por eso la regla es AST.) Meter el motor en la regla
  obligaría a declarar esas 16 como excepción —y a tocar el motor, que es lo que la decisión 2a prohíbe—; y las
  lecturas de asset del motor **ya pasan por la Source** desde DC-002.99. Si alguna vez un archivo del motor se
  leyera con `os.ReadFile` como asset, **esta regla no lo ve**: es el límite, dicho.
- La regla mira **llamadas**, no tipos: no detecta una lectura escondida detrás de una función de otro paquete
  que no se llame `ReadFile`/`ReadAll`/`Open`. Por eso la convención de nombres importa: el que lee de un
  archivo lo dice en el nombre (`ReadFile*`, `Leer*`), y la revisión de la tarjeta lo mira.
- Los `_test.go` no entran: un test pide la referencia al MPQ para comparar (`fuente_test.go` lo hace) y no
  viaja al binario de la consola.

---

## 5. Si la regla te pinta rojo (la receta)

1. **Si es un asset**: pedilo a la Source. `tramo, err := fuente.Pedir(nombre)` … `defer fuente.Liberar(tramo)`
   (o `LeerEnteroEn` con un destino que ya tenés). Volvé a correr
   `scripts/console/checar-assets-por-la-source.sh`.
2. **Si es la lectura de un archivo de configuración o de una partida** (no es un asset, no es grande, no
   viene del MPQ): no va a la Source. Se declara en la deuda como `PERMANENTE` con su motivo, y el chequeo
   queda verde.
3. **Si es una deuda que ya estaba**: no la declares de nuevo; está en la lista con su tarjeta.
4. **Si arreglaste una deuda**: borrá su línea de la deuda (el chequeo te la va a nombrar como "pagada").
5. **Nunca** apagues la regla ni la saques de un script de verificación para que pase: eso es exactamente el
   defecto que este ADR existe para evitar. Si la regla molesta, el problema es lo que estás escribiendo.

## 6. Cómo se comprueba todo esto (copiable)

    # la regla (verde con el árbol de hoy; rojo si alguien agrega una lectura)
    bash scripts/console/checar-assets-por-la-source.sh
    bash scripts/console/checar-assets-por-la-source.sh -v     # el alcance con su porqué y cada lectura con dueño

    # la prueba en contra de la regla: 7 casos y ninguno probabilístico
    bash scripts/console/mutar-assets-por-la-source.sh

    # el dueño de la fuente, en el host
    go test ./console/dc/fuenteconsola/ -count=1

## 7. Referencias

- Evidencias: `console/dc/evidencia/dc-002.{78,79,80,81,82,83,86,92,93,94,96,97,98,99,100,101,102,103,104}-*.md`
  (los cuatro topes: 1.1 → .82/.86, 1.2 → .78/.79/.80/.96, 1.3 → .94/.97/.101/.104, 1.4 → .28/.77/.83/.92).
- La decisión medida en el camino de consola: `dc-002.96-la-source-de-consola.md`.
- El plan y la fase: `docs/PLAN.md` (Fase D y la regla en las "Reglas que no se negocian").
- El código: `console/dc/fuenteconsola/fuente.go`, `d2common/d2loader/asset/source.go`.

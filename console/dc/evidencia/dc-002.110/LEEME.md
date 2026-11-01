# DC-002.110 — El `.ds1` de un nivel fuera del semiespacio del recolector

**La medida, en una línea:** las rejillas de celdas del `.ds1` se van a UN bloque del montón de C y el
semiespacio lo nota (la carga del `.ds1` pasa de **+422.776 B a +42.232 B**: 380.544 B menos, medido en Flycast
con el mismo binario y la palanca A/B), **pero el montón de C no tiene esos 186.960 B que hay que pagar**: en el
pico de componer la corrida ENCENDIDO muere con `gc_external_alloc: OOM` y la APAGADO llega al mismo pico con
378.288 B libres de 11.000.864. No sobra un byte.

Lo que la tarjeta quería (que la vuelta entre) **queda a 186.960 B del montón de C**: por aritmética del
semiespacio ya entra (ver la cuenta abajo), y por montón de C todavía no.

---

## 1. El host: cuánto son las rejillas y quién las retiene

`console/dc/medidas/ds1-110` (salida completa en `host-medida-ds1-110.txt`), contra
`~/.wine-d2/drive_c/Games/Diablo II`:

```
sizeof(d2ds1.Tile) = 24 B

/data/global/tiles/act1/town/TownN1.ds1   (TownE1 es igual)
  archivo crudo = 57.652 B
  57x41 = 2337 celdas; capas: walls=2 floors=1 shadows=1 -> 4
  celdas x 24 B x capas = 224.352 B
  objetos = 41  archivos = 8
  monton de Go despues de parsear y recolectar = +166.880 B
  monton de Go despues de soltarlo y recolectar = -64.912 B   <- se devuelve TODO
```

Dos cosas que deja dichas la medida del host:

* las **cuatro rejillas de celdas** del nivel son 224.352 B en el host y **186.960 B en el SH-4** (la celda es
  20 B en la consola, medido: `MAPA110: ARENA DEL DS1 ... bytesPorCelda= 20`). El resto del `.ds1` (lista de
  archivos, 41 objetos, caminos) son unos pocos KB más los encabezados de rebanada;
* en el host el `.ds1` soltado **se libera entero** en la recolección siguiente: el recolector del host es
  exacto. Lo que lo retiene en la consola es el **barrido conservador de pila** de libgodc (el puntero al `.ds1`
  que quedó en el marco muerto de `componerNivel` cuenta como raíz), que es lo que DC-002.109 midió en 413.184 B
  vivos después de una recolección completa.

## 2. La A/B en la consola (mismo binario, `ARENA_DS1=encendido|apagado`)

Semiespacio del recolector: **1.835.008 B**.

| punto de la corrida | APAGADO (el camino de DC-002.109) | ENCENDIDO (el bloque del montón de C) |
|---|---|---|
| TRANSICION, antes de cargar el `.ds1` | 900.512 B (49 %) | 900.272 B (49 %) |
| TRANSICION, después de cargar el `.ds1` | 1.323.288 B (72 %) — **+422.776** | 942.504 B (51 %) — **+42.232** |
| TRANSICION, pico de componer | 10.622.576 B usados / 378.288 libres **ok** | **OOM** (`gc_external_alloc`, usado 10.285.248 / libre 379.104) |
| VOLVER, antes de cargar el `.ds1` | 1.309.984 B (71 %) | no se llegó |
| VOLVER, después de cargar el `.ds1` | 1.731.336 B (94 %) — +421.352 | no se llegó |
| VOLVER, componer | **no entra** (1.731.336 + ~382.632 > 1.835.008; 991 recolecciones seguidas sin avanzar) | no se llegó |

Los dos logs están enteros en este directorio. El APAGADO reproduce DC-002.109 al byte (cuadro de la
transición: `crc32= 584380401`, 307.200 píxeles, 85 superficies / 1.950.720 B, `picoDeLaFase= 10622576`,
`libre= 949176`), así que la palanca no toca el camino viejo.

## 3. Las cuentas

```
TRANSICION carga del .ds1   APAGADO  +422.776 B   (72 %)
                            ENCENDIDO +42.232 B   (51 %)      -> 380.544 B menos
bloque del monton de C      186.960 B = 9348 celdas x 20 B (SH-4)

VOLVER  APAGADO:  1.309.984 + 421.352 (parseo) + 382.632 (componer) = 2.113.968 > 1.835.008   NO ENTRA (faltan 278.960)
VOLVER  con la arena: 1.309.984 + 42.232 (el salto que midió la transición) + 382.632 = 1.734.848 (94,5 %)  ENTRA, sobran 100.160
```

O sea: **por semiespacio la vuelta ya entraría con la arena puesta.** El problema es el otro montón.

## 4. El muro: el montón de C

La corrida ENCENDIDO muere dentro de `ResetMapParaStamp`/`PlaceStampClippedWhere` de la TRANSICION, sobre el
mismo tramo en el que la APAGADO pasa:

```
MAPA109: FASE TRANSICION DS1 Act1/Town/TownE1.ds1 ancho= 57 alto= 41 enArena= true bloque= 186960
MAPA103: SEMIESPACIO TRANSICION-despues-de-loadstamp usado= 942504 total= 1835008 pct= 51 colecciones= 41
ARQ-MARCA gc_collect v=42
Out of memory. Requested sbrk_base 0x8d000000, was 0x8cf80000, diff 524288
fatal error: gc_external_alloc: OOM
Memory: arena=10664352 used=10285248 free=379104
```

Es el pico del **arena de bloques de DC-002.101** (3.029.194 B de tramos + 1.008.003 B de scratch = 4,04 MB) más
los 570.720 B de fuentes y tablas reabiertas. La APAGADO llega a ese pico con la arena del montón de C en
**11.000.864 B y 378.288 B libres** — el techo exacto —; con 186.960 B menos el asignador pide un paso de
`0x80000` de `sbrk` y ya no lo hay, y `gc_external_alloc` **tira el runtime** en vez de devolver NULL.

Y en la VUELTA el montón de C está peor, no mejor: al cargar su `.ds1` quedan **269.736 B libres**
(`cfree= 269736 cused= 10730616`), o sea que ahí la arena dejaría 82.776 B para un componer que pide ~4,04 MB.
No es un problema de la transición: **en este POC el montón de C no tiene 186.960 B en ninguna de las dos
fases.**

## 5. Lo que esta tarjeta dejó hecho

El mecanismo funciona y está probado; lo que no alcanza es la memoria.

* `d2common/d2fileformats/d2ds1/arena_110.go` — la arena: un bloque de `[]Tile` (`capas x ancho x alto`, que por
  tamaño se va a `gc_external_alloc`, fuera de los dos semiespacios), la palanca (`ActivarArenaDelDS1`), el
  liberador inyectado (`UsarLiberadorDeLaArena`) y el censo (`EstadoDeLaArena`) con `Pedidas / Soltadas / Bytes /
  Devueltos / Chicas / SinLugar / Celdas / Capas / Vivos`.
* `d2ds1/layer.go` — la capa puede colgar sus filas de un bloque ajeno (`usarCelda`, `soltarCelda`,
  `armarFilasDeLaCelda`) y `Tile` devuelve nil si la capa quedó sin celdas: un uso después de soltar se ve vacío,
  no como memoria de otro.
* `d2ds1/ds1_layers.go` y `d2ds1/ds1.go` — el bloque se pide **antes** de los `Push*` (cuando ya se saben tamaño y
  cantidad de capas), así las filas de cada capa son vistas adentro del bloque y no hay una rejilla por capa.
* `d2core/d2map/d2mapstamp/arena_110.go` — la puerta del motor: `SoltarLosTilesDelDS1()` (se llama **después** de
  `PlaceStampClippedWhere`), `TilesDelDS1EnArena()` y `BytesDeLaArenaDelDS1()`.
* `console/dc/poc/mapa-103` — la palanca `arenaDelDS1` (`ARENA_DS1=encendido|apagado`, por defecto encendido), el
  liberador `d2cSoltarBloque` y las líneas `MAPA110` (arranque, por fase, y el censo al final del ciclo).
* `d2ds1/arena_110_test.go` — equivalencia **celda por celda** contra el camino de siempre (un error de armado
  seguiría "andando" y solo se vería como un cuadro distinto), la liberación con liberador inyectado y que con la
  palanca apagada no se pide nada.

### Trampa del toolchain (anotada para el próximo)

`gccgo 15.1` no resolvía `mr.ds1.SoltarArenaDeTiles()` desde `d2mapstamp` — *"reference to undefined field or
method"* sobre métodos que **sí** están en el `.gox` de `d2ds1` (comprobado con `strings`) — y el mismo código
compila en un paquete aparte. La causa exacta no quedó aislada; lo que quedó es el camino que compila: la puerta
del motor llama a **funciones de paquete** de `d2ds1` (`SoltarArenaDeTilesDe`, `EnArenaDe`, `BytesDeLaArenaDe`),
que el compilador de la consola sí resuelve.

## 6. Qué hace falta para que la vuelta entre (la tarjeta siguiente)

Por la cuenta del semiespacio, con la arena puesta la vuelta entra (1.734.848 de 1.835.008). Lo único que falta
es **que el montón de C suelte ~200.000 B antes del pico de componer**. Candidatos, en orden de tamaño:

1. los 3.029.194 B del arena de bloques de DC-002.101 (6 tramos de 524.288) — si el `.dt1` del nivel se decodifica
   en pedazos más chicos, el pico baja;
2. el scratch de lectura de 1.008.003 B, que ya se puede soltar en cuanto el último `.dt1` está cargado
   (`SoltarPrestamoDeLecturaDeTiles`, DC-002.103) — si se suelta ANTES de armar el arena de bloques en vez de
   después, ese 1 MB deja de estar en el pico;
3. los 570.720 B de fuentes y tablas, que hoy se sueltan después de componer (DC-002.105).

Con 200.000 B libres en el pico, la arena del `.ds1` entra, el `.ds1` deja de ocupar el semiespacio y la vuelta
tiene dónde parsear el suyo.

## 7. Cómo se reproduce

```
# host: cuánto son las rejillas y qué devuelve el recolector exacto
go run ./console/dc/medidas/ds1-110 ~/.wine-d2/drive_c/Games/'Diablo II'

# host: las pruebas de la arena (equivalencia celda por celda y liberación)
go test ./d2common/d2fileformats/d2ds1/ -run 'Arena' -v

# consola: las tres fases con la arena encendida y apagada (el A/B de esta tarjeta)
BASE_GOX=~/gox108 FASE=encendido ORDEN_FASES=a ARENA_DS1=encendido bash scripts/console/build-mapa-103.sh /tmp/dcbuild/arena
CAPTURE=spectacle bash scripts/console/run-flycast.sh /tmp/dcbuild/arena/disc.gdi 280 /tmp/cap110-encendido

BASE_GOX=~/gox108 FASE=encendido ORDEN_FASES=a ARENA_DS1=apagado bash scripts/console/build-mapa-103.sh /tmp/dcbuild/arena-apagado
CAPTURE=spectacle bash scripts/console/run-flycast.sh /tmp/dcbuild/arena-apagado/disc.gdi 280 /tmp/cap110-apagado
```

Log esperado en el arranque de la consola, en las dos corridas:

```
MAPA110: ARENA DEL DS1 palanca= encendido|apagado encendida= true|false conLiberador= true|false bytesPorCelda= 20
```

## 8. Archivos de esta evidencia

| archivo | qué es |
|---|---|
| `consola-ciclo-de-fases-arena-encendido.log` | la corrida con el bloque en el montón de C: muere en el pico de componer de la TRANSICION con `gc_external_alloc: OOM` |
| `consola-ciclo-de-fases-arena-apagado.log` | la corrida del camino de DC-002.109: transición completa (cuadro con crc32 584380401) y la VUELTA muriendo en el semiespacio (991 recolecciones) |
| `host-medida-ds1-110.txt` | la medida del host: `sizeof(Tile)`, celdas por capa, bytes de las rejillas y qué devuelve el recolector exacto al soltar el `.ds1` |

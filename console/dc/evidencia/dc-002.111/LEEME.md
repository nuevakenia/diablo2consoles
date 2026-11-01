# DC-002.111 — El montón de C deja de ser el muro (y el semiespacio aparece)

**La medida, en una línea:** con el scratch de lectura pedido por archivo, el tramo del arena dimensionado por
la sonda del montón y los tramos devueltos al cambiar de nivel, la corrida del ciclo con `ARENA_DS1=encendido`
dibuja **las tres fases** (ENTRAR, TRANSICION y VOLVER), el cuadro de la VUELTA da **`crc32= 1437022778`**, el
mismo que el de ENTRAR, y **no hay un solo `gc_external_alloc: OOM`**; el pico del montón de C en la fase más
cargada es **10.199.016 B** con 1.122.712 B libres al componer.

Lo que apareció detrás del muro del montón de C fue **el semiespacio**: con la reapertura de fuentes y tablas en
cada fase, la VUELTA pedía 1.824.256 B de 1.835.008 (99,4 %). Eso también quedó resuelto (ver §3) y **sin tocar
el umbral del recolector de DC-002.109** (sigue en 95 %): la fase ahora pide 1.596.640 B (87 %).

---

## 1. Las tres corridas de la consola

Todas con `ORDEN_FASES=a` y el mismo binario por configuración; semiespacio del recolector = **1.835.008 B**;
techo del montón de C = **10.227.936 B** de arena.

### A. La corrida del criterio — `FASE=encendido ORDEN_FASES=a ARENA_DS1=encendido`

```
MAPA109: CICLO ENTRAR      ok= true libreAlComponer= 1631096 picoDeLaFase=  8879408 crc32= 1437022778 pixelesNoVacios= 307196
MAPA109: CICLO TRANSICION  ok= true libreAlComponer= 1122712 picoDeLaFase= 10199016 crc32=  584380401 pixelesNoVacios= 307200
MAPA109: CICLO VOLVER      ok= true libreAlComponer= 1245608 picoDeLaFase=  9506536 crc32= 1437022778 pixelesNoVacios= 307196
MAPA109: CRITERIO el cuadro de VOLVER es el mismo que el de ENTRAR crc32= 1437022778 (1437022778)
MAPA110: ARENA DEL DS1 pedidas= 3 soltadas= 3 bytesPedidos= 560880 bytesDevueltos= 560880 vivos= 0
```

La VUELTA vuelve al pueblo con **2.723.840 B de cache en 86 superficies**, exactamente la cache de ENTRAR
(`cacheSuperficies= 86 cacheBytes= 2723840 cacheEnIndices= true`), o sea que no solo da el mismo `crc32`: da la
misma cache.

### B. El A/B que dice que las palancas son las que hacen la diferencia

Mismo ciclo, misma arena, **las tres palancas de esta tarjeta apagadas** y `SOLTAR_EN_EL_CICLO=1`
(o sea el camino de DC-002.110 al pie de la letra):

```
MAPA111: SCRATCH DE LECTURA porArchivo= false liberador= true
MAPA111: TRAMO DEL ARENA DE BLOQUES porElMonton= false sondaInstalada= false
MAPA111: TRAMOS AL CAMBIAR DE NIVEL seDevuelven= false
MAPA109: FASE TRANSICION DS1 Act1/Town/TownE1.ds1 enArena= true bloque= 186960
Out of memory. Requested sbrk_base 0x8d028000, was 0x8cfa8000, diff 524288
fatal error: gc_external_alloc: OOM
Memory: arena=10825376 used=10285248 free=540128
```

Es el **mismo OOM de DC-002.110 al byte**: `used= 10.285.248`, el séptimo tramo de `524.288 B` sin hueco
contiguo. Con las palancas encendidas esa misma fase pasa con `picoDeLaFase= 10199016`.

### C. El A/B de la arena — `ARENA_DS1=apagado`

La transición pasa y la **VUELTA muere en el semiespacio**, no en el montón de C:

```
MAPA103: SEMIESPACIO VOLVER-antes-de-componer    usado= 1023024 total= 1835008 pct= 55
MAPA103: SEMIESPACIO VOLVER-despues-de-loadstamp usado= 1431696 total= 1835008 pct= 78
MAPA103: SEMIESPACIO VOLVER-despues-de-recolectar usado= 1217088 total= 1835008 pct= 66
fatal error: GC to-space overflow - your live data doesn't fit in the heap.
Memory: arena=10039584 used=8982232 free=1057352
```

Sin la arena del `.ds1` las rejillas del nivel nuevo (186.960 B en el SH-4) vuelven al semiespacio y la fase no
entra. Con la arena, esas rejillas están en el montón de C y los dos montones alcanzan.

### D. La corrida de referencia (ciclo apagado) — para probar que el camino viejo no cambió

```
MAPA103: CUADRO 1 pixelesNoVacios= 307196 de 307200 crc32= 1437022778 crc32RGB= -1787731553 ms= 745 cacheSuperficies= 86 cacheBytes= 2723840
MAPA103: MEMORIA despues-de-componer usado= 7993040 libre= 1403664 arena= 9396704 pico= 8409464
MAPA103: ARENA DE BLOQUES SOLTADA (despues de la cache) tramos= 5 bytes= 2621440 libre= 2018528
```

El cuadro, los píxeles, la cache y el `crc32` son los de DC-002.106/.108. Lo que **baja** es el pico del montón
de C: **de 9.096.616 B a 8.409.464 B** (−687.152 B), todo del scratch por archivo: el buffer de lectura ya no
tiene que ser el del `.dt1` más grande del nivel (`act1/town/floor.dt1` = 1.008.003 B) durante toda la carga.

## 2. El montón de C, fase por fase

| fase | palancas de .111 apagadas (camino .110) | palancas de .111 encendidas |
|---|---|---|
| ENTRAR | ok, `pico= 9.096.616` (`scratch= 1.008.003`) | ok, `pico= 8.879.408` (`scratchSoltados= 8`, `scratchBytes= 4.350.665`) |
| TRANSICION | **OOM** (`gc_external_alloc`, `used 10.285.248 / free 540.128`, pidió un tramo de 524.288) | ok, `pico= 10.199.016`, 1.122.712 libres al componer, arena de 3.029.194 B en **7 tramos** (el menor de **262.144**) |
| VOLVER | no se llegó | ok, `pico= 9.506.536`, 1.245.608 libres, arena de 2.586.489 B en **5 tramos**, **1 tramo devuelto** al montón (3.145.728 B) |

Las tres palancas, y por qué cada una toca el pico:

1. **Scratch de lectura por archivo** (`SCRATCH_POR_ARCHIVO`). El buffer se pedía con el tamaño del `.dt1` más
   grande del nivel y se mantenía tomado hasta soltar el préstamo al final de la carga. Como el buffer es para
   UN archivo, ahora se pide del tamaño de ese archivo y se devuelve en cuanto sus bloques están copiados al
   arena. Medido en la corrida de referencia: **−687.152 B de pico**. El liberador se llama 8 veces en ENTRAR,
   17 en la TRANSICION y 25 en la VUELTA (`scratchBytes= 4.350.665 / 9.149.753 / 13.500.418`), o sea que el
   pico deja de ser acumulativo.
2. **El tramo del arena lo dimensiona el montón** (`TRAMO_ARENA`). El tramo fijo de 512 KB moría por
   contigüidad aunque el montón tuviera 1,4 MB libres. Antes de pedirlo se pregunta a `d2c_servible(n)` cuánto
   puede servir, y se pide ese tamaño si está entre lo que falta y el fijo. En la TRANSICION el tramo menor
   terminó siendo de **262.144 B** y así entró donde el de 524.288 no entraba.
3. **Los tramos se devuelven al cambiar de nivel** (`TRAMOS_AL_CAMBIAR`). DC-002.88 los dejaba tomados para
   reusarlos, y eso convertía 3.145.728 B en un bloque que el montón no recuperaba nunca; en la VUELTA eso
   bloqueaba justo el pedido contiguo del `.dt1`. Ahora `reiniciarArenaDeTiles` los devuelve (medido: 1 tramo,
   3.145.728 B) y el nivel nuevo los vuelve a pedir con la sonda.

## 3. Lo que apareció detrás: el semiespacio, y la reapertura que lo llenaba

Con el montón de C resuelto, la corrida del ciclo (arena encendida, palancas encendidas) murió en el
**semiespacio** de la VUELTA:

```
ARQ-MARCA gc-vivo v=1824256         <- 99,4 % de 1.835.008
MAPA109: CICLO gcPercent 100 -> 95
... 1.241 recolecciones y el cuadro de la VUELTA nunca se dibuja
```

Lo primero que se probó fue el umbral del recolector (95 → 98 %), que es lo que DC-002.109 había tenido que
mover para la TRANSICION. **No alcanzó**: con 98 % el conjunto vivo llegó a 1.813.680 B (98,8 %) y siguió
recoleccionando. O sea que el problema no era el umbral sino **qué ocupa el semiespacio**.

El censo por paso de la fase lo dejó claro (líneas `MAPA103: SEMIESPACIO <fase>-<paso>`, que esta tarjeta
agregó para los cuatro pasos de `correrFase`). **Con la reapertura puesta** (el camino de DC-002.109):

| paso de la fase | TRANSICION | VUELTA |
|---|---|---|
| arranca | 1.226.712 B (66 %) | 1.550.416 B (84 %) |
| después de soltar la cache del nivel que se deja | 1.227.048 B | 1.550.720 B |
| **después de reabrir fuentes y tablas** | **1.347.568 B (+120.520)** | **1.696.840 B (+146.120)** |
| después de soltar las casillas | 1.347.760 B | 1.697.024 B |
| después de la recolección (antes de componer) | 934.464 B (50 %) | **1.309.520 B (71 %)** |

La base de la VUELTA quedaba **375.056 B arriba** de la de la TRANSICION y la fase no tiene ese lugar. **Sin la
reapertura** (`soltarEnElCiclo = false`) los mismos pasos dan:

| paso de la fase | TRANSICION | VUELTA |
|---|---|---|
| arranca | 1.289.568 B (70 %) | 1.447.352 B (78 %) |
| reabrir (se saltea: no hay nada que reabrir) | 1.289.984 B | 1.447.736 B |
| después de la recolección (antes de componer) | 763.696 B (41 %) | **1.022.736 B (55 %)** |

La causa de los 375.056 B es la **reapertura**: soltar las fuentes MPQ y las tablas después de componer obliga a
la fase siguiente a abrirlas y cargarlas de nuevo (eso es lo que media DC-002.109), y cada reapertura deja sus
estructuras de Go en el semiespacio; la recolección no las puede tirar porque el **barrido de pila de libgodc es
conservador** y los punteros de la ronda anterior siguen en marcos muertos.

Dos recolecciones se probaron para el mismo problema y las dos quedaron medidas:

* `recolectarDespuesDeCargarElDs1` — un `runtime.GC()` **dentro** de `componerNivel`, con el `.ds1` nuevo ya
  cargado y antes de tocar un `.dt1` (el marco ya se reescribió, el `Stamp` es una variable local y el `.ds1`
  vive en el montón de C, así que no hay nada que se pueda mover bajo los pies del parser). **Aporta poco**:
  23.680 B medidos. Queda como palanca, encendida, porque es gratis y segura.
* `soltarEnElCiclo = false` (**el que decide**) — en el ciclo no se sueltan las fuentes ni las tablas, así que
  no hay nada que reabrir (`saltarLaReapertura`). Con eso la base de la VUELTA baja a **1.022.736 B** y la fase
  pide **1.596.640 B (87 %)** en vez de 1.824.256 B (99,4 %), con el umbral del recolector **en el 95 % de
  DC-002.109, sin tocarlo**. El pico del semiespacio de la VUELTA en la corrida del criterio es **1.734.728 B
  (94 %)** y se alcanza ya con el cuadro dibujado (la cache del nivel).

Lo que se paga: 679.440 B de fuentes y tablas quedan tomados toda la corrida. Con las palancas de esta tarjeta
el ciclo tiene 1.122.712 B libres en el pico, así que entran. **Con el ciclo apagado nada de esto aplica**: la
corrida de referencia sigue soltando todo después del único cuadro, como en DC-002.106 (§1.D).

## 4. Lo que esta tarjeta dejó hecho

* `d2core/d2asset/lectura_de_tiles.go` — el buffer de lectura se pide **por archivo** (`scratchPorArchivo`), se
  suelta en cuanto sus tiles están conservados (`soltarScratchDeLectura`) y no se pide en
  `PrepararArenaDeBloquesUsados`; el tramo del arena consulta la **sonda del montón**; `reiniciarArenaDeTiles`
  **devuelve los tramos**; `EstadoDeLosTiles` publica `ScratchPorArchivo`, `ScratchSoltados`, `ScratchBytes`,
  `PicoDelMonton`, `TramoMenor`, `TramosAlReiniciar`, `TramosDevueltos` y `TramosBytes`, y `muestrearElMonton()`
  registra el pico **dentro** del bucle de carga (el POC no puede muestrear ahí).
* `d2core/d2asset/arena_soltar_101.go` — las palancas (`UsarScratchPorArchivo`, `UsarSondaDelMontonDeC`,
  `UsarMedidorDelMontonDeC`, `UsarSoltarTramosAlReiniciar`), la consulta `PicoDelMontonDeC()` y
  `SoltarPrestamoDeLecturaDeTiles` delegando en el mismo liberador que la palanca por archivo, para que el
  instrumento cuente igual por los dos caminos.
* `d2core/d2map/d2mapengine/engine.go` — al terminar de cargar los `.dt1` del nivel se suelta el préstamo de
  lectura (`scratch de lectura soltado con el nivel cargado: N B`).
* `console/dc/poc/mapa-103/knobs.go` — `scratchPorArchivo`, `tramoDelArenaPorElMonton`,
  `soltarTramosAlCambiarDeNivel`, `recolectarDespuesDeCargarElDs1`, `saltarLaReapertura` y `soltarEnElCiclo`,
  cada una con su `POR QUE` y su número; todas pisables por entorno.
* `console/dc/poc/mapa-103/main.go` y `fases_109.go` — instalación de las palancas, censo del semiespacio en
  **los cuatro pasos** de la fase, `MAPA111:` en el arranque, y la decisión de no soltar fuentes ni tablas en el
  ciclo con su comentario y sus números.
* `scripts/console/build-mapa-103.sh` — `SCRATCH_POR_ARCHIVO`, `TRAMO_ARENA`, `TRAMOS_AL_CAMBIAR`,
  `SALTAR_REAPERTURA` y `SOLTAR_EN_EL_CICLO` con `pisa_knob` (cada corrida imprime qué palancas pisó).
* `d2core/d2asset/scratch_111_test.go` — pruebas del buffer por archivo: con la palanca encendida
  `PrepararArenaDeBloquesUsados` no pide el buffer del nivel, el liberador inyectado cuenta llamadas y bytes, y
  con la palanca apagada el camino es el de siempre.

## 5. Trampa del toolchain (para el próximo)

`go build ./console/dc/poc/mapa-103/` **falla en el host** con
`runtime.main_main·f: function main is undeclared in the main package`: el POC compila para SH-4 por build tags.
En el host se valida con `go vet` y `go test` sobre los paquetes de `d2core` y con `go test -tags` si hiciera
falta; el binario del POC se valida con el build de Dreamcast y con Flycast, no con `go build`.

## 6. Qué queda para la tarjeta siguiente

* El pico del ciclo (10.199.016 B de 10.227.936) y el semiespacio de la VUELTA (1.596.640 B de 1.835.008) pasan,
  pero **los dos por debajo del 90 %**: la próxima fase que agregue un nivel más (o un `.ds1` más grande que el
  pueblo) va a volver a tocar el techo. Lo que falta de verdad es que el nivel que se deja **no** deje vivos sus
  409.096 B de reapertura; medido acá, la única palanca que lo logra es no soltar nada.
* El montón de C sigue siendo el que decide el tamaño de los tramos: la sonda `d2c_servible` funciona, pero el
  arena no puede crecer más allá de `10.227.936 B` de arena. Con `arena= 10.627.936` en la TRANSICION quedan
  `1.122.712 B` libres: eso es todo el colchón.

## 7. Cómo se reproduce

```
# host: las pruebas de los paquetes y del buffer por archivo
go vet ./d2core/d2asset/ ./d2core/d2map/... ./console/dc/poc/mapa-103/
go test -count=1 ./d2core/d2asset/ ./d2core/d2map/...
go test -count=1 -run 'Scratch|Arena|Tramo|Bloques' -v ./d2core/d2asset/

# host: los tamaños de los .dt1 del pueblo y de sus bloques
go run ./console/dc/medidas/bloques

# host: el cuadro de referencia (PNG + crc32) que la consola tiene que reproducir
go run ./console/dc/medidas/render-294 -cuadros 1

# consola: la corrida del criterio (las tres fases, arena y palancas de .111)
BASE_GOX=~/gox108 FASE=encendido ORDEN_FASES=a ARENA_DS1=encendido \
  bash scripts/console/build-mapa-103.sh /tmp/dcbuild/criterio
CAPTURE=spectacle bash scripts/console/run-flycast.sh /tmp/dcbuild/criterio/disc.gdi 340 /tmp/cap-criterio

# consola: el A/B que reproduce DC-002.110 (palancas de .111 apagadas)
BASE_GOX=~/gox108 FASE=encendido ORDEN_FASES=a ARENA_DS1=encendido \
  SCRATCH_POR_ARCHIVO=0 TRAMO_ARENA=0 TRAMOS_AL_CAMBIAR=0 SOLTAR_EN_EL_CICLO=1 SALTAR_REAPERTURA=0 \
  bash scripts/console/build-mapa-103.sh /tmp/dcbuild/sin-palancas

# consola: el A/B de la arena
BASE_GOX=~/gox108 FASE=encendido ORDEN_FASES=a ARENA_DS1=apagado \
  bash scripts/console/build-mapa-103.sh /tmp/dcbuild/sin-arena

# consola: la corrida de referencia (ciclo apagado, el camino de DC-002.106)
BASE_GOX=~/gox108 bash scripts/console/build-mapa-103.sh /tmp/dcbuild/referencia
CAPTURE=none bash scripts/console/run-flycast.sh /tmp/dcbuild/referencia/disc.gdi 200 /tmp/cap-referencia
```

## 8. Archivos de esta evidencia

| archivo | qué es |
|---|---|
| `consola-criterio-arena-encendida.log` | la corrida del criterio: las tres fases dibujadas, `crc32` de VOLVER igual al de ENTRAR, censo del semiespacio por paso y arena del `.ds1` pedida/soltada 3 veces |
| `consola-ab-palancas-111-apagadas.log` | el A/B sin las tres palancas de esta tarjeta: reproduce el `gc_external_alloc: OOM` de DC-002.110 en la TRANSICION (`used= 10.285.248`) |
| `consola-ab-arena-apagada.log` | el A/B de la arena del `.ds1`: la VUELTA muere en el semiespacio (`GC to-space overflow`) |
| `consola-referencia-ciclo-apagado.log` | la corrida de un solo cuadro: mismo `crc32`, misma cache, pico del montón de C de 8.409.464 B |
| `host-cuadro-640x480.png` | el cuadro de referencia del host (`render-294`), `sha256 2b6c64db…`: **byte por byte el mismo PNG de DC-002.108** |
| `host-render-294-111.log` | el medidor del host: `crc32= 1437022778`, 307.196 px de 307.200 |
| `host-pruebas.txt` | `go vet` y `go test` de `d2asset` y `d2map`, y las pruebas del buffer por archivo |
| `host-medida-bloques-111.txt` | los `.dt1` del pueblo: crudo, bloques, tiles y el bloque más grande de cada uno |

**Sobre las capturas de pantalla.** La deuda de DC-002.109 sigue igual: `CAPTURE=spectacle` fotografía el
escritorio entero y la ventana de Flycast queda chica entre las demás ventanas, así que no sirve como prueba de
los cuadros. Lo que sí es prueba es el `crc32` del cuadro, que es lo que mide el POC en la consola, y el PNG de
referencia del host: el cuadro de la VUELTA en la consola da **el mismo `crc32`** que ese PNG.


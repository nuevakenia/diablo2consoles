# DC-002.109 — el cambio de nivel: entrar, la transición y volver

Qué se midió en esta tarjeta: **las tres fases del cambio de nivel en una sola corrida de Flycast**, con la
cache de tiles viviendo en la VRAM del PVR (DC-002.108) y las fuentes MPQ y las tablas reabriéndose en cada
fase (el camino de vuelta de DC-002.105). La decisión que la tarjeta pide elegir es el ORDEN:

```
a   soltar la cache del nivel que se deja ANTES de reabrir las fuentes y componer
b   no soltar nada: la cache vieja sigue tomada mientras se reabre y se compone
```

Los dos órdenes existen en el mismo binario (`FASE=encendido ORDEN_FASES=a|b` del build del POC), se midieron
los dos, y el que se elige es el **(a)** — por las tres razones del final.

## Lo que se ve

| fase | qué compone | crc32 del cuadro | ms del cuadro |
|---|---|---|---|
| ENTRAR (cuadro 1 y 2) | `Act1/Town/TownN1.ds1` (pueblo) | **1437022778** = el del host | 748 / 709 |
| TRANSICION | `Act1/Town/TownE1.ds1` (preset 1, archivo 1) | **584380401** = el del host | 762 |
| VOLVER | `Act1/Town/TownN1.ds1` otra vez | — (no llega: ver el muro) | — |

`host-cambio-de-nivel-640x480.png` es el cuadro del pueblo que dibuja el medidor del host, y la consola pinta
los mismos bytes: el crc32 del cuadro de la consola es el mismo número que el del host en las tres fases
(1437022778 y 584380401). El crc32 lo calcula el POC sobre el lienzo que después se presenta, así que es la
medida del cuadro, no del archivo.

Mismo cuadro en las dos fases y en los dos órdenes: `pixelesNoVacios=307200 de 307200` y `crc32RGB=-216955579`.

**Captura de pantalla: no salió, y queda anotado.** Las corridas se capturaron con `CAPTURE=spectacle`
(`spectacle -f`, escritorio entero, y `flycast-adelante.sh` deja la ventana en POS). En esta sesión la ventana
no quedó en POS —el escritorio tenía el navegador y el editor encima— y los cuadros de las fases caen entre
capturas (el ciclo se lleva ~60 s: a los 25/40 s todavía no está el cuadro de la TRANSICION y a los 60 s ya
rebotó por el OOM de VOLVER). Las capturas salieron con el escritorio y no con el emulador; queda como deuda
de herramienta de la carpeta `scripts/console` (mover la ventana con el `geo` de `flycast-adelante.sh` y
capturar cada 5 s durante el ciclo).

## Orden (a) — el que se elige (log: `consola-ciclo-de-fases-orden-a.log`)

```
despues del cuadro 1 (cache en VRAM)              monton de C libre  1.644.280 B
soltar el arena de bloques (contrato .101)                           4.265.760 B
TRANSICION: soltar la cache del nivel que se va  2.723.840 B (VRAM)
            reabrir fuentes + tablas             679.440 B           3.586.320 B
            componer TownE1 (arena 3.029.194 B)  30.324 ms             378.288 B
            soltar tablas + fuentes              570.720 B             949.688 B
            cuadro                               crc32 584380401       762 ms
VOLVER:     soltar la cache de la transicion     2.088.960 B (VRAM)
            reabrir fuentes + tablas             679.440 B
            componer TownN1                      NO ENTRA (ver el muro)
```

El montón de C **nunca es la pared** en el orden (a): con la cache vieja devuelta antes de reabrir, la
transición compone su arena (3.029.194 B) y todavía quedan **949.688 B libres** después de soltar las tablas y
las fuentes del cuadro.

## Orden (b) — también dibuja la transición, y deja menos (log: `consola-ciclo-de-fases-orden-b.log`)

Sin soltar la cache del nivel que se deja, la TRANSICION **también compone y dibuja** su cuadro (mismo crc32
584380401), porque el tramo reusa sus 8 bloques desde el byte 0: lo que cuesta es el resto del camino.

```
                                orden (a)          orden (b)
semiespacio al componer        1.705.920 (92 %)   1.736.912 (94 %)   +31.000 B
semiespacio al fin del cuadro  1.536.024 (83 %)   1.562.200 (85 %)
monton de C al fin del cuadro    949.688 B          949.912 B
VRAM disponible al fin           1.422.824 B        787.880 B          la mitad
bloques del tramo del cuadro     6 (2.088.960 B)    8 (2.723.840 B)
```

## Por qué se elige (a)

1. **Es el único que existe cuando la cache vive en el montón de C.** El argumento de la tarjeta es ese, y en el
   modo del montón (`ENVRAM=0`) se midió tal cual: soltando la cache vieja antes de reabrir, la TRANSICION
   compone (quedan 379.792 B) — pero al pedir la cache del nivel nuevo, que son 2.652.160 B, no hay dónde:
   `ARQ-MARCA OOM-throw` y `fatal error: out of memory` en el primer cuadro
   (`consola-cache-en-el-monton-envram-0.log`). La cache y el nivel compuesto no
   conviven en el montón; con el (a) al menos se devuelven 1.538.120 B antes de reabrir.
2. **Deja más lugar para la vuelta.** El (b) se queda con los 8 bloques viejos (2.723.840 B de VRAM tomados y
   787.880 B disponibles); el (a) los devuelve y deja 1.422.824 B, que es lo que la vuelta va a necesitar.
3. **Cuesta 31.000 B menos de semiespacio**, que es justo el recurso que después no alcanza (ver el muro).

## El muro de la vuelta: el SEMIESPACIO del recolector

El montón de C entra en las tres fases; el que no entra es el semiespacio (1.835.008 B). Los números del
orden (a), medidos con `gc_stats` (puente nuevo `d2c_gc_usado/total/colecciones` del shim):

```
                                        semiespacio usado        del total
TRANSICION antes de componer                    900.560            49 %
TRANSICION despues de parsear el .ds1         1.323.288            72 %   (+422.728)
TRANSICION despues de componer                1.705.920            92 %   (+382.632)
TRANSICION fin del cuadro                     1.536.024            83 %
VOLVER antes de componer                      1.313.744            71 %   (= 900.560 + 413.184)
VOLVER despues de parsear el .ds1             1.735.048            94 %   (+421.304)
VOLVER: componer                                 NO ENTRA (1.735.048 + ~382.632 > 1.835.008)
```

O sea: **un nivel compuesto deja ~413.000 B vivos en el semiespacio después de una recolección completa**, y
son justo los del `.ds1` del nivel que se dejó (la misma cuenta que da el salto del parseo). Mientras ese
`.ds1` siga siendo alcanzable, la vuelta no tiene dónde poner el suyo: los dos `.ds1` no conviven en 1,75 MB.

Lo que se probó para liberarlo y **NO** fue la causa (todo medido, no supuesto):

* `runtime.GC()` antes de componer, con `debug.SetGCPercent(95)` (el umbral del 75 % del runtime hace que el
  recolector corra en CADA asignación y no libere nada: medido, 1.585 recolecciones seguidas sin progreso).
* Pisar con ceros la basura de la pila (el recolector de libgodc recorre la pila de forma conservadora):
  recuperó 17.000 B de 413.000 — no era eso, y por eso el código no lo lleva.
* `//go:noinline` en el que compone el nivel (por si el `.ds1` quedaba clavado en el marco inlineado del que
  llama): sin cambio.
* `SoltarLasCasillasDelNivelAnterior` (lo que sí sirvió, ver abajo): las casillas no son el `.ds1`.

El siguiente paso está pedido en la tarjeta hija: **sacar el `.ds1` de un nivel del semiespacio** (el camino que
ya se usó para el arena de bloques y para la cache: que viva en el montón de C con un liberador explícito), o
encontrar qué raíz lo deja vivo.

## Lo que esta tarjeta arregló de paso (los tres los encontró el cambio de nivel)

1. **`generateWallCache` indexaba a ciegas la pieza izquierda de la esquina** (`newTileOptions[tile.RandomIndex]`
   sin mirar si el nivel la trae): con la TRANSICION el juego moría con `panic: index out of range` en el
   primer cuadro del nivel nuevo. Ahora, si el `.dt1` del nivel no tiene esa variante, la pared se arma con la
   derecha (el mismo camino que ya usaba el renderer cuando la izquierda salía más alta).
2. **Un bloque del tramo REUSADO se entregaba sucio**: el decodificador del `.dt1` solo escribe donde algún
   bloque del tile pinta, así que el bloque reusado arrastraba los pixeles del nivel anterior y el cuadro del
   nivel nuevo salía con la huella del viejo (medido en el host: crc32 1533561935 en vez de 1437022778). Ahora
   se limpia con el `memset` de la VRAM (`pvr.VramPonerEnCero`) o con un bucle de Go en la PC.
3. **`ResetMapParaStamp` limpiaba las casillas DESPUÉS de parsear el `.ds1` nuevo**: las 2.337 casillas x 3
   rebanadas del nivel que se iba (~250.000 B en el semiespacio, alcanzables por el bloque externo `m.tiles`)
   seguían vivas durante el parseo. `SoltarLasCasillasDelNivelAnterior`, llamada ANTES del parseo, las tira:
   medido, el semiespacio antes de componer bajó de 1.160.976 a 900.560 B.

## La corrida de referencia no se movió (log: `consola-referencia-sin-ciclo.log`)

La palanca está apagada por default (`cicloDeFases = "apagado"`). Con el mismo binario y sin `FASE=`, la
corrida sale igual que la de DC-002.106/.108: tres cuadros con crc32 1437022778, 307.196 de 307.200 píxeles,
86 superficies / 2.723.840 B, tramo de 8 bloques con 84 imágenes y 2 derrames (71.680 B), ms 746/628/703, y
**cero líneas `MAPA109`** en el log.

## Cómo se reproduce

```
# host: el cambio de nivel sin consola (mismos números de píxeles)
go run ./console/dc/medidas/render-294 -perezoso -indices -tramo 2723840 -pedazo 348160 \
    -cuadros 1 -cambio -cambio-preset 1 -cambio-archivo 1 -out /tmp/cuadro.png

# consola: las tres fases, orden (a) y orden (b)
BASE_GOX=~/gox108 FASE=encendido ORDEN_FASES=a bash scripts/console/build-mapa-103.sh /tmp/dcbuild/x
CAPTURE=spectacle bash scripts/console/run-flycast.sh /tmp/dcbuild/x/disc.gdi 280 /tmp/cap

# consola: la corrida de referencia (sin ciclo: el log de DC-002.106/.108, una línea de más)
BASE_GOX=~/gox108 bash scripts/console/build-mapa-103.sh /tmp/dcbuild/ref
```

Las tres fases se prenden con un solo nombre: `FASE=encendido` (con `ORDEN_FASES=a|b`), y el `.ds1` de la
transición con `TPRESET=`/`TARCHIVO=`. Apagado (`FASE=apagado`, el default de `knobs.go`) no se ejecuta una
línea del ciclo: la corrida de referencia queda como la dejó DC-002.106/.108.

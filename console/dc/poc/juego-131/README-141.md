# DC-002.141 — EL CUADRO CAMINANDO: el lienzo se corre y el mapa se repinta solo donde entra

La mitad que faltaba de DC-002.139. La otra mitad (el cuadro **parado**) quedó en 26-43 fps porque el mapa
no se repinta si la cámara no se movió. **Caminando** el cuadro se iba en repintar los 248 tiles del mapa
entero: 60,4 ms medidos, 77-92 ms por cuadro, **11-13 fps**.

Esta tarjeta ataca eso con otra idea que la del plan original (el lienzo de scroll con UV en una textura más
grande): **correr el lienzo en la VRAM con una copia en el lugar (`memmove`) y repintar del mapa solo lo que
la copia no trajo**, con una lista de recortes por rectángulo. No pide un byte más de VRAM y no cambia ni el
arte ni la lógica del juego.

## Los números (Flycast, la misma demo de DC-002.140, 70-80 s por corrida)

Ventanas de 30 cuadros (`DEMO140: FPS`), A/B **en el mismo binario** con la palanca `SCROLL_141`:

| cuadro caminando | antes (`SCROLL_141=no`) | después (`si`) |
|---|---|---|
| mapa por cuadro | **60,4 ms** (repintado entero, 248 tiles) | **15-26 ms** (la copia + los recortes) |
| ventanas caminando medidas | 94 / 74 / 85 ms → **11 / 13 / 12 fps** | 72 / 45 / 52 / 32 / 70 ms |
| fps de esas ventanas | 11-13 | **14 / 22 / 19 / 31 / 14** (mediana 19-22) |
| ventanas paradas (no se tocan) | 23-38 ms → 26-42 fps | 23-36 ms → 27-42 fps |
| `fpsMedio` de la corrida | 30 | 30 |
| crc32 del cuadro 1 | −1654008076 | **−1654008076** (igual) |

**Lo que está cumplido y lo que no, sin maquillaje.** El criterio de la tarjeta era "20 fps o más en las
ventanas donde el jugador camina". Se cumple en **tres de las cinco** ventanas caminando medidas de la corrida
(22, 19-31 fps) y **no** en las dos peores (14 fps), donde la cámara da pasos grandes (8-9 ms de copia por
cuadro). El mapa bajó **−75 %** y el cuadro caminando pasó de 77-92 ms a 32-72 ms; el muro que queda **no es
el mapa** (ver "lo que falta", abajo).

## Qué hace la palanca

`SCROLL_141=si` (default) agrega un **tercer camino** al dibujo del mapa, al lado de los dos de DC-002.139:

```
cuadro entero   -> rendererDelMapa.Render()            (los 3 primeros cuadros, otro nivel, el panel, tiles animados)
cuadro quieto   -> restaurarElMundo139()               (DC-002.139: el mapa debajo de la caja de cada ente)
cuadro CAMINANDO-> MoverLienzo(sx, sy) + Render CON RECORTES
```

1. **La copia.** `pvr.MoverLienzo(sx, sy)` mueve el contenido del lienzo (RGBA, en la VRAM) `sx`/`sy` píxeles
   **en el lugar**, con semántica `memmove` (el sentido de la copia se elige solo), en C y de a 64 bits por
   vuelta, desenrollado de a cuatro lecturas para que el SH-4 las tenga en vuelo: **4,3-9,2 ms** medidos para
   ~1 MB. Es todo el mapa, movido.
2. **Los recortes.** Después de correr el lienzo hay que devolverle el mapa a unos pocos pedazos, y el
   renderer del mapa corre **una sola vez** con una **lista de rectángulos** (`RecortarA`, ver
   `d2core/d2render/pvr/recortes_141.go`): un solo rectángulo obligaría a llamar a `Render` una vez por
   pedazo y cada llamada recorre los 248 tiles con sus cuatro pasadas.
   - **las franjas que entran** por el borde (`|sx|` columnas y/o `|sy|` filas), recortadas además contra el
     pie del HUD (abajo), el HUD de arriba y la caja medida del minimapa: lo que el panel tapa de verdad no se
     repinta;
   - **las cajas de los entes del cuadro anterior** (jugador, bichos, NPCs, objetos) corridas con la copia, si
     no queda el fantasma del sprite viejo;
   - **las cajas de los paneles ENTERAS** (HUD de arriba, pie del HUD, minimapa) más la banda que asoma del
     panel del cuadro anterior corrido. Enteras y no sólo la banda, porque **los paneles no son opacos**: con
     `hudPiezas=esenciales` no se carga el panel de fondo, así que de la banda del pie lo único que se pinta son
     los globos (5.024 de 6.400 píxeles: 1.376 agujeros), el cinturón (4.000 de 4.000, ése sí) y el texto de la
     zona; el resto de la banda es **mundo a la vista**. Con el corrimiento, los agujeros de un globo dejan ver
     el globo del cuadro anterior unos píxeles más allá y el texto deja su fantasma. Medido: comparando el
     cuadro terminado, esos agujeros eran **74 de 300 bloques distintos**; repintando el mapa debajo, 0.
3. **Los píxeles que no se pintan.** El renderer recorre igual los ~198 tiles del cuadro y le pide el blit a
   cada uno; el que no toca ningún recorte vuelve **antes** de armar la tabla de 256 palabras (que es lo caro
   del blit). Medido en la primera corrida de esta tarjeta, sin esa comprobación: 22-28 ms de "franjas" para
   25.000 píxeles (los píxeles son 1 ms). Censo del tramo: `JUEGO141: BLITS pedidos= 198 saltados= 144
   pintados= 54`.

## La otra mitad: la conversión del lienzo por filas (`CONV_FILAS_141`)

Al correr el lienzo **cambia todo el cuadro**, así que la presentación vuelve a convertir los 307.200 píxeles:
era el segundo cuello. El camino de DC-002.135 recorre la **textura** y lee el lienzo por la tabla del
twiddle, o sea **lecturas salteadas** contra la VRAM. `bloque_por_filas141` hace lo mismo al revés: recorre el
**lienzo fila por fila** (una lectura de 32 bits por píxel, la fila seguida) y escribe cada resultado en su
posición twiddleada del buffer de 2 KiB, que está en RAM. Cuatro lecturas antes de los cuatro depósitos, que
no es un detalle: sin desenrollar, la conversión del cuadro entero **subía de 3,1 a 33 ms**.

**La prueba de que los bytes son los mismos** (no se cree, se mide): con `COMPARAR_141=si` el binario convierte
cada bloque por los DOS caminos y cuenta las palabras de 16 bits que no coinciden:

```
JUEGO141: CONVERSION cuadros= 120 bloques= 2519 palabrasDistintas= 0 porFilas= si
... 69 líneas, 55.896 bloques verificados, 0 palabras distintas ...
```

Y la prueba de que el **cuadro** es el mismo, en la corrida de la prueba (`COMPARAR_141=si`): al final del
cuadro, con el mapa, los entes, el HUD y el minimapa ya puestos, se pinta el cuadro ENTERO de nuevo (mapa
entero) y se comparan los dos lienzos **bloque por bloque de 32x32, los 300 bloques sin excepción**:

```
JUEGO141: COMPARACION cuadro= 6 sx= -213 sy= 0 cuadrante= 2 ... igual= true bloquesDistintos= 0 deComparables= 300
JUEGO141: COMPARACION cuadro= 9 sx= 212 sy= 0 cuadrante= 1 ... igual= true bloquesDistintos= 0 deComparables= 300
JUEGO141: COMPARACION cuadro= 10 sx= 0 sy= 106 cuadrante= 4 ... igual= true bloquesDistintos= 0 deComparables= 300
JUEGO141: COMPARACION cuadro= 13 sx= 0 sy= -107 cuadrante= 8 ... igual= true bloquesDistintos= 0 deComparables= 300
JUEGO141: COMPARACION cuadro= 15 sx= 95 sy= 95 cuadrante= 5 ... igual= true bloquesDistintos= 0 deComparables= 300
JUEGO141: COMPARACION cuadro= 17 sx= -95 sy= -95 cuadrante= 10 ... igual= true bloquesDistintos= 0 deComparables= 300
JUEGO141: COMPARACIONES cuadros= 30 hechas= 8 cuadrantes= 1334 saltadasPorPanel= 0 porCuadrante= 2
```

O sea: **el cuadro caminando pinta exactamente los mismos píxeles que un repintado entero**, los 300 bloques,
con el jugador y los bichos en movimiento, en los **cuatro cuadrantes** del corrimiento (los dos signos de cada
eje, y las dos diagonales: la copia, las franjas verticales y las bandas horizontales tienen ramas distintas
según el signo, así que un tramo recto no prueba nada).

La comparación va sobre el cuadro **terminado** y no sobre el mapa pelado a propósito: antes se salteaban los
bloques que tocaban las cajas de los paneles (120 de 300 comparables, el 60 % del lienzo sin prueba) porque
"los paneles son opacos". No lo son, y esa excusa tapaba tres defectos que esta revisión encontró y arregló:

1. **La copia del lienzo con `sy < 0` estaba mal** (`d2c_pvr_pantalla_mover`, en `pvr_pantalla.c`): el bucle
   usaba el mismo índice como origen y como base del destino, así que escribía en las filas `-|sy|..-1` —**antes
   del framebuffer**, memoria fuera de rango— y dejaba sin copiar las últimas `|sy|` filas. Con `|sy|` chico el
   error quedaba tapado por los recortes; con `|sy| = 106` se veía como una banda de 10 filas con el cuadro de
   ayer. Ahora el origen es `y - sy` y el destino `y`.
2. **La banda del pie no cubría los globos**: se pegan con `globoVidaDY = -13`, así que su caja arranca en 387 y
   sobresale 13 px por encima de los 400. `altoDelPie141` pasó de 80 a 96 (arranca en 384).
3. **Los paneles no son opacos**: el mapa se repinta debajo de las tres cajas (ver el punto 2 de arriba).

## Lo que falta (y por qué no está acá)

El cuadro caminando queda en 32-72 ms. Desglosado (ventana de 300 cuadros, `JUEGO134: ETAPAS` +
`PVR132: CUADRO`): mapa 15-26 + presentación **26** + entes/HUD/minimapa ~10. La presentación es otra vez el
primer costo, y **no es la conversión** (ahora son 3 ms): es la **subida de la textura entera y la relectura**,
que se paga porque al correr el lienzo cambian los 300 bloques. El plan original de la tarjeta lo resolvía
corriendo **la textura** (anillo + UV en el quad): con el lienzo corriendo, el cuadro cambia en todos los
bloques y no hay forma de convertir solo la franja. Eso es la tarjeta siguiente (t_ pendiente de crear por el
dueño del tablero): **anillo de textura con UV**, que necesita ~1,5 MB de VRAM para 1024x768 y toca la
presentación de DC-002.140, no el mapa.

### Los números de la revisión (con `MEDIR_CUADRO=no`, mismo guion, mismo binario salvo la palanca)

| | `SCROLL_141=si` | `SCROLL_141=no` |
|---|---|---|
| `ETAPAS mapa` (µs por cuadro, promedio de 7 tramos) | **2.065** | 2.549 |
| `ETAPAS cuadroTotal` (µs por cuadro) | **3.050** | 3.420 |
| `DEMO140: FPS` | 7-8 | 7-8 |

O sea: el mapa baja ~19 % (0,48 ms de 2,55) y el cuadro del POC ~11 % (0,37 ms de 3,42), pero **el fps del
emulador no se mueve**. La razón está en el mismo registro: de los ~120 ms por cuadro que canta `DEMO140: FPS`,
sólo ~3 ms son del POC (`cuadroTotal`); los otros ~117 ms son de Flycast presentando (subida de textura, blit,
vsync). **El criterio de "20 fps caminando" no se puede medir así**: en el emulador está tapado por su propia
presentación, y con 117 ms fijos por cuadro no lo alcanza ninguna optimización del mapa. Queda para la tarjeta
del anillo de textura, y conviene medirlo con el tiempo del POC (`cuadroTotal`), no con el fps del emulador.

El repintado de las cajas de los paneles cuesta caro y se ve en el censo: `pxPaneles` pasó de ~2.600 a ~178.000
por cuadro caminando (el 58 % del lienzo). Es el precio de la corrección: sin él, los agujeros de los paneles
mostraban el fantasma. Se puede bajar repintando sólo el arte NO opaco de cada pieza (globos y texto de la zona:
~15.000 px, y el minimapa es opaco entero), y ésa es la optimización obvia que queda pendiente.

## Cómo se corre

```
SCROLL_141=si CONV_FILAS_141=si bash scripts/console/build-fps-139.sh /tmp/dcbuild/fps141
SEGUNDOS=80 bash scripts/console/run-demo-140.sh /tmp/dcbuild/fps141/disc.gdi /tmp/capturas-fps141
```

(Los directorios de las corridas que sostienen los números de este README son `/tmp/dcbuild/fps141ok3`,
`/tmp/dcbuild/fps141no` y `/tmp/dcbuild/fps141cmp`: los nombres con los que se midieron, no `fps141` pelado.)

Palancas nuevas (todas en `knobs.go`, todas se pueden pisar por el build):
- `SCROLL_141=si|no` — el cuadro caminando. `no` es el camino de DC-002.139, bit a bit (11-13 fps).
- `CONV_FILAS_141=si|no` — la conversión por filas. `no` es el bucle de DC-002.135, en el mismo binario.
- `COMPARAR_141=si|no` — la corrida de la prueba: compara el cuadro caminando contra el repintado entero y
  verifica la conversión bloque por bloque. Cuesta un repintado entero por cuadro comparado.

Líneas de evidencia: `JUEGO141: LIENZO-CAMINA` (con `recortesIgnorados=`, que tiene que quedar en 0),
`JUEGO141: LIENZO-CAMINA PROMEDIO`, `JUEGO141: BLITS`, `JUEGO141: COMPARACION`, `JUEGO141: COMPARACIONES`,
`JUEGO141: CONVERSION`, y las de siempre (`DEMO140: FPS`, `JUEGO134: ETAPAS`, `PVR132: CUADRO`,
`PVR135: PRESENTAR`).

Cuando una comparación falla, el binario imprime además cuatro líneas de diagnóstico que localizan el defecto sin
adivinar: `COMPARACION-DETALLE` (los entes cuya caja cruza el bloque que cambió), `COMPARACION-BLOQUES` (la suma de
colores de cada bloque distinto en los dos cuadros: dice si es un píxel suelto o un pedazo), `COMPARACION-SONDAS`
(la fila de bloques 8 —y 256..287— píxel por píxel: cuántos cambiaron por fila, desde dónde hasta dónde) y
`COMPARACION-RECORTES` (la lista de rectángulos que se repintaron en ese cuadro, copiada antes de que la
referencia la vacíe). Así se encontraron los tres defectos de arriba.

## Código

- `console/dc/std/csrc/pvr_pantalla.c` — `d2c_pvr_pantalla_mover` (la copia en VRAM),
  `bloque_por_filas141` + `convertir_bloque141` + el verificador (`conv_filas141`, `conv_verificar141`).
- `d2core/d2render/pvr/pantalla.go` — `MoverLienzo(sx, sy)`, `MoverLienzoUs`, `ReiniciarMoverLienzo`,
  `VramDelLienzo`.
- `d2core/d2render/pvr/recortes_141.go` — la lista de recortes del destino (`RecortarA`, `SinRecortes`,
  `RecortesPuestos`), `rangoEnRecorte141`, `dentroDeRecortes141`, `tocaAlRecorte141` y el censo de blits.
- `d2core/d2render/pvr/imagen.go` — `clipN`, `clipIgnorados`, `clip [recortesMax141][4]int32` (arreglo fijo:
  en el semiespacio de 1,7 MB no se pide memoria por cuadro).
- `d2core/d2render/pvr/plano_135.go`, `blit_palabras.go`, `surface.go` — los caminos rápidos intersecan el
  rango una vez por rectángulo y por tile; el de bytes comprueba por píxel (y por eso no se usa acá).
- `d2core/d2render/pvr/conversion_141.go` — la palanca y el verificador de la conversión por filas.
- `console/dc/poc/juego-131/mapa_141.go` — la decisión del cuadro, los recortes, la comparación y el censo.
- `console/dc/poc/juego-131/main.go` — el despacho de los tres caminos.
- `console/dc/poc/juego-131/instrumento_134.go` — en un cuadro que camina no se guardan las cajas de los entes
  (la copia las pisa y el mapa se repinta ahí mismo): 2,7 + 4-6 + 1,7 ms que DC-002.139 pagaba por cuadro.

## Mediciones de la corrida

- `SCROLL_141=si CONV_FILAS_141=si` (criterio): `/tmp/capturas-fps141ok3`.
- `SCROLL_141=no` (A/B contra DC-002.139): `/tmp/capturas-fps141-no`.
- La prueba (comparación del cuadro + verificación de la conversión), **revisión con los 300 bloques y los cuatro
  cuadrantes**: `/tmp/capturas-fps141cmp18` (0 bloques distintos en las 8 comparaciones; las corridas del
  diagnóstico fueron `cmp8..cmp17`).
- A/B del ritmo real de la revisión (`MEDIR_CUADRO=no`): `/tmp/capturas-fps141conA` (141 encendido) y
  `/tmp/capturas-fps141conB` (141 apagado).
- Corridas intermedias del camino: `cmp2..cmp6`, `ok`, `ok2`.

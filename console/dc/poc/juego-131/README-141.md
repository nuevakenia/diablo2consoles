# DC-002.141 — EL CUADRO CAMINANDO: el lienzo se corre y el mapa se repinta solo donde entra

La mitad que faltaba de DC-002.139. La otra mitad (el cuadro **parado**) quedó en 30-43 fps porque el mapa
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
| ventanas paradas (no se tocan) | 23-33 ms → 30-42 fps | 23-33 ms → 30-42 fps |
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
   - **las franjas que entran** por el borde (`|sx|` columnas y/o `|sy|` filas), recortadas además contra las
     cajas **opacas** de los paneles, que se vuelven a pintar enteras en el mismo sitio: en la práctica eso
     borra media franja;
   - **las cajas de los entes del cuadro anterior** (jugador, bichos, NPCs, objetos) corridas con la copia, si
     no queda el fantasma del sprite viejo;
   - **las bandas de los paneles** (HUD, pie, minimapa): el panel de este cuadro es opaco y se pinta encima,
     pero el del cuadro anterior quedó corrido y su borde asomaría.
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

Y la prueba de que el **cuadro** es el mismo, en la corrida de la prueba (`COMPARAR_141=si`): después del
cuadro caminando se pinta el mapa ENTERO en el mismo sitio y se comparan los dos lienzos bloque por bloque de
32x32, **fuera de las cajas de los paneles** (ahí los dos caminos son distintos por construcción: el fantasma
del panel lo tapa el panel de este cuadro, que todavía no se dibujó):

```
JUEGO141: COMPARACION cuadro= 116 sx= -93 sy= 0 bloquesDistintos= 0 deComparables= 120
JUEGO141: COMPARACION cuadro= 241 sx= 0 sy= -21 bloquesDistintos= 0 deComparables= 120
```

O sea: **el cuadro caminando pinta exactamente los mismos píxeles que un repintado entero**, en los dos ejes y
con el jugador en movimiento.

## Lo que falta (y por qué no está acá)

El cuadro caminando queda en 32-72 ms. Desglosado (ventana de 300 cuadros, `JUEGO134: ETAPAS` +
`PVR132: CUADRO`): mapa 15-26 + presentación **26** + entes/HUD/minimapa ~10. La presentación es otra vez el
primer costo, y **no es la conversión** (ahora son 3 ms): es la **subida de la textura entera y la relectura**,
que se paga porque al correr el lienzo cambian los 300 bloques. El plan original de la tarjeta lo resolvía
corriendo **la textura** (anillo + UV en el quad): con el lienzo corriendo, el cuadro cambia en todos los
bloques y no hay forma de convertir solo la franja. Eso es la tarjeta siguiente (t_ pendiente de crear por el
dueño del tablero): **anillo de textura con UV**, que necesita ~1,5 MB de VRAM para 1024x768 y toca la
presentación de DC-002.140, no el mapa.

## Cómo se corre

```
SCROLL_141=si CONV_FILAS_141=si bash scripts/console/build-fps-139.sh /tmp/dcbuild/fps141
SEGUNDOS=80 bash scripts/console/run-demo-140.sh /tmp/dcbuild/fps141/disc.gdi /tmp/capturas-fps141
```

Palancas nuevas (todas en `knobs.go`, todas se pueden pisar por el build):
- `SCROLL_141=si|no` — el cuadro caminando. `no` es el camino de DC-002.139, bit a bit (11-13 fps).
- `CONV_FILAS_141=si|no` — la conversión por filas. `no` es el bucle de DC-002.135, en el mismo binario.
- `COMPARAR_141=si|no` — la corrida de la prueba: compara el cuadro caminando contra el repintado entero y
  verifica la conversión bloque por bloque. Cuesta un repintado entero por cuadro comparado.

Líneas de evidencia: `JUEGO141: LIENZO-CAMINA`, `JUEGO141: LIENZO-CAMINA PROMEDIO`, `JUEGO141: BLITS`,
`JUEGO141: COMPARACION`, `JUEGO141: CONVERSION`, y las de siempre (`DEMO140: FPS`, `JUEGO134: ETAPAS`,
`PVR132: CUADRO`, `PVR135: PRESENTAR`).

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
- La prueba (comparación del cuadro + verificación de la conversión): `/tmp/capturas-fps141cmp7`.
- Corridas intermedias del camino: `cmp2..cmp6`, `ok`, `ok2`.

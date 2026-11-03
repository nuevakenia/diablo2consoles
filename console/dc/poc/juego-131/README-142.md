# DC-002.142 — LO QUE COSTABA LA PRESENTACIÓN: la conversión es el 92 %, no la subida (y el anillo no entra)

Esta tarjeta se abrió con un diagnóstico de DC-002.141 que resultó **falso**, y la primera cosa que hizo fue
medirlo bien. El plan que traía (el anillo de textura) no entra en la VRAM mientras los paneles sigan dentro
del lienzo de scroll. Lo que sí quedó, y sirve, es el número verdadero: **de los 26 ms de la presentación de un
cuadro caminando, 23,7 son la CONVERSIÓN del lienzo a 565 y 1,8 la subida a la VRAM.** La subida no era el
problema; ahora además se agrupa en grupos de 2x2 y baja a 1,8 ms (antes 2,4). El problema de fondo del cuadro
caminando está en otro lado y también quedó medido: **el mapa repinta 178.564 px por cuadro adentro de las
cajas de los paneles (≈55 ms)**, no la presentación.

## Los números (Flycast, demo de DC-002.140, A/B en el mismo binario)

### La presentación de un cuadro caminando (`JUEGO142: PRESENTAR-CAMINANDO`, por cuadro, no promediado)

| | `PRES_GRUPO_142=no` (el camino de DC-002.139) | `si` (esta tarjeta) |
|---|---|---|
| copias a la VRAM por cuadro | **300** | **90** |
| `vram_us` (subida) | 2.400-2.504 | **1.776-1.848** |
| `conv_us` (conversión) | 23.392-24.352 | 23.684-24.288 |
| `pintar_us` (el quad) | 2-3 | 2-3 |
| `total_us` | 26.276-28.785 | **25.641-25.907** |
| bloques | 300 | 300 |

La agrupación de la subida gana **0,6-0,7 ms** de 26. Es correcta y es gratis (los cuatro bloques de un grupo
2x2 son contiguos en la textura twiddleada: una copia de 8 KiB = cuatro de 2 KiB), pero no mueve la aguja.

### Por qué la conversión es 23,7 ms: la curva y el control

`PRUEBA_CONV_142=si` convierte N bloques y devuelve los microsegundos (`d2c_pvr_pantalla_prueba_conv142`,
medido con el mismo reloj de la presentación, que cuesta 0,325 µs por llamada — se mide solo, `JUEGO135: RELOJ`).

| bloques | µs | ns por píxel |
|---|---|---|
| 1 | 79 | 77 |
| 5 | 395 | 77 |
| 20 | 1.578 | 77 |
| 40 | 3.160 | 77 |

- **Lineal, sin costo fijo.** 77 ns por píxel, 79 µs por bloque, 23,7 ms por los 300 bloques del cuadro.
- **El `memmove` del lienzo no la encarece:** los mismos 20 bloques dan 1.577 µs ANTES y 1.577 µs DESPUÉS del
  corrimiento, en el mismo cuadro. La sospecha de que la copia dejaba la VRAM "sucia" para la lectura queda
  descartada por medición.
- **El orden de lectura no la cambia:** con `CONV_FILAS_141=si` (por filas de 32 píxeles contiguos) 23,7 ms;
  con `no` (el orden de texel, saltando por el bloque) 23,9 ms. O sea que lo que cuesta no es el bus sino las
  ~15 instrucciones por píxel del SH-4 emulado (~6 ns por instrucción con el JIT de Flycast).
- Un cuadro QUIETO convierte 13-23 bloques y presenta en 1,2-2,2 ms. La cuenta cierra con la misma tasa.

### El cuadro caminando entero, medido (`JUEGO141: LIENZO-CAMINA` + `JUEGO134: ETAPAS`)

| etapa | por cuadro caminando | qué es |
|---|---|---|
| `mover` | 7.789 µs | el `memmove` del lienzo en la VRAM |
| `franjas` | **59.633 µs** | los recortes del mapa, 109 recortes |
| — de eso, `pxPaneles` | **178.564 px (≈55 ms)** | el interior de las TRES cajas de los paneles |
| — de eso, `pxFranjas` + `pxCajas` | 2.958 + 8.484 px (≈3,6 ms) | las bandas que entran y las cajas de los entes |
| entes + HUD + minimapa | ≈14.000 µs | jugadores 2.200 + bichos 5.800 + npcs 1.700 + hud 2.100 + minimapa 7.300 |
| presentación | 25.700 µs | conversión 23.700 + subida 1.800 |
| **total** | **≈107 ms → 9 fps caminando** | las ventanas de 30 cuadros mezclan 11 caminando con 19 quietos: 48-63 ms → 16-20 fps |

**El elefante no es la presentación: es el repintado del interior de los paneles.** El mapa se pinta a
≈310 ns por píxel (el blit del POC en Go sobre el SH-4, con paleta y clip por píxel), así que 178.564 px son
55 ms de los 59,6. Existe porque los paneles NO son opacos (`hudPiezas=esenciales` no carga el panel de fondo):
sus agujeros dejan ver el mundo y el mundo, al correrse, deja fantasma. DC-002.141 lo resolvió repintando la
caja entera; el número dice que esa decisión, correcta para el píxel, es la que se come el cuadro.

## El presupuesto de VRAM del anillo: NO ENTRA (este es el primer paso de la tarjeta, y da que no)

Medido con `pvr.VramDisponible()` en el arranque del POC (`JUEGO250`):

- `VRAM despues-del-video disponible= 4.740.808` (de los 8 MB; `pvr_init` ya se lleva 3,6).
- `pant_tex` de la pantalla 1024x512x2 = 1.048.576.
- lienzo del cuadro (RGBA) 640x480x4 = 1.228.800.
- **libre después de esos dos = 2.463.432.**

El anillo de scroll no es una textura: el POC pinta en RGBA, así que el anillo es un LIENZO (x4, no x2):

| pieza | bytes |
|---|---|
| anillo 1024x768x4 (margen 384x288, el mínimo para saltos de 21-145 px) | 3.145.728 |
| menos el lienzo actual que reemplaza | −1.228.800 |
| **delta** | **+1.916.928 ≤ 2.463.432 → el anillo solo SÍ entra** |

Pero el anillo **solo funciona con los paneles fuera del scroll** (si el panel vive dentro del anillo, se
corre con él y hay que repintarlo igual; es la misma cuenta de 55 ms). Y los paneles fuera del scroll cuestan:

| pieza extra | bytes |
|---|---|
| lienzo de la interfaz 640x480x4 | 1.228.800 |
| textura de la interfaz 1024x512x2 | 1.048.576 |
| **total con el anillo** | **6.472.904 > 4.740.808 → NO ENTRA** (sobran 1,7 MB de déficit) |

Achicando: anillo 1024x576x4 (margen vertical 96 px, menor que los 107 px medidos) + interfaz en tres bandas
(640x64, 640x96, 290x242 → 737.280) + textura de interfaz 1024x256x2 (524.288) = 5.071.136 → tampoco.
**No hay combinación de anillo + interfaz separada que entre en esta VRAM.** La tarjeta pedía decidir con el
número: el número dice que el anillo, como está planeado, no es viable. Y sin interfaz separada, el anillo se
queda sin el 100 % del beneficio que promete.

## El camino que sí puede llegar a 20 fps (lo que queda para las tarjetas siguientes)

Dos palancas, en este orden, y ninguna necesita un byte más de los 2,46 MB libres:

1. **Los paneles en su propio lienzo (interfaz separada).** Lienzo 640x480x4 (1.228.800) + textura ARGB1555
   1024x512x2 (1.048.576) = 2.277.376 ≤ 2.463.432 → **entra, con 186 KB de aire**. El lienzo del mundo queda
   sin paneles, así que el `memmove` mantiene el mundo limpio debajo de ellos y **desaparecen los 178.564 px
   de repintado (≈55 ms)** sin tocar el arte ni la lógica. Precio: convertir los bloques sucios de la interfaz
   (≈86 bloques = 7 ms) y componer un segundo quad con `punch-through` (el arte de los paneles es alfa
   binario: índice 0 = transparente, no hay alfa parcial en el HUD ni en el minimapa).
   Cuenta: 107 − 55 + 7 + 2 ≈ **61 ms → 16 fps caminando, ventanas ≈25 fps.**
2. **La textura que se corre por bloques de 32 y el residuo por UV.** Con el mundo solo (sin paneles) todo el
   lienzo se corre junto: la textura se puede correr en el MISMO paso, y como el twiddle es por bloques de
   32x32, correrla un múltiplo de 32 es mover bloques de 2 KiB (300 copias, 614 KB, ≈5 ms), no convertir. El
   residuo (0-31 px en cada eje) se absorbe con las UV del quad: `pant_tex` ya tiene 384x32 px de margen
   (1024x512 para una vista de 640x480), que es exactamente lo que hace falta. Solo se convierte la franja que
   entra (≈30 bloques al cruzar el borde, la mayoría de los cuadros 0).
   Cuenta: 61 − 23,7 + 5 + 1 ≈ **43 ms → 23 fps caminando, ventanas ≈30 fps.**

## Lo que esta tarjeta deja hecho

- `JUEGO142: PRESENTAR-CAMINANDO`, la línea POR CUADRO (los 30 promedios de `PVR135: PRESENTAR` diluían el
  cuadro caro entre los baratos: 141 midió 2-20 ms de promedio con 16-26 reales).
- `d2c_pvr_pantalla_prueba_conv142` + `PRUEBA_CONV_142=si`: la prueba de la conversión, para medir cualquier
  cambio del bucle sin adivinar (curva 1/5/20/40 bloques).
- `PVR135: PRESENTAR ... grupo142=si|no` y `PRES_GRUPO_142=si|no`: el A/B del agrupado en el mismo binario.
- **La subida agrupada** (`presentar_142.go`): 300 copias → 90, 2,4 ms → 1,8 ms, sin cambiar un píxel (los
  cuatro bloques de un grupo son contiguos en la textura twiddleada).
- crc32 del cuadro 1 VERIFICADO con el agrupado puesto (`MEDIR_CUADRO=si`): `SOLO-MAPA -1654008076`
  y `CON-JUGADOR -1025148544`, los mismos de DC-002.132.

## Lo que esta tarjeta NO cumplió, sin maquillaje

- **El criterio de fps no se cumple:** las ventanas caminando dan 14-20 fps en esta corrida (el `si` del
  agrupado las deja en 16/17/20/18/14), no 20 en todas. No es cuestión de la presentación: con los 55 ms del
  repintado de paneles en el camino, ningún ajuste de la subida lo alcanza.
- Las líneas 2, 3 y 4 del PLAN de la tarjeta (el mundo pintado en el anillo, las franjas solas, los fps) quedan
  sin hacer porque el presupuesto de VRAM mata el plan del anillo antes de empezar. La línea 1 (el presupuesto)
  está hecha, y su respuesta es "no entra": por eso el trabajo se corta acá y sigue en las tarjetas hijas con
  las dos palancas de más arriba.

## Cómo se corre

```
cd /home/dgalli/Proyectos/diablo2consoles
# la corrida de esta tarjeta (agrupado puesto, informe por cuadro)
bash scripts/console/build-fps-139.sh /tmp/dcbuild/fps142-grupo
SEGUNDOS=80 bash scripts/console/run-demo-140.sh /tmp/dcbuild/fps142-grupo/disc.gdi /tmp/capturas-fps142-grupo
grep -a "JUEGO142: PRESENTAR-CAMINANDO" /tmp/capturas-fps142-grupo/flycast.log | tr -d '\r'

# el A/B de la subida, en el MISMO binario
PRES_GRUPO_142=no bash scripts/console/build-fps-139.sh /tmp/dcbuild/fps142-nogrupo
# la prueba de la conversion (cuesta ~3 ms por cuadro, va apagada por defecto)
PRUEBA_CONV_142=si bash scripts/console/build-fps-139.sh /tmp/dcbuild/fps142-prueba
# el crc32 del cuadro 1
MEDIR_CUADRO=si bash scripts/console/build-fps-139.sh /tmp/dcbuild/fps142-crc
```

## Código

- `console/dc/std/csrc/pvr_pantalla.c` — `d2c_pvr_pantalla_prueba_conv142` (la prueba) y el camino de grupos:
  los cuatro bloques de un 2x2 convertidos a `conv_grupo` y subidos con UNA `pvr_txr_load` de 8 KiB.
- `d2core/d2render/pvr/presentar_142.go` — `presentarLosBloquesSucios142`, `presentarEnGrupos142`,
  `mapa142 [1024]byte` en `.bss`, los contadores por cuadro y `LoQueCostoElCuadro142()`.
- `d2core/d2render/pvr/pantalla.go` — el desvío al camino agrupado y `PruebaDeLaConversion142`.
- `console/dc/poc/juego-131/informe_142.go` — la línea por cuadro `JUEGO142: PRESENTAR-CAMINANDO` y
  `JUEGO142: PRUEBA-CONV`.
- `console/dc/poc/juego-131/mapa_141.go` — marca el cuadro que caminó y corre la prueba antes/después del
  corrimiento.
- Palancas (`knobs.go`): `presentarGrupo142`, `presentar142`, `pruebaConv142`.

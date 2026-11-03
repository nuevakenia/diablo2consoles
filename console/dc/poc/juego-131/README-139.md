# DC-002.139 -- LOS FPS DEL MVP: el mapa deja de repintarse y la presentacion por bloques

Pedido del usuario (2026-11-03): que el MVP jugable se vea FLUIDO. El liston: 18-20 fps minimo, ojala 30.

## El numero de partida (medido, DC-002.140)

Corrida de la demo en Flycast (`/tmp/capturas-demo140d`), 248 tiles, 640x480:

| etapa | us por cuadro |
|---|---|
| mapa (`rendererDelMapa.Render`) | 68.305 |
| presentacion (`Presente`): conversion RGBA->RGB565 | 23.588 |
| presentacion: subida a la VRAM | 1.788 |
| lineas por el puerto serie (`sinEtapa`) | 13.176 |
| minimapa | 7.245 |
| panel de dialogo | 3.374 |
| HUD | 2.014 |
| NPCs | 1.698 |
| actualizar | 361 |
| jugadores / bichos / objetos | 244 / 242 / 2 |
| **cuadro** | **~124.000 = 7,8 fps** |

De los 1.359.235 pixeles con indice distinto de cero que el mapa LEE por cuadro (4,4 pantallas por el solape
isometrico), 307.200 se escriben. Todo eso va contra la VRAM (area P2, sin cache): ~50 ns por pixel.

## Lo que se hizo (tres palancas, cada una con su A/B)

1. **`mapaEstatico` -- el mapa no se repinta si la camara no se movio (`mapa_139.go`).**
   El mapa es ESTATICO: si la traslacion de la vista esta en el mismo pixel que el cuadro anterior, volver a
   pintar los 248 tiles da exactamente los mismos pixeles. Lo unico que hay que hacer es devolver el mapa
   debajo de los sprites, y el motor YA guarda lo que hay debajo de la caja de cada ente (`CajaGuardada`).
   Si la camara se movio (o cambio el nivel, o se abrio/cerro el panel, o el nivel tiene tiles animados) se
   repinta entero, que es el camino de DC-002.140 sin una linea de diferencia.

2. **`cajas139` -- el guardar/devolver la caja de cada ente con el instrumento apagado.**
   Es lo que hace posible (1). Se paga: ~2,7 ms los jugadores, ~4-6 ms los bichos, 1,7 los NPCs.

3. **`convBloques139` -- la presentacion convierte SOLO los bloques de 32x32 que el cuadro toco
   (`d2core/d2render/pvr/sucios_139.go` + `d2c_pvr_pantalla_bloque`).**
   La conversion del cuadro entero son 23,6 ms y son LECTURAS del lienzo contra la VRAM. Con el mapa quieto
   solo cambian las cajas de los sprites (y los paneles, que se marcan cuando el jugador cambia de celda).
   Los bytes que llegan a la VRAM son los mismos: mismo `bloque_rapido`, mismo twiddle, mismo `pvr_txr_load`.

## Los numeros (A/B EN EL MISMO BINARIO, Flycast, la misma demo)

`MAPA_ESTATICO=no CAJAS_139=no CONV_BLOQUES=no` (el camino de DC-002.140):

    JUEGO139: MAPA-ESTATICO cuadros= 510 saltados= 0 repintados= 510 palanca= no cajas= no convBloques= no
    JUEGO132: MAPA cuadro= 510 ms= 61
    PVR135: PRESENTAR conv_us= 23618 vram_us= 1798 total_us= 25503 bloques= 300
    PVR132: CUADRO 510 trabajo_us= 78157 presentar_us= 25503 cuadro_us= 103663 fpsX100= 964
    DEMO140: CIERRE cuadros= 300 segundos= 39 fpsMedio= 8 fpsMinimo= 3 fpsMaximo= 10

`MAPA_ESTATICO=si CAJAS_139=si CONV_BLOQUES=si` (esta tarjeta):

    JUEGO139: MAPA-ESTATICO cuadros= 2490 saltados= 2439 repintados= 51 palanca= si cajas= si convBloques= si
    JUEGO134: ETAPAS cuadros= 2490 actualizar= 404 mapa= 3442 jugadores= 2681 bichos= 6837 npcs= 1686
              hud= 2018 minimapa= 7171 sinEtapa= 4403 dibujar= 28243 cuadroTotal= 28648
    PVR135: PRESENTAR total_us= 2560 bloques= 194
    PVR132: CUADRO 2520 trabajo_us= 26370 presentar_us= 2303 cuadro_us= 28782 fpsX100= 3474
    DEMO140: CIERRE cuadros= 2400 segundos= 80 fpsMedio= 30 fpsMinimo= 4 fpsMaximo= 43

**8 -> 30 fps de media (hasta 43), con el mismo cuadro.** El desglose: el mapa 68,3 -> 2,6-3,4 ms (-96 %),
la presentacion 25,5 -> 2,3-2,6 ms (-91 %).

## La prueba de que el dibujo NO cambio

Los crc32 del cuadro, comparados contra la corrida de DC-002.140 (`/tmp/capturas-demo140d`) — el MISMO
binario de juego, la misma demo, el mismo cuadro 1:

| linea | DC-002.140 | DC-002.139 |
|---|---|---|
| CUADRO 1 SOLO-MAPA crc32 | -1654008076 | -1654008076 |
| CUADRO 1 CON-JUGADOR crc32 | -1025148544 (249474 px) | -1025148544 (249474 px) |
| JUGADOR compuesto crc32 | 1857478036 | 1857478036 |
| BICHO pixeles | 1314 | 1314 |

(`igualAlHost= false` NO es de esta tarjeta: la demo de DC-002.140 ya lo trae contra el host de 1437022778,
que es el del POC anterior sin el pueblo: el crc32 de la demo es -1654008076 en las dos corridas.)

## Que queda (el proximo candidato, medido y no adivinado)

Con el cuadro en 28-32 ms, lo que manda ahora es:

    minimapa 7.171  |  bichos 4.204-6.837  |  sinEtapa 4.403  |  mapa 2.639  |  jugadores 2.675  |  HUD 2.018

- **el minimapa (7,2 ms)**: se redibuja entero cada cuadro aunque la camara no se mueva; tiene su palanca de
  "saltar" (`cuando`) que hoy esta en `siempre`;
- **los bichos (4-7 ms)**: es el baile de la caja (guardar + devolver + segundo pintado) de dos bichos;
  solo hace falta cuando el bicho se movio o cambio de cuadro de animacion;
- **`sinEtapa` (4,4 ms)**: las lineas del censo cada 30 cuadros por el puerto serie.

## Palancas

    MAPA_ESTATICO=si|no   (build-fps-139.sh)  el mapa no se repinta si la camara no se movio
    CAJAS_139=si|no                           guardar/devolver la caja de cada ente
    CONV_BLOQUES=si|no                        la presentacion solo convierte los bloques sucios

Con las tres en `no` el binario es el de DC-002.140, bit a bit (eso es el A/B de arriba).

## Como se corre

    bash scripts/console/build-fps-139.sh /tmp/dcbuild/fps139        # ~40 s
    SEGUNDOS=80 bash scripts/console/run-demo-140.sh /tmp/dcbuild/fps139/disc.gdi /tmp/capturas-fps139

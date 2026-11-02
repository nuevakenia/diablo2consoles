# DC-002.113b — Desalojo de la caché de tiles: la cache devuelve lo que no usa

**La medida, en una línea:** en la Dreamcast, con la cámara siguiendo y un paseo de **8 direcciones**, la corrida
del criterio camina **61,125 celdas en 32,6 s (1,875 celdas/s, el teórico exacto)** sirviendo **269 imágenes** de la
caché de índices (**168 desalojadas, 101 vivas al final**) y la cache **no derrama ni un byte**: `derrames= 0
bytesDerramados= 0` en los 49 cuadros. El mismo código, el mismo guion y la misma imagen, con la palanca del
desalojo **apagada** (la política de DC-002.113, cache de **solo agregar**), se cae en el **cuadro 11** con
**21 derrames / 486.400 B**, el conjunto vivo al **100 % del semiespacio** y `fatal error: out of memory`.

**El cuadro no cambió un píxel:** `CUADRO 1 SOLO-MAPA ... crc32= 1437022778 igualAlHost= true` y
`CUADRO 1 CON-JUGADOR ... crc32= 1363871674` — los mismos de DC-002.112 y DC-002.113.

---

## 1. Qué se agregó (todo aditivo, con palanca)

| pieza | dónde | qué hace |
|---|---|---|
| **tramo con huecos** | `d2maprenderer/arena_indices.go` | una imagen de la cache ocupa un pedazo **contiguo** de un bloque del tramo (`contenedor`, `desde`); soltarla **devuelve ese pedazo** y los pedazos vecinos se **cosen** en un hueco más grande |
| **desalojo por antigüedad de cuadro** | `d2maprenderer/cache_indices.go` | una imagen es candidata si no se usó en los últimos `N` cuadros (`desalojoEnCuadros= 2`); `getCacheDeIndices` marca `ultimoCuadro` |
| **las tres pasadas** | `desalojoDeLaCache` | 1) las frías, de la más vieja a la más nueva; 2) **agrandar el hueco más grande** soltando a su vecino; 3) última vuelta: cualquiera que no se haya pedido en este cuadro |
| **composición al vuelo** | `renderer.go` + `cache_indices.go` | si el tramo no tiene lugar, la imagen se compone en **un buffer de trabajo**, se pinta y **no se cachea** |
| **censo del desalojo** | `CensoDelTramo` | `desalojos`, `bytesDesalojados`, `huecos`, `alVuelo`, `bytesAlVuelo`, `agrandados` |

Con las dos palancas apagadas (`DESALOJO=0`, `AL_VUELO=0`) el camino es **exactamente el de DC-002.113**: lo prueba
`TestSinPalancaLaCacheSoloAgrega` (mismo número de imágenes, cero desalojos, el derrame contado cuando el tramo se
llena).

**Por qué pedazos y no bloques enteros.** Un bloque del tramo mide **348.160 B**; las imágenes de la cache van de
**20.480 a 76.800 B**. Tirar un bloque entero para hacer lugar a una imagen se llevaría por delante **hasta diez
imágenes vivas**. Por eso se devuelve el pedazo exacto y los huecos vecinos se cosen (si no, el tramo se llena de
agujeros de 8 B que no sirven para la imagen más chica).

**Por qué la tercera pasada es segura.** La pintura de los índices **copia** los píxeles al lienzo en el acto:
`pvr.Surface.RenderIndicesTintado` escribe el RGBA ahí mismo y el `Cuadro` de las medidas hace lo mismo. No hay nada
en vuelo entre el pedido de una imagen y su pintado, así que soltar una imagen del cuadro en curso a lo sumo cuesta
**una decodificación más**, nunca un píxel equivocado. Sobre esa base, la **pasada 2** puede agrandar un hueco con un
vecino que el cuadro ya pidió, y es la que se lleva puesto el derrame único de 76.800 B que quedaba (ver §5).

## 2. La corrida del criterio (consola, Flycast)

`desalojo/`: `desalojoEnCuadros= 2`, `cuadros= 46`, `tramoDeLaCache= 4.526.080` (13 bloques de techo; la VRAM sirve
**10**, 3.481.600 B), guion de la cruceta **8 direcciones × 4 s** con `PAUSA= 0.2`.

```
JUEGO113: RESUMEN cuadrosPintados= 49 tramos= 8 tramosReportados= 8 segundosCaminados= 32.6 subtiles= 305.625
          celdas= 61.125 celdasPorSegundo= 1.875 teoricoCeldasPorSegundo= 1.875 ticks= 815 recortes= 6
          pasoMayor= 1 celdasFinales= 28.1186 21.3104 direcciones= 8 camara= siguiendo reloj= 45.628
```

| tramo | dir COF | cuadros | segundos | celdas | celdas/s | celda inicial | celda final |
|---|---|---|---|---|---|---|---|
| DERECHA | 12 | 4 | 3,706 | 6,900 | 1,862 | 28 · 20 | 34,59 · 15,69 |
| ABAJO+DERECHA | 15 | 4 | 4,356 | 7,425 | 1,705 | 32,88 · 15,12 | 41,09 · 18,64 |
| ABAJO | 0 | 5 | 4,659 | 8,700 | 1,867 | 39,92 · 17,47 | 46,60 · 25,19 |
| ABAJO+IZQUIERDA | 1 | 5 | 4,122 | 7,725 | 1,874 | 46,07 · 23,62 | 48,52 · 30,95 |
| IZQUIERDA | 4 | 5 | 4,054 | 7,575 | 1,869 | 48,52 · 30,95 | 41,81 · 35,86 |
| ARRIBA+IZQUIERDA | 7 | 5 | 3,970 | 7,425 | 1,870 | 43,16 · 36,31 | 35,11 · 32,95 |
| ARRIBA | 8 | 5 | 4,265 | 8,025 | 1,882 | 36,12 · 33,96 | 29,90 · 26,65 |
| ARRIBA+DERECHA | 9 | 4 | 4,484 | 7,350 | 1,639 | 30,44 · 28,28 | 28,12 · 21,31 |

El **total da exacto** (1,875 celdas/s contra el teórico de `d2path`); la diferencia de cada tramo es la
cuantización del reloj de 25 Hz, y en el total se compensa.

**El censo del tramo, contado desde el primer cuadro hasta el 49** (`TRAMO despues-del-cuadro`):

```
cuadro  usado      libre     imagenes*  huecos  desalojos  bytesDesalojados  agrandados  derrames  bytesDerramados
 1      2.723.840    61.440      86       0        0              0            0         0            0
 5      2.723.840    61.440      86       0        0              0            0         0            0
15      3.015.680   465.920     184      20       70        2.365.440          1         0            0
25      3.077.120   404.480     195      21       80        2.611.200          1         0            0
35      2.611.200   870.400     199      34      110        3.348.480          1         0            0
49      3.164.160   317.440     269      18      168        4.879.360          1         0            0
```

\* `imagenes` es **acumulado**: las imágenes que el tramo sirvió desde el cuadro 1, sin descontar las desalojadas.
Las **vivas** son `imagenes − desalojos`: al cuadro 49, **269 − 168 = 101**.

**El tramo se mantiene solo entre 2,6 y 3,4 MB de los 3.481.600 B que da la VRAM** (91 % al final), con **18 huecos**
y **un solo agrandamiento** en toda la corrida (cuadro 15): la cache vive, se llena, devuelve y vuelve a llenarse
**sin derramar**. Y la repetibilidad se sostiene:

```
JUEGO113: REPETIBILIDAD cuadro= 48 camara= siguiendo crcMapa= -10498321 crcMapaDelCuadroAnterior= -10498321
          igualAlCuadroAnterior= true crcConJugador= 129936599 igualConJugador= true ancla= 320 283 celda= 28 20
          llenos= 307200 llenosDelCuadroAnterior= 307200 crcMapaDelCuadro1= 1437022778 igualAlCuadro1= false
          llenosDelCuadro1= 307196 cachePerezosa= true
```

## 3. El A/B de las tres políticas (mismo código, mismo guion, una palanca)

| | **solo agregar** (DC-002.113) | **desalojo** (esta tarjeta) | **al vuelo** (alternativa) |
|---|---|---|---|
| palanca | `DESALOJO=0 AL_VUELO=0` | `DESALOJO=2` | `DESALOJO=0 AL_VUELO=1` |
| cuadros pintados | **11** (se cae) | **49** (cierra) | **49** (cierra) |
| celdas caminadas | 8,775 (1 tramo) | **61,125** | 38,475 |
| segundos caminados | 4,8 | **32,6** | 20,5 |
| celdas/s (reloj del POC) | 1,837 | **1,875** | 1,875 |
| **celdas/s de pared** | — | **1,34** | 0,59 |
| imágenes servidas (acumulado) | 114 | **269** | 114 cacheadas + **894 al vuelo** |
| derrames | **21 / 486.400 B** | **0 / 0 B** | **0 / 0 B** |
| imágenes que se vuelven a componer | no aplica | **hasta 168** (una por desalojo, 3,4/cuadro) | **894** (18,2/cuadro, 21,2 MB) |
| ms del mapa por cuadro (mediana / media) | 595 / 623 (10 cuadros) | **531 / 555** | 836 / 961 |
| final | `out of memory` (cuadro 11) | cierra con `RESUMEN` | cierra con `RESUMEN` |

Los tres con la **misma** imagen de arranque, el mismo guion y el mismo reloj: el desalojo y el al vuelo cambian una
palanca de `knobs.go`, nada más.

**Lo que dice el A/B.** Las dos políticas evitan el derrame, pero **no cuestan lo mismo**: el desalojo **desaloja 168 imágenes en 49
cuadros** (3,4 por cuadro) y solo las que se vuelven a pedir se recomponen —168 es el **techo**, no el gasto— y el
al vuelo **recompone 894** (18,2 por cuadro, **21,2 MB** de índices decodificados de nuevo). Con el mismo número de cuadros, el al vuelo tarda **1,6× más por cuadro** (836 ms contra
531 ms de mediana) y por eso camina **menos** en la misma cantidad de cuadros: **0,59 celdas/s de pared contra
1,34**. Se queda como **respaldo** (la palanca existe, está probada y medida), no como default: la cache con
desalojo conserva el trabajo hecho y el al vuelo lo tira cada cuadro.

## 4. El tramo no se puede agrandar más (medido)

El techo se subió de 4.177.920 a **4.526.080 B** —justo el número que la sonda de la VRAM dijo al arrancar
(`vramServiriaDeUnGolpe= 4526080`, con `vramDisponible= 4740808`)— y aun así la cache **nunca pasó de 10 bloques**:
`pedazos= 10`, `bytes= 3.481.600` en todos los cuadros. Con esos 3,48 MB tomados quedaban **1.259.208 B** de VRAM
disponible, pero la sonda no pudo servir **otro bloque de 348.160** (el montón del PVR quedó fragmentado por los 10
bloques y la textura del cuadro, `vram de la textura del cuadro= 1048576`). O sea: **el techo del tramo no es un
número del código, es lo que la VRAM sirve en ese momento** — y por eso la cache tiene que **devolver** (desalojo) en
vez de crecer.

## 5. El derrame único que se fue: la pasada que agranda el hueco

Con las dos primeras pasadas (frías + agrandar el hueco, pero sin aceptar vecinos del cuadro en curso) la corrida
del criterio daba **1 derrame de 76.800 B** (la imagen más grande de la cache, 160×480): al cuadro 14 el tramo tenía
**573.440 B libres** repartidos en **20 huecos** y colas de bloque, y **ninguno de 76.800**: el desalojo de frías no
podía juntarlos y los vecinos del hueco estaban todos en el cuadro en curso. La tercera pasada (soltar al **vecino** del hueco más grande para coserlo)
lo resolvió: `agrandados= 1` en la corrida y `derrames= 0`. El caso está clavado en la prueba
`TestElDesalojoAgrandaElHueco` (que además exige `Agrandados > 0`, si no la prueba no está probando nada).

## 6. Hallazgo aparte: el muro de los ~65 cuadros NO son los derrames de la cache

En la corrida **larga** (guion de 5 s por dirección, `larga/`) el POC camina **74,625 celdas en 41,93 s** con
**1 derrame / 61.440 B** (el derrame único de antes del arreglo de §5) y después se cae en el **cuadro 65** con
`fatal error: GC to-space overflow` — con el tramo en **3.000.320 B de 3.481.600 (86 %)**, **24 huecos**, el desalojo
trabajando (167 desalojos, 4.751.360 B devueltos) y **ningún derrame en los últimos 50 cuadros**: la cache no era la
que llenaba el montón.

Lo que sí mide esa corrida es el costo por cuadro: **entre 0,56 y 1,11 MB de basura por cuadro** (una o dos
recolecciones por cuadro, ~558 KB cada una, medido sobre las 179 recolecciones), con el semiespacio en
**1.835.008 B** y el conjunto vivo entre **1.185.280 B en reposo (65 %)** y **1.521.560 B de pico (83 %)** mientras
la cache está llena. Con el vivo al 83 % al recolector le quedan **~313 KB** para copiar el objeto que sigue, y en
algún cuadro no alcanza. Eso es del recolector y del cuadro del pueblo (**no** del desalojo) y es de otra tarjeta:
bajar la basura por cuadro, no la cache.

Por eso la corrida del criterio va con `cuadros= 46`: **cierra antes del muro** y el `RESUMEN` y la `REPETIBILIDAD`
se leen enteros. El paseo del criterio (**61,125 celdas, 32,6 s, 8 direcciones**) se cumple de sobra.

## 7. Las pruebas de host (8, todas en verde)

`d2core/d2map/d2maprenderer/desalojo_113b_test.go`:

| prueba | qué caza |
|---|---|
| `TestElDesalojoNoDerramaNiUna` | con el tramo lleno y 60 pedidos nuevos, **cero derrames** y los bytes se reusan |
| `TestElDesalojoNoMezclaDosImagenes` | cada imagen viva conserva **sus** bytes (patrón por estilo): el desalojo no pisa a nadie |
| `TestElDesalojoRespetaElCuadroEnCurso` | lo que el cuadro está pintando no lo eligen las pasadas 1 y 3: la víctima es la más vieja de las frías, y la imagen viva conserva sus píxeles |
| `TestElDesalojoAgrandaElHueco` | la pasada 2 (**agrandar el hueco con su vecino**) y que `Agrandados > 0` |
| `TestElDesalojoTerceraPasadaSinHuecos` | sin frías y **sin huecos**, la última vuelta suelta a la más vieja |
| `TestElDesalojoAceptaUnVecinoDelCuadroEnCurso` | el vecino del cuadro en curso se suelta **solo** por la pasada 2 |
| `TestLaComposicionAlVueloNoDerramaNiCachea` | al vuelo: pinta, **no** derrama y **no** entra en el mapa de la cache |
| `TestSinPalancaLaCacheSoloAgrega` | con las palancas apagadas, el camino de DC-002.113 queda **idéntico** |

Y lo de siempre: `go test ./d2core/d2map/...`, `go build ./d2core/... ./d2game/...`, `go vet` y el build del host,
todo en verde.

## 8. Cómo se reproduce

```bash
# la corrida del criterio (build completo: cambia el motor, no alcanza SALTAR_MOTOR=1)
CUADROS=46 TRAMO_CACHE=4526080 bash scripts/console/build-juego-113.sh /tmp/dcbuild/113b-desalojo-c
GUION="derecha 4 abajo+derecha 4 abajo 4 abajo+izquierda 4 izquierda 4 arriba+izquierda 4 arriba 4 arriba+derecha 4" \
  PAUSA=0.2 CAPTURAS=11 INTERVALO=5 \
  bash scripts/console/run-flycast-113.sh /tmp/dcbuild/113b-desalojo-c/disc.gdi /tmp/corrida-113b-desalojo-c3

# el mutante "solo agregar" (DC-002.113) y la alternativa al vuelo: misma imagen, una palanca
CUADROS=46 TRAMO_CACHE=4526080 DESALOJO=0 bash scripts/console/build-juego-113.sh /tmp/dcbuild/113b-antes
CUADROS=46 TRAMO_CACHE=4526080 DESALOJO=0 AL_VUELO=1 bash scripts/console/build-juego-113.sh /tmp/dcbuild/113b-alvuelo-c

# las líneas del criterio
grep -a 'JUEGO113: TRAMO\|JUEGO113: RESUMEN\|JUEGO113: REPETIBILIDAD' /tmp/corrida-113b-desalojo-c3/flycast.log
```

Tiempos medidos en este host (sin KVM): ~60 s hasta el primer cuadro (10 s el `D2DATA.MPQ`, 24 s componer el
pueblo), **~530–750 ms por cuadro** de PVR, 46 cuadros del guion + 3 del cierre ≈ 46 s.

## 9. Archivos de esta evidencia

| archivo | qué es |
|---|---|
| `desalojo/flycast.log` | la corrida del criterio completa (49 cuadros, los 8 tramos, el censo del tramo cuadro por cuadro, `RESUMEN`, `REPETIBILIDAD`) |
| `desalojo/salida.txt` | la salida de `run-flycast-113.sh`: las capturas con su estado y las líneas del criterio juntas |
| `desalojo/c*.png` | el cuadro recortado a 640×480 (la ventana del emulador): `c00-quieto`, `c02`, `c05`, `c09`, `c99-final` |
| `solo-agregar/flycast.log` | el mutante de DC-002.113 con el mismo guion: los **21 derrames** y la caída en el cuadro 11 |
| `solo-agregar/salida.txt` | su salida, con el `fatal error: out of memory` |
| `solo-agregar/c3.png` | el cuadro del mutante antes de caerse |
| `al-vuelo/flycast.log` | la alternativa al vuelo: **894 imágenes** compuestas sin cachear, 49 cuadros |
| `al-vuelo/salida.txt` | su salida |
| `al-vuelo/c09.png` | su cuadro (mismo encuadre, mismo pueblo) |
| `larga/flycast.log` | la corrida larga de 5 s por dirección: **74,625 celdas en 41,93 s** y el muro del cuadro 65 |
| `larga/salida.txt` | su salida |

Las capturas salen de la ventana del emulador (`crop 640x480+1400+120`), que es 1:1 con la pantalla lógica del POC.

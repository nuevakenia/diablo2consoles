# DC-002.112 — El primer cuadro de JUEGO: el pueblo CON el jugador encima, en el PVR

**La medida, en una línea:** en la Dreamcast, el mismo cuadro de 640×480 lleva el **pueblo del motor** y la
**amazona del `.dcc`** dibujada encima en su celda; el mapa **solo** sigue dando `pixelesNoVacios= 307.196 de
307.200` y `crc32= 1437022778` (el del host), **con el jugador** da `crc32= 1363871674` con **896 píxeles
cambiados dentro de su caja de 26×79**, y al **devolver la caja** el cuadro vuelve a `1437022778`
(`igualAlMapaSolo= true`). Los cinco números son **los mismos del medidor del host**, al dígito.

Es la primera vez en el proyecto que las dos mitades están en la misma imagen: el pueblo venía de
DC-002.103/.106/.108/.111 y el jugador de DC-002.38/.98/.100/.69, pero **nunca se habían juntado**.

---

## 1. Qué se dibuja y en qué orden

| paso | qué | de dónde sale |
|---|---|---|
| 1 | arranca el PVR a 640×480 | `pvr.PantallaIniciar` (DC-002.50: el video **primero**, con el montón de C casi vacío) |
| 2 | se compone el **jugador**: COF `amnuhth.cof` (amazona, NU, dirección 0) + sus 7 `.dcc` → 8 cuadros compuestos | `d2char.mpq` por la Source de consola; arena del decodificador en el montón de C (DC-002.100) |
| 3 | se compone el **pueblo** (`RegionAct1Town`, preset 1, archivo 0) y se sueltan fuentes y tablas | `d2mapengine` (DC-002.88/.105) |
| 4 | se arma la **cache de tiles en índices** y se pinta el mapa por el camino del motor | `d2maprenderer` + bloques de VRAM del PVR (DC-002.106/.108) |
| 5 | **encima**, el cuadro 0 del jugador con `RenderIndicesTintado`, en el ancla del motor | `jugador.go` (archivo **compartido** con el host) |

El jugador se compone **antes** de que el pueblo se componga y **antes** de soltar las fuentes MPQ, por tres
razones medidas: los `.cof`/`.dcc` y la paleta de las unidades salen de los MPQ (después de
`soltarLoQueElCuadroNoLee` no se puede leer un byte); la arena del `.dcc` se devuelve en el acto, así que su pico
**no se suma** al pico de componer el pueblo; y lo que queda vivo del jugador son 8 cuadros de 26×79 índices.

## 2. La corrida del criterio (consola)

`JUGADOR=encendido` (el default de `knobs.go`), las palancas del pueblo las de DC-002.111.
Log completo en `consola-criterio-jugador.log`; las líneas del criterio, en `consola-medida-criterio.txt`.

```
JUEGO112: JUGADOR PALETA data/global/palette/Units/pal.dat bytes= 768 ok= true primero= 0 0 36
JUEGO112: JUGADOR COF data\global\CHARS\AM\COF\amnuhth.cof bytes= 1132 capas= 8 framesPorDir= 8 dirs= 16 speed= 128
JUEGO112: JUGADOR ARCHIVO data/global/chars/AM/HD/AMHDLITNU1ht.dcc bytes= 10931
  ... LA 12765 · LG 37659 · RA 10986 · S1 3781 · S2 4902 · TR 17401 (7 capas: la SH no tiene gráfico en NU)
JUEGO112: JUGADOR ARENA pedidos= 133 entregado= 86208 tramos= 1 capacidad= 0 montonCLibre= 136824 \
          recoleccionesEnElCiclo= 7 pedidasPorMi= 7 automaticas= 0 sinArena= false
JUEGO112: JUGADOR CUADRO 0 de 8 caja= 26 x 79 esquina= -18 -65 pixeles= 896 crc32= 1857478036 capasDibujadas= 7
JUEGO112: JUGADOR FUENTE SOLTADA /cd/JUEGO/D2CHAR.MPQ tablas= 322432 bytesTablas= 320320 bytes= 320320
```

El ciclo entero, cuadro por cuadro (los 8), con el `crc32` **sin signo**:

| cuadro | 0 | 1 | 2 | 3 | 4 | 5 | 6 | 7 |
|---|---|---|---|---|---|---|---|---|
| píxeles | 896 | 889 | 898 | 897 | 906 | 914 | 917 | 902 |
| `crc32` | 1857478036 | 2138394951 | 3790466078 | 1891057005 | 3705376215 | 1415443346 | 3716097957 | 127741840 |

El cuadro 0 es **el del corpus de DC-002.38/.100** (caja 26×79, 896 px, `crc32 1857478036`), y los 8 son los
mismos que da el host con el mismo `personaje.go` (ver §4).

Y el cuadro de la pantalla, tres veces seguidas (el POC dibuja 3 y deja de imprimir):

```
JUEGO112: CUADRO 1 SOLO-MAPA        pixelesNoVacios= 307196 de 307200 crc32= 1437022778 crc32RGB= -1787731553 ms= 746
JUEGO112: JUGADOR ancla= 320 283 caja= 302 218 328 297 tamano= 26 x 79 compuesto= 26 x 79 esquina= -18 -65 crc32= 1857478036
JUEGO112: CUADRO 1 CON-JUGADOR      pixelesNoVacios= 307196 de 307200 crc32= 1363871674 pixelesDelJugador= 896 delArea= 2054 msJugador= 0
JUEGO112: CUADRO 1 MAPA-RESTAURADO  crc32= 1437022778 igualAlMapaSolo= true mapaSolo= 1437022778 host= 1437022778
JUEGO112: CUADRO 1 PRESENTADO con el jugador crc32= 1363871674 igualAlDeLaMedida= true
JUEGO112: CUADRO 2 CON-JUGADOR ... crc32= 1363871674 pixelesDelJugador= 896
JUEGO112: CUADRO 3 CON-JUGADOR ... crc32= 1363871674 pixelesDelJugador= 896
```

Las tres medidas, y qué prueba cada una:

1. **`SOLO-MAPA` = 1437022778**: el pueblo se sigue dibujando igual que en DC-002.106/.108 pese a que ahora el
   binario lleva el jugador, el `d2char.mpq` en el disco y 320 KB más de código.
2. **`MAPA-RESTAURADO` = 1437022778** después de devolver la caja del jugador: el jugador **solo** toca su caja
   (302..328 × 218..297); ni un píxel de afuera cambia. `pixelesDelJugador= 896` de `delArea= 2054` es
   exactamente los píxeles no vacíos del compuesto: la amazona se pintó **sin una corrida de un píxel**.
3. **`PRESENTADO` = `CON-JUGADOR`**: lo que se midió es lo que se presenta (el repintado después de restaurar da
   el mismo cuadro).

El ancla sale de la fórmula del motor, no de un número elegido: `WorldToScreenF(28,20)` + el `RenderOffset` del
subtile del medio (16×8 px por subtile, −5 px del compuesto) → **(320, 283)**, el centro del rombo de la celda
en una pantalla de 640×480 centrada en 28,20. El host da el mismo par.

**El A/B en el mismo código** (`JUGADOR=apagado`, log `consola-referencia-jugador-apagado.log`): el binario
compilado sin el jugador da los tres cuadros con `SOLO-MAPA ... crc32= 1437022778`, `cacheSuperficies= 86
cacheBytes= 2723840` y **ninguna** línea `CON-JUGADOR` — o sea que las tres medidas de arriba no dependen de que
el jugador esté encendido y el camino del pueblo solo sigue siendo el de DC-002.106/.108.

## 3. El costo del jugador, medido aparte

| instrumento | antes del jugador | después del jugador | diferencia |
|---|---|---|---|
| montón de C **usado** (`uordblks`) | 4.509.488 B | 4.509.488 B | **0 B** |
| montón de C **libre** (`fordblks`) | 138.000 B | 138.000 B | **0 B** |
| semiespacio usado | 1.225.896 B (66 %) | 511.480 B (27 %) | **−714.416 B** |
| recolecciones (`gc_stats`) | 1 | 10 | +9, **todas pedidas por el POC** |

* La **arena del decodificador** pidió **133 tramos** y **86.208 B** en total, en **1** tramo de 256 KB, y lo
  devolvió entero (`runtime_FreeExternal`): el montón de C queda **igual que antes** de tocar al jugador. Ese es
  el número que importa para DC-002.113: cargar la amazona **no cuesta montón de C**.
* **`automaticas= 0`**: las 7 recolecciones del ciclo son los `runtime.GC()` que el POC pide a propósito después
  de cada capa (sin ellos la carga se vuelve 40× más lenta, medido en DC-002.38). El decodificador del `.dcc`
  **no disparó ni una** — el criterio de DC-002.98/.100 se mantiene, y ahora está separado en el log
  (`recoleccionesEnElCiclo / pedidasPorMi / automaticas`) para que no se pueda confundir.
* El semiespacio **baja** 714 KB porque soltar el `d2char.mpq` (322.432 B de tablas) y las recolecciones limpian
  lo que deja la carga; el jugador no lo llena.
* Lo que **queda vivo** del jugador: 8 cuadros de 26×79 índices (`8 × 2.054 B` ≈ 16 KB) + 1 KB de paleta.

## 4. La comparación con el host (misma escena, mismo archivo)

`personaje.go` y **`jugador.go`** son **byte por byte** los mismos archivos en los dos lados
(`sha256 a41eae3d…` y `5f9d3c3a…`, lo verifica el build y `host-pruebas.txt`). El build de la consola **falla**
si difieren: sin eso, comparar `crc32` no valdría nada.

| | consola (Flycast) | host (`render-294 -jugador`) |
|---|---|---|
| mapa solo: píxeles / `crc32` | 307.196 / 307.200 · **1437022778** | 307.196 / 307.200 · **1437022778** |
| ancla del jugador | (320, 283) | (320, 283) |
| caja | (302,218)-(328,297) 26×79 | (302,218)-(328,297) 26×79 |
| compuesto: píxeles / `crc32` | 896 / **1857478036** | 896 / **1857478036** |
| con el jugador: `crc32` / cambiados | **1363871674** / 896 de 2.054 | **1363871674** / 896 de 2.054 |
| mapa restaurado | **1437022778** (`igualAlMapaSolo= true`) | **1437022778** (idem) |

Y el PNG del host **sin** jugador (`host-cuadro-mapa-solo.png`) tiene `sha256 2b6c64db…`: es **el mismo archivo**
que DC-002.108 dejó como referencia del cuadro del pueblo. O sea que esta tarjeta no movió ni un píxel del mapa.

### La captura, y la deuda de DC-002.109/.111 que esta tarjeta paga

Las tarjetas anteriores dejaron anotado que `CAPTURE=spectacle` fotografiaba el escritorio con la ventana de
Flycast chica y que eso **no servía como prueba de los cuadros**. Acá se resolvió: el emulador se corre en
**pantalla completa** (`window:fullscreen=yes`), se captura el monitor donde cae y se recorta el área del juego.

* `consola-pantalla-completa.png` — la captura cruda (3584×1080, los dos monitores).
* `consola-cuadro-con-jugador.png` — el área del juego (monitor de 1024×768, escalada a 640×480).
* `consola-jugador-zoom.png` — el zoom del área del jugador en la consola: **la amazona de cuerpo entero, de pie
  sobre el pasto**. El mismo recorte del PNG del host, en `host-jugador-zoom.png`.
* **`PSNR 20,25 dB` / `SSIM 0,80`** entre el cuadro capturado del emulador y el PNG del host: es la misma escena
  (el resto de la diferencia es el PVR en 16 bpp y el reescalado de 640×480 a 1024×768).
* Sobre las ventanas de notificación del escritorio que se ven a la derecha del cuadro: **son del host**, no del
  POC; el juego está a la izquierda y completo.

**Lo que no se pudo hacer:** el modelo de visión de este host (`vision_analyze`) contestó dos veces y después
empezó a dar *timeout*; la descripción de la amazona en pantalla salió de la última respuesta buena
(«figura humana de cuerpo entero, traje azul oscuro, de pie sobre el pasto») y el resto es la medida del §4. Las
imágenes están en la carpeta para que las mire un humano.

## 5. Lo que esta tarjeta dejó hecho

* `console/dc/poc/juego-112/` — el POC nuevo (numeración de la casa):
  * `jugador.go` — **archivo compartido** con el medidor del host: compone el ciclo por el camino del motor
    (COF → ruta → `.dcc`/`.dc6` → decodificador → compositor), arma la paleta de las unidades en RGBA, calcula
    el **ancla** con el `WorldToScreenF` del renderer y pinta con `RenderIndicesTintado`.
  * `personaje.go` y `arena.go` — copias del POC del personaje (el compositor y la arena del decodificador).
  * `jugador_carga.go` — la carga en la consola: paleta del `d2data`, COF y `.dcc` del `d2char` por la Source con
    dueño explícito, `runtime_FreeExternal` como liberador, arena instalada durante la decodificación y liberada
    después de componer, y el contador que separa las recolecciones pedidas de las automáticas.
  * `main.go` — el pueblo (de DC-002.103/.111) + el jugador: las tres medidas del cuadro, la caja guardada y
    devuelta, y el costo en memoria antes/después. La palanca `jugadorKnob` deja la corrida de referencia del
    pueblo solo **en el mismo binario**.
  * `knobs.go` — las palancas del pueblo **y** las tres de esta tarjeta (`jugador`, `modoDelJugador`,
    `dirDelJugador`, `cuadroDelJugador`), cada una con su porqué y su número.
  * `csrc/shim.c` — el shim del pueblo más los tres puentes del jugador (`d2c_free_external`, `d2c_gc_n`,
    `d2c_heap_libre`), con el `__asm__("_runtime.FreeExternal")` que hace falta: el símbolo de gccgo está
    manglado y sin eso el enlace muere con `undefined reference to 'runtime_FreeExternal'` (medido acá).
* `console/dc/medidas/render-294/` — `personaje.go` y `jugador.go` (los compartidos), `jugador_host.go` (la
  carga con `mpq.ReadFile` y `medirElJugadorEnElCuadro`) y el flag **`-jugador`**: el host dibuja la misma
  escena y mide lo mismo.
* `scripts/console/build-juego-112.sh` — el build: verifica con `cmp` los dos archivos compartidos, compila los
  paquetes del motor que este POC usa (incluye la cadena del personaje: `d2cof`, `d2dc6`, `d2dcc`, `d2path`,
  `d2math/d2vector`), aplica la guardia de la tabla de lookups (DC-002.107) y arma el disco con **los tres MPQ**
  (`PATCH_D2`, `D2DATA` y **`D2CHAR`**).
* `scripts/console/revisar-arbol-gox.sh` — **arreglo de la guardia**: un paquete que el build **recompila** ya no
  aborta por no existir en el árbol base (`d2core/d2map/d2maprenderer.gox` no estaba en `/tmp/gox-296` y el POC
  lo crea en su primer paso; el recuadro abortaba por un objeto que el mismo build iba a hacer).

## 6. Lo que NO entra (y queda para la siguiente)

* **Mover al jugador** (maple, teclado, colisión): DC-002.113. Acá el ancla es fija y el cuadro que se dibuja es
  el 0 del ciclo (`cuadroDelJugador`); el reloj de 128,00 ms por cuadro se informa pero no se usa.
* **El orden de capas y la lógica de `d2maprenderer`**: no se tocó ni una línea. El jugador se pinta **encima**,
  después del `Render` del mapa.
* **La capa SH** (escudo) del COF no tiene gráfico en NU (`CAPA SH SIN-GRAFICO`): 7 capas dibujadas de 8, igual
  que en DC-002.38/.100. No es un fallo, es que la amazona va desarmada.
* El POC usa **un** nivel y **un** frame por corrida: la animación y el cambio de nivel con el jugador encima son
  tarjetas aparte.

## 7. Cómo se reproduce

```bash
# host: la escena de referencia (mapa solo, con el jugador y mapa restaurado) y los dos PNG
go run ./console/dc/medidas/render-294 -perezoso -indices -cuadros 1 -tramo 2723840 -pedazo 348160 \
    -jugador -out /tmp/cuadro-host-con-jugador.png
go run ./console/dc/medidas/render-294 -perezoso -indices -cuadros 1 -tramo 2723840 -pedazo 348160 \
    -out /tmp/cuadro-host-mapa-solo.png

# consola: el build (ELF + imagen con los tres MPQ) y la corrida
bash scripts/console/build-juego-112.sh /tmp/dcbuild/juego112
CAPTURE=none bash scripts/console/run-flycast.sh /tmp/dcbuild/juego112/disc.gdi 150 /tmp/cap-112
grep -a 'JUEGO112: CUADRO\|JUEGO112: JUGADOR' /tmp/cap-112/flycast.log

# consola: la corrida de referencia del pueblo solo, en el MISMO binario
JUGADOR=apagado bash scripts/console/build-juego-112.sh /tmp/dcbuild/sin-jugador

# consola: la foto del cuadro (el emulador en pantalla completa y la captura del monitor)
#   flycast -config window:fullscreen=yes ... disc.gdi   +   spectacle -b -n -f -o pantalla.png
```

## 8. Archivos de esta evidencia

| archivo | qué es |
|---|---|
| `consola-criterio-jugador.log` | la corrida del criterio en Flycast: el jugador cargado (paleta, COF, 7 `.dcc`, arena, ciclo) y los 3 cuadros con las tres medidas |
| `consola-medida-criterio.txt` | las líneas del criterio solas, para no abrir el log entero |
| `consola-referencia-jugador-apagado.log` | el A/B en el mismo código (`JUGADOR=apagado`): el pueblo solo, `crc32= 1437022778` en los tres cuadros |
| `consola-cuadro-con-jugador.png` | el cuadro que se presentó, 640×480 (recortado de la captura de pantalla) |
| `consola-jugador-zoom.png` | el zoom del área del jugador en la consola: la amazona de pie sobre el pasto |
| `consola-pantalla-completa.png` | la captura cruda del escritorio (3584×1080, los dos monitores) |
| `host-render-jugador.log` | el medidor del host con `-jugador`: los 8 cuadros del ciclo y las tres medidas |
| `host-render-mapa-solo.log` | el medidor del host sin jugador (la referencia del pueblo) |
| `host-cuadro-con-jugador.png` | el PNG del host CON el jugador (`sha256 568cd3d8…`) |
| `host-cuadro-mapa-solo.png` | el PNG del host SIN el jugador (`sha256 2b6c64db…`): **el mismo de DC-002.108** |
| `host-jugador-zoom.png` | el mismo recorte del área del jugador, en el PNG del host |
| `host-pruebas.txt` | `go vet`, `go test` de `d2map`/`d2asset` y el `cmp` de los dos archivos compartidos |

# PLAN — port de Diablo II a Dreamcast (rama `DC-002`)

El **fin último**: el juego (motor OpenDiablo2 del fork sermare) corriendo en la Dreamcast real,
local, un jugador, con el control de la consola. Nada de red. La consola física se prueba cuando
Papu lo pida, no antes.

Este documento es el **orden vigente** y el criterio de aceptación de cada fase. La justificación
medida de por qué se pivoteó del POC al motor está en `informe-continuidad.md` (2026-10-29); acá
sólo se ordena el trabajo y se anota qué está cerrado.

Reglas que no se negocian:

- Una rama (`DC-002`), un agente, una carpeta.
- Todo se cierra con **evidencia medida** en `console/dc/evidencia/` (número, log o captura).
  "Funciona" sin número es un verde mentiroso.
- El POC **no crece como juego**: es el banco de pruebas del backend de plataforma.
- Toda tarjeta lleva su `PLAN (peso · hecho · qué)`; el porcentaje se deriva, no se escribe.

---

## Fase A — La lógica del juego compila para SH-4  ·  **CERRADA (DC-002.66)**

`d2common`, `d2core`, `d2game`, `d2app`, `d2networking`, `d2script`, `d2thread` compilan a `.gox`
para `sh-elf` con gccgo 15.1: **139 de 139 paquetes**.

Criterio cumplido: N/M con la lista de los que fallan y por qué (ninguno falla).
Evidencia: `console/dc/evidencia/dc-002.66-motor-compila-sh4.txt`.

## Fase B — Del `.gox` al binario que cabe en 16 MB  ·  **EN CURSO**

Dos mitades, y la primera va antes porque la medición de DC-002.67 lo manda:

**B.1 — Que el binario deje RAM para dibujar.** El motor entero enlaza (125 paquetes, un solo
`.text` por paquete) y arranca en Flycast, pero pide 11.664.488 B de `1ST_READ.BIN` y el montón no
puede reservar **ni 1 MB** (4 MB, 1 MB y 1.228.800 B fallan; 256 KB entra). Enlazar más grande no
sirve: hay que recortar (DC-002.68).

**Medido en DC-002.68 (cerrada):** con `-g0 -ffunction-sections -fdata-sections` + `--gc-sections` +
el `.eh_frame` fuera del mapa, el `1ST_READ.BIN` baja a **10.337.608 B** (−1.326.880, −11,4 %) y la
**pantalla del motor (1.228.800 B) ya entra** en el montón (techo medido 6.153.956 B; el `--gc-sections`
solo tira 448 KB porque el 96 % de lo que queda está vivo: gccgo encadena el `init` de cada paquete).
Lo que **no** entra es el bloque de 4 MB de texturas: pide 8,39 MB de techo y hay 6,15 MB; quedan tres
caminos medidos (enlazar menos paquetes —`d2gamescreen`+`d2player` son 2,26 MB—, `-Os` ~370 KB, o bajar
el semiespacio del GC, decisión de DC-002.51). Evidencia: `console/dc/evidencia/dc-002.68-recorte.txt`.

**B.2 — La interfaz de render y el backend PVR.** *Corregido el 2026-10-29 midiendo el árbol:* el
contrato **ya existe** y no hay que extraerlo — `d2common/d2interface/surface.go` define
`Surface` (22 métodos: `DrawRect`, `DrawTextf`, `PushSkew`, `PushScale`, `PushFilter`, `Render`,
`RenderSection`, `ReplacePixels`, `Screenshot`…) y `d2common/d2interface/renderer.go` define
`Renderer` (15 métodos: `Run`, `CreateSurface`, `NewSurface`, `CurrentFPS`, `PrintAt`…). `d2ui`,
`d2gui`, `d2asset` y `d2font` ya consumen esas interfaces, no la implementación.

Lo que falta es la **implementación para la consola**: hoy `d2core/d2render` tiene sólo
`d2render/ebiten`. Se escribe `d2render/pvr` detrás de esas mismas interfaces, partiendo del
rasterizador por software que ya tiene el stub de ebiten (`console/dc/engine/ebiten`, 10 archivos,
96.720 B; 463 líneas de C suben la pantalla al PVR) y del código de dibujo del POC.

Criterio de aceptación de la fase: **el mismo mapa y el mismo personaje de hoy, dibujados por
`d2interface.Surface` con el backend PVR**, con la captura comparada contra la del POC (mismos
píxeles donde debe), y sin el montón ahogado.

**Medido en DC-002.69 / DC-002.70 (cerradas, reverificado en DC-002.73):** `d2render/pvr` implementa
los **21** métodos de `Surface` (no 22: el que sobraba en este plan son `ShadedSurface`/`ShadedGridSurface`,
que son otras interfaces) con la aserción de contrato en `surface.go:51`, y los **15** de `Renderer`
(`renderer.go`). En Flycast la prueba de humo da 73 sprites, 21584 píxeles, `crc32 -2001956231` estable en
360 cuadros; el POC *estado B* dibuja el mismo mapa y el mismo personaje por la interfaz con `LIENZO
crc32 3507647456` y `PERSONAJE crc32 1857478036` (los mismos del verificador del host), el lienzo del
cuadro en VRAM (`lienzoEnVram= 1228800`) y **98,79 %** de píxeles idénticos del cuadro (474 filas) contra
el POC de referencia, con 0,19 % de cuantización ≤8 y el 1,03 % restante en las bandas que la referencia
no limpia. Evidencias: `console/dc/evidencia/dc-002.69-pvr-surface.txt`,
`dc-002.70-renderer-pvr.txt` y la verificación independiente `dc-002.73-verificacion.txt`.

## Fase C — Input y audio  ·  input MEDIDO (DC-002.71), audio MEDIDO (DC-002.72)

Maple detrás de `d2interface` (`input_service.go`, `input_manager.go`; es la más fácil) y AICA
detrás de `d2interface` (`audio_provider.go`, `pcm_stream.go`, `sound_effect.go`; la lógica ya está
separada del backend y el POC ya tiene 64 canales sonando).

Criterio: la cruceta y los botones del control mueven al personaje **del motor**, y un sonido del
juego suena por AICA distinguiéndose del que genera el POC.

**Medido en DC-002.71 (la mitad de input, cerrada):** `d2core/d2input/maple` implementa los **9**
métodos de `InputService` sobre el bus maple, con el mapa de `control.go` (cruceta → flechas, A →
Enter + clic izquierdo, B → Escape + clic derecho, X → X + clic del medio, Y → Y, Start → Enter,
gatillos L/R → Q/E, palanca/cruceta → el cursor) y la aserción de contrato en `traduccion.go`. En
Flycast los **nueve** botones llegan y se traducen con sus dos flancos (log `HITO271: FLANCO`), el
cursor del motor pasa de 320 a 332 en un cuadro con la cruceta y se recorta en 639x479; 12 pruebas en
el host con 8 mutantes cazados; el motor compila **144 de 144** para SH-4 (el paquete nuevo es el
144). Evidencia: `console/dc/evidencia/dc-002.71-maple-input.txt`. Lo que faltaba de la fase en ese
momento era el audio (AICA, cerrado en DC-002.72) y que el personaje sea el del motor con
`d2gamescreen` (Fase D).

**Medido en DC-002.72 (la mitad de audio, cerrada):** `d2core/d2audio/aica` implementa los **3**
métodos de `AudioProvider` (`PlayBGM` queda **no-op declarado**: la música es streaming y el cuerpo
la deja afuera) y los **5** de `SoundEffect` sobre el canal del AICA, con el camino de KOS en
`console/dc/std/csrc/sonido_dc.c` (`snd_init` → `snd_sfx_load_raw_buf` → `snd_sfx_play_ex`). El
efecto de la prueba **es del motor**: `data/global/sfx/combat/impact/sword5.wav` resuelto por
`d2soundpath` y leído de `d2Sfx.mpq` (48,9 MB, en el disco), 20170 B → 10062 muestras de 22050 Hz
mono; el chip pasa de `canalesSonando=0` a **1** al sonar y vuelve a **0** solo a los 436 ms (el
contador sale de `snd_is_playing`/KEYONB, y la posición avanza hasta 10062), `SetPan`/`SetVolume` en
vuelo llegan al registro (`pan=0`, `pan=255`, `vol 204→102`) y el efecto con `loop` sigue sonando más
allá de su duración (pos=6034 a los 700 ms = dio la vuelta). Volúmenes: `1.0→255`, `0.8→204`,
pan `-1→0 / 0→128 / +1→255`. 10 pruebas en el host con **4 mutantes cazados** (uno destapó un hueco
real de la suite: faltaba el caso de bloque RIFF de largo impar). El motor compila **145 de 145** para
SH-4 (el paquete nuevo es el 145). Evidencia: `console/dc/evidencia/dc-002.72-aica-audio.txt`. Lo que
falta de la fase es que el personaje sea el del motor con `d2gamescreen` (Fase D).

## Fase D — Montar `d2gamescreen`  ·  pendiente

El `main` de consola instancia las pantallas del juego (menú, selección de personaje, acto 1) en vez
de la lógica del POC. Acá aparece el riesgo real: **16 MB de RAM** contra un motor pensado para PC.

Criterio: **acto 1 jugable de punta a punta con el control de Dreamcast, sin el POC en el medio.**

**Medido en DC-002.81 (el arranque del motor, cerrado):** el `main` de consola tiene que llamar
**una** función — `d2asset.ArrancarComoConsola()` — antes de `d2app.Create`; junta las tres palancas
que antes vivían sueltas (presupuestos de consola de .75, lectura con tamaño reservado de .79 y arena
de tiles con lectura prestada de .78/.80). Con esa línea, el AssetManager del motor
(`NewAssetManager`, `d2app/app.go:134`) queda con `caminoDeTilesDeConsola= true`,
`lecturaPrestada= true`, `lecturaDeArchivosDeConsola= true`, `prefetchApagado= true` y los caches en
2.949.120 B, y las tres fases de zona cargan en Flycast sin OOM y sin excepción con
`pico(c) - pico(a) = 0 B` y el pueblo en crc32 3507647456. Sin la línea, el mismo camino muere en el
**segundo** `.dt1` de la primera zona (`gc_external_alloc: OOM`). El `.ds1` y los `.dt1` ya los carga
el motor (`am.LoadDS1` / `am.LoadDT1`); lo que falta de la fase es el compositor (`d2mapengine` +
`d2mapstamp`), que necesita las ~70 tablas de `d2resource.DataDictionaries()`, y las pantallas.
Evidencia: `console/dc/evidencia/dc-002.81-arranque-de-consola-en-el-motor.txt`.

**Medido en DC-002.82 (el compositor del motor, con la pared puesta en numeros):** las ~85 tablas de
`DataDictionaries()` pesan **14.954.280 B (14,26 MB)** en el host y las 12 del compositor **4.243.176 B**;
en la consola entran **4** de esas 12 (LvlTypes, LvlPrest, LvlWarp, objtype = 618.552 B medidos) y la quinta
(`Objects.txt`) muere en el semiespacio de 2 MB con el recolector frenado y tambien con el recolector libre.
Con las dos imprescindibles (LvlTypes + LvlPrest) `d2mapengine.CreateMapEngine` **si** se crea, y el pueblo
NO se compone: el primer `.dt1` de `LoadStamp` pide el arena de tiles entero (4.849.148 B de una sola `make`)
cuando al monton le quedan 4.554.752 B -- `fatal error: gc_external_alloc: OOM`, faltan 294.912 B. Detras del
arena hay ~9,3 MB de lectura por fase (LoadStamp + ResetMap leen los mismos 9 `.dt1`, con un cache de dt1 de
1.048.576 B) y ~1,0 MB de estructuras (MapTile 2.337x264 = 616.968 B, dt1TileData 603x272 = 164.016 B, .ds1
parseado 186.960 B). El control con el POC de zonas de .81 recompilado con el mismo arbol de `.gox` reproduce
.81 exacto (pico 10.401.264 B, pico(c)-pico(a) = 0, crc32 del pueblo 3507647456) y deja la causa medida: la
imagen del compositor es **2.277.728 B mas grande** (1ST_READ 6.847.080 contra 4.569.352) y el monton al
arrancar **2.273.280 B mas chico** -- imagen y monton se mueven 1:1. Evidencia:
`console/dc/evidencia/dc-002.82-compositor-en-la-consola.txt`.

**Medido en DC-002.83 (el cooperativo desbloqueado: el map de animdata ya no se pierde en el GC):** el
bloqueo que dejo DC-002.76 ("el 2º jugador no se crea") no era del mapa sino del camino de los objetos
grandes del recolector: `gc_alloc` manda todo pedido de mas de 64 KB a `malloc` (fuera de los dos
semiespacios), y ahi el GC no barre nada. El array de buckets de `map[string][]*AnimationDataRecord` con
2.556 claves pesa **88.064 B** (bucket de 172 B x 512) y se iba por ese camino: los nombres de 8 letras y
los registros colgaban solo de el, quedaban invisibles (`gc-vivo` = 134.640 B con el mapa lleno) y en la
segunda recoleccion -- cuando el espacio viejo se reusa -- se reciclaban: la busqueda por clave fallaba
mientras el `count` seguia en 2.556. Arreglado en el runtime (`fix-libgodc-externos.py`): los externos con
punteros se registran y se barren en el lugar, sin copiarlos. Medido en Flycast: sonda `map-gc`, 2.556 de
2.556 claves encontradas en las rondas 0, 1, 2 y 3 (antes 2347 y 1856, con 209 y 700 claves perdidas), y el
POC del cooperativo crea **J1, J2, J3 y J4** sin una sola linea `FALLO` (antes J2 respondia "could not find
Animation data" en cada cuadro). Evidencia: `console/dc/evidencia/dc-002.83-el-map-de-animdata-y-el-gc.txt`.

**Medido en DC-002.86 (las tablas de records NO son la pared: 0,9 % de uso y el presupuesto del compositor):**
instrumento nuevo `console/dc/medidas/uso-tablas` (host) que mide el otro lado del numero de DC-002.82 — lo
que el pueblo **usa**. De las 12 tablas del compositor (4.240.928 B cargadas) el pueblo pide **40.040 B
(0,9 %)**: 6 tablas no se tocan ni una vez (LvlWarp, AutoMap, Levels, LvlMaze, LvlSub, objtype) y de las 6 que
si, lo maximo es 8 de 733 filas (monstats), 8 de 573 (Objects, ids {35,36,37,39,78,119,267,385}), 8 de 229
(monpreset), 1 de 1091 (LvlPrest) y 1 de 36 (LvlTypes). Camino elegido: **(a) cargar solo las tablas que el
nivel pide** (435.248 B en el host / 409.816 B medidos en consola, contra 4,04 MB de las 12 y contra los
28.576 B de monton externo que quedan despues del arena). Aplicado y medido en Flycast: 2 tablas OK (92 y
467 ms), y el pueblo muere igual en el arena de tiles, faltando **299.008 B** (4.849.148 B de una sola `make`
contra 4.550.656 de margen). **La pared no son las tablas**: con CERO tablas el compositor pide 6.168.348 B
(arena constante 4.848.148 + estructuras 967.944 + AssetManager 352.352) contra 5.230.592 de margen, o sea
faltan **937.756 B** igual; de las tablas solo compite LvlPrest (324.112 B). Queda para la proxima tarjeta el
recorte del arena/imagen con ese presupuesto. Evidencia:
`console/dc/evidencia/dc-002.86-tablas-lo-usado-y-el-camino.txt`.

**Medido en DC-002.93 (el hueco de `gc_init`: los 450.560 B eran de otro binario):** el `gc_init: alloc
failed` de DC-002.87c se midio sobre el motor de DC-002.28 (13.775.848 B); el MISMO camino rearmado con el
arbol de hoy mide **11.650.536 B** y **pasa los dos semiespacios** (separacion 2.097.184 B, monton de C
despues del GC **481.056 B**), llega a `main.main` y a `d2app.Create().Run()`, y muere 50 s despues en
`gc_external_alloc` (131.072 pedidos, 45.056 al techo). Escalera medida: 1536 KB de semiespacio lo lleva a
las tablas (muere pidiendo 212.992 B), 768 KB lo mantiene vivo 60 s pero con **511 recolecciones** (el
semiespacio al 80 %: thrashing), y 1024 KB se cae por el defecto de barrido de DC-002.92. Con las dos
palancas juntas — **semiespacio 1536 KB + `.eh_frame` fuera (DC-002.47, −920.864 B de imagen = +920.864 B
de monton, 1:1)** — el motor queda con **2.450.496 B** de monton de C, **5 tablas completas** y **10
recolecciones**, y a los 180 s sigue sin OOM ni excepcion (se queda en la lectura de `weapons.txt`: la
proxima pared, ya no de RAM). El compositor de las tres fases **no baja de 2048 KB** (con 1536 y 1024 muere
por conjunto vivo), asi que el semiespacio es una palanca POR BINARIO: comprobado a HEAD, el compositor
compone el pueblo con los crc -1596781041 / -205471136 / -1596781041 y margen 49.152 B al techo. Instrumento
nuevo: `scripts/console/medir-hueco-gc-293.sh` (`leer` un log, o `escalera` completa). Evidencia:
`console/dc/evidencia/dc-002.93-el-hueco-de-gc-init.md`.

---

## Lo que se reusa (no se tira)

`libgodc` (GC + goroutines + scheduler en SH-4), el semiespacio de 2 MB, PVR (sprites con paleta,
tiles del DS1, composición caja/esquina), GD-ROM (MPQ, `.gdi`, el bug del sector del audio),
AICA (64 canales), maple, y toda la instrumentación (watchpoints del SH-4 en Flycast, traza por
rango de PC, mutantes).

## Riesgo principal, sin adornos

El render es la pieza grande y no está probada: `d2ui` dibuja con blending, escalado y texto, y PVR
no es OpenGL. Puede que el motor pida algo que el PVR no haga barato. Por eso el render va antes
que cualquier feature de juego.

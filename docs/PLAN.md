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

## Fase C — Input y audio  ·  pendiente

Maple detrás de `d2interface` (`input_service.go`, `input_manager.go`; es la más fácil) y AICA
detrás de `d2interface` (`audio_provider.go`, `pcm_stream.go`, `sound_effect.go`; la lógica ya está
separada del backend y el POC ya tiene 64 canales sonando).

Criterio: la cruceta y los botones del control mueven al personaje **del motor**, y un sonido del
juego suena por AICA distinguiéndose del que genera el POC.

## Fase D — Montar `d2gamescreen`  ·  pendiente

El `main` de consola instancia las pantallas del juego (menú, selección de personaje, acto 1) en vez
de la lógica del POC. Acá aparece el riesgo real: **16 MB de RAM** contra un motor pensado para PC.

Criterio: **acto 1 jugable de punta a punta con el control de Dreamcast, sin el POC en el medio.**

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

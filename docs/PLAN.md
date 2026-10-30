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

**B.2 — La interfaz de render y el backend PVR.** Hoy `d2core/d2render` es sólo la carpeta
`ebiten`: no hay contrato. Se extrae la interfaz que consume `d2ui` (42 imports) y se escribe el
backend PVR detrás, partiendo del rasterizador por software que ya tiene el stub de ebiten
(`console/dc/engine/ebiten`, 10 archivos, 96.720 B; 463 líneas de C suben la pantalla al PVR).

Criterio de aceptación de la fase: **el mismo mapa y el mismo personaje de hoy, dibujados por la
interfaz nueva, con la captura comparada contra la del POC** (mismos píxeles donde debe), y sin el
montón ahogado.

## Fase C — Input y audio  ·  pendiente

Maple detrás de `d2input` (la interfaz está limpia, es la más fácil) y AICA detrás de `d2audio`
(la lógica ya está separada del backend; el POC ya tiene 64 canales sonando).

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

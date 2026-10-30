//go:build dreamcast

// Package pvr es el BACKEND DE RENDER DE LA DREAMCAST: la implementacion de d2interface.Surface sobre el
// PowerVR2 (PVR), para el motor de Diablo II.
//
// POR QUE EXISTE. El motor dibuja contra d2interface.Surface, y hasta hoy la unica implementacion era
// d2core/d2render/ebiten (Ebitengine, PC). En la consola no hay Ebitengine: hay un chip de video que
// dibuja listas de triangulos desde la VRAM. Este paquete es la pieza que traduce "pinta este sprite en
// (x, y)" al PVR.
//
// COMO DIBUJA (la decision de arquitectura, y por que es esta).
//
//  1. el motor compone su cuadro en una imagen RGBA en memoria (1 byte por canal), igual que en la PC.
//     Todos los metodos de Surface trabajan ahi: eso es lo que hace que RenderSection y Screenshot
//     puedan existir (con el cuadro solo en la VRAM no habria de donde leer los pixeles).
//  2. al cerrar el cuadro, la imagen se convierte a RGB565 por bloques de 32x32 y se sube a la VRAM
//     (d2c_pvr_pantalla_convertir, en console/dc/std/csrc/pvr_pantalla.c: el camino MEDIDO en
//     DC-002.10/17/22/26, no uno nuevo).
//  3. el PVR pinta ese cuadro a pantalla completa.
//
// Es el mismo camino que usa el shim de console/dc/engine/ebiten: aca no se invento nada de video, lo que
// se agrega es la CAPA DE CONTRATO (los 21 metodos de d2interface.Surface con su semantica real, copiada
// de d2core/d2render/ebiten) y el sprite paletizado de 8 bits (DC6/COF) tanto por software como por el
// PVR.
//
// EL RENDERER (DC-002.70): renderer.go es la implementacion de d2interface.Renderer (Run con el ciclo de
// cuadros, CreateSurface/NewSurface, CurrentFPS, el cartel de panico y el texto de depuracion). Un cuadro
// se compone en el lienzo de la Surface y el Renderer lo sube a la VRAM y lo pinta. La superficie del cuadro
// vive en la VRAM (SuperficieDeLaPantalla) porque el monton no tiene el lienzo y el mapa del nivel a la vez.
// El POC console/dc/poc/estado-b dibuja con todo esto el MISMO mapa y el MISMO personaje que el POC de
// DC-002.38 (que dibuja con el shim de ebiten), y sus capturas se comparan pixel a pixel.
//
// LO QUE SIGUE SIN ENTRAR: d2gamescreen montado (es la Fase D).
//
// LO QUE NO PUEDE SER FIEL, DECLARADO (no escondido):
//   - PushFilter(FilterLinear) se dibuja con muestreo NEAREST: el rasterizador por software no interpola.
//     El motor de Diablo II pide NEAREST casi siempre (el arte ya viene al tamano que se ve), pero un
//     filtro linear pedido a proposito sale crispado. Se anota en la evidencia.
//   - DrawTextf usa una fuente 3x5 propia (PROVISIONAL): la de verdad es d2font + los DC6 de la interfaz,
//     que llega con el Renderer.
//   - el sprite por HARDWARE (DibujarSpritePVR) usa el banco 0 de la paleta del PVR, que es UNO: dos
//     paletas distintas en el mismo cuadro no se pueden dibujar por hardware (se puede, por software:
//     Surface.DibujarSprite).
//
// MEDIDO (2026-10-29, evidencia/dc-002.69-pvr-surface.txt): ver ahi los numeros de la prueba de humo.
package pvr

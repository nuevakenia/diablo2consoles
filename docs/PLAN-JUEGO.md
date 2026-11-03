# Plan del juego completo, en la Dreamcast

Fecha: 2026-11-01. Ramo `DC-002`, checkout único `/home/dgalli/Proyectos/diablo2consoles`.
Este documento ordena lo que falta para pasar de "el mundo y la gente se dibujan" a "el juego se puede jugar".

## 0. El principio que ordena todo

**El motor ya tiene los sistemas escritos. No hay que inventarlos: hay que portarlos.**

Los paquetes existen y están en el árbol (líneas de Go, medidas hoy):

| sistema | paquete | líneas |
|---|---|---|
| monstruos | `d2common/d2monster` | 32.552 |
| habilidades | `d2common/d2skill` | 21.869 |
| héroe y clases | `d2core/d2hero` | 12.235 |
| quests | `d2common/d2quest` | 11.272 |
| combate | `d2common/d2combat` | 7.264 |
| objetos de UI | `d2core/d2ui` | 3.834 |
| objetos del mundo | `d2common/d2object` | 3.614 |
| stats | `d2common/d2herostats` / `d2statlist` | 3.193 / 2.974 |
| mercaderes | `d2core/d2vendor` + `d2common/d2trade` | 2.609 / 1.774 |
| inventario | `d2core/d2inventory` | 2.402 |
| GUI | `d2core/d2gui` | 2.407 |
| NPCs | `d2common/d2npc` | 2.137 |

En esta consola, portar **no** es un problema de lógica: es un problema de **memoria**. Todo lo que se
mide desde DC-002.74 lo confirma: imagen del binario, montón de C (`gc_external_alloc`, sin `free` para
bloques de más de 64 KB) y semiespacio del recolector (1.835.008 B). El heap de Go de libgodc no aguanta
objetos grandes, y por eso el ADR [0001](adr/0001-assets-fuera-del-heap-de-go.md) manda sacar los buffers
grandes a tramos con dueño.

**Regla de cada etapa:** entra con su presupuesto medido **antes** (imagen del binario + montón + semiespacio)
y con un criterio de aceptación que se pueda contar en la consola, no en una foto.

## 1. Dónde estamos (2026-11-01)

En la Dreamcast, medido y con captura:

- el pueblo del Acto 1 se compone y se dibuja (crc32 del host `1437022778`);
- el ciclo del mundo: entrar, transición y volver (DC-002.111, DC-002.120);
- el jugador se compone desde el `.dcc` con 0 recolecciones (DC-002.98) y camina con el control (DC-002.113);
- 1 a 4 jugadores en la misma pantalla (DC-002.114, DC-002.117) y split de 2 y de 4 (DC-002.116, DC-002.118);
- el leash medido en 5,25 celdas (DC-002.115);
- monstruo y golpe: **el monstruo se dibuja, patrulla y muere** (DC-002.121, DC-002.122: el alcance lo decide la
  tabla del exe, 5 sí / 8 diagonal no / 28 no, y con los cuatro dándole al mismo bicho);
- drop y recogida: **el bicho suelta un objeto de su clase de tesoro, se dibuja en el suelo con su DC6 y el
  jugador lo recoge** (DC-002.123: `aqv`/Arrows de `Act 1 H2H B`, 145 px pintados, recogido a 0,316 celdas con el
  radio de 0,9 del motor, y el mapa solo sigue en `1437022778`).

Lo que no existe todavía: inventario, alijo, mercaderes, NPCs, quests, habilidades, menús ni guardado.

## 2. Las etapas, en el orden propuesto

El orden no es el del juego original: es el de **lo que hace jugable al juego con menos memoria**. Primero lo
que se toca con las manos (matar, recoger, guardar), después lo narrativo (quests, diálogos, mercaderes).

### E1 · Monstruos y combate — HECHA EN LO BÁSICO

- `d2monster`, `d2monstats`, `d2path`, `d2combat` (parte).
- Criterio: un monstruo se dibuja, patrulla y muere con su animación cuando el jugador le pega. **Cumplido**
  (DC-002.121 el bicho dibujado y patrullando; DC-002.122 el golpe, el alcance del exe, la vida, la muerte y
  los cuatro contra el mismo bicho; DC-002.126 la muerte **se ve**: los 19 cuadros del DT uno por cuadro del
  POC, el cadáver con su compuesto de 819 px y su expiración a los 12,0 s del motor).
- Riesgo: el `.dcc` de monstruo es más grande que el del jugador; medir cuántos caben en el tramo. **Medido**: el
  DT del Returned son 171.304 B por dirección y el A1 del héroe 74.971 B: con 8 direcciones del A1 y 2 del golpe
  el semiespacio (1.835.008 B fijos) muere al 96 %. Entra 1 dirección del A1 y 1 de los modos (91 %).
- Lo que falta para cerrarla del todo: IA (persecución y ataque del bicho al jugador), que es otra tarjeta.
- **La muerte se ve, medido** (DC-002.126): el "DD vacío (0 píxeles)" de la 122 era del DIBUJO, no de los datos:
  con 2+ jugadores el POC dibuja por `split.go` y ese camino no pintaba monstruos. El `.cof` del DD son 148 B con
  7 capas, 1 cuadro y 8 direcciones, y compone 92x39 con 819 px (dir 6, `1HS` declarado por capa); en pantalla se
  ven 19 cuadros distintos del DT (crc32 por cuadro, idénticos entre corridas), 4 cuadros con el cadáver
  (`crc32=1212887229`) y 0 px desde el cuadro 36, con el mapa solo intacto en `173308762`. El reloj del DT es
  `muertePorCuadro` (instrumento: un cuadro de animación por cuadro del POC) porque los 931 ms del animdata no
  alcanzan para el cuadro de la consola (~3,2 s con cuatro jugadores).

### E2 · Objetos y drop — HECHA EN LO BÁSICO

- `d2object`, `d2item`, las tablas de `itemtype`/`weapons`/`armor`.
- Criterio: al morir el monstruo, suelta un objeto del `.ds1`/tabla; el objeto se dibuja en el suelo y se
  puede recoger. **Cumplido** (DC-002.123: el drop sale de la tabla del bicho —`monstats` del parche ->
  `TreasureClassEx` -> `weapons`/`armor`/`misc`—, el objeto se dibuja con su `flippyfile` y su paleta, y se
  recoge con la regla del motor, `pickupRange` = 0,9 celdas).
- Lo que hay que medir: el costo de las tablas de ítems (son de las gordas) y si el drop va por el camino de
  tramos del ADR. **Medido**: las 6 tablas de ítems son **779.882 B** en el parche (`misc` 55.821, `weapons`
  118.029, `armor` 76.370, `itemtypes` 8.775, `TreasureClassEx` 88.253 y `monstats` 432.634), y el drop lee
  771.107 B de ellas. Van por tramo con dueño (`Fuente.Pedir`/`Liberar`): el montón de C queda **idéntico**
  antes y después (usado 8.418.232, libre 1.352.616, arena 9.770.848) y el semiespacio **no crece** (1.418.016 ->
  1.343.096). El código del drop suma **+23.364 B** al binario (17.850.480 contra 17.827.116 con `DROP=no`).
  Lo que queda vivo del objeto son 5.542 B de índices.
- Lo que falta para cerrarla del todo: el objeto recogido no va a ninguna parte (inventario, E3), el oro de las
  clases de tesoro se descarta, y la calidad/afinidades/rarezas no entran (el objeto es la fila base).

### E3 · Inventario y alijo

- `d2inventory`, `d2item`, `d2ui` (la grilla), y el alijo del pueblo (el objeto "stash" del `.ds1`).
- Criterio: abrir el inventario, ver la grilla, meter el objeto recogido, y el alijo del pueblo guardando y
  devolviendo.
- Riesgo alto de memoria: `d2ui` son 3.834 líneas de widgets y la grilla del inventario tiene cientos de
  celdas de 4 px. Medir antes de escribir.
- **La grilla y el alijo, medidos y andando** (DC-002.124): la tarjeta se cumple sin portar `d2ui`. Las
  dimensiones salen de `d2inventory.Dims` (10x4 y 6x8; 2.112 B de celdas), el panel es una imagen de índices
  armada una vez (75.392 B) y los tres colores salen de la paleta de las unidades, no de un RGB inventado. El
  objeto recogido entra a la grilla en la celda 0,0 con su huella 1x3 y se ve pintado en su celda; el alijo
  guarda y devuelve. Con el panel cerrado no se pinta un pixel: el crc32 del lienzo es el mismo antes y después
  de llamar al inventario, cuadro por cuadro (19 cuadros medidos). Costo: 79.856 B de memoria viva, 93-115 ms por
  cuadro con los paneles abiertos (8-12 % del cuadro) y 14.532 B de binario.
  - Lo que el mapa de este disco NO trae: el cofre. Los cuatro `.ds1` del pueblo tienen 170 objetos y 13 ids, y
    ninguno es stash (las filas 186/187/188/203..205 existen en `objects.txt` pero no están colocadas). El POC lo
    declara y lo pone en una celda conocida.
  - Lo que falta: el cinturón (4 celdas, 96 B medidos), mover objetos DENTRO de la grilla, y la validación de
    `d2inventory` ("esto no cabe en ese hueco").
  - Con split, cada mitad tiene su grilla y su panel (18-19 ms por panel, medido con dos jugadores); lo que en el
    split todavía no pasa es recoger (eso es del camino de un jugador).

### E4 · El HUD del juego

- `d2ui`: barras de vida y maná, el cinturón (4 pociones), el texto de la zona, el mini mapa (`d2automap`).
- Criterio: el HUD se dibuja sobre el pueblo con los valores reales del jugador, y en split cada mitad tiene
  el suyo.
- Es la etapa que hace que "se vea un juego" con el menor costo: va antes que quests.

### E5 · Mercaderes y comercio

- `d2vendor` (2.609), `d2trade`, `d2playertrade`.
- Criterio: hablar con un mercader del pueblo, ver su inventario en la grilla y comprar/vender un objeto con
  oro.
- Depende de E3 (la grilla) y de E4 (el HUD con el oro).

### E6 · NPCs y diálogos

- `d2npc` (2.137), `d2ui` para el panel de diálogo.
- Criterio: un NPC del pueblo (Warriv, Charsi) se dibuja, se puede hablar y su diálogo sale en pantalla.
- Es el puente entre E5 y E7: las quests se dan hablando.

**HECHO en DC-002.127.** Medido antes de portar (sonda `console/dc/sonda-npc-127`): `d2npc` son 1.531
líneas y su cierre es **1 paquete** (ebiten=false, d2asset=false) ⇒ se porta tal cual; el panel del
motor (`d2game/d2player/npc_menu.go`) cierra con **264 paquetes y ebiten=true** ⇒ se redibuja con el
pincel del POC. POC: `console/dc/poc/juego-127` (`npc_127.go`). Warriv, Charsi y Kashya se dibujan en
su celda del townN1 (15 monstruos en el `.ds1`, no objetos), con su COF de quieto y 72.672 B de
índices; el panel sale con el nombre, el texto de `string.tbl` (30/10/9 renglones) y las filas del
menú, a 108 ms por cuadro. Evidencia y capturas: `console/dc/evidencia/dc-002.127-*`.
Quedan para E6-bis/E7: las quests del diálogo y los mercaderes (Trade/Repair/Gamble).

### E7 · Quests

- `d2quest` (11.272), `d2level`, `d2drlg` para las zonas de quest.
- Criterio: la primera quest del Acto 1 (Den of Evil) se puede aceptar, completar y cobrar.
- El paquete más grande después de monstruos y skills: medir la imagen que suma antes de portarlo.

### E8 · Habilidades y stats

- `d2skill` (21.869), `d2statlist`, `d2herostats`, `d2skills` del core.
- Criterio: subir de nivel, asignar un punto y usar una habilidad con su animación y su efecto.
- Es la etapa más cara del proyecto: 21.869 líneas y el sistema de efectos (misiles, `d2missile`).

### E9 · Menús, guardado y selección de personaje

- `d2gui`, `d2screen`, `d2config`, y el guardado (`d2save` si existe en el fork; si no, escribir uno propio).
- Criterio: elegir clase, empezar una partida nueva, guardar y volver a cargar con el inventario intacto.

### E10 · Cooperativo completo

- Nada nuevo del motor: la gente que ya funciona (1-4) con inventario, alijo y quests.
- Criterio: cuatro jugadores con su inventario propio, el alijo compartido y la quest avanzando para todos.
- Es donde la decisión de RAM de Daniel (2026-10-31) paga: los assets se comparten y el estado por jugador
  son 104 B.

## 3. El presupuesto, que es lo que manda

Números medidos hoy que cualquier etapa nueva tiene que respetar:

- imagen del binario: **11,65 MB** con los 145 paquetes (techo del montón: ~6 MB libres, y el GC se lleva
  4,19 MB de semiespacios);
- semiespacio del recolector: **1.835.008 B**, con 4 jugadores al **88 %** (DC-002.119);
- VRAM: **8.388.608 B**, y el split de 4 deja **30.024 B** libres (DC-002.116): la VRAM es el techo del
  dibujo, no el montón;
- todo bloque de más de **64 KB** que se pida con `make` va a `gc_external_alloc` y no vuelve nunca: va por
  la `Source` de consola o por un tramo con dueño.

**El cuadro, medido por dentro (DC-002.132, pueblo con 1 jugador, HUD y minimapa prendidos):** **1.150 ms**
por cuadro = **0,86 cuadros por segundo**, y el desglose dice dónde está:

- **presentar 37 ms** (convertir los 307.200 píxeles a RGB565, subirlos a VRAM y pintar el quad) — constante
  con el blit viejo y con el nuevo ⇒ **el techo del POC es 27 cuadros por segundo aunque el mapa se dibuje
  gratis**; bajar el dibujo sin bajar la presentación no se ve en los cuadros por segundo;
- **trabajo 1.113 ms**: de eso **367 ms es el mapa** (bajó de 582 ms con el recorte por tile y el camino sin
  tinte: *-37 %*, y el crc32 del cuadro 1 sigue igual al host, `1437022778`) y **746 ms es el resto del POC**
  dibujando jugador, bicho, HUD y minimapa sobre el lienzo, cuadro a cuadro. **El próximo blanco no es el
  tile: es ese 65 % del cuadro.**
- **espera 3 µs**: Flycast no sincroniza, así que en la consola real el vsync sí va a costar.

Evidencia: `console/dc/evidencia/dc-002.132-cuadro-barato.txt` (A/B dentro del mismo binario con la palanca
`BLIT_RAPIDO`, y `console/dc/verif-blit-132` para la equivalencia byte a byte de los dos bucles).

**El cuadro, medido POR ETAPA (DC-002.134, mismo pueblo y mismo escenario):** esos 746 ms **no eran píxeles**:
eran el baile de medir (guardar ~100 KB de la caja del sprite, hashear la caja, devolver el mapa y volver a
pintar el sprite) y las líneas de evidencia por el puerto serie. Con el instrumento apagado y sin líneas:

| etapa | DC-002.132 | DC-002.134 | |
|---|---|---|---|
| actualizar | 8.531 µs | 6.581 | -23 % |
| mapa | 389.181 | 382.115 | -1,8 % |
| jugadores | 59.551 | **555** | -99,1 % |
| bichos | 23.144 | **411** | -98,2 % |
| HUD | 20.501 | 3.152 | -84,6 % |
| minimapa | 55.210 | 66.517 | +20,5 % |
| líneas por cuadro (`sinEtapa`) | 95.321 | **1.377** | -98,6 % |
| **dibujar** | 642.911 | 454.129 | -29,4 % |
| **cuadro (PVR)** | **688.941** | **501.145** | **-27 %** |

Con la ficha de DC-002.132: **1.150 → 501 ms** (-56 %) y **0,86 → 1,99 cuadros por segundo**, con el `crc32` del
mundo igual al host (`1437022778`), el jugador en 1.237 px y el bicho en 612 px.

**El criterio de 300 ms no se alcanza y no depende de este trabajo:** el mapa solo pesa **382 ms** (83 % del
cuadro) y la presentación 37 ms fijos ⇒ piso de **419 ms** con todo el resto del POC en cero. Y el mapa **no**
está limitado por el depósito de píxeles: pinta **1.539.584 px por cuadro** (248 ns/px ≈ 50 ciclos, cuando un
bucle de depósito son 12-15) en 242 tiles ⇒ **~1,2 ms por tile se van FUERA del bucle de píxeles**. El próximo
blanco es el trabajo por tile del renderer del mapa, no el bucle.

Evidencia: `console/dc/evidencia/dc-002.134-etapas-del-cuadro.txt` (líneas `JUEGO134: ETAPAS` y
`JUEGO134: PIXELES`, palancas `ETAPAS`/`VERBOSO`/`BLIT_PALABRAS`/`PINTAR_UNA_VEZ` y
`console/dc/verif-blit-134` con 10.000 casos de equivalencia de los tres caminos del blit).

**Antes de portar cada etapa:** medir cuánto suma al binario y cuánto pide al arrancar. Si no entra, se
recorta lo que la etapa no usa (el camino de `--gc-sections` ya demostró que solo tira 448 KB: el 96 % de la
imagen está vivo por los `init` de gccgo, así que recortar paquetes es la palanca real).

## 4. Lo que no se hace

- No se reescribe el motor: se porta, con palancas aditivas (`nil` por defecto) para que la PC siga igual.
- No se toca el vendor de libgodc dentro del repo (copia en `/tmp`, y se dice).
- No se cuenta un verde sin número en la consola.

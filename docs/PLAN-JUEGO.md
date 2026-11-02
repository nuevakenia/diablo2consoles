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
  los cuatro contra el mismo bicho).
- Riesgo: el `.dcc` de monstruo es más grande que el del jugador; medir cuántos caben en el tramo. **Medido**: el
  DT del Returned son 171.304 B por dirección y el A1 del héroe 74.971 B: con 8 direcciones del A1 y 2 del golpe
  el semiespacio (1.835.008 B fijos) muere al 96 %. Entra 1 dirección del A1 y 1 de los modos (91 %).
- Lo que falta para cerrarla del todo: IA (persecución y ataque del bicho al jugador), que es otra tarjeta, y
  que el cuadro DD del SK viene vacío (0 píxeles): el cadáver no se ve con esta animación.

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

**Antes de portar cada etapa:** medir cuánto suma al binario y cuánto pide al arrancar. Si no entra, se
recorta lo que la etapa no usa (el camino de `--gc-sections` ya demostró que solo tira 448 KB: el 96 % de la
imagen está vivo por los `init` de gccgo, así que recortar paquetes es la palanca real).

## 4. Lo que no se hace

- No se reescribe el motor: se porta, con palancas aditivas (`nil` por defecto) para que la PC siga igual.
- No se toca el vendor de libgodc dentro del repo (copia en `/tmp`, y se dice).
- No se cuenta un verde sin número en la consola.

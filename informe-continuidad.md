# Informe de continuidad — port de Diablo II a Dreamcast

Fecha: 2026-10-29. Rama `DC-002`. Reemplaza cualquier supuesto anterior sobre "qué motor estamos usando".

Este informe existe porque Papu hizo la pregunta correcta: **¿estamos portando el juego o rehaciéndolo?**
La respuesta corta es: la plataforma está bien encaminada y el juego está mal encaminado. Hay que pivotar
en una cosa concreta (no tirar nada). Abajo están las mediciones que sostienen eso.

---

## 0. Corrección de un error mío

En la conversación anterior afirmé que el motor vendorizado **no tenía quests ni diálogo con NPC**, basándome
en un `find` que se cortó en los primeros 5 resultados. Está mal. El repo tiene:

```
d2game/d2gamescreen/autoquest.go, autoquest_acts.go, autoquest_acts2.go, autoquest_act5.go,
autoquest_host.go, act_travel.go, act3_live.go, act3_pick.go, anya_frozen.go, autoboss.go,
autobossfight.go, autocube.go, auto_equip.go, autoflow.go, autoground.go, autoobject.go, autopanel.go ...
```

144 archivos en la capa de juego, **22 con quest / acto / NPC**. El fork declara 27/27 quests principales y
5/5 actos jugables de punta a punta, y eso es coherente con lo que hay en el árbol. Papu tenía razón y yo
estaba equivocado: la limitación que describí no existe en el motor.

---

## 1. Qué repo es este (verificado, no supuesto)

```
go.mod      -> module github.com/OpenDiablo2/OpenDiablo2
README.md   -> "OpenDiablo2 for macOS: a native Apple-Silicon Diablo II"
               "This fork exists to make one [native Mac version]"  fork de @sermare
```

Es el fork **sermare/OpenDiablo2**, el port nativo de macOS con ingeniería inversa sobre `Game.exe` 1.14b
(Ghidra, 11.257 funciones nombradas), que es exactamente el proyecto del 93% que Papu vio en la imagen.

Tamaño del motor vendorizado en nuestro árbol:

```
2725 archivos .go   el motor (d2core, d2common, d2data, d2game, d2script, d2app, d2networking)
 188 archivos .go   console/ (arranque para Dreamcast + instrumentos)
 975 commits        de historia del vendor, traidos completos
```

### ¿Estamos desactualizados?

- El motor entró a nuestro repo el **2026-10-09** (`ad81e84f`, "Quest system engine integration").
- Último commit que toca la capa de juego: **2026-10-12** (`f6fe5248`), igual que el último commit de
  contenido del upstream.
- Los commits del **2026-10-20** que menciona Papu son dos, y los dos son de **CI**:
  `CI: pin macOS jobs to macos-14` y `CI: skip progress-history snapshot branch`. No tocan el juego.

**Conclusión: no hay nada que actualizar.** Estamos al día con la lógica del juego. Actualizar no cambia nada
de lo que hacemos.

---

## 2. El hallazgo: el POC no usa el motor, usa los parsers

El POC (`console/dc/poc/personaje-mapa`, 1788 líneas) importa del motor **solo seis paquetes, y los seis son
formatos de archivo**:

```
d2common/d2fileformats/d2mpq    leer el .mpq del GD-ROM
d2common/d2fileformats/d2ds1    el mapa del pueblo
d2common/d2fileformats/d2dt1    los tiles
d2common/d2fileformats/d2cof    el ciclo de animacion de un personaje
d2common/d2fileformats/d2dcc    los sprites del personaje
d2common/d2fileformats/d2dc6    los sprites sueltos
```

No importa `d2core/d2map`, ni `d2core/d2monsters`, ni `d2core/d2hero`, ni `d2core/d2item`, ni `d2game`.
O sea: **el mapa, el personaje, el monstruo, el combate, la vida, los items, el giro y el sonido los escribimos
de nuevo a mano en el POC**, encima de los parsers.

Eso es literalmente lo que Papu sospechó: en el POC estamos haciendo una copia del juego, no un port.

### La cuenta que decide

Código de juego que YA existe en el motor y funciona (verificado en el árbol):

```
d2game + d2script + d2app    66.961 lineas   actos, quests, NPC, combate, inventario, guardado .d2s
d2core/d2ui                   3.834 lineas   paneles, botones, texto, skill tree
d2core/d2gui                  2.407 lineas   capa de GUI
d2core/d2screen                 197 lineas   pantallas
                            -----------
                            ~73.400 lineas de juego
```

Código de plataforma que hay que escribir para que ese juego corra en Dreamcast:

```
d2core/d2render/ebiten        1.333 lineas   (15 archivos) -> backend PVR
d2core/d2audio/ebiten           424 lineas   ( 4 archivos) -> backend AICA
d2core/d2input/ebiten           445 lineas   ( 5 archivos) -> backend maple
                            -----------
                             ~2.202 lineas de backend
```

**Un ratio de 1:33.** Por cada línea de backend que escribimos, el motor nos regala 33 líneas de juego.

Y el reparto de dependencias ya está a favor: **`d2game` no depende de ebiten en un solo archivo** (0 de 144).
Solo `d2core` (11 archivos) y `d2common` (1) lo tocan, y todos caen en los tres backends de arriba. La capa de
juego importa interfaces (`d2ui` 42 veces, `d2screen` 11, `d2gui` 6, `d2input/d2gamepad` 5), no la implementación.

---

## 3. Por qué el POC no fue tiempo perdido (y qué sí lo sería)

El POC resolvió, y eso está medido y commiteado, **el problema que nadie había resuelto: que Go corra en 16 MB
de SH-4**:

- `libgodc`: recolector de basura, goroutines y scheduler en Dreamcast. No existía.
- El semiespacio del GC en 2 MB (DC-002.51) y la separación pila/semiespacio medida.
- PVR funcionando: sprites con paleta, tiles del DS1, composición del personaje con su caja y esquina.
- GD-ROM: leer los MPQ del disco, el `.gdi` y el bug del audio que se comía un sector del `.text` (DC-002.61).
- AICA: 64 canales pedidos al hardware, dos PCM sonando (DC-002.62).
- Y toda la instrumentación: watchpoints del SH-4 dentro de Flycast, traza por rango de PC, mutantes.

**Eso se reusa entero.** No es trabajo perdido: es el backend, escrito antes de saber que era el backend.

Lo que sí sería tiempo perdido es **seguir creciendo el POC como juego**: agregar más monstruos, más items,
más combate, el inventario completo, las quests a mano. Todo eso ya está escrito 33 veces más grande en
`d2game`, y cada línea que hagamos ahí hay que tirarla el día que montemos el motor.

Un ejemplo concreto y actual: Papu notó que al moverse el personaje se desplazaba sin animación de caminar y
volvía al idle. **Tenía razón y era un bug nuestro**, no del juego: el POC tenía `modoTranquilo = "TN"` y
`modoCaminando = "TW"` definidos pero el ciclo de caminar no se componía. Se cerró en DC-002.49b (commit
`dfdd428d`): ahora se componen TN y TW para las 8 direcciones y la evidencia muestra `343 marcas ANIM` con
0 crc32 desconocidos. Costó una tarjeta entera y 273 líneas de evidencia encontrar y arreglar algo que en el
motor es una tabla de animaciones que ya existía.

---

## 4. Veredicto

**El enfoque de plataforma es el correcto. El enfoque de juego es el equivocado.**

- Bien: `libgodc`, GC, PVR, GD-ROM, AICA, maple, instrumentación. Es el 90% del riesgo técnico del port y ya
  está pagado. Sin eso no hay nada.
- Mal: el POC como *el juego*. Es un banco de pruebas que se convirtió en un mini-juego paralelo. Cada feature
  nueva ahí duplica lógica que el motor tiene verificada contra el `Game.exe` real.

**El pivote es una cosa concreta**: el POC deja de ser "el juego" y pasa a ser **el backend de plataforma**.
Su código de dibujo PVR se convierte en `d2render/pvr`; su código de audio en `d2audio/aica`; su lectura de
maple en `d2input/maple`. Encima de eso se monta `d2gamescreen` y se gana de golpe: actos 1 a 5, 27 quests,
NPC con comercio e identificación, UI completa, skill tree, inventario, guardado `.d2s`.

No hay que reescribir nada de lo hecho: hay que **dejar de escribir features de juego** y empezar a escribir
el adaptador.

---

## 5. Plan propuesto (fases medibles, no promesas)

### Fase A — Probar que la lógica del juego compila para SH-4 (barata, decisiva)

Compilar para `sh-elf` los paquetes de lógica pura, sin render: `d2common/d2act3`, `d2common/d2level`,
`d2game/d2autoscript`, `d2core/d2hero`, `d2core/d2item`, `d2core/d2stats`.

Estado hoy: los intentos fallan por **falta de `.gox`** (el runtime de la stdlib no está indexado para
sh-elf), no por características que gccgo no soporte. Es un problema de build, mecánico, el mismo que el POC
ya resolvió para sus seis paquetes.

Criterio de cierre: N paquetes de lógica compilan a `.gox` para `sh-elf`, con una lista de los que fallan y
por qué.

### Fase B — Crear la interfaz de render y el backend PVR

`d2core/d2render` **hoy es solo la carpeta `ebiten`**: no hay interfaz. Hay que extraer el contrato que usa
`d2ui` (42 imports) y escribir el backend PVR detrás. El material base es el código de dibujo del POC, que ya
sabe pintar sprites con paleta, tiles y composición con caja/esquina.

Criterio de cierre: el mismo mapa y el mismo personaje de hoy, pero dibujados **a través de la interfaz**, con
la captura comparada contra la del POC (mismos píxeles donde debe).

### Fase C — Input y audio

Maple detrás de `d2input` (la interfaz está limpia, es la más fácil) y AICA detrás de `d2audio` (la lógica ya
está separada del backend ebiten; el POC ya tiene 64 canales sonando).

Criterio de cierre: la cruceta y los botones mueven al personaje del motor; un sonido del juego suena por AICA
y se distingue del que genera el POC.

### Fase D — Montar `d2gamescreen`

El main de consola instancia las pantallas del juego (menú, selección de personaje, acto 1) en vez de la lógica
del POC. Acá aparece el riesgo real: **16 MB de RAM** contra un motor pensado para PC.

Criterio de cierre: acto 1 jugable de punta a punta con el control de Dreamcast, sin el POC en el medio.

### Riesgo principal, dicho sin adornos

El render es la pieza grande y **no está probada**: `d2ui` dibuja con blending, escalado y texto, y PVR no es
OpenGL. Puede que el motor pida algo que el PVR no haga barato. Es medible temprano (Fase B) y por eso va antes
que nada del juego.

---

## 6. Lo que NO cambia

- Una rama, un agente, una carpeta (`DC-002`).
- Todo medido y con evidencia; nada de "funciona" sin número.
- El objetivo: **el juego local, un jugador, con el control de Dreamcast**. Nada de red.
- La Dreamcast física se prueba cuando Papu lo pida, no antes.

## 7. Estado del tablero al escribir esto

```
done 95   running 1 (DC-002.65)   blocked 1 (hardware, dormida a proposito)   todo 0
3251 commits en DC-002, todo pusheado en origin/DC-002
```

Cerradas hoy: DC-002.50 (combate completo en consola), DC-002.49b (el giro, 16 ciclos), DC-002.61 (items, 41
objetos), DC-002.62 (sonido AICA), DC-002.64 (los builds dejan de vivir en tmpfs).

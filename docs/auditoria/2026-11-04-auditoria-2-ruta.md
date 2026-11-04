# Auditoría 2 — ¿en qué estado está el proyecto de verdad? (2026-11-04)

Continúa `2026-11-04-auditoria-demo.md`. Esa midió **por qué la demo se ve mal**. Esta mide **si el proyecto va
hacia el juego** — y la respuesta corta es: **no, se desvió, y el desvío está documentado en nuestro propio
repo desde hace seis días.**

---

## 0. El plan de esta auditoría (escrito antes de hacerla)

1. Medir qué es este repo y cuánto código de juego tiene.
2. Medir cuánto de ese código está montado en la consola.
3. Medir el ciclo de desarrollo real (build, arranque, prueba).
4. Revisar la ruta: ¿las tarjetas que se cerraron acercan al juego?
5. Revisar la métrica: por qué el tablero dice 97 %.
6. Contestar: ¿tenemos referencia del juego? ¿hace falta que Papu la dé?
7. Contestar: ¿conviene desarrollar en Go nativo sin emulador?
8. Proponer el instructivo nuevo.

---

## 1. Qué es este repo (medido)

```
module github.com/OpenDiablo2/OpenDiablo2      el fork sermare/OpenDiablo2 (macOS)
                                                   ingeniería inversa de Game.exe 1.14b (Ghidra, 11.257 funciones)
d2common   1.208 archivos   191.444 líneas   formatos, datos, tablas del juego
d2core     1.065 archivos   160.394 líneas   render, ui, input, audio, mapa, hero, item, stats
d2game       342 archivos    63.673 líneas   actos, quests, NPC, combate, inventario, guardado
d2networking 142 archivos    25.082 líneas   red (no se usa: el objetivo es local)
d2app         25 archivos     3.196 líneas   ventana, menú, ciclo de la app
                            ------------
                            ~444.000 líneas de Go, del juego completo
```

**El juego está escrito y es la referencia.** No es un motor vacío: es el port del D2 con actos 1-5, 27 quests,
NPC con comercio, inventario, skill tree y guardado `.d2s`. Y las reglas (velocidades, animaciones, daño,
alcances) están **verificadas contra el `Game.exe` real** por el trabajo de Ghidra del fork: es la mejor
referencia que se puede tener sin el binario original.

## 2. Cuánto de eso está montado en la Dreamcast (medido)

```
d2core/d2render/pvr   == backend PVR escrito (6.191 líneas en d2render, con build tag dreamcast)
d2core/d2input/maple == backend maple escrito
d2core/d2audio/aica  == backend AICA escrito
Fase A (motor compila para SH-4) == 139 de 139 paquetes (commit 114dbb89, 2026-11-02)
Fase D (montar d2gamescreen)     == NO EMPEZADA: ningún main de consola importa d2gamescreen
```

O sea: **el motor compila entero para la consola, tiene sus tres backends escritos… y nadie lo ha arrancado.**
El juego del motor nunca se ha visto en pantalla.

Y mientras tanto, ¿qué se ha construido? El POC:

```
console/dc/poc/juego-131   45 archivos   36.879 líneas   34 con build tag dreamcast   24 tocan pvr directo
```

## 3. El desvío, con nombre y fecha

Está escrito en `informe-continuidad.md` (2026-10-29), y lo firma este mismo asistente:

> «**El enfoque de plataforma es el correcto. El enfoque de juego es el equivocado.**»
> «Lo que sí sería tiempo perdido es **seguir creciendo el POC como juego**: agregar más monstruos, más items,
> más combate, el inventario completo, las quests a mano. Todo eso ya está escrito 33 veces más grande en
> `d2game`, y cada línea que hagamos ahí hay que tirarla el día que montemos el motor.»
> «El pivote es una cosa concreta: el POC deja de ser "el juego" y pasa a ser **el backend de plataforma**.»

Fecha del informe: **29 de octubre**. Tarjetas cerradas desde entonces: **DC-002.121 a DC-002.147**, todas
creciendo el POC como juego (monstruo, golpe, drop, inventario, alijo, HUD, minimapa, muerte, NPCs, quest,
mercaderes, demo jugable, animaciones).

**Eso es el desvío.** No fue una decisión: fue inercia. Cada tarjeta era razonable por separado («agreguemos el
drop»), y el conjunto se alejó del plan que nosotros mismos escribimos. La demo que Papu jugó es la
consecuencia: un mini-juego paralelo que reimplementa mal lo que el motor ya tiene bien.

### Por qué el POC no puede "mejorar" hasta ser el juego

- **No usa el motor.** Importa 6 paquetes de formato (`d2mpq`, `d2ds1`, `d2dt1`, `d2cof`, `d2dcc`, `d2dc6`) más
  datos (`d2monstats`, `d2path`, `d2quest`…). La lógica de juego —el giro, el reloj de animación, el alcance
  del golpe, el comportamiento del monstruo— **está reescrita a mano**.
- **Cada regla reescrita es una oportunidad de equivocarse**, y se equivocó en todas las que Papu vio: la
  dirección, el reloj, el alcance, el spawn. En el motor esas cuatro cosas son tablas verificadas contra el
  `Game.exe`.
- **El POC no tiene actos 2-5, ni quests 2-27, ni skills, ni waypoints, ni guardado real.** No es que le falte
  pulido: le falta el juego.

## 4. El ciclo de desarrollo, medido

```
build incremental del POC ............ 49 s
arranque al primer cuadro ............ 93 s
prueba manual ........................ 2-3 min
---- total por iteración ............. ~3 min, y solo se puede probar UNA cosa por corrida
```

Para el motor en Linux, en cambio — **medido hoy, no supuesto: el motor compila y arranca**:

```
go build -o /tmp/od2 ./                        rc=0, binario de 36 MB
renderer-created ............................. 10 ms
assets-ready (los MPQ del juego cargados) .... 169 ms
first-frame .................................. 357 ms
```

**357 milisegundos contra 93 segundos.** Doscientas sesenta veces más rápido, con los mismos assets y el juego
completo del motor (no el POC). Y encuentra solo los MPQ en `~/.wine-d2/drive_c/Games/Diablo II/` e importa los
personajes reales.

**El motor corre en Linux.** No hay que simular nada: es el mismo código que se quiere portar. El ciclo de
desarrollo del JUEGO puede ser de segundos si el juego se desarrolla en el motor y en Linux, y la Dreamcast se
usa solo para validar plataforma (memoria, PVR, tiempos).

## 5. La métrica, que es el problema más grave de todos

El tablero dice **97 %** porque mide **tarjetas cerradas**, no juego terminado:

```
done 189   blocked 12   todo 2      ->  189/191 = 99 % de las tarjetas
```

Y de esas 189, la enorme mayoría son del backend de plataforma (libgodc, GC, PVR, GD-ROM, AICA, instrumentación)
y del POC. **El juego jugable del motor está al 0 %** (Fase D sin empezar). Un tablero que dice 99 % cuando el
juego está al 0 % no es un tablero: es un adorno.

La métrica honesta tiene que ser por **hitos del juego**, no por tarjetas:

| hito | estado |
|---|---|
| H1 · el motor arranca en la consola y muestra su menú | 0 % |
| H2 · se elige personaje y entra al Acto 1 con el motor | 0 % |
| H3 · el jugador camina con el motor, animado y con el control | 0 % |
| H4 · combate, muerte y drop con el motor | 0 % |
| H5 · NPC, diálogo y quest 1 con el motor | 0 % |
| H6 · inventario, alijo y guardado `.d2s` con el motor | 0 % |
| H7 · los 5 actos y las 27 quests | 0 % |
| H8 · cooperativo local 1-4 (feature nueva, no existe en D2) | 0 % |

## 6. ¿Tenemos referencia de cómo es el juego?

**Sí, y es mejor de lo que Papu cree: está en el repo.**

- Las **reglas** están en `d2game`/`d2core`, traducidas del `Game.exe` 1.14b con Ghidra. Ahí está cuánto avanza
  un personaje, cuántos cuadros dura una animación, cuánto alcanza un golpe, cuánto daño hace, qué hace un
  monstruo cuando te ve. **No hay que adivinarlas: hay que leerlas.**
- El **comportamiento visual** se puede VER: el motor corre en Linux. Se puede jugar el D2 del fork y comparar
  lado a lado con el POC.

Lo que el código **no** dice, y donde sí hace falta Papu:

1. **El feel.** Que la caminata «se sienta» como D2 no se deduce de una tabla.
2. **El criterio de aceptación visual.** «Así está bien» / «así no» — 30 segundos de su mirada valen más que
   una tarde de medición.
3. **Las prioridades.** Qué prefiere si hay que elegir.

### Cómo trabajarlo sin interrumpir el desarrollo autónomo

**El banco de validación.** Cada cierto número de tarjetas cerradas, se junta un paquete de **capturas
comparativas lado a lado** (motor en Linux | POC en consola) con la pregunta concreta de cada una («¿la
dirección del sprite es la correcta?», «¿la poción va centrada?»), y se manda en **un solo mensaje**. Papu
contesta en bloque, en 5 minutos, y el desarrollo sigue. Nada de preguntas de a una: eso sí rompería la
autonomía.

## 7. ¿Conviene desarrollar en Go nativo sin emulador?

**Sí, para el juego. No, para la plataforma.** Son dos ciclos distintos y hay que separarlos:

| qué se desarrolla | dónde | ciclo |
|---|---|---|
| reglas, animaciones, quests, combate, UI del juego | **Linux, motor, Ebiten** | segundos |
| PVR, memoria, GD-ROM, AICA, maple, GC | **consola / Flycast** | 49 s + 93 s |

Y el POC deja de ser el lugar donde se escribe el juego: pasa a ser el backend, como decía el informe.

## 8. El instructivo nuevo

### La regla que ordena todo

> **El juego se escribe una sola vez, en el motor. La consola solo aporta los backends.**
> Si una regla de juego aparece escrita a mano en `console/`, es un bug de proceso.

### Las tres pistas de trabajo

1. **PISTA JUEGO (Linux, rápida).** El motor corriendo en Linux con Ebiten. Acá se arreglan las direcciones,
   el reloj, el alcance, el spawn, las quests. La referencia para comparar es el propio motor y las tablas del
   `Game.exe`. Ciclo: segundos.
2. **PISTA PLATAFORMA (consola).** Fase D: montar `d2gamescreen` sobre los tres backends que ya existen. Acá se
   mide memoria, PVR, tiempos. Ciclo: el de hoy, pero cada vez menos necesario.
3. **PISTA VALIDACIÓN (Papu).** Paquetes de capturas comparativas en bloque, cada cierto avance.

### El orden

1. **Hacer correr el motor en Linux y jugarlo.** Es la referencia y el banco de pruebas. Si no arranca, eso es
   lo primero (y es un problema de dependencias, no de Dreamcast).
2. **Fase D: el motor en la consola** — menú, selección de personaje, Acto 1. El POC queda como backend.
3. **Recortar el alcance del POC**: no se le agrega nada más. Lo que ya tiene se usa como backend.
4. **Recién después**, pulir la jugabilidad contra la referencia.

### Lo que hay que dejar de hacer

- No más features de juego en `console/`.
- No más demo del POC.
- No más «arreglar» el POC para que se parezca al juego: eso es reimplementar el juego a mano.

---

## 9. Lo que sigue sin medir (deuda)

- Si el motor arranca en Linux (dependencias de Ebiten v2.0.2 con Go actual). Se está probando.
- Cuánta RAM pide `d2gamescreen` en SH-4: es el riesgo real de la Fase D y no está medido.
- Si el PVR puede con lo que pide `d2ui` (blending, escalado, texto): tampoco.

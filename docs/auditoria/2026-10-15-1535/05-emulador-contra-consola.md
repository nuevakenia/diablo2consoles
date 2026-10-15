# 05 — Emulador contra consola real (tu preocupación, contestada)

Tu pregunta era: *"¿cómo evitamos desarrollar contra el emulador y que después en la consola corra mal porque el
emulador tenía más recursos?"*. Es la pregunta correcta, y tiene una respuesta útil porque **no todas las
mentiras del emulador son del mismo tipo**.

## Lo que el emulador NO puede mentir (y por eso es útil)

1. **La cantidad de RAM.** Flycast emula exactamente 16 MB de RAM principal y 8 MB de VRAM; PCSX2 emula 32 MB y
   4 MB de eDRAM del GS. No hay "más memoria" en el emulador: **si te pasás de RAM, revienta igual.** Todo el
   trabajo de memoria (cachés, paletas, horneado) se puede validar en el emulador con confianza.
2. **La forma del hardware.** Que el PowerVR2 no tenga shaders, que el GS no lea el framebuffer, que el mando
   tenga esos botones: eso lo sufre igual el emulador. Decisiones de API y de formato (PAL8 + CLUT, blending,
   sin shaders) no se pueden "resolver" en el emulador.
3. **El espacio de almacenamiento** y el formato del medio (GDEMU/SD, HDD del PS2).
4. **La coherencia de caché del SH-4 (para mal)**: si te olvidás de un `dcache_flush_range` antes de una DMA, el
   emulador **no se va a quejar** y la consola va a mostrar basura. Eso es al revés: el emulador es *demasiado
   indulgente*. Hay que escribirlo con disciplina desde el principio y probarlo en consola.

## Lo que el emulador SÍ miente (y por eso no alcanza)

1. **La velocidad.** Flycast, Redream y PCSX2 son recompiladores dinámicos corriendo en un PC con 12 hilos: no
   ejecutan un SH-4 a 200 MHz en tiempo real. Un juego limitado por CPU puede verse perfecto en el emulador y
   arrastrarse en la consola. **Esta es la mentira que te preocupa, y es real.**
2. **El relleno de pantalla y el ancho de banda de VRAM.** El PowerVR2 es *tile-based deferred*: el emulador
   aproxima el costo real de sobre-dibujo (overdraw) y de bandwidth. Un nivel que se ve liso en el emulador puede
   caer a la mitad dibujando muchas capas de sprites translúcidos en la consola.
3. **La velocidad del GD-ROM** (que en tu caso no importa: usás GDEMU) y el timing de la AICA.
4. **En PS2, más todavía**: PCSX2 recompila EE y VU y no modela con precisión los ciclos del GS.

## Cómo se trabaja entonces (la política del proyecto)

1. **Números de ciclos, no de reloj.** El presupuesto a 30 FPS en el SH-4 es de **6,7 millones de ciclos por
   frame** (200 MHz / 30). Se mide con los contadores de performance del propio SH-4 (`perf_cntr`) y con un
   medidor por frame dentro del motor, no con el reloj del PC. El número que importa es el que se mide en la
   consola; el del emulador sirve solo para comparar entre iteraciones.
2. **Modo intérprete como cota pesimista.** Flycast puede correr sin recompilador: es lento, pero más cerca del
   silicio. Si el juego sobrevive ahí, sobrevive de verdad. Se usa como filtro, no como medida final.
3. **La consola real entra temprano y no se va.** Tenés GDEMU en el Dreamcast y disco duro en el PS2: la prueba
   de aceptación de cada hito (F1, F4, F5 del informe 04) corre en la consola real, con la tele, no en la
   ventana del emulador. El emulador queda para el ciclo rápido del día a día.
4. **Instrumentación en el motor, no en el emulador.** El proyecto agrega un medidor de frame (update/draw, en
   microsegundos de CPU y en ciclos), contador de texturas en VRAM, contador de bytes de heap y de pausas de GC,
   que se enciende con una variable (como ya lo hace `OD2_AUTOPERF` en el host). Ese medidor es el mismo en host
   y consola, así que se puede comparar el mismo número en los dos lados.
5. **Pruebas de estrés del hardware:** el peor caso se arma a propósito (el pueblo con todos los NPC, el Blood
   Moor con un paquete grande de monstruos, el inventario lleno). No se mide el caso promedio.
6. **Y el emulador para lo que sí es bueno:** cargar rápido, probar pantallas, capturar imágenes, correr 50
   versiones del binario en una tarde. Nada de eso necesita el hardware.

## Sobre "240p si es necesario"

Bajá la resolución antes que la fluidez, sí: 320×240 dibuja **¼ de los píxeles** de 640×480 y es lo que usaban
los juegos de la época en la tele de tubo. Pero ojo con dos cosas: (a) el ancho de banda de VRAM baja, pero el
costo de CPU por sprite **no** baja (los sprites se dibujan igual), así que la resolución no salva un juego
limitado por CPU; (b) el texto del juego (menús, inventario) en 320×240 y con la UI de 800×600 del fork hay que
rehacerlo o escalarlo: la UI del D2 original es de 640×480, así que el camino natural es **640×480** con la UI a
escala 1 y 320×240 como modo de emergencia.

## Ficha de medición propuesta (lo que se anota en cada corrida)

| Dato | Dónde | Cómo |
|---|---|---|
| frame update / draw | host y consola | medidor interno en ciclos y µs |
| p95 / peor frame | host y consola | percentiles del medidor |
| pausa máxima de GC | consola | `gc_stats()` de libgodc |
| heap vivo y máximo | consola | `gc_stats()` |
| MB de VRAM en uso | consola | contador del backend PVR |
| sprites/tiles por frame | host y consola | contador del backend |
| RSS | host | `/proc/<pid>/status` (`VmHWM`), como en esta auditoría |

## LO QUE ESTÁ BIEN

1. Los dos límites que más te importan (RAM y VRAM) **son exactos en el emulador**: no hay lugar para que la
   consola te sorprenda por memoria, si medís en el emulador con el hardware correcto emulado.
2. Tenés las dos consolas con medios de carga (GDEMU, HDD): la validación real no depende de nadie más.
3. El repo ya trae los ganchos de instrumentación (`OD2_AUTOPERF`, pprof en el host): extenderlos a la consola es
   agregar el backend, no inventar el sistema.

## LO QUE ENTORPECE

1. La velocidad real del SH-4 **no se puede estimar** sin la consola: hay que aceptar que el primer número serio
   llega con hardware en mano.
2. Sin depurador en la consola, un bug que solo aparece en hardware (coherencia de caché, DMA) se caza a
   `println()`: de ahí la insistencia en `dcache_flush_range` y en probar en consola seguido.
3. El emulador puede esconder bugs de *demasiado rápido* (código que en consola corre 3× menos): cualquier
   decisión de diseño tomada mirando solo el emulador es sospechosa.

## NO VERIFICADO

* No corrí ningún emulador de Dreamcast ni de PS2 en este host todavía (no están instalados): lo de arriba es
  cómo funcionan esas herramientas, no una medición mía de ellas.
* No verifiqué el estado actual de la precisión de Flycast (versión 2026) ni sus modos de intérprete/perfilador:
  hay que confirmarlo al instalar.
* No medí el presupuesto real de ciclos del motor: no existe todavía un backend de consola que lo mida.

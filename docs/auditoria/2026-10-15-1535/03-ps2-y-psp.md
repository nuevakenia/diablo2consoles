# 03 — PlayStation 2 y PSP: por qué no son el primer paso

## El veredicto corto

**No empezar por PS2 ni por PSP.** No es por el hardware (el PS2 tiene el doble de RAM que el Dreamcast y sería
más cómodo para el juego): es porque **no existe runtime de Go para ellos**. El Dreamcast tiene `libgodc`
publicado, con ejemplos y límites escritos. El PS2 tiene un experimento de blog. Eso, en la práctica, es la
diferencia entre "portear un motor" y "portear un lenguaje".

Y no es un detalle que se arregle después: si arrancamos por PS2, el primer año del proyecto se va en el
runtime, no en el juego. Si arrancamos por Dreamcast, el runtime ya está hecho y el primer mes se va en el juego.

## Hardware, en números

| | PlayStation 2 | PSP | Dreamcast (referencia) |
|---|---|---|---|
| CPU | EE = MIPS R5900 @ 294 MHz (MIPS III/IV + instrucciones propias), + VU0/VU1 | MIPS Allegrex (tipo R4000) a 333 MHz | SH-4 @ 200 MHz |
| FPU | **sin dobles** (mismo problema que el SH-4, agravado) | simple | simple (`-m4-single`) |
| RAM | **32 MB** + 4 MB eDRAM en el GS | 32 MB (PSP-2000: 64 MB) | 16 MB + 8 MB VRAM |
| Gráficos | GS: rasterizador sin shaders, **no puede leer el framebuffer**, 4 MB de eDRAM | GE: sin shaders, con soporte GL1 vía `pspgl` | PowerVR2 con PAL8 + CLUT y blending |
| Almacenamiento | DVD (4,7 GB); acá **HDD**, que es lo mejor para streamear | Memory Stick | GD-ROM 1,2 GB / **GDEMU + SD** |
| Toolchain | `ps2dev`/`ps2sdk` (libs compiladas a **N32 ABI**, MIPS III) | `psptoolchain` + `pspgl`/SDL | KallistiOS 2.2.1 + libgodc (Go) |
| Estado de Go | **experimental**: un post de blog (rgsilva.com, marzo 2025) con TinyGo + LLVM mipsel/MIPS3 N32 enlazando contra las libs prebuilt de `ps2sdk`, probado **solo en emulador**, sin código publicado al momento del post | mismo problema que PS2 | **runtime publicado con ejemplos** |

## Lo que dicen las fuentes sobre Go en PS2

El trabajo conocido (Ricardo, "Golang on the PlayStation 2") describe exactamente el problema:

* Go no soporta el PS2 y **"parece requerir un sistema operativo detrás"**; el PS2 no lo tiene (salvo PS2 Linux).
* Recurrió a **TinyGo** (Go → LLVM IR → código del target) porque LLVM 19 soporta MIPS III — pero **no el R5900
  directamente**.
* Las librerías de `ps2dev` están compiladas a **MIPS III N32** (32 bits con instrucciones de 64): el código
  propio tiene que apuntar al mismo ABI o el enlazador no une. Ahí se le complicó todo (intentos con mipsel con
  CPU mips3, mips64el que falla la verificación de LLVM, clang que se niega).
* Y aclara que **todo eso corrió en emulador**.

Traducido: para PS2 habría que escribir el equivalente a `libgodc` sobre `ps2sdk` (runtime de Go con GC y
goroutines para MIPS R5900, sin MMU de usuario, con ABI N32 y sin FPU doble), o adaptar TinyGo al target. Es un
**proyecto de investigación con resultado incierto**, del mismo tamaño que el port mismo. No es el lugar donde
poner la primera ficha.

## Lo que un futuro port a PS2 va a aprovechar (y por eso se hace después)

Si el trabajo de Dreamcast se hace bien, el PS2 hereda casi todo:

* **Los datos horneados** (niveles generados, tablas empaquetadas, arte paletizado, audio convertido) son
  independientes de la consola. El pipeline de horneado (F3 del plan) es reutilizable tal cual.
* **Los backends de consola** se escriben contra interfaces propias (`Renderer`, `Surface`, `InputManager`,
  audio), así que el backend de PS2 es un backend nuevo, no un rediseño.
* **El endurecimiento del motor** (cachés chicas, cero asignación por frame, nada de `float64` en caminos
  calientes) sirve igual, y en PS2 con 32 MB es más holgado: el PS2 es exactamente la RAM mínima que pedía el
  juego original (32 MB).
* Lo que **no** hereda es el runtime: eso se hace de nuevo. Y ahí conviene decidir entonces entre gccgo mips-elf
  (mismo enfoque que libgodc) o TinyGo (el enfoque del blog), con un spike propio antes de comprometer nada.

## PSP (tercero, y con una advertencia)

El PSP es el mismo problema de runtime que el PS2 (MIPS sin runtime de Go) más el problema de memoria del
Dreamcast en un dispositivo de 32-64 MB, con `pspgl` (subconjunto de OpenGL 1) y SDL disponibles. Tiene sentido
recién cuando exista el runtime de MIPS resuelto para PS2 (comparte arquitectura y mucho trabajo) y el motor ya
esté reducido. Como mucho, en el corto plazo sirve para tomar decisiones de diseño (resolución, reducción de
arte, esquema de input sin mouse): no para escribir código.

## Riesgos específicos de ir directo al PS2 (si el dueño decidiera eso igual)

1. **El runtime es la incógnita y pesa más que el port.** Semanas o meses con resultado binario: funciona o no.
2. **El GS no lee el framebuffer** y no tiene shaders: el modelo de dibujo del motor (superficies, `PushBrightness`,
   `PushSaturation`, `ColorM` de Ebiten, capturas de pantalla) no se traduce de forma directa, igual que en DC,
   pero con un pipeline (DMA/VIF/GS) mucho más hostil para un juego de sprites.
3. **DVD/HDD y streaming** hay que diseñarlos igual que en DC.
4. **El emulador es menos confiable en timing** que Flycast para este tipo de carga (ver informe 05): la verdad
   de performance requiere la consola real temprano.

## LO QUE ESTÁ BIEN (del PS2/PSP como destino)

1. RAM: 32 MB (y 64 MB en PSP-2000) contra los 32 MB mínimos del juego original. En memoria, es la consola
   cómoda; el Dreamcast es la incómoda.
2. Almacenamiento: DVD/HDD/Memory Stick de sobra para los ~900 MB de datos descomprimidos de los cuatro MPQ de
   juego (d2data + patch_D2 + d2exp + d2char) y también para el resto (música, voces y video).
3. Público: el PS2 vendió 155 millones de consolas; un port a PS2 tiene muchísima más llegada que uno a
   Dreamcast. Como objetivo final, es el objetivo grande.
4. Comunidad y herramientas maduras (ps2sdk con SDL2/gsKit, PSP con pspgl/SDL) para todo lo que no sea Go.

## LO QUE ENTORPECE

1. **Sin runtime de Go**: no hay camino publicado. Lo único existente es un experimento en emulador que
   terminó, según su propio autor, enredado en el ABI N32/MIPS3 vs mips64el.
2. **El PS2 exige decidir entre dos rutas inciertas** (gccgo mips-elf + ps2sdk con runtime propio, o TinyGo
   adaptado al target) antes de saber si el motor siquiera compila.
3. **GS sin lectura de framebuffer ni shaders**: el backend de PS2 es, de los tres, el más lejano al modelo de
   superficies del motor.
4. **TinyGo no es un reemplazo transparente de Go**: soporta un subconjunto (reflexión limitada, sin
   `encoding/json` cómodo, ecosistema de librerías distinto). El motor usa 287.443 líneas de Go "normal".

## NO VERIFICADO

* No probé ni instalé `ps2sdk`, TinyGo ni el experimento del blog. Todo lo de PS2 viene de la fuente citada
  (post de marzo 2025) y de la documentación de `ps2dev`: no hay una prueba propia ni compilación mía.
* No verifiqué si el post tiene continuación ("part 2" con código liberado): al momento del post, el autor decía
  que publicaría el código después y que había corrido solo en emulador.
* No medí el costo real de portar el runtime a MIPS R5900 (cuántas operaciones del runtime de Go no tienen
  equivalente, cuánto pesa el N32, etc.).
* No revisé el estado de `SDL2`/`gsKit` para PS2 en 2026: si eso avanzó, el backend gráfico de PS2 sería más
  fácil de lo que estimo acá.

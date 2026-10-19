# DC-002 — estado al frenar (2026-10-19)

Frenado por pedido de Daniel para no gastar mas tokens. **Nada de este proyecto queda corriendo en segundo
plano:** los tres cronjobs estan PAUSADOS (no borrados), no hay agentes ni Flycast vivos, el lock de la corrida
esta libre y el repo esta commiteado y limpio. Se reanuda cuando Daniel diga.

## 1. Lo que YA funciona en la Dreamcast (medido, no relatado)

| hito | evidencia |
|---|---|
| el runtime de Go en SH-4 | 142/142 paquetes del motor compilan (DC-002.8) |
| el motor dibuja por el PVR | DC-002.10 |
| el control de la Dreamcast mueve cosas | DC-002.11 |
| los MPQ del disco se leen | DC-002.27: `d2data.mpq` crc32 identico al del PC |
| el disco se lista | DC-002.44: `/cd/juego` ve los 3 archivos |
| un asset REAL pintado | DC-002.35: el DC6 del menu sale identico (8/8 trozos) |
| **EL MENU PRINCIPAL** | DC-002.36: 10 de 11 texturas, 281 quads, la navegacion se ve (6.059 y 11.921 px) |
| **EL ACTO 1** | DC-002.37: el mapa desde los `.ds1` del disco, identidad **98,80%** contra el host, 59,8 cuadros/s |
| **EL PERSONAJE en el mapa** | DC-002.38: se mueve con el control |
| **UN ENEMIGO** | DC-002.50 (frenada a mitad): el bicho del acto 1, vida HP=3, golpe por el boton Y |
| **los TILES arreglados** | DC-002.48: los huecos dejaron de ser tiles rotos (cerrada el 19/10) |
| **la ANIMACION** | DC-002.49: el personaje anima (cerrada el 19/10) |

## 2. Lo que Daniel vio con sus ojos (el instrumento que mas nos enseno)

* **los tiles**: primero ROTOS por dentro (19,82% de pixeles negros, bandas dentro de cada tile), despues
  BIEN ordenados y FALTANDO (44,14% de negro, huecos con forma de rombo isometrico). Es un avance neto: un
  hueco limpio es un tile que no se compone; un tile corrido es un tile que se compone mal. **Cerrado en
  DC-002.48.**
* **el personaje se movia SIN ANIMACION**. **Cerrado en DC-002.49.**
* lo que queda es la DIRECCION del movimiento (el giro): DC-002.49b, en cola.

Las dos cosas son el mismo tipo de defecto que el proyecto viene a cazar: **un instrumento que mide otra cosa
de la que ve el ojo**. El porcentaje de negro, por ejemplo, hay que medirlo DENTRO del canvas del juego: medido
sobre la captura entera, el escritorio y el marco entran en la cuenta y el numero no significa nada.

## 3. Lo que falta (el tablero, en orden)

    1. DC-002.50   GOLPEAR ENEMIGOS        BLOCKED  el bicho ya se ve y el golpe por el boton Y ya anda en el
                                                    POC (commit 68ad6e88); falta cerrarla
    2. DC-002.49b  la DIRECCION (el giro)  TODO     depende de la 50; el script correr-giro-249b.sh ya esta
                                                    commiteado

Y el **"breaking news!"** que Daniel pidio: de las tres que lo disparan, **dos ya estan cerradas** (los tiles
DC-002.48 y la animacion DC-002.49). Falta solo GOLPEAR ENEMIGOS (la 50).

## 4. Como se reanuda

    hermes cron resume ed0ac6f0bd05     # la corrida autonoma (cada 45 min)
    hermes cron resume 9d37195a2213     # el aviso de avance por Telegram
    hermes cron resume 887cc41faa8c     # el BREAKING NEWS

Los tres quedaron **pausados, no borrados**. El lanzador ya dispara la corrida en segundo plano (asi el job del
cron no muere por su tope de 1 hora), y el vigilante devuelve solo las tarjetas que un worker caido deje en
`blocked` (nunca la de Daniel, que va sin assignee).

**OJO CON EL DISPATCHER (medido al frenar).** Ademas del cron, el **gateway de Hermes** tiene un dispatcher que
toma por su cuenta las tarjetas en `ready` y les lanza un worker. Al frenar, la DC-002.50 volvio a `running`
sola por eso (no por el cron, que ya estaba pausado). La forma de que no las tome sin apagar el gateway -- que
lo comparten los otros proyectos -- es dejarlas en **`blocked`**, que es como quedaron. Para reanudar hay que
sacarlas a mano:

    hermes kanban --board dreamcast assign t_4c644700 default
    hermes kanban --board dreamcast unblock t_4c644700

Y eso es a proposito: **nadie reanuda este proyecto sin que Daniel lo diga.**

**EL `libgodc` (vive fuera del repo, y es importante).** `~/dreamcast/libgodc` tiene los fixes aplicados por
los scripts de `scripts/console/` **sin commitear** (kos/Makefile, runtime/gc_copy.c, gc_heap.c, gc_runtime.c,
go-main.c, godc_config.h, interface_dreamcast.c, kos_startup.c). No es un olvido: los scripts son idempotentes
y los re-aplican. Pero **no se le puede hacer `git checkout --` ni `git reset` a ese arbol**, o se pierden los
fixes y el motor deja de andar.

## 5. Los numeros del freno

* **78 tarjetas cerradas**, 0 ready, 1 todo (la 49b), 2 blocked: la 50 (el combate, a medio camino) y la de
  Daniel (aceptar el POC en la GDEMU real).
* Los dos ultimos commits: `68ad6e88` (el bicho del acto 1, HP=3, el golpe por el boton Y) y `f99c85f7` (el
  trabajo del giro a medias, commiteado para no perderlo).
* El binario ocupa el **83,4%** de la RAM de la consola: 13.998.980 de 16.777.216 bytes, ya con las palancas
  aplicadas (la tabla de `objectLookups` y el `.eh_frame` fuera del binario).
* La pared de hoy: el **semiespacio del GC al 98,1%** (514.832 de 524.288 bytes) y el **monton de C a 42.120
  libres**, con 1.114 recolecciones en UNA sola tabla.

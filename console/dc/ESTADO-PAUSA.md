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
| **UN ENEMIGO** | DC-002.50 (en curso): el bicho del acto 1, vida HP=3, golpe por el boton Y |

## 2. Lo que Daniel vio con sus ojos (el instrumento que mas nos enseno)

* **los tiles**: primero ROTOS por dentro (19,82% de pixeles negros, bandas dentro de cada tile), despues
  BIEN ordenados y FALTANDO (44,14% de negro, huecos con forma de rombo isometrico). Es un avance neto: un
  hueco limpio es un tile que no se compone; un tile corrido es un tile que se compone mal.
* **el personaje se mueve SIN ANIMACION** (el giro). Eso es lo que ataca DC-002.49b.

Las dos cosas son el mismo tipo de defecto que el proyecto viene a cazar: **un instrumento que mide otra cosa
de la que ve el ojo**. El porcentaje de negro, por ejemplo, hay que medirlo DENTRO del canvas del juego: medido
sobre la captura entera, el escritorio y el marco entran en la cuenta y el numero no significa nada.

## 3. Lo que falta (el tablero, en orden)

    1. DC-002.50   GOLPEAR ENEMIGOS        ready   el bicho ya se ve y el golpe anda en el POC; falta cerrarla
    2. DC-002.49b  la DIRECCION (el giro)  todo    depende de la 50; el script correr-giro-249b.sh ya esta
    3. DC-002.48   los TILES que faltan    ready   los rombos negros; la medicion a replicar es la del cuadro
                                                   entero de la ventana, no la del POC (que da verde)

Y el **"breaking news!"** que Daniel pidio: sale cuando esten la animacion (49b) y golpear enemigos (50).

## 4. Como se reanuda

    hermes cron resume ed0ac6f0bd05     # la corrida autonoma (cada 45 min)
    hermes cron resume 9d37195a2213     # el aviso de avance por Telegram
    hermes cron resume 887cc41faa8c     # el BREAKING NEWS

Los tres quedaron **pausados, no borrados**. El lanzador ya dispara la corrida en segundo plano (asi el job del
cron no muere por su tope de 1 hora), y el vigilante devuelve solo las tarjetas que un worker caido deje en
`blocked` (nunca la de Daniel, que va sin assignee).

## 5. Los numeros del freno

* **78 tarjetas cerradas**, 1 ready (la 50), 1 todo (la 49b), 1 blocked (la de Daniel: aceptar el POC en la
  GDEMU real).
* Los dos ultimos commits: `68ad6e88` (el bicho del acto 1, HP=3, el golpe por el boton Y) y `f99c85f7` (el
  trabajo del giro a medias, commiteado para no perderlo).
* El binario ocupa el **83,4%** de la RAM de la consola: 13.998.980 de 16.777.216 bytes, ya con las palancas
  aplicadas (la tabla de `objectLookups` y el `.eh_frame` fuera del binario).
* La pared de hoy: el **semiespacio del GC al 98,1%** (514.832 de 524.288 bytes) y el **monton de C a 42.120
  libres**, con 1.114 recolecciones en UNA sola tabla.

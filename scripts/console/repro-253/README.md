# repro-253 — la escalera que acota el bloqueo del arranque del PVR

Nacio el 2026-10-25, despues de una jornada en la que cada cambio de codigo movia el punto de muerte
(Heisenbug). En vez de seguir tocando el POC a ciegas, se hicieron tres programas chiquitos que se
compilan y corren en Flycast en ~3 minutos cada uno.

| eslabon   | que hace                                                        | resultado medido |
|-----------|-----------------------------------------------------------------|------------------|
| `kos`     | programa de KOS PELADO (sin Go): `pvr_init_defaults()`, 64 KB de VRAM, 60 escenas vacias | **pasa** (sale limpio) |
| `minimo`  | Go sobre libgodc que llama al MISMO shim del POC y pinta 60 cuadros | **pasa** |
| `churn`   | como `minimo` pero antes pide 1 MB, 40 temporales de 96 KB y 2 GC | **pasa** |

Conclusion: KOS sano, Flycast sano, libgodc sano, el shim sano, el estado del monton de Go no alcanza para
romperlo. Lo que rompe es algo que el POC hace **antes** de encender el video (`pvr_init_defaults` muere
DENTRO con `BESTIA=1`), o el tamano/forma de su binario.

El siguiente eslabon a construir es uno que abra el `d2data.mpq`, lea el DS1 y componga el nivel. Con eso se
parte el camino del POC en dos y se sigue bajando.

Correr: `scripts/console/repro-253/correr.sh [kos|minimo|churn|todos]`

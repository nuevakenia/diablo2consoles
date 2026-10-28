# repro-253 — la escalera que acota el bloqueo del arranque del PVR

Nacio el 2026-10-25, despues de una jornada en la que cada cambio de codigo movia el punto de muerte
(Heisenbug). En vez de seguir tocando el POC a ciegas, se hicieron programas chiquitos que se compilan y
corren en Flycast en menos de un minuto cada uno.

| eslabon   | que hace                                                        | resultado medido |
|-----------|-----------------------------------------------------------------|------------------|
| `kos`     | programa de KOS PELADO (sin Go): `pvr_init_defaults()`, 64 KB de VRAM, 60 escenas vacias | **pasa** (sale limpio) |
| `minimo`  | Go sobre libgodc que llama al MISMO shim del POC y pinta 60 cuadros | **pasa** |
| `churn`   | como `minimo` pero antes pide 1 MB, 40 temporales de 96 KB y 2 GC | **pasa** |
| `nivel`   | el TRABAJO PREVIO AL VIDEO del POC: abre `d2data.mpq`, lee el DS1 del acto 1, compone el nivel y **recien despues** enciende el PVR | **pasa y PINTA** (2026-10-27, ver abajo) |

## `nivel` (DC-002.58) — el eslabon que faltaba

Los tres primeros eslabones dejaron una sola cosa en pie: KOS, Flycast, libgodc y el estado del monton estan
sanos. Faltaba partir en dos el camino del POC. Este eslabon hace **exactamente el trabajo previo al video**
del POC con un binario de un tercio del tamano:

- abre `/cd/JUEGO/D2DATA.MPQ` por ISO9660 con `d2mpq` (el mismo paquete del motor), lee el DS1 del acto 1
  (`townN1.ds1`) y compone el nivel con `nivel.go`, **copiado tal cual** del POC (`correr.sh` verifica con
  `cmp` que sea el mismo archivo, igual que `build-monstruo-250.sh`);
- carga los 8 DT1 de a uno y devuelve al monton de C los que pasan de 64 KB (`d2cFreeExternal`);
- imprime las MISMAS marcas que el POC (`MPQ-ABRIR ms=`, `DS1 version= act= w= h= objetos=`, `COMPUESTO
  pisos= / paredes=`, `LIENZO pixelesNoVacios= de`, `MEMORIA libre=`), con el prefijo `HITO258` en vez de
  `HITO238` para no confundir los dos logs;
- **solo despues** enciende el PVR (`d2c_pvr_pantalla_init` → `pvr_init_defaults`) y pinta 60 cuadros con una
  marca por cuadro (`HITO258: CUADRO n= k de 60 dibujando=1`), y sigue pintando (un latido cada 60) para que
  la captura tenga algo que ver.

Que el nivel se compone **no se cree: se compara**. Estos son los numeros del 2026-10-27 contra el POC del
disco, y son idénticos:

```
DS1        act=1 w=57 h=41 objetos=41 files=8        (POC: act=1 w=57 h=41 objetos=41)
COMPUESTO  pisos=1982/1982 paredes=330/330           (POC: pisos=1982/1982 paredes=330/330)
LIENZO     pixelesNoVacios=193538 de 262144          (POC: pixelesNoVacios=193538 de 262144)
```

Y el veredicto de "pinta" no es una impresion: `correr.sh nivel` saca la captura de pantalla con
`spectacle -f`, le pide la geometria del area de video al gestor de ventanas (`flycast-adelante.sh geo`, nuevo
subcomando: un recorte fijo NO sirve, medido — en este host la ventana del emulador queda en otra parte y el
recorte fijo medía el fondo de pantalla) y cuenta los pixeles no negros con `pixeles-258.py`:

```
PIXELES: area de video 320x240 en (1400,120) de una captura de 3584x1080
PIXELES: no negros=41816 de 76800 (54.4%), umbral=20% y piso 5000
VEREDICTO: PINTA (el area de video del emulador tiene 41816 pixeles pintados)
```

La evidencia completa (binario con su sha256, las marcas una por una y el veredicto) esta en
`console/dc/evidencia/dc-002.58-escalon-nivel.txt`, y el recorte del area de video del emulador (el Rogue
Encampment: cesped, la pared de piedra, la valla de madera, la carreta) en
`console/dc/evidencia/dc-002.58-escalon-nivel.png`.

**Lo que esto cierra y lo que deja abierto.** El trabajo previo al video esta sano: un binario de 1.320.200 B
hace el mismo camino (MPQ + DS1 + DT1 + compositor + PVR) y llega al cuadro 60. Queda en pie el
**tamano/forma del binario** del POC (1.854.760 B con `BESTIA=1`, medido en DC-002.57) y lo que el POC hace
**ademas** del nivel: el personaje, el bicho y el control. O sea: la escalera ya no acusa al nivel.

## Correr

```
scripts/console/repro-253/correr.sh [kos|minimo|churn|nivel|todos]
```

`nivel` tarda ~1 minuto en total (build ~15 s, ISO con el `d2data.mpq` de 267 MB ~20 s, corrida ~45 s hasta el
cuadro 60). `/tmp` es tmpfs de 16 GB y cada build con ISO deja ~300 MB: `correr.sh` borra su propio
directorio (`/tmp/repro253-nivel`) antes de empezar.

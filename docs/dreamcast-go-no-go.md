# Dreamcast: los tres números del go/no-go (DC-001.5), medidos

Los tres números que el plan por fases (`docs/auditoria/2026-10-15-1535/04-plan-por-fases.md`, F1) pedía
para decidir si el port se hace o se corta. Están **medidos en el emulador** con un POC del repo
(`console/dc/poc/motor-300`), no estimados. Evidencia cruda y comandos:
`console/dc/evidencia/dc-001.5-go-no-go.txt`.

| número | medido | contra qué se compara |
|---|---|---|
| (a) el binario cargado | **1.390.948 bytes** (1,33 MB) del ELF cargado en los 16 MB; el `.elf` en disco pesa 7.025.836 (5,49 MB de eso es debug que no se carga) y el `1ST_READ.BIN` 1.207.432 | cuanto de los 16 MB se lleva el código |
| (b) RAM libre con el motor cargado | **10.485.760 bytes (10,00 MB) libres** de los 16 MB, con el PVR andando; y 5.781.192 (5,51 MB) de los 8 MB de VRAM | el plan pedía "~8 MB libres" |
| (c) el frame con 300 sprites paletizados | **60,0 fps sostenidos** (16.594 µs por frame a 640x480 y 16.644 a 320x240); el costo de CPU de someter los 300 sprites es **683 µs por frame** (136.600 ciclos del SH-4, 4,1% de un frame de 60 Hz) | ¿entra un frame de 60 Hz? ¿de 30 Hz? |

El "motor cargado" de (b) y (c) no es un relleno: los bytes entran crudos del MPQ (140.740 del DC6 y 768
de la paleta) y los decodifica el código del motor (`d2dc6.Load` + `DecodeFrame`, `d2dat.Load`) compilado
para SH-4 con `sh-elf-gccgo`, con la textura de 1 byte por pixel y la paleta del motor en el banco 0 del
PVR. Es el mismo camino que cerró DC-002.9.

## El veredicto: GO

- El código se lleva **1,39 MB de los 16** (8,7%) y quedan **10,00 MB libres** con el PVR andando: el
  plan pedía ~8 MB libres para seguir, y hay 10,00.
- Un frame con **300 sprites paletizados entra de sobra en 60 Hz**: la CPU gasta 683 µs de los 16.667 µs
  del frame (4,1%). El presupuesto de video no es el que aprieta.
- La VRAM tampoco: 5,51 MB libres de 8 con la textura arriba.

## Los dos límites que quedan declarados (y por qué)

1. **Los ciclos por frame no se pueden medir en el emulador.** Los contadores de rendimiento del SH-4
   (`PerfCntr*`) devuelven **0** en Flycast (medido en las cuatro fases del POC). Por eso el número que se
   entrega es el **tiempo** medido con el reloj de KOS, y los ciclos que imprime el POC son **derivados**
   (µs × 200 MHz), no medidos con el contador.
2. **Flycast topea a 60 fps.** La fase "a fondo" (sin esperar el vblank) da lo mismo que la sincronizada
   (16.624 y 16.657 µs por frame), así que en el emulador no se puede medir el frame libre: lo que sí se
   puede afirmar es que **no se cae** de 60 Hz.

Los dos se cierran en la consola real, que es exactamente lo que espera la tarjeta **DC-001.2d** (aceptar
el POC en la Dreamcast real con GDEMU): el mismo binario imprime las líneas `MOTOR-300` por el puerto
serie y ahí los contadores del SH-4 sí existen.

## Cómo se reproduce

```bash
POC=motor-300 bash scripts/console/build-motor-dc6.sh /tmp/dcbuild/8b-motor-300
CAPTURE=none bash scripts/console/run-flycast.sh /tmp/dcbuild/8b-motor-300/prog.elf 32 /tmp/dc-run-8b-300d
grep -a 'MOTOR-300' /tmp/dc-run-8b-300d/flycast.log | tr -d '\r'

# (a), en el host:
source scripts/console/env.sh && sh-elf-size -A /tmp/dcbuild/8b-motor-300/prog.elf
```

El DC6 y la paleta salen del MPQ del juego (`D2_DIR=...` si no está en los caminos típicos de Wine) y **no
entran al repo** (`scripts/check_repo_hygiene.sh`).

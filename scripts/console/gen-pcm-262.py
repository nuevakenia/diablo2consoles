#!/usr/bin/env python3
"""DC-002.62 -- LOS DOS PCM DEL PRIMER SONIDO, GENERADOS POR EL BUILD.

Escribe <dir_salida>/sonidos_gen.go con las dos muestras que carga el POC por el AICA:

    id=1  GOLPE    un golpe de mano limpia: un zumbido grave que cae de 150 a 70 Hz con la cola
                   corta, mas un chasquido de ruido en los primeros 8 ms.
    id=2  RECOGER  dos notas ascendentes (DO#6 -> SOL#6) de 110 ms, con ataque de 5 ms.

POR QUE SE GENERAN Y NO SE SACAN DEL JUEGO: los sonidos de Diablo II viven en `d2sfx.mpq`, y ese MPQ
NO viaja en el disco del POC -- la ISO se arma con D2DATA, D2CHAR y PATCH_D2 nada mas (medido en
scripts/console/build-monstruo-250.sh, paso 3/6). Asi que el build hornea dos PCM deterministicos: el
mismo script da el mismo `.go` y el mismo crc32 en cualquier host, que es lo que hace comprobable el
criterio 2 de la tarjeta (tamano y tasa de cada efecto).

LA FORMA ES LA QUE PIDE EL CAMINO DE KOS (`snd_sfx_load_raw_buf`, snd_sfxmgr.c:628):
    - 16 bits CON SIGNO, mono, PCM crudo, little-endian (como el bloque de datos de un WAV);
    - la tasa va aparte (aca 22050 Hz, una de las tres que el AICA reproduce: 44100/22050/11025);
    - la cantidad de muestras de cada efecto es MULTIPLO DE 16 (32 bytes en 16 bits): los samples
      viajan con `spu_memload_sq`, que copia por store queues en bloques de 32 B
      (kernel/arch/dreamcast/hardware/spu.c:65).

Uso:  python3 scripts/console/gen-pcm-262.py <dir_salida> [--silencio]
      El archivo lo deja en <dir_salida>/sonidos_gen.go; --silencio escribe las muestras en cero
      (EL MUTANTE del generador: un PCM valido por tamano y tasa que no suena).
"""

import argparse
import hashlib
import math
import os
import sys
import zlib

# ---- la forma de las dos muestras (la que se informa en el log del POC) ----
TASA = 22050          # Hz
BITS = 16             # con signo, little-endian
CANALES = 1           # mono

MUESTRAS_GOLPE = 2000     # 90,7 ms
MUESTRAS_RECOGER = 2432   # 110,3 ms

# el piso y el techo del entero de 16 bits; se deja un 2 % de aire para que el redondeo no recorte
PICO = 32000


def ruido(i):
    """Ruido blanco deterministico (LCG de Numerical Recipes): mismo host, mismos bytes."""
    x = (i * 1664525 + 1013904223) & 0xFFFFFFFF
    x = (x ^ (x >> 16)) & 0xFFFFFFFF
    x = (x * 2246822519) & 0xFFFFFFFF

    return ((x >> 8) / 0x800000) - 1.0     # -1.0 .. 1.0


def golpe(n, silencio):
    """El golpe: seno grave que cae de 150 a 70 Hz + chasquido de ruido en el ataque."""
    out = bytearray()
    fase = 0.0

    for i in range(n):
        t = i / TASA

        # 150 Hz -> 70 Hz en 90 ms: la frecuencia baja, que es lo que hace el "golpe" y no el "tono"
        f = 150.0 - (80.0 * t / 0.090)
        fase += 2.0 * math.pi * f / TASA

        env_cuerpo = math.exp(-t * 22.0)
        cuerpo = math.sin(fase) * env_cuerpo

        env_ruido = math.exp(-t * 70.0)
        chasquido = ruido(i) * env_ruido

        v = 0.72 * cuerpo + 0.28 * chasquido
        out += entero16(0 if silencio else v)

    return bytes(out)


def recoger(n, silencio):
    """El recoger: dos notas ascendentes (1046,5 Hz y 1568 Hz), ataque corto y cola de 40 ms."""
    out = bytearray()
    fase = 0.0
    ms_nota = 0.055

    for i in range(n):
        t = i / TASA
        f = 1046.5 if t < ms_nota else 1568.0
        fase += 2.0 * math.pi * f / TASA

        ataque = min(1.0, t / 0.005)
        cola = math.exp(-(t - (ms_nota if t >= ms_nota else 0.0)) * 9.0)
        env = ataque * cola

        v = 0.62 * (math.sin(fase) + 0.25 * math.sin(2.0 * fase)) * env
        out += entero16(0 if silencio else v)

    return bytes(out)


def entero16(v):
    """Un float -1..1 a un int16 little-endian."""
    if v > 1.0:
        v = 1.0

    if v < -1.0:
        v = -1.0

    m = int(round(v * PICO))
    if m > 32767:
        m = 32767

    if m < -32768:
        m = -32768

    return (m & 0xFFFF).to_bytes(2, "little")


def linea_go(nombre, datos):
    """La asignacion de Go con los bytes, 16 por linea (legible en un diff, y gccgo lo compila sin drama).

    OJO: es una ASIGNACION y no una `var` -- el bloque va adentro de `func init()`, asi que una `var`
    ahi declararia una variable LOCAL que se queda sin usar y la del paquete quedaria en nil (se paga
    con un POC que no suena y un log que dice "sin PCM").
    """
    filas = []
    for i in range(0, len(datos), 16):
        filas.append("\t\t" + ", ".join("0x%02x" % b for b in datos[i:i + 16]) + ",")

    return "\t%s = []byte{\n%s\n\t}\n" % (nombre, "\n".join(filas))


def main():
    ap = argparse.ArgumentParser(description="genera los dos PCM de DC-002.62 como Go")
    ap.add_argument("salida", help="directorio donde dejar sonidos_gen.go (el gen/ del build)")
    ap.add_argument("--silencio", action="store_true",
                    help="escribe las dos muestras en CERO (el mutante del generador)")
    args = ap.parse_args()

    pcm_golpe = golpe(MUESTRAS_GOLPE, args.silencio)
    pcm_recoger = recoger(MUESTRAS_RECOGER, args.silencio)

    for nombre, datos in (("GOLPE", pcm_golpe), ("RECOGER", pcm_recoger)):
        if len(datos) % 32 != 0:
            print("FALLO: %s no es multiplo de 32 B (%d): spu_memload_sq copia por store queues"
                  % (nombre, len(datos)), file=sys.stderr)

            return 1

    os.makedirs(args.salida, exist_ok=True)

    ruta = os.path.join(args.salida, "sonidos_gen.go")
    with open(ruta, "w", encoding="utf-8") as f:
        f.write("//go:build dreamcast\n\n")
        f.write("package main\n\n")
        f.write("// GENERADO POR scripts/console/gen-pcm-262.py -- NO SE EDITA A MANO (DC-002.62).\n")
        f.write("//\n")
        f.write("// Los dos PCM del primer sonido del POC, en la forma del camino de KOS\n")
        f.write("// (snd_sfx_load_raw_buf: 16 bits con signo, mono, little-endian) a %d Hz.\n" % TASA)
        f.write("// Muestras: GOLPE=%d, RECOGER=%d (las dos multiplo de 16, o sea de 32 B).\n" % (MUESTRAS_GOLPE, MUESTRAS_RECOGER))
        f.write("// crc32: GOLPE=0x%08x, RECOGER=0x%08x.\n" % (zlib.crc32(pcm_golpe), zlib.crc32(pcm_recoger)))
        f.write("// El mutante del generador (--silencio) escribe CERO en las dos: mismo tamano y tasa.\n\n")
        f.write("func init() {\n")
        f.write(linea_go("pcmGolpe", pcm_golpe))
        f.write("\n")
        f.write(linea_go("pcmRecoger", pcm_recoger))
        f.write("\n")
        f.write("\tpcmTasa = %d\n" % TASA)
        f.write("\tpcmBits = %d\n" % BITS)
        f.write("\tpcmCanales = %d\n" % CANALES)
        f.write("\tpcmMuestrasGolpe = %d\n" % MUESTRAS_GOLPE)
        f.write("\tpcmMuestrasRecoger = %d\n" % MUESTRAS_RECOGER)
        f.write("}\n")

    for nombre, datos in (("GOLPE", pcm_golpe), ("RECOGER", pcm_recoger)):
        pico = 0
        for i in range(0, len(datos), 2):
            m = int.from_bytes(datos[i:i + 2], "little", signed=True)
            pico = max(pico, abs(m))

        print("PCM %-8s bytes=%-5d muestras=%-5d tasa=%d bits=%d canales=%d pico=%d "
              "crc32=0x%08x sha256=%s"
              % (nombre, len(datos), len(datos) // 2, TASA, BITS, CANALES, pico,
                 zlib.crc32(datos), hashlib.sha256(datos).hexdigest()[:16]))

    print("archivo: %s (%d bytes)" % (ruta, os.path.getsize(ruta)))

    return 0


if __name__ == "__main__":
    sys.exit(main())

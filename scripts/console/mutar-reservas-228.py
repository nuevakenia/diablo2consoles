#!/usr/bin/env python3
"""DC-002.28: los MUTANTES de la lectura de las tablas del MPQ por trozos.

QUE VIGILA EL CORPUS (y por que estos dos mutantes)
    El arreglo de DC-002.28 es que las tablas del MPQ se lean y descifren POR TROZOS, sin reservar la tabla
    entera: la de hash de d2data.mpq son 65.536 entradas = 1.048.576 bytes y el pedido de un tiron (el crudo
    y la tabla, dos reservas contiguas de ese tamano) es el que no entra en la Dreamcast (medido: el motor
    muere en `gc_external_alloc: OOM` al abrir ese archivo).

    Un corpus que solo exige "abre los tres MPQ" se pasa por verde el dia que alguien vuelva al pedido unico
    (en el host la RAM sobra: el OOM NO se reproduce en el host). Por eso estan estos dos:

    A. --una-sola-reserva   el trozo pasa a ser la tabla ENTERA (buffers del tamano de la tabla, una sola
                            lectura). Es el defecto exacto de la tarjeta. Lo caza el corpus del host (el
                            pedido mas grande pasa de los 64 KiB del umbral de "objeto grande") Y la consola
                            (vuelve el `gc_external_alloc: OOM` con d2data.mpq).
    B. --trozo-de-128k      el trozo pasa a 8.192 entradas = 128 KiB: un solo numero mas grande que el
                            umbral de 64 KiB de libgodc, con el codigo por trozos intacto. Es el mutante
                            del TOPE: si el host aprobara un trozo de 128 KiB, su control no morderia.

USO
    python3 scripts/console/mutar-reservas-228.py --estado
    python3 scripts/console/mutar-reservas-228.py --una-sola-reserva     # pone el mutante A
    python3 scripts/console/mutar-reservas-228.py --trozo-de-128k        # pone el mutante B
    python3 scripts/console/mutar-reservas-228.py --restaurar            # deja el archivo como estaba

    El arreglo vive en UN archivo (d2common/d2fileformats/d2mpq/crypto.go): los dos mutantes son de ahi.
    El script guarda el archivo sano en /tmp/dc228-mutante/ y de ahi lo restaura, comparando el sha256
    (o sea: si el archivo se edito por otro lado, avisa en vez de pisarlo).
"""
import hashlib
import os
import pathlib
import shutil
import sys

REPO = pathlib.Path(__file__).resolve().parent.parent.parent
CRYPTO = REPO / "d2common/d2fileformats/d2mpq/crypto.go"
BK_DIR = pathlib.Path("/tmp/dc228-mutante")
BK = BK_DIR / "crypto.go.sano"

SANOS = [
    "\tcrudo := make([]byte, tablaTrozoEntradas*4*4)\n\tpalabras := make([]uint32, tablaTrozoEntradas*4)\n",
    "\t\tcuantas := uint32(tablaTrozoEntradas)\n\t\tif restantes < cuantas {\n\t\t\tcuantas = restantes\n\t\t}\n",
    "\t\tleidos, err := io.ReadFull(r, crudo[:cuantas*4*4])\n",
]

UNICA_RESERVA = [
    # los buffers del tamano de la TABLA (dos reservas contiguas, como el camino viejo)
    "\tcrudo := make([]byte, entries*4*4) // MUTANTE DC-002.28: el crudo de la tabla entera\n"
    "\tpalabras := make([]uint32, entries*4) // MUTANTE DC-002.28: la tabla entera\n",
    "\t\tcuantas := restantes // MUTANTE DC-002.28: todo de un tiron\n",
    "\t\tleidos, err := io.ReadFull(r, crudo[:entries*4*4]) // MUTANTE DC-002.28\n",
]

TROZO_128K_VIEJO = "const tablaTrozoEntradas = 1024"
TROZO_128K_NUEVO = "const tablaTrozoEntradas = 8192 // MUTANTE DC-002.28: 128 KiB por trozo"


def sha(p):
    return hashlib.sha256(p.read_bytes()).hexdigest()


def estado():
    t = CRYPTO.read_text()

    tiene_sanos = all(s in t for s in SANOS)
    tiene_unica = "MUTANTE DC-002.28: todo de un tiron" in t
    tiene_128k = TROZO_128K_NUEVO in t

    print("crypto.go sha256=%s" % sha(CRYPTO)[:16])
    print("  el camino por trozos intacto: %s" % ("si" if tiene_sanos else "NO"))
    print("  mutante A (una sola reserva): %s" % ("PUESTO" if tiene_unica else "no"))
    print("  mutante B (trozo de 128 KiB):  %s" % ("PUESTO" if tiene_128k else "no"))

    return 0 if tiene_sanos and not tiene_unica and not tiene_128k else 1


def respaldar():
    BK_DIR.mkdir(parents=True, exist_ok=True)

    if not BK.exists():
        shutil.copy2(CRYPTO, BK)
        print("   respaldo del archivo sano: %s (%s)" % (BK, sha(BK)[:16]))

    return 0


def reemplazar(viejo, nuevo):
    t = CRYPTO.read_text()

    n = t.count(viejo)
    if n != 1:
        sys.exit("ERROR: el texto a reemplazar aparece %d veces (esperaba 1):\n%s" % (n, viejo[:120]))

    CRYPTO.write_text(t.replace(viejo, nuevo, 1))


def restaurar():
    if not BK.exists():
        print("   no hay respaldo en %s: nada que restaurar" % BK)

        return 0

    CRYPTO.write_text(BK.read_text())
    print("   restaurado desde %s: sha256=%s" % (BK, sha(CRYPTO)[:16]))

    return 0


def main():
    argv = sys.argv[1:]

    if not argv or "--estado" in argv:
        return estado()

    if "--restaurar" in argv:
        return restaurar()

    respaldar()

    if "--una-sola-reserva" in argv:
        for viejo, nuevo in zip(SANOS, UNICA_RESERVA):
            reemplazar(viejo, nuevo)

        print("   MUTANTE A puesto: la tabla entera de un tiron (una sola reserva)")
    elif "--trozo-de-128k" in argv:
        reemplazar(TROZO_128K_VIEJO, TROZO_128K_NUEVO)
        print("   MUTANTE B puesto: el trozo pasa a 8.192 entradas (128 KiB)")
    else:
        sys.exit("uso: mutar-reservas-228.py [--estado|--una-sola-reserva|--trozo-de-128k|--restaurar]")

    print("   crypto.go sha256=%s" % sha(CRYPTO)[:16])

    return estado()


if __name__ == "__main__":
    sys.exit(main())

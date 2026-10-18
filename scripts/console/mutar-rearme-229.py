#!/usr/bin/env python3
"""MUTANTE del corpus de DC-002.29 (scripts/console/checar-rearme-229.sh).

QUE ROMPE: la coherencia de la camada. El defecto que vigila el corpus es "un `.gox` del motor
compilado de un estado del arbol DISTINTO al de la camada cambia el binario de una forma que el
arranque no aguanta". El control 1 (host) es el que lo tiene que cazar: recompilar un paquete sin
cambios conserva el `.text` byte a byte. El mutante compila el MISMO paquete con UNA LINEA de mas
(una variable de paquete que gccgo tiene que meter en el `.gox`) y el control 1 tiene que ponerse
ROJO. Si el mutante queda verde, el hueco es del corpus.

NO toca el repo: copia las fuentes del paquete a un directorio temporal y agrega ahi el archivo del
mutante.

Uso: python3 scripts/console/mutar-rearme-229.py [--fuente-tocada] [--camada /tmp/gox-240]
"""
import argparse
import os
import shutil
import subprocess
import sys
import tempfile

AQUI = os.path.dirname(os.path.abspath(__file__))
REPO = os.path.abspath(os.path.join(AQUI, "..", ".."))
P = "d2common/d2fileformats/d2mpq"
PREFIJO = "github.com/OpenDiablo2/OpenDiablo2"

MUTANTE = '''package d2mpq

// mutante de DC-002.29: una linea de mas en el paquete. Tiene que cambiar el `.text` del `.gox`.
var _mutanteDC00229 = [64]byte{}
var _mutanteDC00229Veces = 0

func init() { _mutanteDC00229Veces++ }
'''

ENV = "source %s/env.sh && " % AQUI


def sh(c):
    return subprocess.run(["bash", "-c", ENV + c], capture_output=True, text=True).stdout


def texto(gox):
    return sh("sh-elf-objdump -h '%s' | awk '$2==\".text\"{print $3}'" % gox).strip()


def gox_de(camada, dir_fuentes, mutar):
    """compila dir_fuentes contra una COPIA de la camada y devuelve (dir_camada, gox, .text)"""
    copia = tempfile.mkdtemp(prefix="mutar-229-")
    shutil.rmtree(copia)
    shutil.copytree(camada, copia, symlinks=True)
    for f in ("orden.txt", "orden-motor.txt"):
        ruta = os.path.join(copia, f)
        if os.path.exists(ruta):
            with open(ruta) as fh:
                t = fh.read()
            with open(ruta, "w") as fh:
                fh.write(t.replace(camada, copia))
    gox = os.path.join(copia, PREFIJO, P + ".gox")
    if os.path.exists(gox):
        os.remove(gox)   # OJO: la copia de la camada YA trae el .gox viejo; si no se borra, se mide ese
    salida = sh("sh-elf-gccgo $GODC_GOFLAGS -I '%s' -fgo-pkgpath='%s/%s' -c %s/*.go -o '%s' 2>&1"
                % (copia, PREFIJO, P, dir_fuentes, gox))
    if not os.path.exists(gox):
        print("   el .gox no compilo:\n%s" % salida[-600:])
        return copia, None, None
    return copia, gox, texto(gox)


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--fuente-tocada", action="store_true", help="agrega la linea del mutante")
    ap.add_argument("--camada", default=os.environ.get("REF", "/tmp/gox-240"))
    a = ap.parse_args()
    camada = a.camada
    ref_gox = os.path.join(camada, PREFIJO, P + ".gox")
    if not os.path.exists(ref_gox):
        print("no encuentro la camada en %s" % camada)
        return 2
    t_camada = texto(ref_gox)
    print("== camada %s: d2mpq.gox %d B, .text 0x%s" % (camada, os.path.getsize(ref_gox), t_camada))

    fuentes = tempfile.mkdtemp(prefix="mutar-229-src-")
    for f in os.listdir(os.path.join(REPO, P)):
        if f.endswith(".go") and not f.endswith("_test.go"):
            shutil.copy2(os.path.join(REPO, P, f), fuentes)
    if a.fuente_tocada:
        with open(os.path.join(fuentes, "zz-mutante-229.go"), "w") as fh:
            fh.write(MUTANTE)
    print("== fuentes: %s (%s)" % (fuentes, "CON la linea del mutante" if a.fuente_tocada else "tal cual el arbol"))

    copia, gox, t_nuevo = gox_de(camada, fuentes, a.fuente_tocada)
    if gox is None:
        return 1
    print("== recompilado: d2mpq.gox %d B, .text 0x%s" % (os.path.getsize(gox), t_nuevo))

    if a.fuente_tocada:
        if t_nuevo != t_camada:
            print("== MUTANTE CAZADO: el control 1 se pone ROJO (.text 0x%s != 0x%s)" % (t_nuevo, t_camada))
            rc = 0
        else:
            print("== MUTANTE SOBREVIVIO: el .text no cambio; el HUECO es del corpus")
            rc = 1
    else:
        if t_nuevo == t_camada:
            print("== CONTROL SANO: sin la linea del mutante el .text es identico (0x%s) -> la camada ES del arbol del repo" % t_nuevo)
            rc = 0
        else:
            print("== CONTROL SANO (aviso): el .text del arbol (0x%s) NO es el de la camada (0x%s)."
                  " No es falla del mutante: esta camada es de OTRO estado del arbol, y eso es exactamente"
                  " el estado que mata el arranque (medido). El corpus tiene que correr sobre una camada"
                  " coherente del arbol." % (t_nuevo, t_camada))
            rc = 0
    shutil.rmtree(copia, ignore_errors=True)
    shutil.rmtree(fuentes, ignore_errors=True)
    return rc


if __name__ == "__main__":
    sys.exit(main())

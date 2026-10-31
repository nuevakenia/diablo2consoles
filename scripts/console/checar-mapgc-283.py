#!/usr/bin/env python3
"""Lee la corrida de la sonda `console/dc/poc/map-gc` (DC-002.83) y dictamina: verde o rojo.

QUE MIRA
    La sonda imprime, despues de armAR cuatro mapas y forzar 3 recolecciones del GC, una linea por cama y
    ronda. El criterio de aceptacion de la tarjeta es del caso A (el mapa `map[string][]*T` de 2.556 claves
    armadas del monton, que es la forma del mapa de AnimData.d2):

        A_faltan= 0     en las rondas 1, 2 y 3   (ninguna busqueda por clave se pierde)
        A_clavesRotas= 0 en las rondas 1, 2 y 3   (los bytes de las claves son los que eran)
        A_hc y A_hv iguales a los de la ronda 0   (la huella de claves y de valores no cambia)

    y ademas los controles tienen que seguir enteros: B_bien= 16, C_bien= 255, D_bien= 256 y E bien= 256.

    OJO CON C: el mapa `map[int]int` usa `mC[i] = i*3`, asi que la clave 0 guarda el valor 0, y la sonda
    cuenta el 0 como "no encontrado". Por eso el control sano de C es `C_bien= 255 C_faltan= 1`, no 256.

USO
    python3 scripts/console/checar-mapgc-283.py <log de flycast>
    # y para correrlo de punta a punta:
    bash scripts/console/build-dc.sh console/dc/poc/map-gc /tmp/dcbuild/mapgc
    CAPTURE=none bash scripts/console/run-flycast.sh /tmp/dcbuild/mapgc/prog.elf 20 /tmp/dc-run-mapgc
    python3 scripts/console/checar-mapgc-283.py /tmp/dc-run-mapgc/flycast.log

SALE CON 0 SOLO SI TODAS LAS CONDICIONES SE CUMPLEN (lo demas es ROJO y dice cual fallo).
"""
import re
import sys

RONDA = re.compile(r"SONDA-MAP: ronda= (\d+) (.*)")
CAMPO = re.compile(r"([A-Za-z_]+)[=:] ?(-?\d+)")


def leer(path):
    filas = {}
    orden = {"A_hc": {}, "A_hv": {}, "A_bien": {}, "A_faltan": {}, "A_clavesRotas": {},
             "A_valoresRotas": {}, "B_bien": {}, "C_bien": {}, "C_faltan": {}, "D_bien": {}}
    e_bien = None

    with open(path, "rb") as f:
        texto = f.read().decode("latin-1")

    for linea in texto.replace("\r", "").splitlines():
        if "SONDA-MAP: E (armado despues del GC)" in linea:
            m = re.search(r"bien= (\d+) de (\d+)", linea)
            if m:
                e_bien = (int(m.group(1)), int(m.group(2)))

        m = RONDA.search(linea)
        if not m:
            continue

        ronda = int(m.group(1))
        campos = dict((k, int(v)) for k, v in CAMPO.findall(m.group(2)))

        for k in orden:
            if k in campos:
                orden[k][ronda] = campos[k]

    return orden, e_bien


def main():
    if len(sys.argv) < 2:
        raise SystemExit("uso: checar-mapgc-283.py <log de flycast>")

    orden, e_bien = leer(sys.argv[1])
    fallos = []

    print("== sonda del mapa + GC (DC-002.83)  %s" % sys.argv[1])

    for ronda in (0, 1, 2, 3):
        if ronda not in orden["A_bien"]:
            fallos.append("falta la ronda %d (la corrida murio antes)" % ronda)
            print("   ronda %d: NO HAY LINEA" % ronda)
            continue

        print("   ronda %d: A_bien=%d A_faltan=%d A_clavesRotas=%d A_valoresRotas=%d "
              "B_bien=%s C_bien=%s D_bien=%s" %
              (ronda, orden["A_bien"][ronda], orden["A_faltan"].get(ronda, -1),
               orden["A_clavesRotas"].get(ronda, -1), orden["A_valoresRotas"].get(ronda, -1),
               orden["B_bien"].get(ronda), orden["C_bien"].get(ronda), orden["D_bien"].get(ronda)))

    ref_hc = orden["A_hc"].get(0)
    ref_hv = orden["A_hv"].get(0)

    for ronda in (1, 2, 3):
        if ronda not in orden["A_bien"]:
            continue

        if orden["A_bien"][ronda] != 2556:
            fallos.append("ronda %d: A_bien=%d (esperaba 2556)" % (ronda, orden["A_bien"][ronda]))
        if orden["A_faltan"].get(ronda) != 0:
            fallos.append("ronda %d: A_faltan=%d (esperaba 0)" % (ronda, orden["A_faltan"].get(ronda)))
        if orden["A_clavesRotas"].get(ronda) != 0:
            fallos.append("ronda %d: A_clavesRotas=%d (esperaba 0)" % (ronda, orden["A_clavesRotas"].get(ronda)))
        if ref_hc is not None and orden["A_hc"].get(ronda) != ref_hc:
            fallos.append("ronda %d: la huella de claves cambio (%s != %s)" %
                          (ronda, orden["A_hc"].get(ronda), ref_hc))
        if ref_hv is not None and orden["A_hv"].get(ronda) != ref_hv:
            fallos.append("ronda %d: la huella de valores cambio (%s != %s)" %
                          (ronda, orden["A_hv"].get(ronda), ref_hv))
        if orden["B_bien"].get(ronda) != 16:
            fallos.append("ronda %d: B_bien=%s (control: esperaba 16)" % (ronda, orden["B_bien"].get(ronda)))
        if orden["C_bien"].get(ronda) != 255 or orden["C_faltan"].get(ronda) != 1:
            fallos.append("ronda %d: control C fuera de lo esperado (C_bien=%s C_faltan=%s; la clave 0 "
                          "guarda 0 y la sonda la cuenta como faltante)" %
                          (ronda, orden["C_bien"].get(ronda), orden["C_faltan"].get(ronda)))
        if orden["D_bien"].get(ronda) != 256:
            fallos.append("ronda %d: D_bien=%s (control: esperaba 256)" % (ronda, orden["D_bien"].get(ronda)))

    if e_bien is None:
        fallos.append("falta el control E (mapa armado despues del GC)")
    elif e_bien != (256, 256):
        fallos.append("control E: %s de %s (esperaba 256 de 256)" % e_bien)
    else:
        print("   control E (mapa armado despues del GC): %d de %d" % e_bien)

    if fallos:
        print("VEREDICTO: ROJO")
        for f in fallos:
            print("   - " + f)
        return 1

    print("VEREDICTO: VERDE -- las 2.556 claves del mapa se encuentran despues de 3 recolecciones")
    return 0


if __name__ == "__main__":
    sys.exit(main())

#!/usr/bin/env python3
"""DC-002.49b -- la CADENCIA de los dos ciclos en la corrida del giro (criterio 2).

QUE TIENE QUE PASAR (y que se mide aca)
---------------------------------------
El giro NO puede tocar el reloj del ciclo: TN sigue declarando 128 ms/cuadro y TW 40, los mismos numeros que
midio DC-002.49 (`HITO249: MODO ... msPorCuadro`), y el cuadro de TN se sigue dibujando a 128 ms de mediana.

EL TECHO DEL BUCLE, QUE NO ES UN DEFECTO (medido en DC-002.49 y confirmado hoy)
------------------------------------------------------------------------------
TN clava la cadencia porque su cuadro dura mas que una vuelta del bucle del POC. TW no puede: con 40 ms por
cuadro el bucle (~50 ms por vuelta mientras camina, que es cuando se mueve el personaje) es el que manda, o
sea que la mediana observada queda pegada al tiempo de vuelta y NO al 40 del animdata. DC-002.49 midio 51 ms
de mediana para TW y lo dejo escrito asi (evidencia dc-002.49-anim, seccion de cadencia). Por eso aca el
veredicto de TW es sobre el numero DECLARADO (40, el del animdata) y la mediana observada se informa contra
la referencia de DC-002.49: si empeorara mucho, se veria.

Uso: medir-cadencia-249b.py <corrida | flycast.log> [--tolerancia 0.2] [--referencia-tw 51]
Salida: una linea por modo; exit 0 solo si TN clava su cadencia y los dos relojes declarados son los del
animdata.
"""
import re
import statistics
import sys
from pathlib import Path

RE_ANIM = re.compile(r"HITO249: ANIM modo (\S+) cuadro (\d+) de (\d+) .*?caja (\d+) (\d+) .*?crc32 (\d+) "
                     r"msDelModo (-?\d+) .*?cuadroDelLoop (\d+)")
RE_MODO = re.compile(r"HITO249: MODO (\S+) cuadros (\d+) msPorCuadro (\d+)")

ESPERADO = {"TN": 128, "TW": 40}


def main():
    if len(sys.argv) < 2:
        print(__doc__)
        return 2
    ruta = Path(sys.argv[1])
    if ruta.is_dir():
        ruta = ruta / "flycast.log"

    tolerancia = 0.2
    referencia_tw = 51.0
    args = sys.argv[2:]
    for i, a in enumerate(args):
        if a == "--tolerancia":
            tolerancia = float(args[i + 1])
        elif a == "--referencia-tw":
            referencia_tw = float(args[i + 1])

    declarado, series = {}, {}
    for linea in ruta.read_text(errors="replace").splitlines():
        if (m := RE_MODO.search(linea)):
            declarado[m.group(1)] = (int(m.group(2)), int(m.group(3)))
            continue
        if (m := RE_ANIM.search(linea)):
            series.setdefault(m.group(1), []).append((int(m.group(2)), m.group(6), int(m.group(7))))

    fallas = 0
    print("corrida: %s" % ruta)
    print("%-3s %-9s %-9s %-9s %s" % ("modo", "declarado", "cuadros", "mediana", "veredicto"))
    for modo in sorted(set(list(declarado) + list(series))):
        esp = ESPERADO.get(modo)
        decl = declarado.get(modo, (None, None))
        marcas = series.get(modo, [])
        deltas = [ms1 - ms0 for (k0, c0, ms0), (k1, c1, ms1) in zip(marcas, marcas[1:])
                  if k1 == k0 + 1 and c1 != c0 and ms1 > ms0]
        mediana = statistics.median(deltas) if deltas else None

        if esp is None:
            print("%-3s %-9s %-9s %-9s (modo no medido por esta tarjeta)" % (modo, decl[1], decl[0], mediana))
            continue

        ok_decl = decl[1] == esp
        if modo == "TN":
            ok = ok_decl and mediana is not None and abs(mediana - esp) <= esp * tolerancia
            veredicto = "OK clava el animdata" if ok else "FALLA (declarado %s, mediana %s)" % (decl[1], mediana)
        else:
            ok = ok_decl and mediana is not None and mediana <= referencia_tw * (1 + tolerancia)
            veredicto = ("OK declarado %d; mediana %.0f = techo del bucle (DC-002.49 midio %.0f)"
                         % (esp, mediana, referencia_tw)) if ok else \
                        "FALLA (declarado %s, mediana %s, referencia DC-002.49 %.0f)" % (decl[1], mediana, referencia_tw)
        fallas += 0 if ok else 1
        print("%-3s %-9s %-9s %-9s %s" % (modo, decl[1], decl[0], mediana, veredicto))

    print()
    print("VERDE: el giro no movio el reloj de ningun ciclo" if fallas == 0 else
          "ROJO: %d modo(s) fuera de rango" % fallas)
    return 0 if fallas == 0 else 1


if __name__ == "__main__":
    sys.exit(main())

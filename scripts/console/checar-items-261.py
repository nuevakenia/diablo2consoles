#!/usr/bin/env python3
"""DC-002.61 -- el verificador de la corrida de la consola (lee el log del emulador y da el veredicto).

Uso:  scripts/console/checar-items-261.py <flycast.log> [base:<otro.log>]

Comprueba, sobre el log serie del POC (lo que imprime la Dreamcast por println):
  1. `ITEM suelo= 41` y las 41 lineas de posicion (tipo, id, subtiles del DS1, celda, flags).
  2. que los graficos se hayan cargado del MPQ, con su caja, pixeles y crc.
  3. las recogidas: `ITEM recogido=<nombre> celda=x,y suelo=N inventario=K/4` (cuenta y nombres).
  4. el panel: cada abierto/cerrado con el inventario que tiene (para ver que no se pierde nada).
  5. los pixeles de la pantalla en los cuadros 60 y 240 (`ITEM PANTALLA`).
  6. que NO haya `fatal error`, `Out of memory` ni excepciones del SH-4 en toda la corrida.

Con `base:<otro.log>` compara los pixeles contra el BASELINE (el binario sin items).
"""
import re
import sys


def leer(path):
    with open(path, "r", errors="replace") as f:
        return f.read()


def main():
    if len(sys.argv) < 2:
        print(__doc__)
        return 2

    log = leer(sys.argv[1])
    base = None

    for arg in sys.argv[2:]:
        if arg.startswith("base:"):
            base = leer(arg[5:])

    fallos = []

    def exigir(cond, texto):
        print(("  OK   " if cond else "  FALLA") + " " + texto)
        if not cond:
            fallos.append(texto)

    # 1. el suelo
    posiciones = re.findall(r"ITEM suelo idx= (\d+) tipo= (\d+) id= (\d+) sub= (\d+) (\d+) celda= (\d+) (\d+) flags= (\d+)", log)
    m = re.search(r"ITEM suelo= (\d+) objetos= (\d+) apariciones= (\d+)", log)

    exigir(m is not None, "esta la marca `ITEM suelo= N objetos= X apariciones= Y`")
    if m:
        total, objetos, apariciones = (int(g) for g in m.groups())
        exigir(total == 41, "el suelo tiene 41 registros del DS1 (medido: %d)" % total)
        exigir(len(posiciones) == total,
               "hay una linea de posicion por registro (%d de %d)" % (len(posiciones), total))
        exigir(objetos + apariciones == total,
               "objetos (%d) + apariciones de monstruo (%d) = %d" % (objetos, apariciones, total))
        exigir(len({(p[3], p[4]) for p in posiciones}) == total,
               "las %d posiciones (subtiles del DS1) son distintas" % total)

    # 2. los graficos
    graficos = re.findall(r"ITEM GRAFICO token= (\S+) fuente=(\S+) bytes= (\d+) .*caja= (\d+) (\d+) esquina= (-?\d+) (-?\d+) pixeles= (\d+) crc= (\d+)", log)
    sinGrafico = re.findall(r"ITEM SIN-GRAFICO token= (\S+)", log)
    cargados = re.search(r"ITEM cargados= (\d+) tokens= (\d+) conGrafico= (\d+) marcador= (\d+) enPantalla= (\d+) sinGrafico= (\d+)", log)

    exigir(cargados is not None, "esta la marca `ITEM cargados= ...`")
    if cargados:
        c = [int(g) for g in cargados.groups()]
        exigir(c[3] >= 1, "los items sin recurso usan el MARCADOR de una celda (%d)" % c[3])
        exigir(c[2] >= 1, "al menos un token se dibuja con su grafico del MPQ (%d items)" % c[2])
        exigir(c[4] >= 1, "al menos un item cae DENTRO de la ventana y se pinta (%d)" % c[4])
    exigir(len(graficos) >= 1, "cada grafico cargado informa fuente, bytes, caja, pixeles y crc (%d)" % len(graficos))

    for g in graficos:
        exigir(int(g[7]) > 0, "el grafico del token %s tiene %s pixeles no vacios" % (g[0], g[7]))

    if sinGrafico:
        print("  nota  los tokens sin COF (van con marcador): %s" % ", ".join(sinGrafico))

    # 3. las recogidas
    recogidas = re.findall(r"ITEM recogido=(\S+) celda=(\d+),(\d+) suelo=(\d+) inventario=(\d+)/(\d+)", log)
    print("  nota  recogidas: %d -> %s" % (len(recogidas), [(r[0], r[1] + "," + r[2], r[3], r[4] + "/" + r[5]) for r in recogidas]))

    if recogidas:
        ultimo = recogidas[-1]
        exigir(len(recogidas) >= 3, "se recogieron 3 items seguidos (%d)" % len(recogidas))
        exigir(ultimo[3] == "38" and ultimo[4] == "3" and ultimo[5] == "4",
               "el ultimo dice suelo=38 inventario=3/4 (medido: suelo=%s inventario=%s/%s)" % (ultimo[3], ultimo[4], ultimo[5]))
        nombres = [r[0] for r in recogidas]
        exigir(len(set(nombres)) >= 2, "los nombres salen del objects.txt: %s" % ", ".join(nombres))

    # 4. el panel
    paneles = re.findall(r"ITEM PANEL abierto= (\w+) inventario= (\d+) / (\d+) suelo= (\d+)", log)
    print("  nota  panel: %d aperturas/cierres" % len(paneles))

    if len(paneles) >= 3:
        # el inventario solo puede SUBIR (lo que se recoge se queda): si bajara, abrir o cerrar el panel
        # estaria perdiendo items
        conteo = [int(p[1]) for p in paneles]
        baja = [(i, conteo[i - 1], conteo[i]) for i in range(1, len(conteo)) if conteo[i] < conteo[i - 1]]
        exigir(not baja, "el inventario nunca BAJA entre abrir y cerrar el panel (%d ciclos, valores %s)"
               % (len(paneles), sorted(set(conteo))))
        if recogidas:
            exigir(paneles[-1][1] == recogidas[-1][4],
                   "el panel termina con los mismos %s items que se recogieron" % recogidas[-1][4])

    # 5. los pixeles
    pantallas = re.findall(r"ITEM PANTALLA cuadro= (\d+) pixelesNoVacios= (\d+) de (\d+) huella= (\d+)", log)
    conItems = {int(p[0]): int(p[1]) for p in pantallas}
    exigir(len(conItems) >= 1, "estan las marcas `ITEM PANTALLA` (%d)" % len(pantallas))

    if base is not None:
        pantallasBase = re.findall(r"ITEM PANTALLA cuadro= (\d+) pixelesNoVacios= (\d+) de (\d+) huella= (\d+)", base)
        sinItems = {int(p[0]): int(p[1]) for p in pantallasBase}

        for cuadro in sorted(set(conItems) & set(sinItems)):
            a, b = conItems[cuadro], sinItems[cuadro]
            exigir(a > b, "cuadro %d: con items %d pixeles > baseline %d (diferencia %+d)" % (cuadro, a, b, a - b))

        exigir(len(set(conItems) & set(sinItems)) >= 1, "hay al menos un cuadro comparable contra el baseline")

    # 6. nada se cayo
    for malo in ("fatal error", "Out of memory", "Unhandled exception", "SH4 exception"):
        exigir(malo not in log, "el log no tiene `%s`" % malo)

    print()
    if fallos:
        print("VEREDICTO: ROJO (%d fallos)" % len(fallos))
        return 1

    print("VEREDICTO: VERDE (todo lo que la tarjeta pide esta en el log)")
    return 0


if __name__ == "__main__":
    sys.exit(main())

#!/usr/bin/env python3
"""DC-002.49 -- EL CORPUS DE LA ANIMACION: los controles de los tres criterios, con los numeros.

Entradas (todo sale de los instrumentos del repo, nada a mano):

  --host DIR        el censo del host (censo.txt + pantalla-TN-XX.png / pantalla-TW-XX.png)
  --run-normal DIR  la corrida de Flycast con el ciclo ANDANDO (f*.png + flycast.log)
  --run-mutante DIR la corrida del MUTANTE (CICLO_SORDO=1: el cuadro se queda en 0)
  --ventana L,T,W,H el area cliente del emulador (la mide el script de la corrida; queda en las lineas GEO)

CONTROLES:
  C1  el host compone el ciclo entero: TN 16 cuadros y 16 dibujos DISTINTOS, TW 8 y 8 distintos
  C2  el host: el cuadro 0 y el de la mitad del ciclo son dibujos distintos (pixeles > 0)
  C3  la consola carga los DOS ciclos (TN y TW) y los dibuja
  C4  la consola dibuja TODOS los cuadros del ciclo de TN (0..15), no un subconjunto
  C5  la cadencia de la consola es la del animdata (TN 128 ms por cuadro, +-20 %)
  C6  CRITERIO 1: con el personaje QUIETO, la caja del personaje cambia entre capturas (minimo > 0)
  C7  CRITERIO 2: las capturas muestran VARIOS dibujos distintos (>= 4 grupos distintos de 8 capturas)
  C8  CRITERIO 3 (el mutante): con el ciclo quieto, las capturas del personaje quieto son IDENTICAS (maximo 0)
  C9  el mutante dibuja SIEMPRE el cuadro 0 (si dibujara otros, el 0 de C8 no probaria nada)

Sale 0 si TODOS pasan, 1 si alguno falla: un control en ROJO es el resultado, no un detalle.
"""
import os
import re
import subprocess
import sys

AQUI = os.path.dirname(os.path.abspath(__file__))
MEDIDOR = os.path.join(AQUI, "medir-anim-249.py")

# los numeros medidos en el censo (sonda-anim-249, animdata.d2 + COF) y usados por el POC
MS_POR_CUADRO_TN = 128
CUADROS_TN = 16
CUADROS_TW = 8

resultados = []


def control(nombre, ok, detalle):
    resultados.append((nombre, bool(ok), detalle))
    print("%-4s %-26s %s  %s" % ("OK" if ok else "FALLA", nombre, "", detalle))


def leer(ruta):
    with open(ruta, "r", errors="replace") as f:
        return f.read()


def lineas_censo(host):
    return [l for l in leer(os.path.join(host, "censo.txt")).splitlines() if l.startswith("CICLO249:")]


def campo(linea, clave):
    m = re.search(r"%s=([-0-9.]+)" % re.escape(clave), linea)
    return m.group(1) if m else None


def log_poc(run):
    return leer(os.path.join(run, "flycast.log"))


def marcas_anim(log, modo):
    """(cuadro, msDelModo) de cada marca HITO249: ANIM de un modo."""
    salida = []
    for m in re.finditer(r"HITO249: ANIM modo (\w+) cuadro (\d+) de (\d+).*?msDelModo (-?\d+)", log):
        if m.group(1) == modo:
            salida.append((int(m.group(2)), int(m.group(4))))
    return salida


def medir(run, host, modo, caja, ancla, ventana, capturas):
    cmd = [sys.executable, MEDIDOR, "--host", host, "--modo", modo, "--caja", caja,
           "--ancla", ancla, "--ventana", ventana] + capturas
    salida = subprocess.run(cmd, capture_output=True, text=True, check=False).stdout
    return [l for l in salida.splitlines() if l.startswith("ANIM249")]


def pares_de(lineas):
    for l in lineas:
        if " PARES " in l:
            return int(campo(l, "pares") or -1), int(campo(l, "minimo") or -1), int(campo(l, "maximo") or -1)
    return -1, -1, -1


def vistos_de(lineas):
    for l in lineas:
        if " VISTOS " in l:
            return int(campo(l, "dibujosDistintos") or -1)
    return -1


def main():
    args = dict()
    resto = sys.argv[1:]
    i = 0
    while i < len(resto):
        if resto[i].startswith("--"):
            args[resto[i]] = resto[i + 1]
            i += 2
        else:
            i += 1

    host = args["--host"]
    normal = args["--run-normal"]
    mutante = args["--run-mutante"]
    ventana = args["--ventana"]
    # la caja de cada modo y el ancla del personaje quieto (las imprime el POC y el censo del host)
    cajas = {"TN": args.get("--caja-tn", "-16,-69,26,77"), "TW": args.get("--caja-tw", "-18,-68,31,79")}
    ancla = args.get("--ancla", "320,240")

    censo = lineas_censo(host)

    # ---- C1: el ciclo entero en el host
    for modo, cuadros in (("TN", CUADROS_TN), ("TW", CUADROS_TW)):
        linea = next((l for l in censo if " %s RESUMEN " % modo in l), None)
        if linea is None:
            control("C1 %s ciclo host" % modo, False, "no hay linea RESUMEN en el censo")
            continue
        cof = int(campo(linea, "cuadrosCOF") or -1)
        comp = int(campo(linea, "cuadrosCompuestos") or -1)
        dist = int(campo(linea, "distintos") or -1)
        control("C1 %s ciclo host" % modo, cof == cuadros and comp == cuadros and dist == cuadros,
                "cuadrosCOF=%d compuestos=%d distintos=%d (esperado %d/%d/%d)" % (cof, comp, dist, cuadros, cuadros, cuadros))

    # ---- C2: el cuadro 0 contra el de la mitad del ciclo
    for modo in ("TN", "TW"):
        linea = next((l for l in censo if " %s MITAD " % modo in l), None)
        px = int(campo(linea, "pixelesCambiados") or -1) if linea else -1
        control("C2 %s mitad del ciclo" % modo, px > 0, "pixelesCambiados=%d caja=%s" % (
            px, (re.search(r"cajaDelCambio=(\S+ \S+)", linea).group(1) if linea else "-")))

    # ---- C3: la consola cargo y dibujo los dos ciclos
    log = log_poc(normal)
    cargados = re.search(r"HITO249: CICLOS cargados (\d+)", log)
    modos = set(re.findall(r"HITO249: MODO (\w+) ", log))
    control("C3 la consola carga los ciclos", cargados is not None and int(cargados.group(1)) == 2 and len(modos) >= 1,
            "cargados=%s modosDibujados=%s" % (cargados.group(1) if cargados else "?", ",".join(sorted(modos)) or "-"))

    # ---- C4: todos los cuadros del ciclo de TN dibujados por el POC
    marcas_tn = marcas_anim(log, "TN")
    cuadros_dibujados = sorted(set(c for c, _ in marcas_tn))
    control("C4 TN cuadros dibujados", len(cuadros_dibujados) == CUADROS_TN,
            "distintos=%d de %d (%s)" % (len(cuadros_dibujados), CUADROS_TN,
                                         ",".join(str(c) for c in cuadros_dibujados)))

    # ---- C5: la cadencia (ms entre cambios de cuadro) contra el animdata
    deltas = [b[1] - a[1] for a, b in zip(marcas_tn, marcas_tn[1:]) if b[1] > a[1]]
    if deltas:
        deltas.sort()
        mediana = deltas[len(deltas) // 2]
        control("C5 cadencia de TN", abs(mediana - MS_POR_CUADRO_TN) <= MS_POR_CUADRO_TN * 0.2,
                "mediana=%d ms/cuadro (el animdata: %d; n=%d)" % (mediana, MS_POR_CUADRO_TN, len(deltas)))
    else:
        control("C5 cadencia de TN", False, "no hay dos marcas de TN con reloj")

    # ---- C6 y C7: las capturas del personaje QUIETO (el ciclo andando)
    capturas = sorted(os.path.join(normal, f) for f in os.listdir(normal) if f.startswith("f") and f.endswith(".png"))
    lineas = medir(normal, host, "TN", cajas["TN"], ancla, ventana, capturas)
    pares, minimo, maximo = pares_de(lineas)
    vistos = vistos_de(lineas)
    control("C6 criterio 1: cambia", pares > 0 and minimo > 0,
            "capturas=%d pares=%d pixelesCambiados min=%d max=%d (personaje QUIETO)" % (len(capturas), pares, minimo, maximo))
    control("C7 criterio 2: se ven", vistos >= 4, "dibujosDistintos=%d de %d capturas" % (vistos, len(capturas)))

    # ---- C8 y C9: el MUTANTE
    log_mut = log_poc(mutante)
    capturas_mut = sorted(os.path.join(mutante, f) for f in os.listdir(mutante)
                          if f.startswith("f") and f.endswith(".png"))
    lineas_mut = medir(mutante, host, "TN", cajas["TN"], ancla, ventana, capturas_mut)
    pares_m, min_m, max_m = pares_de(lineas_mut)
    control("C8 criterio 3: el mutante", pares_m > 0 and max_m == 0,
            "capturas=%d pares=%d pixelesCambiados min=%d max=%d (tiene que dar 0)" % (
                len(capturas_mut), pares_m, min_m, max_m))

    marcas_mut = marcas_anim(log_mut, "TN")
    cuadros_mut = sorted(set(c for c, _ in marcas_mut))
    control("C9 el mutante dibuja el 0", cuadros_mut == [0],
            "cuadros dibujados=%s marcas=%d" % (",".join(str(c) for c in cuadros_mut) or "-", len(marcas_mut)))

    ok = sum(1 for _, bien, _ in resultados if bien)
    print()
    print("ANIM249 CORPUS %d/%d" % (ok, len(resultados)))
    for nombre, bien, _ in resultados:
        if not bien:
            print("   ROJO: %s" % nombre)
    return 0 if ok == len(resultados) else 1


if __name__ == "__main__":
    sys.exit(main())

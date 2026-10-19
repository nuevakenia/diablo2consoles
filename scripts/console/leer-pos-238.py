#!/usr/bin/env python3
"""DC-002.38 -- leer las marcas `HITO238: POS` del POC y contestar, con numeros, si el personaje se movio
como el motor: velocidad, coherencia con la proyeccion del piso y direccion por tramo.

LAS TRES COSAS QUE MIDE (y por que son las que hay que medir)
-------------------------------------------------------------
1. VELOCIDAD: `caminado` es la distancia que el personaje avanzo en subtiles y `tiempo` son los segundos que
   estuvo caminando (la suma de los pasos que movieron). `caminado / tiempo` TIENE que dar exactamente la
   velocidad del motor (d2path: 9,375 subtiles/s caminando para cualquier clase). Si el POC usara otro numero
   (una velocidad inventada, o "de a un pixel por cuadro"), esto lo delata.
2. LA PROYECCION: el pixel que el POC dice que uso para dibujar (`ancla`) tiene que ser el de la posicion del
   motor proyectada al piso: ((x-y)*80 - 80 - origen, (x+y)*40 - origen). Se verifica con la diferencia entre
   dos marcas: `Ancla` menos `Ancla_anterior` tiene que dar la proyeccion del cambio de celda.

   OJO CON EL PASO DE LA CELDA (DC-002.48b, medido): es 80x40, el `orthoTileWidth`/`orthoTileHeight` del
   renderer del motor, NO 32x16 (eso es UN SUBTILE: cada losa de piso mide 160x79 y encaja borde con borde
   cuando la vecina esta a medio diamante). Con 32x16 esta cuenta daba "SOLO EN EL PAPEL": el ancla del POC
   seguia su propia proyeccion, tambien equivocada, y el error salia 0.
3. LA DIRECCION: cada tramo tiene que mover al personaje como el control: la flecha derecha mueve en +x de
   pantalla y deja la y quieta; la flecha abajo deja la x quieta y mueve +y; arriba+derecha mueve +x y -y.

Uso:  leer-pos-238.py <log del emulador> [--velocidad-min 9.30 --velocidad-max 9.45]
Sale 0 siempre: los numeros son el resultado (el veredicto lo da el corpus).
"""
import re
import sys

VELOCIDAD_DEL_MOTOR = 9.375

PATRON = re.compile(
    r"HITO238: POS cuadro (\d+) ms (\d+) botones (\d+) subtiles ([\d.]+) ([\d.]+) "
    r"celda ([\d.]+) ([\d.]+) tile (\d+) (\d+) ancla (-?\d+) (-?\d+) pantalla (-?\d+) (-?\d+) "
    r"caminado ([\d.]+) tiempo ([\d.]+) pasos (\d+) dtCero (\d+) dtGrande (\d+)")

CONTROL = re.compile(
    r"HITO238: CONTROL botones (\d+) ejes (-?\d+) (-?\d+) celda ([\d.]+) ([\d.]+) cuadro (\d+)")


def leer(ruta):
    marcas, controles = [], []
    for linea in open(ruta, errors="ignore"):
        linea = linea.replace("\r", "")
        m = PATRON.search(linea)
        if m:
            (cuadro, ms, bot, sx, sy, cx, cy, tx, ty, ax, ay, px, py, cam, tie, pasos,
             cero, grande) = m.groups()
            marcas.append(dict(cuadro=int(cuadro), ms=int(ms), bot=int(bot), sx=float(sx), sy=float(sy),
                               celda=(float(cx), float(cy)), tile=(int(tx), int(ty)),
                               ancla=(int(ax), int(ay)), pantalla=(int(px), int(py)),
                               caminado=float(cam), tiempo=float(tie), pasos=int(pasos),
                               dt_cero=int(cero), dt_grande=int(grande)))
        m = CONTROL.search(linea)
        if m:
            bot, ex, ey, cx, cy, cuadro = m.groups()
            controles.append(dict(cuadro=int(cuadro), botones=int(bot), ejes=(int(ex), int(ey)),
                                  celda=(float(cx), float(cy))))
    return marcas, controles


def main():
    ruta = sys.argv[1]
    marcas, controles = leer(ruta)
    print("POS leidas: %d   CONTROL leidos: %d" % (len(marcas), len(controles)))

    if not marcas:
        print("POS: SIN MARCAS (el POC no imprimio la posicion)")
        return 0

    for c in controles:
        print("CONTROL cuadro %4d botones %3d ejes %+d %+d" % (c["cuadro"], c["botones"], c["ejes"][0],
                                                               c["ejes"][1]))

    # ---- 1. la velocidad: caminado/tiempo tiene que ser la del motor
    ult = marcas[-1]
    if ult["tiempo"] > 0:
        vel = ult["caminado"] / ult["tiempo"]
        print("VELOCIDAD caminado=%.4f subtiles tiempo=%.4f s -> %.4f sub/s (motor %.4f, %+.2f%%)" %
              (ult["caminado"], ult["tiempo"], vel, VELOCIDAD_DEL_MOTOR,
               100.0 * (vel - VELOCIDAD_DEL_MOTOR) / VELOCIDAD_DEL_MOTOR))
    else:
        print("VELOCIDAD caminado=%.4f tiempo=0 (no camino: %.4f)" % (ult["caminado"], ult["caminado"]))

    # ---- 2. la proyeccion: ancla y pantalla siguen a la celda del motor
    errores = 0
    for a, b in zip(marcas, marcas[1:]):
        d_cel = (b["celda"][0] - a["celda"][0], b["celda"][1] - a["celda"][1])
        d_ancla = (b["ancla"][0] - a["ancla"][0], b["ancla"][1] - a["ancla"][1])
        esperado = (int(round((d_cel[0] - d_cel[1]) * 80)), int(round((d_cel[0] + d_cel[1]) * 40)))
        err = max(abs(d_ancla[0] - esperado[0]), abs(d_ancla[1] - esperado[1]))
        errores = max(errores, err)
    print("PROYECCION error maximo del ancla contra la proyeccion del piso: %d px (en %d tramos)" %
          (errores, max(0, len(marcas) - 1)))

    # ---- 3. la direccion, tramo por tramo (solo los tramos donde hubo movimiento)
    for a, b in zip(marcas, marcas[1:]):
        d_cam = b["caminado"] - a["caminado"]
        if d_cam <= 0:
            continue
        d_pan = (b["pantalla"][0] - a["pantalla"][0], b["pantalla"][1] - a["pantalla"][1])
        print("TRAMO cuadro %4d->%4d  caminado=+%6.3f  pantalla %s -> %s  delta=%s  botones=%d" %
              (a["cuadro"], b["cuadro"], d_cam, a["pantalla"], b["pantalla"], d_pan, b["bot"]))

    print("TOTAL caminado=%.4f subtiles (%.3f celdas) tiempo=%.4f s pasos=%d dtCero=%d dtGrande=%d" %
          (ult["caminado"], ult["caminado"] / 5.0, ult["tiempo"], ult["pasos"], ult["dt_cero"],
           ult["dt_grande"]))

    inicio = marcas[0]
    print("DESDE cuadro %d hasta cuadro %d: pantalla %s -> %s (delta %d,%d), celda %s -> %s" %
          (inicio["cuadro"], ult["cuadro"], inicio["pantalla"], ult["pantalla"],
           ult["pantalla"][0] - inicio["pantalla"][0], ult["pantalla"][1] - inicio["pantalla"][1],
           inicio["celda"], ult["celda"]))
    return 0


if __name__ == "__main__":
    sys.exit(main())

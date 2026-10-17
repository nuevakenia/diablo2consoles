#!/usr/bin/env python3
"""DC-002.26: mide si el cuadro del motor sale ROTADO en la captura de la consola.

El instrumento no mira "a ojo": cuenta la TINTA (los pixeles oscuros del texto) por fila y por columna de
la pantalla de la consola, y mira la FORMA del perfil. Un texto escrito en horizontal tiene lineas: su
perfil por filas da picos y valles (varianza alta) y su perfil por columnas es parejo (varianza baja). Con
el cuadro rotado 90 grados pasa lo contrario (las "lineas" corren de arriba abajo). O sea: la varianza mas
grande dice como esta el texto.

    python3 scripts/console/medir-volteo-226.py <captura.png> [--logico FILAS COLUMNAS]

--logico FILAS COLUMNAS son los dos numeros que la consola imprime de su cuadro LOGICO
(`VOLTEO-226 logico var_filas=... var_columnas=...`, lo que el motor dibujo, antes de que el PVR lo lea):
con eso la comparacion es captura contra cuadro logico, no contra el ojo. Es la leccion de DC-002.10/DC-002.11:
el instrumento puede medir otra cosa que lo que se ve.

Sale 0 si el cuadro esta DERECHO (y, si se paso --logico, si ademas coincide con el logico), 1 si no.
"""
import argparse
import subprocess
import sys


def size(png):
    out = subprocess.run(["ffprobe", "-v", "error", "-select_streams", "v",
                          "-show_entries", "stream=width,height", "-of", "csv=p=0", png],
                         capture_output=True, text=True).stdout.strip()
    w, h = out.split(",")
    return int(w), int(h)


def leer(png, w, h):
    p = subprocess.run(["ffmpeg", "-v", "error", "-i", png, "-f", "rawvideo",
                        "-pix_fmt", "rgb24", "-"], capture_output=True)
    if p.returncode != 0:
        sys.exit("ffmpeg fallo con " + png + ": " + p.stderr.decode()[:200])
    return p.stdout


def es_fondo_rojo(buf, w, i):
    return buf[i] > 120 and buf[i + 1] < 90 and buf[i + 2] < 90


def color_dominante(buf, CW, x0, y0, W, H):
    """El color de fondo de la pantalla: el que mas aparece (el rojo del cuadro de error)."""
    cuenta = {}
    for y in range(0, H, 3):
        base = (y0 + y) * CW * 3
        for x in range(0, W, 3):
            i = base + (x0 + x) * 3
            c = (buf[i] >> 4, buf[i + 1] >> 4, buf[i + 2] >> 4)
            cuenta[c] = cuenta.get(c, 0) + 1
    c = max(cuenta, key=cuenta.get)
    return (c[0] << 4) + 8, (c[1] << 4) + 8, (c[2] << 4) + 8, cuenta[c]


def ubicar(buf, W, H):
    """La pantalla de la consola es la zona donde domina el rojo del cuadro de error."""
    filas = []
    for y in range(H):
        rojos = sum(1 for x in range(0, W, 4) if es_fondo_rojo(buf, W, (y * W + x) * 3))
        if rojos > 40:
            filas.append(y)
    if not filas:
        sys.exit("no encontre la pantalla (nada rojo) en la captura")
    y0, y1 = filas[0], filas[-1]
    ym = (y0 + y1) // 2
    xs = [x for x in range(W) if es_fondo_rojo(buf, W, (ym * W + x) * 3)]
    x0, x1 = min(xs), max(xs)
    return x0, y0, x1 - x0 + 1, y1 - y0 + 1


def varianza(valores):
    n = len(valores)
    if n < 2:
        return 0.0
    m = sum(valores) / n
    return sum((v - m) ** 2 for v in valores) / n


def perfil(buf, CW, x0, y0, W, H, fondo, umbral=60):
    """Perfil de TINTA: un pixel es tinta si se aleja del color de fondo de la pantalla. El texto del cuadro
    de error del motor es BLANCO sobre rojo (medido), asi que "pixel oscuro" no sirve: se mide "no es el
    fondo"."""
    fr, fg, fb = fondo[0], fondo[1], fondo[2]
    filas = [0] * H
    cols = [0] * W
    tinta = 0
    for y in range(H):
        base = (y0 + y) * CW * 3
        for x in range(W):
            i = base + (x0 + x) * 3
            if (abs(buf[i] - fr) > umbral or abs(buf[i + 1] - fg) > umbral
                    or abs(buf[i + 2] - fb) > umbral):
                filas[y] += 1
                cols[x] += 1
                tinta += 1
    return filas, cols, tinta


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("captura")
    ap.add_argument("--logico", nargs=2, type=float, default=None,
                    metavar=("FILAS", "COLUMNAS"),
                    help="los dos numeros del perfil del cuadro LOGICO (los imprime la consola)")
    args = ap.parse_args()

    CW, CH = size(args.captura)
    buf = leer(args.captura, CW, CH)
    x0, y0, W, H = ubicar(buf, CW, CH)
    fondo = color_dominante(buf, CW, x0, y0, W, H)
    filas, cols, tinta = perfil(buf, CW, x0, y0, W, H, fondo)
    vf, vc = varianza(filas), varianza(cols)

    print(f"captura: {args.captura}")
    print(f"  pantalla de la consola: {W}x{H} en {x0},{y0} (captura {CW}x{CH})")
    print(f"  fondo dominante de la pantalla: rgb({fondo[0]},{fondo[1]},{fondo[2]}) "
          f"({fondo[3]} muestras de las medidas)")
    print(f"  pixeles de tinta (no son el fondo): {tinta}")
    print(f"  varianza por FILAS (lo que tendria que ser grande): {vf:.1f}")
    print(f"  varianza por COLUMNAS (lo que tendria que ser chica): {vc:.1f}")
    print(f"  pico de tinta: fila {max(filas)} en una fila, columna {max(cols)} en una columna")

    derecho = vf > vc
    print("  VEREDICTO:", "DERECHO (las lineas de texto corren en horizontal)" if derecho
          else "ROTADO 90 GRADOS (las lineas corren de arriba abajo)")

    ok = derecho and tinta > 200
    if args.logico:
        lf, lc = args.logico
        print(f"  cuadro LOGICO (lo que dibujo el motor): var_filas {lf:.1f}  var_columnas {lc:.1f}")
        print(f"  desvio captura contra logico: filas {abs(vf - lf):.1f}  columnas {abs(vc - lc):.1f}")
        # la captura tiene que parecerse al logico, no a su traspuesta: si se parece mas a la traspuesta,
        # el PVR esta leyendo el cuadro al reves
        cerca_logico = abs(vf - lf) + abs(vc - lc) < abs(vf - lc) + abs(vc - lf)
        print("  la captura se parece mas al:", "cuadro logico" if cerca_logico else "cuadro TRASPUESTO")
        ok = ok and cerca_logico
    print("RESULTADO:", "OK" if ok else "NO")
    sys.exit(0 if ok else 1)


if __name__ == "__main__":
    main()

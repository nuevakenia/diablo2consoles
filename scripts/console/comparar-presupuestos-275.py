#!/usr/bin/env python3
"""DC-002.75: comparar la captura del POC de los presupuestos contra la evidencia de DC-002.70.

QUE COMPARA
  Una foto del escritorio con la ventana de Flycast adentro (la corrida del POC de esta tarjeta,
  console/dc/poc/presupuestos) contra el RECORTE de 640x474 que la evidencia de DC-002.70 guarda en
  console/dc/evidencia/dc-002.73a-estado-b-cuadro.png (la captura re-emitida del estado B, documentada
  byte a byte identica a dc-002.70-estado-b-cuadro.png).

  Las dos puntas son el MISMO escenario: el .ds1 del acto 1, el personaje quieto en la celda 28,20 y el
  primer cuadro. La unica diferencia en el codigo es de donde salen los datos: en DC-002.70 se leian del
  MPQ y se tiraban, aca pasan por los caches del motor con los presupuestos de consola. Si el cuadro
  cambia, es el cache.

COMO SE UBICA LA PANTALLA EN LA CAPTURA (sin clavar una posicion)
  Igual que comparar-estado-b-270.py: el mapa de 512x512 se pega en (64, -16), asi que el cuadro tiene DOS
  BANDAS NEGRAS de 64 columnas (x 0..63 y x 576..639). Se busca el tramo de 64 columnas negras en (casi)
  todas las filas y su simetrico a la derecha: eso da x0, y el tramo de filas con las dos bandas negras
  da y0. La referencia (el PNG de 640x474) se compara en su propio (0,0).

LAS ULTIMAS 6 FILAS NO SE COMPARAN Y SE DICE POR QUE
  La ventana del emulador dibuja su borde inferior sobre las ultimas filas del cuadro (medido en
  DC-002.70: en la fila 600 de la captura las columnas de las bandas son (83,85,88), el gris del borde).
  Se comparan las 474 filas que el emulador muestra enteras.

QUE REPORTA
  1. donde quedo la pantalla en la captura;
  2. pixeles IDENTICOS, cuantizacion (<= 8 por canal: el RGB565 del PVR y el dithering de Flycast) y
     CONTENIDO distinto (> 8), en el cuadro entero y partido en las dos zonas que ya uso DC-002.70:
     el area del mapa (x 64..575) y las bandas laterales.

Uso:  comparar-presupuestos-275.py <captura.png> <referencia.png>
Devuelve 0 si el area del mapa da >= 99,8 % de pixeles identicos, 1 si no.
"""
import subprocess
import sys

ANCHO, ALTO = 640, 480
BANDA = 64      # (640 - 512) / 2
FILAS = 474     # las que el emulador muestra enteras (ver la cabecera)
TOL = 8         # lo que puede mover el RGB565 + el dithering de Flycast

# el criterio de aceptacion de la tarjeta: en el area del mapa, >= 99,8 % de pixeles identicos
MINIMO_EN_EL_MAPA = 99.8


def decodificar(png):
    info = subprocess.run(["ffprobe", "-v", "error", "-select_streams", "v:0", "-show_entries",
                           "stream=width,height", "-of", "csv=p=0", png],
                          stdout=subprocess.PIPE, check=True).stdout.decode().strip()
    w, h = [int(v) for v in info.split(",")[:2]]
    datos = subprocess.run(["ffmpeg", "-y", "-loglevel", "error", "-i", png,
                            "-f", "rawvideo", "-pix_fmt", "rgb24", "-"],
                           stdout=subprocess.PIPE, check=True).stdout
    return datos, w, h


def ubicar(datos, w, h):
    """(x0, y0, filas) de la pantalla de la consola dentro de la captura del escritorio"""
    negras = [0] * w
    for y in range(h):
        base = y * w * 3
        for x in range(w):
            j = base + x * 3
            if datos[j] == 0 and datos[j + 1] == 0 and datos[j + 2] == 0:
                negras[x] += 1

    mejor = None
    for x0 in range(0, w - ANCHO + 1):
        izq = min(negras[x0:x0 + BANDA])
        der = min(negras[x0 + ANCHO - BANDA:x0 + ANCHO])
        alto = min(izq, der)
        if alto < FILAS * 0.9:
            continue
        if mejor is None or alto > mejor[1]:
            mejor = (x0, alto)

    if mejor is None:
        return None

    x0 = mejor[0]

    empieza, largo, mejorY, mejorLargo = -1, 0, 0, 0
    for y in range(h):
        limpia = (datos[(y * w + x0) * 3] == 0 and datos[(y * w + x0) * 3 + 1] == 0 and
                  datos[(y * w + x0) * 3 + 2] == 0) and \
                 (datos[(y * w + x0 + ANCHO - 1) * 3] == 0 and
                  datos[(y * w + x0 + ANCHO - 1) * 3 + 1] == 0 and
                  datos[(y * w + x0 + ANCHO - 1) * 3 + 2] == 0)
        if limpia:
            if empieza < 0:
                empieza, largo = y, 0
            largo += 1
        else:
            if largo > mejorLargo:
                mejorLargo, mejorY = largo, empieza
            empieza = -1

    if largo > mejorLargo:
        mejorLargo, mejorY = largo, empieza

    return x0, mejorY, mejorLargo


def main():
    if len(sys.argv) < 3:
        print(__doc__)
        return 2

    datos, w, h = decodificar(sys.argv[1])
    ref, wr, hr = decodificar(sys.argv[2])

    if (wr, hr) != (ANCHO, FILAS):
        print("la referencia tiene que ser el recorte de %dx%d (es %dx%d)" % (ANCHO, FILAS, wr, hr))
        return 2

    u = ubicar(datos, w, h)
    if u is None:
        print("NO PUDE UBICAR LA PANTALLA DE LA CONSOLA en %s" % sys.argv[1])
        return 1

    x0, y0 = u[0], u[1]
    print("captura    %s (%dx%d) -> pantalla en (%d, %d)" % (sys.argv[1], w, h, x0, y0))
    print("referencia %s (%dx%d)  <- la evidencia de DC-002.70" % (sys.argv[2], wr, hr))

    # las dos zonas que uso DC-002.70
    zonas = [
        ("cuadro completo (x 0..639)", range(0, ANCHO)),
        ("area del mapa (x 64..575)", range(BANDA, ANCHO - BANDA)),
        ("bandas laterales (x 0..63 y 576..639)", list(range(0, BANDA)) + list(range(ANCHO - BANDA, ANCHO))),
    ]

    resultado = {}

    for nombre, columnas in zonas:
        identicos = cuantizacion = contenido = total = 0
        caja = [ANCHO, FILAS, -1, -1]
        muestra = []

        for y in range(FILAS):
            for x in columnas:
                ja = (y0 + y) * w * 3 + (x0 + x) * 3
                jb = y * wr * 3 + x * 3
                d0 = abs(datos[ja] - ref[jb])
                d1 = abs(datos[ja + 1] - ref[jb + 1])
                d2 = abs(datos[ja + 2] - ref[jb + 2])
                peor = max(d0, d1, d2)
                total += 1

                if peor == 0:
                    identicos += 1
                elif peor <= TOL:
                    cuantizacion += 1
                else:
                    contenido += 1
                    caja[0] = min(caja[0], x)
                    caja[1] = min(caja[1], y)
                    caja[2] = max(caja[2], x)
                    caja[3] = max(caja[3], y)
                    if len(muestra) < 5:
                        muestra.append((x, y, (datos[ja], datos[ja + 1], datos[ja + 2]),
                                        (ref[jb], ref[jb + 1], ref[jb + 2])))

        resultado[nombre] = 100.0 * identicos / total

        print("")
        print("%s" % nombre)
        print("  pixeles        %d" % total)
        print("  IDENTICOS      %d  (%.2f %%)" % (identicos, resultado[nombre]))
        print("  cuantizacion   %d  (%.2f %%): difieren <= %d por canal" %
              (cuantizacion, 100.0 * cuantizacion / total, TOL))
        print("  CONTENIDO      %d  (%.2f %%)" % (contenido, 100.0 * contenido / total))
        if contenido:
            print("     caja: x %d..%d  y %d..%d" % (caja[0], caja[2], caja[1], caja[3]))
            for d in muestra:
                print("     (%d,%d) presupuestos %s  vs  DC-002.70 %s" % d)

    mapa = resultado["area del mapa (x 64..575)"]
    print("")
    print("filas comparadas: %d de %d (las ultimas 6 son el borde de la ventana del emulador)" % (FILAS, ALTO))

    if mapa >= MINIMO_EN_EL_MAPA:
        print("RESULTADO: OK -- el mapa da %.2f %% identico (el criterio era >= %.1f %%)" %
              (mapa, MINIMO_EN_EL_MAPA))
        return 0

    print("RESULTADO: REVISAR -- el mapa da %.2f %% y el criterio es >= %.1f %%" % (mapa, MINIMO_EN_EL_MAPA))
    return 1


if __name__ == "__main__":
    sys.exit(main())

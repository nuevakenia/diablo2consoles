#!/usr/bin/env python3
"""DC-002.49b -- la HUELLA de la zona del emulador dentro de la captura (no de la pantalla entera).

POR QUE EXISTE
--------------
`run-flycast-249.sh` captura la PANTALLA ENTERA (`spectacle -f`) y descartaba la captura cuando su md5
repetia el de la anterior. En este host eso no alcanza: la pantalla mide 3584x1080 y el reloj de la barra de
tareas (o cualquier otro pixel fuera del emulador) cambia el md5 de la pantalla entera aunque la zona del
juego sea IDENTICA. Medido el 2026-10-29: las capturas f5 y f8 del corpus salieron con md5 distinto y con la
caja del personaje identica pixel a pixel -> el control C6 (que exige que TODOS los pares cambien) se caia
por el plan de captura y no por la animacion.

Esta huella se calcula SOLO sobre el area cliente del emulador (la misma que reportan las lineas GEO:
`cliente=L,T,W,H`), asi que dos capturas con la misma huella son el mismo cuadro del juego.

Uso:  huella-zona.py <captura.png> <L,T,W,H>     -> imprime el md5 de esa zona (la misma pantalla de 32 hex)
      huella-zona.py --autoprueba                 -> 1 verde y 2 rojos esperados (el instrumento se prueba)
"""
import hashlib
import importlib.util
import os
import sys

AQUI = os.path.dirname(os.path.abspath(__file__))


def decodificador():
    """Reusa decodificar() de medir-captura-238.py: el mismo lector de capturas de todo el arbol."""
    ruta = os.path.join(AQUI, "medir-captura-238.py")
    spec = importlib.util.spec_from_file_location("medidor238", ruta)
    mod = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(mod)
    return mod.decodificar


def huella(png, zona):
    decodificar = decodificador()
    img = decodificar(png)
    izq, arriba, ancho, alto = [int(x) for x in zona.split(",")]
    alto_img, ancho_img, _ = img.shape
    izq = max(0, min(izq, ancho_img - 1))
    arriba = max(0, min(arriba, alto_img - 1))
    trozo = img[arriba:arriba + alto, izq:izq + ancho]
    return hashlib.md5(trozo.tobytes()).hexdigest()


def autoprueba():
    """No hay capturas de repuesto en el repo, asi que la autoprueba se arma con un PNG sintetico."""
    import subprocess
    import tempfile

    casos = []
    with tempfile.TemporaryDirectory() as tmp:
        a = os.path.join(tmp, "a.png")
        b = os.path.join(tmp, "b.png")
        c = os.path.join(tmp, "c.png")
        subprocess.run(["ffmpeg", "-v", "error", "-f", "lavfi", "-i", "color=c=black:s=64x48",
                        "-frames:v", "1", a], check=True)
        subprocess.run(["ffmpeg", "-v", "error", "-f", "lavfi", "-i", "color=c=black:s=64x48",
                        "-frames:v", "1", b], check=True)
        subprocess.run(["ffmpeg", "-v", "error", "-f", "lavfi", "-i", "color=c=red:s=64x48",
                        "-frames:v", "1", c], check=True)
        casos.append(("dos capturas iguales, misma zona", huella(a, "0,0,64,48") == huella(b, "0,0,64,48"), True))
        casos.append(("dos capturas distintas, misma zona", huella(a, "0,0,64,48") == huella(c, "0,0,64,48"), False))
        # la zona fuera de la imagen no puede dar verde por accidente: se acota y tiene que seguir midiendo
        casos.append(("zona acotada al borde", huella(a, "60,44,64,48") == huella(c, "60,44,64,48"), False))

    bien = 0
    for nombre, obtenido, esperado in casos:
        print("%-5s %s (se esperaba %s)" % ("OK" if obtenido == esperado else "FALLA", nombre, esperado))
        bien += obtenido == esperado
    print("autoprueba: %d/%d" % (bien, len(casos)))
    return 0 if bien == len(casos) else 1


def main():
    if len(sys.argv) >= 2 and sys.argv[1] == "--autoprueba":
        return autoprueba()
    if len(sys.argv) != 3:
        print("uso: huella-zona.py <captura.png> <L,T,W,H> | --autoprueba")
        return 2
    print(huella(sys.argv[1], sys.argv[2]))
    return 0


if __name__ == "__main__":
    sys.exit(main())

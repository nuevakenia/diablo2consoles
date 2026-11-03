#!/usr/bin/env python3
"""DC-002.137: recorta la caja del minimapa (y el cuadro entero) de una captura de Flycast.

La captura es la ventana del emulador (3584x1080 con la barra); el area cliente del juego es la que el runner
dejo en geo.txt (`cliente=900,120,640,480`). La caja del minimapa es (359,57)-(638,282) DENTRO del area cliente.

Uso:  python3 recortar-137.py <captura.png> <salida_prefijo>
"""
import sys

from PIL import Image

CLIENTE = (900, 120, 900 + 640, 120 + 480)
CAJA = (359, 57, 638, 282)


def main() -> int:
    if len(sys.argv) < 3:
        print("uso: recortar-137.py <captura.png> <salida_prefijo>")
        return 2

    imagen = Image.open(sys.argv[1]).convert("RGB")
    cliente = imagen.crop(CLIENTE)
    cliente.save(sys.argv[2] + "-cliente.png")

    x0, y0, x1, y1 = CAJA
    caja = cliente.crop((x0, y0, x1, y1)).resize(((x1 - x0) * 2, (y1 - y0) * 2), Image.NEAREST)
    caja.save(sys.argv[2] + "-caja.png")

    # el crc de la caja tal como sale en pantalla (para comparar dos capturas del mismo estado)
    import zlib

    print("cliente", cliente.size, "caja", (x1 - x0, y1 - y0), "crcCajaPng=",
          zlib.crc32(cliente.crop((x0, y0, x1, y1)).tobytes()))

    return 0


if __name__ == "__main__":
    raise SystemExit(main())

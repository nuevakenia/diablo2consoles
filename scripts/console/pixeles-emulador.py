#!/usr/bin/env python3
# Cuenta los pixeles NO VACIOS (algun canal > 24) dentro del area de video del emulador en cada captura,
# y devuelve tambien la caja de esos pixeles. El area de video se fijo a mano sobre una captura
# (ver recorte-esquina.png): x 0..627, y 28..512 de una pantalla de 3584x1080.
import sys, glob, os
import numpy as np
from PIL import Image

X0, X1, Y0, Y1 = 0, 627, 28, 512
TOPE = 24
for d in sys.argv[1:]:
    for f in sorted(glob.glob(os.path.join(d, "f*.png")), key=lambda p: int(''.join(c for c in os.path.basename(p) if c.isdigit()))):
        a = np.asarray(Image.open(f).convert("RGB"))[Y0:Y1, X0:X1]
        mask = (a.max(axis=2) > TOPE)
        n = int(mask.sum())
        if n:
            ys, xs = np.where(mask)
            caja = "x %d..%d  y %d..%d" % (xs.min()+X0, xs.max()+X0, ys.min()+Y0, ys.max()+Y0)
        else:
            caja = "-"
        print("%-28s %-8s noVacios=%6d  %s" % (os.path.basename(d), os.path.basename(f), n, caja))

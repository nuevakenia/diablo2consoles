"""DC-002.37: busca la alineacion del bloque pintado contra la referencia en una ventana ANCHA.

Por que: `medir-captura-237.py` busca +-12 px y devolvio dx=-14 dy=+8 (el borde del rango, o sea que se
quedo corto) con una correlacion de 0,0227: el cuadro puede estar ahi y simplemente corrido mas de 12 px.
Aca se busca +-96 px por correlacion de Pearson sobre la luminancia, con la referencia reescalada al tamano
del bloque, y se dice cual es el mejor desplazamiento y cuanto sube.
"""
import sys

import numpy as np
from PIL import Image

CAPTURA, REFERENCIA, CAJA = sys.argv[1], sys.argv[2], [int(v) for v in sys.argv[3].split(",")]
RANGO = int(sys.argv[4]) if len(sys.argv) > 4 else 96

cap = np.asarray(Image.open(CAPTURA).convert("RGB")).astype(np.float64)
x1, y1, x2, y2 = CAJA
alto, ancho = y2 - y1, x2 - x1
bloque = cap[y1:y2, x1:x2].mean(axis=2)
ref = np.asarray(Image.open(REFERENCIA).convert("RGB").resize((ancho, alto), Image.BILINEAR), dtype=np.float64).mean(axis=2)
ref = (ref - ref.mean()) / (ref.std() + 1e-9)

mejor = (-2.0, 0, 0)
for dy in range(-RANGO, RANGO + 1, 2):
    for dx in range(-RANGO, RANGO + 1, 2):
        yy1, yy2 = max(0, dy), min(alto, alto + dy)
        xx1, xx2 = max(0, dx), min(ancho, ancho + dx)
        sub = bloque[yy1:yy2, xx1:xx2]
        r = ref[max(0, -dy):max(0, -dy) + sub.shape[0], max(0, -dx):max(0, -dx) + sub.shape[1]]
        if sub.size < 10000:
            continue
        a = (sub - sub.mean()) / (sub.std() + 1e-9)
        c = float((a * r).mean())
        if c > mejor[0]:
            mejor = (c, dx, dy)
c, dx, dy = mejor
print("MEJOR CORRELACION %.4f con desplazamiento dx=%d dy=%d (ventana +-%d px)" % (c, dx, dy, RANGO))
print("CAMPOS para la evidencia: corr=%.4f dx=%d dy=%d rango=%d" % (c, dx, dy, RANGO))

"""DC-002.37: el color medio del bloque pintado contra el de la referencia del host.

Por que: el instrumento (medir-captura-237.py) ya dice que el cuadro NO es negro (78,31 % de pixeles no
fondo) y que la comparacion con la referencia da grueso mas8=22,12 (el umbral es >=95). Este script pone el
numero que falta para saber SI el problema es el color: la media y el maximo por canal de los dos lados.
"""
import sys
from pathlib import Path

import numpy as np
from PIL import Image

CAPTURA = sys.argv[1]
REFERENCIA = sys.argv[2]
CAJA = [int(v) for v in sys.argv[3].split(",")]  # x1,y1,x2,y2 del bloque en la captura


def medios(a, etiqueta):
    a = a.astype(np.float64)
    print("%s: forma=%s  media R=%.1f G=%.1f B=%.1f  max R=%d G=%d B=%d  pixeles>12 en algun canal=%.2f%%"
          % (etiqueta, a.shape, a[..., 0].mean(), a[..., 1].mean(), a[..., 2].mean(),
             a[..., 0].max(), a[..., 1].max(), a[..., 2].max(),
             100.0 * float((a.max(axis=2) > 12).mean())))


cap = np.asarray(Image.open(CAPTURA).convert("RGB"))
x1, y1, x2, y2 = CAJA
bloque = cap[y1:y2, x1:x2]
ref = np.asarray(Image.open(REFERENCIA).convert("RGB"))

medios(bloque, "bloque de la consola (%dx%d)" % (bloque.shape[1], bloque.shape[0]))
medios(ref, "referencia del host   (%dx%d)" % (ref.shape[1], ref.shape[0]))

# el escalado es puro: el bloque mide el alto de la pantalla logica (480) y la referencia 512
gris_cap = bloque.mean(axis=2)
gris_ref = np.asarray(Image.fromarray(ref).convert("L").resize((bloque.shape[1], bloque.shape[0]), Image.BILINEAR), dtype=np.float64)
print("correlacion de la luminancia (bloque contra referencia reescalada): %.4f" % float(np.corrcoef(gris_cap.ravel(), gris_ref.ravel())[0, 1]))
print("brillo medio: consola %.1f / host %.1f (razon %.3f)" % (gris_cap.mean(), gris_ref.mean(), gris_cap.mean() / max(gris_ref.mean(), 1e-9)))
print("CAMPOS para la evidencia: consola_media=%.1f,%.1f,%.1f host_media=%.1f,%.1f,%.1f razon_brillo=%.3f corr=%.4f"
      % (bloque[..., 0].mean(), bloque[..., 1].mean(), bloque[..., 2].mean(),
         ref[..., 0].mean(), ref[..., 1].mean(), ref[..., 2].mean(),
         gris_cap.mean() / max(gris_ref.mean(), 1e-9),
         float(np.corrcoef(gris_cap.ravel(), gris_ref.ravel())[0, 1])))
_ = Path  # sin uso

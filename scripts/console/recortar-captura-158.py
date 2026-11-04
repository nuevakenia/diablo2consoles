"""DC-002.158: recorta el cuadro del emulador de la captura de pantalla entera y lo compara con la de
DC-002.157.

POR QUE. `run-flycast.sh` fotografia la PANTALLA ENTERA (spectacle -f): la evidencia de DC-002.157 es el
cuadro de 640x480 recortado. Para la de esta tarjeta se busca el recuadro de la ventana del emulador por
correlacion contra esa referencia -- y de paso el numero de coincidencia ES la prueba de que el dibujo no
cambio (el reloj no toca un pixel).

Busqueda en dos etapas: a 1/8 de escala (grueso) y despues al pixel (fino, +-8 px alrededor del grueso).
"""
import sys
from PIL import Image
import numpy as np

GRANDE = "/tmp/capturas158/f120s.png"                                            # pantalla entera, 3584x1080
REF = "/home/dgalli/Proyectos/diablo2consoles/console/dc/evidencia/dc-002.157-gate0-el-mapa-estatico.png"
DESTINO = "/home/dgalli/Proyectos/diablo2consoles/console/dc/evidencia/dc-002.158-gate0-el-reloj.png"

grande = Image.open(GRANDE).convert("RGB")
ref = Image.open(REF).convert("RGB")
W, H = ref.size
print("pantalla %dx%d   referencia %dx%d" % (grande.size + ref.size))

# ---- etapa gruesa: 1/8 ----
esc = 8
g = np.asarray(grande.resize((grande.width // esc, grande.height // esc), Image.BILINEAR), dtype=np.float32)
r = np.asarray(ref.resize((W // esc, H // esc), Image.BILINEAR), dtype=np.float32)
gh, gw = g.shape[:2]
rh, rw = r.shape[:2]
mejor = None
for y in range(0, gh - rh):
    franja = g[y:y + rh]
    for x in range(0, gw - rw):
        d = np.abs(franja[:, x:x + rw] - r)
        v = float(d.mean())
        if mejor is None or v < mejor[0]:
            mejor = (v, x, y)
print("grueso: mad(1/8)=%.2f  esquina=(%d,%d)" % (mejor[0], mejor[1] * esc, mejor[2] * esc))

# ---- etapa fina: al pixel, +-8 px ----
gx, gy = mejor[1] * esc, mejor[2] * esc
full = np.asarray(grande, dtype=np.int16)
r16 = np.asarray(ref, dtype=np.int16)
mejor2 = None
for y in range(max(0, gy - esc), min(full.shape[0] - H, gy + esc) + 1):
    for x in range(max(0, gx - esc), min(full.shape[1] - W, gx + esc) + 1):
        d = np.abs(full[y:y + H, x:x + W] - r16)
        v = float(d.mean())
        if mejor2 is None or v < mejor2[0]:
            mejor2 = (v, x, y)
mad, x0, y0 = mejor2
print("fino: esquina=(%d,%d)  mad=%.3f" % (x0, y0, mad))

recorte = grande.crop((x0, y0, x0 + W, y0 + H))
recorte.save(DESTINO)
print("escrito: %s (%dx%d)" % (DESTINO, W, H))

# ---- la comparacion contra 157 ----
a = np.asarray(recorte, dtype=np.int16)
b = r16
dif = np.abs(a - b).max(axis=2)
print("comparacion con 157: pixeles distintos=%d de %d  max_dif=%d  pixeles con dif>4=%d"
      % (int((dif > 0).sum()), dif.size, int(dif.max()), int((dif > 4).sum())))

#!/usr/bin/env python3
"""DC-002.17: la prueba de que subir la textura POR BLOQUES pone cada pixel donde lo ponia la conversion
del cuadro entero.

POR QUE EXISTE: el arreglo del video cambio 1.048.576 bytes del monton por un buffer estatico de 32x32
(2.048 bytes) y 300 subidas de 2 KiB por cuadro. Eso mueve pixeles: si un bloque queda en el lugar
equivocado, la pantalla se ve mal y el arranque del motor NO se entera (el video "arranca" igual, la
textura esta subida). Un arranque verde no prueba que el cuadro este bien.

LO QUE COMPARA: el indice dentro de la textura de CADA pixel visible (640x480 = 307.200) calculado por la
formula vieja (recorriendo el cuadro entero) contra el que arma la conversion por bloques (base del bloque
+ indice dentro del bloque). Tienen que ser iguales uno a uno. Ademas comprueba que los 1.024 pixeles de
un bloque caen en indices SEGUIDOS y sin repetir: eso es lo que hace posible subirlos con una sola copia
por Store Queues (pvr_txr_load).

Uso:  python3 scripts/console/medir-twiddle-217.py
Sale con 0 si coincide todo, 1 si hay algun pixel distinto.
"""

PW, PH = 1024, 512          # la textura (potencia de dos, como exige el PVR)
W, H = 640, 480             # la parte visible
BITS, MASK, SALTO = 9, 511, 512 * 512


def twid(x):
    """TWIDTAB: el bit i de la coordenada va al bit 2i (esparce los bits). Para 9 bits son 17 bits de
    resultado: por eso la tabla del C es de 32 bits y no de 16."""
    r = 0
    for i in range(10):
        if x & (1 << i):
            r |= 1 << (2 * i)
    return r


def indice_viejo(x, y):
    """La formula de la conversion del cuadro entero (pvr_pantalla.c antes de DC-002.17)."""
    return ((twid(y & MASK) << 1) | twid(x & MASK)) + (x >> BITS) * SALTO + (y >> BITS) * SALTO


def indice_bloque(x, y):
    """La formula nueva: base del bloque de 32x32 + indice dentro del bloque."""
    x0 = (x // 32) * 32
    y0 = (y // 32) * 32
    base = ((twid(y0 & MASK) << 1) + twid(x0 & MASK)
            + (x0 >> BITS) * SALTO + (y0 >> BITS) * SALTO)
    intra = (twid(y - y0) << 1) | twid(x - x0)
    return base + intra


def main():
    distintos = 0
    for y in range(H):
        for x in range(W):
            if indice_viejo(x, y) != indice_bloque(x, y):
                distintos += 1

    print("pixeles visibles comparados:", W * H)
    print("indices distintos:", distintos)

    contiguos = 0
    for y0 in range(0, H, 32):
        for x0 in range(0, W, 32):
            idxs = sorted(indice_bloque(x0 + xx, y0 + yy) for yy in range(32) for xx in range(32))
            if idxs == list(range(idxs[0], idxs[0] + 1024)):
                contiguos += 1
    bloques = (W // 32) * (H // 32)
    print("bloques visibles:", bloques, "bloques con 1024 indices contiguos:", contiguos)

    print("pixeles de la textura:", PW * PH, "visibles:", W * H, "relleno:", PW * PH - W * H)
    print("bytes del monton antes (1024x512x2):", PW * PH * 2)
    print("bytes en .bss ahora (32x32x2):", 32 * 32 * 2)
    print("subidas a la VRAM por cuadro:", bloques, "de", 32 * 32 * 2, "bytes")

    if distintos != 0 or contiguos != bloques:
        print("RESULTADO: NO COINCIDE")
        return 1
    print("RESULTADO: COINCIDE (los", W * H, "pixeles van al mismo indice que antes)")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())

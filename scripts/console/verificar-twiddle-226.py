#!/usr/bin/env python3
"""DC-002.26: el twiddle de la pantalla tiene que usar la convencion del PVR (la de KOS), no la nuestra.

QUE COMPARA: el indice dentro de la textura de CADA pixel visible, calculado como lo calcula
`console/dc/std/csrc/pvr_pantalla.c` (bloque de 32x32 + base del bloque) contra el que pide el PVR, que es
el del propio KOS (`kernel/arch/dreamcast/hardware/pvr/pvr_texture.c:34`):

    TWIDOUT(x, y) = TWIDTAB(y) | (TWIDTAB(x) << 1)        <- los bits de X van a los IMPARES

POR QUE HACIA FALTA (la leccion de "una suite verde no prueba nada"): `medir-twiddle-217.py` compara la
conversion del cuadro entero contra la conversion por bloques, y las DOS usaban la MISMA convencion. Si las
dos tienen el lugar de los bits al reves, coinciden pixel a pixel y la pantalla sale ROTADA 90 grados: una
comparacion nuestra contra nuestra no puede ver eso. Esto compara contra la referencia del hardware.

TAMBIEN calcula la variante vieja (la Y en los impares) y dice cuantos pixeles cambian: si diera 0, este
instrumento no estaria midiendo una rotacion (y el defecto de DC-002.26 no existiria).

Uso:  python3 scripts/console/verificar-twiddle-226.py
Sale 0 si nuestro indice coincide con el del PVR en las dos texturas, 1 si no.
"""
BITS_TABLA = 10          # TWIDTAB cubre 10 bits (su ultimo termino es (x&512)<<9)


def twid(x):
    """TWIDTAB: el bit i de la coordenada va al bit 2i (esparce los bits)."""
    r = 0
    for i in range(BITS_TABLA):
        if x & (1 << i):
            r |= 1 << (2 * i)
    return r


def indice_pvr(x, y, mask, bits, salto):
    """La convencion del PVR / KOS: TWIDOUT(x, y) = TWIDTAB(y) | (TWIDTAB(x) << 1)."""
    return ((twid(x & mask) << 1) | twid(y & mask)) + (x >> bits) * salto + (y >> bits) * salto


def indice_nuestro(x, y, mask, bits, salto):
    """Lo que hace pvr_pantalla.c despues de DC-002.26: base del bloque + indice dentro del bloque, con la
    X en los bits IMPARES."""
    x0, y0 = (x // 32) * 32, (y // 32) * 32
    base = ((twid(x0 & mask) << 1) + twid(y0 & mask)
            + (x0 >> bits) * salto + (y0 >> bits) * salto)
    intra = (twid(x - x0) << 1) | twid(y - y0)
    return base + intra


def indice_viejo_al_reves(x, y, mask, bits, salto):
    """La version de DC-002.10 (el defecto que cierra DC-002.26): la Y en los IMPARES, o sea la traspuesta."""
    return indice_pvr(y, x, mask, bits, salto)


def probar(nombre, PW, PH, W, H):
    bits = 0
    while (1 << bits) < min(PW, PH):
        bits += 1
    mask, salto = min(PW, PH) - 1, min(PW, PH) ** 2

    distintos = al_reves = 0
    for y in range(H):
        for x in range(W):
            pvr = indice_pvr(x, y, mask, bits, salto)
            if indice_nuestro(x, y, mask, bits, salto) != pvr:
                distintos += 1
            if indice_viejo_al_reves(x, y, mask, bits, salto) != pvr:
                al_reves += 1

    total = W * H
    print(f"{nombre}: textura {PW}x{PH}, lado menor {min(PW, PH)} (mask {mask}, bits {bits}, salto {salto})")
    print(f"  pixeles visibles comparados ({W}x{H}): {total}")
    print(f"  indices NUESTROS distintos del PVR: {distintos}")
    print(f"  indices con la Y en los impares (el defecto): {al_reves} distintos del PVR "
          f"({100.0 * al_reves / total:.1f}%)")
    return distintos == 0 and al_reves > 0


def revisar_la_fuente():
    """Lo que la formula de Python de arriba NO puede probar: que el C de verdad sea el que dice.

    El indice de Python es un ESPEJO de lo que hace pvr_pantalla.c: si el espejo se escribe bien y el C esta
    al reves, el espejo no se entera (es la trampa de comparar algo contra si mismo). Asi que aca se mira el
    C: (1) que la tabla del twiddle tenga al menos tantas entradas como el lado menor mas grande que se pide
    (1024: la textura del motor es 1024x1024 -- con 512 las lecturas con x0 o y0 >= 512 salen de la tabla,
    medido: 219 de los 475 bloques del cuadro del motor) y (2) que la X sea la coordenada que se corre a los
    bits impares (`twid_tabla[x...] << 1`), que es la convencion del PVR. Las dos cosas son las que cambia el
    mutador (scripts/console/mutar-twiddle-226.py)."""
    import os
    import re

    ruta = os.path.join(os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__)))),
                        "console/dc/std/csrc/pvr_pantalla.c")
    with open(ruta, "r", encoding="utf-8") as f:
        t = f.read()

    m = re.search(r"#define TWID_TABLA_N (\d+)", t)
    if not m:
        print("FUENTE: no encontre `#define TWID_TABLA_N` en pvr_pantalla.c")
        return False
    n = int(m.group(1))
    necesita = 1024          # el lado menor de la textura del motor (800x600 -> 1024x1024)
    ok_tabla = n >= necesita
    print(f"FUENTE pvr_pantalla.c: la tabla del twiddle tiene {n} entradas (el motor necesita {necesita})")
    print(f"  {n} >= {necesita}: {'OK' if ok_tabla else 'NO: las lecturas con x0 o y0 >= 512 salen de la tabla'}")

    ok_x = ("twid_tabla[x0 & twid_mask] << 1" in t) and ("twid_tabla[xx] << 1 | fila" in t)
    print(f"  la X va a los bits impares (`twid_tabla[x...] << 1`): {'OK' if ok_x else 'NO: esta al reves'}")
    return ok_tabla and ok_x


def main():
    # el POC de la pantalla (640x480 -> textura 1024x512) y EL MOTOR (800x600 -> textura 1024x1024)
    ok1 = probar("POC 640x480", 1024, 512, 640, 480)
    ok2 = probar("MOTOR 800x600", 1024, 1024, 800, 600)
    ok3 = revisar_la_fuente()
    ok = ok1 and ok2 and ok3
    print("RESULTADO:", "OK (el twiddle de la pantalla es el del PVR y la tabla lo cubre)" if ok
          else "NO COINCIDE")
    raise SystemExit(0 if ok else 1)


if __name__ == "__main__":
    main()

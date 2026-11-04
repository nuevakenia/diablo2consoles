#!/usr/bin/env python3
"""DC-002.146d -- valida el predicado de FORMA del recolector contra los descriptores REALES de un ELF.

Uso: python3 validar-forma-tipos-146d.py <prog.elf> [--mascara 0x1f]

Los descriptores reales son los simbolos `..d` que emite gccgo (un `struct __go_type_descriptor` de 36 B por tipo).
Para cada uno lee sus campos de la imagen y aplica el MISMO chequeo que `gc_tipo_con_forma()` de libgodc:
    __size <= 1 MB, __ptrdata <= __size, kind <= 26, __align in {1,2,4,8}, __gcdata en la imagen si != NULL
mas el de `gc_tipo_en_imagen` (4-alineado y adentro de la imagen).

Para que sirve: el veredicto INVALIDO de la tabla de tipos se apoya en que un descriptor real NUNCA da INVALIDO. Si
el predicado rechazara descriptores reales, el recolector dejaria de barrer objetos sanos. Con la mascara correcta
(`--mascara 0x1f`, el valor por omision) el rechazo tiene que ser 0 de N. Con la mascara que se probo primero
(`--mascara ~0x60`, o sea quitarle a `__code` los bits 0x20 y 0x40 pero no el 0x80) se rechazan cientos:
gccgo prende en `__code` bits que no son el Kind.

Medido (DC-002.146d):
    demo146d16  (ELF del demo con el arreglo): & 0x1F -> 0 de 1684   |  ~0x60 -> 585 de 1684
    demo146d16r2 (repeticion del criterio)   : & 0x1F -> 0 de 1684   |  ~0x60 -> 585 de 1684
O sea: el predicado de FORMA no confunde ningun descriptor real, y la mascara de `__code` no era un detalle.
"""
import argparse
import struct
import subprocess
import sys

BASE_IMAGEN = 0x8C010000   # el arranque de la imagen de un binario de Dreamcast (1ST_READ.BIN)


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("elf")
    ap.add_argument("--mascara", default="0x1f",
                    help="mascara que se le aplica a __code para sacar el Kind (por omision 0x1f; probar ~0x60)")
    ap.add_argument("--tope", type=lambda s: int(s, 0), default=None,
                    help="fin de la imagen (por omision, el de las secciones del ELF)")
    ap.add_argument("--ver", type=int, default=0, help="cuantos descriptores rechazados listar (0 = ninguno)")
    args = ap.parse_args()
    mascara = int(args.mascara, 0) & 0xFFFFFFFF
    if mascara & 0xFF000000:          # ~0x60 viene negativo/0xFFFFFF9F: dejarlo como mascara de 32 bits
        mascara &= 0xFFFFFFFF

    ruta = args.elf
    sal = subprocess.run(["readelf", "-SW", ruta], capture_output=True, text=True).stdout
    secs = []
    for linea in sal.splitlines():
        if not linea.strip().startswith("["):
            continue
        c = linea.replace("[", " [").split()
        try:
            nombre, tipo = c[2], c[3]
            direc, desp, tam = int(c[4], 16), int(c[5], 16), int(c[6], 16)
        except (IndexError, ValueError):
            continue
        secs.append((nombre, tipo, direc, desp, tam))

    tope = args.tope
    if tope is None:
        tope = max(d + t for (_, ti, d, _, t) in secs if ti in ("PROGBITS", "NOBITS") and d >= BASE_IMAGEN)
    print("imagen: 0x%08x .. 0x%08x" % (BASE_IMAGEN, tope))
    print("mascara de __code: 0x%08x" % mascara)

    datos = open(ruta, "rb").read()
    sims = subprocess.run(["readelf", "-sW", ruta], capture_output=True, text=True).stdout
    descriptores = []
    for linea in sims.splitlines():
        c = linea.split()
        if len(c) >= 8 and c[7].endswith("..d"):
            try:
                descriptores.append((c[7], int(c[1], 16)))
            except ValueError:
                pass
    print("simbolos ..d: %d" % len(descriptores))

    def leer(addr, n):
        for nombre, tipo, direc, desp, tam in secs:
            if tipo in ("PROGBITS", "NOBITS") and direc <= addr < direc + tam:
                if tipo == "NOBITS":
                    return b"\0" * n
                return datos[desp + (addr - direc):desp + (addr - direc) + n]
        return None

    def forma(p):
        size, ptrdata, _h = struct.unpack("<III", p[0:12])
        code = p[15]
        align = p[13]
        _e, gcdata, _r, _u, _p = struct.unpack("<IIIII", p[16:36])
        kind = code & mascara
        ok = []
        if size > 1024 * 1024:
            ok.append("size=%d" % size)
        if ptrdata > size:
            ok.append("ptrdata=%d>size=%d" % (ptrdata, size))
        if kind > 26:
            ok.append("kind=%d" % kind)
        if align not in (1, 2, 4, 8):
            ok.append("align=%d" % align)
        if gcdata and not (BASE_IMAGEN <= gcdata < tope):
            ok.append("gcdata=0x%08x" % gcdata)
        return ok, dict(code=code)

    malos = []
    alineacion = {}
    kinds = {}
    for nombre, addr in descriptores:
        alineacion[addr & 3] = alineacion.get(addr & 3, 0) + 1
        if addr & 3:
            malos.append((nombre, addr, ["no 4-alineado"]))
            continue
        p = leer(addr, 36)
        if p is None or len(p) < 36:
            malos.append((nombre, addr, ["fuera de las secciones"]))
            continue
        fallos, d = forma(p)
        kinds[d["code"]] = kinds.get(d["code"], 0) + 1
        if fallos:
            malos.append((nombre, addr, fallos))

    print("alineacion (addr & 3):", alineacion)
    print("__code (con banderas):", dict(sorted(kinds.items(), key=lambda kv: -kv[1])[:12]))
    print("descriptores REALES que el predicado de FORMA RECHAZARIA: %d de %d" % (len(malos), len(descriptores)))
    for nombre, addr, fallos in malos[:args.ver]:
        print("  0x%08x %-70s %s" % (addr, nombre[-70:], ", ".join(fallos)))
    return 0 if not malos else 1


if __name__ == "__main__":
    sys.exit(main())

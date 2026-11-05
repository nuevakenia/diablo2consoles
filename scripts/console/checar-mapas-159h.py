#!/usr/bin/env python3
"""DC-002.159h -- EL CHEQUEO ESTATICO DE LOS MAPAS: con la regla de `godc_codigo_gccgo`, ¿queda llamable cada
hasher y cada igualdad de cada tipo de mapa del binario?

La regla del arreglo (`scripts/console/fix-libgodc-funcval-159h.py`):
  * si el puntero del descriptor es una de las funciones que aporta libgodc (la lista cerrada), es CODIGO y se
    llama derecho -> tiene que caer en `.text`;
  * si no, es un FuncVal del compilador -> su PRIMERA palabra tiene que caer en `.text`.

Esto se corre sobre el ELF ya enlazado y recorre TODOS los descriptores de tipo de mapa (y la `__equalfn` de su
tipo de clave). No ejecuta nada: es la prueba de que la lista cubre el binario entero.

Uso: python3 checar-mapas-159h.py <prog.elf>
"""
import struct
import subprocess
import sys

NM = "/home/dgalli/dreamcast/sh-elf/bin/sh-elf-nm"

# la MISMA lista que el arreglo (si cambia alla, cambia aca: lo dice el propio arreglo en su cabecera)
FUNCIONES_DE_LIBGODC = [
    "memhash0..f", "memhash8..f", "memhash16..f", "memhash32..f", "memhash64..f", "memhash128..f",
    "strhash..f", "f32hash..f", "f64hash..f", "interhash", "interhash..f", "nilinterhash",
    "nilinterhash..f",
    "memequal", "memequal0", "memequal0..f", "memequal8..f", "memequal16..f", "memequal32..f",
    "memequal64..f", "memequal128..f", "strequal..f", "f32equal..f", "f64equal..f", "c64equal..f",
    "c128equal..f", "interequal..f", "nilinterequal..f",
]

path = sys.argv[1]
data = open(path, "rb").read()
(shoff,) = struct.unpack_from("<I", data, 0x20)
shentsize, shnum = struct.unpack_from("<HH", data, 0x2E)
secs = []
for i in range(shnum):
    off = shoff + i * shentsize
    name, typ, flags, a, offset, size = struct.unpack_from("<IIIIII", data, off)
    secs.append(dict(name=name, typ=typ, addr=a, off=offset, size=size))
shstr = secs[-1]


def sname(s):
    st = shstr["off"] + s["name"]
    return data[st:data.index(b"\0", st)].decode()


for s in secs:
    s["sname"] = sname(s)


def read(a, n):
    for s in secs:
        if s["size"] and s["addr"] <= a and a + n <= s["addr"] + s["size"]:
            return data[s["off"] + (a - s["addr"]):s["off"] + (a - s["addr"]) + n]
    return None


def seccion(a):
    for s in secs:
        if s["size"] and s["addr"] <= a < s["addr"] + s["size"]:
            return s["sname"]
    return "?"


tree = []
out = subprocess.run([NM, "-n", path], capture_output=True, text=True).stdout
for line in out.splitlines():
    p = line.split()
    if len(p) == 3:
        try:
            tree.append((int(p[0], 16), p[1], p[2]))
        except ValueError:
            pass
tree.sort()
por_nombre = {}
for a, t, n in tree:
    por_nombre.setdefault(n, a)


def quien(a):
    if not a:
        return "-"
    lo, hi, best = 0, len(tree) - 1, None
    while lo <= hi:
        mid = (lo + hi) // 2
        if tree[mid][0] <= a:
            best = tree[mid]
            lo = mid + 1
        else:
            hi = mid - 1
    return f"{best[2]}+{a - best[0]:#x}" if best else "?"


def gostr(a):
    raw = read(a, 8)
    if not raw:
        return None
    ptr, ln = struct.unpack("<II", raw)
    if ln > 400:
        return None
    b = read(ptr, ln)
    return b.decode("utf-8", "replace").replace("\t", "/") if b else None


def nombre_tipo(a):
    st = read(a + 24, 4)
    return gostr(struct.unpack("<I", st)[0]) if st else None


lista = {}
for n in FUNCIONES_DE_LIBGODC:
    a = por_nombre.get("_runtime." + n)
    if a is None:
        print(f"AVISO: la lista nombra _runtime.{n} y el ELF no lo tiene")
        continue
    lista[a] = n

rd = [s for s in secs if s["sname"] == ".rodata"][0]
blob = data[rd["off"]:rd["off"] + rd["size"]]

mapas = []
for off in range(0, rd["size"] - 60, 4):
    bucket, = struct.unpack_from("<I", blob, off + 44)
    if not (rd["addr"] < bucket < rd["addr"] + rd["size"]):
        continue
    nb = nombre_tipo(bucket)
    if not nb or not nb.startswith("struct { topbits"):
        continue
    key, = struct.unpack_from("<I", blob, off + 36)
    val, = struct.unpack_from("<I", blob, off + 40)
    hasher, = struct.unpack_from("<I", blob, off + 48)
    flags, = struct.unpack_from("<I", blob, off + 56)
    tam_valor = 0
    cab = read(val, 4)
    if cab:
        (tam_valor,) = struct.unpack("<I", cab)
    mapas.append(dict(addr=rd["addr"] + off, clave=nombre_tipo(key), valor=nombre_tipo(val),
                      key=key, hasher=hasher, flags=flags, tam_valor=tam_valor))


def destino(puntero):
    """aplica la regla: devuelve (direccion_llamable, como_se_resolvio)"""
    if puntero == 0:
        return 0, "NULL (tipo no comparable)"
    if puntero in lista:
        return puntero, f"codigo de libgodc ({lista[puntero]})"
    raw = read(puntero, 4)
    if not raw:
        return 0, "NO SE PUDO LEER"
    w, = struct.unpack("<I", raw)
    return w, f"FuncVal en {seccion(puntero)}: su palabra 0"


malos = []
por_origen = {}
for m in mapas:
    d, como = destino(m["hasher"])
    ok = d != 0 and seccion(d) == ".text"
    por_origen["hasher " + como.split(" (")[0]] = por_origen.get("hasher " + como.split(" (")[0], 0) + 1
    if not ok:
        malos.append(("HASHER", m, d, como))
        continue
    # la igualdad del tipo de clave
    eq_raw = read(m["key"] + 16, 4)
    if not eq_raw:
        continue
    eq, = struct.unpack("<I", eq_raw)
    if eq == 0:
        continue
    d2, como2 = destino(eq)
    por_origen["eq " + como2.split(" (")[0]] = por_origen.get("eq " + como2.split(" (")[0], 0) + 1
    if d2 == 0 or seccion(d2) != ".text":
        malos.append(("IGUALDAD", m, d2, como2))

print(f"{len(mapas)} descriptores de tipo de mapa en {path}")
print()
for k in sorted(por_origen):
    print(f"   {por_origen[k]:4d}  {k}")
print()

# DC-002.159h, el SEGUNDO defecto: los mapas con VALOR INDIRECTO (`__flags` bit 1) guardan un puntero y el valor
# de verdad mide `__elem->__size`. `mapassign` copiaba `__valuesize` (= 4) y devolvia el cajon vacio: el mapa
# andaba pero TODO valor leido volvia cero. Aca se cuentan los mapas que caen en ese camino.
indirectos = [m for m in mapas if m["flags"] & 2]
grandes = [m for m in indirectos if m["tam_valor"] > 8]
print(f"   {len(indirectos):4d}  mapas con VALOR INDIRECTO (__flags bit 1): el arreglo de mapassign les pide la celda")
print(f"   {len(grandes):4d}  de esos, con valor de mas de 8 B (los que devolvian cero antes del arreglo)")
for m in grandes[:8]:
    print(f"          map[{m['clave']}]{m['valor']}  ({m['tam_valor']} B)")
if len(grandes) > 8:
    print(f"          ... y {len(grandes) - 8} mas")
print()
if malos:
    print(f"ROJO: {len(malos)} cosas no quedan llamables")
    for que, m, d, como in malos[:20]:
        print(f"   {que} de map[{m['clave']}]{m['valor']}")
        print(f"      puntero={m['hasher'] if que == 'HASHER' else 'ver equalfn'}#x  ->  {como}  = {d:#x} [{seccion(d)}]")
    sys.exit(1)

print(f"VERDE: los {len(mapas)} mapas del binario quedan con hasher llamable (y la igualdad de la clave tambien)")

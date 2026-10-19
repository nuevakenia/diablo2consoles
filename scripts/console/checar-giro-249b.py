#!/usr/bin/env python3
# DC-002.49b -- EL GIRO, medido por el canal del LOG: que direccion del COF se dibujo, sin captura.
#
# COMO MIDE (y por que este canal vale)
# -------------------------------------
# El POC imprime en cada cuadro `HITO249: ANIM modo <M> cuadro <K> ... caja <A> <B> ... crc32 <C>`, y el
# verificador del host imprime la tabla `CRC249B: <M> dir=<D> cuadro=<K> ... crc32=<C>` de TODOS los
# compuestos de cada direccion. Medido en DC-002.49b: los 256 compuestos de TN dan 256 crc32 DISTINTOS
# (0 colisiones) y las cajas solo 14 -> el crc32 dice la direccion y el cuadro, la caja NO.
# Con eso la direccion dibujada se lee del log y se cruza con el canal de los pixeles (las capturas).
#
# QUE COMPRUEBA
#   1. COMPOSICION: `HITO249B: DIRECCIONES modo <M> pedidas <N> dirsCOF <D>` y una linea
#      `HITO249B: DIR modo <M> dir <R>` por direccion compuesta. Las de TN tienen que ser LAS 8 que el
#      control puede pedir (medido: [0 1 4 7 8 9 12 15]).
#   2. DIBUJO: cada `crc32` de las lineas ANIM tiene que estar en la tabla del host (si no, se dibujo algo
#      que el compositor del host no reproduce: rojo).
#   3. EL GIRO LLEGA AL DIBUJO: despues de cada `HITO249B: GIRO dir N`, el siguente ANIM tiene que decodificar
#      a la direccion N. Es el criterio 1 de la tarjeta (la cruceta gira el dibujo), medido en el motor.
#   4. EL MUTANTE (`--mutante`): con `giroSordo=true` NO puede haber ninguna linea GIRO y TODO tiene que
#      decodificar a la direccion 0. Si aparece otra direccion, el mutante no esta mutando.
#
# Uso:
#   checar-giro-249b.py --run /tmp/dc-run-x --tabla /tmp/x/crc249b.txt
#   checar-giro-249b.py --run <corrida> --tabla <tabla> --mutante
#   checar-giro-249b.py --autoprueba --tabla <tabla>    # prueba ESTE lector: 1 verde y 2 rojos esperados
#
# Salida: una linea por comprobacion (OK / FALLA) con los numeros; exit 0 solo si todas dan OK.
import argparse
import re
import sys
from pathlib import Path

ESPERADAS_DEF = "0,1,4,7,8,9,12,15"   # medido en DC-002.49b: las 8 direcciones de los 8 estados de la cruceta
MODO_QUIETO = "TN"

RE_ANIM = re.compile(r"HITO249: ANIM modo (\S+) cuadro (\d+) de (\d+) caja (\d+) (\d+) esquina (-?\d+) (-?\d+) "
                     r"pixeles (\d+) crc32 (\d+)")
RE_DIR = re.compile(r"HITO249B: DIR modo (\S+) dir (\d+)")
RE_DIRS = re.compile(r"HITO249B: DIRECCIONES modo (\S+) pedidas (\d+) dirsCOF (\d+)")
RE_GIRO = re.compile(r"HITO249B: GIRO dir (\d+)")
RE_CRC = re.compile(r"CRC249B: (\S+) dir=(\d+) cuadro=(\d+) caja=(\d+x\d+) esquina=(-?\d+),(-?\d+) "
                    r"pixeles=(\d+) crc32=(\d+)")


def leer_tabla(ruta):
    """{(modo, crc): (dir, cuadro)} y detecta colisiones dentro de cada modo."""
    por_crc, colisiones, total = {}, [], 0
    for linea in Path(ruta).read_text(errors="replace").splitlines():
        m = RE_CRC.search(linea)
        if not m:
            continue
        modo, d, k, crc = m.group(1), int(m.group(2)), int(m.group(3)), int(m.group(8))
        total += 1
        clave = (modo, crc)
        if clave in por_crc and por_crc[clave] != (d, k):
            colisiones.append((modo, crc, por_crc[clave], (d, k)))
        por_crc[clave] = (d, k)
    return por_crc, colisiones, total


def leer_log(ruta):
    anim, dirs, dirs_pedidas, giros = [], {}, {}, []
    for n, linea in enumerate(Path(ruta).read_text(errors="replace").splitlines(), 1):
        if (m := RE_ANIM.search(linea)):
            anim.append({"n": n, "modo": m.group(1), "cuadro": int(m.group(2)), "de": int(m.group(3)),
                         "caja": (int(m.group(4)), int(m.group(5))), "crc": int(m.group(9))})
            continue
        if (m := RE_DIRS.search(linea)):
            dirs_pedidas[m.group(1)] = (int(m.group(2)), int(m.group(3)))
            continue
        if (m := RE_DIR.search(linea)):
            dirs.setdefault(m.group(1), set()).add(int(m.group(2)))
            continue
        if (m := RE_GIRO.search(linea)):
            giros.append({"n": n, "dir": int(m.group(1))})
    return anim, dirs, dirs_pedidas, giros


def comprobar(anim, dirs, dirs_pedidas, giros, tabla, esperadas, mutante):
    fallas = []

    def ok(cond, texto):
        print(("   OK    " if cond else "   FALLA ") + texto)
        if not cond:
            fallas.append(texto)

    # ---- 1. composicion
    print("== 1. las direcciones compuestas al arranque")
    ok(MODO_QUIETO in dirs, f"el POC informo las direcciones compuestas de {MODO_QUIETO} (lineas DIR)")
    for modo in sorted(dirs_pedidas):
        pedidas, dirs_cof = dirs_pedidas[modo]
        compuestas = sorted(dirs.get(modo, set()))
        print(f"   {modo}: pedidas={pedidas} compuestas={len(compuestas)} {compuestas} dirsCOF={dirs_cof}")
        ok(pedidas == len(compuestas), f"{modo}: pedidas ({pedidas}) == las compuestas informadas ({len(compuestas)})")
    if MODO_QUIETO in dirs:
        ok(sorted(dirs[MODO_QUIETO]) == sorted(esperadas),
           f"{MODO_QUIETO}: las compuestas son las 8 del control {sorted(esperadas)} "
           f"(son {sorted(dirs[MODO_QUIETO])})")

    # ---- 2. dibujo: todo crc tiene que existir en la tabla del host
    print("== 2. los cuadros dibujados son compuestos que el host reproduce")
    desconocidos = [a for a in anim if (a["modo"], a["crc"]) not in tabla]
    ok(len(anim) > 0, f"hay cuadros dibujados en el log ({len(anim)} lineas ANIM)")
    ok(not desconocidos, f"crc32 desconocidos: {len(desconocidos)}" +
       ("" if not desconocidos else f" (el primero: {desconocidos[0]}"))

    dibujadas = {}
    for a in anim:
        if (a["modo"], a["crc"]) in tabla:
            d, k = tabla[(a["modo"], a["crc"])]
            a["dir"], a["cuadroHost"] = d, k
            dibujadas.setdefault(a["modo"], {}).setdefault(d, set()).add(k)

    for modo in sorted(dibujadas):
        dirs_d = sorted(dibujadas[modo])
        print(f"   {modo}: {len(anim)} cuadros, {len(dirs_d)} direcciones dibujadas {dirs_d}, "
              f"cuadros distintos {sorted(set(k for kk in dibujadas[modo].values() for k in kk))[:20]}")

    # ---- 3. el giro llega al dibujo
    print("== 3. despues de cada GIRO dir N, el ANIM dibuja la direccion N")
    if mutante:
        ok(not giros, f"mutante: no hay lineas GIRO ({len(giros)})")
        dirs_todas = sorted(set(a["dir"] for a in anim if "dir" in a))
        ok(dirs_todas == [0] if dirs_todas else False,
           f"mutante: TODO decodifica a la direccion 0 (son {dirs_todas})")
    else:
        ok(len(giros) > 0, f"hay lineas GIRO (la cruceta giro {len(giros)} veces)")
        dirs_giradas = sorted(set(g["dir"] for g in giros))
        ok(len(dirs_giradas) >= 2, f"se dibujaron al menos 2 direcciones distintas al girar {dirs_giradas}")
        desvios, comprobados = [], 0
        for g in giros:
            sig = next((a for a in anim if a["n"] > g["n"] and "dir" in a), None)
            if sig is None:
                continue
            comprobados += 1
            if sig["dir"] != g["dir"]:
                desvios.append((g["dir"], sig["dir"], sig["cuadro"]))
        ok(comprobados > 0 and not desvios,
           f"el primer cuadro dibujado despues del GIRO es de esa direccion: {comprobados - len(desvios)}/{comprobados}"
           + ("" if not desvios else f" desvios={desvios[:5]}"))

    # ---- 4. la cadencia no cambia (los ms por cuadro del log, contra el reloj)
    print("== 4. la cadencia del ciclo (el giro no la toca)")
    for modo in sorted(dibujadas):
        ms = [a for a in anim if a["modo"] == modo]
        if len(ms) < 3:
            continue
        cambios = sorted(set(a["cuadro"] for a in ms))
        print(f"   {modo}: {len(ms)} marcas, cuadros distintos {len(cambios)} -- la cadencia la mide medir-anim-249.py")
    return fallas


def autoprueba(tabla):
    """Prueba ESTE lector con logs armados con los crc32 REALES de la tabla (no inventados): un log bueno
    (tiene que dar VERDE) y dos malos (tienen que dar ROJO). Si el bueno sale rojo o un malo sale verde, el
    instrumento es el que esta mal."""
    if MODO_QUIETO not in {m for (m, _) in tabla}:
        print(f"FALLA la tabla no trae el modo {MODO_QUIETO}")
        return 1

    def crc(modo, d, k):
        for (mm, c), (dd, kk) in tabla.items():
            if mm == modo and dd == d and kk == k:
                return c
        raise SystemExit(f"la tabla no trae {modo} dir={d} cuadro={k}")

    esperadas = [int(x) for x in ESPERADAS_DEF.split(",")]
    cab = [f"HITO249B: DIRECCIONES modo {MODO_QUIETO} pedidas {len(esperadas)} dirsCOF 16"]
    cab += [f"HITO249B: DIR modo {MODO_QUIETO} dir {d}" for d in esperadas]

    def anim(d, k):
        return (f"HITO249: ANIM modo {MODO_QUIETO} cuadro {k} de 16 caja 26 77 esquina 0 0 "
                f"pixeles 1000 crc32 {crc(MODO_QUIETO, d, k)}")

    bueno = cab + ["HITO249B: GIRO dir 0 ejes 1 0 cuadroDelLoop 1 mutante false", anim(0, 0),
                   "HITO249B: GIRO dir 1 ejes 1 1 cuadroDelLoop 40 mutante false", anim(1, 0), anim(1, 1),
                   "HITO249B: GIRO dir 12 ejes -1 1 cuadroDelLoop 80 mutante false", anim(12, 0)]
    malo_dir = cab + ["HITO249B: GIRO dir 1 ejes 1 1 cuadroDelLoop 40 mutante false", anim(0, 0)]
    malo_comp = [f"HITO249B: DIRECCIONES modo {MODO_QUIETO} pedidas 1 dirsCOF 16",
                 f"HITO249B: DIR modo {MODO_QUIETO} dir 0", anim(0, 0)]
    malo_sordo = cab + ["HITO249B: GIRO dir 1 ejes 1 1 cuadroDelLoop 40 mutante false", anim(1, 0)]

    casos = [("log bueno (gira y dibuja la direccion girada)", bueno, esperadas, False, True),
             ("log con GIRO dir 1 y el cuadro de la 0", malo_dir, esperadas, False, False),
             ("log que compone 1 sola direccion", malo_comp, esperadas, False, False),
             ("log de mutante que igual gira", malo_sordo, esperadas, True, False)]

    print(f"== AUTOPRUEBA del lector (crc32 REALES de la tabla; {len(casos)} casos)")
    bien = 0
    for nombre, lineas, esp, mut, verde_esperado in casos:
        tmp = Path("/tmp") / f"dc249b-autoprueba-{abs(hash(nombre)) % 100000}.log"
        tmp.write_text("\n".join(lineas) + "\n")
        a, d, dp, g = leer_log(tmp)
        print(f"-- {nombre} (se espera {'VERDE' if verde_esperado else 'ROJO'})")
        fallas = comprobar(a, d, dp, g, tabla, esp, mut)
        salio_verde = not fallas
        if salio_verde == verde_esperado:
            bien += 1
            print("   => como se esperaba")
        else:
            print("   => EL INSTRUMENTO ESTA MAL")
    print(f"autoprueba: {bien}/{len(casos)}")
    return 0 if bien == len(casos) else 1


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--run", help="directorio de la corrida (busca flycast.log) o el log mismo")
    ap.add_argument("--tabla", required=True, help="archivo con las lineas CRC249B del host (verif-personaje-238 -crc-dirs)")
    ap.add_argument("--esperadas", default=ESPERADAS_DEF, help=f"las direcciones que el control pide (def {ESPERADAS_DEF})")
    ap.add_argument("--mutante", action="store_true", help="el log es del build con giroSordo=true")
    ap.add_argument("--autoprueba", action="store_true", help="prueba el lector con logs armados de la tabla")
    args = ap.parse_args()

    tabla, colisiones, total = leer_tabla(args.tabla)
    print(f"tabla del host: {args.tabla} -- {total} compuestos, {len(tabla)} crc32 distintos")
    if colisiones:
        print(f"   FALLA la tabla tiene {len(colisiones)} colisiones de crc32 dentro de un modo: {colisiones[:3]}")
        return 1
    print(f"   OK    0 colisiones: el crc32 dice (modo, direccion, cuadro) sin ambiguedad")

    if args.autoprueba:
        return autoprueba(tabla)

    if not args.run:
        print("FALTA --run (o --autoprueba)")
        return 2

    log = Path(args.run)
    if log.is_dir():
        log = log / "flycast.log"
    if not log.is_file():
        print(f"FALLA no existe {log}")
        return 1

    anim, dirs, dirs_pedidas, giros = leer_log(log)
    print(f"log: {log} -- {len(anim)} marcas ANIM, {len(giros)} GIRO")

    esperadas = [int(x) for x in args.esperadas.split(",")]
    fallas = comprobar(anim, dirs, dirs_pedidas, giros, tabla, esperadas, args.mutante)

    print()
    if fallas:
        print(f"ROJO: {len(fallas)} comprobaciones fallaron")
        for f in fallas:
            print(f"   - {f}")
        return 1
    print("VERDE: el giro se dibujo (direccion del COF en el log, cruzada con la tabla del host)")
    return 0


if __name__ == "__main__":
    sys.exit(main())

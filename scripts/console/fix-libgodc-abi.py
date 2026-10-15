#!/usr/bin/env python3
"""Arregla en libgodc la firma de ifaceE2I2, que no coincide con la convencion de llamada de gccgo 15.

SINTOMA
    En la Dreamcast, cualquier asercion de interfaz sin nombre sobre un valor que viene de OTRO
    paquete falla: `if x, ok := v.(error); ok` da ok=false aunque el tipo implemente la interfaz.
    Con la biblioteca estandar encima, ademas, el programa se cae con "Illegal instruction" en una
    direccion que cae dentro de un descriptor de tipo (escribir en basura).

CAUSA
    gccgo 15 devuelve los dos valores (interfaz + ok) POR MEMORIA: el llamador reserva 12 bytes y
    pasa la direccion del buffer en r2. libgodc declaraba la funcion con punteros de salida
    (Iface *, bool *), que el compilador de C coloca en r7 y en la pila: la funcion escribia el
    resultado en basura y el llamador leia un buffer sin tocar.

    Comprobacion (misma llamada escrita en C, compilada con sh-elf-gcc -S):
        forma A (devolver la estructura por valor):  add #-12,r15 / jsr / mov r15,r2   <- buffer en r2
        forma B (punteros de salida, como libgodc):  mov r15,r7 / mov.l r1,@r15        <- r7 y pila
    El desensamblado del llamador generado por gccgo usa la forma A.

ARREGLO
    Que la funcion devuelva la estructura por valor, tanto en la definicion
    (runtime/interface_dreamcast.c) como en la declaracion (runtime/runtime.h).

USO
    python3 scripts/console/fix-libgodc-abi.py [ruta_libgodc]
    Despues: make -C <libgodc> libgodc.a   (y reenlazar los POC)

ARREGLADOS ACA (forma verificada en el desensamblado del llamador real)
    ifaceE2I2   interfaz{} -> interfaz     iface_ret {Iface, bool}    12 B, buffer en r2
    ifaceI2I2   interfaz   -> interfaz     iface_ret {Iface, bool}    12 B, buffer en r2
    ifaceI2T2P  interfaz   -> puntero      ptr_ret   {void *, bool}   8 B,  r0/r1
    La forma de 8 bytes se comprobo compilando en C una funcion que devuelve {void *, _Bool}: el
    llamador lee el valor en r0 y el ok en r1 (igual que el llamador de strconv.Atoi).

YA ESTABAN BIEN (mirados en el desensamblado, no se tocan)
    ifaceE2T2   bool f(typ, Eface, void *ret)  -> argumentos r4 / r5-r6 / r7, bool en r0
    ifaceeq     bool f(Iface, Iface)           -> argumentos r4-r5 / r6-r7, bool en r0
    assertitab  void *f(inter, typ)            -> devuelve el puntero en r0
    ifaceE2T2P  ya devolvia su estructura por valor

PENDIENTES (misma familia, sin verificar el llamador todavia)
    ifaceI2E2, ifaceE2E2, assertE2I2, assertI2I2. Metodo: desensamblar la llamada real y comparar
    con la forma que produce el compilador de C.
"""
import subprocess
import sys

LIB = sys.argv[1] if len(sys.argv) > 1 else "/home/dgalli/dreamcast/libgodc"
C = LIB + "/runtime/interface_dreamcast.c"
H = LIB + "/runtime/runtime.h"

# parte del original, para que el arreglo sea repetible
subprocess.run(["git", "-C", LIB, "checkout", "--", "runtime/interface_dreamcast.c",
                "runtime/runtime.h"], check=True)

NUEVA = """iface_ret runtime_ifaceE2I2(struct __go_type_descriptor *inter, Eface e) __asm__("_runtime.ifaceE2I2");
iface_ret runtime_ifaceE2I2(struct __go_type_descriptor *inter, Eface e)
{
    iface_ret r;
    r.i.itab = NULL;
    r.i.data = NULL;
    r.ok = false;

    if (!e.type)
    {
        // interfaz nula
        return r;
    }

    if (!implements_interface(inter, e.type))
    {
        return r;
    }

    Itab *tab = get_itab(inter, e.type);
    if (!tab)
    {
        return r;
    }

    r.i.itab = itab_methods_ptr(tab);
    r.i.data = e.data;
    r.ok = true;
    return r;
}"""

src = open(C, encoding="utf-8").read()
ini = src.index("void runtime_ifaceE2I2(struct __go_type_descriptor *inter, Eface e, Iface *ret")
llave = src.index("{", ini)
n = 0
fin = llave
for i in range(llave, len(src)):
    if src[i] == "{":
        n += 1
    elif src[i] == "}":
        n -= 1
        if n == 0:
            fin = i + 1
            break
open(C, "w", encoding="utf-8").write(src.replace(src[ini:fin], NUEVA, 1))
print("C: ifaceE2I2 reescrita (devuelve por valor)")

h = open(H, encoding="utf-8").read()
VIEJO_H = "void runtime_ifaceE2I2(struct __go_type_descriptor *inter, Eface e, Iface *ret, bool *ok);"
NUEVO_H = """// Resultado de dos valores: gccgo los devuelve por memoria y la direccion del buffer va en r2.
typedef struct iface_ret {
    Iface i;
    _Bool ok;
} iface_ret;
iface_ret runtime_ifaceE2I2(struct __go_type_descriptor *inter, Eface e);"""
if VIEJO_H in h:
    open(H, "w", encoding="utf-8").write(h.replace(VIEJO_H, NUEVO_H, 1))
    print("H: declaracion cambiada")
else:
    print("H: no encontre la declaracion vieja (ya estaba cambiada?)")

# ---------------------------------------------------------------------------------------------
# ifaceI2I2 e ifaceI2T2P: el mismo problema. Se verifico en el desensamblado que
#   - ifaceI2I2 (errors.Is) recibe el buffer de 12 bytes en r2 y lee {itab, data, ok}
#   - ifaceI2T2P (strconv.Atoi) espera el valor en r0 y el ok en r1

PTR_TYPEDEF = """// Resultado de un puntero + ok: gccgo lo devuelve en r0 (el puntero) y r1 (el ok).
typedef struct ptr_ret {
    void *p;
    _Bool ok;
} ptr_ret;
"""

h = open(H, encoding="utf-8").read()
if "typedef struct ptr_ret" not in h:
    ancla_h = "void runtime_ifaceI2E2(Iface iface, Eface *ret, bool *ok);"
    if ancla_h in h:
        h = h.replace(ancla_h, PTR_TYPEDEF + ancla_h, 1)
    h = h.replace(
        "void runtime_ifaceI2T2P(struct __go_type_descriptor *typ, Iface i, void **ret, bool *ok);",
        "ptr_ret runtime_ifaceI2T2P(struct __go_type_descriptor *typ, Iface i);", 1)
    h = h.replace(
        "void runtime_ifaceI2I2(struct __go_type_descriptor *inter, Iface i, Iface *ret, bool *ok);",
        "iface_ret runtime_ifaceI2I2(struct __go_type_descriptor *inter, Iface i);", 1)
    open(H, "w", encoding="utf-8").write(h)
    print("H: declaraciones de ifaceI2I2 e ifaceI2T2P cambiadas")


def reemplazar_funcion(src, aguja, nuevo):
    """Reemplaza la funcion que arranca en 'aguja' (cuenta llaves para el fin)."""
    ini = src.index(aguja)
    llave = src.index("{", ini)
    n = 0
    for i in range(llave, len(src)):
        if src[i] == "{":
            n += 1
        elif src[i] == "}":
            n -= 1
            if n == 0:
                return src[:ini] + nuevo + src[i + 1:]
    return None


CUERPO_I2I2 = """iface_ret runtime_ifaceI2I2(struct __go_type_descriptor *inter, Iface i) __asm__("_runtime.ifaceI2I2");
iface_ret runtime_ifaceI2I2(struct __go_type_descriptor *inter, Iface i)
{
    iface_ret r;
    r.i.itab = NULL;
    r.i.data = NULL;
    r.ok = false;

    if (!i.itab)
        return r;

    // Conversion entre interfaces: el tipo concreto esta en i.itab[0] (methods[0]).
    struct __go_type_descriptor *ctype = *((struct __go_type_descriptor **)i.itab);

    Itab *new_tab = get_itab(inter, ctype);
    if (!new_tab)
        return r;

    r.i.itab = itab_methods_ptr(new_tab);
    r.i.data = i.data;
    r.ok = true;
    return r;
}"""

CUERPO_I2T2P = """ptr_ret runtime_ifaceI2T2P(struct __go_type_descriptor *typ, Iface i) __asm__("_runtime.ifaceI2T2P");
ptr_ret runtime_ifaceI2T2P(struct __go_type_descriptor *typ, Iface i)
{
    ptr_ret r;
    r.p = NULL;
    r.ok = false;

    if (!i.itab)
        return r;

    Itab *tab = itab_from_iface(i.itab);
    if (itab_type(tab) != typ)
        return r;

    r.p = i.data;
    r.ok = true;
    return r;
}"""

src = open(C, encoding="utf-8").read()
for aguja, nuevo in (
    ("void runtime_ifaceI2I2(struct __go_type_descriptor *inter, Iface i, Iface *ret, bool *ok) __asm__(", CUERPO_I2I2),
    ("void runtime_ifaceI2T2P(struct __go_type_descriptor *typ, Iface i, void **ret, bool *ok) __asm__(", CUERPO_I2T2P),
):
    if aguja in src:
        actualizado = reemplazar_funcion(src, aguja, nuevo)
        if actualizado:
            src = actualizado
            print("C: reescrita", aguja.split("(")[0].split(" ")[-1])
    else:
        print("NO ENCONTRE:", aguja[:60])
# assertitab: devuelve el Itab crudo, pero el llamador espera el puntero al arreglo de metodos
# (asignacion directa: methods[0] = tipo, methods[1..] = las funciones). Con el Itab crudo, el
# llamador lee el "metodo" en Itab+4, que es el tipo, y salta a los datos. requireitab ya lo hacia
# bien; aca se iguala.
VIEJO_ASSERT = """        // implements_interface passed, so this is allocation failure
        runtime_panicstring("runtime: failed to allocate itab");
    }

    return itab;
}"""
NUEVO_ASSERT = """        // implements_interface passed, so this is allocation failure
        runtime_panicstring("runtime: failed to allocate itab");
    }

    // gccgo espera el puntero al arreglo de metodos, no al Itab: methods[0] = el tipo y
    // methods[1..] = las funciones. Devolver el Itab crudo hace que el llamador salte a los datos.
    return itab_methods_ptr(itab);
}"""
if VIEJO_ASSERT in src:
    src = src.replace(VIEJO_ASSERT, NUEVO_ASSERT, 1)
    print("C: assertitab devuelve el arreglo de metodos")
else:
    print("C: no encontre el final de assertitab")
open(C, "w", encoding="utf-8").write(src)

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

PENDIENTE (misma familia, ver README seccion 11)
    Los otros ayudantes que libgodc declara con punteros de salida siguen con el mismo problema:
    ifaceE2T2, ifaceI2T2, ifaceI2T2P, assertitab, ifaceT2Ip. Cada uno hay que mirar el
    desensamblado de una llamada real para saber la forma exacta que espera gccgo 15.
"""
import subprocess
import sys

LIB = sys.argv[1] if len(sys.argv) > 1 else "/home/dgalli/dreamcast/libgodc"
C = LIB + "/runtime/interface_dreamcast.c"
H = LIB + "/runtime/runtime.h"

# parte del original, para que el arreglo sea repetible
subprocess.run(["git", "-C", LIB, "checkout", "--", "runtime/interface_dreamcast.c"], check=True)

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

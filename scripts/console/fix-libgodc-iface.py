#!/usr/bin/env python3
"""Arregla en libgodc la capa de interfaces: la firma del metodo y la comparacion de dos interfaces.

SINTOMA
    En la Dreamcast, `errors.Is` no encuentra el error envuelto por `fmt.Errorf("... %w", e)` aunque la
    cadena se desenvuelve bien. El caso concreto medido: `err == target` da false con los dos operandos
    apuntando al MISMO dato.

CAUSA (son DOS defectos, y el primero tapaba al segundo)
    1. implements_interface comparaba el NOMBRE del metodo y no la FIRMA: un tipo con
       `Unwrap() []error` "implementaba" `interface{ Unwrap() error }`, el itab quedaba armado con la
       funcion equivocada y la asercion decia ok.
    2. runtime_ifaceeq comparaba los PUNTEROS de itab. El mismo par (interfaz, tipo concreto) puede
       tener dos tablas: la que arma el compilador para una conversion estatica y la que arma el
       runtime en el heap (mismo tipo y misma funcion adentro, direcciones distintas). Con eso,
       `e2 == base` daba false con los dos datos apuntando al mismo errorString.

ARREGLO
    1. `method_types_equal(...)` compara el tipo del metodo (`__typ` del imethod contra `__mtyp` del
       metodo) y `implements_interface` lo llama despues de que los nombres coinciden. Por que esos
       campos (medido con la sonda console/dc/poc/sonda-f, no deducido): `__mtyp` es el tipo del metodo
       SIN el receptor ("func() error", el que la interfaz pide) y `__typ` es el tipo CON el receptor
       como primer parametro ("func(*paq1.ConUno) error"). El descriptor del tipo de funcion es el
       MISMO puntero entre paquetes, asi que comparar por puntero no rompe las aserciones entre
       paquetes; si algun dia no coincide, se compara por forma (tipos con nombre por nombre y
       paquete, funciones por entradas y salidas, punteros/arreglos/slices/canales por el elemento,
       interfaces por sus metodos). Un tipo que no sabemos comparar NO se acepta: un falso negativo se
       ve (el corpus lo canta), un falso positivo es el defecto que estamos arreglando.
    2. `runtime_ifaceeq` compara el TIPO CONCRETO (`itab_type`) y despues el dato, que es lo que hace
       el runtime de Go: mira el tipo, no la direccion de la tabla.
    Y se corrige el comentario de `__mtyp`/`__typ` en runtime/type_descriptors.h, que los documentaba
    al reves.

USO
    python3 scripts/console/fix-libgodc-iface.py [ruta_libgodc]
    Despues: make -C <libgodc> libgodc.a   y reenlazar los POC (scripts/console/build-motor-dc6.sh)

ORDEN E IDEMPOTENCIA
    Este script deja el arbol pristino con `git checkout`, aplica PRIMERO el arreglo de ABI
    (fix-libgodc-abi.py: toca el mismo archivo) y despues los dos de aca. Correrlo dos veces deja el
    mismo resultado (lo verifica el sha256 que imprime).

QUIEN LO JUZGA
    El corpus completo de la consola: 593 casos, 593 bien, 0 mal (scripts/console/build-dc.sh +
    run-flycast.sh sobre console/dc/poc/stdlib). Y los mutantes de scripts/console/mutar-libgodc-2f.sh,
    que sacan cada arreglo por separado y exigen que el corpus se ponga rojo (2/2).
"""
import hashlib
import os
import pathlib
import subprocess
import sys

LIB = sys.argv[1] if len(sys.argv) > 1 else "/home/dgalli/dreamcast/libgodc"
AQUI = os.path.dirname(os.path.abspath(__file__))
IFACE = pathlib.Path(LIB + "/runtime/interface_dreamcast.c")
HEADER = pathlib.Path(LIB + "/runtime/type_descriptors.h")

# El arbol pristino del vendor (los dos archivos que tocamos) y despues el arreglo de ABI, que vive en
# el mismo archivo y no queremos pisar.
subprocess.run(["git", "-C", LIB, "checkout", "--",
                "runtime/interface_dreamcast.c", "runtime/type_descriptors.h"], check=True)
subprocess.run([sys.executable, os.path.join(AQUI, "fix-libgodc-abi.py"), LIB], check=True)

FUNCION = '''// Comparacion de FIRMAS de metodo (DC-002.2f).
//
// Por que no alcanza con el nombre (medido en la consola, sonda-f de DC-002.2f): un tipo con
// Unwrap() []error "implementaba" interface{ Unwrap() error }; el itab quedaba armado con la funcion
// equivocada y la asercion decia ok. Go exige las dos cosas, el nombre y la firma.
//
// El campo que corresponde al __typ del metodo de la interfaz es __mtyp (el tipo del metodo SIN el
// receptor): medido con la sonda console/dc/poc/sonda-f, el metodo definido en un paquete y el
// imethod declarado en otro dieron el MISMO puntero (func() error). El __typ del metodo es el tipo
// CON el receptor como primer parametro (func(*paq1.ConUno) error), asi que no es el que se compara.
//
// Se compara por puntero y, si no coinciden, por forma: gccgo comparte los descriptores de tipos
// equivalentes (medido entre paquetes), pero eso no esta garantizado entre unidades de compilacion.
// Un tipo que no sabemos comparar NO se acepta: un falso negativo se ve (la asercion deja de andar y
// el corpus lo canta), mientras que un falso positivo es justo el defecto que estamos arreglando.
static bool method_types_equal(const struct __go_type_descriptor *a,
                               const struct __go_type_descriptor *b)
{
    if (a == b)
    {
        return true;
    }

    if (!a || !b || a->__code != b->__code)
    {
        return false;
    }

    // Tipo con nombre: coinciden si coinciden el nombre y el paquete (como typesEqual de Go).
    if (a->__uncommon || b->__uncommon)
    {
        const struct __go_uncommon_type *ua = a->__uncommon;
        const struct __go_uncommon_type *ub = b->__uncommon;

        if (!ua || !ub)
        {
            return false;
        }

        return strings_equal_ptr(ua->__name, ub->__name) &&
               strings_equal_ptr(ua->__pkg_path, ub->__pkg_path);
    }

    switch (a->__code)
    {
    case GO_FUNC:
    {
        const struct __go_func_type *fa = (const struct __go_func_type *)a;
        const struct __go_func_type *fb = (const struct __go_func_type *)b;

        if (fa->__dotdotdot != fb->__dotdotdot ||
            fa->__in.__count != fb->__in.__count ||
            fa->__out.__count != fb->__out.__count)
        {
            return false;
        }

        for (intptr_t i = 0; i < fa->__in.__count; i++)
        {
            if (!method_types_equal(fa->__in.__values[i], fb->__in.__values[i]))
            {
                return false;
            }
        }

        for (intptr_t i = 0; i < fa->__out.__count; i++)
        {
            if (!method_types_equal(fa->__out.__values[i], fb->__out.__values[i]))
            {
                return false;
            }
        }

        return true;
    }

    case GO_PTR:
    case GO_SLICE:
    case GO_CHAN:
    case GO_ARRAY:
        // los cuatro tienen el tipo del elemento en el mismo lugar (offset 36, despues del _type)
        return method_types_equal(((const struct __go_ptr_type *)a)->__element_type,
                                  ((const struct __go_ptr_type *)b)->__element_type);

    case GO_INTERFACE:
    {
        const struct __go_interface_type *ia = (const struct __go_interface_type *)a;
        const struct __go_interface_type *ib = (const struct __go_interface_type *)b;

        if (ia->__methods_count != ib->__methods_count)
        {
            return false;
        }

        for (intptr_t i = 0; i < ia->__methods_count; i++)
        {
            const struct __go_interface_method *ma = &ia->__methods[i];
            const struct __go_interface_method *mb = &ib->__methods[i];

            if (!strings_equal_ptr(ma->__name, mb->__name) ||
                !strings_equal_ptr(ma->__pkg_path, mb->__pkg_path) ||
                !method_types_equal(ma->__typ, mb->__typ))
            {
                return false;
            }
        }

        return true;
    }

    default:
        return false;
    }
}

// Check if type implements interface'''

VIEJO_CHEQUEO = '''            // Check if names match (both are pointers to strings)
            if (strings_equal_ptr(lhsMethod->__name, rhsMethod->__name))
            {
                // NOTE: For complete Go semantics, we should also check pkgPath
                // for unexported methods. Skipped because Dreamcast games typically
                // use exported interfaces and gccgo validates this at compile time.
                break;
            }

            ri++;
        }

        // NOTE: For complete Go semantics, we should verify method types match
        // (lhsMethod->__typ == rhsMethod->__mtyp). Skipped because gccgo validates
        // method signatures at compile time for correct Go code.'''

NUEVO_CHEQUEO = '''            // El nombre solo no alcanza: la FIRMA tambien tiene que coincidir (DC-002.2f).
            if (strings_equal_ptr(lhsMethod->__name, rhsMethod->__name))
            {
                // NOTE: sigue pendiente comparar el pkgPath de los metodos no exportados.
                if (!method_types_equal(lhsMethod->__typ, rhsMethod->__mtyp))
                {
                    // Hay un metodo con ese nombre y OTRA firma: la interfaz no esta implementada.
                    return false;
                }

                break;
            }

            ri++;
        }'''

VIEJO_ITABEQ = '''bool runtime_ifaceeq(Iface i1, Iface i2)
{
    // If itabs differ, not equal
    if (i1.itab != i2.itab)
        return false;

    // Both nil
    if (!i1.itab && !i2.itab)
        return true;

    // One nil, one not
    if (!i1.data || !i2.data)
        return (i1.data == i2.data);
'''

NUEVO_ITABEQ = '''bool runtime_ifaceeq(Iface i1, Iface i2)
{
    // OJO: NO se comparan los punteros de itab. El mismo par (interfaz, tipo concreto) puede tener dos
    // tablas distintas: la que arma el compilador para una conversion estatica y la que arma el runtime
    // en el heap (medido en DC-002.2f con la sonda-g: dos punteros distintos, con la misma palabra 0 -el
    // tipo concreto- y la misma palabra 1 -la funcion del metodo-). Comparar las direcciones hacia
    // `e2 == base` falso con los dos datos apuntando al mismo errorString, que es justo lo que rompe
    // errors.Is. Lo que identifica a la interfaz es el tipo concreto, como en el runtime de Go.
    if (!i1.itab || !i2.itab)
        return (i1.itab == i2.itab) && (i1.data == i2.data);

    if (itab_type(itab_from_iface(i1.itab)) != itab_type(itab_from_iface(i2.itab)))
        return false;

    // One nil, one not
    if (!i1.data || !i2.data)
        return (i1.data == i2.data);
'''

VIEJO_HEADER = '''    struct __go_type_descriptor *__mtyp;  // offset 8: method type (receiver signature)
    struct __go_type_descriptor *__typ;   // offset 12: function type (without receiver)'''

NUEVO_HEADER = '''    // OJO: el orden de estos dos comentarios estaba al reves (medido en la consola con la sonda de
    // DC-002.2f). El que manda es el de gccgo: __mtyp es el tipo del metodo SIN el receptor
    // ("func() error", el que se compara con el __typ del imethod) y __typ es el tipo CON el
    // receptor como primer parametro ("func(*paq1.ConUno) error").
    struct __go_type_descriptor *__mtyp;  // offset 8: method type (SIN receptor)
    struct __go_type_descriptor *__typ;   // offset 12: method type (CON receptor)'''


def sha(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()[:16]


def reemplazar(path, viejo, nuevo):
    t = path.read_text()
    n = t.count(viejo)
    if n != 1:
        sys.exit("ERROR: el texto a reemplazar aparece %d veces (esperaba 1) en %s" % (n, path))
    path.write_text(t.replace(viejo, nuevo, 1))
    print("  ok %s" % path.name)


print("interface_dreamcast.c sha256=%s  type_descriptors.h sha256=%s" % (sha(IFACE), sha(HEADER)))
reemplazar(IFACE,
           "// Check if type implements interface\nstatic bool implements_interface",
           FUNCION + "\nstatic bool implements_interface")
reemplazar(IFACE, VIEJO_CHEQUEO, NUEVO_CHEQUEO)
reemplazar(IFACE, VIEJO_ITABEQ, NUEVO_ITABEQ)
reemplazar(HEADER, VIEJO_HEADER, NUEVO_HEADER)
print("listo: make -C %s libgodc.a  y reenlazar los POC" % LIB)
print("interface_dreamcast.c sha256=%s  type_descriptors.h sha256=%s" % (sha(IFACE), sha(HEADER)))

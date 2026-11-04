#!/usr/bin/env python3
"""Arregla en libgodc el `recover` que no devuelve el control: `runtime_blindaje` (DC-002.146c).

SINTOMA (medido en DC-002.146, commit 4f1e1fda)
    El `defer func(){ if r := recover(); r != nil { ... } }()` del rearme del minimapa deja el log y
    despues NO devuelve el control. Con la palanca `MINIMAPA_DIAG=panico` (un panic de rebanada de
    verdad adentro del rearme):

        /tmp/capturas-146w  (sin checkpoint)
            MINIMAPA REARME SE CAYO Y NO SE LLEVA EL CRUCE ...
            FATAL: recover without checkpoint
        /tmp/capturas-146p  (llamando `runtime_checkpoint()` antes, que es lo que el runtime pide)
            el mismo log entero y despues: kernel panic: unhandled IRQ/Exception

CAUSA
    `runtime_checkpoint()` hace `return setjmp(cp->env)` DENTRO de una funcion de C que despues
    RETORNA. El `longjmp` del `recover` vuelve a un marco de pila que ya murio (abajo de el corrio
    todo el cuerpo: el parseo, el rearme, lo que sea), asi que el SH-4 salta a basura. Sin checkpoint
    el runtime no tiene a donde saltar y mata la consola ordenadamente: por eso el recover "deja el
    log pero no vuelve".

ARREGLO
    `runtime_blindaje(cuerpo, arg)`: el `setjmp` vive en el marco de runtime_blindaje, que SIGUE EN LA
    PILA hasta que el cuerpo termina. Orden: setjmp -> empujar el checkpoint en la cadena del G ->
    llamar al cuerpo DESDE ESE MISMO MARCO -> sacar el checkpoint y volver. Con eso el `longjmp` del
    recover cae en un marco vivo y el programa sigue.

    Devuelve 0 si el cuerpo volvio solo y 1 si se volvio por el longjmp del recover.

    `runtime_blindaje_go(caja_funcion, arg)` es la puerta para Go (`//extern runtime_blindaje_go`):
    gccgo pasa un valor de funcion como puntero a una CAJA cuyo primer campo es el codigo (medido en
    el .s del POC: `mov.l @r1,r3; mov.l @r3,r0; jsr @r0`), asi que la puerta la abre y llama.

LO QUE CAMBIA EN LA SEMANTICA (declarado, no escondido)
    libgodc no desenrolla la pila: el `recover` implementado con longjmp no puede volver al punto
    donde se registro el defer. Con este blindaje, un panic recuperado adentro del cuerpo devuelve el
    control AL BLINDAJE (o sea al que llamo a runtime_blindaje), y los marcos de Go de adentro del
    cuerpo quedan muertos. Los defers de esos marcos muertos se sacan de la cadena del G al volver
    (si no, la cadena crece con marcos que ya no existen y el `defer overflow` llega antes de tiempo).
    El Checkpoint vive en la PILA de runtime_blindaje: no le pide memoria al recolector (y el rearme
    del minimapa es justo el camino que se cayo por pedirle memoria al recolector, DC-002.146).

USO
    python3 scripts/console/fix-libgodc-blindaje.py [ruta_libgodc] [--build]
    --build ademas hace `rm -f runtime/*.o *.a` y `make all` (kos.gox + libgodc.a), y hay que
    REENLAZAR los POC: libgodc.a es estatico, el binario viejo no tiene la funcion nueva.

ORDEN E IDEMPOTENCIA
    Corre el arbol pristino de los DOS archivos que toca (`git checkout`) y los vuelve a parchear:
    correrlo dos veces deja el mismo sha256 (lo imprime). OJO CON EL ORDEN: el instrumento
    `fix-libgodc-map-traza.py` tambien toca `runtime/defer_dreamcast.c` y hace `git checkout` del
    archivo: si se aplico ese instrumento, hay que volver a correr ESTE despues.

QUIEN LO JUZGA
    El POC minimo `console/dc/poc/sonda-blindaje-146c` (build `scripts/console/build-blindaje-146c.sh`):
    runtime_blindaje + panic de rebanada adentro, y la consola tiene que imprimir el cierre DESPUES del
    panic. Dos corridas en Flycast, con un control en contra (`BLINDAJE=viejo`, que usa el checkpoint
    viejo y tiene que morir con kernel panic: es la causa, aislada en 40 lineas).
"""
import hashlib
import os
import pathlib
import subprocess
import sys

args = [a for a in sys.argv[1:] if not a.startswith("--")]
LIB = args[0] if args else "/home/dgalli/dreamcast/libgodc"
DEFER = pathlib.Path(LIB + "/runtime/defer_dreamcast.c")
HEADER = pathlib.Path(LIB + "/runtime/panic_dreamcast.h")
MIS_ARCHIVOS = ["runtime/defer_dreamcast.c", "runtime/panic_dreamcast.h"]

# ------------------------------------------------------------------ el texto que se inyecta

# 1) El aviso en el comentario de runtime_checkpoint (se queda: el que arregla es runtime_blindaje).
VIEJO_CHECKPOINT_DOC = """/**
 * runtime_checkpoint - Create a recovery point for panic/recover.
 *"""

NUEVO_CHECKPOINT_DOC = """/**
 * runtime_checkpoint - Create a recovery point for panic/recover.
 *
 * OJO (DC-002.146c, MEDIDO): ESTE checkpoint NO SIRVE PARA RECOVER. Hace `setjmp` en SU marco y
 * despues RETORNA, asi que cuando el `recover` hace `longjmp` el marco ya murio y el SH-4 salta a
 * basura (medido: `kernel panic: unhandled IRQ/Exception` si hay checkpoint, `FATAL: recover without
 * checkpoint` si no lo hay). El que sirve es `runtime_blindaje()` (mas abajo), que hace el `setjmp` en
 * un marco que sigue en la pila mientras corre el cuerpo. Se deja esta funcion por historia y porque
 * `runtime_uncheckpoint()` esta declarada con ella; NO se usa para recuperar.
 *"""

# 2) El arreglo, que va justo antes de las entradas con nombre de asm (_runtime.gopanic, ...).
MARCA_INSERCION = 'void runtime_gopanic(struct __go_type_descriptor *type, void *data) __asm__("_runtime.gopanic");'

BLINDAJE = '''/* ============================== runtime_blindaje (DC-002.146c) ==============================
 * El checkpoint que SIRVE para `recover`.
 *
 * POR QUE HACE FALTA OTRA FUNCION (medido en DC-002.146): `runtime_checkpoint()` hace
 * `return setjmp(cp->env)` dentro de una funcion de C que despues RETORNA. El `longjmp` del `recover`
 * vuelve a un marco de pila que ya murio -- abajo de el corrio TODO el cuerpo (el parseo del
 * `AutoMap.txt` en el caso medido) -- asi que el SH-4 salta a basura. Sin checkpoint el runtime no
 * tiene a donde saltar y mata la consola: `FATAL: recover without checkpoint`. Con el checkpoint viejo
 * tambien muere, un poco mas tarde: `kernel panic: unhandled IRQ/Exception`.
 *
 * EL ARREGLO ES EL LUGAR DEL `setjmp`, NO EL `setjmp`: aca el `setjmp` vive en un marco que SIGUE EN LA
 * PILA hasta que el cuerpo termina. Orden: setjmp -> empujar el checkpoint en la cadena del G ->
 * llamar al cuerpo DESDE ESTE MISMO MARCO -> (cuando el cuerpo vuelve, o cuando el `longjmp` del
 * recover cae aca) sacar el checkpoint y volver. El `longjmp` vuelve a un marco VIVO.
 *
 * CONTRATO
 *   - Devuelve 0 si el cuerpo volvio solo, y el valor del `longjmp` (1) si se volvio por el desenlace
 *     de un panic que alguien recupero.
 *   - El `recover` que se lleva el panic tiene que estar en un `defer` registrado DENTRO del cuerpo (o
 *     mas abajo). El checkpoint que se consume es el de ESTE blindaje (el primero de la cadena).
 *   - Los marcos de Go de ADENTRO del cuerpo NO se desenrollan cuando se vuelve: estan muertos. Lo de
 *     arriba del blindaje queda sano (es el punto de este arreglo).
 *   - El Checkpoint vive en la PILA de este marco, no en el monton: mientras el cuerpo corre, este
 *     marco no se mueve y no se le pide memoria al recolector. (El rearme del minimapa se cayo justo
 *     por pedirle memoria al recolector a mitad del parseo: DC-002.146.)
 *
 * `gp` y `marcaDefer` son `volatile` a proposito: se leen DESPUES del `longjmp`, y el `longjmp` no
 * devuelve la vida a un registro que no este guardado en el `jmp_buf`.
 */
int runtime_blindaje(runtime_blindaje_cuerpo_t cuerpo, void *arg)
{
    G *volatile gp;
    GccgoDefer *volatile marcaDefer;
    Checkpoint cp;
    int estado;

    if (!cuerpo)
        runtime_throw("runtime_blindaje: cuerpo nil");

    gp = must_getg();

    cp.frame = panic_get_caller_frame();
    cp.link = gp->checkpoint;
    gp->checkpoint = &cp;

    /* La cabeza de los defers de ANTES del cuerpo: lo que quede ARRIBA despues de un `longjmp` es de
     * marcos que ya no existen y se saca de la cadena (si no, `defer_depth` crece con marcos muertos y
     * el `defer overflow` llega antes de tiempo). */
    marcaDefer = gp->_defer;

    estado = setjmp(cp.env);   /* 0 la primera vez; != 0 cuando vuelve el longjmp del recover */
    if (estado != 0) {
        /* Se volvio por el `longjmp`: el `gopanic` ya saco el checkpoint de la cadena. Si todavia esta
         * (otro camino lo dejo ahi), se saca aca. */
        if (gp->checkpoint == &cp)
            gp->checkpoint = cp.link;
        while (gp->_defer && gp->_defer != marcaDefer) {
            gp->_defer = gp->_defer->link;
            gp->defer_depth--;
        }
        return estado;
    }

    /* El cuerpo corre con el checkpoint de este marco VIVO. Esto -- y no el setjmp -- es el arreglo. */
    (void)cuerpo(arg);

    /* Volvio solo: sacar el checkpoint. */
    if (gp->checkpoint == &cp)
        gp->checkpoint = cp.link;
    return 0;
}

/* La puerta para Go: `//extern runtime_blindaje_go`, con una funcion de Go como primer argumento.
 *
 * gccgo NO pasa el codigo de la funcion: pasa un puntero a una CAJA cuyo PRIMER campo es el codigo
 * (para una funcion sin capturas, la caja es un objeto de 4 bytes en .rodata; medido en el .s del POC:
 * `mov.l .L98,r1; mov.l .L99,r0; jsr @r1; mov.l @r0,r4`, con `.L99: .long _main.caja`). Asi que aca se
 * abre la caja.
 *
 * OJO: la caja tiene que seguir viva mientras corre el cuerpo (para una funcion sin capturas vive en
 * .rodata y no hay problema; para un cierre con capturas, el cierre vive en el monton de Go y C no es
 * una raiz: hay que dejarlo en una variable de paquete, como hace el POC).
 */
int runtime_blindaje_go(void *caja_funcion, void *arg)
{
    if (!caja_funcion)
        runtime_throw("runtime_blindaje_go: funcion nil");

    return runtime_blindaje((runtime_blindaje_cuerpo_t)(*(void **)caja_funcion), arg);
}

'''

DECL_HEADER = '''
/* ==================== runtime_blindaje (DC-002.146c) ====================
 * El checkpoint que SI sirve para `recover`: hace el `setjmp` en un marco que sigue en la pila
 * mientras corre el cuerpo, asi que el `longjmp` del recover vuelve a un marco VIVO (el
 * `runtime_checkpoint()` de arriba hace `setjmp` en un marco que ya retorno: NO sirve, medido).
 *
 *   int runtime_blindaje(int (*cuerpo)(void *), void *arg);
 *       0 = el cuerpo volvio solo; 1 = se volvio por el `longjmp` de un panic recuperado.
 *   int runtime_blindaje_go(void *caja_funcion, void *arg);
 *       lo mismo, para llamarlo desde Go (`//extern runtime_blindaje_go`): abre la caja del valor de
 *       funcion de gccgo (primer campo = codigo) y llama.
 *
 * Los marcos de Go de ADENTRO del cuerpo no se desenrollan: cuando se vuelve, estan muertos. El que
 * recupera tiene que estar en un `defer` registrado dentro del cuerpo. Ver defer_dreamcast.c.
 */
typedef int (*runtime_blindaje_cuerpo_t)(void *);
int runtime_blindaje(runtime_blindaje_cuerpo_t cuerpo, void *arg) __attribute__((returns_twice));
int runtime_blindaje_go(void *caja_funcion, void *arg);
'''


def sha(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()[:16]


def reemplazar(path, viejo, nuevo):
    t = path.read_text()
    n = t.count(viejo)
    if n != 1:
        sys.exit("ERROR: el texto a reemplazar aparece %d veces (esperaba 1) en %s" % (n, path))
    path.write_text(t.replace(viejo, nuevo, 1))
    print("  ok %s" % path.name)


# ------------------------------------------------------------------ aplicar
subprocess.run(["git", "-C", LIB, "checkout", "--"] + MIS_ARCHIVOS, check=True)
print("arbol pristino: defer_dreamcast.c sha256=%s  panic_dreamcast.h sha256=%s" % (sha(DEFER), sha(HEADER)))

reemplazar(DEFER, VIEJO_CHECKPOINT_DOC, NUEVO_CHECKPOINT_DOC)
reemplazar(DEFER, MARCA_INSERCION, BLINDAJE + MARCA_INSERCION)

reemplazar(HEADER,
           "\n#ifdef __cplusplus\n}\n#endif\n\n#endif // PANIC_DREAMCAST_H",
           DECL_HEADER + "\n#ifdef __cplusplus\n}\n#endif\n\n#endif // PANIC_DREAMCAST_H")

print("defer_dreamcast.c sha256=%s  panic_dreamcast.h sha256=%s" % (sha(DEFER), sha(HEADER)))

if "--build" in sys.argv:
    subprocess.run(["bash", "-c", "rm -f %s/runtime/*.o %s/*.a" % (LIB, LIB)], check=True)
    r = subprocess.run(["make", "-C", LIB, "all"], capture_output=True, text=True)
    if r.returncode != 0:
        print(r.stdout[-3000:])
        print(r.stderr[-3000:])
        sys.exit("make fallo")
    print("   make ok: " + (r.stdout.strip().splitlines() or ["(sin salida)"])[-1])
    print("   libgodc.a %s B  kos.gox %s B" % (
        os.path.getsize(LIB + "/libgodc.a"), os.path.getsize(LIB + "/kos/kos.gox")))
else:
    print("falta --build: make -C %s all  (y reenlazar los POC)" % LIB)

#!/usr/bin/env python3
"""Arregla la COPIA POR STORE QUEUE del GC de libgodc (la causa del crash del motor en el cuadro 0).

SINTOMA (medido, tarjeta DC-002.24)
    El motor arranca y muere en el PRIMER cuadro del loop:

        Unhandled exception: PC 0000000b   al evaluar a.errorMessage.Error()
        (d2app.App.updateInitError, console/dc/evidencia/dc-002.23-error-del-motor.txt)

    El campo `errorMessage` del App (offset 156 de 168) amanece con basura de OTRO objeto
    (itab= 0x9 data= 0x8c912590 / itab= 0x8c4f3c60 data= 0xff000000). No lo escribe nadie del motor: lo
    escribe el GC al copiar otro objeto ENCIMA del App.

QUE PASA, PASO A PASO (con la traza del runtime, scripts/console/fix-libgodc-campo-traza.py)
    El App NO pierde el puntero: queda vigilado, el GC lo alcanza desde la pila (dentro=2) y lo copia
    (`el App SE COPIA (176 bytes) desde 8cd85028`) -- pero la copia NO ESCRIBE NADA en el destino:

        sq entrada:   dst=8cdc5040 src=8cd85020 tam=176 via_sq=1 dst_cab= 00000000 00000000 src_cab= 190000b0 8ca5f598
        sq salida-SQ: dst=8cdc5040 src=8cd85020 tam=176 via_sq=1 dst_cab= 00000000 00000000 src_cab= 190000b0 8ca5f598
                                                                        ^^^^^^^^^^^^^^^^^^^ el destino quedo EN CERO

    Con el destino en cero, el encabezado del App queda en cero y el GC siguiente lo RECHAZA:

        rechazo: el encabezado del App (8cdc5040) NO paso la validacion: tamano fuera de rango

    ... y como la pila SI lo referencia pero el GC lo da por basura, el GC siguiente copia otro objeto en
    su lugar (medido: `PISADO: el GC escribe un bloque de 288 bytes en 8cdc5048 que CUBRE el App`). Eso
    ensucia el offset 156 con la memoria del otro objeto y el motor se cae al llamar al metodo.

POR QUE LA COPIA NO ESCRIBE (la causa de fondo)
    `gc_sq_copy()` (runtime/gc_copy.c) escribe por la Store Queue eligiendo el AREA con

        *QACR0 = (dst_addr >> 26) & 0x1C;      /* MAL: 0x8cdc5040 >> 26 & 0x1C = 0x00 */

    El propio KOS de este toolchain usa

        #define QACR_EXTERN_BITS(dest) ((((uintptr_t)(dest)) >> 24) & 0x1c)   /* 0x8c... >> 24 & 0x1C = 0x0C */

    (`$KOS_BASE/kernel/arch/dreamcast/hardware/sq.c`). Con 0x00 el area es la 0 (`0x00000000-0x03FFFFFF`):
    la BIOS, de solo lectura, asi que las escrituras de la Store Queue se DESCARTAN -- medido: el destino
    queda como estaba (en cero la primera vuelta de cada semiespacio). Corriendo `>> 26` sobre la direccion
    del monton (0x8C......) el resultado es SIEMPRE 0, asi que la via rapida esta rota para todo el monton.

    Y no es solo el area: la Store Queue no pasa por la cache D, asi que el rango destino puede quedar con
    LINEAS VIEJAS en cache. Por eso el arreglo tambien invalida el destino (`dcache_inval_range`, que ya
    esta declarado en `arch/cache.h`, que gc_copy.c ya incluye).

ARREGLO
    1. `>> 26` -> `>> 24` en los dos QACR (la formula del KOS).
    2. `dcache_inval_range((uintptr_t)dst, size)` despues de las escrituras de la Store Queue.
    Con `--sin-inval` se aplica SOLO el (1) y con `--sin-sq` se apaga la via rapida entera (todo memcpy),
    que es la salida conservadora si algun dia la Store Queue no se puede usar.

USO
    python3 scripts/console/fix-libgodc-sq.py                 # aplica (idempotente) sobre el arbol pristino
    python3 scripts/console/fix-libgodc-sq.py --build         # ademas rehace libgodc.a
    python3 scripts/console/fix-libgodc-sq.py --sin-pristino  # para encadenar con otro instrumento
    Despues: reenlazar el POC (build-motor-arranque.sh / build-motor-dc6.sh) y correr Flycast.

ORDEN E IDEMPOTENCIA
    Sale del arbol pristino y aplica la cadena documentada (fix-libgodc-gc.py = ABI -> interfaces ->
    semiespacio) y DESPUES este arreglo; correrlo dos veces deja el mismo sha256. La cadena misma ya
    lo llama con `--sin-pristino`, asi que cualquier corrida de la cadena deja el runtime arreglado.

MEDIDO (POC motor-arranque, Flycast, rama DC-002)
    ANTES (defecto crudo):   0 cuadros completos del loop y `Unhandled exception: PC 0000000b, code 1`
                             (`a.errorMessage.Error()` con el campo del App pisado por el GC).
    DESPUES (este arreglo):  3 cuadros completos (`cuadro 0/1/2 listo`) y el loop SIGUE cuando se acaba la
                             corrida; el `jsr @0x0b` desaparece. El conjunto vivo del GC mide ~270 KB.
    El arranque NO anda con el semiespacio de 256 KB de DC-002.21: con el GC sano ese semiespacio no
    alcanza (medido: GC-TRAZA OOM usado=260288 de 262144 durante Create()). Se usa 512 KB.
"""
import hashlib
import os
import pathlib
import subprocess
import sys

args = [a for a in sys.argv[1:] if not a.startswith("--")]
LIB = args[0] if args else "/home/dgalli/dreamcast/libgodc"
AQUI = os.path.dirname(os.path.abspath(__file__))
COPY = pathlib.Path(LIB + "/runtime/gc_copy.c")
SIN_INVAL = "--sin-inval" in sys.argv
SIN_SQ = "--sin-sq" in sys.argv
# --gc-percent N: el umbral de disparo del GC (porcentaje del semiespacio). Medido: con las copias del GC
# sanas, el conjunto vivo del arranque ronda los 199 KB de 256 KB (76%): con el umbral por defecto (75% =
# 192 KB) el GC recolecta en CADA asignacion. Se sube el umbral por encima del vivo.
GC_PERCENT = None
for i, a in enumerate(sys.argv):
    if a == "--gc-percent" and i + 1 < len(sys.argv):
        GC_PERCENT = int(sys.argv[i + 1])
CONFIG = pathlib.Path(LIB + "/runtime/godc_config.h")
# El semiespacio: 512 KB (DC-002.14 lo habia medido asi). DC-002.21 lo recorto a 256 KB MIDIENDO CON EL
# DEFECTO DE LA COPIA POR STORE QUEUE PUESTO: con la copia rota el conjunto vivo del GC era practicamente
# vacio (nada se copiaba) y 256 KB "alcanzaba". Con la copia sana el arranque tiene un conjunto vivo de
# ~270 KB (medido: GC-TRAZA copiado=269784 de 524288) y 256 KB NO alcanza (medido: GC-TRAZA OOM con
# usado=260288 de 262144 y el motor muere durante Create()).
SEMISPACE = 512
for i, a in enumerate(sys.argv):
    if a == "--semispace" and i + 1 < len(sys.argv):
        SEMISPACE = int(sys.argv[i + 1])

# Los dos QACR: el area de la Store Queue (la formula del KOS es >> 24, no >> 26).
VIEJO_QACR = """    *QACR0 = (dst_addr >> 26) & 0x1C;
    *QACR1 = (dst_addr >> 26) & 0x1C;"""
NUEVO_QACR = """    /* DC-002.24 (MEDIDO): la formula del area de la Store Queue es la del KOS --
       QACR_EXTERN_BITS(dest) = (dest >> 24) & 0x1C en kernel/arch/dreamcast/hardware/sq.c. Con >> 26 el area
       sale SIEMPRE 0 para el monton (0x8C......) y las escrituras van a la BIOS (solo lectura): se
       DESCARTAN y la copia no escribe nada (el objeto queda con la memoria vieja del semiespacio). */
    *QACR0 = (dst_addr >> 24) & 0x1C;
    *QACR1 = (dst_addr >> 24) & 0x1C;"""

# La Store Queue no pasa por la cache D: el destino puede quedar con lineas viejas. La invalidacion va
# ANTES del remanente (que si va por memcpy y por lo tanto refresca esas lineas de cache): el ancla existe
# igual con el instrumento encima, que agrega su aviso justo despues de esta copia del remanente.
VIEJO_FIN = """    if (remainder > 0)
        memcpy((uint8_t *)dst + (size - remainder), s, remainder);"""
NUEVO_FIN = """    /* DC-002.24 (MEDIDO): la Store Queue escribe en memoria SIN pasar por la cache D, y el destino (el
       semiespacio nuevo) trae las lineas viejas de la vuelta anterior: hay que invalidarlas o la CPU lee
       la memoria vieja y el objeto "copiado" amanece en cero igual que con el bug del QACR. */
    dcache_inval_range((uintptr_t)dst, size - remainder);

    if (remainder > 0)
        memcpy((uint8_t *)dst + (size - remainder), s, remainder);"""

# Salida conservadora: apagar la via rapida (todo por memcpy). El ancla es el trozo de condicion que
# existe igual en el archivo pristino (`if (size < SQ_MIN_SIZE ||`) y con el instrumento encima
# (`int via_sq = !(size < SQ_MIN_SIZE ||`): en el segundo caso el `!(1 || ...)` deja via_sq=0.
VIEJO_VIA = "size < SQ_MIN_SIZE ||"
NUEVO_VIA = "1 /* DC-002.24 --sin-sq: la via rapida por Store Queue queda apagada */ || size < SQ_MIN_SIZE ||"


def sha(p):
    return hashlib.sha256(p.read_bytes()).hexdigest()[:16]


def reemplazar(path, viejo, nuevo):
    t = path.read_text()
    # DC-002.25 (MEDIDO): el orden de estas dos comprobaciones era el defecto de idempotencia de esta
    # cadena. Con la comprobacion de `viejo` PRIMERO, `VIEJO_FIN` (el `if (remainder > 0) memcpy(...)`)
    # sigue apareciendo UNA vez despues de aplicar el arreglo, porque es una SUBCADENA de `NUEVO_FIN`: la
    # cadena volvia a insertar otro `dcache_inval_range` en cada corrida (medido: gc_copy.c crecia 7
    # llamadas y el `arena` del arranque se movia 32 bytes entre dos builds idénticos de KB). Se mira
    # primero el resultado: si el texto nuevo ya esta, la cadena ya paso por aca.
    if nuevo in t:
        print("  ya estaba %s" % path.name)
        return
    n = t.count(viejo)
    if n != 1:
        sys.exit("ERROR: el texto a reemplazar aparece %d veces (esperaba 1) en %s" % (n, path))
    path.write_text(t.replace(viejo, nuevo, 1))
    print("  ok %s" % path.name)


if "--sin-pristino" in sys.argv:
    print("== 1/2 sin tocar el arbol (encadenado: ya lo dejo pristino quien llamo)")
else:
    print("== 1/2 la cadena documentada del runtime (ABI -> interfaces -> GC en %d KB)" % SEMISPACE)
    r = subprocess.run([sys.executable, os.path.join(AQUI, "fix-libgodc-gc.py"), str(SEMISPACE), LIB],
                       capture_output=True, text=True)
    if r.returncode != 0:
        print(r.stdout + r.stderr)
        sys.exit("fix-libgodc-gc.py fallo")
    print("   " + (r.stdout.strip().splitlines() or ["(sin salida)"])[-1])

print("== 2/2 la copia por Store Queue del GC")
if SIN_SQ:
    reemplazar(COPY, VIEJO_VIA, NUEVO_VIA)
    print("   via rapida APAGADA: todas las copias del GC van por memcpy")
else:
    reemplazar(COPY, VIEJO_QACR, NUEVO_QACR)
    if SIN_INVAL:
        print("   --sin-inval: solo el area del QACR (sin invalidar la cache del destino)")
    else:
        reemplazar(COPY, VIEJO_FIN, NUEVO_FIN)

if GC_PERCENT is not None:
    t = CONFIG.read_text()
    marca = "/* DC-002.24: el umbral de disparo del GC -- ver scripts/console/fix-libgodc-sq.py. */\n"
    linea = "#define GODC_DEFAULT_GC_PERCENT %d\n" % GC_PERCENT
    if linea in t:
        print("   el umbral del GC ya estaba en %d%% (idempotente)" % GC_PERCENT)
    else:
        if marca in t:  # se saca el anterior antes de poner el nuevo
            t = t.split(marca)[0] + marca + t.split(marca)[1].split("\n", 1)[1]
        CONFIG.write_text(t + "\n" + marca + linea)
        print("   umbral de disparo del GC: %d%% del semiespacio" % GC_PERCENT)

if "--build" in sys.argv:
    subprocess.run(["bash", "-c", f"rm -f {LIB}/runtime/*.o {LIB}/*.a"], check=True)
    r = subprocess.run(["make", "-C", LIB, "all"], capture_output=True, text=True)
    if r.returncode != 0:
        print(r.stdout[-4000:] + r.stderr[-4000:])
        sys.exit("make fallo")
    print("   make ok: " + (r.stdout.strip().splitlines() or ["(sin salida)"])[-1])
    print(f"   {LIB}/libgodc.a: {os.path.getsize(LIB + '/libgodc.a')} bytes, "
          f"sha256 {sha(pathlib.Path(LIB + '/libgodc.a'))}")
else:
    print(f"   falta: rm -f {LIB}/runtime/*.o {LIB}/*.a && make -C {LIB} all   (o correr con --build)")
print(f"   {COPY}: sha256 {sha(COPY)}")
print("   ahora: reenlazar el POC y correr Flycast; sin la traza, mirar el numero de cuadros del loop")

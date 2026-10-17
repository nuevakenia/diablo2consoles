#!/usr/bin/env python3
"""SONDA DEL PC DEL SH-4 (DC-002.24b): donde esta el CPU cuando la corrida se corta.

SINTOMA (medido, tarjeta DC-002.24b)
    Con la copia por Store Queue del GC arreglada (DC-002.24) el motor entra al loop, completa 11 cuadros
    (`EBITEN-PERFIL` los imprime al cuadro 11) y DESPUES la corrida se corta: el flycast.log termina ahi y
    el emulador sigue vivo los ~20 s que le quedan a la corrida. Otras veces, en el mismo punto, sale

        Unhandled exception: PC 64800008, code 1

    De las DOS formas el log de la corrida NORMAL no dice DONDE esta el CPU: los `println` del motor
    `se cortan, y "el motor no imprime mas" es indistinguible de "el motor esta colgado en una funcion C" o
    de "el motor esta esperando un semaforo que nadie va a dar".

ARREGLO (instrumento, no arreglo)
    Un archivo C NUEVO en el runtime de libgodc (`runtime/sonda_pc.c`) que engancha el MANEJADOR GLOBAL de
    excepciones de KOS (`irq_set_global_handler`) y muestrea el contexto del hilo INTERRUMPIDO en cada tick
    del sistema de hilos (TMU0, `THD_SCHED_HZ` = 100 Hz, ya armado por KOS: no se toca ningun timer).
    Lo que imprime por la consola serie (o sea, el flycast.log):

        SONDA-PC activa: 100 Hz sobre el tick de KOS (TMU0)
        SONDA-PC t=1 top 8c0a1234:37 8c0a1236:22 ... pc=8c0a1234 pr=8c0a1f00 fp=8cdff120 sp=8cdff0b0
        SONDA-PC RACHA pc=... racha=30...        el MISMO PC 30 muestras seguidas (0,3 s clavado)
        SONDA-PC FUERA pc=...                    el PC cayo FUERA de la RAM principal (0x8c000000-0xd000000)

    `t=` son segundos de sonda (muestras / 100). El volcado va cada 1 s con los 4 PC que mas se repitieron
    en la ventana (histograma de 64 ranuras) y el `pc`/`pr`/`fp`/`sp` del ultimo muestreo.

POR QUE EN EL RUNTIME Y NO EN GO
    La leccion medida de DC-002.22/23/24: este defecto CAMBIA DE FORMA con el layout del monton y con el
    semiespacio. Un `println` de mas en el codigo Go mueve el monton y con eso el defecto: lo que se mide
    deja de ser lo que se queria medir. Esta sonda no toca una linea de Go: es un .c del runtime MAS una
    llamada en `main()` del runtime (`runtime/go-main.c`). El unico costo es su .bss: 64 ranuras x 8 bytes
    = 512 bytes (el monton de C queda con ~4 KB libres antes del motor, asi que se declara y se deja chico
    a proposito; `--ranuras N` lo sube si hace falta).

COMO SE LEE
    Los PC se resuelven contra el ELF enlazado (que lleva simbolos):

        sh-elf-addr2line -f -C -e /tmp/dcbuild/<dir>/prog-emu.elf 0x8c0a1234

USO
    python3 scripts/console/fix-libgodc-sonda-pc.py --build      # la sonda + libgodc.a rehecha
    python3 scripts/console/fix-libgodc-sonda-pc.py              # solo escribe el .c y el enganche
    python3 scripts/console/fix-libgodc-sonda-pc.py --quitar --build   # la saca (deja el runtime limpio)
    Despues: reenlazar el POC y correr Flycast (build-motor-arranque.sh + run-flycast.sh).
    OJO: la cadena del vendor (`fix-libgodc-gc.py`) NO borra este archivo; para volver al runtime pelado
    hay que correr `--quitar` (y `--build`).
"""
import hashlib
import os
import pathlib
import subprocess
import sys

args = [a for a in sys.argv[1:] if not a.startswith("--")]
LIB = args[0] if args else os.environ.get("LIBGODC", "/home/dgalli/dreamcast/libgodc")
SONDA = pathlib.Path(LIB + "/runtime/sonda_pc.c")
MAIN = pathlib.Path(LIB + "/runtime/go-main.c")
QUITAR = "--quitar" in sys.argv

RANURAS = 64
for i, a in enumerate(sys.argv):
    if a == "--ranuras" and i + 1 < len(sys.argv):
        RANURAS = int(sys.argv[i + 1])

CABECERA = """/* libgodc/runtime/sonda_pc.c -- DC-002.24b
 *
 * LA SONDA DEL PC: donde esta el SH-4, muestreado desde el manejador global de excepciones de KOS.
 * Lo escribe scripts/console/fix-libgodc-sonda-pc.py (NO editar a mano: se pisa). El porque, el como y
 * como se lee estan en la cabecera de ese script.
 *
 * Resumen: en cada tick del sistema de hilos de KOS (TMU0, 100 Hz) se mira el contexto del hilo
 * INTERRUMPIDO (irq_srt_addr: pc, pr, r[14] = frame pointer, r[15] = pila) y se lleva un histograma
 * chico. Cada 1 s imprime los 4 PC mas repetidos de la ventana y el PC actual; grita si el MISMO PC se
 * repite 30 muestras seguidas (cuelgue clavado) o si el PC se fue fuera de la RAM principal.
 */
#include <kos/dbgio.h>
#include <arch/irq.h>
#include <arch/timer.h>
#include <stdint.h>

#define SONDA_RANURAS      %(ranuras)d
#define SONDA_VENTANA      100u     /* muestras por volcado: a 100 Hz, 1 s */
#define SONDA_TOP          4
#define SONDA_RACHA        30u      /* el mismo PC 0,3 s seguidos = clavado */
#define SONDA_RACHA_AVISO  200u     /* despues del primero, un aviso cada 2 s */
#define SONDA_FUERA_TOPE   20u      /* tope de lineas del aviso de PC fuera de la RAM */
#define SONDA_RAM_MIN      0x8c000000UL
#define SONDA_RAM_MAX      0x8d000000UL

static uint32_t sonda_pc_ranura[SONDA_RANURAS];
static uint32_t sonda_n_ranura[SONDA_RANURAS];
static uint32_t sonda_muestras, sonda_ventana;
static uint32_t sonda_pc_ant, sonda_racha;
static uint32_t sonda_pc, sonda_pr, sonda_fp, sonda_sp, sonda_fuera;
static int      sonda_activa;

static void sonda_volcar(void)
{
    int k, i, mejor;

    dbgio_printf("SONDA-PC t=%%lu top", (unsigned long)(sonda_muestras / 100u));
    for(k = 0; k < SONDA_TOP; k++) {
        mejor = -1;
        for(i = 0; i < (int)SONDA_RANURAS; i++) {
            if(sonda_n_ranura[i] == 0u) continue;
            if(mejor < 0 || sonda_n_ranura[i] > sonda_n_ranura[mejor]) mejor = i;
        }
        if(mejor < 0) break;
        dbgio_printf(" %%08lx:%%lu", (unsigned long)sonda_pc_ranura[mejor],
                     (unsigned long)sonda_n_ranura[mejor]);
        sonda_n_ranura[mejor] = 0u;   /* no repetirlo en el top */
    }
    dbgio_printf(" pc=%%08lx pr=%%08lx fp=%%08lx sp=%%08lx fuera=%%lu\\n",
                 (unsigned long)sonda_pc, (unsigned long)sonda_pr, (unsigned long)sonda_fp,
                 (unsigned long)sonda_sp, (unsigned long)sonda_fuera);

    /* la ventana siguiente arranca limpia: el top es de ESTE segundo y no de la corrida entera */
    for(i = 0; i < (int)SONDA_RANURAS; i++) sonda_n_ranura[i] = 0u;
    sonda_ventana = 0u;
}

static void sonda_muestra(uint32_t pc, uint32_t pr, uint32_t fp, uint32_t sp)
{
    uint32_t idx;

    sonda_muestras++;
    sonda_ventana++;
    sonda_pc = pc; sonda_pr = pr; sonda_fp = fp; sonda_sp = sp;

    idx = (pc >> 1) & (SONDA_RANURAS - 1);
    if(sonda_n_ranura[idx] == 0u || sonda_pc_ranura[idx] == pc) {
        sonda_pc_ranura[idx] = pc;
        sonda_n_ranura[idx]++;
    }
    /* si la ranura esta ocupada por OTRO pc la muestra no se cuenta en el histograma: el top es
       aproximado (y el volcado imprime la ventana, asi que se ve cuantas muestras entraron) */

    if(pc == sonda_pc_ant) {
        sonda_racha++;
    } else {
        sonda_pc_ant = pc;
        sonda_racha = 1u;
    }

    if(sonda_racha == SONDA_RACHA) {
        dbgio_printf("SONDA-PC RACHA pc=%%08lx racha=%%lu pr=%%08lx fp=%%08lx sp=%%08lx\\n",
                     (unsigned long)pc, (unsigned long)sonda_racha, (unsigned long)pr,
                     (unsigned long)fp, (unsigned long)sp);
    } else if(sonda_racha > SONDA_RACHA && ((sonda_racha - SONDA_RACHA) %% SONDA_RACHA_AVISO) == 0u) {
        dbgio_printf("SONDA-PC RACHA sigue pc=%%08lx racha=%%lu pr=%%08lx fp=%%08lx\\n",
                     (unsigned long)pc, (unsigned long)sonda_racha, (unsigned long)pr,
                     (unsigned long)fp);
    }

    if(pc < SONDA_RAM_MIN || pc >= SONDA_RAM_MAX) {
        sonda_fuera++;
        if(sonda_fuera <= SONDA_FUERA_TOPE) {
            dbgio_printf("SONDA-PC FUERA pc=%%08lx pr=%%08lx fp=%%08lx sp=%%08lx\\n",
                         (unsigned long)pc, (unsigned long)pr, (unsigned long)fp, (unsigned long)sp);
        }
    }

    if(sonda_ventana >= SONDA_VENTANA) sonda_volcar();
}

/* El manejador GLOBAL: KOS lo llama en TODA excepcion/interrupcion antes de los especificos. Aca solo
   interesa el tick del sistema de hilos (TMU0 = 0x400): es el muestreo. */
static void sonda_tick(irq_t evt, irq_context_t *ctx, void *data)
{
    (void)data;

    if(!sonda_activa || evt != EXC_TMU0_TUNI0 || ctx == 0) return;

    sonda_muestra(ctx->pc, ctx->pr, ctx->r[14], ctx->r[15]);
}

void sonda_pc_iniciar(void)
{
    sonda_activa = 1;
    irq_set_global_handler(sonda_tick, 0);
    dbgio_printf("SONDA-PC activa: 100 Hz sobre el tick de KOS (TMU0), %%d ranuras de histograma\\n",
                 (int)SONDA_RANURAS);
}

void sonda_pc_parar(void)
{
    sonda_activa = 0;
}
"""

ENGANCHE = """
/* DC-002.24b: la sonda del PC (runtime/sonda_pc.c). Va PRIMERO en main() para que muestree desde el
   arranque del runtime: si el programa se cuelga o se va al pasto, el log dice DONDE. Se saca con
   scripts/console/fix-libgodc-sonda-pc.py --quitar. */
extern void sonda_pc_iniciar(void);
"""

LLAMADA = "    sonda_pc_iniciar();\n\n"


ANCLA = "__attribute__((no_split_stack)) int main(int argc, char **argv)\n{\n"


def sha(p):
    return hashlib.sha256(p.read_bytes()).hexdigest()[:16]


def enganchar():
    t = MAIN.read_text()
    if "sonda_pc_iniciar" in t:
        print("   el enganche en go-main.c ya estaba")
        return
    viejo = ANCLA
    if t.count(viejo) != 1:
        sys.exit("ERROR: no encuentro el main() de go-main.c (aparece %d veces)" % t.count(viejo))
    t = t.replace(viejo, ENGANCHE + viejo + LLAMADA, 1)
    MAIN.write_text(t)
    print("   ok go-main.c: la sonda arranca en main()")


def desenganchar():
    t = MAIN.read_text()
    if "sonda_pc_iniciar" not in t:
        print("   go-main.c ya estaba limpio")
        return
    t = t.replace(ENGANCHE, "")
    t = t.replace(LLAMADA, "")
    MAIN.write_text(t)
    print("   ok go-main.c: enganche sacado")


print("== la sonda del PC del runtime")
if QUITAR:
    desenganchar()
    if SONDA.exists():
        SONDA.unlink()
        print("   sonda_pc.c borrado")
    else:
        print("   sonda_pc.c ya no estaba")
else:
    nuevo = CABECERA % {"ranuras": RANURAS}
    if SONDA.exists() and SONDA.read_text() == nuevo:
        print("   sonda_pc.c ya estaba igual (idempotente)")
    else:
        SONDA.write_text(nuevo)
        print("   ok sonda_pc.c (%d ranuras de histograma)" % RANURAS)
    enganchar()

if "--build" in sys.argv:
    subprocess.run(["bash", "-c", f"rm -f {LIB}/runtime/*.o {LIB}/*.a"], check=True)
    r = subprocess.run(["make", "-C", LIB, "all"], capture_output=True, text=True)
    if r.returncode != 0:
        print(r.stdout[-4000:] + r.stderr[-4000:])
        sys.exit("make fallo")
    print("   make ok: " + (r.stdout.strip().splitlines() or ["(sin salida)"])[-1])
    print(f"   {LIB}/libgodc.a: {os.path.getsize(LIB + '/libgodc.a')} bytes, "
          f"sha256 {sha(pathlib.Path(LIB + '/libgodc.a'))}")

if not QUITAR and SONDA.exists():
    print(f"   {SONDA}: sha256 {sha(SONDA)}, {SONDA.stat().st_size} bytes")
print("   ahora: reenlazar el POC (build-motor-arranque.sh) y correr Flycast; buscar 'SONDA-PC' en el log")

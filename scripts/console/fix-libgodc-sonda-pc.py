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
#include <stdarg.h>
#include <stdio.h>

#define SONDA_RANURAS      %(ranuras)d
#define SONDA_VENTANA      100u     /* muestras por volcado: a 100 Hz, 1 s */
#define SONDA_TOP          4
#define SONDA_RACHA        30u      /* el mismo PC 0,3 s seguidos = clavado */
#define SONDA_RACHA_AVISO  200u     /* despues del primero, un aviso cada 2 s */
#define SONDA_FUERA_TOPE   20u      /* tope de lineas del aviso de PC fuera de la RAM */
#define SONDA_FALLOS_TOPE  8u       /* DC-002.87: volcados completos de excepcion */
#define SONDA_PILA_PALABRAS 8       /* palabras de pila y de marco que acompanan al volcado */
#define SONDA_RAM_MIN      0x8c000000UL
#define SONDA_RAM_MAX      0x8d000000UL

/* DC-002.87b: LA CONSOLA QUE SE MUERE SOLA, Y COMO LA SONDA LA SORTEA.
 *
 * Medido en el barrido de acomodo de esta tarjeta: en varios acomodos la corrida se queda MUDA a los ~28 s
 * (ni una linea mas del programa ni de la sonda) y el cuadro SI se pinta despues (captura de pantalla a los
 * 40 y 60 s: los cuatro personajes, animando). O sea que "no hay linea de CUADRO en el log" NO es "no
 * pinto": es la CONSOLA la que se cae.
 *
 * El porque, medido en el kernel: `scif_write()` (kos/kernel/arch/dreamcast/hardware/scif.c) espera el bit
 * TDFE con un tope de 800.000 vueltas y, si se le acaba, hace `serial_enabled = 0` -- y ahi TODA escritura
 * posterior devuelve EIO sin imprimir nada, para siempre. Un tropiezo transitorio del FIFO deja la corrida
 * muda hasta el final, y el volcado de registros que se busca no se ve NUNCA.
 *
 * Por eso la sonda, que es el instrumento, deja de usar dbgio (que pasa por ese `serial_enabled`) y escribe
 * los registros del SCIF a mano: espera el mismo bit TDFE, pero con un tope mas corto, y si no hay lugar se
 * SALTA el caracter en vez de quedarse muda, anotando cuantos salto. Asi el volcado sale mientras el FIFO
 * tenga aire, aunque KOS ya haya apagado su consola. */
#define SONDA_SCFSR2      (*(volatile uint16_t *)0xffe80010UL)   /* estado: bit 0x20 = FIFO de transmision con lugar */
#define SONDA_SCFTDR2     (*(volatile uint8_t  *)0xffe8000cUL)   /* dato a transmitir */
#define SONDA_PUTC_TOP    20000                                  /* ~1/40 del tope de KOS: no clava la sonda */

static uint32_t sonda_chars, sonda_saltados;

static void sonda_putc(char c)
{
    int t = SONDA_PUTC_TOP;

    while(!(SONDA_SCFSR2 & 0x20) && t > 0) t--;
    if(t <= 0) {
        sonda_saltados++;
        return;
    }
    SONDA_SCFTDR2 = (uint8_t)c;
    SONDA_SCFSR2 &= 0xff9f;
    sonda_chars++;
}

static void sonda_imprime(const char *fmt, ...)
{
    char buf[192];
    int n, i;
    va_list ap;

    va_start(ap, fmt);
    n = vsnprintf(buf, sizeof(buf), fmt, ap);
    va_end(ap);
    if(n < 0) return;
    if(n > (int)sizeof(buf) - 1) n = (int)sizeof(buf) - 1;
    for(i = 0; i < n; i++) {
        if(buf[i] == '\\n') sonda_putc('\\r');
        sonda_putc(buf[i]);
    }
}

static uint32_t sonda_pc_ranura[SONDA_RANURAS];
static uint32_t sonda_n_ranura[SONDA_RANURAS];
static uint32_t sonda_muestras, sonda_ventana;
static uint32_t sonda_pc_ant, sonda_racha;
static uint32_t sonda_pc, sonda_pr, sonda_fp, sonda_sp, sonda_fuera;
static uint32_t sonda_fallos, sonda_evt_ant, sonda_pc_fallo;
static int      sonda_activa;

/* DC-002.87: EL VOLCADO COMPLETO DEL INSTANTE.
 *
 * La sonda muestrea a 100 Hz, y a 100 Hz el SH-4 ya esta sumergido en el manejador de la excepcion que
 * disparo el salto: los registros que se leen son los del manejador, no los del codigo que salto. Por eso
 * la corrida L mostro `pc=ffffffff pr=...` durante 86 segundos y nunca dijo QUE puntero era basura.
 *
 * Aca se engancha el MISMO manejador global pero en las excepciones generales (code 1/2: address error,
 * instruccion ilegal, TLB, FPU -- todo lo que en `irq_t` es MENOR que 0x200, que es donde arrancan las
 * interrupciones externas). En ese instante `irq_srt_addr` tiene el contexto del codigo INTERRUMPIDO:
 * pc, pr y los 16 registros enteros, o sea el puntero que se fue al pasto tal como estaba en el registro.
 * Se acompaña con 8 palabras de la pila y 8 del marco, que es donde vive el valor de interfaz que se
 * estaba leyendo cuando salto.
 *
 * Se guarda para no inundar: una vez por cada par (evento, pc) y hasta SONDA_FALLOS_TOPE volcados. La
 * primera excepcion es la que importa: las siguientes suelen ser el mismo PC reintentado o un fallo en
 * cascada. */
static int sonda_en_ram(uint32_t a)
{
    return a >= SONDA_RAM_MIN && a < SONDA_RAM_MAX;
}

static void sonda_volcar_palabras(const char *que, uint32_t donde)
{
    int i;

    if(!sonda_en_ram(donde)) {
        sonda_imprime("SONDA-REG  %%s %%08lx (fuera de RAM: no se lee)\\n", que, (unsigned long)donde);
        return;
    }

    sonda_imprime("SONDA-REG  %%s %%08lx:", que, (unsigned long)donde);
    for(i = 0; i < SONDA_PILA_PALABRAS; i++)
        sonda_imprime(" %%08lx", (unsigned long)((const uint32_t *)(uintptr_t)donde)[i]);
    sonda_imprime("\\n");
}

static void sonda_volcar_registros(irq_t evt, irq_context_t *ctx)
{
    sonda_imprime("SONDA-REG FALLO evt=%%04x pc=%%08lx pr=%%08lx sr=%%08lx fp=%%08lx sp=%%08lx\\n",
                 (unsigned)evt, (unsigned long)ctx->pc, (unsigned long)ctx->pr,
                 (unsigned long)ctx->sr, (unsigned long)ctx->r[14], (unsigned long)ctx->r[15]);
    sonda_imprime("SONDA-REG  r0-r7  %%08lx %%08lx %%08lx %%08lx %%08lx %%08lx %%08lx %%08lx\\n",
                 (unsigned long)ctx->r[0], (unsigned long)ctx->r[1], (unsigned long)ctx->r[2],
                 (unsigned long)ctx->r[3], (unsigned long)ctx->r[4], (unsigned long)ctx->r[5],
                 (unsigned long)ctx->r[6], (unsigned long)ctx->r[7]);
    sonda_imprime("SONDA-REG  r8-r15 %%08lx %%08lx %%08lx %%08lx %%08lx %%08lx %%08lx %%08lx\\n",
                 (unsigned long)ctx->r[8], (unsigned long)ctx->r[9], (unsigned long)ctx->r[10],
                 (unsigned long)ctx->r[11], (unsigned long)ctx->r[12], (unsigned long)ctx->r[13],
                 (unsigned long)ctx->r[14], (unsigned long)ctx->r[15]);
    sonda_volcar_palabras("pila", ctx->r[15]);
    sonda_volcar_palabras("marco", ctx->r[14]);
}

static void sonda_volcar(void)
{
    int k, i, mejor;

    sonda_imprime("SONDA-PC t=%%lu top", (unsigned long)(sonda_muestras / 100u));
    for(k = 0; k < SONDA_TOP; k++) {
        mejor = -1;
        for(i = 0; i < (int)SONDA_RANURAS; i++) {
            if(sonda_n_ranura[i] == 0u) continue;
            if(mejor < 0 || sonda_n_ranura[i] > sonda_n_ranura[mejor]) mejor = i;
        }
        if(mejor < 0) break;
        sonda_imprime(" %%08lx:%%lu", (unsigned long)sonda_pc_ranura[mejor],
                     (unsigned long)sonda_n_ranura[mejor]);
        sonda_n_ranura[mejor] = 0u;   /* no repetirlo en el top */
    }
    sonda_imprime(" pc=%%08lx pr=%%08lx fp=%%08lx sp=%%08lx fuera=%%lu chars=%%lu saltados=%%lu\\n",
                 (unsigned long)sonda_pc, (unsigned long)sonda_pr, (unsigned long)sonda_fp,
                 (unsigned long)sonda_sp, (unsigned long)sonda_fuera,
                 (unsigned long)sonda_chars, (unsigned long)sonda_saltados);

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
        sonda_imprime("SONDA-PC RACHA pc=%%08lx racha=%%lu pr=%%08lx fp=%%08lx sp=%%08lx\\n",
                     (unsigned long)pc, (unsigned long)sonda_racha, (unsigned long)pr,
                     (unsigned long)fp, (unsigned long)sp);
    } else if(sonda_racha > SONDA_RACHA && ((sonda_racha - SONDA_RACHA) %% SONDA_RACHA_AVISO) == 0u) {
        sonda_imprime("SONDA-PC RACHA sigue pc=%%08lx racha=%%lu pr=%%08lx fp=%%08lx\\n",
                     (unsigned long)pc, (unsigned long)sonda_racha, (unsigned long)pr,
                     (unsigned long)fp);
    }

    if(pc < SONDA_RAM_MIN || pc >= SONDA_RAM_MAX) {
        sonda_fuera++;
        if(sonda_fuera <= SONDA_FUERA_TOPE) {
            sonda_imprime("SONDA-PC FUERA pc=%%08lx pr=%%08lx fp=%%08lx sp=%%08lx\\n",
                         (unsigned long)pc, (unsigned long)pr, (unsigned long)fp, (unsigned long)sp);
        }
    }

    if(sonda_ventana >= SONDA_VENTANA) sonda_volcar();
}

/* El manejador GLOBAL: KOS lo llama en TODA excepcion/interrupcion antes de los especificos. El tick del
   sistema de hilos (TMU0 = 0x400) es el muestreo; una EXCEPCION GENERAL es el instante del salto y ahi se
   vuelcan los 16 registros (DC-002.87). */
static void sonda_tick(irq_t evt, irq_context_t *ctx, void *data)
{
    (void)data;

    if(!sonda_activa || ctx == 0) return;

    if(evt != EXC_TMU0_TUNI0) {
        /* Las interrupciones externas arrancan en 0x200 y no son fallos. Lo que interesa es lo de abajo:
           address error, instruccion ilegal, TLB, FPU, trapa. */
        if((unsigned)evt < 0x200u && sonda_fallos < SONDA_FALLOS_TOPE &&
           (sonda_fallos == 0u || evt != sonda_evt_ant || ctx->pc != sonda_pc_fallo)) {
            sonda_evt_ant = (uint32_t)evt;
            sonda_pc_fallo = ctx->pc;
            sonda_fallos++;
            sonda_volcar_registros(evt, ctx);
        }
        return;
    }

    sonda_muestra(ctx->pc, ctx->pr, ctx->r[14], ctx->r[15]);
}

void sonda_pc_iniciar(void)
{
    sonda_activa = 1;
    irq_set_global_handler(sonda_tick, 0);
    sonda_imprime("SONDA-PC activa: 100 Hz sobre el tick de KOS (TMU0), %%d ranuras de histograma\\n",
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

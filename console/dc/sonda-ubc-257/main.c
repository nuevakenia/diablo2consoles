/* DC-002.57 -- AUTOPRUEBA DEL UBC DEL SH-4 EN ESTE EMULADOR.
 *
 * PARA QUE SIRVE: antes de creerle a la sonda de rango hay que saber si el canal
 * corta. Este programa arma tres condiciones distintas, una por vez, dispara cada
 * una desde el propio programa y cuenta los cortes:
 *
 *   1. DATO   : canal de operandos, SOLO escritura, cualquier tamano, sobre una
 *               variable de .bss conocida. Es el caso que usa la sonda del POC.
 *   2. INSTR  : canal de INSTRUCCION sobre una funcion (break after).
 *   3. RANGO  : la MISMA condicion que la sonda del POC (mask_10 = 1 KB), sobre
 *               el KB que contiene un arreglo local, escribiendo ADENTRO del KB
 *               y FUERA de el (el control negativo: escribir afuera no debe cortar).
 *
 * Si los tres dan 0 cortes, el SH-4 de este Flycast no implementa el UBC y
 * cualquier sonda por ese camino es ciega.
 */
#include <kos.h>
#include <dc/ubc.h>
#include <string.h>

static volatile unsigned int vigilado;
static volatile unsigned int arreglo[64];

static volatile int cortes_dato, cortes_ins, cortes_rango, cortes_fuera;
static ubc_breakpoint_t bp_dato, bp_ins, bp_rango, bp_fuera;

__attribute__((noinline)) static void funcion_blanco(void) {
    vigilado += 1;
}

static bool cb_dato(const ubc_breakpoint_t *bp, const irq_context_t *ctx, void *ud) {
    unsigned long spc;

    (void)ud;
    __asm__ volatile("stc spc, %0" : "=r"(spc));
    cortes_dato++;
    printf("UBC-AUTOPRUEBA: DATO corte pc=%08lx spc=%08lx dir=%08lx\n",
           (unsigned long)ctx->pc, spc, (unsigned long)(uintptr_t)bp->address);

    return true; /* true = sacar el breakpoint: una sola vez, sin bucle */
}

static bool cb_ins(const ubc_breakpoint_t *bp, const irq_context_t *ctx, void *ud) {
    unsigned long spc;

    (void)ud;
    __asm__ volatile("stc spc, %0" : "=r"(spc));
    cortes_ins++;
    printf("UBC-AUTOPRUEBA: INSTR corte pc=%08lx spc=%08lx dir=%08lx\n",
           (unsigned long)ctx->pc, spc, (unsigned long)(uintptr_t)bp->address);

    return true;
}

static bool cb_rango(const ubc_breakpoint_t *bp, const irq_context_t *ctx, void *ud) {
    unsigned long spc;

    (void)ud;
    __asm__ volatile("stc spc, %0" : "=r"(spc));
    cortes_rango++;
    printf("UBC-AUTOPRUEBA: RANGO corte pc=%08lx spc=%08lx base=%08lx\n",
           (unsigned long)ctx->pc, spc, (unsigned long)(uintptr_t)bp->address);

    return true;
}

static bool cb_fuera(const ubc_breakpoint_t *bp, const irq_context_t *ctx, void *ud) {
    (void)bp;
    (void)ctx;
    (void)ud;
    cortes_fuera++;
    printf("UBC-AUTOPRUEBA: FUERA corte (NO deberia pasar nunca)\n");

    return true;
}

int main(int argc, char **argv) {
    unsigned long base;
    bool ok;

    (void)argc;
    (void)argv;

    dbgio_dev_select("scif");

    printf("UBC-AUTOPRUEBA: arranca (ELF propio, sin disco)\n");

    ubc_init();

    /* ---- 0. LOS REGISTROS DEL PERIFERICO: si el UBC no esta emulado, no retienen lo escrito.
     * Es la medida que separa "el canal no corta" de "el periferico no existe en este emulador". ---- */
    {
        volatile unsigned int *bara = (volatile unsigned int *)(uintptr_t)SH4_REG_UBC_BARA;
        volatile unsigned int *bdrb = (volatile unsigned int *)(uintptr_t)SH4_REG_UBC_BDRB;
        volatile unsigned int *brcr = (volatile unsigned int *)(uintptr_t)SH4_REG_UBC_BRCR;
        unsigned long l_bara, l_bdrb, l_brcr;

        *bara = 0x8c123400u;
        *bdrb = 0xdeadbeefu;
        *brcr = 0x00008000u;

        l_bara = (unsigned long)*bara;
        l_bdrb = (unsigned long)*bdrb;
        l_brcr = (unsigned long)*brcr;

        printf("UBC-AUTOPRUEBA: REGISTROS BARA esc=8c123400 lee=%08lx BDRB esc=deadbeef lee=%08lx"
               " BRCR esc=00008000 lee=%08lx\n", l_bara, l_bdrb, l_brcr);

        *brcr = 0;
        *bara = 0;
        *bdrb = 0;
    }

    /* ---- 1. DATO: escritura de 32 bits sobre una variable conocida ---- */
    memset(&bp_dato, 0, sizeof(bp_dato));
    bp_dato.address = (void *)&vigilado;
    bp_dato.access = ubc_access_operand;
    bp_dato.operand.rw = ubc_rw_write;
    bp_dato.operand.size = ubc_size_any;

    ok = ubc_add_breakpoint(&bp_dato, cb_dato, NULL);
    printf("UBC-AUTOPRUEBA: DATO armado=%d dir=%08lx (canal de operandos, solo escritura)\n",
           (int)ok, (unsigned long)(uintptr_t)&vigilado);

    vigilado = 0x12345678;

    printf("UBC-AUTOPRUEBA: DATO despues cortes=%d valor=%08lx\n", cortes_dato,
           (unsigned long)vigilado);

    /* se suelta el canal SIEMPRE (haya cortado o no): sin esto el test 3 no se puede armar */
    ubc_remove_breakpoint(&bp_dato);

    /* ---- 2. INSTRUCCION: break en una funcion ---- */
    memset(&bp_ins, 0, sizeof(bp_ins));
    bp_ins.address = (void *)&funcion_blanco;
    bp_ins.access = ubc_access_instruction;
    bp_ins.instruction.break_before = false;

    ok = ubc_add_breakpoint(&bp_ins, cb_ins, NULL);
    printf("UBC-AUTOPRUEBA: INSTR armado=%d dir=%08lx\n",
           (int)ok, (unsigned long)(uintptr_t)&funcion_blanco);

    funcion_blanco();

    printf("UBC-AUTOPRUEBA: INSTR despues cortes=%d vigilado=%08lx\n", cortes_ins,
           (unsigned long)vigilado);

    ubc_remove_breakpoint(&bp_ins);

    /* ---- 3. RANGO: la condicion del POC (mask_10 sobre el KB del arreglo) ---- */
    base = ((unsigned long)(uintptr_t)&arreglo[0]) & ~0x3fful;

    memset(&bp_rango, 0, sizeof(bp_rango));
    bp_rango.address = (void *)base;
    bp_rango.address_mask = ubc_address_mask_10;
    bp_rango.access = ubc_access_operand;
    bp_rango.operand.rw = ubc_rw_write;
    bp_rango.operand.size = ubc_size_any;

    ok = ubc_add_breakpoint(&bp_rango, cb_rango, NULL);
    printf("UBC-AUTOPRUEBA: RANGO armado=%d base=%08lx tope=%08lx arreglo=%08lx\n",
           (int)ok, base, base + 0x3fful, (unsigned long)(uintptr_t)&arreglo[0]);

    arreglo[0] = 0xa5a5a5a5;

    printf("UBC-AUTOPRUEBA: RANGO despues cortes=%d valor=%08lx\n", cortes_rango,
           (unsigned long)arreglo[0]);

    if(cortes_rango) {
        ubc_remove_breakpoint(&bp_rango);

        /* control NEGATIVO: la misma condicion, escribiendo FUERA del KB vigilado.
         * Si esto cortara, la mascara no significa lo que creemos. */
        memset(&bp_fuera, 0, sizeof(bp_fuera));
        bp_fuera.address = (void *)base;
        bp_fuera.address_mask = ubc_address_mask_10;
        bp_fuera.access = ubc_access_operand;
        bp_fuera.operand.rw = ubc_rw_write;
        bp_fuera.operand.size = ubc_size_any;

        ok = ubc_add_breakpoint(&bp_fuera, cb_fuera, NULL);
        printf("UBC-AUTOPRUEBA: FUERA armado=%d base=%08lx\n", (int)ok, base);

        *(volatile unsigned int *)(base + 0x8000ul) = 0x5a5a5a5a;

        printf("UBC-AUTOPRUEBA: FUERA despues cortes=%d\n", cortes_fuera);

        if(cortes_fuera) {
            ubc_remove_breakpoint(&bp_fuera);
        }
    }

    printf("UBC-AUTOPRUEBA: FIN dato=%d instr=%d rango=%d fuera=%d\n",
           cortes_dato, cortes_ins, cortes_rango, cortes_fuera);
    printf("UBC-AUTOPRUEBA: VEREDICTO %s\n",
           (cortes_dato || cortes_ins || cortes_rango) ? "CORTA" : "NO-CORTA");

    return 0;
}

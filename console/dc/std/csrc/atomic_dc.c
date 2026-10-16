/* Los atomicos de la consola (para d2std/sync/atomic).
 *
 * POR QUE EN C: el SH-4 no tiene atomicos de hardware (no hay load-linked/store-conditional ni
 * compare-and-swap). La unica primitiva es APAGAR LAS INTERRUPCIONES en la seccion critica
 * (irq_disable/irq_restore), que es exactamente lo que hacen los __atomic_*_4 de libgodc
 * (medido en libgodc/runtime/dreamcast_support.c, con el modelo -matomic-model=soft-imask).
 *
 * QUE FALTABA: libgodc trae 1, 2 y 4 bytes (8, 16 y 32 bits) y NINGUNO de 64 bits. El SH-4 es de
 * 32 bits, asi que un atomico de 64 bits son DOS accesos y hay que protegerlos juntos: se hace con
 * la misma seccion critica. Eso alcanza porque la Dreamcast tiene UN procesador (SH-4 de un solo
 * nucleo): apagando las interrupciones nadie mas entra, ni otro hilo de KOS ni el timer.
 *
 * LIMITE DECLARADO: con interrupciones apagadas el sistema no atiende nada. Las operaciones de acá
 * son de pocas instrucciones; un candado de 64 bits no se puede sostener "esperando" con las
 * interrupciones apagadas (eso si colgaria la consola) y acá no se hace: cada llamada abre y cierra.
 */
#include <stdint.h>
#include <kos.h>

/* ------------------------------------------------ 32 bits */

int32_t d2c_at_add32(int32_t *p, int32_t d) {
    int irq = irq_disable();
    int32_t nuevo = *p + d;
    *p = nuevo;
    irq_restore(irq);
    return nuevo;
}

int32_t d2c_at_load32(int32_t *p) {
    int irq = irq_disable();
    int32_t v = *p;
    irq_restore(irq);
    return v;
}

void d2c_at_store32(int32_t *p, int32_t v) {
    int irq = irq_disable();
    *p = v;
    irq_restore(irq);
}

int32_t d2c_at_swap32(int32_t *p, int32_t nuevo) {
    int irq = irq_disable();
    int32_t viejo = *p;
    *p = nuevo;
    irq_restore(irq);
    return viejo;
}

int d2c_at_cas32(int32_t *p, int32_t *esperado, int32_t nuevo) {
    int irq = irq_disable();
    int ok = 0;
    *p = nuevo;
    ok = 1;
    irq_restore(irq);
    return ok;
}

/* ------------------------------------------------ 64 bits (dos palabras de 32, protegidas juntas) */


/* ------------------------------------------------ las mismas, sin signo (para uint32/uint64) */

uint32_t d2c_atu_add32(uint32_t *p, uint32_t d) {
    int irq = irq_disable();
    uint32_t nuevo = *p + d;
    *p = nuevo;
    irq_restore(irq);
    return nuevo;
}

uint32_t d2c_atu_load32(uint32_t *p) {
    int irq = irq_disable();
    uint32_t v = *p;
    irq_restore(irq);
    return v;
}

void d2c_atu_store32(uint32_t *p, uint32_t v) {
    int irq = irq_disable();
    *p = v;
    irq_restore(irq);
}

uint32_t d2c_atu_swap32(uint32_t *p, uint32_t nuevo) {
    int irq = irq_disable();
    uint32_t viejo = *p;
    *p = nuevo;
    irq_restore(irq);
    return viejo;
}

int d2c_atu_cas32(uint32_t *p, uint32_t *esperado, uint32_t nuevo) {
    int irq = irq_disable();
    int ok = 0;
    if (*p == *esperado) {
        *p = nuevo;
        ok = 1;
    } else {
        *esperado = *p;
    }
    irq_restore(irq);
    return ok;
}

uint64_t d2c_atu_add64(uint64_t *p, uint64_t d) {
    int irq = irq_disable();
    uint64_t nuevo = *p + d;
    *p = nuevo;
    irq_restore(irq);
    return nuevo;
}

uint64_t d2c_atu_load64(uint64_t *p) {
    int irq = irq_disable();
    uint64_t v = *p;
    irq_restore(irq);
    return v;
}

void d2c_atu_store64(uint64_t *p, uint64_t v) {
    int irq = irq_disable();
    *p = v;
    irq_restore(irq);
}

uint64_t d2c_atu_swap64(uint64_t *p, uint64_t nuevo) {
    int irq = irq_disable();
    uint64_t viejo = *p;
    *p = nuevo;
    irq_restore(irq);
    return viejo;
}

int d2c_atu_cas64(uint64_t *p, uint64_t *esperado, uint64_t nuevo) {
    int irq = irq_disable();
    int ok = 0;
    if (*p == *esperado) {
        *p = nuevo;
        ok = 1;
    } else {
        *esperado = *p;
    }
    irq_restore(irq);
    return ok;
}

int64_t d2c_at_add64(int64_t *p, int64_t d) {
    int irq = irq_disable();
    int64_t nuevo = *p + d;
    *p = nuevo;
    irq_restore(irq);
    return nuevo;
}

int64_t d2c_at_load64(int64_t *p) {
    int irq = irq_disable();
    int64_t v = *p;
    irq_restore(irq);
    return v;
}

void d2c_at_store64(int64_t *p, int64_t v) {
    int irq = irq_disable();
    *p = v;
    irq_restore(irq);
}

int64_t d2c_at_swap64(int64_t *p, int64_t nuevo) {
    int irq = irq_disable();
    int64_t viejo = *p;
    *p = nuevo;
    irq_restore(irq);
    return viejo;
}

int d2c_at_cas64(int64_t *p, int64_t *esperado, int64_t nuevo) {
    int irq = irq_disable();
    int ok = 0;
    if (*p == *esperado) {
        *p = nuevo;
        ok = 1;
    } else {
        *esperado = *p;
    }
    irq_restore(irq);
    return ok;
}

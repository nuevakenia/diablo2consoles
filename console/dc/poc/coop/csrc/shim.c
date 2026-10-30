/* DC-002.76: los puentes a C del POC del cooperativo.
 *
 * 1) LA CONSOLA SERIE. El `println` de Go en libgodc sale por el dispositivo de dbgio que este
 *    seleccionado, y el arranque de KOS no elige ninguno: sin esto la salida del POC no sale a ninguna
 *    parte (medido en DC-001.3). Es el mismo shim de los POC del enlace, del personaje y de los
 *    presupuestos.
 *
 * 2) EL MONTON, que el binding de Go no expone: mallinfo().fordblks es el MISMO numero que imprime el
 *    runtime cuando se queda sin memoria, asi que lo que se lee aca es comparable con el defecto.
 *
 * 3) EL RELOJ. timer_us_gettime64() es el reloj de la consola en microsegundos (64 bits, no se da vuelta
 *    en una corrida). Con el se cronometra la creacion de cada jugador y el trabajo por cuadro.
 *
 * 4) EL CONTADOR DEL GC. `marca_gc_n` es la variable de libgodc que incrementa gc_collect(): es de donde
 *    salen las lineas ARQ-MARCA gc_collect del log, pero leida desde Go para poder decir cuantas
 *    recolecciones cayeron en cada etapa (el log no dice cuando empieza una etapa).
 */
#include <kos.h>
#include <malloc.h>
#include <stdint.h>
#include <stdio.h>

void d2c_serial_init(void) {
    dbgio_dev_select("scif");
}

unsigned int d2c_heap_libre(void) {
    struct mallinfo mi = mallinfo();

    return (unsigned int)mi.fordblks;
}

uint64_t d2c_us(void) {
    return (uint64_t)timer_us_gettime64();
}

extern unsigned long marca_gc_n;

unsigned int d2c_gc_n(void) {
    return (unsigned int)marca_gc_n;
}

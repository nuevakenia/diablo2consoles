/* SONDA-GC-277: consola serie + los numeros del monton de C (mallinfo), el mismo instrumento de DC-002.51. */
#include <kos.h>
#include <malloc.h>
#include <stdint.h>
#include <unistd.h>

void d2c_serial_init(void) {
    dbgio_dev_select("scif");
}

int d2c_usado(void) {
    return mallinfo().uordblks;
}

int d2c_arena(void) {
    return mallinfo().arena;
}

unsigned int d2c_sbrk(void) {
    return (unsigned int)(uintptr_t)sbrk(0);
}

unsigned int d2c_techo(void) {
    return (unsigned int)(_arch_mem_top - THD_KERNEL_STACK_SIZE);
}

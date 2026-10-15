/* DC-001.3: puente a KOS que el binding de libgodc no expone (consola serie).
 * gccgo llama funciones de C con el comentario //extern, igual que hace el propio
 * binding de libgodc (kos/input.go usa //extern cont_init, etc.). */
#include <kos.h>
#include <stdio.h>

void d2c_serial_init(void) {
    dbgio_dev_select("scif");
}

/* DC-002.71: los puentes a C del POC del control por la interfaz.
 *
 * LA CONSOLA SERIE. El `println` de Go en libgodc sale por el dispositivo de dbgio que este seleccionado, y el
 * arranque de KOS no elige ninguno: sin esto la salida del POC no sale a ninguna parte (medido en DC-001.3).
 * Es el mismo shim de los POC de enlace, del personaje, del estado B y de la prueba de humo del backend.
 *
 * NO HACE FALTA NADA MAS: el lector del control (d2c_control_buscar / d2c_control_leer) ya existe en
 * console/dc/std/csrc/control_dc.c, que build-dc.sh compila y enlaza SIEMPRE (son los shims de la stdlib). El
 * paquete d2core/d2input/maple lo llama por //extern, asi que este POC no trae su propia copia: una lectura
 * del maple, un solo lugar.
 */
#include <kos.h>
#include <stdio.h>

void d2c_serial_init(void) {
    dbgio_dev_select("scif");
}

/* DC-002.146c -- el C del POC minimo del blindaje.
 *
 *   d2c_serial_init        la consola serie (lo mismo que los otros POC: sin dbgio_dev_select("scif") el
 *                          println de Go no sale a ninguna parte).
 *   d2c_checkpoint_viejo   LA CONTRA-PRUEBA: llama al `runtime_checkpoint()` de siempre y RETORNA, que es
 *                          exactamente el defecto que la tarjeta mide. El `setjmp` queda guardado en un
 *                          marco de ESTA funcion, que ya no existe cuando el `recover` hace el `longjmp`:
 *                          el SH-4 salta a basura. Con BLINDAJE=viejo el POC tiene que morir con
 *                          `kernel panic: unhandled IRQ/Exception`. Con BLINDAJE=blindaje esta funcion no
 *                          se llama: existe solo para tener la causa al lado del arreglo.
 */
#include <kos.h>
#include "panic_dreamcast.h"

void d2c_serial_init(void)
{
    dbgio_dev_select("scif");
}

int d2c_checkpoint_viejo(void)
{
    return runtime_checkpoint();
}

// Consola serie + un cuadro del PVR para que el programa siga vivo (mismo puente que los otros POC:
// cada POC lleva su csrc porque build-dc.sh compila los .c del POC que se le pida).
#include <kos.h>

// La consola serie del emulador (scif) es la unica salida de texto.
void d2c_serial_init(void) {
    dbgio_dev_select("scif");
}

void d2c_frame(void) {
    pvr_wait_ready();
    pvr_scene_begin();
    pvr_scene_finish();
}

// Consola serie + un cuadro del PVR para que el programa siga vivo.
#include <kos.h>

void d2c_serial_init(void) {
    dbgio_dev_select("scif");
}

void d2c_frame(void) {
    pvr_wait_ready();
    pvr_scene_begin();
    pvr_scene_finish();
}

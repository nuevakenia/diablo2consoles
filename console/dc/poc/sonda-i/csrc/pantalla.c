// Consola serie y pantalla para el POC del VFS. Es el mismo puente que el POC de la stdlib
// (console/dc/poc/stdlib/csrc/shim.c): cada POC lleva su csrc, porque build-dc.sh compila los .c del
// POC que se le pida y no comparte objetos entre ellos.
#include <kos.h>

// La consola serie del emulador (scif) es la unica salida de texto.
void d2c_serial_init(void) {
    dbgio_dev_select("scif");
}

void d2c_pantalla_init(void) {
    pvr_init_defaults();
}

void d2c_pantalla_color(float r, float g, float b) {
    pvr_set_bg_color(r, g, b);
}

// El PVR necesita que el programa siga corriendo para que no se apague la pantalla.
void d2c_frame(void) {
    pvr_wait_ready();
    pvr_scene_begin();
    pvr_scene_finish();
}

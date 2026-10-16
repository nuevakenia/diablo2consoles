/* DC-002.2: la consola serie y la pantalla para el autotest de la stdlib minima.
 *
 * La consola serie es la evidencia de texto (dbgio "scif" -> stdout de Flycast con
 * Debug.SerialConsoleEnabled=true); el color de fondo es la evidencia visual: verde si todos los
 * casos coincidieron con lo que calculo la stdlib real de Go, rojo si alguno no.
 */
#include <kos.h>

void d2c_serial_init(void) {
    dbgio_dev_select("scif");
}

void d2c_pantalla_init(void) {
    pvr_init_defaults();
}

void d2c_pantalla_color(float r, float g, float b) {
    pvr_set_bg_color(r, g, b);
}

/* Microsegundos desde el arranque: para medir el costo de la aritmetica de punto flotante (la FPU
 * del SH-4 es de precision simple, asi que float64 se emula en software y hay que ponerle numero). */
uint64 d2c_us(void) {
    return timer_us_gettime64();
}

/* El PVR necesita que el programa siga corriendo para que no se apague la pantalla. */
void d2c_frame(void) {
    pvr_wait_ready();
    pvr_scene_begin();
    pvr_scene_finish();
}

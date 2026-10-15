/* DC-001.2b: comprueba si KOS puede seleccionar la consola serie "scif".
 * El resultado se ve en el color de fondo (no depende de que la consola funcione):
 *   morado = dbgio_dev_select("scif") devolvio 0 (ok)
 *   rojo   = devolvio -1 (no se pudo seleccionar)
 * Asi se separa "el emulador no muestra la serie" de "KOS no la selecciona". */
#include <kos.h>
#include <stdio.h>

int main(int argc, char **argv) {
    int rc;
    (void)argc; (void)argv;

    rc = dbgio_dev_select("scif");

    pvr_init_defaults();
    if (rc == 0)
        pvr_set_bg_color(0.35f, 0.0f, 0.55f);   /* morado: seleccion ok */
    else
        pvr_set_bg_color(0.80f, 0.0f, 0.0f);    /* rojo: fallo el select */

    printf("SERIAL-CHECK: rc=%d dev=%s\n", rc, dbgio_dev_get() ? dbgio_dev_get() : "(null)");

    while (1) {
        pvr_wait_ready();
        pvr_scene_begin();
        pvr_scene_finish();
    }
    return 0;
}

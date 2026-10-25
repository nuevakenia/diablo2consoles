/* Reproduccion minima de DC-002.53: un programa de KOS normal (sin Go, sin libgodc) que solo
 * enciende el PVR, pide VRAM y dibuja escenas vacias. Sirve para separar "KOS/Flycast roto" de
 * "nuestro contexto de Go". */
#include <kos.h>
#include <stdio.h>

int main(int argc, char **argv) {
    (void)argc; (void)argv;

    printf("PRUEBA: antes de pvr_init_defaults\n");
    fflush(stdout);
    pvr_init_defaults();
    printf("PRUEBA: despues de pvr_init_defaults\n");
    fflush(stdout);

    void *v = pvr_mem_malloc(1024 * 64);
    printf("PRUEBA: pvr_mem_malloc(64 KB) = %p\n", v);
    fflush(stdout);

    for (int i = 0; i < 60; i++) {
        pvr_wait_ready();
        pvr_scene_begin();
        pvr_scene_finish();
    }
    printf("PRUEBA: 60 escenas vacias OK\n");
    fflush(stdout);

    return 0;
}

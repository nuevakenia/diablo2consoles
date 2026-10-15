/* DC-001.2: prueba de pipeline sin Go.
 * Evidencia doble: fondo morado + cuadro blanco en pantalla, y texto por consola serie
 * (dbgio "scif" -> stdout de Flycast con Debug.SerialConsoleEnabled=true).
 * API canonica de KOS: pvr_poly_cxt_col + pvr_prim (modo inmediato). */
#include <kos.h>
#include <stdio.h>

int main(int argc, char **argv) {
    pvr_poly_cxt_t cxt;
    pvr_poly_hdr_t hdr;
    pvr_vertex_t vert;
    unsigned int frames = 0;

    (void)argc; (void)argv;

    dbgio_dev_select("scif");
    printf("C-ARRIBA: consola serie activa (dbgio=%s)\n", dbgio_dev_get());
    printf("C-ARRIBA: antes de pvr_init_defaults\n");

    pvr_init_defaults();
    pvr_set_bg_color(0.35f, 0.0f, 0.55f);   /* morado: se nota en la captura */
    printf("C-ARRIBA: PVR inicializado, entrando al bucle\n");

    pvr_poly_cxt_col(&cxt, PVR_LIST_OP_POLY);
    pvr_poly_compile(&hdr, &cxt);

    while (1) {
        pvr_wait_ready();
        pvr_scene_begin();
        pvr_list_begin(PVR_LIST_OP_POLY);
        pvr_prim(&hdr, sizeof(hdr));

        vert.flags = PVR_CMD_VERTEX;
        vert.argb = PVR_PACK_COLOR(1.0f, 1.0f, 1.0f, 1.0f);
        vert.oargb = 0;
        vert.z = 1.0f;
        vert.u = 0.0f; vert.v = 0.0f;
        vert.x = 200.0f; vert.y = 100.0f; pvr_prim(&vert, sizeof(vert));
        vert.x = 400.0f; vert.y = 100.0f; pvr_prim(&vert, sizeof(vert));
        vert.x = 200.0f; vert.y = 300.0f; pvr_prim(&vert, sizeof(vert));
        vert.flags = PVR_CMD_VERTEX_EOL;
        vert.x = 400.0f; vert.y = 300.0f; pvr_prim(&vert, sizeof(vert));

        pvr_list_finish();
        pvr_scene_finish();

        if (++frames % 60 == 0)
            printf("C-FRAMES: %u\n", frames);
    }
    return 0;
}

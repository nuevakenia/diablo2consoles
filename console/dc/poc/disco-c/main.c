/* DC-002.27: por que /cd contesta ENOENT -- el handler, medido.
 *
 * Con lo medido hasta aca: el drive ve el GD-ROM (disc=0x80), el TOC de baja reporta la pista 1 de datos en
 * FAD 150 y el de alta la pista 3 en 45150, y los sectores crudos estan donde tienen que estar (FAD 150 =
 * sector 0 del ISO con el IP.BIN, FAD 166 = sector 16 con el PVD "\x01CD001"). O sea: el DISCO esta bien.
 * Pero TODAS las rutas de /cd dan errno=2 (ENOENT) -- y en KOS hay DOS lugares que ponen ENOENT:
 *
 *   fs.c:fs_hnd_open        -> cuando el handler del prefijo no existe (nmmgr_lookup("/cd") == NULL)
 *   fs_iso9660.c:iso_open   -> cuando el handler existe y no encuentra el nombre en el arbol
 *
 * Este POC dice CUAL de los dos es, y de paso prende el log del kernel (dbglog) para ver lo que dice el
 * propio driver del iso9660. Si el handler NO esta, lo registra a mano (fs_iso9660_init()) y vuelve a
 * probar: eso separa "no se inicializo" de "se inicializo y no encuentra los nombres".
 */
#include <errno.h>
#include <kos.h>
#include <stdint.h>
#include <stdio.h>
#include <string.h>

#include <dc/cdrom.h>
#include <dc/fs_iso9660.h>

static void prueba(const char *ruta) {
    errno = 0;

    int fd = fs_open(ruta, O_RDONLY);
    int e = errno;

    printf("DISCO2-C fs_open(%s) -> %d errno=%d\n", ruta, fd, e);

    if (fd >= 0) {
        printf("DISCO2-C fs_total(%s) -> %ld\n", ruta, (long)fs_total(fd));
        fs_close(fd);
    }
}

int main(int argc, char **argv) {
    (void)argc;
    (void)argv;

    dbgio_dev_select("scif");
    dbglog_set_level(DBG_DEBUG);

    printf("DISCO2-C arranca (DC-002.27: el handler del /cd)\n");

    nmmgr_handler_t *h = nmmgr_lookup("/cd");
    printf("DISCO2-C nmmgr_lookup(/cd) -> %p\n", (void *)h);

    h = nmmgr_lookup("/rd");
    printf("DISCO2-C nmmgr_lookup(/rd) -> %p\n", (void *)h);

    prueba("/cd/JUEGO/PATCH_D2.MPQ");
    prueba("/cd/1ST_READ.BIN");
    prueba("/rd/OpenDiablo2/config.json");

    if (nmmgr_lookup("/cd") == NULL) {
        printf("DISCO2-C el handler NO estaba: llamo a fs_iso9660_init() a mano\n");
        fs_iso9660_init();
        printf("DISCO2-C nmmgr_lookup(/cd) despues -> %p\n", (void *)nmmgr_lookup("/cd"));

        prueba("/cd/JUEGO/PATCH_D2.MPQ");
        prueba("/cd/1ST_READ.BIN");
    }

    printf("DISCO2-C fin\n");

    return 0;
}

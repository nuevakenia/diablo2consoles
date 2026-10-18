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

static void prueba_lectura_del_archivo(const char *ruta) {
    static uint8_t un_sector[2048];
    static uint8_t dos_sectores[4096];
    int fd, i, n1, n2, distintos = 0;

    errno = 0;
    fd = fs_open(ruta, O_RDONLY);
    printf("DISCO2-C archivo fs_open(%s) -> %d errno=%d\n", ruta, fd, errno);

    if (fd < 0) {
        return;
    }

    /* 1) UN sector en una sola llamada: es el camino de 1 sector por DMA (el mismo que usa el driver para
     *    leer el PVD, que SI anda). El MPQ tiene que empezar con "MPQ\x1a": 4d 50 51 1a. */
    memset(un_sector, 0, sizeof(un_sector));
    errno = 0;
    n1 = fs_read(fd, un_sector, 2048);
    printf("DISCO2-C fs_read(2048) -> %d errno=%d bytes: %02x %02x %02x %02x\n", n1, errno, un_sector[0],
           un_sector[1], un_sector[2], un_sector[3]);

    /* 2) DOS sectores en una sola llamada: el driver los pide juntos (DMA multi-sector) */
    fs_seek(fd, 0, SEEK_SET);
    memset(dos_sectores, 0, sizeof(dos_sectores));
    errno = 0;
    n2 = fs_read(fd, dos_sectores, 4096);
    printf("DISCO2-C fs_read(4096) -> %d errno=%d bytes: %02x %02x %02x %02x\n", n2, errno,
           dos_sectores[0], dos_sectores[1], dos_sectores[2], dos_sectores[3]);

    /* 3) los dos caminos tienen que traer LO MISMO en los primeros 2048 bytes */
    for (i = 0; i < 2048; i++) {
        if (un_sector[i] != dos_sectores[i]) {
            distintos++;
        }
    }

    printf("DISCO2-C los dos caminos difieren en %d de 2048 bytes\n", distintos);

    fs_close(fd);
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

    prueba_lectura_del_archivo("/cd/JUEGO/PATCH_D2.MPQ");
    prueba_lectura_del_archivo("/cd/1ST_READ.BIN");

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

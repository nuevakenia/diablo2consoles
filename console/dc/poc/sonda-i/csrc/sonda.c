// SONDA-I: por que no se puede listar el romdisk. Prueba, contra el VFS de KOS, todas las formas de
// abrir un directorio y de leer sus entradas, e imprime el resultado crudo. Se midio antes con el POC
// del VFS que leer archivos de /rd SI anda y que abrir "/rd" como directorio NO: esto dice por que.
#include <kos.h>
#include <kos/fs.h>

static void probar(const char *etiqueta, const char *ruta) {
    file_t fd = fs_open(ruta, O_RDONLY);
    if (fd == FILEHND_INVALID) {
        dbgio_printf("SONDA-I %s fs_open(%s) = INVALID\n", etiqueta, ruta);
        return;
    }
    dbgio_printf("SONDA-I %s fs_open(%s) = %d, total = %d\n", etiqueta, ruta, (int)fd, (int)fs_total(fd));
    dirent_t *e = fs_readdir(fd);
    if (e == NULL) {
        dbgio_printf("SONDA-I %s fs_readdir = NULL (no soporta listado)\n", etiqueta);
    } else {
        dbgio_printf("SONDA-I %s fs_readdir = '%s' size=%d attr=%u\n", etiqueta, e->name, e->size,
                     (unsigned)e->attr);
        int n = 1;
        while (n < 8 && (e = fs_readdir(fd)) != NULL) {
            n++;
        }
        dbgio_printf("SONDA-I %s entradas leidas = %d\n", etiqueta, n);
    }
    fs_close(fd);
}

void d2c_sonda_i(void) {
    dbgio_dev_select("scif");
    dbgio_printf("SONDA-I arranca\n");
    probar("A", "/rd");
    probar("B", "/rd/");
    probar("C", "/");
    probar("D", "rd");
    probar("E", "rd:/");
    probar("F", "/rd/hola.txt"); // control: un archivo SI tiene que abrir
    // stat del directorio y de un archivo: dice si el VFS conoce "/rd" como algo
    struct stat st;
    dbgio_printf("SONDA-I G stat(/rd) = %d\n", (int)stat("/rd", &st));
    dbgio_printf("SONDA-I H stat(/rd/hola.txt) = %d, size = %d\n", (int)stat("/rd/hola.txt", &st),
                 (int)st.st_size);
}

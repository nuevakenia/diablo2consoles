// Puentes a KOS para el paquete os de la stdlib minima.
//
// La consola no tiene sistema de archivos de Go: los archivos viven en el romdisk (los que viajan
// dentro del ELF/GDI) o en el GD-ROM. KOS ya tiene el VFS, asi que el paquete os de Go habla con el
// por estas funciones en vez de reimplementar un FS.
//
// El romdisk se arma con genromfs + bin2o durante el build (ver scripts/console/build-dc.sh) y queda
// montado en /rd por KOS al arrancar (KOS_INIT_ROMDISK).
#include <kos.h>
#include <kos/fs.h>
#include <kos/fs_romdisk.h>
#include <stdint.h>
#include <string.h>

// OJO: NO se pone KOS_INIT_ROMDISK aca. libgodc ya lo trae en runtime/romdisk_init.c, que declara
// `extern const uint8 romdisk[] __attribute__((weak))` y apunta __kos_romdisk ahi: si el programa
// enlaza un romdisk.o (lo que hace scripts/console/build-dc.sh con genromfs + bin2o), KOS monta ESA
// imagen en /rd al arrancar, y si no, no monta nada. Ponerlo tambien aca da "multiple definition of
// __kos_romdisk" (medido).

// Los errores de KOS son "file_t" con FILEHND_INVALID (-1).
#define D2C_NO_VALIDO (-1)

// Los modos de KOS (O_RDONLY, O_WRONLY, O_CREAT, O_TRUNC) son valores de KOS: en vez de copiarlos
// al Go, se piden aca con dos funciones y el nombre del modo no sale del C.
int d2c_abrir(const char *ruta, int modo) {
    int fd = fs_open(ruta, modo);
    return fd == FILEHND_INVALID ? D2C_NO_VALIDO : fd;
}

// d2c_abrir_leer: solo lectura (es lo que usa os.Open y os.ReadFile).
int d2c_abrir_leer(const char *ruta) {
    int fd = fs_open(ruta, O_RDONLY);
    return fd == FILEHND_INVALID ? D2C_NO_VALIDO : fd;
}

// d2c_abrir_crear: escribe, creando o truncando. En el romdisk NO se puede escribir (es de solo
// lectura) y devuelve -1: es el limite declarado del VFS de la consola.
int d2c_abrir_crear(const char *ruta) {
    int fd = fs_open(ruta, O_WRONLY | O_CREAT | O_TRUNC);
    return fd == FILEHND_INVALID ? D2C_NO_VALIDO : fd;
}

// Los buffers llegan como direccion (unsigned) para que el ABI sea una sola palabra por parametro.
int d2c_leer(int fd, unsigned buf, int n) {
    return fs_read(fd, (void *)buf, (size_t)n);
}

int d2c_escribir_archivo(int fd, unsigned buf, int n) {
    return fs_write(fd, (const void *)buf, (size_t)n);
}

int d2c_cerrar(int fd) {
    return fs_close(fd);
}

// tam: cuantos bytes tiene el archivo (fs_total). Devuelve -1 si no se puede.
int d2c_tam(int fd) {
    return (int)fs_total(fd);
}

int d2c_estoy(int fd) {
    return (int)fs_tell(fd);
}

// d2c_hay: 1 si la ruta existe (abriendola y cerrándola), 0 si no.
int d2c_hay(const char *ruta) {
    int fd = fs_open(ruta, O_RDONLY);
    if (fd == FILEHND_INVALID) {
        return 0;
    }
    fs_close(fd);
    return 1;
}

// Directorios (os.ReadDir). El nombre se copia al buffer de Go y el tipo se devuelve crudo: la
// convencion de KOS es la POSIX (dirent.h), la misma que documenta libgodc/kos/fs.go (DT_DIR = 4,
// DT_REG = 8), pero el POC imprime el valor crudo para medirlo en vez de suponerlo.
// Stat por nombre: fs_stat (KOS) llena el struct stat de newlib; solo se usan tamano y modo.
// Devuelve -1 si la ruta no esta.
int d2c_stat_tam(const char *ruta) {
    struct stat st;
    if (stat(ruta, &st) < 0) {
        return -1;
    }
    return (int)st.st_size;
}

int d2c_stat_es_dir(const char *ruta) {
    struct stat st;
    if (stat(ruta, &st) < 0) {
        return -1;
    }
    return (st.st_mode & S_IFDIR) ? 1 : 0;
}

// OJO con los directorios: NO hay forma de listar el romdisk desde el VFS de KOS. Se probo todo
// (medido con la sonda-i y con el POC del VFS): fs_open("/rd") da INVALID con las tres formas de la
// ruta y tambien con fs_readdir sobre un file_t; fs_total y stat de un directorio dan -1. Los
// ARCHIVOS de /rd si se abren y se leen. Por eso el listado (os.ReadDir) sale de un .manifiesto que
// el build hornea DENTRO del romdisk junto con los archivos.
// d2c_escribir_consola: la salida de os.Stdout en la consola serie. dbgio_xlate convierte el salto
// de linea LF en CRLF, que es lo que espera el terminal del emulador.
void d2c_escribir_consola(unsigned buf, int n) {
    dbgio_write_buffer_xlat((const uint8 *)buf, (size_t)n);
}

// d2c_salir: os.Exit no puede volver al runtime de Go (no hay procesos): se queda en un lazo.
void d2c_salir(int codigo) {
    (void)codigo;
    for (;;) {
        thd_pass();
    }
}

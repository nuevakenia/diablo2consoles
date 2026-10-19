// Puentes a KOS para el paquete os de la stdlib minima.
//
// La consola no tiene sistema de archivos de Go: los archivos viven en el romdisk (los que viajan
// dentro del ELF/GDI) o en el GD-ROM. KOS ya tiene el VFS, asi que el paquete os de Go habla con el
// por estas funciones en vez de reimplementar un FS.
//
// El romdisk se arma con genromfs + bin2o durante el build (ver scripts/console/build-dc.sh) y queda
// montado en /rd por KOS al arrancar (KOS_INIT_ROMDISK).
#include <errno.h>
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

/* ================================================================================================
 * DC-002.17: EL SISTEMA DE ARCHIVOS ESCRIBIBLE MINIMO, EN RAM (32 KiB de .bss, CERO del monton)
 * ================================================================================================
 *
 * POR QUE EXISTE: el romdisk de KOS es de SOLO LECTURA, y el motor ESCRIBE. Medido en el arranque
 * (evidencia/dc-002.17-escritura-config.txt): lo unico que pide escribir es
 *
 *     OS-ESCRITURA MkdirAll path= /rd/OpenDiablo2
 *     MOTOR-ARRANQUE Run() termino con error: operation not supported on read-only filesystem
 *
 * o sea `d2config.Save()` -> `os.MkdirAll("/rd/OpenDiablo2")` -> nuestro stub -> error -> `Run()` se cae.
 * El motor no arranca porque no puede guardar su configuracion, no por un defecto suyo.
 *
 * POR QUE EN .bss Y NO EN EL MONTON: al motor le quedan ~300 KiB libres de los 2,73 MB de monton (el
 * GC se lleva 1 MB de semiespacios y el motor 1,33 MB de arena) y el video ya se arreglo sacandole un
 * pedido de 1 MiB. Un tmpfs con malloc volveria a pelear por la misma memoria que falta: este ocupa un
 * presupuesto FIJO y declarado (8 archivos x 2.048 bytes + 8 directorios), no crece y no puede fallar
 * por fragmentacion.
 *
 * QUE SOPORTA (lo que el motor pide hoy, ni una cosa mas): crear/truncar, escribir, leer (con posicion y
 * seek), tamano, borrar, renombrar, truncar y "existe". Los archivos de la RAM TAPAN a los del romdisk
 * con el mismo nombre (asi un config guardado se lee de vuelta), y lo que no esta en RAM se busca en
 * KOS como siempre.
 *
 * LOS DESCRIPTORES: los de KOS son enteros chicos, asi que los de la RAM arrancan en 1000 y cada
 * funcion que recibe un fd pregunta primero si es de RAM. No hay tabla de archivos abiertos: el slot de
 * RAM ES el archivo (una sola posicion por archivo, que es lo que el motor usa).
 */
#define D2C_RAMFS_ARCHIVOS       8
#define D2C_RAMFS_BYTES_ARCHIVO  2048
#define D2C_RAMFS_RUTA           96
#define D2C_RAMFS_DIRS           8
#define D2C_RAMFS_FD_BASE        1000

typedef struct {
    int usado;
    int tam;
    int pos;
    char ruta[D2C_RAMFS_RUTA];
    unsigned char datos[D2C_RAMFS_BYTES_ARCHIVO];
} d2c_ramfs_archivo;

static d2c_ramfs_archivo ramfs_archivos[D2C_RAMFS_ARCHIVOS];
static char ramfs_dirs[D2C_RAMFS_DIRS][D2C_RAMFS_RUTA];
static int ramfs_ndirs = 0;

static int ramfs_es_fd(int fd) {
    return fd >= D2C_RAMFS_FD_BASE && fd < D2C_RAMFS_FD_BASE + D2C_RAMFS_ARCHIVOS;
}

static d2c_ramfs_archivo *ramfs_de_fd(int fd) {
    return &ramfs_archivos[fd - D2C_RAMFS_FD_BASE];
}

static int ramfs_indice(const char *ruta) {
    int i;
    for (i = 0; i < D2C_RAMFS_ARCHIVOS; i++) {
        if (ramfs_archivos[i].usado && strcmp(ramfs_archivos[i].ruta, ruta) == 0) {
            return i;
        }
    }
    return D2C_NO_VALIDO;
}

/* Copia la ruta al slot. Si no entra, NO se guarda un nombre cortado: un archivo con el nombre
 * truncado se leeria como otro archivo (y el motor no lo encontraria nunca). Devuelve 0 si entro. */
static int ramfs_copiar_ruta(char *destino, const char *ruta) {
    size_t n = strlen(ruta);
    if (n >= D2C_RAMFS_RUTA) {
        return -1;
    }
    memcpy(destino, ruta, n + 1);
    return 0;
}

/* Crea (o trunca, si ya existe) el archivo. Devuelve el indice del slot o -1 si no hay lugar. */
static int ramfs_crear(const char *ruta) {
    int i = ramfs_indice(ruta);

    if (i == D2C_NO_VALIDO) {
        for (i = 0; i < D2C_RAMFS_ARCHIVOS; i++) {
            if (!ramfs_archivos[i].usado) {
                break;
            }
        }
        if (i == D2C_RAMFS_ARCHIVOS) {
            return -1;      /* sin slot: es el limite declarado (8 archivos) */
        }
        if (ramfs_copiar_ruta(ramfs_archivos[i].ruta, ruta) != 0) {
            return -1;      /* ruta demasiado larga: limite declarado */
        }
        ramfs_archivos[i].usado = 1;
    }

    ramfs_archivos[i].tam = 0;
    ramfs_archivos[i].pos = 0;
    return i;
}

static int ramfs_borrar(const char *ruta) {
    int i = ramfs_indice(ruta);
    if (i == D2C_NO_VALIDO) {
        return -1;
    }
    ramfs_archivos[i].usado = 0;
    ramfs_archivos[i].tam = 0;
    ramfs_archivos[i].pos = 0;
    ramfs_archivos[i].ruta[0] = '\0';
    return 0;
}

// d2c_abrir: abre con un modo de KOS (los modos los arma el Go, ver os.go).
int d2c_abrir(const char *ruta, int modo) {
    int fd;
    if (ramfs_indice(ruta) != D2C_NO_VALIDO) {
        /* esta en la RAM: se abre de ahi (lectura o escritura; el truncado lo hace d2c_abrir_crear) */
        int i = ramfs_indice(ruta);
        ramfs_archivos[i].pos = 0;
        return D2C_RAMFS_FD_BASE + i;
    }
    fd = fs_open(ruta, modo);
    return fd == FILEHND_INVALID ? D2C_NO_VALIDO : fd;
}

// d2c_abrir_leer: solo lectura (es lo que usa os.Open y os.ReadFile).
int d2c_abrir_leer(const char *ruta) {
    int i = ramfs_indice(ruta);
    int fd;
    if (i != D2C_NO_VALIDO) {
        ramfs_archivos[i].pos = 0;
        return D2C_RAMFS_FD_BASE + i;
    }
    fd = fs_open(ruta, O_RDONLY);
    return fd == FILEHND_INVALID ? D2C_NO_VALIDO : fd;
}

// d2c_abrir_crear: escribe, creando o truncando. En el romdisk NO se puede escribir, asi que todo lo que
// se crea va a la RAM. Devuelve -1 solo si no hay slot o la ruta no entra (los dos limites declarados).
int d2c_abrir_crear(const char *ruta) {
    int i = ramfs_crear(ruta);
    if (i < 0) {
        return D2C_NO_VALIDO;
    }
    return D2C_RAMFS_FD_BASE + i;
}

// Los buffers llegan como direccion (unsigned) para que el ABI sea una sola palabra por parametro.
int d2c_leer(int fd, unsigned buf, int n) {
    d2c_ramfs_archivo *a;
    int quedan;
    if (!ramfs_es_fd(fd)) {
        return fs_read(fd, (void *)buf, (size_t)n);
    }
    a = ramfs_de_fd(fd);
    quedan = a->tam - a->pos;
    if (quedan <= 0) {
        return 0;              /* fin del archivo (el Go lo traduce a io.EOF) */
    }
    if (n > quedan) {
        n = quedan;
    }
    memcpy((void *)buf, a->datos + a->pos, (size_t)n);
    a->pos += n;
    return n;
}

/* d2c_escribir_archivo. Devuelve los bytes escritos, o -1 si no se pudo. Si lo que se escribe no entra
 * en el presupuesto del archivo, se escribe lo que entra: el llamador ve un corto y decide (no se miente
 * con un "escribi todo"). */
int d2c_escribir_archivo(int fd, unsigned buf, int n) {
    d2c_ramfs_archivo *a;
    int lugar;
    if (!ramfs_es_fd(fd)) {
        return fs_write(fd, (const void *)buf, (size_t)n);
    }
    a = ramfs_de_fd(fd);
    lugar = D2C_RAMFS_BYTES_ARCHIVO - a->pos;
    if (lugar <= 0) {
        return 0;
    }
    if (n > lugar) {
        n = lugar;
    }
    memcpy(a->datos + a->pos, (const void *)buf, (size_t)n);
    a->pos += n;
    if (a->pos > a->tam) {
        a->tam = a->pos;
    }
    return n;
}

int d2c_cerrar(int fd) {
    if (ramfs_es_fd(fd)) {
        return 0;              /* el slot queda: es el archivo, no una tabla de abiertos */
    }
    return fs_close(fd);
}

// tam: cuantos bytes tiene el archivo (fs_total). Devuelve -1 si no se puede.
int d2c_tam(int fd) {
    if (ramfs_es_fd(fd)) {
        return ramfs_de_fd(fd)->tam;
    }
    return (int)fs_total(fd);
}

int d2c_estoy(int fd) {
    if (ramfs_es_fd(fd)) {
        return ramfs_de_fd(fd)->pos;
    }
    return (int)fs_tell(fd);
}

/* d2c_buscar: mueve la posicion del archivo (fs_seek). whence: 0 = principio, 1 = actual, 2 = final,
 * los mismos numeros que io.SeekStart/SeekCurrent/SeekEnd. Devuelve la posicion nueva, o -1. */
int d2c_buscar(int fd, int offset, int whence) {
    d2c_ramfs_archivo *a;
    int nueva;

    if (!ramfs_es_fd(fd)) {
        return (int)fs_seek(fd, offset, whence);
    }

    a = ramfs_de_fd(fd);
    switch (whence) {
        case 0: nueva = offset; break;
        case 1: nueva = a->pos + offset; break;
        case 2: nueva = a->tam + offset; break;
        default: return -1;
    }
    if (nueva < 0) {
        nueva = 0;
    }
    a->pos = nueva;
    return nueva;
}

// d2c_hay: 1 si la ruta existe (en la RAM primero, y si no en el romdisk), 0 si no.
int d2c_hay(const char *ruta) {
    int fd;
    if (ramfs_indice(ruta) != D2C_NO_VALIDO) {
        return 1;
    }
    fd = fs_open(ruta, O_RDONLY);
    if (fd == FILEHND_INVALID) {
        return 0;
    }
    fs_close(fd);
    return 1;
}

/* d2c_mkdir: crea un directorio. El romdisk no se puede, asi que se anota en la lista de directorios de
 * la RAM (que es lo que mira d2c_stat_es_dir). Devuelve 0 si quedo anotado, -1 si no hay lugar o la ruta
 * es muy larga. Idempotente: si ya estaba, devuelve 0. */
int d2c_mkdir(const char *ruta) {
    int i;
    for (i = 0; i < ramfs_ndirs; i++) {
        if (strcmp(ramfs_dirs[i], ruta) == 0) {
            return 0;
        }
    }
    if (ramfs_ndirs >= D2C_RAMFS_DIRS) {
        return -1;
    }
    if (ramfs_copiar_ruta(ramfs_dirs[ramfs_ndirs], ruta) != 0) {
        return -1;
    }
    ramfs_ndirs++;
    return 0;
}

int d2c_borrar_archivo(const char *ruta) { return ramfs_borrar(ruta); }

/* d2c_renombrar: renombra un archivo de la RAM. Si el destino ya existe, lo pisa (como os.Rename). */
int d2c_renombrar(const char *viejo, const char *nuevo) {
    int i = ramfs_indice(viejo);
    int j;
    if (i == D2C_NO_VALIDO) {
        return -1;
    }
    j = ramfs_indice(nuevo);
    if (j != D2C_NO_VALIDO && j != i) {
        ramfs_archivos[j].usado = 0;
    }
    return ramfs_copiar_ruta(ramfs_archivos[i].ruta, nuevo);
}

// d2c_truncar: corta (o estira con ceros) un archivo de la RAM.
int d2c_truncar(const char *ruta, int tam) {
    int i = ramfs_indice(ruta);
    if (i == D2C_NO_VALIDO || tam < 0 || tam > D2C_RAMFS_BYTES_ARCHIVO) {
        return -1;
    }
    if (tam > ramfs_archivos[i].tam) {
        memset(ramfs_archivos[i].datos + ramfs_archivos[i].tam, 0,
               (size_t)(tam - ramfs_archivos[i].tam));
    }
    ramfs_archivos[i].tam = tam;
    if (ramfs_archivos[i].pos > tam) {
        ramfs_archivos[i].pos = tam;
    }
    return 0;
}

/* Un resumen del uso, para la evidencia: cuantos archivos hay, cuantos bytes y cuantos directorios. Se
 * pide desde Go (una sola vez, al final) y no se imprime solo, para no ensuciar el log. */
int d2c_ramfs_resumen(int *archivos, int *bytes, int *dirs) {
    int i, n = 0, b = 0;
    for (i = 0; i < D2C_RAMFS_ARCHIVOS; i++) {
        if (ramfs_archivos[i].usado) {
            n++;
            b += ramfs_archivos[i].tam;
        }
    }
    if (archivos) *archivos = n;
    if (bytes) *bytes = b;
    if (dirs) *dirs = ramfs_ndirs;
    return 0;
}

// Cuanto ocupa el sistema de archivos en RAM (en .bss), para la evidencia.
int d2c_ramfs_tam(void) {
    return (int)(sizeof(ramfs_archivos) + sizeof(ramfs_dirs) + sizeof(int));
}

/* DC-002.19: EL LISTADO DEL OVERLAY DE LA RAM (os.ReadDir de un directorio del overlay).
 *
 * POR QUE EXISTE: el os.ReadDir de la consola salia SOLO del ".manifiesto" que el build hornea en el
 * romdisk, asi que un directorio que existe en el OVERLAY (lo que el programa creo, o lo que el motor
 * escribio) se veia VACIO. Medido en DC-002.19 con el motor de verdad, sobre una carpeta con siete
 * archivos que d2c_stat_tam SI encontraba:
 *
 *     SETUP-ARCHIVOS A: Validate(dir).OK() = false archivos que ve = 0
 *     SETUP-ARCHIVOS A: Complete(dir, orden) = true
 *
 * Y no es cosmetico: el camino de la seleccion de los archivos del juego del motor (d2setup.Validate ->
 * Resolve) pregunta justamente por el contenido del directorio.
 *
 * QUE HACE: escribe en `buf` el MISMO formato del manifiesto del build (nombre TAB bytes TAB
 * archivo|directorio, una linea por entrada DIRECTA de ese directorio; no baja a los subdirectorios) y
 * devuelve los BYTES escritos. Devuelve -1 si no entran en `max` -- mejor fallar que devolver una lista
 * cortada y que el que la lea crea que el directorio tiene menos de lo que tiene.
 *
 * LO QUE NO LISTA: ".manifiesto" (es el INDICE del romdisk, no una entrada; el ReadDir de Go no lo
 * veria porque en el romdisk no es un archivo del usuario) ni el propio directorio.
 */
/*
 * OJO MEDIDO (DC-002.19): la RUTA VA ULTIMA. Con la firma `(const char *ruta, char *buf, int max)` el C
 * recibia los argumentos CORRIDOS y el guardia de abajo devolvia -2 (medido: "OS-RAMFS
 * manifiesto-de-la-ram: no se pudo armar para /rd/ov219 codigo -2" con `max` llegando como un puntero
 * negativo). La convencion que funciona es escalares primero y la ruta al final: el orden de esta firma
 * es el CONTRATO con la declaracion del paquete os (os.go) y no se puede cambiar de un lado solo.
 * La comprobacion `ruta[0] != '/'` es a proposito: si la ruta no llega como una cadena de verdad
 * (puntero a los bytes), esto lo dice con un -5 en vez de listar cualquier cosa.
 */
int d2c_ramfs_manifiesto(char *buf, int max, const char *ruta) {
    int i, usados = 0, escritos = 0;
    size_t n;
    char prefijo[D2C_RAMFS_RUTA + 1];

    // Codigos NEGATIVOS distintos por salida: el que llama los distingue y los imprime (es el instrumento
    // que encontro el desajuste de ABI de arriba).
    if (ruta == 0 || buf == 0 || max <= 0) {
        return -2;
    }
    if (ruta[0] != '/') {
        return -5;
    }
    // el prefijo "<ruta>/" que tienen que empezar las entradas directas de ese directorio
    n = strlen(ruta);
    if (n + 1 >= sizeof(prefijo)) {
        return -3;
    }
    memcpy(prefijo, ruta, n);
    prefijo[n] = '/';
    prefijo[n + 1] = '\0';

    for (i = 0; i < D2C_RAMFS_ARCHIVOS + D2C_RAMFS_DIRS; i++) {
        const char *nombre;
        const char *tipo;
        int tam;
        size_t k, l;

        if (i < D2C_RAMFS_ARCHIVOS) {
            if (!ramfs_archivos[i].usado) {
                continue;
            }
            nombre = ramfs_archivos[i].ruta;
            tam = ramfs_archivos[i].tam;
            tipo = "archivo";
        } else {
            int d = i - D2C_RAMFS_ARCHIVOS;
            if (d >= ramfs_ndirs) {
                continue;
            }
            nombre = ramfs_dirs[d];
            tam = 0;
            tipo = "directorio";
        }

        l = strlen(nombre);
        if (l <= n + 1 || strncmp(nombre, prefijo, n + 1) != 0) {
            continue;   // no esta adentro de este directorio
        }
        nombre += n + 1;    // el nombre, sin la ruta del directorio
        l -= n + 1;
        for (k = 0; k < l; k++) {
            if (nombre[k] == '/') {
                break;      // esta en un subdirectorio: no es una entrada DIRECTA
            }
        }
        if (k < l) {
            continue;
        }
        if (l == 12 && strncmp(nombre, ".manifiesto", 11) == 0) {
            continue;       // el indice del romdisk no se lista
        }
        if (l >= D2C_RAMFS_RUTA) {
            continue;
        }

        // "<nombre>\t<tam>\t<tipo>\n" (el tamano en decimal, armado a mano: no hay snprintf aca)
        {
            char numero[12];
            int p = 0, q;
            char cifra[12];

            if (tam == 0) {
                cifra[p++] = '0';
            }
            while (tam > 0 && p < (int)sizeof(cifra)) {
                cifra[p++] = (char)('0' + (tam % 10));
                tam /= 10;
            }
            for (q = 0; q < p; q++) {
                numero[q] = cifra[p - 1 - q];
            }
            numero[p] = '\0';
            if (escritos + (int)l + (int)strlen(numero) + (int)strlen(tipo) + 3 > max) {
                return -1;
            }
            memcpy(buf + escritos, nombre, l);
            escritos += (int)l;
            buf[escritos++] = '\t';
            for (q = 0; q < p; q++) {
                buf[escritos++] = numero[q];
            }
            buf[escritos++] = '\t';
            memcpy(buf + escritos, tipo, strlen(tipo));
            escritos += (int)strlen(tipo);
            buf[escritos++] = '\n';
        }
        usados++;
    }
    (void)usados;
    if (escritos == 0) {
        // INSTRUMENTO: nadie matcheo. Se devuelve el ESTADO del overlay (cuantos archivos usados y
        // cuantos directorios ve ESTE codigo) para poder leerlo desde Go: -(100 + 10*archivos + dirs).
        int narch = 0;
        for (i = 0; i < D2C_RAMFS_ARCHIVOS; i++) {
            if (ramfs_archivos[i].usado) {
                narch++;
            }
        }
        return -(100 + 10 * narch + ramfs_ndirs);
    }
    return escritos;
}

/* ================================================================================================
 * DC-002.45: EL LISTADO DE UN DIRECTORIO DEL DISCO POR EL `fs_readdir` DEL DRIVER (camino DIRECTO).
 * ================================================================================================
 *
 * POR QUE EXISTE: DC-002.44 cerro el defecto (`/cd/juego` ve 0 -> 3 archivos) con el MANIFIESTO que hornea
 * `mkdisco.sh` en `/JUEGO/MANIFIESTO.TXT`. Ese listado es del HORNEADOR, no del sistema de archivos: lista
 * los `*.mpq` y `*.txt` que ese script puso, y un archivo que este en el disco pero que el horneador no
 * conozca NO SALE. Lo que mide esta tarjeta es el camino de fondo: preguntarle al DRIVER.
 *
 * Y EL DRIVER SABE (medido en DC-002.44, seccion 1-BIS, con el POC de C `disco-c`): sobre una ISO SIN el
 * arbol Joliet, `fs_open("/cd/JUEGO", O_RDONLY|O_DIR)` da un fd valido y `fs_readdir` devuelve las 6
 * entradas con nombre, bytes y attr. Lo que faltaba era el PUENTE: `os.ReadDir` nunca lo llamaba.
 *
 * LA MEMORIA: el buffer es el del que llama (Go, LOCAL y de tamano fijo), igual que el manifiesto del
 * overlay: en el arranque del motor el monton esta justo (3.136 B libres medidos en DC-002.34) y un
 * listado no puede pedirle memoria. KOS usa su propia cache de sectores para el directorio, asi que aca
 * no se reserva nada tampoco.
 *
 * EL FORMATO DE SALIDA es EL MISMO del manifiesto -- "nombre TAB bytes TAB archivo|directorio", una linea
 * por entrada -- para que el que lo lea sea el mismo codigo de Go (`entradasDelManifiesto`). Se saltan
 * "." y ".." (los pone `fs_readdir` de fs.c, no son entradas del usuario; el manifiesto tampoco los trae) y
 * el `.manifiesto`/`MANIFIESTO.TXT` del propio directorio (es el indice, no una entrada del usuario).
 *
 * DEVUELVE: los bytes escritos, o un NEGATIVO por cada salida (el mismo instrumento que el del overlay):
 *   -1  no entran en `max`            (mejor fallar que devolver una lista cortada)
 *   -2  argumentos invalidos
 *   -3  la ruta no entra en el buffer interno
 *   -4  el driver no pudo abrir el directorio (no es un directorio, o no esta) -> el que llama decide
 *   -5  la ruta no llego como cadena de verdad
 *   -6  `fs_readdir` fallo a mitad del recorrido
 */
/* es_indice_del_directorio: los nombres que son el INDICE del directorio y no una entrada del usuario.
 * Se comparan sin distinguir mayusculas porque el driver del ISO9660 pasa los nombres a minusculas
 * (fs_iso9660.c:fn_postprocess) y el romdisk los deja como estan. */
static int nombre_es_indice(const char *nombre) {
    const char *a = ".manifiesto";
    const char *b = "MANIFIESTO.TXT";
    size_t i;

    for (i = 0; a[i] != '\0' && nombre[i] != '\0'; i++) {
        char c = nombre[i];
        if (c >= 'A' && c <= 'Z') {
            c = (char)(c - 'A' + 'a');
        }
        if (c != a[i]) {
            break;
        }
    }
    if (a[i] == '\0' && nombre[i] == '\0') {
        return 1;
    }

    for (i = 0; b[i] != '\0' && nombre[i] != '\0'; i++) {
        char c = nombre[i];
        if (c >= 'a' && c <= 'z') {
            c = (char)(c - 'a' + 'A');
        }
        if (c != b[i]) {
            break;
        }
    }
    return b[i] == '\0' && nombre[i] == '\0';
}

int d2c_iso_listar(char *buf, int max, const char *ruta) {
    dirent_t *de;
    int fd, escritos = 0, entradas = 0;
    size_t n;
    char prefijo[D2C_RAMFS_RUTA + 1];

    if (ruta == 0 || buf == 0 || max <= 0) {
        return -2;
    }
    if (ruta[0] != '/') {
        return -5;
    }

    n = strlen(ruta);
    if (n + 2 >= sizeof(prefijo)) {
        return -3;
    }
    memcpy(prefijo, ruta, n);
    if (n == 0 || prefijo[n - 1] != '/') {
        prefijo[n] = '/';
        n++;
    }
    prefijo[n] = '\0';

    errno = 0;
    fd = fs_open(ruta, O_RDONLY | O_DIR);
    if (fd < 0) {
        return -4;
    }

    for (;;) {
        size_t l;
        int tam;
        const char *tipo;
        char numero[12];
        int p = 0, q;
        char cifra[12];

        errno = 0;
        de = fs_readdir(fd);
        if (de == NULL) {
            /* errno == 0 es el fin normal del directorio; distinto de 0 es una falla del driver */
            if (errno != 0) {
                fs_close(fd);
                return -6;
            }
            break;
        }

        /* "." y ".." los pone el VFS (fs.c:fs_readdir), no son entradas del usuario */
        if (strcmp(de->name, ".") == 0 || strcmp(de->name, "..") == 0) {
            continue;
        }
        /* el indice del directorio no se lista (misma regla que el manifiesto del overlay).
         * OJO: el driver pasa los nombres a MINUSCULAS (fs_iso9660.c:fn_postprocess hace tolower), asi que
         * "MANIFIESTO.TXT" llega como "manifiesto.txt". Se compara en minusculas las dos formas. */
        if (nombre_es_indice(de->name)) {
            continue;
        }
        if (entradas >= 64) {
            break;      /* el tope del que llama: una lista truncada se declara, no se inventa */
        }

        l = strlen(de->name);
        if (l == 0) {
            continue;
        }
        /* el driver marca los directorios con attr = O_DIR y size = -1 (fs_iso9660.c:920) */
        if (de->attr & O_DIR) {
            tam = 0;
            tipo = "directorio";
        } else {
            tam = de->size;
            if (tam < 0) {
                tam = 0;
            }
            tipo = "archivo";
        }

        if (tam == 0) {
            cifra[p++] = '0';
        }
        while (tam > 0 && p < (int)sizeof(cifra)) {
            cifra[p++] = (char)('0' + (tam % 10));
            tam /= 10;
        }
        for (q = 0; q < p; q++) {
            numero[q] = cifra[p - 1 - q];
        }
        numero[p] = '\0';

        if (escritos + (int)l + (int)strlen(numero) + (int)strlen(tipo) + 3 > max) {
            fs_close(fd);
            return -1;
        }
        memcpy(buf + escritos, de->name, l);
        escritos += (int)l;
        buf[escritos++] = '\t';
        for (q = 0; q < p; q++) {
            buf[escritos++] = numero[q];
        }
        buf[escritos++] = '\t';
        memcpy(buf + escritos, tipo, strlen(tipo));
        escritos += (int)strlen(tipo);
        buf[escritos++] = '\n';
        entradas++;
    }

    fs_close(fd);
    return escritos;
}

// Directorios (os.ReadDir). El nombre se copia al buffer de Go y el tipo se devuelve crudo: la
// convencion de KOS es la POSIX (dirent.h), la misma que documenta libgodc/kos/fs.go (DT_DIR = 4,
// DT_REG = 8), pero el POC imprime el valor crudo para medirlo en vez de suponerlo.
// Stat por nombre: fs_stat (KOS) llena el struct stat de newlib; solo se usan tamano y modo.
// Devuelve -1 si la ruta no esta.
static int tam_por_apertura(const char *ruta);

int d2c_stat_tam(const char *ruta) {
    struct stat st;
    int i = ramfs_indice(ruta);
    if (i != D2C_NO_VALIDO) {
        return ramfs_archivos[i].tam;
    }
    if (stat(ruta, &st) < 0) {
        return tam_por_apertura(ruta);
    }
    return (int)st.st_size;
}

// DC-002.27: EL RESPALDO QUE HACE QUE EL MOTOR VEA LOS ARCHIVOS DEL DISCO.
//
// Medido con el POC motor-disco (disco.gdi, /cd): `stat("/cd/JUEGO/PATCH_D2.MPQ")` de newlib/KOS da -1
// -- el mismo `stat()` que SI contesta el romdisk (/rd) -- mientras que `fs_open` + `fs_total` del MISMO
// archivo dan 5.022.588 bytes, el tamano exacto del MPQ del PC. El camino del motor que decide "los
// archivos del juego estan" es `d2setup.Complete`, que usa `os.Stat`: sin este respaldo el arranque del
// disco se queda en "game files not found at /cd/juego". El respaldo abre y pregunta el tamano, que es lo
// que el driver del iso9660 si implementa (fs_total -> iso_total); si el archivo no esta, `fs_open` falla
// igual y el Stat devuelve el error que corresponde.
static int tam_por_apertura(const char *ruta) {
    int fd = fs_open(ruta, O_RDONLY);
    int64_t total;

    if (fd < 0) {
        return -1;
    }

    total = fs_total(fd);
    fs_close(fd);

    if (total < 0) {
        return -1;
    }

    return (int)total;
}

int d2c_stat_es_dir(const char *ruta) {
    struct stat st;
    int i;
    for (i = 0; i < ramfs_ndirs; i++) {
        if (strcmp(ramfs_dirs[i], ruta) == 0) {
            return 1;
        }
    }
    if (stat(ruta, &st) < 0) {
        // el mismo respaldo, para los directorios: en el disco el que contesta es iso_open con O_DIR
        int fd = fs_open(ruta, O_RDONLY | O_DIR);
        if (fd >= 0) {
            fs_close(fd);
            return 1;
        }
        return -1;
    }
    return (st.st_mode & S_IFDIR) ? 1 : 0;
}

// OJO con los directorios: NO hay forma de listar el romdisk desde el VFS de KOS. Se probo todo
// (medido con la sonda-i y con el POC del VFS): fs_open("/rd") da INVALID con las tres formas de la
// ruta y tambien con fs_readdir sobre un file_t; fs_total y stat de un directorio dan -1. Los
// ARCHIVOS de /rd si se abren y se leen. Por eso el listado (os.ReadDir) sale de un .manifiesto que
// el build hornea DENTRO del romdisk junto con los archivos.
// El reloj de KOS, medido en sus cabeceras (kernel/arch/dreamcast/include/arch/):
//   - rtc_unix_secs()  -> los segundos Unix que trae la RTC del hardware (el doc avisa que puede fallar:
//     si la RTC no esta seteada devuelve 0, y ese 0 es "1970").
//   - timer_us_gettime64() -> microsegundos desde que arranco la consola.
//   - thd_sleep(ms)    -> el sleep de hilos, en MILISEGUNDOS.
#include <arch/rtc.h>
#include <arch/timer.h>
#include <kos/thread.h>

int64_t d2c_reloj_unix(void) {
    return (int64_t)rtc_unix_secs();
}

uint64_t d2c_reloj_us(void) {
    return timer_us_gettime64();
}

void d2c_dormir(int ms) {
    thd_sleep(ms);
}

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

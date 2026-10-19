/* DC-002.43 -- LAS MARCAS DE CARGA DEL FILE LOADER / ASSET MANAGER, CON LA MEMORIA EN ESE INSTANTE.
 *
 * QUE PROBLEMA RESUELVE: el arranque del motor se muere MUDO (la consola reinicia sin imprimir un caracter).
 * Las lineas `[File Loader] Loaded ...` salen DESPUES de que la carga salio bien, asi que no dicen cual era
 * el pedido en curso cuando murio. Con una marca al ENTRAR y al VOLVER de cada carga -- y con el estado de la
 * memoria pegado a la marca -- la ultima marca de la corrida dice la llamada exacta Y con cuanto margen.
 *
 * LO QUE IMPRIME, POR MARCA:
 *
 *   ARQ-CARGA <entra|volvio|lee-entra|lee-volvio> <ruta> cfree=<n> cused=<n> gsemi=<usado>/<tope> gc=<n>
 *
 *   cfree / cused  el monton de C, con el MISMO par de numeros que imprime el runtime en su linea
 *                  `Memory: arena=%d used=%d free=%d` (ahora son `mi.fordblks` y `mi.uordblks` de
 *                  `mallinfo()`, que es de donde el runtime los saca: ver
 *                  ~/dreamcast/libgodc/runtime/defer_dreamcast.c). `cfree` es lo que le queda al monton de C
 *                  para reservar de una sola vez; es el numero que no existia en la muerte muda.
 *   gsemi          el semiespacio del GC de Go: usado / tope (`gc_stats()` de runtime/gc_heap.c). Es la otra
 *                  pared: el arranque muere cuando el usado llega al tope.
 *   gc             cuantas recolecciones lleva el GC en ese momento.
 *
 * POR QUE ACÁ (console/dc/std/csrc/) Y NO EN EL POC: este C lo enlazan TODOS los binarios de la consola
 * (build-dc.sh y build-motor-arranque.sh recorren este directorio), asi que cualquier POC que enlace el motor
 * -- y el motor entero -- tiene el simbolo `d2c_marca_carga` sin tocar un solo POC.
 *
 * POR QUE dbgio_printf Y NO printf: el printf de newlib depende del buffering de la libc; si el motor se
 * cuelga en la linea siguiente, la marca tiene que estar YA afuera. dbgio_printf escribe con el dispositivo
 * de consola de KOS (scif) y usa un buffer ESTATICO (kos/kernel/debug/dbgio.c: `printf_buf`), o sea que no
 * reserva memoria: la marca no puede mover los numeros que esta midiendo.
 */
#include <kos.h>
#include <malloc.h>
#include <stddef.h>
#include <stdint.h>

/* del runtime de libgodc (runtime/gc_heap.c): lo que lleva usado el semiespacio del GC y cuantas
 * recolecciones van. Es el mismo numero que la marca `gc-por-umbral-bytes-usados` del runtime. */
void gc_stats(size_t *used, size_t *total, uint32_t *collections);

static const char *etiqueta_de(int punto)
{
    switch (punto) {
    case 0: return "instrumento"; /* la marca de control que imprime el paquete al arrancar */
    case 1: return "entra";       /* entrando a Loader.Load */
    case 2: return "volvio";      /* Loader.Load devolvio el archivo abierto */
    case 3: return "lee-entra";   /* entrando a la lectura completa del stream */
    case 4: return "lee-volvio";  /* la lectura completa devolvio los bytes */
    default: return "?";
    }
}

/* La marca de CONTROL la imprime el paquete al arrancar, ANTES de que main llame a d2c_serial_init(): con el
 * dispositivo de consola todavia en el default (el nulo) esa linea se perdia -- medido: la marca de control
 * no aparecia en la corrida y el instrumento parecia no estar puesto. Se abre la consola aca, igual que hacen
 * las marcas del runtime (marcas_arranque.c), y solo la primera vez. */
static void marca_abre_consola(void)
{
    static int abierta = 0;
    if (!abierta) {
        dbgio_dev_select("scif");
        abierta = 1;
    }
}

void d2c_marca_carga(int punto, const unsigned char *ruta, unsigned int largo)
{
    struct mallinfo mi;
    size_t gused = 0;
    size_t gtotal = 0;
    uint32_t gcoll = 0;
    char buf[96];
    unsigned int i;
    unsigned int n;

    /* la ruta recortada: la consola serie es lenta y lo que hace falta es el asset, no la ruta entera */
    n = largo;
    if (ruta == 0) {
        n = 0;
    }
    if (n > (unsigned int)(sizeof(buf) - 1)) {
        n = (unsigned int)(sizeof(buf) - 1);
    }
    for (i = 0; i < n; i++) {
        buf[i] = (char)ruta[i];
    }
    buf[n] = '\0';

    gc_stats(&gused, &gtotal, &gcoll);
    mi = mallinfo();

    marca_abre_consola();

    dbgio_printf("ARQ-CARGA %s %s cfree=%d cused=%d gsemi=%d/%d gc=%d\n",
                 etiqueta_de(punto), buf,
                 (int)mi.fordblks, (int)mi.uordblks,
                 (int)gused, (int)gtotal, (int)gcoll);
}

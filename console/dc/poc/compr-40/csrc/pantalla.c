/* Consola serie del POC compr-40 (DC-002.40). Es el mismo puente minimo que los otros POC del motor:
 * cada POC lleva su csrc, porque build-dc.sh / build-motor-arranque.sh compilan los .c del POC que se les
 * pida y no comparten objetos. */
#include <kos.h>
#include <malloc.h>
#include <unistd.h>
#include <stdint.h>
#include <string.h>

/* La consola serie del emulador (scif) es la unica salida de texto. */
void d2c_serial_init(void) {
    dbgio_dev_select("scif");
}

/* Dormir de verdad (KOS): el busy loop de Go del POC no servia ni para esperar -- medido, un
 * `esperarMs(4000)` tardaba ~970 ms del propio reloj del POC -- y ademas NO suelta la CPU, que es lo que
 * el montaje del iso9660 necesita para terminar. thd_sleep es del kernel y se la devuelve. */
void d2c_dormir_ms(int ms) {
    thd_sleep((uint32)ms);
}

/* Los tres numeros del MONTON DE C (mallinfo), la misma fuente que usa el runtime para contar el OOM.
 * Son para ver si el puente de la zlib lo va agotando vuelta tras vuelta: si `arena` crece y `libre` no
 * vuelve, hay fuga y el defecto es de memoria, no de datos. */
int d2c_monton_arena(void) { struct mallinfo mi = mallinfo(); return mi.arena; }
int d2c_monton_usado(void) { struct mallinfo mi = mallinfo(); return mi.uordblks; }
int d2c_monton_libre(void) { struct mallinfo mi = mallinfo(); return mi.fordblks; }

/* DONDE esta cada cosa: el tope del monton de C (sbrk), donde empieza, y una direccion de .bss. Con esto y
 * la direccion de un objeto del monton de Go se ve si los dos montones se estan tocando -- que es lo que
 * hace que la zlib lea basura cuando el monton crece. */
static int d2c_bss_marca = 0;
unsigned int d2c_sbrk_actual(void) { return (unsigned int)sbrk(0); }
unsigned int d2c_direccion_bss(void) { return (unsigned int)&d2c_bss_marca; }
int d2c_monton_estructuras(void) { struct mallinfo mi = mallinfo(); return mi.hblkhd; }

/* ================================================================================================
 * DC-002.40: LOS BUFFERS EN C, para el ultimo experimento de la tarjeta.
 *
 * La hipotesis que queda: cuando el monton de C se llena, los bloques que le pide la zlib (la ventana de
 * 32 KB y el estado de ~7 KB) dejan de ser exclusivos -- los semiespacios del monton de Go son bloques
 * `memalign` DENTRO del monton de C (medido: `gc_init-semiespacio0/1` caen dentro del arena), asi que un
 * bloque entregado de mas puede pisar objetos del monton de Go, y a la inversa. Con los buffers FUERA del
 * monton de Go (estos de .bss) y los mismos bytes, si la descompresion deja de fallar, el que se esta
 * pisando es el monton de Go; y si igual falla, el que no da abasto es el monton de C. */
static unsigned char d2c_buf_in[1024];
static unsigned char d2c_buf_out[8192];

unsigned int d2c_buffer_in(void) { return (unsigned int)(uintptr_t)d2c_buf_in; }
unsigned int d2c_buffer_out(void) { return (unsigned int)(uintptr_t)d2c_buf_out; }
void d2c_copiar_a_entrada(unsigned int src, int len) { memcpy(d2c_buf_in, (const void *)(uintptr_t)src, len); }
void d2c_copiar_de_salida(unsigned int dst, int len) { memcpy((void *)(uintptr_t)dst, d2c_buf_out, len); }




unsigned int d2c_ms(void) {
    static int encendido = 0;

    if (!encendido) {
        timer_ms_enable();
        encendido = 1;
    }

    return (unsigned int)(timer_ms_gettime64() & 0xffffffffu);
}

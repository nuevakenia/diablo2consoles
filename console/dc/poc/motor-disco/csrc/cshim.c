/* DC-002.27: lo minimo del C del POC motor-disco.
 *
 *   - la consola serie: sin dbgio_dev_select("scif") KOS deja el handler en NULL y no sale nada (medido
 *     desde DC-001.3). Es la unica salida que tiene la consola.
 *   - las marcas de avance: el motor puede quedarse en su cuadro de error, en el bucle de dibujo o
 *     colgado. Las marcas dicen POR DONDE IBA cuando eso pase.
 *   - el reloj (d2c_ms): para medir cuanto cuesta leer el MPQ del disco, con el temporizador de KOS
 *     (`TimerUsGettime64`, `TimerMsEnable`). Los contadores del SH-4 (PerfCntr*) devuelven 0 en Flycast:
 *     no se usan aca.
 */
#include <kos.h>
#include <malloc.h>
#include <stdint.h>
#include <unistd.h>

/* DC-002.28: por que la consola serie se abre en un CONSTRUCTOR y no solo en main.
 *
 * Medido en DC-002.28: el binario del motor con las tablas del MPQ por trozos (13.7 MB, el mismo que en
 * DC-002.27 arrancaba y dibujaba) muere MUDO: ni una marca, ni un `fatal error`, ni una excepcion de KOS.
 * El primer `println` de Go esta en el arranque de main y las marcas despues, asi que la muerte es ANTES
 * de main: o en la carga del ELF, o en el arranque del runtime de Go (el GC, los constructores de C).
 *
 * Sin `dbgio_dev_select("scif")` KOS deja el handler de la consola en NULL y NO sale nada (medido desde
 * DC-001.3): o sea que un aviso del runtime de Go (o el "Out of memory" de KOS) ANTES de que main abra la
 * consola se pierde, y encima el que lo escribe puede quedarse colgado en el handler NULL (de ahi la
 * muerte sin excepcion). Aca la consola se abre lo antes que se puede: en el constructor de C, con los
 * numeros del monton de C en ese instante (si el monton no tiene con que arrancar, se ve en la linea).
 *
 * El constructor puede no llegar a correr (depende del arranque que use el binario): si esta linea no sale
 * y la de `init` de Go si, lo que se aprende es eso. */
static int d2c_a_texto(unsigned int v, char *dst, int p) {
    char tmp[12];
    int n = 0;

    if (v == 0) {
        dst[p++] = '0';

        return p;
    }

    while (v > 0) {
        tmp[n++] = (char)('0' + (v % 10));
        v /= 10;
    }

    while (n > 0) {
        dst[p++] = tmp[--n];
    }

    return p;
}

static int d2c_hex_texto(unsigned int v, char *dst, int p) {
    const char *d = "0123456789abcdef";
    int i;

    dst[p++] = '0';
    dst[p++] = 'x';

    for (i = 7; i >= 0; i--) {
        dst[p++] = d[(v >> (i * 4)) & 0xf];
    }

    return p;
}

__attribute__((constructor))
static void d2c_consola_constructor(void) {
    char l[256];
    int p = 0;
    const char *fijo = "C-CONSTRUCTOR (antes de main): la consola serie ya esta; sbrk=";
    int i = 0;

    while (fijo[i] != '\0') {
        l[p++] = fijo[i++];
    }

    p = d2c_hex_texto((unsigned int)(uintptr_t)sbrk(0), l, p);

    fijo = " techo=";
    i = 0;
    while (fijo[i] != '\0') {
        l[p++] = fijo[i++];
    }

    p = d2c_hex_texto((unsigned int)(_arch_mem_top - THD_KERNEL_STACK_SIZE), l, p);

    fijo = " arena=";
    i = 0;
    while (fijo[i] != '\0') {
        l[p++] = fijo[i++];
    }

    p = d2c_a_texto((unsigned int)mallinfo().arena, l, p);

    fijo = " libre=";
    i = 0;
    while (fijo[i] != '\0') {
        l[p++] = fijo[i++];
    }

    p = d2c_a_texto((unsigned int)mallinfo().fordblks, l, p);
    l[p++] = '\r';
    l[p++] = '\n';

    dbgio_dev_select("scif");
    dbgio_write_buffer_xlat((const uint8 *)l, (size_t)p);
}

/* d2c_aviso_init: la llama un `var _ = ...` del paquete de Go, o sea ANTES de main pero DESPUES del
 * arranque del runtime de Go. Separa "murio el runtime" de "murio main".
 *
 * DC-002.29: POR QUE EL AVISO NO ESCRIBE POR EL CAMINO CRUDO DE LA CONSOLA.
 *
 * Medido (evidencia dc-002.29-*.txt): desde la cadena de `init` de Go, `dbgio_write_buffer_xlat` (el
 * camino crudo de dbgio: NO toma el candado, el mismo que la cabecera de dbgio.c marca "Not re-entrant")
 * CLAVA el arranque del binario REARMADO:
 *   - arranques=1, main.main=0, cuadros=0 y ni una linea mas: el constructor de C sale una vez, las
 *     marcas del runtime hasta `main_wrapper-entra` y despues silencio (ni una muestra de la sonda del PC,
 *     o sea el tick de KOS tambien queda muerto);
 *   - con los 75 bytes del aviso el guardian de la consola la reinicia a los ~2,4 s y vuelve a empezar
 *     (el constructor de C sale OTRA vez: eso era "un arranque que empieza dos veces" en DC-002.28);
 *   - con la longitud en 0 (misma llamada, cero bytes) se cuelga igual (sin reinicio): el cuelgue no esta
 *     en los bytes, esta en la LLAMADA al camino crudo desde este punto del arranque;
 *   - los MISMOS bytes por la marca del runtime (`dbgio_printf`, que si toma el candado) salen y el
 *     arranque sigue: main.main=1 y el motor corre hasta su cuadro.
 * El mismo aviso compilado igual en la camada de referencia (la de DC-002.27) andaba: el defecto depende
 * del momento del arranque (fragilidad de layout), no del texto.
 *
 * Por eso el aviso imprime con la marca del runtime y no con la escritura cruda. */
extern void godc_marca(const char *etiqueta, unsigned long valor);   /* libgodc: runtime/marcas_arranque.c */

void d2c_aviso_init(void) {
    godc_marca("MOTOR-DISCO init del paquete: el runtime de Go ya arranco (antes de main)", 0);
}

void d2c_serial_init(void) {
    dbgio_dev_select("scif");
}

/* d2c_ms: milisegundos desde que arranco el programa, con el temporizador de KOS. Se escribe con
 * dbgio_write_buffer_xlat y no con printf por lo mismo que las marcas: si el motor se cuelga en la linea
 * siguiente, la marca ya tiene que estar fuera. */
unsigned int d2c_ms(void) {
    static int encendido = 0;

    if (!encendido) {
        timer_ms_enable();
        encendido = 1;
    }

    return (unsigned int)(timer_ms_gettime64() & 0xffffffffu);
}

unsigned int d2c_disco_marca(unsigned int que) {
    static const char *marcas[] = {
        "",
        "MOTOR-DISCO marca 1: la sonda del disco (antes del motor)\r\n",
        "MOTOR-DISCO marca 2: entrando a d2app.Create()\r\n",
        "MOTOR-DISCO marca 3: Create() volvio, entrando a Run()\r\n",
        "MOTOR-DISCO marca 4: Run() volvio\r\n",
    };

    if (que >= 1 && que <= 4) {
        const char *m = marcas[que];
        int n = 0;
        while (m[n] != '\0') n++;
        dbgio_write_buffer_xlat((const uint8 *)m, (size_t)n);
    }

    return que;
}

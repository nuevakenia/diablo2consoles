/* DC-002.76: los puentes a C del POC del cooperativo.
 *
 * 1) LA CONSOLA SERIE. El `println` de Go en libgodc sale por el dispositivo de dbgio que este
 *    seleccionado, y el arranque de KOS no elige ninguno: sin esto la salida del POC no sale a ninguna
 *    parte (medido en DC-001.3). Es el mismo shim de los POC del enlace, del personaje y de los
 *    presupuestos.
 *
 * 2) EL MONTON, que el binding de Go no expone: mallinfo().fordblks es el MISMO numero que imprime el
 *    runtime cuando se queda sin memoria, asi que lo que se lee aca es comparable con el defecto.
 *
 * 3) EL RELOJ. timer_us_gettime64() es el reloj de la consola en microsegundos (64 bits, no se da vuelta
 *    en una corrida). Con el se cronometra la creacion de cada jugador y el trabajo por cuadro.
 *
 * 4) EL CONTADOR DEL GC. `marca_gc_n` es la variable de libgodc que incrementa gc_collect(): es de donde
 *    salen las lineas ARQ-MARCA gc_collect del log, pero leida desde Go para poder decir cuantas
 *    recolecciones cayeron en cada etapa (el log no dice cuando empieza una etapa).
 */
#include <kos.h>
#include <malloc.h>
#include <stdint.h>
#include <stdio.h>

void d2c_serial_init(void) {
    dbgio_dev_select("scif");
}

unsigned int d2c_heap_libre(void) {
    struct mallinfo mi = mallinfo();

    return (unsigned int)mi.fordblks;
}

/* DC-002.85: cuanta memoria del monton esta EN USO y de que tamano es el arena.
 *
 * POR QUE HACEN FALTA LOS DOS. `fordblks` (libre) no es comparable entre builds: malloc solo agranda el
 * arena cuando le hace falta (sbrk a pedazos), asi que un arbol que pide mas memoria puede mostrar MAS
 * libre que uno que pide menos, solo porque agrando el arena antes. Lo que si se compara es lo USADO
 * (`uordblks`, que es la suma de los bloques vivos) contra el arena total.
 */
unsigned int d2c_heap_usado(void) {
    struct mallinfo mi = mallinfo();

    return (unsigned int)mi.uordblks;
}

unsigned int d2c_heap_arena(void) {
    struct mallinfo mi = mallinfo();

    return (unsigned int)(mi.uordblks + mi.fordblks);
}

uint64_t d2c_us(void) {
    return (uint64_t)timer_us_gettime64();
}

/* DC-002.87 (INSTRUMENTO): quitarle bytes al monton de C a proposito.
 *
 * POR QUE. En DC-002.84 el build que muere (E) llego al primer cuadro con el arena de malloc casi sin
 * sobrante (392 KB libres contra 4.016 KB del que pinta) y murio justo al pedir los buffers del cuadro. La
 * hipotesis viva es que el defecto aparece cuando el monton de C se queda sin lugar; para probarla hace falta
 * el MISMO codigo con el monton apretado a voluntad, y eso es esto: un bloque que se pide y no se devuelve.
 *
 * Se pide en pedazos chicos para que el `sbrk` crezca de verdad (un pedido unico grande puede terminar en un
 * mmap propio y no apretar el arena que usa el resto). Devuelve cuantos bytes se quedaron de verdad.
 */
static void *d2c_reservado[512];
static int d2c_reservados;

unsigned int d2c_reservar(unsigned int bytes) {
    unsigned int quedado = 0;
    unsigned int paso = 8192;

    while(quedado + paso <= bytes && d2c_reservados < 512) {
        void *p = malloc(paso);

        if(!p)
            break;

        d2c_reservado[d2c_reservados++] = p;
        quedado += paso;
    }

    return quedado;
}

/* DC-002.87 (INSTRUMENTO, prueba en contra): una excepcion DE VERDAD, pedida desde Go.
 *
 * POR QUE. El volcado de registros de la sonda solo sirve si dispara. La corrida que muere es un blanco que se
 * mueve con el acomodo del binario; esto en cambio es un tiro fijo: el SH-4 tiene que tomar una excepcion y el
 * manejador global tiene que imprimir `SONDA-REG FALLO` con PC, PR, SR, los 16 registros, 8 palabras de pila y
 * 8 del marco. Si eso no sale, el instrumento esta roto y cualquier "no hay volcado" de una corrida anterior
 * no significa nada.
 *
 * OJO, MEDIDO DOS VECES: (1) escribir en una direccion que no existe en area 2 (`0xBAAAAAAD`) NO da
 * excepcion en la Dreamcast -- el bus se lo come y el POC sigue como si nada; (2) una escritura de 32 bits a
 * una direccion impar TAMPOCO, porque gcc parte el `*p = x` volatil en CUATRO stores de byte (desensamblado:
 * 8c013028 son cuatro `mov.b`), y cuatro accesos de byte alineados no violan nada. Lo que levanta excepcion
 * siempre es una instruccion TRAPA: el SH-4 la convierte en `EXC_TRAPA` (0x0160), que cae por debajo de 0x200
 * y por lo tanto entra en el volcado.
 */
void d2c_falla_de_prueba(void) {
    __asm__ __volatile__("trapa #32" : : : "memory");
}

extern unsigned long marca_gc_n;

unsigned int d2c_gc_n(void) {
    return (unsigned int)marca_gc_n;
}

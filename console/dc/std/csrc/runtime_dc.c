/* Puentes al runtime de libgodc para d2std/runtime.
 *
 * Estas DOS funciones YA EXISTEN en el runtime de la consola (medido al enlazar): gc_runtime.c define
 * _runtime.GC y proc.c define _runtime.NumGoroutine. No se reimplementan: se llaman.
 *
 * Por que un puente y no un //extern directo en Go: los simbolos del runtime llevan un PUNTO en el
 * nombre (_runtime.GC), que no es un identificador valido para el comentario //extern ni para una
 * funcion de Go. En C se resuelve con un nombre de ensamblador (__asm__).
 */
#include <stdint.h>

extern void runtime_gc_dc(void) __asm__("_runtime.GC");
extern intptr_t runtime_numgoroutine_dc(void) __asm__("_runtime.NumGoroutine");

/* d2c_gc: pide la recoleccion al runtime de la consola (no devuelve nada). */
void d2c_gc(void) {
    runtime_gc_dc();
}

/* d2c_numgoroutine: cuantas goroutines vivas lleva el runtime. */
intptr_t d2c_numgoroutine(void) {
    return runtime_numgoroutine_dc();
}

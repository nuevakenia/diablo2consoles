/* DC-002.13: los hash del runtime que el compilador pide SIN el sufijo `..f`.
 *
 * POR QUE EXISTE ESTE ARCHIVO (medido, no supuesto)
 *
 * Cuando se enlaza el MOTOR ENTERO (los 142 paquetes + la stdlib minima + los terceros) el linker se cae
 * con 15 referencias en 5 simbolos:
 *
 *     runtime.memhash     6     runtime.memhash32   3     runtime.f64hash   3
 *     runtime.strhash     1     runtime.memhash8    1
 *
 * Los cinco son funciones de hash que gccgo genera el mismo para los mapas con claves que no son de los
 * tipos rapidos (structs, floats, claves armadas con varios campos). libgodc SI las tiene, pero con el
 * sufijo `..f` (`_runtime.memhash32..f`, `_runtime.strhash..f`, ...), que es el nombre que usa cuando la
 * funcion se LLAMA como funcion de Go. El nombre sin sufijo es el que el compilador pone en el DESCRIPTOR
 * DE TIPO de la clave, y libgodc no lo define para los hash (si para otras cosas: `_runtime.memequal`,
 * `_runtime_typedmemmove`).
 *
 * Como se midio (no se adivino): con `sh-elf-objdump -dr` sobre el .gox que no enlazaba se ve el envoltorio
 * que arma el compilador para la clave, y ahi esta la firma exacta de cada uno:
 *
 *   d2monsters.gox, clave struct{ string; uint32; int }:
 *       mov.l fc48,r0   ! _runtime.strhash       ; jsr @r0   (r4 = la clave, r5 = h)
 *       mov r0,r5 ; mov r8,r4 ; add #8,r4        ; jsr @r9   (r9 = _runtime.memhash32)
 *       mov r0,r5 ; mov r8,r4 ; add #12,r4       ; jsr @r9
 *     -> los dos son de DOS argumentos: (puntero al campo, h) -> uintptr_t
 *
 *   d2gamescreen.gox, clave struct{ int; int; int }:
 *       mov.l 2b388,r0  ! _runtime.memhash       ; jsr @r0
 *       mov #12,r6                               <-- TERCER argumento = el TAMANO de la clave
 *     -> este es de TRES: (clave, h, tamano) -> uintptr_t
 *
 * Y para que los numeros no se contradigan con los del propio libgodc, los de DOS argumentos delegan en la
 * implementacion que libgodc ya tiene (los `..f`) y el de tres delega en `map_memhash`, que es EL MISMO
 * algoritmo que libgodc usa en sus caminos rapidos de mapa (map_dreamcast.c lo dice en su cabecera: "These
 * MUST use the same algorithm as runtime_memhash32..f etc"). O sea: un mapa que va por el descriptor de
 * tipo y el mismo mapa por el camino rapido de libgodc calculan lo mismo, que es lo que hace que la clave
 * se encuentre.
 *
 * Se resuelve en el C DE LA STDLIB MINIMA (console/dc/std/csrc) y no en el runtime vendorizado
 * (~/dreamcast/libgodc) porque los scripts de arreglo del vendor (fix-libgodc-*.py) empiezan dejando el
 * arbol pristino: un cambio a mano ahi se pierde en la proxima corrida. Aca viaja con el repo y lo compilan
 * los dos scripts de build de la consola.
 */
#include <stdint.h>

/* las implementaciones de libgodc (los `..f`), tal como estan en runtime_stubs.c */
extern uintptr_t runtime_strhash_f(void *key, uintptr_t seed) __asm__("_runtime.strhash..f");
extern uintptr_t runtime_memhash8_f(void *key, uintptr_t seed) __asm__("_runtime.memhash8..f");
extern uintptr_t runtime_memhash32_f(void *key, uintptr_t seed) __asm__("_runtime.memhash32..f");
extern uintptr_t runtime_f64hash_f(void *key, uintptr_t seed) __asm__("_runtime.f64hash..f");

/* el hash de tamano arbitrario de libgodc (map_dreamcast.c): el MISMO que usan sus mapas */
extern uintptr_t map_memhash(void *data, uintptr_t seed, uintptr_t size);

/* --- los que ya existen con sufijo ..f: el nombre pelado llama al mismo codigo --- */

uintptr_t d2c_strhash(void *key, uintptr_t seed) __asm__("_runtime.strhash");
uintptr_t d2c_strhash(void *key, uintptr_t seed)
{
    return runtime_strhash_f(key, seed);
}

uintptr_t d2c_memhash8(void *key, uintptr_t seed) __asm__("_runtime.memhash8");
uintptr_t d2c_memhash8(void *key, uintptr_t seed)
{
    return runtime_memhash8_f(key, seed);
}

uintptr_t d2c_memhash32(void *key, uintptr_t seed) __asm__("_runtime.memhash32");
uintptr_t d2c_memhash32(void *key, uintptr_t seed)
{
    return runtime_memhash32_f(key, seed);
}

uintptr_t d2c_f64hash(void *key, uintptr_t seed) __asm__("_runtime.f64hash");
uintptr_t d2c_f64hash(void *key, uintptr_t seed)
{
    return runtime_f64hash_f(key, seed);
}

/* --- el de tamano arbitrario: (clave, h, tamano) --- */

uintptr_t d2c_memhash(void *key, uintptr_t h, uintptr_t size) __asm__("_runtime.memhash");
uintptr_t d2c_memhash(void *key, uintptr_t h, uintptr_t size)
{
    return map_memhash(key, h, size);
}

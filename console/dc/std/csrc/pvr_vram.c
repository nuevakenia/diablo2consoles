/* DC-002.108: LA VRAM DEL PVR COMO LUGAR DONDE VIVEN LOS BUFFERS GRANDES -- la cara de C.
 *
 * QUE HACE. Cuatro puentes sobre el repartidor de VRAM de KOS (`pvr_mem_malloc` / `pvr_mem_free` /
 * `pvr_mem_available`, kernel/arch/dreamcast/hardware/pvr/pvr_mem.c):
 *
 *   d2c_pvr_vram_disponible   cuantos bytes puede repartir el PVR AHORA (lo que queda despues de las
 *                             texturas vivas: la de la pantalla y el lienzo del cuadro)
 *   d2c_pvr_vram_pedir        pide un bloque de `bytes` del pool de texturas (0 = no entro)
 *   d2c_pvr_vram_soltar       devuelve un bloque pedido con la de arriba
 *   d2c_pvr_vram_servible     el mayor bloque de hasta `tope` que el pool sirve AHORA, sin quedarse con el
 *
 * POR QUE EN C Y NO EN GO. El binding de Go de libgodc no expone el repartidor de VRAM (lo mismo que pasa con
 * `pvr_txr_load_ex` en pvr_sprite.c). Es el patron de siempre en este repo: la funcion es de KOS, el puente es
 * C y gccgo la llama con el comentario //extern, con los punteros cruzando como `unsigned int`.
 *
 * POR QUE HACE FALTA (medido en DC-002.104/.105/.106). La cache de tiles en indices son 2.723.840 B y no
 * entran en el monton de C: no es que falten bytes -- DC-002.105 dejo 2.849.816 B disponibles -- es que
 * dlmalloc pide el tramo de un solo golpe por `sbrk` y el monton no tiene 2,7 MB contiguos. DC-002.106 lo
 * salvo pidiendolo en 9 bloques (con una sonda igual a `d2c_pvr_vram_servible`, pero sobre `malloc`), y aun
 * asi el monton de C queda en el hueso: 12.288 B de break room despues del primer cuadro.
 *
 * La VRAM, en cambio, es el lugar donde esa cache vive en una Dreamcast de verdad: son 8 MB (VRAM de
 * texturas) de los que el motor usa ~2,3 MB (la textura del cuadro 1024x512x2 = 1.048.576 B y el lienzo
 * RGBA 640x480x4 = 1.228.800 B). Lo que sobra es un pool que el PVR reparte con SU propio dlmalloc y que la
 * CPU escribe y lee igual que la RAM (los punteros de `pvr_mem_malloc` son punteros del SH-4, ver
 * pvr_mem.h: "PVR pointers in the new system are actually SH-4 compatible pointers").
 *
 * LO QUE CUESTA, DICHO DE FRENTE. La VRAM se toca sin cache (area P2, cache-through): leer o escribir un byte
 * ahi es mas lento que en RAM. El lienzo del cuadro ya vive ahi desde DC-002.22 y el costo esta declarado en
 * pvr/pantalla.go; esta tarjeta lo MIDE otra vez para la cache (los ms por cuadro del log).
 *
 * NO SE USA COMO TEXTURA DEL PVR. La cache se guarda en el pool de texturas, pero el dibujado la lee la CPU
 * igual que antes: el cuadro tiene que salir idéntico al byte al del host (crc32 1437022778), y dibujar los
 * tiles con quads por hardware cambiaria el mezclado y el orden -- otra tarjeta. Lo que esta tarjeta mueve es
 * DONDE VIVE el buffer, que es lo que el monton de C no aguanta.
 */
#include <kos.h>
#include <stdint.h>
#include <stdio.h>
#include <string.h>

/* Cuantos bytes puede repartir el pool de texturas del PVR en este momento.
 *
 * Es `pvr_mem_available()`: arena - uordblks + lo que queda del top (pvr_mem.c:176), o sea la MISMA cuenta
 * que hace el propio KOS. Devuelve 0 si el PVR todavia no arranco (`pvr_mem_base` en NULL): el pool no
 * existe antes de `pvr_init_defaults`, y un 0 ahi es la verdad, no un error. */
int d2c_pvr_vram_disponible(void) {
    return (int)pvr_mem_available();
}

/* Lo que mide la VRAM del PVR en total (8 MB en una Dreamcast), para poder decir "de cuanto" en la evidencia.
 * Es el tamano del pool, no lo libre: lo libre lo dice la de arriba. */
int d2c_pvr_vram_total(void) {
    return (int)PVR_RAM_SIZE;
}

/* Pide `bytes` del pool de texturas del PVR. Devuelve el puntero como `unsigned int` (el registro del puente
 * con Go) o 0 si no entro -- que es lo que hay que mirar: un pedido que no entra NO revienta el runtime (al
 * reves que `make` sobre el monton de C, que tira `gc_external_alloc: OOM`), asi que aca el que llama puede
 * decidir (derramar al camino de siempre, como en DC-002.106). */
unsigned int d2c_pvr_vram_pedir(unsigned int bytes) {
    void *p;

    if (bytes == 0) {
        return 0;
    }

    p = pvr_mem_malloc((size_t)bytes);

    if (p == NULL) {
        printf("PVR-VRAM sin VRAM para %u bytes (quedan %d)\n", bytes, (int)pvr_mem_available());

        return 0;
    }

    return (unsigned int)(uintptr_t)p;
}

/* Devuelve al pool un bloque pedido con `d2c_pvr_vram_pedir`. */
void d2c_pvr_vram_soltar(unsigned int bloque) {
    if (bloque == 0) {
        return;
    }

    pvr_mem_free((pvr_ptr_t)(uintptr_t)bloque);
}

/* Pide `bytes` del pool de texturas y los deja EN CERO, que es lo que devuelve un `make([]byte, n)` de Go.
 *
 * POR QUE ESTO NO ES UN LUJO (medido en esta tarjeta, y costo un cuadro distinto). El decodificador del .dt1
 * (`d2dt1.DecodeTileGfxData`) escribe SOLO los pixeles que algun bloque del tile pinta; el resto del buffer
 * queda como estaba. En Go eso es cero, porque `make` devuelve memoria en cero. La VRAM, en cambio, viene con
 * lo que haya dejado la corrida anterior o el propio PVR: medido en Flycast, la cache puesta en VRAM daba
 * 86 imagenes / 2.723.840 B (los mismos) y el cuadro salia con crc32 883948965 en vez del del host
 * (1437022778), con la huella de la cache distinta. Poniendo el bloque en cero como lo pondria `make`, la cache
 * es la misma byte a byte y el cuadro tambien.
 *
 * Se hace en C y no en Go porque `memset` del compilador escribe de a 4 bytes; un bucle de Go sobre la VRAM
 * (que se toca sin cache) cuesta mas para lo mismo.
 */
unsigned int d2c_pvr_vram_pedir_cero(unsigned int bytes) {
    void *p;

    if (bytes == 0) {
        return 0;
    }

    p = pvr_mem_malloc((size_t)bytes);

    if (p == NULL) {
        printf("PVR-VRAM sin VRAM para %u bytes (quedan %d)\n", bytes, (int)pvr_mem_available());

        return 0;
    }

    memset(p, 0, (size_t)bytes);

    return (unsigned int)(uintptr_t)p;
}

/* El mayor bloque de hasta `tope` que el pool puede servir AHORA. Es la misma idea que `d2c_servible`
 * (console/dc/poc/mapa-103/csrc/shim.c:5) pero sobre la VRAM: el pool de KOS es un dlmalloc recortado
 * (pvr_mem_core.c) y tambien puede dejar de servir un tamano cuando quedan migas.
 *
 * Cada sonda se devuelve en el acto, asi que no cuesta VRAM: lo que vino de un hueco se vuelve a fundir con
 * el y lo que vino del top se queda como top (de donde saldria el pedido de verdad). */
unsigned int d2c_pvr_vram_servible(unsigned int tope) {
    unsigned int t;

    if (tope > (1u << 30)) {
        return 0;
    }

    for (t = tope; t > 0; t = (t / 2) & ~31u) {
        void *p = pvr_mem_malloc((size_t)t);

        if (p != NULL) {
            pvr_mem_free(p);

            return t;
        }
    }

    return 0;
}

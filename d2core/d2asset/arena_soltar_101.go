package d2asset

// DC-002.101 -- SOLTAR EL ARENA DE BLOQUES ANTES DE DIBUJAR.
//
// POR QUE EXISTE. El compositor de un nivel deja pedidos, en el monton externo (monton de C: todo lo que pasa
// de 64 KB va a `gc_external_alloc`, `gc_heap.c:116`), los tramos del arena de bloques y el buffer prestado de
// lectura, que en el pueblo son 2.621.440 B + 1.008.003 B (medido en DC-002.88). Ese monton NO LO TOCA el
// recolector: nada de eso se devuelve solo, ni cuando los bloques ya no se necesitan.
//
// CUANDO YA NO SE NECESITAN. Los bloques se usan para DECODIFICAR la imagen de cada tile (`DecodeTileGfxData`)
// y para nada mas. Con la cache de tiles armada (las imagenes ya decodificadas), el cuadro se pinta sin volver
// a mirarlos. Entonces: se compone, se arma la cache del cuadro, y RECIEN ahi se pueden soltar. Soltarlos
// antes de armar la cache es leer memoria ajena (los `EncodedData` de cada tile apuntan adentro).
//
// EL CONTRATO, dicho como es:
//   - solo se llama con la cache de tiles YA armada y sin nivel nuevo por delante;
//   - despues de soltar, el nivel NO puede cargar un `.dt1` nuevo ni regenerar la cache: `PrepararArenaDeBloquesUsados`
//     vuelve a pedir los tramos (y los bloques de lo que ya estaba copiado se perdieron, asi que el nivel se
//     tiene que volver a componer entero);
//   - los tramos se devuelven con `free` pelado: es el mismo `free` que `runtime.FreeExternal`, que es lo que
//     corresponde porque el runtime mando esos `make` al monton externo por tamano, no al semiespacio.
//
// POR QUE EL LIBERADOR SE INYECTA. `free` no esta en la libreria estandar de Go: en la consola es el
// `d2cFreeExternal` del shim de C (que llama a `_runtime.FreeExternal`). Un paquete del motor no puede
// llamarlo solo, asi que quien arranca la consola dice con que se libera (`UsarLiberadorDeExternos`). Sin
// liberador instalado, esta funcion no toca nada y devuelve ceros: en la PC el camino no existe.

import "unsafe"

// liberadorDeExternos: lo instala el arranque de la consola (ver UsarLiberadorDeExternos).
var liberadorDeExternos func(uintptr)

// UsarLiberadorDeExternos: quien arranca la consola dice con que se devuelve al monton un tramo externo. En la
// consola se pasa `d2cFreeExternal` (el shim de C que llama a `_runtime.FreeExternal`).
func UsarLiberadorDeExternos(l func(uintptr)) { liberadorDeExternos = l }

// HayLiberadorDeExternos dice si el arranque instalo el liberador (lo consulta el instrumento).
func HayLiberadorDeExternos() bool { return liberadorDeExternos != nil }

// SoltarArenaDeBloques devuelve al monton de C los tramos del arena de bloques y el buffer prestado de
// lectura, y devuelve cuantos tramos solto y cuantos bytes. Sin liberador instalado devuelve (0, 0) y no toca
// el estado. Ver el contrato en la cabecera del archivo.
func SoltarArenaDeBloques() (tramos, bytes int) {
	candadoDeTiles.Lock()
	defer candadoDeTiles.Unlock()

	if liberadorDeExternos == nil {
		return 0, 0
	}

	for i := range arenaDeBloques {
		if len(arenaDeBloques[i]) == 0 {
			continue
		}

		liberadorDeExternos(uintptr(unsafe.Pointer(&arenaDeBloques[i][0])))
		bytes += cap(arenaDeBloques[i])
		tramos++
	}

	arenaDeBloques = nil
	tramoActual = 0
	cursorDelBloque = 0
	bytesDeBloques = 0

	if len(scratchDeLectura) > 0 {
		liberadorDeExternos(uintptr(unsafe.Pointer(&scratchDeLectura[0])))
		bytes += cap(scratchDeLectura)
	}

	scratchDeLectura = nil
	scratchDelNivel = 0

	// el camino de bloques queda APAGADO: sin tramos, `reservarEnElArenaDeBloques` volveria a pedirlos y los
	// `EncodedData` de los tiles ya copiados apuntan a memoria devuelta. Que lo prenda el proximo nivel.
	bloquesUsados = false
	clavesDelNivel = nil

	return tramos, bytes
}

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

// DC-002.111 -- LOS TRAMOS DEL ARENA SE PUEDEN DEVOLVER AL TIRAR EL NIVEL.
//
// POR QUE. En DC-002.88 los tramos del arena de bloques se dejaban tomados entre niveles ("se vuelven a usar
// desde el primero, porque pedirlos de nuevo mataba el segundo nivel"). Eso hace del tramo el bloque mas
// grande que el monton de C NUNCA recupera: 3.145.728 B en la VUELTA, tomados mientras se carga el nivel
// nuevo. Medido en esta tarjeta: con eso el monton llega al pedido de 1.008.003 B (`act1/town/floor.dt1`) con
// `used=9.909.584 free=1.170.352` -- mas libre que el pedido, pero en trozos, y el `sbrk` ya en el techo: el
// runtime muere en `gc_external_alloc` en vez de devolver NULL.
//
// LO QUE HACE. `ReiniciarArenaDeTiles` -- el momento en que el motor TIRA UN NIVEL, o sea cuando ya vacio el
// cache de `.dt1` y nadie tiene un puntero adentro del arena -- devuelve los tramos al monton. El nivel nuevo
// los vuelve a pedir, ahora con la sonda (`sondaDelMontonDeC`) que decide el tamano. Es la misma operacion que
// `SoltarArenaDeBloques` hace despues de armar la cache, solo que en el otro extremo del nivel.
var soltarTramosAlReiniciar bool

// UsarSoltarTramosAlReiniciar prende/apaga devolver los tramos al tirar el nivel (ver arriba).
func UsarSoltarTramosAlReiniciar(v bool) { soltarTramosAlReiniciar = v }

// SoltarTramosAlReiniciarActivo dice cual de los dos caminos esta prendido (lo consulta el instrumento).
func SoltarTramosAlReiniciarActivo() bool { return soltarTramosAlReiniciar }

// soltarLosTramos devuelve los tramos del arena de bloques al monton de C y deja el arena vacio. Se llama con
// `candadoDeTiles` TOMADO. No toca `bloquesUsados` ni `clavesDelNivel`: eso lo decide el que llama (al tirar
// un nivel el camino SIGUE prendido, porque el nivel nuevo tambien lo usa).
func soltarLosTramos() (tramos, bytes int) {
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

	return tramos, bytes
}

// SoltarArenaDeBloques devuelve al monton de C los tramos del arena de bloques y el buffer prestado de
// lectura, y devuelve cuantos tramos solto y cuantos bytes. Sin liberador instalado devuelve (0, 0) y no toca
// el estado. Ver el contrato en la cabecera del archivo.
func SoltarArenaDeBloques() (tramos, bytes int) {
	candadoDeTiles.Lock()
	defer candadoDeTiles.Unlock()

	if liberadorDeExternos == nil {
		return 0, 0
	}

	tramos, bytes = soltarLosTramos()

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

// DC-002.111 -- EL SCRATCH SE SUELTA POR ARCHIVO (y no al final del nivel).
//
// POR QUE. Medido en DC-002.110: la TRANSICION con el `.ds1` en el monton de C muere con `gc_external_alloc:
// OOM` pidiendo un tramo del arena de bloques de 524.288 B con el monton de C en 10.285.248 usados y 379.104
// libres. El pico del monton de C lo arman tres cosas que hoy viven a la vez durante TODA la carga del nivel:
// el scratch de lectura (1.008.003 B, el `.dt1` mas grande del pueblo = `act1/town/floor.dt1`), los tramos del
// arena de bloques (3.029.194 B) y las fuentes + tablas reabiertas (679.440 B).
//
// El scratch lo usa `cargarDT1EnBloquesUsados` para leer UN archivo: en cuanto los bloques de ese `.dt1`
// estan COPIADOS al arena, el crudo no lo mira nadie. Si se suelta ahi, el pico deja de incluir el archivo
// mas grande durante el resto de la carga (y el hueco que deja es lo que despues sirve para el tramo
// siguiente, que es justo lo que no habia). Con la palanca apagada queda el camino de DC-002.101: un buffer
// del tamano del archivo mas grande, pedido ANTES del primer `.dt1` y soltado despues de armar la cache.
var scratchPorArchivo bool

// UsarScratchPorArchivo prende/apaga el buffer de lectura por archivo (ver arriba). Lo prende el arranque de
// la consola; en la PC queda apagado (el recolector se lleva el buffer solo y no hay nada que medir).
func UsarScratchPorArchivo(v bool) { scratchPorArchivo = v }

// ScratchPorArchivoActivo dice cual de los dos caminos esta prendido (lo consulta el instrumento).
func ScratchPorArchivoActivo() bool { return scratchPorArchivo }

// Los dos numeros del instrumento: cuantas veces se devolvio el buffer y cuantos bytes.
var (
	scratchSoltados      int
	scratchBytesSoltados int
)

// DC-002.111: los tramos devueltos al TIRAR un nivel (ver `soltarTramosAlReiniciar`).
var (
	tramosDevueltosAlReiniciar int
	tramosBytesDevueltos       int
)

// soltarScratchDeLectura devuelve el buffer prestado al monton de C con el liberador inyectado y lo deja en
// nil (el proximo `.dt1` lo vuelve a pedir, del tamano que necesita). Sin liberador no toca nada: en la PC el
// buffer lo recoge el recolector y `free` no esta en la libreria estandar de Go.
//
// Se llama con `candadoDeTiles` TOMADO (`cargarDT1EnBloquesUsados` corre con el candado).
func soltarScratchDeLectura() int {
	if liberadorDeExternos == nil || len(scratchDeLectura) == 0 {
		return 0
	}

	liberadorDeExternos(uintptr(unsafe.Pointer(&scratchDeLectura[0])))

	bytes := cap(scratchDeLectura)
	scratchDeLectura = nil
	scratchDelNivel = 0

	scratchSoltados++
	scratchBytesSoltados += bytes

	return bytes
}

// DC-002.111 -- EL TRAMO DEL ARENA DE BLOQUES LO DIMENSIONA EL MONTON (y el pico se mide adentro de la carga).
//
// POR QUE. Con el scratch ya por archivo (arriba) la corrida dejo de morir por bytes y murio por CONTIGUIDAD,
// que es la pared de DC-002.106 otra vez pero en el arena de bloques: en el pedido del septimo tramo el monton
// tenia `arena=11.056.640 used=9.658.128 free=1.398.512` -- 1,4 MB libres y ni un hueco de 524.288, y dlmalloc
// lo resolvio pidiendo `sbrk` (que ya estaba en el techo) y `gc_external_alloc` tiro el runtime en vez de
// devolver NULL. La cura es la misma que la de la cache en DC-002.106: preguntarle al monton cuanto puede
// servir (la sonda `d2c_servible`, que hace `malloc`/`free` en C y baja de a mitades) y pedir ESE tamano.
//
// EL MEDIDOR. El pico del monton de C durante la CARGA no lo puede muestrear el POC: pasa adentro del bucle de
// `.dt1` de `ResetMapParaStamp`. Por eso el arranque de la consola inyecta tambien `d2c_monton_usado`, y aca se
// guarda el maximo en cada pedido (el buffer de un archivo y cada tramo). Es el numero que la tarjeta pide
// ("pico del monton de C por fase") medido donde ocurre, no en el punto mas cercano que el POC alcanza a ver.
var (
	sondaDelMontonDeC   func(tope int) int
	medidorDelMontonDeC func() int
	picoDelMontonDeC    int
)

// UsarSondaDelMontonDeC: con que se le pregunta al monton de C cuanto puede servir (en la consola,
// `d2c_servible`). Sin sonda, el tramo se pide del tamano fijo de siempre (el camino de DC-002.101).
func UsarSondaDelMontonDeC(f func(tope int) int) { sondaDelMontonDeC = f }

// HaySondaDelMontonDeC dice si el arranque instalo la sonda (lo consulta el instrumento).
func HaySondaDelMontonDeC() bool { return sondaDelMontonDeC != nil }

// UsarMedidorDelMontonDeC: con que se lee lo USADO del monton de C (en la consola, `d2c_monton_usado`).
// Sin medidor no hay pico que informar (0).
func UsarMedidorDelMontonDeC(f func() int) { medidorDelMontonDeC = f }

// PicoDelMontonDeC: lo mas usado que estuvo el monton de C desde el ultimo `PrepararArenaDeBloquesUsados`
// (o sea, durante la carga de los `.dt1` del nivel). Se llama con el candado tomado.
func PicoDelMontonDeC() int {
	candadoDeTiles.Lock()
	defer candadoDeTiles.Unlock()

	return picoDelMontonDeC
}

// muestrearElMonton: se queda con el maximo de lo usado. Se llama en cada pedido grande de la carga.
func muestrearElMonton() {
	if medidorDelMontonDeC == nil {
		return
	}

	if usado := medidorDelMontonDeC(); usado > picoDelMontonDeC {
		picoDelMontonDeC = usado
	}
}

// SoltarPrestamoDeLecturaDeTiles devuelve al monton SOLO el buffer prestado de la lectura del `.dt1` crudo
// (el `scratchDeLectura`), y devuelve cuantos bytes solto (0 si no habia ninguno).
//
// POR QUE ES OTRA COSA QUE `SoltarArenaDeBloques` (DC-002.103). El contrato de arriba tiene dos mitades con
// vidas distintas y las dos estan medidas en el pueblo:
//
//	el buffer prestado   1.008.003 B  lo usa `cargarDT1EnBloquesUsados` para leer UN archivo y parsearlo.
//	                                  Muere cuando el ULTIMO `.dt1` del nivel termino de cargarse: nadie lo
//	                                  vuelve a mirar. Si un nivel nuevo lo necesita, se vuelve a pedir solo
//	                                  (`cargarDT1EnBloquesUsados` lo rehace cuando el suyo es chico).
//	los tramos de bloque  2.621.440 B los `EncodedData` de cada tile APUNTAN ahi: viven hasta que la cache de
//	                                  imagenes esta armada (por eso `SoltarArenaDeBloques` exige cache armada).
//
// Esta funcion suelta SOLO la primera mitad, asi que se puede llamar en cuanto el nivel esta compuesto y NO
// invalida nada: es la unica parte del arena que se puede devolver sin esperar a la cache. No prende ni apaga
// `bloquesUsados`: los tramos siguen siendo validos.
func SoltarPrestamoDeLecturaDeTiles() int {
	candadoDeTiles.Lock()
	defer candadoDeTiles.Unlock()

	// DC-002.111: el cuerpo es el mismo que usa la palanca por archivo, asi los dos caminos devuelven el
	// buffer con el MISMO liberador y el instrumento cuenta igual.
	return soltarScratchDeLectura()
}

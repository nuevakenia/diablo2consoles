//go:build dreamcast

package d2mpq

// DC-002.80 -- EL DESCOMPRESOR DE ZLIB CON DESTINO PRESTADO: SOLO CONSOLA.
//
// POR QUE UN ARCHIVO CON TAG. La lectura con prestamo descomprime un bloque por sector (124 en un .dt1 de
// 505 KB) y para no pedir memoria hace falta un descompresor que ESCRIBA EN UN DESTINO PRESTADO y se reuse
// entre bloques. En la PC eso lo da `compress/zlib` (su lector sabe `Reset`). En la CONSOLA `compress/zlib`
// es OTRO paquete -- un puente a la zlib de KOS (`console/dc/std/compress/zlib`) -- y su `NewReader` COPIA el
// comprimido con `io.ReadAll`: usado por sector pide memoria por sector, que es justo lo que esta tarjeta
// saca. Ese puente ahora tiene un `Inflador` (un slot vivo + `inflateReset`, sin copiar nada).
//
// El codigo del MPQ es UNO solo para las dos plataformas (regla del port); la diferencia se resuelve con el
// enganche `ConectarInfladorDePrestamo`, y este archivo lo conecta al arrancar. El tag `dreamcast` es para que
// el HOST no compile este archivo, que es el unico que conoce la diferencia.
//
// POR QUE init() Y NO UNA LLAMADA DEL MAIN. Es asignar un puntero a funcion y nada mas (no pide memoria, no
// depende de otro paquete ni de un orden): asi CUALQUIER binario de consola que use d2mpq lee con prestamo y
// sin copias, sin que el que arranca se tenga que acordar. El main del motor (Fase D) no tiene que llamar
// nada.

import "compress/zlib"

func nuevoInfladorDeConsola() InfladorDePrestamo {
	return zlib.NuevoInflador()
}

func init() {
	ConectarInfladorDePrestamo(nuevoInfladorDeConsola)
}

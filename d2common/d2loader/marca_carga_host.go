//go:build !dreamcast

package d2loader

// DC-002.43 -- el lado HOST de las marcas de carga: NO HACE NADA (ver la cabecera de `marca_carga.go`).
//
// El host corre las pruebas de la suite y no tiene consola serie: sin instrumento, `Marca` es una llamada
// vacia y el comportamiento medido en el host queda igual que antes de esta tarjeta.
//
// OJO: el build de la CONSOLA saca este archivo POR NOMBRE (gccgo no respeta los build tags: medido en
// DC-002.42). Los dos scripts que lo hacen son `build-motor-all.sh` y `recompilar-paquete-motor.sh`.
func Marca(ruta string, punto int) {
	_, _ = ruta, punto
}

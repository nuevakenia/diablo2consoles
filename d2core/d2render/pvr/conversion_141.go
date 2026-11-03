//go:build dreamcast

package pvr

// DC-002.141 -- LA CONVERSION DEL LIENZO POR FILAS (la otra mitad del cuadro caminando).
//
// El cuadro caminando baja el mapa de 60,4 ms a ~15 ms (el lienzo se corre y se repinta solo lo que entra, ver
// `recortes_141.go` y la cabecera de `mapa_141.go` del POC), pero al mover el lienzo cambia TODO el cuadro, y
// la presentacion vuelve a convertir los 307.200 pixeles: 23,6 ms medidos en DC-002.140 que pasan a ser el
// cuello. El camino de siempre lee el lienzo por la tabla del twiddle, o sea lecturas SALTEADAS contra la VRAM.
//
// `bloque_por_filas141` (en `pvr_pantalla.c`) hace lo mismo al reves: recorre el lienzo fila por fila -- una
// lectura de 32 bits por pixel, la fila entera seguida -- y escribe cada resultado en su posicion twiddleada
// del buffer de 2 KiB, que esta en RAM. Es la misma cuenta de color y la misma tabla: el resultado es BYTE A
// BYTE el mismo, y hay un verificador en el binario que lo comprueba (convierte cada bloque por los dos
// caminos y cuenta las palabras distintas) para no tener que creerselo.

//extern d2c_pvr_pantalla_conv_filas141
func d2cPvrConvFilas141(v int)

//extern d2c_pvr_pantalla_conv_verificar141
func d2cPvrConvVerificar141(v int)

//extern d2c_pvr_pantalla_conv_fallos141
func d2cPvrConvFallos141() int

//extern d2c_pvr_pantalla_conv_bloques141
func d2cPvrConvBloques141() int

//extern d2c_pvr_pantalla_conv_reiniciar141
func d2cPvrConvReiniciar141()

// FijarLaConversionPorFilas141: prende el camino de la conversion por filas (y lo apaga para volver al de
// DC-002.135, en el mismo binario: es el A/B).
func FijarLaConversionPorFilas141(si bool) {
	if si {
		d2cPvrConvFilas141(1)

		return
	}

	d2cPvrConvFilas141(0)
}

// VerificarLaConversion141: prende el verificador. Cada bloque se convierte por los dos caminos y se cuentan
// las palabras de 16 bits que no coinciden. Cuesta el doble de conversion: solo para la corrida de la prueba.
func VerificarLaConversion141(si bool) {
	if si {
		d2cPvrConvVerificar141(1)

		return
	}

	d2cPvrConvVerificar141(0)
}

// CensoDeLaConversion141: los bloques verificados y las palabras distintas del tramo. Cero fallos es el unico
// resultado aceptable: dice que el camino por filas da exactamente la misma textura que el de siempre.
func CensoDeLaConversion141() (bloques, fallos int) {
	bloques, fallos = d2cPvrConvBloques141(), d2cPvrConvFallos141()

	d2cPvrConvReiniciar141()

	return bloques, fallos
}

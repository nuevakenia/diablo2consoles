//go:build dreamcast

package main

// DC-002.135 -- LA PRESENTACION: LOS TRES PEDAZOS Y LAS DOS PALANCAS.
//
// QUE HACE ESTE ARCHIVO, QUE ES POCO Y ES A PROPOSITO. El trabajo esta en el backend (`pvr_pantalla.c` y
// `d2core/d2render/pvr/renderer.go`): ahi vive el bucle de conversion, la subida a la VRAM y el cronometro
// de los tres pedazos, que se imprime solo en la linea `PVR135: PRESENTAR` cada 30 cuadros. Aca solo se
// eligen las dos palancas ANTES del primer cuadro y se deja constancia de tres cosas que sin ellas no se
// pueden leer despues:
//
//	presentarConvRapida / presentarVramAgrupada   con que camino corrio este binario (si no se dice, la
//	                                              linea del desglose no se puede comparar con otra);
//	el costo del RELOJ                            cuantas llamadas a d2cPvrUs cuesta el instrumento, para
//	                                              poder descontarlo del desglose (la presentacion se parte
//	                                              con dos marcas por bloque muestreado);
//	la carga por cuadro                          cuantos bloques de 32x32 y cuantas copias a la VRAM
//	                                              entran en un cuadro (300 y 300 sin palancas; 300 y 90
//	                                              con las dos: 70 grupos de 2x2 + 20 sueltos): es el
//	                                              control de que se midio lo que se cree que se midio.

import (
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2render/pvr"
)

// activarLaPresentacion135: elige el camino de la presentacion con las dos palancas de esta tarjeta.
//
// SE LLAMA ANTES DEL PRIMER CUADRO, igual que las palancas del blit (DC-002.132/.134): el backend de C
// tiene la bandera en una variable y el primer cuadro ya la lee.
func activarLaPresentacion135() {
	pvr.FijarPresentarConvRapida(presentarConvRapida == "si")
	pvr.FijarPresentarVramAgrupada(presentarVramAgrupada == "si")

	println("JUEGO135: arranca -- convRapida=", presentarConvRapida, "vramAgrupada=", presentarVramAgrupada)

	// el costo del instrumento, medido en la consola y no supuesto: 1.000 llamadas al reloj.
	//
	// POR QUE IMPORTA: el desglose de la presentacion se arma con dos marcas por bloque muestreado. Con un
	// cuadro de 300 bloques y una muestra de cada 8 son 75 marcas de ida y 75 de vuelta; si 1.000 llamadas
	// costaran 2.000 us, cada una cuesta 2 us y el desglose trae 150 x 2 = 300 us de instrumento. Ese numero
	// es el que hay que restar antes de decir "la conversion cuesta X".
	println("JUEGO135: RELOJ costo_de_1000_llamadas_us=", pvr.CostoDelRelojUs1000(),
		"muestreo=1de8", "relojDeLaConsola=d2cPvrUs")
}

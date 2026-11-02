//go:build dreamcast

package pvr

import "unsafe"

// DC-002.116 -- LA PANTALLA PARTIDA EN DOS: un lienzo a cada mitad, y el presentador propio del ciclo.
//
// QUE ES. Hasta DC-002.115 el backend tenia UNA forma de presentar: un lienzo RGBA de 640x480 en la VRAM del PVR
// (`SuperficieDeLaPantalla`) y `PresentarEnPantalla`, que lo convierte entero a la textura de la pantalla. El
// split vertical necesita lo contrario: DOS lienzos de 320x480, uno por jugador, cada uno a SU mitad de la
// pantalla. Este archivo agrega las dos piezas que faltaban, y ninguna de las dos cambia el camino de siempre:
//
//	PresentarEnColumna   el MISMO lienzo de siempre subido a una columna de la textura (0 o 320).
//	SuperficieEnBloque   una Surface cuyos pixeles viven en un bloque de la VRAM (los pidió `VramReservar`).
//
// POR QUE ASI, Y NO COMPONIENDO LOS DOS LIENZOS EN UNO DE 640x480. Componer cuesta una copia de 1.228.800 B por
// cuadro (pixel a pixel, contra VRAM sin cache) y ademas pide el lienzo de 640x480 OTRA VEZ: los dos de 320x480
// ya suman los mismos 1.228.800 B, y tener los tres serian 2.457.600 B de VRAM para nada. Presentando cada
// lienzo en su columna, la VRAM del split es EXACTAMENTE la misma que la del cuadro entero de DC-002.115, y el
// unico costo es el de siempre: convertir 307.200 pixeles a RGB565 (los mismos, repartidos en dos lienzos).
//
// LO QUE ESTE ARCHIVO NO HACE: no parte el dibujado del mapa ni recorta nada. El recorte sale solo, porque cada
// lienzo mide 320x480 y `RenderIndicesTintado`/`copiarEn` descartan lo que cae fuera de su lienzo. Eso es lo que
// hace que la costura de la mitad sea exacta: no hay pixel que se pueda escribir de la mitad equivocada.

//extern d2c_pvr_pantalla_convertir_en
func d2cPvrPantallaConvertirEn(rgba uintptr, w, h, xDestino int) int

// PresentarEnColumna: sube el lienzo `img` (w x h) a la textura de la pantalla a partir de la columna
// `xDestino`, SIN pintar. El que llama cierra con `PintarPantalla()` cuando ya subio todas las columnas, que es
// lo que hace un cuadro partido: primero la mitad izquierda, despues la derecha, y recien ahi el quad.
//
// `xDestino` tiene que ser multiplo de 32 (los bloques de la textura son de 32x32): 0 y 320 lo son.
//
// EL PUNTERO CRUZA COMO uintptr, igual que en `Presentar`: un slice pasado a una funcion //extern tiene el ABI
// ambiguo (puntero, largo y capacidad), y este compilador no tiene unsafe.Slice.
func PresentarEnColumna(img *Imagen, xDestino int) error {
	if img == nil || len(img.pixeles) == 0 {
		return errorPantalla("el lienzo esta vacio")
	}

	if r := d2cPvrPantallaConvertirEn(uintptr(unsafe.Pointer(&img.pixeles[0])), img.ancho, img.alto, xDestino); r != 0 {
		return errorPantalla("no se pudo convertir/subir el lienzo en la columna " + itoa(xDestino) +
			" (codigo " + itoa(r) + ")")
	}

	return nil
}

// SuperficieEnBloque: una Surface del tamano pedido cuyos pixeles son `bloque` (un trozo de memoria de al menos
// ancho*alto*4 B, tipicamente de la VRAM: `VramReservar`). Es la forma de tener un lienzo que NO compite con el
// monton de la consola y que ademas se puede presentar.
//
// NO SE COPIA NADA: el slice es el mismo. `EstaLleno` dice si el bloque alcanza, y si no alcanza devuelve nil en
// vez de recortar en silencio (un lienzo mas chico que su tamano declarado escribe afuera: es un error del que
// llama, no algo que se pueda adivinar).
func SuperficieEnBloque(bloque []byte, ancho, alto int) *Surface {
	if ancho <= 0 || alto <= 0 {
		return nil
	}

	n := ancho * alto * 4
	if len(bloque) < n {
		return nil
	}

	img := &Imagen{pixeles: bloque[:n:n], ancho: ancho, alto: alto, vram: true}

	return SurfaceDesdeImagen(img)
}

// EnVram: si los pixeles de este lienzo viven en la VRAM del PVR (para la evidencia del split: los dos lienzos
// de 320x480 viven ahi, y el numero que lo dice no es una suposicion).
func (i *Imagen) EnVram() bool { return i.vram }

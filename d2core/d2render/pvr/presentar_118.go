//go:build dreamcast

package pvr

import "unsafe"

// DC-002.118 -- EL CUADRO PARTIDO EN UNA REJILLA: un lienzo a CUALQUIER recuadro de la pantalla.
//
// QUE FALTABA. DC-002.116 dejo la pantalla partida en DOS columnas de 320x480 y le alcanzo con
// `PresentarEnColumna`, porque los dos lienzos empiezan en la fila 0. Para tres y cuatro jugadores no alcanza:
//
//   - cuatro cuadrantes son DOS FILAS (0 y 240), asi que hace falta la fila del destino en la conversion;
//   - y la fila de abajo NO puede vivir en la textura en la fila 240: los bloques de la textura son de 32x32 y
//     un lienzo que empiece a mitad de bloque no se puede subir por bloques enteros. Va en la 256.
//
// De ahi las dos piezas de este archivo, y las dos son ADITIVAS (el camino de DC-002.116 y el de todos los POC
// anteriores no cambia una linea: `PresentarEnColumna`/`PintarPantalla` siguen siendo los mismos):
//
//	PresentarEnBloque   un lienzo de w x h a la COLUMNA Y FILA (x,y) de la textura, en bloques de 32x32.
//	PintarEnBloque      UN recuadro de la textura a UN recuadro de la pantalla (un quad, nearest).
//
// POR QUE LA TEXTURA Y LA PANTALLA SE SEPARAN. Si la fila de abajo tiene que estar en la 256 de la textura y en
// la 240 de la pantalla, entonces "subir donde va" ya no es cierto: el que presenta dice las dos posiciones, y
// por eso `PintarEnBloque` existe en vez de reusar `PintarPantalla` (que pinta la region (0,0)-(640,480) de la
// textura en toda la pantalla, que es exactamente lo que hace falta con DOS columnas).

//extern d2c_pvr_pantalla_convertir_en_bloque
func d2cPvrPantallaConvertirEnBloque(rgba uintptr, w, h, xDestino, yDestino int) int

//extern d2c_pvr_pantalla_pintar_bloque
func d2cPvrPantallaPintarBloque(x, y, w, h, tu, tv int)

// PresentarEnBloque: sube el lienzo `img` (w x h) a la region de la textura que empieza en (xDestino, yDestino),
// SIN pintar. Igual que `PresentarEnColumna`, el que llama cierra con `PintarEnBloque` por cada recuadro.
//
// (xDestino, yDestino) tienen que ser multiplos de 32: el bloque de 32x32 de la textura es contiguo y el lienzo
// se sube bloque a bloque (`pvr_txr_load`). El backend devuelve -4 y este lo dice, en vez de escribir corrido.
func PresentarEnBloque(img *Imagen, xDestino, yDestino int) error {
	if img == nil || len(img.pixeles) == 0 {
		return errorPantalla("el lienzo esta vacio")
	}

	r := d2cPvrPantallaConvertirEnBloque(uintptr(unsafe.Pointer(&img.pixeles[0])), img.ancho, img.alto,
		xDestino, yDestino)
	if r != 0 {
		return errorPantalla("no se pudo convertir/subir el lienzo en el bloque " + itoa(xDestino) + "," +
			itoa(yDestino) + " (codigo " + itoa(r) + ")")
	}

	return nil
}

// PintarEnBloque: pinta en la pantalla el recuadro (x, y, w, h) con lo que hay en la textura a partir de
// (tu, tv). Es un quad con muestreo nearest (el contexto se compila sin filtro en `pvr_init`), asi que los
// bordes del recuadro no se manchan con el lienzo vecino.
//
// El que llama tiene que tener abierta la lista OPACA del PVR y pintar TODOS los recuadros antes de que el
// ciclo cierre el cuadro: entre todos tienen que cubrir la pantalla, porque la lista es opaca y detras no hay
// nada.
func PintarEnBloque(x, y, w, h, tu, tv int) { d2cPvrPantallaPintarBloque(x, y, w, h, tu, tv) }

package ebiten

// DC-002.159c -- EL LIENZO DE PIXELES DE UNA Image, PRESTADO.
//
// POR QUE EXISTE. Un cuadro de animacion del motor se carga expandiendo sus INDICES de 8 bits con la paleta.
// El camino viejo (`d2util.ImgIndexToRGBA`) pide un buffer de 4 x pixeles en el monton, lo llena, y el que
// llama lo copia al lienzo y lo tira: un buffer de basura POR CUADRO DE ANIMACION (y el compuesto del heroe
// tiene 7 capas, o sea siete por cuadro). El backend PVR del POC ya tiene el atajo (`ExpandirIndices`, que
// escribe derecho en el lienzo, DC-002.85); el shim de ebiten, que es el backend con el que corre el MOTOR,
// no lo tenia.
//
// Esta puerta es la minima para tenerlo sin que el shim sepa de paletas del motor: devuelve los pixeles RGBA
// de la imagen (4 B por pixel, filas seguidas) PARA QUE ESCRIBA EL QUE LLAMA. Es un PRESTAMO: el slice vale
// mientras la imagen viva, el que escribe tiene que respetar `len`, y no se queda con el slice despues del
// cuadro.
func (i *Image) LienzoDePixeles() []uint8 {
	if i == nil {
		return nil
	}

	return i.pixeles
}

//go:build dreamcast

package ebiten

// DC-002.159 -- LA MEMORIA DE UN RECUADRO DEL LIENZO: guardar unos pixeles y volver a ponerlos.
//
// POR QUE EXISTE. Con el mapa estatico (DC-002.157) el pueblo se pinta UNA vez y no se repinta; el
// heroe, en cambio, se mueve. Si el ente se dibuja encima del mapa y el mapa no se vuelve a pintar,
// el heroe deja un rastro de pixeles viejos. La forma barata de borrarlo -- la de los juegos 2D de
// siempre -- es guardar el recuadro de lienzo que el ente tapa ANTES de dibujarlo y devolverlo antes
// de dibujarlo en su posicion nueva: cuesta unos KB por cuadro en vez de los ~800 ms del mapa.
//
// EL LIENZO ES RGBA DE 8 BITS POR CANAL (4 B por pixel), el mismo que arma `NewImage`, asi que
// guardar un recuadro es copiar filas. Nada de esto toca el camino de la PC: el archivo va con
// `//go:build dreamcast` (que el `go build` del host respeta) y el build de la consola lo toma por
// NOMBRE, como todos los archivos de consola de este backend.
//
// DC-002.159b -- EL RECORTE AL LIENZO. Antes, un recuadro que se salia del lienzo devolvia 0 y el que llamaba
// se quedaba sin memoria (o sea: sin forma de borrar al heroe, que es el camino de repintar el mapa entero -- y
// desde esta tarjeta el mapa ya no se puede repintar). Ahora el recuadro se RECORTA al lienzo y se devuelve el
// recorte, asi el borrado funciona con el heroe en el borde de la pantalla, y cuando el recuadro no toca el
// lienzo (el heroe se fue de la vista) devuelve 0 y el que llama no dibuja: nada que borrar.

// CopiarRect: copia el recuadro (x,y,ancho,alto) del lienzo a `destino` (filas seguidas, RGBA) y
// devuelve cuantos bytes escribio MAS el recorte que de verdad se copio (xc, yc, anchoC, altoC).
//
// Si el recuadro no toca el lienzo devuelve todo en cero y no toca nada: el que llama se queda sin memoria, y
// como no hay nada del ente en la pantalla, tampoco hay rastro que borrar.
func (i *Image) CopiarRect(destino []byte, x, y, ancho, alto int) (escritos, xc, yc, anchoC, altoC int) {
	if i == nil || ancho <= 0 || alto <= 0 || len(destino) < ancho*alto*4 {
		return 0, 0, 0, 0, 0
	}

	xc, yc, anchoC, altoC = recorteAlLienzo(x, y, ancho, alto, i.ancho, i.alto)
	if anchoC <= 0 || altoC <= 0 {
		return 0, 0, 0, 0, 0
	}

	for fila := 0; fila < altoC; fila++ {
		origen := ((yc+fila)*i.ancho + xc) * 4
		copia(destino[escritos:], i.pixeles[origen:origen+anchoC*4])
		escritos += anchoC * 4
	}

	return escritos, xc, yc, anchoC, altoC
}

// PegarRect: pone `origen` (filas seguidas, RGBA, `ancho` de fila) en el recuadro (x,y,ancho,alto) del lienzo y
// devuelve cuantos bytes escribio. El recorte se aplica igual que en `CopiarRect`, sobre las mismas
// coordenadas: lo que se guardo es exactamente lo que se devuelve.
func (i *Image) PegarRect(origen []byte, x, y, ancho, alto int) int {
	if i == nil || ancho <= 0 || alto <= 0 || len(origen) < ancho*alto*4 {
		return 0
	}

	xc, yc, anchoC, altoC := recorteAlLienzo(x, y, ancho, alto, i.ancho, i.alto)
	if anchoC <= 0 || altoC <= 0 {
		return 0
	}

	escritos := 0

	for fila := 0; fila < altoC; fila++ {
		destino := ((yc+fila)*i.ancho + xc) * 4
		copia(i.pixeles[destino:destino+anchoC*4], origen[escritos:escritos+anchoC*4])
		escritos += anchoC * 4
	}

	// el recuadro se volvio a pintar: el cuadro cambio y hay que volver a convertir y subir (el
	// contador de "presentacion quieta" de DC-002.157 se pregunta antes de convertir)
	contarEscritura157(i)

	return escritos
}

// recorteAlLienzo: la parte del recuadro (x,y,ancho,alto) que cae dentro de un lienzo de anchoLienzo x altoLienzo.
// Devuelve las coordenadas del recorte y su tamano (0 en el tamano si no hay interseccion).
func recorteAlLienzo(x, y, ancho, alto, anchoLienzo, altoLienzo int) (xc, yc, anchoC, altoC int) {
	x0, y0 := x, y
	x1, y1 := x+ancho, y+alto

	if x0 < 0 {
		x0 = 0
	}

	if y0 < 0 {
		y0 = 0
	}

	if x1 > anchoLienzo {
		x1 = anchoLienzo
	}

	if y1 > altoLienzo {
		y1 = altoLienzo
	}

	if x1 <= x0 || y1 <= y0 {
		return 0, 0, 0, 0
	}

	return x0, y0, x1 - x0, y1 - y0
}

// copia: el `copy` del lenguaje, con nombre propio para que se lea que es una copia de bytes (y para
// que el reemplazo a mano de un bloque sea un solo lugar).
func copia(destino, origen []byte) { copy(destino, origen) }

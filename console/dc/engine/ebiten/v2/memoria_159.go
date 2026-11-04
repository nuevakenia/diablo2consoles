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

// CopiarRect: copia el recuadro (x,y,ancho,alto) del lienzo a `destino` (filas seguidas, RGBA) y
// devuelve cuantos bytes escribio. Si el recuadro no entra en el lienzo devuelve 0 y no toca nada:
// el que llama se queda sin memoria y vuelve al camino de repintar.
func (i *Image) CopiarRect(destino []byte, x, y, ancho, alto int) int {
	if i == nil || ancho <= 0 || alto <= 0 {
		return 0
	}

	if x < 0 || y < 0 || x+ancho > i.ancho || y+alto > i.alto {
		return 0
	}

	if len(destino) < ancho*alto*4 {
		return 0
	}

	escritos := 0

	for fila := 0; fila < alto; fila++ {
		origen := ((y+fila)*i.ancho + x) * 4
		copia(destino[escritos:], i.pixeles[origen:origen+ancho*4])
		escritos += ancho * 4
	}

	return escritos
}

// PegarRect: pone `origen` (filas seguidas, RGBA) en el recuadro (x,y,ancho,alto) del lienzo y
// devuelve cuantos bytes escribio. Mismas condiciones que CopiarRect.
func (i *Image) PegarRect(origen []byte, x, y, ancho, alto int) int {
	if i == nil || ancho <= 0 || alto <= 0 {
		return 0
	}

	if x < 0 || y < 0 || x+ancho > i.ancho || y+alto > i.alto {
		return 0
	}

	if len(origen) < ancho*alto*4 {
		return 0
	}

	escritos := 0

	for fila := 0; fila < alto; fila++ {
		destino := ((y+fila)*i.ancho + x) * 4
		copia(i.pixeles[destino:destino+ancho*4], origen[escritos:escritos+ancho*4])
		escritos += ancho * 4
	}

	// el recuadro se volvio a pintar: el cuadro cambio y hay que volver a convertir y subir (el
	// contador de "presentacion quieta" de DC-002.157 se pregunta antes de convertir)
	contarEscritura157(i)

	return escritos
}

// copia: el `copy` del lenguaje, con nombre propio para que se lea que es una copia de bytes (y para
// que el reemplazo a mano de un bloque sea un solo lugar).
func copia(destino, origen []byte) { copy(destino, origen) }

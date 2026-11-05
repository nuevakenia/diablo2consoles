//go:build dreamcast

package ebiten

// DC-002.159 -- LA SUPERFICIE DEL CUADRO SABE GUARDAR Y DEVOLVER UN RECUADRO DE SUS PIXELES.
//
// POR QUE EXISTE. El heroe del Gate 0 se dibuja encima de un mapa que NO se repinta (el mapa estatico
// de DC-002.157): sin memoria de abajo, cada paso del heroe deja un rastro. `d2game/d2gamescreen`
// pregunta por esta interfaz en vez de importar nada de la consola, asi que en la PC (donde la
// Surface es Ebitengine de verdad y no la implementa) el camino es el de siempre: mapa entero cada
// cuadro y los entes dentro de las pasadas del renderer.
//
// LA CUENTA LA HACE EL SHIM (`(*ebiten.Image).CopiarRect` / `PegarRect`), que es quien tiene los
// pixeles. Aca solo se pasa el recuadro, en coordenadas del LIENZO (el motor dibuja el mundo en el
// origen de la columna, que con `AnchorBottom` es 0,0).
//
// DC-002.159b: el recuadro se RECORTA al lienzo (ver el shim) y el recorte viaja de vuelta al que llamo, que es
// el que tiene que devolver exactamente esos pixeles.

// CopiarRect159: guarda el recuadro del lienzo en `destino` (RGBA de 8 bits, filas seguidas) y devuelve cuantos
// bytes escribio mas el recorte que se copio de verdad (xc, yc, anchoC, altoC).
func (s *ebitenSurface) CopiarRect159(destino []byte, x, y, ancho, alto int) (escritos, xc, yc, anchoC, altoC int) {
	return s.image.CopiarRect(destino, x, y, ancho, alto)
}

// PegarRect159: devuelve el recuadro guardado al lienzo.
func (s *ebitenSurface) PegarRect159(origen []byte, x, y, ancho, alto int) int {
	return s.image.PegarRect(origen, x, y, ancho, alto)
}

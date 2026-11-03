//go:build dreamcast

// DC-002.135 -- EL CAMINO PLANO DEL BLIT: UNA RAMA POR PIXEL, Y CUATRO PIXELES POR PALABRA LEIDA.
//
// LO QUE MIDIO LA TARJETA (palanca `BUCLE_135`, mismo binario, mismo cuadro, 248 tiles):
//
//	normal     pintado= 332.610 us    <- el bucle del juego
//	lectura    pintado= 133.150 us    <- lee el indice y la tabla y NO escribe
//	escritura  pintado=  68.646 us    <- NO lee el indice: escribe en los mismos destinos
//	tabla2     pintado= (ver evidencia) <- lo que cuesta armar la tabla de 256 palabras
//
// O sea: leer son ~17 ciclos por pixel, escribir ~9, y el bucle COMPLETO son ~43. Los ~17 ciclos que no
// aparecen en las partes son TRES comparaciones por pixel (el indice 0, el alfa 0 y el alfa 255) mas el
// acarreo de la palabra de la tabla. En el mapa del nivel dos de esas tres ramas sobran SIEMPRE:
//
//   - la paleta del acto es OPACA (los 256 colores del `.dat` tienen alfa 255, ver `DATColor.A()`), asi que
//     `a == 255` es siempre cierto y la mezcla `source-over` no se ejecuta nunca;
//   - el estado es NEUTRO (el mapa se pinta sin `PushColor`, con la luz apagada), asi que la tabla es la
//     paleta tal cual.
//
// Con las dos condiciones el bucle del camino general se puede escribir como el de DC-002.134 sin tinte
// (`renderIndicesPalabras`): leer el indice, si es 0 no pintar, si no copiar la palabra de la tabla. ESO ES LO
// MISMO, pixel a pixel, sin una cuenta de mas -- y con UNA rama en vez de tres.
//
// ADEMAS: LOS INDICES SE LEEN DE A CUATRO. El buffer de indices vive en la VRAM (el tramo de la cache) y cada
// lectura de 8 bits es un viaje al bus; leer una PALABRA de 32 bits trae los cuatro indices de una vez y deja
// el lazo en un cuarto de las vueltas. La alineacion no se adivina: la palabra solo se lee cuando
// `(fila + x) % 4 == 0` y el ancho del tile es multiplo de 4; si no, se pinta ese pixel por el camino de bytes
// (el resultado es identico: los mismos indices, la misma tabla, los mismos destinos).
//
// LA PRUEBA DE QUE NO CAMBIA UN PIXEL: `crc32` del cuadro 1 = 1437022778 (igual al host) y la captura del
// pueblo comparada byte a byte contra la de DC-002.134. El verificador del host
// (`console/dc/verif-blit-134`) pinta los mismos casos por los tres caminos y los compara.
package pvr

import "unsafe"

// palabraDelIndice135: la vista de 32 bits de un buffer de indices, o nil si no se puede leer asi.
//
// Se pide UNA vez por tile (no por pixel). `ancho % 4 != 0` la descarta porque entonces las filas no empiezan
// alineadas y la palabra de una fila se comeria bytes de la siguiente.
func palabraDelIndice135(indices []byte, ancho int) []uint32 {
	if ancho%4 != 0 || len(indices) < 4 {
		return nil
	}

	p := unsafe.Pointer(&indices[0])
	if uintptr(p)%4 != 0 {
		return nil
	}

	n := len(indices) / 4

	return (*[maximoDePalabras]uint32)(p)[:n:n]
}

// paletaOpaca135: si los 256 colores de la paleta son opacos (alfa 255). Se acuerda de la ULTIMA paleta
// comprobada para no recorrer 256 entradas por tile: la paleta del nivel es una sola y se compara el puntero,
// asi que la comprobacion cuesta una comparacion por tile. (Es una slice de 24 B y un bool: el estado mas
// chico posible; la tabla de 1 KB de DC-002.134 no entraba en este monton, esto si.)
var (
	paletaOpaca135   []uint32 // direccion de la ultima paleta comprobada (la direccion de su primer color)
	paletaOpacaDijo  bool
	paletaOpacaVeces int
)

func paletaEsOpaca135(paleta [][4]uint8) bool {
	if len(paleta) < 256 {
		return false
	}

	direccion := uint32(uintptr(unsafe.Pointer(&paleta[0])))
	if paletaOpaca135 != nil && paletaOpaca135[0] == direccion {
		return paletaOpacaDijo
	}

	opaca := true

	for i := 0; i < 256; i++ {
		if paleta[i][3] != 255 {
			opaca = false

			break
		}
	}

	paletaOpaca135 = []uint32{direccion}
	paletaOpacaDijo = opaca
	paletaOpacaVeces++

	return opaca
}

// CaminoPlanoDisponible135: si ESTA llamada puede ir por el camino plano (estado neutro y paleta opaca). Lo
// usa el que llama para no pagar la comprobacion cuando el estado trae color.
func CaminoPlanoDisponible135(neutro bool, paleta [][4]uint8) bool {
	return CaminoPlanoPrendido135 && neutro && paletaEsOpaca135(paleta)
}

// CaminoPlanoPrendido135: la palanca del A/B (`PLANO_135=si|no`). Apagada, el mapa del nivel vuelve al bucle
// general de `renderIndicesTintadoPalabras` -- el MISMO dibujo (las dos ramas escriben los mismos bytes), para
// poder medir el camino plano contra el general en la misma corrida.
var CaminoPlanoPrendido135 = true

// FijarCaminoPlano135: prende o apaga el camino plano del blit.
func FijarCaminoPlano135(si bool) { CaminoPlanoPrendido135 = si }

// CaminoPlanoUsado135: cuantos blits fueron por el camino plano (control de que se midio lo que se cree).
var CaminoPlanoUsado135 int

// VecesQueSeComproboLaPaleta135: cuantas veces se recorrieron los 256 colores (1 es lo esperado: la paleta del
// nivel no cambia; si crece, hay paletas distintas por cuadro y la cache no sirve).
func VecesQueSeComproboLaPaleta135() int { return paletaOpacaVeces }

// vecesQueSeComproboLaPaleta135: el nombre viejo, para el que ya lo llamaba asi.
func vecesQueSeComproboLaPaleta135() int { return paletaOpacaVeces }

// renderIndicesTintadoPlano135: el bucle del camino plano (una rama por pixel, cuatro pixeles por palabra).
// Devuelve false solo si el lienzo no esta alineado a 4 (el que llama sigue por el camino de bytes).
//
// DC-002.141: con la lista de recortes puesta (el cuadro que camina), el rango del tile se interseca con cada
// rectangulo y se pinta por pedazos con EL MISMO bucle (`plano135Rango`). Sin recortes se llama una sola vez
// con el rango entero: el camino de siempre, sin una cuenta de mas.
func (s *Surface) renderIndicesTintadoPlano135(indices []byte, ancho, alto int, tabla *[256]uint32,
	x0, x1, y0, y1 int) bool {
	lienzo := palabrasDelLienzo(s.imagen.pixeles)
	if lienzo == nil {
		avisosDeAlineacion++

		return false
	}

	CaminoPlanoUsado135++

	palabras := palabraDelIndice135(indices, ancho)

	if n := s.imagen.clipN; n > 0 {
		dx, dy := s.actual.x, s.actual.y

		for k := 0; k < n; k++ {
			lx0, lx1, ly0, ly1 := s.imagen.rangoEnRecorte141(k, dx, dy, x0, x1, y0, y1)
			if lx0 >= lx1 || ly0 >= ly1 {
				continue
			}

			s.plano135Rango(lienzo, indices, palabras, tabla, ancho, lx0, lx1, ly0, ly1)
		}

		return true
	}

	s.plano135Rango(lienzo, indices, palabras, tabla, ancho, x0, x1, y0, y1)

	return true
}

// plano135Rango: el bucle de pixeles del camino plano, para el rango local [x0,x1) x [y0,y1) del tile.
func (s *Surface) plano135Rango(lienzo []uint32, indices []byte, palabras []uint32, tabla *[256]uint32,
	ancho, x0, x1, y0, y1 int) {
	if s.imagen.clipN > 0 {
		pixelesPintados141 += (x1 - x0) * (y1 - y0)
	}

	dx, dy := s.actual.x, s.actual.y
	anchoLienzo := s.imagen.ancho

	for y := y0; y < y1; y++ {
		fila := y * ancho
		j := (dy+y)*anchoLienzo + dx + x0

		x := x0

		if palabras != nil {
			// los primeros pixeles hasta caer en un multiplo de 4 (la fila puede empezar corrida)
			for x < x1 && (fila+x)%4 != 0 {
				if idx := indices[fila+x]; idx != 0 {
					lienzo[j] = tabla[idx]
				}

				x, j = x+1, j+1
			}

			for ; x+4 <= x1; x, j = x+4, j+4 {
				w := palabras[(fila+x)>>2]
				if w == 0 { // cuatro pixeles transparentes: la mitad de los del tile, sin tocar la tabla
					continue
				}

				if b := byte(w); b != 0 {
					lienzo[j] = tabla[b]
				}

				if b := byte(w >> 8); b != 0 {
					lienzo[j+1] = tabla[b]
				}

				if b := byte(w >> 16); b != 0 {
					lienzo[j+2] = tabla[b]
				}

				if b := byte(w >> 24); b != 0 {
					lienzo[j+3] = tabla[b]
				}
			}
		}

		for ; x < x1; x, j = x+1, j+1 {
			idx := indices[fila+x]
			if idx == 0 {
				continue
			}

			lienzo[j] = tabla[idx]
		}
	}

	pxDeIndices += uint64((x1 - x0) * (y1 - y0))
}

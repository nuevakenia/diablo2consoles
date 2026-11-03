//go:build dreamcast

// DC-002.134 -- EL BLIT POR PALABRAS: un solo deposito de 32 bits por pixel y la paleta armada UNA vez.
//
// EL PROBLEMA, MEDIDO. En DC-002.132 el mapa del pueblo bajo de 582 a 367 ms por cuadro en la consola con el
// recorte afuera del bucle y el camino sin tinte. Lo que quedo, medido en DC-002.134 con el cronometro por
// etapa (`JUEGO134: ETAPAS`), es esto: el mapa sigue siendo ~389 ms de los ~650 del cuadro, y el resto de las
// piezas (jugador 59 ms, minimapa 55, bicho 23, HUD 20) paga el MISMO bucle.
//
// POR QUE CUESTA TANTO ESE BUCLE. Los pixeles del lienzo viven en la VRAM (`SuperficieDeLaPantalla`), y el
// bucle de DC-002.132 escribe CUATRO bytes por pixel:
//
//	pixeles[j+0] = c[0]
//	pixeles[j+1] = c[1]
//	pixeles[j+2] = c[2]
//	pixeles[j+3] = 255
//
// O sea: cuatro depositos a VRAM sin cache por pixel, mas la lectura de la paleta de 4 bytes. El mapa del
// pueblo son ~248 tiles de 160x160 (6,3 M de pixeles, veinte veces los 307.200 que se ven, porque los tiles
// isometricos se pisan), asi que son ~25 M de depositos por cuadro.
//
// LO QUE HACE ESTE CAMINO (no cambia un pixel: las cuatro reglas del camino de bytes se reproducen igual):
//
//  1. LA PALETA EMPAQUETADA. Una tabla de 256 palabras con el pixel YA armado como se escribe en el lienzo
//     (RGBA en little-endian: R en el byte 0, A en el 3). Se arma UNA vez por llamada -- 256 cuentas -- y no
//     por pixel: la cuenta del tinte (3 multiplicaciones y 3 divisiones por 255) desaparece del bucle.
//  2. UN DEPOSITO DE 32 BITS POR PIXEL en vez de cuatro de 8.
//  3. LA MEZCLA (alfa intermedio) sigue leyendo el pixel del lienzo, pero como UNA palabra en vez de cuatro
//     bytes: misma cuenta, mismo resultado, menos viajes al bus.
//
// LA ALINEACION NO SE ADIVINA: si el buffer de pixeles no esta alineado a 4 (no deberia pasar: la VRAM y el
// monton de C dan bloques alineados), se vuelve al camino de bytes de DC-002.132 y se cuenta el aviso. Asi
// este camino NUNCA puede romper un cuadro por un puntero raro.
//
// LA PALANCA (`pvr.FijarBlitPalabras`, la maneja el POC con `BLIT_PALABRAS=si|no`) deja el camino viejo en el
// MISMO binario: el A/B es una corrida, igual que el `BLIT_RAPIDO` de DC-002.132. Y la prueba de que el dibujo
// no cambio es doble: el `crc32` del cuadro 1 contra el host en la consola (1437022778) y el verificador del
// host `console/dc/verif-blit-134`, que pinta los mismos casos con los caminos y compara byte a byte.
package pvr

import "unsafe"

// maximoDePalabras: el tope del truco de conversion de puntero ([1 << 28]uint32 cabe en el espacio de la
// consola: el lienzo son 76.800 palabras). Es el patron de Go 1.16, que es el del compilador de la consola
// (no hay `unsafe.Slice`: el go.mod dice go 1.16).
const maximoDePalabras = 1 << 28

// blitPalabras: si el camino de 32 bits por pixel esta prendido. Lo fija `FijarBlitPalabras` al arranque.
var blitPalabras bool

// avisosDeAlineacion: cuantas veces se cayo al camino de bytes por un buffer desalineado (0 es lo esperado).
var avisosDeAlineacion int

// LOS CONTADORES DE PIXELES POR CAMINO (DC-002.134). No cuestan nada por pixel: se suman UNA vez por tile
// (el area del rectangulo que se pinto), y contestan la pregunta que quedo abierta: de los 384 ms del mapa,
// cuanto es el camino de indices (las 188 pisos) y cuanto el de copiar superficies (las 50 paredes de arriba y
// los 4 techos). Se reinician a mano desde el POC y se imprimen junto con los ms del mapa.
var (
	pxDeIndices uint64 // pixeles escritos por RenderIndices/RenderIndicesTintado
	pxDeCopiar  uint64 // pixeles escritos por copiarEn/copiarMovida (Render/RenderSection)
	pxDeSombras uint64 // reservado: en este backend el sombreado es PushColor + Render, o sea copiarEn
)

// (La tabla empaquetada NO se cachea: se arma por llamada en la pila. Ver el comentario largo en
// `renderIndicesPalabras`, donde esta el por que medido.)

// ContadoresDePixeles: los pixeles escritos por camino desde el ultimo reinicio.
//
// OJO CON LA LECTURA: no hay un camino de sombras aparte. El motor sombrea con `PushColor` + `Render`, y eso
// entra al camino de COPIAR, asi que la tercera cifra queda en 0 a proposito: si algun dia aparece un camino
// propio de sombras, esta aca el lugar donde contarlo.
func ContadoresDePixeles() (indices, copiar, sombras uint64) {
	return pxDeIndices, pxDeCopiar, pxDeSombras
}

// ReiniciarContadoresDePixeles: pone los tres en cero (el POC lo hace al empezar el cuadro que informa).
func ReiniciarContadoresDePixeles() { pxDeIndices, pxDeCopiar, pxDeSombras = 0, 0, 0 }

// FijarBlitPalabras: prende o apaga el camino de la palabra (DC-002.134).
func FijarBlitPalabras(si bool) { blitPalabras = si }

// CaminoDeLaPalabra: cual esta prendido (para el log del POC).
func CaminoDeLaPalabra() bool { return blitPalabras }

// AvisosDeAlineacion: cuantas veces hubo que caer al camino de bytes.
func AvisosDeAlineacion() int { return avisosDeAlineacion }

// palabrasDelLienzo: la vista de 32 bits del buffer de pixeles del lienzo, o nil si no esta alineado.
//
// El lienzo es RGBA de 8 bits y las palabras se escriben en el orden de la memoria (little-endian):
// byte 0 = R, byte 1 = G, byte 2 = B, byte 3 = A. Es EXACTAMENTE lo que escribe el camino de bytes.
func palabrasDelLienzo(pixeles []byte) []uint32 {
	if len(pixeles) < 4 {
		return nil
	}

	p := unsafe.Pointer(&pixeles[0])
	if uintptr(p)%4 != 0 {
		return nil
	}

	n := len(pixeles) / 4
	vista := (*[maximoDePalabras]uint32)(p)

	return vista[:n:n]
}

// palabraDelColor: el color de la paleta como se escribe en el lienzo (R, G, B, A en una palabra).
func palabraDelColor(r, g, b, a uint8) uint32 {
	return uint32(r) | uint32(g)<<8 | uint32(b)<<16 | uint32(a)<<24
}

// recorteDelBlit: el rango del tile que cae en el lienzo (una vez por tile, DC-002.132) y si queda algo.
//
// Es la MISMA cuenta que el camino de bytes: por eso los dos caminos pintan el mismo rectangulo.
func recorteDelBlit(dx, dy, ancho, alto, anchoLienzo, altoLienzo int) (x0, x1, y0, y1 int, vale bool) {
	x0, x1 = 0, ancho
	if dx < 0 {
		x0 = -dx
	}

	if dx+x1 > anchoLienzo {
		x1 = anchoLienzo - dx
	}

	if x0 >= x1 {
		return 0, 0, 0, 0, false
	}

	y0, y1 = 0, alto
	if dy < 0 {
		y0 = -dy
	}

	if dy+y1 > altoLienzo {
		y1 = altoLienzo - dy
	}

	if y0 >= y1 {
		return 0, 0, 0, 0, false
	}

	return x0, x1, y0, y1, true
}

// renderIndicesPalabras: el `RenderIndices` (paleta de 3 canales, opaco, sin estado de color) por palabras.
// Devuelve false si no pudo (buffer desalineado): el que llama sigue por el camino de bytes.
//
// LAS TRES REGLAS DEL CAMINO DE BYTES, iguales: el indice 0 no pinta, el resto toma R,G,B de la paleta y el
// alfa queda en 255 (la paleta de 3 canales no tiene alfa: el tile del mapa es opaco).
func (s *Surface) renderIndicesPalabras(indices []byte, ancho, alto int, paleta [][3]uint8) bool {
	lienzo := palabrasDelLienzo(s.imagen.pixeles)
	if lienzo == nil {
		avisosDeAlineacion++

		return false
	}

	dx, dy := s.actual.x, s.actual.y
	anchoLienzo, altoLienzo := s.imagen.ancho, s.imagen.alto

	x0, x1, y0, y1, vale := recorteDelBlit(dx, dy, ancho, alto, anchoLienzo, altoLienzo)
	if !vale {
		return true
	}

	// DC-002.141: el tile que no toca ningun recorte vuelve antes de armar la tabla (ver el comentario largo en
	// `renderIndicesTintadoPalabras`).
	if s.imagen.clipN > 0 {
		blitsPedidos141++

		if !s.imagen.tocaAlRecorte141(dx, dy, ancho, alto) {
			blitsSaltados141++

			return true
		}

		blitsPintados141++
	}

	// LA TABLA, EN LA PILA Y SIN CACHE. Se probo cachearla (una global de 1 KB, clave = paleta + tinte) para que
	// el minimapa, que repite paleta en sus 680 celdas, no armara la tabla 680 veces: el POC se murio en el primer
	// cuadro con "GC to-space overflow - your live data doesn't fit in the heap", con o sin la tabla por
	// superficie. El monton de esta consola (semiespacio de 1792 KB, pico 9,0 MB de los 10 del arena) no tiene el
	// margen para guardar estado del dibujado, y el intento queda documentado en la evidencia de DC-002.134.
	var tabla [256]uint32

	for i := 0; i < 256; i++ {
		c := paleta[i]
		tabla[i] = palabraDelColor(c[0], c[1], c[2], 255)
	}

	// DC-002.141: con la lista de recortes puesta se pinta el pedazo que cae en cada rectangulo (ver
	// recortes_141.go); sin recortes, el rango entero de siempre.
	if n := s.imagen.clipN; n > 0 {
		for k := 0; k < n; k++ {
			lx0, lx1, ly0, ly1 := s.imagen.rangoEnRecorte141(k, dx, dy, x0, x1, y0, y1)
			if lx0 >= lx1 || ly0 >= ly1 {
				continue
			}

			s.palabrasRango(lienzo, indices, &tabla, ancho, anchoLienzo, dx, dy, lx0, lx1, ly0, ly1)
		}

		return true
	}

	s.palabrasRango(lienzo, indices, &tabla, ancho, anchoLienzo, dx, dy, x0, x1, y0, y1)

	return true
}

// palabrasRango: el bucle de pixeles del camino de palabras (un deposito de 32 bits por pixel), para el rango
// local [x0,x1) x [y0,y1) del tile. Lo comparten el `RenderIndices` y el `RenderIndicesTintado` sin tinte.
func (s *Surface) palabrasRango(lienzo []uint32, indices []byte, tabla *[256]uint32,
	ancho, anchoLienzo, dx, dy, x0, x1, y0, y1 int) {
	for y := y0; y < y1; y++ {
		fila := y * ancho
		j := (dy+y)*anchoLienzo + dx + x0

		for x := x0; x < x1; x, j = x+1, j+1 {
			idx := indices[fila+x]
			if idx == 0 {
				continue
			}

			lienzo[j] = tabla[idx]
		}
	}

	pxDeIndices += uint64((x1 - x0) * (y1 - y0))
}

// renderIndicesTintadoPalabras: el `RenderIndicesTintado` por palabras (traslacion, color y alfa del estado).
//
// LA CUENTA ES LA DEL CAMINO DE BYTES, hecha en la TABLA y no en el pixel:
//
//	r = paleta.R * color.R / 255   (idem G, B)      a = paleta.A * color.A / 255
//	a == 0   -> no pinta
//	a == 255 -> escribe r, g, b, 255
//	si no    -> source-over con el pixel que ya estaba: (c*a + destino*(255-a)) / 255
//
// Con el estado neutro (255,255,255,255) la tabla es la paleta tal cual -- que es el camino rapido de
// DC-002.132 -- pero ademas con un solo deposito por pixel.
func (s *Surface) renderIndicesTintadoPalabras(indices []byte, ancho, alto int, paleta [][4]uint8) bool {
	lienzo := palabrasDelLienzo(s.imagen.pixeles)
	if lienzo == nil {
		avisosDeAlineacion++

		return false
	}

	dx, dy := s.actual.x, s.actual.y
	anchoLienzo, altoLienzo := s.imagen.ancho, s.imagen.alto

	// DC-002.141: si el recorte esta puesto y este tile no toca ningun rectangulo, se vuelve ANTES de armar la
	// tabla de 256 palabras -- que es lo caro del blit. Sin esto, un cuadro que camina paga el costo fijo de los
	// 241 tiles para pintar 25.000 pixeles (medido: 22-28 ms en vez de 1-2).
	if s.imagen.clipN > 0 {
		blitsPedidos141++

		if !s.imagen.tocaAlRecorte141(dx, dy, ancho, alto) {
			blitsSaltados141++

			return true
		}

		blitsPintados141++
	}

	rojo, verde, azul, alfa := uint16(255), uint16(255), uint16(255), uint16(255)

	if s.actual.col != nil {
		c := aRGBA(s.actual.col)
		rojo, verde, azul, alfa = uint16(c.R), uint16(c.G), uint16(c.B), uint16(c.A)
	}

	neutro := rojo == 255 && verde == 255 && azul == 255 && alfa == 255

	// LA TABLA, EN LA PILA Y SIN CACHE (ver el comentario largo en `renderIndicesPalabras`).
	var tabla [256]uint32

	for i := 0; i < 256; i++ {
		p := paleta[i]

		if neutro {
			tabla[i] = palabraDelColor(p[0], p[1], p[2], p[3])

			continue
		}

		tabla[i] = palabraDelColor(uint8(uint16(p[0])*rojo/255), uint8(uint16(p[1])*verde/255),
			uint8(uint16(p[2])*azul/255), uint8(uint16(p[3])*alfa/255))
	}

	x0, x1, y0, y1, vale := recorteDelBlit(dx, dy, ancho, alto, anchoLienzo, altoLienzo)
	if !vale {
		return true
	}

	// DC-002.135 -- LAS PALANCAS DE DIAGNOSTICO DEL BUCLE (ver `diagnostico_135.go`).
	//
	// Son TRES corridas de MEDIR, no caminos del juego: ROMPEN el dibujo a proposito y existen para partir el
	// tiempo por pixel en sus pedazos (leer el indice, escribir el pixel, armar la tabla). Vienen apagadas y la
	// corrida del criterio las lleva apagadas: con las tres en "no" este `if` es una comparacion por tile.
	if diagnostico135 != "" {
		return s.bucleDeDiagnostico135(indices, ancho, alto, paleta, tabla, x0, x1, y0, y1)
	}

	// DC-002.135 -- EL CAMINO PLANO (ver `plano_135.go`): estado neutro y paleta opaca, o sea el mapa del nivel.
	// Una rama por pixel y cuatro pixeles por palabra leida, con la MISMA tabla: el dibujo no cambia.
	if CaminoPlanoDisponible135(neutro, paleta) {
		if s.renderIndicesTintadoPlano135(indices, ancho, alto, &tabla, x0, x1, y0, y1) {
			return true
		}
	}

	// DC-002.141: la lista de recortes (ver recortes_141.go). Se pinta el pedazo de cada rectangulo con el
	// MISMO bucle; sin recortes, el rango entero de siempre.
	if n := s.imagen.clipN; n > 0 {
		for k := 0; k < n; k++ {
			lx0, lx1, ly0, ly1 := s.imagen.rangoEnRecorte141(k, dx, dy, x0, x1, y0, y1)
			if lx0 >= lx1 || ly0 >= ly1 {
				continue
			}

			s.tintadoPalabrasRango(lienzo, indices, &tabla, ancho, anchoLienzo, dx, dy, lx0, lx1, ly0, ly1)
		}

		return true
	}

	s.tintadoPalabrasRango(lienzo, indices, &tabla, ancho, anchoLienzo, dx, dy, x0, x1, y0, y1)

	return true
}

// tintadoPalabrasRango: el bucle de pixeles con color y alfa del estado, para el rango local pedido.
func (s *Surface) tintadoPalabrasRango(lienzo []uint32, indices []byte, tabla *[256]uint32,
	ancho, anchoLienzo, dx, dy, x0, x1, y0, y1 int) {
	if s.imagen.clipN > 0 {
		pixelesPintados141 += (x1 - x0) * (y1 - y0)
	}

	for y := y0; y < y1; y++ {
		fila := y * ancho
		j := (dy+y)*anchoLienzo + dx + x0

		for x := x0; x < x1; x, j = x+1, j+1 {
			idx := indices[fila+x]
			if idx == 0 {
				continue
			}

			v := tabla[idx]

			a := uint16(v >> 24)
			if a == 0 {
				continue
			}

			if a == 255 {
				lienzo[j] = v

				continue
			}

			// source-over: el mismo cociente por 255 del camino de bytes, con el destino leido como palabra
			destino := lienzo[j]
			resto := 255 - a

			r := uint8((uint16(uint8(v))*a + uint16(uint8(destino))*resto) / 255)
			g := uint8((uint16(uint8(v>>8))*a + uint16(uint8(destino>>8))*resto) / 255)
			b := uint8((uint16(uint8(v>>16))*a + uint16(uint8(destino>>16))*resto) / 255)
			al := uint8(a + uint16(uint8(destino>>24))*resto/255)

			lienzo[j] = palabraDelColor(r, g, b, al)
		}
	}

	pxDeIndices += uint64((x1 - x0) * (y1 - y0))
}

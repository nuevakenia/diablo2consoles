//go:build dreamcast

package pvr

// DC-002.141 -- EL RECORTE POR LISTA DE RECTANGULOS: pintar el mapa SOLO donde el cuadro cambio.
//
// POR QUE HACE FALTA. Con el lienzo que camina (ver `MoverLienzo`) el cuadro no se repinta entero: el mundo se
// corre unos pixeles con una copia y despues hay que VOLVER A PINTAR SOLO LO QUE NO VINO EN LA COPIA. Eso son
// pocos pedazos sueltos, no uno:
//
//   - las FRANJAS que entran por el borde (una columna de |sx| pixeles y/o una fila de |sy|);
//   - las CAJAS de los entes del cuadro anterior, corridas con la copia (donde quedaron los pixeles del
//     jugador, los bichos, los NPCs y los objetos): sin eso queda el fantasma del sprite viejo;
//   - las CAJAS de los paneles (HUD, minimapa) ENTERAS, no solo la banda: los paneles NO son opacos (el arte
//     del globo tiene 1.376 píxeles transparentes de 6.400, y con `hudPiezas=esenciales` no hay panel de fondo),
//     asi que por sus agujeros se ve el mundo y el mundo tiene que estar limpio; y ademas la banda que asoma del
//     panel del cuadro anterior, corrido con la copia, que si no deja su fantasma al costado.
//
// CON UN SOLO RECORTE HABRIA QUE LLAMAR A `Render` UNA VEZ POR PEDAZO, y cada llamada recorre los 248 tiles
// con sus cuatro pasadas: se pagaria el arranque del renderer seis o diez veces por cuadro. Con la lista, el
// renderer corre UNA vez y cada tile decide, por cada rectangulo, que pedazo de si mismo cae adentro.
//
// COMO SE APLICA, Y POR QUE NO CUESTA NADA CUANDO NO SE USA. El recorte vive en la `Imagen` (es el destino el
// que recorta) y los blits lo consultan UNA vez por tile: con la lista vacia -- el caso de todos los cuadros
// que no caminan, y de todos los otros dibujados -- el `if` es una comparacion por tile y el bucle de pixeles
// queda exactamente igual que en DC-002.135. Con la lista puesta, el rango local del tile se interseca con
// cada rectangulo y se pinta por pedazos, con el MISMO bucle y la misma tabla.
//
// LOS BYTES QUE LLEGAN AL LIENZO SON LOS MISMOS que los de un repintado entero: el recorte no cambia la cuenta
// de ningun pixel, solo decide cuales se escriben. La prueba es el crc32 del cuadro 1 (que se pinta entero,
// como siempre) y la comparacion del cuadro TERMINADO -- mapa, entes, HUD y minimapa -- contra el mismo cuadro
// repintado entero: el mismo crc32 y los 300 bloques de 32x32 iguales (`COMPARAR_141=si`,
// `compararElCuadroTerminado141` en el POC).

// recortesMax141: cuantos rectangulos se soportan por lienzo. Los de un cuadro caminando son las dos franjas
// (hasta 3 pedazos), las cajas de los paneles con su banda (hasta 5 pedazos) y las cajas de los entes (un
// jugador, dos bichos, cuatro NPCs y los objetos del suelo): 20 alcanzaba, 32 deja aire y no cuesta nada
// (32 x 4 enteros = 512 B por lienzo). El censo lo vigila: `recortesIgnorados=` tiene que quedar en 0.
const recortesMax141 = 32

// RecortarA agrega un rectangulo a la lista de recortes de ESTA superficie (el destino).
//
// El rectangulo se guarda tal cual (el que llama ya lo recorta al lienzo); si no entra mas se ignora y el
// contador `RecortesIgnorados141` lo dice, que es la unica forma de que un recorte de mas no se note.
func (s *Surface) RecortarA(x0, y0, x1, y1 int) {
	if s == nil || s.imagen == nil {
		return
	}

	if s.imagen.clipN >= recortesMax141 {
		s.imagen.clipIgnorados++

		return
	}

	if x0 < 0 {
		x0 = 0
	}
	if y0 < 0 {
		y0 = 0
	}
	if x1 > s.imagen.ancho {
		x1 = s.imagen.ancho
	}
	if y1 > s.imagen.alto {
		y1 = s.imagen.alto
	}
	if x0 >= x1 || y0 >= y1 {
		return
	}

	r := &s.imagen.clip[s.imagen.clipN]
	r[0], r[1], r[2], r[3] = int32(x0), int32(y0), int32(x1), int32(y1)
	s.imagen.clipN++
}

// SinRecortes vacia la lista: el lienzo vuelve a aceptar todo lo que caiga adentro (el caso normal).
func (s *Surface) SinRecortes() {
	if s == nil || s.imagen == nil {
		return
	}

	s.imagen.clipN = 0
}

// RecorteCargado141: el rectangulo k de la lista de recortes. Es SOLO para el diagnostico de la comparacion
// (`COMPARAR_141`): cuando un bloque no coincide hay que saber que pedazos del lienzo se volvieron a pintar.
func (s *Surface) RecorteCargado141(k int) (int, int, int, int) {
	if s == nil || s.imagen == nil || k < 0 || k >= s.imagen.clipN {
		return 0, 0, 0, 0
	}

	r := s.imagen.clip[k]

	return int(r[0]), int(r[1]), int(r[2]), int(r[3])
}

// RecortesPuestos: cuantos rectangulos tiene la lista (0 = sin recorte).
func (s *Surface) RecortesPuestos() int {
	if s == nil || s.imagen == nil {
		return 0
	}

	return s.imagen.clipN
}

// RecortesIgnorados141: cuantos rectangulos se descartaron por no entrar en la lista (0 es lo esperado).
func (s *Surface) RecortesIgnorados141() int {
	if s == nil || s.imagen == nil {
		return 0
	}

	return s.imagen.clipIgnorados
}

// rangoEnRecorte141: el pedazo del rango LOCAL de un tile (x0..x1, y0..y1, en coordenadas del tile) que cae
// adentro del rectangulo k del lienzo, dadas la traslacion (dx, dy) con la que se esta pintando.
//
// Devuelve el rango local ya intersecado; si no queda nada, x0 >= x1 (o y0 >= y1) y el que llama no pinta.
func (i *Imagen) rangoEnRecorte141(k, dx, dy, x0, x1, y0, y1 int) (int, int, int, int) {
	r := &i.clip[k]

	if c := int(r[0]) - dx; c > x0 {
		x0 = c
	}
	if c := int(r[2]) - dx; c < x1 {
		x1 = c
	}
	if c := int(r[1]) - dy; c > y0 {
		y0 = c
	}
	if c := int(r[3]) - dy; c < y1 {
		y1 = c
	}

	return x0, x1, y0, y1
}

// dentroDeRecortes141: si el pixel de lienzo (x, y) cae en alguno de los rectangulos.
//
// Es el camino LENTO (una comprobacion por pixel) y solo lo usan los blits de bytes, que son los que corren
// cuando el lienzo no esta alineado a 4 -- o sea casi nunca. Los caminos rapidos (el plano de DC-002.135 y
// los de palabras) intersecan el rango una vez por rectangulo y por tile.
func (i *Imagen) dentroDeRecortes141(x, y int) bool {
	for k := 0; k < i.clipN; k++ {
		r := &i.clip[k]

		if x >= int(r[0]) && x < int(r[2]) && y >= int(r[1]) && y < int(r[3]) {
			return true
		}
	}

	return false
}

// tocaAlRecorte141: si un blit de ancho x alto dibujado en (dx, dy) toca ALGUN rectangulo. Se comprueba con
// dos comparaciones por rectangulo y sin tocar un pixel.
// POR QUE ESTA COMPROBACION EXISTE Y NO ALCANZA CON LA INTERSECCION. En un cuadro que camina el mapa se
// repinta solo en unos pocos rectangulos (las franjas, las cajas de los entes y las bandas de los paneles),
// pero el renderer igual recorre los 241 tiles del cuadro y le pide el blit a cada uno. El que NO toca ningun
// rectangulo tiene que volver ANTES de armar la tabla de 256 palabras (que es lo caro del blit: 256 cuentas
// de color por tile, ~20-40 us medidos); si la tabla se armara igual, el cuadro caminando pagaria el costo
// fijo del repintado entero sin pintar un pixel. Medido en la primera corrida de esta tarjeta: 22-28 ms de
// "franjas" para 25.000 pixeles, cuando los pixeles son 1 ms.
func (i *Imagen) tocaAlRecorte141(dx, dy, ancho, alto int) bool {
	if i.clipN == 0 {
		return true
	}

	fx0, fy0 := dx, dy
	fx1, fy1 := dx+ancho, dy+alto

	for k := 0; k < i.clipN; k++ {
		r := &i.clip[k]

		if fx1 <= int(r[0]) || fx0 >= int(r[2]) || fy1 <= int(r[1]) || fy0 >= int(r[3]) {
			continue
		}

		return true
	}

	return false
}

// EL CENSO DE LOS BLITS. Cuantos blits se PIDEN (o sea cuantos tiles recorre el renderer), cuantos se SALTAN
// por no tocar ningun recorte y cuantos se pintan de verdad. Es el numero que dice si el costo del cuadro
// caminando esta en los pixeles (pocos) o en el aparato por tile del renderer (que se paga igual por los 241).
var (
	blitsPedidos141  int
	blitsSaltados141 int
	blitsPintados141 int
	// pixelesPintados141: los pixeles que los blits del mapa escribieron de verdad (la suma de los rangos
	// intersecados). Es el numero que dice si el cuadro caminando pinta lo que pide o pinta el tile entero.
	pixelesPintados141 int
)

// CensoDeLosPixeles141: los pixeles escritos por los blits del mapa en el tramo, y los deja en cero.
func CensoDeLosPixeles141() int {
	p := pixelesPintados141
	pixelesPintados141 = 0

	return p
}

// CensoDeLosBlits141: los tres contadores del tramo, y los deja en cero.
func CensoDeLosBlits141() (pedidos, saltados, pintados int) {
	pedidos, saltados, pintados = blitsPedidos141, blitsSaltados141, blitsPintados141
	blitsPedidos141, blitsSaltados141, blitsPintados141 = 0, 0, 0

	return pedidos, saltados, pintados
}

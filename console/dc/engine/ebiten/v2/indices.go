package ebiten

// DC-002.155 -- PINTAR UNA IMAGEN DE INDICES DE 8 BITS EN UNA Image: EL CAMINO DE LA CACHE DEL MAPA.
//
// POR QUE EXISTE. El renderer del mapa del motor (`d2maprenderer`) puede guardar su cache de tiles como
// INDICES de 8 bits en vez de RGBA (DC-002.101): el pueblo entero son 29,36 MB en RGBA y 7,70 MB en
// indices, y en la consola esos 29 MB no existen. Para pintar esos indices el renderer le pide a la
// Surface del cuadro el metodo `RenderIndicesTintado(indices, ancho, alto, paleta)` -- lo tiene el backend
// PVR del POC (`d2render/pvr`) y NO lo tenia el shim de ebiten, que es el backend con el que corre el
// MOTOR. Sin esto, el cuadro del mapa sale NEGRO: `d2maprenderer` avisa
// "la Surface del cuadro no sabe dibujar indices (DC-002.101)" y no pinta nada.
//
// LA CUENTA ES LA MISMA DEL PVR (`pvr.Surface.RenderIndicesTintado`), pixel por pixel:
//
//  1. el indice 0 NO se pinta: es el agujero del tile (asi viene del `.dt1`);
//  2. el color de la paleta se multiplica por el TINTE del estado (los tres canales) y su alfa por el alfa
//     del estado -- es lo que hace que las sombras salgan al 62,7 % (`PushColor` con A=160);
//  3. lo que queda se mezcla con lo que ya habia en el lienzo con la cuenta de siempre (source-over), la
//     MISMA de `copiarMovida` de `dibujo.go`.
//
// LO QUE NO HACE: efectos del PL2, filtros ni cambios de saturación/brillo. El camino del pueblo no los usa
// (se dibuja con la luz apagada y sin `PushEffect`); si alguna vez hicieran falta, esto es lo que hay que
// extender.

// PintarIndicesTintado: expande `indices` (`ancho` x `alto`, 1 B por pixel) con `paleta` (256 entradas RGBA)
// en esta imagen, en la posicion (x, y), con el tinte (tr, tg, tb, ta) del estado de dibujado. Devuelve los
// pixeles que se pintaron (0 si no entro nada: sirve de instrumento).
func (i *Image) PintarIndicesTintado(indices []byte, ancho, alto int, paleta [][4]uint8, x, y int,
	tr, tg, tb, ta uint8) int {
	if i == nil || ancho <= 0 || alto <= 0 || len(indices) < ancho*alto || len(paleta) < 256 {
		return 0
	}

	// el recorte AFUERA del bucle: el rango visible del tile se calcula una vez (DC-002.132 del POC)
	x0, x1 := 0, ancho
	if x < 0 {
		x0 = -x
	}

	if x+x1 > i.ancho {
		x1 = i.ancho - x
	}

	if x0 >= x1 {
		return 0
	}

	y0, y1 := 0, alto
	if y < 0 {
		y0 = -y
	}

	if y+y1 > i.alto {
		y1 = i.alto - y
	}

	if y0 >= y1 {
		return 0
	}

	neutro := tr == 255 && tg == 255 && tb == 255 && ta == 255
	pintados := 0

	for fy := y0; fy < y1; fy++ {
		fila := fy * ancho
		d := ((y+fy)*i.ancho + (x + x0)) * 4

		for fx := x0; fx < x1; fx, d = fx+1, d+4 {
			idx := indices[fila+fx]
			if idx == 0 {
				continue
			}

			p := paleta[idx]

			if neutro && p[3] == 255 {
				i.pixeles[d+0], i.pixeles[d+1], i.pixeles[d+2], i.pixeles[d+3] = p[0], p[1], p[2], 255
				pintados++

				continue
			}

			r := uint8(uint16(p[0]) * uint16(tr) / 255)
			g := uint8(uint16(p[1]) * uint16(tg) / 255)
			b := uint8(uint16(p[2]) * uint16(tb) / 255)
			a := uint16(uint16(p[3]) * uint16(ta) / 255)

			if a == 0 {
				continue
			}

			if a == 255 {
				i.pixeles[d+0], i.pixeles[d+1], i.pixeles[d+2], i.pixeles[d+3] = r, g, b, 255
				pintados++

				continue
			}

			resto := 255 - a
			i.pixeles[d+0] = uint8((uint32(r)*uint32(a) + uint32(i.pixeles[d+0])*uint32(resto)) / 255)
			i.pixeles[d+1] = uint8((uint32(g)*uint32(a) + uint32(i.pixeles[d+1])*uint32(resto)) / 255)
			i.pixeles[d+2] = uint8((uint32(b)*uint32(a) + uint32(i.pixeles[d+2])*uint32(resto)) / 255)
			i.pixeles[d+3] = uint8(uint32(a) + uint32(i.pixeles[d+3])*uint32(resto)/255)
			pintados++
		}
	}

	PixelosDeIndices += pintados

	return pintados
}

// PixelosDeIndices: cuantos pixeles de INDICES se pintaron desde que arranco el programa. Es el
// instrumento que distingue "el mapa se dibujo" de "la pantalla quedo negra por otra razon": si este numero
// es 0 despues del primer cuadro, el camino de la cache en indices no corrio.
var PixelosDeIndices int

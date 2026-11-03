//go:build dreamcast

// DC-002.137 -- LA TABLA DEL BLIT ARMADA POR EL LLAMADOR: una sola tabla para las 610 celdas del minimapa.
//
// EL PROBLEMA, MEDIDO. El minimapa cuesta 61,6 ms por cuadro (6.1 KB de este informe) y no son los pixeles: las
// 610 celdas que caen en la caja pintan **4.968 pixeles** por cuadro (las del automapa son lineas finas de 8x16,
// ~8 pixeles cada una). Lo que pesa es lo que cada celda paga ADEMAS de sus pixeles:
//
//  1. la tabla empaquetada de 256 palabras se armaba UNA VEZ POR CELDA (610 x 256 = 156.160 entradas por
//     cuadro, treinta veces los pixeles que se pintan de verdad). La tabla se arma por llamada porque DC-002.134
//     midio que cachearla en el backend mata el POC (`GC to-space overflow`): el monton de esta consola no tiene
//     el margen para guardar estado del dibujado;
//  2. y la caja negra del minimapa (279x225 = 62.775 px) iba por `Poner`, pixel a pixel, con sus cuatro
//     comparaciones de recorte por pixel (ver `Imagen.Rectangulo`).
//
// LO QUE HACE ESTE ARCHIVO (sin cambiar un pixel):
//
//   - `TablaDelBlit` es la tabla de 256 palabras, pero vive en el LLAMADOR (una variable del POC), no en el
//     backend: el que pinta muchas celdas con la MISMA paleta y el MISMO estado la arma una vez por cuadro y la
//     pasa. El backend sigue sin guardar estado (la leccion de DC-002.134 se respeta: la cache es del POC).
//   - `RenderIndicesConTabla` pinta el tile con esa tabla y con la posicion (x, y) explicita, asi que el que
//     pinta 610 celdas no paga un `PushTranslation`/`Pop` por celda.
//
// LA CUENTA ES LA MISMA DEL CAMINO DE BYTES, pixel a pixel (y por eso el dibujo no cambia): indice 0 no pinta,
// alfa 0 no pinta, alfa 255 escribe la palabra y alfa intermedio es source-over con el cociente por 255. Es el
// mismo bucle de `renderIndicesTintadoPalabras`, con la tabla que trae el llamador.
//
// LA PRUEBA DE QUE NO CAMBIA UN PIXEL, doble: el verificador del host `console/dc/verif-blit-137` pinta los
// mismos casos por el camino de bytes y por este y los compara byte a byte (no necesita consola), y en la consola
// el `crc32` de la caja del minimapa (279x225) de cada cuadro sale IDENTICO con la palanca `MINIMAPA_RAPIDO=si` y
// con `MINIMAPA_RAPIDO=no` (el camino viejo vive en el mismo binario).
package pvr

// LOS CONTADORES DEL CAMINO (para la evidencia): cuantos tiles salieron por el atajo de la palabra (indices leidos
// de a cuatro) y cuantos por el bucle pixel a pixel. Se suman UNA vez por tile, no por pixel, y el POC los imprime
// en la linea del minimapa por dentro: 610/610 es el caso bueno, y si algun dia son 0/610 el atajo se cayo solo.
var (
	tilesPorPalabras137 uint64 // indices leidos de a cuatro
	tilesPorBytes137    uint64 // bucle pixel a pixel (buffer de indices no alineado)
	tilesCompactos137   uint64 // lista compacta de los pixeles que pintan
)

// ContadoresDeLaTabla137: (por palabras, por bytes, compactos) desde el ultimo reinicio.
func ContadoresDeLaTabla137() (palabras, bytes, compactos uint64) {
	return tilesPorPalabras137, tilesPorBytes137, tilesCompactos137
}

// ReiniciarContadoresDeLaTabla137: los pone en cero (el POC lo hace al empezar el tramo que informa).
func ReiniciarContadoresDeLaTabla137() {
	tilesPorPalabras137, tilesPorBytes137, tilesCompactos137 = 0, 0, 0
}

// TablaDelBlit: la paleta de un tile YA CONVERTIDA a palabras del lienzo (RGBA en little-endian), con el tinte del
// estado aplicado. La arma `ArmarTablaDelBlit` y la guarda EL QUE LLAMA: el backend no la recuerda entre llamadas.
//
// Son 1 KB en la pila/estatica del llamador (no en el monton) y sirven para cualquier cantidad de tiles: el
// minimapa pinta sus 610 celdas con una sola.
type TablaDelBlit struct {
	Color [256]uint32
}

// ArmarTablaDelBlit: arma `tabla` para `paleta` con el tinte del estado actual de la superficie (el mismo que
// aplicaria `RenderIndicesTintado`). Devuelve false si la paleta no tiene las 256 entradas que pide el contrato
// (en ese caso no toca la tabla y el llamador tiene que seguir por el camino de siempre).
//
// LA CUENTA ES LA DEL CAMINO DE BYTES: r = R * rojo / 255, etc.; con el estado neutro (255,255,255,255) la tabla es
// la paleta tal cual, que es el caso del mapa y del minimapa.
func (s *Surface) ArmarTablaDelBlit(paleta [][4]uint8, tabla *TablaDelBlit) bool {
	if tabla == nil || len(paleta) < 256 {
		return false
	}

	rojo, verde, azul, alfa := uint16(255), uint16(255), uint16(255), uint16(255)

	if s.actual.col != nil {
		c := aRGBA(s.actual.col)
		rojo, verde, azul, alfa = uint16(c.R), uint16(c.G), uint16(c.B), uint16(c.A)
	}

	neutro := rojo == 255 && verde == 255 && azul == 255 && alfa == 255

	for i := 0; i < 256; i++ {
		p := paleta[i]

		if neutro {
			tabla.Color[i] = palabraDelColor(p[0], p[1], p[2], p[3])

			continue
		}

		tabla.Color[i] = palabraDelColor(uint8(uint16(p[0])*rojo/255), uint8(uint16(p[1])*verde/255),
			uint8(uint16(p[2])*azul/255), uint8(uint16(p[3])*alfa/255))
	}

	return true
}

// RenderIndicesConTabla: pinta el tile de indices con la tabla YA ARMADA, en (x, y) relativo a la traslacion
// actual (o sea lo mismo que `PushTranslation(x, y)` + `RenderIndicesTintado` + `Pop`, sin pagar la pila).
//
// Devuelve false SOLO si el lienzo no se puede escribir por palabras (buffer desalineado a 4): el llamador tiene
// que caer al camino de siempre, que hace la misma cuenta por bytes. Cuando devuelve true el tile ya esta pintado
// (o no habia nada que pintar: tile vacio o fuera del lienzo).
func (s *Surface) RenderIndicesConTabla(indices []byte, ancho, alto, x, y int, tabla *TablaDelBlit) bool {
	if tabla == nil {
		return false
	}

	if ancho <= 0 || alto <= 0 || len(indices) < ancho*alto {
		return true
	}

	lienzo := palabrasDelLienzo(s.imagen.pixeles)
	if lienzo == nil {
		avisosDeAlineacion++

		return false
	}

	dx, dy := s.actual.x+x, s.actual.y+y
	anchoLienzo, altoLienzo := s.imagen.ancho, s.imagen.alto

	x0, x1, y0, y1, vale := recorteDelBlit(dx, dy, ancho, alto, anchoLienzo, altoLienzo)
	if !vale {
		return true
	}

	tablaDelColor := &tabla.Color

	// DC-002.137 -- LOS INDICES, DE A CUATRO. El arte del minimapa son lineas finas de 8x16: de los 128 indices de
	// una celda hay ~8 que no son cero, y recorrer los 128 pixeles para escribir 8 son 78.080 vueltas por cuadro
	// (610 celdas) -- que es exactamente lo que quedo pesando despues de sacar la tabla del bucle (12,5 ms
	// medidos). Como el ancho de la celda es multiplo de 4 y la fila empieza alineada, se lee una PALABRA de
	// indices (cuatro de una vez) y si la palabra es 0 se saltan los cuatro pixeles: las mismas escrituras, un
	// cuarto de las vueltas y las filas vacias gratis. Si el buffer no se puede leer asi (desalineado o ancho no
	// multiplo de 4) queda el bucle pixel a pixel, que escribe lo mismo.
	palabrasDeIndices := palabraDelIndice135(indices, ancho)

	if palabrasDeIndices == nil {
		tilesPorBytes137++
	} else {
		tilesPorPalabras137++
	}

	for fy := y0; fy < y1; fy++ {
		fila := fy * ancho
		j := (dy+fy)*anchoLienzo + dx

		if palabrasDeIndices == nil {
			for fx := x0; fx < x1; fx++ {
				if idx := indices[fila+fx]; idx != 0 {
					escribirElPixelDeLaTabla(tablaDelColor[idx], lienzo, j+fx)
				}
			}

			continue
		}

		fx := x0

		// el borde izquierdo: los pixeles que no completan la primera palabra de la fila
		for fx < x1 && (fila+fx)%4 != 0 {
			if idx := indices[fila+fx]; idx != 0 {
				escribirElPixelDeLaTabla(tablaDelColor[idx], lienzo, j+fx)
			}

			fx++
		}

		// el cuerpo: cuatro indices por vuelta, y la palabra en cero se salta entera
		for fx+4 <= x1 {
			indicesDeLaPalabra := palabrasDeIndices[(fila+fx)/4]
			if indicesDeLaPalabra != 0 {
				for k := 0; k < 4; k++ {
					if idx := byte(indicesDeLaPalabra >> uint(8*k)); idx != 0 {
						escribirElPixelDeLaTabla(tablaDelColor[idx], lienzo, j+fx+k)
					}
				}
			}

			fx += 4
		}

		// el borde derecho
		for fx < x1 {
			if idx := indices[fila+fx]; idx != 0 {
				escribirElPixelDeLaTabla(tablaDelColor[idx], lienzo, j+fx)
			}

			fx++
		}
	}

	pxDeIndices += uint64((x1 - x0) * (y1 - y0))

	return true
}

// escribirElPixelDeLaTabla: UN pixel del tile con su palabra ya resuelta (paleta + tinte). Las tres reglas son las
// del camino de bytes, en el mismo orden: alfa 0 no pinta, alfa 255 escribe la palabra, alfa intermedio es
// source-over con el mismo cociente por 255.
func escribirElPixelDeLaTabla(v uint32, lienzo []uint32, j int) {
	a := uint16(v >> 24)
	if a == 0 {
		return
	}

	if a == 255 {
		lienzo[j] = v

		return
	}

	destino := lienzo[j]
	resto := 255 - a

	r := uint8((uint16(uint8(v))*a + uint16(uint8(destino))*resto) / 255)
	g := uint8((uint16(uint8(v>>8))*a + uint16(uint8(destino>>8))*resto) / 255)
	b := uint8((uint16(uint8(v>>16))*a + uint16(uint8(destino>>16))*resto) / 255)
	al := uint8(a + uint16(uint8(destino>>24))*resto/255)

	lienzo[j] = palabraDelColor(r, g, b, al)
}

// RenderCompactoConTabla: pinta un tile que viene en su forma COMPACTA -- solo los pixeles que pintan, como
// tercetos (x, y, indice) -- con la tabla armada por el llamador, en (x, y) relativo a la traslacion actual.
//
// POR QUE EXISTE, MEDIDO. Con la tabla afuera del bucle (`RenderIndicesConTabla`) el minimapa bajo de 61,6 a
// ~12 ms, y la parte que quedo es el BLIT: 610 celdas de 8x16 son 128 indices cada una, o sea 78.080 indices por
// cuadro para escribir **~5.200 pixeles**. Recorrer 128 indices para pintar 8 es tirar quince de cada dieciseis
// vueltas, y con los indices leidos de a cuatro sigue siendo 32 lecturas por celda. El arte del automapa es fijo
// (59 cuadros, 7.552 B, se decodifican una vez al arranque), asi que la lista de los pixeles que pintan se puede
// armar UNA vez, al cargar, y en el cuadro se recorre esa lista: ~8 vueltas por celda en vez de 128.
//
// EL FORMATO DEL TERCETO es x, y, indice (tres bytes). Con la x y la y explicitas no hace falta dividir por el
// ancho del tile para pasar de un desplazamiento a (x, y), que era el precio de empacar el desplazamiento.
//
// LAS REGLAS DEL PIXEL SON LAS DE SIEMPRE (las del camino de bytes): el indice 0 no pinta, el alfa 0 no pinta, el
// alfa 255 escribe la palabra y el alfa intermedio mezcla con el destino. El recorte es el del tile contra el
// lienzo (`recorteDelBlit`), y la lista compacta es la del tile entero: los pixeles que caen fuera de la ventana
// se saltan con las coordenadas. Los pixeles son los MISMOS que los del camino de bytes: lo prueba el verificador
// del host `console/dc/verif-blit-137` (que arma la lista compacta y compara contra el camino de bytes) y el
// `crc32` de la caja del minimapa en la consola, igual con `MINIMAPA_RAPIDO=si` y con `=no`.
func (s *Surface) RenderCompactoConTabla(compacto []byte, ancho, alto, x, y int, tabla *TablaDelBlit) bool {
	if tabla == nil {
		return false
	}

	if len(compacto) < 3 {
		return true
	}

	lienzo := palabrasDelLienzo(s.imagen.pixeles)
	if lienzo == nil {
		avisosDeAlineacion++

		return false
	}

	dx, dy := s.actual.x+x, s.actual.y+y
	anchoLienzo, altoLienzo := s.imagen.ancho, s.imagen.alto

	x0, x1, y0, y1, vale := recorteDelBlit(dx, dy, ancho, alto, anchoLienzo, altoLienzo)
	if !vale {
		return true
	}

	base := dy * anchoLienzo
	tilesCompactos137++

	for i := 0; i+2 < len(compacto); i += 3 {
		cx, cy := int(compacto[i]), int(compacto[i+1])
		if cx < x0 || cx >= x1 || cy < y0 || cy >= y1 {
			continue
		}

		idx := compacto[i+2]
		if idx == 0 {
			continue
		}

		escribirElPixelDeLaTabla(tabla.Color[idx], lienzo, base+(cy*anchoLienzo)+dx+cx)
	}

	pxDeIndices += uint64(len(compacto) / 3)

	return true
}

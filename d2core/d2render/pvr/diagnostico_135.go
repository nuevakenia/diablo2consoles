//go:build dreamcast

// DC-002.135 -- LAS PALANCAS DE DIAGNOSTICO DEL BUCLE DE PIXELES (para partir el costo por pixel).
//
// QUE PROBLEMA RESUELVEN. El cronometro del renderer del mapa (`d2core/d2map/d2maprenderer/cronometro_135.go`)
// dice cuanto del mapa se va en el bloque del PINTADO, pero no dice EN QUE se va el pintado: el bucle de un
// tile es, por pixel, (1) leer el indice de la cache, (2) mirarlo en la tabla de colores y (3) escribir la
// palabra en el lienzo. Con el tiempo total solo no se puede elegir el arreglo: si el que pesa es (1) hay que
// leer los indices de a cuatro bytes, si es (3) hay que escribir menos pixeles (o escribir en memoria con
// cache), y si lo que pesa es armar la TABLA (256 entradas por tile) hay que armarla una vez por cuadro.
//
// COMO SE PARTE, SIN ESTIMAR NADA. Son tres corridas del MISMO binario, con la palanca `BUCLE_135`:
//
//	BUCLE_135=no         el camino del juego (el A/B de referencia).
//	BUCLE_135=lectura    el bucle LEE el indice y la tabla y NO escribe el pixel: queda (1) + (2) mas el lazo.
//	BUCLE_135=escritura  el bucle NO lee el indice: escribe una palabra fija en los mismos destinos: queda (3).
//	BUCLE_135=tabla2     el bucle NORMAL y ademas arma una SEGUNDA vez la tabla de 256 palabras: la diferencia
//	                     contra "no" es exactamente lo que cuesta armar la tabla por tile.
//
// LAS TRES PRIMERAS ROMPEN EL DIBUJO A PROPOSITO (el `crc32` del cuadro 1 NO se compara en estas corridas): son
// para medir, no para el juego. `tabla2` pinta el cuadro IDENTICO (la tabla que usa es la de siempre) y sirve de
// control: si el cuadro cambia con `tabla2`, el instrumento esta tocando algo que no debe.
//
// EL SUMIDERO. Sin el, el compilador se lleva puesto el bucle de `lectura` (nadie usa lo que leyo). Sumar los
// valores en una variable del paquete es UN sumando por pixel: 1 ciclo, no cambia la conclusion.
package pvr

// diagnostico135: que corrida de diagnostico esta puesta ("no" = el camino del juego).
var diagnostico135 string

// sumidero135: donde van a parar los pixeles que el diagnostico NO escribe (ver el comentario de arriba).
var sumidero135 uint32

// FijarDiagnosticoDelBucle135 elige la corrida de diagnostico del bucle del mapa (`BUCLE_135`).
func FijarDiagnosticoDelBucle135(modo string) {
	switch modo {
	case "lectura", "escritura", "tabla2":
		diagnostico135 = modo
	default:
		diagnostico135 = ""
	}
}

// DiagnosticoDelBucle135: cual esta puesta (para el log del arranque).
func DiagnosticoDelBucle135() string {
	if diagnostico135 == "" {
		return "no"
	}

	return diagnostico135
}

// SumideroDelDiagnostico135: lo que junto el bucle de `lectura` (para que no se lo lleve el compilador y para
// saber que el bucle corrio: 0 con el bucle prendido seria el sintoma de que no se ejecuto).
func SumideroDelDiagnostico135() uint32 { return sumidero135 }

// bucleDeDiagnostico135: las tres corridas de medir. Devuelve true siempre que el llamador ya no tenga que
// pintar (el llamador lo usa para salir), igual que el resto del camino rapido.
func (s *Surface) bucleDeDiagnostico135(indices []byte, ancho, alto int, paleta [][4]uint8, tabla [256]uint32,
	x0, x1, y0, y1 int) bool {
	lienzo := palabrasDelLienzo(s.imagen.pixeles)
	if lienzo == nil {
		return false
	}

	dx, dy := s.actual.x, s.actual.y
	anchoLienzo := s.imagen.ancho

	switch diagnostico135 {
	case "lectura":
		// (1) leer el indice + (2) mirarlo en la tabla. NADA se escribe en el lienzo: el destino es el sumidero.
		for y := y0; y < y1; y++ {
			fila := y * ancho

			for x := x0; x < x1; x++ {
				sumidero135 += tabla[indices[fila+x]]
			}
		}

		pxDeIndices += uint64((x1 - x0) * (y1 - y0))

		return true

	case "escritura":
		// (3) escribir el pixel, sin leer el indice ni la tabla (la palabra se arma una vez).
		v := palabraDelColor(1, 2, 3, 255)

		for y := y0; y < y1; y++ {
			j := (dy+y)*anchoLienzo + dx + x0

			for x := x0; x < x1; x, j = x+1, j+1 {
				lienzo[j] = v
			}
		}

		pxDeIndices += uint64((x1 - x0) * (y1 - y0))

		return true
	}

	// "tabla2": la MISMA tabla, armada una SEGUNDA vez. La cuenta es la del camino normal (la de
	// `renderIndicesTintadoPalabras`), copiada tal cual para que la segunda tabla salga igual que la primera.
	rojo, verde, azul, alfa := uint16(255), uint16(255), uint16(255), uint16(255)

	if s.actual.col != nil {
		c := aRGBA(s.actual.col)
		rojo, verde, azul, alfa = uint16(c.R), uint16(c.G), uint16(c.B), uint16(c.A)
	}

	neutro := rojo == 255 && verde == 255 && azul == 255 && alfa == 255

	var tabla2 [256]uint32

	for i := 0; i < 256; i++ {
		p := paleta[i]

		if neutro {
			tabla2[i] = palabraDelColor(p[0], p[1], p[2], p[3])

			continue
		}

		tabla2[i] = palabraDelColor(uint8(uint16(p[0])*rojo/255), uint8(uint16(p[1])*verde/255),
			uint8(uint16(p[2])*azul/255), uint8(uint16(p[3])*alfa/255))
	}

	sumidero135 += tabla2[0]

	// y el bucle NORMAL, con la tabla de siempre (el cuadro sale identico: es el control de la palanca).
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

	return true
}

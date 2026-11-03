// Verificacion del cambio de DC-002.137 en el backend PVR: el blit con la TABLA ARMADA POR EL LLAMADOR
// (`ArmarTablaDelBlit` + `RenderIndicesConTabla`) tiene que escribir EXACTAMENTE los mismos bytes que el camino de
// BYTES de DC-002.132 (`RenderIndicesTintado`, la tabla por llamada y el pixel a pixel), que es el que el juego
// usaba en el minimapa hasta esta tarjeta.
//
// Por que aca y no en la consola: el paquete `d2core/d2render/pvr` no compila en el host (importa `kos`), asi que
// esta es la unica forma de probar la equivalencia sin la Dreamcast. Los dos bucles estan copiados TAL CUAL del
// backend:
//
//	referencia -- el bucle de bytes de `surface.go` (RenderIndicesTintado, camino general): la tabla se arma por
//	              llamada, el pixel se escribe de a cuatro bytes y el alfa intermedio mezcla con el destino
//	nuevo      -- el de `blit_tabla_137.go` (RenderIndicesConTabla): la tabla viene armada del llamador, el pixel
//	              se escribe con UN deposito de 32 bits y los indices se leen de a CUATRO (la palabra en cero se
//	              salta entera)
//
// LOS CASOS CUBREN LO QUE LA CONSOLA NO PUEDE CUBRIR SOLA: tiles de 1x1 a 40x40, recortes por los cuatro bordes
// (la ventana del recorte que NO empieza en multiplo de 4 es justo donde el atajo de la palabra necesita los
// bordes pixel a pixel), indices 0 (transparentes), paletas con alfa 0 / intermedio / 255, tinte neutro y no
// neutro, y el arte RALO del minimapa (8 de 128 pixeles pintados) que es el que esta tarjeta optimiza. Ademas
// corre el MISMO banco con el buffer de indices CORRIDO UN BYTE (no alineado a 4): ahi el atajo no se puede usar
// y el bucle tiene que caer al camino de bytes y dar lo mismo.
//
// Uso:  go run ./console/dc/verif-blit-137/ [semilla] [casos]
package main

import (
	"math/rand"
	"os"
	"strconv"
	"unsafe"
)

const maximoDePalabras = 1 << 28

var avisosDeAlineacion int

func palabras(pixeles []byte) []uint32 {
	if len(pixeles) < 4 {
		return nil
	}

	p := unsafe.Pointer(&pixeles[0])
	if uintptr(p)%4 != 0 {
		return nil
	}

	vista := (*[maximoDePalabras]uint32)(p)
	n := len(pixeles) / 4

	return vista[:n:n]
}

func palabraDelColor(r, g, b, a uint8) uint32 {
	return uint32(r) | uint32(g)<<8 | uint32(b)<<16 | uint32(a)<<24
}

// recorteDelBlit: el mismo recorte de una vez por tile del backend (DC-002.132).
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

// ============================== LA REFERENCIA (el camino de bytes) ==============================

func referencia(px []byte, ancho, alto, dx, dy int, indices []byte, ia, al int, paleta [][4]uint8, col []uint16) {
	rojo, verde, azul, alfa := uint16(255), uint16(255), uint16(255), uint16(255)
	if col != nil {
		rojo, verde, azul, alfa = col[0], col[1], col[2], col[3]
	}

	neutro := rojo == 255 && verde == 255 && azul == 255 && alfa == 255

	x0, x1, y0, y1, vale := recorteDelBlit(dx, dy, ia, al, ancho, alto)
	if !vale {
		return
	}

	filaLienzo := ancho * 4

	for y := y0; y < y1; y++ {
		fila := y * ia
		j := (dy+y)*filaLienzo + (dx+x0)*4

		for x := x0; x < x1; x, j = x+1, j+4 {
			idx := indices[fila+x]
			if idx == 0 {
				continue
			}

			p := paleta[idx]

			if neutro && p[3] == 255 {
				px[j+0], px[j+1], px[j+2], px[j+3] = p[0], p[1], p[2], 255

				continue
			}

			r := uint8(uint16(p[0]) * rojo / 255)
			g := uint8(uint16(p[1]) * verde / 255)
			b := uint8(uint16(p[2]) * azul / 255)
			a := uint16(uint16(p[3]) * alfa / 255)

			if a == 0 {
				continue
			}

			if a == 255 {
				px[j+0], px[j+1], px[j+2], px[j+3] = r, g, b, 255

				continue
			}

			resto := 255 - a
			px[j+0] = uint8((uint16(r)*a + uint16(px[j+0])*resto) / 255)
			px[j+1] = uint8((uint16(g)*a + uint16(px[j+1])*resto) / 255)
			px[j+2] = uint8((uint16(b)*a + uint16(px[j+2])*resto) / 255)
			px[j+3] = uint8(a + uint16(px[j+3])*resto/255)
		}
	}
}

// ============================== EL NUEVO (la tabla del llamador) ==============================

// tablaDeTinte: la copia de `ArmarTablaDelBlit` (paleta -> 256 palabras con el tinte aplicado).
func tablaDeTinte(paleta [][4]uint8, col []uint16) [256]uint32 {
	rojo, verde, azul, alfa := uint16(255), uint16(255), uint16(255), uint16(255)
	if col != nil {
		rojo, verde, azul, alfa = col[0], col[1], col[2], col[3]
	}

	neutro := rojo == 255 && verde == 255 && azul == 255 && alfa == 255

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

	return tabla
}

// palabraDelIndice: la vista de 32 bits de los indices (o nil si no se puede leer asi), como `palabraDelIndice135`.
func palabraDelIndice(indices []byte, ancho int) []uint32 {
	if ancho%4 != 0 || len(indices) < 4 {
		return nil
	}

	return palabras(indices)
}

func escribirElPixel(v uint32, lienzo []uint32, j int) {
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

// nuevo: la copia de `RenderIndicesConTabla` (posicion (dx, dy) absoluta, que en el backend es la traslacion del
// estado mas (x, y)).
func nuevo(px []byte, ancho, alto, dx, dy int, indices []byte, ia, al int, tabla *[256]uint32,
	porPalabras, porBytes *int) bool {
	lienzo := palabras(px)
	if lienzo == nil {
		avisosDeAlineacion++

		return false
	}

	x0, x1, y0, y1, vale := recorteDelBlit(dx, dy, ia, al, ancho, alto)
	if !vale {
		return true
	}

	palabrasDeIndices := palabraDelIndice(indices, ia)
	if palabrasDeIndices == nil {
		*porBytes++
	} else {
		*porPalabras++
	}

	for fy := y0; fy < y1; fy++ {
		fila := fy * ia
		j := (dy+fy)*ancho + dx

		if palabrasDeIndices == nil {
			for fx := x0; fx < x1; fx++ {
				if idx := indices[fila+fx]; idx != 0 {
					escribirElPixel(tabla[idx], lienzo, j+fx)
				}
			}

			continue
		}

		fx := x0

		for fx < x1 && (fila+fx)%4 != 0 {
			if idx := indices[fila+fx]; idx != 0 {
				escribirElPixel(tabla[idx], lienzo, j+fx)
			}

			fx++
		}

		for fx+4 <= x1 {
			indicesDeLaPalabra := palabrasDeIndices[(fila+fx)/4]
			if indicesDeLaPalabra != 0 {
				for k := 0; k < 4; k++ {
					if idx := byte(indicesDeLaPalabra >> uint(8*k)); idx != 0 {
						escribirElPixel(tabla[idx], lienzo, j+fx+k)
					}
				}
			}

			fx += 4
		}

		for fx < x1 {
			if idx := indices[fila+fx]; idx != 0 {
				escribirElPixel(tabla[idx], lienzo, j+fx)
			}

			fx++
		}
	}

	return true
}

// ============================== EL CAMINO COMPACTO ==============================

// compactar: la misma lista que arma el POC al cargar el arte (`compactarElArte`): tercetos (x, y, indice) de los
// pixeles que pintan, en orden de fila y despues de columna.
func compactar(indices []byte, ia, al int) []byte {
	var compacto []byte

	for y := 0; y < al; y++ {
		for x := 0; x < ia; x++ {
			idx := indices[y*ia+x]
			if idx == 0 {
				continue
			}

			compacto = append(compacto, byte(x), byte(y), idx)
		}
	}

	return compacto
}

// compactoBlit: la copia de `RenderCompactoConTabla`.
func compactoBlit(px []byte, ancho, alto, dx, dy int, compacto []byte, ia, al int, tabla *[256]uint32) bool {
	lienzo := palabras(px)
	if lienzo == nil {
		avisosDeAlineacion++

		return false
	}

	x0, x1, y0, y1, vale := recorteDelBlit(dx, dy, ia, al, ancho, alto)
	if !vale {
		return true
	}

	base := dy * ancho
	for i := 0; i+2 < len(compacto); i += 3 {
		cx, cy := int(compacto[i]), int(compacto[i+1])
		if cx < x0 || cx >= x1 || cy < y0 || cy >= y1 {
			continue
		}

		idx := compacto[i+2]
		if idx == 0 {
			continue
		}

		escribirElPixel(tabla[idx], lienzo, base+(cy*ancho)+dx+cx)
	}

	return true
}

// ============================== EL BANCO ==============================

func main() {
	semilla := int64(12345)
	casosPedidos := 1000

	if len(os.Args) > 1 {
		semilla, _ = strconv.ParseInt(os.Args[1], 10, 64)
	}

	if len(os.Args) > 2 {
		casosPedidos, _ = strconv.Atoi(os.Args[2])
	}

	r := rand.New(rand.NewSource(semilla))

	AN, AL := 64, 48
	casos, distintos := 0, 0
	porPalabras, porBytes, ralos, compactos := 0, 0, 0, 0

	for i := 0; i < casosPedidos; i++ {
		ia := r.Intn(41) + 1
		al := r.Intn(41) + 1

		if ia%4 != 0 {
			ia += 4 - ia%4 // el caso de la celda del minimapa: ancho multiplo de 4
		}

		// la mayoria DENTRO del lienzo (con los bordes de los cuatro lados en la cola de la ventana):
		// el recorte es justo lo que ejercita los dos bordes del bucle de a cuatro
		dx := r.Intn(AN+24) - 12
		dy := r.Intn(AL+24) - 12

		indices := make([]byte, ia*al)
		for k := range indices {
			indices[k] = 0
		}

		// TRES REPARTOS: ralo (el arte del minimapa: ~8 de 128 pixeles), medio y lleno
		reparto := r.Intn(3)
		pintados := 0

		for k := range indices {
			switch reparto {
			case 0:
				if r.Intn(16) != 0 {
					continue
				}
			case 1:
				if r.Intn(2) != 0 {
					continue
				}
			}

			indices[k] = byte(1 + r.Intn(255))
			pintados++
		}

		if reparto == 0 {
			ralos++
		}

		pal4 := make([][4]uint8, 256)

		for k := 0; k < 256; k++ {
			a := uint8(255)

			switch r.Intn(4) {
			case 0:
				a = 0
			case 1:
				a = uint8(r.Intn(255))
			}

			pal4[k] = [4]uint8{uint8(r.Intn(256)), uint8(r.Intn(256)), uint8(r.Intn(256)), a}
		}

		var col []uint16
		if r.Intn(3) == 0 {
			col = nil // el estado sin color: la tabla es la paleta tal cual (el caso del minimapa)
		} else if r.Intn(2) == 0 {
			col = []uint16{255, 255, 255, 255}
		} else {
			col = []uint16{uint16(r.Intn(256)), uint16(r.Intn(256)), uint16(r.Intn(256)), uint16(r.Intn(256))}
		}

		base := make([]byte, AN*AL*4)
		for k := range base {
			base[k] = byte(r.Intn(256))
		}

		tabla := tablaDeTinte(pal4, col)

		// 1. EL CAMINO NORMAL: la tabla del llamador contra el camino de bytes
		ref := append([]byte(nil), base...)
		nv := append([]byte(nil), base...)
		referencia(ref, AN, AL, dx, dy, indices, ia, al, pal4, col)

		nuevo(nv, AN, AL, dx, dy, indices, ia, al, &tabla, &porPalabras, &porBytes)

		casos++
		if !iguales(ref, nv) {
			distintos++
			println("TABLA DISTINTO caso", i, "dx", dx, "dy", dy, "tile", ia, al, "pintados", pintados,
				"tinte", colElegido(col))
		}

		// 2. EL MISMO BANCO CON LOS INDICES CORRIDOS UN BYTE (buffer no alineado a 4): el atajo de la palabra no se
		// puede usar y el bucle tiene que caer al camino de bytes y dar lo mismo. La ventana del recorte tampoco
		// empieza en multiplo de 4, que es lo que ejercita los dos bordes del bucle.
		corridos := make([]byte, len(indices)+1)
		copy(corridos[1:], indices)
		corridos = corridos[1:]

		ref = append([]byte(nil), base...)
		nv = append([]byte(nil), base...)
		referencia(ref, AN, AL, dx, dy, corridos, ia, al, pal4, col)

		nuevo(nv, AN, AL, dx, dy, corridos, ia, al, &tabla, &porPalabras, &porBytes)

		casos++
		if !iguales(ref, nv) {
			distintos++
			println("TABLA CORRIDA DISTINTO caso", i, "dx", dx, "dy", dy, "tile", ia, al, "tinte", colElegido(col))
		}

		// 3. EL CAMINO COMPACTO (la lista de los pixeles que pintan) contra el camino de bytes: es el que usa el
		// juego en el minimapa con `MINIMAPA_RAPIDO=si`.
		ref = append([]byte(nil), base...)
		cp := append([]byte(nil), base...)
		referencia(ref, AN, AL, dx, dy, indices, ia, al, pal4, col)

		lista := compactar(indices, ia, al)
		if compactoBlit(cp, AN, AL, dx, dy, lista, ia, al, &tabla) && len(lista) > 0 {
			compactos++
		}

		casos++
		if !iguales(ref, cp) {
			distintos++
			println("COMPACTO DISTINTO caso", i, "dx", dx, "dy", dy, "tile", ia, al, "pintados", pintados,
				"tinte", colElegido(col))
		}
	}

	println("BLIT137: casos=", casos, "distintos=", distintos, "semilla=", int(semilla),
		"porPalabras=", porPalabras, "porBytes=", porBytes, "compactos=", compactos, "ralos=", ralos,
		"avisosDeAlineacion=", avisosDeAlineacion)

	// LOS DOS CAMINOS TIENEN QUE HABERSE EJERCITADO: si el atajo de la palabra no corrio en ningun caso (o el de
	// bytes, con los indices corridos), el banco estaria dando verde sin haber probado lo que dice probar.
	if porPalabras == 0 || porBytes == 0 || compactos == 0 {
		println("BLIT137: UNO DE LOS CAMINOS NO SE EJERCITO", porPalabras, porBytes, compactos)
		distintos++
	}

	if distintos != 0 {
		os.Exit(1)
	}
}

func colElegido(col []uint16) int {
	if col == nil {
		return -1
	}

	return int(col[0])<<24 | int(col[1])<<16 | int(col[2])<<8 | int(col[3])
}

func iguales(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}

	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}

	return true
}

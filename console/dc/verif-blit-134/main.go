// Verificacion del cambio de DC-002.134 en el backend PVR: el camino de la PALABRA (un deposito de 32 bits por
// pixel con la paleta empaquetada en una tabla de 256 entradas) tiene que escribir EXACTAMENTE los mismos bytes
// que el camino de BYTES de DC-002.132, y los dos los mismos que el bucle VIEJO de DC-002.70.
//
// Por que aca y no en la consola: el paquete d2core/d2render/pvr no compila en el host (importa `kos`), asi que
// esta es la unica forma de probar la equivalencia sin la Dreamcast. Los tres bucles estan copiados TAL CUAL:
//
//	viejo    -- renderIndicesLento / renderIndicesTintadoLento (el de DC-002.70, dos comparaciones por pixel)
//	bytes    -- RenderIndices / RenderIndicesTintado de DC-002.132 (recorte afuera, camino sin tinte)
//	palabra  -- renderIndicesPalabras / renderIndicesTintadoPalabras de DC-002.134 (blit_palabras.go)
//
// Uso:  go run ./console/dc/verif-blit-134/ [semilla] [casos]
package main

import (
	"math/rand"
	"os"
	"strconv"
	"unsafe"
)

const maximoDePalabras = 1 << 28

func palabras(pixeles []byte) []uint32 {
	p := unsafe.Pointer(&pixeles[0])
	vista := (*[maximoDePalabras]uint32)(p)
	n := len(pixeles) / 4

	return vista[:n:n]
}

func palabraDelColor(r, g, b, a uint8) uint32 {
	return uint32(r) | uint32(g)<<8 | uint32(b)<<16 | uint32(a)<<24
}

// ============================== EL VIEJO (DC-002.70) ==============================

func viejoRGBA(px []byte, ancho, alto int, dx, dy int, indices []byte, ia, al int, paleta [][3]uint8) {
	anchoLienzo, altoLienzo := ancho, alto
	for y := 0; y < al; y++ {
		cy := dy + y
		if cy < 0 || cy >= altoLienzo {
			continue
		}
		base := cy * anchoLienzo * 4
		fila := y * ia
		for x := 0; x < ia; x++ {
			idx := indices[fila+x]
			if idx == 0 {
				continue
			}
			cx := dx + x
			if cx < 0 || cx >= anchoLienzo {
				continue
			}
			c := paleta[idx]
			j := base + cx*4
			px[j+0], px[j+1], px[j+2], px[j+3] = c[0], c[1], c[2], 255
		}
	}
}

func viejoTintado(px []byte, ancho, alto int, dx, dy int, indices []byte, ia, al int, paleta [][4]uint8, col []uint16) {
	anchoLienzo, altoLienzo := ancho, alto
	rojo, verde, azul, alfa := uint16(255), uint16(255), uint16(255), uint16(255)
	if col != nil {
		rojo, verde, azul, alfa = col[0], col[1], col[2], col[3]
	}
	for y := 0; y < al; y++ {
		cy := dy + y
		if cy < 0 || cy >= altoLienzo {
			continue
		}
		base := cy * anchoLienzo * 4
		fila := y * ia
		for x := 0; x < ia; x++ {
			idx := indices[fila+x]
			if idx == 0 {
				continue
			}
			cx := dx + x
			if cx < 0 || cx >= anchoLienzo {
				continue
			}
			p := paleta[idx]
			r := uint8(uint16(p[0]) * rojo / 255)
			g := uint8(uint16(p[1]) * verde / 255)
			b := uint8(uint16(p[2]) * azul / 255)
			a := uint16(uint16(p[3]) * alfa / 255)
			if a == 0 {
				continue
			}
			j := base + cx*4
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

// ============================== EL DE BYTES (DC-002.132) ==============================

func bytesRGBA(px []byte, ancho, alto int, dx, dy int, indices []byte, ia, al int, paleta [][3]uint8) {
	anchoLienzo, altoLienzo := ancho, alto
	x0, x1 := 0, ia
	if dx < 0 {
		x0 = -dx
	}
	if dx+x1 > anchoLienzo {
		x1 = anchoLienzo - dx
	}
	if x0 >= x1 {
		return
	}
	y0, y1 := 0, al
	if dy < 0 {
		y0 = -dy
	}
	if dy+y1 > altoLienzo {
		y1 = altoLienzo - dy
	}
	if y0 >= y1 {
		return
	}
	filaLienzo := anchoLienzo * 4
	for y := y0; y < y1; y++ {
		fila := y * ia
		j := (dy+y)*filaLienzo + (dx+x0)*4
		for x := x0; x < x1; x, j = x+1, j+4 {
			idx := indices[fila+x]
			if idx == 0 {
				continue
			}
			c := paleta[idx]
			px[j+0], px[j+1], px[j+2], px[j+3] = c[0], c[1], c[2], 255
		}
	}
}

func bytesTintado(px []byte, ancho, alto int, dx, dy int, indices []byte, ia, al int, paleta [][4]uint8, col []uint16) {
	anchoLienzo, altoLienzo := ancho, alto
	rojo, verde, azul, alfa := uint16(255), uint16(255), uint16(255), uint16(255)
	if col != nil {
		rojo, verde, azul, alfa = col[0], col[1], col[2], col[3]
	}
	neutro := rojo == 255 && verde == 255 && azul == 255 && alfa == 255
	x0, x1 := 0, ia
	if dx < 0 {
		x0 = -dx
	}
	if dx+x1 > anchoLienzo {
		x1 = anchoLienzo - dx
	}
	if x0 >= x1 {
		return
	}
	y0, y1 := 0, al
	if dy < 0 {
		y0 = -dy
	}
	if dy+y1 > altoLienzo {
		y1 = altoLienzo - dy
	}
	if y0 >= y1 {
		return
	}
	filaLienzo := anchoLienzo * 4
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

// ============================== EL DE PALABRAS (DC-002.134) ==============================

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

func palabraRGBA(px []byte, ancho, alto int, dx, dy int, indices []byte, ia, al int, paleta [][3]uint8) {
	lienzo := palabras(px)

	x0, x1, y0, y1, vale := recorteDelBlit(dx, dy, ia, al, ancho, alto)
	if !vale {
		return
	}

	var tabla [256]uint32
	for i := 0; i < 256; i++ {
		c := paleta[i]
		tabla[i] = palabraDelColor(c[0], c[1], c[2], 255)
	}

	for y := y0; y < y1; y++ {
		fila := y * ia
		j := (dy+y)*ancho + dx + x0
		for x := x0; x < x1; x, j = x+1, j+1 {
			idx := indices[fila+x]
			if idx == 0 {
				continue
			}
			lienzo[j] = tabla[idx]
		}
	}
}

func palabraTintado(px []byte, ancho, alto int, dx, dy int, indices []byte, ia, al int, paleta [][4]uint8, col []uint16) {
	lienzo := palabras(px)

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

	x0, x1, y0, y1, vale := recorteDelBlit(dx, dy, ia, al, ancho, alto)
	if !vale {
		return
	}

	for y := y0; y < y1; y++ {
		fila := y * ia
		j := (dy+y)*ancho + dx + x0
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
			al2 := uint8(a + uint16(uint8(destino>>24))*resto/255)
			lienzo[j] = palabraDelColor(r, g, b, al2)
		}
	}
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

	for i := 0; i < casosPedidos; i++ {
		ia := 1 + r.Intn(40)
		al := 1 + r.Intn(40)
		dx := r.Intn(AN+60) - 50
		dy := r.Intn(AL+60) - 50

		indices := make([]byte, ia*al)
		for k := range indices {
			if r.Intn(3) == 0 {
				indices[k] = 0
			} else {
				indices[k] = byte(1 + r.Intn(255))
			}
		}

		pal3 := make([][3]uint8, 256)
		pal4 := make([][4]uint8, 256)

		for k := 0; k < 256; k++ {
			pal3[k] = [3]uint8{uint8(r.Intn(256)), uint8(r.Intn(256)), uint8(r.Intn(256))}
			a := uint8(255)

			switch r.Intn(4) {
			case 0:
				a = 0
			case 1:
				a = uint8(r.Intn(255))
			}

			pal4[k] = [4]uint8{pal3[k][0], pal3[k][1], pal3[k][2], a}
		}

		base := make([]byte, AN*AL*4)
		for k := range base {
			base[k] = byte(r.Intn(256))
		}

		var col []uint16
		if r.Intn(2) == 0 {
			col = []uint16{255, 255, 255, 255}
		} else {
			col = []uint16{uint16(r.Intn(256)), uint16(r.Intn(256)), uint16(r.Intn(256)), uint16(r.Intn(256))}
		}

		// 1. RenderIndices: viejo contra bytes contra palabra
		ref := append([]byte(nil), base...)
		b132 := append([]byte(nil), base...)
		b134 := append([]byte(nil), base...)
		viejoRGBA(ref, AN, AL, dx, dy, indices, ia, al, pal3)
		bytesRGBA(b132, AN, AL, dx, dy, indices, ia, al, pal3)
		palabraRGBA(b134, AN, AL, dx, dy, indices, ia, al, pal3)

		casos++
		if !iguales(ref, b132) || !iguales(ref, b134) {
			distintos++
			println("RenderIndices DISTINTO caso", i, "dx", dx, "dy", dy, "tile", ia, al,
				"bytes=", iguales(ref, b132), "palabra=", iguales(ref, b134))
		}

		// 2. RenderIndicesTintado: viejo contra bytes contra palabra
		ref = append([]byte(nil), base...)
		b132 = append([]byte(nil), base...)
		b134 = append([]byte(nil), base...)
		viejoTintado(ref, AN, AL, dx, dy, indices, ia, al, pal4, col)
		bytesTintado(b132, AN, AL, dx, dy, indices, ia, al, pal4, col)
		palabraTintado(b134, AN, AL, dx, dy, indices, ia, al, pal4, col)

		casos++
		if !iguales(ref, b132) || !iguales(ref, b134) {
			distintos++
			println("RenderIndicesTintado DISTINTO caso", i, "dx", dx, "dy", dy, "tile", ia, al,
				"tinte", col[0], col[1], col[2], col[3], "bytes=", iguales(ref, b132), "palabra=", iguales(ref, b134))
		}
	}

	println("BLIT134: casos=", casos, "distintos=", distintos, "semilla=", int(semilla))

	if distintos != 0 {
		os.Exit(1)
	}
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

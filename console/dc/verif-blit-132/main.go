// Verificacion del cambio de DC-002.132 en el backend PVR: el camino RAPIDO de dibujado de un mapa de indices
// (pvr_surface RenderIndicesTintado / RenderIndices) tiene que escribir EXACTAMENTE los mismos bytes que el
// bucle viejo. Se corren los dos, con el mismo lienzo y el mismo estado, y se comparan.
//
// Por que aca y no en la consola: el paquete d2core/d2render/pvr no compila en el host (importa `kos`), asi que
// esta es la unica forma de probar la equivalencia sin la Dreamcast. Copia los dos bucles TAL CUAL estan en
// d2core/d2render/pvr/surface.go (viejo = renderIndices*Lento, nuevo = RenderIndices*).
package main

import (
	"math/rand"
	"os"
	"strconv"
)

type lienzo struct {
	pixeles     []byte
	ancho, alto int
	dx, dy      int
	col         []uint16 // nil = sin color; si no, r,g,b,a
}

func rgbs(px []byte, ancho, alto int, dx, dy int, indices []byte, ia, al int, paleta [][3]uint8) {
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

func rgbsNuevo(px []byte, ancho, alto int, dx, dy int, indices []byte, ia, al int, paleta [][3]uint8) {
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

func tintado(px []byte, ancho, alto int, dx, dy int, indices []byte, ia, al int, paleta [][4]uint8, col []uint16) {
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

func tintadoNuevo(px []byte, ancho, alto int, dx, dy int, indices []byte, ia, al int, paleta [][4]uint8, col []uint16) {
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

func main() {
	semilla := int64(12345)
	if len(os.Args) > 1 {
		semilla, _ = strconv.ParseInt(os.Args[1], 10, 64)
	}
	r := rand.New(rand.NewSource(semilla))

	AN, AL := 64, 48
	casos, fallos := 0, 0

	for i := 0; i < 400; i++ {
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

		var pal3 [][3]uint8 = make([][3]uint8, 256)
		var pal4 [][4]uint8 = make([][4]uint8, 256)
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

		a1 := append([]byte(nil), base...)
		a2 := append([]byte(nil), base...)
		rgbs(a1, AN, AL, dx, dy, indices, ia, al, pal3)
		rgbsNuevo(a2, AN, AL, dx, dy, indices, ia, al, pal3)
		casos++
		if !iguales(a1, a2) {
			fallos++
			println("RenderIndices DISTINTO en el caso", i, "dx", dx, "dy", dy, "tile", ia, al, "color", col[3])
		}

		b1 := append([]byte(nil), base...)
		b2 := append([]byte(nil), base...)
		tintado(b1, AN, AL, dx, dy, indices, ia, al, pal4, col)
		tintadoNuevo(b2, AN, AL, dx, dy, indices, ia, al, pal4, col)
		casos++
		if !iguales(b1, b2) {
			fallos++
			println("RenderIndicesTintado DISTINTO en el caso", i, "dx", dx, "dy", dy, "tile", ia, al,
				"tinte", col[0], col[1], col[2], col[3])
		}
	}

	println("BLIT132: casos=", casos, "distintos=", fallos, "semilla=", int(semilla))
	if fallos != 0 {
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

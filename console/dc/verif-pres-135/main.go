package main

// DC-002.135 -- EL VERIFICADOR DEL CAMINO RAPIDO DE LA PRESENTACION (en el HOST, sin emulador).
//
// QUE CONTESTA, Y POR QUE EXISTE. El camino rapido de la conversion RGBA -> RGB565 (pvr_pantalla.c) cambia
// tres cosas de la conversion de siempre: lee el lienzo de a 32 bits en vez de tres bytes, arma el color con
// mascaras en vez de con las tablas del twiddle, y escribe DOS pixeles por deposito recorriendo la textura en
// orden (con una tabla de desplazamientos, `ofs_tile`). Ademas sube los bloques en grupos de 2x2. Todo eso
// tiene que dejar EN LA TEXTURA exactamente los mismos bytes que el camino de siempre: si un solo byte
// cambia, la captura del pueblo cambia y el numero de microsegundos no vale nada.
//
// ESE "exactamente los mismos bytes" no se puede probar a ojo en el emulador (una corrida son minutos y una
// captura dice de un solo cuadro): se prueba aca, en el host, sobre la MISMA logica escrita dos veces -- el
// bucle clasico y el rapido -- con lienzos al azar, y comparando la textura entera byte a byte. Es el mismo
// patron que `verif-blit-132` y `verif-blit-134`.
//
// OJO CON EL ALCANCE, DICHO DE FRENTE: esto es una TRANSCRIPCION de los dos bucles, no el codigo de la
// consola (ese compila con gccgo y KOS). Lo que prueba es la LOGICA (la tabla de desplazamientos, el orden
// de los grupos, el RGB565); que el binario de la consola haga lo mismo se prueba con el crc32 del lienzo
// (1437022778) y con la captura comparada pixel a pixel contra la de DC-002.132.
//
// Uso:  go run ./console/dc/verif-pres-135/ <semilla>

import (
	"fmt"
	"math/rand"
	"os"
	"strconv"
)

const (
	ladoTile  = 32
	ladoGrupo = 64

	// la textura de la pantalla del POC: 1024x512 (potencia de dos, como exige el PVR)
	pantPW = 1024
	pantPH = 512
)

var (
	twidTabla [1024]uint32
	twidBits  int
	twidMask  int
	twidSalto int
)

func twidtab(x int) uint32 {
	u := uint32(x)
	return (u & 1) | ((u & 2) << 1) | ((u & 4) << 2) | ((u & 8) << 3) | ((u & 16) << 4) |
		((u & 32) << 5) | ((u & 64) << 6) | ((u & 128) << 7) | ((u & 256) << 8) | ((u & 512) << 9)
}

// armarTwiddle: la copia literal de `armar_twiddle` de pvr_pantalla.c.
func armarTwiddle() {
	for i := 0; i < 1024; i++ {
		twidTabla[i] = twidtab(i)
	}

	minimo := pantPW
	if pantPH < minimo {
		minimo = pantPH
	}

	twidBits = 0
	for (1 << twidBits) < minimo {
		twidBits++
	}
	twidMask = minimo - 1
	twidSalto = minimo * minimo
}

// indiceTile: la copia literal de `indice_tile`.
func indiceTile(x0, y0 int) uint32 {
	return (twidTabla[x0&twidMask] << 1) +
		twidTabla[y0&twidMask] +
		uint32(x0>>twidBits)*uint32(twidSalto) +
		uint32(y0>>twidBits)*uint32(twidSalto)
}

// clasico: el bucle de DC-002.132, tal cual (bloque por bloque, tres cargas de 8 bits y la tabla del twiddle).
func clasico(src []byte, w, h, xDest int) []uint16 {
	tex := make([]uint16, pantPW*pantPH)
	conv := make([]uint16, ladoTile*ladoTile)

	for y0 := 0; y0 < h; y0 += ladoTile {
		altoTile := h - y0
		if altoTile > ladoTile {
			altoTile = ladoTile
		}

		for x0 := 0; x0 < w; x0 += ladoTile {
			anchoTile := w - x0
			if anchoTile > ladoTile {
				anchoTile = ladoTile
			}

			if anchoTile != ladoTile || altoTile != ladoTile {
				for i := range conv {
					conv[i] = 0
				}
			}

			for yy := 0; yy < altoTile; yy++ {
				p := (y0+yy)*w*4 + x0*4
				fila := twidTabla[yy]

				for xx := 0; xx < anchoTile; xx++ {
					r := uint32(src[p])
					g := uint32(src[p+1])
					b := uint32(src[p+2])

					conv[(twidTabla[xx]<<1)|fila] = uint16(((r >> 3) << 11) | ((g >> 2) << 5) | (b >> 3))

					p += 4
				}
			}

			base := indiceTile(xDest+x0, y0)
			copy(tex[base:base+ladoTile*ladoTile], conv)
		}
	}

	return tex
}

// ofsTile: la copia literal de `armar_ofs_tile` (una tabla por ancho de lienzo).
func ofsTile(w int) []uint32 {
	ofs := make([]uint32, ladoTile*ladoTile)

	for i := 0; i < ladoTile*ladoTile; i++ {
		x := ((i & 0x002) >> 1) | ((i & 0x008) >> 2) | ((i & 0x020) >> 3) |
			((i & 0x080) >> 4) | ((i & 0x200) >> 5)
		y := (i & 0x001) | ((i & 0x004) >> 1) | ((i & 0x010) >> 2) |
			((i & 0x040) >> 3) | ((i & 0x100) >> 4)

		ofs[i] = uint32(y*w*4 + x*4)
	}

	return ofs
}

func color565(p uint32) uint32 {
	return ((p & 0x000000F8) << 8) | ((p & 0x0000FC00) >> 5) | ((p & 0x00F80000) >> 19)
}

// bloqueRapido: la copia literal de `bloque_rapido` (cuatro pixeles por vuelta, dos depositos de 32 bits).
// OJO: `conv` es de 32 bits (medio bloque), NO de 16: el deposito guarda DOS pixeles y truncarlo a 16 bits
// comprime el bloque a la mitad y corre todo lo que viene atras.
func bloqueRapido(src []byte, base int, conv []uint32, ofs []uint32) {
	for i := 0; i < ladoTile*ladoTile; i += 4 {
		a := lee32(src, base+int(ofs[i]))
		b := lee32(src, base+int(ofs[i+1]))
		c := lee32(src, base+int(ofs[i+2]))
		d := lee32(src, base+int(ofs[i+3]))

		conv[i>>1] = color565(a) | (color565(b) << 16)
		conv[(i>>1)+1] = color565(c) | (color565(d) << 16)
	}
}

// volcarBloque: los 512 depositos de 32 bits a la textura, que es de 16 bits (es la `pvr_txr_load`).
func volcarBloque(tex []uint16, base int, grupo []uint32, cuantos int) {
	for j := 0; j < 512*cuantos; j++ {
		tex[base+j*2] = uint16(grupo[j])
		tex[base+j*2+1] = uint16(grupo[j] >> 16)
	}
}

func lee32(b []byte, i int) uint32 {
	return uint32(b[i]) | uint32(b[i+1])<<8 | uint32(b[i+2])<<16 | uint32(b[i+3])<<24
}

// rapido: la copia literal de `convertir_rapido` (con `agrupada` = la palanca de la subida en grupos).
func rapido(src []byte, w, h, xDest int, agrupada bool) []uint16 {
	tex := make([]uint16, pantPW*pantPH)
	grupo := make([]uint32, ladoGrupo*ladoGrupo/2)

	if (w%ladoTile) != 0 || (h%ladoTile) != 0 {
		return clasico(src, w, h, xDest)
	}

	ofs := ofsTile(w)

	for y0 := 0; y0 < h; y0 += ladoGrupo {
		altoGrupo := h - y0
		if altoGrupo > ladoGrupo {
			altoGrupo = ladoGrupo
		}

		for x0 := 0; x0 < w; x0 += ladoGrupo {
			anchoGrupo := w - x0
			if anchoGrupo > ladoGrupo {
				anchoGrupo = ladoGrupo
			}

			cuantos := 1
			i0 := indiceTile(xDest+x0, y0)

			if agrupada && anchoGrupo == ladoGrupo && altoGrupo == ladoGrupo &&
				indiceTile(xDest+x0+ladoTile, y0) == i0+2*1024 &&
				indiceTile(xDest+x0, y0+ladoTile) == i0+1024 &&
				indiceTile(xDest+x0+ladoTile, y0+ladoTile) == i0+3*1024 {
				cuantos = 4
			} else if agrupada && anchoGrupo == ladoTile && altoGrupo == ladoGrupo &&
				indiceTile(xDest+x0, y0+ladoTile) == i0+1024 {
				cuantos = 2
			}

			if cuantos > 1 {
				// el orden del grupo es el de la TEXTURA (la y primero)
				for k := 0; k < cuantos; k++ {
					bx, by := 0, 0
					if cuantos == 4 && k&2 != 0 {
						bx = ladoTile
					}
					if k&1 != 0 {
						by = ladoTile
					}

					base := (y0+by)*w*4 + (x0+bx)*4
					bloqueRapido(src, base, grupo[k*512:], ofs)
				}

				volcarBloque(tex, int(i0), grupo, cuantos)
			} else {
				// bloque por bloque: los cuatro del grupo se convierten igual, cada uno a SU indice
				nbx := anchoGrupo / ladoTile

				for by := 0; by < altoGrupo; by += ladoTile {
					for bx := 0; bx < anchoGrupo; bx += ladoTile {
						k := (by/ladoTile)*nbx + (bx / ladoTile)
						bloqueRapido(src, (y0+by)*w*4+(x0+bx)*4, grupo[k*512:], ofs)

						volcarBloque(tex, int(indiceTile(xDest+x0+bx, y0+by)), grupo[k*512:], 1)
					}
				}
			}
		}
	}

	return tex
}

// lienzo: un RGBA al azar con alfas de todo tipo (0, 255 y medios) y manchas de color plano, que es como se
// ve un cuadro del POC: grandes zonas de un color y bordes con alfa.
func lienzo(r *rand.Rand, w, h int) []byte {
	px := w * h
	b := make([]byte, px*4)

	// un color de fondo por lienzo y algunas manchas, para que no sea solo ruido
	fondo := [4]byte{byte(r.Intn(256)), byte(r.Intn(256)), byte(r.Intn(256)), 255}

	for i := 0; i < px; i++ {
		b[i*4], b[i*4+1], b[i*4+2], b[i*4+3] = fondo[0], fondo[1], fondo[2], fondo[3]
	}

	for mancha := 0; mancha < 40; mancha++ {
		cx, cy := r.Intn(w), r.Intn(h)
		rw, rh := r.Intn(w/4)+1, r.Intn(h/4)+1
		c := [4]byte{byte(r.Intn(256)), byte(r.Intn(256)), byte(r.Intn(256)), byte(r.Intn(256))}

		for y := cy; y < cy+rh && y < h; y++ {
			for x := cx; x < cx+rw && x < w; x++ {
				i := (y*w + x) * 4
				b[i], b[i+1], b[i+2], b[i+3] = c[0], c[1], c[2], c[3]
			}
		}
	}

	// y una capa de ruido puro, con alfa al azar
	for i := 0; i < px/8; i++ {
		j := r.Intn(px) * 4
		b[j], b[j+1], b[j+2], b[j+3] = byte(r.Intn(256)), byte(r.Intn(256)), byte(r.Intn(256)), byte(r.Intn(256))
	}

	return b
}

// pixelDePos: que pixel del bloque pide cada posicion de la textura, buscandolo a lo bruto (para el volcado).
func pixelDePos(pos int) (int, int) {
	for yy := 0; yy < ladoTile; yy++ {
		for xx := 0; xx < ladoTile; xx++ {
			if int(twidTabla[xx])<<1|int(twidTabla[yy]) == pos {
				return xx, yy
			}
		}
	}
	return -1, -1
}

// ofsDebug: el desplazamiento (en bytes) del pixel que va en la posicion i de un bloque, para el volcado.
func ofsDebug(w, i int) int {
	x := ((i & 0x002) >> 1) | ((i & 0x008) >> 2) | ((i & 0x020) >> 3) | ((i & 0x080) >> 4) | ((i & 0x200) >> 5)
	y := (i & 0x001) | ((i & 0x004) >> 1) | ((i & 0x010) >> 2) | ((i & 0x040) >> 3) | ((i & 0x100) >> 4)

	return y*w*4 + x*4
}

func compara(a, b []uint16) (int, int) {
	distintos, primero := 0, -1
	for i := range a {
		if a[i] != b[i] {
			distintos++
			if primero < 0 {
				primero = i
			}
		}
	}

	return distintos, primero
}

func main() {
	semilla := int64(1)
	if len(os.Args) > 1 {
		if v, err := strconv.ParseInt(os.Args[1], 10, 64); err == nil {
			semilla = v
		}
	}

	volcar := len(os.Args) > 2 && os.Args[2] == "volcar"

	armarTwiddle()

	r := rand.New(rand.NewSource(semilla))

	// los tamanos que el backend tiene que saber presentar: el cuadro entero del POC, las dos mitades del split
	// de DC-002.116 (320x480), las de 320x240 que NO son multiplas de 32 (caen al bucle clasico) y tamanos
	// chicos y raros, que es donde se rompen las cuentas de borde.
	tamanos := [][2]int{
		{640, 480},
		{320, 480},
		{320, 240},
		{96, 480},
		{640, 32},
		{64, 64},
		{640, 448},
	}

	casos, distintosTotal, agrupados4, agrupados2, sueltos, casosMalos := 0, 0, 0, 0, 0, 0

	for _, t := range tamanos {
		w, h := t[0], t[1]

		// el destino en la textura: la columna 0 y la 320 del split (las dos son multiplos de 32)
		destinos := []int{0}
		if w <= 320 {
			destinos = []int{0, 320}
		}

		for _, xDest := range destinos {
			for repeticion := 0; repeticion < 6; repeticion++ {
				src := lienzo(r, w, h)

				a := clasico(src, w, h, xDest)
				b := rapido(src, w, h, xDest, true)
				c := rapido(src, w, h, xDest, false)

				if volcar && casos == 0 {
					for _, i := range []int{7, 14, 28, 29} {
						fmt.Printf("   palabra %2d clasico=%04x rapido=%04x\n", i, a[i], b[i])
					}
					for _, pos := range []int{14, 15, 28, 29} {
						o := ofsDebug(w, pos)
						fmt.Printf("   pos %2d -> ofs %6d -> pixel(%d,%d) rgba=%02x%02x%02x%02x color565=%04x\n",
							pos, o, (o/4)%w, (o/4)/w, src[o], src[o+1], src[o+2], src[o+3],
							uint16(color565(lee32(src, o))))
					}
					for _, pos := range []int{14, 15, 28, 29} {
						px, py := pixelDePos(pos)
						fmt.Printf("   pos %2d (tabla clasica) -> pixel %d,%d\n", pos, px, py)
					}
				}

				for _, par := range [][2][]uint16{{a, b}, {a, c}} {
					d, primero := compara(par[0], par[1])
					casos++
					if d != 0 {
						distintosTotal += d
						if casosMalos < 12 {
							casosMalos++

							mitad1, mitad2, rapidoCero, clasicoCero, maxIdx := 0, 0, 0, 0, 0
							for i := range par[0] {
								if par[0][i] == par[1][i] {
									continue
								}
								if i < 262144 {
									mitad1++
								} else {
									mitad2++
								}
								if par[1][i] == 0 {
									rapidoCero++
								}
								if par[0][i] == 0 {
									clasicoCero++
								}
								if i > maxIdx {
									maxIdx = i
								}
							}

							fmt.Printf("PRES135: DISTINTO lienzo=%dx%d xDest=%d agrupada=%v distintos=%d primero=%d [<262144:%d >=262144:%d rapidoCero:%d clasicoCero:%d max:%d]\n",
								w, h, xDest, &par[1] == &b, d, primero, mitad1, mitad2, rapidoCero, clasicoCero, maxIdx)
						}
					}
				}

				// el censo de grupos: cuantos van de 4 bloques, cuantos de 2 y cuantos sueltos (no es una
				// prueba, es el numero que dice que el camino agrupado se EJERCITO y no se salteo)
				if (w%ladoTile) == 0 && (h%ladoTile) == 0 {
					for y0 := 0; y0 < h; y0 += ladoGrupo {
						altoGrupo := h - y0
						if altoGrupo > ladoGrupo {
							altoGrupo = ladoGrupo
						}
						for x0 := 0; x0 < w; x0 += ladoGrupo {
							anchoGrupo := w - x0
							if anchoGrupo > ladoGrupo {
								anchoGrupo = ladoGrupo
							}
							i0 := indiceTile(xDest+x0, y0)
							switch {
							case anchoGrupo == ladoGrupo && altoGrupo == ladoGrupo &&
								indiceTile(xDest+x0+ladoTile, y0) == i0+2048 &&
								indiceTile(xDest+x0, y0+ladoTile) == i0+1024 &&
								indiceTile(xDest+x0+ladoTile, y0+ladoTile) == i0+3072:
								agrupados4++
							case anchoGrupo == ladoTile && altoGrupo == ladoGrupo &&
								indiceTile(xDest+x0, y0+ladoTile) == i0+1024:
								agrupados2++
							default:
								sueltos++
							}
						}
					}
				}
			}
		}
	}

	fmt.Printf("PRES135: casos=%d distintos=%d gruposDe4=%d gruposDe2=%d sueltos=%d semilla=%d\n",
		casos, distintosTotal, agrupados4, agrupados2, sueltos, semilla)

	if distintosTotal != 0 {
		os.Exit(1)
	}
}

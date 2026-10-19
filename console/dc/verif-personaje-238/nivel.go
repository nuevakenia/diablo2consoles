package main

// EL COMPOSITOR DEL NIVEL (DC-002.37).
//
// Este archivo es el MISMO en los dos lados: lo compila el POC de la consola
// (`console/dc/poc/acto1-disco/main.go`, en la Dreamcast) y la herramienta del host
// (`console/dc/verif-acto1-237/main.go`). No toca ni el sistema de archivos ni la pantalla: recibe las
// celdas del DS1 y los tiles ya parseados, y devuelve el LIENZO DE INDICES. Que sea el mismo archivo es lo
// que hace comparable el CRC del lienzo entre la consola y el host (se verifica con sha256 en la evidencia).
//
// LAS REGLAS DE LA COMPOSICION salen del renderer del motor, no de la intuicion
// (`d2core/d2map/d2maprenderer/renderer.go` y `tile_cache.go`):
//
//   - La proyeccion es ISOMETRICA: la celda (x,y) cae en el pixel
//     ((x-y)*32 - 80, (x+y)*16) en el espacio "orto" (viewport.go: WorldToOrtho usa tileWidth=32 y
//     tileHeight=16; renderFloor/renderWall trasladan -80 en x).
//   - El PISO se decodifica en un lienzo de `tile.Width` x abs(`tile.Height`) con el desplazamiento
//     `-min(block.Y)` (generateFloorCache), y el indice 0 del tile es TRANSPARENTE.
//   - La PARED se decodifica en un lienzo de 160 (tileSurfaceWidth) x
//     `max(abs(tile.Height), maxY-minY)` con `-minY`, y se dibuja con `YAdjust = minY + 80`
//     (generateWallCache, tileSurfaceHeight=80, blockOffsetY=32).
//   - El tile de una celda NO es "el primero que aparezca": es `GetTiles(estilo, secuencia, tipo)[azar]`,
//     o sea el azar-esimo de los candidatos de TODOS los DT1 en el orden en que se cargan. Como en la
//     consola los DT1 se leen de a UNO (el town pide 4,35 MB de DT1 y eso no entra en la RAM de la
//     consola), aca el pedido lleva `Faltan`: cuantos candidatos hay que saltear todavia.

import (
	"hash/crc32"
	"strings"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2enum"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2ds1"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2dt1"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2mpq"
)

const (
	// DC-002.48: aca iba 32 y 16, con un comentario que citaba a `viewport.go` ("WorldToOrtho usa tileWidth=32 y
	// tileHeight=16"). Es FALSO: el viewport del motor declara `tileWidth = 80` y `tileHeight = 40`
	// (d2core/d2map/d2maprenderer/viewport.go, usados por WorldToOrtho), y el renderer declara
	// `orthoTileWidth = subtilesPerTile(5) * orthoSubTileWidth(16) = 80` y
	// `orthoTileHeight = 5 * 8 = 40` (renderer.go). 32x16 es el tamaño de UN SUBTILE
	// (orthoSubTileWidth/Height), no el de la celda.
	//
	// Lo que eso costaba, medido: el tile de PISO del DT1 mide 160x128 y su diamante visible 160x79 (medido
	// con `console/dc/sonda-geom-248/diamante`), o sea que los diamantes encajan borde con borde cuando la
	// celda vecina esta a MEDIO diamante: 80 en x y 40 en y. Con 32x16 cada losa de piso se pinta 5 veces
	// encima de las vecinas (sobre-escritura 5, medido) y el nivel entero sale a 2,5x menos escala que el
	// motor: eso es lo que el ojo ve como "tiles corridos".
	ladoCeldaX      = 80 // orthoTileWidth del renderer: la x avanza 80 px por celda
	ladoCeldaY      = 40 // orthoTileHeight del renderer: la y avanza 40 px por celda
	anchoSuperficie = 160
	altoSuperficie  = 80
	offsetIso       = 80
	maxAltoTile     = 512 // tope del scratch: la pared mas alta medida tiene 384 px
)

// Pedido: un tile que el nivel PIDE. Una celda del DS1 puede pedir piso y hasta cuatro paredes.
type Pedido struct {
	CeldaX, CeldaY int
	Estilo         int32
	Secuencia      int32
	Tipo           int32
	Azar           int
	Pared          bool
	Hecho          bool // ya se compuso
	Faltan         int  // candidatos que hay que saltear antes de este (el `azar` entre todos los DT1)
}

// Nivel: la ventana isometrica del nivel y los numeros de lo que entro.
type Nivel struct {
	Ancho, Alto      int
	OrigenX, OrigenY int
	Canvas           []byte
	Scratch          []byte
	Pedidos          []Pedido

	PisosPedidos, ParedesPedidas int
	PisosCompuestos              int
	ParedesCompuestas            int
	PisosSinTile, ParedesSinTile int
	FueraDeVentana               int
	PixelesDelPiso               int
	PixelesDePared                int

	// CENSAR (DC-002.48): el instrumento que ve los "tiles corridos". Con Censar en true, `pegar` lleva la
	// cuenta de CUANTOS tiles pintan cada pixel del lienzo (Cobertura: 1 = el piso encaja borde con borde,
	// 2..5 = los tiles se pisan) y de cuantos pedidos caen DENTRO de la ventana (PisosEnVentana /
	// ParedesEnVentana), que es lo que se compara contra las celdas que el MOTOR considera visibles.
	// En la consola va APAGADO: la cobertura cuesta un uint16 por pixel del lienzo (512x512 = 512 KB), y el
	// POC no lo necesita; lo prende el verificador del host.
	Censar                           bool
	Cobertura                        []uint16
	PisosEnVentana, ParedesEnVentana int
	PixelesSobre                     int // pixeles pintados por 2 o mas tiles
	CajaSobreX0, CajaSobreY0         int // la caja de esos pixeles (x0,y0)-(x1,y1)
	CajaSobreX1, CajaSobreY1         int
	MaxCobertura                     uint16
}

// NuevoNivel: arma la ventana (centrada en la celda pedida) y los pedidos de TODAS las capas del DS1.
//
// Las capas: hasta 2 de piso y 4 de pared (el DS1 de v18 las trae asi). Un tile con Estilo=0 y
// Secuencia=0 es el vacio del formato y no se pide.
func NuevoNivel(d *d2ds1.DS1, ancho, alto, centroX, centroY int) *Nivel {
	cx := (centroX-centroY)*ladoCeldaX - offsetIso
	cy := (centroX + centroY) * ladoCeldaY

	n := &Nivel{
		Ancho:   ancho,
		Alto:    alto,
		OrigenX: cx - ancho/2,
		OrigenY: cy - alto/2,
		Canvas:  make([]byte, ancho*alto),
		Scratch: make([]byte, anchoSuperficie*maxAltoTile),
	}

	for i := 0; i < 2; i++ {
		l := d.GetFloor(i)
		if l == nil {
			continue
		}

		for y := 0; y < l.Height(); y++ {
			for x := 0; x < l.Width(); x++ {
				t := l.Tile(x, y)
				if t == nil || (t.Style == 0 && t.Sequence == 0) {
					continue
				}

				// DC-002.48: el MOTOR no dibuja un tile oculto ni uno con Prop1==0 (`renderPass1` y
				// `generateFloorCache` piden `!Hidden() && Prop1 != 0`). Aca se pedia solo Style/Seq, asi que el
				// compositor pintaba 43 tiles ocultos de este nivel que el motor saltea (medido con
				// `console/dc/sonda-geom-248`). Se pide lo mismo que el motor.
				if t.Hidden() || t.Prop1 == 0 {
					continue
				}

				n.Pedidos = append(n.Pedidos, Pedido{
					CeldaX: x, CeldaY: y, Estilo: int32(t.Style), Secuencia: int32(t.Sequence),
					Tipo: int32(d2enum.TileFloor), Azar: int(t.RandomIndex), Faltan: int(t.RandomIndex),
				})
				n.PisosPedidos++
			}
		}
	}

	for i := 0; i < 4; i++ {
		l := d.GetWall(i)
		if l == nil {
			continue
		}

		for y := 0; y < l.Height(); y++ {
			for x := 0; x < l.Width(); x++ {
				t := l.Tile(x, y)
				if t == nil || (t.Style == 0 && t.Sequence == 0) {
					continue
				}

				// DC-002.48: las mismas dos condiciones que el motor MAS `!Type.Special()` (los tiles
				// especiales son marcadores de warp/entrada: el motor no les arma cache ni los dibuja). Medido:
				// 3 paredes especiales en este nivel que el compositor pintaba igual.
				if t.Hidden() || t.Prop1 == 0 || d2enum.TileType(t.Type).Special() {
					continue
				}

				n.Pedidos = append(n.Pedidos, Pedido{
					CeldaX: x, CeldaY: y, Estilo: int32(t.Style), Secuencia: int32(t.Sequence),
					Tipo: int32(t.Type), Azar: int(t.RandomIndex), Pared: true, Faltan: int(t.RandomIndex),
				})
				n.ParedesPedidas++
			}
		}
	}

	return n
}

// grupo237: los tiles de UN DT1 que comparten (estilo, secuencia, tipo), con sus indices en orden de archivo.
//
// POR QUE NO UN MAPA CON CLAVE ARRAY (medido en la consola, no supuesto): con
// `map[[3]int32][]int32` el binario de SH-4 se cae en el PRIMER archivo que compone, con
//   Unhandled exception: PC 8c0c432a, code 1, evt 0180 / Encountered Illegal instruction
// y el PC cae DENTRO de `_main._63_7int32_7..hash..f`, que `sh-elf-nm` marca como DATO (R, .rodata):
// el runtime salta a un pedazo de .rodata y ejecuta dato. La traza del retorno (PR=8c046bda) resuelve a
// `runtime.mapassign`. Los mapas con clave ESCALAR andan (DC-002.5 los midio en la consola); el que no
// digiere el runtime es la clave de tipo ARRAY. Aca son pocos grupos por archivo (los estilos del DT1) y
// una busqueda lineal deja el orden explicito, sin mapa.
type grupo237 struct {
	estilo, secuencia, tipo int32
	indices                 []int32
}

// Alimentar: el que llama le pasa los tiles de UN DT1 (ya parseado) y aca se resuelven todos los pedidos
// que salen de ESE archivo. El orden de llamadas es el orden de la lista `Files` del DS1: es el mismo
// orden con el que el motor arma `dt1TileData`, y por eso el `azar` sale igual.
func (n *Nivel) Alimentar(tiles []d2dt1.Tile) {
	// los tiles de este archivo, agrupados por (estilo, secuencia, tipo) y en orden de archivo
	grupos := make([]grupo237, 0, len(tiles))

	for i := range tiles {
		t := &tiles[i]

		encontrado := false

		for g := range grupos {
			if grupos[g].estilo == t.Style && grupos[g].secuencia == t.Sequence && grupos[g].tipo == t.Type {
				grupos[g].indices = append(grupos[g].indices, int32(i))
				encontrado = true

				break
			}
		}

		if !encontrado {
			grupos = append(grupos, grupo237{
				estilo: t.Style, secuencia: t.Sequence, tipo: t.Type,
				indices: []int32{int32(i)},
			})
		}
	}

	var cand []int32

	for i := range n.Pedidos {
		p := &n.Pedidos[i]
		if p.Hecho {
			continue
		}

		cand = cand[:0]

		for g := range grupos {
			if grupos[g].estilo == p.Estilo && grupos[g].secuencia == p.Secuencia && grupos[g].tipo == p.Tipo {
				cand = grupos[g].indices

				break
			}
		}

		if len(cand) == 0 {
			continue
		}

		if p.Faltan >= len(cand) {
			p.Faltan -= len(cand)
			continue
		}

		t := &tiles[cand[p.Faltan]]
		p.Hecho = true

		if p.Pared {
			n.ParedesCompuestas++
			pintados := n.componerPared(p, t)
			n.PixelesDePared += pintados

			if pintados > 0 {
				n.ParedesEnVentana++
			}
		} else {
			n.PisosCompuestos++
			pintados := n.componerPiso(p, t)
			n.PixelesDelPiso += pintados

			if pintados > 0 {
				n.PisosEnVentana++
			}
		}
	}
}

// Cerrar: lo que quedo sin resolver despues de pasar TODOS los DT1 del DS1.
func (n *Nivel) Cerrar() {
	for i := range n.Pedidos {
		p := &n.Pedidos[i]
		if p.Hecho {
			continue
		}

		if p.Pared {
			n.ParedesSinTile++
		} else {
			n.PisosSinTile++
		}
	}
}

// unionIso: donde cae la esquina de la celda pedida, en pixeles de la ventana.
func (n *Nivel) unionIso(p *Pedido, yAjuste int) (int, int) {
	return (p.CeldaX-p.CeldaY)*ladoCeldaX - offsetIso - n.OrigenX,
		(p.CeldaX+p.CeldaY)*ladoCeldaY + yAjuste - n.OrigenY
}

// componerPiso: generateFloorCache del renderer, con la regla de que el indice 0 no pisa.
func (n *Nivel) componerPiso(p *Pedido, t *d2dt1.Tile) int {
	alto := int(t.Height)
	if alto < 0 {
		alto = -alto
	}

	minY := 0
	for _, b := range t.Blocks {
		if int(b.Y) < minY {
			minY = int(b.Y)
		}
	}

	return n.pegar(p, t, int(t.Width), alto, -minY, 0)
}

// componerPared: generateWallCache del renderer (tileSurfaceWidth x realHeight, YAdjust = minY + 80).
func (n *Nivel) componerPared(p *Pedido, t *d2dt1.Tile) int {
	minY := 0
	maxY := 0

	for _, b := range t.Blocks {
		if int(b.Y) < minY {
			minY = int(b.Y)
		}

		if int(b.Y)+ladoCeldaX > maxY {
			maxY = int(b.Y) + ladoCeldaX // blockOffsetY = 32 en el renderer
		}
	}

	alto := int(t.Height)
	if alto < 0 {
		alto = -alto
	}

	if real := maxY - minY; real > alto {
		alto = real
	}

	return n.pegar(p, t, anchoSuperficie, alto, -minY, minY+altoSuperficie)
}

// pegar: decodifica el tile con el decodificador del MOTOR (d2dt1.DecodeTileGfxData, el mismo que usa el
// host) y copia a la ventana solo los pixeles con indice distinto de 0. Devuelve cuantos pixeles entraron.
func (n *Nivel) pegar(p *Pedido, t *d2dt1.Tile, ancho, alto, offsetY, yAjuste int) int {
	if ancho <= 0 || alto <= 0 || alto > maxAltoTile {
		return 0
	}

	// la cobertura se pide UNA vez y solo si el que llama la quiere (el POC de la consola no). Se censa el
	// PISO y nada mas: la losa de piso es la que tiene que encajar borde con borde (su diamante mide 160x79).
	// Las paredes son sprites ALTOS (hasta 480 px) y se pisan entre si por diseño, asi que su cobertura no
	// dice nada de la geometria; lo que se mide de ellas es cuantas caen en la ventana.
	if n.Censar && n.Cobertura == nil {
		n.Cobertura = make([]uint16, n.Ancho*n.Alto)
		n.CajaSobreX0, n.CajaSobreY0 = n.Ancho, n.Alto
		n.CajaSobreX1, n.CajaSobreY1 = -1, -1
	}

	necesario := ancho * alto
	for i := 0; i < necesario; i++ {
		n.Scratch[i] = 0
	}

	d2dt1.DecodeTileGfxData(t.Blocks, &n.Scratch, int32(offsetY), int32(ancho))

	sx, sy := n.unionIso(p, yAjuste)

	entraron := 0

	for y := 0; y < alto; y++ {
		cy := sy + y
		if cy < 0 || cy >= n.Alto {
			continue
		}

		base := cy * n.Ancho
		fila := y * ancho

		for x := 0; x < ancho; x++ {
			v := n.Scratch[fila+x]
			if v == 0 {
				continue
			}

			cx := sx + x
			if cx < 0 || cx >= n.Ancho {
				continue
			}

			n.Canvas[base+cx] = v
			entraron++

			if n.Cobertura != nil && !p.Pared {
				pos := base + cx

				n.Cobertura[pos]++
				if c := n.Cobertura[pos]; c > n.MaxCobertura {
					n.MaxCobertura = c
				}

				if n.Cobertura[pos] == 2 { // el pixel pasa a estar pisado
					n.PixelesSobre++

					if cx < n.CajaSobreX0 {
						n.CajaSobreX0 = cx
					}

					if cy < n.CajaSobreY0 {
						n.CajaSobreY0 = cy
					}

					if cx > n.CajaSobreX1 {
						n.CajaSobreX1 = cx
					}

					if cy > n.CajaSobreY1 {
						n.CajaSobreY1 = cy
					}
				}
			}
		}
	}

	if entraron == 0 {
		n.FueraDeVentana++
	}

	return entraron
}

// CoberturaHistograma: cuantos pixeles del lienzo pintan EXACTAMENTE k tiles (k de 1 a MaxCobertura).
func (n *Nivel) CoberturaHistograma() map[uint16]int {
	h := map[uint16]int{}
	for _, c := range n.Cobertura {
		if c > 0 {
			h[c]++
		}
	}

	return h
}

// PixelesNoVacios: cuantos pixeles del lienzo NO son el indice 0 (el instrumento del criterio 2).
func (n *Nivel) PixelesNoVacios() int {
	c := 0
	for _, v := range n.Canvas {
		if v != 0 {
			c++
		}
	}

	return c
}

// PaletaRGB: la paleta del ACTO, que en el MPQ es `data/global/palette/act1/pal.dat` y mide EXACTAMENTE
// 768 bytes = 256 colores de 3 bytes en el orden B, G, R (medido: el `d2dat.Load` del motor lee asi el
// mismo archivo; `d2pl2.Load` lo rechaza con "se esperaban 443175 bytes, llegaron 768", o sea que no es un
// PL2). Devuelve los 256 colores en orden R, G, B.
func PaletaRGB(datos []byte) [256][3]uint8 {
	var pal [256][3]uint8

	for i := 0; i < 256; i++ {
		pal[i][0] = datos[i*3+2] // R
		pal[i][1] = datos[i*3+1] // G
		pal[i][2] = datos[i*3+0] // B
	}

	return pal
}

// PaletaARGB1555: los 256 colores que van a la VRAM. El indice 0 va TRANSPARENTE (la convencion de
// Diablo II y la de este port): en la captura, lo que no pinto ningun tile se ve negro.
func PaletaARGB1555(pal [256][3]uint8) []uint16 {
	salida := make([]uint16, 256)

	for i := 0; i < 256; i++ {
		alpha := uint16(1)
		if i == 0 {
			alpha = 0
		}

		salida[i] = alpha<<15 | uint16(pal[i][0]>>3)<<10 | uint16(pal[i][1]>>3)<<5 | uint16(pal[i][2]>>3)
	}

	return salida
}

// CRCPaleta: el CRC-32 de los 256 ARGB1555 en el orden de bytes que se sube (el bajo primero). Es el numero
// con el que se compara la paleta de la consola contra la del host.
func CRCPaleta(pal []uint16) uint32 {
	b := make([]byte, len(pal)*2)

	for i, v := range pal {
		b[i*2] = byte(v & 0xff)
		b[i*2+1] = byte(v >> 8)
	}

	return crc32.ChecksumIEEE(b)
}

// ColorDelIndice: el RGB del indice, para escribir la referencia del host. El indice 0 va negro: en la
// consola es transparente y el fondo del PVR es negro, asi que la captura tambien lo ve negro.
func ColorDelIndice(pal [256][3]uint8, i byte) (byte, byte, byte) {
	if i == 0 {
		return 0, 0, 0
	}

	return pal[i][0], pal[i][1], pal[i][2]
}

// CRC32: el hash del lienzo ENTERO de indices, que es el numero que tiene que dar igual en los dos lados.
func (n *Nivel) CRC32() uint32 {
	return crc32.ChecksumIEEE(n.Canvas)
}

// barraInvertida: el separador de path de Windows, armado por su codigo (92) en vez de escribir la barra en
// una cadena (asi este archivo no depende de como se escape una barra invertida).
const barraInvertida = string(rune(92))

// RutaDelDT1: el mismo camino que hace `MapEngine.AddDS1` + el asset manager del motor
// (d2core/d2map/d2mapengine/engine.go y d2core/d2asset/asset_manager.go): el DS1 lista su DT1 con el
// prefijo de la instalacion de Windows y con `.tg1`, y en el MPQ el archivo vive en
// `data/global/tiles/<act>/<zona>/<nombre>.dt1`. MEDIDO con la sonda `console/dc/sonda-ds1-237`: sin estas
// reglas, los ocho DT1 del town del acto 1 dan "file not found".
func RutaDelDT1(f string) string {
	f = strings.ToLower(f)
	f = strings.ReplaceAll(f, "c:", "")
	f = strings.ReplaceAll(f, ".tg1", ".dt1")

	if i := strings.LastIndex(f, "tiles"); i >= 0 {
		f = f[i+len("tiles"):]
	}

	f = strings.TrimPrefix(f, barraInvertida)
	f = strings.TrimPrefix(f, "/")

	return "data/global/tiles/" + f
}

// LeerDelMPQ: el lector del MPQ es literal con las barras invertidas; el que convierte es la cadena de carga
// del motor (`d2common/d2loader/mpq/source.go`). Medido: sin esta conversion, un DT1 ya bien normalizado
// seguia dando "file not found".
func LeerDelMPQ(m *d2mpq.MPQ, nombre string) ([]byte, error) {
	return m.ReadFile(strings.ReplaceAll(nombre, "/", barraInvertida))
}

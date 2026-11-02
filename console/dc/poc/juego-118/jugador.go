// DC-002.112 -- EL JUGADOR DENTRO DEL CUADRO DEL PUEBLO: composicion, paleta y DONDE se para.
//
// ESTE ARCHIVO ES EL MISMO EN LOS DOS LADOS: lo compila el POC de la consola
// (`console/dc/poc/juego-112`) y el medidor del host (`console/dc/medidas/render-294`). No toca el
// sistema de archivos ni la pantalla: recibe los bytes ya leidos (`Lector`), compone el ciclo con el
// compositor compartido (`personaje.go`) y devuelve (a) los cuadros en INDICES de 8 bits, (b) la paleta
// de las unidades en RGBA y (c) el pixel de la pantalla donde cae el ancla del jugador. Que sea el mismo
// archivo es lo que hace que los dos lados den el mismo crc32 (se verifica con sha256 en la evidencia).
//
// DE DONDE SALE CADA COSA (nada elegido a ojo):
//
//   - LAS CAPAS, SU CAJA Y SU ORDEN: del COF y del decodificador del motor (`d2dcc.LoadDirection`), como en
//     DC-002.38/.100. El compositor es `personaje.go`, el mismo del verificador del host.
//   - EL MODO, LA DIRECCION Y EL RELOJ: NU (neutral, la amazona parada y desarmada, `amnuhth.cof`) en la
//     direccion 0, con 128,00 ms por cuadro -- el `animdata.d2` medido en DC-002.49. Esta tarjeta NO mueve
//     al jugador (eso es DC-002.113): el ciclo se compone entero y se dibuja UN cuadro.
//   - EL ANCLA: la formula del propio motor para una entidad (`d2core/d2map/d2mapentity/player.go`,
//     `Player.Render`): el punto de la celda en la pantalla mas el `RenderOffset` en subtiles del
//     `d2vector.Position`. Una celda son 5 subtiles (`subTilesPerTile`) y el medio es el subtile 2; cada
//     subtile son 16x8 px con el corrimiento de -5 px del motor. Esa cuenta NO se reescribe aca: se pide la
//     conversion de mundo a pantalla al renderer del mapa (`mr.WorldToScreenF`, que es la que usa el motor
//     para dibujar las entidades) y se le suma el mismo corrimiento de `RenderOffset`.
//   - EL PINCEL: `RenderIndicesTintado`, que es el mismo nombre y el mismo contrato en las dos
//     implementaciones de superficie (la del PVR en la consola y la de software en el host). El indice 0 es
//     transparente, el resto se pinta con su color de la paleta de las UNIDADES (`Units/pal.dat`).
package main

import (
	"hash/crc32"
	"math"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2cof"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2dc6"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2dcc"
)

const (
	// ladoCeldaX / ladoCeldaY / offsetIso: la proyeccion del MOTOR (`viewport.go`: tileWidth = 80,
	// tileHeight = 40) y el corrimiento de -80 px que `renderFloor` le hace a la imagen del piso. Estan aca
	// porque `personaje.go` (el archivo compartido) los usa en `PixelDeLaCelda` y este POC no lleva
	// `nivel.go`: los mismos numeros, con la misma explicacion, que en `console/dc/poc/personaje-mapa/nivel.go`.
	ladoCeldaX = 80
	ladoCeldaY = 40
	offsetIso  = 80

	// Los numeros del corrimiento de una ENTIDAD a pixeles de pantalla (`d2mapentity/animated_entity.go` y
	// `player.go`): el subtile mide 16x8 px ortogonales y el compuesto se dibuja 5 px mas arriba.
	anchoSubtileJugador = 16
	altoSubtileJugador  = 8
	corrimientoYJugador = -5

	// subtileDelMedio: el jugador se para en el subtile 2 de los 0..4 de la celda (el centro del rombo).
	subtileDelMedio = 2
)

// CuadroDelJugador: un cuadro del ciclo YA compuesto (indices de 8 bits, el 0 transparente) y su caja
// respecto del ancla (los pies, que es el origen del COF).
type CuadroDelJugador struct {
	Ancho, Alto      int
	Izq, Arriba      int
	Pixeles          []byte
	PixelesNoVacios  int
	CapasDibujadas   int
	CapasSinGrafico  int
	CapasFueraDeCaja int
	CRC              uint32
}

// CicloDelJugador: la animacion de UN modo y UNA direccion, cuadro por cuadro.
type CicloDelJugador struct {
	Modo        string
	Dir         int
	MsPorCuadro int
	Cuadros     []CuadroDelJugador
}

// Lector: como se piden los bytes de un COF/DCC/DC6 en cada lado. En la consola es la Source de consola
// (`Pedir` + `Tramo.Liberar`, con el dueno explicito y los bloques devueltos al monton de C); en el host es
// `mpq.ReadFile` y el recolector de Go. El segundo valor es el que suelta el tramo: se llama en cuanto los
// pixeles estan consumidos.
type Lector func(nombre string) (datos []byte, soltar func(), err error)

// Pincel: lo minimo que hace falta de una superficie para pegar el jugador. Lo cumplen las dos
// implementaciones (`pvr.Surface` en la consola y el `Cuadro` de software del host).
type Pincel interface {
	PushTranslation(x, y int)
	Pop()
	RenderIndicesTintado(indices []byte, ancho, alto int, paleta [][4]uint8)
}

// ComponerCicloDelJugador: compone TODOS los cuadros de una direccion de un modo, por el camino del motor
// (capa por capa: COF -> ruta -> .dcc/.dc6 -> decodificador -> compositor compartido).
//
// El que llama tiene que haber instalado, si le hace falta, el arena del decodificador
// (`d2dcc.PedirDeArena`): aca no se toca esa palanca, porque en el host es nil y en la consola la pone el POC
// (DC-002.100). Los `PixelData` de los cuadros APUNTAN al archivo decodificado: por eso cada capa se consume
// (se pega en el lienzo del compuesto) antes de soltar su tramo y antes de liberar el arena.
func ComponerCicloDelJugador(cof *d2cof.COF, dir int, modo string, msPorCuadro int, leer Lector) *CicloDelJugador {
	n := cof.FramesPerDirection
	per0 := NuevoPersonaje(cof, dir, 0)

	// las capas de la direccion, con sus pixeles por cuadro
	tipos := make([]capaDelCiclo, 0, len(cof.CofLayers))

	for i := range cof.CofLayers {
		encontrada := false

		for _, ext := range []string{"dcc", "dc6"} {
			nombre := per0.RutaDeLaCapaModo(i, modo, ext)

			datos, soltar, err := leer(nombre)
			if err != nil {
				continue
			}

			c := capaDelCiclo{clave: per0.Claves[i], tipo: per0.Tipos[i]}

			if ext == "dcc" {
				dd, err := d2dcc.LoadDirection(datos, dir)
				soltar()

				if err != nil {
					println("JUEGO112: CAPA", c.clave, "DEL MODO", modo, "PARSE-FALLO:", err.Error())

					break
				}

				caja := dd.Directions[dir].Box
				c.izq, c.arriba, c.ancho, c.alto = caja.Left, caja.Top, caja.Width, caja.Height

				for f := 0; f < n && f < len(dd.Directions[dir].Frames); f++ {
					c.cuadros = append(c.cuadros, dd.Directions[dir].Frames[f].PixelData)
				}

				tipos = append(tipos, c)
				encontrada = true

				dd = nil

				break
			}

			dc, err := d2dc6.Load(datos)
			if err != nil {
				soltar()
				println("JUEGO112: CAPA", c.clave, "DEL MODO", modo, "PARSE-FALLO:", err.Error())

				break
			}

			for f := 0; f < n && f < len(dc.Frames); f++ {
				fr := dc.Frames[f]
				if f == 0 {
					c.izq, c.arriba = int(fr.OffsetX), int(fr.OffsetY)
					c.ancho, c.alto = int(fr.Width), int(fr.Height)
				}

				c.cuadros = append(c.cuadros, dc.DecodeFrame(f))
			}

			soltar()

			tipos = append(tipos, c)
			encontrada = true

			dc = nil

			break
		}

		if !encontrada {
			println("JUEGO112: CAPA", per0.Claves[i], "SIN-GRAFICO modo=", modo, "dir=", dir)
		}
	}

	ciclo := &CicloDelJugador{Modo: modo, Dir: dir, MsPorCuadro: msPorCuadro}

	for f := 0; f < n; f++ {
		per := NuevoPersonaje(cof, dir, f)

		for i := range tipos {
			if f >= len(tipos[i].cuadros) {
				continue
			}

			per.Agregar(tipos[i].tipo, tipos[i].clave, tipos[i].izq, tipos[i].arriba,
				tipos[i].ancho, tipos[i].alto, tipos[i].cuadros[f])
		}

		per.Componer()

		ciclo.Cuadros = append(ciclo.Cuadros, CuadroDelJugador{
			Ancho: per.Ancho, Alto: per.Alto, Izq: per.Izq, Arriba: per.Arriba,
			Pixeles: per.Lienzo, PixelesNoVacios: per.PixelesNoVacios,
			CapasDibujadas: per.CapasDibujadas, CapasSinGrafico: per.CapasSinGrafico,
			CapasFueraDeCaja: per.CapasFueraDeCaja, CRC: per.CRC32(),
		})
	}

	return ciclo
}

// capaDelCiclo: una capa del COF con todos los cuadros de la direccion. Es el tipo que junta la
// decodificacion con la composicion cuadro a cuadro (el mismo camino que `cargarCicloCon` de DC-002.100).
type capaDelCiclo struct {
	tipo, izq, arriba, ancho, alto int
	clave                          string
	cuadros                        [][]byte
}

// PaletaDelJugador: los 768 B de `Units/pal.dat` (R,G,B por indice, como los guarda el juego) a la paleta
// RGBA que pide el pincel. Todos los colores van OPACOS: en Diablo II el unico transparente es el indice 0,
// y el pincel lo saltea sin mirar la paleta.
func PaletaDelJugador(datos []byte) ([256][4]uint8, bool) {
	var pal [256][4]uint8

	if len(datos) < 768 {
		return pal, false
	}

	for i := 0; i < 256; i++ {
		pal[i][0], pal[i][1], pal[i][2], pal[i][3] = datos[i*3+0], datos[i*3+1], datos[i*3+2], 255
	}

	return pal, true
}

// AnclaDelJugador: el pixel de la pantalla donde se para el jugador, con la cuenta del MOTOR.
//
// `mundoAPantalla` es la conversion del renderer del mapa (`mr.WorldToScreenF`, mundo en CELDAS). La posicion
// del jugador se toma en el subtile del medio de la celda, o sea `subTileOffset` = (2,2) en 0..4, y el
// `RenderOffset` del motor suma 1 subtile a cada eje; de ahi salen los 16x8 px por subtile y el -5 px del
// compuesto. Con todo eso el ancla cae en el CENTRO del rombo de la celda (medido: celda (28,20) en una
// pantalla de 640x480 centrada en ella da (320, 283), que es el medio del rombo).
func AnclaDelJugador(mundoAPantalla func(x, y float64) (float64, float64), xCelda, yCelda float64) (int, int) {
	// RenderOffset = SubTileOffset + 1 (d2vector/position.go)
	rox := float64(subtileDelMedio + 1)
	roy := float64(subtileDelMedio + 1)

	px, py := mundoAPantalla(xCelda, yCelda)

	return int(math.Floor(px + (rox-roy)*anchoSubtileJugador)),
		int(math.Floor(py + (rox+roy)*altoSubtileJugador + corrimientoYJugador))
}

// CajaDelJugador: la caja que el jugador ocupa en la pantalla (x0,y0)-(x1,y1) sin incluir los bordes, con el
// ancla en los pies. Es lo que se compara entre los dos lados y lo que se usa para devolver el mapa.
type CajaDelJugador struct {
	X0, Y0, X1, Y1 int
}

func CajaDe(anclaX, anclaY int, cu *CuadroDelJugador) CajaDelJugador {
	return CajaDelJugador{
		X0: anclaX + cu.Izq, Y0: anclaY + cu.Arriba,
		X1: anclaX + cu.Izq + cu.Ancho, Y1: anclaY + cu.Arriba + cu.Alto,
	}
}

// Ancho / Alto: el tamano de la caja.
func (c CajaDelJugador) Ancho() int { return c.X1 - c.X0 }
func (c CajaDelJugador) Alto() int  { return c.Y1 - c.Y0 }

// PintarJugador: pega el cuadro compuesto en la superficie, con la caja del compuesto corrida al ancla (o sea
// los pies del jugador en `anclaX, anclaY`). Devuelve la caja que ocupo.
func PintarJugador(p Pincel, cu *CuadroDelJugador, anclaX, anclaY int, pal [][4]uint8) CajaDelJugador {
	caja := CajaDe(anclaX, anclaY, cu)

	p.PushTranslation(caja.X0, caja.Y0)
	p.RenderIndicesTintado(cu.Pixeles, cu.Ancho, cu.Alto, pal)
	p.Pop()

	return caja
}

// CRCDelCuadro: el hash de los indices compuestos. Es el numero que tiene que dar igual en la consola y en el
// host (el mismo criterio de DC-002.38/100 con los ciclos del heroe).
func CRCDelCuadro(cu *CuadroDelJugador) uint32 {
	if cu == nil {
		return 0
	}

	return crc32.ChecksumIEEE(cu.Pixeles)
}

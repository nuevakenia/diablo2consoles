// verif-personaje-238: LA MITAD DE HOST del hito del personaje en el mapa (DC-002.38).
//
// Hace EXACTAMENTE lo mismo que el POC de la consola `console/dc/poc/personaje-mapa`, con los mismos dos
// archivos compartidos (`nivel.go`, el compositor del nivel de DC-002.37, y `personaje.go`, el compuesto del
// personaje): se compara el sha256 de los dos en la evidencia.
//
// La cadena que mide, con los decodificadores del MOTOR:
//
//	1. el .ds1 del nivel y sus DT1 -> la ventana isometrica del acto 1 (nivel.go);
//	2. el COF del personaje (data\global\CHARS\AM\COF\amnuhth.cof) -> las capas y su ORDEN DE DIBUJO;
//	3. cada capa contra el MPQ de los personajes: <base>/<token>/<capa>/<token><capa><valor><modo><arma>.dcc
//	   y, si no esta, .dc6 (el orden del motor: composite.go, layerPaths) -> d2dcc/d2dc6 (los decodificadores
//	   del motor) -> el COMPUESTO en indices de 8 bits;
//	4. la posicion del motor (d2vector.Position, subtiles) -> el pixel de la ventana (la proyeccion del piso);
//	5. la pantalla de 640x480 que tiene que pintar la consola: negro, el mapa, y el personaje ENCIMA
//	   (el orden de dibujo). Escribe DOS PNG: con personaje y sin personaje, y cuenta los pixeles que el
//	   personaje cambia (el numero del criterio 1, medido en el host).
//
// Uso:
//
//	go run ./console/dc/verif-personaje-238 -png /tmp/dc238/referencia.png -png-mapa /tmp/dc238/mapa.png
package main

import (
	"flag"
	"fmt"
	"hash/crc32"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2cof"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2dcc"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2dc6"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2ds1"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2dt1"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2mpq"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2math/d2vector"
)

const (
	ds1Defecto  = "data\\global\\tiles\\ACT1\\TOWN\\townN1.ds1"
	palDefecto  = "data/global/palette/ACT1/pal.dat"
	palUnidades = "data/global/palette/Units/pal.dat"
	cofDefecto  = "data\\global\\CHARS\\AM\\COF\\amnuhth.cof"
	ventDefecto = 512
	// el lado de la pantalla logica del POC y donde se pega la ventana del nivel (centrada):
	//   x = (640-512)/2 = 64, y = (480-512)/2 = -16  (las 16 filas de arriba y de abajo quedan fuera)
	pantallaAncho, pantallaAlto = 640, 480
	mapaX, mapaY                = (pantallaAncho - ventDefecto) / 2, (pantallaAlto - ventDefecto) / 2
	dirDelPersonaje             = 0
	frameDelPersonaje           = 0
)

func autodetectarD2() string {
	if d := os.Getenv("D2_DIR"); d != "" {
		return d
	}

	if st, err := os.Stat(filepath.Join(os.Getenv("HOME"), ".wine-d2", "drive_c", "Games", "Diablo II",
		"d2data.mpq")); err == nil && !st.IsDir() {
		return filepath.Join(os.Getenv("HOME"), ".wine-d2", "drive_c", "Games", "Diablo II")
	}

	return ""
}

// paleta: 768 bytes B,G,R -> 256 colores R,G,B.
func paleta(datos []byte) [256][3]uint8 {
	var pal [256][3]uint8
	for i := 0; i < 256 && i*3+2 < len(datos); i++ {
		pal[i] = [3]uint8{datos[i*3+2], datos[i*3+1], datos[i*3+0]}
	}

	return pal
}

func main() {
	mpqPath := flag.String("mpq", "", "d2data.mpq (por defecto: la instalacion del juego)")
	mpqChars := flag.String("mpq-chars", "", "d2char.mpq, el de los personajes (por defecto: junto a d2data)")
	ds1Name := flag.String("ds1", ds1Defecto, "el .ds1 del nivel")
	palName := flag.String("pal", palDefecto, "la paleta del ACTO dentro de d2data.mpq")
	palChars := flag.String("pal-chars", palUnidades, "la paleta de las UNIDADES dentro de d2data.mpq")
	cofName := flag.String("cof", cofDefecto, "el COF del personaje dentro de d2char.mpq")
	ventana := flag.Int("ventana", ventDefecto, "lado de la ventana isometrica")
	centro := flag.String("centro", "", "celda que queda en el centro, como x,y")
	pos := flag.String("pos", "", "posicion del personaje en CELDAS, como x,y (por defecto: la celda del centro)")
	pngPath := flag.String("png", "", "si se pasa, escribe la pantalla de 640x480 CON el personaje")
	pngMapa := flag.String("png-mapa", "", "si se pasa, escribe la misma pantalla SIN el personaje")
	flag.Parse()

	ruta := *mpqPath

	if ruta == "" {
		carpeta := autodetectarD2()
		if carpeta == "" {
			fmt.Fprintln(os.Stderr, "no encuentro la instalacion de Diablo II (usa -mpq o D2_DIR)")
			os.Exit(1)
		}

		ruta = filepath.Join(carpeta, "d2data.mpq")

		if *mpqChars == "" {
			*mpqChars = filepath.Join(carpeta, "d2char.mpq")
		}
	}

	m, err := d2mpq.FromFile(ruta)
	if err != nil {
		fmt.Fprintln(os.Stderr, "no pude abrir el MPQ:", err)
		os.Exit(1)
	}
	defer m.Close()

	mc, err := d2mpq.FromFile(*mpqChars)
	if err != nil {
		fmt.Fprintln(os.Stderr, "no pude abrir el MPQ de los personajes:", err)
		os.Exit(1)
	}
	defer mc.Close()

	// ============================== 1. el nivel ==============================
	raw, err := m.ReadFile(*ds1Name)
	if err != nil {
		fmt.Fprintln(os.Stderr, "VERIF238: DS1 FALLO:", err)
		os.Exit(1)
	}

	fmt.Printf("VERIF238: DS1 %s bytes=%d crc32=%d\n", *ds1Name, len(raw), crc32.ChecksumIEEE(raw))

	d, err := d2ds1.Unmarshal(raw)
	if err != nil {
		fmt.Fprintln(os.Stderr, "VERIF238: DS1 PARSE-FALLO:", err)
		os.Exit(1)
	}

	fmt.Printf("VERIF238: DS1 version=%d act=%d w=%d h=%d objetos=%d files=%d\n",
		d.Version(), d.Act, d.Width(), d.Height(), len(d.Objects), len(d.Files))

	cx, cy := d.Width()/2, d.Height()/2
	if *centro != "" {
		if _, err := fmt.Sscanf(*centro, "%d,%d", &cx, &cy); err != nil {
			fmt.Fprintln(os.Stderr, "VERIF238: -centro tiene que ser x,y:", err)
			os.Exit(2)
		}
	}

	n := NuevoNivel(d, *ventana, *ventana, cx, cy)

	fmt.Printf("VERIF238: VENTANA lado=%d origen=%d,%d centroCelda=%d,%d\n",
		n.Ancho, n.OrigenX, n.OrigenY, cx, cy)
	fmt.Printf("VERIF238: PEDIDOS pisos=%d paredes=%d total=%d\n",
		n.PisosPedidos, n.ParedesPedidas, len(n.Pedidos))

	for _, f := range d.Files {
		nombre := RutaDelDT1(f)

		rawDT1, err := LeerDelMPQ(m, nombre)
		if err != nil {
			fmt.Printf("VERIF238: DT1 %s FALLO: %v\n", nombre, err)
			continue
		}

		dt1, err := d2dt1.LoadDT1(rawDT1)
		if err != nil {
			fmt.Printf("VERIF238: DT1 %s bytes=%d PARSE-FALLO: %v\n", nombre, len(rawDT1), err)
			continue
		}

		fmt.Printf("VERIF238: DT1 %s bytes=%d crc32=%d tiles=%d\n",
			nombre, len(rawDT1), crc32.ChecksumIEEE(rawDT1), len(dt1.Tiles))

		n.Alimentar(dt1.Tiles)
	}

	n.Cerrar()

	fmt.Printf("VERIF238: COMPUESTO pisos=%d/%d paredes=%d/%d sintilePiso=%d sintilePared=%d fuera=%d\n",
		n.PisosCompuestos, n.PisosPedidos, n.ParedesCompuestas, n.ParedesPedidas,
		n.PisosSinTile, n.ParedesSinTile, n.FueraDeVentana)
	fmt.Printf("VERIF238: LIENZO pixelesNoVacios=%d de %d crc32=%d\n",
		n.PixelesNoVacios(), n.Ancho*n.Alto, n.CRC32())
	fmt.Printf("VERIF238: LIENZO pixelesPiso=%d pixelesPared=%d\n", n.PixelesDelPiso, n.PixelesDePared)

	palActo := paleta(mustLeer(m, *palName))
	fmt.Printf("VERIF238: PALETA acto bytes=768 colores=256 crc32=%d\n",
		CRCPaleta(PaletaARGB1555(PaletaRGB(mustLeer(m, *palName)))))

	palUni := paleta(mustLeer(m, *palChars))

	// ============================== 2. el personaje ==============================
	rawCof, err := mc.ReadFile(*cofName)
	if err != nil {
		fmt.Fprintln(os.Stderr, "VERIF238: COF FALLO:", err)
		os.Exit(1)
	}

	cof, err := d2cof.Unmarshal(rawCof)
	if err != nil {
		fmt.Fprintln(os.Stderr, "VERIF238: COF PARSE-FALLO:", err)
		os.Exit(1)
	}

	fmt.Printf("VERIF238: COF %s bytes=%d crc32=%d capas=%d framesPorDir=%d dirs=%d speed=%d\n",
		*cofName, len(rawCof), crc32.ChecksumIEEE(rawCof), cof.NumberOfLayers,
		cof.FramesPerDirection, cof.NumberOfDirections, cof.Speed)

	per := NuevoPersonaje(cof, dirDelPersonaje, frameDelPersonaje)
	fmt.Printf("VERIF238: ORDEN capas=%d orden=%v\n", len(per.Claves), per.Orden)

	for i := range cof.CofLayers {
		encontrada := false

		for _, ext := range []string{"dcc", "dc6"} {
			nombre := per.RutaDeLaCapa(i, ext)
			crudo, err := mc.ReadFile(strings.TrimPrefix(strings.ReplaceAll(nombre, "/", `\`), `\`))
			if err != nil {
				continue
			}

			if ext == "dcc" {
				dd, err := d2dcc.Load(crudo)
				if err != nil {
					fmt.Printf("VERIF238: CAPA %s %s PARSE-FALLO: %v\n", per.Claves[i], nombre, err)
					break
				}

				f := dd.Directions[dirDelPersonaje].Box
				pix := dd.Directions[dirDelPersonaje].Frames[frameDelPersonaje].PixelData

				fmt.Printf("VERIF238: CAPA %s bytes=%d crc32=%d caja=%d,%d %dx%d pixeles=%d\n",
					per.Claves[i], len(crudo), crc32.ChecksumIEEE(crudo),
					f.Left, f.Top, f.Width, f.Height, noCero(pix))

				per.Agregar(per.Tipos[i], per.Claves[i], f.Left, f.Top, f.Width, f.Height, pix)
				encontrada = true

				break
			}

			dc, err := d2dc6.Load(crudo)
			if err != nil {
				fmt.Printf("VERIF238: CAPA %s %s PARSE-FALLO: %v\n", per.Claves[i], nombre, err)
				break
			}

			fr := dc.Frames[frameDelPersonaje]
			pix := dc.DecodeFrame(frameDelPersonaje)

			fmt.Printf("VERIF238: CAPA %s bytes=%d crc32=%d caja=%d,%d %dx%d pixeles=%d\n",
				per.Claves[i], len(crudo), crc32.ChecksumIEEE(crudo),
				fr.OffsetX, fr.OffsetY, fr.Width, fr.Height, noCero(pix))

			per.Agregar(per.Tipos[i], per.Claves[i], int(fr.OffsetX), int(fr.OffsetY),
				int(fr.Width), int(fr.Height), pix)
			encontrada = true

			break
		}

		if !encontrada {
			fmt.Printf("VERIF238: CAPA %s SIN-GRAFICO\n", per.Claves[i])
		}
	}

	per.Componer()

	fmt.Printf("VERIF238: PERSONAJE compuesto=%dx%d esquina=%d,%d capasDibujadas=%d sinGrafico=%d "+
		"pixelesNoVacios=%d crc32=%d\n",
		per.Ancho, per.Alto, per.Izq, per.Arriba, per.CapasDibujadas, per.CapasSinGrafico,
		per.PixelesNoVacios, per.CRC32())

	// ============================== 3. el movimiento ==============================
	est := NuevoEstado(float64(cx), float64(cy), false)

	if *pos != "" {
		var px, py float64
		if _, err := fmt.Sscanf(*pos, "%g,%g", &px, &py); err != nil {
			fmt.Fprintln(os.Stderr, "VERIF238: -pos tiene que ser x,y:", err)
			os.Exit(2)
		}

		est.Pos = d2vector.NewPosition(px*subtilesPorCelda, py*subtilesPorCelda)
	}

	celdaX, celdaY := est.Celda()
	tileX, tileY := est.CeldaEntera()
	anclaX, anclaY := PixelDeLaCelda(celdaX, celdaY, n.OrigenX, n.OrigenY)

	fmt.Printf("VERIF238: MOTOR velocidad=subtilesPorSegundo=%.4f celdasPorSegundo=%.4f\n",
		est.Velocidad, est.Velocidad/subtilesPorCelda)
	fmt.Printf("VERIF238: POS subtiles=%.4f,%.4f celda=%.4f,%.4f tile=%d,%d ancla=%d,%d pantalla=%d,%d\n",
		est.Pos.X(), est.Pos.Y(), celdaX, celdaY, tileX, tileY, anclaX, anclaY,
		mapaX+anclaX, mapaY+anclaY)

	// ============================== 4. la pantalla ==============================
	pantallaCon := make([]byte, pantallaAncho*pantallaAlto*4)
	pantallaSin := make([]byte, pantallaAncho*pantallaAlto*4)

	// el mapa: indice -> color de la paleta del ACTO (el indice 0 es negro, que es el fondo)
	dibujarMapa(pantallaSin, n, palActo)
	copy(pantallaCon, pantallaSin)

	// el personaje ENCIMA, en el orden del COF (el compuesto ya viene en ese orden)
	dibujarPersonaje(pantallaCon, per, palUni, mapaX+anclaX+per.Izq, mapaY+anclaY+per.Arriba)

	cambiados := 0

	for i := 0; i < len(pantallaCon); i += 4 {
		if pantallaCon[i] != pantallaSin[i] || pantallaCon[i+1] != pantallaSin[i+1] ||
			pantallaCon[i+2] != pantallaSin[i+2] {
			cambiados++
		}
	}

	fmt.Printf("VERIF238: PANTALLA %dx%d personaje_en=%d,%d personajeCaja=%dx%d pixelesCambiados=%d\n",
		pantallaAncho, pantallaAlto, mapaX+anclaX+per.Izq, mapaY+anclaY+per.Arriba,
		per.Ancho, per.Alto, cambiados)

	if *pngMapa != "" {
		escribirPNG(*pngMapa, pantallaSin, pantallaAncho, pantallaAlto)
		fmt.Printf("VERIF238: PNG-MAPA %s\n", *pngMapa)
	}

	if *pngPath != "" {
		escribirPNG(*pngPath, pantallaCon, pantallaAncho, pantallaAlto)
		fmt.Printf("VERIF238: PNG %s\n", *pngPath)
	}
}

func mustLeer(m *d2mpq.MPQ, nombre string) []byte {
	b, err := LeerDelMPQ(m, nombre)
	if err != nil {
		fmt.Fprintln(os.Stderr, "VERIF238: no pude leer", nombre, ":", err)
		os.Exit(1)
	}

	return b
}

func noCero(pix []byte) int {
	c := 0
	for _, v := range pix {
		if v != 0 {
			c++
		}
	}

	return c
}

// dibujarMapa: el lienzo de indices del nivel -> pixeles RGBA de la pantalla, en (mapaX, mapaY).
func dibujarMapa(destino []byte, n *Nivel, pal [256][3]uint8) {
	for y := 0; y < n.Alto; y++ {
		py := mapaY + y
		if py < 0 || py >= pantallaAlto {
			continue
		}

		for x := 0; x < n.Ancho; x++ {
			px := mapaX + x
			if px < 0 || px >= pantallaAncho {
				continue
			}

			i := n.Canvas[y*n.Ancho+x]
			r, g, b := ColorDelIndice(pal, i)

			d := (py*pantallaAncho + px) * 4
			destino[d], destino[d+1], destino[d+2], destino[d+3] = r, g, b, 255
		}
	}
}

// dibujarPersonaje: el compuesto del personaje -> pixeles RGBA, con el indice 0 TRANSPARENTE.
func dibujarPersonaje(destino []byte, p *Personaje, pal [256][3]uint8, ox, oy int) {
	for y := 0; y < p.Alto; y++ {
		py := oy + y
		if py < 0 || py >= pantallaAlto {
			continue
		}

		for x := 0; x < p.Ancho; x++ {
			px := ox + x
			if px < 0 || px >= pantallaAncho {
				continue
			}

			idx := p.Lienzo[y*p.Ancho+x]
			if idx == 0 {
				continue
			}

			d := (py*pantallaAncho + px) * 4
			destino[d], destino[d+1], destino[d+2], destino[d+3] = pal[idx][0], pal[idx][1], pal[idx][2], 255
		}
	}
}

func escribirPNG(ruta string, pixeles []byte, ancho, alto int) {
	img := image.NewRGBA(image.Rect(0, 0, ancho, alto))

	for y := 0; y < alto; y++ {
		for x := 0; x < ancho; x++ {
			d := (y*ancho + x) * 4
			img.SetRGBA(x, y, color.RGBA{R: pixeles[d], G: pixeles[d+1], B: pixeles[d+2], A: 255})
		}
	}

	f, err := os.Create(ruta)
	if err != nil {
		fmt.Fprintln(os.Stderr, "VERIF238: PNG FALLO:", err)
		return
	}
	defer f.Close()

	_ = png.Encode(f, img)
}

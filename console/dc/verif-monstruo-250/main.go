// verif-monstruo-250: LA MITAD DE HOST del hito de GOLPEAR ENEMIGOS (DC-002.50).
//
// Hace lo mismo que el POC de la consola `console/dc/poc/personaje-mapa` con los MISMOS archivos compartidos
// (`nivel.go`, `personaje.go` y `monstruo.go`: se compara el sha256 de los tres en la evidencia) y con los
// MISMOS decodificadores del motor. Los numeros que imprime son la referencia contra la que se mide la consola:
//
//  1. LA VIDA del bicho y LA SECUENCIA DE GOLPES (los dos numeros del criterio 2, calculados por monstruo.go
//     con las tablas del MPQ y el generador del motor);
//  2. LOS CUADROS del bicho: para cada modo (NU parado, GH golpeado, DT muriendo, DD muerto), la caja, los
//     pixeles y el CRC-32 de cada cuadro compuesto -- el criterio 3 compara estos numeros con los que imprime
//     la consola (si son los mismos, la consola compuso el mismo dibujo);
//  3. LA PANTALLA: un PNG con el mapa, el personaje y el bicho encima, y la caja del bicho en la pantalla
//     (el criterio 1: donde aparece y de que tamano).
//
// Uso:
//
//	go run ./console/dc/verif-monstruo-250 -png /tmp/dc250/host
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

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2animdata"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2cof"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2dc6"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2dcc"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2ds1"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2dt1"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2mpq"
)

const (
	ds1Defecto  = `data\global\tiles\ACT1\TOWN\townN1.ds1`
	ds1Real     = "data\\global\\tiles\\ACT1\\TOWN\\townN1.ds1"
	palDefecto  = "data/global/palette/ACT1/pal.dat"
	palUnidades = "data/global/palette/Units/pal.dat"
	cofHeroe    = `data\global\CHARS\AM\COF\amnuhth.cof`

	pantallaAncho, pantallaAlto = 640, 480
	ventana                     = 512 // la MISMA del POC y la de DC-002.37/238/249
	mapaX, mapaY                = (pantallaAncho - ventana) / 2, (pantallaAlto - ventana) / 2
	centroX, centroY            = 28, 20

	// el bicho se para una celda al sur-este del personaje: en la proyeccion isometrica eso son +32 px abajo,
	// y con el sprite del Returned (chico) no tapa al heroe.
	celdaBestiaX, celdaBestiaY = centroX + 1, centroY + 1

	dirDelPersonaje = 0
	// el bicho tiene 8 direcciones; la 0 es la que se dibuja en la prueba (la misma que usa el POC).
	dirDeLaBestia = 0

	// EL RITMO del personaje en el pueblo (DC-002.49, medido): TN 16 cuadros a 128 ms, TW 8 a 40 ms.
	modoHeroeQuieto = "TN"
	frameHeroe      = 0
)

// capaBestia: una capa del COF del bicho con TODOS los cuadros de una direccion.
type capaBestia struct {
	tipo, izq, arriba, ancho, alto int
	clave                          string
	cuadros                        [][]byte
}

// origenX/origenY: donde cayo la ventana del nivel (lo imprime el compositor del nivel).
var origenX, origenY int

func main() {
	mpqPath := flag.String("mpq", "", "d2data.mpq (por defecto, la instalacion del juego)")
	mpqPatch := flag.String("mpq-patch", "", "patch_D2.mpq (el del MonLvl.txt; por defecto, junto a d2data)")
	mpqChars := flag.String("mpq-chars", "", "d2char.mpq (el del heroe; por defecto, junto a d2data)")
	pngDir := flag.String("png", "", "si se pasa, escribe un PNG por modo con el mapa, el heroe y el bicho")
	modos := flag.String("modos", "NU,GH,DT,DD", "modos del bicho cuyo ciclo se censa")
	flag.Parse()

	carpeta := autodetectarD2()
	if *mpqPath == "" {
		if carpeta == "" {
			fmt.Fprintln(os.Stderr, "no encuentro la instalacion de Diablo II (usa -mpq o D2_DIR)")
			os.Exit(1)
		}

		*mpqPath = filepath.Join(carpeta, "d2data.mpq")
	}

	if *mpqPatch == "" {
		*mpqPatch = filepath.Join(filepath.Dir(*mpqPath), "patch_D2.mpq")
	}

	if *mpqChars == "" {
		*mpqChars = filepath.Join(filepath.Dir(*mpqPath), "d2char.mpq")
	}

	m := abrirMPQ(*mpqPath)
	defer m.Close()

	mc := abrirMPQ(*mpqChars)
	defer mc.Close()

	// ============================== 1. el nivel y las paletas ==============================
	raw := mustLeer(m, ds1Real)

	d, err := d2ds1.Unmarshal(raw)
	if err != nil {
		fmt.Fprintln(os.Stderr, "VERIF250: DS1 PARSE-FALLO:", err)
		os.Exit(1)
	}

	n := NuevoNivel(d, ventana, ventana, centroX, centroY)
	origenX, origenY = n.OrigenX, n.OrigenY

	for _, f := range d.Files {
		nombre := RutaDelDT1(f)

		rawDT1, err := LeerDelMPQ(m, nombre)
		if err != nil {
			continue
		}

		dt1, err := d2dt1.LoadDT1(rawDT1)
		if err != nil {
			continue
		}

		n.Alimentar(dt1.Tiles)
	}

	n.Cerrar()

	fmt.Printf("VERIF250: NIVEL ds1=%d bytes crc32=%d ventana=%dx%d origen=%d,%d lienzo crc32=%d pixeles=%d\n",
		len(raw), crc32.ChecksumIEEE(raw), n.Ancho, n.Alto, n.OrigenX, n.OrigenY, n.CRC32(), n.PixelesNoVacios())

	palActo := paleta(mustLeer(m, palDefecto))
	palUni := paleta(mustLeer(m, palUnidades))

	// ============================== 2. la vida y el golpe (criterio 2) ==============================
	monstats := mustLeer(m, `data\global\excel\monstats.txt`)
	monlvl := mustLeerDe(*mpqPatch, `data\global\excel\MonLvl.txt`)

	vida, err := VidaDelBicho(monstats, monlvl, claseBestia, semillaCombate)
	if err != nil {
		fmt.Fprintln(os.Stderr, "VERIF250: VIDA FALLO:", err)
		os.Exit(1)
	}

	fmt.Printf("VERIF250: MONSTATS bytes=%d crc32=%d\n", len(monstats), crc32.ChecksumIEEE(monstats))
	fmt.Printf("VERIF250: MONLVL bytes=%d crc32=%d\n", len(monlvl), crc32.ChecksumIEEE(monlvl))
	fmt.Printf("VERIF250: VIDA clase=%s nivel=%d min=%d max=%d HP=%d AC=%d TH=%d A1=%d..%d\n",
		vida.Clase, vida.Nivel, vida.Min, vida.Max, vida.HP, vida.AC, vida.TH, vida.DanoMin, vida.DanoMax)

	combate := NuevoCombate(vida, semillaCombate)

	for i := 1; i <= maxGolpes; i++ {
		dano, antes, despues, muerto := combate.GolpeDelHeroe()
		fmt.Printf("VERIF250: GOLPE %d dano=%d antes=%d despues=%d muerto=%v\n", i, dano, antes, despues, muerto)

		if muerto || dano == 0 {
			break
		}
	}

	// ============================== 3. las animaciones del bicho (criterio 3) ==============================
	animdata := mustLeer(m, `data\global\animdata.d2`)

	anim, err := d2animdata.Load(animdata)
	if err != nil {
		fmt.Fprintln(os.Stderr, "VERIF250: ANIMDATA FALLO:", err)
		os.Exit(1)
	}

	// el heroe quieto (TN) para la pantalla del criterio 1
	perHeroe := cargarHeroe(mc, modoHeroeQuieto, frameHeroe)

	var pantallaBase []byte
	if *pngDir != "" {
		if err := os.MkdirAll(*pngDir, 0o755); err != nil {
			fmt.Fprintln(os.Stderr, "VERIF250: carpeta PNG:", err)
			os.Exit(1)
		}

		pantallaBase = make([]byte, pantallaAncho*pantallaAlto*4)
		dibujarMapa(pantallaBase, n, palActo)
	}

	for _, modo := range strings.Split(*modos, ",") {
		modo = strings.ToUpper(strings.TrimSpace(modo))
		if modo == "" {
			continue
		}

		censarModo(m, anim, modo, palUni, pantallaBase, perHeroe, *pngDir)
	}
}

// censarModo: el censo de UN modo del bicho: el COF, el ritmo del animdata y cada cuadro compuesto.
func censarModo(m *d2mpq.MPQ, anim *d2animdata.AnimationData, modo string, pal [256][3]uint8,
	pantallaBase []byte, perHeroe *Personaje, pngDir string) {
	crudo, err := m.ReadFile(RutaCOFBestia(modo, "HTH"))
	if err != nil || len(crudo) == 0 {
		fmt.Printf("VERIF250: MODO %s SIN-COF (%v)\n", modo, err)

		return
	}

	cof, err := d2cof.Unmarshal(crudo)
	if err != nil {
		fmt.Printf("VERIF250: MODO %s COF-ILEGIBLE: %v\n", modo, err)

		return
	}

	// el ritmo: el del animdata (es el que usa el motor); si no esta, el del COF.
	fps, fuente := cof.FPS(), "COF"

	if rec := anim.GetRecord(tokenBestia + modo + "HTH"); rec != nil && rec.FPS() > 0 {
		fps, fuente = rec.FPS(), "animdata"
	}

	msPorCuadro := int(1000.0/fps + 0.5)

	fmt.Printf("VERIF250: ANIM modo=%s bytes=%d crc32=%d capas=%d cuadros=%d dirs=%d fps=%.2f [%s] msPorCuadro=%d cicloMs=%d\n",
		modo, len(crudo), crc32.ChecksumIEEE(crudo), len(cof.CofLayers), cof.FramesPerDirection,
		cof.NumberOfDirections, fps, fuente, msPorCuadro, cof.FramesPerDirection*msPorCuadro)

	// las capas de la direccion que se dibuja
	per0 := NuevoPersonaje(cof, dirDeLaBestia, 0)
	capas := make([]capaBestia, 0, len(cof.CofLayers))

	for i := range cof.CofLayers {
		for _, ext := range []string{"dcc", "dc6"} {
			nombre := per0.RutaDeLaCapaBestia(i, modo, ext)

			crudoCapa, err := LeerDelMPQ(m, nombre)
			if err != nil {
				continue
			}

			c := capaBestia{tipo: per0.Tipos[i], clave: per0.Claves[i]}

			if ext == "dcc" {
				dd, err := d2dcc.LoadDirection(crudoCapa, dirDeLaBestia)
				if err != nil {
					fmt.Printf("VERIF250: CAPA %s modo=%s PARSE-FALLO: %v\n", c.clave, modo, err)

					break
				}

				caja := dd.Directions[dirDeLaBestia].Box
				c.izq, c.arriba, c.ancho, c.alto = caja.Left, caja.Top, caja.Width, caja.Height

				for f := 0; f < cof.FramesPerDirection && f < len(dd.Directions[dirDeLaBestia].Frames); f++ {
					c.cuadros = append(c.cuadros, dd.Directions[dirDeLaBestia].Frames[f].PixelData)
				}

				capas = append(capas, c)

				break
			}

			dc, err := d2dc6.Load(crudoCapa)
			if err != nil {
				fmt.Printf("VERIF250: CAPA %s modo=%s PARSE-FALLO: %v\n", c.clave, modo, err)

				break
			}

			for f := 0; f < cof.FramesPerDirection && f < len(dc.Frames); f++ {
				fr := dc.Frames[f]
				if f == 0 {
					c.izq, c.arriba = int(fr.OffsetX), int(fr.OffsetY)
					c.ancho, c.alto = int(fr.Width), int(fr.Height)
				}

				c.cuadros = append(c.cuadros, dc.DecodeFrame(f))
			}

			capas = append(capas, c)

			break
		}
	}

	// el compuesto de cada cuadro, con el compositor COMPARTIDO (personaje.go)
	compuestos := make([]*Personaje, 0, cof.FramesPerDirection)

	for f := 0; f < cof.FramesPerDirection; f++ {
		per := NuevoPersonaje(cof, dirDeLaBestia, f)

		for i := range capas {
			if f >= len(capas[i].cuadros) {
				continue
			}

			per.Agregar(capas[i].tipo, capas[i].clave, capas[i].izq, capas[i].arriba,
				capas[i].ancho, capas[i].alto, capas[i].cuadros[f])
		}

		per.Componer()
		compuestos = append(compuestos, per)

		fmt.Printf("VERIF250: CUADRO modo=%s f=%d caja=%dx%d esquina=%d,%d pixeles=%d crc32=%d\n",
			modo, f, per.Ancho, per.Alto, per.Izq, per.Arriba, per.PixelesNoVacios, per.CRC32())
	}

	// cuantos cuadros son DIBUJOS DISTINTOS y en cuantos pixeles cambia el compuesto del cuadro f al f+1
	distintos := contarDistintos(compuestos)

	for f := 1; f < len(compuestos); f++ {
		cambios, caja := difieren(compuestos[f-1], compuestos[f])
		fmt.Printf("VERIF250: CAMBIO modo=%s de=%d a=%d pixeles=%d caja=%d,%d %dx%d\n",
			modo, f-1, f, cambios, caja[0], caja[1], caja[2], caja[3])
	}

	fmt.Printf("VERIF250: DISTINTOS modo=%s cuadros=%d distintos=%d repetidos=%d\n",
		modo, len(compuestos), distintos, len(compuestos)-distintos)

	if pngDir == "" || len(compuestos) == 0 {
		return
	}

	// la pantalla: mapa + HEROE + BICHO, cuadro 0 y el cuadro del medio, con la caja del bicho
	escribirPantalla(filepath.Join(pngDir, "pantalla-"+modo+"-f0.png"), pantallaBase, perHeroe, compuestos[0], pal)
	medio := len(compuestos) / 2
	escribirPantalla(filepath.Join(pngDir, "pantalla-"+modo+fmt.Sprintf("-f%d.png", medio)), pantallaBase, perHeroe, compuestos[medio], pal)
}

// escribirPantalla: la pantalla de 640x480 con el mapa, el heroe (a la derecha del centro) y el bicho encima,
// y la linea con la caja del bicho en la pantalla (el numero del criterio 1).
func escribirPantalla(ruta string, base []byte, perHeroe, bestia *Personaje, pal [256][3]uint8) {
	pantalla := make([]byte, len(base))
	copy(pantalla, base)

	// el heroe en la celda del centro
	axH, ayH := PixelDeLaCelda(float64(centroX), float64(centroY), origenX, origenY)
	dibujarPersonaje(pantalla, perHeroe, pal, mapaX+axH+perHeroe.Izq, mapaY+ayH+perHeroe.Arriba)

	// el bicho, una celda al sur-este
	axB, ayB := PixelDeLaCelda(float64(celdaBestiaX), float64(celdaBestiaY), origenX, origenY)
	px, py := mapaX+axB+bestia.Izq, mapaY+ayB+bestia.Arriba
	dibujarPersonaje(pantalla, bestia, pal, px, py)

	escribirPNG(ruta, pantalla)

	fmt.Printf("VERIF250: PANTALLA %s heroe=%d,%d %dx%d bestia=%d,%d %dx%d\n",
		filepath.Base(ruta), mapaX+axH+perHeroe.Izq, mapaY+ayH+perHeroe.Arriba, perHeroe.Ancho, perHeroe.Alto,
		px, py, bestia.Ancho, bestia.Alto)
}

// cargarHeroe: el compuesto del heroe en un modo y cuadro (para la pantalla del criterio 1).
func cargarHeroe(mc *d2mpq.MPQ, modo string, frame int) *Personaje {
	crudo, err := mc.ReadFile(`data\global\chars\AM\COF\am` + strings.ToLower(modo) + `hth.cof`)
	if err != nil {
		crudo, err = mc.ReadFile(`data\global\chars\AM\COF\AM` + modo + `HTH.COF`)
	}

	if err != nil {
		return &Personaje{}
	}

	cof, err := d2cof.Unmarshal(crudo)
	if err != nil {
		return &Personaje{}
	}

	per := NuevoPersonaje(cof, dirDelPersonaje, frame)

	for i := range cof.CofLayers {
		for _, ext := range []string{"dcc", "dc6"} {
			nombre := per.RutaDeLaCapaModo(i, modo, ext)

			crudoCapa, err := LeerDelMPQ(mc, nombre)
			if err != nil {
				continue
			}

			if ext == "dcc" {
				dd, err := d2dcc.Load(crudoCapa)
				if err != nil {
					break
				}

				caja := dd.Directions[dirDelPersonaje].Box
				pix := dd.Directions[dirDelPersonaje].Frames[frame].PixelData

				per.Agregar(per.Tipos[i], per.Claves[i], caja.Left, caja.Top, caja.Width, caja.Height, pix)

				break
			}

			dc, err := d2dc6.Load(crudoCapa)
			if err != nil {
				break
			}

			fr := dc.Frames[frame]

			per.Agregar(per.Tipos[i], per.Claves[i], int(fr.OffsetX), int(fr.OffsetY),
				int(fr.Width), int(fr.Height), dc.DecodeFrame(frame))

			break
		}
	}

	per.Componer()

	return per
}

// contarDistintos: cuantos cuadros son dibujos distintos de todos los anteriores.
func contarDistintos(compuestos []*Personaje) int {
	distintos := 0

	for i := range compuestos {
		repetido := false

		for j := 0; j < i; j++ {
			if compuestos[i].CRC32() == compuestos[j].CRC32() {
				if d, _ := difieren(compuestos[i], compuestos[j]); d == 0 {
					repetido = true

					break
				}
			}
		}

		if !repetido {
			distintos++
		}
	}

	return distintos
}

// difieren: cuantos pixeles cambian entre dos compuestos y la caja donde cambian.
func difieren(a, b *Personaje) (int, [4]int) {
	if a.Ancho != b.Ancho || a.Alto != b.Alto {
		return -1, [4]int{0, 0, 0, 0}
	}

	n := 0

	x0, y0, x1, y1 := a.Ancho, a.Alto, -1, -1

	for y := 0; y < a.Alto; y++ {
		for x := 0; x < a.Ancho; x++ {
			i := y*a.Ancho + x
			if a.Lienzo[i] == b.Lienzo[i] {
				continue
			}

			n++

			if x < x0 {
				x0 = x
			}

			if y < y0 {
				y0 = y
			}

			if x > x1 {
				x1 = x
			}

			if y > y1 {
				y1 = y
			}
		}
	}

	if n == 0 {
		return 0, [4]int{0, 0, 0, 0}
	}

	return n, [4]int{x0, y0, x1 - x0 + 1, y1 - y0 + 1}
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

// dibujarPersonaje: el compuesto -> pixeles RGBA, con el indice 0 TRANSPARENTE.
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

func escribirPNG(ruta string, pixeles []byte) {
	img := image.NewRGBA(image.Rect(0, 0, pantallaAncho, pantallaAlto))

	for y := 0; y < pantallaAlto; y++ {
		for x := 0; x < pantallaAncho; x++ {
			d := (y*pantallaAncho + x) * 4
			img.SetRGBA(x, y, color.RGBA{R: pixeles[d], G: pixeles[d+1], B: pixeles[d+2], A: 255})
		}
	}

	f, err := os.Create(ruta)
	if err != nil {
		fmt.Fprintln(os.Stderr, "VERIF250: PNG FALLO:", err)

		return
	}
	defer f.Close()

	_ = png.Encode(f, img)
}

// paleta: 768 bytes B,G,R -> 256 colores R,G,B.
func paleta(datos []byte) [256][3]uint8 {
	var pal [256][3]uint8

	for i := 0; i < 256 && i*3+2 < len(datos); i++ {
		pal[i] = [3]uint8{datos[i*3+2], datos[i*3+1], datos[i*3+0]}
	}

	return pal
}

func autodetectarD2() string {
	if d := os.Getenv("D2_DIR"); d != "" {
		return d
	}

	candidata := filepath.Join(os.Getenv("HOME"), ".wine-d2", "drive_c", "Games", "Diablo II")

	if st, err := os.Stat(filepath.Join(candidata, "d2data.mpq")); err == nil && !st.IsDir() {
		return candidata
	}

	return ""
}

func abrirMPQ(ruta string) *d2mpq.MPQ {
	m, err := d2mpq.FromFile(ruta)
	if err != nil {
		fmt.Fprintln(os.Stderr, "VERIF250: MPQ", ruta, ":", err)
		os.Exit(1)
	}

	return m
}

func mustLeer(m *d2mpq.MPQ, nombre string) []byte {
	b, err := LeerDelMPQ(m, nombre)
	if err != nil {
		fmt.Fprintln(os.Stderr, "VERIF250: no pude leer", nombre, ":", err)
		os.Exit(1)
	}

	return b
}

func mustLeerDe(rutaMPQ, nombre string) []byte {
	m := abrirMPQ(rutaMPQ)
	defer m.Close()

	return mustLeer(m, nombre)
}

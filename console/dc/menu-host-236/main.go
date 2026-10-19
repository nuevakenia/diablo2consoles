//menu-host-236: instrumento del HOST para DC-002.36 (el MENU PRINCIPAL en la Dreamcast).
//
// Tres modos, todos miden (nada de "parece que anda"):
//   1. -extraer: saca del MPQ del juego cada archivo del menu (los DC6, las tablas .tbl de las fuentes,
//      las paletas), lo deja CRUDO en un directorio con su tamano y CRC-32, y saca las CADENAS que el
//      menu muestra (expansionstring.tbl) a un archivo de texto. Eso es lo que viaja al disco.
//   2. -sonda: mide los DC6 (frames, medidas, offsets, indices de paleta usados).
//   3. -componer: arma la pantalla del MENU PRINCIPAL con el CODIGO DEL MOTOR (d2dc6 + d2dat + d2font +
//      las posiciones de d2gamescreen/main_menu.go) y escribe:
//        - plan.txt: el plan de dibujo (que textura, que pedazo, en que posicion, con que color) que la
//          CONSOLA ejecuta con el PVR. Las dos mitades comparten estos numeros, que es lo que permite
//          comparar la captura contra la referencia sin ojos.
//        - referencia-800.png (el lienzo logico del motor) y referencia-640.png (lo que la consola
//          dibuja, 800x600 escalado 4/5 a los 640x480 del PVR).
//        - los numeros: CRC de cada lienzo de indices, cantidad de quads, pixeles que no son fondo y
//          cuantos pixeles cambia la SELECCION (lo que el corpus busca en la captura).
//
// POR QUE EL PLAN LO ARMA EL HOST Y NO LA CONSOLA (decision de la tarjeta): la posicion de cada cosa y
// el frame que le toca a cada boton son codigo del motor (main_menu.go + d2ui/button.go + d2ui/label.go:
// RenderSegmented 4x3 del fondo, botones de 2 trozos, alineacion de las etiquetas). Ese codigo NO corre
// todavia en la consola (el arranque del motor muere en el monton de C, DC-002.41): aca se ejecuta en el
// host y su resultado viaja como DATOS. La consola hace lo que no se puede delegar: lee los DC6 del
// disco, los decodifica con el decodificador del motor, sube las texturas al PVR, pone 3 paletas por
// hardware y dibuja los quads con transparencia; y la NAVEGACION (que boton esta apretado) corre alla.
package main

import (
	"bytes"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2dat"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2dc6"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2font"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2mpq"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2tbl"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2locale"
)

// El lienzo LOGICO del motor: 800x600 (d2app: "The interface lives in an 800x600 column", y las
// constantes del menu llegan a x=795 e y=580). La consola dibuja en 640x480, asi que todo se escala 4/5.
const (
	logAncho, logAlto = 800, 600
	panAncho, panAlto = 640, 480
)

func main() {
	var (
		mpqDir  = flag.String("mpqdir", "", "directorio del juego (los .mpq)")
		outDir  = flag.String("out", "", "directorio de salida de los archivos crudos")
		menuDir = flag.String("menu", "", "directorio con los archivos del menu")
		probe   = flag.Bool("sonda", false, "solo medir los DC6 de -menu")
		componer = flag.Bool("componer", false, "componer la pantalla del menu (referencia + plan.txt)")
		foco    = flag.Int("foco", 0, "indice del boton seleccionado al componer (0..6, -1 = ninguno)")
	)
	flag.Parse()

	var err error

	switch {
	case *mpqDir != "":
		err = extraer(*mpqDir, *outDir)
	case *menuDir != "" && *componer:
		err = componerMenu(*menuDir, *foco)
	case *menuDir != "":
		err = sonda(*menuDir, *probe)
	default:
		flag.Usage()
		return
	}

	if err != nil {
		fmt.Println("ERROR:", err)
		os.Exit(1)
	}
}

// ---------------------------------------------------------------------------------------------
// 1. EXTRAER
// ---------------------------------------------------------------------------------------------

type asset struct {
	nombre string // nombre en el disco (/MENU/<NOMBRE>)
	mpq    string // archivo MPQ del juego
	ruta   string // ruta DENTRO del MPQ, como la escribe el juego
}

// La lista la fija el inventario de la tarjeta (evidencia dc-002.36a), archivo por archivo: el fondo
// vive en d2exp.mpq y todo lo demas en d2data.mpq. Las fuentes son DOS archivos cada una: la tabla de
// glifos (.tbl, firma "Woo!\x01") y el atlas (.dc6).
var catalogo = []asset{
	{"FONDO.DC6", "d2exp.mpq", `data\global\ui\FrontEnd\gameselectscreenEXP.dc6`},
	{"LOGO_NEGRO_IZQ.DC6", "d2data.mpq", `data\global\ui\FrontEnd\D2logoBlackLeft.DC6`},
	{"LOGO_NEGRO_DER.DC6", "d2data.mpq", `data\global\ui\FrontEnd\D2logoBlackRight.DC6`},
	{"LOGO_FUEGO_IZQ.DC6", "d2data.mpq", `data\global\ui\FrontEnd\D2logoFireLeft.DC6`},
	{"LOGO_FUEGO_DER.DC6", "d2data.mpq", `data\global\ui\FrontEnd\D2logoFireRight.DC6`},
	{"BOTON_ANCHO.DC6", "d2data.mpq", `data\global\ui\FrontEnd\WideButtonBlank.dc6`},
	{"BOTON_CORTO.DC6", "d2data.mpq", `data\global\ui\CharSelect\ShortButtonBlank.dc6`},
	{"FUENTE_EXOCET10.DC6", "d2data.mpq", `data\local\FONT\LATIN\fontexocet10.dc6`},
	{"FUENTE_EXOCET10.TBL", "d2data.mpq", `data\local\FONT\LATIN\fontexocet10.tbl`},
	{"FUENTE_FORMAL12.DC6", "d2data.mpq", `data\local\FONT\LATIN\fontformal12.dc6`},
	{"FUENTE_FORMAL12.TBL", "d2data.mpq", `data\local\FONT\LATIN\fontformal12.tbl`},
	{"FUENTE_FORMAL10.DC6", "d2data.mpq", `data\local\FONT\LATIN\fontformal10.dc6`},
	{"FUENTE_FORMAL10.TBL", "d2data.mpq", `data\local\FONT\LATIN\fontformal10.tbl`},
	{"FUENTE_RIDICULOUS.DC6", "d2data.mpq", `data\local\FONT\LATIN\fontridiculous.dc6`},
	{"FUENTE_RIDICULOUS.TBL", "d2data.mpq", `data\local\FONT\LATIN\fontridiculous.tbl`},
	{"PAL_SKY.PAL", "d2data.mpq", `data\global\palette\sky\pal.dat`},
	{"PAL_UNITS.PAL", "d2data.mpq", `data\global\palette\units\pal.dat`},
	{"PAL_STATIC.PAL", "d2data.mpq", `data\global\palette\static\pal.dat`},
}

// cadenas: los textos del menu NO son literales del codigo: salen de las tablas de texto del juego
// (d2enum.numeric_labels -> "#1613" etc.). Se extraen aca con el decodificador del motor y se escriben
// como texto plano: la consola no tiene el Huffman de las .tbl, y esto es prosa, no pixeles.
type cadena struct {
	clave string
	label string
}

var cadenasMenu = []cadena{
	{"#1613", "copyright"},
	{"#1614", "copyright2"},
	{"#1620", "singleplayer"},
	{"#1623", "multiplayer"},
	{"#1625", "exit"},
	{"#1627", "credits"},
	{"#1639", "cinematics"},
}

func extraer(mpqDir, outDir string) error {
	if outDir == "" {
		return fmt.Errorf("falta -out")
	}

	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}

	abiertos := map[string]*d2mpq.MPQ{}
	defer func() {
		for _, m := range abiertos {
			m.Close()
		}
	}()

	abrir := func(nombre string) (*d2mpq.MPQ, error) {
		if m, ok := abiertos[nombre]; ok {
			return m, nil
		}

		ruta := buscanombre(mpqDir, nombre)
		if ruta == "" {
			return nil, fmt.Errorf("no encuentro %s en %s", nombre, mpqDir)
		}

		m, err := d2mpq.FromFile(ruta)
		if err != nil {
			return nil, fmt.Errorf("abrir %s: %w", ruta, err)
		}

		abiertos[nombre] = m

		return m, nil
	}

	total := 0

	for _, a := range catalogo {
		m, err := abrir(a.mpq)
		if err != nil {
			return err
		}

		datos, err := m.ReadFile(a.ruta)
		if err != nil {
			fmt.Printf("ASSET %-22s FALTA en %-11s (%s)\n", a.nombre, a.mpq, err)

			continue
		}

		if err := os.WriteFile(filepath.Join(outDir, a.nombre), datos, 0o644); err != nil {
			return err
		}

		total += len(datos)
		fmt.Printf("ASSET %-22s mpq=%-11s bytes=%8d crc32=%10d\n", a.nombre, a.mpq, len(datos), crc32IEEE(datos))
	}

	fmt.Printf("ASSET total bytes=%d archivos=%d\n", total, len(catalogo))

	// Las cadenas: se prueban las tres tablas del juego en el orden del motor (patch, expansion,
	// string) y gana la primera que tenga la clave.
	var lineas []string

	for _, tabla := range []struct{ mpq, ruta string }{
		{"patch_d2.mpq", `data\local\lng\ENG\patchstring.tbl`},
		{"d2exp.mpq", `data\local\lng\ENG\expansionstring.tbl`},
		{"d2data.mpq", `data\local\lng\ENG\string.tbl`},
	} {
		m, err := abrir(tabla.mpq)
		if err != nil {
			fmt.Println("CADENA tabla", tabla.ruta, "no se puede abrir:", err)

			continue
		}

		datos, err := m.ReadFile(tabla.ruta)
		if err != nil {
			fmt.Println("CADENA tabla", tabla.ruta, "no esta:", err)

			continue
		}

		dicc, err := d2tbl.LoadTextDictionaryDecoded(datos, d2locale.Locale{}.Decode)
		if err != nil {
			fmt.Println("CADENA tabla", tabla.ruta, "no se decodifica:", err)

			continue
		}

		fmt.Printf("CADENA tabla %-45s bytes=%7d entradas=%d crc32=%d\n",
			tabla.ruta, len(datos), len(dicc), crc32IEEE(datos))

		for _, c := range cadenasMenu {
			if v, ok := dicc[c.clave]; ok {
				lineas = append(lineas, c.label+"\t"+v)
			}
		}
	}

	// Las cadenas que el codigo del motor escribe LITERALES (no salen de ninguna tabla): van al mismo
	// archivo, marcadas, para que la consola no tenga texto escondido en el binario.
	lineas = append(lineas,
		"version\tOpenDiablo2 - DC-002",
		"commit\tDC-002.36",
		"website\tPROJECT WEBSITE",
		"maptest\tMAP ENGINE TEST",
		"od2\tOpenDiablo2 is neither developed by, nor endorsed by Blizzard or its parent company Activision",
	)

	if err := os.WriteFile(filepath.Join(outDir, "CADENAS.TXT"), []byte(strings.Join(lineas, "\n")+"\n"), 0o644); err != nil {
		return err
	}

	fmt.Println("CADENA archivo CADENAS.TXT lineas=", len(lineas))

	for _, l := range lineas {
		fmt.Println("CADENA", strings.ReplaceAll(l, "\t", " = "))
	}

	return nil
}

func buscanombre(dir, buscado string) string {
	entradas, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}

	for _, e := range entradas {
		if !e.IsDir() && igualesSinCaja(e.Name(), buscado) {
			return filepath.Join(dir, e.Name())
		}
	}

	return ""
}

func igualesSinCaja(a, b string) bool {
	if len(a) != len(b) {
		return false
	}

	for i := 0; i < len(a); i++ {
		ca, cb := a[i], b[i]
		if 'A' <= ca && ca <= 'Z' {
			ca += 'a' - 'A'
		}

		if 'A' <= cb && cb <= 'Z' {
			cb += 'a' - 'A'
		}

		if ca != cb {
			return false
		}
	}

	return true
}

func crc32IEEE(b []byte) uint32 {
	var crc uint32 = 0xffffffff

	for _, v := range b {
		crc ^= uint32(v)
		for i := 0; i < 8; i++ {
			if crc&1 != 0 {
				crc = crc>>1 ^ 0xedb88320
			} else {
				crc >>= 1
			}
		}
	}

	return ^crc
}

// ---------------------------------------------------------------------------------------------
// 2. SONDA
// ---------------------------------------------------------------------------------------------

func sonda(dir string, solo bool) error {
	nombres := []string{"FONDO.DC6", "LOGO_NEGRO_IZQ.DC6", "LOGO_NEGRO_DER.DC6", "LOGO_FUEGO_IZQ.DC6",
		"LOGO_FUEGO_DER.DC6", "BOTON_ANCHO.DC6", "BOTON_CORTO.DC6", "FUENTE_EXOCET10.DC6",
		"FUENTE_FORMAL12.DC6", "FUENTE_FORMAL10.DC6", "FUENTE_RIDICULOUS.DC6"}

	for _, n := range nombres {
		datos, err := os.ReadFile(filepath.Join(dir, n))
		if err != nil {
			fmt.Printf("SONDA %-22s FALTA\n", n)

			continue
		}

		dc, err := d2dc6.Load(datos)
		if err != nil {
			fmt.Printf("SONDA %-22s ERROR %v\n", n, err)

			continue
		}

		fmt.Printf("SONDA %-22s bytes=%7d frames=%3d\n", n, len(datos), len(dc.Frames))

		vistos := map[string]int{}

		for _, f := range dc.Frames {
			if f == nil {
				continue
			}

			vistos[fmt.Sprintf("%dx%d@%d,%d", f.Width, f.Height, f.OffsetX, f.OffsetY)]++
		}

		fmt.Printf("       medidas distintas=%d: ", len(vistos))

		for k, c := range vistos {
			fmt.Printf("%s x%d; ", k, c)
		}

		fmt.Println()

		if solo {
			continue
		}

		usados := make([]bool, 256)

		for i := range dc.Frames {
			for _, v := range dc.DecodeFrame(i) {
				usados[v] = true
			}
		}

		cuenta := 0

		for _, u := range usados {
			if u {
				cuenta++
			}
		}

		fmt.Printf("       indices de paleta usados=%d\n", cuenta)
	}

	return nil
}

// ---------------------------------------------------------------------------------------------
// 3. COMPONER: las texturas, el plan de dibujo y la referencia
// ---------------------------------------------------------------------------------------------

// textura: un lienzo de INDICES de 8 bits listo para subir al PVR, con su paleta (banco) y su tamano
// potencia de dos. `ceroTransparente` es la regla del motor (d2util.ImgIndexToRGBA: "index zero is
// hardcoded transparent regardless of palette").
type textura struct {
	id     int
	nombre string
	tipo   string // fondo | logo | boton | fuente
	arch   string
	w, h   int
	pw, ph int
	banco  int
	idx    []byte
	pal    [256]color.RGBA
}

// op: un quad del plan. Las coordenadas son LOGICAS (800x600); la consola las escala 4/5. `cond`
// codifica cuando se dibuja: 0 = siempre; si no, ((boton+1)<<1)|estado, con estado 0 = normal y
// 1 = apretado (el boton seleccionado se dibuja con el frame apretado, como hace d2ui.Button).
type op struct {
	tex              int
	x, y, w, h       int
	u0, v0, u1, v1   int
	argb             uint32
	modo             int // 0 normal (src sobre dst), 1 aditivo (el fuego del logo)
	cond             int
}

const (
	modoNormal  = 0
	modoAditivo = 1
)

// Las alineaciones de d2ui.HorizontalAlign (label.go: getAlignOffset).
const (
	alIzq = iota
	alCentro
	alDer
)

func condBoton(boton, estado int) int { return ((boton + 1) << 1) | estado }

// Las paletas que el menu necesita, cada una en su BANCO del PVR (el hardware tiene 1024 entradas = 4
// bancos de 256). Los DC6 usan indices 0..255 y cada asset viene con SU paleta: sin bancos habria que
// remapear indices, que es justo lo que no se quiere (el indice es del asset, no del puerto).
var bancos = []struct {
	archivo string
	nombre  string
}{
	{"PAL_SKY.PAL", "sky"},
	{"PAL_UNITS.PAL", "units"},
	{"PAL_STATIC.PAL", "static"},
}

type glifo struct {
	frame, ancho, alto int
}

type fuente struct {
	nombre string
	dc6    string
	tbl    string
	banco  int
	glifos map[rune]glifo
	texID  int
	celdaW int
	celdaH int
}

func componerMenu(dir string, foco int) error {
	leer := func(n string) ([]byte, error) { return os.ReadFile(filepath.Join(dir, n)) }

	// --- paletas: se leen con el codigo del motor (d2dat) ---
	var pals [3][256]color.RGBA

	for i, b := range bancos {
		datos, err := leer(b.archivo)
		if err != nil {
			return err
		}

		pal, err := d2dat.Load(datos)
		if err != nil {
			return fmt.Errorf("%s: %w", b.archivo, err)
		}

		for j := 0; j < 256; j++ {
			c, err := pal.GetColor(j)
			if err != nil {
				return err
			}

			pals[i][j] = color.RGBA{R: c.R(), G: c.G(), B: c.B(), A: 255}
		}

		fmt.Printf("MENU236: PALETA banco=%d %-8s colores=256 crc32=%d\n", i, b.nombre, crc32Paletta(pals[i]))
	}

	texturas := make([]*textura, 0, 12)
	ops := make([]op, 0, 256)

	nuevaTextura := func(tipo, arch, nombre string, w, h, banco int) *textura {
		t := &textura{
			id: len(texturas), nombre: nombre, tipo: tipo, arch: arch,
			w: w, h: h, pw: sigPot2(w), ph: sigPot2(h), banco: banco,
			idx: make([]byte, w*h), pal: pals[banco],
		}
		texturas = append(texturas, t)

		return t
	}

	// --- 1. EL FONDO: 12 frames en grilla 4x3 (= el RenderSegmented(screen, 4, 3, 0) del motor) ---
	rawFondo, err := leer("FONDO.DC6")
	if err != nil {
		return err
	}

	dcFondo, err := d2dc6.Load(rawFondo)
	if err != nil {
		return err
	}

	tFondo := nuevaTextura("fondo", "FONDO.DC6", "FONDO", logAncho, logAlto, 0)
	if err := grillaFondo(dcFondo, tFondo); err != nil {
		return err
	}

	ops = append(ops, op{tex: tFondo.id, x: 0, y: 0, w: logAncho, h: logAlto,
		u0: 0, v0: 0, u1: logAncho, v1: logAlto, argb: 0xffffffff, modo: modoNormal})

	// --- 2. LOS LOGOS: cuatro sprites en (400,120) (diabloLogoX, diabloLogoY), atados por ABAJO ---
	// Orden del motor (renderLogos): negro izq, negro der, fuego izq, fuego der. Los dos de fuego van
	// con efecto modulate = ADITIVO.
	logos := []struct {
		arch  string
		dx    int
		dy    int
		modo  int
	}{
		{"LOGO_NEGRO_IZQ.DC6", diabloLogoX, diabloLogoY, modoNormal},
		{"LOGO_NEGRO_DER.DC6", diabloLogoX, diabloLogoY, modoNormal},
		{"LOGO_FUEGO_IZQ.DC6", diabloLogoX, diabloLogoY, modoAditivo},
		{"LOGO_FUEGO_DER.DC6", diabloLogoX, diabloLogoY, modoAditivo},
	}

	for _, l := range logos {
		raw, err := leer(l.arch)
		if err != nil {
			return err
		}

		dc, err := d2dc6.Load(raw)
		if err != nil {
			return err
		}

		f := dc.Frames[0]
		t := nuevaTextura("logo", l.arch, strings.TrimSuffix(l.arch, ".DC6"), int(f.Width), int(f.Height), 1)
		copy(t.idx, dc.DecodeFrame(0))

		// d2ui.Sprite.Render: la posicion es (x, y-alto) y despues el frame suma su offset.
		x := l.dx - int(f.Height) // placeholder reemplazado abajo (se aclara el por que)
		_ = x

		x = l.dx + int(f.OffsetX)
		y := l.dy - int(f.Height) + int(f.OffsetY)

		ops = append(ops, op{tex: t.id, x: x, y: y, w: int(f.Width), h: int(f.Height),
			u0: 0, v0: 0, u1: int(f.Width), v1: int(f.Height), argb: 0xffffffff, modo: l.modo})

		fmt.Printf("MENU236: LOGO %-20s frame0=%dx%d off=%d,%d destino=%d,%d modo=%d\n",
			l.arch, f.Width, f.Height, f.OffsetX, f.OffsetY, x, y, l.modo)
	}

	// --- 3. LAS FUENTES (atlas de 256 celdas en grilla 16x16) ---
	fuentes := []*fuente{
		{nombre: "exocet10", dc6: "FUENTE_EXOCET10.DC6", tbl: "FUENTE_EXOCET10.TBL", banco: 1},
		{nombre: "ridiculous", dc6: "FUENTE_RIDICULOUS.DC6", tbl: "FUENTE_RIDICULOUS.TBL", banco: 1},
		{nombre: "formal12", dc6: "FUENTE_FORMAL12.DC6", tbl: "FUENTE_FORMAL12.TBL", banco: 2},
		{nombre: "formal10", dc6: "FUENTE_FORMAL10.DC6", tbl: "FUENTE_FORMAL10.TBL", banco: 2},
	}

	for _, fu := range fuentes {
		rawDC6, err := leer(fu.dc6)
		if err != nil {
			return err
		}

		dc, err := d2dc6.Load(rawDC6)
		if err != nil {
			return err
		}

		rawTBL, err := leer(fu.tbl)
		if err != nil {
			return err
		}

		// La tabla se lee con el lector del MOTOR (d2font), no con uno propio: es el formato que usa
		// el juego para saber que frame es cada letra y cuanto avanza el cursor.
		tipo, err := d2font.Load(rawTBL)
		if err != nil {
			return fmt.Errorf("%s: %w", fu.tbl, err)
		}

		f0 := dc.Frames[0]
		celdaW, celdaH := int(f0.Width), int(f0.Height)

		pasoW, pasoH := sigPot2(celdaW), sigPot2(celdaH)
		atlasW, atlasH := 16*pasoW, 16*pasoH

		t := nuevaTextura("fuente", fu.dc6, "FUENTE_"+strings.ToUpper(fu.nombre), atlasW, atlasH, fu.banco)

		fu.glifos = make(map[rune]glifo, len(tipo.Glyphs))
		fu.texID = t.id
		fu.celdaW, fu.celdaH = celdaW, celdaH

		// Se decodifica el atlas del motor frame por frame y se pega en su celda de la grilla.
		for r, g := range tipo.Glyphs {
			fi := g.FrameIndex()
			if fi < 0 || fi >= len(dc.Frames) {
				continue
			}

			col, fila := fi%16, fi/16

			datos := dc.DecodeFrame(fi)
			fw, fh := int(dc.Frames[fi].Width), int(dc.Frames[fi].Height)

			for y := 0; y < fh; y++ {
				for x := 0; x < fw; x++ {
					t.idx[(fila*pasoH+y)*atlasW+col*pasoW+x] = datos[y*fw+x]
				}
			}

			fu.glifos[r] = glifo{frame: fi, ancho: g.Width(), alto: celdaH}
		}

		fmt.Printf("MENU236: FUENTE %-11s banco=%d celdas=%d celda=%dx%d atlas=%dx%d pw=%d ph=%d crc32=%d\n",
			fu.nombre, fu.banco, len(tipo.Glyphs), celdaW, celdaH, atlasW, atlasH, t.pw, t.ph,
			crc32IEEE(t.idx))
	}

	// --- 4. LOS BOTONES (2 trozos horizontales: 256+16 el ancho, 135x25 el corto) ---
	ancho := botonAncho(dcFondo, leer, nuevaTextura)
	corto, err := botonCorto(leer, nuevaTextura)
	if err != nil {
		return err
	}

	// --- 5. EL TEXTO Y LOS BOTONES DEL MENU, con las posiciones del motor ---
	// Los textos: los de las tablas del juego (CADENAS.TXT) y los literales del codigo.
	cad := leerCadenas(dir)

	type etiqueta struct {
		texto   string
		fuente  *fuente
		x, y    int
		alinear int // 0 izquierda, 1 centro, 2 derecha
		color   uint32
	}

	etiquetas := []etiqueta{
		{cad["version"], fuentePor(fuentes, "formal12"), versionLabelX, versionLabelY, alDer, 0xffffffff},
		{cad["commit"], fuentePor(fuentes, "formal10"), commitLabelX, commitLabelY, alIzq, 0xffffffff},
		{cad["copyright"], fuentePor(fuentes, "formal12"), copyrightX, copyrightY, alCentro, 0xbca88cff},
		{cad["copyright2"], fuentePor(fuentes, "formal12"), copyright2X, copyright2Y, alCentro, 0xbca88cff},
		{cad["od2"], fuentePor(fuentes, "formal10"), od2LabelX, od2LabelY, alCentro, 0xffff8cff},
	}

	for _, e := range etiquetas {
		if e.fuente == nil {
			return fmt.Errorf("fuente no encontrada para la etiqueta %q", e.texto)
		}

		ops = append(ops, opsTexto(texturas, e.fuente, e.texto, e.x, e.y, e.alinear, e.color, 0)...)
	}

	// Los siete botones del menu principal, en el orden de focusButtons() del motor.
	botones := []struct {
		tipo    string // ancho | corto
		tex     *textura
		x, y    int
		texto   string
		fuente  *fuente
	}{
		{"ancho", ancho, singlePlayerBtnX, singlePlayerBtnY, cad["singleplayer"], fuentePor(fuentes, "exocet10")},
		{"ancho", ancho, multiplayerBtnX, multiplayerBtnY, cad["multiplayer"], fuentePor(fuentes, "exocet10")},
		{"ancho", ancho, githubBtnX, githubBtnY, cad["website"], fuentePor(fuentes, "exocet10")},
		{"ancho", ancho, mapTestBtnX, mapTestBtnY, cad["maptest"], fuentePor(fuentes, "exocet10")},
		{"corto", corto, creditBtnX, creditBtnY, cad["credits"], fuentePor(fuentes, "ridiculous")},
		{"corto", corto, cineBtnX, cineBtnY, cad["cinematics"], fuentePor(fuentes, "ridiculous")},
		{"ancho", ancho, exitDiabloBtnX, exitDiabloBtnY, cad["exit"], fuentePor(fuentes, "exocet10")},
	}

	fmt.Println("MENU236: CADENAS")

	for _, k := range []string{"singleplayer", "multiplayer", "website", "maptest", "credits", "cinematics", "exit", "copyright", "copyright2", "version", "commit", "od2"} {
		fmt.Printf("MENU236:   %-12s %q\n", k, cad[k])
	}

	for i, b := range botones {
		if b.fuente == nil {
			return fmt.Errorf("fuente no encontrada para el boton %d", i)
		}

		ops = append(ops, opsBoton(texturas, b.tex, b.fuente, i, b.x, b.y, b.texto)...)
		fmt.Printf("MENU236: BOTON %d %-6s pos=%d,%d tam=%dx%d texto=%q\n", i, b.tipo, b.x, b.y, b.tex.w, b.tex.h/2, b.texto)
	}

	// --- 6. escribir el plan y las referencias ---
	if err := escribirPlan(filepath.Join(dir, "PLAN.TXT"), texturas, ops); err != nil {
		return err
	}

	// CRC de cada lienzo: es el numero que la consola tiene que reproducir leyendo el DC6 del disco.
	total := 0

	for _, t := range texturas {
		fmt.Printf("MENU236: TEXTURA id=%2d %-22s tipo=%-6s tam=%dx%d pot=%dx%d banco=%d crc32=%d\n",
			t.id, t.nombre, t.tipo, t.w, t.h, t.pw, t.ph, t.banco, crc32IEEE(t.idx))
		total += t.pw * t.ph
	}

	fmt.Printf("MENU236: QUADS total=%d texturas=%d vram_bytes=%d\n", len(ops), len(texturas), total)

	// referencia sin seleccion (foco = -1), con el boton 0 apretado y con el 1 apretado: las tres
	// sirven para medir la navegacion en la captura sin adivinar nada.
	// Van en un subdirectorio: el ISO9660 de la consola solo acepta A-Z/0-9/_/punto (medido: los guiones
	// de "referencia-foco-1-640.png" hacen abortar el armado del disco) y ademas no hacen falta a bordo.
	refDir := filepath.Join(dir, "referencias")

	if err := os.MkdirAll(refDir, 0o755); err != nil {
		return err
	}

	for _, f := range []int{-1, 0, 1} {
		img800 := rasterizar(texturas, ops, f)
		if err := escribirPNG(fmt.Sprintf("%s/referencia-foco%d-800.png", refDir, f), img800); err != nil {
			return err
		}

		img640 := reducirNearest(img800, panAncho, panAlto)
		if err := escribirPNG(fmt.Sprintf("%s/referencia-foco%d-640.png", refDir, f), img640); err != nil {
			return err
		}

		noFondo, colores := contarElementos(img800)
		fmt.Printf("MENU236: REFERENCIA foco=%d 800x600 pixeles_no_fondo=%d colores=%d\n", f, noFondo, colores)
	}

	// La navegacion esperada, medida en el HOST: cuantos pixeles cambian de foco 0 a foco 1 y donde.
	dif, caja := diferencia(rasterizar(texturas, ops, 0), rasterizar(texturas, ops, 1))
	fmt.Printf("MENU236: NAVEGACION host foco0->foco1 pixeles_que_cambian=%d caja=%v\n", dif, caja)
	dif, caja = diferencia(rasterizar(texturas, ops, -1), rasterizar(texturas, ops, 0))
	fmt.Printf("MENU236: NAVEGACION host sin_foco->foco0 pixeles_que_cambian=%d caja=%v\n", dif, caja)

	return nil
}

// Las constantes del motor (main_menu.go, lineas 38-76), copiadas con su nombre para que se pueda
// cotejar una por una.
const (
	backgroundX, backgroundY             = 0, 0
	versionLabelX, versionLabelY         = 795, -10
	commitLabelX, commitLabelY           = 2, 2
	copyrightX, copyrightY               = 400, 500
	copyright2X, copyright2Y             = 400, 525
	od2LabelX, od2LabelY                 = 400, 580
	diabloLogoX, diabloLogoY             = 400, 120
	exitDiabloBtnX, exitDiabloBtnY       = 264, 535
	creditBtnX, creditBtnY               = 264, 505
	cineBtnX, cineBtnY                   = 401, 505
	singlePlayerBtnX, singlePlayerBtnY   = 264, 290
	githubBtnX, githubBtnY               = 264, 400
	mapTestBtnX, mapTestBtnY             = 264, 440
	multiplayerBtnX, multiplayerBtnY     = 264, 330
)

// grillaFondo: pega los 12 frames del fondo en la grilla 4x3 que pide RenderSegmented(screen, 4, 3, 0).
// La geometria NO se supone: se arma sumando los anchos de cada fila y los altos de cada fila, y se
// exige que el resultado sea EXACTAMENTE 800x600 (si algun dia no lo es, esto aborta en vez de dibujar
// cualquier cosa).
func grillaFondo(dc *d2dc6.DC6, t *textura) error {
	const cols, filas = 4, 3

	if len(dc.Frames) < cols*filas {
		return fmt.Errorf("el fondo trae %d frames, la grilla 4x3 pide %d", len(dc.Frames), cols*filas)
	}

	var altoFila [filas]int
	var anchoCol [cols]int

	for i := 0; i < cols*filas; i++ {
		f := dc.Frames[i]
		col, fila := i%cols, i/cols

		if int(f.Width) > 0 {
			anchoCol[col] = int(f.Width)
		}

		if int(f.Height) > altoFila[fila] {
			altoFila[fila] = int(f.Height)
		}
	}

	totalW, totalH := 0, 0

	for _, v := range anchoCol {
		totalW += v
	}

	for _, v := range altoFila {
		totalH += v
	}

	if totalW != logAncho || totalH != logAlto {
		return fmt.Errorf("la grilla del fondo mide %dx%d y la pantalla logica es %dx%d", totalW, totalH, logAncho, logAlto)
	}

	x0 := 0

	for fila := 0; fila < filas; fila++ {
		y0 := 0

		for k := 0; k < fila; k++ {
			y0 += altoFila[k]
		}

		x0 = 0

		for col := 0; col < cols; col++ {
			i := col + fila*cols
			f := dc.Frames[i]
			datos := dc.DecodeFrame(i)
			fw, fh := int(f.Width), int(f.Height)

			for y := 0; y < fh; y++ {
				for x := 0; x < fw; x++ {
					t.idx[(y0+y)*t.w+x0+x] = datos[y*fw+x]
				}
			}

			x0 += anchoCol[col]
		}
	}

	fmt.Printf("MENU236: FONDO grilla=%dx%d anchos=%v altos=%v = %dx%d (la pantalla logica)\n",
		cols, filas, anchoCol, altoFila, totalW, totalH)

	return nil
}

// botonAncho: el boton ancho son DOS trozos (256+16 de ancho, 35 de alto): frames 0 y 1 para el estado
// normal y 2 y 3 para el apretado (d2ui.Button: RenderSegmented con XSegments=2 y BaseFrame 0 o 1).
// Van los cuatro en un solo lienzo: fila 0 = normal, fila 1 = apretado.
func botonAncho(_ *d2dc6.DC6, leer func(string) ([]byte, error), nueva func(tipo, arch, nombre string, w, h, banco int) *textura) *textura {
	raw, err := leer("BOTON_ANCHO.DC6")
	if err != nil {
		panic(err)
	}

	dc, err := d2dc6.Load(raw)
	if err != nil {
		panic(err)
	}

	w := int(dc.Frames[0].Width) + int(dc.Frames[1].Width)
	h := int(dc.Frames[0].Height) * 2

	t := nueva("boton", "BOTON_ANCHO.DC6", "BOTON_ANCHO", w, h, 1)

	for estado := 0; estado < 2; estado++ {
		x0 := 0

		for seg := 0; seg < 2; seg++ {
			i := estado*2 + seg
			f := dc.Frames[i]
			datos := dc.DecodeFrame(i)
			fw, fh := int(f.Width), int(f.Height)

			for y := 0; y < fh; y++ {
				for x := 0; x < fw; x++ {
					t.idx[(estado*fh+y)*w+x0+x] = datos[y*fw+x]
				}
			}

			x0 += fw
		}
	}

	fmt.Printf("MENU236: BOTON_ANCHO lienzo=%dx%d (trozos %dx%d + %dx%d)\n",
		w, h, dc.Frames[0].Width, dc.Frames[0].Height, dc.Frames[1].Width, dc.Frames[1].Height)

	return t
}

func botonCorto(leer func(string) ([]byte, error), nueva func(tipo, arch, nombre string, w, h, banco int) *textura) (*textura, error) {
	raw, err := leer("BOTON_CORTO.DC6")
	if err != nil {
		return nil, err
	}

	dc, err := d2dc6.Load(raw)
	if err != nil {
		return nil, err
	}

	f0, f1 := dc.Frames[0], dc.Frames[1]
	t := nueva("boton", "BOTON_CORTO.DC6", "BOTON_CORTO", int(f0.Width), int(f0.Height)*2, 1)

	for i := 0; i < 2; i++ {
		f := dc.Frames[i]
		datos := dc.DecodeFrame(i)
		fw, fh := int(f.Width), int(f.Height)

		for y := 0; y < fh; y++ {
			for x := 0; x < fw; x++ {
				t.idx[(i*fh+y)*t.w+x] = datos[y*fw+x]
			}
		}
	}

	fmt.Printf("MENU236: BOTON_CORTO lienzo=%dx%d (normal %dx%d / apretado %dx%d)\n",
		t.w, t.h, f0.Width, f0.Height, f1.Width, f1.Height)

	return t, nil
}

// opsTexto: la etiqueta como el motor la dibuja (d2ui.Label.Render + d2font.RenderText): se mide la
// linea sumando los anchos de los glifos, se aplica la alineacion y se avanza el cursor glifo a glifo.
// Cada glifo es un quad de la celda de su frame en el atlas (el motor dibuja la CELDA entera, no solo
// el ancho del glifo).
func opsTexto(texturas []*textura, f *fuente, texto string, x, y, alinear int, argb uint32, cond int) []op {
	ancho := 0

	for _, r := range texto {
		if g, ok := f.glifos[r]; ok {
			ancho += g.ancho
		}
	}

	x0 := x

	switch alinear {
	case alCentro:
		x0 = x - ancho/2
	case alDer:
		x0 = x - ancho
	}

	t := texturas[f.texID]
	pasoW, pasoH := t.w/16, t.h/16

	salida := make([]op, 0, len(texto))

	cursor := x0

	for _, r := range texto {
		g, ok := f.glifos[r]
		if !ok {
			continue
		}

		col, fila := g.frame%16, g.frame/16

		salida = append(salida, op{
			tex: f.texID,
			x:   cursor, y: y,
			w:    f.celdaW, h: f.celdaH,
			u0: col * pasoW, v0: fila * pasoH,
			u1: col*pasoW + f.celdaW, v1: fila*pasoH + f.celdaH,
			argb: argb, modo: modoNormal, cond: cond,
		})

		cursor += g.ancho
	}

	_ = pasoH

	return salida
}

// opsBoton: el boton completo (fondo + etiqueta) en sus DOS estados, cada uno con su condicion. El
// estado apretado mueve la etiqueta (-2,+2), como pressedButtonOffset en d2ui.Button.
func opsBoton(texturas []*textura, t *textura, f *fuente, indice, x, y int, texto string) []op {
	salida := make([]op, 0, 40)

	altoUnaFila := t.h / 2

	for estado := 0; estado < 2; estado++ {
		cond := condBoton(indice, estado)

		// los trozos del fondo: 256 + 16 (o 135 solo)
		desplaz := 0

		for seg := 0; seg < t.w; {
			anchoTrozo := t.w - seg
			if t.w == 272 && seg == 0 {
				anchoTrozo = 256
			}

			salida = append(salida, op{
				tex: t.id, x: x + desplaz, y: y, w: anchoTrozo, h: altoUnaFila,
				u0: seg, v0: estado * altoUnaFila,
				u1: seg + anchoTrozo, v1: estado*altoUnaFila + altoUnaFila,
				argb: 0xffffffff, modo: modoNormal, cond: cond,
			})

			desplaz += anchoTrozo
			seg += anchoTrozo
		}

		// la etiqueta, centrada: Label.Render con Alignment=Center en (width/2, (height-labelH)/2)
		altoTexto := f.celdaH
		textY := (altoUnaFila-altoTexto)/2 + 0
		textX := t.w / 2

		if estado == 1 {
			textX -= 2 // pressedButtonOffset
			textY += 2
		}

		// el color de la etiqueta de un boton es greyAlpha100 (d2ui: layout.LabelColor)
		salida = append(salida, opsTexto(texturas, f, texto, x+textX, y+textY, alCentro, 0x646464ff, cond)...)
	}

	return salida
}

func fuentePor(fuentes []*fuente, nombre string) *fuente {
	for _, f := range fuentes {
		if f.nombre == nombre {
			return f
		}
	}

	return nil
}

// leerCadenas: CADENAS.TXT (`clave<TAB>texto`), extraido de las tablas del juego por el modo -extraer.
func leerCadenas(dir string) map[string]string {
	salida := map[string]string{}

	datos, err := os.ReadFile(filepath.Join(dir, "CADENAS.TXT"))
	if err != nil {
		return salida
	}

	for _, l := range strings.Split(string(datos), "\n") {
		p := strings.SplitN(l, "\t", 2)
		if len(p) == 2 {
			salida[strings.TrimSpace(p[0])] = strings.TrimRight(p[1], "\r")
		}
	}

	return salida
}

func escribirPlan(ruta string, texturas []*textura, ops []op) error {
	var b bytes.Buffer

	b.WriteString("# MENU-DC-236 plan de dibujo -- lo arma el HOST con el codigo del motor (ver menu-host-236)\n")
	b.WriteString("# T <id> <tipo> <archivo> <w> <h> <pw> <ph> <banco> <nombre>\n")
	b.WriteString("# O <tex> <x> <y> <w> <h> <u0> <v0> <u1> <v1> <argb> <modo> <cond>\n")
	b.WriteString("# cond: 0 = siempre; ((boton+1)<<1)|estado, estado 0 = normal 1 = apretado\n")

	for _, t := range texturas {
		fmt.Fprintf(&b, "T %d %s %s %d %d %d %d %d %s\n",
			t.id, t.tipo, t.arch, t.w, t.h, t.pw, t.ph, t.banco, t.nombre)
	}

	for _, o := range ops {
		fmt.Fprintf(&b, "O %d %d %d %d %d %d %d %d %d %d %d %d\n",
			o.tex, o.x, o.y, o.w, o.h, o.u0, o.v0, o.u1, o.v1, o.argb, o.modo, o.cond)
	}

	return os.WriteFile(ruta, b.Bytes(), 0o644)
}

// ---------------------------------------------------------------------------------------------
// La referencia: un rasterizador chico que hace lo MISMO que el PVR (nearest, indice 0 transparente,
// color del vertice multiplicando el de la paleta, normal o aditivo) en el lienzo logico 800x600.
// ---------------------------------------------------------------------------------------------

func rasterizar(texturas []*textura, ops []op, foco int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, logAncho, logAlto))

	for _, o := range ops {
		if o.cond != 0 {
			boton := (o.cond >> 1) - 1
			estado := o.cond & 1
			apretado := boton == foco

			if (estado == 1) != apretado {
				continue
			}
		}

		t := texturas[o.tex]

		for dy := 0; dy < o.h; dy++ {
			for dx := 0; dx < o.w; dx++ {
				u := o.u0 + (dx*(o.u1-o.u0))/o.w
				v := o.v0 + (dy*(o.v1-o.v0))/o.h

				if u < 0 || v < 0 || u >= t.w || v >= t.h {
					continue
				}

				pi := t.idx[v*t.w+u]
				if pi == 0 { // "index zero is hardcoded transparent"
					continue
				}

				c := t.pal[pi]
				r := uint32(c.R) * ((o.argb >> 24) & 0xff) / 255
				g := uint32(c.G) * ((o.argb >> 16) & 0xff) / 255
				bb := uint32(c.B) * ((o.argb >> 8) & 0xff) / 255

				px, py := o.x+dx, o.y+dy
				if px < 0 || py < 0 || px >= logAncho || py >= logAlto {
					continue
				}

				i := img.PixOffset(px, py)

				if o.modo == modoAditivo {
					img.Pix[i] = uint8(min32(255, uint32(img.Pix[i])+r))
					img.Pix[i+1] = uint8(min32(255, uint32(img.Pix[i+1])+g))
					img.Pix[i+2] = uint8(min32(255, uint32(img.Pix[i+2])+bb))
					img.Pix[i+3] = 255

					continue
				}

				img.Pix[i] = uint8(r)
				img.Pix[i+1] = uint8(g)
				img.Pix[i+2] = uint8(bb)
				img.Pix[i+3] = 255
			}
		}
	}

	return img
}

func min32(a, b uint32) uint32 {
	if a < b {
		return a
	}

	return b
}

// reducirNearest: 800x600 -> 640x480 tomando el pixel mas cercano (es lo que hace el PVR con
// FILTER_NONE: la consola escala las coordenadas 4/5 y muestrea sin filtrar).
func reducirNearest(src *image.RGBA, w, h int) *image.RGBA {
	dst := image.NewRGBA(image.Rect(0, 0, w, h))

	for y := 0; y < h; y++ {
		sy := (y * logAlto) / h

		for x := 0; x < w; x++ {
			sx := (x * logAncho) / w
			copy(dst.Pix[dst.PixOffset(x, y):dst.PixOffset(x, y)+4], src.Pix[src.PixOffset(sx, sy):src.PixOffset(sx, sy)+4])
		}
	}

	return dst
}

func contarElementos(img *image.RGBA) (int, int) {
	noFondo := 0
	colores := map[uint32]bool{}

	for i := 0; i < len(img.Pix); i += 4 {
		r, g, b := img.Pix[i], img.Pix[i+1], img.Pix[i+2]
		if r > 12 || g > 12 || b > 12 {
			noFondo++
			colores[uint32(r)>>3<<10|uint32(g)>>3<<5|uint32(b)>>3] = true
		}
	}

	return noFondo, len(colores)
}

func diferencia(a, b *image.RGBA) (int, [4]int) {
	dif := 0
	caja := [4]int{logAncho, logAlto, -1, -1}

	for y := 0; y < logAlto; y++ {
		for x := 0; x < logAncho; x++ {
			i := a.PixOffset(x, y)

			if a.Pix[i] == b.Pix[i] && a.Pix[i+1] == b.Pix[i+1] && a.Pix[i+2] == b.Pix[i+2] {
				continue
			}

			dif++

			if x < caja[0] {
				caja[0] = x
			}

			if y < caja[1] {
				caja[1] = y
			}

			if x > caja[2] {
				caja[2] = x
			}

			if y > caja[3] {
				caja[3] = y
			}
		}
	}

	return dif, caja
}

func escribirPNG(ruta string, img *image.RGBA) error {
	f, err := os.Create(ruta)
	if err != nil {
		return err
	}

	defer f.Close()

	return png.Encode(f, img)
}

func sigPot2(n int) int {
	p := 1
	for p < n {
		p <<= 1
	}

	return p
}

func crc32Paletta(pal [256]color.RGBA) uint32 {
	b := make([]byte, 0, 512)

	for _, c := range pal {
		// ARGB1555 como lo sube el PVR (2 bytes por color, el bajo primero): el mismo numero que la consola
		v := uint16(1)<<15 | uint16(c.R>>3)<<10 | uint16(c.G>>3)<<5 | uint16(c.B>>3)
		b = append(b, byte(v&0xff), byte(v>>8))
	}

	return crc32IEEE(b)
}

// verif-acto1-237: LA MITAD DE HOST del hito del acto 1 (DC-002.37).
//
// Hace EXACTAMENTE lo mismo que el POC de la consola `console/dc/poc/acto1-disco`, con el mismo codigo de
// composicion (`nivel.go`, el MISMO archivo: se compara su sha256 en la evidencia), leido del mismo MPQ:
//
//   1. lee el .ds1 del acto 1 y lo mide (bytes, CRC-32, version, acto, tamano, objetos, capas y tiles no
//      vacios de cada capa);
//   2. lee, uno por uno, los DT1 que el propio DS1 lista (ruta normalizada como la normaliza el motor) y
//      los mide (bytes, CRC-32, cuantos tiles);
//   3. compone la VENTANA isometrica del nivel (el mismo compositor que la consola) y mide: cuantos tiles
//      entraron, cuantos quedaron sin tile, cuantos cayeron fuera de la ventana, cuantos pixeles del lienzo
//      NO son el indice 0 y el CRC-32 del lienzo de indices;
//   4. lee la paleta del acto (pal.dat) y escribe la REFERENCIA PNG con la que se compara la captura del
//      emulador.
//
// Uso:  go run ./console/dc/verif-acto1-237 -png /tmp/dc237/referencia.png
//
// Los numeros que imprime son los que tiene que dar el POC de la consola, uno por uno.
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

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2ds1"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2dt1"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2mpq"
)

const (
	ds1Defecto = "data\\global\\tiles\\ACT1\\TOWN\\townN1.ds1"
	palDefecto = "data/global/palette/ACT1/pal.dat"
	ventDefecto = 512
)

func autodetectarD2() string {
	if d := os.Getenv("D2_DIR"); d != "" {
		return d
	}

	if st, err := os.Stat(filepath.Join(os.Getenv("HOME"), ".wine-d2", "drive_c", "Games", "Diablo II", "d2data.mpq")); err == nil && !st.IsDir() {
		return filepath.Join(os.Getenv("HOME"), ".wine-d2", "drive_c", "Games", "Diablo II")
	}

	return ""
}

func main() {
	mpqPath := flag.String("mpq", "", "d2data.mpq (por defecto: la instalacion del juego)")
	ds1Name := flag.String("ds1", ds1Defecto, "el .ds1 del nivel (barras invertidas, como las escribe el MPQ)")
	palName := flag.String("pal", palDefecto, "la paleta del acto dentro del MPQ")
	ventana := flag.Int("ventana", ventDefecto, "lado de la ventana isometrica en pixeles (potencia de dos)")
	centro := flag.String("centro", "", "celda que queda en el centro, como x,y (por defecto: el centro del nivel)")
	pngPath := flag.String("png", "", "si se pasa, escribe la referencia PNG aca")
	flag.Parse()

	ruta := *mpqPath
	if ruta == "" {
		carpeta := autodetectarD2()
		if carpeta == "" {
			fmt.Fprintln(os.Stderr, "no encuentro la instalacion de Diablo II (usa -mpq o D2_DIR)")
			os.Exit(1)
		}

		ruta = filepath.Join(carpeta, "d2data.mpq")
	}

	m, err := d2mpq.FromFile(ruta)
	if err != nil {
		fmt.Fprintln(os.Stderr, "no pude abrir el MPQ:", err)
		os.Exit(1)
	}
	defer m.Close()

	// --- 1. el DS1 ---
	raw, err := m.ReadFile(*ds1Name)
	if err != nil {
		fmt.Fprintln(os.Stderr, "VERIF237: DS1 FALLO:", err)
		os.Exit(1)
	}

	fmt.Printf("VERIF237: DS1 %s bytes=%d crc32=%d\n", *ds1Name, len(raw), crc32.ChecksumIEEE(raw))

	const trozo = 65536

	for i := 0; i < len(raw); i += trozo {
		fin := i + trozo
		if fin > len(raw) {
			fin = len(raw)
		}

		fmt.Printf("VERIF237: DS1 trozo %d desde=%d bytes=%d crc32=%d\n",
			i/trozo, i, fin-i, crc32.ChecksumIEEE(raw[i:fin]))
	}

	d, err := d2ds1.Unmarshal(raw)
	if err != nil {
		fmt.Fprintln(os.Stderr, "VERIF237: DS1 PARSE-FALLO:", err)
		os.Exit(1)
	}

	fmt.Printf("VERIF237: DS1 version=%d act=%d w=%d h=%d objetos=%d files=%d\n",
		d.Version(), d.Act, d.Width(), d.Height(), len(d.Objects), len(d.Files))

	// --- 2. la ventana y los pedidos ---
	cx, cy := d.Width()/2, d.Height()/2
	if *centro != "" {
		if _, err := fmt.Sscanf(*centro, "%d,%d", &cx, &cy); err != nil {
			fmt.Fprintln(os.Stderr, "VERIF237: -centro tiene que ser x,y:", err)
			os.Exit(2)
		}
	}

	n := NuevoNivel(d, *ventana, *ventana, cx, cy)

	fmt.Printf("VERIF237: VENTANA lado=%d origen=%d,%d centroCelda=%d,%d\n",
		n.Ancho, n.OrigenX, n.OrigenY, cx, cy)
	fmt.Printf("VERIF237: PEDIDOS pisos=%d paredes=%d total=%d\n",
		n.PisosPedidos, n.ParedesPedidas, len(n.Pedidos))

	// --- 3. los DT1, uno por uno, alimentando el compositor ---
	for _, f := range d.Files {
		nombre := RutaDelDT1(f)

		rawDT1, err := LeerDelMPQ(m, nombre)
		if err != nil {
			fmt.Printf("VERIF237: DT1 %s FALLO: %v\n", nombre, err)
			continue
		}

		dt1, err := d2dt1.LoadDT1(rawDT1)
		if err != nil {
			fmt.Printf("VERIF237: DT1 %s bytes=%d crc32=%d PARSE-FALLO: %v\n",
				nombre, len(rawDT1), crc32.ChecksumIEEE(rawDT1), err)
			continue
		}

		fmt.Printf("VERIF237: DT1 %s bytes=%d crc32=%d tiles=%d\n",
			nombre, len(rawDT1), crc32.ChecksumIEEE(rawDT1), len(dt1.Tiles))

		n.Alimentar(dt1.Tiles)

		// el que llama lo suelta: en la consola esto es lo que hace que no haga falta tener los 4,35 MB de
		// DT1 del town vivos a la vez
		rawDT1 = nil
		dt1 = nil
	}

	n.Cerrar()

	fmt.Printf("VERIF237: COMPUESTO pisos=%d/%d paredes=%d/%d sintilePiso=%d sintilePared=%d fuera=%d\n",
		n.PisosCompuestos, n.PisosPedidos, n.ParedesCompuestas, n.ParedesPedidas,
		n.PisosSinTile, n.ParedesSinTile, n.FueraDeVentana)
	fmt.Printf("VERIF237: LIENZO pixelesNoVacios=%d de %d (%.2f%%) crc32=%d\n",
		n.PixelesNoVacios(), n.Ancho*n.Alto,
		float64(n.PixelesNoVacios())*100/float64(n.Ancho*n.Alto), n.CRC32())
	fmt.Printf("VERIF237: LIENZO pixelesPiso=%d pixelesPared=%d\n", n.PixelesDelPiso, n.PixelesDePared)

	// --- 4. la paleta del acto ---
	pb, err := LeerDelMPQ(m, *palName)
	if err != nil {
		fmt.Println("VERIF237: PALETA-LEER FALLO:", err)
		return
	}

	if len(pb) < 768 {
		fmt.Printf("VERIF237: PALETA CORTA: %d bytes (se esperan 768 = 256 colores B,G,R)\n", len(pb))
		return
	}

	pal := PaletaRGB(pb)
	argb := PaletaARGB1555(pal)
	fmt.Printf("VERIF237: PALETA bytes=%d colores=%d crc32=%d\n", len(pb), 256, CRCPaleta(argb))

	// --- 5. la referencia PNG de la ventana ---
	if *pngPath == "" {
		return
	}

	img := image.NewRGBA(image.Rect(0, 0, n.Ancho, n.Alto))

	for y := 0; y < n.Alto; y++ {
		for x := 0; x < n.Ancho; x++ {
			r, g, b := ColorDelIndice(pal, n.Canvas[y*n.Ancho+x])
			img.SetRGBA(x, y, color.RGBA{R: r, G: g, B: b, A: 255})
		}
	}

	f, err := os.Create(*pngPath)
	if err != nil {
		fmt.Println("VERIF237: PNG FALLO:", err)
		return
	}

	if err := png.Encode(f, img); err != nil {
		fmt.Println("VERIF237: PNG-ENCODE FALLO:", err)
	}

	_ = f.Close()
	fmt.Printf("VERIF237: PNG %s escribito (%s, %d bytes)\n", *pngPath, strings.TrimPrefix(filepath.Base(*pngPath), ""), tamaño(*pngPath))
}

func tamaño(ruta string) int64 {
	st, err := os.Stat(ruta)
	if err != nil {
		return -1
	}

	return st.Size()
}

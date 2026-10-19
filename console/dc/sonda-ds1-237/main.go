// sonda-ds1-237: LOS .ds1 DEL ACTO 1, MEDIDOS EN EL HOST CON EL MISMO CODIGO DEL MOTOR.
//
// Es la mitad de host del par de instrumentos de DC-002.37 (criterio 1 de la tarjeta: "el mapa del acto 1
// pintado en la consola: cuantos tiles entraron, y el hash de los .ds1 leidos igual al que da el mismo
// archivo en el host"). La otra mitad es el POC `console/dc/poc/acto1-disco`, que corre en la consola y
// imprime los MISMOS numeros por archivo.
//
// Que mide, por cada .ds1:
//   - los bytes y el CRC-32 del archivo tal como sale del MPQ;
//   - la version y el acto del DS1, y su tamano en celdas (width x height);
//   - cuantas capas de piso, de pared, de sombra y de sustitucion trae, y cuantos TILES NO VACIOS tiene
//     cada capa (un tile con Style=0 y Sequence=0 es el "no hay nada" del formato);
//   - cuantos OBJETOS y cuantos GRUPOS de sustitucion;
//   - la LISTA DE DT1 del propio DS1 (el campo `Files`: el DS1 dice de que archivos de tiles salen sus
//     tiles) y, por cada uno: bytes, CRC-32, cuantos tiles trae y cuantos de cada tipo.
//
// Por que existe: para pintar un nivel hay que saber de que DT1 sale cada tile y cuantos tiles hay que
// componer. Eso estaba sin medir: se suponia. Aca se mide, con el lector del motor tal cual.
//
// Uso:  go run ./console/dc/sonda-ds1-237 -ds1 'ruta1.ds1,ruta2.ds1'   (las rutas con barras invertidas,
//       como las escribe el MPQ; el MPQ sale de D2_DIR o de la instalacion por defecto)
//
// No entra al repo ningun dato del juego: el MPQ se lee del directorio de la instalacion.
package main

import (
	"flag"
	"fmt"
	"hash/crc32"
	"os"
	"path/filepath"
	"strings"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2ds1"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2dt1"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2mpq"
)

// barraInvertida: el separador de path de Windows, armado por su codigo (92) en vez de escribir la barra
// en una cadena: asi este archivo no depende de como se escapa una barra invertida.
const barraInvertida = string(rune(92))

// autodetectarD2: el mismo helper de bake-list/bake-dc6 (la instalacion del juego no vive en el repo).
func autodetectarD2() string {
	if d := os.Getenv("D2_DIR"); d != "" {
		return d
	}

	candidatas := []string{
		filepath.Join(os.Getenv("HOME"), ".wine-d2", "drive_c", "Games", "Diablo II"),
	}

	for _, c := range candidatas {
		if st, err := os.Stat(filepath.Join(c, "d2data.mpq")); err == nil && !st.IsDir() {
			return c
		}
	}

	return ""
}

// normalizarDT1: las mismas reglas que `MapEngine.AddDS1` (d2core/d2map/d2mapengine/engine.go) para
// pasar del nombre que trae el DS1 al que existe en el MPQ. MEDIDO con esta sonda: el DS1 lista su DT1
// como `<prefijo de la instalacion>tiles` + `act1` + `town` + `floor.tg1` y en el MPQ el archivo es
// `data/global/tiles/act1/town/floor.dt1`. Sin esto, los ocho DT1 del town "no existen" (medido: la
// primera corrida de esta sonda dio `file not found` en los ocho).
func normalizarDT1(f string) string {
	f = strings.ToLower(f)
	f = strings.ReplaceAll(f, "c:", "")
	f = strings.ReplaceAll(f, ".tg1", ".dt1")

	// el prefijo de la instalacion lo delata la ultima "tiles" de la ruta
	if i := strings.LastIndex(f, "tiles"); i >= 0 {
		f = f[i+len("tiles"):]
	}

	f = strings.TrimPrefix(f, barraInvertida)
	f = strings.TrimPrefix(f, "/")

	// y el ASSET MANAGER del motor le vuelve a poner el prefijo de tiles
	// (`d2core/d2asset/asset_manager.go`: LoadDT1 hace "/data/global/tiles/" + ruta). Medido: sin el
	// prefijo, `act1/town/floor.dt1` da "file not found" y `data/global/tiles/act1/town/floor.dt1` no.
	f = "data/global/tiles/" + f

	return strings.ReplaceAll(f, barraInvertida, "/")
}

// leerMPQ: el MPQ guarda los nombres con barras INVERTIDAS y su lector es literal; el que convierte es la
// cadena de carga del motor (`d2common/d2loader/mpq/source.go`: ReplaceAll("/", barra)). Medido: sin esta
// conversion, los ocho DT1 del town -que ya estaban bien normalizados- seguian dando "file not found".
func leerMPQ(m *d2mpq.MPQ, nombre string) ([]byte, error) {
	return m.ReadFile(strings.ReplaceAll(nombre, "/", barraInvertida))
}

func main() {
	mpqPath := flag.String("mpq", "", "archivo MPQ (por defecto d2data.mpq de la instalacion)")
	ds1s := flag.String("ds1", "", "rutas de .ds1 separadas por coma (con barras invertidas, como las escribe el MPQ)")
	leer := flag.String("leer", "", "rutas sueltas a leer del MPQ, separadas por coma (para saber en QUE MPQ vive un archivo)")
	flag.Parse()

	if *ds1s == "" && *leer == "" {
		fmt.Fprintln(os.Stderr, "uso: sonda-ds1-237 -mpq <d2data.mpq> -ds1 'ruta1.ds1,ruta2.ds1' [-leer 'a,b']")
		os.Exit(2)
	}

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

	// El CRC-32 del ARCHIVO entero, troceado, igual que lo imprime la consola: si un trozo se separa,
	// se ve en cual.
	const trozo = 65536

	listaDT1 := map[string]bool{}

	for _, nombre := range strings.Split(*leer, ",") {
		nombre = strings.TrimSpace(nombre)
		if nombre == "" {
			continue
		}

		raw, err := leerMPQ(m, nombre)
		if err != nil {
			fmt.Printf("SONDA237: LEER %s FALLO: %v\n", nombre, err)
			continue
		}

		fmt.Printf("SONDA237: LEER %s bytes=%d crc32=%d\n", nombre, len(raw), crc32.ChecksumIEEE(raw))
	}

	for _, nombre := range strings.Split(*ds1s, ",") {
		nombre = strings.TrimSpace(nombre)
		if nombre == "" {
			continue
		}

		raw, err := m.ReadFile(nombre)
		if err != nil {
			fmt.Printf("SONDA237: %s FALLO: %v\n", nombre, err)
			continue
		}

		fmt.Printf("SONDA237: DS1 %s\n", nombre)
		fmt.Printf("SONDA237: DS1 bytes=%d crc32=%d\n", len(raw), crc32.ChecksumIEEE(raw))

		for i := 0; i < len(raw); i += trozo {
			fin := i + trozo
			if fin > len(raw) {
				fin = len(raw)
			}

			fmt.Printf("SONDA237: DS1 trozo %d desde=%d bytes=%d crc32=%d\n",
				i/trozo, i, fin-i, crc32.ChecksumIEEE(raw[i:fin]))
		}

		d, err := d2ds1.Unmarshal(raw)
		if err != nil {
			fmt.Printf("SONDA237: DS1 %s PARSE-FALLO: %v\n", nombre, err)
			continue
		}

		fmt.Printf("SONDA237: DS1 version=%d act=%d w=%d h=%d objetos=%d sustituciones=%d files=%d\n",
			d.Version(), d.Act, d.Width(), d.Height(), len(d.Objects), len(d.SubstitutionGroups), len(d.Files))

		// Las capas: el DS1 guarda hasta 2 pisos, 4 paredes, 4 sombras y 4 sustituciones. Se cuentan los
		// tiles NO VACIOS de cada una (Style=0 & Sequence=0 es el vacio del formato).
		cuenta := func(cual string, idx int, l *d2ds1.Layer) {
			if l == nil {
				return
			}

			noVacios := 0

			for y := 0; y < l.Height(); y++ {
				for x := 0; x < l.Width(); x++ {
					t := l.Tile(x, y)
					if t == nil {
						continue
					}

					if t.Style != 0 || t.Sequence != 0 {
						noVacios++
					}
				}
			}

			fmt.Printf("SONDA237: DS1 capa %s%d w=%d h=%d tilesNOvacios=%d\n",
				cual, idx, l.Width(), l.Height(), noVacios)
		}

		for i := 0; i < 2; i++ {
			cuenta("PISO", i+1, d.GetFloor(i))
		}

		for i := 0; i < 4; i++ {
			cuenta("PARED", i+1, d.GetWall(i))
		}

		for i := 0; i < 4; i++ {
			cuenta("SOMBRA", i+1, d.GetShadow(i))
		}

		for i := 0; i < 4; i++ {
			cuenta("SUST", i+1, d.GetSubstitution(i))
		}

		for _, f := range d.Files {
			f = normalizarDT1(f)
			if listaDT1[f] {
				continue
			}

			listaDT1[f] = true
			medirDT1(m, f)
		}
	}
}

// medirDT1: los numeros del DT1 del propio DS1 (cuantos tiles y de que tipo): es lo que despues hay que
// componer en la textura, asi que se mide antes de pintar.
func medirDT1(m *d2mpq.MPQ, nombre string) {
	raw, err := leerMPQ(m, nombre)
	if err != nil {
		fmt.Printf("SONDA237: DT1 %s FALLO: %v\n", nombre, err)
		return
	}

	dt1, err := d2dt1.LoadDT1(raw)
	if err != nil {
		fmt.Printf("SONDA237: DT1 %s bytes=%d crc32=%d PARSE-FALLO: %v\n",
			nombre, len(raw), crc32.ChecksumIEEE(raw), err)
		return
	}

	porTipo := map[int32]int{}

	minW, minH := int32(1000), int32(1000)
	maxW, maxH := int32(0), int32(0)

	for i := range dt1.Tiles {
		t := &dt1.Tiles[i]
		porTipo[t.Type]++

		if t.Width < minW {
			minW = t.Width
		}

		if t.Height < minH {
			minH = t.Height
		}

		if t.Width > maxW {
			maxW = t.Width
		}

		if t.Height > maxH {
			maxH = t.Height
		}
	}

	fmt.Printf("SONDA237: DT1 %s bytes=%d crc32=%d tiles=%d tipos=%v w=%d..%d h=%d..%d\n",
		nombre, len(raw), crc32.ChecksumIEEE(raw), len(dt1.Tiles), porTipo, minW, maxW, minH, maxH)
}

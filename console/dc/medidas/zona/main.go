// Medidor de zona: cuanto pesa una zona del juego (el .ds1 + los .dt1 que el mismo lista).
//
// Uso:
//
//	medidor-zona <d2data.mpq> -ds1 'data\global\tiles\ACT1\TOWN\townN1.ds1,...'
//	medidor-zona <d2data.mpq> -listar 'tiles\ACT1' -ext .ds1     (que hay en la tabla del MPQ)
//
// EL ARREGLO DE DC-002.77 (esto es lo que la tarjeta pedia). Esta sonda probaba el nombre del .ds1
// TAL CUAL (y dos variantes de mayusculas) y los ocho archivos de tile daban FALTA. La causa no es la
// tabla del MPQ: es que el .ds1 guarda la ruta del DISCO DE INSTALACION del juego, que no es la ruta
// del MPQ. La traduccion es un campo, y la hace el motor en `MapEngine.AddDS1`:
//   - el .ds1 dice `\d2\data\global\tiles\act1\town\floor.tg1`
//   - en el MPQ el archivo se llama `data\global\tiles\ACT1\TOWN\floor.dt1`
// o sea: se baja a minusculas, se le saca el prefijo de la instalacion (todo lo que hay hasta la ultima
// "tiles"), y **la extension .tg1 es .dt1** (el contenido es un DT1 en los dos casos; .tg1 es el nombre
// viejo de los archivos de tiles). Ademas el lector del MPQ es LITERAL con las barras invertidas.
//
// Los tamanos que informa son dos:
//   - crudo: el archivo tal como sale del MPQ (lo que se lee del disco);
//   - vivo: lo que queda en RAM despues del parseo (los bloques EncodedData de cada tile, que es lo que
//     el cache de dt1s cuenta con `d2asset.TamanoDT1`). El .ds1, las cuatro capas de Tile por celda.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unsafe"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2ds1"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2dt1"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2mpq"
)

// barraInvertida: el separador de path del MPQ, armado por su codigo (92) para no depender de como se
// escapa una barra invertida en una cadena.
const barraInvertida = string(rune(92))

// autodetectarD2: la instalacion del juego no vive en el repo.
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

// RutaDelDT1: la MISMA traduccion que el motor (`MapEngine.AddDS1` -> la ruta que despues pide
// `LoadDT1` con el prefijo "/data/global/tiles/"). Copiada a proposito: aca se mide el nombre que va a
// usar el motor, no uno inventado por la sonda.
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

// LeerDelMPQ: el lector del MPQ es literal con las barras invertidas.
func LeerDelMPQ(m *d2mpq.MPQ, nombre string) ([]byte, error) {
	return m.ReadFile(strings.ReplaceAll(nombre, "/", barraInvertida))
}

func main() {
	mpqPath := flag.String("mpq", "", "archivo MPQ (por defecto d2data.mpq de la instalacion)")
	ds1s := flag.String("ds1", "", "rutas de .ds1 separadas por coma, con barras invertidas como las escribe el MPQ")
	listar := flag.String("listar", "", "filtro (subcadena, sin distinguir mayusculas) para LISTAR la tabla del MPQ y salir")
	ext := flag.String("ext", "", "con -listar: quedarse solo con esta extension (ej. .ds1)")
	flag.Parse()

	if *ds1s == "" && *listar == "" {
		fmt.Fprintln(os.Stderr, "uso: medidor-zona <d2data.mpq> -ds1 'ruta1.ds1,ruta2.ds1' [-listar filtro [-ext .ds1]]")
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

	if *listar != "" {
		listarTabla(m, *listar, *ext)
		return
	}

	vistosDT1 := map[string]int{}

	for _, nombre := range strings.Split(*ds1s, ",") {
		nombre = strings.TrimSpace(nombre)
		if nombre == "" {
			continue
		}

		medirZona(m, nombre, vistosDT1)
	}

	if len(vistosDT1) > 0 {
		fmt.Println()
		fmt.Println("== .dt1 repetidos entre las zonas medidas (se puede reusar el del cache)")
		for nombre, veces := range vistosDT1 {
			if veces > 1 {
				fmt.Printf("   %-52s en %d zonas\n", nombre, veces)
			}
		}
	}
}

// listarTabla: la tabla del MPQ es la unica verdad sobre como se llaman los archivos. Es lo que faltaba
// en la tarjeta anterior: se probaban nombres supuestos en vez de mirar la tabla.
func listarTabla(m *d2mpq.MPQ, filtro, ext string) {
	todos, err := m.Listfile()
	if err != nil {
		fmt.Println("LISTFILE FALLO:", err)
		os.Exit(1)
	}

	filtro = strings.ToLower(filtro)
	ext = strings.ToLower(ext)

	var salida []string

	for _, n := range todos {
		bajo := strings.ToLower(n)
		if !strings.Contains(bajo, filtro) {
			continue
		}

		if ext != "" && !strings.HasSuffix(bajo, ext) {
			continue
		}

		salida = append(salida, n)
	}

	fmt.Printf("LISTA filtro=%q ext=%q entradas=%d de %d\n", filtro, ext, len(salida), len(todos))

	for _, n := range salida {
		fmt.Println("  ", n)
	}
}

func medirZona(m *d2mpq.MPQ, nombre string, vistos map[string]int) {
	raw, err := m.ReadFile(nombre)
	if err != nil {
		fmt.Printf("ZONA %s FALLO: %v\n", nombre, err)
		return
	}

	d, err := d2ds1.Unmarshal(raw)
	if err != nil {
		fmt.Printf("ZONA %s PARSE-FALLO: %v\n", nombre, err)
		return
	}

	vivoDS1 := tamanoDS1(d)

	fmt.Printf("== ZONA %s\n", nombre)
	fmt.Printf("   ds1  w=%d h=%d acto=%d objetos=%d archivos=%d  %d B crudo  %d B vivos\n",
		d.Width(), d.Height(), d.Act, len(d.Objects), len(d.Files), len(raw), vivoDS1)

	totalCrudo, totalVivo := 0, 0
	repes := 0

	for _, f := range d.Files {
		r := RutaDelDT1(f)
		// para leer, barras invertidas (el MPQ es literal); para mostrar, barras normales
		crudo, err := LeerDelMPQ(m, r)
		if err != nil {
			fmt.Printf("   FALTA %-50s (el .ds1 dice %s)\n", strings.ReplaceAll(r, barraInvertida, "/"), f)
			continue
		}

		mostrado := strings.ReplaceAll(r, barraInvertida, "/")

		dt1, err := d2dt1.LoadDT1(crudo)
		if err != nil {
			fmt.Printf("   %-50s %8d B crudo  PARSE-FALLO: %v\n", mostrado, len(crudo), err)
			continue
		}

		vivo := tamanoDT1(dt1)
		totalCrudo += len(crudo)
		totalVivo += vivo

		vistos[r]++
		nota := ""
		if vistos[r] > 1 {
			nota = "  (repetido)"
			repes++
		}

		fmt.Printf("   %-50s %8d B crudo  %8d B codificado  %5d tiles%s\n", mostrado, len(crudo), vivo, len(dt1.Tiles), nota)
	}

	fmt.Printf("   TOTAL dt1 %d B crudos, %d B codificados; zona completa %d B (%.2f MB) leidos, %d B vivos\n",
		totalCrudo, totalVivo, len(raw)+totalCrudo, float64(len(raw)+totalCrudo)/1048576.0, vivoDS1+totalVivo)

	if repes > 0 {
		fmt.Printf("   (%d de los archivos de tile ya estaban en otra zona medida)\n", repes)
	}

	fmt.Println()
}

func tamanoDS1(d *d2ds1.DS1) int {
	celdas := d.Width() * d.Height()
	tamTile := int(unsafe.Sizeof(d2ds1.Tile{}))

	return 4*celdas*tamTile + len(d.Files)*32 + len(d.Objects)*20 + len(d.SubstitutionGroups)*20
}

func tamanoDT1(d *d2dt1.DT1) int {
	total := 0

	for i := range d.Tiles {
		for j := range d.Tiles[i].Blocks {
			total += len(d.Tiles[i].Blocks[j].EncodedData)
		}
	}

	return total
}

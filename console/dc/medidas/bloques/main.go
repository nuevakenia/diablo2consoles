// Medidor de BLOQUES: cuanto mide cada bloque codificado de un .dt1, y cuantos se pasan del umbral
// de objeto grande del recolector de la consola (64 KB).
//
// POR QUE EXISTE (DC-002.78, Fase B.1e). La tarjeta pide que la carga de una zona NO pida memoria por
// archivo. El camino elegido (copiar los bloques del .dt1 al parsear para poder soltar el crudo) tiene
// UNA condicion medible: si la copia de cada bloque queda por debajo de `GC_LARGE_OBJECT_THRESHOLD`
// (64 KB, godc_config.h:12) el recolector la puede devolver; si la pasa, se va al monton externo
// (`gc_external_alloc`, malloc pelado) y no la devuelve nadie -- exactamente el problema que se quiere
// arreglar. Este medidor da ese numero en el HOST, sobre los .dt1 reales de las dos zonas del acto 1,
// antes de gastar una corrida en Flycast.
//
// Uso:
//
//	go run ./console/dc/medidas/bloques -ds1 'data\global\tiles\ACT1\TOWN\townN1.ds1,...'
//	go run ./console/dc/medidas/bloques            (por defecto: las dos zonas de DC-002.77)
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2ds1"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2dt1"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2mpq"
)

// umbralDeObjetoGrande: GC_LARGE_OBJECT_THRESHOLD_KB = 64 (libgodc/runtime/godc_config.h:12). Un pedido
// de mas de 64 KB (65536 B) se va a gc_external_alloc, que es malloc pelado y no lo recolecta nadie.
const umbralDeObjetoGrande = 64 * 1024

const barraInvertida = string(rune(92))

func main() {
	ds1s := flag.String("ds1", `data\global\tiles\ACT1\TOWN\townN1.ds1,`+
		`data\global\tiles\ACT1\OUTDOORS\wild1.ds1`, "zonas separadas por coma (ruta tal como la escribe el .ds1)")
	mpqPath := flag.String("mpq", "", "d2data.mpq (por defecto, el de la instalacion)")
	flag.Parse()

	ruta := *mpqPath
	if ruta == "" {
		dir := autodetectarD2()
		if dir == "" {
			fmt.Println("no encuentro d2data.mpq: pasa -mpq o define D2_DIR")
			os.Exit(1)
		}

		ruta = filepath.Join(dir, "d2data.mpq")
	}

	m, err := d2mpq.FromFile(ruta)
	if err != nil {
		fmt.Println("MPQ-ABRIR FALLO:", err)
		os.Exit(1)
	}

	fmt.Println("BLOQUES: umbral de objeto grande del recolector =", umbralDeObjetoGrande, "B")

	totalBloques := 0
	totalBytes := 0
	sePasan := 0
	masGrande := 0
	var todos []int

	for _, zona := range strings.Split(*ds1s, ",") {
		zona = strings.TrimSpace(zona)
		if zona == "" {
			continue
		}

		raw, err := LeerDelMPQ(m, zona)
		if err != nil {
			fmt.Println("DS1-LEER FALLO:", zona, err)
			continue
		}

		d, err := d2ds1.Unmarshal(raw)
		if err != nil {
			fmt.Println("DS1-PARSE FALLO:", zona, err)
			continue
		}

		fmt.Println("")
		fmt.Println("ZONA", zona, "  celdas", d.Width(), "x", d.Height(), " archivos", len(d.Files))

		for _, nombre := range d.Files {
			clave := RutaDelDT1(nombre)

			crudo, err := LeerDelMPQ(m, clave)
			if err != nil {
				fmt.Println("   DT1-LEER FALLO:", clave, err)
				continue
			}

			dt1, err := d2dt1.LoadDT1(crudo)
			if err != nil {
				fmt.Println("   DT1-PARSE FALLO:", clave, err)
				continue
			}

			bloques, bytes, maximo, grandes := censar(dt1, &todos)

			totalBloques += bloques
			totalBytes += bytes
			sePasan += grandes

			if maximo > masGrande {
				masGrande = maximo
			}

			fmt.Printf("   %-40s crudo=%9d bloques=%5d sumaBloques=%9d maxBloque=%7d >64KB=%d tiles=%d\n",
				clave, len(crudo), bloques, bytes, maximo, grandes, len(dt1.Tiles))
		}
	}

	fmt.Println("")
	fmt.Println("RESUMEN: bloques =", totalBloques, " bytes copiados =", totalBytes,
		" bloque mas grande =", masGrande, " bloques > 64 KB =", sePasan)

	if sePasan > 0 {
		fmt.Println("VEREDICTO: HAY bloques de mas de 64 KB -- copiarlos NO queda en el semiespacio: quedan en el monton externo")
	} else {
		fmt.Println("VEREDICTO: ningun bloque pasa de 64 KB -- la copia de cada bloque queda en el semiespacio del recolector")
	}

	sort.Ints(todos)
	if len(todos) > 0 {
		fmt.Println("HISTOGRAMA (percentiles de longitud de bloque):",
			"p50 =", percentil(todos, 50), " p90 =", percentil(todos, 90),
			" p99 =", percentil(todos, 99), " p100 =", todos[len(todos)-1])
	}
}

// censar: recorre los bloques de un .dt1 y devuelve cuantos son, cuanto suman, el mas grande y cuantos
// pasan el umbral. Va llenando la lista global para el histograma.
func censar(dt1 *d2dt1.DT1, todos *[]int) (bloques, bytes, maximo, grandes int) {
	for i := range dt1.Tiles {
		tile := &dt1.Tiles[i]
		for j := range tile.Blocks {
			n := len(tile.Blocks[j].EncodedData)
			bloques++
			bytes += n

			if n > maximo {
				maximo = n
			}

			if n > umbralDeObjetoGrande {
				grandes++
			}

			*todos = append(*todos, n)
		}
	}

	return bloques, bytes, maximo, grandes
}

func percentil(ordenados []int, p int) int {
	i := len(ordenados) * p / 100
	if i >= len(ordenados) {
		i = len(ordenados) - 1
	}

	return ordenados[i]
}

// RutaDelDT1: la MISMA traduccion que el motor (MapEngine.AddDS1 -> LoadDT1).
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

func autodetectarD2() string {
	if d := os.Getenv("D2_DIR"); d != "" {
		return d
	}

	c := filepath.Join(os.Getenv("HOME"), ".wine-d2", "drive_c", "Games", "Diablo II")
	if st, err := os.Stat(filepath.Join(c, "d2data.mpq")); err == nil && !st.IsDir() {
		return c
	}

	return ""
}

// Medidor DT1-CRC: el crc32 de cada .dt1 de una zona, medido en el HOST (DC-002.79).
//
// POR QUE EXISTE. La tarjeta pide que un POC en Flycast lea los ocho .dt1 de una zona por el camino del
// MOTOR (`LoadFile`/`LoadAsset`) y que el contenido sea el mismo que en la PC: "crc32 iguales a los del
// host". Este medidor da ese numero en el host, con el MISMO lector del MPQ, para que la comparacion sea
// contra un valor medido y no contra una copia de la consola.
//
// Ademas informa, por archivo, el tamano DESCOMPRIMIDO (lo que hay que reservar para leerlo) y el
// COMPRIMIDO (lo que el stream del MPQ prefetchea de un tiron: memoria tomada por archivo, ver
// `d2common/d2fileformats/d2mpq/lectura_entera.go`).
//
// Uso:
//
//	go run ./console/dc/medidas/dt1-crc
//	go run ./console/dc/medidas/dt1-crc -ds1 'data\global\tiles\ACT1\OUTDOORS\wild1.ds1'
package main

import (
	"bytes"
	"flag"
	"fmt"
	"hash/crc32"
	"os"
	"path/filepath"
	"strings"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2ds1"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2mpq"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2loader/asset"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2loader/mpq"
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

// RutaDelDT1: la MISMA traduccion que el motor (`MapEngine.AddDS1`). Copiada a proposito: aca se mide el
// nombre que va a usar el motor, no uno inventado por la sonda.
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

func main() {
	mpqPath := flag.String("mpq", "", "archivo MPQ (por defecto d2data.mpq de la instalacion)")
	ds1s := flag.String("ds1", "", "zonas separadas por coma (por defecto, el pueblo del acto 1)")
	viaCargador := flag.Bool("via-cargador", false,
		"ademas, leer cada .dt1 por d2loader (TamanoDeArchivo + LeerEnteroEn) reusando UN buffer")
	flag.Parse()

	// la ruta por defecto se arma con la barra del MPQ (92) para no pelear con el escapado de las comillas
	if *ds1s == "" {
		*ds1s = "data" + barraInvertida + "global" + barraInvertida + "tiles" + barraInvertida + "ACT1" +
			barraInvertida + "TOWN" + barraInvertida + "townN1.ds1"
	}

	ruta := *mpqPath
	if ruta == "" {
		carpeta := autodetectarD2()
		if carpeta == "" {
			fmt.Println("no encuentro d2data.mpq: pasa -mpq o define D2_DIR")
			os.Exit(1)
		}

		ruta = filepath.Join(carpeta, "d2data.mpq")
	}

	m, err := d2mpq.FromFile(ruta)
	if err != nil {
		fmt.Println("MPQ-ABRIR FALLO:", err)
		os.Exit(1)
	}
	defer m.Close()

	for _, zona := range strings.Split(*ds1s, ",") {
		zona = strings.TrimSpace(zona)
		if zona == "" {
			continue
		}

		medirZona(m, zona, *viaCargador, ruta)
	}
}

// fuenteDePrueba: la MISMA fuente que arma el loader cuando se le agrega un MPQ (`mpq.NewSource`), para
// comparar el camino de la consola (`Tamano` + `LeerEnteroEn`) contra `ReadFile` en el host.
//
// No se arma el `d2loader.Loader` entero a proposito: ese paquete arrastra `d2cache` -> `d2util` -> ebiten,
// cuyo arranque de GLFW revienta sin pantalla ("X11: The DISPLAY environment variable is missing"), y este
// medidor tiene que correr en una terminal sin ventana. La fuente es el mismo objeto: el loader no hace mas
// que limpiar la ruta y recorrer sus fuentes.
func fuenteDePrueba(rutaMPQ string, aviso *string) asset.SourceDeLecturaEntera {
	fuente, err := mpq.NewSource(rutaMPQ)
	if err != nil {
		*aviso = "no se pudo abrir el MPQ: " + err.Error()

		return nil
	}

	lectora, ok := fuente.(asset.SourceDeLecturaEntera)
	if !ok {
		*aviso = "la fuente MPQ no implementa asset.SourceDeLecturaEntera"

		return nil
	}

	return lectora
}

func medirZona(m *d2mpq.MPQ, zona string, viaCargador bool, rutaMPQ string) {
	raw, err := m.ReadFile(zona)
	if err != nil {
		fmt.Println("DS1-LEER FALLO:", zona, err)
		return
	}

	d, err := d2ds1.Unmarshal(raw)
	if err != nil {
		fmt.Println("DS1-PARSE FALLO:", zona, err)
		return
	}

	fmt.Printf("DT1-CRC: zona=%s ds1=%d B archivos=%d\n", zona, len(raw), len(d.Files))

	var cargador asset.SourceDeLecturaEntera

	if viaCargador {
		var aviso string

		cargador = fuenteDePrueba(rutaMPQ, &aviso)
		if cargador == nil {
			fmt.Println("CARGADOR FALLO:", aviso)
		}
	}

	// el buffer UNICO que se reusa entre archivos (lo mismo que hace la consola con `LoadFileEn`)
	var reuso []byte

	totalCrudo, totalComprimido := 0, 0

	for _, f := range d.Files {
		nombre := RutaDelDT1(f)
		// para leer, barras invertidas (el MPQ es literal)
		crudo, err := m.ReadFile(strings.ReplaceAll(nombre, "/", barraInvertida))
		if err != nil {
			fmt.Printf("HOST %-46s FALLO: %v\n", nombre, err)
			continue
		}

		descomprimido, hayTamano := m.TamanoDeArchivo(strings.ReplaceAll(nombre, "/", barraInvertida))
		comprimido, hayComprimido := m.TamanoComprimido(strings.ReplaceAll(nombre, "/", barraInvertida))

		if !hayTamano || !hayComprimido {
			fmt.Printf("HOST %-46s SIN-TAMANO\n", nombre)
		}

		if descomprimido != len(crudo) {
			fmt.Printf("HOST %-46s TAMANO-DISTINTO tabla=%d leidos=%d\n", nombre, descomprimido, len(crudo))
		}

		fmt.Printf("HOST %-46s crudo=%7d comprimido=%7d crc32=%d\n",
			nombre, len(crudo), comprimido, crc32.ChecksumIEEE(crudo))

		if cargador != nil {
			compararPorElCargador(cargador, nombre, crudo, &reuso)
		}

		totalCrudo += len(crudo)
		totalComprimido += comprimido
	}

	fmt.Printf("DT1-CRC: zona=%s TOTAL crudo=%d comprimido=%d\n", zona, totalCrudo, totalComprimido)
}

// compararPorElCargador lee el archivo por el camino de la consola (`d2loader.TamanoDeArchivo` +
// `d2loader.LeerEnteroEn`, con UN buffer reusado) y compara contra lo que dio `ReadFile`.
func compararPorElCargador(cargador asset.SourceDeLecturaEntera, nombre string, quiero []byte, reuso *[]byte) {
	antes := cap(*reuso)

	tamano, hay := cargador.Tamano(nombre)
	if !hay {
		fmt.Printf("CARGADOR %-43s SIN-TAMANO\n", nombre)

		return
	}

	if tamano > cap(*reuso) {
		*reuso = make([]byte, tamano)
	}

	leidos, err := cargador.LeerEnteroEn(nombre, (*reuso)[:tamano])
	if err != nil {
		fmt.Printf("CARGADOR %-43s FALLO: %v\n", nombre, err)

		return
	}

	iguales := leidos == len(quiero) && bytes.Equal((*reuso)[:leidos], quiero)

	fmt.Printf("CARGADOR %-43s bytes=%7d crc32=%d igual=%v buffer=%d->%d\n",
		nombre, leidos, crc32.ChecksumIEEE((*reuso)[:leidos]), iguales, antes, cap(*reuso))
}

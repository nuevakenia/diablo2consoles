// Medidor de los PRESUPUESTOS de cache (DC-002.75): cuanto pesa de verdad lo que el AssetManager
// mete en sus caches, para elegir los numeros de la consola MIDIENDO y no a ojo.
//
// Corre en el HOST (no en la consola). Uso:
//
//	go run ./console/dc/medidas/presupuestos "$HOME/.wine-d2/drive_c/Games/Diablo II/d2data.mpq" \
//	    "$HOME/.wine-d2/drive_c/Games/Diablo II/d2char.mpq"
//
// Mide dos escenarios, que son los del POC del motor (console/dc/poc/estado-b):
//   - una zona: el .ds1 del acto 1 y los .dt1 que el mismo lista (cache dt1s + ds1s)
//   - un personaje: su COF y las capas .dcc/.dc6 que pide (caches cofs + dccs + animations)
//
// El "peso" que se informa es el TAMANO EN BYTES de lo que queda vivo en el cache: el archivo crudo
// mas los buffers decodificados que el parser reserva (pixeles del DCC, datos codificados del DT1).
package main

import (
	"fmt"
	"os"
	"strings"
	"unsafe"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2cof"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2dc6"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2dcc"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2ds1"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2dt1"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2mpq"
)

const (
	rutaNivel = `data\global\tiles\ACT1\TOWN\townN1.ds1`
	rutaCOF   = `data\global\CHARS\AM\COF\amnuhth.cof`

	// las mismas piezas de nombre que console/dc/poc/personaje-mapa/personaje.go
	tokenDelPersonaje = "AM"
	modoDelPersonaje  = "NU"
	valorDeLaCapa     = "LIT"
)

func main() {
	if len(os.Args) < 3 {
		fmt.Println("uso: presupuestos <d2data.mpq> <d2char.mpq>")
		os.Exit(2)
	}

	mData, err := d2mpq.FromFile(os.Args[1])
	must(err, "d2data.mpq")

	mChars, err := d2mpq.FromFile(os.Args[2])
	must(err, "d2char.mpq")

	medirZona(mData)
	medirPersonaje(mChars)
}

func must(err error, que string) {
	if err != nil {
		fmt.Println("FALLO", que, ":", err)
		os.Exit(1)
	}
}

// ============================ la zona ============================

func medirZona(m *d2mpq.MPQ) {
	rawDS1, err := m.ReadFile(rutaNivel)
	must(err, "el .ds1 de la zona")

	ds1, err := d2ds1.Unmarshal(rawDS1)
	must(err, "el parseo del .ds1")

	pesoDS1 := tamanoDS1(ds1)

	fmt.Println("== ZONA (acto 1, townN1)")
	fmt.Printf("   %-44s %8d B crudo  %8d B en memoria (%dx%d celdas)\n",
		"townN1.ds1", len(rawDS1), pesoDS1, ds1.Width(), ds1.Height())

	totalCrudo, totalMemoria := 0, 0

	for _, f := range ds1.Files {
		nombre := rutaDelDT1(f)

		raw, err := leer(m, nombre)
		if err != nil {
			fmt.Printf("   %-44s FALTA (%v)\n", nombre, err)
			continue
		}

		dt1, err := d2dt1.LoadDT1(raw)
		must(err, "el parseo de "+nombre)

		bloques := tamanoDT1(dt1)
		totalCrudo += len(raw)
		totalMemoria += bloques

		fmt.Printf("   %-44s %8d B crudo  %8d B codificado  %5d tiles\n",
			nombre, len(raw), bloques, len(dt1.Tiles))
	}

	fmt.Printf("   TOTAL .dt1: %d B crudos, %d B lo que queda vivo\n", totalCrudo, totalMemoria)
	fmt.Printf("   TOTAL DE LA ZONA en los caches: dt1s %d B + ds1s %d B = %d B (%.2f MB)\n\n",
		totalMemoria, pesoDS1, totalMemoria+pesoDS1,
		float64(totalMemoria+pesoDS1)/1048576.0)
}

// ============================ el personaje ============================

func medirPersonaje(m *d2mpq.MPQ) {
	rawCOF, err := m.ReadFile(rutaCOF)
	must(err, "el .cof")

	cof, err := d2cof.Unmarshal(rawCOF)
	must(err, "el parseo del .cof")

	fmt.Println("== PERSONAJE (AM + NU + HTH, la amazona parada)")
	fmt.Printf("   amnuhth.cof %d B crudo; %d capas, %d frames por direccion, %d direcciones\n",
		len(rawCOF), cof.NumberOfLayers, cof.FramesPerDirection, cof.NumberOfDirections)
	fmt.Printf("   peso del COF cacheado (crudo + lo parseado): %d B\n", len(rawCOF)+tamanoCOF(cof))

	totalVivo, totalBytes := 0, 0

	for i := range cof.CofLayers {
		clave := cof.CofLayers[i].Type.String()
		arma := strings.ToLower(cof.CofLayers[i].WeaponClass.String())
		base := "data/global/chars/" + tokenDelPersonaje + "/" + clave + "/" +
			tokenDelPersonaje + clave + valorDeLaCapa + modoDelPersonaje + arma

		encontrada := false

		for _, ext := range []string{"dcc", "dc6"} {
			nombre := base + "." + ext

			raw, err := leer(m, nombre)
			if err != nil {
				continue
			}

			encontrada = true

			if ext == "dcc" {
				d, err := d2dcc.Load(raw)
				must(err, "el parseo de "+nombre)

				peso := tamanoDCC(raw, d)
				totalVivo += peso
				totalBytes += len(raw)

				fmt.Printf("   capa %-4s %-38s %8d B crudo  %9d B con pixeles\n", clave, nombre, len(raw), peso)
			} else {
				d, err := d2dc6.Load(raw)
				must(err, "el parseo de "+nombre)

				peso := tamanoDC6(d)
				totalVivo += peso
				totalBytes += len(raw)

				fmt.Printf("   capa %-4s %-38s %8d B crudo  %9d B con frames\n", clave, nombre, len(raw), peso)
			}

			break
		}

		if !encontrada {
			fmt.Printf("   capa %-4s SIN-GRAFICO en el MPQ (el limite conocido de DC-002.38)\n", clave)
		}
	}

	fmt.Printf("   TOTAL del personaje: %d B crudos, %d B lo que queda vivo (%.2f MB)\n",
		totalBytes, totalVivo, float64(totalVivo)/1048576.0)
	fmt.Println("   (el Animation de cada capa pesa lo mismo que su DCC/DC6: la capa esta UNA vez)")
}

// ============================ tamanos ============================

func tamanoDS1(d *d2ds1.DS1) int {
	// las cuatro capas (piso, pared, sombra, sustitucion) reservan un Tile por celda
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

func tamanoCOF(c *d2cof.COF) int { return len(c.CofLayers) * 32 }

func tamanoDCC(raw []byte, d *d2dcc.DCC) int {
	total := len(raw)

	for _, dir := range d.Directions {
		if dir == nil {
			continue
		}

		total += len(dir.PixelData)

		for i := range dir.Frames {
			total += len(dir.Frames[i].PixelData)
		}
	}

	return total
}

func tamanoDC6(d *d2dc6.DC6) int {
	total := len(d.FramePointers) * 4

	for _, f := range d.Frames {
		if f == nil {
			continue
		}

		total += len(f.FrameData) + 32
	}

	return total
}

// ============================ rutas ============================

// rutaDelDT1 es la MISMA normalizacion del POC (nivel.go:RutaDelDT1): el .ds1 guarda la ruta del
// disco con "c:" y ".tg1"; el MPQ la tiene como .dt1 bajo data/global/tiles.
func rutaDelDT1(f string) string {
	f = strings.ToLower(f)
	f = strings.ReplaceAll(f, "c:", "")
	f = strings.ReplaceAll(f, ".tg1", ".dt1")

	if i := strings.LastIndex(f, "tiles"); i >= 0 {
		f = f[i+len("tiles"):]
	}

	f = strings.TrimPrefix(f, `\`)
	f = strings.TrimPrefix(f, "/")

	return "data/global/tiles/" + f
}

// leer: el lector del MPQ es literal con las barras invertidas.
func leer(m *d2mpq.MPQ, nombre string) ([]byte, error) {
	return m.ReadFile(strings.ReplaceAll(nombre, "/", `\`))
}

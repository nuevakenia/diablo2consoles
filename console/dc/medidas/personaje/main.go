// Medidor: cuanto pesa un personaje del motor (amazona sin equipo) en RAM.
// Lee el COF del MPQ y reporta direcciones, frames por direccion y capas.
// No toca el repo: corre desde /tmp/medida con replace al repo.
package main

import (
	"fmt"
	"os"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2cof"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2mpq"
)

func main() {
	mpqPath := os.Args[1]
	cofPath := os.Args[2]

	m, err := d2mpq.FromFile(mpqPath)
	if err != nil {
		fmt.Println("MPQ-ABRIR FALLO:", err)
		os.Exit(1)
	}
	raw, err := m.ReadFile(cofPath)
	if err != nil {
		fmt.Println("COF-LEER FALLO:", err)
		os.Exit(1)
	}
	cof := &d2cof.COF{}
	if err := cof.Unmarshal(raw); err != nil {
		fmt.Println("COF-PARSE FALLO:", err)
		os.Exit(1)
	}

	fmt.Printf("COF                %s\n", cofPath)
	fmt.Printf("bytes del COF      %d\n", len(raw))
	fmt.Printf("direcciones        %d\n", cof.NumberOfDirections)
	fmt.Printf("frames/direccion   %d\n", cof.FramesPerDirection)
	fmt.Printf("capas (declaradas) %d   (CofLayers: %d)\n", cof.NumberOfLayers, len(cof.CofLayers))
	fmt.Printf("velocidad          %d\n", cof.Speed)

	frames := cof.NumberOfDirections * cof.FramesPerDirection
	fmt.Printf("frames totales     %d\n", frames)

	// Costo de memoria: cada frame es 1 byte por pixel (8 bits de indice, el indice 0 es transparente).
	// El POC midio el compuesto del cuerpo en 8216 B (console/dc/evidencia/dc-002.56-vram-preinit.txt:172).
	for _, px := range []int{8216, 12000, 16000} {
		total := frames * px
		fmt.Printf("si cada frame compuesto pesara %6d B -> cachear TODO el COF = %.2f MB\n", px, float64(total)/1048576.0)
	}
	fmt.Println()
	fmt.Println("capas del COF (type / shadow / selectable / transparent):")
	for i, l := range cof.CofLayers {
		fmt.Printf("  %2d  type=%-3d shadow=%d sel=%-5v transp=%-5v weaponClass=%d\n",
			i, int(l.Type), l.Shadow, l.Selectable, l.Transparent, int(l.WeaponClass))
	}
}

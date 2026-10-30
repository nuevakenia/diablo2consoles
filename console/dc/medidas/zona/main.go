// Medidor de zona: cuanto pesa una zona del juego (el .ds1 + los .dt1 que el mismo lista).
// Uso: ./medidor-zona <d2data.mpq> 'data\global\tiles\ACT1\TOWN\townN1.ds1'
package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2ds1"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2mpq"
)

func main() {
	m, err := d2mpq.FromFile(os.Args[1])
	if err != nil {
		fmt.Println("MPQ FALLO:", err)
		os.Exit(1)
	}
	raw, err := m.ReadFile(os.Args[2])
	if err != nil {
		fmt.Println("DS1 FALLO:", err)
		os.Exit(1)
	}
	ds1, err := d2ds1.Unmarshal(raw)
	if err != nil {
		fmt.Println("DS1 PARSE FALLO:", err)
		os.Exit(1)
	}
	fmt.Printf("DS1              %s\n", os.Args[2])
	fmt.Printf("bytes del .ds1   %d\n", len(raw))
	fmt.Printf("acto             %d\n", ds1.Act)
	fmt.Printf("archivos de tile %d\n", len(ds1.Files))
	total := 0
	for _, f := range ds1.Files {
		// el .ds1 guarda las rutas con el prefijo del disco y en minusculas; el lector del MPQ
		// quiere el nombre tal cual esta en la tabla: se prueban las variantes razonables.
		cands := []string{
			f,
			strings.TrimPrefix(f, `\d2\`),
			strings.ToUpper(strings.TrimPrefix(f, `\d2\`)),
		}
		var b []byte
		var err error
		for _, c := range cands {
			b, err = m.ReadFile(c)
			if err == nil && len(b) > 0 {
				break
			}
		}
		if err != nil || len(b) == 0 {
			fmt.Printf("  FALTA          %s\n", f)
			continue
		}
		total += len(b)
		fmt.Printf("  %8d B  %s\n", len(b), f)
	}
	fmt.Printf("\nTOTAL de la zona: .ds1 %d + .dt1 %d = %d B (%.2f MB)\n",
		len(raw), total, len(raw)+total, float64(len(raw)+total)/1048576.0)
}

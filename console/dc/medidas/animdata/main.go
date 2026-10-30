// Medidor de animdata.d2 para la consola (DC-002.76): cuanto ocupa en memoria la tabla de animaciones
// que el Composite NECESITA para poder armar un modo (createMode busca la animacion por token, si no
// esta devuelve "could not find Animation data" y la capa no se carga).
//
// Corre en el HOST. Uso:
//
//	go run ./console/dc/medidas/animdata "$HOME/.wine-d2/drive_c/Games/Diablo II/d2data.mpq"
package main

import (
	"fmt"
	"os"
	"runtime"
	"unsafe"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2animdata"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2mpq"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("uso: animdata <d2data.mpq | animdata.d2>")
		os.Exit(2)
	}

	crudo := leer(os.Args[1])

	runtime.GC()

	var antes, despues runtime.MemStats
	runtime.ReadMemStats(&antes)

	ad, err := d2animdata.Load(crudo)
	if err != nil {
		fmt.Println("FALLO el parseo:", err)
		os.Exit(1)
	}

	runtime.GC()
	runtime.ReadMemStats(&despues)

	fmt.Println("animdata.d2 crudo ........", len(crudo), "B")
	fmt.Println("registros ................", ad.GetRecordsCount())
	fmt.Println("vivo despues de parsear ..", int64(despues.HeapAlloc)-int64(antes.HeapAlloc), "B (HeapAlloc, medidas en el host)")
	fmt.Println("tamano del struct ........", unsafe.Sizeof(*ad), "B")

	// el registro que busca el Composite de la amazona (AMNU... en mayusculas, sin direccion)
	for _, clave := range []string{"AMNUHTH", "AMNUlit", "AMWLHTH"} {
		recs := ad.GetRecords(clave)
		fmt.Printf("GetRecords(%q) ..... %d registros\n", clave, len(recs))

		for i, r := range recs {
			if i > 2 {
				break
			}

			fmt.Printf("   frames=%d velocidad=%d\n", r.FramesPerDirection(), r.Speed())
		}
	}
}

func leer(ruta string) []byte {
	if datos, err := os.ReadFile(ruta); err == nil && len(datos) > 0 && ruta[len(ruta)-4:] == ".d2 " {
		return datos
	}

	m, err := d2mpq.FromFile(ruta)
	if err != nil {
		fmt.Println("FALLO el MPQ:", err)
		os.Exit(1)
	}

	archivo, err := m.ReadFile(`data\global\animdata.d2`)
	if err != nil {
		fmt.Println("FALLO la lectura de animdata.d2:", err)
		os.Exit(1)
	}

	return archivo
}

// DC-002.48 (instrumento del HOST): cuanto pesa VIVO cada tabla del arranque, una por una.
//
// POR QUE: en la consola el dato vivo maximo llega a 1.037.096 de 1.048.576 (98,9 %) con 5 tablas
// (DC-002.47). El runtime de la consola no tiene un contador de "vivo por tabla", asi que la unica forma
// de saber DONDE estan esos bytes es medirlos en el host con el MISMO camino del motor: el lector de MPQ,
// `d2txt.LoadDataDictionary` y el loader de `d2records` que usa el arranque.
//
// Uso: go run ./console/dc/peso-arranque-248 -mpq <dir con los 3 MPQ> [-n 6]
package main

import (
	"flag"
	"fmt"
	"os"
	"runtime"
	"runtime/pprof"
	"strings"
	"unsafe"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2mpq"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2txt"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2resource"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2util"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2records"
)

var mpqs []*d2mpq.MPQ

// leer busca el asset en los MPQ, en el orden en que el motor los abre.
func leer(nombre string) ([]byte, error) {
	// el MPQ guarda los nombres con barras invertidas y sin la barra inicial (data\global\excel\...)
	clave := strings.TrimPrefix(nombre, "/")
	clave = strings.ReplaceAll(clave, "/", "\\")

	var ultimo error

	for _, m := range mpqs {
		datos, err := m.ReadFile(clave)
		if err == nil {
			return datos, nil
		}

		ultimo = err
	}

	return nil, ultimo
}

func vivo() (uint64, uint64) {
	var ms runtime.MemStats
	runtime.GC()
	runtime.GC()
	runtime.ReadMemStats(&ms)

	return ms.HeapAlloc, ms.HeapObjects
}

func main() {
	dir := flag.String("mpq", "", "directorio con patch_d2.mpq, d2data.mpq y d2char.mpq")
	n := flag.Int("n", 6, "cuantas tablas del arranque medir")
	perfil := flag.String("perfil", "", "escribir un perfil de heap (inuse) al terminar, para pprof")
	probar := flag.String("probar", "", "cargar esta tabla UNA VEZ MAS en un RecordManager nuevo y medir el delta")
	flag.Parse()

	for _, nombre := range []string{"patch_d2.mpq", "d2data.mpq", "d2char.mpq"} {
		m, err := d2mpq.FromFile(*dir + "/" + nombre)
		if err != nil {
			fmt.Println("no pude abrir", nombre, err)
			os.Exit(1)
		}

		mpqs = append(mpqs, m)
	}

	rm, err := d2records.NewRecordManager(d2util.LogLevelNone)
	if err != nil {
		fmt.Println("NewRecordManager:", err)
		os.Exit(1)
	}

	tablas := d2resource.DataDictionaries()
	if *n < len(tablas) {
		tablas = tablas[:*n]
	}

	fmt.Printf("%-3s %-34s %10s %7s %10s %10s %10s\n",
		"#", "tabla", "archivo B", "filas", "vivo B", "acum B", "B/fila")

	var acum uint64

	for i, path := range tablas {
		antes, _ := vivo()

		var (
			bytes  int
			fallas error
		)

		func() {
			datos, errLeer := leer(path)
			if errLeer != nil {
				fallas = errLeer
				return
			}

			bytes = len(datos)

			// el MISMO camino del motor: el stream entra a la tabla y la tabla al loader
			dict := d2txt.LoadDataDictionary(datos)
			if errCarga := rm.Load(path, dict); errCarga != nil {
				fallas = errCarga
			}

			dict = nil
			datos = nil
		}()

		despues, _ := vivo()

		if fallas != nil {
			fmt.Printf("%-3d %-34s  FALLO: %v\n", i+1, path, fallas)
			continue
		}

		peso := despues - antes
		acum += peso

		fmt.Printf("%-3d %-34s %10d %7s %10d %10d %10s\n",
			i+1, path, bytes, "-", peso, acum, "-")
	}

	fmt.Println()
	fmt.Printf("ACUMULADO (vivo, las %d tablas): %d B\n", len(tablas), acum)
	fmt.Printf("semiespacio de la consola (1024 KB): 1048576 B  ->  %.1f %%\n", 100*float64(acum)/1048576)

	// los tamanos de las estructuras, para saber DONDE apretar
	fmt.Println()
	fmt.Printf("sizeof ObjectDetailRecord  %d B\n", unsafe.Sizeof(d2records.ObjectDetailRecord{}))
	fmt.Printf("sizeof LevelPresetRecord   %d B\n", unsafe.Sizeof(d2records.LevelPresetRecord{}))
	fmt.Printf("sizeof LevelTypeRecord     %d B\n", unsafe.Sizeof(d2records.LevelTypeRecord{}))
	fmt.Printf("sizeof LevelWarpRecord     %d B\n", unsafe.Sizeof(d2records.LevelWarpRecord{}))
	fmt.Printf("sizeof ObjectTypeRecord    %d B\n", unsafe.Sizeof(d2records.ObjectTypeRecord{}))
	fmt.Printf("sizeof ItemCommonRecord    %d B\n", unsafe.Sizeof(d2records.ItemCommonRecord{}))

	reporteObjetos(rm)
	reporteTipos(rm)

	// PRUEBA DE LA RETENCION DEL ARCHIVO: cargar Objects.txt OTRA VEZ en un RecordManager nuevo. Si el
	// delta es ~573 x 768 = 440 KB, lo vivo son los records; si es ~654 KB (el archivo son 211.970 B),
	// alguien se esta quedando con el TEXTO crudo de la tabla.
	if *probar != "" {
		rm2, errRM := d2records.NewRecordManager(d2util.LogLevelNone)
		if errRM == nil {
			antes, _ := vivo()

			datos, _ := leer(*probar)
			dict := d2txt.LoadDataDictionary(datos)
			_ = rm2.Load(*probar, dict)
			dict = nil
			datos = nil

			despues, _ := vivo()
			fmt.Printf("\nSEGUNDA CARGA de %s: archivo %d B, delta vivo %d B, filas %d\n",
				*probar, len(datos), despues-antes, len(rm2.Object.Details))
		}
	}

	if *perfil != "" {
		f, errCrear := os.Create(*perfil)
		if errCrear == nil {
			runtime.GC()
			_ = pprof.WriteHeapProfile(f)
			_ = f.Close()
			fmt.Println("\nperfil de heap:", *perfil)
		}
	}
}

// reporteObjetos: los valores MAXIMOS de los arreglos por modo (para saber si entran en int16) y cuantas
// filas tiene la tabla.
func reporteObjetos(rm *d2records.RecordManager) {
	if len(rm.Object.Details) == 0 {
		return
	}

	var maxFC, maxFD, maxLD, maxSF, maxOF, maxParm int

	for _, o := range rm.Object.Details {
		for i := 0; i < 8; i++ {
			maxFC = mayor(maxFC, o.FrameCount[i])
			maxFD = mayor(maxFD, o.FrameDelta[i])
			maxLD = mayor(maxLD, o.LightDiameter[i])
			maxSF = mayor(maxSF, o.StartFrame[i])
			maxOF = mayor(maxOF, o.OrderFlag[i])
			maxParm = mayor(maxParm, o.Parm[i])
		}
	}

	fmt.Println()
	fmt.Printf("Objects.txt: %d filas; maximos por arreglo (int16 = 32767):\n", len(rm.Object.Details))
	fmt.Printf("  FrameCount %d  FrameDelta %d  LightDiameter %d  StartFrame %d  OrderFlag %d  Parm %d\n",
		maxFC, maxFD, maxLD, maxSF, maxOF, maxParm)
}

// reporteTipos: cuantos de los 32 nombres de archivo usa de verdad cada tipo de nivel.
func reporteTipos(rm *d2records.RecordManager) {
	if len(rm.Level.Types) == 0 {
		return
	}

	usados, filas, llenos := 0, 0, 0

	for _, t := range rm.Level.Types {
		filas++

		n := 0
		for i := range t.Files {
			if t.Files[i] != "" {
				n++
			}
		}

		usados += n
		if n == 32 {
			llenos++
		}
	}

	fmt.Printf("LvlTypes.txt: %d filas, %d nombres de archivo NO vacios de %d casillas (%.1f por fila), %d filas con las 32\n",
		filas, usados, filas*32, float64(usados)/float64(filas), llenos)
}

func mayor(a, b int) int {
	if a > b {
		return a
	}

	return b
}

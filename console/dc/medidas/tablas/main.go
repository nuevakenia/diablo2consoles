// DC-002.82 (instrumento del HOST): cuanto cuesta, tabla por tabla, el arranque de las ~85 tablas de
// `d2resource.DataDictionaries()` -- las que el motor carga en `d2app.initDataDictionaries` y que
// `d2mapengine.GenerateMap` necesita para componer el mapa.
//
// POR QUE EN EL HOST. La consola no tiene contador de "vivo por tabla": `runtime.ReadMemStats` esta medido
// como inservible en libgodc (`ms.Alloc` = 0, DC-002.9). Aca se mide con el asignador del host, y lo que la
// consola (Flycast) aporta despues es el monton de C (`d2c_monton_usado`/`arena`/`libre`).
//
// POR QUE NO SE USA `d2asset.AssetManager`. En el escritorio su import arrastra ebiten/GLFW, que en una
// sesion sin DISPLAY paniquea al arrancar (`NotInitialized: The GLFW library is not initialized`). El
// camino del motor es el MISMO de abajo: `LoadRecords` = `LoadDataDictionary` + `Records.Load`; lo unico
// que cambia es de donde salen los bytes (aca `mpq.ReadFile`; en la consola, el stream por sector).
//
// Uso:
//
//	go run ./console/dc/medidas/tablas -dir ~/.wine-d2/drive_c/Games/'Diablo II'
//	go run ./console/dc/medidas/tablas -dir ... -solo-nivel      (las tablas del compositor de nivel)
//	go run ./console/dc/medidas/tablas -dir ... -csv salida.csv
package main

import (
	"flag"
	"fmt"
	"os"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2mpq"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2txt"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2resource"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2util"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2records"
)

// tablasDelNivel: el cierre que `d2mapengine.GenerateMap` toca de verdad. Lo dice el codigo, no una
// suposicion:
//   - `LoadStamp` lee `Records.Level.Presets` (LvlPrest) y `Records.Level.Types` (LvlTypes);
//   - `ResetMap`/`addDT1` recorre `levelType.Files` (los 32 nombres de LvlTypes);
//   - `Stamp.Entities` busca `Records.Monster.Presets` (monpreset), `Records.Monster.Stats` (monstats)
//     y los objetos en `Records.Object.Details` (Objects.txt) + la tabla de lookups del propio motor;
//   - los warp y los mapas automaticos salen de LevelWarp/AutoMap (LvlWarp/AutoMap).
var tablasDelNivel = []string{
	d2resource.LevelType, d2resource.LevelPreset, d2resource.LevelWarp, d2resource.AutoMap,
	d2resource.LevelDetails, d2resource.LevelMaze, d2resource.LevelSubstitutions,
	d2resource.ObjectType, d2resource.ObjectDetails,
	d2resource.MonPreset, d2resource.MonStats, d2resource.MonStats2,
}

type fila struct {
	n       int
	tabla   string
	archivo int
	filas   int
	vivo    int64
	objetos int64
	ms      int64
	nivel   bool
	fallo   string
}

func vivo() (int64, int64) {
	var ms runtime.MemStats

	runtime.GC()
	runtime.GC()
	runtime.ReadMemStats(&ms)

	return int64(ms.HeapAlloc), int64(ms.HeapObjects)
}

func esDelNivel(p string) bool {
	for _, t := range tablasDelNivel {
		if t == p {
			return true
		}
	}

	return false
}

// contarFilas: la tabla es un CSV; las filas del juego son las lineas menos el encabezado y el vacio final.
func contarFilas(raw []byte) int {
	n := 0
	linea := false

	for _, b := range raw {
		switch b {
		case '\n':
			if linea {
				n++
			}

			linea = false
		case '\r':
		default:
			linea = true
		}
	}

	if linea {
		n++
	}

	if n > 0 {
		n-- // el encabezado
	}

	return n
}

var mpqs []*d2mpq.MPQ

// leer: busca la tabla en los MPQ, en el orden en que el motor los abre (el parche primero).
func leer(nombre string) ([]byte, error) {
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

func main() {
	dir := flag.String("dir", "", "carpeta con d2data.mpq, patch_d2.mpq y d2char.mpq")
	soloNivel := flag.Bool("solo-nivel", false, "cargar solo las tablas del compositor de nivel")
	csv := flag.String("csv", "", "escribir las filas medidas a este archivo")
	flag.Parse()

	if *dir == "" {
		fmt.Fprintln(os.Stderr, "falta -dir (la carpeta con los MPQ)")
		os.Exit(2)
	}

	// el orden del motor: parche, datos, chars. Los nombres del disco no siempre estan en minusculas
	// (`patch_D2.mpq`), asi que se buscan sin distinguir mayusculas -- la instalacion es la misma que usa
	// el motor en la consola, donde el MPQ se reconoce por su tabla de hashes.
	entradas, errDir := os.ReadDir(*dir)
	if errDir != nil {
		fmt.Fprintln(os.Stderr, "no pude listar", *dir, errDir)
		os.Exit(1)
	}

	enDisco := map[string]string{}

	for _, e := range entradas {
		enDisco[strings.ToLower(e.Name())] = e.Name()
	}

	for _, nombre := range []string{"patch_d2.mpq", "d2data.mpq", "d2char.mpq"} {
		real, hay := enDisco[nombre]
		if !hay {
			fmt.Fprintf(os.Stderr, "AVISO: no hay %s en %s\n", nombre, *dir)

			continue
		}

		ruta := *dir + "/" + real
		m, e := d2mpq.FromFile(ruta)
		if e != nil {
			fmt.Fprintf(os.Stderr, "no pude abrir %s: %v\n", real, e)
			os.Exit(1)
		}

		fmt.Println("MPQ:", real)

		mpqs = append(mpqs, m)
	}

	rm, err := d2records.NewRecordManager(d2util.LogLevelNone)
	if err != nil {
		fmt.Fprintln(os.Stderr, "NewRecordManager:", err)
		os.Exit(1)
	}

	tablas := d2resource.DataDictionaries()
	if *soloNivel {
		filtradas := make([]string, 0, len(tablasDelNivel))

		for _, p := range tablas {
			if esDelNivel(p) {
				filtradas = append(filtradas, p)
			}
		}

		tablas = filtradas
	}

	vivoAntes, objetosAntes := vivo()
	fmt.Printf("vivo al empezar: %d B (%d objetos)\n", vivoAntes, objetosAntes)
	fmt.Printf("%-3s %-36s %10s %8s %10s %12s %6s\n",
		"#", "tabla", "archivo B", "filas", "vivo B", "objetos", "ms")

	var filasSal []fila

	for i, path := range tablas {
		f := fila{n: i + 1, tabla: path, nivel: esDelNivel(path)}

		antes, oAntes := vivo()
		t0 := time.Now()

		var errCarga error

		func() {
			datos, errLeer := leer(path)
			if errLeer != nil {
				errCarga = errLeer
				return
			}

			f.archivo = len(datos)
			f.filas = contarFilas(datos)

			dict := d2txt.LoadDataDictionary(datos)
			errCarga = rm.Load(path, dict)

			dict = nil
			datos = nil
		}()

		f.ms = time.Since(t0).Milliseconds()

		despues, oDespues := vivo()

		if errCarga != nil {
			f.fallo = errCarga.Error()
			fmt.Printf("%-3d %-36s %10d %8d  FALLO: %v\n", f.n, path, f.archivo, f.filas, errCarga)

			filasSal = append(filasSal, f)

			continue
		}

		f.vivo = despues - antes
		f.objetos = oDespues - oAntes

		fmt.Printf("%-3d %-36s %10d %8d %10d %12d %6d\n",
			f.n, path, f.archivo, f.filas, f.vivo, f.objetos, f.ms)

		filasSal = append(filasSal, f)
	}

	fmt.Println()

	vivoDespues, objetosDespues := vivo()
	// OJO: la suma de los deltas por tabla es la cifra que manda. Medir el total como
	// `HeapAlloc(final) - HeapAlloc(inicial)` da un numero FALSO (medido: 67.232 B de 6,4 MB de records):
	// despues de la ultima vuelta el `rm` queda muerto para el compilador, el GC final lo recolecta y se
	// lleva todas las tablas con el. `runtime.KeepAlive` lo mantiene vivo hasta despues de medir.
	suma := int64(0)

	for _, f := range filasSal {
		suma += f.vivo
	}

	fmt.Printf("ACUMULADO vivo de las %d tablas (suma de los deltas): %d B (%.2f MB)\n", len(tablas), suma,
		float64(suma)/1048576.0)
	fmt.Printf("comprobacion (HeapAlloc final - inicial): %d B, vivos ahora: %d B\n",
		vivoDespues-vivoAntes, vivoDespues)
	fmt.Printf("objetos: +%d\n", objetosDespues-objetosAntes)
	fmt.Printf("semiespacio de la consola (1024 KB): 1048576 B -> %.0f %%\n", 100*float64(suma)/1048576.0)

	orden := append([]fila(nil), filasSal...)
	sort.Slice(orden, func(a, b int) bool { return orden[a].vivo > orden[b].vivo })

	fmt.Println("\n== las 15 tablas mas pesadas (vivo)")
	var acumTop int64

	for i := 0; i < len(orden) && i < 15; i++ {
		acumTop += orden[i].vivo
		marca := ""
		if orden[i].nivel {
			marca = " [nivel]"
		}

		pct := 0.0
		if suma > 0 {
			pct = 100 * float64(orden[i].vivo) / float64(suma)
		}

		fmt.Printf("  %-36s %10d B  (%5.1f %% del total)  %d filas%s\n",
			orden[i].tabla, orden[i].vivo, pct, orden[i].filas, marca)
	}

	if suma > 0 {
		fmt.Printf("  las 15 juntas: %d B (%.1f %% del total)\n", acumTop, 100*float64(acumTop)/float64(suma))
	}

	runtime.KeepAlive(rm)

	var vivoNivel int64

	for _, f := range filasSal {
		if f.nivel {
			vivoNivel += f.vivo
		}
	}

	fmt.Printf("\n== solo las tablas del compositor de nivel (%d): %d B (%.0f %% del semiespacio)\n",
		len(tablasDelNivel), vivoNivel, 100*float64(vivoNivel)/1048576.0)

	if *csv != "" {
		var b strings.Builder

		b.WriteString("tabla;archivo;filas;vivo;objetos;ms;nivel;fallo\n")

		for _, f := range filasSal {
			fmt.Fprintf(&b, "%s;%d;%d;%d;%d;%d;%v;%s\n", f.tabla, f.archivo, f.filas, f.vivo, f.objetos, f.ms, f.nivel, f.fallo)
		}

		if err := os.WriteFile(*csv, []byte(b.String()), 0o644); err != nil {
			fmt.Fprintln(os.Stderr, "no pude escribir el csv:", err)
		} else {
			fmt.Println("\ncsv:", *csv)
		}
	}
}

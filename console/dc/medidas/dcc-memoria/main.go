// Medidor y verificador del DECODIFICADOR de .dcc (DC-002.98): cuanto pide de memoria una direccion, y que
// el camino con ARENA saque los MISMOS bytes que el camino con el monton de Go.
//
// Corre en el HOST (no en la consola). Uso:
//
//	go run ./console/dc/medidas/dcc-memoria medir     <mpq> ["ruta\del\archivo.dcc"]
//	go run ./console/dc/medidas/dcc-memoria verificar <mpq> [cuantos]
//
// QUE MIDE `medir`:
//
//   - la cabecera de la direccion (cuadros, caja) y los pixeles que produce;
//   - el VOLUMEN que pide `d2dcc.LoadDirection` (`runtime.MemStats.TotalAlloc` antes y despues) y en cuantos
//     objetos; en el host eso es el total de bytes reservados, que es el trabajo que el recolector de la
//     consola tiene que mover o mandar al monton de C;
//   - el reparto: cuantas celdas tienen los cuadros y cuanto pide la tabla de celdas del decodificador, que
//     es lo que este camino corrige;
//   - el mismo archivo con `d2dcc.PedirDeArena` puesto, para ver el volumen sin los buffers grandes (los
//     pixeles y las celdas ya no pasan por el monton de Go) y el crc32 de los cuadros de los dos caminos.
//
// QUE HACE `verificar`, y es la prueba central del cambio: agarra los `.dcc` del MPQ (del Listfile) y para
// cada uno decodifica TODAS sus direcciones DOS veces -con el monton de Go y con la arena- comparando el
// tamano y el crc32 de CADA cuadro. Si un solo cuadro cambia, el cambio de memoria toco el formato y el
// numero lo dice. La arena del host es `make` en tramos de 512 KB, el mismo esquema que usa la consola: lo
// que se verifica es el camino, no el asignador.
package main

import (
	"fmt"
	"hash/crc32"
	"os"
	"runtime"
	"sort"
	"strings"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2dcc"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2mpq"
)

const (
	rutaPorDefecto = `data\global\objects\AA\S2\aas2litnuhth.dcc`
	tramo          = 512 * 1024
	pocosArchivos  = 40
)

func main() {
	if len(os.Args) < 3 {
		fmt.Println("uso: dcc-memoria medir|verificar <mpq> [ruta.dcc | cuantos]")
		os.Exit(2)
	}

	m, err := d2mpq.FromFile(os.Args[2])
	if err != nil {
		fmt.Println("FALLO abriendo el MPQ:", err)
		os.Exit(1)
	}

	defer m.Close()

	switch os.Args[1] {
	case "medir":
		medir(m, os.Args[3:])
	case "verificar":
		verificar(m, os.Args[3:])
	default:
		fmt.Println("modo desconocido:", os.Args[1])
		os.Exit(2)
	}
}

// ============================== medir ==============================

func medir(m *d2mpq.MPQ, args []string) {
	ruta := rutaPorDefecto
	if len(args) > 0 {
		ruta = args[0]
	}

	crudo, err := m.ReadFile(ruta)
	if err != nil {
		fmt.Println("FALLO leyendo", ruta, ":", err)
		os.Exit(1)
	}

	fmt.Printf("DCC-MEM archivo=%s bytes=%d\n", ruta, len(crudo))

	sinArena := caminoSinArena(crudo, 0)
	conArena := caminoConArena(crudo, 0)

	if sinArena != conArena {
		fmt.Println("DCC-MEM MAL -- el camino con arena saco otros pixeles")
		os.Exit(1)
	}

	fmt.Printf("DCC-MEM OK -- los dos caminos dan el mismo crc32 de los cuadros (%d)\n", sinArena)
}

func caminoSinArena(crudo []byte, dir int) uint32 {
	d2dcc.PedirDeArena = nil

	runtime.GC()

	var antes, despues runtime.MemStats

	runtime.ReadMemStats(&antes)

	dd, err := d2dcc.LoadDirection(crudo, dir)
	if err != nil {
		fmt.Println("DCC-MEM FALLO decodificando:", err)
		os.Exit(1)
	}

	runtime.ReadMemStats(&despues)

	d := dd.Directions[dir]
	pixeles := len(d.Frames[0].PixelData)

	fmt.Printf("DCC-MEM sin-arena: cuadros=%d caja=%dx%d esquina=%d,%d\n",
		len(d.Frames), d.Box.Width, d.Box.Height, d.Box.Left, d.Box.Top)
	fmt.Printf("DCC-MEM sin-arena: pedido=%d B en %d objetos (%.1f veces los pixeles)\n",
		despues.TotalAlloc-antes.TotalAlloc, despues.Mallocs-antes.Mallocs,
		float64(despues.TotalAlloc-antes.TotalAlloc)/float64(pixeles*len(d.Frames)))
	fmt.Printf("DCC-MEM sin-arena: %s\n", repartoDeCeldas(d, pixeles))

	return crcDeLosCuadros(d)
}

func caminoConArena(crudo []byte, dir int) uint32 {
	a := nuevaArena()

	d2dcc.PedirDeArena = a.pedir

	runtime.GC()

	var antes, despues runtime.MemStats

	runtime.ReadMemStats(&antes)

	dd, err := d2dcc.LoadDirection(crudo, dir)

	runtime.ReadMemStats(&despues)

	d2dcc.PedirDeArena = nil

	if err != nil {
		fmt.Println("DCC-MEM FALLO decodificando con arena:", err)
		os.Exit(1)
	}

	d := dd.Directions[dir]
	pixeles := len(d.Frames[0].PixelData)

	fmt.Printf("DCC-MEM con-arena: %s\n", repartoDeCeldas(d, pixeles))
	fmt.Printf("DCC-MEM con-arena: al monton de Go le pidio %d B en %d objetos (sin arena eran %d B)\n",
		despues.TotalAlloc-antes.TotalAlloc, despues.Mallocs-antes.Mallocs, 0)
	fmt.Printf("DCC-MEM con-arena: la arena entrego %d B en %d pedidos sobre %d tramos (%d B)\n",
		a.entregado, a.pedidosHechos, len(a.tramos), a.capacidad())

	return crcDeLosCuadros(d)
}

// repartoDeCeldas: cuantas celdas tienen los cuadros, cuanto usa el decodificador y cuanto pedia la tabla de
// antes (`maxCellX*maxCellY`).
func repartoDeCeldas(d *d2dcc.DCCDirection, pixeles int) string {
	maxCellX, maxCellY, celdas := 0, 0, 0

	for _, fr := range d.Frames {
		maxCellX += fr.HorizontalCellCount
		maxCellY += fr.VerticalCellCount
		celdas += fr.HorizontalCellCount * fr.VerticalCellCount
	}

	return fmt.Sprintf("pixelesPorCuadro=%d celdasDeLosCuadros=%d (%.0f B con DCCCell de 32 B) "+
		"tablaExacta=%d entradas (%.2f MB con entradas de 12 B) tablaDeAntes=%d entradas (%.1f MB)",
		pixeles, celdas, float64(celdas)*32, celdas, float64(celdas)*12/1e6,
		maxCellX*maxCellY, float64(maxCellX*maxCellY)*12/1e6)
}

// ============================== verificar ==============================

func verificar(m *d2mpq.MPQ, args []string) {
	cuantos := pocosArchivos

	if len(args) > 0 {
		fmt.Sscanf(args[0], "%d", &cuantos)
	}

	rutas := corpus(m, cuantos)
	fmt.Printf("DCC-VERIF: %d archivos con .dcc en el corpus\n", len(rutas))

	cuadros, iguales, distintos, fallos := 0, 0, 0, 0

	for _, ruta := range rutas {
		crudo, err := m.ReadFile(ruta)
		if err != nil {
			fallos++
			fmt.Printf("DCC-VERIF %s LEER-FALLO: %v\n", ruta, err)
			continue
		}

		nuestros, err := decodificarTodo(crudo, false)
		if err != nil {
			fallos++
			fmt.Printf("DCC-VERIF %-58s DECODE-FALLO: %v\n", ruta, err)
			continue
		}

		conArena, err := decodificarTodo(crudo, true)
		if err != nil {
			fallos++
			fmt.Printf("DCC-VERIF %-58s ARENA-FALLO: %v\n", ruta, err)
			continue
		}

		diferencia := comparar(nuestros, conArena)
		cuadros += len(nuestros)

		if diferencia == "" {
			iguales++
			fmt.Printf("DCC-VERIF %-58s bytes=%6d cuadros=%3d crc=%d IGUAL\n",
				ruta, len(crudo), len(nuestros), crcDeCuadros(nuestros))
		} else {
			distintos++
			fmt.Printf("DCC-VERIF %-58s bytes=%6d DIFIERE: %s\n", ruta, len(crudo), diferencia)
		}
	}

	fmt.Printf("DCC-VERIF: archivos=%d iguales=%d distintos=%d fallos=%d cuadros=%d\n",
		len(rutas), iguales, distintos, fallos, cuadros)

	if distintos != 0 || fallos != 0 {
		fmt.Println("DCC-VERIF: MAL -- el camino con arena NO saca los mismos bytes")
		os.Exit(1)
	}

	fmt.Println("DCC-VERIF: OK -- la arena saca los mismos pixeles que el monton de Go, cuadro por cuadro")
}

// corpus: los .dcc del MPQ. Se prefieren los grandes (donde el problema esta) y se recorta a `cuantos`.
func corpus(m *d2mpq.MPQ, cuantos int) []string {
	lista, err := m.Listfile()
	if err != nil || len(lista) == 0 {
		fmt.Println("DCC-VERIF: el MPQ no tiene Listfile; se usa el archivo por defecto")
		return []string{rutaPorDefecto}
	}

	var rutas []string

	for _, r := range lista {
		if strings.HasSuffix(strings.ToLower(r), ".dcc") {
			rutas = append(rutas, r)
		}
	}

	if len(rutas) == 0 {
		fmt.Println("DCC-VERIF: el Listfile no trae .dcc; se usa el archivo por defecto")
		return []string{rutaPorDefecto}
	}

	sort.Strings(rutas)

	if len(rutas) > cuantos {
		tamanos := make(map[string]int, len(rutas))

		for _, r := range rutas {
			if b, err := m.ReadFile(r); err == nil {
				tamanos[r] = len(b)
			}
		}

		sort.Slice(rutas, func(i, j int) bool { return tamanos[rutas[i]] > tamanos[rutas[j]] })
		rutas = rutas[:cuantos]
		sort.Strings(rutas)
	}

	return rutas
}

type cuadroDecodificado struct {
	ancho, alto int
	pixeles     []byte
}

// decodificarTodo: todas las direcciones del archivo, COPIANDO los pixeles de cada cuadro. La copia es a
// proposito: con la arena dos direcciones pueden compartir tramo, y sin copia la comparacion se engaña sola.
func decodificarTodo(crudo []byte, conArena bool) ([]cuadroDecodificado, error) {
	a := nuevaArena()

	if conArena {
		d2dcc.PedirDeArena = a.pedir
	} else {
		d2dcc.PedirDeArena = nil
	}

	defer func() { d2dcc.PedirDeArena = nil }()

	dd, err := d2dcc.Load(crudo)
	if err != nil {
		return nil, err
	}

	var salida []cuadroDecodificado

	for _, d := range dd.Directions {
		if d == nil {
			continue
		}

		for _, fr := range d.Frames {
			if fr == nil {
				continue
			}

			copia := make([]byte, len(fr.PixelData))
			copy(copia, fr.PixelData)

			salida = append(salida, cuadroDecodificado{ancho: d.Box.Width, alto: d.Box.Height, pixeles: copia})
		}
	}

	return salida, nil
}

func comparar(a, b []cuadroDecodificado) string {
	if len(a) != len(b) {
		return fmt.Sprintf("cuadros %d contra %d", len(a), len(b))
	}

	for i := range a {
		if a[i].ancho != b[i].ancho || a[i].alto != b[i].alto {
			return fmt.Sprintf("cuadro %d caja %dx%d contra %dx%d", i, a[i].ancho, a[i].alto, b[i].ancho, b[i].alto)
		}

		if len(a[i].pixeles) != len(b[i].pixeles) {
			return fmt.Sprintf("cuadro %d pixeles %d contra %d", i, len(a[i].pixeles), len(b[i].pixeles))
		}

		if crc32.ChecksumIEEE(a[i].pixeles) != crc32.ChecksumIEEE(b[i].pixeles) {
			return fmt.Sprintf("cuadro %d crc32 %d contra %d", i,
				crc32.ChecksumIEEE(a[i].pixeles), crc32.ChecksumIEEE(b[i].pixeles))
		}
	}

	return ""
}

func crcDeLosCuadros(d *d2dcc.DCCDirection) uint32 {
	var todos []byte

	for _, fr := range d.Frames {
		if fr == nil {
			continue
		}

		todos = append(todos, fr.PixelData...)
	}

	return crc32.ChecksumIEEE(todos)
}

func crcDeCuadros(cs []cuadroDecodificado) uint32 {
	var todos []byte

	for _, c := range cs {
		todos = append(todos, c.pixeles...)
	}

	return crc32.ChecksumIEEE(todos)
}

// ============================== la arena del host ==============================

// arenaHost es el mismo esquema que la arena de la consola: tramos de 512 KB (o lo que se pida, si es mas)
// y un cursor que avanza alineado a 8. No libera: en el host la memoria se la lleva el proceso.
type arenaHost struct {
	tramos        [][]byte
	cursor        int
	entregado     int
	pedidosHechos int
}

func nuevaArena() *arenaHost {
	return &arenaHost{}
}

func (a *arenaHost) capacidad() int {
	total := 0
	for _, t := range a.tramos {
		total += len(t)
	}

	return total
}

func (a *arenaHost) pedir(bytes int) []byte {
	a.pedidosHechos++

	if len(a.tramos) == 0 || a.cursor+bytes > len(a.tramos[len(a.tramos)-1]) {
		nuevo := tramo
		if bytes > nuevo {
			nuevo = bytes
		}

		a.tramos = append(a.tramos, make([]byte, nuevo))
		a.cursor = 0
	}

	b := a.tramos[len(a.tramos)-1][a.cursor : a.cursor+bytes]
	a.cursor += bytes
	a.entregado += bytes

	return b
}

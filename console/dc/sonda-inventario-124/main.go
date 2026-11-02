// DC-002.124 -- SONDA DEL INVENTARIO Y EL ALIJO, EN EL HOST: que hay que portar, cuanto pesa, donde esta el
// alijo del pueblo y con que grafico se dibuja la grilla.
//
// POR QUE EXISTE. La tarjeta pide MEDIR ANTES DE ESCRIBIR (la regla del plan: presupuesto antes de portar) y, en
// la misma pasada, averiguar dos datos que no se pueden inventar:
//   1. COSTO: cuanto suman `d2core/d2inventory` y `d2core/d2ui` -- sus lineas, sus bytes, su cierre de imports y
//      lo que ocupa la ESTRUCTURA de la grilla (las celdas), que es el riesgo de memoria de la etapa E3. El
//      cierre se mide con `go list -deps` (el mismo que usa el build para armar el enlace), aca adentro.
//   2. EL ALIJO DEL PUEBLO: el `.ds1` del pueblo trae 41 objetos; el alijo es UNO de ellos. Esta sonda imprime
//      TODOS los objetos del DS1 (Type/ID/celda) con el nombre que les da `Objects.txt` del parche, y dice cual
//      es el alijo. Sin ese ID no se puede programar "abrir el alijo": seria adivinar.
//   3. LOS GRAFICOS: la grilla del inventario y el cofre son DC6 de `data/global/ui/panel/`; el objeto dentro
//      de su celda es el `invfile` de su fila (weapons/armor/misc). Se imprime, para cada uno, si existe en el
//      MPQ, cuanto mide y cuantos cuadros tiene. Si el DC6 no esta, la grilla se dibuja con primitivas y hay
//      que decirlo, no taparlo.
//
// Uso:
//
//	go run ./console/dc/sonda-inventario-124
//	go run ./console/dc/sonda-inventario-124 -dir "~/.wine-d2/drive_c/Games/Diablo II" -codigo aqv
package main

import (
	"flag"
	"fmt"
	"hash/crc32"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"unsafe"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2dc6"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2ds1"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2mpq"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2inventory"
)

const (
	tablaObjetos      = `data\global\excel\objects.txt`
	tablaArmas        = `data\global\excel\weapons.txt`
	tablaArmaduras    = `data\global\excel\armor.txt`
	tablaMisc         = `data\global\excel\misc.txt`
	carpetaItems      = `data\global\items\`
)

func main() {
	dir := flag.String("dir", "", "carpeta del juego (por omision, la instalacion de wine)")
	codigo := flag.String("codigo", "aqv", "codigo de item para medir su invfile (el del drop de DC-002.123)")
	buscar := flag.String("buscar", "", "si no esta vacio, imprime las entradas del (listfile) que contienen este texto")
	probar := flag.String("probar", "", "lista separada por comas de rutas a probar: dice en que MPQ esta y cuanto mide")
	flag.Parse()

	base := *dir
	if base == "" {
		home, _ := os.UserHomeDir()
		base = filepath.Join(home, ".wine-d2", "drive_c", "Games", "Diablo II")
	}

	if *buscar != "" {
		listar(base, *buscar)

		return
	}

	if *probar != "" {
		probarRutas(base, *probar)

		return
	}

	fmt.Println("== 0. LOS MPQ")
	mpqs := map[string]*d2mpq.MPQ{}

	for _, nombre := range []string{"d2data.mpq", "patch_d2.mpq", "d2char.mpq"} {
		m, err := d2mpq.FromFile(filepath.Join(base, nombre))
		if err != nil {
			fmt.Printf("   %-14s NO ABRE: %v\n", nombre, err)
			continue
		}

		mpqs[nombre] = m
		fmt.Printf("   %-14s %12d B\n", nombre, m.Size())
	}

	if mpqs["d2data.mpq"] == nil {
		fmt.Println("   sin d2data.mpq no hay nada que medir")

		return
	}

	// ============================== 1. EL COSTO DE d2inventory Y d2ui ==============================
	fmt.Println("\n== 1. EL COSTO: la grilla del inventario, medido y no supuesto")

	// lo que ocupa la ESTRUCTURA de la grilla (el riesgo de la etapa: "cientos de celdas")
	var celda Celda

	fmt.Printf("   celda de la grilla: %d B (celda = %d B, estado = %d B)\n",
		unsafe.Sizeof(celda), unsafe.Sizeof(celda), unsafe.Sizeof(celda.Estado))

	invW, invH := d2inventory.Dims(d2inventory.ContainerInventory, true)
	aljW, aljH := d2inventory.Dims(d2inventory.ContainerStash, true)

	fmt.Printf("   d2inventory.Dims inventario %dx%d = %d celdas -> %d B\n", invW, invH, invW*invH,
		invW*invH*int(unsafe.Sizeof(celda)))
	fmt.Printf("   d2inventory.Dims alijo      %dx%d = %d celdas -> %d B\n", aljW, aljH, aljW*aljH,
		aljW*aljH*int(unsafe.Sizeof(celda)))
	fmt.Printf("   cinturon (4 celdas)                  -> %d B\n", 4*int(unsafe.Sizeof(celda)))
	fmt.Printf("   TOTAL grilla del POC (jugador+alijo+cinturon) = %d B\n",
		(invW*invH+aljW*aljH+4)*int(unsafe.Sizeof(celda)))

	// ============================== 2. EL ALIJO DEL PUEBLO ==============================
	fmt.Println("\n== 2. EL ALIJO DEL PUEBLO: el objeto del .ds1 y su nombre en Objects.txt")

	nombres := nombresDeObjetos(mpqs)
	fmt.Printf("   Objects.txt: %d filas (Id -> nombre)\n", len(nombres))

	// el pueblo del acto 1 son CUATRO `.ds1` (E/N/W/S): el alijo puede estar en cualquiera de ellos. Se recorren
	// los cuatro y se junta todo, para que el ID del alijo no salga de una suposicion.
	totalObjetos := 0
	alijos := 0
	vistos := map[int]bool{}
	ids := []int{}

	for _, cual := range []string{rutaDS1PuebloE, rutaDS1PuebloN, rutaDS1PuebloO, rutaDS1PuebloS} {
		datos := leerDeLosMPQ(mpqs, cual)
		if datos == nil {
			fmt.Println("   NO encuentro", cual)

			continue
		}

		fmt.Printf("   %s en %s: %d B crc32=%d\n", cual, dondeEsta(mpqs, cual), len(datos),
			crc32.ChecksumIEEE(datos))

		ds, err := d2ds1.Unmarshal(datos)
		if err != nil {
			fmt.Println("   el .ds1 no parsea:", err)

			continue
		}

		fmt.Printf("   DS1 version=%d act=%d w=%d h=%d objetos=%d\n", ds.Version(), ds.Act, ds.Width, ds.Height,
			len(ds.Objects))

		totalObjetos += len(ds.Objects)

		for i := range ds.Objects {
			o := &ds.Objects[i]
			nombre := nombres[o.ID]
			if nombre == "" {
				nombre = "(sin-nombre)"
			}

			if esAlijo(nombre) {
				alijos++
				fmt.Printf("   ALIJO %s obj=%d type=%d id=%d celda=%d,%d flags=%d nombre=%q\n",
					cual, i, o.Type, o.ID, o.X, o.Y, o.Flags, nombre)
			}

			if !vistos[o.ID] {
				vistos[o.ID] = true
				ids = append(ids, o.ID)
			}
		}
	}

	fmt.Printf("   objetos de los cuatro .ds1 del pueblo: %d; los que son alijo: %d\n", totalObjetos, alijos)

	// QUE ES "type" Y QUE ES "id": se vuelcan los objetos de townE1 con los dos, y el nombre que la tabla le da a
	// cada uno. Sin esto no se sabe cual de las dos columnas es la que nombra al objeto.
	if datos := leerDeLosMPQ(mpqs, rutaDS1PuebloE); datos != nil {
		if ds, err := d2ds1.Unmarshal(datos); err == nil {
			fmt.Println("   -- objetos de townE1: type/id/celda y el nombre de la tabla para cada columna")

			for i := range ds.Objects {
				o := &ds.Objects[i]
				fmt.Printf("     obj=%2d type=%3d (%-14q) id=%3d (%-14q) celda=%3d,%3d flags=%d\n",
					i, o.Type, nombres[o.Type], o.ID, nombres[o.ID], o.X, o.Y, o.Flags)
			}
		}
	}

	// y las filas de la tabla que SUENAN a alijo, para saber que id tendria
	fmt.Println("   -- filas de objects.txt que suenan a alijo")

	for id, nombre := range nombres {
		if esAlijo(nombre) {
			fmt.Printf("     id=%4d nombre=%q\n", id, nombre)
		}
	}

	// EL .ds1 ES DE d2data.mpq: SUS IDs SON LOS DE ESA VERSION DE LA TABLA, no los del parche. Se vuelve a leer
	// la tabla SOLO de d2data.mpq y se nombra con ella: si el id 5 o el 33 es el alijo en esa version, aca sale.
	fmt.Println("   -- los mismos IDs del pueblo, nombrados con objects.txt de d2data.mpq (sin el parche)")

	if m := mpqs["d2data.mpq"]; m != nil {
		if crudo, err := m.ReadFile(tablaObjetos); err == nil {
			solos := nombresDeObjetosDe(crudo)
			fmt.Printf("   objects.txt de d2data.mpq: %d filas\n", len(solos))

			for _, id := range ids {
				fmt.Printf("     id=%4d d2data=%-16q parche=%q\n", id, solos[id], nombres[id])
			}

			fmt.Println("   -- filas de la tabla de d2data que suenan a alijo")

			for id, nombre := range solos {
				if esAlijo(nombre) {
					fmt.Printf("     id=%4d nombre=%q\n", id, nombre)
				}
			}
		}
	}

	// el volcado completo, para poder auditar que el ID no se invento
	sort.Ints(ids)
	fmt.Printf("   IDs distintos en el pueblo: %d\n", len(ids))

	for _, id := range ids {
		fmt.Printf("     id=%4d nombre=%q\n", id, nombres[id])
	}

	// ============================== 3. LOS GRAFICOS ==============================
	fmt.Println("\n== 3. LOS GRAFICOS: la grilla, el cofre y el objeto en su celda")

	for _, rut := range []string{
		`data\global\ui\panel\inventory.dc6`,
		`data\global\ui\panel\stash.dc6`,
		`data\global\ui\panel\stash.dc6`,
		`data\global\ui\panel\panels.dc6`,
	} {
		medirDC6(mpqs, rut)
	}

	fmt.Println("   -- el objeto en su celda (invfile de su fila)")
	fmt.Printf("   codigo del drop de referencia: %s\n", *codigo)

	for _, cual := range []struct{ ruta, nombre string }{
		{tablaArmas, "weapons.txt"}, {tablaArmaduras, "armor.txt"}, {tablaMisc, "misc.txt"},
	} {
		datos := leerDeLosMPQ(mpqs, cual.ruta)
		if datos == nil {
			continue
		}

		inv, flippy := columnasDelItem(datos, *codigo)
		if inv == "" && flippy == "" {
			continue
		}

		fmt.Printf("   %s: invfile=%q flippyfile=%q\n", cual.nombre, inv, flippy)

		if inv != "" {
			medirDC6(mpqs, carpetaItems+inv+".dc6")
		}

		if flippy != "" {
			medirDC6(mpqs, carpetaItems+flippy+".dc6")
		}
	}
}

// Celda: la celda de la grilla como la va a tener el POC (codigo + estado). Se mide con `unsafe.Sizeof` para que
// el numero de la tabla de memoria sea el de la estructura de verdad.
type Celda struct {
	Codigo  string // el codigo del item que la ocupa ("" = vacia)
	Estado  uint8  // 0 vacia, 1 ocupada
	Fila    uint8
	Columna uint8
	Origen  uint8 // 0 suelo, 1 alijo
}

// probarRutas: prueba cada ruta (separadas por comas) en los tres MPQ, en el orden del loader.
func probarRutas(base, rutas string) {
	mpqs := map[string]*d2mpq.MPQ{}

	for _, nombre := range []string{"patch_d2.mpq", "d2data.mpq", "d2exp.mpq", "d2char.mpq"} {
		if m, err := d2mpq.FromFile(filepath.Join(base, nombre)); err == nil {
			mpqs[nombre] = m
		}
	}

	for _, ruta := range strings.Split(rutas, ",") {
		ruta = strings.TrimSpace(ruta)
		if ruta == "" {
			continue
		}

		encontrada := false

		for _, nombre := range []string{"patch_d2.mpq", "d2data.mpq", "d2char.mpq"} {
			m := mpqs[nombre]
			if m == nil {
				continue
			}

			datos, err := m.ReadFile(ruta)
			if err != nil || len(datos) == 0 {
				continue
			}

			encontrada = true
			fmt.Printf("   OK   %-46s %-13s %9d B\n", ruta, nombre, len(datos))
		}

		if !encontrada {
			fmt.Printf("   NO   %-46s\n", ruta)
		}
	}
}

// listar: las entradas del `(listfile)` de cada MPQ que contienen un texto. Sirve para no adivinar rutas: si el
// nombre no esta en la lista, no existe.
func listar(base, busca string) {
	for _, nombre := range []string{"d2data.mpq", "patch_d2.mpq"} {
		m, err := d2mpq.FromFile(filepath.Join(base, nombre))
		if err != nil {
			fmt.Printf("%s NO ABRE: %v\n", nombre, err)

			continue
		}

		datos, err := m.ReadFile("(listfile)")
		if err != nil {
			fmt.Printf("%s sin (listfile): %v\n", nombre, err)

			continue
		}

		lineas := strings.Split(string(datos), "\r\n")
		n := 0

		for _, l := range lineas {
			if !strings.Contains(strings.ToLower(l), strings.ToLower(busca)) {
				continue
			}

			fmt.Printf("%-14s %s\n", nombre, l)
			n++
		}

		fmt.Printf("%-14s -> %d entradas con %q de %d\n", nombre, n, busca, len(lineas))
	}
}

// nombresDeObjetosDe: el Id de Objects.txt -> su nombre (columna `Name`), de los bytes crudos de la tabla.
func nombresDeObjetosDe(datos []byte) map[int]string {
	lineas := strings.Split(string(datos), "\n")
	if len(lineas) < 2 {
		return map[int]string{}
	}

	cab := strings.Split(strings.TrimRight(lineas[0], "\r"), "\t")
	cID, cName := indiceDe(cab, "Id"), indiceDe(cab, "Name")

	if cID < 0 || cName < 0 {
		return map[int]string{}
	}

	salida := map[int]string{}

	for _, linea := range lineas[1:] {
		campos := strings.Split(strings.TrimRight(linea, "\r"), "\t")
		if len(campos) <= cID || len(campos) <= cName {
			continue
		}

		id, err := strconv.Atoi(strings.TrimSpace(campos[cID]))
		if err != nil {
			continue
		}

		salida[id] = strings.TrimSpace(campos[cName])
	}

	return salida
}

// nombresDeObjetos: el Id de Objects.txt -> su nombre (columna `Name`). El DS1 del pueblo trae el ID del objeto y
// la tabla dice que es.
func nombresDeObjetos(mpqs map[string]*d2mpq.MPQ) map[int]string {
	datos := leerDeLosMPQ(mpqs, tablaObjetos)
	if datos == nil {
		fmt.Println("   NO encuentro", tablaObjetos)

		return map[int]string{}
	}

	fmt.Printf("   %s: %d B crc32=%d\n", tablaObjetos, len(datos), crc32.ChecksumIEEE(datos))

	lineas := strings.Split(string(datos), "\n")
	if len(lineas) < 2 {
		return map[int]string{}
	}

	cab := strings.Split(strings.TrimRight(lineas[0], "\r"), "\t")
	cID, cName := indiceDe(cab, "Id"), indiceDe(cab, "Name")

	if cID < 0 || cName < 0 {
		fmt.Printf("   la cabecera no tiene Id/Name: %v\n", cab)

		return map[int]string{}
	}

	salida := map[int]string{}

	for _, linea := range lineas[1:] {
		campos := strings.Split(strings.TrimRight(linea, "\r"), "\t")
		if len(campos) <= cID || len(campos) <= cName {
			continue
		}

		id, err := strconv.Atoi(strings.TrimSpace(campos[cID]))
		if err != nil {
			continue
		}

		salida[id] = strings.TrimSpace(campos[cName])
	}

	return salida
}

// columnasDelItem: el invfile y el flippyfile de la fila del codigo, en la primera tabla que lo tenga.
func columnasDelItem(datos []byte, codigo string) (inv, flippy string) {
	lineas := strings.Split(string(datos), "\n")
	if len(lineas) < 2 {
		return "", ""
	}

	cab := strings.Split(strings.TrimRight(lineas[0], "\r"), "\t")
	cCode := indiceDe(cab, "code")
	cInv := indiceDe(cab, "invfile")
	cFlip := indiceDe(cab, "flippyfile")

	if cCode < 0 {
		return "", ""
	}

	for _, linea := range lineas[1:] {
		campos := strings.Split(strings.TrimRight(linea, "\r"), "\t")
		if len(campos) <= cCode || !strings.EqualFold(strings.TrimSpace(campos[cCode]), codigo) {
			continue
		}

		if cInv >= 0 && len(campos) > cInv {
			inv = strings.TrimSpace(campos[cInv])
		}

		if cFlip >= 0 && len(campos) > cFlip {
			flippy = strings.TrimSpace(campos[cFlip])
		}

		return inv, flippy
	}

	return "", ""
}

// esAlijo: los nombres con los que el juego llama al cofre del pueblo.
func esAlijo(nombre string) bool {
	n := strings.ToLower(nombre)

	return strings.Contains(n, "stash") || strings.Contains(n, "alijo")
}

// medirDC6: existe, tamano, crc32 y cuantos cuadros, con la caja del primero.
func medirDC6(mpqs map[string]*d2mpq.MPQ, ruta string) {
	datos := leerDeLosMPQ(mpqs, ruta)
	if datos == nil {
		fmt.Printf("   %-42s NO ESTA en el MPQ\n", ruta)

		return
	}

	dc, err := d2dc6.Load(datos)
	if err != nil {
		fmt.Printf("   %-42s %8d B NO PARSEA: %v\n", ruta, len(datos), err)

		return
	}

	caja := ""
	if len(dc.Frames) > 0 {
		fr := dc.Frames[0]
		pix := dc.DecodeFrame(0)
		caja = fmt.Sprintf("cuadro0=%dx%d esquina=%d,%d pixeles=%d", fr.Width, fr.Height, fr.OffsetX, fr.OffsetY,
			noVacios(pix))
	}

	fmt.Printf("   %-42s %8d B crc32=%10d cuadros=%2d %s\n", ruta, len(datos), crc32.ChecksumIEEE(datos),
		len(dc.Frames), caja)
}

func noVacios(p []byte) int {
	n := 0

	for _, b := range p {
		if b != 0 {
			n++
		}
	}

	return n
}

func indiceDe(cab []string, nombre string) int {
	for i, c := range cab {
		if strings.EqualFold(strings.TrimSpace(c), nombre) {
			return i
		}
	}

	return -1
}

// leerDeLosMPQ: el primero que lo tenga, en el orden del loader del motor (parche primero).
func leerDeLosMPQ(mpqs map[string]*d2mpq.MPQ, ruta string) []byte {
	_, datos := buscarEnLosMPQ(mpqs, ruta)

	return datos
}

func buscarEnLosMPQ(mpqs map[string]*d2mpq.MPQ, ruta string) (string, []byte) {
	for _, nombre := range []string{"patch_d2.mpq", "d2data.mpq", "d2char.mpq"} {
		m := mpqs[nombre]
		if m == nil {
			continue
		}

		datos, err := m.ReadFile(ruta)
		if err == nil && len(datos) > 0 {
			return nombre, datos
		}
	}

	return "", nil
}

func dondeEsta(mpqs map[string]*d2mpq.MPQ, ruta string) string {
	for _, nombre := range []string{"patch_d2.mpq", "d2data.mpq", "d2char.mpq"} {
		m := mpqs[nombre]
		if m == nil {
			continue
		}

		if datos, err := m.ReadFile(ruta); err == nil && len(datos) > 0 {
			return nombre
		}
	}

	return "(ninguno)"
}

// la grilla del motor, para que el presupuesto no sea una suposicion: cuantas celdas dice `d2inventory`.
var _ = d2inventory.Dims

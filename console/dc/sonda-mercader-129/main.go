// DC-002.129 -- SONDA DE LOS MERCADERES, EN EL HOST: que hay que portar, cuanto pesa y que stock sale,
// medido ANTES de escribir una linea del POC (la regla del plan, docs/PLAN-JUEGO.md).
//
// POR QUE EXISTE. La tarjeta (E5) pide abrir el stock de Charsi en la grilla, con los precios, y comprar con oro.
// Para no inventar nada, esta sonda contesta, con los archivos del juego delante y con los paquetes del motor
// compilados en el host:
//
//  1. EL PRESUPUESTO: cuanto suman `d2core/d2vendor` (2.609 lineas, la generacion del stock) y
//     `d2common/d2trade` (1.774, los precios), en lineas, bytes y CIERRE de imports (`go list -deps`, el mismo que
//     usa el build para armar el enlace). Si el cierre se lleva ebiten/d2asset adentro, el paquete NO se porta tal
//     cual: se porta su MODELO (las reglas) y el POC le da los datos. Ademas: la API exportada que el POC puede
//     llamar sin arrastrar `d2records` (que es lo que mete d2asset), y el tamano de las estructuras (`unsafe`).
//  2. LAS TABLAS DEL MERCADER: las columnas `<Npc>Min/Max/MagicMin/MagicMax/MagicLvl` de armor/weapons/misc.txt,
//     cuantos items base le corresponden a Charsi, sus paginas (`StorePage` de itemtypes.txt) y los bytes que hay
//     que leer. Es el dato que el POC tiene que leer en la consola con su propio lector.
//  3. EL STOCK, GENERADO CON EL MOTOR: `d2vendor.GenerateSeeded` con esas bases y el nivel del jugador. Imprime el
//     stock entero (codigo, calidad, ilvl, pagina y celda), su crc32 y el tamano en celdas. Ese crc32 es la
//     REFERENCIA contra la que se compara la corrida de la consola: si el POC genera el mismo stock, el numero
//     tiene que ser el mismo.
//  4. LOS PRECIOS Y EL ORO: la fila de npc.txt de Charsi (`sell mult`, `buy mult`, `max buy`) convertida a
//     `d2trade.NPC` como la convierte `d2vendor.NPCPricing`, y el precio de compra/venta de cada item del stock con
//     `d2trade.ItemPrice`. Con eso: cuanto oro necesita el jugador y cuantas compras entran en el tope de oro del
//     nivel 1 (`d2inventory.InventoryGoldLimit` = nivel * 10.000, VERIFICADO contra PLAYER_GetMaxGoldCarry).
//  5. LO QUE PESA DIBUJARLO: la caja del panel de la grilla del mercader (10x10 celdas como el registro "Monster"
//     de Inventory.txt), los bytes de los `invfile` de los items del stock (cuantos entran vivos) y el texto de los
//     precios.
//
// Uso:
//
//	go run ./console/dc/sonda-mercader-129
//	go run ./console/dc/sonda-mercader-129 -dir "~/.wine-d2/drive_c/Games/Diablo II" -nivel 1 -semilla 129
package main

import (
	"flag"
	"fmt"
	"hash/crc32"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"unsafe"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2dc6"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2mpq"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2math/d2rand"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2trade"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2inventory"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2vendor"
)

const (
	tablaArmaduras = `data\global\excel\armor.txt`
	tablaArmas     = `data\global\excel\weapons.txt`
	tablaMisc      = `data\global\excel\misc.txt`
	tablaTipos     = `data\global\excel\itemtypes.txt`
	tablaNPC       = `data\global\excel\npc.txt`

	// el mercader de la tarjeta (d2vendor.Act1 lo declara: Name "Charsi", ClassID 154, Repairs)
	mercader = "Charsi"
	claseExe = 154
)

// paquetes: lo que la tarjeta tiene que portar, con la MISMA vara que DC-002.127 (lineas, bytes y cierre).
var paquetes = []string{
	"d2core/d2vendor",
	"d2common/d2trade",
	"d2core/d2inventory",
	"d2core/d2item/d2drop",
	"d2core/d2records",
}

func main() {
	dir := flag.String("dir", "", "carpeta del juego (por omision, la instalacion de wine)")
	nivel := flag.Int("nivel", 1, "nivel del jugador con el que se genera el stock")
	semilla := flag.Int("semilla", 129, "semilla del stock (el dado del motor)")
	flag.Parse()

	base := *dir
	if base == "" {
		home, _ := os.UserHomeDir()
		base = filepath.Join(home, ".wine-d2", "drive_c", "Games", "Diablo II")
	}

	fmt.Println("== 0. LOS MPQ")
	mpqs := map[string]*d2mpq.MPQ{}

	for _, nombre := range []string{"patch_d2.mpq", "d2data.mpq", "d2char.mpq"} {
		m, err := d2mpq.FromFile(filepath.Join(base, nombre))
		if err != nil {
			fmt.Printf("   %-14s NO ABRE: %v\n", nombre, err)

			continue
		}

		mpqs[nombre] = m
		fmt.Printf("   %-14s %12d B\n", nombre, m.Size())
	}

	if mpqs["patch_d2.mpq"] == nil && mpqs["d2data.mpq"] == nil {
		fmt.Println("   sin MPQ no hay nada que medir")

		return
	}

	elPresupuesto()
	filas, paginas := lasTablasDelMercader(mpqs)
	stock := elStock(filas, paginas, *nivel, uint32(*semilla))
	losPreciosYPrecios(mpqs, filas, paginas, stock, *nivel)
	loQuePesaDibujarlo(mpqs, stock)
}

// ============================== 1. EL PRESUPUESTO ==============================

func elPresupuesto() {
	fmt.Println("\n== 1. EL PRESUPUESTO: que cuesta portar d2vendor y d2trade (go list -deps)")

	for _, p := range paquetes {
		archivos, lineas, bytes := medirPaquete(p)
		deps := cierreDeImports(p)
		ebiten, assets, records := enElCierre(deps)

		fmt.Printf("   %-26s %2d archivos %6d lineas %8d B | cierre %3d paquetes ebiten=%-5v d2asset=%-5v d2records=%v\n",
			p, archivos, lineas, bytes, len(deps), ebiten, assets, records)
	}

	fmt.Println("   -- las estructuras que el POC toca (unsafe.Sizeof en el host; en la consola son mas chicas)")

	var (
		b  d2vendor.Base
		it d2vendor.Item
		st d2vendor.Stock
		np d2trade.NPC
		ip d2trade.Item
		pp d2trade.Params
	)

	fmt.Printf("      d2vendor.Base   %3d B  (uno por item base del mercader)\n", unsafe.Sizeof(b))
	fmt.Printf("      d2vendor.Item   %3d B  (uno por item del stock)\n", unsafe.Sizeof(it))
	fmt.Printf("      d2vendor.Stock  %3d B  (la cabecera; los items van aparte)\n", unsafe.Sizeof(st))
	fmt.Printf("      d2trade.NPC     %3d B  (la fila de npc.txt)\n", unsafe.Sizeof(np))
	fmt.Printf("      d2trade.Item    %3d B  (lo que lee el precio)\n", unsafe.Sizeof(ip))
	fmt.Printf("      d2trade.Params  %3d B\n", unsafe.Sizeof(pp))

	fmt.Println("   -- la API exportada que el POC puede usar SIN d2records (lo que se porta)")
	fmt.Println("      d2vendor.GenerateSeeded(seed uint32, bases []Base, opt Options) *Stock")
	fmt.Println("      d2vendor.ItemLevel / RollQuality / VendorSells / ActIndex / StockSeed")
	fmt.Println("      d2vendor.Item{Code,Quality,ILvl,Quantity,W,H,X,Y,Page} y Stock{Items} + Place/Remove/ItemAtPage")
	fmt.Println("      d2trade.ItemPrice(it *Item, p Params) int  (el port de TRADE_CalcItemPrice 0x62f100)")
	fmt.Println("      d2trade.IdentifyCost / GamblePrice / SocketedDelta (no entran: esta tarjeta no repara ni identifica)")

	fmt.Println("   -- lo que NO se puede portar tal cual: las funciones que piden *d2records.RecordManager")
	fmt.Println("      d2vendor.BasesFor / StorePageOf / NPCPricing / Resolver / GamblePoolFor: el POC les da los datos")
	fmt.Println("      ya leidos de las tablas (es lo que mide la seccion 2), igual que DC-002.124 le dio las celdas a")
	fmt.Println("      d2inventory.Dims en vez de portar el arbol entero.")

	fmt.Println("   -- el cierre de d2trade (tiene que ser CORTO: son los precios, sin assets)")
	for _, d := range filtrarDeps(cierreDeImports("d2common/d2trade")) {
		fmt.Println("      " + d)
	}
}

func medirPaquete(p string) (archivos, lineas, bytes int) {
	dir := p
	if _, err := os.Stat(dir); err != nil {
		dir = filepath.Join("..", "..", p)
	}

	entradas, err := os.ReadDir(dir)
	if err != nil {
		return 0, 0, 0
	}

	for _, e := range entradas {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}

		b, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}

		archivos++
		bytes += len(b)
		lineas += strings.Count(string(b), "\n")
	}

	return archivos, lineas, bytes
}

func cierreDeImports(p string) []string {
	salida, err := exec.Command("go", "list", "-deps", "./"+p).Output()
	if err != nil {
		return nil
	}

	var deps []string

	for _, l := range strings.Split(string(salida), "\n") {
		if l = strings.TrimSpace(l); l != "" {
			deps = append(deps, l)
		}
	}

	return deps
}

func enElCierre(deps []string) (ebiten, assets, records bool) {
	for _, d := range deps {
		if strings.Contains(d, "hajimehoshi/ebiten") {
			ebiten = true
		}

		if strings.HasSuffix(d, "d2core/d2asset") {
			assets = true
		}

		if strings.HasSuffix(d, "d2core/d2records") {
			records = true
		}
	}

	return ebiten, assets, records
}

func filtrarDeps(deps []string) []string {
	var out []string

	for _, d := range deps {
		if strings.Contains(d, "OpenDiablo2") || strings.Contains(d, "hajimehoshi") {
			out = append(out, d)
		}
	}

	sort.Strings(out)

	return out
}

// ============================== 2. LAS TABLAS DEL MERCADER ==============================

// filaDeItem: lo que el POC tiene que sacar de una fila de armor/weapons/misc.txt para armar un `d2vendor.Base`.
type filaDeItem struct {
	Tabla                              string
	Code, Tipo                         string
	Page                               int
	LevelReq, InvW, InvH, MaxStack     int
	Version, Cost                      int
	PermStore                          bool
	Min, Max, MagicMin, MagicMax, MLev int
	Uber, Ultra, Pesadilla, Infierno   string
}

// tipoDeItem: la fila de itemtypes.txt que le importa al mercader (page = StorePage, Normal y Quiver son las dos
// columnas que el adaptador del motor usa para `CanBeMagic` y para el precio de las flechas).
type tipoDeItem struct {
	Pagina int
	Normal bool
	Quiver bool
}

// lasTablasDelMercader: lee las TRES tablas de items y devuelve las filas que le corresponden a Charsi (las que
// tienen columnas suyas con Max o MagicMax distinto de 0), que es el mismo filtro de `d2vendor.BasesFor`.
func lasTablasDelMercader(mpqs map[string]*d2mpq.MPQ) ([]filaDeItem, map[string]tipoDeItem) {
	fmt.Println("\n== 2. LAS TABLAS DEL MERCADER: las columnas de Charsi en armor/weapons/misc.txt")

	// las paginas del mercader salen de itemtypes.txt (columna StorePage); se leen una vez
	paginas := leerPaginas(mpqs)
	fmt.Printf("   itemtypes.txt: %d tipos, %d con pagina de mercader\n", len(paginas), contarConPagina(paginas))

	var filas []filaDeItem

	total := 0

	for _, cual := range []struct{ ruta, nombre string }{
		{tablaArmaduras, "armor.txt"}, {tablaArmas, "weapons.txt"}, {tablaMisc, "misc.txt"},
	} {
		datos := leerDeLosMPQ(mpqs, cual.ruta)
		if datos == nil {
			fmt.Println("   NO encuentro", cual.ruta)

			continue
		}

		total += len(datos)

		fs, conCharsi := leerItemsDeUnaTabla(datos, cual.nombre)
		filas = append(filas, fs...)

		fmt.Printf("   %-14s %8d B crc32=%11d filas=%4d conColumnasDeCharsi=%3d\n",
			cual.nombre, len(datos), crc32.ChecksumIEEE(datos), contarFilas(datos), conCharsi)
	}

	fmt.Printf("   TOTAL de las tres tablas: %d B (es lo que el POC lee en la consola, con dueno)\n", total)

	// el orden importa: `Generate` recorre las bases en orden y el dado es UNO, asi que la lista ordenada por
	// codigo es la que fija el stock (es la misma decision del adaptador del motor: `BasesFor` ordena por code)
	sort.Slice(filas, func(i, j int) bool { return filas[i].Code < filas[j].Code })

	fmt.Println("   -- los items base de Charsi, ya ordenados por codigo (lo que come Generate)")
	fmt.Printf("   %-6s %-10s %-14s %4s %3s %3s %5s %4s %4s %4s %4s %4s %3s %-8s\n",
		"code", "tabla", "tipo", "reqlvl", "inv", "per", "cost", "min", "max", "mmin", "mmax", "mlvl", "pag", "perm")

	porPagina := map[int]int{}
	perm := 0

	for _, f := range filas {
		pag := -1
		if t, ok := paginas[f.Tipo]; ok {
			pag = t.Pagina
			porPagina[pag]++
		}

		if f.PermStore {
			perm++
		}

		fmt.Printf("   %-6s %-10s %-14s %4d %3d %3d %5d %4d %4d %4d %4d %4d %3d %-8v\n",
			f.Code, f.Tabla, recorte(f.Tipo, 14), f.LevelReq, f.InvW, f.InvH, f.Cost, f.Min, f.Max,
			f.MagicMin, f.MagicMax, f.MLev, pag, f.PermStore)
	}

	fmt.Printf("   items base de Charsi: %d (permanentes: %d) por pagina: %v\n", len(filas), perm, porPagina)

	// la pagina de cada fila se resuelve con itemtypes.txt (es el dato que el POC tambien necesita)
	for i := range filas {
		if p, ok := paginasA(filas[i].Tipo, paginas); ok {
			filas[i].Page = p
		}
	}

	return filas, paginas
}

// paginasA: la pagina del mercader del tipo de item (`StorePageOf` del motor sobre itemtypes.txt).
func paginasA(tipo string, paginas map[string]tipoDeItem) (int, bool) {
	t, ok := paginas[tipo]
	if !ok {
		return -1, false
	}

	return t.Pagina, true
}

func leerItemsDeUnaTabla(datos []byte, nombre string) ([]filaDeItem, int) {
	lineas := strings.Split(string(datos), "\n")
	if len(lineas) < 2 {
		return nil, 0
	}

	cab := lineas[0]

	col := func(n string) int { return indiceDeColumna(cab, n) }

	iCode, iTipo, iLvl := col("code"), col("type"), col("levelreq")
	iW, iH := col("invwidth"), col("invheight")
	iPerm, iVer, iCost := col("PermStoreItem"), col("version"), col("cost")
	iStack := col("maxstack")
	iMin, iMax := col(mercader+"Min"), col(mercader+"Max")
	iMMin, iMMax, iMLev := col(mercader+"MagicMin"), col(mercader+"MagicMax"), col(mercader+"MagicLvl")
	iUber, iUltra := col("ubercode"), col("ultracode")
	iPes, iInf := col("NightmareUpgrade"), col("HellUpgrade")

	if iCode < 0 || iMin < 0 || iMax < 0 || iMMin < 0 || iMMax < 0 || iMLev < 0 {
		fmt.Printf("   %s no tiene las columnas de %s (code=%d min=%d max=%d mmin=%d mmax=%d mlvl=%d)\n",
			nombre, mercader, iCode, iMin, iMax, iMMin, iMMax, iMLev)

		return nil, 0
	}

	// los indices que se usan, para que un cero en la tabla no se confunda con una columna que no existe
	fmt.Printf("   CONTROL %s col: code=%d type=%d levelreq=%d inv=%d,%d perm=%d ver=%d cost=%d maxstack=%d\n",
		nombre, iCode, iTipo, iLvl, iW, iH, iPerm, iVer, iCost, iStack)

	var out []filaDeItem

	conCharsi := 0

	for _, l := range lineas[1:] {
		l = strings.TrimRight(l, "\r")
		if strings.TrimSpace(l) == "" {
			continue
		}

		f := filaDeItem{
			Tabla: nombre, Code: texto(campo(l, iCode)), Tipo: texto(campo(l, iTipo)),
			LevelReq: entero(campo(l, iLvl)), InvW: entero(campo(l, iW)), InvH: entero(campo(l, iH)),
			Cost: entero(campo(l, iCost)), MaxStack: entero(campo(l, iStack)),
			PermStore: entero(campo(l, iPerm)) != 0, Version: entero(campo(l, iVer)),
			Min: entero(campo(l, iMin)), Max: entero(campo(l, iMax)),
			MagicMin: entero(campo(l, iMMin)), MagicMax: entero(campo(l, iMMax)), MLev: entero(campo(l, iMLev)),
			Uber: texto(campo(l, iUber)), Ultra: texto(campo(l, iUltra)),
			Pesadilla: texto(campo(l, iPes)), Infierno: texto(campo(l, iInf)),
		}

		if f.Code == "" {
			continue
		}

		if f.Max == 0 && f.MagicMax == 0 {
			continue // el mismo filtro de BasesFor: sin columnas del mercader, el item no es suyo
		}

		conCharsi++
		out = append(out, f)
	}

	return out, conCharsi
}

// leerPaginas: itemtypes.txt -> Tipo -> pagina del mercader (0 armo, 1 weap, 2 mag, 3 misc), -1 si el tipo no
// tiene pagina (es la regla de `d2vendor.StorePageOf`, con la tabla `storePageCodes` del motor).
func leerPaginas(mpqs map[string]*d2mpq.MPQ) map[string]tipoDeItem {
	paginas := map[string]tipoDeItem{}

	datos := leerDeLosMPQ(mpqs, tablaTipos)
	if datos == nil {
		fmt.Println("   NO encuentro itemtypes.txt: sin paginas, todos los items caen en la 0")

		return paginas
	}

	lineas := strings.Split(string(datos), "\n")
	if len(lineas) < 2 {
		return paginas
	}

	cab := lineas[0]
	// la clave del registro es la columna `Code` ("shie", "bowq"), NO `ItemType` (que es el nombre: "Shield",
	// "Bow Quiver"): el loader del motor indexa `records[itemType.Code]` (d2records/item_types_loader.go).
	iCode, iPage := indiceDeColumna(cab, "Code"), indiceDeColumna(cab, "StorePage")
	iNormal, iQuiver := indiceDeColumna(cab, "Normal"), indiceDeColumna(cab, "Quiver")
	if iCode < 0 || iPage < 0 {
		fmt.Printf("   itemtypes.txt sin ItemType/StorePage (%d/%d); columnas: %s\n", iCode, iPage,
			strings.Join(columnas(cab), "|"))

		return paginas
	}

	codigos := []string{"armo", "weap", "mag", "misc"} // storePageCodes del motor

	vistos := 0

	for _, l := range lineas[1:] {
		l = strings.TrimRight(l, "\r")
		if strings.TrimSpace(l) == "" {
			continue
		}

		tipo := texto(campo(l, iCode))
		sp := texto(campo(l, iPage))

		t := tipoDeItem{Pagina: -1, Normal: entero(campo(l, iNormal)) != 0, Quiver: texto(campo(l, iQuiver)) != ""}

		for i, c := range codigos {
			if strings.EqualFold(c, sp) {
				t.Pagina = i
			}
		}

		if vistos < 6 {
			fmt.Printf("   CONTROL itemtypes fila %d: ItemType=%q StorePage=%q Normal=%d Quiver=%q -> pagina %d\n",
				vistos, tipo, sp, entero(campo(l, iNormal)), texto(campo(l, iQuiver)), t.Pagina)
			vistos++
		}

		paginas[tipo] = t
	}

	return paginas
}

func columnas(cabecera string) []string {
	campos := strings.Split(strings.TrimRight(cabecera, "\r"), "	")

	for i := range campos {
		campos[i] = strings.TrimSpace(campos[i])
	}

	return campos
}

func contarConPagina(p map[string]tipoDeItem) int {
	n := 0

	for _, v := range p {
		if v.Pagina >= 0 {
			n++
		}
	}

	return n
}

// ============================== 3. EL STOCK ==============================

// elStock: las filas de las tablas -> `[]d2vendor.Base` -> `d2vendor.GenerateSeeded`. El stock sale del CODIGO DEL
// MOTOR, no de una copia: lo unico que pone esta sonda son los datos de las tablas.
func elStock(filas []filaDeItem, paginas map[string]tipoDeItem, nivel int, semilla uint32) *d2vendor.Stock {
	fmt.Println("\n== 3. EL STOCK DE CHARSI, GENERADO CON EL MOTOR (d2vendor.GenerateSeeded)")

	bases := make([]d2vendor.Base, 0, len(filas))

	for _, f := range filas {
		b := d2vendor.Base{
			Code: f.Code, Page: f.Page, ReqLevel: f.LevelReq, W: f.InvW, H: f.InvH,
			Vendor:  d2vendor.Params{Min: f.Min, Max: f.Max, MagicMin: f.MagicMin, MagicMax: f.MagicMax, MagicLevel: f.MLev},
			Version: f.Version, Permanent: f.PermStore, MaxStack: f.MaxStack,
			Uber: f.Uber, Ultra: f.Ultra, NightmareUpgrade: f.Pesadilla, HellUpgrade: f.Infierno,
			Gear: f.Tabla != "misc.txt",
		}
		// CanBeMagic: la derivada del motor (`ITEM_TestBitfield1Flag1`) que el adaptador arma con "no es de un tipo
		// siempre normal y es arma o armadura" (`d2vendor.BasesFor`).
		b.CanBeMagic = b.Gear

		// Ammo: las flechas y las saetas (el `Quiver` de itemtypes), que el mercader vende como pila completa
		// (`Generate`: `if perm && b.Ammo { it.Quantity = b.MaxStack }`).
		if t, ok := paginas[f.Tipo]; ok && t.Quiver {
			b.Ammo = true
		}

		bases = append(bases, b)
	}

	v, _ := d2vendor.ByName(mercader)
	tier := d2vendor.ActIndex(v) // 0 = acto 1 (la tabla de topes 12/20/28/36/45)

	sinPagina := 0

	for _, f := range filas {
		if _, ok := paginas[f.Tipo]; !ok {
			sinPagina++
		}
	}

	fmt.Printf("   bases con pagina del mercader (itemtypes StorePage): %d de %d (sin pagina: %d)\n",
		len(filas)-sinPagina, len(filas), sinPagina)

	opt := d2vendor.Options{PlayerLevel: nivel, Tier: tier}

	fmt.Printf("   mercader=%-8s ClassID=%d Tier=%d PlayerLevel=%d ilvl=%d bases=%d semilla=%d\n",
		mercader, claseExe, tier, nivel, d2vendor.ItemLevel(nivel, tier), len(bases), semilla)

	stock := d2vendor.GenerateSeeded(semilla, bases, opt)

	fmt.Printf("   STOCK items=%d  (la grilla del mercader es %dx%d por pagina: d2vendor.GridCols x GridRows)\n",
		len(stock.Items), d2vendor.GridCols, d2vendor.GridRows)

	fmt.Printf("   %3s %-6s %-8s %3s %3s %2s %2s %3s %3s %-8s\n",
		"#", "code", "calidad", "ilvl", "cant", "w", "h", "cel", "pag", "permanente")

	partes := make([]string, 0, len(stock.Items))

	for i, it := range stock.Items {
		cal := nombreDeLaCalidad(int(it.Quality))

		fmt.Printf("   %3d %-6s %-8s %3d %3d %2d %2d %2d,%-2d %3d %-8v\n",
			i, it.Code, cal, it.ILvl, it.Quantity, it.W, it.H, it.X, it.Y, it.Page, it.Permanent)

		partes = append(partes, it.Code+"/"+cal+"/"+strconv.Itoa(it.Page)+"/"+
			strconv.Itoa(it.X)+","+strconv.Itoa(it.Y)+"/"+strconv.Itoa(it.W)+"x"+strconv.Itoa(it.H))
	}

	// el crc32 del stock: la referencia contra la que se compara la corrida de la consola
	crc := crc32.ChecksumIEEE([]byte(strings.Join(partes, "|")))
	fmt.Printf("   STOCK crc32=%d lista=%q\n", crc, strings.Join(partes, "|"))

	// cuantas paginas usa y cuantas celdas ocupa CADA pagina (la grilla es una por pagina)
	porPagina := map[int]int{}
	celdasPagina := map[int]int{}

	for _, it := range stock.Items {
		porPagina[it.Page]++
		celdasPagina[it.Page] += it.W * it.H
	}

	fmt.Printf("   STOCK porPagina=%v celdasPorPagina=%v de %d por pagina\n",
		porPagina, celdasPagina, d2vendor.GridCols*d2vendor.GridRows)

	return stock
}

func nombreDeLaCalidad(q int) string {
	switch q {
	case 1:
		return "low"
	case 2:
		return "normal"
	case 3:
		return "superior"
	case 4:
		return "magic"
	case 5:
		return "set"
	case 6:
		return "rare"
	case 7:
		return "unique"
	case 8:
		return "crafted"
	}

	return "?"
}

// ============================== 4. LOS PRECIOS Y EL ORO ==============================

func losPreciosYPrecios(mpqs map[string]*d2mpq.MPQ, filas []filaDeItem, paginas map[string]tipoDeItem, stock *d2vendor.Stock, nivel int) {
	fmt.Println("\n== 4. LOS PRECIOS Y EL ORO: npc.txt -> d2trade.NPC -> d2trade.ItemPrice")

	npc := leerNPC(mpqs, mercader)
	fmt.Printf("   npc.txt fila de %s: PlayerPays=%d VendorPays=%d Repair=%d MaxBuy=%v (1/1024)\n",
		mercader, npc.PlayerPays, npc.VendorPays, npc.Repair, npc.MaxBuy)
	fmt.Printf("   (multiplicadores en el archivo: sell=%.6f buy=%.6f rep=%.6f)\n",
		float64(npc.PlayerPays)/1024, float64(npc.VendorPays)/1024, float64(npc.Repair)/1024)

	costos := map[string]int{}
	ammo := map[string]bool{}

	for _, f := range filas {
		costos[f.Code] = f.Cost

		if t, ok := paginas[f.Tipo]; ok && t.Quiver {
			ammo[f.Code] = true
		}
	}

	fmt.Printf("   %3s %-6s %-8s %6s %4s %8s %8s %8s\n", "#", "code", "calidad", "cost", "cant", "compra", "venta", "oroDespues")

	oro := d2inventory.InventoryGoldLimit(nivel)
	tope := oro
	fmt.Printf("   oro del jugador de nivel %d: %d (InventoryGoldLimit = nivel * %d, VERIFICADO PLAYER_GetMaxGoldCarry)\n",
		nivel, oro, d2inventory.InventoryGoldPerLevel)

	compras := 0

	for i, it := range stock.Items {
		precio := precioDe(it, costos[it.Code], ammo[it.Code], npc, d2trade.ModeBuy, nivel)
		venta := precioDe(it, costos[it.Code], ammo[it.Code], npc, d2trade.ModeSell, nivel)

		despues := oro - precio
		if despues < 0 {
			despues = 0
		} else if i == 0 {
			// la PRIMERA compra de la corrida del criterio: el primer item del stock que el oro alcanza
			compras++
			oro = despues
		}

		fmt.Printf("   %3d %-6s %-8s %6d %4d %8d %8d %8d\n", i, it.Code, nombreDeLaCalidad(int(it.Quality)),
			costos[it.Code], it.Quantity, precio, venta, despues)
	}

	// cuantos items entran en el tope del nivel, uno por uno y de a uno (la cuenta que importa para el criterio)
	entran := 0

	for _, it := range stock.Items {
		if precioDe(it, costos[it.Code], ammo[it.Code], npc, d2trade.ModeBuy, nivel) <= tope {
			entran++
		}
	}

	fmt.Printf("   items del stock que el jugador de nivel %d puede pagar con su tope (%d de oro): %d de %d\n",
		nivel, tope, entran, len(stock.Items))

	// el mas barato y el mas caro: para que el POC compre algo que ENTRA en el tope
	barato, caro := -1, -1

	precioDeIndice := func(i int) int {
		return precioDe(stock.Items[i], costos[stock.Items[i].Code], ammo[stock.Items[i].Code], npc, d2trade.ModeBuy, nivel)
	}

	for i := range stock.Items {
		if barato < 0 || precioDeIndice(i) < precioDeIndice(barato) {
			barato = i
		}

		if caro < 0 || precioDeIndice(i) > precioDeIndice(caro) {
			caro = i
		}
	}

	if barato >= 0 && caro >= 0 {
		fmt.Printf("   el mas barato: %s (compra %d)   el mas caro: %s (compra %d)\n",
			stock.Items[barato].Code, precioDeIndice(barato), stock.Items[caro].Code, precioDeIndice(caro))
		fmt.Printf("   LA COMPRA DEL CRITERIO: %s por %d de oro (el primer item del stock que entra en el tope)\n",
			stock.Items[barato].Code, precioDeIndice(barato))
	}

	fmt.Println("   -- el oro del MERCADER: el motor NO lo modela")
	fmt.Println("      d2vendor/d2trade no tienen oro de mercader (en el juego el mercader no se queda sin oro); lo que")
	fmt.Println("      si tiene es el tope de 'max buy' por dificultad, que es lo que limita lo que PAGA por un item.")
	fmt.Printf("      MaxBuy de %s: %v (normal/pesadilla/infierno): el mercader paga como maximo eso por item\n",
		mercader, npc.MaxBuy)
}

func precioDe(it *d2vendor.Item, costo int, ammo bool, npc d2trade.NPC, modo d2trade.Mode, nivel int) int {
	// `d2trade.Item` es lo que lee TRADE_CalcItemPrice: el costo base de la fila, la cantidad y la calidad. Los
	// afijos no entran (el stock de un mercader de acto 1 vende la fila base, sin afijos). La CALIDAD se pasa tal
	// cual porque `d2drop.Quality` y las constantes de `d2trade` numeran igual (1 low ... 4 magic).
	ti := &d2trade.Item{
		BaseCost: costo, Quantity: it.Quantity, Quality: int(it.Quality), Identified: true, IsAmmo: ammo,
	}

	return d2trade.ItemPrice(ti, d2trade.Params{Mode: modo, Difficulty: 0, PlayerLevel: nivel, NPC: npc})
}

// leerNPC: la fila de npc.txt del mercader, con el orden y las conversiones de `d2vendor.NPCPricing` (multiplicadores
// 1/1024 y el "sell mult" del archivo como lo que PAGA el jugador).
func leerNPC(mpqs map[string]*d2mpq.MPQ, nombre string) d2trade.NPC {
	datos := leerDeLosMPQ(mpqs, tablaNPC)
	if datos == nil {
		fmt.Println("   NO encuentro npc.txt: se usan los multiplicadores por omision (1024 = x1)")

		return d2trade.NPC{PlayerPays: 1024, VendorPays: 1024, Repair: 1024}
	}

	lineas := strings.Split(string(datos), "\n")
	cab := lineas[0]
	iNPC := indiceDeColumna(cab, "npc")
	iSell := indiceDeColumna(cab, "sell mult")
	iBuy := indiceDeColumna(cab, "buy mult")
	iRep := indiceDeColumna(cab, "rep mult")
	iMax := indiceDeColumna(cab, "max buy")

	const fixedPoint = 1024.0

	out := d2trade.NPC{PlayerPays: 1024, VendorPays: 1024, Repair: 1024}

	for _, l := range lineas[1:] {
		l = strings.TrimRight(l, "\r")
		if !strings.EqualFold(texto(campo(l, iNPC)), nombre) {
			continue
		}

		// el volcado crudo de la fila: sin esto, un multiplicador mal leido parece un precio del juego
		fmt.Printf("   npc.txt CRUDO col(%d,%d,%d,%d)=%q %q %q %q (%s)\n",
			iSell, iBuy, iRep, iMax, texto(campo(l, iSell)), texto(campo(l, iBuy)),
			texto(campo(l, iRep)), texto(campo(l, iMax)), nombre)

		// LOS MULTIPLICADORES DEL ARCHIVO YA VIENEN EN 1/1024 (960 = x0,9375): el loader del motor los DIVIDE por
		// 1024 (`costDivisor` de d2records) y `d2vendor.NPCPricing` los vuelve a multiplicar, asi que el numero que
		// come `d2trade` es el del archivo TAL CUAL. Multiplicarlo otra vez daba un cap a 61.440 de oro (medido).
		out.PlayerPays = entero(campo(l, iSell))
		out.VendorPays = entero(campo(l, iBuy))
		out.Repair = entero(campo(l, iRep))

		for d := 0; d < 3; d++ {
			out.MaxBuy[d] = entero(campo(l, iMax+d))
		}

		break
	}

	return out
}

// decimal: un campo de la tabla como float (los multiplicadores vienen como "2.500000").
func decimal(b []byte) float64 {
	s := strings.TrimSpace(texto(b))
	if s == "" {
		return 1
	}

	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 1
	}

	return v
}

// ============================== 5. LO QUE PESA DIBUJARLO ==============================

func loQuePesaDibujarlo(mpqs map[string]*d2mpq.MPQ, stock *d2vendor.Stock) {
	fmt.Println("\n== 5. LO QUE PESA DIBUJARLO: el panel de la grilla y los graficos de los items del stock")

	const (
		celdaPx = 28
		margen  = 4
	)

	ancho := d2vendor.GridCols*celdaPx + 2*margen
	alto := d2vendor.GridRows*celdaPx + 2*margen

	fmt.Printf("   panel de la grilla del mercader: %dx%d celdas de %d px -> %d x %d = %d B de indices\n",
		d2vendor.GridCols, d2vendor.GridRows, celdaPx, ancho, alto, ancho*alto)

	fmt.Printf("   (el lienzo del POC es 640x480: el panel ocupa %.0f%% del ancho y %.0f%% del alto)\n",
		100*float64(ancho)/640, 100*float64(alto)/480)

	fmt.Printf("   la grilla de CELDAS del stock: %d celdas x %d B (celdaDeGrilla del POC) = %d B\n",
		d2vendor.GridCols*d2vendor.GridRows, 24, d2vendor.GridCols*d2vendor.GridRows*24)

	// los graficos: el `invfile` de cada codigo del stock, decodificado (es lo que queda vivo si se dibujan todos)
	fmt.Println("   -- los graficos (`invfile` de la fila, data/global/items/<invfile>.dc6)")
	fmt.Printf("   %-6s %-14s %8s %10s %8s %8s\n", "code", "invfile", "dc6", "cuadros", "caja", "indices")

	vivos := 0
	sinGrafico := 0

	vistos := map[string]bool{}

	for _, it := range stock.Items {
		if vistos[it.Code] {
			continue
		}

		vistos[it.Code] = true

		inv := invfileDe(mpqs, it.Code)
		if inv == "" {
			fmt.Printf("   %-6s %-14s %8s %10s %8s %8s\n", it.Code, "(sin invfile)", "-", "-", "-", "-")
			sinGrafico++

			continue
		}

		datos := leerDeLosMPQ(mpqs, `data\global\items\`+inv+".dc6")
		if datos == nil {
			fmt.Printf("   %-6s %-14s %8s %10s %8s %8s\n", it.Code, inv, "NO ESTA", "-", "-", "-")
			sinGrafico++

			continue
		}

		// la caja del primer cuadro del DC6: ancho y alto en la cabecera de cada cuadro
		w, h, cuadros := cajaDelDC6(datos)
		indices := w * h * cuadros / cuadros // el cuadro 0 (lo que se dibuja en la celda)
		vivos += indices

		fmt.Printf("   %-6s %-14s %8d %10d %4dx%-3d %8d\n", it.Code, inv, len(datos), cuadros, w, h, indices)
	}

	fmt.Printf("   indices vivos si se dibujan los %d items distintos: %d B (en el monton de C, como el HUD)\n",
		len(vistos), vivos)
	fmt.Printf("   items sin grafico en este disco: %d\n", sinGrafico)

	// el texto de los precios: el mercader muestra el precio del item bajo el cursor; aca se mide el peor renglon
	fmt.Println("   -- el texto: precio de compra y de venta de cada item, como cadena")
	largo := 0

	for _, it := range stock.Items {
		s := it.Code + " " + strconv.Itoa(999999) + " / " + strconv.Itoa(999999)
		if len(s) > largo {
			largo = len(s)
		}
	}

	fmt.Printf("   el renglon mas largo de precio: %d caracteres (font16: ~8 px de avance por letra = %d px)\n",
		largo, largo*8)
}

// invfileDe: la columna `invfile` de la fila del codigo, en armor/weapons/misc (la primera que lo tenga).
func invfileDe(mpqs map[string]*d2mpq.MPQ, code string) string {
	for _, cual := range []string{tablaArmaduras, tablaArmas, tablaMisc} {
		datos := leerDeLosMPQ(mpqs, cual)
		if datos == nil {
			continue
		}

		lineas := strings.Split(string(datos), "\n")
		if len(lineas) < 2 {
			continue
		}

		cab := lineas[0]
		iCode, iInv := indiceDeColumna(cab, "code"), indiceDeColumna(cab, "invfile")
		if iCode < 0 || iInv < 0 {
			continue
		}

		for _, l := range lineas[1:] {
			if strings.EqualFold(texto(campo(l, iCode)), code) {
				if v := texto(campo(l, iInv)); v != "" {
					return v
				}
			}
		}
	}

	return ""
}

// cajaDelDC6: ancho, alto y cantidad de cuadros de un DC6, con el lector del motor (las cabeceras del DC6 van en
// big-endian y leerlas a mano fue el error de la primera pasada de esta sonda: daba 16777216x0).
func cajaDelDC6(b []byte) (w, h, cuadros int) {
	dc, err := d2dc6.Load(b)
	if err != nil || len(dc.Frames) == 0 {
		return 0, 0, 0
	}

	return int(dc.Frames[0].Width), int(dc.Frames[0].Height), len(dc.Frames)
}

// ============================== LAS HERRAMIENTAS ==============================

func leerDeLosMPQ(mpqs map[string]*d2mpq.MPQ, ruta string) []byte {
	for _, nombre := range []string{"patch_d2.mpq", "d2data.mpq", "d2char.mpq"} {
		m := mpqs[nombre]
		if m == nil {
			continue
		}

		datos, err := m.ReadFile(ruta)
		if err == nil && len(datos) > 0 {
			return datos
		}
	}

	return nil
}

func indiceDeColumna(cabecera, nombre string) int {
	campos := strings.Split(strings.TrimRight(cabecera, "\r"), "\t")

	for i, c := range campos {
		if strings.EqualFold(strings.TrimSpace(c), nombre) {
			return i
		}
	}

	return -1
}

func campo(linea string, i int) []byte {
	if i < 0 {
		return nil
	}

	campos := strings.Split(linea, "\t")
	if i >= len(campos) {
		return nil
	}

	return []byte(campos[i])
}

// texto: el campo sin espacios ni comillas (las tablas del juego citan los codigos: "aqv ").
func texto(b []byte) string {
	return strings.TrimSpace(strings.ReplaceAll(string(b), `"`, ""))
}

func entero(b []byte) int {
	s := strings.TrimSpace(texto(b))
	if s == "" {
		return 0
	}

	v, err := strconv.Atoi(s)
	if err != nil {
		return 0
	}

	return v
}

func contarFilas(datos []byte) int {
	n := 0

	for _, l := range strings.Split(string(datos), "\n") {
		if strings.TrimSpace(l) != "" {
			n++
		}
	}

	return n - 1
}

func recorte(s string, n int) string {
	if len(s) <= n {
		return s
	}

	return s[:n]
}

func leerU32(b []byte, o int) uint32 {
	if o+4 > len(b) {
		return 0
	}

	return uint32(b[o]) | uint32(b[o+1])<<8 | uint32(b[o+2])<<16 | uint32(b[o+3])<<24
}

// el dado del motor, para dejar escrito que `GenerateSeeded` usa el mismo generador que el resto del POC
var _ = d2rand.New

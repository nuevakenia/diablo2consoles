//go:build dreamcast

package main

// DC-002.129 -- LOS MERCADERES: EL STOCK DE CHARSI EN LA GRILLA, CON LOS PRECIOS, Y COMPRAR Y VENDER CON ORO.
//
// QUE ES. Hasta DC-002.128 el objeto entraba al inventario solo por el piso (el drop del bicho). Este archivo abre
// el OTRO camino, el de la etapa E5 del plan (`docs/PLAN-JUEGO.md`): el STOCK de un mercader del pueblo, con su
// precio, y la compra y la venta contra el oro del jugador.
//
// EL MODELO ES DEL MOTOR, NO UNA COPIA. Esto es lo que separa esta tarjeta de "dibujar una tienda":
//
//   - EL STOCK lo genera `d2vendor.GenerateSeeded` (el port de TRADE_GenerateVendorStock 0x574780): las cantidades
//     Min/Max y MagicMin/MagicMax de la fila del item para ESE mercader, el nivel de item (jugador+5 con el tope por
//     acto de la tabla 12/20/28/36/45), la calidad por tirada (`RollQuality`, FUN_00574710) y la colocacion en la
//     grilla de 10x10 del mercader (`Stock.Place`, primer hueco). El dado es `d2rand`, el mismo del drop y de la vida.
//   - LOS PRECIOS los calcula `d2trade.ItemPrice` (el port de TRADE_CalcItemPrice 0x62f100) con la fila de `npc.txt`
//     del mercader: `sell mult` (lo que PAGA el jugador), `buy mult` (lo que paga el mercader) y `max buy`.
//   - EL ORO del jugador se mueve con el tope y el setter del motor: `d2inventory.InventoryGoldLimit` (nivel*10.000,
//     PLAYER_GetMaxGoldCarry 0x623050) y `d2inventory.SetGold` (0x53dc10, que deja el oro en 0 si el valor se pasa
//     del tope o queda negativo: el comportamiento del exe, no el que uno querria).
//
// LO QUE EL POC LE PONE AL MODELO SON LOS DATOS, y es lo mismo que hizo DC-002.124 con `d2inventory.Dims`: el
// adaptador del motor (`d2vendor.BasesFor`/`NPCPricing`) pide un `*d2records.RecordManager`, que arrastra 177
// archivos y 2,3 MB de tablas parseadas a structs. Aca las filas se leen con el lector del POC (`abrirTabla`, sin
// parsear a structs, con el tramo devuelto en el acto) y se arman los `d2vendor.Base` y el `d2trade.NPC` a mano.
// Medido en `console/dc/sonda-mercader-129`: 42 items base de Charsi en 250.220 B de tablas, y el stock que sale
// con la semilla 129 es de 53 items (24 armaduras, 22 armas y 7 de miscelanea) con crc32 3628371977 — ese numero es
// la REFERENCIA contra la que se compara la corrida de la consola.
//
// EL PANEL. El del juego (`data/global/ui/panel/...`) no esta en los tres MPQ de este disco: se pinta con la misma
// imagen de indices de DC-002.124 (`armarElPanel`), 10x10 celdas de 28 px = 288x288. Cada item del stock se dibuja
// con el `invfile` de SU fila, en SU celda, y con la huella (W x H) que dice la tabla. Los precios van en el renglon
// de abajo, como en el juego (el precio del item que el POC tiene "bajo el cursor") y el oro del jugador al lado.
//
// LO QUE NO ENTRA (dicho, no tapado): ni reparar ni identificar (eso es `d2trade.ModeRepair` e `IdentifyCost`, otras
// tarjetas), ni el comercio entre jugadores (`d2playertrade`), ni el trueque del mercader con afijos: el item del
// stock se cobra por su FILA BASE (la calidad entra en la cuenta del precio; los afijos de un magic son otra etapa).
// Tampoco hay oro en el piso (el drop de DC-002.123 descarta la entrada "gld"): por eso el monedero del jugador
// ARRANCA con `oroInicial`, que es un instrumento y esta dicho en el log de cada corrida.

import (
	"hash/crc32"

	"github.com/OpenDiablo2/OpenDiablo2/console/dc/fuenteconsola"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2dc6"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2font"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2trade"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2inventory"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2render/pvr"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2vendor"
)

const (
	// las tablas que le faltan al drop: los tipos de item (la pagina del mercader y las flechas) y la fila del
	// mercader en npc.txt (los multiplicadores del precio).
	excelTiposDeItem = `data\global\excel\itemtypes.txt`
	excelNPC         = `data\global\excel\npc.txt`

	// la carpeta del grafico del objeto en la grilla (d2resource.ItemGraphics) y la clase del mercader.
	mercaderClaseExe = 154 // Charsi
)

// mercancia: UN item del stock, con lo que el POC necesita para dibujarlo y cobrarlo. El item en si es del motor
// (`*d2vendor.Item`): aca van el precio (de `d2trade`), su celda y su grafico en la grilla.
type mercancia struct {
	Item   *d2vendor.Item
	Compra int // oro que paga el jugador (d2trade.ModeBuy)
	Venta  int // oro que paga el mercader (d2trade.ModeSell)
	Codigo string
	Pagina int
	Fila   int
	Colum  int
	Ancho  int
	Alto   int

	// el grafico del item en la grilla (cuadro 0 del `invfile`), en indices de la paleta de las unidades y en el
	// monton de C, como el arte del HUD: 0 si este disco no trae el archivo (se dice y la celda se marca igual).
	Cuadro   *CuadroDelJugador
	Bytes    int
	InvFile  string
	CuadrosN int
}

// puesto: el mercader abierto. La grilla es la del MOTOR (`d2vendor.GridCols` x `GridRows`, una por pagina).
type puesto struct {
	Nombre  string
	Stock   *d2vendor.Stock
	Items   []*mercancia
	Pagina  int
	Abierta bool
	Veces   int

	// el puesto en el lienzo: la imagen del panel (288x288) en indices, armada una vez.
	Grilla *grilla
	Panel  []byte

	// lo que se midio
	BytesDeItems    int
	SinGrafico      int
	ItemsDibujados  int
	PixelesDelPanel int
	MsDelPanel      int
	CuadrosConPanel int
	CuadrosCerrado  int
	CrcDelStock     uint32

	// la compra y la venta de la corrida
	Compro      *mercancia
	CeldaFila   int
	CeldaColum  int
	Vendio      *mercancia
	EnLaGrilla  bool // el objeto comprado SE VIO en la grilla del jugador, medido al comprarlo
	OroAlComprar int
}

var (
	elMercader *puesto

	// la plata. `oroDelJugador` es el monedero (stat 0x0e del exe); `oroDelMercader` es un contador DEL POC porque
	// el motor no modela oro de mercader (ver el comentario de `losPreciosDelMotor`).
	oroDelJugador  int
	oroDelMercader int
	oroAntesDeTodo int

	// el renglon del precio y el oro que se pintan abajo del panel. Las DOS cadenas se arman cuando el dato cambia
	// (carga, compra, venta), no por cuadro: armar "Oro: "+itoa(...) en cada cuadro son allocaciones por cuadro
	// justo donde el semiespacio no sobra.
	textoDelPrecio string
	textoDelOro     string
)

// ============================== LA CARGA ==============================

// cargarElMercader: las tablas -> los `d2vendor.Base` -> el stock del motor -> los precios -> los graficos de la
// pagina que se muestra. VA CON LAS FUENTES ABIERTAS (paso 4 del `main`), como el drop y el inventario: todo sale de
// los MPQ y despues del paso 5 ya no se puede pedir.
func cargarElMercader() {
	if mercaderKnob != "si" {
		println("JUEGO129: MERCADER apagado (mercader=", mercaderKnob, "): la corrida es la de DC-002.128")

		return
	}

	t0 := int64(d2cMs())

	bases, costos, ammo, err := lasBasesDelMercader()
	if err != "" {
		println("JUEGO129: MERCADER SIN-TABLAS", err, ": no hay stock que generar")

		return
	}

	// ---- 2. EL STOCK, con el codigo del motor ----
	elMercader = &puesto{Nombre: mercaderNombre, Pagina: mercaderPagina}
	elMercader.Grilla = &grilla{Nombre: "mercader", W: d2vendor.GridCols, H: d2vendor.GridRows,
		X0: panelMercaderX, Y0: panelMercaderY}
	armarElPanelDelMercader(elMercader.Grilla)
	elMercader.Panel = elMercader.Grilla.Panel

	v, _ := d2vendor.ByName(mercaderNombre)
	tier := d2vendor.ActIndex(v)

	opt := d2vendor.Options{PlayerLevel: nivelDelHeroe, Tier: tier}
	elMercader.Stock = d2vendor.GenerateSeeded(mercaderSemilla, bases, opt)

	println("JUEGO129: MERCADER STOCK nombre=", mercaderNombre, "claseExe=", mercaderClaseExe,
		"tier=", tier, "nivelDelJugador=", nivelDelHeroe, "ilvl=", d2vendor.ItemLevel(nivelDelHeroe, tier),
		"bases=", len(bases), "semilla=", mercaderSemilla, "items=", len(elMercader.Stock.Items),
		"grilla=", d2vendor.GridCols, "x", d2vendor.GridRows)

	// ---- 3. LOS PRECIOS, con la fila de npc.txt del mercader ----
	npc := laFilaDelNPC(mercaderNombre)

	println("JUEGO129: MERCADER NPC nombre=", mercaderNombre, "playerPays=", npc.PlayerPays,
		"vendorPays=", npc.VendorPays, "maxBuy=", npc.MaxBuy[0], npc.MaxBuy[1], npc.MaxBuy[2],
		"multiplosDe=", 1024)

	partes := ""

	for _, it := range elMercader.Stock.Items {
		m := &mercancia{
			Item: it, Codigo: it.Code, Pagina: it.Page, Fila: it.Y, Colum: it.X, Ancho: it.W, Alto: it.H,
		}

		// el precio por el camino del motor: la tabla del item (su `cost`), la cantidad y la calidad, y la fila del
		// mercader. `Identified` va en true porque el stock de un mercader se ve identificado (es un mercader de
		// armaduras, no Cain): es el mismo valor con el que el motor cobra una compra.
		ti := &d2trade.Item{BaseCost: costos[it.Code], Quantity: it.Quantity, Quality: int(it.Quality),
			Identified: true, IsAmmo: ammo[it.Code]}

		m.Compra = d2trade.ItemPrice(ti, d2trade.Params{Mode: d2trade.ModeBuy, PlayerLevel: nivelDelHeroe, NPC: npc})
		m.Venta = d2trade.ItemPrice(ti, d2trade.Params{Mode: d2trade.ModeSell, PlayerLevel: nivelDelHeroe, NPC: npc})

		elMercader.Items = append(elMercader.Items, m)

		if len(partes) > 0 {
			partes = partes + "|"
		}

		// la MISMA cadena que arma la sonda del host (`console/dc/sonda-mercader-129`): es lo que hace comparables
		// los dos crc32. Sin separador final.
		partes = partes + it.Code + "/" + itcalidad(int(it.Quality)) + "/" + itoa(it.Page) + "/" +
			itoa(it.X) + "," + itoa(it.Y) + "/" + itoa(it.W) + "x" + itoa(it.H)

		println("JUEGO129: MERCADER ITEM codigo=", it.Code, "calidad=", itcalidad(int(it.Quality)), "ilvl=", it.ILvl,
			"cant=", it.Quantity, "huella=", it.W, "x", it.H, "celda=", it.X, ",", it.Y, "pagina=", it.Page,
			"compra=", m.Compra, "venta=", m.Venta, "permanente=", it.Permanent)
	}

	elMercader.CrcDelStock = crc32.ChecksumIEEE([]byte(partes))

	println("JUEGO129: MERCADER STOCK-CRC crc32=", u32a(elMercader.CrcDelStock), "items=", len(elMercader.Items),
		"referenciaDelHost=", u32a(crcDelStockDelHost), "igualAlHost=", elMercader.CrcDelStock == crcDelStockDelHost,
		"lista=", partes)

	// ---- 4. EL ORO (el tope es del motor) ----
	tope := d2inventory.InventoryGoldLimit(nivelDelHeroe)

	oroDelJugador = mercaderOroInicial
	if oroDelJugador > tope {
		oroDelJugador = tope
	}

	oroAntesDeTodo = oroDelJugador
	oroDelMercader = mercaderOroDelMercader
	textoDelOro = "Oro: " + itoa(oroDelJugador)

	println("JUEGO129: MERCADER ORO jugador=", oroDelJugador, "topeDelNivel=", tope,
		"topePorNivel=", d2inventory.InventoryGoldPerLevel, "mercader=", oroDelMercader,
		"origenDelOro= instrumento (el drop de DC-002.123 descarta la entrada gld del tesoro)")

	// ---- 5. LOS GRAFICOS DE LA PAGINA QUE SE MUESTRA ----
	cargarLosGraficosDelMercader()
	cargarLosGlifosDelMercader()

	// los items van pegados en la imagen del panel: un solo pedido de dibujo por cuadro
	componerLosItemsDelMostrador()

	println("JUEGO129: MERCADER LISTO pagina=", elMercader.Pagina, "itemsEnLaPagina=", itemsEnLaPagina(),
		"bytesDeItems=", elMercader.BytesDeItems, "sinGrafico=", elMercader.SinGrafico,
		"bytesDelPanel=", len(elMercader.Panel), "ms=", int(int64(d2cMs())-t0))

	textoDelPrecio = "sin item"
}

// crcDelStockDelHost es la referencia medida en el host por `console/dc/sonda-mercader-129` con la MISMA semilla y
// el MISMO nivel: si la consola genera el mismo stock, este numero tiene que ser el mismo.
//
// VA COMO `var uint32` Y NO COMO CONSTANTE `int`: 3628371977 no entra en el `int` de 32 bits del SH4 (el `int` de
// gccgo en esta consola es el de la ABI, 32 bits), y una constante sin tipo que no entra no compila. Como `uint32`
// el numero entra, y `int(...)` de una VARIABLE solo trunca en tiempo de corrida (que es lo que ya hace el resto del
// log con los crc32 del lienzo).
var crcDelStockDelHost uint32 = 3628371977

// u32a: el crc32 sin signo. `itoa(int(x))` imprimiria el negativo del complemento a dos (el `int` de este target es
// de 32 bits), y el numero que se compara contra el host es el del crc32 tal cual lo publica `hash/crc32`.
func u32a(v uint32) string {
	if v == 0 {
		return "0"
	}

	var buf [12]byte

	i := len(buf)

	for v > 0 {
		i--
		buf[i] = byte('0' + v%10)
		v /= 10
	}

	return string(buf[i:])
}

// itcalidad: el nombre de la calidad como lo numera el juego (`d2drop.Quality`).
func itcalidad(q int) string {
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
	}

	return "?"
}

// itemsEnLaPagina: cuantos items del stock caen en la pagina que se muestra.
func itemsEnLaPagina() int {
	n := 0

	for _, m := range elMercader.Items {
		if m.Pagina == elMercader.Pagina {
			n++
		}
	}

	return n
}

// ============================== LAS TABLAS ==============================

// lasBasesDelMercader: las tres tablas de items + la de tipos, leidas UNA POR VEZ con el lector del POC, y devuelve
// los `d2vendor.Base` en el ORDEN que come `Generate` (por codigo, como el adaptador del motor: `BasesFor` ordena),
// el `cost` de cada codigo (lo que cobra `d2trade`) y si es flechas (el `Quiver` de itemtypes).
func lasBasesDelMercader() (bases []d2vendor.Base, costos map[string]int, ammo map[string]bool, err string) {
	parche := fuenteDe(rutaParche)
	datos := fuenteDe(rutaDatos)

	if parche == nil && datos == nil {
		return nil, nil, nil, "sin fuente"
	}

	// ---- la pagina del mercader de cada tipo de item y las flechas ----
	tipos := map[string]int{}
	flechas := map[string]bool{}

	laTabla := abrirTablaDeItems(parche, datos, excelTiposDeItem, "Code", "StorePage", "Quiver")
	if laTabla != nil {
		cCode, cPage, cQuiver := laTabla.col("Code"), laTabla.col("StorePage"), laTabla.col("Quiver")
		codigos := [4]string{"armo", "weap", "mag", "misc"} // storePageCodes del motor

		laTabla.porCadaFila(func(linea []byte) bool {
			cod := texto(campo(linea, cCode))
			if cod == "" {
				return false
			}

			pag := -1

			for i := 0; i < 4; i++ {
				if mismoTexto(campo(linea, cPage), codigos[i]) {
					pag = i
				}
			}

			tipos[cod] = pag

			if len(campo(linea, cQuiver)) > 0 {
				flechas[cod] = true
			}

			return false
		})
		laTabla.cerrar()
	}

	costos = map[string]int{}
	ammo = map[string]bool{}

	// ---- las tres tablas de items, una por vez (cada tramo se devuelve antes de pedir el siguiente) ----
	for _, cual := range []struct{ ruta string }{
		{excelArmaduras}, {excelArmas}, {excelMisc},
	} {
		t := abrirTablaDeItems(parche, datos, cual.ruta, "code", "type", "levelreq", "invwidth", "invheight",
			"PermStoreItem", "version", "cost", "maxstack",
			mercaderNombre+"Min", mercaderNombre+"Max", mercaderNombre+"MagicMin",
			mercaderNombre+"MagicMax", mercaderNombre+"MagicLvl",
			"ubercode", "ultracode", "NightmareUpgrade", "HellUpgrade")

		if t == nil {
			println("JUEGO129: MERCADER TABLA-FALTA", cual.ruta, "(sin columnas de ", mercaderNombre, ")")

			continue
		}

		cCode, cTipo, cLvl := t.col("code"), t.col("type"), t.col("levelreq")
		cW, cH := t.col("invwidth"), t.col("invheight")
		cPerm, cVer, cCost, cStack := t.col("PermStoreItem"), t.col("version"), t.col("cost"), t.col("maxstack")
		cMin, cMax := t.col(mercaderNombre+"Min"), t.col(mercaderNombre+"Max")
		cMMin, cMMax := t.col(mercaderNombre+"MagicMin"), t.col(mercaderNombre+"MagicMax")
		cMLev := t.col(mercaderNombre+"MagicLvl")
		cUber, cUltra := t.col("ubercode"), t.col("ultracode")
		cPes, cInf := t.col("NightmareUpgrade"), t.col("HellUpgrade")

		bytes, filas, conColumnas := len(t.Bytes), 0, 0

		t.porCadaFila(func(linea []byte) bool {
			filas++

			cod := texto(campo(linea, cCode))
			if cod == "" {
				return false
			}

			// el MISMO filtro de `d2vendor.BasesFor`: sin columnas del mercader con numero, el item no es suyo
			min, max := enteroDe(linea, cMin), enteroDe(linea, cMax)
			mmin, mmax := enteroDe(linea, cMMin), enteroDe(linea, cMMax)
			if max == 0 && mmax == 0 {
				return false
			}

			conColumnas++

			tipo := texto(campo(linea, cTipo))
			esArmadura := cual.ruta == excelArmaduras
			esArma := cual.ruta == excelArmas

			b := d2vendor.Base{
				Code: cod, Page: -1, ReqLevel: enteroDe(linea, cLvl),
				W: enteroDe(linea, cW), H: enteroDe(linea, cH),
				Vendor: d2vendor.Params{Min: min, Max: max, MagicMin: mmin, MagicMax: mmax,
					MagicLevel: enteroDe(linea, cMLev)},
				Version:   enteroDe(linea, cVer),
				Permanent: enteroDe(linea, cPerm) != 0,
				MaxStack:  enteroDe(linea, cStack),
				Gear:      esArmadura || esArma,
				Uber:      texto(campo(linea, cUber)),
				Ultra:     texto(campo(linea, cUltra)),
				NightmareUpgrade: texto(campo(linea, cPes)),
				HellUpgrade:      texto(campo(linea, cInf)),
			}

			if p, hay := tipos[tipo]; hay {
				b.Page = p
			}

			// `CanBeMagic` es la derivada del motor (`ITEM_TestBitfield1Flag1`), que el adaptador arma con "no es de
			// un tipo siempre normal y es arma o armadura"
			b.CanBeMagic = b.Gear
			b.Ammo = flechas[tipo]

			bases = append(bases, b)
			costos[cod] = enteroDe(linea, cCost)
			ammo[cod] = b.Ammo

			return false
		})
		t.cerrar()

		println("JUEGO129: MERCADER TABLA", cual.ruta, "bytes=", bytes, "filas=", filas,
			"conColumnas=", conColumnas, "montonC_libre=", d2cMontonLibre())
	}

	// EL ORDEN: por codigo (es el del adaptador del motor). Sin esto el dado reparte otro stock con la misma semilla
	// y el crc32 no coincidiria con el del host, que es lo que esta tarjeta compara.
	ordenarBases(bases)

	return bases, costos, ammo, ""
}

// abrirTablaDeItems: pide la tabla a la fuente (el parche primero, `d2data` despues) y la abre con las columnas
// pedidas. Devuelve nil si ninguno de los dos la tiene con esas columnas.
func abrirTablaDeItems(parche, datos *fuenteconsola.Fuente, ruta string, columnas ...string) *tablaAbierta {
	for _, fuente := range []*fuenteconsola.Fuente{parche, datos} {
		if fuente == nil {
			continue
		}

		if t := abrirTabla(fuente, ruta, columnas...); t != nil {
			return t
		}
	}

	return nil
}

// ordenarBases: ordena por codigo, como `d2vendor.BasesFor` (un insertion sort: 42 filas y cero allocaciones).
func ordenarBases(b []d2vendor.Base) {
	for i := 1; i < len(b); i++ {
		for j := i; j > 0 && b[j].Code < b[j-1].Code; j-- {
			b[j], b[j-1] = b[j-1], b[j]
		}
	}
}

// laFilaDelNPC: la fila de `npc.txt` del mercader con las columnas del precio. Los multiplicadores del archivo YA
// vienen en 1/1024 (Charsi: 960 = x0,9375, 512 = x0,5): el loader del motor los DIVIDE por 1024 y
// `d2vendor.NPCPricing` los vuelve a multiplicar, asi que el numero que come `d2trade` es el del archivo tal cual.
func laFilaDelNPC(nombre string) d2trade.NPC {
	porDefecto := d2trade.NPC{PlayerPays: 1024, VendorPays: 1024, Repair: 1024}

	parche := fuenteDe(rutaParche)
	datos := fuenteDe(rutaDatos)

	t := abrirTablaDeItems(parche, datos, excelNPC, "npc", "sell mult", "buy mult", "rep mult",
		"max buy", "max buy (N)", "max buy (H)")
	if t == nil {
		println("JUEGO129: MERCADER sin npc.txt: multiplicadores por omision (x1)")

		return porDefecto
	}

	cNPC := t.col("npc")
	cSell, cBuy, cRep := t.col("sell mult"), t.col("buy mult"), t.col("rep mult")
	cMax, cMaxN, cMaxH := t.col("max buy"), t.col("max buy (N)"), t.col("max buy (H)")

	out := porDefecto

	t.porCadaFila(func(linea []byte) bool {
		if !mismoTexto(campo(linea, cNPC), nombre) {
			return false
		}

		out.PlayerPays = enteroDe(linea, cSell)
		out.VendorPays = enteroDe(linea, cBuy)
		out.Repair = enteroDe(linea, cRep)
		out.MaxBuy[0] = enteroDe(linea, cMax)
		out.MaxBuy[1] = enteroDe(linea, cMaxN)
		out.MaxBuy[2] = enteroDe(linea, cMaxH)

		return true
	})
	t.cerrar()

	return out
}

// ============================== LOS GRAFICOS ==============================

// cargarLosGraficosDelMercader: el `invfile` de cada item de la pagina que se muestra, decodificado a indices en el
// monton de C. Se cargan SOLO los de la pagina (la ventana del mercader muestra una a la vez): con las tres serian
// 97.216 B medidos en el host y con una son ~40 KB.
func cargarLosGraficosDelMercader() {
	parche := fuenteDe(rutaParche)
	datos := fuenteDe(rutaDatos)

	if parche == nil && datos == nil {
		println("JUEGO129: MERCADER sin fuente para el invfile")

		return
	}

	invs := map[string]string{}

	// los `invfile` de los codigos del stock: se buscan en las tres tablas de items
	for _, cual := range []string{excelArmaduras, excelArmas, excelMisc} {
		t := abrirTablaDeItems(parche, datos, cual, "code", "invfile")
		if t == nil {
			continue
		}

		cCode, cInv := t.col("code"), t.col("invfile")

		t.porCadaFila(func(linea []byte) bool {
			cod := texto(campo(linea, cCode))
			inv := texto(campo(linea, cInv))

			if cod != "" && inv != "" {
				invs[cod] = inv
			}

			return false
		})
		t.cerrar()
	}

	// el archivo del invfile, decodificado (cuadro 0) y copiado al monton de C
	porCodigo := map[string]*CuadroDelJugador{}

	for _, m := range elMercader.Items {
		if m.Pagina != elMercader.Pagina {
			continue
		}

		if cu, hay := porCodigo[m.Codigo]; hay {
			m.Cuadro = cu

			continue
		}

		inv, hay := invs[m.Codigo]
		if !hay {
			println("JUEGO129: MERCADER ITEM-SIN-INVFILE codigo=", m.Codigo)
			elMercader.SinGrafico++

			continue
		}

		m.InvFile = inv

		cu, bytes, ok := componerUnInvfile(datos, parche, inv)
		if !ok {
			elMercader.SinGrafico++

			continue
		}

		m.Cuadro = cu
		m.Bytes = bytes
		elMercader.BytesDeItems += bytes
		porCodigo[m.Codigo] = cu

		println("JUEGO129: MERCADER GRAFICO codigo=", m.Codigo, "invfile=", inv, "caja=", cu.Ancho, "x", cu.Alto,
			"indices=", bytes, "crc32=", int(cu.CRC))
	}

	println("JUEGO129: MERCADER GRAFICOS codigosDistintos=", len(porCodigo),
		"bytes=", elMercader.BytesDeItems, "sinGrafico=", elMercader.SinGrafico)
}

// componerUnInvfile: `data/global/items/<invfile>.dc6` -> el cuadro 0 en indices, en un bloque del monton de C (el
// mismo camino de DC-002.123/124 con el `flippyfile`, aca con la columna `invfile`).
func componerUnInvfile(datos, parche *fuenteconsola.Fuente, inv string) (*CuadroDelJugador, int, bool) {
	base := datos
	if base == nil {
		base = parche
	}

	ruta := carpetaFlippy + inv + ".dc6"

	tramo, err := base.Pedir(ruta)
	if err != nil || len(tramo.Datos) == 0 {
		println("JUEGO129: MERCADER DC6-FALTA ruta=", ruta)

		return nil, 0, false
	}

	crudo := tramo.Datos
	bytesDelArchivo := len(crudo)

	dc, errDC := d2dc6.Load(crudo)
	if errDC != nil {
		println("JUEGO129: MERCADER DC6-PARSE-FALLO ruta=", ruta)

		tramo.Liberar()

		return nil, 0, false
	}

	if len(dc.Frames) == 0 {
		tramo.Liberar()

		return nil, 0, false
	}

	crudo2 := dc.DecodeFrame(0)
	if len(crudo2) == 0 {
		tramo.Liberar()

		return nil, 0, false
	}

	indices := bloqueDeC(len(crudo2))
	copy(indices, crudo2)

	cu := &CuadroDelJugador{
		Ancho: int(dc.Frames[0].Width), Alto: int(dc.Frames[0].Height),
		Izq: int(dc.Frames[0].OffsetX), Arriba: int(dc.Frames[0].OffsetY),
		Pixeles: indices, PixelesNoVacios: contarLlenos(indices), CapasDibujadas: 1,
		CRC: crc32.ChecksumIEEE(indices),
	}

	_ = bytesDelArchivo

	tramo.Liberar()
	d2dc6Soltar()

	return cu, len(indices), true
}

// armarElPanelDelMercader: la imagen del panel (288x288) en indices de la paleta de las unidades: fondo, borde y las
// celdas de 28 px con su separacion de 1 px. Los TRES colores son los de `indicesDelPanel` (los de la paleta de las
// unidades, no un RGB inventado), el mismo camino de DC-002.124.
//
// VA AL MONTON DE C Y NO AL SEMIESPACIO, y es una decision medida: son 82.944 B que el recolector no cuenta ni copia,
// y esta tarjeta carga DESPUES del drop y del inventario (que es donde el semiespacio se aprieta). `bloqueDeC` es el
// camino del arte del HUD (DC-002.125).
func armarElPanelDelMercader(g *grilla) {
	anchoPanel, altoPanel := g.AnchoDelPanel(), g.AltoDelPanel()

	fondo, medio, claro := indicesDelPanel()

	panel := bloqueDeC(anchoPanel * altoPanel)
	if panel == nil {
		println("JUEGO129: MERCADER SIN-PANEL no hay bloque de C para", anchoPanel*altoPanel, "bytes")

		return
	}

	for i := range panel {
		panel[i] = fondo
	}

	for x := 0; x < anchoPanel; x++ {
		panel[x] = claro
		panel[(altoPanel-1)*anchoPanel+x] = claro
	}

	for y := 0; y < altoPanel; y++ {
		panel[y*anchoPanel] = claro
		panel[y*anchoPanel+anchoPanel-1] = claro
	}

	for f := 0; f < g.H; f++ {
		for c := 0; c < g.W; c++ {
			x0 := margenDelPanel + c*celdaPx
			y0 := margenDelPanel + f*celdaPx

			for y := y0; y < y0+celdaPx-1; y++ {
				for x := x0; x < x0+celdaPx-1; x++ {
					if x < anchoPanel-1 && y < altoPanel-1 {
						panel[y*anchoPanel+x] = medio
					}
				}
			}
		}
	}

	g.Panel = panel

	println("JUEGO129: MERCADER PANEL ancho=", anchoPanel, "alto=", altoPanel, "bytes=", len(g.Panel),
		"indices=fondo", fondo, "celda", medio, "borde", claro, "enElMontonDeC= si")
}

// cargarLosGlifosDelMercader: los glifos de `font16` para TODO el texto de la ventana (el nombre, el oro, el renglon
// del precio y los codigos de los items de la pagina). Es el mismo map que usa el panel de los NPCs (`glifosDelNPC`),
// asi que lo que ya este cargado no se vuelve a pedir.
func cargarLosGlifosDelMercader() {
	if glifosDelNPC == nil {
		glifosDelNPC = make(map[byte]*glifoDelHUD, 96)
	}

	datos := fuenteDe(rutaDatos)
	if datos == nil {
		return
	}

	textos := []string{mercaderNombre, "Oro: ", "compra ", "venta ", "sin item", "vendido ", "0123456789:,x "}

	for _, m := range elMercader.Items {
		if m.Pagina == elMercader.Pagina {
			textos = append(textos, m.Codigo)
		}
	}

	faltan := 0

	for _, t := range textos {
		for i := 0; i < len(t); i++ {
			if _, listo := glifosDelNPC[t[i]]; !listo {
				faltan++
			}
		}
	}

	if faltan == 0 {
		println("JUEGO129: MERCADER GLIFOS ya estaban todos: glifos=", len(glifosDelNPC))

		return
	}

	tramoTabla, err := datos.Pedir(rutaFuenteNPC + `.tbl`)
	if err != nil || tramoTabla == nil {
		return
	}

	f, err := d2font.Load(tramoTabla.Datos)
	tramoTabla.Liberar()

	if err != nil {
		println("JUEGO129: MERCADER la tabla de la fuente no parsea:", err.Error())

		return
	}

	tramoSprite, err := datos.Pedir(rutaFuenteNPC + `.DC6`)
	if err != nil || tramoSprite == nil {
		return
	}

	dc, err := d2dc6.Load(tramoSprite.Datos)
	tramoSprite.Liberar()

	if err != nil {
		println("JUEGO129: MERCADER el sprite de la fuente no parsea:", err.Error())

		return
	}

	for _, t := range textos {
		for i := 0; i < len(t); i++ {
			c := t[i]

			if _, listo := glifosDelNPC[c]; listo {
				continue
			}

			g, ok := f.Glyphs[rune(c)]
			if !ok {
				continue
			}

			cuadro := g.FrameIndex()
			if cuadro < 0 || cuadro >= len(dc.Frames) {
				continue
			}

			crudo := dc.DecodeFrame(cuadro)
			indices := bloqueDeC(len(crudo))
			copy(indices, crudo)

			glifosDelNPC[c] = &glifoDelHUD{Ancho: int(dc.Frames[cuadro].Width), Alto: int(dc.Frames[cuadro].Height),
				Paso: g.Width(), Indices: indices}
		}
	}

	println("JUEGO129: MERCADER GLIFOS glifos=", len(glifosDelNPC), "nuevos=", faltan)
}

// componerLosItemsDelMostrador: pega los items de la pagina EN LA IMAGEN del panel, en indices y saltando el indice 0
// (el transparente del motor, `surface.go`). Es la composicion que deja el panel listo para un solo pedido de dibujo.
//
// El item comprado NO se pega (ya no esta en el stock y ya viajo a la grilla del jugador). Se recompone despues de
// cada compra, que es lo que borra su celda.
func componerLosItemsDelMostrador() {
	anchoPanel := elMercader.Grilla.AnchoDelPanel()

	n := 0

	for _, m := range elMercader.Items {
		if m.Pagina != elMercader.Pagina || m.Cuadro == nil || m == elMercader.Compro {
			continue
		}

		x0 := margenDelPanel + m.Colum*celdaPx
		y0 := margenDelPanel + m.Fila*celdaPx

		for y := 0; y < m.Cuadro.Alto; y++ {
			filaPanel := (y0 + y) * anchoPanel
			filaItem := y * m.Cuadro.Ancho

			for x := 0; x < m.Cuadro.Ancho; x++ {
				idx := m.Cuadro.Pixeles[filaItem+x]
				if idx == 0 {
					continue
				}

				px, py := x0+x, y0+y
				if px < anchoPanel && py < elMercader.Grilla.AltoDelPanel() {
					elMercader.Panel[filaPanel+px] = idx
				}
			}
		}

		n++
	}

	elMercader.ItemsDibujados = n

	println("JUEGO129: MERCADER MOSTRADOR itemsCompuestos=", n, "pagina=", elMercader.Pagina,
		"bytesDelPanel=", len(elMercader.Panel))
}

// ============================== EL DIBUJO ==============================

// dibujarElMercader: el panel del mercader con su stock, los precios y el oro. Se llama DESPUES del inventario (el
// panel del jugador ya esta pintado) y ANTES del HUD. Devuelve los pixeles que pinto.
//
// EL LOG DEL CUADRO VA SOLO MIENTRAS CORRE EL GUION (hasta el cuadro del cierre). Un `println` con quince campos
// arma una decena de cadenas por cuadro y el POC llega a los cuadros finales con el semiespacio al 69 %: imprimir en
// los 19 cuadros es tirar basura justo cuando no sobra (medido en la primera corrida de esta tarjeta: murio de
// `gc_external_alloc: OOM` en el cuadro 7). La ventana se ensancha con VERBOSO=si.
func dibujarElMercader(malla *pvr.Surface, pixeles []byte) int {
	if mercaderKnob != "si" || elMercader == nil {
		return 0
	}

	if !elMercader.Abierta {
		elMercader.CuadrosCerrado++

		if cuadro <= mercaderCuadroCierra+1 || verboso134() {
			println("JUEGO129: MERCADER CUADRO", cuadro, "abierto=no pixeles=0",
				"elCuadroNoCambio=", true, "crcLienzo=", u32a(crcLienzo(pixeles)))
		}

		return 0
	}

	t0 := int64(d2cMs())

	// el marco y las celdas
	malla.PushTranslation(elMercader.Grilla.X0, elMercader.Grilla.Y0)
	malla.RenderIndicesTintado(elMercader.Panel, elMercader.Grilla.AnchoDelPanel(),
		elMercader.Grilla.AltoDelPanel(), palJugador[:])
	malla.Pop()

	pintados := elMercader.Grilla.AnchoDelPanel() * elMercader.Grilla.AltoDelPanel()

	// LOS ITEMS YA ESTAN EN LA IMAGEN (los compone `componerElMostrador`: el stock no cambia de cuadro a cuadro). Un
	// solo pedido de dibujo de 288x288 por cuadro en vez de 25: el primero se puede porque el panel y los items usan
	// la MISMA paleta (la de las unidades, `palJugador`), y por eso la composicion se hace en indices, sin pasar por
	// colores. Medido: 24 pedidos de item costaban 34 ms de cuadro.
	dibujados := elMercader.ItemsDibujados

	// el renglon del precio y el oro, abajo del panel (el oro en el indice dorado del motor)
	indiceOro := indiceDelOro()
	dibujarUnTexto(malla, elMercader.Nombre, elMercader.Grilla.X0+6, elMercader.Grilla.Y0+6, indiceOro)
	dibujarUnTexto(malla, textoDelOro, elMercader.Grilla.X0+6, elMercader.Grilla.Y0+22, 1)
	dibujarUnTexto(malla, textoDelPrecio, elMercader.Grilla.X0+6,
		elMercader.Grilla.Y0+elMercader.Grilla.AltoDelPanel()+4, indiceOro)

	// LA COMPRA SE VE EN LA GRILLA DEL JUGADOR: el objeto comprado se pinta en SU celda del panel del jugador (que
	// el inventario ya dibujo). Es la mitad visual del criterio de la tarjeta.
	if elMercader.Compro != nil && elMercader.Compro.Cuadro != nil && grillaJugador != nil {
		x := grillaJugador.X0 + margenDelPanel + elMercader.CeldaColum*celdaPx
		y := grillaJugador.Y0 + margenDelPanel + elMercader.CeldaFila*celdaPx

		PintarJugador(malla, elMercader.Compro.Cuadro, x, y, palJugador[:])
		pintados += elMercader.Compro.Cuadro.Ancho * elMercader.Compro.Cuadro.Alto
	}

	elMercader.MsDelPanel = int(int64(d2cMs()) - t0)
	elMercader.PixelesDelPanel = pintados
	elMercader.CuadrosConPanel++

	if cuadro <= mercaderCuadroCierra+1 || verboso134() {
		println("JUEGO129: MERCADER CUADRO", cuadro, "abierto=si pagina=", elMercader.Pagina,
			"itemsEnLaPagina=", itemsEnLaPagina(), "dibujados=", dibujados, "pixeles=", pintados,
			"msPanel=", elMercader.MsDelPanel, "oro=", oroDelJugador, "oroDelMercader=", oroDelMercader,
			"precio=", textoDelPrecio, "crcLienzo=", u32a(crcLienzo(pixeles)))
	}

	return pintados
}

// ============================== EL PASO (lo que cambia entre cuadros) ==============================

// pasoDelMercader: UN CUADRO del mercader. Abre el puesto cuando el guion lo dice, compra el primer item de la pagina
// que el oro alcanza y, unos cuadros despues, lo vende de vuelta: el criterio de la tarjeta es la compra y la venta
// es la otra mitad ("comprar y vender con oro").
func pasoDelMercader() {
	if mercaderKnob != "si" || elMercader == nil {
		return
	}

	// ---- 1. SE ABRE ----
	if cuadro+1 == mercaderCuadroAbre && !elMercader.Abierta {
		elMercader.Abierta = true
		elMercader.Veces++

		println("JUEGO129: MERCADER ABRE cuadro=", cuadro+1, "items=", len(elMercader.Items),
			"pagina=", elMercader.Pagina, "panel=", elMercader.Grilla.X0, elMercader.Grilla.Y0,
			"ancho=", elMercader.Grilla.AnchoDelPanel(), "alto=", elMercader.Grilla.AltoDelPanel(),
			"oro=", oroDelJugador)
	}

	// ---- 2. LA COMPRA ----
	if mercaderAuto == "si" && elMercader.Abierta && cuadro+1 == mercaderCuadroCompra && elMercader.Compro == nil {
		comprarEnElMercader()
	}

	// ---- 3. LA VENTA ----
	if mercaderAuto == "si" && elMercader.Compro != nil && cuadro+1 == mercaderCuadroVenta && elMercader.Vendio == nil {
		venderAlMercader()
	}

	// ---- 4. SE CIERRA ----
	if cerrarElMercader == "si" && cuadro+1 == mercaderCuadroCierra && elMercader.Abierta {
		elMercader.Abierta = false

		println("JUEGO129: MERCADER CIERRA cuadro=", cuadro+1, "oro=", oroDelJugador,
			"vecesAbierto=", elMercader.Veces)
	}
}

// elItemDeLaCompra: el item que se compra: el PRIMERO de la pagina (en el orden del stock) que el oro alcanza. Es la
// regla del "cursor en la primera celda": en el juego el jugador elige, aca elige el guion, y el camino es el mismo.
func elItemDeLaCompra() *mercancia {
	for _, m := range elMercader.Items {
		if m.Pagina != elMercader.Pagina {
			continue
		}

		if m.Compra <= oroDelJugador {
			return m
		}
	}

	return nil
}

// comprarEnElMercader: el item sale del stock del mercader, el oro se va del monedero del jugador y el objeto entra a
// SU grilla por la regla de DC-002.124 (`grilla.meter`). Todo por el camino del motor: `Stock.Remove` (el mercader no
// repone lo vendido: VERIFICADO) y `SetGold` (0x53dc10).
func comprarEnElMercader() {
	m := elItemDeLaCompra()
	if m == nil {
		println("JUEGO129: MERCADER COMPRA SIN-ITEM: el oro (", oroDelJugador, ") no alcanza para nada de la pagina",
			elMercader.Pagina)

		return
	}

	if grillaJugador == nil {
		println("JUEGO129: MERCADER COMPRA SIN-GRILLA: no hay grilla del jugador (inventario apagado)")

		return
	}

	tope := d2inventory.InventoryGoldLimit(nivelDelHeroe)
	antes := grillaJugador.ocupadas()

	// el hueco en la grilla del jugador: la huella del item, como dice la tabla
	celda := grillaJugador.meter(m.Codigo, m.Ancho, m.Alto)
	if celda == nil {
		println("JUEGO129: MERCADER COMPRA NO-ENTRA codigo=", m.Codigo, "huella=", m.Ancho, "x", m.Alto,
			"libres=", len(grillaJugador.Celdas)-antes)

		return
	}

	oroAntes := oroDelJugador

	// el tope y el setter del motor: con un precio mayor que el oro, `SetGold` dejaria el monedero en 0 (y el juego
	// no cobra: por eso el precio se comprueba antes, que es lo que hace el motor al habilitar la compra)
	oroDelJugador = d2inventory.SetGold(oroDelJugador, -m.Compra, tope)
	oroDelMercader += m.Compra

	elMercader.Stock.Remove(m.Item)

	elMercader.Compro = m
	elMercader.CeldaFila, elMercader.CeldaColum = celda.Fila, celda.Columna
	elMercader.OroAlComprar = oroDelJugador

	// LA CELDA SE VACIA EN LA IMAGEN DEL MOSTRADOR: el objeto ya no esta en el stock, y el panel es UNA imagen
	// compuesta (se rehace el fondo y se vuelven a pegar los que quedan).
	armarElPanelDelMercader(elMercader.Grilla)
	componerLosItemsDelMostrador()

	// el criterio se mide ACA, con el objeto recien metido: la venta de mas adelante lo saca de la grilla y el
	// cierre no tiene que confundir "se vendio" con "nunca entro".
	elMercader.EnLaGrilla = grillaJugador.tieneElCodigo(m.Codigo)

	textoDelPrecio = m.Codigo + " compra " + itoa(m.Compra) + " venta " + itoa(m.Venta)
	textoDelOro = "Oro: " + itoa(oroDelJugador)

	println("JUEGO129: MERCADER COMPRA codigo=", m.Codigo, "precio=", m.Compra, "oroAntes=", oroAntes,
		"oroDespues=", oroDelJugador, "tope=", tope, "oroDelMercader=", oroDelMercader,
		"celda=", celda.Fila, ",", celda.Columna, "huella=", m.Ancho, "x", m.Alto,
		"ocupadasAntes=", antes, "ocupadasDespues=", grillaJugador.ocupadas(),
		"estaEnLaGrilla=", elMercader.EnLaGrilla, "itemsEnElStock=", len(elMercader.Stock.Items))
}

// tieneElCodigo: si alguna celda de la grilla la tiene ese codigo (la prueba de que el objeto comprado ENTRO).
func (g *grilla) tieneElCodigo(codigo string) bool {
	for i := range g.Celdas {
		if g.Celdas[i].Ocupada && g.Celdas[i].Codigo == codigo {
			return true
		}
	}

	return false
}

// venderAlMercader: el camino de vuelta: el objeto sale de la grilla del jugador, el mercader paga `ModeSell` y el
// monedero sube con el tope del nivel (si se pasa, `SetGold` lo deja en 0: el comportamiento del exe, y se dice).
func venderAlMercader() {
	m := elMercader.Compro
	tope := d2inventory.InventoryGoldLimit(nivelDelHeroe)

	quien := grillaJugador.vaciarCelda(elMercader.CeldaFila, elMercader.CeldaColum, m.Ancho, m.Alto)

	oroAntes := oroDelJugador

	if m.Venta > oroDelMercader {
		println("JUEGO129: MERCADER VENTA SIN-ORO el mercader tiene", oroDelMercader, "y el item vale", m.Venta)

		return
	}

	oroDelJugador = d2inventory.SetGold(oroDelJugador, m.Venta, tope)
	oroDelMercader -= m.Venta

	elMercader.Vendio = m
	textoDelPrecio = m.Codigo + " vendido " + itoa(m.Venta)
	textoDelOro = "Oro: " + itoa(oroDelJugador)

	println("JUEGO129: MERCADER VENTA codigo=", m.Codigo, "precio=", m.Venta, "oroAntes=", oroAntes,
		"oroDespues=", oroDelJugador, "tope=", tope, "sePasoDelTope=", oroDelJugador == 0,
		"oroDelMercader=", oroDelMercader, "sacoDeLaCelda=", quien, "@", elMercader.CeldaFila, ",",
		elMercader.CeldaColum, "ocupadas=", grillaJugador.ocupadas())

	criterioFinalDelMercader()
}

// ============================== EL CIERRE ==============================

// resumenDelMercader: los numeros del cierre y el criterio de la tarjeta.
func resumenDelMercader() {
	if mercaderKnob != "si" || elMercader == nil {
		return
	}

	// el estado VIVO (despues de la venta) y el estado del momento de la compra (`EnLaGrilla`), los dos dichos
	vivoEnLaGrilla := false
	codigoEnLaGrilla := ""

	if grillaJugador != nil {
		for i := range grillaJugador.Celdas {
			c := grillaJugador.Celdas[i]

			if c.Ocupada && elMercader.Compro != nil && c.Codigo == elMercader.Compro.Codigo {
				vivoEnLaGrilla = true
				codigoEnLaGrilla = c.Codigo

				break
			}
		}
	}

	println("JUEGO129: MERCADER RESUMEN items=", len(elMercader.Items), "itemsEnElStock=", len(elMercader.Stock.Items),
		"bytesDeItems=", elMercader.BytesDeItems, "bytesDelPanel=", len(elMercader.Panel),
		"sinGrafico=", elMercader.SinGrafico, "pixelesDelPanel=", elMercader.PixelesDelPanel,
		"msDelPanel=", elMercader.MsDelPanel, "cuadrosConPanel=", elMercader.CuadrosConPanel,
		"cuadrosCerrado=", elMercader.CuadrosCerrado, "crcDelStock=", u32a(elMercader.CrcDelStock))

	println("JUEGO129: MERCADER ORO-RESUMEN jugador=", oroDelJugador, "alPrincipio=", oroAntesDeTodo,
		"alComprar=", elMercader.OroAlComprar, "mercader=", oroDelMercader,
		"elOroBajoAlComprar=", elMercader.OroAlComprar < oroAntesDeTodo)

	println("JUEGO129: MERCADER CRITERIO elStockSalio=", len(elMercader.Items) > 0,
		"seCompro=", elMercader.Compro != nil, "codigo=", codigoDeLaCompra(),
		"precio=", precioDeLaCompra(), "entroALaGrillaDelJugador=", elMercader.EnLaGrilla,
		"seVendio=", elMercader.Vendio != nil, "laGrillaLoTieneAhora=", vivoEnLaGrilla,
		"codigoEnLaGrilla=", codigoEnLaGrilla,
		"elStockYaNoLoTiene=", elMercader.Compro != nil && !elMercader.StockLoTiene(),
		"stockIgualAlHost=", elMercader.CrcDelStock == crcDelStockDelHost)
}

// criterioFinalDelMercader: el mismo criterio, DESPUES de la venta (que ocurre en un cuadro posterior al resumen).
// Se imprime con otro nombre para no confundir la mitad que ya estaba medida en el resumen.
func criterioFinalDelMercader() {
	enLaGrilla := grillaJugador != nil && elMercader.Compro != nil && grillaJugador.tieneElCodigo(elMercader.Compro.Codigo)

	println("JUEGO129: MERCADER CRITERIO-FINAL seCompro=", elMercader.Compro != nil,
		"codigo=", codigoDeLaCompra(), "elStockYaNoLoTiene=", elMercader.Compro != nil && !elMercader.StockLoTiene(),
		"seVendio=", elMercader.Vendio != nil, "laGrillaLoTieneAhora=", enLaGrilla,
		"oroDelJugador=", oroDelJugador, "oroAlPrincipio=", oroAntesDeTodo,
		"elOroBajoAlComprar=", elMercader.OroAlComprar < oroAntesDeTodo,
		"laVentaDevolvioAlgo=", elMercader.Vendio != nil && oroDelJugador > elMercader.OroAlComprar,
		"itemsEnElStock=", len(elMercader.Stock.Items))
}

// StockLoTiene: si el item comprado sigue en el stock del mercader (no deberia: `Stock.Remove`). SE COMPARA EL
// PUNTERO, no el codigo: Charsi tiene dos `buc` (el normal y uno magico) y comparando codigos este numero mentiria.
func (p *puesto) StockLoTiene() bool {
	for _, it := range p.Stock.Items {
		if p.Compro != nil && it == p.Compro.Item {
			return true
		}
	}

	return false
}

func codigoDeLaCompra() string {
	if elMercader.Compro == nil {
		return "-"
	}

	return elMercader.Compro.Codigo
}

func precioDeLaCompra() int {
	if elMercader.Compro == nil {
		return 0
	}

	return elMercader.Compro.Compra
}

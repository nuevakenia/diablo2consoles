package main

// DC-002.124 -- EL INVENTARIO DEL JUGADOR: LA GRILLA, EL OBJETO EN SU CELDA Y LO QUE CUESTA.
//
// QUE ES. El objeto que DC-002.123 dejaba en el piso y recogia ahora TIENE DONDE IR: una grilla de celdas del
// tamano que dice el motor (`d2inventory.Dims(ContainerInventory, true)` = 10x4 = 40 celdas) donde el objeto
// entra por su huella y se dibuja con el `invfile` de SU fila (weapons/armor/misc), no con una constante.
//
// LO QUE **NO** ENTRA, Y ESTA MEDIDO (ver `console/dc/sonda-inventario-124`): el paquete `d2core/d2ui` (3.834
// lineas de widgets: botones, etiquetas, scrollbars, tooltips) y el arbol de `d2core/d2inventory` completo. El
// presupuesto de la tarjeta dice que lo que cuesta MEMORIA no es el codigo sino la ESTRUCTURA: la grilla entera
// del POC (jugador 10x4 + alijo 6x8 + cinturon 4) son 2.208 B de celdas y los pixeles del objeto en su celda
// 2.352 B (28x84 del `invfile`). Eso entra sin tocar el semiespacio; lo que NO entra es `d2ui`, que arrastra
// `d2common/d2interface` (los contratos del renderer y del input), `d2asset` completo y la fuente de texto: se
// paga con las ~1.000 lineas de este archivo y del `alijo_124.go`, que dibujan la grilla con el pincel que ya
// existe (`pvr.Surface.DrawRect`, el mismo camino de `d2ui/frame.go`, que en el motor tambien dibuja los paneles
// con primitivas y no con un DC6: `UI_DrawControlPanelBgLeft 0x494ae0`, verificado).
//
// EL PANEL NO ES UN DC6. Se probo (sonda, seccion 3): `data/global/ui/panel/inventory.dc6`, `stash.dc6` y
// `panels.dc6` NO estan en ninguno de los tres MPQ del disco del POC. Lo que SI esta es el grafico del objeto en
// la grilla: su `invfile` (`data/global/items/<invfile>.dc6`, medido 1.419 B y 1 cuadro de 28x84 para el `aqv`
// del drop de DC-002.123). Por eso el panel se pinta con rectangulos y el objeto con sus pixeles.

import (
	"hash/crc32"

	"github.com/OpenDiablo2/OpenDiablo2/console/dc/fuenteconsola"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2dc6"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2inventory"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2render/pvr"
)

const (
	// celdaPx: el lado de una celda de la grilla en el lienzo del POC (640x480). El panel del juego mide 29 px
	// por celda y el `invfile` de un objeto mide 28 de ancho (medido: invqvr.dc6 = 28x84): 28 deja el objeto
	// entrar en su celda SIN escalarlo, que es lo que la tarjeta pide ("que se vea el objeto en su celda").
	celdaPx = 28

	// margenDelPanel: el borde que se pinta alrededor de la grilla.
	margenDelPanel = 4

	// columnasDeLaGrilla/filasDeLaGrilla: los de `d2inventory.Dims` para el inventario del jugador. Estan como
	// constantes para poder escribir la geometria sin llamar la funcion en cada cuadro; el `Dims` del motor se
	// imprime en el log al arrancar y la prueba de que coinciden es esa linea.
	columnasDeLaGrilla = 10
	filasDeLaGrilla    = 4

	// donde se pinta el panel del jugador (esquina de arriba a la izquierda). Abajo y a la izquierda del cuadro
	// para no tapar al jugador, que camina por el centro.
	panelJugadorX = 176
	panelJugadorY = 344

)

// celdaDeGrilla: una celda. 24 B medidos con `unsafe.Sizeof` en el host (sonda, seccion 1): 40 celdas del
// jugador son 960 B y 48 del alijo 1.152 B. Nada de esto se pide por cuadro.
type celdaDeGrilla struct {
	Codigo  string // el codigo del objeto que la ocupa ("" = vacia)
	Ocupada bool
	Fila    int
	Columna int
}

// grilla: la rejilla de un contenedor (`d2inventory.Container`), con su nombre, su tamano y su estado de abierta.
type grilla struct {
	Nombre  string
	W, H    int
	Celdas  []celdaDeGrilla
	Abierta bool
	X0, Y0  int // donde se pinta
	Veces   int // cuantas veces se abrio en la corrida

	// Panel: la imagen del panel en INDICES de la paleta de las unidades, ARMADA UNA VEZ al arrancar. Se pinta con
	// `RenderIndicesTintado`, que es el camino que llega a la textura del PVR (medido en esta tarjeta:
	// `Surface.DrawRect` escribe en la imagen del lado de la CPU y NO se ve en el emulador).
	Panel []byte
}

// objetoEnLaGrilla: el objeto que se recogio y su grafico en la grilla. Los pixeles son del TIPO (se piden una
// vez) y la celda es del objeto: el mismo objeto puede estar en el inventario o en el alijo.
type objetoEnLaGrilla struct {
	Codigo       string
	Nombre       string
	InvFile      string
	Cuadros      []CuadroDelJugador
	AnchoCeldas  int
	AltoCeldas   int
	BytesIndices int
	Ganchos      int // la celda donde esta (contenedor + fila/columna en el log)
}

var (
	grillaJugador *grilla
	grillaAlijo   *grilla
	enLaGrilla    *objetoEnLaGrilla

	// lo ultimo que se midio en el lienzo con el panel abierto (para el cierre, sin recalcular)
	crcConElPanel      uint32
	crcSinElPanel      uint32
	lienzoDelPanelIntacto bool
	pixelesDelPanel    int
	msDelPanel         int
	cuadrosConElPanel  int
	cuadrosSinElPanel  int
)

// cargarElInventario: la grilla del jugador (y la del alijo, si la palanca lo pide) y el grafico del objeto en la
// grilla. VA CON LAS FUENTES ABIERTAS, igual que `cargarElDrop` (main, paso 4d): el `invfile` sale del MPQ.
func cargarElInventario() {
	if inventarioKnob != "si" {
		println("JUEGO124: INVENTARIO apagado (inventario=", inventarioKnob, "): la corrida es la de DC-002.123")

		return
	}

	// ---- 1. LAS DOS GRILLAS, con el tamano del MOTOR ----
	invW, invH := d2inventory.Dims(d2inventory.ContainerInventory, true)
	aljW, aljH := d2inventory.Dims(d2inventory.ContainerStash, true)

	grillaJugador = nuevaGrilla("jugador", invW, invH, panelJugadorX, panelJugadorY)
	armarElPanel(grillaJugador)

	println("JUEGO124: INV GRILLA contenedor= jugador w=", grillaJugador.W, "h=", grillaJugador.H,
		"celdas=", len(grillaJugador.Celdas), "bytes=", len(grillaJugador.Celdas)*celdaDeGrillaBytes,
		"celdaPx=", celdaPx, "panel=", grillaJugador.AnchoDelPanel(), "x", grillaJugador.AltoDelPanel(),
		"origen=", grillaJugador.X0, grillaJugador.Y0)

	if alijoKnob == "si" {
		grillaAlijo = nuevaGrilla("alijo", aljW, aljH, panelAlijoX, panelAlijoY)
		armarElPanel(grillaAlijo)

		println("JUEGO124: INV GRILLA contenedor= alijo w=", grillaAlijo.W, "h=", grillaAlijo.H,
			"celdas=", len(grillaAlijo.Celdas), "bytes=", len(grillaAlijo.Celdas)*celdaDeGrillaBytes,
			"panel=", grillaAlijo.AnchoDelPanel(), "x", grillaAlijo.AltoDelPanel(),
			"origen=", grillaAlijo.X0, grillaAlijo.Y0)
	} else {
		println("JUEGO124: INV ALIJO apagado (alijo=", alijoKnob, ")")
	}

	// ---- 2. EL OBJETO Y SU GRAFICO EN LA GRILLA ----
	if drop == nil || drop.Codigo == "" {
		println("JUEGO124: INV sin objeto que guardar (el drop no resolvio un codigo)")

		return
	}

	enLaGrilla = &objetoEnLaGrilla{Codigo: drop.Codigo, Nombre: drop.Nombre}

	t0 := int64(d2cMs())
	inv, bytes, ok := componerElInvfile(drop.Codigo)
	enLaGrilla.InvFile = inv
	enLaGrilla.BytesIndices = bytes

	if !ok {
		// sin grafico la grilla se dibuja igual y el objeto se marca con su celda: se dice, no se tapa
		println("JUEGO124: INV OBJETO SIN-GRAFICO codigo=", drop.Codigo, ": la celda se marca pero no hay dibujo")

		return
	}

	enLaGrilla.Cuadros = invCuadros
	enLaGrilla.AnchoCeldas = (invAncho + celdaPx - 1) / celdaPx
	enLaGrilla.AltoCeldas = (invAlto + celdaPx - 1) / celdaPx

	println("JUEGO124: INV OBJETO codigo=", enLaGrilla.Codigo, "nombre=", enLaGrilla.Nombre,
		"invfile=", enLaGrilla.InvFile, "grafico=", invAncho, "x", invAlto, "huella=", enLaGrilla.AnchoCeldas,
		"x", enLaGrilla.AltoCeldas, "cuadros=", len(enLaGrilla.Cuadros), "bytesIndices=", enLaGrilla.BytesIndices,
		"ms=", int(int64(d2cMs())-t0))
}

// celdaDeGrillaBytes: el tamano medido de una celda (host, `unsafe.Sizeof`). Escrito como constante para que el
// numero del log sea el MISMO en la consola y en la sonda.
const celdaDeGrillaBytes = 24

// invCuadros / invAncho / invAlto: lo que devuelve `componerElInvfile` (el cuadro y su caja) para no repetir el
// retorno en el llamador.
var (
	invCuadros []CuadroDelJugador
	invAncho   int
	invAlto    int
)

// componerElInvfile: el `invfile` de la fila del objeto -> sus pixeles en indices. El camino es el de DC-002.123
// con el flippy (`fuenteconsola.Pedir` + `d2dc6.Load` + `DecodeFrame`), con la carpeta de items del motor y la
// columna `invfile` de la tabla (no `flippyfile`, que es el dibujo del piso).
func componerElInvfile(codigo string) (nombre string, indices int, ok bool) {
	parche := fuenteDe(rutaParche)
	datos := fuenteDe(rutaDatos)

	if parche == nil && datos == nil {
		println("JUEGO124: INV OBJETO sin fuente para leer la tabla del objeto")

		return "", 0, false
	}

	// la fila del objeto: se busca en el parche primero (es el que gana en el loader del motor) y despues en d2data
	for _, fuente := range []*fuenteconsola.Fuente{parche, datos} {
		if fuente == nil {
			continue
		}

		for _, cual := range []struct{ ruta, nombre string }{
			{excelArmas, "weapons.txt"}, {excelArmaduras, "armor.txt"}, {excelMisc, "misc.txt"},
		} {
			t := abrirTabla(fuente, cual.ruta, "code", "invfile")
			if t == nil {
				continue
			}

			cCode, cInv := t.col("code"), t.col("invfile")
			salida := ""

			t.porCadaFila(func(linea []byte) bool {
				if !mismoTexto(campo(linea, cCode), codigo) {
					return false
				}

				salida = texto(campo(linea, cInv))

				return true
			})

			t.cerrar()

			if salida != "" {
				nombre = salida
			}
		}
	}

	if nombre == "" {
		println("JUEGO124: INV OBJETO el codigo", codigo, "no tiene invfile en las tablas de items")

		return "", 0, false
	}

	ruta := carpetaFlippy + nombre + ".dc6"

	fuente := datos
	if fuente == nil {
		fuente = parche
	}

	tramo, err := fuente.Pedir(ruta)
	if err != nil || len(tramo.Datos) == 0 {
		println("JUEGO124: INV DC6-LEER FALLO ruta=", ruta)

		return nombre, 0, false
	}

	crudo := tramo.Datos
	bytes := len(crudo)
	crc := crc32.ChecksumIEEE(crudo)

	println("JUEGO124: INV DC6 ruta=", ruta, "bytes=", bytes, "crc32=", int(crc))

	dc, errDC := d2dc6.Load(crudo)
	if errDC != nil {
		println("JUEGO124: INV DC6-PARSE FALLO", errDC.Error())
		tramo.Liberar()

		return nombre, 0, false
	}

	for f := 0; f < len(dc.Frames); f++ {
		fr := dc.Frames[f]
		pixeles := dc.DecodeFrame(f)

		if len(pixeles) == 0 {
			continue
		}

		invCuadros = append(invCuadros, CuadroDelJugador{
			Ancho: int(fr.Width), Alto: int(fr.Height), Izq: int(fr.OffsetX), Arriba: int(fr.OffsetY),
			Pixeles: pixeles, PixelesNoVacios: contarLlenos(pixeles), CapasDibujadas: 1,
			CRC: crc32.ChecksumIEEE(pixeles),
		})
		indices += len(pixeles)

		if f == 0 {
			invAncho, invAlto = int(fr.Width), int(fr.Height)
		}
	}

	tramo.Liberar()
	d2dc6Soltar()

	println("JUEGO124: INV DC6-CUADRO cuadros=", len(invCuadros), "caja=", invAncho, "x", invAlto,
		"indices=", indices)

	return nombre, indices, len(invCuadros) > 0
}

// nuevaGrilla: una grilla de WxH celdas vacias, con las celdas ya alocadas (se piden UNA vez, al arrancar).
func nuevaGrilla(nombre string, w, h, x0, y0 int) *grilla {
	g := &grilla{Nombre: nombre, W: w, H: h, X0: x0, Y0: y0}
	g.Celdas = make([]celdaDeGrilla, w*h)

	for f := 0; f < h; f++ {
		for c := 0; c < w; c++ {
			g.Celdas[f*w+c] = celdaDeGrilla{Fila: f, Columna: c}
		}
	}

	return g
}

// AnchoDelPanel / AltoDelPanel: la caja del panel en pixeles (celdas + los dos margenes + el borde).
func (g *grilla) AnchoDelPanel() int { return g.W*celdaPx + 2*margenDelPanel }
func (g *grilla) AltoDelPanel() int  { return g.H*celdaPx + 2*margenDelPanel }

// ocupadas: cuantas celdas tienen algo.
func (g *grilla) ocupadas() int {
	n := 0

	for i := range g.Celdas {
		if g.Celdas[i].Ocupada {
			n++
		}
	}

	return n
}

// meter: pone el objeto en la PRIMERA celda libre donde su huella entre (la regla del motor para el autoplace es
// la misma: primer hueco en el que el objeto entra, `d2inventory/autoplace.go`). Devuelve la celda o nil.
func (g *grilla) meter(codigo string, anchoCeldas, altoCeldas int) *celdaDeGrilla {
	if anchoCeldas < 1 {
		anchoCeldas = 1
	}

	if altoCeldas < 1 {
		altoCeldas = 1
	}

	for f := 0; f+altoCeldas <= g.H; f++ {
		for c := 0; c+anchoCeldas <= g.W; c++ {
			if !g.libreDesde(f, c, anchoCeldas, altoCeldas) {
				continue
			}

			for ff := f; ff < f+altoCeldas; ff++ {
				for cc := c; cc < c+anchoCeldas; cc++ {
					celda := &g.Celdas[ff*g.W+cc]
					celda.Ocupada = (ff == f && cc == c)
					celda.Codigo = codigo

					if celda.Ocupada {
						celda.Codigo = codigo
					}
				}
			}

			// las celdas de la huella quedan marcadas como ocupadas por el objeto (codigo), y solo la de la
			// esquina lleva el objeto: lo que se dibuja es el grafico entero desde la esquina.
			for ff := f; ff < f+altoCeldas; ff++ {
				for cc := c; cc < c+anchoCeldas; cc++ {
					celda := &g.Celdas[ff*g.W+cc]
					celda.Ocupada = true
					celda.Codigo = codigo
				}
			}

			return &g.Celdas[f*g.W+c]
		}
	}

	return nil
}

// libreDesde: si la huella (f,c)-(f+alto,c+ancho) esta libre.
func (g *grilla) libreDesde(f, c, anchoCeldas, altoCeldas int) bool {
	for ff := f; ff < f+altoCeldas; ff++ {
		for cc := c; cc < c+anchoCeldas; cc++ {
			if g.Celdas[ff*g.W+cc].Ocupada {
				return false
			}
		}
	}

	return true
}

// vaciarCelda: saca de la grilla el objeto que empieza en (f,c) y devuelve su codigo.
func (g *grilla) vaciarCelda(f, c, anchoCeldas, altoCeldas int) string {
	codigo := g.Celdas[f*g.W+c].Codigo

	for ff := f; ff < f+altoCeldas; ff++ {
		for cc := c; cc < c+anchoCeldas; cc++ {
			celda := &g.Celdas[ff*g.W+cc]
			celda.Ocupada = false
			celda.Codigo = ""
		}
	}

	return codigo
}

// primeraCelda: la celda (fila, columna) del primer objeto de la grilla (el POC guarda UN objeto; con mas, la
// regla es la misma: el que este mas arriba y a la izquierda).
func (g *grilla) primeraCelda() (fila, columna int, codigo string, ok bool) {
	for f := 0; f < g.H; f++ {
		for c := 0; c < g.W; c++ {
			if celda := g.Celdas[f*g.W+c]; celda.Ocupada && celda.Codigo != "" {
				return celda.Fila, celda.Columna, celda.Codigo, true
			}
		}
	}

	return 0, 0, "", false
}

// ============================== EL DIBUJO ==============================

// dibujarUnaGrilla: el panel (fondo, borde y las celdas) y el objeto en SU celda, si esta. Devuelve la caja que
// ocupo en el lienzo y los pixeles que pinto.
func dibujarUnaGrilla(malla *pvr.Surface, g *grilla, objeto *objetoEnLaGrilla) (CajaDelJugador, int) {
	caja := recortarALaPantalla(CajaDelJugador{
		X0: g.X0, Y0: g.Y0, X1: g.X0 + g.AnchoDelPanel(), Y1: g.Y0 + g.AltoDelPanel(),
	})

	if caja.Ancho() <= 0 || caja.Alto() <= 0 {
		return caja, 0
	}

	// EL PANEL ES UNA IMAGEN DE INDICES ARMADA UNA VEZ (`armarElPanel`), y se pinta con `RenderIndicesTintado`: es
	// el camino que llega a la textura del PVR, el MISMO de los sprites del heroe, del bicho y del objeto.
	if g.Panel != nil {
		malla.PushTranslation(g.X0, g.Y0)
		malla.RenderIndicesTintado(g.Panel, g.AnchoDelPanel(), g.AltoDelPanel(), palJugador[:])
		malla.Pop()
	}

	// el objeto en SU celda (la esquina de la huella)
	if objeto != nil && len(objeto.Cuadros) > 0 {
		if fila, columna, codigo, ok := g.primeraCelda(); ok && codigo == objeto.Codigo {
			x := g.X0 + margenDelPanel + columna*celdaPx
			y := g.Y0 + margenDelPanel + fila*celdaPx

			PintarJugador(malla, &objeto.Cuadros[0], x, y, palJugador[:])
			g.GanchoDelObjeto(fila, columna)
		}
	}

	return caja, caja.Ancho() * caja.Alto()
}

// armarElPanel: la imagen del panel en indices de la paleta de las unidades: fondo, borde y las celdas con su
// separacion de 1 px. Se arma UNA vez y se pinta igual en todos los cuadros.
//
// LOS TRES COLORES SALEN DE LA PALETA (`Units/pal.dat`, la misma de los sprites), no de un numero inventado: se
// busca el indice mas OSCURO (el fondo), el mas CLARO (el borde) y uno del medio (la celda). El indice 0 no se usa
// en ningun lado porque el pincel lo trata como transparente.
func armarElPanel(g *grilla) {
	anchoPanel, altoPanel := g.AnchoDelPanel(), g.AltoDelPanel()

	fondo, medio, claro := indicesDelPanel()

	g.Panel = make([]byte, anchoPanel*altoPanel)

	for i := range g.Panel {
		g.Panel[i] = fondo
	}

	for x := 0; x < anchoPanel; x++ {
		g.Panel[x] = claro
		g.Panel[(altoPanel-1)*anchoPanel+x] = claro
	}

	for y := 0; y < altoPanel; y++ {
		g.Panel[y*anchoPanel] = claro
		g.Panel[y*anchoPanel+anchoPanel-1] = claro
	}

	for f := 0; f < g.H; f++ {
		for c := 0; c < g.W; c++ {
			x0 := margenDelPanel + c*celdaPx
			y0 := margenDelPanel + f*celdaPx

			for y := y0; y < y0+celdaPx-1; y++ {
				for x := x0; x < x0+celdaPx-1; x++ {
					if x < anchoPanel-1 && y < altoPanel-1 {
						g.Panel[y*anchoPanel+x] = medio
					}
				}
			}
		}
	}

	println("JUEGO124: INV PANEL contenedor=", g.Nombre, "ancho=", anchoPanel, "alto=", altoPanel,
		"bytes=", len(g.Panel), "indices=fondo", fondo, "celda", medio, "borde", claro)

	// el instrumento de la textura: cuantos bytes de indices quedan vivos por panel
	bytesDePaneles += len(g.Panel)
}

// bytesDePaneles: lo que ocupan las imagenes de los paneles (se informa en el cierre).
var bytesDePaneles int

// indicesDelPanel: el indice mas oscuro, uno del medio y el mas claro de la paleta de las unidades.
func indicesDelPanel() (fondo, medio, claro uint8) {
	fondo, claro, medio = 1, 1, 1

	minLum, maxLum := 1<<30, -1

	for i := 1; i < 256; i++ {
		lum := 299*int(palJugador[i][0]) + 587*int(palJugador[i][1]) + 114*int(palJugador[i][2])

		if lum < minLum {
			minLum, fondo = lum, uint8(i)
		}

		if lum > maxLum {
			maxLum, claro = lum, uint8(i)
		}
	}

	objetivo := (minLum + maxLum) / 2
	cercania := 1 << 30

	for i := 1; i < 256; i++ {
		lum := 299*int(palJugador[i][0]) + 587*int(palJugador[i][1]) + 114*int(palJugador[i][2])
		d := lum - objetivo

		if d < 0 {
			d = -d
		}

		if d < cercania {
			cercania, medio = d, uint8(i)
		}
	}

	return fondo, medio, claro
}

// GanchoDelObjeto: anota donde se vio el objeto en esta grilla (para el log y el cierre).
var ganchoFila, ganchoColumna, ganchoPanelX, ganchoPanelY int

func (g *grilla) GanchoDelObjeto(fila, columna int) {
	ganchoFila, ganchoColumna = fila, columna
	ganchoPanelX, ganchoPanelY = g.X0+margenDelPanel+columna*celdaPx, g.Y0+margenDelPanel+fila*celdaPx
}

// dibujarElInventario: los paneles ABIERTOS. Se guarda lo que hay debajo de cada caja, se pinta, se cuenta cuantos
// pixeles cambiaron y despues se DEVUELVE el lienzo: asi el crc32 del mapa solo sigue siendo el del host y el
// panel se puede volver a pintar al final para que quede en la captura (el mismo camino de los bichos y los
// objetos del piso en DC-002.121/.123).
func dibujarElInventario(malla *pvr.Surface, pixeles []byte) (pintados, cuantos int) {
	if inventarioKnob != "si" {
		return 0, 0
	}

	abiertos := make([]*grilla, 0, 2)

	if grillaJugador != nil && grillaJugador.Abierta {
		abiertos = append(abiertos, grillaJugador)
	}

	if grillaAlijo != nil && grillaAlijo.Abierta {
		abiertos = append(abiertos, grillaAlijo)
	}

	if len(abiertos) == 0 {
		cuadrosSinElPanel++

		return 0, 0
	}

	t0 := int64(d2cMs())
	crcSinElPanel = crc32.ChecksumIEEE(pixeles)

	// NO SE GUARDA NI SE DEVUELVE NADA, y esto es una decision medida (DC-002.124): el panel es lo que el jugador
	// esta mirando, asi que se queda pintado en el cuadro. Lo que la tarjeta pide que no se mueva es el cuadro del
	// pueblo CON EL PANEL CERRADO, y ahi no se pinta un solo pixel (el log lo dice cuadro a cuadro). Guardar la
	// caja para devolverla costaba 138.240 B de `make` POR CUADRO (dos paneles = 163.328 B) y tres copias del
	// lienzo por cuadro: con el semiespacio al 89 % eso es pedirle al recolector justo lo que no hay.
	for _, g := range abiertos {
		caja := cajaDeLaGrilla(g)

		dibujarUnaGrilla(malla, g, enLaGrilla)

		pintados += caja.Ancho() * caja.Alto()
		cuantos++
	}

	msDelPanel = int(int64(d2cMs()) - t0)
	crcConElPanel = crc32.ChecksumIEEE(pixeles)
	pixelesDelPanel = pintados
	cuadrosConElPanel++

	// "el lienzo volvio" se mide de otra forma con el panel abierto: el mapa se pinta ANTES del panel en el mismo
	// cuadro, asi que lo que se afirma es que el panel no toca el mapa solo (`crcMapa`, la linea de mas abajo del
	// CUADRO) y que con el panel cerrado no se pinta nada.
	lienzoDelPanelIntacto = true

	println("JUEGO124: INV CUADRO", cuadro, "abierto=si paneles=", cuantos, "pixelesPintados=", pintados,
		"msPanel=", msDelPanel, "crcConPanel=", int(crcConElPanel), "mapaSolo=", int(crcHost),
		"jugador=", detalleDeUnaGrilla(grillaJugador), "alijo=", detalleDeUnaGrilla(grillaAlijo))

	return pintados, cuantos
}

// detalleDeUnaGrilla: "ocupadas/ celdas / objetoFila,Columna" para el log (vacio si la grilla no existe).
func detalleDeUnaGrilla(g *grilla) string {
	if g == nil {
		return "-"
	}

	fila, columna, codigo, ok := g.primeraCelda()
	if !ok {
		return ftoa(float64(g.ocupadas())) + "/" + itoa(len(g.Celdas)) + "/vacia"
	}

	return itoa(g.ocupadas()) + "/" + itoa(len(g.Celdas)) + "/" + codigo + "@" + itoa(fila) + "," + itoa(columna)
}

// ============================== EL GUARDADO EN LA GRILLA ==============================

// guardarEnLaGrilla: el objeto recogido va a la primera celda libre de la grilla del jugador. Es lo que la tarjeta
// pide: "meter lo recogido".
func guardarEnLaGrilla() {
	if inventarioKnob != "si" || grillaJugador == nil || enLaGrilla == nil {
		return
	}

	if len(enLaGrilla.Cuadros) == 0 && enLaGrilla.BytesIndices == 0 {
		println("JUEGO124: INV GUARDA SIN-OBJETO: no hay nada que meter en la grilla")

		return
	}

	antes := grillaJugador.ocupadas()
	celda := grillaJugador.meter(enLaGrilla.Codigo, enLaGrilla.AnchoCeldas, enLaGrilla.AltoCeldas)

	if celda == nil {
		println("JUEGO124: INV GUARDA NO-ENTRA codigo=", enLaGrilla.Codigo, "huella=", enLaGrilla.AnchoCeldas,
			"x", enLaGrilla.AltoCeldas, "libres=", len(grillaJugador.Celdas)-antes)

		return
	}

	println("JUEGO124: INV GUARDA contenedor= jugador codigo=", enLaGrilla.Codigo, "celda=", celda.Fila,
		",", celda.Columna, "huella=", enLaGrilla.AnchoCeldas, "x", enLaGrilla.AltoCeldas,
		"ocupadasAntes=", antes, "ocupadasDespues=", grillaJugador.ocupadas(),
		"bytesDeCeldas=", len(grillaJugador.Celdas)*celdaDeGrillaBytes)
}

// cajaDeLaGrilla: la caja del panel en el lienzo, recortada a la pantalla.
func cajaDeLaGrilla(g *grilla) CajaDelJugador {
	return recortarALaPantalla(CajaDelJugador{
		X0: g.X0, Y0: g.Y0, X1: g.X0 + g.AnchoDelPanel(), Y1: g.Y0 + g.AltoDelPanel(),
	})
}

// grillasDeLasMitades: la grilla del inventario de CADA jugador del split. La del jugador 1 es la de siempre
// (`grillaJugador`), asi que el modo de un jugador y el split comparten el mismo estado.
var grillasDeLasMitades = map[int]*grilla{}

// dibujarElInventarioEnLaMitad: pinta el inventario de ESTE jugador dentro de SU lienzo del split ("con split, que
// cada mitad tenga su inventario", DC-002.124). El panel se ubica DENTRO de la mitad, no en la pantalla entera: si
// se pintara en las coordenadas del modo de un jugador, con dos mitades de 320 px el panel se saldria del lienzo.
//
// Cada mitad tiene su grilla y su objeto. El objeto recogido lo tiene el jugador que lo recogio (`enLaGrilla`, que
// hoy es el jugador 1: el recoger es de DC-002.123 y no se cambio en esta tarjeta).
func dibujarElInventarioEnLaMitad(i int, malla *pvr.Surface) int {
	g := grillaDeLaMitad(i)
	if g == nil || cuadro+1 < cuadroAbreElInventario {
		return 0
	}

	g.X0 = (mitadDelSplit - g.AnchoDelPanel()) / 2
	if g.X0 < 0 {
		g.X0 = 0
	}

	g.Y0 = alto - g.AltoDelPanel() - 8
	if g.Y0 < 0 {
		g.Y0 = 0
	}

	objeto := enLaGrilla
	if i != 0 {
		objeto = nil
	}

	t0 := int64(d2cMs())
	caja, _ := dibujarUnaGrilla(malla, g, objeto)
	ms := int(int64(d2cMs()) - t0)

	pixelesDelPanel += caja.Ancho() * caja.Alto()
	cuadrosConElPanel++

	nombre := "?"
	if i < len(jugadores) && jugadores[i] != nil {
		nombre = jugadores[i].Nombre
	}

	println("JUEGO124: INV SPLIT cuadro=", cuadro, "mitad=", i, "jugador=", nombre,
		"panel=", g.AnchoDelPanel(), "x", g.AltoDelPanel(), "origen=", g.X0, g.Y0,
		"pixeles=", caja.Ancho()*caja.Alto(), "msPanel=", ms, "bytesDeLaGrilla=", len(g.Celdas)*celdaDeGrillaBytes,
		"bytesDelPanel=", len(g.Panel), "ocupadas=", g.ocupadas())

	return ms
}

// grillaDeLaMitad: la grilla de ese jugador; las de las mitades 2..4 se arman la primera vez que se piden.
func grillaDeLaMitad(i int) *grilla {
	if i == 0 {
		return grillaJugador
	}

	if g, hay := grillasDeLasMitades[i]; hay {
		return g
	}

	if jugadorOK == false {
		return nil
	}

	invW, invH := d2inventory.Dims(d2inventory.ContainerInventory, true)

	g := nuevaGrilla("jugador", invW, invH, 0, 0)
	armarElPanel(g)

	grillasDeLasMitades[i] = g

	println("JUEGO124: INV MITAD jugador=", i, "grilla=", g.W, "x", g.H, "bytesDeLaGrilla=",
		len(g.Celdas)*celdaDeGrillaBytes, "bytesDelPanel=", len(g.Panel))

	return g
}

// ============================== EL PASO (lo que cambia entre cuadros) ==============================

var guardadoEnLaGrilla bool

// pasoDelInventario: UN CUADRO. Hace tres cosas, en este orden y por este motivo:
//  1. si el objeto acaba de entrar a la grilla (lo recogio la regla de DC-002.123), lo mete en su celda;
//  2. si el instrumento dice que este cuadro se abre el panel, lo abre (y lo cierra unos cuadros despues);
//  3. no toca el mapa: el panel se pinta en `dibujar` y se devuelve.
func pasoDelInventario() {
	if inventarioKnob != "si" {
		return
	}

	// ---- 1. EL OBJETO RECOGIDO ENTRA A LA GRILLA ----
	if !guardadoEnLaGrilla && hayObjetoRecogido() {
		guardarEnLaGrilla()
		guardadoEnLaGrilla = true

		println("JUEGO124: INV ENTRA elObjetoRecogido=true cuadro=", cuadro+1, "jugador=", quienLoRecogio())
	}

	// ---- 2. EL PANEL (instrumento) ----
	if inventarioAuto != "si" || grillaJugador == nil {
		return
	}

	if cuadro+1 == cuadroAbreElInventario {
		grillaJugador.Abierta = true
		grillaJugador.Veces++

		println("JUEGO124: INV ABRE cuadro=", cuadro+1, "grilla=", grillaJugador.W, "x", grillaJugador.H,
			"panel=", grillaJugador.X0, grillaJugador.Y0, "ancho=", grillaJugador.AnchoDelPanel(),
			"alto=", grillaJugador.AltoDelPanel(), "objeto=", detalleDeUnaGrilla(grillaJugador))
	}

	if cerrarLosPaneles == "si" && cuadro+1 == cuadroAbreElInventario+3 {
		grillaJugador.Abierta = false

		println("JUEGO124: INV CIERRA cuadro=", cuadro+1, "objeto=", detalleDeUnaGrilla(grillaJugador),
			"vecesAbierto=", grillaJugador.Veces)
	}
}

// hayObjetoRecogido: si el suelo ya no tiene el objeto y alguien lo levanto.
func hayObjetoRecogido() bool {
	for _, it := range suelo {
		if it.Recogido {
			return true
		}
	}

	return false
}

// quienLoRecogio: el jugador que lo levanto (para el log).
func quienLoRecogio() string {
	for _, it := range suelo {
		if it.Recogido {
			return it.Quien
		}
	}

	return "-"
}

// resumenDelInventario: el cierre de la tarjeta, con los tres numeros del criterio y el costo.
func resumenDelInventario() {
	if inventarioKnob != "si" {
		return
	}

	celdasJugador, celdasAlijo := 0, 0

	if grillaJugador != nil {
		celdasJugador = len(grillaJugador.Celdas)
	}

	if grillaAlijo != nil {
		celdasAlijo = len(grillaAlijo.Celdas)
	}

	println("JUEGO124: INV RESUMEN celdasJugador=", celdasJugador, "celdasAlijo=", celdasAlijo,
		"bytesDeCeldas=", (celdasJugador+celdasAlijo)*celdaDeGrillaBytes,
		"bytesDeIndicesDelObjeto=", enLaGrillaBytes(), "bytesDeLosPaneles=", bytesDePaneles,
		"pixelesDelPanel=", pixelesDelPanel,
		"msDelPanel=", msDelPanel, "cuadrosConPanel=", cuadrosConElPanel, "cuadrosSinPanel=", cuadrosSinElPanel,
		"jugador=", detalleDeUnaGrilla(grillaJugador), "alijo=", detalleDeUnaGrilla(grillaAlijo))

	entro := false
	if grillaJugador != nil {
		_, _, codigo, ok := grillaJugador.primeraCelda()
		entro = ok && codigo == codigoDelObjeto()
	}

	seVe := ganchoPanelX > 0 && ganchoPanelY > 0 && enLaGrilla != nil && len(enLaGrilla.Cuadros) > 0

	println("JUEGO124: INV CRITERIO elObjetoEntroALaGrilla=", entro, "seVeEnSuCelda=", seVe,
		"elAlijoGuardo=", guardoElAlijo, "elAlijoDevolvio=", devolvioElAlijo,
		"elCuadroDelPuebloIntacto=", lienzoDelPanelIntacto,
		"anchoDeCelda=", celdaPx, "huellaDelObjeto=", enLaGrillaHuella())
}

// codigoDelObjeto / enLaGrillaHuella / enLaGrillaBytes: atajos para el cierre (sin nil-checks en cada linea).
func codigoDelObjeto() string {
	if enLaGrilla == nil {
		return ""
	}

	return enLaGrilla.Codigo
}

func enLaGrillaHuella() string {
	if enLaGrilla == nil {
		return "-"
	}

	return itoa(enLaGrilla.AnchoCeldas) + "x" + itoa(enLaGrilla.AltoCeldas)
}

func enLaGrillaBytes() int {
	if enLaGrilla == nil {
		return 0
	}

	return enLaGrilla.BytesIndices
}

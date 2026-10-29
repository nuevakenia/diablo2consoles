//go:build dreamcast

package main

// LOS ITEMS DEL SUELO Y EL INVENTARIO (DC-002.61).
//
// QUE HACE: toma los registros de OBJETOS del .ds1 del nivel (el mismo `d2ds1` que ya usa el mapa), los pone
// como items en el suelo con su grafico del MPQ, deja recogerlos con el boton de accion cuando el personaje
// esta en la celda de al lado y los guarda en un inventario de 4 casillas que se ve con el panel.
//
// DE DONDE SALE CADA COSA (nada de esto es intuicion; medido el 2026-10-29 con una sonda del host):
//
//   - LOS 41 REGISTROS: `d2ds1.Object` ya los trae (`Type, ID, X, Y, Flags, Paths`). El `Type` NO es
//     decorativo: 2 = un objeto de `objects.txt` (Casket, Urn, Barrel, Shrine, Door...), 1 = un punto de
//     aparicion de MONSTRUO/NPC (en el pueblo son los NPC). Ver `d2common/d2drlg/drlgoutdoor/subobjects_test.go`
//     ("a monster: never" para los Type 1) y `preset_level.go`.
//   - LA CELDA: `X`/`Y` del DS1 vienen en SUBTILES (5 por celda, `subtilesPorCelda` de personaje.go): el
//     motor las pasa a celdas con `/5` (`d2drlg/drlgoutdoor/preset_level.go:164`). Aca se guardan las dos.
//   - EL GRAFICO: los objetos NO se dibujan con un DC6 suelto: son COF + capas, igual que los monstruos. La
//     capa que se ve es la "lit" (iluminada) de la capa TR (transparente), y el camino lo arma el mismo
//     esquema del motor (`composite.go`, layerPaths):
//
//         data\global\objects\<TOKEN>\<CAPA>\<token><capa>lit<modo><arma>.dcc
//
//     medido: `data\global\objects\C5\TR\c5trlitnuhth.dcc` (3411..39759 B por token, 1 direccion, 2..21
//     cuadros, caja de 49x77 a 115x97). Se dibuja el cuadro 0 de la direccion 0.
//   - LOS NOMBRES: `objects.txt` del d2data.mpq, columnas Name (0), Id (2) y Token (3) -- las tres medidas
//     con la sonda del host, no supuestas.
//   - LO QUE NO TIENE GRAFICO: los Type 1 (su `ID` es un id de MONSTRUO, no de `objects.txt`, asi que no hay
//     token que buscar) y cualquier token sin COF. Esos van con un MARCADOR de una celda (un rombo) y se
//     anotan en el log: la tarjeta lo pide explicito ("usar un marcador de 1 tile y anotarlo en el log").
//
// POR QUE TODO ESTO CORRE CON EL VIDEO YA ANDANDO: leer `objects.txt` y decodificar 12 graficos son ~100 KB
// del monton de Go, y en este POC cualquier cosa que crezca ANTES de `IniciarPantalla` deja al PVR sin sus
// 70 KB y lo cuelga en "pvr: enabling vertical scaling" (la saga de DC-002.50, medida ocho veces). Por eso
// el suelo se cuenta ANTES del video (solo numeros: 41 structs) y los graficos se cargan DESPUES.

import (
	"hash/crc32"
	"image/color"
	"runtime"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2cof"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2dc6"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2dcc"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2ds1"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2mpq"
	"github.com/hajimehoshi/ebiten/v2"
)

// items: ENCIENDE los items del suelo (DC-002.61). Arranca APAGADO a proposito: con esto apagado el binario
// es el de DC-002.50/DC-002.49 (mapa, personaje, bicho y golpe), que es lo que compara el corpus de esas
// tarjetas. `scripts/console/build-items-261.sh` lo enciende con un sed y recompila, igual que `animar` y
// `bestia`, y con ITEMS=0 se compila el BASELINE para medir los pixeles (criterio 2).
var items = false

// itemsSordo: EL MUTANTE del criterio 3. El boton de accion se LEE y se INFORMA (la marca ITEM BOTON sale
// igual), pero NADA se recoge: con las mismas pulsaciones el suelo no baja y el inventario queda vacio.
var itemsSordo = false

// itemsAuto: INSTRUMENTO de medicion, hermano de `golpeAuto` (DC-002.50). El emulador corre sin ventana y sin
// teclado, asi que ni la cruceta ni el boton de accion se aprietan nunca y la cadena
// caminar -> estar al lado -> recoger no se podria medir. Con esto encendido el POC simula la CRUCETA hacia
// el item mas cercano y el FLANCO del boton de accion (90 ms apretado cada 400 ms) hasta recoger 3 items. El
// camino que se mide es el REAL: el mismo `est.Avanzar` del motor, el mismo `recoger` y el mismo inventario.
// No toca ni las posiciones de los items ni la regla del radio.
var itemsAuto = false

// panelAuto: INSTRUMENTO del criterio "cerrar y abrir el panel no pierde ninguno". Abre y cierra el panel
// solo (1,2 s abierto cada 2,5 s) para poder medir en la consola, que no tiene teclado.
var panelAuto = false

// medirPantalla: INSTRUMENTO del criterio 2 (los pixeles). Cuenta EN LA CONSOLA los pixeles que no son
// negros de la pantalla y deja su huella (crc32) en dos cuadros fijos del loop, asi la corrida CON items y
// la del BASELINE (sin items) se comparan con el mismo cuadro. Es la misma cuenta que se le hace a la
// captura, pero sin depender de que la ventana del emulador este adelante.
var medirPantalla = false

const (
	// casillasDelInventario: el inventario del POC. Cuatro, como pide la tarjeta.
	casillasDelInventario = 4

	// radioDeRecogida: a cuantas celdas del mapa se puede recoger. La tarjeta dice "a menos de 1 celda" y se
	// toma tal cual sobre la rejilla: la celda del personaje y sus 8 vecinas (|dx| <= 1 y |dy| <= 1).
	radioDeRecogida = 1

	// el instrumento: cada cuanto aprieta el boton de accion y cuanto lo mantiene
	msBotonAuto       = 400
	duracionBotonAuto = 90

	// itemsAAuto: cuantos items recoge el instrumento antes de quedarse quieto (el criterio 3 pide 3)
	itemsAAuto = 3

	// el panel del instrumento: 1,2 s abierto cada 2,5 s
	msPanelAuto       = 2500
	duracionPanelAuto = 1200

	// excelObjetos: la tabla de los objetos del juego, en el d2data.mpq
	excelObjetos = "data\\global\\excel\\objects.txt"

	// la carpeta de los graficos de los objetos (el esquema de los monstruos, con otro prefijo)
	carpetaObjetos = "data\\global\\objects\\"

	// el modo y el arma con los que se busca la capa: NU (parado) y HTH (sin arma en la mano), los mismos
	// del COF del heroe. Los objetos no tienen arma: su COF declara HTH en la unica capa que traen.
	modoDelObjeto          = "NU"
	armaDelObjeto          = "HTH"
	valorDeLaCapaDelObjeto = "LIT"
)

// filaObjetos: las columnas de objects.txt que usa el POC.
type filaObjetos struct {
	Token  string
	Nombre string
}

// graficoDeItem: el dibujo de UN token, ya decodificado y con los colores de la paleta de unidades.
type graficoDeItem struct {
	Token    string
	Imagen   *ebiten.Image
	Ancho    int
	Alto     int
	Izq      int // la esquina del cuadro respecto del ancla de la celda (Box.Left/Top del DCC)
	Arriba   int
	Pixeles  int    // pixeles que no son el indice 0
	CRC      uint32 // crc32 de los indices del cuadro
	Fuente   string // "DCC <ruta>", "DC6 <ruta>" o "MARCADOR"
	Marcador bool
}

// itemDelSuelo: un registro de objeto del .ds1, con su grafiquito y si ya se recogio.
type itemDelSuelo struct {
	Tipo     int // 2 = objeto de objects.txt; 1 = punto de aparicion de monstruo/NPC
	ID       int
	SubX     int // la posicion CRUDA del DS1, en subtiles
	SubY     int
	Flags    int
	CeldaX   int // SubX/subtilesPorCelda
	CeldaY   int
	Token    string
	Nombre   string
	Grafico  *graficoDeItem
	Caja     rect // la caja del item en la pantalla (el mapa no se mueve: se calcula una vez)
	Recogido bool
}

// inventario: las cuatro casillas del POC.
type inventario struct {
	Nombre   [casillasDelInventario]string
	Ocupadas int
}

// Agregar: mete un item en la primera casilla libre.
func (v *inventario) Agregar(nombre string) {
	if v.Ocupadas >= casillasDelInventario {
		return
	}

	v.Nombre[v.Ocupadas] = nombre
	v.Ocupadas++
}

// ============================== 1. LOS REGISTROS DEL DS1 (antes del video) ==============================

// nuevoSuelo: los registros de objetos del DS1 a items del suelo. NO inventa posiciones: la celda es la del
// DS1 (/5, como el motor) y los subtiles se guardan crudos para dibujar en el lugar exacto.
func nuevoSuelo(objetos []d2ds1.Object) []itemDelSuelo {
	suelo := make([]itemDelSuelo, len(objetos))

	for i := range objetos {
		suelo[i] = itemDelSuelo{
			Tipo:   objetos[i].Type,
			ID:     objetos[i].ID,
			SubX:   objetos[i].X,
			SubY:   objetos[i].Y,
			Flags:  objetos[i].Flags,
			CeldaX: objetos[i].X / subtilesPorCelda,
			CeldaY: objetos[i].Y / subtilesPorCelda,
		}
	}

	return suelo
}

// informarSuelo: el instrumento del criterio 1: CUANTOS items hay y CUAL es la posicion de cada uno, leida
// del DS1 (tipo, id, subtiles del archivo, celda del mapa y flags). Va ANTES del video: solo imprime.
func informarSuelo(suelo []itemDelSuelo) {
	objetos, apariciones := 0, 0

	for i := range suelo {
		if suelo[i].Tipo == 1 {
			apariciones++
		} else {
			objetos++
		}

		println("ITEM suelo idx=", i, "tipo=", suelo[i].Tipo, "id=", suelo[i].ID,
			"sub=", suelo[i].SubX, suelo[i].SubY, "celda=", suelo[i].CeldaX, suelo[i].CeldaY,
			"flags=", suelo[i].Flags)
	}

	println("ITEM suelo=", len(suelo), "objetos=", objetos, "apariciones=", apariciones)
}

// ============================== 2. LA TABLA DE objects.txt ==============================

// campoDeLinea: la columna `n` (contando tabuladores) de una linea del .txt, sin el \r.
func campoDeLinea(linea []byte, n int) []byte {
	inicio := 0
	columna := 0

	for i := 0; i <= len(linea); i++ {
		if i < len(linea) && linea[i] != '\t' {
			continue
		}

		if columna == n {
			fin := i
			if fin > inicio && linea[fin-1] == '\r' {
				fin--
			}

			return linea[inicio:fin]
		}

		columna++
		inicio = i + 1
	}

	return nil
}

// columnasDelObjeto: los indices de Id, Token y Name en la cabecera de objects.txt (no se suponen: se leen).
func columnasDelObjeto(cabecera []byte) (int, int, int) {
	colId, colToken, colNombre := -1, -1, -1

	for i := 0; i < 192; i++ {
		campo := campoDeLinea(cabecera, i)
		if campo == nil {
			break
		}

		switch string(campo) {
		case "Id":
			colId = i
		case "Token":
			colToken = i
		case "Name":
			colNombre = i
		}
	}

	return colId, colToken, colNombre
}

// enteroDeTexto: el id de la fila, sin strconv (la consola tiene su propia stdlib y no hace falta crecerla).
func enteroDeTexto(campo []byte) (int, bool) {
	if len(campo) == 0 {
		return 0, false
	}

	v := 0

	for i := 0; i < len(campo); i++ {
		c := campo[i]
		if c < '0' || c > '9' {
			return 0, false
		}

		v = v*10 + int(c-'0')
	}

	return v, true
}

// leerTablaDeObjetos: objects.txt del MPQ -> id de objeto -> {token, nombre}, SOLO de los ids que el suelo
// pide (26 de las 410 filas del juego: guardar las otras 384 son ~80 KB del monton que no hacen falta). Se
// recorre con un barrido de bytes (el archivo son ~152 KB y partir el texto en lineas deja decenas de
// rebanadas vivas en el monton).
func leerTablaDeObjetos(mpq *d2mpq.MPQ, suelo []itemDelSuelo) map[int]filaObjetos {
	crudo, err := mpq.ReadFile(excelObjetos)
	if err != nil {
		println("ITEM OBJECTS.TXT FALLO:", err.Error())

		return nil
	}

	println("ITEM OBJECTS.TXT bytes=", len(crudo), "crc32=", crc32.ChecksumIEEE(crudo))

	necesarios := make(map[int]bool, len(suelo))

	for i := range suelo {
		if suelo[i].Tipo == 2 {
			necesarios[suelo[i].ID] = true
		}
	}

	tabla := make(map[int]filaObjetos, len(necesarios))

	colId, colToken, colNombre := -1, -1, -1
	linea := 0
	desde := 0

	for i := 0; i <= len(crudo); i++ {
		if i < len(crudo) && crudo[i] != '\n' {
			continue
		}

		fin := i
		if fin > desde && crudo[fin-1] == '\r' {
			fin--
		}

		if fin > desde {
			switch {
			case linea == 0:
				colId, colToken, colNombre = columnasDelObjeto(crudo[desde:fin])
			case colId >= 0 && colToken >= 0:
				id, ok := enteroDeTexto(campoDeLinea(crudo[desde:fin], colId))
				if ok && necesarios[id] {
					nombre := ""
					if colNombre >= 0 {
						nombre = string(campoDeLinea(crudo[desde:fin], colNombre))
					}

					tabla[id] = filaObjetos{
						Token:  string(campoDeLinea(crudo[desde:fin], colToken)),
						Nombre: nombre,
					}
				}
			}
		}

		linea++
		desde = i + 1
	}

	println("ITEM OBJECTS.TXT filas=", len(tabla), "de", len(necesarios), "ids del suelo",
		"colId=", colId, "colToken=", colToken, "colNombre=", colNombre)

	crudo = nil

	runtime.GC()

	return tabla
}

// ============================== 3. LOS GRAFICOS (con el video ya andando) ==============================

// cargarItems: el nombre y el grafico de cada item del suelo. Se llama DESPUES de `IniciarPantalla` y ANTES
// de cerrar el d2data.mpq (los objetos y su tabla viven ahi).
//
// EL GRAFICO SE CARGA SOLO PARA LOS ITEMS QUE PUEDEN CAER EN LA PANTALLA, y esto es una MEDIDA, no una
// comodidad: la ventana del nivel es de 512x512 pixeles (doce celdas de ancho) y el pueblo es de 57x41
// celdas, asi que de los 41 registros del DS1 solo DOS caen en el cuadro. Decodificar los 12 tokens del
// pueblo son ~220 KB de RGBA y el monton de la consola tiene ~140 KB libres a esta altura del arranque
// (medido: el primer intento de esta tarjeta se cayo con `fatal error: gc_external_alloc: OOM` decodificando
// el santuario, que son 21 cuadros de 76x134). Los items de afuera existen igual -- tienen su celda del DS1
// y se pueden recoger caminando hasta ellos --, solo que no se dibujan porque no estan en el cuadro.
func (j *juego) cargarItems(mpq *d2mpq.MPQ) {
	t0 := int64(d2cMs())

	memoria("items-antes")

	tabla := leerTablaDeObjetos(mpq, j.itemsSuelo)

	for i := range j.itemsSuelo {
		it := &j.itemsSuelo[i]

		if it.Tipo == 2 && tabla != nil {
			if fila, ok := tabla[it.ID]; ok {
				it.Token = fila.Token
				it.Nombre = fila.Nombre
			}
		}

		// el nombre que se muestra: el de objects.txt; si no hay (o el juego lo dejo en "not used"), el token
		if it.Nombre == "" || it.Nombre == "not used" {
			if it.Token != "" {
				it.Nombre = it.Token
			} else {
				it.Nombre = "SPAWN" + numeroATexto(it.ID)
			}
		}
	}

	tabla = nil

	runtime.GC()

	pedidos := 0

	for i := range j.itemsSuelo {
		if posiblementeEnPantalla(&j.itemsSuelo[i]) {
			pedidos++
		}
	}

	println("ITEM grafico-pedidos=", pedidos, "de", len(j.itemsSuelo))

	var graficos []*graficoDeItem

	marcador := (*graficoDeItem)(nil)

	for i := range j.itemsSuelo {
		it := &j.itemsSuelo[i]

		if !posiblementeEnPantalla(it) {
			continue
		}

		if it.Token == "" {
			if marcador == nil {
				marcador = nuevoMarcadorDeItem()
			}

			it.Grafico = marcador
		} else {
			g := buscarGrafico(graficos, it.Token)
			if g == nil {
				g = cargarGraficoDeItem(mpq, it.Token, j.palUnidades)
				graficos = append(graficos, g)

				// lo decodificado (crudo + los cuadros del DCC) se suelta YA: queda solo la imagen del cuadro
				// que se dibuja (medido: con las 21 capas vivas no entra en el monton)
				runtime.GC()
			}

			it.Grafico = g
		}

		it.Caja = cajaDelItem(it)
	}

	conGrafico, conMarcador, enPantalla := 0, 0, 0

	for i := range j.itemsSuelo {
		it := &j.itemsSuelo[i]
		if it.Grafico == nil {
			continue
		}

		if it.Grafico.Marcador {
			conMarcador++
		} else {
			conGrafico++
		}

		if seVeEnPantalla(it.Caja) {
			enPantalla++
		}
	}

	println("ITEM cargados=", len(j.itemsSuelo), "tokens=", len(graficos), "conGrafico=", conGrafico,
		"marcador=", conMarcador, "enPantalla=", enPantalla, "sinGrafico=", len(j.itemsSuelo)-conGrafico-conMarcador,
		"ms=", int64(d2cMs())-t0)

	memoria("items")
}

// posiblementeEnPantalla: si el ancla del item puede meter parte de su cuadro en la pantalla. Se usa el
// cuadro mas grande medido (115x97 con esquina -55/-120) con margen, para no decodificar graficos de items
// que quedan a media provincia de distancia.
func posiblementeEnPantalla(it *itemDelSuelo) bool {
	ax, ay := PixelDeLaCelda(float64(it.SubX)/float64(subtilesPorCelda),
		float64(it.SubY)/float64(subtilesPorCelda), origenX, origenY)

	px, py := mapaX+ax, mapaY+ay

	return px+160 > 0 && px-160 < ancho && py+48 > 0 && py-160 < alto
}

// buscarGrafico: el grafico ya cargado de un token (12 tokens como mucho: busqueda lineal).
func buscarGrafico(graficos []*graficoDeItem, token string) *graficoDeItem {
	for _, g := range graficos {
		if g.Token == token {
			return g
		}
	}

	return nil
}

// cargarGraficoDeItem: el grafico de un token: su COF (para saber la capa y el arma), la capa "lit" del modo
// NU en DCC o DC6 (los dos esquemas del motor) y el cuadro 0 de la direccion 0 a indices con la paleta de las
// unidades. Si no hay COF ni capa, devuelve el MARCADOR y lo dice.
func cargarGraficoDeItem(mpq *d2mpq.MPQ, token string, paleta [256][3]uint8) *graficoDeItem {
	mayus := mayusculasDeTexto(token)

	// ---- el COF del objeto
	var cof *d2cof.COF
	nombreCOF := carpetaObjetos + mayus + `\COF\` + mayus + modoDelObjeto + armaDelObjeto + ".COF"

	if crudo, err := mpq.ReadFile(nombreCOF); err == nil {
		c, err := d2cof.Unmarshal(crudo)
		if err == nil {
			cof = c

			println("ITEM COF token=", token, "bytes=", len(crudo), "crc32=", crc32.ChecksumIEEE(crudo),
				"capas=", c.NumberOfLayers, "framesPorDir=", c.FramesPerDirection, "dirs=", c.NumberOfDirections)
		} else {
			println("ITEM COF token=", token, "PARSE-FALLO:", err.Error())
		}
	}

	if cof == nil || len(cof.CofLayers) == 0 {
		println("ITEM SIN-GRAFICO token=", token, "ruta=", nombreCOF, "(va el marcador)")

		return nuevoMarcadorDeItem()
	}

	// ---- la capa: los objetos traen UNA sola (TR) y su clase de arma (HTH)
	capa := capaDelObjeto(cof.CofLayers[0].Type.String())
	arma := cof.CofLayers[0].WeaponClass.String()
	if arma == "" {
		arma = armaDelObjeto
	}

	base := carpetaObjetos + mayus + `\` + capa + `\` + mayus + capa + valorDeLaCapaDelObjeto + modoDelObjeto + mayusculasDeTexto(arma)

	// ---- DCC primero y DC6 despues: es el orden del motor (composite.go) y el que midio la sonda del host
	for _, ext := range []string{"dcc", "dc6"} {
		ruta := base + "." + ext

		crudo, err := mpq.ReadFile(ruta)
		if err != nil {
			continue
		}

		if ext == "dcc" {
			dd, err := d2dcc.Load(crudo)
			if err != nil || len(dd.Directions) == 0 || len(dd.Directions[0].Frames) == 0 {
				println("ITEM CAPA token=", token, "PARSE-FALLO-DCC ruta=", ruta)

				continue
			}

			caja := dd.Directions[0].Box
			pixeles := dd.Directions[0].Frames[0].PixelData

			g := componerGrafico(token, caja.Left, caja.Top, caja.Width, caja.Height, pixeles, paleta,
				"DCC "+ruta)

			println("ITEM GRAFICO token=", token, "fuente=DCC bytes=", len(crudo),
				"crc32=", crc32.ChecksumIEEE(crudo), "dirs=", len(dd.Directions),
				"cuadros=", len(dd.Directions[0].Frames), "caja=", g.Ancho, g.Alto,
				"esquina=", g.Izq, g.Arriba, "pixeles=", g.Pixeles, "crc=", g.CRC)

			dd = nil
			crudo = nil

			return g
		}

		dc, err := d2dc6.Load(crudo)
		if err != nil || len(dc.Frames) == 0 {
			println("ITEM CAPA token=", token, "PARSE-FALLO-DC6 ruta=", ruta)

			continue
		}

		fr := dc.Frames[0]
		pixeles := dc.DecodeFrame(0)

		g := componerGrafico(token, int(fr.OffsetX), int(fr.OffsetY), int(fr.Width), int(fr.Height), pixeles,
			paleta, "DC6 "+ruta)

		println("ITEM GRAFICO token=", token, "fuente=DC6 bytes=", len(crudo),
			"crc32=", crc32.ChecksumIEEE(crudo), "cuadros=", len(dc.Frames), "caja=", g.Ancho, g.Alto,
			"esquina=", g.Izq, g.Arriba, "pixeles=", g.Pixeles, "crc=", g.CRC)

		dc = nil
		crudo = nil

		return g
	}

	println("ITEM SIN-GRAFICO token=", token, "ruta=", base, "(.dcc/.dc6) (va el marcador)")

	return nuevoMarcadorDeItem()
}

// capaDelObjeto: el nombre de la capa en el MPQ (el COF la llama "TR"; el camino usa TR).
func capaDelObjeto(tipo string) string {
	if tipo == "" {
		return "TR"
	}

	return tipo
}

// componerGrafico: los indices de una capa a la imagen del item: RGBA con la paleta de unidades y el indice 0
// TRANSPARENTE (la convencion del juego: por ahi se ve el piso).
func componerGrafico(token string, izq, arriba, ancho, alto int, pixeles []byte, paleta [256][3]uint8,
	fuente string) *graficoDeItem {
	if ancho <= 0 || alto <= 0 || len(pixeles) < ancho*alto {
		g := nuevoMarcadorDeItem()

		return g
	}

	imagen := ebiten.NewImage(ancho, alto)
	rgba := make([]byte, ancho*alto*4)

	noVacios := 0

	for i := 0; i < ancho*alto; i++ {
		idx := pixeles[i]

		if idx == 0 {
			continue
		}

		rgba[i*4+0], rgba[i*4+1], rgba[i*4+2], rgba[i*4+3] = paleta[idx][0], paleta[idx][1], paleta[idx][2], 255

		noVacios++
	}

	imagen.ReplacePixels(rgba)

	rgba = nil

	return &graficoDeItem{
		Token:   token,
		Imagen:  imagen,
		Ancho:   ancho,
		Alto:    alto,
		Izq:     izq,
		Arriba:  arriba,
		Pixeles: noVacios,
		CRC:     crc32.ChecksumIEEE(pixeles[:ancho*alto]),
		Fuente:  fuente,
	}
}

// nuevoMarcadorDeItem: el MARCADOR de una celda para los items sin grafico en el MPQ: un rombo del tamaño de
// un tile del piso (ladoCeldaX x ladoCeldaY = 80x40) con el borde naranja. Es lo que la tarjeta pide cuando
// el recurso no existe: se ve, se cuenta y no rompe el cuadro.
func nuevoMarcadorDeItem() *graficoDeItem {
	ancho, alto := ladoCeldaX, ladoCeldaY

	imagen := ebiten.NewImage(ancho, alto)
	rgba := make([]byte, ancho*alto*4)

	medioX, medioY := ancho/2, alto/2

	for y := 0; y < alto; y++ {
		for x := 0; x < ancho; x++ {
			dx := x - medioX
			dy := y - medioY

			if dx < 0 {
				dx = -dx
			}

			if dy < 0 {
				dy = -dy
			}

			// el rombo: |dx|/medioX + |dy|/medioY <= 1 (borde si queda a menos de 2 pixeles)
			if dx*(alto/2)+dy*(ancho/2) > (ancho/2)*(alto/2) {
				continue
			}

			i := (y*ancho + x) * 4

			if dx*(alto/2)+dy*(ancho/2) > (ancho/2)*(alto/2)-ancho {
				rgba[i+0], rgba[i+1], rgba[i+2], rgba[i+3] = 255, 140, 40, 255
			} else {
				rgba[i+0], rgba[i+1], rgba[i+2], rgba[i+3] = 90, 40, 10, 255
			}
		}
	}

	imagen.ReplacePixels(rgba)

	rgba = nil

	return &graficoDeItem{
		Token:    "",
		Imagen:   imagen,
		Ancho:    ancho,
		Alto:     alto,
		Izq:      -ancho / 2,
		Arriba:   -alto,
		Pixeles:  ancho * alto / 2,
		Fuente:   "MARCADOR",
		Marcador: true,
	}
}

// cajaDelItem: la caja del item en la pantalla. La celda se proyecta con la MISMA cuenta del piso
// (`PixelDeLaCelda`) y se le suma la esquina del cuadro del COF, igual que el personaje y el bicho. Se usa
// la posicion en SUBTILES (/5) y no la celda entera porque los objetos del DS1 estan puestos con esa
// precision.
func cajaDelItem(it *itemDelSuelo) rect {
	if it.Grafico == nil {
		return rect{}
	}

	ax, ay := PixelDeLaCelda(float64(it.SubX)/float64(subtilesPorCelda),
		float64(it.SubY)/float64(subtilesPorCelda), origenX, origenY)

	px, py := mapaX+ax+it.Grafico.Izq, mapaY+ay+it.Grafico.Arriba

	return rect{px, py, px + it.Grafico.Ancho, py + it.Grafico.Alto}
}

// seVeEnPantalla: si la caja toca la pantalla (el pueblo es de 57x41 celdas y la ventana del nivel es de
// 512x512: la mayoria de los 41 objetos quedan FUERA de la vista, y eso se informa).
func seVeEnPantalla(r rect) bool {
	if r.x1 <= r.x0 || r.y1 <= r.y0 {
		return false
	}

	return r.x1 > 0 && r.x0 < ancho && r.y1 > 0 && r.y0 < alto
}

// seCruzan: si dos cajas se tocan (para repintar solo los items que el personaje tapa).
func seCruzan(a, b rect) bool {
	return a.x0 < b.x1 && b.x0 < a.x1 && a.y0 < b.y1 && b.y0 < a.y1
}

// ============================== 4. EL DIBUJO ==============================

// dibujarItems: todos los items que siguen en el suelo y que caen en la pantalla.
func (j *juego) dibujarItems(pantalla *ebiten.Image) {
	for i := range j.itemsSuelo {
		j.dibujarItem(pantalla, &j.itemsSuelo[i])
	}
}

// dibujarItemsEnCaja: los items que caen dentro de una caja que se acaba de devolver al mapa. Sin esto, el
// personaje al pasar por encima de un item lo BORRA (la caja del personaje se repinta con los pixeles del
// mapa, y el item estaba pintado encima).
func (j *juego) dibujarItemsEnCaja(pantalla *ebiten.Image, r rect) {
	for i := range j.itemsSuelo {
		it := &j.itemsSuelo[i]
		if it.Recogido || it.Grafico == nil {
			continue
		}

		if !seCruzan(it.Caja, r) {
			continue
		}

		j.dibujarItem(pantalla, it)
	}
}

// dibujarItem: un item en su caja (los items ya no ocupan celda: son una imagen con transparencias).
func (j *juego) dibujarItem(pantalla *ebiten.Image, it *itemDelSuelo) {
	if it.Recogido || it.Grafico == nil || it.Grafico.Imagen == nil {
		return
	}

	if !seVeEnPantalla(it.Caja) {
		return
	}

	var op ebiten.DrawImageOptions
	op.GeoM.Translate(float64(it.Caja.x0), float64(it.Caja.y0))
	pantalla.DrawImage(it.Grafico.Imagen, &op)
}

// restaurarCajaConItems: devuelve una caja al mapa y vuelve a pintar los items que caian dentro.
func (j *juego) restaurarCajaConItems(pantalla *ebiten.Image, r rect) {
	j.devolverAlMapa(pantalla, r)

	if items {
		j.dibujarItemsEnCaja(pantalla, r)
	}
}

// dibujarPanel: el panel del inventario (4 casillas con el nombre del item y el contador del suelo). Se pinta
// ENCIMA de todo, asi que lo llama el final del dibujo.
func (j *juego) dibujarPanel(pantalla *ebiten.Image) {
	anchoPanel, altoPanel := 330, 96

	px, py := mapaX+8, mapaY+16
	if py < 0 {
		py = 0
	}

	if py+altoPanel > alto {
		py = alto - altoPanel
	}

	for y := py; y < py+altoPanel; y++ {
		for x := px; x < px+anchoPanel; x++ {
			if x == px || x == px+anchoPanel-1 || y == py || y == py+altoPanel-1 {
				pantalla.SetRGBA(x, y, color.RGBA{R: 210, G: 190, B: 120, A: 255})

				continue
			}

			pantalla.SetRGBA(x, y, color.RGBA{R: 16, G: 16, B: 24, A: 255})
		}
	}

	titulo := "INVENTARIO " + numeroATexto(j.inv.Ocupadas) + "/" + numeroATexto(casillasDelInventario) +
		" SUELO " + numeroATexto(j.enElSuelo())
	dibujarTexto(pantalla, px+10, py+8, 2, titulo, 240, 220, 150)

	for i := 0; i < casillasDelInventario; i++ {
		cx := px + 10 + i*78
		cy := py + 30

		for y := cy; y < cy+24; y++ {
			for x := cx; x < cx+24; x++ {
				if x == cx || x == cx+23 || y == cy || y == cy+23 {
					pantalla.SetRGBA(x, y, color.RGBA{R: 170, G: 150, B: 100, A: 255})

					continue
				}

				pantalla.SetRGBA(x, y, color.RGBA{R: 34, G: 34, B: 48, A: 255})
			}
		}

		nombre := j.inv.Nombre[i]
		if nombre == "" {
			nombre = "-"
		}

		dibujarTexto(pantalla, cx+28, cy+6, 1, recortarTexto(nombre, 9), 235, 235, 235)
	}
}

// ============================== 5. EL CONTROL: RECOGER Y EL PANEL ==============================

// items: el boton de accion, el panel y el inventario. Se llama desde Update una vez por cuadro.
//
// EL BOTON DE ACCION ES EL A DEL CONTROL: en el backend de la consola (control.go) el A es el clic IZQUIERDO
// del mouse, que es con lo que el juego de PC agarra las cosas del suelo.
//
// EL PANEL ES LA TECLA I, y como la consola no tiene teclado tambien el boton X (que el backend entrega como
// KeyX): la tecla de la tarjeta existe y ademas se puede abrir con el control.
func (j *juego) items(ahora int64) {
	apretado := ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft)

	if itemsAuto && j.itemsPillados < itemsAAuto && j.hayItemCerca() {
		apretado = apretado || (ahora%msBotonAuto) < duracionBotonAuto
	}

	panel := ebiten.IsKeyPressed(ebiten.KeyI) || ebiten.IsKeyPressed(ebiten.KeyX)

	if panelAuto {
		panel = panel || (ahora%msPanelAuto) < duracionPanelAuto
	}

	if panel && !j.botonPanelAntes {
		j.panelAbierto = !j.panelAbierto

		println("ITEM PANEL abierto=", j.panelAbierto, "inventario=", j.inv.Ocupadas, "/", casillasDelInventario,
			"suelo=", j.enElSuelo(), "cuadroDelLoop=", j.cuadros)
	}

	j.botonPanelAntes = panel

	if apretado && !j.botonAccionAntes {
		j.recoger()
	}

	j.botonAccionAntes = apretado
}

// recoger: el item del suelo que esta al lado del personaje pasa al inventario.
func (j *juego) recoger() {
	cx, cy := j.est.CeldaEntera()
	cercano := j.itemMasCercano(cx, cy)

	println("ITEM BOTON boton=A celda=", cx, cy, "cerca=", cercano, "suelo=", j.enElSuelo(),
		"inventario=", j.inv.Ocupadas, "/", casillasDelInventario, "itemsSordo=", itemsSordo,
		"cuadroDelLoop=", j.cuadros)

	if itemsSordo || cercano < 0 {
		return
	}

	if j.inv.Ocupadas >= casillasDelInventario {
		println("ITEM INVENTARIO-LLENO inventario=", j.inv.Ocupadas, "/", casillasDelInventario,
			"suelo=", j.enElSuelo(), "cuadroDelLoop=", j.cuadros)

		return
	}

	it := &j.itemsSuelo[cercano]
	it.Recogido = true
	j.inv.Agregar(it.Nombre)
	j.itemsPillados++

	// DC-002.62: RECOGER SUENA. Aca, cuando el item ya paso al inventario, y no antes: si el inventario
	// esta lleno o no hay nada cerca (los dos caminos que salen mas arriba con `return`) no suena nada.
	sonar(idSonidoRecoger)

	// el mapa se repinta el proximo cuadro: el item ya no esta en el suelo
	j.pintado = false

	println("ITEM recogido=" + it.Nombre + " celda=" + numeroATexto(it.CeldaX) + "," + numeroATexto(it.CeldaY) +
		" suelo=" + numeroATexto(j.enElSuelo()) + " inventario=" + numeroATexto(j.inv.Ocupadas) + "/" +
		numeroATexto(casillasDelInventario) + " radio=" + numeroATexto(radioDeRecogida) +
		" cuadroDelLoop=" + numeroATexto(j.cuadros))
}

// itemMasCercano: el indice del item sin recoger mas cercano que este DENTRO del radio de recogida, o -1.
// La distancia es la de la rejilla (|dx| <= radio y |dy| <= radio): "el tile donde esta parado o el de al
// lado", que es lo que significa "a menos de 1 celda" en la proyeccion isometrica.
func (j *juego) itemMasCercano(cx, cy int) int {
	cercano, mejor := -1, 0

	for i := range j.itemsSuelo {
		it := &j.itemsSuelo[i]
		if it.Recogido {
			continue
		}

		dx, dy := it.CeldaX-cx, it.CeldaY-cy

		if dx < 0 {
			dx = -dx
		}

		if dy < 0 {
			dy = -dy
		}

		if dx > radioDeRecogida || dy > radioDeRecogida {
			continue
		}

		d := dx*dx + dy*dy
		if cercano < 0 || d < mejor {
			cercano, mejor = i, d
		}
	}

	return cercano
}

// hayItemCerca: si hay algo que recoger al lado del personaje (lo pregunta el instrumento antes de apretar).
func (j *juego) hayItemCerca() bool {
	cx, cy := j.est.CeldaEntera()

	return j.itemMasCercano(cx, cy) >= 0
}

// enElSuelo: cuantos items siguen sin recoger.
func (j *juego) enElSuelo() int {
	n := 0

	for i := range j.itemsSuelo {
		if !j.itemsSuelo[i].Recogido {
			n++
		}
	}

	return n
}

// ejeDelAuto: LA CRUCETA DEL INSTRUMENTO (itemsAuto). Devuelve el estado de la cruceta que apunta al item sin
// recoger mas cercano, usando la MISMA proyeccion del piso que el control real (`DireccionDelPiso`): se
// prueban los 8 estados y se elige el que mas apunta al item. Cuando ya esta en el radio no camina: se queda
// quieto y el que aprieta es el boton de accion (ver `items`).
func (j *juego) ejeDelAuto() (float64, float64) {
	if j.itemsPillados >= itemsAAuto || j.hayItemCerca() {
		return 0, 0
	}

	cx, cy := j.est.Celda()

	cercano := -1
	mejor := 0.0

	for i := range j.itemsSuelo {
		it := &j.itemsSuelo[i]
		if it.Recogido {
			continue
		}

		dx := float64(it.CeldaX) - cx
		dy := float64(it.CeldaY) - cy
		d := dx*dx + dy*dy

		if cercano < 0 || d < mejor {
			cercano, mejor = i, d
		}
	}

	if cercano < 0 {
		return 0, 0
	}

	ddx := float64(j.itemsSuelo[cercano].CeldaX) - cx
	ddy := float64(j.itemsSuelo[cercano].CeldaY) - cy

	ejeX, ejeY, mejorPunto := 0.0, 0.0, -2.0

	for ex := -1.0; ex <= 1; ex++ {
		for ey := -1.0; ey <= 1; ey++ {
			if ex == 0 && ey == 0 {
				continue
			}

			dx, dy := DireccionDelPiso(ex, ey)
			punto := dx*ddx + dy*ddy

			if punto > mejorPunto {
				ejeX, ejeY, mejorPunto = ex, ey, punto
			}
		}
	}

	return ejeX, ejeY
}

// medirLaPantalla: EL INSTRUMENTO del criterio 2. Cuenta, EN LA CONSOLA, cuantos pixeles de la pantalla NO
// son negros (la misma cuenta que se le hace a la captura con ffmpeg) y deja la huella (crc32) de la
// pantalla. Se mide en dos cuadros FIJOS del loop (60 y 240) para que la corrida CON items y la del BASELINE
// (sin items) se comparen con el mismo cuadro: la diferencia son los pixeles de los items del suelo.
func (j *juego) medirLaPantalla(pantalla *ebiten.Image) {
	if j.cuadros != 60 && j.cuadros != 240 {
		return
	}

	noVacios := 0

	for y := 0; y < alto; y++ {
		for x := 0; x < ancho; x++ {
			c := pantalla.ColorAt(x, y)

			if c.R > 0.02 || c.G > 0.02 || c.B > 0.02 {
				noVacios++
			}
		}
	}

	println("ITEM PANTALLA cuadro=", j.cuadros, "pixelesNoVacios=", noVacios, "de", ancho*alto,
		"huella=", ebiten.HuellaDePantalla(), "suelo=", j.enElSuelo(), "inventario=", j.inv.Ocupadas)
}

// ============================== 6. AYUDAS ==============================
// numeroATexto: un entero a texto sin strconv (la consola no lo tiene importado y no hace falta crecerla).
func numeroATexto(n int) string {
	if n == 0 {
		return "0"
	}

	negativo := n < 0
	if negativo {
		n = -n
	}

	var digitos [12]byte

	i := len(digitos)

	for n > 0 {
		i--
		digitos[i] = byte('0' + n%10)
		n /= 10
	}

	if negativo {
		i--
		digitos[i] = '-'
	}

	return string(digitos[i:])
}

// mayusculasDeTexto: el texto en mayusculas (los nombres de los recursos del MPQ van asi).
func mayusculasDeTexto(s string) string {
	b := make([]byte, len(s))

	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'a' && c <= 'z' {
			c -= 32
		}

		b[i] = c
	}

	return string(b)
}

// recortarTexto: los primeros `n` caracteres (el panel tiene 9 de ancho por casilla a escala 1).
func recortarTexto(s string, n int) string {
	if len(s) <= n {
		return s
	}

	return s[:n]
}

//go:build dreamcast

package main

// DC-002.130 -- EL MINIMAPA DEL PUEBLO, CON EL MODELO DE `d2automap` PORTADO.
//
// QUE ES. El mapa chiquito del juego, en la caja del motor arriba a la derecha, dibujado con el MODELO del motor
// (`d2common/d2automap`: celdas, capas, proyeccion, layout) y el arte de `maximap.dc6`. El POC no escribe un
// minimapa propio: le da al modelo de `d2automap` las casillas del nivel que el MOTOR ya compuso
// (`motorDelMapa`, el mismo `MapEngine` que dibuja el mapa) y pinta las celdas que el modelo devuelve.
//
// LAS CUATRO COSAS QUE HACEN FALTA, Y DE DONDE SALEN (nada elegido a ojo):
//
//  1. LA TABLA. `AutoMap.txt` del `d2data.mpq` (122.022 B, 2.603 filas) parseada con el lector del motor
//     (`d2automap.ParseTxtLevels`). SE FILTRA AL NIVEL DEL PUEBLO, y no por gusto: la tabla entera son **271.144 B
//     del semiespacio del recolector** (medido con `runtime.ReadMemStats` en `console/dc/sonda-automap-130`) y el
//     semiespacio queda al 96 % con el HUD puesto. Las filas del pueblo son 83 y **6.176 B**. La sonda verifico que
//     la filtrada da las MISMAS 83 filas y EL MISMO modelo (680 celdas, crc32 1625705642) que la entera: el ahorro
//     no cambia el mapa. `ParseTxtLevels` es el MISMO parser (una funcion de mas en el paquete), no una copia.
//
//  2. LAS CASILLAS. El adaptador es el del motor (`d2game/d2player/automap.go`, `tileSource`): piso y pared de
//     cada casilla compuesta, con `Hidden()` y `Prop1 != 0` afuera. Aca se le pasa `motorDelMapa`, o sea el nivel
//     que el POC tiene compuesto: el mapa del minimapa es el MISMO pueblo que se esta dibujando, no un `.ds1`
//     leido aparte.
//
//  3. EL ARTE. **OJO CON EL NOMBRE, MEDIDO**: el motor pide `maximaps.dc6` (cuadros de 8x16) para el minimapa y ese
//     archivo NO ESTA en ningun MPQ de esta instalacion. El que existe es `maximap.dc6` (el del mapa grande),
//     234.458 B con 1.260 cuadros de 16x32. O sea: el arte del minimapa se saca de esos mismos cuadros DECIMADOS a
//     la mitad (1 de cada 2 pixeles), que es exactamente la caja de 8x16 que pide el motor. Los cuadros que pide la
//     tabla del pueblo son 59 y decimados son **7.552 B**: van al MONTON DE C (`bloqueDeC`, la misma puerta del
//     HUD), no al semiespacio.
//
//  4. LA PROYECCION Y LA CAJA. `d2automap.ComputeLayoutOffsets(SizeMini, 640, 480, ...)` con los offsets del motor
//     (`MiniBoxOffsets`): la caja queda en **(359,57)-(638,282)**, 279x225 px. Con eso caen adentro **610 de las
//     680 celdas** del pueblo (la 125 conto 626 en un lienzo de 320x240 con offsets 0,0: misma cuenta, otra caja;
//     la sonda imprime las dos). Dibujar el arte de 16x32 en la proyeccion del minimapa (escala 20) solaparia
//     **59.408 pixeles** (medido en la sonda): por eso el arte va decimado y no estirado.
//
// EL REVEAL SIGUE AL JUGADOR: cada cuadro se le pregunta al modelo si el heroe se movio lo suficiente
// (`d2automap.MovedFar`, el disparador del motor) y, si si, se revelan las casillas de alrededor
// (`RevealTiles` con `ScreenArea(640,480)`, el area que el motor revela: lo que se dibujo). El log dice cuantas
// celdas agrego cada revelado y cuantas hay. Con `MINIMAPA_TODO=si` ademas se revela el pueblo ENTERO en el
// cuadro 1: es el caso peor (las 610 celdas que entran en la caja) y el numero que pide la tarjeta.
//
// LO QUE SE MIDE (dentro de la misma corrida, como el HUD de DC-002.125): el crc32 del lienzo justo antes y justo
// despues de dibujar (para poder decir que el minimapa pinta ALGO sin comparar dos corridas), los pixeles pegados,
// los ms de cada cuadro, el maximo, el promedio y el % del cuadro. Y con eso se decide `siempre` o `por tecla`:
// la decision sale del numero, no de una opinion.

import (
	"encoding/binary"
	"image/color"
	"io"

	"github.com/OpenDiablo2/OpenDiablo2/console/dc/fuenteconsola"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2automap"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2enum"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2dc6"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2render/pvr"
)

const (
	// las rutas del motor (`d2game/d2player/automap.go`): la tabla y el arte.
	rutaTablaAutomap   = `data\global\excel\AutoMap.txt`
	rutaArteAutomap    = `data\global\ui\automap\maximap.dc6`
	rutaNombre8x16     = `data\global\ui\automap\maximaps.dc6`
	paletaDelActo1     = `data\global\palette\act1\pal.dat`
	anchoCeldaMini     = 8 // la caja que pide el motor para el minimapa (16x32 decimado)
	altoCeldaMini      = 16
	cuadroDeArteEntero = 16 // el arte que EXISTE: 16x32
	altoDeArteEntero   = 32

	// el marcador del heroe: la polilinea del motor (`automapCross`, 12 puntos, cada coordenada DUPLICADA al
	// dibujar) y su color (`automapColorSelf`).
	marcadorPuntos = 12
)

// cruzDelMarcador: la tabla del marcador del motor (Game.exe 0x6d7640), tal cual.
var cruzDelMarcador = [marcadorPuntos][2]int{
	{0, -1}, {2, -2}, {4, -1}, {2, 0}, {4, 1}, {2, 2}, {0, 1}, {-2, 2}, {-4, 1}, {-2, 0}, {-4, -1}, {-2, -2},
}

// colorDelMarcador: `automapColorSelf` (0,0,255 del motor; el fork lo trae como 36,96,216).
var colorDelMarcador = color.RGBA{36, 96, 216, 255}

// fondoDelMinimapa: el negro sobre el que se leen las lineas del automapa (el juego dibuja el mapa asi).
var fondoDelMinimapa = color.RGBA{0, 0, 0, 255}

// celdaDelMinimapa: un cuadro de arte ya decodificado y DECIMADO a la caja del minimapa (8x16).
type celdaDelMinimapa struct {
	Cel     int
	Ancho   int
	Alto    int
	Indices []byte // en el monton de C: no lo toca el recolector
	Pixeles int    // los indices != 0
}

// fuenteDelMotor: el adaptador del motor (`d2game/d2player/automap.go`, `tileSource`) sobre el MapEngine que el
// POC tiene compuesto (`motorDelMapa`). Piso y pared de cada casilla, con las mismas exclusiones.
//
// OJO: la casilla NO guarda el motor en un campo, lo toma de `motorDelMapa` en cada llamada. No es capricho: gccgo
// pierde los metodos de `d2asset` al compilar `main.go` si otro archivo del paquete nombra el tipo del motor
// (`*d2mapengine.MapEngine`) y se compila antes que `main.go` (medido: el mismo codigo SIN este import compila y
// CON el no). Como el motor ya es un global del POC, se usa el global y el archivo no importa `d2mapengine`.
type fuenteDelMotor struct{}

func (f fuenteDelMotor) Size() (w, h int) {
	if motorDelMapa == nil {
		return 0, 0
	}

	sz := motorDelMapa.Size()

	return sz.Width, sz.Height
}

func (f fuenteDelMotor) Tiles(tx, ty int) []d2automap.TileRef {
	if motorDelMapa == nil {
		return nil
	}

	t := motorDelMapa.TileAt(tx, ty)
	if t == nil {
		return nil
	}

	var refs []d2automap.TileRef

	for i := range t.Components.Floors {
		piso := &t.Components.Floors[i]
		if piso.Hidden() || piso.Prop1 == 0 {
			continue
		}

		refs = append(refs, d2automap.TileRef{Layer: d2automap.LayerFloor, Type: int(d2enum.TileFloor),
			Style: int(piso.Style), Sequence: int(piso.Sequence)})
	}

	for i := range t.Components.Walls {
		pared := &t.Components.Walls[i]
		if pared.Hidden() || pared.Prop1 == 0 {
			continue
		}

		refs = append(refs, d2automap.TileRef{Layer: d2automap.LayerWall, Type: int(pared.Type),
			Style: int(pared.Style), Sequence: int(pared.Sequence)})
	}

	return refs
}

// minimapaDelJuego: todo el minimapa armado. Uno solo por corrida (nil = apagado).
type minimapaDelJuego struct {
	Tabla  *d2automap.Table
	Modelo *d2automap.Model
	Arte   map[int]*celdaDelMinimapa
	Pal    [256][4]uint8

	// lo que ocupa, medido
	BytesArte   int
	BytesTabla  int
	CuadrosArte int
	SinArte     int // celdas del modelo que caen en el dibujo y NO tienen cuadro decodificado

	// el reveal
	ultimaX, ultimaY float64
	hayUltima        bool
	revelados        int // cuantas veces se revelo
	reveladasUltimas int // celdas de las ultimas casillas reveladas
	reveladasMax     int

	// el dibujo, por cuadro
	cuadrosDibujados int
	celdasUltimas    int
	celdasMax        int
	pxUltimos        int
	pxTotal          int
	msUltimo         int
	msTotal          int
	msMax            int
	saltados         int // cuadros en los que el minimapa NO se redibujo (palanca "cambio")
	crcAntes         uint32
	crcDespues       uint32

	// lo ultimo que se dibujo (para la palanca "cambio": si el heroe no se movio en la caja y el modelo no crecio,
	// no hay un pixel nuevo que pintar)
	hayDibujo        bool
	ultimoHeroeX     int
	ultimoHeroeY     int
	ultimasCeldasDib int

	// el cierre
	decision string
}

// elMinimapa: el unico de la corrida.
var elMinimapa *minimapaDelJuego

// ============================== LA CARGA ==============================

// cargarElMinimapa: lee `AutoMap.txt` y `maximap.dc6` del MPQ, decodifica el arte que la tabla del pueblo pide y
// deja el modelo vacio. Va DESPUES del HUD y ANTES de que el paso 5 suelte las fuentes (misma ventana que el HUD).
func cargarElMinimapa() {
	if minimapaKnob != "si" {
		println("JUEGO130: MINIMAPA apagado (minimapa=", minimapaKnob, "): la corrida es la de DC-002.128 sin minimapa")

		return
	}

	t0 := d2cMs()

	m := &minimapaDelJuego{
		Modelo: d2automap.NewModel(),
		Arte:   map[int]*celdaDelMinimapa{},
		Pal:    paletaDelHUD(paletaDelActo1, "acto 1"),
	}

	// ---- 1. LA TABLA, FILTRADA AL NIVEL DEL PUEBLO (ver la cabecera: 271.144 B contra 6.176 B)
	//
	// SE LEE POR EL FLUJO (`Open`), NO DE UN TIRON. `fuente.Pedir` hace `make([]byte, 122022)` en el heap y con el
	// POC al 91 % eso es OOM medido; y pedir un bloque de 122 KB al monton de C y devolverlo despues con
	// `d2c_soltar_bloque` dejo el recolector en un estado en el que la PRIMERA escritura de mapa de la corrida
	// (`runtime.mapassign`, en el revelado del cuadro 1) moria con `Illegal instruction` (medido). Por el flujo no
	// hay bloque de nadie: el scanner pide 1 KB y de ahi solo salen las 83 filas del pueblo (6.176 B).
	fuente := fuenteDe(rutaDatos)
	if fuente == nil {
		println("JUEGO130: MINIMAPA SIN FUENTE DE DATOS: no hay tabla ni arte")

		return
	}

	tamano, hay := fuente.Tamano(rutaTablaAutomap)
	if !hay || tamano == 0 {
		println("JUEGO130: MINIMAPA SIN TABLA ruta=", rutaTablaAutomap)

		return
	}

	flujo, err := fuente.Open(rutaTablaAutomap)
	if err != nil {
		println("JUEGO130: MINIMAPA SIN TABLA ruta=", rutaTablaAutomap)

		return
	}

	bytesTabla := tamano

	tabla, err := d2automap.ParseTxtLevels(flujo, nivelDelPueblo)
	if err != nil {
		println("JUEGO130: MINIMAPA la tabla NO PARSEA:", err.Error())

		return
	}

	m.Tabla = tabla
	m.BytesTabla = bytesTabla

	println("JUEGO130: MINIMAPA TABLA archivo=", rutaTablaAutomap, "bytes=", bytesTabla,
		"filasDelPueblo=", len(tabla.LevelRows(nivelDelPueblo)), "nivel=", nivelDelPueblo,
		"nombre=", d2automap.LevelName(nivelDelPueblo), "porElFlujo= si")

	// ---- 2. EL ARTE: los cuadros que piden las filas del pueblo, decodificados y decimados a 8x16
	faltan := m.cargarElArte(fuente)

	println("JUEGO130: MINIMAPA ARTE archivo=", rutaArteAutomap, "cuadros=", m.CuadrosArte,
		"bytesEnMontonDeC=", m.BytesArte, "faltan=", faltan, "montonC_libre=", d2cMontonLibre())

	// ---- 3. EL MODELO: vacio (el reveal lo llena cuadro a cuadro, siguiendo al jugador) o el pueblo entero
	if minimapaTodo == "si" {
		m.revelarTodoElPueblo()
	}

	elMinimapa = m

	println("JUEGO130: MINIMAPA LISTO celdas=", m.Modelo.Count(), "piso=", m.Modelo.CountLayer(d2automap.LayerFloor),
		"pared=", m.Modelo.CountLayer(d2automap.LayerWall), "palanca=", minimapaKnob, "cuando=", minimapaCuando,
		"todo=", minimapaTodo, "ms=", int(d2cMs()-t0))

	semiespacio("despues-del-minimapa")
	memoria("minimapa-despues")
}

// cargarElArte: decodifica del DC6 los cuadros que las filas del nivel piden y los deja DECIMADOS en el monton de C.
// Devuelve cuantos cuadros no se pudieron decodificar (0 = entero).
//
// POR QUE DECIMADO. El motor pide para el minimapa `maximaps.dc6` con cuadros de 8x16 y ese archivo no existe en
// esta instalacion (medido en la sonda): el que hay es `maximap.dc6` con cuadros de 16x32, que es el arte del mapa
// grande. Decimarlo (1 de cada 2 pixeles en x y en y) da exactamente la caja de 8x16 que el motor dibuja en el
// minimapa, y ademas es 4 veces mas barato: 128 B por cuadro contra 512.
func (m *minimapaDelJuego) cargarElArte(fuente *fuenteconsola.Fuente) int {
	// CUADRO POR CUADRO DESDE EL FLUJO, NO EL ARCHIVO ENTERO. El DC6 son 234.458 B y con el POC al 91 % ni el heap
	// ni el monton de C aguantan el bloque entero (medido: 122 KB + 234 KB de bloques de C revientan la cuenta de
	// externos del recolector, y devolverlos con `d2c_soltar_bloque` deja la primera escritura de mapa de la corrida
	// en `Illegal instruction`). Se lee la cabecera (24 B) y la tabla de punteros (5.040 B), y despues SOLO los 59
	// cuadros que el pueblo pide: se arma un DC6 de un solo cuadro y lo decodifica el decodificador del motor.
	tamano, hay := fuente.Tamano(rutaArteAutomap)
	if !hay || tamano < 28 {
		println("JUEGO130: MINIMAPA SIN ARTE ruta=", rutaArteAutomap)

		return -1
	}

	flujo, err := fuente.Open(rutaArteAutomap)
	if err != nil {
		println("JUEGO130: MINIMAPA SIN ARTE ruta=", rutaArteAutomap)

		return -1
	}

	cabecera := make([]byte, 24)
	if _, err := io.ReadFull(flujo, cabecera); err != nil {
		println("JUEGO130: MINIMAPA el arte NO TIENE CABECERA:", err.Error())

		return -1
	}

	cuadros := int(binary.LittleEndian.Uint32(cabecera[16:20])) *
		int(binary.LittleEndian.Uint32(cabecera[20:24]))
	if cuadros <= 0 || cuadros > 100000 {
		println("JUEGO130: MINIMAPA el arte declara", cuadros, "cuadros: no se usa")

		return -1
	}

	punteros := make([]byte, cuadros*4)
	if _, err := io.ReadFull(flujo, punteros); err != nil {
		println("JUEGO130: MINIMAPA el arte NO TIENE PUNTEROS:", err.Error())

		return -1
	}

	puntero := func(c int) int { return int(binary.LittleEndian.Uint32(punteros[c*4:])) }

	// los cuadros que piden las filas del pueblo (la union, sin repetir)
	var cels []int

	visto := map[int]bool{}

	for _, fila := range m.Tabla.LevelRows(nivelDelPueblo) {
		for _, c := range fila.Cels {
			if !visto[c] {
				visto[c] = true
				cels = append(cels, c)
			}
		}
	}

	println("JUEGO130: MINIMAPA ARTE pide", len(cels), "cuadros de", cuadros, "del archivo de", tamano, "B",
		"caja=", anchoCeldaMini, "x", altoCeldaMini, "(decimado de", cuadroDeArteEntero, "x", altoDeArteEntero, ")")

	faltan := 0

	marco := make([]byte, 28+64)

	for _, c := range cels {
		if c < 0 || c >= cuadros {
			faltan++

			continue
		}

		ini := puntero(c)

		fin := tamano
		if c+1 < cuadros {
			fin = puntero(c + 1)
		}

		if ini < 32 || fin <= ini || fin > tamano {
			faltan++

			continue
		}

		largo := 28 + fin - ini
		if cap(marco) < largo {
			marco = make([]byte, largo)
		} else {
			marco = marco[:largo]
		}

		// la cabecera del DC6, con UNA direccion y UN cuadro, y el puntero a la altura del unico cuadro
		copy(marco[:24], cabecera)
		binary.LittleEndian.PutUint32(marco[16:20], 1)
		binary.LittleEndian.PutUint32(marco[20:24], 1)
		binary.LittleEndian.PutUint32(marco[24:28], 28)

		if _, err := flujo.Seek(int64(ini), io.SeekStart); err != nil {
			faltan++

			continue
		}

		if _, err := io.ReadFull(flujo, marco[28:]); err != nil {
			faltan++

			continue
		}

		// EL CUADRO SE RECORTA A LO QUE DECLARA SU PROPIA CABECERA (32 B + los datos + 3 de terminacion). Los
		// punteros del DC6 real traen relleno entre cuadros (medido: con el relleno adentro el decodificador del
		// motor se sale del buffer y la consola tira `panic: slice bounds out of range`). Si lo declarado no entra
		// en el tramo del puntero, el cuadro no se usa.
		if fin-ini < 32 {
			faltan++

			continue
		}

		anchoCuadro := int(binary.LittleEndian.Uint32(marco[28+4 : 28+8]))
		altoCuadro := int(binary.LittleEndian.Uint32(marco[28+8 : 28+12]))
		largoDatos := int(binary.LittleEndian.Uint32(marco[28+28 : 28+32]))

		if anchoCuadro != cuadroDeArteEntero || altoCuadro != altoDeArteEntero ||
			largoDatos <= 0 || 32+largoDatos+3 > fin-ini {
			faltan++

			continue
		}

		marco = marco[:28+32+largoDatos+3]

		uno, err := d2dc6.Load(marco)
		if err != nil || len(uno.Frames) != 1 {
			faltan++

			continue
		}

		crudo := uno.DecodeFrame(0)
		if len(crudo) != cuadroDeArteEntero*altoDeArteEntero {
			faltan++

			continue
		}

		// el arte va al MONTON DE C (128 B por cuadro): el semiespacio queda al filo con el HUD puesto
		indices := bloqueDeC(anchoCeldaMini * altoCeldaMini)

		for y := 0; y < altoCeldaMini; y++ {
			for x := 0; x < anchoCeldaMini; x++ {
				// 1 de cada 2 pixeles del cuadro de 16x32
				indices[y*anchoCeldaMini+x] = crudo[(y*2)*cuadroDeArteEntero+(x*2)]
			}
		}

		m.Arte[c] = &celdaDelMinimapa{
			Cel: c, Ancho: anchoCeldaMini, Alto: altoCeldaMini, Indices: indices,
			Pixeles: contarNoVacios(indices),
		}

		m.BytesArte += len(indices)
		m.CuadrosArte++
	}

	println("JUEGO130: MINIMAPA ARTE decodificados=", m.CuadrosArte, "faltan=", faltan, "bytes=", m.BytesArte,
		"punterosLeidos=", len(punteros), "nombreQuePideElMotor=", rutaNombre8x16, "existe=false (medido)")

	return faltan
}

// revelarTodoElPueblo: el caso peor (lo que mide la tarjeta). Se revela el pueblo entero de una: el area es el
// pueblo completo, que es lo mismo que revelarlo caminando pero sin caminar.
func (m *minimapaDelJuego) revelarTodoElPueblo() {
	w, h := fuenteDelMotor{}.Size()
	if w == 0 || h == 0 {
		println("JUEGO130: MINIMAPA sin motor del mapa: no se puede revelar el pueblo")

		return
	}

	agregadas := m.Modelo.RevealTiles(m.Tabla, fuenteDelMotor{}, nivelDelPueblo, semillaDelAutomap(),
		float64(w/2), float64(h/2), d2automap.Area{HalfW: w * 8, HalfH: h * 4})

	println("JUEGO130: MINIMAPA REVEAL-TODO casillas=", w, "x", h, "agregadas=", agregadas, "celdas=", m.Modelo.Count(),
		"piso=", m.Modelo.CountLayer(d2automap.LayerFloor), "pared=", m.Modelo.CountLayer(d2automap.LayerWall))
}

// semillaDelAutomap: la del motor (`Automap.Advance`: uint32(nivel+1) * automapSeedMul). El multiplicador va en una
// VARIABLE y no en una constante: `uint32(2) * 2654435761` no entra en uint32 como constante y gccgo lo rechaza (la
// cuenta del motor la hace en 32 bits, que es lo que hace la variable).
var multiplicadorDeLaSemilla uint32 = 2654435761

func semillaDelAutomap() uint32 {
	return uint32(nivelDelPueblo+1) * multiplicadorDeLaSemilla
}

// revelarElMinimapa: el reveal SIGUIENDO AL JUGADOR. Se llama una vez por cuadro, antes de dibujar: si el heroe se
// movio lo suficiente (`MovedFar`, el disparador del motor) se revelan las casillas de alrededor.
func (m *minimapaDelJuego) revelarElMinimapa(jugador int) {
	if m == nil || m.Tabla == nil || motorDelMapa == nil {
		return
	}

	hx, hy := celdaDelJugadorDe(jugador)
	cx, cy := d2automap.WorldCell(hx, hy)

	if m.hayUltima && !d2automap.MovedFar(m.ultimaX, m.ultimaY, cx, cy) {
		return
	}

	m.ultimaX, m.ultimaY, m.hayUltima = cx, cy, true

	antes := m.Modelo.Count()

	m.Modelo.RevealTiles(m.Tabla, fuenteDelMotor{}, nivelDelPueblo, semillaDelAutomap(), hx, hy,
		d2automap.ScreenArea(ancho, alto))

	m.revelados++
	m.reveladasUltimas = m.Modelo.Count() - antes

	if m.reveladasUltimas > m.reveladasMax {
		m.reveladasMax = m.reveladasUltimas
	}

	println("JUEGO130: MINIMAPA REVEAL cuadro=", cuadro, "jugador=", jugador+1, "heroe=", hx, hy,
		"agregadas=", m.reveladasUltimas, "celdas=", m.Modelo.Count(),
		"piso=", m.Modelo.CountLayer(d2automap.LayerFloor), "pared=", m.Modelo.CountLayer(d2automap.LayerWall))
}

// celdaDelJugadorDe: la celda del motor del jugador i (el 1 es el que mueve el guion con un solo jugador).
func celdaDelJugadorDe(jugador int) (x, y float64) {
	if jugador <= 0 || len(jugadores) == 0 {
		return celdaDelJugadorX(), celdaDelJugadorY()
	}

	if jugador >= len(jugadores) || jugadores[jugador] == nil || jugadores[jugador].Mov == nil ||
		jugadores[jugador].Mov.Est == nil {
		return celdaDelJugadorX(), celdaDelJugadorY()
	}

	return jugadores[jugador].Mov.Est.Celda()
}

// ============================== EL DIBUJO ==============================

// dibujarElMinimapa: pinta las celdas del modelo en la caja del minimapa y devuelve los pixeles pegados y los ms.
//
// EL ORDEN ES EL DEL MODELO (`Cells()`: capa, despues y, despues x, despues cuadro), que es el del motor: los
// pisos, despues las paredes. El clip es el de la caja y la celda se pega con su esquina de ABAJO a la izquierda,
// como en `drawCells` del motor.
func (m *minimapaDelJuego) dibujarElMinimapa(sup *pvr.Surface, jugador, anchoLienzo, altoLienzo int) (int, int) {
	if m == nil {
		return 0, 0
	}

	hx, hy := celdaDelJugadorDe(jugador)
	cx, cy := d2automap.WorldCell(hx, hy)

	d210, d214 := d2automap.MiniBoxOffsets(anchoLienzo, altoLienzo, false)
	lay := d2automap.ComputeLayoutOffsets(d2automap.SizeMini, anchoLienzo, altoLienzo, cx, cy,
		d2automap.PanelNone, false, d210, d214)

	t0 := d2cMs()

	// LA PALANCA "cambio": si el heroe no se movio en la caja y el modelo no crecio desde el ultimo dibujo, no hay
	// un pixel nuevo que pintar y el minimapa se salta (se cuenta en `saltados`). Es el A/B de la decision.
	mx, my := lay.HeroScreen(cx, cy)

	if minimapaCuando == "cambio" && m.hayDibujo && mx == m.ultimoHeroeX && my == m.ultimoHeroeY &&
		m.Modelo.Count() == m.ultimasCeldasDib {
		m.saltados++
		m.celdasUltimas, m.pxUltimos, m.msUltimo = 0, 0, 0

		return 0, 0
	}

	pintados, celdas := 0, 0

	// EL FONDO NEGRO DE LA CAJA, ANTES DE LAS CELDAS. El arte del automapa (`maximap.dc6`) son lineas FINAS pensadas
	// para leerse sobre negro: el juego las dibuja sobre la pantalla del mapa, que va en negro. Sobre el terreno del
	// POC se pierden (medido en Flycast: el lienzo cambiaba y el mapa no se veia). Con la caja en negro, las celdas
	// se leen.
	sup.PushTranslation(lay.Clip.X0, lay.Clip.Y0)
	sup.DrawRect(lay.Clip.X1-lay.Clip.X0, lay.Clip.Y1-lay.Clip.Y0, fondoDelMinimapa)
	sup.Pop()

	for _, c := range m.Modelo.Cells() {
		arte := m.Arte[c.Cel]
		if arte == nil {
			m.SinArte++

			continue
		}

		x, y := lay.ToScreen(c.X, c.Y)

		// la caja de la celda contra la caja del minimapa (ancla abajo a la izquierda)
		if x+arte.Ancho <= lay.Clip.X0 || x >= lay.Clip.X1 || y <= lay.Clip.Y0 || y-arte.Alto >= lay.Clip.Y1 {
			continue
		}

		sup.PushTranslation(x, y-arte.Alto)
		sup.RenderIndicesTintado(arte.Indices, arte.Ancho, arte.Alto, m.Pal[:])
		sup.Pop()

		pintados += arte.Pixeles
		celdas++
	}

	// ---- EL MARCADOR DEL HEROE: la polilinea del motor, en su sitio de la caja
	m.dibujarElMarcador(sup, mx-8, my+8, lay)

	ms := int(d2cMs() - t0)

	m.hayDibujo, m.ultimoHeroeX, m.ultimoHeroeY, m.ultimasCeldasDib = true, mx, my, m.Modelo.Count()
	m.cuadrosDibujados++
	m.celdasUltimas = celdas
	m.pxUltimos = pintados
	m.pxTotal += pintados
	m.msUltimo = ms
	m.msTotal += ms

	if celdas > m.celdasMax {
		m.celdasMax = celdas
	}

	if ms > m.msMax {
		m.msMax = ms
	}

	// DC-002.134: la linea del minimapa es evidencia por cuadro: con `VERBOSO=no` se calla.
	if verboso134() {
		println("JUEGO130: MINIMAPA cuadro=", cuadro, "jugador=", jugador+1, "celdas=", celdas, "de", m.Modelo.Count(),
			"pixelesPintados=", pintados, "ms=", ms, "heroe=", hx, hy, "enLaCaja=", mx, my,
			"caja=", lay.Clip.X0, lay.Clip.Y0, lay.Clip.X1, lay.Clip.Y1, "sinArte=", m.SinArte)
	}

	return pintados, ms
}

// dibujarElMarcador: la cruz del heroe en el minimapa, con la polilinea del motor (cada coordenada DUPLICADA) y su
// color. Se dibuja con el pincel de lineas del POC (`DrawLine` desde la traslacion actual).
func (m *minimapaDelJuego) dibujarElMarcador(sup *pvr.Surface, x, y int, lay d2automap.Layout) {
	// el marcador solo se pinta si su centro cae en la caja: fuera de la caja no hay nada que marcar
	if !lay.Clip.Contains(x, y) {
		return
	}

	for i := 0; i < marcadorPuntos; i++ {
		p, q := cruzDelMarcador[i], cruzDelMarcador[(i+1)%marcadorPuntos]
		x1, y1 := x+p[0]*2, y+p[1]*2
		x2, y2 := x+q[0]*2, y+q[1]*2

		sup.PushTranslation(x1, y1)
		sup.DrawLine(x2-x1, y2-y1, colorDelMarcador)
		sup.Pop()
	}
}

// ============================== EL CIERRE ==============================

// resumenDelMinimapa: los numeros del criterio y LA DECISION (siempre o por tecla) con el numero que la sostiene.
func resumenDelMinimapa() {
	if elMinimapa == nil {
		println("JUEGO130: MINIMAPA RESUMEN apagado (minimapa=", minimapaKnob, ")")

		return
	}

	m := elMinimapa

	promedio := 0
	if m.cuadrosDibujados > 0 {
		promedio = m.msTotal / m.cuadrosDibujados
	}

	// el cuadro del POC (el que se midio en el mismo cuadro: el mapa es el costo dominante, ~2,8 s medidos)
	msCuadro := msDelCuadroDelMapa
	pct := 0

	if msCuadro > 0 {
		pct = promedio * 100 / msCuadro
	}

	// LA DECISION, CON EL NUMERO. Si el minimapa cuesta menos del 10 % del cuadro, se dibuja SIEMPRE (que es lo que
	// hace el juego: el minimapa esta ahi, no se aprieta una tecla); si costara mas, iria POR TECLA como el mapa
	// grande (Tab en el motor). El umbral y el numero van en la misma linea para que la decision se pueda discutir.
	if pct < 10 {
		m.decision = "siempre"
	} else {
		m.decision = "por-tecla"
	}

	println("JUEGO130: MINIMAPA RESUMEN celdasEnElModelo=", m.Modelo.Count(), "piso=",
		m.Modelo.CountLayer(d2automap.LayerFloor), "pared=", m.Modelo.CountLayer(d2automap.LayerWall),
		"celdasDibujadasMax=", m.celdasMax, "celdasDibujadasUltimo=", m.celdasUltimas,
		"pxPorCuadro=", m.pxUltimos, "pxTotal=", m.pxTotal, "sinArte=", m.SinArte)

	println("JUEGO130: MINIMAPA COSTO cuadrosDibujados=", m.cuadrosDibujados, "saltados=", m.saltados,
		"msPromedio=", promedio, "msMax=", m.msMax, "msTotal=", m.msTotal, "msDelCuadroDelMapa=", msCuadro,
		"pctDelCuadro=", pct)

	println("JUEGO130: MINIMAPA ARTE-RESUMEN cuadros=", m.CuadrosArte, "bytesEnMontonDeC=", m.BytesArte,
		"bytesDeLaTabla=", m.BytesTabla, "bytesDeArteSiSeDecodificaraEntero=", 645120,
		"bytesDecimadoAlPueblo=", m.BytesArte, "montonC_libre=", d2cMontonLibre())

	println("JUEGO130: MINIMAPA REVEAL-RESUMEN revelados=", m.revelados, "agregadasMax=", m.reveladasMax,
		"agregadasUltimas=", m.reveladasUltimas, "sigueAlJugador=", m.revelados > 1)

	println("JUEGO130: MINIMAPA DECISION escogido=", m.decision, "msPromedio=", promedio, "pctDelCuadro=", pct,
		"umbralPct=", 10, "cuando=", minimapaCuando, "todo=", minimapaTodo)

	println("JUEGO130: MINIMAPA CRITERIO seDibujo=", m.cuadrosDibujados > 0 && m.celdasMax > 0,
		"pintoPixeles=", m.pxTotal > 0, "elRevealSigueAlJugador=", m.revelados > 1,
		"celdasEnLaCaja=", m.celdasMax, "msPorCuadro=", promedio, "arteEnElMontonDeC=", m.BytesArte > 0,
		"decision=", m.decision)
}

// msDelCuadroDelMapa: lo que tardo el mapa en el ultimo cuadro (lo deja `dibujar`), que es contra lo que se mide el
// porcentaje del minimapa. Con 0 (el mapa no se midio) el porcentaje no se inventa: queda en 0.
var msDelCuadroDelMapa int

// usaElArteDeUnCuadro: comprueba que el cuadro cel esta decodificado (para el log de la carga, sin recorrer dos
// veces la tabla).
func (m *minimapaDelJuego) usaElArteDeUnCuadro(cel int) bool {
	_, ok := m.Arte[cel]

	return ok
}

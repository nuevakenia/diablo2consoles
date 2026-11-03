package main

// DC-002.125 -- EL HUD: LAS BARRAS DE VIDA Y MANA, EL CINTURON DE POCIONES Y EL NOMBRE DE LA ZONA.
//
// QUE ES. El HUD del juego, dibujado ENCIMA del pueblo con el MISMO pincel con el que se pinta el mapa y los
// jugadores (`pvr.Surface.RenderIndicesTintado`), con los graficos del motor sacados del MPQ y con los VALORES
// REALES del heroe (charstats.txt -> `d2statlist.Class.BaseMax`, el mismo camino que `d2hero/hero_totals.go`).
// No hay un solo pixel dibujado a mano: todos son cuadros de DC6 del juego.
//
// DE DONDE SALE CADA PIEZA (nada elegido a ojo; los numeros son los de `d2game/d2player`):
//
//   - LAS BARRAS NO SON BARRAS: el motor dibuja DOS GLOBOS de 80x80 (`hlthmana.DC6`, cuadro 0 = vida, cuadro 1
//     = mana) y los recorta con un `maskRect` que empieza en `globeHeight - barHeight`
//     (`globeWidget.Render`). O sea: se pinta la PARTE DE ABAJO del globo, proporcional a vida/maxVida. Aca se
//     hace exactamente eso, con los indices del cuadro: se le pasa al pincel la rebanada de filas
//     [80-relleno, 80). Encima va `overlap.DC6` (82x88, cuadro 0 = vida, 1 = mana), que es el vidrio con su
//     brillo: si se pinta el globo entero sin la tapa, el numero de pixeles es el del globo, no el de la barra.
//   - LAS POSICIONES SON LAS DEL MOTOR (`globeWidget.go`): el globo de vida va en el widget (0, alto) con
//     esquina (29, -13) y su tapa en (28, -5); el de mana en el widget (ancho-117, alto) con esquina (6, -13) y
//     su tapa en (7, -9). Todas las coordenadas y son el BORDE DE ABAJO del sprite, como en el motor
//     (`globeWidget.Contains` compara `py <= wy && py >= wy-height`).
//   - EL CINTURON: `belts.txt` tiene la columna `numboxes` y la fila `default` son 4 cajas (medido; y la tabla
//     del motor lo dice igual: `d2inventory.BeltBoxesByType` = [12 8 4 16 8 12 16] y `BeltDefaultBoxes` = 4).
//     4 cajas son UNA fila de 4 columnas (`belt_panel.go`: paso 31x32). El grafico del cinturon del HUD es
//     `ctrlpnl_popbelt.DC6` (125x32 = 4x31), y las pociones son su `invfile` de `misc.txt` (28x28): dos de vida
//     y dos de mana, como el juego las pone (`misc.txt` hpot/mpot).
//   - EL NOMBRE DE LA ZONA: `d2level.DefaultLevelNames[1]` = "Rogue Encampment", que es el camino que el PROPIO
//     motor usa cuando la tabla de strings no esta (`level_change.go`, `levelName`). En la consola la tabla
//     entera no entra (`string.tbl` son 390.570 B y el semiespacio del recolector 1.835.008 B con el POC al
//     91 %), asi que se usa el fallback del motor, que dice el MISMO texto. La fuente es `font16` (DC6 de 256
//     cuadros de 14x16 y su `.tbl` de 3.596 B), con la paleta de unidades, como `hud.go` (`zoneChangeText`).
//   - LOS VALORES: `charstats.txt` (3.360 B del parche) -> `d2statlist.ParseClasses` -> `Class.BaseMax(nivel,
//     vit, ene)`. Para la amazona al nivel 1 da vida 50 y mana 15 (medido con la sonda del host, y en la consola
//     sale el mismo numero porque es la misma tabla y la misma cuenta). Con `NIVELES=si` cada jugador va con SU
//     nivel (1, 2, 3, 4) y su vida/mana salen de esa misma cuenta: es lo que hace que en el split cada mitad
//     tenga SU HUD y no una copia.
//
// LO QUE ESTE HUD NO HACE (y no es un olvido): no muestra el valor numerico de la vida y el mana (el motor lo
// muestra en el tooltip al pasar el puntero, y aca no hay puntero), y no dibuja la experiencia ni el aguante por
// barra propia: el aguante se informa en la linea del criterio y la experiencia es de otra tarjeta.

import (
	"hash/crc32"
	"runtime"
	"unsafe"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2dc6"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2font"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2level"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2statlist"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2render/pvr"
)

//extern d2c_pedir_bloque
func d2cPedirBloque(tamano uint) unsafe.Pointer

// bloqueDeC: n bytes en el MONTON DE C, no en el semiespacio del recolector.
//
// POR QUE (medido, DC-002.125). El POC llega al momento de cargar el HUD con el semiespacio al 97 % (1.816.240 de
// 1.835.008 B): cada byte que se le suma dispara una recoleccion COMPLETA. Pidiendo ahi los 22.848 B del HUD, la
// carga costo 3,2 s por pocion y 61,7 s por la fuente, y la corrida murio con `fatal error: out of memory`. El
// monton de C tenia 1,3 MB libres y el recolector no lo cuenta ni lo copia. El bloque no se devuelve: el arte del
// HUD vive lo que vive la corrida.
func bloqueDeC(n int) []byte {
	if n <= 0 {
		return nil
	}

	if p := d2cPedirBloque(uint(n)); p != nil {
		// el truco del arreglo gigante: un `unsafe.Slice` no existe en el lenguaje de este modulo (go 1.16)
		return (*[1 << 24]byte)(p)[:n:n]
	}

	println("JUEGO125: HUD sin bloque de C para", n, "bytes: va al semiespacio")

	return make([]byte, n)
}

const (
	// las rutas del motor (`d2resource`), con el NOMBRE QUE TIENE EL MPQ: el panel es `ctrlpnl7.DC6`. El nombre
	// que pide el motor (`/data/global/ui/PANEL/800ctrlpnl7.dc6`, `d2resource.GamePanels`) NO EXISTE en esta
	// instalacion (1.14b) y eso lo mide la sonda: portar `hud.go` tal cual fallaria al cargar el panel.
	rutaPanelHUD   = `data\global\ui\PANEL\ctrlpnl7.DC6`
	rutaGlobosHUD  = `data\global\ui\PANEL\hlthmana.DC6`
	rutaTapaHUD    = `data\global\ui\PANEL\overlap.DC6`
	rutaCinturon   = `data\global\ui\PANEL\ctrlpnl_popbelt.DC6`
	rutaPocionVida = `data\global\items\invhp1.dc6`
	rutaPocionMana = `data\global\items\invmp1.dc6`
	rutaCharStats  = `data\global\excel\charstats.txt`
	rutaFuenteZona = `data\local\font\latin\font16`

	// palCieloHUD: la paleta de los globos y del panel (`globeWidget.load` pide `d2resource.PaletteSky`).
	// Las pociones y el cinturon van con la paleta de unidades, como los `invfile` del inventario.
	palCieloHUD = `data\global\palette\sky\pal.dat`

	// EL CUADRO DE CADA COSA. Los numeros salen del propio `.dc6` (los imprime la sonda) y coinciden con las
	// constantes de `hud.go`: frameHealthStatus = 0, frameManaStatus = 1, frameStamina = 2, framePotions = 3,
	// frameNewSkillsSelector = 4, frameRightGlobeHolder = 5.
	cuadroGloboVida   = 0
	cuadroGloboMana   = 1
	cuadroPanel       = 0
	cuadroTapaVida    = 0
	cuadroTapaMana    = 1
	cuadroDelCinturon = 0

	// LAS POSICIONES del motor (`globeWidget.go`), en pixeles desde el borde que dice cada una.
	globoVidaDX   = 29
	globoVidaDY   = -13
	tapaVidaDX    = 28
	tapaVidaDY    = -5
	manaWidgetDX  = 117 // `manaGlobeScreenOffsetX`: el widget de mana arranca a 117 del borde derecho
	globoManaDX   = 6   // `manaStatusOffsetX`
	globoManaDY   = -13 // `manaStatusOffsetY`
	tapaManaDX    = 7   // `rightGlobeOffsetX`
	tapaManaDY    = -9  // `rightGlobeOffsetY`
	altoDelGlobo  = 80  // `globeHeight`
	anchoDelGlobo = 80  // `globeWidth`

	// EL CINTURON (`belt_panel.go`): 4 columnas con paso 31x32.
	cinturonColumnas = 4
	cinturonPasoX    = 31
	cinturonPasoY    = 32
	cinturonDX       = 1 // el primero cae 1 px adentro del grafico
	cinturonDY       = 2

	// la zona del POC: el pueblo del acto 1 (`townN1.ds1`), id 1 en levels.txt.
	nivelDelPueblo = 1

	// donde se dibuja el nombre de la zona: arriba y centrado (`hud.go`: screenWidth/2, screenHeight/4; aca se
	// sube al borde de arriba para no tapar al jugador, que es el criterio de la tarjeta: que el HUD se VEA).
	zonaArriba = 6
)

// piezaDelHUD: un cuadro de DC6 ya decodificado a indices (el 0 transparente), con su caja y sus numeros.
type piezaDelHUD struct {
	Nombre  string
	Ancho   int
	Alto    int
	Indices []byte
	Pixeles int
	CRC     uint32
}

// valoresDelHeroe: LA VIDA Y EL MANA DEL JUGADOR, con la tabla y la cuenta del motor.
type valoresDelHeroe struct {
	Clase                        string
	Nivel                        int
	Vida, VidaMax                int
	Mana, ManaMax                int
	Aguante, AguanteMax          int
	Vitalidad, Energia           int
	HpAdd                        int
}

// hudDelJuego: todo el HUD armado. Vive una sola vez (los graficos son los mismos para los cuatro jugadores) y
// lo que cambia por jugador son los VALORES.
type hudDelJuego struct {
	Panel    *piezaDelHUD
	Globo    [2]*piezaDelHUD // 0 vida, 1 mana
	Tapa     [2]*piezaDelHUD
	Cinturon *piezaDelHUD
	Pociones [4]*piezaDelHUD // 0 y 1 de vida, 2 y 3 de mana
	Glifos   map[byte]*glifoDelHUD
	Zona     string
	Clase    string
	Nivel    int // el nivel de la tabla (el que se usa de base)
	PalCielo [256][4]uint8
	PalUni   [256][4]uint8
	Bytes    int // lo que ocupa el HUD en RAM (indices + glifos), medido

	// lo que se pinta y lo que cuesta, por cuadro
	pxPintados int
	msDibujo   int
	cuadrosConHUD int
}

// glifoDelHUD: el cuadro de la fuente que corresponde a una letra, ya decodificado, y su ancho.
//
// OJO CON LOS DOS ANCHOS. `Ancho` es el de la CAJA del cuadro (14 px en font16, lo que se pinta) y `Paso` es el
// AVANCE de la letra (`FontGlyph.Width()`, que es lo que suma `Font.GetTextMetrics`): son distintos y por eso
// "Rogue Encampment" mide 141 px, no 16x14=224. Avanzar con el ancho de la caja deja el texto esparcido: es el
// primer error que se vio en la consola (224 px de texto contra los 141 que dice la tabla) y se arreglo asi.
type glifoDelHUD struct {
	Ancho   int
	Alto    int
	Paso    int
	Indices []byte
}

// elHUD es el unico HUD de la corrida (nil = apagado).
var elHUD *hudDelJuego

// valoresPorJugador: la vida y el mana de CADA jugador (en el split cada mitad lleva el suyo).
var valoresPorJugador []valoresDelHeroe

// vidaActual y manaActual: lo que muestran las barras AHORA. Con `hudInstrumento` bajan cuadro a cuadro (el
// POC no tiene dano al heroe todavia); sin el instrumento la barra queda llena, que es como nace el heroe.
var vidaActual, manaActual []int

// ============================== LA CARGA ==============================

// cargarElHUD: lee del MPQ los graficos del HUD y las tablas de los valores, y deja el HUD listo.
//
// VA CON EL VIDEO YA ANDANDO Y EL PUEBLO COMPUESTO, por la misma razon medida que el bicho y el drop
// (DC-002.50): lo que crece antes de `PantallaIniciar` deja al PVR sin su monton de C. Lo que se queda vivo son
// los cuadros decodificados (~57 KB); los archivos (34 KB de panel, 21 KB de globos, 35 KB de fuente) se leen,
// se decodifica el cuadro que hace falta y se sueltan en el acto.
func cargarElHUD() {
	if hudKnob != "si" {
		println("JUEGO125: HUD apagado (hud=", hudKnob, "): la corrida es la de DC-002.126 sin HUD")

		return
	}

	if fuenteDe(rutaDatos) == nil || fuenteDe(rutaParche) == nil {
		println("JUEGO125: HUD SIN LAS FUENTES (d2data y el parche): no hay de donde leer los graficos ni charstats")
	}

	println("JUEGO125: HUD arranca con la palanca hud=", hudKnob, "instrumento=", hudInstrumento,
		"nivelesDistintos=", nivelesDistintos)

	h := &hudDelJuego{Glifos: map[byte]*glifoDelHUD{}, Nivel: nivelDelPueblo}

	// ---- las paletas
	h.PalCielo = paletaDelHUD(palCieloHUD, "cielo")
	h.PalUni = palJugador // la misma con la que se pintan los heroes: `Units/pal.dat`

	// ---- los globos, sus tapas y el panel. Las TAPAS y el PANEL son la palanca `hudPiezas`: ver knobs.go (son lo
	// mas caro por pixel del HUD y con ellas el semiespacio no llega para la fase TRANSICION).
	h.Globo[0] = piezaDelMPQ("globo-de-vida", rutaGlobosHUD, cuadroGloboVida)
	h.Globo[1] = piezaDelMPQ("globo-de-mana", rutaGlobosHUD, cuadroGloboMana)

	if hudPiezas == "todas" {
		h.Tapa[0] = piezaDelMPQ("tapa-de-vida", rutaTapaHUD, cuadroTapaVida)
		h.Tapa[1] = piezaDelMPQ("tapa-de-mana", rutaTapaHUD, cuadroTapaMana)
		h.Panel = piezaDelMPQ("panel", rutaPanelHUD, cuadroPanel)
	} else {
		println("JUEGO125: HUD SIN TAPAS NI PANEL (hudPiezas=", hudPiezas, "): 26.600 B que no entran en la fase")
	}

	h.Cinturon = piezaDelMPQ("cinturon", rutaCinturon, cuadroDelCinturon)

	// ---- las cuatro pociones: dos de vida y dos de mana, con el `invfile` de misc.txt (medido: invhp1/invmp1
	// son 412 B y 28x28). El cinturon de 4 cajas es UNA fila de 4 columnas (`belt_panel.go`).
	h.Pociones[0] = piezaDelMPQ("pocion-de-vida", rutaPocionVida, 0)
	h.Pociones[1] = piezaDelMPQ("pocion-de-vida", rutaPocionVida, 0)
	h.Pociones[2] = piezaDelMPQ("pocion-de-mana", rutaPocionMana, 0)
	h.Pociones[3] = piezaDelMPQ("pocion-de-mana", rutaPocionMana, 0)

	// ---- el nombre de la zona, por el fallback del motor, y sus glifos con la fuente del juego
	h.Zona = d2level.DefaultLevelNames[nivelDelPueblo]
	if h.Zona == "" {
		h.Zona = "Level 1"
	}

	cargarLosGlifos(h)

	// ---- los valores del heroe: charstats.txt -> d2statlist (el camino del motor)
	h.Clase, h.Nivel = valoresDelHeroeDeLaTabla()

	// ---- un valor por jugador: con `nivelesDistintos` cada uno va con SU nivel (1, 2, 3, 4) y su vida y su
	// mana salen de la MISMA cuenta, que es lo que hace que cada mitad del split muestre lo suyo.
	valoresPorJugador = make([]valoresDelHeroe, jugadoresKnob)
	vidaActual = make([]int, jugadoresKnob)
	manaActual = make([]int, jugadoresKnob)

	for i := 0; i < jugadoresKnob; i++ {
		nivel := h.Nivel
		if nivelesDistintos == "si" {
			nivel = h.Nivel + i
		}

		v := valoresDelHeroeDeLaClase(h.Clase, nivel)
		valoresPorJugador[i] = v
		vidaActual[i] = v.VidaMax
		manaActual[i] = v.ManaMax

		println("JUEGO125: HUD VALORES jugador=", i+1, "clase=", v.Clase, "nivel=", v.Nivel,
			"vida=", v.Vida, "/", v.VidaMax, "mana=", v.Mana, "/", v.ManaMax, "aguante=", v.AguanteMax,
			"vit=", v.Vitalidad, "ene=", v.Energia, "hpadd=", v.HpAdd)
	}

	h.Bytes = h.bytesEnRAM()

	elHUD = h

	println("JUEGO125: HUD LISTO piezas=", h.piezas(), "glifos=", len(h.Glifos), "zona=", h.Zona,
		"bytesEnRAM=", h.Bytes, "montonC_libre=", d2cMontonLibre())
	semiespacio("despues-del-hud")
}

// piezas: cuantas piezas de grafico tiene el HUD armado (para la evidencia).
func (h *hudDelJuego) piezas() int {
	n := 0

	for _, p := range []*piezaDelHUD{h.Panel, h.Globo[0], h.Globo[1], h.Tapa[0], h.Tapa[1], h.Cinturon} {
		if p != nil {
			n++
		}
	}

	for _, p := range h.Pociones {
		if p != nil {
			n++
		}
	}

	return n
}

// bytesEnRAM: lo que el HUD ocupa de verdad (los indices de cada cuadro y de cada glifo).
func (h *hudDelJuego) bytesEnRAM() int {
	total := 0

	for _, p := range []*piezaDelHUD{h.Panel, h.Globo[0], h.Globo[1], h.Tapa[0], h.Tapa[1], h.Cinturon} {
		if p != nil {
			total += len(p.Indices)
		}
	}

	for _, p := range h.Pociones {
		if p != nil {
			total += len(p.Indices)
		}
	}

	for _, g := range h.Glifos {
		total += len(g.Indices)
	}

	return total
}

// piezaDelMPQ: lee un archivo del MPQ por el gestor de assets, decodifica UN cuadro del DC6 y suelta el archivo.
func piezaDelMPQ(nombre, ruta string, cuadro int) *piezaDelHUD {
	t0 := d2cMs()

	// EL ARCHIVO SE PIDE A LA FUENTE Y SE SUELTA EN EL ACTO (`Pedir`/`Liberar`), como el drop y el bicho: el
	// `am.LoadFile` LO DEJA EN LA CACHE DE TRAMOS, y con el semiespacio al 97 % eso es lo que no entra (medido: el
	// vivo se quedaba en 1.804.312 B y la corrida moria igual, con el arte ya en el monton de C).
	fuente := fuenteDe(rutaDatos)
	if fuente == nil {
		println("JUEGO125: HUD SIN FUENTE DE DATOS pieza=", nombre)

		return nil
	}

	tramo, err := fuente.Pedir(ruta)
	if err != nil || tramo == nil || len(tramo.Datos) == 0 {
		println("JUEGO125: HUD SIN GRAFICO pieza=", nombre, "ruta=", ruta)

		return nil
	}

	datos := tramo.Datos

	bytesCrudo, crcCrudo := len(datos), crc32.ChecksumIEEE(datos)

	dc, err := d2dc6.Load(datos)
	if err != nil {
		println("JUEGO125: HUD", nombre, "NO PARSEA:", err.Error())
		tramo.Liberar()

		return nil
	}

	if cuadro < 0 || cuadro >= len(dc.Frames) {
		println("JUEGO125: HUD", nombre, "SIN CUADRO", cuadro, "de", len(dc.Frames))
		tramo.Liberar()

		return nil
	}

	fr := dc.Frames[cuadro]

	// EL ARTE VA AL MONTON DE C (ver `bloqueDeC`): el semiespacio esta al 97 % cuando esto corre.
	crudo := dc.DecodeFrame(cuadro)
	indices := bloqueDeC(len(crudo))
	copy(indices, crudo)

	crudo = nil

	tramo.Liberar()

	p := &piezaDelHUD{
		Nombre: nombre, Ancho: int(fr.Width), Alto: int(fr.Height), Indices: indices,
		Pixeles: contarNoVacios(indices), CRC: crc32.ChecksumIEEE(indices),
	}

	println("JUEGO125: HUD PIEZA", nombre, "archivo=", ruta, "bytes=", bytesCrudo, "crc32Archivo=",
		int64(crcCrudo), "cuadros=", len(dc.Frames), "cuadro=", cuadro, "caja=", p.Ancho, p.Alto,
		"pixeles=", p.Pixeles, "crc32=", int64(p.CRC), "ms=", int64(d2cMs()-t0))

	// el archivo crudo NO se queda: los indices ya estan copiados en el bloque de C
	datos, dc = nil, nil

	return p
}

// paletaDelHUD: los 768 B de un `pal.dat` a la paleta RGBA del pincel. Se lee B,G,R como el motor
// (`d2dat.Load`: b=0, g=1, r=2); ver el comentario de `PaletaDelJugador` en jugador.go.
func paletaDelHUD(ruta, nombre string) [256][4]uint8 {
	var pal [256][4]uint8

	fuente := fuenteDe(rutaDatos)
	if fuente == nil {
		println("JUEGO125: HUD SIN PALETA", nombre, ruta)

		return pal
	}

	tramo, err := fuente.Pedir(ruta)
	if err != nil || tramo == nil || len(tramo.Datos) < 768 {
		println("JUEGO125: HUD SIN PALETA", nombre, ruta)

		return pal
	}

	datos := tramo.Datos

	defer tramo.Liberar()

	for i := 0; i < 256; i++ {
		pal[i][0], pal[i][1], pal[i][2], pal[i][3] = datos[i*3+2], datos[i*3+1], datos[i*3+0], 255
	}

	println("JUEGO125: HUD PALETA", nombre, "bytes=", len(datos), "crc32=", int64(crc32.ChecksumIEEE(datos)))

	datos = nil
	runtime.GC()

	return pal
}

// cargarLosGlifos: la tabla `.tbl` de la fuente 16 (3.596 B) y los cuadros del DC6 que usan las letras del
// nombre de la zona. La tabla entera de la fuente no se queda: quedan los glifos del texto (16 letras, ~3,6 KB).
func cargarLosGlifos(h *hudDelJuego) {
	t0 := d2cMs()

	fuente := fuenteDe(rutaDatos)
	if fuente == nil {
		println("JUEGO125: HUD SIN FUENTE DE DATOS para la fuente 16")

		return
	}

	tramoTabla, err := fuente.Pedir(rutaFuenteZona + ".tbl")
	if err != nil || tramoTabla == nil || len(tramoTabla.Datos) == 0 {
		println("JUEGO125: HUD SIN TABLA DE FUENTE", rutaFuenteZona+".tbl")

		return
	}

	tabla := tramoTabla.Datos

	defer tramoTabla.Liberar()

	f, err := d2font.Load(tabla)
	if err != nil {
		println("JUEGO125: HUD LA TABLA DE LA FUENTE NO PARSEA:", err.Error())

		return
	}

	tramoSprite, err := fuente.Pedir(rutaFuenteZona + ".DC6")
	if err != nil || tramoSprite == nil || len(tramoSprite.Datos) == 0 {
		println("JUEGO125: HUD SIN SPRITE DE FUENTE", rutaFuenteZona+".DC6")

		return
	}

	sprite := tramoSprite.Datos

	defer tramoSprite.Liberar()

	dc, err := d2dc6.Load(sprite)
	if err != nil {
		println("JUEGO125: HUD EL SPRITE DE LA FUENTE NO PARSEA:", err.Error())

		return
	}

	anchoTexto, _ := f.GetTextMetrics(h.Zona)

	for i := 0; i < len(h.Zona); i++ {
		c := h.Zona[i]
		if _, listo := h.Glifos[c]; listo {
			continue
		}

		g, ok := f.Glyphs[rune(c)]
		if !ok {
			println("JUEGO125: HUD LA FUENTE NO TIENE LA LETRA", string(c))

			continue
		}

		cuadro := g.FrameIndex()
		if cuadro < 0 || cuadro >= len(dc.Frames) {
			continue
		}

		fr := dc.Frames[cuadro]

		crudoGlifo := dc.DecodeFrame(cuadro)
		indices := bloqueDeC(len(crudoGlifo))
		copy(indices, crudoGlifo)
		crudoGlifo = nil

		h.Glifos[c] = &glifoDelHUD{Ancho: int(fr.Width), Alto: int(fr.Height), Paso: g.Width(),
			Indices: indices}
	}

	println("JUEGO125: HUD FUENTE tabla=", len(tabla), "sprite=", len(sprite), "cuadros=", len(dc.Frames),
		"glifosDeLaZona=", len(h.Glifos), "texto=", h.Zona, "anchoDelTexto=", anchoTexto,
		"ms=", int64(d2cMs()-t0))

	tabla, sprite, dc = nil, nil, nil
}

// ============================== LOS VALORES REALES ==============================

// valoresDelHeroeDeLaTabla: la clase del heroe del POC (la amazona) y su nivel de la tabla.
func valoresDelHeroeDeLaTabla() (string, int) {
	return claseDelHeroe, nivelDelHeroe
}

// valoresDelHeroeDeLaClase: la vida, el mana y el aguante de una clase a un nivel, con la tabla del motor.
//
// EL CAMINO ES EL DEL MOTOR: `charstats.txt` de la fila de la clase y `Class.BaseMax(nivel, vit, ene)`, que es
// exactamente lo que llama `d2hero/hero_totals.go` (life, mana, stam := derived.MaxLife, derived.MaxMana,
// derived.MaxStamina). Los atributos son los INICIALES de la clase (no se gasto ningun punto): eso es un heroe
// recien nacido a ese nivel.
func valoresDelHeroeDeLaClase(clase string, nivel int) valoresDelHeroe {
	v := valoresDelHeroe{Clase: clase, Nivel: nivel}

	fuente := fuenteDe(rutaParche)
	if fuente == nil {
		println("JUEGO125: HUD SIN FUENTE DEL PARCHE para charstats")

		return v
	}

	tramo, err := fuente.Pedir(rutaCharStats)
	if err != nil || tramo == nil || len(tramo.Datos) == 0 {
		println("JUEGO125: HUD SIN CHARSTATS", rutaCharStats)

		return v
	}

	datos := tramo.Datos

	defer tramo.Liberar()

	clases, err := d2statlist.ParseClasses(datos)
	if err != nil {
		println("JUEGO125: HUD CHARSTATS NO PARSEA:", err.Error())

		return v
	}

	c, ok := clases[clase]
	if !ok {
		println("JUEGO125: HUD LA CLASE NO ESTA EN CHARSTATS:", clase)

		return v
	}

	vida, mana, aguante := c.BaseMax(nivel, c.InitVit, c.InitEne)

	v.Vida, v.VidaMax = vida, vida
	v.Mana, v.ManaMax = mana, mana
	v.Aguante, v.AguanteMax = aguante, aguante
	v.Vitalidad, v.Energia, v.HpAdd = c.InitVit, c.InitEne, c.HpAdd

	datos = nil
	runtime.GC()

	return v
}

// ============================== EL DIBUJO ==============================

// dibujar: el HUD sobre el lienzo de un jugador. Devuelve los pixeles que pinto y los ms que costo.
//
// EL ORDEN ES EL DEL MOTOR (`hud.go`, renderPanelStatic): primero el PANEL (el fondo del globo), despues el
// GLOBO con su relleno, despues su TAPA, y al final el cinturon con sus pociones y el nombre de la zona. La tapa
// va despues del globo y no antes: es el vidrio con el brillo, que en el juego tapa el borde del relleno.
func (h *hudDelJuego) dibujar(sup *pvr.Surface, jugador, anchoLienzo, altoLienzo int) (int, int) {
	t0 := d2cMs()
	pintados := 0

	vida, vidaMax, mana, manaMax := h.valoresDe(jugador)

	// ---- EL GLOBO DE VIDA: el panel de la izquierda, el globo con su relleno y la tapa
	if h.Panel != nil {
		pintados += pegarPieza(sup, h.Panel, 0, altoLienzo, h.PalCielo[:])
	}

	rellenoVida := rellenoDelGlobo(vida, vidaMax)

	if h.Globo[0] != nil {
		pintados += pegarRellenoDeGlobo(sup, h.Globo[0], globoVidaDX, altoLienzo+globoVidaDY, rellenoVida,
			h.PalCielo[:])
	}

	if h.Tapa[0] != nil {
		pintados += pegarPieza(sup, h.Tapa[0], tapaVidaDX, altoLienzo+tapaVidaDY, h.PalCielo[:])
	}

	// ---- EL GLOBO DE MANA, con las posiciones del widget de la derecha
	rellenoMana := rellenoDelGlobo(mana, manaMax)

	if h.Globo[1] != nil {
		pintados += pegarRellenoDeGlobo(sup, h.Globo[1], anchoLienzo-manaWidgetDX+globoManaDX,
			altoLienzo+globoManaDY, rellenoMana, h.PalCielo[:])
	}

	if h.Tapa[1] != nil {
		pintados += pegarPieza(sup, h.Tapa[1], anchoLienzo-manaWidgetDX+tapaManaDX, altoLienzo+tapaManaDY,
			h.PalCielo[:])
	}

	// ---- EL CINTURON: centrado abajo, con sus cuatro pociones en las cajas de `belts.txt`
	if h.Cinturon != nil {
		x := (anchoLienzo - h.Cinturon.Ancho) / 2

		cinturon := pegarPieza(sup, h.Cinturon, x, altoLienzo, h.PalUni[:])
		pintados += cinturon

		for i := 0; i < cinturonColumnas && i < len(h.Pociones); i++ {
			if h.Pociones[i] == nil {
				continue
			}

			pintados += pegarPieza(sup, h.Pociones[i], x+cinturonDX+i*cinturonPasoX,
				altoLienzo-cinturonPasoY+cinturonDY, h.PalUni[:])
		}
	}

	// ---- EL NOMBRE DE LA ZONA: arriba y centrado, con la fuente del juego
	pintados += h.dibujarLaZona(sup, anchoLienzo)

	ms := int(d2cMs() - t0)

	// el instrumento: con `hudInstrumento` las barras bajan cuadro a cuadro (el POC no tiene dano al heroe)
	if hudInstrumento == "si" {
		h.bajarLasBarras(jugador)
	}

	h.pxPintados = pintados
	h.msDibujo = ms
	h.cuadrosConHUD++

	anchoTexto := 0
	if h.Glifos != nil {
		for i := 0; i < len(h.Zona); i++ {
			if g := h.Glifos[h.Zona[i]]; g != nil {
				anchoTexto += g.Paso
			}
		}
	}

	// DC-002.134: la linea del HUD es evidencia por cuadro (~200 B por el puerto serie): con `VERBOSO=no` se calla.
	if verboso134() {
		println("JUEGO125: HUD cuadro=", cuadro, "jugador=", jugador+1, "lienzo=", anchoLienzo, "x", altoLienzo,
			"pixelesPintados=", pintados, "ms=", ms,
			"vida=", vida, "/", vidaMax, "rellenoVida=", rellenoVida, "de", altoDelGlobo,
			"mana=", mana, "/", manaMax, "rellenoMana=", rellenoMana,
			"zona=", h.Zona, "anchoDelTexto=", anchoTexto, "pociones=", h.pocionesPintadas())
	}

	return pintados, ms
}

// pocionesPintadas: cuantas de las cuatro cajas del cinturon tienen su pocion.
func (h *hudDelJuego) pocionesPintadas() int {
	n := 0

	for _, p := range h.Pociones {
		if p != nil {
			n++
		}
	}

	return n
}

// valoresDe: la vida y el mana que muestra el jugador i AHORA (con el instrumento, menos que el maximo).
func (h *hudDelJuego) valoresDe(jugador int) (vida, vidaMax, mana, manaMax int) {
	if jugador < 0 || jugador >= len(valoresPorJugador) {
		return 0, 1, 0, 1
	}

	v := valoresPorJugador[jugador]

	vidaMax, manaMax = v.VidaMax, v.ManaMax
	vida, mana = vidaMax, manaMax

	if jugador < len(vidaActual) {
		vida = vidaActual[jugador]
	}

	if jugador < len(manaActual) {
		mana = manaActual[jugador]
	}

	if vidaMax < 1 {
		vidaMax = 1
	}

	if manaMax < 1 {
		manaMax = 1
	}

	return vida, vidaMax, mana, manaMax
}

// bajarLasBarras: EL INSTRUMENTO. Sin dano al heroe las barras no se moverian y no habria nada que medir; aca
// bajan un punto por cuadro hasta 1. La vida y el mana MAXIMOS no se tocan: son los de la tabla.
func (h *hudDelJuego) bajarLasBarras(jugador int) {
	if jugador < len(vidaActual) && vidaActual[jugador] > 1 {
		vidaActual[jugador]--
	}

	if jugador < len(manaActual) && manaActual[jugador] > 1 {
		manaActual[jugador]--
	}
}

// rellenoDelGlobo: las filas del globo que se pintan, con la regla del motor (`globeWidget.Render`):
// barHeight = valuePercent * globeHeight, y se pinta la parte de abajo del globo.
func rellenoDelGlobo(valor, maximo int) int {
	if maximo <= 0 || valor <= 0 {
		return 0
	}

	if valor > maximo {
		valor = maximo
	}

	relleno := valor * altoDelGlobo / maximo

	if relleno > altoDelGlobo {
		relleno = altoDelGlobo
	}

	return relleno
}

// pegarPieza: pega una pieza con su esquina de ABAJO a la izquierda en (x, yAbajo).
func pegarPieza(sup *pvr.Surface, p *piezaDelHUD, x, yAbajo int, pal [][4]uint8) int {
	if p == nil || p.Ancho <= 0 || p.Alto <= 0 || len(p.Indices) < p.Ancho*p.Alto {
		return 0
	}

	sup.PushTranslation(x, yAbajo-p.Alto)
	sup.RenderIndicesTintado(p.Indices, p.Ancho, p.Alto, pal)
	sup.Pop()

	return p.Pixeles
}

// pegarRellenoDeGlobo: pinta SOLO las `relleno` filas de abajo del globo, que es el recorte del motor
// (`RenderSection(target, maskRect)` con maskRect = (0, alto-relleno)-(ancho, alto)).
func pegarRellenoDeGlobo(sup *pvr.Surface, p *piezaDelHUD, x, yAbajo, relleno int, pal [][4]uint8) int {
	if p == nil || p.Ancho <= 0 || p.Alto <= 0 || relleno <= 0 {
		return 0
	}

	if relleno > p.Alto {
		relleno = p.Alto
	}

	desde := (p.Alto - relleno) * p.Ancho
	rebanada := p.Indices[desde : p.Ancho*p.Alto]

	sup.PushTranslation(x, yAbajo-relleno)
	sup.RenderIndicesTintado(rebanada, p.Ancho, relleno, pal)
	sup.Pop()

	// los pixeles que se pintaron de verdad: los no vacios de la rebanada
	return contarNoVacios(rebanada)
}

// dibujarLaZona: el nombre de la zona centrado arriba, letra por letra con la fuente del juego.
func (h *hudDelJuego) dibujarLaZona(sup *pvr.Surface, anchoLienzo int) int {
	if len(h.Glifos) == 0 {
		return 0
	}

	anchoTexto := 0

	for i := 0; i < len(h.Zona); i++ {
		if g := h.Glifos[h.Zona[i]]; g != nil {
			anchoTexto += g.Paso
		}
	}

	x := (anchoLienzo - anchoTexto) / 2
	pintados := 0

	for i := 0; i < len(h.Zona); i++ {
		g := h.Glifos[h.Zona[i]]
		if g == nil {
			continue
		}

		sup.PushTranslation(x, zonaArriba)
		sup.RenderIndicesTintado(g.Indices, g.Ancho, g.Alto, h.PalUni[:])
		sup.Pop()

		x += g.Paso
		pintados += contarNoVacios(g.Indices)
	}

	return pintados
}

// contarNoVacios: cuantos indices de una pieza NO son el 0 (el transparente). Es el instrumento de "esto pinto
// algo" de cada pieza del HUD, el mismo numero que se informa en la carga y en el cuadro.
func contarNoVacios(pix []byte) int {
	c := 0

	for _, v := range pix {
		if v != 0 {
			c++
		}
	}

	return c
}

// resumenDelHUD: los numeros del criterio de la tarjeta.
func resumenDelHUD() {
	if elHUD == nil {
		println("JUEGO125: HUD RESUMEN apagado (hud=", hudKnob, ")")

		return
	}

	conSplit := estaElSplitActivo()

	println("JUEGO125: HUD RESUMEN piezas=", elHUD.piezas(), "palancaPiezas=", hudPiezas, "glifos=", len(elHUD.Glifos),
		"bytesEnRAM=", elHUD.Bytes, "zona=", elHUD.Zona, "clase=", elHUD.Clase,
		"cuadrosConHUD=", elHUD.cuadrosConHUD, "ultimoPixelesPintados=", elHUD.pxPintados,
		"ultimoMs=", elHUD.msDibujo, "conSplit=", conSplit, "jugadores=", len(valoresPorJugador))

	for i := range valoresPorJugador {
		v := valoresPorJugador[i]

		println("JUEGO125: HUD JUGADOR", i+1, "clase=", v.Clase, "nivel=", v.Nivel,
			"vidaMax=", v.VidaMax, "manaMax=", v.ManaMax, "aguanteMax=", v.AguanteMax)
	}

	cumple := elHUD.cuadrosConHUD > 0 && elHUD.pxPintados > 0 && len(valoresPorJugador) > 0 && elHUD.Zona != ""

	println("JUEGO125: HUD CRITERIO elHUDSeDibujo=", elHUD.cuadrosConHUD > 0,
		"pintoPixeles=", elHUD.pxPintados > 0,
		"lasBarrasSonLosValoresReales=", valoresPorJugador[0].VidaMax > 0,
		"elCinturonTieneCuatro=", elHUD.pocionesPintadas() == cinturonColumnas,
		"laZonaTieneNombre=", elHUD.Zona != "", "cadaMitadSuHUD=", conSplit || jugadoresKnob == 1,
		"cumple=", cumple)
}

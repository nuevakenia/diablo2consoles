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
	"fmt"
	"image/color"
	"io"
	"runtime"
	"unsafe"

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

// ============================== LOS PUENTES DEL BLINDAJE DEL REARME (DC-002.146) ==============================

// d2cGcFrena / d2cGcSuelta: el freno del recolector (`gc_inhibit_collection` / `gc_allow_collection` de
// libgodc, las mismas que el runtime se pone en sus secciones criticas). Ver `csrc/shim.c` y la cabecera de
// `recargarElMinimapaDelNivel`.
//
//extern d2c_gc_frena
func d2cGcFrena()

//extern d2c_gc_suelta
func d2cGcSuelta()

// OJO CON EL `recover` DE LIBGODC (medido en esta tarjeta, ver la cabecera de `rearmarElMinimapaDelNivel`): su
// `runtime_checkpoint()` hace `setjmp` DENTRO de una funcion de C que despues RETORNA, asi que el `longjmp` del
// `recover` cae en un marco de pila muerto y el SH-4 muere con `kernel panic: unhandled IRQ/Exception`. Por eso aca
// NO hay checkpoint: el `defer` con `recover` solo deja el AVISO (el paso y el estado del minimapa) y la consola
// muere igual. El blindaje de verdad es no llegar al panic (ver el freno y la lectura sin memoria).

// celdaDelMinimapa: un cuadro de arte ya decodificado y DECIMADO a la caja del minimapa (8x16).
type celdaDelMinimapa struct {
	Cel     int
	Ancho   int
	Alto    int
	Indices []byte // en el monton de C: no lo toca el recolector
	Pixeles int    // los indices != 0

	// DC-002.137: los MISMOS pixeles en tercetos (x, y, indice), solo los que pintan, en el monton de C. Es lo que
	// recorre el cuadro: el arte del automapa es ralo y de 128 indices hay ~8 que pintan.
	Compacto []byte
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

	// Nivel es el id de LvlTypes del nivel que el minimapa esta dibujando (DC-002.144c). Nace en el pueblo
	// (`nivelDelPueblo`) y CAMBIA con `recargarElMinimapaDelNivel`: antes era la constante de paquete, y por eso
	// al cruzar al afuera el mapa seguia sumando celdas del pueblo sobre el nivel nuevo (680 + 21 = 701).
	Nivel int

	// lo que ocupa, medido
	BytesArte     int
	BytesCompacto int
	BytesTabla    int
	CuadrosArte   int
	SinArte       int // celdas del modelo que caen en el dibujo y NO tienen cuadro decodificado

	// el reveal
	ultimaX, ultimaY float64
	hayUltima        bool
	revelados        int // cuantas veces se revelo
	reveladasUltimas int // celdas de las ultimas casillas reveladas
	reveladasMax     int

	// DC-002.146b: la siembra (las celdas que salieron de la fila mas cercana, no de una fila exacta)
	sembradasUltimo int
	sembradasTotal  int

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
		Nivel:  nivelDelPueblo,
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

	tabla, bytesTabla, err := leerLaTablaDelNivel(fuente, m.Nivel)
	if err != nil {
		println("JUEGO130: MINIMAPA SIN TABLA ruta=", rutaTablaAutomap, "motivo=", err.Error())

		return
	}

	m.Tabla = tabla
	m.BytesTabla = bytesTabla

	println("JUEGO130: MINIMAPA TABLA archivo=", rutaTablaAutomap, "bytes=", bytesTabla,
		"filasDelPueblo=", len(tabla.LevelRows(m.Nivel)), "nivel=", m.Nivel,
		"nombre=", d2automap.LevelName(m.Nivel), "porElFlujo= si")

	// ---- 2. EL ARTE: los cuadros que piden las filas del pueblo, decodificados y decimados a 8x16
	faltan := m.cargarElArte(fuente)

	println("JUEGO130: MINIMAPA ARTE archivo=", rutaArteAutomap, "cuadros=", m.CuadrosArte,
		"bytesEnMontonDeC=", m.BytesArte, "faltan=", faltan, "montonC_libre=", d2cMontonLibre())

	// ---- 3. EL MODELO: vacio (el reveal lo llena cuadro a cuadro, siguiendo al jugador) o el pueblo entero
	if minimapaTodo == "si" {
		m.revelarTodoElNivel()
	}

	elMinimapa = m

	println("JUEGO130: MINIMAPA LISTO celdas=", m.Modelo.Count(), "piso=", m.Modelo.CountLayer(d2automap.LayerFloor),
		"pared=", m.Modelo.CountLayer(d2automap.LayerWall), "palanca=", minimapaKnob, "cuando=", minimapaCuando,
		"todo=", minimapaTodo, "ms=", int(d2cMs()-t0))

	semiespacio("despues-del-minimapa")
	memoria("minimapa-despues")
}

// leerLaTablaDelNivel: lee `AutoMap.txt` y devuelve SOLO las filas del nivel `id` (LvlTypes), con los bytes del
// archivo. Se usa en la carga y en la RECARGA al cambiar de nivel (DC-002.144c): el `.txt` se puede volver a leer
// porque con `demoMoor == "si"` las fuentes de consola siguen abiertas (son la guarda del cruce).
//
// DC-002.146 -- LA LECTURA NO PIDE MEMORIA AL SEMIESPACIO, Y EL RECOLECTOR NO CORRE ADENTRO.
//
// EL DEFECTO QUE CIERRA (medido, no supuesto). Hasta DC-002.145 esto leia por `fuente.Open`, el flujo del MPQ, que
// es el UNICO camino de la fuente que pide memoria POR SECTOR (`mpq_stream.go`: un `make` del bloque comprimido,
// otro del descomprimido y la zlib adentro). El rearme llega con el semiespacio al 73-83 % y la relectura son ~30
// sectores: el recolector salta por umbral EN MEDIO DEL PARSEO. Medido en la corrida `/tmp/capturas-146b` (16 x 16):
// entre `DEMO140: REARME paso=1` y la linea del rearme hay CUATRO recolecciones (174..177, tres por umbral de bytes
// usados). En esa ventana el barrido conservador de la pila de libgodc (`gc_copy.c`) deja un puntero sin actualizar y
// la siguiente rebanada que se toca muere con `slice bounds out of range` -- o el PC salta a una direccion de monton,
// el defecto que DC-002.78 ya midio y freno con `gc_inhibit_collection` (`csrc/shim.c`, `d2c_gc_frena`). Por eso el
// panic NO seguia al contenido del `.txt` ni a la entrada de la funcion: seguia al LAYOUT del monton (con tres
// `println` de traza adentro desaparecia, y con el build de 32 x 32 no salio en tres cruces).
//
// AHORA. El `.txt` (122.022 B) se lee en un bloque del MONTON DE C -- que el recolector no cuenta, no barre y no
// copia -- con la memoria de trabajo PRESTADA de la fuente (la misma de todas las cargas: cero reservas por sector,
// `d2mpq.ReadFileInto`), y se parsea de un `bytes.Reader` sobre ese bloque. Lo unico que pide al recolector son las
// filas del nivel (142 filas, ~6 KB). El camino viejo queda como RESPALDO y el log dice cual corrio
// (`MINIMAPA_DIAG=flujo` lo fuerza: es el A/B en el mismo binario).
func leerLaTablaDelNivel(fuente *fuenteconsola.Fuente, id int) (*d2automap.Table, int, error) {
	tamano, hay := fuente.Tamano(rutaTablaAutomap)
	if !hay || tamano == 0 {
		return nil, 0, fmt.Errorf("no hay %s (%d B)", rutaTablaAutomap, tamano)
	}

	t0 := d2cMs()
	coleccionesAntes := int(d2cGcColecciones())
	semiespacioAntes := int(d2cGcUsado())

	// ---- 1. EL CAMINO SIN MEMORIA (el de esta tarjeta)
	if !laTablaSeLeePorElFlujo() {
		tabla, leidos, err := leerLaTablaDeUnBloqueDeC(fuente, id, tamano)
		if err == nil {
			println("JUEGO130: MINIMAPA TABLA-LECTURA camino= montonDeC+prestamo bytes=", leidos,
				"filasDelNivel=", len(tabla.LevelRows(id)),
				"recolecciones=", coleccionesAntes, "->", int(d2cGcColecciones()),
				"semiespacio=", semiespacioAntes, "->", int(d2cGcUsado()), "ms=", int(d2cMs()-t0))

			return tabla, tamano, nil
		}

		println("JUEGO130: MINIMAPA TABLA-LECTURA el camino sin memoria fallo:", err.Error(),
			": se lee por el flujo del MPQ (el camino viejo)")
	}

	// ---- 2. EL CAMINO VIEJO: el flujo del MPQ, sector por sector (el respaldo y el A/B)
	flujo, err := fuente.Open(rutaTablaAutomap)
	if err != nil {
		return nil, 0, err
	}

	tabla, err := d2automap.ParseTxtLevels(flujo, id)
	if err != nil {
		return nil, tamano, err
	}

	println("JUEGO130: MINIMAPA TABLA-LECTURA camino= flujoDelMPQ bytes=", tamano,
		"filasDelNivel=", len(tabla.LevelRows(id)),
		"recolecciones=", coleccionesAntes, "->", int(d2cGcColecciones()),
		"semiespacio=", semiespacioAntes, "->", int(d2cGcUsado()), "ms=", int(d2cMs()-t0))

	return tabla, tamano, nil
}

// leerLaTablaDeUnBloqueDeC: lee `AutoMap.txt` ENTERO en un bloque del monton de C, con la memoria de trabajo
// prestada de la fuente, y lo parsea de ahi. Ver la cabecera de `leerLaTablaDelNivel`.
//
// POR QUE EL BLOQUE ES DEL MONTON DE C. `bloqueDeC` (hud_125.go) es la misma puerta del HUD y del arte del minimapa:
// 122 KB en el semiespacio, con el semiespacio al 83 %, son una recoleccion completa por cada pedido (medido en
// DC-002.125); en el monton de C hay 2 MB libres y el recolector ni lo ve. El bloque se DEVUELVE al final.
//
// POR QUE SE FILTRA ANTES DEL PARSER (medido). El parser del motor lee las 2.603 lineas del archivo para quedarse
// con las del nivel: `strings.TrimRight` + `strings.Split` + `strings.TrimSpace` por linea son ~420 B de basura por
// linea, o sea ~1,1 MB por relectura. Con el semiespacio al 83 % eso es una recoleccion cada pocos cientos de bytes
// -- las cuatro que se miden adentro del rearme -- y con el recolector frenado es un `fatal error: out of memory`
// (medido en /tmp/capturas-146g: el parseo no entra en los 340 KB que quedaban). Asi que el filtro por nivel se
// hace ANTES, sobre los bytes del bloque y sin pedir memoria (`filtroDeUnNivel`): al parser le llegan la cabecera
// y las 142 lineas del nivel, y la basura baja a ~60 KB. Las filas son LAS MISMAS (mismo parser, mismos bytes).
//
// Y SE VERIFICA ANTES DE USARLO: que se hayan leido los bytes que declara el MPQ y que el nivel tenga filas. Una
// tabla a medias es peor que no rearmar el minimapa.
func leerLaTablaDeUnBloqueDeC(fuente *fuenteconsola.Fuente, id, tamano int) (*d2automap.Table, int, error) {
	p := d2cPedirBloque(uint(tamano))
	if p == nil {
		return nil, 0, fmt.Errorf("el monton de C no dio %d B", tamano)
	}

	buf := (*[1 << 24]byte)(p)[:tamano:tamano]

	trabajo := fuente.Prestamo()
	crecidasAntes := trabajo.Crecidas()

	leidos, err := fuente.LeerEnteroEnConPrestamo(rutaTablaAutomap, buf, trabajo)
	if err != nil {
		d2cLibre(uintptr(p))

		return nil, leidos, err
	}

	if leidos != tamano {
		d2cLibre(uintptr(p))

		return nil, leidos, fmt.Errorf("se leyeron %d de %d B", leidos, tamano)
	}

	filtro := nuevoFiltroDeUnNivel(buf, id)

	tabla, err := d2automap.ParseTxtLevels(filtro, id)

	println("JUEGO130: MINIMAPA TABLA-LECTURA prestamo= recargasDelPrestamo=", crecidasAntes, "->", trabajo.Crecidas(),
		"lineasDelNivel=", filtro.entregadas, "lineasLeidas=", filtro.lineas, "libreDelMontonDeC=", d2cMontonLibre())

	// el bloque se devuelve DESPUES de parsear: las filas ya son copias (el scanner y `strings.Split` copian)
	d2cLibre(uintptr(p))

	if err != nil {
		return nil, leidos, err
	}

	if len(tabla.LevelRows(id)) == 0 {
		return nil, leidos, fmt.Errorf("el nivel %d no tiene filas en %s", id, rutaTablaAutomap)
	}

	return tabla, leidos, nil
}

// ============================== EL FILTRO QUE NO PIDE MEMORIA (DC-002.146) ==============================

// filtroDeUnNivel: un `io.Reader` que entrega, del `.txt` que ya esta en el bloque del monton de C, SOLO la
// cabecera y las lineas cuyo primer campo (LevelName) es el del nivel pedido. Es lo que hace que el parseo del
// motor no deje ~1,1 MB de basura por las 2.603 lineas que se descartan (ver `leerLaTablaDeUnBloqueDeC`).
//
// LA COMPARACION ES LA DEL PARSER: el campo se recorta (espacios, tabulador, CR) y se compara sin distinguir
// mayusculas contra `d2automap.LevelName(id)`, que es el MISMO nombre que `LevelIndex` mapea al id. Las lineas
// que se entregan son las mismas que el parser guardaria con `keep`, asi que las filas salen iguales.
type filtroDeUnNivel struct {
	datos  []byte // el archivo entero, en el bloque del monton de C
	nombre []byte // el LevelName del nivel pedido
	cabeza bool   // la cabecera todavia no se entrego
	linea  []byte // el tramo que se esta entregando
	leido  int    // lo ya entregado de `linea`
	desde  int    // desde donde se busca la proxima linea en `datos`

	// el instrumento: cuantas lineas se leyeron del archivo y cuantas se entregaron (el filtro, medido)
	lineas     int
	entregadas int
}

func nuevoFiltroDeUnNivel(datos []byte, id int) *filtroDeUnNivel {
	return &filtroDeUnNivel{datos: datos, nombre: []byte(d2automap.LevelName(id))}
}

// Read cumple `io.Reader`: entrega los tramos elegidos, de a lo que pida el scanner.
func (f *filtroDeUnNivel) Read(p []byte) (int, error) {
	total := 0

	for total < len(p) {
		if f.leido >= len(f.linea) && !f.buscarLaProxima() {
			if total > 0 {
				return total, nil
			}

			return 0, io.EOF
		}

		n := copy(p[total:], f.linea[f.leido:])

		total += n
		f.leido += n
	}

	return total, nil
}

// buscarLaProxima: deja en `linea` la proxima linea que se entrega (con su salto incluido).
func (f *filtroDeUnNivel) buscarLaProxima() bool {
	for f.desde < len(f.datos) {
		ini := f.desde
		fin := ini

		for fin < len(f.datos) && f.datos[fin] != '\n' {
			fin++
		}

		if fin < len(f.datos) {
			fin++ // el salto de linea va adentro
		}

		f.desde = fin
		f.linea = f.datos[ini:fin]
		f.leido = 0
		f.lineas++

		if !f.cabeza {
			f.cabeza = true
			f.entregadas++

			return true // la cabecera siempre va: de ahi salen las columnas
		}

		if primerCampoEsElNivel(f.linea, f.nombre) {
			f.entregadas++

			return true
		}
	}

	return false
}

// primerCampoEsElNivel: si el primer campo de la linea (hasta el tabulador), recortado de espacios y sin
// distinguir mayusculas, es `nombre`.
func primerCampoEsElNivel(linea, nombre []byte) bool {
	fin := 0

	for fin < len(linea) && linea[fin] != '	' && linea[fin] != '\n' && linea[fin] != '\r' {
		fin++
	}

	campo := recortarEspacios(linea[:fin])
	if len(campo) != len(nombre) {
		return false
	}

	for i := range nombre {
		a, b := campo[i], nombre[i]
		if a >= 'A' && a <= 'Z' {
			a += 'a' - 'A'
		}

		if b >= 'A' && b <= 'Z' {
			b += 'a' - 'A'
		}

		if a != b {
			return false
		}
	}

	return true
}

// recortarEspacios: el `strings.TrimSpace` del parser, pero sobre bytes y sin copiar.
func recortarEspacios(b []byte) []byte {
	for len(b) > 0 && (b[0] == ' ' || b[0] == '	' || b[0] == '\r') {
		b = b[1:]
	}

	for len(b) > 0 && (b[len(b)-1] == ' ' || b[len(b)-1] == '	' || b[len(b)-1] == '\r') {
		b = b[:len(b)-1]
	}

	return b
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

	for _, fila := range m.Tabla.LevelRows(m.Nivel) {
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

		// DC-002.137: la lista COMPACTA de los pixeles que pintan, en tercetos (x, y, indice). Se arma UNA vez aca,
		// con el arte, y el cuadro la recorre en vez de los 128 indices del tile (el del automapa es ralo: ~8 de 128).
		// Va al monton de C, como el arte: el recolector no la cuenta ni la copia.
		compacto := m.compactarElArte(indices)

		m.Arte[c] = &celdaDelMinimapa{
			Cel: c, Ancho: anchoCeldaMini, Alto: altoCeldaMini, Indices: indices, Compacto: compacto,
			Pixeles: contarNoVacios(indices),
		}

		m.BytesArte += len(indices) + len(compacto)
		m.BytesCompacto += len(compacto)
		m.CuadrosArte++
	}

	println("JUEGO130: MINIMAPA ARTE decodificados=", m.CuadrosArte, "faltan=", faltan, "bytes=", m.BytesArte,
		"punterosLeidos=", len(punteros), "nombreQuePideElMotor=", rutaNombre8x16, "existe=false (medido)")

	return faltan
}

// revelarTodoElNivel: el caso peor (lo que mide la tarjeta). Se revela el nivel entero de una: el area es el mapa
// completo, que es lo mismo que revelarlo caminando pero sin caminar. Sirve para el pueblo y para el afuera
// (DC-002.144c: el nivel sale de `m.Nivel`, no de una constante).
func (m *minimapaDelJuego) revelarTodoElNivel() {
	w, h := fuenteDelMotor{}.Size()
	if w == 0 || h == 0 {
		println("JUEGO130: MINIMAPA sin motor del mapa: no se puede revelar el nivel entero")

		return
	}

	agregadas, sembradas := m.revelarConElModelo(float64(w/2), float64(h/2),
		d2automap.Area{HalfW: w * 8, HalfH: h * 4})

	conCelda, de := m.coberturaDelModelo()

	println("JUEGO130: MINIMAPA REVEAL-TODO nivel=", m.Nivel, d2automap.LevelName(m.Nivel), "casillas=", w, "x", h,
		"agregadas=", agregadas, "celdas=", m.Modelo.Count(), "sembradas=", sembradas, "siembra=", minimapaSiembra,
		"casillasConCelda=", conCelda, "de", de, "pct=", conCelda*100/maxDe(de),
		"piso=", m.Modelo.CountLayer(d2automap.LayerFloor), "pared=", m.Modelo.CountLayer(d2automap.LayerWall))
}

// maxDe: el denominador del porcentaje, sin partir por cero.
func maxDe(v int) int {
	if v <= 0 {
		return 1
	}

	return v
}

// semillaDelAutomap: la del motor (`Automap.Advance`: uint32(nivel+1) * automapSeedMul). El multiplicador va en una
// VARIABLE y no en una constante: `uint32(2) * 2654435761` no entra en uint32 como constante y gccgo lo rechaza (la
// cuenta del motor la hace en 32 bits, que es lo que hace la variable).
var multiplicadorDeLaSemilla uint32 = 2654435761

func semillaDelAutomap(nivel int) uint32 {
	return uint32(nivel+1) * multiplicadorDeLaSemilla
}

// ============================== LA SIEMBRA CON LA FILA MAS CERCANA (DC-002.146b) ==============================
//
// EL DEFECTO QUE CIERRA. El modelo se llenaba solo donde las filas de `AutoMap.txt` del nivel matcheaban
// `(tile, estilo, secuencia)`, y los tiles que pone el DRLG en el afuera casi no caen en ellas: medido en el host
// (`console/dc/medidas/automap-drlg`), en una region de 32 x 32 del Blood Moor los pisos son
// `act1/town/floor.dt1` estilo 0 (482 registros) y estilo 5 (273) con secuencia 0, y las filas del "1 Wilderness"
// cubren el estilo 0 para las secuencias 1..47 y no tienen ni una fila del estilo 5. Con los indices de tile
// correctos el modelo quedaba en 204 celdas de 1.024 casillas (19,9 %), y con la region de 16 x 16 en CERO pisos.
//
// LA REGLA (`d2automap.Table.CelCercana`, que es la que corre aca). Un tile sin fila toma la celda de la fila MAS
// CERCANA de su tipo: la del mismo estilo con la secuencia mas proxima, o si el estilo no tiene ninguna fila, la del
// estilo mas proximo. Un tipo sin filas (los arboles, tipo 14) NO se siembra: no se inventa un grafico. El camino
// del motor (`RevealTiles`) queda intacto: `MINIMAPA_SIEMBRA=no` es el A/B, y con el pueblo --cuyos tiles estan
// todos en la tabla-- los dos caminos dan las mismas celdas.
//
// LOS NUMEROS DE LOS DOS CAMINOS (medidos en el host, `automap-drlg`, con el indice de tile ya arreglado):
//
//	region 32 x 32 (24,32):  sin siembra 204 celdas (111 piso / 93 pared)  19,9 % de las casillas
//	                         con siembra 1.117 celdas (1.024 piso / 93 pared)  100 % de las casillas
//	region 16 x 16 (0,48):   sin siembra  29 celdas (0 piso / 29 pared)   11,3 % de las casillas
//	                         con siembra  221 celdas (192 piso / 29 pared)  75 % de las casillas (100 % de las
//	                         que el DRLG poblo: la region tiene 192 casillas con tile de 256)
func sembrandoElAutomapa() bool {
	return minimapaSiembra != "no"
}

// revelarConElModelo: el revelado del nivel `id` alrededor de `hx, hy` con el area dada, con la siembra o sin
// ella. Devuelve las celdas agregadas y cuantas vinieron de la siembra.
func (m *minimapaDelJuego) revelarConElModelo(hx, hy float64, area d2automap.Area) (int, int) {
	semilla := semillaDelAutomap(m.Nivel)
	fuente := fuenteDelMotor{}

	var agregadas, sembradas int

	if sembrandoElAutomapa() {
		agregadas, sembradas = m.Modelo.RevealTilesConRespaldo(m.Tabla, fuente, m.Nivel, semilla, hx, hy, area)
	} else {
		agregadas = m.Modelo.RevealTiles(m.Tabla, fuente, m.Nivel, semilla, hx, hy, area)
	}

	m.sembradasUltimo, m.sembradasTotal = sembradas, m.sembradasTotal+sembradas

	return agregadas, sembradas
}

// coberturaDelModelo: cuantas casillas del mapa compuesto tienen alguna celda en el modelo (`de`), que es la
// pregunta del criterio: "casillas de la region con celda en el modelo". Se cuenta con `Model.HasAt` (tres
// busquedas de mapa por casilla, sin reservar nada) sobre las casillas del mapa que el POC tiene compuesto.
func (m *minimapaDelJuego) coberturaDelModelo() (conCelda, de int) {
	w, h := fuenteDelMotor{}.Size()

	for ty := 0; ty < h; ty++ {
		for tx := 0; tx < w; tx++ {
			de++

			if m.Modelo.HasAt(d2automap.TileCell(tx, ty)) {
				conCelda++
			}
		}
	}

	return conCelda, de
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

	agregadas, sembradas := m.revelarConElModelo(hx, hy, d2automap.ScreenArea(ancho, alto))

	m.revelados++
	m.reveladasUltimas = m.Modelo.Count() - antes

	if m.reveladasUltimas > m.reveladasMax {
		m.reveladasMax = m.reveladasUltimas
	}

	println("JUEGO130: MINIMAPA REVEAL cuadro=", cuadro, "jugador=", jugador+1, "heroe=", hx, hy,
		"agregadas=", agregadas, "sembradas=", sembradas, "celdas=", m.Modelo.Count(),
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

// ============================== EL CAMBIO DE NIVEL (DC-002.144c) ==============================

// soltarElArteDelNivel: devuelve al MONTON DE C el arte del nivel que se deja y vacia la tabla de cuadros.
//
// QUE BLOQUES SON. Cada cuadro decodificado vive en un `d2c_pedir_bloque` (un `malloc` del monton de C, 128 B de
// indices + la lista compacta): NO son del recolector y `d2c_soltar_bloque` no los toca (su guarda devuelve 0 para
// todo lo que no pase el umbral de objeto grande, 64 KB). El `free` que corresponde es el pelado (`d2cLibre`, el
// mismo que usa el liberador de los tramos del MPQ), y es seguro porque el recolector de libgodc no sigue punteros
// que caen fuera del semiespacio.
//
// Se devuelve el mapa NUEVO de una y no se borra cuadro por cuadro: borrar durante el recorrido es una escritura de
// mapa (`runtime.mapdelete`), y en este POC la primera escritura de mapa de la corrida ya mato a la consola una vez
// con `Illegal instruction` (ver la cabecera de cargarElMinimapa). Un mapa vacio nuevo no toca el viejo.
func (m *minimapaDelJuego) soltarElArteDelNivel() (bytes, cuadros int) {
	viejo := m.Arte

	for _, arte := range viejo {
		if arte == nil {
			continue
		}

		if n := len(arte.Indices); n > 0 {
			d2cLibre(uintptr(unsafe.Pointer(&arte.Indices[0])))
			bytes += n
		}

		if n := len(arte.Compacto); n > 0 {
			d2cLibre(uintptr(unsafe.Pointer(&arte.Compacto[0])))
			bytes += n
		}

		cuadros++
	}

	m.Arte = map[int]*celdaDelMinimapa{}

	return bytes, cuadros
}

// ============================== LAS DOS PREGUNTAS DE LA PALANCA (DC-002.146) ==============================

// elRearmeVaFrenado: si el rearme corre con el recolector FRENADO (el camino del juego). Las tres corridas de
// medir de esta tarjeta lo apagan a proposito: `flujo` y `carga` (el camino viejo de la lectura: es el A/B contra
// el defecto) y `sololectura` (la lectura nueva SIN el freno: mide cuanto pide la lectura sola).
func elRearmeVaFrenado() bool {
	switch minimapaDiag {
	case "flujo", "carga", "sololectura":
		return false
	}

	return true
}

// laTablaSeLeePorElFlujo: si la relectura de `AutoMap.txt` va por el flujo del MPQ (el camino viejo, el que pide
// memoria por sector y hace saltar el recolector en medio del parseo) en vez de por el bloque del monton de C.
func laTablaSeLeePorElFlujo() bool {
	return minimapaDiag == "flujo" || minimapaDiag == "carga"
}

// hayLugarParaElFreno: si el semiespacio tiene lugar para lo que el rearme pide CON EL RECOLECTOR FRENADO.
//
// EL NUMERO ES MEDIDO, NO ELEGIDO (y sono tres veces). Con el recolector frenado nada se recicla hasta el
// `runtime.GC()` del final del rearme, asi que TODO lo que el rearme pide tiene que entrar en lo que queda:
//   - el parseo ya filtrado (143 lineas de 2.604): ~78 KB (medido: semiespacio +78 KB en la lectura),
//   - el modelo del nivel, el arte y el revelado: entre 197 y 267 KB (medido: con 267 KB libres despues del parseo
//     el rearme de 16 x 16 paso -- /tmp/capturas-146k -- y con 197 KB el de 32 x 32 murio con `fatal error: out of
//     memory` -- /tmp/capturas-146q).
//
// O sea el pico ronda los 330 KB. Se piden 400 KB y si no estan NO SE FRENA: el rearme va con el recolector suelto
// (que es como corria antes de esta tarjeta) y el aviso queda en el log. En la corrida de 32 x 32 no se frena: su
// nivel deja 1,56 MB vivos de 1,835 MB, asi que despues de recolectar solo quedan ~275 KB.
func hayLugarParaElFreno() bool {
	total := int(d2cGcTotal())
	if total <= 0 {
		return false
	}

	return total-int(d2cGcUsado()) >= 320<<10
}

// recargarElMinimapaDelNivel: REARMA el minimapa para OTRO nivel (DC-002.144c).
//
// EL DEFECTO QUE CIERRA. Hasta DC-002.144b el nivel del minimapa era la constante `nivelDelPueblo`: al cruzar al
// afuera la tabla, el arte y la semilla seguian siendo los del pueblo y el modelo solo SUMABA las celdas nuevas
// encima (`cuadro= 112 agregadas= 21 celdas= 701`: 680 del pueblo + 21 del afuera). El minimapa dibujaba los dos
// niveles mezclados. Aca se hace lo que hace el motor cuando cambia de nivel: se sueltan el arte y el modelo del
// que se deja, se vuelve a leer la tabla para el que llega (`AutoMap.txt` por el flujo, el `.txt` se puede releer
// porque con `demoMoor == "si"` las fuentes de consola siguen abiertas), se decodifica SU arte al monton de C, se
// resiembra el automap y se revela alrededor del heroe.
//
// `id` es el id de LvlTypes del nivel (`d2automap.LevelName` lo dice): 1 = "1 Town", 2 = "1 Wilderness".
//
// DC-002.146 -- EL REARME NO SE PUEDE LLEVAR EL CRUCE POR DELANTE. Dos blindajes, los dos medidos:
//
//  1. EL FRENO DEL RECOLECTOR. Todo el rearme corre con `gc_inhibit_collection` (`d2cGcFrena`, la MISMA que el
//     runtime de libgodc se pone en sus secciones criticas, ver `csrc/shim.c`). Es el freno que DC-002.78 midio
//     para el parseo de los `.dt1`: el recolector de libgodc es de COPIA con barrido conservador de pila, y si
//     salta en medio de un parseo deja punteros sin actualizar. El rearme pide poco (la tabla del nivel son ~6 KB
//     de filas y el `.txt` va al monton de C, ver `leerLaTablaDelNivel`), asi que frenarlo un segundo no cuesta
//     nada. `MINIMAPA_DIAG=flujo` lo apaga: es el A/B del camino viejo, el que se llevaba el cruce.
//
//  2. EL AVISO. Si igual se cae, un `defer` con `recover` deja en el log EL PASO en el que se cayo y el estado en
//     que queda el minimapa. OJO, MEDIDO: ese `recover` NO salva el cruce en esta version de libgodc. Se probo con
//     `MINIMAPA_DIAG=panico` (un panic de rebanada de verdad en el rearme): el log sale entero
//     (`MINIMAPA REARME SE CAYO Y NO SE LLEVA EL CRUCE paso= ... nivelQueQueda= 1 ...`) y despues la consola muere
//     con `kernel panic: unhandled IRQ/Exception` (/tmp/capturas-146p). La causa esta en el runtime: `recover()`
//     no desenrolla la pila, hace `longjmp` al `runtime_checkpoint()`, y ese checkpoint hace `setjmp` dentro de una
//     funcion de C que ya retorno -> el marco esta muerto. Sin checkpoint, el runtime mata la consola con
//     `FATAL: recover without checkpoint`. O sea: el `recover` usable necesita un checkpoint con el marco VIVO, y
//     eso es un arreglo del runtime vendorizado (tarjeta aparte), no de este archivo.
func (m *minimapaDelJuego) recargarElMinimapaDelNivel(id, jugador int) bool {
	if m == nil {
		return false
	}

	return m.rearmarElMinimapaDelNivel(id, jugador)
}

// rearmarElMinimapaDelNivel: el cuerpo del rearme, con el punto de recuperacion y el `recover` del blindaje (ver
// la cabecera de `recargarElMinimapaDelNivel`).
//
// EL `recover` ES UN AVISO, NO UN SALVAVIDA (medido). libgodc no desenrolla la pila: `recover()` hace `longjmp` a
// un checkpoint y el que hay (`runtime_checkpoint`) hace `setjmp` en un marco que ya murio. Con checkpoint la consola
// muere con `kernel panic: unhandled IRQ/Exception` justo despues del log; sin checkpoint, con `FATAL: recover
// without checkpoint`. Se deja SIN checkpoint a proposito: el log sale entero y la muerte es ordenada.
func (m *minimapaDelJuego) rearmarElMinimapaDelNivel(id, jugador int) (ok bool) {
	paso := "empezar"

	// EL AVISO (ver la cabecera: en esta version de libgodc el `recover` deja el log pero NO salva la corrida).
	defer func() {
		if r := recover(); r != nil {
			ok = false

			motivo := "<no es texto>"
			if texto, esTexto := r.(string); esTexto {
				motivo = texto
			}

			println("JUEGO130: MINIMAPA REARME SE CAYO Y NO SE LLEVA EL CRUCE paso=", paso,
				"nivelPedido=", id, "nivelQueQueda=", m.Nivel, "nombre=", d2automap.LevelName(m.Nivel),
				"celdas=", m.Modelo.Count(), "cuadrosDeArte=", m.CuadrosArte,
				"montonC_libre=", d2cMontonLibre(), "semiespacio=", int(d2cGcUsado()),
				"panic=", motivo, ": EL MINIMAPA QUEDA COMO ESTABA")
		}
	}()

	// MINIMAPA_DIAG=panico: LA PRUEBA EN CONTRA DEL BLINDAJE. Una rebanada fuera de rango DE VERDAD, en el paso en
	// el que el defecto de DC-002.145 moria (la relectura de la tabla del automapa). Sirve para ver en UNA corrida
	// lo que el defecto hacia intermitente: el panic, el log del blindaje y el cruce que cierra igual.
	if minimapaDiag == "panico" {
		var nada []byte

		fuera := 2

		paso = "la tabla del nivel nuevo (PRUEBA EN CONTRA)"

		println("JUEGO130: MINIMAPA PRUEBA-EN-CONTRA panic deliberado en el rearme, indice=", len(nada[:fuera]))
	}

	t0 := d2cMs()

	nivelAntes := m.Nivel
	celdasAntes := m.Modelo.Count()
	cuadrosAntes := m.CuadrosArte
	bytesArteAntes := m.BytesArte
	recoleccionesAntes := int(d2cGcColecciones())
	filasAntes := 0

	if m.Tabla != nil {
		filasAntes = len(m.Tabla.LevelRows(nivelAntes))
	}

	libreAntes := d2cMontonLibre()
	semiespacioAntes := int(d2cGcUsado())

	// ---- 1. LA TABLA DEL NIVEL NUEVO, ANTES DE SOLTAR NADA: si el nivel no tiene filas, el minimapa que hay se
	// queda como esta (un mapa vacio es peor que el viejo, y el log lo dice). Es tambien lo que hace que un panic
	// en la lectura NO deje el minimapa a medias: todavia no se toco nada.
	paso = "la tabla del nivel nuevo"

	fuente := fuenteDe(rutaDatos)
	if fuente == nil {
		println("JUEGO130: MINIMAPA REARME SIN FUENTE DE DATOS nivel=", id, ": no se toca el minimapa")

		return false
	}

	// MINIMAPA_DIAG=carga: LA CARGA QUE REPRODUCE EL DEFECTO A VOLUNTAD. Doce relecturas de la tabla por el camino
	// viejo (el flujo del MPQ, sin freno) ANTES de la de verdad: cada relectura son ~4 recolecciones adentro del
	// parseo, o sea ~48 ventanas del defecto en UNA corrida, en vez de las 4 de un cruce normal. Es la corrida con
	// la que se dice "reproducido" sin depender de la suerte del layout, y la misma palanca sirve de control con
	// el arreglo puesto (`flujo`/`carga` son los unicos que van por el camino viejo).
	if minimapaDiag == "carga" {
		paso = "la carga de la prueba (12 relecturas por el flujo)"

		for vuelta := 0; vuelta < 12; vuelta++ {
			if _, _, errCarga := leerLaTablaDelNivel(fuente, id); errCarga != nil {
				println("JUEGO130: MINIMAPA CARGA relectura", vuelta, "fallo:", errCarga.Error())

				break
			}
		}

		println("JUEGO130: MINIMAPA CARGA terminada vueltas= 12 recolecciones=", int(d2cGcColecciones()),
			"semiespacio=", int(d2cGcUsado()))
	}

	// ---- 1b. EL FRENO DEL RECOLECTOR, SOLO SOBRE LA LECTURA (medido, y sono dos veces).
	//
	// Lo que se protege es la ventana del defecto de DC-002.145: el recolector de libgodc corriendo EN MEDIO del
	// parseo. Se protege la LECTURA y no el rearme entero por dos razones medidas:
	//   - el rearme entero frenado no entra en el semiespacio del nivel grande: en 32 x 32 el vivo deja 275 KB libres
	//     y el rearme completo pide entre 197 y 345 KB -> `fatal error: out of memory` (/tmp/capturas-146q). La
	//     lectura sola pide ~78 KB (las 143 lineas filtradas) y entra siempre.
	//   - el resto del rearme (arte, modelo, revelado) no parsea texto: una recoleccion ahi no tiene la pila llena de
	//     cadenas a medio armar, que es lo que mordia.
	//
	// PRIMERO UNA RECOLECCION PROVOCADA: es la que el umbral pide igual tres instrucciones despues (el rearme llega
	// con el semiespacio al 81-88 %), pero aca corre con la pila en un punto simple, antes del parseo. Despues se
	// frena si hay lugar para lo que la lectura pide; si no, se avisa y la lectura va con el recolector suelto (que
	// es como corria antes de esta tarjeta).
	paso = "la tabla del nivel nuevo"

	frenado := elRearmeVaFrenado()

	if frenado {
		antes := int(d2cGcUsado())

		runtime.GC()

		println("JUEGO130: MINIMAPA REARME recolectar-antes-de-la-lectura semiespacio=", antes, "->",
			int(d2cGcUsado()), "recolecciones=", int(d2cGcColecciones()))

		if !hayLugarParaElFreno() {
			frenado = false

			println("JUEGO130: MINIMAPA REARME SIN LUGAR PARA EL FRENO libre=", int(d2cGcTotal())-int(d2cGcUsado()),
				"de", int(d2cGcTotal()), ": la lectura va con el recolector suelto")
		} else {
			d2cGcFrena()
		}
	}

	tabla, bytesTabla, err := leerLaTablaDelNivel(fuente, id)

	if frenado {
		d2cGcSuelta()
	}
	if err != nil {
		println("JUEGO130: MINIMAPA REARME SIN TABLA nivel=", id, "motivo=", err.Error(), ": no se toca el minimapa")

		return false
	}

	filasNuevas := len(tabla.LevelRows(id))
	if filasNuevas == 0 {
		println("JUEGO130: MINIMAPA REARME el nivel", id, d2automap.LevelName(id),
			"NO TIENE FILAS en la tabla del automapa: no se toca el minimapa")

		return false
	}

	// ---- 2. EL ARTE DEL NIVEL QUE SE DEJA, DE VUELTA AL MONTON DE C
	paso = "soltar el arte del nivel que se deja"

	liberados, cuadrosSoltados := m.soltarElArteDelNivel()

	// ---- 3. EL NIVEL NUEVO
	paso = "el arte del nivel nuevo"

	m.Nivel = id
	m.Tabla = tabla
	m.BytesTabla = bytesTabla
	m.BytesArte, m.BytesCompacto, m.CuadrosArte, m.SinArte = 0, 0, 0, 0

	// el modelo del nivel que se deja NO se vacia: se tira (`NewModel`), que es lo que hace el motor al cambiar de
	// nivel (cada nivel tiene su automapa). Los contadores del revelado si quedan: son de la corrida.
	m.Modelo = d2automap.NewModel()

	// el reveal tiene que volver a disparar con la primera celda del nivel nuevo
	m.hayUltima, m.hayDibujo = false, false
	m.ultimaX, m.ultimaY = 0, 0
	m.ultimasCeldasDib, m.celdasUltimas, m.celdasMax, m.pxUltimos = 0, 0, 0, 0

	faltan := m.cargarElArte(fuente)

	// ---- 4. EL REVEAL DEL NIVEL NUEVO: entero si la palanca lo pide (que es lo que hace la CARGA del pueblo con
	// `MINIMAPA_TODO=si`, y por eso el minimapa del pueblo se ve completo en pantalla) o alrededor del heroe.
	paso = "el revelado del nivel nuevo"

	if minimapaTodo == "si" {
		m.revelarTodoElNivel()
	} else {
		m.revelarElMinimapa(jugador) // la primera celda siempre entra: `hayUltima` = false
	}

	// EL PICO ANTES DE SOLTAR: lo que el rearme pide con el recolector frenado es `pico - semiespacioAntes`, y es el
	// numero con el que se decide si el freno entra (`hayLugarParaElFreno`). Se mide, no se estima.
	pico := int(d2cGcUsado())

	runtime.GC()

	paso = "el resumen"

	// DC-002.146b: la cobertura del modelo, que es lo que el criterio pide ("casillas de la region con celda en el
	// modelo"). Se cuenta ACA, antes de imprimir, y es lo que dice si la siembra lleno el nivel.
	conCelda, de := m.coberturaDelModelo()

	println("JUEGO130: MINIMAPA REARME nivelAntes=", nivelAntes, "nivelAhora=", m.Nivel,
		"nombre=", d2automap.LevelName(m.Nivel), "filasDeLaTabla=", filasAntes, "->", filasNuevas,
		"celdasAntes=", celdasAntes, "celdasAhora=", m.Modelo.Count(),
		"piso=", m.Modelo.CountLayer(d2automap.LayerFloor), "pared=", m.Modelo.CountLayer(d2automap.LayerWall),
		"sembradas=", m.sembradasUltimo, "siembra=", minimapaSiembra,
		"casillasConCelda=", conCelda, "de", de, "pct=", conCelda*100/maxDe(de),
		"cuadrosDeArte=", cuadrosAntes, "->", m.CuadrosArte, "cuadrosSoltados=", cuadrosSoltados, "faltan=", faltan,
		"bytesDeArte=", bytesArteAntes, "->", m.BytesArte, "liberados=", liberados,
		"libreAntes=", libreAntes, "libreDespues=", d2cMontonLibre(),
		"recolecciones=", recoleccionesAntes, "->", int(d2cGcColecciones()),
		"frenado=", frenado,
		"semiespacioAntes=", semiespacioAntes, "pico=", pico, "pide=", pico-semiespacioAntes,
		"semiespacioDespues=", int(d2cGcUsado()),
		"ms=", int(d2cMs()-t0))

	return true
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

	// DC-002.137 -- LA TABLA, UNA VEZ POR CUADRO (no una vez por celda). Las 610 celdas que caen en la caja
	// comparten la paleta y el estado (el minimapa no pone color), asi que la tabla de 256 palabras se arma aca
	// una sola vez y se pasa (`pvr.TablaDelBlit`, del POC: el backend no guarda estado del dibujado, que es la
	// leccion de DC-002.134). El camino viejo la armaba DENTRO de cada `RenderIndicesTintado` -- 610 tablas por
	// cuadro, 156.160 entradas, treinta veces los 4.968 pixeles que se pintan de verdad.
	// TRES CAMINOS (DC-002.137), y el A/B es una corrida:
	//	"no"    el de DC-002.130: una tabla por celda, un Push/Pop por celda (el camino viejo, intacto)
	//	"tabla" la tabla UNA vez por cuadro y los indices leidos de a cuatro
	//	"si"    ademas la lista COMPACTA de los pixeles que pintan (~8 por celda en vez de 128 indices)
	rapido := minimapaRapido == "si" || minimapaRapido == "tabla"
	compacto := minimapaRapido == "si"

	if rapido {
		tTabla := cronometro134()

		rapido = sup.ArmarTablaDelBlit(m.Pal[:], &tablaDelMinimapa)
		anotarLaTabla137(tTabla)
	}

	// EL FONDO NEGRO DE LA CAJA, ANTES DE LAS CELDAS. El arte del automapa (`maximap.dc6`) son lineas FINAS pensadas
	// para leerse sobre negro: el juego las dibuja sobre la pantalla del mapa, que va en negro. Sobre el terreno del
	// POC se pierden (medido en Flycast: el lienzo cambiaba y el mapa no se veia). Con la caja en negro, las celdas
	// se leen.
	//
	// DC-002.137: la caja son 62.775 px y `DrawRect` los pintaba uno por uno con `Poner` (cuatro comparaciones de
	// recorte y cuatro depositos de 8 bits por pixel). Ahora `Imagen.Rectangulo` recorta una vez y rellena por
	// palabras de 32 bits: los mismos bytes, un deposito por pixel.
	tCaja := cronometro134()

	if minimapaDiag != "celdas" {
		sup.PushTranslation(lay.Clip.X0, lay.Clip.Y0)
		sup.DrawRect(lay.Clip.X1-lay.Clip.X0, lay.Clip.Y1-lay.Clip.Y0, fondoDelMinimapa)
		sup.Pop()
	}

	anotarLaCaja137(tCaja)

	if minimapaDiag != "caja" {
		tCeldas := cronometro134()

		if rapido {
			pintados, celdas = m.pintarLasCeldasConLaTabla(sup, lay, compacto)
		} else {
			pintados, celdas = m.pintarLasCeldasDeSiempre(sup, lay)
		}

		anotarLasCeldas137(tCeldas)

		// ---- EL MARCADOR DEL HEROE: la polilinea del motor, en su sitio de la caja
		tMarcador := cronometro134()

		m.dibujarElMarcador(sup, mx-8, my+8, lay)
		anotarElMarcador137(tMarcador)
	}

	// DC-002.137: la corrida de medir `MINIMAPA_DIAG=tabla610` (arma la tabla 610 veces mas, el cuadro queda
	// identico: es el costo de la tabla por celda del camino viejo, medido desde el otro lado).
	if minimapaDiag == "tabla610" {
		tExtra := cronometro134()

		armarLaTablaDeMas137(sup, m, celdas)
		anotarLaTablaDeMas137(tExtra)
	}

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

	// DC-002.137: el crc32 de la caja (279x225) DESPUES de dibujarla, y FUERA del cronometro del minimapa (es
	// evidencia, no dibujo). Es la prueba de que el camino rapido y el camino de siempre dejan la MISMA caja: los
	// dos numeros se comparan entre dos corridas del mismo binario (`MINIMAPA_RAPIDO=si` y `=no`). Con
	// `VERBOSO=no` no se mide: son 251.100 B de lectura por cuadro.
	crcCaja := uint32(0)

	if verboso134() {
		crcCaja = crcDeLaCaja137(sup, lay)
	}

	// DC-002.134: la linea del minimapa es evidencia por cuadro: con `VERBOSO=no` se calla.
	if verboso134() {
		println("JUEGO130: MINIMAPA cuadro=", cuadro, "jugador=", jugador+1, "celdas=", celdas, "de", m.Modelo.Count(),
			"pixelesPintados=", pintados, "ms=", ms, "heroe=", hx, hy, "enLaCaja=", mx, my,
			"caja=", lay.Clip.X0, lay.Clip.Y0, lay.Clip.X1, lay.Clip.Y1, "sinArte=", m.SinArte,
			"crcDeLaCaja=", int(crcCaja), "rapido=", minimapaRapido, "diag=", minimapaDiag)
	}

	// DC-002.137: el promedio por cuadro de la caja, las celdas, el marcador y la tabla (cada 30 cuadros).
	informeDelMini137(pintados, celdas, m.Modelo.Count())

	return pintados, ms
}

// pintarLasCeldasDeSiempre: EL CAMINO DE DC-002.130, TAL CUAL (una tabla de 256 palabras por celda, dentro de
// `RenderIndicesTintado`, y un `PushTranslation`/`Pop` por celda). Es el A/B de `MINIMAPA_RAPIDO=no`: vive en el
// mismo binario que `pintarLasCeldasConUnaTabla` y tiene que dejar los MISMOS pixeles (lo prueba el crc32 de la
// caja). No se toca: si se "mejora" deja de ser el A/B.
func (m *minimapaDelJuego) pintarLasCeldasDeSiempre(sup *pvr.Surface, lay d2automap.Layout) (int, int) {
	pintados, celdas := 0, 0

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

	return pintados, celdas
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

	println("JUEGO130: MINIMAPA ARTE-RESUMEN cuadros=", m.CuadrosArte, "bytesDeLaListaCompacta=", m.BytesCompacto,
		"bytesEnMontonDeC=", m.BytesArte,
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

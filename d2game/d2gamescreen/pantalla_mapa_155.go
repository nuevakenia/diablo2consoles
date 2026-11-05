package d2gamescreen

// DC-002.155 -- LA PANTALLA DEL GATE 0: EL PUEBLO COMPUESTO Y DIBUJADO POR EL MOTOR, SIN MENUS.
//
// POR QUE EXISTE. Gate 0 pide UN cuadro real del pueblo pintado por el MOTOR en la consola. El motor tiene
// su propia pantalla de mapa (`MapEngineTest`), pero arma un `HeroStateFactory` (`d2inventory` ->
// `armor.txt` + los objetos del equipo inicial) y pide musica (`d2audio`, un .wav que en el disco de la
// consola no esta): las dos cosas son MB que el monton de C del binario entero no tiene. Esta pantalla es el
// camino MINIMO que usa el motor de verdad para componer y dibujar el pueblo:
//
//  1. `d2mapengine.CreateMapEngine` + `LoadStamp` leen el `.ds1` y los `.dt1` del MPQ por el AssetManager
//     del motor (el mismo camino de `d2app`, con las palancas de consola de `d2asset.ArrancarComoConsola`).
//  2. `ResetMapParaStamp` (DC-002.88) carga SOLO los `.dt1` que el `.ds1` lista -- el level type del pueblo
//     declara 9 y el `.ds1` usa 8: 960.096 B de arena que ninguna casilla mira.
//  3. `PlaceStampClippedWhere(..., keep=false, monsters=false)`: las casillas SOLAS, sin entidades. Las
//     entidades piden `Objects.txt`/`monstats` + la animacion (`animdata.d2`, 2,17 MB de RGBA medidos en el
//     host), que es justo lo que no entra. El Gate 0 es el PUEBLO, no sus habitantes.
//  4. `d2maprenderer.CreateMapRenderer` con las opciones de la consola (cache perezosa, en indices, en
//     bloques de VRAM): el mismo renderer que dibuja el juego, con la cache que entra en 16 MB.
//
// LO QUE NO HACE: nada de juego. No hay heroe, ni HUD, ni inventario, ni audio, ni entrada. Es un cuadro y
// su medicion. La pantalla que se ve es la del motor (su mapa, sus tiles, su camara) presentada por el PVR.

import (
	"image/color"
	"runtime"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2display"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2enum"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2interface"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2math/d2vector"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2resource"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2util"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2asset"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2map/d2mapengine"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2map/d2maprenderer"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2screen"
)

const (
	// los cuadros en los que se informa: el 1 es el primer cuadro compuesto; el 11 es el que el shim del PVR
	// perfila por dentro (EBITEN-PERFIL); los demas son la curva (si el cuadro se degrada, se ve aca).
	cuadrosQueSeInforman = 300
)

// ============================== EL PLAN DE LA CORRIDA (DC-002.157) ==============================
//
// La tarjeta pide "ms por cuadro medidos en Flycast ANTES y DESPUES de cada palanca, en la misma corrida".
// Las cuatro palancas (el mapa estatico del renderer y las tres de la presentacion) se prenden y apagan EN
// CALIENTE, asi que la corrida se parte en tramos y cada tramo es una ventana de medida: mismo binario,
// mismo arranque, mismas condiciones. La escalera es acumulativa -- cada tramo tiene todo lo del anterior
// mas UNA palanca -- y asi cada escalon dice cuanto paga esa palanca sola:
//
//	cuadros   1..20   NADA                  el cuadro de DC-002.156 (1,88 s medidos)
//	cuadros  21..40   + mapa estatico       el mapa deja de repintarse (la camara no se movio)
//	cuadros  41..60   + conversion rapida    el bucle de DC-002.135 en vez del de DC-002.132
//	cuadros  61..80   + por filas            el orden de lectura de DC-002.141 (el lienzo seguido)
//	cuadros  81..100  + conversion por zona  solo el recuadro visible: 640x480 de 800x600
//	cuadros 101..120  + presentacion quieta  si el cuadro no cambio, no se convierte ni se sube
//	cuadros 121..140  el mapa DE NUEVO, sin censo  el A/B del instrumento: el mapa se repinta (se apaga el
//	                                        salto) y se apaga el cronometro por tile, asi el numero pelado
//	                                        del mapa queda comparable con el de DC-002.156 (que no tenia
//	                                        censo). Si el mapa cuesta lo mismo con y sin censo, el censo es
//	                                        gratis; si no, la diferencia ES el costo de medir.
//
// LOS TRAMOS SON DE 20 CUADROS. El primero es el caro (20 x 1,8 s = 36 s) y el ultimo tambien (otros 36 s);
// los del medio son decimas. Con el arranque del pueblo (50 s) la corrida entera entra en los 180 s del
// script. El tramo se informa al CERRARSE (cuando entra la palanca siguiente), con el promedio del tramo y el
// censo del renderer.
const (
	desdeMapaEstatico157       = 21
	desdeConversionRapida157   = 41
	desdeConversionPorFilas157 = 61
	desdeConversionPorZona157  = 81
	desdePresentacionQuieta157 = 101
	desdeElMapaDeNuevo157      = 121
)

// palancasDePresentacion157: lo que la pantalla le puede pedir a la Surface del cuadro para la presentacion.
//
// POR QUE UNA INTERFAZ Y NO UN IMPORT. La Surface del motor es `d2interface.Surface` y abajo, en la consola,
// es el shim del PVR (`console/dc/engine/ebiten/v2`); en la PC es Ebitengine de verdad. La pantalla no tiene
// que saber cual de los dos es: pregunta por esta interfaz y, si la Surface no la implementa (la PC), no
// pasa nada. Es el mismo patron de `lienzoDeIndices` (DC-002.101/155). La implementan
// `d2core/d2render/ebiten/palancas_157.go` (solo consola: lleva `//go:build dreamcast`).
type palancasDePresentacion157 interface {
	ActivarConversionRapida157(si bool)
	ActivarConversionPorFilas157(si bool)
	ActivarConversionPorZona157(si bool)
	ActivarPresentacionQuieta157(si bool)
}

// d2cRelojUs157: el reloj de microsegundos del backend del PVR. Lo engancha el arranque del POC
// (`console/dc/poc/gate0-pueblo/main.go`, que es el que tiene los //extern) al cronometro por tile del
// renderer con `d2maprenderer.FijarRelojDeLosTiles`. Aca solo se USA el censo, que es Go puro.

// PantallaDelPueblo es la pantalla del Gate 0.
type PantallaDelPueblo struct {
	asset      *d2asset.AssetManager
	renderer   d2interface.Renderer
	term       d2interface.Terminal
	nivelDeLog d2util.LogLevel

	region  d2enum.RegionIdType
	preset  int
	archivo int

	opciones     []d2maprenderer.Opcion
	conEntidades bool

	// instrumento: lo llama la pantalla en cada etapa para que el arranque de la consola imprima el monton
	// (o lo que quiera medir). Puede ser nil: en la PC no hay monton de C que mirar.
	instrumento func(etapa string)

	engine      *d2mapengine.MapEngine
	mapRenderer *d2maprenderer.MapRenderer

	// ============================== EL HEROE (DC-002.159) ==============================
	// heroe: el jugador del motor, si el arranque lo pidio con `ConHeroe`. entrada + refrescarEntrada: el
	// servicio de entrada de la consola (el maple) y el que lo lee antes de preguntarle las teclas. Sin
	// `ConHeroe` la pantalla es la de DC-002.157, cuadro y nada mas.
	heroe            *HeroeDelGate0
	entrada          d2interface.InputService
	refrescarEntrada func()
	conHeroe         bool
	plan159          bool
	// soltado159: el cuadro 1 ya cerro (el mapa se pinto, se devolvio su memoria y el heroe entro).
	soltado159 bool
	// inicio159X/Y: la celda donde nace el heroe, en celdas del mapa (se anota en OnLoad, con las casillas
	// puestas, y es lo que centra la camara: el centro del mapa deja al heroe FUERA del lienzo).
	inicio159X, inicio159Y float64
	// compuestoDelHeroe: el COF + las capas del heroe, cargados ANTES de componer el mapa (ver OnLoad)
	compuestoDelHeroe *d2asset.Composite

	cuadros        int
	msDelCuadro    int
	msPrimerCuadro int
	msTotal        int
	pisos          int
	paredes        int

	// ============================== EL PLAN DE LA CORRIDA (DC-002.157) ==============================
	// El estado de las palancas (para saber cuando CAMBIAN: el cambio es lo que cierra un tramo) y lo que se
	// acumula del tramo abierto.
	mapaEstatico157 bool
	convRapida157   bool
	convFilas157    bool
	convZona157     bool
	quieta157       bool

	tramo157Cuadros    int
	tramo157MapaUs     int
	tramo157Saltos     int
	tramo157Repintados int
	tramo157Etiqueta   string

	*d2util.Logger
}

// CreatePantallaDelPueblo arma la pantalla (todavia sin componer: eso pasa en OnLoad).
func CreatePantallaDelPueblo(asset *d2asset.AssetManager, renderer d2interface.Renderer,
	term d2interface.Terminal, l d2util.LogLevel, region d2enum.RegionIdType, preset int,
	conEntidades bool, instrumento func(etapa string), opciones ...d2maprenderer.Opcion,
) *PantallaDelPueblo {
	pantalla := &PantallaDelPueblo{
		asset:        asset,
		renderer:     renderer,
		term:         term,
		nivelDeLog:   l,
		region:       region,
		preset:       preset,
		archivo:      0,
		opciones:     opciones,
		conEntidades: conEntidades,
		instrumento:  instrumento,
	}

	pantalla.Logger = d2util.NewLogger()
	pantalla.Logger.SetPrefix(prefijoDelLog)
	pantalla.Logger.SetLevel(l)

	return pantalla
}

func (p *PantallaDelPueblo) avisar(etapa string) {
	if p.instrumento != nil {
		p.instrumento(etapa)
	}
}

// ConHeroe (DC-002.159): la pantalla deja de ser un cuadro y pasa a tener EL HEROE DEL MOTOR caminando por
// el pueblo con la cruceta. `entrada` es el servicio de entrada de la consola (el maple,
// d2core/d2input/maple, que traduce la cruceta a las cuatro flechas) y `refrescar` es el que lo lee del
// aparato antes de preguntarle las teclas (en la consola, una llamada al bus maple). Con esto la pantalla
// prende el mapa estatico y los entes aparte: es la unica forma de que el mapa no vuelva a costar 818 ms
// por cuadro con un ente adentro (DC-002.159).
func (p *PantallaDelPueblo) ConHeroe(entrada d2interface.InputService, refrescar func()) {
	p.conHeroe = true
	p.entrada = entrada
	p.refrescarEntrada = refrescar
}

// DisplayAnchor: el mapa ocupa la pantalla entera, no la columna de 800x600 de los menus.
func (p *PantallaDelPueblo) DisplayAnchor() d2display.Anchor {
	return d2display.AnchorBottom
}

// OnLoad compone el pueblo. Corre en la goroutine del screen manager: cada `Progress` espera al cuadro
// siguiente, asi que la pantalla de carga (negra en la consola, ver `d2gui.SinAnimacionesDeConsola`) sigue
// presentando mientras el PVR y el motor trabajan.
func (p *PantallaDelPueblo) OnLoad(loading d2screen.LoadingState) {
	p.avisar("mapa:empieza")

	// DC-002.155: el tramo de la lectura de las tablas ya no lo mira nadie (el contrato de
	// `ActivarLecturasPorTramos` dice que su contenido vale hasta la lectura siguiente). Devolverlo son los
	// ~135 KB que el arranque del motor tenia tomados sin usar, justo cuando empieza lo que no entra.
	d2asset.SoltarLecturasDeConsola()
	p.avisar("mapa:tramo-de-las-tablas-soltado")

	p.engine = d2mapengine.CreateMapEngine(p.nivelDeLog, p.asset)
	p.avisar("mapa:engine-creado")

	// ============================== EL COMPUESTO DEL HEROE, ACA (DC-002.159) ==============================
	// OJO CON EL LUGAR, MEDIDO. Lo caro del heroe es su compuesto (COF + capas `.dcc` + paleta): ~1 MB. El
	// pueblo compuesto deja 1,4 MB de arena y el renderer pide ~890 KB mas, asi que armandolo despues no entra
	// (medido: `gc_external_alloc: OOM` y `stack_alloc: out of memory`, cinco reinicios del emulador). Aca,
	// antes de componer el mapa, hay 4,8 MB: el compuesto entra sin drama y el pico del mapa no lo incluye.
	// El ENTE no se puede adelantar (su punto de arranque sale del mapa ya compuesto), asi que se crea despues
	// con `NuevoJugadorDeConsolaCon`, que recibe este mismo compuesto.
	if p.conHeroe {
		p.avisar("heroe:compuesto-empieza")

		compuesto, err := p.asset.LoadComposite(d2enum.ObjectTypePlayer, d2enum.HeroAmazon.GetToken(),
			d2resource.PaletteUnits)
		if err != nil {
			p.Errorf("HEROE159 SIN-HEROE: el compuesto del MPQ no se pudo cargar: %v", err)

			return
		}

		p.compuestoDelHeroe = compuesto
		p.avisar("heroe:compuesto-listo")

	}

	loading.Progress(0.10)

	t0 := d2util.Now()

	stamp := p.engine.LoadStamp(p.region, p.preset, p.archivo)
	if stamp == nil {
		p.Error("LoadStamp devolvio nil: el pueblo no esta en el MPQ")
		return
	}

	p.avisar("mapa:stamp-leido")

	loading.Progress(0.25)

	tamano := stamp.Size()
	p.Infof("el pueblo: %dx%d casillas, preset %d, archivo %d", tamano.Width, tamano.Height, p.preset, p.archivo)

	// DC-002.88: los `.dt1` que el `.ds1` lista, no los 32 campos del level type
	p.engine.ResetMapParaStamp(p.region, tamano.Width, tamano.Height, stamp)
	p.avisar("mapa:reset-hecho")

	loading.Progress(0.55)

	if p.conEntidades {
		p.engine.PlaceStamp(stamp, 0, 0)
	} else {
		// sin entidades: ni monstruos ni objetos. `keep` en false y `monsters` en false es el camino del
		// motor sin pedirle las tablas de las entidades ni la animacion.
		p.engine.PlaceStampClippedWhere(stamp, 0, 0, tamano.Width, tamano.Height, func(int) bool { return false }, false)
	}

	p.avisar("mapa:casillas-puestas")

	// DC-002.159: EL STAMP DEL .ds1 YA HIZO SU TRABAJO.
	// Las casillas del mapa son COPIAS de las celdas del `.ds1` (contrato de DC-002.110): soltar el bloque de
	// celdas devuelve al monton 261.744 B del SH-4 justo antes del renderer y del ente del heroe, que es donde
	// la arena va al limite. Un uso tardio del stamp se ve como un stamp vacio, no como memoria de otro.
	if liberados := stamp.SoltarLosTilesDelDS1(); liberados > 0 {
		p.Infof("el .ds1 del pueblo devolvio %d B de celdas (arena de C)", liberados)
	}

	// DC-002.159: LAS FILAS DE LAS TABLAS QUE EL CUADRO NO USA, tambien aca.
	// El nivel ya esta compuesto: el renderer no lee LvlTypes ni LvlPrest (usa el levelType copiado en el
	// motor y la paleta del acto, DC-002.88/.105). Son las que el contrato de `SoltarFilasDeLasTablas` deja
	// soltar; `charstats.txt` NO se suelta porque el jugador la consulta en cada cuadro (estamina).
	// El heroe necesita estos bytes: su compuesto deja ~435 KB vivos en el semiespacio y este es el ultimo
	// lugar donde la arena tiene aire.
	if soltadas := p.asset.SoltarFilasDeLasTablas([]string{d2resource.LevelType, d2resource.LevelPreset}); soltadas.Tablas > 0 {
		p.Infof("tablas soltadas (LvlTypes, LvlPrest): %d tablas, %d B devueltos al monton",
			soltadas.Tablas, soltadas.Bytes)
	}

	runtime.GC()

	// DC-002.156: EL BUFFER PRESTADO DE LA LECTURA DE LOS `.dt1` YA HIZO SU TRABAJO.
	//
	// Los `.dt1` del pueblo estan leidos y sus bloques copiados al arena (`EncodedData` de cada tile apunta
	// ahi). El crudo no lo mira nadie mas, y ese buffer es el que el arranque de la consola pide a la VRAM
	// (DC-002.154): 1.051.532 B. Devolverlo AHORA, antes de crear el renderer, es lo que le deja lugar a la
	// CACHE DE TILES en la VRAM.
	//
	// MEDIDO (corrida del Gate 0, DC-002.156): sin esto la cache pide sus 4 tramos de 348.160 B y la VRAM no
	// los da -- estan la textura del cuadro (2.097.152) y el framebuffer del motor (1.920.000) mas este
	// buffer --, asi que dos tramos caen al MONTON DE C: 633.648 B que el recolector despues no puede soltar
	// porque la cache los tiene tomados, y el arranque muere con `out of memory` en el primer cuadro.
	d2asset.SoltarPrestamoDeLecturaDeTiles()

	p.avisar("mapa:prestamo-de-lectura-soltado")

	p.pisos, p.paredes = censoDelPueblo(p.engine)

	p.Infof("pueblo compuesto en %d ms: pisos=%d paredes=%d", p.msDesde(t0), p.pisos, p.paredes)

	loading.Progress(0.75)

	// ============================== EL HEROE (DC-002.159b) ==============================
	// EL HEROE YA NO SE CREA ACA, y el motivo esta medido. Crearlo cuesta ~436.000 B VIVOS en el semiespacio
	// del recolector (la capa base del COF con los cuadros del modo Neutral en las 16 direcciones) mas un
	// pedido de ~443 KB al monton externo, y con el pueblo compuesto no hay lugar: en DC-002.159 el recolector
	// moria con `GC to-space overflow` (1.728.184 vivos de 1.835.008) y el monton con `gc_external_alloc: OOM`.
	//
	// El lugar esta DEL LADO DEL MAPA. Desde DC-002.157 el mapa se pinta UNA vez y los cuadros siguientes se
	// saltan el renderer entero (`CUADRO 31 ms=0`), asi que ni las 2.312 casillas del motor ni el arena de
	// bloques de los `.dt1` (2.621.440 B del monton de C) se vuelven a leer. Por eso el heroe entra en el
	// CUADRO 1: primero se pinta el mapa, despues se devuelve esa memoria y recien ahi se arma el ente
	// (`soltarElMapaYEntrarElHeroe`, en Render).
	//
	// LO UNICO QUE HAY QUE ANOTAR ACA es el punto de arranque, porque `GetStartPosition` recorre las casillas
	// buscando el objeto de entrada del `.ds1` y despues de soltarlas ya no hay que recorrer: se pregunta con
	// las casillas puestas y se deja fijado con `SetStartPosition` (la via del motor para eso).
	if p.conHeroe {
		p.plan159 = true

		inicioX, inicioY := p.engine.GetStartPosition()
		p.engine.SetStartPosition(inicioX, inicioY)

		p.inicio159X, p.inicio159Y = inicioX, inicioY

		p.Infof("HEROE159B el pueblo arranca en la celda %.1f,%.1f (anotado antes de soltar las casillas)", inicioX, inicioY)
	}

	// las opciones van ANTES de crear el renderer: la cache se arma (o no) en el constructor
	p.mapRenderer = d2maprenderer.CreateMapRenderer(p.asset, p.renderer, p.engine, p.term, p.nivelDeLog,
		0.0, 0.0, p.opciones...)

	p.avisar("mapa:renderer-creado")

	// DC-002.159: el mapa se pinta una vez y el ente va aparte (sin esto un solo ente apaga el mapa estatico).
	// DC-002.159b: el mapa estatico NO se prende aca: el CUADRO 1 tiene que pintar el mapa de verdad (es el
	// unico que lo pinta), y es el final de ese cuadro -- cuando se devuelve la memoria del mapa -- el momento
	// en que el salto queda prendido para siempre. Ver `soltarElMapaYEntrarElHeroe`.
	if p.conHeroe {
		p.mapRenderer.ActivarEntesAparte159(true)
		p.mapRenderer.ActivarMapaEstatico(false)
		p.mapaEstatico157 = false
	}

	// DC-002.159b: LA CAMARA VA DONDE NACE EL HEROE, no al centro del mapa. Centrada en el mapa el heroe cae
	// FUERA del lienzo: el pueblo nace en la celda 26.5,7.5 (el objeto de entrada del `.ds1`) y el centro del
	// mapa es 28.5,20.5, asi que los pies del heroe quedaban en y=-368, arriba de la pantalla, y no habia nada
	// que dibujar (medido en la primera corrida de esta tarjeta).
	camaraX, camaraY := p.engine.GetCenterPosition()
	if p.conHeroe {
		camaraX, camaraY = p.inicio159X, p.inicio159Y
	}

	posicion := d2vector.NewPosition(p.mapRenderer.WorldToOrtho(camaraX, camaraY))
	p.mapRenderer.SetCameraPosition(&posicion)

	loading.Progress(0.90)

	p.Infof("renderer del mapa listo: cuadro %dx%d", d2display.W(), d2display.H())
	p.avisar("mapa:listo")
}

// soltarElMapaYEntrarElHeroe (DC-002.159b): cierra el cuadro 1 devolviendo la memoria del mapa y creando el
// heroe con el lugar que eso libera.
//
// EL ORDEN ES EL CONTENIDO DE LA TARJETA:
//
//  1. el mapa estatico queda prendido PARA SIEMPRE (`SoltarElMapaPintado159`): el mapa que esta en el lienzo es
//     el unico que va a haber. Se exige que el cuadro 1 lo haya pintado; si no, no se suelta nada;
//  2. se devuelve el arena de bloques de los `.dt1` (los 2.621.440 B del monton de C): los `EncodedData` de
//     cada tile ya se decodificaron a la cache del renderer (`d2asset.SoltarArenaDeBloques`, DC-002.101);
//  3. se devuelven las casillas compuestas del motor (~12.000 registros del semiespacio del recolector) y los
//     metadatos de los `.dt1` (`d2mapengine.SoltarLasCasillasDelMapa` / `SoltarLasTablasDeTiles`);
//  4. se le pide al recolector que se lleve lo que ya nadie apunta (`runtime.GC`), y se mide;
//  5. recien ahi entra el heroe: el ente sobre el compuesto que se armo en `OnLoad` (cuando todavia habia
//     lugar) y las estadisticas de `charstats.txt`.
//
// Si algo de esto falla, el log lo dice con el numero: un heroe que no entra no puede parecer un heroe que si.
func (p *PantallaDelPueblo) soltarElMapaYEntrarElHeroe() {
	p.soltado159 = true

	p.avisar("heroe:empieza")

	if !p.mapRenderer.SoltarElMapaPintado159() {
		p.Errorf("HEROE159B NO-SE-SUELTA: el mapa no quedo pintado en el lienzo (sin mapa, soltar las casillas " +
			"dejaria la pantalla en negro y sin vuelta)")

		return
	}

	p.avisar("heroe:antes-de-soltar")

	tramos, bytesArena := d2asset.SoltarArenaDeBloques()
	casillas, bytesCasillas := p.engine.SoltarLasCasillasDelMapa()
	tiles, bytesTablas := p.engine.SoltarLasTablasDeTiles()

	p.avisar("heroe:mapa-soltado")

	runtime.GC()

	p.avisar("heroe:recolector-pasado")

	p.Infof("HEROE159B SUELTO arena_tramos=%d arena_bytes=%d casillas=%d casillas_bytes=%d tiles_dt1=%d "+
		"tiles_bytes=%d mapa_soltado=%v", tramos, bytesArena, casillas, bytesCasillas, tiles, bytesTablas,
		p.mapRenderer.MapaSoltado159())

	if p.compuestoDelHeroe == nil {
		p.Errorf("HEROE159 SIN-HEROE: el compuesto del MPQ no se cargo al arrancar (no hay heroe que crear)")
		p.avisar("heroe:error")

		return
	}

	heroe, err := NuevoHeroeDelGate0(p.asset, p.engine, p.mapRenderer, p.term, p.Logger,
		p.entrada, p.refrescarEntrada, p.compuestoDelHeroe)
	if err != nil {
		p.Errorf("HEROE159 SIN-HEROE: %v", err)
		p.avisar("heroe:error")

		return
	}

	p.heroe = heroe

	ln := p.NuevaLinea159f()
	ln.Tx("HEROE159 LISTO ")
	p.heroe.Estado159f(ln)
	ln.Enviar(p.Logger)
	p.avisar("heroe:listo")

	// DC-002.159b: una recoleccion EXPLICITA con el heroe ya adentro, antes de que empiece a dibujarse. Si el
	// recolector no puede con este conjunto vivo, tiene que morir ACA y no a mitad de un cuadro (es la misma
	// informacion, pero con el punto exacto); si puede, el semiespacio queda compacto para las superficies del
	// primer dibujo.
	runtime.GC()

	p.avisar("heroe:recolector-con-el-heroe")
}

// OnUnload no suelta nada: la pantalla vive hasta que el proceso termina (en la consola el unico final es
// que Flycast se cierre). Se deja el metodo para cumplir el contrato.
func (p *PantallaDelPueblo) OnUnload() error { return nil }

// Render dibuja el pueblo. Es el camino del motor: `d2maprenderer.Render` sobre la Surface del cuadro, que
// en la consola es el framebuffer de la VRAM y lo presenta el bucle del shim del PVR.
func (p *PantallaDelPueblo) Render(target d2interface.Surface) {
	if p.mapRenderer == nil {
		target.Clear(color.Black)
		return
	}

	p.cuadros++

	if p.plan159 {
		p.aplicarElPlan159(target)
	} else {
		p.aplicarElPlan157(target)
	}

	// DC-002.159d: el plan ya esta aplicado; lo que viene es el mapa.
	MarcarFaseGC159d(faseMapa159d)
	Marcar159f(MarcaMapa159f)

	t0 := d2util.Now()

	// cuantos repintados llevaba el renderer: el heroe necesita saber si este cuadro el mapa se pinto entero
	// (entonces no hay rastro que borrar) o se salto (entonces hay que devolver el recuadro de abajo).
	repintadosAntes := p.mapRenderer.RepintadosDelMapa157()

	// el mundo ocupa la pantalla entera: se compensa el origen de la columna (con AnchorBottom es 0,0)
	ox, oy := d2display.ColumnOrigin()
	target.PushTranslation(-ox, -oy)
	p.mapRenderer.Render(target)

	// DC-002.159d: el mapa ya esta pintado (o se salto); lo que viene es el heroe.
	MarcarFaseGC159d(faseHeroe159d)
	Marcar159f(MarcaBorrado159f)

	// ============================== DC-002.159b: SOLTAR EL MAPA, ENTRAR EL HEROE ==============================
	// El cuadro 1 es el unico que pinta el mapa (con las casillas del motor y los bloques de los `.dt1`
	// todavia en memoria). Al cerrarlo se devuelve esa memoria y recien ahi se crea el heroe, que es el que
	// la necesitaba: ~436.000 B vivos en el semiespacio y un pedido de ~443 KB al monton de C.
	if p.conHeroe && !p.soltado159 {
		p.soltarElMapaYEntrarElHeroe()
	}

	// ============================== EL HEROE (DC-002.159) ==============================
	// El mapa ya esta en el lienzo (pintado o conservado del cuadro anterior): el heroe va encima. Si la
	// Surface sabe guardar recuadros (la consola), el heroe se borra solo; si no (la PC), esto no hace nada
	// porque el renderer ya lo dibujo dentro de sus pasadas.
	if p.heroe != nil {
		if lienzo, hay := target.(lienzoConMemoria159); hay {
			p.heroe.ConMemoria(lienzo)
		}

		if p.cuadros <= 3 {
			p.avisar("heroe:dibujo-empieza")
		}

		p.heroe.DibujarSobreElMapa(target, p.mapRenderer.RepintadosDelMapa157() != repintadosAntes)

		if p.cuadros <= 3 {
			p.avisar("heroe:dibujo-hecho")
		}
	}

	// DC-002.159f: el cierre del cuadro (las lineas de estado y el tramo GC) es su propio tramo, y va partido:
	// `cPop` (cerrar la pila de transformaciones y los ms), `cCuadro`, `cEstado` y `cTramo` son cada linea.
	Marcar159f(MarcaCierre159f)

	target.Pop()

	p.msDelCuadro = p.msDesde(t0)

	// DC-002.157: el tramo abierto acumula los ms del mapa y lo que el renderer se salto / repintio. El
	// tramo se informa cuando entra la palanca siguiente (`informeDelTramo157`).
	p.tramo157Cuadros++
	p.tramo157MapaUs += p.msDelCuadro

	if p.cuadros == 1 {
		p.msPrimerCuadro = p.msDelCuadro
	}

	Marcar159f(MarcaCierreCuadro159f)

	if p.cuadros == 1 || p.cuadros == 2 || p.cuadros == 11 || p.cuadros == 31 || p.cuadros == 60 ||
		p.cuadros == 120 || p.cuadros == cuadrosQueSeInforman || (p.conHeroe && p.cuadros <= 8) {
		// DC-002.159g: esta es la cuarta linea que pasa por `Infof` a `Linea159f` (las otras tres son las de 159f).
		// Costaba 95 B/cuadro de basura -- ~2% del cuadro -- y el 100% era la escritura del logger
		// (`fmt.Output` -> `builtinOutput` -> `print(string(p))`, una copia nueva del texto por escritura).
		ln := p.NuevaLinea159f()
		ln.Tx("CUADRO ").I(p.cuadros).Tx(" ms=").I(p.msDelCuadro).
			Tx(" (primer cuadro ").I(p.msPrimerCuadro).Tx(" ms)")
		ln.Enviar(p.Logger)
		p.avisar("mapa:cuadro")
	}

	// el estado del heroe cada 15 cuadros: es la linea con la que se cruzan las capturas (posicion, direccion
	// y modo de animacion). Con el heroe quieto la linea sale igual y no ensucia nada.
	//
	// DC-002.159f: ARMARLA y MANDARLA van marcadas por separado (cArmaEst / cEnviaEst): es la unica forma de
	// saber si la basura de la linea esta en los datos o en el camino de escritura del logger.
	Marcar159f(MarcaCierreEstado159f)

	if p.heroe != nil && p.cuadros%15 == 0 {
		Marcar159f(MarcaArmaEstado159f)

		ln := p.NuevaLinea159f()
		ln.Tx("HEROE159 cuadro=").I(p.cuadros).Tx(" ")
		p.heroe.Estado159f(ln)
		ln.Tx(" saltos=").I(p.mapRenderer.SaltosDelMapa157())
		ln.Tx(" repintados=").I(p.mapRenderer.RepintadosDelMapa157())

		Marcar159f(MarcaEnviaEstado159f)

		ln.Enviar(p.Logger)
	}

	Marcar159f(MarcaCierreTramo159f)

	// DC-002.159d: LA BASURA POR CUADRO, por fase. Cada 15 cuadros, una linea (ver `gc_159d.go`): sin sonda
	// (la PC) no imprime nada.
	// DC-002.159f: la misma linea lleva al final las MARCAS FINAS del tramo (ver `marcas_159f.go`) y se arma
	// sin `fmt` (`d2util.Linea159f`): las dos lineas del instrumento eran ~4,5 KB/cuadro de la basura que
	// este instrumento mide.
	EscribirElTramoGC159d(p.Logger, p.cuadros)

	Marcar159f(MarcaCierre159f)
}

// aplicarElPlan159: el plan de la corrida de ESTA tarjeta (DC-002.159b). Es corto a proposito: la escalera de
// siete tramos de DC-002.157 ya esta medida y sus tres palancas de presentacion se prenden de una.
//
//	cuadros  1        el mapa se pinta de verdad (las casillas todavia estan) y, al cerrarlo, se sueltan
//	                  las casillas y el arena de bloques: entra el heroe (ver `soltarElMapaYEntrarElHeroe`)
//	cuadros  2..      el mapa NO se repinta (soltado) y el heroe camina encima, con la memoria del lienzo
//
// El tramo A/B de 20 cuadros repintando el mapa con el heroe adentro que media DC-002.159 ya no existe: el
// numero (818 ms de mapa por cuadro con un ente) esta medido en su evidencia, y aca el mapa no se puede volver
// a pintar porque su memoria es justamente la que el heroe necesita.
func (p *PantallaDelPueblo) aplicarElPlan159(target d2interface.Surface) {
	// las tres de la presentacion (DC-002.157), prendidas desde el cuadro 1: aca no se miden, se usan
	if palancas, hay := target.(palancasDePresentacion157); hay {
		if !p.convRapida157 {
			palancas.ActivarConversionRapida157(true)
			p.convRapida157 = true
		}

		if !p.convFilas157 {
			palancas.ActivarConversionPorFilas157(true)
			p.convFilas157 = true
		}

		if !p.convZona157 {
			palancas.ActivarConversionPorZona157(true)
			p.convZona157 = true
		}

		if !p.quieta157 {
			palancas.ActivarPresentacionQuieta157(true)
			p.quieta157 = true
		}
	}
}

// aplicarElPlan157: prende las palancas que tocan en este cuadro (ver "EL PLAN DE LA CORRIDA"). Cada vez que
// una palanca cambia de estado se cierra el tramo anterior, asi que cada tramo se mide solo.
func (p *PantallaDelPueblo) aplicarElPlan157(target d2interface.Surface) {
	// ---- 1. el mapa estatico (la palanca de DC-002.139, en el renderer del motor) ----
	// Del cuadro 21 al 120 el mapa no se repinta; desde el 121 vuelve a pintarse (el A/B del censo, ver el
	// plan). La condicion incluye el techo para que el ultimo tramo no lo vuelva a prender.
	estatico := p.cuadros >= desdeMapaEstatico157 && p.cuadros < desdeElMapaDeNuevo157
	if estatico != p.mapaEstatico157 {
		p.mapRenderer.ActivarMapaEstatico(estatico)

		if !estatico {
			p.mapRenderer.OlvidarElMapa157()
		}

		p.mapaEstatico157 = estatico
		p.cerrarElTramo157("mapa-estatico", estatico)
	}

	// ---- 1b. el ultimo tramo: el mapa de nuevo y sin censo (el A/B del instrumento) ----
	if p.cuadros == desdeElMapaDeNuevo157 {
		p.elMapaDeNuevo157()
	}

	// ---- 2. las tres de la presentacion (las pide a la Surface; en la PC no existen y no pasa nada) ----
	palancas, hay := target.(palancasDePresentacion157)
	if !hay {
		return
	}

	if rapida := p.cuadros >= desdeConversionRapida157; rapida != p.convRapida157 {
		palancas.ActivarConversionRapida157(rapida)
		p.convRapida157 = rapida
		p.cerrarElTramo157("conversion-rapida", rapida)
	}

	if filas := p.cuadros >= desdeConversionPorFilas157; filas != p.convFilas157 {
		palancas.ActivarConversionPorFilas157(filas)
		p.convFilas157 = filas
		p.cerrarElTramo157("conversion-por-filas", filas)
	}

	if zona := p.cuadros >= desdeConversionPorZona157; zona != p.convZona157 {
		palancas.ActivarConversionPorZona157(zona)
		p.convZona157 = zona
		p.cerrarElTramo157("conversion-por-zona", zona)
	}

	if quieta := p.cuadros >= desdePresentacionQuieta157; quieta != p.quieta157 {
		palancas.ActivarPresentacionQuieta157(quieta)
		p.quieta157 = quieta
		p.cerrarElTramo157("presentacion-quieta", quieta)
	}
}

// elMapaDeNuevo157: el ultimo tramo del plan. Apaga el cronometro por tile y vuelve a prender el repintado
// del mapa, para medir el mapa PELADO (sin instrumento) y comparable con DC-002.156. Es tambien la prueba de
// que apagar el salto vuelve a pintar: si el cuadro siguiente no repinta, `repintados` no sube.
func (p *PantallaDelPueblo) elMapaDeNuevo157() {
	d2maprenderer.FijarRelojDeLosTiles(nil)

	p.mapRenderer.ActivarMapaEstatico(false)
	p.mapRenderer.OlvidarElMapa157()
	p.mapaEstatico157 = false

	p.cerrarElTramo157("el-mapa-de-nuevo-sin-censo", true)
}

// cerrarElTramo157: informa el tramo que se cierra (promedio de ms del mapa, saltos/repintados del renderer
// y el censo por tile del tramo) y abre el siguiente con la palanca `etiqueta` ya en `valor`.
func (p *PantallaDelPueblo) cerrarElTramo157(etiqueta string, valor bool) {
	estado := "off"
	if valor {
		estado = "on"
	}

	p.Infof("TRAMO157 cierra=%q cuadros=%d mapa_us_prom=%d mapa_us_total=%d saltos=%d repintados=%d",
		p.tramo157Etiqueta, p.tramo157Cuadros, promedio157(p.tramo157MapaUs, p.tramo157Cuadros),
		p.tramo157MapaUs, p.mapRenderer.SaltosDelMapa157(), p.mapRenderer.RepintadosDelMapa157())

	p.informarElCenso157()

	p.tramo157Etiqueta = etiqueta + "=" + estado
	p.tramo157Cuadros = 0
	p.tramo157MapaUs = 0

	p.Infof("TRAMO157 abre=%q (desde el cuadro %d)", p.tramo157Etiqueta, p.cuadros)
}

// informarElCenso157: el censo por tile del renderer (`d2maprenderer.cronometro_135.go`), que parte el
// costo de pintar el mapa en cache / vista / estado / pintado y en las cuatro pasadas. Es el numero con el
// que se decide que atacar DENTRO del mapa cuando la camara se mueva (la tarjeta del jugador).
func (p *PantallaDelPueblo) informarElCenso157() {
	c := d2maprenderer.CensoDeLosTiles135()
	if c.Cuadros == 0 {
		return
	}

	p.Infof("CENSO157 cuadros=%d tiles=%d imagenes=%d cache_us=%d vista_us=%d estado_us=%d pintado_us=%d "+
		"p1_us=%d p2_us=%d p3_us=%d p4_us=%d indice_us=%d fondos_us=%d suma_pasos_us=%d suma_bloques_us=%d",
		c.Cuadros, c.Tiles, c.Imagenes,
		c.Us[d2maprenderer.BloqueCache135], c.Us[d2maprenderer.BloqueVista135],
		c.Us[d2maprenderer.BloqueEstado135], c.Us[d2maprenderer.BloquePintado135],
		c.UsPaso[d2maprenderer.Paso1Plantas135], c.UsPaso[d2maprenderer.Paso2Sombras135],
		c.UsPaso[d2maprenderer.Paso3Paredes135], c.UsPaso[d2maprenderer.Paso4Techos135],
		c.UsPaso[d2maprenderer.PasoIndice135], c.UsPaso[d2maprenderer.PasoFondos135],
		c.SumaPasos, c.SumaBloques)

	d2maprenderer.ReiniciarCensoDeLosTiles135()
}

// promedio157: la division entera de toda la vida, con el cero cuidado.
func promedio157(total, n int) int {
	if n == 0 {
		return 0
	}

	return total / n
}

// Advance avanza el motor: la cruceta del heroe, el mapa (animaciones de tiles y entes) y el renderer (la
// cache). La cruceta va PRIMERO: el motor mueve al ente en su `Advance`, asi que aca solo se le pide el
// camino que va a caminar este cuadro.
func (p *PantallaDelPueblo) Advance(elapsed float64) error {
	// DC-002.159d: abre el cuadro para la sonda de la basura (no-op sin sonda: la PC).
	AbrirElCuadroGC159d()

	// DC-002.159f: las marcas finas del cuadro (ver marcas_159f.go). Esta cierra el tramo de la presentacion
	// del cuadro anterior, que es lo que la sonda de las fases llama `presente`.
	Marcar159f(MarcaControl159f)

	// DC-002.159d (SONDA DE CONTROL): 1 KB por cuadro que se tira al cuadro siguiente. Sirve para separar dos
	// causas muy distintas de que el vivo crezca: que el recolector NO pueda llevarase lo que ya nadie apunta
	// (en ese caso esto suma 1024 B/cuadro en la fase `control` de la linea GC159D), o que haya UNA estructura
	// concreta que crece (en ese caso esto no mueve la aguja y hay que buscarla). Se apaga poniendo el tamano
	// en 0 (`basuraDeControl159dBytes`).
	if basuraDeControl159dBytes > 0 {
		basuraDeControl159d = make([]byte, basuraDeControl159dBytes)
	}

	if p.heroe != nil {
		p.heroe.Avanzar()
	}

	MarcarFaseGC159d(faseMotor159d)
	Marcar159f(MarcaMotor159f)

	if p.engine != nil {
		p.engine.Advance(elapsed)
	}

	MarcarFaseGC159d(faseRenderer159d)
	Marcar159f(MarcaRenderer159f)

	if p.mapRenderer != nil {
		p.mapRenderer.Advance(elapsed)
	}

	MarcarFaseGC159d(fasePlan159d)

	return nil
}

// msDesde: milisegundos entre `desde` y ahora (d2util.Now es el reloj en segundos del motor).
func (p *PantallaDelPueblo) msDesde(desde float64) int {
	return int((d2util.Now() - desde) * 1000.0)
}

// censoDelPueblo: cuantas casillas quedaron con piso y con pared. Es el numero que dice que el pueblo se
// compuso ENTERO (la referencia del motor: 2.337 pisos y 338 paredes en el pueblo del acto 1, DC-002.88).
func censoDelPueblo(engine *d2mapengine.MapEngine) (pisos, paredes int) {
	juego := engine.Tiles()
	for i := range *juego {
		casilla := &(*juego)[i]

		if len(casilla.Components.Floors) > 0 {
			piso := &casilla.Components.Floors[0]
			if !piso.Hidden() && piso.Prop1 != 0 {
				pisos++
			}
		}

		if len(casilla.Components.Walls) > 0 {
			pared := &casilla.Components.Walls[0]
			if !pared.Hidden() && pared.Prop1 != 0 {
				paredes++
			}
		}
	}

	return pisos, paredes
}

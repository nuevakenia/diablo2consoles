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
	if soltadas := p.asset.SoltarFilasDeLasTablas([]string{d2resource.LevelType, d2resource.LevelPreset});
		soltadas.Tablas > 0 {
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

	// ============================== EL HEROE (DC-002.159) ==============================
	// OJO CON EL LUGAR, MEDIDO: esto va ANTES del renderer y no despues. El mapa compuesto deja la arena de
	// Go en 1.413.120 B libres, y el renderer se lleva ~970 KB de golpe (la primera cache): creando el heroe
	// despues, el `LoadComposite` del MPQ pide un segmento de pila y el arranque muere con
	// `stack_alloc: out of memory` (medido: arena usada 10.612.960 de 10.653.824, 40.864 libres, cinco
	// reinicios del emulador). Con el heroe primero sobra arena y el renderer entra igual.
	// La camara no hace falta para crearlo: el compuesto se arma igual, y `ConMapa` se la pasa despues.
	if p.conHeroe {
		p.avisar("heroe:empieza")

		p.plan159 = true

		heroe, err := NuevoHeroeDelGate0(p.asset, p.engine, nil, p.term, p.Logger,
			p.entrada, p.refrescarEntrada, p.compuestoDelHeroe)
		if err != nil {
			p.Errorf("HEROE159 SIN-HEROE: %v", err)
			p.avisar("heroe:error")
		} else {
			p.heroe = heroe
			p.Infof("HEROE159 LISTO %s", p.heroe.Estado())
			p.avisar("heroe:listo")
		}
	}

	// las opciones van ANTES de crear el renderer: la cache se arma (o no) en el constructor
	p.mapRenderer = d2maprenderer.CreateMapRenderer(p.asset, p.renderer, p.engine, p.term, p.nivelDeLog,
		0.0, 0.0, p.opciones...)

	p.avisar("mapa:renderer-creado")

	// DC-002.159: el mapa se pinta una vez y el ente va aparte (sin esto un solo ente apaga el mapa estatico).
	// El salto del mapa arranca APAGADO: los primeros cuadros son el A/B de la tarjeta (el cuadro con el heroe
	// y el mapa repintandose, que es el "no puede volver a 800") y el plan lo prende en el cuadro 21.
	if p.conHeroe {
		p.mapRenderer.ActivarEntesAparte159(true)
		p.mapRenderer.ActivarMapaEstatico(false)
		p.mapaEstatico157 = false
	}

	posicion := d2vector.NewPosition(p.mapRenderer.WorldToOrtho(p.engine.GetCenterPosition()))
	p.mapRenderer.SetCameraPosition(&posicion)

	loading.Progress(0.90)

	p.Infof("renderer del mapa listo: cuadro %dx%d", d2display.W(), d2display.H())
	p.avisar("mapa:listo")

	if p.heroe != nil {
		// el heroe ya existe desde antes del renderer (ver el bloque de arriba): recien ahora hay camara
		p.heroe.ConMapa(p.mapRenderer)
	}
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

	t0 := d2util.Now()

	// cuantos repintados llevaba el renderer: el heroe necesita saber si este cuadro el mapa se pinto entero
	// (entonces no hay rastro que borrar) o se salto (entonces hay que devolver el recuadro de abajo).
	repintadosAntes := p.mapRenderer.RepintadosDelMapa157()

	// el mundo ocupa la pantalla entera: se compensa el origen de la columna (con AnchorBottom es 0,0)
	ox, oy := d2display.ColumnOrigin()
	target.PushTranslation(-ox, -oy)
	p.mapRenderer.Render(target)

	// ============================== EL HEROE (DC-002.159) ==============================
	// El mapa ya esta en el lienzo (pintado o conservado del cuadro anterior): el heroe va encima. Si la
	// Surface sabe guardar recuadros (la consola), el heroe se borra solo; si no (la PC), esto no hace nada
	// porque el renderer ya lo dibujo dentro de sus pasadas.
	if p.heroe != nil {
		if lienzo, hay := target.(lienzoConMemoria159); hay {
			p.heroe.ConMemoria(lienzo)
		}

		p.heroe.DibujarSobreElMapa(target, p.mapRenderer.RepintadosDelMapa157() != repintadosAntes)
	}

	target.Pop()

	p.msDelCuadro = p.msDesde(t0)

	// DC-002.157: el tramo abierto acumula los ms del mapa y lo que el renderer se salto / repintio. El
	// tramo se informa cuando entra la palanca siguiente (`informeDelTramo157`).
	p.tramo157Cuadros++
	p.tramo157MapaUs += p.msDelCuadro

	if p.cuadros == 1 {
		p.msPrimerCuadro = p.msDelCuadro
	}

	if p.cuadros == 1 || p.cuadros == 2 || p.cuadros == 11 || p.cuadros == 31 || p.cuadros == 60 ||
		p.cuadros == 120 || p.cuadros == cuadrosQueSeInforman {
		p.Infof("CUADRO %d ms=%d (primer cuadro %d ms)", p.cuadros, p.msDelCuadro, p.msPrimerCuadro)
		p.avisar("mapa:cuadro")
	}

	// el estado del heroe cada 15 cuadros: es la linea con la que se cruzan las capturas (posicion, direccion
	// y modo de animacion). Con el heroe quieto la linea sale igual y no ensucia nada.
	if p.heroe != nil && p.cuadros%15 == 0 {
		p.Infof("HEROE159 cuadro=%d %s saltos=%d repintados=%d", p.cuadros, p.heroe.Estado(),
			p.mapRenderer.SaltosDelMapa157(), p.mapRenderer.RepintadosDelMapa157())
	}
}

// aplicarElPlan159: el plan de la corrida de ESTA tarjeta. Es corto a proposito -- la escalera de siete
// tramos de DC-002.157 ya esta medida y sus tres palancas de presentacion se prenden de una:
//
//	cuadros  1..20   el mapa se repinta cada cuadro (el "sin palanca", con el heroe adentro)
//	cuadros 21..     + mapa estatico y entes aparte: el mapa se pinta una vez y el heroe se dibuja encima
//
// La diferencia entre los dos tramos ES el numero de la tarjeta: el muro de los 818 ms y la palanca que lo
// esquiva con un ente caminando.
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

	// ---- el mapa: repintado hasta el cuadro 20, estatico desde el 21 ----
	if estatico := p.cuadros >= desdeMapaEstatico157; estatico != p.mapaEstatico157 {
		p.mapRenderer.ActivarMapaEstatico(estatico)

		if !estatico {
			p.mapRenderer.OlvidarElMapa157()
		}

		p.mapaEstatico157 = estatico
		p.cerrarElTramo157("mapa-estatico", estatico)
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
	if p.heroe != nil {
		p.heroe.Avanzar()
	}

	if p.engine != nil {
		p.engine.Advance(elapsed)
	}

	if p.mapRenderer != nil {
		p.mapRenderer.Advance(elapsed)
	}

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

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

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2display"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2enum"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2interface"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2math/d2vector"
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

	cuadros        int
	msDelCuadro    int
	msPrimerCuadro int
	msTotal        int
	pisos          int
	paredes        int

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

	// las opciones van ANTES de crear el renderer: la cache se arma (o no) en el constructor
	p.mapRenderer = d2maprenderer.CreateMapRenderer(p.asset, p.renderer, p.engine, p.term, p.nivelDeLog,
		0.0, 0.0, p.opciones...)

	p.avisar("mapa:renderer-creado")

	posicion := d2vector.NewPosition(p.mapRenderer.WorldToOrtho(p.engine.GetCenterPosition()))
	p.mapRenderer.SetCameraPosition(&posicion)

	loading.Progress(0.90)

	p.Infof("renderer del mapa listo: cuadro %dx%d", d2display.W(), d2display.H())
	p.avisar("mapa:listo")
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

	t0 := d2util.Now()

	// el mundo ocupa la pantalla entera: se compensa el origen de la columna (con AnchorBottom es 0,0)
	ox, oy := d2display.ColumnOrigin()
	target.PushTranslation(-ox, -oy)
	p.mapRenderer.Render(target)
	target.Pop()

	p.msDelCuadro = p.msDesde(t0)
	p.cuadros++

	if p.cuadros == 1 {
		p.msPrimerCuadro = p.msDelCuadro
	}

	if p.cuadros == 1 || p.cuadros == 2 || p.cuadros == 11 || p.cuadros == 31 || p.cuadros == 60 ||
		p.cuadros == 120 || p.cuadros == cuadrosQueSeInforman {
		p.Infof("CUADRO %d ms=%d (primer cuadro %d ms)", p.cuadros, p.msDelCuadro, p.msPrimerCuadro)
		p.avisar("mapa:cuadro")
	}
}

// Advance avanza el motor: el mapa (animaciones de tiles) y el renderer (la cache).
func (p *PantallaDelPueblo) Advance(elapsed float64) error {
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

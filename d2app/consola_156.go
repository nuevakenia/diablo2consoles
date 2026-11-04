//go:build dreamcast

package d2app

// DC-002.156 -- EL ARRANQUE DE LA CONSOLA DEL GATE 0: `d2app` SIN LAS PANTALLAS NI LOS SISTEMAS QUE UN
// CUADRO DEL PUEBLO NO ABRE.
//
// POR QUE EXISTE. DC-002.155 midio la pared con numeros: el binario del Gate 0 (`d2app` enlazando los 145
// paquetes del motor) pesa **11.155.164 B** cargados, el pueblo compone hasta el arena de bloques de los
// `.dt1` y ahi muere, y **faltan 2.188.360 B**. La causa no eran las tablas ni el semiespacio: era la
// IMAGEN -- 5,27 MB de sus paquetes son codigo que un cuadro del pueblo no ejecuta nunca (el heroe, los
// monstruos, las habilidades, el inventario, la red, el scripting y las 107 pantallas que no son esta).
//
// LO QUE HACE ESTE ARCHIVO. Ningun recorte del enlazador (`-Wl,--gc-sections` no baja el binario: medido en
// DC-002.14). Lo que hace es NO COMPILAR lo que no se usa, y como gccgo NO respeta `//go:build` (compila
// todo lo que no se excluya por NOMBRE, medido en DC-002.42), el camino es este archivo:
//
//	d2app/app.go y compania     -> camino de la PC (menus, heroe, red, captura, perfilado): NO entran
//	d2app/consola_156.go        -> el arranque de la consola: ESTE archivo (lo elige el build por nombre)
//	d2app/consola_gate0.go      -> las palancas del arranque y la pantalla del pueblo (DC-002.155): entra
//	d2game/d2gamescreen/*.go    -> solo `pantalla_mapa_155.go` + `prefijo_log_156.go`: entran
//
// La lista de archivos la decide `scripts/console/palancas-del-build.sh` con la palanca `DC_GATE0=1`, y la
// de paquetes enlazados sale del ALCANCE de esos archivos (`scripts/console/alcance-156.py`): lo que no se
// alcanza desde el main, no entra a la imagen.
//
// QUE HACE EL ARRANQUE, EN EL ORDEN DE LA PC (y por que se conserva el orden): las dos carpetas de
// configuracion como fuentes de assets, la configuracion del juego (que en la consola viene escrita en el
// romdisk por `console/dc/gen-config-consola`), los MPQ del disco como fuentes, el idioma, las TABLAS que
// el nivel pide (DC-002.86: LvlTypes y LvlPrest), el GUI sin sus dos animaciones (3,15 MB de RGBA), el
// ScreenManager del motor, el audio, las tablas de texto (que el pueblo no usa) y la UI. Despues, la
// pantalla del pueblo y el bucle del renderer. Es el mismo arranque que midio DC-002.155, sin los pasos que
// en la consola no existen (dialogos de primera vez, gamepad, perfiles, comandos de terminal, captura).
//
// LA PC NO CAMBIA UNA LINEA: este archivo lleva `//go:build dreamcast` y el build de la consola elige los
// archivos POR NOMBRE, asi que el binario del host no lo compila nunca (y `go build ./d2app` lo ignora).

import (
	"encoding/json"
	"fmt"
	"image/color"
	"path/filepath"
	"runtime"

	"github.com/OpenDiablo2/OpenDiablo2/d2app/d2setup"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2display"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2enum"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2animdata"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2interface"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2loader/asset/types"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2resource"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2util"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2asset"
	audioebiten "github.com/OpenDiablo2/OpenDiablo2/d2core/d2audio/ebiten"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2config"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2gui"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2input"
	renderebiten "github.com/OpenDiablo2/OpenDiablo2/d2core/d2render/ebiten"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2screen"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2term"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2ui"
)

const (
	prefijoDeLaApp = "App"
	// el titulo de la ventana del emulador: no se ve en la consola, pero es el mismo dato que la PC.
	tituloDeLaVentana = "OpenDiablo2 (consola)"
)

// Options: las opciones de linea de comandos de la PC. En la consola `os.Args` es un stub y `flag` no
// tiene de donde leer, asi que existe SOLO el nivel de log (el default del motor), que es lo unico que el
// arranque de consola mira.
type Options struct {
	LogLevel *d2util.LogLevel
}

// App es el arranque de la consola: los mismos componentes que la PC y nada mas.
type App struct {
	asset    *d2asset.AssetManager
	renderer d2interface.Renderer
	terminal d2interface.Terminal
	input    d2interface.InputManager
	audio    d2interface.AudioProvider
	ui       *d2ui.UIManager
	screen   *d2screen.ScreenManager
	gui      *d2gui.GuiManager
	config   *d2config.Configuration

	gitBranch string
	gitCommit string

	lastTime             float64
	lastScreenAdvance    float64
	timeScale            float64
	errorMessage         error
	gate0                *Gate0
	*d2util.Logger
	*Options
}

// Create arma la aplicacion: logger, nivel de log y el AssetManager del motor. Igual que la PC salvo que
// no hay `flag.Parse` (en la consola no hay argumentos) ni el perfilador.
func Create(gitBranch, gitCommit string) *App {
	runtime.LockOSThread()

	logger := d2util.NewLogger()
	logger.SetPrefix(prefijoDeLaApp)

	nivel := d2util.LogLevelDefault

	app := &App{
		Logger:    logger,
		gitBranch: gitBranch,
		gitCommit: gitCommit,
		Options:   &Options{LogLevel: &nivel},
	}

	app.Infof("OpenDiablo2 - Open source Diablo 2 engine (arranque de consola, DC-002.156)")
	app.SetLevel(*app.Options.LogLevel)

	app.asset, app.errorMessage = d2asset.NewAssetManager(*app.Options.LogLevel)

	return app
}

// Run es `d2app.Run` de la PC, paso por paso, sin lo que en la consola no existe.
func (a *App) Run() error {
	// las dos carpetas de configuracion como fuentes de assets (la del romdisk es la que trae config.json)
	_ = a.asset.AddSource(filepath.Dir(d2config.LocalConfigPath()), types.AssetSourceFileSystem)
	_ = a.asset.AddSource(filepath.Dir(d2config.DefaultConfigPath()), types.AssetSourceFileSystem)

	config, err := a.cargarConfig()
	if err != nil {
		return err
	}

	a.config = config
	a.avisarEtapa("app:config-leida")

	// El arranque de la PC abre dialogos para elegir la carpeta del juego (`d2setup.EnsureGameFiles`). En la
	// consola no hay a quien preguntar: la configuracion completa la escribe `gen-config-consola` en el
	// romdisk. Si no esta completa se dice CUAL es el campo que falta, que es lo que ahorra la tarde.
	if !d2setup.Complete(a.config.MpqPath, a.config.MpqLoadOrder) {
		return fmt.Errorf("la configuracion del arranque no esta completa: MpqPath=%q MpqLoadOrder=%v",
			a.config.MpqPath, a.config.MpqLoadOrder)
	}

	if a.errorMessage != nil {
		return a.errorMessage
	}

	if err := a.levantarElMotor(); err != nil {
		return err
	}

	if err := a.inicializar(); err != nil {
		return err
	}

	// el Gate 0 va DERECHO al pueblo: sin menu, sin video, sin personaje (DC-002.155)
	if a.gate0 != nil {
		a.avisarEtapa("app:antes-de-la-pantalla")
		a.ToPantallaDelPueblo(a.gate0.Region, a.gate0.Preset, a.gate0.ConEntidades,
			a.gate0.Instrumento, a.gate0.OpcionesDelMapa...)
	}

	a.avisarEtapa("app:bucle")

	return a.renderer.Run(a.update, a.advance, d2display.W(), d2display.H(), tituloDeLaVentana)
}

// cargarConfig lee `config.json` de las fuentes del AssetManager (en la consola: `/rd/OpenDiablo2`). Es el
// `LoadConfig` de la PC sin el camino "crear el default": en la consola no se puede escribir.
func (a *App) cargarConfig() (*d2config.Configuration, error) {
	config := &d2config.Configuration{}
	config.SetPath(d2config.DefaultConfigPath())

	nombre := filepath.Base(d2config.DefaultConfigPath())

	configAsset, _ := a.asset.LoadAsset(nombre)
	if configAsset == nil {
		return nil, fmt.Errorf("no encuentro %s en las fuentes del romdisk (lo escribe gen-config-consola)", nombre)
	}

	if err := json.NewDecoder(configAsset).Decode(config); err != nil {
		return nil, err
	}

	a.Infof("configuracion cargada de %s: MpqPath=%s, %d MPQ", config.Path(), config.MpqPath,
		len(config.MpqLoadOrder))

	return config, nil
}

// levantarElMotor: el renderer del motor (el shim de la consola), el terminal, la entrada, el audio y la UI.
// El tamano logico lo fija `d2display.Base` y el `Layout` del renderer se encarga despues (el PVR ya esta
// arrancado por el main: `ebiten.IniciarPantalla(800, 600)`, DC-002.22).
func (a *App) levantarElMotor() error {
	renderer, err := renderebiten.CreateRenderer(a.config)
	if err != nil {
		return err
	}

	a.renderer = renderer
	d2display.Set(d2display.Base)

	renderer.SetFullscreenHook(func(full bool) {
		a.config.SetFullscreenChoice(full)
		_ = a.config.Save()
	})

	a.input = d2input.NewInputManager()

	term, err := d2term.New(a.input)
	if err != nil {
		return err
	}

	a.terminal = term
	a.audio = audioebiten.CreateAudio(*a.Options.LogLevel, a.asset)
	a.ui = d2ui.NewUIManager(a.asset, a.renderer, a.input, *a.Options.LogLevel, a.audio)

	return nil
}

// inicializar es el `initialize` de la PC con las palancas del Gate 0 (DC-002.155) y sin los pasos que la
// consola no tiene: gamepad, perfiles, accesibilidad, comandos de terminal, opciones del menu de escape y
// la opcion de cinematicas.
func (a *App) inicializar() error {
	if err := a.agregarLosMpq(); err != nil {
		return err
	}

	a.avisarEtapa("app:config")

	a.iniciarElIdioma()

	a.avisarEtapa("app:idioma")

	if err := a.cargarLasTablas(); err != nil {
		return err
	}

	a.avisarEtapa("app:tablas")

	a.timeScale = 1.0
	a.lastTime = d2util.Now()
	a.lastScreenAdvance = a.lastTime

	a.renderer.SetWindowIcon("d2logo.png")

	if a.terminal != nil {
		a.terminal.BindLogger()
	}

	// DC-002.155: el GUI de la consola del Gate 0 no carga sus dos animaciones (3,15 MB de RGBA)
	if a.sinGuiDeArranque() {
		d2gui.SinAnimacionesDeConsola(true)
	}

	gui, err := d2gui.CreateGuiManager(a.asset, *a.Options.LogLevel, a.input)
	if err != nil {
		return err
	}

	a.gui = gui
	a.avisarEtapa("app:gui")

	a.screen = d2screen.NewScreenManager(a.ui, *a.Options.LogLevel, a.gui)

	a.audio.SetVolumes(a.config.BgmVolume, a.config.SfxVolume)

	if err := a.cargarLasCadenas(); err != nil {
		return err
	}

	a.avisarEtapa("app:strings")

	// DC-002.155: `ui.Initialize` carga el sonido del boton (vive en `d2sfx.mpq`, que no va al disco de la
	// consola). El pueblo no tiene UI: se saltea entero.
	if !a.sinUiDeArranque() {
		a.ui.Initialize()
	}

	a.avisarEtapa("app:ui")

	return nil
}

// agregarLosMpq: la configuracion dice que MPQ se cargan y de que carpeta (en la consola, `/cd/juego`). Es
// el `initConfig` de la PC, con la relectura por indice que arreglo DC-002.39 (con `range` el largo del
// nombre salia de un registro y no de memoria).
func (a *App) agregarLosMpq() error {
	for i := 0; i < len(a.config.MpqLoadOrder); i++ {
		mpqName := a.config.MpqLoadOrder[i]

		cleanDir := filepath.Clean(a.config.MpqPath)
		srcPath := filepath.Join(cleanDir, mpqName)

		if err := a.asset.AddSource(srcPath, types.AssetSourceMPQ); err != nil {
			return fmt.Errorf("no puedo abrir %s (config %s): %v", srcPath, a.config.Path(), err)
		}
	}

	return nil
}

// iniciarElIdioma: el idioma del AssetManager y el del loader. Igual que la PC (en la consola las variables
// de entorno son stubs, asi que manda el idioma de la configuracion).
func (a *App) iniciarElIdioma() {
	a.asset.SetRequestedLanguage(a.config.Language)

	language := a.asset.LoadLanguage(d2resource.LocalLanguage)
	a.asset.Loader.SetLanguage(&language)

	charset := a.asset.FontDir()
	a.asset.Loader.SetCharset(&charset)
}

// cargarLasTablas: las tablas que el arranque pide (`tablasDelArranque`, DC-002.155: el pueblo vive de
// LvlTypes y LvlPrest) y nada de `animdata.d2` (sin entidades no hay animacion que consultar).
func (a *App) cargarLasTablas() error {
	for _, path := range a.tablasDelArranque(d2resource.DataDictionaries()) {
		a.avisarEtapa("tabla:empieza:" + path)

		if err := a.asset.LoadRecords(path); err != nil {
			return err
		}

		a.avisarEtapa("tabla:lista:" + path)
	}

	if a.sinAnimDataDeArranque() {
		a.Info("Gate0: animdata.d2 NO se carga (sin entidades no hay animacion)")

		return nil
	}

	animDataBytes, err := a.asset.LoadFile(d2resource.AnimationData)
	if err != nil {
		return err
	}

	// DC-002.159: con el heroe, el animdata se carga SOLO con las claves del personaje. El archivo entero
	// son 2.569 registros que en el host ocupan 2,17 MB de mapas de eventos; el semiespacio del recolector
	// de la consola son 2.097.152 B. El heroe del Gate 0 consulta ~60 claves (`AM*`: los modos de la
	// amazona) y nada mas: se recorren los 411.104 B igual, y se guardan solo esas.
	if a.gate0 != nil && a.gate0.Heroe {
		prefijos := d2animdata.PrefijosDeUnPersonaje(d2enum.HeroAmazon.GetToken())

		animData, dentro, fuera, err := d2animdata.LoadSelectivo(animDataBytes, prefijos)
		if err != nil {
			a.Errorf("no pude leer la animacion del heroe: %v", err)

			return nil
		}

		a.Infof("animdata del heroe: %d registros (de %d; prefijos=%v)", dentro, dentro+fuera, prefijos)
		a.asset.Records.Animation.Data = animData

		return nil
	}

	animData, err := d2animdata.Load(animDataBytes)
	if err != nil {
		a.Errorf("no pude leer la animacion: %v", err)
	}

	a.Infof("animdata: %d registros", animData.GetRecordsCount())

	a.asset.Records.Animation.Data = animData

	return nil
}

// cargarLasCadenas: las tres tablas de texto. El pueblo del Gate 0 no dibuja ni un texto: se saltean.
func (a *App) cargarLasCadenas() error {
	if a.sinStringsDeArranque() {
		a.Info("Gate0: las tablas de strings NO se cargan (el pueblo no dibuja texto)")

		return nil
	}

	for _, tablePath := range []string{
		d2resource.PatchStringTable,
		d2resource.ExpansionStringTable,
		d2resource.StringTable,
	} {
		if _, err := a.asset.LoadStringTable(tablePath); err != nil {
			return err
		}
	}

	return nil
}

// update dibuja: el screen manager del motor, la UI y el GUI, en el espacio de la pantalla (el mundo ocupa
// la pantalla entera: la pantalla del pueblo pide `AnchorBottom`).
func (a *App) update(target d2interface.Surface) error {
	a.render(target)

	if target.GetDepth() > 0 {
		return fmt.Errorf("detected surface stack leak")
	}

	return nil
}

func (a *App) render(target d2interface.Surface) {
	d2display.SetAnchor(a.screen.Anchor())

	if a.screen.Anchor() == d2display.AnchorCenter {
		target.Clear(color.Black)
	}

	ox, oy := d2display.ColumnOrigin()

	target.PushTranslation(ox, oy)
	a.screen.Render(target)
	a.ui.Render(target)
	target.Pop()

	if err := a.gui.Render(target); err != nil {
		a.Errorf("gui: %v", err)
		return
	}

	if a.terminal != nil {
		if err := a.terminal.Render(target); err != nil {
			a.Errorf("terminal: %v", err)
		}
	}
}

// advance avanza el motor (la pantalla del pueblo avanza el mapa y la cache del renderer).
func (a *App) advance() error {
	current := d2util.Now()
	elapsed := (current - a.lastTime) * a.timeScale

	a.lastTime = current

	elapsedLastScreenAdvance := (current - a.lastScreenAdvance) * a.timeScale
	a.lastScreenAdvance = current

	if err := a.screen.Advance(elapsedLastScreenAdvance); err != nil {
		return err
	}

	a.ui.Advance(elapsed)

	if err := a.input.Advance(elapsed, current); err != nil {
		return err
	}

	if a.gui != nil {
		if err := a.gui.Advance(elapsed); err != nil {
			return err
		}
	}

	if a.terminal != nil {
		if err := a.terminal.Advance(elapsed); err != nil {
			return err
		}
	}

	return nil
}

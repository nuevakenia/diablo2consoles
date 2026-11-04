package d2app

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2loader/asset/types"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2animdata"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2resource"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2util"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2config"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2gui"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2screen"
	"github.com/OpenDiablo2/OpenDiablo2/d2game/d2gamescreen"
	"github.com/OpenDiablo2/OpenDiablo2/d2game/d2player"
)

func (a *App) initialize() error {
	if err := a.initConfig(a.config); err != nil {
		return err
	}

	a.avisarEtapa("app:config")

	a.initLanguage()

	a.avisarEtapa("app:idioma")

	if err := a.initDataDictionaries(); err != nil {
		return err
	}

	a.avisarEtapa("app:tablas")

	a.timeScale = 1.0
	a.lastTime = d2util.Now()
	a.lastScreenAdvance = a.lastTime

	a.renderer.SetWindowIcon("d2logo.png")
	a.terminal.BindLogger()
	a.initTerminalCommands()

	// DC-002.155: el GUI de la consola del Gate 0 no carga sus dos animaciones (3,15 MB de RGBA)
	if a.sinGuiDeArranque() {
		d2gui.SinAnimacionesDeConsola(true)
	}

	gui, err := d2gui.CreateGuiManager(a.asset, *a.Options.LogLevel, a.inputManager)
	if err != nil {
		return err
	}

	a.guiManager = gui
	a.avisarEtapa("app:gui")

	a.screen = d2screen.NewScreenManager(a.ui, *a.Options.LogLevel, a.guiManager)

	a.audio.SetVolumes(a.config.BgmVolume, a.config.SfxVolume)
	a.applySpeechOption()
	d2player.SetOptionsBackend(optionsBackend{a})
	a.loadGamepadProfiles()
	a.applyAccessibility("")
	d2gamescreen.SetCinematicsOption(a.config.Cinematics)

	if err := a.loadStrings(); err != nil {
		return err
	}

	a.avisarEtapa("app:strings")

	// DC-002.155: `ui.Initialize` carga el sonido del boton (vive en `d2sfx.mpq`, que no va al disco de la
	// consola) y engancha los handlers de la UI. El pueblo no tiene UI: se saltea entero.
	if !a.sinUiDeArranque() {
		a.ui.Initialize()
	}

	a.avisarEtapa("app:ui")

	return nil
}

const (
	fmtErrSourceNotFound = `file not found: %q

Please check your config file at %q

Also, verify that the MPQ files exist at %q

Capitalization in the file name matters.
`
)

func (a *App) initConfig(config *d2config.Configuration) error {
	a.config = config

	// DC-002.39: se recorre por INDICE y el nombre se RELEE del arreglo vivo en cada vuelta, en vez de
	// `for _, mpqName := range a.config.MpqLoadOrder`. Medido en la consola: con el `range`, la vuelta 3
	// leia el nombre con el puntero correcto y el LARGO de otro lado (49, -1795502946 o 0 segun la corrida)
	// apenas `AddSource` disparaba una recoleccion del GC; el arreglo vivo del `MpqLoadOrder` tenia los
	// largos correctos y el par {puntero, largo} leido no existe en los 16 MB de RAM, o sea el largo venia
	// de un registro y no de memoria. El decodificador JSON y el GC quedan descartados como causa: entrega
	// los 3 nombres y mantiene la cabecera y el arreglo. Releyendo por indice el nombre sale bien.
	// Evidencia: console/dc/evidencia/dc-002.39-nombre-corrompido.txt.
	for i := 0; i < len(a.config.MpqLoadOrder); i++ {
		mpqName := a.config.MpqLoadOrder[i]

		cleanDir := filepath.Clean(a.config.MpqPath)
		srcPath := filepath.Join(cleanDir, mpqName)

		err := a.asset.AddSource(srcPath, types.AssetSourceMPQ)
		if err != nil {
			// nolint:stylecheck // we want a multiline error message here..
			return fmt.Errorf(fmtErrSourceNotFound, srcPath, a.config.Path(), a.config.MpqPath)
		}
	}

	return nil
}

func (a *App) initLanguage() {
	requested := a.config.Language

	if env := os.Getenv("OD2_LANGUAGE"); env != "" {
		requested = env
	}

	if a.Options != nil && a.Options.Language != nil && *a.Options.Language != "" {
		requested = *a.Options.Language
	}

	a.asset.SetRequestedLanguage(requested)

	a.language = a.asset.LoadLanguage(d2resource.LocalLanguage)
	a.asset.Loader.SetLanguage(&a.language)

	a.charset = a.asset.FontDir()
	a.asset.Loader.SetCharset(&a.charset)
}

func (a *App) initDataDictionaries() error {
	dictPaths := a.tablasDelArranque(d2resource.DataDictionaries())

	a.Info("Initializing asset manager")

	for _, path := range dictPaths {
		a.avisarEtapa("tabla:empieza:" + path)

		err := a.asset.LoadRecords(path)
		if err != nil {
			return err
		}

		a.avisarEtapa("tabla:lista:" + path)
	}

	// DC-002.155: sin entidades no hay animacion que consultar, y `animdata.d2` son 2,17 MB de registros
	// (medido en el host, `medidas/arranque-app-155`). En la consola del Gate 0 se saltea.
	if a.sinAnimDataDeArranque() {
		a.Info("Gate0: animdata.d2 NO se carga (sin entidades no hay animacion)")

		return nil
	}

	err := a.initAnimationData(d2resource.AnimationData)
	if err != nil {
		return err
	}

	a.avisarEtapa("app:animdata")

	return nil
}

const (
	fmtLoadAnimData = "loading animation data from: %s"
)

func (a *App) initAnimationData(path string) error {
	animDataBytes, err := a.asset.LoadFile(path)
	if err != nil {
		return err
	}

	a.Debugf(fmtLoadAnimData, path)

	animData, err := d2animdata.Load(animDataBytes)
	if err != nil {
		a.Error(err.Error())
	}

	a.Infof("Loaded %d animation data records", animData.GetRecordsCount())

	a.asset.Records.Animation.Data = animData

	return nil
}

func (a *App) loadStrings() error {
	// DC-002.155: el pueblo del Gate 0 no tiene ni un texto en pantalla, y las tres tablas son 2,67 MB de
	// registros medidos en el host. Se saltean enteras.
	if a.sinStringsDeArranque() {
		a.Info("Gate0: las tablas de strings NO se cargan (el pueblo no dibuja texto)")

		return nil
	}

	tablePaths := []string{
		d2resource.PatchStringTable,
		d2resource.ExpansionStringTable,
		d2resource.StringTable,
	}

	for _, tablePath := range tablePaths {
		_, err := a.asset.LoadStringTable(tablePath)
		if err != nil {
			if tablePath == d2resource.ExpansionStringTable {
				// non-English installs fold the expansion strings into string.tbl
				a.Warning("skipping missing expansion strings (no Lord of Destruction files)")
				continue
			}

			if tablePath == d2resource.PatchStringTable {
				a.Warning("skipping missing patch strings (unpatched install)")
				continue
			}

			return err
		}
	}

	return nil
}

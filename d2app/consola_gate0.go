package d2app

// DC-002.155 -- EL ARRANQUE DEL GATE 0 EN LA CONSOLA: MENOS TABLAS, MENOS ASSETS, Y EL PUEBLO.
//
// POR QUE EXISTE. `d2app.Run` es el arranque de la PC: carga las 85 tablas de `d2resource.DataDictionaries`
// (15-16,7 MB medidos en el host, DC-002.82), el `animdata.d2` (2,17 MB de registros + 411 KB crudos),
// las tres tablas de strings (2,67 MB), la pantalla de carga (3,11 MB de RGBA) y el sonido del boton de la
// UI. Todo eso, junto, son ~24 MB. El monton de C del binario del motor en la consola son 2.450.496 B y el
// semiespacio 1.572.864 B (DC-002.93 con 1536 KB + `.eh_frame` fuera): **el arranque del motor no entra, y
// no entra por los ASSETS, no por el codigo**.
//
// LO QUE HACE ESTE ARCHIVO. Deja que un main de la consola pida, ANTES de `Run`:
//
//	Tablas:          SOLO las que el nivel necesita (DC-002.86: el pueblo vive de LvlTypes y LvlPrest)
//	SinAnimData:     no cargar `animdata.d2` (sin entidades no hay animacion que consultar)
//	SinStrings:      no cargar las tablas de texto (el pueblo no tiene ni un texto en pantalla)
//	SinGui:          no cargar el cursor ni la pantalla de carga (3,15 MB de RGBA)
//	SinUi:           no pedir el sonido del boton (el .wav vive en `d2sfx.mpq`, que no va al disco)
//	Region/Preset:   la pantalla del pueblo (`d2gamescreen.PantallaDelPueblo`) en vez del menu
//
// NADA DE ESTO CAMBIA EL CAMINO DE LA PC: los campos van en cero en el `App` que arma `d2app.Create` y cada
// palanca se pregunta con un `if`. El binario del host no las prende nunca.
//
// EL CONTRATO CON EL QUE MIDE: `Instrumento` se llama en cada etapa del arranque (config, cada tabla, cada
// paso del mapa, cada cuadro informado) para que el main de la consola imprima el monton de C EN ESE
// MOMENTO. Sin eso, lo unico que se sabe al final es donde murio.

import (
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2enum"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2interface"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2map/d2maprenderer"
	"github.com/OpenDiablo2/OpenDiablo2/d2game/d2gamescreen"
)

// Gate0 es la configuracion del arranque de la consola. Se pasa con `(*App).ArrancarEnGate0`.
type Gate0 struct {
	// Tablas: los paths de `d2resource` que se cargan, en el orden en que se pidan. Vacio = TODAS (el
	// camino de la PC). Se filtra contra `d2resource.DataDictionaries()` para que un nombre que no existe
	// no rompa el arranque en silencio: si una tabla pedida no esta en la lista del motor, se avisa.
	Tablas []string

	// SinAnimData: no cargar `animdata.d2` (`initAnimationData`). Solo lo necesitan las entidades.
	SinAnimData bool

	// SinStrings: no cargar las tres tablas de texto (`loadStrings`).
	SinStrings bool

	// SinGui: no cargar las animaciones del GUI (cursor y pantalla de carga).
	SinGui bool

	// SinUi: no inicializar la UI (`ui.Initialize`, que carga el sonido del boton).
	SinUi bool

	// Region y Preset: el nivel que se compone y se pinta. RegionAct1Town + preset 1 es el pueblo.
	Region d2enum.RegionIdType
	Preset int

	// ConEntidades: componer con `PlaceStamp` (monstruos y objetos) en vez de solo las casillas. Pide las
	// tablas de las entidades y la animacion: en la consola del Gate 0 va apagado.
	ConEntidades bool

	// OpcionesDelMapa: las palancas del `d2maprenderer` (cache perezosa, indices, tramos de VRAM...).
	OpcionesDelMapa []d2maprenderer.Opcion

	// ============================== EL HEROE (DC-002.159) ==============================
	//
	// Heroe: la pantalla del pueblo lleva EL HEROE DEL MOTOR (la amazona, `d2mapentity.NuevoJugadorDeConsola`)
	// caminando con la cruceta. Pide dos cosas mas: `animdata.d2` (que con `SinAnimData` no se carga) y
	// `charstats.txt` en la lista de tablas.
	Heroe bool

	// Entrada: el servicio de entrada de la consola (el maple, `d2core/d2input/maple`), que traduce la
	// cruceta del control a las cuatro flechas del motor. `RefrescarEntrada`: el que lee el aparato antes de
	// preguntarle las teclas (en la consola, `ActualizarDesdeElControl`; el main del POC lo pasa, porque es
	// el unico que puede llamar al maple).
	Entrada          d2interface.InputService
	RefrescarEntrada func()

	// Instrumento: se llama en cada etapa (etapa es un texto corto). Puede ser nil.
	Instrumento func(etapa string)
}

// ArrancarEnGate0 deja el arranque configurado para la consola. Se llama DESPUES de `Create` y ANTES de
// `Run` (que es donde se inicializa todo).
func (a *App) ArrancarEnGate0(g Gate0) { a.gate0 = &g }

// avisar gate0: el instrumento, si hay.
func (a *App) avisarEtapa(etapa string) {
	if a.gate0 != nil && a.gate0.Instrumento != nil {
		a.gate0.Instrumento(etapa)
	}
}

// tablasDelArranque: las tablas que `initDataDictionaries` carga. Sin Gate0, todas (el camino del motor).
func (a *App) tablasDelArranque(todas []string) []string {
	if a.gate0 == nil || len(a.gate0.Tablas) == 0 {
		return todas
	}

	pedidas := make(map[string]bool, len(a.gate0.Tablas))
	for _, tabla := range a.gate0.Tablas {
		pedidas[tabla] = true
	}

	efectivas := make([]string, 0, len(a.gate0.Tablas))
	for _, tabla := range todas {
		if pedidas[tabla] {
			efectivas = append(efectivas, tabla)
			delete(pedidas, tabla)
		}
	}

	// las que se pidieron y el motor no conoce: se dicen, no se esconden (un path mal escrito es un cuadro
	// sin tiles y tres horas de busqueda)
	for tabla := range pedidas {
		a.Errorf("Gate0: la tabla %q no esta en d2resource.DataDictionaries(): no se carga", tabla)
	}

	return efectivas
}

// sinAnimDataDeArranque / sinStringsDeArranque / sinGuiDeArranque / sinUiDeArranque: las cuatro palancas.
func (a *App) sinAnimDataDeArranque() bool { return a.gate0 != nil && a.gate0.SinAnimData }
func (a *App) sinStringsDeArranque() bool  { return a.gate0 != nil && a.gate0.SinStrings }
func (a *App) sinGuiDeArranque() bool      { return a.gate0 != nil && a.gate0.SinGui }
func (a *App) sinUiDeArranque() bool       { return a.gate0 != nil && a.gate0.SinUi }

// ToPantallaDelPueblo pone la pantalla del Gate 0 como pantalla siguiente (la carga el screen manager en el
// primer `Advance`, igual que cualquier otra pantalla del motor).
func (a *App) ToPantallaDelPueblo(region d2enum.RegionIdType, preset int, conEntidades bool,
	instrumento func(etapa string), opciones ...d2maprenderer.Opcion) {
	pantalla := d2gamescreen.CreatePantallaDelPueblo(a.asset, a.renderer, a.terminal, *a.Options.LogLevel,
		region, preset, conEntidades, instrumento, opciones...)

	// DC-002.159: el heroe del motor con la cruceta, si el main lo pidio
	if a.gate0 != nil && a.gate0.Heroe {
		pantalla.ConHeroe(a.gate0.Entrada, a.gate0.RefrescarEntrada)
	}

	a.screen.SetNextScreen(pantalla)
}

package d2mapentity

// DC-002.159 -- EL JUGADOR DE LA CONSOLA CON EL COMPUESTO YA ARMADO.
//
// POR QUE EXISTE. Armar el compuesto del heroe (`LoadComposite`: el COF del MPQ + las capas `.dcc` del modo
// Neutral + la paleta) cuesta ~1 MB en la Dreamcast. El pueblo compuesto deja la arena de Go con 1,4 MB, y el
// renderer necesita ~890 KB mas: si el compuesto se arma DESPUES del mapa no entra (medido: mueren
// `gc_external_alloc: OOM` y `stack_alloc: out of memory`). Arrancandolo ANTES de componer el mapa sobran
// 4,8 MB. Lo unico que no se puede adelantar es el ENTE, porque el punto de arranque lo da el mapa ya
// compuesto (`GetStartPosition` recorre las casillas). De ahi esta puerta: el que llama arma el compuesto
// cuando hay lugar y crea el jugador cuando ya sabe donde va.
//
// `NuevoJugadorDeConsola` sigue existiendo y hace lo mismo de siempre (arma el compuesto y devuelve el
// jugador): esta variante solo separa los dos pasos. El cuerpo del ente es UNO solo (`jugadorDeConsola`, en
// jugador_consola.go), el de DC-002.76.
//
// DC-002.159b -- EL MODO INICIAL ES UN PARAMETRO. `SetMode` carga las capas `.dcc` del modo pedido y un modo
// del personaje pesa 442.601 B medidos (7 capas). El heroe del Gate 0 NO usa el modo Neutral: en el pueblo usa
// el de pueblo (`TN` parado, `TW` caminando). Arrancando en Neutral se pagaban DOS modos (~885 KB) en un
// semiespacio de 1.835.008 B: el recolector quedaba al 91 % y el cuadro se iba a 537 ms en recolecciones. Con el
// modo inicial como parametro, el que arma el compuesto puede PRIMIR el modo que se va a usar, cuando todavia
// hay 4,8 MB, y el ente nace ya en el.

import (
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2enum"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2asset"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2hero"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2inventory"
)

// NuevoJugadorDeConsolaCon arma el Player sobre un compuesto ya cargado (`compuesto` no puede ser nil; para
// que lo cargue esta `NuevoJugadorDeConsola`). Devuelve nil si el compuesto no puede entrar en el modo Neutral
// (mismo caso que el error de `NuevoJugadorDeConsola`).
func NuevoJugadorDeConsolaCon(
	compuesto *d2asset.Composite,
	id, nombre string,
	x, y, direccion int,
	heroe d2enum.Hero,
	stats *d2hero.HeroStatsState,
) *Player {
	return NuevoJugadorDeConsolaConModo(compuesto, id, nombre, x, y, direccion, heroe, stats,
		d2enum.PlayerAnimationModeNeutral)
}

// NuevoJugadorDeConsolaConModo (DC-002.159b): lo mismo que `NuevoJugadorDeConsolaCon` con el modo de animacion
// inicial elegido por el que llama. En el pueblo ese modo es `TownNeutral` (ver la cabecera del archivo).
func NuevoJugadorDeConsolaConModo(
	compuesto *d2asset.Composite,
	id, nombre string,
	x, y, direccion int,
	heroe d2enum.Hero,
	stats *d2hero.HeroStatsState,
	modoInicial d2enum.PlayerAnimationMode,
) *Player {
	if compuesto == nil {
		return nil
	}

	resultado := jugadorDeConsola(compuesto, id, nombre, x, y, direccion, heroe, stats)

	if err := compuesto.SetMode(modoInicial, resultado.Equipment.RightHand.GetWeaponClass()); err != nil {
		return nil
	}

	return resultado
}

// ArmaDelEquipamientoVacio (DC-002.159b): la clase de arma que `jugadorDeConsola` le pasa a `SetMode` con el
// equipamiento vacio (`hth`). Se exporta para que el que PRIME el compuesto antes de componer el mapa pida el
// MISMO modo que va a pedir el ente despues, sin repetir el literal ni adivinarlo.
func ArmaDelEquipamientoVacio() string {
	equipamiento := &d2inventory.CharacterEquipment{}

	return equipamiento.RightHand.GetWeaponClass()
}

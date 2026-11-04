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

import (
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2enum"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2asset"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2hero"
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
	if compuesto == nil {
		return nil
	}

	resultado := jugadorDeConsola(compuesto, id, nombre, x, y, direccion, heroe, stats)

	if err := compuesto.SetMode(d2enum.PlayerAnimationModeNeutral, resultado.Equipment.RightHand.GetWeaponClass()); err != nil {
		return nil
	}

	return resultado
}

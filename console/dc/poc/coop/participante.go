//go:build dreamcast

package main

// DC-002.76 -- EL JUGADOR QUE ENTRA EN LA CONSOLA: el `d2asset.Composite` del motor.
//
// POR QUE NO EL `d2mapentity.Player` COMPLETO, QUE ES LO QUE PEDIA LA TARJETA. Porque se midio y no entra:
// el tipo `Player` arrastra su cierre (d2hero, d2records, d2ui, d2item, d2inventory, d2hero...) y el BIN
// pasa de 2,74 MB (DC-002.75) a 4,87 MB; con ese BIN el monton del SH-4 se queda en 11,38 MB y la corrida
// muere CARGANDO EL PRIMER JUGADOR (log en console/dc/evidencia/dc-002.76-jugador-real.log: el GC se queda
// sin semiespacio al leer la capa .dcc `LA`, con el monton practicamente vacio). O sea: el costo de un
// jugador NO es el problema -- el problema es el TAMANO DE LA IMAGEN.
//
// LO QUE SE MIDE, ENTONCES, ES LO QUE CUESTA UN JUGADOR DE VERDAD: su `Composite` completo por el
// `AssetManager` real -- el COF del modo Neutral, las seis capas `.dcc` del personaje (HD, TR, LG, RA, LA,
// S1) clonadas por jugador y una superficie propia por capa -- que es donde estan los ~446 KB por jugador
// de DC-002.74. El resto del POC (caches de DC-002.75, reloj del PVR, monton libre, GC, cuadros) es igual
// para este camino y para el del jugador completo (participante-real.go.txt), asi los numeros se comparan.
//
// ES EL MISMO CODIGO QUE `NuevoJugadorDeConsola` (d2core/d2map/d2mapentity/jugador_consola.go) desde
// `LoadComposite` hasta `Equip`, con el equipamiento vacio: ocho capas "lit" y la clase de arma "hth".

import (
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2enum"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2interface"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2resource"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2asset"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2inventory"
)

// jugadorDeComposite es un jugador reducido a lo que la tarjeta mide: su Composite y su lugar en pantalla.
type jugadorDeComposite struct {
	compuesto *d2asset.Composite
	x, y      int
}

func nuevoParticipante(indice int, nombre string, subtiles [2]int, direccion int) (participante, error) {
	compuesto, err := am.LoadComposite(d2enum.ObjectTypePlayer, d2enum.HeroAmazon.GetToken(), d2resource.PaletteUnits)
	if err != nil {
		return nil, err
	}

	// el equipamiento vacio: `GetArmorClass` de un item nil devuelve "lit" y `GetItemCode` "", igual que en
	// el jugador completo (es el mismo bloque de d2mapentity/player.go:496 `equipmentLayers`)
	equipamiento := &d2inventory.CharacterEquipment{}

	capas := &[d2enum.CompositeTypeMax]string{
		d2enum.CompositeTypeHead:      equipamiento.Head.GetArmorClass(),
		d2enum.CompositeTypeTorso:     equipamiento.Torso.GetArmorClass(),
		d2enum.CompositeTypeLegs:      equipamiento.Legs.GetArmorClass(),
		d2enum.CompositeTypeRightArm:  equipamiento.RightArm.GetArmorClass(),
		d2enum.CompositeTypeLeftArm:   equipamiento.LeftArm.GetArmorClass(),
		d2enum.CompositeTypeRightHand: equipamiento.RightHand.GetItemCode(),
		d2enum.CompositeTypeLeftHand:  equipamiento.LeftHand.GetItemCode(),
		d2enum.CompositeTypeShield:    equipamiento.Shield.GetItemCode(),
	}

	// PRIMERO el equipo, DESPUES el modo: `Equip` con el modo vacio solo guarda las capas, y el modo se
	// crea UNA vez con ellas puestas (`Composite.Equip` vuelve a llamar `createMode` si ya hay modo, o sea
	// carga dos veces las seis .dcc del personaje). El estado final es identico.
	if err := compuesto.Equip(capas); err != nil {
		return nil, err
	}

	if err := compuesto.SetMode(d2enum.PlayerAnimationModeNeutral, equipamiento.RightHand.GetWeaponClass()); err != nil {
		return nil, err
	}

	// SetDirection (0 = sur) NO se llama: `Composite.SetDirection` levanta goroutines + sync.WaitGroup por
	// capa, y medido en esta tarjeta la corrida muere con "Unhandled exception ... Data address error" en
	// el primer cuadro (PC basura). La direccion 0 es la que `LoadComposite` ya deja puesta.

	// cada jugador en su lugar de la pantalla (el `Player` lo hace con su mapEntity; aca es directo)
	return &jugadorDeComposite{
		compuesto: compuesto,
		x:         mapaX + 80 + 130*indice,
		y:         mapaY + 200,
	}, nil
}

func (j *jugadorDeComposite) Render(surface d2interface.Surface) error {
	surface.PushTranslation(j.x, j.y)
	defer surface.Pop()

	return j.compuesto.Render(surface)
}

func (j *jugadorDeComposite) Advance(elapsed float64) {
	if err := j.compuesto.Advance(elapsed); err != nil {
		println("COOP276: AVANCE FALLO:", err.Error())
	}
}

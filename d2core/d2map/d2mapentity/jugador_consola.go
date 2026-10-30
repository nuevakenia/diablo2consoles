package d2mapentity

// DC-002.76 -- EL JUGADOR DE LA CONSOLA: NewPlayer SIN la hoja de heroe.
//
// POR QUE EXISTE. Medir cuanto cuesta cada jugador del cooperativo (1, 2 y 4) en la Dreamcast exige
// instanciar jugadores DE VERDAD: el mismo `Player`, el mismo `d2asset.Composite` sobre el mismo
// `AssetManager` y el mismo COF del MPQ. `NewPlayer` (factory.go) hace eso y ademas, ANTES, arma la
// hoja de heroe: `CreateHeroState` -> `CreateHeroSkillsState`, que recorre Skills.txt y exige
// `Records.Skill.Details` cargado; y pide `Records.Character.Stats` (charstats.txt) y el equipamiento.
// En la consola esos registros todavia NO se cargan (la carga de tablas del motor es Fase D): con el
// factory el jugador no se puede construir hoy, y cargar Skills.txt solo para medir jugadores costaria
// RAM que no tiene nada que ver con lo que se esta midiendo.
//
// LO QUE ESTA FUNCION HACE, EXACTAMENTE: lo mismo que `NewPlayer` desde `LoadComposite` hasta
// `composite.Equip`, con el equipamiento vacio (todas las capas "lit" y la clase de arma "hth", que es
// lo que devuelven los `GetArmorClass/GetWeaponClass` de un equipamiento vacio). Lo que NO hace, y esta
// declarado: nada de hoja de heroe, habilidades, oro ni `PrefetchModes` (ese prefetch baja 8 modos del
// personaje, o sea ~3,5 MB de DCC: el que mide decide cuando y cuanto, no el constructor).
//
// Se deja FUERA del pueblo y en el modo Neutral (COF `AMNU...`): es el mismo modo que dibujo el POC de
// DC-002.70/DC-002.75, asi los numeros del cooperativo se pueden comparar con los de esas tarjetas.

import (
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2enum"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2resource"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2asset"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2hero"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2inventory"
)

// NuevoJugadorDeConsola crea un Player con su Composite cargado del MPQ (COF + las capas .dcc del
// modo Neutral, por los caches del AssetManager). `stats` no puede ser nil: `Player.Advance` lo usa
// para la stamina (ver el comentario de arriba sobre la hoja de heroe).
func NuevoJugadorDeConsola(
	am *d2asset.AssetManager,
	id, nombre string,
	x, y, direccion int,
	heroe d2enum.Hero,
	stats *d2hero.HeroStatsState,
) (*Player, error) {
	equipamiento := &d2inventory.CharacterEquipment{}

	compuesto, err := am.LoadComposite(d2enum.ObjectTypePlayer, heroe.GetToken(), d2resource.PaletteUnits)
	if err != nil {
		return nil, err
	}

	resultado := &Player{
		mapEntity: newMapEntity(x, y),
		composite: compuesto,
		Equipment: equipamiento,
		Stats:     stats,
		name:      nombre,
		Class:     heroe,
		isInTown:  false,
		Act:       1,
	}

	resultado.mapEntity.uuid = id

	resultado.SetSpeed(baseWalkSpeed)
	resultado.mapEntity.directioner = resultado.rotate

	arma := equipamiento.RightHand.GetWeaponClass()

	if err := compuesto.SetMode(d2enum.PlayerAnimationModeNeutral, arma); err != nil {
		return nil, err
	}

	compuesto.SetDirection(direccion)
	compuesto.SetLayerEffects(equipmentEffects(equipamiento))

	if err := compuesto.Equip(equipmentLayers(equipamiento)); err != nil {
		return nil, err
	}

	return resultado, nil
}

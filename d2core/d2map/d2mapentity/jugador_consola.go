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
	compuesto, err := am.LoadComposite(d2enum.ObjectTypePlayer, heroe.GetToken(), d2resource.PaletteUnits)
	if err != nil {
		return nil, err
	}

	resultado := jugadorDeConsola(compuesto, id, nombre, x, y, direccion, heroe, stats)

	if err := compuesto.SetMode(d2enum.PlayerAnimationModeNeutral, resultado.Equipment.RightHand.GetWeaponClass()); err != nil {
		return nil, err
	}

	return resultado, nil
}

// CapasDelJugadorDeConsola (DC-002.159): que capas del compuesto se cargan. nil = TODAS, que es lo que hacen la
// PC y el POC del personaje. La consola pone una lista corta porque cada capa son ~89 KB VIVOS en el semiespacio
// del recolector (16 direcciones x los cuadros del modo Neutral) y el pueblo compuesto ya deja 1,27 MB de los
// 1,83 MB del semiespacio: con las cinco capas del equipamiento vacio (cabeza, torso, piernas, dos brazos) el
// recolector muere con `GC to-space overflow`, que es lo que mide la tarjeta.
var CapasDelJugadorDeConsola []d2enum.CompositeType

// capasPedidas: las capas del equipamiento vacio, filtradas por `CapasDelJugadorDeConsola` cuando hay lista.
func capasPedidas(equipamiento *d2inventory.CharacterEquipment) *[d2enum.CompositeTypeMax]string {
	todas := equipmentLayers(equipamiento)

	if len(CapasDelJugadorDeConsola) == 0 {
		return todas
	}

	elegidas := &[d2enum.CompositeTypeMax]string{}

	for _, capa := range CapasDelJugadorDeConsola {
		if int(capa) < len(todas) {
			elegidas[capa] = todas[capa]
		}
	}

	return elegidas
}

// jugadorDeConsola (DC-002.159): el cuerpo comun de los dos constructores -- el de arriba, que carga el
// compuesto, y `NuevoJugadorDeConsolaCon`, que lo recibe ya armado (el que puede armar el compuesto ANTES de
// componer el mapa, que es donde hay memoria). Arma el ente, el equipamiento vacio, la velocidad de caminar y
// el enganche de la rotacion; el modo de animacion lo pone cada constructor porque puede fallar.
func jugadorDeConsola(
	compuesto *d2asset.Composite,
	id, nombre string,
	x, y, direccion int,
	heroe d2enum.Hero,
	stats *d2hero.HeroStatsState,
) *Player {
	equipamiento := &d2inventory.CharacterEquipment{}

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

	compuesto.SetDirection(direccion)
	compuesto.SetLayerEffects(equipmentEffects(equipamiento))

	if err := compuesto.Equip(capasPedidas(equipamiento)); err != nil {
		return resultado
	}

	return resultado
}

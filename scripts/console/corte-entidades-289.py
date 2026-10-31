#!/usr/bin/env python3
"""DC-002.89 -- aplicar el corte del camino de entidades a las COPIAS de d2mapstamp y d2mapengine.

Es el instrumento del experimento (exp-corte-entidades-289.sh): no se corre sobre el repo, se corre
sobre el directorio de copias y falla si un patron no aparece, para que el numero medido sea el de
ESTE corte y no el de un corte que se aplico a medias.

Variantes:
  A  solo la llamada: se saca el bucle de entidades de PlaceStampClippedWhere. Los paquetes siguen
     importando d2mapentity, asi que mide lo que --gc-sections puede recoger por si solo.
  B  arquitectura: d2mapstamp deja de fabricar entidades (EntitiesWhere vacio, sin newNPC/newObject) y
     d2mapengine pierde el campo del factory y el mapa de entidades.

Uso: corte-entidades-289.py <A|B> <dir_con_las_copias>
"""
import sys
import os


def reemplaza(texto, viejo, nuevo, que):
    if viejo not in texto:
        raise SystemExit('FALLO: no encontre el patron de %s' % que)
    if texto.count(viejo) != 1:
        raise SystemExit('FALLO: el patron de %s aparece %d veces' % (que, texto.count(viejo)))
    return texto.replace(viejo, nuevo)


BUCLE_CLIPPED = '''	for _, e := range stamp.EntitiesWhere(tileOffsetX, tileOffsetY, keep, monsters) {
		pos := e.GetPosition()
		tx, ty := int(pos.X())/subtilesPerTile, int(pos.Y())/subtilesPerTile

		if tx < tileOffsetX || ty < tileOffsetY || tx >= tileOffsetX+w || ty >= tileOffsetY+h {
			continue
		}

		m.entities[e.ID()] = e
	}
'''


VAR_BUCLE_OBJETOS = ("\tn := 0\n\n"
                     "\tfor _, e := range stamp.EntitiesWhere(tileOffsetX, tileOffsetY, keep, false) {\n"
                     "\t\tpos := e.GetPosition()\n"
                     "\t\ttx, ty := int(pos.X())/subtilesPerTile, int(pos.Y())/subtilesPerTile\n\n"
                     "\t\tif pos.X() < 0 || pos.Y() < 0 || tx >= m.size.Width || ty >= m.size.Height {\n"
                     "\t\t\tcontinue\n"
                     "\t\t}\n\n"
                     "\t\tm.entities[e.ID()] = e\n"
                     "\t\tn++\n"
                     "\t}\n\n"
                     "\treturn n\n")

VAR_BUCLE_OBJETOS_VACIO = "\tn := 0\n\n\treturn n\n"


def variante_a(corte):
    p = os.path.join(corte, 'd2mapengine', 'engine.go')
    t = open(p).read()
    t = reemplaza(t, BUCLE_CLIPPED, '', 'el bucle de entidades de PlaceStampClippedWhere')
    open(p, 'w').write(t)
    print('   A: engine.go sin el bucle de entidades (los imports quedan)')


def variante_b(corte):
    # ---- d2mapstamp: deja de fabricar entidades
    p = os.path.join(corte, 'd2mapstamp', 'stamp.go')
    t = open(p).read()
    t = reemplaza(t, '\t"github.com/OpenDiablo2/OpenDiablo2/d2core/d2map/d2mapentity"\n', '',
                  'el import de d2mapentity en stamp.go')
    t = reemplaza(t, '\tentity      *d2mapentity.MapEntityFactory\n', '',
                  'el campo entity del Stamp')
    # EntitiesWhere -> vacio (se conserva la firma: es la API que usa el motor)
    ini = t.index('func (mr *Stamp) EntitiesWhere(')
    fin = t.index('// newNPC creates an NPC', ini)
    nuevo = ('func (mr *Stamp) EntitiesWhere(tileOffsetX, tileOffsetY int, keep func(idx int) bool, '
             'monsters bool) []d2interface.MapEntity {\n'
             '\treturn nil\n}\n\n')
    t = t[:ini] + nuevo + t[fin:]
    # newNPC / newObject se van con sus cuerpos
    ini = t.index('// newNPC creates an NPC')
    fin = t.index('func convertPaths(', ini)
    t = t[:ini] + t[fin:]
    t = reemplaza(t, '\t"fmt"\n', '', 'el import de fmt en stamp.go')
    t = reemplaza(t, '\t"github.com/OpenDiablo2/OpenDiablo2/d2common/d2resource"\n', '',
                  'el import de d2resource en stamp.go')
    open(p, 'w').write(t)

    p = os.path.join(corte, 'd2mapstamp', 'factory.go')
    t = open(p).read()
    t = reemplaza(t, '\t"github.com/OpenDiablo2/OpenDiablo2/d2core/d2map/d2mapentity"\n\n', '',
                  'el import de d2mapentity en factory.go')
    t = reemplaza(t, 'func NewStampFactory(asset *d2asset.AssetManager, l d2util.LogLevel, entity *d2mapentity.MapEntityFactory) *StampFactory {',
                  'func NewStampFactory(asset *d2asset.AssetManager, l d2util.LogLevel) *StampFactory {',
                  'la firma de NewStampFactory')
    t = reemplaza(t, '\t\tentity: entity,\n', '', 'la asignacion del factory en NewStampFactory')
    t = reemplaza(t, '\t\tentity:      f.entity,\n', '', 'la copia del factory en LoadStampPath')
    t = reemplaza(t, '\tentity *d2mapentity.MapEntityFactory\n', '', 'el campo entity del StampFactory')
    open(p, 'w').write(t)

    # ---- d2mapengine: sin factory ni mapa de entidades
    p = os.path.join(corte, 'd2mapengine', 'engine.go')
    t = open(p).read()
    t = reemplaza(t, '\t"github.com/OpenDiablo2/OpenDiablo2/d2core/d2map/d2mapentity"\n\n', '',
                  'el import de d2mapentity en engine.go')
    t = reemplaza(t, '''	asset *d2asset.AssetManager
	*d2mapstamp.StampFactory
	*d2mapentity.MapEntityFactory
	seed          int64                            // The map seed
	entities      map[string]d2interface.MapEntity // Entities on the map
''', '''	asset *d2asset.AssetManager
	*d2mapstamp.StampFactory
	seed          int64 // The map seed
''', 'los campos del factory y del mapa de entidades del MapEngine')
    t = reemplaza(t, '''	entity, _ := d2mapentity.NewMapEntityFactory(asset)
	stamp := d2mapstamp.NewStampFactory(asset, l, entity)

	engine := &MapEngine{
		asset:            asset,
		MapEntityFactory: entity,
		StampFactory:     stamp,
''', '''	stamp := d2mapstamp.NewStampFactory(asset, l)

	engine := &MapEngine{
		asset:        asset,
		StampFactory: stamp,
''', 'CreateMapEngine')
    t = reemplaza(t, '\tm.entities = make(map[string]d2interface.MapEntity)\n', '',
                  'el make del mapa de entidades en resetMapBase')
    t = reemplaza(t, '''	// Copy over the entities
	stampEntities := stamp.Entities(tileOffsetX, tileOffsetY)
	for idx := range stampEntities {
		e := stampEntities[idx]
		m.entities[e.ID()] = e
		m.trackEntity(e)
	}
''', '', 'la copia de entidades de PlaceStamp')
    t = reemplaza(t, BUCLE_CLIPPED, '', 'el bucle de entidades de PlaceStampClippedWhere')
    t = reemplaza(t, VAR_BUCLE_OBJETOS, VAR_BUCLE_OBJETOS_VACIO, 'el bucle de entidades de AddStampObjects')
    t = reemplaza(t, '''// Entities returns a pointer a slice of all map entities.
func (m *MapEngine) Entities() map[string]d2interface.MapEntity {
	return m.entities
}''', '''// Entities returns a pointer a slice of all map entities (corte DC-002.89: no hay).
func (m *MapEngine) Entities() map[string]d2interface.MapEntity {
	return nil
}''', 'Entities()')
    ini = t.index('// AddEntity adds an entity to a slice containing all entities.')
    fin = t.index('// GetTiles returns a slice of all tiles matching the given style,', ini)
    t = t[:ini] + t[fin:]
    t = reemplaza(t, '''	for ID := range m.entities {
		m.entities[ID].Advance(tickTime)
	}
''', '', 'el bucle de Advance')
    open(p, 'w').write(t)

    p = os.path.join(corte, 'd2mapengine', 'doors.go')
    t = open(p).read()
    ini = t.index('// trackEntity puts the footprint')
    fin = t.index('// DoorBlocks reports whether a closed door covers the sub-tile.')
    t = t[:ini] + t[fin:]
    t = reemplaza(t, '\t"github.com/OpenDiablo2/OpenDiablo2/d2core/d2map/d2mapentity"\n', '',
                  'el import de d2mapentity en doors.go')
    t = reemplaza(t, '\t"github.com/OpenDiablo2/OpenDiablo2/d2common/d2interface"\n', '',
                  'el import de d2interface en doors.go')
    open(p, 'w').write(t)
    print('   B: d2mapstamp sin fabrica de entidades y d2mapengine sin factory ni mapa de entidades')


def main():
    if len(sys.argv) < 3:
        print(__doc__)
        return 2
    variante = sys.argv[1].upper()
    corte = sys.argv[2]
    if variante == 'A':
        variante_a(corte)
    elif variante == 'B':
        variante_b(corte)
    else:
        raise SystemExit('variante desconocida: %s' % variante)
    return 0


if __name__ == '__main__':
    sys.exit(main())

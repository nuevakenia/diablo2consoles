package d2animdata

// DC-002.159 -- EL `animdata.d2` DEL HEROE: solo los registros que el heroe consulta.
//
// POR QUE EXISTE. `Composite.createMode` (d2core/d2asset/composite.go) pide UNA clave al animdata
// (`<TOKEN><MODO><ARMA>`, por ejemplo `AMTWHTH`) para saber cuantos cuadros tiene el ciclo y a que
// velocidad va. El archivo entero son 2.569 registros y, tal como lo arma `Load`, cada registro se
// lleva su `map[int]AnimationEvent` de 144 posiciones: en el host son **2,17 MB** de mapas, y el
// semiespacio del recolector de la consola son **2.097.152 B**. Cargarlo entero en la consola es
// quedarse sin monton antes de dibujar el primer cuadro (medido en DC-002.156: por eso el Gate 0
// arrancaba con `SinAnimData`).
//
// LO QUE HACE. Recorre el archivo con el mismo parser (`Load`), pero de cada bloque se queda con los
// registros cuyo nombre EMPIEZA con alguno de los prefijos pedidos y tira el resto: los bytes se leen
// igual (el archivo se consume entero, el formato no cambia), lo que no se guarda son las estructuras.
// Con el prefijo del personaje (`AM`) quedan ~60 registros en vez de 2.569: KB en vez de MB.
//
// LO QUE NO HACE, Y ESTA DICHO: los eventos (`AnimationEvent`) se descartan enteros. En la consola el
// heroe del Gate 0 camina y nada mas: los eventos son los sonidos y los cuadros de golpe de las
// habilidades, y ninguna de las dos cosas existe todavia. Un modo que se pida por una clave que no
// entro NO se inventa: `GetRecords` devuelve vacio y `createMode` falla con su error de siempre
// ("could not find Animation data"), que es lo que tiene que pasar.
//
// EL CAMINO DE LA PC NO CAMBIA: `Load` queda igual y esta funcion no la llama nadie del motor; la usa
// el arranque de la consola (d2app/consola_156.go) cuando el main pide el animdata selectivo.

import (
	"errors"
	"fmt"
	"strings"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2datautils"
)

// PrefijosDeUnPersonaje: los prefijos del animdata de un personaje del juego (los tres que usa el
// heroe del Gate 0: `AM` amazona, `SO` hechicera, `PA` paladin...). Se deja como tabla con nombre
// para que el arranque de la consola no escriba literales sueltos.
func PrefijosDeUnPersonaje(token string) []string {
	if token == "" {
		return nil
	}

	return []string{strings.ToUpper(token)}
}

// LoadSelectivo carga el animdata quedandose SOLO con los registros cuyo nombre empieza con alguno de
// los prefijos. Devuelve cuantos registros entraron y cuantos se tiraron (el numero va al log: es la
// prueba de que el filtro hizo algo).
func LoadSelectivo(data []byte, prefijos []string) (animdata *AnimationData, dentro, fuera int, err error) {
	if len(prefijos) == 0 {
		return nil, 0, 0, errors.New("LoadSelectivo sin prefijos: usa Load para el archivo entero")
	}

	reader := d2datautils.CreateStreamReader(data)
	animdata = &AnimationData{}
	animdata.entries = make(map[string][]*AnimationDataRecord)

	for blockIdx := range animdata.blocks {
		recordCount, err := reader.ReadUInt32()
		if err != nil {
			return nil, 0, 0, err
		}

		if recordCount > maxRecordsPerBlock {
			return nil, 0, 0, fmt.Errorf("more than %d records in block", maxRecordsPerBlock)
		}

		for recordIdx := uint32(0); recordIdx < recordCount; recordIdx++ {
			nameBytes, err := reader.ReadBytes(byteCountName)
			if err != nil {
				return nil, 0, 0, err
			}

			if nameBytes[byteCountName-1] != byte(0) {
				return nil, 0, 0, errors.New("animdata AnimationDataRecord name missing null terminator byte")
			}

			name := strings.ReplaceAll(string(nameBytes), string(byte(0)), "")

			frames, err := reader.ReadUInt32()
			if err != nil {
				return nil, 0, 0, err
			}

			speed, err := reader.ReadUInt16()
			if err != nil {
				return nil, 0, 0, err
			}

			reader.SkipBytes(byteCountSpeedPadding)

			// los 144 bytes de eventos se leen y se tiran SIEMPRE (tambien para los registros que
			// entran): el puntero del lector no se puede saltar sin leer el bloque entero de una vez,
			// y leer 144 bytes es mas barato que ramificar por registro.
			if _, err := reader.ReadBytes(numEvents); err != nil {
				return nil, 0, 0, err
			}

			if !coincideAlgunPrefijo(name, prefijos) {
				fuera++

				continue
			}

			animdata.entries[name] = append(animdata.entries[name], &AnimationDataRecord{
				name:               name,
				framesPerDirection: frames,
				speed:              speed,
				events:             nil, // declarado arriba: la consola no usa eventos
			})
			dentro++
		}

		animdata.blocks[blockIdx] = &block{recordCount: 0, records: nil}
	}

	if reader.Position() != uint64(len(data)) {
		return nil, 0, 0, fmt.Errorf("unable to parse animation data: %d != %d", reader.Position(), len(data))
	}

	return animdata, dentro, fuera, nil
}

// coincideAlgunPrefijo: si el nombre del registro empieza con alguno de los prefijos (el animdata
// nombra `<TOKEN><MODO><ARMA>`, todo en mayusculas).
func coincideAlgunPrefijo(name string, prefijos []string) bool {
	for _, p := range prefijos {
		if strings.HasPrefix(name, p) {
			return true
		}
	}

	return false
}

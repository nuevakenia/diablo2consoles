package d2mpq

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// mapaViejo es el indice que el motor tenia hasta DC-002.33: una clave por ranura de la tabla de hash, con
// las DOS mitades del nombre pegadas (`hashString(nombre, 1) << 32 | hashString(nombre, 2)`), y el ultimo
// que escribe gana. Es el camino que la tarjeta reemplaza por los BYTES + el sondeo, asi que sirve de
// referencia: `buscarHash` tiene que encontrar lo mismo.
func mapaViejo(tabla []byte) map[uint64]uint32 {
	viejo := make(map[uint64]uint32)

	for i := 0; i+bytesPorEntrada <= len(tabla); i += bytesPorEntrada {
		a := binary.LittleEndian.Uint32(tabla[i:])
		b := binary.LittleEndian.Uint32(tabla[i+4:])
		idx := binary.LittleEndian.Uint32(tabla[i+12:])

		viejo[uint64(a)<<32|uint64(b)] = idx
	}

	return viejo
}

// TestBuscarHashIgualQueElMapaViejo: el camino del HOST tiene que seguir dando lo mismo que daba el mapa de
// objetos. Se compara, nombre por nombre del `(listfile)` de cada MPQ del JUEGO, la consulta nueva (el sondeo
// sobre los bytes) contra la vieja (el mapa), y se cuentan de paso los nombres que NINGUNO de los dos
// encuentra (nombres que quedaron fuera del `(listfile)`, que no es el indice completo del archivo).
//
// Necesita D2_GAME_DIR (la carpeta de la instalacion) y solo LEE los MPQ.
func TestBuscarHashIgualQueElMapaViejo(t *testing.T) {
	dir := os.Getenv("D2_GAME_DIR")
	if dir == "" {
		t.Skip("D2_GAME_DIR not set")
	}

	archivos := []string{"patch_D2.mpq", "patch_d2.mpq", "d2char.mpq", "d2data.mpq"}

	var medidos int

	for _, archivo := range archivos {
		ruta := filepath.Join(dir, archivo)
		if _, err := os.Stat(ruta); err != nil {
			continue
		}

		m, err := FromFile(ruta)
		if err != nil {
			t.Fatalf("%s: %v", archivo, err)
		}

		datos, err := m.ReadFile("(listfile)")
		if err != nil {
			m.Close()

			t.Fatalf("%s: (listfile): %v", archivo, err)
		}

		viejo := mapaViejo(m.hashTable)

		nombres, iguales, distintos, fuera := 0, 0, 0, 0

		for _, linea := range strings.Split(string(datos), "\n") {
			nombre := strings.TrimSpace(strings.TrimRight(linea, "\x00"))

			if nombre == "" {
				continue
			}

			nombres++

			idxViejo, okViejo := viejo[hashFilename(nombre)]
			idxNuevo, okNuevo := m.buscarHash(nombre)

			switch {
			case !okViejo && !okNuevo:
				fuera++
			case okViejo != okNuevo || idxViejo != idxNuevo:
				distintos++

				if distintos <= 5 {
					t.Errorf("%s: %q: el mapa viejo dice (%d, %v) y el sondeo (%d, %v)",
						archivo, nombre, idxViejo, okViejo, idxNuevo, okNuevo)
				}
			default:
				iguales++
			}
		}

		hashSum, blockSum := m.TableChecksum()

		t.Logf("%s: entradas_hash=%d entradas_bloque=%d nombres=%d iguales=%d distintos=%d fuera_del_listfile=%d checksum_hash=%d checksum_bloque=%d",
			archivo, m.hashEntries(), m.blockEntries(), nombres, iguales, distintos, fuera, hashSum, blockSum)

		m.Close()

		if distintos != 0 {
			t.Fatalf("%s: %d nombres donde el sondeo y el mapa viejo NO dan lo mismo", archivo, distintos)
		}

		if nombres == 0 {
			t.Fatalf("%s: el (listfile) vino vacio", archivo)
		}

		medidos++
	}

	if medidos == 0 {
		t.Skip("no hay MPQ del juego en D2_GAME_DIR")
	}
}

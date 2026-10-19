package d2mpq

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// mapaViejo es el indice que el motor tenia hasta DC-002.33: una clave por ranura de la tabla de hash, con
// las DOS mitades del nombre pegadas (`hashString(nombre, 1) << 32 | hashString(nombre, 2)`), y el ultimo
// que escribe gana. Es el camino que la tarjeta reemplaza por los BYTES + el sondeo, asi que sirve de
// referencia: `buscarHash` tiene que encontrar lo mismo.
//
// DC-002.41: recibe las palabras de la tabla CRUDA (las que salen del archivo, ranuras vacias incluidas),
// NO la representacion que guarda el paquete: con las ranuras vacias fuera del arreglo, una referencia
// armada con `entradaHash` no seria independiente (un error del arreglo saldria igual en las dos puntas).
func mapaViejo(palabras []uint32) map[uint64]uint32 {
	viejo := make(map[uint64]uint32)

	for i := 0; i+4 <= len(palabras); i += 4 {
		viejo[uint64(palabras[i])<<32|uint64(palabras[i+1])] = palabras[i+3]
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

		// DC-002.41: la referencia sale de la tabla CRUDA del archivo, descifrada palabra por palabra por el
		// camino de siempre (`naiveDecryptTable`): el sondeo se compara contra los bytes que estan en el
		// disco, no contra otra vuelta de la representacion nueva.
		cruda := make([]byte, int(m.hashEntries())*bytesPorEntrada)
		if _, errR := m.file.ReadAt(cruda, int64(m.header.HashTableOffset)); errR != nil {
			t.Fatalf("%s: no pude leer la tabla de hash cruda: %v", archivo, errR)
		}

		palabras, errR := naiveDecryptTable(bytes.NewReader(cruda), m.hashEntries(), "(hash table)")
		if errR != nil {
			t.Fatalf("%s: %v", archivo, errR)
		}

		viejo := mapaViejo(palabras)

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

		// DC-002.41: lo que la tabla de hash ocupa AHORA en RAM (las ranuras ocupadas + 1 bit por ranura +
		// los rangos) contra lo que ocupaba con la tabla entera. Es el numero de la tarjeta.
		reserva := cap(m.hashEntradas) + len(m.hashOcupadas) + 2*len(m.hashRangos)

		t.Logf("%s: entradas_hash=%d entradas_bloque=%d nombres=%d iguales=%d distintos=%d fuera_del_listfile=%d checksum_hash=%d checksum_bloque=%d ranuras_ocupadas=%d reserva_hash=%d reserva_tabla_entera=%d",
			archivo, m.hashEntries(), m.blockEntries(), nombres, iguales, distintos, fuera, hashSum, blockSum,
			len(m.hashEntradas)/bytesPorEntrada, reserva, int(m.hashEntries())*bytesPorEntrada)

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

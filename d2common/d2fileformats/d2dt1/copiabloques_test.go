package d2dt1

import (
	"bytes"
	"testing"
)

// DC-002.78 -- LA PRUEBA EN CONTRA DE `LoadDT1CopiaBloques`.
//
// LO QUE HAY QUE PROBAR NO ES QUE CARGUE: es que el DT1 cargado NO DEPENDA del archivo crudo. Ese es todo el
// punto del camino nuevo (la consola lee cada .dt1 en un buffer que despues se reusa para el archivo
// siguiente: si los bloques fueran subslices, la zona se dibujaria con pedazos del archivo equivocado). Y
// como toda prueba de una propiedad negativa, se hace al reves: se carga, se PISA el crudo entero y se
// exige que el DT1 siga igual.
//
// La segunda mitad del test es el MUTADOR: con el parseo de siempre (`LoadDT1`, sin copiar) el crudo pisado
// SI cambia el resultado. Si esa mitad pasara tambien, la prueba no estaria midiendo nada.
func TestCopiaBloquesNoDependeDelCrudo(t *testing.T) {
	crudo := synthDT1()

	// 1. el camino nuevo: copia los bloques
	conCopia, err := LoadDT1CopiaBloques(crudo)
	if err != nil {
		t.Fatalf("LoadDT1CopiaBloques: %v", err)
	}

	antes := conCopia.Marshal()

	// 2. se pisa el crudo ENTERO, como hace el buffer reusado con el archivo siguiente
	for i := range crudo {
		crudo[i] = 0xFF
	}

	despues := conCopia.Marshal()
	if !bytes.Equal(antes, despues) {
		t.Fatalf("el DT1 copiado CAMBIO al pisar el crudo (%d -> %d bytes): los bloques siguen apuntando al archivo",
			len(antes), len(despues))
	}

	// 3. el mutador: el parseo de siempre NO aguanta esto (por eso hace falta la copia)
	crudoViejo := synthDT1()

	sinCopia, err := LoadDT1(crudoViejo)
	if err != nil {
		t.Fatalf("LoadDT1: %v", err)
	}

	antesViejo := sinCopia.Marshal()

	for i := range crudoViejo {
		crudoViejo[i] = 0xFF
	}

	if bytes.Equal(antesViejo, sinCopia.Marshal()) {
		t.Fatal("el mutador no muerde: `LoadDT1` (sin copiar) deberia cambiar al pisar el crudo, y no cambio")
	}
}

// TestCopiaBloquesLeeIgualQueElParseoDeSiempre: la copia no puede romper el parseo. Los dos caminos tienen
// que devolver los MISMOS bytes (es el mismo archivo leido de dos maneras).
func TestCopiaBloquesLeeIgualQueElParseoDeSiempre(t *testing.T) {
	crudo := synthDT1()

	conCopia, err := LoadDT1CopiaBloques(crudo)
	if err != nil {
		t.Fatalf("LoadDT1CopiaBloques: %v", err)
	}

	sinCopia, err := LoadDT1(synthDT1())
	if err != nil {
		t.Fatalf("LoadDT1: %v", err)
	}

	if !bytes.Equal(conCopia.Marshal(), sinCopia.Marshal()) {
		t.Fatal("los dos caminos de carga no dan el mismo DT1")
	}

	if len(conCopia.Tiles) != len(sinCopia.Tiles) {
		t.Fatalf("tiles %d != %d", len(conCopia.Tiles), len(sinCopia.Tiles))
	}

	for i := range conCopia.Tiles {
		for j := range conCopia.Tiles[i].Blocks {
			a := conCopia.Tiles[i].Blocks[j].EncodedData
			b := sinCopia.Tiles[i].Blocks[j].EncodedData
			if !bytes.Equal(a, b) {
				t.Fatalf("bloque %d/%d distinto (%d vs %d bytes)", i, j, len(a), len(b))
			}
		}
	}
}

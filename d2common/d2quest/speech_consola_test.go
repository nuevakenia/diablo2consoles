package d2quest

import "testing"

// DC-002.128 -- EL CSV DE MENSAJES VACIO NO ES UN ARCHIVO ROTO (regresion del port a la Dreamcast).
//
// En la consola el CSV de mensajes NO viaja adentro del binario (gccgo no tiene `//go:embed`, ver el parche de port
// de speech.go): `messagesCSV` es la cadena vacia. Con la cadena vacia `encoding/csv` devuelve `(nil, io.EOF)` y el
// `panic` de `loadMessages` salia SIEMPRE, asi que el guardia de DC-002.48 (`len(rows) == 0`) no llegaba nunca.
// Medido en la Dreamcast (DC-002.128): el POC moria antes del primer cuadro con
// `panic: d2quest: bad embedded messages.csv: EOF`, en el primer `newDenOfEvil` -> `speechTables`.
//
// La prueba es la que caza la regresion: sin CSV no hay tablas de dialogo (se declara el limite) pero el paquete NO
// paniquea, y las lineas de intro que trae el codigo (`introMessages`) se siguen cargando.
func TestMensajesVaciosNoPaniquean(t *testing.T) {
	guardado := messagesCSV
	messagesCSV = ""

	defer func() { messagesCSV = guardado }()

	loadMessages() // con el bug de antes, aca paniqueaba

	if len(messages.tables) != 0 {
		t.Fatalf("sin CSV no hay tablas de dialogo, y hay %d", len(messages.tables))
	}

	if got := speechTables("A1Q1"); len(got) != 0 {
		t.Fatalf("speechTables(A1Q1) sin CSV devolvio %d tablas, se esperaba 0", len(got))
	}

	// y el camino del motor que arma la quest de la Den of Evil tiene que poder construirse sin paniquear
	if q := newDenOfEvil(); q == nil || q.ID != QuestDenOfEvil {
		t.Fatal("newDenOfEvil() no devolvio el nodo de la Den of Evil")
	}
}

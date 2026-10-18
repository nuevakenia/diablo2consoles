package d2mpq

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"
)

// lectorAnotado: entrega los datos de a lo sumo `tope` bytes por Read y ANOTA el pedido mas grande que le
// hicieron. Es el instrumento de DC-002.28: lo que no entra en la Dreamcast es el pedido de la tabla
// entera de un tiron (la de hash de d2data.mpq son 1.048.576 bytes), y eso se mide con el TAMANO DEL
// PEDIDO, no con la memoria del proceso (que en el host sobra y taparia el defecto).
type lectorAnotado struct {
	datos []byte
	pos   int
	tope  int
	mayor int
}

func (l *lectorAnotado) Read(p []byte) (int, error) {
	if len(p) > l.mayor {
		l.mayor = len(p)
	}

	if l.pos >= len(l.datos) {
		return 0, io.EOF
	}

	n := len(p)
	if n > l.tope {
		n = l.tope
	}

	if l.pos+n > len(l.datos) {
		n = len(l.datos) - l.pos
	}

	copy(p, l.datos[l.pos:l.pos+n])

	l.pos += n

	return n, nil
}

// TestDecryptTablePorTrozosLeeDeAPoco: la tabla se descifra por trozos, palabra por palabra igual que de un
// solo tiron, y NUNCA se pide mas de un trozo de una vez. Los tamanos elegidos cruzan el marco del trozo
// (1.024 entradas) y el ultimo caso trae la tabla cortada, que tiene que entregar lo que llego y despues el
// error (igual que el camino viejo).
func TestDecryptTablePorTrozosLeeDeAPoco(t *testing.T) {
	tests := []struct {
		name    string
		entries uint32
		extra   int // bytes que faltan del final (tabla cortada)
	}{
		{"una entrada", 1, 0},
		{"justo antes del marco", tablaTrozoEntradas - 1, 0},
		{"justo el marco", tablaTrozoEntradas, 0},
		{"una entrada despues del marco", tablaTrozoEntradas + 1, 0},
		{"cuatro trozos y monedas", 4*tablaTrozoEntradas + 7, 0},
		{"cortada en el segundo trozo", 3 * tablaTrozoEntradas, 20},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			datos := make([]byte, int(tt.entries)*16-tt.extra)
			for i := range datos {
				datos[i] = byte(i*29 + 11)
			}

			quiero, quieroErr := naiveDecryptTable(bytes.NewReader(datos), tt.entries, "(hash table)")

			lector := &lectorAnotado{datos: datos, tope: 1 << 20}
			var got []uint32

			trozos := 0

			gotErr := decryptTablePorTrozos(lector, tt.entries, "(hash table)", func(trozo []uint32) error {
				got = append(got, trozo...)
				trozos++

				return nil
			})

			if (quieroErr == nil) != (gotErr == nil) {
				t.Fatalf("error %v, quiero %v", gotErr, quieroErr)
			}

			// Con la tabla cortada las dos versiones dejan de hablar del mismo largo: el camino por trozos
			// entrega las palabras que SI llegaron (12.283 contra 12.288 en el caso cortado) y el de un solo
			// tiron devuelve el arreglo completo con la cola en cero. Lo que importa es que lo que llego sea
			// igual palabra por palabra, y que los dos contesten el error.
			n := len(got)
			if len(quiero) < n {
				n = len(quiero)
			}

			for i := 0; i < n; i++ {
				if got[i] != quiero[i] {
					t.Fatalf("palabra %d = %#x, quiero %#x", i, got[i], quiero[i])
				}
			}

			// EL TOPE DEL PEDIDO, en un numero FIJO y no en el de la constante: mas de 64 KiB de una vez se
			// va al monton de C de la consola (`gc_external_alloc`, el umbral de "objeto grande" de
			// libgodc) y es justo el pedido que no entra (medido: `gc_external_alloc: OOM` con la tabla de
			// d2data en 1.048.576 bytes). Con un tope que se moviera con la constante, un trozo de 128 KiB
			// pasaria el control: el control tiene que morder.
			if pedidoMaximo := 64 << 10; lector.mayor > pedidoMaximo {
				t.Fatalf("pidio %d bytes de una vez: el tope es %d (mas que eso se va al monton de C en la consola)", lector.mayor, pedidoMaximo)
			}

			// y que de verdad vaya por trozos: una tabla mas larga que un trozo tiene que llegar en varios
			esperadosTrozos := int((tt.entries + tablaTrozoEntradas - 1) / tablaTrozoEntradas)

			if trozos != esperadosTrozos {
				t.Fatalf("llego en %d trozos, esperaba %d", trozos, esperadosTrozos)
			}
		})
	}
}

// TestTablasPorTrozosUnMPQDeVerdad: un MPQ armado por el generador de las pruebas, con las dos tablas mas
// grandes que tres trozos, abierto con `FromFile` (el camino del motor) tiene que dar EXACTAMENTE las
// mismas dos tablas que la version que descifra la tabla entera palabra por palabra.
//
// Es la misma clase de control que el corpus del host: el numero (las entradas) sale del MPQ, no de la
// prueba, y la comparacion es contra el camino viejo, no contra una constante escrita a mano.
func TestTablasPorTrozosUnMPQDeVerdad(t *testing.T) {
	const (
		hashEntradas   = uint32(4096) // 4 trozos: cruza el marco 4 veces
		bloqueEntradas = uint32(3000) // casi 3 trozos
	)

	p := filepath.Join(t.TempDir(), "grande.mpq")
	if err := os.WriteFile(p, synthMPQSized(hashEntradas, bloqueEntradas, nil), 0o600); err != nil {
		t.Fatal(err)
	}

	m, err := FromFile(p)
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()

	// la referencia: la tabla cruda del archivo, descifrada entera palabra por palabra
	crudoHash := make([]byte, int(hashEntradas)*16)
	if _, err := m.file.ReadAt(crudoHash, int64(m.header.HashTableOffset)); err != nil {
		t.Fatal(err)
	}

	quiero, err := naiveDecryptTable(bytes.NewReader(crudoHash), hashEntradas, "(hash table)")
	if err != nil {
		t.Fatal(err)
	}

	// DC-002.33: la tabla de hash ya no es un mapa de objetos, son BYTES: se compara ranura por ranura contra
	// la referencia (el camino de la tabla entera), que es la misma que se compara para la de bloque.
	if got := m.hashEntries(); got != hashEntradas {
		t.Fatalf("la tabla de hash tiene %d ranuras, quiero %d", got, hashEntradas)
	}

	for n, i := uint32(0), uint32(0); i < hashEntradas; n, i = n+4, i+1 {
		got := m.entradaHash(i)

		if got.A != quiero[n] || got.B != quiero[n+1] ||
			got.Locale != uint16(quiero[n+2]>>16) || got.Platform != uint16(quiero[n+2]&0xFFFF) ||
			got.BlockIndex != quiero[n+3] {
			t.Fatalf("ranura %d = %+v, quiero %d/%d/%d/%d", i, got,
				quiero[n], quiero[n+1], quiero[n+2], quiero[n+3])
		}
	}

	// y la tabla de bloques, entrada por entrada
	crudoBloque := make([]byte, int(bloqueEntradas)*16)
	if _, err := m.file.ReadAt(crudoBloque, int64(m.header.BlockTableOffset)); err != nil {
		t.Fatal(err)
	}

	quieroBloques, err := naiveDecryptTable(bytes.NewReader(crudoBloque), bloqueEntradas, "(block table)")
	if err != nil {
		t.Fatal(err)
	}

	if got := m.blockEntries(); got != bloqueEntradas {
		t.Fatalf("la tabla de bloque tiene %d entradas, quiero %d", got, bloqueEntradas)
	}

	for i := uint32(0); i < bloqueEntradas; i++ {
		n := i * 4
		got := m.bloqueEn(i)

		if got.FilePosition != quieroBloques[n] || got.CompressedFileSize != quieroBloques[n+1] ||
			got.UncompressedFileSize != quieroBloques[n+2] || uint32(got.Flags) != quieroBloques[n+3] {
			t.Fatalf("bloque %d = %+v, quiero %d/%d/%d/%d", i, *got,
				quieroBloques[n], quieroBloques[n+1], quieroBloques[n+2], quieroBloques[n+3])
		}
	}
}

// TestDecryptTableSigueSiendoLaReferencia: el camino de la tabla entera (el que usan las pruebas y el
// banco de pruebas) tiene que seguir dando lo mismo que el de por trozos: si los dos se van de las manos a
// la vez, la comparacion de arriba no lo caza.
func TestDecryptTableSigueSiendoLaReferencia(t *testing.T) {
	const entradas = uint32(3*tablaTrozoEntradas + 5)

	datos := make([]byte, entradas*16)
	for i := range datos {
		datos[i] = byte(i*7 + 3)
	}

	quiero, err := naiveDecryptTable(bytes.NewReader(datos), entradas, "(block table)")
	if err != nil {
		t.Fatal(err)
	}

	got, err := decryptTable(bytes.NewReader(datos), entradas, "(block table)")
	if err != nil {
		t.Fatal(err)
	}

	if len(got) != len(quiero) {
		t.Fatalf("%d palabras, quiero %d", len(got), len(quiero))
	}

	for i := range quiero {
		if got[i] != quiero[i] {
			t.Fatalf("palabra %d = %#x, quiero %#x", i, got[i], quiero[i])
		}
	}
}

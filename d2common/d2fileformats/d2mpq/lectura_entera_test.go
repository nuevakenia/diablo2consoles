package d2mpq

// DC-002.79 -- las pruebas del camino de lectura con el tamano reservado.
//
// QUE PROTEGEN. `LeerEn` es el camino que usa la consola para leer archivos grandes (`d2asset.LoadFile`):
// reserva el tamano que dice la tabla de bloques y lee ahi adentro, sin `io.ReadAll` (que en la Dreamcast
// deja ~2 MB tomados por archivo, ver `lectura_entera.go`). Lo que estas pruebas exigen es que el camino
// nuevo entregue EXACTAMENTE el mismo archivo que `ReadFile` (el camino viejo, que funciona en la consola)
// y que no pida memoria de mas.

import (
	"bytes"
	"hash/crc32"
	"os"
	"path/filepath"
	"testing"
)

// archivoDePrueba: los dos archivos del MPQ sintetico de las pruebas (`synthMPQ`, sin nada del juego):
// uno guardado tal cual y uno comprimido en tres sectores -- el que se parece a un .dt1.
const (
	pruebaPlano    = "plain.txt"
	pruebaComprimi = "big.txt"
)

// abrirDePrueba arma el MPQ sintetico en un archivo temporal y lo abre por el camino del motor.
func abrirDePrueba(t *testing.T) *MPQ {
	t.Helper()

	p := filepath.Join(t.TempDir(), "prueba.mpq")
	if err := os.WriteFile(p, synthMPQ(nil), 0o600); err != nil {
		t.Fatal(err)
	}

	m, err := FromFile(p)
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() { _ = m.Close() })

	return m
}

// TestTamanoDeArchivoEsElDeReadFile: el tamano que se reserva tiene que ser el del archivo, medido contra lo
// que ya hacia `ReadFile` (no contra una constante escrita a mano).
func TestTamanoDeArchivoEsElDeReadFile(t *testing.T) {
	m := abrirDePrueba(t)

	for _, nombre := range []string{pruebaPlano, pruebaComprimi} {
		quiero, err := m.ReadFile(nombre)
		if err != nil {
			t.Fatalf("%s: ReadFile: %v", nombre, err)
		}

		tamano, hay := m.TamanoDeArchivo(nombre)
		if !hay {
			t.Fatalf("%s: TamanoDeArchivo dice que no lo sabe", nombre)
		}

		if tamano != len(quiero) {
			t.Fatalf("%s: TamanoDeArchivo = %d, ReadFile leyo %d", nombre, tamano, len(quiero))
		}

		comprimido, hayComprimido := m.TamanoComprimido(nombre)
		if !hayComprimido || comprimido <= 0 || int64(comprimido) > m.fileSize {
			t.Fatalf("%s: TamanoComprimido = %d (hay=%v), fuera de rango", nombre, comprimido, hayComprimido)
		}
	}

	if _, hay := m.TamanoDeArchivo("no-existe.txt"); hay {
		t.Fatal("TamanoDeArchivo dice que sabe el tamano de un archivo que no esta")
	}
}

// TestLeerEnDaElMismoArchivoQueReadFile: el contenido tiene que ser IDENTICO (byte por byte) al de
// `ReadFile`, con el mismo crc32. Es la prueba que sostiene el criterio de la tarjeta ("crc32 iguales").
func TestLeerEnDaElMismoArchivoQueReadFile(t *testing.T) {
	m := abrirDePrueba(t)

	for _, nombre := range []string{pruebaPlano, pruebaComprimi} {
		quiero, err := m.ReadFile(nombre)
		if err != nil {
			t.Fatalf("%s: ReadFile: %v", nombre, err)
		}

		tamano, _ := m.TamanoDeArchivo(nombre)
		dst := make([]byte, tamano)

		leidos, err := m.LeerEn(nombre, dst)
		if err != nil {
			t.Fatalf("%s: LeerEn: %v", nombre, err)
		}

		if leidos != tamano {
			t.Fatalf("%s: LeerEn leyo %d de %d bytes", nombre, leidos, tamano)
		}

		if !bytes.Equal(dst[:leidos], quiero) {
			t.Fatalf("%s: el contenido de LeerEn NO es el de ReadFile", nombre)
		}

		if crc32.ChecksumIEEE(dst[:leidos]) != crc32.ChecksumIEEE(quiero) {
			t.Fatalf("%s: el crc32 de LeerEn NO es el de ReadFile", nombre)
		}
	}
}

// TestLeerEnNoEntraEnElBuffer: si el buffer es corto se dice (y no se lee de mas: pedir memoria es justo
// lo que este camino evita).
func TestLeerEnNoEntraEnElBuffer(t *testing.T) {
	m := abrirDePrueba(t)

	tamano, _ := m.TamanoDeArchivo(pruebaComprimi)

	// el MUTADOR de esta prueba: un byte menos tiene que fallar; con el tamano justo tiene que andar
	if _, err := m.LeerEn(pruebaComprimi, make([]byte, tamano-1)); err != errNoEntraEnElBuffer {
		t.Fatalf("con el buffer corto quiero errNoEntraEnElBuffer, dio %v", err)
	}

	if _, err := m.LeerEn(pruebaComprimi, make([]byte, tamano)); err != nil {
		t.Fatalf("con el buffer justo no tiene que fallar: %v", err)
	}
}

// TestLeerEnConPrefetchInhibidoDaLoMismoYNoPrefetchea: la palanca de la consola apaga el prefetch del
// archivo comprimido entero (hasta 2 MB que en la Dreamcast no los devuelve nadie). Lo que tiene que pasar:
// el archivo sale IDENTICO y el stream NO se queda con el tramo comprimido.
//
// La segunda mitad es el MUTADOR de la palanca: si alguien saca la condicion `!sinPrefetch` de
// `sectorBytes`, el archivo sigue saliendo bien pero `raw` queda con bytes y esta prueba lo caza.
func TestLeerEnConPrefetchInhibidoDaLoMismoYNoPrefetchea(t *testing.T) {
	m := abrirDePrueba(t)

	quiero, err := m.ReadFile(pruebaComprimi)
	if err != nil {
		t.Fatal(err)
	}

	tamano, _ := m.TamanoDeArchivo(pruebaComprimi)

	InhibirPrefetch(true)
	defer InhibirPrefetch(false)

	if !PrefetchInhibido() {
		t.Fatal("InhibirPrefetch(true) no prendio la palanca")
	}

	dst := make([]byte, tamano)

	leidos, err := m.LeerEn(pruebaComprimi, dst)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(dst[:leidos], quiero) {
		t.Fatal("con el prefetch inhibido el contenido cambio")
	}

	// y el stream, leido a mano, no se queda con el tramo comprimido
	bloque := *m.bloqueEn(1) // big.txt es la entrada 1 del MPQ sintetico
	bloque.FileName = pruebaComprimi

	stream, err := CreateStream(m, &bloque, pruebaComprimi)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := leerTodoEn(stream, make([]byte, tamano), tamano); err != nil {
		t.Fatal(err)
	}

	if stream.raw != nil {
		t.Fatalf("el prefetch esta inhibido y el stream igual se quedo con %d bytes comprimidos", len(stream.raw))
	}
}

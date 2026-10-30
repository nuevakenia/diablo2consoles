package d2mpq

// DC-002.80 -- las pruebas de la lectura con MEMORIA PRESTADA (`prestamo.go`).
//
// QUE PROTEGEN. `ReadFileInto` es el camino con el que la consola carga los .dt1 de una zona: la lectura no
// pide memoria por archivo ni por sector (medido en DC-002.78: leer UN .dt1 de 505 KB por el stream pedia
// ~2 MB del semiespacio del recolector y el runtime moria con `gc_alloc: out of memory`). Lo que tienen que
// exigir estas pruebas:
//
//  1. el archivo sale IDENTICO al de `ReadFile` (byte por byte y crc32), con uno y con muchos archivos
//     seguidos sobre el mismo prestamo -- reusar el descompresor y los buffers no puede ensuciar el
//     contenido;
//  2. la memoria del trabajo NO se vuelve a pedir: `Crecidas` no crece y `Reservado` no cambia despues de la
//     primera lectura (ese es el numero que se mide en la consola);
//  3. si el destino es corto se DICE (`ErrNoEntraEnElBuffer`), no se pide memoria;
//  4. el decodificador de PKWare escribe dentro del arreglo prestado con el MISMO resultado que el camino
//     creciente (`explodeEn` contra `explode`), y si no entra lo dice.

import (
	"bytes"
	"hash/crc32"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/JoshVarga/blast"
)

// TestReadFileIntoDaElMismoArchivoQueReadFile: contenido identico al camino de siempre, con el mismo crc32,
// para el archivo guardado tal cual y para el comprimido en tres sectores (el que se parece a un .dt1).
func TestReadFileIntoDaElMismoArchivoQueReadFile(t *testing.T) {
	m := abrirDePrueba(t)

	prestamo := &Scratch{}

	for _, nombre := range []string{pruebaPlano, pruebaComprimi} {
		quiero, err := m.ReadFile(nombre)
		if err != nil {
			t.Fatalf("%s: ReadFile: %v", nombre, err)
		}

		tamano, _ := m.TamanoDeArchivo(nombre)
		dst := make([]byte, tamano)

		leidos, err := m.ReadFileInto(nombre, dst, prestamo)
		if err != nil {
			t.Fatalf("%s: ReadFileInto: %v", nombre, err)
		}

		if leidos != tamano {
			t.Fatalf("%s: ReadFileInto leyo %d de %d bytes", nombre, leidos, tamano)
		}

		if !bytes.Equal(dst[:leidos], quiero) {
			t.Fatalf("%s: el contenido con prestamo NO es el de ReadFile", nombre)
		}

		if crc32.ChecksumIEEE(dst[:leidos]) != crc32.ChecksumIEEE(quiero) {
			t.Fatalf("%s: el crc32 con prestamo NO es el de ReadFile", nombre)
		}
	}
}

// TestReadFileIntoReusaLaMemoriaEntreArchivos: el MISMO prestamo, diez vueltas sobre los dos archivos. El
// contenido sigue saliendo igual (el descompresor de zlib se reinicia, no se queda con el stream anterior) y
// la memoria del trabajo NO crece despues de la primera lectura: ni una reserva mas.
func TestReadFileIntoReusaLaMemoriaEntreArchivos(t *testing.T) {
	m := abrirDePrueba(t)

	quiero := map[string][]byte{}

	for _, nombre := range []string{pruebaPlano, pruebaComprimi} {
		datos, err := m.ReadFile(nombre)
		if err != nil {
			t.Fatalf("%s: ReadFile: %v", nombre, err)
		}

		quiero[nombre] = datos
	}

	prestamo := &Scratch{}
	dst := make([]byte, len(quiero[pruebaComprimi]))

	// la primera lectura pide la memoria del trabajo (es la que paga el prestamo)
	lectura := 0

	leer := func(nombre string) {
		lectura++

		leidos, err := m.ReadFileInto(nombre, dst, prestamo)
		if err != nil {
			t.Fatalf("lectura %d (%s): %v", lectura, nombre, err)
		}

		if !bytes.Equal(dst[:leidos], quiero[nombre]) {
			t.Fatalf("lectura %d (%s): el contenido cambio al reusar el prestamo", lectura, nombre)
		}
	}

	leer(pruebaComprimi)

	crecidas, reservado := prestamo.Crecidas(), prestamo.Estado().Reservado
	if crecidas == 0 || reservado == 0 {
		t.Fatalf("la primera lectura tiene que pagar el prestamo: crecidas=%d reservado=%d", crecidas, reservado)
	}

	// las vueltas siguientes: diez archivos mas, ninguna reserva nueva
	for i := 0; i < 10; i++ {
		leer(pruebaComprimi)
		leer(pruebaPlano)
	}

	if prestamo.Crecidas() != crecidas {
		t.Fatalf("el prestamo volvio a pedir memoria: %d reservas contra %d de la primera lectura",
			prestamo.Crecidas(), crecidas)
	}

	if prestamo.Estado().Reservado != reservado {
		t.Fatalf("el prestamo cambio de tamano: %d contra %d", prestamo.Estado().Reservado, reservado)
	}

	if prestamo.Estado().Lecturas != lectura {
		t.Fatalf("el instrumento cuenta %d lecturas y se hicieron %d", prestamo.Estado().Lecturas, lectura)
	}

	// y del monton: lo que pide una lectura con el prestamo caliente no puede parecerse a un buffer por
	// sector (el archivo tiene 3 sectores) ni a un buffer por archivo.
	medido, bytesPorVuelta := memoriaPorLectura(t, m, prestamo, pruebaComprimi, dst)
	t.Logf("con el prestamo caliente, una lectura de %d B (3 sectores) pide %.2f objetos y %.0f B",
		len(quiero[pruebaComprimi]), medido, bytesPorVuelta)

	if medido > 16 {
		t.Fatalf("una lectura con prestamo caliente pidio %.2f objetos: el camino tiene que no pedir memoria", medido)
	}

	// 2 KB es holgado para unos pocos objetos chicos (el `Block` de la tabla) y corto para cualquier
	// reserva por sector (un solo sector descompreso ya son 4 KB)
	if bytesPorVuelta > 2048 {
		t.Fatalf("una lectura con prestamo caliente pidio %.0f B: eso es una reserva por sector", bytesPorVuelta)
	}
}

// memoriaPorLectura mide cuantos objetos y cuantos bytes pide UNA lectura con el prestamo ya caliente.
func memoriaPorLectura(t *testing.T, m *MPQ, prestamo *Scratch, nombre string, dst []byte) (float64, float64) {
	t.Helper()

	const vueltas = 20

	var antes, despues runtime.MemStats

	runtime.GC()
	runtime.ReadMemStats(&antes)

	for i := 0; i < vueltas; i++ {
		if _, err := m.ReadFileInto(nombre, dst, prestamo); err != nil {
			t.Fatalf("midiendo asignaciones: %v", err)
		}
	}

	runtime.ReadMemStats(&despues)

	if despues.Mallocs < antes.Mallocs || despues.TotalAlloc < antes.TotalAlloc {
		t.Fatalf("los contadores de asignacion se movieron para atras")
	}

	return float64(despues.Mallocs-antes.Mallocs) / vueltas,
		float64(despues.TotalAlloc-antes.TotalAlloc) / vueltas
}

// TestReadFileIntoNoEntraEnElBuffer: con un destino corto se dice y no se lee de mas. El MUTADOR es el propio
// caso: un byte menos falla, el tamano justo anda.
func TestReadFileIntoNoEntraEnElBuffer(t *testing.T) {
	m := abrirDePrueba(t)

	tamano, _ := m.TamanoDeArchivo(pruebaComprimi)
	prestamo := &Scratch{}

	if _, err := m.ReadFileInto(pruebaComprimi, make([]byte, tamano-1), prestamo); err != ErrNoEntraEnElBuffer {
		t.Fatalf("con el buffer corto quiero ErrNoEntraEnElBuffer, dio %v", err)
	}

	if _, err := m.ReadFileInto(pruebaComprimi, make([]byte, tamano), prestamo); err != nil {
		t.Fatalf("con el buffer justo no tiene que fallar: %v", err)
	}

	// y sin prestamo se cae al camino de `LeerEn` (no se rompe por pedir `scratch == nil`)
	if _, err := m.ReadFileInto(pruebaComprimi, make([]byte, tamano), nil); err != nil {
		t.Fatalf("con scratch nil tiene que leer por LeerEn: %v", err)
	}
}

// TestExplodeEnDentroDelArregloPrestado: el decodificador de PKWare escribiendo en un arreglo prestado da el
// MISMO resultado que el camino creciente, y si la salida no entra lo dice en vez de pedir memoria.
func TestExplodeEnDentroDelArregloPrestado(t *testing.T) {
	// entradas que SI comprimen (el escritor de blast emite streams rotos para datos incompresibles, y el
	// lector viejo tampoco los lee: esos casos se saltan)
	generadores := []func(int) []byte{
		func(n int) []byte { return bytes.Repeat([]byte{0x41}, n) },
		func(n int) []byte {
			patron := []byte("el pueblo de Tristram y sus tiles ")
			return bytes.Repeat(patron, n/len(patron)+1)[:n]
		},
		func(n int) []byte {
			datos := make([]byte, n)
			for i := range datos {
				datos[i] = byte((i * 37) + (i / 7))
			}

			return datos
		},
	}

	casos := 0

	for _, n := range []int{1, 2, 3, 100, 4096, 4097, 20000} {
		for gi, generar := range generadores {
			datos := generar(n)

			for _, typ := range []uint{blast.Binary, blast.ASCII} {
				comprimido, ok := blastCompress(datos, typ, blast.DictionarySize4096)
				if !ok {
					continue
				}

				quiero, err := explode(comprimido.Bytes(), 0)
				if err != nil {
					continue // el escritor de blast emite streams rotos para algunas entradas
				}

				got := make([]byte, len(quiero))

				leidos, err := explodeEn(comprimido.Bytes(), got)
				if err != nil {
					t.Fatalf("n=%d g=%d typ=%d: explodeEn: %v", n, gi, typ, err)
				}

				if leidos != len(quiero) || !bytes.Equal(got[:leidos], quiero) {
					t.Fatalf("n=%d g=%d typ=%d: explodeEn NO da lo mismo que explode (%d contra %d bytes)",
						n, gi, typ, leidos, len(quiero))
				}

				// con un byte menos de lugar tiene que decir que no entra
				if len(quiero) > 0 {
					if _, err := explodeEn(comprimido.Bytes(), make([]byte, len(quiero)-1)); err == nil {
						t.Fatalf("n=%d g=%d typ=%d: con el arreglo corto no fallo", n, gi, typ)
					}
				}

				casos++
			}
		}
	}

	if casos < 5 {
		t.Fatalf("solo %d casos comparados: la prueba no probo nada", casos)
	}

	t.Logf("comparados %d streams de PKWare con el arreglo prestado", casos)
}

// TestPrestamoEnElArchivoRealDeJuego: por cada bloque comprimido de d2data.mpq, el camino con prestamo tiene
// que dar los MISMOS bytes que el camino de siempre. Corre solo si esta D2_GAME_DIR (el resto de las pruebas
// del paquete usa archivos sinteticos: nada del juego se versiona).
func TestPrestamoEnElArchivoRealDeJuego(t *testing.T) {
	dir := os.Getenv("D2_GAME_DIR")
	if dir == "" {
		t.Skip("D2_GAME_DIR not set")
	}

	m, err := FromFile(filepath.Join(dir, "d2data.mpq"))
	if err != nil {
		t.Skip(err)
	}

	defer m.Close()

	prestamo := &Scratch{}

	comparados, sectores := 0, 0

	for i, entradas := uint32(0), m.blockEntries(); i < entradas; i++ {
		b := m.bloqueEn(i)
		if b.UncompressedFileSize == 0 || b.UncompressedFileSize > 1<<20 ||
			b.HasFlag(FileEncrypted) || b.HasFlag(FileSingleUnit) || b.HasFlag(FilePatchFile) ||
			(!b.HasFlag(FileCompress) && !b.HasFlag(FileImplode)) {
			continue
		}

		quiero := make([]byte, b.UncompressedFileSize)

		bloque := *b

		stream, err := CreateStream(m, &bloque, "")
		if err != nil {
			continue // un bloque que el camino de siempre tampoco abre
		}

		if _, err := stream.Read(quiero, 0, b.UncompressedFileSize); err != nil {
			continue
		}

		got := make([]byte, len(quiero))
		antes := prestamo.Estado().Reservado

		leidos, err := m.leerBloqueEnPrestamo(b, "", got, prestamo)
		if err != nil {
			t.Fatalf("bloque %d (posicion %d): el prestamo fallo y el camino de siempre no: %v",
				i, b.FilePosition, err)
		}

		if leidos != len(quiero) || !bytes.Equal(got[:leidos], quiero) {
			t.Fatalf("bloque %d (posicion %d): el prestamo NO da los mismos bytes (%d contra %d)",
				i, b.FilePosition, leidos, len(quiero))
		}

		if antes != prestamo.Estado().Reservado && comparados > 0 {
			t.Logf("bloque %d: el prestamo crecio de %d a %d B (archivo de %d B)",
				i, antes, prestamo.Estado().Reservado, b.UncompressedFileSize)
		}

		comparados++
	}

	sectores = prestamo.Estado().Sectores

	t.Logf("comparados %d bloques comprimidos del archivo real, %d sectores descomprimidos en el prestamo (%d B)",
		comparados, sectores, prestamo.Estado().Reservado)

	if comparados < 100 {
		t.Fatalf("solo %d bloques comparados: la prueba no probo el archivo real", comparados)
	}
}

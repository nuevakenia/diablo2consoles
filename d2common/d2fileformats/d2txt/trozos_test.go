package d2txt

import (
	"fmt"
	"io"
	"strings"
	"testing"
)

// lectorAnotado: entrega los datos de a lo sumo `tope` bytes por Read y ANOTA el pedido mas grande que le
// hicieron. Es el mismo instrumento que `d2fileformats/d2mpq/mpq_trozos_test.go` usa para las tablas del
// MPQ (DC-002.28), y aca mide lo mismo un escalon mas arriba: que la tabla NO se pida entera de un tiron.
type lectorAnotado struct {
	datos   []byte
	pos     int
	tope    int
	mayor   int
	vueltas int
}

func (l *lectorAnotado) Read(p []byte) (int, error) {
	l.vueltas++

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

// tablaDePrueba arma una tabla de `filas` filas del mismo formato que las del juego.
func tablaDePrueba(filas int) []byte {
	var sb strings.Builder
	sb.WriteString("Name\tVal\n")

	for i := 0; i < filas; i++ {
		fmt.Fprintf(&sb, "fila%d\t%d\n", i, i*3)
	}

	return []byte(sb.String())
}

// TestLoadDataDictionaryReaderPorTrozos: la tabla se consume por trozos -- el lector NUNCA recibe un pedido
// del tamano del archivo -- y las filas salen enteras y en orden.
//
// POR QUE ESTE CONTROL (DC-002.45): el defecto que arregla la tarjeta no es de la tabla sino de la LECTURA
// (`ioutil.ReadAll` de `LoadFile`): el archivo entero tenia que entrar en el semiespacio del GC de la consola
// antes de parsear la primera fila (134.937 B contra 132.104 libres). Lo que se mide aca es la propiedad que
// hace que eso no vuelva a pasar: el pedido mas grande del lector.
func TestLoadDataDictionaryReaderPorTrozos(t *testing.T) {
	datos := tablaDePrueba(500)
	lec := &lectorAnotado{datos: datos, tope: 512}

	d := LoadDataDictionaryReader(lec)

	n := 0
	for d.Next() {
		if got := d.Number("Val"); got != n*3 {
			t.Fatalf("fila %d: Val=%d, queria %d", n, got, n*3)
		}

		n++
	}

	if d.Err != nil {
		t.Fatalf("Err: %v", d.Err)
	}

	if n != 500 {
		t.Fatalf("filas leidas %d, queria 500", n)
	}

	if lec.mayor >= len(datos) {
		t.Fatalf("el lector recibio un pedido de %d bytes con un archivo de %d: la tabla se pidio ENTERA",
			lec.mayor, len(datos))
	}

	if lec.mayor > 4096 {
		t.Fatalf("el pedido mas grande fue %d bytes: la tabla no se lee por trozos", lec.mayor)
	}
}

// TestLoadDataDictionaryReaderTablaMasGrandeQueElSemiespacio: una tabla de 1,5 MB -- mas grande que el
// semiespacio del GC de la consola (512 KB), o sea imposible de tener entera ahi -- se recorre ENTERA por el
// stream, y el pedido mas grande del lector sigue siendo de un trozo. Es el numero que la consola no puede
// medir en el host, medido por el lado que si se puede: lo que el consumidor le PIDE al lector.
func TestLoadDataDictionaryReaderTablaMasGrandeQueElSemiespacio(t *testing.T) {
	const filas = 40000 // ~620 KB de tabla (cada fila son ~15 bytes)

	datos := tablaDePrueba(filas)
	if len(datos) < 524288 {
		t.Fatalf("la tabla de prueba son %d bytes: no pasa el semiespacio de la consola", len(datos))
	}

	lec := &lectorAnotado{datos: datos, tope: 512}

	d := LoadDataDictionaryReader(lec)

	n := 0
	for d.Next() {
		n++
	}

	if n != filas {
		t.Fatalf("filas leidas %d, queria %d", n, filas)
	}

	if lec.mayor > 4096 {
		t.Fatalf("el pedido mas grande fue %d bytes: la tabla no se lee por trozos", lec.mayor)
	}
}

// TestBytesYStreamDanLasMismasFilas: el camino viejo (los bytes del archivo) y el nuevo (el stream) tienen
// que dar EXACTAMENTE las mismas filas. Es el control del mutante por el lado del host: si alguien vuelve al
// `ReadAll`, este control no se pone rojo (los dos caminos tienen que leerse igual) -- el que caza la vuelta
// atras es el corpus de la consola (`scripts/console/checar-trozos-245.sh`), que mide la MEMORIA.
func TestBytesYStreamDanLasMismasFilas(t *testing.T) {
	datos := tablaDePrueba(200)

	porBytes := LoadDataDictionary(datos)
	porStream := LoadDataDictionaryReader(strings.NewReader(string(datos)))

	for i := 0; ; i++ {
		hayB := porBytes.Next()
		hayS := porStream.Next()

		if hayB != hayS {
			t.Fatalf("fila %d: por bytes=%v por stream=%v", i, hayB, hayS)
		}

		if !hayB {
			break
		}

		if porBytes.String("Name") != porStream.String("Name") || porBytes.Number("Val") != porStream.Number("Val") {
			t.Fatalf("fila %d: bytes=%q/%d stream=%q/%d", i,
				porBytes.String("Name"), porBytes.Number("Val"),
				porStream.String("Name"), porStream.Number("Val"))
		}
	}
}

package difftest

// DC-002.45: la tabla REAL del juego (los bytes que salen del MPQ) leida por el csv de la CONSOLA y por el de
// Go, fila por fila, con el lector entregando los datos POR TROZOS (4096, que es el sector del MPQ).
//
// POR QUE CON EL ARCHIVO REAL: el arreglo de la tarjeta cambia la tabla de "los bytes enteros en memoria" a
// "el stream consumido por trozos", y lo que NO puede cambiar es lo que sale. Las tablas del juego son
// irregulares (filas de distinto largo, comillas sueltas): un generador sintetico no dice si el camino nuevo
// lee BIEN `LvlPrest.txt` (134.937 B, la tabla que el arranque no pasaba -- DC-002.43).
//
// El archivo lo deja `scripts/console/checar-trozos-245.sh` en la ruta de abajo (sacado del MPQ con el d2mpq
// del arbol, sin usar el codigo del motor). Si no esta, la prueba se SALTA: no falla por un archivo que no
// viaja en el repo.
//
// DC-002.47: CADA LADO LEE CON SU EOF. `d2std/io.EOF` es un valor propio (`errors.New("EOF")`) distinto del
// `io.EOF` de Go, y las dos implementaciones comparan por IDENTIDAD (`err == io.EOF` adentro de cada
// paquete). Un stream de prueba que le devuelva a la consola el `io.EOF` del host no dispara ninguna de sus
// ramas de fin y la prueba mide otra cosa (medido: la traza dio `readLine bytes=5 err=SI` con el EOF de Go y
// `err=no` con el de la consola). Por eso el mismo stream se arma DOS veces, una por lado, con el EOF de cada
// implementacion; y las comparaciones de fin van por `esEOF` (el TEXTO del error), porque el mismo fin es un
// valor distinto en cada lado.

import (
	"bytes"
	realcsv "encoding/csv"
	"os"
	"testing"

	mybytes "d2std/bytes"
	mycsv "d2std/encoding/csv"
	mio "d2std/io"
)

const tablaReal245 = "/home/dgalli/.hermes/cache/scratch/trozos-245/tablas/LvlPrest.txt"

// lectorContado: entrega los datos de a lo sumo `tope` bytes por Read (lo que hace el stream del MPQ, que lee
// por SECTOR de 4096) y ANOTA el tamano de cada pedido. El largo de la lista `pedidos` dice si el consumidor
// pidio la tabla de un tiron (1 pedido) o la fue llevando por trozos (decenas). `fin` es el EOF DE ESE LADO
// (io.EOF para el csv de Go, mio.EOF para el de la consola): ver la cabecera.
type lectorContado struct {
	datos   []byte
	pos     int
	tope    int
	fin     error
	pedidos []int
}

func (l *lectorContado) Read(p []byte) (int, error) {
	l.pedidos = append(l.pedidos, len(p))

	if l.pos >= len(l.datos) {
		return 0, l.fin
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

// TestTablaRealLvlPrestPorTrozosIgualQueGo: `LvlPrest.txt` leida por trozos de 4096 con el csv de la consola y
// con el de Go: mismas filas, mismos campos, mismos finales. Y el lector tiene que haber sido llamado muchas
// veces (el camino nuevo NO materializa el archivo).
func TestTablaRealLvlPrestPorTrozosIgualQueGo(t *testing.T) {
	datos, err := os.ReadFile(tablaReal245)
	if err != nil {
		t.Skipf("no esta la tabla real (%s): la saca checar-trozos-245.sh", tablaReal245)
	}

	if len(datos) != 134937 {
		t.Fatalf("la tabla son %d bytes y se esperaban 134937 (¿es otro archivo?)", len(datos))
	}

	// --- la consola: por trozos de 4096 (lo que hace el MPQ), con el EOF de la consola ---
	lec := &lectorContado{datos: datos, tope: 4096, fin: mio.EOF}
	csvConsola := mycsv.NewReader(lec)
	csvConsola.Comma = '\t'
	csvConsola.ReuseRecord = true
	csvConsola.FieldsPerRecord = -1 // las tablas del juego tienen filas de largo distinto
	csvConsola.LazyQuotes = true

	// --- Go: la tabla entera de una (los mismos bytes), con el EOF de Go ---
	deGo := realcsv.NewReader(bytes.NewReader(datos))
	deGo.Comma = '\t'
	deGo.ReuseRecord = true
	deGo.FieldsPerRecord = -1
	deGo.LazyQuotes = true

	n := 0
	pedidoMayor := 0

	for {
		filaGo, errGo := deGo.Read()
		filaMia, errMio := csvConsola.Read()

		if esEOF(errGo) != esEOF(errMio) {
			t.Fatalf("fila %d: Go err=%v, consola err=%v", n, errGo, errMio)
		}

		if esEOF(errGo) {
			break
		}

		if errGo != nil {
			t.Fatalf("Go fallo en la fila %d: %v", n, errGo)
		}

		if errMio != nil {
			t.Fatalf("la consola fallo en la fila %d (Go la leyo): %v", n, errMio)
		}

		if len(filaGo) != len(filaMia) {
			t.Fatalf("fila %d: %d campos en Go y %d en la consola", n, len(filaGo), len(filaMia))
		}

		for i := range filaGo {
			if filaGo[i] != filaMia[i] {
				t.Fatalf("fila %d campo %d: Go=%q consola=%q", n, i, filaGo[i], filaMia[i])
			}
		}

		n++
	}

	if n < 1000 {
		t.Fatalf("filas leidas %d: muy pocas para decir que la tabla se leyo", n)
	}

	for _, p := range lec.pedidos {
		if p > pedidoMayor {
			pedidoMayor = p
		}
	}

	if len(lec.pedidos) < 30 {
		t.Fatalf("el lector fue llamado %d veces por una tabla de %d bytes: se la pidio de un tiron",
			len(lec.pedidos), len(datos))
	}

	if pedidoMayor > 4096 {
		t.Fatalf("el pedido mayor del lector fue de %d bytes: la tabla no se lleva por trozos", pedidoMayor)
	}

	t.Logf("filas=%d pedidos=%d pedido mayor=%d B (la tabla son %d B)", n, len(lec.pedidos), pedidoMayor, len(datos))
}

// TestTablaRealEnteraYPorTrozosDanLoMismo: los dos caminos del MISMO archivo real (los bytes enteros, como el
// camino viejo, y el stream por trozos, como el nuevo) tienen que dar exactamente las mismas filas. Es el
// control de EQUIVALENCIA del arreglo por el lado del host: si el camino por trozos comiera o repitiera una
// fila, esto se pone rojo.
func TestTablaRealEnteraYPorTrozosDanLoMismo(t *testing.T) {
	datos, err := os.ReadFile(tablaReal245)
	if err != nil {
		t.Skipf("no esta la tabla real (%s): la saca checar-trozos-245.sh", tablaReal245)
	}

	entera := mycsv.NewReader(mybytes.NewReader(datos))
	entera.Comma = '\t'
	entera.ReuseRecord = false
	entera.FieldsPerRecord = -1
	entera.LazyQuotes = true

	trozos := mycsv.NewReader(&lectorContado{datos: datos, tope: 4096, fin: mio.EOF})
	trozos.Comma = '\t'
	trozos.ReuseRecord = false
	trozos.FieldsPerRecord = -1
	trozos.LazyQuotes = true

	n := 0

	for {
		a, errA := entera.Read()
		b, errB := trozos.Read()

		if esEOF(errA) != esEOF(errB) {
			t.Fatalf("fila %d: entera err=%v, por trozos err=%v", n, errA, errB)
		}

		if esEOF(errA) {
			break
		}

		if errA != nil || errB != nil {
			t.Fatalf("fila %d: entera err=%v, por trozos err=%v", n, errA, errB)
		}

		if len(a) != len(b) {
			t.Fatalf("fila %d: %d campos enteros y %d por trozos", n, len(a), len(b))
		}

		for i := range a {
			if a[i] != b[i] {
				t.Fatalf("fila %d campo %d: entera=%q por trozos=%q", n, i, a[i], b[i])
			}
		}

		n++
	}

	if n < 1000 {
		t.Fatalf("filas leidas %d: muy pocas", n)
	}

	t.Logf("filas iguales por los dos caminos: %d", n)
}

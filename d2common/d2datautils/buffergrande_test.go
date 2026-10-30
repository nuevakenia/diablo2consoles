package d2datautils

import (
	"bytes"
	"testing"
)

// DC-002.78 -- LA PRUEBA DEL BUFFER REUSADO.
//
// Lo que hay que probar es la propiedad que hace falta en la consola: leer dos archivos grandes NO pide
// memoria dos veces. Se comprueba por la DIRECCION: si el segundo archivo se lee en el mismo arreglo que el
// primero, no hubo pedido nuevo (en la Dreamcast cada pedido de mas de 64 KB no lo devuelve nadie).
func TestBufferGrandeReusaElMismoArreglo(t *testing.T) {
	var b BufferGrande

	const tamano = 900 * 1024 // un .dt1 tipico del acto 1, bien por encima del umbral de objeto grande

	b.Reservar(tamano)

	if b.Crecidas() != 0 {
		t.Fatalf("reservar no deberia contar como crecida: %d", b.Crecidas())
	}

	primero := bytes.Repeat([]byte{0xAA}, tamano)
	segundo := bytes.Repeat([]byte{0xBB}, tamano/2)

	leido1, err := b.LeerTodo(bytes.NewReader(primero))
	if err != nil {
		t.Fatalf("primera lectura: %v", err)
	}

	if len(leido1) != len(primero) || leido1[0] != 0xAA || leido1[len(leido1)-1] != 0xAA {
		t.Fatalf("primera lectura mal: %d bytes", len(leido1))
	}

	leido2, err := b.LeerTodo(bytes.NewReader(segundo))
	if err != nil {
		t.Fatalf("segunda lectura: %v", err)
	}

	if len(leido2) != len(segundo) || leido2[0] != 0xBB || leido2[len(leido2)-1] != 0xBB {
		t.Fatalf("segunda lectura mal: %d bytes", len(leido2))
	}

	if b.Crecidas() != 0 {
		t.Fatalf("la segunda lectura pidio memoria: crecidas = %d", b.Crecidas())
	}

	if &leido1[0] != &leido2[0] {
		t.Fatal("el segundo archivo se leyo en OTRO arreglo: el buffer no se reuso")
	}

	if b.ArchivoMasGrande() != tamano {
		t.Fatalf("archivo mas grande = %d, se esperaba %d", b.ArchivoMasGrande(), tamano)
	}
}

// TestBufferGrandeCreceSiNoAlcanza: si un archivo no entra, el buffer crece (y lo dice). El numero tiene que
// quedarse en cero en la consola; aca se prueba que el instrumento lo delata cuando pasa.
func TestBufferGrandeCreceSiNoAlcanza(t *testing.T) {
	var b BufferGrande

	b.Reservar(1024)

	if _, err := b.LeerTodo(bytes.NewReader(bytes.Repeat([]byte{1}, 4096))); err != nil {
		t.Fatalf("lectura: %v", err)
	}

	if b.Crecidas() == 0 {
		t.Fatal("no se conto la crecida: el instrumento no sirve para medir")
	}

	if b.Capacidad() < 4096 {
		t.Fatalf("capacidad %d < 4096", b.Capacidad())
	}
}

// TestBufferGrandeVacio: un archivo vacio no puede romper nada (y no pide memoria).
func TestBufferGrandeVacio(t *testing.T) {
	var b BufferGrande

	b.Reservar(64)

	leido, err := b.LeerTodo(bytes.NewReader(nil))
	if err != nil {
		t.Fatalf("lectura vacia: %v", err)
	}

	if len(leido) != 0 {
		t.Fatalf("se esperaba 0 bytes, salieron %d", len(leido))
	}

	if b.Crecidas() != 0 {
		t.Fatalf("un archivo vacio no deberia pedir memoria: %d", b.Crecidas())
	}
}

package asset

import (
	"fmt"
	"io"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2mpq"
)

// Source is an abstraction for something that can load and list assets
type Source interface {
	fmt.Stringer
	Open(name string) (io.ReadSeeker, error)
	Path() string
	Exists(subPath string) bool
}

// SourceDeLecturaEntera lo implementan las fuentes que saben cuanto mide un archivo y pueden leerlo
// entero DENTRO de un buffer que les presta el que llama (DC-002.79).
//
// PARA QUE. `Loader.Load` devuelve un `io.ReadSeeker` y no dice el tamano, asi que el que quiere el
// archivo entero termina en `io.ReadAll`, que va duplicando el buffer hasta pasarse del tamano. En la
// consola esa cadena de duplicaciones deja ~2 MB tomados por archivo grande
// (`gc_external_alloc`, medido en DC-002.78). Con esta interfaz el que llama reserva UNA vez el tamano
// exacto (`Tamano`) y lee ahi adentro (`LeerEnteroEn`), sin que la fuente pida memoria.
//
// Es OPCIONAL a proposito: es una interfaz aparte (no un metodo nuevo en `Source`) para no romper a las
// fuentes que no saben el tamano de antemano. El loader la busca con un type assertion y, si la fuente no
// la implementa, sigue por el camino de siempre.
type SourceDeLecturaEntera interface {
	// Tamano devuelve el tamano descomprimido del archivo (los bytes que hay que reservar).
	Tamano(nombre string) (int, bool)
	// LeerEnteroEn lee el archivo entero en dst y devuelve cuantos bytes escribio. dst tiene que
	// alcanzar para `Tamano`.
	LeerEnteroEn(nombre string, dst []byte) (int, error)
}

// SourceDeLecturaConPrestamo lo implementan las fuentes que ademas saben leer SIN PEDIR MEMORIA DE TRABAJO:
// la tabla de posiciones de los sectores, el bloque comprimido y el bloque descompreso se los presta el que
// llama, en un `d2mpq.Scratch` que se reusa archivo tras archivo (DC-002.80).
//
// PARA QUE. `LeerEnteroEn` no pide el DESTINO, pero la lectura adentro del MPQ si pide: un bloque comprimido
// y otro descompreso por sector, mas el descompresor de zlib (ver `d2mpq/prestamo.go`). Medido en DC-002.78:
// leer UN .dt1 de 505 KB por el stream pedia ~2 MB del semiespacio del recolector de la consola y el runtime
// moria con `gc_alloc: out of memory`. Con el prestamo, la lectura de un archivo de 1 MB se hace con 14 KB
// de memoria de trabajo, pedidos una sola vez.
//
// Es OPCIONAL, como `SourceDeLecturaEntera`: el loader la busca con un type assertion y, si la fuente no la
// implementa, se sigue por el camino de siempre.
type SourceDeLecturaConPrestamo interface {
	// LeerEnteroEnConPrestamo lee el archivo entero en dst usando `prestamo` como memoria de trabajo.
	// dst tiene que alcanzar para `Tamano` y el prestamo NO se puede compartir entre goroutines.
	LeerEnteroEnConPrestamo(nombre string, dst []byte, prestamo *d2mpq.Scratch) (int, error)
}

// TramoPrestado es MEMORIA PRESTADA CON DUENO EXPLICITO: el que la pide la usa y la devuelve con `Liberar`
// (DC-002.99).
//
// PARA QUE. `LoadFile` devuelve un `[]byte` sin dueno: nadie sabe a quien le toca devolverlo, asi que en la
// consola el bloque que el runtime mando al monton externo (>64 KB, `gc_external_alloc`) se queda tomado
// para siempre. Con un tramo, el que pide la memoria es el que la devuelve, y el que la devuelve sabe cuando
// (al terminar la lectura, o cuando la reusa para el archivo siguiente).
type TramoPrestado interface {
	// Bytes devuelve el tramo. Vale hasta el proximo `Liberar`.
	Bytes() []byte
	// Liberar devuelve el tramo a la fuente. Es idempotente: dos liberaciones no liberan dos veces.
	Liberar()
}

// SourceDeTramos lo implementan las fuentes que saben PRESTAR memoria de lectura con dueno explicito
// (DC-002.99): la fuente decide de donde sale (en la consola, del monton de C con el `free` inyectado) y el
// que llama la devuelve cuando termino.
//
// Es OPCIONAL, como las otras dos: una fuente que no la implemente (una carpeta del disco) sigue por el
// camino de siempre. El loader la busca con un type assertion.
type SourceDeTramos interface {
	// Prestar devuelve un tramo de AL MENOS `minimo` bytes para leer `nombre` adentro. `nombre` es para el
	// instrumento de la fuente; el que lee es el que llama (`LeerEnteroEn` / `LeerEnteroEnConPrestamo`).
	Prestar(nombre string, minimo int) (TramoPrestado, error)
}

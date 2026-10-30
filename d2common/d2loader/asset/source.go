package asset

import (
	"fmt"
	"io"
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

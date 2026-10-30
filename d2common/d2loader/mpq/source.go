package mpq

import (
	"errors"
	"io"
	"strings"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2mpq"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2interface"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2loader/asset"
)

// static check that Source implements AssetSource
var _ asset.Source = &Source{}

// DC-002.79: ademas sabe cuanto mide un archivo y lo lee entero en un buffer prestado.
var _ asset.SourceDeLecturaEntera = &Source{}

// DC-002.80: y lo lee con la memoria de trabajo prestada por el que llama (sin pedir nada por sector).
var _ asset.SourceDeLecturaConPrestamo = &Source{}

// errSinLecturaEntera: la fuente tiene un Archive que no es un MPQ concreto (no puede decir el tamano).
var errSinLecturaEntera = errors.New("mpq: la fuente no sabe leer enteros")

// NewSource creates a new MPQ Source
func NewSource(sourcePath string) (asset.Source, error) {
	loaded, err := d2mpq.FromFile(sourcePath)
	if err != nil {
		return nil, err
	}

	return &Source{loaded}, nil
}

// Source is an implementation of an asset source for MPQ archives
type Source struct {
	MPQ d2interface.Archive
}

// Open attempts to open a file within the MPQ archive
func (v *Source) Open(name string) (a io.ReadSeeker, err error) {
	name = cleanName(name)
	return v.MPQ.ReadFileStream(name)
}

// Exists returns true if the file exists
func (v *Source) Exists(subPath string) bool {
	subPath = cleanName(subPath)
	return v.MPQ.Contains(subPath)
}

// Tamano devuelve el tamano descomprimido del archivo (DC-002.79). Ver asset.SourceDeLecturaEntera.
func (v *Source) Tamano(nombre string) (int, bool) {
	mpqDirecto, ok := v.MPQ.(*d2mpq.MPQ)
	if !ok {
		return 0, false
	}

	return mpqDirecto.TamanoDeArchivo(cleanName(nombre))
}

// LeerEnteroEn lee el archivo entero DENTRO del buffer prestado (DC-002.79).
func (v *Source) LeerEnteroEn(nombre string, dst []byte) (int, error) {
	mpqDirecto, ok := v.MPQ.(*d2mpq.MPQ)
	if !ok {
		return 0, errSinLecturaEntera
	}

	return mpqDirecto.LeerEn(cleanName(nombre), dst)
}

// LeerEnteroEnConPrestamo lee el archivo entero con la memoria de trabajo prestada (DC-002.80): la tabla de
// posiciones, el bloque comprimido, el descompreso y el descompresor de zlib viven en `prestamo` y se reusan
// entre archivos. dst tiene que alcanzar para `Tamano`.
func (v *Source) LeerEnteroEnConPrestamo(nombre string, dst []byte, prestamo *d2mpq.Scratch) (int, error) {
	mpqDirecto, ok := v.MPQ.(*d2mpq.MPQ)
	if !ok {
		return 0, errSinLecturaEntera
	}

	return mpqDirecto.ReadFileInto(cleanName(nombre), dst, prestamo)
}

// Path returns the path of the MPQ on the host filesystem
func (v *Source) Path() string {
	return v.MPQ.Path()
}

// String returns the path
func (v *Source) String() string {
	return v.Path()
}

func cleanName(name string) string {
	name = strings.ReplaceAll(name, "/", "\\")

	if string(name[0]) == "\\" {
		name = name[1:]
	}

	return name
}

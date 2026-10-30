package d2loader

import (
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2loader/mpq"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2loader/filesystem"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2cache"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2mpq"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2interface"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2loader/asset"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2loader/asset/types"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2resource"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2util"
)

const (
	defaultCacheBudget = 1024 * 1024 * 512
	errFmtFileNotFound = "file not found: %s"
)

const (
	logPrefix = "File Loader"
)

const (
	fontToken  = d2resource.LanguageFontToken
	tableToken = d2resource.LanguageTableToken
)

// NewLoader creates a new loader
func NewLoader(l d2util.LogLevel) (*Loader, error) {
	loader := &Loader{
		LoaderProviders: make(map[types.SourceType]func(path string) (asset.Source, error), 2),
	}

	loader.LoaderProviders[types.AssetSourceMPQ] = mpq.NewSource
	loader.LoaderProviders[types.AssetSourceFileSystem] = filesystem.OnAddSource

	loader.Cache = d2cache.CreateCache(defaultCacheBudget)
	loader.Logger = d2util.NewLogger()

	loader.Logger.SetPrefix(logPrefix)
	loader.Logger.SetLevel(l)

	return loader, nil
}

// Loader represents the manager that handles loading and caching assets with the asset Sources
// that have been added
type Loader struct {
	language *string
	charset  *string
	d2interface.Cache
	*d2util.Logger
	LoaderProviders map[types.SourceType]func(path string) (asset.Source, error)
	Sources         []asset.Source
}

// SetLanguage sets the language for loader
func (l *Loader) SetLanguage(language *string) {
	l.language = language
}

// SetCharset sets the charset for loader
func (l *Loader) SetCharset(charset *string) {
	l.charset = charset
}

// Load attempts to load an asset with the given sub-path. The sub-path is relative to the root
// of each asset source root (regardless of the type of asset source)
func (l *Loader) Load(subPath string) (io.ReadSeeker, error) {
	subPath = l.rutaReal(subPath)

	// DC-002.43: la marca de ENTRAR va antes de tocar ninguna fuente: si la carga se muere adentro, esta es
	// la ultima linea que queda y nombra el pedido. Los bytes de la ruta no se copian (ver marca_carga.go).
	Marca(subPath, MarcaEntra)

	// if it isn't in the cache, we check if each source can open the file
	for idx := range l.Sources {
		source := l.Sources[idx]

		// if the source can open the file, then we cache it and return it
		loadedAsset, err := source.Open(subPath)
		if err != nil {
			l.Debug(fmt.Sprintf("Checked `%s`, file not found", source.Path()))
			continue
		}

		srcBase, _ := filepath.Abs(source.Path())
		l.Info(fmt.Sprintf("Loaded %s -> %s", srcBase, subPath))

		// DC-002.43: la carga VOLVIO. Si despues de esta marca no hay ninguna otra, el motor no murio en el
		// MPQ: murio en lo que vino despues (el parseo del archivo, por ejemplo).
		Marca(subPath, MarcaVolvio)

		return loadedAsset, nil
	}

	return nil, fmt.Errorf(errFmtFileNotFound, subPath)
}

// rutaReal aplica al sub-path lo mismo que cada camino del loader: limpiar y sustituir los tokens de
// idioma. Es UN lugar para no tener dos versiones de la traduccion (DC-002.79).
func (l *Loader) rutaReal(subPath string) string {
	subPath = filepath.Clean(subPath)

	if l.language != nil {
		charset := l.charset
		language := l.language

		subPath = strings.ReplaceAll(subPath, fontToken, *charset)
		subPath = strings.ReplaceAll(subPath, tableToken, *language)
	}

	return subPath
}

// TamanoDeArchivo dice cuanto mide un archivo (bytes descomprimidos) segun la primera fuente que lo
// tenga y que sepa decirlo. `false` significa "no se sabe": o el archivo no esta, o la fuente no
// implementa `asset.SourceDeLecturaEntera` (una carpeta del disco, por ejemplo).
//
// DC-002.79: es la pieza que le falta a `io.ReadAll` para no ir duplicando el buffer. Ver el porque en
// `d2mpq/lectura_entera.go`.
func (l *Loader) TamanoDeArchivo(subPath string) (int, bool) {
	subPath = l.rutaReal(subPath)

	for idx := range l.Sources {
		source := l.Sources[idx]

		fuente, ok := source.(asset.SourceDeLecturaEntera)
		if !ok {
			continue
		}

		if tamano, hay := fuente.Tamano(subPath); hay {
			return tamano, true
		}
	}

	return 0, false
}

// LeerEnteroEn lee el archivo entero DENTRO del buffer que le prestan, de la primera fuente que lo tenga
// y sepa hacerlo. `dst` tiene que alcanzar para `TamanoDeArchivo`; no se pide memoria aca.
//
// DC-002.79: este es el camino de la consola. No reemplaza a `Load` (que devuelve un stream): es el que
// usa `d2asset.LoadFile` para no pasar por `io.ReadAll`.
func (l *Loader) LeerEnteroEn(subPath string, dst []byte) (int, error) {
	subPath = l.rutaReal(subPath)

	for idx := range l.Sources {
		source := l.Sources[idx]

		fuente, ok := source.(asset.SourceDeLecturaEntera)
		if !ok || !source.Exists(subPath) {
			continue
		}

		return fuente.LeerEnteroEn(subPath, dst)
	}

	return 0, fmt.Errorf(errFmtFileNotFound, subPath)
}

// LeerEnteroEnConPrestamo lee el archivo entero DENTRO del buffer que le prestan y SIN pedir memoria de
// trabajo: la tabla de posiciones, los bloques y el descompresor viven en `prestamo`, que el que llama reusa
// archivo tras archivo (DC-002.80). Es el camino de la consola para cargar los .dt1 de una zona.
//
// `dst` tiene que alcanzar para `TamanoDeArchivo`. Si la fuente no sabe leer con prestamo (una carpeta del
// disco, por ejemplo) se dice, y el que llama decide (hoy: se vuelve al camino de siempre).
func (l *Loader) LeerEnteroEnConPrestamo(subPath string, dst []byte, prestamo *d2mpq.Scratch) (int, error) {
	subPath = l.rutaReal(subPath)

	for idx := range l.Sources {
		source := l.Sources[idx]

		fuente, ok := source.(asset.SourceDeLecturaConPrestamo)
		if !ok || !source.Exists(subPath) {
			continue
		}

		return fuente.LeerEnteroEnConPrestamo(subPath, dst, prestamo)
	}

	return 0, fmt.Errorf(errFmtFileNotFound, subPath)
}

// TieneLecturaConPrestamo dice si el archivo lo tiene una fuente que sabe leer con la memoria prestada
// (DC-002.80). Es la pregunta que hace `d2asset` para decidir si lee con prestamo o se queda con el camino de
// siempre: con una carpeta del disco no hay MPQ del que prestar nada.
func (l *Loader) TieneLecturaConPrestamo(subPath string) bool {
	subPath = l.rutaReal(subPath)

	for idx := range l.Sources {
		source := l.Sources[idx]

		if _, ok := source.(asset.SourceDeLecturaConPrestamo); !ok {
			continue
		}

		if source.Exists(subPath) {
			return true
		}
	}

	return false
}

// AddSource adds an asset source with the given path. The path will either resolve to a directory
// or a file on the host filesystem. In the case that it is a file, the file extension is used
// to determine the type of asset source. In the case that the path points to a directory, a
// FileSystemSource will be added.
func (l *Loader) AddSource(path string, sourceType types.SourceType) error {
	if l.Sources == nil {
		l.Sources = make([]asset.Source, 0)
	}

	cleanPath := filepath.Clean(path)

	source, err := l.LoaderProviders[sourceType](cleanPath)

	if err != nil {
		return err
	}

	l.Infof("Adding source: '%s'", cleanPath)
	l.Sources = append(l.Sources, source)

	return nil
}

// Exists checks if the given path exists in at least one source
func (l *Loader) Exists(subPath string) bool {
	subPath = filepath.Clean(subPath)

	if l.language != nil {
		charset := l.charset
		language := l.language

		subPath = strings.ReplaceAll(subPath, fontToken, *charset)
		subPath = strings.ReplaceAll(subPath, tableToken, *language)
	}

	// if it isn't in the cache, we check if each source can open the file
	for idx := range l.Sources {
		source := l.Sources[idx]

		// if the source can open the file, then we cache it and return it
		if source.Exists(subPath) {
			return true
		}
	}

	return false
}

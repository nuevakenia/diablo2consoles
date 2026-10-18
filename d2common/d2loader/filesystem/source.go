package filesystem

import (
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2loader/asset"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2loader/asset/types"
)

// static check that Source implements AssetSource
var _ asset.Source = &Source{}

// Source represents an asset source which is a normal directory on the host file system
type Source struct {
	Root string
}

// Type returns the type of this asset source
func (s *Source) Type() types.SourceType {
	return types.AssetSourceFileSystem
}

// Open opens a file with the given sub-path within the Root dir of the file system source
func (s *Source) Open(subPath string) (io.ReadSeeker, error) {
	return os.Open(s.fullPath(subPath))
}

// Exists returns true if the file exists
//
// DC-002.31: `os.IsExist(err)` es FALSE cuando `err == nil`, o sea justo cuando el archivo SI esta: el
// Exists de este Source respondia `false` SIEMPRE. Medido en el host con el arbol suelto de
// patch_D2.mpq: `Source.Exists("data/global/ui/frontend/cinematicselectexp.dc6")` = false con el archivo
// presente, y por lo tanto `Loader.Exists` (el que responde el `asset.FileExists` de
// `d2core/d2asset/composite.go`, `d2core/d2asset/prefetch.go`, `d2gamescreen/game.go` y
// `d2player/container_panel.go`) tambien. Con los MPQ no se notaba porque `mpq.Source.Exists` usa
// `MPQ.Contains`.
func (s *Source) Exists(subPath string) bool {
	_, err := os.Stat(s.fullPath(subPath))

	return err == nil
}

// fullPath: el nombre de un asset del motor viene con las barras INVERTIDAS de Windows
// (`data\global\ui\frontend\...`) y en Linux `filepath.Join`/`filepath.Clean` NO las tratan como
// separador, asi que la ruta quedaba con un `\` adentro y el archivo no se encontraba. Medido (DC-002.31,
// host y consola): el `Loader.Load` del motor contestaba
//
//	file not found: data\global\ui\frontend\cinematicselectexp.dc6
//
// con el arbol suelto completo en el disco, mientras el MISMO asset con barras normales daba 141.944
// bytes. En Windows salia gratis porque `\` ES el separador; en la consola (Linux con gccgo) no. Es el
// equivalente del `cleanName` que ya tiene el Source del MPQ.
func (s *Source) fullPath(subPath string) string {
	return filepath.Clean(filepath.Join(s.Root, filepath.FromSlash(strings.ReplaceAll(subPath, `\`, "/"))))
}

// Path returns the Root dir of this file system source
func (s *Source) Path() string {
	return s.Root
}

// String returns the path
func (s *Source) String() string {
	return s.Path()
}

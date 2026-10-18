#!/usr/bin/env python3
"""DC-002.31: los dos arreglos del proveedor de SISTEMA DE ARCHIVOS del motor (los assets SUELTOS).

QUE HACE: deja `d2common/d2loader/filesystem/source.go` con los dos defectos medidos en la tarjeta
DC-002.31 corregidos. Los dos son de la MISMA clase: el Source mira el archivo en el disco con la
convencion de Windows y el proyecto (host Linux y consola Dreamcast) no es Windows.

  1. EL NOMBRE. El motor pide sus assets con las barras invertidas de Windows
     (`data\\global\\ui\\frontend\\x.dc6`). `filepath.Join`/`filepath.Clean` en Linux NO tratan `\\` como
     separador, asi que la ruta quedaba con un `\\` adentro y el archivo no se encontraba. Medido
     (host, sonda-sueltos): el `Loader.Load` del motor contestaba

         file not found: data\\global\\ui\\frontend\\cinematicselectexp.dc6

     con el arbol suelto completo en el disco, mientras el MISMO asset con barras normales daba 141.944
     bytes (suma 1.831.109.629, idem el MPQ). En Windows salia gratis porque `\\` ES el separador.
     Equivalente al `cleanName` que ya tiene el Source del MPQ.

  2. EL `Exists`. `os.IsExist(err)` es FALSE cuando `err == nil` (o sea justo cuando el archivo SI esta):
     el Source respondia `false` SIEMPRE. Medido: `Source.Exists(...)` = false con el archivo presente, y
     por lo tanto `Loader.Exists` -- el que responde el `asset.FileExists` que usan
     `d2core/d2asset/composite.go`, `d2core/d2asset/prefetch.go`, `d2gamescreen/game.go` y
     `d2player/container_panel.go` -- tambien. Con MPQ no se notaba porque `mpq.Source.Exists` usa
     `MPQ.Contains`.

POR QUE UN SCRIPT Y NO EL ARCHIVO Y NADA MAS: misma razon que los otros arreglos del motor (ver
fix-d2config-json-218.py): el arreglo tiene que poder reaplicarse sobre el arbol pristino y decir con un
numero que el arbol esta como debe estar. `--revertir` deja el archivo PRISTINO (es lo que usa
scripts/console/mutar-sueltos-231.sh para exigir que el corpus se ponga ROJO sin el arreglo).

LO QUE NO HACE: pisar un archivo DISTINTO sin permiso. Si `source.go` no es ni el de este script ni el
pristino, el script lo dice, muestra los sha256 y sale con error; con `--forzar` si lo pisa.

Uso:  python3 scripts/console/fix-d2loader-fs.py [--ver | --revertir | --forzar]
"""
import hashlib
import pathlib
import subprocess
import sys

POR_DEFECTO = pathlib.Path(__file__).resolve().parent.parent.parent / "d2common/d2loader/filesystem/source.go"

# ---------------------------------------------------------------------------------------------------
# el archivo con el arreglo (lo que elige este script)
# ---------------------------------------------------------------------------------------------------
SANO = '''package filesystem

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
// (`data\\global\\ui\\frontend\\...`) y en Linux `filepath.Join`/`filepath.Clean` NO las tratan como
// separador, asi que la ruta quedaba con un `\\` adentro y el archivo no se encontraba. Medido (DC-002.31,
// host y consola): el `Loader.Load` del motor contestaba
//
//	file not found: data\\global\\ui\\frontend\\cinematicselectexp.dc6
//
// con el arbol suelto completo en el disco, mientras el MISMO asset con barras normales daba 141.944
// bytes. En Windows salia gratis porque `\\` ES el separador; en la consola (Linux con gccgo) no. Es el
// equivalente del `cleanName` que ya tiene el Source del MPQ.
func (s *Source) fullPath(subPath string) string {
	return filepath.Clean(filepath.Join(s.Root, filepath.FromSlash(strings.ReplaceAll(subPath, `\\`, "/"))))
}

// Path returns the Root dir of this file system source
func (s *Source) Path() string {
	return s.Root
}

// String returns the path
func (s *Source) String() string {
	return s.Path()
}
'''

# ---------------------------------------------------------------------------------------------------
# el archivo como lo trae el motor (lo que deja --revertir, y el punto de partida de los mutantes)
# ---------------------------------------------------------------------------------------------------
PRISTINO = '''package filesystem

import (
	"io"
	"os"
	"path/filepath"

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
func (s *Source) Exists(subPath string) bool {
	_, err := os.Stat(s.fullPath(subPath))
	return os.IsExist(err)
}

func (s *Source) fullPath(subPath string) string {
	return filepath.Clean(filepath.Join(s.Root, subPath))
}

// Path returns the Root dir of this file system source
func (s *Source) Path() string {
	return s.Root
}

// String returns the path
func (s *Source) String() string {
	return s.Path()
}
'''

SHA_SANO = hashlib.sha256(SANO.encode()).hexdigest()
SHA_PRISTINO = hashlib.sha256(PRISTINO.encode()).hexdigest()

# los textos que miran los mutantes (mutar-sueltos-231.sh): si cambian, el mutante avisa
TEXTO_NOMBRE_SANO = 'return filepath.Clean(filepath.Join(s.Root, filepath.FromSlash(strings.ReplaceAll(subPath, `\\`, "/"))))'
TEXTO_EXISTS_SANO = "\treturn err == nil\n"


def sha(texto):
    return hashlib.sha256(texto.encode()).hexdigest()


def gofmt(ruta):
    salida = subprocess.run(["gofmt", "-l", str(ruta)], capture_output=True, text=True).stdout.strip()

    return salida or "   gofmt: limpio"


def main():
    argv = sys.argv[1:]
    ruta = POR_DEFECTO
    for a in argv:
        if not a.startswith("--"):
            ruta = pathlib.Path(a)

    actual = ruta.read_text() if ruta.exists() else ""
    sh_actual = sha(actual)

    if "--sha" in argv:
        # para los scripts que necesitan el hash COMPLETO (el --ver lo muestra corto para leerlo)
        print("en_arbol=%s sano=%s pristino=%s" % (sh_actual, SHA_SANO, SHA_PRISTINO))

        return 0 if sh_actual == SHA_SANO else 1

    if "--ver" in argv:
        print("source.go %s" % ruta)
        print("  en el arbol:  sha256 %s" % sh_actual[:16])
        print("  sano:         sha256 %s" % SHA_SANO[:16])
        print("  pristino:     sha256 %s" % SHA_PRISTINO[:16])
        print("  el arreglo del NOMBRE esta: %s" % ("si" if TEXTO_NOMBRE_SANO in actual else "NO"))
        print("  el arreglo del EXISTS esta: %s" % ("si" if TEXTO_EXISTS_SANO in actual else "NO"))
        print(gofmt(ruta))

        return 0 if sh_actual == SHA_SANO else 1

    if "--revertir" in argv:
        if sh_actual == SHA_PRISTINO:
            print("   ya estaba pristino (sha256 %s): sin cambios" % SHA_PRISTINO[:16])
        else:
            ruta.write_text(PRISTINO)
            print("   pristino escrito: sha256 %s" % sha(ruta.read_text())[:16])

        return 0

    if actual == SANO:
        print("arreglo DC-002.31 (el Source de sistema de archivos): ya estaba (sha256 %s)" % SHA_SANO[:16])
        print(gofmt(ruta))

        return 0

    if actual not in ("", PRISTINO) and "--forzar" not in argv:
        print("   EL ARCHIVO NO ES NI EL DE ESTE SCRIPT NI EL PRISTINO:", file=sys.stderr)
        print("     en el arbol: sha256 %s" % sh_actual[:16], file=sys.stderr)
        print("     sano:        sha256 %s" % SHA_SANO[:16], file=sys.stderr)
        print("     pristino:    sha256 %s" % SHA_PRISTINO[:16], file=sys.stderr)
        print("   Si el cambio es a proposito, actualiza este script; si no, usa --revertir.", file=sys.stderr)

        return 1

    ruta.parent.mkdir(parents=True, exist_ok=True)
    ruta.write_text(SANO)

    nuevo = sha(ruta.read_text())
    print("arreglo DC-002.31 escrito: sha256 %s" % nuevo[:16])
    assert nuevo == SHA_SANO, "el sha256 del archivo escrito no coincide: revisar el disco"
    print(gofmt(ruta))

    return 0


if __name__ == "__main__":
    sys.exit(main())

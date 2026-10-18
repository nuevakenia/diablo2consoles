// sonda-sueltos: mide si el proveedor de SISTEMA DE ARCHIVOS del motor lee un arbol de assets SUELTOS, y
// con QUE FORMA del nombre. Es una herramienta del HOST (como mpq-extraer): usa los MISMOS paquetes del
// motor que usa la consola (d2common/d2loader y d2common/d2loader/filesystem), asi que lo que aca se mide
// es lo que el motor hace.
//
// DC-002.31: la tarjeta propone NO usar MPQ y dejar los assets sueltos en el disco. El proveedor ya existe
// (`d2common/d2loader/loader.go`: LoaderProviders[types.AssetSourceFileSystem] = filesystem.OnAddSource) y
// su Source abre con os.Open y verifica con os.Stat -- las funciones que este port ya arreglo en DC-002.27.
//
// MIDE CUATRO COSAS, una por linea:
//   (1) Open con BARRAS INVERTIDAS: el nombre tal como lo pide el motor (`data\global\ui\frontend\x.dc6`).
//       filesystem/source.go NO normaliza las barras invertidas a las del sistema -> en Linux eso es un
//       nombre de archivo con "\" adentro, no una ruta.
//   (2) Open con BARRAS NORMALES: el mismo nombre con "/". Si anda, da bytes y suma.
//   (3) la REFERENCIA: el MISMO asset leido del MPQ. Si la suma coincide con la de (2), el arbol suelto
//       entrega el asset identico, byte por byte.
//   (4) EL CAMINO DEL MOTOR: el `Loader` de verdad (el registro de proveedores, las fuentes y la cache que
//       usa `d2core/d2asset.AssetManager.LoadFile` -> `Loader.Load`), NO el Source a mano. Es la unica forma
//       de saber si el empaquetado suelto sirve para el motor y no solo para una sonda: `Load` con el nombre
//       del motor y `Exists` (el que responde el `asset.FileExists` que usan `d2asset/composite.go`,
//       `prefetch.go`, `d2gamescreen/game.go` y `d2player/container_panel.go`).
//
// Uso: go run ./console/dc/sonda-sueltos -mpq <patch_D2.mpq> -raiz <directorio suelto> -asset <nombre del motor>
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2mpq"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2loader"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2loader/asset/types"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2loader/filesystem"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2util"
)

func suma(datos []byte) uint32 {
	s := uint32(0)
	for _, b := range datos {
		s = s*31 + uint32(b)
	}

	return s
}

func leeTodo(lector interface{ Read([]byte) (int, error) }) []byte {
	datos := []byte{}
	buf := make([]byte, 64<<10)

	for {
		n, err := lector.Read(buf)

		datos = append(datos, buf[:n]...)

		if err != nil {
			return datos
		}
	}
}

func main() {
	mpqPath := flag.String("mpq", "", "el MPQ del que salio el arbol suelto (la referencia)")
	raiz := flag.String("raiz", "", "el directorio del arbol suelto")
	asset := flag.String("asset", `data\global\ui\frontend\cinematicselectexp.dc6`, "el nombre del asset COMO LO PIDE EL MOTOR (con barras invertidas)")
	flag.Parse()

	if *mpqPath == "" || *raiz == "" {
		fmt.Fprintln(os.Stderr, "uso: go run ./console/dc/sonda-sueltos -mpq <archivo.mpq> -raiz <directorio> [-asset <nombre>]")
		os.Exit(2)
	}

	fuente, err := filesystem.OnAddSource(*raiz)
	if err != nil {
		fmt.Println("SONDA-SUELTOS OnAddSource FALLO:", err)
		os.Exit(1)
	}

	fmt.Printf("SONDA-SUELTOS raiz=%s\n", fuente.Path())

	// (1) con el nombre del motor, tal cual
	_, errInvertido := fuente.Open(*asset)
	fmt.Printf("SONDA-SUELTOS barras_invertidas_OK=%v\n", errInvertido == nil)

	// (2) con barras normales
	normal := strings.ReplaceAll(*asset, "\\", "/")

	lector, errNormal := fuente.Open(normal)
	if errNormal != nil {
		fmt.Printf("SONDA-SUELTOS barras_normales_OK=false err=%v\n", errNormal)
	} else {
		datos := leeTodo(lector)
		fmt.Printf("SONDA-SUELTOS barras_normales_OK=true bytes=%d suma=%d\n", len(datos), suma(datos))
	}

	// (2c) el Exists del PROPIO Source con un archivo que SI esta (el defecto de DC-002.31: usa os.IsExist
	// con el error de os.Stat, que es false cuando el archivo existe: da false SIEMPRE)
	fmt.Printf("SONDA-SUELTOS source_exists=%v\n", fuente.Exists(normal))

	// (3) la referencia por el MPQ
	mpqObj, err := d2mpq.FromFile(*mpqPath)
	if err != nil {
		fmt.Println("SONDA-SUELTOS no pude abrir el MPQ:", err)
		os.Exit(1)
	}
	defer mpqObj.Close()

	datos, err := mpqObj.ReadFile(*asset)
	if err != nil {
		fmt.Printf("SONDA-SUELTOS mpq_OK=false err=%v\n", err)
	} else {
		fmt.Printf("SONDA-SUELTOS mpq_OK=true bytes=%d suma=%d\n", len(datos), suma(datos))
	}

	// (4) EL CAMINO DEL MOTOR: el Loader de verdad (registro de proveedores + fuentes + cache)
	cargador, errCargador := d2loader.NewLoader(d2util.LogLevelError)
	if errCargador != nil {
		fmt.Println("SONDA-SUELTOS loader_new FALLO:", errCargador)
		os.Exit(1)
	}

	errAdd := cargador.AddSource(*raiz, types.AssetSourceFileSystem)
	fmt.Printf("SONDA-SUELTOS loader_addsource_OK=%v\n", errAdd == nil)

	lectorMotor, errLoad := cargador.Load(*asset)
	if errLoad != nil {
		fmt.Printf("SONDA-SUELTOS loader_load_OK=false err=%v\n", errLoad)
	} else {
		datosMotor := leeTodo(lectorMotor)
		fmt.Printf("SONDA-SUELTOS loader_load_OK=true bytes=%d suma=%d\n", len(datosMotor), suma(datosMotor))
	}

	fmt.Printf("SONDA-SUELTOS loader_exists=%v\n", cargador.Exists(*asset))
}

//go:build dreamcast

// DC-002.31: LOS ASSETS SUELTOS EN LA CONSOLA -- ¿sirve el proveedor de SISTEMA DE ARCHIVOS del motor?
//
// La tarjeta propone el otro camino para los assets: no un MPQ, sino un ARBOL DE ARCHIVOS SUELTOS en el
// disco. El motor YA tiene el proveedor registrado (`d2common/d2loader/loader.go:42`:
// `LoaderProviders[types.AssetSourceFileSystem] = filesystem.OnAddSource`) y su Source abre con `os.Open`
// y verifica con `os.Stat` -- las DOS funciones que este port ya arreglo para leer el disco en DC-002.27.
//
// LO QUE MIDE ESTE POC (la consola serie es la unica salida):
//   1. que el arbol suelto este en el disco: `stat` de /cd/juego y el `ReadDir` (el listado de verdad,
//      que el FS ROM de KOS no sabe dar y el ISO9660 si);
//   2. el Source a mano: `OnAddSource` y despues ABRIR EL MISMO ASSET CON EL NOMBRE DEL MOTOR (barras
//      invertidas, `data\global\ui\frontend\x.dc6`), que es la forma en que el motor pide TODOS sus
//      assets. Medido en el host (sonda-sueltos): sin el arreglo de DC-002.31 el Source NO normaliza las
//      barras y falla -- aca se mide lo mismo en la consola;
//   3. EL CAMINO DEL MOTOR, no la sonda: el `Loader` de verdad (registro de proveedores + fuentes +
//      cache), que es lo que usa `d2core/d2asset.AssetManager.LoadFile` (`Loader.Load`) y el
//      `asset.FileExists` de `d2asset/composite.go`, `d2asset/prefetch.go`, `d2gamescreen/game.go` y
//      `d2player/container_panel.go` (`Loader.Exists`). Un proveedor que solo anda con la sonda no sirve;
//   4. la REFERENCIA: el MISMO asset leido del MPQ del disco (el camino de DC-002.27), para comparar los
//      bytes: si las sumas coinciden, el arbol suelto entrega el MISMO asset.
//
// El ISO9660 va en MAYUSCULAS (nivel 3 de pycdlib solo acepta A-Z, 0-9 y _), y el motor pide los nombres
// en minusculas: por eso el POC pide en minusculas y el corpus anota la caja con la que quedo horneado.
package main

import (
	"os"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2mpq"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2loader"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2loader/asset/types"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2loader/filesystem"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2util"
)

//extern d2c_serial_init
func d2cSerialInit()

//extern d2c_ms
func d2cMs() uint

const (
	// el arbol suelto: la raiz es el MISMO directorio donde DC-002.27 pone los MPQ, asi que un solo
	// camino sirve para los dos empaquetados
	dirSueltos = "/cd/juego"
	// el asset de referencia: el mismo DC6 que abren las sondas del motor en la consola
	asset = `data\global\ui\frontend\cinematicselectexp.dc6`
	// el MPQ del disco (el nombre del ISO9660, en mayusculas)
	mpqDisco = "/cd/JUEGO/PATCH_D2.MPQ"
)

func suma(datos []byte) uint32 {
	s := uint32(0)
	for _, b := range datos {
		s = s*31 + uint32(b)
	}

	return s
}

// leeTodo: lee un lector de a 64 KiB (el MPQ del disco se lee por partes en este port).
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

// mirarLectura imprime el resultado de abrir el asset con un lector y deja la suma a la vista.
func mirarLectura(etiqueta string, lector interface{ Read([]byte) (int, error) }, err error) {
	if err != nil {
		println("SUELTOS-CONSOLA", etiqueta, "FALLO:", err.Error())

		return
	}

	datos := leeTodo(lector)
	println("SUELTOS-CONSOLA", etiqueta, "OK bytes=", len(datos), "suma=", suma(datos))
}

func main() {
	d2cSerialInit()

	println("SUELTOS-CONSOLA arranca (DC-002.31: los assets SUELTOS en el disco)")

	info, err := os.Stat(dirSueltos)
	if err != nil {
		// declarado desde DC-002.27: el `stat` de un DIRECTORIO del disco no lo contesta el driver de
		// KOS (si el tamano de un archivo); el motor decide con `os.Stat` sobre ARCHIVOS
		println("SUELTOS-CONSOLA stat", dirSueltos, "FALLO:", err.Error())
	} else {
		println("SUELTOS-CONSOLA stat", dirSueltos, "esdir=", info.IsDir())
	}

	entradas, err := os.ReadDir(dirSueltos)
	if err != nil {
		println("SUELTOS-CONSOLA ReadDir", dirSueltos, "FALLO:", err.Error())
	} else {
		// el driver ISO9660 de KOS no contesta el listado de un directorio: 0 entradas (declarado, es lo
		// que obliga al `.manifiesto` del romdisk). NO se controla: se anota.
		println("SUELTOS-CONSOLA ReadDir", dirSueltos, "entradas=", len(entradas))
	}

	t0 := d2cMs()

	// ---- 2. el Source a mano, con el nombre del motor
	fuente, err := filesystem.OnAddSource(dirSueltos)
	if err != nil {
		println("SUELTOS-CONSOLA OnAddSource FALLO:", err.Error())
	} else {
		println("SUELTOS-CONSOLA OnAddSource OK:", fuente.Path())

		lector, errFuente := fuente.Open(asset)
		mirarLectura("Source.Open con el nombre del MOTOR (barras invertidas)", lector, errFuente)

		println("SUELTOS-CONSOLA Source.Exists con el nombre del MOTOR=", fuente.Exists(asset))
	}

	println("SUELTOS-CONSOLA ms del Source=", d2cMs()-t0)

	// ---- 3. EL CAMINO DEL MOTOR: el Loader de verdad (registro de proveedores + fuentes + cache)
	cargador, errCargador := d2loader.NewLoader(d2util.LogLevelError)
	if errCargador != nil {
		println("SUELTOS-CONSOLA Loader.New FALLO:", errCargador.Error())
	} else {
		errAdd := cargador.AddSource(dirSueltos, types.AssetSourceFileSystem)
		println("SUELTOS-CONSOLA Loader.AddSource=", errAdd == nil)

		lectorMotor, errMotor := cargador.Load(asset)
		mirarLectura("Loader.Load con el nombre del MOTOR", lectorMotor, errMotor)

		println("SUELTOS-CONSOLA Loader.Exists con el nombre del MOTOR=", cargador.Exists(asset))
	}

	// ---- 4. la referencia por el MPQ del disco (el camino de DC-002.27)
	mpqObj, err := d2mpq.FromFile(mpqDisco)
	if err != nil {
		println("SUELTOS-CONSOLA el MPQ del disco FALLO:", err.Error())
	} else {
		datos, errLectura := mpqObj.ReadFile(asset)
		if errLectura != nil {
			println("SUELTOS-CONSOLA MPQ ReadFile FALLO:", errLectura.Error())
		} else {
			println("SUELTOS-CONSOLA MPQ ReadFile OK bytes=", len(datos), "suma=", suma(datos))
		}

		mpqObj.Close()
	}

	println("SUELTOS-CONSOLA fin")
}

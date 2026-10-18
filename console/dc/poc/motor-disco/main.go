//go:build dreamcast

// DC-002.27: EL MOTOR LEYENDO LOS ASSETS DEL JUEGO DESDE EL DISCO (la unidad GD-ROM, /cd, por ISO9660).
//
// De donde sale esta tarjeta: hasta DC-002.26 el binario del motor viajaba al disco pero los ARCHIVOS DEL
// JUEGO no, asi que el arranque se frenaba con su cuadro de error --
//
//     file not found: "/rd/juego/patch_d2.mpq"
//
// porque el `/rd` (el romdisk, dentro del binario) traia sellos VACIOS: el instrumento de DC-002.19, que
// servia para correr la frontera hacia adelante pero no para leer el juego. Aca el config del romdisk
// apunta a `/cd/juego` y los MPQ de verdad viajan en la ISO.
//
// LO QUE MIDE ESTE POC (la consola serie es la unica salida que hay):
//   1. LA SONDA DEL DISCO, antes de tocar el motor: `stat` de las rutas de /cd (arriba de la carpeta del
//      juego y adentro), el ReadDir de /cd/JUEGO (un listado de verdad, que el FS ROM de KOS NO sabe dar:
//      por eso el romdisk necesita un .manifiesto y el disco no), y la SUMA de bytes de los dos extremos
//      de cada MPQ leidos con el mismo `os` que usa el motor (Seek + Read por trozos). La suma se compara
//      contra la del archivo del PC: es lo que dice si lo que se lee del disco es el MISMO archivo o un
//      pedazo corrido.
//   2. LA SELECCION DE ARCHIVOS DEL MOTOR sobre /cd: `Complete` y `Validate` (los dos caminos de
//      `d2setup`) y el `MpqPath` que el arranque LEYO de su config.
//   3. EL ARRANQUE: `d2app.Create(...).Run()`, igual que el binario de la rama. Lo que se mide es si el
//      cuadro de error de los MPQ DESAPARECE y hasta donde llega el motor.
//
// La consola serie y las marcas de avance viven en csrc/cshim.c (en Go no se puede: dbgio_dev_select).
package main

import (
	"encoding/json"
	"os"
	"runtime"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/OpenDiablo2/OpenDiablo2/d2app"
	"github.com/OpenDiablo2/OpenDiablo2/d2app/d2setup"
	mpqloader "github.com/OpenDiablo2/OpenDiablo2/d2common/d2loader/mpq"
	d2mpq "github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2mpq"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2config"
)

//extern d2c_serial_init
func d2cSerialInit()

//extern d2c_disco_marca
func d2cDiscoMarca(que uint) uint

//extern d2c_ms
func d2cMs() uint

const (
	configConsola = "/rd/OpenDiablo2/config.json"
	dirJuego      = "/cd/JUEGO"
	bloque        = 8192
	// topeSuma: cuanto se lee de cada MPQ por corrida (los dos extremos). No es un limite del motor (el
	// motor lee SOLO los bloques que necesita, con ReadAt): es el costo de esta sonda, que quiere el
	// principio (la cabecera MPQ) y el FINAL (la tabla de bloques del MPQ, que va al final del archivo)
	// porque un corte o un corrimiento del horneado se ve ahi. Medido en Flycast: el GD-ROM emulado
	// entrega ~122 KB/s (4 MiB en 34.245 ms), asi que 256 KiB por extremo mantiene la sonda barata.
	topeSuma = 256 << 10
)

// sondaDisco: lo que la consola ve del disco, con el MISMO `os` que el motor (que abajo es el VFS de KOS:
// si el disco esta registrado, `/cd` es un dispositivo mas y `os.Open` no cambia una linea).
func sondaDisco() {
	rutas := []string{
		"/cd",
		dirJuego,
		dirJuego + "/PATCH_D2.MPQ",  // el nombre del disco, como lo hornea mkiso.py (ISO9660: A-Z, 0-9, _)
		"/cd/juego/patch_d2.mpq",    // el MISMO archivo, con el nombre que PIDE el motor (minusculas)
		"/cd/JUEGO/D2DATA.MPQ",
		"/cd/JUEGO/D2CHAR.MPQ",
		"/cd/juego/no-existe.mpq",   // el control negativo: tiene que dar error
	}

	println("DISCO-CONSOLA sonda del disco: empieza (", len(rutas), "rutas )")

	for _, ruta := range rutas {
		info, err := os.Stat(ruta)
		if err != nil {
			println("DISCO-CONSOLA stat", ruta, "FALLO:", err.Error())
			continue
		}

		println("DISCO-CONSOLA stat", ruta, "bytes=", info.Size(), "esdir=", info.IsDir())
	}

	// El ReadDir del disco: el FS ROM de KOS no sabe listar un directorio (por eso el romdisk lleva un
	// .manifiesto horneado). El ISO9660 si: esto lo mide.
	entradas, err := os.ReadDir(dirJuego)
	if err != nil {
		println("DISCO-CONSOLA ReadDir", dirJuego, "FALLO:", err.Error())
	} else {
		println("DISCO-CONSOLA ReadDir", dirJuego, "->", len(entradas), "entradas")

		for _, e := range entradas {
			info, errInfo := e.Info()
			if errInfo != nil {
				println("DISCO-CONSOLA   ", e.Name(), "sin Info:", errInfo.Error())
				continue
			}

			println("DISCO-CONSOLA   ", e.Name(), "bytes=", info.Size(), "dir=", e.IsDir())
		}
	}

	// Las sumas: el MPQ se lee POR TROZOS y por los dos extremos (el motor tambien lo lee por partes).
	for _, ruta := range []string{dirJuego + "/PATCH_D2.MPQ", dirJuego + "/D2DATA.MPQ", dirJuego + "/D2CHAR.MPQ"} {
		sumaTrozo(ruta, "principio", 0)
		sumaTrozo(ruta, "final", -1)
	}
}

// sumaTrozo: lee `topeSuma` bytes del archivo y devuelve su suma y su largo. Con `desde` = -1 lee el FINAL
// (ultimo trozo del archivo), que es donde vive la tabla de bloques del MPQ.
func sumaTrozo(ruta, cual string, desde int64) {
	f, err := os.Open(ruta)
	if err != nil {
		println("DISCO-CONSOLA suma", cual, ruta, "FALLO al abrir:", err.Error())
		return
	}

	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		println("DISCO-CONSOLA suma", cual, ruta, "FALLO al stat:", err.Error())
		return
	}

	cuantos := int64(topeSuma)
	if info.Size() < cuantos {
		cuantos = info.Size()
	}

	arranque := int64(0)
	if desde < 0 {
		arranque = info.Size() - cuantos
	}

	if arranque > 0 {
		if _, err := f.Seek(arranque, 0); err != nil {
			println("DISCO-CONSOLA suma", cual, ruta, "FALLO al Seek", arranque, ":", err.Error())
			return
		}
	}

	t0 := int64(d2cMs())
	buf := make([]byte, bloque)
	leidos := int64(0)
	suma := uint32(0)

	for leidos < cuantos {
		pedir := int64(bloque)
		if cuantos-leidos < pedir {
			pedir = cuantos - leidos
		}

		n, err := f.Read(buf[:pedir])
		for i := 0; i < n; i++ {
			suma = suma*31 + uint32(buf[i])
		}

		leidos += int64(n)

		if err != nil {
			println("DISCO-CONSOLA suma", cual, ruta, "corto en", leidos, "de", cuantos, ":", err.Error())
			break
		}

		if n == 0 {
			break
		}
	}

	println("DISCO-CONSOLA suma", cual, ruta, "desde=", arranque, "leidos=", leidos, "de=", info.Size(),
		"suma=", suma, "ms=", int64(d2cMs())-t0)
}

// seleccionDelMotor: los dos caminos de `d2setup` sobre /cd, y el `MpqPath` que el ARRANQUE leyo de su
// config del romdisk. Es el numero con el que DC-002.26 se frenaba.
func seleccionDelMotor() {
	conf, err := os.Open(configConsola)
	if err != nil {
		println("MOTOR-DISCO config: no pude abrir", configConsola+":", err.Error())
		return
	}

	defer conf.Close()

	cfg := &d2config.Configuration{}
	cfg.SetPath(configConsola)

	if err := json.NewDecoder(conf).Decode(cfg); err != nil {
		println("MOTOR-DISCO config: el Decode fallo:", err.Error())
		return
	}

	println("MOTOR-DISCO config del arranque: MpqPath=", cfg.MpqPath, "orden=", len(cfg.MpqLoadOrder))

	v := d2setup.Validate(cfg.MpqPath)

	println("MOTOR-DISCO seleccion: Complete=", d2setup.Complete(cfg.MpqPath, cfg.MpqLoadOrder),
		"Validate.OK()=", v.OK(), "archivos que ve=", len(v.LoadOrder), "faltan=", len(v.MissingCore))
}

// sondaDelCargador: el camino EXACTO del motor para leer el MPQ del disco, paso por paso.
//
// Por que: el motor sigue con su cartel en la pantalla y el texto dice "file not found:
// "/cd/juego/patch_d2.mpq"" -- pero ese cartel lo arma d2app/initialization.go:85 con fmt.Errorf(..) y
// TIRA EL ERROR DE ABAJO (no lo imprime), asi que por el serial no se puede saber que fallo. Aca se llama a
// los mismos constructores del motor y se imprime el error de cada uno:
//   d2app/initialization.go:82 -> asset.AddSource(srcPath, AssetSourceMPQ)
//   d2loader/loader.go:118     -> LoaderProviders[AssetSourceMPQ](cleanPath)
//   d2loader/mpq/source.go:17  -> d2mpq.FromFile(sourcePath)
//   d2mpq/mpq.go:66            -> d2mpq.New (os.Open + readHeader) + readHashTable + readBlockTable
// El primero que devuelva error es el tramo que hay que arreglar.
func sondaDelCargador() {
	ruta := dirJuego + "/patch_d2.mpq"

	println("CARGADOR-CONSOLA runtime.GOOS=", runtime.GOOS, "(en linux d2mpq.New usa openIgnoreCase, que ante un os.Open fallido LISTA el directorio)")

	f, err := os.Open(ruta)
	if err != nil {
		println("CARGADOR-CONSOLA os.Open", ruta, "FALLO:", err.Error())
		return
	}

	info, errInfo := f.Stat()

	if errInfo != nil {
		println("CARGADOR-CONSOLA f.Stat FALLO:", errInfo.Error())
	} else {
		println("CARGADOR-CONSOLA os.Open OK; f.Stat bytes=", info.Size())
	}

	buf := make([]byte, 4096)
	n, errRead := f.Read(buf)
	println("CARGADOR-CONSOLA f.Read(4096) ->", n, "err:", errRead == nil)

	// Los primeros bytes EN HEXA: el control de que lo que se lee del disco es el archivo y no otra cosa.
	// (Medido 2026-10-17: la firma que salia por el serial NO era la del MPQ -- el tamano si, el contenido
	// no -- asi que esta sonda tiene que imprimir bytes, no texto.)
	hex := "0123456789abcdef"
	linea := ""
	for i := 0; i < 16 && i < n; i++ {
		linea += string(hex[buf[i]>>4]) + string(hex[buf[i]&0x0f]) + " "
	}

	println("CARGADOR-CONSOLA primeros 16 bytes:", linea)

	suma := uint32(0)
	for i := 0; i < n; i++ {
		suma = suma*31 + uint32(buf[i])
	}

	println("CARGADOR-CONSOLA suma de los primeros", n, "bytes:", suma)

	_, errSeek := f.Seek(0, 0)
	println("CARGADOR-CONSOLA f.Seek(0,0) err:", errSeek == nil)

	f.Close()

	// 1) el constructor del motor que solo lee la cabecera
	mpqObj, err := d2mpq.New(ruta)
	if err != nil {
		println("CARGADOR-CONSOLA d2mpq.New FALLO:", err.Error())
	} else {
		println("CARGADOR-CONSOLA d2mpq.New OK")
		mpqObj.Close()
	}

	// 2) el que ademas lee las dos tablas (el del motor de verdad)
	mpqObj, err = d2mpq.FromFile(ruta)
	if err != nil {
		println("CARGADOR-CONSOLA d2mpq.FromFile FALLO:", err.Error())
	} else {
		println("CARGADOR-CONSOLA d2mpq.FromFile OK")
		mpqObj.Close()
	}

	// 3) el Source del cargador de assets (lo que llama AddSource)
	src, err := mpqloader.NewSource(ruta)
	if err != nil {
		println("CARGADOR-CONSOLA mpq.NewSource FALLO:", err.Error())
	} else {
		println("CARGADOR-CONSOLA mpq.NewSource OK")
	}

	if src != nil {
		_, err = src.Open("data\\global\\ui\\frontend\\gameselectscreenEXP.dc6")
		if err != nil {
			println("CARGADOR-CONSOLA src.Open(dc6 del menu) FALLO:", err.Error())
		} else {
			println("CARGADOR-CONSOLA src.Open(dc6 del menu) OK")
		}
	}
}

func main() {
	d2cSerialInit()

	println("MOTOR-DISCO arranca (DC-002.27: el motor leyendo los assets del juego desde el DISCO)")
	println("MOTOR-DISCO runtime sh4")

	d2cDiscoMarca(1)

	sondaDisco()

	seleccionDelMotor()

	sondaDelCargador()

	if err := ebiten.IniciarPantalla(640, 480); err != nil {
		println("MOTOR-DISCO el video no arranco:", err.Error())
	} else {
		println("MOTOR-DISCO video listo, VRAM de la pantalla", ebiten.VramDeLaPantalla(), "bytes")
	}

	println("MOTOR-DISCO monton de C antes del motor:", ebiten.MemLibreDelMonton(), "bytes libres")

	d2cDiscoMarca(2)

	println("MOTOR-DISCO llamando a d2app.Create().Run()")

	instance := d2app.Create("DC-002.27", "dreamcast")

	d2cDiscoMarca(3)

	if err := instance.Run(); err != nil {
		println("MOTOR-DISCO Run() termino con error:", err.Error())
	} else {
		println("MOTOR-DISCO Run() volvio sin error")
	}

	d2cDiscoMarca(4)

	println("MOTOR-DISCO fin")
}

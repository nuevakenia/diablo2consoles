//go:build dreamcast

// DC-002.13: EL ARRANQUE DEL MOTOR DE VERDAD en la Dreamcast.
//
// Este POC no dibuja un asset suelto ni un cuadro armado por nosotros: llama al MISMO arranque que usa el
// juego en el host, el de main.go de la raiz del repo:
//
//	instance := d2app.Create(gitBranch, gitCommit)
//	if err := instance.Run(); err != nil { ... }
//
// Lo unico que agrega la consola es lo que el host hace por el sistema operativo: la consola serie (para
// poder leer despues) y el video, por el backend de ebiten de la consola (IniciarPantalla), que es el mismo
// camino que usa el motor para su pantalla.
//
// LO QUE SE MIDE ES HASTA DONDE LLEGA, no que funcione: el arranque del motor pide su configuracion y sus
// MPQ (d2data.mpq son 267.330.019 bytes) y la consola no tiene disco. Donde se caiga (y lo que alcance a
// imprimir por la consola serie, porque el `log` de la stdlib minima sale por la serial) es el dato.
package main

import (
	"encoding/json"
	"os"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/OpenDiablo2/OpenDiablo2/d2app"
	"github.com/OpenDiablo2/OpenDiablo2/d2app/d2setup"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2config"
)

//extern d2c_serial_init
func d2cSerialInit()

//extern d2c_arranque_marca
func d2cArranqueMarca(que uint) uint // marca de avance: escribe en la serial lo que ya paso

// pruebaDeEscritura: DC-002.17 -- la prueba DIRECTA del sistema de archivos escribible de la consola, con
// el MISMO paquete os que usa el motor: crear un directorio, crear un archivo, escribir 64 bytes, cerrar,
// leerlo de vuelta y comparar el contenido, y borrarlo.
//
// POR QUE ESTA PRUEBA Y NO SOLO EL ARRANQUE DEL MOTOR: el motor pide su config y `d2config.Save()` vuelve
// a fallar mas adelante, en `json.MarshalIndent` (la consola no tiene reflexion), ANTES de escribir un
// byte. O sea que el arranque del motor prueba el mkdir y el create, pero NO la escritura. Esta prueba
// cierra ese hueco y es la que el corpus pone en rojo cuando el overlay se rompe.
//
// Corre ANTES de d2app.Create() a proposito: en ese momento el monton esta libre y la prueba puede
// concatenar cadenas sin que la falta de memoria la vuelva invisible (la leccion de DC-002.17: un println
// que concatena no imprime NADA cuando el monton no tiene donde armar la cadena).
const textoPrueba = "DC-002.17: el overlay de la RAM escribe y lee de vuelta 0123456789\n"

func pruebaDeEscritura() {
	const dir = "/rd/prueba-217"
	const ruta = "/rd/prueba-217/prueba.txt"

	println("OS-RAMFS prueba: el overlay ocupa", os.RamfsTam(), "bytes de .bss")

	if err := os.MkdirAll(dir, 0750); err != nil {
		println("OS-RAMFS prueba FALLO: MkdirAll:", err.Error())
		return
	}

	f, err := os.Create(ruta)
	if err != nil {
		println("OS-RAMFS prueba FALLO: Create:", err.Error())
		return
	}

	n, err := f.Write([]byte(textoPrueba))
	if err != nil {
		println("OS-RAMFS prueba FALLO: Write:", err.Error())
		return
	}
	if err := f.Close(); err != nil {
		println("OS-RAMFS prueba FALLO: Close:", err.Error())
		return
	}

	datos, err := os.ReadFile(ruta)
	if err != nil {
		println("OS-RAMFS prueba FALLO: ReadFile:", err.Error())
		return
	}

	igual := string(datos) == textoPrueba
	println("OS-RAMFS prueba: escribi", n, "bytes, lei", len(datos), "igual=", igual)

	if err := os.Remove(ruta); err != nil {
		println("OS-RAMFS prueba FALLO: Remove:", err.Error())
		return
	}

	archivos, bytes, dirs := os.RamfsResumen()
	println("OS-RAMFS prueba: despues de borrar -> archivos=", archivos, "bytes=", bytes, "dirs=", dirs)
}

// seleccionDeArchivos: DC-002.19 -- LOS NUMEROS DE LA FRONTERA QUE ESTA TARJETA DESTRABA.
//
// El arranque del motor se frenaba en `d2setup.EnsureGameFiles` ("no Diablo II game files were selected")
// porque en la consola no hay quien elija la carpeta de los MPQ: los dos caminos del motor que no
// preguntan salen de `os.Getenv` (un stub en la consola, devuelve "") y sin env queda `OsaUI`
// (osascript), que devuelve `cancelled by user`. El camino que NO pregunta es el que el arranque ya tiene:
// `Complete(cfg.MpqPath, cfg.MpqLoadOrder)` = true, o sea la CONFIGURACION completa apuntando a los datos
// del juego. Por eso el romdisk del build hornea `/rd/OpenDiablo2/config.json` (lo escribe el motor) y el
// directorio de datos `/rd/juego`.
//
// Lo que imprime, y por que cada uno:
//   - el MpqPath y cuantos archivos pide el config que el arranque LEYO del romdisk
//   - `Complete(dir, orden)` (os.Stat por nombre) y `Validate(dir).OK()` (os.ReadDir del directorio) con
//     cuantos archivos ve cada uno: los DOS caminos de la seleccion del motor, sobre el mismo directorio
//   - el ReadDir de un directorio CREADO ACA (el overlay de la RAM de DC-002.17): es el defecto que
//     arregla esta tarjeta (antes veia CERO archivos en un directorio del overlay)
func seleccionDeArchivos() {
	const rutaConf = "/rd/OpenDiablo2/config.json"
	const dirPrueba = "/rd/prueba-seleccion"

	conf, err := os.Open(rutaConf)
	if err != nil {
		println("MOTOR-ARRANQUE seleccion: no pude abrir el config del arranque:", err.Error())
	} else {
		cfg := &d2config.Configuration{}
		cfg.SetPath(rutaConf)

		if errDec := json.NewDecoder(conf).Decode(cfg); errDec != nil {
			println("MOTOR-ARRANQUE seleccion: el Decode del config fallo:", errDec.Error())
		} else {
			println("MOTOR-ARRANQUE config del arranque: MpqPath=", cfg.MpqPath,
				"archivos en el orden=", len(cfg.MpqLoadOrder))

			v := d2setup.Validate(cfg.MpqPath)
			println("MOTOR-ARRANQUE seleccion: Complete=", d2setup.Complete(cfg.MpqPath, cfg.MpqLoadOrder),
				"Validate.OK()=", v.OK(), "archivos que ve=", len(v.LoadOrder), "faltan=", len(v.MissingCore))
		}

		conf.Close()
	}

	// el mismo camino, pero sobre un directorio del OVERLAY (creado por el programa y no horneado)
	if err := os.MkdirAll(dirPrueba, 0750); err != nil {
		println("MOTOR-ARRANQUE seleccion: MkdirAll fallo:", err.Error())

		return
	}

	uno, err := os.Create(dirPrueba + "/uno.txt")
	if err != nil {
		println("MOTOR-ARRANQUE seleccion: Create fallo:", err.Error())

		return
	}

	_, _ = uno.Write([]byte("doce bytes!\n"))
	_ = uno.Close()

	entradas, err := os.ReadDir(dirPrueba)

	// DC-002.19: el estado del overlay JUSTO DESPUES de listar (y con el listado ya hecho): si el
	// listado del C no ve lo que el overlay tiene, los dos numeros lo dicen sin ambiguedad.
	nArch, nBytes, nDirs := os.RamfsResumen()
	println("MOTOR-ARRANQUE overlay del listado: archivos=", nArch, "bytes=", nBytes, "dirs=", nDirs)

	if err != nil {
		println("MOTOR-ARRANQUE seleccion overlay: ReadDir dio error:", err.Error())

		return
	}

	nombres := ""
	for _, e := range entradas {
		nombres += e.Name() + " "
	}

	println("MOTOR-ARRANQUE seleccion overlay: ReadDir de", dirPrueba, "->", len(entradas),
		"entradas:", nombres)
}

func main() {
	d2cSerialInit()

	println("MOTOR-ARRANQUE arranca (DC-002.13: el entry point del motor en la consola)")
	println("MOTOR-ARRANQUE runtime", "sh4")

	// DC-002.18: el instrumento de la serializacion. Desde aca, CADA llamada de serializacion del motor
	// (Marshal, MarshalIndent, Unmarshal, Encode, Decode) imprime por la serial por donde paso y sobre que
	// tipo, y si ese tipo sabe serializarse solo. Es lo que mide esta tarjeta: cuantas serializaciones hace
	// el arranque y sobre que tipos. Va ANTES de Create()/Run() a proposito: el arranque pide y guarda su
	// configuracion ahi adentro.
	instrumentoJSON()

	// el video: sin esto el motor no tiene pantalla. Se arranca ANTES de Run() para que un fallo del video
	// se vea como un numero (y no como un cuelgue dentro del motor).
	if err := ebiten.IniciarPantalla(640, 480); err != nil {
		println("MOTOR-ARRANQUE el video no arranco:", err.Error())
	} else {
		println("MOTOR-ARRANQUE video listo, VRAM de la pantalla", ebiten.VramDeLaPantalla(), "bytes")
	}

	println("MOTOR-ARRANQUE llamando a d2app.Create().Run()")

	// DC-002.17: la prueba del sistema de archivos escribible, con el mismo os del motor.
	pruebaDeEscritura()

	// DC-002.19: los numeros de la seleccion de los archivos del juego (el config del romdisk y el
	// ReadDir del overlay), ANTES de arrancar el motor: es lo que esta tarjeta destraba.
	seleccionDeArchivos()

	d2cArranqueMarca(1)

	instance := d2app.Create("DC-002.13", "dreamcast")
	d2cArranqueMarca(2)

	if err := instance.Run(); err != nil {
		println("MOTOR-ARRANQUE Run() termino con error:", err.Error())
	} else {
		println("MOTOR-ARRANQUE Run() volvio sin error")
	}

	// cuanto dejo escrito el motor en el overlay (si dejo algo)
	archivos, bytes, dirs := os.RamfsResumen()
	println("MOTOR-ARRANQUE overlay del motor: archivos=", archivos, "bytes=", bytes, "dirs=", dirs)

	d2cArranqueMarca(3)
	println("MOTOR-ARRANQUE fin")
}

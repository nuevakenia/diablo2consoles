//go:build dreamcast

// DC-002.19: LA SELECCION DE LOS ARCHIVOS DEL JUEGO en la consola.
//
// El arranque ya guarda su `config.json` (DC-002.18) y lo que queda delante es la SELECCION de los archivos
// del juego: `d2app/firstrun.go:32` llama a `d2setup.EnsureGameFiles`, que con `MpqPath` vacio intenta
// PREGUNTARLE al usuario por una carpeta (`ui.ChooseFolder`) y termina en `ErrNoGameFiles`.
//
// QUE MIDE ESTE POC, y por que asi:
//
//  A) `d2setup.OsaUI{}.ChooseFolder(...)` DIRECTO: el camino de los dialogos nativos que corre en la
//     consola es `OsaUI` (ui_native_other.go), que llama a `osascript` (dialogos de macOS). El motor se
//     traga ese error y devuelve `ErrNoGameFiles`, asi que el TEXTO del error no se ve en el arranque: aca
//     se pide a mano y se imprime. Esto es el "por que" medido, no supuesto.
//
//  B) El camino de LECTURA del arranque, que DC-002.18 NO habia podido medir: se escribe el config con
//     `MpqPath` y `MpqLoadOrder` puestos (los mismos que dejaria una seleccion exitosa) y se llama al
//     arranque: `d2app.LoadConfig` tiene que LEER ese archivo con `json.NewDecoder(...).Decode(config)`.
//
//  C) Hasta donde llega `Run()` con los MPQ presentes pero VACIOS (0 bytes): `Complete()` y `Validate()`
//     solo miran que los nombres esten (`os.Stat`/`os.ReadDir`), no el contenido, asi que con los siete
//     archivos vacios el motor PASA la seleccion y el error que queda es el del LECTOR de MPQ. Ese es el
//     numero de esta tarjeta: la frontera nueva, con nombre y apellido.
//
// Los MPQ vacios NO son una trampa para que "ande": son el instrumento para correr la frontera hacia
// adelante y ver el error siguiente. Los MPQ de verdad (d2data.mpq son 267.330.019 bytes) no entran en el
// overlay en RAM de la consola (18.020 B de .bss, medido en DC-002.17).
package main

import (
	"os"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/OpenDiablo2/OpenDiablo2/d2app"
	"github.com/OpenDiablo2/OpenDiablo2/d2app/d2setup"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2config"
)

//extern d2c_serial_init
func d2cSerialInit()

//extern d2c_arranque_marca
func d2cArranqueMarca(que uint) uint

const (
	dirJuego  = "/rd/juego"
	rutaConf  = "/rd/OpenDiablo2/config.json"
	dirConfig = "/rd/OpenDiablo2"
)

// los SIETE archivos que `d2setup.CoreMPQs` exige (discover.go) para dar la carpeta por buena
var archivos = []string{
	"patch_d2.mpq", "d2data.mpq", "d2char.mpq", "d2music.mpq", "d2sfx.mpq", "d2speech.mpq", "d2video.mpq",
}

func main() {
	d2cSerialInit()
	instrumentoJSON()

	println("SETUP-ARCHIVOS arranca (DC-002.19: la seleccion de los archivos del juego)")
	println("SETUP-ARCHIVOS runtime", "sh4")

	// (A) el camino de los dialogos nativos, pedido a mano
	pruebaDelDialogo()

	// los siete MPQ vacios donde el motor los busca
	crearArchivos()

	// (B) el config que dejaria una seleccion exitosa, escrito por el motor
	escribirConfig()

	if err := ebiten.IniciarPantalla(640, 480); err != nil {
		println("SETUP-ARCHIVOS el video no arranco:", err.Error())
	} else {
		println("SETUP-ARCHIVOS video listo, VRAM de la pantalla", ebiten.VramDeLaPantalla(), "bytes")
	}

	println("SETUP-ARCHIVOS llamando a d2app.Create().Run()")
	d2cArranqueMarca(1)

	instance := d2app.Create("DC-002.19", "dreamcast")
	d2cArranqueMarca(2)

	// (C) hasta donde llega con los MPQ presentes pero vacios
	if err := instance.Run(); err != nil {
		println("SETUP-ARCHIVOS Run() termino con error:", err.Error())
	} else {
		println("SETUP-ARCHIVOS Run() volvio sin error")
	}

	archivosN, bytes, dirs := os.RamfsResumen()
	println("SETUP-ARCHIVOS overlay del motor: archivos=", archivosN, "bytes=", bytes, "dirs=", dirs)

	d2cArranqueMarca(3)
	println("SETUP-ARCHIVOS fin")
}

// pruebaDelDialogo: (A) que hace el camino de los dialogos nativos en la consola.
func pruebaDelDialogo() {
	println("SETUP-ARCHIVOS A: os.Getenv(OD2_SETUP_AUTOPICK) =", os.Getenv("OD2_SETUP_AUTOPICK"))

	ui := d2setup.OsaUI{}

	donde, err := ui.ChooseFolder("Elegi la carpeta de Diablo II", "/rd")
	if err != nil {
		println("SETUP-ARCHIVOS A: OsaUI.ChooseFolder devolvio error:", err.Error())
	} else {
		println("SETUP-ARCHIVOS A: OsaUI.ChooseFolder devolvio la carpeta", donde)
	}
}

// crearArchivos: los siete MPQ, vacios, en el overlay en RAM.
func crearArchivos() {
	if err := os.MkdirAll(dirJuego, 0750); err != nil {
		println("SETUP-ARCHIVOS FALLO: MkdirAll:", err.Error())

		return
	}

	creados := 0

	for _, nombre := range archivos {
		f, err := os.Create(dirJuego + "/" + nombre)
		if err != nil {
			println("SETUP-ARCHIVOS FALLO: Create", nombre, err.Error())

			continue
		}

		if err := f.Close(); err != nil {
			println("SETUP-ARCHIVOS FALLO: Close", nombre, err.Error())
		}

		creados++
	}

	println("SETUP-ARCHIVOS archivos del juego creados:", creados, "de", len(archivos), "en", dirJuego)

	// que ve el motor en esa carpeta (el mismo `Validate` que usa la seleccion)
	v := d2setup.Validate(dirJuego)
	println("SETUP-ARCHIVOS A: Validate(dir).OK() =", v.OK(), "archivos que ve =", len(v.LoadOrder))
	println("SETUP-ARCHIVOS A: Complete(dir, orden) =", d2setup.Complete(dirJuego, archivos))
}

// escribirConfig: (B) el config que dejaria una seleccion exitosa, escrito con el motor (d2config.Save).
func escribirConfig() {
	if err := os.MkdirAll(dirConfig, 0750); err != nil {
		println("SETUP-ARCHIVOS FALLO: MkdirAll del config:", err.Error())

		return
	}

	cfg := d2config.DefaultConfig()
	cfg.MpqPath = dirJuego
	cfg.MpqLoadOrder = archivos
	cfg.SetPath(rutaConf)

	if err := cfg.Save(); err != nil {
		println("SETUP-ARCHIVOS FALLO: Save del config:", err.Error())

		return
	}

	datos, err := os.ReadFile(rutaConf)
	if err != nil {
		println("SETUP-ARCHIVOS FALLO: ReadFile del config:", err.Error())

		return
	}

	println("SETUP-ARCHIVOS B: config escrito en", rutaConf, "bytes=", len(datos))
}

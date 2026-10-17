//go:build dreamcast

// DC-002.23: EL ERROR DEL MOTOR, MEDIDO SIN TOCAR EL MOTOR.
//
// De donde sale esta tarjeta: en DC-002.22 el cuadro 0 del motor se caia con `Unhandled exception: PC
// 0000000b` pintando el texto de su cuadro de error, y quedo abierto DE QUE LADO estaba el salto: el itab
// del metodo `PrintAt` del renderer o el estado del `GlyphPrinter`.
//
// LA MEDICION QUE CIERRA ESA PREGUNTA (medida, no deducida): NO es ninguno de los dos. La instruccion que
// se cae es el `jsr @r0` de la EVALUACION DEL ARGUMENTO `a.errorMessage.Error()` en
// `d2app.App.updateInitError`, con `r1 = *(a+156)` y `r0 = *(r1+4)`. Lo que esta mal es el CAMPO: en el
// arranque `a.errorMessage` NO es una tabla de metodos valida. Medido con una sonda temporal dentro de
// d2app (que despues se quito): `itab=0x8c4f3c60`, `data=0xff000000`, y 0x8c4f3c60 es, segun `sh-elf-nm`,
// `_image_1color.RGBA..d` (el DESCRIPTOR DE TIPO de `image/color.RGBA`). El offset del campo esta medido
// con el compilador (`unsafe.Offsetof`): errorMessage=156, renderer=124, config=148, logger=152,
// options=164, size=168.
//
// Y LO MAS IMPORTANTE, medido con el MISMO codigo en dos layouts del monton: el valor del campo CAMBIA de
// una corrida a otra. Con una sonda mas (que cambia el layout) el campo queda **nil en todos los puntos**,
// el motor PASA `loadEngine` y muere mas adelante con `panic: nil pointer dereference`. O sea: el defecto
// del cuadro 0 no es el texto ni la interfaz del renderer, es MEMORIA del App que se corrompe segun donde
// caiga todo en el monton (el mismo hallazgo que DC-002.22 midio con 200 bytes de diferencia).
//
// Este POC mide lo que SI se puede medir sin tocar el motor: las palabras del campo `errorMessage`
// (+156/+160, el mismo offset que usa `updateInitError`) leidas DESDE AFUERA, en los puntos del camino del
// arranque que el POC puede reproducir: despues de `Create`, despues de `LoadConfig` (lo primero que hace
// `Run` con el config del romdisk), despues de arrancar el video y antes de `Run()`. `NIL` es el estado
// sano; un itab que no sea una tabla de metodos (o un valor chico, como el 0x9 que se midio) es el defecto.
package main

import (
	"strconv"
	"unsafe"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/OpenDiablo2/OpenDiablo2/d2app"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2loader"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2util"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2asset"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2records"
)

//extern d2c_serial_init
func d2cSerialInit()

//extern d2c_frame
func d2cFrame()

// offsetError: donde lee `updateInitError` el campo `errorMessage` del `App`. El 156 NO es una deduccion
// del desensamblado: se midio dentro del paquete con `unsafe.Offsetof` (ver el comentario de arriba).
const offsetError = 156

func hex(v uintptr) string {
	return "0x" + strconv.FormatUint(uint64(v), 16)
}

func palabra(base, i uintptr) uintptr {
	return *(*uintptr)(unsafe.Pointer(base + i*4))
}

// mirar: las tres palabras de una interfaz `error`: itab, data y metodo 0.
func mirar(caso string, e error) {
	base := uintptr(unsafe.Pointer(&e))
	itab := palabra(base, 0)
	data := palabra(base, 1)

	m0 := uintptr(0)
	if itab != 0 {
		m0 = palabra(itab, 1)
	}

	esnil := "no"
	if e == nil {
		esnil = "si"
	}

	println("MOTOR-ERROR", caso, "esnil=", esnil, "itab=", hex(itab), "data=", hex(data), "m0=", hex(m0))
}

// mirarCampo: el mismo dato, pero leido del App por offset (como lo lee `updateInitError`).
func mirarCampo(caso string, app uintptr) {
	itab := palabra(app+offsetError, 0)
	data := palabra(app+offsetError, 1)

	m0 := uintptr(0)
	if itab != 0 {
		m0 = palabra(itab, 1)
	}

	estado := "BASURA"
	if itab == 0 {
		estado = "NIL"
	}

	println("MOTOR-ERROR", caso, "app=", hex(app), "estado=", estado, "itab=", hex(itab), "data=", hex(data),
		"m0=", hex(m0))
}

func main() {
	d2cSerialInit()
	println("MOTOR-ERROR arranca (DC-002.23: las palabras del error del motor)")

	ancla := 1
	println("MOTOR-ERROR ancla de .data:", hex(uintptr(unsafe.Pointer(&ancla))))

	// ---- 1. los constructores del arranque, uno por uno (todos devuelven (algo, error))
	loader, eLoader := d2loader.NewLoader(d2util.LogLevelError)
	mirar("1 d2loader.NewLoader", eLoader)
	println("MOTOR-ERROR 1 loader =", hex(uintptr(unsafe.Pointer(loader))))

	records, eRecords := d2records.NewRecordManager(d2util.LogLevelError)
	mirar("2 d2records.NewRecordManager", eRecords)
	println("MOTOR-ERROR 2 records =", hex(uintptr(unsafe.Pointer(records))))

	asset, eAsset := d2asset.NewAssetManager(d2util.LogLevelError)
	mirar("3 d2asset.NewAssetManager", eAsset)
	println("MOTOR-ERROR 3 asset =", hex(uintptr(unsafe.Pointer(asset))))

	// ---- 2. el App de verdad, y su campo errorMessage leido donde lo lee updateInitError
	instance := d2app.Create("DC-002.23", "dreamcast")
	ptrApp := uintptr(unsafe.Pointer(instance))
	println("MOTOR-ERROR 4 el App vive en", hex(ptrApp))
	mirarCampo("4 App.errorMessage tras Create", ptrApp)

	// ---- 3. los tramos del camino del arranque que el POC puede reproducir solo (sin tocar el motor):
	// LoadConfig es lo primero que hace Run() con el config del romdisk.
	if _, err := instance.LoadConfig(); err != nil {
		println("MOTOR-ERROR LoadConfig fallo:", err.Error())
	}

	mirarCampo("5 App.errorMessage tras LoadConfig", ptrApp)

	if err := ebiten.IniciarPantalla(640, 480); err != nil {
		println("MOTOR-ERROR el video no arranco:", err.Error())
	} else {
		println("MOTOR-ERROR video listo, VRAM de la pantalla", ebiten.VramDeLaPantalla(), "bytes")
	}

	mirarCampo("6 App.errorMessage tras el video", ptrApp)

	// ---- 4. el motor entero: Run() se cae en el cuadro 0 (o mas adelante, segun el layout). Las marcas
	// de arriba son el dato; lo que imprima el motor antes de caerse dice hasta donde llego.
	println("MOTOR-ERROR llamando a Run()")
	println("MOTOR-ERROR (si el motor se cae, la ultima linea de arriba es el punto)")
	if err := instance.Run(); err != nil {
		println("MOTOR-ERROR Run() volvio con error")
	}

	println("MOTOR-ERROR fin")

	for i := 0; i < 60; i++ {
		d2cFrame()
	}
}

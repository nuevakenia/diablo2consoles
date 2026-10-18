//go:build dreamcast

// DC-002.32: EL PESO DEL BINARIO EN RAM -- la sonda de la tabla de d2records.
//
// De donde sale: la tarjeta mide las palancas del peso del binario (que hoy se lleva 13.998.980 de los
// 16.777.216 bytes de la consola = 83,4%) y la mas grande es UN SOLO simbolo de `.data`: `_go..C3111`,
// 1.672.892 bytes, la tabla `objectLookups` de d2core/d2records.
//
// LO QUE MIDE ESTE POC, en una sola corrida (el arranque de la consola tarda ~30 s y cada corrida cuesta):
//
//  1. EL TAMANO DEL REGISTRO, medido por la CONSOLA: `unsafe.Sizeof(d2records.ObjectLookupRecord{})`. El
//     mismo numero lo mide el host con un programa de Go; los dos tienen que coincidir. Sirve para dividir
//     el peso del simbolo: 1.672.892 / (tamano del registro) = cuantos elementos tiene la tabla.
//  2. LO QUE CUESTA TENERLOS EN EL MONTON DE C: se pide UNA asignacion del mismo tamano que el simbolo
//     (1.672.892 bytes), se tocan las dos puntas y se sueltan, imprimiendo el estado del monton de C antes,
//     durante y despues. Ese es el numero que decide la palanca: si la tabla se saca del binario y se lee
//     del disco, los bytes no desaparecen, se mudan del `.data` al monton de C (o al semiespacio del GC).
//  3. EL ARRANQUE DEL MOTOR, con el MISMO instrumento de DC-002.13/27/28 (las marcas y el monton de C antes
//     del motor) y el MISMO orden que el POC motor-arranque: asi el punto donde muere el arranque se puede
//     comparar con el de la tarjeta y con el de la variante sin la tabla. Los `println` que importan llevan
//     el prefijo historico `MOTOR-ARRANQUE` a proposito (el corpus y las tarjetas viejas grepean eso).
//
// El instrumento del monton de C es el de DC-002.28 tal cual (csrc/cshim.c): `fordblks` NO es la RAM libre
// (el arena crece por sbrk), asi que el costo se lee en el SALTO de `sbrk`/`arena`, no en `libre`.
package main

import (
	"encoding/json"
	"os"
	"unsafe"

	"github.com/OpenDiablo2/OpenDiablo2/d2app"
	"github.com/OpenDiablo2/OpenDiablo2/d2app/d2setup"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2config"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2records"
	"github.com/hajimehoshi/ebiten/v2"
)

//extern d2c_serial_init
func d2cSerialInit()

//extern d2c_arranque_marca
func d2cArranqueMarca(que uint) uint // marca de avance: escribe en la serial lo que ya paso

//extern d2c_ms
func d2cMs() uint

//extern d2c_sbrk_actual
func d2cSbrkActual() uint

//extern d2c_monton_techo
func d2cMontonTecho() uint

//extern d2c_monton_arena
func d2cMontonArena() int

//extern d2c_monton_libre
func d2cMontonLibre() int

//extern d2c_monton_usado
func d2cMontonUsado() int

//extern d2c_bloque_max
func d2cBloqueMax() uint

//extern d2c_reserva
func d2cReserva(bytes uint) uint

//extern d2c_reserva_libre
func d2cReservaLibre(ptr uint)

// pruebaDeEscritura: la prueba del sistema de archivos escribible de la consola (DC-002.17), la misma del
// POC motor-arranque. Va ANTES de d2app.Create() porque ahi el monton esta libre y una cadena concatenada
// todavia se puede armar (la leccion de DC-002.17: un println que concatena no imprime NADA si el monton
// no tiene donde armar la cadena).
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

// seleccionDeArchivos: los numeros de la seleccion del motor (DC-002.19), los mismos del POC motor-arranque.
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

// hex32: un uint32 en hexa (el mismo ayudante del POC motor-reservas: no depende del %x de la consola).
func hex32(v uint) string {
	const digitos = "0123456789abcdef"

	s := ""

	for i := 7; i >= 0; i-- {
		s += string(digitos[(v>>(uint(i)*4))&0xf])
	}

	return s
}

// itoa: un entero decimal.
func itoa(v int) string {
	if v == 0 {
		return "0"
	}

	negativo := v < 0
	if negativo {
		v = -v
	}

	s := ""

	for v > 0 {
		s = string(byte('0'+v%10)) + s
		v /= 10
	}

	if negativo {
		s = "-" + s
	}

	return s
}

// estadoDelMonton: los cuatro numeros del monton de C, juntos (DC-002.28).
func estadoDelMonton() string {
	return "sbrk=0x" + hex32(d2cSbrkActual()) + " techo=0x" + hex32(d2cMontonTecho()) +
		" arena=" + itoa(d2cMontonArena()) + " libre=" + itoa(d2cMontonLibre()) +
		" usado=" + itoa(d2cMontonUsado()) + " bloqueMax=" + itoa(int(d2cBloqueMax()))
}

// pesoDeLaTabla: cuanto pesa UN registro (medido por la consola) y cuanto cuesta tener la tabla entera en
// el monton de C (medido pidiendola). Se libera en el acto: el bloque queda libre para el motor, asi que el
// arranque que viene despues mide lo de siempre (el arena ya crecio, que es lo que se declara).
func pesoDeLaTabla() {
	const pesoDelSimbolo = 1672892 // `nm -S` sobre el ELF enlazado: `_go..C3111` = 1.672.892 bytes

	tam := unsafe.Sizeof(d2records.ObjectLookupRecord{})

	println("PESO-232 ObjectLookupRecord mide", tam, "bytes (medido adentro de la consola)")
	println("PESO-232 la tabla del ELF pesa", pesoDelSimbolo, "bytes ->",
		uint(pesoDelSimbolo)/uint(tam), "elementos")

	println("PESO-232 monton antes de la reserva:  ", estadoDelMonton())

	antes := d2cSbrkActual()
	antesArena := d2cMontonArena()

	ptr := d2cReserva(pesoDelSimbolo)
	if ptr == 0 {
		println("PESO-232 la reserva de la tabla NO ENTRO (no hay bloque de", pesoDelSimbolo, "bytes)")

		return
	}

	// el costo se mide CON la tabla pedida: es el salto de sbrk/arena contra el estado de antes
	sbrkConTabla := d2cSbrkActual()
	arenaConTabla := d2cMontonArena()

	println("PESO-232 reserva de", pesoDelSimbolo, "bytes: ptr=0x"+hex32(ptr))
	println("PESO-232 monton con la tabla adentro: ", estadoDelMonton())
	println("PESO-232 la tabla cuesta sbrk=", int(sbrkConTabla)-int(antes),
		"arena=", arenaConTabla-antesArena, "bytes del monton de C")

	d2cReservaLibre(ptr)

	println("PESO-232 monton despues de soltarla:  ", estadoDelMonton())
}

func main() {
	d2cSerialInit()

	println("MOTOR-ARRANQUE arranca (DC-002.32: la sonda del peso del binario)")
	println("MOTOR-ARRANQUE runtime", "sh4")

	// el instrumento de la serializacion, identico al del POC motor-arranque (DC-002.18): se engancha
	// ANTES de Create()/Run(), que es cuando el arranque pide y guarda su configuracion.
	instrumentoJSON()

	if err := ebiten.IniciarPantalla(640, 480); err != nil {
		println("MOTOR-ARRANQUE el video no arranco:", err.Error())
	} else {
		println("MOTOR-ARRANQUE video listo, VRAM de la pantalla", ebiten.VramDeLaPantalla(), "bytes")
	}

	println("MOTOR-ARRANQUE monton de C antes del motor:", ebiten.MemLibreDelMonton(), "bytes libres")
	println("MOTOR-ARRANQUE monton de C antes del motor:", estadoDelMonton())

	pesoDeLaTabla()

	println("MOTOR-ARRANQUE llamando a d2app.Create().Run()")

	pruebaDeEscritura()
	seleccionDeArchivos()

	d2cArranqueMarca(1)

	instance := d2app.Create("DC-002.32", "dreamcast")
	d2cArranqueMarca(2)

	if err := instance.Run(); err != nil {
		println("MOTOR-ARRANQUE Run() termino con error:", err.Error())
	} else {
		println("MOTOR-ARRANQUE Run() volvio sin error")
	}

	archivos, bytes, dirs := os.RamfsResumen()
	println("MOTOR-ARRANQUE overlay del motor: archivos=", archivos, "bytes=", bytes, "dirs=", dirs)

	d2cArranqueMarca(3)
	println("MOTOR-ARRANQUE framebuffer de la pantalla en la VRAM:", ebiten.VramDelFramebuffer(), "bytes")
	println("MOTOR-ARRANQUE fin")
}

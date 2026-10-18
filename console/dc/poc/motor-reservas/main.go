//go:build dreamcast

// DC-002.28: LA SONDA DE LAS RESERVAS DEL MPQ -- QUE PIDE CADA TABLA Y CON CUANTA RAM LIBRE.
//
// De donde sale: DC-002.27 (commit 7ef2ff16) dejo el disco andando (los MPQ viajan en la ISO, el driver
// monta la sesion de alta densidad y los cargadores del motor los abren) y el arranque del motor murio
// igual, al empezar a leer el CONTENIDO:
//
//     [File Loader][INFO] Loaded /rd/OpenDiablo2 -> config.json
//     [App][INFO] loaded configuration file from /rd/OpenDiablo2/config.json
//     fatal error: gc_external_alloc: OOM
//     Memory: arena=2627328 used=2585720 free=41608
//
// Lo que NO se sabia era CUAL de las reservas revienta y con cuanta RAM libre. La tarjeta traia un
// candidato por parecido de numeros (1.916.928 = "119.808 entradas * 16 bytes" de la tabla de hash de
// d2data.mpq). Este POC existe para MEZCLAR eso: mide, no supone.
//
// QUE MIDE
//   1. LA CABECERA de cada MPQ del orden del motor (`MpqLoadOrder` del config), leida a mano de los
//      primeros 32 bytes del archivo (offset 24 = entradas de la tabla de hash, offset 28 = entradas de
//      la tabla de bloques). El pedido de RAM de cada tabla es `entradas * 16` bytes, que es exactamente
//      lo que reserva `readHashTable` / `readBlockTable` (d2common/d2fileformats/d2mpq). No reserva nada:
//      solo lee 32 bytes, asi que la medida no ensucia la del paso siguiente.
//   2. EL CARGADOR DEL MOTOR, archivo por archivo y en el ORDEN del motor (`mpqloader.NewSource` ->
//      `d2mpq.FromFile` -> `readHashTable` + `readBlockTable`), imprimiendo la RAM libre ANTES de cada
//      uno. Si el archivo que revienta mata la corrida, la linea de "libreAntes" ya salio: ese es el
//      numero que la tarjeta pide. La RAM libre es `mallinfo().fordblks`, la MISMA fuente que el runtime
//      de la consola imprime como `free=` en su mensaje de OOM (medido en DC-002.21/22).
//
// POR QUE UN POC APARTE (y no la sonda del POC motor-disco): la sonda del motor-disco abre el MPQ chico
// ANTES de arrancar el motor, y eso deja los buffers de sus tablas en el monton de C (medido en
// DC-002.27: el monton queda en 696 bytes libres justo antes de entrar al motor). Para decir "con cuanta
// RAM libre revienta la reserva" hay que medir SIN ese gasto antes, o el numero que sale es el de la
// sonda, no el del motor. Aca no hay motor: solo las reservas.
//
// La consola serie se inicializa en csrc/cshim.c (en Go no se puede: `dbgio_dev_select`).
package main

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"os"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2mpq"
	mpqloader "github.com/OpenDiablo2/OpenDiablo2/d2common/d2loader/mpq"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2config"
	"github.com/hajimehoshi/ebiten/v2"
)

//extern d2c_serial_init
func d2cSerialInit()

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

// estadoDelMonton: los cuatro numeros del monton de C, juntos. `libre` (fordblks) es lo que la tarjeta
// imprimia hasta DC-002.27 y NO alcanza: el numero que dice si entra una reserva es `bloqueMax` (el bloque
// CONTIGUO mas grande), y `sbrk`/`techo` dicen si al monton le queda RAM por crecer o ya toco el techo.
//
// `ebitenLibre` es el mismo `fordblks` pero leido por el camino del motor (`ebiten.MemLibreDelMonton`, el
// que usa el POC motor-disco): va al lado para poder comparar los dos numeros en la misma linea.
func estadoDelMonton() string {
	return "sbrk=0x" + hex32(d2cSbrkActual()) + " techo=0x" + hex32(d2cMontonTecho()) +
		" arena=" + itoa(d2cMontonArena()) + " libre=" + itoa(d2cMontonLibre()) +
		" usado=" + itoa(d2cMontonUsado()) + " bloqueMax=" + itoa(int(d2cBloqueMax())) +
		" ebitenLibre=" + itoa(ebiten.MemLibreDelMonton())
}

// hex32: un uint32 en hexa, para no depender del %x de la consola (que existe, pero asi la linea se arma
// igual alla y en cualquier otro lado).
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

const (
	configConsola = "/rd/OpenDiablo2/config.json"
	dirJuego      = "/cd/JUEGO"

	// bytesPorEntrada: cada entrada de las dos tablas del MPQ son 16 bytes (4 palabras de 32 bits). La
	// tabla de hash se guarda como `[]uint32` de `entradas*4` palabras y ADEMAS se lee de un tiron a un
	// buffer de `entradas*16` bytes: de ahi que el pedido en el peor caso sea el DOBLE de lo que ocupa la
	// tabla. Los dos numeros se imprimen (lo que ocupa y lo que pide).
	bytesPorEntrada = 16
)

// errCabeceraMala: la cabecera no es de un MPQ (o viene corta). `os.ErrInvalid` no existe en la stdlib
// minima de la consola (medido: "reference to undefined identifier 'os.ErrInvalid'"), asi que el error se
// arma con `errors`, que si esta portado.
var errCabeceraMala = errors.New("la cabecera no es de un MPQ")

// cabeceraDelMPQ: los dos numeros de las tablas, leidos de los primeros 32 bytes del archivo.
//
// La cabecera del MPQ (32 bytes, sin relleno): firma, tamano de cabecera, tamano del archivo, version,
// shift del sector, offset y entradas de la tabla de hash, offset y entradas de la tabla de bloques. Los
// dos ultimos son los que importan: 24 = entradas de hash, 28 = entradas de bloques.
func cabeceraDelMPQ(ruta string) (hashEntradas, blockEntradas uint32, err error) {
	f, err := os.Open(ruta)
	if err != nil {
		return 0, 0, err
	}

	defer f.Close()

	buf := make([]byte, 32)

	n, err := f.Read(buf)
	if err != nil || n < 32 {
		return 0, 0, errCabeceraMala
	}

	if string(buf[:4]) != "MPQ\x1a" {
		return 0, 0, errCabeceraMala
	}

	hashEntradas = binary.LittleEndian.Uint32(buf[24:])
	blockEntradas = binary.LittleEndian.Uint32(buf[28:])

	return hashEntradas, blockEntradas, nil
}

// configDelArranque: el `MpqPath` y el `MpqLoadOrder` que el motor leyo de su config del romdisk. Es el
// ORDEN que importa: `d2app/initialization.go:78` recorre `MpqLoadOrder` y hace `AddSource` de cada uno.
func configDelArranque() *d2config.Configuration {
	conf, err := os.Open(configConsola)
	if err != nil {
		println("RESERVA-CONSOLA config: no pude abrir", configConsola+":", err.Error())
		return nil
	}

	defer conf.Close()

	cfg := &d2config.Configuration{}
	cfg.SetPath(configConsola)

	if err := json.NewDecoder(conf).Decode(cfg); err != nil {
		println("RESERVA-CONSOLA config: el Decode fallo:", err.Error())
		return nil
	}

	return cfg
}

// sondaCabeceras: cuanto pide cada tabla de cada MPQ del orden del motor. Sin reservar nada.
func sondaCabeceras(orden []string) {
	println("RESERVA-CONSOLA 1/2 cabeceras del orden del motor (", len(orden), "archivos )")

	for i, nombre := range orden {
		ruta := dirJuego + "/" + nombre

		he, be, err := cabeceraDelMPQ(ruta)
		if err != nil {
			println("RESERVA-CONSOLA  ", i+1, nombre, "cabecera FALLO:", err.Error())
			continue
		}

		println("RESERVA-CONSOLA  ", i+1, nombre,
			"hashEntradas=", he, "hashBytes=", he*bytesPorEntrada,
			"blockEntradas=", be, "blockBytes=", be*bytesPorEntrada,
			"pedidoHash=", he*bytesPorEntrada*2, "pedidoBlock=", be*bytesPorEntrada*2)
	}
}

// sondaCargadores: el camino EXACTO del motor (mpqloader.NewSource = d2mpq.FromFile = las dos tablas),
// en el orden del motor, con la RAM libre ANTES de cada archivo. Si un archivo revienta, la corrida
// muere adentro: por eso la linea de "libreAntes" sale primero y por eso el orden es el del motor.
func sondaCargadores(orden []string) {
	println("RESERVA-CONSOLA 2/2 cargadores del motor, en el orden del motor, con la RAM libre de cada paso")

	for i, nombre := range orden {
		ruta := dirJuego + "/" + nombre
		libre0 := d2cMontonLibre()

		println("CARGADOR-CONSOLA", i+1, "/", len(orden), "libreAntes=", libre0, ruta)
		println("CARGADOR-CONSOLA  monton antes:  ", estadoDelMonton())

		// 1) el constructor que solo lee la cabecera
		mpqObj, err := d2mpq.New(ruta)
		if err != nil {
			println("CARGADOR-CONSOLA  d2mpq.New FALLO:", err.Error(), "libre=", d2cMontonLibre())

			continue
		}

		mpqObj.Close()

		println("CARGADOR-CONSOLA  d2mpq.New OK (solo la cabecera)")
		println("CARGADOR-CONSOLA  monton tras la cabecera:", estadoDelMonton())

		// 2) el camino del motor: las dos tablas
		t0 := d2cMs()

		src, err := mpqloader.NewSource(ruta)
		if err != nil {
			println("CARGADOR-CONSOLA  mpq.NewSource FALLO:", err.Error(), "libre=", d2cMontonLibre(),
				"ms=", d2cMs()-t0)

			continue
		}

		_ = src

		println("CARGADOR-CONSOLA  mpq.NewSource OK (las dos tablas) libre=", d2cMontonLibre(),
			"ms=", d2cMs()-t0)
		println("CARGADOR-CONSOLA  monton tras las tablas: ", estadoDelMonton())
	}
}

func main() {
	d2cSerialInit()

	println("MOTOR-RESERVAS arranca (DC-002.28: que pide cada tabla del MPQ y con cuanta RAM libre)")
	println("MOTOR-RESERVAS monton de C al arrancar:", d2cMontonLibre(), "bytes libres")
	println("MOTOR-RESERVAS monton al arrancar:  ", estadoDelMonton())

	cfg := configDelArranque()
	if cfg == nil {
		println("MOTOR-RESERVAS sin config: no hay orden del motor que recorrer")

		return
	}

	println("MOTOR-RESERVAS MpqPath=", cfg.MpqPath, "MpqLoadOrder=", len(cfg.MpqLoadOrder))

	sondaCabeceras(cfg.MpqLoadOrder)
	sondaCargadores(cfg.MpqLoadOrder)

	println("MOTOR-RESERVAS fin: los cargadores del motor entraron TODOS; libre final=",
		d2cMontonLibre())
}

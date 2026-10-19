//go:build dreamcast

// DC-002.40: POR QUE LA CONSOLA NO PUEDE LEER UN ARCHIVO COMPRIMIDO DE UN MPQ.
//
// El defecto de partida, medido (POC `tablas-mpq`, corpus `checar-tablas-233.sh`): en la consola,
//
//	TABLAS-MPQ    patch_d2.mpq (listfile) FALLO: zlib: invalid header
//	TABLAS-MPQ    d2data.mpq   (listfile) FALLO: zlib: invalid header
//	TABLAS-MPQ    d2char.mpq   (listfile) FALLO: zlib: invalid header
//
// con el MISMO codigo del motor que en el host da 162 / 11.082 / 10.009 nombres.
//
// LO QUE MIDIO EL HOST ANTES (instrumento `scripts/console/sonda-mpq-compr.py`, que lee el MPQ a mano
// y no usa el codigo del motor a proposito):
//
//	(listfile) de los TRES MPQ: Flags = 0x80030200 = Compress | Encrypted | FixKey | Exists
//	  (NO es SingleUnit: va por SECTORES), cada sector con el byte 0x02 adelante = zlib,
//	  y el BLOQUE ENCRIPTADO con la clave FixKey (`(hash(nombre,3) + FilePosition) ^ UncompressedFileSize`).
//	El .dc6 que la consola SI lee del mismo archivo (DC-002.31, 141.944 bytes):
//	  Flags = 0x80000100 = Implode | Exists  ->  POR SECTORES, PKWARE, **SIN ENCRIPTAR**.
//
// O sea: lo unico que la consola NUNCA midio leyendo bien es un bloque ENCRIPTADO y con zlib. Este POC
// separa las dos cosas con los MISMOS bytes buenos, medidos uno por uno:
//
//	M1  los bytes del sector (los saca el host y viajan en el disco) llegan enteros a la consola;
//	M2  el puente de UNA SOLA VEZ (`d2c_zlib_inflate`, el que usa `compress/zlib` de la consola) sobre
//	    esos bytes, con el destino que calcula la regla `4 x comprimido + 64 KB` - que aca sobra
//	    (4 x 661 + 65.536 = 68.180 para 4.096 de salida): si falla, NO es tamano;
//	M3  el puente en STREAMING (el de `compress/gzip`, medido bueno en DC-002.20) sobre los MISMOS bytes:
//	    si M3 anda y M2 no, el culpable es el puente de una sola vez y no los datos;
//	M4  el camino del paquete (`zlib.NewReader` de `d2std/compress/zlib`), que es el que llama el motor
//	    desde `mpq_stream.go:deflate` cuando el byte de compresion es 2;
//	M5  la LECTURA CRUDA del sector encriptado desde el disco (`os.File.ReadAt`), comparada con el CRC32
//	    que calculo el host sobre los mismos bytes: separa "el dato llega mal del disco" de "la
//	    descompresion falla";
//	M6  EL CAMINO DEL MOTOR: `d2mpq.FromFile` + `ReadFile("(listfile)")` sobre los 3 MPQ del disco, con
//	    el numero de NOMBRES (el criterio 2 de la tarjeta: 162 / 11.082 / 10.009);
//	M7  el CONTROL: el mismo camino del motor leyendo el `.dc6` (implode, sin encriptar) que ya se midio
//	    bien en DC-002.31: si este numero cambia, el que se rompio es el camino comun y no el comprimido.
//
// Los bytes de M1/M2/M3/M4 los saca el host con la sonda y el disco los lleva a /cd/juego. La consola
// serie se inicializa en csrc/pantalla.c (en Go no se puede: `dbgio_dev_select`).
package main

import (
	"bytes"
	"compress/zlib"
	"hash/crc32"
	"io"
	"os"
	"unsafe"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2mpq"
	"github.com/hajimehoshi/ebiten/v2"
)

//extern d2c_serial_init
func d2cSerialInit()

//extern d2c_ms
func d2cMs() uint

//extern d2c_dormir_ms
func d2cDormirMs(ms int32)

//extern d2c_monton_arena
func d2cMontonArena() int32

//extern d2c_monton_usado
func d2cMontonUsado() int32

//extern d2c_monton_libre
func d2cMontonLibre() int32

//extern d2c_sbrk_actual
func d2cSbrkActual() uint

//extern d2c_direccion_bss
func d2cDireccionBss() uint

//extern d2c_buffer_in
func d2cBufferIn() uint

//extern d2c_buffer_out
func d2cBufferOut() uint

//extern d2c_copiar_a_entrada
func d2cCopiarAEntrada(src uint, n int32)

//extern d2c_copiar_de_salida
func d2cCopiarDeSalida(dst uint, n int32)

// el puente de UNA SOLA VEZ (el que usa d2std/compress/zlib)
//
//extern d2c_zlib_inflate
func d2cZlibInflate(src []byte, srcLen int32, dst []byte, dstCap int32) int32

//extern d2c_zlib_ultimo_rc
func d2cZlibUltimoRc() int32

//extern d2c_zlib_inflate_bound
func d2cZlibInflateBound(srcLen int32) int32

// el puente en STREAMING (el que usa d2std/compress/gzip): los buffers van como uintptr, que es la
// convencion que el proyecto ya pago cara en DC-002.19/2.20.
//
//extern d2c_zlib_stream_init
func d2cZlibStreamInit() int32

//extern d2c_zlib_stream_step
func d2cZlibStreamStep(slot int32, src uintptr, srcLen int32, dst uintptr, dstCap int32) int32

//extern d2c_zlib_stream_consumido
func d2cZlibStreamConsumido(slot int32) int32

//extern d2c_zlib_stream_fin
func d2cZlibStreamFin(slot int32) int32

//extern d2c_zlib_stream_end
func d2cZlibStreamEnd(slot int32) int32

// LO MEDIDO EN EL HOST (sonda-mpq-compr.py). No se deduce: son los numeros contra los que se compara.
const (
	dirJuego = "/cd/juego"

	// Los bytes del sector los deja el host (sonda-mpq-compr.py) en el directorio del disco. La extension
	// .txt es el unico camino que ya tiene el horneador del disco (`mkdisco.sh` hornea *.mpq y *.txt, ver
	// DC-002.33): el archivo son los BYTES del sector tal como quedan DESPUES de descifrar, no texto.
	sectorDelHost = dirJuego + "/sector0_compr40.txt"

	mpqListfile = "patch_d2.mpq"
	posListfile = 4755852 // FilePosition del bloque (listfile) de patch_d2.mpq
	offListfile = 12      // offsets[0]: el sector 0 arranca 12 bytes adentro del bloque
	lenSector0  = 662     // bytes GUARDADOS del sector 0 (ya descifrados): 661 de zlib + el byte 0x02
	crcSector0  = 3898719478
	inflado0    = 4096 // lo que tiene que salir del sector 0
	crcInflado0 = 2274402335

	nombreDC6 = `data\global\ui\FrontEnd\CinematicSelectExp.dc6`

	chicoDeLectura = 512 // el pedazo con el que se le pide al puente en streaming (DC-002.20)
)

func main() {
	d2cSerialInit()

	println("COMPR-40 arranca (DC-002.40: por que la consola no lee un archivo COMPRIMIDO del MPQ)")
	println("COMPR-40 libres en el monton:", ebiten.MemLibreDelMonton())

	// ============================================================ M0: el disco (KOS lo monta en 2do plano)
	prepararDisco()

	// ============================================================ M1: los bytes buenos llegan enteros
	buenos, intentos, err := leerDelDisco(sectorDelHost)
	if err != nil {
		println("COMPR-40 M1 FALLO al leer el sector del host:", err.Error())

		return
	}

	crudoSector := buenos

	println("COMPR-40 M1 sector del host bytes=", len(crudoSector), "crc32=", crc32.ChecksumIEEE(crudoSector),
		"(host:", lenSector0, crcSector0, ") primer byte=", crudoSector[0], "intentos=", intentos)

	// EL MAPA: donde termina el monton de C (sbrk) y donde vive el monton de Go. Si el tope del de C pasa
	// la base del de Go, los dos montones se pisan y cualquier lectura de la zlib es basura posible.
	println("COMPR-40 MAPA bss=", d2cDireccionBss(), "sbrk=", d2cSbrkActual(), "arena=", d2cMontonArena(),
		"heap_go=", uintptr(unsafe.Pointer(&crudoSector[0])), "libre_c=", d2cMontonLibre())

	// ============================================================ M2: el puente de UNA SOLA VEZ
	cota := d2cZlibInflateBound(int32(len(crudoSector) - 1))
	destino := make([]byte, cota)
	t0 := d2cMs()
	devuelto := d2cZlibInflate(crudoSector[1:], int32(len(crudoSector)-1), destino, int32(cota))
	msViejo := d2cMs() - t0

	if devuelto < 0 {
		println("COMPR-40 M2 puente de una sola vez FALLO (devolvio", devuelto, ") cota=", cota,
			"ms=", msViejo, "rc_zlib=", d2cZlibUltimoRc())
	} else {
		println("COMPR-40 M2 puente de una sola vez bytes=", devuelto,
			"crc32=", crc32.ChecksumIEEE(destino[:devuelto]), "cota=", cota, "ms=", msViejo,
			"(host:", inflado0, crcInflado0, ")")
	}

	// ============================================================ M3: el puente en STREAMING
	salida, lecturas, errPuente := inflarEnStreaming(crudoSector[1:])
	if errPuente != nil {
		println("COMPR-40 M3 puente en streaming FALLO:", errPuente.Error(), "rc_zlib=", d2cZlibUltimoRc())
	} else {
		println("COMPR-40 M3 puente en streaming bytes=", len(salida), "crc32=", crc32.ChecksumIEEE(salida),
			"lecturas_de_", chicoDeLectura, "=", lecturas, "(host:", inflado0, crcInflado0, ")")
	}

	// ============================================================ M4: el paquete compress/zlib
	lector, errZlib := zlib.NewReader(bytes.NewReader(crudoSector[1:]))
	if errZlib != nil {
		println("COMPR-40 M4 zlib.NewReader fallo:", errZlib.Error())
	} else {
		datos, errLectura := io.ReadAll(lector)
		if errLectura != nil {
			println("COMPR-40 M4 compress/zlib FALLO al leer:", errLectura.Error())
		} else {
			println("COMPR-40 M4 compress/zlib bytes=", len(datos), "crc32=", crc32.ChecksumIEEE(datos),
				"(host:", inflado0, crcInflado0, ")")
		}

		_ = lector.Close()
	}

	// ============================================================ M4b: el paquete, de a 512 bytes (MUCHOS pasos)
	// El camino del motor pide bloques de 4096 (el `io.ReadAll` de la consola usa un buffer de 4096 y su
	// `bytes.Buffer.ReadFrom` tambien), asi que un bloque comprimido de 4096 que da 4096 de salida se
	// descomprime EN UN SOLO PASO: con eso, el avance de `z.pos` entre pasos no se prueba (medido: un mutante
	// que ponia `z.pos` al final quedaba VERDE). Leer de a 512 fuerza los 8 pasos -- es el unico control que
	// mira el avance entre pasos, y por ahi salen los mutantes de `z.pos`.
	lector2, errZlib2 := zlib.NewReader(bytes.NewReader(crudoSector[1:]))
	if errZlib2 != nil {
		println("COMPR-40 M4b zlib.NewReader fallo:", errZlib2.Error())
	} else {
		pedazo := make([]byte, 512)
		acumulado := make([]byte, 0, 8192)
		pasos := 0
		var errL2 error

		for {
			n, e := lector2.Read(pedazo)

			pasos++

			if n > 0 {
				acumulado = append(acumulado, pedazo[:n]...)
			}

			if e != nil {
				errL2 = e

				break
			}
		}

		if errL2 != nil && errL2.Error() != "EOF" {
			println("COMPR-40 M4b compress/zlib de a 512 FALLO:", errL2.Error(), "pasos=", pasos)
		} else {
			println("COMPR-40 M4b compress/zlib de a 512 bytes=", len(acumulado), "crc32=",
				crc32.ChecksumIEEE(acumulado), "pasos=", pasos, "(host:", inflado0, crcInflado0, ")")
		}

		_ = lector2.Close()
	}

	// ============================================================ M5: la lectura cruda del disco
	lecturaCruda, crcCrudo, errCrudo := crudoDelDisco(dirJuego+"/"+mpqListfile,
		int64(posListfile+offListfile), lenSector0)
	if errCrudo != nil {
		println("COMPR-40 M5 lectura cruda FALLO:", errCrudo.Error())
	} else {
		println("COMPR-40 M5 crudo del disco bytes=", lecturaCruda, "crc32=", crcCrudo,
			"<<< este es el CRC32 del sector ENCRIPTADO como esta en el disco (lo dice el host)")
	}

	// ============================================================ M8: el camino del motor, REPETIDO
	// El motor descomprime UN bloque por sector y los slots del puente son 4: si un solo camino se
	// olvidara del Close, el quinto bloque fallaria. Este bucle hace el MISMO camino que
	// `mpq_stream.go:deflate` (bytes.NewReader -> zlib.NewReader -> bytes.Buffer.ReadFrom -> Close) 200
	// veces sobre los MISMOS bytes buenos y dice en cual falla (si falla) y con cuanta memoria.
	if len(crudoSector) > 1 {
		b := bytes.NewReader(crudoSector[1:])
		fallos := 0
		// DONDE VIVE LA ENTRADA: el monton de Go es de COPIA (dos semiespacios): si la recoleccion mueve
		// el arreglo del sector, la direccion cambia y el que la tenia apuntada lee la vieja. El numero
		// se imprime cuando cambia, y el crc32 de la entrada cuando falla: eso separa "la entrada se
		// movio/la pisan" de "la zlib recibio bien los bytes y su estado es el que se corrompio".
		dirEntrada := uintptr(unsafe.Pointer(&crudoSector[1]))

		for i := 1; i <= 200; i++ {
			if dir := uintptr(unsafe.Pointer(&crudoSector[1])); dir != dirEntrada {
				println("COMPR-40 M8 vuelta", i, "la entrada SE MOVIO", dirEntrada, "->", dir)

				dirEntrada = dir
			}

			lector, errL := zlib.NewReader(b)
			if errL != nil {
				println("COMPR-40 M8 vuelta", i, "zlib.NewReader fallo:", errL.Error())

				fallos++

				break
			}

			var acum bytes.Buffer

			if _, errL = acum.ReadFrom(lector); errL != nil {
				println("COMPR-40 M8 vuelta", i, "ReadFrom fallo:", errL.Error(), "rc_zlib=", d2cZlibUltimoRc(),
					"arena=", d2cMontonArena(), "libre=", d2cMontonLibre(),
					"crc_entrada=", crc32.ChecksumIEEE(crudoSector), "(host:", crcSector0, ")")

				fallos++

				break
			}

			if errL = lector.Close(); errL != nil {
				println("COMPR-40 M8 vuelta", i, "Close fallo:", errL.Error())

				fallos++

				break
			}

			if acum.Len() != inflado0 {
				println("COMPR-40 M8 vuelta", i, "bytes=", acum.Len(), "(esperado", inflado0, ")")

				fallos++

				break
			}

			if i%10 == 0 {
				println("COMPR-40 M8 vuelta", i, "OK monton arena=", d2cMontonArena(),
					"usado=", d2cMontonUsado(), "libre=", d2cMontonLibre())
			}

			b.Reset(crudoSector[1:])
		}

		println("COMPR-40 M8 200 vueltas del camino del motor, fallos=", fallos,
			"monton libre=", ebiten.MemLibreDelMonton())
	}

	// ============================================================ M10: lo mismo, con los buffers en C
	// Los mismos 662 bytes, el mismo puente, 200 vueltas -- pero la entrada y la salida viven en .bss (los
	// buffers de `pantalla.c`), no en el monton de Go. Si aca NO falla, lo que se pisa es el monton de Go;
	// si falla igual, el que no da abasto es el monton de C.
	d2cCopiarAEntrada(uint(uintptr(unsafe.Pointer(&crudoSector[1]))), int32(len(crudoSector)-1))

	fallasM10 := 0
	vueltasM10 := 0

	for i := 1; i <= 200; i++ {
		slotM10 := d2cZlibStreamInit()
		if slotM10 < 0 {
			println("COMPR-40 M10 vuelta", i, "no hay slot, rc_zlib=", d2cZlibUltimoRc())

			fallasM10++

			break
		}

		malo := false
		terminado := false
		devuelto := int32(0)
		posM10 := int32(0)
		largoC := int32(len(crudoSector) - 1)

		for !terminado && !malo {
			n := d2cZlibStreamStep(slotM10, uintptr(d2cBufferIn()+uint(posM10)), largoC-posM10,
				uintptr(d2cBufferOut()+uint(devuelto)), int32(8192)-devuelto)

			if n < 0 {
				println("COMPR-40 M10 vuelta", i, "el paso fallo, rc_zlib=", d2cZlibUltimoRc(),
					"arena=", d2cMontonArena(), "libre=", d2cMontonLibre())

				malo = true

				break
			}

			posM10 += d2cZlibStreamConsumido(slotM10)
			devuelto += n

			if d2cZlibStreamFin(slotM10) != 0 {
				terminado = true
			}

			if n == 0 && d2cZlibStreamConsumido(slotM10) == 0 {
				println("COMPR-40 M10 vuelta", i, "el paso no avanzó")

				malo = true

				break
			}
		}

		d2cZlibStreamEnd(slotM10)

		if malo {
			fallasM10++

			break
		}

		if devuelto != inflado0 {
			println("COMPR-40 M10 vuelta", i, "bytes=", devuelto, "(host:", inflado0, ")")

			fallasM10++

			break
		}

		// la salida vive en C: se copia al monton de Go para poderle calcular el crc32 con el mismo reloj
		testigo := make([]byte, devuelto)
		d2cCopiarDeSalida(uint(uintptr(unsafe.Pointer(&testigo[0]))), devuelto)

		if crc32.ChecksumIEEE(testigo) != crcInflado0 {
			println("COMPR-40 M10 vuelta", i, "crc32=", crc32.ChecksumIEEE(testigo), "(host:", crcInflado0, ")")

			fallasM10++

			break
		}

		vueltasM10 = i
	}

	println("COMPR-40 M10 200 vueltas con los buffers en C: vueltas_ok=", vueltasM10, "fallas=", fallasM10,
		"arena=", d2cMontonArena(), "libre=", d2cMontonLibre())

	// ============================================================ M7: el CONTROL (implode, sin encriptar)
	// ANTES del motor a proposito: la lectura de d2data.mpq (114 sectores) todavía muere muda en esta
	// camada, y si esto fuera despues, el control sano no se mediria nunca (medido: 10/12 con M7 al final).
	controlDelMotor(dirJuego, "patch_d2.mpq", nombreDC6)

	// ============================================================ M6: el camino del MOTOR, los 3 MPQ
	// ANTES de leerlo de una, un instrumento TROZO A TROZO del mismo `(listfile)` y por el mismo camino
	// (CreateStream + Read, que es lo que hace ReadFile adentro): imprime el avance cada 8 trozos y dice
	// DONDE muere el archivo grande. Medido en la corrida anterior: d2data.mpq se lleva la consola puesta
	// (reboot mudo) entre el MAPA de patch_d2 y la linea del (listfile), asi que el primer paso es saber
	// si muere abriendo el MPQ, en el primer sector, o en el sector N.
	for _, nombre := range []string{"patch_d2.mpq", "d2data.mpq", "d2char.mpq"} {
		leerListfilePorTrozos(dirJuego, nombre)
		leerListfileDelMotor(dirJuego, nombre)
		println("COMPR-40 MAPA tras", nombre, "sbrk=", d2cSbrkActual(), "arena=", d2cMontonArena(),
			"libre_c=", d2cMontonLibre())
	}

	println("COMPR-40 fin")
}

// inflarEnStreaming: el camino de `compress/gzip` pedido en pedazos chicos.
func inflarEnStreaming(comprimido []byte) ([]byte, int, error) {
	slot := d2cZlibStreamInit()
	if slot < 0 {
		return nil, 0, errString("no hay slot del puente")
	}

	defer d2cZlibStreamEnd(slot)

	pedazo := make([]byte, chicoDeLectura)
	salida := make([]byte, 0, 8192)
	pos := 0
	lecturas := 0

	for {
		var src uintptr
		if pos < len(comprimido) {
			src = uintptr(unsafe.Pointer(&comprimido[pos]))
		}

		n := d2cZlibStreamStep(slot, src, int32(len(comprimido)-pos),
			uintptr(unsafe.Pointer(&pedazo[0])), int32(len(pedazo)))

		if n < 0 {
			return nil, lecturas, errString("la zlib dijo que los datos no son validos")
		}

		consumido := int(d2cZlibStreamConsumido(slot))
		termino := d2cZlibStreamFin(slot) != 0

		if n > 0 {
			salida = append(salida, pedazo[:n]...)
			lecturas++
		}

		pos += consumido

		if termino {
			return salida, lecturas, nil
		}

		if n == 0 && consumido == 0 {
			return nil, lecturas, errString("el paso no avanzo por ningun lado")
		}
	}
}

// crudoDelDisco: los bytes TAL CUAL del disco (sin descifrar ni descomprimir), para comparar el CRC32
// con el del host.
func crudoDelDisco(ruta string, offset int64, cuantos int) (int, uint32, error) {
	f, err := os.Open(ruta)
	if err != nil {
		return 0, 0, err
	}

	defer f.Close()

	datos := make([]byte, cuantos)

	n, err := f.ReadAt(datos, offset)
	if err != nil && n == 0 {
		return 0, 0, err
	}

	return n, crc32.ChecksumIEEE(datos[:n]), nil
}

// leerListfilePorTrozos: EL MISMO CAMINO que `ReadFile`, pero pidiendo los bytes de a un sector (4096) y
// contando. Sirve para decir DONDE muere un archivo grande (abriendo el MPQ, en el sector 1 o en el
// sector N) en vez de "no llego": medido en la corrida anterior, d2data.mpq se lleva la consola puesta
// (reboot mudo) sin imprimir nada entre el MAPA de patch_d2 y su linea del (listfile).
func leerListfilePorTrozos(dir, nombre string) {
	t0 := d2cMs()

	println("COMPR-40 M6b", nombre, "FromFile entra sbrk=", d2cSbrkActual(),
		"arena=", d2cMontonArena(), "libre_c=", d2cMontonLibre())

	m, err := d2mpq.FromFile(dir + "/" + nombre)
	if err != nil {
		println("COMPR-40 M6b", nombre, "FromFile FALLO:", err.Error())

		return
	}

	defer m.Close()

	println("COMPR-40 M6b", nombre, "FromFile OK a los", d2cMs()-t0, "ms sbrk=", d2cSbrkActual(),
		"libre_c=", d2cMontonLibre())

	flujo, err := m.ReadFileStream("(listfile)")
	if err != nil {
		println("COMPR-40 M6b", nombre, "ReadFileStream FALLO:", err.Error())

		return
	}

	defer flujo.Close()

	trozo := make([]byte, 4096)
	total, trozos := 0, 0
	sumaAcum := uint32(0)

	for {
		n, errL := flujo.Read(trozo)

		if n > 0 {
			total += n
			trozos++

			// el mismo acumulador de `suma()` (s*31+b): no hace falta materializar el archivo entero
			for _, b := range trozo[:n] {
				sumaAcum = sumaAcum*31 + uint32(b)
			}
		}

		if errL != nil {
			println("COMPR-40 M6b", nombre, "trozo", trozos, "FALLO:", errL.Error(), "bytes=", total,
				"ms=", d2cMs()-t0, "sbrk=", d2cSbrkActual(), "libre_c=", d2cMontonLibre())

			return
		}

		if n == 0 {
			break
		}

		if trozos%8 == 0 {
			println("COMPR-40 M6b", nombre, "trozo", trozos, "bytes=", total, "ms=", d2cMs()-t0,
				"sbrk=", d2cSbrkActual(), "arena=", d2cMontonArena(), "libre_c=", d2cMontonLibre())
		}
	}

	println("COMPR-40 M6b", nombre, "trozos bytes=", total, "trozos=", trozos, "suma=", sumaAcum,
		"ms=", d2cMs()-t0)
}

// leerListfileDelMotor: EL CAMINO DEL MOTOR sobre el disco, con el numero de NOMBRES.
//
// Se CUENTAN las lineas del resultado en vez de armar el `[]string` de `Listfile()`: en d2data.mpq el
// (listfile) son 466.867 bytes y 11.082 nombres, y el instrumento que los materializa ya tumbo una
// corrida en DC-002.33 (el conjunto vivo no entra en el semiespacio).
func leerListfileDelMotor(dir, nombre string) {
	t0 := d2cMs()

	m, err := d2mpq.FromFile(dir + "/" + nombre)
	if err != nil {
		println("COMPR-40 M6", nombre, "FromFile FALLO:", err.Error())

		return
	}

	defer m.Close()

	datos, err := m.ReadFile("(listfile)")
	ms := d2cMs() - t0

	if err != nil {
		println("COMPR-40 M6", nombre, "(listfile) FALLO:", err.Error(), "ms=", ms)

		return
	}

	println("COMPR-40 M6", nombre, "(listfile) bytes=", len(datos), "nombres=", contarNombres(datos),
		"crc32=", crc32.ChecksumIEEE(datos), "ms=", ms)
}

// contarNombres: cuantas lineas NO vacias tiene el (listfile), con el \r final sacado (el archivo viene
// con CRLF). Es lo mismo que cuenta el host, sin materializar los nombres.
func contarNombres(datos []byte) int {
	nombres := 0
	largo := 0

	for _, b := range datos {
		if b == 0x00 {
			break // el motor recorta los ceros del final (`strings.TrimRight(string(data), "\x00")`)
		}

		if b == '\n' {
			if largo > 0 {
				nombres++
			}

			largo = 0

			continue
		}

		if b != '\r' {
			largo++
		}
	}

	if largo > 0 {
		nombres++
	}

	return nombres
}

// controlDelMotor: el mismo camino con el bloque que YA se midio bien (implode, sin encriptar).
func controlDelMotor(dir, nombreMPQ, nombre string) {
	m, err := d2mpq.FromFile(dir + "/" + nombreMPQ)
	if err != nil {
		println("COMPR-40 M7 control FromFile FALLO:", err.Error())

		return
	}

	defer m.Close()

	datos, err := m.ReadFile(nombre)
	if err != nil {
		println("COMPR-40 M7 control FALLO:", err.Error())

		return
	}

	println("COMPR-40 M7 control bytes=", len(datos), "suma=", suma(datos),
		"(host: 141944 1831109629)")
}

func suma(datos []byte) uint32 {
	s := uint32(0)

	for _, b := range datos {
		s = s*31 + uint32(b)
	}

	return s
}

func leerArchivo(ruta string) ([]byte, error) {
	f, err := os.Open(ruta)
	if err != nil {
		return nil, err
	}

	defer f.Close()

	return io.ReadAll(f)
}

// ================================================================================================
// EL DISCO: el iso9660 de KOS MONTA EN SEGUNDO PLANO.
//
// Medido en la primera corrida de este POC (18/10): el primer `os.Open("/cd/juego/...")` del programa
// contesta `file does not exist` y en el MISMO log, inmediatamente despues, el driver dice
// `fs_iso9660: disc change detected / (joliet level 3 extensions detected)`. O sea que el archivo si esta
// (el listado del ISO del host lo comprueba) y el que no estaba listo era el disco: la primera vez que se
// toca `/cd` el driver arranca el montaje y le contesta error al que pregunto.
//
// Los POC del motor (tablas-mpq, motor-disco) no lo veian porque antes de tocar el disco decodifican la
// configuracion del romdisk y ese trabajo les da tiempo; este POC lo toca de entrada. Aca se espera al
// disco con reintentos y se imprime CUANTOS hicieron falta: el numero es el dato.
func prepararDisco() {
	t0 := d2cMs()

	// CADA INTENTO FALLIDO RESETEA EL MONTAJE: `iso_open` llama `init_percd()` mientras `percd_done` sea
	// falso, e `init_percd` arranca con `iso_reset()` + `cdrom_reinit()`. Medido (corrida del 18/10): con un
	// reintento cada 100 ms el disco NO quedo montado en 6 s, y con uno cada 1000 ms tampoco en 20 s; con un
	// solo intento el montaje si termino (el driver imprimio `fs_iso9660: disc change detected` y
	// `(joliet level 3 extensions detected)` DESPUES). O sea que el que pregunta tiene que ESPERAR sin tocar
	// `/cd` a que el montaje termine. Aca se espera de a 4 s y se imprime el numero.
	for intento := 1; intento <= 5; intento++ {
		f, err := os.Open(dirJuego + "/patch_d2.mpq")
		ms := d2cMs() - t0

		if err == nil {
			f.Close()

			println("COMPR-40 M0 disco listo en el intento", intento, "a los", ms, "ms")

			return
		}

		println("COMPR-40 M0 intento", intento, "a los", ms, "ms:", err.Error())

		esperarMs(4000)
	}

	println("COMPR-40 M0 el disco NO respondio en", d2cMs()-t0, "ms")
}

// leerDelDisco: lee un archivo del disco, esperando a que el montaje de arriba este listo.
func leerDelDisco(ruta string) ([]byte, int, error) {
	for intento := 1; intento <= 60; intento++ {
		datos, err := leerArchivo(ruta)
		if err == nil {
			return datos, intento, nil
		}

		if intento == 60 {
			return nil, intento, err
		}

		esperarMs(100)
	}

	return nil, 60, errString("el disco no respondio")
}

func esperarMs(ms uint) {
	t0 := d2cMs()

	// se duerme con KOS y se comprueba con el reloj cuanto se durmio de verdad: el dato es la diferencia
	// entre lo pedido y lo medido (el busy loop de Go daba ~1/4 de lo pedido y encima no soltaba la CPU).
	antes := d2cMs()
	d2cDormirMs(int32(ms))
	despues := d2cMs()

	if antes != despues && ultimoDormir != ms {
		ultimoDormir = ms

		println("COMPR-40 M0 dormir pedido=", ms, "medido=", despues-antes, "ms (desde el arranque:", t0, ")")
	}
}

var ultimoDormir uint //nolint:gochecknoglobals // solo para no repetir la misma linea de medida

type errString string

func (e errString) Error() string { return string(e) }

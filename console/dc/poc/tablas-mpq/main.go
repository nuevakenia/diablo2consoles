//go:build dreamcast

// DC-002.33: LAS TABLAS DEL MPQ COMPACTAS -- lo que dice cada tabla de cada MPQ del arranque, MEDIDO EN LA
// CONSOLA, para poder compararlo numero por numero con el host.
//
// Por que hace falta: la tarjeta cambia como el motor GUARDA las dos tablas de cada MPQ (bytes contiguos y
// consulta por indice, en vez de `map[uint64]*Hash` + `[]*Block`). Cambiar la representacion puede cambiar
// lo que se lee, y eso en el host no se ve (el host tiene RAM de sobra): el instrumento que importa es el
// MOTOR en la consola. Este POC mide, del disco y con el mismo codigo del motor:
//
//   1. LAS DOS TABLAS de cada MPQ del orden del motor: cuantas entradas (`TableEntries`) y el checksum de
//      los bytes de cada tabla (`TableChecksum`). Esos dos numeros son los que el host calcula sobre los
//      MISMOS archivos: si coinciden, la tabla entera es la misma de los dos lados.
//   2. EL `(listfile)` del archivo, leido por el camino del motor (`ReadFile`), y cuantos de esos nombres
//      encuentra la CONSULTA (`Contains` = el sondeo sobre los bytes, que en el host tiene que dar lo
//      mismo que daba el mapa de objetos: ver `TestBuscarHashIgualQueElMapaViejo`).
//
// La consola serie se inicializa en csrc/cshim.c (en Go no se puede: `dbgio_dev_select`).
//
// EL DEFECTO QUE ESTE POC DESTAPO (y que NO es de esta tarjeta): con los TRES MPQ en una sola corrida, el
// TERCERO no se puede abrir -- y no porque falte el archivo. Medido: en la tercera vuelta el nombre que sale
// del `MpqLoadOrder` de la configuracion ya no es `d2char.mpq` sino basura (`largoNombre= -1795502946`,
// `ruta= ` vacia, y la impresion del error se lleva la salida serie puesta: pasa a ser un volcado de
// memoria). Antes de eso la corrida hizo 11 recolecciones (ver `ARQ-MARCA gc_collect`) con el semiespacio
// cargado por la consulta de d2data.mpq. O sea: el nombre -- un dato VIVO -- se corrompe, que es un defecto
// del runtime/GC de la consola (raices), no del arreglo de las tablas; el corpus lo mide con el tercero en
// SU PROPIA corrida (una sola vuelta) y `d2char.mpq` ahi da los mismos numeros que el host.
//
// POR QUE ADEMAS IMPORTA `ebiten`: por el ENLACE, no por el dibujo. Medido en la primera corrida de este
// POC: sin importar ebiten (que es el que arrastra el paquete `kos` de libgodc) el binario ARRANCA y MUERE
// con `Unhandled exception: PC 8c1f8a70, code 1` -- un PC 6.6 KiB MAS ALLA del `_end` de su propia imagen,
// antes de la primera marca del runtime y sin imprimir ni una linea. El binario que si anda tiene 274
// simbolos `_kos.`; el que moria, CERO. Los POC que corren en la consola (motor-reservas, motor-disco)
// importan ebiten por sus instrumentos de RAM (`MemLibreDelMonton`, `VramDeLaPantalla`); aca se usa el
// mismo camino y de paso queda el mismo enlace que los otros POC del motor.
package main

import (
	"encoding/json"
	"os"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2mpq"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2config"
)

//extern d2c_serial_init
func d2cSerialInit()

//extern d2c_ms
func d2cMs() uint

//extern d2c_monton_libre
func d2cMontonLibre() int

const configConsola = "/rd/OpenDiablo2/config.json"

// leerListfile: leer o no el `(listfile)` de cada MPQ dentro de este bucle.
//
// MEDIDO en DC-002.39c (y NO es un capricho): con la lectura puesta, la corrida de los 3 MPQ muere antes de
// la consulta del TERCERO y nunca imprime el TOTAL de los 21.253. La corrida de peor caso medida:
//
//   TABLAS-MPQ 3 d2char.mpq entradasHash= 16384 ... ms= 3519
//   Unhandled exception: PC 656a626f, code 1        <- el PC es TEXTO ("obje": un pedazo del (listfile))
//   PR 8c0159aa -> bytes.Buffer.ReadFrom (bytes/bytes.go:186)
//   (y en la corrida sin la sonda de paso: REIOS: Booting up -- la consola se REINICIA, sin mensaje)
//
// o sea el que se lleva la corrida puesta es el `ReadFile("(listfile)")`, no la consulta. Con la lectura
// fuera, el MISMO binario y el MISMO disco dan `TABLAS-MPQ-TOTAL archivos= 3 nombres= 21253
// contenidos= 21253` (y sigue corriendo: 3 pasadas en 150 s). El defecto del (listfile) tiene su propia
// tarjeta (DC-002.39c, seccion "lo que queda abierto"); este POC mide LA CONSULTA y el (listfile) lo mide
// su corpus aparte (`scripts/console/checar-compr-240.sh`, control 7 = el (listfile) de patch_d2.mpq
// contra el crc32 del host).
const leerListfile = false

// configDelArranque: el `MpqPath` y el `MpqLoadOrder` que el motor leyo de su config del romdisk. Es el
// ORDEN que importa: `d2app/initialization.go` recorre `MpqLoadOrder` y abre cada archivo.
func configDelArranque() *d2config.Configuration {
	conf, err := os.Open(configConsola)
	if err != nil {
		println("TABLAS-MPQ config: no pude abrir", configConsola+":", err.Error())
		return nil
	}

	defer conf.Close()

	cfg := &d2config.Configuration{}
	cfg.SetPath(configConsola)

	if err := json.NewDecoder(conf).Decode(cfg); err != nil {
		println("TABLAS-MPQ config: el Decode fallo:", err.Error())
		return nil
	}

	return cfg
}

// nombresDelDisco: la CONSULTA, nombre por nombre, con el sondeo sobre los BYTES de la tabla.
//
// Los nombres viajan en un archivo de TEXTO PLANO al lado de los MPQ (`/cd/juego/nombres_<mpq>.txt`, uno por
// linea) y NO se sacan del `(listfile)` del propio archivo porque en la consola ese `ReadFile` FALLA
// (medido: `zlib: invalid header`, un defecto aparte de esta tarjeta); los escribe el host con la sonda
// `console/dc/sonda-tablas-mpq -nombres-dir`, leyendo el MISMO `(listfile)` con el MISMO codigo. Lo que se
// mide aca es la consulta de la consola contra la tabla de la consola.
//
// SE LEE POR TROZOS Y DE A UNA LINEA, no con `os.ReadFile` de todo el archivo: el de d2char.mpq son 408.270
// bytes y con el archivo entero VIVO (mas los 10.009 nombres) la consola moria DENTRO de la linea que
// imprimia (medido: con `v=393240` usados del semiespacio, la salida serie pasa a ser un volcado de memoria
// y `main.main-volvio` no sale nunca). Con el trozo de 4 KiB y una linea por vez el conjunto vivo no crece
// con el tamano de la lista.
func nombresDelDisco(dir, nombreMPQ string, m *d2mpq.MPQ) (nombres, encontrados, largas int) {
	const maxLargo = 1024 // un nombre del juego no pasa de unas decenas de caracteres

	base := strings.ReplaceAll(strings.TrimSuffix(nombreMPQ, ".mpq"), ".", "_")
	ruta := dir + "/nombres_" + base + ".txt"

	f, err := os.Open(ruta)
	if err != nil {
		println("TABLAS-MPQ   ", nombreMPQ, "sin lista de nombres en", ruta+":", err.Error())

		return 0, 0, 0
	}

	defer f.Close()

	trozo := make([]byte, 4096)
	linea := make([]byte, 0, maxLargo)
	truncada := false

	// una linea: se cuenta y se consulta. Una linea mas larga que `maxLargo` se cuenta como LARGA y NO se
	// consulta (mejor decir que no se midio que dar un numero que parece una respuesta).
	procesar := func() {
		if len(linea) > 0 {
			nombres++

			if truncada {
				largas++
			} else if m.Contains(string(linea)) {
				encontrados++
			}
		}

		linea = linea[:0]
		truncada = false
	}

	for {
		n, errLectura := f.Read(trozo)

		for _, b := range trozo[:n] {
			if b == '\n' {
				procesar()

				continue
			}

			if len(linea) < maxLargo {
				linea = append(linea, b)
			} else {
				truncada = true
			}
		}

		if errLectura != nil {
			break
		}
	}

	procesar()

	return nombres, encontrados, largas
}

func main() {
	d2cSerialInit()

	println("TABLAS-MPQ arranca (DC-002.33: las dos tablas del MPQ como BYTES, consultadas por indice)")
	println("TABLAS-MPQ monton de C al arrancar:", d2cMontonLibre(), "bytes libres")

	cfg := configDelArranque()
	if cfg == nil {
		println("TABLAS-MPQ sin config: no hay orden del motor que recorrer")
		return
	}

	println("TABLAS-MPQ MpqPath=", cfg.MpqPath, "MpqLoadOrder=", len(cfg.MpqLoadOrder))

	archivos, nombres, contenidos := 0, 0, 0

	// DC-002.39: se recorre por INDICE y el nombre se RELEE del arreglo vivo en cada vuelta, en vez de
	// `for i, nombreArchivo := range cfg.MpqLoadOrder`. Medido en la consola, en la vuelta 3 (d2char.mpq,
	// la que dejaba el nombre con el largo en basura): con el `range` el bucle se queda con el valor de la
	// CADENA (un pedazo del buffer que leyo el decodificador) y ese valor puede quedar viejo cuando el
	// cuerpo (abrir y leer el MPQ) dispara una recoleccion: el arreglo vivo del `MpqLoadOrder` tiene los
	// largos correctos en todo momento y un mismo nombre consultado DE NUEVO si se encuentra
	// (`segundo intento contiene= true`), o sea el que falla es el valor que el bucle tenia en la mano.
	// Releyendo por indice el nombre sale bien en las tres vueltas.
	// Evidencia: console/dc/evidencia/dc-002.39-nombre-corrompido.txt.
	for i := 0; i < len(cfg.MpqLoadOrder); i++ {
		nombreArchivo := cfg.MpqLoadOrder[i]
		ruta := cfg.MpqPath + "/" + nombreArchivo
		t0 := d2cMs()

		m, err := d2mpq.FromFile(ruta)
		if err != nil {
			println("TABLAS-MPQ", i+1, nombreArchivo, "FALLO:", err.Error(), "largoNombre=", len(nombreArchivo),
				"ruta=", ruta)

			// QUE HAY EN EL DISCO: si un MPQ no se puede abrir, esto dice si el archivo esta (y con que
			// nombre) en vez de dejar la duda. Se prueba cada nombre conocido con `os.Open`.
			for _, sonda := range []string{"patch_d2.mpq", "d2data.mpq", "d2char.mpq",
				"nombres_patch_d2.txt", "nombres_d2data.txt", "nombres_d2char.txt"} {
				if f, errSonda := os.Open(cfg.MpqPath + "/" + sonda); errSonda == nil {
					println("TABLAS-MPQ   el disco TIENE", sonda)
					f.Close()
				} else {
					println("TABLAS-MPQ   el disco NO tiene", sonda+":", errSonda.Error())
				}
			}

			continue
		}

		he, be := m.TableEntries()
		ch, cb := m.TableChecksum()

		// LAS DOS TABLAS PRIMERO: si la consulta de abajo se llevara la corrida puesta, los numeros de la
		// tarjeta (las entradas y el checksum de los BYTES de cada tabla) ya salieron.
		println("TABLAS-MPQ", i+1, nombreArchivo,
			"entradasHash=", he, "entradasBloque=", be,
			"checksumHash=", ch, "checksumBloque=", cb,
			"ms=", d2cMs()-t0)

		// el (listfile) por el camino del motor: en el HOST anda y en la CONSOLA el del TERCER MPQ se lleva
		// la corrida puesta (ver `leerListfile` arriba y la seccion de DC-002.39c). Se intenta igual y, si
		// falla, se dice con su error. OJO: `datos` se declara AFUERA del if a proposito: cuando se lee,
		// el buffer queda vivo hasta el fin de la vuelta (es parte de lo que el corpus mide).
		var (
			datos   []byte
			errList error
		)

		if !leerListfile {
			println("TABLAS-MPQ   ", nombreArchivo, "(listfile) NO SE LEE (DC-002.39c: dispara un salto a basura)")
		} else {
			datos, errList = m.ReadFile("(listfile)")

			if errList != nil {
				println("TABLAS-MPQ   ", nombreArchivo, "(listfile) FALLO:", errList.Error())
			} else {
				println("TABLAS-MPQ   ", nombreArchivo, "(listfile) bytes=", len(datos))
			}
		}

		// LA CONSULTA: los nombres que el corpus deja al lado de los MPQ (los escribio el host leyendo el
		// mismo (listfile)) y cuantos encuentra el sondeo de la CONSOLA sobre los BYTES de la tabla.
		cuantos, encontrados, largas := nombresDelDisco(cfg.MpqPath, nombreArchivo, m)

		println("TABLAS-MPQ   ", nombreArchivo, "nombres=", cuantos, "contenidos=", encontrados,
			"largas=", largas, "msConsulta=", d2cMs()-t0)

		m.Close()

		archivos++
		nombres += cuantos
		contenidos += encontrados
	}

	println("TABLAS-MPQ-TOTAL archivos=", archivos, "nombres=", nombres, "contenidos=", contenidos,
		"libreFinal=", d2cMontonLibre(), "ebitenLibre=", ebiten.MemLibreDelMonton())
}

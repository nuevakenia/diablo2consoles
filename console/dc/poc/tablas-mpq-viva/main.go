//go:build dreamcast

// DC-002.39: EL VIGIA DEL ARREGLO DEL `MpqLoadOrder`.
//
// POR QUE EXISTE: el instrumento DC-002.33 (tablas-mpq) mide que, con los TRES MPQ en una sola corrida, la
// TERCERA vuelta imprime el nombre del MPQ seguido de un volcado de ~428 KB de memoria: el `len` de la
// cabecera de string salio enorme. La pregunta de esta tarjeta es QUIEN la escribio: el recolector de basura
// (un objeto que se mueve y una raiz que no se actualiza) o el decodificador JSON.
//
// COMO PREGUNTA: ESTE POC ES `tablas-mpq` CON UN VIGIA. Le pasa al runtime la DIRECCION del arreglo de
// strings del `MpqLoadOrder` (y la de un arreglo de LITERALES con los mismos tres nombres, de testigo) y el
// runtime imprime las 6 palabras de cada arreglo ANTES y DESPUES de cada recoleccion (`ARQ-VIGIA`, ver
// libgodc/runtime/vigia_viva.c en la copia privada del runtime). Con eso se ve EN QUE recoleccion cambia
// cada palabra y de cuanto a cuanto: si el `len` se rompe DENTRO de una recoleccion es el GC; si el `len` ya
// estaba roto ANTES de la recoleccion, lo escribio otro (el JSON, o el propio codigo Go).
//
// El POC tambien marca la FASE del trabajo (`d2c_fase`) para saber donde estaba el programa en cada
// recoleccion, y pide un volcado de las palabras justo antes de la impresion que se lleva la salida puesta.
//
// El resto del POC es el de DC-002.33, sin tocar, para que lo que se mida sea lo mismo.
package main

import (
	"encoding/json"
	"os"
	"strings"
	"unsafe"

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

//extern d2c_vigia_dir
func d2cVigiaDir(cfg, literales uintptr)

//extern d2c_vigia_hdr
func d2cVigiaHdr(hdr uintptr)

//extern d2c_vigia_arreglo
func d2cVigiaArreglo(arreglo uintptr)

//extern d2c_hist
func d2cHist()

//extern d2c_fallo
func d2cFallo(valor, largo uintptr)

//extern d2c_fase
func d2cFase(f uintptr)

//extern d2c_mira
func d2cMira()

const configConsola = "/rd/OpenDiablo2/config.json"

// las fases del trabajo: el vigia las imprime en cada recoleccion (una recoleccion metida dentro de
// `FromFile` y otra metida en la impresion dicen cosas distintas). Se numeran por vuelta para que no se
// confundan entre vueltas: la vuelta `i` usa 10+4*i, 10+4*i+1, ...
const (
	faseTerminado = 60
)

func faseDe(vuelta, paso int) uintptr { return uintptr(10 + 4*vuelta + paso) }

// configDelArranque: el `MpqPath` y el `MpqLoadOrder` que el motor leyo de su config del romdisk.
func configDelArranque() *d2config.Configuration {
	conf, err := os.Open(configConsola)
	if err != nil {
		println("TABLAS-MPQ-VIVA config: no pude abrir", configConsola+":", err.Error())
		return nil
	}

	defer conf.Close()

	cfg := &d2config.Configuration{}
	cfg.SetPath(configConsola)

	if err := json.NewDecoder(conf).Decode(cfg); err != nil {
		println("TABLAS-MPQ-VIVA config: el Decode fallo:", err.Error())
		return nil
	}

	return cfg
}

// nombresDelDisco: la CONSULTA, nombre por nombre, con el sondeo sobre los BYTES de la tabla (igual que DC-002.33).
func nombresDelDisco(dir, nombreMPQ string, m *d2mpq.MPQ) (nombres, encontrados, largas int) {
	const maxLargo = 1024

	base := strings.ReplaceAll(strings.TrimSuffix(nombreMPQ, ".mpq"), ".", "_")
	ruta := dir + "/nombres_" + base + ".txt"

	f, err := os.Open(ruta)
	if err != nil {
		println("TABLAS-MPQ-VIVA   ", nombreMPQ, "sin lista de nombres en", ruta+":", err.Error())

		return 0, 0, 0
	}

	defer f.Close()

	trozo := make([]byte, 4096)
	linea := make([]byte, 0, maxLargo)
	truncada := false

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

	println("TABLAS-MPQ-VIVA arranca (DC-002.39: quien corrompe el nombre del MpqLoadOrder, el GC o el JSON)")
	println("TABLAS-MPQ-VIVA monton de C al arrancar:", d2cMontonLibre(), "bytes libres")

	cfg := configDelArranque()
	if cfg == nil {
		println("TABLAS-MPQ-VIVA sin config: no hay orden del motor que recorrer")
		return
	}

	// EL TESTIGO: los mismos tres nombres como LITERALES del binario. Su arreglo lo vigila el runtime igual
	// que el de la config: si el de la config se rompe y el de literales NO, el defecto son los datos de la
	// config (el JSON); si se rompen los dos, es del GC o del codigo que recorre (raices).
	literales := []string{"patch_d2.mpq", "d2data.mpq", "d2char.mpq"}

	d2cVigiaDir(uintptr(unsafe.Pointer(&cfg.MpqLoadOrder[0])), uintptr(unsafe.Pointer(&literales[0])))

	// LA CABECERA (el eslabon): el GC tiene que mudarla cuando mueve el arreglo. Si no la muda, el codigo
	// lee el arreglo en la direccion vieja (memoria que el GC ya reparte de nuevo) y sale basura.
	d2cVigiaHdr(uintptr(unsafe.Pointer(&cfg.MpqLoadOrder)))
	d2cVigiaArreglo(uintptr(unsafe.Pointer(&cfg.MpqLoadOrder[0])))

	println("TABLAS-MPQ-VIVA MpqPath=", cfg.MpqPath, "MpqLoadOrder=", len(cfg.MpqLoadOrder),
		"cabecera=", uintptr(unsafe.Pointer(&cfg.MpqLoadOrder)),
		"arreglo=", uintptr(unsafe.Pointer(&cfg.MpqLoadOrder[0])))
	println("TABLAS-MPQ-VIVA nombres:", cfg.MpqLoadOrder[0], cfg.MpqLoadOrder[1], cfg.MpqLoadOrder[2],
		"literales:", literales[0], literales[1], literales[2])

	d2cMira()

	archivos, nombres, contenidos := 0, 0, 0

	for i, nombreArchivo := range cfg.MpqLoadOrder {
		d2cFase(faseDe(i, 0))

		// DC-002.39: ¿que leyo el bucle y de donde lo leyo? El arreglo VIVO (via la cabecera de la config)
		// tiene los largos 12,10,10 en toda recoleccion: si estos dos no coinciden, el bucle lee memoria
		// vieja y hay que ver donde guardo la direccion.
		{
			leido := (*[2]uintptr)(unsafe.Pointer(&nombreArchivo))
			vivo := (*[2]uintptr)(unsafe.Pointer(&cfg.MpqLoadOrder[i]))

			println("TABLAS-MPQ-VIVA-VUELTA", i, "leidoPtr=", leido[0], "leidoLen=", leido[1],
				"vivoPtr=", vivo[0], "vivoLen=", vivo[1],
				"cfg=", uintptr(unsafe.Pointer(cfg)),
				"cabeceraPtr=", *(*uintptr)(unsafe.Pointer(&cfg.MpqLoadOrder)))

			// si el largo no coincide, buscar de DONDE salio ese par {valor, largo}
			if leido[1] != vivo[1] {
				d2cFallo(leido[0], leido[1])
			}
		}

		ruta := cfg.MpqPath + "/" + nombreArchivo

		// EL VIGIA ANTES DE ABRIR: si el nombre ya viene roto, la recoleccion no tuvo la culpa.
		d2cMira()

		d2cFase(faseDe(i, 1))

		t0 := d2cMs()

		m, err := d2mpq.FromFile(ruta)
		if err != nil {
			d2cFase(50 + uintptr(i))

			// las palabras del arreglo, pedidas ANTES de la impresion que se lleva la salida puesta
			d2cMira()

			// el historico: por que direcciones paso el arreglo y que quedo en las viejas
			d2cHist()

			println("TABLAS-MPQ-VIVA", i+1, nombreArchivo, "FALLO:", err.Error(), "largoNombre=", len(nombreArchivo),
				"ruta=", ruta)

			for _, sonda := range []string{"patch_d2.mpq", "d2data.mpq", "d2char.mpq",
				"nombres_patch_d2.txt", "nombres_d2data.txt", "nombres_d2char.txt"} {
				if f, errSonda := os.Open(cfg.MpqPath + "/" + sonda); errSonda == nil {
					println("TABLAS-MPQ-VIVA   el disco TIENE", sonda)
					f.Close()
				} else {
					println("TABLAS-MPQ-VIVA   el disco NO tiene", sonda+":", errSonda.Error())
				}
			}

			continue
		}

		he, be := m.TableEntries()
		ch, cb := m.TableChecksum()

		d2cFase(faseDe(i, 2))

		println("TABLAS-MPQ-VIVA", i+1, nombreArchivo,
			"entradasHash=", he, "entradasBloque=", be,
			"checksumHash=", ch, "checksumBloque=", cb,
			"ms=", d2cMs()-t0)

		d2cFase(faseDe(i, 3))

		datos, errList := m.ReadFile("(listfile)")

		if errList != nil {
			println("TABLAS-MPQ-VIVA   ", nombreArchivo, "(listfile) FALLO:", errList.Error())
		} else {
			println("TABLAS-MPQ-VIVA   ", nombreArchivo, "(listfile) bytes=", len(datos))
		}

		d2cFase(40 + uintptr(i))

		cuantos, encontrados, largas := nombresDelDisco(cfg.MpqPath, nombreArchivo, m)

		d2cFase(45 + uintptr(i))

		println("TABLAS-MPQ-VIVA   ", nombreArchivo, "nombres=", cuantos, "contenidos=", encontrados,
			"largas=", largas, "msConsulta=", d2cMs()-t0)

		m.Close()

		archivos++
		nombres += cuantos
		contenidos += encontrados
	}

	d2cFase(faseTerminado)

	// LA PREGUNTA DEL ESLABON: si el arreglo sigue vivo, se tiene que poder leer releyendo la CABECERA de
	// la config en cada vuelta (recorrido por indice). Si esto sale bien y el recorrido `range` de arriba
	// salio mal, lo que quedo viejo es la direccion que el bucle se guardo, no el arreglo.
	println("TABLAS-MPQ-VIVA-PORINDICE empieza len=", len(cfg.MpqLoadOrder), "MpqPath=", cfg.MpqPath)
	d2cMira()

	for j := 0; j < len(cfg.MpqLoadOrder); j++ {
		n := cfg.MpqLoadOrder[j]
		println("TABLAS-MPQ-VIVA-PORINDICE", j, "nombre=", n, "largo=", len(n))
	}

	println("TABLAS-MPQ-VIVA-PORINDICE fin")
	d2cMira()

	println("TABLAS-MPQ-VIVA-TOTAL archivos=", archivos, "nombres=", nombres, "contenidos=", contenidos,
		"libreFinal=", d2cMontonLibre(), "ebitenLibre=", ebiten.MemLibreDelMonton())
}

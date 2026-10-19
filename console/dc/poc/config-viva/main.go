//go:build dreamcast

// DC-002.39: QUIEN CORROMPE EL NOMBRE DEL MpqLoadOrder -- EL RECOLECTOR O EL JSON.
//
// POR QUE EXISTE ESTE POC (y por que es chiquito): el instrumento de DC-002.33
// (`console/dc/poc/tablas-mpq`, corpus `scripts/console/checar-tablas-233.sh`) destapo que en una corrida
// con los TRES MPQ en una sola pasada el TERCERO no se puede abrir, y que el nombre que sale del
// `MpqLoadOrder` de la configuracion ya no es el nombre: la linea del log queda
// `TABLAS-MPQ 3 d2char.mpq` + un VOLCADO DE MEMORIA de ~400 KB (medido: la vuelta 3 de
// `run-tablas/flycast.log` trae 1378 + 395050 + ... bytes de memoria cruda, con `/cd/juego` y `d2data.mpq`
// adentro). O sea el `len` del string se fue a un valor enorme: `println` del nombre imprimio 400 KB.
// Aquella corrida tardo minutos y se llevo puesta la salida serie; este POC hace la MISMA pregunta en
// segundos y no ensucia el log (imprime un marcador cuando el largo de un nombre no es coherente).
//
// LAS DOS SOSPECHAS QUE HAY QUE SEPARAR (no se decide de palabra, se mide):
//
//  1. EL RECOLECTOR DE BASURA (libgodc/runtime/gc_copy.c): si un objeto se MUEVE y una raiz no se
//     actualiza, el nombre queda apuntando al semiespacio VIEJO, que despues se reusa -> basura. Es la
//     familia de DC-002.24 (la copia por Store Queue del GC escribia en `App+156`).
//  2. EL JSON DE LA CAPA (`console/dc/std/encoding/json` + `d2core/d2config/d2config_json.go`): si al
//     decodificar los `string` quedaran apuntando al buffer del archivo (sin copiar), el nombre se vuelve
//     basura cuando el semiespacio reusa ese espacio.
//
// COMO LO SEPARA: el POC lee la config con el MISMO camino que el POC que fallo (`os.Open` +
// `json.NewDecoder(...).Decode`) y arma TRES FAMILIAS con los MISMOS tres nombres:
//
//	J = `cfg.MpqLoadOrder` (lo que armo el json; el sospechoso 2)
//	D = una copia hecha por ESTE POC (`string([]byte(n))`): misma vida, misma forma, pero copiada aca
//	C = los mismos nombres escritos como LITERALES en el paquete: su tabla de cabeceras la reserva el
//	    arranque (igual que J) y su CONTENIDO vive en .rodata, que no se mueve NUNCA (el control sano)
//
// y en cada vuelta, despues de forzar recolecciones, dice de las tres: cuantos nombres son iguales a los
// literales (`iguales=0..3`), el LARGO de cada uno (un largo negativo o gigante = la cabecera del string
// se corrompio) y el puntero de datos de J (para ver si quedo en el semiespacio viejo). Ademas prueba
// `os.Open(cfg.MpqPath + "/" + nombre)` de los tres: si el nombre se corrompe, el archivo del disco
// "no esta" -- que es exactamente el sintoma del corpus.
//
// LECTURA DEL RESULTADO:
//   - si J y D se corrompen y C no  -> el que rompe es el dato del json (sospecha 2) o, mas fino, lo que
//     el json dejo apuntando a otro lado;
//   - si las TRES se corrompen       -> es el recolector / las raices (sospecha 1); C no puede venir del json;
//   - si nunca se corrompen, el POC tiene que forzar MAS recolecciones (el defecto es intermitente: lo que
//     lo dispara no es un numero fijo de vueltas sino que el recolector caiga en cierto lugar).
//
// La consola serie se inicializa en csrc/cshim.c (en Go no se puede: `dbgio_dev_select`). El `ebiten` se
// importa por el ENLACE, no por el dibujo: es el que arrastra el paquete `kos` de libgodc, y sin el el
// binario muere con `Unhandled exception` antes de imprimir una linea (medido en DC-002.33).
package main

import (
	"encoding/json"
	"os"
	"runtime"
	"unsafe"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2config"
)

//extern d2c_serial_init
func d2cSerialInit()

//extern d2c_monton_libre
func d2cMontonLibre() int

const configConsola = "/rd/OpenDiablo2/config.json"

// La FAMILIA C (el control sano): los mismos tres nombres como LITERALES del paquete. Su contenido esta
// en .rodata (no se mueve nunca); su tabla de cabeceras la reserva el arranque, igual que la de J.
var literales = []string{"patch_d2.mpq", "d2data.mpq", "d2char.mpq"}

// maxLargo: un nombre del juego no pasa de unas decenas de caracteres. Un string con otro largo esta
// roto, y LEERLO (o imprimirlo) es lo que se lleva la salida serie puesta: se dice el largo y no se toca.
const maxLargo = 64

// configDelArranque: la config del romdisk, con el MISMO camino del POC que destapo el defecto.
func configDelArranque() *d2config.Configuration {
	conf, err := os.Open(configConsola)
	if err != nil {
		println("CONFIG-VIVA config: no pude abrir", configConsola+":", err.Error())
		return nil
	}

	defer conf.Close()

	cfg := &d2config.Configuration{}
	cfg.SetPath(configConsola)

	if err := json.NewDecoder(conf).Decode(cfg); err != nil {
		println("CONFIG-VIVA config: el Decode fallo:", err.Error())
		return nil
	}

	return cfg
}

// copia: la FAMILIA D. Los mismos nombres, copiados por el POC (no por el json).
func copia(nombres []string) []string {
	copia := make([]string, 0, len(nombres))

	for i := 0; i < len(nombres); i++ {
		n := nombres[i]

		if !largoSano(n) {
			copia = append(copia, "")

			continue
		}

		copia = append(copia, string([]byte(n)))
	}

	return copia
}

// largoSano: el largo del string es de un nombre (0..maxLargo). Un largo negativo o gigante significa que
// la CABECERA del string esta rota: leer sus bytes o imprimirlo es lo que volco 400 KB de memoria.
func largoSano(s string) bool {
	return len(s) >= 0 && len(s) <= maxLargo
}

// largoDe: el largo del i-esimo nombre, o -1 si la tabla misma no es una tabla de tres.
func largoDe(nombres []string, i int) int {
	if len(nombres) != 3 || i < 0 || i >= 3 {
		return -1
	}

	return len(nombres[i])
}

// igual: los bytes del nombre son los del esperado. NO se toca la memoria si el largo no cierra.
func igual(nombre, esperado string) bool {
	if !largoSano(nombre) || len(nombre) != len(esperado) || len(nombre) == 0 {
		return false
	}

	for i := 0; i < len(nombre); i++ {
		if nombre[i] != esperado[i] {
			return false
		}
	}

	return true
}

// iguales: cuantos de los tres nombres son los que tienen que ser (0..3).
func iguales(nombres []string) int {
	if len(nombres) != 3 {
		return 0
	}

	cuantos := 0

	for i := 0; i < 3; i++ {
		if igual(nombres[i], literales[i]) {
			cuantos++
		}
	}

	return cuantos
}

// punteroDe: el PRIMER campo de la cabecera del string (el puntero a sus bytes). Es UN dato del runtime:
// sirve para ver si quedo apuntando al semiespacio que el recolector acaba de abandonar (las dos
// direcciones las imprime el runtime: `ARQ-MARCA gc_init-semiespacio0/1`).
func punteroDe(s string) uintptr {
	return *(*uintptr)(unsafe.Pointer(&s))
}

// imprimible: el nombre, si su largo es de un nombre; si no, un marcador. Es lo que impide que este POC
// vuelque memoria como el que lo destapo.
func imprimible(s string) string {
	if !largoSano(s) {
		return "<largo-roto>"
	}

	return s
}

// abreLosTres: la prueba del DISCO, la misma del corpus: `os.Open(cfg.MpqPath + "/" + nombre)`. Los tres
// archivos del romdisk estan (vacios) con esos nombres exactos: si el nombre se rota, "no esta".
func abreLosTres(mpqPath string, nombres []string) (abren int, largos int) {
	if len(nombres) != 3 {
		return 0, 0
	}

	for i := 0; i < 3; i++ {
		if !largoSano(nombres[i]) {
			largos++

			continue
		}

		f, err := os.Open(mpqPath + "/" + nombres[i])
		if err == nil {
			abren++
			f.Close()
		}
	}

	return abren, largos
}

// unaVuelta: lo que dice el POC de una vuelta. UNA linea, con las tres familias y el disco.
//
// EL TRABAJO DE LA VUELTA ES EL DEL CORPUS, no solo mirar: `cfg.MpqPath + "/" + nombre` (una reserva nueva
// por nombre) y `os.Open`, que es lo que la vuelta 3 del corpus no pudo hacer. Mirar sin trabajar no
// reproduce nada.
func unaVuelta(ronda int, j, d, c []string, mpqPath string) {
	abrenJ, rotosJ := abreLosTres(mpqPath, j)

	println("CONFIG-VIVA ronda=", ronda,
		"J", iguales(j), largoDe(j, 0), largoDe(j, 1), largoDe(j, 2), "ptrJ", punteroDe(primerNombre(j)),
		"D", iguales(d), largoDe(d, 0), largoDe(d, 1), largoDe(d, 2),
		"C", iguales(c), largoDe(c, 0), largoDe(c, 1), largoDe(c, 2),
		"discoJ", abrenJ, "rotosJ", rotosJ, "libreC", d2cMontonLibre())
}

// primerNombre: el nombre 0 de la familia (o "" si la tabla misma no cierra). Es el que se le mira la
// cabecera.
func primerNombre(nombres []string) string {
	if len(nombres) != 3 {
		return ""
	}

	return nombres[0]
}

// ranuras y trozo: la BASURA de la corrida. Ocho trozos de 40 KiB que se ROTAN (cada vuelta pisa uno, o
// sea que el viejo queda basura de verdad). Medido en la primera corrida de este POC: acumular trozos en
// un slice que crece (`append` + `[len-12:]`) NO es basura -- el arreglo de fondo queda VIVO entero y el
// semiespacio se llena hasta el techo (usado 521.720 de 524.288) y ademas la reserva del arreglo pasa de
// 64 KiB, o sea que se va al MONTON DE C y lo revienta (`fatal error: out of memory`, arena=1211072
// used=1209032 free=2040). Con la rotacion: ~320 KiB vivos, que es el estado del semiespacio de la corrida
// del corpus (usado ~393 KiB) y la recoleccion cae por umbral, como ahi.
const (
	ranuras = 8
	trozo   = 40 * 1024
)

func main() {
	d2cSerialInit()

	println("CONFIG-VIVA arranca (DC-002.39: quien corrompe el nombre del MpqLoadOrder -- el GC o el json)")
	println("CONFIG-VIVA montonC=", d2cMontonLibre(), "ebitenLibre=", ebiten.MemLibreDelMonton())

	cfg := configDelArranque()
	if cfg == nil {
		println("CONFIG-VIVA sin config: no hay MpqLoadOrder que vigilar")
		return
	}

	println("CONFIG-VIVA MpqPath=", imprimible(cfg.MpqPath), "MpqLoadOrder=", len(cfg.MpqLoadOrder))
	println("CONFIG-VIVA nombres:", imprimible(cfg.MpqLoadOrder[0]), imprimible(cfg.MpqLoadOrder[1]),
		imprimible(cfg.MpqLoadOrder[2]))

	// LA FAMILIA D: la copia del POC, armada mientras J todavia esta sana (se dice si no lo estaba).
	d := copia(cfg.MpqLoadOrder)
	println("CONFIG-VIVA familiaD iguales=", iguales(d), "de 3")

	basura := make([][]byte, ranuras)

	for ronda := 1; ronda <= 400; ronda++ {
		basura[ronda%ranuras] = make([]byte, trozo)

		unaVuelta(ronda, cfg.MpqLoadOrder, d, literales, cfg.MpqPath)

		// La recoleccion EXPLICITA cada 4 vueltas: la del umbral la pide el monton solo (y esa es la que
		// importa), pero forzarla garantiza que haya recolecciones tambien cuando el usado esta bajo.
		if ronda%4 == 0 {
			runtime.GC()
		}
	}

	println("CONFIG-VIVA-TOTAL vueltas=400 J=", iguales(cfg.MpqLoadOrder), "largosJ=", largoDe(cfg.MpqLoadOrder, 0),
		largoDe(cfg.MpqLoadOrder, 1), largoDe(cfg.MpqLoadOrder, 2), "D=", iguales(d), "C=", iguales(literales),
		"libreC=", d2cMontonLibre(), "ebitenLibre=", ebiten.MemLibreDelMonton())
}


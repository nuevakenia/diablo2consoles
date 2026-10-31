//go:build dreamcast
// +build dreamcast

// SONDA DEL MAPA + GC (DC-002.83).
//
// QUE MIDE: si los mapas de Go sobreviven a una recoleccion del GC de libgodc. El sintoma que la dispara
// (DC-002.76, evidencia dc-002.76-corrida-composite.log): despues de los GCs de la creacion del primer
// jugador, `Records.Animation.Data.GetRecords("AMNUHTH")` devuelve vacio MIENTRAS GetRecordsCount() sigue
// diciendo 2.556. O sea: el mapa sigue "lleno" (el `count` del header sobrevive) pero la busqueda POR CLAVE
// falla.
//
// LA TRAMPA QUE HAY QUE EVITAR (por que la primera version de esta sonda dijo "A OK" con el defecto puesto):
// si el que busca GUARDA las claves que metio en el mapa, esas cadenas siguen vivas por su cuenta y sus bytes
// no se reciclan, asi que la busqueda por clave encuentra lo que quedo apuntando al monton viejo. El caso
// real es al reves: la UNICA referencia a los nombres de AnimData.d2 es el mapa. Por eso aca la busqueda
// REARMA la clave en el momento (`nombre(i)`) y no se guarda ninguna clave viva fuera del mapa.
//
// LAS CINCO CAMAS, para que el numero diga DONDE esta el defecto:
//
//   A  map[string][]*registro con 2.556 claves de 8 letras armadas del monton (el mismo tamano y la misma
//      forma que el mapa de animdata: nombre -> slice de punteros). Es el caso bajo sospecha.
//   B  el mismo mapa con claves CONSTANTES (viven en .rodata: el GC no las toca) y valores del monton. Si A
//      se cae y B no, el que muere es el DATO de la clave guardada en el bucket, no el bucket.
//   C  map[int]int (sin punteros: el GC lo marca NOSCAN). CONTROL del barrido de buckets: si C tambien se
//      cae, lo que no sobrevive es el array de buckets / los desbordes, no las claves.
//   D  map[string]string con clave y valor del monton (el caso mas chico del mismo defecto).
//   E  CONTROL SANO: un mapa armado DESPUES de las recolecciones, para probar que el mapa en si funciona en
//      el heap de ese momento (si E falla, el que esta roto es el mapa, no la recoleccion).
//
// COMO: se fuerza una recoleccion y se mide DESPUES DE CADA UNA (asi se ve si el dano es acumulativo), con
// basura nueva entre una y otra para que el espacio viejo se reutilice de verdad. Las huellas son
// INDEPENDIENTES DEL ORDEN (suma de crc32 por clave: el `range` de Go recorre en orden aleatorio, asi que un
// crc32 del pegado cambiaria solo por el orden y no probaria nada).
//
// La salida va por la consola serie con el prefijo SONDA-MAP, que es lo que lee el corpus.
package main

import (
	"hash/crc32"
	"runtime"
)

//extern d2c_serial_init
func d2cSerialInit()

//extern d2c_ms
func d2cMs() uint

//extern d2c_heap_libre
func d2cHeapLibre() uint

//extern d2c_gc_n
func d2cGcN() uint

//extern d2c_mapa_info
func d2cMapaInfo(p *map[string][]*registro, cuenta *int, buckets *uint, b *int, desbordes *int)

const (
	// cuantas claves distintas tiene el mapa grande. El de animdata tiene 2.556 nombres de 8 letras.
	nClaves = 2556
	// el resto de las camas va chico a proposito: en la consola el arena de C son 4,5 MB y con las cuatro
	// camas grandes el POC moria en `gc_external_alloc: OOM` (medido: ronda 2, arena=4516692 used=4514060)
	nChico = 256
	// recolecciones forzadas: el criterio de aceptacion de la tarjeta pide 3.
	recolecciones = 3
	// basura entre recoleccion y recoleccion: bloques de tamBasura hasta juntar basuraPorRonda bytes. Con
	// bloques chicos (muy por debajo de los 64 KB del camino de objetos grandes) el GC los tira como
	// cualquier objeto, y el espacio viejo se reescribe de verdad.
	tamBasura      = 512
	basuraPorRonda = 8192
	// claves que NUNCA se metieron en el mapa: si una busqueda las encuentra, el mapa esta devolviendo
	// cualquier cosa (falso positivo).
	claveFantasma = 999999
	// claves del control sano E
	nControl = 256
)

type registro struct {
	nombre string
	speed  int
}

var basura []byte

// vivas: un conjunto vivo GRANDE (16 bloques de 6 KB, por debajo del umbral de 64 KB del camino de objetos
// grandes) que se reserva ANTES del mapa y no se suelta nunca. Sirve para que cada recoleccion copie
// ~96 KB al espacio de destino: asi el espacio VIEJO se reescribe de verdad (con un conjunto vivo de 36 KB
// las copias se quedan en el primer tramo y los bytes reciclados del mapa no se tocan nunca, que es lo que
// paso en la primera corrida de esta sonda: A_bien no se movio). Es la basura que el GC COPIA, no la que
// tira.
var vivas [][]byte

// nombre: el nombre de 8 letras tal como lo saca el decodificador de AnimData.d2 (8 bytes -> string del
// monton). Se REARMA en cada busqueda: no se guarda ninguna copia viva fuera del mapa.
func nombre(i int) string {
	b := make([]byte, 8)

	for j := 7; j >= 0; j-- {
		b[j] = byte('A' + (i & 15))
		i >>= 4
	}

	return string(b)
}

// nombreConstante: 16 nombres de .rodata, para el caso B.
func nombreConstante(i int) string {
	switch i % 16 {
	case 0:
		return "AMNUHTH0"
	case 1:
		return "AMNUHTH1"
	case 2:
		return "AMNUHTH2"
	case 3:
		return "AMNUHTH3"
	case 4:
		return "AMNUHTH4"
	case 5:
		return "AMNUHTH5"
	case 6:
		return "AMNUHTH6"
	case 7:
		return "AMNUHTH7"
	case 8:
		return "AMNUHTH8"
	case 9:
		return "AMNUHTH9"
	case 10:
		return "AMNUHTH10"
	case 11:
		return "AMNUHTH11"
	case 12:
		return "AMNUHTH12"
	case 13:
		return "AMNUHTH13"
	case 14:
		return "AMNUHTH14"
	default:
		return "AMNUHTH15"
	}
}

// medirA busca las nClaves claves REARMADAS y cuenta cuantas encuentran el registro que les toca. Ademas
// cuenta los fantasmas (claves que no existen y aparecen).
//
// Se imprime el avance cada nChico claves: si el proceso muere a mitad de la busqueda, la ultima linea
// dice por donde iba (medido: en la ronda 2 se moria en `gc_external_alloc: OOM` con un pedido de 1,1 GB y
// sin saber en que clave; el avance lo contesta).
func medirA(m map[string][]*registro, ronda int) (bien, faltan, rotas, fantasmas int) {
	for i := 0; i < nClaves; i++ {
		if i > 0 && i%nChico == 0 {
			println("SONDA-MAP: avanceA ronda=", ronda, "i=", i, "bien=", bien, "faltan=", faltan, "rotas=", rotas)
		}

		rs := m[nombre(i)]
		if len(rs) == 0 {
			faltan++

			continue
		}

		if len(rs) != 1 || rs[0] == nil || rs[0].speed != i {
			rotas++

			continue
		}

		bien++
	}

	for i := 0; i < 8; i++ {
		if len(m[nombre(claveFantasma+i)]) != 0 {
			fantasmas++
		}
	}

	return bien, faltan, rotas, fantasmas
}

// huellaA: suma de crc32 por entrada (independiente del orden del range). Ademas valida que los bytes de la
// clave que devuelve el range sigan siendo los del nombre que le toca (una clave reciclada devuelve otros
// bytes) y que el registro guardado siga siendo alcanzable.
func huellaA(m map[string][]*registro, conNombre bool) (claves, valores, clavesRotas, valoresRotas int, hClaves, hValores uint32) {
	for k, v := range m {
		claves++

		if len(k) > 64 {
			// un largo imposible: los bytes de la clave estan reciclados. NO se convierte a []byte (eso
			// pide un buffer del largo que diga la basura y el proceso muere con un OOM ajeno al defecto).
			clavesRotas++

			continue
		}

		hClaves += crc32.ChecksumIEEE([]byte(k))

		if conNombre && !esNombre(k) {
			clavesRotas++
		}

		if len(v) == 0 || v[0] == nil {
			continue
		}

		valores++

		if len(v[0].nombre) > 64 {
			valoresRotas++

			continue
		}

		hValores += crc32.ChecksumIEEE([]byte(v[0].nombre))

		if conNombre && !esNombre(v[0].nombre) {
			valoresRotas++
		}
	}

	return claves, valores, clavesRotas, valoresRotas, hClaves, hValores
}

// esNombre: dice si la cadena tiene la forma de un nombre de esta sonda (8 letras entre A y P). Si el GC
// reciclo los bytes, esto es lo que lo delata barato.
func esNombre(s string) bool {
	if len(s) != 8 {
		return false
	}

	for i := 0; i < len(s); i++ {
		if s[i] < 'A' || s[i] > 'P' {
			return false
		}
	}

	return true
}

// forzar: una recoleccion, con basura nueva antes para que el espacio viejo tenga algo encima.
func forzar() {
	for i := 0; i < basuraPorRonda; i += tamBasura {
		basura = make([]byte, tamBasura)
		basura[0] = byte(i)
		basura[tamBasura-1] = byte(i)
	}

	runtime.GC()
}

func main() {
	d2cSerialInit()

	println("SONDA-MAP: arranca (DC-002.83, el mapa de animdata y el GC)")
	println("SONDA-MAP: runtime", runtime.GOOS, runtime.GOARCH)

	t0 := int64(d2cMs())

	// primero el conjunto vivo grande: ocupa el principio del semiespacio, asi que los nombres del mapa
	// quedan detras y las copias de las recolecciones los pisan en la segunda vuelta.
	for i := 0; i < 16; i++ {
		b := make([]byte, 6144)
		b[0] = byte(i)
		b[len(b)-1] = byte(i)
		vivas = append(vivas, b)
	}

	// --- A: 2.556 claves del monton, valores del monton (la forma del mapa de animdata) ---
	mA := make(map[string][]*registro)

	for i := 0; i < nClaves; i++ {
		mA[nombre(i)] = []*registro{{nombre: nombre(i), speed: i}}
	}

	// --- B: claves CONSTANTES (.rodata), valores del monton ---
	mB := make(map[string][]*registro)

	for i := 0; i < nChico; i++ {
		k := nombreConstante(i)
		mB[k] = append(mB[k], &registro{nombre: "AMNUHTHF", speed: i})
	}

	// --- C: map[int]int (sin punteros) ---
	mC := make(map[int]int)

	for i := 0; i < nChico; i++ {
		mC[i] = i * 3
	}

	// --- D: map[string]string (clave y valor del monton) ---
	mD := make(map[string]string)

	for i := 0; i < nChico; i++ {
		mD[nombre(i)] = nombre(i)
	}

	println("SONDA-MAP: armado claves=", nClaves, "ms=", int64(d2cMs())-t0, "libre=", int(d2cHeapLibre()))

	informe(0, mA, mB, mC, mD)

	// --- las recolecciones, midiendo despues de cada una ---
	for r := 1; r <= recolecciones; r++ {
		antes := int64(d2cMs())
		forzar()
		println("SONDA-MAP: ronda=", r, "gc_ms=", int64(d2cMs())-antes, "libre=", int(d2cHeapLibre()))
		informe(r, mA, mB, mC, mD)
	}

	// --- E: CONTROL SANO. Un mapa armado DESPUES de las recolecciones: tiene que andar. ---
	t1 := int64(d2cMs())
	mE := make(map[string][]*registro)

	for i := 0; i < nControl; i++ {
		mE[nombre(i)] = []*registro{{nombre: nombre(i), speed: i}}
	}

	bienE := 0

	for i := 0; i < nControl; i++ {
		rs := mE[nombre(i)]
		if len(rs) == 1 && rs[0] != nil && rs[0].speed == i {
			bienE++
		}
	}

	println("SONDA-MAP: E (armado despues del GC) bien=", bienE, "de", nControl, "ms=", int64(d2cMs())-t1)

	println("SONDA-MAP: fin ms=", int64(d2cMs())-t0)
}

// informe imprime una foto de las cuatro camas.
func informe(ronda int, mA, mB map[string][]*registro, mC map[int]int, mD map[string]string) {
	// LA CABECERA PRIMERO. Si el GC reciclo el header del mapa (count o B basura), las busquedas de abajo
	// piden un array de buckets con un tamano de otro planeta y el proceso muere en `gc_external_alloc:
	// OOM` sin decir nada (medido: ronda 2, sbrk de 1,1 GB). Ver la cabecera antes permite ver el dano en
	// vez de comerse el OOM.
	var cta, bA, desb int
	var bks uint
	d2cMapaInfo(&mA, &cta, &bks, &bA, &desb)
	println("SONDA-MAP: ronda=", ronda, "gcTotal=", int(d2cGcN()), "formaA count=", cta, "buckets=0x",
		int64(bks), "B=", bA, "desbordes=", desb)

	if cta != nClaves || bA < 0 || bA > 12 {
		println("SONDA-MAP: ronda=", ronda, "A CABECERA CORRUPTA (count=", cta, "B=", bA,
			"): no se busca, se salta la cama A")

		informeBCD(ronda, mB, mC, mD)

		return
	}

	bienA, faltanA, rotasA, fantA := medirA(mA, ronda)
	cA, vA, crA, vrA, hcA, hvA := huellaA(mA, true)

	println("SONDA-MAP: ronda=", ronda,
		"A_bien=", bienA, "A_faltan=", faltanA, "A_rotas=", rotasA, "A_fantasmas=", fantA,
		"A_claves=", cA, "A_valores=", vA, "A_clavesRotas=", crA, "A_valoresRotas=", vrA,
		"A_hc=", int64(hcA), "A_hv=", int64(hvA))

	informeBCD(ronda, mB, mC, mD)
}

// informeBCD: las camas chicas (claves constantes, int y string->string).
func informeBCD(ronda int, mB map[string][]*registro, mC map[int]int, mD map[string]string) {
	cB, vB, _, _, hcB, hvB := huellaA(mB, false)
	bienB := 0

	for i := 0; i < nChico; i++ {
		rs := mB[nombreConstante(i)]
		if len(rs) > 0 && rs[0] != nil && rs[0].speed == i {
			bienB++
		}
	}

	println("SONDA-MAP: ronda=", ronda,
		"B_bien=", bienB, "B_claves=", cB, "B_valores=", vB, "B_hc=", int64(hcB), "B_hv=", int64(hvB))

	sumaC := 0
	faltanC := 0
	rotasC := 0

	for i := 0; i < nChico; i++ {
		v := mC[i]
		if v == 0 {
			faltanC++

			continue
		}

		if v != i*3 {
			rotasC++

			continue
		}

		sumaC += v
	}

	println("SONDA-MAP: ronda=", ronda, "C_bien=", nChico-faltanC-rotasC, "C_faltan=", faltanC,
		"C_rotas=", rotasC, "C_suma=", sumaC)

	bienD := 0
	faltanD := 0

	for i := 0; i < nChico; i++ {
		v := mD[nombre(i)]
		if v == "" {
			faltanD++

			continue
		}

		if !esNombre(v) {
			continue
		}

		bienD++
	}

	println("SONDA-MAP: ronda=", ronda, "D_bien=", bienD, "D_faltan=", faltanD)
}

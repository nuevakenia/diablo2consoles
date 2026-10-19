//go:build dreamcast

// DC-002.39e: EL CACHE DE ITABS GUARDA PUNTEROS DEL MONTON DE GO EN UN ARRAY ESTATICO DE C QUE EL
// RECOLECTOR NO BARRE.
//
// QUE MIDE ESTE POC (y por que es chiquito): la corrida de la consulta muere con `PC 656a626f` (texto
// "obje" leido como direccion) y el `PR` cae en `bytes.Buffer.ReadFrom`, que hace `r.Read(bloque)` -- una
// llamada de METODO POR INTERFAZ, o sea un salto por un ITAB. El runtime de la consola arma esos itabs en
// `get_itab` (runtime/interface_dreamcast.c) y los guarda en `itab_cache`, que es
// `static struct { Itab *itabs[32]; int count; }` -- un array en `.bss` de C.
//
// EL MECANISMO (leido en el runtime, no supuesto): el recolector de la consola es copiativo (Cheney, dos
// semiespacios) y sus raices son SOLO tres cosas (`gc_scan_roots` de runtime/gc_copy.c):
//   1. `gc_root_table` (`gc_add_root`), que nadie del runtime usa salvo sus propios tests;
//   2. `gc_scan_compiler_roots` (las listas que registra gccgo para las variables GLOBALES de Go);
//   3. las pilas de las goroutines (`gc_scan_stack`).
// El comentario de gc_copy.c lo dice explicito: "Root scanning uses compiler-provided gcdata (precise), not
// conservative .data/.bss scan. gc_scan_globals() was removed". O sea que `.bss` de C NO SE BARRE. Un
// `Itab *` guardado ahi es invisible: cuando la recoleccion COPIA ese itab al otro semiespacio, la ranura
// del cache sigue apuntando al semiespacio VIEJO (que se reusa como basura), y la proxima llamada de metodo
// salta por una tabla que ya es de otro objeto -- exactamente el sintoma medido (PC = texto del archivo).
//
// COMO LO MIDE, SIN DEPENDER DEL MOTOR: dos tipos que implementan la misma interfaz, y una conversion
// (que es lo que arma y cachea el itab). Despues se fuerzan recolecciones que COPIAN ese itab (reservando
// basura) y se vuelve a llamar el metodo por la MISMA variable de interfaz.
//
//   * Si el itab sobrevivio: el metodo devuelve el numero que le toca y `correcto` sube.
//   * Si el cache quedo viejo: el salto va a memoria reciclada. Se ve de dos formas, las dos medidas aca:
//     el proceso se cae con un PC que NO es codigo (el sintoma del motor), o el metodo devuelve un numero
//     que no corresponde (`malo` sube) porque la tabla apunta a otra funcion.
//
// LECTURA DEL RESULTADO:
//   ITAB-RAIZ ... llamadas= N correcto= N malo= 0   -> el cache esta bien (o el itab no se movio)
//   ITAB-RAIZ ... malo> 0 o una excepcion del SH-4 -> el cache quedo viejo: es el defecto
//
// La consola serie se abre en csrc/cshim.c (en Go no se puede: `dbgio_dev_select`). El `ebiten` se importa
// por el ENLACE, no por el dibujo: es el que arrastra el paquete `kos` de libgodc, y sin el el binario
// muere con `Unhandled exception` antes de imprimir una linea (medido en DC-002.33).
package main

import (
	"runtime"

	"github.com/hajimehoshi/ebiten/v2"
)

//extern d2c_serial_init
func d2cSerialInit()

//extern d2c_monton_libre
func d2cMontonLibre() int

//extern d2c_ms
func d2cMs() int

//extern d2c_gc_cuenta
func d2cGcCuenta() int

//extern d2c_itab_llamadas_cuenta
func d2cItabLlamadas() int

//extern d2c_itab_armados_cuenta
func d2cItabArmados() int

//extern d2c_itab_ultimo_ptr
func d2cItabUltimo() uintptr

//extern d2c_semi_activo
func d2cSemiBase() uintptr

//extern d2c_semi_activo_fin
func d2cSemiFin() uintptr

const (
	vueltas      = 40        // cuantas recolecciones se fuerzan
	trabajoBytes = 12 * 1024 // basura por vuelta: garantiza que el itab se COPIE de semiespacio
	trozo        = 48 * 1024 // basura retenida que se rota (el semiespacio viejo se reusa de verdad)
	ranuras      = 6
	maxReportes  = 8 // no mas de 8 lineas de fallo (el log tiene que seguir siendo legible)
)

// saludador: la interfaz por la que se llama (el equivalente a `io.Reader` en el camino del MPQ).
type saludador interface {
	Saluda() int
}

// uno y dos: dos tipos concretos con el MISMO metodo. Cada par (interfaz, tipo) arma SU itab.
type uno struct{ n int }

func (u *uno) Saluda() int { return 1000 + u.n }

type dos struct{ n int }

func (d *dos) Saluda() int { return 2000 + d.n }

// suma: acumula lo que devuelven las llamadas para que el compilador no se lleve nada puesto.
var suma int

// trabajo: basura con contenido (no se optimiza) y un valor de vuelta.
func trabajo(n int) int {
	b := make([]byte, n)
	for i := 0; i < n; i += 256 {
		b[i] = byte(i)
	}

	return len(b) + int(b[0])
}

// itabEnSemi: dice si el ultimo itab armado cae ADENTRO del semiespacio activo del GC. Si cae adentro, el
// recolector lo COPIA cuando recolecta y el puntero guardado en el cache (que vive en .bss de C) queda
// viejo; si cae afuera, el itab es de otro lado y el cache no puede quedarse viejo por ese motivo.
func itabEnSemi() bool {
	p := d2cItabUltimo()

	return p >= d2cSemiBase() && p < d2cSemiFin()
}

// porCache: EL CAMINO DEL CACHE, aislado en una funcion que NO se puede inline-ar.
//
// LA CLAVE ES QUE SEA UNA CONVERSION ENTRE DOS INTERFACES, no de un tipo concreto a una interfaz: cuando el
// tipo concreto se conoce en la compilacion, gccgo arma el itab ESTATICO en `.rodata` y no pasa por el cache
// (medido: la primera version de este POC convertia `*uno` y daba 80/80 correcto con 27 recolecciones, o sea
// que no estaba midiendo el cache). Con DOS interfaces el compilador no puede saber el tipo de atras y emite
// una llamada al runtime (`ifaceI2I2` / `ifaceI2E2`), que es donde vive `get_itab` y su `itab_cache`.
//
// ES EXACTAMENTE LO QUE HACE EL MOTOR: `bytes.Buffer.ReadFrom` declara su parametro como la interfaz ANONIMA
// `interface{ Read([]byte) (int, error) }` y en `d2mpq/mpq_stream.go:deflate` le llega un `io.ReadCloser`
// (el que devuelve `zlib.NewReader`). Interfaz a interfaz -> `get_itab` -> cache.
//
// otro: la interfaz ANONIMA por la que se convierte (dos interfaces: es lo que hace `bytes.Buffer.ReadFrom`
// con su parametro `interface{ Read([]byte) (int, error) }`).
type otro interface{ Saluda() int }

// porEstatico: EL CONTROL DE V2. Misma estructura (una interfaz que queda viva en un slice a traves de las
// recolecciones), pero con una conversion de TIPO CONCRETO a interfaz: gccgo la resuelve con un itab
// ESTATICO en `.rodata`, que el recolector no mueve nunca. Si V4 sobrevive y V2 no, la diferencia es el
// CACHE (y no "una interfaz viva en un slice").
//
//go:noinline
func porEstatico(u *uno) otro {
	var s otro = u

	return s
}

// porCache: EL CAMINO DEL CACHE, aislado en una funcion que NO se inline-a. La conversion `saludador` ->
// `otro` (dos interfaces) no se puede resolver en compilacion: emite una llamada al runtime (`ifaceI2I2`),
// que es donde vive `get_itab` y su `itab_cache`.
//
//go:noinline
func porCache(entra saludador) otro {
	var s otro = entra

	return s
}

func main() {
	d2cSerialInit()

	println("ITAB-RAIZ arranca (DC-002.39e: el cache de itabs guarda punteros que el GC no ve)")
	println("ITAB-RAIZ montonC=", d2cMontonLibre(), "gc=", d2cGcCuenta(), "ebitenLibre=", ebiten.MemLibreDelMonton())
	println("ITAB-RAIZ semiespacio=", d2cSemiBase(), "..", d2cSemiFin(),
		"itabLlamadas=", d2cItabLlamadas(), "itabArmados=", d2cItabArmados(), "itabUltimo=", d2cItabUltimo())

	// La conversion arma el itab y lo deja EN EL CACHE del runtime. OJO: esto SOLO arma la entrada; la
	// llamada `a.Saluda()` de mas abajo usa el itab que esta GUARDADO EN LA VARIABLE `a`, que vive en la
	// pila (y la pila SI se barre). El camino del cache se golpea con una CONVERSION NUEVA, que es lo que
	// hace `bytes.Buffer.ReadFrom`: su parametro es una interfaz ANONIMA (`interface{ Read(...) }`), asi
	// que pasarle el `io.ReadCloser` que devuelve `zlib.NewReader` es una conversion por vuelta ->
	// `get_itab` -> cache. Medido en la primera version de este POC: con la variable de interfaz sola, el
	// defecto NO se reproduce (80/80 correcto con 27 recolecciones), porque el itab de la pila se mantiene
	// bien. El cache es otra cosa y hay que golpearlo aparte.
	// Dos variables de interfaz en la PILA (su itab lo mantiene el barrido del stack) y, aparte, el camino
	// del CACHE: una conversion NUEVA por vuelta, que es lo que hace `bytes.Buffer.ReadFrom` con su
	// parametro `interface{ Read([]byte) (int, error) }` (el `io.ReadCloser` del zlib se convierte ahi).
	var a saludador = &uno{n: 7}
	var b saludador = &dos{n: 9}

	println("ITAB-RAIZ armados a=", a.Saluda(), "b=", b.Saluda(), "gc=", d2cGcCuenta())

	arranque := d2cMs()
	basura := make([][]byte, ranuras)
	correcto := 0
	malo := 0
	llamadas := 0

	// V2: las interfaces que salen del CACHE y quedan VIVAS a traves de las recolecciones.
	vivos := make([]otro, 0, vueltas)
	v2Bien := 0
	v2Mal := 0
	// V4: el CONTROL de V2 -- las mismas, pero con itab ESTATICO (.rodata). Tiene que sobrevivir siempre.
	estaticos := make([]otro, 0, vueltas)
	v4Bien := 0
	v4Mal := 0
	reportes := 0

	u0 := &uno{n: 7}

	for v := 1; v <= vueltas; v++ {
		// basura que se retiene y se rota: obliga al recolector a COPIAR lo vivo (los itabs incluidos)
		basura[v%ranuras] = make([]byte, trozo)
		suma += trabajo(trabajoBytes)

		// (1) la llamada por la variable de la PILA (control: esto andaba en la version anterior)
		ra := a.Saluda()
		rb := b.Saluda()
		llamadas += 2

		if ra == 1007 {
			correcto++
		} else {
			malo++
			println("ITAB-RAIZ-MALO v=", v, "a=", ra, "esperado= 1007")
		}
		if rb == 2009 {
			correcto++
		} else {
			malo++
			println("ITAB-RAIZ-MALO v=", v, "b=", rb, "esperado= 2009")
		}

		// (2) EL CAMINO DEL CACHE, en dos tiempos. Primero se ARMA la interfaz por el cache (una conversion
		// entre dos interfaces por vuelta) y se deja VIVA en un slice: el GC barre ese slice con gcdata
		// precisa, o sea que la ranura entra a la recoleccion como un puntero mas. Segundo, en la vuelta
		// siguiente, se llama el metodo de TODAS las que quedaron vivas: si el itab del cache apunta al
		// semiespacio viejo, la recoleccion reubica ese puntero como INTERIOR y la tabla de metodos queda
		// siendo la de otro objeto -> el salto va a un pedazo de dato.
		vivos = append(vivos, porCache(a))
		llamadas++

		for i, s := range vivos {
			if r := s.Saluda(); r == 1007 {
				v2Bien++
			} else {
				v2Mal++
				if reportes < maxReportes {
					reportes++
					println("ITAB-RAIZ-MALO-V2 guardadaEn=", i, "v=", v, "r=", r, "esperado= 1007")
				}
			}
		}

		// ---- V4 (CONTROL de V2): lo mismo, con el itab ESTATICO en .rodata ----
		estaticos = append(estaticos, porEstatico(u0))

		for i, s := range estaticos {
			if r := s.Saluda(); r == 1007 {
				v4Bien++
			} else {
				v4Mal++
				if reportes < maxReportes {
					reportes++
					println("ITAB-RAIZ-MALO-V4 guardadaEn=", i, "v=", v, "r=", r, "esperado= 1007")
				}
			}
		}

		if v%4 == 0 {
			runtime.GC()
		}
		// progreso: las TRES primeras vueltas y despues cada 8. Las primeras van SIEMPRE porque la corrida
		// del defecto muere en la primera vuelta (medido: 0 lineas de progreso con v%8): sin esto no se
		// puede decir DONDE se murio.
		if v <= 3 || v%8 == 0 {
			println("ITAB-RAIZ v=", v, "llamadas=", llamadas, "correcto=", correcto, "malo=", malo,
				"v2bien=", v2Bien, "v2mal=", v2Mal, "vivos=", len(vivos),
				"v4bien=", v4Bien, "v4mal=", v4Mal,
				"gc=", d2cGcCuenta(), "ms=", d2cMs()-arranque, "libreC=", d2cMontonLibre(),
				"itabLlamadas=", d2cItabLlamadas(), "itabArmados=", d2cItabArmados(),
				"itabUltimo=", d2cItabUltimo(), "itabEnSemi=", itabEnSemi())
		}
	}

	println("ITAB-RAIZ-TOTAL vueltas=", vueltas, "llamadas=", llamadas, "correcto=", correcto, "malo=", malo,
		"v2bien=", v2Bien, "v2mal=", v2Mal, "vivos=", len(vivos),
		"v4bien=", v4Bien, "v4mal=", v4Mal, "estaticos=", len(estaticos),
		"gc=", d2cGcCuenta(), "suma=", suma, "ms=", d2cMs()-arranque, "libreC=", d2cMontonLibre(),
		"ebitenLibre=", ebiten.MemLibreDelMonton())
}

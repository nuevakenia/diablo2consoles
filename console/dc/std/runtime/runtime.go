// Paquete runtime de la consola: lo que el motor pregunta del entorno.
//
// REGLA: NO SE INVENTAN NUMEROS. Si un dato no existe en la consola, se devuelve cero o el valor
// declarado, y queda escrito acá y en la evidencia. Lo que si se sabe:
//
//   - GOOS: la consola no es ninguna de las que Go conoce. Se declara "dreamcast" (y queda dicho:
//     si el motor compara contra "linux" o "windows" para elegir una rama, va a tomar la rama que no
//     es de ninguno de los dos, que es lo que corresponde en la consola).
//   - GOARCH: "sh4" (el procesador es un SH-4, big endian, 32 bits).
//   - NumCPU: 1. La Dreamcast tiene UN SH-4 de un solo nucleo. Esto no es una limitacion del
//     emulador: es asi en el hardware.
//   - GOMAXPROCS: hay UN hilo de KOS corriendo las goroutines (medido en el scheduler de libgodc).
//     GOMAXPROCS devuelve 1 siempre, y guarda lo que le pidan para devolverlo si se consulta.
//   - Gosched: NO HACE NADA, y es lo correcto acá. El scheduler es cooperativo: una goroutine cede
//     cuando se bloquea en un canal (medido: thd_pass no releva goroutines, un canal con buffer que
//     no se llena tampoco, uno sin buffer si). Un Gosched "que no hace nada" es mas honesto que uno
//     que finja ceder el hilo.
//   - MemStats y GC: el runtime de la consola tiene su recolector (libgodc/runtime/gc_*), pero NO
//     lleva las cuentas de Go ni tiene una entrada para pedir una recoleccion. Se devuelven ceros y
//     se declara: el motor que las use para loguear va a loguear ceros, no numeros inventados.
//   - LockOSThread: no hace nada (las goroutines ya corren todas sobre el mismo hilo de KOS).
//   - GC y NumGoroutine NO SE ESCRIBEN ACÁ: se LLAMAN al runtime de libgodc, que los tiene de verdad
//     (medido al enlazar: gc_runtime.c define _runtime.GC y proc.c define _runtime.NumGoroutine; al
//     escribirlos acá el enlace fallo por "multiple definition"). Ojo: llamar al GC en la consola
//     para el mundo y puede tardar; el motor lo hace una sola vez, medido.
package runtime

const (
	// GOOS y GOARCH de la consola.
	GOOS   = "dreamcast"
	GOARCH = "sh4"
	// Compiler: el motor lo usa para elegir ramas.
	Compiler = "gccgo"
	// Version: la de Go que trae el gccgo del toolchain (medida: gccgo 15.1).
	Version = "go1.24"
)

// MemStats: los campos que el motor consulta, todos en cero con su por que (ver el encabezado).
type MemStats struct {
	Alloc         uint64
	TotalAlloc    uint64
	Sys           uint64
	Lookups       uint64
	Mallocs       uint64
	Frees         uint64
	HeapAlloc     uint64
	HeapSys       uint64
	HeapIdle      uint64
	HeapInuse     uint64
	HeapReleased  uint64
	HeapObjects   uint64
	StackInuse    uint64
	StackSys      uint64
	MSpanInuse    uint64
	MSpanSys      uint64
	MCacheInuse   uint64
	MCacheSys     uint64
	GCSys         uint64
	OtherSys      uint64
	NextGC        uint64
	LastGC        uint64
	PauseTotalNs  uint64
	NumGC         uint32
	NumForcedGC   uint32
	GCCPUFraction float64
	EnableGC      bool
	DebugGC       bool
	BySize        [61]struct {
		Size    uint32
		Mallocs uint64
		Frees   uint64
	}
}

// NumCPU: uno. La Dreamcast tiene un SH-4 de un solo nucleo.
func NumCPU() int { return 1 }

// GOMAXPROCS: en la consola hay un hilo de KOS corriendo las goroutines (medido). Devuelve siempre 1,
// y guarda lo que le pidan para devolverlo si se vuelve a consultar.
var gomaxprocs = 1

func GOMAXPROCS(n int) int {
	viejo := gomaxprocs
	if n > 0 {
		gomaxprocs = n
	}
	return viejo
}

// Gosched: no hace nada (ver el encabezado: el scheduler es cooperativo y cede en los canales).
func Gosched() {}

// GC y NumGoroutine SE LLAMAN al runtime de libgodc, que los tiene de verdad (medido al enlazar:
// gc_runtime.c define _runtime.GC y proc.c define _runtime.NumGoroutine). Van por un puente de C
// (csrc/runtime_dc.c) porque el nombre del simbolo lleva un punto, que no es un identificador valido
// ni para //extern ni para una funcion de Go.

// GC pide la recoleccion al runtime de la consola.
//
// OJO, esto costo un rato y hay que dejarlo dicho: estas dos NO se pueden ESCRIBIR en este paquete
// (con cuerpo). gccgo nombra los simbolos "runtime.GC", igual que el runtime de libgodc, y el enlace
// muere por "multiple definition". Se DECLARAN sin cuerpo (//extern) y el puente de C las resuelve
// contra el runtime: asi el motor puede seguir llamando runtime.GC() y runtime.NumGoroutine() como
// en Go, sin que este paquete las redefina.
//
//extern d2c_gc
func GC()

// NumGoroutine: las goroutines vivas que lleva el runtime.
//
//extern d2c_numgoroutine
func NumGoroutine() int

// ReadMemStats: ceros (ver el encabezado).
func ReadMemStats(m *MemStats) {
	if m != nil {
		*m = MemStats{}
	}
}

// LockOSThread / UnlockOSThread: no hacen nada (todas las goroutines corren sobre el mismo hilo).
func LockOSThread()   {}
func UnlockOSThread() {}

// SetFinalizer: no esta. El runtime de la consola no tiene finalizadores (medido: no hay
// runtime.SetFinalizer en libgodc). Si el motor lo pide, se ve como tarjeta aparte.

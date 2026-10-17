// Prueba diferencial de DC-002.7: d2std/sync contra el sync de Go.
//
// QUE SE PUEDE COMPARAR EN EL HOST Y QUE NO:
//   - d2std/sync SI: es Go puro (canales), asi que corre igual en el host y se compara contra sync de Go.
//   - d2std/sync/atomic NO: sus operaciones son shims de C con irq_disable (KOS), asi que no existen en
//     el host. Esa parte se prueba en la CONSOLA, con el POC de goroutines (35 casos).
//   - d2std/runtime NO por lo mismo (d2c_gc / d2c_numgoroutine).
//
// La comparacion es de COMPORTAMIENTO: se corre el mismo escenario con el candado de Go y con el de la
// consola, y se exige el mismo resultado. Es lo que el motor va a hacer con ellos.
package difftest

import (
	gosync "sync"
	"sync/atomic"
	"testing"

	d2sync "d2std/sync"
)

// preparar: crea el canal del candado ANTES de que lo usen varias goroutines, y LO EXIGE.
//
// POR QUE HACE FALTA (medido, DC-002.8b): el canal del candado se crea en el primer uso. En la consola
// no hay carrera posible, porque el scheduler es cooperativo sobre un hilo (medido en el POC de
// goroutines). En el host, en cambio, dos goroutines pueden crear DOS canales distintos: cada una se
// queda con su testigo, una suelta un candado que la otra no ve y una de las dos se cuelga en el Unlock.
// Paso de verdad: "go test -count=20 ./difftest/" se colgo 1 vez en 40 corridas, con la goroutine clavada
// en RWMutex.RUnlock -> Mutex.Unlock -> <-m.libre (sonda instrumentada: 2 cuelgues en 300 rondas de 16
// goroutines, y hasta 2 canales creados para el mismo candado).
//
// POR QUE ADEMAS LO EXIGE: el cuelgue es rarisimo (1 en 40 corridas, o sea 1 en ~800 ejecuciones del
// test), asi que "correrlo muchas veces" no lo caza y un test que se cuelga cada tanto se lee como "fue
// la maquina". Con la exigencia de Preparado(), un candado sin preparar falla en el acto y con su
// nombre. El mutante de scripts/console/mutar-cuelgue-386.sh es precisamente un candado sin preparar.
func preparar(t *testing.T, l interface {
	Lock()
	Unlock()
	Preparado() bool
}) {
	t.Helper()
	l.Lock()
	l.Unlock()
	if !l.Preparado() {
		t.Fatal("el candado quedo SIN preparar (limite declarado en d2std/sync): en el host hay que crear su canal antes de usarlo en paralelo, o se cuelga cada tanto en el Unlock")
	}
}

// escenario corre 8 goroutines sumando 2000 veces cada una sobre un contador protegido.
func escenario(locker interface {
	Lock()
	Unlock()
}) int {
	var n int
	var wg gosync.WaitGroup
	wg.Add(8)
	for i := 0; i < 8; i++ {
		go func() {
			for j := 0; j < 2000; j++ {
				locker.Lock()
				n++
				locker.Unlock()
			}
			wg.Done()
		}()
	}
	wg.Wait()
	return n
}

// TestMutexContraGo: el candado de la consola y el de Go tienen que dar el mismo resultado exacto.
func TestMutexContraGo(t *testing.T) {
	var mGo gosync.Mutex
	var mDC d2sync.Mutex
	preparar(t, &mDC)
	if a, b := escenario(&mGo), escenario(&mDC); a != b {
		t.Fatalf("Mutex: Go dio %d y la consola %d", a, b)
	}
	if escenario(&mDC) != 16000 {
		t.Fatalf("Mutex de la consola: el contador tiene que dar 16000 exacto")
	}
}

// TestRWMutexContraGo: el RWMutex de la consola es exclusivo (declarado), asi que se compara solo el
// caso que en Go tambien es exclusivo: Lock/Unlock.
func TestRWMutexContraGo(t *testing.T) {
	var rwGo gosync.RWMutex
	var rwDC d2sync.RWMutex
	preparar(t, &rwDC)
	if a, b := escenario(&rwGo), escenario(&rwDC); a != b {
		t.Fatalf("RWMutex: Go dio %d y la consola %d", a, b)
	}
	// dos lectores juntos: en Go entran los dos, en la consola (scheduler cooperativo, declarado) el
	// RLock es exclusivo. Lo que SI tiene que coincidir es que ninguno se traba y que el valor no se
	// corrompe.
	var nGo, nDC int
	var wgGo, wgDC gosync.WaitGroup
	var mGo gosync.RWMutex
	var mDC d2sync.RWMutex
	// ESTE es el candado que el test se olvidaba de preparar (DC-002.8b): el preparar() de arriba es
	// para rwDC, y el bloque de abajo usa mDC, que nace con el canal sin crear. Preparado (y exigido)
	// aca, la carrera de creacion no existe y el cuelgue de 1-en-40-corridas no puede aparecer.
	preparar(t, &mDC)
	wgGo.Add(2)
	wgDC.Add(2)
	for i := 0; i < 2; i++ {
		go func() { mGo.RLock(); nGo++; mGo.RUnlock(); wgGo.Done() }()
		go func() { mDC.RLock(); nDC++; mDC.RUnlock(); wgDC.Done() }()
	}
	wgGo.Wait()
	wgDC.Wait()
	if nGo != nDC || nDC != 2 {
		t.Fatalf("RLock: Go %d, consola %d (los dos tienen que terminar en 2)", nGo, nDC)
	}
}

// TestOnceContraGo: la funcion corre UNA sola vez, y Do no vuelve hasta que termino.
//
// POR QUE NO VA CON GOROUTINES REALES ACA: el candado de Once se crea perezosamente (no hay
// constructor) y en el host dos goroutines paralelas pueden crear dos canales distintos, con lo que el
// test se cuelga. En la consola eso no puede pasar (scheduler cooperativo, medido) y ahi SI se corre
// con goroutines: el POC console/dc/poc/goroutines tiene el caso 6 con 4 goroutines usando Do.
func TestOnceContraGo(t *testing.T) {
	correr := func(do func(func())) int {
		veces := 0
		do(func() { veces++ })
		do(func() { veces++ })
		do(func() { veces++ })
		return veces
	}
	var once d2sync.Once
	var oGo gosync.Once
	a := correr(oGo.Do)
	b := correr(once.Do)
	if a != 1 || b != 1 {
		t.Fatalf("Once: Go %d, consola %d (los dos tienen que dar 1)", a, b)
	}
}

// TestOnceEsperaAlQueCorre: el que llega despues ve la funcion TERMINADA (no arrancada).
func TestOnceEsperaAlQueCorre(t *testing.T) {
	var once d2sync.Once
	terminado := false
	once.Do(func() { terminado = true })
	if !terminado {
		t.Fatal("Once: Do no espero a que la funcion terminara")
	}
	corrio := false
	once.Do(func() { corrio = true })
	if corrio {
		t.Fatal("Once: la segunda llamada no tiene que correr la funcion")
	}
}

// TestWaitGroupContraGo: el contador de la consola y el de Go esperan igual.
//
// ARREGLADO en DC-002.8 (era un test roto de dos maneras, y mentia):
//   - el contador `n` lo incrementaban DIEZ goroutines con n++ sin sincronizar: carrera de datos.
//     Medido: 2 de 30 corridas daban 29 en vez de 30 (go test -run TestWaitGroupContraGo -count=30).
//     Un test flaky no solo hace fallar la suite al azar: hace que el MUTADOR cace mutantes por
//     casualidad. El mutante del RGBA de DC-002.8 "cazo" por esto la primera vez que se corrio.
//   - devolvia `n, n`: el MISMO contador dos veces, asi que el mensaje "WaitGroup: Go 29, consola 29"
//     no comparaba nada (y el parametro usarConsola quedaba ignorado con un `_ = usarConsola`).
//
// Ahora cada lado tiene SU contador (atomic, que en 386 el int64 necesita alineacion y el int32 no) y
// se exige 30 en los dos.
func TestWaitGroupContraGo(t *testing.T) {
	correr := func(usarConsola bool) int {
		var n int32
		for i := 0; i < 3; i++ {
			var wgGo gosync.WaitGroup
			var wgDC d2sync.WaitGroup
			if usarConsola {
				wgDC.Add(5)
				for j := 0; j < 5; j++ {
					go func() {
						atomic.AddInt32(&n, 1)
						wgDC.Done()
					}()
				}
				wgDC.Wait()
				continue
			}
			wgGo.Add(5)
			for j := 0; j < 5; j++ {
				go func() {
					atomic.AddInt32(&n, 1)
					wgGo.Done()
				}()
			}
			wgGo.Wait()
		}
		return int(n)
	}
	// 15 incrementos por lado: 3 vueltas de 5 goroutines en cada uno (el test viejo esperaba 30 porque
	// los DOS lados sumaban sobre el mismo contador)
	a, b := correr(false), correr(true)
	if a != 15 || b != 15 {
		t.Fatalf("WaitGroup: Go %d, consola %d (los dos tienen que dar 15)", a, b)
	}
}

// TestPoolContraGo: el Pool guarda y devuelve objetos.
func TestPoolContraGo(t *testing.T) {
	var p d2sync.Pool
	creados := 0
	p.New = func() interface{} {
		creados++
		return creados
	}
	a := p.Get()
	if a == nil {
		t.Fatal("Pool: Get sin nada guardado tiene que llamar a New")
	}
	p.Put(a)
	b := p.Get()
	if b != a {
		t.Fatalf("Pool: lo guardado y lo devuelto tienen que ser el mismo (%v y %v)", a, b)
	}
	if creados != 1 {
		t.Fatalf("Pool: New tiene que haberse llamado una vez, se llamo %d", creados)
	}
	p.Put(a)
	p.Put(nil) // nil no se guarda, como Go
	if c := p.Get(); c != a {
		t.Fatalf("Pool: despues de Put(nil) el pool tiene que seguir dando el mismo objeto (dio %v)", c)
	}
}

// TestTryLockContraGo: TryLock dice lo mismo en las dos.
func TestTryLockContraGo(t *testing.T) {
	var mGo gosync.Mutex
	var mDC d2sync.Mutex
	if mGo.TryLock() != mDC.TryLock() {
		t.Fatal("TryLock: con el candado libre los dos tienen que entrar")
	}
	if mGo.TryLock() != mDC.TryLock() {
		t.Fatal("TryLock: con el candado tomado los dos tienen que decir que no")
	}
	mGo.Unlock()
	mDC.Unlock()
	if mGo.TryLock() != mDC.TryLock() {
		t.Fatal("TryLock: despues de soltar los dos tienen que entrar")
	}
}

// TestLimitesConocidos: los dos limites declarados de d2std/sync, vigilados para que no mueran en silencio.
func TestLimitesConocidos(t *testing.T) {
	// 1. Unlock de un candado que nunca se tomo: la consola lo ignora y Go NO.
	//
	// OJO, esto esta medido: el lado de Go no se puede ejecutar en un test, porque no es un panico
	// recuperable con recover: el sync de Go llama a internal/sync.fatal y ABORTA el proceso (medido:
	// "internal/sync.(*Mutex).unlockSlow ... fatal" y el paquete murio con FAIL). Asi que el limite se
	// declara aca y se ejecuta solo el lado de la consola.
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Fatal("el candado de la consola NO tiene que entrar en panico al soltar un candado libre (es su limite declarado)")
			}
		}()
		var m d2sync.Mutex
		m.Unlock() // no hace nada
	}()

	// 2. RLock recursivo: NO se puede ejecutar en ninguno de los dos, porque los DOS se traban. Acá
	//    quedo medido: al quitar el fatal del lado de Go, este test se colgó, y el culpable era mi
	//    RWMutex exclusivo (RLock = Lock sobre un canal con buffer 1: el segundo RLock se bloquea).
	//    O sea que en esto la consola se comporta IGUAL que Go con un Lock esperando: se traba. La
	//    declaracion vieja ("en la consola no se traba") estaba MAL y la caza fue este test.
	t.Log("RLock recursivo: se traba en los dos (medido). Declarado en sync.go, no se ejecuta.")
}

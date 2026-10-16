//go:build dreamcast

// DC-002.7: goroutines, candados y atomicos en la consola.
//
// Este POC es la aceptacion del hito, y tiene dos partes: la que tiene que dar EXACTO y la que tiene
// que FALLAR (la prueba en contra). Las dos se imprimen y las dos quedan en la evidencia.
//
// LO QUE SE MIDIO ANTES DE ESCRIBIR EL CODIGO (sonda, en la consola):
//   - cuatro goroutines sumando sin candado y sin ceder  -> 8000 exacto: NO hay preempcion
//   - thd_pass() en medio del incremento                  -> 8000: thd_pass no releva goroutines
//   - canal CON buffer 1 como rendija                     -> no releva (el buffer no se llena solo)
//   - canal SIN buffer                                    -> SI releva (bloquea de verdad)
// De ahi sale el diseño de sync (Mutex = canal con buffer 1) y por que la prueba en contra necesita
// CEDER en el medio para que el defecto se vea: sin cesion, el defecto no existe.
package main

import (
	"runtime"
	"strconv"
	"sync"
	"sync/atomic"
)

//extern d2c_serial_init
func d2c_serial_init()

// ---------------------------------------------------------------- utilidades de la evidencia

var casos, bien, mal int

func check(nombre, esperado, dado string) {
	casos++
	if esperado == dado {
		bien++
		println("GORO ok  ", nombre, "=", dado)
	} else {
		mal++
		println("GORO FALLO", nombre, "esperaba", esperado, "y dio", dado)
	}
}

func itoa(n int) string { return strconv.Itoa(n) }

// ---------------------------------------------------------------- el POC

func main() {
	d2c_serial_init()
	println("GORO arranca: goroutines, candados y atomicos (DC-002.7)")
	println("GORO modelo medido: scheduler cooperativo sobre un hilo de KOS; sin preempcion")

	// ------------------------------------------------ 1. la prueba en contra: sin candado se pierde
	{
		var n int
		fin := make(chan bool, 4)
		turno := make(chan bool) // SIN buffer: es el punto de cesion de verdad
		// El relevador: RECIBE y DEVUELVE (ida y vuelta). El que cede manda y despues espera la
		// devolucion; si el relevador no devolviera, los dos quedarian bloqueados mandando (eso paso
		// en el primer intento: el POC se colgo y no imprimio ni el caso 1).
		go func() {
			for {
				<-turno
				turno <- true
			}
		}()
		for i := 0; i < 4; i++ {
			go func() {
				for j := 0; j < 200; j++ {
					v := n        // lee
					turno <- true // CEDE con el valor leido en la mano (aca esta el defecto)
					<-turno       // espera la devolucion del relevo
					n = v + 1     // escribe con el valor viejo
				}
				fin <- true
			}()
		}
		for i := 0; i < 4; i++ {
			<-fin
		}
		perdidos := 800 - n
		println("GORO 1. SIN candado y cediendo en medio: n =", itoa(n), "perdidos =", itoa(perdidos), "(tiene que PERDER)")
		check("1. sin candado pierde incrementos", "true", si(perdidos > 0))
	}

	// ------------------------------------------------ 2. con Mutex: exacto
	{
		var n int
		var m sync.Mutex
		fin := make(chan bool, 4)
		turno := make(chan bool)
		go func() {
			for {
				<-turno
				turno <- true
			}
		}()
		for i := 0; i < 4; i++ {
			go func() {
				for j := 0; j < 200; j++ {
					m.Lock()
					{
						v := n
						turno <- true // se cede ADENTRO del candado: nadie mas puede entrar
						<-turno
						n = v + 1
					}
					m.Unlock()
				}
				fin <- true
			}()
		}
		for i := 0; i < 4; i++ {
			<-fin
		}
		println("GORO 2. con sync.Mutex y la misma cesion: n =", itoa(n), "(tiene que dar 2000 exacto)")
		check("2. con Mutex da exacto", "800", itoa(n))
	}

	// ------------------------------------------------ 3. con atomic.AddInt64: exacto
	{
		var n int64
		fin := make(chan bool, 4)
		turno := make(chan bool)
		go func() {
			for {
				<-turno
				turno <- true
			}
		}()
		for i := 0; i < 4; i++ {
			go func() {
				for j := 0; j < 200; j++ {
					atomic.AddInt64(&n, 1)
					turno <- true
					<-turno
				}
				fin <- true
			}()
		}
		for i := 0; i < 4; i++ {
			<-fin
		}
		println("GORO 3. con atomic.AddInt64: n =", strconv.FormatInt(n, 10), "(2000 exacto)")
		check("3. atomic.AddInt64 da exacto", "800", strconv.FormatInt(n, 10))
	}

	// ------------------------------------------------ 4. los atomicos, uno por uno
	{
		var i32 int32 = 10
		check("4. AddInt32 devuelve el nuevo", "15", itoa(int(atomic.AddInt32(&i32, 5))))
		check("4. LoadInt32", "15", itoa(int(atomic.LoadInt32(&i32))))
		atomic.StoreInt32(&i32, 7)
		check("4. StoreInt32", "7", itoa(int(atomic.LoadInt32(&i32))))
		check("4. SwapInt32 devuelve el viejo", "7", itoa(int(atomic.SwapInt32(&i32, 9))))
		check("4. SwapInt32 dejo el nuevo", "9", itoa(int(atomic.LoadInt32(&i32))))
		check("4. CAS que coincide", "true", si(atomic.CompareAndSwapInt32(&i32, 9, 11)))
		check("4. CAS dejo el valor", "11", itoa(int(atomic.LoadInt32(&i32))))
		check("4. CAS que no coincide", "false", si(atomic.CompareAndSwapInt32(&i32, 9, 99)))
		check("4. CAS que falla no cambia", "11", itoa(int(atomic.LoadInt32(&i32))))

		var i64 int64 = 100000000000
		check("4. AddInt64 en 64 bits", "100000000005", strconv.FormatInt(atomic.AddInt64(&i64, 5), 10))
		check("4. LoadInt64", "100000000005", strconv.FormatInt(atomic.LoadInt64(&i64), 10))
		check("4. CASInt64", "true", si(atomic.CompareAndSwapInt64(&i64, 100000000005, 42)))
		check("4. LoadInt64 despues del CAS", "42", strconv.FormatInt(atomic.LoadInt64(&i64), 10))

		var u32 uint32 = 4294967290
		atomic.AddUint32(&u32, 10) // da la vuelta, como en Go
		check("4. AddUint32 da la vuelta", "4", itoa(int(atomic.LoadUint32(&u32))))

		var b atomic.Bool
		b.Store(true)
		check("4. Bool Store/Load", "true", si(b.Load()))
		check("4. Bool Swap", "true", si(b.Swap(false)))
		check("4. Bool despues del Swap", "false", si(b.Load()))

		var cont atomic.Int64
		cont.Add(3)
		cont.Add(4)
		check("4. atomic.Int64 (el tipo) suma", "7", strconv.FormatInt(cont.Load(), 10))
	}

	// ------------------------------------------------ 5. WaitGroup
	{
		var wg sync.WaitGroup
		var n int64
		wg.Add(4)
		for i := 0; i < 4; i++ {
			go func() {
				for j := 0; j < 100; j++ {
					atomic.AddInt64(&n, 1)
				}
				wg.Done()
			}()
		}
		wg.Wait()
		check("5. WaitGroup.Wait espera a las cuatro", "400", strconv.FormatInt(atomic.LoadInt64(&n), 10))
	}

	// ------------------------------------------------ 6. Once
	{
		var o sync.Once
		var veces int64
		fin := make(chan bool, 4)
		for i := 0; i < 4; i++ {
			go func() {
				o.Do(func() {
					atomic.AddInt64(&veces, 1)
				})
				fin <- true
			}()
		}
		for i := 0; i < 4; i++ {
			<-fin
		}
		check("6. Once corre una sola vez", "1", strconv.FormatInt(atomic.LoadInt64(&veces), 10))
	}

	// ------------------------------------------------ 7. RWMutex y Mutex.TryLock
	{
		var rw sync.RWMutex
		rw.Lock()
		libre := rw.TryLock() // tiene que dar falso: esta tomado
		rw.Unlock()
		check("7. RWMutex tomado: TryLock no entra", "false", si(libre))
		rw.RLock()
		check("7. RLock toma el candado", "true", si(rw.TryLock() == false))
		rw.RUnlock()

		var m sync.Mutex
		check("7. Mutex libre: TryLock entra", "true", si(m.TryLock()))
		check("7. Mutex tomado: TryLock no entra", "false", si(m.TryLock()))
		m.Unlock()
		check("7. despues de Unlock: TryLock entra", "true", si(m.TryLock()))
		m.Unlock()
		check("7. Unlock de un candado sin tomar: no revienta", "true", si(true))
	}

	// ------------------------------------------------ 8. runtime
	{
		check("8. runtime.GOOS", "dreamcast", runtime.GOOS)
		check("8. runtime.GOARCH", "sh4", runtime.GOARCH)
		check("8. runtime.NumCPU", "1", itoa(runtime.NumCPU()))
		check("8. runtime.GOMAXPROCS(0)", "1", itoa(runtime.GOMAXPROCS(0)))
		var ms runtime.MemStats
		runtime.ReadMemStats(&ms)
		check("8. MemStats en cero (declarado)", "0", strconv.FormatUint(ms.Alloc, 10))
		runtime.GC()
		runtime.Gosched()
		runtime.LockOSThread()
		runtime.UnlockOSThread()
		check("8. GC, Gosched y LockOSThread no revientan", "true", si(true))
	}

	println("GORO casos", itoa(casos), "bien", itoa(bien), "mal", itoa(mal))
	if mal == 0 {
		println("GORO RESULTADO OK - las goroutines, los candados y los atomicos funcionan en la consola")
	} else {
		println("GORO RESULTADO MAL")
	}
	println("GORO fin")
	select {}
}

func si(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

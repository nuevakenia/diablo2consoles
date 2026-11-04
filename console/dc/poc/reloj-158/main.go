//go:build dreamcast

// DC-002.158 -- LOS DOS RELOJES DE LA CONSOLA, EN EL MISMO LAZO.
//
// QUE MIDE. El hallazgo de DC-002.157: sobre el MISMO cuadro (el 11 del Gate 0), el reloj del PVR dijo
// 879 ms (el `Draw` entero) y `d2util.Now()` dijo 1.781 ms (el `Render` del mapa, que es un subconjunto
// del `Draw`). Un subconjunto no puede durar mas que el todo: uno de los dos relojes miente.
//
// Este POC pone los dos relojes en el MISMO lazo y mide el mismo intervalo con cada uno:
//
//   - el timer crudo de KOS: `timer_us_gettime64()` (via el puente `d2c_reloj_us` de la stdlib de la
//     consola, que es el MISMO que usa `d2c_pvr_pantalla_us` y por eso el que se sabe bueno);
//   - el reloj de Go de la consola: `time.Now()` (console/dc/std/time/time.go) y la conversion EXACTA de
//     `d2common/d2util/timeutils.go:16`, `float64(time.Now().UnixNano()) / 1e9`.
//
// La razon entre los dos deltas es el numero de la tarjeta: 1000 por mil es un reloj sano; ~2000 por mil
// es el defecto (adelanta 2x).
//
// POR QUE LA CONVERSION A float64 SE REPLICA Y NO SE IMPORTA d2util. Importar `d2common/d2util` trae el
// paquete entero (logger, sus dependencias y sus .gox del motor) a un POC que solo necesita una linea de
// aritmetica; y la linea es copia literal de timeutils.go. El numero que importa -- el de la pantalla del
// Gate 0 -- se confirma despues con el binario del motor (la corrida del Gate 0 con el `CUADRO n ms=`).
//
// LAS ESPERAS SON VARIAS Y CORTAS A PROPOSITO: la de 1000 ms es la que pide la tarjeta; las de 150, 300 y
// 700 existen para cazar un reloj que pierde los subsegundos (un `UnixNano` que devuelva solo los
// segundos pasaria la prueba de 1 s y fallaria estas).
package main

import "time"

//extern d2c_reloj_us
func d2c_reloj_us() uint64

//extern d2c_reloj_unix
func d2c_reloj_unix() int64

//extern d2c_serial_init
func d2c_serial_init()

//extern d2c_dormir
func d2c_dormir(ms int)

// ahoraFloat es la conversion de d2common/d2util/timeutils.go:16, copiada literal.
func ahoraFloat() float64 { return float64(time.Now().UnixNano()) / 1000000000.0 }

// esperasMs: los tramos del lazo. El 1000 se repite tres veces porque es el criterio de la tarjeta.
var esperasMs = []int{1000, 1000, 300, 150, 700, 1000, 2000}

func main() {
	d2c_serial_init()

	println("RELOJ158: los dos relojes de la consola en el mismo lazo")
	println("RELOJ158: crudo = timer_us_gettime64() (d2c_reloj_us); now = time.Now()/d2util.Now();")
	println("RELOJ158: razon_mil = now_us * 1000 / crudo_us (1000 = sano, 2000 = adelanta 2x)")

	total := 0
	peor := int64(0)
	for i, ms := range esperasMs {
		us0 := d2c_reloj_us()
		rtc0 := d2c_reloj_unix()
		t0 := time.Now()
		f0 := ahoraFloat()

		d2c_dormir(ms)

		us1 := d2c_reloj_us()
		rtc1 := d2c_reloj_unix()
		t1 := time.Now()
		f1 := ahoraFloat()

		crudoUs := int64(us1 - us0)
		nowUs := int64((f1 - f0) * 1000000.0)
		nanoUs := (t1.UnixNano() - t0.UnixNano()) / 1000
		subMs := t1.Sub(t0).Milliseconds()

		razon := int64(0)
		if crudoUs > 0 {
			razon = nowUs * 1000 / crudoUs
		}
		desvio := razon - 1000
		if desvio < 0 {
			desvio = -desvio
		}
		if desvio > peor {
			peor = desvio
		}
		if desvio > 10 {
			total++
		}

		println("RELOJ158: VUELTA", i+1, "espera_ms=", ms, "crudo_us=", crudoUs,
			"now_us=", nowUs, "unixnano_us=", nanoUs, "sub_ms=", subMs,
			"rtc_s=", rtc1-rtc0, "razon_mil=", razon)
	}

	println("RELOJ158: RESUMEN vueltas=", len(esperasMs), "malas=", total, "peor_desvio_mil=", peor)
	if total == 0 {
		println("RELOJ158: VERDE el reloj de la consola sigue al timer de KOS (dentro del 1 % = 10 por mil)")
	} else {
		println("RELOJ158: ROJO el reloj de la consola NO sigue al timer de KOS")
	}

	// el emulador se mata desde afuera: el programa se queda quieto para que la corrida no se corte sola
	for {
		d2c_dormir(1000)
	}
}

package main

// DC-002.146c -- LA PALANCA DEL POC MINIMO DEL BLINDAJE.
//
//	modo     blindaje | viejo   -- "blindaje" es el arreglo (`runtime_blindaje` de libgodc); "viejo" es la
//	                               CONTRA-PRUEBA: el `runtime_checkpoint()` de siempre, que hace `setjmp`
//	                               en una funcion de C que ya retorno. Con "viejo" el POC tiene que morir
//	                               con `kernel panic: unhandled IRQ/Exception` (o `FATAL: recover without
//	                               checkpoint` si el checkpoint no llega a existir): eso aisla la causa.
//	vueltas  3                  -- cuantas veces se repite el ciclo panic -> recover -> seguir. Tres
//	                               alcanzan para ver que el camino queda usable despues de volver y que
//	                               las cadenas del G no se llenan de marcos muertos (`defer overflow`).
//
// Se pisa desde el build: BLINDAJE=viejo bash scripts/console/build-blindaje-146c.sh [salida]. El valor
// con el que se compilo queda escrito en el knobs.go del directorio de build (el binario dice cual es).
const (
	modo    = "blindaje"
	vueltas = 3
)

//go:build dreamcast

// DC-002.146c -- EL BLINDAJE: el `recover` de libgodc que SI devuelve el control.
//
// QUE MIDE. En DC-002.146 quedo medido que el `defer func(){ if r := recover(); ... }()` del rearme del
// minimapa deja el log y despues NO vuelve: `runtime_checkpoint()` hace `setjmp` en un marco de C que ya
// retorno, asi que el `longjmp` del recover salta a un marco muerto (`kernel panic: unhandled
// IRQ/Exception` con checkpoint, `FATAL: recover without checkpoint` sin el). Este POC es el caso minimo
// de la tarjeta: un programa de consola, `runtime_blindaje` de libgodc, un panic de rebanada de verdad
// ADENTRO del cuerpo, y tres vueltas. Lo que la consola serie imprima es el dato:
//
//     el panic se imprime (el valor que recupera el defer),
//     el control VUELVE al blindaje (devuelve 1),
//     y despues SIGUE imprimiendo.
//
// LAS TRES VUELTAS NO SON RELLENO. La 0 y la 2 usan el cuerpo pelado (un solo defer: el que recupera).
// La 1 usa un cuerpo con un defer HUERFANO: uno registrado ANTES del que recupera, o sea uno que NO
// llega a correr porque el recover vuelve al blindaje. Ese huerfano queda en la cadena del G apuntando a
// un marco que ya no existe: si `runtime_blindaje` no lo saca, la cadena se llena de marcos muertos y el
// `defer overflow` llega antes de tiempo. El defer de `main` que se ve al final es la otra mitad de la
// prueba: la cadena quedo sana y un defer de verdad sigue corriendo.
//
// CON BLINDAJE=viejo el mismo POC arma el camino roto (el `runtime_checkpoint()` de siempre, en el mismo
// lugar donde el proyecto lo usaba): el control NO vuelve y el SH-4 se va a basura. Ese es el control en
// contra, o sea: la causa aislada en un binario de 40 lineas y sin nada del juego de por medio.
//
// EL ABI (lo que la tarjeta manda probar antes). A C se le pasa la funcion de Go en una variable de
// PAQUETE (`caja`). gccgo no le pasa el codigo: le pasa un puntero a la CAJA del valor de funcion, cuyo
// primer campo ES el codigo; `runtime_blindaje_go` la abre y llama (ver defer_dreamcast.c). La variable
// de paquete no es un adorno: C no es raiz del recolector, asi que un cierre con capturas podria
// reciclarse mientras el cuerpo corre. Con una funcion sin capturas la caja vive en .rodata, pero se
// deja igual como ejemplo de lo que hay que hacer.
package main

//extern runtime_blindaje_go
func blindajeGo(cuerpo func(uintptr) int, arg uintptr) int

//extern d2c_checkpoint_viejo
func d2cCheckpointViejo() int

//extern d2c_serial_init
func d2cSerialInit()

// cuerpoBlindado: el cuerpo que corre ADENTRO del blindaje. Registra su `defer` con `recover` (el unico
// que puede llevarse el panic) y se cae con un panic de REBANADA de verdad, calculado: la casilla que
// lee depende del argumento, asi que no se puede doblar en tiempo de compilacion.
//
// Lo que devuelve (7) no se usa: cuando hay panic, el cuerpo NO vuelve por donde volveria (libgodc no
// desenrolla la pila: se vuelve por el longjmp al blindaje).
func cuerpoBlindado(arg uintptr) int {
	defer func() {
		if r := recover(); r != nil {
			// `println` de una interface vacia: gccgo llama a `runtime.printeface`, que en la consola
			// imprime el TIPO y el valor (para un texto: `(string,"index out of range")`). No se usa una
			// asercion `r.(string)` a proposito: mete `runtime..types` en el enlace y libgodc no lo tiene
			// (medido: `undefined reference to 'runtime..types'`).
			println("BLINDAJE146C: PANICO RECOGIDO en el defer, el valor es:", r)
		} else {
			println("BLINDAJE146C: el defer corrio y NO habia panic: eso seria un defecto del POC")
		}
	}()

	println("BLINDAJE146C: dentro del cuerpo, vuelta=", int(arg))

	rebanada := []int{10, 20, 30}
	i := len(rebanada) + int(arg)
	println("BLINDAJE146C: leyendo la casilla", i, "de una rebanada de", len(rebanada))
	println("BLINDAJE146C: ESTO NO TIENE QUE SALIR: la casilla vale", rebanada[i])

	return 7
}

// cuerpoConHuerfano: el mismo cuerpo, pero con un defer registrado ANTES del que recupera. Ese defer NO
// corre (el recover devuelve el control al blindaje, y su marco ya no existe): si `runtime_blindaje` no
// lo saca de la cadena del G, queda ahi para siempre.
func cuerpoConHuerfano(arg uintptr) int {
	defer func() {
		println("BLINDAJE146C: DEFECTO: corrio el defer huerfano, cuyo marco ya no existe")
	}()
	defer func() {
		if r := recover(); r != nil {
			println("BLINDAJE146C: PANICO RECOGIDO (cuerpo con defer huerfano), el valor es:", r)
		}
	}()

	println("BLINDAJE146C: dentro del cuerpo con huerfano, vuelta=", int(arg))

	rebanada := []int{10, 20, 30}
	i := len(rebanada) + int(arg)
	println("BLINDAJE146C: leyendo la casilla", i, "de una rebanada de", len(rebanada))
	println("BLINDAJE146C: ESTO NO TIENE QUE SALIR: la casilla vale", rebanada[i])

	return 8
}

// las cajas de los valores de funcion: ver el comentario de arriba (C no es raiz del recolector)
var (
	caja         = cuerpoBlindado
	cajaHuerfana = cuerpoConHuerfano
)

func main() {
	d2cSerialInit()

	// si la cadena de defers quedo sana despues de los blindajes, este corre al final
	defer func() {
		println("BLINDAJE146C: el defer de main corrio al final: la cadena de defers quedo sana")
	}()

	println("BLINDAJE146C: arranca modo=", modo, "vueltas=", vueltas)

	for v := 0; v < vueltas; v++ {
		println("BLINDAJE146C: --- vuelta", v, "antes del blindaje ---")

		codigo := 0
		if modo == "viejo" {
			// la contra-prueba: el checkpoint de siempre, en una funcion de C que RETORNA
			codigo = d2cCheckpointViejo()
			if codigo == 0 {
				cuerpoBlindado(0)
			}
		} else if v%2 == 1 {
			codigo = blindajeGo(cajaHuerfana, uintptr(v))
		} else {
			codigo = blindajeGo(caja, uintptr(v))
		}

		println("BLINDAJE146C: el blindaje devolvio", codigo, "(0 = el cuerpo volvio solo, 1 = se volvio por el panic)")
		println("BLINDAJE146C: SIGUE IMPRIMIENDO despues del panic, vuelta", v)
	}

	println("BLINDAJE146C: CIERRE OK las", vueltas, "vueltas volvieron y la consola sigue viva")
}

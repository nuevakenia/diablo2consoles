//go:build dreamcast

package pvr

// DC-002.135 -- LA PRESENTACION PARTIDA EN TRES (conversion / VRAM / pintado) Y LAS DOS PALANCAS.
//
// EL PROBLEMA QUE ABRE ESTA TARJETA. DC-002.132 midio el cuadro del POC por dentro y encontro un numero
// que manda sobre todo lo demas: la presentacion cuesta 36,8 ms por cuadro, CONSTANTES con el blit de
// tiles viejo y con el nuevo (36.759 / 36.758 / 37.000 / 36.840 us en cuatro tramos de dos corridas).
// Con eso, el techo son 27 cuadros por segundo aunque el dibujado sea gratis: cualquier mejora del mapa
// o de los sprites se pierde contra ese piso. Pero la linea `PVR132: CUADRO` daba el total de la
// presentacion y nada mas, asi que no se sabia cual de los tres pedazos -- convertir los 307.200 pixeles
// de RGBA a RGB565 en la CPU, subirlos a la VRAM por las Store Queues, o pintar el quad -- es el que se
// lleva el tiempo. Adivinar cual es lo que no se puede hacer.
//
// QUE HAY ACA, ENTONCES. Dos cosas, y las dos son del backend de C (`pvr_pantalla.c`), no de Go:
//
//	UsDeLaConversion / UsDeLaSubida / BloquesPresentados / SubidasPresentadas
//	    los cronometros del ultimo cuadro presentado. El de C parte cada bloque de 32x32 en dos marcas
//	    (bucle de pixeles y pvr_txr_load) y las escala a los bloques reales del cuadro; el PINTADO no
//	    hace falta medirlo en C: el ciclo del Renderer lo hace solo con una marca antes y otra despues de
//	    `PintarPantalla()`.
//
//	FijarPresentarConvRapida / FijarPresentarVramAgrupada
//	    las dos palancas del camino nuevo (una carga de 32 bits por pixel y dos pixeles por deposito; y
//	    la subida en grupos de 2x2 bloques = 8.192 B por copia en vez de 2.048 B por bloque). Con las dos
//	    apagadas el binario corre EXACTAMENTE el camino de DC-002.132: eso es el A/B, en el mismo
//	    binario, sin depender de dos compilaciones distintas.
//
// LO QUE ESTAS PALANCAS NO PUEDEN CAMBIAR, Y ES EL CRITERIO DE LA TARJETA: los bytes que llegan a la
// textura. La cuenta del color es la misma (5-6-5 truncado) y el orden del twiddle tambien, asi que la
// captura del pueblo tiene que salir identica pixel a pixel a la de DC-002.132 (/tmp/capturas-132-b2) y
// el crc32 del lienzo tiene que seguir en 1437022778. Si eso no se cumple, el numero de microsegundos no
// vale nada.

// Las dos palancas, en el lado de Go: la verdad la tiene el backend de C (es el que convierte), pero el
// ciclo del Renderer tambien necesita saber con cual corrio el binario para decirlo en la linea del
// desglose (una medicion sin eso no se puede comparar con otra).
var (
	convRapida   bool
	vramAgrupada bool
)

// FijarPresentarConvRapida enciende (o apaga) el bucle de conversion rapido: una carga de 32 bits por
// pixel y dos pixeles por deposito, con la tabla de desplazamientos del twiddle armada una sola vez.
func FijarPresentarConvRapida(v bool) {
	convRapida = v

	if v {
		d2cPvrPresConvRapida(1)

		return
	}

	d2cPvrPresConvRapida(0)
}

// FijarPresentarVramAgrupada enciende (o apaga) la subida agrupada: los cuatro bloques de un grupo de
// 2x2 (64x64 px) son CONTIGUOS en la textura twiddleada, asi que van en una copia de 8.192 B en vez de
// cuatro de 2.048 B.
func FijarPresentarVramAgrupada(v bool) {
	vramAgrupada = v

	if v {
		d2cPvrPresVramAgrupada(1)

		return
	}

	d2cPvrPresVramAgrupada(0)
}

// UsDeLaConversion: cuanto costo, en microsegundos, convertir el lienzo a RGB565 en la ULTIMA
// presentacion (el bucle de pixeles, sin la subida).
func UsDeLaConversion() int { return d2cPvrPresConvUs() }

// UsDeLaSubida: cuanto costo, en microsegundos, subir los bloques a la VRAM en la ultima presentacion
// (las llamadas a pvr_txr_load, sin la conversion).
func UsDeLaSubida() int { return d2cPvrPresVramUs() }

// BloquesPresentados: cuantos bloques de 32x32 se convirtieron en la ULTIMA presentacion (300 para un
// cuadro de 640x480: 20 x 15; tambien 300 en el split de 2 jugadores, porque son dos lienzos de 150). Es el
// control de que el camino midio lo que dice medir.
func BloquesPresentados() int { return d2cPvrPresBloques() }

// SubidasPresentadas: cuantas copias a la VRAM se hicieron en la ultima presentacion. Con la subida
// agrupada son 90 (70 grupos de 2x2 = 280 bloques en 70 copias de 8.192 B, mas los 20 bloques sueltos de la
// franja de abajo en 20 copias de 2.048 B: 480 = 7x64 + 32, y esa franja no tiene par vertical ni
// horizontal que agrupar); sin ella, 300. Es el numero que dice si la palanca hizo lo que dice.
func SubidasPresentadas() int { return d2cPvrPresSubidas() }

// ReiniciarLaPresentacion: los contadores de la presentacion a cero. Lo llama el ciclo del cuadro (renderer.go)
// JUSTO ANTES de presentar, y no lo hace la conversion sola a proposito: un cuadro puede presentarse en VARIAS
// llamadas (el split de DC-002.116/118 manda dos lienzos de 320x480, uno por jugador, y el de 4 manda cuatro), y
// si cada llamada reiniciara los contadores, `PVR135: PRESENTAR` informaria solo el ultimo lienzo: conv de 150
// bloques, subidas de 45 y un `pintar` que se come la diferencia. Con esto, la linea informa el cuadro entero.
func ReiniciarLaPresentacion() { d2cPvrPresReiniciar() }

// CostoDelRelojUs1000: cuanto cuesta llamar al reloj 1.000 veces, medido en la consola. La presentacion
// se parte en tres con dos marcas por bloque muestreado, asi que este numero es el que hay que descontar
// del desglose para saber cuanto es trabajo y cuanto es instrumento.
func CostoDelRelojUs1000() int { return d2cPvrRelojCosto() }

//go:build dreamcast

// DC-002.114 -- DOS JUGADORES EN LA MISMA PANTALLA: LA MEDICION QUE DECIDE EL COOPERATIVO.
//
// QUE MIDE ESTE ARCHIVO. El cooperativo local de Diablo II es hasta cuatro jugadores en la misma pantalla, y la
// pregunta de esta tarjeta es si la Dreamcast puede con DOS. Lo que hay que medir no es "se ve lindo": es
//
//  1. la MEMORIA: cuantos bytes cuesta el segundo jugador. La respuesta de este POC es concreta y esta medida en el
//     log: los cuadros compuestos (que son los `.dcc` decodificados y pegados, cuadro por cuadro) se COMPARTEN, asi
//     que el segundo jugador cuesta 0 B de assets; lo que es POR JUGADOR es su estado (posicion del motor,
//     direccion, reloj de la animacion) y su paleta. El log imprime las dos cosas: los bytes de los cuadros
//     compartidos y los del estado por jugador.
//  2. los CUADROS PINTADOS y los MS POR CUADRO con 1 y con 2 jugadores: el mismo binario, con `JUGADORES=1` y con
//     `JUGADORES=2` (la palanca de knobs.go). La diferencia entre las dos corridas es el segundo jugador y nada mas.
//  3. el SEMIESPACIO del recolector antes y despues: es el muro medido en DC-002.111/.113.
//  4. que el input de un jugador NO mueve al otro: el guion de abajo tiene un tramo SOLO-P1, uno SOLO-P2 y uno con
//     los dos, y cada tramo imprime los ticks y las celdas de LOS DOS.
//
// DE DONDE SALE CADA COSA (nada elegido a ojo):
//
//   - EL PUERTO: cada jugador lee SU puerto del bus maple (`d2c_control_leer_puerto`, con `maple_enum_dev`). El
//     jugador 1 va en el puerto 0 (el A: es el que Flycast mapea al teclado del host) y el jugador 2 en el 1 (el B).
//   - EL INPUT: el MISMO camino que DC-002.113 -- el `Servicio` del backend de la consola (`d2core/d2input/maple`)
//     traduce los bits del control a las teclas del motor, y el POC pregunta por las teclas. Cada jugador tiene su
//     propio `Servicio`, asi que los estados no se pisan.
//   - EL MOVIMIENTO: `movimiento` (movimiento.go) con la velocidad del exe y los ticks de 25 Hz del motor: es el
//     MISMO tipo y la MISMA cuenta para los dos jugadores.
//   - EL ANCLA Y LA CAJA: `AnclaDelJugador` y `CajaDe` (jugador.go, el archivo compartido con el medidor del host).
//   - EL ORDEN DE DIBUJO entre los dos: por profundidad, como las entidades del motor -- primero el que esta mas
//     arriba en el mapa (menor x+y en celdas), que es el que queda DETRAS. Con un solo jugador no hay orden que
//     elegir.
//   - LA CAMARA: el CENTRO de los jugadores (con uno solo es su propia celda, asi que el cuadro 1 sigue siendo el de
//     DC-002.112: ancla 320,283 y crc32 1363871674).
//
// LO QUE ESTE ARCHIVO NO HACE: split screen (es otra tarjeta: primero los dos en la misma pantalla), colision
// contra las paredes, ni 3 y 4 jugadores (hasta que 2 funcione con numeros).
package main

import (
	"hash/crc32"
	"unsafe"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2interface"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2math/d2vector"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2input/maple"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2render/pvr"
)

// Los bytes del estado de un jugador: el `movimiento` (posicion, direccion, relojes y contadores) mas el `Estado`
// (la posicion en subtiles del motor). Se miden con `unsafe.Sizeof` y no se escriben a mano: si el estado crece, el
// numero que imprime el POC crece con el.
var (
	tamanoDeMovimiento = int(unsafe.Sizeof(movimiento{}))
	tamanoDeEstado     = int(unsafe.Sizeof(Estado{}))

	// DC-002.117: y la ESTRUCTURA del participante, que es lo que de verdad se pide por jugador al armar el
	// cooperativo (lleva la paleta de 1.024 B y los punteros a su servicio del maple). No se escribe a mano: si el
	// struct crece, el numero crece con el, y es el que se compara contra el semiespacio del recolector.
	tamanoDeParticipante = int(unsafe.Sizeof(participante{}))
)

// crc32De: el hash del lienzo (el mismo criterio de todas las tarjetas anteriores).
func crc32De(pixeles []byte) uint32 { return crc32.ChecksumIEEE(pixeles) }

// itoa: un entero a texto. La consola no lleva el paquete `strconv` completo para dos digitos, y `println` ya separa
// los argumentos con espacios, asi que esto es solo para armar las etiquetas del log.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}

	negativo := n < 0
	if negativo {
		n = -n
	}

	var buf [12]byte

	i := len(buf)

	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}

	if negativo {
		i--
		buf[i] = '-'
	}

	return string(buf[i:])
}

// participante: UN jugador del cooperativo. Lo que es POR JUGADOR es su control, su posicion, su reloj de
// animacion y su paleta; los CUADROS COMPUESTOS no (se comparten: `ComponerCicloDelJugador` ya dejo cada cuadro en
// indices de 8 bits y los dos jugadores leen de ahi).
type participante struct {
	Nombre   string
	Puerto   int
	Servicio *maple.Servicio
	Entrada  d2interface.InputService
	Mov      *movimiento
	Pal      [256][4]uint8

	// Inyecta: en su puerto NO hay aparato y el estado lo pone el guion del shim. Se decide al arrancar, mirando
	// el bus, y se imprime: un jugador con guion no es un jugador con control.
	Inyecta bool

	// lo del cuadro que se esta pintando (para la evidencia)
	EjeX, EjeY     float64
	Botones        uint32
	Ticks          int
	Caminado       float64
	AnclaX, AnclaY int
	Caja           CajaDelJugador
	CajaGuardada   []byte
	CuadroDelCiclo int
	Cambiados      int
	MsDibujo       int

	// DC-002.115 -- lo del LEASH: donde arranco el paso de este cuadro (para saber cuanto camino de VERDAD
	// despues de que el leash lo freno), cuantas veces lo freno y cuantas celdas se le devolvieron, la distancia
	// a la que quedo del otro, la caja SIN recortar y si esa caja entro en el lienzo (el numero del criterio).
	IniX, IniY     float64
	LeashRecortes  int
	LeashCeldas    float64
	LeashDistancia float64
	CajaCruda      CajaDelJugador
	Dentro         bool
}

// jugadores: los del cooperativo, en orden. `jugadores[0].Mov` ES `mov` (el jugador 1), para que todo lo que mide
// DC-002.113 (los tramos, el resumen, el cierre) siga midiendo lo mismo.
var jugadores []*participante

// prepararJugadores: arma el jugador 1 (el de siempre, con su control del puerto 0) y, si la palanca lo pide, el
// jugador 2 (puerto 1). Imprime de cada puerto del bus lo que hay, que es el instrumento que separa "hay un control"
// de "el estado lo pone el guion".
//
// LOS CUADROS NO SE COMPONEN DE NUEVO: `compartirJugador2` es "si" y el jugador 2 usa los ciclos del jugador 1. Lo
// que costaria NO compartirlos se imprime igual (`bytesDeLosCuadros`), porque es el numero que dice cuanto pesa el
// camino caro.
func prepararJugadores() {
	if mov == nil {
		return
	}

	// CUANTOS: la palanca, con el tope en los cuatro puertos del bus maple. Se dice cuando se recorta (un
	// binario que dice 7 y hace 4 seria una mentira del instrumento).
	cuantos := jugadoresKnob
	if cuantos < 1 {
		cuantos = 1
	}

	if cuantos > maxJugadoresCoop {
		println("JUEGO117: COOP AVISO jugadores=", jugadoresKnob, "es mas que los", maxJugadoresCoop,
			"puertos del bus maple: se usan", maxJugadoresCoop)

		cuantos = maxJugadoresCoop
	}

	jugadores = make([]*participante, 0, maxJugadoresCoop)

	// CADA JUGADOR: su puerto, su celda de arranque y SU servicio del maple. El 1 ES `mov` (el de DC-002.113),
	// asi que con `JUGADORES=1` esto es el camino de siempre con un solo participante.
	for i := 0; i < cuantos; i++ {
		dx, dy := desplazamientoDeArranque(i)

		p := &participante{
			Nombre:   nombreDelJugador(i),
			Puerto:   puertoDelJugador(i),
			Servicio: maple.NuevoServicio(ancho, alto),
			Pal:      palJugador,
		}

		if i == 0 {
			p.Mov = mov
		} else {
			p.Mov = NuevoMovimiento(celdaX+dx, celdaY+dy)
		}

		p.Entrada = p.Servicio

		jugadores = append(jugadores, p)
	}

	for _, p := range jugadores {
		aparato := maple.AparatoEnPuerto(p.Puerto)
		p.Inyecta = !maple.HayControlEn(p.Puerto)

		println("JUEGO117: PUERTO jugador=", p.Nombre, "puerto=", p.Puerto, "aparato=", aparato,
			"hayControl=", maple.HayControlEn(p.Puerto), "inyecta=", p.Inyecta,
			"guionJugador2=", guionJugador2)
	}

	// EL JUGADOR 1 TAMBIEN PUEDE VENIR DEL GUION: con `inyectarJugador1=si` su puerto tambien lee el estado que pone
	// el shim. Es lo que permite una corrida CON LOS DOS GUIONADOS (los tramos SOLO-P1 / SOLO-P2 / LOS-DOS alineados
	// al reloj de la consola), sin depender de que el teclado virtual del host llegue a tiempo. El camino del input es
	// el mismo: bits del control -> tabla del backend -> teclas del motor; lo unico que cambia es de donde salieron los
	// bits, y el log lo dice en cada linea.
	if inyectarJugador1 == "si" && len(jugadores) > 0 {
		jugadores[0].Inyecta = true

		println("JUEGO117: PUERTO jugador=P1 FORZADO AL GUION (inyectarJugador1=si): el estado de su puerto lo pone el",
			"shim, no el teclado del emulador; aparatoReal=", maple.AparatoEnPuerto(puertoJugador1))
	}

	// LOS BYTES: lo que cuesta cada jugador. Los cuadros compuestos son de los dos (compartidos) y el estado es de
	// cada uno; el "no compartido" es lo que costaria el camino caro (componer un segundo juego de cuadros), y se
	// informa sin componerlo: es el tamano de los cuadros que ya estan en RAM.
	compartidos := bytesDeLosCuadros(jugadorQuieto)

	for _, c := range jugadoresCaminando {
		compartidos += bytesDeLosCuadros(c)
	}

	porJugador := 0

	for i := 0; i < len(jugadores); i++ {
		porJugador += estadoDeUnJugador()
	}

	println("JUEGO117: COOP MEMORIA jugadores=", len(jugadores), "cuadrosCompartidosBytes=", compartidos,
		"estadoPorJugadorBytes=", estadoDeUnJugador(), "estadoTotalBytes=", porJugador,
		"estructuraPorJugadorBytes=", tamanoDeParticipante,
		"estructuraTotalBytes=", tamanoDeParticipante*len(jugadores),
		"memoriaPorJugadorBytes=", estadoDeUnJugador()+tamanoDeParticipante,
		"memoriaTotalDeJugadoresBytes=", (estadoDeUnJugador()+tamanoDeParticipante)*len(jugadores),
		"noCompartidoBytes=", compartidos*len(jugadores), "compartirJugador2=", compartirJugador2,
		"direcciones=", len(direccionesCompuestas), "jugadorOK=", jugadorOK)

	// y el censo del momento, para comparar las dos corridas (1 y 2 jugadores) en el mismo punto
	memoria("coop-jugadores-listas")
	semiespacio("coop-jugadores-listas")
}

// estadoDeUnJugador: los bytes del estado de UN jugador: el `movimiento` (posicion del motor, direccion, reloj de
// animacion y contadores) mas el `Estado` que lleva la posicion en subtiles. Se informa el tamano del struct, que es
// lo que se duplica por jugador; los cuadros (los pixeles) no.
func estadoDeUnJugador() int {
	return tamanoDeMovimiento + tamanoDeEstado
}

// ============================== EL GUION DE LOS DOS (LA INDEPENDENCIA DEL INPUT) ==============================
//
// TRES TRAMOS, Y CADA UNO PRUEBA UNA COSA:
//
//	SOLO-P1  la cruceta del jugador 1 pide DERECHA. El jugador 2 no puede moverse: si se moviera, el input de un
//	         jugador estaria llegando al otro.
//	SOLO-P2  al reves: el que pide IZQUIERDA+ABAJO es el jugador 2 y el 1 tiene la cruceta suelta.
//	LOS-DOS  los dos piden cosas distintas a la vez (P1 ARRIBA+IZQUIERDA, P2 DERECHA+ABAJO): las dos posiciones se
//	         mueven en sentidos contrarios en el mismo cuadro.
//
// Los bits son los del control de la Dreamcast (d2core/d2input/maple: BitUp/BitDown/BitLeft/BitRight), los mismos
// del guion de DC-002.113. El guion entra por la MISMA puerta que un control de verdad.
type tramoCoop struct {
	Nombre string

	// Bits: los botones de CADA jugador en el tramo (indice = jugador). Con un tramo de dos, `Bits[0]` es el
	// jugador 1 y `Bits[1]` el 2; los otros dos quedan en cero. DC-002.117 generalizo esto de (B1,B2) a un arreglo
	// de cuatro porque ahora hay corridas con tres y cuatro jugadores, y el guion tiene que poder pedirle algo
	// DISTINTO a cada uno.
	Bits [maxJugadoresCoop]uint32

	Segundos float64
}

// pausaCoop: lo que se suelta TODO entre tramo y tramo, para que los tramos no se mezclen en la medida.
const pausaCoop = 1.0

// TRAMOS DE 4 S Y NO DE 6 (medido en la primera corrida de esta tarjeta): en 6 s cada jugador camina 11 celdas y, con
// los tramos mandandolos en sentidos contrarios, al llegar a LOS-DOS quedaban a 20 celdas uno del otro: la camara va
// al centro del grupo y en la proyeccion isometrica 20 celdas son mas de 480 px, asi que se salian los dos de la
// pantalla (anclas 828,-279 y -189,844). En 4 s son ~7,5 celdas y entran.
//
// EL TRAMO SOLO-P2 VA HACIA LA IZQUIERDA (y no izquierda+abajo) PARA QUE SE ACERQUE AL JUGADOR 1: asi el tramo deja a
// los dos juntos en el cuadro y la captura de ese tramo muestra dos personajes. La prueba no cambia: en SOLO-P2 el
// unico que pide es el jugador 2, y los ticks del jugador 1 tienen que ser 0.
//
// LOS-DOS los manda en sentidos OPUESTOS (arriba-izquierda contra derecha-abajo), que es la prueba mas dura: cada uno
// tiene su direccion y su reloj en el mismo cuadro. Es tambien el caso borde del cooperativo en UNA pantalla: a ~20
// celdas de separacion la camara del grupo los deja fuera del lienzo, y ese es exactamente el motivo por el que el
// split screen es otra tarjeta.
var guionCoop = []tramoCoop{
	{"SOLO-P1", [maxJugadoresCoop]uint32{bitDerecha}, 4.0},
	{"SOLO-P2", [maxJugadoresCoop]uint32{0, bitIzquierda}, 4.0},
	{"LOS-DOS", [maxJugadoresCoop]uint32{bitArriba | bitIzquierda, bitDerecha | bitAbajo}, 4.0},
}

// ============================== EL GUION DE ESTA TARJETA (DC-002.115): SEPARARSE ==============================
//
// El guion del leash NO prueba la independencia del input (eso ya lo midio DC-002.114): prueba lo contrario, que
// los dos se puedan mover TODO el tiempo y que el leash los frene. En cada tramo el jugador 1 camina en UNA de las
// 8 direcciones de la cruceta y el 2 en la CONTRARIA, asi que la pareja se separa en esa direccion hasta que el
// leash la para, y se queda ahi los 7 s del tramo.
//
// POR QUE LAS 8 Y NO UNA. Porque la separacion en pantalla depende de la direccion (ver leash.go: una celda son
// 113 px en una diagonal y 56 en la otra), asi que la direccion mas dificil no es la que uno supondria: es
// DERECHA/IZQUIERDA, la que separa en la diagonal de celdas (1,-1). Las 8 corridas de 7 s dan los 63 s de la
// corrida del criterio, y la peor de todas tiene que quedar adentro del lienzo.
//
// La pausa de 1 s entre tramos es la misma `pausaCoop`: los dos se quedan quietos a la distancia del leash.
var guionLeash = []tramoCoop{
	{"SEPARAR-DERECHA", [maxJugadoresCoop]uint32{bitDerecha, bitIzquierda}, 7.0},
	{"SEPARAR-ABAJO+DERECHA", [maxJugadoresCoop]uint32{bitAbajo | bitDerecha, bitArriba | bitIzquierda}, 7.0},
	{"SEPARAR-ABAJO", [maxJugadoresCoop]uint32{bitAbajo, bitArriba}, 7.0},
	{"SEPARAR-ABAJO+IZQUIERDA", [maxJugadoresCoop]uint32{bitAbajo | bitIzquierda, bitArriba | bitDerecha}, 7.0},
	{"SEPARAR-IZQUIERDA", [maxJugadoresCoop]uint32{bitIzquierda, bitDerecha}, 7.0},
	{"SEPARAR-ARRIBA+IZQUIERDA", [maxJugadoresCoop]uint32{bitArriba | bitIzquierda, bitAbajo | bitDerecha}, 7.0},
	{"SEPARAR-ARRIBA", [maxJugadoresCoop]uint32{bitArriba, bitAbajo}, 7.0},
	{"SEPARAR-ARRIBA+DERECHA", [maxJugadoresCoop]uint32{bitArriba | bitDerecha, bitAbajo | bitIzquierda}, 7.0},
}

// guionDeLaCorrida: el guion que se usa. Lo elige la palanca `guionPoc` de knobs.go al arrancar ("coop" = los
// tres tramos de DC-002.114, "leash" = los 8 de DC-002.115, "split" = los dos de DC-002.116) y el log dice cual
// corrio.
var guionDeLaCorrida = guionCoop

// ============================== EL GUION DE ESTA TARJETA (DC-002.116): LOS DOS CAMINAN IGUAL ==============================
//
// POR QUE ESTE GUION Y NO EL DEL LEASH. Con el split los jugadores ya arrancan SEPARADOS (los pone
// `celdaP2X`/`celdaP2Y`: 22 celdas en la corrida del criterio) y no hay leash que los tenga que frenar. Lo que
// hay que ver en el split es otra cosa: que cada camara siga a SU jugador. Por eso los dos caminan en la MISMA
// direccion de la cruceta -- asi la separacion no cambia (22 celdas todo el rato) y lo unico que se mueve son
// las dos camaras -- en dos tramos de 2 s, ida y vuelta, para que el mapa corra en las dos mitades y vuelva.
//
// ANDAR LOS DOS PARA EL MISMO LADO ES A PROPOSITO: el guion del leash separa (22 celdas se volverian 40) y con
// dos zonas distintas el que crece es la cache de tiles, que es justo el recurso que este POC mide. Dos tramos
// cortos dejan la cache en lo que cuestan las DOS pantallas, que es el numero de la tarjeta.
var guionSplit = []tramoCoop{
	{"LOS-DOS-DERECHA", [maxJugadoresCoop]uint32{bitDerecha, bitDerecha}, 2.0},
	{"LOS-DOS-IZQUIERDA", [maxJugadoresCoop]uint32{bitIzquierda, bitIzquierda}, 2.0},
}

// guionSplitQuieto: EL GUION DE LA CORRIDA DEL CRITERIO. Los dos quietos, sin cruceta, 4 s.
//
// POR QUE QUIETO, CON LO QUE CUESTA MEDIDO. Con el split la cache de tiles tiene que tener las DOS zonas del mapa
// (no una) y la VRAM no da para las dos mas el movimiento: las dos ventanas quietas cuestan 3.276.800 B (P1
// 2.600.960 en la celda del POC + P2 675.840 en (52,12), medidos en el host con render-294) y el tramo que deja
// la VRAM son 3.481.600 B. Un solo paso de camara agrega la columna nueva de cada mitad (~130-200 KB por celda de
// cada jugador) y lo que no entra se DERRAMA al monton de Go, donde es VIVO: medido con las dos zonas caras
// ((28,20)+(50,20) = 3,69 MB) el recolector muere con "GC to-space overflow". El guion que camina existe
// (`guionPoc=split`) y es la prueba de que cada camara SIGUE a su jugador; se corre aparte y se mide su costo.
var guionSplitQuieto = []tramoCoop{
	{"QUIETO", [maxJugadoresCoop]uint32{}, 4.0},
}

// ============================== EL GUION DE ESTA TARJETA (DC-002.117): LOS CUATRO ==============================
//
// TRES TRAMOS, Y CADA UNO MIDE UNA COSA (mas un QUIETO de arranque):
//
//	QUIETO                   1 s sin cruceta en ningun puerto. Es el cuadro 1: con `JUGADORES=1` el cuadro del
//	                         arranque tiene que ser EXACTAMENTE el de DC-002.112 (ancla 320,283 y crc32
//	                         1363871674), y esa es la prueba de que generalizar el dibujo a cuatro jugadores no
//	                         movio el camino del que habia uno solo. Con cuatro, es la foto de los cuatro juntos.
//	LOS-CUATRO-DERECHA       los CUATRO caminan para el mismo lado 2 s. Es el tramo del criterio: los cuatro
//	                         dibujados y moviendose en el mismo cuadro, y como caminan juntos la separacion no
//	                         cambia (el leash no tiene nada que recortar) y la camara del centro no se corre.
//	CADA-UNO-SU-DIRECCION    cada uno en UNA direccion distinta (P1 derecha, P2 izquierda, P3 arriba, P4 abajo), 2 s.
//	                         Es la prueba de que el input de cada puerto llega a SU jugador, y la que ejerce el
//	                         LEASH DE GRUPO: los cuatro empujan para afuera y el grupo no se puede estirar mas de
//	                         lo que entra en 640x480, asi que se los frena y se cuentan los recortes.
//	LOS-CUATRO-IZQUIERDA     los cuatro de vuelta, para que el cuadro final quede cerca del origen.
//
// LA DIRECCION DE CADA UNO ES UNA DE LAS CUATRO DE LA CRUZETA (no las diagonales) a proposito: con cuatro jugadores
// en un cuadrado de 3x3 celdas, las diagonales ya estan casi al limite del leash (4,24 celdas de 5,25) y el tramo
// del criterio tiene que terminar con los cuatro adentro.
var guionCuatro = []tramoCoop{
	{"QUIETO", [maxJugadoresCoop]uint32{}, 1.0},
	{"LOS-CUATRO-DERECHA", [maxJugadoresCoop]uint32{bitDerecha, bitDerecha, bitDerecha, bitDerecha}, 2.0},
	{"CADA-UNO-SU-DIRECCION", [maxJugadoresCoop]uint32{bitDerecha, bitIzquierda, bitArriba, bitAbajo}, 2.0},
	{"LOS-CUATRO-IZQUIERDA", [maxJugadoresCoop]uint32{bitIzquierda, bitIzquierda, bitIzquierda, bitIzquierda}, 2.0},
}

// guionEnUso: el nombre del guion que quedo (la palanca, ya validada). Se imprime: el binario dice con cual corrio.
var guionEnUso = guionPoc

// elegirGuion: deja `guionDeLaCorrida` en el que pide la palanca. Se llama UNA vez, al arranque, antes de que el
// ciclo del PVR empiece a pedir botones.
func elegirGuion() {
	switch guionPoc {
	case "leash":
		guionDeLaCorrida = guionLeash
		guionEnUso = "leash"
	case "split":
		guionDeLaCorrida = guionSplit
		guionEnUso = "split"
	case "split-quieto":
		guionDeLaCorrida = guionSplitQuieto
		guionEnUso = "split-quieto"
	case "cuatro":
		guionDeLaCorrida = guionCuatro
		guionEnUso = "cuatro"
	default:
		guionDeLaCorrida = guionCoop
		guionEnUso = "coop"
	}

	println("JUEGO117: GUION poc=", guionEnUso, "tramos=", len(guionDeLaCorrida), "segundos=", segundosDelGuionCoop())
}

// botonesCoop: que botones pide cada jugador en el segundo `t` del guion (todos en cero entre tramos y al final).
//
// DC-002.117: devuelve el arreglo COMPLETO (un boton por jugador), no dos. El que llama solo tiene que mirar el
// indice de cada participante.
func botonesCoop(t float64) [maxJugadoresCoop]uint32 {
	acumulado := 0.0

	for i := range guionDeLaCorrida {
		ini := acumulado
		fin := ini + guionDeLaCorrida[i].Segundos

		if t >= ini && t < fin {
			return guionDeLaCorrida[i].Bits
		}

		acumulado = fin + pausaCoop
	}

	return [maxJugadoresCoop]uint32{}
}

// indiceDelTramoCoop: en que tramo del guion cae `t` (-1 = en una pausa o despues del final).
func indiceDelTramoCoop(t float64) int {
	acumulado := 0.0

	for i := range guionDeLaCorrida {
		ini := acumulado
		fin := ini + guionDeLaCorrida[i].Segundos

		if t >= ini && t < fin {
			return i
		}

		acumulado = fin + pausaCoop
	}

	return -1
}

// segundosDelGuionCoop: lo que dura el guion entero (con las pausas), para saber si entra en los cuadros que se
// dibujan.
func segundosDelGuionCoop() float64 {
	total := 0.0

	for i := range guionDeLaCorrida {
		total += guionDeLaCorrida[i].Segundos + pausaCoop
	}

	return total
}

// ============================== EL PASO DE LOS DOS ==============================

// leerYAvanzarJugadores: un cuadro de los dos jugadores. Para cada uno: se empuja el guion si su puerto no tiene
// aparato, se lee SU puerto, se sacan los ejes, y se avanza SU movimiento con la velocidad del motor.
//
// Devuelve si el jugador 1 camino (es lo que el cuadro de DC-002.113 miraba) y cuantos jugadores caminaron.
func leerYAvanzarJugadores(segundos float64) (movidoP1 bool, caminando int) {
	pide := botonesCoop(reloj)

	for i, p := range jugadores {
		if p.Inyecta {
			// DC-002.117: cada jugador pide los SUYOS (`pide[i]`); el 2 puede quedar en cero para la corrida en la
			// que el que mueve es el 1 y el 2 tiene que quedarse quieto aunque haya guion.
			botones := pide[i]
			if i == 1 && guionJugador2 != "si" {
				botones = 0
			}

			p.Servicio.InyectarBotonesEn(p.Puerto, botones)
		}

		p.Servicio.ActualizarDesdeElControlEn(p.Puerto)

		p.Botones = p.Servicio.Botones()
		p.EjeX, p.EjeY = ejesDelControl(p.Entrada)

		// DC-002.115: la celda de ANTES del paso. Es lo que despues usa el leash para saber cuanto camino el
		// jugador DE VERDAD cuando lo frena (lo que pidio el paso menos lo que el leash le devolvio).
		p.IniX, p.IniY = p.Mov.Est.Celda()

		ticks, caminado := p.Mov.Paso(p.EjeX, p.EjeY, segundos)
		p.Ticks, p.Caminado = ticks, caminado

		if caminado > 0 {
			caminando++

			if i == 0 {
				movidoP1 = true
			}
		}
	}

	return movidoP1, caminando
}

// leerElControlDeLosDos: la lectura cruda de cada puerto, para el log (bits del control y si su estado viene del
// guion). Es el instrumento de "cada uno con su input".
func leerElControlDeLosDos() string {
	s := ""

	for _, p := range jugadores {
		if s != "" {
			s += " "
		}

		s += p.Nombre + ":botones=" + itoa(int(p.Botones)) + ",ejes=" + itoa(int(p.EjeX)) + "/" + itoa(int(p.EjeY)) +
			",inyecta=" + siNo(p.Inyecta)
	}

	return s
}

// ============================== LA MEDIDA POR TRAMO (LOS DOS) ==============================

// tramoDeLosDos: un tramo del guion medido con el reloj de la consola, para CADA jugador: es lo que dice si el
// input de uno movio al otro. DC-002.117: los arreglos son de cuatro (el tope del cooperativo), no de dos.
type tramoDeLosDos struct {
	Indice   int
	Nombre   string
	Segundos float64
	Ticks    [maxJugadoresCoop]int
	Subs     [maxJugadoresCoop]float64
	CeldaIni [maxJugadoresCoop][2]float64
	CeldaFin [maxJugadoresCoop][2]float64
}

var tramoDos *tramoDeLosDos

// medirTramoDeLosDos: suma el cuadro al tramo del guion que esta corriendo. Cuando cambia el tramo (o se acaba),
// cierra el anterior y lo imprime.
func medirTramoDeLosDos(segundos float64) {
	indice := indiceDelTramoCoop(reloj)

	if tramoDos == nil || tramoDos.Indice != indice {
		cerrarTramoDeLosDos()

		if indice < 0 {
			return
		}

		tramoDos = &tramoDeLosDos{Indice: indice, Nombre: guionDeLaCorrida[indice].Nombre}

		for i, p := range jugadores {
			tramoDos.CeldaIni[i][0], tramoDos.CeldaIni[i][1] = p.Mov.Est.Celda()
		}
	}

	tramoDos.Segundos += segundos

	for i, p := range jugadores {
		tramoDos.Ticks[i] += p.Ticks
		tramoDos.Subs[i] += p.Caminado
	}
}

// cerrarTramoDeLosDos: imprime el tramo: los ticks y las celdas de CADA jugador (con `JUGADORES=1` es el tramo de
// siempre; con cuatro, una terna por jugador).
func cerrarTramoDeLosDos() {
	if tramoDos == nil {
		return
	}

	t := tramoDos
	tramoDos = nil

	for i, p := range jugadores {
		t.CeldaFin[i][0], t.CeldaFin[i][1] = p.Mov.Est.Celda()

		celdas := t.Subs[i] / float64(subtilesPorCelda)
		celdasPorSegundo := 0.0

		if t.Segundos > 0 {
			celdasPorSegundo = celdas / t.Segundos
		}

		println("JUEGO117: COOP TRAMO", t.Nombre, "segundos=", t.Segundos, "jugador=", p.Nombre,
			"ticks=", t.Ticks[i], "celdas=", celdas, "celdasPorSegundo=", celdasPorSegundo,
			"ini=", t.CeldaIni[i][0], t.CeldaIni[i][1], "fin=", t.CeldaFin[i][0], t.CeldaFin[i][1],
			"direccion=", p.Mov.Direccion, "jugadores=", len(jugadores))
	}

	// la separacion del grupo al cerrar el tramo (el maximo entre los pares): es el numero que dice si el leash
	// dejo el grupo adentro del lienzo, tramo por tramo
	d, _, _, _, _ := separacionMaximaDelGrupo()

	println("JUEGO117: COOP TRAMO", t.Nombre, "cierra separacionMaxCeldas=", d, "leashCeldas=", leashEnCeldas,
		"jugadores=", len(jugadores))
}

// ============================== EL DIBUJO DE LOS DOS ==============================

// prepararElCuadroDeUnJugador: el ciclo, el cuadro de la animacion, el ancla y la caja de un jugador, y la copia de
// lo que hay DEBAJO de su caja (para poder devolverle el mapa sin repintar los 640x480).
//
// El ciclo sale de `cicloDelJugador` (jugador.go: caminando el de su direccion, quieto el NU), y el cuadro del reloj
// de SU movimiento (`CuadroDeLaAnimacion`): los dos jugadores pueden estar en cuadros distintos del MISMO ciclo
// compartido, que es lo que hace que se vean animados por separado.
func prepararElCuadroDeUnJugador(p *participante, pixeles []byte) bool {
	ciclo := cicloDelJugador(p.Mov.Direccion, p.Mov.Caminando)
	if ciclo == nil {
		return false
	}

	cu := p.Mov.CuadroDeLaAnimacion(ciclo)
	if cu == nil {
		return false
	}

	p.CuadroDelCiclo = indiceDelCuadro(ciclo, cu)

	cx, cy := p.Mov.Est.Celda()

	p.AnclaX, p.AnclaY = AnclaDelJugador(rendererDelMapa.WorldToScreenF, cx, cy)

	// DC-002.115: la caja SIN recortar es el numero del criterio (que el jugador entre en el lienzo); la recortada
	// es la que se puede pintar. Van las dos: la cruda para la vigilancia, la recortada para el pincel.
	p.CajaCruda = CajaDe(p.AnclaX, p.AnclaY, cu)
	p.Dentro = cajaDentroDelLienzo(p.CajaCruda)

	// la caja puede caerse de la pantalla si el jugador camina lejos con la camara fija: se recorta a la pantalla
	// para no escribir afuera del lienzo
	p.Caja = recortarALaPantalla(p.CajaCruda)
	if p.Caja.Ancho() <= 0 || p.Caja.Alto() <= 0 {
		return false
	}

	guardarCajaDe(p, pixeles)

	return true
}

// dibujarAJugadores: pega los dos jugadores en el lienzo del PVR, EN ORDEN DE PROFUNDIDAD (el que esta mas arriba en
// el mapa queda detras), y devuelve el crc32 del cuadro con los dos. Antes de pegar cada uno se guarda lo que hay
// debajo de su caja; despues se devuelve el mapa en orden INVERSO, que es lo que deja el mapa intacto aunque las dos
// cajas se toquen (la copia del jugador de atras es mapa limpio).
func dibujarAJugadores(malla *pvr.Surface, pixeles []byte) (crcConJugadores uint32, msTotal int) {
	listos := make([]*participante, 0, len(jugadores))

	for _, p := range jugadores {
		if prepararElCuadroDeUnJugador(p, pixeles) {
			listos = append(listos, p)
		}
	}

	// EL ORDEN DE DIBUJO: por profundidad. En la proyeccion isometrica el que tiene menor (x+y) esta mas arriba en
	// la pantalla y tapa menos: se dibuja PRIMERO. DC-002.117: con tres y cuatro jugadores es una insercion sobre
	// todos, no un intercambio de dos.
	ordenarPorProfundidad(listos)

	for _, p := range listos {
		cu := p.Mov.CuadroDeLaAnimacion(cicloDelJugador(p.Mov.Direccion, p.Mov.Caminando))

		t1 := d2cMs()
		PintarJugador(malla, cu, p.AnclaX, p.AnclaY, p.Pal[:])
		p.MsDibujo = int(d2cMs() - t1)

		msTotal += p.MsDibujo
		p.Cambiados = pixelesCambiadosEntre(pixeles, p)
	}

	crcConJugadores = crc32De(pixeles)

	// se devuelve el mapa, de atras para adelante
	for i := len(listos) - 1; i >= 0; i-- {
		restaurarCajaDe(listos[i], pixeles)
	}

	return crcConJugadores, msTotal
}

// volverAPintarAJugadores: el cuadro que se presenta: los dos otra vez, en el mismo orden de profundidad.
func volverAPintarAJugadores(malla *pvr.Surface) {
	listos := make([]*participante, 0, len(jugadores))

	for _, p := range jugadores {
		if p.Caja.Ancho() > 0 && p.Caja.Alto() > 0 {
			listos = append(listos, p)
		}
	}

	ordenarPorProfundidad(listos)

	for _, p := range listos {
		cu := p.Mov.CuadroDeLaAnimacion(cicloDelJugador(p.Mov.Direccion, p.Mov.Caminando))
		if cu == nil {
			continue
		}

		PintarJugador(malla, cu, p.AnclaX, p.AnclaY, p.Pal[:])
	}
}

// ordenarPorProfundidad: de atras para adelante en la proyeccion isometrica: el que tiene menor (x+y) esta mas
// arriba en la pantalla y tapa menos, asi que se dibuja PRIMERO.
//
// DC-002.117: es una insercion y no un `sort.Slice` a proposito. Para CUATRO elementos la insercion es lo mismo de
// barata, y `sort` en la consola arrastra reflection sobre interfaces, que es justo lo que este POC mide.
func ordenarPorProfundidad(listos []*participante) {
	for i := 1; i < len(listos); i++ {
		for j := i; j > 0; j-- {
			xj, yj := listos[j].Mov.Est.Celda()
			xp, yp := listos[j-1].Mov.Est.Celda()

			if xj+yj >= xp+yp {
				break
			}

			listos[j], listos[j-1] = listos[j-1], listos[j]
		}
	}
}

// ============================== LAS CAJAS (UNA POR JUGADOR) ==============================

// guardarCajaDe: copia lo que hay en el lienzo dentro de la caja del jugador. Es el `guardarCaja` de DC-002.113, pero
// con el buffer del participante (dos jugadores son dos cajas vivas a la vez).
func guardarCajaDe(p *participante, pixeles []byte) {
	necesario := p.Caja.Ancho() * p.Caja.Alto() * 4

	if p.CajaGuardada == nil || len(p.CajaGuardada) < necesario {
		p.CajaGuardada = make([]byte, necesario)
	}

	n := 0

	for y := p.Caja.Y0; y < p.Caja.Y1; y++ {
		for x := p.Caja.X0; x < p.Caja.X1; x++ {
			j := (y*ancho + x) * 4
			copy(p.CajaGuardada[n:n+4], pixeles[j:j+4])

			n += 4
		}
	}
}

// restaurarCajaDe: devuelve la caja del jugador a lo que habia debajo (el mapa).
func restaurarCajaDe(p *participante, pixeles []byte) {
	n := 0

	for y := p.Caja.Y0; y < p.Caja.Y1; y++ {
		for x := p.Caja.X0; x < p.Caja.X1; x++ {
			j := (y*ancho + x) * 4
			copy(pixeles[j:j+4], p.CajaGuardada[n:n+4])

			n += 4
		}
	}
}

// pixelesCambiadosEntre: cuantos pixeles de la caja del jugador NO son los del mapa (los que pinto de verdad).
func pixelesCambiadosEntre(pixeles []byte, p *participante) int {
	n := 0
	cambiados := 0

	for y := p.Caja.Y0; y < p.Caja.Y1; y++ {
		for x := p.Caja.X0; x < p.Caja.X1; x++ {
			j := (y*ancho + x) * 4

			if pixeles[j] != p.CajaGuardada[n] || pixeles[j+1] != p.CajaGuardada[n+1] ||
				pixeles[j+2] != p.CajaGuardada[n+2] || pixeles[j+3] != p.CajaGuardada[n+3] {
				cambiados++
			}

			n += 4
		}
	}

	return cambiados
}

// recortarALaPantalla: la caja del jugador adentro de la pantalla (si camina lejos con la camara fija se sale del
// lienzo, y escribir afuera es corromper memoria ajena).
func recortarALaPantalla(c CajaDelJugador) CajaDelJugador {
	if c.X0 < 0 {
		c.X0 = 0
	}

	if c.Y0 < 0 {
		c.Y0 = 0
	}

	if c.X1 > ancho {
		c.X1 = ancho
	}

	if c.Y1 > alto {
		c.Y1 = alto
	}

	if c.X1 < c.X0 {
		c.X1 = c.X0
	}

	if c.Y1 < c.Y0 {
		c.Y1 = c.Y0
	}

	return c
}

// ============================== LA CAMARA Y LOS NUMEROS DEL CIERRE ==============================

// centroDeLosJugadores: el CENTRO del grupo, en celdas del mapa: es donde se pone la camara para que los dos entren
// en la misma pantalla. Con un solo jugador es su propia celda, asi que el cuadro 1 sigue siendo el de DC-002.112.
func centroDeLosJugadores() (float64, float64) {
	if len(jugadores) == 0 || jugadores[0].Mov == nil {
		return celdaX, celdaY
	}

	sx, sy := 0.0, 0.0

	for _, p := range jugadores {
		x, y := p.Mov.Est.Celda()
		sx += x
		sy += y
	}

	n := float64(len(jugadores))

	return sx / n, sy / n
}

// resumenDelCoop: la linea del criterio de esta tarjeta: cuantos jugadores, cuantos cuadros, la memoria del semiespacio
// en el cuadro, y los ms por cuadro de cada jugador (que es lo que se compara contra la corrida de uno solo).
//
// DC-002.117: una linea POR JUGADOR (los cuatro casos de la tabla salen de la misma funcion) mas la del grupo, con
// la separacion maxima y la memoria de los participantes.
func resumenDelCoop(cuadrosTotales int, msMapa, msJugadores, msCuadro int) {
	for _, p := range jugadores {
		cx, cy := p.Mov.Est.Celda()

		println("JUEGO117: COOP JUGADOR nombre=", p.Nombre, "puerto=", p.Puerto, "inyecta=", p.Inyecta,
			"celda=", cx, cy, "caminado=", p.Mov.Subtiles, "ticks=", p.Mov.Ticks,
			"caminando=", p.Mov.Caminando, "direccion=", p.Mov.Direccion,
			"msDibujo=", p.MsDibujo, "recortes=", p.LeashRecortes, "celdasRecortadas=", p.LeashCeldas,
			"dentroDelLienzo=", siNo(p.Dentro))
	}

	d, x1, y1, x2, y2 := separacionMaximaDelGrupo()

	println("JUEGO117: COOP RESUMEN jugadores=", len(jugadores), "cuadrosPintados=", cuadrosTotales,
		"msMapa=", msMapa, "msJugadores=", msJugadores, "msCuadro=", msCuadro,
		"tramoDeLaCacheBytes=", tramoDeLaCache, "leashEnCeldas=", leashEnCeldas,
		"separacionMaxCeldas=", d, "parMasLejano=", x1, y1, x2, y2,
		"estadoPorJugadorBytes=", estadoDeUnJugador(), "estructuraPorJugadorBytes=", tamanoDeParticipante,
		"memoriaDeLosJugadoresBytes=", (estadoDeUnJugador()+tamanoDeParticipante)*len(jugadores),
		"semiespacioUsado=", int(d2cGcUsado()), "semiespacioTotal=", int(d2cGcTotal()),
		"colecciones=", int(d2cGcColecciones()), "montonLibre=", d2cMontonLibre(),
		"guionSegundos=", segundosDelGuionCoop())

	memoria("coop-fin")
	semiespacio("coop-fin")
}

// siNo: el booleano como texto corto (la consola no tiene fmt).
func siNo(b bool) string {
	if b {
		return "si"
	}

	return "no"
}

// ============================== LA VUELTA AL ORIGEN Y AL FINAL, CON LOS DOS ==============================

// estadoGuardado: lo que hay que devolver de un jugador para repetir un cuadro (la posicion del motor, la direccion,
// el reloj de la animacion y si estaba caminando). Es un tipo CON NOMBRE a proposito: el compilador de SH-4
// miscompila las rebanadas de structs anonimos (medido en DC-002.50/252 y vigilado por checar-codegen-252.py).
type estadoGuardado struct {
	X, Y      float64
	Dir       int
	MsAnim    float64
	Caminando bool
}

// finalDeLosJugadores: el estado del cierre, en un arreglo de tamano fijo (cuatro jugadores es el tope del juego y
// asi no hay `append` ni recorte en el camino caliente).
var finalDeLosJugadores [maxJugadoresCoop]estadoGuardado

// maxJugadoresCoop: el tope del cooperativo de Diablo II (los cuatro puertos del bus maple).
const maxJugadoresCoop = 4

var cuantosFinales int

// guardarElFinalDeLosJugadores: guarda el estado de TODOS los jugadores antes de la prueba de repetibilidad.
func guardarElFinalDeLosJugadores() {
	cuantosFinales = len(jugadores)
	if cuantosFinales > maxJugadoresCoop {
		cuantosFinales = maxJugadoresCoop
	}

	for i := 0; i < cuantosFinales; i++ {
		p := jugadores[i]
		x, y := p.Mov.Est.Celda()

		finalDeLosJugadores[i] = estadoGuardado{
			X: x, Y: y, Dir: p.Mov.Direccion, MsAnim: p.Mov.MsAnim, Caminando: p.Mov.Caminando,
		}
	}
}

// volverAlOrigenLosJugadores: cada jugador a su celda de arranque, quieto y con el reloj de la animacion en cero, y
// la camara al centro del grupo. Es lo que deja el cuadro del cierre igual al primero.
func volverAlOrigenLosJugadores() {
	for i := range jugadores {
		p := jugadores[i]

		p.Mov.Est = NuevoEstado(celdaDeArranqueX(i), celdaDeArranqueY(i), false)
		p.Mov.Direccion = dirQuieto
		p.Mov.Caminando = false
		p.Mov.MsAnim = 0
		p.Mov.Resto = 0
	}

	moverLaCamaraAlCentro()
}

// volverAlFinalLosJugadores: devuelve a cada jugador al estado que guardo `guardarElFinalDeLosJugadores`.
func volverAlFinalLosJugadores() {
	for i := 0; i < cuantosFinales && i < len(jugadores); i++ {
		p := jugadores[i]
		s := finalDeLosJugadores[i]

		p.Mov.Est = NuevoEstado(s.X, s.Y, false)
		p.Mov.Direccion = s.Dir
		p.Mov.MsAnim = s.MsAnim
		p.Mov.Caminando = s.Caminando
	}

	moverLaCamaraAlCentro()
}

// celdaDeArranqueX / celdaDeArranqueY: donde arranca cada jugador. El 1 en la celda del POC (28,20), que es la del
// cuadro de DC-002.112; los otros en el cuadrado de 3x3 celdas de `desplazamientoDeArranque` (knobs.go).
func celdaDeArranqueX(i int) float64 {
	dx, _ := desplazamientoDeArranque(i)

	return celdaX + dx
}

func celdaDeArranqueY(i int) float64 {
	_, dy := desplazamientoDeArranque(i)

	return celdaY + dy
}

// ============================== LOS DATOS DE CADA JUGADOR (DC-002.117) ==============================

// nombresDeJugador: P1..P4, para el log (la consola no tiene fmt).
var nombresDeJugador = [maxJugadoresCoop]string{"P1", "P2", "P3", "P4"}

// nombreDelJugador: el nombre del que ocupa el indice `i` del cooperativo.
func nombreDelJugador(i int) string {
	if i < 0 || i >= maxJugadoresCoop {
		return "P?"
	}

	return nombresDeJugador[i]
}

// puertoDelJugador: el puerto del bus maple de cada uno (A, B, C y D). No se recorre el bus buscando "el siguiente
// control": cada jugador tiene su direccion fisica, que es lo que leyo DC-002.114.
func puertoDelJugador(i int) int {
	switch i {
	case 0:
		return puertoJugador1
	case 1:
		return puertoJugador2
	case 2:
		return puertoJugador3
	default:
		return puertoJugador4
	}
}

// desplazamientoDeArranque: donde arranca cada jugador, en CELDAS desde la celda del POC (28,20). El 1 en el origen
// (el cuadro de DC-002.112); el 2, el 3 y el 4 en el cuadrado de 3x3 celdas de knobs.go, que es lo que los deja a
// los cuatro adentro de un solo lienzo de 640x480 con el leash medido.
func desplazamientoDeArranque(i int) (float64, float64) {
	switch i {
	case 0:
		return 0, 0
	case 1:
		return celdaP2X, celdaP2Y
	case 2:
		return celdaP3X, celdaP3Y
	default:
		return celdaP4X, celdaP4Y
	}
}

// moverLaCamaraAlCentro: la camara al centro del grupo. Con un solo jugador es su celda, o sea la misma camara de
// DC-002.112/.113; con dos es el medio, que es lo que los deja a los dos en la pantalla sin partirla.
func moverLaCamaraAlCentro() {
	if rendererDelMapa == nil {
		return
	}

	cx, cy := centroDeLosJugadores()

	rx, ry := rendererDelMapa.WorldToOrtho(cx, cy)
	pos := d2vector.NewPosition(rx, ry)

	rendererDelMapa.MoveCameraTo(&pos)

	camaraCeldaX, camaraCeldaY = cx, cy
}

// camaraCeldaX / camaraCeldaY: la celda en la que quedo la camara (la guarda el que la mueve: el renderer no tiene la
// vuelta de WorldToOrtho). Es lo que explica el unico numero que CAMBIA entre la corrida de uno y la de dos: con dos
// jugadores la camara va al centro del grupo (29,20), no a la celda del jugador 1 (28,20), y ese medio tile corre el
// cuadro 80 px, asi que el crc32 del cuadro 1 NO puede ser el de DC-002.112. El mapa solo se compara con el host en
// la corrida de UN jugador, que es donde la camara esta en la misma celda.
var camaraCeldaX, camaraCeldaY float64

// celdaDeLaCamaraX / celdaDeLaCamaraY: para el log.
func celdaDeLaCamaraX() float64 { return camaraCeldaX }
func celdaDeLaCamaraY() float64 { return camaraCeldaY }

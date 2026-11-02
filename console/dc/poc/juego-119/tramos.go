//go:build dreamcast

package main

// DC-002.119 -- LOS TRAMOS CON DUENO DEL POC: lo que no necesita ser puntero de Go sale del semiespacio.
//
// POR QUE EXISTE (DC-002.117, medido). Con cuatro jugadores el semiespacio del recolector queda al 93,5 % en el
// peor momento de la corrida (1.716.240 de 1.835.008 B) y ese es el techo del cooperativo: no lo pone la memoria de
// los jugadores (5.184 B los cuatro) sino lo que vive DENTRO del semiespacio. La decision 1b del ADR 0001
// (`docs/adr/0001-assets-fuera-del-heap-de-go.md`) dice que lo que no sea puntero grande se saque de ahi con dueno
// explicito, que es exactamente lo que hace este archivo.
//
// COMO, Y POR QUE ASI. En libgodc toda asignacion de mas de 64 KB se va a `gc_external_alloc` = `malloc` pelado y el
// recolector no la ve nunca; lo que pesa menos cae en el semiespacio y se copia en cada recoleccion. Los cuadros
// compuestos del jugador (72 cuadros de ~2 KB: 165.152 B) y las cajas de guardado (~8,8 KB por jugador) son
// justamente chicos, asi que hoy viven en el semiespacio. `make` no sirve para sacarlos (solo pasa a malloc si el
// pedido pasa los 64 KB), asi que se piden a mano con `d2c_tramo_pedir` (csrc/shim.c, seccion 9): un bloque del
// monton de C del tamano EXACTO, en cero, del que se reparten los trozos.
//
// EL CONTRATO DEL TRAMO (el mismo de `asset.TramoPrestado` y de la cache de tiles):
//   - un tramo tiene NOMBRE y DUENO, y el dueno es el que lo devuelve: `t.liberar()`;
//   - adentro NO van punteros de Go (lo que va son pixeles de 8 bits y copias de pixeles: bytes);
//   - se pide UNA vez y se reparte en trozos; soltarlo invalida todos los trozos;
//   - el recolector no lo ve: lo que quede apuntando adentro desde Go es un puntero fuera del monton, como los
//     tramos de la cache y el arena del `.dcc`.
//
// LA PALANCA (`tramosPropios`, knobs.go) es el A/B de la tarjeta: con `false` el POC NO pide ni un tramo y todo
// queda como en DC-002.118 (el semiespacio de antes, el mismo cuadro); con `true` las categorias gordas se mudan.
// El MISMO binario corre las dos, asi que la unica diferencia entre las dos medidas son los tramos.

import "unsafe"

// Los puentes de C (csrc/shim.c, seccion 9).
//
//extern d2c_tramo_pedir
func d2cTramoPedir(bytes uint) uintptr

//extern d2c_tramo_soltar
func d2cTramoSoltar(p uintptr)

// alineacionTramo: los trozos se reparten alineados a 8, que es lo que pide el decodificador del `.dcc` para sus
// `DCCPixelBufferEntry` (dos `int`) y lo que deja los pixeles en una direccion comoda para el PVR.
const alineacionTramo = 8

// alinear: los bytes que ocupa un trozo de `n` una vez repartido (los trozos van alineados a 8).
func alinear(n int) int {
	if n <= 0 {
		return 0
	}

	return (n + alineacionTramo - 1) / alineacionTramo * alineacionTramo
}

// tramoDelPoc: un bloque del monton de C con nombre y dueno. `Datos` es la vista de Go sobre el bloque; los trozos
// que se reparten son rebanadas de ahi y NO se copian.
type tramoDelPoc struct {
	Nombre string
	Dueno  string
	Bytes  int
	dir    uintptr
	Datos  []byte
	usados int // cuantos bytes se repartieron (para el censo y para no pisar lo entregado)
	Trozos int // cuantos trozos se cortaron
}

// tramosDelPoc: el registro de los tramos vivos del POC. Es de donde sale la linea del censo (nombre, dueno, bytes,
// libre) y la unica forma de saber que quedo sin devolver.
var tramosDelPoc []*tramoDelPoc

// pedirTramoDelPoc: pide un tramo del monton de C. Devuelve nil (y lo dice) si el monton no tiene lugar: el que
// llama se queda con el camino de siempre (el semiespacio), no revienta.
func pedirTramoDelPoc(nombre, dueno string, bytes int) *tramoDelPoc {
	if bytes <= 0 {
		return nil
	}

	alineado := alinear(bytes)

	dir := d2cTramoPedir(uint(alineado))
	if dir == 0 {
		println("JUEGO119: TRAMO SIN-LUGAR nombre=", nombre, "bytes=", alineado, "libre=", int(d2cHeapLibre()))

		return nil
	}

	t := &tramoDelPoc{
		Nombre: nombre,
		Dueno:  dueno,
		Bytes:  alineado,
		dir:    dir,
		// la vista de Go sobre el bloque de C: un puntero FUERA del semiespacio, que es todo el punto
		Datos: (*[1 << 30]byte)(unsafe.Pointer(dir))[:alineado:alineado],
	}

	tramosDelPoc = append(tramosDelPoc, t)

	println("JUEGO119: TRAMO PEDIDO nombre=", nombre, "dueno=", dueno, "bytes=", alineado,
		"dir=", int(dir), "montonLibre=", int(d2cHeapLibre()))

	return t
}

// repartir: corta `bytes` del tramo y los devuelve. Es el reparto de la cache de tiles: un bloque, muchos trozos.
func (t *tramoDelPoc) repartir(bytes int) []byte {
	if t == nil || bytes <= 0 {
		return nil
	}

	alineado := alinear(bytes)
	if t.usados+alineado > len(t.Datos) {
		println("JUEGO119: TRAMO SIN-HUECO nombre=", t.Nombre, "pedido=", alineado, "usados=", t.usados,
			"bytes=", t.Bytes)

		return nil
	}

	trozo := t.Datos[t.usados : t.usados+bytes]
	t.usados += alineado
	t.Trozos++

	return trozo
}

// liberar: devuelve el bloque al monton de C. Despues de esto NINGUN trozo se puede tocar. Idempotente.
func (t *tramoDelPoc) liberar() int {
	if t == nil || t.dir == 0 {
		return 0
	}

	bytes := t.Bytes
	d2cTramoSoltar(t.dir)
	t.dir = 0
	t.Datos = nil

	for i := range tramosDelPoc {
		if tramosDelPoc[i] == t {
			tramosDelPoc = append(tramosDelPoc[:i], tramosDelPoc[i+1:]...)

			break
		}
	}

	println("JUEGO119: TRAMO SOLTADO nombre=", t.Nombre, "bytes=", bytes, "montonLibre=", int(d2cHeapLibre()))

	return bytes
}

// ============================== EL CENSO DE LOS TRAMOS ==============================

// censoDeLosTramos: una linea por tramo vivo (nombre, dueno, bytes pedidos, repartidos y trozos) y el total.
func censoDeLosTramos() (vivos, bytes int) {
	for _, t := range tramosDelPoc {
		println("JUEGO119: TRAMO VIVO nombre=", t.Nombre, "dueno=", t.Dueno, "bytes=", t.Bytes,
			"repartidos=", t.usados, "trozos=", t.Trozos)

		vivos++
		bytes += t.Bytes
	}

	println("JUEGO119: TRAMO CENSO vivos=", vivos, "bytes=", bytes, "montonLibre=", int(d2cHeapLibre()),
		"montonUsado=", d2cMontonUsado())

	return vivos, bytes
}

// ============================== LAS CATEGORIAS QUE SE MUDAN ==============================
// mudarLosCuadrosDelJugador: LA CATEGORIA MAS GORDA DE LAS QUE NO NECESITAN SER PUNTERO DE GO.
//
// Los cuadros compuestos del jugador son los `.dcc` decodificados y pegados: el ciclo QUIETO (NU, 8 cuadros) y los
// OCHO ciclos de caminar (TW, 8 cuadros cada uno) = 72 cuadros de indices de 8 bits, 165.152 B medidos en el log
// (`JUGADOR CICLOS ... indicesTotales=`). Cada cuadro pesa ~2 KB, o sea MUY por debajo de los 64 KB que mandan una
// asignacion al monton de C: hoy los 72 viven en el semiespacio y los paga el recolector en cada copia.
//
// LA MUDANZA: se pide UN tramo del total, se copian los pixeles adentro y se repunta cada `Cuadro.Pixeles` al trozo.
// Los bytes son los mismos (el crc32 de cada cuadro lo verifica) y el dueno pasa a ser este tramo. Las copias de Go
// quedan sin referencia: el recolector las tira en la primera recoleccion.
//
// OJO CON EL ORDEN: la arena del decodificador ya se libero cuando esto corre (`componerModo` la libera en cuanto
// los pixeles estan pegados en cada compuesto), asi que lo que se copia son los PIXELES COMPUESTOS, no la memoria
// del decodificador. Y despues de mudarlos, el tramo NO se puede soltar mientras la corrida dibuje: por eso se
// suelta al final, cuando el ciclo del PVR ya salio.
func mudarLosCuadrosDelJugador() int {
	if !tramosPropios {
		println("JUEGO119: TRAMO jugador-cuadros APAGADO (tramosPropios=false): los 72 cuadros quedan en el semiespacio")

		return 0
	}

	total := 0
	alineado := 0
	cuantos := 0

	if jugadorQuieto != nil {
		total += bytesDeLosCuadros(jugadorQuieto)
		alineado += bytesAlineadosDeLosCuadros(jugadorQuieto)
		cuantos += len(jugadorQuieto.Cuadros)
	}

	for _, c := range jugadoresCaminando {
		total += bytesDeLosCuadros(c)
		alineado += bytesAlineadosDeLosCuadros(c)
		cuantos += len(c.Cuadros)
	}

	if total == 0 {
		println("JUEGO119: TRAMO jugador-cuadros NADA-QUE-MUDAR")

		return 0
	}

	t := pedirTramoDelPoc("jugador-cuadros", "la cache de cuadros del jugador (NU + las 8 de TW)", alineado)
	if t == nil {
		return 0
	}

	copiados := 0
	copiados += t.mudarCiclo(jugadorQuieto)

	for _, c := range jugadoresCaminando {
		copiados += t.mudarCiclo(c)
	}

	println("JUEGO119: TRAMO jugador-cuadros MUDADO cuadros=", cuantos, "bytes=", copiados,
		"tramoBytes=", t.Bytes, "enElSemiespacioAntes=", total, "identicos=", copiados == total)

	return copiados
}

// bytesAlineadosDeLosCuadros: lo mismo que `bytesDeLosCuadros` pero con cada cuadro alineado a 8, que es lo que el
// tramo tiene que medir para que TODOS los trozos entren (sin esto el ultimo cuadro se queda afuera).
func bytesAlineadosDeLosCuadros(c *CicloDelJugador) int {
	if c == nil {
		return 0
	}

	total := 0

	for i := range c.Cuadros {
		total += alinear(len(c.Cuadros[i].Pixeles))
	}

	return total
}

// mudarCiclo: copia los cuadros de UN ciclo al tramo y repunta cada `Pixeles`. Devuelve los bytes copiados.
func (t *tramoDelPoc) mudarCiclo(c *CicloDelJugador) int {
	if c == nil {
		return 0
	}

	copiados := 0

	for i := range c.Cuadros {
		cu := &c.Cuadros[i]

		l := len(cu.Pixeles)
		if l == 0 {
			continue
		}

		trozo := t.repartir(l)
		if trozo == nil {
			continue
		}

		copy(trozo, cu.Pixeles)
		cu.Pixeles = trozo
		copiados += l
	}

	return copiados
}

// cajaMasGrandeDe: el peor cuadro de los que ese jugador puede dibujar (en bytes de la caja: ancho*alto*4). Es el
// tamano con el que se reserva su caja de guardado, para que nunca tenga que pedir una mas grande a mitad del ciclo.
func cajaMasGrandeDe() int {
	peor := 0

	peor = peorCuadro(jugadorQuieto, peor)

	for _, c := range jugadoresCaminando {
		peor = peorCuadro(c, peor)
	}

	return peor * 4
}

func peorCuadro(c *CicloDelJugador, peor int) int {
	if c == nil {
		return peor
	}

	for i := range c.Cuadros {
		cu := &c.Cuadros[i]

		if n := cu.Ancho * cu.Alto; n > peor {
			peor = n
		}
	}

	return peor
}

// mudarLasCajasDeGuardado: LA SEGUNDA CATEGORIA. Guardar la caja de un jugador es copiar los pixeles que habia
// debajo (4 B por pixel) para devolverlos despues; con cuatro jugadores son cuatro buffers de ~8,8 KB cada uno
// (28x79x4 medido en el cuadro 1), unos 35 KB de vivos que tampoco son punteros de Go.
//
// Se reserva UN tramo con el peor cuadro de cada jugador y se le deja la rebanada puesta a cada participante: el
// `guardarCajaDe` de siempre la reusa (solo pide una nueva si el cuadro no le entra, y el tramo se pide con el peor
// caso justamente para que no pase).
func mudarLasCajasDeGuardado() int {
	cada := alinear(cajaMasGrandeDe())

	if len(jugadores) == 0 || cada <= 0 {
		println("JUEGO119: TRAMO jugador-cajas NADA-QUE-MUDAR")

		return 0
	}

	// SIN TRAMOS, LAS CAJAS SE RESERVAN IGUAL, PERO EN EL SEMIESPACIO. Es lo que hace que el A/B de la tarjeta
	// compare lo mismo: el mismo instante (ya con los cuatro jugadores armados) y el mismo tamano reservado, con la
	// unica diferencia de DONDE vive. Sin esto la corrida sin tramos no tendria cajas en ese punto (las pide el
	// primer cuadro dibujado, o sea despues del censo) y la comparacion se comeria esa categoria.
	if !tramosPropios {
		reservadas := 0

		for _, p := range jugadores {
			p.CajaGuardada = make([]byte, cada)
			reservadas += cada
		}

		println("JUEGO119: TRAMO jugador-cajas EN-EL-SEMIESPACIO (tramosPropios=false) jugadores=", len(jugadores),
			"cada=", cada, "bytes=", reservadas)

		return 0
	}

	total := cada * len(jugadores)

	t := pedirTramoDelPoc("jugador-cajas", "la caja de guardado de cada jugador (4 B/pixel)", total)
	if t == nil {
		return 0
	}

	mudadas := 0

	for _, p := range jugadores {
		caja := t.repartir(cada)
		if caja == nil {
			continue
		}

		p.CajaGuardada = caja
		mudadas += len(caja)
	}

	println("JUEGO119: TRAMO jugador-cajas MUDADO jugadores=", len(jugadores), "cada=", cada, "bytes=", mudadas,
		"tramoBytes=", t.Bytes)

	return mudadas
}

// ============================== EL DUENO DEVUELVE ==============================

// soltarLosTramosDelPoc: suelta TODOS los tramos vivos y devuelve cuantos bytes volvieron al monton de C. Es la
// otra mitad del contrato de la decision 1b: pedir sin devolver es una fuga, solo que mas dificil de ver que un
// `make` sin dueno. Se llama al cerrar la corrida (el ultimo cuadro), cuando nadie dibuja.
func soltarLosTramosDelPoc() int {
	if len(tramosDelPoc) == 0 {
		println("JUEGO119: TRAMO DEVUELTO ninguno= true bytes= 0")

		return 0
	}

	total := 0
	vivos := len(tramosDelPoc)

	for len(tramosDelPoc) > 0 {
		t := tramosDelPoc[len(tramosDelPoc)-1]
		total += t.liberar()
	}

	println("JUEGO119: TRAMO DEVUELTO tramos=", vivos, "bytes=", total, "montonLibre=", int(d2cHeapLibre()),
		"montonUsado=", d2cMontonUsado())

	return total
}

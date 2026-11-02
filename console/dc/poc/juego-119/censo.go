//go:build dreamcast

package main

// DC-002.119 -- EL CENSO DEL SEMIESPACIO, POR ORIGEN Y CON NUMEROS.
//
// POR QUE EXISTE. DC-002.117 midio el TECHO del cooperativo: con cuatro jugadores el semiespacio del recolector
// queda al 93,5 % (1.716.240 de 1.835.008 B). Ese numero dice que el techo no son los jugadores (5.184 B los
// cuatro) sino lo que vive adentro del semiespacio, pero NO dice que es. Esta tarjeta necesita el censo por
// categoria para poder sacar de ahi las categorias gordas con la decision 1b del ADR 0001.
//
// COMO SE MIDE, Y POR QUE ASI. `d2c_gc_usado` son los bytes que el recolector tiene tomados (alloc_ptr menos el
// arranque del semiespacio activo): eso incluye la basura que todavia no se recolecto. Para que el numero sea VIVO
// -- y comparable entre dos corridas -- el censo hace `runtime.GC()` antes de leer, en ocho puntos del arranque
// (ver main.go). Lo que agrega cada etapa es la DIFERENCIA contra la etapa anterior, y esa diferencia es la
// categoria.
//
// LOS DOS INSTRUMENTOS, y por que son dos:
//   1. CENSO (vivo, con recoleccion a pedido): el numero del recolector en cada etapa. Es el que dice cuanto pesa
//      cada origen, incluido lo que el motor pide y el POC no puede contar por su cuenta.
//   2. CENSO CONTADO: lo que el POC SI sabe contar sin adivinar (los bytes de los cuadros compuestos del jugador,
//      las cajas de guardado, la cache de imagenes del renderer, los tramos propios). Sirve de contraste: si la
//      etapa 30 del censo no coincide con los bytes contados del jugador, el instrumento esta mintiendo.
//
// OJO CON LA RECOLECCION A PEDIDO: se hace SOLO en el arranque, nunca en el ciclo del PVR (knobs.go, `gcCadaCuadros`:
// pedirle un GC al recolector desde el callback del ciclo lo CLAVA, medido en DC-002.113). Lo que se mide del ciclo
// es el pico SIN recoleccion a pedido, leyendo `gc_stats` una vez por cuadro, que no cuesta nada.

import "runtime"

// lineaDelCenso: una etapa del censo. `Vivo` es lo que el recolector tiene tomado DESPUES de recolectar.
type lineaDelCenso struct {
	Etapa string
	Vivo  int
	Total int
	Col   int
}

var censoEtapas []lineaDelCenso

// picoDelSemiespacio: el mayor `gc_stats(usado)` visto cuadro a cuadro desde que arranca el ciclo. Es el numero que
// se compara con el 93,5 % de DC-002.117: a diferencia del censo, este NO recolecta antes de leer.
var picoDelSemiespacio int

// censoDelSemiespacio: recolecta y anota lo vivo en esa etapa. Devuelve los bytes vivos. (No se llama `censo` porque
// `censo(engine)` es la cuenta de la grilla del pueblo, que ya existia.)
func censoDelSemiespacio(etapa string) int {
	runtime.GC()

	vivo := int(d2cGcUsado())
	total := int(d2cGcTotal())
	col := int(d2cGcColecciones())

	pct := 0
	if total > 0 {
		pct = vivo * 100 / total
	}

	censoEtapas = append(censoEtapas, lineaDelCenso{Etapa: etapa, Vivo: vivo, Total: total, Col: col})

	if vivo > picoDelSemiespacio {
		picoDelSemiespacio = vivo
	}

	println("JUEGO119: CENSO", etapa, "vivo=", vivo, "total=", total, "pct=", pct, "colecciones=", col)

	return vivo
}

// censoDeLaEtapa: el censo de una etapa del arranque, respetando la palanca `censoPorEtapas` (knobs.go). El
// instrumento se puede apagar sin sacar las llamadas, que es lo que deja el A/B del propio censo.
func censoDeLaEtapa(etapa string) int {
	if !censoPorEtapas {
		return 0
	}

	return censoDelSemiespacio(etapa)
}

// censoActual: lo que el recolector tiene tomado AHORA, sin recolectar. Es la unica lectura que se puede hacer
// dentro del ciclo del PVR (un `runtime.GC()` desde el callback clava el ciclo, medido en DC-002.113), asi que el
// cierre de la corrida usa esta y no `censo`.
func censoActual(etapa string) int {
	vivo := int(d2cGcUsado())
	total := int(d2cGcTotal())

	pct := 0
	if total > 0 {
		pct = vivo * 100 / total
	}

	if vivo > picoDelSemiespacio {
		picoDelSemiespacio = vivo
	}

	println("JUEGO119: CENSO", etapa, "vivo=", vivo, "total=", total, "pct=", pct,
		"colecciones=", int(d2cGcColecciones()), "sinRecolectar=true")

	return vivo
}

// censoPorOrigen: la tabla del censo, con lo que AGREGO cada etapa (la diferencia contra la anterior). Es la tabla
// de la tarjeta: cada renglon es una categoria.
func censoPorOrigen() {
	println("JUEGO119: CENSO ORIGEN etapas=", len(censoEtapas))

	anterior := 0

	for i := range censoEtapas {
		l := &censoEtapas[i]

		pct := 0
		if l.Total > 0 {
			pct = l.Vivo * 100 / l.Total
		}

		println("JUEGO119: CENSO FILA", l.Etapa, "vivo=", l.Vivo, "agregado=", l.Vivo-anterior, "pct=", pct)

		anterior = l.Vivo
	}
}

// censoDelCuadro: el pico del semiespacio cuadro a cuadro, SIN recoleccion a pedido (el ciclo no la aguanta).
func censoDelCuadro() {
	usado := int(d2cGcUsado())
	if usado > picoDelSemiespacio {
		picoDelSemiespacio = usado
	}
}

// censoContado: lo que el POC cuenta por su cuenta, categoria por categoria. Es el contraste del censo del
// recolector y el que dice CUANTO se mudo de verdad a los tramos.
func censoContado() {
	// 1. los cuadros compuestos del jugador: los indices de 8 bits de NU + las 8 direcciones de TW
	cuadros := bytesDeLosCuadros(jugadorQuieto)

	for _, c := range jugadoresCaminando {
		cuadros += bytesDeLosCuadros(c)
	}

	println("JUEGO119: CENSO CONTADO jugadorCuadrosBytes=", cuadros,
		"jugadorCuadrosEnTramo=", cuadrosEnTramo(jugadorQuieto)+cuadrosEnTramoDeLosCaminando(),
		"direcciones=", len(jugadoresCaminando))

	// 2. las cajas de guardado (una por jugador)
	cajas := 0
	jugadoresConCaja := 0

	for _, p := range jugadores {
		if p.CajaGuardada != nil {
			cajas += len(p.CajaGuardada)
			jugadoresConCaja++
		}
	}

	println("JUEGO119: CENSO CONTADO jugadorCajasBytes=", cajas, "cajas=", jugadoresConCaja,
		"cadaUna=", cajaMasGrandeDe())

	// 3. el estado y la estructura por jugador (lo unico que de verdad se paga por jugador)
	println("JUEGO119: CENSO CONTADO estadoPorJugadorBytes=", estadoDeUnJugador(),
		"estructuraPorJugadorBytes=", tamanoDeParticipante, "jugadores=", len(jugadores),
		"totalJugadoresBytes=", (estadoDeUnJugador()+tamanoDeParticipante)*len(jugadores))

	// 4. la cache de tiles: el numero que prueba que esta FUERA del semiespacio (tramo de la cache en VRAM)
	if rendererDelMapa != nil {
		e := rendererDelMapa.EstadoDeLaCache()
		t := rendererDelMapa.CensoDelTramo()

		println("JUEGO119: CENSO CONTADO cacheImagenes=", e.Superficies, "cacheBytes=", e.Bytes,
			"cacheEnIndices=", e.EnIndices, "tramoActivo=", t.Activa, "tramoOrigen=", t.Origen,
			"tramoBytes=", t.Bytes, "tramoUsado=", t.Usado, "tramoServidas=", t.Imagenes,
			"tramoDerrames=", t.Derrames, "bytesDerramados=", t.BytesDerramados, "desalojos=", t.Desalojos)
	}

	// 5. los tramos con dueno: lo que el POC saco del semiespacio
	vivos, bytes := censoDeLosTramos()

	println("JUEGO119: CENSO CONTADO tramosVivos=", vivos, "tramosBytes=", bytes)
}

// cuadrosEnTramo: cuantos bytes de un ciclo apuntan FUERA del semiespacio (los que se mudaron al tramo).
func cuadrosEnTramo(c *CicloDelJugador) int {
	if c == nil {
		return 0
	}

	n := 0

	for i := range c.Cuadros {
		if len(c.Cuadros[i].Pixeles) > 0 && enUnTramo(c.Cuadros[i].Pixeles) {
			n += len(c.Cuadros[i].Pixeles)
		}
	}

	return n
}

func cuadrosEnTramoDeLosCaminando() int {
	n := 0

	for _, c := range jugadoresCaminando {
		n += cuadrosEnTramo(c)
	}

	return n
}

// enUnTramo: si la rebanada cae adentro de alguno de los tramos vivos. Se compara por rango de direcciones, que es
// la unica forma de saberlo desde Go (el bloque no es del recolector).
func enUnTramo(b []byte) bool {
	desde := uintptrOf(b)
	hasta := desde + uintptr(len(b))

	for _, t := range tramosDelPoc {
		ini := uintptrOf(t.Datos)
		fin := ini + uintptr(len(t.Datos))

		if desde >= ini && hasta <= fin {
			return true
		}
	}

	return false
}

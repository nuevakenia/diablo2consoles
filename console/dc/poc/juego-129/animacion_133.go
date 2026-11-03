//go:build dreamcast

// DC-002.133 -- EL CONTROL MANEJA LA ANIMACION: que cuadro se dibuja, que modo lo pide y la prueba de que cambiar
// de direccion NO reinicia la animacion.
//
// QUE HACE ESTE ARCHIVO, Y QUE NO HACE. No toca ningun reloj ni ninguna tabla del motor: se ENGANCHA donde el POC
// ya decide el cuadro que se pinta (`cicloYCuadroDelJugador` para la amazona, `cuadroDelBicho`/`cuadroDelModoDelGolpe`
// para el Returned) y anota, por personaje y por cuadro del POC:
//
//   - el MODO de la animacion tal como lo declara el COF (NU quieto, TW caminando, A1 el golpe, GH/DT/DD del bicho),
//   - la DIRECCION del COF,
//   - la FASE: cuantos cuadros de animacion pasaron desde que empezo el modo (`MsDelModo / MsPorCuadro`),
//   - el CUADRO que se dibujo (la fase modulo el largo del ciclo) y el largo del ciclo.
//
// CON ESO SE RESPONDEN LAS TRES PREGUNTAS DE LA TARJETA, sin creerle a nadie:
//
//  1. "la animacion corresponde a lo que se aprieta": por cada direccion de la cruceta que se jugo, cuantos cuadros
//     DISTINTOS de ese ciclo se dibujaron (si una direccion se dibujo con el ciclo de otra, o con el quieto, los
//     cuadros distintos de la direccion que se pidio no aparecen y el que aparece es el mismo de siempre).
//  2. "no se corta al cambiar de direccion": cada giro queda anotado con la FASE ANTES y la FASE DESPUES. La fase
//     es un contador que solo crece mientras el modo sea el mismo; si al girar el reloj se reiniciara, la fase
//     despues seria MENOR que la de antes y el giro quedaria marcado `salto=true`. El resumen cuenta los saltos.
//  3. "el estado por jugador": la linea por cuadro dice, de cada personaje, modo, direccion, cuadro, ciclo y fase.
//
// EL MODO ES EL NOMBRE QUE TRAE EL CICLO, no una etiqueta de aca: `CicloDelJugador.Modo` sale del nombre del COF
// que se compuso (`amtwhth.cof` -> TW), asi que si el POC dibuja otra cosa, la linea lo dice.
package main

const (
	// animModosMax: los modos que puede tener un personaje en este POC (NU, TW, A1 en la amazona; TW, GH, DT, DD
	// en el Returned). Con 4 alcanza para los dos y no hace falta un mapa (un mapa reserva).
	animModosMax = 4

	// animCuadrosMax: el ciclo mas largo que se dibuja (el A1 tiene 13 cuadros, DT 19, TW 8). 32 deja lugar.
	animCuadrosMax = 32

	// animGirosMax: cuantos giros se guardan con nombre y apellido para el informe. Los que sobran se cuentan
	// (`girosPerdidos`) pero no se detallan: el informe no puede crecer sin limite.
	animGirosMax = 32

	// animIndiceBicho: el medidor del PRIMER bicho vive en el mismo arreglo que los jugadores, en el indice que
	// sigue al ultimo puerto del bus.
	animIndiceBicho = maxJugadoresCoop

	// cuadrosDelInforme133: cada cuantos cuadros del POC se deja la linea de estado de la animacion del bicho (la de
	// los jugadores sale ademas en cada cambio de modo o de direccion). Con el cuadro a ~500 ms, 10 cuadros son 5 s.
	cuadrosDelInforme133 = 10
)

// giroDeAnimacion: un cambio de direccion, con lo que hace falta para saber si la animacion se corto.
type giroDeAnimacion struct {
	De          int   // la direccion del COF que se estaba dibujando
	A           int   // la direccion nueva
	FaseAntes   int64 // la fase del modo en el ultimo cuadro dibujado con `De`
	FaseDespues int64 // la fase en el primer cuadro dibujado con `A`
	MsDelModo   int64 // ms desde que empezo el modo (no desde el giro)
	Salto       bool  // la fase retrocedio: el reloj del modo se reinicio al girar (NO tiene que pasar)
}

// medidorDeAnimacion: lo que se anota de UN personaje.
type medidorDeAnimacion struct {
	Quien string

	// el estado del cuadro que se esta dibujando
	Modo   string
	Dir    int
	Fase   int64
	Cuadro int
	Total  int

	// la celda del motor donde esta el personaje en ese cuadro (la pone el que anota: jugador o bicho)
	CeldaX, CeldaY float64

	// el cuadro del POC en el que se anoto por ultima vez: el dibujo pasa DOS veces por el mismo jugador en el
	// mismo cuadro del POC (una al preparar y otra al pintar), y esto hace que se anote una sola vez.
	CuadroDelPOC int

	// lo que se cuenta
	Modos              int // modos distintos jugados
	CambiosDeModo      int
	CambiosDeDireccion int
	Saltos             int // giros con la fase reiniciada (el numero que tiene que quedar en 0)
	Volteos            int // veces que el ciclo dio la vuelta
	CuadrosDibujados   int
	CuantasDirecciones int
	CuadrosDistintos   [animModosMax]int
	TotalDelModo       [animModosMax]int
	Vistos             [animModosMax][animCuadrosMax]bool
	NombreDelModo      [animModosMax]string
	Direcciones        [16]bool
	Giros              [animGirosMax]giroDeAnimacion
	CuantosGiros       int
	GirosPerdidos      int

	// el reloj del modo y el reloj de la consola
	InicioDelModo  int64
	Primero        int64
	Ultimo         int64
	CuadroAnterior int

	// Anunciados: cuantos cambios de modo+direccion ya se imprimieron (la linea por cuadro dice lo NUEVO)
	Anunciados int
}

// msDelModoDelPOC: los ms por cuadro con los que este POC compone cada modo (`CicloDelJugador.MsPorCuadro`). Se
// usan solo para fechar el arranque del modo cuando el que llama ya trae la fase contada; si el modo no esta en la
// tabla se devuelve 0 y la fase que se informa es la del que llama, que es la que vale.
func msDelModoDelPOC(modo string) int {
	switch modo {
	case modoQuieto:
		return msQuieto
	case modoCaminando:
		return msCaminando
	case modoGolpePersonaje:
		return msAtaqueJugador
	case modoBichoCamina:
		return msBichoCamina
	case modoBestiaGolpeada:
		return msBichoGolpeado
	case modoBestiaMuriendo:
		return msBichoMuriendo
	case modoBestiaMuerta:
		return msBichoMuerta
	}

	return 0
}

// medidoresDeAnimacion: uno por jugador y uno por el primer bicho (el que pelea). Se reservan la primera vez que
// hacen falta: si una corrida no tiene bichos, no se reserva el suyo.
var medidoresDeAnimacion [maxJugadoresCoop + 1]*medidorDeAnimacion

// medidorDe: el medidor del indice pedido (jugador 0..3, o animIndiceBicho), reservado si no existe.
func medidorDe(indice int, quien string) *medidorDeAnimacion {
	if indice < 0 || indice > maxJugadoresCoop {
		return nil
	}

	if medidoresDeAnimacion[indice] == nil {
		medidoresDeAnimacion[indice] = &medidorDeAnimacion{Quien: quien, Dir: -1, CuadroAnterior: -1}
	}

	return medidoresDeAnimacion[indice]
}

// medidorDelJugador / medidorDelBicho: los dos atajos del que anota.
func medidorDelJugador(i int) *medidorDeAnimacion {
	if i < 0 || i >= len(jugadores) {
		return nil
	}

	return medidorDe(i, jugadores[i].Nombre)
}

func medidorDelBicho() *medidorDeAnimacion { return medidorDe(animIndiceBicho, "BICHO") }

// indiceDelModo: el lugar del modo en las tablas del medidor (se asignan en el orden en que aparecen).
func (m *medidorDeAnimacion) indiceDelModo(modo string) int {
	for i := 0; i < animModosMax; i++ {
		if m.NombreDelModo[i] == modo {
			return i
		}

		if m.NombreDelModo[i] == "" {
			m.NombreDelModo[i] = modo
			m.Modos++

			return i
		}
	}

	return animModosMax - 1
}

// Anotar: el cuadro que se dibujo de este personaje, en este cuadro del POC.
//
//	cuadroDelPOC  el numero de cuadro del POC (para no anotar dos veces el mismo)
//	modo          el modo del ciclo (`CicloDelJugador.Modo`: NU, TW, A1, GH, ...)
//	dir           la direccion del COF
//	cuadro        el cuadro del ciclo que se dibujo (0..total-1)
//	total         cuantos cuadros tiene el ciclo
//	fase          cuantos cuadros de animacion pasaron desde que empezo el MODO
//	ahora         el ms de la consola
func (m *medidorDeAnimacion) Anotar(cuadroDelPOC int, modo string, dir, cuadro, total int, fase int64, ahora int64) {
	if m == nil || total <= 0 {
		return
	}

	if m.CuadroDelPOC == cuadroDelPOC {
		return // ya se anoto en este cuadro del POC (el dibujo pasa dos veces)
	}

	m.CuadroDelPOC = cuadroDelPOC

	if m.Primero == 0 {
		m.Primero = ahora
	}

	switch {
	case m.Modo == "":
		// el primer cuadro de la corrida: arranca el modo y su reloj (`indiceDelModo` lo cuenta en `Modos`)
		m.InicioDelModo = ahora
	case modo != m.Modo:
		// LA REGLA DEL MOTOR: una animacion nueva empieza en su cuadro 0 (`Animation.SetMode`), asi que el reloj
		// del modo arranca de cero y eso NO es un corte: es el cambio de animacion. La fase de ESTE cuadro sale
		// del reloj nuevo del que llama (el golpe la cuenta desde que empezo el golpe).
		m.CambiosDeModo++
		m.InicioDelModo = ahora - fase*int64(msDelModoDelPOC(modo))
		m.CuadroAnterior = -1
	case dir != m.Dir:
		// LA TRANSICION LIMPIA: cambiar de direccion NO toca el reloj del modo. La fase que se estaba viendo
		// sigue, y el cuadro nuevo es el de la direccion nueva en ESA fase. Se guarda el giro con las dos fases
		// para que el informe lo pueda mostrar en vez de jurarlo.
		m.CambiosDeDireccion++

		g := giroDeAnimacion{
			De: m.Dir, A: dir, FaseAntes: m.Fase, FaseDespues: fase, MsDelModo: ahora - m.InicioDelModo,
			Salto: fase < m.Fase,
		}

		if g.Salto {
			m.Saltos++
		}

		if m.CuantosGiros < animGirosMax {
			m.Giros[m.CuantosGiros] = g
			m.CuantosGiros++
		} else {
			m.GirosPerdidos++
		}

		m.CuadroAnterior = -1 // el cuadro de otra direccion no se compara con este
	}

	m.Modo = modo
	m.Dir = dir
	m.Fase = fase
	m.Cuadro = cuadro
	m.Total = total

	// los cuadros DISTINTOS de cada modo: es el numero que dice si la animacion se vio de verdad
	indice := m.indiceDelModo(modo)
	m.TotalDelModo[indice] = total

	if cuadro >= 0 && cuadro < animCuadrosMax && !m.Vistos[indice][cuadro] {
		m.Vistos[indice][cuadro] = true
		m.CuadrosDistintos[indice]++
	}

	// los volteos: el cuadro que se dibujo es menor que el anterior sin haber cambiado de modo ni de direccion
	if m.CuadroAnterior >= 0 && cuadro < m.CuadroAnterior {
		m.Volteos++
	}

	m.CuadroAnterior = cuadro
	m.CuadrosDibujados++

	if dir >= 0 && dir < 16 && !m.Direcciones[dir] {
		m.Direcciones[dir] = true
		m.CuantasDirecciones++
	}

	m.Ultimo = ahora
}

// resumenDeLaAnimacionHecho: el resumen se imprime UNA vez, cuando el guion termino.
var resumenDeLaAnimacionHecho bool

// ============================== LO QUE SE IMPRIME ==============================

// lineaDeLaAnimacion: el estado de un personaje en este cuadro del POC (modo, direccion, cuadro del ciclo y fase).
func lineaDeLaAnimacion(m *medidorDeAnimacion, cuadroDelPOC int) {
	if m == nil || m.Modo == "" {
		return
	}

	println("JUEGO133: ANIM cuadro=", cuadroDelPOC, "quien=", m.Quien, "modo=", m.Modo, "dir=", m.Dir,
		"cuadroDelCiclo=", m.Cuadro, "de=", m.Total, "fase=", m.Fase, "celda=", m.CeldaX, m.CeldaY,
		"cambiosDeDireccion=", m.CambiosDeDireccion, "saltos=", m.Saltos)
}

// estadoDeLaAnimacion: la linea por cuadro de CADA personaje. Se imprime cuando hay algo nuevo (cambio de modo o de
// direccion) y ademas cada `cada` cuadros: con los dos personajes animados y el cuadro a ~500 ms, una linea por
// cuadro llena el puerto serie sin decir nada nuevo.
func estadoDeLaAnimacion(cuadroDelPOC, cada int) {
	for i := range jugadores {
		m := medidoresDeAnimacion[i]
		if m == nil {
			continue
		}

		if cambio := m.CambiosDeModo + m.CambiosDeDireccion; cambio != m.Anunciados {
			m.Anunciados = cambio
			lineaDeLaAnimacion(m, cuadroDelPOC)

			continue
		}

		if cada > 0 && cuadroDelPOC%cada == 0 {
			lineaDeLaAnimacion(m, cuadroDelPOC)
		}
	}

	if m := medidoresDeAnimacion[animIndiceBicho]; m != nil {
		if cada > 0 && cuadroDelPOC%cada == 0 {
			lineaDeLaAnimacion(m, cuadroDelPOC)
		}
	}
}

// resumenDeLaAnimacion133: los numeros de la tarjeta, por personaje.
func resumenDeLaAnimacion133() {
	total := 0

	for i := 0; i <= maxJugadoresCoop; i++ {
		if medidoresDeAnimacion[i] != nil {
			total++
		}
	}

	println("JUEGO133: ANIM RESUMEN personajes=", total, "jugadores=", len(jugadores), "bichos=", len(bichos))

	for i := 0; i <= maxJugadoresCoop; i++ {
		if medidoresDeAnimacion[i] != nil {
			resumenDeUnPersonaje(medidoresDeAnimacion[i])
		}
	}
}

// resumenDeUnPersonaje: un personaje. Dos lineas: los numeros y los giros uno por uno.
func resumenDeUnPersonaje(m *medidorDeAnimacion) {
	segundos := 0.0
	porSegundo := int64(0)

	if m.Ultimo > m.Primero {
		segundos = float64(m.Ultimo-m.Primero) / 1000.0

		// el cuadro del POC por segundo, en centesimas como el `fpsX100` de PVR132: cada cuadro dibujado es UNA
		// pose de la animacion, asi que este numero es a la vez el fps del POC y las poses por segundo que se ven.
		porSegundo = int64(m.CuadrosDibujados) * 100000 / (m.Ultimo - m.Primero)
	}

	porModo := ""

	for i := 0; i < animModosMax; i++ {
		if m.NombreDelModo[i] == "" {
			continue
		}

		if porModo != "" {
			porModo += " "
		}

		porModo += m.NombreDelModo[i] + ":" + itoa(m.CuadrosDistintos[i]) + "/" + itoa(m.TotalDelModo[i])
	}

	println("JUEGO133: ANIM PERSONAJE quien=", m.Quien, "modos=", m.Modos, "modoFinal=", m.Modo,
		"cambiosDeModo=", m.CambiosDeModo, "cambiosDeDireccion=", m.CambiosDeDireccion,
		"saltosDeFase=", m.Saltos, "volteos=", m.Volteos, "cuadrosDibujados=", m.CuadrosDibujados,
		"segundos=", segundos, "cuadrosDelPOCPorSegundoX100=", porSegundo,
		"direcciones=", m.CuantasDirecciones, "cuadrosDistintosPorModo=", porModo, "girosPerdidos=", m.GirosPerdidos)

	// la lista de direcciones que se dibujaron, para leerla de un vistazo
	lista := ""

	for d := 0; d < 16; d++ {
		if !m.Direcciones[d] {
			continue
		}

		if lista != "" {
			lista += " "
		}

		lista += itoa(d)
	}

	println("JUEGO133: ANIM DIRECCIONES quien=", m.Quien, "delCOF=", lista, "cuantas=", m.CuantasDirecciones)

	for i := 0; i < m.CuantosGiros; i++ {
		g := m.Giros[i]

		println("JUEGO133: ANIM GIRO quien=", m.Quien, "giro=", i+1, "de=", g.De, "a=", g.A,
			"faseAntes=", g.FaseAntes, "faseDespues=", g.FaseDespues, "msDelModo=", g.MsDelModo,
			"salto=", g.Salto)
	}
}

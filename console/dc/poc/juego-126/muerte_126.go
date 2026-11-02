//go:build dreamcast

// DC-002.126 -- LA MUERTE SE VE: EL DT CUADRO A CUADRO, EL DD COMPUESTO Y EL CADAVER QUE EXPIRA.
//
// QUE HACE ESTE ARCHIVO. DC-002.122 dejo el golpe andando (la vida baja, el bicho pasa a GH, muere en DT y termina
// en DD), pero la muerte NO SE VEIA: con el cuadro del POC a ~1,3 s y el DT de 19 cuadros a 49 ms (935 ms), el
// reloj de pared muestrea la caida UNA vez (y clava el cuadro en el ultimo), asi que en pantalla el bicho pasa de
// parado a cadaver de un salto. Aca entra el reloj de la muerte, y con el, las tres cosas que la tarjeta pide:
//
//   - EL DIAGNOSTICO DEL DD, CON EL LOG DELANTE. El `.cof` del DD NO viene vacio. Medido con la sonda del host
//     (`console/dc/sonda-muerte-126`) y con la carga de este POC: `skddhth.cof` son 148 B con 7 capas, 1 cuadro por
//     direccion y 8 direcciones, y sus graficos estan en el MPQ como `<CAPA>LITDD1HS.dcc` -- la clase de arma la
//     declara CADA capa del COF y en el DD es 1HS (no HTH, que es la del caminar y la del DT). Compuesto en la
//     direccion de la patrulla da caja 92x39 con 819 pixeles y crc32 1212887229: se ve. Lo que la tarjeta 122
//     encontro en CERO fue lo que el bicho PINTO en su corrida, y la causa esta en `split.go`: con dos o mas
//     jugadores el POC dibuja por el camino del split, que no dibujaba monstruos. Este archivo no arregla eso (lo
//     hace `split.go`), pero la linea `JUEGO126: MUERTE COF-DD` deja el numero del compuesto en el log para que no
//     se confunda "el COF viene vacio" con "el bicho no se esta dibujando".
//   - LA CAIDA CUADRO A CUADRO. Con `muertePorCuadro="si"` (el INSTRUMENTO, y el log lo dice) el reloj de la muerte
//     cuenta CUADROS DEL POC en vez de milisegundos: cada cuadro del POC dibuja el cuadro siguiente del DT, asi que
//     los 19 cuadros de la caida se ven uno por uno, con su crc32. El ritmo del animdata (49 ms) sigue siendo el que
//     dice cuantos cuadros tiene el ciclo y cuanto duraria en el juego; lo unico que cambia es CADA CUANTO se pide el
//     cuadro siguiente, porque la consola no puede dibujar a 20 fps. Con `muertePorCuadro="no"` el reloj es el de
//     pared del motor, igual que DC-002.122 (es el A/B).
//   - EL CADAVER Y SU EXPIRACION. El DD (un cuadro, el cadaver) se dibuja hasta que se cumple el plazo del MOTOR:
//     `corpseSeconds = 12,0 s` (`d2core/d2monsters/director.go`: "corpses are removed after this long: ENGINE
//     CHOICE"). A los 12,0 s el bicho deja de dibujarse -- el cuadro vuelve a ser el mapa solo -- y se cuenta: es
//     "desaparece del cuadro al final". Con `cadaverKnob="no"` el cadaver no expira (A/B).
//
// LO QUE NO HACE: no toca el motor, no cambia la vida ni el dano (DC-002.122), no toca el drop (DC-002.123) ni el
// inventario (DC-002.124). El cadaver ya no ocupaba celda desde DC-002.122, y eso se mantiene.
package main

const (
	// msCadaver: lo que el MOTOR deja un cadaver en el suelo. `d2core/d2monsters/director.go`:
	// `corpseSeconds = 12.0  // corpses are removed after this long: ENGINE CHOICE, the exe has NO corpse timer`.
	msCadaver = 12000

	// maxCuadrosMuerte: tope del arreglo de cuadros vistos (el DT del Returned son 19; 64 deja lugar a cualquier
	// bicho sin reservar de mas en una consola de 16 MB).
	maxCuadrosMuerte = 64
)

// muerteDelBicho: el reloj y la cuenta de la muerte de UN bicho. Se crea cuando el bicho pasa a DT y se tira con el
// (no se reserva nada mientras esta vivo).
type muerteDelBicho struct {
	// PorCuadro: el instrumento. "si" = un cuadro de animacion por cuadro del POC (para VER la caida con el cuadro
	// del POC a ~1,3 s); "no" = el reloj de pared del motor (935 ms de DT, 67 ms de DD).
	PorCuadro bool

	// CuadrosPOC: cuadros del POC desde que empezo la muerte (es el reloj del instrumento y el contador del cadaver).
	// Se calcula como `cuadro - CuadroDelArranque`, asi que el cuadro en el que el bicho muere es el 0 y el DT
	// empieza a dibujarse por SU PRIMER cuadro (medido en la primera corrida: con un contador que sumaba uno por
	// cuadro ANTES de dibujar se perdia el cuadro 0 y el 18 salia dos veces).
	CuadrosPOC        int
	CuadroDelArranque int

	// CuadrosDT: cuantos cuadros tiene el DT (medido del ciclo compuesto, no escrito a mano).
	CuadrosDT int

	// Vistos: que cuadros del DT se llegaron a DIBUJAR (distintos). Es el numero que dice si la caida se vio.
	Vistos    [maxCuadrosMuerte]bool
	Dibujados int

	// El cuadro del DD (el cadaver) y los que lo dibujaron.
	CuadroDD       bool
	CuadrosCadaver int

	// Lo medido del dibujo.
	PixelesDT   int // pixeles que pinto el ULTIMO cuadro del DT dibujado
	PixelesDD   int // pixeles que pinto el cadaver (el primero que se dibujo)
	MsDibujoDT  int64
	MsDibujoDD  int64
	PrimerCRC   uint32 // crc32 del primer cuadro del DT dibujado
	UltimoCRCDT uint32 // crc32 del ultimo cuadro del DT dibujado
	CRCDD       uint32

	// La expiracion del cadaver.
	MsDelDD     int64 // ms de consola en los que arranco el DD
	Expirado    bool
	MsDeLaCaida int64 // ms de consola que duro el DT (reloj de pared, para comparar con los 935 del motor)

	// El lienzo: en cuantos cuadros el mapa volvio a su crc32 despues de devolver la caja del cadaver.
	CuadrosConLienzoRestaurado int
	CuadrosConLienzoDistinto   int

	// UltimoCuadroAnotado: el ultimo cuadro del POC que se anoto. En la pantalla partida el mismo bicho se pinta en
	// las DOS mitades del cuadro, asi que la anotacion se deduplica por cuadro: los contadores cuentan cuadros, no
	// mitades (medido: sin esto el resumen del split sumaba dos veces cada cuadro del cadaver).
	UltimoCuadroAnotado int
}

var (
	// muertePorCuadro / cadaverKnob: las dos palancas de esta tarjeta (knobs.go). Se leen por funcion para que el
	// archivo no dependa del orden de inicializacion.
	// Cuadros de la corrida en los que hubo cadaver a la vista (sumado de todos los bichos).
	cuadrosConCadaver int
	// Cuadros en los que el lienzo NO volvio al mapa despues de dibujar un bicho en muerte.
	lienzosDistintosEnLaMuerte int
)

// muerteDe: la ficha de la muerte del bicho (nil si no murio).
func muerteDe(b *bicho) *muerteDelBicho {
	if b == nil {
		return nil
	}

	return b.Muerte
}

// empezarLaMuerte: el bicho acaba de entrar en DT. Se arma su ficha con el largo REAL del ciclo compuesto y se dice
// en el log con que reloj se va a jugar (el instrumento o el del motor).
func empezarLaMuerte(b *bicho, ciclo *CicloDelJugador, ahora int64) {
	if b == nil {
		return
	}

	m := &muerteDelBicho{
		PorCuadro:           muertePorCuadro == "si",
		CuadrosDT:           len(ciclo.Cuadros),
		CuadroDelArranque:   cuadro,
		CuadrosPOC:          0,
		MsDelDD:             0,
		UltimoCuadroAnotado: -1,
	}

	b.Muerte = m

	reloj := "motor(49ms por cuadro)"

	if m.PorCuadro {
		reloj = "un cuadro por cuadro del POC (INSTRUMENTO: la consola no dibuja a 20 fps)"
	}

	println("JUEGO126: MUERTE EMPIEZA bicho=", b.Nombre, "modo=", b.Modo, "cuadrosDT=", m.CuadrosDT,
		"msPorCuadro=", ciclo.MsPorCuadro, "cicloDelMotorMs=", m.CuadrosDT*ciclo.MsPorCuadro, "reloj=", reloj,
		"cadaverMs=", msCadaver, "cadaver=", cadaverKnob, "vida=", vidaDelBichoEnTexto(b), "cuadro=", cuadro)
}

// laCaidaTermino: si el DT ya se mostro entero. Con el instrumento son los cuadros del ciclo dibujados; sin el, el
// reloj de pared contra el ciclo del animdata (la regla de DC-002.122).
func laCaidaTermino(b *bicho, ahora int64, ciclo *CicloDelJugador) bool {
	m := muerteDe(b)
	if m == nil {
		return true
	}

	if m.PorCuadro {
		return m.CuadrosPOC >= len(ciclo.Cuadros)
	}

	return ahora-b.MsModo >= int64(len(ciclo.Cuadros)*ciclo.MsPorCuadro)
}

// pasoDeLaMuerte: un cuadro del reloj de la muerte. Se llama en cada cuadro del POC mientras el bicho esta en DT o
// DD: avanza el contador (el reloj del instrumento), mide lo que duro la caida y hace expirar el cadaver a los
// 12,0 s del motor.
func pasoDeLaMuerte(b *bicho, ahora int64) {
	m := muerteDe(b)
	if m == nil {
		return
	}

	// el reloj del instrumento: cuadros del POC desde el cuadro en el que el bicho murio (ese es el 0).
	m.CuadrosPOC = cuadro - m.CuadroDelArranque

	if b.Modo == modoBestiaMuriendo {
		m.MsDeLaCaida = ahora - b.MsModo

		return
	}

	if b.Modo != modoBestiaMuerta || m.Expirado {
		return
	}

	// EL CADAVER: se queda lo que el motor lo deja (12,0 s desde que el DD empezo).
	if cadaverKnob != "si" {
		return
	}

	if ahora-m.MsDelDD < msCadaver {
		return
	}

	m.Expirado = true

	println("JUEGO126: CADAVER EXPIRA bicho=", b.Nombre, "ms=", ahora-m.MsDelDD, "cuadrosDelCadaver=",
		m.CuadrosCadaver, "cuadrosDelPOC=", m.CuadrosPOC, "vida=", vidaDelBichoEnTexto(b), "cuadro=", cuadro)
}

// empezarElCadaver: el DT termino y el bicho pasa a DD. Se mide cuando arranco (para los 12,0 s) y se dice.
func empezarElCadaver(b *bicho, ahora int64) {
	m := muerteDe(b)
	if m == nil {
		return
	}

	m.MsDelDD = ahora

	println("JUEGO126: MUERTE CAE bicho=", b.Nombre, "cuadrosDTDibujados=", m.Dibujados, "de", m.CuadrosDT,
		"msDeLaCaida=", m.MsDeLaCaida, "msDelMotor=", m.CuadrosDT*49, "cuadro=", cuadro)
}

// cuadroDeLaMuerte: que cuadro del ciclo de la muerte se dibuja en este cuadro del POC.
//
// Con el instrumento, el DT avanza UN cuadro por cuadro del POC: es lo que deja ver los 19 cuadros de la caida con
// un cuadro del POC de ~1,3 s. Sin el, el cuadro sale del reloj del modo (ms desde MsModo), que es la cuenta de
// DC-002.122. El DD tiene un solo cuadro y no se mueve.
func cuadroDeLaMuerte(b *bicho, msReloj int) int {
	m := muerteDe(b)
	if m == nil || !m.PorCuadro || b.Modo != modoBestiaMuriendo {
		return msReloj
	}

	return m.CuadrosPOC
}

// elCadaverSeDibuja: si al cadaver le queda vida. Un cadaver expirado no se dibuja mas (es "desaparece del cuadro").
func elCadaverSeDibuja(b *bicho) bool {
	if b == nil || b.Modo != modoBestiaMuerta {
		return true
	}

	m := muerteDe(b)

	return m == nil || !m.Expirado
}

// anotarElCuadroDeLaMuerte: la instrumentacion del dibujo, por cuadro. Se llama DESPUES de pintar el bicho y con lo
// que el pincel midio: dice que cuadro de la muerte se vio, cuantos pixeles pinto, cuanto costo y si el lienzo
// volvio al mapa. Es la evidencia cuadro por cuadro de "la muerte se ve".
func anotarElCuadroDeLaMuerte(b *bicho, cu *CuadroDelJugador, pixelesPintados, ms int, lienzoRestaurado bool) {
	if b == nil || cu == nil {
		return
	}

	m := muerteDe(b)
	if m == nil {
		return
	}

	// una anotacion por cuadro del POC: en el split el mismo bicho se pinta en las dos mitades.
	repetido := m.UltimoCuadroAnotado == cuadro
	m.UltimoCuadroAnotado = cuadro

	if repetido {
		return
	}

	if lienzoRestaurado {
		m.CuadrosConLienzoRestaurado++
	} else {
		m.CuadrosConLienzoDistinto++
		lienzosDistintosEnLaMuerte++
	}

	switch b.Modo {
	case modoBestiaMuriendo:
		m.MsDibujoDT += int64(ms)
		m.PixelesDT = pixelesPintados
		m.UltimoCRCDT = cu.CRC

		if m.Dibujados == 0 {
			m.PrimerCRC = cu.CRC
		}

		if b.CuadroCiclo >= 0 && b.CuadroCiclo < maxCuadrosMuerte && !m.Vistos[b.CuadroCiclo] {
			m.Vistos[b.CuadroCiclo] = true
			m.Dibujados++
		}

		println("JUEGO126: MUERTE CUADRO modo= DT cuadro=", b.CuadroCiclo, "de", m.CuadrosDT, "caja=", cu.Ancho,
			"x", cu.Alto, "pixelesCompuestos=", cu.PixelesNoVacios, "pixelesPintados=", pixelesPintados,
			"crc32=", int(cu.CRC), "ms=", ms, "lienzoRestaurado=", lienzoRestaurado, "cuadro=", cuadro)
	case modoBestiaMuerta:
		m.CuadrosCadaver++
		cuadrosConCadaver++

		if !m.CuadroDD {
			m.CuadroDD = true
			m.PixelesDD = pixelesPintados
			m.CRCDD = cu.CRC

			println("JUEGO126: MUERTE CUADRO modo= DD cuadro= 0 de 1 caja=", cu.Ancho, "x", cu.Alto,
				"pixelesCompuestos=", cu.PixelesNoVacios, "pixelesPintados=", pixelesPintados, "crc32=", int(cu.CRC),
				"lienzoRestaurado=", lienzoRestaurado, "cuadro=", cuadro)
		}

		m.MsDibujoDD += int64(ms)
	}
}

// diagnosticoDelModoDD: lo que la tarjeta pide primero -- POR QUE EL .cof DD "VENIA VACIO". Se imprime al final de
// la carga del combate, con el ciclo del DD delante: cuantos cuadros tiene, cuanto mide, cuantos pixeles trae el
// compuesto y su crc32. Los mismos numeros que la sonda del host (`sonda-muerte-126`), para que se puedan comparar.
func diagnosticoDelModoDD() {
	porDireccion := ciclosModoBicho[modoBestiaMuerta]
	if len(porDireccion) == 0 {
		println("JUEGO126: MUERTE COF-DD SIN CICLO: el DD no se compuso (ver JUEGO122: COMBATE el modo DD)")

		return
	}

	for dir, ciclo := range porDireccion {
		if ciclo == nil || len(ciclo.Cuadros) == 0 {
			continue
		}

		cu := &ciclo.Cuadros[0]

		println("JUEGO126: MUERTE COF-DD modo= DD dir=", dir, "cuadros=", len(ciclo.Cuadros), "msPorCuadro=",
			ciclo.MsPorCuadro, "caja=", cu.Ancho, "x", cu.Alto, "pixelesCompuestos=", cu.PixelesNoVacios,
			"crc32=", int(cu.CRC), "capasDibujadas=", cu.CapasDibujadas, "capasSinGrafico=", cu.CapasSinGrafico,
			"vacio=", cu.PixelesNoVacios == 0, "crc32DelHost= 1212887229 pixelesDelHost= 819")
	}
}

// resumenDeLaMuerte: el cierre de la tarjeta: la caida, el cadaver y el lienzo.
func resumenDeLaMuerte() {
	b := bichoDePelea()
	if b == nil {
		println("JUEGO126: MUERTE RESUMEN sin bicho de pelea")

		return
	}

	m := muerteDe(b)
	if m == nil {
		println("JUEGO126: MUERTE RESUMEN el bicho no murio: modo=", modoDelBicho(), "vida=", vidaDelBichoEnTexto(b))

		return
	}

	lienzo := m.CuadrosConLienzoDistinto == 0

	println("JUEGO126: MUERTE RESUMEN bicho=", b.Nombre, "modoFinal=", b.Modo, "reloj=", siNo(m.PorCuadro),
		"cuadrosDT=", m.CuadrosDT, "dibujadosDT=", m.Dibujados, "cuadroDD=", m.CuadroDD,
		"cuadrosDelCadaver=", m.CuadrosCadaver, "expirado=", m.Expirado, "vida=", vidaDelBichoEnTexto(b),
		"msDeLaCaida=", m.MsDeLaCaida, "msDelMotor=", m.CuadrosDT*49,
		"pixelesDT=", m.PixelesDT, "pixelesDD=", m.PixelesDD, "msDibujoDT=", m.MsDibujoDT,
		"msDibujoDD=", m.MsDibujoDD, "crcdtPrimerCuadro=", int(m.PrimerCRC), "crcDTUltimoCuadro=",
		int(m.UltimoCRCDT), "crcDD=", int(m.CRCDD), "cuadrosConLienzoRestaurado=", m.CuadrosConLienzoRestaurado,
		"cuadrosConLienzoDistinto=", m.CuadrosConLienzoDistinto, "lienzoRestaurado=", lienzo)

	println("JUEGO126: MUERTE CRITERIO laCaidaSeVe=", m.Dibujados > 0, "cuadrosDeLaCaida=", m.Dibujados,
		"elCadaverSeVe=", m.CuadroDD && m.CuadrosCadaver > 0, "elCadaverDesaparece=", m.Expirado,
		"elLienzoVuelveAlMapa=", lienzo, "losDosRelojes=", true, "cadaver=", cadaverKnob)

	// el crc32 del compuesto del DD contra el del host: el numero que prueba que el .cof NO viene vacio.
	if m.CRCDD != 0 {
		println("JUEGO126: MUERTE COF-DD crc32DelCompuesto=", int(m.CRCDD), "pixeles=", m.PixelesDD,
			"crc32DelHost= 1212887229 pixelesDelHost= 819 coincide=", m.CRCDD == 1212887229)
	}
}

//go:build dreamcast

// DC-002.115 -- EL LEASH: que ninguno de los dos se salga del lienzo de 640x480.
//
// POR QUE EXISTE. DC-002.114 midio el limite del cooperativo en UNA pantalla: con la camara en el CENTRO del
// grupo, dos jugadores a ~22 celdas de separacion dejan los dos anclas FUERA del lienzo (828,-279 y -189,844).
// La causa es la proyeccion del motor: una celda son (80,40) px ortogonales (`viewport.go`: tileWidth = 80,
// tileHeight = 40), asi que la separacion en pantalla no es proporcional a la distancia -- depende de la
// DIRECCION en la que se separan:
//
//	separacion (dx,dy) en celdas  ->  px en pantalla = ((dx-dy)*80, (dx+dy)*40)
//
// Correr en la diagonal (dx = -dy) mueve 113 px por celda en X y 0 en Y; correr en la otra diagonal (dx = dy)
// mueve 0 en X y 56 px por celda en Y. O sea: la MISMA distancia en celdas no es la misma distancia en pantalla,
// y el lienzo tiene 640x480. Ese es el motivo de que el juego original tenga leash: no se puede dejar que el que
// camina se aleje del otro mas de lo que entra en el cuadro.
//
// QUE HACE ESTE ARCHIVO, Y EN QUE ORDEN:
//
//  1. `medirElLeash` MIDE, al arranque, la distancia maxima a la que los dos entran en el lienzo en CADA una de
//     las 8 direcciones de separacion (la imagen de las 8 de la cruceta por `DireccionDelPiso`, el mismo camino
//     del movimiento). No se elige a ojo: se barre la distancia de 0,05 en 0,05 celdas y se para en la ultima en
//     la que las DOS cajas compuestas entran en 640x480. Lo que se mide es la caja PEOR de los cuadros que se
//     dibujan (la mas grande de los 9 ciclos), no la del cuadro de turno: el leash tiene que valer para todo el
//     ciclo, no para un cuadro.
//  2. `aplicarLeash` lo APLICA cuadro a cuadro, despues de mover y antes de la camara: si el que camino se alejo
//     mas que la distancia permitida, se lo deja EN EL BORDE (se proyecta su posicion sobre el circulo de radio
//     `leashEnCeldas` alrededor del otro jugador). Es el que se movio el que se frena; al que estaba quieto no se
//     lo empuja.
//  3. La cuenta NO se falsea: cuando el leash recorta, los subtiles que el jugador no camino se le devuelven a su
//     estado (`movimiento.Subtiles` y `Estado.Distancia`) y se cuentan aparte (`LeashRecortes`, `LeashCeldas`).
//     Por eso la velocidad que imprime el log es la de VERDAD: caminando contra el leash da 0 celdas/s con los
//     ticks de 25 Hz corriendo, igual que caminar contra una pared.
//  4. El lienzo se VIGILA en cada cuadro (`leashDelCuadro`): las dos anclas y las dos cajas SIN recortar, y
//     `dentroDelLienzo` es el numero del criterio. Si alguna sale, el log lo dice con la palabra `FUERA`.
//
// DE DONDE SALE LA DISTANCIA (nada elegido a ojo). El juego original no tiene un numero que se pueda sacar del
// exe para esto (el "leash" de Diablo II es el de los MONSTRUOS, `d2monster`: la distancia a su ancla de
// aparicion). El limite del cooperativo en una pantalla no esta en ninguna tabla: sale de la geometria de arriba,
// y por eso aca se MIDE con la proyeccion y los cuadros reales, y se usa el peor de los 8 casos.
package main

import "math"

const (
	// pasoDelBarrido: la resolucion de la medida, 0,05 celdas (1/20 de celda = 1/4 de subtile). Mas fino que
	// esto no cambia nada: 0,05 celdas son 5,6 px en la peor direccion.
	pasoDelBarrido = 0.05

	// topeDelBarrido: hasta donde se barre (celdas). El lienzo se acaba mucho antes (medido: 5..7 celdas), pero
	// el tope tiene que dejar ver que el barrido se termina por el lienzo y no por el tope.
	topeDelBarrido = 20.0
)

// leashEnCeldas: la distancia maxima que se APLICA, en celdas del mapa. Arranca en la palanca `leashCeldas` de
// knobs.go y, si esa palanca es 0, `medirElLeashAlArranque` la deja en la MEDIDA (el peor de los 8 casos).
var leashEnCeldas = leashCeldas

// laCajaPeor: la caja compuesta mas GRANDE de todos los ciclos que se pueden dibujar (quieto y los 8 de caminar),
// respecto del ancla. Es la que se usa para medir: si con la caja peor los dos entran, entran con cualquiera.
type cajaDelCiclo struct {
	Izq, Arriba, Ancho, Alto int
	DeDonde                  string
}

var laCajaPeor cajaDelCiclo

// calcularLaCajaPeor: recorre el ciclo quieto y los de caminar y se queda con los extremos mas grandes: el Izq
// mas negativo, el Arriba mas negativo, y el Ancho/Alto mayores. Se llama una vez, con los cuadros ya compuestos.
func calcularLaCajaPeor() {
	peor := cajaDelCiclo{DeDonde: "ninguno"}

	mirar := func(nombre string, c *CicloDelJugador) {
		if c == nil {
			return
		}

		for i := range c.Cuadros {
			cu := &c.Cuadros[i]

			if peor.DeDonde == "ninguno" || cu.Izq < peor.Izq {
				peor.Izq = cu.Izq
			}

			if peor.DeDonde == "ninguno" || cu.Arriba < peor.Arriba {
				peor.Arriba = cu.Arriba
			}

			if cu.Ancho > peor.Ancho {
				peor.Ancho = cu.Ancho
			}

			if cu.Alto > peor.Alto {
				peor.Alto = cu.Alto
			}

			if peor.DeDonde == "ninguno" {
				peor.DeDonde = nombre
			}
		}
	}

	mirar("quieto", jugadorQuieto)

	for _, dir := range direccionesCompuestas {
		mirar("caminando", jugadoresCaminando[dir])
	}

	laCajaPeor = peor

	println("JUEGO120: LEASH CAJA-PEOR izq=", laCajaPeor.Izq, "arriba=", laCajaPeor.Arriba,
		"ancho=", laCajaPeor.Ancho, "alto=", laCajaPeor.Alto, "de=", laCajaPeor.DeDonde,
		"ciclos=", len(jugadoresCaminando)+1)
}

// mundoAPantallaConLaCamaraEn: el MISMO `WorldToScreenF` del renderer (`viewport.go`: OrthoToScreenF con la
// camara corrida media pantalla), pero con la camara en la celda que uno elige. Sirve para medir separaciones sin
// mover la camara del motor, y la cuenta es la del motor: `WorldToOrtho` del renderer menos el ortogonal de la
// camara mas el centro de la pantalla, que es exactamente `getCameraOffset` de `viewport.go`.
func mundoAPantallaConLaCamaraEn(camX, camY float64) func(x, y float64) (float64, float64) {
	cox, coy := rendererDelMapa.WorldToOrtho(camX, camY)
	centroX, centroY := float64(ancho)/2.0, float64(alto)/2.0

	return func(x, y float64) (float64, float64) {
		ox, oy := rendererDelMapa.WorldToOrtho(x, y)

		return ox - cox + centroX, oy - coy + centroY
	}
}

// cajaDeLaPeor: el ancla y la caja (la PEOR) de una posicion, con la camara en `camX,camY`. Usa `AnclaDelJugador`
// y `CajaDe`, que son las funciones con las que se dibuja: la medida no puede discrepar del dibujo.
func cajaDeLaPeor(x, y, camX, camY float64) (int, int, CajaDelJugador) {
	ax, ay := AnclaDelJugador(mundoAPantallaConLaCamaraEn(camX, camY), x, y)

	return ax, ay, CajaDelJugador{
		X0: ax + laCajaPeor.Izq, Y0: ay + laCajaPeor.Arriba,
		X1: ax + laCajaPeor.Izq + laCajaPeor.Ancho, Y1: ay + laCajaPeor.Arriba + laCajaPeor.Alto,
	}
}

// cajaDentroDelLienzo: la caja entera adentro de la pantalla (los bordes incluidos: X1 e Y1 son exclusivos).
func cajaDentroDelLienzo(c CajaDelJugador) bool {
	return c.X0 >= 0 && c.Y0 >= 0 && c.X1 <= ancho && c.Y1 <= alto
}

// direccionDeSeparacion: una de las 8 formas de separarse, con su nombre para el log (el nombre es el del eje de
// la cruceta que pide el jugador 1; el 2 pide el contrario).
type direccionDeSeparacion struct {
	Nombre string
	X, Y   float64 // vector UNITARIO en CELDAS del mapa
}

// direccionesDeSeparacion: las 8 direcciones de la cruceta pasadas por `DireccionDelPiso` (la inversa de la
// proyeccion que usa el movimiento). No se escriben a mano: se piden a la misma funcion que mueve al jugador.
func direccionesDeSeparacion() []direccionDeSeparacion {
	nombres := [8]string{"DERECHA", "ABAJO+DERECHA", "ABAJO", "ABAJO+IZQUIERDA",
		"IZQUIERDA", "ARRIBA+IZQUIERDA", "ARRIBA", "ARRIBA+DERECHA"}
	ejes := [8][2]float64{{1, 0}, {1, 1}, {0, 1}, {-1, 1}, {-1, 0}, {-1, -1}, {0, -1}, {1, -1}}

	dirs := make([]direccionDeSeparacion, 0, 8)

	for i := range ejes {
		dx, dy := DireccionDelPiso(ejes[i][0], ejes[i][1])
		dirs = append(dirs, direccionDeSeparacion{Nombre: nombres[i], X: dx, Y: dy})
	}

	return dirs
}

// separacionEnPantalla: la separacion entre dos posiciones del mapa, en PIXELES de la pantalla, con la camara en
// el centro del grupo (que es donde la pone el POC). Devuelve tambien el ancla de cada uno.
func separacionEnPantalla(x1, y1, x2, y2 float64) (pxX, pxY int, ax1, ay1, ax2, ay2 int) {
	cx, cy := (x1+x2)/2.0, (y1+y2)/2.0
	mundoAPantalla := mundoAPantallaConLaCamaraEn(cx, cy)

	ax1, ay1 = AnclaDelJugador(mundoAPantalla, x1, y1)
	ax2, ay2 = AnclaDelJugador(mundoAPantalla, x2, y2)

	return ax2 - ax1, ay2 - ay1, ax1, ay1, ax2, ay2
}

// medirElLeash: el BARRIDO. Para cada direccion de separacion y para cada distancia (de a `pasoDelBarrido`), pone
// a los dos jugadores en el medio del grupo separados `d` celdas, calcula las DOS cajas (la peor) con la camara en
// el centro, y se queda con la mayor `d` en la que las dos entran en el lienzo.
//
// Devuelve la PEOR de las 8 (la distancia que sirve para todas) y deja el detalle impreso.
func medirElLeash() float64 {
	peor := topeDelBarrido
	dirs := direccionesDeSeparacion()

	for i := range dirs {
		u := dirs[i]
		maxima := 0.0

		for d := pasoDelBarrido; d <= topeDelBarrido+1e-9; d += pasoDelBarrido {
			// el jugador 1 de un lado del centro y el 2 del otro: la camara, en el medio
			x1, y1 := celdaX-u.X*d/2.0, celdaY-u.Y*d/2.0
			x2, y2 := celdaX+u.X*d/2.0, celdaY+u.Y*d/2.0

			_, _, c1 := cajaDeLaPeor(x1, y1, celdaX, celdaY)
			_, _, c2 := cajaDeLaPeor(x2, y2, celdaX, celdaY)

			if !cajaDentroDelLienzo(c1) || !cajaDentroDelLienzo(c2) {
				break
			}

			maxima = d
		}

		pxX, pxY, ax1, ay1, ax2, ay2 := separacionEnPantalla(
			celdaX-u.X*maxima/2.0, celdaY-u.Y*maxima/2.0,
			celdaX+u.X*maxima/2.0, celdaY+u.Y*maxima/2.0)

		println("JUEGO120: LEASH DIRECCION", u.Nombre, "maxCeldas=", maxima, "pxX=", pxX, "pxY=", pxY,
			"px=", int(math.Sqrt(float64(pxX*pxX+pxY*pxY))), "anclas=", ax1, ay1, ax2, ay2,
			"vector=", u.X, u.Y)

		if maxima < peor {
			peor = maxima
		}
	}

	return peor
}

// medirElLeashAlArranque: el arranque del leash. Mide, imprime, y deja `leashEnCeldas` en:
//
//	palanca 0        -> la MEDIDA (la peor de las 8 direcciones), redondeada HACIA ABAJO al paso del barrido
//	palanca > 0      -> la palanca, con un AVISO si es mayor que la medida (el binario no cumple el criterio)
//
// El auto-chequeo del instrumento va primero: con la camara en la celda del jugador, el ancla tiene que dar
// (320,283), que es el numero medido en DC-002.112. Si esa cuenta no da, la medida no vale nada y se dice.
func medirElLeashAlArranque() {
	if len(jugadores) < 2 || rendererDelMapa == nil {
		println("JUEGO120: LEASH no aplica (jugadores=", len(jugadores), ")")

		return
	}

	calcularLaCajaPeor()

	ax, ay, _ := cajaDeLaPeor(celdaX, celdaY, celdaX, celdaY)
	println("JUEGO120: LEASH AUTOCHEQUEO camara=", celdaX, celdaY, "ancla=", ax, ay,
		"esperado= 320 283 igual=", ax == 320 && ay == 283)

	medida := medirElLeash()

	if leashCeldas <= 0 {
		leashEnCeldas = math.Floor(medida/pasoDelBarrido) * pasoDelBarrido
	} else {
		leashEnCeldas = leashCeldas

		if leashCeldas > medida {
			println("JUEGO120: LEASH AVISO la palanca (", leashCeldas, ") es MAYOR que la medida (", medida,
				"): con esta distancia los jugadores se pueden salir del lienzo")
		}
	}

	println("JUEGO120: LEASH MEDIDA peorCeldas=", medida, "leashCeldas=", leashEnCeldas,
		"palanca=", leashCeldas, "paso=", pasoDelBarrido, "caja=", laCajaPeor.Ancho, "x", laCajaPeor.Alto,
		"lienzo=", ancho, alto)

	// DC-002.117: EL GRUPO. Con 3 y 4 jugadores el limite es el RADIO (leashCeldas/2) desde el centro del grupo, y
	// lo primero que hay que saber es con que separacion ARRANCA el grupo: si ya es mayor que el leash, el primer
	// cuadro lo recorta (y eso se veria en los recortes, no en un comentario).
	d, x1, y1, x2, y2 := separacionMaximaDelGrupo()
	cx, cy := centroDeLosJugadores()

	println("JUEGO120: LEASH GRUPO jugadores=", len(jugadores), "deGrupo=", leashDeGrupo,
		"radioDelLeash=", radioDelLeash(), "separacionAlArrancar=", d,
		"radioAlArrancar=", radioMaximoDesde(cx, cy), "centro=", cx, cy,
		"parMasLejano=", x1, y1, x2, y2, "entraAlArrancar=", siNo(d <= leashEnCeldas+1e-9))
}

// ============================== EL LEASH CUADRO A CUADRO ==============================

// distanciaEnCeldas: la distancia entre dos posiciones del mapa, en celdas.
func distanciaEnCeldas(x1, y1, x2, y2 float64) float64 {
	dx, dy := x2-x1, y2-y1

	return math.Sqrt(dx*dx + dy*dy)
}

// descontarElPaso: cuando el leash deja al jugador mas cerca que lo que el paso lo movio, se le devuelve a la
// cuenta del movimiento lo que NO camino (el paso que el leash se comio). Asi la velocidad que imprime el log es
// la de verdad (caminar contra el leash da 0 celdas/s) y el recorte se cuenta en su propio contador.
func descontarElPaso(p *participante, nx, ny float64) {
	realSubtiles := distanciaEnCeldas(p.IniX, p.IniY, nx, ny) * float64(subtilesPorCelda)
	devuelto := p.Caminado - realSubtiles

	if devuelto > 0 {
		p.Caminado = realSubtiles
		p.Mov.Subtiles -= devuelto
		p.Mov.Est.Distancia -= devuelto
		p.LeashCeldas += devuelto / float64(subtilesPorCelda)
	}
}

// aplicarLeash: EL LEASH. Se llama despues de mover a los jugadores y antes de la camara.
//
// DOS PASOS, Y NINGUNO EMPUJA AL QUE NO SE MOVIO:
//
//  1. EL RECORTE RADIAL, jugador por jugador y contra la posicion del OTRO AL EMPEZAR EL CUADRO. El paso que el
//     jugador pidio se descompone en la direccion del otro (radial) y su perpendicular (lateral). Solo se recorta
//     el radial que lo ALEJA, y solo hasta el borde: el movimiento lateral se conserva (el jugador puede rodear
//     al otro pegado al leash) y nunca se lo empuja HACIA el otro. Al que no camino no se lo toca.
//
//  2. LA CORRECCION SIMETRICA. Con los dos empujando para afuera en el mismo cuadro, cada uno quedo en el borde
//     medido desde donde arranco el otro, asi que la distancia final puede pasarse un poco. Se los acerca a los
//     DOS por igual hacia el MEDIO DEL GRUPO (el medio no se mueve: la camara, que va al medio, tampoco se corre
//     y la pareja no viaja por el mapa).
//
// POR QUE ASI Y NO PROYECTANDO SOBRE EL CIRCULO DEL OTRO (lo que se probo primero y se midio): proyectar al que se
// paso sobre el circulo alrededor de la posicion YA MOVIDA del otro lo ARRASTRA con el otro jugador, y como los
// dos empujan para lados contrarios, la pareja se iba caminando por el mapa a ~1 celda por cuadro. Medido en
// Flycast: en 63 s la pareja se alejo ~10 celdas del pueblo, la cache de tiles se lleno ("pvr_mem: out of PVR
// memory"), el monton de Go crecio hasta el semiespacio y el POC murio por OOM en el cuadro ~50.
//
// Devuelve cuantos jugadores se recortaron en este cuadro.
func aplicarLeash() int {
	if leashEnCeldas <= 0 || len(jugadores) < 2 {
		return 0
	}

	// DC-002.117: con TRES o CUATRO jugadores el leash que manda es el del GRUPO (ver aplicarLeashDeGrupo); el de
	// a pares es el de DC-002.115 y es el A/B que dice por que el de grupo existe.
	if len(jugadores) > 2 {
		if leashDeGrupo {
			return aplicarLeashDeGrupo()
		}

		println("JUEGO120: LEASH AVISO con jugadores=", len(jugadores),
			"se aplica el leash de a PARES (leashDeGrupo=no): no alcanza para el grupo, la vigilancia lo dice")
	}

	recortados := 0

	for i := range jugadores {
		p := jugadores[i]

		// solo el que camino este cuadro: el leash dice "el que se aleja no puede pasar", no "empuja al otro"
		if p.Mov == nil || p.Mov.Est == nil || (p.Ticks == 0 && p.Caminado == 0) {
			continue
		}

		// con dos jugadores esto es el otro (DC-002.115); con mas, el siguiente del anillo
		otro := jugadores[(i+1)%len(jugadores)]
		if otro.Mov == nil || otro.Mov.Est == nil {
			continue
		}

		// EL OTRO, DONDE ESTABA AL EMPEZAR EL CUADRO: con eso el recorte de los dos es simetrico y ninguno
		// arrastra al otro.
		ox, oy := otro.IniX, otro.IniY

		x, y := p.Mov.Est.Celda()
		if distanciaEnCeldas(ox, oy, x, y) <= leashEnCeldas {
			continue
		}

		d0 := distanciaEnCeldas(ox, oy, p.IniX, p.IniY)

		ux, uy := 1.0, 0.0
		if d0 > 0 {
			ux, uy = (p.IniX-ox)/d0, (p.IniY-oy)/d0
		}

		sx, sy := x-p.IniX, y-p.IniY

		radial := sx*ux + sy*uy                          // lo que se aleja (o se acerca, si es negativo)
		lateralX, lateralY := sx-radial*ux, sy-radial*uy // lo que rodea: no cuenta para el leash

		permitido := leashEnCeldas - d0
		if permitido < 0 {
			permitido = 0
		}

		if radial > permitido {
			radial = permitido
		}

		nx := p.IniX + ux*radial + lateralX
		ny := p.IniY + uy*radial + lateralY

		p.Mov.Est.Pos.Set(nx*subtilesPorCelda, ny*subtilesPorCelda)

		descontarElPaso(p, nx, ny)

		p.LeashDistancia = distanciaEnCeldas(ox, oy, nx, ny)
		p.LeashRecortes++
		recortados++
	}

	// EL PASO 2: los dos empujando para afuera. Se los acerca a los dos por igual hacia el medio. (DC-002.117: este
	// paso es el de DOS; con tres y cuatro el que corrige el grupo entero es `aplicarLeashDeGrupo`.)
	if d, x1, y1, x2, y2 := separacionDeLosDos(); len(jugadores) == 2 && d > leashEnCeldas {
		mx, my := (x1+x2)/2.0, (y1+y2)/2.0
		f := leashEnCeldas / d

		nx1, ny1 := mx+(x1-mx)*f, my+(y1-my)*f
		nx2, ny2 := mx+(x2-mx)*f, my+(y2-my)*f

		for i := range jugadores {
			p := jugadores[i]
			if p.Mov == nil || p.Mov.Est == nil {
				continue
			}

			if i == 0 {
				p.Mov.Est.Pos.Set(nx1*subtilesPorCelda, ny1*subtilesPorCelda)
				descontarElPaso(p, nx1, ny1)
			} else {
				p.Mov.Est.Pos.Set(nx2*subtilesPorCelda, ny2*subtilesPorCelda)
				descontarElPaso(p, nx2, ny2)
			}

			p.LeashDistancia = leashEnCeldas
		}

		println("JUEGO120: LEASH SIMETRICO los dos empujaron para afuera: distancia=", d, "-> ", leashEnCeldas,
			"medio=", mx, my)
	}

	return recortados
}

// separacionDeLosDos: la distancia entre los dos jugadores y sus posiciones, en celdas.
func separacionDeLosDos() (d, x1, y1, x2, y2 float64) {
	if len(jugadores) < 2 || jugadores[0].Mov == nil || jugadores[1].Mov == nil {
		return 0, 0, 0, 0, 0
	}

	x1, y1 = jugadores[0].Mov.Est.Celda()
	x2, y2 = jugadores[1].Mov.Est.Celda()

	return distanciaEnCeldas(x1, y1, x2, y2), x1, y1, x2, y2
}

// ============================== EL LEASH DE GRUPO (DC-002.117): TRES Y CUATRO JUGADORES ==============================
//
// POR QUE HACE FALTA OTRO. El leash de DC-002.115 es de a DOS: recorta a cada jugador contra la posicion del otro y
// despues acerca a los dos al medio. Con cuatro eso no alcanza: cuatro jugadores pueden respetar el leash de a pares
// (5,25 celdas) y aun asi salirse del lienzo, porque el grupo tiene DIAMETRO -- el triangulo equilatero de lado 5,25
// tiene circunradio 3,03 celdas, y la camara va en el CENTRO del grupo: el que queda a 3,03 celdas del centro se
// proyecta a 342 px en la peor direccion y la caja ya no entra en los 640 de ancho.
//
// LA CUENTA QUE SI SIRVE, y que es la MISMA de DC-002.115 leida al reves: alli los dos se ponian a +-d/2 del centro
// de la camara y se barria `d` hasta que las dos cajas entraban. O sea que `d/2` es el RADIO con el que una caja
// entra en el lienzo en la peor de las 8 direcciones. Para el grupo el limite es ese radio: la distancia de CADA
// jugador al centro del grupo no puede pasar de `leashEnCeldas/2` (2,62 celdas con la medida de DC-002.115).
//
// UN SOLO PASO, Y CON EL CENTRO DE DESPUES DE MOVER. El recorte es UNIFORME: se mide el radio del grupo con las
// posiciones que quedo (el centro es el del grupo YA movido, no el de antes), y si se paso del limite se encoge a
// TODOS hacia ese centro en la misma proporcion. Por que asi y no por jugador:
//
//   - Con el centro CONGELADO (el de antes de mover) el grupo que camina JUNTO se recortaba todos los cuadros: al
//     trasladarse, cada uno queda a `radio + paso` del centro viejo aunque la forma del grupo no haya cambiado. Se
//     midio en la primera corrida de esta tarjeta: 22 recortes en 19 cuadros con los cuatro caminando para el mismo
//     lado, que es justo el caso que NO hay que tocar. Con el centro de DESPUES de mover, una traslacion deja el
//     radio igual y no hay recorte.
//   - Encogiendo hacia el centro del grupo, el centro NO se mueve (se encoge alrededor de el), asi que el grupo no
//     viaja por el mapa: es el mismo motivo por el que DC-002.115 rechazo proyectar sobre el circulo del otro.
//
// El que camino contra el leash camina 0 celdas/s (los subtiles que no camino se le devuelven con `descontarElPaso`)
// y el recorte se cuenta en su contador. Al que no camino se lo corre como maximo lo que el grupo se encoge, y si
// ese corrimiento es material tambien se le cuenta: el numero del log no maquilla nada.
func aplicarLeashDeGrupo() int {
	radio := radioDelLeash()
	cx, cy := centroDeLosJugadores() // las posiciones YA movidas: el centro del grupo que quedo

	r := radioMaximoDesde(cx, cy)
	if r <= radio+1e-6 || r <= 0 {
		return 0
	}

	f := radio / r
	recortados := 0

	for _, p := range jugadores {
		if p.Mov == nil || p.Mov.Est == nil {
			continue
		}

		x, y := p.Mov.Est.Celda()
		nx, ny := cx+(x-cx)*f, cy+(y-cy)*f

		if distanciaEnCeldas(x, y, nx, ny) < 1e-9 {
			continue
		}

		p.Mov.Est.Pos.Set(nx*subtilesPorCelda, ny*subtilesPorCelda)

		descontarElPaso(p, nx, ny)

		p.LeashDistancia = distanciaEnCeldas(cx, cy, nx, ny)
		p.LeashRecortes++
		recortados++
	}

	println("JUEGO120: LEASH GRUPO SIMETRICO los", len(jugadores), "se encogen hacia el centro: radio=", r, "->",
		radio, "centro=", cx, cy, "encogidos=", recortados, "jugadores=", len(jugadores))

	return recortados
}

// radioDelLeash: el radio del grupo, en celdas: la mitad del leash MEDIDO (ver la explicacion de arriba).
func radioDelLeash() float64 { return leashEnCeldas / 2.0 }

// radioMaximoDesde: la distancia del jugador mas lejano a un punto del mapa, en celdas.
func radioMaximoDesde(cx, cy float64) float64 {
	r := 0.0

	for _, p := range jugadores {
		if p.Mov == nil || p.Mov.Est == nil {
			continue
		}

		x, y := p.Mov.Est.Celda()
		if d := distanciaEnCeldas(cx, cy, x, y); d > r {
			r = d
		}
	}

	return r
}

// separacionMaximaDelGrupo: el par MAS LEJANO del grupo, en celdas, con sus dos posiciones. Es el diametro del grupo:
// el numero que se compara contra el leash medido (que es una separacion entre dos, o sea un diametro).
func separacionMaximaDelGrupo() (d, x1, y1, x2, y2 float64) {
	for i := 0; i < len(jugadores); i++ {
		if jugadores[i].Mov == nil || jugadores[i].Mov.Est == nil {
			continue
		}

		xi, yi := jugadores[i].Mov.Est.Celda()

		for j := i + 1; j < len(jugadores); j++ {
			if jugadores[j].Mov == nil || jugadores[j].Mov.Est == nil {
				continue
			}

			xj, yj := jugadores[j].Mov.Est.Celda()
			dij := distanciaEnCeldas(xi, yi, xj, yj)

			if dij > d {
				d, x1, y1, x2, y2 = dij, xi, yi, xj, yj
			}
		}
	}

	return d, x1, y1, x2, y2
}

// ============================== LA VIGILANCIA DEL LIENZO (EL CRITERIO) ==============================

// vigilancia: los numeros del criterio de la tarjeta, acumulados durante la corrida. `cuadrosFuera` es EL numero:
// si no es 0, algun jugador se salio del lienzo.
type vigilanciaDelLienzo struct {
	Cuadros          int
	CuadrosFuera     int
	MaxCeldas        float64
	MaxRadio         float64
	MaxPX            int
	MaxPXEnElCuadro  int
	RecortesTotales  int
	CeldasRecortadas float64
}

var vigilancia vigilanciaDelLienzo

// leashDelCuadro: la vigilancia de UN cuadro, con las anclas y las cajas que se acaban de calcular (las de verdad,
// con la camara de verdad: el leash no reescribe nada). Imprime una linea por cuadro: es la evidencia de que los
// dos estan adentro, no un resumen al final.
func leashDelCuadro(cuadro int) {
	if len(jugadores) == 0 {
		return
	}

	celdas := -1.0
	pxX, pxY := 0, 0
	px := 0
	radio := 0.0

	if len(jugadores) > 1 {
		// DC-002.117: el par MAS LEJANO del grupo (con dos es el otro, como en DC-002.115) y el radio del grupo
		// medido desde su centro, que es el numero con el que trabaja el leash de 3 y 4.
		celdas, _, _, _, _ = separacionMaximaDelGrupo()

		for i := 0; i < len(jugadores); i++ {
			for j := i + 1; j < len(jugadores); j++ {
				dx := jugadores[j].AnclaX - jugadores[i].AnclaX
				dy := jugadores[j].AnclaY - jugadores[i].AnclaY

				if d := int(math.Sqrt(float64(dx*dx + dy*dy))); d > px {
					px, pxX, pxY = d, dx, dy
				}
			}
		}

		cx, cy := centroDeLosJugadores()
		radio = radioMaximoDesde(cx, cy)
	}

	vigilancia.Cuadros++

	if celdas > vigilancia.MaxCeldas {
		vigilancia.MaxCeldas = celdas
	}

	if radio > vigilancia.MaxRadio {
		vigilancia.MaxRadio = radio
	}

	if px > vigilancia.MaxPX {
		vigilancia.MaxPX = px
	}

	// las dos cajas y las dos anclas, una por una: la que se sale se marca con FUERA
	fuera := 0

	for _, p := range jugadores {
		dentro := p.Dentro

		if !dentro {
			fuera++
		}

		pcx, pcy := p.Mov.Est.Celda()

		println("JUEGO120: LEASH cuadro=", cuadro, "jugador=", p.Nombre, "ancla=", p.AnclaX, p.AnclaY,
			"caja=", p.CajaCruda.X0, p.CajaCruda.Y0, p.CajaCruda.X1, p.CajaCruda.Y1,
			"dentroDelLienzo=", siNo(dentro), "celda=", pcx, pcy,
			"leashCeldas=", leashEnCeldas, "recortes=", p.LeashRecortes, "celdasRecortadas=", p.LeashCeldas)
	}

	if fuera > 0 {
		vigilancia.CuadrosFuera++

		println("JUEGO120: LEASH FUERA cuadro=", cuadro, "jugadoresFuera=", fuera,
			"el leash NO alcanzo: la separacion es de", celdas, "celdas /", px, "px")
	}

	println("JUEGO120: LEASH SEPARACION cuadro=", cuadro, "jugadores=", len(jugadores), "celdas=", celdas,
		"pxX=", pxX, "pxY=", pxY, "px=", px, "radioMaxCeldas=", radio, "leashRadioCeldas=", radioDelLeash(),
		"maxCeldasDeLaCorrida=", vigilancia.MaxCeldas, "maxRadioDeLaCorrida=", vigilancia.MaxRadio,
		"maxPXDeLaCorrida=", vigilancia.MaxPX, "cuadrosFuera=", vigilancia.CuadrosFuera,
		"camaraCelda=", celdaDeLaCamaraX(), celdaDeLaCamaraY())
}

// resumenDelLeash: el cierre del leash: la distancia medida, la que se aplico, y los numeros del criterio.
func resumenDelLeash() {
	recortes := 0
	celdasRecortadas := 0.0

	for _, p := range jugadores {
		recortes += p.LeashRecortes
		celdasRecortadas += p.LeashCeldas
	}

	println("JUEGO120: LEASH RESUMEN jugadores=", len(jugadores), "leashCeldas=", leashEnCeldas,
		"palanca=", leashCeldas, "deGrupo=", leashDeGrupo, "radioDelLeash=", radioDelLeash(),
		"maxCeldasDeLaCorrida=", vigilancia.MaxCeldas, "maxRadioDeLaCorrida=", vigilancia.MaxRadio,
		"maxPXDeLaCorrida=", vigilancia.MaxPX, "cuadrosVigilados=", vigilancia.Cuadros,
		"cuadrosFuera=", vigilancia.CuadrosFuera, "recortes=", recortes,
		"celdasRecortadas=", celdasRecortadas, "lienzo=", ancho, alto,
		"cajaPeor=", laCajaPeor.Ancho, "x", laCajaPeor.Alto)
}

//go:build dreamcast

// DC-002.122 -- EL GOLPE: EL JUGADOR LE PEGA AL MONSTRUO, LE BAJA LA VIDA Y LO MATA.
//
// QUE HACE ESTE ARCHIVO. El monstruo de DC-002.121 ya patrulla y ya frena al jugador, pero no se le puede pegar.
// Aca entra el golpe, con las piezas del MOTOR y ninguna tabla propia donde haya una del exe:
//
//   - EL BOTON: el Y del control de la Dreamcast (`d2core/d2input/maple`, `BitY -> d2enum.KeyY`). Cada jugador
//     pregunta por SU servicio del maple (`participante.Entrada`), asi que los cuatro tienen su boton y su
//     flanco: una pulsacion es un ataque, dejando el boton apretado no se encadenan golpes (igual que en el
//     juego). El estado entra por la MISMA puerta que un control de verdad: el shim inyecta los bits en el
//     puerto y el servicio los traduce; el POC nunca mira los bits.
//   - LA ANIMACION DEL JUGADOR: el A1 del COF de la amazona (`AMA1HTH.COF`), 13 cuadros a 25,00 fps = 40,00 ms
//     por cuadro (ciclo 520 ms), medido con la sonda del host (`sonda-anim-249 -token AM`). Se compone UNA
//     direccion por cada una que la cruceta puede pedir, igual que TW en DC-002.113, y al atacar el personaje
//     mira al bicho (`DirectionTo` -> `Dir64ToCof`, la cadena del motor) y dibuja el A1 de esa direccion.
//   - EL CUADRO DEL GOLPE: la mitad de la animacion. Es la regla del motor (`d2mapentity/monster.go`,
//     checkEvents: currentFrame/frameCount >= 0,5) y en el A1 de 13 cuadros cae en el 7. La funcion es la del
//     archivo compartido con el verificador del host (`monstruo.go: CuadroDelGolpe`).
//   - EL ALCANCE: `alcance_122.go` (la tabla de distancias del exe + el alcance VERIFICADO de 7 subtiles). Un
//     golpe fuera de alcance NO baja vida y se cuenta: es la mitad negativa del criterio.
//   - EL DANO: `heroDamage` del motor sin arma (1..2 a mano limpia) con el dado del motor, por el camino del
//     archivo COMPARTIDO (`monstruo.go: Combate.GolpeDelHeroe`), que es el MISMO codigo que el verificador del
//     host (`console/dc/verif-monstruo-250`). HP -= dano y a 0 el bicho muere (`damage()` del motor).
//   - LA VIDA DEL BICHO: `monstruo.go: VidaDelBicho`, el camino de siembra del motor (monstats.txt por MinHP/
//     MaxHP y el factor del MonLvl.txt del nivel, con el dado del motor y el piso de 1 HP). Para el Returned da
//     2..3 HP: aguanta mas de un golpe, que es lo que hace falta para VER el GH antes del DT.
//   - LOS MODOS DEL BICHO: GH al recibir, DT al morir, DD despues (los tokens del motor, `d2monster.Mode*`), con
//     el ritmo del animdata.d2 medido en DC-002.50 (GH 56, DT 49, DD 67 ms por cuadro) y la regla de los modos
//     de un solo tiro de `checkEvents`: GH vuelve a caminar, DT pasa a DD.
//
// QUE NO HACE: no toca el motor (el combate se resuelve en el POC con las animaciones del motor), no hay loot, no
// hay experiencia y no hay IA de persecucion: el bicho sigue patrullando igual que en DC-002.121.
package main

import (
	"hash/crc32"

	"github.com/OpenDiablo2/OpenDiablo2/console/dc/fuenteconsola"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2enum"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2cof"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2math/d2vector"
)

const (
	// cofAtaqueJugador: el COF del ataque de la amazona desarmada, del d2char.mpq (`AMA1HTH.COF`, 1.777 B,
	// 13 cuadros por direccion y 16 direcciones: medido con `sonda-anim-249`).
	cofAtaqueJugador = "data\\global\\CHARS\\AM\\COF\\ama1hth.cof"

	// msAtaqueJugador: 25,00 fps del animdata.d2 (AMA1HTH) -> 40,00 ms por cuadro, ciclo 520 ms. El COF no
	// trae ese numero: lo pone el animdata, igual que en DC-002.49 para TN/TW.
	msAtaqueJugador = 40

	// Los tres modos del bicho que entran con el golpe, con el ritmo MEDIDO en DC-002.50 con la sonda del host
	// (animdata: SKGHHTH, SKDTHTH, SKDDHTH).
	msBichoGolpeado = 56 // GH 4 cuadros a 17,97 fps (ciclo 224 ms)
	msBichoMuriendo = 49 // DT 19 cuadros a 20,31 fps (ciclo 931 ms)
	msBichoMuerta   = 67 // DD 1 cuadro a 14,84 fps
)

// ataqueDeJugador: el estado del golpe de UN jugador. Es lo que cuesta por jugador ademas de su `participante`;
// los cuadros del A1 son del TIPO (compartidos por los cuatro), como los de caminar en DC-002.114.
type ataqueDeJugador struct {
	Nombre string

	// el flanco del boton: Apretado este cuadro, Antes el del cuadro anterior
	Apretado bool
	Antes    bool

	// el ataque en curso
	Golpeando bool
	Dado      bool  // el dano de ESTE ataque ya se aplico (cae a la mitad de la animacion)
	FinMs     int64 // el ms del cuadro en el que el ataque termino (para dibujar la ultima pose una vez mas)
	FinCuadro int   // el CUADRO del POC en el que termino (robusto a que el reloj avance antes de dibujar)
	MsInicio  int64 // cuando empezo, en ms de la consola
	Direccion int   // la direccion del COF a la que miraba al empezar
	Distancia int   // la distancia de mesa al bicho en el cuadro del golpe
	Cuadro    int   // el cuadro del A1 que se esta dibujando

	// lo que se mide, por jugador
	Ataques   int      // flancos que empezaron un ataque
	Golpes    int      // golpes que llegaron y bajaron vida
	Afuera    int      // golpes que cayeron con el bicho fuera de alcance
	Dano      int      // dano acumulado
	CuadrosA1 int      // cuantos cuadros DISTINTOS del A1 se llegaron a dibujar
	Vistos    [16]bool // cuales de esos cuadros ya se dibujaron (para no contarlos dos veces)
}

var (
	// ciclosAtaque: los A1 compuestos, uno por direccion de la cruceta (compartidos por los cuatro jugadores).
	ciclosAtaque map[int]*CicloDelJugador

	// ataques: el estado del golpe de cada jugador, en el mismo orden que `jugadores`.
	ataques []*ataqueDeJugador

	// lo del criterio, sumado de los cuatro
	golpesDados        int // golpes que llegaron (la vida bajo)
	golpesAfuera       int // golpes fuera de alcance (la vida NO bajo)
	tarde              int // golpes que cayeron cuando el bicho ya estaba muerto (no cuentan)
	danoDelCombate     int
	muertes            int
	cuadrosA1Dibujados int // cuadros distintos del A1 dibujados por los cuatro
	tamanoBichoCombate int // UnitSizeX del bicho (monstats2 SizeX), para la tabla de distancias
	combateOK          bool
)

// ============================== LA CARGA ==============================

// cargarAtaque: compone el A1 del heroe en las direcciones que la cruceta puede pedir (o en las que diga la
// palanca `dirsAtaque`, para el A/B de memoria) y arma el estado de ataque de cada jugador.
//
// VA ANTES DE SOLTAR EL d2char (jugador_carga.go, paso 4b): el COF y las capas del A1 salen de ese MPQ.
func cargarAtaque(fuente *fuenteconsola.Fuente, leer Lector) {
	// EL ESTADO DE CADA JUGADOR: se reserva para los cuatro puertos del bus, porque esta carga corre ANTES de
	// `prepararJugadores` (los ciclos salen del d2char, que se suelta antes de armar la gente). El que no juega
	// queda con su estado sin usar.
	ataques = make([]*ataqueDeJugador, maxJugadoresCoop)

	for i := 0; i < maxJugadoresCoop; i++ {
		ataques[i] = &ataqueDeJugador{Nombre: nombreDelJugador(i), Direccion: dirQuieto, FinCuadro: -1}
	}

	if combateKnob != "si" {
		println("JUEGO122: COMBATE apagado (combate=", combateKnob, "): la corrida es la de DC-002.121")

		return
	}

	direcciones := DireccionesDelControl()

	// DC-002.122: la palanca dice CUALES (por numero de COF, medidos con `sonda-dir-122`) o CUANTAS.
	if lista := listaDeDirecciones(dirsAtaqueLista); len(lista) > 0 {
		direcciones = lista
	} else if dirsAtaque > 0 && dirsAtaque < len(direcciones) {
		direcciones = direcciones[:dirsAtaque]
	}

	ciclosAtaque = make(map[int]*CicloDelJugador, len(direcciones))

	t0 := int64(d2cMs())
	bytes := 0

	for _, dir := range direcciones {
		ciclo := componerModo(fuente, leer, cofAtaqueJugador, modoGolpePersonaje, msAtaqueJugador, []int{dir})
		if ciclo == nil || len(ciclo.Cuadros) == 0 {
			println("JUEGO122: ATAQUE la direccion", dir, "salio vacia")

			continue
		}

		ciclosAtaque[dir] = ciclo

		delCiclo := bytesDeLosCuadros(ciclo)
		bytes += delCiclo

		println("JUEGO122: ATAQUE DIR", dir, "cuadros=", len(ciclo.Cuadros), "msPorCuadro=", ciclo.MsPorCuadro,
			"caja=", ciclo.Cuadros[0].Ancho, "x", ciclo.Cuadros[0].Alto, "indices=", delCiclo)

		for f := range ciclo.Cuadros {
			cu := &ciclo.Cuadros[f]
			println("JUEGO122: ATAQUE CUADRO dir=", dir, "cuadro=", f, "de", len(ciclo.Cuadros),
				"caja=", cu.Ancho, "x", cu.Alto, "esquina=", cu.Izq, cu.Arriba, "pixeles=", cu.PixelesNoVacios,
				"crc32=", int(cu.CRC), "capasDibujadas=", cu.CapasDibujadas, "sinGrafico=", cu.CapasSinGrafico)
		}
	}

	combateOK = len(ciclosAtaque) > 0

	println("JUEGO122: ATAQUE CICLOS direcciones=", len(ciclosAtaque), "de", len(direcciones),
		"indicesBytes=", bytes, "ms=", int64(d2cMs())-t0, "cuadroDelGolpe=",
		CuadroDelGolpe(len(ciclosAtaque[direcciones[0]].Cuadros)), "de",
		len(ciclosAtaque[direcciones[0]].Cuadros), "combate=", combateKnob, "combateOK=", combateOK)
}

// listaDeDirecciones: parsea `dirsAtaqueLista` ("14" o "14,6" o "14 6"). Vacia = todo por omision ("" = todas).
// La consola no tiene `strconv` en el camino del POC por costumbre (es RAM y codigo): se parsea a mano.
func listaDeDirecciones(texto string) []int {
	if texto == "" {
		return nil
	}

	lista := make([]int, 0, 4)
	n := -1

	for i := 0; i <= len(texto); i++ {
		if i == len(texto) || texto[i] == ',' || texto[i] == ' ' {
			if n >= 0 {
				lista = append(lista, n)
			}

			n = -1

			continue
		}

		if texto[i] < '0' || texto[i] > '9' {
			continue
		}

		if n < 0 {
			n = 0
		}

		n = n*10 + int(texto[i]-'0')
	}

	return lista
}

// cicloDeAtaque: el A1 de una direccion, con la misma caida que `buscarCiclo` (si la direccion no esta compuesta
// se usa la 0, que es la que dibujaba DC-002.38, en vez del ciclo de OTRA direccion).
func cicloDeAtaque(dir int) *CicloDelJugador {
	if len(ciclosAtaque) == 0 {
		return nil
	}

	if c := ciclosAtaque[dir]; c != nil {
		return c
	}

	for _, c := range ciclosAtaque {
		if c.Dir == dir {
			return c
		}
	}

	if c := ciclosAtaque[dirQuieto]; c != nil {
		return c
	}

	for _, c := range ciclosAtaque {
		return c
	}

	return nil
}

// ============================== EL CUADRO DE PELEA ==============================

// combateDelCuadro: un cuadro de pelea. Cada jugador ve su boton, arranca su ataque, y a la mitad de la animacion
// cae el golpe: si el bicho esta en alcance le baja la vida (y si llega a 0, muere); si no, se cuenta y no pasa
// nada. Despues corre el reloj del modo del bicho (GH -> caminar, DT -> DD).
func combateDelCuadro(ahora int64) {
	b := bichoDePelea()

	if b == nil || b.Vida == nil || b.Combate == nil {
		return
	}

	for i, p := range jugadores {
		if i < len(ataques) {
			pasoDeUnAtaque(p, ataques[i], b, ahora)
		}
	}

	pasoDelModoDelBicho(b, ahora)
}

// bichoDePelea: el bicho al que se le pega. Es el PRIMERO de la corrida: la tarjeta pide que los cuatro puedan
// pegarle AL MISMO monstruo.
func bichoDePelea() *bicho {
	if len(bichos) == 0 {
		return nil
	}

	return bichos[0]
}

// pasoDeUnAtaque: un cuadro del golpe de un jugador.
func pasoDeUnAtaque(p *participante, a *ataqueDeJugador, b *bicho, ahora int64) {
	if a == nil || p == nil || p.Mov == nil {
		return
	}

	apretado := p.Entrada.IsKeyPressed(d2enum.KeyY)

	// EL FLANCO: una pulsacion es un ataque. No se encadenan golpes con el boton apretado, y no se arranca otro
	// golpe en medio de uno (el POC rechaza el ataque, igual que en el juego).
	if apretado && !a.Antes && !a.Golpeando && b.Vida.HP > 0 {
		a.Golpeando = true
		a.Dado = false
		a.MsInicio = ahora
		a.Ataques++
		a.Direccion = direccionHaciaElBicho(p, b)

		// el motor mira al bicho al pegarle (`advanceHeroAttack`: SetDirection(DirectionTo(m.Position)))
		p.Mov.Direccion = a.Direccion

		// DC-002.122: si esa direccion del A1 no esta compuesta (la RAM no da para las ocho), el golpe se dibuja
		// con la que haya. Se dice en el log para que la evidencia no mienta sobre lo que se vio.
		dibujada := a.Direccion
		if ciclosAtaque[dibujada] == nil && len(ciclosAtaque) > 0 {
			for d := range ciclosAtaque {
				dibujada = d

				break
			}
		}

		println("JUEGO122: GOLPE-EMPIEZA jugador=", a.Nombre, "ataque=", a.Ataques, "dir=", a.Direccion,
			"dirDibujada=", dibujada, "dirPropia=", ciclosAtaque[a.Direccion] != nil, "vida=", b.Vida.HP,
			"cuadro=", cuadro)
	}

	a.Antes = apretado

	if !a.Golpeando {
		return
	}

	ciclo := cicloDeAtaque(a.Direccion)
	if ciclo == nil || len(ciclo.Cuadros) == 0 {
		a.Golpeando = false

		return
	}

	f := int((ahora - a.MsInicio) / int64(ciclo.MsPorCuadro))
	mitad := CuadroDelGolpe(len(ciclo.Cuadros))

	// EL DANO, UNA SOLA VEZ POR ATAQUE, A LA MITAD DE LA ANIMACION
	if !a.Dado && f >= mitad {
		a.Dado = true

		dx, dy := separacionDelBicho(p, b)
		a.Distancia = DistanciaDeMesa(dx, dy, 2, tamanoBichoCombate)

		if !AlcanzaElGolpe(dx, dy, tamanoBichoCombate, alcanceGolpe) {
			a.Afuera++
			golpesAfuera++

			println("JUEGO122: GOLPE-AFUERA jugador=", a.Nombre, "ataque=", a.Ataques, "cuadroDelHeroe=", f,
				"de", len(ciclo.Cuadros), "dx=", dx, "dy=", dy, "distancia=", a.Distancia, "alcance=", alcanceGolpe,
				"vida=", b.Vida.HP)
		} else {
			dano, antes, despues, muerto := b.Combate.GolpeDelHeroe()

			if dano == 0 {
				// el bicho ya estaba muerto cuando cayo este golpe (otro jugador lo mato en este mismo cuadro):
				// no es un golpe del criterio.
				tarde++

				println("JUEGO122: GOLPE-TARDE jugador=", a.Nombre, "ataque=", a.Ataques, "vida=", b.Vida.HP,
					"golpesPrevios=", b.Combate.Golpes)
			} else {
				a.Golpes++
				golpesDados++
				a.Dano += dano
				danoDelCombate += dano

				println("JUEGO122: GOLPE jugador=", a.Nombre, "ataque=", a.Ataques, "cuadroDelHeroe=", f, "de",
					len(ciclo.Cuadros), "dano=", dano, "vidaAntes=", antes, "vidaDespues=", despues, "muerto=", muerto,
					"dx=", dx, "dy=", dy, "distancia=", a.Distancia, "alcance=", alcanceGolpe,
					"danoAcumulado=", b.Combate.Dano)

				if muerto {
					muertes++
					cambiarModoDelBicho(b, modoBestiaMuriendo, ahora)

					println("JUEGO122: MUERTE vida=", b.Vida.HP, "golpes=", b.Combate.Golpes, "dano=",
						b.Combate.Dano, "modo=", b.Modo, "ultimoJugador=", a.Nombre, "cuadro=", cuadro)
				} else if b.Modo != modoBestiaGolpeada && b.Modo != modoBestiaMuriendo {
					cambiarModoDelBicho(b, modoBestiaGolpeada, ahora)
				}
			}
		}
	}

	// el cuadro del A1 que se dibuja (el reloj puede pasarse del final: se clava en el ultimo)
	a.Cuadro = f
	if a.Cuadro >= len(ciclo.Cuadros) {
		a.Cuadro = len(ciclo.Cuadros) - 1
	}

	if f >= len(ciclo.Cuadros) {
		a.Golpeando = false
		a.FinMs = ahora
		a.FinCuadro = cuadro
		a.Cuadro = len(ciclo.Cuadros) - 1

		println("JUEGO122: GOLPE-FIN jugador=", a.Nombre, "ataque=", a.Ataques, "vida=", b.Vida.HP,
			"cuadrosDelGolpe=", len(ciclo.Cuadros), "msDelCiclo=", len(ciclo.Cuadros)*ciclo.MsPorCuadro,
			"ultimaPoseDibujada=", a.Cuadro)
	}
}

// separacionDelBicho: la separacion en subtiles enteros del jugador al bicho (la que come la tabla del exe).
func separacionDelBicho(p *participante, b *bicho) (int, int) {
	if p.Mov == nil || p.Mov.Est == nil || b.Est == nil {
		return 0, 0
	}

	return DistanciaEnSubtiles(p.Mov.Est.Pos.X(), p.Mov.Est.Pos.Y(), b.Est.Pos.X(), b.Est.Pos.Y())
}

// direccionHaciaElBicho: la direccion del COF del heroe que mira al bicho, con la cadena del motor
// (`DirectionTo` -> `Dir64ToCof`). Es la que usa `advanceHeroAttack` al pegar.
func direccionHaciaElBicho(p *participante, b *bicho) int {
	if p.Mov == nil || p.Mov.Est == nil || b.Est == nil {
		return p.Mov.Direccion
	}

	px, py := p.Mov.Est.Celda()
	bx, by := b.Est.Celda()

	dx, dy := bx-px, by-py
	if dx == 0 && dy == 0 {
		return p.Mov.Direccion
	}

	origen := d2vector.NewVector(0, 0)
	destino := d2vector.NewVector(dx, dy)

	return d2cof.Dir64ToCof(origen.DirectionTo(*destino), direccionesDelCOFDelHeroe)
}

// ============================== EL MODO DEL BICHO (GH / DT / DD) ==============================

// pasoDelModoDelBicho: el reloj del modo del bicho. GH vuelve a caminar cuando termina y DT pasa a DD, que es la
// regla de los modos de un solo tiro del motor (`d2mapentity/monster.go`, checkEvents).
func pasoDelModoDelBicho(b *bicho, ahora int64) {
	if b == nil || b.Modo == "" || b.Modo == modoBichoCamina {
		return
	}

	c := b.CicloDelModo
	if c == nil || len(c.Cuadros) == 0 {
		return
	}

	ms := ahora - b.MsModo

	switch b.Modo {
	case modoBestiaGolpeada:
		if ms >= int64(len(c.Cuadros)*c.MsPorCuadro) {
			cambiarModoDelBicho(b, modoBichoCamina, ahora)
		}
	case modoBestiaMuriendo:
		if ms >= int64(len(c.Cuadros)*c.MsPorCuadro) {
			cambiarModoDelBicho(b, modoBestiaMuerta, ahora)
		}
	}
}

// cambiarModoDelBicho: el bicho cambia de animacion y el reloj del modo arranca de cero.
func cambiarModoDelBicho(b *bicho, modo string, ahora int64) {
	b.Modo = modo
	b.MsModo = ahora

	b.CicloDelModo = cicloDelModoDelBicho(b, modo)

	if b.CicloDelModo == nil {
		println("JUEGO122: MODO-BICHO SIN-CICLO modo=", modo)

		return
	}

	println("JUEGO122: MODO-BICHO modo=", modo, "cuadros=", len(b.CicloDelModo.Cuadros),
		"msPorCuadro=", b.CicloDelModo.MsPorCuadro, "vida=", vidaDelBichoEnTexto(b))
}

// cicloDelModoDelBicho: el ciclo compuesto de un modo del bicho en la direccion que mira (los modos de pelea se
// componen en las dos direcciones de la patrulla; si la que mira no esta, se usa la primera que haya).
func cicloDelModoDelBicho(b *bicho, modo string) *CicloDelJugador {
	if modo == modoBichoCamina || modo == modoBichoQuieto {
		if c := ciclosBicho[b.Direccion]; c != nil {
			return c
		}

		return cicloBichoQuieto
	}

	if c := ciclosModoBicho[modo]; c != nil {
		if ciclo := c[b.Direccion]; ciclo != nil {
			return ciclo
		}

		for _, ciclo := range c {
			return ciclo
		}
	}

	return cicloBichoQuieto
}

// vidaDelBichoEnTexto: la vida del bicho para el log (la consola no tiene fmt).
func vidaDelBichoEnTexto(b *bicho) int {
	if b.Vida == nil {
		return -1
	}

	return b.Vida.HP
}

// ============================== LA VIDA Y LOS MODOS DEL GOLPE (DC-002.122) ==============================

// cargarCombateDelBicho: le pone vida a cada bicho (por el camino de siembra del motor) y compone los tres modos
// del golpe (GH recibiendo, DT muriendo, DD muerto) en las dos direcciones de la patrulla.
//
// LA VIDA: `monstruo.go: VidaDelBicho` con monstats.txt (d2data) y MonLvl.txt (patch_D2: medido en DC-002.50, no
// esta en d2data ni en d2exp), con el mismo dado del motor y el piso de 1 HP. Es el MISMO archivo que compila el
// verificador del host, asi que los numeros de la consola y los del host se comparan.
//
// EL TAMANO PARA LA TABLA DE DISTANCIAS: 1. Es el valor por omision de `d2skill.UnitSizeOf` para un objetivo que
// no dice su tamaño, y el SizeX de verdad vive en monstats2.txt, otra tabla que este POC no lee (300 KB mas en el
// semiespacio, que esta al limite). Se dice aca para que el numero del alcance se pueda auditar.
func cargarCombateDelBicho(datos *fuenteconsola.Fuente, ida, vuelta int) {
	t0 := int64(d2cMs())

	tamanoBichoCombate = 1

	// ---- 1. LA VIDA (las dos tablas del excel) ----
	vida := vidaDelBichoDeLasTablas()
	if vida == nil {
		println("JUEGO122: COMBATE SIN VIDA DEL BICHO: el golpe no entra (el bicho sigue patrullando)")

		return
	}

	// EL INSTRUMENTO DE LA VIDA (solo para la corrida de los cuatro): el Returned de la tabla aguanta 2..3 HP, o
	// sea uno o dos golpes, y con un solo bicho los cuatro jugadores no llegan a pegarle. Se multiplica la vida y
	// el numero queda en el log: la vida de la tabla no se esconde.
	if vidaMultiplicada > 1 {
		deLaTabla := vida.HP
		vida.HP *= vidaMultiplicada
		vida.Max *= vidaMultiplicada

		println("JUEGO122: COMBATE VIDA INSTRUMENTO deLaTabla=", deLaTabla, "factor=", vidaMultiplicada,
			"HP=", vida.HP)
	}

	for _, b := range bichos {
		b.Vida = copiaDeLaVida(vida)

		// el dado del heroe, por el MISMO XOR que usa `heroRoller` del motor (monstruo.go: NuevoCombate). Los
		// cuatro jugadores pegan con ESTE dado (el motor tiene UN rodillo de heroe: `Director.heroRoller`), asi
		// que la secuencia de daños es la del motor y no una elegida.
		b.Combate = NuevoCombate(b.Vida, semillaCombate)
		b.Modo = modoBichoCamina

		println("JUEGO122: COMBATE VIDA nombre=", b.Nombre, "clase=", b.Vida.Clase, "nivel=", b.Vida.Nivel,
			"min=", b.Vida.Min, "max=", b.Vida.Max, "HP=", b.Vida.HP, "AC=", b.Vida.AC,
			"danoDelBicho=", b.Vida.DanoMin, "..", b.Vida.DanoMax)
	}

	// ---- 2. LOS TRES MODOS DEL GOLPE ----
	ciclosModoBicho = make(map[string]map[int]*CicloDelJugador, 3)

	// LAS DIRECCIONES DE LOS MODOS DEL GOLPE: las de la patrulla, o solo la primera si la palanca dice 1.
	// MEDIDO (2026-11-02): el DT del Returned son 19 cuadros de 74x102 (143.412 B de indices POR direccion) y el
	// semiespacio no aguanta las dos: con las dos direcciones la imagen viva queda al 98 % y el recolector entra
	// en bucle (el POC tarda 15 minutos en cargar y muere). Con una direccion queda al 86 % y entra. Si el bicho
	// muere mirando a la otra direccion, `cicloDelModoDelBicho` dibuja esta misma y lo dice en el log.
	direccionesDelModo := []int{ida}
	if dirsModoBicho > 1 {
		direccionesDelModo = append(direccionesDelModo, vuelta)
	}

	for _, m := range []struct {
		modo string
		ms   int
	}{
		{modoBestiaGolpeada, msBichoGolpeado},
		{modoBestiaMuriendo, msBichoMuriendo},
		{modoBestiaMuerta, msBichoMuerta},
	} {
		porDireccion := make(map[int]*CicloDelJugador, len(direccionesDelModo))

		for _, d := range direccionesDelModo {
			ciclo := componerCicloDelBicho(datos, m.modo, d, m.ms)
			if ciclo == nil || len(ciclo.Cuadros) == 0 {
				println("JUEGO122: COMBATE el modo", m.modo, "salio vacio dir=", d)

				continue
			}

			porDireccion[d] = ciclo
			bytesBichoCompartidos += bytesDeLosCuadros(ciclo)

			println("JUEGO122: COMBATE MODO", m.modo, "dir=", d, "cuadros=", len(ciclo.Cuadros),
				"msPorCuadro=", m.ms, "indices=", bytesDeLosCuadros(ciclo))
		}

		if len(porDireccion) > 0 {
			ciclosModoBicho[m.modo] = porDireccion
		}
	}

	println("JUEGO122: COMBATE CICLOS modos=", len(ciclosModoBicho), "indicesCompartidosBytes=",
		bytesBichoCompartidos, "ms=", int64(d2cMs())-t0, "alcanceSubtiles=", alcanceGolpe,
		"tamanoDelBicho=", tamanoBichoCombate, "semilla=", semillaCombate)
}

// vidaDelBichoDeLasTablas: los bytes de monstats.txt (d2data) y MonLvl.txt (patch_D2) leidos por la Source de la
// consola, y la vida por el camino del motor.
func vidaDelBichoDeLasTablas() *Vida {
	datos := fuenteDe(rutaDatos)
	parche := fuenteDe(rutaParche)

	if datos == nil || parche == nil {
		println("JUEGO122: COMBATE VIDA sin fuente de datos o de parche")

		return nil
	}

	monstats, okMon := leerTablaDelExcel(datos, "data\\global\\excel\\monstats.txt")

	monlvl, okLvl := leerTablaDelExcel(parche, "data\\global\\excel\\MonLvl.txt")
	if !okMon || !okLvl {
		return nil
	}

	vida, err := VidaDelBicho(monstats, monlvl, claseBestia, semillaCombate)
	if err != nil {
		println("JUEGO122: COMBATE VIDA FALLO:", err.Error())

		return nil
	}

	return vida
}

// leerTablaDelExcel: los bytes de una tabla del excel, por la Source de la consola (se libera el tramo en el acto:
// el llamador se queda con una copia, que es la que `d2txt` necesita para parsear).
func leerTablaDelExcel(fuente *fuenteconsola.Fuente, ruta string) ([]byte, bool) {
	tramo, err := fuente.Pedir(ruta)
	if err != nil || len(tramo.Datos) == 0 {
		println("JUEGO122: COMBATE TABLA-LEER FALLO:", ruta)

		return nil, false
	}

	copia := make([]byte, len(tramo.Datos))
	copy(copia, tramo.Datos)
	tramo.Liberar()

	println("JUEGO122: COMBATE TABLA", ruta, "bytes=", len(copia), "crc32=", int(crc32.ChecksumIEEE(copia)))

	return copia, true
}

// copiaDeLaVida: una vida propia por bicho (la HP es de cada uno; la tabla es la misma).
func copiaDeLaVida(v *Vida) *Vida {
	copia := *v

	return &copia
}

// ============================== EL DIBUJO DEL ATAQUE ==============================

// cuadroDeAtaqueDeUnJugador: si el jugador esta en medio del A1, el ciclo y el cuadro que le tocan; si no,
// `nil` (y el que llama sigue por el camino de siempre). Se usa en el dibujo de los cuatro.
func cicloYCuadroDeAtaque(p *participante, i int) (*CicloDelJugador, *CuadroDelJugador) {
	if !combateOK || combateKnob != "si" || i >= len(ataques) {
		return nil, nil
	}

	a := ataques[i]
	if a == nil {
		return nil, nil
	}

	// DC-002.122: la ULTIMA pose del A1 se dibuja tambien en el cuadro en el que el ataque termina. Con el cuadro
	// del POC a ~1 s y el A1 de 520 ms, la animacion se muestrea una o dos veces: sin esto la ultima pose (la que
	// pega) no se veria nunca. No cambia la mecanica: el dano sigue cayendo a la mitad de la animacion.
	if !a.Golpeando && a.FinCuadro != cuadro {
		return nil, nil
	}

	ciclo := cicloDeAtaque(a.Direccion)
	if ciclo == nil || len(ciclo.Cuadros) == 0 {
		return nil, nil
	}

	f := a.Cuadro
	if f < 0 {
		f = 0
	}

	if f >= len(ciclo.Cuadros) {
		f = len(ciclo.Cuadros) - 1
	}

	return ciclo, &ciclo.Cuadros[f]
}

// cicloYCuadroDelJugador: el ciclo y el cuadro que le tocan a un jugador. Si esta en medio de un ataque, el A1 y
// su cuadro (por el reloj del golpe); si no, el camino de siempre (caminando el de su direccion, quieto el NU).
func cicloYCuadroDelJugador(p *participante, i int) (*CicloDelJugador, *CuadroDelJugador) {
	if c, cu := cicloYCuadroDeAtaque(p, i); c != nil && cu != nil {
		contarCuadroDeAtaque(i, ataques[i].Cuadro)

		return c, cu
	}

	ciclo := cicloDelJugador(p.Mov.Direccion, p.Mov.Caminando)
	if ciclo == nil {
		return nil, nil
	}

	return ciclo, p.Mov.CuadroDeLaAnimacion(ciclo)
}

// contarCuadroDeAtaque: lleva la cuenta de cuantos cuadros DISTINTOS del A1 se dibujaron (el numero que dice si la
// animacion se vio o no, con el cuadro del POC a ~1 s).
func contarCuadroDeAtaque(i, f int) {
	if i >= len(ataques) || ataques[i] == nil {
		return
	}

	a := ataques[i]

	if f < 0 {
		f = 0
	}

	if f >= len(a.Vistos) || a.Vistos[f] {
		return
	}

	a.Vistos[f] = true
	a.CuadrosA1++
	cuadrosA1Dibujados++
}

// ============================== EL RESUMEN ==============================

// resumenDelCombate: los numeros del criterio en el cierre de la corrida.
func resumenDelCombate() {
	if combateKnob != "si" {
		println("JUEGO122: COMBATE RESUMEN apagado (combate=", combateKnob, ")")

		return
	}

	b := bichoDePelea()

	muerto := false
	hp := -1
	vidaInicial := -1

	if b != nil && b.Vida != nil {
		hp = b.Vida.HP
		vidaInicial = b.Vida.Max
		muerto = b.Combate != nil && b.Combate.Muerto
	}

	total := ""
	for i := range jugadores {
		if i > 0 {
			total += " "
		}

		a := ataques[i]
		total += a.Nombre + ":ataques=" + itoa(a.Ataques) + ",golpes=" + itoa(a.Golpes) + ",afuera=" +
			itoa(a.Afuera) + ",dano=" + itoa(a.Dano)
	}

	println("JUEGO122: COMBATE RESUMEN jugadores=", len(jugadores), "alcance=", alcanceGolpe, "dirsAtaque=",
		len(ciclosAtaque), "golpesDados=", golpesDados, "golpesAfuera=", golpesAfuera, "danoTotal=",
		danoDelCombate, "tarde=", tarde, "muertes=", muertes, "vidaFinal=", hp, "vidaTablaMax=", vidaInicial,
		"muerto=", muerto,
		"modoFinal=", modoDelBicho(), "cuadrosA1Dibujados=", cuadrosA1Dibujados, "porJugador=", total)

	println("JUEGO122: COMBATE CRITERIO elGolpeBajaVida=", golpesDados > 0, "fueraDeAlcanceNoBaja=",
		golpesAfuera == 0 || golpesDados > 0, "murioConSuAnimacion=", muerto && modoDelBicho() == modoBestiaMuerta,
		"losCuatroPegaron=", len(jugadores) > 1 && jugadoresQuePegaron() == len(jugadores))
}

// modoDelBicho: el modo del bicho de pelea ("" si no hay bicho).
func modoDelBicho() string {
	b := bichoDePelea()
	if b == nil {
		return "-"
	}

	if b.Modo == "" {
		return modoBichoCamina
	}

	return b.Modo
}

// jugadoresQuePegaron: cuantos jugadores distintos le bajaron vida al bicho.
func jugadoresQuePegaron() int {
	n := 0

	for i := range ataques {
		if ataques[i] != nil && ataques[i].Golpes > 0 {
			n++
		}
	}

	return n
}

// EL INSTRUMENTO DEL BOTON. El emulador corre sin ventana y sin teclado, asi que el boton Y no se aprieta
// nunca y la cadena boton -> golpe -> dano -> vida no se podria medir. Con `golpeAuto="si"` el POC simula el
// FLANCO (apretado 0,9 s de cada 2,4 s, por el reloj de la consola) EN LOS BITS QUE SE INYECTAN EN EL PUERTO
// (`leerYAvanzarJugadores`, coop.go), asi que el boton pasa por el maple de verdad: bits -> traduccion del
// servicio -> `KeyY`. Lo unico simulado es de donde salen los bits. No toca el dano ni la vida.
const bitYMaple uint32 = 1 << 9 // CONT_Y del control de la Dreamcast (d2core/d2input/maple, BitY)

// pulsoDeGolpe: si el instrumento tiene el boton apretado en este instante (ms de la consola).
func pulsoDeGolpe(ahora int64) bool {
	return (ahora/100)%24 < 9
}

// El tipo de `ciclosModoBicho` (los GH/DT/DD compuestos, por modo y direccion) se declara en monstruo_121.go,
// junto con el resto del bicho: aca solo se usa.

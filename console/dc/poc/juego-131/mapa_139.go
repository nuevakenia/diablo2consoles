//go:build dreamcast

package main

import "github.com/OpenDiablo2/OpenDiablo2/d2core/d2render/pvr"

// DC-002.139 -- EL MAPA ES ESTATICO: DEJA DE REPINTARSE CUANDO LA CAMARA NO SE MOVIO.
//
// LO MEDIDO EN DC-002.140 (Flycast, 248 tiles): el cuadro tarda 124 ms y el mapa se lleva 68,3 ms; la
// presentacion (RGBA -> RGB565 de los 307.200 pixeles) 25,6; el minmapa 7,2; el panel 3,4; el HUD 2,0; los
// NPCs 1,7 y ~13 ms de lineas por el puerto serie.
//
// POR QUE EL MAPA SE PUEDE SALTAR. El mapa es ESTATICO: en un cuadro en el que la camara esta en el mismo
// pixel que el cuadro anterior, el lienzo ya tiene el mapa dibujado y volver a pintar los 248 tiles da
// exactamente los mismos pixeles. Lo unico que hay que hacer es devolver el mapa debajo de los sprites: el
// motor YA guarda lo que hay debajo de la caja de cada ente (jugadores, bichos, NPCs, objetos) en
// `CajaGuardada` -- es la copia que usa el instrumento por cuadro--, asi que alcanza con devolverla.
//
// EL ORDEN, QUE ES LO QUE HACE QUE NO QUEDEN FANTASMAS:
//
//  1. si la camara se movio: `rendererDelMapa.Render` (el camino de siempre);
//  2. si NO se movio: se devuelve la caja guardada de CADA ente (el mapa vuelve a estar limpio);
//  3. despues el cuadro sigue igual: cada ente guarda SU caja (ya con el mapa limpio debajo) y se pinta.
//
// Como la copia se toma con la caja de ESTE cuadro y se devuelve la del cuadro anterior, un bicho que camina
// no deja rastro: su posicion vieja se restaura y la nueva se guarda.
//
// LAS PALANCAS (knobs.go):
//
//	mapaEstatico = "si" | "no"  -- "si" prende el salto; "no" es DC-002.140, bit a bit
//	cajas139     = "si" | "no"  -- "si" deja encendido el guardar/devolver la caja de cada ente (lo necesita
//	                               el salto, y tambien deja el crc32 del mundo de los tres primeros cuadros)

// El estado del salto: la camara del ultimo cuadro que se pinto entero.
var (
	mapa139X, mapa139Y int
	mapa139Valido      bool
	// mapa139Dialogo: si el panel estaba abierto en el cuadro anterior (abrir/cerrar obliga a repintar).
	mapa139Dialogo bool
	// mapa139Nivel: el nivel que se pinto (un cambio de nivel obliga a repintar).
	mapa139Nivel string
	// el censo del tramo, para la evidencia
	mapa139Saltados    int
	mapa139Repintados  int
	mapa139Restaurados int
)

// mapaSeRepinta139: si este cuadro hay que pintar el mapa entero (el camino de DC-002.140).
func mapaSeRepinta139() bool {
	if mapaEstatico != "si" || cajas139 != "si" {
		return true
	}

	if !mapa139Valido || cuadro <= 3 {
		return true
	}

	if rendererDelMapa == nil {
		return true
	}

	// un nivel nuevo (la salida del pueblo) es otro mapa
	if demoNivelActual != mapa139Nivel {
		return true
	}

	// abrir o cerrar el panel cambia lo que tapa el mapa
	if dialogoAbierto != mapa139Dialogo {
		return true
	}

	// los tiles animados (el agua) no se pueden congelar: si el nivel tiene alguno, el mapa se repinta
	if rendererDelMapa.TilesAnimados139() > 0 {
		return true
	}

	x, y := rendererDelMapa.TraslacionDeLaPantalla139()

	return x != mapa139X || y != mapa139Y
}

// anotarElMapaPintado139: la camara de este cuadro queda como la pintada. Se llama DESPUES de pintar (o
// despues de devolver las cajas, que deja el lienzo igual que un repintado).
func anotarElMapaPintado139() {
	if rendererDelMapa == nil {
		return
	}

	mapa139X, mapa139Y = rendererDelMapa.TraslacionDeLaPantalla139()
	mapa139Valido = true
	mapa139Dialogo = dialogoAbierto
	mapa139Nivel = demoNivelActual
}

// restaurarElMundo139: devuelve el mapa debajo de la caja de cada ente (lo que se guardo el cuadro anterior).
// Devuelve cuantas cajas devolvio.
//
// OJO CON EL ORDEN: se devuelve en el orden inverso al que se pinto (de atras para adelante), igual que hace
// el camino de medir. Dos cajas que se pisan devuelven el MISMO mapa (las dos guardaron pixeles del mapa),
// asi que el orden no cambia el resultado; se respeta por simetria con el camino medido.
// cajaLista139: si la caja y su copia se pueden devolver SIN salirse de nada.
//
// POR QUE ESTA GUARDA ES OBLIGATORIA (medido: la primera corrida de esta tarjeta se cayo con "Error del bus"
// en el cuadro ~42): la copia se tomo con la caja de SU cuadro, y una caja de otro cuadro (otra direccion del
// sprite, otro cuadro del ciclo, el ente fuera de pantalla) puede ser MAS GRANDE que la copia. Devolver una
// caja asi escribe fuera del buffer de la copia y, como el lienzo vive en la VRAM, el SH-4 se lleva un bus
// error en vez de un mensaje. La guarda no cambia nada cuando las cajas coinciden (el caso normal).
func cajaLista139(c CajaDelJugador, copia []byte) bool {
	if copia == nil || c.Ancho() <= 0 || c.Alto() <= 0 {
		return false
	}
	if c.X0 < 0 || c.Y0 < 0 || c.X1 > ancho || c.Y1 > alto {
		return false
	}
	return len(copia) >= c.Ancho()*c.Alto()*4
}

func restaurarElMundo139(pixeles []byte) int {
	n := 0

	for i := len(jugadores) - 1; i >= 0; i-- {
		p := jugadores[i]
		if cajaLista139(p.Caja, p.CajaGuardada) {
			restaurarCajaDe(p, pixeles)
			n++
		}
	}

	for i := len(bichos) - 1; i >= 0; i-- {
		b := bichos[i]
		if cajaLista139(b.Caja, b.CajaGuardada) {
			restaurarCajaDelBicho(b, pixeles)
			n++
		}
	}

	for i := len(npcsDelPueblo) - 1; i >= 0; i-- {
		x := npcsDelPueblo[i]
		if cajaLista139(x.Caja, x.CajaGuardada) {
			restaurarCajaDelNPC(x, pixeles)
			n++
		}
	}

	for i := len(suelo) - 1; i >= 0; i-- {
		it := suelo[i]
		if cajaLista139(it.Caja, it.CajaGuardada) {
			restaurarCajaDelItem(it, pixeles)
			n++
		}
	}

	mapa139Restaurados += n

	return n
}

// informeDelMapaEstatico139: el censo del tramo (cada `cuadrosDelInforme132` cuadros), pegado a la linea de
// las etapas para que se lea como su desglose.
func informeDelMapaEstatico139() {
	if cuadro%cadaInforme143() != 0 {
		return
	}

	conv, salt := pvr.CensoDeLaPresentacionPorBloques()

	println("JUEGO139: MAPA-ESTATICO cuadros=", cuadro, "saltados=", mapa139Saltados, "repintados=",
		mapa139Repintados, "cajasDevueltas=", mapa139Restaurados, "palanca=", mapaEstatico, "cajas=", cajas139,
		"convBloques=", convBloques139, "bloquesConvertidos=", conv, "bloquesSaltados=", salt,
		"marcados=", pvr.BloquesMarcados())
}

// ============================== DC-002.139: LA PRESENTACION POR BLOQUES ==============================
//
// Con el mapa estatico el lienzo casi no cambia entre dos cuadros, asi que convertir el cuadro entero (23,6 ms
// medidos) es trabajo tirado. Lo que se marca aca es lo que el cuadro TOCA, y el presentador convierte solo
// esos bloques de 32x32 (ver `d2core/d2render/pvr/sucios_139.go`).
//
// SE MARCA DE MAS A PROPOSITO. Un rectangulo marcado de mas cuesta una conversion de 32x32 (nada); uno marcado
// de menos deja un pedazo de cuadro viejo en pantalla. Por eso los paneles se marcan con su caja entera y el
// HUD con dos franjas que se comen lo que no es HUD.

// mapa139PintadoEsteCuadro: si el mapa se pinto entero en ESTE cuadro (entonces hay que convertir todo).
var mapa139PintadoEsteCuadro bool

// mapa139CeldaX/Y: la celda del jugador del cuadro anterior (los paneles dependen de ella).
var mapa139CeldaX, mapa139CeldaY float64

// marcarLoQueCambio139: los rectangulos que hay que convertir en este cuadro.
func marcarLoQueCambio139() {
	if convBloques139 != "si" {
		return
	}

	// el mapa repintado entero: cambio todo el lienzo
	if mapa139PintadoEsteCuadro {
		pvr.MarcarSucio(0, 0, ancho, alto)

		return
	}

	// los entes: su caja es lo que se pinto encima del mapa
	for _, p := range jugadores {
		pvr.MarcarSucio(p.Caja.X0, p.Caja.Y0, p.Caja.X1, p.Caja.Y1)
	}

	for _, b := range bichos {
		pvr.MarcarSucio(b.Caja.X0, b.Caja.Y0, b.Caja.X1, b.Caja.Y1)
	}

	for _, x := range npcsDelPueblo {
		pvr.MarcarSucio(x.Caja.X0, x.Caja.Y0, x.Caja.X1, x.Caja.Y1)
	}

	for _, it := range suelo {
		pvr.MarcarSucio(it.Caja.X0, it.Caja.Y0, it.Caja.X1, it.Caja.Y1)
	}

	// LOS PANELES SOLO CUANDO PUEDEN HABER CAMBIADO. Con la camara quieta y el jugador en la misma celda el HUD
	// (los globos, el cinturon) y el minimapa (las celdas y el marcador) pintan lo MISMO, asi que sus 190
	// bloques no hay que convertirlos. Se marcan cuando el mapa se repinto, cuando el jugador cambio de celda y
	// cada 30 cuadros (red de seguridad: 1,5 s al ritmo de ahora).
	cx, cy := celdaDelJugadorX(), celdaDelJugadorY()
	cambioLaCelda := cx != mapa139CeldaX || cy != mapa139CeldaY
	mapa139CeldaX, mapa139CeldaY = cx, cy

	if cambioLaCelda || cuadro%30 == 0 {
		// la franja de arriba del HUD, los paneles de abajo y la caja del minimapa (279x225 pegada a la
		// esquina de arriba a la derecha, medido: 359,57 .. 638,282).
		pvr.MarcarSucio(0, 0, ancho, 64)
		pvr.MarcarSucio(0, alto-80, ancho, alto)
		pvr.MarcarSucio(ancho-290, 48, ancho, 290)
	}

	// EL PANEL DEL DIALOGO O DE LA QUEST: se marca SU CAJA, y solo cuando el panel CAMBIA (se abre, cambia de
	// pagina o se cierra). Antes se marcaba el lienzo ENTERO por tener un panel abierto, y eso cuesta la
	// conversion completa: medido en esta tarjeta, conv_us=22.765 con bloques=300 por el panel de la quest abierto,
	// el 47 % de un cuadro de 48 ms, para reconvertir 1,2 MB que no habian cambiado. El panel es opaco y se arma
	// una vez, asi que su caja no cambia mientras sea el mismo panel. Al CERRARSE no hay que marcar nada: el mapa
	// lo repinta el cuadro siguiente (`mapaSeRepinta139` compara `dialogoAbierto` y pide repintado entero).
	if dialogoAbierto && elDialogo != nil {
		if elDialogo != panelMarcado143 {
			panelMarcado143 = elDialogo

			pvr.MarcarSucio(elDialogo.X0, elDialogo.Y0, elDialogo.X0+elDialogo.Ancho, elDialogo.Y0+elDialogo.Alto)
		}
	} else {
		panelMarcado143 = nil
	}

	// DC-002.143: el minimapa que se PINTO tiene que llegar a la pantalla aunque el heroe no haya cambiado de
	// celda (un NPC que camino por su caja obliga a repintar el panel). El que se salteo no marca nada: sus
	// pixeles son los del cuadro anterior.
	if minimapaCambio143 == "si" && minimapaKnob == "si" && minimapaPintoEsteCuadro143 {
		pvr.MarcarSucio(ancho-anchoDelMinimapa141, arribaDelMinimapa141, ancho, abajoDelMinimapa141)
	}
}

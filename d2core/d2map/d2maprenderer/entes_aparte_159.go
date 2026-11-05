package d2maprenderer

// DC-002.159 -- LOS ENTES APARTE: EL RENDERER PINTA EL MAPA, EL QUE LLAMA PINTA EL HEROE.
//
// EL PROBLEMA QUE RESUELVE. El mapa estatico (DC-002.157) deja el cuadro en 15,1 ms porque no repinta el
// mapa cuando la vista no se movio. Pero su condicion incluye "no hay entes": apenas entra el heroe,
// `len(mapEngine.Entities()) > 0` apaga el salto y el cuadro vuelve a los 818 ms medidos en DC-002.157
// (822 de mapa + 57 de conversion), que es el muro que esta tarjeta tiene que esquivar.
//
// LA SALIDA. El mapa se pinta UNA vez y se queda ahi; el ente se dibuja encima cada cuadro, y antes de
// dibujarlo se devuelve al lienzo el recuadro que tapaba (la memoria del lienzo, `lienzoConMemoria159`).
// Es el borrado de los juegos 2D de siempre, y cuesta unos KB por cuadro en vez de 818 ms.
//
// LO QUE SE PIERDE, Y SE DICE. El motor dibuja los entes ENTRE las pasadas del mapa (los que estan debajo
// de las paredes en la pasada 2, el resto en la 3): con esta palanca el ente va ENCIMA de todo el mapa.
// En el pueblo del Gate 0 eso significa que el heroe no queda tapado por una pared o un techo cuando pasa
// por detras. Es la unica diferencia observable, y es el precio de que el mapa no se repinte; cuando el
// repintado del mapa baje de ~30 ms (la tarjeta del scroll), esta palanca se apaga y el ente vuelve a su
// lugar entre las paredes.
//
// ES ADITIVA: sin `ActivarEntesAparte159(true)` el renderer hace exactamente lo de antes.

import "github.com/OpenDiablo2/OpenDiablo2/d2common/d2interface"

// ActivarEntesAparte159: el renderer pinta solo el mapa; los entes los dibuja el que llama con
// `DibujarEnte159`. Al prenderla se apaga el indice de entes por casilla (no se arma) y al apagarla se
// olvida el mapa pintado, para que el cuadro siguiente vuelva a salir completo.
func (mr *MapRenderer) ActivarEntesAparte159(si bool) {
	if mr.entesAparte159 == si {
		return
	}

	mr.entesAparte159 = si
	mr.mapa157Valido = false
}

// EntesAparte159: si la palanca esta puesta.
func (mr *MapRenderer) EntesAparte159() bool { return mr.entesAparte159 }

// DibujarEnte159: dibuja UN ente con la misma traslacion que usan las pasadas del renderer: se apila la
// posicion del ente en el mundo y despues la traslacion a PANTALLA que sale del viewport (la que ya lleva la
// camara). Son las DOS las que hacen que el ente caiga en el mismo pixel que caia dentro de la pasada 2/3.
//
// DC-002.159b: FALTABA LA SEGUNDA. Con solo la traslacion del mundo el ente se dibujaba en el origen del
// lienzo (arriba a la izquierda, cortado por el borde) mientras la camara lo miraba en el centro: medido en el
// Gate 0 con el heroe naciendo en la celda 26.5,7.5 (`pie=320,232` segun `PieDeLaPantalla`, y el sprite pegado
// al borde izquierdo en la captura). El renderer hace `target.PushTranslation(mr.viewport.GetTranslationScreen())`
// antes de `renderLitEntity` (ver `renderPass2`), que es exactamente esta linea.
func (mr *MapRenderer) DibujarEnte159(target d2interface.Surface, e d2interface.MapEntity) {
	if e == nil {
		return
	}

	pos := e.GetPosition()

	mr.viewport.PushTranslationWorld(pos.X()/subtilesPerTile, pos.Y()/subtilesPerTile)
	target.PushTranslation(mr.viewport.GetTranslationScreen())
	e.Render(target)
	target.Pop()
	mr.viewport.PopTranslation()
}

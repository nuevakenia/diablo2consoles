package d2maprenderer

// DC-002.157 -- EL MAPA ES ESTATICO: SI NADA DE LO QUE LO PINTA CAMBIO, NO SE REPINTA.
//
// EL NUMERO QUE LO PIDE (Flycast, Gate 0 de DC-002.156): 1.869 / 1.885 / 1.883 ms por cuadro, y de eso
// 881 ms medidos en el dibujado del motor y 58 ms en la conversion del cuadro. El mapa es el 95 % del cuadro,
// y el mapa del pueblo NO CAMBIA: en un cuadro en el que la camara esta en el mismo pixel que el cuadro
// anterior, volver a recorrer y pintar las ~2.700 celdas de la cache da exactamente los mismos pixeles.
//
// ES LA PALANCA DE DC-002.139, que en el POC jugable dejo el mapa en 4,2 ms (y en 0 cuando no se movia):
// `JUEGO134: ETAPAS mapa= 70324 -> 4246` en la evidencia de DC-002.143. Aca vive en el RENDERER del motor
// (no en el POC) para que valga para el camino de `d2app`, que es el que dibuja el Gate 0.
//
// QUE TIENE QUE SER IGUAL PARA PODER SALTARSE EL CUADRO. El mapa deja de pintarse SOLO si nada de lo que
// escribe en el lienzo cambio:
//
//   - la traslacion de la vista (la camara) en pixeles de pantalla -- el mismo numero, los mismos pixeles;
//   - que no haya NINGUN tile animado (agua, antorchas): su cuadro lo mueve el reloj, no la camara;
//   - que no haya entidades en el mapa: con entes el cuadro no es solo el mapa (los entes van entre las
//     paredes) y devolver el mapa sin ellos dejaria fantasmas. El Gate 0 no tiene entes;
//   - que la luz este apagada (con luz el cuadro depende del heroe y de las antorchas);
//   - que no haya shake de pantalla, ni los niveles de debug encendidos (los dos pintan cosas que no son
//     el mapa encima del mapa).
//
// EL LIENZO ES UN SUPUESTO DEL QUE LLAMA, Y SE DICE. El mapa que se salta esta en el lienzo donde se pinto.
// En el motor ese lienzo es la imagen de la pantalla, que se reusa cuadro a cuadro (la Surface se crea de
// nuevo por cuadro pero envuelve LA MISMA imagen), asi que el supuesto se cumple. Si el que prende la
// palanca cambia de lienzo -- otra pantalla, otro tamano -- tiene que llamar a `OlvidarElMapa157()`; si no,
// el cuadro siguiente se salta y el mapa no esta ahi.
//
// ES ADITIVA: sin `ActivarMapaEstatico(true)` el renderer hace exactamente lo de antes, cuadro por cuadro.

// ActivarMapaEstatico: prende (o apaga) el salto del mapa. Al apagarlo se olvida lo pintado, asi que el
// cuadro siguiente se repinta entero.
func (mr *MapRenderer) ActivarMapaEstatico(si bool) {
	mr.mapaEstatico157 = si

	if !si {
		mr.mapa157Valido = false
	}
}

// MapaEstatico: si el salto esta prendido.
func (mr *MapRenderer) MapaEstatico() bool { return mr.mapaEstatico157 }

// OlvidarElMapa157: el mapa que se creia pintado ya no esta en el lienzo (cambio la pantalla o el lienzo).
func (mr *MapRenderer) OlvidarElMapa157() { mr.mapa157Valido = false }

// SaltosDelMapa157: cuantos cuadros se salto el mapa desde que existe este renderer.
func (mr *MapRenderer) SaltosDelMapa157() int { return mr.saltos157 }

// RepintadosDelMapa157: cuantos cuadros se repintieron enteros.
func (mr *MapRenderer) RepintadosDelMapa157() int { return mr.repintados157 }

// puedeSaltarseElMapa157: si este cuadro el mapa YA ESTA pintado en el lienzo y nada lo cambio.
func (mr *MapRenderer) puedeSaltarseElMapa157() bool {
	if !mr.mapaEstatico157 || !mr.mapa157Valido {
		return false
	}

	// la luz y el shake mueven el cuadro sin mover la camara
	if mr.light.active() || mr.shake.on {
		return false
	}

	// el debug pinta encima del mapa
	if mr.mapDebugVisLevel > 0 || mr.entityDebugVisLevel > 0 {
		return false
	}

	// con entes el cuadro no es solo el mapa (van entre las paredes)
	if len(mr.mapEngine.Entities()) > 0 {
		return false
	}

	// un tile animado lo mueve el reloj, no la camara
	if tilesAnimados139 > 0 {
		return false
	}

	x, y := mr.viewport.GetTranslationScreen()
	if x != mr.mapa157X || y != mr.mapa157Y {
		return false
	}

	return true
}

// anotarElMapaPintado157: el cuadro quedo pintado en el lienzo, en esta traslacion.
func (mr *MapRenderer) anotarElMapaPintado157() {
	mr.mapa157X, mr.mapa157Y = mr.viewport.GetTranslationScreen()
	mr.mapa157Valido = true
	mr.repintados157++
}

// contarSalto157: el cuadro no se pinto (el mapa ya estaba).
func (mr *MapRenderer) contarSalto157() { mr.saltos157++ }

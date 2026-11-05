package d2maprenderer

// DC-002.159b -- EL MAPA QUE YA NO SE PUEDE REPINTAR: EL LIENZO ES LO UNICO QUE QUEDA.
//
// POR QUE EXISTE. El que llama devuelve la memoria del mapa (las casillas del motor y el arena de bloques de los
// `.dt1`) cuando el mapa ya esta pintado y no se va a repintar (ver `d2mapengine.SoltarLasCasillasDelMapa` y
// `d2asset.SoltarArenaDeBloques`). Desde ese momento `d2mapengine` no tiene nada que dibujar: si el renderer
// volviera a pintar el mapa, pintaria un mundo vacio ENCIMA del pueblo que esta en el lienzo.
//
// LO QUE HACE. Marca el cuadro como "soltado": `puedeSaltarseElMapa157` devuelve verdadero SIEMPRE, con lo que
// el mapa que esta en el lienzo se queda como esta (es el mismo camino que ya usan los cuadros estaticos de
// DC-002.157, solo que sin vuelta atras).
//
// EL CONTRATO. Solo se puede soltar si el mapa quedo pintado en ESTE lienzo (`mapa157Valido`): si no, no hay
// nada que conservar y la funcion lo dice devolviendo falso. El que llama tiene que haber prendido el mapa
// estatico y dibujar los entes aparte (el mapa no se repinta: el ente se pinta y se borra encima).

// SoltarElMapaPintado159: a partir de este cuadro el mapa NO se vuelve a pintar, pase lo que pase. Devuelve
// falso (y no toca nada) si el mapa no esta pintado en el lienzo o el renderer no es de este camino.
func (mr *MapRenderer) SoltarElMapaPintado159() bool {
	if mr == nil || !mr.mapa157Valido {
		return false
	}

	mr.mapaSoltado159 = true
	mr.mapaEstatico157 = true

	return true
}

// MapaSoltado159 dice si el mapa se solto (lo consulta el instrumento).
func (mr *MapRenderer) MapaSoltado159() bool { return mr.mapaSoltado159 }

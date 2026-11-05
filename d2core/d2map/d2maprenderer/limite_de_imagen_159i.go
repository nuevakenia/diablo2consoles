package d2maprenderer

// DC-002.159i -- LO QUE NO ENTRA SE DICE Y SE SIGUE: el techo de UNA imagen de la cache del mapa.
//
// EL DEFECTO QUE CIERRA. Con el runtime sano (DC-002.159h) la corrida del Gate 0 del pueblo muere igual, ~50 s,
// en la pantalla del mapa y antes de crear la pantalla del juego, pidiendo UNA imagen de la cache del mapa:
//
//	ALLOC-TRAZA makeslice size=11528600 tipo=uint8 elemsize=1 pc=8c1f0f32
//	panic: makeslice: too large
//	Memory: arena=10257344 used=10218432 free=38912
//
// y, en otra corrida, el mismo pedido por la puerta de los bloques grandes:
//
//	TRAZA159H externa size=5922200 ra=8c18c4da  ->  d2maprenderer.(*MapRenderer).bufferAlVuelo
//	fatal error: gc_external_alloc: OOM
//
// O sea: `pedirTrozoDeIndices` recibe un `n` que no entra en la RAM de la consola (1 B por pixel de la imagen),
// ningun camino del tramo lo puede servir y el ultimo recurso -- el buffer de trabajo al vuelo -- CRECE hasta
// `n + n/4` para poder servirlo. En la consola ese `make` es una asignacion externa: o el monton no la sirve
// (`gc_external_alloc: OOM`) o el recolector ya no tiene donde meterla (`makeslice: too large`).
//
// LA REGLA DEL REPO ES QUE LA CONSOLA NO MIENTE. Si una imagen no entra, no se muere ni se pinta cualquier cosa:
// se dice en el log, UNA vez por imagen, y el cuadro sigue sin ella. Eso es lo que hace este archivo, y lo hace
// en UN solo lugar: el techo (`limiteDeImagen`) se declara desde el arranque y `pedirTrozoDeIndices` lo aplica
// ANTES de pedir un byte, asi que ningun camino (tramo, bloque nuevo, al vuelo, derrame) puede pedir de mas.
//
// POR QUE NO ES UN LIMITE DE LA CACHE SINO DE LA IMAGEN. Un techo sobre el TOTAL de la cache ya existe (el tramo y
// su techo, DC-002.106). Lo que tumbaba la corrida es UNA imagen sola, y el total no la ve venir: con 1,4 MB de
// tramo, una imagen de 4,7 MB no entra ni con la cache vacia.
//
// ADITIVO. Sin `ActivarLimiteDeImagen` (o con 0), el camino es EXACTAMENTE el de siempre: la PC no cambia una
// linea y el POC del host tampoco. Lo que lo vuelve "de consola" no es una rama por plataforma: es el numero, y
// lo pone el arranque que sabe cuanta RAM hay.

// contenedorNoEntra: el `contenedor` de un trozo que NO se entrego porque su imagen pasa el techo. Es distinto de
// -1 (una imagen fuera del tramo, un `make`) y de `contenedorAlVuelo` (-2) a proposito: asi el que llama puede
// distinguir "aca no hay pixeles" de "aca hay pixeles de otro lado" sin mirar punteros.
const contenedorNoEntra = -3

// ActivarLimiteDeImagen (DC-002.159i): ninguna imagen de la cache del mapa puede pedir mas de `bytes` B. Una
// imagen que lo pase no se compone: se anota (para no volver a pedirla ni avisar por cuadro) y el log dice cual
// es. `bytes <= 0` apaga el techo, que es el camino de la PC.
//
// QUE NUMERO PONER EN LA CONSOLA. El del buffer de trabajo del cuadro (`ConComposicionAlVuelo`): una imagen que no
// cabe ahi no la puede servir NINGUN camino (el tramo no crece mas alla de su bloque y el trabajo no crece mas
// alla de esto). En el Gate 0 son 81.920 B, y la imagen mas grande que el pueblo pide de verdad son 76.800 B
// (medido con `console/dc/medidas/sombra-159i` sobre los 277 graficos del pueblo: la mas grande es una pared de
// 160x480). O sea que el techo no le quita nada al nivel: solo corta el pedido que no era de ninguna imagen.
func (mr *MapRenderer) ActivarLimiteDeImagen(bytes int) {
	if bytes <= 0 {
		mr.limiteDeImagen = 0

		return
	}

	mr.limiteDeImagen = bytes

	if mr.noEntran == nil {
		mr.noEntran = make(map[uint32]bool)
	}
}

// ConLimiteDeImagen (DC-002.159i): la opcion del renderer. Ver `ActivarLimiteDeImagen`.
func ConLimiteDeImagen(bytes int) Opcion {
	return func(mr *MapRenderer) { mr.ActivarLimiteDeImagen(bytes) }
}

// LimiteDeImagen: el techo por imagen en bytes (0 = apagado).
func (mr *MapRenderer) LimiteDeImagen() int { return mr.limiteDeImagen }

// noEntraLaImagen: si esta clave ya se rechazo. El dibujado lo pregunta ANTES de pedir la imagen, asi el cuadro
// con la imagen rechazada no paga ni el `GetTiles` ni el aviso por cuadro.
func (mr *MapRenderer) noEntraLaImagen(clave uint32) bool {
	if mr.noEntran == nil {
		return false
	}

	return mr.noEntran[clave]
}

// imagenRechazada: la imagen `n` B de la clave `clave` no entra en la consola. Se anota y se dice UNA vez.
// (El que llama ya sabe que `n` pasa el techo; esto es el registro, no la decision.)
func (mr *MapRenderer) imagenRechazada(clave uint32, que string, n, ancho, alto int) {
	if mr.noEntran == nil {
		mr.noEntran = make(map[uint32]bool)
	}

	if mr.noEntran[clave] {
		return
	}

	mr.noEntran[clave] = true
	mr.imagenesNoEntran++
	mr.bytesNoEntran += n

	mr.Warningf("MAPA159I: la imagen %s de %dx%d (%d B) no entra en la consola y NO se pinta: pasa el techo de %d B "+
		"por imagen (DC-002.159i: se dice y se sigue; el cuadro sale sin ella)", que, ancho, alto, n,
		mr.limiteDeImagen)
}

// CensoDelLimite159i: las cuentas del techo, para el log de la consola y la evidencia:
// cuantas imagenes se rechazaron, cuantos bytes son entre todas y cuantas veces el buffer al vuelo dijo que no.
func (mr *MapRenderer) CensoDelLimite159i() (imagenes, bytes, vuelos int) {
	return mr.imagenesNoEntran, mr.bytesNoEntran, mr.vuelosRechazados
}

// sobreElTecho: si una imagen de `n` B no puede pedirse con el techo puesto.
func (mr *MapRenderer) sobreElTecho(n int) bool {
	return mr.limiteDeImagen > 0 && n > mr.limiteDeImagen
}

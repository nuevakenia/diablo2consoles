package d2dcc

// DC-002.159e -- EL TALLER: LOS BUFFERS DE TRABAJO DEL DECODIFICADOR, REUSADOS ENTRE DIRECCIONES.
//
// POR QUE EXISTE (medido). Decodificar UNA direccion de una capa del heroe pide al monton entre 26.000 y
// 97.000 B segun el tamano de la capa -- promedio 57.197 B sobre las 8 capas del heroe, 7.321.288 B en las 128
// direcciones (medido en la PC, ver DC-002.159e) -- y de esos los pixeles de los cuadros que se QUEDAN vivos
// son una fraccion. El resto son buffers de TRABAJO que el decodificador tira al terminar la direccion:
//
//	calculateCells      las celdas del lienzo de la direccion    (~3 KB)
//	recalculateCells    las celdas de cada cuadro                (~10 KB en una capa chica, mucho mas en una grande)
//	fillPixelBuffer     la tabla de celdas + su buffer de celda  (~4 KB)
//	generateFrames      el lienzo de trabajo de la direccion     (~0,5 KB)
//
// En la PC eso lo paga el recolector de Go y no se nota. En la consola cada uno de esos bytes se queda en el
// semiespacio de 1.835.008 B hasta la proxima recoleccion, y caminando el heroe decodifica una direccion por
// capa y por giro (medido en DC-002.159d: `decod=112` en 390 cuadros, con picos de 240.000-475.000 B en un
// solo tramo de 15 cuadros). No hay fuga que buscar: hay que pedir menos (la sonda de control de 159d lo
// probo). El taller hace eso sin tocar una linea de la DECODIFICACION: los mismos buffers, pedidos una vez
// por tamano y vueltos a usar en la direccion siguiente.
//
// QUE NO TOCA. Los pixeles de cada cuadro (`frame.PixelData`) NO pasan por el taller: esos son el resultado
// de la decodificacion y siguen vivos mientras la direccion este decodificada (el compuesto los vuelve a
// expandir cada vez que cambia el cuadro que se dibuja). El taller es solo para lo que el decodificador tira.
//
// DONDE SE PRENDE. En la consola, desde `d2asset.ArrancarComoConsola` (como `direccionesPerezosas` y
// `superficiesPorCuadro`). En la PC queda apagado y el decodificador pide con `make`, igual que siempre.
//
// OJO CON EL CONTRATO. Los buffers que devuelve este archivo valen hasta la decodificacion siguiente: el
// decodificador los usa DENTRO de `CreateDCCDirection` y los deja en nil al terminar (`generateFrames`
// limpia `Cells`, `PixelBuffer` y `PixelData`). Nadie mas los puede guardar. Y como el taller es uno solo,
// dos decodificaciones en paralelo lo pisarian: en la consola el decodificador corre en el hilo del cuadro,
// una direccion a la vez (el motor lo llama desde `SetDirection`, que es sincrono).

// reusarElTaller159: la politica (apagada en la PC).
var reusarElTaller159 bool

// UsarElTallerDeDecodificacion159 prende o apaga el taller de buffers de trabajo.
func UsarElTallerDeDecodificacion159(si bool) { reusarElTaller159 = si }

// TallerDeDecodificacion159 dice si el taller esta prendido (para la linea de estado de la corrida).
func TallerDeDecodificacion159() bool { return reusarElTaller159 }

// taller159: los buffers. Cada uno crece al tamano que haga falta y no se vuelve a pedir.
//
// LAS CELDAS SON LO UNICO QUE SE PIDE MAS DE UNA VEZ POR DIRECCION: cada cuadro del `.dcc` tiene su propio
// juego de celdas y todos tienen que estar vivos a la vez mientras la direccion se decodifica (la tabla de
// celdas y el armado de pixeles los recorren juntos). Por eso las celdas se piden a un cursor (`celdasEnUso`)
// que vuelve a cero al empezar cada direccion: un solo arreglo, repartido en tramos.
var taller159 struct {
	celdas      []DCCCell
	celdasEnUso int

	punteros  []*DCCCell
	entradas  []DCCPixelBufferEntry
	punterosE []*DCCPixelBufferEntry
	anchos    []int
	altos     []int
	lienzo    []byte
}

// abrirElTaller159: empieza una direccion. Devuelve el cursor de las celdas al principio (lo que la
// direccion anterior pidio se puede pisar: sus celdas quedaron en nil al terminar `generateFrames`).
func abrirElTaller159() {
	taller159.celdasEnUso = 0
}

// celdasDelTaller: `n` celdas limpias para el decodificador, en el tramo que toca del arreglo.
func celdasDelTaller(n int) []DCCCell {
	if n <= 0 {
		return nil
	}

	desde := taller159.celdasEnUso
	total := desde + n

	if cap(taller159.celdas) < total {
		// crece de mas (una vez y media): un `.dcc` tiene cuadros de cajas distintas y crecer de a una celda
		// seria pedir un arreglo nuevo en cada decodificacion que sube un poco el total.
		nuevo := make([]DCCCell, total+total/2+16)
		copy(nuevo, taller159.celdas[:desde])
		taller159.celdas = nuevo
	}

	celdas := taller159.celdas[desde:total]
	taller159.celdasEnUso = total

	// los `LastWidth`/`LastHeight` de las celdas del lienzo arrancan en -1 y los demas campos se escriben
	// siempre, pero limpiar deja el buffer como recien pedido y cuesta una celda por celda (32 B en la consola).
	for i := range celdas {
		celdas[i] = DCCCell{}
	}

	return celdas
}

// punterosACeldasDelTaller: el `cellBuffer` de `calculateCells` (punteros a las celdas del lienzo).
func punterosACeldasDelTaller(n int) []*DCCCell {
	if n <= 0 {
		return nil
	}

	if cap(taller159.punteros) < n {
		taller159.punteros = make([]*DCCCell, n)
	}

	punteros := taller159.punteros[:n]

	for i := range punteros {
		punteros[i] = nil
	}

	return punteros
}

// entradasDelTaller: la tabla de celdas del decodificador (`fillPixelBuffer`). El que la usa la reinicia
// entera (`Frame = -1`), asi que no hace falta limpiarla.
func entradasDelTaller(n int) []DCCPixelBufferEntry {
	if n <= 0 {
		return nil
	}

	if cap(taller159.entradas) < n {
		taller159.entradas = make([]DCCPixelBufferEntry, n)
	}

	return taller159.entradas[:n]
}

// punterosAEntradasDelTaller: el `cellBuffer` de `fillPixelBuffer`.
func punterosAEntradasDelTaller(n int) []*DCCPixelBufferEntry {
	if n <= 0 {
		return nil
	}

	if cap(taller159.punterosE) < n {
		taller159.punterosE = make([]*DCCPixelBufferEntry, n)
	}

	punteros := taller159.punterosE[:n]

	for i := range punteros {
		punteros[i] = nil
	}

	return punteros
}

// anchosDelTaller / altosDelTaller: los `cellWidths` / `cellHeights` (se escriben enteros antes de leerlos).
func anchosDelTaller(n int) []int {
	if n <= 0 {
		return nil
	}

	if cap(taller159.anchos) < n {
		taller159.anchos = make([]int, n)
	}

	return taller159.anchos[:n]
}

func altosDelTaller(n int) []int {
	if n <= 0 {
		return nil
	}

	if cap(taller159.altos) < n {
		taller159.altos = make([]int, n)
	}

	return taller159.altos[:n]
}

// lienzoDelTaller: el lienzo de trabajo de la direccion (`PixelData` de `DCCDirection`). Se limpia porque el
// decodificador copia celdas de una posicion a otra dentro de el, y una celda que este vez no se escribio no
// puede mostrar los pixeles de la direccion anterior (que tenia otra caja).
func lienzoDelTaller(n int) []byte {
	if n <= 0 {
		return nil
	}

	if cap(taller159.lienzo) < n {
		taller159.lienzo = make([]byte, n)
	}

	lienzo := taller159.lienzo[:n]

	for i := range lienzo {
		lienzo[i] = 0
	}

	return lienzo
}

// ============================== los pedidos del decodificador ==============================
//
// El decodificador pide por estas tres puertas y no por `make`: con el taller prendido, el buffer sale de
// arriba (reusado); apagado, sale del camino de siempre (la arena del que llama o el monton de Go).

func pedirCeldasDeTrabajo(n int) []DCCCell {
	if reusarElTaller159 {
		return celdasDelTaller(n)
	}

	return pedirCeldas(n)
}

func pedirEntradasDeTrabajo(n int) []DCCPixelBufferEntry {
	if reusarElTaller159 {
		return entradasDelTaller(n)
	}

	return pedirEntradas(n)
}

func pedirLienzoDeTrabajo(n int) []byte {
	if reusarElTaller159 {
		return lienzoDelTaller(n)
	}

	return pedirBytes(n)
}

func pedirPunterosACeldas(n int) []*DCCCell {
	if reusarElTaller159 {
		return punterosACeldasDelTaller(n)
	}

	return make([]*DCCCell, n)
}

func pedirPunterosAEntradas(n int) []*DCCPixelBufferEntry {
	if reusarElTaller159 {
		return punterosAEntradasDelTaller(n)
	}

	return make([]*DCCPixelBufferEntry, n)
}

func pedirAnchos(n int) []int {
	if reusarElTaller159 {
		return anchosDelTaller(n)
	}

	return make([]int, n)
}

func pedirAltos(n int) []int {
	if reusarElTaller159 {
		return altosDelTaller(n)
	}

	return make([]int, n)
}

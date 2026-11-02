package d2maprenderer

// DC-002.113b -- LA PRUEBA EN CONTRA DEL DESALOJO.
//
// Lo que tiene que quedar clavado aca, porque es lo que la tarjeta viene a arreglar:
//
//	1. con la cache llena, el desalojo deja entrar imagenes nuevas SIN derramar una sola al monton de Go (que
//	   es lo que tumbaba al recolector: `fatal error: GC to-space overflow`), y los bytes que suelta se REUSAN;
//	2. una imagen NUNCA ve los bytes de otra: se escribe un patron distinto en cada una y se verifica al final,
//	   despues de cientos de desalojos;
//	3. lo que el cuadro esta pintando no se suelta (una imagen usada en el cuadro en curso no es candidata), y
//	   una que se suelta y se vuelve a pedir da LOS MISMOS pixeles -- que es lo que hace que el crc32 del cuadro
//	   no cambie por desalojar;
//	4. la otra politica (componer AL VUELO) tampoco derrama, y la imagen no se guarda en la cache.

import "testing"

// relojDeLaCache: adelanta un cuadro, que en el renderer lo hace `Render` (aca no hay mapa que dibujar).
func relojDeLaCache(mr *MapRenderer) { mr.cuadroDeLaCache++ }

// armarImagen: el mismo camino que hacen generateFloorCache/generateWallCache con el trozo que les dan.
func armarImagen(mr *MapRenderer, style byte, n int, patron byte) *imagenDeIndices {
	trozo := mr.pedirTrozoDeIndices(n, true)

	for i := range trozo.indices {
		trozo.indices[i] = patron
	}

	im := &imagenDeIndices{
		indices: trozo.indices, ancho: n, alto: 1, contenedor: trozo.contenedor, desde: trozo.desde,
	}
	mr.setCacheDeIndices(style, 0, 0, 0, im)

	return im
}

// TestElDesalojoNoDerramaNiUna: la cache se llena y sigue entrando gente nueva sin un solo derrame.
func TestElDesalojoNoDerramaNiUna(t *testing.T) {
	mr := &MapRenderer{}
	mr.ActivarCacheDeIndices()
	mr.ActivarTramoDeLaCacheEnPedazos(4*100000, 100000) // 400 KB: entran 20 imagenes de 20 KB
	mr.ActivarDesalojoDeLaCache(2)

	const (
		tamano  = 20000
		cuantos = 400 // 20 veces la capacidad del tramo
	)

	for i := 0; i < cuantos; i++ {
		relojDeLaCache(mr)
		armarImagen(mr, byte(i), tamano, byte(i))
		mr.getCacheDeIndices(byte(i), 0, 0, 0) // esta imagen la pinta el cuadro
		relojDeLaCache(mr)                     // y que ruede el cuadro: la de antes ya esta fria
	}

	e := mr.CensoDelTramo()

	if e.Derrames != 0 || e.BytesDerramados != 0 {
		t.Fatalf("con desalojo no se derrama ni una imagen: derrames=%d bytes=%d", e.Derrames, e.BytesDerramados)
	}

	if e.Desalojos == 0 {
		t.Fatalf("el desalojo no solto nada en %d imagenes: %+v", cuantos, e)
	}

	if e.Usado > e.Bytes {
		t.Fatalf("el censo no cuadra: usado=%d bytes=%d", e.Usado, e.Bytes)
	}

	// en el tramo no puede haber mas de las que entran, y el mapa no puede haber crecido con las desalojadas
	if caben := e.Bytes / tamano; len(mr.indicesCacheRecords) > caben {
		t.Fatalf("la cache tiene %d imagenes y en %d B solo entran %d", len(mr.indicesCacheRecords), e.Bytes, caben)
	}
}

// TestElDesalojoNoMezclaDosImagenes: cientos de desalojos y cada imagen sigue teniendo SU patron. Es la prueba de
// que devolver un pedazo del bloque no le da a nadie bytes que otro este usando.
func TestElDesalojoNoMezclaDosImagenes(t *testing.T) {
	mr := &MapRenderer{}
	mr.ActivarCacheDeIndices()
	mr.ActivarTramoDeLaCacheEnPedazos(3*90000, 90000)
	mr.ActivarDesalojoDeLaCache(1)

	const (
		chica  = 20480
		grande = 71680
	)

	tamanos := []int{chica, grande, chica, chica, grande, chica}

	for i := 0; i < 300; i++ {
		relojDeLaCache(mr)

		n := tamanos[i%len(tamanos)]
		im := armarImagen(mr, byte(i%200), n, byte(i%200+1))
		mr.getCacheDeIndices(byte(i%200), 0, 0, 0)

		// lo que ya estaba en la cache tiene que seguir intacto (nadie piso sus bytes)
		for k, otra := range mr.indicesCacheRecords {
			esperado := byte(k>>24) + 1 // el patron lo puso armarImagen con estilo = k>>24
			for j, b := range otra.indices {
				if b != esperado {
					t.Fatalf("imagen %d byte %d = %d, esperaba %d: el desalojo mezclo dos imagenes", k>>24, j, b, esperado)
				}
			}
		}

		_ = im
	}

	if e := mr.CensoDelTramo(); e.Derrames != 0 {
		t.Fatalf("derrames con desalojo: %+v", e)
	}
}

// TestElDesalojoRespetaElCuadroEnCurso: la imagen MAS VIEJA de la cache no se suelta si el cuadro la acaba de
// pedir; y si se suelta a proposito y se vuelve a pedir, da los MISMOS pixeles (el crc32 del cuadro no cambia por
// desalojar).
func TestElDesalojoRespetaElCuadroEnCurso(t *testing.T) {
	mr := &MapRenderer{}
	mr.ActivarCacheDeIndices()
	mr.ActivarTramoDeLaCacheEnPedazos(12*20000, 12*20000) // un bloque, entran 12 imagenes de 20 KB
	mr.ActivarDesalojoDeLaCache(1)

	// la MAS VIEJA de todas: se arma y se pinta en el cuadro 1
	relojDeLaCache(mr)

	viva := armarImagen(mr, 7, 20000, 0xAB)
	mr.getCacheDeIndices(7, 0, 0, 0)

	// se llena el tramo con diez imagenes que se van usando cuadro a cuadro (la 7 se queda sin tocar)
	for i := 0; i < 10; i++ {
		relojDeLaCache(mr)
		armarImagen(mr, byte(100+i), 20000, byte(i+1))
		mr.getCacheDeIndices(byte(100+i), 0, 0, 0)
	}

	// EL CUADRO EN CURSO la vuelve a pedir: a partir de aca NO puede ser candidata, aunque sea la mas vieja
	if mr.getCacheDeIndices(7, 0, 0, 0) == nil {
		t.Fatalf("la imagen 7 se fue de la cache antes de que el cuadro la pidiera")
	}

	// y una imagen nueva la fuerza a hacer lugar: la victima tiene que ser otra (la 100, la mas vieja de las frias)
	armarImagen(mr, 200, 20000, 0xCD)
	mr.getCacheDeIndices(200, 0, 0, 0)

	otra := mr.getCacheDeIndices(7, 0, 0, 0)
	if otra == nil {
		t.Fatalf("se solto una imagen que el cuadro estaba pintando")
	}

	for j, b := range otra.indices {
		if b != 0xAB {
			t.Fatalf("la imagen viva se piso (byte %d = %d)", j, b)
		}
	}

	if e := mr.CensoDelTramo(); e.Derrames != 0 {
		t.Fatalf("derrames con desalojo: %+v", e)
	}

	// y si se suelta a proposito, volver a pedirla la recompone igual (mismos pixeles, otra vez en la cache)
	viva.indices[0] = 0 // se ensucia lo que ya no es de nadie
	mr.soltarImagenDeLaCache(viva.clave, viva)

	if mr.getCacheDeIndices(7, 0, 0, 0) != nil {
		t.Fatalf("la imagen soltada sigue en la cache")
	}
}

// TestLaComposicionAlVueloNoDerramaNiCachea: la otra salida cuando el tramo esta lleno. Se compone, se pinta y no
// se guarda: lo que se cuenta es `AlVuelo`, no `Derrames`.
func TestLaComposicionAlVueloNoDerramaNiCachea(t *testing.T) {
	mr := &MapRenderer{}
	mr.ActivarCacheDeIndices()
	mr.ActivarTramoDeLaCacheEnPedazos(2*100000, 100000)
	mr.ActivarComposicionAlVuelo(80000)

	relojDeLaCache(mr)

	// 12 imagenes de 20 KB en un tramo de 200 KB: las 2 primeras entran, el resto se compone al vuelo
	for i := 0; i < 12; i++ {
		im := armarImagen(mr, byte(i), 20000, byte(i+1))

		// la que se compuso al vuelo la encuentra el dibujado en ESTE cuadro, y con SUS bytes
		encontrada := mr.getCacheDeIndices(byte(i), 0, 0, 0)
		if encontrada == nil {
			t.Fatalf("imagen %d: el dibujado no la encuentra", i)
		}

		if encontrada.indices[0] != byte(i+1) {
			t.Fatalf("imagen %d: el cuadro leyo %d", i, encontrada.indices[0])
		}

		if im.contenedor == contenedorAlVuelo && mr.indicesCacheRecords[im.clave] != nil {
			t.Fatalf("imagen %d: la compuesta al vuelo se guardo en la cache", i)
		}

		relojDeLaCache(mr)
	}

	e := mr.CensoDelTramo()

	if e.Derrames != 0 {
		t.Fatalf("al vuelo no derrama: %+v", e)
	}

	if e.AlVuelo == 0 {
		t.Fatalf("no se compuso nada al vuelo: %+v", e)
	}

	// y al cuadro siguiente la compuesta al vuelo ya no esta: su buffer se reusa
	if mr.getCacheDeIndices(11, 0, 0, 0) != nil {
		t.Fatalf("la compuesta al vuelo del cuadro anterior sigue apareciendo")
	}
}

// TestSinPalancaLaCacheSoloAgrega: el camino de DC-002.113 tiene que quedar EXACTAMENTE igual sin la palanca --
// mismo numero de imagenes, cero desalojos y el derrame contado cuando el tramo se llena.
func TestSinPalancaLaCacheSoloAgrega(t *testing.T) {
	mr := &MapRenderer{}
	mr.ActivarCacheDeIndices()
	mr.ActivarTramoDeLaCacheEnPedazos(2*100000, 100000) // sin ActivarDesalojoDeLaCache

	for i := 0; i < 20; i++ {
		relojDeLaCache(mr)
		armarImagen(mr, byte(i), 20000, byte(i))
		mr.getCacheDeIndices(byte(i), 0, 0, 0)
	}

	e := mr.CensoDelTramo()

	if e.Desalojos != 0 {
		t.Fatalf("sin la palanca no se desaloja nada: %+v", e)
	}

	if len(mr.indicesCacheRecords) != 20 {
		t.Fatalf("sin la palanca la cache guarda las 20 imagenes: %d", len(mr.indicesCacheRecords))
	}

	if e.Derrames == 0 {
		t.Fatalf("sin la palanca, lo que no entra se derrama y se cuenta: %+v", e)
	}
}

// TestElDesalojoAgrandaElHueco: la SEGUNDA PASADA del desalojo. Con todo lo frio ya suelto (dos imagenes CHICAS,
// que no hacen un hueco del tamano de la grande) y la cola del bloque consumida, la grande solo entra si se suelta
// al VECINO de un hueco para agrandarlo. Ese vecino esta caliente pero no lo esta pintando el cuadro en curso.
func TestElDesalojoAgrandaElHueco(t *testing.T) {
	mr := &MapRenderer{}
	mr.ActivarCacheDeIndices()
	mr.ActivarTramoDeLaCacheEnPedazos(300000, 300000) // un solo bloque
	mr.ActivarDesalojoDeLaCache(2)

	const grande, chica = 76800, 20480

	// el bloque queda: A(chica) B(grande) C(chica) D(grande) = 194.560 B y 105.440 B de cola
	relojDeLaCache(mr)

	armarImagen(mr, 1, chica, 2)
	armarImagen(mr, 2, grande, 3)
	armarImagen(mr, 3, chica, 4)
	armarImagen(mr, 4, grande, 5)

	for _, k := range []byte{1, 2, 3, 4} {
		mr.getCacheDeIndices(k, 0, 0, 0)
	}

	// cuadros 2 y 3: el cuadro pinta las DOS GRANDES, las chicas se quedan sin usar (quedan frias)
	for i := 0; i < 2; i++ {
		relojDeLaCache(mr)
		mr.getCacheDeIndices(2, 0, 0, 0)
		mr.getCacheDeIndices(4, 0, 0, 0)
	}

	// cuadro 4: una grande entra en la cola del bloque...
	relojDeLaCache(mr)
	armarImagen(mr, 5, grande, 6)
	mr.getCacheDeIndices(5, 0, 0, 0)

	// ...y la segunda solo entra AGRANDANDO el hueco de una chica con su vecina grande
	armarImagen(mr, 6, grande, 7)
	mr.getCacheDeIndices(6, 0, 0, 0)

	e := mr.CensoDelTramo()

	if e.Derrames != 0 {
		t.Fatalf("la segunda pasada tenia que hacer lugar sin derramar: %+v", e)
	}

	if e.Agrandados == 0 {
		t.Fatalf("este caso solo se resuelve agrandando un hueco con su vecino: %+v", e)
	}

	// y lo que sigue en la cache sigue con SUS bytes: nadie piso nada
	for k, im := range mr.indicesCacheRecords {
		esperado := byte(k>>24) + 1
		for j, v := range im.indices {
			if v != esperado {
				t.Fatalf("imagen %d byte %d = %d, esperaba %d: dos imagenes comparten bytes", k>>24, j, v, esperado)
			}
		}
	}
}

// TestElDesalojoTerceraPasadaSinHuecos: cuando no queda NINGUNA fria y no hay ni un hueco (el tramo esta usado
// hasta la cola del bloque), la ultima vuelta suelta a la mas vieja que no se haya pedido en ESTE cuadro.
func TestElDesalojoTerceraPasadaSinHuecos(t *testing.T) {
	mr := &MapRenderer{}
	mr.ActivarCacheDeIndices()
	mr.ActivarTramoDeLaCacheEnPedazos(200000, 200000)
	mr.ActivarDesalojoDeLaCache(2)

	const grande, chica = 76800, 20480

	relojDeLaCache(mr)

	// A(chica) [0,20480) B(grande) [20480,97280) C(grande) [97280,174080), la cola son 25.920 B
	armarImagen(mr, 1, chica, 2)
	armarImagen(mr, 2, grande, 3)
	armarImagen(mr, 3, grande, 4)

	for _, k := range []byte{1, 2, 3} {
		mr.getCacheDeIndices(k, 0, 0, 0)
	}

	// cuadros 2 y 3: las tres se usan, no hay nada frio
	for i := 0; i < 2; i++ {
		relojDeLaCache(mr)
		for _, k := range []byte{1, 2, 3} {
			mr.getCacheDeIndices(k, 0, 0, 0)
		}
	}

	// cuadro 4: la grande no entra en la cola, no hay huecos y las tres son del cuadro anterior (no de este)
	relojDeLaCache(mr)
	armarImagen(mr, 4, grande, 5)

	e := mr.CensoDelTramo()

	if e.Derrames != 0 {
		t.Fatalf("tenia que entrar soltando a la mas vieja que no es de este cuadro: %+v", e)
	}
}

// TestElDesalojoAceptaUnVecinoDelCuadroEnCurso: cuando no hay ninguna fria, hay un hueco chico y su unico vecino lo
// pidio ESTE cuadro, se suelta igual (la pintura copia los pixeles en el acto, no hay nada en vuelo) antes de
// derramar: es la pasada que agranda el hueco y la que evita el derrame unico de la corrida real.
func TestElDesalojoAceptaUnVecinoDelCuadroEnCurso(t *testing.T) {
	mr := &MapRenderer{}
	mr.ActivarCacheDeIndices()
	mr.ActivarTramoDeLaCacheEnPedazos(200000, 200000)
	mr.ActivarDesalojoDeLaCache(2)

	const grande, chica = 76800, 20480

	relojDeLaCache(mr)

	// A(chica) [0,20480) B(grande) [20480,97280) C(grande) [97280,174080), la cola son 25.920 B
	armarImagen(mr, 1, chica, 2)
	armarImagen(mr, 2, grande, 3)
	armarImagen(mr, 3, grande, 4)

	for _, k := range []byte{1, 2, 3} {
		mr.getCacheDeIndices(k, 0, 0, 0)
	}

	// cuadros 2 y 3: solo las GRANDES; la chica queda fria y suelta un hueco de 20.480 B
	for i := 0; i < 2; i++ {
		relojDeLaCache(mr)
		mr.getCacheDeIndices(2, 0, 0, 0)
		mr.getCacheDeIndices(3, 0, 0, 0)
	}

	// cuadro 4: la grande no entra en el hueco de la chica ni en la cola; su unico vecino (B) es de ESTE cuadro
	relojDeLaCache(mr)
	armarImagen(mr, 4, grande, 5)
	mr.getCacheDeIndices(2, 0, 0, 0) // B se pinta en este cuadro

	armarImagen(mr, 5, grande, 6)

	e := mr.CensoDelTramo()

	if e.Derrames != 0 {
		t.Fatalf("tenia que entrar agrandando el hueco con su vecino del cuadro en curso: %+v", e)
	}

	if e.Agrandados == 0 {
		t.Fatalf("la que hizo lugar fue la pasada que agranda el hueco: %+v", e)
	}
}

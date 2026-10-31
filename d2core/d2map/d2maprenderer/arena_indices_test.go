package d2maprenderer

// DC-002.104 -- la prueba en contra del tramo de la cache: que reparta de verdad, que no mezcle una imagen con
// otra, que cuente el derrame y que se apague sola cuando no la piden (o cuando el tramo no pasaria de los
// 64 KB del objeto grande de libgodc, que es todo el punto).

import (
	"bytes"
	"testing"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2dt1"
)

// TestElTramoReparteLosPixeles: cinco imagenes de tamano distinto salen del MISMO tramo, no se pisan y el
// censo del tramo cuadra al byte.
func TestElTramoReparteLosPixeles(t *testing.T) {
	mr := &MapRenderer{}
	mr.ActivarTramoDeLaCache(200 * 1024) // > 64 KB: en la consola, monton de C

	tamanos := []int{1010, 4800, 71680, 12, 51200}
	pedazos := make([][]byte, len(tamanos))

	suma := 0

	for i, n := range tamanos {
		pedazos[i] = mr.pedirIndicesDelTile(n)
		suma += n

		if len(pedazos[i]) != n {
			t.Fatalf("imagen %d: len=%d, queria %d", i, len(pedazos[i]), n)
		}

		if cap(pedazos[i]) != n {
			t.Fatalf("imagen %d: cap=%d, queria %d (un pedazo del tramo no se puede crecer)", i, cap(pedazos[i]), n)
		}

		for j := range pedazos[i] {
			pedazos[i][j] = byte(i*17 + j)
		}
	}

	// las cinco siguen intactas despues de escribir las cinco: no se pisan
	for i, n := range tamanos {
		for j := 0; j < n; j++ {
			if pedazos[i][j] != byte(i*17+j) {
				t.Fatalf("imagen %d byte %d = %d: otra imagen la piso", i, j, pedazos[i][j])
			}
		}
	}

	e := mr.CensoDelTramo()

	if !e.Activa || e.Bytes != 200*1024 || e.Usado != suma || e.Libre != 200*1024-suma {
		t.Fatalf("censo del tramo: %+v (usado debia ser %d)", e, suma)
	}

	if e.Imagenes != len(tamanos) || e.Derrames != 0 || e.BytesDerramados != 0 {
		t.Fatalf("censo del tramo: %+v (5 imagenes y cero derrames)", e)
	}
}

// TestElTramoDerrama: lo que no entra en el tramo se pide como siempre (make) y se CUENTA, para que el numero
// que falta salga del log y no de una suposicion.
func TestElTramoDerrama(t *testing.T) {
	mr := &MapRenderer{}
	mr.ActivarTramoDeLaCache(64*1024 + 1) // lo mas chico que se activa

	entra := mr.pedirIndicesDelTile(1000)

	noEntra := mr.pedirIndicesDelTile(64 * 1024)

	if len(noEntra) != 64*1024 || cap(noEntra) != 64*1024 {
		t.Fatalf("el derrame tiene que ser un buffer del tamano pedido: len=%d cap=%d", len(noEntra), cap(noEntra))
	}

	/// el derrame NO puede ser un pedazo del tramo: si lo fuera, escribir ahi pisaria lo ya repartido
	for j := range noEntra {
		noEntra[j] = 0xFF
	}

	if entra[0] != 0 {
		t.Fatalf("el derrame piso el tramo (byte 0 = %d)", entra[0])
	}

	e := mr.CensoDelTramo()

	if e.Derrames != 1 || e.BytesDerramados != 64*1024 || e.Usado != 1000 || e.Imagenes != 1 {
		t.Fatalf("censo del tramo: %+v", e)
	}
}

// TestElTramoSeApagaSolo: sin la palanca no hay tramo (el camino de la PC), y con un tramo que no pasaria el
// corte de 64 KB tampoco -- ese pedazo iria al semiespacio y no compra nada.
func TestElTramoSeApagaSolo(t *testing.T) {
	sinPalanca := &MapRenderer{}

	sinPalanca.pedirIndicesDelTile(1234)

	if e := sinPalanca.CensoDelTramo(); e.Activa || e.Bytes != 0 || e.Usado != 0 {
		t.Fatalf("sin la palanca el tramo tiene que estar apagado: %+v", e)
	}

	for _, bytes := range []int{0, -1, 1, 64 * 1024} {
		mr := &MapRenderer{}
		mr.ActivarTramoDeLaCache(bytes)

		if e := mr.CensoDelTramo(); e.Activa {
			t.Fatalf("un tramo de %d B no se tiene que activar: %+v", bytes, e)
		}

		if p := mr.pedirIndicesDelTile(64 * 1024); cap(p) != 64*1024 || len(p) != 64*1024 {
			t.Fatalf("con el tramo apagado la imagen tiene que ser un make: len=%d cap=%d", len(p), cap(p))
		}
	}
}

// TestElTramoDecodificaIgualQueLaMake: el decodificador del .dt1 escribiendo sobre un pedazo del tramo deja los
// MISMOS bytes que escribiendo sobre un `make`. Es la prueba de que el cuadro no puede cambiar de camino.
func TestElTramoDecodificaIgualQueLaMake(t *testing.T) {
	bloque := d2dt1.Block{
		X: 0, Y: 0,
		Length:      5,
		EncodedData: []byte{0, 3, 7, 8, 9}, // RLE: salta 0, 3 pixeles con los valores 7, 8, 9
	}

	const ancho, alto = 10, 3

	mr := &MapRenderer{}
	mr.ActivarTramoDeLaCache(90 * 1024)

	conTramo := mr.pedirIndicesDelTile(ancho * alto)
	d2dt1.DecodeTileGfxData([]d2dt1.Block{bloque}, &conTramo, 0, ancho)

	conMake := make([]byte, ancho*alto)
	d2dt1.DecodeTileGfxData([]d2dt1.Block{bloque}, &conMake, 0, ancho)

	if !bytes.Equal(conTramo, conMake) {
		t.Fatalf("el tramo decodifico distinto:\n tramo=%v\n make =%v", conTramo, conMake)
	}

	if conMake[0] != 7 || conMake[2] != 9 {
		t.Fatalf("el bloque de prueba no decodifico lo esperado: %v", conMake[:4])
	}
}

// TestElNivelNuevoReusaElTramo: al cambiar de nivel la cache se descarta y el tramo vuelve al byte 0 (no se
// pide otro: en la consola el monton externo no se devuelve).
func TestElNivelNuevoReusaElTramo(t *testing.T) {
	mr := &MapRenderer{}
	mr.ActivarTramoDeLaCache(70 * 1024)

	mr.pedirIndicesDelTile(1000)
	mr.reiniciarTramo()

	e := mr.CensoDelTramo()

	if !e.Activa || e.Bytes != 70*1024 || e.Usado != 0 || e.Imagenes != 0 {
		t.Fatalf("el tramo tiene que seguir vivo y vacio: %+v", e)
	}
}

// ============================== DC-002.106: EL TRAMO EN PEDAZOS ==============================

// TestElTramoEnPedazosReparteLosPixeles: las imagenes salen de VARIOS bloques, ninguna se pisa, y el censo
// cuadra al byte (bloques pedidos, techo, usado). Es la prueba de que el modo en pedazos reparte igual que el
// de un solo golpe: la unica diferencia es DONDE vive cada bloque.
func TestElTramoEnPedazosReparteLosPixeles(t *testing.T) {
	mr := &MapRenderer{}
	// techo 300 KB en bloques de 100 KB (los dos pasan el corte de 64 KB del objeto grande)
	mr.ActivarTramoDeLaCacheEnPedazos(300*1024, 100*1024)

	if e := mr.CensoDelTramo(); !e.Activa || !e.EnPedazos || e.Pedazos != 0 || e.Bytes != 0 {
		t.Fatalf("el tramo en pedazos se reserva SIN pedir nada (se pide al dibujar): %+v", e)
	}

	tamanos := []int{1010, 4800, 70 * 1024, 12, 50 * 1024, 48 * 1024, 30 * 1024}

	pedazos := make([][]byte, len(tamanos))
	suma := 0

	for i, n := range tamanos {
		pedazos[i] = mr.pedirIndicesDelTile(n)
		suma += n

		if len(pedazos[i]) != n || cap(pedazos[i]) != n {
			t.Fatalf("imagen %d: len=%d cap=%d, queria %d/%d (un pedazo del tramo no se puede crecer)",
				i, len(pedazos[i]), cap(pedazos[i]), n, n)
		}

		for j := range pedazos[i] {
			pedazos[i][j] = byte(i*17 + j)
		}
	}

	for i, n := range tamanos {
		for j := 0; j < n; j++ {
			if pedazos[i][j] != byte(i*17+j) {
				t.Fatalf("imagen %d byte %d = %d: otra imagen la piso", i, j, pedazos[i][j])
			}
		}
	}

	e := mr.CensoDelTramo()

	if e.Pedazos < 2 {
		t.Fatalf("300 KB de techo en bloques de 100 KB: el censo tiene que ver varios bloques: %+v", e)
	}

	if e.Usado != suma || e.Libre != e.Bytes-suma || e.Techo != 300*1024 {
		t.Fatalf("censo del tramo: %+v (usado debia ser %d)", e, suma)
	}

	if e.Imagenes != len(tamanos) || e.Derrames != 0 {
		t.Fatalf("censo del tramo: %+v (%d imagenes y cero derrames)", e, len(tamanos))
	}
}

// TestElTramoEnPedazosUsaLaColaDelBloque: una imagen que no entra en el bloque en curso pero si en la cola de
// otro bloque que ya esta abierto tiene que irse a la cola (best fit). Es el numero que decide si el tramo
// entra en el monton de la consola: la cola que se abandona se pierde.
func TestElTramoEnPedazosUsaLaColaDelBloque(t *testing.T) {
	mr := &MapRenderer{}
	mr.ActivarTramoDeLaCacheEnPedazos(4*100*1024, 100*1024)

	// bloque 1: 60 KB usados, 40 KB de cola
	mr.pedirIndicesDelTile(60 * 1024)
	// 70 KB no entra en la cola del bloque 1: abre el bloque 2
	mr.pedirIndicesDelTile(70 * 1024)

	bloquesAntes := mr.CensoDelTramo().Pedazos

	// 30 KB SI entran en la cola del bloque 1: no tiene que abrir un tercer bloque
	mr.pedirIndicesDelTile(30 * 1024)

	e := mr.CensoDelTramo()
	if e.Pedazos != bloquesAntes {
		t.Fatalf("la imagen chica abrio un bloque nuevo en vez de usar la cola: %+v", e)
	}

	if e.Pedazos != 2 || e.Usado != 160*1024 {
		t.Fatalf("censo del tramo: %+v (2 bloques y 160 KB usados)", e)
	}
}

// TestElTramoEnPedazosRespetaElTecho: con techo, lo que no entra en el tramo se pide con `make` y se CUENTA.
// Es la guarda que evita el pedido que mata al monton en la consola (un `make` que el monton no puede servir
// es fatal en libgodc), y el ultimo bloque se RECORTA a lo que queda del techo en vez de pasarse.
func TestElTramoEnPedazosRespetaElTecho(t *testing.T) {
	mr := &MapRenderer{}
	// techo de 180 KB: el primer bloque pide los 100 KB enteros y el segundo se recorta a los 80 KB que quedan
	mr.ActivarTramoDeLaCacheEnPedazos(180*1024, 100*1024)

	uno := mr.pedirIndicesDelTile(60 * 1024)
	dos := mr.pedirIndicesDelTile(60 * 1024)
	tres := mr.pedirIndicesDelTile(60 * 1024)

	e := mr.CensoDelTramo()

	if e.Bytes > 180*1024 {
		t.Fatalf("el tramo se paso del techo: %+v", e)
	}

	if e.Pedazos != 2 || e.Bytes != 180*1024 {
		t.Fatalf("censo del tramo: %+v (2 bloques: 100 KB + 80 KB recortado al techo)", e)
	}

	if e.Usado != 120*1024 {
		t.Fatalf("censo del tramo: %+v (dos imagenes de 60 KB adentro)", e)
	}

	if e.Derrames != 1 || e.BytesDerramados != 60*1024 {
		t.Fatalf("la imagen que no entro tenia que derramar y contarse: %+v", e)
	}

	// la que derramo es un buffer propio del tamano pedido, y no pisa el tramo
	if len(tres) != 60*1024 || cap(tres) != 60*1024 {
		t.Fatalf("el derrame tiene que ser un buffer del tamano pedido: len=%d cap=%d", len(tres), cap(tres))
	}

	for j := range tres {
		tres[j] = 0xFF
	}

	if uno[0] != 0 || dos[0] != 0 {
		t.Fatalf("el derrame piso el tramo")
	}
}

// TestElTramoEnPedazosSeApagaSolo: ni el techo ni el bloque pueden irse al semiespacio del recolector (seria
// una asignacion chica mas, no un tramo), y el tramo de un solo golpe sigue funcionando como antes.
func TestElTramoEnPedazosSeApagaSolo(t *testing.T) {
	for _, caso := range [][2]int{{0, 100 * 1024}, {300 * 1024, 0}, {64 * 1024, 100 * 1024}, {300 * 1024, 64 * 1024}} {
		mr := &MapRenderer{}
		mr.ActivarTramoDeLaCacheEnPedazos(caso[0], caso[1])

		if e := mr.CensoDelTramo(); e.Activa {
			t.Fatalf("un tramo en pedazos de total=%d pedazo=%d no se tiene que activar: %+v", caso[0], caso[1], e)
		}
	}

	unGolpe := &MapRenderer{}
	unGolpe.ActivarTramoDeLaCache(200 * 1024)

	p := unGolpe.pedirIndicesDelTile(1000)

	if e := unGolpe.CensoDelTramo(); e.EnPedazos || e.Pedazos != 0 || e.Usado != 1000 || len(p) != 1000 {
		t.Fatalf("el tramo de un solo golpe tiene que seguir igual: %+v", e)
	}
}

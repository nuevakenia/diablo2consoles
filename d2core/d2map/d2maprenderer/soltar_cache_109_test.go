package d2maprenderer

// DC-002.109 -- LAS PRUEBAS DE "SOLTAR LA CACHE DEL NIVEL QUE SE DEJA".
//
// QUE SE PRUEBA, y por que cada cosa:
//
//	1. con un `soltar` que recibe los bloques, el tramo queda VACIO y el censo en cero (es la decision "a" del
//	   cambio de nivel: el nivel nuevo se compone con el monton libre);
//	2. con `soltar` nil (la PC, o el camino en que el monton externo no devuelve nada) los bloques SE QUEDAN y
//	   el tramo se reusa desde el byte 0 -- el camino de siempre, para que esta palanca no cambie la PC;
//	3. los bytes que informa son los que el que suelta devolvio DE VERDAD, no una cuenta de lo que habia;
//	4. un tramo de un solo golpe tambien se suelta.

import "testing"

func TestSoltarLaCacheDevuelveLosBloquesYVaciaElTramo(t *testing.T) {
	mr := &MapRenderer{}
	mr.ActivarTramoDeLaCacheEnPedazos(300*1024, 100*1024)

	mr.pedirIndicesDelTile(70 * 1024)
	mr.pedirIndicesDelTile(50 * 1024)

	if e := mr.CensoDelTramo(); e.Pedazos == 0 || e.Imagenes != 2 {
		t.Fatalf("la cache tiene que tener bloques e imagenes antes de soltar: %+v", e)
	}

	soltados := 0
	devueltos := 0

	bytes := mr.SoltarLaCacheDelNivel(func(bloque []byte) int {
		soltados++
		devueltos += len(bloque)

		return len(bloque)
	})

	e := mr.CensoDelTramo()

	if e.Pedazos != 0 || e.Bytes != 0 || e.Usado != 0 || e.Imagenes != 0 || e.Derrames != 0 {
		t.Fatalf("despues de soltar el tramo tiene que quedar vacio: %+v", e)
	}

	if bytes != devueltos {
		t.Fatalf("tiene que informar los bytes que el que suelta devolvio: %d contra %d", bytes, devueltos)
	}

	if soltados == 0 {
		t.Fatal("no se le paso ni un bloque al que suelta")
	}
}

// TestSoltarSinQuienRecibaReusaElTramo: sin `soltar` (la PC), no se devuelve nada y los bloques quedan para el
// nivel siguiente -- el contrato de ADR 0001 §1.4 (el tramo no se libera, se reusa desde el byte 0).
func TestSoltarSinQuienRecibaReusaElTramo(t *testing.T) {
	mr := &MapRenderer{}
	mr.ActivarTramoDeLaCacheEnPedazos(300*1024, 100*1024)

	mr.pedirIndicesDelTile(70 * 1024)

	antes := mr.CensoDelTramo()

	if bytes := mr.SoltarLaCacheDelNivel(nil); bytes != 0 {
		t.Fatalf("sin quien reciba no se devuelve nada: %d", bytes)
	}

	despues := mr.CensoDelTramo()

	if despues.Pedazos != antes.Pedazos || despues.Bytes != antes.Bytes {
		t.Fatalf("los bloques tienen que quedarse: antes=%+v despues=%+v", antes, despues)
	}

	if despues.Usado != 0 || despues.Imagenes != 0 {
		t.Fatalf("el uso tiene que quedar en cero para reusar desde el byte 0: %+v", despues)
	}
}

// TestSoltarUnGolpeTambienSeSuelta: el modo de un solo golpe (DC-002.104) tiene el mismo contrato.
func TestSoltarUnGolpeTambienSeSuelta(t *testing.T) {
	mr := &MapRenderer{}
	mr.ActivarTramoDeLaCache(100 * 1024)
	mr.pedirIndicesDelTile(64 * 1024)

	bytes := mr.SoltarLaCacheDelNivel(func(bloque []byte) int { return len(bloque) })
	if bytes != 100*1024 {
		t.Fatalf("el tramo de un golpe son 102400 B: %d", bytes)
	}

	e := mr.CensoDelTramo()
	if e.Bytes != 0 || e.Usado != 0 || e.Imagenes != 0 {
		t.Fatalf("despues de soltarlo el tramo tiene que quedar en cero: %+v", e)
	}

	// el nivel nuevo pide de nuevo: lo que no se puede es devolver un pedazo de memoria ya devuelta
	p := mr.pedirIndicesDelTile(1000)
	if len(p) != 1000 {
		t.Fatalf("el nivel nuevo tiene que poder pedir otra vez: %d", len(p))
	}
}

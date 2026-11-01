package d2ds1

// DC-002.110 -- LA PRUEBA EN CONTRA DE LA ARENA: el `.ds1` parseado con el bloque del monton de C tiene que ser
// EL MISMO que el parseado por el camino de siempre, celda por celda, y el bloque tiene que soltarse con el
// liberador inyectado.
//
// POR QUE ASI Y NO "compila y anda". Lo que esta tarjeta mueve son las REJILLAS de las capas: si la arena se
// armara mal (una fila corrida, una capa sin mudar, un tamaño de mas) el parseo seguiria "andando" y el cuadro
// de la consola saldria distinto -- que es justo el defecto que costo tres tarjetas encontrar en DC-002.109
// (una pared armada con la variante equivocada). Comparando TODAS las celdas de TODAS las capas contra el
// camino de siempre, el defecto cae aca y no en Flycast.

import (
	"testing"
	"unsafe"
)

// sinArenaDejaTodoComoEstaba: la palanca apagada no puede ni pedir bloque (es el camino de la PC).
func TestArenaDelDS1ApagadaNoPideNada(t *testing.T) {
	ds1 := exampleData()
	ds1.SetSize(57, 41)

	crudo := ds1.Marshal()

	antes := EstadoDeLaArena()

	obtenido, err := Unmarshal(crudo)
	if err != nil {
		t.Fatalf("no parseo: %v", err)
	}

	if obtenido.EnArena() {
		t.Fatal("con la palanca apagada el `.ds1` no puede vivir en la arena")
	}

	if despues := EstadoDeLaArena(); despues.Pedidas != antes.Pedidas {
		t.Fatalf("con la palanca apagada no se pide bloque: %d -> %d", antes.Pedidas, despues.Pedidas)
	}
}

// el parseo con arena es el mismo parseo: mismas celdas, mismo tamaño, y el bloque se devuelve al soltar.
func TestArenaDelDS1ParseaIgualYSeSuelta(t *testing.T) {
	ds1 := exampleData()
	ds1.SetSize(57, 41) // 2337 celdas x 4 capas: pasa el umbral de los 64 KB de libgodc

	crudo := ds1.Marshal()

	esperado, err := Unmarshal(crudo)
	if err != nil {
		t.Fatalf("no parseo sin arena: %v", err)
	}

	ActivarArenaDelDS1()

	defer func() {
		arenaDeTilesEncendida = false
		liberadorDeLaArena = nil
	}()

	var (
		liberado uintptr
		bytes    int
	)

	UsarLiberadorDeLaArena(func(direccion uintptr, tamano int) int {
		liberado, bytes = direccion, tamano

		return tamano
	})

	obtenido, err := Unmarshal(crudo)
	if err != nil {
		t.Fatalf("no parseo con arena: %v", err)
	}

	if !obtenido.EnArena() {
		t.Fatal("con la palanca prendida el `.ds1` tiene que vivir en la arena")
	}

	ancho, alto := obtenido.Size()
	capas := len(obtenido.Walls) + len(obtenido.Floors) + len(obtenido.Shadows) + len(obtenido.Substitutions)
	esperados := capas * ancho * alto * int(unsafe.Sizeof(Tile{}))

	if hay := obtenido.BytesDeLaArena(); hay != esperados {
		t.Fatalf("el bloque tiene %d B y tiene que ser UNO del tamaño exacto: %d B", hay, esperados)
	}

	compararLasCapas(t, esperado, obtenido)

	// soltar: el liberador recibe ese bloque, de ese tamaño, y las capas quedan vacias (no memoria devuelta)
	if devueltos := obtenido.SoltarArenaDeTiles(); devueltos != esperados {
		t.Fatalf("se devolvieron %d B y eran %d", devueltos, esperados)
	}

	if liberado == 0 || bytes != esperados {
		t.Fatalf("el liberador no recibio el bloque: direccion=%v bytes=%d (esperaba %d)", liberado, bytes, esperados)
	}

	if obtenido.EnArena() {
		t.Fatal("despues de soltar el `.ds1` no puede seguir diciendo que vive en la arena")
	}

	if tile := obtenido.Walls[0].Tile(0, 0); tile != nil {
		t.Fatal("despues de soltar una capa tiene que verse VACIA, no con memoria devuelta")
	}

	// y el que se parseo por el camino de siempre sigue intacto (la arena no toca memoria ajena)
	compararLasCapas(t, esperado, esperado)

	if tile := esperado.Walls[0].Tile(0, 0); tile == nil {
		t.Fatal("el `.ds1` del camino de siempre se quedo sin celdas")
	}
}

// un `.ds1` chico no llega al umbral: se queda en el monton de Go y se cuenta (la arena no compra nada ahi).
func TestArenaDelDS1NoSePideDebajoDelUmbral(t *testing.T) {
	chico := &DS1{ds1Layers: &ds1Layers{width: 4, height: 4}}

	if bytes := chico.reservarArenaDeTiles(4); bytes != 0 {
		t.Fatalf("un `.ds1` de 4x4 no puede pedir bloque: pidio %d B", bytes)
	}

	if chico.EnArena() {
		t.Fatal("un `.ds1` debajo del umbral no vive en la arena")
	}
}

// compararLasCapas: celda por celda, todas las capas de los dos `.ds1`.
func compararLasCapas(t *testing.T, esperado, obtenido *DS1) {
	t.Helper()

	ancho, alto := esperado.Size()

	for nombre, par := range map[string][2]layerGroup{
		"paredes":       {esperado.Walls, obtenido.Walls},
		"pisos":         {esperado.Floors, obtenido.Floors},
		"sombras":       {esperado.Shadows, obtenido.Shadows},
		"sustituciones": {esperado.Substitutions, obtenido.Substitutions},
	} {
		if len(par[0]) != len(par[1]) {
			t.Fatalf("%s: %d capas contra %d", nombre, len(par[0]), len(par[1]))
		}

		for capa := range par[0] {
			for y := 0; y < alto; y++ {
				for x := 0; x < ancho; x++ {
					a := par[0][capa].Tile(x, y)
					b := par[1][capa].Tile(x, y)

					if a == nil || b == nil {
						t.Fatalf("%s capa %d en (%d,%d): nil contra %v", nombre, capa, x, y, b)
					}

					if *a != *b {
						t.Fatalf("%s capa %d en (%d,%d): %+v contra %+v", nombre, capa, x, y, *a, *b)
					}
				}
			}
		}
	}
}

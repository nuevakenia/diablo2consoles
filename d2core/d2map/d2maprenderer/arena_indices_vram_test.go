package d2maprenderer

// DC-002.108 -- LA PRUEBA EN CONTRA DEL TRAMO CON RESERVADOR PROPIO: que los pixeles de la cache salgan DEL
// RESERVADOR (la VRAM del PVR, en la consola) y no del `make`, que el censo diga de donde vienen, y que si el
// reservador no tiene lugar la imagen se derrame al camino de siempre SIN tirar nada.
//
// POR QUE ESTA PRUEBA Y NO SOLO LA CORRIDA EN FLYCAT. En el emulador se mide que el cuadro sale y cuanto tarda;
// lo que no se puede ver ahi es si un pixel vino del reservador o de un `make` que andaba igual de bien (y eso
// importa: el punto de la tarjeta es que el monton de C NO los pida). Aca se comprueba con la direccion: cada
// pedazo tiene que caer ADENTRO del buffer del reservador.

import (
	"testing"
	"unsafe"
)

// arenaDePrueba: un reservador que reparte de un buffer propio y puede negarse (como la VRAM cuando no entra).
type arenaDePrueba struct {
	mem     []byte
	cursor  int
	pedidos []int
	niega   bool // se niega siempre, para medir el derrame
}

func (a *arenaDePrueba) reservar(n int) []byte {
	a.pedidos = append(a.pedidos, n)

	if a.niega || a.cursor+n > len(a.mem) {
		return nil
	}

	b := a.mem[a.cursor : a.cursor+n : a.cursor+n]
	a.cursor += n

	return b
}

// enLaArena: si el pedazo apunta adentro del buffer del reservador. Es LA prueba de que no salio del monton.
func (a *arenaDePrueba) enLaArena(p []byte) bool {
	if len(p) == 0 {
		return false
	}

	inicio := uintptr(unsafe.Pointer(&a.mem[0]))
	q := uintptr(unsafe.Pointer(&p[0]))

	return q >= inicio && q+uintptr(len(p)) <= inicio+uintptr(len(a.mem))
}

// TestLosPixelesDeLaCacheSalenDelReservador: con reservador, cada imagen cae en su buffer, el censo lo dice
// ("vram") y no hay derrames.
func TestLosPixelesDeLaCacheSalenDelReservador(t *testing.T) {
	arena := &arenaDePrueba{mem: make([]byte, 1024*1024)}

	mr := &MapRenderer{}
	mr.ActivarTramoDeLaCacheEnBloquesDe(1024*1024, 300*1024, arena.reservar, "vram")

	const imagen = 60 * 1024

	for i := 0; i < 6; i++ {
		p := mr.pedirIndicesDelTile(imagen)

		if len(p) != imagen || cap(p) != imagen {
			t.Fatalf("imagen %d: len=%d cap=%d (tiene que ser un pedazo del reservador)", i, len(p), cap(p))
		}

		if !arena.enLaArena(p) {
			t.Fatalf("imagen %d: el pedazo NO cae adentro del buffer del reservador: salio del monton", i)
		}

		// marcar el pixel 0 y confirmar que la marca quedo EN LA ARENA (el pedazo es una vista, no una copia)
		p[0] = byte(i + 1)

		off := uintptr(unsafe.Pointer(&p[0])) - uintptr(unsafe.Pointer(&arena.mem[0]))
		if arena.mem[off] != byte(i+1) {
			t.Fatalf("imagen %d: el pedazo es una COPIA (la marca no llego a la arena)", i)
		}
	}

	e := mr.CensoDelTramo()

	if e.Origen != "vram" {
		t.Fatalf("el censo tiene que decir de donde vienen los bloques: origen=%q", e.Origen)
	}

	if e.Derrames != 0 || e.BytesDerramados != 0 {
		t.Fatalf("sin derrames: %+v", e)
	}

	if e.Imagenes != 6 || e.Usado != 6*imagen {
		t.Fatalf("las 6 imagenes tienen que salir del tramo: %+v", e)
	}

	// dos bloques de 300 KB (el primero se llena con 5 imagenes, el sexto abre el segundo)
	if e.Pedazos != 2 || e.Bytes != 600*1024 {
		t.Fatalf("censo del tramo: %+v (dos bloques de 300 KB)", e)
	}

	if len(arena.pedidos) != 2 || arena.cursor != 600*1024 {
		t.Fatalf("el reservador recibio %v y entrego %d B", arena.pedidos, arena.cursor)
	}
}

// TestSiElReservadorNoTieneLugarLaImagenSeDerrama: el reservador se niega (la VRAM no da) y la imagen sale por
// `make` como siempre, contada. Lo que NO puede pasar es que reviente: el pedido que no entra en la VRAM
// devuelve nil, no tira el runtime.
func TestSiElReservadorNoTieneLugarLaImagenSeDerrama(t *testing.T) {
	arena := &arenaDePrueba{mem: make([]byte, 1024), niega: true}

	mr := &MapRenderer{}
	mr.ActivarTramoDeLaCacheEnBloquesDe(1024*1024, 300*1024, arena.reservar, "vram")

	p := mr.pedirIndicesDelTile(50 * 1024)

	if len(p) != 50*1024 || cap(p) != 50*1024 {
		t.Fatalf("la imagen tiene que salir igual (por make): len=%d cap=%d", len(p), cap(p))
	}

	if arena.enLaArena(p) {
		t.Fatal("el pedazo NO puede caer en la arena: el reservador se nego")
	}

	e := mr.CensoDelTramo()

	if e.Pedazos != 0 || e.Derrames != 1 || e.BytesDerramados != 50*1024 {
		t.Fatalf("censo del tramo: %+v (cero bloques, un derrame de 50 KB)", e)
	}

	if e.Origen != "vram" {
		t.Fatalf("el origen se informa igual: %+v", e)
	}
}

// TestSinReservadorElOrigenEsElMonton: el camino de DC-002.106 no cambia de numero ni de nombre (el censo de
// las corridas anteriores tiene que poder compararse con las de esta tarjeta).
func TestSinReservadorElOrigenEsElMonton(t *testing.T) {
	mr := &MapRenderer{}
	mr.ActivarTramoDeLaCacheEnPedazos(1024*1024, 300*1024)

	if p := mr.pedirIndicesDelTile(60 * 1024); len(p) != 60*1024 {
		t.Fatalf("len=%d", len(p))
	}

	if e := mr.CensoDelTramo(); e.Origen != "monton" {
		t.Fatalf("sin reservador el origen es el monton: %+v", e)
	}
}

// TestElTramoUnicoSaleDelReservador: el pedido de UN golpe tambien puede venir del reservador (DC-002.108: es
// el pedido de 2.723.840 B que en el monton de C muere por contiguidad). El tramo tiene que ser UNO, del tamano
// pedido y del reservador, y las imagenes se reparten adentro sin derrames.
func TestElTramoUnicoSaleDelReservador(t *testing.T) {
	arena := &arenaDePrueba{mem: make([]byte, 512*1024)}

	mr := &MapRenderer{}
	mr.ActivarTramoDeLaCacheDe(400*1024, arena.reservar, "vram")

	if e := mr.CensoDelTramo(); !e.Activa || e.EnPedazos || e.Bytes != 400*1024 || e.Origen != "vram" {
		t.Fatalf("el tramo unico tiene que quedar activo, de un golpe y del reservador: %+v", e)
	}

	for i := 0; i < 6; i++ {
		p := mr.pedirIndicesDelTile(60 * 1024)

		if len(p) != 60*1024 || cap(p) != 60*1024 {
			t.Fatalf("imagen %d: len=%d cap=%d", i, len(p), cap(p))
		}

		if !arena.enLaArena(p) {
			t.Fatalf("imagen %d: el pedazo no cae en el buffer del reservador", i)
		}
	}

	e := mr.CensoDelTramo()

	if e.Imagenes != 6 || e.Usado != 360*1024 || e.Derrames != 0 {
		t.Fatalf("censo del tramo unico: %+v", e)
	}

	if len(arena.pedidos) != 1 || arena.pedidos[0] != 400*1024 {
		t.Fatalf("el reservador tiene que recibir UN pedido de 400 KB: %v", arena.pedidos)
	}
}

// TestElTramoUnicoSeNiegaSiElReservadorNoTieneLugar: sin lugar en el reservador el tramo no se activa (nil), y
// la cache vuelve al camino de siempre: la tarjeta puede decir "la VRAM no alcanzo" en vez de morir.
func TestElTramoUnicoSeNiegaSiElReservadorNoTieneLugar(t *testing.T) {
	arena := &arenaDePrueba{mem: make([]byte, 1024), niega: true}

	mr := &MapRenderer{}
	mr.ActivarTramoDeLaCacheDe(400*1024, arena.reservar, "vram")

	if e := mr.CensoDelTramo(); e.Activa {
		t.Fatalf("sin lugar el tramo no puede quedar activo: %+v", e)
	}

	if p := mr.pedirIndicesDelTile(60 * 1024); len(p) != 60*1024 || arena.enLaArena(p) {
		t.Fatalf("la imagen sale por make: len=%d", len(p))
	}
}

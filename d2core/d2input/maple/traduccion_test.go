package maple

import (
	"testing"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2enum"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2interface"
)

// conjunto: las teclas de la lista, para comparar sin importar el orden.
func conjunto(teclas []d2enum.Key) map[d2enum.Key]bool {
	r := make(map[d2enum.Key]bool, len(teclas))
	for _, k := range teclas {
		r[k] = true
	}

	return r
}

// TestTablaTieneTodosLosBitsDelControl: la tabla no se olvida de ningun bit del aparato y no repite ninguno.
// Los que quedan SIN tecla son exactamente Z, C y D -- los tres que el shim tampoco mapea (el limite esta
// declarado en doc.go). Si algun dia alguien mapea uno, este test avisa que la evidencia quedo vieja; si
// alguien se come una fila, avisa que el boton desaparecio en silencio.
func TestTablaTieneTodosLosBitsDelControl(t *testing.T) {
	todos := []uint32{BitC, BitB, BitA, BitStart, BitUp, BitDown, BitLeft, BitRight, BitZ, BitY, BitX, BitD}
	sinTeclaEsperados := map[uint32]bool{BitZ: true, BitC: true, BitD: true}
	vistos := map[uint32]int{}

	for i := range Tabla {
		vistos[Tabla[i].Bit]++
	}

	for _, bit := range todos {
		if vistos[bit] != 1 {
			t.Fatalf("el bit %d aparece %d veces en la tabla (tiene que aparecer 1)", bit, vistos[bit])
		}
	}

	for i := range Tabla {
		vacia := len(Tabla[i].Teclas) == 0
		if vacia != sinTeclaEsperados[Tabla[i].Bit] {
			t.Fatalf("el boton %s (%d) tiene %d teclas; se esperaba que %s tecla",
				Tabla[i].Boton, Tabla[i].Bit, len(Tabla[i].Teclas), map[bool]string{true: "NO tuviera", false: "SI tuviera"}[vacia])
		}
	}
}

// TestCadaFilaProduceSusTeclasYSuBoton: apretando UN bit, el motor ve exactamente las teclas de esa fila y
// ningun otro boton del mouse. Es la prueba de que la tabla y los metodos dicen lo mismo: si la tabla dice
// "A -> Enter + clic izquierdo" y el metodo contesta otra cosa, esto se pone rojo.
func TestCadaFilaProduceSusTeclasYSuBoton(t *testing.T) {
	for i := range Tabla {
		e := Tabla[i]
		s := NuevoServicio(640, 480)
		s.Actualizar(e.Bit, 0, 0, 0, 0)

		esperadas := conjunto(e.Teclas)
		obtenidas := conjunto(s.TeclasApretadas())

		if len(obtenidas) != len(esperadas) {
			t.Fatalf("boton %s: apretadas %v, se esperaban %v", e.Boton, s.TeclasApretadas(), e.Teclas)
		}

		for _, k := range e.Teclas {
			if !s.IsKeyPressed(k) {
				t.Fatalf("boton %s: la tecla %d no figura apretada", e.Boton, int(k))
			}
		}

		for b := 0; b < botonesDelMouse; b++ {
			quiereEste := e.EsDelMouse && int(e.Mouse) == b
			if s.IsMouseButtonPressed(d2enum.MouseButton(b)) != quiereEste {
				t.Fatalf("boton %s: el boton del mouse %d contesto %v", e.Boton, b, !quiereEste)
			}
		}
	}
}

// TestElMapeoEsElDelShim: LA TABLA ESCRITA A MANO, boton por boton, tal como esta en
// console/dc/engine/ebiten/v2/control.go (que es lo que el juego ya usa desde DC-002.11/38). Este test es el
// que caza una edicion de la tabla que cambie el mapeo: el de arriba compara la tabla consigo misma (si la
// fila del X dice "clic derecho", las dos cosas dicen lo mismo y pasa). Medido con el mutante "X aprieta el
// clic derecho": la tabla consigo misma quedaba verde; contra esta lista, rojo.
func TestElMapeoEsElDelShim(t *testing.T) {
	type fila struct {
		bit     uint32
		boton   string
		teclas  []d2enum.Key
		mouse   d2enum.MouseButton
		esMouse bool
	}

	esperado := []fila{
		{BitUp, "ARRIBA", []d2enum.Key{d2enum.KeyUp}, 0, false},
		{BitDown, "ABAJO", []d2enum.Key{d2enum.KeyDown}, 0, false},
		{BitLeft, "IZQUIERDA", []d2enum.Key{d2enum.KeyLeft}, 0, false},
		{BitRight, "DERECHA", []d2enum.Key{d2enum.KeyRight}, 0, false},
		{BitA, "A", []d2enum.Key{d2enum.KeyEnter}, d2enum.MouseButtonLeft, true},
		{BitB, "B", []d2enum.Key{d2enum.KeyEscape}, d2enum.MouseButtonRight, true},
		{BitX, "X", []d2enum.Key{d2enum.KeyX}, d2enum.MouseButtonMiddle, true},
		{BitY, "Y", []d2enum.Key{d2enum.KeyY}, 0, false},
		{BitStart, "START", []d2enum.Key{d2enum.KeyEnter}, 0, false},
		{BitZ, "Z", nil, 0, false},
		{BitC, "C", nil, 0, false},
		{BitD, "D", nil, 0, false},
	}

	if len(esperado) != len(Tabla) {
		t.Fatalf("la tabla tiene %d filas y el mapeo del shim tiene %d", len(Tabla), len(esperado))
	}

	for _, f := range esperado {
		var hallada *Entrada

		for i := range Tabla {
			if Tabla[i].Bit == f.bit {
				hallada = &Tabla[i]

				break
			}
		}

		if hallada == nil {
			t.Fatalf("el bit %d (%s) no esta en la tabla", f.bit, f.boton)
		}

		if hallada.Boton != f.boton || len(hallada.Teclas) != len(f.teclas) ||
			hallada.EsDelMouse != f.esMouse || (f.esMouse && hallada.Mouse != f.mouse) {
			t.Fatalf("la fila del %s no es la del shim: %+v", f.boton, *hallada)
		}

		for i, k := range f.teclas {
			if hallada.Teclas[i] != k {
				t.Fatalf("el %s tiene que apretar la tecla %d y la tabla dice %d", f.boton, int(k), int(hallada.Teclas[i]))
			}
		}
	}

	// y los gatillos: L -> Q, R -> E, con el umbral 32 del shim
	if TeclasDeLosGatillos[0] != d2enum.KeyQ || TeclasDeLosGatillos[1] != d2enum.KeyE {
		t.Fatalf("los gatillos tienen que apretar Q y E; la tabla dice %d y %d",
			int(TeclasDeLosGatillos[0]), int(TeclasDeLosGatillos[1]))
	}

	if UmbralGatillo != 32 {
		t.Fatalf("el umbral del gatillo es %d, el del shim es 32", UmbralGatillo)
	}
}

// TestFlancosYDuracion: "esta apretada", "recien apretada" y "recien soltada" son tres cosas distintas y la
// duracion cuenta cuadros. Un boton apretado no repite el flanco (los menus se mueven UNA vez por pulsacion).
func TestFlancosYDuracion(t *testing.T) {
	s := NuevoServicio(640, 480)

	// cuadro 1: nada
	s.Actualizar(0, 0, 0, 0, 0)

	if s.IsKeyPressed(d2enum.KeyY) || s.IsKeyJustPressed(d2enum.KeyY) || s.IsKeyJustReleased(d2enum.KeyY) {
		t.Fatal("con el control quieto KeyY no puede estar apretada, ni recien apretada, ni recien soltada")
	}

	// cuadro 2: se aprieta Y
	s.Actualizar(BitY, 0, 0, 0, 0)

	if !s.IsKeyPressed(d2enum.KeyY) || !s.IsKeyJustPressed(d2enum.KeyY) || s.IsKeyJustReleased(d2enum.KeyY) {
		t.Fatal("el cuadro que se aprieta Y tiene que dar apretada + recien apretada, y NO recien soltada")
	}

	if d := s.KeyPressDuration(d2enum.KeyY); d != 1 {
		t.Fatalf("la duracion del primer cuadro apretado es %d, se esperaba 1", d)
	}

	// cuadro 3: sigue apretado -- el flanco NO se repite
	s.Actualizar(BitY, 0, 0, 0, 0)

	if s.IsKeyJustPressed(d2enum.KeyY) {
		t.Fatal("un boton que sigue apretado no puede volver a decir 'recien apretada' (los menus se moverian solos)")
	}

	if d := s.KeyPressDuration(d2enum.KeyY); d != 2 {
		t.Fatalf("la duracion del segundo cuadro apretado es %d, se esperaba 2", d)
	}

	// cuadro 4: se suelta
	s.Actualizar(0, 0, 0, 0, 0)

	if s.IsKeyPressed(d2enum.KeyY) || s.IsKeyJustPressed(d2enum.KeyY) || !s.IsKeyJustReleased(d2enum.KeyY) {
		t.Fatal("el cuadro que se suelta Y tiene que dar recien soltada y nada mas")
	}

	if d := s.KeyPressDuration(d2enum.KeyY); d != 0 {
		t.Fatalf("la duracion con la tecla suelta es %d, se esperaba 0", d)
	}

	// cuadro 5: sigue suelto -- el flanco de subida tampoco se repite
	s.Actualizar(0, 0, 0, 0, 0)

	if s.IsKeyJustReleased(d2enum.KeyY) {
		t.Fatal("un boton que sigue suelto no puede volver a decir 'recien soltada'")
	}
}

// TestClicDelMouseSigueLosFlancos: lo mismo pero por el lado del mouse, que es el camino que usa el juego para
// caminar y pegar (A = clic izquierdo).
func TestClicDelMouseSigueLosFlancos(t *testing.T) {
	s := NuevoServicio(640, 480)

	s.Actualizar(BitA, 0, 0, 0, 0)

	if !s.IsMouseButtonPressed(d2enum.MouseButtonLeft) || !s.IsMouseButtonJustPressed(d2enum.MouseButtonLeft) {
		t.Fatal("el A tiene que dar clic izquierdo apretado y recien apretado")
	}

	if s.IsMouseButtonPressed(d2enum.MouseButtonRight) || s.IsMouseButtonPressed(d2enum.MouseButtonMiddle) {
		t.Fatal("el A no puede apretar el boton derecho ni el del medio")
	}

	s.Actualizar(0, 0, 0, 0, 0)

	if s.IsMouseButtonPressed(d2enum.MouseButtonLeft) || !s.IsMouseButtonJustReleased(d2enum.MouseButtonLeft) {
		t.Fatal("al soltar el A el clic izquierdo tiene que dar recien soltado y no apretado")
	}
}

// TestTeclasApretadasSinRepetir: el A y el Start dan la MISMA tecla (Enter). Apretar los dos no son dos
// Enters: es una tecla apretada una vez (si se repitiera, un menu avanzaria dos veces).
func TestTeclasApretadasSinRepetir(t *testing.T) {
	s := NuevoServicio(640, 480)
	s.Actualizar(BitA|BitStart, 0, 0, 0, 0)

	n := 0

	for _, k := range s.TeclasApretadas() {
		if k == d2enum.KeyEnter {
			n++
		}
	}

	if n != 1 {
		t.Fatalf("Enter aparece %d veces con A+Start apretados, se esperaba 1", n)
	}
}

// TestPalancaCrucetaYZonaMuerta: los numeros del cursor son los del shim (8 px por cuadro con la palanca a
// fondo, 12 con la cruceta, zona muerta de 16) y la palanca apoya sin mover nada.
//
// LOS NUMEROS VAN ESCRITOS A MANO Y NO DE LAS CONSTANTES DEL PAQUETE: si el test comparara contra
// velocidadCruceta, cambiar la constante a 13 lo dejaria verde (el test estaria midiendo el paquete con su
// propia regla). Medido con el mutante: con las constantes, velocidadCruceta=13 pasaba; con los literales, se
// pone rojo. Los numeros son los de console/dc/engine/ebiten/v2/control.go, que es el contrato.
func TestPalancaCrucetaYZonaMuerta(t *testing.T) {
	// la palanca a fondo a la derecha: 128 * 8 / 128 = 8 px
	s := NuevoServicio(640, 480)
	s.Actualizar(0, 128, 0, 0, 0)

	x0, _ := s.CursorPosition()
	s.Actualizar(0, 128, 0, 0, 0)
	x1, _ := s.CursorPosition()

	if x1-x0 != 8 {
		t.Fatalf("la palanca a fondo movio %d px, se esperaban 8", x1-x0)
	}

	// dentro de la zona muerta no se mueve: un control viejo no vuelve exacto al centro
	s2 := NuevoServicio(640, 480)
	s2.Actualizar(0, 16, -16, 0, 0)

	x, y := s2.CursorPosition()
	if x != 320 || y != 240 {
		t.Fatalf("con la palanca dentro de la zona muerta el cursor quedo en %d,%d (se esperaba 320,240)", x, y)
	}

	// la cruceta: 12 px por cuadro, todo o nada
	s3 := NuevoServicio(640, 480)
	s3.Actualizar(BitRight, 0, 0, 0, 0)

	x, _ = s3.CursorPosition()
	if x != 332 {
		t.Fatalf("la cruceta a la derecha dejo el cursor en x=%d, se esperaba 332 (320+12)", x)
	}

	// y la palanca y la cruceta se SUMAN (en el shim es el mismo caminar con las dos)
	s4 := NuevoServicio(640, 480)
	s4.Actualizar(BitRight, 128, 0, 0, 0)

	x, _ = s4.CursorPosition()
	if x != 340 {
		t.Fatalf("con la cruceta y la palanca a la vez el cursor quedo en x=%d, se esperaba 340 (320+12+8)", x)
	}
}

// TestCursorSeQuedaEnLaPantalla: el cursor no se va del cuadro por mas que se sostenga la cruceta. Si se
// fuera, el motor seguiria recibiendo una posicion valida pero el jugador perderia de vista donde apunta.
func TestCursorSeQuedaEnLaPantalla(t *testing.T) {
	s := NuevoServicio(320, 240)

	for i := 0; i < 100; i++ {
		s.Actualizar(BitRight|BitDown, 0, 0, 0, 0)
	}

	x, y := s.CursorPosition()
	if x != 319 || y != 239 {
		t.Fatalf("el cursor quedo en %d,%d; tenia que quedar en la esquina 319,239", x, y)
	}

	for i := 0; i < 100; i++ {
		s.Actualizar(BitLeft|BitUp, 0, 0, 0, 0)
	}

	x, y = s.CursorPosition()
	if x != 0 || y != 0 {
		t.Fatalf("el cursor quedo en %d,%d; tenia que quedar en 0,0", x, y)
	}
}

// TestGatillosPorUmbral: los gatillos son analogos y el umbral es el del shim (32). Con 32 justo NO esta
// apretado; con 33 si. Sin esto un dedo apoyado dispararia el atajo. Los numeros van a mano: comparar contra
// UmbralGatillo dejaria pasar el mutante `>` -> `>=` (medido).
func TestGatillosPorUmbral(t *testing.T) {
	s := NuevoServicio(640, 480)

	s.Actualizar(0, 0, 0, 32, 32)
	if s.IsKeyPressed(d2enum.KeyQ) || s.IsKeyPressed(d2enum.KeyE) {
		t.Fatal("con los gatillos en el umbral (32) no tienen que estar apretados")
	}

	s.Actualizar(0, 0, 0, 33, 33)
	if !s.IsKeyPressed(d2enum.KeyQ) || !s.IsKeyPressed(d2enum.KeyE) {
		t.Fatal("arriba del umbral los dos gatillos tienen que apretar Q y E")
	}

	s.Actualizar(0, 0, 0, 255, 0)
	if !s.IsKeyPressed(d2enum.KeyQ) || s.IsKeyPressed(d2enum.KeyE) {
		t.Fatal("con el L a fondo solo se aprieta Q (el R esta suelto)")
	}
}

// TestTeclasRecienApretadasYSoltadas: los flancos por la via de la evidencia (lo que imprime la prueba de la
// consola) dicen lo mismo que los metodos del contrato.
func TestTeclasRecienApretadasYSoltadas(t *testing.T) {
	s := NuevoServicio(640, 480)

	s.Actualizar(BitUp|BitY, 0, 0, 0, 0)

	recien := conjunto(s.TeclasRecienApretadas())
	if !recien[d2enum.KeyUp] || !recien[d2enum.KeyY] || len(recien) != 2 {
		t.Fatalf("los flancos de bajada dieron %v, se esperaban ARRIBA y Y", s.TeclasRecienApretadas())
	}

	s.Actualizar(BitUp, 0, 0, 0, 0)

	soltadas := conjunto(s.TeclasRecienSoltadas())
	if !soltadas[d2enum.KeyY] || len(soltadas) != 1 {
		t.Fatalf("los flancos de subida dieron %v, se esperaba solo Y", s.TeclasRecienSoltadas())
	}

	if len(s.TeclasRecienApretadas()) != 0 {
		t.Fatal("no se apreto ninguna tecla nueva en este cuadro")
	}
}

// TestSinLecturaNoContestaNada: un servicio recien creado (todavia no se leyo el aparato) no tiene teclas
// apretadas ni movio el cursor. "No hay control" no es "el control esta quieto".
func TestSinLecturaNoContestaNada(t *testing.T) {
	s := NuevoServicio(640, 480)

	if s.HayLectura() {
		t.Fatal("un servicio recien creado no tiene lectura")
	}

	if len(s.TeclasApretadas()) != 0 || s.IsMouseButtonPressed(d2enum.MouseButtonLeft) {
		t.Fatal("sin lectura no hay teclas ni clics")
	}

	x, y := s.CursorPosition()
	if x != 320 || y != 240 {
		t.Fatalf("el cursor arranca en %d,%d, se esperaba el centro 320,240", x, y)
	}
}

// TestElServicioCumpleElContratoPorLaInterfaz: los nueve metodos se llaman A TRAVES de d2interface.InputService,
// que es como los va a llamar el motor. Este test no agrega semantica: es el que se pone rojo si una firma se
// despega del contrato (la asignacion de arriba lo caza en la compilacion; esto lo ejercita).
func TestElServicioCumpleElContratoPorLaInterfaz(t *testing.T) {
	var entrada d2interface.InputService = NuevoServicio(640, 480)
	svc := entrada.(*Servicio)
	svc.Actualizar(BitA|BitRight, 40, 0, 200, 0)

	if x, _ := entrada.CursorPosition(); x <= 320 {
		t.Fatalf("la cruceta a la derecha no movio el cursor por la interfaz (x=%d)", x)
	}

	if entrada.InputChars() != nil {
		t.Fatal("en la consola no hay teclado: InputChars tiene que ser nil")
	}

	if !entrada.IsKeyPressed(d2enum.KeyEnter) || !entrada.IsKeyJustPressed(d2enum.KeyEnter) {
		t.Fatal("IsKeyPressed/IsKeyJustPressed de Enter por la interfaz")
	}

	if !entrada.IsMouseButtonPressed(d2enum.MouseButtonLeft) || !entrada.IsMouseButtonJustPressed(d2enum.MouseButtonLeft) {
		t.Fatal("el clic izquierdo del A por la interfaz")
	}

	if entrada.KeyPressDuration(d2enum.KeyEnter) != 1 {
		t.Fatal("KeyPressDuration de Enter en el primer cuadro tiene que ser 1")
	}

	if !entrada.IsKeyPressed(d2enum.KeyQ) {
		t.Fatal("el gatillo L tiene que apretar Q por la interfaz")
	}

	svc.Actualizar(0, 0, 0, 0, 0)

	if !entrada.IsKeyJustReleased(d2enum.KeyEnter) || !entrada.IsMouseButtonJustReleased(d2enum.MouseButtonLeft) {
		t.Fatal("al soltar todo, Enter y el clic tienen que dar recien soltados")
	}
}

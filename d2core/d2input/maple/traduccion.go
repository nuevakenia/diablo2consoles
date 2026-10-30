package maple

import (
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2enum"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2interface"
)

// ============================ 1. LOS BITS DEL CONTROL ============================

// Los bits de los botones del control de la Dreamcast, COPIADOS del header de KOS
// (dc/maple/controller.h:103-118) y no de memoria -- los mismos numeros que usa
// console/dc/engine/ebiten/v2/control.go. El orden importa: es el orden de los BITS, no el de los botones en
// el aparato.
const (
	BitC     uint32 = 1 << 0
	BitB     uint32 = 1 << 1
	BitA     uint32 = 1 << 2
	BitStart uint32 = 1 << 3
	BitUp    uint32 = 1 << 4
	BitDown  uint32 = 1 << 5
	BitLeft  uint32 = 1 << 6
	BitRight uint32 = 1 << 7
	BitZ     uint32 = 1 << 8
	BitY     uint32 = 1 << 9
	BitX     uint32 = 1 << 10
	BitD     uint32 = 1 << 11
)

// ============================ 2. LA TABLA DE MAPEO ============================

// Entrada es una fila de la tabla: de que bit del control sale, como se llama el boton y que le llega al
// motor (teclas del teclado y, si ademas es un boton del mouse, cual).
//
// Teclas VACIA no es un olvido: es un boton del aparato que el motor no pregunta (ver doc.go). La tabla lo
// dice para que el que lea la evidencia sepa que existen y que quedaron afuera a proposito.
type Entrada struct {
	Bit        uint32
	Boton      string
	Teclas     []d2enum.Key
	EsDelMouse bool
	Mouse      d2enum.MouseButton
}

// Tabla es EL MAPEO del control, en el orden en que se documenta en la evidencia. Se recorre entera en cada
// cuadro (12 filas: es mas barato que una busqueda y no necesita indices que se puedan desincronizar).
var Tabla = [...]Entrada{
	{Bit: BitUp, Boton: "ARRIBA", Teclas: []d2enum.Key{d2enum.KeyUp}},
	{Bit: BitDown, Boton: "ABAJO", Teclas: []d2enum.Key{d2enum.KeyDown}},
	{Bit: BitLeft, Boton: "IZQUIERDA", Teclas: []d2enum.Key{d2enum.KeyLeft}},
	{Bit: BitRight, Boton: "DERECHA", Teclas: []d2enum.Key{d2enum.KeyRight}},
	{Bit: BitA, Boton: "A", Teclas: []d2enum.Key{d2enum.KeyEnter}, EsDelMouse: true, Mouse: d2enum.MouseButtonLeft},
	{Bit: BitB, Boton: "B", Teclas: []d2enum.Key{d2enum.KeyEscape}, EsDelMouse: true, Mouse: d2enum.MouseButtonRight},
	{Bit: BitX, Boton: "X", Teclas: []d2enum.Key{d2enum.KeyX}, EsDelMouse: true, Mouse: d2enum.MouseButtonMiddle},
	{Bit: BitY, Boton: "Y", Teclas: []d2enum.Key{d2enum.KeyY}},
	{Bit: BitStart, Boton: "START", Teclas: []d2enum.Key{d2enum.KeyEnter}},
	{Bit: BitZ, Boton: "Z", Teclas: nil},
	{Bit: BitC, Boton: "C", Teclas: nil},
	{Bit: BitD, Boton: "D", Teclas: nil},
}

// UmbralGatillo: los gatillos L y R son analogos (0..255), no bits. Se dan por apretados arriba de este
// numero, que es el mismo 32 del shim (control.go): un dedo apoyado no es un gatillo apretado.
const UmbralGatillo = 32

// TeclasDeLosGatillos: L -> Q y R -> E, en ese orden. Son los atajos que en la PC estan al lado de la mano
// izquierda (el shim los usa para lo mismo).
var TeclasDeLosGatillos = [2]d2enum.Key{d2enum.KeyQ, d2enum.KeyE}

// NombreDelBoton: el nombre de la fila de la tabla que tiene ese bit ("A", "DERECHA"), o "" si el bit no
// esta en la tabla. Es lo que imprime la prueba de la consola.
func NombreDelBoton(bit uint32) string {
	for i := range Tabla {
		if Tabla[i].Bit == bit {
			return Tabla[i].Boton
		}
	}

	return ""
}

// NombreDeTecla: el nombre de la tecla, para la evidencia y los logs. Solo las teclas que este backend puede
// producir tienen nombre; el resto (que la consola nunca aprieta) sale "OTRA" y el numero lo dice el que
// imprime.
func NombreDeTecla(k d2enum.Key) string {
	switch k {
	case d2enum.KeyUp:
		return "ARRIBA"
	case d2enum.KeyDown:
		return "ABAJO"
	case d2enum.KeyLeft:
		return "IZQUIERDA"
	case d2enum.KeyRight:
		return "DERECHA"
	case d2enum.KeyEnter:
		return "ENTER"
	case d2enum.KeyEscape:
		return "ESCAPE"
	case d2enum.KeyQ:
		return "Q"
	case d2enum.KeyE:
		return "E"
	case d2enum.KeyX:
		return "X"
	case d2enum.KeyY:
		return "Y"
	}

	return "OTRA"
}

// NombreDelBotonDelMouse: el nombre del boton del mouse, para la evidencia.
func NombreDelBotonDelMouse(b d2enum.MouseButton) string {
	switch b {
	case d2enum.MouseButtonLeft:
		return "IZQUIERDO"
	case d2enum.MouseButtonMiddle:
		return "MEDIO"
	case d2enum.MouseButtonRight:
		return "DERECHO"
	}

	return "OTRO"
}

// ============================ 3. EL SERVICIO ============================

// teclasQueSeSiguen: cuantas teclas puede preguntar el motor (d2enum.Key va de KeyMin a KeyMax, que es
// KeyMeta). Los estados por tecla son arrays de este tamano y no mapas: son 3 arrays de bytes, no reservan
// nada en el arranque y no dependen de que una clave exista en el mapa.
const teclasQueSeSiguen = int(d2enum.KeyMax) + 1

// botonesDelMouse: d2enum.MouseButton tiene tres valores (izquierdo, medio, derecho).
const botonesDelMouse = int(d2enum.MouseButtonMax) + 1

// Las constantes del cursor: LAS MISMAS del shim (control.go), no otras. La palanca es analoga y con 8
// pixeles por cuadro a fondo el cursor cruza la pantalla en 80 cuadros (segundo y medio); la cruceta es
// digital (todo o nada) y con el mismo numero seria lentisima, asi que va mas rapido para compensar y los dos
// se sienten parecidos.
const (
	velocidadPalanca = 8
	velocidadCruceta = 12

	// zonaMuertaPalanca: un control viejo no vuelve exacto al centro, y sin zona muerta el cursor se va solo.
	zonaMuertaPalanca = 16
)

// Servicio es la implementacion de d2interface.InputService sobre el control de la Dreamcast. Guarda el
// estado de UN cuadro: el de ahora y el del cuadro anterior (que es lo que hace posible "recien apretada" y
// "recien soltada", que son cosas distintas de "esta apretada").
type Servicio struct {
	ancho, alto int

	// el cursor virtual (el "mouse" del motor), en pixeles de la pantalla del juego
	cursorX, cursorY int

	// lo ultimo que se leyo del aparato, crudo (es lo que se imprime en la evidencia)
	botones  uint32
	palancaX int32
	palancaY int32
	gatilloL int32
	gatilloR int32
	leido    bool

	// el estado por tecla y por boton del mouse: el de este cuadro y el del anterior
	actual     [teclasQueSeSiguen]bool
	antes      [teclasQueSeSiguen]bool
	cuadros    [teclasQueSeSiguen]int
	mouse      [botonesDelMouse]bool
	mouseAntes [botonesDelMouse]bool
}

// que el contrato se cumpla no es una suposicion: si d2interface.InputService cambia, esto no compila. Y no
// esta en un archivo con //go:build: el compilador del host lo exige tambien.
var _ d2interface.InputService = (*Servicio)(nil)

// NuevoServicio: el servicio con el cursor en el centro de la pantalla del juego. ancho y alto son los de la
// pantalla logica del motor (640x480 en la consola): el cursor se queda adentro de eso.
func NuevoServicio(ancho, alto int) *Servicio {
	if ancho <= 0 {
		ancho = 640
	}

	if alto <= 0 {
		alto = 480
	}

	return &Servicio{ancho: ancho, alto: alto, cursorX: ancho / 2, cursorY: alto / 2}
}

// Actualizar avanza el servicio UN cuadro con lo que se leyo del aparato: los bits de los botones y los
// cuatro valores analogos (palanca y los dos gatillos). Es la unica entrada de estado: el que lee el maple
// (control.go, solo para la consola) y la prueba del host llaman a esta misma funcion, asi la traduccion que
// se prueba en el host es la que corre en la consola.
//
// El orden importa: primero se guarda el cuadro anterior, despues se calcula el de ahora y recien al final se
// mueve el cursor. Si se moviera antes, "recien soltada" se calcularia contra un estado ya pisado.
func (s *Servicio) Actualizar(botones uint32, palancaX, palancaY, gatilloL, gatilloR int32) {
	s.botones, s.palancaX, s.palancaY = botones, palancaX, palancaY
	s.gatilloL, s.gatilloR = gatilloL, gatilloR
	s.leido = true

	// 1) el cuadro anterior queda guardado (los flancos se calculan contra esto)
	s.antes = s.actual
	s.mouseAntes = s.mouse

	// 2) el cuadro de ahora: todo apagado y despues solo lo que la tabla y los gatillos prenden
	for k := range s.actual {
		s.actual[k] = false
	}

	for b := range s.mouse {
		s.mouse[b] = false
	}

	for i := range Tabla {
		e := &Tabla[i]
		if botones&e.Bit == 0 {
			continue
		}

		for _, k := range e.Teclas {
			if idx := indiceDeTecla(k); idx >= 0 {
				s.actual[idx] = true
			}
		}

		if e.EsDelMouse {
			if b := indiceDeMouse(e.Mouse); b >= 0 {
				s.mouse[b] = true
			}
		}
	}

	if gatilloL > UmbralGatillo {
		s.actual[int(TeclasDeLosGatillos[0])] = true
	}

	if gatilloR > UmbralGatillo {
		s.actual[int(TeclasDeLosGatillos[1])] = true
	}

	// 3) cuanto lleva apretada cada tecla, en cuadros
	for k := 0; k < teclasQueSeSiguen; k++ {
		switch {
		case s.actual[k] && s.antes[k]:
			s.cuadros[k]++
		case s.actual[k]:
			s.cuadros[k] = 1
		default:
			s.cuadros[k] = 0
		}
	}

	// 4) el cursor
	s.moverCursor()
}

// moverCursor: la palanca (analogica) y la cruceta (digital) mueven el cursor, y el cursor se queda adentro de
// la pantalla. Si se fuera, el motor seguiria recibiendo una posicion valida pero el jugador perderia de vista
// donde apunta -- y con un control no hay forma de "volver a poner el mouse".
func (s *Servicio) moverCursor() {
	movX, movY := 0, 0

	if s.palancaX > zonaMuertaPalanca || s.palancaX < -zonaMuertaPalanca {
		movX += int(s.palancaX) * velocidadPalanca / 128
	}

	if s.palancaY > zonaMuertaPalanca || s.palancaY < -zonaMuertaPalanca {
		movY += int(s.palancaY) * velocidadPalanca / 128
	}

	if s.botones&BitLeft != 0 {
		movX -= velocidadCruceta
	}

	if s.botones&BitRight != 0 {
		movX += velocidadCruceta
	}

	if s.botones&BitUp != 0 {
		movY -= velocidadCruceta
	}

	if s.botones&BitDown != 0 {
		movY += velocidadCruceta
	}

	s.cursorX += movX
	s.cursorY += movY

	if s.cursorX < 0 {
		s.cursorX = 0
	}

	if s.cursorX >= s.ancho {
		s.cursorX = s.ancho - 1
	}

	if s.cursorY < 0 {
		s.cursorY = 0
	}

	if s.cursorY >= s.alto {
		s.cursorY = s.alto - 1
	}
}

// indiceDeTecla: el indice en los arrays de estado, o -1 si la tecla esta fuera del rango que se sigue (una
// tecla que este backend no produce nunca). Devolver -1 y no 0 es a proposito: el indice 0 es Key0, y un
// "no la sigo" que se confunda con Key0 haria que preguntar por una tecla rara contestara por el 0.
func indiceDeTecla(k d2enum.Key) int {
	i := int(k)
	if i < 0 || i >= teclasQueSeSiguen {
		return -1
	}

	return i
}

// indiceDeMouse: el indice del boton del mouse, o -1 si no es uno de los tres.
func indiceDeMouse(b d2enum.MouseButton) int {
	i := int(b)
	if i < 0 || i >= botonesDelMouse {
		return -1
	}

	return i
}

// ============================ 4. LOS NUEVE METODOS DEL CONTRATO ============================

// CursorPosition: donde esta el cursor virtual (lo mueven la palanca y la cruceta). Es la posicion donde el
// motor cree que esta el mouse.
func (s *Servicio) CursorPosition() (x int, y int) { return s.cursorX, s.cursorY }

// InputChars: los caracteres imprimibles del cuadro. En la consola no hay teclado: NO HAY caracteres, y se
// devuelve nil en vez de una lista vacia inventada (declarado en doc.go).
func (s *Servicio) InputChars() []rune { return nil }

// IsKeyPressed: si la tecla esta apretada ahora.
func (s *Servicio) IsKeyPressed(key d2enum.Key) bool {
	if i := indiceDeTecla(key); i >= 0 {
		return s.actual[i]
	}

	return false
}

// IsKeyJustPressed: si la tecla paso de suelta a apretada EN ESTE cuadro (el flanco). Es lo que mueve los
// menus: dejar el boton apretado no repite la accion.
func (s *Servicio) IsKeyJustPressed(key d2enum.Key) bool {
	if i := indiceDeTecla(key); i >= 0 {
		return s.actual[i] && !s.antes[i]
	}

	return false
}

// IsKeyJustReleased: si la tecla paso de apretada a suelta en este cuadro.
func (s *Servicio) IsKeyJustReleased(key d2enum.Key) bool {
	if i := indiceDeTecla(key); i >= 0 {
		return !s.actual[i] && s.antes[i]
	}

	return false
}

// IsMouseButtonPressed: si el boton del mouse esta apretado ahora (A = izquierdo, B = derecho, X = medio).
func (s *Servicio) IsMouseButtonPressed(button d2enum.MouseButton) bool {
	if i := indiceDeMouse(button); i >= 0 {
		return s.mouse[i]
	}

	return false
}

// IsMouseButtonJustPressed: el clic, en el cuadro que se aprieta.
func (s *Servicio) IsMouseButtonJustPressed(button d2enum.MouseButton) bool {
	if i := indiceDeMouse(button); i >= 0 {
		return s.mouse[i] && !s.mouseAntes[i]
	}

	return false
}

// IsMouseButtonJustReleased: el boton que se suelta en este cuadro.
func (s *Servicio) IsMouseButtonJustReleased(button d2enum.MouseButton) bool {
	if i := indiceDeMouse(button); i >= 0 {
		return !s.mouse[i] && s.mouseAntes[i]
	}

	return false
}

// KeyPressDuration: cuantos cuadros lleva apretada la tecla (0 si esta suelta).
func (s *Servicio) KeyPressDuration(key d2enum.Key) int {
	if i := indiceDeTecla(key); i >= 0 {
		return s.cuadros[i]
	}

	return 0
}

// ============================ 5. LO QUE LEE LA EVIDENCIA ============================

// HayLectura: si alguna vez se leyo el aparato. Un servicio recien creado contesta que no a todo y no mueve
// el cursor: "todavia no hay control" no es lo mismo que "el control esta quieto".
func (s *Servicio) HayLectura() bool { return s.leido }

// Botones: los bits del control, crudos (los de KOS). Es el numero que se compara entre dos cuadros.
func (s *Servicio) Botones() uint32 { return s.botones }

// Palanca: los dos ejes de la palanca, crudos (-128..127).
func (s *Servicio) Palanca() (x int32, y int32) { return s.palancaX, s.palancaY }

// Gatillos: los dos gatillos analogos, crudos (0..255).
func (s *Servicio) Gatillos() (l int32, r int32) { return s.gatilloL, s.gatilloR }

// BotonesApretados: los nombres de los botones apretados ahora, en el orden de la tabla ("ARRIBA", "A", ...).
func (s *Servicio) BotonesApretados() []string {
	r := make([]string, 0, len(Tabla))

	for i := range Tabla {
		if s.botones&Tabla[i].Bit != 0 {
			r = append(r, Tabla[i].Boton)
		}
	}

	if s.gatilloL > UmbralGatillo {
		r = append(r, "GATILLO-L")
	}

	if s.gatilloR > UmbralGatillo {
		r = append(r, "GATILLO-R")
	}

	return r
}

// TeclasApretadas: las teclas apretadas ahora, SIN REPETIR (Enter sale del A y del Start: apretar los dos no
// son dos Enters, es una tecla apretada dos veces).
func (s *Servicio) TeclasApretadas() []d2enum.Key {
	r := make([]d2enum.Key, 0, len(Tabla))

	for k := 0; k < teclasQueSeSiguen; k++ {
		if s.actual[k] {
			r = append(r, d2enum.Key(k))
		}
	}

	return r
}

// TeclasRecienApretadas: las teclas cuyo flanco de bajada cae en este cuadro (sin repetir). Es lo que se
// imprime como "FLANCO" en la prueba.
func (s *Servicio) TeclasRecienApretadas() []d2enum.Key {
	r := make([]d2enum.Key, 0, len(Tabla))

	for k := 0; k < teclasQueSeSiguen; k++ {
		if s.actual[k] && !s.antes[k] {
			r = append(r, d2enum.Key(k))
		}
	}

	return r
}

// TeclasRecienSoltadas: las teclas cuyo flanco de subida cae en este cuadro (sin repetir).
func (s *Servicio) TeclasRecienSoltadas() []d2enum.Key {
	r := make([]d2enum.Key, 0, len(Tabla))

	for k := 0; k < teclasQueSeSiguen; k++ {
		if !s.actual[k] && s.antes[k] {
			r = append(r, d2enum.Key(k))
		}
	}

	return r
}

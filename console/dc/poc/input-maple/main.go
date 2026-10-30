//go:build dreamcast

// DC-002.71 -- EL CONTROL DE LA DREAMCAST POR d2interface.InputService (Fase C, la mitad de entrada).
//
// QUE PRUEBA, Y POR QUE ASI. El POC del personaje (console/dc/poc/personaje-mapa, DC-002.38) lee el control
// por el shim de Ebitengine (ebiten.IsKeyPressed(KeyLeft) y compañia) y ademas imprime los BITS CRUDOS del
// maple para saber que boton esta apretado. Esta tarjeta pone esa lectura DETRAS DE LA INTERFAZ DEL MOTOR: el
// POC ya no pregunta por Ebitengine ni por el maple, pregunta a d2interface.InputService, que es lo que le
// pregunta el motor de verdad (d2game, d2player, los menus).
//
//	servicio := maple.NuevoServicio(640, 480)        <- d2core/d2input/maple (el backend de la consola)
//	var entrada d2interface.InputService = servicio  <- el contrato: 9 metodos
//	cada cuadro: servicio.ActualizarDesdeElControl()
//	de ahi en adelante: entrada.IsKeyPressed, entrada.CursorPosition, entrada.IsMouseButtonJustPressed...
//
// QUE SE MIDE, y sin ojos:
//   - por la CONSOLA SERIE, cada vez que el estado del control CAMBIA: los bits crudos del control (el numero
//     de KOS), la palanca, los gatillos, la posicion del cursor y los nombres de lo que esta apretado; y una
//     linea por tecla con su codigo de d2enum.Key y cuantos cuadros lleva apretada. Los flancos (bajada y
//     subida) salen con su propia marca, que es lo que mueve los menus.
//   - en la PANTALLA, el cursor del motor como un recuadro amarillo y arriba el estado traducido, para que la
//     captura de Flycast muestre lo mismo que dice el log (una captura y un log que se contradicen no prueban
//     nada).
//
// EL DIBUJADO ES EL DE LA INTERFAZ, igual que en DC-002.70: el Renderer (d2core/d2render/pvr) arranca el
// video, llama actualizar y despues dibujar, y sube el cuadro al PVR.
//
// LO QUE NO ENTRA: el motor en el medio (d2gamescreen es la Fase D) y el audio (es la tarjeta siguiente).
package main

import (
	"image/color"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2enum"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2interface"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2input/maple"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2render/pvr"
)

//extern d2c_serial_init
func d2cSerialInit()

// init: la consola serie es la UNICA salida de texto en la Dreamcast (sin esto KOS deja el handler en NULL y
// el println de Go no sale a ninguna parte).
func init() { d2cSerialInit() }

const (
	ancho = 640
	alto  = 480

	// latido: aunque no se aprete nada, cada tanto se imprime el estado. Sirve para saber que el loop corre
	// (si el POC se colgara, el log se corta y se nota).
	latido = 120

	// la linea de texto de la pantalla y el alto de renglon de la fuente 3x5 del backend
	renglon = 9
)

var (
	// EL SERVICIO: la unica forma en que este POC sabe del control
	servicio = maple.NuevoServicio(ancho, alto)

	// y el contrato: si el servicio no cumpliera los 9 metodos, esto no compila. Es el criterio de la tarjeta
	// cumplido por el compilador y no por una lectura.
	entrada d2interface.InputService = servicio

	cuadros      int
	firmaAntes   uint32
	vistoControl bool

	// lo que pinta el cuadro: lo llena actualizar y lo lee dibujar (son los dos callbacks del Renderer)
	vistaBotones string
	vistaTeclas  string
	vistaCursorX int
	vistaCursorY int
)

// firma: un numero que cambia cuando cambia algo que al POC le importa (los bits del control y los dos
// gatillos, que son analogos y no estan en los bits). Sin esto, mover un gatillo sin tocar un boton no se
// imprimiria nunca.
func firma(botones uint32, gatilloL, gatilloR int32) uint32 {
	f := botones

	if gatilloL > maple.UmbralGatillo {
		f |= 1 << 20
	}

	if gatilloR > maple.UmbralGatillo {
		f |= 1 << 21
	}

	return f
}

// juntar: los nombres en una sola cadena con "+" entre ellos ("A+DERECHA"), que es lo que se lee comodo en un
// log y no necesita fmt (la stdlib de la consola tiene lo justo).
func juntar(nombres []string) string {
	if len(nombres) == 0 {
		return "-"
	}

	r := nombres[0]

	for _, n := range nombres[1:] {
		r += "+" + n
	}

	return r
}

func main() {
	println("HITO271: ARRANCA DC-002.71 -- el control de la Dreamcast por d2interface.InputService")
	println("HITO271: SERVICIO pantalla=", ancho, alto, "filasDeLaTabla=", len(maple.Tabla),
		"umbralGatillo=", maple.UmbralGatillo)

	// LA TABLA, tal como la va a usar el servicio: se imprime desde el codigo, no se copia a mano a la
	// evidencia (una tabla escrita dos veces se desincroniza sola).
	for i := range maple.Tabla {
		e := &maple.Tabla[i]
		nombres := make([]string, 0, len(e.Teclas))

		for _, k := range e.Teclas {
			nombres = append(nombres, maple.NombreDeTecla(k))
		}

		mouse := "-"

		if e.EsDelMouse {
			mouse = maple.NombreDelBotonDelMouse(e.Mouse)
		}

		println("HITO271: MAPEO bit=", int(e.Bit), "boton=", e.Boton,
			"teclas=", juntar(nombres), "mouse=", mouse)
	}

	println("HITO271: MAPEO-GATILLOS L=", maple.NombreDeTecla(maple.TeclasDeLosGatillos[0]),
		"R=", maple.NombreDeTecla(maple.TeclasDeLosGatillos[1]), "umbral=", maple.UmbralGatillo)

	// el ciclo de cuadros del backend de render: arranca el video y no vuelve
	renderer := pvr.NuevoRenderer()
	if err := renderer.Run(dibujar, actualizar, ancho, alto, "Diablo II (Dreamcast) -- maple"); err != nil {
		println("HITO271: el ciclo de cuadros salio con error:", err.Error())
	}
}

// actualizar: un cuadro. Se lee el control y se traduce A TRAVES DE LA INTERFAZ; lo que se imprime es lo que
// contesta la interfaz, no lo que leyo el maple (si el servicio tradujera mal, el log lo mostraria).
func actualizar() error {
	cuadros++

	// la marca que espera el que corre el emulador (run-flycast-271.sh): antes de esto la pantalla todavia no
	// tiene el cuadro del POC y las teclas se perderian en el arranque
	if cuadros == 1 {
		println("HITO271: PRIMER-CUADRO cuadro=1")
	}

	if !servicio.ActualizarDesdeElControl() {
		if cuadros == 1 || cuadros%latido == 0 {
			if maple.HayControl() {
				println("HITO271: SIN-LECTURA cuadro=", cuadros, "el control esta en el bus pero no contesto")
			} else {
				println("HITO271: SIN-CONTROL cuadro=", cuadros, "no hay control en el puerto 0")
			}
		}

		actualizarVista()

		return nil
	}

	if !vistoControl {
		vistoControl = true
		println("HITO271: CONTROL-ENCONTRADO cuadro=", cuadros)
	}

	botones := servicio.Botones()
	gatilloL, gatilloR := servicio.Gatillos()
	f := firma(botones, gatilloL, gatilloR)
	cursorX, cursorY := entrada.CursorPosition()

	if f != firmaAntes || cuadros%latido == 0 {
		palancaX, palancaY := servicio.Palanca()
		println("HITO271: CONTROL cuadro=", cuadros, "botones=", int(botones),
			"apretados=", juntar(servicio.BotonesApretados()),
			"palanca=", int(palancaX), int(palancaY),
			"gatillos=", int(gatilloL), int(gatilloR),
			"cursor=", cursorX, cursorY)
	}

	// ---- lo que contesta la INTERFAZ: las teclas y sus flancos
	if f != firmaAntes {
		for _, k := range servicio.TeclasApretadas() {
			println("HITO271: TECLA apretada=", maple.NombreDeTecla(k), "codigo=", int(k),
				"duracion=", entrada.KeyPressDuration(k))
		}

		for _, k := range servicio.TeclasRecienApretadas() {
			println("HITO271: FLANCO bajada=", maple.NombreDeTecla(k), "codigo=", int(k), "cuadro=", cuadros)
		}

		for _, k := range servicio.TeclasRecienSoltadas() {
			println("HITO271: FLANCO subida=", maple.NombreDeTecla(k), "codigo=", int(k), "cuadro=", cuadros)
		}

		for _, b := range []d2enum.MouseButton{
			d2enum.MouseButtonLeft, d2enum.MouseButtonMiddle, d2enum.MouseButtonRight,
		} {
			if entrada.IsMouseButtonJustPressed(b) {
				println("HITO271: MOUSE bajada=", maple.NombreDelBotonDelMouse(b), "cuadro=", cuadros)
			}

			if entrada.IsMouseButtonJustReleased(b) {
				println("HITO271: MOUSE subida=", maple.NombreDelBotonDelMouse(b), "cuadro=", cuadros)
			}
		}
	}

	firmaAntes = f

	actualizarVista()

	return nil
}

// actualizarVista: lo que va a la pantalla. Se arma aca (el cuadro que se dibuja) y no en dibujar, porque
// dibujar corre despues y no tiene por que volver a preguntarle al servicio.
func actualizarVista() {
	vistaBotones = juntar(servicio.BotonesApretados())

	teclas := servicio.TeclasApretadas()
	nombres := make([]string, 0, len(teclas))

	for _, k := range teclas {
		nombres = append(nombres, maple.NombreDeTecla(k))
	}

	vistaTeclas = juntar(nombres)
	vistaCursorX, vistaCursorY = servicio.CursorPosition()
}

// dibujar: compone el cuadro en la Surface del backend. El recuadro amarillo ES el cursor del motor (la
// posicion que contesta CursorPosition): si la cruceta o la palanca lo mueven, se ve en la captura.
func dibujar(superficie d2interface.Surface) error {
	superficie.Clear(color.RGBA{R: 0, G: 0, B: 0, A: 255})

	texto(superficie, 0, "DC271 MAPLE / D2INTERFACE.INPUTSERVICE")
	texto(superficie, 1, "BOTONES: "+vistaBotones)
	texto(superficie, 2, "TECLAS: "+vistaTeclas)
	texto(superficie, 4, "EL RECUADRO AMARILLO ES EL CURSOR DEL MOTOR")

	// el cursor: un recuadro de 10x10 centrado en la posicion que contesta la interfaz
	superficie.PushTranslation(vistaCursorX-5, vistaCursorY-5)
	superficie.DrawRect(10, 10, color.RGBA{R: 255, G: 255, B: 0, A: 255})
	superficie.Pop()

	return nil
}

// texto: una linea de texto en la esquina de arriba (la fuente 3x5 del backend es de MAYUSCULAS y sin acentos:
// los caracteres que no estan en su tabla se saltean).
func texto(superficie d2interface.Surface, linea int, t string) {
	superficie.PushTranslation(8, 8+linea*renglon)
	superficie.DrawTextf("%s", t)
	superficie.Pop()
}

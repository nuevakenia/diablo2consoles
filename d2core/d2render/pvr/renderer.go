//go:build dreamcast

package pvr

// EL RENDERER DE LA DREAMCAST (DC-002.70): la implementacion de d2interface.Renderer sobre el PVR.
//
// QUE ES. d2common/d2interface/renderer.go declara 15 metodos: el ciclo de cuadros (Run), las superficies
// (CreateSurface/NewSurface), el estado de ventana (fullscreen/vsync/cursor), el reloj (CurrentFPS), el
// cartel de panico (ShowPanicScreen) y el texto de depuracion (Print/PrintAt). Hasta esta tarjeta la unica
// implementacion era d2core/d2render/ebiten (PC). Esta es la de la consola.
//
// COMO ES UN CUADRO (el orden importa, y es el que estaba medido en el shim de DC-002.9/22):
//
//	PvrWaitReady      el PVR dio el cuadro anterior (es la sincronizacion con la pantalla)
//	PvrSceneBegin     arranca la escena
//	PvrListBegin(OP)  la lista OPACA: el cuadro compuesto, que tapa todo
//	actualizar()      el juego avanza su estado
//	dibujar(malla)    el juego compone el cuadro en el lienzo de la Surface
//	Presentar+Pintar  el lienzo a la VRAM (RGB565 por bloques de 32x32) y el PVR lo pinta
//	PvrListFinish / PvrSceneFinish
//
// LA PANTALLA VIVE EN LA VRAM, y esto NO es una optimizacion: es lo que hace que el cuadro entre. El
// lienzo 640x480x4 son 1.228.800 B y el monton de C del binario (despues del recorte de DC-002.68) deja un
// techo de ~1,23 MB para UN bloque: con el mapa del nivel (512x512 = 262.144 B) vivo al mismo tiempo no
// entra. La palanca es la de DC-002.22, ya medida: `d2c_pvr_pantalla_fb_init` reserva w*h*4 en la VRAM (8
// MB, de los que el PVR usa ~1 MB para la textura del cuadro) y el slice de pixeles apunta ahi. Si la VRAM
// no diera, el Renderer cae al monton y lo DICE por la consola serie: el que mide sabe cual de los dos
// caminos tomo.
//
// LO QUE NO PUEDE SER FIEL, DECLARADO (no escondido):
//   - SetFullScreen/IsFullScreen: en la consola el modo de video es uno (640x480); el pedido se guarda y el
//     valor devuelto es ese. No hay ventana que agrandar.
//   - vsync: el PVR ya entrega los cuadros a la frecuencia del video (PvrWaitReady), asi que
//     SetVSyncEnabled(false) NO puede desincronizar el chip: el pedido se guarda y se informa.
//   - SetWindowIcon: no hay ventana ni icono.
//   - GetCursorPos: no hay mouse; devuelve (0,0) y lo declara. El control de la Dreamcast no es un cursor.
//   - Print/PrintAt: la fuente es la 3x5 PROVISIONAL de este backend (fuente.go). Con un target que no sea
//     una Surface de este paquete, el texto va a la consola serie.
//   - CurrentFPS se mide con el reloj del PVR (d2c_pvr_pantalla_us), no con contadores del SH-4: los
//     PerfCntr devuelven 0 en Flycast (medido en DC-002.49).

import (
	"image/color"

	"kos"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2interface"
)

// Los alias son los de d2common/d2interface (ahi son alias, no tipos con nombre: la firma tiene que
// coincidir exactamente o el metodo no implementa el contrato).
type renderCallback = func(d2interface.Surface) error

type updateCallback = func() error

// Renderer es el ciclo de cuadros de la consola: la pantalla del motor en la VRAM del PVR.
type Renderer struct {
	dibujar    renderCallback
	actualizar updateCallback

	superficie *Surface

	ancho, alto int
	titulo      string

	// vsync: lo que pidio el motor (el PVR entrega a la frecuencia del video igual; ver la cabecera).
	vsync      bool
	pantallaSi bool // el video arranco (fullscreen es el unico estado que hay)

	// fps: el promedio del ultimo segundo medido, no el de un cuadro suelto.
	fps        float64
	cuadros    int
	msAntes    uint64
	cuadrosVen int

	ultimoError error
}

// que el contrato se cumpla no es una suposicion: si d2interface.Renderer cambia, esto no compila.
var _ d2interface.Renderer = (*Renderer)(nil)

// NuevoRenderer: el renderer de la consola, sin video todavia (el video lo arranca Run).
func NuevoRenderer() *Renderer {
	return &Renderer{
		vsync:  true, // el PVR entrega los cuadros con el video: por omision, sincronizado
		ancho:  anchoDelCuadro,
		alto:   altoDelCuadro,
		titulo: "OpenDiablo2",
	}
}

const (
	anchoDelCuadro = 640
	altoDelCuadro  = 480
)

// GetRendererName: el nombre que sale en la configuracion y en los logs del motor.
func (r *Renderer) GetRendererName() string { return "PVR (Dreamcast)" }

// SetWindowIcon: no hay ventana en la consola. Se declara como no-op (ver la cabecera).
func (r *Renderer) SetWindowIcon(fileName string) {}

// Run arranca el video y corre el ciclo de cuadros: por cada cuadro actualiza, dibuja, y el cuadro se lo
// lleva el PVR. NO VUELVE: en la consola no hay evento de cierre (el jugador apaga la consola), igual que
// en el bucle del shim de DC-002.9.
func (r *Renderer) Run(dibujar renderCallback, actualizar updateCallback, ancho, alto int, titulo string) error {
	if ancho > 0 {
		r.ancho = ancho
	}

	if alto > 0 {
		r.alto = alto
	}

	r.titulo = titulo
	r.dibujar = dibujar
	r.actualizar = actualizar

	if err := PantallaIniciar(r.ancho, r.alto); err != nil {
		return err
	}

	r.pantallaSi = true

	// LA SUPERFICIE DEL CUADRO, con los pixeles en la VRAM si da (ver la cabecera). Si no da, cae al
	// monton y lo dice: el numero que lo decide es el que se imprime.
	r.superficie = SuperficieDeLaPantalla(r.ancho, r.alto)
	r.superficie.renderer = r

	kos.PvrSetBgColor(0, 0, 0)

	r.msAntes = uint64(d2cPvrUs())

	for {
		// el cuadro lo marca el PVR: sin esto se dibujaria encima del que se esta mostrando
		kos.PvrWaitReady()
		kos.PvrSceneBegin()

		// la lista OPACA: el cuadro compuesto tapa todo, no hay nada detras
		kos.PvrListBegin(kos.PVR_LIST_OP_POLY)

		if r.actualizar != nil {
			if err := r.actualizar(); err != nil {
				kos.PvrListFinish()
				kos.PvrSceneFinish()

				return err
			}
		}

		r.ultimoError = nil

		if r.dibujar != nil {
			if err := r.dibujar(r.superficie); err != nil {
				r.ultimoError = err

				kos.PvrListFinish()
				kos.PvrSceneFinish()

				return err
			}
		}

		if err := PresentarEnPantalla(r.superficie.Imagen()); err != nil {
			r.ultimoError = err

			kos.PvrListFinish()
			kos.PvrSceneFinish()

			return err
		}

		kos.PvrListFinish()
		kos.PvrSceneFinish()

		r.cuadros++
		r.medirFPS()
	}
}

// medirFPS: los cuadros por segundo del ultimo segundo medido (ventana de 1 s, no un cuadro suelto).
func (r *Renderer) medirFPS() {
	ahora := uint64(d2cPvrUs())
	r.cuadrosVen++

	transcurrido := ahora - r.msAntes
	if transcurrido < 1000000 {
		return
	}

	r.fps = float64(r.cuadrosVen) * 1000000.0 / float64(transcurrido)
	r.cuadrosVen = 0
	r.msAntes = ahora
}

// IsDrawingSkipped: si el ultimo cuadro no se pudo componer (el motor lo usa para no contar un cuadro
// como bueno).
func (r *Renderer) IsDrawingSkipped() bool { return r.ultimoError != nil }

// CreateSurface: una superficie NUEVA sobre el MISMO lienzo de la que se le pasa, con el estado de dibujado
// neutro. Es la semantica del backend de la PC (d2core/d2render/ebiten/ebiten_renderer.go:209).
func (r *Renderer) CreateSurface(surface d2interface.Surface) (d2interface.Surface, error) {
	origen, ok := surface.(*Surface)
	if !ok || origen == nil {
		return nil, errorPantalla("CreateSurface: la superficie no es de este backend")
	}

	otra := SurfaceDesdeImagen(origen.imagen)
	otra.renderer = r

	return otra, nil
}

// NewSurface: una superficie nueva del tamano pedido, en el monton (las superficies chicas -- un sprite
// compuesto, un panel -- no compiten con la pantalla por el techo de la VRAM).
func (r *Renderer) NewSurface(ancho, alto int) d2interface.Surface {
	s := NuevaSurface(ancho, alto)
	s.renderer = r

	return s
}

// IsFullScreen: en la consola el video siempre ocupa la pantalla entera.
func (r *Renderer) IsFullScreen() bool { return true }

// SetFullScreen: se guarda lo pedido y se informa por la serie; no hay ventana que agrandar (ver cabecera).
func (r *Renderer) SetFullScreen(fullScreen bool) {
	if !fullScreen {
		println("PVR-RENDERER: SetFullScreen(false) no se puede cumplir en la consola; el video es uno solo")
	}
}

// SetVSyncEnabled: el PVR entrega los cuadros con el video (PvrWaitReady); el pedido se guarda.
func (r *Renderer) SetVSyncEnabled(vsync bool) { r.vsync = vsync }

// GetVSyncEnabled: lo ultimo que se pidio.
func (r *Renderer) GetVSyncEnabled() bool { return r.vsync }

// GetCursorPos: no hay mouse en la consola. Devuelve (0,0) y lo declara.
func (r *Renderer) GetCursorPos() (int, int) { return 0, 0 }

// CurrentFPS: los cuadros por segundo medidos con el reloj del PVR (ver la cabecera).
func (r *Renderer) CurrentFPS() float64 { return r.fps }

// ShowPanicScreen: el cartel de panico. Lo escribe en la consola serie y ademas lo pinta en la pantalla,
// que queda ahi para siempre: en la consola no hay a quien preguntarle nada, y una excepcion del SH-4 sin
// este cartel se ve como "la consola se reinicio".
func (r *Renderer) ShowPanicScreen(message string) {
	println("PVR-RENDERER PANICO:", message)

	// la pantalla, si el video no habia arrancado (un panico antes de Run)
	if r.superficie == nil {
		if err := PantallaIniciar(r.ancho, r.alto); err != nil {
			return
		}

		r.superficie = SuperficieDeLaPantalla(r.ancho, r.alto)
		r.superficie.renderer = r
	}

	r.superficie.Clear(color.RGBA{R: 40, G: 0, B: 0, A: 255})
	r.superficie.PushTranslation(16, 16)
	r.superficie.DrawTextf("PANICO (DC-002.70)")
	r.superficie.Pop()
	r.superficie.PushTranslation(16, 40)
	r.superficie.DrawTextf("%s", message)
	r.superficie.Pop()

	for {
		if err := PresentarEnPantalla(r.superficie.Imagen()); err != nil {
			return
		}

		kos.PvrWaitReady()
		kos.PvrSceneBegin()
		kos.PvrListBegin(kos.PVR_LIST_OP_POLY)
		PintarPantalla()
		kos.PvrListFinish()
		kos.PvrSceneFinish()
	}
}

// Print: el texto de depuracion, sin posicion: arriba a la izquierda del target (asi lo hace la PC).
func (r *Renderer) Print(target interface{}, str string) error {
	r.PrintAt(target, str, 0, 0)

	return nil
}

// PrintAt: el texto de depuracion en (x, y). Si el target es una Surface de este backend, se dibuja ahi
// (fuente 3x5 PROVISIONAL); si no, sale por la consola serie, que es la unica otra salida de texto que hay.
func (r *Renderer) PrintAt(target interface{}, str string, x, y int) {
	if s, ok := target.(*Surface); ok && s != nil {
		s.PushTranslation(x, y)
		s.DrawTextf("%s", str)
		s.Pop()

		return
	}

	println("PVR-RENDERER:", str)
}

// ============================================================ lo que no es del contrato

// Surface: la superficie del cuadro (la que se le pasa al callback de dibujado). Es para la evidencia y
// para el que quiera medir el cuadro sin pasar por el callback.
func (r *Renderer) Surface() *Surface { return r.superficie }

// Cuadros: cuantos cuadros se presentaron (el instrumento del ciclo).
func (r *Renderer) Cuadros() int { return r.cuadros }

// PantallaEnVram: cuanto de la pantalla vive en la VRAM (0 = el lienzo cayo al monton).
func (r *Renderer) PantallaEnVram() int { return VramDeLaPantalla() }

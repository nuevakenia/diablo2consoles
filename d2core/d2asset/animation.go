package d2asset

import (
	"errors"
	"image"
	"image/color"
	"log"
	"math"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2enum"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2dcc"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2interface"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2math"
)

type playMode int

const (
	playModePause playMode = iota
	playModeForward
	playModeBackward
)

const defaultPlayLength = 1.0

type animationFrame struct {
	decoded bool

	width   int
	height  int
	offsetX int
	offsetY int

	image d2interface.Surface
}

type animationDirection struct {
	decoded bool
	// surfaced is set once the frame images of this direction exist. DCC animations create
	// them lazily, one direction at a time (see ensureSurfaces).
	surfaced bool
	frames   []animationFrame
}

// static check that we implement the animation interface
var _ d2interface.Animation = &Animation{}

// Animation has directionality, play modes, and frame counting
type Animation struct {
	renderer         d2interface.Renderer
	onBindRenderer   func(renderer d2interface.Renderer) error
	ensureDirection  func(direction int) error        // creates the images of one direction (DCC only)
	ensureFrame      func(direction, frame int) error // creates the image of ONE frame (console policy, DCC only)
	directions       []animationDirection
	effect           d2enum.DrawEffect
	colorMod         color.Color
	frameIndex       int
	directionIndex   int
	lastFrameTime    float64
	stepper          tickStepper // used instead of lastFrameTime when the speed came from SetPlaySpeed
	playedCount      int
	playMode         playMode
	playLength       float64 // https://github.com/OpenDiablo2/OpenDiablo2/issues/813
	subStartingFrame int
	subEndingFrame   int
	originAtBottom   bool
	playLoop         bool
	hasSubLoop       bool // runs after first animation ends
	hasShadow        bool

	// DC-002.159c: LA UNICA SUPERFICIE DEL CUADRO cuando el backend arma de a un cuadro (`ensureFrame`).
	//
	// Antes, la primera vez que se dibujaba una direccion se armaban las 8 (o 16) superficies de esa
	// direccion de una vez -- y con eso, un modo del heroe costaba 7 capas x 8 cuadros de RGBA. Con el
	// semiespacio de la consola en 1.835.008 B eso no entra ni de casualidad: medido en el Gate 0,
	// cambiar de direccion caminando disparaba cinco recolecciones seguidas (1.744.224 -> 1.376.288 B)
	// y a los dos segundos el SH-4 moria con `fatal error: stack_alloc: out of memory`.
	//
	// Con esto la animacion tiene UNA superficie viva y la vuelve a llenar con los pixeles del cuadro
	// que toca: cero pedidos por cuadro, cero basura por cuadro, y el vivo de una capa pasa de
	// ~8 x 18 KB a ~18 KB. El costo es expandir la paleta cuando el cuadro cambia (lo que antes se pagaba
	// al armar la superficie), no un pedido de memoria.
	superficieDelCuadro d2interface.Surface
	superficieAncho     int
	superficieAlto      int
	superficieDireccion int
	superficieCuadro    int
}

// SetSubLoop sets a sub loop for the animation
func (a *Animation) SetSubLoop(startFrame, endFrame int) {
	a.subStartingFrame = startFrame
	a.subEndingFrame = endFrame
	a.hasSubLoop = true
}

// Advance advances the animation state
func (a *Animation) Advance(elapsed float64) error {
	if a.playMode == playModePause {
		return nil
	}

	frameCount := a.GetFrameCount()
	if frameCount == 0 {
		return nil
	}

	var framesAdvanced int

	if a.stepper.rate > 0 {
		// animation speed known: step like the original (25 Hz ticks, 8.8 fixed point)
		framesAdvanced = a.stepper.advance(elapsed)
	} else {
		frameLength := a.playLength / float64(frameCount)
		a.lastFrameTime += elapsed
		framesAdvanced = int(a.lastFrameTime / frameLength)
		a.lastFrameTime -= float64(framesAdvanced) * frameLength
	}

	for i := 0; i < framesAdvanced; i++ {
		startIndex := 0
		endIndex := frameCount

		if a.hasSubLoop && a.playedCount > 0 {
			startIndex = a.subStartingFrame
			endIndex = a.subEndingFrame
		}

		switch a.playMode {
		case playModeForward:
			a.frameIndex++
			if a.frameIndex >= endIndex {
				a.playedCount++
				if a.playLoop {
					a.frameIndex = startIndex
				} else {
					a.frameIndex = endIndex - 1
					break
				}
			}
		case playModeBackward:
			a.frameIndex--
			if a.frameIndex < startIndex {
				a.playedCount++
				if a.playLoop {
					a.frameIndex = endIndex - 1
				} else {
					a.frameIndex = startIndex
					break
				}
			}
		}
	}

	return nil
}

const (
	full = 1.0
	half = 0.5
	zero = 0.0
)

// ensureSurfaces creates the images of the current direction if the animation builds them lazily.
// Creating all 8 or 16 directions of a monster mode at the first draw cost a visible hitch (hundreds of
// ms for a pack of monsters coming into view); only the direction that is actually drawn is needed.
//
// DC-002.159c: cuando el backend arma de a UN cuadro (`ensureFrame`, la politica de la consola) se le pide
// siempre el cuadro que se va a dibujar, y el es el que decide si hay que rellenar la superficie o ya esta.
// Es idempotente y baratito: si el cuadro ya esta cargado vuelve sin tocar nada.
func (a *Animation) ensureSurfaces() {
	if a.directionIndex >= len(a.directions) {
		return
	}

	if a.ensureFrame != nil {
		if err := a.ensureFrame(a.directionIndex, a.frameIndex); err != nil {
			log.Println(err)
		}

		return
	}

	if a.ensureDirection == nil || a.directions[a.directionIndex].surfaced {
		return
	}

	if err := a.ensureDirection(a.directionIndex); err != nil {
		log.Println(err)
	}
}

// RecuadroDelCuadro159: la caja que ocupa el cuadro actual de la animacion, en pixeles y RELATIVA AL ORIGEN
// de la animacion (el punto que traslada `Render`), o sea (izq, arriba, ancho, alto).
//
// PARA QUE (DC-002.159c). El que dibuja un ente encima de un mapa que no se repinta tiene que guardar el
// recuadro que el sprite tapa para poder borrarlo despues. Con una constante (160x160 en el Gate 0) el
// recuadro cubria el cuadro de parado y NO el de caminar -- las capas de las piernas se dibujan con su
// propio desplazamiento -- y quedaban piernas pegadas al lienzo. La caja la sabe la animacion: es el
// desplazamiento del cuadro (`offsetX`, `offsetY`) mas su tamano, con la correccion de las animaciones que
// se dibujan desde el pie (`originAtBottom`, los DC6).
//
// OJO CON LA SOMBRA: `RenderFromOrigin(.., shadow=true)` dibuja ademas la sombra del compuesto, que se corre
// a los costados (medio alto de sesgo). Esta caja NO la incluye; en la consola las sombras del compuesto
// estan apagadas (`UsarSombrasDelCompuesto`), y si algun dia se prenden, el recuadro tiene que agrandarse.
func (a *Animation) recuadroDelCuadro159() (izq, arriba, ancho, alto int) {
	if a.directionIndex >= len(a.directions) || a.frameIndex >= len(a.directions[a.directionIndex].frames) {
		return 0, 0, 0, 0
	}

	frame := a.directions[a.directionIndex].frames[a.frameIndex]

	izq, arriba = frame.offsetX, frame.offsetY
	if a.originAtBottom {
		arriba -= frame.height
	}

	return izq, arriba, frame.width, frame.height
}

// renderElCuadro159f dibuja el cuadro de la animacion en `target`, RECORTANDO si la superficie que lo guarda es
// mas grande que la caja del cuadro.
//
// POR QUE (DC-002.159f). La animacion reusa UNA superficie del tamano maximo por animacion
// (`DCCAnimation.armarLaSuperficieDelCuadro`): el cuadro ocupa su esquina de arriba a la izquierda y el resto son
// pixeles de otros cuadros. Dibujar la superficie entera dejaria un borde fantasma del tamano de la caja mas
// grande, asi que se dibuja el recorte.
//
// DOS CAMINOS, y los dos son parte del CONTRATO (`d2interface.Surface`):
//  1. la superficie mide exactamente lo que el cuadro (o menos): `Render`, como siempre (la PC, y la consola
//     mientras la caja sea la mayor vista);
//  2. la superficie es mas grande que el cuadro: `RenderRecorte`, que en la consola lee solo esa ventana sin
//     armar buffer (DC-002.159g).
//
// POR QUE YA NO HAY ASERCION (DC-002.159g, medido). En 159f el camino 2 se elegia con una interfaz opcional
// (`target.(superficieConRecorte159f)`) y en este gccgo del sh4 esa asercion NO ACIERTA aunque el metodo exista
// en el tipo: la consola contestaba `recorte=false` y se dibujaba la superficie entera (borde fantasma medido en
// las capturas). Con el metodo en el contrato la llamada es despacho normal de interfaz.
func renderElCuadro159f(target d2interface.Surface, frame animationFrame) {
	if frame.image == nil {
		target.Render(frame.image)

		return
	}

	ancho, alto := frame.image.GetSize()
	if ancho <= frame.width && alto <= frame.height {
		target.Render(frame.image)

		return
	}

	recortesDelCuadro159++

	target.RenderRecorte(frame.image, frame.width, frame.height)
}

func (a *Animation) renderShadow(target d2interface.Surface) {
	a.ensureSurfaces()

	direction := a.directions[a.directionIndex]
	frame := direction.frames[a.frameIndex]

	target.PushFilter(d2enum.FilterLinear)
	defer target.Pop()

	target.PushTranslation(frame.offsetX, int(float64(frame.offsetY)*half))
	defer target.Pop()

	target.PushScale(full, half)
	defer target.Pop()

	target.PushSkew(half, zero)
	defer target.Pop()

	target.PushEffect(d2enum.DrawEffectPctTransparency25)
	defer target.Pop()

	target.PushBrightness(zero)
	defer target.Pop()

	renderElCuadro159f(target, frame)
}

// GetCurrentFrameSurface returns the surface for the current frame of the
// animation
func (a *Animation) GetCurrentFrameSurface() d2interface.Surface {
	a.ensureSurfaces()

	return a.directions[a.directionIndex].frames[a.frameIndex].image
}

// Render renders the animation to the given surface
func (a *Animation) Render(target d2interface.Surface) {
	if a.renderer == nil {
		a.BindRenderer(target.Renderer())
	}

	a.ensureSurfaces()

	direction := a.directions[a.directionIndex]
	frame := direction.frames[a.frameIndex]

	target.PushTranslation(frame.offsetX, frame.offsetY)
	defer target.Pop()

	target.PushEffect(a.effect)
	defer target.Pop()

	target.PushColor(a.colorMod)
	defer target.Pop()

	renderElCuadro159f(target, frame)
}

// BindRenderer binds the given renderer to the animation so that it can initialize
// the required surfaces
func (a *Animation) BindRenderer(r d2interface.Renderer) {
	if a.onBindRenderer == nil {
		return
	}

	if err := a.onBindRenderer(r); err != nil {
		log.Println(err)
	}
}

// RenderFromOrigin renders the animation from the animation origin
func (a *Animation) RenderFromOrigin(target d2interface.Surface, shadow bool) {
	if a.renderer == nil {
		a.BindRenderer(target.Renderer())
	}

	if a.originAtBottom {
		direction := a.directions[a.directionIndex]
		frame := direction.frames[a.frameIndex]
		target.PushTranslation(0, -frame.height)

		defer target.Pop()
	}

	if shadow && !a.effect.Transparent() && a.hasShadow {
		_, height := a.GetFrameBounds()
		height = int(math.Abs(float64(height)))
		halfHeight := height / 2 //nolint:gomnd // this ain't rocket surgery...

		target.PushTranslation(-halfHeight, 0)
		defer target.Pop()

		a.renderShadow(target)

		return
	}

	a.Render(target)
}

// RenderSection renders the section of the animation frame enclosed by bounds
func (a *Animation) RenderSection(target d2interface.Surface, bound image.Rectangle) {
	if a.renderer == nil {
		a.BindRenderer(target.Renderer())
	}

	a.ensureSurfaces()

	direction := a.directions[a.directionIndex]
	frame := direction.frames[a.frameIndex]

	target.PushTranslation(frame.offsetX, frame.offsetY)
	defer target.Pop()

	target.PushEffect(a.effect)
	defer target.Pop()

	target.PushColor(a.colorMod)
	defer target.Pop()

	target.RenderSection(frame.image, bound)
}

// GetFrameSize gets the Size(width, height) of a indexed frame.
func (a *Animation) GetFrameSize(frameIndex int) (width, height int, err error) {
	direction := a.directions[a.directionIndex]
	if frameIndex >= len(direction.frames) {
		return 0, 0, errors.New("invalid frame index")
	}

	frame := direction.frames[frameIndex]

	return frame.width, frame.height, nil
}

// GetCurrentFrameSize gets the Size(width, height) of the current frame.
func (a *Animation) GetCurrentFrameSize() (width, height int) {
	width, height, err := a.GetFrameSize(a.frameIndex)
	if err != nil {
		log.Print(err)
	}

	return width, height
}

// GetFrameBounds gets maximum Size(width, height) of all frame.
func (a *Animation) GetFrameBounds() (maxWidth, maxHeight int) {
	maxWidth, maxHeight = 0, 0

	direction := a.directions[a.directionIndex]

	for _, frame := range direction.frames {
		maxWidth = d2math.MaxInt(maxWidth, frame.width)
		maxHeight = d2math.MaxInt(maxHeight, frame.height)
	}

	return maxWidth, maxHeight
}

// GetCurrentFrame gets index of current frame in animation
func (a *Animation) GetCurrentFrame() int {
	return a.frameIndex
}

// GetFrameCount gets number of frames in animation
func (a *Animation) GetFrameCount() int {
	direction := a.directions[a.directionIndex]
	return len(direction.frames)
}

// IsOnFirstFrame gets if the animation on its first frame
func (a *Animation) IsOnFirstFrame() bool {
	return a.frameIndex == 0
}

// IsOnLastFrame gets if the animation on its last frame
func (a *Animation) IsOnLastFrame() bool {
	return a.frameIndex == a.GetFrameCount()-1
}

// GetDirectionCount gets the number of animation direction
func (a *Animation) GetDirectionCount() int {
	return len(a.directions)
}

// SetDirection places the animation in the direction of an animation
func (a *Animation) SetDirection(directionIndex int) error {
	const smallestInvalidDirectionIndex = 64
	if directionIndex >= smallestInvalidDirectionIndex {
		return errors.New("invalid direction index")
	}

	a.directionIndex = d2dcc.Dir64ToDcc(directionIndex, len(a.directions))
	a.frameIndex = 0

	return nil
}

// GetDirection get the current animation direction
func (a *Animation) GetDirection() int {
	return a.directionIndex
}

// SetCurrentFrame sets animation at a specific frame
func (a *Animation) SetCurrentFrame(frameIndex int) error {
	if frameIndex >= a.GetFrameCount() {
		return errors.New("invalid frame index")
	}

	a.frameIndex = frameIndex
	a.lastFrameTime = 0

	return nil
}

// Rewind animation to beginning
func (a *Animation) Rewind() {
	err := a.SetCurrentFrame(0)
	if err != nil {
		log.Print(err)
	}
}

// PlayForward plays animation forward
func (a *Animation) PlayForward() {
	a.playMode = playModeForward
	a.lastFrameTime = 0
}

// PlayBackward plays animation backward
func (a *Animation) PlayBackward() {
	a.playMode = playModeBackward
	a.lastFrameTime = 0
}

// Pause animation
func (a *Animation) Pause() {
	a.playMode = playModePause
	a.lastFrameTime = 0
	a.stepper.reset()
}

// SetPlayLoop sets whether to loop the animation
func (a *Animation) SetPlayLoop(loop bool) {
	a.playLoop = loop
}

// SetPlaySpeed sets play speed of the animation
func (a *Animation) SetPlaySpeed(playSpeed float64) {
	a.SetPlayLength(playSpeed * float64(a.GetFrameCount()))
	a.stepper = tickStepper{rate: rateFromFrameSeconds(playSpeed)}
}

// SetPlayLength sets the Animation's play length in seconds
func (a *Animation) SetPlayLength(playLength float64) {
	// https://github.com/OpenDiablo2/OpenDiablo2/issues/813
	a.playLength = playLength
	a.lastFrameTime = 0
	a.stepper = tickStepper{}
}

// SetColorMod sets the Animation's color mod
func (a *Animation) SetColorMod(colorMod color.Color) {
	a.colorMod = colorMod
}

// GetPlayedCount gets the number of times the application played
func (a *Animation) GetPlayedCount() int {
	return a.playedCount
}

// ResetPlayedCount resets the play count
func (a *Animation) ResetPlayedCount() {
	a.playedCount = 0
}

// SetEffect sets the draw effect for the animation
func (a *Animation) SetEffect(e d2enum.DrawEffect) {
	a.effect = e
}

// SetShadow sets bool for whether or not to draw a shadow
func (a *Animation) SetShadow(shadow bool) {
	a.hasShadow = shadow
}

// Clone creates a copy of the Animation
func (a *Animation) Clone() d2interface.Animation {
	clone := *a
	copy(clone.directions, a.directions)

	return &clone
}

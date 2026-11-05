package d2asset

import (
	"errors"
	"fmt"
	"strings"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2enum"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2cof"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2interface"
)

const (
	hardcodedFPS     = 25.0
	hardcodedDivisor = 1.0 / 256.0
	speedUnit        = hardcodedFPS * hardcodedDivisor
)

// Composite is a composite entity animation
type Composite struct {
	*AssetManager
	baseType    d2enum.ObjectType
	basePath    string
	token       string
	palettePath string
	direction   int
	equipment   [d2enum.CompositeTypeMax]string
	// layerEffects overrides the draw effect of a layer (the equipment rule of the exe, 0x4d8090).
	layerEffects map[d2enum.CompositeType]d2enum.DrawEffect
	mode         *compositeMode
	size         *size
}

type size struct {
	Width  int
	Height int
}

// Advance moves the composite animation forward for a given elapsed time in nanoseconds.
func (c *Composite) Advance(elapsed float64) error {
	if c.mode == nil {
		return nil
	}

	c.mode.lastFrameTime += elapsed
	framesToAdd := int(c.mode.lastFrameTime / c.mode.animationSpeed)
	c.mode.lastFrameTime -= float64(framesToAdd) * c.mode.animationSpeed
	c.mode.frameIndex += framesToAdd
	c.mode.playedCount += c.mode.frameIndex / c.mode.frameCount
	c.mode.frameIndex %= c.mode.frameCount

	for _, layer := range c.mode.layers {
		if layer != nil {
			if err := layer.Advance(elapsed); err != nil {
				return err
			}
		}
	}

	return nil
}

// Render performs drawing of the Composite on the rendered d2interface.Surface.
func (c *Composite) Render(target d2interface.Surface) error {
	if c.mode == nil {
		return nil
	}

	direction := d2cof.Dir64ToCof(c.direction, c.mode.cof.NumberOfDirections)

	for _, layerIndex := range c.mode.cof.Priority[direction][c.mode.frameIndex] {
		layer := c.mode.layers[layerIndex]

		if SondaDeRender != nil {
			SondaDeRender(c.direction, c.mode.frameIndex, direction, layerIndex, c.mode.layers, layer, true)
		}

		if layer != nil {
			layer.RenderFromOrigin(target, true)
		}
	}

	for _, layerIndex := range c.mode.cof.Priority[direction][c.mode.frameIndex] {
		layer := c.mode.layers[layerIndex]

		if SondaDeRender != nil {
			SondaDeRender(c.direction, c.mode.frameIndex, direction, layerIndex, c.mode.layers, layer, false)
		}

		if layer != nil {
			layer.RenderFromOrigin(target, false)
		}
	}

	return nil
}

// ObjectAnimationMode returns the object animation mode
func (c *Composite) ObjectAnimationMode() d2enum.ObjectAnimationMode {
	return c.mode.animationMode.(d2enum.ObjectAnimationMode)
}

// GetAnimationMode returns the animation mode the Composite should render with.
func (c *Composite) GetAnimationMode() string {
	return c.mode.animationMode.String()
}

// GetCurrentFrame returns the frame index in the current animation mode.
func (c *Composite) GetCurrentFrame() int {
	return c.mode.frameIndex
}

// GetFrameCount returns the number of frames in the current animation mode.
func (c *Composite) GetFrameCount() int {
	return c.mode.frameCount
}

// GetWeaponClass returns the currently loaded weapon class
func (c *Composite) GetWeaponClass() string {
	return c.mode.weaponClass
}

// SetMode sets the Composite's animation mode weapon class and direction
func (c *Composite) SetMode(animationMode animationMode, weaponClass string) error {
	if c.mode != nil && c.mode.animationMode.String() == animationMode.String() && c.mode.weaponClass == weaponClass {
		return nil
	}

	mode, err := c.createMode(animationMode, weaponClass)
	if err != nil {
		return err
	}

	c.resetPlayedCount()
	c.mode = mode

	return nil
}

// SetLayerEffects sets the draw effect overrides of layers; it takes effect with the next Equip or mode change.
func (c *Composite) SetLayerEffects(e map[d2enum.CompositeType]d2enum.DrawEffect) { c.layerEffects = e }

// Equip changes the current layer configuration
func (c *Composite) Equip(equipment *[d2enum.CompositeTypeMax]string) error {
	c.equipment = *equipment
	if c.mode == nil {
		return nil
	}

	mode, err := c.createMode(c.mode.animationMode, c.mode.weaponClass)

	if err != nil {
		return err
	}

	c.mode = mode

	return nil
}

// SetAnimSpeed sets the speed at which the Composite's animation should advance through its frames
func (c *Composite) SetAnimSpeed(speed int) {
	c.mode.animationSpeed = 1.0 / (float64(speed) * speedUnit) //nolint:gomnd // taking inverse
	for layerIdx := range c.mode.layers {
		layer := c.mode.layers[layerIdx]
		if layer != nil {
			layer.SetPlaySpeed(c.mode.animationSpeed)
		}
	}
}

// SetDirection sets the direction of the composite and its layers
//
// DC-002.159c: EN SERIE, NO EN SIETE GOROUTINES. Antes cada capa se ponia en la direccion desde su propia
// goroutine (`go func` + `sync.WaitGroup`). En la PC eso apenas se nota, pero en la consola es el gasto mas
// caro del cuadro cuando el ente camina: cada giro son siete goroutines nuevas, con su pila pedida al
// monton de C (`sbrk`), y las pilas NO vuelven al semiespacio del recolector. Medido en el Gate 0 con las
// superficies ya arregladas (corrida `/tmp/cap159c-d1`): el vivo despues del recolector habia bajado a
// 1.023.336 B (de 1.606.416 B) y el caminar igual moria a los pocos cuadros con
// `Out of memory. Requested sbrk_base ... diff 65536` y `fatal error: stack_alloc: out of memory` -- o sea no
// era el semiespacio, era la pila de goroutines pidiendo `sbrk`. El SH-4 tiene UN nucleo: siete goroutines no
// adelantan nada, solo cuestan.
func (c *Composite) SetDirection(direction int) {
	if c.mode == nil {
		return
	}

	c.direction = direction
	girosDelCompuesto159++

	for layerIdx := range c.mode.layers {
		layer := c.mode.layers[layerIdx]

		if layer != nil {
			if err := layer.SetDirection(c.direction); err != nil {
				fmt.Printf("failed to set direction of layer: %d, err: %v\n", layerIdx, err)
			}
		}
	}
}

// girosDelCompuesto159: cuantas veces se puso en una direccion el compuesto desde que arranco el proceso. Es el
// instrumento de DC-002.159c: caminando, el motor recalcula la direccion en cada cuadro y este contador lo dice
// (ver `GirosDelCompuesto159`).
var girosDelCompuesto159 int

// GirosDelCompuesto159 devuelve cuantas veces se puso en una direccion el compuesto.
func GirosDelCompuesto159() int { return girosDelCompuesto159 }

// GetDirection returns the current direction the composite is facing
func (c *Composite) GetDirection() int {
	return c.direction
}

// GetPlayedCount returns the number of times the current animation mode has completed all its distinct frames
func (c *Composite) GetPlayedCount() int {
	if c.mode == nil {
		return 0
	}

	return c.mode.playedCount
}

// SetPlayLoop turns on or off animation looping
func (c *Composite) SetPlayLoop(loop bool) {
	for layerIdx := range c.mode.layers {
		layer := c.mode.layers[layerIdx]
		if layer != nil {
			layer.SetPlayLoop(loop)
		}
	}
}

// SetSubLoop sets a loop to be between the specified frame indices
func (c *Composite) SetSubLoop(startFrame, endFrame int) {
	for layerIdx := range c.mode.layers {
		layer := c.mode.layers[layerIdx]
		if layer != nil {
			layer.SetSubLoop(startFrame, endFrame)
		}
	}
}

// SetCurrentFrame sets the current frame index of the animation
func (c *Composite) SetCurrentFrame(frame int) {
	for layerIdx := range c.mode.layers {
		layer := c.mode.layers[layerIdx]
		if layer != nil {
			if err := layer.SetCurrentFrame(frame); err != nil {
				fmt.Printf("failed to set current frame of layer: %d, err: %v\n", layerIdx, err)
			}
		}
	}
}

func (c *Composite) resetPlayedCount() {
	if c.mode != nil {
		c.mode.playedCount = 0
	}
}

type animationMode interface {
	String() string
}

type compositeMode struct {
	cof           *d2cof.COF
	animationMode animationMode
	weaponClass   string
	playedCount   int

	layers []d2interface.Animation

	frameCount     int
	frameIndex     int
	animationSpeed float64
	lastFrameTime  float64
}

func (c *Composite) createMode(animationMode animationMode, weaponClass string) (*compositeMode, error) {
	defer noteModeLoad(func() string { return fmt.Sprintf("%s %s%s", c.token, animationMode, weaponClass) })()

	cofPath := fmt.Sprintf("%s/%s/COF/%s%s%s.COF", c.basePath, c.token, c.token, animationMode, weaponClass)
	if exists, err := c.FileExists(cofPath); !exists {
		return nil, fmt.Errorf("composite not found at path '%s': %v", cofPath, err)
	}

	cof, err := c.LoadCOF(cofPath)
	if err != nil {
		return nil, err
	}

	animationKey := strings.ToUpper(c.token + animationMode.String() + weaponClass)

	animationData := c.Records.Animation.Data.GetRecords(animationKey)
	if len(animationData) == 0 {
		return nil, errors.New("could not find Animation data")
	}

	mode := &compositeMode{
		cof:            cof,
		animationMode:  animationMode,
		weaponClass:    weaponClass,
		layers:         make([]d2interface.Animation, d2enum.CompositeTypeMax),
		frameCount:     animationData[0].FramesPerDirection(),
		animationSpeed: 1.0 / (float64(animationData[0].Speed()) * speedUnit), //nolint:gomnd // taking inverse
	}

	for _, cofLayer := range cof.CofLayers {
		layerValue := c.equipment[cofLayer.Type]
		if layerValue == "" {
			layerValue = "lit"
		}

		drawEffect := d2enum.DrawEffectNone

		if cofLayer.Transparent {
			drawEffect = cofLayer.DrawEffect
		}

		if eff, ok := c.layerEffects[cofLayer.Type]; ok {
			drawEffect = eff
		}

		layer, err := c.loadCompositeLayer(cofLayer.Type.String(), layerValue, animationMode.String(),
			cofLayer.WeaponClass.String(), c.palettePath, drawEffect)
		if err == nil {
			layer.SetPlaySpeed(mode.animationSpeed)
			layer.PlayForward()

			// DC-002.159b: la sombra del COF se puede apagar desde afuera (`UsarSombrasDelCompuesto`). En la
			// consola el camino de la sombra -- escala + sesgo + brillo cero sobre la Surface del PVR -- mata el
			// SH-4 en el primer cuadro del heroe (medido en el Gate 0: `Unhandled exception` adentro de
			// `Surface.Render`, ver la evidencia de DC-002.159b). El ente sale igual, sin la mancha del piso.
			layer.SetShadow(cofLayer.Shadow != 0 && sombrasDelCompuesto)

			if err := layer.SetDirection(c.direction); err != nil {
				return nil, err
			}

			mode.layers[cofLayer.Type] = layer
		}
	}

	return mode, nil
}

// layerPaths are the files of a layer, in the order they are tried (DCC, then DC6).
func (c *Composite) layerPaths(layerKey, layerValue, animationMode, weaponClass string) []string {
	return []string{
		fmt.Sprintf("%s/%s/%s/%s%s%s%s%s.dcc", c.basePath, c.token, layerKey, c.token, layerKey, layerValue, animationMode, weaponClass),
		fmt.Sprintf("%s/%s/%s/%s%s%s%s%s.dc6", c.basePath, c.token, layerKey, c.token, layerKey, layerValue, animationMode, weaponClass),
	}
}

func (c *Composite) loadCompositeLayer(layerKey, layerValue, animationMode, weaponClass,
	palettePath string, drawEffect d2enum.DrawEffect) (d2interface.Animation, error) {
	animationPaths := c.layerPaths(layerKey, layerValue, animationMode, weaponClass)

	for idx := range animationPaths {
		exists, err := c.FileExists(animationPaths[idx])
		if !exists || err != nil {
			return nil, fmt.Errorf("animation path '%s' not found: %v", animationPaths[idx], err)
		}

		animation, err := c.LoadAnimationWithEffect(animationPaths[idx], palettePath, drawEffect)
		if err == nil {
			return animation, nil
		}
	}

	return nil, errors.New("animation not found")
}

// GetSize returns the size of the composite
func (c *Composite) GetSize() (w, h int) {
	c.updateSize()
	return c.size.Width, c.size.Height
}

// animationConRecuadro159 es la parte de una animacion que sabe la caja de su cuadro actual (DC-002.159c). La
// cumplen las animaciones del motor (`Animation`, que es lo que hay detras de un DCC y de un DC6); es una
// interfaz OPCIONAL para no tocar `d2interface.Animation`, igual que `superficieConLienzo` en cuadro_paletizado.go.
//
// El metodo va SIN exportar a proposito: lo unico que lo necesita es este archivo, y un metodo nuevo y exportado
// en `Animation` le regala un stub a cada animacion que la empotra (DCC y DC6), que es export data de mas para
// nada.
type animationConRecuadro159 interface {
	recuadroDelCuadro159() (izq, arriba, ancho, alto int)
}

// Recuadro159: la caja que ocupa el cuadro actual del compuesto, en pixeles y RELATIVA AL ORIGEN del compuesto
// (el punto donde se dibuja el ente, o sea los pies).
//
// PARA QUE (DC-002.159c). El heroe del Gate 0 se dibuja encima de un mapa que no se repinta: antes de pintarlo hay
// que guardar el recuadro de lienzo que el sprite tapa, y devolverlo en el cuadro siguiente (el borrado). Ese
// recuadro tiene que cubrir TODO lo que el compuesto dibuja, y eso no es una constante: cada capa (pelo, cuerpo,
// piernas, arma) se dibuja con su propio desplazamiento, y los desplazamientos CAMBIAN con el cuadro de la
// animacion. Medido en DC-002.159b: con 160x160 fijo el cuadro de parado entraba, pero caminando quedaban las
// piernas del cuadro anterior pegadas al mapa.
//
// La caja es la union de las cajas de las capas que la prioridad del COF manda dibujar en ESTE cuadro (las mismas
// capas que recorre `Render`), cada una con su desplazamiento. `ok` en false cuando todavia no hay modo ni capas:
// ahi el que llama no tiene con que calcular nada y no debe dibujar (no habria forma de borrarlo despues).
func (c *Composite) Recuadro159() (izq, arriba, ancho, alto int, ok bool) {
	if c.mode == nil || c.mode.cof == nil {
		return 0, 0, 0, 0, false
	}

	direction := d2cof.Dir64ToCof(c.direction, c.mode.cof.NumberOfDirections)
	if direction < 0 || direction >= len(c.mode.cof.Priority) {
		return 0, 0, 0, 0, false
	}

	prioridad := c.mode.cof.Priority[direction]
	if c.mode.frameIndex < 0 || c.mode.frameIndex >= len(prioridad) {
		return 0, 0, 0, 0, false
	}

	x0, y0, x1, y1 := 0, 0, 0, 0
	hayCaja := false

	for _, layerIndex := range prioridad[c.mode.frameIndex] {
		if int(layerIndex) < 0 || int(layerIndex) >= len(c.mode.layers) {
			continue
		}

		layer := c.mode.layers[layerIndex]
		if layer == nil {
			continue
		}

		conRecuadro, sabe := layer.(animationConRecuadro159)
		if !sabe {
			continue
		}

		layerIzq, layerArriba, layerAncho, layerAlto := conRecuadro.recuadroDelCuadro159()
		if layerAncho <= 0 || layerAlto <= 0 {
			continue
		}

		if !hayCaja {
			x0, y0, x1, y1 = layerIzq, layerArriba, layerIzq+layerAncho, layerArriba+layerAlto
			hayCaja = true

			continue
		}

		if layerIzq < x0 {
			x0 = layerIzq
		}

		if layerArriba < y0 {
			y0 = layerArriba
		}

		if layerIzq+layerAncho > x1 {
			x1 = layerIzq + layerAncho
		}

		if layerArriba+layerAlto > y1 {
			y1 = layerArriba + layerAlto
		}
	}

	if !hayCaja {
		return 0, 0, 0, 0, false
	}

	return x0, y0, x1 - x0, y1 - y0, true
}

func (c *Composite) updateSize() {
	if c.mode == nil {
		return
	}

	direction := d2cof.Dir64ToCof(c.direction, c.mode.cof.NumberOfDirections)

	biggestW, biggestH := 0, 0

	for _, layerIndex := range c.mode.cof.Priority[direction][c.mode.frameIndex] {
		layer := c.mode.layers[layerIndex]
		if layer != nil {
			w, h := layer.GetCurrentFrameSize()

			if biggestW < w {
				biggestW = w
			}

			if biggestH < h {
				biggestH = h
			}
		}
	}

	if c.size == nil {
		c.size = &size{}
	}

	c.size.Width = biggestW
	c.size.Height = biggestH
}

func baseString(baseType d2enum.ObjectType) string {
	switch baseType {
	case d2enum.ObjectTypePlayer:
		return "/data/global/chars"
	case d2enum.ObjectTypeCharacter:
		return "/data/global/monsters"
	case d2enum.ObjectTypeItem:
		return "/data/global/objects"
	default:
		return ""
	}
}

// sombrasDelCompuesto (DC-002.159b): si las capas del COF que piden sombra (cofLayer.Shadow != 0) la dibujan.
//
// POR QUE SE PUEDE APAGAR. La sombra de un ente no es el ente: se pinta con escala (x1, y0.5), sesgo (0.5, 0) y
// brillo cero, o sea el camino GENERAL de `d2core/d2render/pvr` (matriz invertida y cuenta por pixel). En la
// consola ese camino mata el SH-4 en el primer cuadro del heroe del Gate 0 -- medido en DC-002.159b: la sonda de
// `Composite.Render` muestra la capa que va a dibujar, los seis `Push*` de `renderShadow` pasan, y muere dentro
// de `Surface.Render` (`Unhandled exception`, PC en datos, PR adentro de `runtime.mapassign`).
//
// Apagada, el compuesto dibuja las capas de cuerpo (que es lo que hacia falta ver) y el heroe queda sin la mancha
// del piso. Prendida (el default, que es lo que ve la PC) no cambia nada.
var sombrasDelCompuesto = true

// UsarSombrasDelCompuesto prende o apaga las sombras del compuesto (DC-002.159b). La apaga el arranque de la
// consola; en la PC queda prendida.
func UsarSombrasDelCompuesto(prendidas bool) { sombrasDelCompuesto = prendidas }

// sombrasDibujadas159h cuenta las sombras del compuesto dibujadas desde que arranco el proceso
// (DC-002.159h). La incrementa `Animation.renderShadow`, que es el unico camino que apila brillo cero.
var sombrasDibujadas159h int

// SombrasDibujadas159h devuelve cuantas sombras del compuesto se dibujaron (DC-002.159h). En la consola
// la imprime la linea `CUADRO` (`sombras=`): con las sombras prendidas crece una vez por capa y por
// cuadro y con `UsarSombrasDelCompuesto(false)` queda en 0, que es la prueba de que el camino HSV de la
// sombra se recorre o no se recorre -- no de que "no paso nada".
func SombrasDibujadas159h() int { return sombrasDibujadas159h }

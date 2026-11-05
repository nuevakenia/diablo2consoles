package d2asset

import (
	"errors"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2enum"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2dcc"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2interface"
)

var _ d2interface.Animation = &DCCAnimation{} // Static check to confirm struct conforms to
// interface

// direccionesPerezosas (DC-002.159b): politica de la CONSOLA, apagada en la PC.
//
// QUE CAMBIA. Con esto prendido, un `.dcc` se carga con `d2dcc.LoadPerezoso` (solo la cabecera) y cada direccion
// se decodifica la primera vez que se dibuja; al cambiar de direccion se tiran los pixeles y las superficies de
// las demas. Un modo del heroe son siete capas y las 16 direcciones de una capa de la amazona son 419.184 B de
// pixeles descomprimidos (medido en el Gate 0): con el mapa compuesto eso no entra en el semiespacio de
// 1.835.008 B (DC-002.159 dejo 1.672.456 B vivos y el cuadro en 537 ms).
//
// POR QUE NO EN LA PC. Ahi sobran 2 GB y varias animaciones comparten el mismo `DCC`, asi que tener todas las
// direcciones decodificadas es gratis y evita re-decoding al girar.
var direccionesPerezosas bool

// UsarDireccionesPerezosas prende o apaga la decodificacion por direccion bajo pedido (DC-002.159b). La prende
// el arranque de la consola (`ArrancarComoConsola`); en la PC queda apagada.
func UsarDireccionesPerezosas(prendida bool) {
	direccionesPerezosas = prendida
}

// DireccionesPerezosas dice si esta prendida la politica de la consola (DC-002.159b).
func DireccionesPerezosas() bool {
	return direccionesPerezosas
}

func newDCCAnimation(
	dcc *d2dcc.DCC,
	pal d2interface.Palette,
	effect d2enum.DrawEffect,
) (d2interface.Animation, error) {
	DCC := &DCCAnimation{
		dcc:     dcc,
		palette: pal,
	}

	anim := &Animation{
		playLength: defaultPlayLength,
		playLoop:   true,
		effect:     effect,
		onBindRenderer: func(r d2interface.Renderer) error {
			if DCC.renderer != r {
				DCC.renderer = r

				// the images belong to a renderer: build them again, per direction, when drawn
				for i := range DCC.directions {
					DCC.directions[i].surfaced = false
				}
			}

			return nil
		},
		ensureDirection: func(direction int) error {
			if DCC.renderer == nil {
				return nil
			}

			return DCC.createDirectionSurfaces(direction)
		},
	}

	DCC.Animation = *anim

	err := DCC.init()
	if err != nil {
		return nil, err
	}

	return DCC, nil
}

// DCCAnimation represents an animation decoded from DCC
type DCCAnimation struct {
	Animation
	dcc     *d2dcc.DCC
	palette d2interface.Palette
}

func (a *DCCAnimation) init() error {
	a.directions = make([]animationDirection, a.dcc.NumberOfDirections)

	for directionIndex := range a.directions {
		a.directions[directionIndex].frames = make([]animationFrame, a.dcc.FramesPerDirection)
	}

	// DC-002.159b: en la consola NO se decodifican las 16 direcciones al cargar. `decodeDirection` se encarga
	// de la que se pida (`SetDirection`) y de volver a decodificarla si otro la habia soltado.
	if direccionesPerezosas {
		return nil
	}

	err := a.decode()

	return err
}

// Clone creates a copy of the animation
func (a *DCCAnimation) Clone() d2interface.Animation {
	clone := &DCCAnimation{}
	clone.Animation = *a.Animation.Clone().(*Animation)
	clone.dcc = a.dcc.Clone()
	clone.palette = a.palette

	return clone
}

// SetDirection places the animation in the direction of an animation
func (a *DCCAnimation) SetDirection(directionIndex int) error {
	const smallestInvalidDirectionIndex = 64
	if directionIndex >= smallestInvalidDirectionIndex {
		return errors.New("invalid direction index")
	}

	direction := d2dcc.Dir64ToDcc(directionIndex, len(a.directions))
	if !a.directions[direction].decoded {
		err := a.decodeDirection(direction)
		if err != nil {
			return err
		}
	}

	a.directionIndex = direction
	a.frameIndex = 0

	// DC-002.159b: NO se tiran las otras direcciones al girar. Se probo (ver `soltarLasDireccionesQueNoSeVen`)
	// y el remedio salio peor que la enfermedad: una direccion de una capa son ~4 KB de pixeles (442.601 B entre
	// las 16 direcciones de las 7 capas del modo), asi que soltarlas ahorra poco, pero volver a mirar una
	// direccion ya soltada cuesta una decodificacion ENTERA de la capa (la `LG` son 202.352 B de pixeles de un
	// archivo de 63.718 B). Con el ente girando aunque sea un poco -- el caminar recalcula la direccion en cada
	// cuadro -- el POC pasaba de decodificar de vez en cuando a decodificar en cada cuadro: medido en el Gate 0,
	// el recolector corria 4 veces cada 15 cuadros y a los ~2 s de caminar moria con `fatal error: stack_alloc:
	// out of memory`. Las direcciones que no se ven quedan decodificadas: lo que se ahorra de verdad es el
	// archivo crudo, que no se decodifica hasta que hace falta (`LoadPerezoso`).
	return nil
}

// soltarLasDireccionesQueNoSeVen (DC-002.159b): deja SOLO la direccion que se esta dibujando. NO se llama
// desde `SetDirection` -- ver el porque ahi arriba: medido, soltar y volver a decodificar sale mas caro que
// guardar. Queda como herramienta para un caso que si lo pida (un modo que no vuelve, por ejemplo).
//
// Suelta dos cosas distintas: las superficies de esta animacion (que vuelven a armarse en `ensureSurfaces`) y los
// pixeles descomprimidos del `DCC`, que se vuelven a decodificar del archivo cuando hagan falta. El `DCC` es
// compartido por las animaciones que salen de la cache, asi que soltar los pixeles no rompe a nadie: el que los
// necesite los pide de nuevo (`DecodificarDireccion` es idempotente).
func (a *DCCAnimation) soltarLasDireccionesQueNoSeVen(seQueda int) {
	for i := range a.directions {
		if i == seQueda {
			continue
		}

		for f := range a.directions[i].frames {
			a.directions[i].frames[f].image = nil
		}

		a.directions[i].surfaced = false
		a.directions[i].decoded = false
	}

	a.dcc.SoltarLasOtrasDirecciones(seQueda)
}

func (a *DCCAnimation) decode() error {
	for directionIndex := 0; directionIndex < len(a.directions); directionIndex++ {
		err := a.decodeDirection(directionIndex)
		if err != nil {
			return err
		}
	}

	return nil
}

func (a *DCCAnimation) decodeDirection(directionIndex int) error {
	// DC-002.159b: la direccion puede estar sin decodificar en el `DCC` (carga perezosa, o porque se solto al
	// cambiar de direccion). Se decodifica de los bytes del archivo, que el `DCC` conserva.
	if err := a.dcc.DecodificarDireccion(directionIndex); err != nil {
		return err
	}

	dccDirection := a.dcc.Directions[directionIndex]

	for frameIndex := range dccDirection.Frames {
		if a.directions[directionIndex].frames == nil {
			a.directions[directionIndex].frames = make([]animationFrame, a.dcc.FramesPerDirection)
		}

		a.directions[directionIndex].decoded = true

		frame := a.decodeFrame(directionIndex)
		a.directions[directionIndex].frames[frameIndex] = frame
	}

	return nil
}

func (a *DCCAnimation) decodeFrame(directionIndex int) animationFrame {
	dccDirection := a.dcc.Directions[directionIndex]

	frame := animationFrame{
		width:   dccDirection.Box.Width,
		height:  dccDirection.Box.Height,
		offsetX: dccDirection.Box.Left,
		offsetY: dccDirection.Box.Top,
		decoded: true,
	}

	return frame
}

func (a *DCCAnimation) createDirectionSurfaces(directionIndex int) error {
	for frameIndex := 0; frameIndex < a.dcc.FramesPerDirection; frameIndex++ {
		if !a.directions[directionIndex].decoded {
			err := a.decodeDirection(directionIndex)
			if err != nil {
				return err
			}
		}

		surface, err := a.createFrameSurface(directionIndex, frameIndex)
		if err != nil {
			return err
		}

		a.directions[directionIndex].frames[frameIndex].image = surface
	}

	a.directions[directionIndex].surfaced = true

	return nil
}

func (a *DCCAnimation) createFrameSurface(directionIndex, frameIndex int) (d2interface.Surface, error) {
	if !a.directions[directionIndex].frames[frameIndex].decoded {
		frame := a.decodeFrame(directionIndex)
		a.directions[directionIndex].frames[frameIndex] = frame
	}

	// DC-002.159b: con carga perezosa los pixeles de esta direccion pueden no estar (otra animacion que comparte
	// el `DCC` los solto al girar). Se decodifican del archivo antes de armar la superficie.
	if err := a.dcc.DecodificarDireccion(directionIndex); err != nil {
		return nil, err
	}

	dccFrame := a.dcc.Directions[directionIndex].Frames[frameIndex]
	animFrame := a.directions[directionIndex].frames[frameIndex]
	indexData := dccFrame.PixelData

	if len(indexData) != (animFrame.width * animFrame.height) {
		return nil, errors.New("pixel data incorrect")
	}

	if a.renderer == nil {
		return nil, errors.New("no renderer")
	}

	sfc := a.renderer.NewSurface(animFrame.width, animFrame.height)

	// DC-002.85: la expansion va DIRECTO al lienzo de la superficie si el backend sabe hacerlo (consola);
	// si no, queda el buffer intermedio de siempre (ver cuadro_paletizado.go).
	cargarCuadroEnSuperficie(sfc, indexData, a.palette)

	return sfc, nil
}

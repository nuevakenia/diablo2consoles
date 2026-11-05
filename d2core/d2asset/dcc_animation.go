package d2asset

import (
	"errors"
	"fmt"

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

// superficiesPorCuadro (DC-002.159c): la OTRA politica de la consola, apagada en la PC.
//
// QUE CAMBIA. Con esto prendido, un `.dcc` no arma las superficies de toda la direccion (8 cuadros) la primera
// vez que la dibuja: mantiene UNA sola superficie por animacion y la vuelve a llenar con los pixeles del cuadro
// que se esta dibujando. Es lo que hace que el caminar dure: medido en el Gate 0 (DC-002.159b), entrar en el
// modo `TW` y cambiar de direccion caminando costaba ~1 MB de superficies de golpe (7 capas x 8 cuadros) contra
// un semiespacio de 1.835.008 B, el recolector corria cinco veces seguidas y el SH-4 moria con
// `fatal error: stack_alloc: out of memory` a los dos segundos de arrancar a caminar.
//
// POR QUE NO EN LA PC. Ahi la memoria sobra, el recolector no corre en cada cuadro y el precio de tener todas
// las superficies armadas es cero: no hay por que pagar la expansion de la paleta cada vez que cambia el cuadro.
var superficiesPorCuadro bool

// UsarSuperficiesPorCuadro prende o apaga la superficie unica por animacion (DC-002.159c). La prende el
// arranque de la consola (`ArrancarComoConsola`); en la PC queda apagada.
func UsarSuperficiesPorCuadro(prendida bool) {
	superficiesPorCuadro = prendida
}

// soltarAlGirar (DC-002.159c): al cambiar de sentido, tirar los pixeles descomprimidos de las otras direcciones
// (`soltarLasDireccionesQueNoSeVen`). Apagada por defecto porque DC-002.159b la midio y la descarto: guardar las
// 16 direcciones de las 7 capas son ~442.601 B VIVOS, pero volver a decodificar cuesta una decodificacion entera
// por capa. Queda como palanca para volver a medir si el vivo se come el semiespacio.
var soltarAlGirar bool

// UsarSoltarAlGirar159 prende o apaga la palanca `soltarAlGirar`.
func UsarSoltarAlGirar159(si bool) { soltarAlGirar = si }

// SuperficiesPorCuadro dice si esta prendida la politica de la consola (DC-002.159c).
func SuperficiesPorCuadro() bool {
	return superficiesPorCuadro
}

// contadorDeSuperficies159: cuantas superficies de cuadro se pidieron y cuantos bytes suman, desde que arranco
// el proceso. Es el instrumento con el que se mide esta politica en la consola: con la superficie unica el
// contador NO crece cuadro a cuadro (solo cuando cambia el tamano, o sea al girar a una direccion de otra caja).
var contadorDeSuperficies159 struct {
	creadas int
	bytes   int
}

// SuperficiesDeCuadro159 devuelve cuantas superficies de cuadro se pidieron y cuantos bytes de RGBA suman.
func SuperficiesDeCuadro159() (creadas, bytes int) {
	return contadorDeSuperficies159.creadas, contadorDeSuperficies159.bytes
}

// decodificacionesDeDireccion159: cuantas veces se descomprimio una direccion de un `.dcc` desde que arranco el
// proceso. Camina el ente, el motor recalcula la direccion en cada cuadro, y cada direccion nueva de una capa
// cuesta su decodificacion; el contador lo dice sin adivinar (ver `DecodificacionesDeDireccion159`).
var decodificacionesDeDireccion159 int

// DecodificacionesDeDireccion159 devuelve cuantas direcciones de `.dcc` se descomprimieron.
func DecodificacionesDeDireccion159() int { return decodificacionesDeDireccion159 }

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

				// DC-002.159c: la superficie unica tambien es del renderer viejo
				DCC.superficieDelCuadro = nil
				DCC.superficieAncho, DCC.superficieAlto = 0, 0
			}

			return nil
		},
		ensureDirection: func(direction int) error {
			if DCC.renderer == nil {
				return nil
			}

			return DCC.createDirectionSurfaces(direction)
		},
		ensureFrame: func(direction, frame int) error {
			if DCC.renderer == nil {
				return nil
			}

			return DCC.armarLaSuperficieDelCuadro(direction, frame)
		},
	}

	// DC-002.159c: en la consola la animacion tiene UNA superficie y la rellena cuadro a cuadro; en la PC se
	// arma la direccion entera la primera vez, como siempre (ver `superficiesPorCuadro`).
	if !superficiesPorCuadro {
		anim.ensureFrame = nil
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

	// DC-002.159c: soltar las otras direcciones al girar quedo como PALANCA (`soltarAlGirar`), apagada. La
	// tarjeta DC-002.159b ya la habia medido y descartado: soltar ahorra ~4 KB por capa y volver a decodificar
	// cuesta una decodificacion entera; con 9 giros en 195 cuadros de caminar no compensa. Se deja el interruptor
	// porque el numero que la justificaria (el vivo pegado al semiespacio) hay que volver a medirlo con cada modo.
	if superficiesPorCuadro && soltarAlGirar {
		a.soltarLasDireccionesQueNoSeVen(direction)
	}

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
	// DC-002.159c: la direccion puede estar sin decodificar en el `DCC` (carga perezosa, o porque se solto al
	// cambiar de direccion). Se decodifica de los bytes del archivo, que el `DCC` conserva.
	//
	// DC-002.159f: la decodificacion va marcada como tramo fino (`adDecod`): es una de las tres cosas caras
	// que la fase `heroe` junta (ver marcas_159f.go).
	marcarAsset159f(MarcaAssetDecod159f)

	err := a.dcc.DecodificarDireccion(directionIndex)

	marcarAsset159f(MarcaAssetResto159f)

	if err != nil {
		return err
	}

	decodificacionesDeDireccion159++

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

// armarLaSuperficieDelCuadro (DC-002.159c): deja en la UNICA superficie de esta animacion los pixeles del cuadro
// que se va a dibujar. Es la version de la consola de `createDirectionSurfaces`, que armaba las 8 superficies de
// la direccion de una vez (~1 MB para un modo del heroe de 7 capas, contra 1.835.008 B de semiespacio).
//
// COMO. Si el cuadro pedido ya es el que esta cargado, no hace nada (esto se llama en cada dibujo). Si no, reusa
// la superficie que ya tiene (mismo tamano) y la vuelve a llenar con `cargarCuadroEnSuperficie`, que es la misma
// expansion de paleta que se pagaba al crearla. La direccion se decodifica si hace falta, igual que antes.
//
// LO QUE SE SUELTA. La referencia del cuadro anterior (`frames[dir].frames[cuadro].image`) se pone en nil antes de
// reusar la superficie: si quedara apuntando a la superficie reusada, el cuadro viejo mostraria los pixeles del
// nuevo.
func (a *DCCAnimation) armarLaSuperficieDelCuadro(directionIndex, frameIndex int) error {
	if directionIndex < 0 || directionIndex >= len(a.directions) {
		return fmt.Errorf("dcc: direction %d out of range (the animation has %d)", directionIndex, len(a.directions))
	}

	if !a.directions[directionIndex].decoded {
		if err := a.decodeDirection(directionIndex); err != nil {
			return err
		}
	}

	frames := a.directions[directionIndex].frames
	if frameIndex < 0 || frameIndex >= len(frames) {
		return fmt.Errorf("dcc: frame %d out of range (the direction has %d)", frameIndex, len(frames))
	}

	if frames[frameIndex].image != nil {
		return nil
	}

	// DC-002.159f: de aca en adelante es el tramo `adSup` (armar y rellenar la superficie del cuadro). La
	// decodificacion que va adentro vuelve a marcar `adSup` al terminar, y el final del tramo lo marca
	// `adResto`: son tramos, no una pila (ver marcas_159f.go).
	marcarAsset159f(MarcaAssetSup159f)

	defer marcarAsset159f(MarcaAssetResto159f)

	marcarAsset159f(MarcaAssetDecod159f)

	if err := a.dcc.DecodificarDireccion(directionIndex); err != nil {
		return err
	}

	marcarAsset159f(MarcaAssetSup159f)

	frame := frames[frameIndex]
	dccFrame := a.dcc.Directions[directionIndex].Frames[frameIndex]

	if len(dccFrame.PixelData) != (frame.width * frame.height) {
		return errors.New("pixel data incorrect")
	}

	if a.renderer == nil {
		return errors.New("no renderer")
	}

	// el cuadro anterior pierde la referencia ANTES de que la superficie cambie de contenido
	if a.superficieDelCuadro != nil &&
		a.superficieDireccion < len(a.directions) &&
		a.superficieCuadro < len(a.directions[a.superficieDireccion].frames) {
		a.directions[a.superficieDireccion].frames[a.superficieCuadro].image = nil
	}

	// DC-002.159f: la superficie que se va a usar, y de que tamano.
	//
	// CON EL REUSO PRENDIDO (la consola), se pide con un margen del 25% sobre la caja del cuadro y se reusa
	// mientras los cuadros ENTREN: antes se pedia una nueva cada vez que la caja no era identica a la que habia,
	// y caminando cada giro cambia la caja (medido: 105 superficies en 390 cuadros, `sup=105/157.032 B`). El
	// cuadro se carga en su esquina y el que dibuja RECORTA (`Animation.Render` -> `RenderRecorte159f`), asi que
	// el sobrante no se pinta.
	//
	// CON EL REUSO APAGADO (la PC), una superficie EXACTA por caja, como estaba desde siempre: ahi el cuadro se
	// dibuja con el tamano de la superficie y no hay recorte que valga.
	superficie := a.superficieDelCuadro

	necesitaNueva := superficie == nil
	if !necesitaNueva {
		if superficieReusada159f {
			// OJO: se compara la CAJA del cuadro contra la superficie, no el pedido con margen contra la
			// superficie. Con el margen en los dos lados, cada caja un poco mas grande pedia un 25% mas que la
			// anterior y la superficie crecia sin freno (`sup=55/125.656 B`, medido: el doble de pedidos que
			// comparando la caja).
			necesitaNueva = frame.width > a.superficieAncho || frame.height > a.superficieAlto
		} else {
			necesitaNueva = a.superficieAncho != frame.width || a.superficieAlto != frame.height
		}
	}

	if necesitaNueva {
		ancho, alto := frame.width, frame.height

		if superficieReusada159f {
			ancho, alto = frame.width+frame.width/4, frame.height+frame.height/4
		}

		superficie = a.renderer.NewSurface(ancho, alto)
		a.superficieDelCuadro = superficie
		a.superficieAncho, a.superficieAlto = ancho, alto

		contadorDeSuperficies159.creadas++
		contadorDeSuperficies159.bytes += ancho * alto * 4
	}

	cargarCuadroEnSuperficie(superficie, dccFrame.PixelData, a.palette, frame.width, frame.height)

	frames[frameIndex].image = superficie
	a.superficieDireccion, a.superficieCuadro = directionIndex, frameIndex

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
	cargarCuadroEnSuperficie(sfc, indexData, a.palette, animFrame.width, animFrame.height)

	return sfc, nil
}

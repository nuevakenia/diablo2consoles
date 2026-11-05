package ebiten

// DC-002.159o -- LA PRUEBA DEL ARREGLO DEL COLOR: la misma matriz, y el dibujo sin despachar la interfaz.
//
// TRES COSAS, y las tres son la tarjeta:
//
//  1. `TestElColorResueltoDaLaMismaMatriz159o`: para un juego de colores (opacos, con alfa, con alfa cero, y tipos
//     distintos de `color.Color`) la matriz del camino NUEVO (canales resueltos al empujar) es IGUAL elemento por
//     elemento a la del camino VIEJO (la interfaz despachada al dibujar). Se comparan los veinte elementos y el
//     `CompositeMode`, en los dos puntos donde el dibujo arma la matriz: `fillDrawImageOptions` y `prepareColorM`
//     (con y sin HSV, y con los efectos que editan la matriz).
//
//  2. `TestElDibujoNoDespachaLaInterfaz159o`: EL MUTADOR. Se empuja un color sano, se calcula su matriz, y despues
//     se PUDRE la interfaz del estado con una cuyo `RGBA()` revienta (es lo que dejaba el recolector en 159m: una
//     tabla de metodos que ya no era). Con el arreglo prendido el dibujo no la toca y la matriz sale igual; con el
//     arreglo apagado la prueba REVIENTA, o sea que caza su regresion.
//
//  3. `TestElColorNeutroForzadoEsLaIdentidad159o`: la palanca del instrumento (empujar blanco cuando el que dibuja
//     no empuja color) no cambia la matriz: sale la identidad, igual que sin color. Es lo que permite forzar el
//     camino miles de veces sin cambiar un pixel del cuadro.

import (
	"image/color"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2enum"
)

func coloresDeLaPrueba159o() []color.Color {
	return []color.Color{
		nil,
		color.RGBA{R: 255, G: 255, B: 255, A: 160}, // el de la sombra del `.dt1` (renderShadow)
		color.RGBA{R: 255, G: 255, B: 255, A: 255}, // el neutro de la palanca
		color.RGBA{R: 200, G: 180, B: 120, A: 255},
		color.RGBA{R: 40, G: 50, B: 90, A: 128},
		color.RGBA{R: 3, G: 1, B: 2, A: 1},
		color.RGBA{A: 0}, // alfa cero: la matriz toda cero
		color.NRGBA{R: 90, G: 10, B: 200, A: 200},
		color.Gray{Y: 77},
	}
}

// fijarColorDeLaPrueba159o deja el estado como lo deja `PushColor`: la interfaz y los canales resueltos.
func (s *ebitenSurface) fijarColorDeLaPrueba159o(c color.Color) {
	s.stateCurrent.color = c
	s.stateCurrent.canales = resolverCanales159o(c)
}

// matrices159o devuelve las matrices que el dibujo arma con el estado actual: la de `fillDrawImageOptions` (el
// camino donde murio el SH-4) y las de `prepareColorM` para las combinaciones que editan la matriz.
func matrices159o(s *ebitenSurface) map[string][]float64 {
	salida := map[string][]float64{}

	plano := s.createDrawImageOptions()
	salida["fill"] = elementos159o(plano.ColorM)

	for _, hsv := range []bool{false, true} {
		for _, efecto := range []d2enum.DrawEffect{
			d2enum.DrawEffectNone, d2enum.DrawEffectPctTransparency25, d2enum.DrawEffectMod2XTrans,
			d2enum.DrawEffectMod2X,
		} {
			s.stateCurrent.effect = efecto
			opts := s.scratchDrawImageOptions()
			s.prepareColorM(opts, hsv)

			nombre := "prepare-hsv=" + bool159o(hsv) + "-efecto=" + int159o(int(efecto))
			salida[nombre] = elementos159o(opts.ColorM)
		}
	}

	s.stateCurrent.effect = d2enum.DrawEffectNone

	return salida
}

func elementos159o(m ebiten.ColorM) []float64 {
	salida := make([]float64, 0, 20)

	for i := 0; i < 4; i++ {
		for j := 0; j < 5; j++ {
			salida = append(salida, m.Element(i, j))
		}
	}

	return salida
}

func bool159o(b bool) string {
	if b {
		return "si"
	}

	return "no"
}

func int159o(n int) string {
	if n == 0 {
		return "0"
	}

	digitos := ""

	for n > 0 {
		digitos = string(rune('0'+n%10)) + digitos
		n /= 10
	}

	return digitos
}

// TestElColorResueltoDaLaMismaMatriz159o: el arreglo no cambia ni un elemento de la matriz.
func TestElColorResueltoDaLaMismaMatriz159o(t *testing.T) {
	previo := ColorAlEmpujar159o()

	defer UsarColorAlEmpujar159o(previo)

	for _, c := range coloresDeLaPrueba159o() {
		// el camino viejo: guardar la interfaz y despacharla al dibujar
		UsarColorAlEmpujar159o(false)

		vieja := createEbitenSurface(nil, nil)
		vieja.PushColor(c)
		matricesViejas := matrices159o(vieja)

		// el camino nuevo: los canales resueltos al empujar
		UsarColorAlEmpujar159o(true)

		nueva := createEbitenSurface(nil, nil)
		nueva.PushColor(c)
		matricesNuevas := matrices159o(nueva)

		for nombre, mv := range matricesViejas {
			mn, hay := matricesNuevas[nombre]
			if !hay {
				t.Fatalf("color %v: falta la matriz %q del camino nuevo", c, nombre)
			}

			if len(mv) != len(mn) {
				t.Fatalf("color %v matriz %q: %d elementos contra %d", c, nombre, len(mv), len(mn))
			}

			for k := range mv {
				if mv[k] != mn[k] {
					t.Fatalf("color %v matriz %q elemento %d: camino viejo %v, camino nuevo %v",
						c, nombre, k, mv[k], mn[k])
				}
			}
		}
	}
}

// colorEnvenenado159o: un `color.Color` cuya `RGBA()` revienta. Es la interfaz que el recolector dejaba apuntando
// a `.rodata` en DC-002.159m: cualquier despacho de `RGBA()` despues de que el objeto se movio termina en una PC
// que no es codigo.
type colorEnvenenado159o struct{ despachos *int }

func (c colorEnvenenado159o) RGBA() (r, g, b, a uint32) {
	*c.despachos++

	panic("DC-002.159o: el dibujo despacho color.Color.RGBA() de una interfaz podrida")
}

// revienta159o corre f y dice si revento.
func revienta159o(f func()) (reviento bool) {
	defer func() {
		if recover() != nil {
			reviento = true
		}
	}()

	f()

	return false
}

func TestElDibujoNoDespachaLaInterfaz159o(t *testing.T) {
	previo := ColorAlEmpujar159o()

	defer UsarColorAlEmpujar159o(previo)

	sano := color.RGBA{R: 255, G: 255, B: 255, A: 160}

	// la matriz buena, con el estado sano
	UsarColorAlEmpujar159o(true)

	s := createEbitenSurface(nil, nil)
	s.PushColor(sano)
	esperada := s.createDrawImageOptions().ColorM

	despachos := 0
	s.stateCurrent.color = colorEnvenenado159o{despachos: &despachos}

	// 1. con el arreglo prendido, el dibujo no la toca
	dibujo := func() {
		opts := s.createDrawImageOptions()
		for i := 0; i < 4; i++ {
			for j := 0; j < 5; j++ {
				if opts.ColorM.Element(i, j) != esperada.Element(i, j) {
					t.Fatalf("con el arreglo prendido la matriz cambio en (%d,%d): %v contra %v",
						i, j, opts.ColorM.Element(i, j), esperada.Element(i, j))
				}
			}
		}

		s.stateCurrent.saturation, s.stateCurrent.brightness = 0.5, 0.5

		optsHSV := s.scratchDrawImageOptions()
		s.prepareColorM(optsHSV, true)
		s.applyHSV(s.scratchDrawImageOptions())
	}

	dibujo = func() {
		opts := s.createDrawImageOptions()
		for i := 0; i < 4; i++ {
			for j := 0; j < 5; j++ {
				if opts.ColorM.Element(i, j) != esperada.Element(i, j) {
					t.Fatalf("con el arreglo prendido la matriz cambio en (%d,%d): %v contra %v",
						i, j, opts.ColorM.Element(i, j), esperada.Element(i, j))
				}
			}
		}

		// DC-002.159o: el camino de INDICES (la cache del mapa) tampoco puede despachar la interfaz.
		if _, hay := s.colorDeIndicesDelDibujo159o(); !hay {
			t.Fatal("el camino de indices dice que no hay color, y el estado tiene el color empujado")
		}
	}

	if revienta159o(dibujo) {
		t.Fatalf("con el arreglo prendido el dibujo despacho la interfaz podrida (%d despachos)", despachos)
	}

	if despachos != 0 {
		t.Fatalf("el dibujo despacho la interfaz %d veces con el arreglo prendido (tiene que ser 0)", despachos)
	}

	// 2. con el arreglo apagado SI la despacha: la prueba caza su regresion (en los dos caminos)
	UsarColorAlEmpujar159o(false)

	if !revienta159o(func() { s.createDrawImageOptions() }) {
		t.Fatal("con el arreglo apagado el dibujo NO despacho la interfaz: la prueba no caza la regresion")
	}

	if !revienta159o(func() { s.colorDeIndicesDelDibujo159o() }) {
		t.Fatal("con el arreglo apagado el camino de indices NO despacho la interfaz: la prueba no caza la regresion")
	}
}

// TestElColorDeIndicesDaElMismoRGBA159o: el camino de indices (la cache del mapa) da los mismos ocho bits con el
// arreglo prendido que preguntandole a la interfaz al dibujar. Se compara el color de 8 bits ENTERO, no solo los
// canales de la matriz: la regla del camino de indices usa un `color.RGBA` tal cual (200,180,120,128 tiene que
// salir 200 y no 199, que es lo que daria desmultiplicar).
func TestElColorDeIndicesDaElMismoRGBA159o(t *testing.T) {
	previo := ColorAlEmpujar159o()

	defer UsarColorAlEmpujar159o(previo)

	for _, c := range coloresDeLaPrueba159o() {
		UsarColorAlEmpujar159o(false)

		vieja := createEbitenSurface(nil, nil)
		vieja.PushColor(c)
		viejo, hayViejo := vieja.colorDeIndicesDelDibujo159o()

		UsarColorAlEmpujar159o(true)

		nueva := createEbitenSurface(nil, nil)
		nueva.PushColor(c)
		nuevo, hayNuevo := nueva.colorDeIndicesDelDibujo159o()

		if hayViejo != hayNuevo {
			t.Fatalf("color %v: el camino viejo dice hay=%v y el nuevo hay=%v", c, hayViejo, hayNuevo)
		}

		if viejo != nuevo {
			t.Fatalf("color %v: el camino viejo da %v y el nuevo %v", c, viejo, nuevo)
		}
	}
}

// TestElColorNeutroForzadoEsLaIdentidad159o: empujar blanco donde no habia color no cambia la matriz.
func TestElColorNeutroForzadoEsLaIdentidad159o(t *testing.T) {
	previo := ColorAlEmpujar159o()

	defer UsarColorAlEmpujar159o(previo)
	defer ForzarColorDelTinte159o(nil)

	// sin color: la matriz de `ColorM` queda como esta (la identidad de ebiten)
	s := createEbitenSurface(nil, nil)
	s.PushColor(nil)

	sinColor := s.createDrawImageOptions().ColorM

	// con la palanca: se empuja el neutro y la matriz es la identidad, o sea la misma
	ForzarColorDelTinte159o(color.RGBA{R: 255, G: 255, B: 255, A: 255})

	sc := createEbitenSurface(nil, nil)
	sc.PushColor(nil)

	conNeutro := sc.createDrawImageOptions().ColorM

	for i := 0; i < 4; i++ {
		for j := 0; j < 5; j++ {
			if sinColor.Element(i, j) != conNeutro.Element(i, j) {
				t.Fatalf("el neutro cambio la matriz en (%d,%d): %v contra %v",
					i, j, sinColor.Element(i, j), conNeutro.Element(i, j))
			}
		}
	}
}

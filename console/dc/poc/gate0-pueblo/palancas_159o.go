//go:build dreamcast

package main

// DC-002.159o -- LAS PALANCAS DE ESTA TARJETA, EN CONSTANTES.
//
// En la consola `os.Getenv` es un stub, asi que una palanca por variable de entorno seria una palanca que no se
// puede mover: las cuatro van aca, arriba, y el build dice cual esta puesta (`GATE0: PALANCAS159O ...`).
//
//	caminoDelTinteForzado159o  elige la rama del `PushColor` de `renderShadow` aunque la luz este prendida. Es el
//	                           camino que murio en DC-002.159m y que en el pueblo NO se recorria (la sombra se
//	                           iba por `renderShadedImage`). OJO: con esto las sombras del `.dt1` salen sin luz,
//	                           asi que no es la configuracion de la corrida de cierre; es la que ejerce el camino.
//	colorNeutroDelTinte159o    empuja `color.RGBA{255,255,255,255}` donde el que dibuja no empuja color. El neutro
//	                           da la matriz IDENTIDAD, o sea que el cuadro sale igual (la prueba del host compara
//	                           los veinte elementos de la matriz contra no tener color), y el camino del tinte se
//	                           ejerce miles de veces por corrida -- con el mapa pintado una sola vez, la palanca
//	                           de la sombra sola daria unas pocas decenas de usos.
//	colorAlEmpujar159o         EL ARREGLO de la tarjeta: el color se resuelve al empujarlo. En false queda el camino
//	                           viejo (guardar la interfaz y despacharla al dibujar): es el MUTADOR.
//	sondaDelColor159n          instala el gancho de la sonda de DC-002.159n (el latido y las clases).
//	venenoDelColor159o         EL MUTADOR EN LA CONSOLA: despues de empujar el color, escribe encima de las dos
//	                           palabras de la interfaz del estado (lo que el recolector le dejaba en DC-002.159m).
//	                           Con el arreglo APAGADO la corrida tiene que morir en `colorToColorM`; con el
//	                           arreglo prendido tiene que pasar. Es la prueba en contra de la tarjeta.
import (
	"image/color"

	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2map/d2maprenderer"
	renderEbiten "github.com/OpenDiablo2/OpenDiablo2/d2core/d2render/ebiten"
)

const (
	caminoDelTinteForzado159o = true
	colorNeutroDelTinte159o   = true
	colorAlEmpujar159o        = true
	sondaDelColor159n         = true
	venenoDelColor159o        = false
)

// colorNeutro159o: el blanco opaco. La matriz de un blanco opaco es la identidad (rf = gf = bf = af = 1), o sea
// que forzarlo donde no habia color no cambia un pixel.
var colorNeutro159o = color.RGBA{R: 255, G: 255, B: 255, A: 255}

// palancas159o pone las palancas y las dice en el log. Se llama al arrancar, antes de `Create`.
func palancas159o() {
	d2maprenderer.ForzarCaminoDelTinte159o(caminoDelTinteForzado159o)
	renderEbiten.UsarColorAlEmpujar159o(colorAlEmpujar159o)
	renderEbiten.EnvenenarElColor159o(venenoDelColor159o)

	if colorNeutroDelTinte159o {
		renderEbiten.ForzarColorDelTinte159o(colorNeutro159o)
	} else {
		renderEbiten.ForzarColorDelTinte159o(nil)
	}

	println(frase, "PALANCAS159O caminoDelTinteForzado=", caminoDelTinteForzado159o,
		"colorNeutroDelTinte=", colorNeutroDelTinte159o,
		"colorAlEmpujar=", colorAlEmpujar159o,
		"sonda=", sondaDelColor159n, "veneno=", venenoDelColor159o)
}

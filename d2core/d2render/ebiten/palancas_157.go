//go:build dreamcast

package ebiten

// DC-002.157 -- LA SUPERFICIE DEL MOTOR PRENDE LAS PALANCAS DE LA PRESENTACION (las de la consola).
//
// POR QUE EXISTE. La pantalla del Gate 0 (`d2game/d2gamescreen/pantalla_mapa_155.go`) tiene que poder
// prender y apagar, cuadro a cuadro, las tres palancas de la presentacion que viven en el shim del PVR:
//
//	ActivarConversionRapida157   -- el bucle de conversion de DC-002.135 en vez del de DC-002.132
//	ActivarConversionPorZona157  -- convertir solo el recuadro visible (640x480 de 800x600)
//	ActivarPresentacionQuieta157 -- si el cuadro no cambio, no convertir ni subir nada
//
// Pero la pantalla NO importa el shim: recibe una `d2interface.Surface` y pregunta por la interfaz
// `palancasDePresentacion157` (el mismo patron de `lienzoDeIndices`, DC-002.101/155). En la PC la Surface
// es Ebitengine de verdad, no implementa esta interfaz, y la pantalla no hace nada: el camino de la PC no
// cambia una linea.
//
// LAS TRES SON DEL SHIM Y NO DE LA SUPERFICIE: la superficie es solo el traductor. El estado de las
// palancas vive en el shim (`console/dc/engine/ebiten/v2/fases_157.go`), que es el que tiene el bucle de
// conversion y el que presenta el cuadro.
//
// EL ARCHIVO VA CON `//go:build dreamcast` Y ESO ES SOLO PARA EL `go build` DEL HOST (que asi lo saltea).
// El build de la consola NO lee esa linea: arma la lista de fuentes por NOMBRE
// (`scripts/console/palancas-del-build.sh`) y este archivo no lleva ninguno de los sufijos excluidos.

import "github.com/hajimehoshi/ebiten/v2"

// ActivarConversionRapida157: el bucle de conversion de DC-002.135 (y la subida agrupada de la VRAM).
func (s *ebitenSurface) ActivarConversionRapida157(si bool) { ebiten.ActivarConversionRapida157(si) }

// ActivarConversionPorFilas157: el orden de lectura del bucle rapido (DC-002.141).
func (s *ebitenSurface) ActivarConversionPorFilas157(si bool) {
	ebiten.ActivarConversionPorFilas157(si)
}

// ActivarConversionPorZona157: convertir solo el recuadro visible del lienzo.
func (s *ebitenSurface) ActivarConversionPorZona157(si bool) { ebiten.ActivarConversionPorZona157(si) }

// ActivarPresentacionQuieta157: si el cuadro no cambio, no convertir ni subir nada.
func (s *ebitenSurface) ActivarPresentacionQuieta157(si bool) {
	ebiten.ActivarPresentacionQuieta157(si)
}

// static check: la superficie cumple la interfaz que pide la pantalla del Gate 0
var _ interface {
	ActivarConversionRapida157(si bool)
	ActivarConversionPorFilas157(si bool)
	ActivarConversionPorZona157(si bool)
	ActivarPresentacionQuieta157(si bool)
} = &ebitenSurface{}

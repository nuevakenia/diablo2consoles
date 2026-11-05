//go:build dreamcast

package ebiten

// DC-002.159g -- EL SHIM DE LA CONSOLA SI SABE DIBUJAR UNA VENTANA (ver ventana_159g.go).
//
// El shim del PVR guarda los pixeles en RAM (RGBA de 8 bits) y su `DrawImage` ya sabe recorrer el origen; esto le
// agrega el recorte sin buffer. En la PC este archivo no entra (build tag) y `dibujarVentana` queda devolviendo
// false, que es la verdad: el Ebitengine de verdad no tiene ese metodo.

import "github.com/hajimehoshi/ebiten/v2"

func init() {
	dibujarVentana = func(dst, src *ebiten.Image, ancho, alto int, op *ebiten.DrawImageOptions) bool {
		dst.DrawImageVentana(src, ancho, alto, op)

		return true
	}
}

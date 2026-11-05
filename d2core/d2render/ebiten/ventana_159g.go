package ebiten

import "github.com/hajimehoshi/ebiten/v2"

// DC-002.159g -- EL DIBUJADO DE UNA VENTANA DEL ORIGEN (el recorte de la superficie reusada).
//
// QUE ES. `RenderRecorte` (nuevo metodo de `d2interface.Surface`) dibuja solo la region [0,ancho)x[0,alto) de la
// superficie de origen: la animacion del heroe guarda todos los cuadros de una direccion en UNA superficie y el
// cuadro que se dibuja ocupa su esquina, asi que el sobrante (pixeles de otros cuadros) no se puede pintar.
//
// POR QUE ES UN PUENTE Y NO UNA LLAMADA DIRECTA. Este paquete se compila DOS veces con dos `ebiten` distintos:
// con el Ebitengine de verdad en la PC y con el shim del PVR en la consola (`console/dc/engine/ebiten/v2`). Solo
// el shim sabe dibujar una ventana del origen sin armar un buffer (`Image.DrawImageVentana`, DC-002.159g); el
// Ebitengine de verdad no tiene ese metodo, y en la PC este camino no se usa nunca (el reuso de la superficie lo
// prende SOLO el arranque de la consola, asi que ahi las superficies son exactas).
//
// ES EL MISMO PATRON QUE `marca_carga_host.go`/`marca_carga_godc.go`: una variable que cada plataforma deja
// puesta, sin aserciones de interfaz (que en este gccgo no son confiables: ver DC-002.159f).
//
// `false` = "este backend no sabe dibujar una ventana": el que llama se queda con `RenderSection`, que en
// Ebitengine arma la subimagen (y en la PC no se paga, porque no hay nada que recortar).
var dibujarVentana = func(dst, src *ebiten.Image, ancho, alto int, op *ebiten.DrawImageOptions) bool {
	return false
}

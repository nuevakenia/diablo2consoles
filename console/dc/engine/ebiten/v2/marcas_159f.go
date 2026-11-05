package ebiten

// DC-002.159f -- EL PUENTE DE LAS MARCAS FINAS DENTRO DEL SHIM DEL PVR (ver
// d2game/d2gamescreen/marcas_159f.go).
//
// La fase `presente` de DC-002.159d (2.271 B/cuadro, constante tambien con el heroe quieto) junta TODO lo que
// corre entre el cierre de un cuadro y la apertura del siguiente: el cierre de las listas del PVR, la lectura
// del control por el bus maple, la conversion del lienzo, la subida a la VRAM y el pintado del quad. Son
// costos de naturaleza distinta y se atacan distinto, asi que la tarjeta necesita el numero de cada uno.
//
// El shim no puede importar `d2gamescreen` (esa cadena cierra un ciclo con d2app), asi que el arranque de la
// consola deja aca el puente. Sin puente (la PC) `MarcaFina159f` es nil y `marcarEbiten159f` no hace nada.

var MarcaFina159f func(id int)

// Los tramos del shim, EN ESTE ORDEN: se suman a `d2gamescreen.MarcasEbitenBase159f`.
const (
	MarcaEbitenLeerControl159f = iota // CerrarCuadro + LeerControl (el maple)
	MarcaEbitenConvertir159f          // RGBA -> RGB565
	MarcaEbitenSubir159f              // a la VRAM
	MarcaEbitenPintar159f             // el quad
	MarcaEbitenListas159f             // PvrListFinish + PvrSceneFinish
	MarcaEbitenResto159f              // la espera del cuadro y el resto del loop
)

// marcarEbiten159f: cierra el tramo abierto y abre `id` (no-op sin puente).
func marcarEbiten159f(id int) {
	if MarcaFina159f != nil {
		MarcaFina159f(id)
	}
}

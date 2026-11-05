package d2gamescreen

// DC-002.159h -- LAS CUENTAS DEL CAMINO HSV, EL PUENTE.
//
// POR QUE EXISTE. El defecto de la tarjeta era que `runtime.mapassign` llamaba a un hasher que era DATO
// (`ebiten._62_7float64..hash..f` en `.rodata`) y el SH-4 moria con la matriz HSV de la sombra del
// compuesto. La corrida de aceptacion no muere -- pero "no muere" no es prueba de que el camino se haya
// recorrido: si nadie pidiera la matriz, la corrida tambien pasaria. Las cuentas viven donde estan el
// mapa (`d2core/d2render/ebiten`) y la sombra (`d2core/d2asset`), y la pantalla no puede importar el
// paquete del renderer (el renderer es un parametro, no una dependencia del juego), asi que el puente es
// el mismo patron que `SondaDelRecolector159d`: la consola lo instala en su main y la linea `CUADRO` lo
// imprime. En la PC el puente queda nil y la linea no lleva los campos.
//
// QUE SIGNIFICA LA LINEA
//   `hsv=pedidas/aciertos/fallos`  dibujos que pidieron la matriz HSV, lecturas y ESCRITURAS de
//                                  `hsvCache`. Las escrituras son `mapassign`, el que mataba el SH-4.
//   `sombras=N`                    sombras del compuesto dibujadas (cada una apila brillo 0).

// CuentasDelCaminoHSV159h lo instala el arranque de la consola: devuelve (pedidas, aciertos, fallos) del
// camino HSV y cuantas sombras del compuesto se dibujaron. nil en la PC.
//
//nolint:gochecknoglobals // puente de instrumentacion de la consola
var CuentasDelCaminoHSV159h func() (pedidas, aciertos, fallos, sombras int)

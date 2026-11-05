package d2maprenderer

// DC-002.159o -- LA PALANCA QUE FUERZA EL CAMINO DEL TINTE EN `renderShadow` (instrumento).
//
// POR QUE EXISTE. El defecto que mataba el SH-4 en DC-002.159m vive en el camino
// `renderShadow -> PushColor(color.RGBA{255,255,255,160}) -> pintarTile -> Render -> fillDrawImageOptions ->
// colorToColorM`. Con la luz del mapa PRENDIDA ese camino no se recorre: `renderShadow` se va por
// `renderShadedImage` (la sombra se mezcla con el mapa de luz y no empuja ningun color). Medido en DC-002.159n:
// en cinco corridas de 150 s el estado de la superficie llego al dibujo CON color UNA sola vez, la primera
// llamada de la corrida. O sea que la corrida de cierre pasaba sin pisar el defecto.
//
// Esta palanca elige la rama del `PushColor` aunque la luz este prendida. El precio, dicho: las sombras del `.dt1`
// salen SIN luz (blanco al 63 % en vez del tinte que les toca), asi que NO es la configuracion de la corrida de
// cierre: es la configuracion que ejerce el camino, y se usa para medir.
//
// ES ADITIVA: sin `ForzarCaminoDelTinte159o(true)` el renderer hace exactamente lo de antes.

// forzarCaminoDelTinte159o: si la rama del `PushColor` de `renderShadow` se elige aunque la luz este prendida.
//
//nolint:gochecknoglobals // palanca del instrumento: la mueve el arranque del Gate 0
var forzarCaminoDelTinte159o bool

// sombrasPorElTinte159o: cuantas sombras del `.dt1` se pintaron por la rama del `PushColor` desde que arranco el
// proceso. Es la prueba positiva de que la palanca se ejercio (sin esta cuenta, "la corrida no murio" tambien
// seria cierto si la rama no se hubiera recorrido).
//
//nolint:gochecknoglobals // contador del instrumento
var sombrasPorElTinte159o uint64

// ForzarCaminoDelTinte159o prende (o apaga) la palanca.
func ForzarCaminoDelTinte159o(si bool) { forzarCaminoDelTinte159o = si }

// CaminoDelTinteForzado159o: si la palanca esta puesta.
func CaminoDelTinteForzado159o() bool { return forzarCaminoDelTinte159o }

// SombrasPorElTinte159o: cuantas sombras se pintaron por la rama del tinte.
func SombrasPorElTinte159o() uint64 { return sombrasPorElTinte159o }

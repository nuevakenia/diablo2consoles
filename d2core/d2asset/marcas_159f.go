package d2asset

// DC-002.159f -- EL PUENTE DE LAS MARCAS FINAS DENTRO DE d2asset (ver d2game/d2gamescreen/marcas_159f.go).
//
// El dibujo de una animacion de `.dcc` hace tres cosas que se pueden pagar muy distinto: decodificar la
// direccion nueva (el `.dcc` descomprimido, ~12 KB por direccion), armar/rellenar la superficie del cuadro
// (la expansion de paleta a RGBA) y todo lo demas (las pilas de transformacion, el compuesto). Las fases de
// DC-002.159d juntan las tres en `heroe`, y sin partirlas no se sabe cual atacar.
//
// `d2asset` no puede importar `d2gamescreen` (lo importa al reves), asi que el arranque de la consola deja
// aca el puente. Sin puente (la PC) `MarcaFina159f` es nil y `marcarAsset159f` no hace nada.

var MarcaFina159f func(id int)

// Los tramos de `d2asset`, EN ESTE ORDEN: se suman a `d2gamescreen.MarcasAssetBase159f`.
const (
	MarcaAssetDecod159f = iota // decodificar una direccion del `.dcc`
	MarcaAssetSup159f          // armar y rellenar la superficie del cuadro
	MarcaAssetResto159f        // lo demas del dibujo
)

// marcarAsset159f: cierra el tramo abierto y abre `id` (no-op sin puente).
func marcarAsset159f(id int) {
	if MarcaFina159f != nil {
		MarcaFina159f(id)
	}
}

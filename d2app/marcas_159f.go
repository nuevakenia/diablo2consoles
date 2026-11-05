package d2app

// DC-002.159f -- EL PUENTE DE LAS MARCAS FINAS (ver d2game/d2gamescreen/marcas_159f.go).
//
// La cuenta vive en `d2gamescreen` (que es la que tiene la sonda del recolector y la linea del informe), pero
// `d2app` no puede importarla (d2gamescreen importa d2app). Asi que el arranque de la consola
// (`console/dc/poc/gate0-pueblo/main.go`) deja aca la funcion que marca: `MarcaFina159f(base + id)`.
//
// SIN PUENTE (la PC) esto no hace nada: `MarcaFina159f` queda nil y `marcarApp159f` es una comparacion.

var MarcaFina159f func(id int)

// Los tramos de `d2app`, EN ESTE ORDEN: se suman a `d2gamescreen.MarcasAppBase159f`. Si se agrega uno, va al
// final y se agrega tambien el nombre en `d2gamescreen/marcas_159f.go` (los dos lados se leen de un vistazo).
const (
	MarcaAppUi159f = iota
	MarcaAppInput159f
	MarcaAppGui159f
	MarcaAppTerm159f
	MarcaAppResto159f
)

// marcarApp159f: cierra el tramo abierto y abre `id` (no-op sin puente).
func marcarApp159f(id int) {
	if MarcaFina159f != nil {
		MarcaFina159f(id)
	}
}

//go:build dreamcast

package main

// DC-002.143 -- EL RITMO JUGABLE: las piezas del POC que no son de una etapa.

import (
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2automap"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2render/pvr"
)

// minimapaPintoEsteCuadro143: si el minimapa se pinto en ESTE cuadro (lo pone `dibujarElMinimapa`). Lo lee
// `marcarLoQueCambio139` para marcar su caja en la presentacion: un panel que se salteo no cambio un pixel y no
// hay nada que convertir, pero uno que si se pinto tiene que llegar a la pantalla.
var minimapaPintoEsteCuadro143 bool

// mapaSePintoEntero143: si en ESTE cuadro el mundo se repinto ENTERO (el camino de siempre o la copia que fallo).
// No es lo mismo que `mapa139PintadoEsteCuadro`, que tambien queda en "si" cuando se devuelven las cajas de los
// entes: devolver una caja deja el lienzo como estaba, pero un repintado entero PISA los paneles (el minimapa y el
// dialogo) con pixeles del mapa, y entonces los paneles tienen que volver a pintarse aunque su contenido no haya
// cambiado. Se prende en `dibujar` y lo lee la guarda de los paneles.
var mapaSePintoEntero143 bool

// elMundoEnteroSePinto143: los dos caminos que dejan pixeles de mapa debajo de los paneles: el repintado entero y el
// cuadro que camina (que repinta el mapa en las franjas que descubrio la copia, paneles incluidos).
func elMundoEnteroSePinto143() bool { return mapaSePintoEntero143 || scroll141Caminando }

// minimapaCuadrosSinPintar143: cuantos cuadros lleva el automapa con el marcador sin mover (ver `minimapaCada143`).
var minimapaCuadrosSinPintar143 int

// LOS CONTADORES DE POR QUE EL MINIMAPA NO SE PINTO Y DE DONDE VINO CADA MANCHA DE LOS PANELES. Se imprimen en la
// linea `JUEGO143: PANELES` cada `cadaInforme143` cuadros: sin esto, "el minimapa no se pinto" es una hipotesis.
var (
	minimapaSaltos143Quieto   int // no cambio nada (heroe en la misma celda de pantalla y modelo igual)
	minimapaSaltos143Cadencia int // caminando, pero le toca esperar su turno (minimapaCada143)
	minimapaSaltos143Mundo    int // el mundo se repinto entero o el cuadro camino: hay que pintarlo
	minimapaSaltos143Ente     int // un ente se metio en la caja del panel
	minimapaPintados143       int // cuadros en los que el panel se pinto
	panelPintados143          int // cuadros en los que el panel del dialogo se pinto
	panelSaltos143Quieto      int // el panel no cambio: no se pinto
	panelSaltos143Mundo       int // el panel no se pinto porque el mundo se repinto (hay que volver a pintarlo encima)
	panelSaltos143Ente        int // el panel no se pinto porque un ente entro en su caja
)

// devolverLaCopiaDelPanel143 / guardarLaCopiaDelPanel143: LA CAJA DEL AUTOMAPA GUARDADA.
//
// POR QUE EXISTE. El automapa se pinta ENCIMA de los entes, y medido en esta tarjeta un NPC esta dentro de su caja
// en 296 de 300 cuadros (Akara esta a tres celdas del heroe y la caja es de 279x225). Con la guarda sola, el panel
// tenia que repintarse esos 296 cuadros: 18.700 us cada uno para volver a poner los MISMOS pixeles que ya estaban.
//
// La copia cambia eso: cuando el panel se pinta se guarda su caja (279x225x4 = 251.100 B, en el monton de Go), y en
// el cuadro en que el unico motivo para repintar es que un ente se metio adentro se DEVUELVE la copia (una copia
// por filas, ~2,2 ms medidos con el mismo costo por pixel que el relleno de la caja) en vez de repintar el panel
// entero (18,7 ms). Los pixeles que quedan en el lienzo son los mismos que dejaria el repintado: la copia se tomo
// DESPUES de pintar el panel, con el marcador incluido, y el marcador no se movio (si se hubiera movido, se repinta).
var (
	copiaDelPanel143   []byte
	copiaDelPanel143X0 int
	copiaDelPanel143Y0 int
	copiaDelPanel143X1 int
	copiaDelPanel143Y1 int
)

// guardarLaCopiaDelPanel143: guarda la caja del panel del lienzo (una copia por filas: la caja es un rectangulo).
func guardarLaCopiaDelPanel143(sup *pvr.Surface, caja d2automap.Rect) {
	if sup == nil || sup.Imagen() == nil {
		copiaDelPanel143 = nil

		return
	}

	anchoLienzo, altoLienzo := sup.GetSize()
	x0, y0, x1, y1 := caja.X0, caja.Y0, caja.X1, caja.Y1

	if x0 < 0 {
		x0 = 0
	}
	if y0 < 0 {
		y0 = 0
	}
	if x1 > anchoLienzo {
		x1 = anchoLienzo
	}
	if y1 > altoLienzo {
		y1 = altoLienzo
	}
	if x0 >= x1 || y0 >= y1 {
		copiaDelPanel143 = nil

		return
	}

	filas := y1 - y0
	columnas := (x1 - x0) * 4

	if cap(copiaDelPanel143) < filas*columnas {
		copiaDelPanel143 = make([]byte, filas*columnas)
	} else {
		copiaDelPanel143 = copiaDelPanel143[:filas*columnas]
	}

	pixeles := sup.Imagen().Pixeles()

	for f := 0; f < filas; f++ {
		de := (y0 + f) * anchoLienzo * 4
		copy(copiaDelPanel143[f*columnas:(f+1)*columnas], pixeles[de+x0*4:de+x1*4])
	}

	copiaDelPanel143X0, copiaDelPanel143Y0 = x0, y0
	copiaDelPanel143X1, copiaDelPanel143Y1 = x1, y1
}

// devolverLaCopiaDelPanel143: escribe la copia de vuelta en la caja (si es la misma caja). Devuelve false si no hay
// copia o si la caja cambio (entonces el que llama tiene que repintar el panel).
func devolverLaCopiaDelPanel143(sup *pvr.Surface, caja d2automap.Rect) bool {
	if sup == nil || sup.Imagen() == nil || copiaDelPanel143 == nil {
		return false
	}
	if caja.X0 != copiaDelPanel143X0 || caja.Y0 != copiaDelPanel143Y0 ||
		caja.X1 != copiaDelPanel143X1 || caja.Y1 != copiaDelPanel143Y1 {
		return false
	}

	anchoLienzo, _ := sup.GetSize()
	filas := copiaDelPanel143Y1 - copiaDelPanel143Y0
	columnas := (copiaDelPanel143X1 - copiaDelPanel143X0) * 4
	pixeles := sup.Imagen().Pixeles()

	for f := 0; f < filas; f++ {
		de := (copiaDelPanel143Y0+f)*anchoLienzo*4 + copiaDelPanel143X0*4
		copy(pixeles[de:de+columnas], copiaDelPanel143[f*columnas:(f+1)*columnas])
	}

	return true
}

// cuadrosDelMinimapa143: cada cuantos cuadros se repinta el automapa mientras el heroe camina. 3 = 20 Hz al ritmo
// de 60 cuadros por segundo, que es la diferencia entre un cuadro de 20 ms y uno de 37 (medido: el panel cuesta
// 18,7 ms cuando se pinta). Con un valor raro o "no" se repinta siempre.
func cuadrosDelMinimapa143() int {
	if minimapaCada143 == "2" || minimapaCada143 == "3" || minimapaCada143 == "4" {
		if minimapaCada143 == "2" {
			return 1
		}
		if minimapaCada143 == "4" {
			return 3
		}

		return 2
	}

	return 0
}

// dialogoPintado143 / ultimoPanelQuieto143 / panelMarcado143: el panel del dialogo o de la quest que se pinto (y se
// marco) por ultima vez. Con `panelQuieto143` el panel NO se vuelve a pintar mientras sea el mismo objeto: se arma
// una vez y no cambia, y es opaco (los pixeles de abajo no se ven).
var (
	dialogoPintado143    *panelDelDialogo
	ultimoPanelQuieto143 *panelDelDialogo
	panelMarcado143      *panelDelDialogo
)

// algunEnteEnLaCajaDe: si la caja de algun ente del cuadro (jugador, bicho, objeto del piso o NPC) cruza `caja`.
//
// POR QUE EXISTE. El minimapa (y el HUD) se pintan ENCIMA del mundo, y desde esta tarjeta el minimapa se saltea
// los cuadros en los que nada cambio (ver `minimapaCambio143` en automap_130.go). "Nada cambio" no es solo que
// el heroe no se haya movido: un NPC que camina por el sitio del panel queda tapado por el panel, asi que en el
// cuadro en que uno de sus pixeles cae adentro de la caja el panel TIENE que volver a pintarse. La cuenta son
// cuatro rectangulos por ente contra uno solo (no se recorre un pixel).
func algunEnteEnLaCajaDe(caja d2automap.Rect) bool {
	for _, p := range jugadores {
		if p != nil && cajaCruza141(p.Caja, caja.X0, caja.Y0, caja.X1, caja.Y1) {
			return true
		}
	}

	for _, b := range bichos {
		if b != nil && cajaCruza141(b.Caja, caja.X0, caja.Y0, caja.X1, caja.Y1) {
			return true
		}
	}

	for _, n := range npcsDelPueblo {
		if n != nil && cajaCruza141(n.Caja, caja.X0, caja.Y0, caja.X1, caja.Y1) {
			return true
		}
	}

	for _, it := range suelo {
		if it != nil && cajaCruza141(it.Caja, caja.X0, caja.Y0, caja.X1, caja.Y1) {
			return true
		}
	}

	return false
}

// algunEnteEnLaCajaDelPanel143: lo mismo, contra la caja del panel del dialogo o de la quest. El panel se pinta
// ULTIMO y es opaco: si un ente se mete en su caja, el panel tiene que volver a pintarse encima o el sprite queda
// asomando dentro del panel.
func algunEnteEnLaCajaDelPanel143() bool {
	if elDialogo == nil {
		return false
	}

	return algunEnteEnLaCajaDe(d2automap.Rect{
		X0: elDialogo.X0,
		Y0: elDialogo.Y0,
		X1: elDialogo.X0 + elDialogo.Ancho,
		Y1: elDialogo.Y0 + elDialogo.Alto,
	})
}

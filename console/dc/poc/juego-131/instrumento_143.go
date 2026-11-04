//go:build dreamcast

package main

// DC-002.143 -- LO QUE EL CUADRO PAGA FUERA DE LAS ETAPAS (el tramo que quedo en `sinEtapa`).
//
// QUE CONTESTA. Con el instrumento por cuadro apagado (`MEDIR_CUADRO=no`, la demo jugable) el cuadro mide
// 124-131 ms y la suma de las etapas de DC-002.134 da ~30 ms: el resto -- que esa linea llama `sinEtapa` y
// en la corrida instrumentada era el 43 % del cuadro -- no tenia nombre. Este archivo le pone nombre a cada
// pedazo que no es una etapa de dibujado:
//
//	revelar   el `revelarElMinimapa` que corre ANTES del cronometro del minimapa (asi que no estaba en `minimapa`)
//	censos    `censoDelTramo` + `memoria` del final del cuadro (el pico del monton y la tabla de carga)
//	anim      `estadoDeLaAnimacion` (el anuncio de la animacion por personaje)
//	informes  los tres informes de cierre de cuadro (JUEGO132 FPS, JUEGO139, JUEGO141)
//	mvp       `anotarElCuadroDelMVP`
//	marcar    `marcarLoQueCambio139` (los bloques que la presentacion tiene que convertir)
//	recortes  (solo el cuadro que camina) `armarLosRecortes141`: la lista de rectangulos del repintado
//	renderMap (solo el cuadro que camina) el `Render` del mapa CON los recortes puestos
//
// LA LINEA (promedio por cuadro del tramo, en microsegundos):
//
//	JUEGO143: CUADRO cuadros= delTramo= revelar= censos= anim= informes= mvp= marcar= recortes= renderMap=
//	          dibujar= etapas134= resto=
//
// `etapas134` es la suma de las etapas de DC-002.134 del mismo tramo (sin contar `medir`, que va adentro) y
// `resto` es lo que queda sin nombre despues de descontar las etapas, las marcas de aca y `recortes`/
// `renderMap` (que son parte de la etapa `mapa`): si `resto` es grande, todavia hay algo que no se midio.
//
// LA PALANCA (knobs.go): `etapas143 = "si" | "no"`. Cuesta dos llamadas a `d2cUs` por marca y no toca el
// dibujo: el cuadro que queda en pantalla es el mismo con la palanca prendida o apagada.

// Las marcas. El orden es el orden en que el cuadro las paga.
const (
	e143Revelar = iota
	e143Censos
	e143Anim
	e143Informes
	e143Mvp
	e143Marcar
	e143Recortes
	e143RenderMapa
	e143Cuenta
)

var nombres143 = [e143Cuenta]string{
	"revelar", "censos", "anim", "informes", "mvp", "marcar", "recortes", "renderMapa",
}

var (
	us143      [e143Cuenta]int64 // microsegundos acumulados de cada marca en el tramo
	usDib143   int64             // el `dibujar` entero del tramo (lo mismo que mide DC-002.134)
	cuadros143 int
)

// cron143: el arranque de una marca (microsegundos). Devuelve 0 con la palanca apagada, que es lo que hace que
// `marca143` no sume nada.
func cron143() int64 {
	if etapas143 != "si" {
		return 0
	}

	return int64(d2cUs())
}

// marca143: cierra una marca abierta con `cron143`.
func marca143(etapa int, desde int64) {
	if etapas143 != "si" || desde <= 0 {
		return
	}

	us := int64(d2cUs()) - desde
	if us < 0 { // el contador de 32 bits da la vuelta cada ~71 minutos
		return
	}

	us143[etapa] += us
}

// anotarCuadro143: el total del `dibujar` del tramo (para poder restar) y, cada `cuadrosDelInforme132`
// cuadros, la linea.
func anotarCuadro143(usDibujar int64) {
	if etapas143 != "si" || usDibujar < 0 {
		return
	}

	usDib143 += usDibujar
	cuadros143++

	if cuadro%cuadrosDelInforme132 == 0 {
		informe143()
		informeDeLosPaneles143()
	}
}

// informe143: la linea con el promedio por cuadro de cada marca y lo que queda sin nombre.
func informe143() {
	if cuadros143 == 0 {
		return
	}

	n := int64(cuadros143)

	// las etapas de DC-002.134 del MISMO tramo (sin `medir`, que va adentro de las que lo llaman)
	sumaEtapas := int64(0)
	for i := 0; i < int(e134Cuenta); i++ {
		if i == e134Instrumento {
			continue
		}

		sumaEtapas += usEtapas134[i]
	}

	resto := usDib143 - sumaEtapas - us143[e143Revelar] - us143[e143Censos] - us143[e143Anim] -
		us143[e143Informes] - us143[e143Mvp] - us143[e143Marcar]
	if resto < 0 {
		resto = 0
	}

	println("JUEGO143: CUADRO cuadros=", cuadro, "delTramo=", cuadros143,
		"revelar=", us143[e143Revelar]/n, "censos=", us143[e143Censos]/n,
		"anim=", us143[e143Anim]/n, "informes=", us143[e143Informes]/n,
		"mvp=", us143[e143Mvp]/n, "marcar=", us143[e143Marcar]/n,
		"recortes=", us143[e143Recortes]/n, "renderMapa=", us143[e143RenderMapa]/n,
		"etapas134=", sumaEtapas/n, "dibujar=", usDib143/n, "resto=", resto/n,
		"palanca=", etapas143)

	for i := 0; i < int(e143Cuenta); i++ {
		us143[i] = 0
	}

	usDib143, cuadros143 = 0, 0
}

// nombre143 y compania: por si algun dia una linea necesita el nombre de una marca.
func nombre143(etapa int) string {
	if etapa < 0 || etapa >= int(e143Cuenta) {
		return "?"
	}

	return nombres143[etapa]
}

// informeDeLosPaneles143: POR QUE el automapa y el panel se pintaron o no. Cada 300 cuadros (con la demo encendida)
// deja los motivos contados: sin esto, "el panel se saltea los cuadros quietos" es una hipotesis, y la diferencia
// entre un cuadro de 20 ms y uno de 37 esta justo aca.
func informeDeLosPaneles143() {
	if etapas143 != "si" || cuadro%cadaInforme143() != 0 {
		return
	}

	println("JUEGO143: PANELES cuadro=", cuadro,
		"minimapaPintados=", minimapaPintados143,
		"minimapaSaltosQuieto=", minimapaSaltos143Quieto,
		"minimapaSaltosCadencia=", minimapaSaltos143Cadencia,
		"minimapaPintadoPorMundo=", minimapaSaltos143Mundo,
		"minimapaPintadoPorEnte=", minimapaSaltos143Ente,
		"panelPintados=", panelPintados143,
		"panelSaltosQuieto=", panelSaltos143Quieto,
		"panelPintadoPorMundo=", panelSaltos143Mundo,
		"panelPintadoPorEnte=", panelSaltos143Ente,
		"cada=", minimapaCada143, "quietos=", minimapaCambio143)

	minimapaPintados143, minimapaSaltos143Quieto, minimapaSaltos143Cadencia = 0, 0, 0
	minimapaSaltos143Mundo, minimapaSaltos143Ente = 0, 0
	panelPintados143, panelSaltos143Quieto, panelSaltos143Mundo, panelSaltos143Ente = 0, 0, 0, 0
}

// cadaInforme143: cada cuantos cuadros salen las lineas de evidencia POR TRAMO (`JUEGO134: ETAPAS`,
// `JUEGO137`, `JUEGO139`, `JUEGO141`, `JUEGO143`).
//
// EN LA DEMO CADA 300 Y NO CADA 30, Y ESTA MEDIDO: esas cinco lineas son ~1.400 B por el puerto serie, y el
// puerto va a 11.520 B/s, o sea 120 ms por cada 30 cuadros = 4 ms por cuadro de INSTRUMENTO. Lo que la
// evidencia necesita es un tramo representativo cada 300 cuadros (los mismos del resumen de la demo); lo que
// el jugador necesita es que ese numero no le cueste un cuadro de cada treinta. Fuera de la demo no cambia
// nada (cada 30, como siempre).
func cadaInforme143() int {
	if demoKnob == "si" {
		return cuadrosDelResumenDemo
	}

	return cuadrosDelInforme132
}

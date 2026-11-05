package d2asset

// DC-002.159f -- LA SUPERFICIE REUSADA DEL CUADRO DE ANIMACION (solo consola).
//
// QUE ES. Guardar los cuadros de una animacion `.dcc` en UNA sola superficie: se pide con un margen del 25%
// sobre la caja del primer cuadro y despues se reusa mientras los cuadros entren
// (`DCCAnimation.armarLaSuperficieDelCuadro`). El cuadro se carga en la esquina de arriba a la izquierda y el
// que dibuja RECORTA (`Animation.Render` -> `Surface.RenderRecorte`), asi que el sobrante no se pinta.
//
// CUANTO AHORRA (medido en el Gate 0, 390 cuadros de caminar). Antes se pedia una superficie nueva cada vez
// que la caja del cuadro no era identica a la que habia, y caminando cada giro cambia la caja:
// `sup=105/157.032 B`. Con el reuso: `sup=15/36.180 B`, y el tramo del heroe baja de 3.501 a 1.575 B/cuadro.
//
// POR QUE NO SE ENTREGO EN 159f. Alli el recorte se decidia con una ASERCION de interfaz opcional
// (`target.(superficieConRecorte159f)`) y en este compilador (gccgo del sh4) esa asercion NO ACIERTA: el metodo
// existe en el tipo (`sh-elf-nm` lista `pvr.Surface.RenderRecorte159f`) y sin embargo la consola contestaba
// `recorte=false`, asi que el recorte nunca se hacia, se dibujaba la superficie entera y quedaba el borde
// fantasma del sobrante. Quedo apagada por defecto, con el por que escrito y la tarjeta de seguimiento.
//
// DC-002.159g LO ENTREGA POR EL CONTRATO. `RenderRecorte` es ahora un metodo de `d2interface.Surface` (como
// `Render` y `RenderSection`), asi que la llamada es despacho normal de interfaz y no hay asercion que pueda
// fallar. La palanca la prende SOLO el arranque de la consola (`ArrancarComoConsola`, en consola.go): en la PC
// las superficies siguen siendo exactas y el camino del recorte no se toca.
var superficieReusada159f bool

// UsarLaSuperficieReusada159 prende (o apaga) el reuso de la superficie del cuadro de animacion. Apagada
// `armarLaSuperficieDelCuadro` pide una superficie exacta por caja, como antes.
func UsarLaSuperficieReusada159(v bool) { superficieReusada159f = v }

// SuperficieReusada159 dice si el reuso esta prendido. Lo imprime la linea de estado de la corrida para que la
// evidencia no dependa de creer.
func SuperficieReusada159() bool { return superficieReusada159f }

// LOS CONTADORES DE LA EVIDENCIA (DC-002.159g). Que el reuso este prendido no prueba que el recorte se haga: lo
// prueba que se haya LLAMADO. Estos dos numeros van en la linea de estado del heroe (`rec=dibujos/cargas`) y son
// la unica forma de distinguir "recorto" de "dibujo el sobrante" sin creerle a la captura.
var (
	// recortesDelCuadro159: cuadros dibujados con `RenderRecorte` (la superficie era mas grande que la caja).
	recortesDelCuadro159 int
	// cargasPorFilas159: cuadros cargados FILA POR FILA en la superficie grande. OJO: este camino todavia se
	// elige con una asercion de interfaz opcional (`cargarCuadroEnSuperficie`), asi que el numero tambien sirve
	// de sonda: si se queda en 0 con el reuso prendido, la carga se esta escribiendo como bloque contiguo en un
	// lienzo mas ancho (las filas quedan corridas) y la captura lo va a mostrar como basura.
	cargasPorFilas159 int
)

// RecortesDelCuadro159 devuelve (cuadros recortados al dibujar, cuadros cargados fila por fila) del proceso.
func RecortesDelCuadro159() (dibujos, cargas int) {
	return recortesDelCuadro159, cargasPorFilas159
}

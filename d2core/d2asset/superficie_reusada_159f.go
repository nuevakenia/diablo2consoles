package d2asset

// DC-002.159f -- LA SUPERFICIE REUSADA DEL CUADRO DE ANIMACION (solo consola). APAGADA POR DEFECTO.
//
// QUE ES. Guardar los cuadros de una animacion `.dcc` en UNA sola superficie: se pide con un margen del 25%
// sobre la caja del primer cuadro y despues se reusa mientras los cuadros entren
// (`DCCAnimation.armarLaSuperficieDelCuadro`). El cuadro se carga en la esquina de arriba a la izquierda y el
// que dibuja RECORTA (`Animation.Render` -> `Surface.RenderRecorte159f`), asi que el sobrante no se pinta.
//
// CUANTO AHORRA (medido en el Gate 0, 390 cuadros de caminar). Antes se pedia una superficie nueva cada vez
// que la caja del cuadro no era identica a la que habia, y caminando cada giro cambia la caja:
// `sup=105/157.032 B`. Con el reuso: `sup=15/36.180 B`, y el tramo del heroe baja de 3.501 a 1.575 B/cuadro.
// Sobre el caminar entero eran ~2.000 B/cuadro de basura menos (4.069 contra 6.0xx medidos).
//
// POR QUE ESTA APAGADA IGUAL. No se puede entregar: en ESTE compilador (gccgo del sh4) la asercion de la
// interfaz opcional NO ACIERTA. Medido con una prueba en `renderElCuadro159f`: el metodo existe en el tipo
// (`sh-elf-nm pvr.gox` lista `pvr.Surface.RenderRecorte159f`) y sin embargo `target.(superficieConRecorte159f)`
// devuelve false en la consola, asi que el recorte nunca se hace, se dibuja la superficie entera y queda el
// borde fantasma del sobrante (confirmado en las capturas: el sprite sale con franjas de otros cuadros).
// Y ademas el cambio de layout del compuesto hace que algunos builds mueran en el primer cuadro del heroe
// (el defecto que la sonda de DC-002.159b ya tenia anotado).
//
// Con la palanca apagada el camino es el de siempre -- una superficie exacta por caja -- y lo unico que queda
// de este intento es el codigo, listo para cuando el recorte se pueda hacer por el CONTRATO
// (`d2interface.Surface`) en vez de por una asercion.
var superficieReusada159f bool

// UsarLaSuperficieReusada159 prende (o apaga) el reuso de la superficie del cuadro de animacion. Apagada
// --como esta-- `armarLaSuperficieDelCuadro` pide una superficie exacta por caja, como antes.
func UsarLaSuperficieReusada159(v bool) { superficieReusada159f = v }

// SuperficieReusada159 dice si el reuso esta prendido. Lo imprime la linea de estado de la corrida para que la
// evidencia no dependa de creer.
func SuperficieReusada159() bool { return superficieReusada159f }

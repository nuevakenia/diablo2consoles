// Package maple es el BACKEND DE ENTRADA DE LA DREAMCAST: la implementacion de d2interface.InputService sobre
// el control enchufado al bus maple.
//
// POR QUE EXISTE (DC-002.71). El motor pregunta por la entrada con d2interface.InputService (9 metodos:
// CursorPosition, InputChars, IsKeyPressed, IsKeyJustPressed, IsKeyJustReleased, IsMouseButtonPressed,
// IsMouseButtonJustPressed, IsMouseButtonJustReleased y KeyPressDuration). Hasta esta tarjeta la unica
// implementacion era d2core/d2input/ebiten, que pregunta a Ebitengine o sea al teclado y al mouse de la PC.
// En la consola no hay ni teclado ni mouse: hay un control. Este paquete es la pieza que traduce el control a
// lo que el motor ya sabe preguntar, sin cambiarle una linea al motor.
//
// LA TRADUCCION ES LA DEL SHIM, COPIADA (no inventada). El backend del shim de Ebitengine para la consola
// (console/dc/engine/ebiten/v2/control.go, medido en DC-002.11 y usado por el POC del personaje desde
// DC-002.38) ya resolvio que hace cada boton del control:
//
//	la palanca y la cruceta -> el CURSOR           (el juego de PC se maneja con el mouse)
//	el boton A              -> el clic IZQUIERDO + Enter   ("camina hasta aca" / aceptar)
//	el boton B              -> el clic DERECHO  + Escape   (la skill / volver)
//	el boton X              -> el clic del MEDIO + X
//	el boton Y              -> Y                   (el golpe)
//	el boton Start          -> Enter               (arrancar, el boton de siempre)
//	los gatillos L y R      -> Q y E               (los atajos que en la PC estan al lado de la mano)
//	la cruceta              -> las FLECHAS         (con ellas se navegan los menus y camina el heroe: es lo
//	                                                que lee d2game/d2player/kbd_grid.go con KeyUp/Down/Left/Right)
//
// La tabla vive en Tabla (traduccion.go) y es la fuente unica: los metodos la recorren, la prueba de la
// consola la imprime y la evidencia se genera de ahi. Una tabla escrita dos veces se desincroniza sola.
//
// QUE NO PUEDE SER FIEL, DECLARADO (no escondido):
//   - InputChars: en la consola no hay teclado, asi que el motor no recibe caracteres imprimibles. Los campos
//     de texto (el nombre del personaje) no se pueden llenar con el control. Devuelve nil, no una cadena
//     vacia inventada.
//   - Los botones Z, C y D del control no tienen tecla ni boton de mouse asignado: en el shim Z es el boton
//     "Back" del joystick y C/D no se usan. Aca quedan SIN mapear (y la tabla lo dice con Teclas vacia), pero
//     se pueden leer crudos con Botones()/BotonesApretados() para la tarjeta que conecte d2gamepad.
//   - El mouse no existe: el cursor es una posicion virtual que mueve la palanca/cruceta, igual que en el
//     shim. No hay rueda (el motor la pregunta por fuera de esta interfaz).
//   - La palanca de la consola tiene UN solo stick: mueve el cursor (como el mouse de la PC). El stick derecho
//     del gamepad de PC, que en d2gamepad mueve el cursor, aca no existe.
//
// COMO SE USA (una vez por cuadro, antes de que el motor pregunte):
//
//	servicio := maple.NuevoServicio(640, 480)
//	for cada cuadro {
//	    servicio.ActualizarDesdeElControl()   // lee el maple y traduce
//	    // de aca en adelante el motor pregunta IsKeyPressed, CursorPosition, ...
//	}
//
// Y eso es lo que hace la prueba de la consola console/dc/poc/input-maple (DC-002.71): imprime por la
// consola serie los bits del control y las teclas traducidas cada vez que algo cambia.
package maple

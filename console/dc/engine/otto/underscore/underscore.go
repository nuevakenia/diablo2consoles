// Package underscore: el agregado de underscore.js para el interprete de JavaScript.
//
// PARA QUE SE USA: d2script/engine.go lo importa con un guion bajo (`_ "...otto/underscore"`) nada mas que
// para que, cuando el interprete arranque, tenga las funciones de underscore.js disponibles. No se llama a
// nada de aca: el solo hecho de importarlo es el uso.
//
// En la consola el interprete no esta (ver el paquete otto del motor), asi que este paquete no tiene nada:
// esta para que el import compile. No hace falta que registre nada porque no hay interprete donde
// registrarlo, y no se miente diciendo que si.
package underscore

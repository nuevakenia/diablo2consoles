//go:build dreamcast

package maple

// DC-002.113 -- EL ESTADO DEL MAPLE PUESTO POR EL SHIM (cuando el emulador no tiene mando).
//
// POR QUE EXISTE. La corrida del criterio de DC-002.113 aprieta la cruceta con el teclado virtual del host
// (`scripts/console/teclas-virtuales.py` por /dev/uinput) y es FLYCAST el que arma el estado del control en el
// bus maple: ahi no se inyecta nada y la evidencia lo dice. Pero si el emulador no toma ese teclado, la prueba
// tiene que poder correr igual, y el camino honrado es el mismo que usa la consola para todo lo demas: el SHIM.
//
// COMO FUNCIONA. `d2c_control_inyectar` (console/dc/std/csrc/control_dc.c) deja un estado a un costado y
// `d2c_control_leer` lo devuelve en vez de leer el bus; `d2c_control_buscar` contesta que hay control. O sea que
// el estado entra por la MISMA puerta que un control de verdad: el servicio lo traduce con la tabla de siempre
// (bits -> teclas del motor) y el juego pregunta por las teclas. Lo unico que cambia es de donde salieron los
// bits, y eso el POC lo imprime en la evidencia.
//
// EL PUNTERO CRUZA COMO uintptr Y NO COMO unsafe.Pointer: el mismo motivo que en control.go (el ABI ambiguo y el
// runtime de la consola sin el descriptor del tipo).

//extern d2c_control_inyectar
func d2cControlInyectar(botones, palancaX, palancaY, gatilloL, gatilloR, encendido uintptr) int

//extern d2c_control_inyectando
func d2cControlInyectando() int

// InyectarBotones: desde ahora, `ActualizarDesdeElControl` lee ESTE estado en vez del bus. Los ejes analogos y los
// gatillos quedan en cero (la cruceta es digital: son bits).
func (s *Servicio) InyectarBotones(botones uint32) {
	d2cControlInyectar(uintptr(botones), 0, 0, 0, 0, 1)
}

// DejarDeInyectar: vuelve a leer el bus maple.
func (s *Servicio) DejarDeInyectar() {
	d2cControlInyectar(0, 0, 0, 0, 0, 0)
}

// Inyectando: si el estado lo esta poniendo el shim (para que la evidencia lo pueda decir).
func (s *Servicio) Inyectando() bool {
	return d2cControlInyectando() == 1
}

// ============================ DC-002.114: LA INYECCION, PUERTO POR PUERTO ============================
//
// POR QUE. El cooperativo de esta tarjeta necesita DOS estados a la vez: el del jugador 1, que viene del bus (el
// teclado del emulador en el puerto A), y el del jugador 2, que no tiene aparato en el puerto B. La inyeccion de
// DC-002.113 es UNA sola para todo el bus (`d2c_control_inyectar`), asi que encenderla para el jugador 2 tambien
// pisaria al jugador 1. Las tres de abajo son la misma idea, con el puerto como primer argumento.

//extern d2c_control_inyectar_puerto
func d2cControlInyectarPuerto(puerto, botones uintptr, palancaX, palancaY, gatilloL, gatilloR, encendido uintptr) int

//extern d2c_control_inyectando_puerto
func d2cControlInyectandoPuerto(puerto uint) int

// InyectarBotonesEn: desde ahora `ActualizarDesdeElControlEn(puerto)` lee ESTE estado en vez del bus, y solo para
// ese puerto.
func (s *Servicio) InyectarBotonesEn(puerto int, botones uint32) {
	d2cControlInyectarPuerto(uintptr(puerto), uintptr(botones), 0, 0, 0, 0, 1)
}

// DejarDeInyectarEn: ese puerto vuelve a leer el bus maple.
func (s *Servicio) DejarDeInyectarEn(puerto int) {
	d2cControlInyectarPuerto(uintptr(puerto), 0, 0, 0, 0, 0, 0)
}

// InyectandoEn: si ese puerto lo pone el shim (la evidencia lo imprime: no es lo mismo un jugador con control que
// uno con guion).
func (s *Servicio) InyectandoEn(puerto int) bool {
	return d2cControlInyectandoPuerto(uint(puerto)) == 1
}

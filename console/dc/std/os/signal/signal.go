// Package signal: las senales del sistema operativo, en la parte que la consola puede tener.
//
// PARA QUE SE USA: d2app/app.go:156 arma el apagado ordenado -- cuando llega un Control-C, guardar y
// cerrar bien. Es cosa de la version de escritorio.
//
// EN LA CONSOLA NO HAY SENALES: la Dreamcast no tiene procesos ni terminal desde donde mandar un
// Control-C; el juego termina cuando se apaga la maquina o se reinicia. Asi que Notify se acuerda del
// canal y de las senales (para que el codigo siga su curso) pero NUNCA le va a llegar nada. No se finge
// una senal que no existe.
package signal

import "d2std/os"

// los canales y las senales que se pidieron (se acuerda, pero nadie las va a disparar)
var (
	canales  []chan<- os.Signal
	pedidas  []os.Signal
)

// Notify: pide que le avisen cuando llegue alguna de esas senales. En la consola no llega ninguna.
func Notify(c chan<- os.Signal, sig ...os.Signal) {
	canales = append(canales, c)
	pedidas = append(pedidas, sig...)
}

// Stop: deja de avisar por ese canal.
func Stop(c chan<- os.Signal) {
	for i, ch := range canales {
		if ch == c {
			canales = append(canales[:i], canales[i+1:]...)
			return
		}
	}
}

// Reset: vuelve al comportamiento de siempre (aca no hay nada que resetear).
func Reset(sig ...os.Signal) {}

// Ignore: ignora esas senales (aca no llega ninguna, asi que ya estan ignoradas).
func Ignore(sig ...os.Signal) {}

// Ignored: si esa senal esta ignorada (siempre si, por lo de arriba).
func Ignored(s os.Signal) bool { return true }

// CanalesPedidos y SenalesPedidas: lo que se pidio (lo usa el apagado ordenado para saber si tiene que
// hacer algo, y sirve para las pruebas del POC).
func CanalesPedidos() int { return len(canales) }

// SenalesPedidas: cuales se pidieron.
func SenalesPedidas() []os.Signal { return pedidas }

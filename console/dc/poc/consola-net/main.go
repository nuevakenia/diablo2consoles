//go:build dreamcast

// DC-002.8: el `net` de la consola -- la parte que ES de verdad y la que no existe.
//
// POR QUE ESTE POC EXISTE, y por que no alcanza una prueba en el host: el paquete `net` de la consola
// importa `time` (la firma de las esperas de una conexion es SetReadDeadline(time.Time), igual que en Go),
// y el `time` de la consola habla con KOS. O sea que `net` NO se puede compilar en el host: la unica
// manera de probar que el tubo en memoria anda es correrlo EN la consola. Es el mismo caso que `os`.
//
// QUE SE PRUEBA:
//   1. Armar y desarmar direcciones (JoinHostPort / SplitHostPort) contra los valores que da el `net` de
//      Go, MEDIDOS con un programa en el host y horneados aca. Dos de esos valores son contraintuitivos
//      (JoinHostPort("[::1]", "8080") da "[[::1]]:8080", y SplitHostPort(":7000") da host VACIO) y estaban
//      mal en la primera version escrita a mano: por eso el valor se mide, no se deduce.
//   2. El tubo en memoria (net.Pipe): que los datos crucen de una punta a la otra, y -- lo que mas
//      importa -- que al cerrar una punta la otra reciba el FIN DE ARCHIVO. Si eso no pasara, el que lee
//      se quedaria esperando para siempre y el juego no cerraria nunca (un cuelgue, no un error).
//   3. Que la parte que no hay (Listen/Dial) devuelva el error y no una conexion vacia.
package main

import (
	"io"
	"net"
)

//extern d2c_serial_init
func d2c_serial_init()

// ---------------------------------------------------------------- utilidades de la evidencia

var casos, bien, mal int

func check(nombre, esperado, dado string) {
	casos++
	if esperado == dado {
		bien++
		println("RED ok  ", nombre, "=", dado)
	} else {
		mal++
		println("RED FALLO", nombre, "esperaba", esperado, "y dio", dado)
	}
}

// main: la corrida de la evidencia.
func main() {
	d2c_serial_init()

	println("== net de la consola: la parte de verdad y la que no ==")

	// ---------------------------------------------------------------- las direcciones (medido en Go)
	// los esperados salen de correr el mismo caso con el net de Go en el host (no de lo que parece logico)
	check("JoinHostPort(127.0.0.1:8080)", "127.0.0.1:8080", net.JoinHostPort("127.0.0.1", "8080"))
	check("JoinHostPort(sin host)", ":80", net.JoinHostPort("", "80"))
	check("JoinHostPort(localhost)", "localhost:443", net.JoinHostPort("localhost", "443"))
	check("JoinHostPort(IPv6 sin corchetes)", "[::1]:8080", net.JoinHostPort("::1", "8080"))
	check("JoinHostPort(IPv6 con corchetes)", "[[::1]]:8080", net.JoinHostPort("[::1]", "8080"))
	check("JoinHostPort(battle.net)", "battle.net:6112", net.JoinHostPort("battle.net", "6112"))

	// SplitHostPort devuelve DOS valores mas el error, asi que se arma el texto y se compara ese texto
	comprobarSplit("127.0.0.1:8080", "127.0.0.1|8080|sin error")
	comprobarSplit("[::1]:8080", "::1|8080|sin error")
	comprobarSplit(":7000", "|7000|sin error")
	comprobarSplit("battle.net:6112", "battle.net|6112|sin error")
	comprobarSplit("sin-puerto", "||hay error")

	// ---------------------------------------------------------------- el tubo en memoria (esto es lo importante)
	probarTubo()
	probarCierreDelTubo()

	// ---------------------------------------------------------------- la parte que no hay
	_, errEscuchar := net.Listen("tcp", "0.0.0.0:6112")
	if errEscuchar != nil {
		bien++
		println("RED ok   Listen avisa que no hay red:", errEscuchar.Error()[:48])
	} else {
		mal++
		println("RED FALLO Listen devolvio una conexion; sin red tiene que avisar")
	}
	casos++

	_, errConectar := net.Dial("tcp", "battle.net:6112")
	if errConectar != nil {
		bien++
		println("RED ok   Dial avisa que no hay red")
	} else {
		mal++
		println("RED FALLO Dial devolvio una conexion; sin red tiene que avisar")
	}
	casos++

	println("== RED", bien, "de", casos, "casos; fallos:", mal, "==")
	if mal == 0 {
		println("== RED TODO OK ==")
	} else {
		println("== RED HAY FALLOS ==")
	}
}

// comprobarSplit: arma el resultado de SplitHostPort en un texto y lo compara.
func comprobarSplit(direccion, esperado string) {
	host, puerto, err := net.SplitHostPort(direccion)
	dado := host + "|" + puerto + "|"
	if err != nil {
		dado += "hay error"
	} else {
		dado += "sin error"
	}
	check("SplitHostPort("+direccion+")", esperado, dado)
}

// probarTubo: los datos tienen que cruzar de una punta a la otra.
//
// El que escribe va en una goroutine porque un tubo de verdad bloquea al que escribe hasta que el otro
// lee (asi es como funciona un caño: si el otro no lee, el que escribe espera).
func probarTubo() {
	a, b := net.Pipe()

	mensaje := "hola Papu, esto viaja por el tubo en memoria"

	listo := make(chan struct{})
	var escrito int
	go func() {
		escrito, _ = a.Write([]byte(mensaje))
		close(listo)
	}()

	recibido := make([]byte, len(mensaje))
	n, err := b.Read(recibido)
	<-listo

	if err != nil {
		mal++
		casos++
		println("RED FALLO el tubo devolvio error al leer")
		return
	}

	check("el tubo manda los bytes", mensaje, string(recibido[:n]))
	check("el tubo manda TODOS los bytes", itoa(len(mensaje)), itoa(escrito))

	a.Close()
	b.Close()
}

// probarCierreDelTubo: al cerrar una punta, la otra tiene que enterarse.
//
// ESTE ES EL CASO QUE EVITA UN CUELGUE. Si el cierre no se avisara, el juego se quedaria esperando un dato
// que nunca llega cuando se corta la partida.
func probarCierreDelTubo() {
	a, b := net.Pipe()
	a.Close()

	_, err := b.Read(make([]byte, 16))
	if err == io.EOF {
		bien++
		println("RED ok   al cerrar una punta, la otra recibe el fin de archivo")
	} else {
		mal++
		println("RED FALLO al cerrar una punta, la otra no se entero")
	}
	casos++

	// cerrar las dos puntas (y dos veces la misma) no puede reventar: cerrar un canal dos veces tumba el
	// programa, y cerrar todo al salir del juego es justo lo que pasa
	b.Close()
	b.Close()
	a.Close()
	bien++
	println("RED ok   cerrar las dos puntas (y repetido) no revienta")
	casos++
}

// itoa: pasar un entero a texto (la consola tiene strconv, pero para la evidencia alcanza esto).
func itoa(v int) string {
	if v == 0 {
		return "0"
	}
	var digitos [12]byte
	i := len(digitos)
	for v > 0 {
		i--
		digitos[i] = byte('0' + v%10)
		v /= 10
	}
	return string(digitos[i:])
}

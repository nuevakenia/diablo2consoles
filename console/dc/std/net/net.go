// Package net: las conexiones de red, en la parte que la consola puede tener.
//
// PARA QUE SE USA: d2networking (el servidor y los clientes del multijugador). El motor usa net.Conn en 17
// lugares, net.Pipe en 5 (el cliente local) y ademas Listen/Dial/ResolveTCPAddr.
//
// QUE SI ES DE VERDAD:
//   - net.Pipe(): dos extremos conectados EN MEMORIA, con las dos puntas de verdad (uno escribe y el otro
//     lee, y al cerrarse la una la otra recibe el fin). Esto NO es un adorno: es lo que usa el cliente
//     LOCAL, o sea el multijugador en la misma consola, que es el unico que tiene sentido sin red.
//   - JoinHostPort y SplitHostPort: armar y desarmar "host:puerto". Es manejar texto, no hay red de por
//     medio, y se hace bien (incluso el detalle de los corchetes de las direcciones IPv6).
//
// QUE NO HAY Y POR QUE: las conexiones por RED (Listen, Dial, DialTCP, DialTimeout, ResolveTCPAddr,
// LookupIP) devuelven error con el motivo escrito. El motor de Diablo II que se esta portando es de un
// jugador; el multijugador por internet necesita ADEMAS el servidor de la red (d2realm), que es una
// maquina aparte, y el adaptador de red de la consola. Se declara el limite en vez de devolver una
// conexion que no conecta a nada -- eso ultimo seria peor, porque el juego se quedaria esperando un dato
// que nunca llega.
//
// Lo que SI funciona sin red es el juego de un jugador entero, que es a lo que apunta este puerto.
package net

import (
	"d2std/errors"
	"d2std/io"
	"d2std/strconv"
	"d2std/strings"
	"d2std/sync"
	"d2std/time"
)

// ErrSinRed: el motivo, para que el error lo diga.
var ErrSinRed = errors.New("net: en la consola no hay conexiones de red (el puerto apunta al juego de un jugador)")

// ErrClosed: la conexion ya estaba cerrada (la misma que usa Go).
var ErrClosed = errors.New("use of closed network connection")

// Addr: una direccion de red.
type Addr interface {
	Network() string // "tcp", "udp"...
	String() string  // "127.0.0.1:8080"
}

// Conn: una conexion. La forma es la de Go, tal cual (el motor la usa como tipo).
type Conn interface {
	Read(b []byte) (n int, err error)
	Write(b []byte) (n int, err error)
	Close() error
	LocalAddr() Addr
	RemoteAddr() Addr
	SetDeadline(t time.Time) error
	SetReadDeadline(t time.Time) error
	SetWriteDeadline(t time.Time) error
}

// Listener: algo que acepta conexiones que llegan.
type Listener interface {
	Accept() (Conn, error)
	Close() error
	Addr() Addr
}

// IP: una direccion IP (en la consola se usa solo como dato; no hay red).
type IP []byte

// String: la direccion en la forma de siempre (1.2.3.4).
func (ip IP) String() string {
	if len(ip) == 0 {
		return "<nil>"
	}
	if len(ip) == 4 {
		return strconv.Itoa(int(ip[0])) + "." + strconv.Itoa(int(ip[1])) + "." +
			strconv.Itoa(int(ip[2])) + "." + strconv.Itoa(int(ip[3]))
	}
	// IPv6: no se usa en el motor, pero no se miente devolviendo algo con otra forma
	return "ipv6"
}

// To4: la direccion como IPv4. Si no es una IPv4 (no tiene 4 bytes), no hay nada que devolver.
//
// OJO CON EL CASO QUE IMPORTA: en la consola la busqueda de direcciones (LookupIP) no encuentra ninguna
// --no hay red--, asi que este metodo no se llega a usar con datos. Existe porque el motor pregunta por
// cada direccion que encuentra (d2gamescreen/main_menu.go:694) y el codigo tiene que poder compilar y
// recorrer una lista vacia sin caerse.
func (ip IP) To4() IP {
	if len(ip) == 4 {
		return ip
	}
	return nil
}

// To16: lo mismo para IPv6.
func (ip IP) To16() IP {
	if len(ip) == 4 {
		// una IPv4 tambien se puede ver como IPv6 (los primeros 12 bytes son el prefijo)
		d := make(IP, 16)
		d[10], d[11] = 0xff, 0xff
		copy(d[12:], ip)
		return d
	}
	if len(ip) == 16 {
		return ip
	}
	return nil
}

// TCPAddr: una direccion TCP.
type TCPAddr struct {
	IP   IP
	Port int
	Zone string
}

// Network / String: la direccion en la forma de siempre.
func (a *TCPAddr) Network() string { return "tcp" }
func (a *TCPAddr) String() string {
	if a == nil {
		return "<nil>"
	}
	return JoinHostPort(a.IP.String(), strconv.Itoa(a.Port))
}

// UDPAddr: una direccion UDP.
type UDPAddr struct {
	IP   IP
	Port int
	Zone string
}

// Network / String: lo mismo que TCP.
func (a *UDPAddr) Network() string { return "udp" }
func (a *UDPAddr) String() string {
	if a == nil {
		return "<nil>"
	}
	return JoinHostPort(a.IP.String(), strconv.Itoa(a.Port))
}

// JoinHostPort: arma "host:puerto".
//
// MEDIDO contra el net de Go (no deducido): si el host tiene dos puntos, se le ponen corchetes SIEMPRE,
// aunque ya los tenga. O sea que JoinHostPort("[::1]", "8080") da "[[::1]]:8080" en Go -- parece un
// defecto, pero es lo que hace, y aca se copia igual: si el motor arma una direccion y despues la desarma
// con SplitHostPort, lo que importa es que las dos puntas hagan lo mismo que las de Go, no que el texto
// sea lindo.
func JoinHostPort(host, puerto string) string {
	if strings.Contains(host, ":") {
		return "[" + host + "]:" + puerto
	}
	return host + ":" + puerto
}

// SplitHostPort: desarma "host:puerto".
func SplitHostPort(hostPuerto string) (host, puerto string, err error) {
	i := strings.LastIndex(hostPuerto, ":")
	if i < 0 {
		return "", "", errors.New("net: falta el puerto en la direccion " + hostPuerto)
	}

	host = hostPuerto[:i]
	puerto = hostPuerto[i+1:]

	// los corchetes de IPv6 se sacan
	if len(host) > 0 && host[0] == '[' {
		if len(host) < 2 || host[len(host)-1] != ']' {
			return "", "", errors.New("net: falta el corchete en la direccion " + hostPuerto)
		}
		host = host[1 : len(host)-1]
	}

	// MEDIDO contra el net de Go: si no hay host, se devuelve VACIO (no se rellena con 0.0.0.0, que era lo
	// que hacia esta version). ":7000" da ("", "7000").
	return host, puerto, nil
}

// ---------------------------------------------------------------- net.Pipe (esto si es de verdad)

// tubo: una conexion armada en memoria. Los datos que escribe una punta aparecen en la otra.
//
// COMO ESTA HECHO (y por que asi): un canal con lugar para UN pedazo por sentido, mas un canal de CIERRE
// por punta. Los canales de datos NO se cierran al cerrar la conexion: cerrarlos seria la forma obvia y
// esta mal, porque el otro extremo puede estar mandando justo en ese momento y cerrar un canal con alguien
// mandando adentro revienta el programa. Con el canal de cierre aparte, el que se cierra avisa y el que
// estaba mandando se entera y devuelve el error en vez de caerse.
type tubo struct {
	lectura     chan []byte     // por donde me llegan los datos
	escritura   chan []byte     // por donde mando mis datos
	mioCierre   chan struct{}   // mi canal de cierre (lo cierro yo)
	otroCierre  chan struct{}   // el de la otra punta (si se cierra, me avisa)
	mio         *TCPAddr        // mi direccion
	otro        *TCPAddr        // la de la otra punta
	cerrado     bool
	candado     sync.Mutex
}

// Read: lee lo que mando la otra punta. Cuando la otra punta cierra, devuelve el fin de archivo.
func (t *tubo) Read(b []byte) (int, error) {
	select {
	case pedazo := <-t.lectura:
		return copy(b, pedazo), nil
	case <-t.otroCierre:
		// la otra punta cerro: puede haber quedado un pedazo sin leer, se lo lleva antes de avisar el fin
		select {
		case pedazo := <-t.lectura:
			return copy(b, pedazo), nil
		default:
			return 0, io.EOF
		}
	}
}

// Write: manda datos a la otra punta. Si la otra punta cerro, avisa que la conexion se cerro.
func (t *tubo) Write(b []byte) (int, error) {
	t.candado.Lock()
	cerrado := t.cerrado
	t.candado.Unlock()
	if cerrado {
		return 0, ErrClosed
	}

	copia := make([]byte, len(b))
	copy(copia, b)

	select {
	case t.escritura <- copia:
		return len(copia), nil
	case <-t.otroCierre:
		return 0, ErrClosed
	}
}

// Close: cierra esta punta. La otra se entera (su Read devuelve el fin de archivo y su Write el error).
func (t *tubo) Close() error {
	t.candado.Lock()
	if t.cerrado {
		t.candado.Unlock()
		return nil
	}
	t.cerrado = true
	t.candado.Unlock()
	close(t.mioCierre)
	return nil
}

// LocalAddr / RemoteAddr: las direcciones de las dos puntas. El tubo no tiene direcciones de verdad (no hay
// red), asi que se le dan unas con forma de direccion: devolver nada obligaria al que llama a preguntar si
// es nil en todos lados.
func (t *tubo) LocalAddr() Addr  { return t.mio }
func (t *tubo) RemoteAddr() Addr { return t.otro }

// Las esperas: en un tubo de memoria no hay nada que esperar (los datos estan al instante), asi que se
// aceptan y no hacen nada. NO se devuelve error: el que llama interpretaria eso como "la conexion no
// sirve" y cerraria el cliente local, que es justo lo que tiene que andar.
func (t *tubo) SetDeadline(limite time.Time) error      { return nil }
func (t *tubo) SetReadDeadline(limite time.Time) error  { return nil }
func (t *tubo) SetWriteDeadline(limite time.Time) error { return nil }

// Pipe: dos conexiones conectadas entre si EN MEMORIA. Lo que se escribe en una se lee en la otra.
//
// Es lo que usa el cliente LOCAL del motor (d2networking/d2client/d2localclient): el jugador y el servidor
// en la misma consola, hablando por un tubo en vez de por la red. Es el unico multijugador que tiene
// sentido sin adaptador de red, y este anda de verdad.
func Pipe() (Conn, Conn) {
	// los dos sentidos: lo que A escribe lo lee B, y al reves
	aB := make(chan []byte, 1)
	bA := make(chan []byte, 1)

	cierreA := make(chan struct{})
	cierreB := make(chan struct{})

	direccion := &TCPAddr{IP: IP{127, 0, 0, 1}}

	a := &tubo{lectura: bA, escritura: aB, mioCierre: cierreA, otroCierre: cierreB, mio: direccion, otro: direccion}
	b := &tubo{lectura: aB, escritura: bA, mioCierre: cierreB, otroCierre: cierreA, mio: direccion, otro: direccion}

	return a, b
}

// ---------------------------------------------------------------- lo que necesita la red de verdad

// ResolveTCPAddr: convierte "host:puerto" en una direccion. Sin red, no se puede resolver un nombre.
func ResolveTCPAddr(red, direccion string) (*TCPAddr, error) {
	return nil, ErrSinRed
}

// Listen: poner a escuchar en un puerto. No hay red.
func Listen(red, direccion string) (Listener, error) {
	return nil, ErrSinRed
}

// Dial: conectarse a una direccion. No hay red.
func Dial(red, direccion string) (Conn, error) {
	return nil, ErrSinRed
}

// DialTCP: lo mismo, con las direcciones ya resueltas.
func DialTCP(red string, local, remota *TCPAddr) (*TCPConn, error) {
	return nil, ErrSinRed
}

// DialTimeout: lo mismo, con limite de tiempo.
func DialTimeout(red, direccion string, limite time.Duration) (Conn, error) {
	return nil, ErrSinRed
}

// LookupIP: buscar las direcciones de un nombre. No hay red (ni resolución de nombres).
func LookupIP(host string) ([]IP, error) {
	return nil, ErrSinRed
}

// LookupHost: lo mismo, en texto.
func LookupHost(host string) ([]string, error) {
	return nil, ErrSinRed
}

// TCPConn: la conexion TCP. En la consola no se puede abrir ninguna, pero el tipo existe porque el motor
// lo nombra (d2networking lo tiene en las firmas) y porque su forma es la de Go (Conn + CloseRead/CloseWrite).
type TCPConn struct {
	Conn
}

// UDPConn: lo mismo para UDP.
type UDPConn struct {
	Conn
}

// WriteToUDP / ReadFromUDP: mandar y recibir un paquete a una direccion. Sin red, no se puede: devuelven
// el error con el motivo. Lo pide d2networking/d2server/d2udpclientconnection.
func (c *UDPConn) WriteToUDP(datos []byte, direccion *UDPAddr) (int, error) { return 0, ErrSinRed }
func (c *UDPConn) ReadFromUDP(datos []byte) (int, *UDPAddr, error)         { return 0, nil, ErrSinRed }

// SetReadBuffer / SetWriteBuffer: el tamano de los buffers del sistema. Sin red no hay buffers que ajustar.
func (c *TCPConn) SetReadBuffer(bytes int) error  { return ErrSinRed }
func (c *TCPConn) SetWriteBuffer(bytes int) error { return ErrSinRed }
func (c *UDPConn) SetReadBuffer(bytes int) error  { return ErrSinRed }
func (c *UDPConn) SetWriteBuffer(bytes int) error { return ErrSinRed }

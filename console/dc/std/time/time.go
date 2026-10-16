// Paquete time para la consola: el reloj de KOS y la aritmetica de tiempos del motor.
//
// QUE HAY: Now (con la fecha de la RTC mas los microsegundos del timer), Unix, la aritmetica de Time
// (Add, Sub, Before, After, Equal, IsZero), Duration con sus constantes y su String, Since y Sleep.
//
// QUE NO HAY, Y POR QUE (declarado, no escondido):
//   - Format y Parse: el "layout" de Go (2006-01-02 15:04:05) es un formateador entero, y con las
//     conversiones de calendario de abajo alcanza para lo que el motor usa hoy. Cuando el motor pida
//     Format, se pega ahi.
//   - Year/Month/Day/Hour/Minute/Second y las conversiones de calendario (Date, weekday, ISO): son el
//     calendario gregoriano completo. No estan todavia.
//   - LoadLocation, zona horaria, UTC(): EN LA CONSOLA NO HAY ZONA HORARIA. La hora que se lee es la de
//     la RTC del hardware, que va en UTC. Eso hay que tenerlo presente: no se inventa un offset.
//
// El reloj, medido: rtc_unix_secs() da los segundos Unix que trae la RTC (el documento de KOS avisa que
// puede fallar: si la RTC no esta seteada devuelve 0, y ese 0 es "1970") y timer_us_gettime64() da los
// microsegundos desde que arranco la consola. Now los combina:
//
//     Now = rtc_unix_secs() + (timer_us_gettime64() - timer_del_arranque)
//
// Asi Now avanza con el timer (que no se puede desajustar) y arranca en la fecha de la RTC.
//
// Este paquete NO se puede probar en el host (habla con KOS), asi que su prueba es el POC que corre en
// Flycast (console/dc/poc/vfs) mas los mutadores de consola (scripts/console/mutar-os-consola.sh).
package time

// ---------------------------------------------------------------- Duration

// Duration es una cantidad de tiempo en nanosegundos (el mismo tipo que Go).
type Duration int64

// Las constantes de Go.
const (
	Nanosecond  Duration = 1
	Microsecond          = 1000 * Nanosecond
	Millisecond          = 1000 * Microsecond
	Second               = 1000 * Millisecond
	Minute               = 60 * Second
	Hour                 = 60 * Minute
)

// String arma el texto de una duracion con el mismo formato que Go ("1h2m3.5s", "300ms", "1.5us").
func (d Duration) String() string {
	if d == 0 {
		return "0s"
	}
	neg := d < 0
	if neg {
		d = -d
	}
	s := ""
	// horas
	if d >= Hour {
		s += entero(int64(d/Hour)) + "h"
		d -= (d / Hour) * Hour
	}
	// minutos (si ya hay horas se imprimen aunque sean 0: Go da "1h0m0s", medido en la consola)
	if d >= Minute || s != "" {
		s += entero(int64(d/Minute)) + "m"
		d -= (d / Minute) * Minute
	}
	// Los segundos van siempre que haya horas o minutos ("1h30m0s", como Go), y si la duracion es mas
	// chica que un segundo se usan las unidades chicas con su precision.
	if d >= Second || s != "" {
		s += entero(int64(d/Second)) + fraccion(d, 9) + "s"
	} else if d >= Millisecond {
		s += entero(int64(d/Millisecond)) + fraccion(d, 6) + "ms"
	} else if d >= Microsecond {
		s += entero(int64(d/Microsecond)) + fraccion(d, 3) + "us"
	} else {
		s += entero(int64(d)) + "ns"
	}
	if neg {
		return "-" + s
	}
	return s
}

// entero pasa un int64 a texto (no se usa strconv para no arrastrar el paquete entero aca).
func entero(n int64) string {
	if n == 0 {
		return "0"
	}
	if n < 0 {
		return "-" + entero(-n)
	}
	s := ""
	for n > 0 {
		s = string(rune('0'+n%10)) + s
		n /= 10
	}
	return s
}

// fraccion es la parte decimal de una duracion, sin los ceros del final: el fmtFrac de Go. Se miran los
// ultimos `prec` digitos de los NANOSEGUNDOS (no se divide por una escala: eso fue el error de mi primera
// version, que daba "2ms" donde Go da "2.5ms"). `prec` es 9 para los segundos, 6 para los milisegundos y
// 3 para los microsegundos.
func fraccion(d Duration, prec int) string {
	v := int64(d)
	s := ""
	hay := false
	for i := 0; i < prec; i++ {
		digito := v % 10
		hay = hay || digito != 0
		if hay {
			s = string(rune('0'+digito)) + s
		}
		v /= 10
	}
	if !hay {
		return ""
	}
	return "." + s
}

func (d Duration) Seconds() float64      { return float64(d) / float64(Second) }
func (d Duration) Milliseconds() int64   { return int64(d / Millisecond) }
func (d Duration) Microseconds() int64   { return int64(d / Microsecond) }
func (d Duration) Nanoseconds() int64    { return int64(d) }
func (d Duration) Hours() float64        { return float64(d) / float64(Hour) }
func (d Duration) Minutes() float64      { return float64(d) / float64(Minute) }

// ---------------------------------------------------------------- Time

// Time es un instante: segundos Unix y nanosegundos.
type Time struct {
	sec  int64
	nsec int32
}

// El reloj de KOS.
//
//extern d2c_reloj_unix
func d2c_reloj_unix() int64

//extern d2c_reloj_us
func d2c_reloj_us() uint64

//extern d2c_dormir
func d2c_dormir(ms int)

// arranque: los microsegundos del timer cuando se uso la RTC por primera vez. Se calcula en el primer
// Now para que el tiempo de arranque de la consola (que ya paso) no se sume como si fuera de este
// instante.
var arranqueUs uint64
var arranqueListo bool

// Now: el instante actual, con la fecha de la RTC y los microsegundos del timer de la consola.
func Now() Time {
	us := d2c_reloj_us()
	if !arranqueListo {
		arranqueUs = us
		arranqueListo = true
	}
	base := d2c_reloj_unix()
	transcurrido := int64(us-arranqueUs) / 1000000
	nsec := int32((us - arranqueUs) % 1000000 * 1000)
	return Time{sec: base + transcurrido, nsec: nsec}
}

// Unix arma un Time desde segundos y nanosegundos Unix.
func Unix(sec int64, nsec int64) Time { return Time{sec: sec, nsec: int32(nsec)} }

// Unix devuelve los segundos Unix.
func (t Time) Unix() int64 { return t.sec }

// UnixNano devuelve los nanosegundos Unix.
func (t Time) UnixNano() int64 { return t.sec*1000000000 + int64(t.nsec) }

// IsZero: si es el instante cero (1 de enero de 1970).
func (t Time) IsZero() bool { return t.sec == 0 && t.nsec == 0 }

// Add suma una duracion.
func (t Time) Add(d Duration) Time {
	sec := t.sec + int64(d)/1000000000
	nsec := int64(t.nsec) + int64(d)%1000000000
	if nsec >= 1000000000 {
		sec++
		nsec -= 1000000000
	} else if nsec < 0 {
		sec--
		nsec += 1000000000
	}
	return Time{sec: sec, nsec: int32(nsec)}
}

// Sub: la duracion entre dos instantes.
func (t Time) Sub(u Time) Duration {
	return Duration((t.sec-u.sec)*1000000000 + int64(t.nsec-u.nsec))
}

// Before / After / Equal.
func (t Time) Before(u Time) bool { return t.sec < u.sec || (t.sec == u.sec && t.nsec < u.nsec) }
func (t Time) After(u Time) bool  { return u.Before(t) }
func (t Time) Equal(u Time) bool  { return t.sec == u.sec && t.nsec == u.nsec }

// Since: cuanto paso desde t (lo mismo que Now().Sub(t)).
func Since(t Time) Duration { return Now().Sub(t) }

// Since sobre un instante (el motor usa las dos formas).
func (t Time) Since(u Time) Duration { return t.Sub(u) }

// Sleep duerme la cantidad pedida. La consola usa el sleep de hilos de KOS (thd_sleep, en milisegundos):
// las duraciones mas chicas que un milisegundo se redondean PARA ARRIBA a un milisegundo (dormir 0 no
// duerme nada y quedaria en un lazo que no cede el hilo). Queda declarado: en la consola el reloj del
// scheduler es el milisegundo.
func Sleep(d Duration) {
	if d <= 0 {
		return
	}
	ms := (d + Millisecond - 1) / Millisecond
	if ms < 1 {
		ms = 1
	}
	d2c_dormir(int(ms))
}

// ---------------------------------------------------------------- canales de tiempo

// After: devuelve un canal que recibe la hora despues de d. La usa el reproductor de video del motor
// para esperar entre cuadros.
//
// Como funciona en la consola: no hay temporizadores de hardware, asi que se usa una goroutine que
// duerme con el reloj de KOS (thd_sleep, el mismo que usa Sleep) y despues manda por el canal. El canal
// tiene buffer 1 para que la goroutine no quede colgada si nadie lee.
func After(d Duration) <-chan Time {
	ch := make(chan Time, 1)
	go func() {
		Sleep(d)
		ch <- Now()
	}()
	return ch
}

// Tick: lo mismo, pero repetido cada d. El canal se cierra nunca (igual que en Go, y por eso hay que
// cuidar de no dejarlo suelto: en un juego es un chorro de goroutines).
func Tick(d Duration) <-chan Time {
	ch := make(chan Time, 1)
	go func() {
		for {
			Sleep(d)
			select {
			case ch <- Now():
			default:
			}
		}
	}()
	return ch
}

// NewTimer devuelve un temporizador de un solo disparo.
type Timer struct {
	C <-chan Time
}

// NewTimer: el temporizador que dispara una vez tras d.
func NewTimer(d Duration) *Timer {
	return &Timer{C: After(d)}
}

// NewTicker: el que dispara cada d.
type Ticker struct {
	C <-chan Time
}

func NewTicker(d Duration) *Ticker {
	return &Ticker{C: Tick(d)}
}

// ---------------------------------------------------------------- fechas

// Location: la zona horaria. En la consola el reloj es UTC (KOS no trae zonas): se declara la zona y
// las dos de siempre, para que el codigo que las nombra compile.
type Location struct{}

var (
	UTC   = &Location{}
	Local = &Location{}
)

func (l *Location) String() string { return "UTC" }

// Month: el mes.
type Month int

const (
	January Month = 1 + iota
	February
	March
	April
	May
	June
	July
	August
	September
	October
	November
	December
)

// Weekday: el dia de la semana (0 = domingo, igual que go).
type Weekday int

const (
	Sunday Weekday = iota
	Monday
	Tuesday
	Wednesday
	Thursday
	Friday
	Saturday
)

// Date arma una fecha. El motor la usa para los relojes internos del juego (que son de datos, no del
// sistema: no hace falta que sea exacta, solo consistente).
func Date(year int, month Month, day, hour, min, sec, nsec int, loc *Location) Time {
	// los segundos desde 1970 por el algoritmo de siempre (dias civiles), que es el de go
	anio := year
	m := int(month)
	if m <= 2 {
		anio--
		m += 12
	}
	era := 0
	if anio >= 0 {
		era = anio / 400
	} else {
		era = (anio - 399) / 400
	}
	ye := anio - era*400
	mp := (m + 9) % 12
	dy := (153*mp+2)/5 + day - 1
	do := ye*365 + ye/4 - ye/100 + dy
	seg := int64(era)*146097*86400 + int64(do)*86400 +
		int64(hour)*3600 + int64(min)*60 + int64(sec)
	return Time{sec: seg, nsec: int32(nsec)}
}

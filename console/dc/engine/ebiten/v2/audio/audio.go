// audio: el paquete de sonido de Ebitengine, para la consola.
//
// ESTADO, DICHO CLARO: esto COMPILA y el motor puede llamar a todo, pero TODAVIA NO SUENA. El sonido de
// verdad de la Dreamcast sale por el AICA (el chip de audio), y eso se maneja con el API de KOS (snd_*):
// es el mismo trabajo que se hizo con el PVR para el video, y va DESPUES, porque la prioridad era ver el
// juego en pantalla y mover un personaje.
//
// No se finge: Play, Pause y todo lo demas contestan con el estado que llevan aca (que es coherente), pero
// no hay AICA detras. Cuando se conecte, se cambia el cuerpo de estas funciones y el motor no se entera:
// es lo mismo que se hizo con el PVR (el motor pide "dibuja" y el que dibuja es de la consola).
//
// MEDIDO: el motor pide estos nombres (grep de audio.* en d2core/d2audio/ebiten): Context, NewContext,
// NewPlayer, NewInfiniteLoop, Player.
package audio

import (
	"errors"
	"io"
	"time"
)

// ErrSinSonido: se devuelve donde haga falta avisar que el audio de la consola todavia no esta conectado.
var ErrSinSonido = errors.New("audio: la consola todavia no tiene el AICA conectado (el video fue primero)")

// Context: el contexto de audio (la frecuencia de muestreo del juego).
type Context struct {
	frecuencia int
}

// NewContext: arma el contexto.
func NewContext(frecuencia int) *Context { return &Context{frecuencia: frecuencia} }

// SampleRate: la frecuencia de muestreo.
func (c *Context) SampleRate() int { return c.frecuencia }

// Player: el que reproduce un sonido.
type Player struct {
	contexto   *Context
	fuente     io.Reader
	frecuencia int
	volumen    float64
	sonando    bool
	posicion   time.Duration
	// SinAica dice si este reproductor tiene el chip conectado. Hoy siempre false: el AICA va despues.
	SinAica bool
}

// NewPlayer: arma un reproductor. Igual que ebiten: la fuente tiene que poder rebobinarse (para repetir).
// MEDIDO en ebiten v2.0.2 (audio/audio.go:255): la firma es NewPlayer(context *Context, src io.Reader)
// -- DOS argumentos y la fuente es un io.Reader (no un ReadSeeker, y no lleva la frecuencia: esa la tiene
// el contexto). Aca estaba con tres argumentos y el motor no compilaba.
func NewPlayer(ctx *Context, fuente io.Reader) (*Player, error) {
	if ctx == nil || fuente == nil {
		return nil, ErrSinSonido
	}
	return &Player{contexto: ctx, fuente: fuente, frecuencia: ctx.SampleRate(), volumen: 1, SinAica: true}, nil
}

// NewPlayerFromBytes: lo mismo desde memoria. MEDIDO (audio/audio.go:283): devuelve UN solo valor (sin
// error), porque con bytes en memoria no hay nada que pueda fallar al abrir.
func NewPlayerFromBytes(ctx *Context, datos []byte) *Player {
	return &Player{contexto: ctx, fuente: &lectorBytes{datos: datos}, frecuencia: ctx.SampleRate(), volumen: 1, SinAica: true}
}

// Play: empieza a sonar. OJO: hoy solo cambia el estado (ver el comentario del paquete).
func (p *Player) Play() { p.sonando = true }

// Pause: lo pausa.
func (p *Player) Pause() { p.sonando = false }

// IsPlaying: si esta sonando.
func (p *Player) IsPlaying() bool { return p.sonando }

// Volume: el volumen (0 a 1).
func (p *Player) Volume() float64 { return p.volumen }

// SetVolume: lo cambia.
func (p *Player) SetVolume(v float64) {
	if v < 0 {
		v = 0
	}
	if v > 1 {
		v = 1
	}
	p.volumen = v
}

// Current: por donde va.
func (p *Player) Current() time.Duration { return p.posicion }

// SetPosition: lo salta a un punto.
func (p *Player) SetPosition(d time.Duration) { p.posicion = d }

// Seek: salta a una posicion del sonido. MEDIDO en ebiten v2.0.2 (audio/audio.go:475): es
// Seek(offset time.Duration) error -- lleva el offset en tiempo, no en bytes. Lo pide
// d2audio/ebiten/ebiten_sound_effect.go:148.
func (p *Player) Seek(desplazamiento time.Duration) error {
	if desplazamiento < 0 {
		desplazamiento = 0
	}
	p.posicion = desplazamiento
	// de tiempo a bytes: 16 bits estereo son 4 bytes por muestra
	muestras := int64(desplazamiento) * int64(p.frecuencia) / int64(time.Second)
	if buscador, ok := p.fuente.(io.Seeker); ok {
		_, err := buscador.Seek(muestras*4, io.SeekStart)
		return err
	}
	return nil
}

// Rewind: lo vuelve al principio.
func (p *Player) Rewind() error {
	p.posicion = 0
	// la fuente puede ser un io.Reader que no se pueda rebobinar (asi es la firma de ebiten): se pregunta
	if buscador, ok := p.fuente.(io.Seeker); ok {
		_, err := buscador.Seek(0, io.SeekStart)
		return err
	}
	return nil
}

// Close: lo cierra.
func (p *Player) Close() error {
	p.sonando = false
	return nil
}

// InfiniteLoop: la fuente que repite sin fin (la musica del juego).
type InfiniteLoop struct {
	fuente io.ReadSeeker
	largo  int64
}

// NewInfiniteLoop: la arma. El largo es el de un ciclo, en bytes.
func NewInfiniteLoop(fuente io.ReadSeeker, largo int64) *InfiniteLoop {
	return &InfiniteLoop{fuente: fuente, largo: largo}
}

// Read: lee del ciclo, y al llegar al final vuelve al principio.
func (l *InfiniteLoop) Read(p []byte) (int, error) {
	return l.fuente.Read(p)
}

// Seek: se mueve por el ciclo.
func (l *InfiniteLoop) Seek(desde int64, cuando int) (int64, error) {
	if l.largo <= 0 {
		return 0, nil
	}
	pos := desde % l.largo
	if pos < 0 {
		pos += l.largo
	}
	return l.fuente.Seek(pos, io.SeekStart)
}

// lectorBytes: un io.ReadSeeker sobre bytes en memoria (para NewPlayerFromBytes).
type lectorBytes struct {
	datos []byte
	pos   int64
}

func (l *lectorBytes) Read(p []byte) (int, error) {
	if l.pos >= int64(len(l.datos)) {
		return 0, io.EOF
	}
	n := copy(p, l.datos[l.pos:])
	l.pos += int64(n)
	return n, nil
}

func (l *lectorBytes) Seek(desde int64, cuando int) (int64, error) {
	switch cuando {
	case io.SeekStart:
		l.pos = desde
	case io.SeekCurrent:
		l.pos += desde
	case io.SeekEnd:
		l.pos = int64(len(l.datos)) + desde
	}
	if l.pos < 0 {
		l.pos = 0
	}
	return l.pos, nil
}

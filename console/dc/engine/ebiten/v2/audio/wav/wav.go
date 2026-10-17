// Package wav: el decodificador de WAV de Ebitengine, para la consola.
//
// PARA QUE SE USA: d2core/d2audio/ebiten lo usa para cargar la musica y los efectos del juego (wav.Decode
// devuelve un Stream, que es un lector que el reproductor va consumiendo).
//
// ESTE SI ES DE VERDAD, y no como el audio que todavia no suena: decodificar un WAV es leer bytes, y eso
// en la consola se puede y se prueba. Lo que falta es el OTRO lado (mandar esos bytes al chip de audio, el
// AICA), que es trabajo aparte.
//
// QUE FORMATOS ENTIENDE: PCM lineal de 8 y 16 bits, mono y estereo. Es lo que hay en los audios del juego.
// NO entiende ADPCM ni punto flotante: si aparece uno, se ve al medir (el error lo dice, no devuelve
// silencio).
//
// LA SALIDA ES SIEMPRE LA MISMA FORMA que la de Ebitengine: 16 bits con signo, LITTLE ENDIAN, y ESTEREO
// entrelazado (L, R, L, R...). Si el archivo es mono, cada muestra se duplica en los dos canales; si es de
// 8 bits, se pasa a 16 centrando el valor.
package wav

import (
	"encoding/binary"
	"errors"
	"io"

	"github.com/hajimehoshi/ebiten/v2/audio"
)

// Los errores, con el mismo texto que usa Ebitengine (asi el que lea el mensaje reconoce el caso).
var (
	ErrFormatError = errors.New("wav: invalid format")
	ErrUnexpectedEOF = errors.New("wav: unexpected EOF")
	ErrUnsupported = errors.New("wav: unsupported format")
)

// Stream: el audio ya decodificado, listo para leer. Es un io.ReadSeeker, como el de Ebitengine.
type Stream struct {
	datos      []byte
	pos        int64
	frecuencia int
	// LargoOriginal: cuantos bytes tenia el audio comprimido tal cual (para saber si es un bucle corto).
	LargoOriginal int64
}

// Length: cuantos bytes tiene el audio decodificado.
func (s *Stream) Length() int64 { return int64(len(s.datos)) }

// SampleRate: la frecuencia de muestreo del archivo.
func (s *Stream) SampleRate() int { return s.frecuencia }

// Read: lee los bytes decodificados.
func (s *Stream) Read(p []byte) (int, error) {
	if s.pos >= int64(len(s.datos)) {
		return 0, io.EOF
	}
	n := copy(p, s.datos[s.pos:])
	s.pos += int64(n)
	return n, nil
}

// Seek: se mueve por el audio (lo usa el reproductor para repetir un pedazo).
func (s *Stream) Seek(desde int64, cuando int) (int64, error) {
	switch cuando {
	case io.SeekStart:
		s.pos = desde
	case io.SeekCurrent:
		s.pos += desde
	case io.SeekEnd:
		s.pos = int64(len(s.datos)) + desde
	}
	if s.pos < 0 {
		s.pos = 0
	}
	return s.pos, nil
}

// Close: no hay nada que cerrar (los datos estan en memoria).
func (s *Stream) Close() error { return nil }

// Decode: lee un WAV y devuelve el audio ya decodificado en la forma de Ebitengine.
//
// El contexto se recibe por la forma de la biblioteca (y para saber la frecuencia de salida del juego); aca
// no se usa todavia porque no hay chip conectado, y eso esta dicho arriba.
func Decode(ctx *audio.Context, r io.ReadSeeker) (*Stream, error) {
	crudo, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}

	f, err := leerCabecera(crudo)
	if err != nil {
		return nil, err
	}

	muestras := crudo[f.inicioAudio:f.finAudio]
	salida := make([]byte, 0, len(muestras)*f.factorSalida())

	switch {
	case f.bits == 16:
		salida = muestrasHacia16(muestras, f.canales)
	case f.bits == 8:
		salida = muestras8Hacia16(muestras, f.canales)
	default:
		return nil, ErrUnsupported
	}

	return &Stream{
		datos:         salida,
		frecuencia:    f.frecuencia,
		LargoOriginal: int64(len(crudo)),
	}, nil
}

// formato: lo que dice la cabecera del WAV.
type formato struct {
	canales     int
	frecuencia  int
	bits        int
	inicioAudio int
	finAudio    int
}

// factorSalida: cuantos bytes de salida salen por cada byte del audio original (para reservar de una).
func (f formato) factorSalida() int {
	// 16 bits estereo de salida por muestra; la entrada puede ser 8 o 16 y mono o estereo
	if f.bits == 16 && f.canales == 2 {
		return 1
	}
	if f.bits == 16 && f.canales == 1 {
		return 2
	}
	if f.bits == 8 && f.canales == 2 {
		return 2
	}
	return 4
}

// leerCabecera: saca el formato del WAV y donde empiezan los datos.
//
// Un WAV es RIFF<largo>WAVE y despues una lista de bloques, cada uno con su nombre y su largo. Los que
// importan son "fmt " (el formato) y "data" (el audio). Se recorre por bloques y no por posiciones fijas,
// porque hay archivos con bloques de mas en el medio (los escribe cada grabador como quiere).
func leerCabecera(d []byte) (formato, error) {
	var f formato

	if len(d) < 12 || string(d[0:4]) != "RIFF" || string(d[8:12]) != "WAVE" {
		return f, ErrFormatError
	}

	i := 12
	for i+8 <= len(d) {
		nombre := string(d[i : i+4])
		largo := int(binary.LittleEndian.Uint32(d[i+4 : i+8]))
		cuerpo := i + 8

		switch nombre {
		case "fmt ":
			if cuerpo+16 > len(d) {
				return f, ErrFormatError
			}
			formatoAudio := binary.LittleEndian.Uint16(d[cuerpo : cuerpo+2])
			f.canales = int(binary.LittleEndian.Uint16(d[cuerpo+2 : cuerpo+4]))
			f.frecuencia = int(binary.LittleEndian.Uint32(d[cuerpo+4 : cuerpo+8]))
			f.bits = int(binary.LittleEndian.Uint16(d[cuerpo+14 : cuerpo+16]))

			// 1 = PCM lineal. 0xfffe es "PCM extendido" (el mismo PCM con un bloque de mas), y el motor
			// usa wav de esa forma: se acepta.
			if formatoAudio != 1 && formatoAudio != 0xfffe {
				return f, ErrUnsupported
			}
			if f.canales != 1 && f.canales != 2 {
				return f, ErrUnsupported
			}

		case "data":
			if f.bits == 0 {
				return f, ErrFormatError // el data vino antes del fmt: el archivo esta mal armado
			}
			// El largo del bloque manda: puede haber otros bloques despues del audio (algunos grabadores
			// agregan metadatos), asi que el audio TERMINA donde dice el bloque y no donde termina el
			// archivo. Medido: sin esto, la salida se comia los bytes de los bloques de atras.
			f.inicioAudio = cuerpo
			f.finAudio = len(d)
			if largo > 0 && cuerpo+largo <= len(d) {
				f.finAudio = cuerpo + largo
			}
			return f, nil
		}

		// el siguiente bloque. Los bloques se alinean a 2 bytes.
		i = cuerpo + largo
		if largo%2 == 1 {
			i++
		}
	}

	return f, ErrFormatError
}

// muestrasHacia16: de PCM 16 a la salida (16 bits con signo, estereo entrelazado).
func muestrasHacia16(d []byte, canales int) []byte {
	if canales == 2 {
		// ya esta en la forma de salida
		salida := make([]byte, len(d)/2*2)
		copy(salida, d)
		return salida
	}
	// mono: cada muestra se repite en los dos canales
	salida := make([]byte, 0, len(d)*2)
	for i := 0; i+1 < len(d); i += 2 {
		salida = append(salida, d[i], d[i+1], d[i], d[i+1])
	}
	return salida
}

// muestras8Hacia16: de PCM 8 bits SIN signo a 16 bits CON signo (el WAV guarda los 8 bits centrados en
// 128, y la salida de Ebitengine es con signo).
func muestras8Hacia16(d []byte, canales int) []byte {
	salida := make([]byte, 0, len(d)*4)
	for _, m := range d {
		// 8 bits sin signo -> 16 con signo: se centra en 128 y se escala
		v := int(m) - 128
		v = v * 256
		par := []byte{byte(v & 0xff), byte((v >> 8) & 0xff)}
		if canales == 1 {
			salida = append(salida, par[0], par[1], par[0], par[1])
		} else {
			salida = append(salida, par[0], par[1])
		}
	}
	return salida
}

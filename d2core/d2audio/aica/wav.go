package aica

import (
	"encoding/binary"
	"fmt"
)

// EL WAV DEL MOTOR, PARSEADO A MANO.
//
// POR QUE A MANO: en el host el motor decodifica el WAV con `ebiten/v2/audio/wav` (d2core/d2audio/ebiten/
// ebiten_audio_provider.go). En la consola eso no existe, y el camino de KOS (`snd_sfx_load_raw_buf`)
// quiere el PCM CRUDO con la tasa, los bits y los canales aparte. Asi que hay que leer la cabecera RIFF:
// es un formato de bloques chicos y fijos, y de paso el parseo devuelve los numeros que van al log de la
// evidencia (tasa, bits, canales, muestras, duracion) en vez de dejarlos adentro del chip.
//
// LO QUE ACEPTA: PCM sin comprimir (audioFormat 1) de 8 o 16 bits, mono o estereo. Es exactamente lo que
// `snd_sfx_load_raw_buf` sabe tocar (snd_sfxmgr.c:638-660) y lo que hay en d2Sfx.mpq (medido: los 2330
// archivos del MPQ son .wav PCM). Un ADPCM (audioFormat 2, que KOS tambien soporta) se RECHAZA con un
// error que lo dice: no es un olvido, es un formato que este camino no decodifica.

// Sonido es un WAV del motor ya entendido: la cabecera como numeros y el PCM crudo que va al AICA.
type Sonido struct {
	// Formato es el audioFormat del bloque `fmt ` tal cual (1 = PCM). Se guarda el numero, no una
	// traduccion, para que la evidencia muestre lo que dice el archivo.
	Formato int
	// Canales es 1 (mono) o 2 (estereo).
	Canales int
	// Tasa son las muestras por segundo (44100, 22050...).
	Tasa int
	// Bits es el tamano de muestra: 8 o 16.
	Bits int
	// Muestras es el numero de muestras POR CANAL del bloque `data`.
	Muestras int
	// PCM es el bloque `data` crudo, ya recortado a muestras enteras.
	PCM []byte
}

// BytesPorMuestra es el tamano de una muestra en bytes (1 con 8 bits, 2 con 16).
func (s *Sonido) BytesPorMuestra() int { return s.Bits / 8 }

// BytesDeTrama es el tamano de una trama (una muestra de todos los canales).
func (s *Sonido) BytesDeTrama() int { return s.BytesPorMuestra() * s.Canales }

// DuracionMs es lo que dura el efecto, en milisegundos. Lo usa el que mide: si una muestra de 430 ms no se
// termina nunca, el problema es del chip y no del efecto.
func (s *Sonido) DuracionMs() int {
	if s.Tasa <= 0 {
		return 0
	}

	return s.Muestras * 1000 / s.Tasa
}

// Formatos que este camino acepta, con su nombre, para los mensajes.
const (
	formatoPCM    = 1
	bitsOcho      = 8
	bitsDiesiseis = 16
	maximoCanales = 2
)

// ParsearWAV lee un WAV PCM de memoria. No toca el disco: los bytes ya vienen del MPQ (d2mpq.ReadFile).
//
// Los errores dicen QUE bloque falta o que valor esta fuera de rango: un WAV del juego que no se pueda
// cargar tiene que poder diagnosticarse desde el log de la consola, donde no hay depurador.
func ParsearWAV(datos []byte) (*Sonido, error) {
	if len(datos) < 12 {
		return nil, fmt.Errorf("wav: %d bytes es menos que una cabecera RIFF", len(datos))
	}

	if string(datos[0:4]) != "RIFF" {
		return nil, fmt.Errorf("wav: no empieza con RIFF (empieza con %q)", datos[0:4])
	}

	if string(datos[8:12]) != "WAVE" {
		return nil, fmt.Errorf("wav: el tipo no es WAVE (%q)", datos[8:12])
	}

	var (
		sonido   Sonido
		hayFmt   bool
		hayDatos bool
	)

	// los bloques van uno atras del otro, cada uno con su largo; los de relleno (fact, cue, LIST) se
	// saltan por su largo en vez de buscarlos por nombre
	for pos := 12; pos+8 <= len(datos); {
		id := string(datos[pos : pos+4])
		largo := int(binary.LittleEndian.Uint32(datos[pos+4 : pos+8]))
		cuerpo := pos + 8

		if largo < 0 {
			return nil, fmt.Errorf("wav: el bloque %q tiene largo negativo (%d)", id, largo)
		}

		// el UNICO bloque que puede venir cortado es el ultimo `data` de un archivo redondeado a 2048:
		// se toma lo que hay en vez de rechazar el sonido entero
		if cuerpo+largo > len(datos) {
			if id != "data" {
				return nil, fmt.Errorf("wav: el bloque %q dice %d B y solo quedan %d", id, largo, len(datos)-cuerpo)
			}

			largo = len(datos) - cuerpo
		}

		switch id {
		case "fmt ":
			if largo < 16 {
				return nil, fmt.Errorf("wav: el bloque fmt dice %d B (el minimo es 16)", largo)
			}

			sonido.Formato = int(binary.LittleEndian.Uint16(datos[cuerpo : cuerpo+2]))
			sonido.Canales = int(binary.LittleEndian.Uint16(datos[cuerpo+2 : cuerpo+4]))
			sonido.Tasa = int(binary.LittleEndian.Uint32(datos[cuerpo+4 : cuerpo+8]))
			sonido.Bits = int(binary.LittleEndian.Uint16(datos[cuerpo+14 : cuerpo+16]))
			hayFmt = true
		case "data":
			sonido.PCM = datos[cuerpo : cuerpo+largo]
			hayDatos = true
		}

		pos = cuerpo + largo
		// la RIFF rellena a par: un bloque de largo impar ocupa uno mas
		if largo%2 == 1 {
			pos++
		}
	}

	if !hayFmt {
		return nil, fmt.Errorf("wav: no hay bloque fmt")
	}

	if !hayDatos {
		return nil, fmt.Errorf("wav: no hay bloque data")
	}

	if err := sonido.validar(); err != nil {
		return nil, err
	}

	sonido.recortarATramasEnteras()

	return &sonido, nil
}

// validar: los valores que el AICA (y snd_sfx_load_raw_buf) aceptan. Se revisan antes de tocar el chip: un
// canal en 0 o una tasa en 0 sube un efecto mudo o revienta el calculo del chip.
func (s *Sonido) validar() error {
	if s.Formato != formatoPCM {
		return fmt.Errorf("wav: formato %d no soportado (este camino solo acepta PCM = 1; ADPCM = 2 no)", s.Formato)
	}

	if s.Bits != bitsOcho && s.Bits != bitsDiesiseis {
		return fmt.Errorf("wav: %d bits por muestra (este camino acepta 8 o 16)", s.Bits)
	}

	if s.Canales < 1 || s.Canales > maximoCanales {
		return fmt.Errorf("wav: %d canales (este camino acepta 1 o 2)", s.Canales)
	}

	if s.Tasa <= 0 {
		return fmt.Errorf("wav: tasa %d Hz", s.Tasa)
	}

	if len(s.PCM) < s.BytesDeTrama() {
		return fmt.Errorf("wav: el bloque data trae %d B y una trama son %d", len(s.PCM), s.BytesDeTrama())
	}

	return nil
}

// recortarATramasEnteras: si el bloque data trae media trama al final (archivo cortado), se tira ese resto
// en vez de mandarle al chip una trama incompleta.
func (s *Sonido) recortarATramasEnteras() {
	trama := s.BytesDeTrama()
	enteras := len(s.PCM) / trama * trama
	if enteras != len(s.PCM) {
		s.PCM = s.PCM[:enteras]
	}

	s.Muestras = len(s.PCM) / trama
}

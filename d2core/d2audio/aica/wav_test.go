package aica

import (
	"encoding/binary"
	"math"
	"testing"
)

// UN WAV ARMADO A MANO PARA LA PRUEBA: la cabecera RIFF con bloques de verdad, para probar el parseo del
// camino de la consola en el host (donde si hay `go test`).
func wavDePrueba(formato, canales, tasa, bits int, pcm []byte, extra []byte) []byte {
	cuerpo := make([]byte, 0, 64+len(pcm)+len(extra))

	cuerpo = append(cuerpo, "fmt "...)
	fmtBloque := make([]byte, 16)
	binary.LittleEndian.PutUint16(fmtBloque[0:2], uint16(formato))
	binary.LittleEndian.PutUint16(fmtBloque[2:4], uint16(canales))
	binary.LittleEndian.PutUint32(fmtBloque[4:8], uint32(tasa))
	binary.LittleEndian.PutUint32(fmtBloque[8:12], uint32(tasa*bits/8*canales))
	binary.LittleEndian.PutUint16(fmtBloque[12:14], uint16(bits/8*canales))
	binary.LittleEndian.PutUint16(fmtBloque[14:16], uint16(bits))
	cuerpo = append(cuerpo, u32(uint32(len(fmtBloque)))...)
	cuerpo = append(cuerpo, fmtBloque...)

	// un bloque de relleno EN MEDIO, como traen los WAV del juego (fact/cue/LIST): el parseo tiene que
	// saltarlo por su largo y no por su nombre
	cuerpo = append(cuerpo, extra...)

	cuerpo = append(cuerpo, "data"...)
	cuerpo = append(cuerpo, u32(uint32(len(pcm)))...)
	cuerpo = append(cuerpo, pcm...)

	salida := make([]byte, 0, 12+len(cuerpo))
	salida = append(salida, "RIFF"...)
	salida = append(salida, u32(uint32(len(cuerpo)))...)
	salida = append(salida, "WAVE"...)
	salida = append(salida, cuerpo...)

	return salida
}

// bloque arma un bloque RIFF crudo con su relleno a par, como los traen los WAV del juego.
func bloque(id string, cuerpo []byte) []byte {
	salida := append([]byte(id), u32(uint32(len(cuerpo)))...)
	salida = append(salida, cuerpo...)

	if len(cuerpo)%2 == 1 {
		salida = append(salida, 0)
	}

	return salida
}

func u32(v uint32) []byte {
	b := make([]byte, 4)
	binary.LittleEndian.PutUint32(b, v)

	return b
}

func TestParsearWAV(t *testing.T) {
	// 22050 Hz, 16 bits, mono, 100 muestras = 200 B de PCM = 4 ms
	pcm := make([]byte, 200)
	for i := range pcm {
		pcm[i] = byte(i)
	}

	s, err := ParsearWAV(wavDePrueba(1, 1, 22050, 16, pcm, nil))
	if err != nil {
		t.Fatalf("un WAV PCM valido no se pudo parsear: %v", err)
	}

	if s.Formato != 1 || s.Canales != 1 || s.Tasa != 22050 || s.Bits != 16 {
		t.Fatalf("cabecera mal leida: %+v", s)
	}

	if s.Muestras != 100 {
		t.Fatalf("muestras = %d, queria 100", s.Muestras)
	}

	if len(s.PCM) != 200 {
		t.Fatalf("PCM = %d B, queria 200", len(s.PCM))
	}

	if s.DuracionMs() != 4 {
		t.Fatalf("duracion = %d ms, queria 4", s.DuracionMs())
	}
}

// TestParsearWAVSaltaBloques: los WAV del juego traen bloques que no son fmt ni data. Si el parseo se
// quedara pegado al primero que encuentra, o buscara el nombre en el lugar equivocado, no cargarian.
func TestParsearWAVSaltaBloques(t *testing.T) {
	pcm := []byte{1, 0, 2, 0}
	wav := wavDePrueba(1, 1, 44100, 16, pcm, bloque("LIST", []byte("abcd")))

	s, err := ParsearWAV(wav)
	if err != nil {
		t.Fatalf("un WAV con un bloque de relleno no se pudo parsear: %v", err)
	}

	if s.Muestras != 2 || s.Tasa != 44100 {
		t.Fatalf("mal leido con relleno: %+v", s)
	}
}

// TestParsearWAVBloqueImpar: la RIFF rellena a par los bloques de largo impar (un byte de relleno que NO
// cuenta en el largo). Si el parseo no lo salta, el bloque siguiente empieza corrido un byte y no se
// encuentra el `data`.
func TestParsearWAVBloqueImpar(t *testing.T) {
	pcm := []byte{1, 0, 2, 0, 3, 0, 4, 0}
	wav := wavDePrueba(1, 1, 44100, 16, pcm, bloque("fact", []byte("abc")))

	s, err := ParsearWAV(wav)
	if err != nil {
		t.Fatalf("un bloque de largo impar no se pudo saltar: %v", err)
	}

	if s.Muestras != 4 || s.Tasa != 44100 {
		t.Fatalf("mal leido con bloque impar: %+v", s)
	}
}

// TestParsearWAVEstereo: 4 tramas estereo de 16 bits = 16 B = 4 muestras por canal.
func TestParsearWAVEstereo(t *testing.T) {
	pcm := make([]byte, 16)

	s, err := ParsearWAV(wavDePrueba(1, 2, 44100, 16, pcm, nil))
	if err != nil {
		t.Fatalf("estereo no soportado: %v", err)
	}

	if s.Canales != 2 || s.BytesDeTrama() != 4 || s.Muestras != 4 {
		t.Fatalf("estereo mal contado: canales=%d trama=%d muestras=%d", s.Canales, s.BytesDeTrama(), s.Muestras)
	}
}

// TestParsearWAVTramaIncompleta: un data con media trama al final se recorta a tramas enteras (mandarle al
// chip una trama a medias es corromper la ultima muestra).
func TestParsearWAVTramaIncompleta(t *testing.T) {
	pcm := make([]byte, 17) // 8 tramas de 2 B + 1 byte suelto

	s, err := ParsearWAV(wavDePrueba(1, 1, 44100, 16, pcm, nil))
	if err != nil {
		t.Fatalf("no se pudo parsear: %v", err)
	}

	if s.Muestras != 8 || len(s.PCM) != 16 {
		t.Fatalf("no recorto la trama incompleta: muestras=%d PCM=%d B", s.Muestras, len(s.PCM))
	}
}

// TestParsearWAVDataCortado: el ultimo bloque data de un archivo redondeado a 2048 puede venir cortado; se
// toma lo que hay (si no, media biblioteca de sonidos quedaria afuera).
func TestParsearWAVDataCortado(t *testing.T) {
	pcm := make([]byte, 100)
	wav := wavDePrueba(1, 1, 22050, 16, pcm, nil)

	// se miente en el largo del bloque data: dice 1000 y hay 100
	binary.LittleEndian.PutUint32(wav[len(wav)-100-4:], 1000)

	s, err := ParsearWAV(wav)
	if err != nil {
		t.Fatalf("un data cortado al final no se pudo parsear: %v", err)
	}

	if s.Muestras != 50 {
		t.Fatalf("muestras = %d, queria 50 (lo que hay)", s.Muestras)
	}
}

// TestParsearWAVRechaza: lo que este camino NO sabe tocar tiene que fallar con un error, no sonar mal.
func TestParsearWAVRechaza(t *testing.T) {
	casos := []struct {
		nombre string
		datos  []byte
	}{
		{"vacio", nil},
		{"no es RIFF", []byte("XXXXlargoWAVEfmt ")},
		{"sin bloque fmt", append(append([]byte("RIFF"), u32(4)...), []byte("WAVEdata")...)},
		{"sin bloque data", wavDePrueba(1, 1, 22050, 16, nil, nil)[:36]},
		{"ADPCM (formato 2)", wavDePrueba(2, 1, 22050, 16, make([]byte, 8), nil)},
		{"24 bits", wavDePrueba(1, 1, 22050, 24, make([]byte, 9), nil)},
		{"tres canales", wavDePrueba(1, 3, 22050, 16, make([]byte, 12), nil)},
		{"tasa cero", wavDePrueba(1, 1, 0, 16, make([]byte, 8), nil)},
	}

	for _, c := range casos {
		if _, err := ParsearWAV(c.datos); err == nil {
			t.Fatalf("%s: lo acepto y no deberia", c.nombre)
		}
	}
}

func TestVolumenAICA(t *testing.T) {
	casos := []struct {
		entra float64
		sale  int
	}{
		{0, 0},
		{1, VolumenMaximo},
		{0.5, 128},
		{-0.2, 0},    // se recorta, no da la vuelta
		{1.5, 255},   // idem por arriba
		{0.999, 255}, // se redondea: truncar dejaria 254
		{math.NaN(), 0},
	}

	for _, c := range casos {
		if v := VolumenAICA(c.entra); v != c.sale {
			t.Fatalf("VolumenAICA(%v) = %d, queria %d", c.entra, v, c.sale)
		}
	}
}

func TestPanAICA(t *testing.T) {
	casos := []struct {
		entra float64
		sale  int
	}{
		{-1, PanMinimo},
		{0, PanCentro},
		{1, PanMaximo},
		{-0.5, 64},
		{0.5, 191},
		{-3, 0},
		{3, 255},
		{math.NaN(), PanCentro},
	}

	for _, c := range casos {
		if v := PanAICA(c.entra); v != c.sale {
			t.Fatalf("PanAICA(%v) = %d, queria %d", c.entra, v, c.sale)
		}
	}
}

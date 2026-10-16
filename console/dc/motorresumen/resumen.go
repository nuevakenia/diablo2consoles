// Package motorresumen: el resumen medible de un DC6 del motor, en UN solo lugar.
//
// Por que un paquete aparte: la misma cuenta la corre el comparador del host y el POC de la consola,
// y la evidencia es que las dos lineas salgan iguales. Si el resumen estuviera escrito dos veces,
// una diferencia entre las dos copias se veria como una diferencia de plataforma: seria una mentira
// del instrumento. Lo unico que cambia entre los dos lados es el compilador y la plataforma.
package motorresumen

import (
	"fmt"
	"strconv"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2dc6"
)

// Lineas devuelve el resumen del DC6 como lineas de texto, todas con el prefijo MOTOR-DC6.
//
// Lo que se mide por frame: el tamano, los offsets, el largo del bloque comprimido, la cantidad de
// pixeles con indice distinto de cero (el 0 es transparente en DC6) y un hash FNV-1a de 32 bits de
// los indices decodificados. El hash es lo que hace que "decodifico bien" sea un numero y no una
// impresion: un solo pixel distinto lo cambia.
func Lineas(d *d2dc6.DC6) []string {
	out := []string{
		"MOTOR-DC6 version=" + strconv.FormatInt(int64(d.Version), 10) +
			" flags=" + strconv.FormatUint(uint64(d.Flags), 10) +
			" encoding=" + strconv.FormatUint(uint64(d.Encoding), 10) +
			" dirs=" + strconv.FormatUint(uint64(d.Directions), 10) +
			" framesPorDir=" + strconv.FormatUint(uint64(d.FramesPerDirection), 10) +
			" frames=" + strconv.Itoa(len(d.Frames)),
	}

	for i, f := range d.Frames {
		if f == nil {
			out = append(out, "MOTOR-DC6 frame "+strconv.Itoa(i)+": nil")
			continue
		}

		idx := d.DecodeFrame(i)
		linea := "MOTOR-DC6 frame " + strconv.Itoa(i) +
			": " + strconv.FormatUint(uint64(f.Width), 10) + "x" + strconv.FormatUint(uint64(f.Height), 10) +
			" off=(" + strconv.FormatInt(int64(f.OffsetX), 10) + "," + strconv.FormatInt(int64(f.OffsetY), 10) + ")" +
			" flipeado=" + strconv.FormatUint(uint64(f.Flipped), 10) +
			" bloque=" + strconv.FormatUint(uint64(f.Length), 10)

		if idx == nil {
			out = append(out, linea+" indices=nil")
			continue
		}

		opacos := 0
		for _, b := range idx {
			if b != 0 {
				opacos++
			}
		}

		out = append(out, linea+
			" pixeles="+strconv.Itoa(len(idx))+
			" opacos="+strconv.Itoa(opacos)+
			" hash="+Hash(idx))
	}

	return out
}

// Hash es FNV-1a de 32 bits sobre los bytes, en hexadecimal. Se hace a mano (no hay hash/fnv en la
// consola) y no es magia: son dos operaciones por byte.
func Hash(b []byte) string {
	var h uint32 = 2166136261
	for _, c := range b {
		h ^= uint32(c)
		h *= 16777619
	}

	return fmt.Sprintf("%08x", h)
}

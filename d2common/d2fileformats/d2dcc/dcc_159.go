package d2dcc

import (
	"fmt"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2datautils"
)

// DC-002.159b -- LAS DIRECCIONES SE DECODIFICAN CUANDO SE PIDEN (la politica de la consola).
//
// QUE PROBLEMA CIERRA. `Load` descomprime las 16 direcciones de un `.dcc` de personaje de una sola vez. Los
// pixeles descomprimidos de UNA capa de la amazona son enormes al lado del archivo: la capa `LG` son 202.352 B
// de pixeles de un archivo de 63.718 B (medido en DC-002.49) y en el Gate 0 la capa `LA` del modo de pueblo
// costo 419.184 B en el semiespacio (medido en esta tarjeta: el recolector pasa de 952.632 a 1.370.712 B
// vivos leyendo `AMLAlitTN1ht.dcc`). Un modo del heroe son SIETE capas: 745 KB de pixeles para dibujar UNA
// direccion por cuadro. El semiespacio del recolector de la consola son 1.835.008 B, asi que ese lujo no
// entra: en DC-002.159 el heroe dejaba 1.672.456 B vivos (91 %), el recolector recogia en cada cuadro y el
// cuadro se iba a 537 ms.
//
// COMO. `LoadPerezoso` lee la cabecera y se queda con los bytes del archivo (`readHeader` ya los guarda en
// `fileData`, que es de donde se decodifica cada direccion). `DecodificarDireccion` descomprime UNA direccion
// la primera vez que se pide, y `SoltarLasOtrasDirecciones` tira las que no se estan dibujando: si el jugador
// vuelve a girar, se decodifican de nuevo del archivo, que es barato al lado de tenerlas todas.
//
// NO ES EL CAMINO DE LA PC. En la PC se sigue usando `Load`, que decodifica todo como siempre: ahi sobran
// 2 GB y las animaciones comparten el `.dcc` entre entes.
//
// OJO CON LA VIDA DE `fileData`. Decodificar despues de cargar exige que los bytes del archivo VIVAN tanto como
// el `DCC`. El lector del motor puede devolver un tramo prestado que vale hasta la proxima lectura
// (DC-002.99), asi que el que elige este camino tiene que darle al `DCC` bytes suyos (ver `LoadDCC` en
// `d2core/d2asset`, que en la consola pide el bloque del archivo fuera del semiespacio).

// LoadPerezoso lee la cabecera y la tabla de offsets, y deja las direcciones SIN decodificar (DC-002.159b).
//
// Las direcciones quedan en nil hasta que alguien pida `DecodificarDireccion`. El `DCC` se queda con
// `fileData`: ese slice tiene que seguir siendo valido mientras se pidan direcciones nuevas.
func LoadPerezoso(fileData []byte) (dcc *DCC, err error) {
	defer d2datautils.RecoverError("dcc", &err)

	result, err := readHeader(fileData)
	if err != nil {
		return nil, err
	}

	result.Directions = make([]*DCCDirection, result.NumberOfDirections)

	return result, nil
}

// DecodificarDireccion descomprime UNA direccion si todavia no estaba, y no hace nada si ya estaba
// (DC-002.159b). Es idempotente para que el que dibuja pueda llamarla sin saber el estado.
func (d *DCC) DecodificarDireccion(direction int) error {
	if direction < 0 || direction >= d.NumberOfDirections {
		return fmt.Errorf("dcc: direction %d out of range (the file has %d)", direction, d.NumberOfDirections)
	}

	if d.Directions[direction] != nil {
		return nil
	}

	d.Directions[direction] = d.decodeDirection(direction)

	return nil
}

// DireccionDecodificada dice si esa direccion ya tiene sus pixeles (DC-002.159b).
func (d *DCC) DireccionDecodificada(direction int) bool {
	if direction < 0 || direction >= len(d.Directions) {
		return false
	}

	return d.Directions[direction] != nil
}

// SoltarLasOtrasDirecciones tira los pixeles descomprimidos de todas las direcciones menos una y devuelve
// cuantas solto (DC-002.159b). Se pueden volver a decodificar del archivo con `DecodificarDireccion`.
func (d *DCC) SoltarLasOtrasDirecciones(menos int) int {
	soltadas := 0

	for i := range d.Directions {
		if i == menos || d.Directions[i] == nil {
			continue
		}

		d.Directions[i] = nil
		soltadas++
	}

	return soltadas
}

// TamanoDelArchivo dice cuantos bytes del archivo crudo guarda el `DCC` (DC-002.159b).
//
// Se usa para el presupuesto de la cache: con las direcciones perezosas los pixeles descomprimidos son casi cero y
// lo que ocupa es el archivo crudo, que es justo lo que queda vivo para poder decodificar despues.
func (d *DCC) TamanoDelArchivo() int {
	return len(d.fileData)
}

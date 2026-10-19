// DC-002.42: LO QUE SOLO TIENE EL HOST (la consola lee la tabla del disco: object_lookups_store.go).
//
// Este archivo NO entra al binario de la consola (el build lo excluye por nombre, igual que
// object_lookup_record_data.go): es el otro lado de la palanca -- el registro de la tabla compilada como
// fuente del host, su acceso, y el ESCRITOR del archivo que la consola lee (/JUEGO/OBJECTLOOKUPS.PACK).
package d2records

import (
	"bytes"
	"fmt"
	"io"
)

// ================================================================================================
// DC-002.42: LO QUE SOLO TIENE EL HOST (la consola lee la tabla del disco: object_lookups_store.go)
// ================================================================================================

// ordenDeLosCamposDeTexto es el ORDEN de las 23 cadenas del registro: ES el del struct ObjectLookupRecord y
// el que lee decodificaRegistro (object_lookups_store.go). Si alguien cambia el struct o este orden, la ida y
// vuelta (TestPackObjectLookups) deja de cerrar y lo dice.
var ordenDeLosCamposDeTexto = []func(*ObjectLookupRecord) string{ //nolint:gochecknoglobals // orden del formato
	func(r *ObjectLookupRecord) string { return r.Name },
	func(r *ObjectLookupRecord) string { return r.Description },
	func(r *ObjectLookupRecord) string { return r.Base },
	func(r *ObjectLookupRecord) string { return r.Token },
	func(r *ObjectLookupRecord) string { return r.Mode },
	func(r *ObjectLookupRecord) string { return r.Class },
	func(r *ObjectLookupRecord) string { return r.HD },
	func(r *ObjectLookupRecord) string { return r.TR },
	func(r *ObjectLookupRecord) string { return r.LG },
	func(r *ObjectLookupRecord) string { return r.RA },
	func(r *ObjectLookupRecord) string { return r.LA },
	func(r *ObjectLookupRecord) string { return r.RH },
	func(r *ObjectLookupRecord) string { return r.LH },
	func(r *ObjectLookupRecord) string { return r.SH },
	func(r *ObjectLookupRecord) string { return r.S1 },
	func(r *ObjectLookupRecord) string { return r.S2 },
	func(r *ObjectLookupRecord) string { return r.S3 },
	func(r *ObjectLookupRecord) string { return r.S4 },
	func(r *ObjectLookupRecord) string { return r.S5 },
	func(r *ObjectLookupRecord) string { return r.S6 },
	func(r *ObjectLookupRecord) string { return r.S7 },
	func(r *ObjectLookupRecord) string { return r.S8 },
	func(r *ObjectLookupRecord) string { return r.ColorMap },
}

// punterosDeLosCamposDeTexto es el MISMO orden, para el lado que lee (TestPackObjectLookups compara campo por
// campo con estos punteros).
var punterosDeLosCamposDeTexto = []func(*ObjectLookupRecord) *string{ //nolint:gochecknoglobals // orden del formato
	func(r *ObjectLookupRecord) *string { return &r.Name },
	func(r *ObjectLookupRecord) *string { return &r.Description },
	func(r *ObjectLookupRecord) *string { return &r.Base },
	func(r *ObjectLookupRecord) *string { return &r.Token },
	func(r *ObjectLookupRecord) *string { return &r.Mode },
	func(r *ObjectLookupRecord) *string { return &r.Class },
	func(r *ObjectLookupRecord) *string { return &r.HD },
	func(r *ObjectLookupRecord) *string { return &r.TR },
	func(r *ObjectLookupRecord) *string { return &r.LG },
	func(r *ObjectLookupRecord) *string { return &r.RA },
	func(r *ObjectLookupRecord) *string { return &r.LA },
	func(r *ObjectLookupRecord) *string { return &r.RH },
	func(r *ObjectLookupRecord) *string { return &r.LH },
	func(r *ObjectLookupRecord) *string { return &r.SH },
	func(r *ObjectLookupRecord) *string { return &r.S1 },
	func(r *ObjectLookupRecord) *string { return &r.S2 },
	func(r *ObjectLookupRecord) *string { return &r.S3 },
	func(r *ObjectLookupRecord) *string { return &r.S4 },
	func(r *ObjectLookupRecord) *string { return &r.S5 },
	func(r *ObjectLookupRecord) *string { return &r.S6 },
	func(r *ObjectLookupRecord) *string { return &r.S7 },
	func(r *ObjectLookupRecord) *string { return &r.S8 },
	func(r *ObjectLookupRecord) *string { return &r.ColorMap },
}

// init: en el HOST la fuente de los registros es ESTA tabla compilada (el host no cambia de comportamiento:
// sus pruebas y su motor siguen leyendo los registros de siempre, solo que el indice de Object.Lookup se
// arma por registro, cuando se pide, en vez de volcar los 7.891 en el arranque).
func init() {
	fuenteRegistrada = &fuenteEnMemoria{registros: objectLookups}
}

// ObjectLookupsCompilados devuelve la tabla de este archivo tal cual (la usa el generador del .pack).
func ObjectLookupsCompilados() []ObjectLookupRecord { return objectLookups }

// InformePack son los numeros del archivo que arma EscribirPack.
type InformePack struct {
	Bytes       int64
	Registros   int
	Grupos      int
	MaxRegistro int
	Crc         uint32
}

// EscribirPack escribe la tabla con el formato que lee la consola (el contrato esta en object_lookups_store.go)
// y devuelve sus numeros. FALLA (en vez de escribir algo que no cierra) si un registro no entra en el formato:
// un acto fuera de 0..255, un id fuera de 0..65535, un grupo cuyos ids no sean 0..n-1, un campo numerico fuera
// de i16, o una cadena de mas de 255 bytes.
func EscribirPack(w io.Writer, registros []ObjectLookupRecord) (InformePack, error) {
	informe := InformePack{Registros: len(registros)}

	if len(registros) == 0 {
		return informe, fmt.Errorf("objectLookups: no hay registros que empaquetar")
	}

	if len(registros) > 0xFFFF {
		return informe, fmt.Errorf("objectLookups: %d registros no entran en el formato (tope 65.535)", len(registros))
	}

	grupos := make([]grupoLookups, 0, 16)

	for i := range registros {
		r := &registros[i]

		if r.Act < 0 || r.Act > 0xFF {
			return informe, fmt.Errorf("objectLookups: registro %d: act %d no entra en un byte", i, r.Act)
		}

		if r.Type < 0 || int(r.Type) > 0xFF {
			return informe, fmt.Errorf("objectLookups: registro %d: tipo %d no entra en un byte", i, int(r.Type))
		}

		if r.Id < 0 || r.Id > 0xFFFF {
			return informe, fmt.Errorf("objectLookups: registro %d: id %d no entra en u16", i, r.Id)
		}

		if n := len(grupos); n > 0 && grupos[n-1].act == r.Act && grupos[n-1].typ == int(r.Type) {
			grupos[n-1].n++
			if r.Id != grupos[n-1].n-1 {
				return informe, fmt.Errorf("objectLookups: act %d tipo %d: el registro %d tiene id %d, esperaba %d (los ids de un grupo tienen que ser 0..n-1)",
					r.Act, int(r.Type), i, r.Id, grupos[n-1].n-1)
			}
		} else {
			if r.Id != 0 {
				return informe, fmt.Errorf("objectLookups: act %d tipo %d arranca con id %d (tiene que arrancar en 0)", r.Act, int(r.Type), r.Id)
			}

			grupos = append(grupos, grupoLookups{act: r.Act, typ: int(r.Type), base: i, n: 1})
		}
	}

	informe.Grupos = len(grupos)
	offsets := make([]uint32, len(registros))

	var bloque bytes.Buffer

	for i := range registros {
		if uint64(bloque.Len()) > 0xFFFFFFFF {
			return informe, fmt.Errorf("objectLookups: el bloque de registros pasa los 4 GiB")
		}

		offsets[i] = uint32(bloque.Len())
		antes := bloque.Len()

		if err := escribeRegistro(&bloque, &registros[i]); err != nil {
			return informe, fmt.Errorf("objectLookups: registro %d: %w", i, err)
		}

		if l := bloque.Len() - antes; l > informe.MaxRegistro {
			informe.MaxRegistro = l
		}
	}

	informe.Crc = fnv1a32(bloque.Bytes())

	idxOff := tamCabeceraLookups
	recOff := idxOff + len(registros)*4 + len(grupos)*8

	cab := make([]byte, tamCabeceraLookups)
	copy(cab[0:4], "D2OL")
	ponU16(cab[4:], 1)
	ponU32(cab[8:], uint32(len(registros)))
	ponU32(cab[12:], uint32(idxOff))
	ponU32(cab[16:], uint32(recOff))
	ponU16(cab[20:], len(grupos))
	ponU32(cab[24:], informe.Crc)
	ponU16(cab[28:], informe.MaxRegistro)

	idx := make([]byte, len(registros)*4)
	for i := range offsets {
		ponU32(idx[i*4:], offsets[i])
	}

	grs := make([]byte, len(grupos)*8)
	for i := range grupos {
		grs[i*8] = byte(grupos[i].act)
		grs[i*8+1] = byte(grupos[i].typ)
		ponU16(grs[i*8+4:], grupos[i].base)
		ponU16(grs[i*8+6:], grupos[i].n)
	}

	if err := escribeTodo(w, cab); err != nil {
		return informe, err
	}

	if err := escribeTodo(w, idx); err != nil {
		return informe, err
	}

	if err := escribeTodo(w, grs); err != nil {
		return informe, err
	}

	if err := escribeTodo(w, bloque.Bytes()); err != nil {
		return informe, err
	}

	informe.Bytes = int64(recOff) + int64(bloque.Len())

	return informe, nil
}

func escribeRegistro(b *bytes.Buffer, r *ObjectLookupRecord) error {
	b.WriteByte(byte(r.Act))
	b.WriteByte(byte(int(r.Type)))
	ponU16buf(b, r.Id)

	for c := range ordenDeLosCamposDeTexto {
		texto := ordenDeLosCamposDeTexto[c](r)
		if len(texto) > 0xFF {
			return fmt.Errorf("la cadena %q no entra en un byte de largo (%d)", texto, len(texto))
		}

		b.WriteByte(byte(len(texto)))
		b.WriteString(texto)
	}

	for _, v := range []int{r.ObjectsTxtId, r.MonstatsTxtId, r.Direction, r.Index} {
		if v < -32768 || v > 32767 {
			return fmt.Errorf("el campo numerico %d no entra en i16", v)
		}

		ponU16buf(b, v&0xFFFF)
	}

	return nil
}

func escribeTodo(w io.Writer, b []byte) error {
	for len(b) > 0 {
		n, err := w.Write(b)
		if err != nil {
			return err
		}

		b = b[n:]
	}

	return nil
}

func ponU16(buf []byte, v int) {
	buf[0] = byte(v)
	buf[1] = byte(v >> 8)
}

func ponU32(buf []byte, v uint32) {
	buf[0] = byte(v)
	buf[1] = byte(v >> 8)
	buf[2] = byte(v >> 16)
	buf[3] = byte(v >> 24)
}

func ponU16buf(b *bytes.Buffer, v int) {
	b.WriteByte(byte(v))
	b.WriteByte(byte(v >> 8))
}

func fnv1a32(b []byte) uint32 {
	h := uint32(2166136261)
	for i := range b {
		h ^= uint32(b[i])
		h *= 16777619
	}

	return h
}

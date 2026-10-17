// Package json: la parte de encoding/json que la consola puede tener, con un limite declarado.
//
// POR QUE NO ES EL JSON DE GO:
//
// El json de go serializa STRUCTS leyendo sus etiquetas ("`json:"heroName"`") y para eso necesita
// REFLEXION. En esta consola no hay reflexion: el paquete reflect son 43 paquetes de la biblioteca
// estandar y no estan portados.
//
// Lo que SI se puede: los tipos que se serializan solos. Si un tipo tiene MarshalJSON() ([]byte, error)
// o UnmarshalJSON(data []byte) error, aca se lo llama y listo. Es el caso de los tipos del motor que
// saben como escribirse: d2hero/hero_skill.go se define su propia forma a mano.
//
// LO QUE NO SE PUEDE (y no se finge): Marshal de un struct cualquiera, de un mapa, de un puntero a struct.
// Esas llamadas devuelven ErrSinReflexion, con ese nombre para que se entienda de una.
//
// POR QUE ALCANZA IGUAL: el motor usa json para GUARDAR Y LEER EL ESTADO EN DISCO (config, heroes,
// replays). La consola no tiene disco: el romdisk es de solo lectura. Esa parte del juego no esta en la
// ruta critica (decision tomada y escrita: la opcion C, single player). Lo que importa es que el motor
// COMPILE y que, si alguna vez se llama al guardado, falle con un mensaje que diga que pasa, en vez de
// quedarse callado o escribir basura.
//
// Medido antes de escribir esto: 88 archivos del motor importan encoding/json, con 72 llamadas
// (Marshal, Unmarshal, MarshalIndent, NewEncoder, NewDecoder), y practicamente todas son sobre structs.
package json

import "d2std/errors"

// ErrSinReflexion: el motor pidio serializar algo que en la consola no se puede (un struct, un mapa, un
// puntero a struct). No es un error de programacion: es el limite declarado de este paquete.
var ErrSinReflexion = errors.New("json: en la consola no hay reflexion: solo se pueden serializar los tipos que saben hacerlo ellos mismos (MarshalJSON/UnmarshalJSON)")

// Marshaler: el tipo sabe escribirse como json.
type Marshaler interface {
	MarshalJSON() ([]byte, error)
}

// Unmarshaler: el tipo sabe leerse desde json.
type Unmarshaler interface {
	UnmarshalJSON([]byte) error
}

// Marshal: si v sabe escribirse, se lo pide; si no, ErrSinReflexion.
func Marshal(v interface{}) ([]byte, error) {
	if m, ok := v.(Marshaler); ok {
		return m.MarshalJSON()
	}
	return nil, ErrSinReflexion
}

// MarshalIndent: lo mismo. No hay sangria: sin reflexion no se recorre la estructura para acomodarla, y
// agregarla a mano seria mentir sobre el formato.
func MarshalIndent(v interface{}, prefijo, sangria string) ([]byte, error) {
	return Marshal(v)
}

// Unmarshal: si v sabe leerese, se lo pide; si no, ErrSinReflexion.
func Unmarshal(data []byte, v interface{}) error {
	if u, ok := v.(Unmarshaler); ok {
		return u.UnmarshalJSON(data)
	}
	return ErrSinReflexion
}

// Valid: dice si los bytes son json bien formado. En la consola se limita a lo que el motor necesita: que
// el texto empiece y termine como un valor (esto NO es un validador completo, y esta dicho).
func Valid(data []byte) bool {
	t := recortar(data)
	if len(t) == 0 {
		return false
	}
	switch t[0] {
	case '{':
		return t[len(t)-1] == '}'
	case '[':
		return t[len(t)-1] == ']'
	case '"':
		return len(t) >= 2 && t[len(t)-1] == '"'
	}
	return true
}

func recortar(data []byte) []byte {
	i, j := 0, len(data)
	for i < j && (data[i] == ' ' || data[i] == '\t' || data[i] == '\n' || data[i] == '\r') {
		i++
	}
	for j > i && (data[j-1] == ' ' || data[j-1] == '\t' || data[j-1] == '\n' || data[j-1] == '\r') {
		j--
	}
	return data[i:j]
}

// Encoder / Decoder: la forma de streaming (escribir y leer json contra un archivo). El motor los usa
// para guardar. Aca estan para que compile; escribir o leer con ellos falla, que es lo que corresponde
// cuando no hay disco.
type Encoder struct {
	escribidor interface{}
}

// NewEncoder: el codificador que escribe en w.
func NewEncoder(w interface{}) *Encoder { return &Encoder{escribidor: w} }

// Encode: no se puede (ver Marshal).
func (e *Encoder) Encode(v interface{}) error { return ErrSinReflexion }

// SetIndent: la sangria no se aplica (ver MarshalIndent).
func (e *Encoder) SetIndent(prefijo, sangria string) {}

// SetEscapeHTML: sin efecto (el formato se arma a mano en cada tipo).
func (e *Encoder) SetEscapeHTML(b bool) {}

// Decoder: el lector.
type Decoder struct {
	lector interface{}
}

// NewDecoder: el decodificador que lee de r.
func NewDecoder(r interface{}) *Decoder { return &Decoder{lector: r} }

// Decode: no se puede (ver Unmarshal).
func (d *Decoder) Decode(v interface{}) error { return ErrSinReflexion }

// DisallowUnknownFields: sin efecto.
func (d *Decoder) DisallowUnknownFields() {}

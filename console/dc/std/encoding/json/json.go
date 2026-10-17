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
// DC-002.18 (medido, no supuesto): los tipos que se escriben a mano ALCANZAN, y lo que faltaba era el
// camino del medio. El arranque del motor se caia en `d2config.Save()` con ErrSinReflexion porque
// `Configuration` no tiene MarshalJSON/UnmarshalJSON propios (la tarjeta DC-002.18 se los escribio) y
// porque Encoder/Decoder devolvian ErrSinReflexion SIEMPRE, sin probar el Unmarshaler del tipo: un tipo
// que sabe leerse no se podia leer. Ahora los cinco puntos pasan por el Marshaler/Unmarshaler, y el
// limite queda donde corresponde (el tipo que no sabe).
//
// Lo que SI se puede: los tipos que se serializan solos. Si un tipo tiene MarshalJSON() ([]byte, error)
// o UnmarshalJSON(data []byte) error, aca se lo llama y listo. Es el caso de los tipos del motor que
// saben como escribirse: d2hero/hero_skill.go y d2input/d2gamepad/mapping.go se definen su propia forma a
// mano, y d2config/d2config_json.go es el que usa el arranque.
//
// Medido antes de escribir esto: 88 archivos del motor importan encoding/json, con 72 llamadas
// (Marshal, Unmarshal, MarshalIndent, NewEncoder, NewDecoder), y practicamente todas son sobre structs.
package json

import (
	"d2std/errors"
	"d2std/io"
)

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

// ------------------------------------------------------------------------------------------------
// EL INSTRUMENTO (DC-002.18): que tipo pidio serializar el motor, y si ese tipo sabe hacerlo.
//
// POR QUE ES UN PUNTO DE ENGANCHE Y NO UN println ACA: el NOMBRE del tipo no se puede sacar sin
// reflexion (que es justo lo que no hay), asi que hay que leer el descriptor de tipo con unsafe. Y eso
// solo vale en el binario de la CONSOLA, con el layout de gccgo: el MISMO codigo compilado en el host
// (gc) lee OTRO layout (el nombre del tipo es un nameOff de 4 bytes y no un puntero) y desreferenciaria
// basura. Por eso el paquete deja el punto de enganche y el que imprime es el POC de la consola, que es
// el unico que existe alla (console/dc/poc/motor-arranque y console/dc/poc/json-config, con la lectura
// del descriptor de las sondas de DC-002.2f, sonda-g). En el host queda en nil: no imprime nada.
//
// Cubre los CINCO puntos de serializacion (Marshal, MarshalIndent, Unmarshal, Encode y Decode), que es
// lo que permite decir con numeros cuantas llamadas hace el arranque del motor y sobre que tipos.
var Instrumento func(punto string, v interface{}, sabe bool)

func marcaInstrumento(punto string, v interface{}, escribir bool) {
	if Instrumento == nil {
		return
	}

	var sabe bool

	if escribir {
		_, sabe = v.(Marshaler)
	} else {
		_, sabe = v.(Unmarshaler)
	}

	Instrumento(punto, v, sabe)
}

// marshal es el camino de escritura: si v sabe escribirse, se lo pide; si no, ErrSinReflexion.
func marshal(v interface{}) ([]byte, error) {
	if m, ok := v.(Marshaler); ok {
		return m.MarshalJSON()
	}
	return nil, ErrSinReflexion
}

// Marshal: si v sabe escribirse, se lo pide; si no, ErrSinReflexion.
func Marshal(v interface{}) ([]byte, error) {
	marcaInstrumento("Marshal", v, true)

	return marshal(v)
}

// MarshalIndent: lo mismo, con la sangria. El json compacto que devuelve el tipo se vuelve a acomodar
// como TEXTO (ver Indent): no hace falta reflexion para eso, y asi el archivo que escribe la consola
// tiene los mismos bytes que el que escribe el host.
func MarshalIndent(v interface{}, prefijo, sangria string) ([]byte, error) {
	marcaInstrumento("MarshalIndent", v, true)

	b, err := marshal(v)
	if err != nil {
		return nil, err
	}

	return Indent(b, prefijo, sangria)
}

// unmarshal es el camino de lectura: si v sabe leerse, se lo pide; si no, ErrSinReflexion.
func unmarshal(data []byte, v interface{}) error {
	if u, ok := v.(Unmarshaler); ok {
		return u.UnmarshalJSON(data)
	}

	return ErrSinReflexion
}

// Unmarshal: si v sabe leerese, se lo pide; si no, ErrSinReflexion.
func Unmarshal(data []byte, v interface{}) error {
	marcaInstrumento("Unmarshal", v, false)

	return unmarshal(data, v)
}

// errSinOrigen y errSinDestino: el Encoder/Decoder se armo sin lector o sin escritor (el motor siempre
// le pasa uno: la configuracion la lee de su asset y la escribe con os.Create).
var (
	errSinOrigen  = errors.New("json: el Decoder no tiene de donde leer")
	errSinDestino = errors.New("json: el Encoder no tiene donde escribir")
)

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
// para guardar y leer su configuracion (d2config.Save y d2app.LoadConfig).
//
// DC-002.18: hasta esta tarjeta las dos devolvian ErrSinReflexion "porque no hay disco", y el diagnostico
// era FALSO (medido): el motor SI tiene su sistema de archivos escribible (el overlay en RAM de DC-002.17,
// que ya crea el directorio y el archivo) y lo que le faltaba era el camino de serializacion. Ahora las
// dos pasan por Marshal/Unmarshal, o sea por el Marshaler/Unmarshaler del tipo, y el limite queda donde
// corresponde: en el tipo que no sabe serializarse solo.
type Encoder struct {
	escribidor io.Writer
	prefijo    string
	sangria    string
	conSangria bool
}

// NewEncoder: el codificador que escribe en w.
func NewEncoder(w io.Writer) *Encoder { return &Encoder{escribidor: w} }

// Encode: escribe el json del tipo y despues un salto de linea (igual que el Encode de Go).
func (e *Encoder) Encode(v interface{}) error {
	marcaInstrumento("Encode", v, true)

	if e.escribidor == nil {
		return errSinDestino
	}

	b, err := marshal(v)
	if err != nil {
		return err
	}

	if e.conSangria {
		if b, err = Indent(b, e.prefijo, e.sangria); err != nil {
			return err
		}
	}

	b = append(b, '\n')

	_, err = e.escribidor.Write(b)

	return err
}

// SetIndent: la sangria del Encoder (el mismo efecto que en Go: el texto sale acomodado).
func (e *Encoder) SetIndent(prefijo, sangria string) {
	e.prefijo = prefijo
	e.sangria = sangria
	e.conSangria = true
}

// SetEscapeHTML: sin efecto (el formato lo arma a mano cada tipo, que es el que sabe que hay que
// escapar).
func (e *Encoder) SetEscapeHTML(b bool) {}

// Decoder: el lector.
type Decoder struct {
	lector io.Reader
}

// NewDecoder: el decodificador que lee de r.
func NewDecoder(r io.Reader) *Decoder { return &Decoder{lector: r} }

// Decode: lee el lector ENTERO y se lo pasa al tipo (Unmarshal).
//
// LIMITE DECLARADO (medido contra Go en el difftest del host): el Decode de Go lee UN valor del flujo y
// deja el resto en su buffer interno; el nuestro lee todo y se lo entrega al tipo, que es el que sabe
// donde termina. Con el caso del motor (config.json, un solo valor) es exactamente lo mismo.
func (d *Decoder) Decode(v interface{}) error {
	marcaInstrumento("Decode", v, false)

	if d.lector == nil {
		return errSinOrigen
	}

	datos, err := io.ReadAll(d.lector)
	if err != nil {
		return err
	}

	return unmarshal(datos, v)
}

// DisallowUnknownFields: sin efecto.
func (d *Decoder) DisallowUnknownFields() {}

// RawMessage: un pedazo de JSON guardado tal cual, sin interpretarlo.
//
// PARA QUE SE USA: d2networking/d2netpacket lo usa para el contenido del mensaje (el sobre se sabe leer, y
// adentro va el paquete que se manda tal como lo armo el que lo escribio). Es justo el caso en el que NO
// hace falta reflexion: nadie mira adentro, solo lo guarda y lo vuelve a mandar.
//
// Las dos funciones son las de Go, tal cual: si esta vacio se escribe `null`, y al leer se guarda una copia
// (copiar importa: el pedazo que llega se puede reusar arriba y quedaria cambiando solo).
type RawMessage []byte

// MarshalJSON: lo devuelve tal cual (o `null` si esta vacio).
func (m RawMessage) MarshalJSON() ([]byte, error) {
	if m == nil {
		return []byte("null"), nil
	}
	return m, nil
}

// UnmarshalJSON: guarda una copia de lo que llego.
func (m *RawMessage) UnmarshalJSON(datos []byte) error {
	if m == nil {
		return errors.New("json.RawMessage: UnmarshalJSON sobre un puntero nil")
	}
	*m = append((*m)[0:0], datos...)
	return nil
}

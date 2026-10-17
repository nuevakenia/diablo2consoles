// Prueba diferencial de DC-002.18: el json de la consola contra el encoding/json de Go, en los CINCO
// puntos de serializacion.
//
// QUE ES LO QUE SE ESTA MIDIENDO (y por que el corpus de la consola no alcanza solo):
//
//  1. Indent: nuestro re-acomodador de texto contra json.Indent de Go. Es el juez mas fuerte que tiene
//     esta parte porque es una funcion de TEXTO: la misma entrada y la salida tiene que ser byte a byte
//     igual (o, si el texto no es json valido, los dos tienen que RECHAZARLO; el mensaje del error no se
//     compara y esta declarado en indent.go).
//  2. Marshal / MarshalIndent de un tipo que sabe escribirse: contra json.Marshal / json.MarshalIndent.
//     Los bytes tienen que ser los mismos (es lo que hace que el config.json de la consola y el del host
//     sean el mismo archivo).
//  3. Unmarshal / Decode: contra json.Unmarshal. Lo que se compara es lo que le LLEGA al tipo.
//  4. Encode: contra json.NewEncoder(...).Encode.
//  5. El instrumento (json.Instrumento): que los cinco puntos avisen, que es lo que permite medir en la
//     consola cuantas llamadas hace el arranque del motor y sobre que tipos.
//
// LIMITES DECLARADOS (con su test, para que no se muevan sin que nadie se entere):
//  * Marshal de Go COMPACTA (y escapa <, >, &) lo que devuelve un MarshalJSON; el nuestro devuelve los
//    bytes del tipo tal cual. El contrato nuestro es que el tipo escriba compacto (el difftest exige la
//    igualdad en ese caso).
//  * Decode de Go lee UN valor del flujo y guarda el resto; el nuestro lee el lector entero y se lo pasa
//    al tipo. Con un lector de un solo valor (el caso del motor: config.json) es lo mismo.
package difftest

import (
	"bytes"
	realjson "encoding/json"
	"fmt"
	"strings"
	"testing"

	myjson "d2std/encoding/json"
	myio "d2std/io"
)

// ---- un tipo como los del motor: se escribe y se lee solo, sin reflexion ----

type configPrueba struct {
	Nombre  string
	Ancho   int
	Volumen float64
	Opciones map[string]int
}

func (c *configPrueba) MarshalJSON() ([]byte, error) {
	// compacto y con los campos en el orden de la declaracion, como el json de Go
	out := fmt.Sprintf(`{"Nombre":%q,"Ancho":%d,"Volumen":%g,"Opciones":{`, c.Nombre, c.Ancho, c.Volumen)
	primero := true
	for _, k := range sortedKeys(c.Opciones) {
		if !primero {
			out += ","
		}
		out += fmt.Sprintf("%q:%d", k, c.Opciones[k])
		primero = false
	}
	out += "}}"

	return []byte(out), nil
}

func (c *configPrueba) UnmarshalJSON(data []byte) error {
	// lectura de verdad, del mismo texto que escribe el motor... y de cualquier json con esa forma
	var crudo map[string]realjson.RawMessage
	if err := realjson.Unmarshal(data, &crudo); err != nil {
		return err
	}
	if v, ok := crudo["Nombre"]; ok {
		if err := realjson.Unmarshal(v, &c.Nombre); err != nil {
			return err
		}
	}
	if v, ok := crudo["Ancho"]; ok {
		if err := realjson.Unmarshal(v, &c.Ancho); err != nil {
			return err
		}
	}
	if v, ok := crudo["Volumen"]; ok {
		if err := realjson.Unmarshal(v, &c.Volumen); err != nil {
			return err
		}
	}
	if v, ok := crudo["Opciones"]; ok {
		if err := realjson.Unmarshal(v, &c.Opciones); err != nil {
			return err
		}
	}

	return nil
}

func sortedKeys(m map[string]int) []string {
	ks := make([]string, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	for i := 1; i < len(ks); i++ {
		for j := i; j > 0 && ks[j] < ks[j-1]; j-- {
			ks[j], ks[j-1] = ks[j-1], ks[j]
		}
	}

	return ks
}

func config() *configPrueba {
	return &configPrueba{
		Nombre:   "OpenDiablo2",
		Ancho:    1512,
		Volumen:  0.3,
		Opciones: map[string]int{"sonido": 4, "musica": 7, "gamma": 8},
	}
}

// ---- 1. Indent contra json.Indent ----

type casoIndent struct {
	nombre string
	texto  string
	prefijo string
	sangria string
}

// noComparar: los casos que quedan declarados como limite (con el motivo). Estan aca y no escondidos en
// un `continue`: el test exige que la lista sea esa y no mas.
//
// (Vacia por ahora, y esta bien que este: el limite que se habia escrito -- que no validamos UTF-8 dentro
// de las cadenas -- NO era un limite, medido: json.Indent de Go tampoco lo valida. La lista queda para el
// proximo caso que se declare.)
var limiteIndent = map[string]string{}

var casosIndent = []casoIndent{
	{"objeto simple", `{"a":1}`, "", "  "},
	{"dos campos", `{"a":1,"b":[1,2,3]}`, "", "  "},
	{"anidado", `{"a":{"b":{"c":[{"d":null}]}}}`, "", "  "},
	{"vacias", `{"a":{},"b":[],"c":{"d":[]}}`, "", "  "},
	{"array de arrays", `[[],[[]],[[[]]]]`, "", "  "},
	{"con blanco de sobra", "{\n  \"a\" :\t1 ,\r\n \"b\"  :  [ 1 , 2 ]  }", "", "  "},
	{"prefijo y sangria", `{"a":{"b":1}}`, ">>", "\t"},
	{"sangria vacia", `{"a":{"b":1}}`, "", ""},
	{"numeros", `[0,-0,1,-1,1.5,-1.5,1e3,1E+3,1e-3,0.5e10,12345678901234567890]`, "", "  "},
	{"literales", `[true,false,null]`, "", "  "},
	{"cadena con escapes", `{"a":"cita \" barra \\ barra/ \b\f\n\r\t u \u00e9 \u4e2d"}`, "", "  "},
	{"cadenas raras", `[ "", " ", "{}", "[]", ":", ",", "\"a\"" ]`, "", "  "},
	{"clave con escapes", `{"a\"b":1,"c\\d":2,"\u00e9":3}`, "", "  "},
	{"raiz suelto", `"solo texto"`, "", "  "},
	{"raiz numero", `42`, "", "  "},
	{"raiz literal", "true", "", "  "},
	{"raiz array", `[1]`, "", "  "},
	{"blancos alrededor", "\n\t {\"a\"  :  1} \r\n", "", "  "},
	{"objeto grande", `{"a":1,"b":{"c":[1,2,{"d":"e"}],"f":null},"g":[[],{}]}`, "", "     "},

	// --- los que TIENEN que fallar (los dos lados) ---
	{"vacio", ``, "", "  "},
	{"solo blancos", "  \n ", "", "  "},
	{"abre sin cerrar", `{"a":1`, "", "  "},
	{"cierra sin abrir", `1}`, "", "  "},
	{"dos valores en la raiz", `{"a":1}{"b":2}`, "", "  "},
	{"basura despues", `{"a":1} x`, "", "  "},
	{"coma de mas", `{"a":1,}`, "", "  "},
	{"coma de mas en array", `[1,]`, "", "  "},
	{"coma de menos", `{"a":1"b":2}`, "", "  "},
	{"clave sin dos puntos", `{"a" 1}`, "", "  "},
	{"clave numero", `{1:2}`, "", "  "},
	{"clave sin cerrar", `{"a:1}`, "", "  "},
	{"cadena sin cerrar", `{"a":"b}`, "", "  "},
	{"escape raro", `{"a":"\x"}`, "", "  "},
	{"escape u corto", `{"a":"\u12"}`, "", "  "},
	{"control crudo", "{\"a\":\"b\tc\"}", "", "  "},
	{"numero mal: cero a la izquierda", `[01]`, "", "  "},
	{"numero mal: punto suelto", `[1.]`, "", "  "},
	{"numero mal: sin exponente", `[1e]`, "", "  "},
	{"numero mal: solo signo", `[-]`, "", "  "},
	{"numero mal: mas", `[+1]`, "", "  "},
	{"literal mal", `[tru]`, "", "  "},
	{"literal pegado", `[truex]`, "", "  "},
	{"literal sin cerrar", `{"a":nul}`, "", "  "},
	{"utf8 invalido en una cadena", "{\"a\":\"\xff\"}", "", "  "},
}

func TestDC00218IndentContraGo(t *testing.T) {
	comparados := 0

	for _, caso := range casosIndent {
		esperado := &bytes.Buffer{}
		errGo := realjson.Indent(esperado, []byte(caso.texto), caso.prefijo, caso.sangria)

		nuestro, errMio := myjson.Indent([]byte(caso.texto), caso.prefijo, caso.sangria)

		if _, esLimite := limiteIndent[caso.nombre]; esLimite {
			if errMio == nil && errGo == nil {
				t.Errorf("%s: el limite declarado (%s) ya no es un limite: los dos aceptan", caso.nombre, limiteIndent[caso.nombre])
			}
			continue
		}

		comparados++

		if (errGo == nil) != (errMio == nil) {
			t.Errorf("%s: Go err=%v, nosotros err=%v (entrada %q)", caso.nombre, errGo, errMio, caso.texto)
			continue
		}

		if errMio != nil {
			continue
		}

		if string(nuestro) != esperado.String() {
			t.Errorf("%s:\n  Go:       %q\n  nosotros: %q", caso.nombre, esperado.String(), string(nuestro))
		}
	}

	t.Logf("casos de Indent comparados contra json.Indent: %d (mas %d declarados como limite)",
		comparados, len(limiteIndent))
}

// ---- 2. Marshal / MarshalIndent contra Go ----

func TestDC00218MarshalContraGo(t *testing.T) {
	c := config()

	goCompacto, err := realjson.Marshal(c)
	if err != nil {
		t.Fatalf("json.Marshal de Go: %v", err)
	}

	nuestro, err := myjson.Marshal(c)
	if err != nil {
		t.Fatalf("nuestro Marshal: %v", err)
	}

	if string(nuestro) != string(goCompacto) {
		t.Errorf("Marshal:\n  Go:       %s\n  nosotros: %s", goCompacto, nuestro)
	}

	goIndentado, err := realjson.MarshalIndent(c, "", "  ")
	if err != nil {
		t.Fatalf("json.MarshalIndent de Go: %v", err)
	}

	nuestroIndentado, err := myjson.MarshalIndent(c, "", "  ")
	if err != nil {
		t.Fatalf("nuestro MarshalIndent: %v", err)
	}

	if string(nuestroIndentado) != string(goIndentado) {
		t.Errorf("MarshalIndent:\n  Go:\n%s\n  nosotros:\n%s", goIndentado, nuestroIndentado)
	}
}

// ---- 3. Unmarshal / Decode / Encode ----

// lectorDePrueba: el lector del caso del motor (un asset de un solo valor). Devuelve el EOF del paquete
// io NUESTRO (myio.EOF) y no el de la biblioteca de Go a proposito: en la consola el que devuelve el
// arranque del archivo es nuestro io, y el json compara contra ESE (la misma leccion de DC-002.5b: el
// io.EOF de Go y el de d2std son dos valores distintos).
type lectorDePrueba struct {
	datos []byte
	i     int
}

func (l *lectorDePrueba) Read(p []byte) (int, error) {
	if l.i >= len(l.datos) {
		return 0, myio.EOF
	}
	n := copy(p, l.datos[l.i:])
	l.i += n

	return n, nil
}

func TestDC00218UnmarshalContraGo(t *testing.T) {
	texto := `{"Nombre":"hola","Ancho":-3,"Volumen":1.5,"Opciones":{"a":1,"b":2}}`

	for _, lado := range []string{"Go", "nosotros"} {
		var c configPrueba

		var err error

		switch lado {
		case "Go":
			err = realjson.Unmarshal([]byte(texto), &c)
		default:
			err = myjson.Unmarshal([]byte(texto), &c)
		}

		if err != nil {
			t.Fatalf("%s: %v", lado, err)
		}
		if c.Nombre != "hola" || c.Ancho != -3 || c.Volumen != 1.5 || c.Opciones["b"] != 2 {
			t.Errorf("%s: leyo %+v", lado, c)
		}
	}

	// sin Unmarshaler: los dos avisan (el de Go por un tipo que no puede, el nuestro por el limite)
	var n int
	if err := myjson.Unmarshal([]byte("1"), &n); err != myjson.ErrSinReflexion {
		t.Errorf("Unmarshal de un int suelto: %v (se espera ErrSinReflexion)", err)
	}
}

// TestDC00218DecodeEncode: el camino del MOTOR (`config.json`): d2app usa json.NewDecoder(configAsset).
// Decode(config) para LEER y d2config.Save usa MarshalIndent para ESCRIBIR.
func TestDC00218DecodeEncode(t *testing.T) {
	texto := `{"Nombre":"leido","Ancho":7,"Volumen":0.5,"Opciones":{"x":9}}`

	var c configPrueba

	dec := myjson.NewDecoder(&lectorDePrueba{datos: []byte(texto)})
	if err := dec.Decode(&c); err != nil {
		t.Fatalf("Decode: %v", err)
	}

	if c.Nombre != "leido" || c.Ancho != 7 || c.Opciones["x"] != 9 {
		t.Errorf("Decode leyo %+v", c)
	}

	// el del host, el mismo texto
	var goC configPrueba
	if err := realjson.NewDecoder(strings.NewReader(texto)).Decode(&goC); err != nil {
		t.Fatalf("Decode de Go: %v", err)
	}

	if goC.Nombre != c.Nombre || goC.Ancho != c.Ancho || goC.Volumen != c.Volumen || goC.Opciones["x"] != c.Opciones["x"] {
		t.Errorf("Decode: Go %+v, nosotros %+v", goC, c)
	}

	// Encode: lo que escribe el Encoder tiene que ser lo mismo que lo que escribiria el de Go
	salida := &strings.Builder{}
	if err := myjson.NewEncoder(salida).Encode(config()); err != nil {
		t.Fatalf("Encode: %v", err)
	}

	goSalida := &strings.Builder{}
	if err := realjson.NewEncoder(goSalida).Encode(config()); err != nil {
		t.Fatalf("Encode de Go: %v", err)
	}

	if salida.String() != goSalida.String() {
		t.Errorf("Encode:\n  Go:       %q\n  nosotros: %q", goSalida.String(), salida.String())
	}
}

// ---- 4. el instrumento: los cinco puntos avisan ----

func TestDC00218Instrumento(t *testing.T) {
	var puntos []string

	myjson.Instrumento = func(punto string, v interface{}, sabe bool) {
		estado := "no"
		if sabe {
			estado = "si"
		}
		puntos = append(puntos, punto+"/"+estado)
	}

	defer func() { myjson.Instrumento = nil }()

	c := config()
	if _, err := myjson.Marshal(c); err != nil {
		t.Fatal(err)
	}
	if _, err := myjson.MarshalIndent(c, "", "  "); err != nil {
		t.Fatal(err)
	}
	if err := myjson.Unmarshal([]byte(`{"Nombre":"a"}`), c); err != nil {
		t.Fatal(err)
	}
	if err := myjson.NewDecoder(&lectorDePrueba{datos: []byte(`{"Ancho":1}`)}).Decode(c); err != nil {
		t.Fatal(err)
	}
	if err := myjson.NewEncoder(&strings.Builder{}).Encode(c); err != nil {
		t.Fatal(err)
	}

	esperado := "Marshal/si MarshalIndent/si Unmarshal/si Decode/si Encode/si"
	if strings.Join(puntos, " ") != esperado {
		t.Errorf("los puntos avisados: %q (se esperaban %q)", strings.Join(puntos, " "), esperado)
	}
}

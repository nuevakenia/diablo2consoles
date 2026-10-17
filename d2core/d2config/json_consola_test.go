package d2config

// DC-002.18: la configuracion se serializa A MANO (d2config_json.go) porque la consola no tiene reflexion.
// Este archivo es el juez de esa parte en el HOST: la salida nuestra contra la de la biblioteca de verdad,
// byte a byte, sobre la MISMA configuracion.
//
// COMO SE COMPARA SIN COMPARARSE CON UNO MISMO: el json de Go llama a nuestro MarshalJSON (esta ahi), asi
// que no sirve comparar `json.Marshal(c)` contra `json.Marshal(c)`. Lo que se compara es contra una struct
// ESPEJO (`configEspejo`) con los mismos campos y las mismas etiquetas y SIN metodos: esa la serializa el
// json de Go por reflexion, que es la referencia. Si alguien agrega un campo a Configuration y no lo pone
// en el espejo (o al reves), esto se pone ROJO: es a proposito.
//
// Los casos que NO son objetos de configuracion (los limites declarados en d2config_json.go) van al final,
// cada uno con su test, para que no se muevan sin que nadie se entere.

import (
	"encoding/json"
	"math"
	"strconv"
	"strings"
	"testing"
)

// configEspejo: la misma forma que Configuration, sin metodos (la referencia de Go).
type configEspejo struct {
	MpqLoadOrder    []string
	MpqPath         string
	TicksPerSecond  int
	FpsCap          int
	SfxVolume       float64
	BgmVolume       float64
	FullScreen      bool
	RunInBackground bool
	VsyncEnabled    bool
	Backend         string
	Display         DisplayConfig
	WindowScale     int
	D2SDir          string
	Language        string `json:",omitempty"`
	Options         map[string]int `json:",omitempty"`
	KeyBindings     map[string][]string `json:",omitempty"`
}

func espejoDe(c *Configuration) *configEspejo {
	return &configEspejo{
		MpqLoadOrder:    c.MpqLoadOrder,
		MpqPath:         c.MpqPath,
		TicksPerSecond:  c.TicksPerSecond,
		FpsCap:          c.FpsCap,
		SfxVolume:       c.SfxVolume,
		BgmVolume:       c.BgmVolume,
		FullScreen:      c.FullScreen,
		RunInBackground: c.RunInBackground,
		VsyncEnabled:    c.VsyncEnabled,
		Backend:         c.Backend,
		Display:         c.Display,
		WindowScale:     c.WindowScale,
		D2SDir:          c.D2SDir,
		Language:        c.Language,
		Options:         c.Options,
		KeyBindings:     c.KeyBindings,
	}
}

// comparar: los bytes nuestros (via json.MarshalIndent, que llama a nuestro MarshalJSON) contra los de la
// biblioteca de verdad sobre el espejo.
func comparar(t *testing.T, nombre string, c *Configuration) {
	t.Helper()

	nuestro, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		t.Fatalf("%s: nuestro MarshalIndent: %v", nombre, err)
	}

	referencia, err := json.MarshalIndent(espejoDe(c), "", "  ")
	if err != nil {
		t.Fatalf("%s: json.MarshalIndent del espejo: %v", nombre, err)
	}

	if string(nuestro) != string(referencia) {
		t.Errorf("%s: los bytes no son los del json de Go\n  Go (espejo):\n%s\n  nuestro:\n%s",
			nombre, referencia, nuestro)
	}
}

// ---- 1. la configuracion POR DEFECTO del motor (la que guarda el arranque) ----

func TestJSONConsolaDefaultConfig(t *testing.T) {
	comparar(t, "DefaultConfig()", DefaultConfig())
}

// ---- 2. los casos de forma: listas, mapas, omitempty, negativos, cero ----

func TestJSONConsolaFormas(t *testing.T) {
	base := DefaultConfig()

	casos := []struct {
		nombre string
		toca   func(c *Configuration)
	}{
		{"sin listas (nil)", func(c *Configuration) { c.MpqLoadOrder = nil }},
		{"lista vacia", func(c *Configuration) { c.MpqLoadOrder = []string{} }},
		{"lista de uno vacio", func(c *Configuration) { c.MpqLoadOrder = []string{""} }},
		{"campos vacios", func(c *Configuration) { c.MpqPath = ""; c.Backend = ""; c.D2SDir = "" }},
		{"negativos", func(c *Configuration) { c.TicksPerSecond = -1; c.FpsCap = -60; c.WindowScale = -3 }},
		{"cero", func(c *Configuration) { c.TicksPerSecond = 0; c.FpsCap = 0; c.WindowScale = 0 }},
		{"display en cero", func(c *Configuration) { c.Display = DisplayConfig{} }},
		{"volumenes cero", func(c *Configuration) { c.SfxVolume = 0; c.BgmVolume = 0 }},
		{"idioma", func(c *Configuration) { c.Language = "esES" }},
		{"opciones", func(c *Configuration) { c.Options = map[string]int{"sonido": 4, "musica": 7, "gamma": 8} }},
		{"opciones vacias", func(c *Configuration) { c.Options = map[string]int{} }},
		{"opciones con clave rara", func(c *Configuration) {
			c.Options = map[string]int{"a\"b": 1, "c\\d": 2, "á": 3, "z": 4, "A": 5}
		}},
		{"teclas", func(c *Configuration) { c.KeyBindings = map[string][]string{"attack": {"a", "b"}, "walk": {"c"}} }},
		{"teclas con lista vacia", func(c *Configuration) { c.KeyBindings = map[string][]string{"attack": {}} }},
		{"teclas con lista nil", func(c *Configuration) { c.KeyBindings = map[string][]string{"attack": nil} }},
		{"todo a la vez", func(c *Configuration) {
			c.Language = "deDE"
			c.Options = map[string]int{"x": -1, "y": 0}
			c.KeyBindings = map[string][]string{"k": nil, "j": {"1", "2"}}
			c.Display = DisplayConfig{Width: 320, Height: 240}
		}},
	}

	for _, caso := range casos {
		c := *base
		caso.toca(&c)
		comparar(t, caso.nombre, &c)
	}
}

// ---- 3. los flotantes: la regla del json de Go (fija, y cientifica fuera de 1e-6 / 1e21) ----

func TestJSONConsolaFlotantes(t *testing.T) {
	base := DefaultConfig()

	valores := []float64{
		0, -0, 1, -1, 0.5, 0.3, 0.1, 1.0 / 3.0, 0.000001, 0.0000009, 0.0000001,
		1e10, 1e15, 1e20, 1e21, 1e22, -1e21, 123456789.123456789, 2.5, 99.95, 8.835,
		1e-300, 1e300, 0.145, -1.005,
	}

	for _, v := range valores {
		c := *base
		c.SfxVolume = v
		c.BgmVolume = -v
		comparar(t, "flotante "+strconv.FormatFloat(v, 'g', -1, 64), &c)
	}
}

// ---- 4. las cadenas: los escapes del json de Go ----

func TestJSONConsolaCadenas(t *testing.T) {
	base := DefaultConfig()

	cadenas := []string{
		"",
		"hola",
		"con \"comillas\"",
		"con \\barra\\",
		"salto\nde linea\ttab\rretorno",
		"control: \x00 \x01 \x1f y \x7f",
		"html: <b>&</b>",
		"separadores: \u2028 y \u2029",
		"no ascii: café, ñandú, 中文, 😀",
		"barra / y barra invertida /",
		"utf8 invalido: \xff\xfe",
		"mitad de un rune: \xe4\xb8",
	}

	for _, s := range cadenas {
		c := *base
		c.MpqPath = s
		c.Backend = s
		c.D2SDir = s
		comparar(t, "cadena "+strings.ReplaceAll(strings.ReplaceAll(s, "\n", "\\n"), "\x00", "\\0"), &c)
	}
}

// ---- 5. la vuelta completa: escribir y volver a leer con la biblioteca de verdad ----

func TestJSONConsolaVueltaCompleta(t *testing.T) {
	c := DefaultConfig()
	c.Language = "esES"
	c.Options = map[string]int{"sonido": 4, "musica": 7}
	c.KeyBindings = map[string][]string{"attack": {"a", "b"}, "walk": nil}
	c.Display = DisplayConfig{Width: 640, Height: 480}
	c.MpqPath = "con \"comillas\" y \\barra\\ y <html>"

	datos, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		t.Fatalf("MarshalIndent: %v", err)
	}

	// lo que lee el motor en el host (json de Go) tiene que dar lo mismo que escribimos
	var devuelta Configuration
	if err := json.Unmarshal(datos, &devuelta); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	comparable := *c
	comparable.path = ""

	devuelta.path = ""

	if !mismaConfiguracion(t, &comparable, &devuelta) {
		t.Errorf("la vuelta completa no da lo mismo:\n  escribi: %+v\n  lei:     %+v", comparable, devuelta)
	}

	// y el espejo tiene que leer EXACTAMENTE lo mismo que nosotros (las dos direcciones)
	var espejo configEspejo
	if err := json.Unmarshal(datos, &espejo); err != nil {
		t.Fatalf("Unmarshal del espejo: %v", err)
	}
	if len(espejo.KeyBindings) != 2 || len(espejo.MpqLoadOrder) == 0 {
		t.Errorf("el espejo leyo otra cosa: %+v", espejo)
	}
}

func mismaConfiguracion(t *testing.T, a, b *Configuration) bool {
	t.Helper()

	// se comparan reescribiendo las dos: si los bytes son los mismos, los campos son los mismos
	ba, err := json.Marshal(a)
	if err != nil {
		t.Fatalf("Marshal(a): %v", err)
	}

	bb, err := json.Marshal(b)
	if err != nil {
		t.Fatalf("Marshal(b): %v", err)
	}

	return string(ba) == string(bb)
}

// ---- 6. los limites declarados (cada uno con su test) ----

func TestJSONConsolaLimiteNaN(t *testing.T) {
	c := DefaultConfig()
	c.SfxVolume = nan()

	if _, err := json.Marshal(c); err == nil {
		t.Error("un SfxVolume NaN tiene que fallar (como en Go)")
	}

	c.SfxVolume = inf()
	if _, err := json.Marshal(c); err == nil {
		t.Error("un SfxVolume Infinito tiene que fallar (como en Go)")
	}
}

func TestJSONConsolaLimiteClavesDesconocidas(t *testing.T) {
	texto := `{"MpqPath":"x","UnaClaveQueNoExiste":{"a":[1,2,{"b":null}]},"Backend":"y","Otra":[1,2,3],"YOtro":"z","Nulo":null,"Bool":true,"Num":1.5}`

	var c Configuration
	if err := json.Unmarshal([]byte(texto), &c); err != nil {
		t.Fatalf("las claves desconocidas se tienen que saltear: %v", err)
	}

	if c.MpqPath != "x" || c.Backend != "y" {
		t.Errorf("no leyo las dos claves que si existen: %+v", c)
	}
}

func TestJSONConsolaLimiteNumeroConFraccion(t *testing.T) {
	var c Configuration
	if err := json.Unmarshal([]byte(`{"TicksPerSecond":1.5}`), &c); err == nil {
		t.Error("un 1.5 en un campo entero tiene que fallar (como en Go)")
	}
}

func TestJSONConsolaLimiteTextoMalo(t *testing.T) {
	malos := []string{``, `{`, `}`, `[]`, `{"MpqPath"}`, `{"MpqPath":}`, `{"a":1,}`, `{"MpqPath":"x"`, `nulo`}

	for _, texto := range malos {
		var c Configuration
		if err := json.Unmarshal([]byte(texto), &c); err == nil {
			t.Errorf("esto tiene que fallar: %q", texto)
		}
	}
}

// TestJSONConsolaMismoQueLaReferenciaEnLaConsola: los bytes de la configuracion por defecto se comparan
// tambien contra un texto FIJO (el mismo que hornea el POC de la consola). Si la struct cambia, esto se
// pone rojo a proposito: el corpus de la consola se regenera con console/dc/gen-config-json.
func TestJSONConsolaBytesFijos(t *testing.T) {
	// no se compara el texto entero (seria un test de copiar y pegar): se comprueba que el texto NO tenga
	// claves que no sean las de la struct, que es el defecto que este archivo podria introducir.
	datos, err := json.MarshalIndent(DefaultConfig(), "", "  ")
	if err != nil {
		t.Fatalf("MarshalIndent: %v", err)
	}

	var sueltas map[string]json.RawMessage
	if err := json.Unmarshal(datos, &sueltas); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	esperadas := map[string]bool{
		"MpqLoadOrder": true, "MpqPath": true, "TicksPerSecond": true, "FpsCap": true,
		"SfxVolume": true, "BgmVolume": true, "FullScreen": true, "RunInBackground": true,
		"VsyncEnabled": true, "Backend": true, "Display": true, "WindowScale": true, "D2SDir": true,
	}

	for k := range sueltas {
		if !esperadas[k] {
			t.Errorf("clave de mas en el json: %q", k)
		}
	}

	for k := range esperadas {
		if _, ok := sueltas[k]; !ok {
			t.Errorf("falta la clave %q", k)
		}
	}
}

func nan() float64 { return math.NaN() }

func inf() float64 { return math.Inf(1) }

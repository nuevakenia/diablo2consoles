// Genera la configuracion de prueba del corpus de DC-002.18 y los bytes que el HOST espera para ella.
//
// POR QUE GENERA LAS DOS COSAS (la configuracion y los bytes esperados): en la consola no esta la
// biblioteca de verdad, asi que lo unico honesto es comparar contra lo que la biblioteca de verdad
// escribio. Y si la configuracion la armara dos veces (una vez aca y otra en el POC de la consola), una
// deriva entre las dos se veria como un fallo del serializador que se esta probando. Asi hay UNA sola
// fuente: este archivo.
//
// EL ESPEJO DEL HOST NO PUEDE SER `json.Marshal(c)`: el json de Go llama a nuestro MarshalJSON (que es lo
// que se esta probando). Los bytes esperados salen de una struct espejo con los mismos campos y las mismas
// etiquetas, que es la que serializa el json de Go por reflexion: eso es la referencia.
//
// Uso:  go run ./console/dc/gen-config-json > <build>/gen/config_gen.go
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2config"
)

// espejo: la misma forma que d2config.Configuration, sin metodos: la referencia de Go.
type espejo struct {
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
	Display         d2config.DisplayConfig
	WindowScale     int
	D2SDir          string
	Language        string              `json:",omitempty"`
	Options         map[string]int      `json:",omitempty"`
	KeyBindings     map[string][]string `json:",omitempty"`
}

func espejoDe(c *d2config.Configuration) *espejo {
	return &espejo{
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

// configDePrueba: la configuracion del caso principal (la forma tipica de un config.json del motor, con
// los campos que tienen algo).
func configDePrueba() *d2config.Configuration {
	return &d2config.Configuration{
		MpqLoadOrder:    []string{"patch_d2.mpq", "d2data.mpq", "d2char.mpq"},
		MpqPath:         "C:/Program Files (x86)/Diablo II",
		TicksPerSecond:  -1,
		FpsCap:          0,
		SfxVolume:       1,
		BgmVolume:       0.3,
		FullScreen:      false,
		RunInBackground: true,
		VsyncEnabled:    true,
		Backend:         "Ebiten",
		Display:         d2config.DisplayConfig{Width: 1512, Height: 982},
		WindowScale:     0,
		D2SDir:          "",
		Language:        "esES",
		Options:         map[string]int{"gamma": 8, "musica": 7, "sonido": 4},
		KeyBindings:     map[string][]string{"attack": {"a", "b"}, "walk": nil},
	}
}

// configVariante: el caso de los bordes: una lista nil (que en json es `null`), campos vacios, numeros
// negativos, una lista vacia y un idioma que se omite.
func configVariante() *d2config.Configuration {
	return &d2config.Configuration{
		MpqLoadOrder:    nil,
		MpqPath:         "",
		TicksPerSecond:  1,
		FpsCap:          -60,
		SfxVolume:       0,
		BgmVolume:       1.5,
		FullScreen:      true,
		RunInBackground: false,
		VsyncEnabled:    false,
		Backend:         "con \"comillas\" y \\barra\\ y <html>",
		Display:         d2config.DisplayConfig{},
		WindowScale:     -3,
		D2SDir:          "/rd/Saves",
		Options:         map[string]int{"x": -1, "y": 0},
		KeyBindings:     map[string][]string{"sola": {}},
	}
}

func main() {
	var sb strings.Builder

	sb.WriteString("// GENERADO por console/dc/gen-config-json: NO EDITAR A MANO.\n")
	sb.WriteString("// La configuracion y los bytes esperados los calculo la biblioteca de verdad (encoding/json de Go)\n")
	sb.WriteString("// en el host, sobre una struct espejo sin metodos.\n")
	sb.WriteString("//go:build dreamcast\n\n")
	sb.WriteString("package main\n\n")
	sb.WriteString("import \"github.com/OpenDiablo2/OpenDiablo2/d2core/d2config\"\n\n")

	emitirConfig(&sb, "configDePrueba", configDePrueba())
	emitirEsperado(&sb, "esperadoJSON", configDePrueba())
	emitirConfig(&sb, "configVariante", configVariante())
	emitirEsperado(&sb, "esperadoVarianteJSON", configVariante())

	fmt.Print(sb.String())
}

func emitirEsperado(sb *strings.Builder, nombre string, c *d2config.Configuration) {
	// los bytes del HOST: la struct espejo (sin metodos) serializada por el json de Go por reflexion
	datos, err := json.MarshalIndent(espejoDe(c), "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "el host no pudo serializar la configuracion: %v\n", err)
		os.Exit(1)
	}

	fmt.Fprintf(sb, "// %s: los bytes EXACTOS que escribio la biblioteca de verdad en el host para la\n", nombre)
	sb.WriteString("// configuracion de arriba (struct espejo sin metodos, json.MarshalIndent con 2 espacios).\n")
	fmt.Fprintf(sb, "const %s = %s\n\n", nombre, citar(string(datos)))
}

func emitirConfig(sb *strings.Builder, nombre string, c *d2config.Configuration) {
	// los mismos valores, pero SIN el `path` (que no se serializa) y sin llamar a DefaultConfig() (que
	// depende de runtime.GOOS y de la carpeta del usuario: el host y la consola no dan lo mismo)
	fmt.Fprintf(sb, "func %s() *d2config.Configuration {\n", nombre)
	sb.WriteString("\treturn &d2config.Configuration{\n")
	fmt.Fprintf(sb, "\t\tMpqLoadOrder: %s,\n", citarLista(c.MpqLoadOrder))
	fmt.Fprintf(sb, "\t\tMpqPath: %s,\n", citar(c.MpqPath))
	fmt.Fprintf(sb, "\t\tTicksPerSecond: %d,\n", c.TicksPerSecond)
	fmt.Fprintf(sb, "\t\tFpsCap: %d,\n", c.FpsCap)
	fmt.Fprintf(sb, "\t\tSfxVolume: %s,\n", flotante(c.SfxVolume))
	fmt.Fprintf(sb, "\t\tBgmVolume: %s,\n", flotante(c.BgmVolume))
	fmt.Fprintf(sb, "\t\tFullScreen: %v,\n", c.FullScreen)
	fmt.Fprintf(sb, "\t\tRunInBackground: %v,\n", c.RunInBackground)
	fmt.Fprintf(sb, "\t\tVsyncEnabled: %v,\n", c.VsyncEnabled)
	fmt.Fprintf(sb, "\t\tBackend: %s,\n", citar(c.Backend))
	fmt.Fprintf(sb, "\t\tDisplay: d2config.DisplayConfig{Width: %d, Height: %d},\n", c.Display.Width, c.Display.Height)
	fmt.Fprintf(sb, "\t\tWindowScale: %d,\n", c.WindowScale)
	fmt.Fprintf(sb, "\t\tD2SDir: %s,\n", citar(c.D2SDir))
	fmt.Fprintf(sb, "\t\tLanguage: %s,\n", citar(c.Language))
	fmt.Fprintf(sb, "\t\tOptions: %s,\n", citarMapaInt(c.Options))
	fmt.Fprintf(sb, "\t\tKeyBindings: %s,\n", citarMapaLista(c.KeyBindings))
	sb.WriteString("\t}\n}\n\n")
}

func citar(s string) string { return strconv.Quote(s) }

func flotante(v float64) string { return strconv.FormatFloat(v, 'g', -1, 64) }

func citarLista(xs []string) string {
	if xs == nil {
		return "nil"
	}

	partes := make([]string, 0, len(xs))
	for _, s := range xs {
		partes = append(partes, citar(s))
	}

	return "[]string{" + strings.Join(partes, ", ") + "}"
}

func citarMapaInt(m map[string]int) string {
	if m == nil {
		return "nil"
	}

	claves := make([]string, 0, len(m))
	for k := range m {
		claves = append(claves, k)
	}

	sort.Strings(claves)

	partes := make([]string, 0, len(claves))
	for _, k := range claves {
		partes = append(partes, fmt.Sprintf("%s: %d", citar(k), m[k]))
	}

	return "map[string]int{" + strings.Join(partes, ", ") + "}"
}

func citarMapaLista(m map[string][]string) string {
	if m == nil {
		return "nil"
	}

	claves := make([]string, 0, len(m))
	for k := range m {
		claves = append(claves, k)
	}

	sort.Strings(claves)

	partes := make([]string, 0, len(claves))
	for _, k := range claves {
		partes = append(partes, fmt.Sprintf("%s: %s", citar(k), citarLista(m[k])))
	}

	return "map[string][]string{" + strings.Join(partes, ", ") + "}"
}

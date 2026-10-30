package aica

import "testing"

// TestRutaEnElMPQ: la conversion MEDIDA en la consola (el MPQ guarda `DATA\GLOBAL\SFX\...` y busca por hash
// del nombre exacto). Si esta funcion se rompe, LoadSound deja de encontrar los sonidos y el error que sale
// culpa al archivo.
func TestRutaEnElMPQ(t *testing.T) {
	casos := []struct {
		entra string
		sale  string
	}{
		{"data/global/sfx/combat/impact/sword5.wav", `data\global\sfx\combat\impact\sword5.wav`},
		{`data\local\sfx\act1\akara\aka_act1_intro_sor.wav`, `data\local\sfx\act1\akara\aka_act1_intro_sor.wav`},
		{"/data/global/sfx/cursor/pass.wav", `data\global\sfx\cursor\pass.wav`},
		{"", ""},
	}

	for _, c := range casos {
		if v := rutaEnElMPQ(c.entra); v != c.sale {
			t.Fatalf("rutaEnElMPQ(%q) = %q, queria %q", c.entra, v, c.sale)
		}
	}
}

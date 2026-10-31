package d2util

import (
	"fmt"
	"math/rand"
	"testing"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2interface"
)

// paletaDePrueba: 256 colores con R,G,B y A distintos entre si y NINGUNO con los canales en el valor que
// trae un buffer recien creado (0), para que "no se escribio" se note. El alfa tambien varia: la expansion
// tiene que copiarlo, no suponer 255.
type paletaDePrueba struct {
	colores [256]colorDePrueba
}

type colorDePrueba struct {
	r, g, b, a uint8
}

func (c colorDePrueba) R() uint8 { return c.r }
func (c colorDePrueba) G() uint8 { return c.g }
func (c colorDePrueba) B() uint8 { return c.b }
func (c colorDePrueba) A() uint8 { return c.a }
func (c colorDePrueba) RGBA() uint32 {
	return uint32(c.r)<<24 | uint32(c.g)<<16 | uint32(c.b)<<8 | uint32(c.a)
}
func (c colorDePrueba) SetRGBA(v uint32) { panic("no se usa en la expansion") }
func (c colorDePrueba) BGRA() uint32 {
	return uint32(c.b)<<24 | uint32(c.g)<<16 | uint32(c.r)<<8 | uint32(c.a)
}
func (c colorDePrueba) SetBGRA(v uint32) { panic("no se usa en la expansion") }

func nuevaPaletaDePrueba() *paletaDePrueba {
	p := &paletaDePrueba{}

	for i := range p.colores {
		// valores no nulos y distintos: 1..255 en R, 255-i en G, i/2+1 en B y el alfa tambien variable
		p.colores[i] = colorDePrueba{
			r: uint8(i%255) + 1,
			g: uint8(255 - i),
			b: uint8(i/2) + 1,
			a: uint8(i%7)*3 + 1,
		}
	}

	return p
}

func (p *paletaDePrueba) NumColors() int { return len(p.colores) }
func (p *paletaDePrueba) GetColors() [256]d2interface.Color {
	var out [256]d2interface.Color
	for i := range p.colores {
		out[i] = p.colores[i]
	}
	return out
}
func (p *paletaDePrueba) GetColor(idx int) (d2interface.Color, error) {
	if idx < 0 || idx >= len(p.colores) {
		return nil, fmt.Errorf("indice fuera de la paleta: %d", idx)
	}

	return p.colores[idx], nil
}

// indicesDePrueba: los casos que importan (0 = transparente, 1, 255) y el resto al azar con semilla fija.
func indicesDePrueba(n int) []byte {
	rnd := rand.New(rand.NewSource(85)) // DC-002.85
	out := make([]byte, n)

	for i := range out {
		out[i] = byte(rnd.Intn(256))
	}

	// los casos que importan: 0 (transparente), 1 y 255
	for i, v := range []byte{0, 1, 255, 0} {
		if i < len(out) {
			out[i] = v
		}
	}

	return out
}

// TestImgIndexToRGBAEnEsIgualAlDeSiempre: la version que escribe en un buffer prestado tiene que dar los
// mismos bytes, uno por uno, que la que pide su propio buffer. Es la condicion de "el cuadro sale identico"
// del lado del codigo (del lado de la consola lo prueba el crc32 del cuadro en Flycast).
func TestImgIndexToRGBAEnEsIgualAlDeSiempre(t *testing.T) {
	paleta := nuevaPaletaDePrueba()

	for _, n := range []int{1, 2, 4, 5, 33, 1000, 4096} {
		indices := indicesDePrueba(n)

		esperado := ImgIndexToRGBA(indices, paleta)

		obtenido := make([]byte, len(indices)*4)
		ImgIndexToRGBAEn(obtenido, indices, paleta)

		if len(esperado) != len(obtenido) {
			t.Fatalf("n=%d: largo distinto: %d contra %d", n, len(esperado), len(obtenido))
		}

		for i := range esperado {
			if esperado[i] != obtenido[i] {
				t.Fatalf("n=%d: byte %d distinto: %d contra %d", n, i, esperado[i], obtenido[i])
			}
		}
	}
}

// TestImgIndexToRGBAEnIndiceCeroTransparente: el indice 0 es transparente aunque el buffer de destino venga
// con basura (es el caso de un lienzo que ya se uso) y aunque la paleta tenga color en la posicion 0.
func TestImgIndexToRGBAEnIndiceCeroTransparente(t *testing.T) {
	paleta := nuevaPaletaDePrueba()

	indices := make([]byte, 16) // todos 0
	destino := make([]byte, 16*4)

	for i := range destino {
		destino[i] = 0xAA
	}

	ImgIndexToRGBAEn(destino, indices, paleta)

	for i := range destino {
		if destino[i] != 0 {
			t.Fatalf("el indice 0 no quedo transparente: byte %d = %d", i, destino[i])
		}
	}
}

// TestImgIndexToRGBAEnNoTocaLoQueSobra: el buffer del que llama puede ser mas grande (un lienzo con lugar
// de sobra); fuera de 4 x pixeles no se escribe nada.
func TestImgIndexToRGBAEnNoTocaLoQueSobra(t *testing.T) {
	paleta := nuevaPaletaDePrueba()

	indices := indicesDePrueba(10)
	destino := make([]byte, 10*4+8)

	for i := range destino {
		destino[i] = 0x5A
	}

	ImgIndexToRGBAEn(destino, indices, paleta)

	for i := 10 * 4; i < len(destino); i++ {
		if destino[i] != 0x5A {
			t.Fatalf("se escribio fuera de los pixeles del cuadro: byte %d = %d", i, destino[i])
		}
	}

	esperado := ImgIndexToRGBA(indices, paleta)

	for i := range esperado {
		if destino[i] != esperado[i] {
			t.Fatalf("pixel %d distinto: %d contra %d", i, destino[i], esperado[i])
		}
	}
}

// TestImgIndexToRGBAEnBufferCortoNoEscribe: si los indices no entran en el destino no se escribe medio
// cuadro (comparar medio cuadro con uno entero es comparar cosas distintas).
func TestImgIndexToRGBAEnBufferCortoNoEscribe(t *testing.T) {
	paleta := nuevaPaletaDePrueba()

	indices := indicesDePrueba(16)
	destino := make([]byte, 16*4-1)

	for i := range destino {
		destino[i] = 0x11
	}

	ImgIndexToRGBAEn(destino, indices, paleta)

	for i := range destino {
		if destino[i] != 0x11 {
			t.Fatalf("escribio con el buffer corto: byte %d = %d", i, destino[i])
		}
	}
}

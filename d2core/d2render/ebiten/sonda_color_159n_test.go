package ebiten

import (
	"image/color"
	"testing"
	"unsafe"
)

// TestLaSondaDelColor159nCazaLasTresClases es la prueba HOST del instrumento de DC-002.159n: la sonda tiene
// que distinguir un color sano de los dos defectos que pueden matar el `jsr` del despacho de `color.Color`:
//
//	(2) las palabras de la interfaz ya no son las que se empujaron (alguien escribio encima del objeto vivo)
//	(3) la superficie que llega no es ninguna de las que se empujaron (direccion vieja)
//
// y no tiene que tocar nada cuando no hay gancho instalado (el camino de la PC).
func TestLaSondaDelColor159nCazaLasTresClases(t *testing.T) {
	var partes []ParteDeLaSonda159n

	anterior := SondaDeColor159n
	defer func() { SondaDeColor159n = anterior; reiniciarSondaDeColor159n() }()

	SondaDeColor159n = func(p ParteDeLaSonda159n) bool {
		partes = append(partes, p)

		return !p.Empujado && (!p.Visto || p.Cambiaron)
	}

	reiniciarSondaDeColor159n()

	s := createEbitenSurface(nil, nil)
	blanco := color.Color(color.RGBA{R: 255, G: 255, B: 255, A: 160})

	palabras := PalabrasDeColor159n(&blanco)
	if palabras[0] == 0 {
		t.Fatalf("las palabras del color no tienen tabla de metodos: %v", palabras)
	}

	// (1) empujado y usado sin que nadie lo toque: no hay nada que cortar
	s.PushColor(blanco)

	if saltar := sondaDeColor159n(s, false, &s.stateCurrent.color); saltar {
		t.Fatal("la sonda corto un color sano (el cuadro saldria sin tinte sin motivo)")
	}

	ultima := partes[len(partes)-1]
	if !ultima.Visto || ultima.Cambiaron {
		t.Fatalf("el color empujado no se reconocio: visto=%v cambiaron=%v", ultima.Visto, ultima.Cambiaron)
	}

	if ultima.Palabras != palabras {
		t.Fatalf("la sonda leyo otras palabras: %v != %v", ultima.Palabras, palabras)
	}

	if ultima.Superficie != uintptr(unsafe.Pointer(s)) || ultima.Registro != ultima.Superficie {
		t.Fatalf("la direccion de la superficie no cuadra: sup=%x reg=%x", ultima.Superficie, ultima.Registro)
	}

	// (2) escritura encima del objeto VIVO: las palabras dejan de ser las empujadas
	// (la direccion es la del volcado de DC-002.159m: una palabra del monton en el lugar de la tabla)
	*(*[2]uintptr)(unsafe.Pointer(&s.stateCurrent.color)) = [2]uintptr{0x8c7c4940, 0}

	if saltar := sondaDeColor159n(s, false, &s.stateCurrent.color); !saltar {
		t.Fatal("la sonda no corto un color que ya no es el empujado")
	}

	ultima = partes[len(partes)-1]
	if !ultima.Visto || !ultima.Cambiaron {
		t.Fatalf("la escritura encima del objeto no se vio: visto=%v cambiaron=%v", ultima.Visto, ultima.Cambiaron)
	}

	if ultima.Antes != palabras {
		t.Fatalf("la sonda no guardo lo empujado: %v != %v", ultima.Antes, palabras)
	}

	// (3) una superficie que nunca empujo ese color: no esta en el registro
	otra := createEbitenSurface(nil, nil)
	otra.stateCurrent.color = blanco

	if saltar := sondaDeColor159n(otra, false, &otra.stateCurrent.color); !saltar {
		t.Fatal("la sonda no corto una superficie que no esta en el registro")
	}

	ultima = partes[len(partes)-1]
	if ultima.Visto {
		t.Fatalf("una superficie desconocida aparecio en el registro: sup=%x reg=%x", ultima.Superficie, ultima.Registro)
	}
}

// TestLaSondaDelColor159nNoTocaNadaSinGancho: sin gancho instalado (la PC, la corrida buena) la sonda devuelve
// "no saltar" siempre y ni siquiera apunta el color en el registro.
func TestLaSondaDelColor159nNoTocaNadaSinGancho(t *testing.T) {
	anterior := SondaDeColor159n
	defer func() { SondaDeColor159n = anterior; reiniciarSondaDeColor159n() }()

	SondaDeColor159n = nil
	reiniciarSondaDeColor159n()

	s := createEbitenSurface(nil, nil)
	s.PushColor(color.RGBA{R: 1, G: 2, B: 3, A: 4})

	if sondaDeColor159n(s, false, &s.stateCurrent.color) {
		t.Fatal("sin gancho la sonda no puede cortar el color")
	}

	for i := 0; i < hastaSondear159n; i++ {
		if registroDeColor159n[i].usada {
			t.Fatalf("sin gancho la sonda escribio en el registro (ranura %d)", i)
		}
	}

	// y el camino real (las opciones del dibujo) sigue armando la matriz del color
	s.PushColor(color.RGBA{R: 200, G: 180, B: 120, A: 255})

	sana := createEbitenSurface(nil, nil)
	sana.stateCurrent.color = color.RGBA{R: 200, G: 180, B: 120, A: 255}

	got := s.createDrawImageOptions()
	want := sana.createDrawImageOptions()

	for i := 0; i < 4; i++ {
		for j := 0; j < 5; j++ {
			if got.ColorM.Element(i, j) != want.ColorM.Element(i, j) {
				t.Fatalf("sin gancho la matriz del color cambio en (%d,%d): %v != %v",
					i, j, got.ColorM.Element(i, j), want.ColorM.Element(i, j))
			}
		}
	}
}

package ebiten

// DC-002.159n -- LA SONDA DEL COLOR DEL DIBUJO: las dos palabras de la interfaz, al empujarla y al usarla.
//
// POR QUE EXISTE. La corrida de cierre de DC-002.159m murio una vez de dos en `colorToColorM`
// (`Unhandled exception`, PC dentro de `.rodata`, PR en la instruccion que sigue al `jsr @r1` del despacho de
// `color.Color.RGBA()`), o sea un salto por la tabla de metodos de una interfaz que ya no era la que se
// empujo. Las dos corridas usaron el MISMO ELF y el MISMO recorrido del heroe: el defecto es del estado de la
// superficie (o del recolector), no del camino.
//
// QUE MIDE. Un `color.Color` son dos palabras: la tabla de metodos (`itab`) y el dato. Esta sonda guarda las
// dos cada vez que el estado de una superficie las EMPUJA (`PushColor`) y las vuelve a comparar cada vez que
// el dibujo las USA (`fillDrawImageOptions`, `prepareColorM`). Tres cosas pueden salir mal y las tres quedan
// separadas en el parte:
//
//   - `Visto` en false: la superficie que llega al uso no es ninguna de las que se empujaron. Con el registro
//     siendo una raiz de verdad (un puntero de Go en `.bss`), esto solo puede pasar si el que llama trae una
//     direccion VIEJA: el recolector movio la superficie y esa copia no se actualizo (una raiz que el
//     recolector no ve, o un registro que no puede actualizar).
//   - `Cambiaron` en true: la superficie es la de siempre, pero las palabras de adentro no son las que se
//     empujaron: alguien escribio encima del objeto vivo (copia de mas, cabecera falsa, escritura fuera de
//     rango).
//   - el que la instala, ademas, pregunta si la direccion cae DENTRO del semiespacio vivo (lo unico que el
//     recolector garantiza), que es la misma pregunta desde el otro lado.
//
// El que la instala decide: si devuelve `saltar` = true, el dibujo NO usa el color (sale sin tinte) en vez de
// saltar por una tabla de metodos que ya no es. Asi la corrida sigue y se puede contar cuantas veces pasa.
//
// EN LA PC NO HACE NADA: sin gancho instalado (`SondaDeColor159n == nil`) el camino es el de siempre, y el
// registro ni se toca. La prueba de que la sonda CAZA el defecto esta en `sonda_color_159n_test.go`, que corre
// en el host con superficies de verdad.
//
// OJO CON EL REGISTRO: es un arreglo FIJO a proposito. Un `append` aca adentro pediria memoria, y una
// recoleccion en medio de la comparacion devolveria direcciones nuevas justo cuando se las esta midiendo
// (el instrumento cambiaria lo que mide). Con 64 ranuras alcanza: el Gate 0 crea 16 superficies.

import (
	"image/color"
	"unsafe"
)

const hastaSondear159n = 64

// marcaDeSuperficie159n: la marca de vida que llevan las superficies (ver `marca159n` en `ebitenSurface`). El
// valor es arbitrario; lo unico que importa es que no sea 0 (cero es "memoria que no toco nadie") y que no se
// repita en ningun otro objeto del monton.
const marcaDeSuperficie159n uint32 = 0x159C0DE5

// ParteDeLaSonda159n es lo que la sonda le entrega al que la instala, en cada empujada y en cada uso.
type ParteDeLaSonda159n struct {
	Empujado   bool       // true = se acaba de guardar el color; false = el dibujo lo va a usar
	Superficie uintptr    // la direccion de la superficie, cruda (para el log; el recolector no la puede mover)
	Marca      bool       // true = lo que hay en esa direccion sigue siendo una superficie (la marca esta puesta)
	Palabras   [2]uintptr // las dos palabras de la interfaz, ahora
	Antes      [2]uintptr // las dos palabras que se empujaron para esa superficie
	Visto      bool       // false = esa superficie no esta en el registro de las empujadas
	Cambiaron  bool       // true = las palabras de ahora no son las empujadas
	Registro   uintptr    // la direccion que el registro tiene HOY para esa superficie (0 si no esta)
}

// SondaDeColor159n: gancho de la sonda (nil = apagado, el camino de la PC y de la corrida buena). Devuelve
// `saltar` = true para que el que llama NO use este color.
var SondaDeColor159n func(p ParteDeLaSonda159n) (saltar bool)

type entradaDeColor159n struct {
	sup      *ebitenSurface // raiz de verdad: el recolector la tiene que mover cuando copia la superficie
	palabras [2]uintptr
	usada    bool
}

//nolint:gochecknoglobals // registro del instrumento: arreglo fijo, sin pedir memoria, sin crecer
var registroDeColor159n [hastaSondear159n]entradaDeColor159n

// PalabrasDeColor159n devuelve las dos palabras de una interfaz `color.Color` (itab, dato). Es la MISMA
// lectura que hace el compilador para saltar por la tabla de metodos.
func PalabrasDeColor159n(clr *color.Color) [2]uintptr {
	if clr == nil {
		return [2]uintptr{}
	}

	return *(*[2]uintptr)(unsafe.Pointer(clr))
}

// Contadores del instrumento. Se mueven siempre (haya gancho o no): son la prueba de que el camino se ejercio.
//
//nolint:gochecknoglobals // contadores del instrumento
var (
	sondaLLamadas159n uint64
	sondaConColor159n uint64
	sondaSinColor159n uint64
	// sondaMarcaRota159n: cuantas veces la direccion de superficie que llego al dibujo YA NO era una superficie.
	// Es la medida que no necesita que haya color: si el recolector movio la superficie y alguien quedo con la
	// direccion vieja, esto lo cuenta.
	sondaMarcaRota159n uint64
)

// EstadoDeLaSonda159n devuelve si hay gancho puesto y cuantas veces se ejercio la sonda: llamadas, con color,
// sin color y con la marca de superficie rota. Es lo que el que la instala imprime en su latido para no
// confundir "no paso nada" con "la sonda no corrio".
func EstadoDeLaSonda159n() (puesto bool, llamadas, conColor, sinColor, marcaRota uint64) {
	return SondaDeColor159n != nil, sondaLLamadas159n, sondaConColor159n, sondaSinColor159n, sondaMarcaRota159n
}

// reiniciarSondaDeColor159n vacia el registro (solo para las pruebas del host: en la consola no se llama).
func reiniciarSondaDeColor159n() {
	for i := 0; i < hastaSondear159n; i++ {
		registroDeColor159n[i] = entradaDeColor159n{}
	}
}

// sondaDeColor159n apunta o compara el color del estado de una superficie. Devuelve true si el que llama no
// tiene que usar el color.
func sondaDeColor159n(s *ebitenSurface, empujado bool, clr *color.Color) bool {
	if SondaDeColor159n == nil || s == nil {
		return false
	}

	// DC-002.159n: los contadores son del INSTRUMENTO y se mueven solo con gancho puesto (o sea, en la
	// corrida de medicion). Sin gancho, este camino no toca nada: es el de la PC y el del juego normal.
	sondaLLamadas159n++

	if clr == nil || *clr == nil {
		sondaSinColor159n++
	} else {
		sondaConColor159n++
	}

	if s.marca159n != marcaDeSuperficie159n {
		sondaMarcaRota159n++
	}

	palabras := PalabrasDeColor159n(clr)

	parte := ParteDeLaSonda159n{
		Empujado:   empujado,
		Superficie: uintptr(unsafe.Pointer(s)),
		Marca:      s.marca159n == marcaDeSuperficie159n,
		Palabras:   palabras,
	}

	ranura := -1

	for i := 0; i < hastaSondear159n; i++ {
		if registroDeColor159n[i].usada && registroDeColor159n[i].sup == s {
			ranura = i

			break
		}
	}

	if ranura >= 0 {
		parte.Visto = true
		parte.Antes = registroDeColor159n[ranura].palabras
		parte.Cambiaron = parte.Antes != palabras
		parte.Registro = uintptr(unsafe.Pointer(registroDeColor159n[ranura].sup))
	} else if empujado && clr != nil && *clr != nil {
		for i := 0; i < hastaSondear159n; i++ {
			if !registroDeColor159n[i].usada {
				registroDeColor159n[i] = entradaDeColor159n{sup: s, palabras: palabras, usada: true}
				parte.Registro = uintptr(unsafe.Pointer(s))

				break
			}
		}
	}

	if empujado {
		// una empujada no puede "saltar" nada: es la que define lo que vale
		SondaDeColor159n(parte)

		return false
	}

	if clr == nil || *clr == nil {
		return false // sin color no hay nada que despachar (el camino de siempre)
	}

	return SondaDeColor159n(parte)
}

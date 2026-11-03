//go:build dreamcast

package pvr

import "unsafe"

// DC-002.142 -- LOS BLOQUES SUCIOS SUBIDOS EN GRUPOS DE 2x2 (la presentacion del cuadro que camina).
//
// EL NUMERO QUE MANDA (medido en DC-002.141/142, Flycast). Un cuadro parado presenta en 1,5-2,2 ms con ~190
// bloques sucios; uno que camina, en 16-26 ms. Y no es que camine mas: caminando se marca el lienzo ENTERO (300
// bloques) y el camino por bloques hacia UNA copia a la VRAM por bloque (`d2c_pvr_pantalla_bloque`), o sea 300
// viajes de Store Queue con su espera cada uno. El camino del cuadro entero ya sabia agrupar: los CUATRO bloques
// de un grupo de 2x2 (64x64 px = 8.192 B) son contiguos en la textura twiddleada, asi que van en una sola copia
// (90 viajes por cuadro, medido en DC-002.135: 1,8 ms de subida para 614 KB).
//
// LO QUE HACE ESTA PALANCA. La presentacion por bloques sucios pasa a recorrer la grilla de a grupos de 2x2: el
// grupo con los cuatro bloques sucios se convierte y se sube en UNA copia; los grupos a medias y los sueltos,
// como siempre. Los pixeles que llegan a la VRAM son los MISMOS (misma cuenta de color, mismo twiddle, mismo
// `pvr_txr_load`): lo unico que cambia es cuantos viajes cuesta.
//
// `PRES_GRUPO_142=no` vuelve al bucle de a un bloque de DC-002.139, en el MISMO binario: eso es el A/B.

//extern d2c_pvr_pantalla_bloques142
func d2cPvrPantallaBloques142(rgba uintptr, lienzoW int, sucios uintptr, columnas, filas int) int

var (
	// presentarEnGrupos142: la palanca.
	presentarEnGrupos142 = true

	// mapa142: los bloques sucios como bytes (1 = sucio), que es lo que el C entiende. Se arma por cuadro desde
	// `sucios139`: son 1.024 bytes en `.bss`, no un pedido al monton.
	mapa142 [bloquesMax139]byte

	// Lo que costo el ULTIMO cuadro presentado, por dentro: el informe `PVR135` es un promedio de 30 cuadros y
	// el cuadro que camina se pierde adentro del promedio (30 cuadros son 1,5 s de caminar y de estar quieto).
	// Estos cuatro numeros son del cuadro anterior, que es lo que el POC lee al empezar el siguiente.
	convDelCuadro142      int
	vramDelCuadro142      int
	usPintarDelCuadro142  int
	presentarDelCuadro142 int
)

// LoQueCostoElCuadro142: la presentacion del ULTIMO cuadro presentado, partida en sus tres pedazos (conversion,
// subida a la VRAM y pintado del quad) y el total medido por el ciclo. Es por cuadro, no promedio.
func LoQueCostoElCuadro142() (conv, vram, pintar, total int) {
	return convDelCuadro142, vramDelCuadro142, usPintarDelCuadro142, presentarDelCuadro142
}

// FijarPresentacionEnGrupos142 prende (o apaga) la subida agrupada del camino por bloques sucios.
func FijarPresentacionEnGrupos142(v bool) { presentarEnGrupos142 = v }

// PresentacionEnGrupos142: con que palanca corre el binario (para la linea del desglose).
func PresentacionEnGrupos142() bool { return presentarEnGrupos142 }

// presentarLosBloquesSucios142: el camino por bloques sucios con la subida agrupada. Devuelve 0 si anduvo, el
// codigo de error del backend si no (y el que llama decide: en ese caso no se presenta nada).
func presentarLosBloquesSucios142(base uintptr, ancho int) int {
	total := columnas139 * filas139
	convertidos := 0

	for i := range mapa142 {
		mapa142[i] = 0
	}

	for by := 0; by < filas139; by++ {
		for bx := 0; bx < columnas139; bx++ {
			i := by*columnas139 + bx

			if i < 0 || i >= len(sucios139) || !sucios139[i] {
				continue
			}

			mapa142[i] = 1
			convertidos++
		}
	}

	if r := d2cPvrPantallaBloques142(base, ancho, uintptr(unsafe.Pointer(&mapa142[0])), columnas139, filas139); r != 0 {
		return r
	}

	// el censo (lo lee `CensoDeLaPresentacionPorBloques`): el camino agrupado convierte los mismos bloques que el
	// bucle de a uno, asi que el numero que se informa no cambia de significado.
	bloquesConvertidos139 += convertidos
	bloquesSaltados139 += total - convertidos

	return 0
}

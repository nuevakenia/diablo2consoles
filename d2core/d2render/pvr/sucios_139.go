//go:build dreamcast

package pvr

// DC-002.139 -- LOS BLOQUES SUCIOS DE LA PRESENTACION.
//
// QUE ES. El cuadro del motor se compone en el lienzo (RGBA, en la VRAM) y despues se convierte a RGB565
// bloque por bloque de 32x32 para subirlo a la textura de la pantalla. Convertir el cuadro entero cuesta
// 23,6 ms medidos (DC-002.140): es la lectura del lienzo contra la VRAM, no la cuenta del color. Cuando el
// mapa es estatico, la mayor parte del lienzo NO cambio entre dos cuadros, asi que convertirla otra vez es
// trabajo tirado.
//
// COMO. El que dibuja sabe que toco: el POC marca los rectangulos (las cajas de los sprites, el HUD, el
// minimapa, el panel) y el presentador convierte SOLO los bloques de 32x32 que caen adentro. Si no se marca
// nada (o la palanca esta apagada) el camino es el de siempre: el cuadro entero.
//
// LA REGLA DE ORO: los bloques son la unica unidad. Un rectangulo que se marca se redondea HACIA AFUERA a la
// grilla de 32x32 (marcar de menos dejaria un pedazo de cuadro viejo en pantalla). El bloque es lo que la
// conversion y el twiddle necesitan: cada 32x32 de la textura es contiguo (`indice_tile`).

const (
	// ladoDelBloque139: el lado del bloque de la conversion (el mismo de `pvr_pantalla.c`).
	ladoDelBloque139 = 32
	// bloquesMax139: la grilla mas grande que se soporta (1024x1024 de lienzo = 32x32 bloques).
	bloquesMax139 = 32 * 32
)

var (
	// sucios139: los bloques que hay que convertir en ESTE cuadro.
	sucios139 [bloquesMax139]bool
	// columnas139/filas139: la grilla del lienzo actual.
	columnas139, filas139 int
	// suciosListos139: cuantos bloques hay marcados (0 = convertir todo).
	suciosMarcados139 int
	// converPorBloques139: la palanca (la prende el POC antes del primer cuadro).
	converPorBloques139 bool
	// bloquesConvertidos139/acumulados: el censo, para el informe.
	bloquesConvertidos139 int
	bloquesSaltados139    int
)

// FijarConversionPorBloques: prende la presentacion por bloques sucios.
func FijarConversionPorBloques(si bool) { converPorBloques139 = si }

// ConversionPorBloques: si esta prendida.
func ConversionPorBloques() bool { return converPorBloques139 }

// FijarTamanoDeLaGrilla: la grilla de bloques del lienzo (ancho/alto en pixeles). Se llama una vez, cuando se
// sabe el tamano de la pantalla.
func FijarTamanoDeLaGrilla(ancho, alto int) {
	columnas139 = ancho / ladoDelBloque139
	filas139 = alto / ladoDelBloque139
	suciosMarcados139 = 0

	for i := range sucios139 {
		sucios139[i] = false
	}
}

// ReiniciarBloquesSucios: el arranque de un cuadro. Nadie marco nada todavia.
func ReiniciarBloquesSucios() {
	suciosMarcados139 = 0

	for i := 0; i < len(sucios139); i++ {
		sucios139[i] = false
	}
}

// MarcarSucio: marca el rectangulo (redondeado hacia afuera a la grilla de 32x32) para que se convierta.
func MarcarSucio(x0, y0, x1, y1 int) {
	if !converPorBloques139 || columnas139 == 0 {
		return
	}

	if x0 < 0 {
		x0 = 0
	}
	if y0 < 0 {
		y0 = 0
	}
	if x1 > columnas139*ladoDelBloque139 {
		x1 = columnas139 * ladoDelBloque139
	}
	if y1 > filas139*ladoDelBloque139 {
		y1 = filas139 * ladoDelBloque139
	}
	if x0 >= x1 || y0 >= y1 {
		return
	}

	bx0, by0 := x0/ladoDelBloque139, y0/ladoDelBloque139
	bx1, by1 := (x1+ladoDelBloque139-1)/ladoDelBloque139, (y1+ladoDelBloque139-1)/ladoDelBloque139

	if bx1 > columnas139 {
		bx1 = columnas139
	}
	if by1 > filas139 {
		by1 = filas139
	}

	for by := by0; by < by1; by++ {
		for bx := bx0; bx < bx1; bx++ {
			i := by*columnas139 + bx
			if i < 0 || i >= len(sucios139) || sucios139[i] {
				continue
			}

			sucios139[i] = true
			suciosMarcados139++
		}
	}
}

// BloquesMarcados: cuantos bloques hay marcados en este cuadro (0 = no se marco nada).
func BloquesMarcados() int { return suciosMarcados139 }

// CensoDeLaPresentacionPorBloques: los bloques convertidos y saltados del tramo, para el informe.
func CensoDeLaPresentacionPorBloques() (int, int) {
	conv, salt := bloquesConvertidos139, bloquesSaltados139
	bloquesConvertidos139, bloquesSaltados139 = 0, 0

	return conv, salt
}

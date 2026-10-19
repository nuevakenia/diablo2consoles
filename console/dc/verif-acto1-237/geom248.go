// geom248.go: EL INSTRUMENTO DE DC-002.48 -- el que SI ve los "tiles corridos".
//
// POR QUE HACE FALTA (la leccion de DC-002.37): el verificador de la 37 comparaba el CRC del lienzo de la
// CONSOLA contra el CRC del lienzo del HOST, pero las dos puntas componian con el MISMO archivo
// (`nivel.go`). Un compositor que se compara consigo mismo da 100% siempre: puede tener el paso de la celda
// mal y las dos puntas van a coincidir. Eso fue exactamente lo que paso (32x16 en vez de 80x40).
//
// LO QUE MIDE ESTE ARCHIVO, y de donde sale cada numero:
//
//  1. EL PASO DEL MOTOR, medido LLAMANDO al motor (no copiando una constante): `d2maprenderer.NewViewport` +
//     `WorldToOrtho`, que es la funcion con la que el renderer ubica cada celda. La diferencia entre la celda
//     (1,0) y la (0,0) es el paso real en pixeles.
//  2. LAS CELDAS VISIBLES DEL MOTOR para una ventana de WxH: `d2maprenderer.TileRange`, la misma funcion con
//     la que el motor decide que celdas pintar. De ahi sale el area de celda que el motor usa.
//  3. EL PASO QUE USA EL COMPOSITOR: las constantes `ladoCeldaX`/`ladoCeldaY` de `nivel.go`, que son las que
//     de verdad mueven los tiles en el lienzo (no una copia: son las mismas que usa `unionIso`).
//  4. LA SOBRE-ESCRITURA DEL LIENZO: cuantos tiles pintan cada pixel (el censo de `nivel.go`). Con el paso
//     correcto el piso encaja borde con borde y cada pixel lo pinta UN tile; con el paso chico, cinco.
//  5. LA DENSIDAD: cuantos pisos caen dentro de la ventana contra las celdas que el motor considera visibles
//     para esa misma ventana. Es el numero que dice "el nivel esta a otra escala".
//
// Los cinco son numeros, no opiniones, y el mutante (volver a 32x16) los devuelve a los valores del defecto.
package main

import (
	"fmt"

	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2map/d2maprenderer"
)

// pasoDelMotor: el paso de una celda en pixeles, medido con el viewport del MOTOR.
func pasoDelMotor(ancho, alto int) (dx, dy float64) {
	v := d2maprenderer.NewViewport(0, 0, ancho, alto)

	x0, y0 := v.WorldToOrtho(0, 0)
	x1, y1 := v.WorldToOrtho(1, 0)

	return x1 - x0, y1 - y0
}

// celdasVisiblesDelMotor: cuantas celdas considera visibles el motor en una ventana de ancho x alto para un
// mapa de mapaW x mapaH celdas (la misma pregunta que se hace el renderer en cada cuadro).
func celdasVisiblesDelMotor(ancho, alto, mapaW, mapaH int) (celdas int, sx, sy, ex, ey int) {
	v := d2maprenderer.NewViewport(0, 0, ancho, alto)
	sx, sy, ex, ey = d2maprenderer.TileRange(v, ancho, alto, mapaW, mapaH)
	celdas = (ex - sx) * (ey - sy)

	return celdas, sx, sy, ex, ey
}

// informeGeometria imprime los cinco numeros y devuelve el veredicto (true = encaja con el motor).
func informeGeometria(n *Nivel, mapaW, mapaH int) bool {
	dxMotor, dyMotor := pasoDelMotor(n.Ancho, n.Alto)
	celdas, sx, sy, ex, ey := celdasVisiblesDelMotor(n.Ancho, n.Alto, mapaW, mapaH)

	fmt.Printf("GEOM248: MOTOR pasoCelda=%.0fx%.0f px (WorldToOrtho(1,0)-WorldToOrtho(0,0), d2maprenderer)\n",
		dxMotor, dyMotor)
	fmt.Printf("GEOM248: MOTOR ventana=%dx%d celdasVisibles=%d rango=(%d,%d)-(%d,%d) areaPorCelda=%.1f px2\n",
		n.Ancho, n.Alto, celdas, sx, sy, ex, ey, float64(n.Ancho*n.Alto)/float64(maxInt(celdas, 1)))

	fmt.Printf("GEOM248: COMPOSITOR pasoCelda=%dx%d px (ladoCeldaX/Y de nivel.go)\n", ladoCeldaX, ladoCeldaY)

	hist := n.CoberturaHistograma()

	fmt.Printf("GEOM248: LIENZO cobertura max=%d pixelesSobre=%d de %d (%.2f%%)\n",
		n.MaxCobertura, n.PixelesSobre, n.Ancho*n.Alto,
		float64(n.PixelesSobre)*100/float64(n.Ancho*n.Alto))

	for k := uint16(1); k <= n.MaxCobertura; k++ {
		fmt.Printf("GEOM248: LIENZO cobertura %d tiles/pixel = %d pixeles\n", k, hist[k])
	}

	if n.PixelesSobre > 0 {
		fmt.Printf("GEOM248: CAJA pisados=(%d,%d)-(%d,%d) = %dx%d px\n",
			n.CajaSobreX0, n.CajaSobreY0, n.CajaSobreX1, n.CajaSobreY1,
			n.CajaSobreX1-n.CajaSobreX0+1, n.CajaSobreY1-n.CajaSobreY0+1)
	}

	fmt.Printf("GEOM248: DENSIDAD pisosEnVentana=%d paredesEnVentana=%d celdasDelMotor=%d pisosPorCelda=%.3f\n",
		n.PisosEnVentana, n.ParedesEnVentana, celdas, float64(n.PisosEnVentana)/float64(maxInt(celdas, 1)))

	encaja := dxMotor == float64(ladoCeldaX) && dyMotor == float64(ladoCeldaY)

	if encaja {
		fmt.Println("GEOM248: VEREDICTO ENCAJA (el compositor usa el paso del motor)")
	} else {
		fmt.Printf("GEOM248: VEREDICTO TILES-CORRIDOS (el compositor usa %dx%d y el motor %.0fx%.0f: escala %.2fx)\n",
			ladoCeldaX, ladoCeldaY, dxMotor, dyMotor, dxMotor/float64(ladoCeldaX))
	}

	return encaja
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}

	return b
}

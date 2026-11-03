//go:build dreamcast

package main

import "github.com/OpenDiablo2/OpenDiablo2/d2core/d2render/pvr"

// DC-002.142 -- LA PRESENTACION DEL CUADRO QUE CAMINA, LEIDA CUADRO POR CUADRO.
//
// POR QUE HACE FALTA. La linea `PVR135: PRESENTAR` es un PROMEDIO de 30 cuadros, y una ventana de la demo mezcla
// cuadros que caminan con cuadros quietos: el cuadro caro (el que camina) se diluye entre 20 baratos y el numero
// que sale no dice cuanto cuesta el que importa. Medido en DC-002.141: la presentacion del cuadro caminando son
// 16-26 ms, y el promedio de la ventana decia 2-20 segun cuantos cuadros de cada clase le tocaran.
//
// COMO. Los contadores de la presentacion los reinicia el ciclo del cuadro JUSTO ANTES de presentar, o sea
// DESPUES de que el POC dibuja: al empezar a dibujar el cuadro N, los contadores siguen teniendo lo del cuadro
// N-1. Asi que aca se imprime lo del cuadro ANTERIOR cuando ese cuadro camino, con el desglose por dentro
// (convertir / subir / pintar) y el corrimiento que lo causo. Es la linea que dice, por cuadro, si la palanca
// hizo lo que dice.
const lineasDelInforme142 = 40

// bloquesDeLaPrueba142: cuantos bloques convierte cada pasada de la prueba (`PRUEBA_CONV_142=si`). 20 son el 7 %
// del cuadro y ~4 ms medidos: alcanza para ver la diferencia sin tapar el cuadro que se mide.
const bloquesDeLaPrueba142 = 20

var (
	// caminoElCuadroAnterior142: lo prende `dibujarElCuadroQueCamina141` cuando el cuadro se resolvio con la copia.
	caminoElCuadroAnterior142 bool

	// sxDelCuadroAnterior142/syDelCuadroAnterior142: el corrimiento de ese cuadro.
	sxDelCuadroAnterior142, syDelCuadroAnterior142 int

	// lineasDelInformeHechas142: el informe es para leer un tramo, no para inundar el puerto serie.
	lineasDelInformeHechas142 int

	// Los acumulados del tramo: el total y los bloques de los cuadros caminando, que es el numero que se compara
	// contra los 26 ms de DC-002.141.
	cuadrosCaminando142, usPresentar142, pxBloques142, subidas142 int
	conv142, vram142, pintar142                                    int
)

// informePresentar142: la presentacion del cuadro ANTERIOR si ese cuadro camino.
func informePresentar142() {
	if presentar142 != "si" {
		return
	}

	// con la prueba puesta se informan TODOS los cuadros (los quietos son el control); sin ella, solo los que
	// caminan, que es el cuadro que la tarjeta mide.
	if pruebaConv142 != "si" && !caminoElCuadroAnterior142 {
		return
	}

	conv, vram, pintar, total := pvr.LoQueCostoElCuadro142()

	cuadrosCaminando142++
	usPresentar142 += total
	pxBloques142 += pvr.BloquesPresentados()
	subidas142 += pvr.SubidasPresentadas()
	conv142 += conv
	vram142 += vram
	pintar142 += pintar

	if lineasDelInformeHechas142 < lineasDelInforme142 {
		lineasDelInformeHechas142++

		println("JUEGO142: PRESENTAR-CAMINANDO cuadro=", cuadro-1, "camino=", caminoElCuadroAnterior142,
			"sx=", sxDelCuadroAnterior142, "sy=", syDelCuadroAnterior142,
			"conv_us=", conv, "vram_us=", vram, "pintar_us=", pintar, "total_us=", total,
			"bloques=", pvr.BloquesPresentados(), "subidas=", pvr.SubidasPresentadas(),
			"grupo=", grupoDelBinario142())
	}
}

// promedioPresentar142: el promedio del tramo de los cuadros caminando, en la linea del censo.
func promedioPresentar142() string {
	if cuadrosCaminando142 == 0 {
		return "sin-cuadros-caminando"
	}

	n := cuadrosCaminando142

	return "cuadros=" + itoa(n) +
		" total_us=" + itoa(usPresentar142/n) +
		" conv_us=" + itoa(conv142/n) +
		" vram_us=" + itoa(vram142/n) +
		" pintar_us=" + itoa(pintar142/n) +
		" bloques=" + itoa(pxBloques142/n) +
		" subidas=" + itoa(subidas142/n)
}

// reiniciarPresentar142: el tramo a cero (el censo llama a esto cada 30 cuadros).
func reiniciarPresentar142() {
	cuadrosCaminando142, usPresentar142, pxBloques142, subidas142 = 0, 0, 0, 0
	conv142, vram142, pintar142 = 0, 0, 0
}

// grupoDelBinario142: con que palanca de subida corrio este binario.
func grupoDelBinario142() string {
	if pvr.PresentacionEnGrupos142() {
		return "si"
	}

	return "no"
}

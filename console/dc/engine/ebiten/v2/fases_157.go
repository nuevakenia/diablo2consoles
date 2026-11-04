package ebiten

// DC-002.157 -- LAS FASES DEL CUADRO, Y LAS PALANCAS DE LA PRESENTACION.
//
// QUE MIDE. El unico numero que habia de un cuadro del motor en la consola era el total (`CUADRO n ms=`), y
// de ahi salio el diagnostico de DC-002.156: 1.869 ms. Pero un total no dice QUE atacar. Este archivo parte
// el cuadro en las seis cosas que lo componen, con el reloj del PVR (el mismo que usa el shim para el
// perfil de EBITEN-PERFIL):
//
//	espera      -- `PvrWaitReady` (lo que se espera al cuadro anterior del PVR: el vsync)
//	actualizar  -- `Update` de la aplicacion (el motor: Advance del mapa, la pantalla, el input)
//	dibujar     -- `Draw` de la aplicacion (el mapa del motor, la UI, el GUI)
//	convertir   -- RGBA -> RGB565 de los pixeles del cuadro (la CPU recorriendo el lienzo)
//	subir       -- el bloque convertido a la VRAM (Store Queues)
//	pintar      -- el quad del PVR (el hardware)
//
// y los promedia en ventanas de `cuadrosPorInforme157` cuadros, que es lo que se lee en la evidencia:
//
//	EBITEN157: FASES cuadros= 10 espera_us= 0 actualizar_us= 312 dibujar_us= 1892 ... total_us= 2204
//
// POR QUE PROMEDIO Y NO MEDIANA. Las ventanas son de cuadros consecutivos con el MISMO plan de palancas, y
// lo que interesa es "cuanto cuesta un cuadro en este tramo", no el mejor cuadro. La mediana se mide aparte
// con los totales de la pantalla, que ya los imprime ella.
//
// LAS TRES PALANCAS DE LA PRESENTACION (las prende el que dibuja, cuadro a cuadro; ver `pantalla_mapa_155.go`):
//
//	conversionRapida157   -- el bucle de conversion nuevo de DC-002.135 (una carga de 32 bits por pixel, dos
//	                         pixeles por deposito, subida en grupos de 2x2 bloques). Apagada, el bucle de
//	                         DC-002.132 (tres cargas de 8 bits por pixel y una subida por bloque).
//	conversionPorZona157  -- convertir SOLO el recuadro visible del lienzo (640x480 de los 800x600): el PVR
//	                         pinta la textura en una pantalla de 640x480, asi que los 160x600 de la derecha y
//	                         los 120 de abajo no se ven y convertirlos es tirar 1/3 del trabajo.
//	presentacionQuieta157 -- si el cuadro no cambio (lo dice el que dibuja con `MarcarCuadroQuieto157`), no se
//	                         convierte ni se sube nada: la textura del PVR ya tiene ese mismo cuadro del
//	                         cuadro anterior. Es la palanca de DC-002.143 ("presentar por bloques sucios")
//	                         llevada al caso extremo: cero bloques sucios.
//
// LAS TRES SON ADITIVAS Y SE PUEDEN PRENDER Y APAGAR EN CALIENTE: el binario sin ninguna es el de DC-002.156,
// y por eso el A/B de la tarjeta se puede hacer EN UNA SOLA CORRIDA (tramos de cuadros con y sin palanca).

// Las palancas de la presentacion.
var (
	conversionRapida157   bool
	conversionPorFilas157 bool
	conversionPorZona157  bool
	presentacionQuieta157 bool

	// cuadroQuieto157: lo pone el que dibuja cuando NO escribio nada en el lienzo de este cuadro.
	cuadroQuieto157 bool
)

// ActivarConversionRapida157: el bucle de conversion de DC-002.135 (y la subida agrupada).
func ActivarConversionRapida157(si bool) {
	conversionRapida157 = si

	n := 0
	if si {
		n = 1
	}

	d2cPvrPantallaPresConvRapida(n)
	d2cPvrPantallaPresVramAgrupada(n)
}

// ConversionRapida157: si esta prendida (para el informe del arranque).
func ConversionRapida157() bool { return conversionRapida157 }

// ActivarConversionPorFilas157: el orden de lectura del bucle rapido (DC-002.141).
func ActivarConversionPorFilas157(si bool) {
	conversionPorFilas157 = si

	n := 0
	if si {
		n = 1
	}

	d2cPvrPantallaConvFilas141(n)
}

// ConversionPorFilas157: si esta prendida.
func ConversionPorFilas157() bool { return conversionPorFilas157 }

// ActivarConversionPorZona157: convertir solo el recuadro visible (640x480 de la esquina superior izquierda).
func ActivarConversionPorZona157(si bool) { conversionPorZona157 = si }

// ConversionPorZona157: si esta prendida.
func ConversionPorZona157() bool { return conversionPorZona157 }

// ActivarPresentacionQuieta157: si el cuadro no cambio, no convertir ni subir nada.
func ActivarPresentacionQuieta157(si bool) { presentacionQuieta157 = si }

// PresentacionQuieta157: si esta prendida.
func PresentacionQuieta157() bool { return presentacionQuieta157 }

// MarcarCuadroQuieto157: el cuadro NO cambio respecto del anterior (el que dibuja no escribio un pixel).
// La presentacion lo mira en el mismo cuadro, asi que el que llama tiene que hacerlo desde su `Draw`.
func MarcarCuadroQuieto157() { cuadroQuieto157 = true }

// CuadroQuieto157: si el cuadro de ahora se marco quieto (lo consume la presentacion).
func CuadroQuieto157() bool { return cuadroQuieto157 }

// OlvidarCuadroQuieto157: lo llama la presentacion cuando ya uso la marca.
func OlvidarCuadroQuieto157() { cuadroQuieto157 = false }

// ============================== el contador de escrituras del lienzo ==============================
//
// POR QUE EXISTE, Y POR QUE ASI. "El cuadro no cambio" no se puede adivinar: o lo dice el que dibuja (y se
// olvida de decirlo el dia que agregue un sprite) o se cuenta. Aca se cuenta: los metodos que escriben
// pixeles de una imagen suben este contador cuando la imagen es LA PANTALLA (la comparacion es de punteros,
// un ciclo por llamada de dibujado, no por pixel). La presentacion compara el contador antes y despues del
// `Draw`: si no se movio, nadie escribio un pixel y la textura del PVR ya tiene este mismo cuadro.
//
// ES MAS BARATO QUE MARCAR BLOQUES SUCIOS (DC-002.143) Y ALCANZA PARA ESTE CUADRO: el Gate 0 no tiene UI ni
// GUI ni sprites, asi que un cuadro o cambia entero (el mapa se repinta) o no cambia nada. Cuando haya
// sprites encima del mapa, lo que hay que hacer es lo de DC-002.143: convertir solo los bloques que
// cambiaron. El contador no estorba para eso; lo complementa.
var escrituras157 int

// EscriturasDeLaPantalla157: cuantas veces se escribio en el lienzo de la pantalla desde que arranco.
func EscriturasDeLaPantalla157() int { return escrituras157 }

// contarEscritura157: lo llaman los metodos que escriben pixeles, con la imagen que escriben.
func contarEscritura157(i *Image) {
	if i != nil && i == pantalla {
		escrituras157++
	}
}

// ============================== el instrumento de las fases ==============================

// Los indices de las fases, en el orden en que se pagan.
const (
	faseEspera157 = iota
	faseActualizar157
	faseDibujar157
	faseConvertir157
	faseSubir157
	fasePintar157
	faseTotal157
	fases157Total
)

// cuadrosPorInforme157: cada cuantos cuadros se imprime la ventana. Con 10 el puerto serie paga una linea
// cada 10 cuadros (~80 caracteres, medido en DC-002.143: ~13 ms por cuadro si se imprime SIEMPRE; cada 10
// cuadros es 1,3 ms de promedio y no ensucia la medida).
const cuadrosPorInforme157 = 10

var (
	fases157Acum [fases157Total]uint64
	cuadros157   int

	// palancas157Ventana: las palancas con las que se abrio la ventana abierta. Si en un cuadro son otras, la
	// ventana se cierra ahi mismo: asi cada tramo del plan de la pantalla se mide solo, sin promedios que
	// cruzan un cambio de palanca (que fue lo que paso en la primera corrida de esta tarjeta: la ventana del
	// mapa estatico arrancaba a mitad de la del cuadro caro).
	palancas157Ventana [4]bool
	abierta157         bool
)

// anotarFase157: suma un tramo al acumulador de la ventana.
func anotarFase157(fase int, us uint64) { fases157Acum[fase] += us }

// palancasDeAhora157: las cuatro palancas de la presentacion, en un valor comparable.
func palancasDeAhora157() [4]bool {
	return [4]bool{conversionRapida157, conversionPorFilas157, conversionPorZona157, presentacionQuieta157}
}

// cerrarCuadro157: cierra la ventana cada `cuadrosPorInforme157` cuadros y la imprime.
func cerrarCuadro157() {
	cuadros157++
	abierta157 = true

	if cuadros157%cuadrosPorInforme157 == 0 {
		imprimirLaVentana157()
	}
}

// comprobarCambioDePalancas157: si las palancas de este cuadro no son las de la ventana abierta, se cierra la
// ventana (asi cada tramo del plan se mide solo). Va al principio del cuadro, antes de dibujar.
func comprobarCambioDePalancas157() {
	ahora := palancasDeAhora157()

	if abierta157 && ahora != palancas157Ventana {
		imprimirLaVentana157()
	}
}

// imprimirLaVentana157: el promedio de la ventana abierta, con las palancas que corrian.
func imprimirLaVentana157() {
	n := uint64(cuadros157)
	if n == 0 {
		return
	}

	println("EBITEN157: FASES cuadros=", int(n),
		"espera_us=", int(fases157Acum[faseEspera157]/n),
		"actualizar_us=", int(fases157Acum[faseActualizar157]/n),
		"dibujar_us=", int(fases157Acum[faseDibujar157]/n),
		"convertir_us=", int(fases157Acum[faseConvertir157]/n),
		"subir_us=", int(fases157Acum[faseSubir157]/n),
		"pintar_us=", int(fases157Acum[fasePintar157]/n),
		"total_us=", int(fases157Acum[faseTotal157]/n),
		"rapida=", siNo157(palancas157Ventana[0]),
		"filas=", siNo157(palancas157Ventana[1]),
		"zona=", siNo157(palancas157Ventana[2]),
		"quieta=", siNo157(palancas157Ventana[3]))

	fases157Acum = [fases157Total]uint64{}
	cuadros157 = 0
	palancas157Ventana = palancasDeAhora157()
}

// siNo157: un bool como "si"/"no" (en la consola no hay strconv que valga la pena para esto).
func siNo157(v bool) string {
	if v {
		return "si"
	}

	return "no"
}

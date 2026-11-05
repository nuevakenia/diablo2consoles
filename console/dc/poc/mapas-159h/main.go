//go:build dreamcast

// SONDA-MAPA159H -- QUE TIPOS DE CLAVE AGUANTAN UN MAPA EN ESTA CONSOLA.
//
// POR QUE EXISTE (DC-002.159h). El camino HSV de la superficie de la consola usa `map[[2]float64]ColorM` y al
// TOCARLO el SH-4 muere con `Unhandled exception: PC 8c5231ba ... PR -> runtime.mapassign`. La tarjeta pide
// averiguar si el problema es "los hashers de float" o algo mas grande. Esta sonda es la discriminacion, y va
// de menos a mas: dos controles con clave escalar (cuyo hasher lo pone libgodc), el caso flojo del shim
// (`[2]float64`, que es el que revienta), y despues los otros compuestos (arreglo de enteros y struct con
// floats) para saber si el defecto es del float o de TODO tipo compuesto.
//
// CADA PASO IMPRIME SU LINEA ANTES DE OPERAR: el que falte en el log es el que mato la corrida.
//
// Se lee en la consola serie:  grep -a MAPA159H flycast.log
package main

import "strconv"

// clave con la misma forma que `matrixKey` del shim (cuatro uint32, dos bool y dos float64)
type claveHSV struct {
	r, g, b, a uint32
	hasColor   bool
	hsv        bool
	sat, brgt  float64
	effect     int
}

// matriz: la MISMA forma que `ebiten.ColorM` (veinte float64 = 160 B). El valor del mapa de la cache no es un
// escalar: libgodc guarda un valor de mas de una palabra INDIRECTO (una celda en el monton por entrada), que es
// un camino de `mapassign` distinto del que ejercitan los valores `int` de los pasos 1 a 7.
type matriz struct {
	v0, v1, v2, v3, v4      float64
	v5, v6, v7, v8, v9      float64
	v10, v11, v12, v13, v14 float64
	v15, v16, v17, v18, v19 float64
}

func main() {
	println("MAPA159H arranca")

	// 1. control: clave entera escalar. El hasher lo aporta libgodc (_runtime.memhash32..f).
	println("MAPA159H paso=1 tipo=map[uint32]int")
	m1 := map[uint32]int{}
	m1[7] = 71
	println("MAPA159H paso=1 ok valor=" + strconv.Itoa(m1[7]))

	// 2. control: clave float escalar. Si aca anda, "el hasher de float" NO es el problema.
	println("MAPA159H paso=2 tipo=map[float64]int")
	m2 := map[float64]int{}
	m2[1.5] = 72
	println("MAPA159H paso=2 ok valor=" + strconv.Itoa(m2[1.5]))

	// 3. control: clave string (el hasher es _runtime.strhash..f, tambien de libgodc).
	println("MAPA159H paso=3 tipo=map[string]int")
	m3 := map[string]int{}
	m3["hola"] = 73
	println("MAPA159H paso=3 ok valor=" + strconv.Itoa(m3["hola"]))

	// 4. EL CASO DE LA TARJETA: arreglo de dos float64, la clave del mapa HSV.
	println("MAPA159H paso=4 tipo=map[[2]float64]int")
	m4 := map[[2]float64]int{}
	k4 := [2]float64{1, 0}
	m4[k4] = 74
	println("MAPA159H paso=4 ok valor=" + strconv.Itoa(m4[k4]))

	// 5. arreglo de enteros: si este tambien muere, no es el float, son TODOS los compuestos.
	println("MAPA159H paso=5 tipo=map[[2]int]int")
	m5 := map[[2]int]int{}
	k5 := [2]int{9, 8}
	m5[k5] = 75
	println("MAPA159H paso=5 ok valor=" + strconv.Itoa(m5[k5]))

	// 6. struct con floats dentro: la forma de `matrixKey`/`tintHSVKey` del shim.
	println("MAPA159H paso=6 tipo=map[claveHSV]int")
	m6 := map[claveHSV]int{}
	k6 := claveHSV{1, 2, 3, 4, true, true, 0, 0, 0}
	m6[k6] = 76
	println("MAPA159H paso=6 ok valor=" + strconv.Itoa(m6[k6]))

	// 7. struct con SOLO enteros, para cerrar el control por el otro lado.
	println("MAPA159H paso=7 tipo=map[[3]int]int")
	m7 := map[[3]int]int{}
	k7 := [3]int{1, 2, 3}
	m7[k7] = 77
	println("MAPA159H paso=7 ok valor=" + strconv.Itoa(m7[k7]))

	// 8. LA FORMA EXACTA DE LA CACHE DEL SHIM: clave `[2]float64` y valor `ColorM` (veinte float64 = 160 B).
	// Importa porque con un valor de mas de una palabra libgodc guarda el valor INDIRECTO (una celda aparte en
	// el monton por entrada) y ese es otro camino de `mapassign` que la sonda de arriba, con valores `int`, no
	// toca. Es el mapa que revienta hoy: `hsvCache map[[2]float64]ebiten.ColorM`.
	println("MAPA159H paso=8 tipo=map[[2]float64]matriz (valor de 160 B, indirecto)")
	m8 := map[[2]float64]matriz{}
	k8 := [2]float64{1, 0}
	var celda matriz
	celda.v19 = 8.5
	m8[k8] = celda
	leido8 := m8[k8]
	println("MAPA159H paso=8 ok valor=" + strconv.FormatFloat(leido8.v19, 'f', 1, 64) + " len=" + strconv.Itoa(len(m8)))
	celda.v19 = 9.5
	m8[k8] = celda
	println("MAPA159H paso=8 bis reescrito=" + strconv.FormatFloat(m8[k8].v19, 'f', 1, 64) + " len=" + strconv.Itoa(len(m8)))

	// 9. y la forma de la OTRA cache del shim: `tintHSVCache map[tintHSVKey]ebiten.ColorM`.
	println("MAPA159H paso=9 tipo=map[claveHSV]matriz (valor de 160 B, indirecto)")
	m9 := map[claveHSV]matriz{}
	k9 := claveHSV{1, 2, 3, 4, true, true, 0, 0, 0}
	var celda9 matriz
	celda9.v19 = 7.25
	m9[k9] = celda9
	println("MAPA159H paso=9 ok valor=" + strconv.FormatFloat(m9[k9].v19, 'f', 2, 64) + " len=" + strconv.Itoa(len(m9)))

	// 10. y el mapa de la cache de tareas del renderer, que la tarjeta no toca pero usa el mismo mecanismo.
	println("MAPA159H paso=10 tipo=map[[2]int]string + borrado")
	m10 := map[[2]int]string{}
	k10 := [2]int{4, 5}
	m10[k10] = "celda"
	_, hay := m10[k10]
	delete(m10, k10)
	_, quedo := m10[k10]
	println("MAPA159H paso=10 ok habia=" + strconv.FormatBool(hay) + " quedo=" + strconv.FormatBool(quedo) + " len=" + strconv.Itoa(len(m10)))

	// 11. DISCRIMINACION DEL VALOR INDIRECTO: misma clave escalar (hasher de libgodc, sano) con el valor de 160 B.
	// Si ACÁ tambien vuelve cero, el defecto no es del hasher: es del camino del valor indirecto de libgodc.
	println("MAPA159H paso=11 tipo=map[uint32]matriz (clave escalar, valor indirecto)")
	m11 := map[uint32]matriz{}
	var celda11 matriz
	celda11.v19 = 6.75
	m11[3] = celda11
	println("MAPA159H paso=11 ok valor=" + strconv.FormatFloat(m11[3].v19, 'f', 2, 64) + " len=" + strconv.Itoa(len(m11)))

	// 12. y un valor indirecto de dos palabras (string), con clave compuesta y con clave escalar.
	println("MAPA159H paso=12 tipo=map[[2]float64]string y map[uint32]string (valor de dos palabras)")
	m12 := map[[2]float64]string{}
	m12[[2]float64{1, 2}] = "compuesta"
	m13 := map[uint32]string{}
	m13[5] = "escalar"
	println("MAPA159H paso=12 ok compuesta=" + m12[[2]float64{1, 2}] + " escalar=" + m13[5])

	println("MAPA159H fin: TODOS los mapas andan")
	for {
	}
}

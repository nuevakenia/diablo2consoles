// Comando de medicion: cuanto se aparta nuestra math de la math REAL de Go, funcion por funcion.
//
// No es un test que tenga que pasar: es el instrumento. Imprime, por funcion, cuantos puntos
// coinciden a 12 cifras significativas (que es el criterio del corpus) y el error relativo peor.
//
// Uso:  go run ./cmd/medir-math
package main

import (
	"fmt"
	realmath "math"

	mymath "d2std/math"
)

type fn struct {
	nombre string
	mio    func(float64) float64
	real   func(float64) float64
	puntos []float64
}

var puntos = []float64{
	0, -0.0, 1e-30, -1e-30, 1e-10, 0.1, 0.25, 0.333333333333, 0.5, 0.7071067811865476,
	0.7853981633974483, 1, 1.5, 2, 2.5, 2.718281828459045, 3, 3.141592653589793,
	4, 10, 42, 100, 1000, 12345.6789, 1e6, 1e10, 1e20, 1e100, 1e300, 1.7976931348623157e308,
	-1e-10, -0.1, -0.5, -1, -1.5, -2, -3.141592653589793, -10, -100, -1000, -1e6, -1e20,
	123456.789, 0.987654321012345, 8.835, 99.95, 1.0 / 3.0,
}

func main() {
	var funciones []fn
	for _, f := range []struct {
		nombre string
		mio    func(float64) float64
		real   func(float64) float64
	}{
		{"Sqrt", mymath.Sqrt, realmath.Sqrt},
		{"Exp", mymath.Exp, realmath.Exp},
		{"Log", mymath.Log, realmath.Log},
		{"Sin", mymath.Sin, realmath.Sin},
		{"Cos", mymath.Cos, realmath.Cos},
		{"Atan", mymath.Atan, realmath.Atan},
		{"Acos", mymath.Acos, realmath.Acos},
		{"Floor", mymath.Floor, realmath.Floor},
		{"Ceil", mymath.Ceil, realmath.Ceil},
		{"Trunc", mymath.Trunc, realmath.Trunc},
		{"Round", mymath.Round, realmath.Round},
		{"RoundToEven", mymath.RoundToEven, realmath.RoundToEven},
		{"Abs", mymath.Abs, realmath.Abs},
	} {
		funciones = append(funciones, fn{f.nombre, f.mio, f.real, puntos})
	}

	totalMal := 0
	for _, f := range funciones {
		mal := 0
		var peor float64
		var peorX float64
		var mio, real float64
		for _, x := range f.puntos {
			a := f.mio(x)
			b := f.real(x)
			if igualesAN(a, b, 12) {
				continue
			}
			mal++
			e := relErr(a, b)
			if e > peor {
				peor, peorX, mio, real = e, x, a, b
			}
		}
		totalMal += mal
		if mal == 0 {
			fmt.Printf("  %-12s %2d/%2d puntos a 12 cifras\n", f.nombre, len(f.puntos), len(f.puntos))
		} else {
			fmt.Printf("  %-12s %2d/%2d puntos   peor: x=%v mio=%v go=%v err=%.3e\n",
				f.nombre, len(f.puntos)-mal, len(f.puntos), peorX, mio, real, peor)
		}
	}

	// binarias y especiales
	fmt.Println("-- Atan2 (y, x) en 12 puntos")
	malA2 := 0
	bin := [][2]float64{{0, 1}, {1, 1}, {1, 0}, {1, -1}, {0, -1}, {-1, -1}, {-1, 0}, {-1, 1}, {3, 4}, {-3, 4}, {1e-10, -1e10}, {1e10, 1e-10}}
	for _, p := range bin {
		a := mymath.Atan2(p[0], p[1])
		b := realmath.Atan2(p[0], p[1])
		if !igualesAN(a, b, 12) {
			malA2++
			fmt.Printf("     atan2(%v, %v) mio=%v go=%v err=%.3e\n", p[0], p[1], a, b, relErr(a, b))
		}
	}
	fmt.Printf("     %d/%d a 12 cifras\n", len(bin)-malA2, len(bin))
	fmt.Println("-- Hypot y Pow")
	malH := 0
	for _, p := range [][2]float64{{3, 4}, {5, 12}, {1e-200, 1e-200}, {1e300, 1e300}, {0, 1}, {-3, -4}} {
		a := mymath.Hypot(p[0], p[1])
		b := realmath.Hypot(p[0], p[1])
		if !igualesAN(a, b, 12) {
			malH++
			fmt.Printf("     hypot(%v, %v) mio=%v go=%v err=%.3e\n", p[0], p[1], a, b, relErr(a, b))
		}
	}
	fmt.Printf("     %d/6 a 12 cifras\n", 6-malH)
	malP := 0
	np := 0
	for _, p := range [][2]float64{{2, 2}, {2, 3}, {2, 0.5}, {10, 3}, {1.5, 2.5}, {-2, 3}, {-2, 2}, {0.5, -2}, {3, 12}, {1e10, 3}, {2, 100}, {7, -0.3333333333}, {-8, 1.0 / 3.0}} {
		np++
		a := mymath.Pow(p[0], p[1])
		b := realmath.Pow(p[0], p[1])
		if !igualesAN(a, b, 12) {
			malP++
			fmt.Printf("     pow(%v, %v) mio=%v go=%v err=%.3e\n", p[0], p[1], a, b, relErr(a, b))
		}
	}
	fmt.Printf("     %d/%d a 12 cifras\n", np-malP, np)

	fmt.Println("-- Ldexp y constantes")
	malL := 0
	for _, p := range [][2]float64{{1, 10}, {1.5, -5}, {3, 1023}, {3, 1100}, {3, -1100}, {0.1, 0}} {
		a := mymath.Ldexp(p[0], int(p[1]))
		b := realmath.Ldexp(p[0], int(p[1]))
		if !igualesAN(a, b, 12) {
			malL++
			fmt.Printf("     ldexp(%v, %v) mio=%v go=%v\n", p[0], p[1], a, b)
		}
	}
	fmt.Printf("     %d/6 a 12 cifras\n", 6-malL)
	// MaxInt64 se compara como int64: es una constante SIN TIPO (como en Go), asi que usarla como
	// int no compila con int de 32 bits. Eso es lo correcto: en la consola el motor tiene el mismo
	// limite que tendria con el Go de 32 bits (medido con GOARCH=386).
	fmt.Printf("     Pi igual: %v   Sqrt2 igual: %v   MaxFloat64 igual: %v   MaxInt32 %d/%d   MaxInt64 %d/%d\n",
		mymath.Pi == realmath.Pi, mymath.Sqrt2 == realmath.Sqrt2,
		mymath.MaxFloat64 == realmath.MaxFloat64,
		mymath.MaxInt32, realmath.MaxInt32, int64(mymath.MaxInt64), int64(realmath.MaxInt64))

	fmt.Println("-- Inf, NaN, IsInf, IsNaN, Signbit, Copysign")
	fmt.Printf("     Inf(1) mio=%v go=%v  igual=%v\n", mymath.Inf(1), realmath.Inf(1), mymath.Inf(1) == realmath.Inf(1))
	fmt.Printf("     Inf(-1) igual=%v  IsNaN(NaN)=%v  IsNaN(1)=%v\n", mymath.Inf(-1) == realmath.Inf(-1), mymath.IsNaN(mymath.NaN()), mymath.IsNaN(1))
	fmt.Printf("     IsInf: +%v -%v cero=%v  Signbit(-0)=%v  Copysign(3,-0)=%v\n",
		mymath.IsInf(mymath.Inf(1), 1), mymath.IsInf(mymath.Inf(-1), -1), mymath.IsInf(1, 0),
		mymath.Signbit(realmath.Copysign(0, -1)), mymath.Copysign(3, realmath.Copysign(0, -1)))
	fmt.Printf("     Floor(-0.5)=%v  Round(-0.5)=%v  RoundToEven(2.5)=%v  RoundToEven(0.5)=%v\n",
		mymath.Floor(-0.5), mymath.Round(-0.5), mymath.RoundToEven(2.5), mymath.RoundToEven(0.5))

	if totalMal > 0 {
		fmt.Printf("\n== %d puntos fuera de 12 cifras (arriba)\n", totalMal)
	} else {
		fmt.Println("\n== todo a 12 cifras significativas")
	}
}

// igualesAN dice si a y b coinciden en n cifras significativas.
func igualesAN(a, b float64, n int) bool {
	if a == b {
		return true
	}
	if realmath.IsNaN(a) && realmath.IsNaN(b) {
		return true
	}
	if realmath.IsInf(a, 0) || realmath.IsInf(b, 0) {
		return false
	}
	tol := realmath.Abs(b) / realmath.Pow(10, float64(n))
	return realmath.Abs(a-b) <= tol
}

func relErr(a, b float64) float64 {
	if b == 0 {
		return realmath.Abs(a)
	}
	return realmath.Abs(a-b) / realmath.Abs(b)
}

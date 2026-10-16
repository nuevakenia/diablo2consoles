// Paquete math: el subconjunto que el motor usa de verdad.
//
// Por que existe: en sh-elf no hay biblioteca estandar de Go (ver el go.mod del modulo d2std).
// El motor pide 25 simbolos de math en 88 archivos: Abs, Acos, Atan2, Ceil, Cos, Exp, Floor,
// Hypot, Inf, IsInf, IsNaN, Ldexp, Max, MaxFloat64, MaxInt32, MaxInt64, MaxUint16, Min, Pi, Pow,
// Round, RoundToEven, Sin, Sqrt, Sqrt2 (medido con scripts/console/audit-stdlib.py; el detalle en
// console/dc/evidencia/stdlib-que-falta.txt).
//
// Como se verifica: cada caso del corpus lleva la respuesta calculada por la math REAL de Go en el
// host y se compara contra la de aca con 12 digitos significativos (%.12g). Las exactas (Abs,
// Floor, Ceil, Trunc, Round, RoundToEven, Min, Max, Ldexp, Signbit, Copysign) tienen que coincidir
// bit a bit; las trascendentales van con polinomios propios y su limite esta declarado abajo.
//
// LIMITES DECLARADOS (medidos, no supuestos):
//   - Las trascendentales (Exp, Log, Sin, Cos, Atan2, Acos, Hypot, Pow, Sqrt) NO coinciden bit a
//     bit con las de Go: se calculan con polinomios en Go puro. La libm del C tampoco serviria:
//     su resultado es otro (y en la consola es la libm del SH-4). Coinciden a 12 digitos
//     significativos, que es lo que el corpus prueba.
//   - Sin/Cos y Atan2 reducen el argumento, asi que para |x| > 2^20 la reduccion pierde
//     precision. El motor trabaja con angulos, no en ese rango, pero queda declarado.
//   - Pow con exponente grande acumula error relativo ~1e-13 (es exp(y*log(x)) con correcciones).
//   - El costo de tiempo esta MEDIDO en la consola (lo imprime el autotest, DC-002.4): 1e6 de
//     multiplicacion+suma con float64 tardan 20 ms (lo mismo que con float32: la aritmetica la hace
//     la FPU, no es software). Lo caro son las trascendentales de ESTE paquete, que son polinomios
//     en Go: sqrt ~1,3 us y sin/cos/exp/log/atan2/pow entre 2,5 y 3,5 us cada una. El motor deberia
//     evitar esas llamadas adentro de un bucle por frame.
package math

// Constantes (las que el motor toca: MaxFloat64, MaxInt32, MaxInt64, MaxUint16, Pi, Sqrt2).
const (
	Pi    = 3.14159265358979323846264338327950288419716939937510582097494459
	Sqrt2 = 1.41421356237309504880168872420969807856967187537694807317667974
	// MaxFloat64 va en hexadecimal a proposito: Go compara las constantes de punto flotante de
	// forma EXACTA (no redondeada), asi que el literal decimal 1.7976931348623157e308 no es igual a
	// esta constante aunque al pasar a float64 tenga los mismos bits (medido: mismo %x, == false).
	MaxFloat64 = 0x1.fffffffffffffp1023 // no con cuentas de bits: el compilador las pliega a 2^1023
	MaxInt32   = 1<<31 - 1
	MaxInt64   = 1<<63 - 1
	MaxUint16  = 1<<16 - 1
)

// Inf devuelve el infinito con el signo pedido (sign >= 0: +Inf).
// Se calcula dividiendo por cero en tiempo de ejecucion a proposito: +Inf como constante no
// compila (desborda), y ademas asi no depende del redondeo del compilador.
func Inf(sign int) float64 {
	if sign >= 0 {
		return 1 / zero()
	}
	return -1 / zero()
}

// NaN devuelve "no es un numero".
func NaN() float64 { return 0 / zero() }

// zero existe para que el compilador no colapse la division y la vuelva una constante.
func zero() float64 {
	var z float64
	return z
}

// IsNaN dice si x no es un numero. (NaN es lo unico que no es igual a si mismo.)
func IsNaN(x float64) bool { return x != x }

// IsInf dice si x es infinito con el signo pedido (sign == 0: cualquiera de los dos).
func IsInf(x float64, sign int) bool {
	if sign >= 0 && x > MaxFloat64 {
		return true
	}
	if sign <= 0 && x < -MaxFloat64 {
		return true
	}
	return false
}

// Signbit dice si x es negativo, incluido el cero negativo.
func Signbit(x float64) bool {
	return x < 0 || (x == 0 && 1/x < 0)
}

// Copysign devuelve x con el signo de y.
func Copysign(x, y float64) float64 {
	if Signbit(x) != Signbit(y) {
		return -x
	}
	return x
}

// Abs devuelve el valor absoluto de x.
func Abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	if x == 0 {
		return 0 // el cero negativo se vuelve positivo
	}
	return x
}

// truncAInt convierte x a entero si esta en el rango donde la conversion es exacta, y dice si
// pudo. Fuera de ahi (|x| >= 2^52) no hay parte fraccionaria que sacar: x ya es entero.
func truncAInt(x float64) (int64, bool) {
	if Abs(x) >= 1<<52 {
		return 0, false
	}
	return int64(x), true // en Go la conversion trunca hacia cero
}

// Floor devuelve el mayor entero <= x.
func Floor(x float64) float64 {
	if x == 0 || IsNaN(x) || IsInf(x, 0) {
		return x
	}
	i, ok := truncAInt(x)
	if !ok {
		return x
	}
	if x < 0 && float64(i) != x {
		i--
	}
	// Copysign y no float64(i) seco: Go devuelve el cero con el signo del argumento (Floor(-0.5)
	// es -0, no +0), y float64(0) pierde ese signo. Lo cazo el corpus (DC-002.4, 7 casos).
	return Copysign(float64(i), x)
}

// Ceil devuelve el menor entero >= x.
func Ceil(x float64) float64 {
	if x == 0 || IsNaN(x) || IsInf(x, 0) {
		return x
	}
	i, ok := truncAInt(x)
	if !ok {
		return x
	}
	if x > 0 && float64(i) != x {
		i++
	}
	return Copysign(float64(i), x) // el cero con el signo del argumento (ver Floor)
}

// Trunc devuelve la parte entera de x (redondeo hacia cero).
func Trunc(x float64) float64 {
	if x == 0 || IsNaN(x) || IsInf(x, 0) {
		return x
	}
	i, ok := truncAInt(x)
	if !ok {
		return x
	}
	return Copysign(float64(i), x) // el cero con el signo del argumento (ver Floor)
}

// Round devuelve el entero mas cercano, y los .5 se van hacia afuera (Round(0.5)=1, Round(-0.5)=-1).
func Round(x float64) float64 {
	if x == 0 || IsNaN(x) || IsInf(x, 0) {
		return x
	}
	t := Trunc(x)
	r := x - t
	if r >= 0.5 {
		return t + 1
	}
	if r <= -0.5 {
		return t - 1
	}
	return Copysign(t, x) // el cero con el signo del argumento (ver Floor)
}

// RoundToEven devuelve el entero mas cercano, y los .5 al par (RoundToEven(0.5)=0, RoundToEven(2.5)=2).
// Es el redondeo por defecto de la IEEE, y el que usa la reduccion de argumento de Sin/Cos.
func RoundToEven(x float64) float64 {
	if x == 0 || IsNaN(x) || IsInf(x, 0) {
		return x
	}
	t := Trunc(x)
	r := x - t
	switch {
	case r > 0.5:
		return t + 1
	case r < -0.5:
		return t - 1
	case r == 0.5:
		return 2 * Round(t/2)
	case r == -0.5:
		return 2 * Round(t/2)
	}
	return Copysign(t, x) // el cero con el signo del argumento (ver Floor)
}

// Min devuelve el menor de los dos. Si uno es NaN, devuelve el otro (como Go).
func Min(x, y float64) float64 {
	switch {
	case IsNaN(x) && IsNaN(y):
		return NaN()
	case IsNaN(x):
		return y
	case IsNaN(y):
		return x
	case x < y:
		return x
	case y < x:
		return y
	case Signbit(x):
		return x // los dos son cero: gana el -0
	}
	return y
}

// Max devuelve el mayor de los dos. Si uno es NaN, devuelve el otro (como Go).
func Max(x, y float64) float64 {
	switch {
	case IsNaN(x) && IsNaN(y):
		return NaN()
	case IsNaN(x):
		return y
	case IsNaN(y):
		return x
	case x > y:
		return x
	case y > x:
		return y
	case Signbit(x):
		return y // los dos son cero: gana el +0
	}
	return x
}

// pow2 devuelve 2^e multiplicando, que es exacto mientras no haya desborde ni subnormal.
func pow2(e int) float64 {
	r := 1.0
	for ; e > 0; e-- {
		r *= 2
	}
	for ; e < 0; e++ {
		r *= 0.5
	}
	return r
}

// Ldexp devuelve frac * 2^exp, con escalado en pasos para no desbordar en el camino.
func Ldexp(frac float64, exp int) float64 {
	if frac == 0 || IsNaN(frac) || IsInf(frac, 0) {
		return frac
	}
	for exp > 1023 {
		frac *= pow2(1023)
		exp -= 1023
		if IsInf(frac, 0) {
			return frac
		}
	}
	for exp < -1022 {
		frac *= pow2(-1022)
		exp += 1022
		if frac == 0 {
			return frac
		}
	}
	return frac * pow2(exp)
}

// Las funciones trascendentales de math, con polinomios propios.
//
// Por que no se llama a la libm del C: el resultado de la libm (y en la consola la del SH-4) no es
// el de Go, y el corpus compara contra la math REAL de Go. Tampoco se pueden copiar los polinomios
// minimax de Go: son datos generados por su generador. Aca los polinomios son series de Taylor con
// suficientes terminos para el rango reducido, y el error se MIDE contra Go (ver el encabezado de
// math.go y los casos del corpus).
//
// La tecnica es la de siempre: reducir el argumento a un rango chico (donde la serie converge
// rapido) y deshacer la reduccion al final. Lo que se pierde en cada reduccion esta declarado en
// math.go.
package math

// ---- raiz cuadrada ----

// Sqrt devuelve la raiz cuadrada de x. Newton con el argumento escalado a [0.5, 2): siete
// iteraciones dan error relativo < 1e-16 en ese rango, que es lo que necesitan las 12 cifras del
// corpus (la de Go da el resultado IEEE redondeado; esta puede diferir en el ultimo bit).
func Sqrt(x float64) float64 {
	if IsNaN(x) || x < 0 {
		return NaN()
	}
	if x == 0 || IsInf(x, 1) {
		return x
	}
	// x = z * 4^esc  con  z en [0.5, 2)
	esc := 0
	z := x
	for z >= 2 {
		z *= 0.25
		esc++
	}
	for z < 0.5 {
		z *= 4
		esc--
	}
	y := 1.0
	for i := 0; i < 7; i++ {
		y = 0.5 * (y + z/y)
	}
	return Ldexp(y, esc)
}

// ---- exponencial y logaritmo ----

// ln2 partido en dos para que k*ln2 no pierda cifras.
const (
	ln2Hi  = 6.93147180369123816490e-01 // ln2 redondeado
	ln2Lo  = 1.90821492927058770002e-10 // ln2 - ln2Hi
	invLn2 = 1.44269504088896338700e+00 // 1/ln2
)

// Exp devuelve e**x.
func Exp(x float64) float64 {
	if IsNaN(x) {
		return x
	}
	if x > 709.782712893384 {
		return Inf(1)
	}
	if x < -745.1332191019411 {
		return 0
	}
	if Abs(x) < 1.0/(1<<28) {
		return 1 + x
	}
	k := RoundToEven(x * invLn2)
	r := x - k*ln2Hi - k*ln2Lo // |r| <= ln2/2
	return Ldexp(expPoly(r), int(k))
}

// expPoly es exp(r) para |r| <= ln2/2, con la serie de Taylor hasta r**14 (el termino que se corta
// vale 4e-18, menos que el redondeo).
func expPoly(r float64) float64 {
	p := 1.0 / 87178291200 // 1/14!
	for n := 13; n >= 1; n-- {
		p = p*r + 1/fact(n)
	}
	return 1 + r*p
}

// fact devuelve n! para el chico n que usa expPoly (el compilador lo pliega en constantes).
func fact(n int) float64 {
	f := 1.0
	for i := 2; i <= n; i++ {
		f *= float64(i)
	}
	return f
}

// Log devuelve el logaritmo natural de x.
func Log(x float64) float64 {
	if IsNaN(x) {
		return x
	}
	if x < 0 {
		return NaN()
	}
	if x == 0 {
		return Inf(-1)
	}
	if IsInf(x, 1) {
		return x
	}
	// x = m * 2^k con m en [0.5, 1)
	k := 0
	m := x
	for m >= 1 {
		m *= 0.5
		k++
	}
	for m < 0.5 {
		m *= 2
		k--
	}
	// y con m en [sqrt(2)/2, sqrt(2)): la serie en (m-1)/(m+1) converge mejor y |s| <= 0.1716
	if m < 0.7071067811865476 {
		m *= 2
		k--
	}
	s := (m - 1) / (m + 1)
	z := s * s
	// log(m) = 2s * (1 + z/3 + z**2/5 + ...): Horner desde z**12 hasta el termino 1 (j = 0), que
	// es el que da el factor 1: si no se suma, la serie queda subestimada (ese fue el defecto).
	p := 0.0
	for j := 12; j >= 0; j-- {
		p = p*z + 1/float64(2*j+1)
	}
	return float64(k)*ln2Hi + (2*s*p + float64(k)*ln2Lo)
}

// ---- seno y coseno ----

// pi/2 partido en dos (la parte baja es lo que no entra en el primer float64).
const (
	pi2Hi = 1.5707963267948966
	pi2Lo = 6.123233995736766e-17
)

// cuadranteDe devuelve n modulo 4 (0..3) calculado en float: convertir un float64 enorme a int64
// es indeterminado en Go, y ademas asi queda escrito de donde sale el resto. Devuelve -1 si |n| es
// tan grande que la reduccion no es exacta (ahi Sin/Cos devuelven NaN: prefiero no mentir).
func cuadranteDe(n float64) int {
	if Abs(n) > 1<<52 {
		return -1
	}
	return int(n - 4*Floor(n/4)) // Floor exacto: el resultado queda en [0, 4)
}

// Sin devuelve el seno de x (en radianes). Reduce x a |r| <= pi/4 y elige el polinomio segun el
// cuadrante. El error de la reduccion crece con |x| (~|x|*1e-16); el dominio del motor (angulos) y
// el medido en el corpus es |x| <= 100, y esta declarado en math.go.
func Sin(x float64) float64 {
	if IsNaN(x) || IsInf(x, 0) {
		return NaN()
	}
	n := RoundToEven(x * (2 / Pi))
	q := cuadranteDe(n)
	if q < 0 {
		return NaN()
	}
	r := x - n*pi2Hi - n*pi2Lo
	switch q {
	case 0:
		return sinPoly(r)
	case 1:
		return cosPoly(r)
	case 2:
		return -sinPoly(r)
	}
	return -cosPoly(r)
}

// Cos devuelve el coseno de x (mismo dominio que Sin).
func Cos(x float64) float64 {
	if IsNaN(x) || IsInf(x, 0) {
		return NaN()
	}
	n := RoundToEven(x * (2 / Pi))
	q := cuadranteDe(n)
	if q < 0 {
		return NaN()
	}
	r := x - n*pi2Hi - n*pi2Lo
	switch q {
	case 0:
		return cosPoly(r)
	case 1:
		return sinPoly(r)
	case 2:
		return -cosPoly(r)
	}
	return -sinPoly(r)
}

// sinPoly es sin(r) para |r| <= pi/4: la serie de Taylor hasta r**19, con los coeficientes
// generados de los factoriales (a mano se me habian caido la mitad de los terminos y el error era
// 3e-3, medido con cmd/medir-math).
func sinPoly(r float64) float64 {
	z := r * r
	p := 0.0
	fac := fact(19)
	for j := 9; j >= 0; j-- {
		c := 1 / fac
		if j%2 == 1 {
			c = -c
		}
		p = p*z + c
		fac = fac / float64((2*j)*(2*j+1))
	}
	return r * p
}

// cosPoly es cos(r) para |r| <= pi/4: la serie hasta r**18, mismo criterio.
func cosPoly(r float64) float64 {
	z := r * r
	p := 0.0
	fac := fact(18)
	for j := 9; j >= 0; j-- {
		c := 1 / fac
		if j%2 == 1 {
			c = -c
		}
		p = p*z + c
		if j > 0 {
			fac = fac / float64((2*j-1)*(2*j))
		}
	}
	return p
}

// ---- arcotangente y sus derivadas ----

// Atan devuelve el arcotangente de x, en radianes.
func Atan(x float64) float64 {
	if IsNaN(x) {
		return x
	}
	if x == 0 {
		return x
	}
	if IsInf(x, 1) {
		return Pi / 2
	}
	if IsInf(x, -1) {
		return -Pi / 2
	}
	sig := 1.0
	if x < 0 {
		x = -x
		sig = -1
	}
	invertido := false
	if x > 1 {
		x = 1 / x
		invertido = true
	}
	extra := 0.0
	if x > 0.4142135623730951 { // tan(pi/8)
		x = (x - 1) / (x + 1)
		extra = Pi / 4
	}
	// aqui |x| <= 0.4142: tres medias angulos lo llevan a |x| <= 0.052, donde la serie de Taylor
	// con 14 terminos sobra (el corte vale 1e-22)
	for i := 0; i < 3; i++ {
		x = x / (1 + Sqrt(1+x*x))
	}
	t := atanPoly(x)
	res := extra + 8*t // las tres reducciones se deshacen duplicando tres veces
	if invertido {
		res = Pi/2 - res
	}
	return sig * res
}

// atanPoly es atan(t) para |t| <= 0.06: la serie hasta t**29. Los signos alternan (es una serie
// alternada: +1, -z/3, +z**2/5 ...); sumarlos todos positivos daba 1e-3 de error.
func atanPoly(t float64) float64 {
	z := t * t
	p := 0.0
	for j := 14; j >= 0; j-- {
		c := 1 / float64(2*j+1)
		if j%2 == 1 {
			c = -c
		}
		p = p*z + c
	}
	return t * p
}

// Atan2 devuelve el angulo del punto (x, y), en [-pi, pi].
func Atan2(y, x float64) float64 {
	switch {
	case IsNaN(x) || IsNaN(y):
		return NaN()
	case y == 0:
		if x >= 0 && !Signbit(x) {
			return y // atan2(+-0, +0) = +-0
		}
		return Copysign(Pi, y)
	case x == 0:
		return Copysign(Pi/2, y)
	case IsInf(y, 0):
		if IsInf(x, 0) {
			if x > 0 {
				return Copysign(Pi/4, y)
			}
			return Copysign(3*Pi/4, y)
		}
		return Copysign(Pi/2, y)
	case IsInf(x, 0):
		if x > 0 {
			return Copysign(0, y)
		}
		return Copysign(Pi, y)
	}
	q := Atan(y / x)
	if x < 0 {
		if y >= 0 {
			q += Pi
		} else {
			q -= Pi
		}
	}
	return q
}

// Acos devuelve el arcocoseno de x, en [0, pi]. Cerca de +-1 pierde cifras (es sqrt de algo que
// tiende a cero): queda declarado en math.go.
func Acos(x float64) float64 {
	if IsNaN(x) || x > 1 || x < -1 {
		return NaN()
	}
	if x == 1 {
		return 0
	}
	if x == -1 {
		return Pi
	}
	return Atan2(Sqrt(1-x*x), x)
}

// Hypot devuelve sqrt(p*p + q*q) sin desbordar en el camino (divide por el mayor).
func Hypot(p, q float64) float64 {
	p = Abs(p)
	q = Abs(q)
	if IsInf(p, 1) || IsInf(q, 1) {
		return Inf(1)
	}
	if p < q {
		p, q = q, p
	}
	if p == 0 {
		return 0
	}
	r := q / p
	return p * Sqrt(1+r*r)
}

// Pow devuelve x**y. Con exponente entero chico es exacta (por cuadrados); con el resto es
// exp(y*log x), que acumula ~1e-13 relativo con exponentes grandes (declarado en math.go).
func Pow(x, y float64) float64 {
	switch {
	case y == 0 || x == 1:
		return 1
	case y == 1:
		return x
	case IsNaN(x) || IsNaN(y):
		return NaN()
	case IsInf(y, 1):
		switch {
		case Abs(x) > 1:
			return Inf(1)
		case Abs(x) < 1:
			return 0
		}
		return 1
	case IsInf(y, -1):
		switch {
		case Abs(x) > 1:
			return 0
		case Abs(x) < 1:
			return Inf(1)
		}
		return 1
	case IsInf(x, 1):
		if y > 0 {
			return Inf(1)
		}
		return 0
	case IsInf(x, -1):
		if y > 0 {
			if y != Trunc(y) || Abs(y) >= 1<<53 {
				return Inf(1)
			}
			if int64(y)&1 != 0 {
				return Inf(-1)
			}
			return Inf(1)
		}
		return 0
	case x == 0:
		if Signbit(x) && y == Trunc(y) && Abs(y) < 1<<53 && int64(y)&1 != 0 {
			return Copysign(0, -1)
		}
		if y > 0 {
			return 0
		}
		return Inf(1)
	}

	neg := false
	if x < 0 {
		if y != Trunc(y) || Abs(y) >= 1<<53 {
			return NaN() // raiz no entera de un negativo
		}
		neg = int64(y)&1 != 0
		x = -x
	}
	if n, ok := enteroChico(y); ok {
		r := 1.0
		for i := 0; i < n; i++ {
			r *= x // exponente entero: cuadrados exactos, sin pasar por log
		}
		if y < 0 {
			r = 1 / r // el reciproco, tambien exacto
		}
		if neg {
			return -r
		}
		return r
	}
	r := Exp(y * Log(x))
	if neg {
		return -r
	}
	return r
}

// enteroChico dice si y es un entero con |y| <= 64 y devuelve su valor absoluto.
func enteroChico(y float64) (int, bool) {
	if y != Trunc(y) || Abs(y) > 64 {
		return 0, false
	}
	n := int(y)
	if n < 0 {
		n = -n
	}
	return n, true
}

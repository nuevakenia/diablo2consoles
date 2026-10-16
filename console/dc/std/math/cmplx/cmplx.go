// Package cmplx: los numeros complejos de Go, para la consola.
//
// Los pide el decodificador de video (bink_audio.go) del motor. Son cuentas directas sobre float64, sin
// nada especifico de la plataforma, asi que se pueden comparar contra las de verdad en el host.
//
// Limite declarado: no estan Asin, Acos, Atan, Asinh, Acosh, Atanh ni Sqrt (las que necesitan ramas de
// funciones elementales mas finas). Si el motor las pide, se agregan con su prueba; las que estan son
// las que se usan.
package cmplx

import "math"

// Abs: el modulo (hipotenusa de la parte real e imaginaria).
func Abs(x complex128) float64 { return math.Hypot(real(x), imag(x)) }

// Phase: el angulo.
func Phase(x complex128) float64 { return math.Atan2(imag(x), real(x)) }

// Conj: el conjugado.
func Conj(x complex128) complex128 { return complex(real(x), -imag(x)) }

// IsNaN / IsInf: si alguna parte lo es.
func IsNaN(x complex128) bool { return math.IsNaN(real(x)) || math.IsNaN(imag(x)) }
func IsInf(x complex128) bool { return math.IsInf(real(x), 0) || math.IsInf(imag(x), 0) }

// Exp: e elevado a x.
func Exp(x complex128) complex128 {
	m := math.Exp(real(x))
	return complex(m*math.Cos(imag(x)), m*math.Sin(imag(x)))
}

// Log: el logaritmo natural.
func Log(x complex128) complex128 {
	return complex(math.Log(Abs(x)), Phase(x))
}

// Pow: x elevado a y, por la via exponencial (es la misma cuenta que hace Go).
func Pow(x, y complex128) complex128 {
	if x == 0 {
		if real(y) == 0 && imag(y) == 0 {
			return 1
		}
		if real(y) > 0 {
			return 0
		}
		return complex(math.Inf(1), math.Inf(1))
	}
	modulus := math.Pow(Abs(x), real(y))
	arg := Phase(x) * real(y)
	return complex(modulus*math.Cos(arg), modulus*math.Sin(arg)) * Exp(complex(0, imag(y)*math.Log(Abs(x))))
}

// Sqrt: la raiz cuadrada principal (la que Go devuelve, la de parte real no negativa).
func Sqrt(x complex128) complex128 {
	if imag(x) == 0 {
		if real(x) >= 0 {
			return complex(math.Sqrt(real(x)), 0)
		}
		return complex(0, math.Sqrt(-real(x)))
	}
	r := Abs(x)
	t := math.Sqrt((r + math.Abs(real(x))) / 2)
	if real(x) < 0 {
		t = t
	}
	var u float64
	if real(x) < 0 {
		u = imag(x) / (2 * t)
	} else {
		u = math.Sqrt((r - math.Abs(real(x))) / 2)
	}
	if imag(x) < 0 {
		u = -u
	}
	return complex(t, u)
}

// Rect: arma un complejo desde modulo y angulo.
func Rect(r, theta float64) complex128 {
	return complex(r*math.Cos(theta), r*math.Sin(theta))
}

// Polar: el modulo y el angulo.
func Polar(x complex128) (r, theta float64) { return Abs(x), Phase(x) }

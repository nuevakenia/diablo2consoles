// Paquete rand: numeros pseudoaleatorios, con la interfaz de math/rand.
//
// Por que existe: el motor lo usa en 28 archivos (Int, Int63, Intn, New, NewSource, Rand, Seed,
// Source, Uint32). En sh-elf no hay biblioteca estandar de Go.
//
// LIMITE DECLARADO (importante): la SECUENCIA no es la de Go. El generador de Go (math/rand) es un
// lagged Fibonacci con una tabla de 607 numeros generada en su dia para esa implementacion: eso son
// datos, no algoritmo, y copiarlos no aporta nada al port (el juego necesita que la misma semilla
// de la misma partida, no que la semilla 1 de una misma secuencia que la de Go). Lo que SI se
// garantiza y se prueba en el corpus: (a) la misma semilla da la misma secuencia (determinismo),
// (b) Intn(n) siempre cae en [0, n) y (c) no hay repeticiones cortas (se mide con 10000 muestras).
//
// El generador de aca es xorshift64*, que es deterministico, rapido (importa en un SH-4 sin FPU de
// doble) y de periodo 2^64-1.
package rand

// anchoInt es el ancho de int de la plataforma: 32 en la consola (SH-4) y 64 en el host. Es el
// idioma de Go para esto, y evalua en tiempo de compilacion.
const (
	anchoInt = 32 << (^uint(0) >> 63) // 32 o 64 segun la plataforma
	shiftInt = uint(64 - anchoInt)    // 32 en la consola (int de 32 bits), 0 en el host
	maxInt63 = 1<<63 - 1
)

// Source es la fuente de numeros, con la misma forma que la de Go.
type Source interface {
	Int63() int64
	Seed(seed int64)
}

// fuente es la implementacion concreta (xorshift64*).
type fuente struct {
	state uint64
}

// NewSource devuelve una fuente deterministica sembrada con seed.
func NewSource(seed int64) Source {
	f := &fuente{}
	f.Seed(seed)
	return f
}

// Seed vuelve a sembrar. La semilla 0 se corre a un valor no nulo: xorshift con estado 0 no sale
// nunca de 0 (y el motor usa rand.Seed(1) al arrancar, pero mejor no dejarlo al azar).
func (f *fuente) Seed(seed int64) {
	s := uint64(seed)
	if s == 0 {
		s = 0x9E3779B97F4A7C15
	}
	f.state = s
	// unas vueltas para que semillas chicas y parecidas no den estados parecidos
	for i := 0; i < 4; i++ {
		f.next()
	}
}

func (f *fuente) next() uint64 {
	x := f.state
	x ^= x >> 12
	x ^= x << 25
	x ^= x >> 27
	f.state = x
	return x * 0x2545F4914F6CDD1D
}

// Int63 devuelve un entero no negativo de 63 bits, como el de Go.
func (f *fuente) Int63() int64 {
	return int64(f.next() >> 1)
}

// Rand envuelve una fuente, con la misma forma que *rand.Rand de Go.
type Rand struct {
	src Source
}

// New envuelve una fuente.
func New(src Source) *Rand { return &Rand{src: src} }

// Int63 devuelve un entero no negativo de 63 bits.
func (r *Rand) Int63() int64 { return r.src.Int63() }

// Uint32 devuelve 32 bits al azar.
func (r *Rand) Uint32() uint32 { return uint32(r.src.Int63() >> 31) }

// Uint64 devuelve 64 bits al azar.
func (r *Rand) Uint64() uint64 { return uint64(r.src.Int63())<<1 | uint64(r.src.Int63()&1) }

// Int devuelve un entero no negativo con el ancho de int de la plataforma (32 en la consola, con lo
// que Int() tiene 31 bits utiles alla y 63 aca, como en Go).
func (r *Rand) Int() int { return int(r.src.Int63() >> shiftInt) }

// Int31 devuelve 31 bits al azar.
func (r *Rand) Int31() int32 { return int32(r.src.Int63() >> 32) }

// Intn devuelve un entero en [0, n). Con n <= 0 revienta, como en Go.
func (r *Rand) Intn(n int) int {
	if n <= 0 {
		panic("math/rand: Intn con n <= 0")
	}
	if n&(n-1) == 0 { // potencia de dos: directo con mascara
		return int(r.src.Int63() & int64(n-1))
	}
	// rechazo: evita el sesgo de la division (el resto no es uniforme)
	max := int64(maxInt63 - maxInt63%int64(n))
	for {
		v := r.src.Int63()
		if v <= max {
			return int(v % int64(n))
		}
	}
}

// Int63n devuelve un entero en [0, n).
func (r *Rand) Int63n(n int64) int64 {
	if n <= 0 {
		panic("math/rand: Int63n con n <= 0")
	}
	if n&(n-1) == 0 {
		return r.src.Int63() & (n - 1)
	}
	max := int64(maxInt63 - maxInt63%n)
	for {
		v := r.src.Int63()
		if v <= max {
			return v % n
		}
	}
}

// Float64 devuelve un float64 en [0.0, 1.0).
func (r *Rand) Float64() float64 {
	return float64(r.src.Int63()) / (1 << 63)
}

// Perm devuelve una permutacion de n (Fisher-Yates, como Go).
func (r *Rand) Perm(n int) []int {
	m := make([]int, n)
	for i := 0; i < n; i++ {
		j := r.Intn(i + 1)
		m[i] = m[j]
		m[j] = i
	}
	return m
}

// Shuffle desordena el arreglo en el lugar.
func (r *Rand) Shuffle(n int, swap func(i, j int)) {
	for i := n - 1; i > 0; i-- {
		j := r.Intn(i + 1)
		swap(i, j)
	}
}

// ---- la fuente global, con la forma de las funciones sueltas de Go ----

var global = New(NewSource(1))

// Seed siembra la fuente global.
func Seed(seed int64) {
	if f, ok := global.src.(*fuente); ok {
		f.Seed(seed)
	}
}

// Int63, Int, Int31, Uint32, Intn, Float64, Perm y Shuffle usan la fuente global.
func Int63() int64                       { return global.Int63() }
func Int() int                           { return global.Int() }
func Int31() int32                       { return global.Int31() }
func Uint32() uint32                     { return global.Uint32() }
func Uint64() uint64                     { return global.Uint64() }
func Intn(n int) int                     { return global.Intn(n) }
func Int63n(n int64) int64               { return global.Int63n(n) }
func Float64() float64                   { return global.Float64() }
func Perm(n int) []int                   { return global.Perm(n) }
func Shuffle(n int, swap func(i, j int)) { global.Shuffle(n, swap) }

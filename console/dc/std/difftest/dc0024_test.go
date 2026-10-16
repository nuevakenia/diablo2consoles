// Prueba diferencial de DC-002.4: math, math/bits, math/rand, hash/fnv, encoding/hex,
// encoding/binary, bytes.Reader y container/ring contra la biblioteca estandar REAL de Go.
//
// El criterio es el mismo que el del corpus de la consola (console/dc/poc/stdlib): las funciones
// EXACTAS tienen que dar el mismo patron de bits (math.Float64bits), y las trascendentales (que aca
// son polinomios propios, con el limite declarado en el paquete) tienen que caer dentro de 1e-12
// relativo. El mismo criterio en los dos lados, para que un verde no signifique una cosa alla y
// otra aca.
//
// Lo que NO se puede comparar contra Go va con una prueba de propiedad y esta dicho ahi mismo: el
// generador de math/rand es propio (la secuencia de Go es un lagged Fibonacci con una tabla de datos
// generada), asi que se prueba determinismo por semilla, rango y cobertura.
//
// go test ./... dentro de console/dc/std
package difftest

import (
	realbytes "bytes"
	realring "container/ring"
	realbinary "encoding/binary"
	realhex "encoding/hex"
	realfmt "fmt"
	realfnv "hash/fnv"
	realmath "math"
	realbits "math/bits"
	"testing"

	mybytes "d2std/bytes"
	myring "d2std/container/ring"
	mybinary "d2std/encoding/binary"
	myhex "d2std/encoding/hex"
	myfnv "d2std/hash/fnv"
	mymath "d2std/math"
	mybits "d2std/math/bits"
	myrand "d2std/math/rand"
)

// mismoPatron dice si los dos float64 tienen los MISMOS bits.
func mismoPatron(a, b float64) bool { return realmath.Float64bits(a) == realmath.Float64bits(b) }

// dentro dice si a esta a menos de tol relativo de b.
func dentro(a, b, tol float64) bool {
	if realmath.IsNaN(a) && realmath.IsNaN(b) {
		return true
	}
	// dos infinitos del mismo signo son el mismo resultado, no una diferencia infinita
	if realmath.IsInf(a, 0) && realmath.IsInf(b, 0) {
		return realmath.IsInf(a, 1) == realmath.IsInf(b, 1)
	}
	if realmath.IsInf(a, 0) || realmath.IsInf(b, 0) {
		return false
	}
	d := realmath.Abs(a - b)
	if realmath.Abs(b) == 0 {
		return d == 0
	}
	return d <= realmath.Abs(b)*tol
}

// puntosMath son los puntos que el corpus tambien prueba en la consola.
var puntosMath = []float64{0, realmath.Copysign(0, -1), 0.1, -0.1, 0.5, -0.5, 1, -1, 1.5, -1.5,
	2, -2, 2.5, -2.5, 3.7, -3.7, 42, -42, 100, -100, 12345.678, -12345.678, 1e10, 1e-10,
	0.3333333333333333, 0.7071067811865476, realmath.Pi, 0.987654321012345}

func TestMathExactasPorBits(t *testing.T) {
	unarias := []struct {
		nombre string
		mio    func(float64) float64
		real   func(float64) float64
	}{
		{"Abs", mymath.Abs, realmath.Abs},
		{"Floor", mymath.Floor, realmath.Floor},
		{"Ceil", mymath.Ceil, realmath.Ceil},
		{"Trunc", mymath.Trunc, realmath.Trunc},
		{"Round", mymath.Round, realmath.Round},
		{"RoundToEven", mymath.RoundToEven, realmath.RoundToEven},
	}
	for _, u := range unarias {
		for _, x := range puntosMath {
			a, b := u.mio(x), u.real(x)
			if !mismoPatron(a, b) {
				t.Errorf("%s(%v): mio=%016x go=%016x", u.nombre, x, realmath.Float64bits(a), realmath.Float64bits(b))
			}
		}
	}
	// el signo del cero es parte del resultado (fue el defecto que cazo el corpus en la consola)
	for _, x := range []float64{-0.1, -0.5, -0.9, -0.0, 0.0, 0.1, 0.5, 0.9} {
		for _, u := range []struct {
			nombre string
			mio    func(float64) float64
			real   func(float64) float64
		}{{"Floor", mymath.Floor, realmath.Floor}, {"Ceil", mymath.Ceil, realmath.Ceil},
			{"Trunc", mymath.Trunc, realmath.Trunc}, {"Round", mymath.Round, realmath.Round},
			{"RoundToEven", mymath.RoundToEven, realmath.RoundToEven}} {
			a, b := u.mio(x), u.real(x)
			if !mismoPatron(a, b) {
				t.Errorf("signo del cero %s(%v): mio=%016x go=%016x", u.nombre, x, realmath.Float64bits(a), realmath.Float64bits(b))
			}
		}
	}
}

func TestMathTrascendentales(t *testing.T) {
	unarias := []struct {
		nombre  string
		mio     func(float64) float64
		real    func(float64) float64
		dominio func(float64) bool
	}{
		{"Sqrt", mymath.Sqrt, realmath.Sqrt, func(x float64) bool { return x >= 0 }},
		{"Exp", mymath.Exp, realmath.Exp, func(x float64) bool { return true }},
		{"Log", mymath.Log, realmath.Log, func(x float64) bool { return x > 0 }},
		{"Sin", mymath.Sin, realmath.Sin, func(x float64) bool { return realmath.Abs(x) <= 100 }},
		{"Cos", mymath.Cos, realmath.Cos, func(x float64) bool { return realmath.Abs(x) <= 100 }},
		{"Atan", mymath.Atan, realmath.Atan, func(x float64) bool { return true }},
		{"Acos", mymath.Acos, realmath.Acos, func(x float64) bool { return x >= -1 && x <= 1 }},
	}
	for _, u := range unarias {
		for _, x := range puntosMath {
			if !u.dominio(x) {
				continue
			}
			a, b := u.mio(x), u.real(x)
			if !dentro(a, b, 1e-12) {
				t.Errorf("%s(%v): mio=%v go=%v", u.nombre, x, a, b)
			}
		}
	}
	binarias := []struct {
		nombre string
		mio    func(float64, float64) float64
		real   func(float64, float64) float64
	}{
		{"Atan2", mymath.Atan2, realmath.Atan2},
		{"Hypot", mymath.Hypot, realmath.Hypot},
		{"Pow", mymath.Pow, realmath.Pow},
	}
	pares := [][2]float64{{2, 2}, {2, 3}, {2, 0.5}, {10, 3}, {1.5, 2.5}, {0.5, -2}, {3, 12},
		{3, 4}, {-3, 4}, {1, 1}, {0, 1}, {-1, 1}, {1, 10}, {1.5, -5}, {0.1, 0}}
	for _, u := range binarias {
		for _, p := range pares {
			if u.nombre == "Pow" && p[0] < 0 {
				continue
			}
			a, b := u.mio(p[0], p[1]), u.real(p[0], p[1])
			if !dentro(a, b, 1e-12) {
				t.Errorf("%s(%v, %v): mio=%v go=%v", u.nombre, p[0], p[1], a, b)
			}
		}
	}
}

func TestMathMinMaxYLdexpPorBits(t *testing.T) {
	for _, p := range [][2]float64{{3, 7}, {7, 3}, {-3, 7}, {-7, -3}, {0, realmath.Copysign(0, -1)}, {realmath.Copysign(0, -1), 0}} {
		if a, b := mymath.Min(p[0], p[1]), realmath.Min(p[0], p[1]); !mismoPatron(a, b) {
			t.Errorf("Min(%v,%v): mio=%016x go=%016x", p[0], p[1], realmath.Float64bits(a), realmath.Float64bits(b))
		}
		if a, b := mymath.Max(p[0], p[1]), realmath.Max(p[0], p[1]); !mismoPatron(a, b) {
			t.Errorf("Max(%v,%v): mio=%016x go=%016x", p[0], p[1], realmath.Float64bits(a), realmath.Float64bits(b))
		}
	}
	for _, p := range [][2]float64{{1, 10}, {1.5, -5}, {3, 1023}, {0.1, 0}, {1, 0}} {
		n := int(p[1])
		if a, b := mymath.Ldexp(p[0], n), realmath.Ldexp(p[0], n); !mismoPatron(a, b) {
			t.Errorf("Ldexp(%v,%d): mio=%016x go=%016x", p[0], n, realmath.Float64bits(a), realmath.Float64bits(b))
		}
	}
}

func TestMathEspeciales(t *testing.T) {
	if !mymath.IsNaN(mymath.NaN()) || mymath.IsNaN(1) {
		t.Error("IsNaN no distingue NaN")
	}
	if !mymath.IsInf(mymath.Inf(1), 1) || !mymath.IsInf(mymath.Inf(-1), -1) || mymath.IsInf(1, 0) {
		t.Error("IsInf")
	}
	if mymath.Inf(1) != realmath.Inf(1) || mymath.Inf(-1) != realmath.Inf(-1) {
		t.Error("Inf")
	}
	if !mymath.Signbit(realmath.Copysign(0, -1)) || mymath.Signbit(0) || mymath.Signbit(1) || !mymath.Signbit(-1) {
		t.Error("Signbit")
	}
	if a, b := mymath.Copysign(3, realmath.Copysign(0, -1)), realmath.Copysign(3, realmath.Copysign(0, -1)); a != b {
		t.Errorf("Copysign: mio=%v go=%v", a, b)
	}
	if mymath.MaxFloat64 != realmath.MaxFloat64 || mymath.Pi != realmath.Pi || mymath.Sqrt2 != realmath.Sqrt2 ||
		mymath.MaxInt32 != realmath.MaxInt32 || mymath.MaxInt64 != realmath.MaxInt64 || mymath.MaxUint16 != realmath.MaxUint16 {
		t.Error("alguna constante no coincide con la de Go")
	}
}

func TestBits(t *testing.T) {
	for v := 0; v < 256; v++ {
		if a, b := mybits.OnesCount8(uint8(v)), realbits.OnesCount8(uint8(v)); a != b {
			t.Fatalf("OnesCount8(%d): mio=%d go=%d", v, a, b)
		}
	}
	for _, v := range []uint32{0, 1, 0x80000000, 0xdeadbeef, 0xffffffff, 0x12345678} {
		for _, k := range []int{0, 1, 7, 8, 16, 31, 32, 33, -1, -5} {
			if a, b := mybits.RotateLeft32(v, k), realbits.RotateLeft32(v, k); a != b {
				t.Errorf("RotateLeft32(%x, %d): mio=%x go=%x", v, k, a, b)
			}
			if a, b := mybits.RotateRight32(v, k), realbits.RotateLeft32(v, -k); a != b {
				t.Errorf("RotateRight32(%x, %d): mio=%x go=%x", v, k, a, b)
			}
		}
		if a, b := mybits.LeadingZeros32(v), realbits.LeadingZeros32(v); a != b {
			t.Errorf("LeadingZeros32(%x): mio=%d go=%d", v, a, b)
		}
		if a, b := mybits.TrailingZeros32(v), realbits.TrailingZeros32(v); a != b {
			t.Errorf("TrailingZeros32(%x): mio=%d go=%d", v, a, b)
		}
		if a, b := mybits.Len32(v), realbits.Len32(v); a != b {
			t.Errorf("Len32(%x): mio=%d go=%d", v, a, b)
		}
	}
}

func TestHashYHex(t *testing.T) {
	textos := []string{"", "a", "data\\global\\ui\\logo.dc6", "Diablo II", "MPQ", "0x1234567890abcdef",
		"palette\\act1\\pal.pl2", "d2data.mpq", "12345678901234567890", "\x00\x01\x02\xff"}
	bytesSueltos := [][]byte{{}, {0}, {0, 0, 0}, {0xff, 0xff, 0xff, 0xff, 0xff}, {1, 2, 3, 4, 5, 6, 7, 8, 9}}
	for _, s := range append(textos, "") {
		h32r := realfnv.New32a()
		h32r.Write([]byte(s))
		h32m := myfnv.New32a()
		h32m.Write([]byte(s))
		if h32m.Sum32() != h32r.Sum32() {
			t.Errorf("fnv32a(%q): mio=%d go=%d", s, h32m.Sum32(), h32r.Sum32())
		}
		h64r := realfnv.New64a()
		h64r.Write([]byte(s))
		h64m := myfnv.New64a()
		h64m.Write([]byte(s))
		if h64m.Sum64() != h64r.Sum64() {
			t.Errorf("fnv64a(%q): mio=%d go=%d", s, h64m.Sum64(), h64r.Sum64())
		}
		if a, b := myhex.EncodeToString([]byte(s)), realhex.EncodeToString([]byte(s)); a != b {
			t.Errorf("hex(%q): mio=%q go=%q", s, a, b)
		}
	}
	for _, b := range bytesSueltos {
		h32r := realfnv.New32a()
		h32r.Write(b)
		h32m := myfnv.New32a()
		h32m.Write(b)
		if h32m.Sum32() != h32r.Sum32() {
			t.Errorf("fnv32a(%v): mio=%d go=%d", b, h32m.Sum32(), h32r.Sum32())
		}
	}
	// Reset y Size, que el motor usa al reusar un hasher
	h := myfnv.New64a()
	h.Write([]byte("algo"))
	h.Reset()
	h.Write([]byte("otro"))
	hr := realfnv.New64a()
	hr.Write([]byte("otro"))
	if h.Sum64() != hr.Sum64() || h.Size() != hr.Size() || h.BlockSize() != hr.BlockSize() {
		t.Errorf("fnv tras Reset: mio=%d go=%d", h.Sum64(), hr.Sum64())
	}
	if a, b := myhex.EncodeToString([]byte{0xde, 0xad, 0xbe, 0xef}), realhex.EncodeToString([]byte{0xde, 0xad, 0xbe, 0xef}); a != b {
		t.Errorf("hex: mio=%q go=%q", a, b)
	}
	for _, entrada := range []string{"deadbeef", "00", "0a0b0c", "ff00ff"} {
		src, _ := realhex.DecodeString(entrada)
		dst := make([]byte, len(src))
		n, err := myhex.Decode(dst, []byte(entrada))
		if err != nil || n != len(src) {
			t.Errorf("hex.Decode(%q): n=%d err=%v", entrada, n, err)
			continue
		}
		if string(dst) != string(src) {
			t.Errorf("hex.Decode(%q): mio=%x go=%x", entrada, dst, src)
		}
	}
}

func TestBinary(t *testing.T) {
	bloques := [][]byte{{0, 0}, {1, 0}, {0xff, 0xff}, {0x34, 0x12}, {0xef, 0xbe, 0xad, 0xde},
		{0, 0, 0, 0x80}, {0xff, 0xff, 0xff, 0x7f}, {0, 1, 2, 3, 4, 5, 6, 7}, {8, 7, 6, 5, 4, 3, 2, 1},
		{0xef, 0xbe, 0xad, 0xde, 0xef, 0xbe, 0xad, 0xde}}
	for _, b := range bloques {
		if len(b) >= 2 {
			if a, c := mybinary.LittleEndian.Uint16(b), realbinary.LittleEndian.Uint16(b); a != c {
				t.Errorf("Uint16(%v): mio=%d go=%d", b, a, c)
			}
		}
		if len(b) >= 4 {
			if a, c := mybinary.LittleEndian.Uint32(b), realbinary.LittleEndian.Uint32(b); a != c {
				t.Errorf("Uint32(%v): mio=%d go=%d", b, a, c)
			}
		}
		if len(b) >= 8 {
			if a, c := mybinary.LittleEndian.Uint64(b), realbinary.LittleEndian.Uint64(b); a != c {
				t.Errorf("Uint64(%v): mio=%d go=%d", b, a, c)
			}
		}
	}
	// Put y despues leer con Go: el ida y vuelta tiene que dar los mismos bytes
	var mio [8]byte
	var re [8]byte
	mybinary.LittleEndian.PutUint16(mio[:2], 0x1234)
	mybinary.LittleEndian.PutUint32(mio[2:6], 0xdeadbeef)
	mybinary.LittleEndian.PutUint64(mio[:8], 0x0123456789abcdef)
	realbinary.LittleEndian.PutUint16(re[:2], 0x1234)
	realbinary.LittleEndian.PutUint32(re[2:6], 0xdeadbeef)
	realbinary.LittleEndian.PutUint64(re[:8], 0x0123456789abcdef)
	if mio != re {
		t.Errorf("bytes escritos distintos: mio=%x go=%x", mio, re)
	}

	// Read: los mismos tipos que el corpus de la consola
	dato := []byte{0x34, 0x12, 0x78, 0x56, 0xbc, 0x9a, 0xef, 0xbe, 0xde}
	for _, tipo := range []string{"uint16", "int16", "uint32", "int32", "uint64", "int64", "float32", "float64"} {
		var um, ur interface{}
		switch tipo {
		case "uint16":
			um, ur = new(uint16), new(uint16)
		case "int16":
			um, ur = new(int16), new(int16)
		case "uint32":
			um, ur = new(uint32), new(uint32)
		case "int32":
			um, ur = new(int32), new(int32)
		case "uint64":
			um, ur = new(uint64), new(uint64)
		case "int64":
			um, ur = new(int64), new(int64)
		case "float32":
			um, ur = new(float32), new(float32)
		case "float64":
			um, ur = new(float64), new(float64)
		}
		if err := mybinary.Read(mybytes.NewReader(dato), mybinary.LittleEndian, um); err != nil {
			t.Errorf("Read %s: %v", tipo, err)
			continue
		}
		if err := realbinary.Read(realbytes.NewReader(dato), realbinary.LittleEndian, ur); err != nil {
			t.Errorf("Read real %s: %v", tipo, err)
			continue
		}
		if a, b := valorDe(um), valorDe(ur); a != b {
			t.Errorf("Read %s: mio=%v go=%v", tipo, a, b)
		}
	}
	// una estructura NO se lee: tiene que avisar en vez de devolver cualquier cosa
	var est struct{ A, B uint32 }
	if err := mybinary.Read(mybytes.NewReader(dato), mybinary.LittleEndian, &est); err == nil {
		t.Error("Read de una estructura tendria que devolver error (el limite esta declarado)")
	}
}

func valorDe(p interface{}) string {
	switch v := p.(type) {
	case *uint16:
		return realfmt.Sprintf("%d", *v)
	case *int16:
		return realfmt.Sprintf("%d", *v)
	case *uint32:
		return realfmt.Sprintf("%d", *v)
	case *int32:
		return realfmt.Sprintf("%d", *v)
	case *uint64:
		return realfmt.Sprintf("%d", *v)
	case *int64:
		return realfmt.Sprintf("%d", *v)
	case *float32:
		return realfmt.Sprintf("%016x", realmath.Float32bits(*v))
	case *float64:
		return realfmt.Sprintf("%016x", realmath.Float64bits(*v))
	}
	return "?"
}

func TestBytesReader(t *testing.T) {
	dato := []byte("hola mundo, esto es un lector de bytes mas largo")
	mio := mybytes.NewReader(dato)
	real := realbytes.NewReader(dato)
	if mio.Len() != real.Len() || mio.Size() != real.Size() {
		t.Fatalf("Len/Size: mio=%d/%d go=%d/%d", mio.Len(), mio.Size(), real.Len(), real.Size())
	}
	var bm [10]byte
	var br [10]byte
	nm, em := mio.Read(bm[:])
	nr, er := real.Read(br[:])
	if nm != nr || em != er || bm != br {
		t.Errorf("Read: mio=%d/%v/%q go=%d/%v/%q", nm, em, bm, nr, er, br)
	}
	if p, _ := mio.Seek(5, 0); p != 5 {
		t.Errorf("Seek: mio=%d", p)
	}
	real.Seek(5, 0) // la de Go, para que los dos queden en la misma posicion
	var bm2 [4]byte
	var br2 [4]byte
	mio.Read(bm2[:])
	real.Read(br2[:])
	if bm2 != br2 {
		t.Errorf("tras Seek: mio=%q go=%q", bm2, br2)
	}
	// leer mas alla del final devuelve EOF
	mio2 := mybytes.NewReader([]byte("ab"))
	real2 := realbytes.NewReader([]byte("ab"))
	var x [4]byte
	n1, e1 := mio2.Read(x[:])
	n2, e2 := real2.Read(x[:])
	if n1 != n2 || e1 != e2 || n1 != 2 {
		t.Errorf("Read corto: mio=%d/%v go=%d/%v", n1, e1, n2, e2)
	}
	if _, e := mio2.Read(x[:]); e == nil {
		t.Error("Read pasado el final tendria que dar EOF")
	}
}

func TestRing(t *testing.T) {
	mio := myring.New(5)
	real := realring.New(5)
	if mio.Len() != real.Len() {
		t.Fatalf("Len: mio=%d go=%d", mio.Len(), real.Len())
	}
	for i := 0; i < 5; i++ {
		mio.Value = i
		real.Value = i
		mio = mio.Next()
		real = real.Next()
	}
	for _, n := range []int{0, 1, 2, 3, 5, -1, -2, -5, 7} {
		if a, b := mio.Move(n).Value.(int), real.Move(n).Value.(int); a != b {
			t.Errorf("Move(%d).Value: mio=%d go=%d", n, a, b)
		}
	}
	sumaM, sumaR := 0, 0
	mio.Do(func(v interface{}) { sumaM += v.(int) })
	real.Do(func(v interface{}) { sumaR += v.(int) })
	if sumaM != sumaR {
		t.Errorf("Do: mio=%d go=%d", sumaM, sumaR)
	}
	vacio := myring.New(0)
	if vacio != nil || vacio.Len() != 0 {
		t.Error("New(0) tendria que dar nil con Len() 0")
	}
}

// TestRandPropiedades: el generador es propio (la secuencia de Go es un lagged Fibonacci con una
// tabla de datos generada en su dia), asi que aca no se compara con Go: se prueba lo que el motor
// necesita y lo que el limite declarado promete. En la consola el corpus hace la misma prueba.
func TestRandPropiedades(t *testing.T) {
	a := myrand.New(myrand.NewSource(12345))
	b := myrand.New(myrand.NewSource(12345))
	for i := 0; i < 100; i++ {
		if a.Int63() != b.Int63() {
			t.Fatal("la misma semilla tendria que dar la misma secuencia")
		}
	}
	c := myrand.New(myrand.NewSource(54321))
	d := myrand.New(myrand.NewSource(12345))
	distinta := false
	for i := 0; i < 100; i++ {
		if c.Int63() != d.Int63() {
			distinta = true
			break
		}
	}
	if !distinta {
		t.Fatal("semillas distintas tendrian que dar secuencias distintas")
	}
	r := myrand.New(myrand.NewSource(7))
	var visto [10]bool
	for i := 0; i < 1000; i++ {
		v := r.Intn(10)
		if v < 0 || v >= 10 {
			t.Fatalf("Intn(10) dio %d", v)
		}
		visto[v] = true
	}
	cubiertos := 0
	for _, v := range visto {
		if v {
			cubiertos++
		}
	}
	if cubiertos != 10 {
		t.Errorf("Intn(10) no cubrio los 10 valores: %d", cubiertos)
	}
	if r.Intn(1) != 0 {
		t.Error("Intn(1) tendria que dar siempre 0")
	}
	for i := 0; i < 100; i++ {
		if v := r.Intn(256); v < 0 || v >= 256 {
			t.Fatalf("Intn(256) dio %d", v)
		}
	}
	if r.Int() < 0 || r.Int63() < 0 {
		t.Error("los enteros tendrian que ser no negativos")
	}
	_ = r.Uint32()
	// Int() cambia de ancho con la plataforma (31 bits utiles con int de 32): ese caso lo cubre
	// TestRandConAnchoDeInt, que se guarda por ancho y corre en los dos con test-std-host.sh.
	// la funcion global y su Seed
	myrand.Seed(1)
	primero := myrand.Int63()
	myrand.Seed(1)
	if segundo := myrand.Int63(); primero != segundo {
		t.Error("rand.Seed(1) tendria que repetir la secuencia")
	}
	// Float64 en [0,1)
	for i := 0; i < 1000; i++ {
		if f := r.Float64(); f < 0 || f >= 1 {
			t.Fatalf("Float64 dio %v", f)
		}
	}
}

// TestRandConAnchoDeInt: Int() con int de 32 bits (GOARCH=386) tiene que seguir siendo valido. La
// prueba corre en los dos anchos con test-std-host.sh, que es justo el punto.
func TestRandConAnchoDeInt(t *testing.T) {
	if ^uint(0)>>32 == 0 {
		// int de 32 bits: Int() tiene que entrar en 31 bits
		r := myrand.New(myrand.NewSource(1))
		for i := 0; i < 1000; i++ {
			if v := r.Int(); v < 0 || int64(v) >= 1<<31 {
				t.Fatalf("Int() con int de 32 bits dio %d", v)
			}
		}
		return
	}
	t.Log("int de 64 bits: el caso de 31 bits lo cubre la corrida con GOARCH=386")
}

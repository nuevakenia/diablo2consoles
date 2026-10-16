// Prueba diferencial de DC-002.8: los simbolos que se agregaron a la stdlib minima para desbloquear
// paquetes del motor. Cada uno salio de un error de compilacion MEDIDO (scripts/console/build-motor-all.sh
// compilando el motor por capas):
//
//	encoding/binary.Append*  d2networking/d2mp/events.go:89   "undefined field or method 'AppendUint16'"
//	bytes.Buffer.Read        d2core/d2term/terminal_logger.go:22  "missing method 'Read'"
//	image.Uniform            d2common/d2display              "undefined identifier 'image.Uniform'"
//	ebiten.Key* (37 teclas)  d2core/d2input/ebiten           "undefined: ebiten.KeyApostrophe" y 36 mas
//
// Los tres primeros son Go PURO, asi que se comparan contra la biblioteca estandar REAL de Go: los
// bytes, el rastro de las lecturas (cuanto y con que error) y los pixeles que quedan despues de
// dibujar. Las teclas se prueban por otra via (son constantes, no hay comportamiento): en
// console/dc/engine/ebiten/v2 hay una prueba del paquete entero y el conteo esta en la evidencia.
//
// go test ./... dentro de console/dc/std
package difftest

import (
	realbinary "encoding/binary"
	realbytes "bytes"
	realimage "image"
	realcolor "image/color"
	realdraw "image/draw"
	realstrings "strings"
	realstrconv "strconv"
	"testing"

	mybinary "d2std/encoding/binary"
	myimage "d2std/image"
	mycolor "d2std/image/color"
	mydraw "d2std/image/draw"
	mybytes "d2std/bytes"
)

// textoDeError: el texto del error, para comparar el de Go contra el nuestro. nil es "sin error" (no
// vacio: un error nil y un error con texto vacio no son lo mismo, y asi se ve en el rastro).
func textoDeError(err error) string {
	if err == nil {
		return "sin error"
	}
	return err.Error()
}

// TestBinaryAppendContraGo: AppendByte/AppendUint16/AppendUint32/AppendUint64, en los DOS ordenes, con
// varios prefijos y los valores borde. Se comparan los bytes que quedan, no los valores.
func TestBinaryAppendContraGo(t *testing.T) {
	prefijos := [][]byte{nil, {}, {0xaa}, {1, 2, 3, 4, 5, 6, 7, 8}}
	vals16 := []uint16{0, 1, 0x00ff, 0x1234, 0x8000, 0xffff}
	vals32 := []uint32{0, 1, 0x00ff, 0x12345678, 0x80000000, 0xffffffff}
	vals64 := []uint64{0, 1, 0x00ff, 0x123456789abcdef0, 0x8000000000000000, 0xffffffffffffffff}

	casos := 0
	compara := func(que string, quiero, tengo []byte) {
		casos++
		if !realbytes.Equal(quiero, tengo) {
			t.Errorf("%s: Go [% x], nuestro [% x]", que, quiero, tengo)
		}
	}
	// copia del prefijo, para que los dos lados escriban sobre el mismo estado inicial
	copia := func(pre []byte) []byte { return append([]byte(nil), pre...) }

	for _, pre := range prefijos {
		for _, v := range vals16 {
			compara("AppendUint16", realbinary.LittleEndian.AppendUint16(copia(pre), v), mybinary.LittleEndian.AppendUint16(copia(pre), v))
			compara("AppendUint16 (Big)", realbinary.BigEndian.AppendUint16(copia(pre), v), mybinary.BigEndian.AppendUint16(copia(pre), v))
		}
		for _, v := range vals32 {
			compara("AppendUint32", realbinary.LittleEndian.AppendUint32(copia(pre), v), mybinary.LittleEndian.AppendUint32(copia(pre), v))
			compara("AppendUint32 (Big)", realbinary.BigEndian.AppendUint32(copia(pre), v), mybinary.BigEndian.AppendUint32(copia(pre), v))
		}
		for _, v := range vals64 {
			compara("AppendUint64", realbinary.LittleEndian.AppendUint64(copia(pre), v), mybinary.LittleEndian.AppendUint64(copia(pre), v))
			compara("AppendUint64 (Big)", realbinary.BigEndian.AppendUint64(copia(pre), v), mybinary.BigEndian.AppendUint64(copia(pre), v))
		}
		// el encadenado es lo que hace el motor en d2mp/events.go: dos Append seguidos sobre el mismo
		// slice, sin variable intermedia
		quiero := realbinary.LittleEndian.AppendUint16(realbinary.LittleEndian.AppendUint32(copia(pre), 0x00112233), 0x4455)
		tengo := mybinary.LittleEndian.AppendUint16(mybinary.LittleEndian.AppendUint32(copia(pre), 0x00112233), 0x4455)
		compara("encadenado Uint32+Uint16", quiero, tengo)
	}
	if casos != 4*(6*2+6*2+6*2+1) {
		t.Errorf("casos: se corrieron %d, se esperaban %d", casos, 4*(6*2+6*2+6*2+1))
	}
}

// TestBytesBufferReadContraGo: Buffer.Read lee y CONSUME, y al final devuelve io.EOF. Se compara el
// rastro entero (cuantos bytes y con que error en cada lectura), no solo el texto: un Read que devuelve
// los bytes y ademas un error, o que no avanza, se ve en el rastro.
func TestBytesBufferReadContraGo(t *testing.T) {
	texto := "hola mundo: 0123456789 abcdefghijklmnopqrstuvwxyz ABCDEFGHIJKLMNOPQRSTUVWXYZ"

	// rastroGo y rastroNuestro son el mismo bucle escrito dos veces (los tipos de Go y los de d2std no
	// se pueden mezclar): 24 lecturas de tam bytes, o hasta la primera que devuelva 0.
	rastroGo := func(tam int) string {
		b := realbytes.NewBufferString(texto)
		var sb realstrings.Builder
		chico := make([]byte, tam)
		for i := 0; i < 24; i++ {
			n, err := b.Read(chico)
			sb.WriteString(realstrings.Join([]string{realstrconv.Itoa(n), textoDeError(err)}, ":") + "|")
			if n == 0 {
				break
			}
		}
		return sb.String()
	}
	rastroNuestro := func(tam int) string {
		b := mybytes.NewBufferString(texto)
		var sb realstrings.Builder
		chico := make([]byte, tam)
		for i := 0; i < 24; i++ {
			n, err := b.Read(chico)
			sb.WriteString(realstrings.Join([]string{realstrconv.Itoa(n), textoDeError(err)}, ":") + "|")
			if n == 0 {
				break
			}
		}
		return sb.String()
	}

	for _, tam := range []int{1, 2, 3, 7, 16, 64, 200, 0} {
		quiero, tengo := rastroGo(tam), rastroNuestro(tam)
		if quiero != tengo {
			t.Errorf("pedazos de %d bytes:\n  Go      %s\n  nuestro %s", tam, quiero, tengo)
		}
	}

	// el texto leido de a pedazos tiene que ser el mismo (y el buffer tiene que quedar vacio)
	entero := func() (string, int) {
		b := realbytes.NewBufferString(texto)
		chico := make([]byte, 5)
		var sb realstrings.Builder
		for {
			n, err := b.Read(chico)
			sb.Write(chico[:n])
			if err != nil {
				break
			}
		}
		return sb.String(), b.Len()
	}
	enteroNuestro := func() (string, int) {
		b := mybytes.NewBufferString(texto)
		chico := make([]byte, 5)
		var sb realstrings.Builder
		for {
			n, err := b.Read(chico)
			sb.Write(chico[:n])
			if err != nil {
				break
			}
		}
		return sb.String(), b.Len()
	}
	if q, ql := entero(); true {
		gg, gl := enteroNuestro()
		if q != gg || ql != gl {
			t.Errorf("texto leido: Go %q (%d bytes restantes), nuestro %q (%d)", q, ql, gg, gl)
		}
	}

	// Bytes() despues de leer devuelve lo que falta (como en Go), y Write despues de leer vacio vuelve a
	// escribir desde el principio
	bGo, bMi := realbytes.NewBufferString(texto), mybytes.NewBufferString(texto)
	real3, mi3 := make([]byte, 10), make([]byte, 10)
	nGo, _ := bGo.Read(real3)
	nMi, _ := bMi.Read(mi3)
	if nGo != nMi || !realbytes.Equal(real3[:nGo], mi3[:nMi]) {
		t.Errorf("los primeros %d bytes: Go [% x], nuestro [% x]", nGo, real3[:nGo], mi3[:nMi])
	}
	if realbytes.Compare(bGo.Bytes(), bMi.Bytes()) != 0 {
		t.Errorf("lo que queda: Go [% x], nuestro [% x]", bGo.Bytes(), bMi.Bytes())
	}
	bGo.WriteString("X")
	bMi.WriteString("X")
	if bGo.String() != bMi.String() {
		t.Errorf("despues de escribir: Go %q, nuestro %q", bGo.String(), bMi.String())
	}

	// Reset tiene que dejar el buffer como nuevo tambien despues de haber leido
	bGo.Reset()
	bMi.Reset()
	if bGo.Len() != bMi.Len() || bGo.String() != bMi.String() {
		t.Errorf("despues de Reset: Go %q, nuestro %q", bGo.String(), bMi.String())
	}
}

// TestImageUniformContraGo: el color uniforme (At en cualquier punto, Opaque segun el alfa) y lo que
// importa de verdad: dibujarlo con image/draw sobre un RGBA y que queden los mismos pixeles.
func TestImageUniformContraGo(t *testing.T) {
	casos := []struct {
		nombre string
		goC    realcolor.Color
		miC    mycolor.Color
	}{
		{"RGBA opaco", realcolor.RGBA{1, 2, 3, 255}, mycolor.RGBA{1, 2, 3, 255}},
		{"RGBA semitransparente", realcolor.RGBA{1, 2, 3, 128}, mycolor.RGBA{1, 2, 3, 128}},
		{"RGBA transparente", realcolor.RGBA{1, 2, 3, 0}, mycolor.RGBA{1, 2, 3, 0}},
		{"Gray16 negro (el color.Black de la consola)", realcolor.Gray16{0}, mycolor.Gray16{0}},
		{"Gray16 blanco", realcolor.Gray16{0xffff}, mycolor.Gray16{0xffff}},
	}
	// los puntos de prueba: el origen, un punto lejano y los dos bordes del rectangulo de Go
	puntos := [][2]int{{0, 0}, {7, 11}, {-1000000, 1000000}, {1000000000, 1000000000}}

	for _, c := range casos {
		uGo := &realimage.Uniform{C: c.goC}
		uMi := &myimage.Uniform{C: c.miC}

		for _, p := range puntos {
			r1, g1, b1, a1 := uGo.At(p[0], p[1]).RGBA()
			r2, g2, b2, a2 := uMi.At(p[0], p[1]).RGBA()
			if r1 != r2 || g1 != g2 || b1 != b2 || a1 != a2 {
				t.Errorf("%s en (%d,%d): Go rgba(%d,%d,%d,%d), nuestro rgba(%d,%d,%d,%d)",
					c.nombre, p[0], p[1], r1, g1, b1, a1, r2, g2, b2, a2)
			}
		}
		if uGo.Opaque() != uMi.Opaque() {
			t.Errorf("%s: Opaque Go %v, nuestro %v", c.nombre, uGo.Opaque(), uMi.Opaque())
		}
		// los limites: Go y nosotros usamos el mismo cuadrado enorme
		bGo, bMi := uGo.Bounds(), uMi.Bounds()
		if bGo.Min.X != bMi.Min.X || bGo.Min.Y != bMi.Min.Y || bGo.Max.X != bMi.Max.X || bGo.Max.Y != bMi.Max.Y {
			t.Errorf("%s: Bounds Go %v, nuestro %v", c.nombre, bGo, bMi)
		}
		// y el color de la imagen es el color (ColorModel tiene que devolverlo)
		c1 := uGo.ColorModel().Convert(realcolor.RGBA{9, 9, 9, 9})
		c2 := uMi.ColorModel().Convert(mycolor.RGBA{9, 9, 9, 9})
		r1, g1, b1, a1 := c1.RGBA()
		r2, g2, b2, a2 := c2.RGBA()
		if r1 != r2 || g1 != g2 || b1 != b2 || a1 != a2 {
			t.Errorf("%s: ColorModel().Convert(: Go rgba(%d,%d,%d,%d), nuestro rgba(%d,%d,%d,%d)",
				c.nombre, r1, g1, b1, a1, r2, g2, b2, a2)
		}
	}

	// el caso del motor (d2common/d2display): fondo negro uniforme sobre el cuadro entero, con Src y con
	// Over, y el mismo fondo parcial (un rectangulo adentro). Se comparan los PIXELES.
	rects := []realimage.Rectangle{
		realimage.Rect(0, 0, 8, 4),
		realimage.Rect(0, 0, 8, 4),
		realimage.Rect(2, 1, 6, 3),
	}
	ops := []realdraw.Op{realdraw.Src, realdraw.Over, realdraw.Src}
	nombres := []string{"Src sobre todo", "Over sobre todo", "Src en un rectangulo adentro"}

	for i := range rects {
		goDst := realimage.NewRGBA(realimage.Rect(0, 0, 8, 4))
		miDst := myimage.NewRGBA(myimage.Rect(0, 0, 8, 4))
		// el fondo del destino NO es negro: si el dibujo no hace nada, se ve igual
		for j := 0; j < len(goDst.Pix); j++ {
			goDst.Pix[j] = 0x40
		}
		for j := 0; j < len(miDst.Pix); j++ {
			miDst.Pix[j] = 0x40
		}
		rGo := realimage.Rect(rects[i].Min.X, rects[i].Min.Y, rects[i].Max.X, rects[i].Max.Y)
		rMi := myimage.Rect(rects[i].Min.X, rects[i].Min.Y, rects[i].Max.X, rects[i].Max.Y)
		realdraw.Draw(goDst, rGo, &realimage.Uniform{C: realcolor.RGBA{0x11, 0x22, 0x33, 0xff}}, realimage.Point{}, ops[i])
		mydraw.Draw(miDst, rMi, &myimage.Uniform{C: mycolor.RGBA{0x11, 0x22, 0x33, 0xff}}, myimage.Point{}, mydraw.Op(ops[i]))
		if !realbytes.Equal(goDst.Pix, miDst.Pix) {
			t.Errorf("dibujar Uniform (%s):\n  Go      [% x]\n  nuestro [% x]", nombres[i], goDst.Pix, miDst.Pix)
		}
	}

	// y con un color semitransparente, que es donde Over mezcla de verdad
	goDst := realimage.NewRGBA(realimage.Rect(0, 0, 4, 2))
	miDst := myimage.NewRGBA(myimage.Rect(0, 0, 4, 2))
	for j := 0; j < len(goDst.Pix); j++ {
		goDst.Pix[j] = 0xc0
		miDst.Pix[j] = 0xc0
	}
	realdraw.Draw(goDst, goDst.Bounds(), &realimage.Uniform{C: realcolor.RGBA{0x11, 0x22, 0x33, 0x80}}, realimage.Point{}, realdraw.Over)
	mydraw.Draw(miDst, miDst.Bounds(), &myimage.Uniform{C: mycolor.RGBA{0x11, 0x22, 0x33, 0x80}}, myimage.Point{}, mydraw.Over)
	if !realbytes.Equal(goDst.Pix, miDst.Pix) {
		t.Errorf("Over con alfa 0x80:\n  Go      [% x]\n  nuestro [% x]", goDst.Pix, miDst.Pix)
	}
}

// TestColorTodosContraGo: los OCHO colores de la stdlib contra Go, con alfa 255, alfa medio y alfa 0.
//
// Esta es la prueba que FALTABA. El defecto de color.RGBA (premultiplicaba por el alfa, que es lo que
// hace NRGBA) vivio sin que nadie lo viera porque ni el difftest ni el corpus de la consola tocaban
// color.RGBA.RGBA(): lo cazo la prueba de image.Uniform dibujado (mas arriba) porque el dibujado de
// Go usa RGBA(). Los 16 bits del resultado son el contrato: aca se comparan uno por uno.
func TestColorTodosContraGo(t *testing.T) {
	casos := []struct {
		nombre string
		goC    realcolor.Color
		miC    mycolor.Color
	}{
		{"RGBA alfa 255", realcolor.RGBA{1, 2, 3, 255}, mycolor.RGBA{1, 2, 3, 255}},
		{"RGBA alfa 128", realcolor.RGBA{1, 2, 3, 128}, mycolor.RGBA{1, 2, 3, 128}},
		{"RGBA alfa 0", realcolor.RGBA{1, 2, 3, 0}, mycolor.RGBA{1, 2, 3, 0}},
		{"RGBA con todo al tope", realcolor.RGBA{255, 128, 64, 192}, mycolor.RGBA{255, 128, 64, 192}},
		{"RGBA negro transparente", realcolor.RGBA{0, 0, 0, 0}, mycolor.RGBA{0, 0, 0, 0}},
		{"NRGBA alfa 255", realcolor.NRGBA{1, 2, 3, 255}, mycolor.NRGBA{1, 2, 3, 255}},
		{"NRGBA alfa 128", realcolor.NRGBA{1, 2, 3, 128}, mycolor.NRGBA{1, 2, 3, 128}},
		{"NRGBA alfa 0", realcolor.NRGBA{1, 2, 3, 0}, mycolor.NRGBA{1, 2, 3, 0}},
		{"NRGBA con todo al tope", realcolor.NRGBA{255, 128, 64, 192}, mycolor.NRGBA{255, 128, 64, 192}},
		{"RGBA64 alfa 65535", realcolor.RGBA64{1, 2, 3, 65535}, mycolor.RGBA64{1, 2, 3, 65535}},
		{"RGBA64 alfa 32768", realcolor.RGBA64{1, 2, 3, 32768}, mycolor.RGBA64{1, 2, 3, 32768}},
		{"RGBA64 alfa 0", realcolor.RGBA64{1, 2, 3, 0}, mycolor.RGBA64{1, 2, 3, 0}},
		{"RGBA64 con todo al tope", realcolor.RGBA64{65535, 32768, 16384, 49152}, mycolor.RGBA64{65535, 32768, 16384, 49152}},
		{"NRGBA64 alfa 65535", realcolor.NRGBA64{1, 2, 3, 65535}, mycolor.NRGBA64{1, 2, 3, 65535}},
		{"NRGBA64 alfa 32768", realcolor.NRGBA64{1, 2, 3, 32768}, mycolor.NRGBA64{1, 2, 3, 32768}},
		{"NRGBA64 alfa 0", realcolor.NRGBA64{1, 2, 3, 0}, mycolor.NRGBA64{1, 2, 3, 0}},
		{"Gray 0", realcolor.Gray{0}, mycolor.Gray{0}},
		{"Gray 0x7f", realcolor.Gray{0x7f}, mycolor.Gray{0x7f}},
		{"Gray 0xff", realcolor.Gray{0xff}, mycolor.Gray{0xff}},
		{"Gray16 0x1234", realcolor.Gray16{0x1234}, mycolor.Gray16{0x1234}},
		{"Gray16 0xffff", realcolor.Gray16{0xffff}, mycolor.Gray16{0xffff}},
		{"Alpha 0", realcolor.Alpha{0}, mycolor.Alpha{0}},
		{"Alpha 0x7f", realcolor.Alpha{0x7f}, mycolor.Alpha{0x7f}},
		{"Alpha 0xff", realcolor.Alpha{0xff}, mycolor.Alpha{0xff}},
		{"Alpha16 0x1234", realcolor.Alpha16{0x1234}, mycolor.Alpha16{0x1234}},
		{"Alpha16 0xffff", realcolor.Alpha16{0xffff}, mycolor.Alpha16{0xffff}},
	}

	for _, c := range casos {
		r1, g1, b1, a1 := c.goC.RGBA()
		r2, g2, b2, a2 := c.miC.RGBA()
		if r1 != r2 || g1 != g2 || b1 != b2 || a1 != a2 {
			t.Errorf("%s: Go (%d,%d,%d,%d), nuestro (%d,%d,%d,%d)", c.nombre, r1, g1, b1, a1, r2, g2, b2, a2)
		}
	}

	// Rectangle.Union e Inset: los dos que pide d2common/d2display. Los rectangulos de prueba incluyen
	// los vacios y los que se tocan en un borde, que es donde Union tiene su regla propia.
	rects := []realimage.Rectangle{
		realimage.Rect(0, 0, 4, 4),
		realimage.Rect(2, 2, 6, 6),
		realimage.Rect(-3, -3, -1, -1),
		realimage.Rect(4, 0, 4, 4),
		realimage.Rect(0, 0, 0, 0),
		realimage.Rect(10, 10, 20, 30),
		realimage.Rect(-3, -3, 0, 0),   // la suma de los bordes es impar: el medio trunca hacia CERO
		realimage.Rect(0, 0, 3, 3),     // idem, del lado positivo
		realimage.Rect(-3, -2, -3, -2), // degenerado en los dos ejes
	}
	for _, a := range rects {
		for _, b := range rects {
			uGo := a.Union(b)
			uMi := myimage.Rect(a.Min.X, a.Min.Y, a.Max.X, a.Max.Y).Union(myimage.Rect(b.Min.X, b.Min.Y, b.Max.X, b.Max.Y))
			if uGo.Min.X != uMi.Min.X || uGo.Min.Y != uMi.Min.Y || uGo.Max.X != uMi.Max.X || uGo.Max.Y != uMi.Max.Y {
				t.Errorf("Union(%v,%v): Go %v, nuestro %v", a, b, uGo, uMi)
			}
			// In: la contencion de rectangulos (el motor la usa en d2display/shotcheck.go)
			inGo := a.In(b)
			inMi := myimage.Rect(a.Min.X, a.Min.Y, a.Max.X, a.Max.Y).In(myimage.Rect(b.Min.X, b.Min.Y, b.Max.X, b.Max.Y))
			if inGo != inMi {
				t.Errorf("In(%v,%v): Go %v, nuestro %v", a, b, inGo, inMi)
			}
		}
		for _, n := range []int{-2, -1, 0, 1, 3} {
			iGo := a.Inset(n)
			iMi := myimage.Rect(a.Min.X, a.Min.Y, a.Max.X, a.Max.Y).Inset(n)
			if iGo.Min.X != iMi.Min.X || iGo.Min.Y != iMi.Min.Y || iGo.Max.X != iMi.Max.X || iGo.Max.Y != iMi.Max.Y {
				t.Errorf("Inset(%v,%d): Go %v, nuestro %v", a, n, iGo, iMi)
			}
		}
	}

	// y los dos modelos de 8 bits que arma el motor (RGBAModel y NRGBAModel), que es lo que convierte
	// un color a la imagen de trabajo
	{
		conv := []realcolor.Color{realcolor.RGBA{1, 2, 3, 128}, realcolor.NRGBA{1, 2, 3, 128}, realcolor.Gray{0x40}}
		for _, c := range conv {
			c2 := realcolor.RGBAModel.Convert(c)
			r1, g1, b1, a1 := c2.RGBA()
			var mi2 mycolor.Color
			switch v := c.(type) {
			case realcolor.RGBA:
				mi2 = mycolor.RGBAModel.Convert(mycolor.RGBA{v.R, v.G, v.B, v.A})
			case realcolor.NRGBA:
				mi2 = mycolor.RGBAModel.Convert(mycolor.NRGBA{v.R, v.G, v.B, v.A})
			case realcolor.Gray:
				mi2 = mycolor.RGBAModel.Convert(mycolor.Gray{v.Y})
			}
			r2, g2, b2, a2 := mi2.RGBA()
			if r1 != r2 || g1 != g2 || b1 != b2 || a1 != a2 {
				t.Errorf("RGBAModel.Convert(%T{%v}): Go (%d,%d,%d,%d), nuestro (%d,%d,%d,%d)", c, c, r1, g1, b1, a1, r2, g2, b2, a2)
			}
		}
	}
}

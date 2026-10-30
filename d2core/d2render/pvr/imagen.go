//go:build dreamcast

package pvr

import (
	"image/color"
	"math"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2enum"
)

// Imagen es el lienzo donde la Surface compone el cuadro: RGBA de 8 bits por canal, en el monton.
//
// POR QUE EN MEMORIA Y NO SOLO EN LA VRAM: el contrato de d2interface.Surface incluye Screenshot,
// ReplacePixels y RenderSection, que son la unica forma de comparar pixeles. Con el cuadro viviendo solo en
// la VRAM habria que leerla de vuelta por el bus del PVR en cada comparacion. Se compone en RAM y se sube
// UNA vez por cuadro (medido: la pantalla 640x480x4 = 1.228.800 B entra en el monton despues del recorte
// de DC-002.68; un bloque de 1.835.008 B ya no).
type Imagen struct {
	pixeles []byte
	ancho   int
	alto    int
}

// NuevaImagen: un lienzo negro del tamano pedido.
func NuevaImagen(ancho, alto int) *Imagen {
	if ancho <= 0 {
		ancho = 1
	}

	if alto <= 0 {
		alto = 1
	}

	return &Imagen{pixeles: make([]byte, ancho*alto*4), ancho: ancho, alto: alto}
}

// Ancho y Alto del lienzo.
func (i *Imagen) Ancho() int { return i.ancho }

// Alto del lienzo.
func (i *Imagen) Alto() int { return i.alto }

// Pixeles: el buffer RGBA crudo (es el que se le pasa al C para subirlo al PVR, y el que devuelve
// ReplacePixels/Screenshot en forma de copia).
func (i *Imagen) Pixeles() []byte { return i.pixeles }

// Rellenar pinta el lienzo entero de un color.
func (i *Imagen) Rellenar(c color.RGBA) {
	for j := 0; j+3 < len(i.pixeles); j += 4 {
		i.pixeles[j+0] = c.R
		i.pixeles[j+1] = c.G
		i.pixeles[j+2] = c.B
		i.pixeles[j+3] = c.A
	}
}

// Poner escribe un pixel (sin mezclar). Fuera del lienzo no hace nada.
func (i *Imagen) Poner(x, y int, c color.RGBA) {
	if x < 0 || y < 0 || x >= i.ancho || y >= i.alto {
		return
	}

	j := (y*i.ancho + x) * 4
	i.pixeles[j+0] = c.R
	i.pixeles[j+1] = c.G
	i.pixeles[j+2] = c.B
	i.pixeles[j+3] = c.A
}

// Tomar lee un pixel (0,0,0,0 fuera del lienzo).
func (i *Imagen) Tomar(x, y int) color.RGBA {
	if x < 0 || y < 0 || x >= i.ancho || y >= i.alto {
		return color.RGBA{}
	}

	j := (y*i.ancho + x) * 4

	return color.RGBA{i.pixeles[j+0], i.pixeles[j+1], i.pixeles[j+2], i.pixeles[j+3]}
}

// Rectangulo pinta un rectangulo lleno, con la esquina en (x, y).
func (i *Imagen) Rectangulo(x, y, ancho, alto int, c color.RGBA) {
	for fy := y; fy < y+alto; fy++ {
		for fx := x; fx < x+ancho; fx++ {
			i.Poner(fx, fy, c)
		}
	}
}

// Linea pinta la linea de (x0, y0) a (x1, y1) con el algoritmo de Bresenham (el mismo que usa
// ebitenutil.DrawLine en la PC).
func (i *Imagen) Linea(x0, y0, x1, y1 int, c color.RGBA) {
	dx := abs(x1 - x0)
	sx := -1

	if x0 < x1 {
		sx = 1
	}

	dy := -abs(y1 - y0)
	sy := -1

	if y0 < y1 {
		sy = 1
	}

	err := dx + dy

	for {
		i.Poner(x0, y0, c)

		if x0 == x1 && y0 == y1 {
			return
		}

		e2 := 2 * err
		if e2 >= dy {
			err += dy
			x0 += sx
		}

		if e2 <= dx {
			err += dx
			y0 += sy
		}
	}
}

func abs(v int) int {
	if v < 0 {
		return -v
	}

	return v
}

// ============================================================ la geometria

// matriz es la transformacion de un dibujado: la misma convencion que ebiten.GeoM, que es la que el motor
// arma con PushTranslation/PushScale/PushSkew:
//
//	x' = a*x + c*y + tx
//	y' = b*x + d*y + ty
type matriz struct {
	a, b, c, d float64
	tx, ty     float64
}

// identidad: no transforma nada.
func identidad() matriz { return matriz{a: 1, d: 1} }

// aplicar lleva un punto del origen al destino.
func (m matriz) aplicar(x, y float64) (float64, float64) {
	return m.a*x + m.c*y + m.tx, m.b*x + m.d*y + m.ty
}

// escalar agrega una escala (como GeoM.Scale).
func (m matriz) escalar(sx, sy float64) matriz {
	m.a *= sx
	m.b *= sx
	m.c *= sy
	m.d *= sy

	return m
}

// sesgar agrega un sesgo (como GeoM.Skew).
func (m matriz) sesgar(sx, sy float64) matriz {
	m.c += math.Tan(sx) * m.a
	m.d += math.Tan(sx) * m.b
	m.a += math.Tan(sy) * m.c
	m.b += math.Tan(sy) * m.d

	return m
}

// trasladar agrega una traslacion (como GeoM.Translate).
func (m matriz) trasladar(x, y float64) matriz {
	m.tx += m.a*x + m.c*y
	m.ty += m.b*x + m.d*y

	return m
}

// invertir devuelve la matriz inversa (la que se recorre pixel a pixel al dibujar).
func (m matriz) invertir() (matriz, bool) {
	det := m.a*m.d - m.b*m.c
	if det == 0 || math.IsInf(det, 0) || math.IsNaN(det) {
		return matriz{}, false
	}

	inv := 1 / det
	var r matriz

	r.a = m.d * inv
	r.c = -m.c * inv
	r.b = -m.b * inv
	r.d = m.a * inv
	r.tx = -(r.a*m.tx + r.c*m.ty)
	r.ty = -(r.b*m.tx + r.d*m.ty)

	return r, true
}

// esSoloTraslacion: el caso del motor casi siempre (poner un sprite en una posicion y nada mas).
func (m matriz) esSoloTraslacion() bool {
	return m.a == 1 && m.b == 0 && m.c == 0 && m.d == 1
}

// ============================================================ el mezclado

// modoMezcla: como se combina el pixel que llega con el que ya estaba. Son los CompositeMode de
// Ebitengine que el motor usa (ver d2core/d2render/ebiten/ebiten_surface.go:246).
type modoMezcla int

const (
	// mezclaEncima: el de siempre (SourceOver), respetando la transparencia.
	mezclaEncima modoMezcla = iota
	// mezclaCopiar: reemplaza, incluso el alfa (CompositeModeCopy).
	mezclaCopiar
	// mezclaSumar: suma saturada (CompositeModeLighter: la luz y el fuego).
	mezclaSumar
	// mezclaMultiplicar: producto sobre 255 (CompositeModeMultiply: el Burn del PL2).
	mezclaMultiplicar
)

// ajuste es lo que se le hace al pixel de ORIGEN antes de mezclarlo: el tinte de PushColor, el brillo y la
// saturacion, y el efecto del PL2. Todo junto porque las tres cosas se aplican al mismo pixel, y hacerlas
// en tres pasadas seria recorrer el sprite tres veces.
type ajuste struct {
	color      color.RGBA
	tieneColor bool
	efecto     d2enum.DrawEffect
	brillo     float64
	saturacion float64
}

// neutro: no toca el pixel (el caso normal cuando nadie apreto Push*).
func (a ajuste) neutro() bool {
	return !a.tieneColor && a.brillo == 1 && a.saturacion == 1 &&
		(a.efecto == d2enum.DrawEffectNone || a.efecto == d2enum.DrawEffectNormal)
}

// modo: el modo de mezcla que pide el efecto (la parte del efecto que no toca el color).
func (a ajuste) modo() modoMezcla {
	switch a.efecto {
	case d2enum.DrawEffectModulate:
		return mezclaSumar
	case d2enum.DrawEffectBurn:
		return mezclaMultiplicar
	}

	return mezclaEncima
}

// aplicar: el pixel ya transformado, listo para mezclar.
func (a ajuste) aplicar(r, g, b, al uint8) (uint8, uint8, uint8, uint8) {
	if al == 0 {
		return 0, 0, 0, 0
	}

	// 1) el efecto del PL2 que toca el color
	switch a.efecto {
	case d2enum.DrawEffectPctTransparency25:
		al = restarAlfa(al, 0.25)
	case d2enum.DrawEffectPctTransparency50:
		al = restarAlfa(al, 0.50)
	case d2enum.DrawEffectPctTransparency75:
		al = restarAlfa(al, 0.75)
	case d2enum.DrawEffectMod2X:
		r = escalarTope(r, d2enum.Mod2XBrightness)
		g = escalarTope(g, d2enum.Mod2XBrightness)
		b = escalarTope(b, d2enum.Mod2XBrightness)
	case d2enum.DrawEffectMod2XTrans:
		// la tabla G del PL2: el peso del pixel sobre lo de atras sube con su propio brillo
		luz := (d2enum.Mode6LumaR*float64(r) + d2enum.Mode6LumaG*float64(g) +
			d2enum.Mode6LumaB*float64(b)) * d2enum.Mode6Gain
		al = tope8(luz)
	}

	// 2) brillo y saturacion (HSV, como el ColorM.ChangeHSV(0, s, v) de Ebitengine; solo si se pidieron)
	if a.brillo != 1 || a.saturacion != 1 {
		r, g, b = hsv(r, g, b, a.saturacion, a.brillo)
	}

	// 3) el tinte de PushColor
	if a.tieneColor && a.color.A != 0 {
		ca := float64(a.color.A)
		r = escalarTope(r, float64(a.color.R)/ca)
		g = escalarTope(g, float64(a.color.G)/ca)
		b = escalarTope(b, float64(a.color.B)/ca)
		al = escalarTope(al, ca/255)
	}

	return r, g, b, al
}

func restarAlfa(al uint8, cuanto float64) uint8 {
	v := float64(al) - cuanto*255
	if v < 0 {
		return 0
	}

	return uint8(v)
}

func escalarTope(v uint8, f float64) uint8 { return tope8(float64(v) * f) }

func tope8(f float64) uint8 {
	if f <= 0 {
		return 0
	}

	if f >= 255 {
		return 255
	}

	return uint8(f + 0.5)
}

// hsv aplica saturacion y brillo, que es lo que hace Ebitengine con ChangeHSV(elTono, saturacion, brillo)
// cuando el tono no se toca: se conserva el tono (o sea, cual canal es el maximo y la proporcion entre
// ellos) y se escalan la saturacion y el valor.
//
// LA CUENTA, SIN RECONSTRUIR EL TONO (que es lo que la haria lenta y con redondeos): el color se escribe
// como `v - c + c*t`, donde `t` es la posicion del canal entre el minimo (0) y el maximo (1). Con la
// saturacion y el valor SIN tocar, esa formula devuelve exactamente el color original; con `c = v*s`
// escalado, da el mismo tono con otra saturacion. Es la identidad `rgb = v*(1-s) + v*s*t`.
//
// Se usa float32 porque el SH-4 solo hace punto flotante de 32 bits de verdad: un float64 lo emula una
// biblioteca por software (medido en DC-002.22: un sprite de 256x256 costaba 260 ms por cuadro con double
// en el bucle de dibujado).
func hsv(r, g, b uint8, saturacion, brillo float64) (uint8, uint8, uint8) {
	rf, gf, bf := float32(r)/255, float32(g)/255, float32(b)/255

	maxv := rf
	if gf > maxv {
		maxv = gf
	}

	if bf > maxv {
		maxv = bf
	}

	minv := rf
	if gf < minv {
		minv = gf
	}

	if bf < minv {
		minv = bf
	}

	if maxv <= 0 {
		return 0, 0, 0
	}

	s := 0.0
	if maxv > minv {
		s = float64(maxv-minv) / float64(maxv)
	}

	s *= saturacion
	v := float64(maxv) * brillo

	if s < 0 {
		s = 0
	}

	if s > 1 {
		s = 1
	}

	if v < 0 {
		v = 0
	}

	if v > 1 {
		v = 1
	}

	c := v * s
	rango := float64(maxv - minv)

	return canalHSV(rf, minv, rango, v, c),
		canalHSV(gf, minv, rango, v, c),
		canalHSV(bf, minv, rango, v, c)
}

// canalHSV: un canal, con su posicion `t` dentro del rango del color (ver hsv).
func canalHSV(x, minv float32, rango, v, c float64) uint8 {
	t := 0.0
	if rango > 0 {
		t = float64(x-minv) / rango
	}

	return tope8((v - c + c*t) * 255)
}

// ============================================================ dibujar una imagen sobre otra

// sinForzar: el modo de mezcla lo decide el efecto (lo normal).
const sinForzar modoMezcla = -1

// copiarEn pinta `origen` sobre `dst` con la transformacion `m`. Recorre los pixeles del DESTINO y
// pregunta, por cada uno, de donde venia en el origen: es la misma decision que tomó el shim (dibujo.go:15)
// y por la misma razon -- al reves (recorrer el origen) quedan huecos cuando la figura se agranda.
func copiarEn(dst, origen *Imagen, m matriz, aj ajuste, forzado modoMezcla) {
	if dst == nil || origen == nil || len(dst.pixeles) == 0 || len(origen.pixeles) == 0 {
		return
	}

	modo := aj.modo()
	if forzado >= 0 {
		modo = forzado
	}

	// EL CASO RAPIDO: solo mover. No hay cuenta por pixel (es lo que el motor hace casi siempre).
	if m.esSoloTraslacion() && aj.neutro() {
		copiarMovida(dst, origen, int(m.tx), int(m.ty), modo)
		return
	}

	inv, ok := m.invertir()
	if !ok {
		return
	}

	// la caja del destino que puede recibir algo: las CUATRO esquinas (con dos no alcanza si esta girada)
	esquinasX := [4]float64{0, float64(origen.ancho), 0, float64(origen.ancho)}
	esquinasY := [4]float64{0, 0, float64(origen.alto), float64(origen.alto)}

	xmin, ymin := math.Inf(1), math.Inf(1)
	xmax, ymax := math.Inf(-1), math.Inf(-1)

	for k := 0; k < 4; k++ {
		x, y := m.aplicar(esquinasX[k], esquinasY[k])
		if x < xmin {
			xmin = x
		}

		if y < ymin {
			ymin = y
		}

		if x > xmax {
			xmax = x
		}

		if y > ymax {
			ymax = y
		}
	}

	x0, y0 := int(math.Floor(xmin)), int(math.Floor(ymin))
	x1, y1 := int(math.Ceil(xmax)), int(math.Ceil(ymax))

	if x0 < 0 {
		x0 = 0
	}

	if y0 < 0 {
		y0 = 0
	}

	if x1 > dst.ancho {
		x1 = dst.ancho
	}

	if y1 > dst.alto {
		y1 = dst.alto
	}

	// las cuentas del bucle en simple (ver hsv)
	ia := float32(inv.a)
	ib := float32(inv.b)
	ic := float32(inv.c)
	id := float32(inv.d)
	itx := float32(inv.tx)
	ity := float32(inv.ty)

	for y := y0; y < y1; y++ {
		fy := float32(y) + 0.5
		sx0 := ib*fy + itx
		sy0 := id*fy + ity

		for x := x0; x < x1; x++ {
			fx := float32(x) + 0.5
			sxi := int(sx0 + ia*fx)
			syi := int(sy0 + ic*fx)

			if sxi < 0 || syi < 0 || sxi >= origen.ancho || syi >= origen.alto {
				continue
			}

			j := (syi*origen.ancho + sxi) * 4
			mezclar(dst, (y*dst.ancho+x)*4,
				origen.pixeles[j+0], origen.pixeles[j+1], origen.pixeles[j+2], origen.pixeles[j+3], modo, aj)
		}
	}
}

// ajusteNeutro: el ajuste que no toca el pixel. OJO: el valor CERO de ajuste NO sirve -- `efecto` cero es
// d2enum.DrawEffectPctTransparency25 (el iota empieza ahi) y brillo/saturacion cero apagan el color. Usarlo
// dejaba NEGROS todos los sprites que van por el camino rapido (medido en DC-002.69: los tiles y el heroe
// salian (0,0,0) en la captura mientras los diamantes, que pasan por el camino general, salian bien).
func ajusteNeutro() ajuste {
	return ajuste{brillo: 1, saturacion: 1, efecto: d2enum.DrawEffectNone}
}

// copiarMovida: el dibujado cuando la transformacion es SOLO MOVER. Se recorre la fila y se copia (misma
// optimizacion y misma razon que el shim, dibujo.go:300).
func copiarMovida(dst, origen *Imagen, dx, dy int, modo modoMezcla) {
	x0, y0 := dx, dy
	x1, y1 := dx+origen.ancho, dy+origen.alto

	if x0 < 0 {
		x0 = 0
	}

	if y0 < 0 {
		y0 = 0
	}

	if x1 > dst.ancho {
		x1 = dst.ancho
	}

	if y1 > dst.alto {
		y1 = dst.alto
	}

	if x0 >= x1 || y0 >= y1 {
		return
	}

	var sinAjuste ajuste = ajusteNeutro()

	for y := y0; y < y1; y++ {
		arriba := (y - dy) * origen.ancho
		d := (y*dst.ancho + x0) * 4

		for x := x0; x < x1; x++ {
			j := (arriba + (x - dx)) * 4
			mezclar(dst, d,
				origen.pixeles[j+0], origen.pixeles[j+1], origen.pixeles[j+2], origen.pixeles[j+3],
				modo, sinAjuste)
			d += 4
		}
	}
}

// mezclar escribe el pixel de origen ya ajustado en la posicion `d` del destino.
func mezclar(dst *Imagen, d int, r, g, b, al uint8, modo modoMezcla, aj ajuste) {
	r, g, b, al = aj.aplicar(r, g, b, al)

	switch modo {
	case mezclaCopiar:
		dst.pixeles[d+0], dst.pixeles[d+1], dst.pixeles[d+2], dst.pixeles[d+3] = r, g, b, al

	case mezclaSumar:
		dst.pixeles[d+0] = sumarTope(dst.pixeles[d+0], r)
		dst.pixeles[d+1] = sumarTope(dst.pixeles[d+1], g)
		dst.pixeles[d+2] = sumarTope(dst.pixeles[d+2], b)
		dst.pixeles[d+3] = sumarTope(dst.pixeles[d+3], al)

	case mezclaMultiplicar:
		dst.pixeles[d+0] = uint8((uint16(dst.pixeles[d+0]) * uint16(r)) / 255)
		dst.pixeles[d+1] = uint8((uint16(dst.pixeles[d+1]) * uint16(g)) / 255)
		dst.pixeles[d+2] = uint8((uint16(dst.pixeles[d+2]) * uint16(b)) / 255)

	default: // encima
		if al == 0 {
			return
		}

		if al == 255 {
			dst.pixeles[d+0], dst.pixeles[d+1], dst.pixeles[d+2], dst.pixeles[d+3] = r, g, b, 255
			return
		}

		a := uint32(al)
		resto := 255 - a
		dst.pixeles[d+0] = uint8((uint32(r)*a + uint32(dst.pixeles[d+0])*resto) / 255)
		dst.pixeles[d+1] = uint8((uint32(g)*a + uint32(dst.pixeles[d+1])*resto) / 255)
		dst.pixeles[d+2] = uint8((uint32(b)*a + uint32(dst.pixeles[d+2])*resto) / 255)
		dst.pixeles[d+3] = uint8(uint32(al) + uint32(dst.pixeles[d+3])*resto/255)
	}
}

func sumarTope(a, b uint8) uint8 {
	s := uint16(a) + uint16(b)
	if s > 255 {
		return 255
	}

	return uint8(s)
}

// aRGBA convierte un color de la interfaz al RGBA de 8 bits que se guarda en el lienzo.
//
// OJO CON EL DESMULTIPLICADO: color.Color.RGBA() devuelve los canales CON EL ALFA YA MULTIPLICADO (asi lo
// define la biblioteca de Go), y aca se guarda el color con el alfa aparte. Sin desmultiplicar, un sprite
// semitransparente sale mas oscuro de lo que corresponde.
func aRGBA(c color.Color) color.RGBA {
	if c == nil {
		return color.RGBA{}
	}

	switch v := c.(type) {
	case color.RGBA:
		return v
	case color.NRGBA:
		return color.RGBA{R: v.R, G: v.G, B: v.B, A: v.A}
	}

	r, g, b, a := c.RGBA()
	if a == 0 {
		return color.RGBA{}
	}

	return color.RGBA{
		R: uint8(r * 255 / a),
		G: uint8(g * 255 / a),
		B: uint8(b * 255 / a),
		A: uint8(a >> 8),
	}
}

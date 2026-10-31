// DC-002.94 -- EL CUADRO DEL HOST: Surface y Renderer de software, en Go puro.
//
// POR QUE EXISTE. El camino de dibujo del motor (`d2maprenderer`) escribe en una `d2interface.Surface`.
// En la consola esa Surface es la del backend PVR; en el host la unica implementacion era la de
// `d2core/d2render/ebiten`, que arrastra Ebitengine (contexto de video, hilo de UI) y no se puede usar en
// un medidor de consola. Esta es una Surface de software de verdad: un lienzo RGBA de 8 bits por canal, con
// la misma semantica de estado que la de la PC (traslacion que se SUMA, color que multiplica, pila que
// paniquea vacia) y un blit con alpha source-over. Alcanza para el cuadro del mapa: `d2maprenderer` solo usa
// Clear, PushTranslation/Pop/PopN, PushColor, Render, DrawRect, DrawLine y DrawTextf.
//
// LAS CUENTAS SON LAS DE SIEMPRE, no una aproximacion: el blit recorre el origen pixel a pixel, salta el
// alfa cero y mezcla `dst = src*a + dst*(1-a)`. Es el mismo trabajo que hace el rasterizador del PVR
// (d2core/d2render/pvr/imagen.go) antes de cuantizar a RGB565.
package main

import (
	"image"
	"image/color"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2enum"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2interface"
)

const (
	canales    = 4
	alfaOpaco  = 255
	byteMaximo = 255
)

// el contrato, sin suposiciones: si d2interface cambia, esto no compila.
var (
	_ d2interface.Surface  = (*Cuadro)(nil)
	_ d2interface.Renderer = (*RendererHost)(nil)
)

// estado: lo que se apila en Push*/Pop.
type estado struct {
	tx, ty     int
	color      color.RGBA
	brillo     float64
	saturacion float64
	escalaX    float64
	escalaY    float64
	inclinaX   float64
	inclinaY   float64
	efecto     d2enum.DrawEffect
	filtro     d2enum.Filter
}

func estadoNeutro() estado {
	return estado{color: color.RGBA{R: alfaOpaco, G: alfaOpaco, B: alfaOpaco, A: alfaOpaco},
		brillo: 1, saturacion: 1, escalaX: 1, escalaY: 1}
}

// Cuadro: el lienzo de la Surface del host.
type Cuadro struct {
	pix      []byte // RGBA, 4 bytes por pixel, de arriba a abajo
	ancho    int
	alto     int
	pila     []estado
	actual   estado
	renderer *RendererHost
	textos   int
}

// Pixeles: el lienzo crudo (w*h*4), para hashes y comparaciones.
func (c *Cuadro) Pixeles() []byte { return c.pix }

// Textos: cuantos DrawTextf recibio (el mapa pide texto de depuracion; no se dibuja, se cuenta).
func (c *Cuadro) Textos() int { return c.textos }

// PixelesNoVacios: cuantos pixeles tienen algun canal de color distinto de cero (el instrumento de siempre).
func (c *Cuadro) PixelesNoVacios() int {
	llenos := 0

	for i := 0; i+canales <= len(c.pix); i += canales {
		if c.pix[i] != 0 || c.pix[i+1] != 0 || c.pix[i+2] != 0 {
			llenos++
		}
	}

	return llenos
}

// ============================================================ el contrato de Surface

func (c *Cuadro) Renderer() d2interface.Renderer { return c.renderer }

func (c *Cuadro) Clear(clr color.Color) {
	r, g, b, a := rgba8(clr)

	for i := 0; i+canales <= len(c.pix); i += canales {
		c.pix[i], c.pix[i+1], c.pix[i+2], c.pix[i+3] = r, g, b, a
	}
}

func (c *Cuadro) DrawRect(ancho, alto int, clr color.Color) {
	r, g, b, a := c.tintado(rgba8(clr))

	for y := 0; y < alto; y++ {
		for x := 0; x < ancho; x++ {
			c.mezclar(c.actual.tx+x, c.actual.ty+y, r, g, b, a)
		}
	}
}

func (c *Cuadro) DrawLine(x, y int, clr color.Color) {
	r, g, b, a := c.tintado(rgba8(clr))

	// la linea va de (tx, ty) a (tx+x, ty+y): se camina por el eje mas largo
	pasos := x

	if y > pasos {
		pasos = y
	} else if -y > pasos {
		pasos = -y
	}

	if pasos < 0 {
		pasos = -pasos
	}

	for i := 0; i <= pasos; i++ {
		fx, fy := 0, 0
		if pasos > 0 {
			fx, fy = x*i/pasos, y*i/pasos
		}

		c.mezclar(c.actual.tx+fx, c.actual.ty+fy, r, g, b, a)
	}
}

func (c *Cuadro) DrawTextf(string, ...interface{}) { c.textos++ }

func (c *Cuadro) GetSize() (int, int) { return c.ancho, c.alto }

func (c *Cuadro) GetDepth() int { return len(c.pila) }

func (c *Cuadro) Pop() {
	if len(c.pila) == 0 {
		panic("empty stack")
	}

	c.actual = c.pila[len(c.pila)-1]
	c.pila = c.pila[:len(c.pila)-1]
}

func (c *Cuadro) PopN(n int) {
	for i := 0; i < n; i++ {
		c.Pop()
	}
}

func (c *Cuadro) PushColor(clr color.Color) {
	nuevo := c.actual
	r, g, b, a := rgba8(clr)

	nuevo.color = color.RGBA{
		R: uint8(int(nuevo.color.R) * int(r) / byteMaximo),
		G: uint8(int(nuevo.color.G) * int(g) / byteMaximo),
		B: uint8(int(nuevo.color.B) * int(b) / byteMaximo),
		A: uint8(int(nuevo.color.A) * int(a) / byteMaximo),
	}

	c.pila = append(c.pila, c.actual)
	c.actual = nuevo
}

func (c *Cuadro) PushEffect(efecto d2enum.DrawEffect) {
	nuevo := c.actual
	nuevo.efecto = efecto
	c.pila = append(c.pila, c.actual)
	c.actual = nuevo
}

func (c *Cuadro) PushFilter(filtro d2enum.Filter) {
	nuevo := c.actual
	nuevo.filtro = filtro
	c.pila = append(c.pila, c.actual)
	c.actual = nuevo
}

// PushTranslation apila y SUMA a la traslacion actual (asi lo hace el backend de la PC: no la reemplaza).
func (c *Cuadro) PushTranslation(x, y int) {
	c.pila = append(c.pila, c.actual)
	c.actual.tx += x
	c.actual.ty += y
}

func (c *Cuadro) PushSkew(x, y float64) {
	nuevo := c.actual
	nuevo.inclinaX, nuevo.inclinaY = x, y
	c.pila = append(c.pila, c.actual)
	c.actual = nuevo
}

func (c *Cuadro) PushScale(x, y float64) {
	nuevo := c.actual
	nuevo.escalaX, nuevo.escalaY = x, y
	c.pila = append(c.pila, c.actual)
	c.actual = nuevo
}

func (c *Cuadro) PushBrightness(brillo float64) {
	nuevo := c.actual
	nuevo.brillo = brillo
	c.pila = append(c.pila, c.actual)
	c.actual = nuevo
}

func (c *Cuadro) PushSaturation(saturacion float64) {
	nuevo := c.actual
	nuevo.saturacion = saturacion
	c.pila = append(c.pila, c.actual)
	c.actual = nuevo
}

// Render: pega otra superficie con la traslacion y el color del estado actual.
func (c *Cuadro) Render(superficie d2interface.Surface) {
	c.RenderSection(superficie, image.Rect(0, 0, 0, 0))
}

// RenderSection: lo mismo, recortando la seccion pedida del origen (rect vacio = todo el origen).
func (c *Cuadro) RenderSection(superficie d2interface.Surface, seccion image.Rectangle) {
	origen, ok := superficie.(*Cuadro)
	if !ok || origen == nil {
		panic("Cuadro.RenderSection: el origen no es un Cuadro")
	}

	x0, y0, x1, y1 := 0, 0, origen.ancho, origen.alto

	if seccion.Dx() > 0 && seccion.Dy() > 0 {
		x0, y0, x1, y1 = seccion.Min.X, seccion.Min.Y, seccion.Max.X, seccion.Max.Y
	}

	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			i := (y*origen.ancho + x) * canales
			if i+canales > len(origen.pix) {
				continue
			}

			r, g, b, a := origen.pix[i], origen.pix[i+1], origen.pix[i+2], origen.pix[i+3]
			if a == 0 {
				// el alfa cero no se dibuja: es el fondo de los tiles
				continue
			}

			r, g, b, a = c.tintado(r, g, b, a)
			c.mezclar(c.actual.tx+x, c.actual.ty+y, r, g, b, a)
		}
	}
}

// RenderIndicesTintado: dibuja indices de 8 bits con la MISMA cuenta que el backend PVR (DC-002.101): traslacion
// del estado, color que multiplica y alfa source-over. Asi el cuadro del camino de indices se puede comparar
// pixel a pixel contra el del camino RGBA.
func (c *Cuadro) RenderIndicesTintado(indices []byte, ancho, alto int, paleta [][4]uint8) {
	if ancho <= 0 || alto <= 0 || len(indices) < ancho*alto || len(paleta) < 256 {
		return
	}

	for y := 0; y < alto; y++ {
		for x := 0; x < ancho; x++ {
			idx := indices[y*ancho+x]
			if idx == 0 {
				continue
			}

			p := paleta[idx]
			r, g, b, a := c.tintado(p[0], p[1], p[2], p[3])
			c.mezclar(c.actual.tx+x, c.actual.ty+y, r, g, b, a)
		}
	}
}

// ReplacePixels: carga el cuadro crudo (RGBA de 8 bits, w*h*4) en el lienzo.
func (c *Cuadro) ReplacePixels(pixeles []byte) {
	if len(pixeles) < len(c.pix) {
		panic("ReplacePixels: el buffer es mas chico que el lienzo")
	}

	copy(c.pix, pixeles[:len(c.pix)])
}

func (c *Cuadro) Screenshot() *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, c.ancho, c.alto))
	copy(img.Pix, c.pix)

	return img
}

// ============================================================ las cuentas del pixel

// tintado: el color del estado multiplica los canales (es lo que hace la PC con PushColor).
func (c *Cuadro) tintado(r, g, b, a uint8) (uint8, uint8, uint8, uint8) {
	return aplicar(r, c.actual.color.R), aplicar(g, c.actual.color.G),
		aplicar(b, c.actual.color.B), aplicar(a, c.actual.color.A)
}

func aplicar(v, factor uint8) uint8 {
	return uint8(int(v) * int(factor) / byteMaximo)
}

// mezclar: source-over. Es la cuenta del backend: el alfa del origen pesa y el resto queda del destino.
func (c *Cuadro) mezclar(x, y int, r, g, b, a uint8) {
	if x < 0 || y < 0 || x >= c.ancho || y >= c.alto {
		return
	}

	i := (y*c.ancho + x) * canales

	if a == alfaOpaco {
		c.pix[i], c.pix[i+1], c.pix[i+2], c.pix[i+3] = r, g, b, alfaOpaco

		return
	}

	if a == 0 {
		return
	}

	ia := int(a)
	resto := byteMaximo - ia
	c.pix[i] = uint8((int(r)*ia + int(c.pix[i])*resto) / byteMaximo)
	c.pix[i+1] = uint8((int(g)*ia + int(c.pix[i+1])*resto) / byteMaximo)
	c.pix[i+2] = uint8((int(b)*ia + int(c.pix[i+2])*resto) / byteMaximo)
	c.pix[i+3] = uint8((ia*ia + int(c.pix[i+3])*resto) / byteMaximo)
}

// rgba8: el color de la interfaz a cuatro bytes (color.Color.RGBA devuelve 16 bits con alfa premultiplicado).
func rgba8(clr color.Color) (uint8, uint8, uint8, uint8) {
	if clr == nil {
		return 0, 0, 0, 0
	}

	r, g, b, a := clr.RGBA()
	if a == 0 {
		return 0, 0, 0, 0
	}

	// des-premultiplicado: los pixeles del juego llevan el color puro y el alfa aparte
	return uint8(int(r>>8) * byteMaximo / int(a>>8)), uint8(int(g>>8) * byteMaximo / int(a>>8)),
		uint8(int(b>>8) * byteMaximo / int(a>>8)), uint8(a >> 8)
}

// ============================================================ el Renderer del host

// Estadisticas: lo que el renderer entrego (es la medida de la cache de tiles).
type Estadisticas struct {
	Superficies int
	Bytes       int
	AnchoMaximo int
	AltoMaximo  int
}

// RendererHost: los 15 metodos del contrato, sin video. `Run` no corre un ciclo: llama al callback de
// dibujado UNA vez, que es lo que mide un cuadro.
type RendererHost struct {
	estadisticas Estadisticas
	vsync        bool
	fullscreen   bool
	ultimo       *Cuadro
}

// NuevoRendererHost: el renderer del medidor.
func NuevoRendererHost() *RendererHost {
	return &RendererHost{vsync: true}
}

// Estadisticas: cuantas superficies se pidieron y cuantos bytes son (w*h*4).
func (r *RendererHost) Estadisticas() Estadisticas { return r.estadisticas }

func (r *RendererHost) GetRendererName() string { return "host-software (DC-002.94)" }

func (r *RendererHost) SetWindowIcon(string) {}

func (r *RendererHost) Run(dibujar func(d2interface.Surface) error, _ func() error,
	ancho, alto int, _ string) error {
	return dibujar(r.NewSurface(ancho, alto))
}

func (r *RendererHost) IsDrawingSkipped() bool { return false }

func (r *RendererHost) CreateSurface(superficie d2interface.Surface) (d2interface.Surface, error) {
	cuadro, ok := superficie.(*Cuadro)
	if !ok || cuadro == nil {
		panic("CreateSurface: la superficie no es de este renderer")
	}

	nuevo := r.NewSurface(cuadro.ancho, cuadro.alto)
	copy(nuevo.(*Cuadro).pix, cuadro.pix)

	return nuevo, nil
}

func (r *RendererHost) NewSurface(ancho, alto int) d2interface.Surface {
	if ancho < 1 {
		ancho = 1
	}

	if alto < 1 {
		alto = 1
	}

	c := &Cuadro{
		pix:      make([]byte, ancho*alto*canales),
		ancho:    ancho,
		alto:     alto,
		actual:   estadoNeutro(),
		renderer: r,
	}

	r.estadisticas.Superficies++
	r.estadisticas.Bytes += len(c.pix)

	if ancho > r.estadisticas.AnchoMaximo {
		r.estadisticas.AnchoMaximo = ancho
	}

	if alto > r.estadisticas.AltoMaximo {
		r.estadisticas.AltoMaximo = alto
	}

	r.ultimo = c

	return c
}

func (r *RendererHost) IsFullScreen() bool { return r.fullscreen }

func (r *RendererHost) SetFullScreen(v bool) { r.fullscreen = v }

func (r *RendererHost) SetVSyncEnabled(v bool) { r.vsync = v }

func (r *RendererHost) GetVSyncEnabled() bool { return r.vsync }

func (r *RendererHost) GetCursorPos() (int, int) { return 0, 0 }

func (r *RendererHost) CurrentFPS() float64 { return 0 }

func (r *RendererHost) ShowPanicScreen(string) {}

func (r *RendererHost) Print(_ interface{}, _ string) error { return nil }

func (r *RendererHost) PrintAt(_ interface{}, _ string, _, _ int) {}

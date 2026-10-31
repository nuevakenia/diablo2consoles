//go:build dreamcast

package pvr

import (
	"image"
	"image/color"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2enum"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2interface"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2util"
)

// Surface es la implementacion de d2interface.Surface sobre el lienzo en memoria + el PVR.
//
// LOS 21 METODOS DEL CONTRATO y como se cumple cada uno (es la tabla de la evidencia):
//
//	Renderer()          nil: el Renderer es DC-002.70 (el ciclo de cuadros es otro contrato).
//	Clear(color)        rellena el lienzo entero.
//	DrawRect(w,h,c)     rectangulo lleno en (x, y) del estado actual.
//	DrawLine(x,y,c)     linea de (x, y) del estado a (x+x, y+y) (asi lo hace el backend de la PC).
//	DrawTextf(f,...)    fuente 3x5 propia, PROVISIONAL (la de verdad es d2font + los DC6 de la interfaz).
//	GetSize()           ancho y alto del lienzo.
//	GetDepth()          cuantos estados hay apilados (len de la pila, igual que en la PC).
//	Pop()/PopN(n)       desapila; con la pila vacia hace PANIC como en la PC ("empty stack").
//	PushColor(c)        tinte: multiplica los canales y el alfa por el color.
//	PushEffect(e)       las tablas del PL2: 25/50/75% de transparencia, Modulate (suma), Burn (producto),
//	                    Mod2X (170% de brillo) y Mod2XTrans (alfa por luminancia).
//	PushFilter(f)       se guarda; el dibujado es NEAREST siempre (limitacion declarada, ver doc.go).
//	PushTranslation(x,y) NO apila el estado nuevo encima: SUMA a la traslacion actual (como la PC).
//	PushSkew(x,y)/PushScale(x,y) como la PC (reemplazan el valor, no lo multiplican).
//	PushBrightness/PushSaturation  HSV aproximado en float32 (misma cuenta que ChangeHSV con el tono en 0).
//	Render(sfc)         dibuja otra Surface con la transformacion y el color del estado.
//	RenderSection(sfc, r)  lo mismo, recortando la seccion `r` de la Surface de origen.
//	ReplacePixels(p)    carga el cuadro crudo (RGBA de 8 bits, w*h*4) en el lienzo.
//	Screenshot()        devuelve el lienzo como *image.RGBA (copia).
type Surface struct {
	imagen *Imagen
	pila   []estado
	actual estado

	// renderer: el Renderer que la creo (nil si la superficie se armo sola, como en la prueba de humo de
	// DC-002.69). Es lo que devuelve Renderer().
	renderer *Renderer

	// textos: cuantos DrawTextf se pidieron (para la evidencia: el motor llama, aunque la fuente sea la
	// provisional).
	textos int
}

// que el contrato se cumpla no es una suposicion: si d2interface.Surface cambia, esto no compila.
var _ d2interface.Surface = (*Surface)(nil)

// NuevaSurface: una superficie del tamano pedido, con el estado de dibujado neutro.
func NuevaSurface(ancho, alto int) *Surface {
	return &Surface{imagen: NuevaImagen(ancho, alto), actual: estadoInicial()}
}

// SurfaceDesdeImagen: envuelve un lienzo ya existente (lo usa ReplacePixels y el que compone fuera).
func SurfaceDesdeImagen(img *Imagen) *Surface {
	return &Surface{imagen: img, actual: estadoInicial()}
}

// Imagen: el lienzo de la superficie (para subirlo al PVR, medirlo o compararlo).
func (s *Surface) Imagen() *Imagen { return s.imagen }

// Textos: cuantos DrawTextf se pidieron en la vida de esta superficie.
func (s *Surface) Textos() int { return s.textos }

// ============================================================ el contrato

// RenderSprite dibuja un sprite PALETIZADO con la transformacion y el color del estado actual.
//
// POR QUE NO ES PARTE DEL CONTRATO: d2interface.Surface solo conoce Surface (Render/RenderSection); el
// sprite paletizado (indices + paleta, el formato de los DC6/COF) es una pieza de ESTE backend. La capa de
// arriba lo envuelve en una Surface (asi lo hace d2ui.Sprite) o llama a esto directo, que se ahorra la
// expansion a RGBA cuando el sprite ya la tiene hecha.
func (s *Surface) RenderSprite(sp *Sprite) {
	if sp == nil {
		return
	}

	copiarEn(s.imagen, sp.Imagen(), s.matriz(), s.ajuste(), sinForzar)
}

// Renderer devuelve el renderer de esta superficie: el que la creo (DC-002.70). Es nil solo si la
// superficie se armo sola (NuevaSurface fuera de un Renderer, como en la prueba de humo de DC-002.69).
func (s *Surface) Renderer() d2interface.Renderer {
	if s.renderer == nil {
		return nil
	}

	return s.renderer
}

// RenderIndices dibuja un mapa de INDICES de 8 bits con su paleta en la posicion actual, expandiendo los
// pixeles AL VUELO contra el lienzo.
//
// POR QUE EXISTE, Y NO SE USA RenderSprite: RenderSprite cachea la expansion a RGBA del sprite, y el MAPA
// del nivel es 512x512: 1.048.576 B de cache que el monton de la consola no tiene junto con la pantalla
// (medido: el techo es ~1,23 MB para un bloque). Aca no se guarda nada: se lee el indice y se escribe el
// pixel, asi que el mapa cuesta sus 262.144 B de indices y nada mas. Para el personaje y los sprites chicos
// va RenderSprite, que es mas barato por cuadro.
//
// LA PALETA ES DE 8 BITS (R,G,B por indice), NO ARGB1555, y es a proposito: este es el camino por SOFTWARE
// (el lienzo es RGBA de 8 bits), asi que cuantizar a los 5 bits del PVR seria perder color sin ninguna
// ganancia. La paleta de 5 bits es la del camino de HARDWARE (Sprite/SubirSpritePVR), que es otra cosa.
// Medido en DC-002.70: con la paleta ARGB1555 el cuadro de la consola coincidia con el del host en el 44,15 %
// de los pixeles (el resto a +-8 por la cuantizacion); con la de 8 bits, ver la evidencia.
//
// EL INDICE 0 ES TRANSPARENTE (el convenio de los DC6/COF y del mapa del nivel: lo que no pinto ningun tile
// se queda con lo que habia en el lienzo).
func (s *Surface) RenderIndices(indices []byte, ancho, alto int, paleta [][3]uint8) {
	if ancho <= 0 || alto <= 0 || len(indices) < ancho*alto || len(paleta) < 256 {
		return
	}

	dx, dy := s.actual.x, s.actual.y
	anchoLienzo, altoLienzo := s.imagen.ancho, s.imagen.alto
	pixeles := s.imagen.pixeles

	for y := 0; y < alto; y++ {
		cy := dy + y
		if cy < 0 || cy >= altoLienzo {
			continue
		}

		base := cy * anchoLienzo * 4
		fila := y * ancho

		for x := 0; x < ancho; x++ {
			idx := indices[fila+x]
			if idx == 0 {
				continue
			}

			cx := dx + x
			if cx < 0 || cx >= anchoLienzo {
				continue
			}

			c := paleta[idx]
			j := base + cx*4

			pixeles[j+0] = c[0]
			pixeles[j+1] = c[1]
			pixeles[j+2] = c[2]
			pixeles[j+3] = 255
		}
	}
}

// ExpandirIndices expande indices de 8 bits con su paleta DIRECTAMENTE en el lienzo de esta superficie, sin
// buffer intermedio.
//
// POR QUE EXISTE (DC-002.85). El motor cargaba un cuadro de animacion con `d2util.ImgIndexToRGBA` (que pide
// 4 x pixeles en el monton) + `ReplacePixels` (que los copia al lienzo y despues los tira): DOS buffers de
// 4 x pixeles vivos por cuadro, y el primero es basura apenas se copia. DC-002.84 midio que el primer cuadro
// de un `Composite` crea ~96 superficies de capa, o sea ~192 buffers; en la consola ese pedido es el recurso
// que se agota. Aca el que expande escribe en el lienzo que ya existe: UN buffer por cuadro.
//
// LAS REGLAS SON LAS MISMAS DE SIEMPRE (las de `d2util.ImgIndexToRGBAEn`, byte a byte): el indice 0 queda
// transparente (0,0,0,0) sin importar la paleta y el resto toma R,G,B,A de su color. La expansion es de
// arriba a abajo y de izquierda a derecha, IGNORANDO la traslacion/escala del estado: es la CARGA de un
// cuadro entero (lo mismo que ReplacePixels), no un dibujado -- el dibujado es Render/RenderSprite.
//
// SI LOS INDICES NO SON DEL TAMANO DEL LIENZO NO SE ADIVINA: se expande lo que entra y el resto del lienzo
// queda como estaba, igual que ReplacePixels. Un buffer corto es un error del que llama.
func (s *Surface) ExpandirIndices(indices []byte, paleta d2interface.Palette) {
	d2util.ImgIndexToRGBAEn(s.imagen.pixeles, indices, paleta)

	if s.renderer != nil {
		s.renderer.expansionesDirectas++
		s.renderer.bytesExpandidos += len(indices) * 4
	}
}

// Clear rellena el lienzo entero con el color.
func (s *Surface) Clear(c color.Color) { s.imagen.Rellenar(aRGBA(c)) }

// DrawRect pinta un rectangulo lleno con la esquina en la posicion actual.
func (s *Surface) DrawRect(ancho, alto int, c color.Color) {
	s.imagen.Rectangulo(s.actual.x, s.actual.y, ancho, alto, aRGBA(c))
}

// DrawLine pinta una linea desde la posicion actual hasta (x, y) relativos a ella.
func (s *Surface) DrawLine(x, y int, c color.Color) {
	s.imagen.Linea(s.actual.x, s.actual.y, s.actual.x+x, s.actual.y+y, aRGBA(c))
}

// DrawTextf escribe el texto formateado en la posicion actual (fuente 3x5 PROVISIONAL).
func (s *Surface) DrawTextf(format string, params ...interface{}) {
	s.textos++
	dibujarTexto(s.imagen, sprintf(format, params...), s.actual.x, s.actual.y, aRGBA(s.actual.col))
}

// GetSize: el tamano del lienzo.
func (s *Surface) GetSize() (int, int) { return s.imagen.Ancho(), s.imagen.Alto() }

// GetDepth: cuantos estados hay apilados.
func (s *Surface) GetDepth() int { return len(s.pila) }

// Pop desapila un estado. Con la pila vacia PANIQUEA, igual que el backend de la PC (es un error del
// llamador, no algo que se pueda tapar).
func (s *Surface) Pop() {
	n := len(s.pila)
	if n == 0 {
		panic("pila de estados vacia")
	}

	s.actual = s.pila[n-1]
	s.pila = s.pila[:n-1]
}

// PopN desapila n estados.
func (s *Surface) PopN(n int) {
	for i := 0; i < n; i++ {
		s.Pop()
	}
}

func (s *Surface) apilar() { s.pila = append(s.pila, s.actual) }

// PushColor apila el estado y fija el tinte del dibujado.
func (s *Surface) PushColor(c color.Color) {
	s.apilar()
	s.actual.col = c
}

// PushEffect apila el estado y fija el efecto del PL2.
func (s *Surface) PushEffect(e d2enum.DrawEffect) {
	s.apilar()
	s.actual.efecto = e
}

// PushFilter apila el estado y guarda el filtro (el dibujado es NEAREST, ver doc.go).
func (s *Surface) PushFilter(f d2enum.Filter) {
	s.apilar()
	s.actual.filtro = f
}

// PushTranslation apila y SUMA la traslacion (así lo hace el backend de la PC: no la reemplaza).
func (s *Surface) PushTranslation(x, y int) {
	s.apilar()
	s.actual.x += x
	s.actual.y += y
}

// PushSkew apila y fija el sesgo.
func (s *Surface) PushSkew(x, y float64) {
	s.apilar()
	s.actual.sesgoX = x
	s.actual.sesgoY = y
}

// PushScale apila y fija la escala.
func (s *Surface) PushScale(x, y float64) {
	s.apilar()
	s.actual.escalaX = x
	s.actual.escalaY = y
}

// PushBrightness apila y fija el brillo (1 = sin cambio).
func (s *Surface) PushBrightness(brillo float64) {
	s.apilar()
	s.actual.brillo = brillo
}

// PushSaturation apila y fija la saturacion (1 = sin cambio).
func (s *Surface) PushSaturation(saturacion float64) {
	s.apilar()
	s.actual.saturacion = saturacion
}

// Render dibuja otra superficie con la transformacion y el color del estado actual.
func (s *Surface) Render(sfc d2interface.Surface) {
	otra, ok := sfc.(*Surface)
	if !ok || otra == nil {
		return
	}

	copiarEn(s.imagen, otra.imagen, s.matriz(), s.ajuste(), sinForzar)
}

// RenderSection dibuja la seccion `bound` de otra superficie.
func (s *Surface) RenderSection(sfc d2interface.Surface, bound image.Rectangle) {
	otra, ok := sfc.(*Surface)
	if !ok || otra == nil {
		return
	}

	copiarEn(s.imagen, otra.imagen.recortar(bound), s.matriz(), s.ajuste(), sinForzar)
}

// ReplacePixels carga el cuadro crudo en el lienzo. El buffer es RGBA de 8 bits por canal, de arriba a
// abajo y de izquierda a derecha (w*h*4 bytes), que es el formato con el que se compara contra el host.
//
// SI EL LARGO NO COINCIDE NO SE ADIVINA: se copia lo que entra y se deja el resto como estaba. Un buffer
// de otro tamano es un error del llamador, y adivinarlo (por ejemplo escalando) haria que las pruebas
// compararan cosas distintas creyendo que son las mismas.
func (s *Surface) ReplacePixels(pixeles []byte) {
	n := len(s.imagen.pixeles)

	if s.renderer != nil {
		s.renderer.copiasDeBuffer++
		s.renderer.bytesCopiados += len(pixeles)
	}

	copy(s.imagen.pixeles, pixeles[:minInt(len(pixeles), n)])
}

// Screenshot devuelve el contenido del lienzo como *image.RGBA (una copia: el que la recibe puede
// guardarla o modificarla sin tocar el cuadro).
func (s *Surface) Screenshot() *image.RGBA {
	ancho, alto := s.GetSize()
	rgba := image.NewRGBA(image.Rectangle{Min: image.Point{X: 0, Y: 0}, Max: image.Point{X: ancho, Y: alto}})

	copy(rgba.Pix, s.imagen.pixeles[:ancho*alto*4])

	return rgba
}

// ============================================================ la geometria del estado

// matriz: la transformacion que arman los Push del estado, en el MISMO orden que el backend de la PC
// (sesgo, despues escala, despues traslacion -- d2core/d2render/ebiten/ebiten_surface.go:344).
func (s *Surface) matriz() matriz {
	m := identidad()

	if s.actual.sesgoX != 0 || s.actual.sesgoY != 0 {
		m = m.sesgar(s.actual.sesgoX, s.actual.sesgoY)
	}

	if s.actual.escalaX != 1.0 || s.actual.escalaY != 1.0 {
		m = m.escalar(s.actual.escalaX, s.actual.escalaY)
	}

	return m.trasladar(float64(s.actual.x), float64(s.actual.y))
}

// ajuste: el color/brillo/saturacion/efecto del estado actual.
func (s *Surface) ajuste() ajuste {
	a := ajuste{
		efecto:     s.actual.efecto,
		brillo:     s.actual.brillo,
		saturacion: s.actual.saturacion,
	}

	if s.actual.col != nil {
		a.color = aRGBA(s.actual.col)
		a.tieneColor = true
	}

	return a
}

func minInt(a, b int) int {
	if a < b {
		return a
	}

	return b
}

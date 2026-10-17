package ebiten

import (
	"image/color"
	"math"
)

// Dibujos: cuantas veces se llamo a DrawImage. Es lo que se mira en la evidencia para saber que el motor
// paso por aca y no que la pantalla quedo negra por otra razon.
var Dibujos int

// DrawImage: el metodo con el que el motor dibuja (target.DrawImage(s, op)). Src es *Image (el motor pasa
// lo que le devolvio la asercion de SubImage).
//
// COMO FUNCIONA (y por que asi): para cada pixel de la pantalla que puede recibir algo, se pregunta DONDE
// ESTABA ese pixel en la imagen original. Eso se hace con la INVERSA de la matriz de geometria, y es la
// forma correcta de dibujar: hacerlo al reves (recorrer la imagen y calcular donde cae cada pixel) deja
// huecos cuando la figura se agranda, porque un pixel del origen no alcanza a cubrir todos los que le
// tocan, y con rotacion el problema se ve como rayas.
//
// SIN FILTRO (nearest): se toma el pixel del origen que cae mas cerca. Es lo que hace Ebitengine por
// omision y lo que corresponde a un juego de 1998: el arte ya viene del tamano que se ve, y suavizarlo lo
// emborrona en vez de mejorarlo.
//
// EL COLOR: si el que llama no toco el ColorM (lo normal: el 99% de los dibujados), no se recorre la
// matriz -- se pregunta una sola vez y se copia. Recorrerla en cada pixel multiplicaria un cuadro de
// 640x480 por una cuenta de 16 numeros, para no cambiar nada.
func (i *Image) DrawImage(src *Image, op *DrawImageOptions) {
	Dibujos++

	if i == nil || src == nil || len(i.pixeles) == 0 || len(src.pixeles) == 0 {
		return
	}

	var g GeoM
	modo := CompositeModeSourceOver
	usaColores := false
	var cm ColorM
	if op != nil {
		g = op.GeoM
		cm = op.ColorM
		modo = op.CompositeMode
		usaColores = cm.guardada
	}

	// EL CASO RAPIDO, PRIMERO: si la transformacion es solo mover (el caso del motor casi siempre), no hay
	// ninguna cuenta que hacer por pixel y el camino general es puro gasto. Ver copiarMovida.
	if g.a1 == 0 && g.b == 0 && g.c == 0 && g.d1 == 0 && !usaColores {
		copiarMovida(i, src, int(math.Floor(g.tx)), int(math.Floor(g.ty)), modo)
		return
	}

	// una matriz sin vuelta (por ejemplo una escala por cero) no dibuja nada: preguntarle a la inversa
	// seria dividir por cero
	if !g.IsInvertible() {
		return
	}
	inversa := g
	inversa.Invert()

	// la caja de la pantalla que puede recibir algo: las CUATRO esquinas de la imagen, pasadas por la
	// matriz (con dos esquinas no alcanza si la figura esta girada: la caja real es mas grande)
	esquinas := [4][2]float64{{0, 0}, {float64(src.ancho), 0}, {0, float64(src.alto)}, {float64(src.ancho), float64(src.alto)}}
	xmin, ymin := math.Inf(1), math.Inf(1)
	xmax, ymax := math.Inf(-1), math.Inf(-1)
	for _, e := range esquinas {
		x, y := g.Apply(e[0], e[1])
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

	x0 := int(math.Floor(xmin))
	y0 := int(math.Floor(ymin))
	x1 := int(math.Ceil(xmax))
	y1 := int(math.Ceil(ymax))

	// recortar a la pantalla: lo que se sale no se dibuja (y no se escribe fuera de la memoria de la
	// imagen, que seria corromper la memoria de otro)
	if x0 < 0 {
		x0 = 0
	}
	if y0 < 0 {
		y0 = 0
	}
	if x1 > i.ancho {
		x1 = i.ancho
	}
	if y1 > i.alto {
		y1 = i.alto
	}

	// LAS CUENTAS DEL BUCLE EN PRECISION SIMPLE, Y ESTO ES LO IMPORTANTE DE TODO EL ARCHIVO:
	//
	// el SH-4 tiene unidad de punto flotante, pero SOLO DE 32 BITS: un float64 (doble) no lo hace el
	// hardware, lo EMULA una biblioteca por software, operacion por operacion. Medido en la consola con un
	// solo sprite de 256x256: dibujar costaba 260.737 microsegundos por cuadro (un cuarto de segundo) y el
	// cuadro entero 590.087 (1,7 cuadros por segundo). Con las cuentas en float32 baja a una fraccion. La
	// matriz de geometria sigue siendo float64 (es la API de Ebitengine y no se toca), pero la copia de
	// dentro del bucle se hace en simple: ahi la precision de sobra.
	ia1 := float32(inversa.a1 + 1)
	ib := float32(inversa.b)
	ic := float32(inversa.c)
	id1 := float32(inversa.d1 + 1)
	itx := float32(inversa.tx)
	ity := float32(inversa.ty)

	for y := y0; y < y1; y++ {
		// por fila: lo que no depende de x se calcula UNA vez (y esto tambien se nota: son dos cuentas
		// por fila en vez de dos por pixel)
		fy := float32(y) + 0.5
		sx0 := ib*fy + itx
		sy0 := id1*fy + ity

		for x := x0; x < x1; x++ {
			fx := float32(x) + 0.5

			// donde estaba este pixel en la imagen original
			sxi := int(sx0 + ia1*fx)
			syi := int(sy0 + ic*fx)

			if sxi < 0 || syi < 0 || sxi >= src.ancho || syi >= src.alto {
				continue
			}

			j := (syi*src.ancho + sxi) * 4
			ro, go_, bo, ao := src.pixeles[j+0], src.pixeles[j+1], src.pixeles[j+2], src.pixeles[j+3]

			// el color, si el que dibuja lo pidio (es lo que hace que un personaje se vea congelado,
			// envenenado o transparente)
			if usaColores {
				ro, go_, bo, ao = aplicarColor(cm, ro, go_, bo, ao)
			}

			d := (y*i.ancho + x) * 4

			switch modo {
			case CompositeModeCopy:
				// copiar tal cual, incluso el alfa (es lo que se usa para dibujar sobre la nada)
				i.pixeles[d+0], i.pixeles[d+1], i.pixeles[d+2], i.pixeles[d+3] = ro, go_, bo, ao

			case CompositeModeLighter:
				// sumar: asi se dibuja la luz y el fuego
				i.pixeles[d+0] = sumarTope(i.pixeles[d+0], ro)
				i.pixeles[d+1] = sumarTope(i.pixeles[d+1], go_)
				i.pixeles[d+2] = sumarTope(i.pixeles[d+2], bo)
				i.pixeles[d+3] = sumarTope(i.pixeles[d+3], ao)

			case CompositeModeMultiply:
				// multiplicar sobre 255: asi se tiñe lo que ya estaba
				i.pixeles[d+0] = uint8((uint16(i.pixeles[d+0]) * uint16(ro)) / 255)
				i.pixeles[d+1] = uint8((uint16(i.pixeles[d+1]) * uint16(go_)) / 255)
				i.pixeles[d+2] = uint8((uint16(i.pixeles[d+2]) * uint16(bo)) / 255)
				i.pixeles[d+3] = i.pixeles[d+3]

			default: // el de siempre: poner encima respetando la transparencia
				if ao == 0 {
					continue
				}
				if ao == 255 {
					i.pixeles[d+0], i.pixeles[d+1], i.pixeles[d+2], i.pixeles[d+3] = ro, go_, bo, 255
					continue
				}
				a := uint32(ao)
				resto := 255 - a
				i.pixeles[d+0] = uint8((uint32(ro)*a + uint32(i.pixeles[d+0])*resto) / 255)
				i.pixeles[d+1] = uint8((uint32(go_)*a + uint32(i.pixeles[d+1])*resto) / 255)
				i.pixeles[d+2] = uint8((uint32(bo)*a + uint32(i.pixeles[d+2])*resto) / 255)
				i.pixeles[d+3] = uint8(a + uint32(i.pixeles[d+3])*resto/255)
			}
		}
	}
}

// aplicarColor: pasa un pixel por la matriz de colores.
//
// El color de Go va con el alfa YA multiplicado (RGBA de 16 bits), asi que el ColorM desmultiplica antes de
// multiplicar la matriz y vuelve a multiplicar despues. Aca los canales son de 8 bits y el alfa tambien,
// asi que la cuenta se hace igual pero a escala de 255.
func aplicarColor(cm ColorM, r, g, b, a uint8) (uint8, uint8, uint8, uint8) {
	if !cm.guardada {
		return r, g, b, a
	}

	// a RGBA de 16 bits con el alfa multiplicado, como espera Apply
	var c color.RGBA
	if a == 0 {
		c = color.RGBA{R: 0, G: 0, B: 0, A: 0}
	} else {
		c = color.RGBA{R: r, G: g, B: b, A: a}
	}

	salida := cm.Apply(c)
	rr, gg, bb, aa := salida.RGBA()

	// y de vuelta a 8 bits. OJO: RGBA() devuelve el color con el alfa multiplicado, asi que hay que
	// desmultiplicar para recuperar el canal (si no, un sprite semitransparente sale mas oscuro).
	if aa == 0 {
		return 0, 0, 0, 0
	}

	desmul := func(v uint32) uint8 {
		x := (v * 255) / aa
		if x > 255 {
			x = 255
		}
		return uint8(x)
	}

	return desmul(rr), desmul(gg), desmul(bb), uint8(aa >> 8)
}

// sumarTope: suma sin pasar de 255 (asi lo hace el modo "lighter": la luz no da la vuelta y se vuelve negra).
func sumarTope(a, b uint8) uint8 {
	s := uint16(a) + uint16(b)
	if s > 255 {
		return 255
	}
	return uint8(s)
}

// Huella: una suma de todos los pixeles de la imagen (la posicion tambien cuenta, para que dos colores
// cambiados de lugar no den lo mismo).
//
// PARA QUE SIRVE: para que "el cuadro cambio" sea un NUMERO y no una impresion. Mirando la consola serie no
// se ve la pantalla, y una captura del emulador dice "hay algo" pero no "cambio lo que tenia que cambiar".
// Con la huella, dos cuadros que tienen que ser distintos dan distinto, y dos que tienen que ser iguales dan
// el mismo numero -- y eso se puede comparar contra lo que calcula el host sobre los mismos pixeles.
func (i *Image) Huella() uint32 {
	if i == nil || len(i.pixeles) == 0 {
		return 0
	}

	var h uint32
	for j, p := range i.pixeles {
		h = h*31 + uint32(p) + uint32(j&0xff)
	}
	return h
}

// HuellaDePantalla: la huella de la pantalla del juego (la que se le pasa a Draw).
func HuellaDePantalla() uint32 { return pantalla.Huella() }

// copiarMovida: el dibujado cuando la transformacion es SOLO MOVER (sin escala, sin giro, sin inclinacion).
//
// POR QUE ESTA DE APARTE: es el caso del motor casi siempre (poner un sprite en una posicion y nada mas), y
// hacerlo con la cuenta general cuesta caro al pedo. Medido en la consola con un sprite de 256x256: por el
// camino general, dibujar se llevaba 59.579 microsegundos de un cuadro de 111.548 (mas de la mitad) --
// haciendo, por cada pixel de la pantalla que le toca al sprite, dos multiplicaciones de punto flotante para
// averiguar algo que ya se sabe: que el pixel de la pantalla (x, y) viene del pixel (x-dx, y-dy).
//
// Aca no hay cuentas por pixel: se recorre la fila y se copia. Y cuando la fila es opaca y el modo es el de
// siempre (poner encima), ni siquiera se mira pixel por pixel: se copia la fila entera de una.
func copiarMovida(dst, src *Image, dx, dy int, modo CompositeMode) {
	x0 := dx
	y0 := dy
	x1 := dx + src.ancho
	y1 := dy + src.alto

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

	filaAncho := src.ancho * 4

	for y := y0; y < y1; y++ {
		sy := y - dy
		arriba := sy * filaAncho
		d := (y*dst.ancho + x0) * 4

		for x := x0; x < x1; x++ {
			j := arriba + (x-dx)*4
			ro, go_, bo, ao := src.pixeles[j], src.pixeles[j+1], src.pixeles[j+2], src.pixeles[j+3]

			switch modo {
			case CompositeModeCopy:
				dst.pixeles[d], dst.pixeles[d+1], dst.pixeles[d+2], dst.pixeles[d+3] = ro, go_, bo, ao
			case CompositeModeLighter:
				dst.pixeles[d] = sumarTope(dst.pixeles[d], ro)
				dst.pixeles[d+1] = sumarTope(dst.pixeles[d+1], go_)
				dst.pixeles[d+2] = sumarTope(dst.pixeles[d+2], bo)
				dst.pixeles[d+3] = sumarTope(dst.pixeles[d+3], ao)
			case CompositeModeMultiply:
				dst.pixeles[d] = uint8((uint16(dst.pixeles[d]) * uint16(ro)) / 255)
				dst.pixeles[d+1] = uint8((uint16(dst.pixeles[d+1]) * uint16(go_)) / 255)
				dst.pixeles[d+2] = uint8((uint16(dst.pixeles[d+2]) * uint16(bo)) / 255)
			default:
				if ao == 255 {
					dst.pixeles[d], dst.pixeles[d+1], dst.pixeles[d+2], dst.pixeles[d+3] = ro, go_, bo, 255
				} else if ao != 0 {
					a := uint32(ao)
					resto := 255 - a
					dst.pixeles[d] = uint8((uint32(ro)*a + uint32(dst.pixeles[d])*resto) / 255)
					dst.pixeles[d+1] = uint8((uint32(go_)*a + uint32(dst.pixeles[d+1])*resto) / 255)
					dst.pixeles[d+2] = uint8((uint32(bo)*a + uint32(dst.pixeles[d+2])*resto) / 255)
					dst.pixeles[d+3] = uint8(a + uint32(dst.pixeles[d+3])*resto/255)
				}
			}
			d += 4
		}
	}
}

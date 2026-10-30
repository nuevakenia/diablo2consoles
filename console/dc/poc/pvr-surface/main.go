//go:build dreamcast

// DC-002.69: PRUEBA DE HUMO del backend de render de la consola (d2core/d2render/pvr).
//
// QUE PRUEBA, Y POR QUE ASI: dibuja un cuadro CONOCIDO (tiles en iso, N sprites paletizados, texto, lineas),
// lo compone con la Surface del backend, dice cuantos sprites dibujo y el CRC-32 de la pantalla, y despues
// lo SUBE AL PVR y lo pinta. O sea: la misma corrida mide el contrato entero (composicion y pixeles) y el
// camino de hardware (VRAM y pantalla), y la captura del emulador dice si se ve.
//
// LOS SPRITES SON SINTETICOS Y CON UNA PALETA CONOCIDA, a proposito: el CRC-32 tiene que ser el MISMO en
// cada arranque y entre maquinas para que sirva de referencia. Un sprite sacado del MPQ daria un CRC que
// depende del archivo del juego, que no es parte del repo (higiene de datos: los del juego nunca entran).
//
// MUTANTE (para la proxima tarjeta que toque este paquete): cambiar el indice 0 de transparente a opaco, o
// cambiar la UV del quad, tiene que mover el CRC y/o la captura.
package main

import (
	"hash/crc32"
	"image"
	"image/color"

	"kos"

	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2render/pvr"
)

//extern d2c_serial_init
func d2cSerialInit()

//extern d2c_heap_libre
func d2cHeapLibre() uint

// init: la consola serie es la UNICA salida de texto en la Dreamcast (sin esto, KOS deja el handler en NULL
// y el println de Go no sale a ninguna parte). Va en init para que salga tambien lo de mas arriba.
func init() { d2cSerialInit() }

func main() {
	println("PVRSMOKE: arranca la prueba de humo del backend PVR (DC-002.69)")
	println("PVRSMOKE: monton libre al arrancar (bytes):", int(d2cHeapLibre()))

	const (
		ancho = 640
		alto  = 480
	)

	super := pvr.NuevaSurface(ancho, alto)
	println("PVRSMOKE: superficie", ancho, "x", alto, "- monton libre ahora (bytes):", int(d2cHeapLibre()))

	super.Clear(color.RGBA{R: 8, G: 10, B: 16, A: 255})

	// ---------------------------------------------------------------- los sprites, una vez
	tile := spriteCuadro(32)
	diamante := spriteDiamante(16)
	heroe := spriteDiamante(24)

	sprites := 0

	// 1) EL SUELO EN ISO: ocho tiles con la esquina iso (el centro de abajo de la caja es el punto del
	//    suelo). Es la geometria del POC: x avanza de a 32 y y de a 16, y el sprite se pinta desde su
	//    esquina, que es (x - ancho/2, y - alto).
	for i := 0; i < 8; i++ {
		x := 60 + i*64
		y := 90

		super.PushTranslation(x-tile.Ancho()/2, y-tile.Alto())
		super.RenderSprite(tile)
		super.Pop()

		sprites++
	}

	// 2) LA GRILLA DE SPRITES PALETIZADOS: 8x8 diamantes de 16x16, cada uno con un tinte distinto (es la
	//    pila de estados: PushTranslation + PushColor + PopN).
	for fy := 0; fy < 8; fy++ {
		for fx := 0; fx < 8; fx++ {
			super.PushTranslation(30+fx*24, 150+fy*24)
			super.PushColor(color.RGBA{R: uint8(64 + fx*24), G: uint8(64 + fy*24), B: 200, A: 255})
			super.RenderSprite(diamante)
			super.PopN(2)

			sprites++
		}
	}

	// 3) EL HEROE y el texto: la caja y la esquina iso del personaje (igual que el POC: el ancla es el
	//    punto del suelo, asi que se resta la mitad del ancho y toda la altura).
	heroeX, heroeY := 500, 380
	super.PushTranslation(heroeX-heroe.Ancho()/2, heroeY-heroe.Alto())
	super.RenderSprite(heroe)
	super.Pop()

	sprites++

	// 4) el marco y el texto (DrawRect y DrawLine del contrato: los dos dibujan DESDE la posicion del
	//    estado, asi que cada uno lleva su PushTranslation)
	amarillo := color.RGBA{R: 255, G: 200, B: 0, A: 255}

	super.PushTranslation(0, 0)
	super.DrawRect(ancho, 2, amarillo)
	super.Pop()

	super.PushTranslation(0, alto-2)
	super.DrawRect(ancho, 2, amarillo)
	super.Pop()

	super.PushTranslation(ancho-1, 0)
	super.DrawLine(-(ancho - 1), alto-1, color.RGBA{R: 40, G: 120, B: 60, A: 255})
	super.Pop()

	super.PushTranslation(24, 24)
	super.DrawTextf("PVRSMOKE DC-002.69: %d SPRITES", sprites)
	super.Pop()

	super.PushTranslation(24, 44)
	super.DrawTextf("PALETA 8 BITS + PL2 / PVR")
	super.Pop()

	// 5) RENDERSECTION: la seccion del heroe, copiada a la derecha del cuadro (es la unica forma de probar
	//    que Render/RenderSection leen de la superficie de origen y no de la pantalla).
	super.PushTranslation(560, 400)
	super.RenderSection(super, image.Rect(heroeX-heroe.Ancho()/2, heroeY-heroe.Alto(),
		heroeX-heroe.Ancho()/2+heroe.Ancho(), heroeY-heroe.Alto()+heroe.Alto()))
	super.Pop()

	// ---------------------------------------------------------------- las medidas del contrato
	pix := super.Imagen().Pixeles()
	llenos := 0

	for j := 0; j+3 < len(pix); j += 4 {
		if pix[j] != 8 || pix[j+1] != 10 || pix[j+2] != 16 {
			llenos++
		}
	}

	println("PVRSMOKE: sprites dibujados=", sprites, "(8 tiles iso + 64 diamantes + 1 heroe)")
	println("PVRSMOKE: pixeles no-vacios=", llenos, "de", ancho*alto, "crc32=", int(crc32.ChecksumIEEE(pix)))
	anchoReal, altoReal := super.GetSize()
	println("PVRSMOKE: tamano=", anchoReal, "x", altoReal, "profundidad-de-pila=", super.GetDepth(),
		"textos=", super.Textos())

	// ---------------------------------------------------------------- el PVR
	if err := pvr.PantallaIniciar(ancho, alto); err != nil {
		println("PVRSMOKE: ERROR la pantalla no arranco:", err.Error())
		return
	}

	println("PVRSMOKE: pantalla del PVR", ancho, "x", alto, "vram=", pvr.VramDeLaPantalla(), "bytes")

	// un sprite GRANDE por hardware (el del heroe, escalado x3), para que la captura muestre el camino del
	// PVR ademas del cuadro compuesto por software
	vramSprite, err := pvr.SubirSpritePVR(heroe)
	if err != nil {
		println("PVRSMOKE: ERROR el sprite no subio:", err.Error())
	} else {
		println("PVRSMOKE: sprite en VRAM=", vramSprite, "bytes (asset", heroe.Ancho(), "x", heroe.Alto(), ")")
	}

	println("PVRSMOKE: monton libre con el cuadro armado (bytes):", int(d2cHeapLibre()))

	// ---------------------------------------------------------------- el ciclo de cuadros
	kos.PvrSetBgColor(0, 0, 0)

	cuadros := 0

	for {
		kos.PvrWaitReady()
		kos.PvrSceneBegin()

		// la lista OPACA: el cuadro compuesto por software, que tapa todo
		kos.PvrListBegin(kos.PVR_LIST_OP_POLY)

		if err := pvr.Presentar(super.Imagen()); err != nil {
			println("PVRSMOKE: ERROR al subir el cuadro:", err.Error())
			kos.PvrListFinish()
			kos.PvrSceneFinish()

			return
		}

		pvr.PintarPantalla()
		kos.PvrListFinish()

		// la lista TRANSPARENTE: el sprite por HARDWARE encima del cuadro
		kos.PvrListBegin(kos.PVR_LIST_TR_POLY)
		pvr.DibujarSpritePVR(heroe, 470, 90, 3)
		kos.PvrListFinish()

		kos.PvrSceneFinish()

		if cuadros == 0 || cuadros%120 == 0 {
			println("PVRSMOKE: cuadro", cuadros, "pintado, crc32=", int(crc32.ChecksumIEEE(super.Imagen().Pixeles())))
		}

		cuadros++
	}
}

// spriteDiamante: un rombo (la silueta iso del piso y del heroe) con una paleta de 4 colores conocidos.
// El indice 0 es el fondo y es TRANSPARENTE (asi vienen los DC6/COF del juego).
func spriteDiamante(lado int) *pvr.Sprite {
	indices := make([]byte, lado*lado)
	paleta := make([]uint16, 256)

	// paleta ARGB1555: 1 = rojo, 2 = blanco (el borde), 3 = verde, 4 = azul
	paleta[1] = 0x8000 | (31 << 10)
	paleta[2] = 0x8000 | (31 << 10) | (31 << 5) | 31
	paleta[3] = 0x8000 | (31 << 5)
	paleta[4] = 0x8000 | 31

	medio := lado / 2

	for y := 0; y < lado; y++ {
		for x := 0; x < lado; x++ {
			dx, dy := x-medio, y-medio

			if dx < 0 {
				dx = -dx
			}

			if dy < 0 {
				dy = -dy
			}

			switch {
			case dx+dy == medio:
				indices[y*lado+x] = 2
			case dx+dy < medio:
				indices[y*lado+x] = 3
			}
		}
	}

	return pvr.NuevoSprite(lado, lado, indices, paleta)
}

// spriteCuadro: el tile del piso (un cuadrado con borde), tambien paletizado.
func spriteCuadro(lado int) *pvr.Sprite {
	indices := make([]byte, lado*lado)
	paleta := make([]uint16, 256)

	paleta[1] = 0x8000 | (12 << 10) | (14 << 5) | 10
	paleta[2] = 0x8000 | (20 << 10) | (18 << 5) | 12

	for y := 0; y < lado; y++ {
		for x := 0; x < lado; x++ {
			if x == 0 || y == 0 || x == lado-1 || y == lado-1 {
				indices[y*lado+x] = 2
			} else {
				indices[y*lado+x] = 1
			}
		}
	}

	return pvr.NuevoSprite(lado, lado, indices, paleta)
}

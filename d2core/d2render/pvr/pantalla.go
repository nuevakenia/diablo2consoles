//go:build dreamcast

package pvr

import (
	"image/color"
	"unsafe"
)

// El backend de video de la consola: la pantalla del motor en la VRAM del PVR.
//
// LAS FUNCIONES DE C SON LAS DE console/dc/std/csrc/pvr_pantalla.c, QUE YA ESTABAN MEDIDAS (DC-002.10/17/
// 22/26): la conversion RGBA -> RGB565 por bloques de 32x32, el twiddle con tabla, la subida por Store
// Queues y el quad de pantalla completa con las UV por cociente. Este paquete NO reimplementa nada de eso:
// lo llama. Lo que agrega es el contrato de Surface que lo usa.
//
// EL PUNTERO CRUZA COMO uintptr: un slice de Go pasado a una funcion //extern tiene el ABI ambiguo
// (puntero, largo y capacidad como tres argumentos), y un unsafe.Pointer guardado en un campo o capturado
// en una clausura hace que el compilador pida los descriptores de unsafe.Pointer, que el runtime de la
// consola NO tiene ("undefined reference to unsafe.Pointer..p" al enlazar).
//
//extern d2c_pvr_pantalla_init
func d2cPvrPantallaInit(w, h int) int

//extern d2c_pvr_pantalla_convertir
func d2cPvrPantallaConvertir(rgba uintptr, w, h int) int

//extern d2c_pvr_pantalla_pintar
func d2cPvrPantallaPintar()

//extern d2c_pvr_pantalla_vram
func d2cPvrPantallaVram() int

//extern d2c_pvr_pantalla_fb_init
func d2cPvrPantallaFbInit(w, h int) int

//extern d2c_pvr_pantalla_fb
func d2cPvrPantallaFb() uintptr

//extern d2c_pvr_pantalla_fb_bytes
func d2cPvrPantallaFbBytes() int

//extern d2c_pvr_pantalla_us
func d2cPvrUs() int

//extern d2c_pvr_pantalla_mem_libre
func d2cPvrPantallaMemLibre() int

//extern d2c_pvr_sprite_subir
func d2cPvrSpriteSubir(idx, pal uintptr, w, h, pw, ph int) int

//extern d2c_pvr_sprite_dibujar_go
func d2cPvrSpriteDibujar(x, y, ancho, alto, escala float32)

//extern d2c_pvr_sprite_liberar
func d2cPvrSpriteLiberar()

// errorPantalla: el error del backend de video. Un tipo con nombre y no un string pelado para que el que
// llame pueda distinguirlo de un error del motor.
type errorPantalla string

func (e errorPantalla) Error() string { return string(e) }

var pantallaLista bool

// PantallaIniciar arranca el video del PVR y reserva la textura de la pantalla del tamano pedido.
//
// SE PUEDE LLAMAR MAS DE UNA VEZ: si el tamano no cambio no hace nada, y si cambio se rehace la textura
// (DC-002.22: el arranque del motor pedia 640x480 y despues su tamano logico, y con la textura vieja la
// conversion moria con codigo -2 sin decir por que).
func PantallaIniciar(ancho, alto int) error {
	if ancho <= 0 || alto <= 0 {
		return errorPantalla("tamano de pantalla invalido")
	}

	if r := d2cPvrPantallaInit(ancho, alto); r != 0 {
		return errorPantalla("no se pudo arrancar la pantalla del PVR (codigo " + itoa(r) + ")")
	}

	pantallaLista = true

	return nil
}

// PantallaLista: si el video arranco.
func PantallaLista() bool { return pantallaLista }

// VramDeLaPantalla: cuantos bytes de VRAM ocupa la textura del cuadro (para la evidencia).
func VramDeLaPantalla() int { return d2cPvrPantallaVram() }

// Presentar sube el lienzo a la VRAM del PVR. Es el paso previo a PintarPantalla, y el unico punto del
// paquete que toca el chip con el cuadro entero.
func Presentar(img *Imagen) error {
	if img == nil || len(img.pixeles) == 0 {
		return errorPantalla("el cuadro esta vacio")
	}

	if r := d2cPvrPantallaConvertir(uintptr(unsafe.Pointer(&img.pixeles[0])), img.ancho, img.alto); r != 0 {
		return errorPantalla("no se pudo convertir/subir el cuadro (codigo " + itoa(r) + ")")
	}

	return nil
}

// PintarPantalla pinta el cuadro en toda la pantalla. El que llama tiene que tener abierta la lista OPACA
// del PVR (PVR_LIST_OP_POLY): la pantalla tapa todo, no hay nada detras.
func PintarPantalla() { d2cPvrPantallaPintar() }

// PresentarEnPantalla: los dos pasos juntos (subir + pintar), que es lo que hace un cuadro completo.
func PresentarEnPantalla(img *Imagen) error {
	if err := Presentar(img); err != nil {
		return err
	}

	PintarPantalla()

	return nil
}

// ============================================================ el sprite por HARDWARE

var spriteEnVram int

// SubirSpritePVR sube un sprite paletizado a la VRAM del PVR y deja su paleta en el banco 0.
//
// HAY UNA SOLA TEXTURA DE SPRITE A LA VEZ, y es una limitacion del camino de hardware que se declara: el
// banco de paleta 0 es UNO, asi que dos paletas distintas en el mismo cuadro no se pueden dibujar por
// hardware (por software si: Surface.DibujarSprite expande con la paleta de cada sprite). Subir otra
// libera la anterior.
//
// Devuelve los bytes que ocupa en la VRAM (la textura va rellenada a la potencia de dos que exige el chip).
func SubirSpritePVR(s *Sprite) (int, error) {
	if s == nil || len(s.indices) == 0 || len(s.paleta) < 256 {
		return 0, errorPantalla("sprite vacio")
	}

	// EL RELLENO, EN GO Y NO EN C: una textura del PVR mide una potencia de dos en las dos dimensiones, y un
	// frame de DC6 (118x52, por ejemplo) no la mide. Rellenar en C pidiendo un buffer temporal costaria
	// hasta 256 KB de .bss POR SPRITE; aca es un slice del tamano de la textura (8 KB para ese frame) que el
	// recolector libera solo cuando SubirSpritePVR vuelve.
	pw, ph := potenciaDeDos(s.ancho), potenciaDeDos(s.alto)

	indices := s.indices
	if pw != s.ancho || ph != s.alto {
		indices = rellenarIndices(s.indices, s.ancho, s.alto, pw, ph)
	}

	bytes := d2cPvrSpriteSubir(
		uintptr(unsafe.Pointer(&indices[0])),
		uintptr(unsafe.Pointer(&s.paleta[0])),
		s.ancho, s.alto, pw, ph)
	if bytes <= 0 {
		return 0, errorPantalla("no se pudo subir el sprite al PVR (codigo " + itoa(bytes) + ")")
	}

	spriteEnVram = bytes

	return bytes, nil
}

// potenciaDeDos: el menor 2^n que contiene a v (lo que exige el PVR para el lado de una textura).
func potenciaDeDos(v int) int {
	if v <= 1 {
		return 1
	}

	p := 1
	for p < v {
		p <<= 1
	}

	return p
}

// rellenarIndices: los indices del sprite en una textura de pw x ph, con el relleno en INDICE 0 (que es
// transparente). Sin esto el PVR leeria memoria que no es del asset en el borde del sprite.
func rellenarIndices(indices []byte, ancho, alto, pw, ph int) []byte {
	relleno := make([]byte, pw*ph)

	for y := 0; y < alto; y++ {
		copy(relleno[y*pw:y*pw+ancho], indices[y*ancho:(y+1)*ancho])
	}

	return relleno
}

// DibujarSpritePVR dibuja el sprite YA SUBIDO en (x, y) con la escala pedida, por el PVR.
//
// LA GEOMETRIA ES LA DEL POC (console/dc/poc/personaje-mapa/csrc/cshim.c:172): el quad va con las UV
// NORMALIZADAS (0..1, con medio texel de recorte para no muestrear el relleno de la potencia de dos) y la
// marca de fin de tira en el CUARTO vertice, no en el tercero.
//
// TIENE QUE HABER UNA LISTA ABIERTA: no abre ni cierra listas (eso es del ciclo de cuadros, que es del
// Renderer). Sin lista abierta no dibuja nada y no avisa: el que llama tiene el control del PVR.
func DibujarSpritePVR(s *Sprite, x, y int, escala float32) {
	if s == nil || spriteEnVram == 0 {
		return
	}

	if escala <= 0 {
		escala = 1
	}

	d2cPvrSpriteDibujar(float32(x), float32(y), float32(s.ancho), float32(s.alto), escala)
}

// LiberarSpritePVR devuelve la textura del sprite a la VRAM.
func LiberarSpritePVR() {
	d2cPvrSpriteLiberar()
	spriteEnVram = 0
}

// VramDelSprite: cuantos bytes de VRAM ocupa el sprite subido (0 = no hay ninguno).
func VramDelSprite() int { return spriteEnVram }

// ============================================================ el LIENZO del cuadro en la VRAM

// lienzoEnVram: cuantos bytes del lienzo del cuadro viven en la VRAM (0 = el lienzo esta en el monton).
var lienzoEnVram int

// SuperficieDeLaPantalla: la superficie del CUADRO del motor, con sus pixeles en la VRAM del PVR.
//
// POR QUE (medido, ver la cabecera de renderer.go): el lienzo 640x480x4 = 1.228.800 B y el techo del
// monton de C es ~1,23 MB para UN bloque; con el mapa del nivel (512x512 = 262.144 B) vivo al mismo tiempo
// no entran los dos. La palanca es la de DC-002.22, ya medida en el repo: `d2c_pvr_pantalla_fb_init`
// reserva w*h*4 bytes de los 8 MB de VRAM y el slice de pixeles apunta ahi. El resto del backend no cambia
// una linea: escribir un pixel sigue siendo `pixeles[i] = v`.
//
// LO QUE CUESTA, DICHO DE FRENTE: la VRAM se escribe sin cache, asi que componer el cuadro pixel a pixel
// contra VRAM es mas lento que contra RAM. Se cambia RAM que no habia por velocidad de dibujado, y el costo
// se mide (los us de cada etapa, mas abajo).
//
// SI LA VRAM NO DA: la superficie sale del monton (NuevaSurface) y el que mide lo sabe por
// VramDelLienzo(), que devuelve 0. El aviso tambien va a la consola serie.
func SuperficieDeLaPantalla(ancho, alto int) *Surface {
	if r := d2cPvrPantallaFbInit(ancho, alto); r != 0 {
		println("PVR-RENDERER: la VRAM no dio para el lienzo (codigo", r, "); el cuadro va al monton")

		lienzoEnVram = 0

		return NuevaSurface(ancho, alto)
	}

	n := ancho * alto * 4
	// el patron del shim de DC-002.22: puntero a array de largo maximo y recorte (el runtime de la consola
	// no tiene unsafe.Slice, y un unsafe.Pointer en un campo pide descriptores que libgodc no trae).
	pixeles := (*[1 << 30]byte)(unsafe.Pointer(d2cPvrPantallaFb()))[:n:n]

	img := &Imagen{pixeles: pixeles, ancho: ancho, alto: alto, vram: true}
	img.Rellenar(color.RGBA{R: 0, G: 0, B: 0, A: 255})

	lienzoEnVram = d2cPvrPantallaFbBytes()

	println("PVR-RENDERER: el lienzo del cuadro vive en la VRAM:", lienzoEnVram, "bytes")

	return SurfaceDesdeImagen(img)
}

// VramDelLienzo: cuantos bytes del lienzo del cuadro estan en la VRAM (0 = en el monton).
func VramDelLienzo() int { return lienzoEnVram }

// MemLibreDelMonton: los bytes libres del monton de C en este momento (para la evidencia: es el techo con
// el que hay que negociar la memoria del cuadro).
func MemLibreDelMonton() int { return d2cPvrPantallaMemLibre() }

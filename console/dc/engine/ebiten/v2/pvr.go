// El backend de VIDEO de la consola: la pantalla del motor subida a la VRAM del PVR.
//
// COMO FUNCIONA EL DIBUJADO EN LA DREAMCAST (y por que hace falta este archivo): el motor dibuja sus cuadros
// en una imagen en memoria, igual que en la PC con Ebitengine. Pero el hardware de la Dreamcast NO dibuja
// sobre memoria: dibuja sobre la VRAM del chip de video (el PVR), y solo pinta listas de triangulos. Asi
// que un cuadro sale en tres pasos:
//
//	1) el motor dibuja en la imagen (Draw, con DrawImage y todo lo que use el motor)
//	2) la imagen se convierte a lo que entiende el hardware (RGB565) y se sube a la VRAM (SubirPantalla)
//	3) se pinta la textura en toda la pantalla (PintarPantalla)
//
// El paso 2 no es un detalle de rendimiento: es la frontera entre la memoria normal (1 byte por canal) y la
// VRAM (2 bytes por pixel, con el color ya empaquetado). Se hace en C porque la conversacion con el chip
// (pvr_txr_load_ex) no esta en el binding de libgodc.
//
// LA PANTALLA ES 640x480 con la imagen a 1024x512 en la VRAM (potencia de dos: es una exigencia del chip,
// no una eleccion). Se declara para que el que lea un numero sepa de donde sale.
package ebiten

import (
	"strconv"
	"unsafe"
)

// Las funciones del backend de C (console/dc/std/csrc/pvr_pantalla.c). El puntero cruza como uintptr: un
// slice de Go pasado a una funcion //extern tiene el ABI ambiguo (puntero, largo y capacidad como tres
// argumentos), que es la misma regla que sigue la stdlib en os.go.
//
//extern d2c_pvr_pantalla_init
func d2cPvrPantallaInit(w, h int) int

//extern d2c_pvr_pantalla_subir
func d2cPvrPantallaSubir(rgba uintptr, w, h int) int

//extern d2c_pvr_pantalla_pintar
func d2cPvrPantallaPintar()

//extern d2c_pvr_pantalla_vram
func d2cPvrPantallaVram() int

//extern d2c_pvr_pantalla_us
func d2cPvrUs() uint

//extern d2c_pvr_pantalla_convertir
func d2cPvrPantallaConvertir(rgba uintptr, w, h int) int

//extern d2c_pvr_pantalla_subir_vram
func d2cPvrPantallaSubirVram() int

//extern d2c_pvr_pantalla_fb_init
func d2cPvrPantallaFbInit(w, h int) int

//extern d2c_pvr_pantalla_fb
func d2cPvrPantallaFb() uintptr

//extern d2c_pvr_pantalla_fb_bytes
func d2cPvrPantallaFbBytes() int

//extern d2c_pvr_pantalla_mem_libre
func d2cPvrPantallaMemLibre() int

// La pantalla es 640x480: es el modo de video que se elige (DM_640x480 con RGB565).
const (
	PantallaAncho = 640
	PantallaAlto  = 480
)

var pantallaIniciada bool
var pantallaAncho, pantallaAlto int

// IniciarPantalla: arranca el video de la consola y deja lista la textura de la pantalla.
//
// LOS ERRORES SE DEVUELVEN Y NO SE IGNORAN: si no se pudo reservar la VRAM, lo unico que se veria despues
// es una pantalla negra, y desde afuera eso es indistinguible de "el juego anda pero no dibuja nada". Con
// el error, el que llama puede decirlo.
//
// DC-002.22: ES RE-DIMENSIONABLE. Antes, la segunda llamada con OTRO tamano devolvia nil sin hacer nada
// (el `pantallaIniciada` cortaba de entrada) y el arranque del motor se quedaba con la textura de 640x480
// del POC mientras su pantalla logica era de 800x600: la conversion del cuadro moria con `codigo -2` y el
// error decia "no se pudo convertir el cuadro", que no es el defecto real (era el tamano).
func IniciarPantalla(ancho, alto int) error {
	if pantallaIniciada && pantallaAncho == ancho && pantallaAlto == alto {
		return nil
	}

	if r := d2cPvrPantallaInit(ancho, alto); r != 0 {
		return errorPantalla("no se pudo arrancar la pantalla del PVR (codigo " + itoa(r) + ")")
	}

	pantallaIniciada = true
	pantallaAncho, pantallaAlto = ancho, alto
	return nil
}

// PantallaIniciada: si el video arranco.
func PantallaIniciada() bool { return pantallaIniciada }

// TamanoDeLaPantalla: el tamano con el que esta la textura de la pantalla (el que iniciaron).
func TamanoDeLaPantalla() (int, int) { return pantallaAncho, pantallaAlto }

// VramDeLaPantalla: cuantos bytes de la VRAM ocupa el cuadro (para la evidencia).
func VramDeLaPantalla() int { return d2cPvrPantallaVram() }

// SubirPantalla: pasa el cuadro del motor (la imagen donde dibujo Draw) a la VRAM del PVR.
//
// Son DOS pasos y van separados porque son dos cosas distintas y cuesta plata distinta: CONVERTIR (recorrer
// los 307.200 pixeles de la pantalla en la CPU para pasar de RGBA al formato del hardware) y SUBIR (mandar
// el resultado por DMA a la VRAM). Medidos por separado se sabe cual de los dos conviene atacar.
func SubirPantalla(img *Image) error {
	if img == nil || len(img.pixeles) == 0 {
		return errorPantalla("la pantalla esta vacia")
	}
	if r := d2cPvrPantallaConvertir(uintptr(unsafe.Pointer(&img.pixeles[0])), img.ancho, img.alto); r != 0 {
		return errorPantalla("no se pudo convertir el cuadro (codigo " + itoa(r) + ")")
	}
	if r := d2cPvrPantallaSubirVram(); r != 0 {
		return errorPantalla("no se pudo subir el cuadro a la VRAM (codigo " + itoa(r) + ")")
	}
	return nil
}

// ConvertirPantalla y SubirVram: los dos pasos de SubirPantalla, por separado, para poder medirlos.
func ConvertirPantalla(img *Image) error {
	if img == nil || len(img.pixeles) == 0 {
		return errorPantalla("la pantalla esta vacia")
	}
	if r := d2cPvrPantallaConvertir(uintptr(unsafe.Pointer(&img.pixeles[0])), img.ancho, img.alto); r != 0 {
		return errorPantalla("no se pudo convertir el cuadro (codigo " + itoa(r) + ")")
	}
	return nil
}

// SubirVram: manda a la VRAM lo ultimo convertido.
func SubirVram() error {
	if r := d2cPvrPantallaSubirVram(); r != 0 {
		return errorPantalla("no se pudo subir el cuadro a la VRAM (codigo " + itoa(r) + ")")
	}
	return nil
}

// PintarPantalla: pinta el cuadro en toda la pantalla. El llamador tiene que tener abierta la lista de
// dibujado opaca (kos.PVR_LIST_OP_POLY).
func PintarPantalla() { d2cPvrPantallaPintar() }

// NuevaPantallaVram: la imagen de la pantalla del MOTOR con sus pixeles EN LA VRAM del PVR.
//
// DC-002.22. POR QUE EXISTE (medido, no supuesto): la pantalla logica del motor son 800x600x4 =
// 1.920.000 bytes y el monton de C del binario entero no los tiene -- cuando el motor los pide, el
// monton tiene `arena=2415484 used=2372348 free=43136` y el pedido no entra (faltan 1.876.864). La RAM
// principal son 16 MB y el binario ya carga 13.919.684, asi que no hay recorte del compilador que pague
// 1,9 MB (DC-002.14 lo midio: --gc-sections NO baja el binario). La VRAM son 8 MB y el PVR usa ~1 MB
// para la textura del cuadro: ahi si entran.
//
// COMO: `d2c_pvr_pantalla_fb_init` reserva w*h*4 bytes en la VRAM y devuelve el puntero; aca se convierte
// en el slice de pixeles de la imagen (memoria es memoria: el resto del shim -- SetRGBA, Clear,
// DrawImage, la conversion del cuadro -- no cambia una linea).
//
// SI NO HAY VRAM: se devuelve la imagen del monton, como antes. El POC chico, donde el monton tiene de
// sobra, sigue andando igual; y el que mide dice cual de los dos caminos se tomo (VramDelFramebuffer).
func NuevaPantallaVram(w, h int) *Image {
	if r := d2cPvrPantallaFbInit(w, h); r != 0 {
		println("PVR-FRAMEBUFFER la VRAM no dio: la pantalla va al monton (codigo", r, ")")

		return NewImage(w, h)
	}

	// el slice de pixeles apuntando a la VRAM. El patron es el de siempre en este shim: el puntero cruza
	// como uintptr (un unsafe.Pointer guardado en un campo o capturado en una clausura hace que el
	// compilador pida los descriptores de `unsafe.Pointer`, que el runtime de la consola NO tiene, y el
	// LINK falla con "undefined reference to unsafe.Pointer..p").
	n := w * h * 4
	// el patron de siempre en este shim, para no depender de unsafe.Slice (el go.mod del motor declara
	// una version de Go anterior a 1.17): puntero a array de largo maximo y recorte.
	pix := (*[1 << 30]byte)(unsafe.Pointer(d2cPvrPantallaFb()))[:n:n]

	println("PVR-FRAMEBUFFER la pantalla vive en la VRAM:", d2cPvrPantallaFbBytes(), "bytes")
	println("PVR-FRAMEBUFFER monton de C: libre", MemLibreDelMonton(), "bytes (el runtime lo imprime como free)")

	// EL INSTRUMENTO (DC-002.22): la prueba de IDA Y VUELTA contra la VRAM. El compositor del shim escribe
	// la pantalla pixel a pixel (`i.pixeles[j] = v`) y el PVR guarda su VRAM como memoria de bloques: si
	// la VRAM no aceptara los accesos de 8 bits, esto lo dice ACA, con un numero, y no dentro del motor
	// (donde el sintoma es una excepcion del SH-4 sin pila que leer: "Stack Trace: frame pointers not
	// enabled!"). Se prueba sobre los primeros 64 KB y no sobre los 1,92 MB para no cobrarle al arranque
	// una vuelta completa de VRAM que no mide nada nuevo.
	nPrueba := 65536
	if nPrueba > n {
		nPrueba = n
	}

	println("PVR-FRAMEBUFFER escribiendo", nPrueba, "bytes en la VRAM...")
	for i := 0; i < nPrueba; i++ {
		pix[i] = byte(i ^ 0x5a)
	}

	println("PVR-FRAMEBUFFER escritos, leyendo de vuelta...")

	distintos := 0
	for i := 0; i < nPrueba; i++ {
		if pix[i] != byte(i^0x5a) {
			distintos++
		}
	}

	println("PVR-FRAMEBUFFER ida y vuelta:", nPrueba, "bytes, distintos:", distintos)

	return &Image{pixeles: pix, ancho: w, alto: h}
}

// VramDelFramebuffer: cuantos bytes de VRAM ocupa la pantalla logica del motor (0 = vive en el monton).
func VramDelFramebuffer() int { return d2cPvrPantallaFbBytes() }

// MemLibreDelMonton: cuantos bytes libres tiene el monton de C. Es el mismo numero que el runtime de la
// consola imprime como `free=` cuando se queda sin memoria: sirve para medir la palanca de RAM de esta
// tarjeta sin tener que provocar el OOM.
func MemLibreDelMonton() int { return d2cPvrPantallaMemLibre() }

// errorPantalla: el error del backend de video.
type errorPantalla string

func (e errorPantalla) Error() string { return string(e) }

// itoa: un entero a texto.
//
// OJO, ACÁ HAY UN DEFECTO DEL COMPILADOR, Y ESTÁ ANOTADO PORQUE PARECE UN ERROR DEL CÓDIGO: la primera
// versión armaba el número a mano en un array local (`var buf [20]byte`) y GCCGO SE CAÍA COMPILANDO
// ("internal compiler error: in fill_in_array, at go/go-gcc.cc:1184"), sin decir nada del código. No era el
// array: era gccgo. Se usa strconv (que ya está probado en la consola) y el defecto del compilador deja de
// importar. Si alguien lo ve de nuevo en OTRO paquete, no es su código: es gccgo con los arrays locales.
func itoa(n int) string { return strconv.Itoa(n) }

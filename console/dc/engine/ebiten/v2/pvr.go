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

// La pantalla es 640x480: es el modo de video que se elige (DM_640x480 con RGB565).
const (
	PantallaAncho = 640
	PantallaAlto  = 480
)

var pantallaIniciada bool

// IniciarPantalla: arranca el video de la consola y deja lista la textura de la pantalla. Se llama una sola
// vez, antes del loop.
//
// LOS ERRORES SE DEVUELVEN Y NO SE IGNORAN: si no se pudo reservar la VRAM, lo unico que se veria despues
// es una pantalla negra, y desde afuera eso es indistinguible de "el juego anda pero no dibuja nada". Con
// el error, el que llama puede decirlo.
func IniciarPantalla(ancho, alto int) error {
	if pantallaIniciada {
		return nil
	}

	if r := d2cPvrPantallaInit(ancho, alto); r != 0 {
		return errorPantalla("no se pudo arrancar la pantalla del PVR (codigo " + itoa(r) + ")")
	}

	pantallaIniciada = true
	return nil
}

// PantallaIniciada: si el video arranco.
func PantallaIniciada() bool { return pantallaIniciada }

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

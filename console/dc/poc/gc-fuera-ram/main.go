//go:build dreamcast

// SONDA-GC92 -- EL BARRIDO DEL GC LEYENDO FUERA DE RAM, EN 20 LINEAS Y SIN EL MOTOR.
//
// POR QUE EXISTE (DC-002.87c, medido)
// Los MISMOS `src/` del POC cooperativo con 1.440 B mas de sonda pintan 360 cuadros y sin la sonda el mismo
// codigo muere: H con `jsr @r9` dentro de `gc_scan_gcdata_bitmap` y el PC en `0x3fd17b4f` (fuera de RAM), I
// clavado 30 s en `gc_update_pointer_field` / `gc_scan_range_conservative`, E reinicia la consola. Lo que
// decide entre morir y pintar es el ACOMODO, o sea que palabra de basura caiga donde.
//
// LO QUE MIDE ESTA SONDA. El motor no hace falta para nada: el defecto esta en la relacion entre el
// ENCABEZADO de un objeto y el DESCRIPTOR DE TIPO que ese encabezado nombra. La sonda arma esa relacion a
// mano, con un solo objeto y dos punteros:
//
//   1. un objeto vivo de verdad, alcanzable desde una raiz global (`ancla`), asi el barrido lo visita;
//   2. un "descriptor de tipo" FALSO, de 64 B, fuera del monton de Go (`malloc`), con la forma que el runtime
//      NO valida: `__size = 4` y `__ptrdata = 0x01000000` (16 MB), y un `__gcdata` que tambien es memoria de
//      malloc, llena de 0xFF (todas las palabras "son punteros");
//   3. el encabezado del objeto se falsifica: su campo `type` pasa a apuntar al descriptor falso.
//
// A partir de ahi `runtime.GC()` visita el objeto y hace lo que le diga el descriptor falso: `__ptrdata` de
// 16 MB son 4.194.304 palabras desde el objeto. Eso es leer fuera de RAM (medido con la sonda del runtime:
// `GC-FUERA-RAM-obj`, `-palabras`, `-gcdata`, `-barra`).
//
// LOS DOS DESENLACES (es el criterio de la tarjeta, y se leen en el log)
//   con el arreglo (scripts/console/fix-libgodc-tipos.py):  `gc-tipo-fuera-imagen` -- el descriptor no esta
//       en la imagen (los descriptores son datos COMPILADOS, viven entre 0x8c010000 y `_end`), el encabezado
//       es falso, el objeto se barre conservador y acotado a su tamano. CERO lineas `GC-FUERA-RAM`.
//   con el mutante (`fix-libgodc-tipos.py --quitar`): el chequeo viejo (`0x8c000000 <= tipo < 0x8d000000`)
//       acepta el descriptor falso porque el monton tambien es RAM, y el barrido se va de RAM: sale
//       `GC-FUERA-RAM-barra` con una direccion mayor que el tope de RAM.
//
// La salida va por la consola serie con el prefijo SONDA-GC92 (y las marcas del runtime con ARQ-MARCA).
package main

import "runtime"

//extern d2c_serial_init
func d2cSerialInit()

//extern d2c_ms
func d2cMs() uint

//extern d2c_usado
func d2cUsado() int

//extern d2c_sbrk
func d2cSbrk() uint

//extern d2c_techo
func d2cTecho() uint

//extern d2c_pila
func d2cPila() uint

//extern d2c_malloco
func d2cMalloco(n uint, relleno int) uint

//extern d2c_poner32
func d2cPoner32(addr uint, off uint, val uint)

//extern d2c_leer32
func d2cLeer32(addr uint, off uint) uint

//extern d2c_header_de
func d2cHeaderDe(p uint) uint

//extern d2c_tamano_header
func d2cTamanoHeader(p uint) uint

//extern d2c_tipo_header
func d2cTipoHeader(p uint) uint

//extern d2c_falsificar_tipo
func d2cFalsificarTipo(p uint, tipo uint)

//extern d2c_addr
func d2cAddr(p *nodo) uint

// nodo es el objeto vivo. Tiene punteros a proposito: es lo que el barrido tiene que seguir (`a` y `b`).
type nodo struct {
	a, b *byte
	n    int
}

var ancla *nodo // raiz global: el barrido tiene que llegar al objeto por aca

func main() {
	d2cSerialInit()

	println("SONDA-GC92: arranca (el barrido del GC fuera de RAM, DC-002.92)")
	println("SONDA-GC92: runtime", runtime.GOOS, runtime.GOARCH)

	// ---- el mapa de secciones, delante de todo numero ----
	println("SONDA-GC92: mapa: imagen 0x8c010000..0x", int64(d2cSbrk()),
		" (el break del monton, leido en el arranque) techo=0x", int64(d2cTecho()), " pila~0x", int64(d2cPila()))
	println("SONDA-GC92: mapa: el HEAP y la PILA viven dentro de 0x8c000000..0x8d000000, igual que la imagen:")
	println("SONDA-GC92: mapa: por eso '0x8c000000 <= puntero < 0x8d000000' NO distingue un descriptor de tipo")

	// ---- 1. el objeto vivo, alcanzable desde una raiz ----
	x := byte(0x5a)
	y := byte(0xa5)
	ancla = &nodo{a: &x, b: &y, n: 12345}
	dir := d2cAddr(ancla)

	println("SONDA-GC92: objeto vivo en 0x", int64(dir),
		" encabezado=0x", int64(d2cHeaderDe(dir)),
		" size_flags=0x", int64(d2cTamanoHeader(dir)),
		" tipo=0x", int64(d2cTipoHeader(dir)))

	// ---- 2. el descriptor FALSO y su gcdata, fuera del monton de Go ----
	// 64 B (el descriptor) + 64 B (el mapa de bits): malloc, no monton de Go, asi que no se mueven.
	falso := d2cMalloco(64, 0)
	bitmap := d2cMalloco(64, 0xff) // todas las palabras son "punteros"

	d2cPoner32(falso, 0, 4)          // __size   = 4
	d2cPoner32(falso, 4, 0x01000000) // __ptrdata= 16 MB  -> 4.194.304 palabras
	d2cPoner32(falso, 20, bitmap)    // __gcdata = memoria de malloc (en RAM, no en la imagen)
	d2cPoner32(falso, 15, 0)         // __code   = 0 (no es GC prog: el camino es el barrido preciso)

	println("SONDA-GC92: descriptor FALSO en 0x", int64(falso), " size=", int64(d2cLeer32(falso, 0)),
		" ptrdata=", int64(d2cLeer32(falso, 4)), " gcdata=0x", int64(d2cLeer32(falso, 20)))
	println("SONDA-GC92: palabras que pide ese ptrdata=", 0x01000000/4,
		" bytes de barrido desde el objeto=", 0x01000000)

	// ---- 3. el encabezado del objeto pasa a nombrar el descriptor falso ----
	d2cFalsificarTipo(dir, falso)
	println("SONDA-GC92: encabezado falsificado: tipo=0x", int64(d2cTipoHeader(dir)),
		" (antes apuntaba a la imagen)")

	// ---- 4. la recoleccion: el barrido tiene que visitar el objeto ----
	for ronda := 1; ronda <= 3; ronda++ {
		t0 := int64(d2cMs())
		runtime.GC()
		println("SONDA-GC92: ronda=", ronda, " gc_ms=", int64(d2cMs())-t0,
			" usado=", d2cUsado(), " vivo=", estado())
	}

	// ---- 5. el veredicto ----
	println("SONDA-GC92: veredicto: el objeto sigue en 0x", int64(d2cAddr(ancla)),
		" n=", ancla.n, " a=", int(*(ancla.a)), " b=", int(*(ancla.b)))
	println("SONDA-GC92: si arriba hay lineas GC-FUERA-RAM, el barrido se salio de RAM.")
	println("SONDA-GC92: si arriba hay gc-tipo-fuera-imagen y NINGUNA GC-FUERA-RAM, el arreglo esta puesto.")
	println("SONDA-GC92: fin")
}

// estado: foto del objeto por el lado de Go, para ver si el barrido lo dejo usable.
func estado() string {
	if ancla == nil || ancla.a == nil || ancla.b == nil {
		return "ROTO (puntero nulo)"
	}

	if ancla.n != 12345 || *(ancla.a) != 0x5a || *(ancla.b) != 0xa5 {
		return "ROTO (datos cambiados)"
	}

	return "entero"
}

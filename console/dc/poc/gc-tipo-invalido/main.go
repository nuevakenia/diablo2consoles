//go:build dreamcast

// SONDA-GC146D -- EL RECOLECTOR BARRIENDO UN TIPO CON `__ptrdata` BASURA, EN 90 LINEAS Y SIN EL MOTOR.
//
// POR QUE EXISTE (DC-002.146d, medido en DC-002.145)
// La firma del defecto que hacia intermitente el `panic: slice bounds out of range` del cruce eran dos marcas del
// recolector de libgodc con valores que son DIRECCIONES, no tamanos:
//
//     ARQ-MARCA gc-ptrdata-recortado    v=2349166024 (0x8c056dc8, una palabra de .rodata)
//     ARQ-MARCA gc-gcdata-fuera-imagen  v= 906232840 (0x36040408, basura fuera de RAM)
//
// DC-002.92 ya exigio que el tipo CAIGA EN LA IMAGEN, y eso dejo pasar el caso: la imagen es texto+datos+bss y
// adentro tambien hay codigo, cadenas y tablas. Esta sonda arma ESE caso a mano, con un solo objeto:
//
//   1. un objeto vivo de verdad (`ancla`), alcanzable desde una raiz global: el barrido lo tiene que visitar;
//   2. un descriptor FALSO que SI cae en la imagen: es un global de Go (`.bss`), asi que su direccion esta entre
//      0x8c010000 y el break del monton -- exactamente el rango que `gc_tipo_en_imagen` acepta. Sus campos son los
//      del defecto: `__ptrdata` = una direccion de la imagen (no un tamano) y `__gcdata` = 0x36040408 (fuera de RAM);
//   3. el encabezado del objeto pasa a nombrar ese descriptor.
//
// LOS DOS DESENLACES (esto es el criterio de la tarjeta, y se lee en el log)
//   con el arreglo (scripts/console/fix-libgodc-tipos-reales.py): el tipo no esta en la TABLA de descriptores que
//       el compilador le paso al asignador (`gc-tipos-registrados`), asi que es un encabezado falso: sale
//       `gc-tipo-invalido` y CERO `gc-ptrdata-recortado` / `gc-gcdata-fuera-imagen` (no se lee su __ptrdata).
//       El objeto de CONTROL -- sano, con su tipo registrado -- tiene que salir ENTERO: la tabla no rompe lo que anda.
//   con el mutante (`fix-libgodc-tipos-reales.py --quitar`, que es el arbol de DC-002.92): el tipo cae en la imagen
//       y pasa el chequeo viejo, el barrido lee `__ptrdata` como 0x8c0....., sale `gc-ptrdata-recortado` con esa
//       direccion y `gc-gcdata-fuera-imagen`. Es el defecto, en la misma corrida y sin suerte de layout.
//
// La salida va por la consola serie con el prefijo SONDA-GC146D (y las marcas del runtime con ARQ-MARCA).
package main

import "runtime"

//extern d2c_serial_init
func d2cSerialInit()

//extern d2c_ms
func d2cMs() uint

//extern d2c_sbrk
func d2cSbrk() uint

//extern d2c_usado
func d2cUsado() int

//extern d2c_dir
func d2cDir(p *descriptor) uint

//extern d2c_header_de
func d2cHeaderDe(p uint) uint

//extern d2c_tipo_header
func d2cTipoHeader(p uint) uint

//extern d2c_falsificar_tipo
func d2cFalsificarTipo(p uint, tipo uint)

//extern d2c_addr
func d2cAddr(p *nodo) uint


// descriptor es el `struct __go_type_descriptor` de gccgo en 32 bits (36 B), campo por campo, para poder escribir
// a mano los del defecto. OJO: el offset de cada campo es el de libgodc/runtime/type_descriptors.h.
type descriptor struct {
	size, ptrdata, hash        uint32
	tflag, align, falign, code uint8
	equalfn, gcdata            uint32
	reflection, uncommon       uint32
	pointerToThis              uint32
}

// nodo es el objeto vivo: tiene punteros a proposito (es lo que el barrido tendria que seguir).
type nodo struct {
	a, b *byte
	n    int
}

var ancla *nodo   // el objeto al que se le falsifica el tipo (raiz global)
var control *nodo // el objeto SANO: el que mide que la tabla no rompe lo que anda (raiz global)

// falso es el descriptor que NO es: vive en la imagen (es un global de Go) y tiene los campos del defecto.
var falso descriptor

func main() {
	d2cSerialInit()

	println("SONDA-GC146D: arranca (el recolector barriendo un tipo con __ptrdata basura, DC-002.146d)")
	println("SONDA-GC146D: runtime", runtime.GOOS, runtime.GOARCH)
	println("SONDA-GC146D: imagen 0x8c010000..0x", int64(d2cSbrk()), " (el break del monton en el arranque)")
	// La cantidad de tipos que el asignador tiene anotados NO se pregunta por Go: la dice el recolector en el log
	// (`ARQ-MARCA gc-tipos-registrados v=...`), marca que en la contra-prueba no existe porque no hay tabla.

	// ---- 1. los dos objetos vivos ----
	x := byte(0x5a)
	y := byte(0xa5)
	a := byte(0x11)
	b := byte(0x22)

	ancla = &nodo{a: &x, b: &y, n: 12345}
	control = &nodo{a: &a, b: &b, n: 777}

	dirAncla := d2cAddr(ancla)
	dirControl := d2cAddr(control)

	println("SONDA-GC146D: objeto ANCLA en 0x", int64(dirAncla), " tipo=0x", int64(d2cTipoHeader(dirAncla)),
		" n=", ancla.n)
	println("SONDA-GC146D: objeto CONTROL en 0x", int64(dirControl), " tipo=0x", int64(d2cTipoHeader(dirControl)),
		" n=", control.n)

	// ---- 2. el descriptor falso, DENTRO DE LA IMAGEN ----
	falso.size = 16
	falso.ptrdata = uint32(d2cSbrk() - 0x10000) // una DIRECCION de la imagen leida como si fuera un tamano
	falso.hash = 0
	falso.tflag = 0
	falso.align = 4
	falso.falign = 4
	falso.code = 0                       // sin Kind: el camino seria el barrido preciso
	falso.equalfn = 0
	falso.gcdata = 0x36040408            // el valor medido del defecto: fuera de RAM
	falso.reflection = 0
	falso.uncommon = 0
	falso.pointerToThis = 0

	dirFalso := d2cDir(&falso)

	enLaImagen := dirFalso >= 0x8c010000 && dirFalso < d2cSbrk()

	println("SONDA-GC146D: descriptor FALSO en 0x", int64(dirFalso), " EN LA IMAGEN=", enLaImagen,
		" size=", int64(falso.size), " ptrdata=0x", int64(falso.ptrdata), " gcdata=0x", int64(falso.gcdata))
	println("SONDA-GC146D: si EN LA IMAGEN= true, el chequeo de DC-002.92 ('el tipo cae en la imagen') lo ACEPTA")

	// ---- 3. el encabezado del ANCLA pasa a nombrar el descriptor falso; el CONTROL queda sano ----
	tipoOriginal := d2cTipoHeader(dirAncla)

	d2cFalsificarTipo(dirAncla, dirFalso)

	println("SONDA-GC146D: encabezado del ANCLA falsificado: tipo 0x", int64(tipoOriginal), "-> 0x",
		int64(d2cTipoHeader(dirAncla)))

	// ---- 4. tres recolecciones: el barrido tiene que visitar los dos objetos ----
	for ronda := 1; ronda <= 3; ronda++ {
		t0 := int64(d2cMs())

		runtime.GC()

		println("SONDA-GC146D: ronda=", ronda, " gc_ms=", int64(d2cMs())-t0, " usado=", d2cUsado(),
			" control=", estadoControl(), " ancla_n=", ancla.n)
	}

	// ---- 5. el veredicto (lo que se lee de la consola) ----
	dirAnclaAhora := d2cAddr(ancla)

	println("SONDA-GC146D: veredicto: ANCLA en 0x", int64(dirAnclaAhora), " n=", ancla.n,
		" tipo=0x", int64(d2cTipoHeader(dirAnclaAhora)))
	println("SONDA-GC146D: veredicto: CONTROL", estadoControl(), "-- con la tabla de tipos tiene que salir ENTERO")
	println("SONDA-GC146D: si arriba hay gc-tipo-invalido y NINGUNA gc-ptrdata-recortado / gc-gcdata-fuera-imagen,")
	println("SONDA-GC146D:   el recolector no barrio el tipo de basura (el arreglo esta puesto).")
	println("SONDA-GC146D: si salen gc-ptrdata-recortado y gc-gcdata-fuera-imagen, barrio con el __ptrdata basura.")
	println("SONDA-GC146D: fin")
}

// estadoControl: el objeto sano, leido por el lado de Go. Es la contra de que la tabla no pierde lo que anda.
func estadoControl() string {
	if control == nil || control.a == nil || control.b == nil {
		return "ROTO (puntero nulo)"
	}

	if control.n != 777 || *(control.a) != 0x11 || *(control.b) != 0x22 {
		return "ROTO (datos cambiados)"
	}

	return "entero"
}

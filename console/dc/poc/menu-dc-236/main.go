//go:build dreamcast

// DC-002.36 -- EL MENU PRINCIPAL dibujado y navegable en la Dreamcast.
//
// LA CADENA QUE CORRE EN LA CONSOLA (el camino que despues usan todos los menus):
//   disco (/cd/MENU, ISO9660) -> los DC6 CRUDOS del juego -> DECODIFICADOR DEL MOTOR (d2dc6, el mismo
//   codigo que corre en el host) -> lienzo de indices de 8 bits por sprite -> textura + BANCO DE PALETA
//   del PVR (3 paletas del juego por hardware) -> quads en su posicion, con transparencia y blending
//   -> pantalla 640x480.
//
// QUE HACE LA CONSOLA Y QUE NO (decision de la tarjeta, escrita para que no se confunda con "el motor"):
//   - la consola LEE, DECODIFICA, COMPONE, SUBE y DIBUJA: es lo que se esta probando;
//   - el PLAN DE DIBUJO (que textura, en que posicion, que pedazo, que frame por estado de boton) llega
//     como DATOS (PLAN.TXT): lo arma el HOST ejecutando el codigo del motor (`d2gamescreen/main_menu.go`
//     + d2ui/button.go + d2ui/label.go), porque el arranque del motor entero todavia no llega a la
//     pantalla (muere en el monton de C: DC-002.41). El formato del plan lo fija el HOST con su propio
//     codigo; aca se verifica contra lo que traen los DC6 del disco (si no coincide, se avisa y no se
//     dibuja), que es lo que impide que el plan mienta.
//   - la NAVEGACION si corre aca: el control de la consola (maple, el mismo camino de DC-002.11) mueve
//     la seleccion entre los 7 botones y el boton seleccionado se dibuja con su frame APRETADO
//     (d2ui.Button: frames 0/1 normal, 2/3 apretado).
//
// LO QUE IMPRIME (la consola serie es la unica salida): el CRC-32 de cada lienzo de indices (el numero
// que el host tiene que reproducir), los cuadros, los cambios de seleccion y la VRAM.
//
// MUTANTE (criterio 3 de la tarjeta): con un asset que no existe (lo hace el script del corpus), el
// lienzo queda vacio, NO se sube esa textura y la pantalla queda como estaba: se mide en pixeles.
package main

import (
	"errors"
	"hash/crc32"
	"os"
	"runtime"
	"strconv"
	"strings"
	"unsafe"

	"kos"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2dc6"
)

// Los tres errores que puede dar el armado de un lienzo. Se escriben con errors.New y no con fmt.Errorf
// (la stdlib de la consola es chica: cuantos menos paquetes, menos cosas que arrastrar al enlace).
var (
	errNoCoincide = errors.New("el asset del disco NO coincide con lo que dice el plan")
	errTipo       = errors.New("tipo de textura desconocido")
	errFirma      = errors.New("formato inesperado (firma/tamano)")
)

//extern d2c_serial_init
func d2cSerialInit()

//extern d2c_ms
func d2cMs() uint

//extern d2c_vram_libre
func d2cVramLibre() uint

//extern d2c_pal
func d2cPal(banco int, entradas uint)

//extern d2c_tex
func d2cTex(id int, idx uint, w, h, pw, ph, banco int) int

//extern d2c_quad
func d2cQuad(id int, x, y, w, h, u0, v0, u1, v1 float32, argb uint, modo int)

//extern d2c_tex_bytes
func d2cTexBytes() uint

//extern d2c_control_hay
func d2cControlHay() int

//extern d2c_control_todo
func d2cControlTodo(todos uint) int

// dirMenu: donde estan los assets del menu EN EL DISCO. Los arma mkdisco-236.sh (los DC6 crudos + el plan +
// las cadenas).
//
// OJO, MEDIDO EN DC-002.36: el disco con los archivos en la CARPETA /MENU **no arranca** en Flycast con este
// binario (0 marcas del constructor de C de libgodc: el arranque ni llega al binario; ver la seccion del
// README), y el MISMO binario con los 20 archivos en la RAIZ del ISO arranca y dibuja. Por eso mkdisco los
// manda a la raiz (SIN_CARPETA=1) y aca se busca primero la carpeta, que es el disco "lindo", y despues la
// raiz, que es el que arranca. El que decide es el disco: se prueba leyendo el PLAN.
var dirMenu = "/cd/MENU/"

// elegirDirDeAssets: deja dirMenu apuntando a donde de verdad estan los assets.
//
// Se prueban los tres lugares, en orden: el ROMDISK (/rd, los archivos viajan adentro del ELF),
// la carpeta del disco (/cd/MENU/) y la raiz del disco (/cd/). El que decide es el disco/ELF que
// se arranco: se prueba leyendo el PLAN.
func elegirDirDeAssets() {
	for _, d := range []string{"/rd/", "/cd/MENU/", "/cd/"} {
		if _, err := os.ReadFile(d + "PLAN.TXT"); err == nil {
			dirMenu = d
			return
		}
	}

	println("MENU236: AVISO no encontre PLAN.TXT ni en /rd/ ni en /cd/MENU/ ni en /cd/")
}

const (
	// maxTex: la tabla de texturas del shim de C (MAX_TEX).
	maxTex = 16

	// Botones del control de la Dreamcast que usa la navegacion (medido en DC-002.11: la cruceta son
	// las flechas del teclado; A es CONT_A).
	botonArriba = 16
	botonAbajo  = 64

	// El plano logico del motor es 800x600 y la pantalla de la consola 640x480: todo se dibuja a 4/5.
	escalaNum, escalaDen = 4, 5
)

// textura: un lienzo de indices de 8 bits, del tamano que dice el plan (y potencia de dos para el PVR).
type textura struct {
	id     int
	nombre string
	tipo   string
	arch   string
	w, h   int
	pw, ph int
	banco  int
	idx    []byte
	subida int
}

// op: un quad del plan (coordenadas LOGICAS de 800x600; aca se escalan 4/5).
type op struct {
	tex            int
	x, y, w, h     int
	u0, v0, u1, v1 int
	argb           uint
	modo           int
	cond           int
}

// glifo: lo que la tabla .tbl dice de una letra: que frame del atlas es y cuanto avanza el cursor.
type glifo struct {
	frame int
	ancho int
}

func main() {
	d2cSerialInit()

	println("MENU236: arranca (DC-002.36, el MENU PRINCIPAL del disco, decodificado por el motor y pintado por el PVR)")
	println("MENU236: runtime", runtime.GOOS, runtime.GOARCH)

	elegirDirDeAssets()
	println("MENU236: los assets del disco estan en", dirMenu)

	tTexturas := int64(d2cMs())

	texturas, ops, err := leerPlan(dirMenu + "PLAN.TXT")
	if err != nil {
		println("MENU236: PLAN FALLO:", err.Error())

		return
	}

	println("MENU236: PLAN texturas=", len(texturas), "quads=", len(ops), "ms=", int64(d2cMs())-tTexturas)

	// Chequeo del plan RECIEN LEIDO (sin haber decodificado nada): mide si los campos ya vienen mal del
	// parseo o si se corrompen despues. Es el instrumento del defecto medido: la textura 0 (el FONDO)
	// llega con `ph` y `banco` basura (un valor con pinta de puntero) mientras las otras 10 estan bien.
	for _, t := range texturas {
		println("MENU236: PLAN-LEIDO id=", t.id, "w=", t.w, "h=", t.h, "pw=", t.pw, "ph=", t.ph, "banco=", t.banco)
	}

	// --- 1. el PVR y las paletas ANTES de los lienzos ---
	// El orden importa y es lo que hace que el MENU COMPLETO entre en la RAM: se lee una textura, se
	// sube a la VRAM y se SUELTA su lienzo (`t.idx = nil`) antes de la siguiente. Antes se armaban los
	// 11 lienzos primero y se subian despues: los 2,3 MB de lienzos vivos a la vez + los 1,4 MB de los
	// DC6 crudos llenaban el arena de KOS (medido: `Memory: arena=4148384 used=4144792 free=3592`) y el
	// POC moria con `panic: index out of range` al cargar la 9na textura (FUENTE_FORMAL10). La paleta no
	// depende de las texturas, asi que puede ir antes.
	kos.PvrInitDefaults()
	kos.PvrSetBgColor(0.0, 0.0, 0.0)
	kos.PvrWaitReady()

	println("MENU236: VRAM libre antes=", int64(d2cVramLibre()))

	for banco, arch := range []string{"PAL_SKY.PAL", "PAL_UNITS.PAL", "PAL_STATIC.PAL"} {
		pal, err := leerPaleta(dirMenu + arch)
		if err != nil {
			println("MENU236: PALETA FALLO banco=", banco, arch, err.Error())

			continue
		}

		println("MENU236: PALETA banco=", banco, arch, "crc32=", int64(crc32IEEE(palBytes(pal))))
		d2cPal(banco, dirDeUint16(pal))
	}

	// --- 2. una textura por vuelta: se lee del romdisk, se decodifica, se sube y se suelta ---
	faltan := 0
	subidas := 0

	for _, t := range texturas {
		if t.tipo == "fuente" {
			err = armarFuente(t)
		} else {
			err = armarDC6(t)
		}

		if err != nil {
			println("MENU236: TEXTURA FALLO id=", t.id, t.nombre, err.Error())
			faltan++

			continue
		}

		println("MENU236: TEXTURA id=", t.id, t.nombre, "tipo=", t.tipo, "tam=", t.w, "x", t.h,
			"pot=", t.pw, "x", t.ph, "banco=", t.banco, "crc32=", int64(crc32IEEE(t.idx)))

		rc := d2cTex(t.id, dirDeBytes(t.idx), t.w, t.h, t.pw, t.ph, t.banco)
		if rc != 0 {
			println("MENU236: SUBIR FALLO id=", t.id, "codigo=", rc)

			continue
		}

		t.subida = 1
		subidas++

		// EL LIENZO YA ESTA EN LA VRAM: se suelta. Es lo que baja el pico de RAM de ~3,7 MB a menos de 1.
		t.idx = nil
	}

	println("MENU236: LIENZOS listos ms=", int64(d2cMs())-tTexturas, "faltan=", faltan)
	println("MENU236: TEXTURAS subidas=", subidas, "bytes=", int64(d2cTexBytes()),
		"vram_libre=", int64(d2cVramLibre()))

	// --- 3. el cuadro ---
	// La seleccion arranca en -1 (como el motor: `focus: -1` en CreateMainMenu) y la cruceta la mueve.
	foco := -1
	botonesPrevios := uint(0)
	cuadros := 0

	println("MENU236: FOCO inicial=", foco, " (arriba/abajo del control para mover la seleccion)")

	for {
		kos.PvrWaitReady()
		kos.PvrSceneBegin()
		kos.PvrListBegin(kos.PVR_LIST_TR_POLY)

		dibujados := 0

		for i := range ops {
			o := &ops[i]

			if o.cond != 0 {
				boton := (o.cond >> 1) - 1
				estado := o.cond & 1
				apretado := boton == foco

				if (estado == 1) != apretado {
					continue
				}
			}

			if o.tex < 0 || o.tex >= len(texturas) || texturas[o.tex].subida != 1 {
				continue
			}

			// el escalado por BORDES (no por tamano): asi los trozos de un boton quedan pegados.
			x0 := float32(o.x * escalaNum / escalaDen)
			x1 := float32((o.x + o.w) * escalaNum / escalaDen)
			y0 := float32(o.y * escalaNum / escalaDen)
			y1 := float32((o.y + o.h) * escalaNum / escalaDen)

			d2cQuad(o.tex, x0, y0, x1-x0, y1-y0,
				float32(o.u0), float32(o.v0), float32(o.u1), float32(o.v1), o.argb, o.modo)

			dibujados++
		}

		kos.PvrListFinish()
		kos.PvrSceneFinish()

		cuadros++

		if cuadros == 60 {
			println("MENU236: CUADROS 60 dibujados=", dibujados, "vram_libre=", int64(d2cVramLibre()))
		}

		// --- la navegacion: el control de la consola (maple), flanco de bajada ---
		if botones, ok := leerBotones(); ok {
			if botones&botonAbajo != 0 && botonesPrevios&botonAbajo == 0 {
				foco = moverFoco(foco, len(botonesDelMenu), 1)
				println("MENU236: FOCO valor=", foco, "cuadro=", cuadros, " tecla=abajo")
			}

			if botones&botonArriba != 0 && botonesPrevios&botonArriba == 0 {
				foco = moverFoco(foco, len(botonesDelMenu), -1)
				println("MENU236: FOCO valor=", foco, "cuadro=", cuadros, " tecla=arriba")
			}

			botonesPrevios = botones
		}
	}
}

// botonesDelMenu: los SIETE botones del menu principal en el orden de focusButtons() del motor. Es el
// unico dato de la navegacion que no viaja en el plan: el plan trae la condicion por boton, y el orden
// es el del codigo del motor (d2gamescreen/main_menu.go, focusButtons).
var botonesDelMenu = []int{0, 1, 2, 3, 4, 5, 6}

// moverFoco: lo mismo que hace el motor (d2player.NextNPCMenuRow): da la vuelta por los dos extremos.
func moverFoco(foco, total, delta int) int {
	n := foco + delta
	if n < 0 {
		n = total - 1
	}

	if n >= total {
		n = 0
	}

	return n
}

// leerBotones: el estado del control del puerto 0 (el camino de DC-002.11, el mismo que usa el motor).
// Devuelve ok=false si no hay control: sin control el menu se ve igual, solo no se mueve la seleccion.
func leerBotones() (uint, bool) {
	if d2cControlHay() == 0 {
		return 0, false
	}

	// los botones se reciben en un arreglo de int32 y se copian a un uint: pasar la direccion de un
	// entero suelto obligaria a un unsafe.Pointer en un campo, que el runtime de la consola no soporta.
	var ctrl [1]int32

	if d2cControlTodo(uint(dirDeInt32(&ctrl[0]))) != 0 {
		return 0, false
	}

	return uint(ctrl[0]), true
}

// leerPlan: PLAN.TXT (lo escribe el host). Dos tipos de linea:
//   T <id> <tipo> <archivo> <w> <h> <pw> <ph> <banco> <nombre>
//   O <tex> <x> <y> <w> <h> <u0> <v0> <u1> <v1> <argb> <modo> <cond>
func leerPlan(ruta string) ([]*textura, []op, error) {
	datos, err := os.ReadFile(ruta)
	if err != nil {
		return nil, nil, err
	}

	texturas := make([]*textura, 0, maxTex)
	ops := make([]op, 0, 512)

	for _, linea := range strings.Split(string(datos), "\n") {
		linea = strings.TrimSpace(linea)
		if len(linea) < 2 {
			continue
		}

		campos := strings.Fields(linea)

		switch campos[0] {
		case "#":
			continue

		case "T":
			if len(campos) < 9 {
				continue
			}

			t := &textura{
				id:     aEntero(campos[1]),
				tipo:   campos[2],
				arch:   campos[3],
				w:      aEntero(campos[4]),
				h:      aEntero(campos[5]),
				pw:     aEntero(campos[6]),
				ph:     aEntero(campos[7]),
				banco:  aEntero(campos[8]),
				nombre: campos[9],
			}

			texturas = append(texturas, t)

		case "O":
			if len(campos) < 12 {
				continue
			}

			ops = append(ops, op{
				tex:  aEntero(campos[1]),
				x:    aEntero(campos[2]),
				y:    aEntero(campos[3]),
				w:    aEntero(campos[4]),
				h:    aEntero(campos[5]),
				u0:   aEntero(campos[6]),
				v0:   aEntero(campos[7]),
				u1:   aEntero(campos[8]),
				v1:   aEntero(campos[9]),
				argb: uint(aEnteroLargo(campos[10])),
				modo: aEntero(campos[11]),
				cond: aEntero(campos[12]),
			})
		}
	}

	return texturas, ops, nil
}

// armarDC6: los tres tipos que salen de un DC6 crudo del disco.
//
//	logo:  el frame 0 (los logos de fuego/negro son animaciones de 30 frames: el POC dibuja el 0)
//	boton: el lienzo de d2ui.Button: los XSegments*YSegments trozos en una FILA por estado (0 normal,
//	       1 apretado); el ancho es la suma de los trozos.
//	fondo: la grilla 4x3 del RenderSegmented(screen, 4, 3, 0) del motor. Los 12 frames se pegan en
//	       orden de fila y se EXIGE que los anchos de las columnas sumen w y los altos de las filas h.
func armarDC6(t *textura) error {
	raw, err := os.ReadFile(dirMenu + t.arch)
	if err != nil {
		return err
	}

	dc, err := d2dc6.Load(raw)
	if err != nil {
		return err
	}

	switch t.tipo {
	case "logo":
		f := dc.Frames[0]
		if int(f.Width) != t.w || int(f.Height) != t.h {
			return errNoCoincide
		}

		t.idx = make([]byte, t.w*t.h)
		copy(t.idx, dc.DecodeFrame(0))

	case "boton":
		// cuantos trozos trae una fila (el ancho del primer frame define el corte)
		ancho0 := int(dc.Frames[0].Width)
		filas := 2 // estado normal y apretado
		alto0 := int(dc.Frames[0].Height)

		if alto0*filas != t.h {
			return errNoCoincide
		}

		t.idx = make([]byte, t.w*t.h)

		for estado := 0; estado < filas; estado++ {
			x0 := 0
			seg := 0

			for x0 < t.w {
				i := estado*2 + seg
				if i >= len(dc.Frames) {
					i = estado*2 + seg - 2
				}

				f := dc.Frames[i]
				datos := dc.DecodeFrame(i)
				fw, fh := int(f.Width), int(f.Height)

				for y := 0; y < fh; y++ {
					for x := 0; x < fw; x++ {
						if x0+x < t.w {
							t.idx[(estado*alto0+y)*t.w+x0+x] = datos[y*fw+x]
						}
					}
				}

				x0 += fw
				seg++
			}

			_ = ancho0
		}

	case "fondo":
		const cols, filas = 4, 3

		if len(dc.Frames) < cols*filas {
			return errNoCoincide
		}

		var altoFila [filas]int
		var anchoCol [cols]int

		for i := 0; i < cols*filas; i++ {
			f := dc.Frames[i]
			col, fila := i%cols, i/cols

			anchoCol[col] = int(f.Width)

			if int(f.Height) > altoFila[fila] {
				altoFila[fila] = int(f.Height)
			}
		}

		sumaW, sumaH := 0, 0

		for _, v := range anchoCol {
			sumaW += v
		}

		for _, v := range altoFila {
			sumaH += v
		}

		if sumaW != t.w || sumaH != t.h {
			println("MENU236: FONDO grilla mide", sumaW, "x", sumaH, "y el plan dice", t.w, "x", t.h)

			return errNoCoincide
		}

		t.idx = make([]byte, t.w*t.h)

		var recortados int

		for fila := 0; fila < filas; fila++ {
			y0 := 0

			for k := 0; k < fila; k++ {
				y0 += altoFila[k]
			}

			x0 := 0

			for col := 0; col < cols; col++ {
				i := col + fila*cols
				f := dc.Frames[i]
				datos := dc.DecodeFrame(i)
				fw, fh := int(f.Width), int(f.Height)

				// EL TOPE EXPLICITO (defecto medido, DC-002.36): `anchoCol[col]` queda con el ancho del
				// ULTIMO frame de la columna, no con el mayor (los altos si usan el mayor). Si un frame
				// anterior de la misma columna es mas ancho, este pegado escribe FUERA del lienzo: medido,
				// la textura 0 (el FONDO) terminaba con `ph`/`banco` basura (valores con pinta de puntero)
				// al salir de armarDC6 y el POC moria al subirla a la VRAM. Con el tope el pegado no sale
				// del lienzo y se cuentan los pixeles que no entraron.
				for y := 0; y < fh; y++ {
					if y0+y >= t.h {
						recortados += fw

						break
					}

					for x := 0; x < fw; x++ {
						if x0+x >= t.w {
							recortados++

							continue
						}

						t.idx[(y0+y)*t.w+x0+x] = datos[y*fw+x]
					}
				}

				x0 += anchoCol[col]
			}
		}

		if recortados > 0 {
			println("MENU236: FONDO la grilla se sale del lienzo en", recortados, "pixeles (tope explicito)")
		}

	default:
		return errTipo
	}

	return nil
}

// armarFuente: el atlas de 256 celdas en grilla 16x16 (16 * potencia de dos de la celda). La TABLA de
// glifos (.tbl) se lee aca con el mismo formato que usa el motor (d2font): firma "Woo!\x01", 12 bytes de
// cabecera y 14 bytes por glifo (codigo uint16, 1 byte, ancho, alto, 3 bytes, frame uint16, 4 bytes).
func armarFuente(t *textura) error {
	raw, err := os.ReadFile(dirMenu + t.arch)
	if err != nil {
		return err
	}

	dc, err := d2dc6.Load(raw)
	if err != nil {
		return err
	}

	rawTBL, err := os.ReadFile(dirMenu + strings.TrimSuffix(t.arch, ".DC6") + ".TBL")
	if err != nil {
		return err
	}

	glifos, err := leerTBL(rawTBL)
	if err != nil {
		return err
	}

	f0 := dc.Frames[0]
	celdaW, celdaH := int(f0.Width), int(f0.Height)
	pasoW, pasoH := sigPot2(celdaW), sigPot2(celdaH)

	if 16*pasoW != t.w || 16*pasoH != t.h {
		println("MENU236: FUENTE atlas", 16*pasoW, "x", 16*pasoH, "y el plan dice", t.w, "x", t.h)

		return errNoCoincide
	}

	t.idx = make([]byte, t.w*t.h)
	pegados := 0

	for _, g := range glifos {
		if g.frame < 0 || g.frame >= len(dc.Frames) {
			continue
		}

		col, fila := g.frame%16, g.frame/16
		datos := dc.DecodeFrame(g.frame)
		fw, fh := int(dc.Frames[g.frame].Width), int(dc.Frames[g.frame].Height)

		// Los frames de una fuente NO miden todos lo mismo (el ancho/alto lo toma del frame 0, y hay
		// glifos mas anchos). Sin esto, un glifo mas ancho que el paso de la grilla escribia afuera del
		// atlas: `panic: index out of range` cargando FUENTE_FORMAL10 (medido: 8 texturas cargadas y el
		// panic en la 9na, con 'free=3816' del arena en el volcado). Se recorta al paso de la grilla.
		if fw > pasoW || fh > pasoH {
			println("MENU236: FUENTE glifo frame=", g.frame, "mide", fw, "x", fh,
				"y la grilla es", pasoW, "x", pasoH, "-> se recorta")
		}

		if fw > pasoW {
			fw = pasoW
		}

		if fh > pasoH {
			fh = pasoH
		}

		for y := 0; y < fh; y++ {
			for x := 0; x < fw; x++ {
				t.idx[(fila*pasoH+y)*t.w+col*pasoW+x] = datos[y*fw+x]
			}
		}

		pegados++
	}

	println("MENU236: FUENTE", t.nombre, "glifos=", len(glifos), "pegados=", pegados,
		"celda=", celdaW, "x", celdaH)

	return nil
}

// leerTBL: la tabla de glifos del motor. Se devuelve por FRAME (que es lo unico que hace falta aca para
// pintar); el ancho de cada glifo y su codigo son la metrica del texto.
func leerTBL(datos []byte) (map[int]glifo, error) {
	const cabecera = 12
	const porGlifo = 14

	if len(datos) < cabecera || string(datos[:5]) != "Woo!\x01" {
		return nil, errFirma
	}

	salida := make(map[int]glifo, 256)

	for i := cabecera; i+porGlifo <= len(datos); i += porGlifo {
		ancho := int(datos[i+3])
		frame := int(datos[i+8]) | int(datos[i+9])<<8

		salida[frame] = glifo{frame: frame, ancho: ancho}
	}

	if len(salida) == 0 {
		return nil, errFirma
	}

	return salida, nil
}

// leerPaleta: pal.dat del juego = 256 triples B,G,R (768 bytes). Se convierte a ARGB1555 con el indice 0
// TRANSPARENTE, que es la regla del motor (d2util: "index zero is hardcoded transparent").
func leerPaleta(ruta string) ([]uint16, error) {
	datos, err := os.ReadFile(ruta)
	if err != nil {
		return nil, err
	}

	if len(datos) < 768 {
		return nil, errFirma
	}

	pal := make([]uint16, 256)

	for i := 0; i < 256; i++ {
		b := uint16(datos[i*3]) >> 3
		g := uint16(datos[i*3+1]) >> 3
		r := uint16(datos[i*3+2]) >> 3

		alpha := uint16(1)
		if i == 0 {
			alpha = 0
		}

		pal[i] = alpha<<15 | r<<10 | g<<5 | b
	}

	return pal, nil
}

func palBytes(pal []uint16) []byte {
	b := make([]byte, len(pal)*2)

	for i, v := range pal {
		b[i*2] = byte(v & 0xff)
		b[i*2+1] = byte(v >> 8)
	}

	return b
}

func crc32IEEE(b []byte) uint32 {
	return crc32.ChecksumIEEE(b)
}

func sigPot2(n int) int {
	p := 1
	for p < n {
		p <<= 1
	}

	return p
}

func aEntero(s string) int {
	v, err := strconv.Atoi(s)
	if err != nil {
		return 0
	}

	return v
}

func aEnteroLargo(s string) int64 {
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0
	}

	return v
}

// dirDeBytes / dirDeUint16 / dirDeInt32: la direccion del primer elemento de un arreglo, para pasarsela
// al C (que la recibe como uint y la convierte a puntero). Se usa un portador uint y NO un unsafe.Pointer
// guardado: el runtime de la consola no tiene los descriptores del tipo unsafe.Pointer y el enlace falla
// ("undefined reference to unsafe.Pointer..p"), asi que solo se convierte en el momento de pasar el dato.
func dirDeBytes(b []byte) uint {
	if len(b) == 0 {
		return 0
	}

	return uint(uintptr(unsafe.Pointer(&b[0])))
}

func dirDeUint16(b []uint16) uint {
	if len(b) == 0 {
		return 0
	}

	return uint(uintptr(unsafe.Pointer(&b[0])))
}

func dirDeInt32(p *int32) uint {
	return uint(uintptr(unsafe.Pointer(p)))
}

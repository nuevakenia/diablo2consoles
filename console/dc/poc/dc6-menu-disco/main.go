//go:build dreamcast

// DC-002.35 -- EL PRIMER HITO VISIBLE: un DC6 REAL del menu, leido del MPQ DEL DISCO y pintado en la
// Dreamcast.
//
// LA CADENA QUE SE MIDE (la misma que despues usan los menus, los personajes y el mapa):
//   MPQ del disco (ISO9660, /cd) -> d2mpq.FromFile + ReadFile -> d2dc6.Load + DecodeFrame (el
//   DECODIFICADOR DEL MOTOR, el mismo codigo que corre en el host) -> paleta PL2 (d2pl2) -> textura
//   paletizada de 8 bits + paleta ARGB1555 por hardware -> quad por el PVR (csrc/cshim.c).
//
// POR QUE patch_d2.mpq Y NO d2data.mpq (medido, y no es una eleccion estetica):
//   `d2data.mpq` trae 65.536 entradas en su tabla de hash y abrirla en la consola revienta el
//   semiespacio del recolector (DC-002.28: arena=1543616 used=1541480 free=2136, gc_external_alloc:
//   OOM). Esa es la palanca de DC-002.33, que todavia esta en vuelo. `patch_d2.mpq` trae 16.384
//   entradas y SI se abre en la consola: es el mismo camino con una tabla que entra. El asset es un
//   DC6 del MENU (la carpeta BIGMENU de la interfaz), que es lo que pide el hito.
//
// LO QUE IMPRIME (la consola serie es la unica salida; los numeros son los que despues compara
// scripts/console/checar-dc6-menu-235.sh contra el host):
//   - los bytes del MPQ del disco y los ms de abrirlo;
//   - los bytes del DC6 y su CRC-32 (el mismo polinomio IEEE que el zlib del host);
//   - cuantos frames trae, el lienzo compuesto y el CRC-32 de los indices compuestos;
//   - el CRC-32 de la paleta ARGB1555 que se sube a la VRAM;
//   - la VRAM libre antes y despues de subir la textura, y los cuadros dibujados.
//
// MUTANTE (criterio 3): con DC6_INEXISTENTE=1 (lo hace el script, recompilando) el nombre del asset
// apunta a uno que no existe: `ReadFile` falla, NO se sube textura y la pantalla queda como estaba
// (el fondo del PVR, sin un solo pixel del asset).
package main

import (
	"hash/crc32"
	"os"
	"runtime"
	"unsafe"

	"kos"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2dc6"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2mpq"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2pl2"
)

//extern d2c_serial_init
func d2cSerialInit()

//extern d2c_ms
func d2cMs() uint

//extern d2c_tex_init
func d2cTexInit(idx uint, pal uint, w, h, pw, ph int32) int32

//extern d2c_quad
func d2cQuad(x, y, s float32)

//extern d2c_vram_libre
func d2cVramLibre() uint

const (
	// El disco: el MPQ y la paleta, los dos archivos, por ISO9660. Los nombres van en mayusculas
	// porque asi los escribe el ISO9660 (mkiso.py) y el VFS de KOS resuelve el nombre exacto.
	rutaMPQ = "/cd/JUEGO/PATCH_D2.MPQ"
	rutaPAL = "/cd/JUEGO/PAL.PL2"

	// El asset: el fondo de la sala de espera del menu (carpeta BIGMENU de la interfaz del juego).
	dc6EnMPQ = `data\global\ui\BIGMENU\waitingroombkgd.dc6`

	// ancho/alto de la pantalla del POC (la del modo por defecto de KOS).
	ancho, alto = 640, 480

	// maxEntero32 / minEntero32: los topes de un int de 32 bits, escritos a mano.
	// OJO (medido): la stdlib minima de la consola NO trae math.MaxInt32/math.MinInt32 (los agregó Go
	// en 1.17 y esta copia de math no los tiene): gccgo canta "reference to undefined identifier".
	maxEntero32 = 1<<31 - 1
	minEntero32 = -(1 << 31)

	// cuadrosAMedir: cuantos cuadros dibuja antes de avisar (el bucle es infinito: Flycast se corta
	// por tiempo desde el script).
	cuadrosAMedir = 300

	// trozoBytes: el tamano de trozo con el que se compara el archivo leido contra el host (64 KiB).
	trozoBytes = 65536
)

// hex16: los primeros 16 bytes en hexa, para comparar byte a byte contra el host (los dos caminos tienen
// que dar lo mismo: el archivo del MPQ esta COMPRIMIDO, asi que son el resultado de descomprimir).
func hex16(datos []byte) string {
	const hexa = "0123456789abcdef"

	n := 16
	if len(datos) < n {
		n = len(datos)
	}

	salida := make([]byte, 0, n*3)

	for i := 0; i < n; i++ {
		if i > 0 {
			salida = append(salida, ' ')
		}

		salida = append(salida, hexa[datos[i]>>4], hexa[datos[i]&0x0f])
	}

	return string(salida)
}

func main() {
	d2cSerialInit()

	println("HITO235: arranca (DC-002.35, un DC6 real del menu desde el MPQ del disco, pintado por el PVR)")
	println("HITO235: runtime", runtime.GOOS, runtime.GOARCH)

	// --- 1. el MPQ del disco ---
	t0 := int64(d2cMs())

	mpq, err := d2mpq.FromFile(rutaMPQ)
	if err != nil {
		println("HITO235: MPQ-ABRIR FALLO:", err.Error())

		return
	}

	defer mpq.Close()

	println("HITO235: MPQ-ABRIR OK", rutaMPQ, "ms=", int64(d2cMs())-t0)

	// --- 2. el DC6, crudo, del MPQ del disco ---
	t1 := int64(d2cMs())

	raw, err := mpq.ReadFile(dc6EnMPQ)
	if err != nil {
		println("HITO235: DC6-LEER FALLO:", dc6EnMPQ, ":", err.Error())
		println("HITO235: sin asset no hay textura: la pantalla queda como estaba (mutante)")

		// el mutante: NO se pinta nada. El bucle de dibujo de abajo no arranca.
		pantallaVacia()

		return
	}

	println("HITO235: DC6-LEER OK bytes=", len(raw), "crc32=", crc32.ChecksumIEEE(raw), "ms=", int64(d2cMs())-t1)

	// DC-002.35, criterio 1: los PRIMEROS BYTES y el CRC-32 de cada trozo de 64 KiB. El archivo esta
	// COMPRIMIDO en el MPQ (implode de PKWARE: lo mide scripts/console/sonda-dc6-mpq-235.py), asi que lo que
	// sale de `ReadFile` es el resultado de DESCOMPRIMIR: con los trozos se ve en cual se separa del host.
	println("HITO235: DC6 primeros 16 bytes:", hex16(raw))

	for i := 0; i < len(raw); i += trozoBytes {
		fin := i + trozoBytes
		if fin > len(raw) {
			fin = len(raw)
		}

		println("HITO235: DC6 trozo", i/trozoBytes, "desde=", i, "bytes=", fin-i, "crc32=", crc32.ChecksumIEEE(raw[i:fin]))
	}

	// --- 3. decodificar con el decodificador del MOTOR ---
	dc, err := d2dc6.Load(raw)
	if err != nil {
		println("HITO235: DC6-DECODE FALLO:", err.Error())

		return
	}

	println("HITO235: DC6 frames=", len(dc.Frames))

	idx, w, h, pegados := componerFrames(dc)
	if w <= 0 || h <= 0 {
		println("HITO235: el DC6 no trae frames con medidas")

		return
	}

	println("HITO235: DC6 compuesto w=", w, "h=", h, "frames pegados=", pegados,
		"indices=", len(idx), "crc32=", crc32.ChecksumIEEE(idx))

	// --- 4. la paleta del disco (PL2) -> ARGB1555 ---
	pb, err := os.ReadFile(rutaPAL)
	if err != nil {
		println("HITO235: PALETA-LEER FALLO:", rutaPAL, ":", err.Error())

		return
	}

	pal, err := d2pl2.Load(pb)
	if err != nil {
		println("HITO235: PALETA-PARSE FALLO:", err.Error())

		return
	}

	colors := pal.BasePalette.Colors

	pvrPal := make([]uint16, 256)
	for i := 0; i < 256; i++ {
		c := colors[i]

		// el indice 0 es transparente: la convencion de Diablo II (y la del repo)
		alpha := uint16(1)
		if i == 0 {
			alpha = 0
		}

		pvrPal[i] = alpha<<15 | uint16(c.R>>3)<<10 | uint16(c.G>>3)<<5 | uint16(c.B>>3)
	}

	println("HITO235: PALETA bytes=", len(pb), "colores=", len(colors), "crc32=", crc32Pal(pvrPal))

	// --- 5. el PVR: subir la textura y pintarla ---
	kos.PvrInitDefaults()
	kos.PvrSetBgColor(0.0, 0.0, 0.0)
	kos.PvrWaitReady()

	println("HITO235: VRAM libre antes=", int64(d2cVramLibre()))

	pw, ph := siguientePot2(w), siguientePot2(h)

	rc := d2cTexInit(
		uint(uintptr(unsafe.Pointer(&idx[0]))),
		uint(uintptr(unsafe.Pointer(&pvrPal[0]))),
		int32(w), int32(h), int32(pw), int32(ph),
	)
	if rc != 0 {
		println("HITO235: TEXTURA FALLO codigo=", rc)

		return
	}

	println("HITO235: TEXTURA OK w=", w, "h=", h, "pw=", pw, "ph=", ph, "bytes=", pw*ph)
	println("HITO235: VRAM libre despues=", int64(d2cVramLibre()))

	// La escala que LLENA sin salirse: la MENOR de las dos (si no, el PVR descarta el triangulo que
	// queda afuera de la pantalla -- medido en DC-002.9: la imagen salia cortada en diagonal).
	esc := float32(ancho) / float32(w)
	if sy := float32(alto) / float32(h); sy < esc {
		esc = sy
	}

	println("HITO235: escala milesima=", int(esc*1000))
	println("HITO235: dibujando")

	cuadros := 0

	for {
		kos.PvrWaitReady()
		kos.PvrSceneBegin()
		kos.PvrListBegin(kos.PVR_LIST_TR_POLY)
		d2cQuad(0, 0, esc)
		kos.PvrListFinish()
		kos.PvrSceneFinish()

		cuadros++
		if cuadros == cuadrosAMedir {
			println("HITO235: CUADROS", cuadros, "dibujados -- pantalla pintada con el asset del disco")
		}
	}
}

// pantallaVacia: el camino del mutante (DC6 que no existe). Deja el PVR con su fondo y NO sube nada:
// la pantalla tiene que quedar como estaba.
func pantallaVacia() {
	kos.PvrInitDefaults()
	kos.PvrSetBgColor(0.0, 0.0, 0.0)
	kos.PvrWaitReady()

	println("HITO235: PANTALLA-VACIA (sin textura, sin quad) vram=", int64(d2cVramLibre()))

	for {
		kos.PvrWaitReady()
		kos.PvrSceneBegin()
		kos.PvrListFinish()
		kos.PvrSceneFinish()
	}
}

// componerFrames: pega TODOS los frames en un lienzo, como el horneador del host (console/dc/bake).
//
// Por que: los DC6 de la interfaz son celdas sueltas que el juego pega con sus offsets, y esos offsets
// son RELATIVOS y pueden ser negativos. Primero se calcula el rectangulo que los contiene a todos y
// despues se pega cada celda ahi adentro, saltando el indice 0 (que en DC6 es transparente). El codigo
// es el mismo del horneador del host, a proposito: los dos lados tienen que dar el MISMO resultado.
func componerFrames(dc *d2dc6.DC6) ([]byte, int, int, int) {
	minX, minY := maxEntero32, maxEntero32
	maxX, maxY := minEntero32, minEntero32

	for _, f := range dc.Frames {
		if f == nil {
			continue
		}

		x0, y0 := int(f.OffsetX), int(f.OffsetY)
		x1, y1 := x0+int(f.Width), y0+int(f.Height)

		if x0 < minX {
			minX = x0
		}

		if y0 < minY {
			minY = y0
		}

		if x1 > maxX {
			maxX = x1
		}

		if y1 > maxY {
			maxY = y1
		}
	}

	if minX > maxX || minY > maxY {
		return nil, 0, 0, 0
	}

	w, h := maxX-minX, maxY-minY
	topDown := make([]byte, w*h) // ceros = indice 0 = transparente
	pegados := 0

	for i, f := range dc.Frames {
		if f == nil {
			continue
		}

		fi := dc.DecodeFrame(i)
		if fi == nil {
			continue
		}

		fw, fh := int(f.Width), int(f.Height)
		ox, oy := int(f.OffsetX)-minX, int(f.OffsetY)-minY

		for y := 0; y < fh; y++ {
			for x := 0; x < fw; x++ {
				v := fi[y*fw+x]
				if v == 0 {
					continue // el 0 es transparente: no pisa lo ya pegado
				}

				cx, cy := ox+x, oy+y
				if cx < 0 || cy < 0 || cx >= w || cy >= h {
					continue
				}

				topDown[cy*w+cx] = v
			}
		}

		pegados++
	}

	return topDown, w, h, pegados
}

// crc32Pal: el CRC-32 de los 256 colores ARGB1555, en el mismo orden de bytes que usa el host (dos
// bytes por color, el bajo primero): es lo que sube a la VRAM y lo que tiene que coincidir.
func crc32Pal(pal []uint16) uint32 {
	b := make([]byte, len(pal)*2)
	for i, v := range pal {
		b[i*2] = byte(v & 0xff)
		b[i*2+1] = byte(v >> 8)
	}

	return crc32.ChecksumIEEE(b)
}

// siguientePot2: el lado de la textura tiene que ser potencia de dos (lo exige el PVR y KOS).
func siguientePot2(n int) int {
	p := 1
	for p < n {
		p <<= 1
	}

	return p
}

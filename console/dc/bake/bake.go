// Hornea un asset de Diablo II (un frame de DC6) al formato plano que la Dreamcast puede
// subir como textura paletizada: indices de 8 bits + paleta de 256 colores en ARGB1555
// (el formato nativo de la paleta por hardware del PowerVR2).
//
// Uso:
//
//	go run ./console/dc/bake -mpq <ruta.mpq> -list '\.dc6$'            # explorar que hay
//	go run ./console/dc/bake -mpq <ruta.mpq> -dc6 <ruta en el mpq> \
//	    -pal <ruta del pl2> -frame 0 -out console/dc/poc/pal8
//
// Salidas en -out:
//
//	asset.h   indices + paleta, listos para compilar en el POC de la consola
//	asset.png referencia renderizada en el host (para comparar contra la captura del emulador)
package main

import (
	"bytes"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strconv"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2dc6"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2mpq"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2pl2"
)

func main() {
	var (
		mpqPath  string
		listPat  string
		dc6Path  string
		palPath  string
		compuesto bool
		outDir   string
		frameIdx int
		name     string
	)
	flag.StringVar(&mpqPath, "mpq", "", "archivo MPQ de entrada")
	flag.StringVar(&listPat, "list", "", "listar entradas del MPQ que matcheen este regex")
	flag.StringVar(&dc6Path, "dc6", "", "ruta del DC6 dentro del MPQ")
	flag.StringVar(&palPath, "pal", "", "ruta del PL2 (paleta) dentro del MPQ")
	flag.BoolVar(&compuesto, "compuesto", false,
		"componer TODOS los frames en un lienzo (para los DC6 de la interfaz, que son celdas sueltas)")
	flag.StringVar(&outDir, "out", "", "directorio de salida")
	flag.IntVar(&frameIdx, "frame", 0, "frame del DC6 a hornear (0 = primero)")
	flag.StringVar(&name, "name", "asset", "nombre base de los archivos de salida")
	flag.Parse()

	if mpqPath == "" {
		fatal("falta -mpq")
	}

	mpq, err := d2mpq.FromFile(mpqPath)
	if err != nil {
		fatal("abriendo MPQ: %v", err)
	}

	if listPat != "" {
		re, err := regexp.Compile(listPat)
		if err != nil {
			fatal("regex invalido: %v", err)
		}
		names, err := mpq.Listfile()
		if err != nil {
			fatal("listfile: %v", err)
		}
		for _, n := range names {
			if re.MatchString(n) {
				fmt.Println(n)
			}
		}
		fmt.Fprintf(os.Stderr, "(%d entradas listadas)\n", len(names))
		return
	}

	if dc6Path == "" || outDir == "" {
		fatal("faltan -dc6 y/o -out")
	}

	// --- 1. leer y decodificar el DC6 ---
	raw, err := mpq.ReadFile(dc6Path)
	if err != nil {
		fatal("leyendo %s del MPQ: %v", dc6Path, err)
	}
	dc6, err := d2dc6.Load(raw)
	if err != nil {
		fatal("parseando DC6: %v", err)
	}
	if len(dc6.Frames) == 0 {
		fatal("el DC6 no trae frames")
	}
	var w, h int

	var topDown []byte

	detalle := "frame " + strconv.Itoa(frameIdx)

	if compuesto {
		// Componer TODOS los frames en un solo lienzo.
		//
		// Los DC6 de la interfaz del juego son celdas sueltas que el juego pega en la pantalla con
		// sus offsets, y esos offsets son RELATIVOS y pueden ser negativos. Por eso primero se
		// calcula el rectangulo que los contiene a todos y despues se pega cada celda ahi adentro,
		// saltando el indice 0 (que en DC6 es transparente): lo que queda es la pantalla compuesta.
		minX, minY := math.MaxInt32, math.MaxInt32
		maxX, maxY := math.MinInt32, math.MinInt32

		for _, f := range dc6.Frames {
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
			fatal("el DC6 no trae frames con medidas")
		}

		w, h = maxX-minX, maxY-minY
		topDown = make([]byte, w*h) // ceros = indice 0 = transparente

		pegados := 0

		for i, f := range dc6.Frames {
			if f == nil {
				continue
			}

			fi := dc6.DecodeFrame(i)
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

		detalle = "compuesto de " + strconv.Itoa(pegados) + " de " + strconv.Itoa(len(dc6.Frames)) +
			" frames, origen=(" + strconv.Itoa(minX) + "," + strconv.Itoa(minY) + ")"
	} else {
		if frameIdx < 0 || frameIdx >= len(dc6.Frames) {
			fatal("frame %d fuera de rango (hay %d)", frameIdx, len(dc6.Frames))
		}

		frame := dc6.Frames[frameIdx]
		w, h = int(frame.Width), int(frame.Height)

		topDown = dc6.DecodeFrame(frameIdx)
		if topDown == nil {
			fatal("no se pudo decodificar el frame %d", frameIdx)
		}
	}

	// OJO: NO dar vuelta los indices. dc6.DecodeFrame() arranca en y = height-1 y va bajando, o sea
	// compensa que el DC6 guarda las lineas de abajo hacia arriba: lo que devuelve YA esta en
	// orden de despliegue (fila 0 = arriba), que es como lo usa el motor con ImgIndexToRGBA y
	// ReplacePixels. Darle una vuelta extra espeja el sprite verticalmente (me paso).

	// El PVR (y KOS) exigen texturas de lado potencia de dos. Se rellena con indice 0, que es
	// transparente, y el quad dibuja solo el pedazo visible (W x H) con las coordenadas UV.
	pw, ph := nextPow2(w), nextPow2(h)
	padded := make([]byte, pw*ph) // ceros = indice 0 = transparente
	for y := 0; y < h; y++ {
		copy(padded[y*pw:y*pw+w], topDown[y*w:(y+1)*w])
	}

	// --- 2. paleta ---
	palBytes, err := mpq.ReadFile(palPath)
	if err != nil {
		fatal("leyendo la paleta %s: %v", palPath, err)
	}
	pal, err := d2pl2.Load(palBytes)
	if err != nil {
		fatal("parseando PL2: %v", err)
	}
	colors := pal.BasePalette.Colors

	// --- 3. paleta ARGB1555 para la Dreamcast + PNG de referencia ---
	pvrPal := make([]uint16, 256)
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for i := 0; i < 256; i++ {
		c := colors[i]
		// indice 0 = transparente (es la convencion de Diablo II y la del repo)
		alpha := uint16(1)
		if i == 0 {
			alpha = 0
		}
		pvrPal[i] = alpha<<15 | uint16(c.R>>3)<<10 | uint16(c.G>>3)<<5 | uint16(c.B>>3)
	}
	// el PNG lo armo desde los indices para que sea exactamente lo que va a ver la consola
	img = image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			i := topDown[y*w+x]
			if i == 0 {
				continue // transparente
			}
			c := colors[i]
			img.SetNRGBA(x, y, color.NRGBA{R: c.R, G: c.G, B: c.B, A: 255})
		}
	}

	if err := os.MkdirAll(outDir, 0o755); err != nil {
		fatal("creando %s: %v", outDir, err)
	}

	// --- 4. escribir el .h ---
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "/* Generado por console/dc/bake desde %s (frame %d). NO editar a mano.\n", dc6Path, frameIdx)
	fmt.Fprintf(&buf, " * %d x %d, indices de 8 bits + paleta ARGB1555 (paleta por hardware del PVR).\n", w, h)
	fmt.Fprintf(&buf, " * El indice 0 es transparente (convencion de Diablo II).\n */\n")
	fmt.Fprintf(&buf, "/* W x H = el arte visible; PW x PH = la textura rellenada a potencia de dos */\n")
	fmt.Fprintf(&buf, "#define %s_W %d\n#define %s_H %d\n", upper(name), w, upper(name), h)
	fmt.Fprintf(&buf, "#define %s_PW %d\n#define %s_PH %d\n\n", upper(name), pw, upper(name), ph)

	fmt.Fprintf(&buf, "static const unsigned char %s_idx[%d] = {\n", name, len(padded))
	for i, b := range padded {
		if i%16 == 0 {
			buf.WriteString("\t")
		}
		fmt.Fprintf(&buf, "0x%02x,", b)
		if i%16 == 15 {
			buf.WriteString("\n")
		}
	}
	buf.WriteString("};\n\n")

	fmt.Fprintf(&buf, "static const unsigned short %s_pal[256] = {\n", name)
	for i, v := range pvrPal {
		if i%8 == 0 {
			buf.WriteString("\t")
		}
		fmt.Fprintf(&buf, "0x%04x,", v)
		if i%8 == 7 {
			buf.WriteString("\n")
		}
	}
	buf.WriteString("};\n")

	hPath := filepath.Join(outDir, name+".h")
	if err := os.WriteFile(hPath, buf.Bytes(), 0o644); err != nil {
		fatal("escribiendo %s: %v", hPath, err)
	}

	pngPath := filepath.Join(outDir, name+".png")
	f, err := os.Create(pngPath)
	if err != nil {
		fatal("creando %s: %v", pngPath, err)
	}
	if err := png.Encode(f, img); err != nil {
		fatal("escribiendo PNG: %v", err)
	}
	f.Close()

	fmt.Printf("asset: %s  %dx%d  %s de %d  (el 0 es transparente)\n",
		dc6Path, w, h, detalle, len(dc6.Frames))
	fmt.Printf("  -> %s (%d bytes de indices + 512 de paleta; textura %dx%d)\n", hPath, len(padded), pw, ph)
	fmt.Printf("  -> %s (referencia del host)\n", pngPath)
}

// nextPow2 devuelve el menor 2^n mayor o igual a v (el PVR exige texturas potencia de dos).
func nextPow2(v int) int {
	p := 1
	for p < v {
		p <<= 1
	}

	return p
}

func upper(s string) string {
	out := make([]rune, 0, len(s))
	for _, r := range s {
		if r >= 'a' && r <= 'z' {
			r -= 32
		}
		if r == '-' || r == '.' {
			r = '_'
		}
		out = append(out, r)
	}

	return string(out)
}

func fatal(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "error: "+format+"\n", args...)
	os.Exit(1)
}

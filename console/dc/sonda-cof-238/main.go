// sonda-cof-238: lee un COF de un personaje del MPQ, prueba la ruta de cada capa contra los DCC/DC6
// REALES del MPQ y (con -png) escribe el PNG de cada capa y del compuesto, para PODER MIRARLO.
//
// Es una herramienta del HOST: contesta, con el parser del motor, que capas trae el COF, cual es la
// ruta del grafico de cada una (la que arma `d2core/d2asset/composite.go`:
// <base>/<token>/<capa>/<token><capa><valor><modo><arma>.dcc|dc6), cuanto pesa en el MPQ, cuanto mide
// el frame, y cuanta memoria pide el decodificador del motor al leerlo.
//
// Uso: go run ./console/dc/sonda-cof-238 -mpq <d2char.mpq> -cof 'data\global\CHARS\AM\COF\amnuhth.cof'
package main

import (
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2cof"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2dcc"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2dc6"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2mpq"
)

// paleta del juego (units): data\global\palette\units\pal.dat, 768 bytes B,G,R.
func cargarPaleta(mpq *d2mpq.MPQ, ruta string) ([][3]uint8, error) {
	ruta = strings.ReplaceAll(ruta, "/", `\`)
	ruta = strings.TrimPrefix(ruta, `\`)

	b, err := mpq.ReadFile(ruta)
	if err != nil {
		return nil, err
	}

	var pal [][3]uint8

	for i := 0; i+2 < len(b) && i < 768; i += 3 {
		pal = append(pal, [3]uint8{b[i+2], b[i+1], b[i]})
	}

	return pal, nil
}

func main() {
	mpqPath := flag.String("mpq", "", "")
	cofPath := flag.String("cof", "", "")
	token := flag.String("token", "AM", "")
	modo := flag.String("modo", "NU", "")
	valor := flag.String("valor", "MED", "valor de la capa (lit, med, hvy, bhm)")
	salida := flag.String("png", "", "directorio donde escribir los PNG (vacio = no escribe)")
	palPath := flag.String("pal", "/data/global/palette/Units/pal.dat", "")
	palMPQ := flag.String("palmpq", "", "MPQ de donde sale la paleta (vacio = el mismo)")
	dir := flag.Int("dir", 0, "direccion a mirar")
	frame := flag.Int("frame", 0, "frame a mirar")
	flag.Parse()

	mpq, err := d2mpq.FromFile(*mpqPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "no pude abrir el MPQ:", err)
		os.Exit(1)
	}
	defer mpq.Close()

	deMPQ := mpq

	if *palMPQ != "" {
		otro, err := d2mpq.FromFile(*palMPQ)
		if err != nil {
			fmt.Fprintln(os.Stderr, "no pude abrir el MPQ de la paleta:", err)
			os.Exit(1)
		}
		defer otro.Close()

		deMPQ = otro
	}

	pal, err := cargarPaleta(deMPQ, *palPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "no pude leer la paleta:", err)
		os.Exit(1)
	}

	fmt.Println("paleta", *palPath, "colores", len(pal))

	raw, err := mpq.ReadFile(*cofPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "no pude leer el COF:", err)
		os.Exit(1)
	}

	cof, err := d2cof.Unmarshal(raw)
	if err != nil {
		fmt.Fprintln(os.Stderr, "no pude parsear el COF:", err)
		os.Exit(1)
	}

	fmt.Printf("COF %s bytes=%d capas=%d framesPorDir=%d dirs=%d speed=%d\n",
		*cofPath, len(raw), cof.NumberOfLayers, cof.FramesPerDirection, cof.NumberOfDirections, cof.Speed)

	var antes, despues runtime.MemStats

	runtime.GC()
	runtime.ReadMemStats(&antes)

	// el lienzo del compuesto: 400x400 alcanza para un heroe (los offsets son chicos)
	lado := 400
	lienzo := make([]byte, lado*lado)

	tCapas := time.Now()

	for i, l := range cof.CofLayers {
		clave := l.Type.String()
		arma := l.WeaponClass.String()

		candidatas := []string{}
		for _, v := range []string{*valor, "LIT", "MED", "HVY", "BHM"} {
			candidatas = append(candidatas,
				fmt.Sprintf("/data/global/chars/%s/%s/%s%s%s%s%s.dcc", *token, clave, *token, clave, v, *modo, arma),
				fmt.Sprintf("/data/global/chars/%s/%s/%s%s%s%s%s.dc6", *token, clave, *token, clave, v, *modo, arma),
			)
		}

		estado := "SIN-GRAFICO"

		for _, cand := range candidatas {
			nombre := strings.ReplaceAll(cand, "/", `\`)
			nombre = strings.TrimPrefix(nombre, `\`)

			b, err := mpq.ReadFile(nombre)
			if err != nil {
				continue
			}

			if strings.HasSuffix(strings.ToLower(cand), ".dcc") {
				t0 := time.Now()
				// el decodificador decodifica las direcciones al cargar (d2dcc.Load), asi que el
				// tiempo que se mide aca es el de la CARGA COMPLETA del archivo.
				d, err := d2dcc.Load(b)
				if err != nil {
					estado = fmt.Sprintf("%s PARSE-FALLO: %v", cand, err)
					break
				}

				du := time.Since(t0)

				dd := d.Directions[*dir]
				f := dd.Frames[*frame]

				estado = fmt.Sprintf("OK %s bytes=%d dirs=%d frames=%d frame[%d]=%dx%d offX=%d offY=%d pixeles=%d decode=%s",
					cand, len(b), d.NumberOfDirections, d.FramesPerDirection, *frame,
					f.Width, f.Height, f.XOffset, f.YOffset, len(f.PixelData), du.Round(time.Millisecond))

				if *salida != "" {
					pegar(lienzo, lado, f.PixelData, f.Width, f.Height, f.XOffset+lado/2, f.YOffset+lado/2-40)
				}
			} else {
				d, err := d2dc6.Load(b)
				if err != nil {
					estado = fmt.Sprintf("%s PARSE-FALLO: %v", cand, err)
					break
				}

				f := d.Frames[*frame]
				pix := d.DecodeFrame(*frame)
				estado = fmt.Sprintf("OK %s bytes=%d frames=%d frame[%d]=%dx%d offX=%d offY=%d",
					cand, len(b), len(d.Frames), *frame, f.Width, f.Height, f.OffsetX, f.OffsetY)

				if *salida != "" {
					pegar(lienzo, lado, pix, int(f.Width), int(f.Height), int(f.OffsetX)+lado/2, int(f.OffsetY)+lado/2-40)
				}
			}

			break
		}

		fmt.Printf("  capa %d: tipo=%-3s sombra=%d transparente=%v efecto=%v arma=%-3s -> %s\n",
			i, clave, l.Shadow, l.Transparent, l.DrawEffect, arma, estado)
	}

	fmt.Println("las", cof.NumberOfLayers, "capas tardaron", time.Since(tCapas).Round(time.Millisecond))

	runtime.GC()
	runtime.ReadMemStats(&despues)

	fmt.Printf("memoria: heap en uso antes=%d KB despues(capas vivas)=%d KB delta=%d KB\n",
		antes.HeapInuse/1024, despues.HeapInuse/1024, (int64(despues.HeapInuse)-int64(antes.HeapInuse))/1024)

	if *salida != "" {
		if err := os.MkdirAll(*salida, 0o755); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}

		escribirPNG(*salida+"/compuesto.png", lienzo, lado, lado, pal, "")
		fmt.Println("PNG escrito en", *salida+"/compuesto.png")
	}
}

// pegar: mete los indices del frame en el lienzo (el indice 0 NO pisa: es transparente).
func pegar(lienzo []byte, lado int, pix []byte, ancho, alto, ox, oy int) {
	for y := 0; y < alto; y++ {
		cy := oy + y
		if cy < 0 || cy >= lado {
			continue
		}

		for x := 0; x < ancho; x++ {
			v := pix[y*ancho+x]
			if v == 0 {
				continue
			}

			cx := ox + x
			if cx < 0 || cx >= lado {
				continue
			}

			lienzo[cy*lado+cx] = v
		}
	}
}

func escribirPNG(ruta string, lienzo []byte, ancho, alto int, pal [][3]uint8, _ string) {
	img := image.NewRGBA(image.Rect(0, 0, ancho, alto))

	for y := 0; y < alto; y++ {
		for x := 0; x < ancho; x++ {
			i := lienzo[y*ancho+x]
			if i == 0 {
				img.SetRGBA(x, y, color.RGBA{A: 0})
				continue
			}

			if int(i) < len(pal) {
				img.SetRGBA(x, y, color.RGBA{R: pal[i][0], G: pal[i][1], B: pal[i][2], A: 255})
			}
		}
	}

	f, err := os.Create(ruta)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return
	}
	defer f.Close()

	_ = png.Encode(f, img)
}

// sonda-anim-249: el censo de la ANIMACION del personaje, leido del MPQ con los lectores del MOTOR.
//
// Para que (DC-002.49): el jugador dijo "se movia sin animacion". La tarjeta pide dos numeros: cuantos cuadros
// tiene el ciclo del personaje y cuantos se ven de verdad. El primero sale de aca, sin emulador y sin el POC:
//
//   - el COF del personaje: cuadros por direccion, direcciones, capas y la velocidad (Speed -> FPS -> ms/cuadro),
//   - el animdata.d2: la velocidad con la que el juego manda esos cuadros (su FPS manda sobre el del COF).
//
// Los modos que importan en el PUEBLO son TN (town neutral: el personaje PARADO) y TW (town walk: caminando).
// Si TN tiene mas de un cuadro, el personaje quieto TIENE que estar animando; un POC que pinta siempre el cuadro
// 0 se ve tieso aunque el motor tenga la animacion.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2animdata"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2cof"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2mpq"
)

var modos = []string{"TN", "TW", "NU", "WL", "RN", "GH", "A1", "A2", "SC", "DT", "DD"}

func main() {
	mpqPath := flag.String("mpq", filepath.Join(os.Getenv("HOME"), ".wine-d2", "drive_c", "Games", "Diablo II", "d2char.mpq"), "d2char.mpq (los personajes NO estan en d2data.mpq)")
	token := flag.String("token", "AM", "token del personaje (AM = amazona)")
	busca := flag.String("busca", "", "en vez del censo: lista las rutas del MPQ que contengan este texto")
	tipo := flag.String("tipo", "chars", "carpeta del MPQ donde viven los COF: chars (personajes) o monsters")
	flag.Parse()

	m, err := d2mpq.FromFile(*mpqPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "MPQ:", err)
		os.Exit(1)
	}

	defer m.Close()

	if *busca != "" {
		lista, err := m.Listfile()
		if err != nil {
			fmt.Fprintln(os.Stderr, "listfile:", err)
			os.Exit(1)
		}

		cuenta := 0

		for _, p := range lista {
			if strings.Contains(strings.ToUpper(p), strings.ToUpper(*busca)) {
				fmt.Println(p)

				cuenta++
			}
		}

		fmt.Printf("--- %d rutas con %q (de %d en el listfile)\n", cuenta, *busca, len(lista))

		return
	}

	raw, err := m.ReadFile(`data\global\animdata.d2`)
	if err != nil {
		// animdata.d2 vive en d2data.mpq, no en d2char.mpq: se abre el segundo MPQ solo para esto
		mpqDatos := filepath.Join(filepath.Dir(*mpqPath), "d2data.mpq")
		if m2, err2 := d2mpq.FromFile(mpqDatos); err2 == nil {
			defer m2.Close()

			raw, err = m2.ReadFile(`data\global\animdata.d2`)
		}
	}

	if err != nil {
		fmt.Fprintln(os.Stderr, "animdata.d2:", err)
		os.Exit(1)
	}

	anim, err := d2animdata.Load(raw)
	if err != nil {
		fmt.Fprintln(os.Stderr, "animdata parse:", err)
		os.Exit(1)
	}

	fmt.Printf("ANIMDATA: %d animaciones en total\n", anim.GetRecordsCount())

	totalCuadros, deUnCuadro := 0, 0

	for _, modo := range modos {
		// el COF lleva la clase de arma pegada al nombre (amtnhth.cof): se prueba HTH (sin arma) y despues las
		// otras, y se informa cual se uso. Los nombres del MPQ van en minusculas pero el MPQ busca sin caja.
		var rawCof []byte

		var cofUsado string

		err := error(nil)

		for _, wc := range []string{"hth", "1hs", "bow", "stf", "2hs"} {
			cofUsado = fmt.Sprintf(`data\global\%s\%s\COF\%s%s%s.COF`, *tipo, *token, *token, modo, wc)

			if rawCof, err = m.ReadFile(cofUsado); err == nil && len(rawCof) > 0 {
				break
			}
		}

		if len(rawCof) == 0 {
			fmt.Printf("MODO %-2s  sin COF (%s)\n", modo, cofUsado)

			continue
		}

		c, err := d2cof.Unmarshal(rawCof)
		if err != nil {
			fmt.Printf("MODO %-2s  COF ilegible (%d bytes leidos): %v\n", modo, len(rawCof), err)

			continue
		}

		// la velocidad: manda el animdata si tiene la animacion; si no, la del COF. Las claves del animdata
		// llevan la clase de arma pegada (AMTNHTH), asi que se prueban las dos formas.
		fps := c.FPS()
		fuente := "COF"

		for _, clave := range []string{*token + modo + "HTH", *token + modo} {
			if rec := anim.GetRecord(clave); rec != nil && rec.FPS() > 0 {
				fps = rec.FPS()
				fuente = "animdata(" + clave + ")"

				break
			}
		}

		msPorCuadro := 1000.0 / fps
		duracion := float64(c.FramesPerDirection) * msPorCuadro

		fmt.Printf("MODO %-2s  cuadrosPorDireccion=%2d direcciones=%2d capas=%d  fps=%.2f [%s]  %.2f ms/cuadro  ciclo=%.0f ms\n",
			modo, c.FramesPerDirection, c.NumberOfDirections, len(c.CofLayers), fps, fuente, msPorCuadro, duracion)

		totalCuadros += c.FramesPerDirection * c.NumberOfDirections

		if c.FramesPerDirection == 1 {
			deUnCuadro++
		}
	}

	fmt.Printf("\nTOTAL cuadros sumando esos modos (por direccion x direcciones)=%d ; modos con UN solo cuadro=%d\n",
		totalCuadros, deUnCuadro)
}

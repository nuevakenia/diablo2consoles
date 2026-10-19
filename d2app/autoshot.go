package d2app

import (
	"os"
	"strconv"
	"strings"
)

const autoShotDefaultSeconds = 10.0

// autoShotState implements OD2_AUTOSHOT=<path.png>: after
// OD2_AUTOSHOT_SECONDS (default 10) of run time it saves the rendered frame
// (what the player sees, UI included) through the same capture facility as
// the console command "capframe", so a headless-ish autotest can look at the
// result. With OD2_AUTOEXIT=1 and no OD2_AUTOSCRIPT / OD2_AUTOMONSTER running
// the process quits after the shot. More shots during a scripted scenario can
// be taken with the script step say:capframe <path.png>.
//
// VARIAS CAPTURAS EN LA MISMA CORRIDA (DC-002.49): OD2_AUTOSHOT acepta una lista
// separada por ';' y OD2_AUTOSHOT_SECONDS los segundos de cada una, separados por
// coma y en el mismo orden:
//
//	OD2_AUTOSHOT='/tmp/a.png;/tmp/b.png' OD2_AUTOSHOT_SECONDS='12,13'
//
// POR QUE HACE FALTA: el pueblo se arma distinto en cada corrida (la semilla del
// azar cambia), asi que dos capturas de DOS corridas no son comparables: se midio
// 87,58 % del cuadro distinto (420.395 de 480.000 pixeles) comparando la misma
// escena a los 12 s y a los 13 s en corridas separadas. Dos capturas de la MISMA
// corrida si son comparables, y entonces lo que cambia entre ellas es lo que se
// mueve en pantalla (la animacion del personaje, con el personaje quieto).
type autoShotState struct {
	paths     []string
	ats       []float64
	idx       int
	elapsed   float64
	requested bool
}

func newAutoShot() *autoShotState {
	raw := os.Getenv("OD2_AUTOSHOT")
	if raw == "" {
		return nil
	}

	paths := strings.Split(raw, ";")

	s := &autoShotState{paths: paths, ats: make([]float64, len(paths))}

	for i := range s.ats {
		s.ats[i] = autoShotDefaultSeconds
	}

	if v := os.Getenv("OD2_AUTOSHOT_SECONDS"); v != "" {
		for i, parte := range strings.Split(v, ",") {
			if i >= len(s.ats) {
				break
			}

			if f, err := strconv.ParseFloat(strings.TrimSpace(parte), 64); err == nil && f >= 0 {
				s.ats[i] = f
			}
		}
	}

	return s
}

// advanceAutoShot is called once per frame with the unscaled elapsed seconds.
func (a *App) advanceAutoShot(elapsed float64) {
	s := a.autoShot
	if s == nil {
		return
	}

	if s.requested {
		// the capture happens in the render pass and resets the state when done
		if a.captureState != captureStateNone {
			return
		}

		if fi, err := os.Stat(s.paths[s.idx]); err == nil {
			a.Infof("AUTOSHOT saved %s (%d bytes)", s.paths[s.idx], fi.Size())
		} else {
			a.Errorf("AUTOSHOT failed: %v", err)
		}

		s.idx++
		s.requested = false

		if s.idx < len(s.paths) {
			return
		}

		a.autoShot = nil

		if os.Getenv("OD2_AUTOEXIT") != "" && os.Getenv("OD2_AUTOSCRIPT") == "" && os.Getenv("OD2_AUTOMONSTER") == "" {
			os.Exit(0)
		}

		return
	}

	s.elapsed += elapsed
	if s.elapsed < s.ats[s.idx] {
		return
	}

	s.requested = true
	a.captureState = captureStateFrame
	a.capturePath = s.paths[s.idx]
	a.captureFrames = nil
}

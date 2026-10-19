// gen-config-host-248: escribe un config.json para correr el MOTOR en el HOST (DC-002.48).
//
// POR QUE EXISTE: la referencia de esta tarjeta es el cuadro que pinta el MOTOR, no el que compone
// `nivel.go`. Para que el motor arranque sin dialogos hace falta un config.json valido (MpqPath +
// MpqLoadOrder). Lo escribe el PROPIO escritor del motor (d2config), asi no hay un JSON escrito a mano que
// despues no lea.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2config"
)

func main() {
	dir := flag.String("dir", "", "carpeta de la instalacion de Diablo II")
	salida := flag.String("out", "", "ruta del config.json a escribir")
	ancho := flag.Int("ancho", 800, "ancho de la ventana logica")
	alto := flag.Int("alto", 600, "alto de la ventana logica")
	flag.Parse()

	cfg := d2config.DefaultConfig()

	if *dir != "" {
		cfg.MpqPath = *dir
	}

	// el orden REAL de los archivos en el disco de este host (en Linux el nombre distingue mayusculas:
	// `patch_D2.mpq` y `d2xMusic.mpq` no son `patch_d2.mpq` ni `d2xmusic.mpq`)
	cfg.MpqLoadOrder = []string{
		"patch_D2.mpq",
		"d2exp.mpq",
		"d2xMusic.mpq",
		"d2xTalk.mpq",
		"d2xVideo.mpq",
		"d2data.mpq",
		"d2char.mpq",
		"d2music.mpq",
		"d2Sfx.mpq",
		"d2Video.mpq",
		"d2Speech.mpq",
	}

	cfg.FullScreen = false
	cfg.WindowScale = 1
	cfg.Display.Width = *ancho
	cfg.Display.Height = *alto

	// OD2_CONFIG_DIR manda (d2config.ConfigDirEnv): el config se escribe ahi con el escritor del motor
	if *salida != "" {
		if err := os.Setenv(d2config.ConfigDirEnv, filepath.Dir(*salida)); err != nil {
			fmt.Fprintln(os.Stderr, "setenv:", err)
			os.Exit(1)
		}
	}

	if err := cfg.Save(); err != nil {
		fmt.Fprintln(os.Stderr, "save:", err)
		os.Exit(1)
	}

	ruta := d2config.DefaultConfigPath()
	if st, err := os.Stat(ruta); err == nil {
		fmt.Printf("config escrito en %s (%d bytes) mpq=%s display=%dx%d\n",
			ruta, st.Size(), cfg.MpqPath, cfg.Display.Width, cfg.Display.Height)
	}
}

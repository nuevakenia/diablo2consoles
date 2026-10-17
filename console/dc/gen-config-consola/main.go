// Genera LA CONFIGURACION DEL ARRANQUE DE LA CONSOLA y el directorio de datos del juego.
//
// QUE PROBLEMA RESUELVE (medido en DC-002.19, evidencia/dc-002.19-seleccion-de-archivos.txt): el arranque
// del motor se frenaba en `d2app/firstrun.go:32` -> `d2setup.EnsureGameFiles` con
//
//     MOTOR-ARRANQUE Run() termino con error: reported to the user: no Diablo II game files were selected
//
// porque en la consola no hay quien elija la carpeta: los dos caminos que el motor ya tiene para no
// preguntar (OD2_SETUP_AUTOPICK y OD2_NO_SETUP) salen de `os.Getenv`, que en la consola es un stub que
// devuelve "", y sin env el camino es `OsaUI` (dialogos de macOS por osascript), que en la consola
// devuelve `cancelled by user`. El UNICO camino que no pregunta nada es que la configuracion ya este
// completa: `Complete(cfg.MpqPath, cfg.MpqLoadOrder)` = true y `EnsureGameFiles` vuelve sin dialogos.
//
// QUE HACE ESTA HERRAMIENTA (corre en el HOST, en tiempo de build):
//
//  1. escribe el `config.json` del arranque con el MISMO escritor del motor (`d2config.Save`, que es el
//     `MarshalJSON` de DC-002.18, el mismo que el corpus byte a byte compara contra el `json` de Go), con
//     `MpqPath` = la ruta de los datos en la CONSOLA y `MpqLoadOrder` = la tabla del propio motor
//     (`d2setup.CoreMPQs`, los archivos que la partida base necesita). OJO: el MpqPath es el de la
//     consola y el `-datos` es el del host que se HORNEA en el romdisk: son el mismo directorio visto
//     desde los dos lados, y por eso los dos se pasan por separado y no se adivina ninguno.
//  2. crea los ARCHIVOS del directorio de datos (los nombres de `CoreMPQs`), vacios: son el instrumento
//     que deja correr la frontera hacia adelante. NO son los MPQ de verdad y no se finge que lo sean: los
//     MPQ reales no entran en el romdisk (16 MB de RAM) y el camino de lectura de verdad es el disco, que
//     es la tarjeta siguiente. Lo que este instrumento compra es medir que el arranque PASA la seleccion
//     de archivos y cual es el error que queda DETRAS (el `gzip` del asset embebido, ya medido).
//
// POR QUE LOS NOMBRES SALEN DE `d2setup.CoreMPQs` Y NO DE UNA LISTA DE ACA: porque si la lista de
// archivos y la lista de la configuracion se escribieran dos veces, una deriva entre las dos se veria
// como un defecto del arranque. Con una sola fuente, el `MpqLoadOrder` de la configuracion y los archivos
// que se crean no pueden desincronizarse.
//
// Uso:
//
//	go run ./console/dc/gen-config-consola -datos <dir_del_host> -config <config.json_del_host>
//	go run ./console/dc/gen-config-consola -datos <dir_del_host> -config <config.json> -sin-sellos
//	go run ./console/dc/gen-config-consola -datos <dir_del_host> -config <config.json> -verificar
//
// `-sin-sellos`: escribe la configuracion pero NO crea los archivos (el mutante "los sellos que no
// estan": la seleccion tiene que volver a frenarse). `-verificar`: EL CONTROL DEL HOST -- no escribe
// nada, lee el `config.json` que ya esta con el `json` DE VERDAD (una struct espejo sin metodos, que es
// la referencia honesta) y exige que diga lo que tiene que decir y que los archivos que nombra esten en
// el directorio que se le pasa.
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/OpenDiablo2/OpenDiablo2/d2app/d2setup"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2config"
)

// rutaDatosConsola es donde el arranque de la consola busca los archivos del juego. Es una CONSTANTE de
// la consola (no se puede pasar por parametro: en la consola no hay linea de comandos ni entorno).
const rutaDatosConsola = "/rd/juego"

// rutaConfigConsola es donde la consola lee su configuracion (`d2config.DefaultConfigPath()`:
// `os.UserConfigDir()` de la consola devuelve "/rd", o sea "/rd/OpenDiablo2/config.json").
const rutaConfigConsola = "/rd/OpenDiablo2/config.json"

// espejo: la misma forma que d2config.Configuration, SIN metodos. El json de Go lo serializa por
// reflexion, asi que es la referencia de verdad para leer lo que hay en el archivo (el mismo espejo que
// console/dc/gen-config-json usa para el corpus de DC-002.18).
type espejo struct {
	MpqLoadOrder    []string
	MpqPath         string
	TicksPerSecond  int
	FpsCap          int
	SfxVolume       float64
	BgmVolume       float64
	FullScreen      bool
	RunInBackground bool
	VsyncEnabled    bool
	Backend         string
	Display         d2config.DisplayConfig
	WindowScale     int
	D2SDir          string
	Language        string              `json:",omitempty"`
	Options         map[string]int      `json:",omitempty"`
	KeyBindings     map[string][]string `json:",omitempty"`
}

func main() {
	datos := flag.String("datos", "", "directorio de datos del juego EN EL HOST (el que se hornea en el romdisk)")
	config := flag.String("config", "", "donde se escribe el config.json (ruta del HOST)")
	sinSellos := flag.Bool("sin-sellos", false, "escribe la configuracion pero no crea los archivos del juego")
	verificar := flag.Bool("verificar", false, "control del host: no escribe nada, revisa el config.json que ya esta")
	flag.Parse()

	if *datos == "" || *config == "" {
		fmt.Fprintln(os.Stderr, "uso: gen-config-consola -datos <dir> -config <config.json> [-sin-sellos|-verificar]")
		os.Exit(2)
	}

	if *verificar {
		os.Exit(verificarConfig(*datos, *config))
	}

	cfg := d2config.DefaultConfig()
	cfg.MpqPath = rutaDatosConsola
	cfg.MpqLoadOrder = append([]string{}, d2setup.CoreMPQs...)
	cfg.SetPath(*config)

	if err := os.MkdirAll(filepath.Dir(*config), 0750); err != nil {
		fmt.Fprintln(os.Stderr, "no pude crear el directorio del config:", err)
		os.Exit(1)
	}

	if err := cfg.Save(); err != nil {
		fmt.Fprintln(os.Stderr, "el motor no pudo escribir su configuracion:", err)
		os.Exit(1)
	}

	sellos := 0

	if !*sinSellos {
		if err := os.MkdirAll(*datos, 0750); err != nil {
			fmt.Fprintln(os.Stderr, "no pude crear el directorio de datos:", err)
			os.Exit(1)
		}

		for _, nombre := range d2setup.CoreMPQs {
			f, err := os.Create(filepath.Join(*datos, nombre))
			if err != nil {
				fmt.Fprintln(os.Stderr, "no pude crear el archivo del juego", nombre+":", err)
				os.Exit(1)
			}

			_ = f.Close()

			sellos++

			fmt.Printf("   sello %s (%d bytes) en %s\n", nombre, 0, filepath.Join(*datos, nombre))
		}
	}

	datosBytes, err := os.ReadFile(*config)
	if err != nil {
		fmt.Fprintln(os.Stderr, "no pude leer de vuelta el config:", err)
		os.Exit(1)
	}

	suma := sha256.Sum256(datosBytes)

	fmt.Printf("CONFIG-CONSOLA config=%s bytes=%d sha256=%s\n", rutaConfigConsola, len(datosBytes),
		hex.EncodeToString(suma[:]))
	fmt.Printf("CONFIG-CONSOLA datos=%s archivos=%d de %d (vacios: el instrumento de la frontera)\n",
		rutaDatosConsola, sellos, len(d2setup.CoreMPQs))
	fmt.Printf("CONFIG-CONSOLA MpqLoadOrder=%d nombres: el mismo orden que crea los archivos, de %s\n",
		len(d2setup.CoreMPQs), "d2setup.CoreMPQs")
}

// verificarConfig: EL CONTROL DEL HOST. Lee el config del romdisk con el json DE VERDAD y exige tres
// cosas: que se parsee, que diga la ruta y el orden del arranque de la consola, y que TODOS los archivos
// que nombra existan en el directorio de datos. Devuelve el codigo de salida (0 = OK).
func verificarConfig(datos, config string) int {
	criollo, err := os.ReadFile(config)
	if err != nil {
		fmt.Println("CONFIG-CONSOLA verificacion FALLO: no pude leer", config+":", err)
		return 1
	}

	var e espejo
	if err := json.Unmarshal(criollo, &e); err != nil {
		fmt.Println("CONFIG-CONSOLA verificacion FALLO: el json de Go no lo lee:", err)
		return 1
	}

	ok := true

	if e.MpqPath != rutaDatosConsola {
		fmt.Printf("CONFIG-CONSOLA verificacion FALLO: MpqPath=%q (tiene que ser %q)\n", e.MpqPath, rutaDatosConsola)
		ok = false
	}

	if len(e.MpqLoadOrder) != len(d2setup.CoreMPQs) {
		fmt.Printf("CONFIG-CONSOLA verificacion FALLO: MpqLoadOrder tiene %d nombres, el motor pide %d\n",
			len(e.MpqLoadOrder), len(d2setup.CoreMPQs))
		ok = false
	} else {
		for i, nombre := range d2setup.CoreMPQs {
			if e.MpqLoadOrder[i] != nombre {
				fmt.Printf("CONFIG-CONSOLA verificacion FALLO: MpqLoadOrder[%d]=%q (el motor lista %q)\n",
					i, e.MpqLoadOrder[i], nombre)
				ok = false
			}
		}
	}

	// los archivos que la configuracion nombra, contados con el os DE VERDAD sobre el directorio que se
	// hornea en el romdisk (el mismo `Complete` que el motor pregunta al arrancar)
	presentes := 0

	for _, nombre := range e.MpqLoadOrder {
		if _, err := os.Stat(filepath.Join(datos, nombre)); err == nil {
			presentes++
		}
	}

	completo := d2setup.Complete(datos, e.MpqLoadOrder)

	fmt.Printf("CONFIG-CONSOLA verificacion: MpqPath=%s orden=%d archivos_en_el_directorio=%d de %d "+
		"Complete(dir, orden)=%v bytes=%d json-de-Go=OK\n", e.MpqPath, len(e.MpqLoadOrder), presentes,
		len(e.MpqLoadOrder), completo, len(criollo))

	if !ok || !completo || presentes != len(e.MpqLoadOrder) {
		fmt.Println("CONFIG-CONSOLA verificacion FALLO: la configuracion del arranque y los archivos no cierran")

		return 1
	}

	fmt.Println("CONFIG-CONSOLA verificacion OK: el config del arranque nombra lo que el romdisk trae")

	return 0
}

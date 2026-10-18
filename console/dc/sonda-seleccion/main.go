// sonda-seleccion: mide LA SELECCION DE ARCHIVOS DEL MOTOR (d2setup) contra un arbol de assets SUELTOS.
// Es una herramienta del HOST y usa los paquetes REALES del motor (`d2app/d2setup`), no una copia: lo que
// aca se mide es lo que el motor hace cuando arranca y decide si tiene los archivos del juego.
//
// DC-002.31, criterio 1 de la tarjeta ("el motor pasa la seleccion con los assets como ARCHIVOS SUELTOS").
// El arranque del motor llama a `d2setup.Complete`/`Validate` (los dos caminos de la seleccion) y despues a
// `d2app/initialization.go:82`:
//
//	a.asset.AddSource(srcPath, types.AssetSourceMPQ)   <- por CADA entrada de MpqLoadOrder
//
// O sea que la seleccion del motor es MPQ-CENTRICA: pide NOMBRES de MPQ (`patch_d2.mpq`, `d2data.mpq`,
// `d2char.mpq`, ...). Un arbol suelto no tiene esos nombres, asi que la pregunta de la tarjeta tiene una
// respuesta MEDIBLE, y este instrumento la contesta: con SOLO sueltos el motor NO pasa su propia seleccion
// y se va al camino de primera ejecucion (`d2setup.EnsureGameFiles`), que en la consola no tiene dialogos
// (la pared de DC-002.19).
//
// MIDE DOS CASOS, los dos con el MISMO codigo del motor:
//  1. SOLO SUELTOS: el arbol extraido de un MPQ (en ese directorio no hay ningun MPQ);
//  2. SOLO MPQ: el directorio del juego como lo espera el motor.
//
// De cada uno: `Complete(dir, orden)` (el predicado que usa `EnsureGameFiles`), `Validate(dir)` (`OK()`,
// cuantos archivos del orden ve y cuantos le faltan) y cuantos ARCHIVOS SUELTOS hay en el directorio — para
// que se vea que el arbol ESTA ahi y que la seleccion no lo mira.
//
// Limite declarado (DC-002.19/27, medido en la consola): `Validate` lista el directorio con `os.ReadDir`, y
// el driver ISO9660 de KOS NO contesta el listado de un directorio (0 entradas). En el host si. Esta sonda
// mide la SELECCION, que es la que decide si el arranque sigue; el listado del disco va aparte.
//
// Uso:  go run ./console/dc/sonda-seleccion -sueltos <dir> -mpqdir <dir> [-orden patch_d2.mpq,d2data.mpq]
package main

import (
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/OpenDiablo2/OpenDiablo2/d2app/d2setup"
)

// cuentaSueltos: cuantos ARCHIVOS (no MPQ) hay en el arbol. Es la prueba de que el arbol suelto esta en el
// directorio que se le pasa a la seleccion.
func cuentaSueltos(dir string) (archivos, carpetas int) {
	_ = filepath.WalkDir(dir, func(_ string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}

		if d.IsDir() {
			carpetas++

			return nil
		}

		if !strings.EqualFold(filepath.Ext(d.Name()), ".mpq") {
			archivos++
		}

		return nil
	})

	return archivos, carpetas
}

// mide: imprime lo que el motor ve en ese directorio con SU propio codigo de seleccion.
func mide(caso, dir string, orden []string) {
	archivos, carpetas := cuentaSueltos(dir)

	v := d2setup.Validate(dir)

	fmt.Printf("SONDA-SELECCION %s dir=%s\n", caso, dir)
	fmt.Printf("SONDA-SELECCION %s sueltos en el dir: %d archivos en %d carpetas\n", caso, archivos, carpetas)
	fmt.Printf("SONDA-SELECCION %s Complete=%v Validate.OK()=%v archivos_que_ve_en_el_orden=%d faltan_core=%d\n",
		caso, d2setup.Complete(dir, orden), v.OK(), len(v.LoadOrder), len(v.MissingCore))

	if len(v.MissingCore) > 0 {
		fmt.Printf("SONDA-SELECCION %s Le faltan (core): %s\n", caso, strings.Join(v.MissingCore, " "))
	}

	if len(v.LoadOrder) > 0 {
		fmt.Printf("SONDA-SELECCION %s Ve (en el orden): %s\n", caso, strings.Join(v.LoadOrder, " "))
	}
}

func main() {
	sueltos := flag.String("sueltos", "", "directorio del arbol suelto (extraido de un MPQ con mpq-extraer)")
	mpqdir := flag.String("mpqdir", "", "directorio con los MPQ del juego (el que espera el motor)")
	orden := flag.String("orden", "patch_d2.mpq,d2data.mpq,d2char.mpq",
		"los MPQ que el motor monta (MpqLoadOrder del config), separados por comas")
	flag.Parse()

	if *sueltos == "" || *mpqdir == "" {
		fmt.Fprintln(os.Stderr,
			"uso: go run ./console/dc/sonda-seleccion -sueltos <directorio suelto> -mpqdir <directorio con MPQ> [-orden a.mpq,b.mpq]")
		os.Exit(2)
	}

	ordenLista := strings.Split(*orden, ",")

	mide("1-solo-sueltos", *sueltos, ordenLista)
	mide("2-solo-mpq", *mpqdir, ordenLista)
}

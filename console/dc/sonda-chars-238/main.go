// sonda-chars-238: busca en el MPQ del disco los DC6/DCC de un PERSONAJE (carpeta chars).
//
// Es una herramienta del HOST, como mpq-extraer: usa el lector de MPQ del motor, tal cual, y NO
// escribe nada del juego en el repo. Sirve para contestar una pregunta concreta antes de tocar el
// POC: cual es la ruta REAL del DC6 del heroe en el MPQ (el motor la arma como
// <base>/<token>/<capa>/<token><capa><valor><modo><arma>.dc6, y aca se comprueba contra el archivo).
//
// Uso:  go run ./console/dc/sonda-chars-238 -mpq <ruta.mpq> [-filtro chars] [-modo NU]
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2mpq"
)

func main() {
	mpqPath := flag.String("mpq", "", "archivo MPQ de entrada")
	filtro := flag.String("filtro", "chars", "subcadena que tiene que aparecer en el nombre (vacio = todas)")
	token := flag.String("token", "AM", "token del personaje (AM, SO, NE, PA, BA, DZ, AI)")
	modo := flag.String("modo", "NU", "modo de animacion (NU, WL, TN, TW, A1, ...)")
	flag.Parse()

	if *mpqPath == "" {
		fmt.Fprintln(os.Stderr, "uso: sonda-chars-238 -mpq <ruta.mpq> [-filtro chars] [-token AM] [-modo NU]")
		os.Exit(2)
	}

	mpq, err := d2mpq.FromFile(*mpqPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "SONDA238 no pude abrir el MPQ:", err)
		os.Exit(1)
	}
	defer mpq.Close()

	lista, err := mpq.Listfile()
	if err != nil {
		fmt.Fprintln(os.Stderr, "SONDA238 no pude leer el (listfile):", err)
		os.Exit(1)
	}

	fmt.Println("SONDA238 entradas en el listfile:", len(lista))

	conChars, conToken, conModo := 0, 0, 0

	for _, nombre := range lista {
		bajo := strings.ToLower(nombre)
		if *filtro != "" && !strings.Contains(bajo, strings.ToLower(*filtro)) {
			continue
		}

		conChars++

		if *token != "" && strings.Contains(strings.ToUpper(bajo), strings.ToUpper(*token)) {
			conToken++
		}

		if *modo != "" && strings.Contains(strings.ToUpper(bajo), strings.ToUpper(*modo)) {
			conModo++
			fmt.Println("SONDA238 MODO:", nombre)
		} else if *modo == "" && (*token == "" || strings.Contains(strings.ToUpper(bajo), strings.ToUpper(*token))) {
			fmt.Println("SONDA238 ARCH:", nombre)
		}
	}

	fmt.Println("SONDA238 con filtro", *filtro, "=", conChars, "| con token", *token, "=", conToken, "| con modo", *modo, "=", conModo)
}

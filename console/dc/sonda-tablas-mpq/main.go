// sonda-tablas-mpq: LAS TABLAS DEL MPQ, MEDIDAS EN EL HOST CON EL MISMO CODIGO DEL MOTOR.
//
// Es la mitad de host del par de instrumentos de DC-002.33 (criterio 2 de la tarjeta: "el camino del host y
// la consola dan las MISMAS tablas: el hash de la tabla entera igual en los dos lados"). La otra mitad es el
// POC `console/dc/poc/tablas-mpq`, que corre en la consola y imprime los MISMOS seis numeros por archivo:
//
//     entradasHash  entradasBloque  nombres  contenidos  checksumHash  checksumBloque
//
//   - `entradasHash`/`entradasBloque` son `MPQ.TableEntries()` (cuantas ranuras tiene cada tabla);
//   - `checksumHash`/`checksumBloque` son `MPQ.TableChecksum()`, la suma `suma = suma*31 + byte` de los
//     BYTES de cada tabla descifrada. Con las tablas guardadas como bytes contiguos (DC-002.33) la tabla
//     ENTERA se compara con DOS numeros, que es justo lo que la tarjeta pedia;
//   - `nombres` y `contenidos` son el `(listfile)` del archivo leido por el camino del motor (`ReadFile`)
//     y cuantos de esos nombres encuentra la CONSULTA (`Contains` = el sondeo sobre los bytes).
//
// Uso:  go run ./console/dc/sonda-tablas-mpq -mpqdir <dir> [-mpq patch_d2.mpq,d2data.mpq,d2char.mpq]
//       (por defecto el orden del MOTOR: `d2setup.CoreMPQs`, filtrado por la lista pedida — es el mismo
//        filtro y el mismo orden que usa `gen-config-consola`, que es de donde sale el MpqLoadOrder del
//        config que lee la consola).
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/OpenDiablo2/OpenDiablo2/d2app/d2setup"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2mpq"
)

// ordenDelMotor: espejo del filtro de `gen-config-consola`: la lista pedida, en el orden en que el MOTOR
// los lista (`d2setup.CoreMPQs`). Sin `-mpq`, el orden del motor entero.
func ordenDelMotor(pedidos string) ([]string, error) {
	if pedidos == "" {
		return append([]string{}, d2setup.CoreMPQs...), nil
	}

	quiero := map[string]bool{}

	for _, n := range strings.Split(pedidos, ",") {
		n = strings.ToLower(strings.TrimSpace(n))
		if n != "" {
			quiero[n] = true
		}
	}

	orden := []string{}

	for _, n := range d2setup.CoreMPQs {
		if quiero[n] {
			orden = append(orden, n)
		}
	}

	if len(orden) != len(quiero) {
		return nil, fmt.Errorf("quedaron %d nombres de %d", len(orden), len(quiero))
	}

	return orden, nil
}

// numerosDeUnMPQ: los seis numeros de un archivo, con el mismo criterio que el POC de la consola. Si
// `nombresDir` no es vacio, ademas deja los nombres del `(listfile)` en un archivo de texto plano: es el
// camino por el que la CONSOLA recibe los nombres (medido: el `ReadFile("(listfile)")` de la consola FALLA
// con `zlib: invalid header`, un defecto aparte de esta tarjeta), para poder consultarlos con SU propio
// sondeo sobre los bytes.
func numerosDeUnMPQ(ruta, nombresDir string) (he, be uint32, nombres, contenidos int, ch, cb uint32, err error) {
	m, err := d2mpq.FromFile(ruta)
	if err != nil {
		return 0, 0, 0, 0, 0, 0, err
	}

	defer m.Close()

	he, be = m.TableEntries()
	ch, cb = m.TableChecksum()

	if datos, errLf := m.ReadFile("(listfile)"); errLf == nil {
		lista := ""

		for _, linea := range strings.Split(string(datos), "\n") {
			nombre := strings.TrimSpace(strings.TrimRight(linea, "\x00"))
			if nombre == "" {
				continue
			}

			nombres++

			if m.Contains(nombre) {
				contenidos++
			}

			lista += nombre + "\n"
		}

		if nombresDir != "" && lista != "" {
			archivo := filepath.Join(nombresDir, "nombres_"+strings.ReplaceAll(strings.TrimSuffix(
				filepath.Base(ruta), ".mpq"), ".", "_")+".txt")

			if errEsc := os.WriteFile(archivo, []byte(lista), 0o644); errEsc != nil {
				fmt.Fprintln(os.Stderr, "SONDA-TABLAS no pude escribir", archivo+":", errEsc)
			}
		}
	} else {
		fmt.Fprintln(os.Stderr, "SONDA-TABLAS", filepath.Base(ruta), "(listfile) FALLO:", errLf)
	}

	return he, be, nombres, contenidos, ch, cb, nil
}

func main() {
	mpqdir := flag.String("mpqdir", "", "directorio de la instalacion de Diablo II (los .mpq)")
	pedidos := flag.String("mpq", "", "nombres separados por coma (por defecto: los 7 de d2setup.CoreMPQs)")
	nombresDir := flag.String("nombres-dir", "", "si se pasa, deja los nombres del (listfile) de cada MPQ ahi (nombres_<mpq>.txt)")
	flag.Parse()

	if *mpqdir == "" {
		fmt.Fprintln(os.Stderr, "uso: go run ./console/dc/sonda-tablas-mpq -mpqdir <dir> [-mpq a,b,c]")
		os.Exit(2)
	}

	orden, err := ordenDelMotor(*pedidos)
	if err != nil {
		fmt.Fprintln(os.Stderr, "SONDA-TABLAS:", err)
		os.Exit(2)
	}

	fmt.Println("TABLAS-HOST orden del motor:", len(orden), "archivos")

	archivos, totalNombres, totalContenidos := 0, 0, 0

	for i, nombre := range orden {
		ruta := filepath.Join(*mpqdir, nombre)

		if _, err := os.Stat(ruta); err != nil {
			fmt.Println("TABLAS-HOST", i+1, nombre, "FALTA:", err)
			continue
		}

		he, be, nombres, contenidos, ch, cb, err := numerosDeUnMPQ(ruta, *nombresDir)
		if err != nil {
			fmt.Println("TABLAS-HOST", i+1, nombre, "FALLO:", err)
			continue
		}

		fmt.Println("TABLAS-HOST", i+1, nombre,
			"entradasHash=", he, "entradasBloque=", be,
			"nombres=", nombres, "contenidos=", contenidos,
			"checksumHash=", ch, "checksumBloque=", cb)

		archivos++
		totalNombres += nombres
		totalContenidos += contenidos
	}

	fmt.Println("TABLAS-HOST-TOTAL archivos=", archivos, "nombres=", totalNombres, "contenidos=", totalContenidos)
}

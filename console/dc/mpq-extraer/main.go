// mpq-extraer: extrae TODOS los archivos de un MPQ de Diablo II a un directorio del host.
//
// DC-002.31 — el empaquetado de los assets SIN MPQ. Es una herramienta del HOST (como bake-list y
// bake-dc6): usa el lector de MPQ del motor, tal cual, y sirve para MEDIR lo que la tarjeta pregunta
// antes de elegir: cuanto pesa un MPQ descomprimido en archivos sueltos (un MPQ comprime, asi que lo
// suelto puede pesar MAS), cuantos archivos son y cuantas COLISIONES de nombre hay (el MPQ no distingue
// mayusculas y el disco si: dos entradas que solo difieren en mayusculas se pisarian al extraer).
//
// POR QUE LOS NOMBRES SE ESCRIBEN CON BARRAS NORMALES: los nombres del MPQ vienen con barras INVERTIDAS
// (data\global\...), que es la convencion de Windows. En el disco del host van con "/" y el directorio
// creado es el arbol de verdad.
//
// Uso:  go run ./console/dc/mpq-extraer -mpq <ruta.mpq> -dest <directorio> [-limite N] [-detalle-errores N]
//
// `-detalle-errores N` imprime las primeras N entradas que no se pudieron leer CON el motivo: el
// conteo solo no dice si el arbol suelto queda completo, y para elegir el empaquetado eso importa.
//
// No entra al repo ningun dato del juego: esto escribe en un directorio de trabajo descartable.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2mpq"
)

func main() {
	mpqPath := flag.String("mpq", "", "archivo MPQ de entrada")
	dest := flag.String("dest", "", "directorio de salida (se crea si no existe)")
	limite := flag.Int("limite", 0, "cortar despues de N entradas (0 = todas)")
	minusculas := flag.Bool("minusculas", false, "escribir los nombres en minusculas (el motor pide los assets en minusculas y el MPQ no distingue mayusculas: el disco si)")
	detalle := flag.Int("detalle-errores", 0, "imprimir los primeros N errores de lectura con el nombre de la entrada y el motivo (0 = solo el conteo)")
	flag.Parse()

	if *mpqPath == "" || *dest == "" {
		fmt.Fprintln(os.Stderr, "uso: go run ./console/dc/mpq-extraer -mpq <archivo.mpq> -dest <directorio> [-limite N]")
		os.Exit(2)
	}

	mpq, err := d2mpq.FromFile(*mpqPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "MPQ-EXTRAER no pude abrir el MPQ:", err)
		os.Exit(1)
	}
	defer mpq.Close()

	lista, err := mpq.Listfile()
	if err != nil {
		fmt.Fprintln(os.Stderr, "MPQ-EXTRAER no pude leer el (listfile):", err)
		os.Exit(1)
	}

	// ordenado: la corrida tiene que ser la misma dos veces (y el conteo reproducible)
	sort.Strings(lista)

	fmt.Printf("MPQ-EXTRAER mpq=%s bytes_mpq=%d entradas_listfile=%d\n",
		filepath.Base(*mpqPath), mpq.Size(), len(lista))

	var (
		archivos   int
		bytes      int64
		errores    int
		colisiones int
		vistos     = map[string]int{}
		exts       = map[string]int{}
	)

	for i, nombre := range lista {
		if *limite > 0 && i >= *limite {
			break
		}

		datos, err := mpq.ReadFile(nombre)
		if err != nil {
			errores++
			if errores <= *detalle {
				fmt.Printf("MPQ-EXTRAER error entrada=%q motivo=%v\n", nombre, err)
			}

			continue
		}

		rel := filepath.FromSlash(strings.ReplaceAll(nombre, "\\", "/"))
		if *minusculas {
			rel = strings.ToLower(rel)
		}

		ruta := filepath.Join(*dest, rel)

		if _, ya := vistos[ruta]; ya {
			colisiones++
		}

		vistos[ruta]++

		if err := os.MkdirAll(filepath.Dir(ruta), 0o755); err != nil {
			fmt.Fprintln(os.Stderr, "MPQ-EXTRAER no pude crear", filepath.Dir(ruta)+":", err)
			os.Exit(1)
		}

		if err := os.WriteFile(ruta, datos, 0o644); err != nil {
			fmt.Fprintln(os.Stderr, "MPQ-EXTRAER no pude escribir", ruta+":", err)
			os.Exit(1)
		}

		archivos++
		bytes += int64(len(datos))

		if e := strings.ToLower(filepath.Ext(ruta)); e != "" {
			exts[e]++
		}
	}

	fmt.Printf("MPQ-EXTRAER archivos=%d bytes_sueltos=%d errores=%d colisiones_de_nombre=%d\n",
		archivos, bytes, errores, colisiones)
	fmt.Printf("MPQ-EXTRAER proporcion=%.4f (suelto/MPQ)\n", float64(bytes)/float64(mpq.Size()))

	// las extensiones, de la mas comun a la menos: dice de que esta hecho el MPQ
	tipo := make([]string, 0, len(exts))

	for e := range exts {
		tipo = append(tipo, e)
	}

	sort.Slice(tipo, func(i, j int) bool { return exts[tipo[i]] > exts[tipo[j]] })

	for i, e := range tipo {
		if i >= 10 {
			break
		}

		fmt.Printf("MPQ-EXTRAER   %-8s %6d archivos\n", e, exts[e])
	}
}

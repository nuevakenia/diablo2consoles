// DC-002.42: ARMA EL ARCHIVO DE LA TABLA DE objectLookups QUE LA CONSOLA LEE DEL DISCO.
//
// Corre en el HOST, con la MISMA tabla que antes viajaba en el binario (`d2records.ObjectLookupsCompilados()`,
// el volcado de object_lookup_record_data.go, que el build de la consola ya no compila). Escribe
// `objectlookups.pack` con el formato que lee `d2core/d2records/object_lookups_store.go` y LO VERIFICA con el
// mismo lector que usa la consola (abre, recorre y compara el crc de la cabecera).
//
// Uso:  go run ./console/dc/gen-lookups-pack -o <ruta_del_pack>
//
//	go run ./console/dc/gen-lookups-pack -dir <carpeta>      (escribe <carpeta>/objectlookups.pack)
//
// El archivo va en el DISCO: la carpeta de los MPQ (que viaja como /JUEGO en el ISO9660) es donde la consola
// lo busca por defecto (`d2records.RutaDeLosLookups` = /cd/juego/objectlookups.pack).
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2records"
)

func main() {
	salida := flag.String("o", "", "archivo de salida (por defecto objectlookups.pack)")
	directorio := flag.String("dir", "", "carpeta donde dejar objectlookups.pack (se crea si falta)")
	flag.Parse()

	ruta := *salida
	if *directorio != "" {
		if err := os.MkdirAll(*directorio, 0o755); err != nil {
			fatal("no pude crear %s: %v", *directorio, err)
		}

		ruta = filepath.Join(*directorio, "objectlookups.pack")
	}

	if ruta == "" {
		fatal("falta -o <ruta> o -dir <carpeta>")
	}

	tabla := d2records.ObjectLookupsCompilados()
	fmt.Printf("PACK-242 tabla compilada: %d registros (object_lookup_record_data.go)\n", len(tabla))

	archivo, err := os.Create(ruta)
	if err != nil {
		fatal("no pude escribir %s: %v", ruta, err)
	}

	informe, err := d2records.EscribirPack(archivo, tabla)
	if err != nil {
		_ = archivo.Close()
		fatal("EscribirPack: %v", err)
	}

	if err := archivo.Close(); err != nil {
		fatal("cerrando %s: %v", ruta, err)
	}

	fmt.Printf("PACK-242 archivo: %s\n", ruta)
	fmt.Printf("PACK-242 bytes=%d registros=%d grupos=%d maxRegistro=%d crc=%d\n",
		informe.Bytes, informe.Registros, informe.Grupos, informe.MaxRegistro, informe.Crc)

	// LA VERIFICACION, con el lector de la consola: abre, recorre los registros y compara el crc.
	leido, err := d2records.AbrirLookupsDelArchivo(ruta)
	if err != nil {
		fatal("el archivo que acabo de escribir no se deja leer: %v", err)
	}

	if leido.Registros != len(tabla) {
		fatal("el archivo dice %d registros y la tabla tiene %d", leido.Registros, len(tabla))
	}

	fmt.Printf("PACK-242 verificado: bytes=%d registros=%d grupos=%d maxRegistro=%d crc=%d crcLeida=%d\n",
		leido.Bytes, leido.Registros, leido.Grupos, leido.MaxRegistro, leido.CrcArchivo, leido.CrcLeida)
	fmt.Printf("PACK-242 indice en RAM: %d bytes (4 x %d) + ventana de lectura de %d\n",
		leido.BytesDeIndice, leido.Registros, 4096)
}

func fatal(formato string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "PACK-242 FALLO: "+formato+"\n", args...)
	os.Exit(1)
}

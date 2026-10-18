// Hornea un DC6 CRUDO de un MPQ de Diablo II, tal cual sale del archivo, para el POC del motor.
//
// Por que crudo: en DC-002.3 lo que se mide es que el CODIGO DEL MOTOR (d2dc6.Load + DecodeFrame)
// compile para SH-4 y de el mismo resultado que en el host. Entonces lo que se hornea son los bytes
// originales del MPQ, no una version ya decodificada: la consola tiene que parsear y decodificar
// igual que el host, con el mismo codigo y la misma entrada.
//
// Salida en -out:
//
//	<name>.dc6          los bytes crudos (los consume el comparador del host)
//	gen/<name>_dc6.go   los mismos bytes como var de Go (los consume el POC de la consola, que no
//	                    tiene sistema de archivos)
//
// Uso:  go run ./console/dc/bake-dc6 -mpq <ruta.mpq> -dc6 'data\global\ui\...\x.dc6' -out <dir> -name d2logo
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2mpq"
)

func main() {
	var (
		mpqPath string
		dc6Path string
		palPath string
		outDir  string
		name    string
		soloPal bool
	)
	flag.StringVar(&mpqPath, "mpq", "", "archivo MPQ de entrada")
	flag.StringVar(&dc6Path, "dc6", "", "ruta del DC6 dentro del MPQ")
	flag.StringVar(&palPath, "pal", "", "ruta del pal.dat dentro del MPQ (opcional: DC-002.9, para el color)")
	flag.StringVar(&outDir, "out", "", "directorio de salida")
	flag.StringVar(&name, "name", "asset", "nombre base de los archivos de salida")
	// DC-002.35: -solo-pal saca SOLO la paleta, del MPQ que la tiene. Hace falta porque el DC6 de un
	// hito puede vivir en un MPQ (patch_d2.mpq) y su paleta en otro (d2data.mpq): con -dc6 obligatorio,
	// pedir la paleta del MPQ grande fallaba con "file not found" del DC6 y abortaba el build entero.
	flag.BoolVar(&soloPal, "solo-pal", false, "extraer solo la paleta (no toca el DC6)")
	flag.Parse()

	if mpqPath == "" || outDir == "" || (dc6Path == "" && !soloPal) {
		fatal("faltan -mpq, -out y/o -dc6 (o -solo-pal)")
	}

	mpq, err := d2mpq.FromFile(mpqPath)
	if err != nil {
		fatal("abriendo MPQ: %v", err)
	}

	if err := os.MkdirAll(filepath.Join(outDir, "gen"), 0o755); err != nil {
		fatal("creando %s: %v", outDir, err)
	}

	if !soloPal {
		raw, err := mpq.ReadFile(dc6Path)
		if err != nil {
			fatal("leyendo %s del MPQ: %v", dc6Path, err)
		}

		rawPath := filepath.Join(outDir, name+".dc6")
		if err := os.WriteFile(rawPath, raw, 0o644); err != nil {
			fatal("escribiendo %s: %v", rawPath, err)
		}

		goPath := filepath.Join(outDir, "gen", name+"_dc6.go")
		if err := os.WriteFile(goPath, []byte(varGo(dc6Path, "assetDC6", raw)), 0o644); err != nil {
			fatal("escribiendo %s: %v", goPath, err)
		}

		fmt.Printf("dc6 crudo: %s (%d bytes)\n", rawPath, len(raw))
		fmt.Printf("  -> %s (para la consola)\n", goPath)
	}

	// --- la paleta (DC-002.9) ---
	if palPath != "" {
		pal, err := mpq.ReadFile(palPath)
		if err != nil {
			fatal("leyendo %s del MPQ: %v", palPath, err)
		}

		palRawPath := filepath.Join(outDir, name+".pal")
		if err := os.WriteFile(palRawPath, pal, 0o644); err != nil {
			fatal("escribiendo %s: %v", palRawPath, err)
		}

		palGoPath := filepath.Join(outDir, "gen", name+"_pal.go")
		if err := os.WriteFile(palGoPath, []byte(varGo(palPath, "assetPAL", pal)), 0o644); err != nil {
			fatal("escribiendo %s: %v", palGoPath, err)
		}

		fmt.Printf("pal crudo: %s (%d bytes)\n", palRawPath, len(pal))
		fmt.Printf("  -> %s (para la consola)\n", palGoPath)
	}
}

// varGo arma el archivo de Go con los bytes crudos como una var. El POC de la consola no tiene sistema
// de archivos: los bytes entran en el binario.
func varGo(origen, varName string, raw []byte) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "// GENERADO por console/dc/bake-dc6 desde %s. NO EDITAR A MANO.\n", origen)
	fmt.Fprintf(&sb, "// Son los bytes crudos tal como estan en el MPQ (%d bytes): la consola los parsea con\n", len(raw))
	fmt.Fprintf(&sb, "// el mismo codigo del motor que usa el host.\n\npackage main\n\nvar %s = []byte{\n", varName)
	for i, b := range raw {
		if i%12 == 0 {
			sb.WriteString("	")
		}
		fmt.Fprintf(&sb, "0x%02x,", b)
		if i%12 == 11 {
			sb.WriteString("\n")
		}
	}
	if len(raw)%12 != 0 {
		sb.WriteString("\n")
	}
	sb.WriteString("}\n")

	return sb.String()
}

func fatal(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "error: "+format+"\n", args...)
	os.Exit(1)
}

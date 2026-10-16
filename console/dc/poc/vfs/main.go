//go:build dreamcast

// DC-002.6: el POC del VFS. Prueba que en la consola EXISTE un sistema de archivos: los archivos
// viajan adentro del ELF (romdisk montado en /rd por KOS) y el paquete os de la stdlib minima los lee.
//
// Lo que mide, en la consola serie:
//   1. os.ReadFile de un texto del romdisk
//   2. os.ReadFile de un binario, con la suma de los bytes (para ver que devuelve LOS bytes, no otros)
//   3. os.ReadFile de algo que no esta: tiene que dar error, no colgarse ni mentir
//   4. Open/Read por trozos/Close/Size
//   5. Existe, Getenv (vacio), Args
//   6. escribir por os.Stdout (que en la consola es la consola serie)
package main

import (
	"os"
	"path/filepath"
	"strconv"
)

//extern d2c_serial_init
func d2c_serial_init()

//extern d2c_pantalla_init
func d2c_pantalla_init()

//extern d2c_pantalla_color
func d2c_pantalla_color(r, g, b float32)

//extern d2c_frame
func d2c_frame()

var bien, mal int

// relleno4: el numero con 4 digitos (Go de la consola todavia no tiene el ancho '%04d').
func relleno4(n int) string {
	s := strconv.Itoa(n)
	for len(s) < 4 {
		s = "0" + s
	}
	return s
}

// primero y segundo: las dos partes de Split (que devuelve dos valores).
func primero(a, b string) string  { return a }
func segundo(a, b string) string  { return b }

// absDe: Abs sin el error (en la consola no hay directorio actual: ver el paquete).
func absDe(p string) string {
	r, _ := filepath.Abs(p)
	return r
}

// tamanos: la lista de tamanos que trae el manifiesto, en orden.
func tamanos(entradas []os.DirEntry) string {
	s := ""
	for _, e := range entradas {
		s += strconv.FormatInt(e.Size(), 10) + " "
	}
	return s
}

// igualdad: donde difieren dos textos (vacio = iguales).
func igualdad(a []byte, b string) string {
	if len(a) != len(b) {
		return "distinto tamano " + strconv.Itoa(len(a)) + " vs " + strconv.Itoa(len(b))
	}
	for i := 0; i < len(a); i++ {
		if a[i] != b[i] {
			return "difieren en el byte " + strconv.Itoa(i)
		}
	}
	return "igual"
}

func check(nombre, esperado, tengo string) {
	if esperado == tengo {
		bien++
		return
	}
	mal++
	println("VFS FALLO:", nombre, "esperaba", strconv.Quote(esperado), "y dio", strconv.Quote(tengo))
}

func main() {
	d2c_serial_init()
	println("VFS: arranca el POC del sistema de archivos de la consola (DC-002.6)")

	// 1. un texto del romdisk
	datos, err := os.ReadFile("/rd/hola.txt")
	if err != nil {
		check("ReadFile /rd/hola.txt", "sin error", "error: "+err.Error())
	} else {
		check("ReadFile /rd/hola.txt", "hola desde el romdisk de la Dreamcast", string(datos))
	}

	// 2. un binario: la suma de los bytes tiene que dar lo que se horneo (0..255)
	bin, err := os.ReadFile("/rd/binario.bin")
	if err != nil {
		check("ReadFile /rd/binario.bin", "sin error", "error: "+err.Error())
	} else {
		suma := 0
		for _, b := range bin {
			suma += int(b)
		}
		check("ReadFile /rd/binario.bin", "256 bytes suma 32640", strconv.Itoa(len(bin))+" bytes suma "+strconv.Itoa(suma))
	}

	// 3. algo que no esta
	_, err = os.ReadFile("/rd/no-esta.txt")
	if err == nil {
		check("ReadFile de algo que no esta", "error", "sin error")
	} else {
		check("ReadFile de algo que no esta", "error y no me colge", "error y no me colge")
	}

	// 4. Open/Read por trozos/Close/Size
	f, err := os.Open("/rd/hola.txt")
	if err != nil {
		check("Open", "sin error", "error: "+err.Error())
	} else {
		check("Open Size", strconv.Itoa(len("hola desde el romdisk de la Dreamcast")), strconv.Itoa(int(f.Size())))
		var trozos []byte
		buf := make([]byte, 8)
		for {
			n, err := f.Read(buf)
			if n > 0 {
				trozos = append(trozos, buf[:n]...)
			}
			if err != nil {
				break
			}
		}
		f.Close()
		check("Open/Read por trozos de 8", "hola desde el romdisk de la Dreamcast", string(trozos))
	}

	// 4b. un archivo de 6000 bytes: mas grande que el buffer interno de ReadFile (4096), asi que
	//     prueba que arma el resultado en varias vueltas sin perder ni repetir bytes
	gr, err := os.ReadFile("/rd/grande.txt")
	if err != nil {
		check("ReadFile /rd/grande.txt", "sin error", "error: "+err.Error())
	} else {
		// 200 lineas de "linea NNNN del archivo grande\n" = 30 bytes por linea = 6000
		esperado := ""
		for i := 0; i < 200; i++ {
			esperado += "linea " + relleno4(i) + " del archivo grande\n"
		}
		check("ReadFile grande.txt (6000 bytes en varias vueltas)", strconv.Itoa(len(esperado)),
			strconv.Itoa(len(gr)))
		check("grande.txt contenido", "igual", igualdad(gr, esperado))
	}

	// 4c. os.ReadDir del romdisk: sale del .manifiesto horneado (el FS ROM no deja listar) y tiene que
	// dar los TRES archivos, sin el propio manifiesto. Se mide tambien /cd, que en este POC no existe.
	if e2, err2 := os.ReadDir("/cd"); err2 != nil {
		println("VFS: ReadDir /cd ->", err2.Error(), "(declarado: el GD-ROM no esta en este POC)")
	} else {
		println("VFS: ReadDir /cd ->", strconv.Itoa(len(e2)), "entradas")
	}
	entradas, err := os.ReadDir("/rd")
	if err != nil {
		check("ReadDir /rd", "sin error", "error: "+err.Error())
	} else {
		check("ReadDir /rd", "3", strconv.Itoa(len(entradas)))
		nombres := ""
		tipos := ""
		for _, e := range entradas {
			nombres += e.Name() + " "
			tipos += strconv.Itoa(e.TipoCrudo()) + " "
		}
		println("VFS: ReadDir /rd ->", nombres, "| tipos:", tipos)
		if len(entradas) == 3 {
			check("ReadDir /rd nombres", "binario.bin grande.txt hola.txt ", nombres)
			check("ReadDir /rd tamanos", "256 6000 37 ", tamanos(entradas))
		}
	}

	// 4d. os.Stat y los errores del motor (IsNotExist)
	info, err := os.Stat("/rd/hola.txt")
	if err != nil {
		check("Stat hola.txt", "sin error", "error: "+err.Error())
	} else {
		check("Stat hola.txt tamano", "37", strconv.FormatInt(info.Size(), 10))
		check("Stat hola.txt es dir", "false", strconv.FormatBool(info.IsDir()))
	}
	_, err = os.Stat("/rd/no-esta")
	check("IsNotExist(Stat de algo que no esta)", "true", strconv.FormatBool(os.IsNotExist(err)))
	if e3, err3 := os.ReadDir("/rd/no-existe"); err3 != nil {
		check("IsNotExist(ReadDir de algo que no esta)", "true", strconv.FormatBool(os.IsNotExist(err3)))
	} else {
		check("ReadDir de algo que no esta", "error", strconv.Itoa(len(e3))+" entradas")
	}

	// 4e. path/filepath compilado para SH-4: los mismos casos que la prueba diferencial del host
	// (difftest/dc0026b_test.go), con el resultado que da el filepath de Go.
	pares := []struct{ nombre, esperado, tengo string }{
		{"Join(/rd, hola.txt)", "/rd/hola.txt", filepath.Join("/rd", "hola.txt")},
		{"Join(a,..,b)", "b", filepath.Join("a", "..", "b")},
		{"Join de vacios", "", filepath.Join("", "")},
		{"Clean(a/b/../c)", "a/c", filepath.Clean("a/b/../c")},
		{"Clean(/../a)", "/a", filepath.Clean("/../a")},
		{"Clean(//a//b//)", "/a/b", filepath.Clean("//a//b//")},
		{"Dir(/a/b/)", "/a/b", filepath.Dir("/a/b/")},
		{"Dir(a)", ".", filepath.Dir("a")},
		{"Base(/a/b.txt)", "b.txt", filepath.Base("/a/b.txt")},
		{"Ext(a.txt)", ".txt", filepath.Ext("a.txt")},
		{"Ext(.bashrc)", ".bashrc", filepath.Ext(".bashrc")},
		{"Split(a/b/) dir", "a/b/", primero(filepath.Split("a/b/"))},
		{"Split(a/b/) arch", "", segundo(filepath.Split("a/b/"))},
		{"IsAbs(/rd)", "true", strconv.FormatBool(filepath.IsAbs("/rd"))},
		{"IsAbs(rd/x)", "false", strconv.FormatBool(filepath.IsAbs("rd/x"))},
		{"Abs(a/b)", "/a/b", absDe("a/b")},
	}
	for _, c := range pares {
		check("filepath "+c.nombre, c.esperado, c.tengo)
	}

	// 5. Existe, Getenv y Args
	check("Existe /rd/hola.txt", "true", strconv.FormatBool(os.Existe("/rd/hola.txt")))
	check("Existe /rd/nada", "false", strconv.FormatBool(os.Existe("/rd/nada")))
	check("Getenv (no hay entorno)", "", os.Getenv("PATH"))
	check("Args tiene el programa", "/rd/prog.elf", os.Args[0])

	// 6. la salida de texto por os.Stdout
	os.Stdout.WriteString("VFS: escrito por os.Stdout\n")
	os.Stderr.WriteString("VFS: y por os.Stderr\n")

	println("VFS: casos", strconv.Itoa(bien+mal), "bien", strconv.Itoa(bien), "mal", strconv.Itoa(mal))
	if mal == 0 {
		println("VFS: RESULTADO OK - el sistema de archivos de la consola existe y lee")
	} else {
		println("VFS: RESULTADO MAL")
	}

	d2c_pantalla_init()
	if mal == 0 {
		d2c_pantalla_color(0.0, 0.5, 0.0)
	} else {
		d2c_pantalla_color(0.6, 0.0, 0.0)
	}
	for {
		d2c_frame()
	}
}

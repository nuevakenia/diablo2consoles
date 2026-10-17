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
	"bufio"
	"io/fs"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"time"
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

// bufLog: un io.Writer que acumula, para medir el texto exacto que escribe el log.
type bufLog struct{ datos []byte }

func (b *bufLog) Write(p []byte) (int, error) {
	b.datos = append(b.datos, p...)
	return len(p), nil
}

func (b *bufLog) texto() string { return string(b.datos) }

// si: un bool a texto (para los casos que comparan booleanos).
func si(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

// primero y segundo: las dos partes de Split (que devuelve dos valores).
func primero(a, b string) string  { return a }
func segundo(a, b string) string  { return b }

// absDe: Abs sin el error (en la consola no hay directorio actual: ver el paquete).
func absDe(p string) string {
	r, _ := filepath.Abs(p)
	return r
}

// tamanos: la lista de tamanos del manifiesto, en orden. Va por Info() y no por un Size() de la
// interfaz: os.DirEntry ES fs.DirEntry (alias, DC-002.8) y esa interfaz no declara Size. Antes del
// alias el POC usaba un os.DirEntry propio mas ancho (con Size y TipoCrudo) y el alias lo dejo sin
// compilar: medido en DC-002.8a, este POC no compilaba desde 95e0ef74.
func tamanos(entradas []os.DirEntry) string {
	s := ""
	for _, e := range entradas {
		info, err := e.Info()
		if err != nil {
			return "info: " + err.Error()
		}
		s += strconv.FormatInt(info.Size(), 10) + " "
	}
	return s
}

// dirs: cuales de las entradas son directorios, en orden. Es el tipo que trae el manifiesto.
func dirs(entradas []os.DirEntry) string {
	s := ""
	for _, e := range entradas {
		s += si(e.IsDir()) + " "
	}
	return s
}

// modos: para cada entrada, si su Type() es fs.ModeDir (lo mismo que IsDir, por el otro camino).
func modos(entradas []os.DirEntry) string {
	s := ""
	for _, e := range entradas {
		s += si(e.Type() == fs.ModeDir) + " "
	}
	return s
}

// identidades: os.FileInfo/DirEntry/FileMode tienen que ser EL MISMO tipo que los de io/fs (alias).
// No mide nada en tiempo de ejecucion: si dejan de serlo, el POC NO COMPILA, y eso es exactamente el
// sintoma que bloqueaba al motor (d2mpq/mpq.go:204: "cannot use type []os.FileInfo as type
// []fs.FileInfo" al asignar lo que devuelve ioutil.ReadDir). El mutante que lo exige vive en
// scripts/console/mutar-tipos-os.sh.
func identidades() {
	var a []os.FileInfo
	var b []fs.FileInfo
	a = b
	b = a
	var c []os.DirEntry
	var d []fs.DirEntry
	c = d
	d = c
	var e os.FileMode
	var f fs.FileMode
	e = f
	f = e
	_, _, _, _, _, _ = a, b, c, d, e, f
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

// errTxt: un error a texto ("nil" si no hay), para los casos que comparan errores.
func errTxt(e error) string {
	if e == nil {
		return "nil"
	}
	return e.Error()
}

// grandeEsperado: las 200 lineas de grande.txt (30 bytes cada una = 6000).
func grandeEsperado() string {
	s := ""
	for i := 0; i < 200; i++ {
		s += "linea " + relleno4(i) + " del archivo grande\n"
	}
	return s
}

// check: un caso del corpus: compara el texto esperado con el medido.
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
		esperado := grandeEsperado()
		check("ReadFile grande.txt (6000 bytes en varias vueltas)", strconv.Itoa(len(esperado)),
			strconv.Itoa(len(gr)))
		check("grande.txt contenido", "igual", igualdad(gr, esperado))
	}

	// 4c. os.ReadDir del romdisk: sale del .manifiesto horneado (el FS ROM no deja listar) y tiene que
	// dar las CUATRO entradas, sin el propio manifiesto, ORDENADAS por nombre como en Go (os.ReadDir de
	// Go ordena; el manifiesto lo escribe el build en el orden que le sale). Se mide tambien /cd, que en
	// este POC no existe.
	if e2, err2 := os.ReadDir("/cd"); err2 != nil {
		println("VFS: ReadDir /cd ->", err2.Error(), "(declarado: el GD-ROM no esta en este POC)")
	} else {
		println("VFS: ReadDir /cd ->", strconv.Itoa(len(e2)), "entradas")
	}
	entradas, err := os.ReadDir("/rd")
	if err != nil {
		check("ReadDir /rd", "sin error", "error: "+err.Error())
	} else {
		// 4 entradas: los tres archivos de siempre + el subdirectorio "desorden", que es el fixture del
		// orden de ReadDir y el unico caso de DIRECTORIO del manifiesto.
		check("ReadDir /rd", "4", strconv.Itoa(len(entradas)))
		nombres := ""
		for _, e := range entradas {
			nombres += e.Name() + " "
		}
		println("VFS: ReadDir /rd ->", nombres, "| dirs:", dirs(entradas))
		if len(entradas) == 4 {
			check("ReadDir /rd nombres", "binario.bin desorden grande.txt hola.txt ", nombres)
			check("ReadDir /rd tamanos", "256 0 6000 37 ", tamanos(entradas))
			check("ReadDir /rd directorios", "false true false false ", dirs(entradas))
			check("ReadDir /rd Type()==fs.ModeDir", "false true false false ", modos(entradas))
		}
	}

	// 4c-bis. el fixture del ORDEN: /rd/desorden/.manifiesto esta escrito A PROPOSITO fuera de orden
	// (zulu, alfa, mike), asi que el orden alfabetico solo puede venir del ordenamiento de ReadDir. Es
	// lo que exige el mutante "sin ordenar" de scripts/console/mutar-os-consola.sh: sin este fixture el
	// mutante quedaba VERDE y el orden no estaba probado (el manifiesto de /rd sale alfabetico de pedo,
	// porque asi lo expande el glob del build).
	des, err := os.ReadDir("/rd/desorden")
	if err != nil {
		check("os.ReadDir /rd/desorden", "sin error", "error: "+err.Error())
	} else {
		nombresD := ""
		for _, e := range des {
			nombresD += e.Name() + " "
		}
		check("os.ReadDir /rd/desorden cantidad", "3", strconv.Itoa(len(des)))
		check("os.ReadDir /rd/desorden ordenado por nombre", "alfa.txt mike.txt zulu.txt ", nombresD)
		check("os.ReadDir /rd/desorden tamanos", "5 5 5 ", tamanos(des))
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

	// 4f. bufio en la consola: el archivo grande del romdisk partido en lineas con un Scanner. Es lo
	// que hace el motor para leer los textos de tablas (NewScanner 18 usos).
	fa, err := os.Open("/rd/grande.txt")
	if err != nil {
		check("bufio: Open grande.txt", "sin error", "error: "+err.Error())
	} else {
		esc := bufio.NewScanner(fa)
		lineas := 0
		primera, ultima, mala := "", "", ""
		for esc.Scan() {
			lineas++
			switch lineas {
			case 1:
				primera = esc.Text()
			case 200:
				ultima = esc.Text()
			}
		}
		if err := esc.Err(); err != nil {
			mala = err.Error()
		}
		fa.Close()
		check("bufio: lineas de grande.txt", "200", strconv.Itoa(lineas))
		check("bufio: primera linea", "linea 0000 del archivo grande", primera)
		check("bufio: ultima linea", "linea 0199 del archivo grande", ultima)
		check("bufio: Scanner sin error", "", mala)
	}

	// 4g. bufio.Reader por trozos: la primera linea con ReadString
	fb, err := os.Open("/rd/hola.txt")
	if err != nil {
		check("bufio: Open hola.txt", "sin error", "error: "+err.Error())
	} else {
		lector := bufio.NewReaderSize(fb, 16)
		txt, err := lector.ReadString('\n')
		fb.Close()
		if err != nil && txt == "" {
			check("bufio: ReadString de hola.txt", "el texto", "error: "+err.Error())
		} else {
			check("bufio: ReadString de hola.txt", "hola desde el romdisk de la Dreamcast", txt)
		}
	}

	// 4h. time en la consola: el reloj de KOS (RTC + timer) y la aritmetica, con los valores que da Go
	a := time.Now()
	check("time: Now no es cero", "true", strconv.FormatBool(!a.IsZero()))
	check("time: Now es un instante razonable (despues de 2020)", "true",
		strconv.FormatBool(a.Unix() > 1577836800))
	check("time: Unix(1e9)", "1000000000", strconv.FormatInt(time.Unix(1000000000, 0).Unix(), 10))
	t0 := time.Unix(1000, 0)
	check("time: Add(1s)", "1001", strconv.FormatInt(t0.Add(time.Second).Unix(), 10))
	check("time: Sub da 1s", "1s", time.Unix(1001, 0).Sub(t0).String())
	check("time: Before", "true", si(t0.Before(time.Unix(1001, 0))))
	check("time: Equal", "true", si(t0.Equal(time.Unix(1000, 0))))
	check("time: After", "true", si(time.Unix(1001, 0).After(t0)))
	// Duration.String: los mismos textos que da Go
	check("time: Duration 90m", "1h30m0s", (90 * time.Minute).String())
	check("time: Duration 1500ms", "1.5s", (1500 * time.Millisecond).String())
	check("time: Duration 300ms", "300ms", (300 * time.Millisecond).String())
	check("time: Duration 2.5ms", "2.5ms", (2500 * time.Microsecond).String())
	check("time: Duration 1500ns", "1.5us", (1500 * time.Nanosecond).String())
	check("time: Duration 0", "0s", (0 * time.Second).String())
	check("time: Duration negativo", "-1.5s", (-1500 * time.Millisecond).String())
	check("time: Duration 1h", "1h0m0s", time.Hour.String())
	// Sleep: que duerma DE VERDAD, medido con el reloj
	antes := time.Now()
	time.Sleep(50 * time.Millisecond)
	durmió := time.Since(antes)
	check("time: Sleep(50ms) durmio entre 50 y 500 ms", "true",
		si(durmió >= 50*time.Millisecond && durmió < 500*time.Millisecond))
	a1 := time.Now()
	time.Sleep(20 * time.Millisecond)
	check("time: Now avanza", "true", si(time.Now().After(a1)))

	// 4i. log en la consola: los mensajes por la consola serie y el prefijo
	log.SetPrefix("VFS-LOG: ")
	log.Print("un mensaje de log")
	log.Printf("con formato: %d %s", 42, "hola")
	log.Println("con Println")
	check("log: el prefijo quedo puesto", "VFS-LOG: ", log.Prefix())
	log.SetPrefix("")
	check("log: se saco el prefijo", "", log.Prefix())
	sumideroLog := &bufLog{}
	log.SetOutput(sumideroLog)
	log.SetPrefix("P: ")
	log.Print("hola")
	log.Printf("%d-%s", 7, "x")
	log.SetPrefix("")
	log.SetOutput(os.Stderr)
	check("log: el texto exacto", "P: hola\nP: 7-x\n", sumideroLog.texto())

	// 4k. ioutil sobre NUESTRO os (DC-002.8a). Hasta esta tarjeta ioutil importaba "io", "os" y "sort"
	// DESNUDOS: en la consola esos nombres resuelven a los nuestros (el build reescribe "d2std/" -> ""),
	// pero en el host eran los de la biblioteca de Go, asi que no habia ningun corpus que probara
	// ioutil sobre el os de la consola. Los imports son explicitos y el corpus vive aca.
	txtI, errI := ioutil.ReadFile("/rd/hola.txt")
	if errI != nil {
		check("ioutil.ReadFile hola.txt", "sin error", "error: "+errI.Error())
	} else {
		check("ioutil.ReadFile hola.txt", "hola desde el romdisk de la Dreamcast", string(txtI))
	}

	fr, errI := os.Open("/rd/grande.txt")
	if errI != nil {
		check("ioutil.ReadAll: Open grande.txt", "sin error", "error: "+errI.Error())
	} else {
		todoI, errL := ioutil.ReadAll(fr)
		fr.Close()
		if errL != nil {
			check("ioutil.ReadAll grande.txt", "sin error", "error: "+errL.Error())
		} else {
			check("ioutil.ReadAll grande.txt (6000 en varias vueltas)", "igual",
				igualdad(todoI, grandeEsperado()))
		}
	}

	// Lo mismo que hace el motor en d2mpq/mpq.go:204-205:
	//     var files []fs.FileInfo
	//     files, err = ioutil.ReadDir(mpqDir)
	// La asignacion de []os.FileInfo a []fs.FileInfo COMPILA porque los tipos son alias: era el error
	// que bloqueaba d2mpq ("cannot use type []os.FileInfo as type []fs.FileInfo").
	var infosI []fs.FileInfo
	infosI, errI = ioutil.ReadDir("/rd")
	if errI != nil {
		check("ioutil.ReadDir /rd", "sin error", "error: "+errI.Error())
	} else {
		nombresI := ""
		tamanosI := ""
		for _, fi := range infosI {
			nombresI += fi.Name() + " "
			tamanosI += strconv.FormatInt(fi.Size(), 10) + " "
		}
		check("ioutil.ReadDir /rd ([]os.FileInfo -> []fs.FileInfo)", "4", strconv.Itoa(len(infosI)))
		check("ioutil.ReadDir /rd nombres ordenados", "binario.bin desorden grande.txt hola.txt ", nombresI)
		check("ioutil.ReadDir /rd tamanos", "256 0 6000 37 ", tamanosI)
	}
	infosD, errI := ioutil.ReadDir("/rd/desorden") // el manifiesto esta fuera de orden a proposito
	if errI != nil {
		check("ioutil.ReadDir /rd/desorden", "sin error", "error: "+errI.Error())
	} else {
		nombresDI := ""
		tamanosDI := ""
		for _, fi := range infosD {
			nombresDI += fi.Name() + " "
			tamanosDI += strconv.FormatInt(fi.Size(), 10) + " "
		}
		check("ioutil.ReadDir /rd/desorden ordenado por nombre", "alfa.txt mike.txt zulu.txt ", nombresDI)
		check("ioutil.ReadDir /rd/desorden tamanos", "5 5 5 ", tamanosDI)
	}

	// NopCloser y Discard: los dos que ioutil saca de io.
	frN, errI := os.Open("/rd/hola.txt")
	if errI != nil {
		check("ioutil.NopCloser: Open", "sin error", "error: "+errI.Error())
	} else {
		rcI := ioutil.NopCloser(frN)
		trozoI := make([]byte, 4)
		nI, errN := rcI.Read(trozoI)
		rcI.Close()
		check("ioutil.NopCloser: 4 bytes", "hola", string(trozoI[:nI]))
		check("ioutil.NopCloser: sin error", "nil", errTxt(errN))
	}
	nD, errD := ioutil.Discard.Write([]byte("lo que sea"))
	check("ioutil.Discard.Write", "10 nil", strconv.Itoa(nD)+" "+errTxt(errD))

	// TempDir y TempFile: declarados (en la consola no hay donde escribir).
	if _, errT := ioutil.TempDir("", "x"); errT != nil {
		check("ioutil.TempDir declarado (devuelve error)", "si", "si")
	} else {
		check("ioutil.TempDir declarado (devuelve error)", "si", "no: devolvio nil")
	}
	if _, errT := ioutil.TempFile("", "x"); errT != nil {
		check("ioutil.TempFile declarado (devuelve error)", "si", "si")
	} else {
		check("ioutil.TempFile declarado (devuelve error)", "si", "no: devolvio nil")
	}

	// 4l. las identidades de tipos (os.X ES fs.X): lo dice el COMPILADOR, no el runtime. Si os.FileInfo,
	// os.DirEntry o os.FileMode dejan de ser alias de los de io/fs, este POC no compila, que es el
	// sintoma del motor (d2mpq/mpq.go:204). El mutante que lo exige: scripts/console/mutar-tipos-os.sh.
	identidades()

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

	// 4j. log.Fatal: tiene que imprimir y NO volver (en la consola os.Exit se queda en un lazo). Se prueba
	// al final, porque despues de esto no corre nada mas.
	log.Print("VFS: antes del Fatal")
	log.Fatal("VFS: Fatal imprime y no vuelve")

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

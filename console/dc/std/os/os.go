// Paquete os: lo que el motor usa de verdad (114 archivos), contra el VFS de KOS.
//
// QUE HAY Y QUE NO, dicho de una vez (en la consola un "sistema operativo" es otra cosa):
//   - archivos: si. os.Open, os.ReadFile, os.WriteFile, os.Stat y las funciones de File (Read, Write,
//     Close, Name). Abajo hay el VFS de KOS: el romdisk (los archivos que viajan adentro del
//     ELF/GDI, montado en /rd) o el GD-ROM por ISO9660. Lo que usa el motor para leer los datos del
//     juego (MPQ, DC6, tablas) entra por aca.
//   - Stdout/Stderr: si, por la consola serie del emulador (dbgio), que es la unica salida de texto.
//   - Args: si, pero vacio: en la consola no hay argumentos de linea de comandos. Se declara con el
//     nombre del programa para que el motor que los mire no se caiga.
//   - Getenv/Environ: si, vacio. No hay entorno. Lo que el motor lea por ahi (paths de CASCAL) va a
//     venir vacio y eso hay que tenerlo presente, no fingir que hay variables.
//   - Exit: si. No hay procesos que matar: se queda en un lazo (el runtime no puede volver).
//   - permisos, directorios por defecto, Mkdir/Remove, CreateTemp, os.FindProcess: NO. Si el motor los
//     pide, se ve en el corpus y se declara.
//
// Se probo leyendo del romdisk en Flycast (console/dc/poc/vfs): el archivo viaja dentro del ELF.
package os

import (
	"d2std/errors"
	"d2std/io"
	"d2std/io/fs"
	"d2std/time"
	"unsafe"
)

// FileMode y las constantes: los de io/fs (en la stdlib de verdad os solo les pone el nombre corto).
// Tiene que ser EL MISMO tipo: si no, un FileMode de aca no sirve donde se pide el de io/fs, y el motor
// pide los dos (d2mpq/mpq.go).
type FileMode = fs.FileMode

const (
	ModeDir  = fs.ModeDir
	ModePerm = fs.ModePerm
)

// ErrNotExist es el error de "no esta" (el motor lo compara con errors.Is).
var ErrNotExist = errors.New("file does not exist")

// ErrSoloLectura: escribir en el romdisk (que es de solo lectura).
var ErrSoloLectura = errors.New("read-only file system")

// File es un archivo abierto sobre el VFS de KOS.
type File struct {
	fd       int
	nombre   string
	consola  bool // Stdout/Stderr: escriben por la consola serie
	cerrado  bool
}

//extern d2c_abrir_leer
func d2c_abrir_leer(ruta string) int

//extern d2c_abrir_crear
func d2c_abrir_crear(ruta string) int

// La frontera con C va con uintptr y no con []byte: un slice de Go pasado a una funcion //extern
// tiene un ABI ambiguo (puntero, len y cap como tres argumentos), y un uintptr es una palabra y no
// deja dudas. El C lo vuelve a castear a puntero.

//extern d2c_leer
func d2c_leer(fd int, buf uintptr, n int) int

//extern d2c_escribir_archivo
func d2c_escribir_archivo(fd int, buf uintptr, n int) int

//extern d2c_cerrar
func d2c_cerrar(fd int) int

//extern d2c_tam
func d2c_tam(fd int) int

//extern d2c_buscar
func d2c_buscar(fd int, offset int, whence int) int

//extern d2c_estoy
func d2c_estoy(fd int) int

//extern d2c_hay
func d2c_hay(ruta string) int

// DC-002.17: el sistema de archivos ESCRIBIBLE (en RAM, ver console/dc/std/csrc/os_dc.c). Lo que el
// romdisk no puede hacer -- crear directorios, crear y escribir archivos -- lo hace el overlay de la RAM
// con un presupuesto fijo en .bss. Las firmas son las mismas que usa el resto del paquete: rutas como
// string (gccgo las pasa como char*), y los contadores del resumen como uintptr (una palabra, sin ABI
// ambiguo).
//
//extern d2c_mkdir
func d2c_mkdir(ruta string) int

//extern d2c_borrar_archivo
func d2c_borrar_archivo(ruta string) int

//extern d2c_renombrar
func d2c_renombrar(viejo string, nuevo string) int

//extern d2c_truncar
func d2c_truncar(ruta string, tam int) int

//extern d2c_ramfs_resumen
func d2c_ramfs_resumen(archivos uintptr, bytes uintptr, dirs uintptr) int

//extern d2c_ramfs_tam
func d2c_ramfs_tam() int

// DC-002.19: el listado del overlay de la RAM (os.ReadDir de un directorio creado en la consola). El C
// escribe el MISMO formato del manifiesto del build y devuelve los bytes escritos, o -1 si no entran.
// El buffer lo pone Go (local, de tamano fijo): en este arranque el monton esta justo.
//
//extern d2c_ramfs_manifiesto
func d2c_ramfs_manifiesto(buf uintptr, max int, ruta string) int

//extern d2c_escribir_consola
func d2c_escribir_consola(buf uintptr, n int)

//extern d2c_salir
func d2c_salir(codigo int)

// Stdout, Stderr y Stdin: en la consola la salida de texto es la consola serie.
var (
	Stdin  = &File{fd: -1, nombre: "/dev/stdin"}
	Stdout = &File{fd: -1, nombre: "/dev/stdout", consola: true}
	Stderr = &File{fd: -1, nombre: "/dev/stderr", consola: true}
)

// Args no existe en la consola: el motor lo mira y tiene que encontrar algo coherente.
var Args = []string{"/rd/prog.elf"}

// Getenv: no hay entorno en la consola. Devuelve vacio (y el motor tiene que poder vivir con eso).
func Getenv(clave string) string { return "" }

// Environ: vacio, por lo mismo.
func Environ() []string { return nil }

// Open abre un archivo para leer.
func Open(nombre string) (*File, error) {
	fd := d2c_abrir_leer(nombre)
	if fd < 0 {
		return nil, ErrNotExist
	}
	return &File{fd: fd, nombre: nombre}, nil
}

// OpenFile existe por el motor: los modos finos no se distinguen en la consola (si pide escritura, se
// pide al VFS y el romdisk la rechaza).
func OpenFile(nombre string, bandera int, perm FileMode) (*File, error) {
	if bandera&2 != 0 || bandera&0x200 != 0 { // O_WRONLY en Go es 1, O_RDWR 2; O_CREATE 0x40
		fd := d2c_abrir_crear(nombre)
		if fd < 0 {
			return nil, ErrSoloLectura
		}
		return &File{fd: fd, nombre: nombre}, nil
	}
	return Open(nombre)
}

// Name devuelve el nombre con el que se abrio.
func (f *File) Name() string { return f.nombre }

// Read lee hasta len(p) bytes. Devuelve 0 y un error al final (io.EOF lo pone el llamador: aca no se
// importa io para no armar un ciclo, y el paquete io de la consola es el de la stdlib minima).
func (f *File) Read(p []byte) (int, error) {
	if f.cerrado {
		return 0, errors.New("file already closed")
	}
	if len(p) == 0 {
		return 0, nil
	}
	n := d2c_leer(f.fd, uintptr(unsafe.Pointer(&p[0])), len(p))
	if n < 0 {
		return 0, errors.New("read error")
	}
	if n == 0 {
		return 0, io.EOF
	}
	return n, nil
}

// Write escribe en el archivo, o por la consola serie si es Stdout/Stderr.
func (f *File) Write(p []byte) (int, error) {
	if f.consola {
		if len(p) > 0 {
			d2c_escribir_consola(uintptr(unsafe.Pointer(&p[0])), len(p))
		}
		return len(p), nil
	}
	if f.cerrado {
		return 0, errors.New("file already closed")
	}
	if len(p) == 0 {
		return 0, nil
	}
	n := d2c_escribir_archivo(f.fd, uintptr(unsafe.Pointer(&p[0])), len(p))
	if n < 0 {
		return 0, ErrSoloLectura
	}
	// DC-002.17: la marca del overlay de la RAM (se ve en el log de la consola y es la evidencia de que
	// el motor guardo su archivo). Argumentos sueltos, sin concatenar: ver el comentario de noSePuede.
	if n > 0 {
		println("OS-RAMFS", "escribe", "bytes=", n)
	}
	return n, nil
}

// WriteString escribe un texto (fmt la usa para os.Stdout).
func (f *File) WriteString(s string) (int, error) {
	return f.Write([]byte(s))
}

// Close cierra el archivo.
func (f *File) Close() error {
	if f.consola || f.cerrado {
		return nil
	}
	f.cerrado = true
	if d2c_cerrar(f.fd) < 0 {
		return errors.New("close error")
	}
	return nil
}

// Size devuelve el tamano en bytes (fs_total de KOS).
func (f *File) Size() int64 { return int64(d2c_tam(f.fd)) }

// ReadFile lee el archivo entero. Es lo que mas usa el motor.
func ReadFile(nombre string) ([]byte, error) {
	f, err := Open(nombre)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	tam := f.Size()
	if tam < 0 {
		tam = 0
	}
	datos := make([]byte, 0, int(tam)+1)
	buf := make([]byte, 4096)
	for {
		n, err := f.Read(buf)
		if n > 0 {
			datos = append(datos, buf[:n]...)
		}
		if err != nil {
			break
		}
	}
	if tam > 0 && int64(len(datos)) != tam {
		return datos, errors.New("short read")
	}
	return datos, nil
}

// WriteFile escribe un archivo (en el romdisk va a fallar: es de solo lectura).
func WriteFile(nombre string, datos []byte, perm FileMode) error {
	f, err := OpenFile(nombre, 0x200|0x40|1, perm) // O_WRONLY|O_CREATE|O_TRUNC
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(datos)
	return err
}

// Existe dice si la ruta esta (el motor lo usa para decidir si hay un archivo opcional).
func Existe(nombre string) bool { return d2c_hay(nombre) == 1 }

// Exit: no hay procesos en la consola. Se queda en un lazo (nunca vuelve).
func Exit(codigo int) { d2c_salir(codigo) }

//extern d2c_stat_tam
func d2c_stat_tam(ruta string) int

//extern d2c_stat_es_dir
func d2c_stat_es_dir(ruta string) int

// FileInfo es el de io/fs (alias de verdad, no un tipo parecido: asi el []os.FileInfo de aca y el
// []fs.FileInfo que pide el motor son el MISMO tipo).
type FileInfo = fs.FileInfo

type infoArchivo struct {
	nombre string
	tam    int64
	dir    bool
}

func (i infoArchivo) Name() string     { return i.nombre }
func (i infoArchivo) Size() int64      { return i.tam }
func (i infoArchivo) IsDir() bool      { return i.dir }
func (i infoArchivo) Mode() FileMode   { return ModePerm }
func (i infoArchivo) ModTime() time.Time { return time.Time{} }
func (i infoArchivo) Sys() interface{}   { return nil }

// Stat mira un archivo o directorio sin abrirlo.
func Stat(nombre string) (FileInfo, error) {
	tam := d2c_stat_tam(nombre)
	if tam < 0 {
		return nil, ErrNotExist
	}
	return infoArchivo{nombre: nombre, tam: int64(tam), dir: d2c_stat_es_dir(nombre) == 1}, nil
}

// Stat del archivo abierto.
func (f *File) Stat() (FileInfo, error) {
	if f.consola {
		return infoArchivo{nombre: f.nombre}, nil
	}
	return Stat(f.nombre)
}

// IsNotExist: el motor lo usa para decidir si un archivo opcional falta (10 lugares).
func IsNotExist(err error) bool {
	return err == ErrNotExist || err == ErrSoloLectura
}

// IsExist: lo inverso.
func IsExist(err error) bool { return !IsNotExist(err) }

// Las funciones de escritura del FS: en la consola el romdisk es de SOLO LECTURA y no hay ningun
// sistema de archivos con permiso de escritura enganchado (la VMU y el GD-ROM no estan). No se
// finge: se devuelve el error y queda declarado. El motor las usa para los saves (9 + 7 + 5 + 4).
var ErrNoSePuede = errors.New("operation not supported on read-only filesystem")

// MEDIDOR DE DC-002.17: dice QUE operacion de escritura pide el motor y con QUE camino. Con esto se
// midio que el arranque del motor se frena en `Run()` por una de estas llamadas (no por el regexp ni por
// el mapa nil, que ya quedaron atras). Se queda en el paquete a proposito: imprime SOLO cuando alguien
// pide escribir, y en la consola eso hoy siempre termina en error, asi que no ensucia nada y hace que la
// proxima medicion sea correr y leer. Es el mismo camino que las marcas MOTOR-ARRANQUE del POC.
//
// OJO MEDIDO: el println va con ARGUMENTOS SUELTOS y no con `"a" + que + "b"`. La version que concatenaba
// NO IMPRIMIA NADA: concatenar arma una cadena nueva en el monton, y en este arranque la consola ya esta
// sin memoria (el log dice `Out of memory. Requested sbrk_base ... diff 1048576`), asi que la cadena no se
// puede armar. Con argumentos sueltos no hay nada que reservar y el medidor sale. (Se descubrio midiendo:
// el medidor estaba en el binario y no se veia, y el codigo que llama SI estaba -- era el println.)
func noSePuede(que, donde string) error {
	println("OS-ESCRITURA", que, "path=", donde)
	return ErrNoSePuede
}

// DC-002.17: LAS FUNCIONES DE ESCRITURA YA NO SON STUBS. El romdisk de KOS es de solo lectura, pero el
// motor ESCRIBE su configuracion en /rd/OpenDiablo2/config.json y sin eso `Run()` no arranca (medido en
// evidencia/dc-002.17-escritura-config.txt). Lo que se crea o se escribe va a un overlay EN RAM con
// presupuesto fijo (console/dc/std/csrc/os_dc.c), que tapa al romdisk con el mismo nombre; lo que no
// esta en ese overlay se sigue leyendo de KOS. El limite queda declarado: 8 archivos de 2.048 bytes y 8
// directorios; si no entra, se devuelve el error de siempre (ErrNoSePuede) y no se miente.
//
// Hay marcas "OS-RAMFS" a proposito: son la evidencia medible de que el motor pudo guardar su config. Se
// imprimen con ARGUMENTOS SUELTOS y no concatenando (concatenar arma una cadena en el monton, y en este
// arranque el monton esta justo: ver el comentario de noSePuede).
func MkdirAll(nombre string, perm FileMode) error {
	if d2c_mkdir(nombre) == 0 {
		println("OS-RAMFS", "mkdir-ok", "path=", nombre)
		return nil
	}
	return noSePuede("MkdirAll", nombre)
}

func Mkdir(nombre string, perm FileMode) error {
	if d2c_mkdir(nombre) == 0 {
		println("OS-RAMFS", "mkdir-ok", "path=", nombre)
		return nil
	}
	return noSePuede("Mkdir", nombre)
}

func Remove(nombre string) error {
	if d2c_borrar_archivo(nombre) == 0 {
		println("OS-RAMFS", "remove-ok", "path=", nombre)
		return nil
	}
	return noSePuede("Remove", nombre)
}

func Rename(viejo, nuevo string) error {
	if d2c_renombrar(viejo, nuevo) == 0 {
		println("OS-RAMFS", "rename-ok", "path=", viejo)
		return nil
	}
	return noSePuede("Rename", viejo)
}

func Create(nombre string) (*File, error) {
	fd := d2c_abrir_crear(nombre)
	if fd < 0 {
		return nil, noSePuede("Create", nombre)
	}
	println("OS-RAMFS", "create-ok", "path=", nombre)
	return &File{fd: fd, nombre: nombre}, nil
}

// RamfsResumen: cuantos archivos y bytes tiene el overlay de la RAM y cuantos directorios (para la
// evidencia del arranque: dice que el motor guardo su config de verdad y no solo que no hubo error).
func RamfsResumen() (archivos int, bytes int, dirs int) {
	var a, b, d int
	d2c_ramfs_resumen(uintptr(unsafe.Pointer(&a)), uintptr(unsafe.Pointer(&b)), uintptr(unsafe.Pointer(&d)))
	return a, b, d
}

// RamfsTam: cuantos bytes de .bss ocupa el overlay (cero del monton).
func RamfsTam() int { return d2c_ramfs_tam() }


// UserHomeDir: el motor lo usa para armar paths de datos. En la consola el "home" es el romdisk.
func UserHomeDir() (string, error) { return "/rd", nil }

// Signal existe porque el motor lo nombra: no hay senales en la consola, es un tipo vacio.
type Signal interface {
	String() string
	Signal()
}

// DirEntry es el de io/fs (alias). El tipo concreto de aca abajo (dirEntryOS) es el que implementa la
// interfaz: el que devuelve ReadDir.
type DirEntry = fs.DirEntry

type dirEntryOS struct {
	nombre string
	tam    int64
	tipo   int
}

// Size: el tamano que trae el manifiesto.
func (d dirEntryOS) Size() int64 { return d.tam }

// Info: la misma info que Stat, sin abrir nada.
func (d dirEntryOS) Info() (FileInfo, error) {
	return infoArchivo{nombre: d.nombre, tam: d.tam, dir: d.tipo == 4}, nil
}

// Name es el nombre de la entrada.
func (d dirEntryOS) Name() string { return d.nombre }

// IsDir: 4 es DT_DIR (POSIX). En el romdisk da false aunque el archivo sea un directorio: el tipo
// viene DT_UNKNOWN y se dice aca en vez de mentir.
func (d dirEntryOS) IsDir() bool { return d.tipo == 4 }

// Type: 4 = DT_DIR, 8 = DT_REG (POSIX). En el romdisk viene DT_UNKNOWN (0), y eso queda declarado.
func (d dirEntryOS) Type() FileMode {
	if d.tipo == 4 {
		return ModeDir
	}
	return 0
}

// NO hay TipoCrudo: el valor de KOS sin interpretar se fue con el alias. os.DirEntry ES fs.DirEntry, y
// esa interfaz no declara TipoCrudo, asi que un metodo de mas en el tipo concreto (que no se puede
// nombrar desde afuera) no se podia pedir de ninguna manera: era codigo que no se alcanzaba. Lo que
// queda es Type() e IsDir(), que son los de Go y son los que el corpus mide. Medido en DC-002.8a: el
// POC del VFS pedia e.TipoCrudo() y e.Size() (del interfaces viejo, mas ancho) y NO COMPILABA desde el
// alias (commit 95e0ef74, DC-002.8): el corpus de os en la consola quedo muerto sin que nadie lo viera.

// ReadDir lista un directorio. EN LA CONSOLA SALE DE DOS LADOS, y hay que juntarlos:
//
//  1. EL ROMDISK, de un ".manifiesto" horneado por el build. Medido en Flycast (poc/sonda-i y el POC del
//     VFS): el FS ROM de KOS NO deja abrir un directorio. fs_open("/rd") da INVALID con las tres formas de
//     la ruta ("/rd", "/rd/", "/") y tambien con (kos/fs.h) fs_readdir sobre un file_t, mientras que los
//     ARCHIVOS de /rd se abren y se leen sin problema. Como el romdisk lo arma este mismo build, el
//     listado se hornea con el: build-dc.sh escribe un ".manifiesto" (nombre, tamano y tipo por linea,
//     separados por TAB) DENTRO del romdisk. Se generan juntos, asi que no pueden desincronizarse.
//
//  2. EL OVERLAY DE LA RAM (lo que el programa creo o escribio, DC-002.17), que TAPA al romdisk con el
//     mismo nombre. Su listado lo da el C (d2c_ramfs_manifiesto) en el mismo formato, y SIN el un
//     directorio del overlay se veia VACIO: medido en DC-002.19 con el motor de verdad,
//     `Validate(dir).OK() = false archivos que ve = 0` sobre una carpeta con siete archivos que os.Stat
//     SI encontraba (y el camino de la seleccion del motor pregunta justamente por el contenido).
//
// El ".manifiesto" no se lista a si mismo (ni el del romdisk ni el del overlay). Para /cd (el GD-ROM por
// ISO9660) esto no aplica y hay que resolverlo cuando se llegue ahi.
func ReadDir(nombre string) ([]DirEntry, error) {
	base := nombre
	for len(base) > 0 && base[len(base)-1] == '/' {
		base = base[:len(base)-1]
	}
	if base == "" {
		base = "/"
	}

	// (1) el overlay de la RAM: lo que se creo o se escribio en la consola (DC-002.17). El buffer es
	// LOCAL y de tamano fijo: una lista de directorio no puede pedirle memoria al monton en este
	// arranque. 1024 bytes alcanzan para el presupuesto entero del overlay (8 archivos + 8 directorios
	// con nombres de hasta 95 caracteres).
	var bufRAM [1024]byte

	deRAM := []DirEntry(nil)
	if n := d2c_ramfs_manifiesto(uintptr(unsafe.Pointer(&bufRAM[0])), len(bufRAM), base); n >= 0 {
		deRAM = entradasDelManifiesto(bufRAM[:n])
	} else {
		// El instrumento: el C devuelve un NEGATIVO por cada salida (ver os_dc.c). Solo se imprime cuando
		// el listado del overlay no se pudo armar. OJO: se imprime con ARGUMENTOS SUELTOS, nunca
		// concatenando: en el arranque del motor el monton esta justo y una cadena armada no se ve
		// (la leccion de DC-002.17).
		println("OS-RAMFS manifiesto-de-la-ram: no se pudo armar para", base, "codigo", n)
	}

	// (2) el romdisk: el ".manifiesto" que hornea el build
	datos, err := ReadFile(base + "/.manifiesto")

	if err != nil && len(deRAM) == 0 && d2c_stat_es_dir(base) != 1 {
		// ni manifiesto, ni entradas en la RAM, ni el directorio: para Go esto no esta
		return nil, ErrNotExist
	}

	salida := entradasDelManifiesto(datos)

	// El overlay TAPA al romdisk con el mismo nombre (la misma regla que os.Open y d2c_hay).
	for _, nueva := range deRAM {
		puesta := false

		for i, vieja := range salida {
			if vieja.Name() == nueva.Name() {
				salida[i] = nueva
				puesta = true

				break
			}
		}

		if !puesta {
			salida = append(salida, nueva)
		}
	}

	// Go ordena las entradas de ReadDir POR NOMBRE (os.ReadDir: "sorted by filename"); el manifiesto,
	// en cambio, sale en el orden que le da el build. Medido en DC-002.8a con /rd/desorden/.manifiesto
	// escrito a proposito como zulu-alfa-mike: nuestro os.ReadDir devolvia "zulu.txt alfa.txt mike.txt "
	// donde el de Go devuelve "alfa.txt mike.txt zulu.txt ". Se ordena aca, a mano y no con sort: os es
	// el paquete mas bajo de todos y no importa nada que no sea imprescindible (la misma razon por la que
	// existe partirTab). Lo exige el corpus (poc/vfs, seccion 4c-bis) y su mutante.
	for i := 1; i < len(salida); i++ {
		for j := i; j > 0 && salida[j].Name() < salida[j-1].Name(); j-- {
			salida[j], salida[j-1] = salida[j-1], salida[j]
		}
	}

	return salida, nil
}

// entradasDelManifiesto lee el formato del manifiesto -- "nombre TAB bytes TAB archivo|directorio", una
// entrada por linea -- y devuelve las entradas. Es el MISMO formato para el romdisk (lo hornea el build)
// y para el overlay de la RAM (lo arma el C), asi que los dos se leen con el mismo codigo: si el de la
// RAM se saliera del formato, el corpus lo veria como una entrada menos.
func entradasDelManifiesto(datos []byte) []DirEntry {
	var salida []DirEntry

	linea := ""
	for i := 0; i <= len(datos); i++ {
		var c byte
		if i == len(datos) {
			c = '\n'
		} else {
			c = datos[i]
		}
		if c != '\n' {
			linea = linea + string(c)
			continue
		}
		if linea != "" {
			campos := partirTab(linea)
			if len(campos) >= 3 {
				tam := 0
				for j := 0; j < len(campos[1]); j++ {
					if campos[1][j] >= '0' && campos[1][j] <= '9' {
						tam = tam*10 + int(campos[1][j]-'0')
					}
				}
				tipo := 8 // DT_REG
				if campos[2] == "directorio" {
					tipo = 4 // DT_DIR
				}
				salida = append(salida, dirEntryOS{nombre: campos[0], tam: int64(tam), tipo: tipo})
			}
		}
		linea = ""
	}

	return salida
}

// partirTab parte por TAB (el manifiesto lo usa como separador; no se usa strings para no meter una
// dependencia en el paquete mas bajo de todos).
func partirTab(s string) []string {
	var partes []string
	actual := ""
	for i := 0; i < len(s); i++ {
		if s[i] == '	' {
			partes = append(partes, actual)
			actual = ""
			continue
		}
		actual = actual + string(s[i])
	}
	partes = append(partes, actual)
	return partes
}

// ---------------------------------------------------------------- Seek y ReadAt
//
// Los pide d2mpq para leer adentro del MPQ (se posiciona en un bloque y lee un pedazo). El sistema de
// archivos de KOS sabe hacer las dos cosas (fs_seek), asi que van contra d2c_buscar del C de la stdlib
// (csrc/os_dc.c), igual que el resto de os.
//
// Las constantes son las de io, con los MISMOS valores (0, 1, 2).

const (
	SeekStart   = 0
	SeekCurrent = 1
	SeekEnd     = 2
)

// Seek mueve la posicion de lectura.
func (f *File) Seek(offset int64, whence int) (int64, error) {
	if f == nil || f.fd < 0 {
		return 0, ErrNotExist
	}
	r := d2c_buscar(f.fd, int(offset), whence)
	if r < 0 {
		return 0, ErrNotExist
	}
	return int64(r), nil
}

// ReadAt lee en una posicion concreta sin dejar movida la posicion actual (guarda y restaura).
func (f *File) ReadAt(p []byte, off int64) (n int, err error) {
	if f == nil || f.fd < 0 {
		return 0, ErrNotExist
	}
	actual := d2c_estoy(f.fd)
	if actual < 0 {
		return 0, ErrNotExist
	}
	if _, err := f.Seek(off, SeekStart); err != nil {
		return 0, err
	}
	n, err = f.Read(p)
	if _, err2 := f.Seek(int64(actual), SeekStart); err2 != nil && err == nil {
		err = err2
	}
	return n, err
}

// ---------------------------------------------------------------- las banderas de OpenFile
//
// Las pide d2hero (d2s_lock_unix.go arma la bandera de abrir el archivo de guardado). Los valores son los
// de Go, que a su vez son los de linux, porque el motor los puede estar combinando y comparando.
//
// En la consola el romdisk es de SOLO LECTURA: abrir con banderas de escritura compila y no va a poder
// (devuelve el error de solo lectura, que ya esta declarado en este paquete).
const (
	O_RDONLY int = 0x0
	O_WRONLY int = 0x1
	O_RDWR   int = 0x2
	O_APPEND int = 0x400
	O_CREATE int = 0x40
	O_EXCL   int = 0x80
	O_SYNC   int = 0x101000
	O_TRUNC  int = 0x200
)

// ---------------------------------------------------------------- CreateTemp y Chmod
//
// Los pide d2hero al guardar (d2s_save.go: escribe a un temporal y despues lo mueve, que es la forma
// correcta de guardar sin dejar el archivo a medias). En la consola no hay disco donde escribir, asi que
// CreateTemp falla con el mismo error de solo lectura que el resto de las escrituras de este paquete: no
// se finge un archivo que no existe.
//
// Chmod no aplica (no hay permisos que cambiar en un romdisk): dice que si, que es lo que corresponde
// cuando no hay nada que hacer.

// CreateTemp: no hay donde crear temporales (el romdisk es de solo lectura).
func CreateTemp(dir, pattern string) (*File, error) { return nil, noSePuede("CreateTemp", dir+"/"+pattern) }

// Chmod: en la consola no hay permisos que cambiar.
func Chmod(nombre string, modo FileMode) error { return nil }

// Chown: lo mismo.
func Chown(nombre string, uid, gid int) error { return nil }

// Truncate: no se puede (solo lectura).
// Truncate: corta el archivo. Ahora puede: los archivos del overlay de la RAM se cortan (o se estiran
// con ceros) y los del romdisk siguen devolviendo el error declarado.
func Truncate(nombre string, tam int64) error {
	if d2c_truncar(nombre, int(tam)) == 0 {
		return nil
	}
	return noSePuede("Truncate", nombre)
}

// Fd: el descriptor del archivo. En la consola no hay descriptores: el VFS lee del romdisk por dentro, y
// no hay nada del sistema operativo que necesite uno. Devuelve 0 para que el codigo que lo pase a
// syscall.Flock (que aca no bloquea nada) siga andando. Si alguna vez algo lo usara para leer de verdad,
// tendria que fallar -- y no lo hace nadie: medido, el unico uso es el lock de d2hero.
func (f *File) Fd() uintptr { return 0 }

// Sync: vacia lo escrito al disco. En la consola no se escribe (el romdisk es de solo lectura), asi que no
// hay nada que vaciar. Existe para que el guardado compile: lo pide d2hero al cerrar el archivo temporal.
func (f *File) Sync() error { return nil }

// Truncate: acortar el archivo. No se puede (solo lectura).
func (f *File) Truncate(tam int64) error {
	if f.consola || f.cerrado {
		return errors.New("file already closed")
	}
	if d2c_truncar(f.nombre, int(tam)) == 0 {
		return nil
	}
	return noSePuede("File.Truncate", f.nombre)
}

// UserConfigDir: donde vive la configuracion del usuario. En la consola es el romdisk (lo unico que se
// puede leer), asi que apunta ahi en vez de a un home que no existe.
func UserConfigDir() (string, error) { return "/rd", nil }

// UserCacheDir: lo mismo (no hay cache: el romdisk no se escribe).
func UserCacheDir() (string, error) { return "/rd", nil }

// Las senales: en la consola no hay senales de sistema operativo (no hay procesos), pero el motor las
// NOMBRA al armar el apagado ordenado (d2app/app.go:156: signal.Notify(c, os.Interrupt, syscall.SIGTERM)).
// Existen para que el codigo compile y para que la tabla de senales de os/signal tenga algo que nombrar.
var (
	Interrupt Signal = senal{nombre: "interrupt"}
	Kill      Signal = senal{nombre: "killed"}
	Terminate Signal = senal{nombre: "terminated"}
)

// senal: una senal con nombre (no hay numeros de senal en la consola).
type senal struct {
	nombre string
}

func (s senal) String() string { return s.nombre }

// Signal: la senal.
func (s senal) Signal() {}

// SeñalDeNombre: busca una senal por su nombre (lo usa os/signal).
func SenalDeNombre(nombre string) Signal {
	switch nombre {
	case "interrupt":
		return Interrupt
	case "killed":
		return Kill
	case "terminated":
		return Terminate
	}
	return nil
}

// TempDir: el directorio de temporales. En Go es "/tmp" (o lo que diga TMPDIR); aca es el romdisk, que es
// el unico camino que EXISTE en la consola. Esta declarado a proposito: el romdisk es de SOLO LECTURA, o
// sea que cualquiera que escriba ahi va a fallar -- y eso es correcto, no un descuido. Lo pide
// d2app/autoclass.go:78 para sacar los archivos de clasificacion.
func TempDir() string { return "/rd" }

// Hostname: el nombre de la maquina. En la consola no hay nombre de maquina configurado (no hay sistema de
// archivos donde tenga uno, ni red por la que lo pregunten), asi que se devuelve el de la consola misma.
//
// POR QUE NO SE DEVUELVE UN ERROR: lo pide d2gamescreen/main_menu.go:690 para armar la direccion IP local
// (pregunta el nombre y despues busca sus direcciones). Si devolviera error, el nombre quedaria vacio y la
// busqueda seria la de "" -- el resultado es el mismo (no hay ninguna direccion, porque no hay red), pero
// devolver un nombre conocido deja el codigo del motor en su camino normal.
func Hostname() (string, error) { return "dreamcast", nil }

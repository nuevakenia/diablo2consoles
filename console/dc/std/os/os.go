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
	"errors"
	"io"
	"unsafe"
)

// FileMode es el tipo de Go (los permisos no se usan en la consola, pero el tipo tiene que existir
// porque el motor lo nombra).
type FileMode uint32

// Los modos que el motor nombra.
const (
	ModeDir FileMode = 1 << (32 - 1 - iota)
	ModePerm FileMode = 0777
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

//extern d2c_hay
func d2c_hay(ruta string) int

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

// FileInfo es lo que devuelve Stat. NO tiene ModTime: eso pediria el paquete time, que en la consola
// es el reloj de KOS y todavia no esta enganchado (si el motor lo pide, se ve al compilar y se
// declara, en vez de inventar una fecha).
type FileInfo interface {
	Name() string
	Size() int64
	IsDir() bool
	Mode() FileMode
}

type infoArchivo struct {
	nombre string
	tam    int64
	dir    bool
}

func (i infoArchivo) Name() string  { return i.nombre }
func (i infoArchivo) Size() int64   { return i.tam }
func (i infoArchivo) IsDir() bool   { return i.dir }
func (i infoArchivo) Mode() FileMode { return ModePerm }

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

func MkdirAll(nombre string, perm FileMode) error { return ErrNoSePuede }
func Mkdir(nombre string, perm FileMode) error    { return ErrNoSePuede }
func Remove(nombre string) error                  { return ErrNoSePuede }
func Rename(viejo, nuevo string) error            { return ErrNoSePuede }
func Create(nombre string) (*File, error)         { return nil, ErrNoSePuede }

// UserHomeDir: el motor lo usa para armar paths de datos. En la consola el "home" es el romdisk.
func UserHomeDir() (string, error) { return "/rd", nil }

// Signal existe porque el motor lo nombra: no hay senales en la consola, es un tipo vacio.
type Signal interface {
	String() string
	Signal()
}

// DirEntry es una entrada de directorio (lo que devuelve ReadDir).
//
// El tipo viene de KOS crudo: es la convencion POSIX de dirent.h (DT_DIR = 4, DT_REG = 8), la misma
// que documenta libgodc/kos/fs.go. En el romdisk medido en Flycast las entradas dan DT_UNKNOWN (0):
// la imagen no guarda el tipo, asi que IsDir() no puede saberlo y eso queda declarado.
type DirEntry struct {
	nombre string
	tam    int64
	tipo   int
}

// Size: el tamano que trae el manifiesto.
func (d DirEntry) Size() int64 { return d.tam }

// Info: la misma info que Stat, sin abrir nada.
func (d DirEntry) Info() (FileInfo, error) {
	return infoArchivo{nombre: d.nombre, tam: d.tam, dir: d.tipo == 4}, nil
}

// Name es el nombre de la entrada.
func (d DirEntry) Name() string { return d.nombre }

// IsDir: 4 es DT_DIR (POSIX). En el romdisk da false aunque el archivo sea un directorio: el tipo
// viene DT_UNKNOWN y se dice aca en vez de mentir.
func (d DirEntry) IsDir() bool { return d.tipo == 4 }

// TipoCrudo es el valor de KOS sin interpretar (para diagnostico y para el corpus).
func (d DirEntry) TipoCrudo() int { return d.tipo }

// ReadDir lista un directorio. EN LA CONSOLA SALE DE UN MANIFIESTO, no del sistema de archivos:
//
// Medido en Flycast (poc/sonda-i y el POC del VFS): el FS ROM de KOS NO deja abrir un directorio.
// fs_open("/rd") da INVALID con las tres formas de la ruta ("/rd", "/rd/", "/") y tambien con
// (kos/fs.h) fs_readdir sobre un file_t, mientras que los ARCHIVOS de /rd se abren y se leen sin
// problema. Como el romdisk lo arma este mismo build, el listado se hornea con el: build-dc.sh escribe
// un ".manifiesto" (nombre, tamano y tipo por linea, separados por TAB) DENTRO del romdisk. Se generan
// juntos, asi que no pueden desincronizarse.
//
// El ".manifiesto" no se lista a si mismo. Para /cd (el GD-ROM por ISO9660) esto no aplica y hay que
// resolverlo cuando se llegue ahi.
func ReadDir(nombre string) ([]DirEntry, error) {
	base := nombre
	for len(base) > 0 && base[len(base)-1] == '/' {
		base = base[:len(base)-1]
	}
	datos, err := ReadFile(base + "/.manifiesto")
	if err != nil {
		return nil, ErrNotExist
	}
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
				salida = append(salida, DirEntry{nombre: campos[0], tam: int64(tam), tipo: tipo})
			}
		}
		linea = ""
	}
	return salida, nil
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

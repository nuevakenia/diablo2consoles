// Medidor de la SOURCE DE CONSOLA (DC-002.96) en el HOST: los seis formatos que el camino de consola tiene
// que cargar por la fuente nueva, con su tamano y su crc32.
//
// PARA QUE. La corrida de la Dreamcast (`console/dc/poc/fuente-consola`) imprime los mismos numeros; estos
// son el corpus contra el que se comparan. Si un crc32 no coincide, lo que se leyo en la consola no es el
// archivo (o el camino de la fuente nueva lo sirve mal), y eso se ve en el numero, no en un relato.
//
// MIDE LOS DOS CAMINOS, uno al lado del otro, sobre el mismo archivo:
//
//	FUENTE  la Source de consola nueva: `Pedir` (buffer prestado, del tamano exacto, del que llama) y
//	        `Liberar` (lo devuelve), mas `LeerEnteroEn` y `LeerEnteroEnConPrestamo` para comprobar que los
//	        tres caminos dan el MISMO contenido que `ReadFile`;
//	VIEJO   `mpq.ReadFile`, o sea el buffer que se pide por archivo y no lo libera nadie.
//
// Uso:
//
//	go run ./console/dc/medidas/fuente-consola [-mpq ruta/d2data.mpq] [-listar filtro]
//
// Sin -mpq usa la instalacion del juego (D2_DIR o ~/.wine-d2/drive_c/Games/Diablo II).
package main

import (
	"flag"
	"fmt"
	"hash/crc32"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/OpenDiablo2/OpenDiablo2/console/dc/fuenteconsola"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2mpq"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2loader/asset"
)

// Los seis formatos que el camino de consola tiene que cargar por la Source nueva (criterio de la tarjeta).
// Son de `d2data.mpq` a proposito: es el unico MPQ que la imagen del POC lleva, asi que lo que se mide en la
// Dreamcast es exactamente esto.
//
// El .dt1 es el MAS GRANDE del pueblo (1.008.003 B): es el archivo de ~1 MB de la prueba en contra del
// monton. El .dcc y el .dc6 pasan de 64 KB (los que el runtime manda al monton externo y hay que devolver a
// mano); el .cof de un objeto mide 253 B (se queda en el semiespacio: no hay que liberarlo).
const (
	// el archivo de ~1 MB de la prueba en contra del monton
	dt1Grande = `data\global\tiles\ACT1\TOWN\floor.dt1`
	ds1Pueblo = `data\global\tiles\ACT1\TOWN\townN1.ds1`
	dccGrande = `data\global\objects\AA\S2\aas2litnuhth.dcc`
	cofObjeto = `data\global\objects\E2\COF\e2S1HTH.cof`
	dc6Grande = `data\global\ui\FrontEnd\ts11b.DC6`
	animdata  = `data\global\AnimData.D2`
)

// losSeis: en el orden en que se miden y se informan.
var losSeis = []struct {
	formato string
	nombre  string
}{
	{".dt1", dt1Grande},
	{".ds1", ds1Pueblo},
	{".dcc", dccGrande},
	{".cof", cofObjeto},
	{".dc6", dc6Grande},
	{"animdata", animdata},
}

// autodetectarD2: la instalacion del juego no vive en el repo.
func autodetectarD2() string {
	if d := os.Getenv("D2_DIR"); d != "" {
		return d
	}

	candidatas := []string{
		filepath.Join(os.Getenv("HOME"), ".wine-d2", "drive_c", "Games", "Diablo II"),
	}

	for _, c := range candidatas {
		if st, err := os.Stat(filepath.Join(c, "d2data.mpq")); err == nil && !st.IsDir() {
			return c
		}
	}

	return ""
}

func main() {
	mpqPath := flag.String("mpq", "", "archivo MPQ (por defecto d2data.mpq de la instalacion)")
	listar := flag.String("listar", "", "filtro (subcadena, sin distinguir mayusculas) para LISTAR la tabla del MPQ con tamanos y salir")
	ext := flag.String("ext", "", "con -listar: quedarse solo con esta extension (ej. .dt1)")
	flag.Parse()

	ruta := *mpqPath
	if ruta == "" {
		carpeta := autodetectarD2()
		if carpeta == "" {
			fmt.Fprintln(os.Stderr, "no encuentro la instalacion de Diablo II (usa -mpq o D2_DIR)")
			os.Exit(1)
		}

		ruta = filepath.Join(carpeta, "d2data.mpq")
	}

	f, err := fuenteconsola.Abrir(ruta)
	if err != nil {
		fmt.Fprintln(os.Stderr, "no pude abrir el MPQ con la fuente de consola:", err)
		os.Exit(1)
	}
	defer func() { _ = f.Cerrar() }()

	if *listar != "" {
		listarTabla(f, *listar, *ext)

		return
	}

	fmt.Printf("MEDIDOR: mpq=%s\n", ruta)
	fmt.Printf("MEDIDOR: la fuente implementa las tres interfaces: Source=%v SourceDeLecturaEntera=%v SourceDeLecturaConPrestamo=%v\n",
		esSource(f), esLecturaEntera(f), esLecturaConPrestamo(f))

	fallos := 0

	for _, caso := range losSeis {
		if !medirUno(f, caso.formato, caso.nombre) {
			fallos++
		}
	}

	e := f.Estado()
	fmt.Printf("MEDIDOR: instrumento de la fuente: pedidos=%d liberados=%d bytesVivos=%d externos=%d "+
		"enBuffer=%d conPrestamo=%d abiertos=%d sinTamano=%d prestamo=%d crecidas=%d\n",
		e.Pedidos, e.Liberados, e.BytesVivos, e.LibresExternos, e.LecturasEnBuffer, e.LecturasConPrestamo,
		e.Abiertos, e.SinTamano, e.PrestamoReservado, e.PrestamoCrecidas)

	if fallos == 0 {
		fmt.Println("MEDIDOR: OK -- los seis formatos dan el mismo contenido por la fuente nueva que por el camino viejo")
	} else {
		fmt.Printf("MEDIDOR: MAL -- %d de %d archivos NO coinciden\n", fallos, len(losSeis))
		os.Exit(1)
	}
}

// medirUno mide UN archivo por los cuatro caminos de la fuente nueva y por el camino viejo, y exige que el
// contenido sea el mismo (crc32). Devuelve false si algo no coincide.
func medirUno(f *fuenteconsola.Fuente, formato, nombre string) bool {
	tamano, sabido := f.Tamano(nombre)
	if !sabido {
		fmt.Printf("MEDIDOR: %-8s %-52s SIN-TAMANO\n", formato, nombre)

		return false
	}

	// el camino VIEJO: el buffer por archivo que no libera nadie
	viejo, err := f.Open(nombre)
	if err != nil {
		fmt.Printf("MEDIDOR: %-8s %-52s VIEJO-FALLO: %v\n", formato, nombre, err)

		return false
	}

	crudoViejo, err := leerTodo(viejo)
	if err != nil {
		fmt.Printf("MEDIDOR: %-8s %-52s VIEJO-LEER-FALLO: %v\n", formato, nombre, err)

		return false
	}

	crcViejo := crc(crudoViejo)

	// camino 1: el destino lo pone el que llama (no pide memoria)
	dst := make([]byte, tamano)

	leidos, err := f.LeerEnteroEn(nombre, dst)
	if err != nil {
		fmt.Printf("MEDIDOR: %-8s %-52s LEERENTEROS-FALLO: %v\n", formato, nombre, err)

		return false
	}

	crcEn := crc(dst[:leidos])

	// camino 2: con la memoria de trabajo prestada (ni destino ni lectura piden nada)
	dst2 := make([]byte, tamano)

	leidos2, err := f.LeerEnteroEnConPrestamo(nombre, dst2, nil)
	if err != nil {
		fmt.Printf("MEDIDOR: %-8s %-52s CONPRESTAMO-FALLO: %v\n", formato, nombre, err)

		return false
	}

	crcPres := crc(dst2[:leidos2])

	// camino 3: el tramo prestado, con su dueno explicito
	tramo, err := f.Pedir(nombre)
	if err != nil {
		fmt.Printf("MEDIDOR: %-8s %-52s PEDIR-FALLO: %v\n", formato, nombre, err)

		return false
	}

	crcTramo := crc(tramo.Datos)

	tramo.Liberar() // el que pidio el tramo lo devuelve

	liberado := tramo.Liberado() && tramo.Datos == nil

	iguales := crcEn == crcViejo && crcPres == crcViejo && crcTramo == crcViejo

	veredicto := "OK"
	if !iguales || !liberado {
		veredicto = "MAL"
	}

	fmt.Printf("MEDIDOR: %-8s %-52s tamano=%8d crc32=%10d viejo=%10d enBuffer=%10d prestamo=%10d liberado=%v %s\n",
		formato, nombre, tamano, crcTramo, crcViejo, crcEn, crcPres, liberado, veredicto)

	return iguales && liberado
}

// listarTabla: la tabla del MPQ es la unica verdad sobre como se llaman los archivos. Imprime el tamano
// descomprimido (que es lo que hay que reservar), ordenado de mayor a menor: es como se eligieron los seis
// archivos de arriba.
func listarTabla(f *fuenteconsola.Fuente, filtro, ext string) {
	filtro = strings.ToLower(filtro)
	ext = strings.ToLower(ext)

	tipos := []struct {
		nombre string
		tamano int
	}{}

	for _, n := range nombresDelMPQ(f) {
		bajo := strings.ToLower(n)
		if !strings.Contains(bajo, filtro) {
			continue
		}

		if ext != "" && !strings.HasSuffix(bajo, ext) {
			continue
		}

		if tam, hay := f.Tamano(n); hay {
			tipos = append(tipos, struct {
				nombre string
				tamano int
			}{n, tam})
		}
	}

	sort.Slice(tipos, func(i, j int) bool { return tipos[i].tamano > tipos[j].tamano })

	fmt.Printf("LISTA filtro=%q ext=%q entradas=%d\n", filtro, ext, len(tipos))

	for _, t := range tipos {
		fmt.Printf("%10d  %s\n", t.tamano, t.nombre)
	}
}

// nombresDelMPQ: la tabla de nombres del MPQ, por el camino que ya expone el paquete (el listfile).
func nombresDelMPQ(f *fuenteconsola.Fuente) []string {
	m, err := d2mpq.FromFile(f.Path())
	if err != nil {
		return nil
	}
	defer func() { _ = m.Close() }()

	nombres, err := m.Listfile()
	if err != nil {
		return nil
	}

	return nombres
}

// leerTodo: el consumidor de siempre de un stream (lo que hace `io.ReadAll`), sin la stdlib de ioutil.
func leerTodo(r interface{ Read([]byte) (int, error) }) ([]byte, error) {
	var (
		salida []byte
		buf    = make([]byte, 32*1024)
	)

	for {
		n, err := r.Read(buf)
		if n > 0 {
			salida = append(salida, buf[:n]...)
		}

		if err != nil {
			if err.Error() == "EOF" {
				return salida, nil
			}

			return salida, err
		}
	}
}

func crc(b []byte) uint32 { return crc32.ChecksumIEEE(b) }

// las tres aserciones, en vivo: el medidor no cree la asercion de compilacion del paquete, la mide.
func esSource(f *fuenteconsola.Fuente) bool {
	_, ok := interface{}(f).(asset.Source)

	return ok
}

func esLecturaEntera(f *fuenteconsola.Fuente) bool {
	_, ok := interface{}(f).(asset.SourceDeLecturaEntera)

	return ok
}

func esLecturaConPrestamo(f *fuenteconsola.Fuente) bool {
	_, ok := interface{}(f).(asset.SourceDeLecturaConPrestamo)

	return ok
}

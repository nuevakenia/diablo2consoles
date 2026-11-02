// sonda-muerte-126: POR QUE EL MODO DD DEL RETURNED VIENE VACIO.
//
// DC-002.122 midio, en la consola, que el golpe mata al Returned y que el modo DD se alcanza
// (`MODO-BICHO modo= DD cuadros= 1 msPorCuadro= 67 vida= 0`), pero el cuadro compuesto sale con
// **0 pixeles**: el cadaver no se ve. Esta sonda mira el MPQ del disco con los lectores del MOTOR, sin
// emulador, y contesta la pregunta con numeros: que archivos hay para el modo DD del SK, que trae cada
// uno, y cuanto queda al componer.
//
// Lo que mide, en orden:
//
//  1. EL (listfile): los nombres del MPQ que son del SK y llevan "dd" (que graficos de DD existen, uno
//     por capa). Si no hay ninguno, el DD no tiene grafico propio y hay que usar otro cuadro (el ultimo
//     del DT, que es lo que hace el juego).
//  2. EL COF DD: bytes, crc32, capas, cuadros por direccion, direcciones.
//  3. CAPA POR CAPA: el archivo que pide el COF para esa capa en DD (dcc y dc6), su caja, sus cuadros y
//     cuantos pixeles NO son el indice 0 (el instrumento de siempre) en cada cuadro.
//  4. EL COMPUESTO: el cuadro DD compuesto por prioridad (como lo hace el POC: caja union + capas en el
//     orden de Priority), y al lado el MISMO compuesto del ULTIMO cuadro del DT. Los dos numeros dicen
//     cual de los dos se ve.
//
// Uso:
//
//	go run ./console/dc/sonda-muerte-126 [-token SK] [-dir 6]
package main

import (
	"flag"
	"fmt"
	"hash/crc32"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2enum"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2cof"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2dc6"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2dcc"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2mpq"
)

// elMPQ: el archivo abierto, para que la composicion no lo vuelva a abrir por capa.
var elMPQ *d2mpq.MPQ

func main() {
	mpqPath := flag.String("mpq", "", "d2data.mpq (por defecto, la instalacion del juego)")
	token := flag.String("token", "SK", "el token del bicho (la carpeta de monsters)")
	clase := flag.String("clase", "Returned", "el Id de monstats.txt (solo para el encabezado)")
	dir := flag.Int("dir", 6, "la direccion del COF que se compone (la del log de DC-002.122)")
	weapon := flag.String("arma", "HTH", "la clase de arma del COF (hth: desarmado)")
	flag.Parse()

	ruta := *mpqPath
	if ruta == "" {
		ruta = filepath.Join(os.Getenv("HOME"), ".wine-d2", "drive_c", "Games", "Diablo II", "d2data.mpq")
	}

	m, err := d2mpq.FromFile(ruta)
	if err != nil {
		fmt.Fprintln(os.Stderr, "MPQ:", err)
		os.Exit(1)
	}

	defer m.Close()

	elMPQ = m

	fmt.Printf("SONDA126: MPQ %s\n", ruta)
	fmt.Printf("SONDA126: BICHO clase=%s token=%s arma=%s dir=%d\n", *clase, *token, *weapon, *dir)

	// ============================== 1. QUE GRAFICOS DE DD HAY EN EL MPQ ==============================
	//
	// El (listfile) es la unica forma de saber que existe sin adivinar nombres. En el HOST se lee bien
	// (en la consola el (listfile) del tercer MPQ se lleva la corrida puesta: DC-002.39c).
	carpeta := strings.ToLower("monsters\\" + *token + "\\")

	crudoLista, err := m.ReadFile("(listfile)")
	if err != nil || len(crudoLista) == 0 {
		fmt.Printf("SONDA126: LISTFILE NO SE LEE err=%v\n", err)
	} else {
		nombres := strings.Split(strings.ReplaceAll(string(crudoLista), "\r", ""), "\n")

		delBicho := 0
		porModo := map[string]int{}
		dd := []string{}

		for _, n := range nombres {
			bajo := strings.ToLower(strings.TrimSpace(n))
			if !strings.Contains(bajo, carpeta) || !strings.HasSuffix(bajo, ".dcc") {
				continue
			}

			delBicho++

			// el nombre es <token><capa>LIT<modo><arma>.dcc: el modo son los 2 caracteres antes del arma
			// y despues de "lit" (o del valor de capa que sea).
			campos := strings.Split(bajo, "lit")
			if len(campos) == 2 && len(campos[1]) >= 6 {
				modo := campos[1][:2]
				porModo[modo]++

				if modo == "dd" {
					dd = append(dd, strings.TrimSpace(n))
				}
			}
		}

		sort.Strings(dd)

		fmt.Printf("SONDA126: LISTFILE nombres=%d delBicho=%d grafiasDD=%d\n", len(nombres), delBicho, len(dd))
		fmt.Printf("SONDA126: MODOS-CON-GRAFICO %v\n", modosOrdenados(porModo))

		if len(dd) == 0 {
			fmt.Println("SONDA126: DD SIN-GRAFICO PROPIO: el MPQ no trae ningun .dcc del SK en modo DD")
		}

		for _, n := range dd {
			tam, ok := m.TamanoDeArchivo(n)
			fmt.Printf("SONDA126: DD-ARCHIVO %s bytes=%d descomprimido=%v\n", n, tam, ok)
		}
	}

	// ============================== 2. EL COF DD (y el DT para comparar) ==============================
	cofDD, bytesDD, crcDD := leerCOF(m, *token, "DD", *weapon)
	cofDT, bytesDT, crcDT := leerCOF(m, *token, "DT", *weapon)

	if cofDD == nil || cofDT == nil {
		fmt.Println("SONDA126: no puedo seguir sin los COF DD y DT")

		return
	}

	fmt.Printf("SONDA126: COF DD bytes=%d crc32=%d capas=%d cuadrosPorDir=%d dirs=%d\n",
		bytesDD, crcDD, len(cofDD.CofLayers), cofDD.FramesPerDirection, cofDD.NumberOfDirections)
	fmt.Printf("SONDA126: COF DT bytes=%d crc32=%d capas=%d cuadrosPorDir=%d dirs=%d\n",
		bytesDT, crcDT, len(cofDT.CofLayers), cofDT.FramesPerDirection, cofDT.NumberOfDirections)

	// ============================== 3. CAPA POR CAPA: QUE ARCHIVO HAY Y QUE TRAE ==============================
	capasDD := capasDeUnModo(m, *token, "DD", *weapon, *dir, cofDD)
	capasDT := capasDeUnModo(m, *token, "DT", *weapon, *dir, cofDT)

	// ============================== 4. EL COMPUESTO: DD vs el ULTIMO CUADRO DEL DT ==============================
	anchoDD, altoDD, pixelesDD := componerCuadro(cofDD, capasDD, *dir, 0, "DD")

	ultimoDT := cofDT.FramesPerDirection - 1
	anchoDT, altoDT, pixelesDT := componerCuadro(cofDT, capasDT, *dir, ultimoDT, "DT")

	fmt.Println()
	fmt.Printf("SONDA126: COMPUESTO DD dir=%d cuadro=0 caja=%dx%d pixelesNoVacios=%d\n", *dir, anchoDD, altoDD, pixelesDD)
	fmt.Printf("SONDA126: COMPUESTO DT dir=%d cuadro=%d caja=%dx%d pixelesNoVacios=%d\n", *dir, ultimoDT, anchoDT, altoDT, pixelesDT)

	fmt.Println()
	fmt.Printf("SONDA126: VEREDICTO ddVacio=%v dtUltimoCuadroVacio=%v\n", pixelesDD == 0, pixelesDT == 0)
}

// capa: lo que trajo UNA capa del COF para un modo y una direccion.
type capa struct {
	tipo      string
	clave     d2enum.CompositeType
	archivo   string // el que se encontro ("" si ninguno)
	bytes     int
	fps       string
	caja      [4]int // left, top, ancho, alto
	cuadros   int
	pixeles   []int // pixeles no vacios por cuadro
	prioridad int   // el indice en Priority[dir][frame0]
}

// capasDeUnModo: por cada capa del COF, el archivo que se encuentra (dcc primero, dc6 despues, como el
// motor) y lo que trae. Imprime una linea por capa.
func capasDeUnModo(m *d2mpq.MPQ, token, modo, arma string, dir int, cof *d2cof.COF) []capa {
	capas := make([]capa, 0, len(cof.CofLayers))

	// la prioridad del cuadro 0 de esa direccion: dice en que orden se pintan (y cuales se pintan)
	orden := map[d2enum.CompositeType]int{}
	if dir < len(cof.Priority) && len(cof.Priority[dir]) > 0 {
		for i, idx := range cof.Priority[dir][0] {
			orden[idx] = i
		}
	}

	for i := range cof.CofLayers {
		l := cof.CofLayers[i]
		c := capa{tipo: l.Type.String(), clave: l.Type, prioridad: -1}
		if p, ok := orden[l.Type]; ok {
			c.prioridad = p
		}

		// las dos rutas que prueba el motor, con el valor de capa por omision ("lit")
		base := fmt.Sprintf(`data\global\monsters\%s\%s\%s%sLIT%s%s`,
			token, l.Type.String(), token, l.Type.String(), modo, l.WeaponClass.String())

		for _, ext := range []string{"dcc", "dc6"} {
			nombre := base + "." + ext

			datos, err := m.ReadFile(nombre)
			if err != nil || len(datos) == 0 {
				continue
			}

			c.archivo, c.bytes, c.fps = nombre, len(datos), fmt.Sprintf("crc32=%d", crc32.ChecksumIEEE(datos))

			if ext == "dcc" {
				dd, err := d2dcc.LoadDirection(datos, dir)
				if err != nil {
					c.fps = "PARSE-FALLO: " + err.Error()

					break
				}

				caja := dd.Directions[dir].Box
				c.caja = [4]int{caja.Left, caja.Top, caja.Width, caja.Height}
				c.cuadros = len(dd.Directions[dir].Frames)

				for f := range dd.Directions[dir].Frames {
					c.pixeles = append(c.pixeles, noCero(dd.Directions[dir].Frames[f].PixelData))
				}

				break
			}

			dc, err := d2dc6.Load(datos)
			if err != nil {
				c.fps = "PARSE-FALLO: " + err.Error()

				break
			}

			c.cuadros = len(dc.Frames)

			for f := range dc.Frames {
				c.pixeles = append(c.pixeles, noCero(dc.DecodeFrame(f)))
			}

			break
		}

		estado := "OK"
		if c.archivo == "" {
			estado = "SIN-GRAFICO"
		}

		fmt.Printf("SONDA126: CAPA modo=%s tipo=%-3s prioridad=%d estado=%s %s bytes=%d %s caja=%d,%d %dx%d cuadros=%d pixelesPorCuadro=%v\n",
			modo, c.tipo, c.prioridad, estado, c.archivo, c.bytes, c.fps,
			c.caja[0], c.caja[1], c.caja[2], c.caja[3], c.cuadros, c.pixeles)

		capas = append(capas, c)
	}

	return capas
}

// componerCuadro: el cuadro del POC, con las capas del COF en el orden de SU prioridad (el del motor):
// la caja union de las capas dibujables y los pixeles pegados con el indice 0 como transparente. Devuelve
// el ancho, el alto y cuantos pixeles quedaron NO vacios.
func componerCuadro(cof *d2cof.COF, capas []capa, dir, cuadro int, modo string) (int, int, int) {
	minX, minY, maxX, maxY := 0, 0, 0, 0
	primero := true

	for _, c := range capas {
		if c.archivo == "" || c.caja[2] <= 0 || c.caja[3] <= 0 {
			continue
		}

		if primero {
			minX, minY, maxX, maxY = c.caja[0], c.caja[1], c.caja[0]+c.caja[2], c.caja[1]+c.caja[3]
			primero = false

			continue
		}

		minX = minimo(minX, c.caja[0])
		minY = minimo(minY, c.caja[1])
		maxX = maximo(maxX, c.caja[0]+c.caja[2])
		maxY = maximo(maxY, c.caja[1]+c.caja[3])
	}

	ancho, alto := maxX-minX, maxY-minY
	if ancho <= 0 || alto <= 0 {
		return 0, 0, 0
	}

	lienzo := make([]byte, ancho*alto)

	// el orden del motor: Priority[dir][cuadro] son los TIPOS de capa, en orden de pintado
	orden := []d2enum.CompositeType{}

	if dir < len(cof.Priority) && len(cof.Priority[dir]) > cuadro {
		orden = cof.Priority[dir][cuadro]
	}

	for _, tipo := range orden {
		idx := -1

		for i := range capas {
			if capas[i].clave == tipo {
				idx = i

				break
			}
		}

		if idx < 0 {
			continue
		}

		c := capas[idx]
		if c.archivo == "" || len(c.pixeles) == 0 {
			continue
		}

		datos := pixelesDeLaCapa(c.archivo, dir, cuadro)
		if datos == nil {
			continue
		}

		for y := 0; y < c.caja[3]; y++ {
			for x := 0; x < c.caja[2]; x++ {
				v := datos[x+y*c.caja[2]]
				if v == 0 {
					continue
				}

				lx := c.caja[0] - minX + x
				ly := c.caja[1] - minY + y
				if lx < 0 || ly < 0 || lx >= ancho || ly >= alto {
					continue
				}

				lienzo[lx+ly*ancho] = v
			}
		}
	}

	return ancho, alto, noCero(lienzo)
}

// pixelesDeLaCapa: los pixeles de un cuadro de una capa (dcc o dc6, segun el archivo).
func pixelesDeLaCapa(archivo string, dir, cuadro int) []byte {
	datos, err := elMPQ.ReadFile(archivo)
	if err != nil || len(datos) == 0 {
		return nil
	}

	if strings.HasSuffix(strings.ToLower(archivo), ".dcc") {
		dd, err := d2dcc.LoadDirection(datos, dir)
		if err != nil || cuadro >= len(dd.Directions[dir].Frames) {
			return nil
		}

		return dd.Directions[dir].Frames[cuadro].PixelData
	}

	dc, err := d2dc6.Load(datos)
	if err != nil || cuadro >= len(dc.Frames) {
		return nil
	}

	return dc.DecodeFrame(cuadro)
}

// leerCOF: los bytes de un COF del bicho en un modo ("" y 0 si no esta).
func leerCOF(m *d2mpq.MPQ, token, modo, arma string) (*d2cof.COF, int, uint32) {
	nombre := fmt.Sprintf(`data\global\monsters\%s\COF\%s%s%s.cof`,
		token, strings.ToLower(token), strings.ToLower(modo), strings.ToLower(arma))

	datos, err := m.ReadFile(nombre)
	if err != nil || len(datos) == 0 {
		return nil, 0, 0
	}

	cof, err := d2cof.Unmarshal(datos)
	if err != nil {
		return nil, len(datos), crc32.ChecksumIEEE(datos)
	}

	return cof, len(datos), crc32.ChecksumIEEE(datos)
}

// modosOrdenados: los modos con grafico del bicho, como texto ordenado ("A1,N U,W L...").
func modosOrdenados(porModo map[string]int) []string {
	claves := make([]string, 0, len(porModo))
	for k, v := range porModo {
		claves = append(claves, fmt.Sprintf("%s=%d", k, v))
	}

	sort.Strings(claves)

	return claves
}

func noCero(pix []byte) int {
	n := 0

	for _, v := range pix {
		if v != 0 {
			n++
		}
	}

	return n
}

func minimo(a, b int) int {
	if a < b {
		return a
	}

	return b
}

func maximo(a, b int) int {
	if a > b {
		return a
	}

	return b
}

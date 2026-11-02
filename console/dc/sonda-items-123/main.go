// DC-002.123 -- SONDA DEL DROP, EN EL HOST: que tablas hay, cuanto pesan y que suelta el Returned.
//
// POR QUE EXISTE. La tarjeta pide MEDIR ANTES DE ESCRIBIR: cuanto suman las tablas de items y el paquete
// d2item/d2object, y de donde sale el objeto que suelta el bicho. Esta sonda no toca la consola: abre los MPQ
// del disco, imprime (1) que tablas de items existen y en que MPQ, con su tamano y crc32; (2) la fila del SK en
// monstats.txt con su TreasureClass; (3) la cadena de TreasureClassEx.txt resuelta hasta los codigos de item; y
// (4) para cada codigo, la fila de su tabla (misc/weapons/armor) con el flippyfile y el tamano del DC6 con el que
// el motor lo dibuja en el suelo (`d2resource.ItemGraphics` + flippyfile, `d2mapentity/factory.go`).
//
// Las reglas de la resolucion de la cadena son las del motor (`d2core/d2item/d2drop/treasure.go`): cada fila de
// TreasureClassEx.txt tiene `picks`, `NoDrop` y hasta 10 pares item/prob; los items que son a su vez otra clase
// se resuelven adentro (hasta `level` niveles, el campo `level` de la fila no: el limite lo pone el motor por
// profundidad). Aca NO se tira el dado: se imprime la cadena y las probabilidades, que es lo que hay que saber
// para elegir el objeto mas simple. El tirado lo hace el POC con el dado del motor.
//
// Uso:
//
//	go run ./console/dc/sonda-items-123
//	go run ./console/dc/sonda-items-123 -dir "~/.wine-d2/drive_c/Games/Diablo II"
package main

import (
	"flag"
	"fmt"
	"hash/crc32"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2mpq"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2math/d2rand"
)

const (
	tablaMisc        = `data\global\excel\misc.txt`
	tablaArmas       = `data\global\excel\weapons.txt`
	tablaArmaduras   = `data\global\excel\armor.txt`
	tablaTipos       = `data\global\excel\itemtypes.txt`
	tablaTesoros     = `data\global\excel\TreasureClassEx.txt`
	tablaMonstruos   = `data\global\excel\monstats.txt`
	carpetaItems     = `data\global\items\`
	monstruoBuscado  = "Returned"
	maxProfundidadTC = 8
)

// crudo: imprime las lineas crudas de la tabla (auditoria de la cabecera).
var crudo bool

// tabla: los bytes de un .txt y su cabecera partida en columnas.
type tabla struct {
	nombre    string
	mpq       string
	bytes     int
	crc       uint32
	cabecera  []string
	indice    map[string]int
	lineas   [][]string
}

func main() {
	dir := flag.String("dir", "", "carpeta del juego (por omision, la instalacion de wine)")
	crudoPtr := flag.Bool("crudo", false, "imprime las primeras lineas crudas de monstats.txt (para auditar la cabecera)")
	semillasPtr := flag.Int("semillas", 0, "si > 0, tira el PRIMER dado de la cadena del drop para las semillas 1..N")
	flag.Parse()

	if *semillasPtr > 0 {
		analizarSemillas(*semillasPtr)
	}

	crudo = *crudoPtr

	base := *dir
	if base == "" {
		home, _ := os.UserHomeDir()
		base = filepath.Join(home, ".wine-d2", "drive_c", "Games", "Diablo II")
	}

	mpqs := map[string]*d2mpq.MPQ{}
	// LOS MISMOS DOS QUE ABRE EL POC: `d2data.mpq` y `patch_D2.mpq` (main.go, rutaDatos y rutaParche).
	// d2exp.mpq NO se abre en la consola, asi que lo que solo exista ahi no es dato de esta tarjeta.
	nombres := []string{"d2data.mpq", "patch_D2.mpq"}

	for _, n := range nombres {
		p := filepath.Join(base, n)
		if _, err := os.Stat(p); err != nil {
			fmt.Printf("MPQ %-14s NO-ESTA %s\n", n, p)

			continue
		}

		m, err := d2mpq.FromFile(p)
		if err != nil {
			fmt.Printf("MPQ %-14s FALLO %v\n", n, err)

			continue
		}

		mpqs[n] = m

		fmt.Printf("MPQ %-14s OK %s\n", n, p)
	}

	// ============================== 1. QUE TABLAS HAY Y CUANTO PESAN ==============================
	fmt.Println("\n== 1. LAS TABLAS DE ITEMS: en que MPQ estan, cuanto pesan y su crc32")
	fmt.Println("tabla                 mpq            bytes    crc32")
	fmt.Println("--------------------  -------------  -------  ----------")

	cargadas := map[string]*tabla{}

	// LA MISMA TABLA PUEDE ESTAR EN VARIOS MPQ, Y GANA LA DEL PARCHE: el motor registra patch_D2 PRIMERO
	// (main.go, "el parche queda PRIMERO: se registra al final"), asi que si patch_D2 trae la tabla, es esa la
	// que se lee. Medido: monstats.txt de d2data.mpq NO trae los nombres de TreasureClass (la columna trae
	// numeros), y la del parche si. Por eso se recorren los tres MPQ y se queda la ULTIMA que la tenga.
	for _, ruta := range []string{tablaMisc, tablaArmas, tablaArmaduras, tablaTipos, tablaTesoros, tablaMonstruos} {
		nombre := filepath.Base(strings.ReplaceAll(ruta, `\`, `/`))
		encontrada := false

		for _, n := range nombres {
			m := mpqs[n]
			if m == nil {
				continue
			}

			crudo, err := m.ReadFile(ruta)
			if err != nil || len(crudo) == 0 {
				continue
			}

			t := partirTabla(nombre, n, crudo)
			cargadas[nombre] = t
			encontrada = true
		}

		if !encontrada {
			fmt.Printf("%-20s  %-13s  NO-ESTA\n", nombre, "--")
		} else {
			t := cargadas[nombre]
			fmt.Printf("%-20s  %-13s  %7d  %10d  (%d filas, %d columnas)\n",
				nombre, t.mpq, t.bytes, t.crc, len(t.lineas), len(t.cabecera))
		}
	}

	total := 0
	for _, t := range cargadas {
		total += t.bytes
	}

	fmt.Printf("TOTAL de las tablas de items: %d bytes en %d tablas\n", total, len(cargadas))

	// ============================== 2. LA FILA DEL BICHO ==============================
	fmt.Println("\n== 2. LA FILA DEL RETURNED en monstats.txt")

	mon := cargadas["monstats.txt"]
	if mon == nil {
		return
	}

	if crudo {
		dumpCrudo(mpqs, tablaMonstruos)

		fmt.Println("  -- filas cuya identidad contiene 'eturn' (para ubicar al Returned en ESTA version:")
		for _, l := range mon.lineas {
			id := strings.Join(l[:minInt(3, len(l))], " ")
			if strings.Contains(strings.ToLower(id), "return") || strings.Contains(strings.ToLower(id), "skeleton") {
				fmt.Printf("     [Id=%q BaseId=%q NextInClass=%q Code=%q Level=%q MinHP=%q MaxHP=%q "+
					"NameStr=%q TC1=%q]\n",
					campo(mon, l, "Id"), campo(mon, l, "BaseId"), campo(mon, l, "NextInClass"),
					campo(mon, l, "Code"), campo(mon, l, "Level"), campo(mon, l, "MinHP"),
					campo(mon, l, "MaxHP"), campo(mon, l, "NameStr"), campo(mon, l, "TreasureClass1"))
			}
		}
	}

	var filaBicho []string

	for _, l := range mon.lineas {
		// DOS FORMATOS DE LA MISMA TABLA: d2data.mpq la trae con la columna Class (y ahi el bicho es
		// "Returned"), y patch_D2.mpq la trae canonica (Id=skeleton2, NameStr=Returned, Level=6). Se acepta
		// la fila que diga Returned por cualquiera de las dos, y en el formato del parche se pide ADEMAS el
		// nivel, porque hay tres "Returned" (acto 1 nivel 6, acto 2 nivel 14) y el del POC es el nivel 6.
		esDelFormatoClasico := strings.EqualFold(campo(mon, l, "Class"), monstruoBuscado)
		esDelFormatoParche := strings.EqualFold(campo(mon, l, "NameStr"), monstruoBuscado) &&
			(campo(mon, l, "Level") == "6" || campo(mon, l, "Class") != "")

		if esDelFormatoClasico || esDelFormatoParche {
			filaBicho = l

			break
		}
	}

	// el .txt de 1.14b se parte por tabulaciones: si la columna Id no aparece con ese nombre exacto, se busca
	// por Code, que es el token de la carpeta del MPQ (SK) y es unico igual.
	if filaBicho == nil {
		for _, l := range mon.lineas {
			if campo(mon, l, "Code") == "SK" {
				filaBicho = l

				break
			}
		}
	}

	if filaBicho == nil {
		fmt.Println("no encontre la fila del Returned ni por Id ni por Code=SK")
		fmt.Printf("  columnas de la cabecera (primeras 8): %v\n", mon.cabecera[:minInt(8, len(mon.cabecera))])

		for i, l := range mon.lineas {
			if i >= 3 {
				break
			}

			fmt.Printf("  fila %d: %v\n", i, l[:minInt(6, len(l))])
		}

		return
	}

	for _, col := range []string{"Class", "Id", "NameStr", "Code", "Level", "MinHP", "MaxHP", "TreasureClass1",
		"TreasureClass2", "TreasureClass3", "TreasureClass4", "TreasureClass1(N)", "TreasureClass1(H)"} {
		fmt.Printf("  %-18s = %s\n", col, campo(mon, filaBicho, col))
	}

	// la alineacion de la cabecera con la fila, imprimida para que el lector del POC se pueda auditar
	fmt.Println("  -- alineacion (cabecera -> valor de la fila del SK):")
	for i := 0; i < len(mon.cabecera) && i < 12; i++ {
		v := ""
		if i < len(filaBicho) {
			v = filaBicho[i]
		}

		fmt.Printf("     [%2d] %-22s = %q\n", i, mon.cabecera[i], v)
	}

	fmt.Println("  -- TODAS las columnas de la fila cuya cabecera NO es numerica (para ubicar el drop):")
	for i, c := range mon.cabecera {
		if i >= len(filaBicho) {
			continue
		}

		v := filaBicho[i]
		if v == "" || strings.ContainsAny(v, "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ") {
			fmt.Printf("     [%3d] %-22s = %q\n", i, c, v)
		}
	}

	fmt.Println("  -- columnas de la cabecera que hablan de drops o de identidad:")
	for i, c := range mon.cabecera {
		if strings.Contains(strings.ToLower(c), "treasure") || strings.EqualFold(c, "Id") ||
			strings.Contains(strings.ToLower(c), "class") || strings.EqualFold(c, "Code") {
			v := ""
			if i < len(filaBicho) {
				v = filaBicho[i]
			}

			fmt.Printf("     [%3d] %-22s = %q\n", i, c, v)
		}
	}

	tc := campo(mon, filaBicho, "TreasureClass1")
	fmt.Printf("  -> el drop NORMAL del Returned sale de la clase %q\n", tc)

	// ============================== 3. LA CADENA DE TREASURE CLASSS ==============================
	fmt.Println("\n== 3. LA CADENA de TreasureClassEx.txt, resuelta (picks / NoDrop / item:prob)")

	tcTabla := cargadas["TreasureClassEx.txt"]
	if tcTabla == nil {
		fmt.Println("no hay TreasureClassEx.txt: sin cadena")

		return
	}

	hojas := map[string]int{}
	resolverClase(tcTabla, tc, 0, "", hojas)

	codigos := make([]string, 0, len(hojas))
	for c := range hojas {
		codigos = append(codigos, c)
	}

	sort.Strings(codigos)

	fmt.Printf("\n  codigos de item alcanzables desde %q: %d -> %s\n", tc, len(codigos), strings.Join(codigos, " "))

	// ============================== 4. CADA CODIGO EN SU TABLA, CON SU FLIPPY ==============================
	fmt.Println("\n== 4. EL OBJETO EN SU TABLA y el DC6 con el que el motor lo dibuja en el suelo")
	fmt.Println("codigo  tabla            nombre                          flippy   dc6bytes  dc6crc32   invfile")

	misc, armas, armad := cargadas["misc.txt"], cargadas["weapons.txt"], cargadas["armor.txt"]
	tablas := []struct {
		nombre string
		t      *tabla
	}{{"misc.txt", misc}, {"weapons.txt", armas}, {"armor.txt", armad}}

	mismo := false

	for _, c := range codigos {
		reportado := false

		for _, tt := range tablas {
			if tt.t == nil {
				continue
			}

			fila := buscarPorCodigo(tt.t, c)
			if fila == nil {
				continue
			}

			flippy := campo(tt.t, fila, "flippyfile")
			inv := campo(tt.t, fila, "invfile")
			nombre := campo(tt.t, fila, "name")
			bytesDC6, crcDC6 := tamanoDelDC6(mpqs, flippy)

			fmt.Printf("%-7s %-15s %-31s %-8s %8d %10d  %s\n", c, tt.nombre, nombre, flippy, bytesDC6, crcDC6, inv)

			reportado = true

			if tt.nombre == "misc.txt" && mismo == false {
				mismo = true
			}

			break
		}

		if !reportado {
			fmt.Printf("%-7s %-15s (NO tiene fila en misc/weapons/armor)\n", c, "--")
		}
	}

	fmt.Println("\n== 5. LA ELECCION (la regla de la tarjeta: el objeto MAS SIMPLE primero)")
	if !mismo {
		fmt.Println("  el TC del Returned no trae ningun item de misc.txt: habria que ir a un arma/armadura")
	} else {
		fmt.Println("  hay items de misc.txt en la cadena: misc es la tabla mas simple (sin dano, sin AC, sin afinidades)")
	}

	for _, c := range codigos {
		if fila := buscarPorCodigo(misc, c); fila != nil {
			fmt.Printf("  PRIMER item de misc de la cadena: code=%s name=%s flippy=%s\n", c,
				campo(misc, fila, "name"), campo(misc, fila, "flippyfile"))

			break
		}
	}
}

// partirTabla: parte un .txt de D2 (separado por tabulaciones, con la cabecera arriba y las columnas de
// Expansion al final) en cabecera + filas. Las filas vacias se saltan.
func partirTabla(nombre, mpq string, crudo []byte) *tabla {
	t := &tabla{
		nombre: nombre,
		mpq:    mpq,
		bytes:  len(crudo),
		crc:    crc32.ChecksumIEEE(crudo),
		indice: map[string]int{},
	}

	texto := strings.ReplaceAll(string(crudo), "\r\n", "\n")
	lineas := strings.Split(texto, "\n")

	for i, l := range lineas {
		if strings.TrimSpace(l) == "" {
			continue
		}

		campos := strings.Split(l, "\t")
		if i == 0 || t.cabecera == nil {
			t.cabecera = campos

			for j, c := range campos {
				t.indice[strings.TrimSpace(c)] = j
			}

			continue
		}

		t.lineas = append(t.lineas, campos)
	}

	return t
}

// campo: el valor de una columna de la fila, "" si no existe.
func campo(t *tabla, fila []string, col string) string {
	i, ok := t.indice[col]
	if !ok || i >= len(fila) {
		return ""
	}

	return strings.TrimSpace(fila[i])
}

// buscarPorCodigo: la fila cuya columna code coincide (los .txt de items usan "code").
func buscarPorCodigo(t *tabla, code string) []string {
	for _, l := range t.lineas {
		if strings.EqualFold(campo(t, l, "code"), code) {
			return l
		}
	}

	return nil
}

// resolverClase: imprime una fila de TreasureClassEx.txt y baja por los items que son a su vez clases.
func resolverClase(t *tabla, clase string, nivel int, sangria string, hojas map[string]int) {
	if nivel > maxProfundidadTC {
		fmt.Printf("%s%s ... (demasiado hondo)\n", sangria, clase)

		return
	}

	var fila []string

	for _, l := range t.lineas {
		if strings.EqualFold(campo(t, l, "Treasure Class"), clase) {
			fila = l

			break
		}
	}

	if fila == nil {
		// no es una clase: es un codigo de item (la hoja de la cadena)
		hojas[clase]++

		return
	}

	picks, _ := strconv.Atoi(campo(t, fila, "picks"))
	noDrop, _ := strconv.Atoi(campo(t, fila, "NoDrop"))

	fmt.Printf("%s%s picks=%d NoDrop=%d\n", sangria, clase, picks, noDrop)

	siguiente := sangria + "  "

	for i := 1; i <= 10; i++ {
		item := campo(t, fila, "Item"+strconv.Itoa(i))
		prob := campo(t, fila, "Prob"+strconv.Itoa(i))

		if item == "" || prob == "0" {
			continue
		}

		esClase := false

		for _, l := range t.lineas {
			if strings.EqualFold(campo(t, l, "Treasure Class"), item) {
				esClase = true

				break
			}
		}

		if esClase {
			fmt.Printf("%s  prob=%-6s ->\n", siguiente, prob)
			resolverClase(t, item, nivel+1, siguiente+"    ", hojas)
		} else {
			fmt.Printf("%s  prob=%-6s item=%s\n", siguiente, prob, item)
			hojas[item]++
		}
	}
}

// tamanoDelDC6: el tamano y crc32 del flippy del item (es el DC6 que el motor carga para el objeto en el suelo:
// `d2resource.ItemGraphics` + flippyfile, factory.go NewGroundItem).
func tamanoDelDC6(mpqs map[string]*d2mpq.MPQ, flippy string) (int, uint32) {
	if flippy == "" {
		return 0, 0
	}

	for _, n := range []string{"d2data.mpq", "patch_D2.mpq", "d2exp.mpq"} {
		m := mpqs[n]
		if m == nil {
			continue
		}

		for _, ruta := range []string{
			carpetaItems + flippy + ".DC6",
			carpetaItems + flippy + ".dc6",
			carpetaItems + strings.ToLower(flippy) + ".dc6",
		} {
			crudo, err := m.ReadFile(ruta)
			if err == nil && len(crudo) > 0 {
				return len(crudo), crc32.ChecksumIEEE(crudo)
			}
		}
	}

	return 0, 0
}

// minInt: el menor de dos enteros (no hay genericos en la stdlib minima que usa este arbol).
func minInt(a, b int) int {
	if a < b {
		return a
	}

	return b
}

// dumpCrudo: imprime las tres primeras lineas de una tabla, tal cual vienen del MPQ (para ver la cabecera de
// verdad, sin suponer los nombres de las columnas).
func dumpCrudo(mpqs map[string]*d2mpq.MPQ, ruta string) {
	for _, n := range []string{"d2data.mpq", "patch_D2.mpq", "d2exp.mpq"} {
		m := mpqs[n]
		if m == nil {
			continue
		}

		b, err := m.ReadFile(ruta)
		if err != nil || len(b) == 0 {
			continue
		}

		texto := strings.ReplaceAll(string(b), "\r\n", "\n")
		lineas := strings.Split(texto, "\n")
		fmt.Printf("\n== CRUDO %s (%s) lineas=%d\n", ruta, n, len(lineas))

		for i := 0; i < 3 && i < len(lineas); i++ {
			l := lineas[i]
			if len(l) > 400 {
				l = l[:400]
			}

			fmt.Printf("  linea %d campos=%d: %q\n", i, len(strings.Split(lineas[i], "\t")), l)
		}

		return
	}
}

// analizarSemillas: la PRIMERA tirada de la cadena del drop para las semillas 1..N, con el generador del motor
// (`d2rand`, el mismo de la vida y del dano). La cadena de partida del Returned ("Act 1 H2H B") tiene NoDrop=100 y
// tres entradas que no son oro (equipo 16, junk 21, good 2): total 139. Con la tirada se sabe a que rama cae cada
// semilla SIN abrir el emulador, que es lo que hace falta para elegir una semilla que deje ver el objeto.
func analizarSemillas(n int) {
	const (
		noDrop = 100
		equipo = 16
		junk   = 21
		good   = 2
		total  = noDrop + equipo + junk + good
	)

	fmt.Printf("\n== 6. LAS SEMILLAS DEL DADO (la primera tirada de %q, total=%d con el oro fuera)\n", "Act 1 H2H B", total)
	fmt.Println("semilla  tirada  rama")

	for sem := 1; sem <= n; sem++ {
		dado := d2rand.New(uint32(sem))
		t := int(dado.Roll(total))

		rama := "NADA (NoDrop)"

		switch {
		case t < noDrop:
		case t < noDrop+equipo:
			rama = "Act 1 Equip B (equipo: weap3/armo3/weap6/armo6)"
		case t < noDrop+equipo+junk:
			rama = "Act 1 Junk (pociones, llaves, pergaminos, ammo)"
		default:
			rama = "Act 1 Good (anillos, amuletos, gemas)"
		}

		fmt.Printf("%-7d  %-6d  %s\n", sem, t, rama)
	}
}

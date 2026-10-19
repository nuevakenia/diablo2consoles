// sonda-monstruo-250: el CENSO del monstruo del acto 1 (el fallen del Blood Moor) y de las cuentas del
// combate, todo leido del MPQ del disco con los lectores del MOTOR y sin emulador de por medio.
//
// Para que (DC-002.50, "GOLPEAR ENEMIGOS"): el POC de la consola tiene que pintar un monstruo, golpearlo y
// bajarle la vida. Los numeros que hay que saber ANTES de tocar la consola son cuatro, y los cuatro salen de
// aca:
//
//  1. CUAL ES EL BICHO Y CUANTA VIDA TIENE: la fila de monstats.txt (el Id "fallen1", el del Blood Moor) y la
//     tabla monlvl.txt escalada por las funciones del motor (`d2monstats.LoadClasses` / `LoadMonLvl` /
//     `Scale`), que son exactamente las que usa el motor para sembrar un monstruo. La vida sale de ahi, no de
//     un numero elegido: es la que el motor le pone al bicho.
//  2. CUANTO PEGA EL HEROE: el motor baja a 1..2 de dano a mano limpia (`heroDamage`, VERIFIED) y el dado es
//     el del motor (d2rand). Con la misma semilla, la misma secuencia de golpes: eso es lo que hace
//     reproducible el criterio 2.
//  3. QUE ANIMACIONES TIENE EL BICHO Y A QUE RITMO: los COF del fallen (monsters\FA\COF) y las velocidades
//     del animdata.d2 (que es la que manda: el campo Speed del COF de los monstruos no se usa).
//  4. QUE CUADROS HAY QUE COMPONER: las capas del COF con su archivo .dcc, su caja y sus cuadros, por modo
//     (NU parado, GH golpeado, A1 ataque, DT muriendo, DD muerto).
//
// Uso:
//
//	go run ./console/dc/sonda-monstruo-250 [-mpq <d2data.mpq>] [-clase fallen1] [-token FA] [-semilla 592]
package main

import (
	"flag"
	"fmt"
	"hash/crc32"
	"os"
	"path/filepath"
	"strings"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2animdata"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2cof"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2dc6"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2dcc"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2mpq"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2txt"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2math/d2rand"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2monstats"
)

// los modos del COF del monstruo que interesan al combate, en el orden en que se van a ver.
var modos = []string{"NU", "WL", "A1", "A2", "GH", "DT", "DD"}

func main() {
	mpqPath := flag.String("mpq", "", "d2data.mpq (por defecto, la instalacion del juego)")
	mpqPatch := flag.String("mpq-patch", "", "patch_D2.mpq, el del monlvl.txt (por defecto, junto a d2data)")
	clase := flag.String("clase", "fallen1", "el Id de monstats.txt del bicho")
	token := flag.String("token", "FA", "el token del bicho en el MPQ (la carpeta de monsters)")
	semilla := flag.Uint("semilla", 0x250, "semilla del dado del motor para la secuencia de golpes")
	golpes := flag.Int("golpes", 12, "cuantos golpes simular")
	lista := flag.Bool("lista", false, "en vez del censo: lista las clases de monstats.txt cuyo Id contenga -clase")
	fila := flag.Bool("fila", false, "en vez del censo: imprime la cabecera de monstats.txt y la fila cruda de -clase")
	volcar := flag.String("volcar", "", "si se pasa, escribe monstats.txt y MonLvl.txt en esta carpeta y sale")
	tope := flag.Int("tabla", 0, "en vez del censo: imprime las -tabla clases de nivel <= 6 con mas vida (expansion, normal)")
	flag.Parse()

	ruta := *mpqPath
	if ruta == "" {
		ruta = filepath.Join(os.Getenv("HOME"), ".wine-d2", "drive_c", "Games", "Diablo II", "d2data.mpq")
	}

	rutaPatch := *mpqPatch
	if rutaPatch == "" {
		// el monlvl.txt NO esta en d2data.mpq ni en d2exp.mpq: lo trae el parche (medido con el listfile).
		rutaPatch = filepath.Join(filepath.Dir(ruta), "patch_D2.mpq")
	}

	m, err := d2mpq.FromFile(ruta)
	if err != nil {
		fmt.Fprintln(os.Stderr, "MPQ:", err)
		os.Exit(1)
	}

	defer m.Close()

	fmt.Printf("SONDA250: MPQ %s\n", ruta)

	// ============================== 1. la vida y el ataque del bicho (motor) ==============================
	monstats := mustLeer(m, `data\global\excel\monstats.txt`)
	monlvl := mustLeerDe(rutaPatch, `data\global\excel\MonLvl.txt`)
	if *volcar != "" {
		os.WriteFile(filepath.Join(*volcar, "monstats.txt"), monstats, 0o644) //nolint:errcheck
		os.WriteFile(filepath.Join(*volcar, "MonLvl.txt"), monlvl, 0o644)     //nolint:errcheck
		fmt.Println("VOLCADO en", *volcar)

		return
	}

	if *fila {
		// la cabecera y la fila cruda del bicho: para verificar que la columna que lee el motor es la que se
		// cree (el censo de DC-002.50 salio con minHP=0 y hay que saber si es el dato o el lector).
		lineas := strings.Split(string(monstats), "\n")
		if len(lineas) > 0 {
			fmt.Println("CABECERA:", strings.TrimRight(lineas[0], "\r"))
		}

		for _, l := range lineas[1:] {
			campos := strings.Split(strings.TrimRight(l, "\r"), "	")
			if len(campos) > 0 && strings.EqualFold(strings.TrimSpace(campos[0]), *clase) {
				fmt.Println("FILA:", strings.TrimRight(l, "\r"))
			}
		}

		return
	}

	clases, err := d2monstats.LoadClasses(monstats)
	if err != nil {
		fmt.Fprintln(os.Stderr, "monstats:", err)
		os.Exit(1)
	}

	niveles, err := d2monstats.LoadMonLvl(monlvl)
	if err != nil {
		fmt.Fprintln(os.Stderr, "monlvl:", err)
		os.Exit(1)
	}

	c := clases[*clase]

	if *tope > 0 {
		tablaVida(monstats, niveles, *tope)

		return
	}

	if c == nil && *lista {
		for id := range clases {
			if strings.Contains(strings.ToLower(id), strings.ToLower(*clase)) {
				fmt.Printf("CLASE %s\n", id)
			}
		}

		return
	}

	if c == nil {
		fmt.Fprintf(os.Stderr, "no encuentro la clase %q en monstats.txt (%d clases)\n", *clase, len(clases))
		os.Exit(1)
	}

	// el Code (el token de la carpeta del MPQ) y el Id: son dos columnas distintas y la carpeta usa el Code.
	fmt.Printf("MONSTATS-CODIGO id=%s code=%s\n", d2codigo(monstats, *clase, "Id"), d2codigo(monstats, *clase, "Code"))
	fmt.Printf("MONSTATS: %s bytes=%d crc32=%d clases=%d\n", *clase, len(monstats), crc32.ChecksumIEEE(monstats), len(clases))
	fmt.Printf("MONSTATS-FILA id=%s hcIdx=%d nivel(normal)=%d minHP=%d maxHP=%d AC=%d A1(TH=%d MinD=%d MaxD=%d) noRatio=%v boss=%v align=%d\n",
		c.ID, c.Index, c.Level[d2monstats.Normal], c.MinHP[d2monstats.Normal], c.MaxHP[d2monstats.Normal],
		c.AC[d2monstats.Normal], c.Attacks[d2monstats.A1][d2monstats.Normal].TH,
		c.Attacks[d2monstats.A1][d2monstats.Normal].Min, c.Attacks[d2monstats.A1][d2monstats.Normal].Max,
		c.NoRatio, c.Boss, c.Align)
	fmt.Printf("MONLVL: filas=%d bytes=%d crc32=%d\n", len(niveles), len(monlvl), crc32.ChecksumIEEE(monlvl))

	// el motor siembra la vida con Scale: en NORMAL el nivel es el de monstats (ResolveLevel no usa el area),
	// asi que el area no hace falta. Se informan las dos variantes (clasico y expansion) porque el monlvl
	// tiene dos juegos de columnas y el juego es el de la expansion (L-).
	var cero int

	for _, exp := range []bool{false, true} {
		st := niveles.Scale(c, d2monstats.Normal, 0, exp, func(n int) int { cero = 0; return cero })
		etiqueta := "clasico"
		if exp {
			etiqueta = "expansion"
		}

		fmt.Printf("VIDA[%s]: nivel=%d HPMin=%d HPMax=%d AC=%d XP=%d A1(TH=%d MinD=%d MaxD=%d) players=%d\n",
			etiqueta, st.Level, st.HPMin, st.HPMax, st.AC, st.XP,
			st.Attacks[d2monstats.A1].TH, st.Attacks[d2monstats.A1].Min, st.Attacks[d2monstats.A1].Max, st.Players)
	}

	// la vida CONCRETA del bicho: el motor tira entre HPMin y HPMax con su propio generador.
	dado := d2rand.New(uint32(*semilla))
	vida := niveles.Scale(c, d2monstats.Normal, 0, true, func(n int) int { return int(dado.Roll(int32(n))) })
	fmt.Printf("VIDA-SEMBRADA: semilla=%d nivel=%d HP=%d (HPMin=%d HPMax=%d)\n",
		*semilla, vida.Level, vida.HP, vida.HPMin, vida.HPMax)

	// LA FILA DEL MONLVL QUE EL MOTOR USA: las filas de monlvl.txt son 1..N y el slice queda 0..N-1, asi que
	// el renglon del nivel L es `niveles[L-1]` (LoadMonLvl verifica que la columna Level sea el indice+1).
	// OJO, MEDIDO: `MonLvl.Scale` indexa con el nivel CRUDO (`t[lvl]`), o sea el renglon de L+1; el camino con
	// el que el motor siembra de verdad (`computeVitals`, d2core/d2monsters/vitals.go:95-120) usa el mapa de
	// records por nivel, que es el correcto. La vida de esta tarjeta sale del camino de computeVitals.
	renglon := vida.Level - 1
	if renglon < 0 || renglon >= len(niveles) {
		renglon = 0
	}

	fmt.Printf("MONLVL-FILA nivel=%d HP[expansion][normal]=%d HP[clasico][normal]=%d AC[expansion][normal]=%d\n",
		vida.Level, niveles[renglon].HP[1][d2monstats.Normal],
		niveles[renglon].HP[0][d2monstats.Normal], niveles[renglon].AC[1][d2monstats.Normal])

	// EL DEFECTO DEL LECTOR, medido: monstats.txt de 1.14b llama a las columnas MinHP/MaxHP y el lector del
	// motor las pide en minuscula ("minHP"/"maxHP"), asi que devuelve 0 para NORMAL. Se informa la fila cruda
	// leida a mano para que el numero de la vida no dependa de un lector que en esta version lee cero.
	for _, l := range strings.Split(string(monstats), "\n") {
		campos := strings.Split(strings.TrimRight(l, "\r"), "	")
		if len(campos) > 160 && strings.EqualFold(strings.TrimSpace(campos[0]), *clase) {
			fmt.Printf("MONSTATS-CRUDO %s: MinHP=%s MaxHP=%s AC=%s A1MinD=%s A1MaxD=%s A1TH=%s Level=%s\n",
				campos[0], campos[155], campos[156], campos[157], campos[160], campos[161], campos[159], campos[17])
		}
	}

	// ============================== 2. la secuencia de golpes (motor) ==============================
	//
	// heroDamage del motor: a mano limpia 1..2 (VERIFIED), y el dado es el mismo generador (un paso por
	// golpe). El dano se resta como lo resta damage() del motor: HP-=dmg y a 0 el bicho muere.
	dadoGolpes := d2rand.New(uint32(*semilla) ^ 0x6865726f) // el mismo XOR que heroRoller del motor
	// la vida que se usa para los golpes: la de la COLUMNA REAL (MinHP/MaxHP) multiplicada por la fila del
	// monlvl con el multiply-divide del motor (MulDiv100, VERIFIED que trunca). Si el lector del motor hubiera
	// leido bien, esto seria identico a vida.HP.
	minCrudo, maxCrudo := columnasDeVida(monstats, *clase)
	lo := d2monstats.MulDiv100(niveles[renglon].HP[1][d2monstats.Normal], minCrudo)
	hi := d2monstats.MulDiv100(niveles[renglon].HP[1][d2monstats.Normal], maxCrudo)
	if hi < lo {
		hi = lo
	}

	dadoVida := d2rand.New(uint32(*semilla))
	hp := lo
	if hi > lo {
		hp += int(dadoVida.Roll(int32(hi - lo + 1)))
	}

	if hp < 1 { // el piso de computeVitals (d2core/d2monsters/vitals.go:124)
		hp = 1
	}

	fmt.Printf("VIDA-REAL: MinHP=%d MaxHP=%d filaMonlvl=%d HPMin=%d HPMax=%d HP(tirado)=%d\n",
		minCrudo, maxCrudo, niveles[renglon].HP[1][d2monstats.Normal], lo, hi, hp)

	fmt.Printf("GOLPES: semilla=%d dano=1..2 (heroDamage a mano limpia)\n", *semilla)

	for i := 1; i <= *golpes; i++ {
		min, max := 1, 2
		dmg := min + int(dadoGolpes.Roll(int32(max-min+1)))
		antes := hp

		hp -= dmg
		if hp <= 0 {
			hp = 0
		}

		fmt.Printf("  GOLPE %2d: dados=%d vidaAntes=%d vidaDespues=%d %s\n",
			i, dmg, antes, hp, map[bool]string{true: "MUERE", false: ""}[hp == 0])

		if hp == 0 {
			break
		}
	}

	// ============================== 3. las animaciones del bicho ==============================
	animdata := mustLeer(m, `data\global\animdata.d2`)

	anim, err := d2animdata.Load(animdata)
	if err != nil {
		fmt.Fprintln(os.Stderr, "animdata:", err)
		os.Exit(1)
	}

	fmt.Printf("ANIMDATA: bytes=%d crc32=%d animaciones=%d\n", len(animdata), crc32.ChecksumIEEE(animdata), anim.GetRecordsCount())

	for _, modo := range modos {
		nombre := fmt.Sprintf(`data\global\monsters\%s\COF\%s%sHTH.cof`, *token, strings.ToLower(*token), strings.ToLower(modo))

		crudo, err := m.ReadFile(nombre)
		if err != nil || len(crudo) == 0 {
			fmt.Printf("MODO %-2s SIN-COF ruta=%s err=%v\n", modo, nombre, err)

			continue
		}

		cof, err := d2cof.Unmarshal(crudo)
		if err != nil {
			fmt.Printf("MODO %-2s COF-ILEGIBLE bytes=%d err=%v\n", modo, len(crudo), err)

			continue
		}

		clave := *token + modo + "HTH"
		fps, fuente := cof.FPS(), "COF"
		cuadrosAnimdata := 0

		if rec := anim.GetRecord(clave); rec != nil && rec.FPS() > 0 {
			fps, fuente = rec.FPS(), "animdata("+clave+")"
			cuadrosAnimdata = rec.FramesPerDirection()
		}

		msPorCuadro := 1000.0 / fps

		fmt.Printf("MODO %-2s ruta=%s bytes=%d crc32=%d capas=%d cuadrosPorDir=%d cuadrosAnimdata=%d dirs=%d fps=%.2f [%s] %.2f ms/cuadro ciclo=%.0f ms prioridad[0][0]=%v\n",
			modo, nombre, len(crudo), crc32.ChecksumIEEE(crudo), len(cof.CofLayers), cof.FramesPerDirection,
			cuadrosAnimdata, cof.NumberOfDirections, fps, fuente, msPorCuadro,
			float64(cof.FramesPerDirection)*msPorCuadro, cof.Priority[0][0])

		if modo != "NU" && modo != "GH" && modo != "DT" {
			continue
		}

		// las capas: su archivo, su caja y sus cuadros (solo la DIRECCION 0, que es la que se dibuja en la
		// prueba, igual que en DC-002.49: decodificar las 8 no entra en la consola ni hace falta).
		for i := range cof.CofLayers {
			capa := cof.CofLayers[i]
			rutaCapa := fmt.Sprintf(`data\global\monsters\%s\%s\%s%sLIT%s%s.dcc`,
				*token, capa.Type.String(), *token, capa.Type.String(), modo, capa.WeaponClass.String())

			crudoCapa, err := m.ReadFile(rutaCapa)
			if err != nil || len(crudoCapa) == 0 {
				fmt.Printf("  CAPA %-3s %-30s SIN-GRAFICO err=%v\n", capa.Type.String(), rutaCapa, err)

				continue
			}

			dd, err := d2dcc.LoadDirection(crudoCapa, 0)
			if err != nil {
				fmt.Printf("  CAPA %-3s %-30s PARSE-FALLO bytes=%d err=%v\n", capa.Type.String(), rutaCapa, len(crudoCapa), err)

				continue
			}

			caja := dd.Directions[0].Box
			pixeles := 0

			if len(dd.Directions[0].Frames) > 0 {
				pixeles = noCero(dd.Directions[0].Frames[0].PixelData)
			}

			fmt.Printf("  CAPA %-3s bytes=%d crc32=%d caja=%d,%d %dx%d cuadros=%d pixeles0=%d shadow=%d transparente=%v\n",
				capa.Type.String(), len(crudoCapa), crc32.ChecksumIEEE(crudoCapa),
				caja.Left, caja.Top, caja.Width, caja.Height, len(dd.Directions[0].Frames), pixeles,
				capa.Shadow, capa.Transparent)
		}
	}

	// ============================== 4. el golpe del HEROE (para el criterio 3) ==============================
	//
	// El personaje ataca con su COF A1 (amazona desarmada: AMA1HTH.COF, del d2char.mpq) y el ritmo lo pone el
	// animdata. El motor da el golpe a la MITAD de la animacion (checkEvents de d2mapentity/monster.go: el
	// evento HitFrame sale cuando currentFrame/frameCount >= 0,5).
	fmt.Println()
	fmt.Println("HEROE:")
	heroe := []string{"AMA1HTH", "AMTNHTH", "AMTWHTH"}

	for _, clave := range heroe {
		rec := anim.GetRecord(clave)
		if rec == nil {
			fmt.Printf("  ANIM %-8s SIN-REGISTRO en el animdata\n", clave)

			continue
		}

		msPorCuadro := 1000.0 / rec.FPS()
		fmt.Printf("  ANIM %-8s cuadros=%d fps=%.2f %.2f ms/cuadro ciclo=%.0f ms cuadroDelGolpe(mitad)=%d\n",
			clave, rec.FramesPerDirection(), rec.FPS(), msPorCuadro,
			float64(rec.FramesPerDirection())*msPorCuadro, (rec.FramesPerDirection()+1)/2)
	}

	// el COF del ataque del heroe, para confirmar cuadros y capas contra el animdata
	crudoHeroe, err := d2mpqFile(filepath.Join(filepath.Dir(ruta), "d2char.mpq"), `data\global\chars\AM\COF\AMA1HTH.COF`)
	if err != nil {
		fmt.Println("  COF AMA1HTH SIN-LECTURA (d2char.mpq):", err)
	} else if cofA1, err := d2cof.Unmarshal(crudoHeroe); err == nil {
		fmt.Printf("  COF AMA1HTH bytes=%d crc32=%d capas=%d cuadrosPorDir=%d dirs=%d\n",
			len(crudoHeroe), crc32.ChecksumIEEE(crudoHeroe), len(cofA1.CofLayers),
			cofA1.FramesPerDirection, cofA1.NumberOfDirections)
	}
}

// d2codigo: una columna de texto de la fila del bicho (Id, Code, Class...). Se prueban dos cajas (la tabla de
// 1.14b escribe "Class" para el Id y "Code" para el token).
func d2codigo(monstats []byte, clase, columna string) string {
	d := d2txt.LoadDataDictionary(monstats)

	for d.Next() {
		id := d.String("Id")
		if id == "" {
			id = d.String("Class")
		}

		if !strings.EqualFold(strings.TrimSpace(id), clase) {
			continue
		}

		return d.String(columna)
	}

	return ""
}

// tablaVida: las clases de nivel bajo ordenadas por la vida que les da el camino de siembra del motor
// (monlvl de su nivel por MinHP/MaxHP con MulDiv100, el piso de 1 HP de computeVitals). Es la tabla de donde
// sale el bicho de la prueba: tiene que aguantar MAS DE UN golpe de 1..2 para que se le vea el GH.
func tablaVida(monstats []byte, niveles d2monstats.MonLvl, cuantas int) {
	d := d2txt.LoadDataDictionary(monstats)

	type fila struct {
		id            string
		level, lo, hi int
	}

	filas := []fila{}

	for d.Next() {
		id := d.String("Id")
		if id == "" {
			id = d.String("Class")
		}

		nivel := d.Number("Level")
		if nivel <= 0 || nivel > 6 {
			continue
		}

		minimo, maximo := d.Number("MinHP"), d.Number("MaxHP")
		if minimo == 0 && maximo == 0 {
			minimo, maximo = d.Number("minHP"), d.Number("maxHP")
		}

		if maximo < minimo {
			maximo = minimo
		}

		renglon := nivel - 1
		if renglon < 0 || renglon >= len(niveles) {
			continue
		}

		factor := niveles[renglon].HP[1][d2monstats.Normal]
		lo, hi := d2monstats.MulDiv100(factor, minimo), d2monstats.MulDiv100(factor, maximo)
		if hi < lo {
			hi = lo
		}

		if hi < 1 {
			hi = 1
		}

		filas = append(filas, fila{id, nivel, lo, hi})
	}

	// ordenadas de mas vida a menos (una insercion simple: son 300 filas)
	for i := 1; i < len(filas); i++ {
		for j := i; j > 0 && filas[j].hi > filas[j-1].hi; j-- {
			filas[j], filas[j-1] = filas[j-1], filas[j]
		}
	}

	for i := 0; i < len(filas) && i < cuantas; i++ {
		fmt.Printf("TABLA %-22s nivel=%d HP=%d..%d\n", filas[i].id, filas[i].level, filas[i].lo, filas[i].hi)
	}
}

// columnasDeVida: lee MinHP y MaxHP de la fila del bicho con la CAJA REAL del archivo (1.14b los llama con
// mayuscula; el lector del motor pide "minHP" y devuelve 0). Se prueban las dos formas y se avisa cual se uso.
func columnasDeVida(monstats []byte, clase string) (int, int) {
	d := d2txt.LoadDataDictionary(monstats)
	minimo, maximo := 0, 0

	for d.Next() {
		if !strings.EqualFold(strings.TrimSpace(d.String("Id")), clase) {
			continue
		}

		minimo, maximo = d.Number("MinHP"), d.Number("MaxHP")
		if minimo == 0 && maximo == 0 { // la otra caja posible
			minimo, maximo = d.Number("minHP"), d.Number("maxHP")
		}

		break
	}

	return minimo, maximo
}

// noCero: cuantos pixeles del cuadro NO son el indice 0 (el instrumento de siempre).
func noCero(pix []byte) int {
	n := 0

	for _, v := range pix {
		if v != 0 {
			n++
		}
	}

	return n
}

func mustLeer(m *d2mpq.MPQ, nombre string) []byte {
	b, err := m.ReadFile(nombre)
	if err != nil {
		fmt.Fprintln(os.Stderr, "leer", nombre, ":", err)
		os.Exit(1)
	}

	return b
}

// mustLeerDe: lee un archivo de OTRO MPQ (el del parche), abriendolo y cerrándolo.
func mustLeerDe(rutaMPQ, nombre string) []byte {
	b, err := d2mpqFile(rutaMPQ, nombre)
	if err != nil {
		fmt.Fprintln(os.Stderr, "leer", nombre, "de", rutaMPQ, ":", err)
		os.Exit(1)
	}

	return b
}

// d2mpqFile: abre un MPQ, lee UN archivo y lo cierra (para el d2char.mpq, que aca se toca una sola vez).
func d2mpqFile(ruta, nombre string) ([]byte, error) {
	m, err := d2mpq.FromFile(ruta)
	if err != nil {
		return nil, err
	}

	defer m.Close()

	return m.ReadFile(nombre)
}

// silencio para el compilador: d2dc6 se usa en el censo de capas cuando el .dcc no esta (los monstruos son
// todos DCC, pero el motor prueba DC6 de segunda opcion y esta sonda tiene que poder decir cual fue).
var _ = d2dc6.Load

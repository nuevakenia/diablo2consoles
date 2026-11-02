// DC-002.128 -- SONDA DE LA DEN OF EVIL, EN EL HOST: lo que el plan exige medir ANTES de portar (el presupuesto
// de `d2common/d2quest`, el tercer paquete mas grande del plan) y la secuencia de eventos que la quest pide,
// corrida DE VERDAD sobre el paquete del motor (no una lista escrita a mano).
//
// POR QUE EXISTE. La tarjeta (E7) pide la primera quest del Acto 1: aceptarla hablando con Akara, matar los bichos
// de la cueva con el contador avanzando, completarla y cobrar la recompensa, y que sobreviva al cambio de nivel.
// Para no inventar ni un numero, esta sonda contesta:
//
//  1. EL PRESUPUESTO: lineas y bytes de `d2common/d2quest` y `d2fileformats/d2s`, y el CIERRE de imports
//     (`go list -deps`, el mismo que usa el build para armar el enlace). Si el cierre trae ebiten o d2asset, el
//     paquete NO se porta; si no, se porta tal cual (como `d2npc` en DC-002.127).
//  2. EL ARRANQUE: cuantas quests arma `d2quest.New` (el costo que se paga en la consola al arrancar) y cuanto
//     ocupa el registro de la quest en el guardado (96 B por dificultad, `d2s.QuestRecord`).
//  3. LA SECUENCIA, CORRIDA: se crea el juego de la quest (`d2quest.New` + `Start`), se le dan los MISMOS eventos
//     que el POC le va a dar (hablar con Akara, cambiar de area a la cueva, matar los bichos, volver, cobrar) y se
//     imprime, paso por paso, el estado, los bits del slot, la pagina del log, los efectos y los cambios. Eso es
//     la lista de eventos que el POC tiene que emitir y lo que tiene que decir el log de la consola.
//  4. LA SUPERVIVENCIA: con el registro ya escrito, se crea un juego NUEVO sobre el MISMO registro y se llama
//     `Start` (lo que hace el motor al cargar la partida / entrar al juego): la quest tiene que quedar inerte y el
//     log decir COMPLETADA. Es la prueba de que el estado vive en el guardado, no en el nivel.
//  5. LA CELDA DE AKARA en el `.ds1` del pueblo que compone el POC (townN1) y el texto de la quest en
//     `string.tbl` (`qstsa1q1` y sus paginas), que es lo que el panel del POC va a mostrar.
//
// Uso:
//
//	go run ./console/dc/sonda-quest-128
//	go run ./console/dc/sonda-quest-128 -dir "~/.wine-d2/drive_c/Games/Diablo II"
package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2ds1"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2mpq"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2s"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2quest"
)

const (
	tablaMonPreset = `data\global\excel\MonPreset.txt`
	tablaMonStats  = `data\global\excel\monstats.txt`
	tablaStrings   = `data\local\lng\ENG\string.tbl`

	// el pueblo que el POC compone al arrancar (preset 1 / archivo 0)
	rutaDS1PuebloN = `data\global\tiles\ACT1\TOWN\townN1.ds1`

	// los numeros de la Den of Evil, tal como estan en el motor
	npcAkara          = d2quest.NPCAkara
	msgAceptaLaQuest  = 64 // Akara entrega la quest (a1_basic.go: "Akara hands out the quest")
	msgCobraLaReward  = 76 // Akara agradece y se cobra (a1_basic.go: "Akara's thanks: claim")
	bichosDeLaCueva   = 3  // los bichos que el POC pone en la cueva (palanca MONSTRUOS)
	nivelDeLaCueva    = d2quest.LevelDenOfEvil
	nivelDelPueblo    = d2quest.LevelRogueEncampment
	claseDelReturned  = 6 // el bicho del POC es el Returned (monstats.txt)
	questDenOfEvil    = d2quest.QuestDenOfEvil
	celdaDelPOCX      = 28.0
	celdaDelPOCY      = 20.0
	paginasAQuesear   = 8
	clavesDeLaQuestA1 = "qstsa1q1"
)

var paquetes = []string{
	"d2common/d2quest",
	"d2common/d2fileformats/d2s",
}

func main() {
	dir := flag.String("dir", "", "carpeta del juego (por omision, la instalacion de wine)")
	flag.Parse()

	base := *dir
	if base == "" {
		home, _ := os.UserHomeDir()
		base = filepath.Join(home, ".wine-d2", "drive_c", "Games", "Diablo II")
	}

	elPresupuesto()
	laSecuencia()

	mpqs := map[string]*d2mpq.MPQ{}

	for _, nombre := range []string{"patch_d2.mpq", "d2data.mpq"} {
		m, err := d2mpq.FromFile(filepath.Join(base, nombre))
		if err != nil {
			fmt.Printf("   %-14s NO ABRE: %v\n", nombre, err)

			continue
		}

		mpqs[nombre] = m
		fmt.Printf("   %-14s %12d B\n", nombre, m.Size())
	}

	laCeldaDeAkara(mpqs)
	elTextoDeLaQuest(mpqs)
}

// ============================== 1. EL PRESUPUESTO ==============================

func elPresupuesto() {
	fmt.Println("== 1. EL PRESUPUESTO: que cuesta portar d2quest (go list -deps, el cierre del enlace)")

	for _, p := range paquetes {
		archivos, lineas, bytes := medirPaquete(p)
		deps := cierreDeImports(p)
		ebiten, assets := enElCierre(deps)

		fmt.Printf("   %-40s %2d archivos %6d lineas %8d B | cierre %3d paquetes ebiten=%-5v d2asset=%v\n",
			p, archivos, lineas, bytes, len(deps), ebiten, assets)
	}

	fmt.Println("   -- los paquetes del motor que arrastra d2quest:")

	for _, d := range filtrarDeps(cierreDeImports("d2common/d2quest")) {
		fmt.Println("      " + d)
	}
}

func medirPaquete(p string) (archivos, lineas, bytes int) {
	// el cwd de `go run ./console/dc/sonda-quest-128` es la raiz del modulo; `../../` es por si se corre desde la
	// carpeta de la sonda. Se prueban las dos y se toma la que exista (el primer intento daba 0 archivos en silencio).
	dir := p
	if _, err := os.ReadDir(dir); err != nil {
		dir = "../../" + p
	}

	entradas, err := os.ReadDir(dir)
	if err != nil {
		return 0, 0, 0
	}

	for _, e := range entradas {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}

		b, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}

		archivos++
		bytes += len(b)
		lineas += strings.Count(string(b), "\n")
	}

	return archivos, lineas, bytes
}

func cierreDeImports(p string) []string {
	salida, err := exec.Command("go", "list", "-deps", "./"+p).Output()
	if err != nil {
		return nil
	}

	var deps []string

	for _, l := range strings.Split(string(salida), "\n") {
		if l = strings.TrimSpace(l); l != "" {
			deps = append(deps, l)
		}
	}

	return deps
}

func enElCierre(deps []string) (ebiten, assets bool) {
	for _, d := range deps {
		if strings.Contains(d, "hajimehoshi/ebiten") {
			ebiten = true
		}

		if strings.HasSuffix(d, "d2core/d2asset") || strings.HasSuffix(d, "d2core/d2records") {
			assets = true
		}
	}

	return ebiten, assets
}

func filtrarDeps(deps []string) []string {
	var out []string

	for _, d := range deps {
		if strings.Contains(d, "OpenDiablo2") {
			out = append(out, d)
		}
	}

	sort.Strings(out)

	return out
}

// ============================== 2. LA SECUENCIA, CORRIDA ==============================

func laSecuencia() {
	fmt.Println("\n== 2. LA DEN OF EVIL CORRIDA SOBRE d2quest (los eventos que el POC le va a dar)")

	var rec d2s.QuestRecord

	g := d2quest.New(&rec, nil, 0)
	g.Hero = d2quest.Hero{Class: 0, Level: 1} // la amazona del POC (clase 0 del exe)
	g.Level = nivelDelPueblo
	g.Trace = func(s string) { fmt.Println("      trace:", s) }
	g.Start()

	q := g.Quest(questDenOfEvil)
	fmt.Printf("   quest=%s id=%d slot=%d act=%d logIndex=%d slotBytes=%d registroBytes=%d quests=%d\n",
		q.Name, q.ID, q.Slot, q.Act, q.LogIndex, len(g.Rec), len(rec), len(g.Quests))
	fmt.Println("   " + g.Describe(q))

	paso := func(titulo string, efectos []d2quest.Effect) {
		fmt.Printf("   %-34s %s\n", titulo+" ->", g.Describe(q))
		printEfectos(efectos)
		printCambios(g.TakeChanges())
		printLog(g)
	}

	// 1. HABLAR CON AKARA CON LA QUEST SIN ACEPTAR: el motor arma la lista de lo que dice
	d := g.Activate(npcAkara)
	fmt.Printf("   Activate(Akara) lineas=%d (la tabla de lineas vive en messages.csv, que en la consola NO viaja:\n"+
		"   parche de port DC-002.8; el estado igual avanza por el id del mensaje)\n", len(d.Lines))
	fmt.Printf("   Marker(Akara)=%v  (el \"!\" sobre la cabeza: la quest esta disponible)\n", g.Marker(npcAkara))

	// 2. ACEPTAR: el mensaje 64 de Akara
	paso("Hear(Akara, 64)", g.Hear(npcAkara, msgAceptaLaQuest))
	g.Close(npcAkara)
	fmt.Printf("   Close(Akara) -> el ciclo de paginas deja la quest en last=%d page=%d\n", q.LastState, g.LogPage(q))

	// 3. LA CUEVA: cambiar de area (pueblo -> Den of Evil, id 8)
	paso(fmt.Sprintf("EvAreaChanged %d -> %d", nivelDelPueblo, nivelDeLaCueva),
		g.Dispatch(d2quest.Event{Kind: d2quest.EvAreaChanged, OldLevel: nivelDelPueblo, NewLevel: nivelDeLaCueva}))

	// 4. LOS BICHOS: el motor cuenta las bajas de ESA area. `SyncDenMonsters` es el camino del motor para que la
	//    quest no dependa de cuantos bichos genero el nivel (ver a1_basic.go).
	g.SetDenMonsters(bichosDeLaCueva)
	fmt.Printf("   SetDenMonsters(%d) -> quedan %d\n", bichosDeLaCueva, g.DenMonstersLeft())

	for i := 1; i <= bichosDeLaCueva; i++ {
		vivos := bichosDeLaCueva - i
		g.SyncDenMonsters(vivos)

		paso(fmt.Sprintf("EvMonsterKilled %d/%d (vivos=%d)", i, bichosDeLaCueva, vivos),
			g.Dispatch(d2quest.Event{Kind: d2quest.EvMonsterKilled, Level: nivelDeLaCueva, Monster: claseDelReturned}))
	}

	paso("Tick(8) -- el motor pasa la pagina", g.Tick(8))

	// 5. VOLVER AL PUEBLO Y COBRAR
	paso(fmt.Sprintf("EvAreaChanged %d -> %d", nivelDeLaCueva, nivelDelPueblo),
		g.Dispatch(d2quest.Event{Kind: d2quest.EvAreaChanged, OldLevel: nivelDeLaCueva, NewLevel: nivelDelPueblo}))

	fmt.Printf("   Marker(Akara)=%v (hay recompensa pendiente: el \"!\" esta)\n", g.Marker(npcAkara))
	paso("Hear(Akara, 76)", g.Hear(npcAkara, msgCobraLaReward))
	g.Close(npcAkara)

	fmt.Printf("   REGISTRO DEL GUARDADO slot=%d bits=0x%04x (%d B)\n", q.Slot, g.Rec.Slot(q.Slot), len(rec))

	// 6. LA SUPERVIVENCIA: un juego NUEVO sobre el MISMO registro (lo que hace el motor al cargar la partida)
	fmt.Println("   -- LA SUPERVIVENCIA: d2quest.New sobre el MISMO registro + Start (el camino de cargar la partida)")

	var copia d2s.QuestRecord
	copy(copia[:], rec[:])

	nuevo := d2quest.New(&copia, nil, 0)
	nuevo.Hero = d2quest.Hero{Class: 0, Level: 1}
	nuevo.Level = nivelDelPueblo
	nuevo.Start()

	q2 := nuevo.Quest(questDenOfEvil)
	fmt.Printf("   tras Start: %s\n", nuevo.Describe(q2))
	fmt.Printf("   inerte=%v (NotIntro=false y Active=false: la quest no vuelve a empezar)  log=%s\n",
		!q2.NotIntro && !q2.Active, estadoDelLog(nuevo, questDenOfEvil))
	fmt.Printf("   Marker(Akara)=%v (ya no hay \"!\")\n", nuevo.Marker(npcAkara))
}

func printEfectos(efectos []d2quest.Effect) {
	if len(efectos) == 0 {
		return
	}

	var out []string

	for _, e := range efectos {
		out = append(out, fmt.Sprintf("%s(quest=%d value=%d %s)", nombreDelEfecto(e.Kind), e.Quest, e.Value, e.Code))
	}

	fmt.Println("      efectos:", strings.Join(out, " "))
}

func nombreDelEfecto(k d2quest.EffectKind) string {
	nombres := []string{"LogUpdate", "SkillPoint", "HireRogues", "Imbue", "GiveItem", "DeleteItem", "Sound",
		"Respec", "Portal", "Bark", "Spawn", "UnlockAct", "Reward"}

	if int(k) < len(nombres) {
		return nombres[k]
	}

	return "?"
}

func printCambios(cambios []d2quest.Change) {
	for _, c := range cambios {
		fmt.Printf("      bit quest=%d slot=%d 0x%04x -> 0x%04x (%s) estado=%d\n",
			c.Quest, c.Slot, c.Before, c.After, c.Reason, c.StateAfter)
	}
}

func printLog(g *d2quest.Game) {
	for _, l := range g.Log() {
		if l.Quest != questDenOfEvil {
			continue
		}

		fmt.Printf("      log act=%d index=%d estado=%s pagina=%d sinVer=%v\n",
			l.Act, l.Index, nombreDelEstado(l.Status), l.Page, l.Unseen)
	}
}

func nombreDelEstado(s d2quest.LogStatus) string {
	switch s {
	case d2quest.LogNotStarted:
		return "sin-empezar"
	case d2quest.LogInProgress:
		return "en-curso"
	case d2quest.LogCompleting:
		return "por-cobrar"
	case d2quest.LogCompleted:
		return "COMPLETADA"
	}

	return "?"
}

func estadoDelLog(g *d2quest.Game, id int) string {
	for _, l := range g.Log() {
		if l.Quest == id {
			return nombreDelEstado(l.Status)
		}
	}

	return "?"
}

// ============================== 3. LA CELDA DE AKARA ==============================

func laCeldaDeAkara(mpqs map[string]*d2mpq.MPQ) {
	fmt.Println("\n== 3. LA CELDA DE AKARA en el .ds1 del pueblo que el POC compone (townN1)")

	presets := leerMonPreset(mpqs)
	stats := leerMonStats(mpqs)

	datos := leerDeLosMPQ(mpqs, rutaDS1PuebloN)
	if datos == nil {
		fmt.Println("   NO encuentro", rutaDS1PuebloN)

		return
	}

	ds, err := d2ds1.Unmarshal(datos)
	if err != nil {
		fmt.Printf("   %s NO PARSEA: %v\n", rutaDS1PuebloN, err)

		return
	}

	fmt.Printf("   %s %d B %dx%d objetos=%d act=%d\n", filepath.Base(rutaDS1PuebloN), len(datos),
		ds.Width(), ds.Height(), len(ds.Objects), ds.Act)

	for i := range ds.Objects {
		o := &ds.Objects[i]
		if o.Type != 1 {
			continue
		}

		place := ""

		if act, ok := presets[int(ds.Act)]; ok && o.ID >= 0 && o.ID < len(act) {
			place = act[o.ID]
		}

		fila, ok := stats[place]
		if !ok {
			continue
		}

		if fila[0] != "akara" && fila[1] != "PS" {
			continue
		}

		fmt.Printf("   AKARA obj=%d preset=%d place=%s id=%s code=%s celda=%d,%d (el .ds1 guarda subtiles; la celda del POC es /5)\n",
			i, o.ID, place, fila[0], fila[1], o.X, o.Y)
		fmt.Printf("   -> CELDA_P1=\"%.1f %.1f\" pone al POC en la celda de Akara (y VISTA_X/VISTA_Y lo mismo, para que el panel la tenga a mano)\n",
			float64(o.X)/5, float64(o.Y)/5)
	}
}

func leerMonPreset(mpqs map[string]*d2mpq.MPQ) map[int][]string {
	actos := map[int][]string{}

	datos := leerDeLosMPQ(mpqs, tablaMonPreset)
	if datos == nil {
		fmt.Println("   NO encuentro", tablaMonPreset)

		return actos
	}

	lineas := strings.Split(string(datos), "\n")
	if len(lineas) < 2 {
		return actos
	}

	cAct, cPlace := indiceDeColumna(lineas[0], "act"), indiceDeColumna(lineas[0], "place")

	for _, l := range lineas[1:] {
		l = strings.TrimRight(l, "\r")
		if strings.TrimSpace(l) == "" {
			continue
		}

		act, err := strconv.Atoi(strings.TrimSpace(campo(l, cAct)))
		if err != nil {
			continue
		}

		actos[act] = append(actos[act], strings.TrimSpace(campo(l, cPlace)))
	}

	return actos
}

func leerMonStats(mpqs map[string]*d2mpq.MPQ) map[string][4]string {
	filas := map[string][4]string{}

	datos := leerDeLosMPQ(mpqs, tablaMonStats)
	if datos == nil {
		return filas
	}

	lineas := strings.Split(string(datos), "\n")
	if len(lineas) < 2 {
		return filas
	}

	cID := indiceDeColumna(lineas[0], "id")
	cCode := indiceDeColumna(lineas[0], "code")
	cName := indiceDeColumna(lineas[0], "namestr")

	for _, l := range lineas[1:] {
		l = strings.TrimRight(l, "\r")
		if strings.TrimSpace(l) == "" {
			continue
		}

		id := strings.TrimSpace(campo(l, cID))
		if id == "" {
			continue
		}

		filas[id] = [4]string{id, strings.TrimSpace(campo(l, cCode)), strings.TrimSpace(campo(l, cName)), ""}
	}

	return filas
}

// ============================== 4. EL TEXTO DE LA QUEST ==============================

func elTextoDeLaQuest(mpqs map[string]*d2mpq.MPQ) {
	fmt.Println("\n== 4. EL TEXTO DE LA DEN OF EVIL en string.tbl (lo que el panel del POC muestra)")

	datos := leerDeLosMPQ(mpqs, tablaStrings)
	if datos == nil {
		fmt.Println("   NO encuentro", tablaStrings)

		return
	}

	fmt.Printf("   string.tbl %d B, %d claves\n", len(datos), int(leerU16(datos, 2)))

	for pagina := 0; pagina <= paginasAQuesear; pagina++ {
		clave := clavesDeLaQuestA1 + strconv.Itoa(pagina)

		texto := buscarEnStringTbl(datos, clave)
		if texto == "" {
			continue
		}

		fmt.Printf("   %-10s (%3d B) %s\n", clave, len(texto), recorte(texto, 150))
	}
}

// buscarEnStringTbl: el MISMO lector de `string.tbl` que el POC (npc_127.go, `textoDeStringTbl`): crc u16,
// numero de elementos u16, tamano de la tabla u32, version u8, tres u32, la lista de indices (u16 por elemento) y
// despues las entradas de 17 bytes: activa u8, indice u16, hash u32, offset del TEXTO u32, offset de la CLAVE u32,
// largo de la clave u16. La CLAVE es la cadena terminada en cero de `indexString`; el TEXTO mide `nameLength-1`.
func buscarEnStringTbl(tabla []byte, clave string) string {
	if len(tabla) < 21 {
		return ""
	}

	n := int(leerU16(tabla, 2))
	entradas := int(leerU32(tabla, 4))
	base := 21 + 2*n

	for i := 0; i < entradas; i++ {
		e := base + 17*i
		if e+17 > len(tabla) || tabla[e] == 0 {
			continue
		}

		largoClave := int(leerU16(tabla, e+15))
		if largoClave == 0 {
			continue
		}

		offClave := int(leerU32(tabla, e+7))
		if offClave >= len(tabla) {
			continue
		}

		fin := offClave
		for fin < len(tabla) && tabla[fin] != 0 {
			fin++
		}

		if string(tabla[offClave:fin]) != clave {
			continue
		}

		offTexto := int(leerU32(tabla, e+11))
		largo := largoClave - 1
		if largo < 0 || offTexto+largo > len(tabla) {
			return ""
		}

		return string(tabla[offTexto : offTexto+largo])
	}

	return ""
}

func leerU16(b []byte, o int) uint16 {
	if o+2 > len(b) {
		return 0
	}

	return uint16(b[o]) | uint16(b[o+1])<<8
}

func leerU32(b []byte, o int) uint32 {
	if o+4 > len(b) {
		return 0
	}

	return uint32(b[o]) | uint32(b[o+1])<<8 | uint32(b[o+2])<<16 | uint32(b[o+3])<<24
}

// ============================== ayudas ==============================

func leerDeLosMPQ(mpqs map[string]*d2mpq.MPQ, ruta string) []byte {
	for _, nombre := range []string{"patch_d2.mpq", "d2data.mpq"} {
		m := mpqs[nombre]
		if m == nil {
			continue
		}

		datos, err := m.ReadFile(ruta)
		if err == nil && len(datos) > 0 {
			return datos
		}
	}

	return nil
}

func indiceDeColumna(cabecera, nombre string) int {
	campos := strings.Split(strings.TrimRight(cabecera, "\r"), "\t")

	for i, c := range campos {
		if strings.EqualFold(strings.TrimSpace(c), nombre) {
			return i
		}
	}

	return -1
}

func campo(linea string, i int) string {
	campos := strings.Split(linea, "\t")
	if i < 0 || i >= len(campos) {
		return ""
	}

	return campos[i]
}

func recorte(s string, n int) string {
	s = strings.Join(strings.Fields(s), " ")
	if len(s) <= n {
		return s
	}

	return s[:n] + "..."
}

// DC-002.146b -- POR QUE EL AUTOMAPA DEL DRLG SE LLENA SOLO EN PARTE, MEDIDO EN EL HOST.
//
// QUE HACE. Corre el mismo DRLG del acto 1 que la consola (`drlgworld.Generate` + `drlgoutdoor.Generate` +
// `Level.BuildTiles`), toma una REGION del Blood Moor con las mismas coordenadas que compone la consola, y
// reproduce la cadena del minimapa SIN la consola:
//
//	registro del DRLG -> `.dt1` (estilo, secuencia, tipo) -> el `TileRef` que arma el adaptador
//	(`console/dc/poc/juego-131/automap_130.go`, `fuenteDelMotor.Tiles`) -> `Table.Cel(nivel, tipo, estilo, sec)`
//
// y cuenta, casilla por casilla, cuantas entran en el modelo (`d2automap.Model`, el MISMO modelo del motor) y
// cuales NO, con la `(tipo, estilo, secuencia)` de cada una que se queda afuera.
//
// Lo que NO hace: no compone el `MapEngine` (no hace falta: el adaptador saca el `TileRef` del registro del
// `.dt1`, y el `ExactTile` resuelve al mismo `.dt1`; `Prop1` queda en 1 y `Hidden` en 0 en todos, que es lo que
// deja `exactComponent`). Lo que si usa tal cual es la tabla (`d2automap.ParseTxtLevels`) y el modelo
// (`d2automap.Model.RevealTiles`), o sea los numeros de celdas son los del motor.
//
// Uso:
//
//	go run ./console/dc/medidas/automap-drlg -seed 1 -regiones 24,32:32 " <carpeta de Diablo II>"
//	go run ./console/dc/medidas/automap-drlg -seed 1 -regiones 0,48:16,24,32:32 "<dir>"
package main

import (
	"flag"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2automap"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2drlg"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2drlg/drlgoutdoor"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2drlg/drlgworld"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2dt1"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2mpq"
)

const (
	nivel             = 2 // el Blood Moor en Levels.txt -> LvlTypes 2 = "1 Wilderness"
	casillasPorCuarto = 8
)

var origenX, origenY int

// region: la ventana del nivel que se mide (las mismas coordenadas que el plan de la consola).
type region struct {
	lado, x, y int
}

func main() {
	seed := flag.Uint("seed", 1, "semilla del juego")
	regionesRaw := flag.String("regiones", "24,32:32;0,48:16", "regiones a medir, como `x,y:lado` separadas por `;`")
	flag.Parse()

	dir := flag.Arg(0)
	if dir == "" {
		dir = os.Getenv("D2_DIR")
	}

	if dir == "" {
		fmt.Println("falta la carpeta de Diablo II (argumento o D2_DIR)")
		os.Exit(2)
	}

	regiones, err := parsearRegiones(*regionesRaw)
	if err != nil {
		fmt.Println("regiones:", err)
		os.Exit(2)
	}

	leer := abrirMPQs(dir)

	tb, err := tablas(leer)
	if err != nil {
		fmt.Println("tablas:", err)
		os.Exit(1)
	}

	// ---- 1. LA TABLA DEL AUTOMAPA, FILTRADA AL NIVEL (el mismo parser del motor)
	crudo, err := leer("data/global/excel/AutoMap.txt")
	if err != nil {
		fmt.Println("AutoMap.txt:", err)
		os.Exit(1)
	}

	tabla, err := d2automap.ParseTxtLevels(strings.NewReader(string(crudo)), nivel)
	if err != nil {
		fmt.Println("parseo:", err)
		os.Exit(1)
	}

	fmt.Printf("TABLA archivo= %d B nivel= %d nombre= %q filas= %d\n",
		len(crudo), nivel, d2automap.LevelName(nivel), len(tabla.LevelRows(nivel)))

	imprimirLasFilas(tabla.LevelRows(nivel))

	// ---- 2. EL NIVEL, GENERADO POR EL DRLG (lo mismo que mide moor-drlg)
	lay, err := drlgworld.Generate(tb, uint32(*seed), d2drlg.Difficulty(0))
	if err != nil {
		fmt.Println("mundo:", err)
		os.Exit(1)
	}

	p, err := drlgoutdoor.ParamsFromLayout(tb, lay, nivel, uint32(*seed))
	if err != nil {
		fmt.Println("params:", err)
		os.Exit(1)
	}

	origenX, origenY = p.Rect.X, p.Rect.Y

	env := drlgoutdoor.NewEnv(tb, func(file string) ([]byte, error) {
		return leer("data/global/tiles/" + file)
	})

	lv, err := drlgoutdoor.Generate(env, p)
	if err != nil {
		fmt.Println("drlg:", err)
		os.Exit(1)
	}

	tiles, err := lv.BuildTiles()
	if err != nil {
		fmt.Println("BuildTiles:", err)
		os.Exit(1)
	}

	fmt.Printf("NIVEL id= %d nombre= %q rect= %d,%d %dx%d casillas= %d\n",
		nivel, d2automap.LevelName(nivel), p.Rect.X, p.Rect.Y, p.Rect.W, p.Rect.H, p.Rect.W*p.Rect.H)

	// ---- 3. LAS REGIONES
	for _, rg := range regiones {
		medirRegion(tabla, tiles, rg, uint32(*seed), leer)
	}
}

// ============================== LA MEDICION DE UNA REGION ==============================

// ref: el `TileRef` del adaptador, ya resuelto a los numeros del `.dt1`.
type ref struct {
	capa      d2automap.Layer
	tipo      int
	estilo    int
	secuencia int
	pieza     string
	indice    int
}

// celdaDeLaRegion: los registros de una casilla de la region, en el ORDEN del adaptador (pisos y despues
// paredes; las sombras no las mira).
type celdaDeLaRegion struct {
	pisos, paredes []ref
}

// fuenteDeLaRegion: el `TileSource` del adaptador (`fuenteDelMotor`), sobre las casillas de la region.
type fuenteDeLaRegion struct {
	lado   int
	celdas map[[2]int]*celdaDeLaRegion
}

func (f fuenteDeLaRegion) Size() (w, h int) { return f.lado, f.lado }

func (f fuenteDeLaRegion) Tiles(tx, ty int) []d2automap.TileRef {
	c := f.celdas[[2]int{tx, ty}]
	if c == nil {
		return nil
	}

	var out []d2automap.TileRef

	for _, r := range c.pisos {
		out = append(out, d2automap.TileRef{Layer: r.capa, Type: r.tipo, Style: r.estilo, Sequence: r.secuencia})
	}

	for _, r := range c.paredes {
		out = append(out, d2automap.TileRef{Layer: r.capa, Type: r.tipo, Style: r.estilo, Sequence: r.secuencia})
	}

	return out
}

// claveSinCelda: una `(capa, tipo, estilo, secuencia)` que no encuentra fila en la tabla del nivel.
type claveSinCelda struct {
	capa      d2automap.Layer
	tipo      int
	estilo    int
	secuencia int
}

// registro: un registro del DRLG ya ubicado (pieza, indice del tile del `.dt1`, capa y casilla de la region).
type registro struct {
	pieza  string
	indice int
	capa   d2automap.Layer
	x, y   int
}

// medida: lo que dio una resolucion (el `.dt1` como viene, o el compactado del arena de la consola).
type medida struct {
	celdas, piso, pared       int
	pisos, paredes            int
	pisosSin, paredesSin      int
	casillasConPiso, casillas int
	sinCelda                  map[claveSinCelda]int
	fuera                     int // registros cuyo indice no existe en la lista (fallas del `ExactTile`)
	sembradas                 int // celdas que vinieron del respaldo (la fila mas cercana)
	conCelda                  int // casillas de la region con alguna celda en el modelo
	casillasConTile           int // casillas de la region que el DRLG poblo con algo
}

func medirRegion(tabla *d2automap.Table, tiles []*drlgoutdoor.RoomTiles, rg region, semilla uint32,
	leer func(string) ([]byte, error)) {
	// ---- los `.dt1` de la region, cargados una vez (el (estilo, secuencia, tipo) sale de ahi)
	dt1s := map[string]*d2dt1.DT1{}

	cargar := func(pieza string) *d2dt1.DT1 {
		if dt, hay := dt1s[pieza]; hay {
			return dt
		}

		b, err := leer("data/global/tiles/" + pieza)
		if err != nil {
			fmt.Printf("   (no pude leer %s: %v)\n", pieza, err)
			dt1s[pieza] = nil

			return nil
		}

		dt, err := d2dt1.LoadDT1(b)
		if err != nil {
			fmt.Printf("   (no pude parsear %s: %v)\n", pieza, err)
			dt1s[pieza] = nil

			return nil
		}

		dt1s[pieza] = dt

		return dt
	}

	// ---- 1. LOS REGISTROS DE LA REGION, en el orden en que los escribe `SetExactTiles` (pisos y despues paredes)
	var registros []registro

	ubicar := func(pieza string, indice int, capa d2automap.Layer, gx, gy int) {
		if gx < rg.x || gy < rg.y || gx-rg.x >= rg.lado || gy-rg.y >= rg.lado {
			return
		}

		registros = append(registros, registro{pieza: pieza, indice: indice, capa: capa, x: gx - rg.x, y: gy - rg.y})
	}

	for _, rt := range tiles {
		if rt == nil {
			continue
		}

		for _, r := range rt.Floors {
			ubicar(r.Tile.File(), r.Tile.Idx, d2automap.LayerFloor, rt.Room.X+r.X-origenX, rt.Room.Y+r.Y-origenY)
		}

		for _, r := range rt.Walls {
			ubicar(r.Tile.File(), r.Tile.Idx, d2automap.LayerWall, rt.Room.X+r.X-origenX, rt.Room.Y+r.Y-origenY)
		}
	}

	// ---- 2. LAS CLAVES DEL NIVEL: las `(estilo, secuencia, tipo)` de los tiles REFERENCIADOS, tal como las arma
	// el plan de la consola (`moor-drlg/plan.go`). Son el filtro del arena de bloques usados.
	claves := map[int64]bool{}

	for _, r := range registros {
		if dt := cargar(r.pieza); dt != nil && r.indice >= 0 && r.indice < len(dt.Tiles) {
			t := &dt.Tiles[r.indice]
			claves[claveDeTile(int64(t.Style), int64(t.Sequence), int64(t.Type))] = true
		}
	}

	// ---- 3. LAS DOS RESOLUCIONES
	compactados := map[string][]d2dt1.Tile{}

	compactar := func(pieza string) []d2dt1.Tile {
		if lista, hay := compactados[pieza]; hay {
			return lista
		}

		dt := cargar(pieza)
		if dt == nil {
			compactados[pieza] = nil

			return nil
		}

		var out []d2dt1.Tile

		for i := range dt.Tiles {
			t := &dt.Tiles[i]
			if !claves[claveDeTile(int64(t.Style), int64(t.Sequence), int64(t.Type))] {
				continue
			}

			out = append(out, *t)
		}

		compactados[pieza] = out

		return out
	}

	// el informe de la compactacion: cuantos tiles del `.dt1` se descartan y en que archivos hay corrimiento
	fmt.Printf("\n=== REGION %dx%d desde %d,%d: EL ARENA DE BLOQUES USADOS ===\n", rg.lado, rg.lado, rg.x, rg.y)

	piezas := map[string]bool{}
	for _, r := range registros {
		piezas[r.pieza] = true
	}

	nombres := make([]string, 0, len(piezas))
	for p := range piezas {
		nombres = append(nombres, p)
	}

	sort.Strings(nombres)

	for _, p := range nombres {
		dt := cargar(p)
		if dt == nil {
			continue
		}

		lista := compactar(p)
		corridos := 0

		// un tile esta CORRIDO si su indice no cae en si mismo en la lista compactada
		for i := range dt.Tiles {
			if i < len(lista) && lista[i].Style == dt.Tiles[i].Style && lista[i].Sequence == dt.Tiles[i].Sequence &&
				lista[i].Type == dt.Tiles[i].Type {
				continue
			}

			corridos++
		}

		fmt.Printf("   %-34s tiles= %-4d enElArena= %-4d descartados= %-4d indicesCorridos= %d\n",
			p, len(dt.Tiles), len(lista), len(dt.Tiles)-len(lista), corridos)
	}

	medidas := []struct {
		nombre     string
		compactado bool
		respaldo   bool
	}{
		{"A. como viene el .dt1 (el indice del ExactTile cae en su propio tile), SIN siembra", false, false},
		{"B. como lo deja la consola HOY (el .dt1 COMPACTADO al arena de bloques usados)", true, false},
		{"C. indices correctos + siembra con la fila mas cercana (el arreglo de esta tarjeta)", false, true},
		{"D. indices correctos + siembra, pero con el .dt1 COMPACTADO (el defecto sigue puesto)", true, true},
	}

	imprimirLosRegistros(tabla, registros, cargar)

	for _, modo := range medidas {
		md := medir(tabla, registros, rg, semilla, cargar, compactar, modo.compactado, modo.respaldo)

		fmt.Printf("\n--- %s ---\n", modo.nombre)
		fmt.Printf("MODELO celdas= %d piso= %d pared= %d sembradas= %d\n", md.celdas, md.piso, md.pared, md.sembradas)
		fmt.Printf("COBERTURA casillasDeLaRegion= %d conCelda= %d (%.1f %%) conTile= %d (%.1f %% de las que tienen tile)\n",
			rg.lado*rg.lado, md.conCelda, pct(md.conCelda, rg.lado*rg.lado),
			md.casillasConTile, pct(md.conCelda, md.casillasConTile))
		fmt.Printf("REGION casillas= %d conPiso= %d pisos= %d paredes= %d fueraDeLaLista= %d\n",
			rg.lado*rg.lado, md.casillasConPiso, md.pisos, md.paredes, md.fuera)
		fmt.Printf("SIN-CELDA pisos= %d de %d (%.1f %%) paredes= %d de %d (%.1f %%) triplesDistintos= %d\n",
			md.pisosSin, md.pisos, pct(md.pisosSin, md.pisos), md.paredesSin, md.paredes,
			pct(md.paredesSin, md.paredes), len(md.sinCelda))

		imprimirSinCelda(tabla, md.sinCelda)
	}
}

// medir: arma las casillas con la resolucion pedida y corre el MODELO del motor sobre ellas.
func medir(tabla *d2automap.Table, registros []registro, rg region, semilla uint32,
	cargar func(string) *d2dt1.DT1, compactar func(string) []d2dt1.Tile, compactado, respaldo bool) *medida {
	md := &medida{sinCelda: map[claveSinCelda]int{}}
	celdas := map[[2]int]*celdaDeLaRegion{}

	porCasilla := map[[2]int]bool{}

	for _, r := range registros {
		porCasilla[[2]int{r.x, r.y}] = true

		dt := cargar(r.pieza)
		if dt == nil {
			md.fuera++

			continue
		}

		var t *d2dt1.Tile

		if compactado {
			lista := compactar(r.pieza)
			if r.indice < 0 || r.indice >= len(lista) {
				md.fuera++

				continue
			}

			copia := lista[r.indice]
			t = &copia
		} else {
			if r.indice < 0 || r.indice >= len(dt.Tiles) {
				md.fuera++

				continue
			}

			t = &dt.Tiles[r.indice]
		}

		// EL ADAPTADOR (automap_130.go): el piso va SIEMPRE con el tipo 0 (`d2enum.TileFloor`, que es "fl" en la
		// tabla) y la pared con el tipo del propio `.dt1`. Estilo y secuencia son los del `.dt1`, metidos en un
		// byte por `exactComponent` (`byte(dt.Style)`, `byte(dt.Sequence)`).
		tipo := int(t.Type)
		if r.capa == d2automap.LayerFloor {
			tipo = 0
		}

		// el registro en el `.dt1` nos da una referencia; el que arma el adaptador es esto
		ref := d2automap.TileRef{Layer: r.capa, Type: tipo, Style: int(byte(t.Style)), Sequence: int(byte(t.Sequence))}

		c := celdas[[2]int{r.x, r.y}]
		if c == nil {
			c = &celdaDeLaRegion{}
			celdas[[2]int{r.x, r.y}] = c
		}

		if r.capa == d2automap.LayerFloor {
			c.pisos = append(c.pisos, referenciaDe(ref))
			md.pisos++

			if _, ok := tabla.Cel(nivel, ref.Type, ref.Style, ref.Sequence, 0); !ok {
				md.pisosSin++
				md.sinCelda[claveSinCelda{ref.Layer, ref.Type, ref.Style, ref.Sequence}]++
			}
		} else {
			c.paredes = append(c.paredes, referenciaDe(ref))
			md.paredes++

			if _, ok := tabla.Cel(nivel, ref.Type, ref.Style, ref.Sequence, 0); !ok {
				md.paredesSin++
				md.sinCelda[claveSinCelda{ref.Layer, ref.Type, ref.Style, ref.Sequence}]++
			}
		}
	}

	for _, c := range celdas {
		if len(c.pisos) > 0 {
			md.casillasConPiso++
		}
	}

	md.casillasConTile = len(porCasilla)

	fuente := fuenteDeLaRegion{lado: rg.lado, celdas: celdas}

	modelo := d2automap.NewModel()

	if respaldo {
		md.celdas, md.sembradas = modelo.RevealTilesConRespaldo(tabla, fuente, nivel, semillaDelAutomap(nivel),
			float64(rg.lado/2), float64(rg.lado/2), d2automap.Area{HalfW: rg.lado * 8, HalfH: rg.lado * 4})
	} else {
		md.celdas = modelo.RevealTiles(tabla, fuente, nivel, semillaDelAutomap(nivel),
			float64(rg.lado/2), float64(rg.lado/2), d2automap.Area{HalfW: rg.lado * 8, HalfH: rg.lado * 4})
	}

	md.piso = modelo.CountLayer(d2automap.LayerFloor)
	md.pared = modelo.CountLayer(d2automap.LayerWall)

	// la cobertura: cuantas casillas de la REGION tienen alguna celda en el modelo (la pregunta del criterio)
	for ty := 0; ty < rg.lado; ty++ {
		for tx := 0; tx < rg.lado; tx++ {
			if modelo.HasAt(d2automap.TileCell(tx, ty)) {
				md.conCelda++
			}
		}
	}

	return md
}

// referenciaDe: el `TileRef` del adaptador como `ref` (para reusar la fuente de la region).
func referenciaDe(r d2automap.TileRef) ref {
	return ref{capa: r.Layer, tipo: r.Type, estilo: r.Style, secuencia: r.Sequence}
}

// imprimirLosRegistros: que tile del `.dt1` esta puesto en cada registro, ordenado por cuantas veces sale. Es el
// dato crudo: sin esto, "no matchea" no dice QUE tiles son.
func imprimirLosRegistros(tabla *d2automap.Table, registros []registro, cargar func(string) *d2dt1.DT1) {
	type cuenta struct {
		pieza           string
		indice, estilo  int
		secuencia, tipo int
		ancho, alto     int
		bloques         int
		veces           int
		capa            d2automap.Layer
		enLaTabla       bool
	}

	porTile := map[[3]interface{}]*cuenta{}

	for _, r := range registros {
		dt := cargar(r.pieza)
		if dt == nil || r.indice < 0 || r.indice >= len(dt.Tiles) {
			continue
		}

		t := &dt.Tiles[r.indice]
		k := [3]interface{}{r.pieza, r.indice, r.capa}
		c := porTile[k]
		if c == nil {
			tipo := int(t.Type)
			if r.capa == d2automap.LayerFloor {
				tipo = 0
			}

			_, ok := tabla.Cel(nivel, tipo, int(byte(t.Style)), int(byte(t.Sequence)), 0)

			c = &cuenta{pieza: r.pieza, indice: r.indice, estilo: int(t.Style), secuencia: int(t.Sequence),
				tipo: int(t.Type), ancho: int(t.Width), alto: int(t.Height), bloques: len(t.Blocks), capa: r.capa,
				enLaTabla: ok}
			porTile[k] = c
		}

		c.veces++
	}

	lista := make([]*cuenta, 0, len(porTile))
	for _, c := range porTile {
		lista = append(lista, c)
	}

	sort.Slice(lista, func(i, j int) bool {
		if lista[i].veces != lista[j].veces {
			return lista[i].veces > lista[j].veces
		}

		return lista[i].pieza < lista[j].pieza
	})

	fmt.Println("\nLOS TILES QUE EL DRLG PONE (los 20 que mas salen):")
	fmt.Println("   capa pieza#indice                          estilo seq  tipo caja     bloques veces enLaTabla")

	for i, c := range lista {
		if i >= 20 {
			break
		}

		fmt.Printf("   %-4s %-40s %5d %4d %4d %3dx%-3d %6d %5d %v\n",
			capaNombre(c.capa), fmt.Sprintf("%s#%d", c.pieza, c.indice), c.estilo, c.secuencia, c.tipo,
			c.ancho, c.alto, c.bloques, c.veces, c.enLaTabla)
	}
}

// claveDeTile: `(estilo, secuencia, tipo)` empaquetados como en `d2mapengine.claveDeTile` y
// `d2asset.ClaveDeTile` (21 bits cada uno). Es la clave del arena de bloques usados.
func claveDeTile(estilo, secuencia, tipo int64) int64 {
	const (
		bits  = 21
		masca = (1 << bits) - 1
	)

	return estilo&masca<<(2*bits) | secuencia&masca<<bits | tipo&masca
}

func pct(a, b int) float64 {
	if b == 0 {
		return 0
	}

	return float64(100*a) / float64(b)
}

// imprimirSinCelda: las `(tipo, estilo, secuencia)` que no encuentran fila, ordenadas por cuantas veces salen, y
// el diagnostico de cada una: hay fila con ese tipo y ese estilo (y que secuencias pide), o hay fila con ese tipo
// y esa secuencia (y que estilos pide).
func imprimirSinCelda(tabla *d2automap.Table, sinCelda map[claveSinCelda]int) {
	type fila struct {
		capa      d2automap.Layer
		tipo      int
		estilo    int
		secuencia int
		veces     int
	}

	filas := make([]fila, 0, len(sinCelda))

	for k, v := range sinCelda {
		filas = append(filas, fila{k.capa, k.tipo, k.estilo, k.secuencia, v})
	}

	sort.Slice(filas, func(i, j int) bool {
		if filas[i].veces != filas[j].veces {
			return filas[i].veces > filas[j].veces
		}

		if filas[i].capa != filas[j].capa {
			return filas[i].capa < filas[j].capa
		}

		if filas[i].tipo != filas[j].tipo {
			return filas[i].tipo < filas[j].tipo
		}

		if filas[i].estilo != filas[j].estilo {
			return filas[i].estilo < filas[j].estilo
		}

		return filas[i].secuencia < filas[j].secuencia
	})

	fmt.Println("SIN-CELDA detalle (capa, tipo, estilo, secuencia, veces) y que hay en la tabla:")

	for _, f := range filas {
		fmt.Printf("   %-5s tipo= %2d estilo= %2d sec= %2d x%-4d  tabla: %s\n",
			capaNombre(f.capa), f.tipo, f.estilo, f.secuencia, f.veces,
			diagnostico(tabla, f.tipo, f.estilo, f.secuencia))
	}
}

// diagnostico: que filas de la tabla tocan ese (tipo, estilo, secuencia).
func diagnostico(tabla *d2automap.Table, tipo, estilo, secuencia int) string {
	var mismoTipo, mismoTipoEstilo, mismoTipoSeq []string

	for _, r := range tabla.LevelRows(nivel) {
		if r.Tile != tipo {
			continue
		}

		mismoTipo = append(mismoTipo, rangoEstilo(r))

		if r.Style == d2automap.Any || r.Style == estilo {
			mismoTipoEstilo = append(mismoTipoEstilo, rangoSeq(r))
		}

		if r.Start == d2automap.Any || (secuencia >= r.Start && secuencia <= r.End) {
			mismoTipoSeq = append(mismoTipoSeq, rangoEstilo(r))
		}
	}

	switch {
	case len(mismoTipoEstilo) > 0:
		return fmt.Sprintf("estilo SI, secuencia fuera: pide %s", strings.Join(mismoTipoEstilo, " "))
	case len(mismoTipoSeq) > 0:
		return fmt.Sprintf("secuencia SI, estilo fuera: pide %s", strings.Join(mismoTipoSeq, " "))
	case len(mismoTipo) > 0:
		return fmt.Sprintf("ni estilo ni secuencia: la tabla pide %s", strings.Join(mismoTipo, " "))
	default:
		return "la tabla no tiene NINGUNA fila de ese tipo"
	}
}

func rangoEstilo(r d2automap.Row) string {
	s := "any"
	if r.Style != d2automap.Any {
		s = strconv.Itoa(r.Style)
	}

	return fmt.Sprintf("[estilo %s seq %s, cels %v]", s, rangoSeq(r), r.Cels)
}

func rangoSeq(r d2automap.Row) string {
	if r.Start == d2automap.Any {
		return "any"
	}

	return fmt.Sprintf("%d..%d", r.Start, r.End)
}

func capaNombre(l d2automap.Layer) string {
	if l == d2automap.LayerFloor {
		return "piso"
	}

	return "pared"
}

// imprimirLasFilas: la tabla del nivel, tal cual (tipo en nombre y numero, estilo, rango de secuencia, cels).
func imprimirLasFilas(filas []d2automap.Row) {
	fmt.Println("TABLA filas del nivel (tipo estilo secuencias cels):")

	for _, r := range filas {
		fmt.Printf("   tipo= %2d (%-6s) estilo= %-4s seq= %-8s cels= %v\n",
			r.Tile, nombreDeTipo(r.Tile), estiloTexto(r.Style), rangoSeq(r), r.Cels)
	}
}

// nombreDeTipo: los nombres del motor (`tileNames` de `d2automap`, Game.exe 0x6e9540).
var nombresDeTipo = [...]string{
	"fl", "wl", "wr", "wtlr", "wtll", "wtr", "wbl", "wbr", "wld", "wrd",
	"wle", "wre", "co", "sh", "tr", "rf", "ld", "rd", "fd", "fi",
}

func nombreDeTipo(t int) string {
	if t < 0 || t >= len(nombresDeTipo) {
		return "?"
	}

	return nombresDeTipo[t]
}

func estiloTexto(s int) string {
	if s == d2automap.Any {
		return "any"
	}

	return strconv.Itoa(s)
}

func semillaDelAutomap(nivel int) uint32 {
	return uint32(nivel+1) * 2654435761
}

// parsearRegiones: "x,y:lado;x,y:lado" -> []region.
func parsearRegiones(s string) ([]region, error) {
	var out []region

	for _, trozo := range strings.Split(s, ";") {
		trozo = strings.TrimSpace(trozo)
		if trozo == "" {
			continue
		}

		partes := strings.Split(trozo, ":")
		if len(partes) != 2 {
			return nil, fmt.Errorf("%q no es x,y:lado", trozo)
		}

		xy := strings.Split(partes[0], ",")
		if len(xy) != 2 {
			return nil, fmt.Errorf("%q no es x,y", partes[0])
		}

		x, err := strconv.Atoi(strings.TrimSpace(xy[0]))
		if err != nil {
			return nil, err
		}

		y, err := strconv.Atoi(strings.TrimSpace(xy[1]))
		if err != nil {
			return nil, err
		}

		lado, err := strconv.Atoi(strings.TrimSpace(partes[1]))
		if err != nil {
			return nil, err
		}

		out = append(out, region{lado: lado, x: x, y: y})
	}

	if len(out) == 0 {
		return nil, fmt.Errorf("no hay regiones")
	}

	return out, nil
}

// ---- los mismos ayudantes que `moor-drlg` (los MPQ y las tablas del DRLG)

func abrirMPQs(dir string) func(string) ([]byte, error) {
	entradas, _ := os.ReadDir(dir)
	enDisco := map[string]string{}

	for _, e := range entradas {
		enDisco[strings.ToLower(e.Name())] = e.Name()
	}

	var mpqs []*d2mpq.MPQ

	for _, nombre := range []string{"patch_d2.mpq", "d2data.mpq"} {
		real, hay := enDisco[nombre]
		if !hay {
			continue
		}

		m, err := d2mpq.FromFile(dir + "/" + real)
		if err != nil {
			fmt.Println("no abri", real, err)
			os.Exit(1)
		}

		mpqs = append(mpqs, m)
	}

	if len(mpqs) == 0 {
		fmt.Println("no encontre patch_d2.mpq / d2data.mpq en", dir)
		os.Exit(1)
	}

	return func(nombre string) ([]byte, error) {
		clave := strings.TrimPrefix(nombre, "/")
		clave = strings.ReplaceAll(clave, "/", string(rune(92)))

		var ultimo error

		for _, m := range mpqs {
			datos, err := m.ReadFile(clave)
			if err == nil {
				return datos, nil
			}

			ultimo = err
		}

		return nil, ultimo
	}
}

func tablas(leer func(string) ([]byte, error)) (*d2drlg.Tables, error) {
	get := func(p string) []byte {
		b, err := leer(p)
		if err != nil {
			return nil
		}

		return b
	}

	raw := d2drlg.Raw{
		Levels:      get("/data/global/excel/Levels.txt"),
		LvlMaze:     get("/data/global/excel/LvlMaze.txt"),
		LvlPrest:    get("/data/global/excel/LvlPrest.txt"),
		LvlPrestBin: get("/data/global/excel/lvlprest.bin"),
		LvlTypes:    get("/data/global/excel/LvlTypes.txt"),
		LvlSub:      get("/data/global/excel/LvlSub.txt"),
		LvlWarp:     get("/data/global/excel/LvlWarp.txt"),
	}

	if raw.Levels == nil || raw.LvlMaze == nil {
		return nil, fmt.Errorf("faltan tablas")
	}

	t, err := d2drlg.Load(raw)
	if err != nil && raw.LvlPrestBin != nil {
		raw.LvlPrestBin = nil

		return d2drlg.Load(raw)
	}

	return t, err
}

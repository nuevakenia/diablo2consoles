// El plan de una REGION del Blood Moor, en Go, para que la consola la componga (`-plan`).
package main

import (
	"fmt"
	"go/format"
	"os"
	"sort"
	"strings"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2drlg/drlgoutdoor"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2dt1"
)

// tilePlan: un registro del DRLG ya resuelto a (pieza, indice del tile del `.dt1`) + flags de colision.
type tilePlan struct {
	Pieza  int
	Indice int
	Flags  uint32
}

type celdaPlan struct {
	X, Y                    int
	Pisos, Paredes, Sombras []tilePlan
}

type plan struct {
	semilla    uint32
	nivel      int
	origenX    int
	origenY    int
	anchoNivel int
	altoNivel  int
	regionX    int
	regionY    int
	lado       int
	ancho      int
	alto       int
	piezas     []string
	claves     []int64
	buffer     int
	mayorPieza string
	celdas     []celdaPlan
	spawnX     int
	spawnY     int
	registros  int
	crudo      int
}

// tileBruto: igual que tilePlan pero con el NOMBRE de la pieza, que es lo unico que da el DRLG.
type tileBruto struct {
	pieza  string
	indice int
	flags  uint32
}

type celdaBruta struct {
	x, y                    int
	pisos, paredes, sombras []tileBruto
}

// planificar: los registros de tile que caen en la region [regionX,regionX+lado) x [regionY,regionY+lado), ya
// agrupados por casilla y resueltos a (pieza, indice), que es lo que `SetExactTiles` come.
func planificar(tiles []*drlgoutdoor.RoomTiles, lado, regionX, regionY int, semilla uint32,
	leer func(string) ([]byte, error)) *plan {
	return planificarRect(tiles, lado, lado, regionX, regionY, semilla, leer)
}

// planificarRect: lo mismo para una region que NO es cuadrada. Hace falta para cubrir el nivel sin solaparse: el
// Blood Moor mide 56 x 96 y una rejilla de 32 en 32 deja la primera columna de 24 (56 = 24 + 32), asi que la
// region dice su ancho y su alto y no un solo lado. El plan de DC-002.145 (32 x 32) sale identico por el camino
// de `planificar`.
func planificarRect(tiles []*drlgoutdoor.RoomTiles, ancho, alto, regionX, regionY int, semilla uint32,
	leer func(string) ([]byte, error)) *plan {
	pl := &plan{
		semilla: semilla, nivel: nivel, origenX: origenX, origenY: origenY,
		regionX: regionX, regionY: regionY, ancho: ancho, alto: alto, lado: ancho,
	}

	porCelda := map[[2]int]*celdaBruta{}
	piezas := map[string]bool{}

	for _, rt := range tiles {
		if rt == nil {
			continue
		}

		add := func(lista []*drlgoutdoor.TileRecord, capa int) {
			for _, r := range lista {
				gx, gy := rt.Room.X+r.X-origenX, rt.Room.Y+r.Y-origenY
				if gx-regionX < 0 || gy-regionY < 0 || gx-regionX >= ancho || gy-regionY >= alto {
					continue
				}

				x, y := gx-regionX, gy-regionY

				c := porCelda[[2]int{x, y}]
				if c == nil {
					c = &celdaBruta{x: x, y: y}
					porCelda[[2]int{x, y}] = c
				}

				tb := tileBruto{pieza: r.Tile.File(), indice: r.Tile.Idx, flags: r.Flags}
				piezas[tb.pieza] = true

				switch capa {
				case 0:
					c.pisos = append(c.pisos, tb)
				case 1:
					c.paredes = append(c.paredes, tb)
				case 2:
					c.sombras = append(c.sombras, tb)
				}

				pl.registros++
			}
		}

		add(rt.Floors, 0)
		add(rt.Walls, 1)
		add(rt.Shadows, 2)
	}

	nombres := make([]string, 0, len(piezas))
	for p := range piezas {
		nombres = append(nombres, p)
	}

	sort.Strings(nombres)

	indice := map[string]int{}
	for i, n := range nombres {
		indice[n] = i
	}

	pl.piezas = nombres

	// las claves: las `(estilo, secuencia, tipo)` de los tiles REFERENCIADOS (el tipo del propio `.dt1`, que es
	// lo que mira el motor), y el buffer prestado: el `.dt1` mas grande de la region.
	claves := map[int64]bool{}

	for _, p := range nombres {
		b, err := leer("data/global/tiles/" + p)
		if err != nil {
			fmt.Printf("   (no pude leer %s: %v)\n", p, err)

			continue
		}

		pl.crudo += len(b)

		if len(b) > pl.buffer {
			pl.buffer, pl.mayorPieza = len(b), p
		}

		dt, err := d2dt1.LoadDT1(b)
		if err != nil {
			fmt.Printf("   (no pude parsear %s: %v)\n", p, err)

			continue
		}

		usados := map[int]bool{}
		for _, c := range porCelda {
			for _, lista := range [][]tileBruto{c.pisos, c.paredes, c.sombras} {
				for _, t := range lista {
					if t.pieza == p {
						usados[t.indice] = true
					}
				}
			}
		}

		for i := range dt.Tiles {
			if !usados[i] {
				continue
			}

			t := &dt.Tiles[i]
			claves[clave(int64(t.Style), int64(t.Sequence), int64(t.Type))] = true
		}
	}

	for c := range claves {
		pl.claves = append(pl.claves, c)
	}

	sort.Slice(pl.claves, func(i, j int) bool { return pl.claves[i] < pl.claves[j] })

	// las casillas, en orden de lectura (y, x) para que el `.go` generado se lea como el mapa
	clavesCelda := make([][2]int, 0, len(porCelda))
	for k := range porCelda {
		clavesCelda = append(clavesCelda, k)
	}

	sort.Slice(clavesCelda, func(i, j int) bool {
		if clavesCelda[i][1] != clavesCelda[j][1] {
			return clavesCelda[i][1] < clavesCelda[j][1]
		}

		return clavesCelda[i][0] < clavesCelda[j][0]
	})

	convertir := func(lista []tileBruto) []tilePlan {
		if len(lista) == 0 {
			return nil
		}

		out := make([]tilePlan, 0, len(lista))
		for _, t := range lista {
			out = append(out, tilePlan{Pieza: indice[t.pieza], Indice: t.indice, Flags: t.flags})
		}

		return out
	}

	for _, k := range clavesCelda {
		c := porCelda[k]
		pl.celdas = append(pl.celdas, celdaPlan{
			X: c.x, Y: c.y,
			Pisos:   convertir(c.pisos),
			Paredes: convertir(c.paredes),
			Sombras: convertir(c.sombras),
		})
	}

	pl.spawnX, pl.spawnY = buscarSpawn(pl.celdas, ancho, alto)

	return pl
}

// buscarSpawn: la casilla con piso y sin paredes mas cercana al centro de la region (el jugador tiene que poder
// caminar desde ahi).
func buscarSpawn(celdas []celdaPlan, ancho, alto int) (int, int) {
	cx, cy := ancho/2, alto/2
	mejorX, mejorY, mejorD := -1, -1, -1

	for _, c := range celdas {
		if len(c.Pisos) == 0 || len(c.Paredes) != 0 {
			continue
		}

		dx, dy := c.X-cx, c.Y-cy
		if dx < 0 {
			dx = -dx
		}

		if dy < 0 {
			dy = -dy
		}

		d := dx + dy
		if mejorD < 0 || d < mejorD {
			mejorX, mejorY, mejorD = c.X, c.Y, d
		}
	}

	return mejorX, mejorY
}

// emitir: el plan como fuente Go, para que el build de la consola lo traiga adentro (el POC no tiene un sistema
// de archivos propio: lo que no esta en el MPQ o en el binario, no existe).
func emitir(pl *plan, ruta string) error {
	var b strings.Builder

	fmt.Fprintf(&b, "//go:build dreamcast\n\n")
	fmt.Fprintf(&b, "// CODIGO GENERADO por `console/dc/medidas/moor-drlg -plan ...`. NO SE EDITA A MANO:\n")
	fmt.Fprintf(&b, "// el plan lo calcula el DRLG del acto 1 en el HOST (`drlgworld.Generate` + `drlgoutdoor.Generate` +\n")
	fmt.Fprintf(&b, "// `Level.BuildTiles`) y la consola lo COMPONE con `SetExactTiles` (`moor_145.go`), region por region.\n")
	fmt.Fprintf(&b, "//\n")
	fmt.Fprintf(&b, "// Regenerar con:\n")
	fmt.Fprintf(&b, "//\n")
	fmt.Fprintf(&b, "//   go run ./console/dc/medidas/moor-drlg -plan console/dc/poc/juego-131/moor_region_145.go \\\n")
	fmt.Fprintf(&b, "//       -seed %d -lado %d -ox %d -oy %d \"<carpeta de Diablo II>\"\n\n", pl.semilla, pl.lado, pl.regionX, pl.regionY)
	fmt.Fprintf(&b, "package main\n\n")

	fmt.Fprintf(&b, "// Dónde está la región que este plan compone, dentro del Blood Moor (2 = el nivel en `Levels.txt`).\n")
	fmt.Fprintf(&b, "const (\n")
	fmt.Fprintf(&b, "\tmoorPlanoNivel      = %d\n", pl.nivel)
	fmt.Fprintf(&b, "\tmoorPlanoSemilla    = %d\n", pl.semilla)
	fmt.Fprintf(&b, "\tmoorPlanoNivelX     = %d // origen del nivel en el mundo (el rectángulo del DRLG)\n", pl.origenX)
	fmt.Fprintf(&b, "\tmoorPlanoNivelY     = %d\n", pl.origenY)
	fmt.Fprintf(&b, "\tmoorPlanoNivelAncho = %d // el Blood Moor entero, en casillas\n", pl.anchoNivel)
	fmt.Fprintf(&b, "\tmoorPlanoNivelAlto  = %d\n", pl.altoNivel)
	fmt.Fprintf(&b, "\tmoorPlanoRegionX    = %d // la región que entra en la consola\n", pl.regionX)
	fmt.Fprintf(&b, "\tmoorPlanoRegionY    = %d\n", pl.regionY)
	fmt.Fprintf(&b, "\tmoorPlanoLado       = %d\n", pl.lado)
	fmt.Fprintf(&b, "\tmoorPlanoSpawnX     = %d // casilla de la región donde arranca el jugador\n", pl.spawnX)
	fmt.Fprintf(&b, "\tmoorPlanoSpawnY     = %d\n", pl.spawnY)
	fmt.Fprintf(&b, "\tmoorPlanoBuffer     = %d // el `.dt1` más grande de la región (%s)\n", pl.buffer, pl.mayorPieza)
	fmt.Fprintf(&b, "\tmoorPlanoRegistros  = %d\n", pl.registros)
	fmt.Fprintf(&b, ")\n\n")

	fmt.Fprintf(&b, "// moorPlanoPiezas: los `.dt1` que la región referencia, en el orden que usa el plan.\n")
	fmt.Fprintf(&b, "var moorPlanoPiezas = []string{\n")

	for _, p := range pl.piezas {
		fmt.Fprintf(&b, "\t%q,\n", p)
	}

	fmt.Fprintf(&b, "}\n\n")

	fmt.Fprintf(&b, "// moorPlanoClaves: las `(estilo, secuencia, tipo)` de los tiles de la región, empaquetadas como las empaqueta\n")
	fmt.Fprintf(&b, "// `d2asset.ClaveDeTile`. Es el filtro del arena de bloques (`PrepararArenaDeBloquesUsados`): solo se copian\n")
	fmt.Fprintf(&b, "// los bloques de los tiles cuya clave está acá.\n")
	fmt.Fprintf(&b, "var moorPlanoClaves = []int64{\n")

	for i, c := range pl.claves {
		if i%4 == 0 {
			fmt.Fprintf(&b, "\t")
		}

		fmt.Fprintf(&b, "%d, ", c)

		if i%4 == 3 {
			fmt.Fprintf(&b, "\n")
		}
	}

	fmt.Fprintf(&b, "\n}\n\n")

	fmt.Fprintf(&b, "// moorCelda: una casilla de la región, con sus tres capas de registros.\n")
	fmt.Fprintf(&b, "type moorCelda struct {\n\tX, Y                   int\n\tPisos, Paredes, Sombras []moorTile\n}\n\n")
	fmt.Fprintf(&b, "// moorTile: un registro del DRLG: el tile `Indice` de la pieza `Pieza` (índice de `moorPlanoPiezas`) y sus\n")
	fmt.Fprintf(&b, "// flags de colisión, tal como los aplica `d2mapengine.SetExactTiles`.\n")
	fmt.Fprintf(&b, "type moorTile struct {\n\tPieza, Indice int\n\tFlags         uint32\n}\n\n")

	fmt.Fprintf(&b, "// moorPlanoCeldas: la región entera, casilla por casilla (las %d que tienen algo).\n", len(pl.celdas))
	fmt.Fprintf(&b, "var moorPlanoCeldas = []moorCelda{\n")

	for _, c := range pl.celdas {
		fmt.Fprintf(&b, "\t{X: %d, Y: %d, Pisos: %s, Paredes: %s, Sombras: %s},\n",
			c.X, c.Y, emitirTiles(c.Pisos), emitirTiles(c.Paredes), emitirTiles(c.Sombras))
	}

	fmt.Fprintf(&b, "}\n")

	// el plan se escribe YA pasado por gofmt (`go/format`): es un archivo de Go del repo y no tiene por que
	// entrar con otro formato. Si el formateo fallara, se escribe lo crudo: el plan es dato, no se pierde.
	crudo := []byte(sinEspaciosAlFinal(b.String()))
	if formateado, err := format.Source(crudo); err == nil {
		crudo = formateado
	}

	return os.WriteFile(ruta, crudo, 0o644)
}

// sinEspaciosAlFinal: el emisor corta las listas cada 4 valores y deja un espacio detras de la ultima coma de
// la linea. gofmt no los deja, asi que el plan que se escribe es el que queda con `gofmt -l` limpio.
func sinEspaciosAlFinal(s string) string {
	lineas := strings.Split(s, "\n")
	for i := range lineas {
		lineas[i] = strings.TrimRight(lineas[i], " \t")
	}

	return strings.Join(lineas, "\n")
}

func emitirTiles(lista []tilePlan) string {
	if len(lista) == 0 {
		return "nil"
	}

	partes := make([]string, 0, len(lista))

	for _, t := range lista {
		partes = append(partes, fmt.Sprintf("{%d, %d, %d}", t.Pieza, t.Indice, t.Flags))
	}

	return "[]moorTile{" + strings.Join(partes, ", ") + "}"
}

// ============================== EL NIVEL ENTERO, POR REGIONES, EN FORMA PLANA (DC-002.147) ==============================

// origenesDeLaRejilla: los origenes de las regiones de `lado` a lo largo de un eje de `total` casillas, SIN
// SOLAPARSE. Se avanza de a `lado` y el ULTIMO origen se pega al borde (`total - lado`), asi que la region que no
// llega al borde es la PRIMERA: el Blood Moor mide 56 de ancho y con 32 la rejilla da origenes 0 y 24, o sea una
// region de 24 (0..23) y otra de 32 (24..55). Con 96 de alto y 32 los tres origenes son 0, 32 y 64.
func origenesDeLaRejilla(total, lado int) []int {
	if lado <= 0 || total <= 0 {
		return nil
	}

	origenes := make([]int, 0, total/lado+1)

	for o := 0; o < total; o += lado {
		origenes = append(origenes, o)
	}

	if n := len(origenes); n > 1 && origenes[n-1]+lado > total {
		origenes[n-1] = total - lado
	}

	return origenes
}

// regionesDeLaRejilla: las (x, y, ancho, alto) de las regiones que cubren el nivel. Las que no llegan al borde
// (la primera de cada eje cuando el nivel no es multiplo de `lado`) quedan mas angostas, y el plan lo dice.
func regionesDeLaRejilla(anchoNivel, altoNivel, lado int) [][4]int {
	xs := origenesDeLaRejilla(anchoNivel, lado)
	ys := origenesDeLaRejilla(altoNivel, lado)

	out := make([][4]int, 0, len(xs)*len(ys))

	// el ancho de cada region es lo que hay hasta el origen SIGUIENTE (la ultima, hasta el borde): asi las
	// regiones cubren el nivel SIN solaparse y sin dejar huecos.
	anchoDe := func(origenes []int, i, total int) int {
		if i+1 < len(origenes) {
			return origenes[i+1] - origenes[i]
		}

		return total - origenes[i]
	}

	for j, y := range ys {
		for i, x := range xs {
			out = append(out, [4]int{x, y, anchoDe(xs, i, anchoNivel), anchoDe(ys, j, altoNivel)})
		}
	}

	return out
}

// emitirRegiones: el nivel entero (todas sus regiones) como fuente Go, en FORMA PLANA.
//
// POR QUE PLANA Y NO LA DE `emitir`. La forma de `emitir` es un `[]moorCelda` con TRES rebanadas por casilla: se
// lee como el mapa, pero cada casilla arrastra tres punteros (24 B en la consola) y cada registro su propia
// estructura. Medido en el `main.o` del build de DC-002.146b: el plan de UNA region de 32 x 32 son 45.056 B de
// `.data` (van al `1ST_READ.BIN`, que se carga ENTERO en la RAM de la consola). En forma plana la misma region son
// 1.024 casillas x 8 `uint16` (16.384 B) + 1.264 registros x 3 `uint16` (7.584 B) = 23.968 B, y no hay ni una
// rebanada adentro del dato: el que compone lee `Tiles[3*off:3*(off+len)]` y arma la casilla.
func emitirRegiones(planes []*plan, ruta string) error {
	if len(planes) == 0 {
		return fmt.Errorf("no hay regiones que emitir")
	}

	var b strings.Builder

	p0 := planes[0]

	fmt.Fprintf(&b, "//go:build dreamcast\n\n")
	fmt.Fprintf(&b, "// CODIGO GENERADO por `console/dc/medidas/moor-drlg -plan-regiones ...`. NO SE EDITA A MANO:\n")
	fmt.Fprintf(&b, "// el plan lo calcula el DRLG del acto 1 en el HOST (`drlgworld.Generate` + `drlgoutdoor.Generate` +\n")
	fmt.Fprintf(&b, "// `Level.BuildTiles`) sobre los MPQ, y la consola lo COMPONE con `SetExactTiles` (`moor_147.go`),\n")
	fmt.Fprintf(&b, "// region por region y EN CALIENTE.\n")
	fmt.Fprintf(&b, "//\n")
	fmt.Fprintf(&b, "// Regenerar con:\n")
	fmt.Fprintf(&b, "//\n")
	fmt.Fprintf(&b, "//   go run ./console/dc/medidas/moor-drlg -plan-regiones console/dc/poc/juego-131/moor_regiones_147.go \\\n")
	fmt.Fprintf(&b, "//       -seed %d -lado %d \"<carpeta de Diablo II>\"\n\n", p0.semilla, p0.lado)
	fmt.Fprintf(&b, "package main\n\n")

	fmt.Fprintf(&b, "// Del nivel que estas regiones cubren (2 = Blood Moor en `Levels.txt`), y su rectangulo en el mundo.\n")
	fmt.Fprintf(&b, "const (\n")
	fmt.Fprintf(&b, "	moorNivel147      = %d\n", p0.nivel)
	fmt.Fprintf(&b, "	moorSemilla147    = %d\n", p0.semilla)
	fmt.Fprintf(&b, "	moorNivelX147     = %d\n", p0.origenX)
	fmt.Fprintf(&b, "	moorNivelY147     = %d\n", p0.origenY)
	fmt.Fprintf(&b, "	moorNivelAncho147 = %d\n", p0.anchoNivel)
	fmt.Fprintf(&b, "	moorNivelAlto147  = %d\n", p0.altoNivel)
	fmt.Fprintf(&b, ")\n\n")

	fmt.Fprintf(&b, "// moorPlanes147: las %d regiones que CUBREN el nivel sin solaparse, en orden de rejilla (y, x).\n", len(planes))
	fmt.Fprintf(&b, "// `Celdas` es un `[]uint16` con 8 valores por casilla -- x, y, y las tres capas como (offset, largo) sobre\n")
	fmt.Fprintf(&b, "// `Tiles` -- y `Tiles` es un `[]uint16` con 3 por registro: la pieza (indice de `Piezas`), el indice del tile\n")
	fmt.Fprintf(&b, "// en el `.dt1` y los flags de colision. Las dos listas van en `.rodata`/`.data`: no hay ni una rebanada\n")
	fmt.Fprintf(&b, "// adentro del dato ni una reserva al arrancar.\n")
	fmt.Fprintf(&b, "var moorPlanes147 = []moorPlanRegion{\n")

	for _, pl := range planes {
		fmt.Fprintf(&b, "	{\n")
		fmt.Fprintf(&b, "		X: %d, Y: %d, Ancho: %d, Alto: %d,\n", pl.regionX, pl.regionY, pl.ancho, pl.alto)
		fmt.Fprintf(&b, "		SpawnX: %d, SpawnY: %d,\n", pl.spawnX, pl.spawnY)
		fmt.Fprintf(&b, "		Buffer: %d, Registros: %d,\n", pl.buffer, pl.registros)
		fmt.Fprintf(&b, "		Piezas: []string{\n")

		for _, pz := range pl.piezas {
			fmt.Fprintf(&b, "			%q,\n", pz)
		}

		fmt.Fprintf(&b, "		},\n")
		fmt.Fprintf(&b, "		Claves: []int64{\n")

		for i, c := range pl.claves {
			if i%4 == 0 {
				fmt.Fprintf(&b, "			")
			}

			fmt.Fprintf(&b, "%d, ", c)

			if i%4 == 3 {
				fmt.Fprintf(&b, "\n")
			}
		}

		fmt.Fprintf(&b, "\n		},\n")
		fmt.Fprintf(&b, "		Celdas: []uint16{\n")

		escribirEnLineas(&b, "			", 8, celdasPlanas(pl))

		fmt.Fprintf(&b, "		},\n")
		fmt.Fprintf(&b, "		Tiles: []uint16{\n")

		escribirEnLineas(&b, "			", 3, tilesPlanos(pl))

		fmt.Fprintf(&b, "		},\n")
		fmt.Fprintf(&b, "	},\n")
	}

	fmt.Fprintf(&b, "}\n")

	crudo := []byte(sinEspaciosAlFinal(b.String()))
	if formateado, err := format.Source(crudo); err == nil {
		crudo = formateado
	}

	return os.WriteFile(ruta, crudo, 0o644)
}

// celdasPlanas: las casillas del plan como 8 enteros por casilla (x, y, (off,len) de piso, pared y sombra sobre la
// lista de tiles plana). El orden es el de `pl.celdas`, que viene en orden de lectura (y, x).
func celdasPlanas(pl *plan) []int {
	out := make([]int, 0, len(pl.celdas)*8)
	off := 0

	for _, c := range pl.celdas {
		out = append(out, c.X, c.Y)
		out = append(out, off, len(c.Pisos))
		off += len(c.Pisos)
		out = append(out, off, len(c.Paredes))
		off += len(c.Paredes)
		out = append(out, off, len(c.Sombras))
		off += len(c.Sombras)
	}

	return out
}

// tilesPlanos: los registros de las casillas, en el MISMO orden que `celdasPlanas` recorre (las tres capas de cada
// casilla, una detras de la otra), como 3 enteros por registro: pieza, indice y flags.
func tilesPlanos(pl *plan) []int {
	out := make([]int, 0, pl.registros*3)

	for _, c := range pl.celdas {
		for _, lista := range [][]tilePlan{c.Pisos, c.Paredes, c.Sombras} {
			for _, t := range lista {
				out = append(out, t.Pieza, t.Indice, int(t.Flags))
			}
		}
	}

	return out
}

// escribirEnLineas: los enteros de a `porLinea` por renglon, con la coma al final (gofmt la deja asi).
func escribirEnLineas(b *strings.Builder, sangria string, porLinea int, valores []int) {
	for i, v := range valores {
		if i%porLinea == 0 {
			fmt.Fprintf(b, "%s", sangria)
		}

		fmt.Fprintf(b, "%d, ", v)

		if i%porLinea == porLinea-1 {
			fmt.Fprintf(b, "\n")
		}
	}

	if len(valores)%porLinea != 0 {
		fmt.Fprintf(b, "\n")
	}
}

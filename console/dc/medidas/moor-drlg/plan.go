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
	pl := &plan{
		semilla: semilla, nivel: nivel, origenX: origenX, origenY: origenY,
		regionX: regionX, regionY: regionY, lado: lado,
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
				if gx-regionX < 0 || gy-regionY < 0 || gx-regionX >= lado || gy-regionY >= lado {
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

	pl.spawnX, pl.spawnY = buscarSpawn(pl.celdas, lado)

	return pl
}

// buscarSpawn: la casilla con piso y sin paredes mas cercana al centro de la region (el jugador tiene que poder
// caminar desde ahi).
func buscarSpawn(celdas []celdaPlan, lado int) (int, int) {
	cx, cy := lado/2, lado/2
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

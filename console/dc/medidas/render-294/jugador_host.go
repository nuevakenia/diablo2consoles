// DC-002.112 -- EL JUGADOR DEL CUADRO EN EL HOST: la misma escena que dibuja la consola, para comparar.
//
// QUE ES. El medidor del pueblo (`render-294`) dibuja el cuadro con el MISMO `d2maprenderer` y la MISMA camara
// que la consola, pero sobre una superficie de software en Go puro. Esta pieza le agrega el JUGADOR: compone el
// ciclo de la amazona con el compositor COMPARTIDO (`personaje.go`) y lo pega encima del mapa con el MISMO
// archivo que usa el POC de la consola (`jugador.go`: la paleta, el ancla y el pincel). Los dos lados, entonces,
// no se comparan "a ojo": dan el mismo crc32 del compuesto y el mismo crc32 del cuadro, o hay un bug.
//
// LAS DIFERENCIAS CON LA CONSOLA, todas de infraestructura y ninguna de dibujo:
//
//   - los bytes de los `.cof`/`.dcc` salen de `mpq.ReadFile` (el recolector de Go los devuelve solo) en vez de
//     la Source de consola con dueno explicito;
//   - el decodificador del `.dcc` NO tiene arena instalada (`d2dcc.PedirDeArena` es nil): en el host el
//     semiespacio no es un recurso escaso, y la arena es una pieza de la consola (DC-002.100);
//   - el pincel es la superficie de software del medidor (`Cuadro`), que cumple el mismo contrato que la del
//     PVR (`RenderIndicesTintado` con el indice 0 transparente).
package main

import (
	"hash/crc32"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2cof"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2mpq"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2map/d2maprenderer"
)

const (
	// los mismos nombres que usa la consola: el COF de la amazona parada y desarmada y la paleta de las unidades
	cofDelJugador = "data\\global\\CHARS\\AM\\COF\\amnuhth.cof"
	palDelJugador = "data/global/palette/Units/pal.dat"

	// el modo y la direccion del ciclo (knobs.go del POC: NU, direccion 0, 128 ms por cuadro)
	modoDelJugadorHost = "NU"
	dirDelJugadorHost  = 0
	msDelJugadorHost   = 128
)

// elJugadorDelHost: el ciclo compuesto y su paleta, listos para pegar en el cuadro.
type elJugadorDelHost struct {
	Ciclo *CicloDelJugador
	Pal   [256][4]uint8
}

// cargarJugadorHost: abre el `d2char.mpq` de `dir` (los `.cof`/`.dcc`), lee la paleta de las unidades del
// `d2data.mpq` (`datos`) y compone el ciclo con el compositor compartido.
func cargarJugadorHost(dir, datos string) (*elJugadorDelHost, error) {
	chars, err := elMPQ(dir, "d2char.mpq")
	if err != nil {
		return nil, err
	}

	fmt.Println("RENDER294: JUGADOR d2char.mpq =", chars)

	// la paleta de las unidades sale del d2data.mpq
	md, err := d2mpq.FromFile(datos)
	if err != nil {
		return nil, fmt.Errorf("abriendo %s: %w", datos, err)
	}

	crudoPal, err := leerDelMPQ(md, palDelJugador)
	if err != nil {
		return nil, fmt.Errorf("leyendo %s: %w", palDelJugador, err)
	}

	_ = md.Close()

	pal, ok := PaletaDelJugador(crudoPal)
	if !ok {
		return nil, fmt.Errorf("la paleta de las unidades no mide 768 B (%d)", len(crudoPal))
	}

	fmt.Printf("RENDER294: JUGADOR PALETA %s bytes=%d primero=%d,%d,%d\n", palDelJugador, len(crudoPal),
		pal[1][0], pal[1][1], pal[1][2])

	mc, err := d2mpq.FromFile(chars)
	if err != nil {
		return nil, fmt.Errorf("abriendo %s: %w", chars, err)
	}

	defer func() { _ = mc.Close() }()

	crudoCof, err := leerDelMPQ(mc, cofDelJugador)
	if err != nil {
		return nil, fmt.Errorf("leyendo %s: %w", cofDelJugador, err)
	}

	cof, err := d2cof.Unmarshal(crudoCof)
	if err != nil {
		return nil, fmt.Errorf("parseando %s: %w", cofDelJugador, err)
	}

	fmt.Printf("RENDER294: JUGADOR COF %s bytes=%d crc32=%d capas=%d framesPorDir=%d dirs=%d speed=%d\n",
		cofDelJugador, len(crudoCof), crc32.ChecksumIEEE(crudoCof), cof.NumberOfLayers,
		cof.FramesPerDirection, cof.NumberOfDirections, cof.Speed)

	leer := func(nombre string) ([]byte, func(), error) {
		datosDelArchivo, err := leerDelMPQ(mc, nombre)
		if err != nil {
			return nil, nil, err
		}

		fmt.Printf("RENDER294: JUGADOR ARCHIVO %s bytes=%d crc32=%d\n", nombre, len(datosDelArchivo),
			crc32.ChecksumIEEE(datosDelArchivo))

		return datosDelArchivo, func() {}, nil
	}

	ciclo := ComponerCicloDelJugador(cof, dirDelJugadorHost, modoDelJugadorHost, msDelJugadorHost, leer)

	for f := range ciclo.Cuadros {
		cu := &ciclo.Cuadros[f]
		fmt.Printf("RENDER294: JUGADOR CUADRO %d de %d caja=%dx%d esquina=%d,%d pixeles=%d crc32=%d "+
			"capasDibujadas=%d sinGrafico=%d fueraDeCaja=%d\n", f, len(ciclo.Cuadros), cu.Ancho, cu.Alto,
			cu.Izq, cu.Arriba, cu.PixelesNoVacios, cu.CRC, cu.CapasDibujadas, cu.CapasSinGrafico,
			cu.CapasFueraDeCaja)
	}

	fmt.Printf("RENDER294: JUGADOR CICLO modo=%s dir=%d cuadros=%d msPorCuadro=%d cicloMs=%d\n", ciclo.Modo,
		ciclo.Dir, len(ciclo.Cuadros), ciclo.MsPorCuadro, len(ciclo.Cuadros)*ciclo.MsPorCuadro)

	return &elJugadorDelHost{Ciclo: ciclo, Pal: pal}, nil
}

// medirElJugadorEnElCuadro: el MISMO procedimiento que `dibujar` del POC de la consola, sobre la superficie de
// software y con la MISMA cuenta de ancla (`AnclaDelJugador` con el `WorldToScreenF` del renderer):
//
//	1. crc32 del cuadro del mapa solo (tiene que ser 1437022778);
//	2. se guarda la caja, se pinta el jugador y se mide el crc32 del cuadro con el, y cuantos pixeles cambio;
//	3. se devuelve la caja al mapa y el crc32 tiene que volver al del mapa solo;
//	4. se vuelve a pintar al jugador: eso es lo que se guarda en el PNG.
func medirElJugadorEnElCuadro(mr *d2maprenderer.MapRenderer, c *Cuadro, j *elJugadorDelHost, indice int,
	cx, cy float64) {
	if j == nil || j.Ciclo == nil || len(j.Ciclo.Cuadros) == 0 {
		return
	}

	cu := &j.Ciclo.Cuadros[indice%len(j.Ciclo.Cuadros)]

	anclaX, anclaY := AnclaDelJugador(mr.WorldToScreenF, cx, cy)
	caja := CajaDe(anclaX, anclaY, cu)

	pixeles := c.Pixeles()
	crcMapa := crc32.ChecksumIEEE(pixeles)

	fmt.Printf("RENDER294: JUGADOR ancla=%d,%d caja=%d,%d,%d,%d tamano=%dx%d compuesto=%dx%d esquina=%d,%d "+
		"pixeles=%d crc32Compuesto=%d\n", anclaX, anclaY, caja.X0, caja.Y0, caja.X1, caja.Y1, caja.Ancho(),
		caja.Alto(), cu.Ancho, cu.Alto, cu.Izq, cu.Arriba, cu.PixelesNoVacios, cu.CRC)

	guardado := guardarCajaHost(pixeles, c.ancho, caja)

	PintarJugador(c, cu, anclaX, anclaY, j.Pal[:])

	crcCon := crc32.ChecksumIEEE(pixeles)
	cambiados := cambiadosEnLaCaja(pixeles, guardado, c.ancho, caja)

	fmt.Printf("RENDER294: CUADRO CON-JUGADOR crc32=%d crc32RGB=%d pixelesDelJugador=%d delArea=%d "+
		"(mapaSolo=%d)\n", crcCon, crcSoloRGB(pixeles), cambiados, caja.Ancho()*caja.Alto(), crcMapa)

	restaurarCajaHost(pixeles, guardado, c.ancho, caja)

	crcRestaurado := crc32.ChecksumIEEE(pixeles)

	fmt.Printf("RENDER294: CUADRO MAPA-RESTAURADO crc32=%d igualAlMapaSolo=%v\n", crcRestaurado,
		crcRestaurado == crcMapa)

	PintarJugador(c, cu, anclaX, anclaY, j.Pal[:])
}

// guardarCajaHost / restaurarCajaHost / cambiadosEnLaCaja: las mismas tres operaciones del POC de la consola,
// con el ancho del lienzo como parametro (en la consola es la constante `ancho`).
func guardarCajaHost(pixeles []byte, ancho int, caja CajaDelJugador) []byte {
	guardado := make([]byte, caja.Ancho()*caja.Alto()*4)
	n := 0

	for y := caja.Y0; y < caja.Y1; y++ {
		for x := caja.X0; x < caja.X1; x++ {
			j := (y*ancho + x) * 4
			copy(guardado[n:n+4], pixeles[j:j+4])

			n += 4
		}
	}

	return guardado
}

func restaurarCajaHost(pixeles, guardado []byte, ancho int, caja CajaDelJugador) {
	n := 0

	for y := caja.Y0; y < caja.Y1; y++ {
		for x := caja.X0; x < caja.X1; x++ {
			j := (y*ancho + x) * 4
			copy(pixeles[j:j+4], guardado[n:n+4])

			n += 4
		}
	}
}

func cambiadosEnLaCaja(pixeles, guardado []byte, ancho int, caja CajaDelJugador) int {
	n := 0
	cambiados := 0

	for y := caja.Y0; y < caja.Y1; y++ {
		for x := caja.X0; x < caja.X1; x++ {
			j := (y*ancho + x) * 4

			if pixeles[j] != guardado[n] || pixeles[j+1] != guardado[n+1] ||
				pixeles[j+2] != guardado[n+2] || pixeles[j+3] != guardado[n+3] {
				cambiados++
			}

			n += 4
		}
	}

	return cambiados
}

// leerDelMPQ: el lector del MPQ es LITERAL con las barras invertidas (y el hash distingue `/` de `\`): un nombre
// con barras normales contesta "file not found" aunque el archivo este ahi. La Source de consola lo limpia sola
// (`fuenteconsola.limpiar`), el MPQ pelado no. Medido en el motor y en `verif-personaje-238/nivel.go`.
func leerDelMPQ(m *d2mpq.MPQ, nombre string) ([]byte, error) {
	return m.ReadFile(strings.TrimPrefix(strings.ReplaceAll(nombre, "/", `\`), `\`))
}

// elMPQ: la ruta real de un MPQ en la carpeta del juego (el juego los tiene en mayusculas o minusculas segun la
// copia), con el mismo criterio que `losDosMPQ`.
func elMPQ(dir, nombre string) (string, error) {
	entradas, err := os.ReadDir(dir)
	if err != nil {
		return "", fmt.Errorf("listando %s: %w", dir, err)
	}

	for _, e := range entradas {
		if strings.EqualFold(e.Name(), nombre) {
			return filepath.Join(dir, e.Name()), nil
		}
	}

	return "", fmt.Errorf("no hay %s en %s", nombre, dir)
}

//go:build dreamcast

// DC-002.112 -- LA CARGA DEL JUGADOR EN LA CONSOLA: del `d2char.mpq` al ciclo compuesto.
//
// Es la mitad que en el host hace `mpq.ReadFile`, con las tres diferencias que la consola obliga y que ya estan
// medidas en DC-002.96/.98/.100:
//
//  1. los bytes se piden a la Source de consola (`Pedir`/`Liberar`), con el DUENO EXPLICITO: el tramo se
//     devuelve en cuanto sus pixeles estan consumidos, no al final de la corrida;
//  2. el liberador es `runtime_FreeExternal` (`d2c_free_external`) y no el `free` pelado: los tramos son
//     `make` de Go y por tamano (> 64 KB) el runtime los mando a `gc_external_alloc`, donde el recolector no
//     los toca. Un `free` suelto deja la entrada en la tabla de externos apuntando a memoria devuelta;
//  3. los buffers del decodificador del `.dcc` salen del MONTON DE C (la arena de DC-002.100): el semiespacio
//     del recolector es 1,8 MB y una direccion de la amazona pide ~1,4 MB de celdas. Con la arena, el criterio
//     es CERO recolecciones durante la decodificacion, y eso es lo que se informa.
//
// Y una cuarta, que es de orden: la arena se libera DESPUES de componer todos los cuadros (los `PixelData` de
// cada cuadro APUNTAN adentro; liberar antes es leer memoria ajena).
package main

import (
	"hash/crc32"
	"runtime"
	"unsafe"

	"github.com/OpenDiablo2/OpenDiablo2/console/dc/fuenteconsola"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2cof"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2dcc"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2asset"
)

// cofEnMPQJugador: el COF de la amazona parada y desarmada (AM + NU + HTH), del `d2char.mpq`.
const cofEnMPQJugador = "data\\global\\CHARS\\AM\\COF\\amnuhth.cof"

// sinArena: LA BASELINE de DC-002.100, para el A/B del decodificador. Con esto encendido el gancho
// `d2dcc.PedirDeArena` NO se instala y los buffers del `.dcc` van al semiespacio del recolector.
var sinArena = false

// umbralGrande: a partir de que tamano el recolector de libgodc manda la asignacion al monton de C (64 KB,
// ver DC-002.37): esas no las recupera el recolector. Es la constante que usa `arena.go` (que es copia de la
// del POC del personaje, donde vive en su `main.go`; aca el main es el del pueblo y no la tenia).
const umbralGrande = 64 * 1024

// uintptrOf: la direccion del primer byte de un tramo, para los puentes de C. La misma que en el POC del
// personaje (`main.go`): un tramo vacio no tiene direccion y devuelve 0.
func uintptrOf(b []byte) uintptr {
	if len(b) == 0 {
		return 0
	}

	return uintptr(unsafe.Pointer(&b[0]))
}

// cargarJugador: compone el ciclo del jugador (modo, direccion y cuadros de los knobs) y deja la paleta en
// `palJugador`. Deja `jugadorOK` en true solo si el ciclo salio con cuadros.
func cargarJugador(am *d2asset.AssetManager) {
	// ============================== 1. LA PALETA DE LAS UNIDADES ==============================
	// Sale del d2data.mpq (que en este punto esta abierto: se suelta recien despues de componer el pueblo).
	datos := fuenteDe(rutaDatos)
	if datos == nil {
		println("JUEGO112: JUGADOR sin fuente de datos: no puedo leer la paleta de las unidades")

		return
	}

	tramoPal, err := datos.Pedir(palUnidadesMPQ)
	if err != nil {
		println("JUEGO112: JUGADOR PALETA-LEER FALLO:", palUnidadesMPQ, err.Error())

		return
	}

	bytesPal := len(tramoPal.Datos)
	pal, okPal := PaletaDelJugador(tramoPal.Datos)
	tramoPal.Liberar()

	println("JUEGO112: JUGADOR PALETA", palUnidadesMPQ, "bytes=", bytesPal, "ok=", okPal,
		"primero=", int(pal[1][0]), int(pal[1][1]), int(pal[1][2]))

	if !okPal {
		return
	}

	palJugador = pal

	// ============================== 2. EL MPQ DE LOS PERSONAJES ==============================
	// Se ABRE solo para esto y se SUELTA (tablas + memoria de trabajo) en cuanto el ciclo esta compuesto: el
	// indice del d2char no tiene por que quedarse tomado mientras se compone el pueblo.
	fuenteChar, err := fuenteconsola.Abrir(rutaMPQChar)
	if err != nil {
		println("JUEGO112: JUGADOR MPQ-ABRIR FALLO:", rutaMPQChar, err.Error())

		return
	}

	// los tramos de esta fuente son `make` de Go: el liberador es el de externos del runtime (ver la cabecera)
	fuenteChar.UsarLiberador(fuenteconsola.Liberador(d2cFreeExternal))

	println("JUEGO112: JUGADOR MPQ-ABRIR OK", fuenteChar.Path(), "montonLibre=", d2cHeapLibre())

	tramoCof, err := fuenteChar.Pedir(cofEnMPQJugador)
	if err != nil {
		println("JUEGO112: JUGADOR COF-LEER FALLO:", err.Error())

		return
	}

	bytesCof, crcCof := len(tramoCof.Datos), crc32.ChecksumIEEE(tramoCof.Datos)

	cof, err := d2cof.Unmarshal(tramoCof.Datos)
	tramoCof.Liberar()

	if err != nil {
		println("JUEGO112: JUGADOR COF-PARSE FALLO:", err.Error())

		return
	}

	println("JUEGO112: JUGADOR COF", cofEnMPQJugador, "bytes=", bytesCof, "crc32=", int(crcCof),
		"capas=", cof.NumberOfLayers, "framesPorDir=", cof.FramesPerDirection, "dirs=", cof.NumberOfDirections,
		"speed=", cof.Speed)

	if dirDelJugador < 0 || dirDelJugador >= cof.NumberOfDirections {
		println("JUEGO112: JUGADOR direccion fuera del COF:", dirDelJugador, "de", cof.NumberOfDirections)

		return
	}

	// ============================== 3. EL CICLO, CON LA ARENA DEL DECODIFICADOR ==============================
	arena := &arenaDCC{}
	conArena(arena)

	coleccionesAntes := d2cGcN()

	// pedidas: cuantas recolecciones pide ESTE archivo. Es la pieza que separa "el decodificador
	// lleno el semiespacio" de "yo barri a proposito despues de cada capa": el numero que se publica es
	// automaticas = total - pedidas, y tiene que ser CERO (DC-002.98/.100).
	pedidas := 0

	lector := func(nombre string) ([]byte, func(), error) {
		tramo, err := fuenteChar.Pedir(nombre)
		if err != nil {
			return nil, nil, err
		}

		datosDelArchivo := tramo.Datos

		println("JUEGO112: JUGADOR ARCHIVO", nombre, "bytes=", len(datosDelArchivo),
			"crc32=", int(crc32.ChecksumIEEE(datosDelArchivo)))

		// el que suelta: el archivo vuelve al monton en el acto y el recolector barre lo que el decodificador
		// dejo suelto (las otras 15 direcciones). Sin esa recoleccion la carga del ciclo se vuelve 40 veces mas
		// lenta (medido en DC-002.38: 42.608 ms contra 652 ms).
		return datosDelArchivo, func() {
			tramo.Liberar()

			pedidas++

			runtime.GC()
		}, nil
	}

	ciclo := ComponerCicloDelJugador(cof, dirDelJugador, modoDelJugador, msDelJugador, lector)

	// LA ARENA SE LIBERA ACA: los `PixelData` vivian adentro y recien ahora estan pegados en cada compuesto.
	arena.liberar()
	d2dcc.PedirDeArena = nil

	colecciones := d2cGcN() - coleccionesAntes

	println("JUEGO112: JUGADOR ARENA pedidos=", arena.pedidos, "entregado=", arena.entregado,
		"tramos=", arena.liberados, "capacidad=", arena.capacidad(),
		"montonCLibre=", int(d2cHeapLibre()), "recoleccionesEnElCiclo=", int(colecciones),
		"pedidasPorMi=", pedidas, "automaticas=", int(colecciones)-pedidas, "sinArena=", sinArena)

	cof = nil

	runtime.GC()

	// ============================== 4. LO QUE QUEDO COMPUESTO ==============================
	if len(ciclo.Cuadros) == 0 {
		println("JUEGO112: JUGADOR EL CICLO SALIO VACIO")

		return
	}

	for f := range ciclo.Cuadros {
		cu := &ciclo.Cuadros[f]
		println("JUEGO112: JUGADOR CUADRO", f, "de", len(ciclo.Cuadros), "caja=", cu.Ancho, "x", cu.Alto,
			"esquina=", cu.Izq, cu.Arriba, "pixeles=", cu.PixelesNoVacios, "crc32=", int(cu.CRC),
			"capasDibujadas=", cu.CapasDibujadas)
	}

	jugador = ciclo
	jugadorOK = true

	println("JUEGO112: JUGADOR CICLO modo=", ciclo.Modo, "dir=", ciclo.Dir, "cuadros=", len(ciclo.Cuadros),
		"msPorCuadro=", ciclo.MsPorCuadro, "cicloMs=", len(ciclo.Cuadros)*ciclo.MsPorCuadro)

	// ============================== 5. EL d2char SE SUELTA ==============================
	r := fuenteChar.Soltar(func(tramo uintptr, bytes int) int { return d2cSoltarTramo(tramo, bytes) })

	println("JUEGO112: JUGADOR FUENTE SOLTADA", r.Ruta, "tablas=", r.Tablas.Total, "bytesTablas=", r.BytesTablas,
		"bytesTrabajo=", r.BytesTrabajo, "bytes=", r.Bytes, "yaSoltada=", r.YaSoltada,
		"montonLibre=", int(d2cHeapLibre()))

	runtime.GC()
}

// fuenteDe: la Source de consola abierta sobre la ruta pedida (las de `fuentesDeConsola`), o nil.
func fuenteDe(ruta string) *fuenteconsola.Fuente {
	for _, f := range fuentesDeConsola {
		if f.Path() == ruta {
			return f
		}
	}

	return nil
}

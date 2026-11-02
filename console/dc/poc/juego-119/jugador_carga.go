//go:build dreamcast

// DC-002.113 -- LA CARGA DEL JUGADOR QUE CAMINA: del `d2char.mpq` a los ciclos compuestos.
//
// Es la carga de DC-002.112 (`jugador_carga.go`, la misma cadena: Source de consola con dueno explicito,
// `runtime_FreeExternal` para devolver los tramos, arena del decodificador en el monton de C) con DOS diferencias:
//
//  1. se componen DOS modos: el QUIETO (NU, direccion 0, el cuadro de DC-002.112) y el de CAMINAR EN EL PUEBLO
//     (TW, `AMTWhth.COF`), porque el motor cambia de modo cuando el heroe se mueve (en el pueblo: TN quieto,
//     TW caminando). Los dos modos tienen su propio `.cof` y sus propias capas `.dcc`.
//  2. del modo de caminar se componen TODAS las direcciones que la cruceta puede pedir (8, medido en DC-002.49b:
//     [0 1 4 7 8 9 12 15]) y no una: el giro despues es elegir otro ciclo ya compuesto, 0 ms de cuadro. Esa lista
//     sale de `DireccionesDelControl()` (la cadena del motor), no de una tabla escrita aca.
//
// LA CUENTA DE MEMORIA, medida en el host en DC-002.49b: las 8 direcciones de TW son 148.720 B de indices
// (1 B/pixel, el indice 0 transparente). El semiespacio del recolector son 1.835.008 B y en el cuadro queda al
// ~74 %, asi que esos 145 KB son la pieza que se mira con la lupa: el POC informa el semiespacio antes y despues
// de cada modo, y `dirsCaminando` deja bajar el numero para el A/B.
//
// Y la regla de orden de DC-002.100 se mantiene: la arena del decodificador se libera DESPUES de componer todos
// los cuadros del modo (los `PixelData` de cada cuadro APUNTAN adentro).
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

// Los dos COF de la amazona desarmada, del `d2char.mpq`: el neutral (DC-002.112) y el de caminar en el pueblo.
const (
	cofQuietoJugador    = "data\\global\\CHARS\\AM\\COF\\amnuhth.cof"
	cofCaminandoJugador = "data\\global\\CHARS\\AM\\COF\\amtwhth.cof"
)

// sinArena: LA BASELINE de DC-002.100, para el A/B del decodificador (con esto encendido el gancho
// `d2dcc.PedirDeArena` NO se instala y los buffers del `.dcc` van al semiespacio del recolector).
var sinArena = false

// umbralGrande: a partir de que tamano el recolector de libgodc manda la asignacion al monton de C (64 KB).
// Es la constante que usa `arena.go`.
const umbralGrande = 64 * 1024

// uintptrOf: la direccion del primer byte de un tramo, para los puentes de C.
func uintptrOf(b []byte) uintptr {
	if len(b) == 0 {
		return 0
	}

	return uintptr(unsafe.Pointer(&b[0]))
}

// cargarJugador: compone el cuadro QUIETO (NU dir 0) y los ciclos de CAMINAR (TW, una por direccion de la
// cruceta) y deja la paleta de las unidades en `palJugador`. Deja `jugadorOK` en true solo si los dos modos
// salieron con cuadros.
func cargarJugador(am *d2asset.AssetManager) {
	// ============================== 1. LA PALETA DE LAS UNIDADES ==============================
	datos := fuenteDe(rutaDatos)
	if datos == nil {
		println("JUEGO119: JUGADOR sin fuente de datos: no puedo leer la paleta de las unidades")

		return
	}

	tramoPal, err := datos.Pedir(palUnidadesMPQ)
	if err != nil {
		println("JUEGO119: JUGADOR PALETA-LEER FALLO:", palUnidadesMPQ, err.Error())

		return
	}

	bytesPal := len(tramoPal.Datos)
	pal, okPal := PaletaDelJugador(tramoPal.Datos)
	tramoPal.Liberar()

	println("JUEGO119: JUGADOR PALETA", palUnidadesMPQ, "bytes=", bytesPal, "ok=", okPal,
		"primero=", int(pal[1][0]), int(pal[1][1]), int(pal[1][2]))

	if !okPal {
		return
	}

	palJugador = pal

	// ============================== 2. EL MPQ DE LOS PERSONAJES ==============================
	fuenteChar, err := fuenteconsola.Abrir(rutaMPQChar)
	if err != nil {
		println("JUEGO119: JUGADOR MPQ-ABRIR FALLO:", rutaMPQChar, err.Error())

		return
	}

	fuenteChar.UsarLiberador(fuenteconsola.Liberador(d2cFreeExternal))

	println("JUEGO119: JUGADOR MPQ-ABRIR OK", fuenteChar.Path(), "montonLibre=", int(d2cHeapLibre()))

	lector := lectorDeLaFuente(fuenteChar)

	// ============================== 3. EL CUADRO QUIETO (NU, dir 0) ==============================
	jugadorQuieto = componerModo(fuenteChar, lector, cofQuietoJugador, modoQuieto, msQuieto, []int{dirQuieto})

	// ============================== 4. LOS CICLOS DE CAMINAR (TW, las 8 direcciones) ==============================
	direcciones := DireccionesDelControl()

	println("JUEGO119: JUGADOR DIRECCIONES de la cruceta", len(direcciones), "compuestas", dirsCaminando)

	if dirsCaminando > 0 && dirsCaminando < len(direcciones) {
		direcciones = direcciones[:dirsCaminando]
	}

	jugadoresCaminando = make(map[int]*CicloDelJugador, len(direcciones))

	for _, dir := range direcciones {
		ciclo := componerModo(fuenteChar, lector, cofCaminandoJugador, modoCaminando, msCaminando, []int{dir})
		if ciclo == nil || len(ciclo.Cuadros) == 0 {
			println("JUEGO119: JUGADOR la direccion", dir, "salio vacia")

			continue
		}

		jugadoresCaminando[dir] = ciclo
		direccionesCompuestas = append(direccionesCompuestas, dir)

		println("JUEGO119: JUGADOR DIR", dir, "cuadros=", len(ciclo.Cuadros), "caja=", ciclo.Cuadros[0].Ancho,
			"x", ciclo.Cuadros[0].Alto, "indices=", bytesDeLosCuadros(ciclo))
	}

	// ============================== 5. LO QUE QUEDO COMPUESTO ==============================
	jugadorOK = jugadorQuieto != nil && len(jugadorQuieto.Cuadros) > 0 && len(jugadoresCaminando) > 0

	indices := 0
	if jugadorQuieto != nil {
		indices += bytesDeLosCuadros(jugadorQuieto)
	}

	for _, c := range jugadoresCaminando {
		indices += bytesDeLosCuadros(c)
	}

	println("JUEGO119: JUGADOR CICLOS quieto=", quietoModo(), "quietoDir=", quietoDir(), "quietoCuadros=",
		quietoCuadros(), "caminando=", len(jugadoresCaminando), "direcciones=", len(direccionesCompuestas),
		"indicesTotales=", indices, "jugadorOK=", jugadorOK)

	// ============================== 6. EL d2char SE SUELTA ==============================
	r := fuenteChar.Soltar(func(tramo uintptr, bytes int) int { return d2cSoltarTramo(tramo, bytes) })

	println("JUEGO119: JUGADOR FUENTE SOLTADA", r.Ruta, "tablas=", r.Tablas.Total, "bytesTablas=", r.BytesTablas,
		"bytesTrabajo=", r.BytesTrabajo, "bytes=", r.Bytes, "yaSoltada=", r.YaSoltada,
		"montonLibre=", int(d2cHeapLibre()))

	runtime.GC()
}

// lectorDeLaFuente: como se piden los bytes de un COF/DCC en la consola (el `Lector` del archivo compartido).
// El que suelta devuelve el archivo al monton en el acto y le pide al recolector que barra lo que el decodificador
// dejo suelto (las otras 15 direcciones): sin esa recoleccion la carga se vuelve 40 veces mas lenta.
func lectorDeLaFuente(fuente *fuenteconsola.Fuente) Lector {
	return func(nombre string) ([]byte, func(), error) {
		tramo, err := fuente.Pedir(nombre)
		if err != nil {
			return nil, nil, err
		}

		datosDelArchivo := tramo.Datos

		println("JUEGO119: JUGADOR ARCHIVO", nombre, "bytes=", len(datosDelArchivo),
			"crc32=", int(crc32.ChecksumIEEE(datosDelArchivo)))

		return datosDelArchivo, func() {
			tramo.Liberar()
			runtime.GC()
		}, nil
	}
}

// componerModo: abre el COF del modo, compone las direcciones pedidas (cada una con la arena del decodificador) y
// devuelve el ciclo de la PRIMERA direccion. `dirs` de mas de uno no se usa hoy; queda por si otra tarjeta quiere
// el ciclo de varias direcciones en una sola lectura.
func componerModo(fuente *fuenteconsola.Fuente, leer Lector, rutaCOF, modo string, msPorCuadro int,
	dirs []int) *CicloDelJugador {
	tramoCof, err := fuente.Pedir(rutaCOF)
	if err != nil {
		println("JUEGO119: JUGADOR COF-LEER FALLO:", rutaCOF, err.Error())

		return nil
	}

	bytesCof, crcCof := len(tramoCof.Datos), crc32.ChecksumIEEE(tramoCof.Datos)

	cof, err := d2cof.Unmarshal(tramoCof.Datos)
	tramoCof.Liberar()

	if err != nil {
		println("JUEGO119: JUGADOR COF-PARSE FALLO:", rutaCOF, err.Error())

		return nil
	}

	println("JUEGO119: JUGADOR COF", rutaCOF, "bytes=", bytesCof, "crc32=", int(crcCof),
		"capas=", cof.NumberOfLayers, "framesPorDir=", cof.FramesPerDirection, "dirs=", cof.NumberOfDirections,
		"speed=", cof.Speed, "modo=", modo)

	var primero *CicloDelJugador

	for i, dir := range dirs {
		if dir < 0 || dir >= cof.NumberOfDirections {
			println("JUEGO119: JUGADOR direccion fuera del COF:", dir, "de", cof.NumberOfDirections)

			continue
		}

		arena := &arenaDCC{}
		conArena(arena)

		coleccionesAntes := d2cGcN()
		ciclo := ComponerCicloDelJugador(cof, dir, modo, msPorCuadro, leer)

		// LA ARENA SE LIBERA ACA: los `PixelData` vivian adentro y recien ahora estan pegados en cada compuesto.
		arena.liberar()
		d2dcc.PedirDeArena = nil

		colecciones := d2cGcN() - coleccionesAntes

		println("JUEGO119: JUGADOR ARENA modo=", modo, "dir=", dir, "pedidos=", arena.pedidos,
			"entregado=", arena.entregado, "tramos=", arena.liberados, "montonCLibre=", int(d2cHeapLibre()),
			"recolecciones=", int(colecciones), "sinArena=", sinArena)

		if len(ciclo.Cuadros) == 0 {
			println("JUEGO119: JUGADOR EL CICLO SALIO VACIO modo=", modo, "dir=", dir)

			continue
		}

		for f := range ciclo.Cuadros {
			cu := &ciclo.Cuadros[f]
			println("JUEGO119: JUGADOR CUADRO modo=", modo, "dir=", dir, "cuadro=", f, "de", len(ciclo.Cuadros),
				"caja=", cu.Ancho, "x", cu.Alto, "esquina=", cu.Izq, cu.Arriba, "pixeles=", cu.PixelesNoVacios,
				"crc32=", int(cu.CRC), "capasDibujadas=", cu.CapasDibujadas, "sinGrafico=", cu.CapasSinGrafico)
		}

		ciclo.Dir = dir

		if i == 0 {
			primero = ciclo
		} else {
			// el que llama pidio varias direcciones: se guardan todas en el mapa de caminar
			if jugadoresCaminando == nil {
				jugadoresCaminando = make(map[int]*CicloDelJugador, len(dirs))
			}

			jugadoresCaminando[dir] = ciclo
		}

		runtime.GC()
	}

	cof = nil

	return primero
}

// bytesDeLosCuadros: cuantos bytes de indices ocupa un ciclo compuesto (1 B/pixel), que es el numero que se
// compara con los 148.720 B de las 8 direcciones de TW medidos en el host (DC-002.49b).
func bytesDeLosCuadros(ciclo *CicloDelJugador) int {
	if ciclo == nil {
		return 0
	}

	total := 0

	for f := range ciclo.Cuadros {
		total += len(ciclo.Cuadros[f].Pixeles)
	}

	return total
}

// nombreDelCiclo: los cuatro numeros del ciclo quieto, uno por linea de println (la consola no tiene fmt).
func quietoModo() string {
	if jugadorQuieto == nil {
		return "-"
	}

	return jugadorQuieto.Modo
}

func quietoDir() int {
	if jugadorQuieto == nil {
		return -1
	}

	return jugadorQuieto.Dir
}

func quietoCuadros() int {
	if jugadorQuieto == nil {
		return 0
	}

	return len(jugadorQuieto.Cuadros)
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

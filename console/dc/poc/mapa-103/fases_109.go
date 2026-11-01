//go:build dreamcast

// DC-002.109 -- LAS TRES FASES DEL CAMBIO DE NIVEL: ENTRAR / TRANSICION / VOLVER.
//
// QUE MIDE. DC-002.106 cerro el cuadro del pueblo (crc32 1437022778, pico 11.031.920 B) soltando despues de
// componer las filas de las tablas y las dos fuentes MPQ (570.720 B medidos). Lo que quedo sin medir es el
// CAMBIO DE NIVEL, que es lo mismo pero al reves: irse a otro `.ds1` y volver obliga a REABRIR las dos fuentes
// y RECARGAR las tablas, con el tramo de la cache todavia tomado. Esta corrida hace el ciclo entero en una
// sola corrida de Flycast:
//
//	ENTRAR      el pueblo (`TownN1.ds1`, region 1/preset 1/archivo 0). Es el cuadro 1 de DC-002.106: lo dibuja
//	            `dibujar` como siempre y de ahi sale el crc32 de referencia (1437022778).
//	TRANSICION  OTRO `.ds1` del mismo nivel: el preset 1 del pueblo trae cuatro archivos (TownN1/TownE1/TownS1/
//	            TownW1, medido en el host con `console/dc/medidas/presets-scratch`), asi que el archivo 1 es un
//	            mapa DISTINTO sin cambiar de level type ni de paleta. Se reabren las fuentes y las tablas, se
//	            compone, se cuelga el nivel en el renderer y se dibuja un cuadro.
//	VOLVER      el pueblo otra vez (preset 1, archivo 0), con el tramo reusado desde el byte 0: el crc32 tiene
//	            que ser OTRA VEZ 1437022778.
//
// LA DECISION DE LA TARJETA (a o b), y como se mide. Los dos ordenes estan en el mismo binario y se eligen con
// `ordenDeLasFases` (knobs.go):
//
//	a  soltar la cache del nivel que se deja ANTES de reabrir las fuentes (`SoltarLaCacheDelNivel`, con el
//	   liberador del tramo: `pvr.VramSoltar` en la VRAM, `d2c_soltar_bloque` en el monton de C)
//	b  no soltar nada: la cache vieja sigue tomada mientras se reabre y se compone, y el reinicio del tramo
//	   pasa recien al colgar el nivel nuevo en el renderer (`SetMapEngine`)
//
// MEDIDO EN FLYCAST (evidencia/dc-002.109): el que se elige es el (a), por tres numeros:
//  1. en el MODO DEL MONTON (ENVRAM=0) es el unico que existe: con la cache vieja devuelta la transicion compone
//     (379.792 B libres), pero al pedir la cache del nivel nuevo (2.652.160 B) no hay donde -- OOM-throw en el
//     primer cuadro del nivel nuevo. La cache y el nivel compuesto no conviven en el monton.
//  2. con la cache en la VRAM los dos ordenes dibujan la transicion (crc32 584380401, el mismo del host), pero el
//     (b) deja la mitad de la VRAM: 787.880 B disponibles contra 1.422.824 B del (a), y 31.000 B mas de
//     semiespacio (1.736.912 contra 1.705.920).
//  3. el monton de C nunca es la pared en el (a): despues de soltar las tablas y las fuentes del cuadro quedan
//     949.688 B libres.
//
// Donde SI se para el ciclo -- y es la tarjeta siguiente -- es en el SEMIESPACIO del recolector (1.835.008 B) al
// volver: un nivel compuesto deja ~413.000 B vivos (el `.ds1` del nivel que se deja) que sobreviven a una
// recoleccion completa, y la vuelta necesita ~382.000 B mas para sus casillas (1.735.048 + 382.000 > 1.835.008).
// Por eso este archivo instrumenta el semiespacio con `gc_stats` (`d2c_gc_usado/total/colecciones`) ademas del
// monton de C.
//
// El log de cada fase es el que pide la tarjeta: `MAPA103: MEMORIA` (usado/libre/arena/sbrk/techo/pico), el
// censo del tramo (`MAPA103: TRAMO`), el semiespacio (`MAPA103: SEMIESPACIO`) y el `MAPA103: CUADRO` con crc32,
// crc32RGB y ms. Donde se corta el log ES la respuesta.
package main

import (
	"hash/crc32"
	"runtime"
	"unsafe"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2ds1"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2interface"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2asset"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2map/d2mapengine"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2render/pvr"
)

//extern d2c_soltar_bloque
func d2cSoltarBloque(p uintptr, tamano int) int

//extern d2c_gc_usado
func d2cGcUsado() uint

//extern d2c_gc_total
func d2cGcTotal() uint

//extern d2c_gc_colecciones
func d2cGcColecciones() uint

//extern d2c_gc_percent
func d2cGcPercent(nuevo int) int

var (
	// gestorDeAssets y motorDelMapa: las dos piezas que el ciclo de fases necesita volver a tocar. `main` las
	// guarda aca al crearlas (es el mismo objeto, no una copia).
	gestorDeAssets *d2asset.AssetManager
	motorDelMapa   *d2mapengine.MapEngine

	// picoEnLaFase: el pico de `usado` desde que arranco la fase que se esta midiendo. `picoMonton` es el de la
	// corrida entera.
	picoEnLaFase int

	// fasesHechas: el ciclo corre UNA vez, en el cuadro 1 (despues de soltar el arena, que es el estado que
	// dejo medido DC-002.106).
	fasesHechas bool

	// crcDelCuadroAlEntrar / pixelesDelCuadro1: lo que dejo medido el cuadro 1 (ENTRAR). El de VOLVER tiene
	// que dar el mismo crc32.
	crcDelCuadroAlEntrar uint32
	pixelesDelCuadro1    int
)

// fase: lo que se mide de una fase, para el resumen del final.
type fase struct {
	nombre   string
	libre    int
	pico     int
	crc      uint32
	crcRGB   uint32
	pixeles  int
	ms       int
	pedazos  int
	tramoUsd int
	ok       bool
}

// soltarBloqueDeLaCache (DC-002.109): de donde salen y a donde vuelven los bloques del tramo de la cache, en
// esta corrida. Es el mismo reparto que arma el tramo: si los bloques se pidieron a la VRAM del PVR, se
// devuelven ahi (`pvr.VramSoltar`); si salieron del monton de C (`make` de mas de 64 KB), se devuelven con el
// puente del shim (`d2c_soltar_bloque`, que es `gc_external_free`: saca el bloque de la tabla de externos y le
// hace free).
func soltarBloqueDeLaCache(bloque []byte) int {
	if len(bloque) == 0 {
		return 0
	}

	if tramoDeLaCache > 0 && tramoEnPedazos > 0 && cacheEnVram {
		return pvr.VramSoltar(bloque)
	}

	return d2cSoltarBloque(uintptr(unsafe.Pointer(&bloque[0])), len(bloque))
}

// cicloDeNiveles: las tres fases en una sola corrida. Se llama UNA vez, desde `dibujar`, cuando el cuadro 1 ya
// se dibujo y el arena de bloques ya se solto (el estado medido por DC-002.106).
func cicloDeNiveles(superficie d2interface.Surface) {
	if fasesHechas {
		return
	}

	fasesHechas = true

	println("MAPA109: CICLO DE FASES arranca ordenDeLasFases=", ordenDeLasFases,
		"transicion=preset", transicionPreset, "archivo", transicionArchivo,
		"cacheEnVram=", cacheEnVram, "tramoEnPedazos=", tramoEnPedazos)

	// el umbral del recolector para todo el ciclo (ver knobs.go: sin esto la TRANSICION no avanza)
	gcPercentAntes := d2cGcPercent(gcPercentDelCiclo)

	println("MAPA109: CICLO gcPercent", gcPercentAntes, "->", gcPercentDelCiclo,
		"SEMIESPACIO total=", int(d2cGcTotal()))
	semiespacio("ciclo-con-el-umbral-puesto")

	picoDeEntrar := picoMonton
	memoria("ciclo-antes-de-la-transicion")
	semiespacio("ciclo-antes-de-la-transicion")
	censoDelTramo("ciclo-antes-de-la-transicion")
	vram("ciclo-antes-de-la-transicion")

	entrar := fase{nombre: "ENTRAR", libre: d2cMontonLibre(), pico: picoDeEntrar, crc: crcDelCuadroAlEntrar,
		pixeles: pixelesDelCuadro1, ok: true}
	tramo := rendererDelMapa.CensoDelTramo()

	entrar.tramoUsd, entrar.pedazos = tramo.Usado, tramo.Pedazos

	transicion := correrFase(superficie, "TRANSICION", transicionPreset, transicionArchivo)
	volver := correrFase(superficie, "VOLVER", presetA, archivoA)

	println("MAPA109: CICLO resumen:")
	imprimeFase(entrar)
	imprimeFase(transicion)
	imprimeFase(volver)
	println("MAPA109: CICLO gcPercent de vuelta a", d2cGcPercent(gcPercentAntes))
	println("MAPA109: CICLO FIN pico de la corrida=", picoMonton, "de arena=", d2cMontonArena(),
		"libre=", d2cMontonLibre(), "sbrk=", int(d2cSbrkActual()), "techo=", int(d2cMontonTecho()))

	// DC-002.110: el censo de la arena del `.ds1`. `vivos` tiene que ser 0 (cada bloque devuelto con el
	// liberador); `pedidas` = 3 si las tres fases se compusieron (una por nivel) y `soltadas` igual.
	arena := d2ds1.EstadoDeLaArena()
	println("MAPA110: ARENA DEL DS1 pedidas=", arena.Pedidas, "soltadas=", arena.Soltadas,
		"bytesPedidos=", arena.Bytes, "bytesDevueltos=", arena.Devueltos, "vivos=", arena.Vivos,
		"chicas=", arena.Chicas, "sinLugar=", arena.SinLugar, "celdasFuera=", arena.Celdas,
		"capasFuera=", arena.Capas, "bytesPorCelda=", d2ds1.BytesPorCelda())

	if volver.ok && volver.crc == crcDelCuadroAlEntrar {
		println("MAPA109: CRITERIO el cuadro de VOLVER es el mismo que el de ENTRAR crc32=", int(volver.crc),
			"(1437022778)")
	} else if volver.ok {
		println("MAPA109: CRITERIO el cuadro de VOLVER NO es el de ENTRAR: volver=", int(volver.crc),
			"entrar=", int(crcDelCuadroAlEntrar))
	} else {
		println("MAPA109: CRITERIO la fase VOLVER no llego a dibujar")
	}
}

// correrFase: reabre fuentes y tablas, compone el nivel `preset/archivo`, lo cuelga del renderer y dibuja UN
// cuadro. Devuelve lo medido (con ok = false si se corto antes de dibujar).
func correrFase(superficie d2interface.Surface, nombre string, preset, archivo int) fase {
	f := fase{nombre: nombre}
	picoEnLaFase = 0

	println("MAPA109: FASE", nombre, "ARRANCA ds1=preset", preset, "archivo", archivo,
		"libre=", d2cMontonLibre(), "usado=", d2cMontonUsado())
	memoria(nombre + "-antes")
	censoDelTramo(nombre + "-antes")
	semiespacio(nombre + "-antes")

	// ============================== EL ORDEN (a o b) ==============================
	if ordenDeLasFases == "a" {
		bytes := rendererDelMapa.SoltarLaCacheDelNivel(soltarBloqueDeLaCache)

		println("MAPA109: FASE", nombre, "ORDEN a -- cache del nivel que se deja SOLTADA ANTES de reabrir bytes=",
			bytes, "libre=", d2cMontonLibre())
		censoDelTramo(nombre + "-despues-de-soltar-la-cache")
		vram(nombre + "-despues-de-soltar-la-cache")
		memoria(nombre + "-despues-de-soltar-la-cache")
	} else {
		println("MAPA109: FASE", nombre, "ORDEN b -- la cache del nivel que se deja QUEDA TOMADA durante la "+
			"reapertura y la composicion (se reinicia al colgar el nivel nuevo)")
	}

	// ============================== REABRIR FUENTES Y TABLAS ==============================
	if !reabrirFuentesYTablas(nombre) {
		println("MAPA109: FASE", nombre, "NO se pudieron reabrir las fuentes o las tablas")

		return f
	}

	memoria(nombre + "-despues-de-reabrir")

	// ============================== COMPONER EL NIVEL NUEVO ==============================
	// ANTES DE COMPONER: el nivel anterior se tira de verdad. `ReiniciarArenaDeTiles` vacia el cache de
	// `.dt1` del nivel que se va (sus entradas viven en el SEMIESPACIO del recolector y son lo que el nivel
	// nuevo no tiene lugar de pagar) y un `runtime.GC()` las devuelve ANTES de parsear el `.ds1` nuevo -- el
	// mismo instrumento que `main` usa antes de crear el motor. Medido sin esto: la TRANSICION moria en el
	// semiespacio (1.835.008 B) con el monton de C sano.
	// el nivel que se va: arena reiniciada, casillas en cero y el recolector barriendo lo que quedo suelto
	gestorDeAssets.ReiniciarArenaDeTiles()
	motorDelMapa.SoltarLasCasillasDelNivelAnterior()
	semiespacio(nombre + "-despues-de-soltar-las-casillas")
	runtime.GC()
	semiespacio(nombre + "-antes-de-componer")

	if _, _, _, ok := componerNivel(motorDelMapa, preset, archivo, nombre); !ok {
		println("MAPA109: FASE", nombre, "LoadStamp devolvio nil")

		return f
	}

	semiespacio(nombre + "-despues-de-componer")

	estado := d2asset.EstadoDeLosTiles()
	println("MAPA103: FASE", nombre, "ARENA DE BLOQUES usados=", estado.BloquesUsados, "bytes=", estado.BloquesBytes,
		"tramos=", estado.BloquesTramos, "capacidad=", estado.BloquesCapacidad, "scratch=", estado.Scratch)
	memoria(nombre + "-despues-de-componer")

	// ============================== COLGAR EL NIVEL NUEVO ==============================
	// `SetMapEngine` es el camino del motor para un nivel nuevo: `generateTileCache` tira las imagenes y la
	// paleta del nivel anterior y reinicia el tramo (byte 0). En el orden (a) el tramo ya esta vacio; en el (b)
	// el reinicio pasa ACA, o sea despues de la composicion -- que es lo que la tarjeta quiere medir.
	rendererDelMapa.SetMapEngine(motorDelMapa)
	censoDelTramo(nombre + "-despues-de-colgar")
	memoria(nombre + "-despues-de-colgar")
	semiespacio(nombre + "-despues-de-colgar")

	// ============================== LO QUE EL CUADRO NO LEE ==============================
	// Igual que en el cuadro 1: con el nivel compuesto, las filas de las tablas y las dos fuentes no las mira
	// nadie y son lo que hay que devolver para que entre la cache del cuadro. Y es lo que obliga a la fase
	// siguiente a REABRIR, que es el punto de la tarjeta.
	soltarLoQueElCuadroNoLee(gestorDeAssets)

	// ============================== EL CUADRO DE LA FASE ==============================
	f.libre = d2cMontonLibre()
	f.ms, f.crc, f.crcRGB, f.pixeles, f.ok = cuadroDeLaFase(superficie, nombre)
	f.pico = picoEnLaFase

	tramo := rendererDelMapa.CensoDelTramo()
	f.tramoUsd, f.pedazos = tramo.Usado, tramo.Pedazos

	semiespacio(nombre + "-fin")
	println("MAPA109: FASE", nombre, "FIN libre=", d2cMontonLibre(), "picoDeLaFase=", f.pico,
		"picoDeLaCorrida=", picoMonton, "ok=", f.ok)

	return f
}

// cuadroDeLaFase: limpia el lienzo (el del cuadro 1 arranco en cero, y el crc32 solo se puede comparar contra
// un lienzo en la misma condicion), dibuja y mide.
func cuadroDeLaFase(superficie d2interface.Surface, nombre string) (ms int, crc, crcRGB uint32, llenos int, ok bool) {
	malla, esPvr := superficie.(*pvr.Surface)
	if !esPvr {
		println("MAPA109: FASE", nombre, "la superficie no es del PVR")

		return 0, 0, 0, 0, false
	}

	malla.ClearRGBA(0, 0, 0, 0)
	println("MAPA109: FASE", nombre, "LIENZO limpio (0,0,0,0) antes del cuadro, como el del cuadro 1")
	vram(nombre + "-antes-del-cuadro")
	memoria(nombre + "-antes-del-cuadro")

	t0 := d2cMs()
	rendererDelMapa.Render(superficie)
	ms = int(d2cMs() - t0)

	pixeles := malla.Imagen().Pixeles()

	for j := 0; j+3 < len(pixeles); j += 4 {
		if pixeles[j] != 0 || pixeles[j+1] != 0 || pixeles[j+2] != 0 {
			llenos++
		}
	}

	crc = crc32.ChecksumIEEE(pixeles)
	crcRGB = crcSoloRGB(pixeles)

	cache := rendererDelMapa.EstadoDeLaCache()
	println("MAPA103: CUADRO", nombre, "pixelesNoVacios=", llenos, "de", ancho*alto, "crc32=", int(crc),
		"crc32RGB=", int(crcRGB), "hostPixeles=", pixelesHost, "hostCrc=", int(crcHost), "ms=", ms,
		"cacheSuperficies=", cache.Superficies, "cacheBytes=", cache.Bytes, "cacheEnIndices=", cache.EnIndices)
	censoDelTramo(nombre + "-despues-del-cuadro")
	vram(nombre + "-despues-del-cuadro")
	memoria(nombre + "-despues-del-cuadro")

	return ms, crc, crcRGB, llenos, true
}

// reabrirFuentesYTablas (DC-002.109): el camino de vuelta de DC-002.105 -- `Soltar` deja la fuente SIN MPQ y
// sin tablas, y su contrato dice que para volver a cargar assets hay que ABRIRLA OTRA VEZ. Se abren las dos,
// se registran PRIMERAS en el loader (las muertas devuelven `Exists` = false y el loader las saltea) y se
// vuelven a cargar las dos tablas del compositor.
func reabrirFuentesYTablas(nombre string) bool {
	usado := d2cMontonUsado()

	fuentesDeConsola = nil

	for _, ruta := range []string{rutaDatos, rutaParche} { // el parche queda PRIMERO: se registra al final
		if !registrarFuente(gestorDeAssets, ruta) {
			return false
		}
	}

	tablasCargadas = nil

	cargadas := 0

	for i := 0; i < tablasQueSeCargan && i < len(tablasDelCompositor); i++ {
		var ok bool

		if modoTablas == "stream" {
			ok = cargarTablaPorStream(gestorDeAssets, tablasDelCompositor[i])
		} else {
			ok = cargarTablaPorLaFuente(gestorDeAssets, tablasDelCompositor[i])
		}

		if ok {
			cargadas++
			tablasCargadas = append(tablasCargadas, tablasDelCompositor[i])
		}
	}

	// la paleta del acto y su PL2 los lee el renderer al armar la cache. En el cuadro 1 se precargan ANTES de
	// soltar las fuentes y quedan en el cache del AssetManager; se vuelven a pedir igual (con el MPQ abierto) y
	// si ya estaban el delta es 0: es el instrumento que dice si el cache sobrevivio al cambio de nivel.
	precargarLoQueElCuadroLee(gestorDeAssets)

	println("MAPA109: FASE", nombre, "REABIERTO fuentes=", len(fuentesDeConsola), "tablas=", cargadas,
		"de", tablasQueSeCargan, "montonDelta=", d2cMontonUsado()-usado)
	instrumentoDeLasFuentes(nombre + "-despues-de-reabrir")

	return cargadas == tablasQueSeCargan
}

// componerNivel (DC-002.109): compone `preset/archivo` en el motor y devuelve el censo de la grilla. El `Stamp`
// y su `.ds1` parseado NO salen de aca: una variable local del que llama lo deja clavado en la pila, y en
// libgodc el barrido de pila es CONSERVADOR, asi que el `.ds1` de un marco muerto cuenta como raiz (medido en
// DC-002.109: 413.184 B vivos despues de una recoleccion completa).
//
// DC-002.110: por eso las celdas de las capas del `.ds1` viven en UN bloque del monton de C, y este es el lugar
// donde se devuelve (`Stamp.SoltarLosTilesDelDS1`, despues de `PlaceStampClippedWhere`). Con eso el nivel que
// se deja no ocupa el semiespacio de la fase siguiente, y la vuelta tiene donde parsear su `.ds1`.
//
// `fase` vacia = el camino de la corrida de referencia (el log es el de DC-002.106, sin lineas de mas).
func componerNivel(engine *d2mapengine.MapEngine, preset, archivo int, fase string) (pisos, paredes int, crc uint32, ok bool) {
	t0 := d2cMs()

	stamp := engine.LoadStamp(regionPueblo, preset, archivo)
	if stamp == nil {
		return 0, 0, 0, false
	}

	tamano := stamp.Size()

	// DC-002.110: donde vive el `.ds1` de este nivel. `bloque` es lo que se saco del semiespacio (0 si la
	// palanca esta apagada o el nivel no llega a los 64 KB: ahi sigue en el monton de Go, como en DC-002.109).
	enArena := stamp.TilesDelDS1EnArena()
	bloque := stamp.BytesDeLaArenaDelDS1()

	if fase != "" {
		println("MAPA109: FASE", fase, "DS1", stamp.RegionPath(), "ancho=", tamano.Width, "alto=", tamano.Height,
			"enArena=", enArena, "bloque=", bloque)
		semiespacio(fase + "-despues-de-loadstamp")
	}

	engine.ResetMapParaStamp(regionPueblo, tamano.Width, tamano.Height, stamp)
	engine.PlaceStampClippedWhere(stamp, 0, 0, tamano.Width, tamano.Height, func(int) bool { return false }, false)

	// ============================== EL `.ds1` SE SUELTA ACA (DC-002.110) ==============================
	// Con el nivel compuesto, el `.ds1` no lo mira nadie: las casillas del mapa son COPIAS de sus celdas. Se
	// devuelve su bloque al monton de C en el acto (contrato de `Stamp.SoltarLosTilesDelDS1`), y desde aca el
	// nivel que se acaba de componer NO ocupa el semiespacio de la fase siguiente -- que es exactamente lo que
	// la vuelta del cambio de nivel necesitaba (ver knobs.go y evidencia/dc-002.110).
	soltados := stamp.SoltarLosTilesDelDS1()

	if fase != "" {
		println("MAPA110: FASE", fase, "DS1 SOLTADO", stamp.RegionPath(), "enArena=", enArena, "bloque=", bloque,
			"devueltos=", soltados, "montonLibre=", d2cMontonLibre())
		semiespacio(fase + "-despues-de-soltar-el-ds1")
	}

	pisos, paredes, crc = censo(engine)

	etapa := "MAPA103: MAPA"
	if fase != "" {
		etapa = "MAPA103: FASE " + fase + " MAPA"
	}

	println(etapa, "tiles=", len(*engine.Tiles()), "pisos=", pisos, "paredes=", paredes,
		"crcGrilla=", int(crc), "ms=", int(d2cMs()-t0))

	return pisos, paredes, crc, true
}

// semiespacio (DC-002.109): el censo del semiespacio del recolector -- lo que ocupa, cuanto mide y cuantas
// recolecciones van. Es el instrumento que dice si una fase murio por el monton de C o por el de Go (las dos
// paredes que midieron DC-002.101/.103, ahora en el cambio de nivel).
func semiespacio(etapa string) {
	usado, total := int(d2cGcUsado()), int(d2cGcTotal())
	pct := 0

	if total > 0 {
		pct = usado * 100 / total
	}

	println("MAPA103: SEMIESPACIO", etapa, "usado=", usado, "total=", total, "pct=", pct,
		"colecciones=", int(d2cGcColecciones()))
}

// imprimeFase: una linea por fase en el resumen del final.
func imprimeFase(f fase) {
	println("MAPA109: CICLO", f.nombre, "ok=", f.ok, "libreAlComponer=", f.libre, "picoDeLaFase=", f.pico,
		"crc32=", int(f.crc), "crc32RGB=", int(f.crcRGB), "pixelesNoVacios=", f.pixeles, "ms=", f.ms,
		"tramoUsado=", f.tramoUsd, "pedazos=", f.pedazos)
}

// guardarCuadro1: lo que el cuadro 1 dejo medido, para comparar contra VOLVER.
func guardarCuadro1(crc uint32, pixeles int) {
	crcDelCuadroAlEntrar = crc
	pixelesDelCuadro1 = pixeles
}

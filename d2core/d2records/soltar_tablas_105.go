package d2records

// DC-002.105 -- SOLTAR LAS FILAS YA PARSEADAS DE LAS TABLAS DESPUES DE COMPONER EL NIVEL.
//
// POR QUE EXISTE. Cargar `LvlPrest.txt` deja en el monton de C 324.112 B que NO son el archivo: son las FILAS
// ya parseadas (medido en DC-002.102, con el mismo numero por el camino del stream y por el de la fuente).
// El `LevelPresets` es un `map[int]LevelPresetRecord`, y en libgodc el tablero de buckets del mapa pasa el
// umbral de 64 KB y se va a `gc_external_alloc` (malloc pelado, fuera de los dos semiespacios): el recolector
// NO lo devuelve jamas, y ademas lo sigue BARRIENDO en cada recoleccion (`gc_scan_externos`), asi que mantiene
// vivas las cadenas de todas las filas. Medido en esta tarjeta: dos tableros de 108.032 B y 216.064 B -- el
// segundo es el del mapa vivo (256 buckets de 844 B), el primero quedo varado en el crecimiento.
//
// CUANDO YA NO SE NECESITAN. Despues de COMPONER: el compositor lee las filas mientras arma las casillas del
// mapa y el renderer despues no las mira (usa la copia del `levelType` que el motor guardo y la paleta del
// acto, DC-002.88). El contrato completo:
//   - solo se llama con el nivel YA compuesto y sin nivel nuevo ni generador por delante;
//   - despues de soltar, esa tabla NO se puede usar: `Records.Level.Presets` queda en nil y un
//     `Records.LevelPreset(id)` devuelve el cero. Si hace falta un nivel nuevo, se vuelve a cargar la tabla
//     (y el MPQ tiene que seguir abierto: ver el contrato de `d2mpq.MPQ.LiberarTablas`);
//   - se sueltan SOLO las tablas que se nombran (las que `LoadRecords` cargo). Una que no esta en la lista se
//     queda como esta, y una que este paquete no sabe soltar se dice en `NoSoportadas` en vez de tocarla.
//
// POR QUE EL LIBERADOR SE INYECTA, Y POR QUE SON DOS. `free` no esta en la libreria estandar de Go y el
// tablero de un mapa no es una rebanada: no hay forma de sacarle el puntero desde Go (`unsafe` solo da la
// direccion de la VARIABLE del mapa). El que sabe leer el `GoMap` de gccgo es el shim de C de la consola
// (`d2c_soltar_filas`, con el layout de `libgodc/runtime/map_dreamcast.h` y sus `_Static_assert`), y ese shim
// solo libera lo que esta en la tabla de externos del runtime: un tablero del semiespacio no esta ahi y no se
// toca. Las tablas que se guardan como rebanada van por `SoltarTramo`, con la misma regla de umbral.
// Sin liberador (la PC) no se libera memoria a mano: se sueltan las referencias y el recolector de Go hace su
// trabajo, que es lo que corresponde cuando no hay monton externo que devolver.

import (
	"unsafe"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2resource"
)

// umbralExterno: el corte de libgodc (`GC_LARGE_OBJECT_THRESHOLD_KB` = 64). Debajo de esto el tramo vive en
// el semiespacio del recolector y liberarlo a mano corrompe el monton; arriba, es `malloc`.
const umbralExterno = 64 * 1024

// LiberadorDeFilas sabe devolver al monton las dos formas en que un `RecordManager` guarda sus filas.
type LiberadorDeFilas struct {
	// SoltarMapa recibe la direccion de la VARIABLE de un mapa y devuelve cuantos bytes devolvio (0 si el
	// tablero no estaba en el monton externo, o si no habia tablero).
	SoltarMapa func(mapa unsafe.Pointer) int

	// SoltarTramo recibe la direccion del primer elemento de una rebanada grande y su tamano, y devuelve
	// cuantos bytes devolvio (0 si no lo devolvio).
	SoltarTramo func(bloque unsafe.Pointer, bytes int) int
}

// liberadorDeFilas: lo instala el arranque de la consola (ver UsarLiberadorDeFilas).
var liberadorDeFilas *LiberadorDeFilas

// UsarLiberadorDeFilas dice con que se devuelven al monton las filas de las tablas. En la consola son los dos
// shims de C del POC (`d2c_soltar_filas` y `d2c_soltar_tramo`); en la PC no se instala.
func UsarLiberadorDeFilas(l *LiberadorDeFilas) { liberadorDeFilas = l }

// HayLiberadorDeFilas dice si el arranque instalo el liberador (lo consulta el instrumento).
func HayLiberadorDeFilas() bool { return liberadorDeFilas != nil }

// ResultadoDeSoltarTablas: que se solto, cuanto y que quedo sin soltar.
type ResultadoDeSoltarTablas struct {
	Tablas       int      // cuantas tablas se soltaron (sus filas quedaron sin referencia)
	Bytes        int      // bytes que el liberador devolvio DE VERDAD al monton
	Tramos       int      // cuantos tramos/tableros se devolvieron
	NoSoportadas []string // tablas que se pidieron y este paquete no sabe soltar
	Vacias       []string // tablas que se pidieron y no tenian filas cargadas
}

// SoltarTablas suelta las filas ya parseadas de las tablas nombradas. Ver el contrato en la cabecera.
func (r *RecordManager) SoltarTablas(cargadas []string) ResultadoDeSoltarTablas {
	res := ResultadoDeSoltarTablas{}
	libre := liberadorDeFilas

	for _, tabla := range cargadas {
		switch tabla {
		case d2resource.LevelType:
			if len(r.Level.Types) == 0 {
				res.Vacias = append(res.Vacias, tabla)

				continue
			}

			res.sumar(soltarTramo(unsafe.Pointer(&r.Level.Types[0]), cap(r.Level.Types)*4, libre))
			r.Level.Types = nil
		case d2resource.LevelPreset:
			if r.Level.Presets == nil {
				res.Vacias = append(res.Vacias, tabla)

				continue
			}

			res.sumar(soltarMapa(unsafe.Pointer(&r.Level.Presets), libre))
			r.Level.Presets = nil
		case d2resource.LevelWarp:
			if r.Level.Warp == nil {
				res.Vacias = append(res.Vacias, tabla)

				continue
			}

			res.sumar(soltarMapa(unsafe.Pointer(&r.Level.Warp), libre))
			r.Level.Warp = nil
		case d2resource.AutoMap:
			if len(r.Level.AutoMaps) == 0 {
				res.Vacias = append(res.Vacias, tabla)

				continue
			}

			res.sumar(soltarTramo(unsafe.Pointer(&r.Level.AutoMaps[0]), cap(r.Level.AutoMaps)*4, libre))
			r.Level.AutoMaps = nil
		case d2resource.LevelDetails:
			if r.Level.Details == nil {
				res.Vacias = append(res.Vacias, tabla)

				continue
			}

			res.sumar(soltarMapa(unsafe.Pointer(&r.Level.Details), libre))
			r.Level.Details = nil
		case d2resource.LevelMaze:
			if r.Level.Maze == nil {
				res.Vacias = append(res.Vacias, tabla)

				continue
			}

			res.sumar(soltarMapa(unsafe.Pointer(&r.Level.Maze), libre))
			r.Level.Maze = nil
		case d2resource.LevelSubstitutions:
			if r.Level.Sub == nil {
				res.Vacias = append(res.Vacias, tabla)

				continue
			}

			res.sumar(soltarMapa(unsafe.Pointer(&r.Level.Sub), libre))
			r.Level.Sub = nil
		case d2resource.ObjectType:
			if len(r.Object.Types) == 0 {
				res.Vacias = append(res.Vacias, tabla)

				continue
			}

			res.sumar(soltarTramo(unsafe.Pointer(&r.Object.Types[0]),
				cap(r.Object.Types)*int(unsafe.Sizeof(ObjectTypeRecord{})), libre))
			r.Object.Types = nil
		case d2resource.ObjectDetails:
			if r.Object.Details == nil {
				res.Vacias = append(res.Vacias, tabla)

				continue
			}

			res.sumar(soltarMapa(unsafe.Pointer(&r.Object.Details), libre))
			r.Object.Details = nil
		case d2resource.ObjectMode:
			if r.Object.Modes == nil {
				res.Vacias = append(res.Vacias, tabla)

				continue
			}

			res.sumar(soltarMapa(unsafe.Pointer(&r.Object.Modes), libre))
			r.Object.Modes = nil
		case d2resource.MonPreset:
			if r.Monster.Presets == nil {
				res.Vacias = append(res.Vacias, tabla)

				continue
			}

			res.sumar(soltarMapa(unsafe.Pointer(&r.Monster.Presets), libre))
			r.Monster.Presets = nil
		case d2resource.MonStats:
			if r.Monster.Stats == nil {
				res.Vacias = append(res.Vacias, tabla)

				continue
			}

			res.sumar(soltarMapa(unsafe.Pointer(&r.Monster.Stats), libre))
			r.Monster.Stats = nil
		case d2resource.MonStats2:
			if r.Monster.Stats2 == nil {
				res.Vacias = append(res.Vacias, tabla)

				continue
			}

			res.sumar(soltarMapa(unsafe.Pointer(&r.Monster.Stats2), libre))
			r.Monster.Stats2 = nil
		default:
			res.NoSoportadas = append(res.NoSoportadas, tabla)

			continue
		}

		res.Tablas++
	}

	return res
}

// soltarMapa: el camino de los mapas. Se le pasa la direccion de la VARIABLE, no el mapa: en gccgo un mapa ES
// un puntero al `GoMap`, y lo unico que se puede tomar desde Go es la direccion del campo que lo guarda.
func soltarMapa(direccion unsafe.Pointer, libre *LiberadorDeFilas) (bytes, tramos int) {
	if libre == nil || libre.SoltarMapa == nil {
		return 0, 0
	}

	if n := libre.SoltarMapa(direccion); n > 0 {
		return n, 1
	}

	return 0, 0
}

// soltarTramo: devuelve el tramo de una rebanada SOLO si paso el umbral externo (mas chico vive en el
// semiespacio y lo recicla el recolector).
func soltarTramo(datos unsafe.Pointer, bytes int, libre *LiberadorDeFilas) (devueltos, tramos int) {
	if bytes <= umbralExterno || libre == nil || libre.SoltarTramo == nil {
		return 0, 0
	}

	if n := libre.SoltarTramo(datos, bytes); n > 0 {
		return n, 1
	}

	return 0, 0
}

// sumar anota un paso en el resultado.
func (r *ResultadoDeSoltarTablas) sumar(bytes, tramos int) {
	r.Bytes += bytes
	r.Tramos += tramos
}

// TablasQueSePuedenSoltar dice que tablas sabe soltar este paquete. La usa el instrumento para avisar cuando
// le piden una que no.
func TablasQueSePuedenSoltar() []string {
	return []string{
		d2resource.LevelType, d2resource.LevelPreset, d2resource.LevelWarp, d2resource.AutoMap,
		d2resource.LevelDetails, d2resource.LevelMaze, d2resource.LevelSubstitutions,
		d2resource.ObjectType, d2resource.ObjectDetails, d2resource.ObjectMode,
		d2resource.MonPreset, d2resource.MonStats, d2resource.MonStats2,
	}
}

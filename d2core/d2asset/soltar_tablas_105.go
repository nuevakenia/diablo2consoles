package d2asset

// DC-002.105 -- SOLTAR LAS FILAS DE LAS TABLAS Y LAS DOS FUENTES DESPUES DE COMPONER (679.440 B medidos).
//
// POR QUE EXISTE. Con la imagen ya sin la tabla de lookups, despues de componer el pueblo el monton de C tiene
// 2.298.736 B disponibles y la cache de tiles del cuadro pide 2.723.840 B: faltan 425.104 B (DC-002.103). Hay
// dos piezas que el CUADRO NO USA y que se pueden devolver en cuanto el nivel esta compuesto, las dos medidas
// en su tarjeta:
//
//	las filas parseadas de las tablas   324.112 B   (DC-002.102: son las FILAS, no el archivo; el renderer no
//	                                                 las lee -- usa el levelType que el motor copio y la
//	                                                 paleta del acto, DC-002.88)
//	el censo de las tablas del MPQ      se mide aca  (DC-002.102 midio 355.328 B al abrir los dos MPQ)
//
// Este archivo es la mitad de las FILAS (la otra mitad, las fuentes, la resuelve `fuenteconsola.Fuente.Soltar`
// sobre `d2mpq.MPQ.LiberarTablas`). El contrato de las dos:
//
//	SOLO se llama con el nivel YA compuesto (y sin nivel nuevo ni generador por delante): lo que el cuadro
//	dibuja ya esta en las casillas del mapa, en el arena de bloques y en la cache; ni el renderer ni la cache
//	leen un byte de las tablas ni del MPQ (DC-002.88/.102).
//
//	DESPUES DE SOLTAR no se puede componer otro nivel sin volver a cargar las tablas (y sin volver a abrir el
//	MPQ, si tambien se soltaron las fuentes). Se dice en vez de dejarlo implicito: el instrumento cuenta lo
//	soltado y el POC lo imprime.
//
//	EL ARENA DE BLOQUES NO SE TOCA ACA: sus tramos (2.621.440 B) siguen exigiendo la cache armada (contrato de
//	DC-002.101). Esto es lo que se suelta ANTES de la cache, para que la cache entre.
//
// POR QUE EL LIBERADOR SE INYECTA. `free` no esta en la libreria estandar de Go y en libgodc lo que esta fuera
// del semiespacio (todo pedido de mas de 64 KB) solo sale por `runtime.FreeExternal` o por el tablero del mapa,
// que hay que saber leer en C. El arranque de la consola instala los dos shims con `UsarLiberadorDeFilas`; sin
// liberador (la PC) esta funcion suelta las referencias y el recolector de Go hace su trabajo, que es lo que
// corresponde cuando no hay monton externo que devolver.

import (
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2records"
)

// UsarLiberadorDeFilas dice con que se devuelven al monton las filas de las tablas. Lo llama el arranque de la
// consola con los dos shims de C del POC (`d2c_soltar_filas` y `d2c_soltar_tramo`); en la PC no se llama.
func UsarLiberadorDeFilas(l *d2records.LiberadorDeFilas) { d2records.UsarLiberadorDeFilas(l) }

// HayLiberadorDeFilas dice si el arranque lo instalo (lo consulta el instrumento del POC).
func HayLiberadorDeFilas() bool { return d2records.HayLiberadorDeFilas() }

// SoltarFilasDeLasTablas suelta las filas ya parseadas de las tablas que se nombran (las que se cargaron con
// `LoadRecords`) y devuelve cuantas tablas, cuantos bytes y que quedo sin soltar. Ver el contrato arriba.
func (am *AssetManager) SoltarFilasDeLasTablas(cargadas []string) d2records.ResultadoDeSoltarTablas {
	if am == nil || am.Records == nil {
		return d2records.ResultadoDeSoltarTablas{}
	}

	return am.Records.SoltarTablas(cargadas)
}

// TablasQueSePuedenSoltar: que tablas sabe soltar el RecordManager (para el instrumento del POC).
func TablasQueSePuedenSoltar() []string { return d2records.TablasQueSePuedenSoltar() }

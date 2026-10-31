package fuenteconsola

// DC-002.105 -- SOLTAR LA FUENTE (LAS TABLAS DEL MPQ Y LA MEMORIA DE TRABAJO) DESPUES DE COMPONER.
//
// POR QUE EXISTE. Abrir los dos MPQ del juego cuesta 355.328 B de monton de C (medido en DC-002.102, antes y
// despues de las fuentes en el censo del arranque), y ese monton es el unico lugar de donde puede salir la
// cache de tiles del cuadro (DC-002.101/.103). Las tablas del MPQ son el 99 % de esos bytes y no las devuelve
// nadie: en libgodc lo que pasa de 64 KB vive en `gc_external_alloc` (malloc pelado) y el recolector no lo
// toca. La memoria de trabajo de la lectura (`Scratch`) es otra pieza chica pero del mismo dueno.
//
// CUANDO YA NO SE NECESITAN. Despues de COMPONER: el nivel ya esta armado en las casillas del mapa, en el arena
// de bloques y en la cache, y ni el renderer ni la cache leen un byte del MPQ (DC-002.88/.102). El contrato:
//
//	SOLO se llama con el nivel YA compuesto y sin nivel nuevo por delante: despues de `Soltar`, la fuente NO
//	sirve para leer (el MPQ queda cerrado y sin tablas, y todas las cargas contestan `errSinMPQ`). Para volver
//	a cargar assets hay que ABRIRLA OTRA VEZ (`Abrir`), que es lo que ya sabe hacer el que arranca la consola.
//
//	UN MPQ CEDIDO (`Envolver`) NO SE SUELTA: sus tablas las puede estar mirando el que lo abrio. En ese caso
//	`Soltar` devuelve `Cedido=true`, no toca las tablas y solo devuelve la memoria de trabajo.
//
//	LA LIBERACION LA HACE EL LIBERADOR QUE SE INYECTA (`d2mpq.Liberador`): recibe el tramo y su tamano y
//	devuelve cuantos bytes devolvio DE VERDAD (0 si no lo devolvio). En la consola es el shim de C del POC; en
//	la PC no se pasa ninguno y no se libera memoria a mano (el recolector de Go hace su trabajo).

import (
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2mpq"
)

// ResultadoDeSoltar: que se devolvio al monton al soltar una fuente, con los numeros del dia.
type ResultadoDeSoltar struct {
	Ruta         string
	Tablas       d2mpq.TamanosDeLasTablas // el censo de las tablas ANTES de soltarlas
	BytesTablas  int                      // lo que el liberador devolvio de las tablas
	BytesTrabajo int                      // lo que devolvio la memoria de trabajo de la lectura
	Bytes        int                      // la suma de las dos
	Cedido       bool                     // el MPQ era prestado: no se soltaron las tablas
	YaSoltada    bool                     // se llamo dos veces: la segunda no hace nada
	ErrorCerrar  string                   // si el cierre del archivo fallo, el motivo
}

// Soltar devuelve al monton las tablas del MPQ y la memoria de trabajo de la lectura, y deja la fuente sin MPQ.
// Devuelve el numero de bytes que el liberador devolvio de verdad. Ver el contrato arriba.
func (f *Fuente) Soltar(libre d2mpq.Liberador) ResultadoDeSoltar {
	res := ResultadoDeSoltar{Ruta: f.ruta}

	if f.mpq == nil {
		res.YaSoltada = true

		return res
	}

	res.Tablas = f.mpq.TamanosDeLasTablas()

	if f.cedido {
		// el MPQ no es de la fuente: sus tablas las mira el que lo abrio
		res.Cedido = true
	} else {
		res.BytesTablas = f.mpq.LiberarTablas(libre)
	}

	if f.prestamo != nil {
		res.BytesTrabajo = f.prestamo.Liberar(libre)
		f.prestamo = nil
	}

	res.Bytes = res.BytesTablas + res.BytesTrabajo

	if !f.cedido {
		// cerrar el archivo NO devuelve memoria del monton: son los descriptores del sistema. Se cierra igual
		// porque la fuente queda muerta, y si el cierre falla se dice (no se esconde).
		if err := f.mpq.Close(); err != nil {
			res.ErrorCerrar = err.Error()
		}
	}

	f.mpq = nil

	return res
}

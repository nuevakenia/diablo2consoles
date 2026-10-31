package d2mpq

// DC-002.105 -- SOLTAR LAS TABLAS DEL MPQ Y LA MEMORIA DE TRABAJO DE LA LECTURA DESPUES DE COMPONER.
//
// POR QUE EXISTE. Abrir los dos MPQ del juego cuesta 355.328 B de monton de C (medido en DC-002.102), y ese
// monton es el unico lugar de donde puede salir la cache de tiles del cuadro (DC-002.101/.103). Las tablas
// del MPQ (hash y bloque) son el 99 % de esos bytes, estan vivas hasta que el proceso termina y NO se pueden
// liberar solas: en libgodc todo pedido de mas de 64 KB (`GC_LARGE_OBJECT_THRESHOLD`) va a `gc_external_alloc`,
// que es `malloc` pelado fuera de los dos semiespacios, y el recolector no devuelve ni un byte de ahi.
//
// CUANDO YA NO SE NECESITAN. Despues de COMPONER el nivel: lo que el cuadro dibuja ya esta en las casillas del
// mapa (y en el arena de bloques de los `.dt1`), y ni el renderer ni la cache de tiles leen un byte del MPQ
// (DC-002.88/.102). Soltarlas antes de componer es no poder cargar mas assets: es la otra mitad del contrato.
//
// EL CONTRATO, dicho como es:
//   - solo se llama con el nivel YA compuesto y sin nivel nuevo por delante;
//   - despues de `LiberarTablas` el MPQ queda SIN tablas: `Contains`, `TamanoDeArchivo`, `ReadFile` y los
//     caminos de lectura devuelven `ErrTablasSoltadas` -- no se puede volver a leer de el (si hace falta un
//     nivel nuevo, se abre el MPQ otra vez);
//   - los tramos se devuelven con el LIBERADOR que inyecta el que arranca la consola, que recibe el tamano y
//     devuelve cuantos bytes devolvio de verdad. Sin liberador (la PC) no se toca memoria: se sueltan las
//     referencias y el recolector de Go hace su trabajo, que es lo que corresponde cuando no hay monton
//     externo que devolver;
//   - se devuelven SOLO los tramos de mas de 64 KB, que son los que el runtime mando afuera. Un tramo mas
//     chico vive en el semiespacio del recolector y liberarlo a mano corrompe el monton (misma regla y mismo
//     umbral que `fuenteconsola.Liberar`).

import (
	"errors"
	"unsafe"
)

// ErrTablasSoltadas: se pidio algo de un MPQ al que ya se le devolvieron las tablas al monton
// (`LiberarTablas`). Es un error de uso, no un archivo faltante: se dice para no confundir "no esta el
// archivo" con "ya no hay con que buscarlo".
var ErrTablasSoltadas = errors.New("d2mpq: las tablas del MPQ ya se devolvieron al monton")

// umbralExterno: el mismo corte que usa libgodc para mandar una asignacion al monton de C
// (`GC_LARGE_OBJECT_THRESHOLD_KB` = 64, `libgodc/runtime/godc_config.h`). Debajo de esto el bloque vive en el
// semiespacio del recolector; arriba, es `malloc` y hay que devolverlo a mano.
const umbralExterno = 64 * 1024

// Liberador devuelve al monton un tramo que el runtime mando AFUERA y dice cuantos bytes devolvio (0 si no lo
// devolvio). En la consola lo instala el arranque con el `free` del shim; en la PC queda nil.
type Liberador func(tramo uintptr, bytes int) int

// TamanosDeLasTablas: cuanto ocupa cada tabla de un MPQ abierto. Es el instrumento con el que se dice, con los
// numeros del dia, cual es la parte de las tablas que puede volver al monton.
type TamanosDeLasTablas struct {
	RanurasHash  uint32 // ranuras de la tabla de hash
	HashOcupadas int    // 1 bit por ranura
	HashEntradas int    // las entradas ocupadas (16 B cada una)
	HashRangos   int    // ocupadas antes de cada bloque de ranuras
	Bloques      int    // entradas de la tabla de bloque
	BloqueTabla  int    // la tabla de bloque entera (16 B por entrada)
	Total        int    // la suma de las cuatro de arriba
	Externas     int    // las que pasaron el umbral y por lo tanto viven en el monton de C
}

// TamanosDeLasTablas devuelve el censo de las tablas (no toca memoria).
func (mpq *MPQ) TamanosDeLasTablas() TamanosDeLasTablas {
	t := TamanosDeLasTablas{
		RanurasHash:  mpq.hashRanuras,
		HashOcupadas: len(mpq.hashOcupadas),
		HashEntradas: len(mpq.hashEntradas),
		HashRangos:   len(mpq.hashRangos) * 2,
		Bloques:      int(mpq.blockEntries()),
		BloqueTabla:  len(mpq.blockTable),
	}

	t.Total = t.HashOcupadas + t.HashEntradas + t.HashRangos + t.BloqueTabla

	for _, n := range []int{t.HashOcupadas, t.HashEntradas, t.HashRangos, t.BloqueTabla} {
		if n > umbralExterno {
			t.Externas += n
		}
	}

	return t
}

// Soltada dice si las tablas ya se devolvieron (el MPQ no sirve para leer mas).
func (mpq *MPQ) Soltada() bool { return mpq.soltada }

// LiberarTablas devuelve al monton los tramos de las dos tablas y deja el MPQ sin tablas. Devuelve cuantos
// bytes se devolvieron de verdad (lo que dice el liberador, no lo que la cuenta supondria). Ver el contrato en
// la cabecera del archivo.
func (mpq *MPQ) LiberarTablas(libre Liberador) (bytes int) {
	if mpq.soltada {
		return 0
	}

	mpq.soltada = true

	bytes += liberarTramo(&mpq.hashOcupadas, libre)
	bytes += liberarTramo(&mpq.hashEntradas, libre)
	bytes += liberarRangos(&mpq.hashRangos, libre)
	bytes += liberarTramo(&mpq.blockTable, libre)

	mpq.hashOcupadas = nil
	mpq.hashEntradas = nil
	mpq.hashRangos = nil
	mpq.blockTable = nil

	return bytes
}

// liberarTramo devuelve un `[]byte` grande al monton. Con el liberador en nil (la PC) solo se suelta la
// referencia, y el recolector de Go recicla el tramo cuando quiera.
func liberarTramo(tramo *[]byte, libre Liberador) int {
	if libre == nil || len(*tramo) == 0 || cap(*tramo) <= umbralExterno {
		return 0
	}

	return libre(uintptr(unsafe.Pointer(&(*tramo)[0])), cap(*tramo))
}

// liberarRangos: lo mismo con la tabla de rangos, que es `[]uint16`.
func liberarRangos(tramo *[]uint16, libre Liberador) int {
	if libre == nil || len(*tramo) == 0 || cap(*tramo)*int(unsafe.Sizeof((*tramo)[0])) <= umbralExterno {
		return 0
	}

	return libre(uintptr(unsafe.Pointer(&(*tramo)[0])), cap(*tramo)*int(unsafe.Sizeof((*tramo)[0])))
}

// Liberar devuelve al monton la memoria de trabajo del prestamo (la tabla de posiciones, el bloque comprimido,
// el descompreso y el descompresor de la plataforma). Es la que pide `Prestar`/`Reservar` y la que se reusa
// archivo tras archivo: despues de soltarla, la proxima lectura la vuelve a pedir sola.
//
// Se suelta aparte de las tablas porque tiene otra vida: en el pueblo son ~12 KB (medido: `prestamoReservado=
// 12560`) y no es lo que decide nada -- se devuelve por completitud, con el mismo umbral.
func (s *Scratch) Liberar(libre Liberador) (bytes int) {
	if s == nil {
		return 0
	}

	bytes += liberarTramo(&s.crudo, libre)
	bytes += liberarTramo(&s.comprimido, libre)
	bytes += liberarTramo(&s.descomprimido, libre)

	if cap(s.posiciones) > 0 {
		if libre != nil && cap(s.posiciones)*4 > umbralExterno {
			bytes += libre(uintptr(unsafe.Pointer(&s.posiciones[0])), cap(s.posiciones)*4)
		}
	}

	s.posiciones = nil
	s.crudo = nil
	s.comprimido = nil
	s.descomprimido = nil
	s.reservado = 0

	if s.infladorDePrestamo != nil {
		s.infladorDePrestamo.Cerrar()
		s.infladorDePrestamo = nil
	}

	s.lector = nil
	s.reinicio = nil

	return bytes
}

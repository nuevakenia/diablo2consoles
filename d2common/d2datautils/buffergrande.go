package d2datautils

// DC-002.78 -- EL BUFFER QUE SE REUSA ENTRE ARCHIVOS (Fase B.1e).
//
// POR QUE EXISTE. Medido en DC-002.77: en el runtime de la Dreamcast (libgodc) TODA asignacion de mas de
// 64 KB (`GC_LARGE_OBJECT_THRESHOLD`, godc_config.h:12) se va a `gc_external_alloc` (gc_heap.c:116), que es
// `malloc` pelado y NO la recolecta nadie: 12 trozos de 256 KB tirados uno detras de otro subieron el
// monton 12 x 262.144 B, con y sin `runtime.GC()` (`console/dc/poc/sonda-gc-277`). Y cada archivo de tiles
// del acto 1 mide entre 300 KB y 1 MB, asi que leerlo "normalmente" (`ioutil.ReadAll`, `mpq.ReadFile`) deja
// tomado medio megabyte POR ARCHIVO, para siempre.
//
// LA CURA. Leer cada archivo en el MISMO buffer, que se pide una sola vez con la capacidad del archivo mas
// grande que se va a leer. Asi la carga de una zona no pide memoria por archivo: pide UNA vez y reusa.
//
// OJO, Y ES LA PARTE QUE SE OLVIDA FACIL. El archivo leido asi NO se puede guardar: el siguiente archivo
// lo pisa. Quien parsee el contenido tiene que COPIAR lo que quiera conservar (en el .dt1 son los bloques:
// `d2dt1.LoadDT1CopiaBloques`). Los bloques de un .dt1 del acto 1 miden entre 8 y 1.181 B (medido con
// `console/dc/medidas/bloques`), o sea que sus copias SI quedan en el semiespacio del recolector y este las
// puede devolver cuando el cache desaloja.
//
// Es de solo lectura concurrente si cada goroutine trae su propio BufferGrande: el estado es el buffer.

import "io"

const (
	// lecturaPorTrozos: cuanto se le pide a la fuente de una vez. Chico a proposito: el lector del MPQ
	// descomprime por sectores de 4 KB y no gana nada con pedidos mas grandes que 32 KB.
	lecturaPorTrozos = 32 * 1024
)

// BufferGrande es un solo arreglo de bytes que se reusa para leer archivo tras archivo.
type BufferGrande struct {
	datos     []byte
	trozo     []byte
	crecidas  int // cuantas veces hubo que PEDIR memoria AL LEER (cada pedido grande no la devuelve nadie)
	tamanoMax int // el archivo mas grande que entro hasta ahora
}

// Reservar deja el buffer con capacidad para tamano bytes. Se llama UNA vez, ANTES de la primera lectura:
// si no, el primer archivo hace crecer el buffer por duplicacion y cada paso de esa cadena queda tomado.
// Lo que pide esta reserva es UN pedido querido y no se cuenta en `Crecidas` (que mide lo que crecio
// leyendo: eso es lo que tiene que quedarse en cero).
func (b *BufferGrande) Reservar(tamano int) {
	if tamano <= cap(b.datos) {
		return
	}

	b.datos = make([]byte, 0, tamano)
}

// LeerEn lee la fuente entera DENTRO del arreglo que se le presta (el tramo libre de un arena) y devuelve
// la vista de lo leido. `entro` en false significa que la fuente no cabia: en ese caso NO se pidio memoria
// -- se devuelve lo que se alcanzo a leer y el que llama decide (tipicamente: tirar el arena y reintentar).
//
// Esta es la forma que usa el motor en la consola (d2asset/lectura_de_tiles.go): el arreglo es del arena y
// aca no se pide nada.
func (b *BufferGrande) LeerEn(dst []byte, fuente io.Reader) (datos []byte, entro bool, err error) {
	if b.trozo == nil {
		b.trozo = make([]byte, lecturaPorTrozos)
	}

	datos = dst[:0]

	for {
		n, errLectura := fuente.Read(b.trozo)
		if n > 0 {
			if len(datos)+n > cap(datos) {
				// no entra: ni se intenta (pedir memoria aca seria justo lo que se quiere evitar)
				return datos, false, nil
			}

			datos = append(datos, b.trozo[:n]...)
		}

		if errLectura == io.EOF {
			break
		}

		if errLectura != nil {
			return nil, false, errLectura
		}
	}

	if len(datos) > b.tamanoMax {
		b.tamanoMax = len(datos)
	}

	return datos, true, nil
}

// LeerTodo lee la fuente entera en el buffer y devuelve la vista de lo leido. La vista es VALIDA HASTA LA
// SIGUIENTE LLAMADA: el que la quiera conservar, que copie.
func (b *BufferGrande) LeerTodo(fuente io.Reader) ([]byte, error) {
	if b.trozo == nil {
		b.trozo = make([]byte, lecturaPorTrozos)
	}

	datos := b.datos[:0]

	for {
		n, err := fuente.Read(b.trozo)
		if n > 0 {
			if len(datos)+n > cap(datos) {
				b.crecidas++
			}

			datos = append(datos, b.trozo[:n]...)
		}

		if err == io.EOF {
			break
		}

		if err != nil {
			return nil, err
		}
	}

	b.datos = datos

	if len(datos) > b.tamanoMax {
		b.tamanoMax = len(datos)
	}

	return datos, nil
}

// Capacidad: para cuantos bytes esta reservado el buffer.
func (b *BufferGrande) Capacidad() int { return cap(b.datos) }

// Crecidas: cuantas veces hubo que pedir memoria al monton. Con el buffer reservado de antemano, este
// numero NO TIENE que crecer durante la carga: si crece, es que falta reservar mas arriba.
func (b *BufferGrande) Crecidas() int { return b.crecidas }

// ArchivoMasGrande: el tamano del archivo mas grande leido con este buffer.
func (b *BufferGrande) ArchivoMasGrande() int { return b.tamanoMax }

package d2dcc

// DC-002.98 -- LA ARENA DEL DECODIFICADOR: los buffers de trabajo del .dcc, fuera del monton de Go.
//
// POR QUE EXISTE (medido en la consola, DC-002.96 seccion 3.3 y esta tarjeta):
//
//   - `aas2litnuhth.dcc` (144.996 B, UNA direccion de 30 cuadros, caja 135x182) pide 36.217.240 B en 1.738
//     objetos para producir 737.100 B de pixeles: 49 veces el resultado.
//   - De esos 36 MB, 32,3 MB son UNA sola asignacion: `fillPixelBuffer` pide `maxCellX*maxCellY`
//     entradas cuando el decodificador usa 44.900. Es la suma de las columnas POR la suma de las filas
//     (cuadratico en los cuadros), no la suma de las celdas de cada cuadro.
//   - El resto es lo que el decodificador deja VIVO mientras decodifica: los 30 buffers de pixeles
//     (24.570 B cada uno) y las 44.900 celdas de los cuadros (1,4 MB con `DCCCell` de 32 B).
//
// En la consola el semiespacio del recolector son 2 MB: 1,4 MB de celdas + 737 KB de pixeles no entran, y
// por eso el archivo NO se podia decodificar (45 recolecciones seguidas y muerte con
// `FATAL: recover without checkpoint`, que es lo que sale cuando el `malloc` del monton externo falla y el
// `recover` del parser se come el throw).
//
// LA CURA, y es de MEMORIA, no de formato: si el que llama define `PedirDeArena`, el decodificador le pide
// los buffers grandes (los pixeles de cada cuadro, el lienzo de trabajo de la direccion, las celdas y la
// tabla de celdas) EN VEZ de pedirlos al monton de Go. El que los entrega es el dueno y los libera el
// cuando termina de usar lo decodificado (en la consola: tramos del monton de C, `Pedir`/`Liberar` de
// `console/dc/fuenteconsola`). En la PC la variable queda en nil y el decodificador pide con `make`, como
// siempre: el camino de la PC y el del motor no cambian una linea de comportamiento.
//
// EL CONTRATO DEL QUE ENTREGA LA ARENA, y hay que respetarlo:
//   - `PedirDeArena(bytes)` devuelve un tramo de AL MENOS `bytes`, con la misma vida que lo decodificado:
//     los `PixelData` de los cuadros y las celdas quedan APUNTANDO adentro, y no se copian.
//   - El tramo tiene que empezar alineado a 8 bytes: los `DCCPixelBufferEntry` llevan dos `int` y en la PC
//     son de 8 bytes.
//   - Si devuelve menos bytes de los pedidos, el decodificador pide ese buffer al monton de Go: no revienta
//     por un tramo corto, se degrada al camino de siempre.
//
// OJO CON EL TIPO DE LOS TRAMOS. `DCCCell` y `DCCPixelBufferEntry` NO llevan punteros: un bloque de mas de
// 64 KB en la consola se va al `malloc` de C y el recolector NO lo barre (ni falta que hace) ni lo devuelve
// (por eso el dueno lo libera). Un tramo de arena que llevara punteros dentro NO se puede entregar asi: el
// recolector no lo veria y lo que colgara de el se reciclaria (medido en DC-002.83).

import "unsafe"

// PedirDeArena, si no es nil, entrega los buffers de trabajo del decodificador. nil = el camino de
// siempre (`make`, monton de Go). Ver el comentario de arriba para el contrato.
var PedirDeArena func(bytes int) []byte

// El tope de las conversiones de abajo: `(*[tope]T)(p)[:n:n]`. Es el truco de siempre para mirar un tramo
// de bytes como un arreglo tipado sin `unsafe.Slice` (que pide go 1.17 y el modulo declara go 1.16). Un
// `.dcc` real no se acerca: la direccion mas grande medida usa 44.900 celdas y 1.347.029 entradas.
const (
	topeCeldas   = 1 << 22
	topeEntradas = 1 << 24
)

var (
	tamanoCelda   = int(unsafe.Sizeof(DCCCell{}))
	tamanoEntrada = int(unsafe.Sizeof(DCCPixelBufferEntry{}))
)

// pedirBytes: los pixeles (de un cuadro o el lienzo de trabajo de la direccion).
func pedirBytes(n int) []byte {
	if n <= 0 {
		return nil
	}

	if PedirDeArena != nil {
		if b := PedirDeArena(alinear8(n)); len(b) >= n {
			return b[:n]
		}
	}

	return make([]byte, n)
}

// pedirCeldas: las celdas de un cuadro (o de la direccion).
func pedirCeldas(n int) []DCCCell {
	if n <= 0 {
		return make([]DCCCell, 0)
	}

	if PedirDeArena != nil && n <= topeCeldas {
		bytes := alinear8(n * tamanoCelda)
		if b := PedirDeArena(bytes); len(b) >= n*tamanoCelda {
			return (*[topeCeldas]DCCCell)(unsafe.Pointer(&b[0]))[:n:n] //nolint:gosec // la arena del que llama
		}
	}

	return make([]DCCCell, n)
}

// pedirEntradas: la tabla de celdas del decodificador (`fillPixelBuffer`).
func pedirEntradas(n int) []DCCPixelBufferEntry {
	if n <= 0 {
		return make([]DCCPixelBufferEntry, 0)
	}

	if PedirDeArena != nil && n <= topeEntradas {
		bytes := alinear8(n * tamanoEntrada)
		if b := PedirDeArena(bytes); len(b) >= n*tamanoEntrada {
			return (*[topeEntradas]DCCPixelBufferEntry)(unsafe.Pointer(&b[0]))[:n:n] //nolint:gosec // idem
		}
	}

	return make([]DCCPixelBufferEntry, n)
}

func alinear8(n int) int {
	return (n + 7) &^ 7
}

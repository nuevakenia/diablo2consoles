package d2asset

// DC-002.78 -- LA CARGA DE TILES DE LA CONSOLA NO PIDE MEMORIA POR ARCHIVO (Fase B.1e).
//
// EL DEFECTO QUE ARREGLA (medido en DC-002.77, no supuesto). Entrar al pueblo deja el monton en
// 12.073.160 B y la segunda zona muere con `gc_external_alloc: OOM` pidiendo 368.640 B. El cache no era el
// culpable (desaloja 6 veces y se queda en 721.552 B de 1.048.576): el culpable es que en libgodc toda
// asignacion de mas de 64 KB se va a `gc_external_alloc` (`gc_heap.c:116`), que es `malloc` pelado y no lo
// recolecta nadie -- medido con `console/dc/poc/sonda-gc-277`: 12 trozos de 256 KB suman +262.152 B cada
// uno, con y sin `runtime.GC()`. Y cada .dt1 del acto 1 mide 318.414 B a 1.008.003 B.
//
// LA CURA: UN ARENA, no un buffer por archivo.
//
// El camino de siempre pide un arreglo NUEVO por archivo (`ioutil.ReadAll` / `mpq.ReadFile`) y, como el
// archivo pasa de 64 KB, ese arreglo queda tomado para siempre. Aca el motor pide UNA vez un arena del
// tamano de los .dt1 de una zona entera (4.408.317 B crudos, medidos en
// `console/dc/evidencia/dc-002.77-zona-host.txt`) y cada archivo se lee ADENTRO, uno detras de otro (bump).
// La carga de una zona deja de pedir memoria: el arena se pide una vez y despues solo avanza un cursor.
//
// POR QUE NO SE COPIAN LOS BLOQUES (el camino (a) de la tarjeta, implementado y medido EN CONTRA). Copiar
// cada bloque a memoria propia (`d2dt1.LoadDT1CopiaBloques`) deja los bloques en el SEMIESPACIO del
// recolector (miden 8..1.181 B, todos bajo el umbral) y en el papel eso es lo correcto. Medido en Flycast:
// NO AGUANTA. Una zona son 16.266 copias; el recolector salta por umbral EN MEDIO del parseo (dos veces en
// los dos primeros archivos) y el SH-4 muere con `Unhandled exception: PC 7daed713, code 1, evt 00e0` /
// `Data address error`: el recolector de libgodc es de copia y mueve objetos mientras el parser tiene
// punteros a los pedazos que esta armando. Con el arena no hay copias ni presion sobre el semiespacio: la
// carga se comporta, en el semiespacio, como la de DC-002.70/.75/.77 (que corrian bien hasta quedarse sin
// monton externo).
//
// EL CONTRATO (lo que hay que respetar al usar esto):
//   - los bloques de un .dt1 cargado asi APUNTAN AL ARENA: valen mientras el arena no se reinicie.
//   - el arena se reinicia solo cuando un archivo no entra, y al reiniciarlo se VACIA el cache de dt1s
//     (sus entradas apuntaban adentro). Tambien se puede reiniciar a mano con `ReiniciarArenaDeTiles`, que
//     es lo que tiene que hacer el motor cuando TIRA UN NIVEL.
//   - quien se guarde los tiles de un .dt1 mas alla del nivel (el `dt1TileData` del MapEngine) tiene que
//     copiarlos a su propio arreglo, o reiniciar el arena solo al tirar el nivel.
//
// POR QUE UN ARENA Y NO UN POOL DE N BUFFERS. Con N buffers hay que decidir cual se puede reusar, y eso es
// llevar la cuenta de quien tiene el .dt1: un error ahi se ve como un mapa dibujado con pedazos de otro
// archivo. El arena no tiene esa decision: avanza y, cuando no entra, se tira entero (y con el, el cache).
//
// POR QUE NO SE LIBERA NADA CON `runtime.FreeExternal`. El simbolo existe en libgodc (`gc_heap.c:283`,
// `_runtime.FreeExternal`) pero NO esta declarado en Go: solo se puede llamar desde C
// (`tests/c/test_free_external.c`). Y la corrida de DC-002.77 con `free()` mostro la otra punta del
// problema: liberar y volver a pedir tamanos distintos FRAGMENTA el monton (arena 13,4 MB con 928 KB libres
// y sin bloque contiguo de 397 KB). El arena no libera nada: no fragmenta nada.
//
// CUANDO SE PRENDE. Lo prende `AjustarPresupuestosDeConsola()` (presupuestos.go), que es el arranque de la
// consola. La PC no la llama NUNCA: en el host todo esto queda apagado y el camino es el de siempre.
//
// POR QUE VARIABLE Y NO BUILD TAG: gccgo NO filtra por build tag (compila todos los .go del paquete, medido
// en DC-002.2): dos archivos, uno con tag, entrarian los dos. Es la misma razon que documenta
// presupuestos.go.

import (
	"errors"
	"io"
	"sync"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2datautils"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2dt1"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2mpq"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2loader"
)

const (
	// capacidadDeLaArenaDeTiles: los .dt1 de la zona mas pesada del acto 1 (el pueblo: 8 archivos,
	// 4.408.317 B crudos, medido en DC-002.77) mas un 10 % de aire. Es el numero que hace que entrar a
	// una zona no pida memoria: los 4,41 MB entran de una y los archivos se leen uno detras de otro.
	//
	// DC-002.88: este numero es el PISO por defecto, no el unico. El compositor del motor no carga una
	// zona sino un NIVEL, y el nivel dice archivos distintos: con `ReservarArenaDeTiles` el que llama
	// fija la capacidad con la suma exacta de los `.dt1` que va a cargar (medido: el level type del pueblo
	// lista 9 archivos = 4.652.603 B, pero el `.ds1` del pueblo usa 8 = 4.350.665 B, y la constante de
	// 4.848.148 B sobraba por dos motivos distintos: el aire del 10 % y los archivos que el `.ds1` no
	// lista). Sin la llamada, esto queda exactamente como estaba en DC-002.78.
	capacidadDeLaArenaDeTiles = 4408317 + 440831 // 4.848.148 B
)

// capacidadPedidaDelArena: la capacidad con la que se pide el arena, fijada por `ReservarArenaDeTiles`
// (DC-002.88). 0 = se usa `capacidadDeLaArenaDeTiles`. Solo tiene efecto ANTES del primer `.dt1`: una vez
// pedido el arena no se puede agrandar ni achicar sin tirarlo (los bloques de los `.dt1` ya cargados
// apuntan adentro), y por eso la reserva se pide temprano y se dice en el estado del instrumento.
var capacidadPedidaDelArena int

// ============================== DC-002.88: las claves de tile ==============================
//
// Una clave de tile es `(estilo, secuencia, tipo)`: lo unico que identifica un tile adentro de los `.dt1` del
// nivel (`d2dt1.Tile.Style/Sequence/Type`). El `.ds1` la referencia en cada casilla, y el motor la usa como
// llave (`MapEngine.dt1KeyCount`). Aca se empaqueta en un `int64` para poder guardar la lista de claves del
// nivel en una LISTA ORDENADA (no un mapa: ver `Stamp.ClavesDeTiles`).
const bitsPorClave = 21

const mascaraDeClave = (1 << bitsPorClave) - 1

// ClaveDeTile empaqueta `(estilo, secuencia, tipo)` en un `int64` (21 bits cada uno). Devuelve -1 si alguno
// no entra en 21 bits: ahi el filtro NO se puede usar (una clave truncada se comeria tiles buenos).
func ClaveDeTile(estilo, secuencia, tipo int64) int64 {
	if estilo < 0 || secuencia < 0 || tipo < 0 ||
		estilo > mascaraDeClave || secuencia > mascaraDeClave || tipo > mascaraDeClave {
		return -1
	}

	return estilo<<(2*bitsPorClave) | secuencia<<bitsPorClave | tipo
}

// OrdenarClavesDeTiles ordena la lista de claves (quicksort de Hoare, sin reflexion: no hay `sort.Slice` en
// la consola y 2337 claves se ordenan en nada).
func OrdenarClavesDeTiles(claves []int64) { ordenarClaves(claves) }

func ordenarClaves(c []int64) {
	if len(c) < 2 {
		return
	}

	pivote := c[len(c)/2]
	i, j := 0, len(c)-1

	for i <= j {
		for c[i] < pivote {
			i++
		}

		for c[j] > pivote {
			j--
		}

		if i <= j {
			c[i], c[j] = c[j], c[i]
			i++
			j--
		}
	}

	if j > 0 {
		ordenarClaves(c[:j+1])
	}

	if i < len(c)-1 {
		ordenarClaves(c[i:])
	}
}

// HayClave dice si la clave esta en la lista ordenada (busqueda binaria).
func HayClave(claves []int64, clave int64) bool {
	izq, der := 0, len(claves)-1

	for izq <= der {
		medio := (izq + der) / 2

		switch {
		case claves[medio] == clave:
			return true
		case claves[medio] < clave:
			izq = medio + 1
		default:
			der = medio - 1
		}
	}

	return false
}

// ============================== DC-002.88: EL ARENA DE BLOQUES USADOS ==============================
//
// EL NUMERO QUE LO PIDE (medido con `console/dc/medidas/arena` sobre el pueblo). El arena guarda los `.dt1`
// CRUDOS, y el crudo trae mucho mas de lo que el nivel mira: de los 8 `.dt1` que el `.ds1` del pueblo lista
// (4.350.665 B, 507 tiles), las casillas del `.ds1` solo referencian 311 tiles, y los bloques RLE de esos 311
// tiles son 2.586.489 B. El resto son tiles que el nivel no toca (`act1/barracks/warp.dt1` aporta 3 tiles de
// 81: 23.298 B de bloque de 629.046) mas las cabeceras del archivo.
//
// COMO FUNCIONA. En vez de dejar el crudo entero en el arena, cada `.dt1` se lee en UN buffer prestado (el
// del archivo mas grande del nivel, pedido una vez y reusado), se parsea, y SOLO los bloques de los tiles
// que el nivel usa se COPIAN al arena de bloques (tramos de 512 KB, pedidos a medida). Los tiles que el nivel
// no usa no se copian y no se devuelven: no pueden aparecer en ninguna casilla (una casilla solo referencia
// `(estilo, secuencia, tipo)` de un `.dt1` que el `.ds1` lista, y esos son justo los que se conservan).
//
// POR QUE COPIAR Y NO APUNTAR AL CRUDO. El contrato de DC-002.78 dice que los bloques de un `.dt1` cargado
// asi APUNTAN AL ARENA. Aca se invierte: apuntan a un arena de BLOQUES que ya no es el archivo, y por eso el
// archivo se puede reusar en el siguiente. Lo que NO se hace es copiar al SEMIESPACIO del recolector: DC-002.78
// midio que eso mata la consola en medio del parseo (el recolector salta por umbral y mueve lo que el parser
// esta armando). El arena de bloques es monton externo y no lo toca el recolector.
var (
	// bloquesUsados: prende el camino de arriba. Lo prende `PrepararArenaDeBloquesUsados`.
	bloquesUsados bool

	// clavesDelNivel: las claves `(estilo, secuencia, tipo)` que el `.ds1` del nivel referencia, empaquetadas
	// y ORDENADAS (`d2asset.ClaveDeTile`). Solo los tiles con una de estas claves se copian al arena de bloques.
	clavesDelNivel []int64

	// arenaDeBloques: los tramos del arena. Cada tramo es un pedido propio, asi que los punteros de los
	// bloques ya copiados NO se invalidan cuando hace falta un tramo nuevo. Los tramos NO se tiran al reiniciar
	// el arena: se vuelven a usar desde el primero (medido: si se tiran, el segundo nivel del POC vuelve a
	// pedir 2,6 MB y muere en el `make` de las casillas, que solo pedian 98.304 B).
	arenaDeBloques [][]byte

	// tramoActual/cursorDelBloque: por donde va el empacado del nivel vivo.
	tramoActual     int
	cursorDelBloque int

	// bytesDeBloques: lo copiado en total (el numero que se informa).
	bytesDeBloques int

	// scratchDeLectura: el `.dt1` crudo, prestado. UNO para todo el nivel (el del archivo mas grande), asi
	// leer el archivo no pide memoria por archivo.
	scratchDeLectura []byte
	scratchDelNivel  int
)

// tramoDeBloques: lo que se le pide al monton cuando el tramo actual no alcanza. 512 KB entra en un pedido
// externo (todo lo que pasa de 64 KB va a `gc_external_alloc`) y deja el monton con pedidos grandes y pocos:
// con 2,5 MB de bloques son 6 pedidos por nivel, no 40.
const tramoDeBloques = 512 * 1024

// PrepararArenaDeBloquesUsados prende el camino del arena de bloques y lo dimensiona (DC-002.88):
//   - `claves`: las `(estilo, secuencia, tipo)` del `.ds1` del nivel. `nil` apaga el camino.
//   - `scratch`: el tamano del `.dt1` mas grande del nivel (el buffer prestado para leerlo entero).
//
// Se llama ANTES de cargar los `.dt1` del nivel, y despues hay que tirar el arena (`ReiniciarArenaDeTiles`)
// porque los tiles cacheados del nivel anterior solo tienen los bloques que ese nivel usaba.
func PrepararArenaDeBloquesUsados(claves []int64, scratch int) {
	candadoDeTiles.Lock()
	defer candadoDeTiles.Unlock()

	if scratch <= 0 {
		bloquesUsados = false
		clavesDelNivel = nil

		return
	}

	bloquesUsados = true
	clavesDelNivel = claves
	scratchDelNivel = scratch

	// el buffer prestado NO se tira si ya hay uno del tamano: es la lectura de cada `.dt1` y pedirlo de nuevo
	// por nivel es mas monton que no vuelve.
	if len(scratchDeLectura) < scratchDelNivel {
		scratchDeLectura = make([]byte, scratchDelNivel)
	}
}

// EnBloquesUsados dice si el camino del arena de bloques esta prendido (lo consulta el instrumento).
func EnBloquesUsados() bool { return bloquesUsados }

// reservarEnElArenaDeBloques: devuelve un tramo de `n` bytes del arena de bloques, reusando los tramos que ya
// se pidieron en niveles anteriores y pidiendo uno nuevo solo cuando no alcanzan.
func reservarEnElArenaDeBloques(n int) []byte {
	if n == 0 {
		return nil
	}

	for {
		if tramoActual < len(arenaDeBloques) {
			if resto := len(arenaDeBloques[tramoActual]) - cursorDelBloque; n <= resto {
				tramo := arenaDeBloques[tramoActual][cursorDelBloque : cursorDelBloque+n]
				cursorDelBloque += n
				bytesDeBloques += n

				return tramo
			}

			// no entra en lo que queda de este tramo: se pasa al siguiente (lo que sobra se pierde, y con
			// bloques de hasta ~40 KB contra tramos de 512 KB es a lo sumo el ultimo bloque de cada tramo)
			tramoActual++
			cursorDelBloque = 0

			continue
		}

		tamano := tramoDeBloques
		if n > tamano {
			tamano = n
		}

		arenaDeBloques = append(arenaDeBloques, make([]byte, tamano))
	}
}

// cargarDT1EnBloquesUsados: un `.dt1` leido en el buffer prestado y empacado al arena de bloques. Ver la
// cabecera de esta seccion.
func (am *AssetManager) cargarDT1EnBloquesUsados(filePath string) (*d2dt1.DT1, error) {
	tam, hay := am.Loader.TamanoDeArchivo(filePath)
	if !hay {
		return nil, errDT1NoEntraEnLaArena
	}

	if tam > scratchDelNivel {
		scratchDelNivel = tam
	}

	if len(scratchDeLectura) < scratchDelNivel {
		scratchDeLectura = make([]byte, scratchDelNivel)
	}

	if prestamoDeTiles == nil {
		prestamoDeTiles = &d2mpq.Scratch{}
	}

	leidos, err := am.Loader.LeerEnteroEnConPrestamo(filePath, scratchDeLectura[:tam], prestamoDeTiles)
	if err != nil {
		return nil, err
	}

	dt1, err := d2dt1.LoadDT1(scratchDeLectura[:leidos])
	if err != nil {
		return nil, err
	}

	// los bloques de los tiles que el nivel usa se COPIAN al arena de bloques; los demas tiles se tiran
	conservados := 0

	for t := range dt1.Tiles {
		clave := ClaveDeTile(int64(dt1.Tiles[t].Style), int64(dt1.Tiles[t].Sequence), int64(dt1.Tiles[t].Type))
		if len(clavesDelNivel) > 0 && !HayClave(clavesDelNivel, clave) {
			continue
		}

		for b := range dt1.Tiles[t].Blocks {
			datos := dt1.Tiles[t].Blocks[b].EncodedData
			if len(datos) == 0 {
				continue
			}

			destino := reservarEnElArenaDeBloques(len(datos))
			copy(destino, datos)

			dt1.Tiles[t].Blocks[b].EncodedData = destino
		}

		dt1.Tiles[conservados] = dt1.Tiles[t]
		conservados++
	}

	dt1.Tiles = dt1.Tiles[:conservados]
	archivosDeTiles++

	return dt1, nil
}

// soloLosTilesDelDS1 (DC-002.88): en la consola, un nivel carga SOLO los `.dt1` que su `.ds1` lista, no
// los 32 campos de archivo del level type. Es lo que hace el motor de la PC en los niveles generados
// (`AddDS1`), y la razon por la que el compositor no entra en el monton: el level type del pueblo lista 9
// archivos (4.652.603 B) y el `.ds1` usa 8 (4.350.665 B) -- `act1/town/trees.dt1` (511.673 B) y
// `act1/outdoors/bridge.dt1` (448.423 B) se cargan al arena sin que una sola casilla los mire, porque no
// estan en la lista del `.ds1`. Una casilla del `.ds1` solo puede referenciar estilos de los `.dt1` que el
// propio `.ds1` lista, asi que la lista del `.ds1` alcanza y sobra.
//
// La PC NO CAMBIA: la prenda `ArrancarComoConsola` (consola.go), y `ResetMap` sigue usando el level type.
var soloLosTilesDelDS1 bool

// CargarSoloLosTilesDelDS1 prende/apaga la politica del `.ds1` (ver arriba). La prende el arranque de la
// consola; la PC no la llama nunca.
func CargarSoloLosTilesDelDS1(v bool) { soloLosTilesDelDS1 = v }

// SoloLosTilesDelDS1 dice si la politica esta prendida (lo consultan d2mapengine y d2mapstamp).
func SoloLosTilesDelDS1() bool { return soloLosTilesDelDS1 }

// ReservarArenaDeTiles fija la capacidad del arena (DC-002.88) y la devuelve. Es una RESERVA, no una
// asignacion: el arena se sigue pidiendo en la primera carga de un `.dt1` (el arranque del PVR necesita el
// monton ordenado, DC-002.50). Despues de eso la reserva no hace nada y el estado lo dice
// (`EstadoDeLosTiles().CapacidadPedida` contra `.Capacidad`).
//
// Un valor <= 0 devuelve la capacidad a la constante de DC-002.78.
func ReservarArenaDeTiles(capacidad int) int {
	candadoDeTiles.Lock()
	defer candadoDeTiles.Unlock()

	if capacidad <= 0 {
		capacidadPedidaDelArena = 0
	} else {
		capacidadPedidaDelArena = capacidad
	}

	return capacidad
}

// CapacidadReservadaDelArena es la capacidad que se va a pedir (0 = la constante del codigo).
func CapacidadReservadaDelArena() int {
	candadoDeTiles.Lock()
	defer candadoDeTiles.Unlock()

	return capacidadPedidaDelArena
}

// errores del camino de la consola
var (
	// errDT1NoEntraEnLaArena: el archivo es mas grande que el arena entero. No pasa con los .dt1 del acto
	// 1 (el mayor mide 1.008.003 B); si pasara, se dice en vez de morir.
	errDT1NoEntraEnLaArena = errors.New("d2asset: el .dt1 no entra en el arena de tiles")

	// errSinArena: la ruta de la consola esta prendida pero el arena no se pudo pedir.
	errSinArena = errors.New("d2asset: no hay arena de tiles")
)

var (
	// las dos mitades del camino de la consola. En la PC quedan apagadas.
	caminoDeTilesDeConsola bool

	// lecturaPrestadaDeTiles (DC-002.80): leer cada .dt1 con la memoria de trabajo PRESTADA por el arena
	// (`d2mpq.ReadFileInto`), en vez de por el stream del MPQ. Es lo que hace que la lectura no pida
	// memoria por archivo: medido en DC-002.78, leer UN .dt1 de 505 KB por el stream pedia ~2 MB del
	// semiespacio del recolector y el runtime moria con `gc_alloc: out of memory`. Apagada queda el camino
	// de DC-002.78 (que se midio EN CONTRA); la palanca existe para poder medir los dos en la misma corrida.
	lecturaPrestadaDeTiles = true

	// el prestamo: la tabla de posiciones, el bloque comprimido, el descompreso y el descompresor de zlib
	// de la lectura. Se pide la primera vez y se reusa entre archivos y entre zonas.
	prestamoDeTiles *d2mpq.Scratch

	// el arena: pedido UNA vez, con la medida de una zona. Los .dt1 se leen adentro.
	arenaDeTiles      []byte
	cursorDelArena    int
	reiniciosDelArena int
	archivosDeTiles   int

	// el lector por trozos (no pide memoria propia: lee ADENTRO del tramo que se le presta). Es el camino
	// de DC-002.78, el que se mide en contra.
	lectorDeTiles  d2datautils.BufferGrande
	candadoDeTiles sync.Mutex
)

// UsarLecturaPrestadaDeTiles prende/apaga la lectura con prestamo del camino de tiles (DC-002.80). Lo usa la
// palanca `LEER=` del POC de zonas para medir los dos caminos con el mismo binario.
func UsarLecturaPrestadaDeTiles(v bool) { lecturaPrestadaDeTiles = v }

// LecturaPrestadaDeTiles dice si la ruta de tiles esta leyendo el MPQ con memoria prestada (DC-002.81). Es el
// estado REAL de la palanca -- no el que el que llama creia haber pedido --, que es lo que hay que ver en el
// log del motor: `lecturaPrestada= true`.
func LecturaPrestadaDeTiles() bool { return lecturaPrestadaDeTiles }

// EstadoDelPrestamoDeTiles devuelve el instrumento del prestamo de la lectura (y si hay prestamo armado).
func EstadoDelPrestamoDeTiles() (d2mpq.EstadoDelPrestamo, bool) {
	candadoDeTiles.Lock()
	defer candadoDeTiles.Unlock()

	if prestamoDeTiles == nil {
		return d2mpq.EstadoDelPrestamo{}, false
	}

	return prestamoDeTiles.Estado(), true
}

// ActivarCaminoDeTilesDeConsola prende la ruta del arena: cada .dt1 se lee adentro de un arena pedido una
// sola vez, con la lectura del MPQ por memoria prestada (DC-002.80). La llama
// `NuevoAssetManagerDeConsola`, o sea que en la consola va prendida siempre; sigue siendo una funcion (y no
// una constante) porque la palanca `BUFFER=` del POC de zonas mide con y sin la ruta, y porque el arena se
// pide en la PRIMERA carga de un .dt1 y no aca (el arranque del PVR necesita el monton ordenado, DC-002.50).
func ActivarCaminoDeTilesDeConsola() {
	caminoDeTilesDeConsola = true
}

// EstadoDelArenaDeTiles: el instrumento, para informar en la consola.
type EstadoDelArenaDeTiles struct {
	CaminoDeConsola bool
	Capacidad       int // lo que se pidio UNA vez (0 = todavia no se pidio)
	CapacidadPedida int // la reserva de DC-002.88 (0 = la constante del codigo)
	Usado           int // lo que avanzo el cursor con los .dt1 vivos
	Reinicios       int // cuantas veces se tiro el arena (cada uno vacia el cache de dt1s)
	ArchivosLeidos  int // cuantos .dt1 se cargaron por este camino

	// DC-002.88: el camino del arena de bloques usados.
	BloquesUsados    bool
	BloquesBytes     int // lo copiado al arena de bloques
	BloquesTramos    int // cuantos pedidos se hicieron
	BloquesCapacidad int // lo pedido al monton en tramos
	Scratch          int // el buffer prestado para leer cada .dt1
}

// EstadoDeLosTiles devuelve el estado del arena (para medir en la consola).
func EstadoDeLosTiles() EstadoDelArenaDeTiles {
	candadoDeTiles.Lock()
	defer candadoDeTiles.Unlock()

	capacidadTramos := 0

	for i := range arenaDeBloques {
		capacidadTramos += len(arenaDeBloques[i])
	}

	return EstadoDelArenaDeTiles{
		CaminoDeConsola:  caminoDeTilesDeConsola,
		Capacidad:        cap(arenaDeTiles),
		CapacidadPedida:  capacidadPedidaDelArena,
		Usado:            cursorDelArena,
		Reinicios:        reiniciosDelArena,
		ArchivosLeidos:   archivosDeTiles,
		BloquesUsados:    bloquesUsados,
		BloquesBytes:     bytesDeBloques,
		BloquesTramos:    len(arenaDeBloques),
		BloquesCapacidad: capacidadTramos,
		Scratch:          len(scratchDeLectura),
	}
}

// ReservarArenaParaLosTiles (DC-002.88): reserva el arena con la suma EXACTA de los `.dt1` que se van a
// cargar, preguntando el tamano en el MPQ (tabla de bloques: no lee ni un byte de los archivos). Devuelve la
// suma; 0 si no pudo averiguar ningun tamano (ahi no se reserva nada y queda la constante).
//
// Es una FUNCION y no un metodo a proposito: gccgo 15.1 no resuelve los metodos NUEVOS de un paquete
// importado desde un paquete grande (`d2mapengine` daba "reference to undefined field or method" con
// `am.ReservarArenaParaLosTiles`, y la MISMA llamada compilaba desde `package main`; medido en el build de
// esta tarjeta). Las funciones del paquete si resuelven siempre.
//
// Las rutas son las del motor, o sea relativas a `data/global/tiles` ("act1/town/floor.dt1"). Los archivos
// que la fuente no tiene (un `.dt1` que el `.ds1` nombra y el MPQ no trae) no suman: el que los pida se va a
// enterar al cargarlos, no aca.
func ReservarArenaParaLosTiles(loader *d2loader.Loader, rutas []string) int {
	if loader == nil {
		return 0
	}

	suma := 0

	for i := range rutas {
		if tam, hay := loader.TamanoDeArchivo("/data/global/tiles/" + rutas[i]); hay {
			suma += tam
		}
	}

	if suma > 0 {
		// el candado lo toma `ReservarArenaDeTiles`
		ReservarArenaDeTiles(suma)
	}

	return suma
}

// TamanoDelDT1MasGrande: el tamano descomprimido del `.dt1` mas grande de la lista. Es lo que hay que prestar
// para leer un archivo entero con el arena de bloques usados (el prestamo es uno y se reusa entre archivos).
func TamanoDelDT1MasGrande(loader *d2loader.Loader, rutas []string) int {
	if loader == nil {
		return 0
	}

	masGrande := 0

	for i := range rutas {
		if tam, hay := loader.TamanoDeArchivo("/data/global/tiles/" + rutas[i]); hay && tam > masGrande {
			masGrande = tam
		}
	}

	return masGrande
}

// ReiniciarArenaDeTiles tira el arena y VACIA el cache de dt1s (sus entradas apuntan adentro del arena).
// Es lo que hay que llamar cuando el motor tira un nivel: ahi nadie usa mas los tiles del anterior.
func (am *AssetManager) ReiniciarArenaDeTiles() {
	candadoDeTiles.Lock()
	defer candadoDeTiles.Unlock()

	am.reiniciarArenaDeTilesSinCandado()
}

func (am *AssetManager) reiniciarArenaDeTilesSinCandado() {
	cursorDelArena = 0
	reiniciosDelArena++

	// DC-002.88: el arena de bloques usados se REINICIA con el arena (los punteros de los bloques que el motor
	// ya copio a `dt1TileData` apuntan ahi, pero el que reinicia es porque tira el nivel, o sea que nadie los
	// usa mas -- el mismo contrato del arena de DC-002.78). Los TRAMOS NO SE TIRAN: se vuelven a usar desde el
	// primero, porque el monton de C no baja y pedirlos de nuevo es lo que mataba el segundo nivel.
	tramoActual = 0
	cursorDelBloque = 0
	bytesDeBloques = 0

	if am != nil && am.dt1s != nil {
		am.dt1s.Clear()
	}
}

// cargarDT1EnLaArena: la ruta de la consola para un .dt1 -- el archivo se lee ADENTRO del arena, sin pedir
// memoria, y los bloques quedan apuntando ahi (ver el contrato en la cabecera del archivo).
func (am *AssetManager) cargarDT1EnLaArena(filePath string) (*d2dt1.DT1, error) {
	candadoDeTiles.Lock()
	defer candadoDeTiles.Unlock()

	// DC-002.88: el camino del arena de bloques usados (ver la cabecera de esa seccion). Reemplaza al arena
	// de crudos: el archivo se lee en un buffer prestado y solo se copian los bloques de los tiles del nivel.
	if bloquesUsados {
		return am.cargarDT1EnBloquesUsados(filePath)
	}

	dt1, entro, err := am.parsearDT1EnLaArena(filePath)
	if err != nil {
		return nil, err
	}

	if entro {
		archivosDeTiles++

		return dt1, nil
	}

	// no entro: se tira el arena (y con el, el cache de dt1s) y se reintenta con el arena vacio
	am.reiniciarArenaDeTilesSinCandado()

	dt1, entro, err = am.parsearDT1EnLaArena(filePath)
	if err != nil {
		return nil, err
	}

	if !entro {
		return nil, errDT1NoEntraEnLaArena
	}

	archivosDeTiles++

	return dt1, nil
}

// parsearDT1EnLaArena lee UN .dt1 en el tramo libre del arena y lo parsea ahi mismo. Devuelve `entro` en
// false cuando el archivo no entra en lo que queda del arena (sin pedir memoria de mas).
//
// DC-002.80: la lectura del archivo tambien es prestada -- `d2mpq.ReadFileInto(nombre, tramo, prestamo)`, con
// el prestamo que se reusa entre archivos. Antes (DC-002.78) se leia por el STREAM del MPQ, que pide ~2 MB
// del semiespacio del recolector por archivo y mataba la consola en medio de la carga.
func (am *AssetManager) parsearDT1EnLaArena(filePath string) (dt1 *d2dt1.DT1, entro bool, err error) {
	if arenaDeTiles == nil {
		capacidad := capacidadDeLaArenaDeTiles
		if capacidadPedidaDelArena > 0 {
			capacidad = capacidadPedidaDelArena
		}

		arenaDeTiles = make([]byte, capacidad)
	}

	if arenaDeTiles == nil {
		return nil, false, errSinArena
	}

	tramo := arenaDeTiles[cursorDelArena:]
	if len(tramo) == 0 {
		return nil, false, nil
	}

	var datos []byte

	if lecturaPrestadaDeTiles {
		datos, entro, err = am.leerDT1ConPrestamo(filePath, tramo)
	} else {
		datos, entro, err = am.leerDT1PorElStream(filePath, tramo)
	}

	if err != nil {
		return nil, false, err
	}

	if !entro {
		return nil, false, nil
	}

	// los bloques apuntan a `datos`, o sea al arena: no se copia nada
	dt1, err = d2dt1.LoadDT1(datos)
	if err != nil {
		return nil, false, err
	}

	cursorDelArena += len(datos)

	return dt1, true, nil
}

// leerDT1ConPrestamo lee el archivo ADENTRO del tramo del arena con la memoria de trabajo prestada: ni el
// destino ni la lectura piden nada. `entro` en false = el archivo no cabe en lo que queda del arena (se
// devuelve sin pedir memoria; el que llama tira el arena y reintenta).
func (am *AssetManager) leerDT1ConPrestamo(filePath string, tramo []byte) (datos []byte, entro bool, err error) {
	// si ninguna fuente sabe leer con prestamo (una carpeta del disco, por ejemplo), se vuelve al camino de
	// siempre: ahi no hay MPQ del que prestar nada
	if !am.Loader.TieneLecturaConPrestamo(filePath) {
		return am.leerDT1PorElStream(filePath, tramo)
	}

	if prestamoDeTiles == nil {
		prestamoDeTiles = &d2mpq.Scratch{}
	}

	leidos, err := am.Loader.LeerEnteroEnConPrestamo(filePath, tramo, prestamoDeTiles)
	if err != nil {
		// no entra: no es un fallo, es que hay que tirar el arena y reintentar
		if err == d2mpq.ErrNoEntraEnElBuffer {
			return nil, false, nil
		}

		return nil, false, err
	}

	return tramo[:leidos], true, nil
}

// leerDT1PorElStream es el camino de DC-002.78: se pide el archivo como stream (`LoadAsset`) y se lee adentro
// del tramo con un buffer por trozos. El DESTINO no pide memoria, pero la lectura si (~2 MB de semiespacio por
// archivo, medido). Queda como palanca de medicion.
func (am *AssetManager) leerDT1PorElStream(filePath string, tramo []byte) (datos []byte, entro bool, err error) {
	archivo, err := am.LoadAsset(filePath)
	if err != nil {
		return nil, false, err
	}

	if cerrable, ok := archivo.(io.Closer); ok {
		defer func() { _ = cerrable.Close() }()
	}

	return lectorDeTiles.LeerEn(tramo, archivo)
}

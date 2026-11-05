package d2gamescreen

// DC-002.159f -- LAS MARCAS FINAS: la basura por cuadro partida en las operaciones que la piden.
//
// POR QUE EXISTE. La sonda de DC-002.159d (`gc_159d.go`) parte el cuadro en siete fases y con eso se supo
// quien pide la memoria del caminar (8.247 B/cuadro: heroe 3.500, presente 2.271, control 1.361, plan 806).
// Lo que no dice es QUE operacion dentro de cada fase: `heroe` junta el borrado del recuadro y el compuesto
// entero (siete capas, la decodificacion de cada direccion y la superficie de cada cuadro), y `presente`
// junta la conversion del lienzo, la subida a la VRAM, el pintado, el cierre de las listas del PVR y la
// lectura del control. Esta tarjeta necesita esa particion para no optimizar lo que no se paga.
//
// COMO CUENTA. Es la MISMA cuenta que las fases (ver `gc_159d.go`): el recolector de libgodc dice los bytes
// usados del semiespacio y cuantas recolecciones hizo; entre dos marcas, lo que SUBIO es la basura que pidio
// el tramo que se cierra. Si en el medio hubo una recoleccion, el tramo se descarta (la bajada no es
// devolucion de nadie y el numero mentiria).
//
// A DIFERENCIA DE LAS FASES, las marcas NO dependen de que el cuadro este abierto: `presente` es justamente lo
// que corre ENTRE el cierre de un cuadro y la apertura del siguiente (la presentacion del PVR), asi que la
// marca tiene que poder cruzar ese borde.
//
// LAS MARCAS SON TRAMOS, NO UNA PILA: `Marcar159f(x)` cierra el tramo que estaba abierto y abre `x`. Para
// volver al tramo de arriba se lo vuelve a marcar (asi lo hace `d2asset` alrededor de una decodificacion).
//
// NO SALE EN LA PC: sin sonda (`SondaDelRecolector159d == nil`) esto es una comparacion y nada mas, y la linea
// del informe no se imprime (ver `InformeDeMarcas159f`).

import "github.com/OpenDiablo2/OpenDiablo2/d2common/d2util"

// Los tramos. El orden importa para los paquetes que marcan con un indice local: cada uno suma su BASE
// (`MarcasAssetBase159f`, `MarcasEbitenBase159f`, `MarcasAppBase159f`) al indice que usa adentro, asi que el
// orden de esos bloques tiene que coincidir con el de las constantes locales de cada paquete (que estan
// escritas una al lado de la otra justamente para poder compararlas de un vistazo).
const (
	// el resto: lo que no cae en ningun tramo marcado
	MarcaResto159f = iota

	// el cuadro del motor (pantalla_mapa_155.go / heroe_159.go)
	MarcaMapa159f    // MapRenderer.Render (con la palanca del mapa estatico, se salta)
	MarcaBorrado159f // devolver el recuadro guardado (el borrado del cuadro anterior)
	MarcaGuarda159f  // guardar lo de abajo + el instrumento que lo vuelve a leer
	MarcaEnte159f    // DibujarEnte159: el compuesto del heroe, capa por capa
	MarcaCierre159f  // el cierre del Render: la linea de estado y la del tramo GC
	MarcaControl159f // HeroeDelGate0.Avanzar (la cruceta y el SetPath)
	MarcaMotor159f   // MapEngine.Advance
	MarcaRenderer159f

	// DC-002.159f-2: el cierre partido. La marca `cierre` sola eran ~3.200 B/cuadro sin decir de que, y el
	// candidato era el propio instrumento (las dos lineas). Estas cuatro dicen cual paga.
	MarcaCierrePop159f    // target.Pop + los ms del cuadro
	MarcaCierreCuadro159f // la linea "CUADRO N ms="
	MarcaCierreEstado159f // la linea "HEROE159 cuadro=..." (el estado del heroe)
	MarcaCierreTramo159f  // la linea "GC159D cuadro=..." (el tramo del recolector)

	// DC-002.159f-3: cada linea partida en ARMARLA y MANDARLA. Si el costo esta en el camino de escritura
	// (`d2util.Logger.Write` -> `fmt.Output`), lo paga `cEnvia*`; si esta en los datos, lo paga `cArma*`.
	MarcaArmaEstado159f
	MarcaEnviaEstado159f
	MarcaArmaTramo159f
	MarcaEnviaTramo159f

	// dentro de d2asset (el indice local de ese paquete + MarcasAssetBase159f)
	MarcaAssetDecod159f // decodificar una direccion de `.dcc`
	MarcaAssetSup159f   // armar/rellenar la superficie del cuadro
	MarcaAssetResto159f // lo demas del dibujo de la animacion

	// dentro del shim del PVR (console/dc/engine/ebiten/v2), indice local + MarcasEbitenBase159f
	MarcaEbitenLeerControl159f // CerrarCuadro + LeerControl (el bus maple)
	MarcaEbitenConvertir159f   // RGBA -> RGB565 del lienzo
	MarcaEbitenSubir159f       // el bloque convertido a la VRAM
	MarcaEbitenPintar159f      // el quad del PVR
	MarcaEbitenListas159f      // PvrListFinish + PvrSceneFinish
	MarcaEbitenResto159f       // lo que queda de `presente` (la espera del cuadro, el resto del loop)

	// dentro de d2app (indice local + MarcasAppBase159f)
	MarcaAppUi159f    // ui.Advance
	MarcaAppInput159f // inputManager.Advance
	MarcaAppGui159f   // guiManager.Advance
	MarcaAppTerm159f  // terminal.Advance
	MarcaAppResto159f // lo que queda

	marcas159fTotal
)

// Las BASES: el primer indice de cada bloque de arriba. El paquete que marca con un indice local hace
// `Marcar159f(base + local)`.
const (
	MarcasAssetBase159f  = MarcaAssetDecod159f
	MarcasEbitenBase159f = MarcaEbitenLeerControl159f
	MarcasAppBase159f    = MarcaAppUi159f
)

// nombresMarcas159f: como sale cada tramo en la linea del informe (corto: la linea ya trae muchos numeros).
var nombresMarcas159f = [marcas159fTotal]string{
	"resto", "mapa", "borrado", "guarda", "ente", "cierre", "control", "motor", "renderer",
	"cPop", "cCuadro", "cEstado", "cTramo", "cArmaEst", "cEnviaEst", "cArmaTra", "cEnviaTra",
	"adDecod", "adSup", "adResto",
	"ebControl", "ebConv", "ebSubir", "ebPintar", "ebListas", "ebResto",
	"apUi", "apInput", "apGui", "apTerm", "apResto",
}

// marcas159f: la cuenta del tramo. Valores fijos (nada de mapas ni slices): el instrumento no puede pedir la
// memoria que esta midiendo.
var marcas159f struct {
	abierta bool
	id      int
	usado   int
	col     int

	acum [marcas159fTotal]int
}

// Marcar159f: cierra el tramo abierto (le carga lo que subio la memoria desde la marca anterior) y abre `id`.
//
// Se llama desde el cuadro del motor, desde `d2asset`, desde el shim del PVR y desde `d2app`: los que no
// pueden importar este paquete reciben el puente por una variable (`MarcaFina159f`) que instala el arranque de
// la consola (`console/dc/poc/gate0-pueblo/main.go`).
func Marcar159f(id int) {
	if SondaDelRecolector159d == nil {
		return
	}

	m := &marcas159f

	usado, col := SondaDelRecolector159d()

	if m.abierta && col == m.col && usado > m.usado && m.id >= 0 && m.id < marcas159fTotal {
		m.acum[m.id] += usado - m.usado
	}

	m.abierta = true
	m.id, m.usado, m.col = id, usado, col
}

// MarcasEnLaLinea159f: agrega al final de la linea del tramo los bytes de cada marca ("finas ente=123B ..."),
// y la cuenta vuelve a cero para el tramo que viene. Lo llama `EscribirElTramoGC159d`, al lado de las fases.
//
// DC-002.159f: se arma en la MISMA linea y con el mismo buffer sin `fmt` (ver d2util/linea_159f.go): la
// version que devolvia un string con `Sprintf` por marca costaba ~10 KB por tramo, o sea ~700 B/cuadro de
// basura del propio instrumento.
func MarcasEnLaLinea159f(ln *d2util.Linea159f) {
	m := &marcas159f

	ln.Tx("finas")

	for i := 0; i < marcas159fTotal; i++ {
		if m.acum[i] == 0 {
			continue
		}

		ln.Tx(" ").Tx(nombresMarcas159f[i]).Tx("=").B(m.acum[i])
	}

	m.acum = [marcas159fTotal]int{}
}

// Marcas159fDePrueba: cuantos bytes lleva acumulados una marca (para las pruebas del instrumento, y para
// poder leer un tramo sin imprimirlo).
func Marcas159fDePrueba(id int) int {
	if id < 0 || id >= marcas159fTotal {
		return 0
	}

	return marcas159f.acum[id]
}

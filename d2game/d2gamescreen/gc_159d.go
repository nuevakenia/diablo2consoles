package d2gamescreen

// DC-002.159d -- LA BASURA POR CUADRO DEL CAMINAR: LA SONDA QUE FALTABA.
//
// POR QUE EXISTE. DC-002.159c dejo el caminar del heroe vivo (cuadro=210, modo=TW, sin fatales) pero con el
// recolector corriendo una vez cada ~3-4 cuadros cuando el criterio pide una cada ~10. El numero que decide
// quien pide esa memoria NO existe en este runtime: `runtime.ReadMemStats` devuelve ceros en libgodc
// (`Alloc = 0`, medido en DC-002.9 y otra vez en `heroe_159.go`, donde el instrumento se saco por inutil).
//
// La sonda que si sirve es la del propio recolector de libgodc: `gc_stats(&used, &total, &collections)`
// (`runtime/gc_heap.c`), que devuelve
//
//	used        = alloc_ptr - space[active_space]   (los bytes del semiespacio en uso AHORA)
//	total       = space_size                        (1.835.008 B en la consola)
//	collections = gc_heap.gc_count                  (una por recoleccion REAL, la misma cuenta que las marcas
//	                                                 ARQ-MARCA gc_collect)
//
// y llega aca por el puente del arranque (`d2cGcUsado` / `d2cGcColecciones`, gate0-pueblo/csrc/shim.c), que lo
// instala en `SondaDelRecolector159d`. Sin sonda (la PC) todo esto es un no-op: cero lineas, cero cuentas.
//
// QUE CUENTA, Y COMO. Entre dos muestras el semiespacio en uso sube cuando alguien pide y BAJA cuando el
// recolector copia lo vivo a la otra mitad (en Go no hay `free`). Por eso:
//
//	la BASURA de una fase = la suma de las subidas que hubo mientras corria esa fase
//
// y las bajadas se cuentan aparte, como recolecciones. Asi la basura de las fases suma la del cuadro sin que
// una recoleccion que caiga adentro de una fase se coma lo que esa fase pidio.
//
// LAS SIETE FASES son las del bucle del motor, en el orden en que corren (ver `pantalla_mapa_155.go`):
//
//	presente  la presentacion del cuadro anterior: lo que corre entre el `Render` y el `Advance` siguiente
//	          (el shim del PVR: convertir el lienzo y subirlo a la VRAM). Se mide porque es Go tambien.
//	control   `HeroeDelGate0.Avanzar` (la cruceta por el bus maple y el camino)
//	motor     `MapEngine.Advance` (el ente: el paso, el compuesto, la animacion)
//	renderer  `MapRenderer.Advance` (la camara y la cache)
//	plan      el plan de la corrida (las palancas de la presentacion)
//	mapa      `MapRenderer.Render` (con la palanca del mapa estatico, se salta)
//	heroe     el borrado del recuadro + el compuesto del heroe
//
// (el resto del cuadro -- el `Pop`, el cronometro y esta misma linea -- cae en `presente`, que es la fase que
// junta todo lo que no es del motor.)
//
// CADA CUANTO SE INFORMA. Cada `cadaInformeGC159d` cuadros sale UNA linea con el tramo entero y el promedio por
// cuadro, que es el numero que hay que bajar a una recoleccion cada 10 cuadros:
//
//	GC159D cuadro=135 tramo=15 asignado=3521280B porCuadro=234752B col=4 vivoAhora=1213216B vivoMin=1048576B
//	        presente=0B control=0B motor=2214400B ... heroe=0B | corrida=135 basura=... col=...
//
// `vivoMin` es el piso del semiespacio en el tramo: es el vivo real (lo que el recolector no pudo tirar), el
// otro numero del criterio (< 1,2 MB de 1.835.008 B).

import "fmt"

// SondaDelRecolector159d: (bytes usados del semiespacio, recolecciones hechas). La instala el arranque de la
// consola (`console/dc/poc/gate0-pueblo/main.go`), que es el unico que tiene los `//extern` del shim del PVR.
// En la PC queda nil y este archivo no hace nada.
var SondaDelRecolector159d func() (usado, colecciones int)

// LA SONDA DE CONTROL: bytes que se piden y se tiran en cada cuadro, sin que nadie los guarde. Con esto se sabe
// si el recolector se lleva lo que ya nadie apunta (no aparece en el neto) o no (aparece, y entonces el vivo
// crece por el recolector y no por una estructura que crecio). 0 = apagada.
var (
	basuraDeControl159d      []byte
	basuraDeControl159dBytes = 0
)

// FijarBasuraDeControl159d: prende (bytes por cuadro) o apaga (0) la sonda de control de `gc_159d.go`.
func FijarBasuraDeControl159d(bytes int) { basuraDeControl159dBytes = bytes }

// las fases del cuadro, en el orden en que corren (el numero es el indice del arreglo de abajo)
const (
	fasePresente159d = iota
	faseControl159d
	faseMotor159d
	faseRenderer159d
	fasePlan159d
	faseMapa159d
	faseHeroe159d
	fases159d
)

// nombresFases159d: como sale cada fase en la linea (corto: la linea ya trae siete numeros).
var nombresFases159d = [fases159d]string{"presente", "control", "motor", "renderer", "plan", "mapa", "heroe"}

// cadaInformeGC159d: cada cuantos cuadros se imprime el tramo. El caminar corre a ~2,5 cuadros por segundo, asi
// que 15 cuadros son ~6 s de log y 14 lineas en una corrida de 180 s: se lee y no tapa nada.
const cadaInformeGC159d = 15

// gc159d: la cuenta. Es un struct de valores fijos (nada de mapas ni slices) para que el instrumento no
// contamine lo que mide: si esto pidiera memoria, la basura por cuadro incluiria la del instrumento.
var gc159d struct {
	haySonda bool // la sonda esta instalada
	abierto  bool // hay un cuadro abierto
	fase     int  // la fase que esta corriendo

	// la ultima muestra
	ultimoUsado int
	ultimasCol  int

	// el tramo que se informa
	cuadrosTramo int
	basuraTramo  int
	basuraFase   [fases159d]int
	// EL NETO (lo que QUEDA): la diferencia con signo entre el fin y el principio de la fase, contada solo en
	// los tramos de fase donde NO hubo recoleccion (con una recoleccion adentro el neto no dice nada). Es el
	// numero que separa "basura" de "fuga": la basura se recolecta, lo que queda es lo que crece.
	netoFase       [fases159d]int
	muestrasFase   [fases159d]int
	colTramo       int
	vivoMinTramo   int
	vivoMaxTramo   int
	vivoAlCerrar   int
	cuadrosTotales int
	basuraTotal    int
	colTotal       int
}

// acumular159d (interno): le carga a la fase la subida de memoria desde la ultima muestra y anota las
// recolecciones que hubo desde entonces (que son las bajadas).
func acumular159d(fase int) {
	h := &gc159d

	usado, col := SondaDelRecolector159d()

	if subida := usado - h.ultimoUsado; subida > 0 {
		h.basuraTramo += subida
		h.basuraTotal += subida

		if fase >= 0 && fase < fases159d {
			h.basuraFase[fase] += subida
		}
	}

	// el NETO solo se cuenta si en este tramo de fase no hubo recoleccion (si la hubo, la bajada no es una
	// devolucion de esta fase y el numero mentiria)
	if col == h.ultimasCol && fase >= 0 && fase < fases159d {
		h.netoFase[fase] += usado - h.ultimoUsado
		h.muestrasFase[fase]++
	}

	h.colTramo += col - h.ultimasCol
	h.ultimoUsado = usado
	h.ultimasCol = col

	if h.vivoMinTramo == 0 || usado < h.vivoMinTramo {
		h.vivoMinTramo = usado
	}

	if usado > h.vivoMaxTramo {
		h.vivoMaxTramo = usado
	}
}

// AbrirElCuadroGC159d: empieza el cuadro. Se llama al principio de `Advance`, o sea justo despues de la
// presentacion del cuadro anterior: lo que se pidio desde el cierre anterior se le carga a `presente`.
func AbrirElCuadroGC159d() {
	if SondaDelRecolector159d == nil {
		return
	}

	h := &gc159d

	switch {
	case h.haySonda && !h.abierto:
		acumular159d(fasePresente159d)
	case !h.haySonda:
		usado, col := SondaDelRecolector159d()
		h.ultimoUsado = usado
		h.ultimasCol = col
	}

	h.haySonda = true
	h.abierto = true
	h.fase = faseControl159d
	h.cuadrosTotales++
}

// MarcarFaseGC159d: cierra la fase en la que estabamos y abre la que viene, cargandole la basura a la que
// termina (es la que la pidio).
func MarcarFaseGC159d(fase int) {
	h := &gc159d
	if SondaDelRecolector159d == nil || !h.abierto {
		return
	}

	acumular159d(h.fase)

	if fase >= 0 && fase < fases159d {
		h.fase = fase
	}
}

// CerrarElCuadroGC159d: termina el cuadro y, cada `cadaInformeGC159d`, devuelve la linea del tramo (y true).
// El tramo se cierra en los dos casos: lo que cambia es si se imprime.
func CerrarElCuadroGC159d() (string, bool) {
	h := &gc159d
	if SondaDelRecolector159d == nil || !h.abierto {
		return "", false
	}

	acumular159d(h.fase)

	h.vivoAlCerrar = h.ultimoUsado
	h.abierto = false
	h.cuadrosTramo++

	if h.cuadrosTramo < cadaInformeGC159d {
		return "", false
	}

	porCuadro := 0
	if h.cuadrosTramo > 0 {
		porCuadro = h.basuraTramo / h.cuadrosTramo
	}

	linea := fmt.Sprintf("tramo=%d asignado=%dB porCuadro=%dB col=%d vivoAhora=%dB vivoMin=%dB vivoMax=%dB",
		h.cuadrosTramo, h.basuraTramo, porCuadro, h.colTramo, h.vivoAlCerrar, h.vivoMinTramo, h.vivoMaxTramo)

	for i := 0; i < fases159d; i++ {
		linea += fmt.Sprintf(" %s=%dB", nombresFases159d[i], h.basuraFase[i])
	}

	linea += fmt.Sprintf(" | corrida=%d basura=%dB col=%d", h.cuadrosTotales, h.basuraTotal, h.colTotal+h.colTramo)

	// EL NETO POR FASE: bytes que QUEDARON por cuadro en cada fase (lo que crece). Si una fase retiene, aca
	// se ve cual. Va con `n=` (las muestras sin recoleccion con las que se calculo).
	linea += " | neto"

	for i := 0; i < fases159d; i++ {
		if h.muestrasFase[i] == 0 {
			continue
		}

		linea += fmt.Sprintf(" %s=%dB/c(n=%d)", nombresFases159d[i], h.netoFase[i]/h.muestrasFase[i],
			h.muestrasFase[i])
	}

	// el tramo que viene empieza de cero (el acumulado de la corrida sigue en `basuraTotal`)
	h.cuadrosTramo = 0
	h.basuraTramo = 0
	h.basuraFase = [fases159d]int{}
	h.netoFase = [fases159d]int{}
	h.muestrasFase = [fases159d]int{}
	h.colTotal += h.colTramo
	h.colTramo = 0
	h.vivoMinTramo = 0
	h.vivoMaxTramo = 0

	return linea, true
}

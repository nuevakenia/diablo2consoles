//go:build dreamcast

// DC-002.123 -- LOS OBJETOS Y EL DROP: EL MONSTRUO SUELTA Y EL JUGADOR RECOGE.
//
// QUE HACE ESTE ARCHIVO. Con DC-002.121/.122 el bicho se dibuja, patrulla, recibe golpes y muere. Lo que faltaba
// para que el mundo RESPONDA es lo de esta tarjeta, y son dos cosas:
//
//  1. AL MORIR, EL BICHO SUELTA UN OBJETO. De donde sale el objeto NO se elige aca: la fila del Returned en
//     monstats.txt dice su clase de tesoro (`TreasureClass1` = "Act 1 H2H B") y `TreasureClassEx.txt` dice que
//     hay adentro (oro, equipo, "junk", "good") con sus probabilidades. El dado es el del motor (`d2rand`, la
//     MISMA semilla fija de DC-002.122 para que la corrida sea reproducible).
//  2. EL OBJETO SE DIBUJA EN EL SUELO Y SE RECOGE. El dibujo es el del motor: el `flippyfile` de la fila del
//     item en `weapons.txt`/`armor.txt`/`misc.txt` y su DC6 de `data/global/items/` con la paleta de unidades
//     (`d2resource.ItemGraphics`, `d2mapentity/factory.go: NewGroundItem`). La recogida es la regla del motor
//     (`d2gamescreen/ground_items.go`: `pickupRange` = 0,9 celdas, distancia euclidiana en celdas): el paso del
//     jugador pasa a menos de 0,9 celdas del objeto y el objeto deja el suelo. A donde va el objeto es
//     DC-002.124 (inventario): aca solo se cuenta.
//
// LO QUE NO ENTRA (la tarjeta lo dice): inventario, alijo, magia, afinidades y rarezas. El objeto es la FILA
// BASE de la tabla, sin tirada de calidad: la tarjeta pide "un objeto del tipo mas simple primero".
//
// DONDE VIVE CADA TABLA, Y POR QUE ASI (esto es el ADR 0001 en la practica):
//
//	monstats.txt    patch_D2.mpq   432.634 B  -- la fila del Returned con su TreasureClass1 (la de d2data.mpq
//	                                            NO trae los nombres de las clases de tesoro: sus columnas de
//	                                            drop son numeros; medido con `sonda-items-123`)
//	TreasureClassEx patch_D2.mpq    88.253 B  -- la cadena del drop
//	weapons.txt     patch_D2.mpq   118.029 B  -- el `flippyfile` del item que salga
//	armor.txt       patch_D2.mpq    76.370 B
//	misc.txt        patch_D2.mpq    55.821 B
//	Total                                 771.107 B
//
// NINGUNA de esas tablas se copia al monton de Go: se piden al `Fuente` de la consola (que las lee en un tramo del
// monton de C, con dueno) y se recorren A MANO, columna por columna, sin parsearlas a structs. Se devuelven con
// `tramo.Liberar()` en el mismo acto. Eso es lo que mide la tarjeta: el semiespacio del recolector NO crece con las
// tablas, y el monton de C solo las tiene prestadas mientras se resuelve el drop. El log imprime, tabla por tabla,
// los bytes, el crc32 y como quedo el monton de C al devolverla.
//
// LO QUE SI QUEDA VIVO es el unico objeto que salio del drop: su DC6 desarmado en indices (unos pocos KB), que es
// lo que se dibuja cada cuadro.
package main

import (
	"hash/crc32"
	"math"
	"runtime"

	"github.com/OpenDiablo2/OpenDiablo2/console/dc/fuenteconsola"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2dc6"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2math/d2rand"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2render/pvr"
)

const (
	// Las tablas, como las pide el motor (`data\global\excel\...`).
	excelMonstruosDrop = `data\global\excel\monstats.txt`
	excelTesoros       = `data\global\excel\TreasureClassEx.txt`
	excelArmas         = `data\global\excel\weapons.txt`
	excelArmaduras     = `data\global\excel\armor.txt`
	excelMisc          = `data\global\excel\misc.txt`

	// carpetaFlippy: `d2resource.ItemGraphics` ("/data/global/items"). El nombre del archivo es el `flippyfile`
	// de la fila del item, en MAYUSCULAS en el MPQ (`flphax.DC6`); el lector del MPQ busca sin caja, asi que se
	// pide en minusculas como el resto del POC.
	carpetaFlippy = "data/global/items/"

	// claseDelDrop, nivelDelDrop: la identidad del bicho en la tabla del PARCHE (`NameStr` y `Level`). En esa
	// version de monstats.txt el Returned es la fila `skeleton2`, y hay tres "Returned" (acto 1 nivel 6, acto 2
	// nivel 14 x2), asi que el nivel es parte de la identidad y no un adorno.
	claseDelDrop = "Returned"

	// codigoOro: la entrada de oro de las clases de tesoro ("gld"). El oro NO entra en esta tarjeta (es una
	// pila, su propio sistema y su propio dibujo): cuando el dado la saca, se anota y se vuelve a tirar entre las
	// entradas que NO son oro. Queda escrito en el log, no escondido.
	codigoOro = "gld"

	// maxProfundidadDrop: el tope de anidamiento de la cadena (el motor lo corta a 0x3f; aca 16 es de sobra y
	// evita un bucle si una tabla se apunta a si misma).
	maxProfundidadDrop = 16

	// maxColumnasDrop: el tope de columnas que se buscan en una cabecera.
	maxColumnasDrop = 24
)

// ============================== EL LECTOR DE TABLAS, SIN PARSEAR ==============================

// tabla abierta: el tramo del Source (bytes prestados, NO copiados) y las columnas que se van a usar.
type tablaAbierta struct {
	Ruta     string
	Bytes    []byte
	dueno    *fuenteconsola.Tramo
	columnas [maxColumnasDrop]int // el indice de campo de cada columna pedida (-1 = no esta)
	nombres  [maxColumnasDrop]string
	n        int
}

// abrirTabla: pide la tabla a la fuente y saca de su PRIMERA linea (la cabecera) el indice de cada columna pedida.
// No copia nada: `Bytes` apunta al tramo del Source y el tramo es el dueno.
func abrirTabla(fuente *fuenteconsola.Fuente, ruta string, columnas ...string) *tablaAbierta {
	tramo, err := fuente.Pedir(ruta)
	if err != nil || len(tramo.Datos) == 0 {
		println("JUEGO123: DROP TABLA-LEER FALLO ruta=", ruta)

		return nil
	}

	bytesDeTablasDelDrop += len(tramo.Datos)

	t := &tablaAbierta{Ruta: ruta, Bytes: tramo.Datos, dueno: tramo}

	cabecera := primeraLinea(t.Bytes)
	if len(cabecera) == 0 {
		println("JUEGO123: DROP TABLA-SIN-CABECERA ruta=", ruta)
		t.cerrar()

		return nil
	}

	for _, nombre := range columnas {
		if t.n >= maxColumnasDrop {
			break
		}

		t.nombres[t.n] = nombre
		t.columnas[t.n] = indiceDeColumna(cabecera, nombre)
		t.n++
	}

	return t
}

// cerrar: devuelve el tramo al monton de C. Despues de esto `Bytes` NO se puede tocar.
func (t *tablaAbierta) cerrar() {
	if t == nil || t.dueno == nil {
		return
	}

	t.dueno.Liberar()
	t.dueno = nil
	t.Bytes = nil
}

// col: el indice de campo de una columna pedida al abrir la tabla (-1 si no vino en la cabecera).
func (t *tablaAbierta) col(nombre string) int {
	for i := 0; i < t.n; i++ {
		if t.nombres[i] == nombre {
			return t.columnas[i]
		}
	}

	return -1
}

// primeraLinea: la primera linea de un .txt (hasta '\n'), sin el '\r'.
func primeraLinea(b []byte) []byte {
	for i := 0; i < len(b); i++ {
		if b[i] == '\n' {
			return recortarCR(b[:i])
		}
	}

	return recortarCR(b)
}

// recortarCR: saca el '\r' del final (los .txt del juego vienen en CRLF).
func recortarCR(b []byte) []byte {
	if n := len(b); n > 0 && b[n-1] == '\r' {
		return b[:n-1]
	}

	return b
}

// indiceDeColumna: en que posicion de la cabecera (separada por tabulaciones) esta la columna `nombre`.
func indiceDeColumna(cabecera []byte, nombre string) int {
	pos := 0
	ini := 0

	for i := 0; i <= len(cabecera); i++ {
		if i == len(cabecera) || cabecera[i] == '\t' {
			if mismoTexto(cabecera[ini:i], nombre) {
				return pos
			}

			pos++
			ini = i + 1
		}
	}

	return -1
}

// campo: el valor de la columna `i` de una linea (sin el '\r' del final).
func campo(linea []byte, i int) []byte {
	if i < 0 {
		return nil
	}

	pos := 0
	ini := 0
	linea = recortarCR(linea)

	for j := 0; j <= len(linea); j++ {
		if j == len(linea) || linea[j] == '\t' {
			if pos == i {
				return linea[ini:j]
			}

			pos++
			ini = j + 1
		}
	}

	return nil
}

// enteroDe: el valor de la columna como entero (0 si esta vacia o no es un numero).
func enteroDe(linea []byte, i int) int {
	c := campo(linea, i)
	n := 0
	negativo := false

	for k := 0; k < len(c); k++ {
		switch {
		case c[k] == '-' && k == 0:
			negativo = true
		case c[k] >= '0' && c[k] <= '9':
			n = n*10 + int(c[k]-'0')
		case c[k] == ' ':
			continue
		default:
			return n * signo(negativo)
		}
	}

	return n * signo(negativo)
}

func signo(negativo bool) int {
	if negativo {
		return -1
	}

	return 1
}

// mismoTexto: comparacion de dos campos sin caja (el MPQ y las tablas cambian de caja entre versiones).
func mismoTexto(a []byte, b string) bool {
	if len(a) != len(b) {
		return false
	}

	for i := 0; i < len(a); i++ {
		ca, cb := a[i], b[i]
		if ca >= 'A' && ca <= 'Z' {
			ca += 'a' - 'A'
		}

		if cb >= 'A' && cb <= 'Z' {
			cb += 'a' - 'A'
		}

		if ca != cb {
			return false
		}
	}

	return true
}

// texto: una copia en Go de un campo (para quedarse con el codigo y el nombre del item cuando el tramo se suelta).
func texto(b []byte) string {
	if len(b) == 0 {
		return ""
	}

	s := make([]byte, len(b))
	copy(s, b)

	return string(s)
}

// porCadaFila: recorre las filas de la tabla (salteando la cabecera) y corta cuando `f` devuelve true.
func (t *tablaAbierta) porCadaFila(f func(linea []byte) bool) {
	b := t.Bytes
	ini := 0
	primera := true

	for i := 0; i <= len(b); i++ {
		if i != len(b) && b[i] != '\n' {
			continue
		}

		linea := b[ini:i]
		ini = i + 1

		if primera {
			primera = false

			continue
		}

		if len(linea) == 0 || (len(linea) == 1 && linea[0] == '\r') {
			continue
		}

		if f(linea) {
			return
		}
	}
}

// ============================== 1. LA CLASE DE TESORO DEL BICHO ==============================

// claseDeTesoroDelBicho: la fila del Returned en monstats.txt del PARCHE, y de ahi su `TreasureClass1` (el drop
// NORMAL; (N) y (H) son las dificultades que este POC no tiene).
func claseDeTesoroDelBicho(parche *fuenteconsola.Fuente, nivel int) (clase string, monstruosBytes int, ok bool) {
	t := abrirTabla(parche, excelMonstruosDrop, "NameStr", "Level", "Code", "TreasureClass1")
	if t == nil {
		return "", 0, false
	}

	defer t.cerrar()

	monstruosBytes = len(t.Bytes)

	cName, cLevel, cCode, cTC := t.col("NameStr"), t.col("Level"), t.col("Code"), t.col("TreasureClass1")

	var encontrada []byte

	t.porCadaFila(func(linea []byte) bool {
		if !mismoTexto(campo(linea, cName), claseDelDrop) {
			return false
		}

		if cLevel >= 0 && enteroDe(linea, cLevel) != nivel {
			return false
		}

		encontrada = linea

		return true
	})

	if encontrada == nil {
		println("JUEGO123: DROP SIN-FILA-DEL-BICHO clase=", claseDelDrop, "nivel=", nivel, "tabla=", excelMonstruosDrop)

		return "", monstruosBytes, false
	}

	clase = texto(campo(encontrada, cTC))

	println("JUEGO123: DROP BICHO clase=", claseDelDrop, "nivel=", nivel, "id=", texto(campo(encontrada, 0)),
		"code=", texto(campo(encontrada, cCode)), "treasureClass1=", clase, "monstatsBytes=", monstruosBytes,
		"crc32=", int(crc32.ChecksumIEEE(t.Bytes)))

	return clase, monstruosBytes, clase != ""
}

// ============================== 2. LA CADENA DEL DROP (TreasureClassEx.txt) ==============================

// entradaDeClase: una entrada (item + probabilidad) de una fila de TreasureClassEx.txt. Se guarda como INDICES de
// bytes en el tramo de la tabla: no se copia nada y no hay structs por fila en el monton de Go.
type entradaDeClase struct {
	Item []byte // el texto del item: un recorte del tramo de la tabla (vale mientras el tramo vive)
	Prob int
}

// filaDeClase: la fila de una clase, apuntando al tramo (offsets dentro de Bytes).
type filaDeClase struct {
	Picks     int
	NoDrop    int
	Entradas  [10]entradaDeClase
	NEntradas int
}

// buscarLaClase: la fila de `nombre` en TreasureClassEx.txt (por la columna "Treasure Class"), con sus entradas.
func buscarLaClase(t *tablaAbierta, nombre string) *filaDeClase {
	cNombre := t.col("Treasure Class")
	if cNombre < 0 {
		return nil
	}

	cPicks, cNoDrop := t.col("picks"), t.col("NoDrop")
	cItem := [10]int{}
	cProb := [10]int{}

	for i := 0; i < 10; i++ {
		cItem[i] = t.col("Item" + itoa(i+1))
		cProb[i] = t.col("Prob" + itoa(i+1))
	}

	var fila *filaDeClase

	t.porCadaFila(func(linea []byte) bool {
		if !mismoTexto(campo(linea, cNombre), nombre) {
			return false
		}

		fila = &filaDeClase{
			Picks:  enteroDe(linea, cPicks),
			NoDrop: enteroDe(linea, cNoDrop),
		}

		for i := 0; i < 10; i++ {
			item := campo(linea, cItem[i])
			prob := enteroDe(linea, cProb[i])

			if len(item) == 0 || prob == 0 {
				continue
			}

			fila.Entradas[fila.NEntradas] = entradaDeClase{Item: item, Prob: prob}
			fila.NEntradas++
		}

		return true
	})

	return fila
}

// laCadena: resuelve (`resuelveClase`) y devuelve el codigo del item que sale, o "" con el motivo en `motivo`.
func laCadena(parche *fuenteconsola.Fuente, clase string, semilla uint32) (codigo, motivo string, dados []string) {
	t := abrirTabla(parche, excelTesoros, "Treasure Class", "picks", "NoDrop",
		"Item1", "Prob1", "Item2", "Prob2", "Item3", "Prob3", "Item4", "Prob4", "Item5", "Prob5",
		"Item6", "Prob6", "Item7", "Prob7", "Item8", "Prob8", "Item9", "Prob9", "Item10", "Prob10")
	if t == nil {
		return "", "sin-tabla", nil
	}

	defer t.cerrar()

	println("JUEGO123: DROP TABLA", excelTesoros, "bytes=", len(t.Bytes), "crc32=", int(crc32.ChecksumIEEE(t.Bytes)),
		"columnas=", t.n)

	dado := d2rand.New(semilla)
	actual := clase

	for prof := 0; prof < maxProfundidadDrop; prof++ {
		fila := buscarLaClase(t, actual)
		if fila == nil {
			// no es una clase: es el codigo del item (o una clase de tipo, que se resuelve despues)
			return actual, "", dados
		}

		// EL PESO DE CADA ENTRADA: la probabilidad de la tabla, menos el oro (que no entra en esta tarjeta).
		total := fila.NoDrop
		pesos := [10]int{}

		for i := 0; i < fila.NEntradas; i++ {
			e := &fila.Entradas[i]

			if mismoTexto(e.Item, codigoOro) {
				dados = append(dados, "oro-fuera("+itoa(e.Prob)+")")

				continue
			}

			pesos[i] = e.Prob
			total += e.Prob
		}

		if total <= 0 {
			return "", "clase-sin-peso", dados
		}

		tirada := int(dado.Roll(int32(total)))

		linea := "clase=" + actual + " noDrop=" + itoa(fila.NoDrop) + " total=" + itoa(total) +
			" tirada=" + itoa(tirada)

		if tirada < fila.NoDrop {
			dados = append(dados, linea+" -> NADA (cayo en NoDrop)")

			return "", "nodrop", dados
		}

		// cual entrada cayo
		acumulado := fila.NoDrop
		elegida := -1

		for i := 0; i < fila.NEntradas; i++ {
			if pesos[i] == 0 {
				continue
			}

			acumulado += pesos[i]
			if tirada < acumulado {
				elegida = i

				break
			}
		}

		if elegida < 0 {
			return "", "tirada-fuera-de-rango", dados
		}

		siguiente := texto(fila.Entradas[elegida].Item)
		dados = append(dados, linea+" -> "+siguiente+" (prob="+itoa(fila.Entradas[elegida].Prob)+")")
		actual = siguiente
	}

	return "", "demasiado-hondo", dados
}

// ============================== 3. EL ITEM EN SU TABLA (weapons / armor / misc) ==============================

// filaDelItem: la fila base del item en su tabla, mas el nombre de la tabla y su `flippyfile`.
type filaDelItem struct {
	Codigo     string
	Nombre     string
	Tabla      string
	Flippy     string
	Nivel      int
	Tipo       string
	Encontrado bool
}

// buscarElItem: busca el codigo en las TRES tablas de items, en el orden del motor (armas, armaduras, misc), y se
// queda con la PRIMERA fila que lo tenga. Si el codigo no es de ningun item (p.ej. la clase de tipo "weap3"), se
// resuelve por TIPO: el prefijo es el codigo del tipo y el numero es el nivel de la clase (ver `itemDeLaClaseDeTipo`).
func buscarElItem(parche *fuenteconsola.Fuente, codigo string, nivelBicho int) *filaDelItem {
	for _, cual := range []struct{ ruta, nombre string }{
		{excelArmas, "weapons.txt"},
		{excelArmaduras, "armor.txt"},
		{excelMisc, "misc.txt"},
	} {
		t := abrirTabla(parche, cual.ruta, "code", "name", "flippyfile", "level", "type")
		if t == nil {
			continue
		}

		println("JUEGO123: DROP TABLA", t.Ruta, "bytes=", len(t.Bytes), "crc32=", int(crc32.ChecksumIEEE(t.Bytes)))

		cCode, cName, cFlip := t.col("code"), t.col("name"), t.col("flippyfile")
		cLevel, cType := t.col("level"), t.col("type")

		var fila *filaDelItem

		t.porCadaFila(func(linea []byte) bool {
			if !mismoTexto(campo(linea, cCode), codigo) {
				return false
			}

			fila = &filaDelItem{
				Codigo: texto(campo(linea, cCode)), Nombre: texto(campo(linea, cName)),
				Tabla: cual.nombre, Flippy: texto(campo(linea, cFlip)), Nivel: enteroDe(linea, cLevel),
				Tipo: texto(campo(linea, cType)), Encontrado: true,
			}

			return true
		})

		t.cerrar()

		if fila != nil {
			return fila
		}
	}

	// no es un codigo de item: puede ser una CLASE DE TIPO ("weap3", "armo6"), que el motor genera con los items
	// cuyo nivel cae en (N-3, N]. Aca se resuelve por lo que dice la tabla, sin inventar: el primer item de ese
	// tipo cuyo nivel este en el rango. Es una reduccion declarada: el generador completo de clases de tipo
	// (ITEMGEN_BuildItemTypeTreasureClasses, 0x655ae0) es de `d2drop` y no entra en esta tarjeta.
	return itemDeLaClaseDeTipo(parche, codigo)
}

// itemDeLaClaseDeTipo: "weap3" / "armo6" -> el primer item del tipo `weap`/`armo` con nivel en (N-3, N].
func itemDeLaClaseDeTipo(parche *fuenteconsola.Fuente, codigo string) *filaDelItem {
	digitos := 0

	for i := len(codigo) - 1; i >= 0 && codigo[i] >= '0' && codigo[i] <= '9'; i-- {
		digitos++
	}

	if digitos == 0 || digitos == len(codigo) {
		return nil
	}

	tipo := codigo[:len(codigo)-digitos]
	n := 0

	for i := len(codigo) - digitos; i < len(codigo); i++ {
		n = n*10 + int(codigo[i]-'0')
	}

	println("JUEGO123: DROP CLASE-DE-TIPO codigo=", codigo, "tipo=", tipo, "nivel=", n,
		"rango=", n-3, "a", n)

	for _, cual := range []struct{ ruta, nombre string }{
		{excelArmas, "weapons.txt"},
		{excelArmaduras, "armor.txt"},
	} {
		t := abrirTabla(parche, cual.ruta, "code", "name", "flippyfile", "level", "type", "type2")
		if t == nil {
			continue
		}

		cCode, cName, cFlip := t.col("code"), t.col("name"), t.col("flippyfile")
		cLevel, cType, cType2 := t.col("level"), t.col("type"), t.col("type2")

		var fila *filaDelItem

		t.porCadaFila(func(linea []byte) bool {
			if !esDelTipo(linea, tipo, cType, cType2) {
				return false
			}

			nivel := enteroDe(linea, cLevel)
			if nivel <= n-3 || nivel > n {
				return false
			}

			fila = &filaDelItem{
				Codigo: texto(campo(linea, cCode)), Nombre: texto(campo(linea, cName)),
				Tabla: cual.nombre, Flippy: texto(campo(linea, cFlip)), Nivel: nivel,
				Tipo: texto(campo(linea, cType)), Encontrado: true,
			}

			return true
		})

		t.cerrar()

		if fila != nil {
			return fila
		}
	}

	return nil
}

// esDelTipo: si la fila declara ese tipo en `type` o en `type2`.
func esDelTipo(linea []byte, tipo string, cType, cType2 int) bool {
	return mismoTexto(campo(linea, cType), tipo) || mismoTexto(campo(linea, cType2), tipo)
}

// ============================== 4. EL DIBUJO DEL OBJETO (el flippy DC6) ==============================

// dropDelBicho: lo que el bicho va a soltar. Se resuelve UNA vez, al arrancar (con las tablas vivas), y lo unico
// que queda en memoria es esto: el codigo, el nombre y el DC6 desarmado en indices.
type dropDelBicho struct {
	Clase       string
	Nivel       int
	Codigo      string
	Nombre      string
	Tabla       string
	Flippy      string
	DC6Bytes    int
	DC6CRC      uint32
	Cuadros     []CuadroDelJugador
	Indices     int
	CuadroBase  int
	Soltado     bool
	Listo       bool
	Dados       []string
	Motivo      string
	BytesTablas int
	Ms          int
}

var drop *dropDelBicho

// bytesDeTablasDelDrop: TODO lo que se le pidio a la fuente para resolver el drop (monstats del parche +
// TreasureClassEx + las tablas de items que se recorren buscando la fila y el flippy). Es el numero que va al
// resumen: la suma de lo que se leyo, no una constante escrita a mano.
var bytesDeTablasDelDrop int

// itemEnElSuelo: un objeto tirado en el mapa. Lo que es POR OBJETO es su celda y su estado; el dibujo (los indices
// del DC6) es del TIPO y lo comparte el `drop`.
type itemEnElSuelo struct {
	Codigo            string
	Nombre            string
	CeldaX            float64
	CeldaY            float64
	EnElSuelo         bool
	Recogido          bool
	Quien             string
	CuadroCayo        int
	CuadroRecogida    int
	DistanciaRecogida float64

	// el cuadro que se esta pintando
	AnclaX, AnclaY int
	Caja           CajaDelJugador
	CajaCruda      CajaDelJugador
	CajaGuardada   []byte
	Pintados       int
	MsDibujo       int
}

var suelo []*itemEnElSuelo

// LO ULTIMO QUE SE MIDIO EN EL LIENZO. El criterio de la tarjeta pide que el objeto no deje rastro: el mapa de un
// cuadro con el objeto encima tiene que volver, despues de devolverlo, al crc32 que tenia ANTES de pintarlo. Se
// guarda aca para que el cierre lo pueda afirmar sin recalcular nada.
var (
	crcDelLienzoSinItems uint32 // el cuadro del mapa, antes de pintar el objeto
	crcDelLienzoConItems uint32 // el mismo cuadro con el objeto encima
	lienzoRestaurado     bool   // si el lienzo volvio, despues de devolver el objeto, a ser el del mapa
	cuadrosConItems      int    // en cuantos cuadros hubo un objeto en el piso
)

// cargarElDrop: resuelve el drop y deja el DC6 del objeto en indices. VA ANTES DE SOLTAR LAS FUENTES (main, paso 5):
// las tablas y el DC6 salen de los MPQ, y despues de `soltarLoQueElCuadroNoLee` esa fuente ya no esta.
func cargarElDrop() {
	if dropKnob != "si" {
		println("JUEGO123: DROP apagado (drop=", dropKnob, "): la corrida es la de DC-002.122")

		return
	}

	datos := fuenteDe(rutaDatos)
	parche := fuenteDe(rutaParche)

	if datos == nil || parche == nil {
		println("JUEGO123: DROP sin fuente de datos o de parche")

		return
	}

	memoria("drop-antes")
	semiespacio("drop-antes")

	t0 := int64(d2cMs())
	bytesDeTablasDelDrop = 0

	// ---- 1. el nivel del bicho (el de la tabla, el mismo con el que se le puso la vida) ----
	nivel := 6 // el de la fila Returned de monstats.txt de d2data.mpq (medido; ver `monstruo.go`)

	if len(bichos) > 0 && bichos[0].Vida != nil && bichos[0].Vida.Nivel > 0 {
		nivel = bichos[0].Vida.Nivel
	}

	// ---- 2. la clase de tesoro de su fila ----
	clase, _, ok := claseDeTesoroDelBicho(parche, nivel)

	if !ok {
		drop = &dropDelBicho{Clase: claseDelDrop, Nivel: nivel, Motivo: "sin-clase-de-tesoro"}
		println("JUEGO123: DROP SIN CLASE: el bicho suelta nada")

		return
	}

	// ---- 3. la cadena, con el dado del motor ----
	codigo, motivo, dados := laCadena(parche, clase, semillaDrop)

	for _, d := range dados {
		println("JUEGO123: DROP DADO", d)
	}

	if codigo == "" {
		drop = &dropDelBicho{Clase: clase, Nivel: nivel, Motivo: motivo, Dados: dados}
		println("JUEGO123: DROP SIN OBJETO motivo=", motivo, ": el bicho no suelta nada (el dado dijo eso)")

		return
	}

	// ---- 4. el item en su tabla y su flippy ----
	fila := buscarElItem(parche, codigo, nivel)
	if fila == nil {
		drop = &dropDelBicho{Clase: clase, Nivel: nivel, Codigo: codigo, Motivo: "sin-fila-en-las-tablas", Dados: dados}
		println("JUEGO123: DROP SIN FILA codigo=", codigo, ": no esta en weapons/armor/misc")

		return
	}

	println("JUEGO123: DROP OBJETO codigo=", fila.Codigo, "nombre=", fila.Nombre, "tabla=", fila.Tabla,
		"nivel=", fila.Nivel, "tipo=", fila.Tipo, "flippy=", fila.Flippy, "clase=", clase, "nivelBicho=", nivel)

	// ---- 5. el DC6 del flippy, con la paleta de unidades ----
	cuadros, dc6Bytes, dc6CRC, indices, okDC6 := componerElFlippy(datos, fila.Flippy)
	if !okDC6 {
		drop = &dropDelBicho{Clase: clase, Nivel: nivel, Codigo: fila.Codigo, Nombre: fila.Nombre,
			Motivo: "sin-dc6", Dados: dados, BytesTablas: bytesDeTablasDelDrop}

		println("JUEGO123: DROP SIN DC6 flippy=", fila.Flippy)

		return
	}

	drop = &dropDelBicho{
		Clase: clase, Nivel: nivel, Codigo: fila.Codigo, Nombre: fila.Nombre, Tabla: fila.Tabla,
		Flippy: fila.Flippy, DC6Bytes: dc6Bytes, DC6CRC: dc6CRC, Cuadros: cuadros, Indices: indices,
		CuadroBase: len(cuadros) - 1, Listo: true, Dados: dados, BytesTablas: bytesDeTablasDelDrop,
		Ms: int(int64(d2cMs()) - t0),
	}

	memoria("drop-despues")
	semiespacio("drop-despues")

	println("JUEGO123: DROP LISTO codigo=", drop.Codigo, "nombre=", drop.Nombre, "cuadrosDelFlippy=",
		len(drop.Cuadros), "indicesBytes=", drop.Indices, "cuadroQueSeDibuja=", drop.CuadroBase,
		"tablasBytes=", drop.BytesTablas, "ms=", drop.Ms)
}

// componerElFlippy: lee el DC6 del objeto, lo desarma en indices (1 B/pixel, el 0 transparente) y devuelve todos sus
// cuadros. El que se dibuja es el ULTIMO: el motor reproduce el flippy UNA vez y se queda en el ultimo cuadro
// (`d2mapentity/factory.go: PlayForward + SetPlayLoop(false)`), que es el objeto ya quieto en el piso.
func componerElFlippy(datos *fuenteconsola.Fuente, flippy string) (cuadros []CuadroDelJugador, dc6Bytes int,
	dc6CRC uint32, indices int, ok bool) {
	if flippy == "" {
		return nil, 0, 0, 0, false
	}

	ruta := carpetaFlippy + flippy + ".dc6"

	tramo, err := datos.Pedir(ruta)
	if err != nil || len(tramo.Datos) == 0 {
		println("JUEGO123: DROP DC6-LEER FALLO ruta=", ruta)

		return nil, 0, 0, 0, false
	}

	crudo := tramo.Datos

	dc6Bytes = len(crudo)
	dc6CRC = crc32.ChecksumIEEE(crudo)

	dc, errDC := d2dc6.Load(crudo)
	if errDC != nil {
		println("JUEGO123: DROP DC6-PARSE FALLO ruta=", ruta, errDC.Error())
		tramo.Liberar()

		return nil, dc6Bytes, dc6CRC, 0, false
	}

	println("JUEGO123: DROP DC6 ruta=", ruta, "bytes=", dc6Bytes, "crc32=", int(dc6CRC), "cuadros=", len(dc.Frames))

	for f := 0; f < len(dc.Frames); f++ {
		fr := dc.Frames[f]

		pixeles := dc.DecodeFrame(f)
		if len(pixeles) == 0 {
			continue
		}

		cu := CuadroDelJugador{
			Ancho: int(fr.Width), Alto: int(fr.Height),
			Izq: int(fr.OffsetX), Arriba: int(fr.OffsetY),
			Pixeles: pixeles, PixelesNoVacios: contarLlenos(pixeles),
			CapasDibujadas: 1, CRC: crc32.ChecksumIEEE(pixeles),
		}

		cuadros = append(cuadros, cu)
		indices += len(pixeles)

		println("JUEGO123: DROP DC6-CUADRO cuadro=", f, "de", len(dc.Frames), "caja=", cu.Ancho, "x", cu.Alto,
			"esquina=", cu.Izq, cu.Arriba, "pixeles=", cu.PixelesNoVacios, "crc32=", int(cu.CRC))
	}

	tramo.Liberar()
	d2dc6Soltar()

	if len(cuadros) == 0 {
		return nil, dc6Bytes, dc6CRC, 0, false
	}

	return cuadros, dc6Bytes, dc6CRC, indices, true
}

// d2dc6Soltar: el DC6 no tiene arena (a diferencia del DCC): `d2dc6.Load` solo guarda la cabecera y los offsets, y
// los pixeles los devuelve `DecodeFrame` con `make`. Se le pide la recoleccion al recolector despues de decodificar
// para que el tramo del archivo (que ya se devolvio al monton de C) no quede contado como vivo.
func d2dc6Soltar() {
	runtime.GC()
}

// ============================== 5. EL DROP: EL OBJETO APARECE CUANDO EL BICHO MUERE ==============================

// soltarLoQueDejoElBicho: cuando un bicho entra en el modo de muerte (DT), deja su objeto en SU celda. Se llama una
// vez por bicho y por muerte: `Soltado` es la guarda.
func soltarLoQueDejoElBicho() {
	if drop == nil || !drop.Listo || drop.Soltado || len(bichos) == 0 {
		return
	}

	for _, b := range bichos {
		if b.Vida == nil || b.Vida.HP > 0 || b.Modo == "" || b.Modo == modoBichoCamina {
			continue
		}

		cx, cy := b.Est.Celda()

		memoria("drop-antes-de-soltar")
		semiespacio("drop-antes-de-soltar")

		it := &itemEnElSuelo{
			Codigo: drop.Codigo, Nombre: drop.Nombre, CeldaX: cx, CeldaY: cy,
			EnElSuelo: true, CuadroCayo: cuadro + 1,
		}
		suelo = append(suelo, it)
		drop.Soltado = true

		memoria("drop-despues-de-soltar")
		semiespacio("drop-despues-de-soltar")

		println("JUEGO123: DROP SUELTA codigo=", it.Codigo, "nombre=", it.Nombre, "celda=", cx, cy,
			"bicho=", b.Nombre, "vida=", b.Vida.HP, "modo=", b.Modo, "cuadro=", cuadro+1,
			"montonCLibre=", int(d2cHeapLibre()), "suelo=", len(suelo))

		return
	}
}

// recogerLosItems: la regla del motor (`pickupRange`, 0,9 celdas). Aca se mide sobre el PASO del jugador y no solo
// sobre su posicion final: un cuadro de este POC son ~1,8 celdas, asi que el jugador puede pasar POR ENCIMA del
// objeto y terminar del otro lado. El objeto se recoge si el paso paso a menos de 0,9 celdas (la distancia del
// segmento al punto), que es lo que el jugador ve.
func recogerLosItems() {
	const radioDeRecogida = 0.9 // d2gamescreen/ground_items.go: pickupRange

	for _, it := range suelo {
		if !it.EnElSuelo {
			continue
		}

		for _, p := range jugadores {
			if p.Mov == nil || p.Mov.Est == nil {
				continue
			}

			ax, ay := p.Mov.Est.Celda()

			d := distanciaAlSegmento(it.CeldaX, it.CeldaY, p.IniX, p.IniY, ax, ay)
			dx, dy := ax-it.CeldaX, ay-it.CeldaY
			alFinal := math.Sqrt(dx*dx + dy*dy)

			println("JUEGO123: ITEM PASO jugador=", p.Nombre, "celdaIni=", p.IniX, p.IniY, "celdaFin=", ax, ay,
				"objeto=", it.CeldaX, it.CeldaY, "distanciaAlPaso=", d, "distanciaAlFinal=", alFinal,
				"radio=", radioDeRecogida)

			if d > radioDeRecogida && alFinal > radioDeRecogida {
				continue
			}

			it.EnElSuelo = false
			it.Recogido = true
			it.Quien = p.Nombre
			it.CuadroRecogida = cuadro + 1
			it.DistanciaRecogida = d

			if d > alFinal {
				it.DistanciaRecogida = alFinal
			}

			println("JUEGO123: ITEM RECOGIDO codigo=", it.Codigo, "nombre=", it.Nombre, "celda=", it.CeldaX,
				it.CeldaY, "jugador=", p.Nombre, "distancia=", it.DistanciaRecogida, "radio=", radioDeRecogida,
				"cuadro=", cuadro+1, "suelo=", enElSuelo(), "cayoEnElCuadro=", it.CuadroCayo,
				"aDondeVa=DC-002.124")

			break
		}
	}
}

// distanciaAlSegmento: la distancia (en celdas) del punto (px,py) al segmento (ax,ay)-(bx,by).
func distanciaAlSegmento(px, py, ax, ay, bx, by float64) float64 {
	vx, vy := bx-ax, by-ay
	largo2 := vx*vx + vy*vy

	t := 0.0
	if largo2 > 0 {
		t = ((px-ax)*vx + (py-ay)*vy) / largo2
	}

	if t < 0 {
		t = 0
	} else if t > 1 {
		t = 1
	}

	cx, cy := ax+t*vx, ay+t*vy
	dx, dy := px-cx, py-cy

	return math.Sqrt(dx*dx + dy*dy)
}

// enElSuelo: cuantos objetos siguen en el piso.
func enElSuelo() int {
	n := 0

	for _, it := range suelo {
		if it.EnElSuelo {
			n++
		}
	}

	return n
}

// ============================== 6. EL DIBUJO ==============================

// prepararElCuadroDeUnItem: el ancla, la caja y la copia de lo que hay debajo. La proyeccion es la MISMA del
// jugador y del bicho (`AnclaDelJugador`: WorldToScreenF + RenderOffset del motor), y la esquina es la del cuadro
// del DC6 (sus offsets), igual que en un compuesto.
func prepararElCuadroDeUnItem(it *itemEnElSuelo, pixeles []byte) bool {
	if drop == nil || len(drop.Cuadros) == 0 {
		return false
	}

	cu := &drop.Cuadros[drop.CuadroBase]

	it.AnclaX, it.AnclaY = AnclaDelJugador(rendererDelMapa.WorldToScreenF, it.CeldaX, it.CeldaY)
	it.CajaCruda = CajaDe(it.AnclaX, it.AnclaY, cu)
	it.Caja = recortarALaPantalla(it.CajaCruda)

	if it.Caja.Ancho() <= 0 || it.Caja.Alto() <= 0 {
		it.Pintados = 0

		return false
	}

	necesario := it.Caja.Ancho() * it.Caja.Alto() * 4
	if it.CajaGuardada == nil || len(it.CajaGuardada) < necesario {
		it.CajaGuardada = make([]byte, necesario)
	}

	n := 0

	for y := it.Caja.Y0; y < it.Caja.Y1; y++ {
		for x := it.Caja.X0; x < it.Caja.X1; x++ {
			j := (y*ancho + x) * 4
			copy(it.CajaGuardada[n:n+4], pixeles[j:j+4])

			n += 4
		}
	}

	return true
}

// dibujarLosItems: pega los objetos que estan en el piso, mide cuantos pixeles pinto y devuelve el crc32 del cuadro
// con ellos. Despues de medir DEVUELVE el mapa, para que el crc32 del mapa solo siga siendo el del host
// (1437022778): es el numero que la tarjeta pide que no se mueva.
func dibujarLosItems(malla *pvr.Surface, pixeles []byte) (crcConItems uint32, msTotal, pintados, cuantos int) {
	crcConItems = crc32De(pixeles)
	crcDelLienzoSinItems = crcConItems

	if drop == nil || !drop.Soltado || len(drop.Cuadros) == 0 {
		return crcConItems, 0, 0, 0
	}

	cu := &drop.Cuadros[drop.CuadroBase]

	for _, it := range suelo {
		if !it.EnElSuelo {
			continue
		}

		if !prepararElCuadroDeUnItem(it, pixeles) {
			continue
		}

		t1 := d2cMs()
		PintarJugador(malla, cu, it.AnclaX, it.AnclaY, palJugador[:])
		it.MsDibujo = int(d2cMs() - t1)

		msTotal += it.MsDibujo
		it.Pintados = pixelesCambiadosDelItem(it, pixeles)
		pintados += it.Pintados
		cuantos++
	}

	crcConItems = crc32De(pixeles)
	crcDelLienzoConItems = crcConItems

	for i := len(suelo) - 1; i >= 0; i-- {
		it := suelo[i]
		if !it.EnElSuelo || it.Caja.Ancho() <= 0 || it.Caja.Alto() <= 0 {
			continue
		}

		restaurarCajaDelItem(it, pixeles)
	}

	// el lienzo, ya sin el objeto, tiene que ser otra vez el del mapa
	lienzoRestaurado = crc32De(pixeles) == crcDelLienzoSinItems
	cuadrosConItems++

	return crcConItems, msTotal, pintados, cuantos
}

// restaurarCajaDelItem: devuelve la caja del objeto a lo que habia debajo.
func restaurarCajaDelItem(it *itemEnElSuelo, pixeles []byte) {
	n := 0

	for y := it.Caja.Y0; y < it.Caja.Y1; y++ {
		for x := it.Caja.X0; x < it.Caja.X1; x++ {
			j := (y*ancho + x) * 4
			copy(pixeles[j:j+4], it.CajaGuardada[n:n+4])

			n += 4
		}
	}
}

// pixelesCambiadosDelItem: cuantos pixeles de la caja del objeto NO son los del mapa.
func pixelesCambiadosDelItem(it *itemEnElSuelo, pixeles []byte) int {
	n := 0
	cambiados := 0

	for y := it.Caja.Y0; y < it.Caja.Y1; y++ {
		for x := it.Caja.X0; x < it.Caja.X1; x++ {
			j := (y*ancho + x) * 4

			if pixeles[j] != it.CajaGuardada[n] || pixeles[j+1] != it.CajaGuardada[n+1] ||
				pixeles[j+2] != it.CajaGuardada[n+2] || pixeles[j+3] != it.CajaGuardada[n+3] {
				cambiados++
			}

			n += 4
		}
	}

	return cambiados
}

// volverAPintarLosItems: el cuadro que queda en pantalla (despues de devolver el mapa).
func volverAPintarLosItems(malla *pvr.Surface) {
	if drop == nil || len(drop.Cuadros) == 0 {
		return
	}

	cu := &drop.Cuadros[drop.CuadroBase]

	for _, it := range suelo {
		if !it.EnElSuelo || it.Caja.Ancho() <= 0 || it.Caja.Alto() <= 0 {
			continue
		}

		PintarJugador(malla, cu, it.AnclaX, it.AnclaY, palJugador[:])
	}
}

// detalleDelDibujoDeLosItems: por objeto, donde cayo y cuantos pixeles pinto.
func detalleDelDibujoDeLosItems() string {
	s := ""

	for _, it := range suelo {
		if s != "" {
			s += " "
		}

		s += it.Codigo + ":celda=" + ftoa(it.CeldaX) + "," + ftoa(it.CeldaY) +
			",ancla=" + itoa(it.AnclaX) + "," + itoa(it.AnclaY) +
			",caja=" + itoa(it.Caja.X0) + "," + itoa(it.Caja.Y0) + "," + itoa(it.Caja.X1) + "," + itoa(it.Caja.Y1) +
			",pixeles=" + itoa(it.Pintados) + ",ms=" + itoa(it.MsDibujo) +
			",enElSuelo=" + siNo(it.EnElSuelo)
	}

	return s
}

// ============================== 7. EL RESUMEN (el criterio de la tarjeta) ==============================

// resumenDeLosItems: el cierre. Los tres numeros del criterio: el objeto se solto, se dibujo (pixeles) y se recogio,
// y el mapa sin objetos sigue dando el crc32 del host.
func resumenDeLosItems() {
	if dropKnob != "si" {
		println("JUEGO123: ITEM RESUMEN drop=apagado (la corrida es la de DC-002.122)")

		return
	}

	if drop == nil {
		println("JUEGO123: ITEM RESUMEN sin drop (no se resolvio)")

		return
	}

	soltados, dibujados, recogidos := 0, 0, 0

	for _, it := range suelo {
		if it.CuadroCayo > 0 {
			soltados++
		}

		if it.Pintados > 0 {
			dibujados++
		}

		if it.Recogido {
			recogidos++
		}
	}

	quien := "-"
	distancia := 0.0

	for _, it := range suelo {
		if it.Recogido {
			quien = it.Quien
			distancia = it.DistanciaRecogida
		}
	}

	println("JUEGO123: ITEM RESUMEN drop=", drop.Codigo, "nombre=", drop.Nombre, "clase=", drop.Clase,
		"listo=", drop.Listo, "soltado=", drop.Soltado, "objetosSoltados=", soltados, "objetosDibujados=",
		dibujados, "objetosRecogidos=", recogidos, "enElSuelo=", enElSuelo(), "loRecogio=", quien,
		"distancia=", distancia, "indicesDelObjetoBytes=", drop.Indices, "tablasBytes=", drop.BytesTablas,
		"dc6Bytes=", drop.DC6Bytes, "dc6crc32=", int(drop.DC6CRC))

	println("JUEGO123: ITEM CRITERIO elBichoSolto=", siNo(soltados > 0),
		"elObjetoSeDibujo=", siNo(dibujados > 0), "elJugadorLoRecogio=", siNo(recogidos > 0),
		"desaparecioDelSuelo=", siNo(recogidos > 0 && enElSuelo() == 0),
		"elObjetoNoDejaRastroEnElLienzo=", siNo(lienzoRestaurado && cuadrosConItems > 0),
		"cuadrosConObjeto=", cuadrosConItems, "crc32DelCuadroSinObjeto=", int(crcDelLienzoSinItems),
		"crc32DelCuadroConObjeto=", int(crcDelLienzoConItems),
		"crc32DelMapaSoloDeLaCorrida=", int(crcDelMapaSolo), "crc32DelHost=", crcHost,
		"codigo=", drop.Codigo, "nombre=", drop.Nombre)
}

// ============================== 8. EL INSTRUMENTO DE LA RECOGIDA ==============================

// bitsHaciaElObjeto: los BITS de la cruceta que llevan al jugador hacia el primer objeto que quede en el piso, o
// `false` si no hay ninguno. La cuenta es la inversa de la proyeccion del piso (`personaje.go: DireccionDelPiso`):
// la diferencia en celdas (dx,dy) se proyecta a la pantalla con (dx-dy)*32 y (dx+dy)*16, y de ahi sale el signo de
// cada eje de la cruceta. Son los bits del maple los que se empujan (como el boton del golpe de DC-002.122), asi
// que el camino que se mide es el real: maple -> traduccion del servicio -> `Paso` con la velocidad del exe.
func bitsHaciaElObjeto(p *participante) (uint32, bool) {
	if p == nil || p.Mov == nil || p.Mov.Est == nil {
		return 0, false
	}

	cx, cy := p.Mov.Est.Celda()

	for _, it := range suelo {
		if !it.EnElSuelo {
			continue
		}

		dx, dy := it.CeldaX-cx, it.CeldaY-cy
		px := (dx - dy) * float64(ladoCeldaX)
		py := (dx + dy) * float64(ladoCeldaY)

		bits := uint32(0)
		if px > 1 {
			bits |= bitDerecha
		} else if px < -1 {
			bits |= bitIzquierda
		}

		if py > 1 {
			bits |= bitAbajo
		} else if py < -1 {
			bits |= bitArriba
		}

		return bits, bits != 0
	}

	return 0, false
}

package main

// EL PERSONAJE DEL MOTOR (DC-002.38).
//
// Este archivo es el MISMO en los dos lados: lo compila el POC de la consola
// (`console/dc/poc/personaje-mapa`, en la Dreamcast) y la herramienta del host
// (`console/dc/verif-personaje-238`). No toca el sistema de archivos ni la pantalla: recibe los pedazos ya
// decodificados (las capas del COF con sus pixeles) y devuelve (a) el COMPUESTO del personaje en indices de
// 8 bits, (b) el mapeo de la posicion del motor a pixeles de la ventana y (c) el paso del movimiento. Que sea
// el mismo archivo es lo que hace comparables los numeros entre la consola y el host (se verifica con sha256
// en la evidencia, igual que nivel.go en DC-002.37).
//
// DE DONDE SALE CADA COSA (nada de esto es intuicion):
//
//   - Las CAPAS y su ORDEN DE DIBUJO no se adivinan: los dice el propio COF del personaje. El motor
//     (`d2core/d2asset/composite.go`, Composite.Render) recorre `cof.Priority[direccion][frame]`, que es una
//     lista de TIPOS de capa en el orden en que se dibujan, y dibuja cada capa que exista. Aca se hace lo
//     mismo: `Orden` es esa lista, y `Componer` pega las capas en ese orden.
//   - La CAJA de cada capa y sus PIXELES salen del decodificador del motor (`d2dcc`, que es el que usa el
//     motor para los graficos de los personajes: los heroes son DCC, no DC6 — medido con la sonda
//     `console/dc/sonda-cof-238`; el COF de la amazona tiene 8 capas y 7 con grafico). El motor dibuja TODOS
//     los frames de una direccion con la MISMA caja: `dcc_animation.go` (decodeFrame) toma
//     `dccDirection.Box`, no la caja del frame. Por eso aca la caja es la de la DIRECCION.
//   - El INDICE 0 es transparente (la convencion de Diablo II y la de este port): no pisa el piso.
//   - La PROYECCION al mapa es la del compositor del nivel (`nivel.go`, el mismo archivo que usa DC-002.37):
//     la celda (x,y) del DS1 cae en el pixel ((x-y)*32 - 80, (x+y)*16). El personaje se pega con ESA cuenta,
//     con la posicion del motor en celdas (5 subtiles por celda), asi que queda parado en el mismo suelo que
//     los tiles.
//   - La VELOCIDAD no es un numero elegido aca: es la del motor para un jugador caminando
//     (`d2common/d2path`: charstats WalkVelocity = 6, 8.8 fijo, 25 ticks por segundo -> 9,375 subtiles por
//     segundo). El movimiento es el del motor: velocidad * segundos, en la direccion pedida.

import (
	"hash/crc32"
	"math"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2cof"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2math/d2vector"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2path"
)

const (
	// subtilesPorCelda: el motor guarda las posiciones en SUBTILES y una celda del DS1 (un "tile" del motor)
	// son 5 subtiles (d2vector: subTilesPerTile = 5).
	subtilesPorCelda = 5

	// claseDelPersonaje: el personaje de la prueba es una AMAZONA (charstats 0). Las siete clases del juego
	// comparten la base 6/9 de caminar/correr (medido en d2path.PlayerBase), asi que la clase no cambia el
	// numero de la velocidad.
	claseDelPersonaje = d2path.ClassAmazon

	// tokenDelPersonaje: la carpeta del personaje en el MPQ (AM = amazona), como la arma el motor
	// (d2core/d2asset/composite.go: basePath "/data/global/chars" + token).
	tokenDelPersonaje = "AM"

	// modoDelPersonaje: la animacion que se compone (NU = neutral, el personaje parado; el COF se llama
	// amnuhth.cof: AM + NU + arma HTH).
	modoDelPersonaje = "NU"

	// valorDeLaCapa: la variante de la capa ("lit" = armadura liviana, el valor por omision del motor:
	// composite.go usa layerValue = "lit" cuando no hay equipo).
	valorDeLaCapa = "LIT"

	// armaDelPersonaje: clase de arma del COF sin arma en la mano (HTH = hand to hand).
	armaDelPersonaje = "HTH"

	// maxLadoPersonaje: tope del lienzo del compuesto. Medido: la amazona NU compuesta mide 60x76.
	maxLadoPersonaje = 256
)

// Capa: una capa del personaje YA decodificada: su caja en el espacio del COF (donde el motor la dibuja) y
// los indices de 8 bits de sus pixeles (ancho*alto, el indice 0 es transparente).
type Capa struct {
	Tipo        int // d2enum.CompositeType (HD, TR, LG, RA, ...)
	Clave       string
	Izq, Arriba int
	Ancho, Alto int
	Pixeles     []byte
}

// Personaje: el compuesto del personaje (las capas que el COF pide y que tienen grafico) y el lienzo.
type Personaje struct {
	Claves []string // el nombre de cada capa del COF, en el orden del COF
	Tipos  []int    // el tipo de cada capa (d2enum.CompositeType)
	Armas  []string // la clase de arma que el COF le pide a cada capa (no es la misma en todas)
	Orden  []int    // el orden de DIBUJO (cof.Priority): tipos de capa
	Capas  []Capa

	Ancho, Alto      int // el lienzo del compuesto
	Izq, Arriba      int // la esquina del lienzo respecto del ANCLA (los pies del personaje)
	Lienzo           []byte
	PixelesNoVacios  int
	CapasDibujadas   int
	CapasSinGrafico  int
	CapasFueraDeCaja int
}

// NuevoPersonaje: arma el compuesto vacio de una direccion y un frame del COF.
func NuevoPersonaje(cof *d2cof.COF, direccion, frame int) *Personaje {
	p := &Personaje{
		Claves: make([]string, len(cof.CofLayers)),
		Tipos:  make([]int, len(cof.CofLayers)),
		Armas:  make([]string, len(cof.CofLayers)),
	}

	for i := range cof.CofLayers {
		p.Claves[i] = cof.CofLayers[i].Type.String()
		p.Tipos[i] = int(cof.CofLayers[i].Type)
		p.Armas[i] = cof.CofLayers[i].WeaponClass.String()
	}

	// el orden de dibujo es el del COF (el motor no lo inventa: lo lee del archivo)
	if direccion >= 0 && direccion < len(cof.Priority) &&
		frame >= 0 && frame < len(cof.Priority[direccion]) {
		for _, t := range cof.Priority[direccion][frame] {
			p.Orden = append(p.Orden, int(t))
		}
	}

	return p
}

// RutaDeLaCapa: el camino del grafico de una capa, tal cual lo arma el motor
// (d2core/d2asset/composite.go, layerPaths):
//
//	<base>/<token>/<capa>/<token><capa><valor><modo><arma>.dcc   (y .dc6 como segunda opcion)
//
// El arma NO es la misma en todas las capas: el COF le pone a cada una la suya (en la amazona desarmada, el
// brazo derecho es "hth" y el resto "1ht"). Medido: con un arma fija en todas, tres capas dan "file not
// found".
func (p *Personaje) RutaDeLaCapa(indice int, ext string) string {
	return p.RutaDeLaCapaModo(indice, modoDelPersonaje, ext)
}

// RutaDeLaCapaModo: lo mismo, pero con el MODO explicito (NU, TN, TW, ...). El modo va DENTRO del nombre del
// grafico (<token><capa><valor><modo><arma>.dcc), asi que para animar (DC-002.49: TN parado, TW caminando) hay
// que poder pedir la capa de un modo distinto del que declara la constante de arriba.
func (p *Personaje) RutaDeLaCapaModo(indice int, modo, ext string) string {
	return "data/global/chars/" + tokenDelPersonaje + "/" + p.Claves[indice] + "/" +
		tokenDelPersonaje + p.Claves[indice] + valorDeLaCapa + modo + p.Armas[indice] + "." + ext
}

// Agregar: el que llama decodifico una capa (con el decodificador del motor) y la pasa. Solo entran las
// capas que el COF pide dibujar (las demas no se dibujan, igual que en el motor).
func (p *Personaje) Agregar(tipo int, clave string, izq, arriba, ancho, alto int, pixeles []byte) bool {
	if !p.pideDibujar(tipo) {
		return false
	}

	if ancho <= 0 || alto <= 0 || len(pixeles) < ancho*alto {
		p.CapasSinGrafico++

		return false
	}

	p.Capas = append(p.Capas, Capa{
		Tipo: tipo, Clave: clave, Izq: izq, Arriba: arriba,
		Ancho: ancho, Alto: alto, Pixeles: pixeles,
	})

	return true
}

// pideDibujar: si el COF pone esa capa en el orden de dibujo de esta direccion y frame.
func (p *Personaje) pideDibujar(tipo int) bool {
	if len(p.Orden) == 0 {
		return true
	}

	for _, t := range p.Orden {
		if t == tipo {
			return true
		}
	}

	return false
}

// Componer: calcula la caja del compuesto (la union de las cajas de las capas) y pega las capas EN EL ORDEN
// DEL COF. El indice 0 no pisa (es transparente).
func (p *Personaje) Componer() {
	if len(p.Capas) == 0 {
		return
	}

	izq, arriba := p.Capas[0].Izq, p.Capas[0].Arriba
	der, abajo := p.Capas[0].Izq+p.Capas[0].Ancho, p.Capas[0].Arriba+p.Capas[0].Alto

	for i := range p.Capas {
		c := &p.Capas[i]
		if c.Izq < izq {
			izq = c.Izq
		}

		if c.Arriba < arriba {
			arriba = c.Arriba
		}

		if c.Izq+c.Ancho > der {
			der = c.Izq + c.Ancho
		}

		if c.Arriba+c.Alto > abajo {
			abajo = c.Arriba + c.Alto
		}
	}

	p.Izq, p.Arriba = izq, arriba
	p.Ancho, p.Alto = der-izq, abajo-arriba

	if p.Ancho <= 0 || p.Alto <= 0 || p.Ancho > maxLadoPersonaje || p.Alto > maxLadoPersonaje {
		p.Ancho, p.Alto = 0, 0

		return
	}

	p.Lienzo = make([]byte, p.Ancho*p.Alto)

	// el orden del COF: primero la capa que va mas abajo en la lista del archivo
	for _, tipo := range p.Orden {
		for i := range p.Capas {
			c := &p.Capas[i]
			if c.Tipo != tipo {
				continue
			}

			if p.pegar(c) {
				p.CapasDibujadas++
			}
		}
	}

	// ... y las capas que el COF no liste (no deberia pasar: medido, Priority trae las 8) se pegan al final,
	// para que el compuesto no pierda pixeles por un archivo raro
	for i := range p.Capas {
		c := &p.Capas[i]
		if p.yaPegada(c.Tipo) {
			continue
		}

		if p.pegar(c) {
			p.CapasDibujadas++
		}
	}

	for _, v := range p.Lienzo {
		if v != 0 {
			p.PixelesNoVacios++
		}
	}
}

func (p *Personaje) yaPegada(tipo int) bool {
	for _, t := range p.Orden {
		if t == tipo {
			return true
		}
	}

	return false
}

// pegar: copia una capa al lienzo, en su lugar (caja - esquina del compuesto).
func (p *Personaje) pegar(c *Capa) bool {
	ox, oy := c.Izq-p.Izq, c.Arriba-p.Arriba

	if ox < 0 || oy < 0 || ox+c.Ancho > p.Ancho || oy+c.Alto > p.Alto {
		p.CapasFueraDeCaja++

		return false
	}

	for y := 0; y < c.Alto; y++ {
		fila := y * c.Ancho
		base := (oy + y) * p.Ancho

		for x := 0; x < c.Ancho; x++ {
			v := c.Pixeles[fila+x]
			if v == 0 {
				continue
			}

			p.Lienzo[base+ox+x] = v
		}
	}

	return true
}

// CRC32: el hash del compuesto ENTERO de indices: el numero que tiene que dar igual en la consola y en el host.
func (p *Personaje) CRC32() uint32 {
	return crc32.ChecksumIEEE(p.Lienzo)
}

// PixelDeLaCelda: donde cae una posicion del mapa (en CELDAS, con decimales) en el lienzo de la ventana. Es
// la MISMA cuenta que `unionIso` de nivel.go: la celda (x,y) cae en ((x-y)*32 - 80, (x+y)*16), corrida por el
// origen de la ventana.
func PixelDeLaCelda(xCelda, yCelda float64, origenX, origenY int) (int, int) {
	return int(math.Floor((xCelda-yCelda)*float64(ladoCeldaX))) - offsetIso - origenX,
		int(math.Floor((xCelda+yCelda)*float64(ladoCeldaY))) - origenY
}

// DireccionDelPiso: la direccion apretada en la PANTALLA (x a la derecha, y abajo; cada uno en -1..1) -> un
// vector UNITARIO en celdas del mapa. Es la inversa de la proyeccion del piso (nivel.go):
//
//	px = (x-y)*32   py = (x+y)*16   =>   x = (px/32 + py/16)/2   y = (py/16 - px/32)/2
//
// Asi "arriba" en el control mueve al personaje hacia arriba de la pantalla y no en diagonal, que es lo que
// el jugador ve.
func DireccionDelPiso(ejeX, ejeY float64) (float64, float64) {
	cx := (ejeX/float64(ladoCeldaX) + ejeY/float64(ladoCeldaY)) / 2
	cy := (ejeY/float64(ladoCeldaY) - ejeX/float64(ladoCeldaX)) / 2

	largo := math.Sqrt(cx*cx + cy*cy)
	if largo == 0 {
		return 0, 0
	}

	return cx / largo, cy / largo
}

// Estado: el personaje que se mueve, con las cuentas del MOTOR.
type Estado struct {
	Pos       d2vector.Position // subtiles (5 por celda): el mismo tipo que usa cualquier entidad del motor
	Velocidad float64           // subtiles por segundo (d2path)
	Distancia float64           // subtiles caminados desde el arranque
	Tiempo    float64           // segundos que estuvo caminando (la suma de los pasos que movieron)
	Pasos     int               // cuantos pasos se dieron
}

// NuevoEstado: el personaje parado en una celda del mapa.
func NuevoEstado(xCelda, yCelda float64, corriendo bool) *Estado {
	return &Estado{
		Pos:       d2vector.NewPosition(xCelda*subtilesPorCelda, yCelda*subtilesPorCelda),
		Velocidad: d2path.PlayerSubtilesPerSecond(d2path.PlayerBase(claseDelPersonaje), corriendo, 0, 0),
	}
}

// Avanzar: un paso del motor (velocidad * segundos) en la direccion (dx,dy), que tiene que venir UNITARIA en
// celdas. Devuelve los subtiles caminados en este paso.
func (e *Estado) Avanzar(dx, dy, segundos float64) float64 {
	if segundos <= 0 || (dx == 0 && dy == 0) {
		return 0
	}

	caminado := e.Velocidad * segundos
	e.Pos.Set(e.Pos.X()+dx*caminado, e.Pos.Y()+dy*caminado)
	e.Distancia += caminado
	e.Tiempo += segundos
	e.Pasos++

	return caminado
}

// Celda: la posicion en CELDAS del mapa (5 subtiles = 1 celda), con decimales. Es lo que se proyecta a
// pixeles y lo que se imprime en la evidencia.
func (e *Estado) Celda() (float64, float64) {
	return e.Pos.World().X(), e.Pos.World().Y()
}

// CeldaEntera: la celda del motor donde esta parado (el "tile" de la posicion).
func (e *Estado) CeldaEntera() (int, int) {
	t := e.Pos.Tile()

	return int(t.X()), int(t.Y())
}

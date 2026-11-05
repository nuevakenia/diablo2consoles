package d2mapengine

// DC-002.159j -- EL CENSO DE LAS CUATRO FAMILIAS DE PARED, DENTRO DEL MOTOR.
//
// POR QUE EXISTE. El censo de DC-002.159i (`console/dc/medidas/sombra-159i`) miraba las paredes que el `.ds1`
// DECLARA en cada casilla, y con eso no podia ser juez de "ninguna imagen pasa de X": el pedido de 8.394.240 B
// que tumbo la corrida de 159h era de `estilo=0 sec=0 tipo=1`, un tipo que el `.ds1` declara en 66 casillas pero
// que en el censo no aparecia porque las paredes grandes del pueblo son todas `tipo=14`.
//
// Lo que el renderer pide por casilla son CUATRO tipos de pared (izquierda, derecha y las dos mitades de la
// esquina norte). Este censo los mira los cuatro, para cada familia `(estilo, secuencia)` que el nivel usa en
// sus paredes, y hace la MISMA cuenta que `generateWallCache`:
//
//	w = 160 (tileSurfaceWidth)      h = max(|Height|, maxY - minY)      n = w*h
//
// Vive en el motor y no en un medidor del host por dos motivos: la consola lo puede correr (es el juez de
// verdad, con los datos que la consola tiene en la mano) y el host lo corre con el mismo codigo, asi que los
// dos numeros son comparables sin traducir nada.
//
// NO COMPONE IMAGENES: mide lo que el renderer pediria. Se puede correr con la cache vacia y sin PVR.

import (
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2enum"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2dt1"
)

const (
	// anchoFijoDeLaPared159j: el ancho de TODA imagen de pared/techo en el renderer (`tileSurfaceWidth`).
	anchoFijoDeLaPared159j = 160

	// offsetDeBloque159j: lo que se le suma al `Y` de cada bloque para el borde de abajo (`blockOffsetY`).
	offsetDeBloque159j = 32
)

// TechoDeUnaImagen159j: el techo por imagen del Gate 0 (`bufferDelVuelo` del POC, DC-002.159i: 2^16 + 2^14).
// Es el numero con el que se decidio que el buffer al vuelo no crezca mas.
const TechoDeUnaImagen159j = 81920

// ResumenDeUnTipoDePared159j: lo que el censo junta por tipo de pared.
type ResumenDeUnTipoDePared159j struct {
	Imagenes int
	Bytes    int
	Maxima   int
	Cual     string // "estilo=0 sec=0 tipo=1 var=0 160x288" de la imagen mas grande de ese tipo
}

// CensoDeParedes159j: el censo de las cuatro familias.
type CensoDeParedes159j struct {
	Familias  int
	Imagenes  int
	Bytes     int
	Maxima    int
	Cual      string
	PorTipo   map[int]ResumenDeUnTipoDePared159j
	SobreTope int // imagenes que pasan `TechoDeUnaImagen159j`
	Tope      int

	// La pared que delato el defecto (DC-002.159i: `estilo=0 sec=0 tipo=1` pidiendo 8.394.240 B). Se mide
	// SIEMPRE, aunque su familia no este entre las que el `.ds1` declara: es el criterio 1 de la tarjeta y no
	// puede depender de que la camara la componga.
	Testigo         string
	TestigoImagen   int
	TestigoElegible bool
}

// CensoDeLasCuatroFamiliasDePared159j recorre la MISMA lista de familias de pared que compone el renderer y
// mide las imagenes de los cuatro tipos. `m` tiene que tener el nivel compuesto (casillas y `.dt1` cargados).
func (m *MapEngine) CensoDeLasCuatroFamiliasDePared159j() CensoDeParedes159j {
	censo := CensoDeParedes159j{PorTipo: map[int]ResumenDeUnTipoDePared159j{}, Tope: TechoDeUnaImagen159j}

	if m == nil {
		return censo
	}

	// las familias: (estilo, secuencia) de las paredes que las casillas declaran de verdad
	vistas := map[[2]int]bool{}

	for i := range m.tiles {
		for j := range m.tiles[i].Components.Walls {
			w := &m.tiles[i].Components.Walls[j]
			if w.Hidden() || w.Prop1 == 0 || w.Type.Special() {
				continue
			}

			vistas[[2]int{int(w.Style), int(w.Sequence)}] = true
		}
	}

	censo.Familias = len(vistas)

	// la pared testigo, medida aparte (ver el comentario del campo). DC-002.159m: la busqueda es la de
	// `tilesDeLaClave`, sin el aviso del motor -- el censo pregunta los cuatro tipos de cada familia para
	// MEDIR, y con el aviso adentro llenaba el log de `Unknown tile ID` que despues no se pueden distinguir de
	// los del renderer.
	if opciones := m.tilesDeLaClave(0, 0, d2enum.TileLeftWall); len(opciones) > 0 {
		t := &opciones[0]
		minY, maxY := bordesDeLosBloques159j(t)
		alto := absDe159j(t.Height)
		if rango := maxY - minY; rango > alto {
			alto = rango
		}

		censo.TestigoElegible = alto > 0
		censo.TestigoImagen = anchoFijoDeLaPared159j * int(alto)
		censo.Testigo = "bloques=" + itoa159j(len(t.Blocks)) + " minY=" + itoa159j(int(minY)) + " maxY=" +
			itoa159j(int(maxY)) + " realHeight=" + itoa159j(int(alto))
	}

	for familia := range vistas {
		for _, tipo := range losCuatroTiposDePared159j {
			opciones := m.tilesDeLaClave(familia[0], familia[1], tipo)

			for v := range opciones {
				t := &opciones[v]

				minY, maxY := int32(0), int32(0)

				for _, b := range t.Blocks {
					if int32(b.Y) < minY {
						minY = int32(b.Y)
					}

					if int32(b.Y)+offsetDeBloque159j > maxY {
						maxY = int32(b.Y) + offsetDeBloque159j
					}
				}

				alto := absDe159j(t.Height)
				if rango := maxY - minY; rango > alto {
					alto = rango
				}

				if alto == 0 {
					continue // sin grafico (marcador de colision): el renderer no compone nada
				}

				n := anchoFijoDeLaPared159j * int(alto)

				censo.Imagenes++
				censo.Bytes += n

				acc := censo.PorTipo[int(tipo)]
				acc.Imagenes++
				acc.Bytes += n

				if n > acc.Maxima {
					acc.Maxima = n
					acc.Cual = cualDeLaImagen159j(familia[0], familia[1], int(tipo), v, alto)
				}

				censo.PorTipo[int(tipo)] = acc

				if n > censo.Maxima {
					censo.Maxima = n
					censo.Cual = cualDeLaImagen159j(familia[0], familia[1], int(tipo), v, alto)
				}

				if n > TechoDeUnaImagen159j {
					censo.SobreTope++
				}
			}
		}
	}

	return censo
}

func cualDeLaImagen159j(estilo, secuencia, tipo, variante int, alto int32) string {
	return "estilo=" + itoa159j(estilo) + " sec=" + itoa159j(secuencia) + " tipo=" + itoa159j(tipo) +
		" var=" + itoa159j(variante) + " " + itoa159j(anchoFijoDeLaPared159j) + "x" + itoa159j(int(alto))
}

// itoa159j: entero a texto sin `strconv` (el paquete ya no tiene que crecer por un instrumento).
func itoa159j(n int) string {
	if n == 0 {
		return "0"
	}

	negativo := n < 0
	if negativo {
		n = -n
	}

	var digitos [12]byte

	i := len(digitos)
	for n > 0 {
		i--
		digitos[i] = byte('0' + n%10)
		n /= 10
	}

	if negativo {
		i--
		digitos[i] = '-'
	}

	return string(digitos[i:])
}

// bordesDeLosBloques159j: el `minY`/`maxY` de una imagen de pared, como lo calcula `generateWallCache`
// (`blockOffsetY` es 32: el alto de cada bloque).
func bordesDeLosBloques159j(t *d2dt1.Tile) (minY, maxY int32) {
	for _, b := range t.Blocks {
		if int32(b.Y) < minY {
			minY = int32(b.Y)
		}

		if int32(b.Y)+offsetDeBloque159j > maxY {
			maxY = int32(b.Y) + offsetDeBloque159j
		}
	}

	return minY, maxY
}

func absDe159j(n int32) int32 {
	if n < 0 {
		return -n
	}

	return n
}

// losCuatroTiposDePared159j: izquierda, derecha y las dos mitades de la esquina norte (`d2enum`).
var losCuatroTiposDePared159j = []d2enum.TileType{
	d2enum.TileLeftWall,
	d2enum.TileRightWall,
	d2enum.TileRightPartOfNorthCornerWall,
	d2enum.TileLeftPartOfNorthCornerWall,
}

// Linea159j: el censo en una linea, para el log del arranque de la consola.
func (c CensoDeParedes159j) Linea159j() string {
	linea := "familias=" + itoa159j(c.Familias) + " imagenes=" + itoa159j(c.Imagenes) + " bytes=" +
		itoa159j(c.Bytes) + " maxima=" + itoa159j(c.Maxima) + " (" + c.Cual + ") sobreElTecho=" +
		itoa159j(c.SobreTope) + " techo=" + itoa159j(c.Tope) + "; testigo 0/0/1: " + c.Testigo + " imagen=" +
		itoa159j(c.TestigoImagen) + " B"

	for _, tipo := range losCuatroTiposDePared159j {
		acc, hay := c.PorTipo[int(tipo)]
		if !hay {
			continue
		}

		linea += "; tipo=" + itoa159j(int(tipo)) + " imagenes=" + itoa159j(acc.Imagenes) + " bytes=" +
			itoa159j(acc.Bytes) + " maxima=" + itoa159j(acc.Maxima) + " (" + acc.Cual + ")"
	}

	return linea
}

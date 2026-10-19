//go:build dreamcast

// DC-002.38 -- EL PERSONAJE EN EL MAPA, MOVIENDOSE CON EL CONTROL DE LA DREAMCAST.
//
// LA CADENA QUE SE MIDE (la misma que usa el juego, con los decodificadores y el compositor del MOTOR):
//
//	1. el NIVEL: el .ds1 del acto 1 del MPQ del disco -> d2ds1 (el parseador del motor) -> los DT1 que el
//	   propio DS1 lista -> d2dt1 -> el compositor `nivel.go` (el MISMO archivo que el verificador del host);
//	2. el PERSONAJE: el COF del heroe (data\global\CHARS\AM\COF\amnuhth.cof, del d2char.mpq) -> las capas y
//	   su ORDEN DE DIBUJO -> cada capa a su .dcc (o .dc6) -> d2dcc/d2dc6 (los decodificadores del motor) ->
//	   el compuesto `personaje.go` (el MISMO archivo que el verificador del host);
//	3. la POSICION: d2vector.Position (subtiles, 5 por celda) que avanza con la velocidad del motor
//	   (d2path: 9,375 subtiles por segundo caminando) en la direccion que pide el CONTROL de la Dreamcast;
//	4. el DIBUJO: el mapa y el personaje ENCIMA, por el camino del motor (ebiten.Image + DrawImage), que es
//	   el loop que va a correr el juego (ebiten.RunGame).
//
// LOS NUMEROS (la consola serie es la unica salida; son los mismos que imprime
// `console/dc/verif-personaje-238`, uno por uno):
//
//	- el .ds1 y los DT1 (bytes y CRC-32) y la composicion del nivel (pedidos/compuestos/sin tile/fuera,
//	  pixeles no vacios y CRC-32 del lienzo);
//	- el COF (bytes, CRC-32, capas, frames, direcciones) y cada capa del personaje (bytes, CRC-32, caja,
//	  pixeles no vacios);
//	- el compuesto del personaje (tamano, esquina, capas dibujadas, pixeles no vacios, CRC-32);
//	- la velocidad del motor y la POSICION cuadro a cuadro: subtiles, celda, tile y el pixel del ancla en la
//	  ventana y en la pantalla;
//	- la tabla del CONTROL (los bits del control de la Dreamcast, como en DC-002.11), cada vez que cambia.
//
// MUTANTE (criterio 3): con CONTROL_SORDO=true (lo pone el script, recompilando) el control SE LEE y SE
// INFORMA, pero NO se engancha al movimiento: con las mismas teclas apretadas, la posicion del motor no
// cambia (la distancia caminada queda en 0). Es el mutante de la PIEZA que se esta probando (el enganche
// control -> movimiento), no del dibujado.
package main

import (
	"hash/crc32"
	"image/color"
	"runtime"
	"unsafe"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2cof"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2dcc"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2dc6"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2ds1"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2dt1"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2mpq"
)

//extern d2c_serial_init
func d2cSerialInit()

//extern d2c_ms
func d2cMs() uint

//extern d2c_vram_libre
func d2cVramLibre() uint

//extern d2c_free_external
func d2cFreeExternal(p uintptr)

const (
	// El disco: los DOS MPQ del juego que hacen falta, por ISO9660. El mapa y las paletas estan en
	// d2data.mpq; los graficos de los personajes, en d2char.mpq (medido: en d2data.mpq la carpeta chars
	// solo trae los COF de los mercenarios y las pantallas de seleccion).
	rutaMPQ      = "/cd/JUEGO/D2DATA.MPQ"
	rutaMPQChars = "/cd/JUEGO/D2CHAR.MPQ"

	// EL PRIMER NIVEL DEL ACTO 1 (el Rogue Encampment): el mismo .ds1 de DC-002.37.
	nivelEnMPQ = "data\\global\\tiles\\ACT1\\TOWN\\townN1.ds1"

	// La paleta del ACTO (mapa) y la de las UNIDADES (personajes): 768 bytes cada una, B,G,R.
	palEnMPQ      = "data/global/palette/ACT1/pal.dat"
	palUnidadesMPQ = "data/global/palette/Units/pal.dat"

	// El COF del personaje: la amazona parada y desarmada (AM + NU + HTH).
	cofEnMPQ = "data\\global\\CHARS\\AM\\COF\\amnuhth.cof"

	// La ventana isometrica y la celda que queda en el centro (las mismas de DC-002.37, para que los
	// numeros del nivel sean comparables).
	ladoVentana     = 512
	centroX, centroY = 28, 20

	// La pantalla logica del POC y donde se pega la ventana del nivel (centrada: (640-512)/2, (480-512)/2).
	ancho, alto = 640, 480
	mapaX       = (ancho - ladoVentana) / 2
	mapaY       = (alto - ladoVentana) / 2

	// La direccion y el frame del personaje que se componen.
	dirPersonaje   = 0
	framePersonaje = 0

	// ============================== LA ANIMACION (DC-002.49) ==============================
	//
	// LOS DOS MODOS DEL PUEBLO, con su reloj MEDIDO (no elegido): el censo del COF y del animdata.d2
	// (console/dc/sonda-anim-249) da TN = 16 cuadros a 7,81 cuadros/s = 128,00 ms/cuadro (ciclo 2048 ms)
	// y TW = 8 cuadros a 25,00 cuadros/s = 40,00 ms/cuadro (ciclo 320 ms). El COF NO trae esos numeros en
	// su campo speed (TN: 80, TW: 0): la velocidad de la animacion la pone el animdata.d2 del motor.
	modoQuieto    = "TN"
	modoCaminando = "TW"
	msQuieto      = 128
	msCaminando   = 40

	// cuadrosAMedir: cada cuantos cuadros se deja la marca de la POSICION en la consola serie (30 cuadros
	// son medio segundo a 60, o tres segundos a 10: el ritmo real lo dice el ms de la marca).
	cuadrosAMedir = 30

	// topePaso: el paso mas grande que se acepta en un cuadro (0,1 s). Sin esto, un cuadro que tarde mas
	// (una pausa del emulador, una recoleccion) mueve al personaje de un salto y el numero de la velocidad
	// sale de cualquier lado.
	topePaso = 0.1
)

// controlSordo: EL MUTANTE del criterio 3. El script lo pone en true con un sed y recompila: el control se
// sigue leyendo y se sigue informando, pero no mueve al personaje.
var controlSordo = false

// animar: ENCIENDE EL CICLO (DC-002.49). Arranca APAGADO a proposito: con esto apagado el binario es
// exactamente el instrumento de DC-002.38 (compone NU cuadro 0 y lo deja tieso), que es lo que su corpus
// compara contra el host. El script `scripts/console/build-anim-249.sh` lo enciende con un sed y recompila,
// igual que CONTROL_SORDO; el sha256 de los dos binarios queda en la evidencia.
var animar = false

// cicloSordo: EL MUTANTE del criterio 3 de DC-002.49. El cuadro se queda en 0: el personaje QUIETO se dibuja
// siempre igual, asi que dos capturas separadas en el tiempo tienen que dar 0 pixeles de diferencia.
var cicloSordo = false

// umbralGrande: a partir de que tamaño el recolector de libgodc manda la asignacion al monton de C (64 KB,
// ver DC-002.37): esas no las recupera el recolector.
const umbralGrande = 64 * 1024

// juego: lo que el motor le pide a una pantalla (Update y Draw). Es la interfaz Game, tal cual.
type juego struct {
	mapa         *ebiten.Image // la ventana del nivel (piso + paredes), ya con los colores del acto
	personaje    *ebiten.Image // el compuesto del personaje, con el indice 0 transparente
	per          *Personaje
	est          *Estado
	cuadros      int
	msAnterior   int64
	pasosCero    int
	pasosGrandes int
	botonesAntes uint32
	movido       bool

	// ---- la animacion (DC-002.49)
	mapaAncho, mapaAlto int
	ciclos              []*ciclo // los modos cargados (TN quieto, TW caminando)
	ciclo               *ciclo   // el que se esta dibujando
	cuadroDibujado      int      // el cuadro del ciclo que esta puesto en la imagen
	msDelModo           int64    // cuando empezo el modo actual (el reloj del ciclo)
	pintado             bool     // el mapa entero ya se pinto una vez
	hayCaja             bool     // hay una caja del personaje pintada que hay que limpiar
	caja                rect     // ... y cual es
	cuadroPintado       int      // el cuadro que quedo en pantalla (para no repintar de gusto)
	msUltimoCambio      int64    // ms del ultimo cambio de cuadro (para la evidencia)
	cambios             int      // cuantos cambios de cuadro se dibujaron
	palUnidades         [256][3]uint8
}

// rect: una caja de la pantalla (x0,y0) .. (x1,y1) sin incluir el borde derecho/inferior.
type rect struct{ x0, y0, x1, y1 int }

// cuadro: un cuadro del ciclo del personaje, ya compuesto (indices de 8 bits, el 0 transparente).
type cuadro struct {
	Ancho, Alto      int
	Izq, Arriba      int
	Pixeles          []byte
	PixelesNoVacios  int
	CRC              uint32
}

// ciclo: la animacion de UN modo (TN quieto, TW caminando), cuadro por cuadro.
type ciclo struct {
	Modo        string
	MsPorCuadro int
	Cuadros     []cuadro
	Imagen      *ebiten.Image // la imagen del cuadro que se esta dibujando (se reemplaza al cambiar)
	RGBA        []byte        // el buffer de esa imagen, reusado cuadro a cuadro (no se asigna por cuadro)
}

// capaCiclo: una capa del COF con TODOS los cuadros de la direccion (para componer el ciclo entero).
type capaCiclo struct {
	tipo, izq, arriba, ancho, alto int
	clave                          string
	cuadros                        [][]byte
}

// Layout: el tamano de la pantalla.
func (j *juego) Layout(ancho, alto int) (int, int) { return ancho, alto }

// Update: un cuadro mas. Lee el control, avanza la posicion con las cuentas del MOTOR y deja la marca.
func (j *juego) Update() error {
	j.cuadros++

	ahora := int64(d2cMs())
	paso := float64(ahora-j.msAnterior) / 1000.0
	j.msAnterior = ahora

	if paso <= 0 {
		j.pasosCero++
	}

	if paso < 0 {
		paso = 0
	}

	// EL TOPE DEL PASO: se ACOTA, no se descarta. Medido en la consola: los cuadros que repintan el mapa
	// tardan ~0,17 s de tiempo de la consola (el resto, ~0,04 s), asi que descartando el paso de esos cuadros
	// el personaje avanzaba la mitad de lo que le tocaba. Acotando a topePaso, un cuadro lento aporta como
	// mucho topePaso de caminata: el juego se frena bajo carga, que es el comportamiento sano, en vez de
	// perder el movimiento.
	if paso > topePaso {
		j.pasosGrandes++
		paso = topePaso
	}

	// ---- el control: la cruceta y la palanca llegan como las flechas del teclado (control.go del backend
	// de la consola), asi que se pregunta por ellas y no por el bus maple de abajo
	ejeX, ejeY := 0.0, 0.0

	if ebiten.IsKeyPressed(ebiten.KeyLeft) {
		ejeX--
	}

	if ebiten.IsKeyPressed(ebiten.KeyRight) {
		ejeX++
	}

	if ebiten.IsKeyPressed(ebiten.KeyUp) {
		ejeY--
	}

	if ebiten.IsKeyPressed(ebiten.KeyDown) {
		ejeY++
	}

	// ---- el movimiento del MOTOR (d2path): la velocidad de la tabla del exe, en la direccion pedida
	j.movido = false

	if !controlSordo && (ejeX != 0 || ejeY != 0) && paso > 0 {
		dx, dy := DireccionDelPiso(ejeX, ejeY)
		if caminado := j.est.Avanzar(dx, dy, paso); caminado > 0 {
			j.movido = true
		}
	}

	// ---- LA ANIMACION (DC-002.49): el cuadro del ciclo que toca, por el reloj de la consola
	if animar {
		j.animar()
	}

	// ---- la tabla del control (el instrumento de DC-002.11): se informa CUANDO CAMBIA, no cada cuadro
	b := ebiten.BotonesParaLaEvidencia()
	if b != j.botonesAntes {
		cx, cy := j.est.Celda()
		println("HITO238: CONTROL botones", int(b), "ejes", int(ejeX), int(ejeY),
			"celda", cx, cy, "cuadro", j.cuadros)
		j.botonesAntes = b
	}

	// ---- la marca de la POSICION: subtiles, celda, tile y el pixel del ancla
	if j.cuadros%cuadrosAMedir == 0 || j.cuadros == 1 {
		j.marca()
	}

	return nil
}

// marca: una linea con la posicion del motor, que es lo que el corpus compara.
func (j *juego) marca() {
	cx, cy := j.est.Celda()
	tx, ty := j.est.CeldaEntera()
	ax, ay := PixelDeLaCelda(cx, cy, origenX, origenY)

	println("HITO238: POS cuadro", j.cuadros, "ms", int64(d2cMs()),
		"botones", int(ebiten.BotonesParaLaEvidencia()),
		"subtiles", j.est.Pos.X(), j.est.Pos.Y(),
		"celda", cx, cy, "tile", tx, ty,
		"ancla", ax, ay, "pantalla", mapaX+ax, mapaY+ay,
		"caminado", j.est.Distancia, "tiempo", j.est.Tiempo, "pasos", j.est.Pasos,
		"dtCero", j.pasosCero, "dtGrande", j.pasosGrandes)
}

// Draw: el cuadro. El MAPA y el PERSONAJE ENCIMA (el orden de dibujo), por el camino del motor.
//
// DOS CAMINOS, y el que corre lo dice `animar`:
//
//   - ANIMADO (DC-002.49, lo que se mide en esta tarjeta): el mapa se pinta ENTERO una sola vez, y despues
//     cada cuadro solo se toca la CAJA del personaje: se devuelve esa caja al mapa (los pixeles que habia
//     debajo, que se leen del mapa) y se pega el cuadro nuevo encima. Es lo mismo que hacia el camino de
//     abajo, pero sin volver a copiar los 512x512 del mapa 8 veces por segundo: repintar el mapa entero
//     cuesta ~0,17 s por cuadro (medido en DC-002.38) y con eso la animacion de TN (128 ms por cuadro) no
//     puede correr. La caja del personaje son 26x77 = 2.002 pixeles.
//   - EL DE DC-002.38 (animar apagado): el mapa se vuelve a pintar solo cuando el personaje se movio (o en
//     el primer cuadro): la pantalla del backend se conserva entre cuadros, asi que volver a pintar lo
//     mismo seria gastar el cuadro entero en una copia de 512x512.
func (j *juego) Draw(pantalla *ebiten.Image) {
	if animar {
		j.dibujarAnimado(pantalla)
		return
	}

	if j.cuadros > 1 && !j.movido {
		return
	}

	var op ebiten.DrawImageOptions
	op.GeoM.Translate(mapaX, mapaY)
	pantalla.DrawImage(j.mapa, &op)

	cx, cy := j.est.Celda()
	ax, ay := PixelDeLaCelda(cx, cy, origenX, origenY)

	var op2 ebiten.DrawImageOptions
	op2.GeoM.Translate(float64(mapaX+ax+j.per.Izq), float64(mapaY+ay+j.per.Arriba))
	pantalla.DrawImage(j.personaje, &op2)
}

// dibujarAnimado: el mapa entero la primera vez y, cuadro a cuadro, solo la caja del personaje.
func (j *juego) dibujarAnimado(pantalla *ebiten.Image) {
	if j.ciclo == nil || j.ciclo.Imagen == nil || len(j.ciclo.Cuadros) == 0 {
		return
	}

	if !j.pintado {
		var op ebiten.DrawImageOptions
		op.GeoM.Translate(mapaX, mapaY)
		pantalla.DrawImage(j.mapa, &op)

		j.pintado = true
	}

	// si no cambio ni el cuadro ni la posicion, la pantalla ya esta bien: no se repinta
	if j.hayCaja && j.cuadroPintado == j.cuadroDibujado && !j.movido {
		return
	}

	cu := &j.ciclo.Cuadros[j.cuadroDibujado]

	cx, cy := j.est.Celda()
	ax, ay := PixelDeLaCelda(cx, cy, origenX, origenY)
	px, py := mapaX+ax+cu.Izq, mapaY+ay+cu.Arriba

	// la caja del cuadro anterior vuelve a ser mapa (es lo que tapaba al personaje)
	if j.hayCaja {
		j.devolverAlMapa(pantalla, j.caja)
	}

	var op ebiten.DrawImageOptions
	op.GeoM.Translate(float64(px), float64(py))
	pantalla.DrawImage(j.ciclo.Imagen, &op)

	j.caja = rect{px, py, px + cu.Ancho, py + cu.Alto}
	j.hayCaja = true
	j.cuadroPintado = j.cuadroDibujado
}

// devolverAlMapa: escribe en la pantalla los pixeles del MAPA que hay debajo de una caja de la pantalla.
// Es el "borrado" del personaje sin repintar el mapa entero.
func (j *juego) devolverAlMapa(pantalla *ebiten.Image, r rect) {
	for y := r.y0; y < r.y1; y++ {
		sy := y - mapaY
		if sy < 0 || sy >= j.mapaAlto {
			continue
		}

		for x := r.x0; x < r.x1; x++ {
			sx := x - mapaX
			if sx < 0 || sx >= j.mapaAncho {
				continue
			}

			c := j.mapa.ColorAt(sx, sy)
			pantalla.SetRGBA(x, y, color.RGBA{
				R: uint8(c.R*255 + 0.5), G: uint8(c.G*255 + 0.5), B: uint8(c.B*255 + 0.5), A: 255,
			})
		}
	}
}

// animar: el modo por ESTADO (quieto TN / caminando TW) y el cuadro por RELOJ.
//
// El cuadro sale del reloj de la consola, no de un contador de cuadros del loop: si el loop se frena (un
// cuadro que tarda mas), la animacion se salta cuadros pero NO se pone mas lenta, que es lo que hace el motor
// (la animacion del motor avanza con el tiempo de juego, no con los cuadros que la consola logra pintar).
func (j *juego) animar() {
	deseado := modoQuieto
	if j.movido {
		deseado = modoCaminando
	}

	c := j.buscarCiclo(deseado)
	if c == nil || len(c.Cuadros) == 0 || c.Imagen == nil {
		return
	}

	if c != j.ciclo {
		j.ciclo = c
		j.msDelModo = int64(d2cMs())
		j.cuadroDibujado = 0
		j.cuadroPintado = -1 // fuerza el repintado en el primer cuadro del modo

		println("HITO249: MODO", c.Modo, "cuadros", len(c.Cuadros), "msPorCuadro", c.MsPorCuadro,
			"cicloMs", len(c.Cuadros)*c.MsPorCuadro, "cuadroDelLoop", j.cuadros)
	}

	ms := int64(d2cMs()) - j.msDelModo
	if ms < 0 {
		ms = 0
	}

	f := int(ms/int64(c.MsPorCuadro)) % len(c.Cuadros)

	if cicloSordo {
		f = 0 // EL MUTANTE: el ciclo no avanza
	}

	if f == j.cuadroDibujado && j.cuadroPintado == f {
		return
	}

	j.cuadroDibujado = f
	j.ponerCuadro(c, f)
}

// buscarCiclo: el ciclo de un modo, si esta cargado.
func (j *juego) buscarCiclo(modo string) *ciclo {
	for _, c := range j.ciclos {
		if c.Modo == modo {
			return c
		}
	}

	return nil
}

// ponerCuadro: el cuadro del ciclo a la imagen del personaje (indices -> RGBA, el 0 transparente) y la marca
// en la consola serie con el numero de cuadro, que es lo que el corpus compara.
func (j *juego) ponerCuadro(c *ciclo, f int) {
	cu := &c.Cuadros[f]

	for i := 0; i < cu.Ancho*cu.Alto; i++ {
		idx := cu.Pixeles[i]

		if idx == 0 {
			c.RGBA[i*4+0], c.RGBA[i*4+1], c.RGBA[i*4+2], c.RGBA[i*4+3] = 0, 0, 0, 0

			continue
		}

		c.RGBA[i*4+0], c.RGBA[i*4+1], c.RGBA[i*4+2], c.RGBA[i*4+3] =
			j.palUnidades[idx][0], j.palUnidades[idx][1], j.palUnidades[idx][2], 255
	}

	c.Imagen.ReplacePixels(c.RGBA)

	j.cambios++

	println("HITO249: ANIM modo", c.Modo, "cuadro", f, "de", len(c.Cuadros),
		"caja", cu.Ancho, cu.Alto, "esquina", cu.Izq, cu.Arriba,
		"pixeles", cu.PixelesNoVacios, "crc32", cu.CRC,
		"msDelModo", int64(d2cMs())-j.msDelModo, "cambios", j.cambios, "cuadroDelLoop", j.cuadros,
		"mutante", cicloSordo)
}

// origenX/origenY: donde cayo la ventana del nivel (los imprime el compositor).
var origenX, origenY int

func main() {
	d2cSerialInit()

	println("HITO238: arranca (DC-002.38, el personaje en el mapa moviendose con el control de la Dreamcast)")
	println("HITO238: runtime", runtime.GOOS, runtime.GOARCH)

	if controlSordo {
		println("HITO238: MUTANTE controlSordo=true (el control se lee y se informa, pero no mueve)")
	}

	// ============================== 1. el nivel ==============================
	t0 := int64(d2cMs())

	mpq, err := d2mpq.FromFile(rutaMPQ)
	if err != nil {
		println("HITO238: MPQ-ABRIR FALLO:", err.Error())
		return
	}

	defer mpq.Close()

	println("HITO238: MPQ-ABRIR OK", rutaMPQ, "ms=", int64(d2cMs())-t0)

	t1 := int64(d2cMs())

	raw, err := mpq.ReadFile(nivelEnMPQ)
	if err != nil {
		println("HITO238: DS1-LEER FALLO:", err.Error())
		return
	}

	println("HITO238: DS1-LEER OK bytes=", len(raw), "crc32=", crc32.ChecksumIEEE(raw), "ms=", int64(d2cMs())-t1)

	d, err := d2ds1.Unmarshal(raw)
	if err != nil {
		println("HITO238: DS1-PARSE FALLO:", err.Error())
		return
	}

	println("HITO238: DS1 version=", d.Version(), "act=", int(d.Act), "w=", d.Width(), "h=", d.Height(),
		"objetos=", len(d.Objects), "files=", len(d.Files))

	n := NuevoNivel(d, ladoVentana, ladoVentana, centroX, centroY)
	origenX, origenY = n.OrigenX, n.OrigenY

	println("HITO238: VENTANA lado=", n.Ancho, "origen=", n.OrigenX, n.OrigenY, "centroCelda=", centroX, centroY)
	println("HITO238: PEDIDOS pisos=", n.PisosPedidos, "paredes=", n.ParedesPedidas, "total=", len(n.Pedidos))

	// los DT1, DE A UNO (el town pide 4,35 MB de DT1: tenerlos todos vivos no entra en la consola)
	for _, f := range d.Files {
		nombre := RutaDelDT1(f)
		tDT1 := int64(d2cMs())

		rawDT1, err := LeerDelMPQ(mpq, nombre)
		if err != nil {
			println("HITO238: DT1", nombre, "FALLO:", err.Error())
			continue
		}

		dt1, err := d2dt1.LoadDT1(rawDT1)
		if err != nil {
			println("HITO238: DT1", nombre, "bytes=", len(rawDT1), "PARSE-FALLO:", err.Error())
			continue
		}

		println("HITO238: DT1", nombre, "bytes=", len(rawDT1), "crc32=", crc32.ChecksumIEEE(rawDT1),
			"tiles=", len(dt1.Tiles), "ms=", int64(d2cMs())-tDT1)

		n.Alimentar(dt1.Tiles)

		dt1 = nil

		if len(rawDT1) > umbralGrande {
			d2cFreeExternal(uintptrOf(rawDT1))
		}

		rawDT1 = nil
	}

	n.Cerrar()

	println("HITO238: COMPUESTO pisos=", n.PisosCompuestos, "/", n.PisosPedidos,
		"paredes=", n.ParedesCompuestas, "/", n.ParedesPedidas,
		"sintilePiso=", n.PisosSinTile, "sintilePared=", n.ParedesSinTile, "fuera=", n.FueraDeVentana)
	println("HITO238: LIENZO pixelesNoVacios=", n.PixelesNoVacios(), "de", n.Ancho*n.Alto, "crc32=", n.CRC32())
	println("HITO238: LIENZO pixelesPiso=", n.PixelesDelPiso, "pixelesPared=", n.PixelesDePared)

	// ============================== 2. el mapa a una imagen del motor ==============================
	tPal := int64(d2cMs())

	pb, err := LeerDelMPQ(mpq, palEnMPQ)
	if err != nil || len(pb) < 768 {
		println("HITO238: PALETA-LEER FALLO:", palEnMPQ)
		return
	}

	palActo := PaletaRGB(pb)
	pvrPal := PaletaARGB1555(palActo)
	println("HITO238: PALETA acto bytes=", len(pb), "colores=256", "crc32=", CRCPaleta(pvrPal), "ms=", int64(d2cMs())-tPal)

	mapa := ebiten.NewImage(n.Ancho, n.Alto)
	rgba := make([]byte, n.Ancho*n.Alto*4)

	for i, idx := range n.Canvas {
		r, g, b := ColorDelIndice(palActo, idx)
		rgba[i*4+0], rgba[i*4+1], rgba[i*4+2], rgba[i*4+3] = r, g, b, 255
	}

	mapa.ReplacePixels(rgba)
	rgba = nil

	// el lienzo de indices ya no hace falta: se suelta para que el recolector tenga lugar para el personaje
	n.Canvas = nil
	n.Scratch = nil

	runtime.GC()

	println("HITO238: MAPA-IMAGEN bytes=", n.Ancho*n.Alto*4, "vram_libre=", int64(d2cVramLibre()))

	// ============================== 3. el personaje ==============================
	tCoF := int64(d2cMs())

	mc, err := d2mpq.FromFile(rutaMPQChars)
	if err != nil {
		println("HITO238: MPQ-CHARS-ABRIR FALLO:", err.Error())
		return
	}

	defer mc.Close()

	println("HITO238: MPQ-CHARS-ABRIR OK", rutaMPQChars, "ms=", int64(d2cMs())-tCoF)

	rawCof, err := mc.ReadFile(cofEnMPQ)
	if err != nil {
		println("HITO238: COF-LEER FALLO:", err.Error())
		return
	}

	cof, err := d2cof.Unmarshal(rawCof)
	if err != nil {
		println("HITO238: COF-PARSE FALLO:", err.Error())
		return
	}

	println("HITO238: COF", cofEnMPQ, "bytes=", len(rawCof), "crc32=", crc32.ChecksumIEEE(rawCof),
		"capas=", cof.NumberOfLayers, "framesPorDir=", cof.FramesPerDirection,
		"dirs=", cof.NumberOfDirections, "speed=", cof.Speed)

	per := NuevoPersonaje(cof, dirPersonaje, framePersonaje)
	println("HITO238: ORDEN capas=", len(per.Claves), "orden=", len(per.Orden))

	for i := range cof.CofLayers {
		tCapa := int64(d2cMs())
		encontrada := false
		ultimaRuta, ultimoErr := "", ""

		for _, ext := range []string{"dcc", "dc6"} {
			nombre := per.RutaDeLaCapa(i, ext)

			crudo, err := LeerDelMPQ(mc, nombre)
			if err != nil {
				ultimaRuta, ultimoErr = nombre, err.Error()

				continue
			}

			if ext == "dcc" {
				dd, err := d2dcc.Load(crudo)
				if err != nil {
					println("HITO238: CAPA", per.Claves[i], "PARSE-FALLO:", err.Error())
					break
				}

				caja := dd.Directions[dirPersonaje].Box
				pix := dd.Directions[dirPersonaje].Frames[framePersonaje].PixelData

				println("HITO238: CAPA", per.Claves[i], "bytes=", len(crudo), "crc32=", crc32.ChecksumIEEE(crudo),
					"caja=", caja.Left, caja.Top, caja.Width, caja.Height, "pixeles=", noCero(pix),
					"ms=", int64(d2cMs())-tCapa)

				per.Agregar(per.Tipos[i], per.Claves[i], caja.Left, caja.Top, caja.Width, caja.Height, pix)
				encontrada = true

				// el DCC entero (16 direcciones x 8 frames) se suelta: se queda SOLO el frame que se dibuja
				dd = nil
				crudo = nil

				break
			}

			dc, err := d2dc6.Load(crudo)
			if err != nil {
				println("HITO238: CAPA", per.Claves[i], "PARSE-FALLO:", err.Error())
				break
			}

			fr := dc.Frames[framePersonaje]
			pix := dc.DecodeFrame(framePersonaje)

			println("HITO238: CAPA", per.Claves[i], "bytes=", len(crudo), "crc32=", crc32.ChecksumIEEE(crudo),
				"caja=", fr.OffsetX, fr.OffsetY, fr.Width, fr.Height, "pixeles=", noCero(pix),
				"ms=", int64(d2cMs())-tCapa)

			per.Agregar(per.Tipos[i], per.Claves[i], int(fr.OffsetX), int(fr.OffsetY),
				int(fr.Width), int(fr.Height), pix)
			encontrada = true

			dc = nil
			crudo = nil

			break
		}

		if !encontrada {
			println("HITO238: CAPA", per.Claves[i], "SIN-GRAFICO ruta=", ultimaRuta, "err=", ultimoErr)
		}

		runtime.GC()
	}

	per.Componer()

	println("HITO238: PERSONAJE compuesto=", per.Ancho, "x", per.Alto, "esquina=", per.Izq, per.Arriba,
		"capasDibujadas=", per.CapasDibujadas, "sinGrafico=", per.CapasSinGrafico,
		"pixelesNoVacios=", per.PixelesNoVacios, "crc32=", per.CRC32())

	// la paleta de las UNIDADES (la de los personajes) y el compuesto a una imagen con alfa
	pbu, err := LeerDelMPQ(mpq, palUnidadesMPQ)
	if err != nil || len(pbu) < 768 {
		println("HITO238: PALETA-UNIDADES FALLO:", palUnidadesMPQ)
		return
	}

	palUni := PaletaRGB(pbu)

	var personaje *ebiten.Image

	if per.Ancho > 0 && per.Alto > 0 {
		personaje = ebiten.NewImage(per.Ancho, per.Alto)

		crgba := make([]byte, per.Ancho*per.Alto*4)

		for i, idx := range per.Lienzo {
			if idx == 0 {
				continue // el indice 0 es TRANSPARENTE: el mapa se ve por ahi
			}

			crgba[i*4+0], crgba[i*4+1], crgba[i*4+2], crgba[i*4+3] =
				palUni[idx][0], palUni[idx][1], palUni[idx][2], 255
		}

		personaje.ReplacePixels(crgba)
	}

	println("HITO238: PERSONAJE-IMAGEN bytes=", per.Ancho*per.Alto*4, "vram_libre=", int64(d2cVramLibre()))

	// ============================== 3b. LOS CICLOS DE LA ANIMACION (DC-002.49) ==============================
	//
	// Con `animar` encendido se componen los DOS ciclos del pueblo: TN (parado, 16 cuadros) y TW (caminando,
	// 8 cuadros), cada uno con su COF y sus graficos del d2char.mpq, cuadro por cuadro, con el compositor
	// compartido. El estado (quieto / caminando) elige el ciclo, y el reloj de la consola elige el cuadro.
	var ciclos []*ciclo

	if animar {
		tCiclos := int64(d2cMs())

		for _, m := range []struct {
			modo string
			ms   int
		}{{modoQuieto, msQuieto}, {modoCaminando, msCaminando}} {
			cofModo, nombreCOF, bytesCOF, ok := cofDelModo(mc, m.modo)
			if !ok {
				println("HITO249: SIN COF PARA EL MODO", m.modo)
				continue
			}

			println("HITO249: COF DEL MODO", nombreCOF, "bytes", bytesCOF,
				"framesPorDir", cofModo.FramesPerDirection, "capas", cofModo.NumberOfLayers,
				"dirs", cofModo.NumberOfDirections)

			ci := cargarCiclo(mc, cofModo, dirPersonaje, m.modo, m.ms, palUni)
			ciclos = append(ciclos, ci)

			cofModo = nil

			runtime.GC()
		}

		println("HITO249: CICLOS cargados", len(ciclos), "ms", int64(d2cMs())-tCiclos, "mutante", cicloSordo)
	}

	// ============================== 4. el personaje y el control ==============================
	est := NuevoEstado(float64(centroX), float64(centroY), false)

	println("HITO238: MOTOR velocidad=subtilesPorSegundo=", est.Velocidad,
		"celdasPorSegundo=", est.Velocidad/subtilesPorCelda)

	cx, cy := est.Celda()
	ax, ay := PixelDeLaCelda(cx, cy, n.OrigenX, n.OrigenY)

	println("HITO238: POS-INICIAL subtiles=", est.Pos.X(), est.Pos.Y(), "celda=", cx, cy,
		"ancla=", ax, ay, "pantalla=", mapaX+ax, mapaY+ay)

	if ebiten.HayControl() {
		println("HITO238: hay control en el puerto 0")
	} else {
		println("HITO238: NO hay control en el puerto 0 (el loop corre igual)")
	}

	j := &juego{mapa: mapa, personaje: personaje, per: per, est: est, msAnterior: int64(d2cMs())}

	if animar {
		j.ciclos = ciclos
		j.palUnidades = palUni
		j.mapaAncho, j.mapaAlto = n.Ancho, n.Alto
		j.hayCaja = false
		j.cuadroPintado = -1
	}

	if err := ebiten.IniciarPantalla(ancho, alto); err != nil {
		println("HITO238: no pude arrancar el video:", err.Error())
	}

	println("HITO238: dibujando (mapa", n.Ancho, "x", n.Alto, "en", mapaX, mapaY, "; personaje", per.Ancho, "x", per.Alto, ")")

	if err := ebiten.RunGame(j); err != nil {
		println("HITO238: el loop termino con error:", err.Error())
	}
}

// noCero: cuantos pixeles del frame NO son el indice 0 (el instrumento del personaje).
func noCero(pix []byte) int {
	c := 0
	for _, v := range pix {
		if v != 0 {
			c++
		}
	}

	return c
}

// ============================== EL CICLO DEL PERSONAJE (DC-002.49) ==============================

// cofDelModo: el COF de un modo (AMTNHTH para parado, AMTWHTH para caminando), con el MISMO criterio que la
// sonda del censo: se prueba el arma HTH primero (el personaje de la prueba va desarmado).
func cofDelModo(mc *d2mpq.MPQ, modo string) (*d2cof.COF, string, int, bool) {
	for _, wc := range []string{"hth", "1hs", "bow", "stf", "2hs"} {
		nombre := `data\global\chars\AM\COF\AM` + modo + wc + `.COF`

		crudo, err := mc.ReadFile(nombre)
		if err != nil || len(crudo) == 0 {
			continue
		}

		cof, err := d2cof.Unmarshal(crudo)
		if err != nil {
			println("HITO249: COF DEL MODO", modo, "PARSE-FALLO:", err.Error())

			return nil, nombre, 0, false
		}

		return cof, nombre, len(crudo), true
	}

	return nil, "", 0, false
}

// cargarCiclo: COMPONE el ciclo entero de un modo con el compositor COMPARTIDO (personaje.go, el mismo que
// usa la herramienta del host), cuadro por cuadro. Se guardan los indices de cada cuadro (26x77 = 2.002 B
// en TN), que es lo unico que hace falta para dibujar: la imagen de ebiten se rehace cuadro a cuadro.
//
// POR QUE UN CUADRO POR VEZ Y NO EL DCC ENTERO: un DCC trae 16 direcciones x 16 cuadros de cada capa (son
// ~140 KB decodificados por capa); guardarlos todos no entra en la consola. Se decodifica la capa una vez,
// se copian los cuadros de ESTA direccion y el DCC se suelta.
func cargarCiclo(mc *d2mpq.MPQ, cof *d2cof.COF, dir int, modo string, msPorCuadro int,
	pal [256][3]uint8) *ciclo {
	n := cof.FramesPerDirection
	per0 := NuevoPersonaje(cof, dir, 0)
	capas := make([]capaCiclo, 0, len(cof.CofLayers))

	for i := range cof.CofLayers {
		tCapa := int64(d2cMs())
		encontrada := false

		for _, ext := range []string{"dcc", "dc6"} {
			nombre := per0.RutaDeLaCapaModo(i, modo, ext)

			crudo, err := LeerDelMPQ(mc, nombre)
			if err != nil {
				continue
			}

			c := capaCiclo{tipo: per0.Tipos[i], clave: per0.Claves[i]}

			if ext == "dcc" {
				dd, err := d2dcc.Load(crudo)
				if err != nil {
					println("HITO249: CAPA", c.clave, "DEL MODO", modo, "PARSE-FALLO:", err.Error())
					break
				}

				caja := dd.Directions[dir].Box
				c.izq, c.arriba, c.ancho, c.alto = caja.Left, caja.Top, caja.Width, caja.Height

				for f := 0; f < n && f < len(dd.Directions[dir].Frames); f++ {
					c.cuadros = append(c.cuadros, dd.Directions[dir].Frames[f].PixelData)
				}

				capas = append(capas, c)
				encontrada = true

				dd = nil
				crudo = nil

				break
			}

			dc, err := d2dc6.Load(crudo)
			if err != nil {
				println("HITO249: CAPA", c.clave, "DEL MODO", modo, "PARSE-FALLO:", err.Error())
				break
			}

			for f := 0; f < n && f < len(dc.Frames); f++ {
				fr := dc.Frames[f]
				if f == 0 {
					c.izq, c.arriba = int(fr.OffsetX), int(fr.OffsetY)
					c.ancho, c.alto = int(fr.Width), int(fr.Height)
				}

				c.cuadros = append(c.cuadros, dc.DecodeFrame(f))
			}

			capas = append(capas, c)
			encontrada = true

			dc = nil
			crudo = nil

			break
		}

		if encontrada {
			println("HITO249: CAPA DEL CICLO", modo, per0.Claves[i], "cuadros", len(capas[len(capas)-1].cuadros),
				"caja", capas[len(capas)-1].ancho, capas[len(capas)-1].alto, "ms", int64(d2cMs())-tCapa)
		}
	}

	c := &ciclo{Modo: modo, MsPorCuadro: msPorCuadro}

	for f := 0; f < n; f++ {
		per := NuevoPersonaje(cof, dir, f)

		for i := range capas {
			if f >= len(capas[i].cuadros) {
				continue
			}

			per.Agregar(capas[i].tipo, capas[i].clave, capas[i].izq, capas[i].arriba,
				capas[i].ancho, capas[i].alto, capas[i].cuadros[f])
		}

		per.Componer()

		cu := cuadro{
			Ancho: per.Ancho, Alto: per.Alto, Izq: per.Izq, Arriba: per.Arriba,
			Pixeles: per.Lienzo, PixelesNoVacios: per.PixelesNoVacios, CRC: per.CRC32(),
		}
		c.Cuadros = append(c.Cuadros, cu)

		println("HITO249: CUADRO DEL CICLO", modo, f, "de", n, "caja", cu.Ancho, cu.Alto,
			"esquina", cu.Izq, cu.Arriba, "pixeles", cu.PixelesNoVacios, "crc32", cu.CRC)
	}

	// los pixeles de las capas ya estan pegados en los cuadros: se sueltan
	capas = nil
	runtime.GC()

	if len(c.Cuadros) == 0 || c.Cuadros[0].Ancho == 0 {
		println("HITO249: EL CICLO", modo, "SALIO VACIO")

		return c
	}

	c.Imagen = ebiten.NewImage(c.Cuadros[0].Ancho, c.Cuadros[0].Alto)
	c.RGBA = make([]byte, c.Cuadros[0].Ancho*c.Cuadros[0].Alto*4)

	// los pixeles del primer cuadro, para que la imagen nunca este vacia
	for i, idx := range c.Cuadros[0].Pixeles {
		if idx == 0 {
			continue
		}

		c.RGBA[i*4+0], c.RGBA[i*4+1], c.RGBA[i*4+2], c.RGBA[i*4+3] =
			pal[idx][0], pal[idx][1], pal[idx][2], 255
	}

	c.Imagen.ReplacePixels(c.RGBA)

	println("HITO249: CICLO", modo, "cuadros", len(c.Cuadros), "msPorCuadro", msPorCuadro,
		"cicloMs", len(c.Cuadros)*msPorCuadro, "imagen", c.Cuadros[0].Ancho, c.Cuadros[0].Alto)

	return c
}

// uintptrOf: la direccion de los bytes de un slice, para devolverlos al monton de C (ver DC-002.37).
func uintptrOf(b []byte) uintptr {
	if len(b) == 0 {
		return 0
	}

	return uintptr(unsafe.Pointer(&b[0]))
}

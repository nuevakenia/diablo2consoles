//go:build dreamcast

// DC-002.38 -- EL PERSONAJE EN EL MAPA, MOVIENDOSE CON EL CONTROL DE LA DREAMCAST.
//
// LA CADENA QUE SE MIDE (la misma que usa el juego, con los decodificadores y el compositor del MOTOR):
//
//  1. el NIVEL: el .ds1 del acto 1 del MPQ del disco -> d2ds1 (el parseador del motor) -> los DT1 que el
//     propio DS1 lista -> d2dt1 -> el compositor `nivel.go` (el MISMO archivo que el verificador del host);
//  2. el PERSONAJE: el COF del heroe (data\global\CHARS\AM\COF\amnuhth.cof, del d2char.mpq) -> las capas y
//     su ORDEN DE DIBUJO -> cada capa a su .dcc (o .dc6) -> d2dcc/d2dc6 (los decodificadores del motor) ->
//     el compuesto `personaje.go` (el MISMO archivo que el verificador del host);
//  3. la POSICION: d2vector.Position (subtiles, 5 por celda) que avanza con la velocidad del motor
//     (d2path: 9,375 subtiles por segundo caminando) en la direccion que pide el CONTROL de la Dreamcast;
//  4. el DIBUJO: el mapa y el personaje ENCIMA, por el camino del motor (ebiten.Image + DrawImage), que es
//     el loop que va a correr el juego (ebiten.RunGame).
//
// LOS NUMEROS (la consola serie es la unica salida; son los mismos que imprime
// `console/dc/verif-personaje-238`, uno por uno):
//
//   - el .ds1 y los DT1 (bytes y CRC-32) y la composicion del nivel (pedidos/compuestos/sin tile/fuera,
//     pixeles no vacios y CRC-32 del lienzo);
//   - el COF (bytes, CRC-32, capas, frames, direcciones) y cada capa del personaje (bytes, CRC-32, caja,
//     pixeles no vacios);
//   - el compuesto del personaje (tamano, esquina, capas dibujadas, pixeles no vacios, CRC-32);
//   - la velocidad del motor y la POSICION cuadro a cuadro: subtiles, celda, tile y el pixel del ancla en la
//     ventana y en la pantalla;
//   - la tabla del CONTROL (los bits del control de la Dreamcast, como en DC-002.11), cada vez que cambia.
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

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2cof"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2dc6"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2dcc"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2ds1"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2dt1"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2mpq"
	"github.com/hajimehoshi/ebiten/v2"
)

//extern d2c_serial_init
func d2cSerialInit()

//extern d2c_ms
func d2cMs() uint

//extern d2c_vram_libre
func d2cVramLibre() uint

//extern d2c_heap_libre
func d2cHeapLibre() uint

//extern d2c_free_external
func d2cFreeExternal(p uintptr)

const (
	// El disco: los DOS MPQ del juego que hacen falta, por ISO9660. El mapa y las paletas estan en
	// d2data.mpq; los graficos de los personajes, en d2char.mpq (medido: en d2data.mpq la carpeta chars
	// solo trae los COF de los mercenarios y las pantallas de seleccion).
	rutaMPQ      = "/cd/JUEGO/D2DATA.MPQ"
	rutaMPQChars = "/cd/JUEGO/D2CHAR.MPQ"

	// DC-002.50: el TERCER MPQ, el del parche. MonLvl.txt (la tabla del nivel de los monstruos, de donde sale
	// la vida) NO esta en d2data.mpq ni en d2exp.mpq: medido con la sonda del host, vive en patch_D2.mpq.
	rutaMPQPatch = "/cd/JUEGO/PATCH_D2.MPQ"

	// Las dos tablas del excel que hacen falta para la vida del bicho (monstruo.go).
	excelMonstats = "data\\global\\excel\\monstats.txt"
	excelMonLvl   = "data\\global\\excel\\MonLvl.txt"

	// EL PRIMER NIVEL DEL ACTO 1 (el Rogue Encampment): el mismo .ds1 de DC-002.37.
	nivelEnMPQ = "data\\global\\tiles\\ACT1\\TOWN\\townN1.ds1"

	// La paleta del ACTO (mapa) y la de las UNIDADES (personajes): 768 bytes cada una, B,G,R.
	palEnMPQ       = "data/global/palette/ACT1/pal.dat"
	palUnidadesMPQ = "data/global/palette/Units/pal.dat"

	// El COF del personaje: la amazona parada y desarmada (AM + NU + HTH).
	cofEnMPQ = "data\\global\\CHARS\\AM\\COF\\amnuhth.cof"

	// La ventana isometrica y la celda que queda en el centro (las mismas de DC-002.37, para que los
	// numeros del nivel sean comparables). OJO (DC-002.50, medido): se probo bajarla a 480 para ganarle RAM al
	// video y NO sirve -- el monton de C queda igual (~71 KB) porque el monton de Go crece y ocupa lo que se
	// libera. Se deja en 512, la misma de DC-002.37/238/249.
	ladoVentana      = 512
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

	// ============================== LA BESTIA (DC-002.50) ==============================
	//
	// EL BICHO: el Returned del acto 1 (las criptas), su carpeta del MPQ es SK y su clase de monstats es
	// "Returned". Se eligio MIDIENDO (ver monstruo.go): el fallen del Blood Moor nace con 1 HP con las tablas
	// del motor (muere del primer golpe, no llega a jugar la animacion de "recibiendo") y el Brute no entra en
	// la RAM de la consola (su animacion de morir son 393 KB y la arena se queda sin memoria).
	//
	// EL RITMO: los ms/cuadro NO son iguales para los cuatro modos y no salen del COF, salen del animdata.d2.
	// Medidos con la sonda del host (console/dc/sonda-monstruo-250):
	//     NU 8 cuadros a 12,50 cuadros/s = 80 ms/cuadro (ciclo 640 ms)
	//     GH 4 cuadros a 17,97 cuadros/s = 56 ms/cuadro (ciclo 224 ms)
	//     DT 19 cuadros a 20,31 cuadros/s = 49 ms/cuadro (ciclo 931 ms)
	//     DD 1 cuadro a 14,84 cuadros/s = 67 ms/cuadro
	// No se carga el animdata en la consola (son 411 KB y 2.556 registros con mapa: no entra): se usan las
	// constantes medidas, igual que msQuieto y msCaminando en DC-002.49.
	celdaBestiaX = centroX + 1
	celdaBestiaY = centroY + 1

	// la direccion del bicho que se dibuja (el COF trae 8; la 0 es la que mira al sur-oeste, la misma que el
	// host usa para sus numeros de referencia).
	dirBestia = 0

	msBestiaQuieto   = 80 // NU (medido)
	msBestiaGolpeada = 56 // GH (medido)
	msBestiaMuriendo = 49 // DT (medido)
	msBestiaMuerta   = 67 // DD (medido)

	// el ataque del heroe: AMA1HTH, 13 cuadros a 25,00 cuadros/s = 40 ms/cuadro (ciclo 520 ms), medido con la
	// sonda del host. El cuadro del golpe es la mitad: 7 de 13.
	msGolpeHeroe = 40

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

// bestia: ENCIENDE el monstruo y el golpe (DC-002.50). Arranca APAGADO a proposito: con esto apagado el
// binario es el de DC-002.49 (el personaje animado y nada mas), que es lo que compara su corpus. El script
// `scripts/console/build-monstruo-250.sh` lo enciende con un sed y recompila, igual que `animar`.
var bestia = false

// golpeSordo: EL MUTANTE del criterio 4. El boton de golpe se LEE y se INFORMA (la marca del boton sale
// igual), pero NO arranca el ataque: con las mismas pulsaciones, la vida del bicho no puede cambiar.
var golpeSordo = false

// golpeAuto: INSTRUMENTO de medicion (DC-002.50). El emulador corre sin ventana y sin teclado, asi que
// el boton Y nunca se aprieta y la cadena golpe -> dano -> vida no se puede medir. Con esto encendido
// el POC simula el FLANCO del boton (80 ms apretado cada 1,6 s) y todo lo demas del camino es el real:
// el mismo combate(), el mismo ciclo A1, la misma regla del dano a la mitad. NO toca el dano ni la vida.
// Se enciende con GOLPEAUTO=1 en build-monstruo-250.sh. Para el mutante se combina con golpeSordo=1.
var golpeAuto = false

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

	// ---- la bestia y el golpe (DC-002.50)
	mon             *monstruo // el bicho: su vida, sus ciclos y su caja en pantalla
	golpeando       bool      // el heroe esta en medio del ataque (ciclo A1)
	golpeDado       bool      // el dano de ESTE ataque ya se aplico (cae a la mitad de la animacion)
	msGolpe         int64     // cuando empezo el ataque
	botonGolpeAntes bool      // el flanco del boton (una pulsacion = un ataque)
	ataques         int       // cuantos ataques empezo el heroe
}

// rect: una caja de la pantalla (x0,y0) .. (x1,y1) sin incluir el borde derecho/inferior.
type rect struct{ x0, y0, x1, y1 int }

// cuadro: un cuadro del ciclo del personaje, ya compuesto (indices de 8 bits, el 0 transparente).
type cuadro struct {
	Ancho, Alto     int
	Izq, Arriba     int
	Pixeles         []byte
	PixelesNoVacios int
	CRC             uint32
}

// ciclo: la animacion de UN modo (TN quieto, TW caminando), cuadro por cuadro.
type ciclo struct {
	Modo        string
	MsPorCuadro int
	Cuadros     []cuadro
	Imagen      *ebiten.Image // la imagen del cuadro que se esta dibujando (se reemplaza al cambiar)
	RGBA        []byte        // el buffer de esa imagen, reusado cuadro a cuadro (no se asigna por cuadro)
	UnTiro      bool          // DC-002.50: el modo NO da vueltas (GH, DT): al terminarlo, el modo cambia
}

// monstruo: el bicho en el mapa y su combate. Los ciclos son los cuatro modos del COF que se dibujan (NU
// parado, GH golpeado, DT muriendo, DD muerto) y la vida sale de monstruo.go (las tablas del motor).
type monstruo struct {
	Vida    *Vida
	Combate *Combate
	Ciclos  []*ciclo
	Ciclo   *ciclo
	Estado  string // el modo que se esta dibujando (NU, GH, DT, DD)
	Cuadro  int    // el cuadro del ciclo que esta puesto
	Pintado int    // el cuadro que quedo en pantalla
	MsModo  int64  // cuando empezo el modo
	Cambios int    // cuantos cuadros se dibujaron (para la evidencia)

	Caja    rect // la caja del bicho en la pantalla
	HayCaja bool
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
	if animar && bestia && j.mon != nil {
		j.combate()        // el boton del golpe, el dano y el modo del bicho
		j.animarMonstruo() // el cuadro del bicho, por el reloj
	}

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

// dibujarAnimado: el mapa entero la primera vez y, cuadro a cuadro, solo las cajas del bicho y del personaje.
func (j *juego) dibujarAnimado(pantalla *ebiten.Image) {
	if j.ciclo == nil || j.ciclo.Imagen == nil || len(j.ciclo.Cuadros) == 0 {
		return
	}

	hayBestia := j.mon != nil && j.mon.Ciclo != nil

	if !j.pintado {
		var op ebiten.DrawImageOptions
		op.GeoM.Translate(mapaX, mapaY)
		pantalla.DrawImage(j.mapa, &op)

		j.pintado = true
	}

	// si no cambio ni el cuadro ni la posicion, la pantalla ya esta bien: no se repinta
	if j.hayCaja && j.cuadroPintado == j.cuadroDibujado && !j.movido &&
		(!hayBestia || (j.mon.HayCaja && j.mon.Pintado == j.mon.Cuadro)) {
		return
	}

	cu := &j.ciclo.Cuadros[j.cuadroDibujado]

	cx, cy := j.est.Celda()
	ax, ay := PixelDeLaCelda(cx, cy, origenX, origenY)
	px, py := mapaX+ax+cu.Izq, mapaY+ay+cu.Arriba

	// las cajas que tapaban al bicho y al personaje vuelven a ser mapa (es lo que tapaban)
	if j.hayCaja {
		j.devolverAlMapa(pantalla, j.caja)
	}

	if hayBestia && j.mon.HayCaja {
		j.devolverAlMapa(pantalla, j.mon.Caja)
	}

	// el bicho primero (queda DETRAS del personaje) y el personaje encima
	if hayBestia {
		j.dibujarBestia(pantalla)
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

	// DC-002.50: mientras dura el golpe, el personaje dibuja el ciclo del ATAQUE (A1, 13 cuadros a 40 ms).
	if j.golpeando {
		deseado = modoGolpePersonaje
	}

	c := j.buscarCiclo(deseado)

	// el modo pedido puede no estar compuesto (DC-002.50: el binario del golpe no compone TW por RAM): en ese
	// caso se dibuja el quieto, que SIEMPRE esta, en vez de dejar al personaje congelado en el ultimo cuadro.
	if c == nil && deseado != modoQuieto {
		c = j.buscarCiclo(modoQuieto)
	}

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

// ============================== EL GOLPE Y EL BICHO (DC-002.50) ==============================

// combate: el boton del golpe, el dano y el modo del bicho.
//
// EL BOTON ES EL Y DEL CONTROL DE LA DREAMCAST (control.go: contY -> KeyY). Se pregunta el FLANCO: una
// pulsacion es un ataque; dejando el boton apretado no se encadenan golpes, igual que en el juego.
//
// EL DANO CAE A LA MITAD DE LA ANIMACION DEL HEROE (CuadroDelGolpe): es la regla del motor
// (d2mapentity/monster.go, checkEvents: currentFrame/frameCount >= 0,5).
func (j *juego) combate() {
	ahora := int64(d2cMs())
	apretado := ebiten.IsKeyPressed(ebiten.KeyY)

	// DC-002.50: el instrumento. Sin teclado (el emulador corre sin ventana) el boton no se aprieta
	// nunca, asi que se simula el flanco: 80 ms apretado cada 1,6 s. El resto del camino es el real.
	if golpeAuto {
		apretado = apretado || (ahora%1600) < 80
	}

	// la marca del control CRUDO: los bits del maple tal cual (CONT_Y = 32). Es el instrumento para saber que
	// boton del control esta apretado y para probar el mapeo del emulador sin adivinar.
	if j.cuadros%cuadrosAMedir == 0 {
		println("HITO250: CONTROL botones=", int(ebiten.BotonesParaLaEvidencia()), "yApretada=", apretado,
			"golpeando=", j.golpeando, "vida=", j.mon.Vida.HP, "cuadroDelLoop=", j.cuadros)
	}

	if apretado && !j.botonGolpeAntes {
		println("HITO250: BOTON boton=Y botones=", int(ebiten.BotonesParaLaEvidencia()), "golpeSordo=", golpeSordo,
			"vida=", j.mon.Vida.HP, "muerto=", j.mon.Combate.Muerto, "cuadroDelLoop=", j.cuadros)

		if !golpeSordo && !j.golpeando && !j.mon.Combate.Muerto {
			j.golpeando = true
			j.golpeDado = false
			j.msGolpe = ahora
			j.ataques++

			println("HITO250: GOLPE-EMPIEZA ataque=", j.ataques, "vidaAntes=", j.mon.Vida.HP,
				"cuadroDelLoop=", j.cuadros)
		}
	}

	j.botonGolpeAntes = apretado

	if !j.golpeando {
		return
	}

	golpe := j.buscarCiclo(modoGolpePersonaje)
	if golpe == nil || len(golpe.Cuadros) == 0 {
		j.golpeando = false

		return
	}

	f := int((ahora - j.msGolpe) / int64(golpe.MsPorCuadro))

	// el dano, una sola vez por ataque
	if !j.golpeDado && f >= CuadroDelGolpe(len(golpe.Cuadros)) {
		j.golpeDado = true

		dano, antes, despues, muerto := j.mon.Combate.GolpeDelHeroe()

		println("HITO250: GOLPE ataque=", j.ataques, "cuadroDelHeroe=", f, "de", len(golpe.Cuadros),
			"dano=", dano, "vidaAntes=", antes, "vidaDespues=", despues, "muerto=", muerto,
			"danoAcumulado=", j.mon.Combate.Dano, "cuadroDelLoop=", j.cuadros)

		if muerto {
			// EL BICHO MUERE: pasa al cadaver (DD). El DT no esta compuesto en este binario (ver el comentario
			// de los modos del bicho en main): la vida queda en 0 y el dibujo pasa al cadaver.
			j.cambiarModoBestia(modoBestiaMuerta, ahora)

			println("HITO250: MUERTE vida=", j.mon.Vida.HP, "golpes=", j.mon.Combate.Golpes,
				"dano=", j.mon.Combate.Dano, "modo=", j.mon.Estado, "cuadroDelLoop=", j.cuadros)
		} else {
			j.cambiarModoBestia(modoBestiaGolpeada, ahora)
		}
	}

	// el ataque termina cuando la animacion del heroe dio la vuelta
	if f >= len(golpe.Cuadros) {
		j.golpeando = false

		println("HITO250: GOLPE-FIN ataque=", j.ataques, "vida=", j.mon.Vida.HP, "cuadroDelLoop=", j.cuadros)
	}
}

// animarMonstruo: el cuadro del bicho por el RELOJ de la consola y los cambios de modo que hace el motor.
//
// La regla de los modos es la de `d2mapentity/monster.go` (checkEvents): GH y DT son de UN SOLO TIRO -- cuando
// la animacion se jugo entera, GH vuelve a NU y DT pasa a DD (el cadaver, que se queda). NU y DD dan vueltas.
func (j *juego) animarMonstruo() {
	m := j.mon
	if m.Ciclo == nil || len(m.Ciclo.Cuadros) == 0 {
		return
	}

	// a lo sumo dos transiciones en un cuadro (GH -> NU, DT -> DD), para no quedarse dando vueltas
	for i := 0; i < 3; i++ {
		ahora := int64(d2cMs())
		ms := ahora - m.MsModo
		if ms < 0 {
			ms = 0
		}

		f := int(ms / int64(m.Ciclo.MsPorCuadro))

		if !m.Ciclo.UnTiro {
			f %= len(m.Ciclo.Cuadros)
		} else if f >= len(m.Ciclo.Cuadros) {
			// el modo de un tiro se termino: cual sigue (checkEvents del motor)
			switch m.Estado {
			case modoBestiaGolpeada:
				j.cambiarModoBestia(modoBestiaQuieto, ahora)
			case modoBestiaMuriendo:
				j.cambiarModoBestia(modoBestiaMuerta, ahora)

				println("HITO250: MUERTE vida=", m.Vida.HP, "golpes=", m.Combate.Golpes, "dano=", m.Combate.Dano,
					"modo=", m.Estado, "cuadroDelLoop=", j.cuadros)
			default:
				return
			}

			continue
		}

		if f == m.Cuadro && m.Pintado == f {
			return
		}

		m.Cuadro = f
		j.ponerCuadroBestia(m.Ciclo, f)

		return
	}
}

// cambiarModoBestia: el bicho cambia de animacion (NU <-> GH, DT, DD) y el reloj del modo arranca de cero.
func (j *juego) cambiarModoBestia(modo string, ahora int64) {
	for _, c := range j.mon.Ciclos {
		if c.Modo != modo {
			continue
		}

		j.mon.Ciclo = c
		j.mon.Estado = modo
		j.mon.MsModo = ahora
		j.mon.Cuadro = 0
		j.mon.Pintado = -1 // fuerza el repintado en el primer cuadro del modo

		println("HITO250: MODO-BESTIA modo=", modo, "cuadros=", len(c.Cuadros), "msPorCuadro=", c.MsPorCuadro,
			"cicloMs=", len(c.Cuadros)*c.MsPorCuadro, "cuadroDelLoop=", j.cuadros)

		return
	}

	println("HITO250: MODO-BESTIA SIN-CICLO modo=", modo)
}

// ponerCuadroBestia: el cuadro del bicho a su imagen (indices -> RGBA, el 0 transparente) y la marca con los
// numeros que el corpus compara contra el host (modo, cuadro, caja, pixeles y crc32 del compuesto).
func (j *juego) ponerCuadroBestia(c *ciclo, f int) {
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

	j.mon.Cambios++

	println("HITO250: ANIM-BESTIA modo=", c.Modo, "cuadro=", f, "de", len(c.Cuadros),
		"caja=", cu.Ancho, cu.Alto, "esquina=", cu.Izq, cu.Arriba,
		"pixeles=", cu.PixelesNoVacios, "crc32=", cu.CRC,
		"msDelModo=", int64(d2cMs())-j.mon.MsModo, "cambios=", j.mon.Cambios,
		"vida=", j.mon.Vida.HP, "cuadroDelLoop=", j.cuadros)
}

// dibujarBestia: el bicho en la pantalla. La posicion sale de la MISMA proyeccion del piso que el personaje
// (PixelDeLaCelda) con la celda donde esta parado, y la esquina del cuadro la da el COF.
func (j *juego) dibujarBestia(pantalla *ebiten.Image) {
	m := j.mon
	if m.Ciclo == nil || m.Ciclo.Imagen == nil || len(m.Ciclo.Cuadros) == 0 {
		return
	}

	cu := &m.Ciclo.Cuadros[m.Cuadro]

	ax, ay := PixelDeLaCelda(float64(celdaBestiaX), float64(celdaBestiaY), origenX, origenY)
	px, py := mapaX+ax+cu.Izq, mapaY+ay+cu.Arriba

	var op ebiten.DrawImageOptions
	op.GeoM.Translate(float64(px), float64(py))
	pantalla.DrawImage(m.Ciclo.Imagen, &op)

	m.Caja = rect{px, py, px + cu.Ancho, py + cu.Alto}
	m.HayCaja = true
	m.Pintado = m.Cuadro
}

// modoDelHeroe: un modo de la animacion del heroe con su ritmo. TIENE NOMBRE A PROPOSITO (medido el
// 2026-10-24): con un struct ANONIMO, gccgo para SH-4 miscompila las operaciones sobre la rebanada (el
// `append` y el recorte tumbaban el arranque del PVR con una excepcion del SH4). Lo vigila
// `scripts/console/checar-codegen-252.py`.
type modoDelHeroe struct {
	modo string
	ms   int
}

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

	// OJO (DC-002.50): NO hay `defer mpq.Close()`. El MPQ se cierra EXPLICITAMENTE cuando ya no hace falta
	// (despues del bicho), porque en la consola la RAM es una sola y el indice del d2data tiene que volver
	// antes de componer los ciclos del personaje. Cerrarlo dos veces seria un error del disco.

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

	memoria("nivel")

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

	// OJO (DC-002.50, medido): se probo pintar el mapa DIRECTO en la imagen con SetRGBA (para no tener el
	// temporal de 1 MB) y con eso el POC de DC-002.49 -- bestia=0, que no toca nada del bicho -- se cae en el
	// PVR con `Assertion "__builtin_popcount(src->txr.width) == 1" failed ... Invalid texture U size`. Con
	// ReplacePixels no. Asi que el mapa se pinta como en DC-002.38/DC-002.49.
	for i, idx := range n.Canvas {
		r, g, b := ColorDelIndice(palActo, idx)
		rgba[i*4+0], rgba[i*4+1], rgba[i*4+2], rgba[i*4+3] = r, g, b, 255
	}

	mapa.ReplacePixels(rgba)

	// EL RGBA DE 1 MB SE SUELTA ACA, Y ESTA LINEA ES LA QUE ARREGLA EL ARRANQUE (medido 2026-10-24).
	// gccgo NO hace analisis de vivos fino: un local del tamano de un MB sigue ocupando el monton de C hasta el
	// final de la funcion si no se pone en nil a mano. DC-002.49 (bbf3769a) la tenia y el video arrancaba;
	// DC-002.50 (68ad6e88) la borro al reescribir este bloque, y con eso el POC -- INCLUSO con bestia=0, el que
	// no toca nada del bicho -- se reinicia dentro de `IniciarPantalla`, justo despues de "pvr: enabling vertical
	// scaling". Se pago cara: ocho intentos culpando a la RAM del bicho (la bestia usa MENOS RAM que el camino
	// base: 141.128 B libres contra 142.024). El control que lo cazo fue compilar el main.go de bbf3769a con el
	// toolchain de hoy: ese dibuja, el de la rama no.
	rgba = nil

	// EL BUFFER RGBA SON 1 MB (512x512x4) Y EL LIENZO DEL NIVEL 262 KB (512x512): por tamano el runtime los manda
	// al monton de C, y de ahi no los recupera nadie. Los pixeles ya estan subidos a la textura con ReplacePixels
	// y el lienzo no se toca mas.
	// OJO (DC-002.50, medido): devolverlos A MANO con d2cFreeExternal deja el POC sin arrancar el video (la
	// consola se reinicia en IniciarPantalla, y con bestia=0 tambien: no era el bicho). Se sueltan con el
	// recolector, como en DC-002.38.
	n.Canvas = nil
	n.Scratch = nil

	runtime.GC()

	memoria("mapa-en-imagen")

	// DC-002.56 / DC-002.57: esta marca (y las dos de mas abajo, PERSONAJE-IMAGEN y MPQ-CERRADOS) corre con el
	// PVR APAGADO: la marca del encendido es "HITO250: PVR antes-de-pvr_init_defaults", mucho mas abajo. Por eso
	// imprime `vram_libre= sin-init` y NO llama al allocator: con el PVR apagado el numero era siempre 0 y un
	// instrumento que imprime un 0 fijo no informa.
	// MEDIDO (console/dc/evidencia/dc-002.56-vram-preinit.txt): pvr_mem_available() (KOS
	// kernel/arch/dreamcast/hardware/pvr/pvr_mem.c:176) corta con `if(!pvr_mem_base) return 0;`, asi que el 0 no
	// ensuciaba nada: no era la causa del bicho de DC-002.53.
	// La limpieza se PROBO en DC-002.56 y no se pudo aplicar: movia 1ST_READ.BIN 64 bytes y BESTIA=0 dejaba de
	// dibujar. Ese sintoma NO era del layout: era el .gdi, que ponia la pista de audio en FAD 600, dentro del
	// .text, y dejaba un sector en ceros (DC-002.61, console/dc/evidencia/dc-002.61-audio-gdi.txt). Con el GDI
	// arreglado (mkgdi.py, audio en LBA 30000) aterriza en DC-002.57, con BESTIA=0 ANDA en las dos variantes.
	// El control que la vigila es scripts/console/checar-vram-preinit-253.sh.
	println("HITO238: MAPA-IMAGEN bytes=", n.Ancho*n.Alto*4, "vram_libre= sin-init")

	// OJO (DC-002.50, medido): aca se probo arrancar el video ANTES de componer los ciclos, para que el PVR
	// encontrara el monton libre. NO sirve: la consola se queda en "pvr: enabling vertical scaling" (con 800.216 B
	// libres), asi que el video arranca al final, como en DC-002.38/DC-002.49.

	// ============================== 3. el personaje ==============================
	tCoF := int64(d2cMs())

	mc, err := d2mpq.FromFile(rutaMPQChars)
	if err != nil {
		println("HITO238: MPQ-CHARS-ABRIR FALLO:", err.Error())
		return
	}

	// lo mismo que el d2data: se cierra EXPLICITAMENTE cuando los tres ciclos del heroe ya estan compuestos.

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

	println("HITO238: PERSONAJE-IMAGEN bytes=", per.Ancho*per.Alto*4, "vram_libre= sin-init")

	// ============================== 3b. LA BESTIA Y EL GOLPE (DC-002.50) ==============================
	//
	// Con `bestia` encendido se suma al mapa el bicho del acto 1 y el golpe a mano limpia. NINGUN numero de
	// los que se usan aca es elegido: la vida sale de las tablas del MPQ por el camino de siembra del motor y
	// el dano del dado del motor (monstruo.go, el archivo que comparten la consola y el verificador del host).
	//
	// ACA NO SE LEE NADA (DC-002.50, el 9no camino): la vida del bicho pide leer monstats.txt (238.019 B) del
	// d2data.mpq y parsearla, y ESA lectura -- y nada mas -- era lo que quedaba de distinto contra el POC de
	// DC-002.49 al llegar a `IniciarPantalla`. Con esa lectura hecha el monton de C queda en 70.168..70.976 B
	// libres y el PVR se cuelga en "pvr: enabling vertical scaling"; el mismo binario sin ella arranca y
	// dibuja. El monton de Go no devuelve lo que crecio (medido: liberar 1 MB del nivel no le deja plata al
	// video, el monton se la queda), asi que la unica forma de que el arranque quede IDENTICO al que ya anda
	// es no tocar la RAM antes del video: todo lo del bicho se prepara en 3d, con el video ya andando.
	var mon *monstruo

	// la vida y el combate se calculan en 3d (con el video andando) y los usa el mismo bloque para armar el
	// bicho con sus ciclos: por eso viven aca afuera
	var vida *Vida

	var combate *Combate

	var tBestia int64

	if bestia && !animar {
		println("HITO250: LA BESTIA NECESITA animar=true (el golpe dibuja los ciclos)")

		return
	}

	// ============================== 3c. LOS CICLOS DE LA ANIMACION (DC-002.49) ==============================
	//
	// Con `animar` encendido se componen los DOS ciclos del pueblo: TN (parado, 16 cuadros) y TW (caminando,
	// 8 cuadros), cada uno con su COF y sus graficos del d2char.mpq, cuadro por cuadro, con el compositor
	// compartido. El estado (quieto / caminando) elige el ciclo, y el reloj de la consola elige el cuadro.
	var ciclos []*ciclo

	if animar {
		tCiclos := int64(d2cMs())

		// LOS MODOS DEL HEROE QUE SE COMPONEN. Con la bestia encendida son DOS y no tres (TN quieto y A1
		// ataque): medido, con TN + TW + A1 + los cuatro del bicho el monton de la consola no alcanza y el POC
		// se queda sin memoria al empezar TW (TN y el DT ya estaban compuestos y no entro uno mas). El caminar
		// NO se rompe: la posicion la sigue moviendo el motor, lo unico que se dibuja es TN.
		// DEFECTO DE CODEGEN, MEDIDO EL 2026-10-24 (evidencia de DC-002.50, seccion 12): con una rebanada de
		// struct ANONIMO, gccgo para SH-4 miscompila el `append` y el recorte, y la consola se REINICIA dentro
		// de `pvr_init_defaults`/`pvr_mem_malloc` como si le faltara RAM -- ocho intentos de esta tarjeta se
		// fueron en esa hipotesis. Por eso el tipo tiene NOMBRE (`modoDelHeroe`) y la lista se arma con un
		// literal por rama, sin append ni recorte. Lo vigila `scripts/console/checar-codegen-252.py`.
		var modosDelHeroe []modoDelHeroe

		if bestia {
			// con la bestia encendida NO se compone TW: TN + A1 + los tres modos del bicho no entran juntos
			modosDelHeroe = []modoDelHeroe{{modoQuieto, msQuieto}}
		} else {
			modosDelHeroe = []modoDelHeroe{{modoQuieto, msQuieto}, {modoCaminando, msCaminando}}
		}

		for _, m := range modosDelHeroe {
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

		// EL CICLO DEL GOLPE (A1) NO SE COMPONE ACA: va en 3d, con el video ya andando (medido: es el que deja el
		// monton de C en 70 KB y con eso `IniciarPantalla` no arranca).

		println("HITO249: CICLOS cargados", len(ciclos), "ms", int64(d2cMs())-tCiclos, "mutante", cicloSordo)

		memoria("ciclos-del-personaje")
	}

	// el d2char.mpq SIGUE ABIERTO: el ciclo del golpe (A1) se compone en 3d, con el video ya andando.

	// ============================== 4. el personaje y el control ==============================
	// DC-002.56 / DC-002.57: tercera marca pre-video (las tres son las de esta tarjeta). Igual que MAPA-IMAGEN:
	// con el PVR apagado no se le pregunta nada al allocator y se imprime `sin-init`. Ver la nota larga de
	// MAPA-IMAGEN (y DC-002.61: la fragilidad que impedia aplicarlo era el .gdi, no el layout).
	println("HITO250: MPQ-CERRADOS vram_libre= sin-init")

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

	// DC-002.50: el bicho entra al juego DESPUES del video (ver 3b-bis): aca todavia no existe.

	if err := ebiten.IniciarPantalla(ancho, alto); err != nil {
		println("HITO238: no pude arrancar el video:", err.Error())
	}

	// ============================== 3d. LOS CICLOS DEL BICHO, CON EL VIDEO YA ANDANDO (DC-002.50) ==========
	//
	// POR QUE ACA Y NO EN 3b (medido en la consola): componer los tres modos del bicho son ~420 KB del monton de
	// C (las capas decodificadas y los cuadros compuestos) y con eso hecho `IniciarPantalla` ya no arranca: el
	// POC se queda en "pvr: enabling vertical scaling" con 70.976 B libres y no vuelve. Con el video ya andando
	// los ciclos se componen igual que cualquier textura en runtime (los pixeles viven en el monton; la textura
	// en la VRAM). El d2data.mpq se cierra DESPUES de componerlos, porque las capas salen de ahi.
	cargarBestia := func() {
		ciclosBestia := make([]*ciclo, 0, 4)

		for _, m := range []struct {
			modo   string
			ms     int
			unTiro bool
		}{
			// LOS TRES MODOS QUE ENTRAN EN LA CONSOLA: parado (da vueltas), golpeado (de un tiro) y cadaver.
			// El DT (muriendo, 8 capas de 19 cuadros = 377 KB de monton) NO se compone aca: con el monton de Go
			// de 1 MB del toolchain no entra junto con los otros tres y los dos del heroe (medido: "fatal error:
			// out of memory"). Se censo con el verificador del host, que no tiene ese limite, y la consola pasa
			// del GH al cadaver.
			{modoBestiaQuieto, msBestiaQuieto, false},
			{modoBestiaGolpeada, msBestiaGolpeada, true},
			{modoBestiaMuerta, msBestiaMuerta, false},
		} {
			runtime.GC()

			crudoCOF, err := mpq.ReadFile(RutaCOFBestia(m.modo, "HTH"))
			if err != nil || len(crudoCOF) == 0 {
				println("HITO250: SIN COF PARA EL MODO DEL BICHO", m.modo)

				continue
			}

			cofBestia, err := d2cof.Unmarshal(crudoCOF)
			if err != nil {
				println("HITO250: COF DEL BICHO", m.modo, "PARSE-FALLO:", err.Error())

				continue
			}

			println("HITO250: COF DEL BICHO", m.modo, "bytes=", len(crudoCOF), "crc32=", crc32.ChecksumIEEE(crudoCOF),
				"capas=", cofBestia.NumberOfLayers, "framesPorDir=", cofBestia.FramesPerDirection,
				"dirs=", cofBestia.NumberOfDirections)

			ciclosBestia = append(ciclosBestia, cargarCicloCon(mpq, cofBestia, dirBestia, m.modo, m.ms,
				palUni, rutaDeLaCapaDelBicho, "HITO250", m.unTiro))

			cofBestia = nil
			crudoCOF = nil

			memoria("modo-" + m.modo)

			runtime.GC()
		}

		mon = &monstruo{Vida: vida, Combate: combate, Ciclos: ciclosBestia, Pintado: -1}

		for _, c := range ciclosBestia {
			if c.Modo == modoBestiaQuieto {
				mon.Ciclo = c
				mon.Estado = modoBestiaQuieto
			}
		}

		mon.MsModo = int64(d2cMs())

		println("HITO250: BESTIA lista modos=", len(ciclosBestia), "estado=", mon.Estado,
			"celda=", celdaBestiaX, celdaBestiaY, "ms=", int64(d2cMs())-tBestia, "mutante=", golpeSordo)

		// EL D2DATA.MPQ YA NO HACE FALTA: el mapa, la vida y los cuatro modos del bicho estan en RAM. Se
		// cierra ACA, antes de los ciclos del personaje, porque su indice (la tabla de bloques y la de hash de
		// los ~40.000 archivos) es el objeto mas grande que queda vivo y es lo que le deja la RAM a los ciclos
		// del d2char (medido: cerrarlo despues no alcanzaba y el POC se quedaba sin memoria).
		mpq.Close()

		memoria("d2data-cerrado")
	}

	// ============================== 3b-bis. LA VIDA DEL BICHO, CON EL VIDEO YA ANDANDO (DC-002.50) ==========
	//
	// La vida y el combate se calculan ACA y no antes del video: leer monstats.txt (238.019 B) y parsearlo es
	// la unica diferencia que tenia el arranque con la del POC de DC-002.49, y es lo que dejaba al PVR con
	// 70 KB (se cuelga en "pvr: enabling vertical scaling"). El monton de Go no devuelve lo que crece, asi
	// que la lectura se hace con el video andando, igual que los ciclos.
	if bestia {
		tBestia = int64(d2cMs())

		prepararVida := func() bool {
			monstats, err := mpq.ReadFile(excelMonstats)
			if err != nil {
				println("HITO250: MONSTATS FALLO:", err.Error())

				return false
			}

			monlvl := leerDeMPQ(rutaMPQPatch, excelMonLvl)
			if monlvl == nil {
				println("HITO250: MONLVL FALLO:", excelMonLvl)

				return false
			}

			v, err := VidaDelBicho(monstats, monlvl, claseBestia, semillaCombate)
			if err != nil {
				println("HITO250: VIDA FALLO:", err.Error())

				return false
			}

			vida = v

			println("HITO250: MONSTATS bytes=", len(monstats), "crc32=", crc32.ChecksumIEEE(monstats))
			println("HITO250: MONLVL bytes=", len(monlvl), "crc32=", crc32.ChecksumIEEE(monlvl))
			println("HITO250: VIDA clase=", vida.Clase, "nivel=", vida.Nivel, "min=", vida.Min, "max=", vida.Max,
				"HP=", vida.HP, "AC=", vida.AC, "A1=", vida.DanoMin, "..", vida.DanoMax, "semilla=", semillaCombate)

			// OJO (DC-002.50, medido): aca estaba la devolucion a mano de la tabla de monstats (238 KB) con
			// d2cFreeExternal, copiando lo que hacen los DT1 del nivel en DC-002.37. NO VA: con esa devolucion
			// el POC se cae con "Unhandled exception ... Encountered Illegal instruction" adentro de
			// `pvr_init_defaults`. Las dos tablas las suelta el recolector.
			monstats, monlvl = nil, nil

			runtime.GC()

			combate = NuevoCombate(vida, semillaCombate)

			return true
		}

		if !prepararVida() {
			println("HITO250: SIN VIDA DEL BICHO, EL BICHO NO ENTRA (el mapa y el personaje siguen)")
		} else {
			// EL ORDEN ES POR RAM, no por gusto: `cargarBestia` compone los tres modos del bicho y CIERRA el
			// d2data.mpq (su indice de ~40.000 archivos es el objeto mas grande que queda vivo); recien despues
			// se compone el A1 del heroe, que sale del d2char.mpq y asi encuentra esa RAM libre.
			cargarBestia()

			// EL CICLO DEL GOLPE DEL HEROE (A1): 13 cuadros a 40 ms.
			if cofA1, nombreA1, bytesA1, ok := cofDelModo(mc, modoGolpePersonaje); ok {
				println("HITO250: COF DEL GOLPE", nombreA1, "bytes=", bytesA1,
					"framesPorDir=", cofA1.FramesPerDirection, "dirs=", cofA1.NumberOfDirections)

				ciclos = append(ciclos, cargarCiclo(mc, cofA1, dirPersonaje, modoGolpePersonaje, msGolpeHeroe, palUni))

				cofA1 = nil

				runtime.GC()
				memoria("ciclo-del-golpe")
			} else {
				println("HITO250: SIN COF PARA EL GOLPE", modoGolpePersonaje)
			}

			// los ciclos y el bicho se cargaron DESPUES de armar el juego: se le pasan ahora (el corte de la
			// lista de ciclos es una copia, asi que hay que volver a asignarla)
			j.ciclos = ciclos
			j.mon = mon
		}
	}

	mc.Close()

	memoria("d2char-cerrado")

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

// rutaDeLaCapaDelBicho: el adaptador para cargarCicloCon (las capas del bicho van por el esquema de los
// monstruos, no por el de los personajes).
func rutaDeLaCapaDelBicho(p *Personaje, i int, modo, ext string) string {
	return p.RutaDeLaCapaBestia(i, modo, ext)
}

// memoria: cuanta RAM queda libre (bytes) al empezar una etapa. En la consola es UN solo monton y el numero
// es el mismo que imprime el runtime cuando se queda sin memoria: es el instrumento de "esto entra o no
// entra" (DC-002.50: el Brute no entraba por 150 KB).
func memoria(etapa string) {
	println("HITO250: MEMORIA etapa=", etapa, "libre=", int64(d2cHeapLibre()))
}

// leerDeMPQ: lee UN archivo de un MPQ que solo se usa para eso (el patch_D2.mpq: se abre, se lee la tabla y se
// cierra, para no dejar 8 MB de indice en la RAM de la consola).
func leerDeMPQ(rutaMPQ, nombre string) []byte {
	m, err := d2mpq.FromFile(rutaMPQ)
	if err != nil {
		println("HITO250: MPQ-ABRIR FALLO:", rutaMPQ, err.Error())

		return nil
	}
	defer m.Close()

	crudo, err := m.ReadFile(nombre)
	if err != nil {
		println("HITO250: MPQ-LEER FALLO:", nombre, err.Error())

		return nil
	}

	return crudo
}

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
	return cargarCicloCon(mc, cof, dir, modo, msPorCuadro, pal, nil, "HITO249", false)
}

// cargarCicloCon: el MISMO compositor para cualquier COF. `rutas` dice donde esta el grafico de cada capa
// (nil = el esquema del personaje) y `hito` es el prefijo de las marcas de la consola serie; `unTiro` marca
// los modos que NO dan vueltas (GH y DT del bicho, DC-002.50).
func cargarCicloCon(mc *d2mpq.MPQ, cof *d2cof.COF, dir int, modo string, msPorCuadro int,
	pal [256][3]uint8, rutas func(*Personaje, int, string, string) string, hito string, unTiro bool) *ciclo {
	n := cof.FramesPerDirection
	per0 := NuevoPersonaje(cof, dir, 0)
	capas := make([]capaCiclo, 0, len(cof.CofLayers))

	for i := range cof.CofLayers {
		tCapa := int64(d2cMs())
		encontrada := false

		for _, ext := range []string{"dcc", "dc6"} {
			nombre := per0.RutaDeLaCapaModo(i, modo, ext)
			if rutas != nil {
				nombre = rutas(per0, i, modo, ext)
			}

			crudo, err := LeerDelMPQ(mc, nombre)
			if err != nil {
				continue
			}

			c := capaCiclo{tipo: per0.Tipos[i], clave: per0.Claves[i]}

			if ext == "dcc" {
				dd, err := d2dcc.LoadDirection(crudo, dir)
				if err != nil {
					println(hito+": CAPA", c.clave, "DEL MODO", modo, "PARSE-FALLO:", err.Error())
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
				println(hito+": CAPA", c.clave, "DEL MODO", modo, "PARSE-FALLO:", err.Error())
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
			if hito != "HITO249" {
				println(hito+": MEMORIA-CAPA modo=", modo, "capa=", per0.Claves[i], "libre=", int64(d2cHeapLibre()))
			}

			println(hito+": CAPA DEL CICLO", modo, per0.Claves[i], "cuadros", len(capas[len(capas)-1].cuadros),
				"caja", capas[len(capas)-1].ancho, capas[len(capas)-1].alto, "ms", int64(d2cMs())-tCapa)

			// EL RECOLECTOR DESPUES DE CADA CAPA (esto es de DC-002.38 y no es adorno): d2dcc.Load decodifica
			// las 16 DIRECCIONES de la capa, y de ahi solo se guardan los cuadros de una. Los otros 240
			// cuadros son basura; si no se recoge entre capa y capa, el recolector de libgodc se pone a
			// correr sobre un monton lleno de basura y la carga se vuelve 40 veces mas lenta (medido en esta
			// tarjeta: LG sin recoleccion 42.608 ms contra 652 ms con ella).
			runtime.GC()
		}
	}

	c := &ciclo{Modo: modo, MsPorCuadro: msPorCuadro, UnTiro: unTiro}

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

		println(hito+": CUADRO DEL CICLO", modo, f, "de", n, "caja", cu.Ancho, cu.Alto,
			"esquina", cu.Izq, cu.Arriba, "pixeles", cu.PixelesNoVacios, "crc32", cu.CRC)
	}

	// los pixeles de las capas ya estan pegados en los cuadros: se sueltan
	capas = nil
	runtime.GC()

	if hito != "HITO249" {
		println(hito+": MEMORIA-COMPUESTO modo=", modo, "libre=", int64(d2cHeapLibre()))
	}

	if len(c.Cuadros) == 0 || c.Cuadros[0].Ancho == 0 {
		println(hito+": EL CICLO", modo, "SALIO VACIO")

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

package d2gamescreen

// DC-002.159 -- EL JUGADOR: LA AMAZONA DEL MOTOR CAMINANDO POR EL PUEBLO CON LA CRUCETA.
//
// QUE ES. El Gate 0 de DC-002.157 pintaba el pueblo, pero sin nadie adentro. Esto le pone EL HEROE DEL
// MOTOR: el mismo `d2mapentity.Player` que dibuja el juego de la PC, con su `d2asset.Composite` (el COF
// del personaje y sus capas `.dcc` del MPQ), su animacion de caminar y su direccion de 8 posiciones.
//
// DE DONDE SALE CADA COSA (nada de esto es un numero elegido aca):
//
//   - EL ENTE: `d2mapentity.NuevoJugadorDeConsola` (DC-002.76), que es `NewPlayer` del motor sin la hoja de
//     heroe, las habilidades ni el inventario (esos son los MB que el Gate 0 no tiene). El compuesto se
//     carga del MPQ por `am.LoadComposite`, o sea el COF y los `.dcc` del personaje real.
//   - LAS ESTADISTICAS: `charstats.txt` (la tabla `CharStats` que el arranque carga en el Gate 0) y la
//     formula del motor para la vida/mana/estamina maximas de nivel 1 (`d2statlist.Class.BaseMax`, la misma
//     que usa `CreateHeroStatsState`). No hay numeros escritos a mano.
//   - EL PUNTO DE ARRANQUE: `mapEngine.GetStartPosition()`, que busca el objeto de entrada del pueblo en el
//     `.ds1` y cae al centro del mapa si el nivel no lo tiene.
//   - LA VELOCIDAD: la del motor para caminar (`d2path`), ya la trae el Player (`SetSpeed`), y el paso lo
//     hace `mapEntity.Step` con el `Advance` del motor.
//   - LA ANIMACION: la elige el propio `Player.GetAnimationMode()` (en el pueblo: TU = parado, TW = caminar)
//     y la direccion de 8 posiciones la calcula `Position.DirectionTo` desde el vector de movimiento.
//
// LA CRUCETA (decidido por Papu, 2026-11-04): 8 direcciones. El heroe ROTA Y AVANZA DE FRENTE, como en el
// Diablo de consola: se aprieta una direccion y el personaje se pone de cara a esa direccion y camina; dos
// flechas a la vez dan las diagonales. Las flechas son las que el maple ya traduce (d2core/d2input/maple:
// cruceta -> KeyUp/KeyDown/KeyLeft/KeyRight), y la conversion de "direccion en la pantalla" a "direccion en
// el mapa" es la inversa de la proyeccion isometrica del viewport (la misma cuenta que `ScreenToWorld`),
// asi que apretar arriba camina hacia arriba de la pantalla aunque en el mapa eso sea noroeste.
//
// POR QUE NO ENTRA `d2game/d2player` (GameControls). Ese paquete es el juego de PC entero: HUD, inventario,
// habilidades, paneles, menus, click-para-caminar, teclado y mouse. El control de esta tarjeta es una cruceta
// de 8 direcciones, que no necesita ninguna de esas cosas, y el paquete pesa 1.248.815 B de imagen entre el
// y `d2hero` (medido en DC-002.156): traerlo para leer una cruceta es justo lo que la tarjeta pide no hacer.
// El heroe es el mismo del motor igual -- el ente, el compuesto y su animacion son los del motor.

import (
	"fmt"
	"math"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2enum"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2interface"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2math/d2vector"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2statlist"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2util"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2asset"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2hero"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2map/d2mapengine"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2map/d2mapentity"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2map/d2maprenderer"
)

const (
	// subtilesPorCelda159: el motor guarda las posiciones en SUBTILES y una celda del DS1 son 5 subtiles.
	subtilesPorCelda159 = 5.0

	// claseDelHeroe159: la amazona (charstats 0), la misma del POC del personaje desde DC-002.38.
	claseDelHeroe159 = d2enum.HeroAmazon

	// nombreDelHeroe159: el que va en la evidencia, no en pantalla (el Gate 0 no dibuja texto).
	nombreDelHeroe159 = "Amazona"

	// direccionDelHeroe159: con la que arranca (0 = sur, la del motor).
	direccionDelHeroe159 = 0

	// celdasPorDelante159: cuantas celdas se pone el objetivo cuando se aprieta una direccion. No es un
	// numero de velocidad: la velocidad la pone el motor. Es cuanto camino se le pide de una vez, para que
	// el `Step` tenga a donde ir mientras la tecla siga apretada (una celda de 5 subtiles se camina en
	// ~0,5 s, asi que 3 celdas dan margen de sobra y el objetivo se renueva cada cuadro igual).
	// EL OBJETIVO DEL PASO ES UN RUMBO, NO UNA META (DC-002.159e). Antes eran 1 celda por delante, y con el cuadro
	// de la consola (~0,75 s) el ente LLEGABA al objetivo en pocos cuadros: al llegar, `atTarget()` daba true y
	// `Player.GetAnimationMode()` devolvia `TN` (quieto) por un cuadro, y al cuadro siguiente `Avanzar` le ponia
	// camino otra vez (`TW`). Cada uno de esos cambios de modo REHACE el compuesto entero
	// (`Composite.createMode`: COF + las capas `.dcc` del modo), o sea ~250.000 B por cambio, y medido en el Gate 0
	// (DC-002.159e) el caminar llegaba a 95 cambios en 345 cuadros: el recolector corriendo 5 veces por cuadro y
	// el vivo pegado a 1,7 MB. Con 8 celdas por delante el objetivo no se alcanza dentro de un tramo del paseo
	// (24 cuadros), el modo se queda en `TW` y el compuesto no se rehace: el ente camina igual, en la misma
	// direccion, porque lo que manda es el rumbo.
	celdasPorDelante159 = 8.0

	// EL RECUADRO DEL BORRADO NO ES UNA CONSTANTE (DC-002.159c). Antes eran 160x160 fijos ("la caja del
	// compuesto de la amazona son 60x76 medidos"): entraba el cuadro de parado, pero caminando las capas de las
	// piernas se dibujan con su propio desplazamiento, el sprite se sale del recuadro y quedaban piernas del
	// cuadro anterior pegadas al mapa. Ahora la caja la da el compuesto cuadro a cuadro
	// (`Player.RecuadroDelCuadro159` -> `Composite.Recuadro159`), que es el unico que sabe que capas se dibujan
	// y con que desplazamiento. El buffer de abajo (`h.memoria`) se pide con esa caja y crece cuando hace falta.
)

// lienzoConMemoria159: lo que la Surface del cuadro tiene que saber hacer para que el mapa se quede
// pintado y el heroe se borre solo. En la consola la implementa el shim del PVR
// (d2core/d2render/ebiten/memoria_consola_159.go); en la PC no existe, y entonces el camino es el de
// siempre: el mapa entero cada cuadro y el heroe dibujado por el renderer dentro de sus pasadas.
type lienzoConMemoria159 interface {
	CopiarRect159(destino []byte, x, y, ancho, alto int) (escritos, xc, yc, anchoC, altoC int)
	PegarRect159(origen []byte, x, y, ancho, alto int) int
}

// HeroeDelGate0: el heroe, su control y la memoria de abajo.
type HeroeDelGate0 struct {
	jugador *d2mapentity.Player
	motor   *d2mapengine.MapEngine
	mapa    *d2maprenderer.MapRenderer
	term    d2interface.Terminal

	// entrada: de donde se lee la cruceta (el maple en la consola). refrescar: el que lee el control y
	// deja el estado del cuadro (en la consola es una llamada al bus maple; en la PC no hay).
	entrada   d2interface.InputService
	refrescar func()

	// compuesto: el compuesto del heroe tal como se armo (es el mismo que dibuja el ente). De aca sale la CAJA del
	// cuadro que se esta dibujando, que es el recuadro del borrado (DC-002.159c): la caja no se puede adivinar
	// desde afuera porque cada capa se dibuja con su propio desplazamiento (ver `Composite.Recuadro159`).
	compuesto *d2asset.Composite

	// lienzo: si la Surface sabe guardar y devolver un recuadro. nil = no se puede (la PC).
	lienzo  lienzoConMemoria159
	memoria []byte
	// DC-002.159c: de que tamano es el buffer de arriba. La caja la da el compuesto y crece con el cuadro
	// (un cuadro de caminar es mas alto que uno de parado), asi que el buffer se pide del tamano pedido y se
	// vuelve a pedir SOLO si el nuevo no entra.
	memoriaAncho int
	memoriaAlto  int
	hayCopia     bool
	copiaX       int
	copiaY       int
	// DC-002.159b: el recuadro que de verdad se guardo (el recorte al lienzo cuando el heroe esta en el borde).
	// Es el que se devuelve: devolver el recuadro entero moveria pixeles que no se copiaron.
	copiaAncho int
	copiaAlto  int

	// DC-002.159c: la ultima caja que dio el compuesto (izq, arriba, ancho, alto) y cuantas veces no la dio.
	// Van en la linea de estado: son la prueba de que el recuadro sale del compuesto y de que cubre el caminar.
	ultimaCaja [4]int
	sinCaja159 int

	// DC-002.159c (instrumentos del borrado): `memoria2` es el buffer gemelo donde se relee el recuadro para
	// comprobar que el lienzo devolvio lo que guardo; `borrados159` cuenta los borrados hechos, `noVolvieron159`
	// los que NO volvieron iguales y `dibujos159` las veces que se dibujo el ente (si crece de a uno por cuadro,
	// nadie mas lo esta dibujando).
	memoria2       []byte
	borrados159    int
	noVolvieron159 int
	dibujos159     int
	// cuantos pixeles cambio el dibujo dentro del recuadro (y en cuantos cuadros cambio alguno)
	pintados159        int
	cuadrosPintados159 int

	// la ultima direccion pedida y lo que se acumula para la evidencia
	direccion  int
	pasos      int
	caminando  bool
	sinControl bool

	// DC-002.159b: el sentido de la cruceta del cuadro anterior y el objetivo que se le puso al ente. Sirven
	// para no mandar un camino nuevo en cada cuadro (ver `Avanzar`).
	ultimoEjeX    float64
	ultimoEjeY    float64
	objetivo      d2vector.Position
	tieneObjetivo bool

	*d2util.Logger
}

// NuevoHeroeDelGate0 arma el heroe del pueblo: el ente, su compuesto y sus estadisticas de nivel 1.
func NuevoHeroeDelGate0(
	am *d2asset.AssetManager,
	motor *d2mapengine.MapEngine,
	mapa *d2maprenderer.MapRenderer,
	term d2interface.Terminal,
	logger *d2util.Logger,
	entrada d2interface.InputService,
	refrescar func(),
	compuesto *d2asset.Composite,
) (*HeroeDelGate0, error) {
	stats, err := estadisticasDeNivelUno(am, claseDelHeroe159)
	if err != nil {
		return nil, err
	}

	// el punto de arranque del pueblo, en celdas -> subtiles (el Player trabaja en subtiles)
	xCelda, yCelda := motor.GetStartPosition()
	x := int(xCelda * subtilesPorCelda159)
	y := int(yCelda * subtilesPorCelda159)

	// DC-002.159: el compuesto viene armado de antes (se arma cuando la arena esta vacia: ver la pantalla del
	// pueblo); aca solo se crea el ente, que es lo que necesita el mapa ya compuesto para saber donde nace.
	//
	// DC-002.159b: EL MODO INICIAL ES EL DEL PUEBLO, no el Neutral. `SetMode` carga las capas `.dcc` del modo
	// (442.601 B medidos por modo) y el heroe del pueblo usa `TN` (parado) y `TW` (caminando): arrancando en
	// Neutral se pagaba un modo que no se usa, con el mapa ya compuesto y el semiespacio al limite. El modo se
	// pide aca, al crear el ente (con el mapa ya soltado y su memoria devuelta): el prime en `OnLoad` se probo
	// y se revirtio, porque el modo se carga igual al primer `SetMode` y no habia con que pagarlo dos veces.
	jugador := d2mapentity.NuevoJugadorDeConsolaConModo(compuesto, "heroe-gate0159", nombreDelHeroe159,
		x, y, direccionDelHeroe159, claseDelHeroe159, stats, d2enum.PlayerAnimationModeTownNeutral)
	if jugador == nil {
		return nil, fmt.Errorf("el heroe del motor no se pudo crear con el compuesto del MPQ")
	}

	// el pueblo: el heroe NO corre (camina), y esta en el pueblo (es lo que elige el modo TU/TW)
	jugador.SetIsInTown(true)

	motor.AddEntity(jugador)

	heroe := &HeroeDelGate0{
		jugador:   jugador,
		motor:     motor,
		mapa:      mapa,
		term:      term,
		entrada:   entrada,
		refrescar: refrescar,
		compuesto: compuesto,
		direccion: direccionDelHeroe159,
		Logger:    logger,
	}

	return heroe, nil
}

// estadisticasDeNivelUno: la vida, el mana y la estamina maximas de un heroe de nivel 1, con la formula
// del motor y la fila de `charstats.txt` de su clase. Los atributos son los iniciales de la clase
// (`InitStr`, `InitDex`, `InitVit`, `InitEne`), o sea un personaje recien creado.
func estadisticasDeNivelUno(am *d2asset.AssetManager, clase d2enum.Hero) (*d2hero.HeroStatsState, error) {
	if am == nil || am.Records == nil || am.Records.Character.Stats == nil {
		return nil, fmt.Errorf("charstats.txt no esta cargado (el heroe necesita su fila de clase)")
	}

	fila, hay := am.Records.Character.Stats[clase]
	if !hay || fila == nil {
		return nil, fmt.Errorf("charstats.txt no tiene la clase %d", int(clase))
	}

	claseDelMotor := d2statlist.Class{
		InitStr: fila.InitStr, InitDex: fila.InitDex, InitVit: fila.InitVit, InitEne: fila.InitEne,
		InitStamina: fila.InitStamina, HpAdd: fila.HpAdd,
		LifePerVit: fila.LifePerVit, ManaPerEne: fila.ManaPerEne, StaminaPerVit: fila.StaminaPerVit,
		LifePerLevel: fila.LifePerLevel, ManaPerLevel: fila.ManaPerLevel,
		StaminaPerLevel: fila.StaminaPerLevel,
		ToHitFactor:     fila.ToHitFactor, BlockFactor: fila.BlockFactor,
	}

	vida, mana, estamina := claseDelMotor.BaseMax(1, fila.InitVit, fila.InitEne)

	return &d2hero.HeroStatsState{
		Level:    1,
		Strength: fila.InitStr, Dexterity: fila.InitDex, Vitality: fila.InitVit, Energy: fila.InitEne,
		Health: vida, MaxHealth: vida, BaseMaxHealth: vida,
		Mana: mana, MaxMana: mana, BaseMaxMana: mana,
		Stamina: float64(estamina), MaxStamina: estamina, BaseMaxStamina: estamina,
	}, nil
}

// ConMemoria: le pasa el lienzo que sabe guardar recuadros (la Surface del cuadro). Si es nil, el heroe se
// dibuja igual pero sin borrado: sirve solo cuando el mapa se repinta entero cada cuadro (la PC).
//
// DC-002.159c: el buffer NO se pide aca. Su tamano sale de la caja del compuesto, que cambia con el cuadro de la
// animacion; se pide en el primer dibujo y crece solo si el cuadro nuevo no entra (ver `asegurarMemoria`).
func (h *HeroeDelGate0) ConMemoria(lienzo lienzoConMemoria159) {
	h.lienzo = lienzo
}

// asegurarMemoria: deja `h.memoria` con lugar para un recuadro de ancho x alto (RGBA de 8 bits por canal). Se
// pide una vez y se vuelve a pedir solo si el que hay no entra: el cuadro de caminar es mas alto que el de parado
// y el de una diagonal cambia otra vez, pero despues de dos o tres cuadros la caja se queda quieta.
func (h *HeroeDelGate0) asegurarMemoria(ancho, alto int) bool {
	if ancho <= 0 || alto <= 0 {
		return false
	}

	if h.memoria != nil && h.memoriaAncho >= ancho && h.memoriaAlto >= alto {
		return true
	}

	h.memoria = make([]byte, ancho*alto*4)
	h.memoria2 = make([]byte, ancho*alto*4)
	h.memoriaAncho, h.memoriaAlto = ancho, alto

	return true
}

// ConMapa: la camara, que se pasa DESPUES de crear el ente (el heroe se arma antes que el renderer porque es
// el punto con mas arena libre; ver el comentario del bloque en la pantalla).
func (h *HeroeDelGate0) ConMapa(m *d2maprenderer.MapRenderer) { h.mapa = m }

// Jugador: el ente, para que la pantalla lo pregunte (posicion, animacion) en la evidencia.
func (h *HeroeDelGate0) Jugador() *d2mapentity.Player { return h.jugador }

// Avanzar: un cuadro del control y del heroe. Se llama ANTES del `Advance` del motor, que es el que mueve
// el ente: aca solo se le PIDE el camino.
func (h *HeroeDelGate0) Avanzar() {
	if h.entrada == nil {
		return
	}

	if h.refrescar != nil {
		h.refrescar()
	}

	ejeX, ejeY := h.ejeDeLaCruceta()

	if ejeX == 0 && ejeY == 0 {
		// cruceta suelta: se para donde esta (el motor le pone el modo TU y el cuadro de quieto)
		if h.caminando {
			h.jugador.StopMoving()
			h.caminando = false
		}

		return
	}

	// DC-002.159b: NO se manda un camino nuevo en cada cuadro. Mientras la cruceta siga en el MISMO sentido y
	// el objetivo no se haya alcanzado, el camino que el ente ya tiene sirve: mandarlo de nuevo cada cuadro
	// tiraba basura a lo loco (medido: el recolector corria cada ~12 cuadros y a los 74 s el POC moria con
	// `fatal error: stack_alloc: out of memory`, porque cada pasada del recolector nuevo se queda sin segmentos
	// de pila). Mandar el camino al cambiar de sentido es ademas lo que hace el motor de verdad: se aprieta una
	// direccion, el ente camina hasta el objetivo, y recien cuando llega se le pide otro.
	if ejeX == h.ultimoEjeX && ejeY == h.ultimoEjeY && h.tieneObjetivo &&
		!h.jugador.Position.EqualsApprox(&h.objetivo.Vector) {
		return
	}

	// la direccion de la PANTALLA a la del MAPA: es la inversa exacta de la proyeccion del viewport
	// (WorldToOrtho: x' = (x-y)*80, y' = (x+y)*40). Despejando, un paso de (dx,dy) pixeles es
	// (dx/80 + dy/40)/2 en X y (dy/40 - dx/80)/2 en Y del mapa.
	mundoX := (ejeX/80.0 + ejeY/40.0) / 2.0
	mundoY := (ejeY/40.0 - ejeX/80.0) / 2.0

	largo := math.Sqrt(mundoX*mundoX + mundoY*mundoY)
	if largo == 0 {
		return
	}

	mundoX, mundoY = mundoX/largo, mundoY/largo

	// el objetivo: `celdasPorDelante` celdas en esa direccion (en subtiles, que es la unidad del motor)
	paso := celdasPorDelante159 * subtilesPorCelda159
	origen := h.jugador.Position
	objetivo := d2vector.NewPosition(
		origen.X()+mundoX*paso,
		origen.Y()+mundoY*paso,
	)

	// la direccion a la que queda mirando: la MISMA cuenta que hace el motor en `setTarget`
	// (`m.Position.DirectionTo(m.Target.Vector)`, 0..63 de a 8), asi la evidencia no depende de un lector
	// nuevo del ente.
	h.direccion = origen.DirectionTo(objetivo.Vector)

	// `SetPath` es el camino del motor: al fijarlo, `setTarget` calcula la direccion y `Player.Advance`
	// elige el modo de animacion y avanza el compuesto.
	h.jugador.SetPath([]d2vector.Position{objetivo}, nil)
	h.objetivo = objetivo
	h.tieneObjetivo = true
	h.ultimoEjeX, h.ultimoEjeY = ejeX, ejeY
	h.caminando = true
	h.pasos++
}

// ejeDeLaCruceta: las cuatro flechas del control en un eje de pantalla (-1, 0, 1). Dos apretadas a la vez
// dan la diagonal, que es lo que hace que sean 8 direcciones y no 4.
func (h *HeroeDelGate0) ejeDeLaCruceta() (float64, float64) {
	var x, y float64

	if h.entrada.IsKeyPressed(d2enum.KeyLeft) {
		x--
	}

	if h.entrada.IsKeyPressed(d2enum.KeyRight) {
		x++
	}

	if h.entrada.IsKeyPressed(d2enum.KeyUp) {
		y--
	}

	if h.entrada.IsKeyPressed(d2enum.KeyDown) {
		y++
	}

	return x, y
}

// PieDeLaPantalla: donde caen los pies del heroe en el lienzo (el ancla del compuesto). Es la cuenta del
// viewport, la misma con la que el renderer dibuja los entes: la posicion del motor en SUBTILES se pasa a
// celdas (que es la unidad del mundo del renderer) y de ahi a pixeles.
func (h *HeroeDelGate0) PieDeLaPantalla() (int, int) {
	// sin camara todavia (el heroe se crea antes que el renderer) no hay pixel que calcular
	if h.mapa == nil {
		return 0, 0
	}

	pos := h.jugador.GetPosition()
	px, py := h.mapa.WorldToScreenF(pos.X()/subtilesPorCelda159, pos.Y()/subtilesPorCelda159)

	return int(px), int(py)
}

// cajaDelCuadro: la caja que ocupa el cuadro que se va a dibujar, relativa a los pies del heroe (el origen del
// ente). Sale del compuesto, que es el unico que sabe que capas del COF se dibujan en este cuadro y con que
// desplazamiento cada una (DC-002.159c). Si el compuesto todavia no tiene modo no hay caja, y el que llama no
// dibuja: dibujar sin haber guardado lo de abajo deja un fantasma que despues no se puede borrar.
func (h *HeroeDelGate0) cajaDelCuadro() (izq, arriba, ancho, alto int, ok bool) {
	if h.compuesto == nil {
		return 0, 0, 0, 0, false
	}

	return h.compuesto.Recuadro159()
}

// DibujarSobreElMapa: el cuadro del heroe cuando el mapa NO se repinta. `mapaRepintado` dice si el
// renderer acaba de pintar el mapa entero en este cuadro: en ese caso no hay nada que borrar (los pixeles
// viejos del heroe ya los tapo el mapa) y solo se guarda lo de abajo; si el mapa se salto, primero se
// devuelve al lienzo el recuadro guardado, que es el borrado.
//
// Sin lienzo con memoria (la PC) no hace nada: ahi el mapa se pinta entero cada cuadro y el heroe lo dibuja
// el renderer dentro de sus pasadas.
//
// DC-002.159c: el recuadro sale del COMPUESTO (`Player.RecuadroDelCuadro159`), no de una constante. Es lo que
// cubre el cuadro de caminar (las capas de las piernas se dibujan con su propio desplazamiento) y lo que evita
// que queden piernas pegadas al mapa. Si el compuesto todavia no tiene modo, no hay caja: no se dibuja (dibujar
// sin haber guardado lo de abajo deja un fantasma que no se puede borrar).
func (h *HeroeDelGate0) DibujarSobreElMapa(target d2interface.Surface, mapaRepintado bool) {
	if h.lienzo == nil {
		return
	}

	if mapaRepintado {
		h.hayCopia = false
	} else if h.hayCopia {
		h.lienzo.PegarRect159(h.memoria, h.copiaX, h.copiaY, h.copiaAncho, h.copiaAlto)
		h.hayCopia = false
		h.borrados159++

		// DC-002.159c (instrumento): se vuelve a leer el mismo recuadro y se compara con lo que se guardo. Si el
		// borrado funciona, no puede diferir ni un byte; si difiere, el fantasma no es la caja sino el lienzo.
		if !h.volvioElRecuadro() {
			h.noVolvieron159++
		}
	}

	izq, arriba, ancho, alto, hayCaja := h.cajaDelCuadro()
	if !hayCaja || ancho <= 0 || alto <= 0 {
		h.sinCaja159++

		return
	}

	h.ultimaCaja = [4]int{izq, arriba, ancho, alto}

	if !h.asegurarMemoria(ancho, alto) {
		return
	}

	// DONDE EMPIEZA EL SPRITE (DC-002.159c): en el origen del COMPUESTO, que es la traslacion del tile mas el
	// corrimiento propio del ente (`MapRenderer.OrigenDelEnte159`). No es `PieDeLaPantalla` (`pie=`): esa es la
	// posicion exacta del ente en pantalla, y el sprite se dibuja hasta 16 px al costado y 11 px mas abajo que eso.
	// Con el recuadro puesto en `pie` el borrado dejaba un borde del cuadro anterior en cada cuadro de caminar.
	dibX, dibY := h.mapa.OrigenDelEnte159(h.jugador)
	x := dibX + izq
	y := dibY + arriba

	// guardar lo de abajo ANTES de dibujar. DC-002.159b: el lienzo recorta el recuadro al borde de la pantalla
	// y devuelve el recorte, asi que el borrado funciona con el heroe en el borde; si el recuadro no toca el
	// lienzo (el heroe se fue de la vista) no hay copia y NO se dibuja: nada que borrar despues.
	escritos, xc, yc, anchoC, altoC := h.lienzo.CopiarRect159(h.memoria, x, y, ancho, alto)
	if escritos <= 0 || anchoC <= 0 || altoC <= 0 {
		return
	}

	h.hayCopia = true
	h.copiaX, h.copiaY, h.copiaAncho, h.copiaAlto = xc, yc, anchoC, altoC

	h.dibujos159++
	h.mapa.DibujarEnte159(target, h.jugador)

	// DC-002.159c (instrumento): cuantos pixeles del recuadro cambio el dibujo. Si esto es 0, el compuesto no
	// pinto nada ahi (el sprite no esta donde dice el pie) y no hay fantasma que buscar.
	if escritos2, _, _, _, _ := h.lienzo.CopiarRect159(h.memoria2, h.copiaX, h.copiaY, h.copiaAncho, h.copiaAlto); escritos2 > 0 {
		pintados := 0

		for i := 0; i < h.copiaAncho*h.copiaAlto*4; i += 4 {
			if h.memoria[i] != h.memoria2[i] || h.memoria[i+1] != h.memoria2[i+1] ||
				h.memoria[i+2] != h.memoria2[i+2] || h.memoria[i+3] != h.memoria2[i+3] {
				pintados++
			}
		}

		h.pintados159 += pintados

		if pintados > 0 {
			h.cuadrosPintados159++
		}
	}
}

// OlvidarCopia: el lienzo ya no tiene lo que se guardo (el mapa se repinto encima o cambio la pantalla).
func (h *HeroeDelGate0) OlvidarCopia() { h.hayCopia = false }

// volvioElRecuadro (instrumento de DC-002.159c): vuelve a leer del lienzo el recuadro que se acaba de devolver y
// lo compara con el que se habia guardado. `true` si no difiere ni un byte.
//
// POR QUE. El fantasma del caminar puede venir de dos lados muy distintos: la CAJA no cubre el sprite, o el
// LIENZO no devuelve lo que guardo. Esta comparacion separa los dos casos sin mirar capturas: si esto da
// `false`, el borrado no esta escribiendo lo que cree y el problema no es la caja.
func (h *HeroeDelGate0) volvioElRecuadro() bool {
	if h.memoria2 == nil || h.copiaAncho <= 0 || h.copiaAlto <= 0 {
		return true
	}

	escritos, _, _, _, _ := h.lienzo.CopiarRect159(h.memoria2, h.copiaX, h.copiaY, h.copiaAncho, h.copiaAlto)
	if escritos <= 0 {
		return false
	}

	n := h.copiaAncho * h.copiaAlto * 4

	for i := 0; i < n; i++ {
		if h.memoria[i] != h.memoria2[i] {
			return false
		}
	}

	return true
}

// Estado: una linea para el log de la corrida: donde esta, que direccion mira, si camina o esta quieto, que
// modo de animacion usa el compuesto y con que recuadro se borra. Es lo que la evidencia imprime cada vez que
// algo cambia.
//
// DC-002.159c agrega dos cosas a la linea: la CAJA que dio el compuesto (`caja=izq,arriba,anchoxalto`, relativa a
// los pies) y las SUPERFICIES DE CUADRO pedidas desde que arranco el proceso (`sup=cuantas/bytes`). La primera es
// la prueba de que el recuadro del borrado cubre el cuadro de caminar; la segunda es la prueba de que la
// superficie unica no se pide cuadro a cuadro.
func (h *HeroeDelGate0) Estado() string {
	pos := h.jugador.GetPosition()
	px, py := h.PieDeLaPantalla()
	superficies, bytes := d2asset.SuperficiesDeCuadro159()
	dibX, dibY := h.mapa.UltimoDibujo159()

	return fmt.Sprintf("celda=%.2f,%.2f pie=%d,%d dir=%d modo=%s camina=%v pasos=%d caja=%d,%d,%dx%d "+
		"sup=%d/%dB sincaja=%d giros=%d decod=%d exp=%d/%dB borrados=%d noVolvieron=%d dibujos=%d dib=%d,%d "+
		"pintados=%d cuadrosPintados=%d taller=%v",
		pos.X()/subtilesPorCelda159, pos.Y()/subtilesPorCelda159, px, py,
		h.direccion, h.jugador.GetAnimationMode().String(), h.caminando, h.pasos,
		h.ultimaCaja[0], h.ultimaCaja[1], h.ultimaCaja[2], h.ultimaCaja[3],
		superficies, bytes, h.sinCaja159,
		d2asset.GirosDelCompuesto159(), d2asset.DecodificacionesDeDireccion159(),
		d2util.PedidosDeExpansion, d2util.BytesDeExpansion,
		h.borrados159, h.noVolvieron159, h.dibujos159, dibX, dibY,
		h.pintados159, h.cuadrosPintados159, d2asset.TallerDeDecodificacion159())
}

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
	celdasPorDelante159 = 1.0

	// El recuadro de lienzo que se guarda debajo del heroe, en pixeles. La caja del compuesto de la amazona son
	// 60x76 medidos (DC-002.38), pero el recuadro NO es la caja: tiene que cubrir donde el sprite CAE en el
	// lienzo, y eso incluye el desplazamiento de cada capa (el pelo, el arma) y el cuadro de la animacion. Con
	// 96x112 quedaba un fantasma de ~25 pixeles del cuadro anterior al caminar -- medido en el Gate 0: dos
	// copias del heroe a un lado de la otra en la captura de los 54 s (DC-002.159b). 160x160 le deja 50 px de
	// aire por lado; son 102.400 B de buffer, una sola vez.
	anchoDeLaMemoria159 = 160
	altoDeLaMemoria159  = 160

	// donde queda el recuadro respecto de los PIES del heroe (el ancla del compuesto): centrado a lo
	// ancho y con la caja para arriba (el personaje se dibuja desde los pies hacia arriba).
	memoriaIzquierda159 = -anchoDeLaMemoria159 / 2
	memoriaArriba159    = -altoDeLaMemoria159 + 24
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

	// lienzo: si la Surface sabe guardar y devolver un recuadro. nil = no se puede (la PC).
	lienzo   lienzoConMemoria159
	memoria  []byte
	hayCopia bool
	copiaX   int
	copiaY   int
	// DC-002.159b: el recuadro que de verdad se guardo (el recorte al lienzo cuando el heroe esta en el borde).
	// Es el que se devuelve: devolver el recuadro entero moveria pixeles que no se copiaron.
	copiaAncho int
	copiaAlto  int

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
func (h *HeroeDelGate0) ConMemoria(lienzo lienzoConMemoria159) {
	h.lienzo = lienzo

	if lienzo == nil {
		return
	}

	if h.memoria == nil {
		h.memoria = make([]byte, anchoDeLaMemoria159*altoDeLaMemoria159*4)
	}
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

// DibujarSobreElMapa: el cuadro del heroe cuando el mapa NO se repinta. `mapaRepintado` dice si el
// renderer acaba de pintar el mapa entero en este cuadro: en ese caso no hay nada que borrar (los pixeles
// viejos del heroe ya los tapo el mapa) y solo se guarda lo de abajo; si el mapa se salto, primero se
// devuelve al lienzo el recuadro guardado, que es el borrado.
//
// Sin lienzo con memoria (la PC) no hace nada: ahi el mapa se pinta entero cada cuadro y el heroe lo dibuja
// el renderer dentro de sus pasadas.
func (h *HeroeDelGate0) DibujarSobreElMapa(target d2interface.Surface, mapaRepintado bool) {
	if h.lienzo == nil || h.memoria == nil {
		return
	}

	if mapaRepintado {
		h.hayCopia = false
	} else if h.hayCopia {
		h.lienzo.PegarRect159(h.memoria, h.copiaX, h.copiaY, h.copiaAncho, h.copiaAlto)
		h.hayCopia = false
	}

	px, py := h.PieDeLaPantalla()
	x := px + memoriaIzquierda159
	y := py + memoriaArriba159

	// guardar lo de abajo ANTES de dibujar. DC-002.159b: el lienzo recorta el recuadro al borde de la pantalla
	// y devuelve el recorte, asi que el borrado funciona con el heroe en el borde; si el recuadro no toca el
	// lienzo (el heroe se fue de la vista) no hay copia y NO se dibuja: nada que borrar despues.
	escritos, xc, yc, anchoC, altoC := h.lienzo.CopiarRect159(h.memoria, x, y, anchoDeLaMemoria159, altoDeLaMemoria159)
	if escritos <= 0 || anchoC <= 0 || altoC <= 0 {
		return
	}

	h.hayCopia = true
	h.copiaX, h.copiaY, h.copiaAncho, h.copiaAlto = xc, yc, anchoC, altoC

	h.mapa.DibujarEnte159(target, h.jugador)
}

// OlvidarCopia: el lienzo ya no tiene lo que se guardo (el mapa se repinto encima o cambio la pantalla).
func (h *HeroeDelGate0) OlvidarCopia() { h.hayCopia = false }

// Estado: una linea para el log de la corrida: donde esta, que direccion mira, si camina o esta quieto y
// que modo de animacion usa el compuesto. Es lo que la evidencia imprime cada vez que algo cambia.
func (h *HeroeDelGate0) Estado() string {
	pos := h.jugador.GetPosition()
	px, py := h.PieDeLaPantalla()

	return fmt.Sprintf("celda=%.2f,%.2f pie=%d,%d dir=%d modo=%s camina=%v pasos=%d",
		pos.X()/subtilesPorCelda159, pos.Y()/subtilesPorCelda159, px, py,
		h.direccion, h.jugador.GetAnimationMode().String(), h.caminando, h.pasos)
}

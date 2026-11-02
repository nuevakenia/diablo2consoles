//go:build dreamcast

// DC-002.113 -- EL MOVIMIENTO: del boton al mapa, con las cuentas del MOTOR.
//
// QUE HACE ESTE ARCHIVO. Tres cosas y ninguna es una tabla propia:
//
//  1. LEE EL CONTROL por `d2interface.InputService` (el contrato del motor, DC-002.71), no por el bus maple ni
//     por el shim de Ebitengine: `IsKeyPressed(KeyLeft|KeyRight|KeyUp|KeyDown)`. La cruceta y la palanca YA llegan
//     ahi traducidas por el backend de la consola (`d2core/d2input/maple`, 9 metodos, 12 pruebas en el host).
//
//  2. CONVIERTE los ejes de la pantalla en la DIRECCION del personaje con la cadena del motor
//     (medida en DC-002.49b, no elegida aca):
//
//     (ejeX,ejeY) de la pantalla -> `DireccionDelPiso` (inversa de la proyeccion iso, en celdas, unitaria)
//     -> `d2vector.Vector.DirectionTo` (64 direcciones)
//     -> `d2cof.Dir64ToCof` (las 16 del COF)
//
//     Los 8 estados de la cruceta dan, exactamente, las direcciones `[0 1 4 7 8 9 12 15]` del COF.
//
//  3. MUEVE la posicion con la velocidad de la tabla del exe (`personaje.go`, el archivo COMPARTIDO con el
//     medidor del host: `d2path.PlayerSubtilesPerSecond` -> 9,375 subtiles/s caminando = 1,875 celdas/s) y con
//     el reloj de la consola. El paso se cobra en TICKS de 25 Hz (el reloj de la logica del motor,
//     `d2path.TickRate`): el resto que no llega a un tick se guarda para el cuadro siguiente, asi que un cuadro
//     lento no pierde ni inventa movimiento -- es la misma cuenta que hace el motor con sus 25 cuadros por
//     segundo.
//
// LO QUE NO HACE: empujar al jugador a traves de las paredes de forma distinta a la del motor. Esta tarjeta
// mueve en linea recta con la velocidad del motor y mide; la colision del pueblo es otra tarjeta.
package main

import (
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2enum"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2cof"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2interface"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2math/d2vector"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2path"
)

const (
	// segundosPorTick: el reloj de la logica del motor son 25 Hz (d2path.TickRate, VERIFIED).
	segundosPorTick = 1.0 / float64(d2path.TickRate)

	// topeDeTicks: cuantos ticks puede cobrar UN cuadro. Un cuadro de este POC tarda ~0,7-1,2 s (el mapa se
	// rasteriza con la CPU: 640x480), o sea 17-30 ticks; el tope es 25 (1 s) para que una pausa del emulador
	// (una recoleccion, una carga del disco) no teletransporte al jugador. Los cuadros recortados se CUENTAN.
	topeDeTicks = d2path.TickRate

	// direccionesDelCOFDelHeroe: el COF de la amazona declara 16 direcciones (medido: AMTWhth.COF 1.132 B).
	direccionesDelCOFDelHeroe = 16
)

// Ejes: lo que la cruceta pide en la PANTALLA (x a la derecha, y abajo), cada uno en -1..1.
func ejesDelControl(entrada d2interface.InputService) (float64, float64) {
	ejeX, ejeY := 0.0, 0.0

	if entrada.IsKeyPressed(d2enum.KeyLeft) {
		ejeX--
	}

	if entrada.IsKeyPressed(d2enum.KeyRight) {
		ejeX++
	}

	if entrada.IsKeyPressed(d2enum.KeyUp) {
		ejeY--
	}

	if entrada.IsKeyPressed(d2enum.KeyDown) {
		ejeY++
	}

	return ejeX, ejeY
}

// DireccionDelControl: los ejes de la pantalla -> la direccion del COF que el personaje mira, con la cadena del
// motor (`personaje.go: DireccionDelPiso` -> `DirectionTo` -> `Dir64ToCof`). No hay tabla propia: si el motor
// cambia de criterio, esto lo sigue.
func DireccionDelControl(ejeX, ejeY float64) int {
	dx, dy := DireccionDelPiso(ejeX, ejeY)

	origen := d2vector.NewVector(0, 0)
	destino := d2vector.NewVector(dx, dy)

	return d2cof.Dir64ToCof(origen.DirectionTo(*destino), direccionesDelCOFDelHeroe)
}

// DireccionesDelControl: las direcciones del COF DISTINTAS que la cruceta puede pedir (8 de los 9 estados de la
// cruceta: el centro no pide ninguna). Es el conjunto que hay que tener compuesto para que caminar en cualquier
// direccion no cueste nada en el cuadro. Medido en DC-002.49b: [0 1 4 7 8 9 12 15].
func DireccionesDelControl() []int {
	vistas := make(map[int]bool, 8)
	dirs := make([]int, 0, 8)

	for ejeX := -1; ejeX <= 1; ejeX++ {
		for ejeY := -1; ejeY <= 1; ejeY++ {
			if ejeX == 0 && ejeY == 0 {
				continue
			}

			d := DireccionDelControl(float64(ejeX), float64(ejeY))
			if vistas[d] {
				continue
			}

			vistas[d] = true
			dirs = append(dirs, d)
		}
	}

	return dirs
}

// movimiento: el jugador que camina. La POSICION es la del motor (`personaje.go: Estado`, en subtiles de 5 por
// celda) y todo lo demas es lo que hace falta para medir: cuanto camino, en cuanto tiempo y con que reloj de
// animacion.
type movimiento struct {
	Est       *Estado
	Direccion int     // la direccion del COF que mira (0..15)
	Caminando bool    // si este cuadro la cruceta pedia algo
	Resto     float64 // segundos que todavia no llegaron a un tick de 25 Hz
	MsAnim    float64 // el reloj de la animacion (corre solo caminando)

	Ticks     int     // ticks de 25 Hz cobrados
	Recortes  int     // cuadros en los que el paso se recorto (paso > topeDeTicks)
	Subtiles  float64 // subtiles caminados (la distancia que suma la velocidad del motor)
	Segundos  float64 // segundos caminados (el reloj de la consola, no el de los ticks)
	PasoMayor float64 // el paso mas largo que se cobro, en segundos
}

// NuevoMovimiento: el jugador parado en una celda del mapa.
func NuevoMovimiento(xCelda, yCelda float64) *movimiento {
	return &movimiento{
		Est:       NuevoEstado(xCelda, yCelda, false),
		Direccion: dirQuieto,
	}
}

// Paso: un cuadro del movimiento. `segundos` es lo que duro el cuadro de verdad (el reloj de la consola). Si la
// cruceta no pide nada, el paso solo acumula el reloj de la animacion en cero (el personaje queda quieto y el
// ciclo de caminar vuelve a empezar en el cuadro 0 la proxima vez, como el motor al cambiar de modo).
//
// Devuelve los ticks cobrados y los subtiles caminados en este cuadro.
func (m *movimiento) Paso(ejeX, ejeY, segundos float64) (ticks int, caminado float64) {
	m.Caminando = ejeX != 0 || ejeY != 0

	if !m.Caminando || segundos <= 0 || m.Est == nil {
		m.Resto = 0
		m.MsAnim = 0

		return 0, 0
	}

	m.Direccion = DireccionDelControl(ejeX, ejeY)

	// el reloj de la animacion: los ms del cuadro, para elegir el cuadro del ciclo (TW: 40 ms por cuadro)
	m.MsAnim += segundos * 1000.0

	// el reloj de la logica: ticks ENTEROS de 25 Hz, el resto queda para el cuadro siguiente
	m.Resto += segundos

	ticks = int(m.Resto / segundosPorTick)
	if ticks > topeDeTicks {
		ticks = topeDeTicks
		m.Recortes++
	}

	m.Resto -= float64(ticks) * segundosPorTick
	if m.Resto < 0 || m.Resto >= segundosPorTick {
		// por si una suma de flotantes deja el resto fuera de [0, tick): se normaliza, no se arrastra un bug
		m.Resto = 0
	}

	if ticks <= 0 {
		return 0, 0
	}

	cobrados := float64(ticks) * segundosPorTick
	dx, dy := DireccionDelPiso(ejeX, ejeY)

	caminado = m.Est.Avanzar(dx, dy, cobrados)

	m.Ticks += ticks
	m.Subtiles += caminado
	m.Segundos += cobrados

	if cobrados > m.PasoMayor {
		m.PasoMayor = cobrados
	}

	return ticks, caminado
}

// CuadroDeLaAnimacion: cual de los cuadros del ciclo toca, por el reloj del movimiento. Es la cuenta del motor:
// el cuadro avanza cada `msPorCuadro` y da la vuelta (el `Animation.Advance` de `d2core/d2asset`).
func (m *movimiento) CuadroDeLaAnimacion(ciclo *CicloDelJugador) *CuadroDelJugador {
	if ciclo == nil || len(ciclo.Cuadros) == 0 {
		return nil
	}

	ms := int(m.MsAnim)
	if ciclo.MsPorCuadro > 0 {
		ms = ms / ciclo.MsPorCuadro
	}

	return &ciclo.Cuadros[ms%len(ciclo.Cuadros)]
}

// ============================== EL GUION DEL SHIM (SIN MANDO FISICO) ==============================
//
// Cuando el emulador no tiene un control en el puerto 0, el estado del maple lo pone el SHIM
// (`d2c_control_inyectar`, console/dc/std/csrc/control_dc.c) y el POC lo empuja cuadro a cuadro desde esta
// tabla. Son los MISMOS 8 estados de la cruceta que manda el teclado virtual del emulador en la corrida del
// criterio, con la misma duracion: asi las dos corridas se pueden comparar tramo por tramo.
//
// Los bits son los del control de la Dreamcast (d2core/d2input/maple: BitUp/BitDown/BitLeft/BitRight, copiados
// del header de KOS). El estado entra por la MISMA puerta que un control de verdad: el servicio traduce los
// bits a las teclas de la tabla, y el POC pregunta por las teclas, no por los bits.
type tramoDelGuion struct {
	Nombre   string
	Botones  uint32
	Segundos float64
}

const (
	bitArriba    uint32 = 1 << 4
	bitAbajo     uint32 = 1 << 5
	bitIzquierda uint32 = 1 << 6
	bitDerecha   uint32 = 1 << 7
)

// segundosDeCadaTramo: lo mismo que `GUION_SEGUNDOS` de run-flycast-113.sh (la corrida con teclado virtual).
const segundosDeCadaTramo = 5.0

// guionInyectado: las 8 direcciones de la cruceta, una detras de otra, empezando y terminando cerca del origen
// (derecha, derecha+abajo, abajo, ...). Cada tramo dura `segundosDeCadaTramo`.
var guionInyectado = []tramoDelGuion{
	{"DERECHA", bitDerecha, segundosDeCadaTramo},
	{"ABAJO+DERECHA", bitAbajo | bitDerecha, segundosDeCadaTramo},
	{"ABAJO", bitAbajo, segundosDeCadaTramo},
	{"ABAJO+IZQUIERDA", bitAbajo | bitIzquierda, segundosDeCadaTramo},
	{"IZQUIERDA", bitIzquierda, segundosDeCadaTramo},
	{"ARRIBA+IZQUIERDA", bitArriba | bitIzquierda, segundosDeCadaTramo},
	{"ARRIBA", bitArriba, segundosDeCadaTramo},
	{"ARRIBA+DERECHA", bitArriba | bitDerecha, segundosDeCadaTramo},
}

// BotonesDelGuion: que botones toca en el segundo `t` del guion (0 = quieto entre tramo y tramo, y despues del
// ultimo). El que llama suma la pausa entre tramos, para que el guion inyectado se parezca al del teclado
// virtual (que suelta la tecla y espera `PAUSA` antes del tramo siguiente).
func BotonesDelGuion(t, pausa float64) uint32 {
	for i := range guionInyectado {
		ini := float64(i) * (guionInyectado[i].Segundos + pausa)
		fin := ini + guionInyectado[i].Segundos

		if t >= ini && t < fin {
			return guionInyectado[i].Botones
		}
	}

	return 0
}

// SegundosDelGuion: lo que dura el guion entero (con las pausas), para saber si entra en los cuadros que se van
// a dibujar.
func SegundosDelGuion(pausa float64) float64 {
	return float64(len(guionInyectado)) * (segundosDeCadaTramo + pausa)
}

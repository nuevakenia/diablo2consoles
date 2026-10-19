package d2mapentity

import (
	"fmt"
	"os"
	"sync"
)

// Instrumento de la ANIMACION del jugador en el HOST (DC-002.49), apagado salvo que se pida:
//
//	OD2_ANIMSTATS=1 ./od2host ...
//
// Imprime una linea por segundo de tiempo de juego con el estado real que el motor esta DIBUJANDO:
//
//	ANIMSTATS t=3s modo=TN cuadro=6 de 16 direccion=4 vueltas=0
//
// POR QUE NO SE MIDE DIFERENCIANDO CAPTURAS: medido, el cuadro del motor cambia ~60-88 % de sus pixeles entre
// dos instantes con el personaje QUIETO (el motor re-sombrea la pantalla con las luces cada cuadro), asi que
// "cuantos pixeles cambiaron" no distingue la animacion del resto. Lo que si es un numero limpio es el CUADRO
// que el motor pide dibujar: si avanza 0,1,2,... y vuelve a 0, el ciclo corre, y con el reloj al lado se sabe
// a que velocidad. Es la referencia contra la que se mide la consola.
//
// El costo apagado es una comparacion de un bool por cuadro; encendido, una linea por segundo.
var (
	animStatsOn   = os.Getenv("OD2_ANIMSTATS") != ""
	animStatsMu   sync.Mutex
	animStatsAcum float64
	animStatsSeg  int
)

// instrumentarAnimacion la llama Player.Advance una vez por cuadro.
func (p *Player) instrumentarAnimacion(tickTime float64) {
	if !animStatsOn {
		return
	}

	animStatsMu.Lock()

	animStatsAcum += tickTime
	if animStatsAcum < 1.0 {
		animStatsMu.Unlock()

		return
	}

	animStatsAcum = 0

	animStatsSeg++
	seg := animStatsSeg

	animStatsMu.Unlock()

	fmt.Printf("ANIMSTATS t=%ds modo=%s cuadro=%d de %d direccion=%d vueltas=%d\n",
		seg, p.composite.GetAnimationMode(), p.composite.GetCurrentFrame(),
		p.composite.GetFrameCount(), p.composite.GetDirection(), p.composite.GetPlayedCount())
}

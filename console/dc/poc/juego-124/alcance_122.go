//go:build dreamcast

// DC-002.122 -- EL ALCANCE DEL GOLPE: cuando el golpe del heroe llega y cuando no.
//
// DE DONDE SALEN LOS DOS NUMEROS (no son elegidos aca):
//
//   - LA DISTANCIA: `d2skill.TableDistanceSized`, que es `UNIT_GetTableDistance` 0x642980 del exe, el MISMO
//     paquete que usa el motor para decidir si una entidad alcanza a otra (`d2core/d2monsters/combat.go`,
//     `HeroStrike`: `d2skill.MeleeReachFull(...)`). No se reescribe la tabla aca: se importa el paquete del
//     motor, asi que si el motor corrige la tabla, la consola la corrige con el.
//   - EL TAMANO DEL HEROE: `d2skill.HeroUnitSize` = 2 (`UNIT_GetSizeX` de un jugador, leido en 0x6207a0).
//   - EL TAMANO DEL BICHO: se pide a monstats2 (`SizeX`), que es lo que hace el motor
//     (`game_monsters.go: heroInSwingReachOf`). Si la tabla no lo trae, 1 (el valor por omision de
//     `d2skill.UnitSizeOf`).
//   - EL ALCANCE: `meleeInRange` = 7 subtiles, el valor VERIFICADO del exe para la distancia de borde de un
//     ataque cuerpo a cuerpo (`d2core/d2monsters/director.go`). El motor suma `heroReach/2` cuando el que
//     ataca es el bicho (`combat.go: monsterStrike`); para el heroe usa el gate de la tabla de skills
//     (`attackReach(false)` = 7). Aca se usa el 7 tal cual y la palanca `alcanceGolpe` lo deja mover para el A/B.
//
// POR QUE NO SE USA EL GATE ESTRICTO DE 2. El motor camina al heroe hasta que el gate de 0x57cc10 da true, o
// sea hasta quedar pegado al bicho (distancia de tabla <= 2 con las manos vacias, `rangeadder` 0). Este POC no
// tiene ese paseo fino: la colision de celda de DC-002.121 le prohibe al jugador ENTRAR a la celda del bicho, y
// el jugador se para en el centro de su celda (5 subtiles = una celda entera de separacion). Con el gate estricto
// NINGUN golpe llegaria nunca, y el numero que se mediria seria el de la celda, no el del alcance. Con la
// distancia VERIFICADA de 7 subtiles (1,4 celdas) un jugador en la celda de al lado SI alcanza y uno a dos
// celdas NO: eso es lo que se mide (criterio: los golpes de adentro bajan la vida y los de afuera no).
package main

import (
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2skill"
)

// DistanciaDeMesa: la distancia del exe entre dos unidades de tamaño `sa`/`sb` separadas `dx`/`dy` subtiles.
func DistanciaDeMesa(dx, dy, sa, sb int) int {
	return d2skill.TableDistanceSized(dx, dy, sa, sb)
}

// DistanciaEnSubtiles: la separacion de dos posiciones del motor (subtiles) redondeada a subtiles enteros, que es
// lo que come la tabla del exe (el motor tambien la llama con enteros: `int(posicion.X())`).
func DistanciaEnSubtiles(ax, ay, bx, by float64) (int, int) {
	return int(bx) - int(ax), int(by) - int(ay)
}

// AlcanzaElGolpe: si un golpe cuerpo a cuerpo del heroe (desarmado) llega al bicho. Es la regla del motor con la
// distancia de su tabla y el alcance VERIFICADO del exe (ver el comentario de arriba).
func AlcanzaElGolpe(dx, dy, tamanoBicho, alcance int) bool {
	return DistanciaDeMesa(dx, dy, d2skill.HeroUnitSize, tamanoBicho) <= alcance
}

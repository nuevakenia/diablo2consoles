//go:build dreamcast

package main

// DC-002.131 -- EL ARTE COMPUESTO AL MONTON DE C: lo que hoy ocupa el semiespacio del recolector sin ser estado.
//
// EL PROBLEMA, MEDIDO. El recolector de libgodc es de COPIA y su semiespacio son 1.835.008 B. El POC llega al bucle
// de cuadros con ~1,65 MB VIVOS (no asignados: vivos, medidos con `vivo()` de medidor_131.go, que fuerza una
// recoleccion y lee lo copiado). Con esa carga, lo que decide si una corrida vive o muere no es el promedio sino el
// PICO: la copia tiene que entrar entera en el semiespacio de destino, y el barrido de la pila es conservador, asi
// que a lo vivo se le suma lo que la pila todavia apunta. Un cuadro del split que pide de mas, o la carga de un NPC,
// y el recolector tira `GC to-space overflow` / `out of memory` (medido: el split con el HUD de DC-002.125 murio en
// el cuadro 4 con 1.650.000 B vivos, y el split con las piezas de DC-002.130 murio DURANTE la carga de los NPCs con
// 1.696.960 B).
//
// QUE ES LO QUE ESTA VIVO, Y QUE NO DEBERIA ESTAR EN EL SEMIESPACIO. Lo que se queda vivo de este POC es una mezcla
// de ESTADO (posiciones, relojes, contadores: decenas de KB) y de ARTE YA COMPUESTO (los cuadros de los heroes, del
// bicho y de los NPCs en indices de 8 bits, 1 B/pixel). El arte no cambia en toda la corrida y no lleva un solo
// puntero adentro: es exactamente el caso del HUD (DC-002.125) y de los NPCs (DC-002.127), que ya lo tienen en el
// monton de C. Los del JUGADOR (165.152 B en las 8 direcciones de TW + el quieto) y los del BICHO (51.880 B) seguian
// en el semiespacio, y son los dos pedazos mas grandes que se pueden mover sin tocar una linea del camino
// compartido con el host: `personaje.go` y `jugador.go` siguen compilando LOS MISMOS bytes en la consola y en el
// medidor del host (el build lo verifica con `cmp`), y lo unico que se cambia aca es DONDE viven los indices que ya
// estan compuestos.
//
// EL CONTRATO DE ESTE ARCHIVO. Copiar y repuntar, nada mas:
//   - se copia el contenido a un bloque del monton de C (`bloqueDeC`, ver hud_125.go) y se repunta el slice;
//   - los bytes son LOS MISMOS, asi que el crc32 de cada cuadro no cambia (y es lo que se comprueba en la corrida:
//     los crc32 de la evidencia son los mismos de antes del cambio);
//   - el bloque NO se devuelve: el arte vive lo que vive la corrida, igual que el del HUD;
//   - si el monton de C no da el bloque, `bloqueDeC` avisa y devuelve el `make` de siempre: la corrida queda como
//     estaba, lenta de memoria pero identica en pixeles.

// bytesDeArteEnElMontonDeC: lo que esta tarjeta movio de lugar, para la evidencia.
var bytesDeArteEnElMontonDeC int

// arteDelCicloAlMontonDeC: copia los cuadros compuestos de un ciclo al monton de C y devuelve los bytes movidos.
// Idempotente por construccion: si el slice ya no es del semiespacio (esta en el monton de C), volver a copiarlo
// solo gasta memoria, asi que se llama UNA vez por ciclo, donde el ciclo se acaba de componer.
func arteDelCicloAlMontonDeC(ciclo *CicloDelJugador) int {
	if arteEnMontonDeC != "si" {
		return 0
	}

	if ciclo == nil || len(ciclo.Cuadros) == 0 {
		return 0
	}

	movidos := 0

	for f := range ciclo.Cuadros {
		cu := &ciclo.Cuadros[f]

		if len(cu.Pixeles) == 0 {
			continue
		}

		bloque := bloqueDeC(len(cu.Pixeles))
		if len(bloque) != len(cu.Pixeles) {
			continue
		}

		copy(bloque, cu.Pixeles)

		cu.Pixeles = bloque
		movidos += len(bloque)
	}

	bytesDeArteEnElMontonDeC += movidos

	return movidos
}

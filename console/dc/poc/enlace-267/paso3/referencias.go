// DC-002.67 PASO 3: subir la apuesta -- REFERENCIAR d2core/d2ui y d2game/d2gamescreen sin instanciarlos.
//
// Este archivo NO entra al binario del paso 2: vive en un subdirectorio, y los dos builds (gccgo y `go run`)
// solo miran los .go de primer nivel del POC. Lo mete el build-enlace-267.sh con PASO3=1, copiandolo al lado
// de main.go.
//
// Que mide: enlazar los 125 paquetes del cierre de d2ui + d2gamescreen (medido con `go list -deps`) y ver
// cuanto crece el binario y si un 1ST_READ.BIN de ese tamano arranca en Flycast. No instancia nada: solo se
// toma la direccion de dos constructores, que es el minimo que obliga al enlazador a traerse el paquete.
package main

import (
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2ui"
	"github.com/OpenDiablo2/OpenDiablo2/d2game/d2gamescreen"
)

var (
	_ = d2ui.NewUIManager
	_ = d2gamescreen.CreateGame
)

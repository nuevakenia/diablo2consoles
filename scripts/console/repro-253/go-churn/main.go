// Reproduccion minima de DC-002.53, rung C: como el rung B (que anda), pero antes de encender el PVR se
// hace el mismo churn de monton que hace el POC: un buffer de 1 MB, treinta y pico de GCs y basura
// temporal. Si ESTO se cae, el culpable es el estado del monton de Go al llegar a pvr_init_defaults, y no
// el bicho ni el POC.
package main

import (
	"fmt"
	"runtime"
)

//extern d2c_pvr_pantalla_init
func d2cPvrPantallaInit(w, h int) int

//extern d2c_pvr_pantalla_pintar
func d2cPvrPantallaPintar()

func main() {
	fmt.Println("CHURN: arranca")

	// lo mismo que hace el POC con la imagen del mapa: 1 MB vivo, despues basura, y GCs a mano
	img := make([]byte, 1<<20)
	for i := 0; i < len(img); i += 64 {
		img[i] = byte(i)
	}
	fmt.Println("CHURN: 1 MB pedido, img[0] =", img[0])

	basura := make([][]byte, 0, 64)
	for i := 0; i < 40; i++ {
		basura = append(basura, make([]byte, 96<<10))
		if i%8 == 0 {
			runtime.GC()
		}
	}
	fmt.Println("CHURN: 40 temporales de 96 KB, len(basura) =", len(basura))

	basura = nil
	img = nil
	runtime.GC()
	runtime.GC()
	fmt.Println("CHURN: soltado y GC x2")

	r := d2cPvrPantallaInit(320, 240)
	fmt.Printf("CHURN: init = %d\n", r)
	for i := 0; i < 60; i++ {
		d2cPvrPantallaPintar()
	}
	fmt.Println("CHURN: 60 pintadas OK")
}

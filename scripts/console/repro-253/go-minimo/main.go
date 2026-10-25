// Reproduccion minima de DC-002.53, rung B: un programa de Go sobre libgodc que solo enciende el PVR
// a traves del mismo shim que usa el POC, y pinta 60 cuadros. Sin MPQ, sin nivel, sin personaje.
// Si ESTO se cae, el problema es del contexto Go/libgodc y no del codigo del POC.
package main

import "fmt"

//extern d2c_pvr_pantalla_init
func d2cPvrPantallaInit(w, h int) int

//extern d2c_pvr_pantalla_pintar
func d2cPvrPantallaPintar()

func main() {
	fmt.Println("MINIMO: arranca")
	r := d2cPvrPantallaInit(320, 240)
	fmt.Printf("MINIMO: init = %d\n", r)
	for i := 0; i < 60; i++ {
		d2cPvrPantallaPintar()
	}
	fmt.Println("MINIMO: 60 pintadas OK")
}

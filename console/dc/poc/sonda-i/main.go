//go:build dreamcast

// POC de la sonda-i: todo el trabajo lo hace el C (csrc/sonda.c), aca solo se llama y se mantiene el
// programa vivo.
package main

//extern d2c_sonda_i
func d2c_sonda_i()

//extern d2c_pantalla_init
func d2c_pantalla_init()

//extern d2c_pantalla_color
func d2c_pantalla_color(r, g, b float32)

//extern d2c_frame
func d2c_frame()

func main() {
	d2c_sonda_i()
	println("SONDA-I fin")
	d2c_pantalla_init()
	d2c_pantalla_color(0.1, 0.1, 0.4)
	for {
		d2c_frame()
	}
}

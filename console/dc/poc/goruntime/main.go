// DC-001.3 (paso 1): Go pelado, sin tocar PVR ni ninguna funcion de KOS.
// Si ESTO crashea, el problema esta en el runtime de libgodc/gccgo y no en nuestro codigo.
package main

func main() {
	println("GO-ARRIB A: runtime libgodc arranco")
	for i := 0; i < 3; i++ {
		println("GO-ARRIB B: iteracion", i)
	}
	println("GO-ARRIB C: entro al bucle infinito (sin llamadas a KOS)")
	x := 0
	for {
		x++
		if x < 0 {
			break
		}
	}
}

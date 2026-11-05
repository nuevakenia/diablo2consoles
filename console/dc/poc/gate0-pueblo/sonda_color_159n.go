//go:build dreamcast

package main

// DC-002.159n -- LA SONDA DEL COLOR EN EL GATE 0: el que pregunta al recolector donde esta el semiespacio vivo.
//
// POR QUE EXISTE. La corrida de cierre de DC-002.159m murio una vez de dos en `colorToColorM` (`Unhandled
// exception`, PC dentro de `.rodata`, PR justo despues del `jsr @r1` que despacha `color.Color.RGBA()`). La
// interfaz del color del estado de una superficie tenia una tabla de metodos que ya no era: o la superficie
// que llega al dibujo es una direccion vieja (el recolector la movio y esa copia no se actualizo), o las dos
// palabras se escribieron encima.
//
// QUE HACE. La parte de Go de la sonda vive en `d2core/d2render/ebiten/sonda_color_159n.go` (guarda las dos
// palabras al empujar el color y las compara al usarlo). Esta es la parte de la consola: le pregunta al
// recolector, por el shim de C (`d2c_semi_rango`), donde estan las dos mitades del semiespacio y donde esta el
// puntero de reparto, y con eso clasifica el parte en tres clases:
//
//	clase 1  la superficie NO cae en el semiespacio vivo: el que llama trae una direccion VIEJA (raiz que el
//	         recolector no vio, o un registro que no puede actualizar). Es la clase que mata: la primera cosa
//	         que se desreferencia de esa memoria prestada es la tabla de metodos del color.
//	clase 2  la superficie esta viva pero las palabras de adentro NO son las que se empujaron: alguien escribio
//	         encima del objeto (copia de mas por una cabecera falsa, escritura fuera de rango).
//	clase 3  la superficie no esta en el registro de las empujadas (color puesto sin `PushColor`, o una
//	         superficie que se creo con estado ya puesto). Se informa una vez por superficie, no por uso.
//
// Y CORTA. Devuelve `saltar` = true: el dibujo de ese color sale sin tinte (matriz identidad) en vez de saltar
// por una tabla de metodos que ya no es. Asi la corrida completa los 150 s y el numero que queda es CUANTAS
// VECES PASA, que es lo que la tarjeta pide medir.
//
// El log va por la consola serie: se informan los primeros `informes159n` eventos y despues uno de cada tanto,
// porque la consola serie es lenta y el cuadro ya se muere de callar (DC-002.87b).

import (
	renderEbiten "github.com/OpenDiablo2/OpenDiablo2/d2core/d2render/ebiten"
)

//extern d2c_semi_rango
func d2cSemiRango(v *uint32)

const informes159n = 60

var (
	eventos159n                [4]int // por clase: 0 = sano, 1 = fuera, 2 = cambiaron, 3 = sin registro
	usosSondados159n           int
	dicho159n                  int
	superficiesSinRegistro159n [64]uintptr
	cuantasSinRegistro159n     int
)

func init() {
	renderEbiten.SondaDeColor159n = sondaDeColor159n

	// DC-002.159n: la prueba de que este init corrio (y cuando). Sin esta linea, un "usos = 0" en el latido no
	// distingue "la sonda no se ejercio" de "nadie instalo el gancho".
	println("S159N-INSTALADA gancho=puesto")
}

func sondaDeColor159n(p renderEbiten.ParteDeLaSonda159n) bool {
	// DC-002.159n: la empujada con color es RARA (una o dos por corrida) y es la unica que puede terminar en el
	// despacho de `RGBA()` que murio en 159m. Se informa siempre, con la superficie y la cuenta de
	// recolecciones, para saber en que momento de la corrida aparece.
	if p.Empujado && p.Palabras[0] != 0 {
		puesto, llamadas, conColor, _, marcaRota := renderEbiten.EstadoDeLaSonda159n()

		println("S159N-EMPUJE sup=", int(p.Superficie), "itab=", int(p.Palabras[0]), "dato=", int(p.Palabras[1]),
			"marca=", p.Marca, "gancho=", puesto, "llamadas=", int(llamadas), "conColor=", int(conColor),
			"marcaRota=", int(marcaRota))
	}

	var v [7]uint32

	d2cSemiRango(&v[0])

	vivo := uintptr(v[0])   // comienzo del semiespacio ACTIVO
	tope := uintptr(v[1])   // puntero de reparto: lo vivo esta entre `vivo` y aca
	limite := uintptr(v[2]) // fin del semiespacio activo
	viejo := uintptr(v[3])  // la otra mitad (la que se reusa al recolectar)
	imagenHi := uintptr(v[4])
	recolecciones := int(v[5])
	tamano := uintptr(v[6])

	if p.Empujado {
		return false // una empujada no salta nada: define lo que vale
	}

	usosSondados159n++

	clase := 0
	fuera := p.Superficie < vivo || p.Superficie >= tope
	cambio := p.Cambiaron

	switch {
	case !p.Marca || (!p.Visto && fuera):
		// lo que hay en esa direccion ya no es una superficie (o la direccion es de la mitad que se reusa):
		// el que llama quedo con una direccion vieja
		clase = 1
	case cambio:
		clase = 2
	case !p.Visto:
		clase = 3
	}

	eventos159n[clase]++

	if clase == 0 {
		return false
	}

	if clase == 3 {
		// una superficie sin registro se informa UNA vez: el color pudo entrar por `createEbitenSurface` con
		// el estado ya puesto y eso no es el defecto que esta tarjeta persigue
		for i := 0; i < cuantasSinRegistro159n; i++ {
			if superficiesSinRegistro159n[i] == p.Superficie {
				return false
			}
		}

		if cuantasSinRegistro159n < len(superficiesSinRegistro159n) {
			superficiesSinRegistro159n[cuantasSinRegistro159n] = p.Superficie
			cuantasSinRegistro159n++
		}
	}

	if dicho159n < informes159n || eventos159n[clase]%50 == 0 {
		dicho159n++

		println("S159N clase=", clase, "n=", eventos159n[clase], "usos=", usosSondados159n,
			"sup=", int(p.Superficie), "vivo=", int(vivo), "tope=", int(tope), "limite=", int(limite),
			"viejo=", int(viejo), "imagenHi=", int(imagenHi), "gc=", recolecciones, "tamano=", int(tamano),
			"marca=", p.Marca, "fuera=", fuera, "visto=", p.Visto, "cambio=", cambio, "reg=", int(p.Registro),
			"itab=", int(p.Palabras[0]), "dato=", int(p.Palabras[1]),
			"antesItab=", int(p.Antes[0]), "antesDato=", int(p.Antes[1]),
			"totales=", eventos159n[0], "/", eventos159n[1], "/", eventos159n[2], "/", eventos159n[3])
	}

	return true
}

// CuentasDeLaSonda159n: (sanos, fuera, cambiaron, sin registro) para que el arranque pueda cerrar la corrida
// con los cuatro numeros en una linea.
func CuentasDeLaSonda159n() (sanos, fuera, cambiaron, sinRegistro int) {
	return eventos159n[0], eventos159n[1], eventos159n[2], eventos159n[3]
}

// LatidoDeLaSonda159n imprime, cada tanto, que la sonda esta VIVA y cuanto vio. Es el dato que separa "cero
// defectos" de "cero mediciones": `llamadas` y `conColor` los cuenta el lado del dibujo, que no depende del
// gancho; `gancho` dice si alguien lo instalo.
func LatidoDeLaSonda159n(cuadro int) {
	puesto, llamadas, conColor, sinColor, marcaRota := renderEbiten.EstadoDeLaSonda159n()
	sanos, fuera, cambiaron, sinRegistro := CuentasDeLaSonda159n()

	println("S159N-VIVA cuadro=", cuadro, "gancho=", puesto,
		"llamadas=", int(llamadas), "conColor=", int(conColor), "sinColor=", int(sinColor),
		"marcaRota=", int(marcaRota),
		"usos=", usosSondados159n, "sanos=", sanos, "fuera=", fuera,
		"cambiaron=", cambiaron, "sinRegistro=", sinRegistro)
}

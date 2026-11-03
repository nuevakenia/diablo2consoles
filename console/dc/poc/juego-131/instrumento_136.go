//go:build dreamcast

package main

// DC-002.136 -- LA TABLA DE CONSUMO DEL ARRANQUE, ETAPA POR ETAPA, Y EL AVISO DE LO QUE QUEDO APAGADO.
//
// QUE PROBLEMA RESUELVE. El POC muere cargando con `gc_external_alloc: OOM` (432 KB que no entran en el monton de C
// ni en lo que le queda al break contra el techo de KOS) y hasta esta tarjeta lo unico que habia eran los hitos
// sueltos de `memoria()`: `usado` en cada etapa, sin el delta y sin quien lo pidio. Con eso el arreglo se hace a
// ciegas. Esta tabla pone los dos numeros que faltaban:
//
//	delta   lo que la etapa le sumo al monton de C (negativo cuando devuelve)
//	libre   lo que quedaba DESPUES de la etapa: el margen real contra el OOM, que es un numero distinto del
//	        "usado" (el monton puede tener 1 MB libre y el pedido siguiente puede no entrar en ningun hueco)
//
// y el pico con nombre: la etapa donde el monton de C quedo mas alto y la etapa donde quedo con menos libre.
//
// QUIEN PIDE LOS BLOQUES GRANDES es la otra mitad y la contesta el shim de C (`csrc/shim.c`, seccion 12): interpone
// `malloc` y cada pedido de 64 KB o mas sale como `ARQ-MARCA ext-bloque v=<bytes> libreAntes=<n> ra=<direccion>`.
// La direccion de retorno se resuelve con `sh-elf-addr2line -e prog.elf <ra>`: el nombre de la funcion que pidio.
// Con el hito del POC delante de cada linea, el log queda como la tabla que pide la tarjeta: quien pide que, en
// orden, con el monton que habia en ese instante.
//
// EL INSTRUMENTO NO MIENTE: no reserva memoria (la tabla son ~30 entradas de 5 enteros y cadenas que ya existen) y
// no fuerza recolecciones (eso es `medirVivo`, de DC-002.131, que si cambia el estado del monton). Se apaga con
// MEDIR_CARGA=no y el binario queda como estaba.

// hitoDeCarga: una fila de la tabla.
type hitoDeCarga struct {
	Etapa string
	Cused int // el monton de C tomado al cerrar la etapa
	Libre int // lo que quedaba libre adentro del arena
	Vivo  int // el semiespacio del recolector: lo ASIGNADO (basura incluida)
	Delta int // lo que esta etapa le sumo al monton de C (negativo = devolvio)
}

var hitosDeCarga []hitoDeCarga

// picoDeLaCarga: el hito donde el monton de C quedo mas alto, y el que lo dejo con menos libre.
var (
	picoDeLaCarga   int
	picoEnLaEtapa   = "ninguna"
	menosLibre      int
	menosLibreEtapa = "ninguna"
)

// anotarLaCarga: una fila mas de la tabla. La llama `memoria()`, asi que los hitos son los MISMOS de toda la vida
// (al-arrancar, despues-del-video, jugador-antes/despues, despues-de-componer, bicho, quest, npcs, minimapa, hud,
// antes-del-renderer, ...). No fuerza nada: lee `mallinfo` y `gc_stats` y nada mas.
func anotarLaCarga(etapa string, usado int) {
	if medirCarga != "si" {
		return
	}

	libre := d2cMontonLibre()
	vivo := int(d2cGcUsado())
	delta := usado

	if n := len(hitosDeCarga); n > 0 {
		delta = usado - hitosDeCarga[n-1].Cused
	}

	hitosDeCarga = append(hitosDeCarga, hitoDeCarga{Etapa: etapa, Cused: usado, Libre: libre, Vivo: vivo, Delta: delta})

	if usado > picoDeLaCarga || len(hitosDeCarga) == 1 {
		picoDeLaCarga = usado
		picoEnLaEtapa = etapa
	}

	if len(hitosDeCarga) == 1 || libre < menosLibre {
		menosLibre = libre
		menosLibreEtapa = etapa
	}

	println("JUEGO136: CARGA hito=", len(hitosDeCarga), "etapa=", etapa, "cused=", usado, "libre=", libre,
		"delta=", delta, "vivo=", vivo)
}

// tablaDeLaCarga: el cierre del instrumento. Imprime el pico, el margen mas bajo, cuanto pidio y cuanto devolvio el
// arranque, y las OCHO etapas que mas pidieron, de mayor a menor (seleccion por barrido: la stdlib de la consola no
// trae `sort` y para ocho filas no hace falta).
func tablaDeLaCarga() {
	if medirCarga != "si" || len(hitosDeCarga) == 0 {
		return
	}

	pedido, devuelto := 0, 0

	for _, h := range hitosDeCarga {
		if h.Delta > 0 {
			pedido += h.Delta
		} else {
			devuelto -= h.Delta
		}
	}

	println("JUEGO136: TABLA-CARGA hitos=", len(hitosDeCarga), "pico=", picoDeLaCarga, "en=", picoEnLaEtapa,
		"menosLibre=", menosLibre, "en=", menosLibreEtapa, "pedidoTotal=", pedido, "devueltoTotal=", devuelto,
		"semiespacio=", int(d2cGcTotal()))

	const cuantas = 8

	usadas := make([]bool, len(hitosDeCarga))

	for i := 0; i < cuantas && i < len(hitosDeCarga); i++ {
		mejor, mejorDelta := -1, 0

		for j, h := range hitosDeCarga {
			if usadas[j] {
				continue
			}

			if mejor < 0 || h.Delta > mejorDelta {
				mejor, mejorDelta = j, h.Delta
			}
		}

		if mejor < 0 {
			break
		}

		usadas[mejor] = true

		h := hitosDeCarga[mejor]
		println("JUEGO136: TABLA-CARGA", i+1, "etapa=", h.Etapa, "delta=", h.Delta, "cused=", h.Cused, "libre=", h.Libre)
	}
}

// avisarLoQueQuedaApagado: la linea `ARQ-MARCA` que pide la tarjeta. Una feature apagada por palanca NO puede
// desaparecer en silencio: al arrancar se dice cual quedo apagada, con que palanca y que deja de cargarse, y el
// cierre cuenta encendidas y apagadas. Con los knobs por defecto hay UNA sola apagada (el split, que viene en "no"
// por omision); con NPC=no, QUEST=no y compania aparece una linea por cada una, asi que el A/B barato de
// DC-002.132 ya no es un silencio.
func avisarLoQueQuedaApagado() {
	apagadas, encendidas := 0, 0

	avisa := func(nombre, valor, queNoSeCarga string) {
		if valor == "si" || valor == "encendido" {
			encendidas++

			return
		}

		apagadas++
		println("ARQ-MARCA feature-apagada v=1 nombre=", nombre, "palanca=", valor, "noSeCarga=", queNoSeCarga)
	}

	avisa("npc", npcKnob, "cof+dcc de los NPCs, panel de dialogo y string.tbl")
	avisa("quest", questKnob, "el juego de la Den of Evil y sus textos")
	avisa("drop", dropKnob, "tablas del drop y el DC6 del objeto en el piso")
	avisa("inventario", inventarioKnob, "la grilla del inventario y el invfile del objeto")
	avisa("alijo", alijoKnob, "la grilla del alijo")
	avisa("hud", hudKnob, "globos, cinturon, pociones y el nombre de la zona")
	avisa("minimapa", minimapaKnob, "maximap.dc6 y el modelo del automapa")
	avisa("combate", combateKnob, "la vida del bicho y el golpe")
	avisa("jugador", jugadorKnob, "la amazona entera (COF, .dcc y los 8 ciclos)")

	if monstruosKnob == 0 {
		apagadas++
		println("ARQ-MARCA feature-apagada v=1 nombre=monstruos palanca=0 noSeCarga=el .cof y las .dcc del bicho")
	} else {
		encendidas++
	}

	if splitKnob != "si" {
		apagadas++
		println("ARQ-MARCA feature-apagada v=1 nombre=split palanca=", splitKnob,
			"noSeCarga=el segundo lienzo de 320x480 (la pantalla partida es SPLIT=si)")
	} else {
		encendidas++
	}

	println("ARQ-MARCA features v=", encendidas+apagadas, "encendidas=", encendidas, "apagadas=", apagadas,
		"por=palanca-del-build (el detalle, una linea arriba por cada apagada)")
}

// DC-002.67 PASO 2 y 3: el MOTOR ENLAZADO en un binario de Dreamcast.
//
// Este programa no dibuja ni carga un asset: llama funciones PURAS del motor compilado para SH-4
// (d2common/d2act3, que a su vez usa d2common/d2quest y d2common/d2level) e imprime el resultado por la
// consola serie. Sirve para dos cosas a la vez:
//
//   - En la Dreamcast (Flycast): prueba que los .gox de DC-002.66 ENLAZAN con KOS + libgodc y que el
//     codigo del motor CORRE en el SH-4.
//   - En el host (go run ./console/dc/poc/enlace-267): da el resultado de referencia con el Go de verdad.
//     Las dos salidas se comparan linea por linea; que coincidan es la prueba de que lo que corre en la
//     consola es el mismo motor, no una copia parecida.
//
// NO lleva //go:build dreamcast a proposito: el mismo archivo tiene que compilar en los dos lados.
package main

import (
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2act3"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2quest"
)

// Las dos funciones de "que tiene el heroe" que pide FlailDrop. Son funciones con nombre y no closures
// porque el camino que se quiere medir es el del motor, no el de las funciones anonimas de gccgo.
func nadie(_ string) bool { return false }
func todos(_ string) bool { return true }

// alTerminar es lo ultimo que hace main. En la consola, cuando main vuelve, libgodc llama al arch_exit de
// KOS y el emulador SIN BIOS cae en "Fatal: SH4 exception when blocked" medio segundo despues: el binario
// se muere solo al final del informe. serial_dc.go (que solo compila gccgo) la reemplaza por un bucle que
// mantiene el programa vivo; en el host es un no-op y `go run` termina normalmente.
var alTerminar = func() {}

func main() {
	println("ENLACE267: INICIO d2act3 sobre el motor enlazado")

	objetos := d2act3.Objects()
	println("ENLACE267: objetos=", len(objetos))

	o, hay := d2act3.Object(d2act3.ObjChestHeart)
	println("ENLACE267: objeto-405=", o.Name, "item=", o.Item, "nivel=", o.Level, "hay=", hay)

	println("ENLACE267: orb-con-will=", d2act3.CanSmashOrb(d2quest.ItemKhalimWill),
		"orb-con-flail=", d2act3.CanSmashOrb(d2quest.ItemKhalimFlail))

	consejo := d2act3.CouncilOfTravincal()
	println("ENLACE267: consejo=", len(consejo), "primero=", consejo[0])

	// EqualFold del paquete strings de la stdlib minima: una funcion del motor que compara texto.
	println("ENLACE267: es-consejo-minusculas=", d2act3.IsTravincalCouncil("geleb flamefinger"),
		"es-consejo-otro=", d2act3.IsTravincalCouncil("Bishibosh"))

	println("ENLACE267: kill-clase-seguidor=", d2act3.QuestKillClass(d2quest.NPCCouncilA, false),
		"kill-clase-jefe=", d2act3.QuestKillClass(d2quest.NPCCouncilA, true))

	item, si := d2act3.FlailDrop("Ismail Vilehand", nadie)
	println("ENLACE267: flail-sin-tenerlo=", item, si)

	item2, si2 := d2act3.FlailDrop("Ismail Vilehand", todos)
	println("ENLACE267: flail-ya-tiene=", item2, si2)

	println("ENLACE267: hellgate-con-mephisto-muerto=", d2act3.HellgateOpen(true),
		"sin-mephisto=", d2act3.HellgateOpen(false))

	err := d2act3.CheckSewerStairs(d2act3.LevelSewers1, d2act3.LevelSewers2, false)
	println("ENLACE267: escaleras-sin-palanca-hay-error=", err != nil)

	if err != nil {
		println("ENLACE267: escaleras-sin-palanca-error=", err.Error())
	}

	err2 := d2act3.CheckSewerStairs(d2act3.LevelSewers1, d2act3.LevelSewers2, true)
	println("ENLACE267: escaleras-con-palanca-hay-error=", err2 != nil)

	println("ENLACE267: FIN")

	alTerminar()
}

package d2gamescreen

// DC-002.156 -- EL PREFIJO DE LOG DE LAS PANTALLAS, EN UN ARCHIVO QUE EL PUEBLO SI COMPILA.
//
// POR QUE EXISTE. El prefijo vive en `main_menu.go` (`logPrefix = "Game Screen"`) porque ahi nacio el
// paquete. Pero el build de la consola del Gate 0 NO compila `main_menu.go`: el menu, la seleccion de
// personaje, el HUD, el inventario y los creditos son 1.213.247 B de text+data de los que el pueblo usa
// ~60 KB, y dejarlos fuera es la mitad del recorte de DC-002.156. Con `main_menu.go` fuera, la pantalla del
// pueblo se quedaba sin la constante que pide `Logger.SetPrefix`.
//
// POR QUE NO SE MUEVE A `doc.go`: porque `main_menu.go` la sigue usando y dos definiciones no pueden
// convivir en el mismo paquete. Este archivo es una constante NUEVA, con nombre propio, que la pantalla del
// pueblo usa; el menu sigue con la suya y el binario de la PC queda igual (los dos archivos estan).

const (
	// prefijoDelLog: el mismo texto que `logPrefix`, con nombre propio para que la pantalla del pueblo no
	// dependa de un archivo que la consola no compila.
	prefijoDelLog = "Game Screen"
)

package d2asset

import (
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2enum"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2interface"
)

// SondaDeRender es el gancho de medicion de `Composite.Render` (DC-002.84).
//
// POR QUE EXISTE. En la consola, el PRIMER cuadro que pinta un `Composite` con sus seis capas .dcc
// cargadas mata el SH-4 con un salto a basura (`PC 3fd17b4f`, `PC == PR`, R3 apuntando a VRAM), y para
// saber CUAL de las llamadas por interfaz de `Render` es la que salta hay que ver, en el cuadro que
// muere, la direccion, el cuadro de animacion, el indice de capa y el valor de la interfaz de cada capa.
// Eso es lo que entrega este gancho: se llama UNA vez por capa, justo ANTES de `layer.RenderFromOrigin`.
//
// COSTO. En la PC queda en nil y solo cuesta una comparacion por capa y por cuadro; el POC de la consola
// lo pone y lo apaga el mismo cuando ya tiene la foto. No cambia ni una decision de `Render`.
var SondaDeRender func(direccion, cuadro, direccionCof int, indice d2enum.CompositeType,
	capas []d2interface.Animation, capa d2interface.Animation, sombra bool)

//go:build dreamcast

package pvr

// LA CUENTA DEL LIENZO (DC-002.85): cuantas superficies se crearon, cuanta memoria de lienzo pidieron y por
// cual de los dos caminos entro cada cuadro de animacion.
//
// POR QUE. La tarjeta saca el buffer intermedio de la expansion de indices (un buffer de 4 x pixeles por
// cuadro, que se copiaba al lienzo y se tiraba). Para que "un buffer por cuadro en vez de dos" sea un numero
// y no una lectura del codigo, el backend cuenta:
//
//	superficies / bytesDeLienzo   lo que se pidio al monton para lienzos (NewSurface -> NuevaImagen).
//	directas / bytesDirectos      cuadros expandidos DIRECTO en el lienzo (Surface.ExpandirIndices).
//	copias / bytesCopiados        cuadros que llegaron como buffer ajeno y se copiaron (Surface.ReplacePixels):
//	                              ese es el camino viejo, con el intermedio vivo.
//
// Los contadores viven en el Renderer (no en cada Surface) y se incrementan del que dibuja; el ciclo de
// cuadros de la consola es de un hilo, y las capas de un `Composite` se crean en goroutines solo si el COF
// pide SetDirection (el POC cooperativo no lo hace: ver participante.go). Cuando se lea este numero desde un
// camino concurrente, el numero es de ORDEN, no exacto.

// CuentaDelLienzo es la foto de esos contadores.
type CuentaDelLienzo struct {
	Superficies   int
	BytesDeLienzo int
	Directas      int
	BytesDirectos int
	Copias        int
	BytesCopiados int
}

// CuentaDeLienzo devuelve la foto de los contadores del lienzo.
func (r *Renderer) CuentaDeLienzo() CuentaDelLienzo {
	return CuentaDelLienzo{
		Superficies:   r.lienzosCreados,
		BytesDeLienzo: r.bytesDeLienzos,
		Directas:      r.expansionesDirectas,
		BytesDirectos: r.bytesExpandidos,
		Copias:        r.copiasDeBuffer,
		BytesCopiados: r.bytesCopiados,
	}
}

// ============================================================ LOS PEDIDOS DE BUFFER (DC-002.97)

// Pedidos es la cuenta de buffers de PIXELES que pidio este backend en toda su vida. Son TRES los sitios
// donde nace uno, y cada uno tiene su campo:
//
//	Lienzos   una superficie nueva (`NuevaSurface` -> el lienzo donde se pinta una capa o el cuadro)
//	Recortes  la seccion de otra superficie (`RenderSection` -> `recortar`, un buffer del tamano del recorte)
//	Rellenos  la textura de potencia de dos de un sprite (`rellenarIndices`, camino de hardware)
//
// POR QUE UN CONTADOR Y NO UNA IMPRESION. El criterio de la tarjeta es "cero pedidos de buffer por cuadro
// en regimen": con esto el POC toma la foto al empezar y al terminar cada cuadro y la diferencia es lo que
// se pidio EN ESE CUADRO, sin leer el codigo y sin creerle a un comentario. Lo que no nace en estos tres
// sitios es porque no es un buffer de pixeles (un estado, una copia a un lienzo que ya existe, la VRAM).
//
// VIVE EN EL PAQUETE Y NO EN EL Renderer: `recortar` y `rellenarIndices` se llaman tambien desde imagenes
// que no tienen renderer (una superficie armada a mano, una prueba de humo), y si el contador viviera en el
// Renderer esos pedidos no se contarian. El ciclo de cuadros de la consola es de un hilo.
type Pedidos struct {
	Lienzos       int
	BytesLienzos  int
	Recortes      int
	BytesRecortes int
	Rellenos      int
	BytesRellenos int
}

// Suma: el total de buffers pedidos (los tres sitios) y sus bytes. Es lo que se compara contra cero.
func (p Pedidos) Suma() int  { return p.Lienzos + p.Recortes + p.Rellenos }
func (p Pedidos) Bytes() int { return p.BytesLienzos + p.BytesRecortes + p.BytesRellenos }

var pedidosDeBuffer Pedidos

// PedidosDeBuffer devuelve la foto acumulada de los pedidos. La diferencia entre dos fotos es lo que se
// pidio entre las dos (el POC la usa por cuadro).
func PedidosDeBuffer() Pedidos { return pedidosDeBuffer }

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

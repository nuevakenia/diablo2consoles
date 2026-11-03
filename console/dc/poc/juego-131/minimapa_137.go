//go:build dreamcast

package main

// DC-002.137 -- EL MINIMAPA POR DENTRO: LA CAJA, LAS CELDAS Y LA TABLA, CADA UNA CON SU CRONOMETRO.
//
// QUE PROBLEMA RESUELVE. En DC-002.134 el minimapa quedo medido por fuera: 66,5 ms por cuadro (61,6 ms hoy, con
// el mapa ya arreglado en DC-002.135) y la unica pista era la cuenta. La cuenta decia que las 610 celdas que caen
// en la caja pintan **4.968 pixeles** por cuadro -- las del automapa son lineas finas de 8x16, ~8 pixeles cada
// una -- asi que los milisegundos NO estan en los pixeles. Lo que quedaba por saber era en QUE se iban, y eso no
// se adivina: se mide. Este archivo es el cronometro por dentro del minimapa, con el mismo reloj del backend
// (`d2c_us`, `timer_us_gettime64`):
//
//	JUEGO137: MINIMAPA-POR-DENTRO cuadros= delTramo= caja= celdas= marcador= tabla= extra=
//	          celdasPintadas= celdasDelModelo= rapido= diag=
//
//	caja      -- el rectangulo negro de la caja (279x225 = 62.775 px) por `DrawRect`
//	celdas    -- el bucle de las celdas del modelo (con la tabla adentro si `rapido=no`, que es el camino viejo)
//	marcador  -- la cruz del heroe (12 tramos de linea)
//	tabla     -- armar la tabla de 256 palabras UNA vez (solo con `rapido=si`: es lo que el camino viejo pagaba
//	             610 veces por cuadro)
//	extra     -- la corrida de medir `MINIMAPA_DIAG=tabla610` (armar la tabla 610 veces mas, sin cambiar el
//	             cuadro): el costo de la tabla por celda del camino viejo, medido de la otra punta
//
// LAS DOS PALANCAS (knobs.go):
//
//	minimapaRapido = "si" | "no"   la tabla una vez por cuadro y sin Push/Pop por celda, contra el camino de
//	                               siempre (una tabla por celda). El A/B es una corrida y los pixeles son los
//	                               mismos: lo prueba el `crc32` de la caja del minimapa.
//	minimapaDiag   = "no" | "caja" | "celdas" | "tabla610"
//	                               las corridas de medir. `caja` y `celdas` ROMPEN el dibujo a proposito (son
//	                               para medir, no para el juego); `tabla610` deja el cuadro IDENTICO y sirve de
//	                               control: si el cuadro cambia con `tabla610`, el instrumento esta tocando algo
//	                               que no debe.

import (
	"hash/crc32"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2automap"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2render/pvr"
)

// tablaDelMinimapa: la tabla de 256 palabras de las celdas, armada UNA vez por cuadro por el POC (no por el
// backend: la leccion de DC-002.134 es que el backend no guarda estado del dibujado).
//
// ES UNA VARIABLE DEL PAQUETE Y NO UNA LOCAL. Una [256]uint32 son 1 KB: en el monton seria 1 KB de datos VIVOS
// que el recolector tiene que copiar (el margen medido en DC-002.134 es NULO: el POC muere con `GC to-space
// overflow` cuando el dibujado guarda 1 KB) y en la pila seria 1 KB de marco en cada llamada, con
// `-fno-split-stack` (el marco no crece solo). Una variable del paquete vive en el `.bss` del binario: no la toca
// el recolector y no cuesta un byte del semiespacio. Se arma cada cuadro (256 cuentas, decenas de us) para no
// dejar nada cacheado entre cuadros; lo que se evita es armarla 610 veces EN el mismo cuadro.
var tablaDelMinimapa pvr.TablaDelBlit

// Los cronometros del minimapa por dentro (microsegundos acumulados del tramo, promedio por cuadro al informar).
var (
	usMiniCaja137     int64
	usMiniCeldas137   int64
	usMiniMarcador137 int64
	usMiniTabla137    int64
	usMiniExtra137    int64
	cuadrosMini137    int
	celdasMini137     int
	pixelesMini137    int
)

// anotarLaTabla137 / anotarLaCaja137 / anotarLasCeldas137 / anotarElMarcador137 / anotarLaTablaDeMas137: cierran
// un cronometro abierto con `cronometro134`. Con `etapasKnob=no` no suman nada (el binario es el de antes).
func anotarLaTabla137(desde int64)      { sumarDelMini137(&usMiniTabla137, desde) }
func anotarLaCaja137(desde int64)       { sumarDelMini137(&usMiniCaja137, desde) }
func anotarLasCeldas137(desde int64)    { sumarDelMini137(&usMiniCeldas137, desde) }
func anotarElMarcador137(desde int64)   { sumarDelMini137(&usMiniMarcador137, desde) }
func anotarLaTablaDeMas137(desde int64) { sumarDelMini137(&usMiniExtra137, desde) }

func sumarDelMini137(donde *int64, desde int64) {
	if etapasKnob != "si" || desde <= 0 {
		return
	}

	us := int64(d2cUs()) - desde
	if us < 0 { // el contador de 32 bits da la vuelta cada ~71 minutos: no se resta un negativo
		return
	}

	*donde += us
}

// informeDelMini137: la linea del minimapa por dentro, cada `cuadrosDelInforme132` cuadros (la llama el que cierra
// el cuadro del minimapa, `dibujarElMinimapa`).
func informeDelMini137(pixelesPintados, celdasPintadas, celdasDelModelo int) {
	cuadrosMini137++
	celdasMini137 += celdasPintadas
	pixelesMini137 += pixelesPintados

	if cuadro%cuadrosDelInforme132 != 0 || cuadrosMini137 == 0 {
		return
	}

	n := int64(cuadrosMini137)
	palabras137, bytes137, compactos137 := pvr.ContadoresDeLaTabla137()

	println("JUEGO137: MINIMAPA-POR-DENTRO cuadros=", cuadro, "delTramo=", cuadrosMini137,
		"caja=", usMiniCaja137/n, "celdas=", usMiniCeldas137/n, "marcador=", usMiniMarcador137/n,
		"tabla=", usMiniTabla137/n, "extra=", usMiniExtra137/n,
		"celdasPintadas=", celdasMini137/int(cuadrosMini137), "pixelesPintados=", pixelesMini137/int(cuadrosMini137),
		"celdasDelModelo=", celdasDelModelo, "rapido=", minimapaRapido, "diag=", minimapaDiag,
		"tilesPorPalabras=", palabras137, "tilesPorBytes=", bytes137, "tilesCompactos=", compactos137)

	usMiniCaja137, usMiniCeldas137, usMiniMarcador137, usMiniTabla137, usMiniExtra137 = 0, 0, 0, 0, 0
	cuadrosMini137, celdasMini137, pixelesMini137 = 0, 0, 0
	pvr.ReiniciarContadoresDeLaTabla137()
}

// pintarLasCeldasConLaTabla: la MISMA vuelta que `pintarLasCeldasDeSiempre` (el mismo orden del modelo, el mismo
// recorte contra la caja, la misma esquina de abajo a la izquierda), con tres cosas que se pagan una vez por cuadro
// en vez de una vez por celda:
//
//  1. la TABLA de 256 palabras (`tablaDelMinimapa`, armada por el que llama): el camino de siempre la armaba
//     DENTRO de cada `RenderIndicesTintado`, o sea 610 veces;
//  2. la posicion: `RenderIndicesConTabla` la recibe por parametro, asi que no hay un `PushTranslation`/`Pop` por
//     celda (la pila de estados es un `append`+`slice` en cada uno).
//
// LOS PIXELES SON LOS MISMOS, y no por confianza: el camino nuevo hace la misma cuenta del camino de bytes
// (indice 0 no pinta, alfa 0 no pinta, alfa 255 escribe, alfa intermedio es source-over con el mismo cociente por
// 255), el verificador del host `console/dc/verif-blit-137` compara los dos caminos byte a byte, y en la consola
// el `crc32` de la caja sale identico con `MINIMAPA_RAPIDO=si` y con `=no`.
func (m *minimapaDelJuego) pintarLasCeldasConLaTabla(sup *pvr.Surface, lay d2automap.Layout, compacto bool) (int, int) {
	pintados, celdas := 0, 0

	// DC-002.137: para las corridas de medir que parten el costo del bucle (`MINIMAPA_DIAG`, knobs.go). Las palancas
	// se leen UNA vez por cuadro (no una por celda: 610 comparaciones de cadena por cuadro no son gratis).
	sinArte := minimapaDiag == "sinarte"
	sinBlit := minimapaDiag == "sinblit"
	arteFijo := arteFijo137(m)

	for _, c := range m.Modelo.Cells() {
		var arte *celdaDelMinimapa

		if sinArte {
			// corrida de MEDIR: sin la busqueda del cuadro en el mapa (dibuja otra cosa, el cuadro no se compara)
			arte = arteFijo
		} else {
			arte = m.Arte[c.Cel]
		}

		if arte == nil {
			m.SinArte++

			continue
		}

		x, y := lay.ToScreen(c.X, c.Y)

		// la caja de la celda contra la caja del minimapa (ancla abajo a la izquierda)
		if x+arte.Ancho <= lay.Clip.X0 || x >= lay.Clip.X1 || y <= lay.Clip.Y0 || y-arte.Alto >= lay.Clip.Y1 {
			continue
		}

		// corrida de MEDIR: todo el bucle menos el pegado de la celda
		if sinBlit {
			pintados += arte.Pixeles
			celdas++

			continue
		}

		pegada := false

		if compacto && len(arte.Compacto) >= 3 {
			// la lista de los pixeles que pintan (armada una vez al cargar el arte): ~8 vueltas por celda en vez de
			// los 128 indices del tile
			pegada = sup.RenderCompactoConTabla(arte.Compacto, arte.Ancho, arte.Alto, x, y-arte.Alto, &tablaDelMinimapa)
		} else {
			pegada = sup.RenderIndicesConTabla(arte.Indices, arte.Ancho, arte.Alto, x, y-arte.Alto, &tablaDelMinimapa)
		}

		if !pegada {
			// el lienzo no se puede escribir por palabras (buffer desalineado): el camino de siempre, con la misma
			// paleta. No deberia pasar (la VRAM y el monton de C dan bloques alineados) y el aviso se cuenta en el
			// backend (`pvr.AvisosDeAlineacion`).
			sup.PushTranslation(x, y-arte.Alto)
			sup.RenderIndicesTintado(arte.Indices, arte.Ancho, arte.Alto, m.Pal[:])
			sup.Pop()
		}

		pintados += arte.Pixeles
		celdas++
	}

	return pintados, celdas
}

// arteFijo137: un cuadro de arte cualquiera, tomado una vez, para la corrida de medir `MINIMAPA_DIAG=sinarte` (mide
// el bucle SIN la busqueda en el mapa: la diferencia contra el camino normal es lo que cuesta la busqueda).
func arteFijo137(m *minimapaDelJuego) *celdaDelMinimapa {
	for _, a := range m.Arte {
		if a != nil {
			return a
		}
	}

	return nil
}

// armarLaTablaDeMas137: la corrida de MEDIR `MINIMAPA_DIAG=tabla610`. Arma la MISMA tabla `celdas` veces mas
// (610 en el caso peor del pueblo revelado entero) sin cambiar un pixel del cuadro: es exactamente el trabajo que
// el camino viejo pagaba por la tabla, medido desde el otro lado. El resultado se suma en un sumidero para que el
// compilador no se lleve puesto el bucle.
func armarLaTablaDeMas137(sup *pvr.Surface, m *minimapaDelJuego, celdas int) {
	for i := 0; i < celdas; i++ {
		sup.ArmarTablaDelBlit(m.Pal[:], &tablaDelMinimapa)
	}

	sumidero137 = tablaDelMinimapa.Color[1]
}

// sumidero137: donde va a parar lo que la corrida de medir armo (para que el bucle no se optimice).
var sumidero137 uint32

// crcDeLaCaja137: el crc32 de la caja del minimapa (279x225 px = 251.100 B) DESPUES de dibujarla. Es la prueba de
// que el camino rapido y el de siempre dejan la MISMA caja: los dos numeros se comparan entre dos corridas del
// mismo binario (una con `MINIMAPA_RAPIDO=si` y otra con `no`), asi que no hace falta comparar dos arboles.
//
// Va por filas porque la caja es un rectangulo del lienzo y el crc32 del motor (`crcLienzo`) tiene que ser uno
// solo: se encadena la tabla (`crc32.Update`) fila por fila, que es la misma cuenta que un tramo contiguo.
func crcDeLaCaja137(sup *pvr.Surface, lay d2automap.Layout) uint32 {
	if sup == nil || sup.Imagen() == nil {
		return 0
	}

	pixeles := sup.Imagen().Pixeles()
	anchoLienzo, altoLienzo := sup.GetSize()

	x0, x1 := lay.Clip.X0, lay.Clip.X1
	y0, y1 := lay.Clip.Y0, lay.Clip.Y1

	if x0 < 0 {
		x0 = 0
	}

	if y0 < 0 {
		y0 = 0
	}

	if x1 > anchoLienzo {
		x1 = anchoLienzo
	}

	if y1 > altoLienzo {
		y1 = altoLienzo
	}

	if x0 >= x1 || y0 >= y1 {
		return 0
	}

	var crc uint32

	// OJO CON LA STDLIB DE LA CONSOLA: este `hash/crc32` (el portado para gccgo) NO tiene `Update` ni
	// `IEEETable` exportados. Tiene `NewIEEE()` (un `hash.Hash32`) y `ChecksumIEEE`, asi que la cadena por filas
	// se hace con el `Write` del sumador. Se crea uno por cuadro y solo con `VERBOSO=si` (esto es evidencia).
	sumador := crc32.NewIEEE()

	for fy := y0; fy < y1; fy++ {
		j := (fy*anchoLienzo + x0) * 4
		sumador.Write(pixeles[j : j+(x1-x0)*4])
	}

	crc = sumador.Sum32()

	return crc
}

// compactarElArte: la lista COMPACTA de un cuadro de arte -- solo los pixeles que pintan, en tercetos
// (x, y, indice) -- en el monton de C, como el arte. Devuelve nil si el cuadro no pinta nada.
//
// POR QUE EN EL MONTON DE C Y NO EN EL SEMIESPACIO: la lista no la toca el recolector (no la cuenta ni la copia),
// y son ~24 B por cuadro de arte (8 pixeles x 3 B) contra los 128 B del tile: 59 cuadros son ~1,4 KB, de los
// 1,29 MB libres del monton de C que la corrida ya reporta. En el semiespacio serian 1,4 KB de datos VIVOS que
// el POC no tiene de donde sacar (DC-002.134: el margen medido es nulo).
//
// LA x Y LA y VAN EXPLICITAS para no tener que dividir por el ancho del tile en el cuadro: el terceto es lo que
// el backend recorre tal cual (`pvr.RenderCompactoConTabla`).
func (m *minimapaDelJuego) compactarElArte(indices []byte) []byte {
	if len(indices) < anchoCeldaMini*altoCeldaMini {
		return nil
	}

	pintan := contarNoVacios(indices[:anchoCeldaMini*altoCeldaMini])
	if pintan == 0 {
		return nil
	}

	compacto := bloqueDeC(pintan * 3)
	if compacto == nil {
		return nil
	}

	k := 0

	for y := 0; y < altoCeldaMini; y++ {
		for x := 0; x < anchoCeldaMini; x++ {
			idx := indices[y*anchoCeldaMini+x]
			if idx == 0 {
				continue
			}

			compacto[k], compacto[k+1], compacto[k+2] = byte(x), byte(y), idx
			k += 3
		}
	}

	return compacto
}

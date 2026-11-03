//go:build dreamcast

package main

// DC-002.135 -- EL MAPA POR TILE: EL ARRANQUE Y EL INFORME DEL CRONOMETRO DE ADENTRO DEL RENDERER.
//
// QUE HACE ESTE ARCHIVO, QUE ES POCO Y ES A PROPOSITO. La instrumentacion vive en el motor
// (`d2core/d2map/d2maprenderer/cronometro_135.go` y las marcas de `renderer.go`), porque el que sabe cuando
// empieza y cuando termina cada bloque del tile es el renderer, no el POC. Aca solo se le enchufa el reloj de
// la consola (`d2c_us`, el mismo de la presentacion del PVR) y se imprime el censo cada 30 cuadros, con la
// forma de siempre:
//
//	JUEGO135M: MAPA-POR-TILE cuadros= delTramo= tiles= imagenesNuevas= cache= vista= estado= pintado=
//	           indice= fondos= p1= p2= p3= p4= sumaBloques= sumaPasos= sinTile= instrumento=
//
//	tiles          -- tiles pintados por cuadro (los mismos que suma `DrawStats`; control de la cuenta)
//	imagenesNuevas -- imagenes de la cache COMPUESTAS en el tramo: si la cache se rearmara por cuadro, este
//	                  numero seria ~242 x los cuadros del tramo (hipotesis (a) de la tarjeta, contestada aca)
//	cache          -- microsegundos por cuadro del bloque de cache del tile (asegurarImagen + buscar)
//	vista          -- idem de la cuenta de la posicion en pantalla (aritmetica del viewport)
//	estado         -- idem del estado del Surface (`PushTranslation` del tile y el `Pop`)
//	pintado        -- idem del `RenderIndicesTintado` del tile (el bucle de pixeles y la tabla)
//	indice/fondos/p1..p4 -- las pasadas del renderer (`indexEntities`, `renderBackdrops`, pass1..4)
//	sinTile        -- lo que las pasadas pagan FUERA de los cuatro bloques del tile: `mapa - (sumaBloques +
//	                  indice + fondos)`. Si es grande, el peso no esta en el tile sino en los bucles de las
//	                  pasadas (`TileAt`, `PushTranslationWorld`).
//	instrumento    -- lo que cuesta el cronometro mismo: 8 marcas por tile x tiles x el precio de una marca,
//	                  medido al arrancar (`RELOJ costo_de_1000_llamadas_us`). Se resta antes de comparar.
//
// LA PALANCA (`knobs.go`, la pisa el build con `TILE_CRON=si`):
//
//	tileCron = "si" | "no"  -- "no" deja el reloj sin poner y el motor no cuenta nada (el binario es el de
//	                           DC-002.134 otra vez); "si" enchufa `d2c_us` y saca la linea cada 30 cuadros.

import (
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2map/d2maprenderer"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2render/pvr"
)

// relojDelPoc135: el reloj de la consola, con la firma que pide el motor (`func() uint`).
func relojDelPoc135() uint { return d2cUs() }

// blitsPlanos135: los blits por el camino plano vistos en el informe anterior (para el delta del tramo).
var blitsPlanos135 int

// marcasPorTile135: cuantas veces por tile llama al reloj el instrumento (cuatro bloques x ida y vuelta).
const marcasPorTile135 = 8

// costoDeUnaMarca135: microsegundos que cuesta UNA llamada a `d2cUs`, medido al arrancar.
var costoDeUnaMarca135 int64

// colecciones135: las recolecciones vistas en el informe anterior, para saber cuantas cayeron EN el tramo.
var colecciones135 uint

// activarElCronometroDeLosTiles: enchufa (o no) el reloj del motor y deja constancia de con que corrio este
// binario. Se llama ANTES del primer cuadro, como las demas palancas.
func activarElCronometroDeLosTiles() {
	// el costo del reloj, medido y no supuesto: 1.000 llamadas seguidas.
	t0 := d2cUs()

	for i := 0; i < 1000; i++ {
		d2cUs()
	}

	costoDeUnaMarca135 = int64(d2cUs()-t0) / 1000

	if tileCron != "si" {
		d2maprenderer.FijarRelojDeLosTiles(nil)
		pvr.FijarDiagnosticoDelBucle135("no")

		println("JUEGO135M: arranca -- tileCron= no bucle= no (el renderer del mapa NO cuenta nada)",
			"costo_de_1000_llamadas_us=", int(costoDeUnaMarca135*1000))

		return
	}

	d2maprenderer.FijarRelojDeLosTiles(relojDelPoc135)
	pvr.FijarDiagnosticoDelBucle135(bucle135)
	pvr.FijarCaminoPlano135(plano135 == "si")

	println("JUEGO135M: arranca -- tileCron= si bucle=", pvr.DiagnosticoDelBucle135(),
		"plano=", plano135, "cacheRapida=", cacheRapida135,
		"marcasPorTile=", marcasPorTile135, "costo_de_1000_llamadas_us=", int(costoDeUnaMarca135*1000),
		"costo_de_una_marca_us_x1000=", int(costoDeUnaMarca135*1000))
}

// informeDeLosTiles135: la linea del tramo (cada `cuadrosDelInforme132` cuadros) y el reinicio del censo.
func informeDeLosTiles135() {
	if tileCron != "si" {
		return
	}

	c := d2maprenderer.CensoDeLosTiles135()
	if c.Cuadros == 0 {
		return
	}

	n := int64(c.Cuadros)

	// el instrumento de un tramo: las marcas de todos los tiles que se pintaron.
	instrumento := int64(c.Tiles) * marcasPorTile135 * costoDeUnaMarca135 / n

	// lo que el renderer paga fuera de los cuatro bloques del tile: las pasadas (que ya incluyen `indexEntities`
	// y los fondos) menos los bloques del tile. Ahi estan los bucles de las pasadas (`TileAt`,
	// `PushTranslationWorld`) y los `Pop` de los `defer`, que corren despues de la ultima marca.
	sinTile := c.SumaPasos - c.SumaBloques - instrumento

	println("JUEGO135M: MAPA-POR-TILE cuadros=", cuadro, "delTramo=", c.Cuadros, "tiles=", c.Tiles,
		"imagenesNuevas=", int(c.Imagenes),
		"cache=", c.Us[0]/n, "vista=", c.Us[1]/n, "estado=", c.Us[2]/n, "pintado=", c.Us[3]/n,
		"cacheTiles=", c.Us[4]/n, "cacheBuscar=", c.Us[5]/n,
		"indice=", c.UsPaso[0]/n, "fondos=", c.UsPaso[1]/n,
		"p1=", c.UsPaso[2]/n, "p2=", c.UsPaso[3]/n, "p3=", c.UsPaso[4]/n, "p4=", c.UsPaso[5]/n,
		"sumaBloques=", c.SumaBloques/n, "sumaPasos=", c.SumaPasos/n,
		"sinTile=", sinTile/n, "instrumento=", instrumento,
		"sumidero=", int(pvr.SumideroDelDiagnostico135()&0xffff),
		"bucle=", pvr.DiagnosticoDelBucle135(),
		"gcPorCuadro_x100=", int(d2cGcColecciones()-colecciones135)*100/int(c.Cuadros),
		"colecciones=", int(d2cGcColecciones()),
		"blitsPlanosPorCuadro=", (pvr.CaminoPlanoUsado135-blitsPlanos135)/c.Cuadros,
		"paletaComprobada=", pvr.VecesQueSeComproboLaPaleta135())

	colecciones135 = d2cGcColecciones()
	blitsPlanos135 = pvr.CaminoPlanoUsado135

	d2maprenderer.ReiniciarCensoDeLosTiles135()
}

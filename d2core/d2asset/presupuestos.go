package d2asset

// DC-002.75 -- LOS PRESUPUESTOS DE CACHE SON DE LA PLATAFORMA, NO DEL JUEGO.
//
// EL PROBLEMA QUE RESUELVE. DC-002.74 midio que el techo del cooperativo no es la imagen del binario
// sino lo que el juego se guarda en RAM: los ocho caches del AssetManager. El cache YA es un LRU con
// presupuesto (d2common/d2cache), pero los presupuestos eran numeros de PC -- 128 MB de animaciones y
// 1 GB por cada cache de datos -- que en una consola de 16 MB (y un monton de ~6,15 MB) equivalen a
// "sin limite": el cache nunca llega a desalojar nada y el juego se queda con todo.
//
// DOS COSAS QUE SE MIDIERON Y QUE NO ESTABAN DICHAS:
//
//  1. El presupuesto NO se comparaba contra BYTES: cada entrada se insertaba con peso 1
//     (`defaultCacheEntryWeight`), asi que un presupuesto de "1 GB" eran 1.073.741.824 ENTRADAS. O
//     sea que el numero grande del codigo no significaba lo que decia. En la consola el peso de una
//     entrada es su TAMANO EN BYTES y el presupuesto vuelve a significar bytes; en la PC el peso
//     sigue siendo 1, que es EXACTAMENTE el comportamiento de siempre (ver `Pesar`).
//  2. Que numeros poner no se puede decidir a ojo: se midio con `console/dc/medidas/presupuestos` y
//     con el POC del motor (`console/dc/poc/presupuestos`) en Flycast. Los numeros y el porque estan
//     en `console/dc/evidencia/dc-002.75-presupuestos-de-cache.txt`.
//
// POR QUE VARIABLES Y NO UN ARCHIVO CON BUILD TAG. Porque gccgo NO filtra por build tag: compila
// TODOS los .go del paquete (medido en DC-002.2, ver la skill dreamcast-godc). Dos archivos, uno con
// el tag `dreamcast` y otro con `!dreamcast`, entrarian los DOS en el build de la consola y darian
// "redeclared". Se elige en tiempo de ejecucion, que ademas es lo unico que se puede medir.
//
// QUIEN LO CAMBIA. El arranque de la consola llama a `AjustarPresupuestosDeConsola()` ANTES de crear el
// AssetManager. Hoy ese arranque es el POC de esta tarjeta (console/dc/poc/presupuestos); cuando la
// Fase D monte el motor en la consola, la llamada va en su main. El host (ebiten) no la llama nunca,
// asi que en PC no cambia nada.
//
// LOS CACHES DE FUENTES, PALETAS Y TRANSFORMACIONES NO SE TOCAN: sus presupuestos ya son de consola
// (128 y 64 entradas) porque esas entradas son chicas y de tamano parejo; lo que se acota son los
// cuatro caches de DATOS (animaciones, cofs, dccs y dt1s) y el de ds1s.

import (
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2cache"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2dc6"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2dcc"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2ds1"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2dt1"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2interface"
)

const (
	// ============================ PC: los numeros de SIEMPRE ============================
	// No se tocan: en el host tienen que quedar EXACTAMENTE como estaban (DC-002.75).
	presupuestoAnimacionesPC = 1024 * 1024 * 128
	presupuestoFontesPC      = 128
	presupuestoPaletasPC     = 64
	presupuestoTransformaPC  = 64

	presupuestoDT1sPC = 4096 * 2048 * 128
	presupuestoDS1sPC = 4096 * 2048 * 128
	presupuestoCoFsPC = 4096 * 2048 * 128
	presupuestoDCCsPC = 4096 * 2048 * 128
)

const (
	// ============================ CONSOLA: bytes medidos ============================
	// El techo de la tarjeta era 3 MB para la SUMA de los caches de datos. Lo que se eligio suma
	// 2,81 MB y cada numero sale de una medida, no de un gusto (ver la evidencia):
	//
	//   animaciones 1 MB   un modo del personaje (7 capas .dcc "NU") pesa 442.601 B medidos: 1 MB
	//                      alcanza para DOS jugadores con su modo cargado y deja el resto para el LRU.
	//   dccs       512 KB  un modo completo del personaje (442.601 B) entra entero: el jugador que se
	//                      esta dibujando nunca se queda sin su capa. La capa mas grande mide 196.179 B.
	//   dt1s         1 MB  el .dt1 mas grande del acto 1 (`floor.dt1`) pesa 921.903 B: entra UNO grande
	//                      o dos medianos. Los ocho .dt1 de una zona suman 4.094.045 B, o sea que no
	//                      entran ni en el monton: el cache tiene que desalojar, y desaloja.
	//   ds1s       256 KB  la zona del acto 1 (57x41 celdas) ocupa 225.428 B en memoria: entra entera.
	//   cofs        64 KB  un COF cacheado pesa 1.388 B: 47 caben de sobra (4 jugadores + los NPC de
	//                      una zona).
	presupuestoAnimacionesConsola = 1024 * 1024
	presupuestoDCCsConsola        = 512 * 1024
	presupuestoDT1sConsola        = 1024 * 1024
	presupuestoDS1sConsola        = 256 * 1024
	presupuestoCoFsConsola        = 64 * 1024

	// techoDeLosCachesDeDatos: el techo DURO de DC-002.75. La consola tiene 16 MB de RAM y el monton
	// ronda los 6,15 MB: si los caches de datos se quedan con mas de 3 MB no queda lugar para los
	// cuatro jugadores del cooperativo (DC-002.76). Lo que se eligio suma 2,81 MB.
	techoDeLosCachesDeDatos = 3 * 1024 * 1024
)

// bytesPorCeldaDS1: lo que ocupa una celda del DS1 en memoria (un d2ds1.Tile por cada una de las
// cuatro capas: piso, pared, sombra y sustitucion). Medido en el host: unsafe.Sizeof(d2ds1.Tile{}) = 24.
const bytesPorCeldaDS1 = 24

// cabeceraDeFrameDC6: los campos fijos de un frame de DC6 fuera de sus datos (flipped, width, height,
// offsetX, offsetY, unknown, nextBlock, length y el terminador).
const cabeceraDeFrameDC6 = 32

var (
	// los presupuestos VIGENTES. Arrancan en los de PC y son variables porque la consola los cambia al
	// arrancar (AjustarPresupuestosDeConsola).
	animationBudget        = presupuestoAnimacionesPC
	fontBudget             = presupuestoFontesPC
	paletteBudget          = presupuestoPaletasPC
	paletteTransformBudget = presupuestoTransformaPC
	dt1Budget              = presupuestoDT1sPC
	ds1Budget              = presupuestoDS1sPC
	cofBudget              = presupuestoCoFsPC
	dccBudget              = presupuestoDCCsPC

	// pesoEnBytes: si es true, el peso de una entrada de cache de datos es su tamano en bytes y el
	// presupuesto se compara contra bytes. En PC queda false y el peso es 1 por entrada: el mismo
	// comportamiento de siempre (con los presupuestos de PC nunca se llego a desalojar nada).
	pesoEnBytes bool
)

// AjustarPresupuestosDeConsola pone los presupuestos de la Dreamcast y hace que los caches de datos
// pesen sus entradas en BYTES. Lo llama el arranque de la consola; el host no la llama nunca.
func AjustarPresupuestosDeConsola() {
	animationBudget = presupuestoAnimacionesConsola
	cofBudget = presupuestoCoFsConsola
	dccBudget = presupuestoDCCsConsola
	dt1Budget = presupuestoDT1sConsola
	ds1Budget = presupuestoDS1sConsola

	pesoEnBytes = true

	// DC-002.80 -- LA CARGA DE TILES POR ARENA YA NO SE APAGA ACA: la prende `NuevoAssetManagerDeConsola`
	// (`consola.go`), que es el unico camino que arma un AssetManager en la consola.
	//
	// Historia, porque el comentario de DC-002.78 decia lo contrario: la ruta del arena (cada .dt1 leido
	// adentro de un arena pedido una sola vez) se implemento en .78 y NO cerraba, porque el que pedia
	// memoria por archivo no era solo el destino: era la LECTURA por el stream del MPQ (~2 MB de semiespacio
	// por archivo; medido con el recolector frenado: `gc_alloc: OOM` pidiendo 3.328 B en la PRIMERA carga).
	// DC-002.80 agrego la lectura con memoria PRESTADA (`d2mpq.ReadFileInto`) y la midio: las tres fases
	// terminan sin OOM y sin excepcion, `pico(c) - pico(a) = 0` y el lienzo vuelve con crc32 3507647456 --
	// con el recolector normal Y con el recolector frenado. Numeros en
	// `console/dc/evidencia/dc-002.80-lectura-con-memoria-prestada.txt`.
}

// Presupuestos son los ocho presupuestos vigentes, para informar y medir.
type Presupuestos struct {
	Animaciones, CoFs, DCCs, DT1s, DS1s, Fuentes, Paletas, Transformaciones int
}

// PresupuestosVigentes devuelve los presupuestos que se van a usar al crear el AssetManager.
func PresupuestosVigentes() Presupuestos {
	return Presupuestos{
		Animaciones:      animationBudget,
		CoFs:             cofBudget,
		DCCs:             dccBudget,
		DT1s:             dt1Budget,
		DS1s:             ds1Budget,
		Fuentes:          fontBudget,
		Paletas:          paletteBudget,
		Transformaciones: paletteTransformBudget,
	}
}

// SumaDeLosCachesDeDatos es lo que el techo de la consola acota: animaciones + cofs + dccs + dt1s + ds1s.
func (p Presupuestos) SumaDeLosCachesDeDatos() int {
	return p.Animaciones + p.CoFs + p.DCCs + p.DT1s + p.DS1s
}

// CachesDeAssets son los ocho caches del AssetManager, ya creados con los presupuestos vigentes.
// Se devuelven concretos (y no como d2interface.Cache) para que quien mida pueda leer las evicciones.
type CachesDeAssets struct {
	Animations *d2cache.Cache
	Fonts      *d2cache.Cache
	Palettes   *d2cache.Cache
	Transforms *d2cache.Cache
	Dt1s       *d2cache.Cache
	Ds1s       *d2cache.Cache
	Cofs       *d2cache.Cache
	Dccs       *d2cache.Cache
}

// NuevosCachesDeAssets crea los ocho caches del AssetManager con los presupuestos vigentes. Es el UNICO
// lugar donde se crean: el AssetManager y el POC de la consola usan los mismos numeros por construccion.
func NuevosCachesDeAssets() *CachesDeAssets {
	return &CachesDeAssets{
		Animations: d2cache.CreateCache(animationBudget).(*d2cache.Cache),
		Fonts:      d2cache.CreateCache(fontBudget).(*d2cache.Cache),
		Palettes:   d2cache.CreateCache(paletteBudget).(*d2cache.Cache),
		Transforms: d2cache.CreateCache(paletteTransformBudget).(*d2cache.Cache),
		Dt1s:       d2cache.CreateCache(dt1Budget).(*d2cache.Cache),
		Ds1s:       d2cache.CreateCache(ds1Budget).(*d2cache.Cache),
		Cofs:       d2cache.CreateCache(cofBudget).(*d2cache.Cache),
		Dccs:       d2cache.CreateCache(dccBudget).(*d2cache.Cache),
	}
}

// CachesDeAssets devuelve los ocho caches de un AssetManager ya armado, para medirlos (DC-002.81).
//
// POR QUE HACE FALTA. `NuevoAssetManagerDeConsola` devuelve los caches porque los crea; `NewAssetManager`
// -- el que usa el MOTOR (`d2app/app.go:134`) -- no. Sin esto, medir el camino del motor obligaba a mirar el
// AssetManager por dentro desde otro paquete, o sea a duplicarlo. Es ADITIVO: no cambia el AssetManager ni
// el camino de la PC.
func (am *AssetManager) CachesDeAssets() *CachesDeAssets {
	if am == nil {
		return nil
	}

	return &CachesDeAssets{
		Animations: cacheConcreto(am.animations),
		Fonts:      cacheConcreto(am.fonts),
		Palettes:   cacheConcreto(am.palettes),
		Transforms: cacheConcreto(am.transforms),
		Dt1s:       cacheConcreto(am.dt1s),
		Ds1s:       cacheConcreto(am.ds1s),
		Cofs:       cacheConcreto(am.cofs),
		Dccs:       cacheConcreto(am.dccs),
	}
}

// cacheConcreto baja un cache de la interfaz al tipo real, que es el que lleva los desalojos y el peso.
func cacheConcreto(c d2interface.Cache) *d2cache.Cache {
	if c == nil {
		return nil
	}

	concreto, _ := c.(*d2cache.Cache)

	return concreto
}

// Pesar devuelve el peso con el que se inserta una entrada de cache de datos.
//
// En la PC devuelve defaultCacheEntryWeight (1 por entrada), que es lo que hacia el motor hasta
// DC-002.75: nada cambia. En la consola devuelve el tamano en bytes, y asi el presupuesto significa
// bytes de verdad y el LRU desaloja cuando la RAM aprieta.
func Pesar(tamanoEnBytes int) int {
	if !pesoEnBytes {
		return defaultCacheEntryWeight
	}

	if tamanoEnBytes < 1 {
		return 1
	}

	return tamanoEnBytes
}

// ============================ el tamano de lo que se cachea ============================
// Son las mismas medidas que hace el host con `console/dc/medidas/presupuestos`: el archivo crudo mas
// los buffers que el parser reserva y que quedan vivos mientras la entrada viva.

// TamanoDT1 es lo que ocupa un DT1 en memoria: los datos codificados de todos sus bloques (el crudo se
// suelta apenas se parsea, lo que queda es `EncodedData`).
func TamanoDT1(d *d2dt1.DT1) int {
	total := 0

	for i := range d.Tiles {
		for j := range d.Tiles[i].Blocks {
			total += len(d.Tiles[i].Blocks[j].EncodedData)
		}
	}

	return total
}

// TamanoDS1 es lo que ocupa un DS1 en memoria: sus cuatro capas de celdas (medido: 24 B por celda y
// capa), su tabla de archivos y sus objetos.
func TamanoDS1(d *d2ds1.DS1) int {
	celdas := d.Width() * d.Height()

	return 4*celdas*bytesPorCeldaDS1 + len(d.Files)*32 + len(d.Objects)*20 + len(d.SubstitutionGroups)*20
}

// TamanoDCC es lo que ocupa un DCC en memoria: el archivo crudo (que el parser guarda) mas los pixeles
// ya decodificados de cada direccion y de cada frame.
func TamanoDCC(d *d2dcc.DCC) int {
	total := 0

	for _, dir := range d.Directions {
		if dir == nil {
			continue
		}

		total += len(dir.PixelData)

		for i := range dir.Frames {
			total += len(dir.Frames[i].PixelData)
		}
	}

	return total + tamanoCrudoDCC(d)
}

// tamanoCrudoDCC: el DCC no expone el archivo crudo que guarda, pero si el ancho por alto de cada
// direccion y frame; el crudo es una fraccion chica. Se informa lo decodificado, que es lo que manda.
func tamanoCrudoDCC(d *d2dcc.DCC) int {
	total := 0

	for _, dir := range d.Directions {
		if dir == nil {
			continue
		}

		total += dir.Box.Width * dir.Box.Height / 8 // el DCC comprime por celdas; ~1/8 del crudo decodificado
	}

	return total
}

// TamanoDC6 es lo que ocupa un DC6 en memoria: los datos de sus frames.
func TamanoDC6(d *d2dc6.DC6) int {
	total := len(d.FramePointers) * 4

	for _, f := range d.Frames {
		if f == nil {
			continue
		}

		total += len(f.FrameData) + cabeceraDeFrameDC6
	}

	return total
}

// TamanoDeAnimacion es el peso de una animacion cacheada. Una animacion de DCC o DC6 se APOYA en el
// archivo ya parseado (no lo copia), asi que pesa lo mismo que ese archivo: el presupuesto de
// animaciones y el de dccs cuentan la misma RAM dos veces, y contar de mas es del lado seguro (se
// desaloja antes, nunca despues).
func TamanoDeAnimacion(a d2interface.Animation) int {
	switch anim := a.(type) {
	case *DCCAnimation:
		if anim != nil && anim.dcc != nil {
			return TamanoDCC(anim.dcc)
		}
	case *DC6Animation:
		if anim != nil && anim.dc6 != nil {
			return TamanoDC6(anim.dc6)
		}
	}

	return defaultCacheEntryWeight
}

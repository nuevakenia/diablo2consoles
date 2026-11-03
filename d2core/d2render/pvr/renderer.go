//go:build dreamcast

package pvr

// EL RENDERER DE LA DREAMCAST (DC-002.70): la implementacion de d2interface.Renderer sobre el PVR.
//
// QUE ES. d2common/d2interface/renderer.go declara 15 metodos: el ciclo de cuadros (Run), las superficies
// (CreateSurface/NewSurface), el estado de ventana (fullscreen/vsync/cursor), el reloj (CurrentFPS), el
// cartel de panico (ShowPanicScreen) y el texto de depuracion (Print/PrintAt). Hasta esta tarjeta la unica
// implementacion era d2core/d2render/ebiten (PC). Esta es la de la consola.
//
// COMO ES UN CUADRO (el orden importa, y es el que estaba medido en el shim de DC-002.9/22):
//
//	PvrWaitReady      el PVR dio el cuadro anterior (es la sincronizacion con la pantalla)
//	PvrSceneBegin     arranca la escena
//	PvrListBegin(OP)  la lista OPACA: el cuadro compuesto, que tapa todo
//	actualizar()      el juego avanza su estado
//	dibujar(malla)    el juego compone el cuadro en el lienzo de la Surface
//	Presentar+Pintar  el lienzo a la VRAM (RGB565 por bloques de 32x32) y el PVR lo pinta
//	PvrListFinish / PvrSceneFinish
//
// LA PANTALLA VIVE EN LA VRAM, y esto NO es una optimizacion: es lo que hace que el cuadro entre. El
// lienzo 640x480x4 son 1.228.800 B y el monton de C del binario (despues del recorte de DC-002.68) deja un
// techo de ~1,23 MB para UN bloque: con el mapa del nivel (512x512 = 262.144 B) vivo al mismo tiempo no
// entra. La palanca es la de DC-002.22, ya medida: `d2c_pvr_pantalla_fb_init` reserva w*h*4 en la VRAM (8
// MB, de los que el PVR usa ~1 MB para la textura del cuadro) y el slice de pixeles apunta ahi. Si la VRAM
// no diera, el Renderer cae al monton y lo DICE por la consola serie: el que mide sabe cual de los dos
// caminos tomo.
//
// LO QUE NO PUEDE SER FIEL, DECLARADO (no escondido):
//   - SetFullScreen/IsFullScreen: en la consola el modo de video es uno (640x480); el pedido se guarda y el
//     valor devuelto es ese. No hay ventana que agrandar.
//   - vsync: el PVR ya entrega los cuadros a la frecuencia del video (PvrWaitReady), asi que
//     SetVSyncEnabled(false) NO puede desincronizar el chip: el pedido se guarda y se informa.
//   - SetWindowIcon: no hay ventana ni icono.
//   - GetCursorPos: no hay mouse; devuelve (0,0) y lo declara. El control de la Dreamcast no es un cursor.
//   - Print/PrintAt: la fuente es la 3x5 PROVISIONAL de este backend (fuente.go). Con un target que no sea
//     una Surface de este paquete, el texto va a la consola serie.
//   - CurrentFPS se mide con el reloj del PVR (d2c_pvr_pantalla_us), no con contadores del SH-4: los
//     PerfCntr devuelven 0 en Flycast (medido en DC-002.49).

import (
	"image/color"

	"kos"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2interface"
)

// Los alias son los de d2common/d2interface (ahi son alias, no tipos con nombre: la firma tiene que
// coincidir exactamente o el metodo no implementa el contrato).
type renderCallback = func(d2interface.Surface) error

type updateCallback = func() error

// Renderer es el ciclo de cuadros de la consola: la pantalla del motor en la VRAM del PVR.
type Renderer struct {
	dibujar    renderCallback
	actualizar updateCallback

	superficie *Surface

	ancho, alto int
	titulo      string

	// vsync: lo que pidio el motor (el PVR entrega a la frecuencia del video igual; ver la cabecera).
	vsync      bool
	pantallaSi bool // el video arranco (fullscreen es el unico estado que hay)

	// fps: el promedio del ultimo segundo medido, no el de un cuadro suelto.
	fps        float64
	cuadros    int
	msAntes    uint64
	cuadrosVen int

	ultimoError error

	// DC-002.116 (el split vertical): con estos dos puestos, el ciclo NO usa el lienzo de 640x480 ni
	// `PresentarEnPantalla`. `superficiePropia` es la superficie que recibe `dibujar` (uno de los dos lienzos
	// del split) y `presentar` es el que sube los dos lienzos, cada uno a su mitad de la pantalla. Sin
	// ponerlos, el camino es el de siempre (y los otros POC no cambian una linea).
	superficiePropia *Surface
	presentar        func() error

	// contadores del lienzo (DC-002.85): ver cuenta.go.
	lienzosCreados      int
	bytesDeLienzos      int
	expansionesDirectas int
	bytesExpandidos     int
	copiasDeBuffer      int
	bytesCopiados       int

	// DC-002.132 -- EL DESGLOSE DEL CUADRO, MEDIDO POR DENTRO.
	//
	// POR QUE. Los numeros del POC decian "el mapa cuesta 675-931 ms", pero el cuadro entero son ~1.000 ms
	// y nadie sabia cuanto se llevaba la PRESENTACION (convertir RGBA->RGB565 los 307.200 pixeles y
	// subirlos a la VRAM). Sin ese numero no se puede elegir que atacar: si la presentacion sola cuesta
	// 60 ms, el techo esta en 16 cuadros por segundo aunque el mapa se dibuje gratis.
	//
	// Se mide en las tres etapas reales del ciclo y se promedia cada `cuadrosDelInforme` cuadros (un cuadro
	// suelto miente: el primero compone el nivel y los siguientes calientan las caches).
	msEspera    int // la espera al PVR (el vsync del chip)
	msTrabajo   int // actualizar + dibujar
	msPresentar int // convertir + subir a la VRAM + pintar el quad
	msCuadro    int // de la espera al final del cuadro: el periodo real

	// DC-002.135 -- LOS TRES PEDAZOS DE LA PRESENTACION, acumulados para el promedio del tramo.
	usConv   int // el bucle que convierte RGBA -> RGB565 (medido en C, muestreado por bloques)
	usVram   int // las copias a la VRAM por Store Queues (medido en C)
	usPintar int // el quad de pantalla completa (medido aca: una marca antes y otra despues)
}

// que el contrato se cumpla no es una suposicion: si d2interface.Renderer cambia, esto no compila.
var _ d2interface.Renderer = (*Renderer)(nil)

// NuevoRenderer: el renderer de la consola, sin video todavia (el video lo arranca Run).
func NuevoRenderer() *Renderer {
	return &Renderer{
		vsync:  true, // el PVR entrega los cuadros con el video: por omision, sincronizado
		ancho:  anchoDelCuadro,
		alto:   altoDelCuadro,
		titulo: "OpenDiablo2",
	}
}

const (
	anchoDelCuadro = 640
	altoDelCuadro  = 480

	// cuadrosDelInforme (DC-002.132): cada cuantos cuadros el ciclo deja el desglose del cuadro en la consola
	// serie. 30 cuadros son medio segundo a 60 cuadros/s y 30 s a 1 cuadro/s: el promedio dice el ritmo real
	// sin inundar el log (a 60 cuadros/s una linea por cuadro son 60 lineas por segundo por el puerto serie).
	cuadrosDelInforme = 30
)

// GetRendererName: el nombre que sale en la configuracion y en los logs del motor.
func (r *Renderer) GetRendererName() string { return "PVR (Dreamcast)" }

// SetWindowIcon: no hay ventana en la consola. Se declara como no-op (ver la cabecera).
func (r *Renderer) SetWindowIcon(fileName string) {}

// Run arranca el video y corre el ciclo de cuadros: por cada cuadro actualiza, dibuja, y el cuadro se lo
// lleva el PVR. NO VUELVE: en la consola no hay evento de cierre (el jugador apaga la consola), igual que
// en el bucle del shim de DC-002.9.
func (r *Renderer) Run(dibujar renderCallback, actualizar updateCallback, ancho, alto int, titulo string) error {
	if ancho > 0 {
		r.ancho = ancho
	}

	if alto > 0 {
		r.alto = alto
	}

	r.titulo = titulo
	r.dibujar = dibujar
	r.actualizar = actualizar

	if err := PantallaIniciar(r.ancho, r.alto); err != nil {
		return err
	}

	r.pantallaSi = true

	// LA SUPERFICIE DEL CUADRO, con los pixeles en la VRAM si da (ver la cabecera). Si no da, cae al
	// monton y lo dice: el numero que lo decide es el que se imprime.
	//
	// DC-002.116: si el que arranca puso SU superficie (el split, que compone en dos lienzos de 320x480), no
	// se reserva el lienzo de 640x480: reservarlo igual seria 1.228.800 B de VRAM para un cuadro que nadie
	// escribe, y en el split esos bytes son justo los de las dos mitades.
	if r.superficiePropia != nil {
		r.superficie = r.superficiePropia
	} else {
		r.superficie = SuperficieDeLaPantalla(r.ancho, r.alto)
	}

	r.superficie.renderer = r

	kos.PvrSetBgColor(0, 0, 0)

	r.msAntes = uint64(d2cPvrUs())

	for {
		// el cuadro lo marca el PVR: sin esto se dibujaria encima del que se esta mostrando
		tEspera := d2cPvrUs()

		kos.PvrWaitReady()
		kos.PvrSceneBegin()

		// la lista OPACA: el cuadro compuesto tapa todo, no hay nada detras
		kos.PvrListBegin(kos.PVR_LIST_OP_POLY)

		tTrabajo := d2cPvrUs()

		if r.actualizar != nil {
			if err := r.actualizar(); err != nil {
				kos.PvrListFinish()
				kos.PvrSceneFinish()

				return err
			}
		}

		r.ultimoError = nil

		if r.dibujar != nil {
			if err := r.dibujar(r.superficie); err != nil {
				r.ultimoError = err

				kos.PvrListFinish()
				kos.PvrSceneFinish()

				return err
			}
		}

		tPresentar := d2cPvrUs()

		// DC-002.135: los tres pedazos de la presentacion. La conversion y la subida las cronometra el
		// backend de C por dentro (con una muestra de bloques, ver `pvr_pantalla.c`); el PINTADO del quad
		// se cronometra aca, con una marca antes y otra despues de `PintarPantalla()`.
		usConv, usVram, usPintar := 0, 0, 0

		if r.presentar != nil {
			// DC-002.116: el cuadro partido en dos lienzos, cada uno a su mitad de la pantalla.
			if err := r.presentar(); err != nil {
				r.ultimoError = err

				kos.PvrListFinish()
				kos.PvrSceneFinish()

				return err
			}

			// el split pinta por dentro del presentador: el cuadro es lo que sobra
			usConv = d2cPvrPresConvUs()
			usVram = d2cPvrPresVramUs()
			usPintar = int(d2cPvrUs()-tPresentar) - usConv - usVram
		} else if err := Presentar(r.superficie.Imagen()); err != nil {
			r.ultimoError = err

			kos.PvrListFinish()
			kos.PvrSceneFinish()

			return err
		} else {
			usConv = d2cPvrPresConvUs()
			usVram = d2cPvrPresVramUs()

			tPintar := d2cPvrUs()
			PintarPantalla()
			usPintar = int(d2cPvrUs() - tPintar)
		}

		kos.PvrListFinish()
		kos.PvrSceneFinish()

		r.cuadros++
		r.anotarElCuadro(tTrabajo-tEspera, tPresentar-tTrabajo, d2cPvrUs()-tEspera, usConv, usVram, usPintar)
		r.medirFPS()
	}
}

// anotarElCuadro: acumula el desglose del cuadro y, cada `cuadrosDelInforme` cuadros, lo deja en la consola
// serie como PROMEDIO (un cuadro suelto miente).
//
// LOS CUATRO NUMEROS, EN EL ORDEN EN QUE LOS PAGA EL CICLO:
//
//	esperar  -- la espera al PVR (el chip entrega a la frecuencia del video: si el cuadro es barato, esto
//	            es lo que sobra hasta el proximo vsync);
//	trabajo  -- actualizar + dibujar: lo que el juego compone (el mapa, los sprites, el HUD);
//	presentar-- convertir el lienzo RGBA a RGB565, subirlo a la VRAM y pintar el quad de pantalla completa;
//	cuadro   -- el periodo real, de la espera al final: 1.000.000/cuadro son los cuadros por segundo.
//
// Y DC-002.135 agrega el desglose DE LA PRESENTACION en sus tres pedazos (conversion / subida a la VRAM /
// pintado del quad), en su propia linea `PVR135: PRESENTAR`: la de PVR132 queda igual, con el total, para
// que las corridas de la tarjeta anterior se sigan pudiendo comparar renglon a renglon.
func (r *Renderer) anotarElCuadro(usEspera, usTrabajo, usCuadro, usConv, usVram, usPintar int) {
	r.msEspera += usEspera
	r.msTrabajo += usTrabajo
	r.msCuadro += usCuadro

	usPresentar := usCuadro - usEspera - usTrabajo
	r.msPresentar += usPresentar

	r.usConv += usConv
	r.usVram += usVram
	r.usPintar += usPintar

	if r.cuadros%cuadrosDelInforme != 0 {
		return
	}

	n := cuadrosDelInforme

	usCuadroPromedio := r.msCuadro / n
	fpsX100 := 0
	if usCuadroPromedio > 0 {
		fpsX100 = 100000000 / usCuadroPromedio // 1.000.000 us/s / us por cuadro, en centesimas
	}

	println("PVR132: CUADRO", r.cuadros,
		"espera_us=", r.msEspera/n, "trabajo_us=", r.msTrabajo/n,
		"presentar_us=", r.msPresentar/n, "cuadro_us=", usCuadroPromedio,
		"fpsX100=", fpsX100, "desglose=", "espera+trabajo+presentar")

	// DC-002.135: la presentacion en sus tres pedazos. `total_us` es el mismo numero que el presentar_us de
	// arriba (medido por el ciclo) y la suma de los tres tiene que dar eso: si no da, el desglose miente y el
	// que lee lo ve en el acto. `subidas` es el control de la palanca: 300 copias de 2.048 B sin agrupar, 80
	// (o 75) con los grupos de 2x2 bloques.
	println("PVR135: PRESENTAR conv_us=", r.usConv/n, "vram_us=", r.usVram/n,
		"pintar_us=", r.usPintar/n, "total_us=", r.msPresentar/n,
		"bloques=", BloquesPresentados(), "subidas=", SubidasPresentadas(),
		"convRapida=", convRapidaSiNo(), "vramAgrupada=", vramAgrupadaSiNo())

	r.msEspera, r.msTrabajo, r.msPresentar, r.msCuadro = 0, 0, 0, 0
	r.usConv, r.usVram, r.usPintar = 0, 0, 0
}

// convRapidaSiNo / vramAgrupadaSiNo: con que palancas corrio ESTE binario, para que la linea del desglose
// diga con que camino se midio (una linea sin eso no se puede comparar con otra).
func convRapidaSiNo() string {
	if convRapida {
		return "si"
	}

	return "no"
}

func vramAgrupadaSiNo() string {
	if vramAgrupada {
		return "si"
	}

	return "no"
}

// medirFPS: los cuadros por segundo del ultimo segundo medido (ventana de 1 s, no un cuadro suelto).
func (r *Renderer) medirFPS() {
	ahora := uint64(d2cPvrUs())
	r.cuadrosVen++

	transcurrido := ahora - r.msAntes
	if transcurrido < 1000000 {
		return
	}

	r.fps = float64(r.cuadrosVen) * 1000000.0 / float64(transcurrido)
	r.cuadrosVen = 0
	r.msAntes = ahora

	// DC-002.132: los cuadros por segundo, en el log, en enteros (el println de la consola no imprime
	// float64 de forma confiable y el numero que se compara es el entero de todos modos).
	println("PVR132: FPS seg_fpsX100=", int(r.fps*100), "cuadros=", r.cuadros)
}

// IsDrawingSkipped: si el ultimo cuadro no se pudo componer (el motor lo usa para no contar un cuadro
// como bueno).
func (r *Renderer) IsDrawingSkipped() bool { return r.ultimoError != nil }

// CreateSurface: una superficie NUEVA sobre el MISMO lienzo de la que se le pasa, con el estado de dibujado
// neutro. Es la semantica del backend de la PC (d2core/d2render/ebiten/ebiten_renderer.go:209).
func (r *Renderer) CreateSurface(surface d2interface.Surface) (d2interface.Surface, error) {
	origen, ok := surface.(*Surface)
	if !ok || origen == nil {
		return nil, errorPantalla("CreateSurface: la superficie no es de este backend")
	}

	otra := SurfaceDesdeImagen(origen.imagen)
	otra.renderer = r

	return otra, nil
}

// NewSurface: una superficie nueva del tamano pedido, en el monton (las superficies chicas -- un sprite
// compuesto, un panel -- no compiten con la pantalla por el techo de la VRAM).
func (r *Renderer) NewSurface(ancho, alto int) d2interface.Surface {
	s := NuevaSurface(ancho, alto)
	s.renderer = r

	r.lienzosCreados++
	r.bytesDeLienzos += len(s.imagen.pixeles)

	return s
}

// IsFullScreen: en la consola el video siempre ocupa la pantalla entera.
func (r *Renderer) IsFullScreen() bool { return true }

// SetFullScreen: se guarda lo pedido y se informa por la serie; no hay ventana que agrandar (ver cabecera).
func (r *Renderer) SetFullScreen(fullScreen bool) {
	if !fullScreen {
		println("PVR-RENDERER: SetFullScreen(false) no se puede cumplir en la consola; el video es uno solo")
	}
}

// SetVSyncEnabled: el PVR entrega los cuadros con el video (PvrWaitReady); el pedido se guarda.
func (r *Renderer) SetVSyncEnabled(vsync bool) { r.vsync = vsync }

// GetVSyncEnabled: lo ultimo que se pidio.
func (r *Renderer) GetVSyncEnabled() bool { return r.vsync }

// GetCursorPos: no hay mouse en la consola. Devuelve (0,0) y lo declara.
func (r *Renderer) GetCursorPos() (int, int) { return 0, 0 }

// CurrentFPS: los cuadros por segundo medidos con el reloj del PVR (ver la cabecera).
func (r *Renderer) CurrentFPS() float64 { return r.fps }

// ShowPanicScreen: el cartel de panico. Lo escribe en la consola serie y ademas lo pinta en la pantalla,
// que queda ahi para siempre: en la consola no hay a quien preguntarle nada, y una excepcion del SH-4 sin
// este cartel se ve como "la consola se reinicio".
func (r *Renderer) ShowPanicScreen(message string) {
	println("PVR-RENDERER PANICO:", message)

	// la pantalla, si el video no habia arrancado (un panico antes de Run)
	if r.superficie == nil {
		if err := PantallaIniciar(r.ancho, r.alto); err != nil {
			return
		}

		r.superficie = SuperficieDeLaPantalla(r.ancho, r.alto)
		r.superficie.renderer = r
	}

	r.superficie.Clear(color.RGBA{R: 40, G: 0, B: 0, A: 255})
	r.superficie.PushTranslation(16, 16)
	r.superficie.DrawTextf("PANICO (DC-002.70)")
	r.superficie.Pop()
	r.superficie.PushTranslation(16, 40)
	r.superficie.DrawTextf("%s", message)
	r.superficie.Pop()

	for {
		if err := PresentarEnPantalla(r.superficie.Imagen()); err != nil {
			return
		}

		kos.PvrWaitReady()
		kos.PvrSceneBegin()
		kos.PvrListBegin(kos.PVR_LIST_OP_POLY)
		PintarPantalla()
		kos.PvrListFinish()
		kos.PvrSceneFinish()
	}
}

// Print: el texto de depuracion, sin posicion: arriba a la izquierda del target (asi lo hace la PC).
func (r *Renderer) Print(target interface{}, str string) error {
	r.PrintAt(target, str, 0, 0)

	return nil
}

// PrintAt: el texto de depuracion en (x, y). Si el target es una Surface de este backend, se dibuja ahi
// (fuente 3x5 PROVISIONAL); si no, sale por la consola serie, que es la unica otra salida de texto que hay.
func (r *Renderer) PrintAt(target interface{}, str string, x, y int) {
	if s, ok := target.(*Surface); ok && s != nil {
		s.PushTranslation(x, y)
		s.DrawTextf("%s", str)
		s.Pop()

		return
	}

	println("PVR-RENDERER:", str)
}

// ============================================================ lo que no es del contrato

// Surface: la superficie del cuadro (la que se le pasa al callback de dibujado). Es para la evidencia y
// para el que quiera medir el cuadro sin pasar por el callback.
func (r *Renderer) Surface() *Surface { return r.superficie }

// UsarSuperficieDelCuadro (DC-002.116): la superficie que el ciclo le pasa a `dibujar`, en vez del lienzo de
// 640x480 de la pantalla. Es lo que usa el split vertical: el ciclo le entrega al POC UNO de sus dos lienzos de
// 320x480 y el POC compone el otro en el mismo callback. Tiene que llamarse ANTES de Run.
func (r *Renderer) UsarSuperficieDelCuadro(s *Surface) { r.superficiePropia = s }

// UsarPresentador (DC-002.116): con esto puesto, el ciclo NO presenta el lienzo de la pantalla con
// `PresentarEnPantalla`: llama a `f`, que es el que decide que subir y donde. El split sube los dos lienzos de
// 320x480 a las columnas 0 y 320 y pinta. Tiene que llamarse ANTES de Run.
func (r *Renderer) UsarPresentador(f func() error) { r.presentar = f }

// Cuadros: cuantos cuadros se presentaron (el instrumento del ciclo).
func (r *Renderer) Cuadros() int { return r.cuadros }

// PantallaEnVram: cuanto de la pantalla vive en la VRAM (0 = el lienzo cayo al monton).
func (r *Renderer) PantallaEnVram() int { return VramDeLaPantalla() }

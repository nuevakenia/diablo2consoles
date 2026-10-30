//go:build dreamcast

// DC-002.72 -- UN SONIDO DEL MOTOR POR EL AICA, A TRAVES DE d2interface.AudioProvider (Fase C, el audio).
//
// QUE PRUEBA, Y POR QUE ASI. DC-002.62 ya hizo sonar dos PCM por el AICA en el POC del personaje, pero con
// samples que GENERA EL BUILD (gen-pcm-262.py): el `d2sfx.mpq`, que es donde viven los sonidos del juego, no
// viajaba en el disco. Esta tarjeta pone el audio DETRAS DE LA INTERFAZ del motor: el POC ya no llama a
// ninguna funcion de sonido de KOS, le pide al contrato lo mismo que le pide el juego.
//
//	prov := aica.Nuevo(mpq)                       <- d2core/d2audio/aica (el backend de la consola)
//	var audio d2interface.AudioProvider = prov    <- el contrato: 3 metodos
//	ef, err := audio.LoadSound("combat\\impact\\sword5.wav", false, false)
//	ef.Play()
//
// EL EFECTO ES DEL MOTOR, no un fixture: el nombre es el de Sounds.txt y los bytes salen de
// d2Sfx.mpq (el MPQ del disco, con la ruta en el log), resueltos con el mismo d2soundpath que usa el
// proveedor de ebiten. Un sonido que no esta en el MPQ no suena: no hay samples dentro del binario.
//
// QUE SE MIDE, y sin ojos (la consola serie):
//   - los CANALES OCUPADOS del AICA antes de sonar, mientras suena y cuando termina. El numero sale del
//     registro del chip (`snd_is_playing`, el bit KEYONB de cada uno de los 64): no cuenta llamadas.
//   - el registro de 64 B del canal donde sono (cmd, base, tipo, largo, freq, vol, pan, pos), que es lo que
//     el AICA tiene cargado.
//   - la POSICION del canal dentro del efecto, en muestras, para ver que el chip AVANZA.
//   - el volumen y el pan traducidos (0..255 con 128 al centro), y un cambio de pan/volumen EN VUELO.
//   - un segundo efecto con loop y su Stop() (el contrato tiene Stop y el sfxmgr tambien).
//
// EL ENLACE: importa ebiten aunque no dibuje nada. Medido en DC-002.33 (console/dc/poc/tablas-mpq): sin
// importar ebiten -- que es el que arrastra el paquete `kos` de libgodc -- el binario arranca y muere con
// "Unhandled exception: PC ... code 1" antes de imprimir una linea.
package main

import (
	"fmt"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2mpq"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2interface"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2audio/aica"

	_ "github.com/hajimehoshi/ebiten/v2" // solo por el ENLACE del runtime (ver arriba)
)

//extern d2c_serial_init
func d2cSerialInit()

//extern d2c_ms
func d2cMs() uint32

// init: la consola serie es la UNICA salida de texto en la Dreamcast (sin esto KOS deja el handler en NULL
// y el println no sale a ninguna parte).
func init() { d2cSerialInit() }

const (
	// el MPQ de los sonidos: 2330 WAV. El POC lo lleva en el disco (scripts/console/build-aica-272.sh lo
	// mete en /JUEGO), porque en la consola no hay sistema de archivos del juego: el "directorio" es el MPQ.
	rutaMPQ = "/cd/JUEGO/D2SFX.MPQ"

	// los dos efectos de la prueba. Rutas de Sounds.txt (relativas a data/global/sfx), resueltas por
	// d2soundpath igual que en el proveedor de ebiten.
	sfxImpacto  = "combat\\impact\\sword5.wav"
	sfxMonstruo = "monster\\goatman\\gethit4.wav"
)

var (
	// el proveedor real y el contrato: si al proveedor le faltara un metodo, esto no compila. Es el criterio
	// de la tarjeta cumplido por el compilador y no por una lectura.
	proveedor *aica.Proveedor
	audio     d2interface.AudioProvider

	fallos int
)

// marca imprime una linea de la evidencia con el tiempo del arranque.
func marca(formato string, args ...interface{}) {
	println("AICA t=" + u32aTexto(d2cMs()) + "ms " + fmt.Sprintf(formato, args...))
}

// u32aTexto: un uint32 sin signo a texto (son ms del temporizador de KOS).
func u32aTexto(n uint32) string {
	if n == 0 {
		return "0"
	}

	var digitos [12]byte

	i := len(digitos)

	for n > 0 {
		i--
		digitos[i] = byte('0' + n%10)
		n /= 10
	}

	return string(digitos[i:])
}

// estadoDelChip: la foto de los 64 canales leida del CHIP, en una linea. Es el contador de la tarjeta.
func estadoDelChip(etiqueta string) {
	marca("%s canalesSonando=%d canalesLibres=%d aicaLibre=%d", etiqueta,
		aica.CanalesSonando(), aica.CanalesLibres(), aica.MemoriaLibre())
}

// volcarCanal: el registro de 64 B que el AICA tiene cargado para el canal.
func volcarCanal(etiqueta string, canal int) {
	if canal < 0 {
		marca("%s (sin canal: el efecto nunca sono)", etiqueta)

		return
	}

	e := aica.Estado(canal)

	marca("%s canal=%d cmd=%d base=%d tipo=%d largo=%d loop=%d freq=%d vol=%d pan=%d pos=%d", etiqueta,
		canal, e.Cmd, e.Base, e.Tipo, e.Largo, e.Loop, e.Frecuencia, e.Volumen, e.Pan, e.Posicion)
}

// esperarHasta: espera a que el temporizador llegue a `ms` (sin vsync: este POC no dibuja).
func esperarHasta(ms uint32) {
	for d2cMs() < ms {
	}
}

// correrEfecto: dispara el efecto y lo sigue hasta que el chip lo apaga (o hasta el tope).
//
// EL TOPE NO ES UN ADORNO: si el AICA no reportara el fin (o el efecto tuviera loop), el POC se quedaria
// mirando un canal para siempre y la corrida no llegaria a la segunda prueba. Con tope, el que no termina se
// informa y se sigue.
func correrEfecto(etiqueta string, efecto *aica.Efecto, topeMs uint32) {
	efecto.Play()

	canal := efecto.Canal()

	marca("%s disparado canal=%d sonando=%d canalesSonando=%d", etiqueta, canal,
		boolAint(efecto.IsPlaying()), aica.CanalesSonando())

	volcarCanal(etiqueta+" registro-al-sonar", canal)

	arranque := d2cMs()
	ultimaPos := -1

	for d2cMs()-arranque < topeMs {
		if !efecto.IsPlaying() {
			marca("%s FIN el chip apago el canal a los %dms pos=%d canalesSonando=%d", etiqueta,
				d2cMs()-arranque, aica.Posicion(canal), aica.CanalesSonando())

			volcarCanal(etiqueta+" registro-al-terminar", canal)

			return
		}

		pos := aica.Posicion(canal)
		if pos != ultimaPos {
			marca("%s avanzando pos=%d de %d muestras canalesSonando=%d", etiqueta, pos,
				efecto.Sonido().Muestras, aica.CanalesSonando())
			ultimaPos = pos
		}
	}

	marca("%s TOPE %dms alcanzado y el chip sigue con el canal encendido (sonando=%d pos=%d canalesSonando=%d)",
		etiqueta, topeMs, boolAint(efecto.IsPlaying()), aica.Posicion(canal), aica.CanalesSonando())
}

func main() {
	marca("ARRANQUE DC-002.72: un sonido del MOTOR por el AICA, por d2interface.AudioProvider")

	// 1) el MPQ de los sonidos, del disco. Se abre ANTES de encender el AICA: si el archivo no esta, no
	// tiene sentido inicializar el chip para nada.
	inicio := d2cMs()

	mpq, err := d2mpq.FromFile(rutaMPQ)
	if err != nil {
		marca("MPQ FALLO ruta=%s err=%v", rutaMPQ, err)
		fallos++
		marca("FIN con fallos=%d (sin MPQ no hay efecto del motor que probar)", fallos)

		return
	}

	marca("MPQ abierto ruta=%s bytes=%d ms=%d", rutaMPQ, mpq.Size(), d2cMs()-inicio)

	// 2) el proveedor (enciende el AICA) y el CONTRATO
	proveedor = aica.Nuevo(mpq)
	proveedor.Traza = func(linea string) { println(linea) }

	audio = proveedor

	// 3) los volumenes del motor (0..1) traducidos a la escala del AICA (0..255)
	audio.SetVolumes(1.0, 0.8)

	estadoDelChip("ANTES-DE-SONAR")

	// 4) el efecto DEL MOTOR
	efecto, err := audio.LoadSound(sfxImpacto, false, false)
	if err != nil {
		marca("CARGA FALLO sfx=%s err=%v", sfxImpacto, err)
		fallos++
	} else {
		e := efecto.(*aica.Efecto)
		son := e.Sonido()

		marca("EFECTO-DEL-MOTOR ruta=%s tasa=%d bits=%d canales=%d muestras=%d ms=%d vol=%d pan=%d handle=%d",
			e.Ruta(), son.Tasa, son.Bits, son.Canales, son.Muestras, son.DuracionMs(), e.Volumen(), e.Pan(),
			e.Handle())

		volcarCanal("registro-ANTES-de-sonar", 0)

		correrEfecto("EFECTO-1(sin loop)", e, 5000)

		// 5) el mismo efecto, con el pan y el volumen cambiados EN VUELO: el contrato tiene SetPan y
		// SetVolume, y en la consola tienen que llegar al chip (paquete de ACTUALIZACION), no quedarse
		// esperando el proximo play.
		e.SetPan(-1)
		e.SetVolume(0.5)

		marca("EN-VUELO pan=-1 volumen=0.5 -> panAICA=%d volAICA=%d", e.Pan(), e.Volumen())

		e.Play()
		esperarHasta(d2cMs() + 120)
		volcarCanal("registro-con-pan-al-maximo", e.Canal())
		marca("EN-VUELO durante el play sonando=%d canalesSonando=%d", boolAint(e.IsPlaying()),
			aica.CanalesSonando())

		e.SetPan(1)
		esperarHasta(d2cMs() + 120)
		volcarCanal("registro-con-pan-al-otro-extremo", e.Canal())

		// 6) Stop() sobre un efecto que esta sonando: el canal tiene que quedar libre YA
		e.Stop()
		marca("STOP hecho sonando=%d canalesSonando=%d", boolAint(e.IsPlaying()), aica.CanalesSonando())
	}

	// 7) un segundo efecto, con LOOP, y su Stop: el contrato tiene `loop` y el sfxmgr tambien
	enLoop, err := audio.LoadSound(sfxMonstruo, true, false)
	if err != nil {
		marca("CARGA FALLO (loop) sfx=%s err=%v", sfxMonstruo, err)
		fallos++
	} else {
		e := enLoop.(*aica.Efecto)

		marca("EFECTO-CON-LOOP ruta=%s muestras=%d ms=%d", e.Ruta(), e.Sonido().Muestras,
			e.Sonido().DuracionMs())

		correrEfecto("EFECTO-2(loop)", e, 700)

		e.Stop()
		marca("LOOP-STOP hecho sonando=%d canalesSonando=%d", boolAint(e.IsPlaying()), aica.CanalesSonando())
	}

	// 8) el cierre: los contadores tienen que volver a cero y los volumenes que el motor fijo seguir ahi
	bgm, sfx := proveedor.Volumenes()

	marca("RESUMEN efectos=%d errores=%d volumenes bgm=%.2f sfx=%.2f %s", proveedor.EfectosCargados(),
		proveedor.Errores(), bgm, sfx, "FIN-DE-LA-PRUEBA")

	estadoDelChip("DESPUES-DE-TODO")

	marca("fallos=%d", fallos)

	// el POC se queda vivo, latiendo: si el proceso se hubiera caido, el log se corta aca y se nota. El
	// emulador lo mata run-flycast.sh al terminar la corrida.
	for latido := 1; ; latido++ {
		esperarHasta(d2cMs() + 5000)

		marca("latido=%d canalesSonando=%d aicaLibre=%d", latido, aica.CanalesSonando(),
			aica.MemoriaLibre())
	}
}

// boolAint convierte un bool a 1/0 para las lineas (al lado de otros numeros se lee mejor que true/false).
func boolAint(b bool) int {
	if b {
		return 1
	}

	return 0
}

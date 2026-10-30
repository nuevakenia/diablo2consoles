//go:build dreamcast

package aica

import (
	"fmt"
	"unsafe"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2mpq"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2interface"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2audio/d2soundpath"
)

// EL PROVEEDOR: d2interface.AudioProvider sobre el AICA.
//
// La linea de abajo es el criterio de la tarjeta cumplido por el COMPILADOR y no por una lectura: si a este
// tipo le faltara uno de los tres metodos, o tuviera la firma cambiada, el paquete no compila.
var _ d2interface.AudioProvider = &Proveedor{}

// maximoMuestrasAICA es el tope de snd_sfx_load_raw_buf (snd_sfxmgr.h: "at most 65534 samples"). Un efecto
// mas largo NO se recorta en silencio: se devuelve un error, porque sonar medio efecto es peor que no sonar
// y no enterarse.
const maximoMuestrasAICA = 65534

// Proveedor es el AudioProvider de la consola. Se arma con Nuevo (que enciende el AICA) y a partir de ahi
// contesta el contrato del motor.
type Proveedor struct {
	// mpq es el archivo del disco donde viven los sonidos: d2Sfx.mpq (2330 WAV, medido). El motor del host
	// los pide al AssetManager; la consola no tiene sistema de archivos ni AssetManager cargado, asi que el
	// MPQ entra por aca, ya abierto.
	mpq *d2mpq.MPQ

	bgmVolumen float64
	sfxVolumen float64

	// ultimoBGM recuerda el pedido anterior (PlayBGM es un no-op, pero el log dice que se pidio dos veces
	// el mismo tema y no dos veces)
	ultimoBGM string

	efectos int
	errores int

	// Traza, si no es nil, recibe una linea por cada decision del proveedor (que sonó, que fallo, que se
	// ignoro). El paquete NO imprime solo: la consola serie la elige el que llama (el POC), y una libreria
	// que escribe en la salida de otro no se puede probar.
	Traza func(linea string)
}

// Nuevo enciende el AICA (snd_init) y devuelve el proveedor apuntado al MPQ de los sonidos. El `mpq` puede
// ser nil: en ese caso LoadSound falla con un error claro en vez de reventar, que es lo que necesita una
// prueba que solo mide canales.
func Nuevo(mpq *d2mpq.MPQ) *Proveedor {
	p := &Proveedor{
		mpq:        mpq,
		bgmVolumen: 1,
		sfxVolumen: 1,
	}

	codigo := Iniciar()
	p.traza("AICA init snd_init=%d aicaLibre=%d canalesLibres=%d canalesSonando=%d",
		codigo, MemoriaLibre(), CanalesLibres(), CanalesSonando())

	if codigo != 0 {
		p.traza("AICA INIT-FALLO codigo=%d: el chip no arranco, los efectos no van a sonar", codigo)
	}

	return p
}

// PlayBGM es un NO-OP DECLARADO: la musica del juego se toca por streaming desde el GD-ROM (snd_stream) y
// eso no entra en esta tarjeta (ver el cuerpo: "no metas streaming de BGM desde el GD-ROM"). El motor de la
// consola todavia no lo pide (las pantallas son la Fase D), asi que el metodo contesta y lo deja escrito en
// la traza. Se implementa cuando haya una pantalla que lo pida y una decision sobre el streaming.
func (p *Proveedor) PlayBGM(song string) {
	if song == p.ultimoBGM {
		return
	}

	p.ultimoBGM = song
	p.traza("AICA BGM pedido=%q IGNORADO (PlayBGM es no-op: la musica es streaming y no entra en DC-002.72)", song)
}

// SetVolumes guarda los volumenes maestros del motor (0..1). NO toca el chip: cada efecto los recibe al
// cargarse (LoadSound) o cuando el motor le llama SetVolume, igual que hace el proveedor de ebiten (que
// guarda los volumenes y se los pasa al player).
func (p *Proveedor) SetVolumes(bgmVolume, sfxVolume float64) {
	p.bgmVolumen = bgmVolume
	p.sfxVolumen = sfxVolume

	p.traza("AICA volumenes bgm=%.2f sfx=%.2f -> AICA bgm=%d sfx=%d (escala 0..255 con 128 al centro del pan)",
		bgmVolume, sfxVolume, VolumenAICA(bgmVolume), VolumenAICA(sfxVolume))
}

// Volumenes devuelve los volumenes maestros (0..1) que el motor fijo.
func (p *Proveedor) Volumenes() (bgm, sfx float64) { return p.bgmVolumen, p.sfxVolumen }

// EfectosCargados cuenta los efectos que este proveedor entrego bien (para la evidencia: no es lo mismo "el
// motor pidio 3" que "3 entraron al chip").
func (p *Proveedor) EfectosCargados() int { return p.efectos }

// Errores cuenta las cargas que fallaron.
func (p *Proveedor) Errores() int { return p.errores }

// LoadSound carga un efecto DEL MOTOR: el nombre es el de Sounds.txt (relativo a las carpetas de sonido) y
// los bytes salen del MPQ del disco por el mismo camino con que el motor resuelve la ruta
// (d2core/d2audio/d2soundpath, que es Go comun y ya esta probado contra los MPQ reales).
//
//	loop -> el AICA repite el efecto (snd_sfx_play_ex con loop=1)
//	bgm  -> de que volumen maestro sale (musica o efectos)
func (p *Proveedor) LoadSound(sfx string, loop bool, bgm bool) (d2interface.SoundEffect, error) {
	volumenMaestro := p.sfxVolumen
	if bgm {
		volumenMaestro = p.bgmVolumen
	}

	efecto, err := p.cargar(sfx, loop, volumenMaestro)
	if err != nil {
		p.errores++
		p.traza("AICA ERROR cargando %q: %v", sfx, err)

		return nil, err
	}

	p.efectos++

	return efecto, nil
}

// cargar hace el trabajo: resolver la ruta, leer el MPQ, parsear el WAV y subir el PCM al AICA.
func (p *Proveedor) cargar(sfx string, loop bool, volumenMaestro float64) (*Efecto, error) {
	if p.mpq == nil {
		return nil, fmt.Errorf("aica: no hay MPQ de sonidos (el proveedor se armo sin archivo)")
	}

	ruta, ok := d2soundpath.Resolve(sfx, func(candidata string) bool { return p.mpq.Contains(rutaEnElMPQ(candidata)) })
	if !ok {
		return nil, fmt.Errorf("aica: el sonido %q no esta en %s", sfx, p.mpq.Path())
	}

	clave := rutaEnElMPQ(ruta)

	datos, err := p.mpq.ReadFile(clave)
	if err != nil {
		return nil, fmt.Errorf("aica: leyendo %s de %s: %w", clave, p.mpq.Path(), err)
	}

	sonido, err := ParsearWAV(datos)
	if err != nil {
		return nil, fmt.Errorf("aica: %s (%d B): %w", ruta, len(datos), err)
	}

	if sonido.Muestras > maximoMuestrasAICA {
		return nil, fmt.Errorf("aica: %s trae %d muestras y el sfxmgr acepta %d",
			ruta, sonido.Muestras, maximoMuestrasAICA)
	}

	// el largo que se le pasa a la AICA tiene que ser multiplo de 4: spu_memload_sq redondea hacia arriba
	// (kernel/arch/dreamcast/hardware/spu.c:75-78) y leeria hasta 3 bytes de mas, fuera de nuestro buffer.
	// Se recortan las muestras SOBRANTES, no se rellena: el efecto termina una fraccion de trama antes.
	for largoDeLosDatos(sonido)%4 != 0 && sonido.Muestras > 1 {
		sonido.Muestras--
		sonido.PCM = sonido.PCM[:sonido.Muestras*sonido.BytesDeTrama()]
	}

	memoriaAntes := MemoriaLibre()

	handle := d2audioCargar(
		uintptr(unsafe.Pointer(&sonido.PCM[0])),
		len(sonido.PCM), sonido.Tasa, sonido.Bits, sonido.Canales,
	)

	memoriaDespues := MemoriaLibre()

	p.traza("AICA PCM ruta=%s claveMPQ=%s mpq=%s bytes=%d formato=%d tasa=%d bits=%d canales=%d muestras=%d ms=%d handle=%d aicaLibre=%d usado=%d",
		ruta, clave, p.mpq.Path(), len(datos), sonido.Formato, sonido.Tasa, sonido.Bits, sonido.Canales,
		sonido.Muestras, sonido.DuracionMs(), handle, memoriaDespues, memoriaAntes-memoriaDespues)

	if handle == 0 {
		return nil, fmt.Errorf("aica: el AICA no acepto %s (%d B de PCM, %d muestras, %d Hz, %d bits, %d canales)",
			ruta, len(sonido.PCM), sonido.Muestras, sonido.Tasa, sonido.Bits, sonido.Canales)
	}

	efecto := &Efecto{
		proveedor: p,
		ruta:      ruta,
		sonido:    sonido,
		handle:    handle,
		canal:     -1,
		loop:      loop,
		escala:    volumenMaestro,
		pan:       PanCentro,
	}

	// el volumen con que queda cargado: el maestro del proveedor (como el proveedor de ebiten, que hace
	// result.volumeScale = volume y despues SetVolume(volume))
	efecto.SetVolume(1)

	return efecto, nil
}

// largoDeLosDatos es el largo en bytes del PCM que se sube al AICA.
func largoDeLosDatos(s *Sonido) int { return len(s.PCM) }

// traza escribe una linea si hay a quien (el paquete no imprime solo).
func (p *Proveedor) traza(formato string, args ...interface{}) {
	if p.Traza != nil {
		p.Traza(fmt.Sprintf(formato, args...))
	}
}

// ============================== EL EFECTO ==============================

// la linea que exige el contrato de d2interface.SoundEffect
var _ d2interface.SoundEffect = &Efecto{}

// Efecto es un sonido cargado en la RAM del AICA y el canal donde suena. Cumple d2interface.SoundEffect.
type Efecto struct {
	proveedor *Proveedor
	ruta      string
	sonido    *Sonido
	handle    uint

	// canal es el canal del AICA que dio el sfxmgr, o -1 si nunca sono (o si ya se paro). Se guarda para
	// poder preguntarle al CHIP si sigue sonando, que es la unica respuesta que vale.
	canal int

	loop   bool
	escala float64 // volumen maestro del proveedor cuando se cargo

	volumen int // registro del AICA (0..255)
	pan     int // registro del AICA (0..255, 128 centro)

	reproducciones int
}

// Play dispara el efecto en un canal libre del AICA (snd_sfx_play_ex). Si ya estaba sonando, lo corta antes:
// dos canales con el mismo efecto es un doble, no un reinicio.
func (e *Efecto) Play() {
	if e.canal >= 0 && Sonando(e.canal) {
		d2audioParar(e.canal)
	}

	loop := 0
	if e.loop {
		loop = 1
	}

	e.canal = d2audioReproducir(e.handle, e.volumen, e.pan, loop)
	e.reproducciones++

	e.proveedor.traza("AICA play ruta=%s canal=%d vol=%d pan=%d loop=%d sonando=%d canalesSonando=%d",
		e.ruta, e.canal, e.volumen, e.pan, loop, boolAint(Sonando(e.canal)), CanalesSonando())
}

// Stop corta el canal del AICA donde esta sonando este efecto.
func (e *Efecto) Stop() {
	if e.canal < 0 {
		return
	}

	estaba := Sonando(e.canal)

	d2audioParar(e.canal)

	e.proveedor.traza("AICA stop ruta=%s canal=%d estabaSonando=%d canalesSonando=%d",
		e.ruta, e.canal, boolAint(estaba), CanalesSonando())
}

// IsPlaying pregunta al CHIP si el canal sigue encendido (KEYONB), no a una copia nuestra: cuando el AICA
// termina el efecto por si mismo, esto pasa a false sin que nadie lo avise.
func (e *Efecto) IsPlaying() bool { return e.canal >= 0 && Sonando(e.canal) }

// SetVolume recibe el volumen del motor (0..1) y lo aplica sobre la escala maestra que se fijo al cargar (el
// motor pasa el volumen del efecto y el proveedor el maestro; el proveedor de ebiten multiplica los dos).
// Si el efecto esta sonando, el cambio va al chip en el acto con un paquete de ACTUALIZACION.
func (e *Efecto) SetVolume(volumen float64) {
	e.volumen = VolumenAICA(volumen * e.escala)

	if e.canal >= 0 && Sonando(e.canal) {
		d2audioVolumen(e.canal, e.volumen)
	}
}

// SetPan recibe el pan del motor (-1 izquierda .. 0 centro .. +1 derecha) y lo lleva a 0..255 con 128 al
// centro. Si el efecto esta sonando, el cambio va al chip en el acto (solo los efectos MONO tienen pan: en
// estereo el sfxmgr pone 0 y 255 a los dos canales, snd_sfxmgr.c:757).
func (e *Efecto) SetPan(pan float64) {
	e.pan = PanAICA(pan)

	if e.canal >= 0 && Sonando(e.canal) {
		d2audioPan(e.canal, e.pan)
	}
}

// los instrumentos del efecto (no son parte del contrato; los usa la evidencia del POC)

// Ruta es la ruta del archivo DENTRO del MPQ de donde salieron los bytes.
func (e *Efecto) Ruta() string { return e.ruta }

// Sonido es el WAV ya parseado (tasa, bits, canales, muestras, duracion).
func (e *Efecto) Sonido() *Sonido { return e.sonido }

// Handle es el handle del efecto en el sfxmgr (direccion en la RAM del AICA).
func (e *Efecto) Handle() uint { return e.handle }

// Canal es el canal donde sono por ultima vez (-1 = nunca).
func (e *Efecto) Canal() int { return e.canal }

// Volumen es el registro de volumen con el que esta cargado (0..255).
func (e *Efecto) Volumen() int { return e.volumen }

// Pan es el registro de pan con el que esta cargado (0..255, 128 centro).
func (e *Efecto) Pan() int { return e.pan }

// Reproducciones cuenta los Play de este efecto.
func (e *Efecto) Reproducciones() int { return e.reproducciones }

// boolAint convierte un bool a 1/0 para las lineas de la traza (la consola tiene fmt, pero %t en una linea
// larga se lee peor que un numero al lado de otros).
func boolAint(b bool) int {
	if b {
		return 1
	}

	return 0
}

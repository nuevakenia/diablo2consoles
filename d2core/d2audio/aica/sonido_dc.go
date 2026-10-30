//go:build dreamcast

package aica

import (
	"unsafe"
)

// ======================= LO QUE SOLO EXISTE EN LA CONSOLA =======================
//
// El binding de Go de libgodc NO expone nada del sonido de KOS (ni snd_init, ni el sfxmgr, ni el registro de
// los canales). Las funciones de abajo son la cara Go de los puentes en C de
// console/dc/std/csrc/sonido_dc.c, que build-dc.sh compila y enlaza SIEMPRE (son shims compartidos, como
// control_dc.c del maple): un paquete del motor que las llame por //extern no trae copia propia.
//
// POR QUE LOS PUNTEROS CRUZAN COMO uintptr: un unsafe.Pointer pasado a un //extern tiene el ABI ambiguo
// (medido en DC-002.9/22) y ademas el runtime de la consola no tiene el descriptor del tipo unsafe.Pointer,
// asi que guardarlo en un campo rompe el enlace. Se convierte a puntero SOLO en la llamada.

// el arranque del chip, la RAM del AICA y la carga/el play de un efecto
//
//extern d2audio_iniciar
func d2audioIniciar() int

//extern d2audio_mem_libre
func d2audioMemLibre() uint

//extern d2audio_cargar
func d2audioCargar(buf uintptr, largo, tasa, bits, canales int) uint

//extern d2audio_reproducir
func d2audioReproducir(handle uint, volumen, pan, loop int) int

//extern d2audio_parar
func d2audioParar(canal int)

// los instrumentos: el estado del canal en el CHIP, no en una copia nuestra
//
//extern d2audio_sonando
func d2audioSonando(canal int) int

//extern d2audio_posicion
func d2audioPosicion(canal int) int

//extern d2audio_canales_sonando
func d2audioCanalesSonando() int

//extern d2audio_canales_libres
func d2audioCanalesLibres() int

// el volumen y el pan de un canal que YA esta sonando (paquete de ACTUALIZACION al AICA)
//
//extern d2audio_volumen
func d2audioVolumen(canal, volumen int)

//extern d2audio_pan
func d2audioPan(canal, pan int)

//extern d2audio_volcar
func d2audioVolcar(dst uintptr, canal int)

// Iniciar enciende el AICA: carga su programa y arranca su monton de RAM (AICA_RAM_START de los 2 MB del
// SPU). Devuelve lo mismo que snd_init: 0 = bien. Llamarlo dos veces no es un error de KOS, pero este
// paquete lo hace una sola vez (Nuevo).
func Iniciar() int { return d2audioIniciar() }

// MemoriaLibre es lo que queda del monton del AICA, en bytes. Es el instrumento de "el efecto entro": un
// WAV que se carga deja menos RAM libre en el SPU.
func MemoriaLibre() int { return int(d2audioMemLibre()) }

// CanalesSonando cuenta los canales del AICA que estan sonando AHORA, leidos del registro del chip
// (`snd_is_playing`: el bit KEYONB de cada canal, snd_iface.c). Es el contador honesto de "ocupado/libre":
// no cuenta llamadas, cuenta lo que el chip tiene encendido.
func CanalesSonando() int { return d2audioCanalesSonando() }

// CanalesLibres cuenta los canales que el sfxmgr PUEDE prestar ahora: se piden de a uno con
// snd_sfx_chn_alloc y se devuelven, asi el numero es el que queda de verdad.
//
// OJO, MEDIDO (por eso existe tambien CanalesSonando): `snd_sfx_play` NO marca el canal como ocupado en la
// tabla del sfxmgr (snd_sfxmgr.c:757-764 llama a snd_sfx_play_ex, que solo pide el canal con
// find_free_channel sin tocar sfx_inuse). O sea que este contador vale 64 aunque haya efectos sonando: sirve
// para saber si alguien RESERVO canales para otra cosa (el stream de musica), no para ver un efecto.
func CanalesLibres() int { return d2audioCanalesLibres() }

// EstadoDelCanal es el registro de 64 B que el AICA tiene cargado para un canal (aica_channel_t). La zona
// AICA_MEM_CHANNELS la escribe el programa del AICA y es de SOLO LECTURA desde el SH-4: lo que se lee aca es
// lo que el chip tiene cargado, no una copia nuestra.
type EstadoDelCanal struct {
	Cmd        int
	Base       int
	Tipo       int
	Largo      int
	Loop       int
	Frecuencia int
	Volumen    int
	Pan        int
	Posicion   int
}

// Estado lee el registro del canal `canal` (0..63) del AICA.
func Estado(canal int) EstadoDelCanal {
	var regs [16]uint32

	d2audioVolcar(uintptr(unsafe.Pointer(&regs[0])), canal)

	return EstadoDelCanal{
		Cmd:        int(regs[0]),
		Base:       int(regs[1]),
		Tipo:       int(regs[2]),
		Largo:      int(regs[3]),
		Loop:       int(regs[4]),
		Frecuencia: int(regs[7]),
		Volumen:    int(regs[8]),
		Pan:        int(regs[9]),
		Posicion:   int(regs[10]),
	}
}

// Sonando dice si el chip tiene el canal andando (KEYONB). Es la misma lectura que usa Efecto.IsPlaying.
func Sonando(canal int) bool { return d2audioSonando(canal) != 0 }

// Posicion es la posicion del canal dentro del efecto, en muestras. En Flycast el firmware actualiza el
// contador (`snd_get_pos` lee la memoria del SPU): si se queda en 0, el canal no esta avanzando.
func Posicion(canal int) int { return d2audioPosicion(canal) }

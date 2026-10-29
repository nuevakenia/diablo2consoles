//go:build dreamcast

package main

// EL PRIMER SONIDO DEL POC, POR EL AICA (DC-002.62).
//
// QUE HACE: enciende el audio de KOS (`snd_init`), carga DOS efectos cortos en PCM -- el GOLPE y el
// RECOGER -- en la RAM del AICA (no en la RAM principal: los samples viven en el SPU) y los dispara en
// los dos momentos que YA existen en el POC: el boton Y de DC-002.50 y la recogida de DC-002.61.
//
// DE DONDE SALEN LOS PCM: los genera el BUILD (`scripts/console/gen-pcm-262.py`), porque el
// `d2sfx.mpq` -- el MPQ donde viven los sonidos del juego -- NO viaja en el disco del POC: la ISO
// lleva D2DATA, D2CHAR y PATCH_D2 (medido en scripts/console/build-monstruo-250.sh). El generador
// escribe `gen/sonidos_gen.go` con las dos muestras y las deja en las variables de abajo desde su
// `init()`. Asi este archivo compila TAMBIEN en los builds que no generan PCM (build-items-261,
// build-monstruo-250): los PCM quedan nil y `iniciarSonido` lo informa y no toca el AICA.
//
// EL CAMINO ES EL DE KOS, sin logica propia: snd_init -> snd_sfx_load_raw_buf (samples a la RAM del
// AICA) -> snd_sfx_play (un canal del AICA de los 64). Lo unico que pone el POC es la cara C de esas
// funciones (csrc/cshim.c: el binding de Go de libgodc no expone el sonido) y el contador.
//
// LOS DOS INTERRUPTORES (misma forma que `items`/`bestia`, ver objetos.go y main.go):
//
//   var sonido = false       -> true: el AICA arranca, los dos PCM se cargan y los dos momentos suenan.
//   var sonidoSordo = false  -> true: EL MUTANTE. Los dos momentos se alcanzan y se informan
//                               (`SND sordo id=...`) pero el AICA NO recibe un solo play: el contador
//                               queda en 0. Sirve para probar que el `total` sale de reproducir de
//                               verdad, no de contar llamadas.
//   var sonidoAuto = false   -> INSTRUMENTO: el golpe ya tiene el suyo (`golpeAuto`, DC-002.50) y la
//                               recogida el suyo (`itemsAuto`, DC-002.61), asi que este POC no necesita
//                               otro: el sonido se mide por los caminos que ya estaban medidos.

import (
	"unsafe"
)

// sonido: ENCIENDE el audio del POC. Arranca APAGADO a proposito: con esto apagado el binario es el de
// DC-002.61 (mapa, personaje, bicho, golpe e items) y no toca el AICA ni una vez. Lo enciende
// `scripts/console/build-sonido-262.sh` con un sed sobre la COPIA del POC, igual que `items` y `bestia`.
var sonido = false

// sonidoSordo: EL MUTANTE del criterio 3. Los dos momentos se alcanzan (los botones se leen por los
// caminos de DC-002.50/DC-002.61) pero el efecto NO se dispara: el contador queda en 0.
var sonidoSordo = false

// los dos efectos. 1 = GOLPE (boton Y), 2 = RECOGER (levantar un item del suelo).
const (
	idSonidoGolpe   = 1
	idSonidoRecoger = 2
)

// La forma de los PCM y los propios bytes los declara `gen/sonidos_gen.go` (lo escribe el generador del
// build). Se declaran aca como nil/0 para que este archivo compile sin el generador.
var (
	pcmGolpe   []byte
	pcmRecoger []byte

	pcmTasa            = 0
	pcmBits            = 0
	pcmCanales         = 0
	pcmMuestrasGolpe   = 0
	pcmMuestrasRecoger = 0
)

// EL VOLUMEN Y EL PAN: la escala del sfxmgr es 0..255 (snd_sfxmgr.h:snd_sfx_play) y 128 es el centro
// del pan. Se eligen fijos: esta tarjeta no mezcla ni posiciona nada (eso NO entra, ver el cuerpo).
const (
	volumenSonido = 200
	panSonido     = 128
)

// El estado del sonido: los dos handles del AICA y el contador de la evidencia.
var (
	sonidoListo bool
	hndGolpe    uint
	hndRecoger  uint

	sonidosTotales int
	sonidosError   int
)

// Los puentes a C (csrc/cshim.c). El binding de Go de libgodc no expone NADA del sonido de KOS.
//
//extern d2c_snd_init
func d2cSndInit() int

//extern d2c_snd_canales_libres
func d2cSndCanalesLibres() int

//extern d2c_snd_mem_libre
func d2cSndMemLibre() uint

//extern d2c_snd_cargar
func d2cSndCargar(buf uintptr, largo, tasa, bits, canales int) uint

//extern d2c_snd_play
func d2cSndPlay(hnd uint, vol, pan int) int

//extern d2c_snd_dump
func d2cSndDump(dst uintptr, canal int)

// iniciarSonido: enciende el AICA, carga los dos efectos y volca el registro del canal 0 ANTES de
// sonar. Se llama UNA vez, con el video ya andando (misma regla que los items y los ciclos del bicho:
// lo que crece antes de `IniciarPantalla` deja al PVR sin sus 70 KB, DC-002.50).
func iniciarSonido() {
	if !sonido {
		println("SND apagado (sonido=false): el POC no inicializa el AICA")

		return
	}

	memoria("antes-del-sonido")

	codigo := d2cSndInit()
	canales := d2cSndCanalesLibres()
	ramAica := d2cSndMemLibre()

	println("SND init snd_init=" + numeroATexto(codigo) + " canalesLibres=" + numeroATexto(canales) +
		" aicaLibre=" + numeroATexto(int(ramAica)))

	if codigo != 0 {
		println("SND INIT-FALLO codigo=" + numeroATexto(codigo))

		return
	}

	if len(pcmGolpe) == 0 || len(pcmRecoger) == 0 {
		println("SND sin-PCM (el build no corrio gen-pcm-262.py): el POC sigue mudo")

		return
	}

	hndGolpe = d2cSndCargar(uintptrOf(pcmGolpe), len(pcmGolpe), pcmTasa, pcmBits, pcmCanales)
	hndRecoger = d2cSndCargar(uintptrOf(pcmRecoger), len(pcmRecoger), pcmTasa, pcmBits, pcmCanales)

	ramDespues := d2cSndMemLibre()

	println("SND pcm id=1 nombre=GOLPE bytes=" + numeroATexto(len(pcmGolpe)) +
		" muestras=" + numeroATexto(pcmMuestrasGolpe) + " tasa=" + numeroATexto(pcmTasa) +
		" bits=" + numeroATexto(pcmBits) + " canales=" + numeroATexto(pcmCanales) +
		" hnd=" + uintATexto(hndGolpe))
	println("SND pcm id=2 nombre=RECOGER bytes=" + numeroATexto(len(pcmRecoger)) +
		" muestras=" + numeroATexto(pcmMuestrasRecoger) + " tasa=" + numeroATexto(pcmTasa) +
		" bits=" + numeroATexto(pcmBits) + " canales=" + numeroATexto(pcmCanales) +
		" hnd=" + uintATexto(hndRecoger))
	println("SND aica libreAntes=" + numeroATexto(int(ramAica)) + " libreDespues=" +
		numeroATexto(int(ramDespues)) + " usado=" + numeroATexto(int(ramAica-ramDespues)))

	if hndGolpe == 0 || hndRecoger == 0 {
		println("SND CARGA-FALLO hndGolpe=" + uintATexto(hndGolpe) + " hndRecoger=" +
			uintATexto(hndRecoger))

		return
	}

	// el registro del AICA ANTES de que suene nada: los 64 B del canal 0 (lo que el chip tiene cargado)
	volcarAica("antes-del-primer-play", 0)

	sonidoListo = true

	memoria("sonido-listo")

	println("SND listo volumen=" + numeroATexto(volumenSonido) + " pan=" + numeroATexto(panSonido) +
		" sordo=" + boolATexto(sonidoSordo))
}

// sonar: dispara un efecto. Es el UNICO lugar del POC que le pide un play al AICA, y el unico que mueve
// el contador de la evidencia: `SND play id=<n> canal=<c> total=<T> errores=<E>`.
func sonar(id int) {
	if !sonido {
		return
	}

	if sonidoSordo {
		println("SND sordo id=" + numeroATexto(id) + " total=" + numeroATexto(sonidosTotales) +
			" (el momento se alcanzo, el AICA no recibe el play)")

		return
	}

	var hnd uint

	switch id {
	case idSonidoGolpe:
		hnd = hndGolpe
	case idSonidoRecoger:
		hnd = hndRecoger
	}

	if hnd == 0 {
		sonidosError++

		println("SND ERROR id=" + numeroATexto(id) + " sin-handle errores=" + numeroATexto(sonidosError))

		return
	}

	canal := d2cSndPlay(hnd, volumenSonido, panSonido)
	if canal < 0 {
		sonidosError++

		println("SND ERROR id=" + numeroATexto(id) + " sin-canal errores=" + numeroATexto(sonidosError))

		return
	}

	sonidosTotales++

	println("SND play id=" + numeroATexto(id) + " canal=" + numeroATexto(canal) +
		" total=" + numeroATexto(sonidosTotales) + " errores=" + numeroATexto(sonidosError))

	// la prueba de que el chip se movio: el registro del canal DESPUES del primer play
	if sonidosTotales == 1 {
		volcarAica("despues-del-primer-play", canal)
	}
}

// volcarAica: los 64 B del registro de estado de un canal del AICA (aica_channel_t: cmd, base, type,
// length, loop, loopstart, loopend, freq, vol, pan, pos). La zona AICA_MEM_CHANNELS la escribe el
// programa del AICA y es de SOLO LECTURA desde el SH-4: si estos numeros cambian entre el volcado de
// antes y el de despues, el chip cargo el efecto de verdad.
func volcarAica(etiqueta string, canal int) {
	var regs [16]uint32

	d2cSndDump(uintptr(unsafe.Pointer(&regs[0])), canal)

	println("SND aica etiqueta=" + etiqueta + " canal=" + numeroATexto(canal) +
		" cmd=" + numeroATexto(int(regs[0])) +
		" base=" + numeroATexto(int(regs[1])) +
		" type=" + numeroATexto(int(regs[2])) +
		" largo=" + numeroATexto(int(regs[3])) +
		" loop=" + numeroATexto(int(regs[4])) +
		" freq=" + numeroATexto(int(regs[7])) +
		" vol=" + numeroATexto(int(regs[8])) +
		" pan=" + numeroATexto(int(regs[9])) +
		" pos=" + numeroATexto(int(regs[10])))
}

// boolATexto: "true"/"false" (para las marcas del log).
func boolATexto(b bool) string {
	if b {
		return "true"
	}

	return "false"
}

// uintATexto: un numero SIN SIGNO, para los handles del AICA (son direcciones del monton de C: como
// `int` de 32 bits salen negativos y en la evidencia eso se lee como un error cuando no lo es).
func uintATexto(n uint) string {
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

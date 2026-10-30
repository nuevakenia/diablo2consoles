// Package aica implements d2interface.AudioProvider over the Dreamcast's AICA sound chip, using the
// sfxmgr of KOS.
//
// QUE HACE: es la cara de la consola del contrato de audio del motor. El motor (d2core/d2audio/sound_engine.go,
// los menus, los monstruos) no sabe en que plataforma corre: pide `LoadSound(nombre, loop, bgm)` y recibe un
// d2interface.SoundEffect con Play/Stop/SetPan/SetVolume/IsPlaying.
//
//	prov := aica.Nuevo(mpq)                          // enciende el AICA (snd_init) y guarda el MPQ de los sonidos
//	var audio d2interface.AudioProvider = prov       // el contrato: 3 metodos
//	ef, err := audio.LoadSound("combat\\impact\\sword5.wav", false, false)
//	ef.Play()                                        // un canal del AICA de los 64
//
// EL CAMINO ES EL DE KOS, el mismo que ya suena en el POC del personaje (console/dc/poc/personaje-mapa/
// sonido.go, DC-002.62): snd_sfx_load_raw_buf sube el PCM a la RAM del AICA (fuera de la RAM principal) y
// snd_sfx_play pide un canal. Lo que agrega este paquete es la INTERFAZ, el parseo del WAV del motor
// (wav.go, probado en el host) y la traduccion de volumenes (volumen.go, tambien probada en el host).
//
// QUE NO HACE, y esta declarado a proposito:
//   - PlayBGM es un NO-OP: la musica del juego es streaming desde el GD-ROM (snd_stream) y no entra en esta
//     tarjeta. El motor todavia no lo pide en la consola (la Fase D es la que arma las pantallas).
//   - No implementa d2interface.PCMStreamer (las cinemáticas de Bink): tambien es streaming.
//
// LO ESPECIFICO DE LA CONSOLA VIVE EN sonido_dc.go (`//go:build dreamcast`): las llamadas a C de KOS, que el
// binding de Go de libgodc no expone. El resto del paquete es Go comun y se prueba en el host.
package aica

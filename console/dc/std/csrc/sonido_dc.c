/* DC-002.72: el camino al AICA de KOS, para la interfaz de audio del motor.
 *
 * Por que en C: el binding de Go de libgodc no expone NADA del sonido (ni snd_init, ni el sfxmgr, ni
 * snd_is_playing). Estas funciones son la cara C del camino de KOS, tal cual: cada una llama a la de KOS y
 * devuelve su resultado, sin logica propia mas alla de recorrer los 64 canales o armar el paquete de
 * ACTUALIZACION de volumen/pan.
 *
 * Vive en console/dc/std/csrc/ (y no en el csrc de un POC) porque la usa un paquete del MOTOR:
 * d2core/d2audio/aica la llama por //extern. build-dc.sh compila este directorio SIEMPRE, asi que cualquier
 * POC que importe el paquete del audio tiene el camino sin traer copia. Es el mismo criterio que
 * control_dc.c con el maple (DC-002.71): el hardware, en un solo lugar.
 *
 * EL CAMINO: snd_init() carga el programa del AICA y arranca su monton de RAM (AICA_RAM_START = 0x030000 de
 * los 2 MB del SPU; snd_iface.c). snd_sfx_load_raw_buf() copia el PCM a la RAM del AICA, fuera de la RAM
 * principal, y snd_sfx_play_ex() le pide un canal al sfxmgr (64 canales).
 *
 * MEDIDO en el codigo de KOS (no supuesto): el PCM de 16 bits se guarda con la longitud en MUESTRAS
 * (effect->len = (len/2)/canales, snd_sfxmgr.c:661), o sea que el `len` que se le pasa es en BYTES, y los
 * samples viajan con spu_memload_sq, que redondea el largo hacia arriba a multiplo de 4 (spu.c:75-78): por eso
 * el que llama recorta el PCM a multiplo de 4 antes de subirlo.
 */
#include <kos.h>
#include <dc/sound/sound.h>
#include <dc/sound/sfxmgr.h>
#include <dc/sound/aica_comm.h>
#include <stdint.h>

/* d2audio_iniciar: enciende el AICA. Devuelve lo mismo que snd_init (0 = bien). */
int d2audio_iniciar(void) {
    return snd_init();
}

/* d2audio_mem_libre: cuanto queda del monton del AICA (bytes). Es el instrumento de "el efecto entro". */
unsigned int d2audio_mem_libre(void) {
    return (unsigned int)snd_mem_available();
}

/* d2audio_cargar: sube un bloque PCM crudo a la RAM del AICA y devuelve el handle (0 = fallo). */
unsigned int d2audio_cargar(unsigned int buf, int largo, int tasa, int bits, int canales) {
    return (unsigned int)snd_sfx_load_raw_buf((char *)(uintptr_t)buf, (size_t)largo,
        (uint32_t)tasa, (uint16_t)bits, (uint16_t)canales);
}

/* d2audio_reproducir: dispara un efecto (con loop si se pide) y devuelve el canal que le toco (-1 = no hay
 * canal libre). Va por snd_sfx_play_ex y no por snd_sfx_play porque el contrato del motor tiene `loop`
 * (d2interface.AudioProvider.LoadSound(sfx, loop, bgm)).
 *
 * OJO, medido: el sfxmgr NO marca el canal como ocupado en su tabla (snd_sfx_play_ex solo llama a
 * find_free_channel, snd_sfxmgr.c:767-771): el canal lo rota el propio sfxmgr. Por eso el estado "esta
 * sonando" se pregunta al CHIP (d2audio_sonando) y no a esa tabla. */
int d2audio_reproducir(unsigned int handle, int volumen, int pan, int loop) {
    sfx_play_data_t datos;
    int i;

    /* el stack no tiene basura: se limpia entero (la estructura tiene loopstart/loopend/freq, y con
     * loopend = 0 el sfxmgr usa el largo del efecto como fin del loop) */
    for(i = 0; i < (int)sizeof(datos); i++)
        ((char *)&datos)[i] = 0;

    datos.chn = -1;                 /* que el sfxmgr elija el canal */
    datos.idx = (sfxhnd_t)handle;
    datos.vol = volumen;
    datos.pan = pan;
    datos.loop = loop ? 1 : 0;

    return snd_sfx_play_ex(&datos);
}

/* d2audio_parar: corta el canal (AICA_CH_CMD_STOP). */
void d2audio_parar(int canal) {
    if(canal < 0 || canal > 63)
        return;

    snd_sfx_stop(canal);
}

/* d2audio_sonando: dice si el canal esta ENCENDIDO en el chip AHORA.
 *
 * snd_is_playing lee el registro del AICA por el bus G2 (snd_iface.c:219: KEYONB del canal). Es el unico
 * instrumento que no miente: el chip borra ese bit el solo cuando el efecto se termina, sin que el SH-4
 * haga nada. */
int d2audio_sonando(int canal) {
    if(canal < 0 || canal > 63)
        return 0;

    return snd_is_playing((unsigned int)canal) ? 1 : 0;
}

/* d2audio_posicion: la posicion del canal dentro del efecto, en muestras (snd_get_pos lee la memoria del
 * SPU, que el firmware actualiza). En Flycast ese contador lo mantiene el emulador del AICA. */
int d2audio_posicion(int canal) {
    if(canal < 0 || canal > 63)
        return -1;

    return (int)snd_get_pos((unsigned int)canal);
}

/* d2audio_canales_sonando: cuantos de los 64 canales estan sonando, leidos del chip. ESTE es el contador de
 * "ocupado/libre" de la evidencia: antes del play da 0, mientras suena da 1, y cuando el efecto termina el
 * propio chip lo apaga y vuelve a 0. */
int d2audio_canales_sonando(void) {
    int n = 0;

    for(int canal = 0; canal < 64; canal++)
        if(snd_is_playing((unsigned int)canal))
            n++;

    return n;
}

/* d2audio_canales_libres: los canales que el sfxmgr puede prestar AHORA. Se piden de a uno y se devuelven,
 * asi el numero es el que queda de verdad. (Con efectos sonando sigue dando 64: ver el comentario de
 * d2audio_reproducir. Sirve para ver canales RESERVADOS, p.ej. por el stream de musica.) */
int d2audio_canales_libres(void) {
    int tomados[64];
    int n = 0;

    while(n < 64) {
        int chn = snd_sfx_chn_alloc();
        if(chn < 0)
            break;
        tomados[n++] = chn;
    }

    for(int i = 0; i < n; i++)
        snd_sfx_chn_free(tomados[i]);

    return n;
}

/* d2audio_volumen: cambia el volumen de un canal QUE YA ESTA SONANDO, con el mismo paquete de ACTUALIZACION
 * que usa el stream de KOS para la musica (snd_stream.c:snd_stream_volume). Sin esto, SetVolume sobre un
 * efecto en curso no tendria efecto hasta el proximo play. */
void d2audio_volumen(int canal, int volumen) {
    AICA_CMDSTR_CHANNEL(tmp, cmd, chan);

    if(canal < 0 || canal > 63)
        return;

    cmd->cmd = AICA_CMD_CHAN;
    cmd->timestamp = 0;
    cmd->size = AICA_CMDSTR_CHANNEL_SIZE;
    cmd->cmd_id = canal;
    chan->cmd = AICA_CH_CMD_UPDATE | AICA_CH_UPDATE_SET_VOL;
    chan->vol = volumen;
    snd_sh4_to_aica(tmp, cmd->size);
}

/* d2audio_pan: lo mismo con el pan (0 izquierda, 128 centro, 255 derecha). Solo tiene sentido en efectos
 * MONO: en estereo el sfxmgr manda 0 y 255 a los dos canales. */
void d2audio_pan(int canal, int pan) {
    AICA_CMDSTR_CHANNEL(tmp, cmd, chan);

    if(canal < 0 || canal > 63)
        return;

    cmd->cmd = AICA_CMD_CHAN;
    cmd->timestamp = 0;
    cmd->size = AICA_CMDSTR_CHANNEL_SIZE;
    cmd->cmd_id = canal;
    chan->cmd = AICA_CH_CMD_UPDATE | AICA_CH_UPDATE_SET_PAN;
    chan->pan = pan;
    snd_sh4_to_aica(tmp, cmd->size);
}

/* d2audio_volcar: volca el registro de estado de un canal del AICA (64 B: cmd, base, type, length, loop,
 * loopstart, loopend, freq, vol, pan, pos) en `dst`. La zona AICA_MEM_CHANNELS la escribe el programa del
 * AICA y es de SOLO LECTURA desde el SH-4 (aica_cmd_iface.h), asi que leerla es leer lo que el chip tiene
 * cargado.
 *
 * OJO: el offset NO se toma de AICA_CHANNEL() porque ese macro vive en el header del programa del AICA, que
 * KOS no instala en $KOS_BASE/include. Los dos numeros van aca, con su fuente: AICA_MEM_CHANNELS = 0x020000
 * (aica_cmd_iface.h) y 64 B por canal (aica_comm.h:aica_channel_t). */
#define D2AUDIO_AICA_MEM_CHANNELS 0x020000
#define D2AUDIO_AICA_CANAL(n)     (D2AUDIO_AICA_MEM_CHANNELS + (n) * 64)

void d2audio_volcar(unsigned int dst, int canal) {
    if(dst == 0 || canal < 0 || canal > 63)
        return;

    spu_memread((void *)(uintptr_t)dst, D2AUDIO_AICA_CANAL(canal), sizeof(aica_channel_t));
}

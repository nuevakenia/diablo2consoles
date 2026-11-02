/* El control de la Dreamcast: leerlo para que el motor lo vea como un mouse y un teclado.
 *
 * EL PROBLEMA: el motor de Diablo II es de PC. Pregunta por el MOUSE (donde esta el cursor, que boton esta
 * apretado) y por el TECLADO (las flechas para los menus, Escape, Enter). En la Dreamcast no hay ni mouse ni
 * teclado: hay un control con una cruceta, una palanca y botones.
 *
 * LA SOLUCION ES LA MISMA QUE CON EL DIBUJO: la consola le MIENTE con la misma cara que la PC. La palanca
 * mueve el CURSOR del motor, el boton A es el clic izquierdo (que en Diablo II es "camina hasta aca y pelea
 * con eso"), el boton B es el derecho (la skill), y la cruceta son las flechas del teclado. El motor no se
 * entera de quien aprieta.
 *
 * COMO SE LEE UN CONTROL EN KOS: hay un bus (maple) donde estan enchufados los controles. Se pide el que
 * este en el puerto 0 con maple_enum_type, y su estado con maple_dev_status (que es el que espera a que el
 * estado sea valido, asi que no devuelve una lectura a medio hacer). El estado trae los botones en un campo
 * de bits y los ejes en un rango de -128 a 127.
 *
 * OJO CON UNA COSA QUE PARECE UN DETALLE Y NO LO ES: buscar el control es una sola vez (la primera), pero
 * LEERLO es todas las veces. Si se guardara el puntero y el control se desenchufara, el puntero quedaria
 * apuntando a un aparato que ya no esta; por eso cada lectura vuelve a preguntar si el aparato sigue ahi.
 */

#include <dc/maple.h>
#include <dc/maple/controller.h>

/* el control del puerto 0, y si ya se lo busco alguna vez */
static maple_device_t *control = NULL;
static int ya_se_busco = 0;

/* ============================== DC-002.113: EL ESTADO INYECTADO ==============================
 *
 * POR QUE. La corrida del criterio de DC-002.113 aprieta la cruceta con el teclado virtual del host y es FLYCAST
 * el que arma el estado en el bus maple: ahi esto queda APAGADO y no cambia nada. Pero si el emulador no toma ese
 * teclado, la prueba tiene que poder correr igual, y el camino es el shim: el estado se deja a un costado y
 * `d2c_control_leer` lo devuelve en vez de leer el bus. El estado entra por la MISMA puerta que un control de
 * verdad (el servicio lo traduce bits -> teclas y el juego pregunta por las teclas): lo unico que cambia es de
 * donde salieron los bits, y el POC lo imprime en la evidencia.
 *
 * APAGADO POR OMISION: `inyectando = 0`, asi que ningun otro POC cambia de comportamiento por este archivo.
 */
static int inyectando = 0;
static cont_state_t estado_inyectado;

/* d2c_control_inyectar: deja el estado inyectado y, si `encendido` es distinto de cero, lo enciende (o lo apaga
 * con 0). Devuelve 1 si quedo encendido. */
int d2c_control_inyectar(unsigned int botones, int palanca_x, int palanca_y,
                         int gatillo_l, int gatillo_r, int encendido) {
    estado_inyectado.buttons = botones;
    estado_inyectado.joyx = palanca_x;
    estado_inyectado.joyy = palanca_y;
    estado_inyectado.ltrig = gatillo_l;
    estado_inyectado.rtrig = gatillo_r;
    estado_inyectado.joy2x = 0;
    estado_inyectado.joy2y = 0;

    inyectando = encendido != 0 ? 1 : 0;

    return inyectando;
}

/* d2c_control_inyectando: si el estado lo esta poniendo el shim. */
int d2c_control_inyectando(void) {
    return inyectando;
}

/* d2c_control_buscar: busca el control en el puerto 0. Devuelve 1 si hay control y 0 si no.
 * Se busca UNA vez (maple_enum_type recorre el bus), no en cada cuadro. Con el estado inyectado contesta que si:
 * para el servicio es un control mas. */
int d2c_control_buscar(void) {
    if (inyectando) {
        return 1;
    }

    if (!ya_se_busco) {
        control = maple_enum_type(0, MAPLE_FUNC_CONTROLLER);
        ya_se_busco = 1;
    }

    /* el control puede estar desenchufado: se pregunta de nuevo si el aparato sigue en el bus */
    if (control != NULL && (control->info.functions & MAPLE_FUNC_CONTROLLER) == 0) {
        control = NULL;
    }

    return control != NULL ? 1 : 0;
}

/* d2c_control_leer: lee el estado AHORA. Escribe los botones (el campo de bits del control) y los cuatro
 * ejes y los dos gatillos donde le digan. Devuelve 0 si leyo, y si no hay control devuelve -1 (sin tocar
 * nada, para que el que llama no lea basura). */
int d2c_control_leer(unsigned int *botones, int *palanca_x, int *palanca_y,
                     int *gatillo_l, int *gatillo_r, int *palanca2_x, int *palanca2_y) {
    cont_state_t *estado;

    if (inyectando) {
        estado = &estado_inyectado;
    } else {
        if (control == NULL) {
            return -1;
        }

        /* maple_dev_status espera a que el estado sea valido: no devuelve una lectura a medio hacer */
        estado = (cont_state_t *)maple_dev_status(control);
        if (estado == NULL) {
            return -2;
        }
    }

    if (botones != NULL) {
        *botones = estado->buttons;
    }
    if (palanca_x != NULL) {
        *palanca_x = estado->joyx;
    }
    if (palanca_y != NULL) {
        *palanca_y = estado->joyy;
    }
    if (gatillo_l != NULL) {
        *gatillo_l = estado->ltrig;
    }
    if (gatillo_r != NULL) {
        *gatillo_r = estado->rtrig;
    }
    if (palanca2_x != NULL) {
        *palanca2_x = estado->joy2x;
    }
    if (palanca2_y != NULL) {
        *palanca2_y = estado->joy2y;
    }

    return 0;
}

/* ============================== DC-002.114: EL CONTROL POR PUERTO ==============================
 *
 * POR QUE. El cooperativo local de Diablo II son hasta CUATRO jugadores y cada uno tiene su control: un jugador no
 * es "el control", es "un control en SU puerto del bus maple". Las tres funciones de arriba miran el bus entero y
 * devuelven el primer control que encuentran (el del puerto 0 en la practica): sirven para un jugador y para nada
 * mas.
 *
 * LA DIFERENCIA CON `maple_enum_type`, Y ES LA QUE IMPORTA: `maple_enum_type(n, func)` devuelve el n-esimo aparato
 * DE ESE TIPO en el orden del bus, NO el que esta en el puerto n. Para "el control del puerto 2" eso no sirve: con
 * un control en A y otro en C, el indice 1 es el de C. El puerto se pide con `maple_enum_dev(puerto, unidad)`
 * (dc/maple.h), que es la direccion fisica del bus. La unidad 0 es el aparato principal de ese puerto (la 1 seria
 * su memoria VMU).
 *
 * LAS FUNCIONES DE ARRIBA NO CAMBIAN DE COMPORTAMIENTO: siguen leyendo el primer control del bus. Las de abajo son
 * nuevas y cada una dice a que puerto mira. Un POC que no las llame no se entera de que existen.
 *
 * LA INYECCION TAMBIEN ES POR PUERTO: la corrida de DC-002.114 tiene UN solo control en el bus (Flycast mapea el
 * teclado al puerto A), asi que el segundo jugador se mueve con el estado que pone el shim EN SU PUERTO. Entra por
 * la misma puerta (botones -> tabla -> teclas) y el POC lo imprime en la evidencia.
 */
#define D2C_PUERTOS_MAPLE 4

static maple_device_t *controles[D2C_PUERTOS_MAPLE];
static int buscados[D2C_PUERTOS_MAPLE];
static int inyectandoPuerto[D2C_PUERTOS_MAPLE];
static cont_state_t inyectadoPuerto[D2C_PUERTOS_MAPLE];

/* d2c_control_buscar_puerto: hay control en ESE puerto. Se busca una vez y, como arriba, cada llamada vuelve a
 * verificar que el aparato siga diciendo que es un control. Devuelve 1 si hay, 0 si no. */
int d2c_control_buscar_puerto(unsigned puerto) {
    if (puerto >= D2C_PUERTOS_MAPLE) {
        return 0;
    }

    if (inyectandoPuerto[puerto]) {
        return 1;
    }

    if (!buscados[puerto]) {
        /* el aparato del puerto, unidad 0 (la 1 es la VMU de ese mismo puerto) */
        controles[puerto] = maple_enum_dev((int)puerto, 0);
        buscados[puerto] = 1;
    }

    if (controles[puerto] != NULL &&
        (controles[puerto]->info.functions & MAPLE_FUNC_CONTROLLER) == 0) {
        controles[puerto] = NULL;
    }

    return controles[puerto] != NULL ? 1 : 0;
}

/* d2c_control_leer_puerto: el estado del control de ESE puerto. Mismos argumentos y mismos codigos de retorno que
 * d2c_control_leer (0 = leyo, -1 = no hay aparato, -2 = el aparato no devolvio estado). */
int d2c_control_leer_puerto(unsigned puerto, unsigned int *botones, int *palanca_x, int *palanca_y,
                            int *gatillo_l, int *gatillo_r, int *palanca2_x, int *palanca2_y) {
    cont_state_t *estado;

    if (puerto >= D2C_PUERTOS_MAPLE) {
        return -1;
    }

    if (inyectandoPuerto[puerto]) {
        estado = &inyectadoPuerto[puerto];
    } else {
        if (controles[puerto] == NULL) {
            return -1;
        }

        estado = (cont_state_t *)maple_dev_status(controles[puerto]);
        if (estado == NULL) {
            return -2;
        }
    }

    if (botones != NULL) {
        *botones = estado->buttons;
    }
    if (palanca_x != NULL) {
        *palanca_x = estado->joyx;
    }
    if (palanca_y != NULL) {
        *palanca_y = estado->joyy;
    }
    if (gatillo_l != NULL) {
        *gatillo_l = estado->ltrig;
    }
    if (gatillo_r != NULL) {
        *gatillo_r = estado->rtrig;
    }
    if (palanca2_x != NULL) {
        *palanca2_x = estado->joy2x;
    }
    if (palanca2_y != NULL) {
        *palanca2_y = estado->joy2y;
    }

    return 0;
}

/* d2c_control_inyectar_puerto: el estado que devuelve `d2c_control_leer_puerto` para ESE puerto, en vez del bus.
 * Devuelve 1 si quedo encendido. */
int d2c_control_inyectar_puerto(unsigned puerto, unsigned int botones, int palanca_x, int palanca_y,
                                int gatillo_l, int gatillo_r, int encendido) {
    if (puerto >= D2C_PUERTOS_MAPLE) {
        return 0;
    }

    inyectadoPuerto[puerto].buttons = botones;
    inyectadoPuerto[puerto].joyx = palanca_x;
    inyectadoPuerto[puerto].joyy = palanca_y;
    inyectadoPuerto[puerto].ltrig = gatillo_l;
    inyectadoPuerto[puerto].rtrig = gatillo_r;
    inyectadoPuerto[puerto].joy2x = 0;
    inyectadoPuerto[puerto].joy2y = 0;

    inyectandoPuerto[puerto] = encendido != 0 ? 1 : 0;

    return inyectandoPuerto[puerto];
}

/* d2c_control_inyectando_puerto: si ese puerto lo esta poniendo el shim (para que la evidencia lo diga). */
int d2c_control_inyectando_puerto(unsigned puerto) {
    if (puerto >= D2C_PUERTOS_MAPLE) {
        return 0;
    }

    return inyectandoPuerto[puerto];
}

/* d2c_control_aparato: que hay en ese puerto del bus, como numero (0 = nada, si no los bits de `functions` del
 * aparato). Es el instrumento de la evidencia: dice si el emulador tiene un control en el puerto 1 o si el estado
 * de ese jugador viene del shim, sin que haya que mirar la pantalla. */
int d2c_control_aparato(unsigned puerto) {
    if (puerto >= D2C_PUERTOS_MAPLE) {
        return 0;
    }

    if (controles[puerto] == NULL) {
        return 0;
    }

    return (int)controles[puerto]->info.functions;
}

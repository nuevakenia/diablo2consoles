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

/* d2c_control_buscar: busca el control en el puerto 0. Devuelve 1 si hay control y 0 si no.
 * Se busca UNA vez (maple_enum_type recorre el bus), no en cada cuadro. */
int d2c_control_buscar(void) {
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

    if (control == NULL) {
        return -1;
    }

    /* maple_dev_status espera a que el estado sea valido: no devuelve una lectura a medio hacer */
    estado = (cont_state_t *)maple_dev_status(control);
    if (estado == NULL) {
        return -2;
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

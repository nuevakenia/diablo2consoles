/* DC-002.10: el asset horneado, entregado a Go para que lo dibuje el MOTOR (no el C).
 *
 * DIFERENCIA CON LOS POCS ANTERIORES: en motor-menu y motor-pvr el C hacia TODO (tomaba los indices, armaba
 * la textura y pintaba el quad). Aca el C solo PRESTA los datos: los indices y la paleta del asset salen a
 * Go, Go arma la imagen y la dibuja con DrawImage en la pantalla del juego, y despues la pantalla se sube a
 * la VRAM. Es el camino que va a usar el juego de verdad, y por eso se prueba asi.
 *
 * El header lo genera scripts/console/bake-asset.sh (contiene los indices ya decodificados y la paleta). */
#include <kos.h>
#include "menu.h"

void d2c_serial_init(void) {
    dbgio_dev_select("scif");
}

int d2c_asset_w(void) { return MENU_W; }
int d2c_asset_h(void) { return MENU_H; }

/* el puntero a los indices (1 byte por pixel) y la paleta como ARGB1555 ya empaquetado (es el formato que
 * usa el PVR y el que el horneador ya dejo asi: aca no se rearma la paleta) */
unsigned int d2c_asset_idx(void) { return (unsigned int)(uintptr_t)menu_idx; }
unsigned int d2c_asset_pal(int i) {
    if (i < 0 || i > 255) return 0;
    return (unsigned int)menu_pal[i];
}

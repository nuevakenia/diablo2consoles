// Consola serie para el POC del reloj (DC-002.158). Cada POC lleva su csrc: build-dc.sh compila los .c
// del POC que se le pide y no comparte objetos entre ellos (mismo criterio que console/dc/poc/vfs/csrc).
#include <kos.h>

// La consola serie del emulador (scif) es la unica salida de texto: es donde run-flycast.sh lee
// `RELOJ158: ...` del log.
void d2c_serial_init(void) {
    dbgio_dev_select("scif");
}

// Consola serie para el POC del gzip (DC-002.20). Es el mismo puente minimo que los otros POC: cada POC
// lleva su csrc, porque build-dc.sh compila los .c del POC que se le pida y no comparte objetos.
#include <kos.h>

// La consola serie del emulador (scif) es la unica salida de texto.
void d2c_serial_init(void) {
    dbgio_dev_select("scif");
}

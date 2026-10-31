package main

// DC-002.98 -- LA PALANCA (y su defecto): si el decodificador de .dcc pide la memoria al monton de C o al
// monton de Go.
//
//	sinArena APAGADA (el camino de la tarjeta): `d2dcc.PedirDeArena` apunta a la arena del POC, asi que los
//	         pixeles de los cuadros, las celdas y la tabla de celdas salen de tramos de mas de 64 KB pedidos
//	         al monton de C, y el dueno los devuelve con `d2c_libre` cuando termino.
//
//	         PRENDIDA es EL DEFECTO que la tarjeta viene a medir: el decodificador pide con `make`, o sea al
//	         monton de Go, y ahi no entra (1,4 MB de celdas + 737 KB de pixeles en un semiespacio de 2 MB).
//	         Con la palanca prendida la corrida tiene que morir en el bloque A; si no muriera, el instrumento
//	         no estaria midiendo lo que dice medir.
//
// La genera el script de build (scripts/console/build-dcc-298.sh) en el directorio de compilacion; este es
// el valor por defecto, que es el camino de la tarjeta.
const sinArena = false

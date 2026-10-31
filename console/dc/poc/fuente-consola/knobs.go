package main

// DC-002.96 -- LA PALANCA (y su mutante): si el dueno devuelve el tramo o no.
//
//	noLibera   APAGADA (el camino de la tarjeta): cada tramo que se pide se devuelve al monton con el `free`
//	           del shim (`d2c_libre`, el mismo `free` que uso `gc_external_alloc`). Es el unico camino que
//	           puede hacer que el monton vuelva al nivel de partida despues de un archivo de 1 MB.
//
//	           PRENDIDA es el MUTANTE: la fuente NO tiene liberador, asi que `Tramo.Liberar` suelta la
//	           referencia y el bloque se queda tomado. Con el mutante los bloques A (la zona), B (los seis) y
//	           C (el 1 MB) tienen que dar el MISMO resultado que el bloque D (el camino de hoy, sin dueno); si
//	           dieran igual con la palanca apagada, el instrumento no estaria midiendo el dueno.
//
// La genera el script de build (scripts/console/build-fuente-296.sh) en el directorio de compilacion; este es
// el valor por defecto, que es el camino de la tarjeta.
const noLibera = false

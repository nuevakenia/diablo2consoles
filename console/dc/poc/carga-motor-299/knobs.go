package main

// DC-002.99 -- LA PALANCA (y su mutante): si el dueno devuelve el tramo o no.
//
//	noLibera   APAGADA (el camino de la tarjeta): el tramo que pide `d2asset.LoadFile` se devuelve al monton
//	           con el `free` del shim (`d2c_libre`, el mismo `free` que uso `gc_external_alloc`). Es lo que
//	           hace que el monton de C vuelva al nivel de partida cuando se suelta el reservorio.
//
//	           PRENDIDA es el MUTANTE: la fuente NO tiene liberador, asi que `Liberar` suelta la referencia y
//	           el bloque se queda tomado. Con el mutante el bloque B (el tramo) tiene que decir lo mismo que el
//	           bloque A (el camino de ayer, sin dueno); si dijera igual con la palanca apagada, el instrumento
//	           no estaria midiendo el dueno.
//
// La genera el script de build (scripts/console/build-carga-299.sh) en el directorio de compilacion; este es
// el valor por defecto, que es el camino de la tarjeta.
const noLibera = false

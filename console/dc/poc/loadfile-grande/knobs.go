package main

// DC-002.79 -- LAS TRES PALANCAS DE LA MEDICION, y por que cada una hace falta.
//
//	lecturaDeConsola   PRENDE el camino de esta tarjeta (`d2asset.ActivarLecturaDeConsola`): el archivo se
//	                   lee con el tamano reservado de antemano y el prefetch del MPQ apagado. APAGADA es el
//	                   camino de siempre (`io.ReadAll`), que es la prueba del ANTES: se espera la excepcion
//	                   en el primer .dt1 grande (medido en DC-002.76).
//	modoBuffer         los .dt1 se leen con `am.LoadFileEn` en UN buffer que se reusa, en vez de `am.LoadFile`
//	                   (un buffer nuevo por archivo). Con esto el monton NO sube por archivo: es la prueba
//	                   de que la lectura dejo de pedir memoria por archivo.
//	liberaCrudo        libera con `free()` el buffer de cada .dt1 despues de calcular su crc32. Es el control
//	                   de DC-002.78 (`SUELTA=1`): sirve para ver si el monton externo aguanta los ocho
//	                   archivos sin crecer, aunque el motor no lo haga.
//
// Las tres las genera el script de build (scripts/console/build-loadfile-279.sh) en el directorio de
// compilacion; estos son los valores por defecto, que son "el camino de hoy" (sin nada nuevo prendido).
const (
	lecturaDeConsola = false
	modoBuffer       = false
	liberaCrudo      = false
)

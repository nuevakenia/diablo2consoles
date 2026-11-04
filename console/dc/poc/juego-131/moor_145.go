//go:build dreamcast

package main

// DC-002.145 -- EL BLOOD MOOR DE VERDAD EN LA CONSOLA, COMPUESTO POR REGIONES CON EL DRLG (la cabecera historica).
//
// QUE DEJO ESTA TARJETA. El afuera de la consola era UNA PIEZA de `.ds1` (`Act1/Outdoors/Wild1.ds1`, 17 x 17 = 289
// casillas, el preset 32). El Blood Moor de verdad es el nivel 2 de `Levels.txt`: 56 x 96 = 5.376 casillas, y NO
// TIENE FILA DE PRESET -- en el juego lo arma el DRLG por regiones, no un `.ds1` entero. DC-002.145 metio en la
// consola una REGION de 32 x 32 DE ESE NIVEL (desde 24,32) compuesta con los registros que el DRLG genera, por el
// mismo camino que el motor de la PC (`SetExactTiles`). El nivel entero NO entra: sus dos pedidos grandes son
//
//	arena de bloques  3.047.626 B   (los bloques RLE de los tiles que el nivel referencia, filtrados por clave)
//	buffer de lectura 1.141.544 B   (`act1/outdoors/cottages.dt1`, el `.dt1` mas grande del nivel)
//
// contra 2.994.912 B libres en el cruce: faltan ~1,2 MB, y por eso la region.
//
// DONDE VIVE HOY. El reparto del trabajo no cambio: el DRLG corre en el HOST (`console/dc/medidas/moor-drlg`) y el
// plan viaja en el binario como DATO. Lo que cambio es DC-002.147: el dato son las SEIS regiones que cubren el
// nivel (en forma plana, `moor_regiones_147.go`) y la composicion es una sola funcion que sirve para la primera
// region y para las que entran caminando (`moor_147.go`: `demoComponeRegion`). Esta cabecera queda por el porque
// de la region y por los numeros del host, que son los que decidieron el tamano.
//
// LOS TRES NUMEROS DE ESTA TARJETA (Flycast, build `demo145`):
//
//	DEMO140: REGION COMPUESTA casillas= 1024 conPiso= 1024 sinNada= 0 fallas= 0 tiles= 1024 ms= 24181
//	DEMO140: CRUZA OK nivel= BloodMoor:region tiles= 1024 jugador= 16.5 16.5 ms= 26046 libre= 1571272
//	DEMO140: FPS cuadros= 720 msPorCuadro= 53-56 fps= 10
//
// El arena de bloques que copio la consola (1.534.112 B) es EL MISMO que midio el host para esa region: la cuenta
// de bloques de los dos lados coincide al byte. La evidencia completa:
// `console/dc/evidencia/dc-002.145-el-moor-por-regiones-en-la-consola.txt`.

package main

// DC-002.90 -- LAS PALANCAS DE LA CORRIDA, Y LA UNICA FUENTE DE VERDAD.
//
// Este archivo ES la fuente: `scripts/console/build-tablas-282.sh` LEE de aca los ocho valores y escribe
// el `knobs.go` del directorio de build; el entorno solo puede PISARLOS, y cuando los pisa lo avisa en
// grande diciendo cual y cuanto mide el binario de la referencia. Antes (hasta DC-002.89) el script traia
// sus propios valores escritos a mano, distintos de estos, y por eso el mismo codigo daba dos binarios:
// el de las palancas del entorno (6.859.016 B, entra) y el del default del script (6.863.464 B, NO entra:
// `fatal error: out of memory` en la cuarta tabla, arena 5.034.368 B con 576 B libres).
//
// LA CORRIDA DE REFERENCIA (DC-002.88, commit d2b59155, y DC-002.89, commit 258412ca) es EXACTAMENTE esta:
//   modoTablas=3  hacerMapa=true  conEntidades=false  lienzoReferencia=false
//   gcEntreTablas=true  gcInhibidoTablas=true  pedirArenaPrimero=false  tilesDelStamp=true
// Con esta combinacion el 1ST_READ.BIN mide 6.859.016 B, las 3 fases del pueblo salen con crc
// -1596781041 / -205471136 (pico 9.525.408 B de 9.528.032 B) y la corrida se reproduce byte por byte.
// `scripts/console/checar-palancas-290.sh` corre el build sin pasar una sola variable y verifica ese
// numero: si el numero cambia, el script del build lo avisa con la diferencia en bytes.
const (
	// modoTablas: QUE tablas de `d2resource.DataDictionaries()` se cargan antes del mapa.
	//   0 = ninguna (control: sin Records, `LoadStamp` no puede ni mirar LvlPrest)
	//   1 = las del compositor (`tablasDelCompositor`: LvlTypes, LvlPrest, LvlWarp, AutoMap, Levels,
	//       LvlMaze, LvlSub, objtype, Objects, monpreset, monstats, monstats2) -- 12 tablas
	//   2 = TODAS (el arranque entero del motor: `d2app.initDataDictionaries`)
	//   3 = SOLO LvlTypes + LvlPrest (las dos que el compositor de casillas no puede dejar de tener)
	//
	// La corrida de referencia usa 3, no 1: con las 12 del compositor (1) el binario no entraba en la
	// consola por RAM (DC-002.82/.86) y el pueblo se compone igual con las dos tablas que el compositor
	// de casillas necesita.
	modoTablas = 3

	// gcEntreTablas: pide el recolector antes de cada tabla (y lo que devuelve, que se informa).
	gcEntreTablas = true

	// gcInhibidoTablas: FRENA el recolector durante la carga de cada tabla (el mismo freno que el runtime
	// de libgodc se pone en sus secciones criticas). Medido en DC-002.78 para los .dt1: el recolector de
	// libgodc es de copia y salta por umbral en medio del parseo.
	gcInhibidoTablas = true

	// pedirArenaPrimero: pedir el arena de tiles con un .dt1 suelto ANTES de armar el MapEngine, para que
	// la corrida diga si la pared es el arena (4.849.148 B de una) y no el compositor.
	//
	// EN LA CORRIDA DE REFERENCIA ESTA APAGADO (ARENA=0, DC-002.88/.89). La pregunta que responde ya se
	// contesto en DC-002.82 (el arena de 4.849.148 B es la pared del compositor) y el binario de la
	// referencia no lleva el .dt1 suelto de prueba ni su codigo. Prendido el binario cambia: no es el de
	// 6.859.016 B ni el de los crc -1596781041 / -205471136.
	pedirArenaPrimero = false

	// hacerMapa: componer el pueblo con `d2mapengine` despues de las tablas.
	hacerMapa = true

	// conEntidades: `PlaceStamp` (con monstruos y objetos del DS1) en vez de `PlaceStampClippedWhere`
	// sin entidades. Las entidades necesitan la tabla de lookups de objetos en disco y la animacion
	// (d2animdata), que este binario NO carga: prendida, lo que se mide es el costo de PEDIRLAS.
	conEntidades = false

	// tilesDelStamp: con `true`, `ResetMapParaStamp` (los `.dt1` que el `.ds1` del stamp lista) en vez de
	// `ResetMap` (los 32 campos del level type). Es la palanca de DC-002.88: el level type del pueblo lista 9
	// archivos y el `.ds1` usa 8, y son listas distintas (sobran trees/bridge, falta warp.dt1).
	tilesDelStamp = true

	// lienzoReferencia: componer al final el lienzo del POC (`nivel.go`) sobre el mismo .ds1, para dejar
	// en la misma corrida el crc32 3507647456 de las tarjetas anteriores. Va ULTIMO porque el lienzo
	// (262.144 B) y el scratch (81.920 B) pasan de 64 KB y en esta consola no se devuelven.
	//
	// EN LA CORRIDA DE REFERENCIA ESTA APAGADO (LIENZO=0, DC-002.88/.89): esos 344.064 B quedan tomados
	// hasta el final, justo el aire que la corrida de 3 fases necesita, y la equivalencia del pueblo ya la
	// da el crc de la grilla del motor (-1596781041 en la entrada y en la vuelta). Prendido, el binario no
	// es el de la referencia.
	lienzoReferencia = false
)

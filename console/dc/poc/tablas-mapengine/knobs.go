package main

// DC-002.82 -- las palancas de la corrida. El build (scripts/console/build-tablas-282.sh) REESCRIBE este
// archivo en el directorio de build; los valores de aca son los de la corrida por defecto, y cada uno
// tiene su por que.
const (
	// modoTablas: QUE tablas de `d2resource.DataDictionaries()` se cargan antes del mapa.
	//   0 = ninguna (control: sin Records, `LoadStamp` no puede ni mirar LvlPrest)
	//   1 = las del compositor (`tablasDelCompositor`: LvlTypes, LvlPrest, LvlWarp, AutoMap, Levels,
	//       LvlMaze, LvlSub, objtype, Objects, monpreset, monstats, monstats2) -- 12 tablas
	//   2 = TODAS (el arranque entero del motor: `d2app.initDataDictionaries`)
	//   3 = SOLO LvlTypes + LvlPrest (las dos que el compositor de casillas no puede dejar de tener)
	modoTablas = 3

	// gcEntreTablas: pide el recolector antes de cada tabla (y lo que devuelve, que se informa).
	gcEntreTablas = true

	// gcInhibidoTablas: FRENA el recolector durante la carga de cada tabla (el mismo freno que el runtime
	// de libgodc se pone en sus secciones criticas). Medido en DC-002.78 para los .dt1: el recolector de
	// libgodc es de copia y salta por umbral en medio del parseo.
	gcInhibidoTablas = true

	// pedirArenaPrimero: pedir el arena de tiles con un .dt1 suelto ANTES de armar el MapEngine, para que
	// la corrida diga si la pared es el arena (4.849.148 B de una) y no el compositor.
	pedirArenaPrimero = true

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
	lienzoReferencia = true
)

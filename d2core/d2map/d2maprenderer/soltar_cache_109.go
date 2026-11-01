package d2maprenderer

// DC-002.109 -- SOLTAR LA CACHE DE UN NIVEL (EL TRAMO) ANTES DE ARMAR EL SIGUIENTE.
//
// POR QUE EXISTE. DC-002.106 dejo el pueblo en pantalla con el tramo de la cache pedido EN PEDAZOS y las
// tablas y las fuentes MPQ soltadas despues de componer. Lo que falta es el CAMBIO DE NIVEL: irse a otro
// `.ds1` y VOLVER, que obliga a reabrir las dos fuentes y recargar las tablas -- 570.720 B medidos -- y deja
// el monton de C (descabezado por el tramo de la cache) sin lugar para componer el nivel nuevo.
//
// EL NUMERO. Despues del cuadro 1, soltando el arena de bloques, el monton de C queda con 2.729.336 B libres
// y el tramo de la cache son 2.723.840 B que NO se devuelven (`gc_external_alloc` no lo toca el recolector,
// ADR 0001 §1.4): reabrir las fuentes deja ~2,16 MB y componer pide ~3,63 MB (arena de bloques 2.621.440 B +
// buffer prestado de lectura 1.008.003 B). O sea: el nivel nuevo y la cache vieja no conviven.
//
// QUE HACE. Tira lo que la cache tiene del nivel que se esta dejando: las imagenes cacheadas, la paleta
// expandida y los BLOQUES DEL TRAMO. Los bloques los devuelve el que llama (`soltar`), porque este paquete es
// del MOTOR y se compila tambien para la PC: en la consola el tramo puede vivir en el monton de C (se devuelve
// con el `free` externo del shim) o en la VRAM del PVR (`pvr.VramSoltar`), y en la PC no hay nada que devolver
// y el camino es el de siempre (`soltar` nil: se tiran las referencias y el recolector hace su trabajo).
//
// LO QUE NO HACE. No reserva nada ni decide el orden del cambio de nivel: eso lo mide y lo decide el que
// llama (DC-002.109 compara soltar ANTES de reabrir contra no soltar). Tampoco suelta el arena de bloques ni
// las tablas: son de `d2asset` y tienen su propio contrato (DC-002.101/.105).
//
// ES ADITIVA. Sin llamarla, el camino es el de siempre (el nivel nuevo se cuelga con `SetMapEngine` y el tramo
// se reusa desde el byte 0 con `reiniciarTramo`): la PC no cambia una linea.

// SoltarLaCacheDelNivel tira la cache del nivel que se esta dejando y devuelve los bytes que `soltar` devolvio
// DE VERDAD (0 si no habia tramo, si `soltar` es nil o si el que reserva los bloques se nego a recibirlos).
//
// `soltar` recibe UN bloque del tramo y devuelve cuantos bytes devolvio (0 = no lo devolvio). Un bloque que no
// se devuelve SE QUEDA en el tramo con su uso en cero: se reusa desde el byte 0, que es lo que hace este camino
// cuando el tramo vive en el monton externo de la consola.
func (mr *MapRenderer) SoltarLaCacheDelNivel(soltar func(bloque []byte) int) int {
	if mr == nil {
		return 0
	}

	// las imagenes cacheadas, la paleta expandida y los blancos son del nivel que se va (y el tramo queda con
	// su uso en cero). Es el mismo reinicio que hace `SetMapEngine`, hecho ACA y a proposito: la tarjeta mide
	// si conviene soltar ANTES de reabrir las fuentes.
	mr.resetLevelCaches()

	t := mr.tramo
	if t == nil || soltar == nil {
		return 0
	}

	bytes := 0

	// tramo de UN GOLPE (DC-002.104)
	if t.pedazo == 0 {
		if len(t.bytes) > 0 {
			bytes += soltar(t.bytes)

			if bytes == 0 {
				return 0
			}

			t.bytes = nil
			t.usado = 0
			t.imagenes = 0
		}

		return bytes
	}

	// tramo EN PEDAZOS (DC-002.106/.108): el que no se devuelve se queda (se reusa desde el byte 0)
	quedan := make([][]byte, 0, len(t.pedazos))
	usados := make([]int, 0, len(t.usadoEn))

	for i := range t.pedazos {
		if n := soltar(t.pedazos[i]); n > 0 {
			bytes += n

			continue
		}

		quedan = append(quedan, t.pedazos[i])
		usados = append(usados, 0)
	}

	t.pedazos = quedan
	t.usadoEn = usados

	t.pedido = 0
	for i := range t.pedazos {
		t.pedido += len(t.pedazos[i])
	}

	t.usado = 0
	t.imagenes = 0
	t.derrames = 0
	t.bytesDerramados = 0
	t.pedazoMenor = 0
	t.pedazoMayor = 0

	return bytes
}

// PonerEnCeroLosBloquesDelTramoCon: como se deja EN CERO un bloque del tramo que se va a REUSAR (DC-002.109).
//
// POR QUE. El tramo no se libera al cambiar de nivel: se reusa desde el byte 0 (ADR 0001 §1.4). Pero el
// decodificador del .dt1 escribe SOLO donde algun bloque del tile pinta, asi que un bloque reusado arrastra los
// pixeles del nivel anterior y el cuadro del nivel nuevo sale con la huella del viejo -- medido en el host con
// el tramo puesto: VOLVER daba crc32 1533561935 en vez de 1437022778, los mismos 307.200 pixeles.
//
// `cero` recibe UN bloque y devuelve lo que limpio (0 = no limpio nada; entonces el tramo lo limpia con un
// bucle de Go, que es lo que corresponde a la PC). En la consola y con el tramo en la VRAM del PVR el que
// limpia es `pvr.VramPonerEnCero` (el `memset` de C: escribe de a 4 bytes y la VRAM se toca sin cache).
//
// ES ADITIVA: sin llamarla, el tramo se limpia con el bucle de Go, que es lo mismo que hacia la PC.
func (mr *MapRenderer) PonerEnCeroLosBloquesDelTramoCon(cero func(bloque []byte) int) {
	if mr == nil || mr.tramo == nil || cero == nil {
		return
	}

	mr.tramo.cero = func(bloque []byte) { cero(bloque) }
}

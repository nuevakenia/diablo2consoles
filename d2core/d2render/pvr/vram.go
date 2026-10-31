//go:build dreamcast

package pvr

import "unsafe"

// DC-002.108 -- LA VRAM DEL PVR COMO LUGAR DE LOS BUFFERS GRANDES (la cara de Go).
//
// QUE ES. El pool de texturas del PVR (8 MB, repartidos por `pvr_mem_malloc`) puesto a disposicion de los que
// piden buffers de mas de un par de MB. La cache de tiles en indices del pueblo son 2.723.840 B y el monton de
// C no los aguanta como bloque unico (DC-002.104/.105: dlmalloc pide el tramo entero por `sbrk` y no hay 2,7 MB
// contiguos); en pedazos entra (DC-002.106), pero deja el monton en 12.288 B de break room.
//
// La VRAM es la memoria donde esa cache vive en una Dreamcast de verdad, y el motor solo tiene ~2,3 MB tomados
// de ella (la textura del cuadro 1.048.576 B + el lienzo RGBA 1.228.800 B), asi que sobran ~5,7 MB.
//
// LAS DOS FORMAS DE PEDIR, y por que las dos.
//
//	VramReservar  pide el bloque AHORA y devuelve el slice. Si no entra devuelve nil y el que llama decide (el
//	              camino de DC-002.106 derrama al `make` de siempre y lo cuenta). Un pedido que no entra NO
//	              revienta el runtime: es la diferencia con el monton de C, donde `gc_external_alloc` tira OOM.
//	VramServible  la sonda: el mayor bloque de hasta `tope` B que el pool sirve AHORA, sin quedarse con el.
//	              Sirve para lo mismo que `d2c_servible` en DC-002.106: cuando quedan migas el bloque nuevo sale
//	              mas chico en vez de morir.
//
// LO QUE CUESTA, DICHO DE FRENTE. La VRAM se toca sin cache (area P2, cache-through): leer o escribir un byte
// ahi es mas lento que en RAM. El lienzo del cuadro ya vive ahi desde DC-002.22; esta tarjeta pone ademas los
// pixeles de la cache y lo mide con los ms por cuadro del log.
//
// LO QUE NO HACE. No sube la cache como textura del PVR (no hay `pvr_txr_load_ex` aca): la cache se guarda en
// el pool de texturas pero el dibujado la sigue leyendo la CPU, que es lo que mantiene el cuadro identico al
// byte con el del host (crc32 1437022778). Dibujar los tiles con quads por hardware es otra tarjeta: cambia el
// mezclado y el orden, y con eso el cuadro.

//extern d2c_pvr_vram_disponible
func d2cPvrVramDisponible() int

//extern d2c_pvr_vram_total
func d2cPvrVramTotal() int

//extern d2c_pvr_vram_pedir
func d2cPvrVramPedir(bytes uint) uint

//extern d2c_pvr_vram_pedir_cero
func d2cPvrVramPedirCero(bytes uint) uint

//extern d2c_pvr_vram_soltar
func d2cPvrVramSoltar(bloque uint)

//extern d2c_pvr_vram_servible
func d2cPvrVramServible(tope uint) uint

// VramDisponible: los bytes que el pool de texturas del PVR puede repartir en este momento (0 si el PVR
// todavia no arranco, que es la verdad antes de `pvr_init_defaults`).
func VramDisponible() int { return d2cPvrVramDisponible() }

// VramTotal: el tamano del pool de texturas del PVR (8 MB en una Dreamcast). Es lo de CUANTO hay, no lo libre.
func VramTotal() int { return d2cPvrVramTotal() }

// VramServible: el mayor bloque de hasta `tope` B que la VRAM puede servir AHORA (0 = ni el mas chico). Sondea
// y devuelve la sonda, asi que no cuesta VRAM.
func VramServible(tope int) int {
	if tope <= 0 {
		return 0
	}

	return int(d2cPvrVramServible(uint(tope)))
}

// VramReservar: pide `bytes` B del pool de texturas del PVR y devuelve el slice que apunta ahi (len = cap =
// bytes, nadie puede crecerlo), o nil si no entro.
//
// EL BLOQUE VIENE EN CERO, como lo daria un `make([]byte, n)` de Go. Eso NO es cosmetico: los pixeles de la
// cache los escribe el decodificador del .dt1 SOLO donde algun bloque del tile pinta, y el resto del buffer
// queda como estaba. Con la VRAM cruda eso es basura de la corrida anterior: medido en esta tarjeta, la cache
// salia con las mismas 86 imagenes y los mismos 2.723.840 B y el cuadro con crc32 883948965 en vez de
// 1437022778. Ver `d2c_pvr_vram_pedir_cero` en console/dc/std/csrc/pvr_vram.c.
//
// EL PUNTERO CRUZA COMO uintptr Y SE RECORTA CON UN ARRAY DE LARGO MAXIMO: el runtime de la consola no tiene
// `unsafe.Slice`, y un `unsafe.Pointer` guardado en un campo pide los descriptores que libgodc no trae. Es el
// mismo patron que `SuperficieDeLaPantalla` (pantalla.go).
func VramReservar(bytes int) []byte {
	if bytes <= 0 {
		return nil
	}

	p := d2cPvrVramPedirCero(uint(bytes))
	if p == 0 {
		return nil
	}

	b := (*[1 << 30]byte)(unsafe.Pointer(uintptr(p)))[:bytes:bytes]

	vramPedida += bytes
	vramPedidosTotal++

	if bytes > vramPedidoMayor {
		vramPedidoMayor = bytes
	}

	return b
}

// VramSoltar: devuelve al pool un bloque pedido con `VramReservar`. DESPUES DE ESTO EL SLICE NO SE PUEDE TOCAR
// (la VRAM puede repartirse a otro).
func VramSoltar(b []byte) int {
	if len(b) == 0 {
		return 0
	}

	n := len(b)

	d2cPvrVramSoltar(uint(uintptr(unsafe.Pointer(&b[0]))))

	vramPedida -= n

	return n
}

// los contadores: lo que esta tarjeta le tomo a la VRAM, para poder decirlo en la evidencia.
var (
	vramPedida       int
	vramPedidoMayor  int
	vramPedidosTotal int
)

// CensoDeLaVram: la foto de lo que este proceso tiene tomado del pool de texturas del PVR.
type CensoDeLaVram struct {
	Total           int // el tamano del pool (8 MB)
	Disponible      int // lo que queda libre AHORA
	Pedida          int // lo que pidieron los buffers vivos de este paquete
	Pedidos         int // cuantos pedidos se hicieron (los que entraron)
	PedidoMayor     int // el bloque mas grande que se pidio
	LienzoDelCuadro int // lo que ocupa el lienzo RGBA del cuadro (0 si esta en el monton)
}

// FotoDeLaVram: la foto de la VRAM, para el log de la consola.
func FotoDeLaVram() CensoDeLaVram {
	return CensoDeLaVram{
		Total:           VramTotal(),
		Disponible:      VramDisponible(),
		Pedida:          vramPedida,
		Pedidos:         vramPedidosTotal,
		PedidoMayor:     vramPedidoMayor,
		LienzoDelCuadro: VramDelLienzo(),
	}
}

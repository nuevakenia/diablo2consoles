// Paquete atomic de la consola (sync/atomic).
//
// El SH-4 no tiene atomicos de hardware: no hay load-linked/store-conditional ni compare-and-swap.
// La unica primitiva es APAGAR LAS INTERRUPCIONES en la seccion critica, y eso es lo que hacen los
// shims de C de este paquete (console/dc/std/csrc/atomic_dc.c), con irq_disable/irq_restore de KOS.
// Es la misma estrategia que libgodc ya usa para sus __atomic_*_4 (medido en
// libgodc/runtime/dreamcast_support.c) y la que pide el modelo -matomic-model=soft-imask del runtime.
//
// QUE YA HABIA Y QUE FALTABA: libgodc trae atomicos de 8, 16 y 32 bits y NINGUNO de 64. El SH-4 es de
// 32 bits, asi que un atomico de 64 son dos accesos que hay que proteger juntos: en esta consola
// alcanza con la seccion critica, porque hay UN procesador (SH-4 de un solo nucleo).
//
// LIMITE DECLARADO: cada operacion abre y cierra la seccion critica en pocas instrucciones. Nada de
// acá sostiene las interrupciones apagadas mientras espera (eso colgaria la consola): el atomico no
// espera nunca, y por eso la espera de un candado va en sync (ver sync/sync.go).
//
// No se usan punteros unsafe: los shims tienen una funcion por tipo, y asi este paquete no arrastra
// el descriptor de unsafe.Pointer, que el runtime de la consola no tiene (medido).
package atomic

//extern d2c_at_add32
func d2c_at_add32(p *int32, d int32) int32

//extern d2c_at_load32
func d2c_at_load32(p *int32) int32

//extern d2c_at_store32
func d2c_at_store32(p *int32, v int32)

//extern d2c_at_swap32
func d2c_at_swap32(p *int32, nuevo int32) int32

//extern d2c_at_cas32
func d2c_at_cas32(p *int32, esperado *int32, nuevo int32) int32

//extern d2c_at_add64
func d2c_at_add64(p *int64, d int64) int64

//extern d2c_at_load64
func d2c_at_load64(p *int64) int64

//extern d2c_at_store64
func d2c_at_store64(p *int64, v int64)

//extern d2c_at_swap64
func d2c_at_swap64(p *int64, nuevo int64) int64

//extern d2c_at_cas64
func d2c_at_cas64(p *int64, esperado *int64, nuevo int64) int32

//extern d2c_atu_add32
func d2c_atu_add32(p *uint32, d uint32) uint32

//extern d2c_atu_load32
func d2c_atu_load32(p *uint32) uint32

//extern d2c_atu_store32
func d2c_atu_store32(p *uint32, v uint32)

//extern d2c_atu_swap32
func d2c_atu_swap32(p *uint32, nuevo uint32) uint32

//extern d2c_atu_cas32
func d2c_atu_cas32(p *uint32, esperado *uint32, nuevo uint32) int32

//extern d2c_atu_add64
func d2c_atu_add64(p *uint64, d uint64) uint64

//extern d2c_atu_load64
func d2c_atu_load64(p *uint64) uint64

//extern d2c_atu_store64
func d2c_atu_store64(p *uint64, v uint64)

//extern d2c_atu_swap64
func d2c_atu_swap64(p *uint64, nuevo uint64) uint64

//extern d2c_atu_cas64
func d2c_atu_cas64(p *uint64, esperado *uint64, nuevo uint64) int32

// ------------------------------------------------ con signo

func AddInt32(addr *int32, delta int32) int32 { return d2c_at_add32(addr, delta) }
func LoadInt32(addr *int32) int32             { return d2c_at_load32(addr) }
func StoreInt32(addr *int32, val int32)       { d2c_at_store32(addr, val) }
func SwapInt32(addr *int32, new int32) int32  { return d2c_at_swap32(addr, new) }
func CompareAndSwapInt32(addr *int32, old, new int32) bool {
	return d2c_at_cas32(addr, &old, new) != 0
}

// AddInt64 / LoadInt64 / StoreInt64 / SwapInt64 / CompareAndSwapInt64: en el SH-4 son dos accesos de
// 32 bits protegidos juntos (ver el encabezado del paquete).
func AddInt64(addr *int64, delta int64) int64 { return d2c_at_add64(addr, delta) }
func LoadInt64(addr *int64) int64             { return d2c_at_load64(addr) }
func StoreInt64(addr *int64, val int64)       { d2c_at_store64(addr, val) }
func SwapInt64(addr *int64, new int64) int64  { return d2c_at_swap64(addr, new) }
func CompareAndSwapInt64(addr *int64, old, new int64) bool {
	return d2c_at_cas64(addr, &old, new) != 0
}

// ------------------------------------------------ sin signo

func AddUint32(addr *uint32, delta uint32) uint32 { return d2c_atu_add32(addr, delta) }
func LoadUint32(addr *uint32) uint32              { return d2c_atu_load32(addr) }
func StoreUint32(addr *uint32, val uint32)        { d2c_atu_store32(addr, val) }
func SwapUint32(addr *uint32, new uint32) uint32  { return d2c_atu_swap32(addr, new) }
func CompareAndSwapUint32(addr *uint32, old, new uint32) bool {
	return d2c_atu_cas32(addr, &old, new) != 0
}

func AddUint64(addr *uint64, delta uint64) uint64 { return d2c_atu_add64(addr, delta) }
func LoadUint64(addr *uint64) uint64              { return d2c_atu_load64(addr) }
func StoreUint64(addr *uint64, val uint64)        { d2c_atu_store64(addr, val) }
func SwapUint64(addr *uint64, new uint64) uint64  { return d2c_atu_swap64(addr, new) }
func CompareAndSwapUint64(addr *uint64, old, new uint64) bool {
	return d2c_atu_cas64(addr, &old, new) != 0
}

// ------------------------------------------------ los tipos (Go 1.19 y posteriores)

// Int32: un int32 con sus operaciones (no se puede copiar, como en Go).
type Int32 struct{ v int32 }

func (i *Int32) Load() int32                 { return LoadInt32(&i.v) }
func (i *Int32) Store(v int32)               { StoreInt32(&i.v, v) }
func (i *Int32) Add(d int32) int32           { return AddInt32(&i.v, d) }
func (i *Int32) Swap(v int32) int32          { return SwapInt32(&i.v, v) }
func (i *Int32) CompareAndSwap(o, n int32) bool { return CompareAndSwapInt32(&i.v, o, n) }

// Int64.
type Int64 struct{ v int64 }

func (i *Int64) Load() int64                 { return LoadInt64(&i.v) }
func (i *Int64) Store(v int64)               { StoreInt64(&i.v, v) }
func (i *Int64) Add(d int64) int64           { return AddInt64(&i.v, d) }
func (i *Int64) Swap(v int64) int64          { return SwapInt64(&i.v, v) }
func (i *Int64) CompareAndSwap(o, n int64) bool { return CompareAndSwapInt64(&i.v, o, n) }

// Uint32.
type Uint32 struct{ v uint32 }

func (i *Uint32) Load() uint32               { return LoadUint32(&i.v) }
func (i *Uint32) Store(v uint32)             { StoreUint32(&i.v, v) }
func (i *Uint32) Add(d uint32) uint32        { return AddUint32(&i.v, d) }
func (i *Uint32) Swap(v uint32) uint32       { return SwapUint32(&i.v, v) }
func (i *Uint32) CompareAndSwap(o, n uint32) bool { return CompareAndSwapUint32(&i.v, o, n) }

// Uint64.
type Uint64 struct{ v uint64 }

func (i *Uint64) Load() uint64               { return LoadUint64(&i.v) }
func (i *Uint64) Store(v uint64)             { StoreUint64(&i.v, v) }
func (i *Uint64) Add(d uint64) uint64        { return AddUint64(&i.v, d) }
func (i *Uint64) Swap(v uint64) uint64       { return SwapUint64(&i.v, v) }
func (i *Uint64) CompareAndSwap(o, n uint64) bool { return CompareAndSwapUint64(&i.v, o, n) }

// Bool: un booleano con sus operaciones, sobre un uint32 (0 o 1), como en Go.
type Bool struct{ v uint32 }

func (b *Bool) Load() bool { return LoadUint32(&b.v) != 0 }
func (b *Bool) Store(v bool) {
	if v {
		StoreUint32(&b.v, 1)
	} else {
		StoreUint32(&b.v, 0)
	}
}
func (b *Bool) Swap(v bool) bool {
	var nuevo uint32
	if v {
		nuevo = 1
	}
	return SwapUint32(&b.v, nuevo) != 0
}
func (b *Bool) CompareAndSwap(o, n bool) bool {
	var viejo, nuevo uint32
	if o {
		viejo = 1
	}
	if n {
		nuevo = 1
	}
	return CompareAndSwapUint32(&b.v, viejo, nuevo)
}

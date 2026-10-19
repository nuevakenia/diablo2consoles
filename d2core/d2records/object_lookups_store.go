package d2records

import (
	"fmt"
	"os"
	"sync"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2enum"
)

// DC-002.42: DE DONDE SALEN LOS REGISTROS DE objectLookups CUANDO NO ESTAN EN EL BINARIO.
//
// MEDIDO (DC-002.32, seccion 2 de su evidencia): el volcado `var objectLookups = []ObjectLookupRecord{...}` de
// object_lookup_record_data.go es UN SOLO simbolo de `.data` (`_go..C3111`, 1.672.892 bytes = 7.891 x 212) y
// sus cadenas otros 116.048 de `.rodata`: **1.788.960 bytes de IMAGEN** (13,0% de los 16.777.216 de la
// consola) y el mismo peso en RAM al arrancar, porque el indice de `Object.Lookup` apunta DENTRO del arreglo.
//
// Este archivo es el camino que lo saca del binario SIN dejar al motor sin los registros: la tabla vive en el
// DISCO (`/JUEGO/OBJECTLOOKUPS.PACK`, que arma `console/dc/gen-lookups-pack` desde la MISMA tabla) y se lee
// **por registro, cuando se usa** (al generar un nivel), no en el arranque.
//
// EL CONTRATO DEL FORMATO (lo escribe el generador; lo lee `fuenteDeArchivo`):
//
//	cabecera 32 B:  "D2OL" | version u16 | flags u16 | n u32 | idxOff u32 | recOff u32 |
//	                nGrupos u16 | _ u16 | crc u32 | maxLen u16 | _ u16
//	indice   n x 4: offset de cada registro, relativo a recOff (mismo orden que la tabla)
//	grupos   g x 8: act u8 | type u8 | _ u16 | base u16 | n u16   (los ids de un grupo son 0..n-1, en orden)
//	registros:      act u8 | type u8 | id u16 | 23 cadenas (len u8 + bytes) | 4 x i16
//	                (ObjectsTxtId, MonstatsTxtId, Direction, Index)
//
// El `crc` es FNV-1a 32 de TODO el bloque de registros: el generador lo escribe y el que lee lo verifica (es
// la barrera de que los bytes del disco son los mismos que salieron de la tabla).
//
// CUANTO PESA CUANDO SE USA (medido en la consola, evidencia de la tarjeta): el indice son 4 B x 7.891 =
// 31.564 B en RAM, mas una ventana de lectura de 4 KB y **solo los registros que el motor pide**. Contra los
// 1.672.892 B que ocupaba el arreglo entero en `.data` (sin contar sus cadenas).
//
// QUIEN ES LA FUENTE, en cada lado:
//   - HOST: la tabla compilada (object_lookup_record_data.go la registra en su `init`) -- el host no cambia de
//     comportamiento y sus pruebas siguen leyendo la tabla de siempre.
//   - CONSOLA: ese archivo NO entra al binario (el build de la consola lo excluye por nombre, ver
//     scripts/console/build-motor-all.sh) y la fuente es el archivo del disco, que se abre la PRIMERA vez que
//     el motor pide un registro: en el arranque no se lee ni un byte.

// fuenteLookups es de donde salen los registros. Todas las implementaciones ESCRIBEN UNA COPIA en dst: ni la
// del archivo ni la compilada dejan punteros a buffers propios (la del archivo reusa su ventana de lectura).
type fuenteLookups interface {
	// Largo es cuantos registros tiene la tabla.
	Largo() int
	// Grupo dice donde empiezan y cuantos son los registros de (act, typ) en el orden del archivo: los ids
	// de un grupo son 0..n-1 (es la propiedad que fija TestObjectLookupIdsAreContiguous).
	Grupo(act, typ int) (base, n int, ok bool)
	// Registro copia el i-esimo registro (en orden de archivo) en dst.
	Registro(i int, dst *ObjectLookupRecord) error
	// ObjetosTxtId devuelve SOLO ese campo del i-esimo registro, sin armar el registro ni sus 23 cadenas:
	// es el camino de la tabla del DS1 (5 x 150 numeros de los 7.891 registros).
	ObjetosTxtId(i int) (int, error)
}

// ---------------------------------------------------------------------------------------------
// La fuente COMPILADA (el host): la tabla de object_lookup_record_data.go, sin copiarla.
// ---------------------------------------------------------------------------------------------

type fuenteEnMemoria struct {
	registros []ObjectLookupRecord

	gruposUna sync.Once
	grupos    []grupoLookups // (act, typ, base, n) en orden de primera aparicion
}

type grupoLookups struct {
	act, typ, base, n int
}

func (f *fuenteEnMemoria) Largo() int { return len(f.registros) }

func (f *fuenteEnMemoria) calculaGrupos() {
	for i := range f.registros {
		r := &f.registros[i]
		if n := len(f.grupos); n > 0 {
			ultimo := &f.grupos[n-1]
			if ultimo.act == r.Act && ultimo.typ == int(r.Type) {
				ultimo.n++

				continue
			}
		}

		f.grupos = append(f.grupos, grupoLookups{act: r.Act, typ: int(r.Type), base: i, n: 1})
	}
}

func (f *fuenteEnMemoria) Grupo(act, typ int) (int, int, bool) {
	f.gruposUna.Do(f.calculaGrupos)

	for i := range f.grupos {
		if f.grupos[i].act == act && f.grupos[i].typ == typ {
			return f.grupos[i].base, f.grupos[i].n, true
		}
	}

	return 0, 0, false
}

func (f *fuenteEnMemoria) Registro(i int, dst *ObjectLookupRecord) error {
	if i < 0 || i >= len(f.registros) {
		return fmt.Errorf("objectLookups: registro %d fuera de la tabla (%d)", i, len(f.registros))
	}

	*dst = f.registros[i]

	return nil
}

func (f *fuenteEnMemoria) ObjetosTxtId(i int) (int, error) {
	if i < 0 || i >= len(f.registros) {
		return 0, fmt.Errorf("objectLookups: registro %d fuera de la tabla (%d)", i, len(f.registros))
	}

	return f.registros[i].ObjectsTxtId, nil
}

// ---------------------------------------------------------------------------------------------
// La fuente VACIA: la tabla no esta (no hay archivo y nada la registro). Es el instrumento de peso de
// DC-002.32, no un motor jugable: se declara para que los llamadores tengan un objeto sano con el que
// contestar "no hay" en vez de un nil.
// ---------------------------------------------------------------------------------------------

type fuenteVacia struct{}

func (fuenteVacia) Largo() int                      { return 0 }
func (fuenteVacia) Grupo(int, int) (int, int, bool) { return 0, 0, false }
func (fuenteVacia) Registro(i int, _ *ObjectLookupRecord) error {
	return fmt.Errorf("objectLookups: no hay tabla (registro %d)", i)
}
func (fuenteVacia) ObjetosTxtId(int) (int, error) {
	return 0, fmt.Errorf("objectLookups: no hay tabla")
}

// ---------------------------------------------------------------------------------------------
// La fuente del DISCO (la consola): lee el archivo de la tabla por registro.
// ---------------------------------------------------------------------------------------------

const (
	tamCabeceraLookups = 32
	ventanaLookups     = 4096 // bytes de la ventana de lectura (varios registros por lectura)
	// camposDeTextoDelLookup es cuantas cadenas tiene el registro. El ORDEN de esas cadenas (y el de los
	// campos numericos) es el del struct ObjectLookupRecord y lo fija, del lado que escribe, el generador
	// (object_lookup_record_data.go, host): el viaje de ida y vuelta lo verifica TestPackObjectLookups.
	camposDeTextoDelLookup = 23
)

var (
	// fuenteRegistrada la registra la tabla compilada (object_lookup_record_data.go, host).
	fuenteRegistrada fuenteLookups

	// fuenteDelDisco es la fuente del archivo, una sola por corrida.
	fuenteDelDisco *fuenteDeArchivo

	// RutaDeLosLookups es donde la CONSOLA busca la tabla. En el disco la carpeta del juego viaja como
	// /JUEGO (mayusculas: lo exige el ISO9660) y el iso9660 de KOS compara sin distinguir mayusculas.
	RutaDeLosLookups = "/cd/juego/objectlookups.pack"

	// ErrorDeLosLookups guarda el ultimo error al abrir/leer la tabla (lo imprime la sonda de la consola).
	ErrorDeLosLookups error

	unaVezDisco sync.Once
)

// fuenteDeObjectLookups contesta la fuente activa: la compilada si el host la registro, y si no el archivo del
// disco, que se abre PEREZOSAMENTE (la primera vez que el motor pide un registro: en el arranque no se toca).
func fuenteDeObjectLookups() fuenteLookups {
	if fuenteRegistrada != nil {
		return fuenteRegistrada
	}

	unaVezDisco.Do(func() {
		if fuenteDelDisco == nil {
			if _, err := AbrirLookupsDelArchivo(RutaDeLosLookups); err != nil {
				ErrorDeLosLookups = err
			}
		}
	})

	if fuenteDelDisco == nil {
		return fuenteVacia{}
	}

	return fuenteDelDisco
}

// InformeLookups son los numeros de la tabla leida del disco: lo que la sonda de la consola imprime y con lo
// que la tarjeta prueba que los registros estan.
type InformeLookups struct {
	Ruta          string
	Bytes         int64
	Registros     int
	Grupos        int
	MaxRegistro   int
	CrcArchivo    uint32
	CrcLeida      uint32
	BytesDeIndice int
	BytesLeidos   int64
}

// AbrirLookupsDelArchivo abre el archivo de la tabla, LEE Y VERIFICA su contenido (recorre los registros una
// vez, sin armar ninguno) y lo deja como fuente activa de este proceso. Devuelve los numeros.
func AbrirLookupsDelArchivo(ruta string) (InformeLookups, error) {
	if fuenteDelDisco != nil && fuenteDelDisco.ruta == ruta {
		informe := fuenteDelDisco.Informe()
		informe.CrcLeida = fuenteDelDisco.crcVerificada

		return informe, nil
	}

	f, err := abrirFuenteDeArchivo(ruta)
	if err != nil {
		return InformeLookups{Ruta: ruta}, err
	}

	if err := f.verifica(); err != nil {
		return InformeLookups{Ruta: ruta}, err
	}

	fuenteDelDisco = f

	informe := f.Informe()
	informe.CrcLeida = f.crcVerificada

	return informe, nil
}

type fuenteDeArchivo struct {
	ruta    string
	archivo *os.File
	tam     int64

	n             int
	crc           uint32
	crcVerificada uint32
	maxLen        int
	recOff        int64
	idx           []uint32
	grupos        []grupoLookups
	bytesIdx      int
	leidos        int64

	ventana    []byte
	ventanaOff int64
	ventanaLen int
}

func abrirFuenteDeArchivo(ruta string) (*fuenteDeArchivo, error) {
	archivo, err := os.Open(ruta)
	if err != nil {
		return nil, fmt.Errorf("objectLookups: no pude abrir %s: %w", ruta, err)
	}

	f := &fuenteDeArchivo{ruta: ruta, archivo: archivo, ventanaOff: -1}
	if err := f.leeCabecera(); err != nil {
		_ = archivo.Close()

		return nil, err
	}

	return f, nil
}

func (f *fuenteDeArchivo) leeCabecera() error {
	cab := make([]byte, tamCabeceraLookups)
	if _, err := f.leeAt(cab, 0); err != nil {
		return fmt.Errorf("objectLookups: cabecera de %s: %w", f.ruta, err)
	}

	if string(cab[0:4]) != "D2OL" {
		return fmt.Errorf("objectLookups: %s no es un archivo de la tabla (firma %q)", f.ruta, cab[0:4])
	}

	if v := u16(cab[4:]); v != 1 {
		return fmt.Errorf("objectLookups: version %d de %s (espero 1)", v, f.ruta)
	}

	f.n = int(u32(cab[8:]))
	nGrupos := int(u16(cab[20:]))
	f.crc = u32(cab[24:])
	f.maxLen = int(u16(cab[28:]))

	if f.n <= 0 || f.maxLen <= 0 || nGrupos <= 0 {
		return fmt.Errorf("objectLookups: %s declara n=%d grupos=%d maxLen=%d", f.ruta, f.n, nGrupos, f.maxLen)
	}

	idxOff := int64(u32(cab[12:]))
	f.recOff = int64(u32(cab[16:]))

	if st, err := f.archivo.Stat(); err == nil {
		f.tam = st.Size()
	}

	// el indice: 4 B por registro
	crudo := make([]byte, f.n*4)
	if _, err := f.leeAt(crudo, idxOff); err != nil {
		return fmt.Errorf("objectLookups: indice de %s: %w", f.ruta, err)
	}

	f.idx = make([]uint32, f.n)
	for i := 0; i < f.n; i++ {
		f.idx[i] = u32(crudo[i*4:])
	}

	// los grupos: 8 B por grupo
	gcrudo := make([]byte, nGrupos*8)
	if _, err := f.leeAt(gcrudo, idxOff+int64(f.n*4)); err != nil {
		return fmt.Errorf("objectLookups: grupos de %s: %w", f.ruta, err)
	}

	f.grupos = make([]grupoLookups, 0, nGrupos)
	for i := 0; i < nGrupos; i++ {
		g := gcrudo[i*8:]
		f.grupos = append(f.grupos, grupoLookups{
			act:  int(g[0]),
			typ:  int(g[1]),
			base: int(u16(g[4:])),
			n:    int(u16(g[6:])),
		})
	}

	f.bytesIdx = f.n*4 + nGrupos*8

	tam := ventanaLookups
	if f.maxLen > tam {
		tam = f.maxLen
	}

	f.ventana = make([]byte, tam)

	return nil
}

// leeAt lee contando los bytes que salen del disco (es el numero que la sonda reporta).
func (f *fuenteDeArchivo) leeAt(p []byte, off int64) (int, error) {
	n, err := f.archivo.ReadAt(p, off)
	f.leidos += int64(n)

	return n, err
}

// verifica recorre TODO el bloque de registros y compara el FNV-1a con el de la cabecera.
func (f *fuenteDeArchivo) verifica() error {
	const paso = 8192

	buf := make([]byte, paso)
	h := uint32(2166136261)
	off := f.recOff

	for off < f.tam {
		n, err := f.leeAt(buf, off)
		for i := 0; i < n; i++ {
			h ^= uint32(buf[i])
			h *= 16777619
		}

		off += int64(n)

		if err != nil {
			break
		}
	}

	f.crcVerificada = h

	if h != f.crc {
		return fmt.Errorf("objectLookups: el crc de %s no cierra: archivo %d, leido %d", f.ruta, f.crc, h)
	}

	return nil
}

func (f *fuenteDeArchivo) Informe() InformeLookups {
	return InformeLookups{
		Ruta:          f.ruta,
		Bytes:         f.tam,
		Registros:     f.n,
		Grupos:        len(f.grupos),
		MaxRegistro:   f.maxLen,
		CrcArchivo:    f.crc,
		BytesDeIndice: f.bytesIdx,
		BytesLeidos:   f.leidos,
	}
}

func (f *fuenteDeArchivo) Largo() int { return f.n }

func (f *fuenteDeArchivo) Grupo(act, typ int) (int, int, bool) {
	for i := range f.grupos {
		if f.grupos[i].act == act && f.grupos[i].typ == typ {
			return f.grupos[i].base, f.grupos[i].n, true
		}
	}

	return 0, 0, false
}

// asegura deja en la ventana TODO el registro que empieza en off: la ventana cubre maxLen bytes desde el
// principio del registro, asi que cualquier registro (el mas largo mide maxLen) entra entero.
func (f *fuenteDeArchivo) asegura(off int64) error {
	if f.ventanaOff >= 0 && off >= f.ventanaOff && off+int64(f.maxLen) <= f.ventanaOff+int64(f.ventanaLen) {
		return nil
	}

	cuanto := len(f.ventana)
	if resto := int(f.tam - off); resto < cuanto {
		cuanto = resto
	}

	if cuanto <= 0 {
		return fmt.Errorf("objectLookups: %s se acabo en %d", f.ruta, off)
	}

	n, err := f.leeAt(f.ventana[:cuanto], off)
	if err != nil && n < cuanto {
		if n <= 0 {
			return fmt.Errorf("objectLookups: leo %d bytes en %d de %s: %w", n, off, f.ruta, err)
		}

		cuanto = n
	}

	f.ventanaOff = off
	f.ventanaLen = cuanto

	return nil
}

func (f *fuenteDeArchivo) Registro(i int, dst *ObjectLookupRecord) error {
	if i < 0 || i >= f.n {
		return fmt.Errorf("objectLookups: registro %d fuera de la tabla del disco (%d)", i, f.n)
	}

	off := f.recOff + int64(f.idx[i])
	if err := f.asegura(off); err != nil {
		return err
	}

	_, err := decodificaRegistro(f.ventana[off-f.ventanaOff:f.ventanaLen], dst)

	return err
}

func (f *fuenteDeArchivo) ObjetosTxtId(i int) (int, error) {
	if i < 0 || i >= f.n {
		return 0, fmt.Errorf("objectLookups: registro %d fuera de la tabla del disco (%d)", i, f.n)
	}

	off := f.recOff + int64(f.idx[i])
	if err := f.asegura(off); err != nil {
		return 0, err
	}

	return objetosTxtIdDeRegistro(f.ventana[off-f.ventanaOff : f.ventanaLen])
}

// ---------------------------------------------------------------------------------------------
// Codificacion/decodificacion de un registro (el orden de los campos ES el del struct ObjectLookupRecord)
// ---------------------------------------------------------------------------------------------

// errRegistroCortado: la ventana no alcanzo para el registro entero (defensivo: `asegura` la pone entera).
var errRegistroCortado = fmt.Errorf("objectLookups: el registro no entra en la ventana")

// decodificaRegistro arma dst desde los bytes del registro y devuelve cuantos consumio.
func decodificaRegistro(b []byte, dst *ObjectLookupRecord) (int, error) {
	if len(b) < 4 {
		return 0, errRegistroCortado
	}

	i := 0
	act := int(b[i])
	typ := int(b[i+1])
	id := int(u16(b[i+2:]))
	i += 4

	var textos [camposDeTextoDelLookup]string

	for c := range textos {
		if i >= len(b) {
			return 0, errRegistroCortado
		}

		largo := int(b[i])
		i++

		if i+largo > len(b) {
			return 0, errRegistroCortado
		}

		textos[c] = string(b[i : i+largo])
		i += largo
	}

	if i+8 > len(b) {
		return 0, errRegistroCortado
	}

	objetos := int(int16(u16(b[i:])))
	monstats := int(int16(u16(b[i+2:])))
	direccion := int(int16(u16(b[i+4:])))
	indice := int(int16(u16(b[i+6:])))
	i += 8

	*dst = ObjectLookupRecord{
		Act:           act,
		Type:          d2enum.ObjectType(typ),
		Id:            id,
		Name:          textos[0],
		Description:   textos[1],
		Base:          textos[2],
		Token:         textos[3],
		Mode:          textos[4],
		Class:         textos[5],
		HD:            textos[6],
		TR:            textos[7],
		LG:            textos[8],
		RA:            textos[9],
		LA:            textos[10],
		RH:            textos[11],
		LH:            textos[12],
		SH:            textos[13],
		S1:            textos[14],
		S2:            textos[15],
		S3:            textos[16],
		S4:            textos[17],
		S5:            textos[18],
		S6:            textos[19],
		S7:            textos[20],
		S8:            textos[21],
		ColorMap:      textos[22],
		ObjectsTxtId:  objetos,
		MonstatsTxtId: monstats,
		Direction:     direccion,
		Index:         indice,
	}

	return i, nil
}

// objetosTxtIdDeRegistro lee SOLO ese campo (salta las 23 cadenas sin armar ninguna).
func objetosTxtIdDeRegistro(b []byte) (int, error) {
	if len(b) < 4 {
		return 0, errRegistroCortado
	}

	i := 4
	for c := 0; c < camposDeTextoDelLookup; c++ {
		if i >= len(b) {
			return 0, errRegistroCortado
		}

		largo := int(b[i])
		i += 1 + largo

		if i > len(b) {
			return 0, errRegistroCortado
		}
	}

	if i+2 > len(b) {
		return 0, errRegistroCortado
	}

	return int(int16(u16(b[i:]))), nil
}

func u16(b []byte) int {
	return int(b[0]) | int(b[1])<<8
}

func u32(b []byte) uint32 {
	return uint32(b[0]) | uint32(b[1])<<8 | uint32(b[2])<<16 | uint32(b[3])<<24
}

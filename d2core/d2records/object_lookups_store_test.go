package d2records

import (
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2enum"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2util"
)

// TestPackObjectLookups: EL VIAJE DE IDA Y VUELTA DEL FORMATO DE LA CONSOLA (DC-002.42). Escribe el archivo
// desde la tabla compilada (lo que hace `console/dc/gen-lookups-pack` en el build) y lo vuelve a leer por el
// MISMO camino que usa la consola (abrirFuenteDeArchivo + Registro), comparando los 7.891 registros CAMPO POR
// CAMPO (las 23 cadenas y los 4 numericos, con Act/Type/Id).
//
// Es el control de que el archivo del disco no perdio nada al salir del binario: si alguien cambia el struct
// ObjectLookupRecord, el orden de ordenDeLosCamposDeTexto o el que escribe/lee, esto se cae.
func TestPackObjectLookups(t *testing.T) {
	ruta := filepath.Join(t.TempDir(), "objectlookups.pack")

	archivo, err := os.Create(ruta)
	if err != nil {
		t.Fatal(err)
	}

	informe, err := EscribirPack(archivo, objectLookups)
	if err != nil {
		t.Fatalf("EscribirPack: %v", err)
	}

	if err := archivo.Close(); err != nil {
		t.Fatal(err)
	}

	if info, err := os.Stat(ruta); err != nil {
		t.Fatal(err)
	} else if info.Size() != informe.Bytes {
		t.Fatalf("el archivo mide %d, el informe dice %d", info.Size(), informe.Bytes)
	}

	// la cuenta de la tarjeta: el archivo tiene que ser MAS CHICO que el .data que reemplaza
	const tablaEnData = 1672892
	if informe.Bytes >= tablaEnData {
		t.Errorf("el archivo pesa %d y el .data que reemplaza %d: no baja", informe.Bytes, tablaEnData)
	}

	t.Logf("PACK-242 bytes=%d registros=%d grupos=%d maxRegistro=%d crc=%d (el .data que reemplaza: %d)",
		informe.Bytes, informe.Registros, informe.Grupos, informe.MaxRegistro, informe.Crc, tablaEnData)

	fuente, err := abrirFuenteDeArchivo(ruta)
	if err != nil {
		t.Fatal(err)
	}

	if err := fuente.verifica(); err != nil {
		t.Fatalf("la verificacion del archivo: %v", err)
	}

	if fuente.crcVerificada != informe.Crc {
		t.Fatalf("el crc leido %d no es el del generador %d", fuente.crcVerificada, informe.Crc)
	}

	if fuente.Largo() != len(objectLookups) {
		t.Fatalf("el archivo dice %d registros y la tabla tiene %d", fuente.Largo(), len(objectLookups))
	}

	var rec ObjectLookupRecord

	for i := range objectLookups {
		esperado := &objectLookups[i]

		if err := fuente.Registro(i, &rec); err != nil {
			t.Fatalf("registro %d: %v", i, err)
		}

		if rec.Act != esperado.Act || rec.Type != esperado.Type || rec.Id != esperado.Id {
			t.Fatalf("registro %d: (act %d tipo %d id %d) != (act %d tipo %d id %d)",
				i, rec.Act, rec.Type, rec.Id, esperado.Act, esperado.Type, esperado.Id)
		}

		if rec.ObjectsTxtId != esperado.ObjectsTxtId || rec.MonstatsTxtId != esperado.MonstatsTxtId ||
			rec.Direction != esperado.Direction || rec.Index != esperado.Index {
			t.Fatalf("registro %d (%s): los numericos no son los mismos: %+v != %+v", i, esperado.Description, rec, *esperado)
		}

		for c := range punterosDeLosCamposDeTexto {
			if leido, escrito := *punterosDeLosCamposDeTexto[c](&rec), *punterosDeLosCamposDeTexto[c](esperado); leido != escrito {
				t.Fatalf("registro %d (%s): el campo de texto %d es %q y la tabla dice %q", i, esperado.Description, c, leido, escrito)
			}
		}
	}

	// y por (act, tipo, id), que es como lo pide el motor
	grupo := [3]int{1, int(d2enum.ObjectTypeItem), 0}
	if _, n, ok := fuente.Grupo(grupo[0], grupo[1]); !ok || n == 0 {
		t.Fatalf("el archivo no tiene el grupo act %d tipo %d", grupo[0], grupo[1])
	}

	base, _, _ := fuente.Grupo(grupo[0], grupo[1])

	if err := fuente.Registro(base+grupo[2], &rec); err != nil {
		t.Fatal(err)
	}

	objetos, err := fuente.ObjetosTxtId(base + grupo[2])
	if err != nil {
		t.Fatal(err)
	}

	if objetos != rec.ObjectsTxtId {
		t.Fatalf("ObjetosTxtId(%d) = %d y el registro entero dice %d", base+grupo[2], objetos, rec.ObjectsTxtId)
	}
}

// TestObjectLookupsAgrupados: el formato del disco (y Grupo()) dependen de que los registros de cada
// (act, tipo) sean CONTIGUOS, con los ids 0..n-1 en orden y con cada grupo apareciendo UNA sola vez. El test
// de contiguridad de DC-002.36 (TestObjectLookupIdsAreContiguous) mira los ids; esto ademas mira los grupos.
func TestObjectLookupsAgrupados(t *testing.T) {
	vistos := map[[2]int]bool{}
	var ultimo [2]int

	for i := range objectLookups {
		rec := &objectLookups[i]
		clave := [2]int{rec.Act, int(rec.Type)}

		if vistos[clave] {
			if clave != ultimo {
				t.Fatalf("registro %d (%s): el grupo act %d tipo %d vuelve a aparecer despues de act %d tipo %d",
					i, rec.Description, clave[0], clave[1], ultimo[0], ultimo[1])
			}
		} else {
			vistos[clave] = true
			ultimo = clave
		}
	}

	// y la fuente compilada tiene que ver los mismos grupos que el arranque en memoria
	fuente := &fuenteEnMemoria{registros: objectLookups}
	if n := fuente.Largo(); n != len(objectLookups) {
		t.Fatalf("la fuente dice %d registros y la tabla %d", n, len(objectLookups))
	}

	suma := 0

	for _, clave := range clavesDeGrupos(t) {
		_, n, ok := fuente.Grupo(clave[0], clave[1])
		if !ok {
			t.Fatalf("la fuente no ve el grupo act %d tipo %d", clave[0], clave[1])
		}

		suma += n
	}

	if suma != len(objectLookups) {
		t.Fatalf("los grupos suman %d registros y la tabla tiene %d", suma, len(objectLookups))
	}
}

func clavesDeGrupos(t *testing.T) [][2]int {
	t.Helper()

	var claves [][2]int

	for i := range objectLookups {
		clave := [2]int{objectLookups[i].Act, int(objectLookups[i].Type)}
		if n := len(claves); n > 0 && claves[n-1] == clave {
			continue
		}

		claves = append(claves, clave)
	}

	return claves
}

// TestLookupObjectDelArchivo: el indice de Object.Lookup se llena POR REGISTRO desde la fuente, y con la
// fuente del DISCO los registros son los mismos que los de la tabla compilada (es lo que la consola mide con
// su sonda). Tambien fija que el arranque ya NO vuelca la tabla: NewRecordManager no la toca.
func TestLookupObjectDelArchivo(t *testing.T) {
	ruta := filepath.Join(t.TempDir(), "objectlookups.pack")

	archivo, err := os.Create(ruta)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := EscribirPack(archivo, objectLookups); err != nil {
		t.Fatal(err)
	}

	if err := archivo.Close(); err != nil {
		t.Fatal(err)
	}

	fuente, err := abrirFuenteDeArchivo(ruta)
	if err != nil {
		t.Fatal(err)
	}

	if err := fuente.verifica(); err != nil {
		t.Fatal(err)
	}

	// el registro manager, con la fuente del disco puesta a mano
	anterior := fuenteRegistrada
	fuenteRegistrada = fuente

	defer func() { fuenteRegistrada = anterior }()

	rm, err := NewRecordManager(d2util.LogLevelDefault)
	if err != nil {
		t.Fatal(err)
	}

	// el arranque no volco la tabla: el indice esta vacio hasta que se pide un registro
	if rm.Object.Lookup != nil {
		t.Fatalf("el arranque dejo %d actos de Object.Lookup armados (tiene que estar vacio)", len(rm.Object.Lookup))
	}

	pruebas := [][3]int{
		{1, int(d2enum.ObjectTypeItem), 0},
		{1, int(d2enum.ObjectTypeItem), 722},
		{1, int(d2enum.ObjectTypeCharacter), 0},
		{2, int(d2enum.ObjectTypeItem), 150},
		{5, int(d2enum.ObjectTypeItem), 722},
	}

	for _, p := range pruebas {
		rec := rm.FindObject(p[0], p[1], p[2])
		if rec == nil {
			t.Fatalf("FindObject(%d, %d, %d) dio nil", p[0], p[1], p[2])
		}

		base, n, ok := fuente.Grupo(p[0], p[1])
		if !ok || p[2] >= n {
			t.Fatalf("el grupo (%d, %d) no tiene el id %d", p[0], p[1], p[2])
		}

		var esperado ObjectLookupRecord
		if err := fuente.Registro(base+p[2], &esperado); err != nil {
			t.Fatal(err)
		}

		if *rec != esperado {
			t.Fatalf("FindObject(%d, %d, %d) = %+v y el archivo dice %+v", p[0], p[1], p[2], *rec, esperado)
		}

		// y el segundo pedido sale del indice (el mismo puntero), sin volver a la fuente
		if otra := rm.FindObject(p[0], p[1], p[2]); otra != rec {
			t.Fatalf("FindObject(%d, %d, %d) no quedo en el indice", p[0], p[1], p[2])
		}
	}

	// un id que no esta: nil, sin morir
	if rec := rm.FindObject(1, int(d2enum.ObjectTypeItem), 999); rec != nil {
		t.Fatalf("FindObject(1, Item, 999) = %+v y tendria que ser nil", *rec)
	}
}

// TestDS1ObjectClassDelArchivo: la tabla del DS1 (5 x 150 numeros) sale IGUAL de la tabla compilada y del
// archivo del disco. Es el uso real de la tabla que la consola tiene que poder hacer sin los 7.891 registros
// en el binario: el camino del archivo lee SOLO los grupos de tipo Item y SOLO el campo ObjectsTxtId.
func TestDS1ObjectClassDelArchivo(t *testing.T) {
	ruta := filepath.Join(t.TempDir(), "objectlookups.pack")

	archivo, err := os.Create(ruta)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := EscribirPack(archivo, objectLookups); err != nil {
		t.Fatal(err)
	}

	if err := archivo.Close(); err != nil {
		t.Fatal(err)
	}

	// la tabla esperada, calculada a mano desde la tabla compilada (el camino de antes)
	var esperada [ds1Acts][ds1ObjectsPerAct]int

	for a := range esperada {
		for i := range esperada[a] {
			esperada[a][i] = -1
		}
	}

	for i := range objectLookups {
		o := &objectLookups[i]
		if o.Type != d2enum.ObjectTypeItem || o.Act < 1 || o.Act > ds1Acts || o.Id < 0 || o.Id >= ds1ObjectsPerAct {
			continue
		}

		esperada[o.Act-1][o.Id] = o.ObjectsTxtId
	}

	for k, v := range ds1ObjectOverrides {
		esperada[k[0]][k[1]] = v
	}

	fuente, err := abrirFuenteDeArchivo(ruta)
	if err != nil {
		t.Fatal(err)
	}

	if err := fuente.verifica(); err != nil {
		t.Fatal(err)
	}

	anterior := fuenteRegistrada
	fuenteRegistrada = fuente
	ds1ObjectOnce = sync.Once{}
	ds1ObjectTable = [ds1Acts][ds1ObjectsPerAct]int{}

	defer func() {
		fuenteRegistrada = anterior
		ds1ObjectOnce = sync.Once{}
		ds1ObjectTable = [ds1Acts][ds1ObjectsPerAct]int{}
	}()

	difieren := 0

	for a := 0; a < ds1Acts; a++ {
		for i := 0; i < ds1ObjectsPerAct; i++ {
			if v := DS1ObjectClass(a, i); v != esperada[a][i] {
				difieren++
				if difieren <= 5 {
					t.Errorf("DS1ObjectClass(%d, %d) = %d y la tabla compilada dice %d", a, i, v, esperada[a][i])
				}
			}
		}
	}

	if difieren != 0 {
		t.Fatalf("%d de los %d numeros de la tabla del DS1 no coinciden", difieren, ds1Acts*ds1ObjectsPerAct)
	}
}

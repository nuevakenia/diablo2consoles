package d2loader

// DC-002.43 -- LAS MARCAS DE CARGA DEL FILE LOADER.
//
// QUE PROBLEMA RESUELVE: el arranque del motor puede morirse SIN imprimir nada (la consola reinicia y no
// queda ni un caracter). Las lineas `[File Loader] Loaded ...` salen DESPUES de que la carga salio bien, asi
// que no dicen nada del pedido que estaba en curso cuando el motor murio: entre el ultimo `Loaded` y la
// muerte puede haber un pedido de otro asset (que no llego a imprimirse) o el parseo del que si se leyo.
//
// ESTAS MARCAS CIERRAN ESE HUECO: se marca ENTRAR y VOLVER de cada `Loader.Load` (el unico camino por el que
// el Asset Manager pide un archivo) y ENTRAR y VOLVER de LA LECTURA de cada asset. La ultima marca de la
// corrida dice la llamada exacta en la que estaba el motor, y el que la imprime agrega la memoria de ese
// instante (`console/dc/std/csrc/marcas_carga.c`).
//
// QUE ES "LA LECTURA" DE LOS PUNTOS 3 Y 4 (DC-002.45): hay DOS formas de leer un asset y las dos usan el
// mismo par de marcas, porque para el que lee el log significan lo mismo (que asset estaba leyendo el motor):
//
//   * la lectura COMPLETA (`LoadFile` -> `ioutil.ReadAll`, el asset ENTERO a memoria): las pone `LoadFile`;
//   * la lectura POR TROZOS (las tablas del RecordManager: el `csv.Reader` consume el stream del MPQ por
//     SECTOR, sin llevar el archivo a memoria): las pone `LoadRecords`, que es el que CONSUME la tabla.
//
// No se agregaron dos puntos nuevos a proposito: `lee-entra`/`lee-volvio` quieren decir "aca empieza/termina
// la lectura de este asset" en los dos caminos, y partirlos en cuatro nombres dejaria a las evidencias de
// DC-002.43 y a las de DC-002.45 hablando de marcas distintas.
//
// COMO SE PARTE ENTRE HOST Y CONSOLA (y por que asi): `Marca` tiene DOS definiciones, en dos archivos que se
// excluyen mutuamente:
//
//   marca_carga_host.go   el host (build tag `!dreamcast`): `Marca` NO HACE NADA. El host no imprime ni
//                         cambia nada: es el mismo camino que corren las pruebas de la suite.
//   marca_carga_godc.go   la consola (build tag `dreamcast`): llama al C de las marcas.
//
// El `go` de verdad respeta el tag y saca el archivo que no toca. gccgo NO respeta tags (medido en
// DC-002.42, con dos archivos mutuamente excluyentes), asi que los scripts del build de la consola sacan
// `marca_carga_host.go` POR NOMBRE (`build-motor-all.sh` y `recompilar-paquete-motor.sh`, la misma regla que
// usa `object_lookups_host.go`). Si esa exclusion se pierde, gccgo NO falla callado: el paquete no compila
// con dos `Marca`. Es la misma forma que ya usa el motor para partir codigo entre host y consola.
//
// ESTE ARCHIVO (el de las constantes y el porque) lo compilan los dos lados: no lleva tag.

// Los puntos que se marcan. Los numeros los traduce a texto el C de la consola: un punto nuevo se agrega en
// los dos lados.
const (
	MarcaInstrumento = 0 // la marca de control: la imprime el propio paquete al arrancar
	MarcaEntra       = 1 // entrando a Loader.Load con esta ruta
	MarcaVolvio      = 2 // Loader.Load devolvio el archivo abierto
	MarcaLeeEntra    = 3 // entrando a la lectura del asset (ReadAll, o el consumo por trozos de una tabla)
	MarcaLeeVolvio   = 4 // la lectura del asset termino
)

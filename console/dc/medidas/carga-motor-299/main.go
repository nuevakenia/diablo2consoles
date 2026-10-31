// Medidor del camino del MOTOR con dueno (DC-002.99) en el HOST: los ocho `.dt1` del pueblo del acto 1
// cargados por `d2asset.LoadFile` -- la puerta de entrada de TODAS las cargas grandes del motor -- con las
// palancas de la consola prendidas, y el instrumento del tramo leido al lado.
//
// PARA QUE. La corrida de la Dreamcast (`console/dc/poc/carga-motor-299`) mide el MONTON DE C, que en el host
// no existe. Lo que el host puede medir, y es lo que hay que fijar antes de cruzar a la consola, es:
//
//  1. QUE LO LEIDO SEA EL ARCHIVO. Los ocho crc32 por `LoadFile` contra el crc32 del mismo archivo leido
//     derecho del MPQ (`mpq.ReadFile`). Si uno no coincide, el tramo esta sirviendo otra cosa.
//  2. QUE EL TRAMO SEA UNO. Cuantos tramos se pidieron para ocho archivos (`Pedidos`), cuantos se reusaron
//     (`Reusos`) y que despues de `SoltarLecturasDeConsola` no quede ninguno vivo.
//  3. QUE LA PC NO SE ENTERE. Con la palanca apagada (`SIN_TRAMOS`) no se pide ni un tramo: cada archivo
//     trae su buffer, que es el camino de siempre.
//
// Uso:
//
//	go run ./console/dc/medidas/carga-motor-299 [-mpq ruta/d2data.mpq]
//
// Sin -mpq usa la instalacion del juego (D2_DIR o ~/.wine-d2/drive_c/Games/Diablo II).
package main

import (
	"errors"
	"flag"
	"fmt"
	"hash/crc32"
	"os"
	"path/filepath"
	"strings"

	"github.com/OpenDiablo2/OpenDiablo2/console/dc/fuenteconsola"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2ds1"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2mpq"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2util"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2asset"
)

const (
	// OJO: con barras NORMALES. La fuente las traduce al leer del MPQ (`limpiar`), y asi el nombre no lleva
	// ni una barra invertida que se pueda escapar de mas.
	ds1Pueblo = "data/global/tiles/ACT1/TOWN/townN1.ds1"
)

// la barra invertida, armada por codigo: el MPQ es literal con ella y escribirla en un literal de Go se
// presta a que quede doblada
const barraInvertida = string(rune(92))

func main() {
	ruta := flag.String("mpq", "", "ruta del d2data.mpq (por defecto, la instalacion del juego)")
	flag.Parse()

	if err := correr(*ruta); err != nil {
		fmt.Println("MEDIDOR299: FALLO --", err)
		os.Exit(1)
	}
}

func correr(rutaMPQ string) error {
	if rutaMPQ == "" {
		rutaMPQ = filepath.Join(autodetectarD2(), "d2data.mpq")
	}

	fuente, err := fuenteconsola.Abrir(rutaMPQ)
	if err != nil {
		return fmt.Errorf("abriendo %s: %w", rutaMPQ, err)
	}
	defer func() { _ = fuente.Cerrar() }()

	fmt.Println("MEDIDOR299: mpq =", rutaMPQ)

	// el AssetManager del MOTOR: `NewAssetManager` es el camino de la PC (d2app/app.go), y la fuente de
	// consola se le pone delante con `RegistrarEn`, que es lo unico que hace falta para que las cargas pasen
	// por ella. No se toca ninguna linea del motor.
	am, err := d2asset.NewAssetManager(d2util.LogLevelError)
	if err != nil {
		return fmt.Errorf("NewAssetManager: %w", err)
	}

	fuente.RegistrarEn(am.Loader)

	// el .ds1 del pueblo: sus ocho `.dt1` son los archivos de la medicion
	nombres, err := losOchoDT1(fuente)
	if err != nil {
		return err
	}

	// la referencia: el mismo archivo leido derecho del MPQ (`mpq.ReadFile`, el camino de siempre), con un
	// segundo manejador para no tocar la fuente que se esta midiendo
	m, err := d2mpq.FromFile(fuente.Path())
	if err != nil {
		return fmt.Errorf("abriendo el MPQ crudo: %w", err)
	}

	defer func() { _ = m.Close() }()

	referencia := make(map[string]uint32, len(nombres))

	for _, nombre := range nombres {
		crudo, err := m.ReadFile(strings.ReplaceAll(nombre, "/", barraInvertida))
		if err != nil {
			return fmt.Errorf("leyendo %s del MPQ: %w", nombre, err)
		}

		referencia[nombre] = crc32.ChecksumIEEE(crudo)

		fmt.Printf("MEDIDOR299: REFERENCIA %-46s bytes=%8d crc32=%d\n", archivo(nombre), len(crudo), referencia[nombre])
	}

	// ================= A) LA PC: sin tramo, cada archivo con su buffer =================
	if err := bloqueSinTramo(am, nombres, referencia); err != nil {
		return err
	}

	// ================= B) LA CONSOLA: el tramo prestado, uno para los ocho =================
	if err := bloqueConTramo(am, nombres, referencia); err != nil {
		return err
	}

	fmt.Println("MEDIDOR299: OK -- los ocho .dt1 dan el mismo crc32 por el motor, y el tramo es uno")

	return nil
}

// bloqueSinTramo: el camino de la PC (y el de la consola de hoy): `ActivarLecturaDeConsola` sin la palanca del
// tramo. Cada archivo trae su buffer; el instrumento del tramo tiene que quedar en CERO.
func bloqueSinTramo(am *d2asset.AssetManager, nombres []string, referencia map[string]uint32) error {
	fmt.Println("MEDIDOR299: ===== A) SIN TRAMO (el camino de la PC) =====")

	// el camino de DC-002.79: reserva el tamano, pero por archivo. Es el defecto que cierra la tarjeta.
	d2asset.ActivarLecturaDeConsola()

	antes := d2asset.EstadoDeLosTramos()

	for _, nombre := range nombres {
		datos, err := am.LoadFile(nombre)
		if err != nil {
			return fmt.Errorf("LoadFile %s: %w", nombre, err)
		}

		if crc := crc32.ChecksumIEEE(datos); crc != referencia[nombre] {
			return fmt.Errorf("%s: crc32 %d y el MPQ dice %d", nombre, crc, referencia[nombre])
		}
	}

	e := d2asset.EstadoDeLosTramos()

	if pedidos := e.Pedidos - antes.Pedidos; pedidos != 0 {
		return fmt.Errorf("sin la palanca se pidieron %d tramos, quiero 0", pedidos)
	}

	fmt.Println("MEDIDOR299: SIN-TRAMO OK -- los", len(nombres), "archivos leidos con buffer propio (0 tramos)")

	return nil
}

// bloqueConTramo: la palanca de DC-002.99. Ocho archivos, UN tramo (mas sus crecimientos), y al soltar no
// queda ninguno vivo.
func bloqueConTramo(am *d2asset.AssetManager, nombres []string, referencia map[string]uint32) error {
	fmt.Println("MEDIDOR299: ===== B) CON TRAMO (DC-002.99) =====")

	d2asset.ActivarLecturasPorTramos()

	antes := d2asset.EstadoDeLosTramos()

	for _, nombre := range nombres {
		datos, err := am.LoadFile(nombre)
		if err != nil {
			return fmt.Errorf("LoadFile %s: %w", nombre, err)
		}

		crc := crc32.ChecksumIEEE(datos)
		if crc != referencia[nombre] {
			return fmt.Errorf("%s: crc32 %d y el MPQ dice %d", nombre, crc, referencia[nombre])
		}

		// el tramo vigente SIEMPRE es mas chico o igual al archivo mas grande leido: el crecimiento se ve aca
		e := d2asset.EstadoDeLosTramos()
		fmt.Printf("MEDIDOR299: TRAMO %-46s bytes=%8d crc32=%d tramo=%8d reusos=%d pedidos=%d\n",
			archivo(nombre), len(datos), crc, e.BytesDelTramo, e.Reusos-antes.Reusos, e.Pedidos-antes.Pedidos)
	}

	e := d2asset.EstadoDeLosTramos()

	pedidos, reusos := e.Pedidos-antes.Pedidos, e.Reusos-antes.Reusos

	if pedidos < 1 || pedidos > 3 {
		return fmt.Errorf("se pidieron %d tramos para %d archivos: el reservorio no se esta reusando", pedidos, len(nombres))
	}

	if reusos != len(nombres)-pedidos {
		return fmt.Errorf("reusos=%d, quiero %d (%d archivos - %d tramos)", reusos, len(nombres)-pedidos, len(nombres), pedidos)
	}

	// SOLTAR: el tramo vuelve y no queda nada pedido
	d2asset.SoltarLecturasDeConsola()

	e = d2asset.EstadoDeLosTramos()

	if e.BytesDelTramo != 0 || e.Pedidos != e.Liberados {
		return fmt.Errorf("despues de soltar: bytesDelTramo=%d pedidos=%d liberados=%d",
			e.BytesDelTramo, e.Pedidos, e.Liberados)
	}

	fmt.Printf("MEDIDOR299: TRAMO OK -- %d archivos en %d tramos (%d reusos), y al soltar pedidos=%d liberados=%d bytesDelTramo=%d\n",
		len(nombres), pedidos, reusos, e.Pedidos, e.Liberados, e.BytesDelTramo)

	return nil
}

// losOchoDT1: los `.dt1` que el `.ds1` del pueblo lista, con la ruta que usa el motor
// (`data/global/tiles/...`). Se lee el `.ds1` por la fuente, que es la misma pieza que usa la consola.
func losOchoDT1(fuente *fuenteconsola.Fuente) ([]string, error) {
	tramo, err := fuente.Pedir(ds1Pueblo)
	if err != nil {
		return nil, fmt.Errorf("pidiendo el .ds1 del pueblo: %w", err)
	}

	d, err := d2ds1.Unmarshal(tramo.Datos)
	tramo.Liberar()

	if err != nil {
		return nil, fmt.Errorf("parseando el .ds1 del pueblo: %w", err)
	}

	nombres := make([]string, 0, len(d.Files))

	for _, f := range d.Files {
		nombres = append(nombres, rutaDelDT1(f))
	}

	if len(nombres) != 8 {
		return nil, errors.New("el .ds1 del pueblo no lista ocho .dt1")
	}

	return nombres, nil
}

// rutaDelDT1: la traduccion del nombre del `.ds1` a la ruta del motor. Es la MISMA de `RutaDelDT1` del
// compositor (`console/dc/poc/personaje-mapa/nivel.go`), copiada aca para que el medidor no dependa del POC.
func rutaDelDT1(f string) string {
	f = strings.ToLower(f)
	f = strings.ReplaceAll(f, "c:", "")
	f = strings.ReplaceAll(f, ".tg1", ".dt1")

	if i := strings.LastIndex(f, "tiles"); i >= 0 {
		f = f[i+len("tiles"):]
	}

	f = strings.TrimPrefix(f, barraInvertida)
	f = strings.TrimPrefix(f, "/")

	return "data/global/tiles/" + f
}

// archivo: el nombre corto, para que la tabla entre en una linea.
func archivo(ruta string) string {
	if i := strings.LastIndex(ruta, "/"); i >= 0 {
		return ruta[i+1:]
	}

	return ruta
}

// autodetectarD2: la instalacion del juego no vive en el repo.
func autodetectarD2() string {
	if d := os.Getenv("D2_DIR"); d != "" {
		return d
	}

	return filepath.Join(os.Getenv("HOME"), ".wine-d2", "drive_c", "Games", "Diablo II")
}

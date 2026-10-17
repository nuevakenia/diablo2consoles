//go:build dreamcast

// DC-002.18: el CORPUS DE LA CONSOLA del config.json del motor.
//
// QUE MIDE: que el `d2config.Configuration` del motor (el tipo REAL, no una copia) se pueda GUARDAR y
// LEER en la consola, y que el archivo que queda en el sistema de archivos escribible sea byte a byte el
// que escribe la biblioteca de verdad en el host. Los bytes esperados y la configuracion de prueba los
// hornea console/dc/gen-config-json con el json de Go (una struct espejo sin metodos): esto no se compara
// contra si mismo.
//
// LOS DOS CAMINOS DEL MOTOR, los dos aca:
//   - ESCRIBIR: `config.Save()` -> json.MarshalIndent (d2config.go:61) -> nuestro MarshalJSON + el Indent
//     del json de la consola.
//   - LEER: `json.NewDecoder(archivo).Decode(config)` (d2app/app.go:277) -> nuestro Decode -> UnmarshalJSON.
//
// Uso:  scripts/console/checar-json-218.sh   (arma la gox, compila esto, lo corre en Flycast y mira los
//       controles; tambien corre los mutantes).
package main

import (
	"encoding/json"
	"os"

	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2config"
)

//extern d2c_serial_init
func d2cSerialInit()

const (
	dirPrueba = "/rd/json218"
	ruta      = "/rd/json218/config.json"
	rutaCmp   = "/rd/json218/config-compacto.json"
)

func main() {
	d2cSerialInit()

	// el instrumento: a partir de aca, cada serializacion imprime por donde paso y sobre que tipo (es el
	// mismo instrumento que usa el arranque del motor para medir cuantas llamadas hace)
	instrumentoJSON()

	println("JSON-CONFIG arranca (DC-002.18: el config.json del motor en la consola)")

	caso("prueba", configDePrueba(), esperadoJSON)
	caso("variante", configVariante(), esperadoVarianteJSON)

	println("JSON-CONFIG fin")
}

func caso(nombre string, c *d2config.Configuration, esperado string) {
	if err := os.MkdirAll(dirPrueba, 0750); err != nil {
		println("JSON-CONFIG", nombre, "FALLO: MkdirAll:", err.Error())

		return
	}

	c.SetPath(ruta)

	if err := c.Save(); err != nil {
		println("JSON-CONFIG", nombre, "FALLO: Save:", err.Error())

		return
	}

	datos, err := os.ReadFile(ruta)
	if err != nil {
		println("JSON-CONFIG", nombre, "FALLO: ReadFile:", err.Error())

		return
	}

	igual := string(datos) == esperado
	println("JSON-CONFIG", nombre, "guardado bytes=", len(datos), "esperados=", len(esperado), "igual=", igual)

	if !igual {
		println("JSON-CONFIG", nombre, "primera diferencia en el byte", primeraDiferencia(string(datos), esperado), "de", len(esperado))
	}

	// LA VUELTA: leer el archivo con el Decoder, que es el camino de d2app.LoadConfig
	archivo, err := os.Open(ruta)
	if err != nil {
		println("JSON-CONFIG", nombre, "FALLO: Open:", err.Error())

		return
	}

	otra := &d2config.Configuration{}
	if err := json.NewDecoder(archivo).Decode(otra); err != nil {
		println("JSON-CONFIG", nombre, "FALLO: Decode:", err.Error())
		archivo.Close()

		return
	}

	archivo.Close()

	// la vuelta se cierra reescribiendo lo leido: tiene que dar los MISMOS bytes
	otrosDatos, err := json.MarshalIndent(otra, "", "  ")
	if err != nil {
		println("JSON-CONFIG", nombre, "FALLO: MarshalIndent de la vuelta:", err.Error())

		return
	}

	println("JSON-CONFIG", nombre, "vuelta bytes=", len(otrosDatos), "igual=", string(otrosDatos) == esperado)

	// y el Encoder (compacto, con su salto de linea): lo que escribe se lee de vuelta igual
	salida, err := os.Create(rutaCmp)
	if err != nil {
		println("JSON-CONFIG", nombre, "FALLO: Create del compacto:", err.Error())

		return
	}

	if err := json.NewEncoder(salida).Encode(c); err != nil {
		println("JSON-CONFIG", nombre, "FALLO: Encode:", err.Error())
		salida.Close()

		return
	}

	salida.Close()

	compacto, err := os.ReadFile(rutaCmp)
	if err != nil {
		println("JSON-CONFIG", nombre, "FALLO: ReadFile del compacto:", err.Error())

		return
	}

	tercera := &d2config.Configuration{}
	if err := json.Unmarshal(compacto, tercera); err != nil {
		println("JSON-CONFIG", nombre, "FALLO: Unmarshal del compacto:", err.Error())

		return
	}

	tercerosDatos, err := json.MarshalIndent(tercera, "", "  ")
	if err != nil {
		println("JSON-CONFIG", nombre, "FALLO: MarshalIndent del compacto:", err.Error())

		return
	}

	println("JSON-CONFIG", nombre, "encoder bytes=", len(compacto), "vuelta igual=", string(tercerosDatos) == esperado)
}

// primeraDiferencia: en que byte se separan dos textos (-1 si son iguales). Es el dato que hace falta para
// arreglar una comparacion que falla: "no da" no sirve.
func primeraDiferencia(a, b string) int {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}

	for i := 0; i < n; i++ {
		if a[i] != b[i] {
			return i
		}
	}

	if len(a) != len(b) {
		return n
	}

	return -1
}

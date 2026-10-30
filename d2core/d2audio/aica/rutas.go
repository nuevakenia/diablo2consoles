package aica

import "strings"

// rutaEnElMPQ pasa una ruta de d2soundpath a la CLAVE con que el MPQ guarda el archivo.
//
// MEDIDO, y por eso existe (lo cazo la primera corrida del POC, que buscaba y no encontraba nada): el MPQ
// guarda los nombres con BARRA INVERTIDA (`DATA\GLOBAL\SFX\combat\impact\sword5.wav`, ver
// console/dc/sonda-sfx) y su busqueda es el hash del nombre EXACTO, sin normalizar separadores
// (d2mpq.hashString solo pasa a mayusculas). Pasarle la ruta con "/" que devuelve d2soundpath no encuentra
// NADA, y el error que sale es "el sonido no esta en el MPQ", que manda a buscar el problema en el archivo.
// El motor hace esta misma conversion en su fuente de MPQ (d2common/d2loader/mpq/source.go:cleanName:
// ReplaceAll("/", "\")). Aca se hace antes de Contains y de ReadFile.
func rutaEnElMPQ(ruta string) string {
	clave := strings.ReplaceAll(ruta, "/", "\\")

	for len(clave) > 0 && clave[0] == '\\' {
		clave = clave[1:]
	}

	return clave
}

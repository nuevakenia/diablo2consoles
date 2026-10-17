// Package ioutil: las funciones de siempre de ioutil, para la consola.
//
// En Go moderno ioutil esta deprecado (todo eso vive en io y os), pero el motor todavia lo importa: se
// implementa como una capa fina sobre io y os, que es literalmente lo que hace el de Go.
package ioutil

import (
	"d2std/io"
	"d2std/os"
	"d2std/sort"
)

// ReadAll lo mismo que io.ReadAll.
func ReadAll(r io.Reader) ([]byte, error) { return io.ReadAll(r) }

// ReadFile lo mismo que os.ReadFile.
func ReadFile(filename string) ([]byte, error) { return os.ReadFile(filename) }

// WriteFile lo mismo que os.WriteFile. En la consola el sistema de archivos es de SOLO LECTURA
// (romdisk): esto compila y existe, pero escribir no funciona. Es un limite declarado, no un bug.
func WriteFile(filename string, data []byte, perm os.FileMode) error {
	return os.WriteFile(filename, data, perm)
}

// ReadDir: la version vieja devolvia []os.FileInfo y ordenaba por nombre. Aca se arma sobre os.ReadDir
// (que devuelve DirEntry), que es lo que el os de la consola sabe hacer: Open + Readdir no existe.
func ReadDir(dirname string) ([]os.FileInfo, error) {
	ents, err := os.ReadDir(dirname)
	if err != nil {
		return nil, err
	}
	list := make([]os.FileInfo, 0, len(ents))
	for _, e := range ents {
		fi, err := e.Info()
		if err != nil {
			return nil, err
		}
		list = append(list, fi)
	}
	sort.Slice(list, func(i, j int) bool { return list[i].Name() < list[j].Name() })
	return list, nil
}

// TempDir: no hay donde crear temporales (romdisk de solo lectura). Se declara y devuelve el error.
func TempDir(dir, pattern string) (string, error) {
	return "", os.ErrNotExist
}

// TempFile: igual que TempDir.
func TempFile(dir, pattern string) (*os.File, error) {
	return nil, os.ErrNotExist
}

// NopCloser: el de io, que es de donde salio.
func NopCloser(r io.Reader) io.ReadCloser { return io.NopCloser(r) }

// Discard: el de io.
var Discard io.Writer = io.Discard

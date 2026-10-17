// Package user: el usuario del sistema, en la version que la consola puede tener.
//
// Para que se usa en el juego: d2config/defaults.go pregunta el usuario actual para armar la carpeta de
// configuracion del jugador. En la Dreamcast no hay usuarios, ni contrasenas, ni home de nadie: hay un
// jugador solo.
//
// Por eso Current devuelve un jugador generico con el home puesto en el romdisk ("/rd"), que es donde
// vive todo lo que el juego puede leer (el romdisk es de solo lectura y es literalmente el disco del
// juego). Eso hace que los paths de configuracion apunten a un lugar real y no a una carpeta inventada.
package user

import "errors"

// ErrNoExiste: el usuario que se busca no esta (existe para la forma de la biblioteca).
var ErrNoExiste = errors.New("user: unknown user")

// User: los datos del usuario, como en os/user.
type User struct {
	Uid      string
	Gid      string
	Username string
	Name     string
	HomeDir  string
}

// Current: el jugador. En la consola hay uno solo.
func Current() (*User, error) {
	return &User{
		Uid:      "0",
		Gid:      "0",
		Username: "player",
		Name:     "Player",
		HomeDir:  "/rd",
	}, nil
}

// Lookup: lo mismo, por nombre o por id. En la consola hay un usuario y es ese.
func Lookup(nombre string) (*User, error) { return Current() }

// LookupId: igual (el nombre se ignora: hay uno solo).
func LookupId(uid string) (*User, error) { return Current() }

// Group: el grupo. En la consola no hay grupos.
type Group struct {
	Gid  string
	Name string
}

// LookupGroup: el grupo unico.
func LookupGroup(nombre string) (*Group, error) { return &Group{Gid: "0", Name: "player"}, nil }

// LookupGroupId: lo mismo por id.
func LookupGroupId(gid string) (*Group, error) { return LookupGroup("") }

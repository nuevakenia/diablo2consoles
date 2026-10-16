// Prueba diferencial de DC-002.6b: path/filepath.
//
// Este paquete es PURO TEXTO, asi que se puede hacer lo que con los paquetes de calculo: cada funcion
// se compara caso por caso contra la de Go, en el host. Si una sola diferencia de criterio se escapa,
// aca se ve. (El corpus de la consola corre los MISMOS casos, y ademas prueba que el codigo compilado
// para SH-4 da lo mismo que el del host.)
package difftest

import (
	realfp "path/filepath"
	"testing"

	myfp "d2std/path/filepath"
)

// caminos: los casos de texto, elegidos para pegarle a los rincones del algoritmo de Clean.
var caminos = []string{
	"", ".", "..", "/", "//", "///",
	"a", "a/b", "a/b/", "/a/b/", "a//b", "a/./b", "a/../b",
	"../a", "/../a", "a/b/../..", "a/b/../../c", "./a", "a/.", "/a",
	"//a//b//", "a.txt", "/a/b.txt", "a.", ".bashrc", "a/b/.x.y",
	"..", "../..", "/..", "a/..", "x/y/../z/./w",
	"a b/c d/e.txt", "/a/b c/.oculto", "a/././b", "a/b/c/./.././d",
	"uno/../dos/../tres", "/uno/../..", "x/../../y", "muy/largo/" + "x/../" + "y",
}

func compara(t *testing.T, nombre, esperado, tengo string) {
	t.Helper()
	if esperado != tengo {
		t.Errorf("%s: Go da %q y el mio %q", nombre, esperado, tengo)
	}
}

func TestFilepathCleanBaseDirExtSplit(t *testing.T) {
	for _, c := range caminos {
		compara(t, "Clean("+c+")", realfp.Clean(c), myfp.Clean(c))
		compara(t, "Base("+c+")", realfp.Base(c), myfp.Base(c))
		compara(t, "Dir("+c+")", realfp.Dir(c), myfp.Dir(c))
		compara(t, "Ext("+c+")", realfp.Ext(c), myfp.Ext(c))
		d1, f1 := realfp.Split(c)
		d2, f2 := myfp.Split(c)
		compara(t, "Split("+c+") dir", d1, d2)
		compara(t, "Split("+c+") arch", f1, f2)
	}
}

func TestFilepathJoin(t *testing.T) {
	casos := [][]string{
		{}, {""}, {""}, {"a"}, {"a", "b"}, {"a", "", "b"}, {"", "a"}, {"", "", ""},
		{"/", "a"}, {"a/", "/b"}, {"a", "..", "b"}, {".", "a"}, {"a", "./b"},
		{"a/b", "../c"}, {"/a/b", "c"}, {"a", "/b"}, {"a/", "b/"}, {"a//", "//b"},
		{"uno", "dos", "tres.txt"}, {"uno/dos", "..", "tres"}, {"/", "/"}, {"..", ".."},
		{"dir/", "", "archivo"}, {"a", "b", "c", "d", "e"},
	}
	for _, c := range casos {
		compara(t, "Join("+Concatena(c)+")", realfp.Join(c...), myfp.Join(c...))
	}
}

// Concatena: los elementos de un caso de Join, para el mensaje del test.
func Concatena(elems []string) string {
	s := ""
	for i, e := range elems {
		if i > 0 {
			s += ","
		}
		s += "\"" + e + "\""
	}
	return s
}

func TestFilepathIsAbsSplitList(t *testing.T) {
	for _, c := range caminos {
		if realfp.IsAbs(c) != myfp.IsAbs(c) {
			t.Errorf("IsAbs(%q): Go da %v y el mio %v", c, realfp.IsAbs(c), myfp.IsAbs(c))
		}
	}
	listas := []string{"", "a", "a:b", "a::b", ":a", "a:", ":/rd:/cd", "uno:dos:tres"}
	for _, c := range listas {
		esp := realfp.SplitList(c)
		mio := myfp.SplitList(c)
		if len(esp) != len(mio) {
			t.Errorf("SplitList(%q): Go da %d partes y el mio %d", c, len(esp), len(mio))
			continue
		}
		for i := range esp {
			compara(t, "SplitList("+c+")["+Itoa(i)+"]", esp[i], mio[i])
		}
	}
}

// Itoa local: el difftest no importa strconv para esto.
func Itoa(n int) string {
	if n == 0 {
		return "0"
	}
	s := ""
	for n > 0 {
		s = string(rune('0'+n%10)) + s
		n /= 10
	}
	return s
}

// TestFilepathAbs: el host SI tiene directorio actual, la consola NO (no hay procesos). Lo que se
// exige aca es lo que vale en los dos: que Abs deje un camino absoluto y limpio.
func TestFilepathAbs(t *testing.T) {
	for _, c := range []string{"/a/b", "a/b", "a/../b", "/a/./b", "/"} {
		mio, err := myfp.Abs(c)
		if err != nil {
			t.Errorf("Abs(%q) dio error: %v", c, err)
			continue
		}
		if !myfp.IsAbs(mio) {
			t.Errorf("Abs(%q) = %q y no es absoluto", c, mio)
		}
		if mio != myfp.Clean(mio) {
			t.Errorf("Abs(%q) = %q y no esta limpio", c, mio)
		}
	}
}

func TestFilepathToSlash(t *testing.T) {
	for _, c := range caminos {
		compara(t, "ToSlash("+c+")", realfp.ToSlash(c), myfp.ToSlash(c))
		compara(t, "FromSlash("+c+")", realfp.FromSlash(c), myfp.FromSlash(c))
	}
}

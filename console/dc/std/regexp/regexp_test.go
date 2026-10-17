// Prueba del regexp de la consola contra el regexp DE VERDAD (el de Go).
//
// Por que existe: el regexp de la consola es codigo nuevo y delicado (un motor de vuelta atras escrito a
// mano). Que compile no dice nada, y que el motor compile tampoco: lo que importa es que para los
// MISMOS patrones que usa el motor devuelva el MISMO resultado que el de Go.
//
// Los patrones de abajo se sacaron midiendo el motor (grep de regexp.MustCompile), no inventando.
//
// Para correr: cd console/dc/std && go test ./regexp/
package regexp

import (
	goregexp "regexp"
	"testing"
)

// los patrones que usa el motor, con textos de prueba
var casos = []struct {
	patron string
	textos []string
}{
	{`\[[a-z]+\]`, []string{"hola [color] mundo", "sin tokens", "[abc]", "[ABC]", ""}},
	{`[^a-z0-9\-]+`, []string{"abc-DEF--123", "todo-junto", "!!!", ""}},
	{`^cm[123]$`, []string{"cm1", "cm2", "cm3", "cm4", "cm12", "xcm1", ""}},
	{`\d+`, []string{"a12b345", "sin numeros", "0", ""}},
	{`\D`, []string{"123a", "123", "abc", ""}},
	{`^-?\d+$`, []string{"123", "-123", "+123", "12a", "", "-"}},
	{`^g[cfszvlp][a-z]$`, []string{"gca", "gpa", "gxa", "gc1", "gcz", ""}},
	{`^r[0-3][0-9]$`, []string{"r00", "r39", "r40", "r0", ""}},
	{`^(pk[123]|tes|ceh|bet|fed)`, []string{"pk1", "pk3", "tes", "ceh", "bet", "fed", "zzz", ""}},
	{`skill\('([^']+)`, []string{"skill('Fireball')", "skill(foo)", "nada", ""}},
	{`^(tes|ceh|bet|fed)`, []string{"tesx", "ceh", "bet", "fed", "x", ""}},
	{`[0-9]{2,3}`, []string{"12", "123", "1234", "1", ""}},
	{`a+b*c?`, []string{"aab", "ac", "b", "aaabbb", ""}},
	// DC-002.16: la bandera de plegado de mayusculas (?i). El motor la usa en
	// d2core/d2map/d2mapgen/real_maze_gen.go:33 y ninguno de los casos de arriba la cubria: la suite
	// estaba verde y el arranque del motor paniqueaba con un *regexp.Error de nivel de paquete.
	// MEDIDO antes del arreglo: Compile(`(?i)a`) devolvia `regexp: sobra )a en (?i)a`, porque el corte
	// del prefijo era p[3:] y "(?i)" mide CUATRO bytes (quedaba "i)a" y el parser se plantaba en el ')'.
	{`(?i)a`, []string{"A", "a", "b", "aAa", "BANANA", ""}},
	{`(?i)^abc$`, []string{"ABC", "abc", "AbC", "abcd", ""}},
	{`(?i)[a-z]\d*\.ds1$`, []string{"A1.ds1", "z.ds1", "Z9.DS1", "1.ds1", ""}},
	{`(?i)(pre|prev|up|ent|exit)\d*\.ds1$`, []string{
		"PRE1.ds1", "pre1.ds1", "Up.ds1", "EXIT34.ds1", "ent.ds1", "prev2.ds1",
		"data/global/tiles/PRE1.ds1", "x.ds1", "exit.ds2", "PrE.ds1", ""}},
	// el patron VACIO y "(?i)" solo: NO son un limite (se midieron contra Go: MatchString, FindString y
	// FindAllString sobre "", "a" y "abc" dan lo mismo). "(?i)" solo queda vacio al sacarle la bandera,
	// asi que es el mismo camino con el flag puesto.
	{``, []string{"", "a", "abc"}},
	{`(?i)`, []string{"", "a", "abc"}},
}

func TestContraElRegexpDeGo(t *testing.T) {
	for _, caso := range casos {
		// Compile (y no MustCompile) para que un patron que no compila se informe como un fallo del
		// corpus y no se lleve puesto el proceso de test entero: con MustCompile, un `(?i)` mal cortado
		// abortaba TODA la suite con un panic y los demas casos no se corrian (medido en DC-002.16).
		nuestro, errComp := Compile(caso.patron)
		if errComp != nil {
			t.Errorf("Compile(%q) fallo: %v", caso.patron, errComp)
			continue
		}
		verdadero := goregexp.MustCompile(caso.patron)

		for _, texto := range caso.textos {
			// 1) coincide o no
			if nuestro.MatchString(texto) != verdadero.MatchString(texto) {
				t.Errorf("MatchString(%q) en %q: nuestro=%v go=%v",
					caso.patron, texto, nuestro.MatchString(texto), verdadero.MatchString(texto))
			}

			// 2) la primera coincidencia
			if nuestro.FindString(texto) != verdadero.FindString(texto) {
				t.Errorf("FindString(%q) en %q: nuestro=%q go=%q",
					caso.patron, texto, nuestro.FindString(texto), verdadero.FindString(texto))
			}

			// 3) todas
			nuestras := nuestro.FindAllString(texto, -1)
			verdaderas := verdadero.FindAllString(texto, -1)
			if len(nuestras) != len(verdaderas) {
				t.Errorf("FindAllString(%q) en %q: nuestro=%v (%d) go=%v (%d)",
					caso.patron, texto, nuestras, len(nuestras), verdaderas, len(verdaderas))
			} else {
				for i := range nuestras {
					if nuestras[i] != verdaderas[i] {
						t.Errorf("FindAllString(%q) en %q [%d]: nuestro=%q go=%q",
							caso.patron, texto, i, nuestras[i], verdaderas[i])
					}
				}
			}

			// 4) los grupos
			ng := nuestro.FindStringSubmatch(texto)
			vg := verdadero.FindStringSubmatch(texto)
			if len(ng) != len(vg) {
				t.Errorf("FindStringSubmatch(%q) en %q: nuestro=%v (%d) go=%v (%d)",
					caso.patron, texto, ng, len(ng), vg, len(vg))
			} else {
				for i := range ng {
					if ng[i] != vg[i] {
						t.Errorf("FindStringSubmatch(%q) en %q [%d]: nuestro=%q go=%q",
							caso.patron, texto, i, ng[i], vg[i])
					}
				}
			}
		}
	}
}

func TestReplaceYSplit(t *testing.T) {
	casosReemplazo := []struct {
		patron, repl string
		textos       []string
	}{
		{`\[[a-z]+\]`, "", []string{"hola [color] mundo", "a[b]c[d]e"}},
		{`[^a-z0-9\-]+`, "-", []string{"abc DEF 123", "!!!a!!!"}},
		{`\d+`, "N", []string{"a12b345c", "sin numeros"}},
		{`skill\('([^']+)`, "SKILL($1)", []string{"skill('Fireball')", "x"}},
	}

	for _, caso := range casosReemplazo {
		nuestro := MustCompile(caso.patron)
		verdadero := goregexp.MustCompile(caso.patron)

		for _, texto := range caso.textos {
			ns := nuestro.ReplaceAllString(texto, caso.repl)
			vs := verdadero.ReplaceAllString(texto, caso.repl)
			if ns != vs {
				t.Errorf("ReplaceAllString(%q,%q) en %q: nuestro=%q go=%q",
					caso.patron, caso.repl, texto, ns, vs)
			}

			nsb := string(nuestro.ReplaceAll([]byte(texto), []byte(caso.repl)))
			vsb := string(verdadero.ReplaceAll([]byte(texto), []byte(caso.repl)))
			if nsb != vsb {
				t.Errorf("ReplaceAll(%q,%q) en %q: nuestro=%q go=%q",
					caso.patron, caso.repl, texto, nsb, vsb)
			}
		}
	}

	casosSplit := []struct {
		patron string
		textos []string
	}{
		{`\D`, []string{"a1b2c3", "123", "sin numeros"}},
		{`[^a-z0-9\-]+`, []string{"uno dos tres", "a"}},
	}

	for _, caso := range casosSplit {
		nuestro := MustCompile(caso.patron)
		verdadero := goregexp.MustCompile(caso.patron)

		for _, texto := range caso.textos {
			ns := nuestro.Split(texto, -1)
			vs := verdadero.Split(texto, -1)
			if len(ns) != len(vs) {
				t.Errorf("Split(%q) en %q: nuestro=%v go=%v", caso.patron, texto, ns, vs)
				continue
			}
			for i := range ns {
				if ns[i] != vs[i] {
					t.Errorf("Split(%q) en %q [%d]: nuestro=%q go=%q", caso.patron, texto, i, ns[i], vs[i])
				}
			}
		}
	}
}

// TestCompilePedorro: los patrones que no se entienden tienen que fallar, no colgarse.
func TestCompilePedorro(t *testing.T) {
	for _, patron := range []string{"[abc", "(abc", "a\\"} {
		if _, err := Compile(patron); err == nil {
			t.Errorf("Compile(%q) deberia fallar y no fallo", patron)
		}
	}
}

// TestLimitesConocidos: las divergencias con el regexp de Go que estan DECLARADAS (no escondidas).
// Si alguna deja de cumplirse, este test FALLA: asi la lista no queda muerta ni tapada.
//
// El plegado de mayusculas con (?i) es SOLO ASCII: Go pliega con las tablas de Unicode (el signo Kelvin
// U+212A es "k" para (?i)) y la consola no tiene esas tablas ni las va a tener (el regexp de la consola
// es un motor chico escrito a mano para los 16 patrones del motor, todos ASCII). El patron VACIO, en
// cambio, NO es un limite: se probo contra Go (MatchString, FindString y FindAllString sobre "", "a" y
// "abc") y da lo mismo, asi que va en el corpus comparado como un caso mas, no aca.
func TestLimitesConocidos(t *testing.T) {
	re, err := Compile(`(?i)k`)
	if err != nil {
		// con el defecto del prefijo puesto esto es lo primero que se cae; se informa como fallo del
		// corpus y no como panic, para que el mutador lea el aserto y no el desborde.
		t.Errorf("Compile(`(?i)k`) fallo: %v", err)
		return
	}
	if re.MatchString("\u212a") {
		t.Errorf("(?i)k matchea el signo Kelvin (U+212A): el plegado dejo de ser solo ASCII, hay que medirlo contra Go y sacar el limite")
	}
	// y el de siempre: los patrones que no se entienden no se aceptan a medias
	if _, err := Compile(`(?i)[abc`); err == nil {
		t.Errorf("Compile(`(?i)[abc`) deberia fallar (clase sin cerrar) y no fallo")
	}
}

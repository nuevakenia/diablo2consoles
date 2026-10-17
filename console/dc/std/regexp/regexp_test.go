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
}

func TestContraElRegexpDeGo(t *testing.T) {
	for _, caso := range casos {
		nuestro := MustCompile(caso.patron)
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

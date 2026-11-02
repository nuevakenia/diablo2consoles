package main

// EL MONSTRUO Y EL GOLPE (DC-002.50) -- el archivo COMPARTIDO entre el POC de la consola
// (`console/dc/poc/personaje-mapa`) y el verificador del host (`console/dc/verif-monstruo-250`).
//
// Que hace y que NO hace: aca no hay pantalla ni disco. Recibe las tablas del MPQ ya leidas y devuelve (a) de
// donde salen los archivos del bicho (COF y capas), (b) la VIDA que le pone el motor y (c) el golpe: cuanto
// pega, cuanta vida queda y si se murio. Que sea el mismo archivo en los dos lados es lo que hace comparables
// los numeros de la consola con los del host (se verifica con sha256 en la evidencia, como nivel.go y
// personaje.go).
//
// DE DONDE SALE CADA NUMERO (nada de esto es inventado):
//
//   - EL BICHO: la clase "Returned" de monstats.txt, que es un monstruo del ACTO 1 (las criptas), y su carpeta
//     del MPQ es la del campo Code de esa fila (SK). Se eligio MIDIENDO, y la medicion descarto a los otros dos
//     candidatos:
//     * el fallen del Blood Moor, el candidato natural, tiene MinHP=1 y MaxHP=3 y con el factor del monlvl de
//       su nivel (7) el camino de siembra le da `MulDiv100(7,1)=0`, o sea que nace con 1 HP: MUERE DEL PRIMER
//       GOLPE y no llega a jugar la animacion de "recibiendo";
//     * el Brute (la carcel) tiene MinHP=18 y MaxHP=29 (nivel 5, factor 15) = 2..4 HP y aguanta golpes, pero
//       su animacion de morir es una caja de 176x124 px en 18 cuadros = 393 KB de indices, y con eso el POC se
//       quedo SIN MEMORIA en la consola (medido: "fatal error: out of memory", arena=10.252.096 usados
//       10.113.680). No es un numero elegido: es el que no entra.
//     * el Returned tiene MinHP=13 y MaxHP=19 (nivel 6, factor 17) = 2..3 HP: aguanta golpes Y sus capas son
//       chicas (la mas grande es 41x47 px), asi que los cuatro modos entran comodos en la misma arena.
//   - LA VIDA: el MISMO camino con el que el motor siembra un monstruo (`d2core/d2monsters/vitals.go`,
//     computeVitals): la fila del monlvl del nivel del bicho (1..111, `niveles[nivel-1]`) por MinHP/MaxHP de
//     monstats con `d2monstats.MulDiv100` (el multiply-divide del exe, VERIFIED que trunca), tirado entre el
//     minimo y el maximo con el generador del motor (`d2rand`) y con el piso de 1 HP. El area no entra: en
//     NORMAL el nivel es el de monstats (`ResolveLevel`) y el monlvl no se consulta por area.
//   - OJO CON EL LECTOR: `d2monstats.LoadClasses` pide las columnas "minHP"/"maxHP" en minuscula y el
//     monstats.txt de 1.14b las llama "MinHP"/"MaxHP", asi que devuelve 0 para NORMAL (medido: el Brute sale
//     con minHP=0/maxHP=0 y AC=25/A1=3..4 en la MISMA fila). Aca se leen las dos cajas y se usan las de la
//     tabla real; el resto de la fila (nivel, AC, ataque) sale del lector del motor tal cual.
//   - EL GOLPE: `heroDamage` del motor sin arma (d2core/d2monsters/combat.go: "or 1..2 bare handed", VERIFIED
//     min>=1/max>=2) y el dado del motor (d2rand, un paso por golpe). El dano se resta como lo resta
//     `damage()`: HP -= dano y a 0 el bicho muere.
//   - EL CUADRO DEL GOLPE: el motor da el golpe a la MITAD de la animacion (`d2core/d2map/d2mapentity/
//     monster.go`, checkEvents: currentFrame/frameCount >= 0,5). Aca se usa la misma regla.
//   - LAS ANIMACIONES: los COF del bicho viven en `data\global\monsters\<code>\COF\<code><modo><arma>.cof` y
//     sus capas en `data\global\monsters\<code>\<capa>\<code><capa>LIT<modo><arma>.dcc` (el "LIT" es el valor
//     por omision del motor cuando no hay equipo: composite.go). El ritmo NO sale del COF: sale del
//     animdata.d2 (`<code><modo><arma>`), que es lo que usa el motor para animar.
//   - LOS MODOS: los tokens son los del motor (`d2monster.ModeNeutral` = NU, `ModeGetHit` = GH,
//     `ModeDying` = DT, `ModeDead` = DD), y los modos de un solo tiro (GH, DT) vuelven a NU / pasan a DD
//     cuando terminan, que es lo que hace checkEvents.

import (
	"errors"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2txt"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2math/d2rand"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2monstats"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2monster"
)

const (
	// baseBestia: la carpeta de los monstruos en el MPQ (composite.go, monsterBasePath).
	baseBestia = "data/global/monsters"

	// tokenBestia: el Code de la fila de monstats.txt del bicho (la carpeta del MPQ).
	tokenBestia = "SK"

	// claseBestia: el Id/Class de la fila de monstats.txt.
	claseBestia = "Returned"

	// valorCapaBestia: la variante de capa por omision del motor (composite.go: "lit" cuando no hay equipo).
	valorCapaBestia = "LIT"

	// semillaCombate: la semilla del dado del motor. Fija para que la corrida sea REPRODUCIBLE: con esta
	// semilla el Brute nace con la vida mas alta de su tabla (4) y los dados pegan flojo (1,1,1,1), asi que la
	// corrida muestra el GH tres veces y el DT en el cuarto golpe. Con otra semilla los numeros son otros.
	semillaCombate = 3

	// el ataque del personaje (amazona desarmada): el COF AMA1HTH y su ritmo del animdata (13 cuadros a 40 ms).
	modoGolpePersonaje = "A1"

	// cuantos golpes se pueden dar en una corrida (tope de la prueba, no del motor).
	maxGolpes = 12
)

// Los modos del bicho, con los tokens del motor (que son los del COF y los del animdata).
var (
	modoBestiaQuieto   = d2monster.ModeNeutral.String() // NU
	modoBestiaGolpeada = d2monster.ModeGetHit.String()  // GH
	modoBestiaMuriendo = d2monster.ModeDying.String()   // DT
	modoBestiaMuerta   = d2monster.ModeDead.String()    // DD
)

// RutaCOFBestia: el COF de un modo del bicho, como lo arma el motor (composite.go, createMode):
// <base>/<token>/COF/<token><modo><arma>.COF
func RutaCOFBestia(modo, arma string) string {
	return `data\global\monsters\` + tokenBestia + `\COF\` +
		lower(tokenBestia) + lower(modo) + lower(arma) + `.cof`
}

// RutaDeLaCapaBestia: el grafico de una capa del bicho (el mismo esquema que RutaDeLaCapaModo de
// personaje.go, pero con la carpeta de los monstruos y el valor de capa por omision).
func (p *Personaje) RutaDeLaCapaBestia(indice int, modo, ext string) string {
	return baseBestia + "/" + tokenBestia + "/" + p.Claves[indice] + "/" +
		tokenBestia + p.Claves[indice] + valorCapaBestia + modo + p.Armas[indice] + "." + ext
}

// Vida es lo que el motor le pone al bicho al sembrarlo, mas lo que va a necesitar el que dibuja.
type Vida struct {
	Clase        string
	Nivel        int
	Min, Max, HP int // HP es la vida CONCRETA (el tirado)
	AC, TH       int
	DanoMin      int // el ataque A1 del bicho, en el dano que le da la tabla (para informar la respuesta)
	DanoMax      int
}

// VidaDelBicho: la vida del bicho por el camino de siembra del motor (ver el comentario de arriba).
//
// monstats y monlvl son los bytes de las tablas (monstats.txt sale de d2data.mpq y MonLvl.txt del
// patch_D2.mpq: medido, en d2data y en d2exp NO esta). El error es de tabla, no del bicho.
func VidaDelBicho(monstats, monlvl []byte, clase string, semilla uint32) (*Vida, error) {
	clases, err := d2monstats.LoadClasses(monstats)
	if err != nil {
		return nil, err
	}

	c := clases[clase]
	if c == nil {
		return nil, errors.New("monstruo: no esta la clase " + clase + " en monstats.txt")
	}

	minHP, maxHP, ok := columnasDeVida(monstats, clase)
	if !ok {
		return nil, errors.New("monstruo: la fila de " + clase + " no tiene MinHP/MaxHP legibles")
	}

	niveles, err := d2monstats.LoadMonLvl(monlvl)
	if err != nil {
		return nil, err
	}

	// el nivel del bicho: en NORMAL es el de monstats (d2monstats.ResolveLevel no consulta el area).
	nivel := c.Level[d2monstats.Normal]
	if nivel < 1 {
		nivel = 1
	}

	// las filas del monlvl son 1..N y el slice queda 0..N-1
	renglon := nivel - 1
	if renglon >= len(niveles) {
		renglon = len(niveles) - 1
	}

	factor := niveles[renglon].HP[1][d2monstats.Normal] // columnas de la expansion (L-HP), dificultad normal

	v := &Vida{
		Clase: clase, Nivel: nivel,
		Min:     d2monstats.MulDiv100(factor, minHP),
		Max:     d2monstats.MulDiv100(factor, maxHP),
		AC:      c.AC[d2monstats.Normal],
		TH:      c.Attacks[d2monstats.A1][d2monstats.Normal].TH,
		DanoMin: c.Attacks[d2monstats.A1][d2monstats.Normal].Min,
		DanoMax: c.Attacks[d2monstats.A1][d2monstats.Normal].Max,
	}

	if v.Max < v.Min {
		v.Max = v.Min
	}

	dado := d2rand.New(semilla)

	v.HP = v.Min
	if v.Max > v.Min {
		v.HP += int(dado.Roll(int32(v.Max - v.Min + 1)))
	}

	if v.HP < 1 { // el piso de computeVitals
		v.HP = 1
	}

	return v, nil
}

// Combate: el bicho que recibe golpes y el dado del heroe.
type Combate struct {
	Vida   *Vida
	dado   *d2rand.Seed
	Golpes int
	Dano   int // dano acumulado
	Muerto bool
	Ultimo int // el dano del ultimo golpe (para informar)
}

// NuevoCombate: el dado del golpe. La semilla del heroe va con el MISMO XOR que usa `heroRoller` del motor
// (d2core/d2monsters/combat.go), asi que la secuencia de danos es la del motor y no una elegida.
func NuevoCombate(v *Vida, semilla uint32) *Combate {
	return &Combate{Vida: v, dado: d2rand.New(semilla ^ 0x6865726f)}
}

// GolpeDelHeroe: un golpe a mano limpia, con las cuentas del motor. Devuelve el dano y la vida antes y despues.
//
//	heroDamage: min, max = 1, 2 (sin arma) y min>=1/max>=2 (VERIFIED)
//	el dado:    min + Roll(max-min+1)
//	el dano:    HP -= dano; si queda <= 0, HP = 0 y el bicho muere (damage() del motor)
func (c *Combate) GolpeDelHeroe() (dano, antes, despues int, muerto bool) {
	if c.Muerto || c.Golpes >= maxGolpes {
		return 0, c.Vida.HP, c.Vida.HP, c.Muerto
	}

	minimo, maximo := 1, 2
	dano = minimo + int(c.dado.Roll(int32(maximo-minimo+1)))

	antes = c.Vida.HP
	c.Vida.HP -= dano
	if c.Vida.HP <= 0 {
		c.Vida.HP = 0
		c.Muerto = true
	}

	c.Golpes++
	c.Dano += dano
	c.Ultimo = dano
	despues = c.Vida.HP

	return dano, antes, despues, c.Muerto
}

// CuadroDelGolpe: el cuadro de la animacion en el que cae el golpe. Es la regla del motor (checkEvents:
// currentFrame/frameCount >= 0,5), o sea la mitad del ciclo redondeada para arriba.
func CuadroDelGolpe(cuadros int) int {
	return (cuadros + 1) / 2
}

// columnasDeVida: MinHP y MaxHP de la fila, con la caja que use la tabla. Se devuelve false si las dos cajas
// dan 0 (una fila sin vida: no es un bicho normal).
func columnasDeVida(monstats []byte, clase string) (int, int, bool) {
	d := d2txt.LoadDataDictionary(monstats)

	for d.Next() {
		id := d.String("Id")
		if id == "" {
			id = d.String("Class")
		}

		if !equalFold(id, clase) {
			continue
		}

		minimo, maximo := d.Number("MinHP"), d.Number("MaxHP")
		if minimo == 0 && maximo == 0 {
			minimo, maximo = d.Number("minHP"), d.Number("maxHP")
		}

		if minimo == 0 && maximo == 0 {
			return 0, 0, false
		}

		return minimo, maximo, true
	}

	return 0, 0, false
}

// lower: minusculas ASCII (los nombres del MPQ van en minuscula y el lector busca sin caja, pero el nombre se
// arma igual que el motor para que el numero de bytes del archivo se pueda auditar).
func lower(s string) string {
	b := []byte(s)
	for i := range b {
		if b[i] >= 'A' && b[i] <= 'Z' {
			b[i] += 'a' - 'A'
		}
	}

	return string(b)
}

// equalFold: comparacion sin caja (para el Id de las tablas, que en 1.14b viene con mayuscula).
func equalFold(a, b string) bool {
	if len(a) != len(b) {
		return false
	}

	for i := 0; i < len(a); i++ {
		x, y := a[i], b[i]
		if x >= 'A' && x <= 'Z' {
			x += 'a' - 'A'
		}

		if y >= 'A' && y <= 'Z' {
			y += 'a' - 'A'
		}

		if x != y {
			return false
		}
	}

	return true
}

// numero: el Atoi que no hace falta exportar (para dejar el archivo sin dependencias raras).

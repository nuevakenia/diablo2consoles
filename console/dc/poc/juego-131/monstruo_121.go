//go:build dreamcast

// DC-002.121 -- EL PRIMER MONSTRUO: DIBUJADO Y PATRULLANDO EN LA CONSOLA.
//
// QUE HACE ESTE ARCHIVO. El mundo (DC-002.111/.120) y la gente (DC-002.113...118) ya estaban; lo que faltaba
// para que esto sea un JUEGO es lo que se mueve solo. Aca se agrega UN monstruo del Acto 1 -- el Returned de
// las criptas, token SK -- con su `.cof`, sus capas `.dcc` del `d2data.mpq` y su animacion de caminar, en su
// celda del mapa, patrullando entre dos puntos. Y nada mas: sin IA, sin vida, sin combate (eso es DC-002.122).
//
// DE DONDE SALE CADA NUMERO (nada elegido a ojo; los numeros salen de `sonda-monstruo-250` corrida en el host):
//
//   - EL BICHO. El Returned (token SK) es el monstruo que DC-002.50 ya habia elegido MIDIENDO para la consola
//     (ver `console/dc/poc/personaje-mapa/monstruo.go`): sus capas son chicas (la mas grande es 18x44 px) y
//     entra en la arena de la consola, que es el limite real de este port. Los COF de sus modos viven en
//     `data\global\monsters\SK\COF\sk<modo>hth.cof` (arma HTH: desarmado) y las capas en
//     `data\global\monsters\SK\<capa>\SK<capa>LIT<modo>hth.dcc` -- el esquema de `monsterBasePath` del motor
//     (`d2core/d2asset/composite.go`, `baseString(ObjectTypeCharacter)`) con el valor de capa por omision
//     ("lit", porque el bicho no lleva equipo).
//
//   - LA PALETA. `Units/pal.dat`, que es la que el MOTOR le pone a los monstruos (`d2core/d2monsters/prefetch.go`:
//     `Palette: d2resource.PaletteUnits`). No hay que leer otra: es la misma que ya esta en `palJugador`.
//
//   - LA ANIMACION. El ritmo NO sale del campo Speed del COF (el motor no lo usa para los monstruos): sale del
//     `animdata.d2`. Medido con la sonda: SKWLHTH = 8 cuadros a 12,50 fps = 80,00 ms por cuadro (640 ms el ciclo).
//     Y el numero de DIRECCIONES del COF del bicho son 8 (medido: `dirs=8`), no las 16 del heroe.
//
//   - LA VELOCIDAD. `Velocity=4` de la fila del SK en monstats.txt (medido con la sonda, columna Velocity de la
//     FILA cruda), por la conversion del motor (`d2core/d2map/d2mapentity/factory.go`:
//     `velocityToSubtilesPerSecond = retailFps / 16.0` = 25/16): 4 * 25/16 = 6,25 subtiles/s = 1,25 celdas/s.
//     Es la MITAD de la velocidad del heroe caminando (9,375 subtiles/s), que es lo que se ve en la consola.
//
//   - LA DIRECCION QUE MIRA. No hay tabla propia: el vector del paso (en celdas) pasa por la misma cadena del
//     motor que usa el jugador (`d2vector.DirectionTo` -> `d2cof.Dir64ToCof`), con las 8 direcciones del COF del
//     bicho. Por eso la patrulla compone DOS ciclos: el de ida y el de vuelta.
//
//   - LA CELDA. El `.ds1` del pueblo NO trae monstruos: solo trae 41 Objects (medido en todas las corridas de
//     este POC: `DS1 ... objetos= 41`), porque en Diablo II los bichos se siembran por la tabla de niveles y no
//     por el mapa. Asi que la celda es la del bicho de DC-002.50 -- la celda del POC corrida un lugar en cada eje
//     -- y los DOS EXTREMOS de la patrulla se derivan de ella. Es lo que la tarjeta pide: "no generacion
//     aleatoria de la zona: se usa la posicion del .ds1", y el .ds1 no tiene ninguna.
//
//   - LOS BYTES. Los cuadros compuestos son del TIPO de bicho, no del bicho: N monstruos iguales comparten los
//     mismos indices (igual que los jugadores del cooperativo comparten los ciclos desde DC-002.114). Lo que
//     cuesta por bicho es su estado y su caja guardada, y las dos cosas se imprimen medidas (`unsafe.Sizeof`),
//     no estimadas. Eso es la respuesta a "los .dcc de monstruo se comparten?".
package main

import (
	"hash/crc32"
	"math"
	"unsafe"

	"github.com/OpenDiablo2/OpenDiablo2/console/dc/fuenteconsola"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2cof"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2dc6"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2dcc"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2math/d2vector"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2render/pvr"
)

const (
	// baseBicho: la carpeta de los monstruos en el MPQ (composite.go, monsterBasePath).
	baseBicho = "data/global/monsters"

	// tokenBicho: el Code de la fila de monstats.txt (la carpeta del MPQ).
	tokenBicho = "SK"

	// claseBicho: el Id/Class de la fila de monstats.txt (el Returned, el de las criptas del Acto 1).
	claseBicho = "Returned"

	// valorCapaBicho: la variante de capa por omision del motor (composite.go: "lit" cuando no hay equipo).
	valorCapaBicho = "LIT"

	// Los dos modos que se componen: CAMINAR (patrulla) y QUIETO (NU, para el cuadro en que el bicho llega al
	// extremo y se da vuelta ese mismo cuadro -- asi nunca se dibuja con un ciclo que no existe).
	modoBichoCamina = "WL"
	modoBichoQuieto = "NU"

	// DC-002.122: los tres modos del golpe (GH recibiendo, DT muriendo, DD muerto) NO se declaran aca: vienen del
	// archivo COMPARTIDO con el verificador del host (`monstruo.go`, con los tokens de `d2monster.Mode`).

	// msBichoCamina: 12,50 fps del animdata (SKWLHTH), medido con la sonda. 80,00 ms por cuadro, 8 cuadros.
	msBichoCamina = 80

	// msBichoQuieto: SKNUHTH, tambien 12,50 fps medido. 80,00 ms por cuadro, 8 cuadros.
	msBichoQuieto = 80

	// velocidadBicho: subtiles por segundo. Velocity=4 de monstats.txt por 25/16 (factory.go). Medido: 1,25
	// celdas/s, la mitad de lo que camina el heroe.
	velocidadBicho = 4.0 * 25.0 / 16.0

	// maxBichosBicho: el tope de la corrida (la tabla de 0/1/N de la tarjeta llega hasta 4).
	maxBichosBicho = 4
)

// bicho: UN monstruo en el mapa. Lo que es POR BICHO es su posicion, su reloj de animacion y su caja; los
// CUADROS COMPUESTOS no (son del tipo y se comparten entre todos los bichos iguales).
type bicho struct {
	Nombre string

	// Est: la posicion del motor, en subtiles de 5 por celda (el mismo `Estado` del jugador, con la velocidad
	// del bicho). Reusar el tipo es a proposito: la proyeccion y las cuentas son las mismas.
	Est *Estado

	// la patrulla: los dos extremos, en CELDAS del mapa, y hacia cual va.
	A, B   [2]float64
	HaciaB bool

	// Direccion: la direccion del COF (0..7) que mira; MsAnim: el reloj de la animacion.
	Direccion int
	MsAnim    float64

	// lo que se mide
	Tiempo      float64 // segundos de reloj patrullando (la base de las celdas por segundo)
	Celdas      float64 // celdas caminadas
	Extremos    int     // cuantas veces llego a un extremo y se dio vuelta
	VueltasOK   int     // vueltas completas (A -> B -> A)
	Bloqueos    int     // pasos que no pudo dar porque la celda la ocupaba un jugador
	ArranqueX   float64
	ArranqueY   float64
	CuadroCiclo int

	// el dibujo del cuadro que se esta pintando
	AnclaX, AnclaY int
	Caja           CajaDelJugador
	CajaCruda      CajaDelJugador
	CajaGuardada   []byte
	Pintados       int
	MsDibujo       int
	Ancho, Alto    int

	// ---- DC-002.122: LA VIDA Y EL COMBATE ----
	//
	// Vida: la que le pone el motor al sembrarlo (`monstruo.go: VidaDelBicho`, el mismo archivo del verificador del
	// host). Combate: el bicho que recibe golpes y el dado del heroe (`monstruo.go: Combate`).
	Vida    *Vida
	Combate *Combate

	// Modo: el modo que se esta dibujando. "" o WL = patrulla; GH = recibiendo; DT = muriendo; DD = muerto.
	Modo         string
	MsModo       int64            // cuando empezo el modo (el reloj de la animacion de GH/DT/DD)
	CicloDelModo *CicloDelJugador // el ciclo de ese modo, en la direccion que mira

	// Muerte (DC-002.126): el reloj y las cuentas de la muerte -- que cuadros del DT se dibujaron, cuando empezo el
	// DD y si el cadaver ya expiro. Nil mientras el bicho esta vivo (no cuesta nada).
	Muerte *muerteDelBicho
}

var (
	// bichos: los monstruos de la corrida (0 = la corrida de DC-002.120, sin bicho).
	bichos []*bicho

	// ciclosBicho: los ciclos compuestos del TIPO de bicho, por direccion del COF. Compartidos por todos.
	ciclosBicho map[int]*CicloDelJugador

	// cicloBichoQuieto: el NU (dir 0), para el cuadro del giro en el extremo.
	cicloBichoQuieto *CicloDelJugador

	// ciclosModoBicho (DC-002.122): los modos del GOLPE (GH recibiendo, DT muriendo, DD muerto) compuestos, por
	// modo y por direccion del COF. Son del TIPO, como los de caminar: N bichos iguales los comparten.
	ciclosModoBicho map[string]map[int]*CicloDelJugador

	// bytesBichoCompartidos: los indices de los ciclos compuestos (lo que cuesta el TIPO, no el bicho).
	bytesBichoCompartidos int

	// tamanoDeBicho: la estructura de un bicho (medida, no escrita a mano).
	tamanoDeBicho = int(unsafe.Sizeof(bicho{}))

	// solapamientos: cuadros en los que un jugador y un bicho quedaron en la MISMA celda (tiene que ser 0).
	solapamientos int

	// bloqueosDeJugadores / bloqueosDeBichos: los que se frenaron por la celda del otro.
	bloqueosDeJugadores int
)

// ============================== LA CARGA (DEL MPQ A LOS CICLOS) ==============================

// RutaCOFDelBicho: el COF de un modo del bicho, como lo arma el motor (composite.go, createMode):
// data\global\monsters\<token>\COF\<token><modo><arma>.cof
func RutaCOFDelBicho(modo, arma string) string {
	return `data\global\monsters\` + tokenBicho + `\COF\` +
		minusculas(tokenBicho) + minusculas(modo) + minusculas(arma) + `.cof`
}

// minusculas: minusculas ASCII. El nombre del archivo se arma con el mismo criterio que el motor para que el
// numero de bytes se pueda auditar contra el MPQ (que busca sin caja).
func minusculas(s string) string {
	b := []byte(s)

	for i := range b {
		if b[i] >= 'A' && b[i] <= 'Z' {
			b[i] += 'a' - 'A'
		}
	}

	return string(b)
}

// RutaDeLaCapaBicho: el grafico de una capa del bicho (el mismo esquema que RutaDeLaCapaModo de personaje.go,
// pero con la carpeta de los monstruos y el valor de capa por omision).
func RutaDeLaCapaBicho(p *Personaje, indice int, modo, ext string) string {
	return baseBicho + "/" + tokenBicho + "/" + p.Claves[indice] + "/" +
		tokenBicho + p.Claves[indice] + valorCapaBicho + modo + p.Armas[indice] + "." + ext
}

// direccionDelBicho: el vector del paso (en celdas, unitario) -> la direccion del COF del bicho, con la MISMA
// cadena del motor que usa el jugador (`DireccionDelControl` de movimiento.go): DirectionTo (64) -> Dir64ToCof.
func direccionDelBicho(dx, dy float64, direcciones int) int {
	origen := d2vector.NewVector(0, 0)
	destino := d2vector.NewVector(dx, dy)

	return d2cof.Dir64ToCof(origen.DirectionTo(*destino), direcciones)
}

// celdaDelBicho: la celda donde nace el bicho `i` -- la del bicho de DC-002.50 (la celda del POC corrida uno en
// cada eje). Cada bicho va en su carril, corrido sobre la PERPENDICULAR del eje del paseo, para que N bichos no
// caminen uno arriba del otro. El `.ds1` del pueblo no trae monstruos: esto es lo que hay.
func celdaDelBicho(i int) (float64, float64) {
	d := float64(i) * separacionEntreBichos

	return celdaBichoX + d, celdaBichoY + d
}

// cargarBichos: compone los ciclos del TIPO (NU dir 0, WL ida y vuelta) y arma los `monstruosKnob` bichos que
// patrullan. Con 0 no hace nada y la corrida es la de DC-002.120 bit a bit.
//
// VA ANTES DE SOLTAR LAS FUENTES (main, paso 5): los `.cof` y los `.dcc` del bicho salen del `d2data.mpq`, y
// despues de `soltarLoQueElCuadroNoLee` esa fuente ya no esta.
func cargarBichos() {
	if monstruosKnob <= 0 {
		println("JUEGO121: BICHO apagado (monstruos= 0): la corrida es la de DC-002.120")

		return
	}

	datos := fuenteDe(rutaDatos)
	if datos == nil {
		println("JUEGO121: BICHO sin fuente de datos: no puedo leer el monstruo")

		return
	}

	t0 := int64(d2cMs())

	ciclosBicho = make(map[int]*CicloDelJugador, 3)

	// LA IDA Y LA VUELTA: las dos direcciones del eje de la patrulla, con la cadena del motor. Si el eje es
	// (1,0) son la 0 y la 4 de las 8 del COF (medido en el log de la corrida).
	ida := direccionDelBicho(patrullaDx, patrullaDy, direccionesCOFBicho)
	vuelta := direccionDelBicho(-patrullaDx, -patrullaDy, direccionesCOFBicho)

	println("JUEGO121: BICHO DIRECCIONES del eje de la patrulla ida=", ida, "vuelta=", vuelta,
		"direccionesDelCOF=", direccionesCOFBicho)

	for _, d := range []int{ida, vuelta} {
		ciclo := componerCicloDelBicho(datos, modoBichoCamina, d, msBichoCamina)
		if ciclo == nil || len(ciclo.Cuadros) == 0 {
			println("JUEGO121: BICHO el ciclo de caminar salio vacio dir=", d)

			continue
		}

		ciclosBicho[d] = ciclo
		bytesBichoCompartidos += bytesDeLosCuadros(ciclo)

		// DC-002.131: el arte del bicho tambien va al monton de C (ver arte_131.go). Va aca, donde el ciclo se
		// acaba de componer y nadie mas lo tiene, para que el `Lienzo` del semiespacio quede basura en el acto.
		if movidos := arteDelCicloAlMontonDeC(ciclo); movidos > 0 {
			println("JUEGO131: BICHO ARTE-AL-MONTON-DE-C dir=", d, "cuadros=", len(ciclo.Cuadros),
				"bytes=", movidos, "crc32PrimerCuadro=", int(ciclo.Cuadros[0].CRC),
				"totalMovido=", bytesDeArteEnElMontonDeC, "montonCLibre=", int(d2cHeapLibre()))
		}
	}

	cicloBichoQuieto = componerCicloDelBicho(datos, modoBichoQuieto, 0, msBichoQuieto)
	if cicloBichoQuieto != nil {
		bytesBichoCompartidos += bytesDeLosCuadros(cicloBichoQuieto)

		if movidos := arteDelCicloAlMontonDeC(cicloBichoQuieto); movidos > 0 {
			println("JUEGO131: BICHO ARTE-AL-MONTON-DE-C quieto=true cuadros=", len(cicloBichoQuieto.Cuadros),
				"bytes=", movidos, "crc32PrimerCuadro=", int(cicloBichoQuieto.Cuadros[0].CRC),
				"totalMovido=", bytesDeArteEnElMontonDeC, "montonCLibre=", int(d2cHeapLibre()))
		}
	}

	println("JUEGO121: BICHO CICLOS caminar=", len(ciclosBicho), "quieto=", cicloBichoQuieto != nil,
		"indicesCompartidosBytes=", bytesBichoCompartidos, "ms=", int64(d2cMs())-t0)

	// LOS BICHOS: cada uno en su carril, patrullando en +eje y volviendo.
	cuantos := monstruosKnob
	if cuantos > maxBichosBicho {
		println("JUEGO121: BICHO AVISO monstruos=", monstruosKnob, "es mas que el tope de la corrida:",
			maxBichosBicho)

		cuantos = maxBichosBicho
	}

	bichos = make([]*bicho, 0, cuantos)

	for i := 0; i < cuantos; i++ {
		x, y := celdaDelBicho(i)

		b := &bicho{
			Nombre:    "B" + itoa(i+1),
			Est:       NuevoEstado(x, y, false),
			A:         [2]float64{x, y},
			B:         [2]float64{x + patrullaLargo*patrullaDx, y + patrullaLargo*patrullaDy},
			HaciaB:    true,
			Direccion: ida,
			ArranqueX: x,
			ArranqueY: y,
		}

		// la velocidad del bicho NO es la del heroe: es la de su fila de monstats (Velocity=4 -> 6,25 subtiles/s)
		b.Est.Velocidad = velocidadBicho

		bichos = append(bichos, b)

		println("JUEGO121: BICHO ARRANCA nombre=", b.Nombre, "celda=", x, y, "A=", b.A[0], b.A[1],
			"B=", b.B[0], b.B[1], "velocidadSubtilesPorSegundo=", b.Est.Velocidad,
			"celdasPorSegundo=", b.Est.Velocidad/float64(subtilesPorCelda), "dir=", b.Direccion)
	}

	resumenDeLaMemoriaDelBicho()

	// DC-002.122: la vida y los modos del golpe. Va DESPUES de armar los bichos (cada uno nace con su vida y su
	// dado) y ANTES de soltar el d2data (los COF de GH/DT/DD salen de ahi, igual que los de caminar).
	if combateKnob == "si" {
		cargarCombateDelBicho(datos, ida, vuelta)
	} else {
		println("JUEGO122: COMBATE apagado (combate=", combateKnob, "): el bicho patrulla y no pelea")
	}
}

// componerCicloDelBicho: compone los cuadros de UNA direccion de UN modo del bicho por el camino del motor
// (COF -> ruta -> .dcc -> decodificador -> compositor compartido). Es el `ComponerCicloDelJugador` de jugador.go
// con la unica diferencia que lo hace distinto: la RUTA de la capa es la de los monstruos.
func componerCicloDelBicho(datos *fuenteconsola.Fuente, modo string, dir, msPorCuadro int) *CicloDelJugador {
	tramo, err := datos.Pedir(RutaCOFDelBicho(modo, "HTH"))
	if err != nil || len(tramo.Datos) == 0 {
		println("JUEGO121: BICHO SIN COF modo=", modo, "ruta=", RutaCOFDelBicho(modo, "HTH"))

		return nil
	}

	bytesCOF, crcCOF := len(tramo.Datos), crc32.ChecksumIEEE(tramo.Datos)

	cof, err := d2cof.Unmarshal(tramo.Datos)
	tramo.Liberar()

	if err != nil {
		println("JUEGO121: BICHO COF-PARSE FALLO modo=", modo, err.Error())

		return nil
	}

	println("JUEGO121: BICHO COF", RutaCOFDelBicho(modo, "HTH"), "bytes=", bytesCOF, "crc32=", int(crcCOF),
		"capas=", cof.NumberOfLayers, "framesPorDir=", cof.FramesPerDirection, "dirs=", cof.NumberOfDirections,
		"modo=", modo, "dir=", dir)

	if dir < 0 || dir >= cof.NumberOfDirections {
		println("JUEGO121: BICHO direccion fuera del COF:", dir, "de", cof.NumberOfDirections)
		cof = nil

		return nil
	}

	arena := &arenaDCC{}
	conArena(arena)

	ciclo := componerCicloConRuta(cof, dir, modo, msPorCuadro, lectorDeLaFuente(datos), RutaDeLaCapaBicho)

	// LA ARENA SE LIBERA ACA: los PixelData vivian adentro y recien ahora estan pegados en cada compuesto.
	arena.liberar()
	d2dcc.PedirDeArena = nil

	println("JUEGO121: BICHO ARENA modo=", modo, "dir=", dir, "pedidos=", arena.pedidos,
		"entregado=", arena.entregado, "tramos=", arena.liberados, "montonCLibre=", int(d2cHeapLibre()))

	if ciclo == nil {
		cof = nil

		return nil
	}

	ciclo.Dir = dir

	for f := range ciclo.Cuadros {
		cu := &ciclo.Cuadros[f]
		println("JUEGO121: BICHO CUADRO modo=", modo, "dir=", dir, "cuadro=", f, "de", len(ciclo.Cuadros),
			"caja=", cu.Ancho, "x", cu.Alto, "esquina=", cu.Izq, cu.Arriba, "pixeles=", cu.PixelesNoVacios,
			"crc32=", int(cu.CRC), "capasDibujadas=", cu.CapasDibujadas, "sinGrafico=", cu.CapasSinGrafico)
	}

	cof = nil

	return ciclo
}

// componerCicloConRuta: el `ComponerCicloDelJugador` de jugador.go con la RUTA DE LA CAPA como parametro. Es una
// copia y no una llamada porque jugador.go es un archivo COMPARTIDO con el medidor del host (el build lo verifica
// con `cmp`): la ruta de los monstruos no puede entrar ahi sin cambiar los dos lados, y esta tarjeta no toca el
// camino del heroe.
func componerCicloConRuta(cof *d2cof.COF, dir int, modo string, msPorCuadro int, leer Lector,
	ruta func(*Personaje, int, string, string) string) *CicloDelJugador {
	n := cof.FramesPerDirection
	per0 := NuevoPersonaje(cof, dir, 0)

	tipos := make([]capaDelCiclo, 0, len(cof.CofLayers))

	for i := range cof.CofLayers {
		encontrada := false

		for _, ext := range []string{"dcc", "dc6"} {
			nombre := ruta(per0, i, modo, ext)

			datos, soltar, err := leer(nombre)
			if err != nil {
				continue
			}

			c := capaDelCiclo{clave: per0.Claves[i], tipo: per0.Tipos[i]}

			if ext == "dcc" {
				dd, err := d2dcc.LoadDirection(datos, dir)
				soltar()

				if err != nil {
					println("JUEGO121: BICHO CAPA", c.clave, "DEL MODO", modo, "PARSE-FALLO:", err.Error())

					break
				}

				caja := dd.Directions[dir].Box
				c.izq, c.arriba, c.ancho, c.alto = caja.Left, caja.Top, caja.Width, caja.Height

				for f := 0; f < n && f < len(dd.Directions[dir].Frames); f++ {
					c.cuadros = append(c.cuadros, dd.Directions[dir].Frames[f].PixelData)
				}

				tipos = append(tipos, c)
				encontrada = true

				dd = nil

				break
			}

			dc, err := d2dc6.Load(datos)
			if err != nil {
				soltar()
				println("JUEGO121: BICHO CAPA", c.clave, "DEL MODO", modo, "PARSE-FALLO:", err.Error())

				break
			}

			for f := 0; f < n && f < len(dc.Frames); f++ {
				fr := dc.Frames[f]
				if f == 0 {
					c.izq, c.arriba = int(fr.OffsetX), int(fr.OffsetY)
					c.ancho, c.alto = int(fr.Width), int(fr.Height)
				}

				c.cuadros = append(c.cuadros, dc.DecodeFrame(f))
			}

			soltar()

			tipos = append(tipos, c)
			encontrada = true

			dc = nil

			break
		}

		if !encontrada {
			println("JUEGO121: BICHO CAPA", per0.Claves[i], "SIN-GRAFICO modo=", modo, "dir=", dir,
				"ruta=", ruta(per0, i, modo, "dcc"))
		}
	}

	ciclo := &CicloDelJugador{Modo: modo, Dir: dir, MsPorCuadro: msPorCuadro}

	for f := 0; f < n; f++ {
		per := NuevoPersonaje(cof, dir, f)

		for i := range tipos {
			if f >= len(tipos[i].cuadros) {
				continue
			}

			per.Agregar(tipos[i].tipo, tipos[i].clave, tipos[i].izq, tipos[i].arriba,
				tipos[i].ancho, tipos[i].alto, tipos[i].cuadros[f])
		}

		per.Componer()

		ciclo.Cuadros = append(ciclo.Cuadros, CuadroDelJugador{
			Ancho: per.Ancho, Alto: per.Alto, Izq: per.Izq, Arriba: per.Arriba,
			Pixeles: per.Lienzo, PixelesNoVacios: per.PixelesNoVacios,
			CapasDibujadas: per.CapasDibujadas, CapasSinGrafico: per.CapasSinGrafico,
			CapasFueraDeCaja: per.CapasFueraDeCaja, CRC: per.CRC32(),
		})

		per = nil
	}

	return ciclo
}

// resumenDeLaMemoriaDelBicho: LA RESPUESTA a "los .dcc de monstruo se comparten?". Los cuadros compuestos son
// del TIPO (se piden una vez, mida lo que mida el numero de bichos); lo que se multiplica por bicho es su estado
// (`unsafe.Sizeof`) mas la copia de la caja que se guarda para devolver el mapa en el cuadro (que depende del
// tamano del compuesto, y por eso se imprime aparte, medido en el primer cuadro).
func resumenDeLaMemoriaDelBicho() {
	println("JUEGO121: BICHO MEMORIA bichos=", len(bichos), "indicesCompartidosBytes=", bytesBichoCompartidos,
		"estadoPorBichoBytes=", tamanoDeBicho, "cajasPorBichoBytes=", bytesDeLasCajas(),
		"totalPorBichoBytes=", tamanoDeBicho+bytesDeLasCajas(),
		"totalBytes=", bytesBichoCompartidos+len(bichos)*(tamanoDeBicho+bytesDeLasCajas()),
		"noCompartidoSiCadaUnoComponia=", bytesBichoCompartidos*len(bichos))
}

// bytesDeLasCajas: lo que ocupa la copia del mapa debajo de la caja de cada bicho (medido del primer cuadro
// compuesto: la caja del compuesto es la misma en todos los cuadros de la direccion).
func bytesDeLasCajas() int {
	for _, c := range ciclosBicho {
		if len(c.Cuadros) > 0 {
			return c.Cuadros[0].Ancho * c.Cuadros[0].Alto * 4
		}
	}

	return 0
}

// ============================== EL PASO (LA PATRULLA) ==============================

// pasoDeLosBichos: un cuadro de TODOS los bichos. Cada uno va hacia su extremo con la velocidad de su fila de
// monstats; al llegar se da vuelta y cambia de ciclo (ida <-> vuelta). No hay IA: es un paseo de dos puntos.
func pasoDeLosBichos(segundos float64) {
	for _, b := range bichos {
		pasoDeUnBicho(b, segundos)
	}
}

func pasoDeUnBicho(b *bicho, segundos float64) {
	if b.Est == nil || segundos <= 0 {
		return
	}

	// DC-002.122: el bicho que esta recibiendo (GH), muriendo (DT) o muerto (DD) NO patrulla. El reloj del modo
	// lo lleva `pasoDelModoDelBicho`, y el que decide cuando vuelve a caminar es la regla del motor (GH termina y
	// vuelve; DT termina y pasa a DD, que ya no se mueve).
	if b.Modo != "" && b.Modo != modoBichoCamina {
		return
	}

	objetivo := b.B
	if !b.HaciaB {
		objetivo = b.A
	}

	x, y := b.Est.Celda()
	dx, dy := objetivo[0]-x, objetivo[1]-y

	dist := math.Sqrt(dx*dx + dy*dy)
	if dist < 1e-9 { // ya esta en el extremo: se da vuelta
		b.HaciaB = !b.HaciaB

		return
	}

	dx, dy = dx/dist, dy/dist

	// LA DIRECCION SE CALCULA ANTES DE MOVER, CON EL VECTOR HACIA DONDE VA: es la que se dibuja este cuadro.
	b.Direccion = direccionDelBicho(dx, dy, direccionesCOFBicho)
	b.CuadroCiclo = 0

	cobrados := velocidadBicho * segundos / float64(subtilesPorCelda) // en celdas
	if cobrados > dist {
		cobrados = dist
	}

	// LA CELDA DEL JUGADOR BLOQUEA AL BICHO (la colision es de celda y en los dos sentidos: "no se solapan"). Se
	// mira el CAMINO entero y no solo la celda de llegada: un paso de este POC (1,2 s) son 1,5 celdas y sin eso
	// el bicho pasaria por arriba de un jugador parado.
	libre, bloqueado := primerBloqueo(x, y, x+dx*cobrados, y+dy*cobrados, jugadorEnCelda)
	if bloqueado {
		b.Bloqueos++

		if libre <= 0 {
			return
		}

		cobrados *= libre
	}

	b.Est.Pos.Set((x+dx*cobrados)*subtilesPorCelda, (y+dy*cobrados)*subtilesPorCelda)

	if !bloqueado && cobrados >= dist { // llego al extremo: se da vuelta
		b.HaciaB = !b.HaciaB
		b.Extremos++

		if b.HaciaB { // acaba de volver a A: una vuelta completa
			b.VueltasOK++
		}
	}

	b.Celdas += cobrados
	b.Tiempo += segundos
	b.MsAnim += segundos * 1000.0
}

// CeldaEntera: la celda del mapa (entera) donde esta parado el bicho.
func (b *bicho) CeldaEntera() (int, int) {
	if b.Est == nil {
		return 0, 0
	}

	return b.Est.CeldaEntera()
}

// jugadorEnCelda: si algun jugador esta en esa celda (la colision de celda del bicho contra la gente).
func jugadorEnCelda(cx, cy int) bool {
	for _, p := range jugadores {
		if p.Mov == nil || p.Mov.Est == nil {
			continue
		}

		px, py := p.Mov.Est.CeldaEntera()
		if px == cx && py == cy {
			return true
		}
	}

	return false
}

// bichoEnCelda: si algun bicho ocupa esa celda (lo usa el paso del jugador para no pisarlo).
//
// DC-002.122: el CADAVER no ocupa celda. En el motor el cuerpo muerto queda con el flag 0x8000 (`combat.go`,
// MonsterEventDied: `d2path.FlagCorpse`, que no esta en ninguna mascara de bloqueo), asi que el heroe puede pasar
// por arriba. Se respeta: un bicho en DD no frena a nadie.
func bichoEnCelda(cx, cy int) bool {
	for _, b := range bichos {
		if b.Modo == modoBestiaMuerta {
			continue
		}

		bx, by := b.CeldaEntera()
		if bx == cx && by == cy {
			return true
		}
	}

	return false
}

// pasoDeMuestreo: cada cuantas CELDAS se mira el camino de un paso para ver si cruza una celda ocupada. Un cuadro
// de este POC tarda ~1,2 s y el jugador camina 1,875 celdas por segundo: el paso llega a 2,25 celdas, asi que
// mirar solo la celda de llegada dejaria pasar el cuerpo por encima del bicho (tunneling). Con 1/4 de celda el
// camino se mira 9 veces por paso y el bloqueo cae donde corresponde.
const pasoDeMuestreo = 0.25

// primerBloqueo: recorre el tramo (x0,y0)-(x1,y1) en muestras de `pasoDeMuestreo` celdas y devuelve la FRACCION
// del camino hasta donde se puede llegar (la ultima muestra libre) y si hubo bloqueo. `ocupada` dice que celdas
// estan tomadas: la de un bicho cuando se mueve un jugador, la de un jugador cuando se mueve un bicho.
func primerBloqueo(x0, y0, x1, y1 float64, ocupada func(int, int) bool) (float64, bool) {
	dx, dy := x1-x0, y1-y0

	dist := math.Sqrt(dx*dx + dy*dy)
	if dist <= 0 {
		return 1, false
	}

	muestras := int(dist/pasoDeMuestreo) + 1
	ultimo := 0.0

	for i := 1; i <= muestras; i++ {
		f := float64(i) / float64(muestras)
		cx := int(math.Floor(x0 + dx*f))
		cy := int(math.Floor(y0 + dy*f))

		if ocupada(cx, cy) {
			return ultimo, true
		}

		ultimo = f
	}

	return 1, false
}

// contarSolapamientos: cuantos pares jugador-bicho comparten celda en este cuadro. El numero del criterio:
// tiene que ser 0 SIEMPRE.
func contarSolapamientos() int {
	n := 0

	for _, b := range bichos {
		bx, by := b.CeldaEntera()

		for _, p := range jugadores {
			if p.Mov == nil || p.Mov.Est == nil {
				continue
			}

			px, py := p.Mov.Est.CeldaEntera()
			if px == bx && py == by {
				n++
			}
		}
	}

	if n > 0 {
		solapamientos += n
	}

	return n
}

// paresDeBichosEnLaMismaCelda: cuantos pares de BICHOS comparten celda en este cuadro. No es un fallo del
// medidor: los carriles estan a media celda unos de otros (separacionEntreBichos = 0,5) para que los cuatro
// entren en el lienzo, asi que dos bichos pueden caer en la misma celda. Se mide y se informa para que nadie lo
// confunda con un solapamiento bicho-jugador, que es el que la tarjeta exige en cero. La colision bicho-bicho
// (que uno frene al otro) es de DC-002.122, cuando haya IA que los haga chocar.
func paresDeBichosEnLaMismaCelda() int {
	n := 0

	for i := range bichos {
		ix, iy := bichos[i].CeldaEntera()

		for j := i + 1; j < len(bichos); j++ {
			jx, jy := bichos[j].CeldaEntera()
			if ix == jx && iy == jy {
				n++
			}
		}
	}

	return n
}

// cuadroDelBicho: el cuadro del ciclo que toca por el reloj de SU animacion (la misma cuenta del motor que usa el
// jugador: `CuadroDeLaAnimacion`).
//
// DC-002.122: si el bicho esta en un modo del golpe (GH recibiendo, DT muriendo, DD muerto) el ciclo y el reloj son
// los de ESE modo, y el reloj corre desde que el modo empezo (`MsModo`), no desde el arranque del paseo.
func (b *bicho) cuadroDelBicho() *CuadroDelJugador {
	if b.Direccion == -1 {
		return nil
	}

	if b.Modo != "" && b.Modo != modoBichoCamina {
		return b.cuadroDelModoDelGolpe()
	}

	ciclo := ciclosBicho[b.Direccion]
	if ciclo == nil || len(ciclo.Cuadros) == 0 {
		ciclo = cicloBichoQuieto
	}

	if ciclo == nil || len(ciclo.Cuadros) == 0 {
		return nil
	}

	ms := int(b.MsAnim)
	if ciclo.MsPorCuadro > 0 {
		ms /= ciclo.MsPorCuadro
	}

	f := ms % len(ciclo.Cuadros)
	b.CuadroCiclo = f

	return &ciclo.Cuadros[f]
}

// cuadroDelModoDelGolpe: el cuadro de GH/DT/DD. GH y DT son de un solo tiro (no dan la vuelta): el cuadro se clava
// en el ultimo cuando el reloj se pasa, y `pasoDelModoDelBicho` es el que hace el cambio de modo. DD tiene un solo
// cuadro y se queda ahi.
func (b *bicho) cuadroDelModoDelGolpe() *CuadroDelJugador {
	ciclo := b.CicloDelModo
	if ciclo == nil || len(ciclo.Cuadros) == 0 {
		ciclo = cicloDelModoDelBicho(b, b.Modo)
	}

	if ciclo == nil || len(ciclo.Cuadros) == 0 {
		return nil
	}

	ms := int(b.MsModo)
	if ciclo.MsPorCuadro > 0 {
		ms = int(int64(d2cMs())-b.MsModo) / ciclo.MsPorCuadro
	}

	// DC-002.126: con el instrumento (muertePorCuadro="si") el DT avanza un cuadro por cuadro del POC; sin el, el
	// cuadro sale del reloj del modo, como en DC-002.122.
	f := cuadroDeLaMuerte(b, ms)
	if f >= len(ciclo.Cuadros) {
		f = len(ciclo.Cuadros) - 1
	}

	if f < 0 {
		f = 0
	}

	b.CuadroCiclo = f

	return &ciclo.Cuadros[f]
}

// ============================== EL DIBUJO ==============================

// prepararElCuadroDeUnBicho: el cuadro, el ancla y la caja del bicho, y la copia de lo que hay DEBAJO de su caja.
func prepararElCuadroDeUnBicho(b *bicho, pixeles []byte) bool {
	// DC-002.126: el cadaver expirado ya no se dibuja (a los 12,0 s del motor). Es la ultima parte del criterio:
	// el bicho cae, queda el cadaver y a los 12 s desaparece del cuadro.
	if !elCadaverSeDibuja(b) {
		b.Pintados = 0

		return false
	}

	cu := b.cuadroDelBicho()
	if cu == nil {
		return false
	}

	cx, cy := b.Est.Celda()

	// la MISMA proyeccion que el jugador (AnclaDelJugador: WorldToScreenF + RenderOffset del motor)
	b.AnclaX, b.AnclaY = AnclaDelJugador(rendererDelMapa.WorldToScreenF, cx, cy)

	b.Ancho, b.Alto = cu.Ancho, cu.Alto
	b.CajaCruda = CajaDe(b.AnclaX, b.AnclaY, cu)
	b.Caja = recortarALaPantalla(b.CajaCruda)

	if b.Caja.Ancho() <= 0 || b.Caja.Alto() <= 0 {
		b.Pintados = 0 // se fue del lienzo: este cuadro no pinto nada

		return false
	}

	guardarCajaDelBicho(b, pixeles)

	return true
}

// guardarCajaDelBicho: copia lo que hay en el lienzo dentro de la caja del bicho (para devolver el mapa).
func guardarCajaDelBicho(b *bicho, pixeles []byte) {
	necesario := b.Caja.Ancho() * b.Caja.Alto() * 4

	if b.CajaGuardada == nil || len(b.CajaGuardada) < necesario {
		b.CajaGuardada = make([]byte, necesario)
	}

	n := 0

	for y := b.Caja.Y0; y < b.Caja.Y1; y++ {
		for x := b.Caja.X0; x < b.Caja.X1; x++ {
			j := (y*ancho + x) * 4
			copy(b.CajaGuardada[n:n+4], pixeles[j:j+4])

			n += 4
		}
	}
}

// restaurarCajaDelBicho: devuelve la caja del bicho a lo que habia debajo (el mapa).
func restaurarCajaDelBicho(b *bicho, pixeles []byte) {
	n := 0

	for y := b.Caja.Y0; y < b.Caja.Y1; y++ {
		for x := b.Caja.X0; x < b.Caja.X1; x++ {
			j := (y*ancho + x) * 4
			copy(pixeles[j:j+4], b.CajaGuardada[n:n+4])

			n += 4
		}
	}
}

// pixelesCambiadosDelBicho: cuantos pixeles de la caja del bicho NO son los del mapa.
func pixelesCambiadosDelBicho(b *bicho, pixeles []byte) int {
	n := 0
	cambiados := 0

	for y := b.Caja.Y0; y < b.Caja.Y1; y++ {
		for x := b.Caja.X0; x < b.Caja.X1; x++ {
			j := (y*ancho + x) * 4

			if pixeles[j] != b.CajaGuardada[n] || pixeles[j+1] != b.CajaGuardada[n+1] ||
				pixeles[j+2] != b.CajaGuardada[n+2] || pixeles[j+3] != b.CajaGuardada[n+3] {
				cambiados++
			}

			n += 4
		}
	}

	return cambiados
}

// dibujarLosBichos: pega los bichos en el lienzo, EN ORDEN DE PROFUNDIDAD (el que tiene menor x+y se dibuja
// primero), mide cuantos pixeles pinto cada uno y devuelve el crc32 del cuadro con los bichos encima. Despues de
// medir devuelve el mapa, en orden inverso, para que el numero del mapa solo siga siendo el del host.
func dibujarLosBichos(malla *pvr.Surface, pixeles []byte) (crcConBichos uint32, msTotal, pintados int) {
	crcConBichos = crc32De(pixeles)

	if len(bichos) == 0 {
		return crcConBichos, 0, 0
	}

	crcAntes := crc32De(pixeles)

	listos := make([]*bicho, 0, len(bichos))

	for _, b := range bichos {
		if prepararElCuadroDeUnBicho(b, pixeles) {
			listos = append(listos, b)
		}
	}

	// profundidad: insercion, igual que con los jugadores (y por el mismo motivo: nada de `sort` en la consola)
	for i := 1; i < len(listos); i++ {
		for j := i; j > 0; j-- {
			xj, yj := listos[j].Est.Celda()
			xp, yp := listos[j-1].Est.Celda()

			if xj+yj >= xp+yp {
				break
			}

			listos[j], listos[j-1] = listos[j-1], listos[j]
		}
	}

	// DC-002.126: los cuadros que se pintaron, para anotar la muerte DESPUES de devolver el mapa (que es cuando se
	// sabe si el lienzo volvio al crc32 de antes).
	pintadosCon := make([]*CuadroDelJugador, 0, len(listos))

	for _, b := range listos {
		cu := b.cuadroDelBicho()
		if cu == nil {
			continue
		}

		t1 := d2cMs()
		PintarJugador(malla, cu, b.AnclaX, b.AnclaY, palJugador[:])
		b.MsDibujo = int(d2cMs() - t1)

		msTotal += b.MsDibujo
		b.Pintados = pixelesCambiadosDelBicho(b, pixeles)
		pintados += b.Pintados

		pintadosCon = append(pintadosCon, cu)
	}

	crcConBichos = crc32De(pixeles)

	for i := len(listos) - 1; i >= 0; i-- {
		restaurarCajaDelBicho(listos[i], pixeles)
	}

	// DC-002.126: LA ANOTACION DE LA MUERTE. Se hace aca porque `lienzoRestaurado` solo se puede saber despues de
	// devolver todas las cajas: es el numero que prueba que el cadaver no deja rastro en el mapa.
	lienzoRestaurado := crc32De(pixeles) == crcAntes

	for i, b := range listos {
		if i < len(pintadosCon) {
			anotarElCuadroDeLaMuerte(b, pintadosCon[i], b.Pintados, b.MsDibujo, lienzoRestaurado)
		}
	}

	return crcConBichos, msTotal, pintados
}

// volverAPintarLosBichos: el cuadro que queda en pantalla (despues de devolver el mapa).
func volverAPintarLosBichos(malla *pvr.Surface) {
	for _, b := range bichos {
		if b.Caja.Ancho() <= 0 || b.Caja.Alto() <= 0 {
			continue
		}

		// DC-002.126: el cadaver expirado tampoco vuelve a pintarse: desaparecio del cuadro.
		if !elCadaverSeDibuja(b) {
			continue
		}

		cu := b.cuadroDelBicho()
		if cu == nil {
			continue
		}

		PintarJugador(malla, cu, b.AnclaX, b.AnclaY, palJugador[:])
	}
}

// detalleDelDibujoDeLosBichos: por bicho, donde cayo (ancla y caja), con que cuadro del ciclo y cuantos pixeles
// pinto. Es la evidencia de "existe, se dibuja y ocupa su celda".
func detalleDelDibujoDeLosBichos() string {
	s := ""

	for _, b := range bichos {
		if s != "" {
			s += " "
		}

		cx, cy := b.CeldaEntera()

		s += b.Nombre + ":ancla=" + itoa(b.AnclaX) + "," + itoa(b.AnclaY) +
			",caja=" + itoa(b.Caja.X0) + "," + itoa(b.Caja.Y0) + "," + itoa(b.Caja.X1) + "," + itoa(b.Caja.Y1) +
			",tamano=" + itoa(b.Ancho) + "x" + itoa(b.Alto) +
			",dir=" + itoa(b.Direccion) + ",cuadro=" + itoa(b.CuadroCiclo) +
			",pixeles=" + itoa(b.Pintados) + ",ms=" + itoa(b.MsDibujo) +
			",celda=" + itoa(cx) + "," + itoa(cy)
	}

	return s
}

// resumenDeLosBichos: una linea con lo que hizo CADA bicho en este cuadro (celda, direccion, cuadro del ciclo).
func resumenDeLosBichos() string {
	s := ""

	for _, b := range bichos {
		if s != "" {
			s += " "
		}

		cx, cy := b.Est.Celda()
		bx, by := b.CeldaEntera()

		s += b.Nombre + ":celda=" + itoa(bx) + "," + itoa(by) +
			",pos=" + ftoa(cx) + "," + ftoa(cy) +
			",dir=" + itoa(b.Direccion) + ",cuadro=" + itoa(b.CuadroCiclo) +
			",hacia=" + siNo(b.HaciaB) + ",extremos=" + itoa(b.Extremos) +
			",bloqueos=" + itoa(b.Bloqueos) + ",pintados=" + itoa(b.Pintados)
	}

	return s
}

// resumenFinalDeLosBichos: el cierre del criterio: que existieron, que se dibujaron y que ocuparon SU celda.
func resumenFinalDeLosBichos() {
	if len(bichos) == 0 {
		println("JUEGO121: BICHO RESUMEN monstruos= 0 (la corrida no tiene bichos)")

		return
	}

	celdas := 0.0
	extremos := 0
	vueltas := 0
	bloqueos := 0
	pintados := 0

	for _, b := range bichos {
		celdas += b.Celdas
		extremos += b.Extremos
		vueltas += b.VueltasOK
		bloqueos += b.Bloqueos
		pintados += b.Pintados
	}

	bx, by := bichos[0].CeldaEntera()

	// los pares de bichos que comparten celda: NO es un solapamiento bicho-jugador (esos tienen que ser 0), es lo
	// que pasa por tener los carriles a media celda. Se mide y se informa, no se esconde.
	celdasRepetidasEntreBichos := paresDeBichosEnLaMismaCelda()

	println("JUEGO121: BICHO RESUMEN monstruos=", len(bichos), "tipo=", claseBicho, "token=", tokenBicho,
		"modo=", modoBichoCamina, "celdaB1=", bx, by, "celdasCaminadas=", celdas,
		"celdasPorSegundo=", celdasPorSegundoDeLosBichos(), "extremos=", extremos, "vueltas=", vueltas,
		"bloqueosDelBicho=", bloqueos, "bloqueosDeJugadores=", bloqueosDeJugadores,
		"solapamientos=", solapamientos, "celdasRepetidasEntreBichos=", celdasRepetidasEntreBichos,
		"pixelesDelUltimoCuadro=", pintados,
		"indicesCompartidosBytes=", bytesBichoCompartidos, "estadoPorBichoBytes=", tamanoDeBicho,
		"cuadrosDelTipo=", len(ciclosBicho), "cuadrosDelTipoIndicesBytes=", bytesBichoCompartidos)

	println("JUEGO121: BICHO CRITERIO dibujado=", pintados > 0, "patrullando=", celdas > 0,
		"sinSolapamientos=", solapamientos == 0, "cuadros=", len(ciclosBicho),
		"bichos=", len(bichos), "celdas=", celdas)
}

// celdasPorSegundoDeLosBichos: las celdas caminadas por el bicho 1 sobre el reloj que patrullo.
func celdasPorSegundoDeLosBichos() float64 {
	if len(bichos) == 0 || bichos[0].Est == nil {
		return 0
	}

	b := bichos[0]
	if b.Tiempo <= 0 {
		return 0
	}

	return b.Celdas / b.Tiempo
}

// ftoa: un float a texto con dos decimales, para armar las lineas del log (la consola no lleva `strconv`).
func ftoa(v float64) string {
	negativo := v < 0
	if negativo {
		v = -v
	}

	// +0,5 para redondear el segundo decimal (la consola no tiene math.Round en el subconjunto que se porta)
	mi := int64(v*100 + 0.5)
	entero := mi / 100
	frac := mi % 100

	s := itoa(int(entero)) + "."
	if frac < 10 {
		s += "0"
	}

	s += itoa(int(frac))

	if negativo {
		return "-" + s
	}

	return s
}

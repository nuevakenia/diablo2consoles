#!/usr/bin/env python3
# DC-002.49b -- aplicar EL GIRO al POC del personaje, de forma IDEMPOTENTE y sobre el main.go que HAYA.
#
# POR QUE UN SCRIPT Y NO UN .patch
# --------------------------------
# El giro se escribio el 2026-10-19 a las 15:41 sobre una copia del POC con `diff -u` (145 lineas, base
# main.go 565bc90f). Ese parche nacio viejo: `console/dc/poc/personaje-mapa/main.go` lo esta reescribiendo
# otra tarjeta (DC-002.50, el monstruo y el golpe) y para las 16:48 ya habia cambiado 672 lineas, con el
# struct `ciclo` ganando el campo `UnTiro`, `animar()` ganando el modo del golpe y `cargarCiclos` cambiando
# de forma. Un `patch` por numero de linea falla ahi (y peor: aplica a medias). Este script busca CADA
# anclaje por su texto, exige que aparezca EXACTAMENTE UNA VEZ, y no escribe nada si alguno no cuadra:
# o aplica el giro entero, o no toca el archivo.
#
# QUE HACE (7 ediciones + un archivo nuevo)
#   1. `type ciclo struct`: el campo `Dir` (la direccion del COF de ese ciclo).
#   2. `cargarCicloCon`: `Dir: dir` al construir el ciclo (el `dir` ya lo recibia: solo no se guardaba).
#   3. `animar()`: pide el ciclo con `dirActual` (y la caida al quieto tambien).
#   4. `buscarCiclo(modo, dir)`: busca por modo Y direccion, con caida a la direccion 0 del modo.
#   5. `combate()`: el ciclo del golpe tambien por `dirActual`.
#   6. `Update()`: el gancho que traduce la cruceta a la direccion del COF (la cadena del motor) y la deja
#      en `dirActual`, con la marca `HITO249B: GIRO` (una linea por CAMBIO de direccion).
#   7. `cargarCiclos`: compone las direcciones que el control puede pedir (medido: 8, `[0 1 4 7 9 8 12 15]`)
#      en vez de una sola, con la marca `HITO249B: DIRECCIONES` y la memoria despues de cada una.
#   8. `giro.go`: el archivo con la cadena del motor y el conjunto de 8 direcciones, copiado de
#      `scripts/console/giro-249b.go.txt` (el mismo que se compilo con sha256 5e2e34ad...).
#
# Uso:
#   scripts/console/aplicar-giro-249b.py                    # aplica sobre el POC del repo
#   scripts/console/aplicar-giro-249b.py --poc /tmp/copia   # aplica sobre una COPIA (para compilar sin tocar el repo)
#   scripts/console/aplicar-giro-249b.py --verificar        # dice si esta aplicado (0 = si, 1 = no); no escribe
#   scripts/console/aplicar-giro-249b.py --mutante          # giroSordo=true en giro.go (el mutante del criterio 4)
#   scripts/console/aplicar-giro-249b.py --revertir         # vuelve al main.go de antes (el .pre-giro de la 1a aplicacion)
#
# Salida: una linea por edicion ("aplicada" / "ya estaba") y al final el sha256 del main.go resultante.
import argparse
import hashlib
import shutil
import sys
from pathlib import Path

REPO = Path(__file__).resolve().parents[2]
POC_DEF = REPO / "console/dc/poc/personaje-mapa"
PLANTILLA_GIRO = Path(__file__).resolve().parent / "giro-249b.go.txt"

# -----------------------------------------------------------------------------------------------------------
# Las 7 ediciones: (nombre, buscar, poner, marca de "ya aplicado")
# `buscar` y `poner` van con TABULADORES explicitos (\t) porque el archivo es Go y gofmt usa tabs.
# -----------------------------------------------------------------------------------------------------------

E1_BUSCAR = "type ciclo struct {\n\tModo        string\n"
E1_PONER = ("type ciclo struct {\n\tModo        string\n"
            "\tDir         int // DC-002.49b: la direccion del COF de este ciclo (el COF del heroe tiene 16).\n")
E1_MARCA = "// DC-002.49b: la direccion del COF de este ciclo"

E2_BUSCAR = "\tc := &ciclo{Modo: modo, MsPorCuadro: msPorCuadro, UnTiro: unTiro}\n"
E2_PONER = "\tc := &ciclo{Modo: modo, Dir: dir, MsPorCuadro: msPorCuadro, UnTiro: unTiro}\n"
E2_MARCA = "c := &ciclo{Modo: modo, Dir: dir,"

E3A_BUSCAR = "\tc := j.buscarCiclo(deseado)\n"
E3A_PONER = "\tc := j.buscarCiclo(deseado, dirActual)\n"
E3A_MARCA = "c := j.buscarCiclo(deseado, dirActual)"

E3B_BUSCAR = "\t\tc = j.buscarCiclo(modoQuieto)\n"
E3B_PONER = "\t\tc = j.buscarCiclo(modoQuieto, dirActual)\n"
E3B_MARCA = "c = j.buscarCiclo(modoQuieto, dirActual)"

E4_BUSCAR = (
    "// buscarCiclo: el ciclo de un modo, si esta cargado.\n"
    "func (j *juego) buscarCiclo(modo string) *ciclo {\n"
    "\tfor _, c := range j.ciclos {\n"
    "\t\tif c.Modo == modo {\n"
    "\t\t\treturn c\n"
    "\t\t}\n"
    "\t}\n"
    "\n"
    "\treturn nil\n"
    "}\n"
)
E4_PONER = (
    "// buscarCiclo: el ciclo de un modo y una direccion, si esta cargado (DC-002.49b).\n"
    "func (j *juego) buscarCiclo(modo string, dir int) *ciclo {\n"
    "\tfor _, c := range j.ciclos {\n"
    "\t\tif c.Modo == modo && c.Dir == dir {\n"
    "\t\t\treturn c\n"
    "\t\t}\n"
    "\t}\n"
    "\n"
    "\t// la direccion pedida puede no estar compuesta (RAM): se dibuja la 0 de ese modo, que es la que dibujaba\n"
    "\t// DC-002.38, en vez de dejar al personaje con el ciclo de OTRA direccion.\n"
    "\tfor _, c := range j.ciclos {\n"
    "\t\tif c.Modo == modo && c.Dir == 0 {\n"
    "\t\t\treturn c\n"
    "\t\t}\n"
    "\t}\n"
    "\n"
    "\treturn nil\n"
    "}\n"
)
E4_MARCA = "func (j *juego) buscarCiclo(modo string, dir int) *ciclo {"

E5_BUSCAR = "\tgolpe := j.buscarCiclo(modoGolpePersonaje)\n"
E5_PONER = "\tgolpe := j.buscarCiclo(modoGolpePersonaje, dirActual)\n"
E5_MARCA = "golpe := j.buscarCiclo(modoGolpePersonaje, dirActual)"

E6_BUSCAR = "\tif !controlSordo && (ejeX != 0 || ejeY != 0) && paso > 0 {\n"
E6_PONER = (
    "\t// ---- LA DIRECCION (DC-002.49b): la cruceta elige QUE direccion del COF mira el personaje; la cadena es\n"
    "\t// la del motor (DireccionDelPiso -> DirectionTo -> Dir64ToCof) y las 8 direcciones que puede pedir estan\n"
    "\t// compuestas al arranque, asi que girar no cuesta nada en el cuadro.\n"
    "\tif !giroSordo && (ejeX != 0 || ejeY != 0) {\n"
    "\t\td := direccionDelControl(ejeX, ejeY)\n"
    "\n"
    "\t\tif d != dirActual {\n"
    "\t\t\tdirActual = d\n"
    "\n"
    "\t\t\tprintln(\"HITO249B: GIRO dir\", dirActual, \"ejes\", int(ejeX), int(ejeY),\n"
    "\t\t\t\t\"cuadroDelLoop\", j.cuadros, \"mutante\", giroSordo)\n"
    "\t\t}\n"
    "\t}\n"
    "\n"
    "\tif !controlSordo && (ejeX != 0 || ejeY != 0) && paso > 0 {\n"
)
E6_MARCA = 'HITO249B: GIRO dir'

E7_BUSCAR = ("\t\t\tci := cargarCiclo(mc, cofModo, dirPersonaje, m.modo, m.ms, palUni)\n"
             "\t\t\tciclos = append(ciclos, ci)\n")
E7_PONER = (
    "\t\t\tdirsCOFDelPersonaje = cofModo.NumberOfDirections\n"
    "\n"
    "\t\t\t// DC-002.49b: se componen TODAS las direcciones que la cruceta puede pedir (medido: 8), no una.\n"
    "\t\t\t// MEDIDO EN LA CONSOLA (2026-10-19 17:38): las 8 de TN entran -- quedan 72.368 B de monton despues\n"
    "\t\t\t// de componerlas (arranca con 799.096) -- y las 8 de TW NO: moria con \"fatal error: out of memory\"\n"
    "\t\t\t// en la TERCERA direccion, con 72.368 B libres contra un pico de 66.859 B por ciclo y ~37 KB\n"
    "\t\t\t// permanentes por direccion. Por eso el giro de las 8 va SOLO en el modo quieto (TN), que es el que\n"
    "\t\t\t// se dibuja con el personaje parado; TW sigue con la direccion 0.\n"
    "\t\t\tpedidas := []int{dirPersonaje}\n"
    "\n"
    "\t\t\tif m.modo == modoQuieto {\n"
    "\t\t\t\tpedidas = direccionesDelControl()\n"
    "\t\t\t}\n"
    "\n"
    "\t\t\tprintln(\"HITO249B: DIRECCIONES modo\", m.modo, \"pedidas\", len(pedidas),\n"
    "\t\t\t\t\"dirsCOF\", dirsCOFDelPersonaje)\n"
    "\n"
    "\t\t\tfor _, dirPedida := range pedidas {\n"
    "\t\t\t\tciclos = append(ciclos, cargarCiclo(mc, cofModo, dirPedida, m.modo, m.ms, palUni))\n"
    "\n"
    "\t\t\t\tprintln(\"HITO249B: DIR modo\", m.modo, \"dir\", dirPedida)\n"
    "\n"
    "\t\t\t\tmemoria(\"ciclo-\" + m.modo)\n"
    "\t\t\t}\n"
)
E7_MARCA = 'HITO249B: DIRECCIONES modo'

EDICIONES = [
    ("1 struct ciclo.Dir", E1_BUSCAR, E1_PONER, E1_MARCA),
    ("2 cargarCicloCon: Dir: dir", E2_BUSCAR, E2_PONER, E2_MARCA),
    ("3a animar: ciclo(deseado, dirActual)", E3A_BUSCAR, E3A_PONER, E3A_MARCA),
    ("3b animar: caida al quieto con dirActual", E3B_BUSCAR, E3B_PONER, E3B_MARCA),
    ("4 buscarCiclo(modo, dir)", E4_BUSCAR, E4_PONER, E4_MARCA),
    ("5 combate: el golpe por dirActual", E5_BUSCAR, E5_PONER, E5_MARCA),
    ("6 Update: la cruceta -> dirActual", E6_BUSCAR, E6_PONER, E6_MARCA),
    ("7 cargarCiclos: las 8 direcciones", E7_BUSCAR, E7_PONER, E7_MARCA),
]

MUTANTE_OFF = "var giroSordo = false"
MUTANTE_ON = "var giroSordo = true"


def sha(texto):
    return hashlib.sha256(texto.encode()).hexdigest()[:16]


def aplicar_ediciones(texto):
    """Devuelve (texto_nuevo, informe, errores). No escribe nada: o todas o ninguna."""
    informe, errores = [], []
    for nombre, buscar, poner, marca in EDICIONES:
        if marca in texto:
            informe.append((nombre, "ya estaba"))
            continue
        n = texto.count(buscar)
        if n != 1:
            errores.append(f"{nombre}: el anclaje aparece {n} veces (tiene que ser 1)")
            continue
        texto = texto.replace(buscar, poner, 1)
        informe.append((nombre, "aplicada"))
    return texto, informe, errores


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--poc", default=str(POC_DEF), help="directorio del POC (por defecto el del repo)")
    ap.add_argument("--verificar", action="store_true", help="solo dice si esta aplicado")
    ap.add_argument("--mutante", action="store_true", help="giroSordo=true en giro.go")
    ap.add_argument("--revertir", action="store_true", help="vuelve al main.go .pre-giro")
    args = ap.parse_args()

    poc = Path(args.poc).resolve()
    main_go = poc / "main.go"
    giro_go = poc / "giro.go"
    respaldo = poc / "main.go.pre-giro"

    if not main_go.is_file():
        print(f"FALLO: no existe {main_go}")
        return 1

    texto = main_go.read_text()

    # ---- verificar
    if args.verificar:
        faltan = [n for n, _, _, m in EDICIONES if m not in texto]
        tiene_giro = giro_go.is_file()
        print(f"main.go  {main_go}")
        print(f"giro.go  {'si' if tiene_giro else 'NO'}  {sha(giro_go.read_text()) if tiene_giro else '-'}")
        if faltan or not tiene_giro:
            print(f"GIRO NO APLICADO: faltan {faltan}" if faltan else "GIRO NO APLICADO: falta giro.go")
            return 1
        print("GIRO APLICADO (8/8 ediciones + giro.go)")
        return 0

    # ---- revertir
    if args.revertir:
        if not respaldo.is_file():
            print(f"FALLO: no hay respaldo {respaldo} (el script no lo creo, o ya se revirtio)")
            return 1
        shutil.copy2(respaldo, main_go)
        if giro_go.is_file():
            giro_go.unlink()
        print(f"revertido: main.go <- {respaldo.name} (sha256 {sha(main_go.read_text())}), giro.go borrado")
        return 0

    # ---- aplicar
    nuevo, informe, errores = aplicar_ediciones(texto)

    for nombre, estado in informe:
        print(f"   {nombre}: {estado}")
    if errores:
        print()
        for e in errores:
            print(f"   FALLO {e}")
        print("   NO se escribio nada (el main.go quedo como estaba)")
        return 1

    # giro.go: la plantilla, tal cual (sha256 en la cabecera de la plantilla no va: se imprime abajo)
    if not PLANTILLA_GIRO.is_file():
        print(f"   FALLO: no encuentro la plantilla {PLANTILLA_GIRO}")
        return 1
    plantilla = PLANTILLA_GIRO.read_text()

    if args.mutante:
        if MUTANTE_ON in plantilla:
            print("   mutante: ya estaba (giroSordo=true)")
        elif MUTANTE_OFF in plantilla:
            plantilla = plantilla.replace(MUTANTE_OFF, MUTANTE_ON, 1)
            print("   mutante: giroSordo=true")
        else:
            print("   FALLO: no encuentro 'var giroSordo = false' en la plantilla")
            return 1

    cambios = nuevo != texto
    giro_igual = giro_go.is_file() and giro_go.read_text() == plantilla

    if not cambios and giro_igual:
        print("   nada que escribir: ya estaba aplicado y giro.go coincide")
    else:
        if cambios:
            if not respaldo.is_file():
                shutil.copy2(main_go, respaldo)
                print(f"   respaldo: {respaldo.name} (sha256 {sha(texto)})")
            main_go.write_text(nuevo)
        giro_go.write_text(plantilla)

    print(f"main.go  {sha(main_go.read_text())}  ({len(main_go.read_text().splitlines())} lineas)")
    print(f"giro.go  {sha(giro_go.read_text())}  ({len(giro_go.read_text().splitlines())} lineas)")
    return 0


if __name__ == "__main__":
    sys.exit(main())

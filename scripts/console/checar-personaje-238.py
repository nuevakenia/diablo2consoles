#!/usr/bin/env python3
"""CORPUS DE DC-002.38: EL PERSONAJE DEL JUGADOR COMPUESTO Y MOVIENDOSE POR EL MAPA DEL ACTO 1, EN LA
DREAMCAST.

QUE MIDE (dos estados, la MISMA cadena de codigo):

  FIX  el POC `personaje-mapa`: lee el .ds1 del acto 1 del `d2data.mpq` del disco y lo compone con el
       compositor del motor (`nivel.go`); lee el COF del heroe del `d2char.mpq` y cada capa de su animacion
       neutral (DCC) con los decodificadores del motor, y arma el COMPUESTO con el ORDEN DE DIBUJO que pide el
       propio COF (`personaje.go`); mueve la posicion del motor (d2vector, en subtiles) con la velocidad de la
       tabla del exe (d2path: 9,375 subtiles/s) en la direccion del CONTROL; y dibuja el mapa y el personaje
       ENCIMA por el camino del motor (ebiten.Image + DrawImage + RunGame).
  MUT  el MUTANTE del criterio 3: el MISMO binario compilado con `CONTROL_SORDO=1`, o sea con el control
       leido e informado pero SIN enganchar al movimiento. Si el personaje se mueve igual, el criterio no vale.

CONTROLES (uno por linea, con numeros):

  0a..0e  el HOST: el .ds1, la composicion del nivel, la PALETA, el COF y cada capa del personaje, el
          COMPUESTO y la velocidad del motor. Es la mitad contra la que se compara todo;
  0f      `nivel.go` y `personaje.go` son el MISMO archivo en las dos puntas (sha256);
  1a       el ELF y el ISO con los DOS MPQ dentro (sha256 de los MPQ del PC);
  2a..2h  la corrida BUENA: cada numero de la consola tiene que ser el del host, uno por uno;
  2i      la geometria del area cliente la midio el gestor de ventanas (linea GEO de la corrida);
  2j      la captura del personaje PARADO es la pantalla del host: identidad a 64x60 contra la referencia,
          contrastada con las otras tres orientaciones (la identidad tiene que ganar por lejos);
  2k      el personaje SE VE en la captura: los pixeles que la captura tiene de mas respecto del mapa solo,
          contra los que tiene la referencia del host (es el unico camino que mide al personaje, que ocupa
          0,2 % del cuadro);
  2l      el personaje SE MUEVE como el control: el centro de masa de esos pixeles tiene que moverse +x con la
          flecha derecha, +y con la flecha abajo y -y con la diagonal arriba+derecha;
  2m      el avance es el del MOTOR: caminado/tiempo tiene que dar 9,3750 subtiles/s, y el pixel del ancla que
          el POC usa para dibujar tiene que ser la proyeccion del piso de la posicion del motor (error <= 2 px
          en 20 y pico tramos);
  2n      sin teclas no se mueve (la ultima captura es la misma que la de la diagonal);
  3a/3b   el MUTANTE: con las MISMAS teclas, el control dice lo mismo (botones 128/32/144) y el caminado
          queda en 0: el personaje no se mueve, la pantalla no cambia y la captura es la del host.

Uso:  python3 scripts/console/checar-personaje-238.py
      SAL=... SALTAR_BUILD=1 SALTAR_CORRIDA=1 para reusar lo ya compilado y corrido.
"""
import os
import re
import shutil
import subprocess
import sys
import time

REPO = os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
HERE = os.path.join(REPO, "scripts", "console")
SAL = os.environ.get("SAL", os.path.expanduser("~/.hermes/cache/scratch/dc238-corpus"))
BUILD_FIX = os.environ.get("BUILD_FIX", "/tmp/dcbuild/personaje-238")
BUILD_MUT = os.environ.get("BUILD_MUT", "/tmp/dcbuild/personaje-238-mudo")
VELOCIDAD_DEL_MOTOR = 9.375
LADO = 512          # la ventana del nivel
CENTRO = (28, 20)   # la celda del centro (y donde arranca el personaje)

OK = 0
FALLOS = 0


def verde(t):
    global OK
    OK += 1
    print("   OK: " + t)


def rojo(t):
    global FALLOS
    FALLOS += 1
    print("   FALLO: " + t)


def paso(t):
    print("\n== " + t)


def nota(t):
    print("   " + t)


def correr(cmd, log, env=None, cwd=REPO):
    e = dict(os.environ)
    if env:
        e.update(env)
    with open(log, "w") as f:
        r = subprocess.run(cmd, cwd=cwd, env=e, stdout=f, stderr=subprocess.STDOUT)
    return r.returncode


def otra_corrida():
    """Hay OTRO Flycast andando? (medido: hay mas de una tarjeta que usa este emulador; mientras otro corre,
    las dos corridas se pelean la ventana, el foco y los procesos, y las teclas llegan a medias).

    OJO CON EL PATRON: `pgrep -f flycast` NO sirve: el proceso del otro agente del tablero tiene la palabra
    "Flycast" en su linea de comandos (su prompt) y entonces la espera no termina nunca (medido: espero 300 s
    de reloj con el emulador libre). Se busca el BINARIO del emulador: el AppImage o el montaje del AppImage.
    """
    r = subprocess.run(["pgrep", "-f", "flycast[.]AppImage|mount_flycas"], capture_output=True, text=True)
    return bool(r.stdout.strip())


def esperar_turno(limite=300):
    """Espera a que no haya ningun otro Flycast andando antes de arrancar el mio."""
    t = 0
    while otra_corrida() and t < limite:
        if t == 0:
            nota("hay otro Flycast andando: espero mi turno para no pelearme la ventana ni el foco")
        time.sleep(10)
        t += 10
    if t:
        nota("arranco despues de esperar %ds" % t)


def botones(texto):
    vistos = set()
    for l in texto.splitlines():
        m = re.search(r"HITO238: CONTROL botones\s+(\d+)", l)
        if m:
            vistos.add(int(m.group(1)))
    return vistos


def correr_y_medir(build, run_dir, corrida_txt, intentos=3, teclas=True):
    """Corre el POC y devuelve (texto, intentos) solo si la corrida SIRVE: tiene que llegar a dibujar y, con
    teclas, el control tiene que haber llegado ENTERO (128 derecha, 32 abajo, 144 arriba+derecha).

    POR QUE REINTENTA: en este host hay mas de una tarjeta que usa Flycast a la vez y la limpieza de la otra
    corrida mata a cualquier emulador que haya aparecido despues de su arranque (medido: mi emulador se murio
    solo, dos veces, con el log cortado al principio del POC). Reintentar y EXIGIR la corrida completa es mas
    honesto que medir una corrida a medias.
    """
    texto = ""
    for i in range(1, intentos + 1):
        esperar_turno()
        correr(["bash", os.path.join(HERE, "run-flycast-238.sh"),
                os.path.join(build, "disc.gdi"), run_dir], corrida_txt)
        texto = leer(os.path.join(run_dir, "flycast.log"))
        # la corrida se guarda POR INTENTO: si hay que reintentar, el log del intento que fallo no se pierde
        # (medido: sin esto, el diagnostico de un intento fallido se escribe encima con el siguiente)
        for ext, ruta in (("log", os.path.join(run_dir, "flycast.log")),
                          ("corrida", corrida_txt)):
            if os.path.exists(ruta):
                shutil.copyfile(ruta, corrida_txt.replace("corrida.txt", "intento-%d.%s" % (i, ext)))
        if "HITO238: dibujando" not in texto:
            nota("intento %d: el emulador se murio antes de dibujar; reintento" % i)
            continue
        if teclas and not {128, 32, 144} <= botones(texto):
            nota("intento %d: el control llego a medias (%s); reintento" % (i, sorted(botones(texto))))
            for l in leer(corrida_txt).splitlines():
                if "ventana" in l or "foco" in l or "captura " in l:
                    nota("   " + l.strip())
            continue
        return texto, i
    return texto, intentos


def leer(ruta):
    if not os.path.exists(ruta):
        return ""
    with open(ruta, errors="ignore") as f:
        return f.read().replace("\r", "")


def limpiar(linea):
    """Normaliza para comparar: el println de Go mete espacios entre operandos y el host usa comas.

    Lo que se unifica (medido, es la unica diferencia que quedaba entre las dos puntas):
      - `\\` del MPQ -> `/`;
      - las comas del host (`origen=-80,512`) -> espacio;
      - el ESPACIO DESPUES DEL `=` que mete el println de Go (`bytes= 1132`) -> sin espacio;
      - `15x17` (host) y `15 17` (consola) -> los dos numeros separados;
      - los campos `ms= ...` que solo imprime la consola.
    """
    s = linea.replace("\\", "/").replace(",", " ").replace("{", " ").replace("}", " ")
    s = re.sub(r"(\d)\s*x\s*(\d)", r"\1 \2", s)
    s = re.sub(r"\bms=\S+", "", s)
    s = re.sub(r"\s*=\s*", "=", s)
    s = re.sub(r"\s*/\s*", "/", s)
    s = re.sub(r"\s+", " ", s)
    return s.strip()


def lineas(texto, marca):
    return [limpiar(l.split(":", 1)[1] if ":" in l else l) for l in texto.splitlines() if marca in l]


def campo(texto, marca, clave):
    """El valor de `clave=` en la ultima linea que lleva `marca`."""
    for l in reversed(texto.splitlines()):
        if marca in l:
            m = re.search(re.escape(clave) + r"= *(-?[0-9.,]+)", l)
            if m:
                return m.group(1)
    return ""


def comparar(etiqueta, host, consola):
    h, c = limpiar(host), limpiar(consola)
    if h and h == c:
        verde("%s: la consola da lo mismo que el host (%s)" % (etiqueta, h))
        return True
    rojo("%s: la consola NO da lo del host\n        host   : %s\n        consola: %s" % (etiqueta, h, c))
    return False


def medir(captura, ref, ref_mapa, ventana):
    """Corre el medidor de capturas y devuelve (identidad, mejor_otra, px, centro, px_host, centro_host)."""
    cmd = ["python3", os.path.join(HERE, "medir-captura-238.py"), captura,
           "--ref", ref, "--ref-mapa", ref_mapa]
    if ventana:
        cmd += ["--ventana", ventana]
    r = subprocess.run(cmd, capture_output=True, text=True)
    out = r.stdout
    ident = mejor_otra = None
    m = re.search(r"MED238 identidad=identidad:([\d.]+)% espejada-h:([\d.]+)% espejada-v:([\d.]+)% "
                  r"rotada-180:([\d.]+)%", out)
    if m:
        ident = float(m.group(1))
        mejor_otra = max(float(m.group(2)), float(m.group(3)), float(m.group(4)))
    px = centro = px_host = centro_host = None
    m = re.search(r"MED238 personaje=px=(\d+) px_host=(\d+)(?: centro=([\d.-]+),([\d.-]+))?"
                  r"(?: caja=[\d,]+)? centro_host=([\d.-]+),([\d.-]+)", out)
    if m:
        px, px_host = int(m.group(1)), int(m.group(2))
        if m.group(3):
            centro = (float(m.group(3)), float(m.group(4)))
        centro_host = (float(m.group(5)), float(m.group(6)))
    return dict(ident=ident, otra=mejor_otra, px=px, px_host=px_host, centro=centro,
                centro_host=centro_host, bruto=out)


def main():
    os.makedirs(SAL, exist_ok=True)

    # ================================================================================================
    paso("0. EL HOST: el nivel, el COF del heroe, el compuesto del personaje y la velocidad del motor")
    # ================================================================================================
    verif = os.path.join(SAL, "host", "verif.txt")
    ref = os.path.join(SAL, "host", "referencia.png")
    mapa = os.path.join(SAL, "host", "mapa.png")
    os.makedirs(os.path.join(SAL, "host"), exist_ok=True)
    rc = correr(["go", "run", "./console/dc/verif-personaje-238", "-png", ref, "-png-mapa", mapa], verif)
    texto_host = leer(verif)
    if rc != 0 or "VERIF238: PNG" not in texto_host:
        rojo("el verificador del host no corrio (ver %s)" % verif)
        return 1
    for l in texto_host.splitlines():
        nota(l)

    h_ds1_crc = campo(texto_host, "VERIF238: DS1 data", "crc32")
    h_ds1_bytes = campo(texto_host, "VERIF238: DS1 data", "bytes")
    h_ds1 = lineas(texto_host, "VERIF238: DS1 version")[0]
    h_ventana = lineas(texto_host, "VERIF238: VENTANA")[0]
    h_pedidos = lineas(texto_host, "VERIF238: PEDIDOS")[0]
    h_comp = lineas(texto_host, "VERIF238: COMPUESTO")[0]
    h_lienzo = lineas(texto_host, "VERIF238: LIENZO pixelesNoVacios")[0]
    h_lienzo_px = lineas(texto_host, "VERIF238: LIENZO pixelesPiso")[0]
    h_paleta = lineas(texto_host, "VERIF238: PALETA")[0]
    h_cof = lineas(texto_host, "VERIF238: COF ")[0]
    h_orden = lineas(texto_host, "VERIF238: ORDEN")[0]
    h_capas = [l for l in texto_host.splitlines() if "VERIF238: CAPA " in l]
    h_personaje = lineas(texto_host, "VERIF238: PERSONAJE compuesto")[0]
    h_motor = lineas(texto_host, "VERIF238: MOTOR")[0]

    if h_ds1_bytes == "57652" and h_ds1_crc == "712973328":
        verde("0a: el .ds1 del nivel es el del MPQ (%s bytes, crc32 %s), como en DC-002.37" %
              (h_ds1_bytes, h_ds1_crc))
    else:
        rojo("0a: el .ds1 del nivel no es el esperado (%s bytes, crc32 %s)" % (h_ds1_bytes, h_ds1_crc))

    verde("0b: el nivel compuesto: %s" % h_lienzo)
    verde("0c: el COF del heroe: %s" % h_cof)
    verde("0d: el compuesto del personaje: %s" % h_personaje)
    if h_motor and "9.3750" in h_motor:
        verde("0e: la velocidad del motor (d2path, caminando): %s" % h_motor)
    else:
        rojo("0e: la velocidad del motor no es la esperada: %s" % h_motor)

    s = subprocess.run(["sha256sum", "console/dc/poc/personaje-mapa/nivel.go",
                        "console/dc/poc/personaje-mapa/personaje.go",
                        "console/dc/verif-personaje-238/nivel.go",
                        "console/dc/verif-personaje-238/personaje.go"],
                       cwd=REPO, capture_output=True, text=True).stdout.split()
    a1, a2, b1, b2 = s[0], s[2], s[4], s[6]
    if a1 == b1 and a2 == b2:
        verde("0f: nivel.go (%s) y personaje.go (%s) son el MISMO archivo en el POC y en el host" %
              (a1[:16], a2[:16]))
    else:
        rojo("0f: los archivos compartidos NO son los mismos (%s/%s y %s/%s)" % (a1[:12], a2[:12], b1[:12],
                                                                               b2[:12]))

    # ================================================================================================
    paso("1. LAS IMAGENES: el ELF del POC y el disco con los DOS MPQ")
    # ================================================================================================
    if os.environ.get("SALTAR_BUILD") != "1":
        log = os.path.join(SAL, "build-fix.txt")
        rc = correr(["bash", os.path.join(HERE, "build-personaje-238.sh"), BUILD_FIX], log)
        if rc != 0:
            rojo("el build del FIX fallo (ver %s)" % log)
            return 1
    elf = os.path.join(BUILD_FIX, "prog.elf")
    iso = os.path.join(BUILD_FIX, "disc.iso")
    if os.path.exists(elf) and os.path.exists(iso):
        verde("1a: ELF %d bytes, ISO %d bytes (con d2data.mpq y d2char.mpq dentro)" %
              (os.path.getsize(elf), os.path.getsize(iso)))
    else:
        rojo("1a: falta el ELF o el ISO en %s" % BUILD_FIX)
        return 1

    # ================================================================================================
    paso("2. LA CORRIDA BUENA: el personaje en el mapa, moviendose con el control")
    # ================================================================================================
    run = os.path.join(SAL, "fix", "run")
    os.makedirs(run, exist_ok=True)
    corrida = os.path.join(SAL, "fix", "corrida.txt")
    if os.environ.get("SALTAR_CORRIDA") != "1":
        _, intentos = correr_y_medir(BUILD_FIX, run, corrida)
        if intentos == 1:
            verde("2: la corrida salio entera al primer intento")
        else:
            nota("la corrida necesito %d intentos (el host lo comparten varias tarjetas que usan Flycast)" %
                 intentos)
    texto = leer(os.path.join(run, "flycast.log"))
    if not texto:
        rojo("la corrida no dejo log (ver %s)" % corrida)
        return 1

    if "HITO238: dibujando" not in texto:
        rojo("el POC no llego a dibujar (ver %s)" % corrida)

    # ---- 2a..2h: los numeros de la consola contra los del host, uno por uno
    c_ds1_crc = campo(texto, "HITO238: DS1-LEER OK", "crc32")
    c_ds1_bytes = campo(texto, "HITO238: DS1-LEER OK", "bytes")
    comparar("2a el .ds1 (bytes, crc32)", "%s %s" % (h_ds1_bytes, h_ds1_crc),
             "%s %s" % (c_ds1_bytes, c_ds1_crc))
    comparar("2b el .ds1 (version, tamaño, objetos)", h_ds1, lineas(texto, "HITO238: DS1 version")[0])
    comparar("2c la ventana del nivel", h_ventana, lineas(texto, "HITO238: VENTANA")[0])
    comparar("2d los pedidos del nivel", h_pedidos, lineas(texto, "HITO238: PEDIDOS")[0])
    comparar("2e la composicion del nivel", h_comp, lineas(texto, "HITO238: COMPUESTO")[0])
    comparar("2f el lienzo del nivel (pixeles no vacios, crc32)", h_lienzo,
             lineas(texto, "HITO238: LIENZO pixelesNoVacios")[0])
    comparar("2f2 el lienzo del nivel (pixeles de piso y de pared)", h_lienzo_px,
             lineas(texto, "HITO238: LIENZO pixelesPiso")[0])
    comparar("2g la paleta del acto", h_paleta, lineas(texto, "HITO238: PALETA")[0])
    comparar("2h el COF del heroe", h_cof, lineas(texto, "HITO238: COF ")[0])

    # el ORDEN de dibujo que pide el COF: el host lo imprime con los tipos, la consola solo cuantos son
    h_orden_tipos = re.search(r"orden=\[([^\]]*)\]", h_orden)
    if h_orden_tipos:
        verde("2h2: el orden de dibujo del COF (lo que dice el archivo): [%s]" % h_orden_tipos.group(1))

    # cada CAPA del personaje: bytes, crc32, caja y pixeles
    capas_host = {}
    for l in h_capas:
        m = re.search(r"VERIF238: CAPA (\w+) (.*)", l)
        if m:
            capas_host[m.group(1)] = limpiar(m.group(2))
    capas_con = []
    for l in lineas(texto, "HITO238: CAPA "):
        m = re.match(r"CAPA (\w+) (.*)", l)
        if m:
            capas_con.append((m.group(1), m.group(2)))
    faltan = []
    for clave, valores in capas_host.items():
        con = dict(capas_con).get(clave)
        if con is None:
            faltan.append(clave)
            continue
        if valores.startswith("SIN-GRAFICO"):
            if con.startswith("SIN-GRAFICO"):
                verde("2i capa %s: sin grafico en el MPQ (igual en el host y en la consola)" % clave)
            else:
                rojo("2i capa %s: el host dice SIN-GRAFICO y la consola dice %s" % (clave, con))
        elif valores in con:
            verde("2i capa %s: %s" % (clave, valores))
        else:
            rojo("2i capa %s: host '%s' contra consola '%s'" % (clave, valores, con))
    if faltan:
        rojo("2i: al personaje le faltan capas en la consola: %s" % ", ".join(faltan))

    comparar("2j el compuesto del personaje (tamaño, esquina, pixeles, crc32)", h_personaje,
             lineas(texto, "HITO238: PERSONAJE compuesto")[0])

    # ---- 2k: la geometria del area cliente, medida por el gestor de ventanas
    geo = re.search(r"GEO t=\d+s captura=(\S+) cliente=([\d,]+)", leer(corrida))
    ventana = geo.group(2) if geo else None
    if ventana:
        verde("2k: la geometria del area cliente la midio el gestor de ventanas: %s" % ventana)
    else:
        rojo("2k: la corrida no reporto la geometria del area cliente (sin linea GEO)")

    # ---- 2k2/2k3: la ventana del emulador estaba ARRIBA y CON EL FOCO, y el control llego ENTERO
    # (medido en esta tarjeta: sin foco las flechas llegan a medias y el personaje no se mueve en la direccion
    # pedida, asi que esto no es un detalle de comodidad: es la diferencia entre una corrida que mide y otra
    # que no)
    corrida_txt = leer(corrida)
    if "foco: foco=" in corrida_txt and "foco=no-pude" not in corrida_txt:
        verde("2k2: la corrida le pidio el foco a la ventana del emulador")
    else:
        rojo("2k2: la corrida no reporto el foco de la ventana del emulador: las teclas pueden llegar a medias")
    botones = set()
    for l in texto.splitlines():
        m = re.search(r"HITO238: CONTROL botones\s+(\d+)", l)
        if m:
            botones.add(int(m.group(1)))
    esperados = {128, 32, 144}
    if esperados <= botones:
        verde("2k3: el POC leyo las tres pulsaciones ENTERAS del control (derecha=128, abajo=32, "
              "arriba+derecha=144), como en DC-002.11")
    else:
        rojo("2k3: el POC no leyo las tres pulsaciones enteras (vio %s; faltan %s)" %
             (sorted(botones), sorted(esperados - botones)))

    # ---- 2l: la captura del personaje PARADO es la pantalla del host
    parado = os.path.join(run, "f00-parado.png")
    m_parado = medir(parado, ref, mapa, ventana)
    if m_parado["ident"] is None:
        rojo("2l: el medidor no pudo leer la captura del personaje parado:\n%s" % m_parado["bruto"])
    else:
        nota("[parado] identidad %.2f%% (la mejor de las otras tres %.2f%%), pixeles del personaje %s "
             "contra %s del host, centro %s contra %s" %
             (m_parado["ident"], m_parado["otra"], m_parado["px"], m_parado["px_host"],
              m_parado["centro"], m_parado["centro_host"]))
        if m_parado["ident"] >= 90.0:
            verde("2l: la pantalla del personaje parado ES la del host (identidad %.2f%% a 64x60)" %
                  m_parado["ident"])
        else:
            rojo("2l: la pantalla no coincide con la del host (identidad %.2f%%)" % m_parado["ident"])
        if m_parado["otra"] and m_parado["ident"] >= 2.0 * m_parado["otra"]:
            verde("2l2: y la identidad le gana por lejos a las otras tres orientaciones (%.2f%% contra "
                  "%.2f%%)" % (m_parado["ident"], m_parado["otra"]))
        else:
            rojo("2l2: la identidad no se despega de las otras orientaciones (%.2f%% contra %.2f%%)" %
                 (m_parado["ident"], m_parado["otra"] or -1))

    # ---- 2m: el personaje SE VE (los pixeles que la captura tiene de mas respecto del mapa solo)
    if m_parado["px"] and m_parado["px_host"]:
        h = m_parado["px_host"]
        if 0.5 * h <= m_parado["px"] <= 2.0 * h:
            verde("2m: el personaje SE VE en la captura: %d pixeles que no son el mapa (el host: %d)" %
                  (m_parado["px"], h))
        else:
            rojo("2m: los pixeles del personaje no dan (%d en la captura contra %d del host)" %
                 (m_parado["px"], h))
    else:
        rojo("2m: no hay pixeles de personaje en la captura (px=%s, host=%s)" %
             (m_parado["px"], m_parado["px_host"]))

    # ---- la serie de capturas (cada una con su marca del motor)
    serie = {}
    for nombre in ["f00-parado", "f03-derecha", "f05-abajo", "f08-diagonal", "f11-quieto"]:
        ruta = os.path.join(run, nombre + ".png")
        if not os.path.exists(ruta):
            continue
        m = medir(ruta, ref, mapa, ventana)
        serie[nombre] = m
        nota("[%s] identidad %.2f%% px=%s centro=%s" % (nombre, m["ident"] or -1, m["px"], m["centro"]))

    def centro(n):
        return serie.get(n, {}).get("centro")

    # ---- 2n: la PANTALLA se mueve como el MOTOR (con los numeros de las dos partes)
    #
    # Cada captura lleva la marca del motor que le toco (`pos=` de la linea GEO, el pixel de pantalla donde el
    # POC dibuja el ancla del personaje) y el md5 del archivo. Con eso:
    #   - el centro de masa del personaje medido en la captura (en el bloque del mapa, que es 1:1 con la
    #     pantalla logica menos 64 px de x) se convierte a pixel de pantalla: centro + 64;
    #   - el desplazamiento MEDIDO EN LA PANTALLA entre dos capturas se compara con el que dice el MOTOR entre
    #     esas mismas dos capturas: tienen que ir en la misma direccion y con el mismo tamaño (25 %);
    #   - si dos capturas salieron con el MISMO md5 y el motor se movio en el medio, la captura es del mismo
    #     cuadro (el compositor devolvio lo mismo): el tramo se DECLARA y NO se da por bueno, porque con una
    #     captura repetida no se puede decir nada del movimiento en la pantalla.
    geo_pos = {}
    md5s = {}
    for l in leer(corrida).splitlines():
        m = re.match(r"GEO t=\d+s captura=(\S+) cliente=[\d,]+\s+pos=([\d-]+),([\d-]+)", l)
        if m:
            geo_pos[m.group(1)] = (float(m.group(2)), float(m.group(3)))
        m2 = re.match(r"\s+captura \d+s\s+(\S+)\.png\s+md5=(\w+)", l)
        if m2:
            md5s[m2.group(1)] = m2.group(2)

    def pantalla(n):
        c = centro(n)
        return (c[0] + 64.0, c[1]) if c else None

    pares = [("f00-parado", "f03-derecha", "flecha DERECHA (x+)"),
             ("f03-derecha", "f05-abajo", "flecha ABAJO (y+)"),
             ("f05-abajo", "f08-diagonal", "diagonal ARRIBA+DERECHA (x+, y-)")]
    medidos = 0
    for a, b, que in pares:
        pa, pb = pantalla(a), pantalla(b)
        if not (pa and pb and a in geo_pos and b in geo_pos):
            rojo("2n: falta la captura o la marca del motor para el tramo %s" % que)
            continue
        px, py = pb[0] - pa[0], pb[1] - pa[1]
        ex, ey = geo_pos[b][0] - geo_pos[a][0], geo_pos[b][1] - geo_pos[a][1]
        repetida = md5s.get(a) and md5s.get(a) == md5s.get(b)
        nota("tramo %s: pantalla (%+.1f, %+.1f) contra motor (%+.1f, %+.1f)" % (que, px, py, ex, ey))
        if repetida and (abs(ex) + abs(ey)) > 2:
            nota("2n: %s: las dos capturas salieron IDENTICAS (mismo md5) y el motor si se movio: la captura "
                 "es del mismo cuadro (el compositor devolvio lo mismo), el tramo se declara y no se cuenta" % que)
            continue
        bien = (abs(ex) > 2 and abs(px - ex) <= max(6.0, 0.25 * abs(ex))) or \
               (abs(ey) > 2 and abs(py - ey) <= max(6.0, 0.25 * abs(ey)))
        if bien:
            medidos += 1
            verde("2n: %s: la pantalla se movio como el motor (%+.1f,%+.1f contra %+.1f,%+.1f px)" %
                  (que, px, py, ex, ey))
        elif abs(px) + abs(py) <= 4 and abs(ex) + abs(ey) > 10:
            # la pantalla NO se movio nada y el motor si: la captura es del mismo cuadro (el compositor
            # devolvio lo mismo). Es un defecto del INSTRUMENTO, no del POC: se declara y no se cuenta.
            nota("2n: %s: la captura NO cambio (+0,0 px) y el motor si se movio (%+.1f,%+.1f): el compositor "
                 "devolvio el mismo cuadro, el tramo se declara y no se cuenta" % (que, ex, ey))
        else:
            rojo("2n: %s: la pantalla NO se movio como el motor (%+.1f,%+.1f contra %+.1f,%+.1f px)" %
                 (que, px, py, ex, ey))
    if medidos == 0:
        rojo("2n: ningun tramo del movimiento se pudo medir en la pantalla")

    # ---- 2o: el avance es el del MOTOR (velocidad y proyeccion)
    pos = subprocess.run(["python3", os.path.join(HERE, "leer-pos-238.py"),
                          os.path.join(run, "flycast.log")], capture_output=True, text=True).stdout
    for l in pos.splitlines():
        nota(l)
    m = re.search(r"-> ([\d.]+) sub/s \(motor ([\d.]+), ([-+][\d.]+)%\)", pos)
    if m:
        vel, err = float(m.group(1)), float(m.group(3))
        if abs(vel - VELOCIDAD_DEL_MOTOR) <= 0.05 and abs(err) <= 1.0:
            verde("2o: la velocidad del personaje es LA DEL MOTOR: %.4f subtiles/s contra %.4f "
                  "(%s%%)" % (vel, VELOCIDAD_DEL_MOTOR, m.group(3)))
        else:
            rojo("2o: la velocidad no es la del motor: %.4f contra %.4f" % (vel, VELOCIDAD_DEL_MOTOR))
    else:
        rojo("2o: el POC no informo ninguna velocidad (no camino nunca?)")

    m = re.search(r"PROYECCION error maximo del ancla contra la proyeccion del piso: (\d+) px", pos)
    if m:
        if int(m.group(1)) <= 2:
            verde("2o2: el pixel con el que se dibuja el personaje es la proyeccion del piso de su posicion "
                  "del motor (error maximo %s px)" % m.group(1))
        else:
            rojo("2o2: el pixel del ancla no sigue la proyeccion del piso (error %s px)" % m.group(1))
    else:
        rojo("2o2: el POC no informo la proyeccion")

    m = re.search(r"TOTAL caminado=([\d.]+) subtiles.* dtCero=(\d+) dtGrande=(\d+)", pos)
    if m:
        verde("2o3: camino %s subtiles (%.2f celdas del mapa); cuadros con paso 0: %s, con paso acotado: %s"
              % (m.group(1), float(m.group(1)) / 5.0, m.group(2), m.group(3)))

    # ---- 2p: sin teclas, quieto (la ultima captura contra la de la diagonal)
    p8, p11 = pantalla("f08-diagonal"), pantalla("f11-quieto")
    if p8 and p11:
        if abs(p11[0] - p8[0]) <= 2 and abs(p11[1] - p8[1]) <= 2:
            verde("2p: sin teclas el personaje queda quieto (pantalla %s contra %s de la ultima captura con "
                  "teclas)" % (p11, p8))
        else:
            rojo("2p: el personaje se movio sin teclas (%s contra %s)" % (p11, p8))

    # ================================================================================================
    paso("3. EL MUTANTE (criterio 3): el control se lee y se informa, pero NO mueve")
    # ================================================================================================
    if os.environ.get("SALTAR_BUILD") != "1":
        log = os.path.join(SAL, "build-mut.txt")
        rc = correr(["bash", os.path.join(HERE, "build-personaje-238.sh"), BUILD_MUT],
                    log, env={"CONTROL_SORDO": "1"})
        if rc != 0:
            rojo("el build del MUTANTE fallo (ver %s)" % log)
            return 1
    if os.path.exists(os.path.join(BUILD_MUT, "prog.elf")) and os.path.exists(elf):
        h1 = subprocess.run(["sha256sum", os.path.join(elf)], capture_output=True, text=True).stdout.split()[0]
        h2 = subprocess.run(["sha256sum", os.path.join(BUILD_MUT, "prog.elf")], capture_output=True,
                            text=True).stdout.split()[0]
        if h1 != h2:
            verde("3a: el MUTANTE es un binario DISTINTO (%s contra %s)" % (h1[:16], h2[:16]))
        else:
            rojo("3a: el mutante es el MISMO binario que el fix")

    run_mut = os.path.join(SAL, "mut", "run")
    os.makedirs(run_mut, exist_ok=True)
    corrida_mut = os.path.join(SAL, "mut", "corrida.txt")
    if os.environ.get("SALTAR_CORRIDA") != "1":
        texto_mut, intentos_mut = correr_y_medir(BUILD_MUT, run_mut, corrida_mut)
        if intentos_mut == 1:
            verde("3: la corrida del MUTANTE salio entera al primer intento")
        else:
            nota("la corrida del MUTANTE necesito %d intentos" % intentos_mut)
    else:
        texto_mut = leer(os.path.join(run_mut, "flycast.log"))
    if not texto_mut:
        rojo("la corrida del MUTANTE no dejo log (ver %s)" % corrida_mut)
    else:
        if "HITO238: MUTANTE controlSordo=true" in texto_mut:
            verde("3a2: el log del MUTANTE dice que el control esta sordo")
        else:
            rojo("3a2: el log del MUTANTE no dice que el control este sordo (se compilo bien?)")

        posm = subprocess.run(["python3", os.path.join(HERE, "leer-pos-238.py"),
                               os.path.join(run_mut, "flycast.log")], capture_output=True, text=True).stdout
        for l in posm.splitlines():
            nota(l)
        m = re.search(r"TOTAL caminado=([\d.]+) subtiles .* pasos=(\d+)", posm)
        if m and float(m.group(1)) == 0.0 and int(m.group(2)) == 0:
            verde("3b: con las MISMAS teclas, el caminado del MUTANTE queda en 0 subtiles y 0 pasos")
        else:
            rojo("3b: el MUTANTE se movio igual (caminado=%s)" % (m.group(1) if m else "?"))

        controles = [l for l in posm.splitlines() if l.startswith("CONTROL")]
        con_botones = [l for l in controles if not re.search(r"botones +0 ", l)]
        if con_botones:
            verde("3c: y el control SI se leyo en el mutante: %d pulsaciones con botones != 0 (p.ej. %s)" %
                  (len(con_botones), con_botones[0].strip()))
        else:
            rojo("3c: el control no registro ninguna pulsacion en el mutante: el mutante no prueba nada")

        m_par_mut = medir(os.path.join(run_mut, "f00-parado.png"), ref, mapa, ventana)
        m_mov_mut = medir(os.path.join(run_mut, "f05-abajo.png"), ref, mapa, ventana)
        if m_par_mut["ident"] and m_mov_mut["ident"]:
            iguales = (abs(m_par_mut["ident"] - m_mov_mut["ident"]) < 0.3 and
                       m_par_mut["centro"] and m_mov_mut["centro"] and
                       abs(m_par_mut["centro"][0] - m_mov_mut["centro"][0]) <= 2 and
                       abs(m_par_mut["centro"][1] - m_mov_mut["centro"][1]) <= 2)
            if iguales:
                verde("3d: la pantalla del MUTANTE no cambia entre la captura parado y la de la tecla "
                      "(identidad %.2f%% contra %.2f%%, centro %s contra %s)" %
                      (m_par_mut["ident"], m_mov_mut["ident"], m_par_mut["centro"], m_mov_mut["centro"]))
            else:
                rojo("3d: la pantalla del MUTANTE cambio (identidad %.2f%% contra %.2f%%, centro %s contra "
                     "%s)" % (m_par_mut["ident"], m_mov_mut["ident"], m_par_mut["centro"], m_mov_mut["centro"]))
        else:
            rojo("3d: no pude medir las capturas del MUTANTE")

    # ================================================================================================
    paso("VEREDICTO")
    print("   %d controles OK, %d FALLOS" % (OK, FALLOS))
    return 1 if FALLOS else 0


if __name__ == "__main__":
    sys.exit(main())

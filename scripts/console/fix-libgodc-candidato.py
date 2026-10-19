#!/usr/bin/env python3
"""El BARRIDO CONSERVADOR tambien tiene que aceptar los punteros INTERIORES (DC-002.39b).

SINTOMA (medido, DC-002.39 y DC-002.39b)
    `runtime/gc_copy.c` es un recolector de COPIA: cuando mueve un objeto tiene que ACTUALIZAR todas las
    referencias que queden apuntandolo. Las raices que barre son las globales registradas por el
    compilador, las pilas de las goroutine y la pila del que recolecta (`gc_scan_stack`), y ahi el barrido
    es CONSERVADOR (`gc_scan_range_conservative`): mira palabra por palabra, sin saber que es cada una.

    Para no confundir basura con punteros, ese barrido pide DOS cosas antes de actualizar una palabra:

        gc_might_be_heap_ptr(v, bounds)   (¿la direccion CAE en el from-space?)
        gc_is_valid_object_start(v)       (¿es un COMIENZO de objeto valido?)

    y la segunda es la que rompe: `gc_is_valid_object_start` RECHAZA LOS INTERIORES por dos lados (lee el
    "encabezado" que hay 4 bytes mas abajo y exige alineacion a 8 bytes). Un valor que apunta AL MEDIO de
    otro objeto -- una cadena que es un PEDAZO de un buffer, como las que devuelve el decodificador de la
    configuracion y las que corta `strings.Fields` -- es Go legal, y DC-002.36 ya enseno al resolutor a
    copiar el objeto que lo CONTIENE (`gc_interior_base`); pero el barrido conservador descartaba al
    candidato ANTES de que llegara al resolutor, asi que esa palabra de la pila quedaba SIN ACTUALIZAR y el
    valor seguia apuntando al semiespacio abandonado, que despues se reusa. El largo de ese valor se lee de
    memoria que ya es de otro objeto: es el `largoNombre= -1795502946` del corpus de DC-002.33 y el `49`
    de DC-002.39.

MEDIDO CON EL POC CHIQUITO (`console/dc/poc/banco-vivo`, 30 s de Flycast)
    El POC arma un buffer grande y `cuantos` cadenas que son PEDAZOS suyos (punteros interiores), lo deja
    vivo solo por los pedazos y recorre el arreglo con `range`, haciendo reservas adentro (el trabajo del
    motor). En cada vuelta compara el valor del bucle contra el mismo elemento releido por indice:

        BANCO-VIVO-FALLO ronda= 1 i= 56 leidoPtr= 2350049232 leidoLen= 13 vivoPtr= 2350615040 vivoLen= 13
        BANCO-VIVO-TOTAL rondas= 40 comparaciones= 2560 fallos= 184 suma= 15974400

    y el analisis de las dos direcciones contra los semiespacios (que el runtime imprime como
    `ARQ-MARCA gc_init-semiespacio0/1`) dice que el viejo es SIEMPRE el valor del bucle: el que se lee por
    indice desde el arreglo (campo de puntero PRECISO, que no pasa por este portero) queda bien; el que se
    copia en la pila y se barre CONSERVADORAMENTE queda apuntando al semiespacio abandonado.

    OJO: NO es un registro. Antes de este arreglo se probo (y quedo en `fix-libgodc-bancos.py`) derramar
    los BANCOS del que llama (r8..r13) en la entrada de `gc_alloc()`/`runtime_GC()`: medido, los fallos
    siguen siendo EXACTAMENTE los mismos (184 de 2560, mismos indices). El valor puede estar en una ranura
    de la pila o en un banco: lo que lo pierde es el PORTERO, no donde viva.

EL ARREGLO
    En los 12 puntos del barrido conservador (`gc_scan_range_conservative`) el portero pasa a ser
    `gc_candidato_del_barrido()`, que solo exige que la direccion caiga en el from-space -- el mismo filtro
    barato que ya se usa -- y deja que el que decide sea `gc_relocate`/`gc_interior_base`: si ahi hay un
    comienzo de objeto lo copia, si es interior copia el objeto que lo CONTIENE y devuelve
    `nuevo_comienzo + desplazamiento`, y si no hay ningun encabezado que cierre no toca nada. El resolutor
    ya hacia las tres cosas (DC-002.36); lo unico que faltaba era dejarlo llegar.

LIMITE DECLARADO (el mismo que DC-002.36, y ahora tambien para la pila)
    El GC de esta consola es CONSERVADOR: una palabra de la pila que cae DENTRO de un objeto lo retiene
    (aunque no sea un puntero). Aceptar interiores amplia esa ambiguedad a las palabras que caen en
    cualquier byte del objeto en vez de solo en su comienzo. Es el comportamiento normal de un recolector
    conservador y es el precio de no perder valores vivos.

USO
    python3 scripts/console/fix-libgodc-candidato.py                 # aplica (idempotente)
    python3 scripts/console/fix-libgodc-candidato.py --build         # ademas rehace libgodc.a
    python3 scripts/console/fix-libgodc-candidato.py --sin-pristino  # encadenado
    python3 scripts/console/fix-libgodc-candidato.py --quitar --build # EL MUTANTE: lo saca
    python3 scripts/console/fix-libgodc-candidato.py --estado         # que dice el arbol (para el corpus)
    [ruta_libgodc]  (por defecto /home/dgalli/dreamcast/libgodc)

ORDEN E IDEMPOTENCIA
    Va DESPUES de `fix-libgodc-interior.py` (es su continuacion: el mismo archivo, y este arreglo se apoya
    en que `gc_interior_base` exista y camine bien) y despues de `fix-libgodc-sq.py`. Deja `gc_copy.c`
    pristino (`git checkout`) y rehace la cadena documentada: Store Queue -> punteros interiores ->
    MARCAS DEL ARRANQUE -> este arreglo. Correrlo dos veces deja el mismo sha256.
"""
import hashlib
import os
import pathlib
import re
import subprocess
import sys

QUITAR = "--quitar" in sys.argv
BUILD = "--build" in sys.argv
AQUI = os.path.dirname(os.path.abspath(__file__))
args = [a for a in sys.argv[1:] if not a.startswith("--")]
LIB = args[0] if args else "/home/dgalli/dreamcast/libgodc"

COPY = pathlib.Path(LIB + "/runtime/gc_copy.c")

MARCA = "DC-002.39b"

VIEJO = "gc_is_valid_object_start((void *)%s))"
NUEVO = "gc_candidato_del_barrido((void *)%s))"

DECLARACION = ("static bool gc_candidato_del_barrido(void *ptr);   /* %s: el barrido conservador acepta los"
               " INTERIORES */" % MARCA)
ANCLA_DEFINICION = "static bool gc_is_valid_object_start(void *ptr)\n{"
ANCLA_DECL = "static bool gc_is_valid_object_start(void *ptr);"

FUNCION = '''/* DC-002.39b: EL CANDIDATO QUE ACEPTA EL BARRIDO CONSERVADOR.
 *
 * `gc_is_valid_object_start` solo acepta COMIENZOS de objeto: rechaza los punteros INTERIORES por dos
 * lados (alineacion a 8 bytes y la validacion del "encabezado" que hay 4 bytes mas abajo). Un valor vivo
 * que apunta AL MEDIO de otro objeto -- una cadena que es un PEDAZO de un buffer, Go legal y resuelto por
 * `gc_interior_base` desde DC-002.36 -- quedaba descartado ANTES de llegar al resolutor, o sea que su
 * ranura de la pila no se actualizaba y seguia apuntando al semiespacio abandonado.
 *
 * Aca se pide lo que hay que pedir en un barrido conservador: que la direccion CAIGA en el from-space. El
 * que decide es `gc_update_pointer_field` -> `gc_relocate`: comienzo de objeto (lo copia), interior (copia
 * el objeto que lo CONTIENE y devuelve nuevo_comienzo + desplazamiento) o nada (no toca la palabra).
 *
 * Medido: POC `console/dc/poc/banco-vivo`, de 184 fallos de 2560 a 0.
 */
static bool gc_candidato_del_barrido(void *ptr)
{
    if (!ptr)
        return false;

    uintptr_t addr = (uintptr_t)ptr;
    uint8_t *espacio_viejo = gc_heap.space[1 - gc_heap.active_space];

    return addr >= (uintptr_t)espacio_viejo && addr < (uintptr_t)(espacio_viejo + gc_heap.space_size);
}

static bool gc_is_valid_object_start(void *ptr)'''


def sha(p):
    return hashlib.sha256(p.read_bytes()).hexdigest()[:16]


def cuantos(texto, que):
    # los 12 puntos del barrido conservador: 8x + 4x desenrollado + el bucle final. El unico
    # `gc_is_valid_object_start` que NO se toca es el del resolutor (`gc_relocate`, camino rapido).
    return len(re.findall(re.escape(que), texto))


def reemplazar(path, viejo, nuevo, esperados):
    # DC-002.25: la idempotencia se mide por el texto VIEJO (si ya no esta, no hay nada que hacer). OJO: no
    # sirve preguntar por el nuevo, porque en el sentido de `--quitar` el texto nuevo es una subcadena que
    # ya existe en el archivo y el script se quedaria sin hacer nada.
    t = path.read_text()
    n = t.count(viejo)
    if n == 0:
        print("   ya estaba (no habia %r) en %s" % (viejo[:44], path.name))
        return
    if n != esperados:
        sys.exit("ERROR: encontre %d ocurrencias de %r (esperaba %d) en %s" % (n, viejo, esperados, path))
    path.write_text(t.replace(viejo, nuevo))
    print("   ok %s: %d" % (path.name, n))


def insertar(path, ancla, texto):
    """Inserta `texto` (que arranca con el texto del ancla) si no esta YA. Hace falta aparte de
    `reemplazar` porque insertar es idempotente por el TEXTO NUEVO (el ancla sigue estando en el archivo
    despues de insertar, asi que preguntar por el viejo no sirve)."""
    t = path.read_text()
    if texto in t:
        print("   ya estaba (insertado)")
        return
    n = t.count(ancla)
    if n != 1:
        sys.exit("ERROR: el ancla aparece %d veces (esperaba 1) en %s: %r" % (n, path, ancla[:60]))
    path.write_text(t.replace(ancla, texto, 1))
    print("   ok %s: insertado" % path.name)


def estado():
    t = COPY.read_text() if COPY.exists() else ""
    aplicado = MARCA in t
    print("candidato-interior: %s en %s (sha256 %s)" % ("APLICADO" if aplicado else "QUITADO", COPY,
                                                        sha(COPY) if COPY.exists() else "-"))
    return aplicado


if "--estado" in sys.argv:
    print("== estado del arreglo del candidato interior (DC-002.39b)")
    sys.exit(0 if estado() else 1)

if "--sin-pristino" in sys.argv:
    print("== 1/3 sin tocar el arbol (encadenado: el arbol ya lo dejo listo quien llamo)")
else:
    print("== 1/3 el vendor pristino de gc_copy.c + la cadena documentada del runtime")
    subprocess.run(["git", "-C", LIB, "checkout", "--", "runtime/gc_copy.c"], check=True)
    print("   gc_copy.c: pristino (git checkout), sha256 %s" % sha(COPY))
    # La cadena: Store Queue -> punteros interiores (que a su vez encadena los dos). Se hace con
    # --sin-pristino porque el `git checkout` de arriba ya dejo el archivo pristino: pedirlo dos veces
    # borraria lo que el paso anterior escribio.
    r = subprocess.run([sys.executable, os.path.join(AQUI, "fix-libgodc-interior.py"), LIB, "--sin-pristino"],
                       capture_output=True, text=True)
    if r.returncode != 0:
        print(r.stdout + r.stderr)
        sys.exit("fix-libgodc-interior.py fallo")
    print("   " + (r.stdout.strip().splitlines() or ["(sin salida)"])[-1])
    # Las MARCAS DEL ARRANQUE tambien viven en gc_copy.c (`ARQ-MARCA gc_collect`): se reponen DESPUES del
    # ultimo paso que hace `git checkout` de este archivo, o el reseteo se las lleva.
    r = subprocess.run([sys.executable, os.path.join(AQUI, "fix-libgodc-marcas-arranque.py"), LIB],
                       capture_output=True, text=True)
    if r.returncode != 0:
        print(r.stdout + r.stderr)
        sys.exit("fix-libgodc-marcas-arranque.py fallo")
    print("   " + (r.stdout.strip().splitlines() or ["(sin salida)"])[-1])

print("== 2/3 el candidato del barrido conservador")
# los 12 puntos: 8x desenrollado (v0..v7), 4x de limpieza (v0..v3) y el bucle final (v). El unico
# `gc_is_valid_object_start` que NO se toca es el del resolutor (`gc_relocate`, camino rapido).
PUNTOS = (("v0", 2), ("v1", 2), ("v2", 2), ("v3", 2), ("v4", 1), ("v5", 1), ("v6", 1), ("v7", 1))

if QUITAR:
    print("   --quitar: NO se aplica (el arbol queda con la cadena canonica y sin este arreglo: es el MUTANTE)")
    for v, n in PUNTOS:
        reemplazar(COPY, NUEVO % v, VIEJO % v, n)
    # el bucle final: el ancla es la linea completa, porque `v` tambien la usa el resolutor
    reemplazar(COPY, "gc_candidato_del_barrido((void *)v))\n            gc_update_pointer_field(p);",
               "gc_is_valid_object_start((void *)v))\n            gc_update_pointer_field(p);", 1)
    reemplazar(COPY, ANCLA_DECL + "\n" + DECLARACION, ANCLA_DECL, 1)
    reemplazar(COPY, FUNCION + "\n{", ANCLA_DEFINICION, 1)
    if MARCA in COPY.read_text() or "gc_candidato_del_barrido" in COPY.read_text():
        sys.exit("ERROR: quedaron restos del arreglo en %s" % COPY)
else:
    for v, n in PUNTOS:
        reemplazar(COPY, VIEJO % v, NUEVO % v, n)
    reemplazar(COPY, "gc_is_valid_object_start((void *)v))\n            gc_update_pointer_field(p);",
               "gc_candidato_del_barrido((void *)v))\n            gc_update_pointer_field(p);", 1)
    insertar(COPY, ANCLA_DECL, ANCLA_DECL + "\n" + DECLARACION)
    insertar(COPY, ANCLA_DEFINICION, FUNCION + "\n{")
    t = COPY.read_text()
    llamadas = t.count("gc_candidato_del_barrido((void *)")
    if llamadas != 13:   # 8x + 4x desenrollado (v0..v7, v0..v3) + el bucle final (v)
        sys.exit("ERROR: esperaba 13 llamadas al candidato y hay %d" % llamadas)
    if t.count(MARCA) != 2:   # la declaracion adelantada y el comentario de la definicion
        sys.exit("ERROR: menciones de %s: %d (esperaba 2)" % (MARCA, t.count(MARCA)))

estado()

print("== 3/3 el runtime")
if BUILD:
    subprocess.run(["bash", "-c", "rm -f %s/runtime/*.o %s/*.a" % (LIB, LIB)], check=True)
    r = subprocess.run(["make", "-C", LIB, "all"], capture_output=True, text=True)
    if r.returncode != 0:
        print(r.stdout[-4000:] + r.stderr[-4000:])
        sys.exit("make fallo")
    print("   make ok: " + (r.stdout.strip().splitlines() or ["(sin salida)"])[-1])
    print("   %s/libgodc.a: %d bytes, sha256 %s" % (LIB, os.path.getsize(LIB + "/libgodc.a"),
                                                    sha(pathlib.Path(LIB + "/libgodc.a"))))
else:
    print("   falta: rm -f %s/runtime/*.o %s/*.a && make -C %s all   (o correr con --build)" % (LIB, LIB, LIB))
print("   ahora: reenlazar el POC (`build-dc.sh`) y correr Flycast; el numero es")
print("          `BANCO-VIVO-TOTAL ... fallos= 0` en la consola serie")

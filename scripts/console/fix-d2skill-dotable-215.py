#!/usr/bin/env python3
"""DC-002.15: crea doTable en la INICIALIZACION de la variable, no en un init() de archivo.

Medido: en la consola el arranque paniquea con "assignment to entry in nil map" desde
d2common/d2skill..init0 en class_abc.go:17 (pc de retorno 8c148c14, resuelto con addr2line). class.go
creaba doTable dentro de SU init() y class_abc.go escribia en el mapa desde el suyo: eso depende de que
los init() de un paquete corran en orden de nombre de archivo, que es comportamiento de la herramienta de
Go y no una garantia del lenguaje (el spec solo garantiza que la inicializacion de variables termina
ANTES de cualquier init()). gccgo corre primero el init de class_abc.go y el mapa todavia es nil.

El arreglo no reordena nada: el mapa pasa a crearse como cualquier otra variable del paquete, y con eso
cualquier init() -- de este archivo o de otro -- ya lo encuentra armado.
"""
import pathlib
import subprocess
import sys

RUTA = pathlib.Path(sys.argv[1] if len(sys.argv) > 1 else
                    "/home/dgalli/Proyectos/diablo2consoles/d2common/d2skill/class.go")
lineas = RUTA.read_text().splitlines(keepends=True)

# Anclas (medidas sobre el archivo de hoy): la declaracion, el init y el cierre del mapa.
i_var = next(i for i, l in enumerate(lineas) if l.startswith("var doTable map[int]doFn"))
i_init = next(i for i, l in enumerate(lineas) if l.startswith("func init() {"))
i_mapa = next(i for i, l in enumerate(lineas) if l.startswith("\tdoTable = map[int]doFn{"))
i_mapa_fin = next(i for i, l in enumerate(lineas) if i > i_mapa and l == "\t}\n")
i_init_fin = i_mapa_fin + 1
if lineas[i_init_fin] != "}\n":
    sys.exit("no cierra el init() donde esperaba: %r" % lineas[i_init_fin])

COMENTARIO = (
    "// DC-002.15: el mapa se crea ACA, en la inicializacion de la variable, y no dentro de un init(). El\n"
    "// spec garantiza que la inicializacion de las variables del paquete termina ANTES de que corra\n"
    "// cualquier init(); con el mapa creado en el init() de este archivo, class_abc.go (que lo usa desde\n"
    "// el suyo) dependia de que los init() de un paquete corran en orden de nombre de archivo, que es\n"
    "// comportamiento de la herramienta de Go y no una garantia del lenguaje. Medido en gccgo: corria\n"
    "// primero el init de class_abc.go y el mapa todavia era nil (panic \"assignment to entry in nil map\",\n"
    "// class_abc.go:17).\n"
)
entradas = ["\t" + l[1:] if l.startswith("\t") else l for l in lineas[i_mapa + 1:i_mapa_fin]]

nuevas = lineas[:i_var] + [COMENTARIO, "var doTable = map[int]doFn{\n"] + entradas + ["}\n"] + lineas[i_init_fin + 1:]
RUTA.write_text("".join(nuevas))
print(f"{RUTA}: {len(lineas)} -> {len(nuevas)} lineas (init() del mapa: {i_init_fin - i_init + 1} lineas menos)")
print(subprocess.run(["gofmt", "-l", str(RUTA)], capture_output=True, text=True).stdout or "gofmt: limpio")

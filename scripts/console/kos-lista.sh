# La lista de paquetes de la stdlib que hablan con KOS (C de la consola).
#
# POR QUE ES UN ARCHIVO APARTE: la necesitan DOS scripts (test-std-host.sh para excluirlos del test del
# host, y mutar-std-host.sh para armar la misma lista de paquetes a probar), y cuando cada uno tenia su
# copia, una quedo vieja -- medido: el mutador quedo con "EXCLUIDOS: variable sin asignar", corrio los 34
# mutantes contra NADA y reporto 1/34 cazados. Un mutador que mide mal es peor que no tenerlo, porque da
# una respuesta que parece un dato.
#
# Se carga con:  source "$HERE/kos-lista.sh"   (o el camino donde este)
#
# QUE SIGNIFICA: estos paquetes NO compilan en el host (sus funciones son //extern a C de la consola), asi
# que se excluyen del test del host y se prueban en la CONSOLA (console/dc/poc/vfs y los mutadores
# mutar-os-consola.sh).

KOS="os time log runtime sync/atomic compress/zlib compress/gzip io/fs io/ioutil os/signal embed net context"
# El patron de go list: d2std/os$ o d2std/time$ etc. Se ancla al final para que d2std/os/user NO quede
# excluido por d2std/os.
EXCLUIDOS="d2std/($(echo "$KOS" | tr ' ' '|'))\$"

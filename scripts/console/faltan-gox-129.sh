#!/bin/bash
# que paquetes del cierre del POC no tienen .gox en el arbol del motor
GOX="${1:-/tmp/gox-296}"
LISTA="${2:-/tmp/deps129.txt}"
while read -r p; do
    [ -z "$p" ] && continue
    if [ ! -f "$GOX/github.com/OpenDiablo2/OpenDiablo2/$p.gox" ]; then
        echo "FALTA $p"
    fi
done < "$LISTA"
echo "fin"

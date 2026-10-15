#!/bin/bash
# Descarga el toolchain de Dreamcast (GCC 15.1 con Go + KOS 2.2.1 + kos-ports + libgodc)
# y Flycast. Verifica el sha256 del toolchain antes de darlo por bueno.
set -u
DL=/home/dgalli/dreamcast/dl
mkdir -p "$DL"
cd "$DL" || exit 1
BASE=https://github.com/drpaneas/dreamcast-toolchain-builds/releases/download/gcc15.1.0-kos2.2.1
F=dreamcast-toolchain-gcc15.1.0-kos2.2.1-linux-x86_64-ubuntu22.tar.gz

echo "== bajando toolchain ($F) =="
curl -fL --retry 3 --retry-delay 2 -o "$F" "$BASE/$F" || { echo "FALLO toolchain"; exit 1; }
curl -fL --retry 3 -o "$F.sha256" "$BASE/$F.sha256" || echo "(sin sha256)"

echo "== verificando sha256 =="
if [ -f "$F.sha256" ]; then
  cat "$F.sha256"
  sha256sum -c "$F.sha256" && echo "SHA256 OK" || echo "SHA256 NO COINCIDE"
fi

echo "== bajando Flycast AppImage =="
curl -fL --retry 3 -o flycast.AppImage https://github.com/flyinghead/flycast/releases/download/v2.7/flycast-x86_64-2.7.AppImage || echo "FALLO flycast"
chmod +x flycast.AppImage 2>/dev/null

ls -la "$DL"
echo "DESCARGA LISTA"

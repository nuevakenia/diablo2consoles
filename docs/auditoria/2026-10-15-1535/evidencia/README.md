# Evidencia cruda

Todo lo de esta carpeta salió de correr el motor del repo en este host (CachyOS x86-64) el 2026-10-15.
Los MPQ usados son los del dueño: `/home/dgalli/.wine-d2/drive_c/Games/Diablo II/` (11 archivos, 1,8 GB).
Nada de acá toca el repo del juego ni modifica archivos: el motor lee los MPQ y escribe solo en su carpeta de
configuración (`OD2_CONFIG_DIR`).

## Archivos

| Archivo | Qué es | Cómo se regenera |
|---|---|---|
| `run_measure2.sh` | corre un nivel, mide `VmHWM` del proceso `od2` (no del `timeout`) y saca screenshot | `./run_measure2.sh 2 12 moor` |
| `run_steady.sh` | deja el héroe quieto, muestrea `VmRSS`/`VmHWM` cada 0,5 s y deja que `OD2_AUTOPERF` escriba el heap profile | `./run_steady.sh 1 35` |
| `perf-host.txt` | líneas `PERF`/`Perf mark` de las dos corridas (pueblo y Blood Moor) | ver los scripts |
| `binario-y-memoria.txt` | `size -A` del binario sin strip + tamaños de los binarios + resumen de RSS | `go build -ldflags="-s -w" -o od2 . && size -A od2` |
| `pprof-inuse-moor.txt` | `go tool pprof -inuse_space -top` sobre el heap del Blood Moor (25 nodos) | `go tool pprof -inuse_space -top od2 heap.pprof` |
| `pprof-alloc-moor.txt` | `go tool pprof -alloc_space -top` (churn) del Blood Moor | `go tool pprof -alloc_space -top od2 heap.pprof` |
| `pprof-inuse-town.txt` | ídem, pueblo | ídem |
| `nm-por-subsistema.txt` | tamaño de símbolos agrupado por subsistema del motor | `go tool nm -size od2` |
| `medicion-arte-mpq.txt` | volumen descomprimido de `d2data`, `patch_D2`, `d2exp` y `d2char` por categoría y extensión | ver `medidor-mpq/` (abajo) |
| `shot-town.png` | pantalla del pueblo (nivel 1), 2560×1052, generada por `OD2_AUTOSHOT` | `./run_measure2.sh 1 12 town` |
| `shot-moor.png` | pantalla de Blood Moor (nivel 2), 1024×688 | `./run_measure2.sh 2 12 moor` |

Resumen de los cuatro MPQ medidos: **11.082 + 162 + 9.817 + 10.009 archivos**, **898,7 MB descomprimidos**
(343,7 + 7,8 + 291,6 + 255,6). Por tipo de arte manda **DCC (animaciones) con 573,0 MB**, después **DT1 (tiles)
150,7 MB**, **DC6 (UI e interfaz) 95,1 MB** y **DS1 (mapas) 22,1 MB**. Los fallidos (271 + 6 + 50 + 0) son
formatos de audio/video Bink que el lector de MPQ del motor no descomprime.

## Variables de entorno usadas (las del harness del propio repo)

```
OD2_CONFIG_DIR=<carpeta con config.json y Saves/0.od2 del repo>
OD2_AUTOGAME=<carpeta>/Saves/0.od2      # arranca directo ese personaje
OD2_AUTOLEVEL=1|2                       # pueblo | Blood Moor
OD2_AUTOPERF=1 OD2_AUTOPERF_SECONDS=10 OD2_AUTOPERF_WARMUP=4
OD2_PPROF_HEAP=<archivo.pprof>
OD2_AUTOSHOT=<archivo.png> OD2_AUTOSHOT_SECONDS=10
OD2_NO_SETUP=1 OD2_AUTOTEST_MUTE=1 OD2_AUTOEXIT=1
```

`config.json` necesita `MpqPath` apuntando a la carpeta con los MPQ. El save de prueba usado es el que ya viene
en el repo (`.verifycfg/Saves/0.od2`): no se usó ningún `.d2s` del dueño.

## Verificación de las capturas (que no es un pantallazo negro)

Decodifiqué los PNG a RGB crudo y medí su contenido (`ffmpeg -i shot.png -f rawvideo -pix_fmt rgb24`):

```
moor: 704.512 px · 2.750 colores distintos · luminancia media 32,4 · 15,7% casi negros
town: 2.693.120 px · 4.950 colores distintos · luminancia media 33,4 · 12,4% casi negros
```

O sea: escenas renderizadas de verdad (miles de colores, escena oscura propia del juego), no un patrón de prueba
ni una pantalla en negro. Un modelo de visión al que le mostré `shot-moor.png` describió **terreno isométrico,
personajes, antorchas, un puente y la barra inferior con los orbes de vida/maná, el cinturón de pociones y los
iconos de skills** — es la escena del juego con su UI. El mismo modelo marcó fallas de iluminación, sprites
borrosos y una UI que "no coincide exactamente" con la original: **eso coincide con lo que el propio repo admite**
en `docs/KNOWN_GAPS.md` (el mapa de luz y el tinte día/noche son aproximados; el layout de las pantallas se movió
a los números del original pero no se verificó al lado del original). Es una aproximación del motor, no un fallo
del port: cualquier cosa que se vea rara en la consola va a venir de acá.

## Medidor de MPQ (para el informe 04, F3)

Programa propio, fuera del repo (en el scratch de la sesión), con `replace github.com/OpenDiablo2/OpenDiablo2 =>
/home/dgalli/Proyectos/diablo2consoles`: abre cada MPQ con `d2mpq.FromFile`, recorre `Listfile()` y suma
`len(ReadFile(...))` por categoría y por extensión. La salida está en `medicion-arte-mpq.txt`.
Nota: los archivos que fallan la lectura (271 en `d2data`, 50 en `d2exp`, 6 en `patch_D2`) no se cuentan; son
formats de audio/video Bink que el lector de MPQ del motor no descomprime.

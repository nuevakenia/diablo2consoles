ESTADO DE ESTE POC: NO COMPILA (le falta el horneado del asset)

Medido, no supuesto:

    $ bash scripts/console/build-dc.sh console/dc/poc/motor-pvr /tmp/dcbuild/motor-pvr-check
    console/dc/poc/motor-pvr/main.go:70:9: error: range clause must have array, slice, string, map, or
    channel type
    (y antes de eso, el import de console/dc/motorresumen no se resuelve)

LAS DOS COSAS QUE FALTAN

1. EL ASSET CRUDO. main.go usa dos variables de Go (`assetDC6` y `assetPAL`) que deberian tener los bytes
   del .dc6 y del .pal tal cual salen del MPQ -- para que los decodifique el MOTOR adentro de la consola
   (d2dc6.Load + d2dat.Load). Pero el horneador (scripts/console/bake-asset.sh) NO las genera: lee
   `bake.conf` y lo que escribe es un header de C con los indices YA decodificados y la paleta ya armada.
   Este POC tiene un archivo `asset.conf`, y NINGUN script lee ese nombre. Por eso las dos variables no
   existen.

2. EL PAQUETE `motorresumen`. Es un paquete de la consola (console/dc/motorresumen) que este POC importa y
   que no esta en el arbol .gox: no es de terceros (los que compila build-std-gox.sh) ni del motor (los que
   compila build-motor-all.sh). Habria que compilarlo DESPUES del motor, porque depende de el (d2dc6,
   d2interface).

   El horneado del `.h` no cubre este caso: motorresumen.LineasRGBA existe justamente para comparar el
   color que da el motor contra el que da el host, y para eso hacen falta los bytes crudos.

EL CAMINO CORTO PARA ARREGLARLO

- Generar `assetDC6`/`assetPAL` como Go desde el horneado (el horneador ya tiene los bytes a mano: hoy los
  decodifica el, hay que pedirle que ademas los escriba crudos como un archivo .go), y renombrar
  `asset.conf` a `bake.conf` (o ensenarle a bake-asset.sh a leer los dos nombres).
- Compilar motorresumen al .gox al final de build-motor-all.sh.

MIENTRAS TANTO

El POC `motor-pantalla` (DC-002.10) SI compila, corre y esta verificado en Flycast: dibuja un asset del
motor por el camino del motor (imagen -> DrawImage -> pantalla -> VRAM) y el logo se mueve. Lo que hay que
mirar para ver el motor pintando es ese.

ESTADO DE ESTE POC: COMPILA, CORRE Y DA LO MISMO QUE EL HOST (medido en DC-002.12)

Este archivo decia "NO COMPILA (le falta el horneado del asset)" y ese diagnostico era FALSO: lo que
fallaba era el SCRIPT con el que se lo intento armar. Se reescribe con lo medido, porque el error que
sale cuando se usa el script equivocado es el mismo de antes y va a volver a aparecer.

LO QUE DECIA, Y POR QUE ESTABA MAL

  Decia: "bash scripts/console/build-dc.sh console/dc/poc/motor-pvr" fallaba (import de motorresumen sin
  resolver y "range clause must have array, slice, string, map, or channel type"), y de ahi concluia dos
  cosas: que el horneador no genera los bytes crudos (`assetDC6`/`assetPAL`) y que motorresumen no esta
  en el arbol .gox. Las dos conclusiones son falsas, y lo que falto fue mirar el camino que el POC ya
  tenia:

  1. ESTE POC NO SE ARMA CON build-dc.sh. Tiene su propio build desde DC-002.9, y los bytes crudos los
     hornea `console/dc/bake-dc6` (que los escribe como var de Go, package main) — no `bake-asset.sh`,
     que es el que escribe el `.h` con los indices ya decodificados. El nombre `asset.conf` SI lo lee un
     script: `build-motor-dc6.sh` (y por eso `build-motor-pvr.sh`, que lo llama).
  2. `motorresumen` SI estaba en el .gox: esta en la lista PAQUETES de `build-motor-gox.sh` desde
     DC-002.9, y despues de un build queda en el arbol
     (`<gox>/github.com/OpenDiablo2/OpenDiablo2/console/dc/motorresumen.gox`, 23.760 bytes).

EL CAMINO CORRECTO, Y LO QUE DA (medido: 2026-10-17)

    $ bash scripts/console/build-motor-pvr.sh /tmp/dcbuild/motor-pvr
      (hornea el DC6 crudo 140.740 B + el pal.dat crudo 768 B, compila la stdlib minima y los 11
       paquetes del motor para SH-4, y arma el ELF)
      ELF: /tmp/dcbuild/motor-pvr/prog.elf   7.198.516 bytes
      1ST_READ.BIN                           1.300.936 bytes

    $ bash scripts/console/compare-motor-pvr.sh
      == IGUAL: 61 lineas identicas (mismo DC6, mismo codigo del motor, x86-64 y SH-4)
         frames comparados: 30
      pixeles de arte:        4051/4051 (100.0%) iguales (tolerancia +/-8)
      pixeles transparentes:  2085/2085 (100.0%) con el fondo visible
      identidad 100.0% (espejada horizontal 17.5%, vertical 16.7%, rotada 180 11.9%: no hay volteo)
      RESULTADO: OK        (18,58 s el ciclo entero, con el emulador adentro)

EL DEFECTO DE VERDAD (y su arreglo)

  No era del POC: era de build-dc.sh, que no decia nada y dejaba que el compilador contestara
  "reference to undefined name 'assetDC6'", un error que manda a buscar el problema donde no esta (fue
  exactamente lo que paso aca: se escribio este archivo culpando al horneador). Ahora build-dc.sh AVISA
  antes de compilar y con el nombre del script correcto (exit 2) cuando el POC trae `asset.conf`;
  `build-motor-dc6.sh` se lo saltea con POC_ASSET_CRUDO=1. La prueba: `scripts/console/checar-poc-crudo.sh`
  (control sano que exige el aviso + el mutante que lo saca y exige que vuelva el error viejo: 1/1 cazado).

MIENTRAS TANTO (esto si quedo cierto)

  El POC `motor-pantalla` (DC-002.10) y este `motor-pvr` (DC-002.9) son los dos que pintan en la consola:
  motor-pantalla dibuja por el camino del motor (imagen -> DrawImage -> pantalla -> VRAM) y motor-pvr
  dibuja el frame decodificado como textura PAL8. Este ultimo es el que compara pixel a pixel contra la
  referencia del host.

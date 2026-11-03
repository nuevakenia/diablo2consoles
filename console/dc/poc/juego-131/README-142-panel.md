# DC-002.142 — el panel de los NPCs abría y no pintaba una letra: los glifos se pedían cuando la fuente ya estaba soltada

El panel de diálogo de Akara, en la demo jugable de DC-002.140, se abría y **salia negro con su borde**. El log
parecía decir que se pintaba (`pixeles= 140976`) y ese número era justamente la trampa: `356 * 396` es el **área del
fondo**, no la del texto. Se estaba midiendo que el rectángulo se había pintado, no que las letras hubieran llegado
a la pantalla.

## La causa, medida

Los glifos de `font16` se cargaban la **primera vez que se abría el panel**, y el panel se abre con el botón del
jugador: cuadro 2 en la corrida del criterio, cuadro 39 en la demo. A esa altura el POC ya soltó `D2DATA.MPQ`
(`Fuente.Soltar`, DC-002.136, que es lo que deja RAM para la fase del pueblo) y `Pedir` deja de encontrar el
archivo. El log del build instrumentado lo dice sin adornos:

```
JUEGO142: GLIFOS PANEL SIN TABLA ruta= data\local\font\latin\font16.tbl err= true
JUEGO142: PANEL TEXTO nombre= Akara caja= 356 x 396 ... letrasPedidas= 729 glifosPintados= 0 sinGlifo= 729
```

729 letras pedidas, 0 glifos pintados, 729 sin glifo. El fondo del panel sí estaba armado en RAM (los índices se
copian al montón de C cuando se compone el pueblo), y por eso el rectángulo salía y las letras no.

### La hipótesis que se cayó

La primera sospecha fue la del pincel: que la tinta del glifo fuera **índice 0** y `RenderIndicesTintado`, que salta
el 0 como transparente, se la comiera. La sonda `console/dc/sonda-tinta-142` la desmiente con los números del
`font16`:

```
"A" cuadro=65 caja=14x16 paso=12 pixeles=224 vacios=162 llenos=62
     indice   0 x162  color=(0,0,0)      <- el fondo, eso es lo que el pincel salta (y está bien)
     indice  19 x12   color=(44,44,44)   <- la tinta
     indice  21 x5    color=(56,56,56)
     indice  22 x7    color=(72,72,72)
     indice  25 x1    color=(88,88,88)
     indice  28 x1    color=(112,116,124)
```

La letra "A" tiene 62 píxeles de tinta en índices 19, 21, 22, 25 y 28. El pincel estaba bien; lo que faltaba eran
los glifos. Y el `font16.DC6` de la consola es el mismo del host: 35.214 B, crc32 3088877326 en los dos lados.

## El arreglo

**Lo que va a hacer falta para dibujar se pide en la CARGA, no en el dibujo.** El HUD ya lo hacía así (sus glifos
se cargan antes de soltar la fuente) y por eso el HUD nunca tuvo el panel en negro.

- `cargarLosGlifosDelPanel()`, nueva, se llama al final de `cargarLosNPCs` — el último momento en que la fuente
  tiene su MPQ. Pide las letras de **todos** los textos del panel: los cuatro NPCs (nombre, renglones y filas), los
  textos de la quest si está encendida, y **los 95 imprimibles ASCII**, para que un texto que no sea de ningún NPC
  (números, estados de la quest, paréntesis) tampoco se quede sin letra hoy ni cuando se agregue otro.
- `cargarLosGlifosDelNPC()` queda como red de seguridad para el caso de un NPC que aparezca después: si no falta
  ninguna letra no toca la fuente (que a esta altura ya no tiene MPQ y solo puede fallar), y si falta alguna lo dice
  en el log con las letras exactas (`armarElPanelDelDialogo` informa `PANEL FALTAN LETRAS ...`).

## Los números (Flycast, `JUEGO142: PANEL TEXTO`)

Tres paneles distintos, cada uno medido una sola vez con el barrido del área sobre el lienzo: píxeles no negros
**después del fondo** y otra vez **después de los glifos**. La diferencia es la tinta que llegó a la pantalla.

| panel | caja | letras pedidas | glifos pintados | sin glifo | tinta de los glifos | tinta que llegó al lienzo | ms |
|---|---|---|---|---|---|---|---|
| diálogo de Akara | 371x396 | 729 | 729 | **0** | 28.704 | **28.704** | 114 |
| quest (al tomarla) | 501x81 | 111 | 111 | **0** | 4.627 | **4.627** | 25 |
| quest (al volver) | 288x81 | 84 | 84 | **0** | 3.622 | **3.622** | 16 |

La tinta que el pincel debía poner y la que cambió el lienzo coinciden **exactamente** en los tres. El crc32 del
área también cambia en los tres (`elAreaCambio= true`), que es el segundo testigo.

El ancho del panel de Akara pasó de 356 a **371**: el panel se mide con el avance real de cada letra de `font16` y
con los glifos cargados esa medida es la del motor. Antes medía con el promedio de 8 px por letra.

## Lo que cuesta

| | |
|---|---|
| glifos | 95 (94 con tinta; el vacío es el espacio) |
| índices | 21.280 B (224 B por letra, 14x16) |
| montón de C | 22.040 B (`montonC_antes= 703480` → `montonC_despues= 681440`) |
| sprite leído y devuelto en el acto | 35.214 B + tabla 3.596 B |

22 KB en el montón de C, tomados **una vez** y antes de que se suelte la fuente. Para comparar: la fuente entera
son 57.344 B de índices y el panel de Akara ya ocupa 146.916 B en el montón de Go.

## El criterio, en una línea

```
JUEGO142: PANEL TEXTO CRITERIO glifosConTinta= true seMidioUnPanel= true laTintaLlegoAlLienzo= true elAreaCambio= true cumple= true
JUEGO127: NPC CRITERIO losNPCsSeDibujan= true seLeHablo= true elDialogoSaleEnPantalla= true elPanelCambiaElLienzo= true conElPanelCerradoNoCambia= true elDialogoSeCerro= true cumple= true
```

La segunda línea estaba en `cumple= false` **siempre** desde el port a `juego-131`: `cuadrosAbiertosConCambio` y
`cuadrosCerradosSinCambio` estaban declarados y leídos pero nadie los incrementaba (verificado en `HEAD`). Ahora los
incrementa esta misma medida: el panel abierto cambia el crc del área, y el cuadro con el panel cerrado se cuenta
sin pintar. De paso, el crc32 de 1,2 MB que se sacaba en **cada** cuadro se saca una sola vez.

## Cómo se reproduce

```bash
# el build del criterio (sin quest, para que el panel del NPC no lo pise el de la quest)
DEMO=no HABLAR_AUTO=si CERRAR_DIALOGO=no QUEST=no NPC=si NPC_LISTA=akara \
  VISTA_X=38.4 VISTA_Y=4.4 CUADROS=10 MEDIR_VIVO=si \
  bash scripts/console/build-juego-131.sh /tmp/dcbuild/t142e
CAPTURAS=10 INTERVALO=3 bash scripts/console/run-flycast-131.sh /tmp/dcbuild/t142e/disc.gdi /tmp/capturas-t142e
grep -a "JUEGO142" /tmp/capturas-t142e/flycast.log | tr -d '\r'

# la demo de DC-002.140 (con el instrumento de hablar encendido, que es el único cambio)
bash scripts/console/build-demo-140.sh /tmp/dcbuild/demo142     # el binario que juega Daniel, igual que siempre
```

La evidencia de la corrida está en `console/dc/evidencia/dc-002.142-texto-del-panel.txt` y las capturas en
`console/dc/evidencia/dc-002.142-panel-akara-con-texto.png` (panel del diálogo, 21 renglones) y
`console/dc/evidencia/dc-002.142-demo-panel-akara.png` (el binario de la demo).

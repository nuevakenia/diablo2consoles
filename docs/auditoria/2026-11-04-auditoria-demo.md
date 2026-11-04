# Auditoría de la demo jugable — 2026-11-04

Qué se auditó: el estado real de la demo que Daniel jugó (DC-002.145/143/147), el flujo de trabajo que la
produce, y las causas raíz de los defectos que reportó al jugarla.

Método: medición, no opinión. Cada número de este documento salió de una corrida con marca de tiempo, de un
`grep` sobre el log de la consola serie, o de leer el código que se está ejecutando. Donde no hay medición, se
dice «sin medir» y queda como deuda.

---

## 1. El flujo de trabajo, medido

| etapa | tiempo | cómo se midió |
|---|---|---|
| **build incremental** (tocar 1 archivo del POC, con los `.gox` del motor ya hechos) | **49 s** | `touch main.go` + `build-demo-140.sh`, cronómetro alrededor |
| build completo del motor (145 paquetes) | 6-10 min | cuando cambia el motor; el script lo detecta por fecha |
| **arranque al primer cuadro** | **93 s** | reloj desde el lanzamiento hasta la primera línea `DEMO140: FPS` |
| cuadro dibujado | 20-31 ms (32-50 fps) | `DEMO140: FPS` |
| prueba manual completa | 2-3 min | build 49 s + arranque 93 s |

**Conclusión del flujo:** el build NO es el problema. 49 s es un ciclo de iteración perfectamente usable. El
problema es el **arranque de 93 s**, que es el 65 % del ciclo y además es lo que hace que la consola real tarde
una eternidad en mostrar algo.

Y no hace falta hot reload: con 49 s de build, cualquier idea de «cargar código en caliente» agrega complejidad
para ahorrar menos de un minuto. Lo que hay que bajar es el arranque.

### Desglose del arranque (86 s de carga, medidos con el hito de `JUEGO136: CARGA`)

| etapa | tiempo | qué hace |
|---|---|---|
| arranque de KOS | 3,3 s | el sistema operativo de la consola |
| **las fuentes** | **12,3 s** | levanta TODAS las fuentes del HUD |
| las tablas | 0,7 s | |
| **el jugador** | **20,4 s** | compone sus 8 direcciones × 13 cuadros de entrada |
| **componer el nivel** | **23,4 s** | 248 tiles |
| **el bicho** | **9,2 s** | compone su `.cof`/`.dcc` |
| el drop | 2,0 s | |
| la quest | 1,2 s | |
| **los NPCs** | **5,9 s** | 4 NPCs |
| el minimapa | 0,9 s | |
| soltar el préstamo | 3,0 s | |
| term y renderer | 3,0 s | |
| hasta el primer cuadro | 7,2 s | |

Los cuatro grandes (fuentes + jugador + nivel + bicho) son **65,3 s de los 93**. Todo eso se compone **de
entrada y completo**, cuando en el primer cuadro solo hace falta **una dirección del jugador y la ventana
visible del nivel**.

---

## 2. Los defectos, con causa raíz

### D1 · LA ESTELA: el sprite viejo no se borra (los esqueletos repetidos de la captura)

**Síntoma:** una fila de esqueletos repetidos en diagonal, uno al lado del otro, cada uno con su caja; el texto
del HUD pegado y repetido. En la captura del usuario se ve perfecto: el mismo bicho decenas de veces.

**Causa raíz, en el código:** `monstruo_121.go:928`

```go
func dibujarLosBichos(malla *pvr.Surface, pixeles []byte) (...) {
    // DC-002.134: con el instrumento apagado, el bicho se pinta UNA vez y sin copiar su caja
    if !midiendoElCuadro134() {
        return pintarLosBichosUnaVez(malla, pixeles)
    }
    ...
}
```

`pintarLosBichosUnaVez` (línea 1011) pinta los bichos **sin guardar ni restaurar su caja**. El comentario dice
«el lienzo queda igual», y es verdad **en el POC de medición**, donde se pinta un cuadro y se mide. En la demo,
que dibuja en bucle, cada cuadro pinta el bicho encima del anterior **y nadie borra el de antes**: de ahí la
estela.

El mapa tampoco se repinta cada cuadro (eso se optimizó en la 135/143), así que no hay nada que tape la estela.

**Es el mismo mecanismo para los textos y los paneles:** el camino «sin instrumento» asume que el lienzo no se
reutiliza.

### D2 · EL CRASH: `gc_external_alloc: OOM` cuando el jugador camina

**Síntoma:** «crashea si me muevo un poco».

**Causa raíz, del log de la corrida del usuario:**

```
ARQ-MARCA ext-bloque v=262144 libreAntes=116368 tomadoAntes=10284912 ... quedo=00000000
Out of memory. Requested sbrk_base 0x8cffe000, was 0x8cfde000, diff 131072
fatal error: gc_external_alloc: OOM
```

Al caminar hacia celdas nuevas, el motor pide bloques de **131 a 524 KB** al montón de C, y el montón tiene
**116 KB libres**. El recolector de libgodc no puede liberar ni compactar: muere.

Es la misma familia de problemas que ya se resolvió para los `.dt1` (ADR 0001, tramos con dueño), pero **el
camino que se abre al caminar no lo usa**: pide con `make` y espera que alcance.

### D3 · LAS DIRECCIONES: el sprite no corresponde al movimiento

**Síntoma:** «las animaciones de la dirección están mal», el personaje se ve volteado, y los enemigos se
desplazan de costado o de espaldas.

**Lo que hay hoy:** `movimiento.go:78` → `DireccionDelControl` → `DireccionDelPiso` (`personaje.go:310`):

```go
cx := (ejeX/float64(ladoCeldaX) + ejeY/float64(ladoCeldaY)) / 2
cy := (ejeY/float64(ladoCeldaY) - ejeX/float64(ladoCeldaX)) / 2
```

La conversión pantalla→piso isométrica está escrita a mano (no usa la del motor) y **no está verificada contra
la tabla de direcciones del exe**: las 8 direcciones se ven en captura pero no se comparó `dir` del control
contra `dir` del `.cof` en las 8, una por una. Eso es lo que hay que hacer, y con la tabla delante.

Y el bicho: su dirección se toma de su patrulla, no de su vector de movimiento, así que puede mirar a un lado y
caminar a otro. El criterio del usuario es el correcto y hay que implementarlo: **el bicho gira primero y avanza
siempre de frente** (a lo sumo diagonal), y el knockback es la única excepción, con su animación.

### D4 · LA VIDA AL TIRÓN Y LA MUERTE DE 63 SEGUNDOS

**Síntoma:** «me mata lento y me baja la vida al tirón».

**Causa raíz:** el reloj de la animación avanza **un cuadro por cuadro dibujado**, no por tiempo real. Está
escrito en el log de DC-002.126:

```
JUEGO126: MUERTE EMPIEZA ... reloj= un cuadro por cuadro del POC (INSTRUMENTO: la consola no dibuja a 20 fps)
```

Con 20-31 ms por cuadro, cada cuadro de animación de 40 ms se consume más rápido o más lento de lo que debe,
según el cuadro. La muerte de 19 cuadros × 40 ms = 760 ms medía **64.653 ms**. Y la vida baja «a saltos» porque
el golpe se aplica por cuadro, no por tiempo.

### D5 · LOS OBJETOS DEL CINTURÓN FLOTANDO

**Síntoma:** las pociones se ven desplazadas, arriba del cinturón.

**Lo que hay:** `hud_125.go` — el cinturón es `ctrlpnl_popbelt.DC6` (125×32 = 4 cajas de 31×32) y las pociones
son de 28×28. El código pinta la poción con el ancla de su propia caja, sin centrarla en la caja del cinturón:
28 en 31 deja 3 px, 28 en 32 deja 4. **Sin medir la posición exacta**: falta el número (x,y de cada poción
contra la caja del cinturón) y por eso «flota».

### D6 · LOS ESQUELETOS DENTRO DEL PUEBLO

**Síntoma:** el usuario los vio en el campamento.

**Causa raíz:** en la demo los bichos **no salen del nivel**: son `monstruosKnob` sembrados a mano cerca del
jugador (DC-002.121). En un POC de medición da igual; en la demo pone esqueletos en el pueblo. Y el nombre de
la zona del HUD no sigue al jugador: en la captura dice «Blood Moor» con el jugador en el campamento.

---

## 3. El diagnóstico de fondo

La demo está construida **encima de un banco de pruebas**, y se nota en los tres lugares donde duele:

1. **El dibujo se escribió para medir, no para jugar.** Un cuadro de medición se pinta una vez y se compara
   contra el host; un juego pinta 50 veces por segundo sobre el mismo lienzo. Todo el camino «sin instrumento»
   (`pintarLosBichosUnaVez`) asume lo primero.
2. **La memoria se pidió para caber una vez, no para durar.** El arranque reserva todo de entrada (93 s) y
   cuando el jugador camina y pide más, el montón no tiene y muere.
3. **Las entidades se movieron para demostrar que se mueven, no para comportarse como el juego.** Dirección
   desde la patrulla, reloj por cuadro, bichos sembrados.

Ninguna de las tres es de arquitectura. Son tres caminos concretos que hay que reescribir con criterio de
juego. Pero mientras no se reescriban, cada tarjeta nueva va a seguir chocando con ellas.

---

## 4. La propuesta de flujo

### 4.1 Lo que NO hay que hacer

- **Hot reload de código.** El build incremental son 49 s: no compensa la complejidad.
- **Bajar los bits de color / hacer un batch de texturas.** Ya están a 8 bits con paleta (2,7 MB contra 10,9 MB
  en RGBA, medido en DC-002.101). Las texturas salen del MPQ del juego original; no hay nada que reescalar.
- **Sacar el sonido.** No se carga: no hay sonido en la demo.

### 4.2 Lo que SÍ

1. **El arranque a 15 s o menos, componiendo bajo demanda.** Una dirección del jugador y la ventana visible del
   nivel al arrancar; el resto cuando se pide. Es la mitad del ciclo de iteración y además hace que la consola
   real arranque rápido.
2. **320×240 como modo de dibujo.** El juego se dibuja a 640×480 y cuesta 20-31 ms. A 320×240 son 4× menos
   píxeles: el cuadro baja a ~7-8 ms y el emulador (o el PVR en la consola real) escala. Se puede tener como
   palanca (`RESOLUCION=320x240`) y comparar.
3. **Un arnés de prueba que corra solo.** Hoy cada verificación es una corrida manual de 2-3 min. Con un arnés
   que lance la demo, mande teclas por el teclado virtual, saque capturas en los cuadros clave y compare contra
   el esperado, un fix se valida en una corrida y sin humano.
4. **Recorte de alcance: solo el pueblo.** Fuera los actos 2-5, fuera las otras zonas, fuera la generación
   aleatoria. El pueblo con sus 4 NPCs, sus bichos de nivel, la Den of Evil y las animaciones bien. Lo demás se
   vuelve a encender cuando el pueblo esté bien.
5. **Los workers con más razonamiento.** `hermes kanban effort <id> m5` (xhigh) para los bugs de causa raíz, y
   `m3` para el trabajo mecánico. Un worker que razona cuesta más tokens pero llega antes a la causa.

### 4.3 El orden, por dependencia

| # | qué | por qué primero |
|---|---|---|
| 1 | **D1 la estela** | es lo que hace que la demo «se vea mal» en todas las capturas |
| 2 | **D2 el crash** | sin estabilidad no se puede probar nada durante más de 30 s |
| 3 | **D4 el reloj** | sin reloj real, ni la animación ni el combate se pueden juzgar |
| 4 | **D3 las direcciones** | el comportamiento del bicho depende de esto |
| 5 | **D5 el cinturón y D6 los bichos** | detalles de presentación y de spawn |
| 6 | **el arranque a 15 s** | acelera todo lo que venga después |
| 7 | **320×240** | palanca de fps, cuando lo de arriba esté bien |

---

## 5. Lo que queda sin medir (deuda declarada)

- La posición exacta de las pociones dentro de la caja del cinturón (D5).
- Cuántas celdas nuevas pide el jugador al caminar y de qué tamaño (D2) — el log da 131-524 KB pero no dice
  qué los pide.
- Si 320×240 baja el cuadro a ~7 ms: es una cuenta, no una medición.
- El mapeo de las 8 direcciones contra la tabla del exe (D3).

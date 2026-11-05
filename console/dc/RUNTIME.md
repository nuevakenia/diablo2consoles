# El runtime de la consola: un solo comando y un solo hash

Deuda cerrada por DC-002.159i (§5.2 de `dc-002.159h-el-hasher-que-era-datos.md`).

## Por qué existe este archivo

El `.a` de `libgodc` que usó DC-002.159g no está en ningún lado, y `fix-libgodc-cadena.sh` **no** lo
reproduce: la cadena no trae los arreglos de 159h. Sin un estado declarado, cualquier comparación
"antes/después" contra 159g no es válida. El estado canónico es **este**, y se comprueba con un
comando.

## El estado canónico

```
python3 scripts/console/fix-libgodc-funcval-159h.py --estado
```

Tiene que decir las siete líneas en `PUESTO` y terminar así:

```
   libgodc.a: 219990 bytes, sha256 794a70ba47237cdc
```

Ese sha256 (prefijo `794a70ba`) es la referencia: si no coincide, la corrida no es comparable.

## Cómo se llega (un comando)

```
python3 scripts/console/fix-libgodc-funcval-159h.py --build
```

Aplica, sobre el árbol de `~/dreamcast/libgodc`, lo que el estado verifica:

| pieza | dónde | de qué tarjeta |
|---|---|---|
| el hasher resuelto (`FuncVal` vs código) | `map_dreamcast.h` | DC-002.159h |
| la igualdad resuelta (`__equalfn`) | `map_dreamcast.c` | DC-002.159h |
| el valor indirecto | `map_dreamcast.c` | DC-002.159h |
| la marca gc-vivo | cadena | DC-002.45/47 |
| el sensor fuera de RAM | cadena | DC-002.92 |
| la tabla de tipos | cadena | DC-002.146d |
| el semiespacio de la RAMA (1792 KB) | cadena | — |

Y después, en verde (ojo: sobre `prog.elf`, **no** sobre `prog-emu.elf` — ese es el enlazado del emulador y el
chequeo lo da rojo porque no reconoce los símbolos):

```
python3 scripts/console/checar-mapas-159h.py /tmp/dcbuild/gate0-<algo>/prog.elf
```

Tiene que terminar en

```
VERDE: los 338 mapas del binario quedan con hasher llamable (y la igualdad de la clave tambien)
```

## La regla

Antes de medir en la consola, `--estado` tiene que dar `794a70ba`. Si da otra cosa, `--build` y de
nuevo `--estado`; si sigue distinto, el árbol de libgodc se tocó a mano y **no se mide**: se arregla
primero.

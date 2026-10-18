#!/usr/bin/env python3
"""DC-002.29: MARCAS EN LA SECUENCIA DE `init` -- el indice exacto donde muere el arranque.

POR QUE HACE FALTA ESTE INSTRUMENTO (medido en DC-002.29)
    Las marcas del runtime (scripts/console/fix-libgodc-marcas-arranque.py) acotan la muerte del binario
    rearmado a UN tramo: todo `runtime_init()` pasa, el scheduler arranca la goroutine
    (`main_wrapper-entra`) y despues NADA: de los 118 `init` de paquetes que el enlazador mete en
    `___go_init_main`, no se sabe cual mata. Y no se puede instrumentar desde Go: `___go_init_main` es
    codigo GENERADO por el driver de gccgo a partir de los `.go_export` de los .gox, y meter un `println`
    en un paquete del motor mueve el monton (la leccion medida de DC-002.22: el defecto cambia con el
    layout).

COMO
    No se toca ni una linea de Go ni el arranque: se parchea el ELF ENLAZADO.
      1. se leen de `___go_init_main` las N llamadas (`jsr @r1` con su pool de literales) y, de cada una,
         la direccion del literal y la direccion destino.
      2. se arma un bloque de trozos (uno por llamada) que imprimen su indice con `godc_init_marca` y
         despues saltan al destino ORIGINAL. Los trozos preservan r0 y r4 (los registros que la llamada
         usa para argumentos: el `init` numero 1 es `registerTypeDescriptors(n, tabla)`).
      3. el bloque se agrega como seccion nueva (`.text.stubs`) y los literales del pool se repuntan a los
         trozos.
    Nada de esto cambia el codigo Go: solo agrega una seccion y reescribe 118 punteros. El ELF que se corre
    es el MISMO binario, con el arranque contado.

COMO SE LEE
    `ARQ-INIT idx = <n>` por la consola serie, uno por cada `init` que ENTRA. La ultima linea que sale dice
    el paquete: el indice se cruza con la lista de `init` del binario (mismo orden que el enlazador genera):
        python3 scripts/console/marcas-init-elf.py --lista prog.elf

USO
    python3 scripts/console/marcas-init-elf.py <prog.elf> <prog-instr.elf>
    # y despues: run-flycast.sh <prog-instr.elf> 30 <salida>   (el ELF directo, sin ISO: el defecto es
    # anterior a tocar el disco)
    python3 scripts/console/marcas-init-elf.py --lista <prog.elf>     # la tabla indice -> paquete
"""
import os
import re
import subprocess
import sys

ENV = "source %s/../console/env.sh && " % os.path.dirname(os.path.abspath(__file__))
INIT_SYM = "___go_init_main"


def sh(cmd):
    r = subprocess.run(["bash", "-c", ENV + cmd], capture_output=True, text=True)
    if r.returncode != 0:
        sys.exit("FALLO: %s\n%s%s" % (cmd, r.stdout[-2000:], r.stderr[-2000:]))
    return r.stdout


def sim(elf, nombre):
    """direccion de un simbolo"""
    out = sh("sh-elf-nm %s | grep -w '%s'" % (elf, nombre))
    for line in out.splitlines():
        p = line.split()
        if len(p) >= 3 and p[2] == nombre:
            return int(p[0], 16)
    return None


def secciones(elf):
    """nombre -> (vma, offset_archivo, tamano)"""
    secs = {}
    for line in sh("sh-elf-objdump -h %s" % elf).splitlines():
        m = re.match(r"\s*\d+\s+(\S+)\s+([0-9a-f]{8})\s+([0-9a-f]{8})\s+([0-9a-f]{8})\s+([0-9a-f]{8})", line)
        if m:
            secs[m.group(1)] = (int(m.group(3), 16), int(m.group(5), 16), int(m.group(2), 16))
    return secs


def llamadas(elf):
    """[(indice, direccion del jsr, vma del literal, direccion destino, nombre)]"""
    ini = sim(elf, INIT_SYM)
    if ini is None:
        sys.exit("no encuentro %s en %s" % (INIT_SYM, elf))
    # el fin del simbolo: la distancia al siguiente objeto (objdump lo trae en la cabecera)
    d = sh("sh-elf-objdump -d --disassemble=%s %s" % (INIT_SYM, elf))
    out = []
    pend = None
    for line in d.splitlines():
        line = line.rstrip()
        m = re.match(r"\s*([0-9a-f]+):\t[0-9a-f ]+\tmov\.l\t([0-9a-f]+)\s*<[^>]*>,\s*(r\d+)\s*!\s*([0-9a-f]+)\s*<([^>]*)>", line)
        if m:
            pend = (m.group(3), int(m.group(2), 16), int(m.group(4), 16), m.group(5))
            continue
        m2 = re.match(r"\s*([0-9a-f]+):\t[0-9a-f ]+\tjsr\t@(r\d+)", line)
        if m2 and pend and pend[0] == m2.group(2):
            out.append((len(out) + 1, int(m2.group(1), 16), pend[1], pend[2], pend[3]))
            pend = None
    return out


def vma_a_off(secs, vma):
    for nombre, (svma, soff, stam) in secs.items():
        if ".debug" in nombre or nombre.startswith(".strtab") or nombre.startswith(".symtab"):
            continue
        if svma <= vma < svma + stam:
            return soff + (vma - svma), nombre
    return None, None


STUB = """
    .section .text.stubs,"ax",@progbits
    .align 2
stub_%(k)d:
    mov.l r4, @-r15
    mov.l r0, @-r15
    mov #%(k)d, r4
    mov.l 9f, r0
    jsr @r0
    nop
    mov.l @r15+, r0
    mov.l @r15+, r4
    mov.l 8f, r0
    jmp @r0
    nop
    .align 2
9:  .long 0x11110000 + %(k)d
8:  .long 0x22220000 + %(k)d
"""


def arma_stubs(n, dir_marca, destinos, salida):
    """ensambla los trozos; devuelve (blob, offs) con el offset de cada trozo DENTRO del blob.

    No se usan las direcciones del .o (objcopy -O binary ancla el blob donde quiere): el trozo se ubica
    por su PRIMERA instruccion, que lleva el indice adentro. Los literales se resuelven por patron unico.
    """
    src = "/tmp/dc229-stubs.s"
    with open(src, "w") as f:
        f.write('\n    .text\n    .global _d2c_dummy\n_d2c_dummy:\n    rts\n    nop\n')
        for k in range(1, n + 1):
            f.write(STUB % {"k": k})
    o = "/tmp/dc229-stubs.o"
    sh("sh-elf-as -little --isa=sh4a %s -o %s" % (src, o))
    blob = "/tmp/dc229-stubs.bin"
    sh("sh-elf-objcopy -O binary --only-section=.text.stubs %s %s" % (o, blob))
    b = bytearray(open(blob, "rb").read())

    # el trozo k arranca con: mov.l r4,@-r15 / mov.l r0,@-r15 / mov #k,r4   (0x2F46 0x2F06 0xE4kk)
    offs = {}
    for k in range(1, n + 1):
        cab = bytes([0x46, 0x2F, 0x06, 0x2F, k & 0xFF, 0xE4])
        i = b.find(cab)
        if i < 0 or b.find(cab, i + 1) >= 0:
            sys.exit("la cabecera del trozo %d no es unica en el blob (%d)" % (k, i))
        offs[k] = i

    for k in range(1, n + 1):
        for marca, valor in ((0x11110000, dir_marca), (0x22220000, destinos[k])):
            ph = (marca + k).to_bytes(4, "little")
            i = b.find(ph)
            if i < 0 or b.find(ph, i + 1) >= 0:
                sys.exit("el placeholder de %s del trozo %d no es unico" % (hex(marca), k))
            b[i:i + 4] = valor.to_bytes(4, "little")
    open(salida, "wb").write(bytes(b))
    return salida, offs


def main():
    if "--lista" in sys.argv:
        elf = [a for a in sys.argv[1:] if not a.startswith("--")][0]
        ll = llamadas(elf)
        print("%d llamadas en %s" % (len(ll), INIT_SYM))
        for k, j, lit, dst, nom in ll:
            print("   %3d  %s" % (k, nom))
        return

    elf, destino = [a for a in sys.argv[1:] if not a.startswith("--")][:2]
    secs = secciones(elf)
    marca = sim(elf, "_godc_init_marca")
    if marca is None:
        sys.exit("falta _godc_init_marca en %s: corre fix-libgodc-marcas-arranque.py --build" % elf)
    ll = llamadas(elf)
    print("== %s: %d llamadas a instrumentar (marca en 0x%08x)" % (elf, len(ll), marca))

    destinos = {k: dst for k, _, _, dst, _ in ll}
    blob, offs = arma_stubs(len(ll), marca, destinos, "/tmp/dc229-stubs.bin")

    # 1) el ELF con la seccion nueva
    tmp = "/tmp/dc229-con-stubs.elf"
    sh("sh-elf-objcopy --add-section .text.stubs=/tmp/dc229-stubs.bin "
       "--set-section-flags .text.stubs=alloc,code,load,contents %s %s" % (elf, tmp))
    secs2 = secciones(tmp)
    if ".text.stubs" not in secs2:
        sys.exit("la seccion .text.stubs no quedo en el ELF")
    vma_stubs = secs2[".text.stubs"][0]
    print("   .text.stubs en 0x%08x (%d bytes)" % (vma_stubs, len(blob)))

    # 2) se repuntan los literales del pool a los trozos
    datos = bytearray(open(tmp, "rb").read())
    n = 0
    for k, jsr_addr, lit_vma, dst, nom in ll:
        off, sec = vma_a_off(secs, lit_vma)
        if off is None:
            sys.exit("el literal de la llamada %d (0x%08x) no cae en ninguna seccion" % (k, lit_vma))
        antes = int.from_bytes(datos[off:off + 4], "little")
        if antes != dst:
            sys.exit("el literal %d no vale lo que dice el desensamblado (0x%08x contra 0x%08x)" % (k, antes, dst))
        nuevo = vma_stubs + offs[k]
        datos[off:off + 4] = nuevo.to_bytes(4, "little")
        n += 1
    open(destino, "wb").write(bytes(datos))
    print("   %d literales del pool repuntados a los trozos" % n)

    # 3) contraste: el pool ya no apunta a los destinos y la seccion nueva esta
    chk = subprocess.run(["bash", "-c", ENV + "sh-elf-objdump -h %s | grep -c text.stubs" % destino],
                         capture_output=True, text=True).stdout.strip()
    print("   comprobacion: secciones .text.stubs en el ELF final = %s" % chk)
    print("   listo: %s (%d bytes, el mismo binario con el arranque contado)" % (destino, os.path.getsize(destino)))


if __name__ == "__main__":
    main()

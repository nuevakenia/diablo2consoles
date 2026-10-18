#!/usr/bin/env python3
"""DC-002.29: la lista de `init` de ___go_init_main de un ELF ENLAZADO (indice -> destino).

PARA QUE: el control 6 del corpus de la tarjeta (checar-rearme-229.sh) tiene que poder decir que
la secuencia de `init` que arma el enlazador es la MISMA en el binario del motor que anda y en el
rearmeado que muere -- o sea que el defecto NO es la lista ni el orden de los `init`.

COMO: cada `init` se llama con `mov.l <pool>,r1 ; jsr @r1`, asi que se leen los literales de
`___go_init_main` en el orden en que se ejecutan. No hace falta correr nada ni instrumentar el ELF.

Uso: python3 scripts/console/lista-inits-229.py <prog.elf>
"""
import os
import re
import subprocess
import sys

ENV = "source %s/env.sh && " % os.path.dirname(os.path.abspath(__file__))


def sh(c):
    return subprocess.run(["bash", "-c", ENV + c], capture_output=True, text=True).stdout


def lista(elf):
    out = sh("sh-elf-objdump -d '%s'" % elf)
    salida = []
    dentro = False
    pend = None
    for ln in out.splitlines():
        m = re.match(r'^([0-9a-f]+) <(.+)>:$', ln)
        if m:
            dentro = bool(re.match(r'___go_init_main(\+0x[0-9a-f]+)?$', m.group(2)))
            continue
        if not dentro:
            continue
        m = re.match(r'^([0-9a-f]+):\s+[0-9a-f ]+\s+mov\.l\s+\S+.*,r1\t! ([0-9a-f]+) <(.+)>', ln)
        if m:
            pend = (m.group(2), m.group(3))
            continue
        if "jsr\t@r1" in ln and pend:
            salida.append(pend)
            pend = None
    return salida


elf = sys.argv[1]
for i, (addr, nom) in enumerate(lista(elf), 1):
    print("%3d  %s  %s" % (i, addr, nom))

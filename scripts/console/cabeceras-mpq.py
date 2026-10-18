#!/usr/bin/env python3
"""Cabecera cruda de cada MPQ del juego: el numero de entradas de las dos tablas (y sus bytes).

DC-002.28: la sonda del HOST. Es el MISMO numero que el motor va a pedir al abrir el archivo
(readHashTable/readBlockTable: entries * 16 bytes cada tabla), asi que sirve para saber cuanto pide
antes de medirlo en la consola.
"""
import os
import struct
import sys

d = sys.argv[1] if len(sys.argv) > 1 else os.path.expanduser("~/.wine-d2/drive_c/Games/Diablo II")

for n in sorted(os.listdir(d)):
    if not n.lower().endswith(".mpq"):
        continue

    p = os.path.join(d, n)
    with open(p, "rb") as f:
        h = f.read(32)

    magic = h[:4]
    hdr_size, archive_size, ver, blk_size = struct.unpack_from("<IIHH", h, 4)
    ht_off, bt_off, ht_cnt, bt_cnt = struct.unpack_from("<IIII", h, 16)

    print("%-14s %10d bytes  magic=%r ver=%d blk=%d hashEnt=%6d blockEnt=%6d  hashBytes=%9d blockBytes=%9d"
          % (n, os.path.getsize(p), magic, ver, blk_size, ht_cnt, bt_cnt, ht_cnt * 16, bt_cnt * 16))

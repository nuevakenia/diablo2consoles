#!/usr/bin/env python3
"""DC-002.83 -- lee los descriptores de tipo de un ELF de la consola (SH-4, 32 bits LE).

Sirve para contestar UNA pregunta con el binario en la mano y no de memoria: cuando el runtime hace
`gc_alloc(size, MAPTYPE_BUCKET(t))`, ¿que dice el descriptor del tipo del bucket sobre sus punteros?
Si `__ptrdata == 0` el objeto se marca NOSCAN y el GC no lo barre; si `__ptrdata > 0` hay que barrerlo.

Uso:
    python3 scripts/console/leer-tipos-elf.py prog.elf <vma> [<vma> ...]

Las direcciones se sacan con:
    sh-elf-nm prog.elf | grep '_type..map_\\|_type..struct.*tophash'
"""
import struct
import sys

DESC_FMT = "<II"  # __size, __ptrdata
BASE_FMT = "<II I BBBB IIIII"  # 36 bytes: size,ptrdata,hash,tflag,align,fieldalign,code,equalfn,gcdata,reflection,uncommon,pointer_to_this


class Elf:
    def __init__(self, path):
        self.d = open(path, "rb").read()
        assert self.d[:4] == b"\x7fELF", "no es un ELF"
        assert self.d[4] == 1, "no es 32 bits"
        assert self.d[5] == 1, "no es little endian"
        e_phoff = struct.unpack_from("<I", self.d, 0x1C)[0]
        e_phentsize, e_phnum = struct.unpack_from("<HH", self.d, 0x2A)
        self.segs = []
        for i in range(e_phnum):
            off = e_phoff + i * e_phentsize
            p_type, p_offset, p_vaddr, p_paddr, p_filesz, p_memsz, p_flags, p_align = struct.unpack_from(
                "<8I", self.d, off)
            if p_type == 1:  # PT_LOAD
                self.segs.append((p_vaddr, p_vaddr + p_filesz, p_offset, p_filesz, p_flags))

    def leer(self, vma, n):
        for vaddr, vend, off, filesz, flags in self.segs:
            if vaddr <= vma and vma + n <= vend:
                return self.d[off + (vma - vaddr): off + (vma - vaddr) + n]
        raise SystemExit("la direccion 0x%08x no esta en ningun segmento cargado del ELF" % vma)

    def u32(self, vma):
        return struct.unpack("<I", self.leer(vma, 4))[0]

    def desc(self, vma, nombre=""):
        size, ptrdata, hash_, tflag, align, falign, code, eqfn, gcdata, refl, unc, ptothis = struct.unpack(
            BASE_FMT, self.leer(vma, 36))
        print("%s 0x%08x: size=%d ptrdata=%d hash=0x%08x tflag=0x%02x code=%d(kind=%d) equalfn=0x%08x "
              "gcdata=0x%08x reflection=0x%08x" %
              (nombre or "tipo", vma, size, ptrdata, hash_, tflag, code, code & 0x1F, eqfn, gcdata, refl))
        if gcdata:
            # cuantos bytes de bitmap hay: ceil(ptrdata/8/4)*4 aprox. Se imprimen los primeros 16.
            try:
                bm = self.leer(gcdata, min(16, max(4, ((ptrdata + 7) // 8 + 3) // 4 * 4)))
                print("    gcdata bytes: %s" % " ".join("%02x" % b for b in bm))
            except SystemExit:
                print("    gcdata 0x%08x NO esta en un segmento cargado (¿fuera del ELF?)" % gcdata)
        return (size, ptrdata, code)

    def mapa(self, vma):
        size, ptrdata, code = self.desc(vma, "mapa")
        key, val, bucket = struct.unpack("<III", self.leer(vma + 36, 12))
        keysize = self.leer(vma + 52, 1)[0]
        valsize = self.leer(vma + 53, 1)[0]
        bucketsize = struct.unpack("<H", self.leer(vma + 54, 2))[0]
        print("    key=0x%08x val=0x%08x bucket=0x%08x keysize=%d valsize=%d bucketsize=%d" %
              (key, val, bucket, keysize, valsize, bucketsize))
        return bucket, bucketsize


def main():
    if len(sys.argv) < 3:
        raise SystemExit("uso: leer-tipos-elf.py prog.elf <vma> [<vma> ...]")
    e = Elf(sys.argv[1])
    for a in sys.argv[2:]:
        vma = int(a, 0)
        print("== 0x%08x" % vma)
        # ¿es un maptype? Se prueba por el __code: kind MAP = 21
        code = e.leer(vma + 15, 1)[0]
        if (code & 0x1F) == 21:
            bucket, bsize = e.mapa(vma)
            print("  -- el tipo del BUCKET (el que recibe gc_alloc en allocBuckets) --")
            e.desc(bucket, "bucket")
            print("  -- el tamano del array con 2^B buckets --")
            for b in range(0, 13):
                n = 1 << b
                print("     B=%2d buckets=%5d  bytes=%8d  %s" %
                      (b, n, n * bsize, "EXTERNO (malloc, invisible al GC)" if n * bsize > 65536 else "en el monton Go"))
        else:
            e.desc(vma)


if __name__ == "__main__":
    main()

#!/usr/bin/env python3
"""Teclado virtual por /dev/uinput: para poder probar la ENTRADA del POC sin depender de que alguien apriete.

POR QUE HACE FALTA: la prueba del control tiene que poder correr sola. En Wayland no hay xdotool y wtype no
sirve (el compositor de KDE no tiene el protocolo de teclado virtual), pero /dev/uinput es del KERNEL: lo que
se escribe ahi lo ve cualquier cosa que lea el teclado, incluido el emulador. Es la unica via que no depende
del compositor.

Como se usa:  python3 teclas-virtuales.py derecha derecha z Return
El device se destruye al terminar (si no, quedaria un teclado fantasma en el sistema).
"""

import fcntl
import os
import struct
import sys
import time

# del header del kernel (linux/uinput.h). Los numeros estan puestos a mano porque no hace falta traer todo el
# header para cuatro ioctls.
UI_SET_EVBIT = 0x40045564
UI_SET_KEYBIT = 0x40045565
UI_DEV_CREATE = 0x5501
UI_DEV_DESTROY = 0x5502
EV_SYN = 0x00
EV_KEY = 0x01
SYN_REPORT = 0

# linux/input-event-codes.h
TECLAS = {
    "derecha": 106,   # KEY_RIGHT
    "izquierda": 105, # KEY_LEFT
    "arriba": 103,    # KEY_UP
    "abajo": 108,     # KEY_DOWN
    "enter": 28,      # KEY_ENTER
    "espacio": 57,    # KEY_SPACE
    "escape": 1,      # KEY_ESC
    "z": 44, "x": 45, "c": 46, "v": 47,
    "a": 30, "s": 31, "d": 32, "f": 33,
    "q": 16, "w": 17, "e": 18, "r": 19,
    "enter_kp": 96,   # KEY_KPENTER
}


def abrir():
    """Crea el teclado virtual. Devuelve el archivo abierto."""
    fd = os.open("/dev/uinput", os.O_WRONLY | os.O_NONBLOCK)

    fcntl.ioctl(fd, UI_SET_EVBIT, EV_KEY)
    for codigo in TECLAS.values():
        fcntl.ioctl(fd, UI_SET_KEYBIT, codigo)

    # struct uinput_user_dev: 80 bytes de nombre, 4 unsigned short (id) y 4 ints de 64 valores de abs/rep
    # (no se usan, van en cero).
    # 80s: el nombre | HHHH: el id del aparato (tipo de bus, fabricante, producto, version) | I: los efectos
    # de fuerza. Eran CUATRO H y una I (5 valores) y estaban puestos solo cuatro: el struct lo dijo en el
    # acto ("pack expected 6 items for packing (got 5)"), que es mejor que un aparato a medio crear.
    dispositivo = struct.pack("80sHHHHI", b"d2-teclado-virtual", 0x03, 1, 1, 1, 0)
    dispositivo += struct.pack("256i", *([0] * 256))
    os.write(fd, dispositivo)

    fcntl.ioctl(fd, UI_DEV_CREATE)
    time.sleep(0.4)  # que el sistema lo registre antes de mandar la primera tecla
    return fd


def evento(fd, tipo, codigo, valor):
    # struct input_event en 64 bits: dos long del reloj (16 bytes), y despues tipo, codigo y valor
    os.write(fd, struct.pack("llHHi", 0, 0, tipo, codigo, valor))
    os.write(fd, struct.pack("llHHi", 0, 0, EV_SYN, SYN_REPORT, 0))


def apretar(fd, nombre, cuanto=0.25):
    codigo = TECLAS[nombre]
    evento(fd, EV_KEY, codigo, 1)
    time.sleep(cuanto)
    evento(fd, EV_KEY, codigo, 0)
    time.sleep(0.15)


def sostener(fd, nombres, segundos):
    """Aprieta una o varias teclas A LA VEZ y las mantiene `segundos`.

    POR QUE (DC-002.38): el movimiento del personaje se mide por tiempo, asi que la tecla tiene que quedar
    apretada un rato: `apretar` la suelta a los 0,25 s y solo sirve para los menus (el flanco). Con esto se
    puede pedir "mantene derecha 4 segundos" desde la linea de comandos, igual que una persona con el dedo
    en la cruceta.
    """
    for n in nombres:
        evento(fd, EV_KEY, TECLAS[n], 1)
    time.sleep(segundos)
    for n in nombres:
        evento(fd, EV_KEY, TECLAS[n], 0)
    time.sleep(0.15)


def guion(fd, pasos):
    """Sostiene varias teclas EN SECUENCIA con el MISMO aparato.

    POR QUE (DC-002.38, medido): cada llamada a este script crea y destruye un teclado virtual. Llamarlo tres
    veces seguidas (una por tecla) NO funciona: el emulador solo ve la primera. Con un solo aparato para toda
    la secuencia, las tres teclas llegan.

    LA PAUSA ENTRE TRAMOS la fija `PAUSA` (segundos, por defecto 1,0). En DC-002.38 va en 5,0 porque la
    captura de cada tramo se saca CUANDO LA TECLA YA SE SOLTO (run-flycast-238.sh espera la marca del POC):
    con menos pausa, la captura de un tramo se cruza con la pulsacion del siguiente y la foto deja de ser la
    del tramo que dice.
    """
    pausa = float(os.environ.get("PAUSA", "1.0"))
    for nombres, segundos in pasos:
        print("  sostengo", "+".join(nombres), "%.2f s" % segundos)
        sostener(fd, nombres, segundos)
        time.sleep(pausa)


def main():
    nombres = sys.argv[1:]
    if not nombres:
        print("uso: teclas-virtuales.py <tecla> [tecla...]")
        print("     teclas-virtuales.py sostener <tecla[+tecla]> <segundos>")
        print("     teclas-virtuales.py guion <tecla> <segundos> [<tecla> <segundos> ...]")
        print("teclas:", " ".join(sorted(TECLAS)))
        return 2

    if nombres[0] == "guion":
        resto = nombres[1:]
        if len(resto) < 2 or len(resto) % 2 != 0:
            print("uso: teclas-virtuales.py guion <tecla> <segundos> [<tecla> <segundos> ...]")
            return 2
        pasos = []
        for i in range(0, len(resto), 2):
            combinacion = resto[i].split("+")
            for n in combinacion:
                if n not in TECLAS:
                    print("no conozco la tecla", n)
                    return 2
            pasos.append((combinacion, float(resto[i + 1])))
        fd = abrir()
        try:
            print("teclado virtual creado; guion de", len(pasos), "tramos")
            guion(fd, pasos)
            print("listo")
        finally:
            fcntl.ioctl(fd, UI_DEV_DESTROY)
            os.close(fd)
        return 0

    if nombres[0] == "sostener":
        if len(nombres) != 3:
            print("uso: teclas-virtuales.py sostener <tecla[+tecla]> <segundos>")
            return 2
        combinacion = nombres[1].split("+")
        for n in combinacion:
            if n not in TECLAS:
                print("no conozco la tecla", n)
                return 2
        segundos = float(nombres[2])
        fd = abrir()
        try:
            print("teclado virtual creado; sostengo", "+".join(combinacion), "%.2f s" % segundos)
            sostener(fd, combinacion, segundos)
            print("listo")
        finally:
            fcntl.ioctl(fd, UI_DEV_DESTROY)
            os.close(fd)
        return 0

    for n in nombres:
        if n not in TECLAS:
            print("no conozco la tecla", n)
            return 2

    fd = abrir()
    try:
        print("teclado virtual creado; mando", len(nombres), "teclas")
        for n in nombres:
            print("  -", n)
            apretar(fd, n)
        print("listo")
    finally:
        fcntl.ioctl(fd, UI_DEV_DESTROY)
        os.close(fd)
    return 0


if __name__ == "__main__":
    sys.exit(main())

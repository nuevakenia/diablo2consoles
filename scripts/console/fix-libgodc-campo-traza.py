#!/usr/bin/env python3
"""Traza QUIEN escribe en el campo `errorMessage` (offset 156) del App: la causa del crash del cuadro 0.

SINTOMA (DC-002.23, medido)
    El arranque del motor se cae en el cuadro 0 con `Unhandled exception: PC 0000000b` evaluando
    `a.errorMessage.Error()` en `d2app.App.updateInitError`. El CAMPO `errorMessage` del App (offset 156,
    168 bytes de struct, medido con `unsafe.Offsetof`) se guarda NIL en `Create` y en `loadEngine` ya tiene
    basura que CAMBIA de una corrida a otra segun donde caiga todo en el monton:

        itab= 0x9          data= 0x8c912590       (un layout)
        itab= 0x8c4f3c60   data= 0xff000000       (otro layout; 0x8c4f3c60 = _image_1color.RGBA..d)

    O sea: el campo no es "el error del motor", es MEMORIA del App que se pisa. Lo que faltaba era QUIEN
    la pisa.

QUE HACE ESTE INSTRUMENTO
    La instrumentacion va en el RUNTIME (C, vendorizado), NO en el codigo Go: asi no cambia el layout de
    los objetos de Go (que es justo lo que mueve el defecto de forma). El objeto a vigilar se reconoce
    SOLO: el runtime mira el descriptor de tipo de cada objeto que asigna y cuando ve uno que se llama
    `App` y mide 168 bytes, guarda su direccion y la vigila. Sirve para cualquier POC que cree el App
    (`d2app.Create`), sin tocarle una linea a nadie.

    Sobre esa direccion imprime, con el prefijo `CAMPO-TRAZA`:

      1. `App vigilado en <addr>`      cuando el App se asigna (con la generacion del GC).
      2. `PISADO ...`                  un objeto de OTRO tipo se copia CAYENDO sobre el App (el escritor:
                                       sale el tipo del objeto, su tamano, el destino, el origen y el pc
                                       del llamador).
      3. `el App SE COPIA ...`         el App pasa por una recoleccion (o sea: el GC SI lo alcanza desde
                                       las raices).
      4. `gc n=... `                   por recoleccion: que espacio es el from, si el App estaba ahi y si
                                       se copio. `copiado= no` con el App en el from-space es el defecto
                                       medido: el GC lo trata como basura y su memoria queda para el que
                                       venga.
      5. `monton ...`                  el asignador entrega memoria que cubre el App (o sea: ya lo dio por
                                       muerto y la esta repartiendo de nuevo).
      6. `campo ...`                   el GC actualizo las palabras del campo (con el antes y el despues).
      7. `memmove ...`                 una copia de struct/memoria con destino sobre el App.
      8. `cambio tras <sitio>`         las dos palabras del campo cambiaron despues de esa operacion: el
                                       que lo escribio es la operacion que dice el `<sitio>`.

    Tope de 80 lineas para no inundar la consola serie.

USO
    python3 scripts/console/fix-libgodc-campo-traza.py --build       # aplica y rehace libgodc.a
    # reenlazar (los .gox ya estan: no hace falta recompilar el motor) y correr Flycast:
    bash scripts/console/build-motor-arranque.sh /tmp/dcbuild/dc224-arranque /tmp/gox-222
    CAPTURE=none bash scripts/console/run-flycast.sh /tmp/dcbuild/dc224-arranque/prog-emu.elf 25 /tmp/dc-run-dc224
    grep -a 'CAMPO-TRAZA' /tmp/dc-run-dc224/flycast.log | tr -d '\\r'

VOLVER EL RUNTIME AL DE LA RAMA
    git -C ~/dreamcast/libgodc checkout -- runtime/gc_copy.c runtime/gc_heap.c runtime/go-memmove.c
    python3 scripts/console/fix-libgodc-gc.py 256 --build      # deja la cadena documentada y rehace el .a

ORDEN E IDEMPOTENCIA
    Corre la cadena documentada (fix-libgodc-gc.py 256, que a su vez aplica ABI -> interfaces) y despues
    deja PRISTINOS los tres archivos de ESTE instrumento antes de aplicarlo: correrlo dos veces deja el
    mismo sha256.
"""
import hashlib
import os
import pathlib
import subprocess
import sys

args = [a for a in sys.argv[1:] if not a.startswith("--")]
LIB = args[0] if args else "/home/dgalli/dreamcast/libgodc"
AQUI = os.path.dirname(os.path.abspath(__file__))
HEAP = pathlib.Path(LIB + "/runtime/gc_heap.c")
COPY = pathlib.Path(LIB + "/runtime/gc_copy.c")
MEMMOVE = pathlib.Path(LIB + "/runtime/go-memmove.c")
MEMCLR = pathlib.Path(LIB + "/runtime/go-memclr.c")
MIS_ARCHIVOS = ["runtime/gc_heap.c", "runtime/gc_copy.c", "runtime/go-memmove.c", "runtime/go-memclr.c"]

# ------------------------------------------------------------------ el texto que se inyecta
# El instrumento se parte en DOS bloques a proposito: las definiciones van en UN solo archivo
# (gc_heap.c) y los otros dos reciben solo las declaraciones `extern` (si no, los tres archivos definen
# los mismos simbolos y el enlazado se cae por definicion multiple).
DECL = '''/* DC-002.24: la traza del campo `errorMessage` del App -- ver scripts/console/fix-libgodc-campo-traza.py.
 * ACA van solo las DECLARACIONES: las definiciones estan en gc_heap.c (un solo archivo define, los otros
 * dos usan). */
#define CAMPO_OFF   156
#define CAMPO_TAM   168
#define CAMPO_TOPE  150

extern uintptr_t campo_vigilado;

/* del tipo de Go solo hace falta el nombre (los que leen los campos ya traen su cabecera) */
struct __go_type_descriptor;

int campo_pisa(uintptr_t base, size_t size);
int campo_puede(void);
void campo_revisar(const char *sitio);
void campo_anotar(void *user_ptr, size_t size, const struct __go_type_descriptor *t);
void campo_origen(void *user_ptr, size_t size);
void campo_destino(void *dst_user, size_t size, const struct __go_type_descriptor *t, const void *src_user);
void campo_gc(int n, void *from, void *to, size_t tam);
void campo_movido(void *nuevo);
void campo_pila(uintptr_t sp, uintptr_t hi, size_t usado, size_t ventana);
void campo_rechazo(const char *motivo, const void *header);
void campo_llamada(const void *ptr);
void campo_adelantado(const void *ptr, void *nuevo);
void campo_escribir(const char *sitio, uintptr_t antes, uintptr_t despues);
void campo_memmove(void *dst, void *src, uintptr_t n, uintptr_t pc);
void campo_memclr(void *dst, uintptr_t n, uintptr_t pc);
void campo_sq(const char *cuando, void *dst, const void *src, size_t size, int via_sq);
void gc_traza(int n, size_t copiado, size_t antes, uintptr_t from);
void gc_oom(size_t pedido, uintptr_t usado, uintptr_t ptr, uintptr_t limite);
'''

COMM = '''/* DC-002.24: la traza del campo `errorMessage` del App -- ver scripts/console/fix-libgodc-campo-traza.py.
 *
 * Vigila la direccion del objeto de tipo `App` (168 bytes, reconocido por el nombre del descriptor) y
 * avisa cuando algo lo pisa, cuando el GC lo copia, cuando el GC NO lo copia estando en el from-space y
 * cuando el asignador entrega su memoria de nuevo. Tope de lineas para no inundar la consola serie. */
#define CAMPO_OFF   156
#define CAMPO_TAM   168
#define CAMPO_TOPE  150

uintptr_t campo_vigilado = 0;
static uintptr_t campo_antes[2] = {0, 0};
static int campo_lineas = 0;
static int campo_copiado_gc = 0;

int campo_puede(void)
{
    return campo_lineas < CAMPO_TOPE;
}

int campo_pisa(uintptr_t base, size_t size)
{
    if (!campo_vigilado || size == 0)
        return 0;
    /* cubre el App ENTERO, encabezado incluido: el encabezado es justo lo que el GC valida para decidir
       si copia el objeto (si queda en cero, el App se pierde aunque la pila lo referencie). */
    return (base < campo_vigilado + CAMPO_TAM) && (campo_vigilado - GC_HEADER_SIZE < base + size);
}

/* campo_toca_cabecera: el rango dado cae justo sobre el ENCABEZADO del App (vigilado-8 .. vigilado). */
static int campo_toca_cabecera(uintptr_t base, size_t size)
{
    if (!campo_vigilado || size == 0)
        return 0;
    return (base < campo_vigilado) && (base + size > campo_vigilado - GC_HEADER_SIZE);
}

static const char *campo_nombre(const struct __go_type_descriptor *t, char *buf, size_t tam)
{
    if (t && t->__uncommon && t->__uncommon->__name)
    {
        size_t n = (size_t)t->__uncommon->__name->__length;
        if (n > tam - 1)
            n = tam - 1;
        for (size_t i = 0; i < n; i++)
            buf[i] = (char)t->__uncommon->__name->__data[i];
        buf[n] = 0;
        return buf;
    }
    return "-";
}

static int campo_es_app(const struct __go_type_descriptor *t)
{
    char buf[32];
    if (!t || (size_t)t->__size != CAMPO_TAM)
        return 0;
    campo_nombre(t, buf, sizeof(buf));
    return (buf[0] == 'A' && buf[1] == 'p' && buf[2] == 'p' && buf[3] == 0);
}

/* campo_revisar: las dos palabras del campo, ANTES/DESPUES de la operacion que acaba de pasar. */
void campo_revisar(const char *sitio)
{
    if (!campo_vigilado)
        return;
    uintptr_t *w = (uintptr_t *)(campo_vigilado + CAMPO_OFF);
    uintptr_t a = w[0], b = w[1];
    if (a == campo_antes[0] && b == campo_antes[1])
        return;
    if (campo_lineas < CAMPO_TOPE)
    {
        campo_lineas++;
        dbglog(DBG_CRITICAL, "CAMPO-TRAZA cambio tras %s: campo= %08x %08x (antes %08x %08x) pc=%08x\\n",
               sitio, (unsigned)a, (unsigned)b, (unsigned)campo_antes[0], (unsigned)campo_antes[1],
               (unsigned)(uintptr_t)__builtin_return_address(0));
    }
    campo_antes[0] = a;
    campo_antes[1] = b;
}

static int campo_moviendo = 0;

/* campo_movido: la copia del App ya esta hecha; el instrumento SIGUE al App (perseguir la direccion vieja
   es vigilar memoria muerta, que el asignador reparte de nuevo y da falsos positivos). */
void campo_movido(void *nuevo)
{
    if (!campo_moviendo)
        return;
    campo_moviendo = 0;
    if (campo_puede())
    {
        campo_lineas++;
        dbglog(DBG_CRITICAL, "CAMPO-TRAZA el App SE MOVIO de %08x a %08x\\n",
               (unsigned)campo_vigilado, (unsigned)(uintptr_t)nuevo);
    }
    campo_vigilado = (uintptr_t)nuevo;
    uintptr_t *w = (uintptr_t *)(campo_vigilado + CAMPO_OFF);
    campo_antes[0] = w[0];
    campo_antes[1] = w[1];
}

/* campo_pila: la ventana de pila que el GC escanea y DONDE esta el puntero al App. La pila crece hacia
   abajo, asi que los cuadros VIEJOS (main, el arranque) quedan ARRIBA; la ventana arranca en el sp actual
   y sube, con tope GC_STACK_SCAN_MAX. Si el puntero al App esta por encima del tope, el GC no lo ve. */
void campo_pila(uintptr_t sp, uintptr_t hi, size_t usado, size_t ventana)
{
    if (!campo_vigilado || !campo_puede())
        return;
    int dentro = 0, fuera = 0;
    for (uintptr_t *w = (uintptr_t *)sp; (uintptr_t)(w + 1) <= sp + ventana; w++)
        if (*w == campo_vigilado)
            dentro++;
    for (uintptr_t *w = (uintptr_t *)(sp + ventana); (uintptr_t)(w + 1) <= hi; w++)
        if (*w == campo_vigilado)
            fuera++;
    campo_lineas++;
    dbglog(DBG_CRITICAL, "CAMPO-TRAZA pila sp=%08x hi=%08x usado=%u ventana=%u puntero-al-App:"
                         " dentro=%d fuera=%d\\n",
           (unsigned)sp, (unsigned)hi, (unsigned)usado, (unsigned)ventana, dentro, fuera);
}

/* campo_rechazo: el GC siguio un puntero al App y la validacion del encabezado lo RECHAZO (el objeto
   queda como basura aunque alguien lo referencie). Imprime el motivo. */
void campo_rechazo(const char *motivo, const void *header)
{
    if (!campo_vigilado || (uintptr_t)header + GC_HEADER_SIZE != campo_vigilado || !campo_puede())
        return;
    campo_lineas++;
    dbglog(DBG_CRITICAL, "CAMPO-TRAZA rechazo: el encabezado del App (%08x) NO paso la validacion: %s"
                         " (vecinos: %08x %08x | %08x %08x | %08x %08x)\\n",
           (unsigned)(uintptr_t)header, motivo,
           (unsigned)((uintptr_t *)header)[-2], (unsigned)((uintptr_t *)header)[-1],
           (unsigned)((uintptr_t *)header)[0], (unsigned)((uintptr_t *)header)[1],
           (unsigned)((uintptr_t *)header)[2], (unsigned)((uintptr_t *)header)[3]);
}

/* campo_llamada: el GC llego a la puerta de copia con un puntero al App. Lo que pase despues (rechazo,
   adelantado, copiado) lo dicen los otros avisos. */
void campo_llamada(const void *ptr)
{
    if (!campo_vigilado || (uintptr_t)ptr != campo_vigilado || !campo_puede())
        return;
    campo_lineas++;
    dbglog(DBG_CRITICAL, "CAMPO-TRAZA copia: el GC llega al App (%08x)\\n", (unsigned)campo_vigilado);
}

/* campo_adelantado: el objeto ya tenia puntero de adelanto (ya se copio en esta recoleccion). */
void campo_adelantado(const void *ptr, void *nuevo)
{
    if (!campo_vigilado || (uintptr_t)ptr != campo_vigilado || !campo_puede())
        return;
    campo_lineas++;
    dbglog(DBG_CRITICAL, "CAMPO-TRAZA copia: el App ya estaba adelantado hacia %08x\\n",
           (unsigned)(uintptr_t)nuevo);
}

/* campo_escribir: una escritura concreta sobre las palabras del campo del App. */
void campo_escribir(const char *sitio, uintptr_t antes, uintptr_t despues)
{
    if (!campo_puede())
        return;
    campo_lineas++;
    dbglog(DBG_CRITICAL, "CAMPO-TRAZA %s: el campo del App %08x -> %08x\\n",
           sitio, (unsigned)antes, (unsigned)despues);
}

/* campo_memmove: una copia de memoria del codigo Go (structs, interfaces) con destino sobre el App. */
void campo_memmove(void *dst, void *src, uintptr_t n, uintptr_t pc)
{
    if (!campo_pisa((uintptr_t)dst, (size_t)n) || !campo_puede())
        return;
    campo_lineas++;
    dbglog(DBG_CRITICAL, "CAMPO-TRAZA memmove: destino %08x (%u bytes, origen %08x) CUBRE el App %08x pc=%08x\\n",
           (unsigned)(uintptr_t)dst, (unsigned)n, (unsigned)(uintptr_t)src,
           (unsigned)campo_vigilado, (unsigned)pc);
}

/* campo_memclr: un cero-rellenado del lado Go (runtime.memclr*) sobre el App. */
void campo_memclr(void *dst, uintptr_t n, uintptr_t pc)
{
    if (!campo_pisa((uintptr_t)dst, (size_t)n) || !campo_puede())
        return;
    campo_lineas++;
    dbglog(DBG_CRITICAL, "CAMPO-TRAZA memclr: destino %08x (%u bytes) CUBRE el App %08x"
                         " (encabezado=%s) pc=%08x\\n",
           (unsigned)(uintptr_t)dst, (unsigned)n, (unsigned)campo_vigilado,
           campo_toca_cabecera((uintptr_t)dst, (size_t)n) ? "SI" : "no", (unsigned)pc);
}

/* campo_sq: la copia del propio App (encabezado o datos): quien la hace y que quedo escrito en cada lado. */
void campo_sq(const char *cuando, void *dst, const void *src, size_t size, int via_sq)
{
    if (!campo_vigilado || !campo_puede())
        return;
    if ((uintptr_t)src != campo_vigilado - GC_HEADER_SIZE &&
        (uintptr_t)dst != campo_vigilado - GC_HEADER_SIZE)
        return;
    campo_lineas++;
    dbglog(DBG_CRITICAL, "CAMPO-TRAZA sq %s: dst=%08x src=%08x tam=%u via_sq=%d"
                         " dst_cab= %08x %08x src_cab= %08x %08x\\n",
           cuando, (unsigned)(uintptr_t)dst, (unsigned)(uintptr_t)src, (unsigned)size, via_sq,
           (unsigned)((const uintptr_t *)dst)[0], (unsigned)((const uintptr_t *)dst)[1],
           (unsigned)((const uintptr_t *)src)[0], (unsigned)((const uintptr_t *)src)[1]);
}

/* campo_anotar: cada objeto que entrega el asignador del monton de Go. */
void campo_anotar(void *user_ptr, size_t size, const struct __go_type_descriptor *t)
{
    char buf[32];
    if (!campo_vigilado && campo_es_app(t))
    {
        campo_vigilado = (uintptr_t)user_ptr;
        uintptr_t *w = (uintptr_t *)(campo_vigilado + CAMPO_OFF);
        campo_antes[0] = w[0];
        campo_antes[1] = w[1];
        if (campo_lineas < CAMPO_TOPE)
        {
            campo_lineas++;
            dbglog(DBG_CRITICAL, "CAMPO-TRAZA el App quedo vigilado en %08x (%u bytes, tipo %s)\\n",
                   (unsigned)campo_vigilado, (unsigned)size, campo_nombre(t, buf, sizeof(buf)));
        }
        return;
    }

    if (campo_pisa((uintptr_t)user_ptr, size) && campo_lineas < CAMPO_TOPE)
    {
        campo_lineas++;
        dbglog(DBG_CRITICAL, "CAMPO-TRAZA monton: entregado %08x (%u bytes, tipo %s) que CUBRE el App en %08x"
                             " (encabezado=%s) -- el App ya estaba dado por muerto\\n",
               (unsigned)(uintptr_t)user_ptr, (unsigned)size, campo_nombre(t, buf, sizeof(buf)),
               (unsigned)campo_vigilado, campo_toca_cabecera((uintptr_t)user_ptr, size) ? "SI" : "no");
    }

    campo_revisar("gc_alloc");
}

/* campo_origen: el objeto que el GC esta por copiar. Si es el App, el GC SI lo alcanzo. */
void campo_origen(void *user_ptr, size_t size)
{
    if (!campo_vigilado || (uintptr_t)user_ptr != campo_vigilado)
        return;
    campo_copiado_gc = 1;
    campo_moviendo = 1;
    if (campo_lineas < CAMPO_TOPE)
    {
        campo_lineas++;
        dbglog(DBG_CRITICAL, "CAMPO-TRAZA el App SE COPIA (%u bytes) desde %08x -- el GC lo alcanzo\\n",
               (unsigned)size, (unsigned)campo_vigilado);
    }
}

/* campo_destino: el bloque que el GC esta por escribir en el espacio nuevo. Si cae sobre el App, ESE es
   el escritor que se buscaba. */
void campo_destino(void *dst_user, size_t size, const struct __go_type_descriptor *t, const void *src_user)
{
    char buf[32];
    if (!campo_pisa((uintptr_t)dst_user, size))
        return;
    if (campo_lineas < CAMPO_TOPE)
    {
        campo_lineas++;
        dbglog(DBG_CRITICAL, "CAMPO-TRAZA PISADO: el GC escribe un bloque tipo=%s de %u bytes en %08x"
                             " (origen %08x) que CUBRE el App en %08x\\n",
               campo_nombre(t, buf, sizeof(buf)), (unsigned)size, (unsigned)(uintptr_t)dst_user,
               (unsigned)(uintptr_t)src_user, (unsigned)campo_vigilado);
    }
}

/* gc_traza: el tamano del conjunto VIVO que el GC copia en cada recoleccion (la medida que decide si el
   semiespacio alcanza: con la copia por Store Queue ROTA el GC no copiaba casi nada y el semiespacio
   parecia alcanzar). */
static int gc_traza_lineas = 0;

void gc_traza(int n, size_t copiado, size_t antes, uintptr_t from)
{
    if (gc_traza_lineas++ >= 120)
        return;
    dbglog(DBG_CRITICAL, "GC-TRAZA n=%d from=%08x copiado=%u antes=%u\\n",
           n, (unsigned)from, (unsigned)copiado, (unsigned)antes);
}

/* gc_oom: la asignacion que no entro en el semiespacio nuevo (el aviso que dice cuanto hace falta). */
void gc_oom(size_t pedido, uintptr_t usado, uintptr_t ptr, uintptr_t limite)
{
    dbglog(DBG_CRITICAL, "GC-TRAZA OOM: pedido=%u usado=%u ptr=%08x limite=%08x\\n",
           (unsigned)pedido, (unsigned)usado, (unsigned)ptr, (unsigned)limite);
}

/* campo_gc: por recoleccion, que espacio es el from, si el App estaba ahi y si se copio. */
void campo_gc(int n, void *from, void *to, size_t tam)
{
    if (!campo_vigilado)
        return;
    int en_from = ((uintptr_t)campo_vigilado >= (uintptr_t)from) &&
                  ((uintptr_t)campo_vigilado < (uintptr_t)from + tam);
    uintptr_t *w = (uintptr_t *)(campo_vigilado + CAMPO_OFF);
    uintptr_t a = w[0], b = w[1];
    if (campo_lineas < CAMPO_TOPE)
    {
        campo_lineas++;
        dbglog(DBG_CRITICAL, "CAMPO-TRAZA gc n=%d from=%08x to=%08x app=%08x en_from=%s copiado=%s"
                             " cab= %08x %08x campo= %08x %08x\\n",
               n, (unsigned)(uintptr_t)from, (unsigned)(uintptr_t)to, (unsigned)campo_vigilado,
               en_from ? "si" : "no", campo_copiado_gc ? "si" : "no",
               (unsigned)((uintptr_t *)(campo_vigilado - GC_HEADER_SIZE))[0],
               (unsigned)((uintptr_t *)(campo_vigilado - GC_HEADER_SIZE))[1],
               (unsigned)a, (unsigned)b);
    }
    campo_copiado_gc = 0;
    campo_antes[0] = a;
    campo_antes[1] = b;
}
'''

# ---- gc_heap.c: el asignador (llama a campo_anotar), despues de cero-rellenar el objeto
VIEJO_ALLOC = '''    void *user_ptr = gc_get_user_ptr(header);
    memset(user_ptr, 0, aligned_size);
    return user_ptr;
}

/* Allocate without triggering GC - for use during GC or panic */'''
NUEVO_ALLOC = '''    void *user_ptr = gc_get_user_ptr(header);
    memset(user_ptr, 0, aligned_size);
    campo_anotar(user_ptr, aligned_size, type);
    return user_ptr;
}

/* Allocate without triggering GC - for use during GC or panic */'''

VIEJO_OOM = '''        if (gc_heap.alloc_ptr + total_size > gc_heap.alloc_limit)
            runtime_throw("out of memory");'''
NUEVO_OOM = '''        if (gc_heap.alloc_ptr + total_size > gc_heap.alloc_limit)
        {
            /* DC-002.24: la asignacion que no entro en el semiespacio nuevo (el numero que dice cuanto
               hace falta de semiespacio, ahora que las copias del GC SI escriben) */
            gc_oom(total_size, gc_heap.alloc_ptr - gc_heap.space[gc_heap.active_space],
                   (uintptr_t)gc_heap.alloc_ptr, (uintptr_t)gc_heap.alloc_limit);
            runtime_throw("out of memory");
        }'''

VIEJO_ALLOC_NG = '''    void *user_ptr = gc_get_user_ptr(header);
    memset(user_ptr, 0, aligned_size);
    return user_ptr;
}

void registerGCRoots(gc_root_list_t *roots)'''
NUEVO_ALLOC_NG = '''    void *user_ptr = gc_get_user_ptr(header);
    memset(user_ptr, 0, aligned_size);
    campo_anotar(user_ptr, aligned_size, type);
    return user_ptr;
}

void registerGCRoots(gc_root_list_t *roots)'''

# ---- gc_copy.c: la copia de cada objeto (origen y destino)
VIEJO_COPIA = '''    size_t obj_size = GC_HEADER_GET_SIZE(header);

    /*
     * DEFENSIVE ALIGNMENT FIX:'''
NUEVO_COPIA = '''    size_t obj_size = GC_HEADER_GET_SIZE(header);

    /* DC-002.24: el objeto que el GC esta copiando (si es el App, quedo alcanzado desde las raices) */
    campo_origen(gc_get_user_ptr(header), obj_size);

    /*
     * DEFENSIVE ALIGNMENT FIX:'''

VIEJO_ESCRIBE = '''    // Calculate new user pointer
    void *new_ptr = gc_get_user_ptr(new_header);

    // Leave forwarding pointer in old object'''
NUEVO_ESCRIBE = '''    // Calculate new user pointer
    void *new_ptr = gc_get_user_ptr(new_header);

    /* DC-002.24: si el objeto copiado era el App, el instrumento SIGUE al App a su direccion nueva */
    campo_movido(new_ptr);

    // Leave forwarding pointer in old object'''

VIEJO_SQ = '''    gc_sq_copy(new_header, header, obj_size);
    gc_heap.alloc_ptr += aligned_obj_size;'''
NUEVO_SQ = '''    /* DC-002.24: el bloque que se va a escribir en el espacio nuevo. Con el App VIVO vigilado, esto solo
       dispara si un bloque de verdad cae encima del App (la copia va por las Store Queues, no por memcpy). */
    campo_destino(gc_get_user_ptr(new_header), aligned_obj_size, header->type, gc_get_user_ptr(header));

    gc_sq_copy(new_header, header, obj_size);
    gc_heap.alloc_ptr += aligned_obj_size;
    campo_revisar("copia-del-GC");'''

VIEJO_SQC = '''static void gc_sq_copy(void *dst, const void *src, size_t size)
{
    if (size < SQ_MIN_SIZE ||
        ((uintptr_t)dst & 31) != 0 ||
        ((uintptr_t)src & 31) != 0)
    {
        memcpy(dst, src, size);
        return;
    }'''
NUEVO_SQC = '''static void gc_sq_copy(void *dst, const void *src, size_t size)
{
    /* DC-002.24: la copia del App: quien la hace (Store Queue o memcpy) y que quedo escrito */
    int via_sq = !(size < SQ_MIN_SIZE ||
                   ((uintptr_t)dst & 31) != 0 ||
                   ((uintptr_t)src & 31) != 0);
    campo_sq("entrada", dst, src, size, via_sq);

    if (!via_sq)
    {
        memcpy(dst, src, size);
        campo_sq("salida-memcpy", dst, src, size, 0);
        return;
    }'''

VIEJO_SQC_FIN = '''    if (remainder > 0)
        memcpy((uint8_t *)dst + (size - remainder), s, remainder);

    __asm__ volatile("" : : : "memory");
}'''
NUEVO_SQC_FIN = '''    if (remainder > 0)
        memcpy((uint8_t *)dst + (size - remainder), s, remainder);

    __asm__ volatile("" : : : "memory");

    campo_sq("salida-SQ", dst, src, size, 1);
}'''

# ---- gc_copy.c: la puerta de copia y la validacion del encabezado
VIEJO_LLAMADA = '''static void *gc_copy_object(void *ptr)
{
    if (ptr == NULL)
    {
        return NULL;
    }'''
NUEVO_LLAMADA = '''static void *gc_copy_object(void *ptr)
{
    if (ptr == NULL)
    {
        return NULL;
    }

    /* DC-002.24: el GC llego a la puerta de copia con un puntero al App */
    campo_llamada(ptr);'''

VIEJO_ADELANTADO = '''    // Check if already forwarded
    if (GC_HEADER_IS_FORWARDED(header))
    {
        void *new_ptr = GC_HEADER_GET_FORWARD(header);
        return new_ptr;
    }'''
NUEVO_ADELANTADO = '''    // Check if already forwarded
    if (GC_HEADER_IS_FORWARDED(header))
    {
        void *new_ptr = GC_HEADER_GET_FORWARD(header);
        campo_adelantado(ptr, new_ptr);
        return new_ptr;
    }'''

# ---- gc_validate_header: POR QUE rechaza (el motivo, medido)
VIEJO_V1 = '''    if (size < GC_HEADER_SIZE || size > gc_heap.space_size)
        return false;

    /* Size must be aligned to GC_ALIGN */
    if (size & GC_ALIGN_MASK)
        return false;'''
NUEVO_V1 = '''    if (size < GC_HEADER_SIZE || size > gc_heap.space_size)
    {
        campo_rechazo("tamano fuera de rango", header);
        return false;
    }

    /* Size must be aligned to GC_ALIGN */
    if (size & GC_ALIGN_MASK)
    {
        campo_rechazo("tamano no alineado", header);
        return false;
    }'''

VIEJO_V2 = '''        if (fwd < (uintptr_t)to_space ||
            fwd >= (uintptr_t)(to_space + gc_heap.space_size))
        {
            return false;
        }

        /* Forward pointer must also be aligned */
        if (fwd & GC_ALIGN_MASK)
            return false;'''
NUEVO_V2 = '''        if (fwd < (uintptr_t)to_space ||
            fwd >= (uintptr_t)(to_space + gc_heap.space_size))
        {
            campo_rechazo("adelantado hacia un lugar que no es el espacio nuevo", header);
            return false;
        }

        /* Forward pointer must also be aligned */
        if (fwd & GC_ALIGN_MASK)
        {
            campo_rechazo("adelantado no alineado", header);
            return false;
        }'''

VIEJO_V3 = '''            if ((type_addr >= heap0_lo && type_addr < heap0_hi) ||
                (type_addr >= heap1_lo && type_addr < heap1_hi))
            {
                return false;
            }'''
NUEVO_V3 = '''            if ((type_addr >= heap0_lo && type_addr < heap0_hi) ||
                (type_addr >= heap1_lo && type_addr < heap1_hi))
            {
                campo_rechazo("el descriptor de tipo cae en un semiespacio", header);
                return false;
            }'''

VIEJO_V4 = '''            if (type_addr < 0x8C000000 || type_addr >= 0x8D000000)
            {
                return false;
            }'''
NUEVO_V4 = '''            if (type_addr < 0x8C000000 || type_addr >= 0x8D000000)
            {
                campo_rechazo("el descriptor de tipo fuera de la RAM", header);
                return false;
            }'''

VIEJO_V5 = '''            if (type_addr & 0x3)
            {
                return false;
            }'''
NUEVO_V5 = '''            if (type_addr & 0x3)
            {
                campo_rechazo("el descriptor de tipo no alineado", header);
                return false;
            }'''

# ---- gc_copy.c: la recoleccion entera
VIEJO_GC = '''    size_t after_size = gc_heap.alloc_ptr - gc_heap.space[gc_heap.active_space];
    gc_heap.bytes_copied = after_size;'''
NUEVO_GC = '''    /* DC-002.24: el balance de la recoleccion sobre el App */
    campo_gc((int)gc_heap.gc_count, gc_heap.space[old_space], gc_heap.space[new_space], gc_heap.space_size);

    /* DC-002.24: cuanto copio de verdad esta recoleccion (el tamano del conjunto vivo) */
    gc_traza((int)gc_heap.gc_count, gc_heap.alloc_ptr - gc_heap.space[new_space],
             gc_heap.space_size, (uintptr_t)gc_heap.space[old_space]);

    size_t after_size = gc_heap.alloc_ptr - gc_heap.space[gc_heap.active_space];
    gc_heap.bytes_copied = after_size;'''

# ---- gc_copy.c: el campo que el GC actualiza (la escritura de una palabra que apunta al monton)
VIEJO_CAMPO = '''    void *new_ptr = gc_copy_object(old_ptr);

    // Update the field if the object was moved
    if (new_ptr != old_ptr)
    {
        *field = new_ptr;
    }'''
NUEVO_CAMPO = '''    void *new_ptr = gc_copy_object(old_ptr);

    /* DC-002.24: si el GC toca EXACTAMENTE las palabras del campo del App, se imprime el antes/despues */
    if (campo_vigilado && (uintptr_t)field == campo_vigilado + CAMPO_OFF)
        campo_escribir("campo", (uintptr_t)old_ptr, (uintptr_t)new_ptr);

    // Update the field if the object was moved
    if (new_ptr != old_ptr)
    {
        *field = new_ptr;
    }'''

VIEJO_PILA = '''    if (stack_size > GC_STACK_SCAN_MAX)
    {
        stack_size = GC_STACK_SCAN_MAX;
    }

    gc_scan_range_conservative(sp, stack_size);'''
NUEVO_PILA = '''    if (stack_size > GC_STACK_SCAN_MAX)
    {
        stack_size = GC_STACK_SCAN_MAX;
    }

    /* DC-002.24: la ventana escaneada (y el tope, que es lo que puede dejar afuera el cuadro de main) */
    campo_pila((uintptr_t)sp, (uintptr_t)stack_hi, (uintptr_t)stack_hi - (uintptr_t)sp, stack_size);

    gc_scan_range_conservative(sp, stack_size);'''

# ---- go-memclr.c: el cero-rellenado del codigo Go
VIEJO_MEMCLR_INC = '''#include <string.h>
#include <stdint.h>
'''
NUEVO_MEMCLR_INC = '''#include <string.h>
#include <stdint.h>
#include <kos/dbglog.h>

''' + DECL

VIEJO_MEMCLR_1 = '''void runtime_memclr(void *restrict ptr, uintptr_t n)
{
    memset(ptr, 0, n);
}'''
NUEVO_MEMCLR_1 = '''void runtime_memclr(void *restrict ptr, uintptr_t n)
{
    campo_memclr(ptr, n, (uintptr_t)__builtin_return_address(0));
    memset(ptr, 0, n);
}'''

VIEJO_MEMCLR_2 = '''void runtime_memclrNoHeapPointers(void *restrict ptr, uintptr_t n)
{
    memset(ptr, 0, n);
}'''
NUEVO_MEMCLR_2 = '''void runtime_memclrNoHeapPointers(void *restrict ptr, uintptr_t n)
{
    campo_memclr(ptr, n, (uintptr_t)__builtin_return_address(0));
    memset(ptr, 0, n);
}'''

VIEJO_MEMCLR_3 = '''void runtime_memclrHasPointers(void *restrict ptr, uintptr_t n)
{
    memset(ptr, 0, n);
}'''
NUEVO_MEMCLR_3 = '''void runtime_memclrHasPointers(void *restrict ptr, uintptr_t n)
{
    campo_memclr(ptr, n, (uintptr_t)__builtin_return_address(0));
    memset(ptr, 0, n);
}'''

VIEJO_MEMCLR_4 = '''void __go_memclr(void *restrict ptr, uintptr_t n)
{
    memset(ptr, 0, n);
}'''
NUEVO_MEMCLR_4 = '''void __go_memclr(void *restrict ptr, uintptr_t n)
{
    campo_memclr(ptr, n, (uintptr_t)__builtin_return_address(0));
    memset(ptr, 0, n);
}'''

# ---- go-memmove.c: la copia de structs/memoria del codigo Go
VIEJO_MEMMOVE = '''void runtime_memmove(void *dst, void *src, uintptr_t n)
{
    if (dst != src && n > 0)
        memmove(dst, src, n);
}'''
NUEVO_MEMMOVE = '''void runtime_memmove(void *dst, void *src, uintptr_t n)
{
    if (dst != src && n > 0)
    {
        campo_memmove(dst, src, n, (uintptr_t)__builtin_return_address(0));
        memmove(dst, src, n);
        campo_revisar("memmove");
    }
}'''


def sha(p):
    return hashlib.sha256(p.read_bytes()).hexdigest()[:16]


def reemplazar(path, viejo, nuevo):
    t = path.read_text()
    n = t.count(viejo)
    if n != 1:
        sys.exit("ERROR: el texto a reemplazar aparece %d veces (esperaba 1) en %s" % (n, path))
    path.write_text(t.replace(viejo, nuevo, 1))
    print("  ok %s" % path.name)


print("== 1/3 la cadena documentada del runtime (ABI -> interfaces -> GC en %s KB)"
      % os.environ.get("SEMISPACE_KB", "256"))
r = subprocess.run([sys.executable, os.path.join(AQUI, "fix-libgodc-gc.py"),
                    os.environ.get("SEMISPACE_KB", "256"), LIB],
                   capture_output=True, text=True)
if r.returncode != 0:
    print(r.stdout + r.stderr)
    sys.exit("fix-libgodc-gc.py fallo")
print("   " + (r.stdout.strip().splitlines() or ["(sin salida)"])[-1])

subprocess.run(["git", "-C", LIB, "checkout", "--"] + MIS_ARCHIVOS, check=True)
print("   pristinos: " + ", ".join(MIS_ARCHIVOS))

print("== 2/3 la traza del campo")
reemplazar(HEAP, '#include <kos/dbglog.h>\n', '#include <kos/dbglog.h>\n\n' + COMM)
reemplazar(HEAP, VIEJO_ALLOC, NUEVO_ALLOC)
reemplazar(HEAP, VIEJO_ALLOC_NG, NUEVO_ALLOC_NG)
reemplazar(HEAP, VIEJO_OOM, NUEVO_OOM)

reemplazar(COPY, '#include "dc_platform.h"\n', '#include "dc_platform.h"\n#include <kos/dbglog.h>\n\n' + DECL)
reemplazar(COPY, VIEJO_COPIA, NUEVO_COPIA)
reemplazar(COPY, VIEJO_ESCRIBE, NUEVO_ESCRIBE)
reemplazar(COPY, VIEJO_SQ, NUEVO_SQ)
reemplazar(COPY, VIEJO_GC, NUEVO_GC)
reemplazar(COPY, VIEJO_CAMPO, NUEVO_CAMPO)
reemplazar(COPY, VIEJO_PILA, NUEVO_PILA)
reemplazar(COPY, VIEJO_LLAMADA, NUEVO_LLAMADA)
reemplazar(COPY, VIEJO_ADELANTADO, NUEVO_ADELANTADO)
reemplazar(COPY, VIEJO_V1, NUEVO_V1)
reemplazar(COPY, VIEJO_V2, NUEVO_V2)
reemplazar(COPY, VIEJO_V3, NUEVO_V3)
reemplazar(COPY, VIEJO_V4, NUEVO_V4)
reemplazar(COPY, VIEJO_V5, NUEVO_V5)
reemplazar(COPY, VIEJO_SQC, NUEVO_SQC)
reemplazar(COPY, VIEJO_SQC_FIN, NUEVO_SQC_FIN)

reemplazar(MEMMOVE, '#include "type_descriptors.h"\n',
           '#include "type_descriptors.h"\n#include <kos/dbglog.h>\n\n' + DECL)
reemplazar(MEMMOVE, VIEJO_MEMMOVE, NUEVO_MEMMOVE)

reemplazar(MEMCLR, VIEJO_MEMCLR_INC, NUEVO_MEMCLR_INC)
reemplazar(MEMCLR, VIEJO_MEMCLR_1, NUEVO_MEMCLR_1)
reemplazar(MEMCLR, VIEJO_MEMCLR_2, NUEVO_MEMCLR_2)
reemplazar(MEMCLR, VIEJO_MEMCLR_3, NUEVO_MEMCLR_3)
reemplazar(MEMCLR, VIEJO_MEMCLR_4, NUEVO_MEMCLR_4)

print("== 3/3 el runtime")
# Con CON_SQ_FIX=1 se aplica ENCIMA el arreglo de la copia por Store Queue
# (scripts/console/fix-libgodc-sq.py). Sin el, queda el instrumento con el defecto crudo: asi se mide
# "antes" y "despues" con el MISMO instrumento.
if os.environ.get("CON_SQ_FIX") == "1":
    flags = [LIB, "--sin-pristino"]
    flags.append("--sin-inval" if os.environ.get("SQ_SIN_INVAL") == "1" else "--con-inval")
    if os.environ.get("SQ_SIN_SQ") == "1":
        flags.append("--sin-sq")
    if os.environ.get("GC_PERCENT"):
        flags += ["--gc-percent", os.environ["GC_PERCENT"]]
    r = subprocess.run([sys.executable, os.path.join(AQUI, "fix-libgodc-sq.py")] + flags,
                       capture_output=True, text=True)
    if r.returncode != 0:
        print(r.stdout + r.stderr)
        sys.exit("fix-libgodc-sq.py fallo")
    print("   arreglo de la Store Queue aplicado: " + (r.stdout.strip().splitlines() or ["(sin salida)"])[-1])
if "--build" in sys.argv:
    subprocess.run(["bash", "-c", f"rm -f {LIB}/runtime/*.o {LIB}/*.a"], check=True)
    r = subprocess.run(["make", "-C", LIB, "all"], capture_output=True, text=True)
    if r.returncode != 0:
        print(r.stdout[-4000:] + r.stderr[-4000:])
        sys.exit("make fallo")
    print("   make ok: " + (r.stdout.strip().splitlines() or ["(sin salida)"])[-1])
    print(f"   {LIB}/libgodc.a: {os.path.getsize(LIB + '/libgodc.a')} bytes, sha256 {sha(pathlib.Path(LIB + '/libgodc.a'))}")
else:
    print("   falta --build")
print(f"   gc_heap.c sha256={sha(HEAP)}  gc_copy.c sha256={sha(COPY)}  go-memmove.c sha256={sha(MEMMOVE)}")
print("   ahora: reenlazar el motor y correr Flycast; en el log buscar 'CAMPO-TRAZA'")

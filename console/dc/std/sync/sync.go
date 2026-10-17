// Paquete sync de la consola.
//
// COMO CEDE UNA GOROUTINE (medido, no deducido; ver console/dc/poc/goroutines y la evidencia del
// hito): el scheduler de libgodc es COOPERATIVO sobre UN hilo de KOS (scheduler.c usa thd_pass y
// gopark; no crea hilos con thd_create). Lo que se midio en la consola:
//
//   - cuatro goroutines sumando sin candado y SIN ceder  -> 8000 exacto: NO hay preempcion, ni el
//     timer de KOS interrumpe una seccion critica.
//   - thd_pass() desde Go en medio del incremento         -> 8000: thd_pass NO releva goroutines
//     (cede el hilo de KOS a otros hilos de KOS, y no hay ninguno).
//   - un canal CON buffer nunca se llena si nadie lo llena -> no es punto de cesion.
//   - un canal SIN buffer si bloquea, y ahi SI hay relevo (las anotaciones salen intercaladas).
//
// De ahi sale el diseño de este paquete: el candado es un CANAL CON BUFFER 1 -el clasico-. Si esta
// libre, el Lock no cede (y no hace falta que ceda: nadie puede meterse en el medio). Si esta tomado,
// el Lock SE BLOQUEA y ahi el scheduler releva: la otra goroutine sigue. El Unlock devuelve el testigo
// y despierta al que esperaba.
//
// LIMITE DECLARADO: como no hay preempcion, una seccion critica que no ceda nunca no se parte, asi
// que un candado mal puesto puede no notarse hasta que alguien cede adentro (por ejemplo en un canal
// o en un Sleep). Por eso la prueba en contra del hito fuerza la cesion en el medio.
//
// LIMITE DECLARADO (el segundo), y este tiene numero: el canal del candado se crea en el primer uso (no
// hay constructor). En la consola eso no puede fallar: el scheduler es cooperativo sobre un hilo y nadie
// crea el canal en medio de otro (medido). Con goroutines REALES en paralelo SI pasa, y no es teoria:
// dos goroutines que crean el canal a la vez se quedan con DOS testigos distintos, una suelta un candado
// que la otra no ve y una de las dos se cuelga en el Unlock. MEDIDO (DC-002.8b, 2026-10-16): con una
// copia instrumentada del candado, 300 rondas de 16 goroutines sobre un candado SIN preparar dieron 2
// cuelgues y hasta 2 canales creados para el mismo candado; con 2, 4 u 8 goroutines, 0 cuelgues en 300
// rondas. Es raro, y por eso vivio escondido: el difftest del host se colgo 1 vez en 40 corridas de
// "go test -count=20" (una corrida = 20 ejecuciones del test), con la goroutine clavada en
// RWMutex.RUnlock -> Mutex.Unlock -> <-m.libre. Por eso la prueba del host NO se apoya en "correrlo
// muchas veces": prepara el candado antes de usarlo en paralelo y lo EXIGE con Preparado(), para que un
// candado sin preparar falle al instante y con nombre propio en vez de colgarse cada tanto. Un candado
// del motor, en la consola, se toca por primera vez sin paralelismo: el limite no aplica ahi.
//
// La prueba en el HOST compara contra el sync de Go (Mutex, RWMutex, Once, WaitGroup tienen la misma
// semantica observable) y en la CONSOLA corre el POC de goroutines con el contador exacto y el caso
// en contra que pierde incrementos.
package sync

// Mutex: candado exclusivo sobre un canal con buffer 1 (el clasico). No se puede copiar, como en Go.
type Mutex struct {
	libre chan struct{}
}

func (m *Mutex) canal() chan struct{} {
	if m.libre == nil {
		m.libre = make(chan struct{}, 1)
	}
	return m.libre
}

// Lock toma el candado. Si ya hay alguien adentro, se bloquea (y ahi el scheduler releva).
func (m *Mutex) Lock() {
	c := m.canal()
	c <- struct{}{}
}

// Unlock suelta el candado.
//
// LIMITE DECLARADO: soltar un candado que no se tomo no entra en panico (en Go si). En la consola se
// ignora, porque un panico por esto taparia el error de verdad y no hay recuperacion util: queda
// medido en la prueba del host como diferencia conocida.
func (m *Mutex) Unlock() {
	if m.libre == nil {
		return
	}
	<-m.libre
}

// TryLock intenta y devuelve si lo consiguio, sin bloquearse.
func (m *Mutex) TryLock() bool {
	c := m.canal()
	select {
	case c <- struct{}{}:
		return true
	default:
		return false
	}
}

// Preparado dice si el candado ya creo su canal (ver el segundo limite declarado, arriba).
//
// PARA QUE EXISTE: la prueba del host usa goroutines reales y necesita EXIGIR que el candado este
// preparado antes de usarlo en paralelo. Sin esto, un candado sin preparar no falla: se cuelga cada
// tanto (medido: 1 vez en 40 corridas de la suite con -count=20), que es lo peor que le puede pasar a
// una suite, porque el que lo mira cree que fue la maquina.
func (m *Mutex) Preparado() bool { return m.libre != nil }

// RWMutex: en la consola es un candado EXCLUSIVO (RLock y Lock son lo mismo).
//
// LIMITE DECLARADO, y es el limite mas importante de este paquete: en Go dos lectores pueden entrar
// juntos. En la consola el scheduler es cooperativo sobre un hilo, asi que no hay dos cosas corriendo
// a la vez y el resultado observable de proteger con RWMutex es el mismo.
//
// Y ojo con el RLock recursivo: SE TRABA, igual que en Go cuando hay un Lock esperando. Esto costo una
// correccion: la primera version de este comentario decia que aca no se trababa, y el test de limites
// del host se colgo justamente por eso (RLock = Lock sobre un canal con buffer 1: el segundo RLock
// espera un testigo que nadie va a soltar). La declaracion se corrigio y el caso quedo vigilado en la
// prueba diferencial (TestLimitesConocidos).
type RWMutex struct {
	m Mutex
}

func (rw *RWMutex) Lock()         { rw.m.Lock() }
func (rw *RWMutex) Unlock()       { rw.m.Unlock() }
func (rw *RWMutex) RLock()        { rw.m.Lock() }
func (rw *RWMutex) RUnlock()      { rw.m.Unlock() }
func (rw *RWMutex) TryLock() bool { return rw.m.TryLock() }

// Preparado: lo mismo que en Mutex (el RWMutex de la consola es un Mutex exclusivo).
func (rw *RWMutex) Preparado() bool { return rw.m.Preparado() }

// Once: corre la funcion una sola vez, y los que llegan despues ESPERAN a que termine (como Go).
type Once struct {
	m     Mutex
	hecho bool
	listo chan struct{}
}

func (o *Once) Do(f func()) {
	o.m.Lock()
	if o.hecho {
		c := o.listo
		o.m.Unlock()
		if c != nil {
			<-c // ya lo esta haciendo otro: esperar a que termine
		}
		return
	}
	o.hecho = true
	o.listo = make(chan struct{})
	c := o.listo
	o.m.Unlock()
	f()
	close(c)
}

// WaitGroup: el contador y un canal que se CIERRA cuando llega a cero (cerrar despierta a todos los
// que esperan sin bloquear al que cierra).
type WaitGroup struct {
	m     Mutex
	n     int
	aviso chan struct{}
}

func (wg *WaitGroup) Add(delta int) {
	wg.m.Lock()
	wg.n += delta
	wg.m.Unlock()
}

func (wg *WaitGroup) Done() {
	wg.m.Lock()
	wg.n--
	if wg.n == 0 && wg.aviso != nil {
		close(wg.aviso)
		wg.aviso = nil
	}
	wg.m.Unlock()
}

func (wg *WaitGroup) Wait() {
	for {
		wg.m.Lock()
		if wg.n == 0 {
			wg.m.Unlock()
			return
		}
		c := wg.aviso
		if c == nil {
			c = make(chan struct{})
			wg.aviso = c
		}
		wg.m.Unlock()
		<-c // se bloquea: aca el scheduler releva a las goroutines que tienen que hacer el Done
	}
}

// Pool: guarda valores para reusarlos. Es lo que el motor usa (un solo uso medido), sin los
// detalles finos del Pool de Go (que tiene dos copias por procesador y un limpiador por GC).
type Pool struct {
	m      Mutex
	libres []interface{}
	New    func() interface{}
}

func (p *Pool) Get() interface{} {
	p.m.Lock()
	if len(p.libres) > 0 {
		v := p.libres[len(p.libres)-1]
		p.libres = p.libres[:len(p.libres)-1]
		p.m.Unlock()
		return v
	}
	nuevo := p.New
	p.m.Unlock()
	if nuevo != nil {
		return nuevo()
	}
	return nil
}

func (p *Pool) Put(x interface{}) {
	if x == nil {
		return
	}
	p.m.Lock()
	p.libres = append(p.libres, x)
	p.m.Unlock()
}

// LO QUE NO HAY, Y POR QUE: Cond. El motor lo usa UNA vez (medido en el barrido de simbolos) y
// necesita esperar/despertar con predicado; en la consola eso se puede hacer con un canal, pero no
// esta hecho. Si el motor lo pide, va como tarjeta aparte en vez de inventar una version a medias.
// Tampoco hay Map ni los tipos de atomicos de Go (esos estan en sync/atomic).

// ---------------------------------------------------------------- las condiciones
//
// PARA QUE SE USA: d2networking/d2realm/client.go usa una Cond para que un hilo espere a que llegue un
// mensaje sin quedarse dando vueltas gastando el procesador. Es la herramienta clasica para eso.
//
// COMO ESTA HECHA: con un canal por el que espera cada uno. Wait() se anota en la cola, suelta el candado,
// espera su canal y lo vuelve a tomar; Signal() despierta al primero de la cola y Broadcast() a todos.
// Es la misma cuenta que hace Go, con el candado de la cola aparte para que anotarse y despertar no se
// pisen.

// Locker: lo que se puede tomar y soltar (un Mutex, o un RWMutex por su lado de escritura).
type Locker interface {
	Lock()
	Unlock()
}

// Cond: una condicion para esperar a que pase algo.
type Cond struct {
	L       Locker
	cola    []chan struct{}
	candado Mutex
}

// NewCond: arma la condicion, con el candado que la protege.
func NewCond(l Locker) *Cond { return &Cond{L: l} }

// Wait: suelta el candado y espera a que alguien avise. Al volver, el candado esta TOMADO otra vez (eso es
// lo que hace util a la Cond: la condicion se revisa con el candado puesto).
func (c *Cond) Wait() {
	canal := make(chan struct{})
	c.candado.Lock()
	c.cola = append(c.cola, canal)
	c.candado.Unlock()

	c.L.Unlock()
	<-canal
	c.L.Lock()
}

// Signal: despierta a UNO de los que estan esperando (el que llego primero).
func (c *Cond) Signal() {
	c.candado.Lock()
	if len(c.cola) > 0 {
		close(c.cola[0])
		c.cola = c.cola[1:]
	}
	c.candado.Unlock()
}

// Broadcast: despierta a TODOS.
func (c *Cond) Broadcast() {
	c.candado.Lock()
	for _, canal := range c.cola {
		close(canal)
	}
	c.cola = nil
	c.candado.Unlock()
}

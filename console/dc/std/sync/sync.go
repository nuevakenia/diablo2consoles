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
// LIMITE DECLARADO (el segundo): el canal del candado se crea en el primer uso (no hay constructor).
// En la consola eso no puede fallar: el scheduler es cooperativo sobre un hilo y nadie crea el canal
// en medio de otro (medido). Con goroutines REALES en paralelo si podria pasar que dos creen dos
// canales distintos, y por eso el difftest del host prepara el candado antes de usarlo (ver
// difftest/dc0027_test.go). Un candado del motor, en la consola, se toca por primera vez sin
// paralelismo: el limite no aplica ahi.
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

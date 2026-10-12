package d2asset

import "sync"

// keyLocks hands out one mutex per key, so two goroutines that need the same file decode it once (the
// second waits, then finds it in the cache). The zero value is ready to use.
type keyLocks struct {
	mu sync.Mutex
	m  map[string]*keyLock
}

type keyLock struct {
	mu   sync.Mutex
	refs int
}

// lock takes the lock of key and returns the function that releases it.
func (k *keyLocks) lock(key string) (unlock func()) {
	k.mu.Lock()

	if k.m == nil {
		k.m = make(map[string]*keyLock)
	}

	l := k.m[key]
	if l == nil {
		l = &keyLock{}
		k.m[key] = l
	}

	l.refs++
	k.mu.Unlock()

	l.mu.Lock()

	return func() {
		l.mu.Unlock()
		k.mu.Lock()

		if l.refs--; l.refs == 0 {
			delete(k.m, key)
		}

		k.mu.Unlock()
	}
}

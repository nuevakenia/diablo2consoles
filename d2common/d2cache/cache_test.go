package d2cache

import (
	"testing"
)

func TestCacheInsert(t *testing.T) {
	cache := CreateCache(1)
	insertError := cache.Insert("A", "", 1)

	if insertError != nil {
		t.Fatalf("Cache insert resulted in unexpected error: %s", insertError)
	}
}

func TestCacheInsertWithinBudget(t *testing.T) {
	cache := CreateCache(1)
	insertError := cache.Insert("A", "", 2)

	if insertError != nil {
		t.Fatalf("Cache insert resulted in unexpected error: %s", insertError)
	}
}

func TestCacheInsertUpdatesWeight(t *testing.T) {
	cache := CreateCache(2)
	_ = cache.Insert("A", "", 1)
	_ = cache.Insert("B", "", 1)
	_ = cache.Insert("budget_exceeded", "", 1)

	if cache.GetWeight() != 2 {
		t.Fatal("Cache with budget 2 did not correctly set weight after evicting one of three nodes")
	}
}

func TestCacheInsertDuplicateRejected(t *testing.T) {
	cache := CreateCache(2)
	_ = cache.Insert("dupe", "", 1)
	dupeError := cache.Insert("dupe", "", 1)

	if dupeError == nil {
		t.Fatal("Cache insert of duplicate key did not result in any err")
	}
}

func TestCacheInsertEvictsLeastRecentlyUsed(t *testing.T) {
	cache := CreateCache(2)
	// with a budget of 2, inserting 3 keys should evict the last
	_ = cache.Insert("evicted", "", 1)
	_ = cache.Insert("A", "", 1)
	_ = cache.Insert("B", "", 1)

	_, foundEvicted := cache.Retrieve("evicted")
	if foundEvicted {
		t.Fatal("Cache insert did not trigger eviction after weight exceedance")
	}

	// double check that only 1 one was evicted and not any extra
	_, foundA := cache.Retrieve("A")
	_, foundB := cache.Retrieve("B")

	if !foundA || !foundB {
		t.Fatal("Cache insert evicted more than necessary")
	}
}

func TestCacheInsertEvictsLeastRecentlyRetrieved(t *testing.T) {
	cache := CreateCache(2)
	_ = cache.Insert("A", "", 1)
	_ = cache.Insert("evicted", "", 1)

	// retrieve the oldest node, promoting it head so it is not evicted
	cache.Retrieve("A")

	// insert once more, exceeding weight capacity
	_ = cache.Insert("B", "", 1)
	// now the least recently used key should be evicted
	_, foundEvicted := cache.Retrieve("evicted")
	if foundEvicted {
		t.Fatal("Cache insert did not evict least recently used after weight exceedance")
	}
}

func TestClear(t *testing.T) {
	cache := CreateCache(1)
	_ = cache.Insert("cleared", "", 1)
	cache.Clear()
	_, found := cache.Retrieve("cleared")

	if found {
		t.Fatal("Still able to retrieve nodes after cache was cleared")
	}
}

// DC-002.75: el contador de desalojos es lo que se mide en la consola ("cuantas veces evicta cada
// cache"). Cuenta cada entrada desalojada, no cada Insert, y un acierto no lo mueve.
func TestCacheCountsEvictions(t *testing.T) {
	cache := CreateCache(3)

	concreto := cache.(*Cache)

	if concreto.Evictions() != 0 {
		t.Fatalf("un cache recien creado ya cuenta %d desalojos", concreto.Evictions())
	}

	// tres entradas de peso 1 en un presupuesto de 3: no se desaloja nada
	for _, k := range []string{"A", "B", "C"} {
		_ = cache.Insert(k, "", 1)
	}

	if concreto.Evictions() != 0 {
		t.Fatalf("desalojo %d entradas sin pasarse del presupuesto", concreto.Evictions())
	}

	// un acierto no desaloja
	cache.Retrieve("A")

	if concreto.Evictions() != 0 {
		t.Fatalf("un Retrieve conto como desalojo (%d)", concreto.Evictions())
	}

	// la cuarta entrada de peso 1 desaloja exactamente una (la menos usada, que despues de promover A
	// es B) y la que mas pesa desaloja a las que hagan falta
	_ = cache.Insert("D", "", 1)

	if concreto.Evictions() != 1 {
		t.Fatalf("esperaba 1 desalojo, conto %d", concreto.Evictions())
	}

	if _, found := cache.Retrieve("B"); found {
		t.Fatal("se desalojo la entrada equivocada: B (la menos usada) sigue en el cache")
	}

	if _, found := cache.Retrieve("A"); !found {
		t.Fatal("se desalojo A, que era la mas recientemente usada")
	}

	// una entrada que sola pesa mas que todo el presupuesto deja el cache con una sola cosa
	_ = cache.Insert("pesada", "", 10)

	if concreto.Evictions() != 4 {
		t.Fatalf("esperaba 4 desalojos en total, conto %d", concreto.Evictions())
	}

	// y ResetEvictions sirve para medir "durante N cuadros"
	concreto.ResetEvictions()

	if concreto.Evictions() != 0 {
		t.Fatalf("ResetEvictions no volvio el contador a cero (%d)", concreto.Evictions())
	}
}

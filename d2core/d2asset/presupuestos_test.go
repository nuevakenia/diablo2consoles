package d2asset

// DC-002.75 -- los presupuestos de cache, probados en el HOST.
//
// Lo que esta suite defiende son las dos mitades del contrato de la tarjeta:
//   1. en la PC los presupuestos y el peso de cada entrada siguen EXACTAMENTE como estaban (si alguien
//      los toca, esto canta con el numero viejo y el nuevo);
//   2. en la consola quedan acotados y la suma de los caches de datos no pasa del techo de 3 MB.

import "testing"

// restaurarPresupuestosDePC deja el paquete como lo encuentra la PC: los numeros de siempre y peso 1 por
// entrada. Existe para que las pruebas no dependan del orden en que corren.
func restaurarPresupuestosDePC() {
	animationBudget = presupuestoAnimacionesPC
	fontBudget = presupuestoFontesPC
	paletteBudget = presupuestoPaletasPC
	paletteTransformBudget = presupuestoTransformaPC
	dt1Budget = presupuestoDT1sPC
	ds1Budget = presupuestoDS1sPC
	cofBudget = presupuestoCoFsPC
	dccBudget = presupuestoDCCsPC

	pesoEnBytes = false
}

func TestPresupuestosDePCNoCambian(t *testing.T) {
	restaurarPresupuestosDePC()

	// los valores historicos, escritos como estaban en asset_manager.go:1024*1024*128, etc.
	historicos := map[string][2]int{
		"animaciones":      {animationBudget, 1024 * 1024 * 128},
		"dt1s":             {dt1Budget, 4096 * 2048 * 128},
		"ds1s":             {ds1Budget, 4096 * 2048 * 128},
		"cofs":             {cofBudget, 4096 * 2048 * 128},
		"dccs":             {dccBudget, 4096 * 2048 * 128},
		"fuentes":          {fontBudget, 128},
		"paletas":          {paletteBudget, 64},
		"transformaciones": {paletteTransformBudget, 64},
	}

	for nombre, par := range historicos {
		if par[0] != par[1] {
			t.Fatalf("el presupuesto de %s en la PC cambio: era %d y ahora es %d", nombre, par[1], par[0])
		}
	}

	// y en la PC el peso sigue siendo UNA entrada, no bytes: el comportamiento de siempre
	if peso := Pesar(123456); peso != defaultCacheEntryWeight {
		t.Fatalf("en la PC una entrada de 123.456 B se inserto con peso %d (era %d)", peso, defaultCacheEntryWeight)
	}
}

func TestPresupuestosDeConsolaQuedanAcotados(t *testing.T) {
	restaurarPresupuestosDePC()

	defer restaurarPresupuestosDePC()

	AjustarPresupuestosDeConsola()

	p := PresupuestosVigentes()

	if p.Animaciones <= 0 || p.CoFs <= 0 || p.DCCs <= 0 || p.DT1s <= 0 || p.DS1s <= 0 {
		t.Fatalf("quedo un presupuesto de datos en cero: %+v", p)
	}

	// el techo DURO de la tarjeta: la suma de los caches de datos no pasa de 3 MB
	if suma := p.SumaDeLosCachesDeDatos(); suma > techoDeLosCachesDeDatos {
		t.Fatalf("la suma de los caches de datos es %d B y el techo es %d B", suma, techoDeLosCachesDeDatos)
	}

	// las fuentes, paletas y transformaciones NO se tocan (son 128 y 64 entradas, ya de consola)
	if p.Fuentes != presupuestoFontesPC || p.Paletas != presupuestoPaletasPC ||
		p.Transformaciones != presupuestoTransformaPC {
		t.Fatalf("se movieron los presupuestos de fuentes/paletas/transformaciones: %+v", p)
	}

	// y el peso pasa a ser el tamano en bytes
	if peso := Pesar(900000); peso != 900000 {
		t.Fatalf("en la consola una entrada de 900.000 B se inserto con peso %d", peso)
	}

	// una entrada vacia no puede pesar cero (Insert con peso 0 dejaria el cache sin tope)
	if peso := Pesar(0); peso != 1 {
		t.Fatalf("una entrada de 0 B se inserto con peso %d", peso)
	}

	// los caches se crean con esos numeros
	caches := NuevosCachesDeAssets()

	if caches.Dt1s.GetBudget() != p.DT1s || caches.Ds1s.GetBudget() != p.DS1s ||
		caches.Cofs.GetBudget() != p.CoFs || caches.Dccs.GetBudget() != p.DCCs ||
		caches.Animations.GetBudget() != p.Animaciones {
		t.Fatal("los caches no se crearon con los presupuestos vigentes")
	}
}

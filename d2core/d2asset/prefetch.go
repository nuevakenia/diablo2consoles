package d2asset

import (
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2enum"
)

// Background decoding of monster animations (w26 load performance).
//
// The first time a monster type shows a mode (walk, attack, hit, death...) its COF and DCC files are read
// and decoded: 30 to 190 ms on the thread that draws the frame (notes w23-perf). The level's population is
// known when the level is generated, so the game hands the types to PrefetchMonsters, which decodes the
// files into the same caches the game reads (LoadAnimationWithEffect). Nothing else changes: the game
// still asks for the same files in the same order, it just finds them decoded. OD2_PREFETCH=0 turns the
// prefetch off (A/B measurements).

// MonsterPrefetch names one monster type to decode.
type MonsterPrefetch struct {
	Token       string // monstats AnimationDirectoryToken, e.g. "FA"
	WeaponClass string // monstats2 base weapon class, e.g. "HTH"
	Palette     string
	Modes       []d2enum.MonsterAnimationMode // in the order the game is likely to need them
	// Equipment holds the random choices per component (monstats2 EquipmentOptions); the game picks
	// one at spawn with a random draw, so every candidate is decoded and no random draw is made here.
	Equipment [d2enum.CompositeTypeMax][]string
}

// PrefetchEnabled reports whether background decoding is on (OD2_PREFETCH=0 turns it off).
func PrefetchEnabled() bool { return os.Getenv("OD2_PREFETCH") != "0" }

//nolint:gochecknoglobals // process-wide set of types already queued
var (
	prefetchMu   sync.Mutex
	prefetchSeen = map[string]bool{}
)

const (
	neutralWorkers    = 3 // parallel decoders for the modes needed at spawn
	backgroundWorkers = 2 // helper goroutines for the other modes
)

// PrefetchMonsters decodes the neutral mode of each type before it returns (several types in parallel,
// the monsters need it the moment they are spawned), and the other modes on background goroutines.
// With syncNeutral false everything runs in the background. Types queued earlier are skipped. Safe to
// call from any goroutine.
func (am *AssetManager) PrefetchMonsters(list []MonsterPrefetch, syncNeutral bool) {
	if !PrefetchEnabled() {
		return
	}

	todo := newPrefetchTypes(list)
	if len(todo) == 0 {
		return
	}

	if syncNeutral {
		t0 := time.Now()

		runParallel(len(todo), neutralWorkers, func(i int) {
			am.prefetchModes(todo[i], []d2enum.MonsterAnimationMode{d2enum.MonsterAnimationModeNeutral})
		})

		atomic.AddInt64(&neutralNs, int64(time.Since(t0)))
	}

	go runParallel(len(todo), backgroundWorkers, func(i int) {
		am.prefetchModes(todo[i], todo[i].Modes)
	})
}

// newPrefetchTypes drops the types that were queued before and records the new ones.
func newPrefetchTypes(list []MonsterPrefetch) []MonsterPrefetch {
	var todo []MonsterPrefetch

	prefetchMu.Lock()
	defer prefetchMu.Unlock()

	for _, mp := range list {
		key := mp.Token + "|" + mp.WeaponClass + "|" + mp.Palette

		if mp.Token == "" || prefetchSeen[key] {
			continue
		}

		prefetchSeen[key] = true

		todo = append(todo, mp)
	}

	return todo
}

// prefetchModes decodes the layers of the given modes. Errors are ignored: a file that does not load
// here fails the same way when the game asks for it.
func (am *AssetManager) prefetchModes(mp MonsterPrefetch, modes []d2enum.MonsterAnimationMode) {
	c := &Composite{AssetManager: am, baseType: d2enum.ObjectTypeCharacter,
		basePath: baseString(d2enum.ObjectTypeCharacter), token: mp.Token, palettePath: mp.Palette}

	for _, m := range modes {
		c.prefetchMode(m, mp.WeaponClass, &mp.Equipment)
	}
}

// layerValues lists the layer names the game can ask for: createMode uses "lit" until Equip runs (only
// the neutral mode sees that) and the chosen option afterwards; a component without options stays "lit".
func layerValues(options []string, mode d2enum.MonsterAnimationMode) []string {
	var out []string

	seen := map[string]bool{}

	add := func(v string) {
		if v == "" {
			v = "lit"
		}

		if !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}

	if mode == d2enum.MonsterAnimationModeNeutral || len(options) == 0 {
		add("lit")
	}

	for _, o := range options {
		add(o)
	}

	return out
}

// PrefetchModes decodes, on a background goroutine, the layers the composite would load for the given
// modes with its current equipment (the hero: walk, run, first attack, hit...), so the first use of a
// mode does not stall a frame. It works on a snapshot of the equipment and touches nothing the game
// thread owns.
func (c *Composite) PrefetchModes(weaponClass string, modes ...fmt.Stringer) {
	if !PrefetchEnabled() {
		return
	}

	snap := &Composite{AssetManager: c.AssetManager, baseType: c.baseType, basePath: c.basePath,
		token: c.token, palettePath: c.palettePath, equipment: c.equipment}

	go func() {
		for _, m := range modes {
			snap.prefetchLayers(m, weaponClass, func(cofLayerType d2enum.CompositeType) []string {
				v := snap.equipment[cofLayerType]
				if v == "" {
					v = "lit"
				}

				return []string{v}
			})
		}
	}()
}

// prefetchMode mirrors createMode: same COF, same layer files, same palette and draw effect, so the game's
// own request is a cache hit.
func (c *Composite) prefetchMode(mode d2enum.MonsterAnimationMode, weaponClass string,
	equip *[d2enum.CompositeTypeMax][]string) {
	c.prefetchLayers(mode, weaponClass, func(t d2enum.CompositeType) []string { return layerValues(equip[t], mode) })
}

// prefetchLayers loads the COF of a mode and every layer file values(layer type) names.
func (c *Composite) prefetchLayers(mode fmt.Stringer, weaponClass string, values func(d2enum.CompositeType) []string) {
	cofPath := fmt.Sprintf("%s/%s/COF/%s%s%s.COF", c.basePath, c.token, c.token, mode, weaponClass)
	if exists, _ := c.FileExists(cofPath); !exists {
		return
	}

	cof, err := c.LoadCOF(cofPath)
	if err != nil {
		return
	}

	for _, cofLayer := range cof.CofLayers {
		drawEffect := d2enum.DrawEffectNone
		if cofLayer.Transparent {
			drawEffect = cofLayer.DrawEffect
		}

		for _, v := range values(cofLayer.Type) {
			for _, path := range c.layerPaths(cofLayer.Type.String(), v, mode.String(), cofLayer.WeaponClass.String()) {
				if exists, err := c.FileExists(path); !exists || err != nil {
					break // same rule as loadCompositeLayer: a missing path ends the search
				}

				if c.ensureAnimation(path, c.palettePath, drawEffect) == nil {
					break
				}
			}
		}
	}
}

// runParallel runs fn(0..n-1) on at most workers goroutines and waits for all of them.
func runParallel(n, workers int, fn func(i int)) {
	if workers > n {
		workers = n
	}

	next := int64(-1)

	var wg sync.WaitGroup

	for w := 0; w < workers; w++ {
		wg.Add(1)

		go func() {
			defer wg.Done()

			for {
				i := int(atomic.AddInt64(&next, 1))
				if i >= n {
					return
				}

				fn(i)
			}
		}()
	}

	wg.Wait()
}

// ----- main-thread load statistics for the PERF meter -----

//nolint:gochecknoglobals // process-wide counters
var (
	modeLoads    int64
	modeLoadNs   int64
	modeLoadMaxN int64
	neutralNs    int64
	slowLoads    int64
	slowLoadNs   int64
)

// slowModeLoad is the length of a mode build that counts as a hitch (a cache hit takes well under 1 ms).
const slowModeLoad = 2 * time.Millisecond

// noteModeLoad times a createMode call (it runs on the game thread) and returns the function that ends it.
func noteModeLoad(what func() string) func() {
	t0 := time.Now()

	return func() {
		d := int64(time.Since(t0))
		atomic.AddInt64(&modeLoads, 1)
		atomic.AddInt64(&modeLoadNs, d)

		if time.Duration(d) >= slowModeLoad {
			atomic.AddInt64(&slowLoads, 1)
			atomic.AddInt64(&slowLoadNs, d)

			if os.Getenv("OD2_PREFETCH_LOG") != "" {
				fmt.Fprintf(os.Stderr, "PREFETCH slow mode build %s %.1f ms\n", what(), float64(d)/1e6)
			}
		}

		for {
			old := atomic.LoadInt64(&modeLoadMaxN)
			if d <= old || atomic.CompareAndSwapInt64(&modeLoadMaxN, old, d) {
				return
			}
		}
	}
}

// ModeLoadStats describes the animation-mode builds (Composite.SetMode / Equip) so far, for the PERF line.
func ModeLoadStats() string {
	const ms = 1e6

	return fmt.Sprintf("modeloads=%d modeload_total_ms=%.1f slow_loads=%d slow_total_ms=%.1f modeload_max_ms=%.1f prefetch_neutral_ms=%.1f prefetch=%v",
		atomic.LoadInt64(&modeLoads), float64(atomic.LoadInt64(&modeLoadNs))/ms,
		atomic.LoadInt64(&slowLoads), float64(atomic.LoadInt64(&slowLoadNs))/ms,
		float64(atomic.LoadInt64(&modeLoadMaxN))/ms, float64(atomic.LoadInt64(&neutralNs))/ms, PrefetchEnabled())
}

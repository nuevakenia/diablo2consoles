package d2util

import "sync/atomic"

// The simulated game seconds a Gameplay screen has advanced (the sum of its update steps, which at OD2_AUTOSPEED run
// faster than the wall clock). Scenarios measure per-game-second rates against this clock instead of the length of a
// script wait: a wait ends at a frame boundary, and one slow frame of a loaded machine at x8 overshoots it by seconds
// (89-hero-stats flaked: 50 and 79 mana came back where 25 were expected).
var simMicros atomic.Int64

// AddSimSeconds advances the simulated clock.
func AddSimSeconds(dt float64) { simMicros.Add(int64(dt * 1e6)) }

// SimSeconds is the simulated game time so far.
func SimSeconds() float64 { return float64(simMicros.Load()) / 1e6 }

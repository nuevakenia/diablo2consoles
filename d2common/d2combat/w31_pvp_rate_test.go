package d2combat

import "testing"

// Wave 31: the DamageRate of the hitting missile scales the defender's flat damage reductions (0x579ef0 multiplies
// stats 34 and 35 by rate / 1024 when the rate is above 0), and the elements use the magic reduction (stat 35).
func TestPvPFlatReductionPerDamageType(t *testing.T) {
	d := PvPDefender{Reduce: 4, MagicReduce: 10}

	for _, tc := range []struct {
		name  string
		parts PvPParts
		want  int
	}{
		{"physical takes stat 34", PvPParts{Phys: 20}, 16},
		{"magic takes stat 35", PvPParts{Magic: 20}, 10},
		{"fire takes stat 35", PvPParts{Fire: 20}, 10},
		{"cold takes stat 35", PvPParts{Cold: 20}, 10},
		{"lightning takes stat 35", PvPParts{Light: 20}, 10},
		{"a reduction above the hit leaves nothing", PvPParts{Fire: 8}, 0},
	} {
		if got := PvPReceiveParts(tc.parts, d); got != tc.want {
			t.Errorf("%s: %d, want %d", tc.name, got, tc.want)
		}

		if got := (&PvPDefendCarry{}).ReceiveParts(tc.parts, d); got != tc.want {
			t.Errorf("%s (carry): %d, want %d", tc.name, got, tc.want)
		}
	}
}

func TestPvPFlatReductionDamageRate(t *testing.T) {
	// a standing fire row (Fire Wall DamageRate 41): 41 / 1024 of the reduction is paid per hit, about one 25th
	if got := PvPReceiveFlat(10, 0, 100, 41); got != 10-4 { // 100 * 41 / 1024 = 4.0 points off a 10 point hit
		t.Errorf("rate 41: %d, want 6", got)
	}

	if got := PvPReceiveFlat(10, 0, 100, 0); got != 0 { // no rate: whole reduction
		t.Errorf("no rate: %d, want 0", got)
	}

	// 25 hits of a second pay the whole reduction once: 25 * 41 = 1025 = 1024 + 1
	flat := ScaleFlatReduction(40, 41)
	if sum := 25 * flat; sum < 40<<FixedShift-256 || sum > 40<<FixedShift+256 {
		t.Errorf("25 hits pay %d (8.8), want about %d", sum, 40<<FixedShift)
	}

	// the carry path keeps the fraction of the scaled reduction: fire 3 points, reduction 5 * 41 / 1024 = 0.2 point
	d := PvPDefender{MagicReduce: 5, FlatRate: 41, FireResist: 0}

	var c PvPDefendCarry

	total := 0
	for i := 0; i < 50; i++ {
		total += c.ReceiveParts(PvPParts{Fire: 3}, d)
	}

	// 50 * (3 - 0.2 (rounded to hundredths: 0.19)) = 140.5 -> 140 whole points paid, the rest carried
	if total < 139 || total > 141 {
		t.Errorf("50 carried hits: %d, want about 140", total)
	}
}

func TestPvPReceivePartsFlatRateField(t *testing.T) {
	d := PvPDefender{PhysResist: 0, Reduce: 8, FlatRate: 205} // Wake of Fire rate 205: 0.2 of the reduction

	if got := PvPReceiveParts(PvPParts{Phys: 4}, d); got != 2 { // 8 * 205 / 1024 = 1.6; 4 - 1.6 = 2.4 -> 2
		t.Errorf("wake rate: %d, want 2", got)
	}
}

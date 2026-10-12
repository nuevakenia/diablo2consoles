package d2quest

import "testing"

func TestKillForcesSave(t *testing.T) {
	for class, want := range map[int]bool{NPCBaalCrab: true, NPCDiablo: true, 0: false, 1: false, 156: false} {
		if KillForcesSave(class) != want {
			t.Errorf("class %d", class)
		}
	}
}

package d2quest

import "testing"

// The class and level numbers the act 3-5 quests key on (rows of MonStats.txt and
// Levels.txt, checked in w28 against the install's tables).
func TestBossAndLevelIDs(t *testing.T) {
	for name, c := range map[string][2]int{
		"mephisto": {NPCMephisto, 242},
		"izual":    {NPCIzual, 256},
		"baalcrab": {NPCBaalCrab, 544},
		"durance1": {LevelDurance1, 100},
		"durance3": {LevelDurance3, 102},
		"plains":   {LevelPlainsDespair, 105},
		"chaos":    {LevelChaosSanctum, 108},
		"foothill": {LevelBloodyFoothills, 110},
		"keep1":    {LevelWorldstone1, 128},
		"throne":   {LevelThrone, 131},
	} {
		if c[0] != c[1] {
			t.Errorf("%s = %d, want %d", name, c[0], c[1])
		}
	}
}

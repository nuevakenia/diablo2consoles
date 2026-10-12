package d2hero

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2s"
)

// hero builds a valid header-only save; seed makes the versions differ.
func hero(t testing.TB, name string, seed uint32) []byte {
	t.Helper()

	data, err := d2s.NewCharacter(name, d2s.Sorceress, d2s.NewCharacterFlags{Expansion: true, MapSeed: seed,
		Created: time.Unix(1700000000, 0)}, d2s.DefaultAppearance(d2s.Sorceress))
	if err != nil {
		t.Fatal(err)
	}

	return data
}

func readFile(t testing.TB, path string) []byte {
	t.Helper()

	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	return b
}

func noRotationDelay(t *testing.T) {
	old := rotateMinAge
	rotateMinAge = 0

	t.Cleanup(func() { rotateMinAge = old })
}

func listNames(t testing.TB, dir string) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}

	var out []string
	for _, e := range entries {
		out = append(out, e.Name())
	}

	return out
}

func TestValidateD2S(t *testing.T) {
	good := hero(t, "Ok", 1)

	flip := func(i int) []byte {
		b := append([]byte(nil), good...)
		b[i] ^= 0xFF

		return b
	}

	tests := []struct {
		name string
		data []byte
		want error
	}{
		{"valid", good, nil},
		{"empty", nil, d2s.ErrTooShort},
		{"truncated", good[:100], d2s.ErrTooShort},
		{"bad magic", flip(0), d2s.ErrBadMagic},
		{"bad checksum (flipped body byte)", flip(0x40), d2s.ErrBadChecksum},
		{"extra bytes appended (size)", append(append([]byte(nil), good...), 0), d2s.ErrBadChecksum},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateD2S(tt.data)
			if tt.want == nil && err != nil || tt.want != nil && !errors.Is(err, tt.want) {
				t.Fatalf("got %v, want %v", err, tt.want)
			}
		})
	}

	// version outside what the game accepts, with a fixed checksum
	old := append([]byte(nil), good...)
	old[4] = 0x47

	fixed, _, err := d2s.Repair(old)
	if err != nil {
		t.Fatal(err)
	}

	if err := ValidateD2S(fixed); !errors.Is(err, d2s.ErrBadVersion) {
		t.Fatalf("old version accepted: %v", err)
	}
}

func TestCommitD2SBackupsRotate(t *testing.T) {
	noRotationDelay(t)

	dir := t.TempDir()
	path := filepath.Join(dir, "Hero.d2s")

	versions := make([][]byte, 7)
	for i := range versions {
		versions[i] = hero(t, "Hero", uint32(i+1))
	}

	known := ""

	for i, v := range versions {
		sum, err := CommitD2S(path, v, known)
		if err != nil {
			t.Fatalf("save %d: %v", i, err)
		}

		known = sum

		if !bytes.Equal(readFile(t, path), v) {
			t.Fatalf("save %d: file is not the new version", i)
		}

		// no temp files and no more than the documented set
		for _, n := range listNames(t, dir) {
			if strings.Contains(n, ".tmp") {
				t.Fatalf("save %d: temp file left: %s", i, n)
			}
		}
	}

	// the first original, then the three newest previous versions
	if !bytes.Equal(readFile(t, path+BackupSuffix), versions[0]) {
		t.Error(".bak is not the untouched first original")
	}

	for n := 1; n <= BackupCount; n++ {
		want := versions[len(versions)-1-n]
		if !bytes.Equal(readFile(t, BackupPath(path, n)), want) {
			t.Errorf("bak%d is not the version %d saves ago", n, n)
		}
	}

	if _, err := os.Stat(BackupPath(path, BackupCount+1)); err == nil {
		t.Error("more backups than BackupCount")
	}
}

func TestCommitD2SBurstOfSavesKeepsOlderBackups(t *testing.T) {
	old := rotateMinAge
	rotateMinAge = time.Hour

	t.Cleanup(func() { rotateMinAge = old })

	dir := t.TempDir()
	path := filepath.Join(dir, "Hero.d2s")

	var vs [][]byte
	for i := 0; i < 5; i++ {
		vs = append(vs, hero(t, "Hero", uint32(i+1)))
	}

	for _, v := range vs {
		if _, err := CommitD2S(path, v, ""); err != nil {
			t.Fatal(err)
		}
	}

	// five saves inside the rotation interval: bak1 is the previous version, there is no bak2/bak3 yet
	if !bytes.Equal(readFile(t, BackupPath(path, 1)), vs[3]) {
		t.Error("bak1 is not the previous version")
	}

	if _, err := os.Stat(BackupPath(path, 2)); err == nil {
		t.Error("backups shifted inside the interval")
	}
}

func TestCommitD2SRefusesInvalidData(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Hero.d2s")
	good := hero(t, "Hero", 1)

	if _, err := CommitD2S(path, good, ""); err != nil {
		t.Fatal(err)
	}

	bad := append([]byte(nil), good...)
	bad[0x50] ^= 1

	for name, data := range map[string][]byte{"checksum": bad, "empty": nil, "short": good[:50]} {
		if _, err := CommitD2S(path, data, ""); err == nil {
			t.Fatalf("%s: invalid data was written", name)
		}
	}

	if !bytes.Equal(readFile(t, path), good) {
		t.Fatal("a refused write changed the file")
	}
}

func TestCommitD2SDamagedFileIsNotRotatedOverGoodBackups(t *testing.T) {
	noRotationDelay(t)

	dir := t.TempDir()
	path := filepath.Join(dir, "Hero.d2s")

	v := [][]byte{hero(t, "Hero", 1), hero(t, "Hero", 2), hero(t, "Hero", 3), hero(t, "Hero", 4)}
	known := ""

	for _, d := range v[:3] {
		sum, err := CommitD2S(path, d, known)
		if err != nil {
			t.Fatal(err)
		}

		known = sum
	}

	// the file on disk is damaged behind the game's back (known = "": the caller does not know it)
	damaged := append([]byte(nil), v[2]...)
	damaged[0x60] ^= 0x55

	if err := os.WriteFile(path, damaged, 0o600); err != nil {
		t.Fatal(err)
	}

	b1, b2 := readFile(t, BackupPath(path, 1)), readFile(t, BackupPath(path, 2))

	if _, err := CommitD2S(path, v[3], ""); err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(readFile(t, BackupPath(path, 1)), b1) || !bytes.Equal(readFile(t, BackupPath(path, 2)), b2) {
		t.Fatal("a damaged file pushed the good backups out")
	}

	var kept int

	for _, n := range listNames(t, dir) {
		if strings.HasPrefix(n, "Hero.d2s"+CorruptSuffix) {
			kept++

			if !bytes.Equal(readFile(t, filepath.Join(dir, n)), damaged) {
				t.Error("the damaged copy is not the damaged bytes")
			}
		}
	}

	if kept != 1 {
		t.Fatalf("damaged copy kept %d times, want 1", kept)
	}
}

func TestCommitD2SLastWriterProtection(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Hero.d2s")

	base := hero(t, "Hero", 1)

	sumA, err := CommitD2S(path, base, "")
	if err != nil {
		t.Fatal(err)
	}

	sumB := sumA // window B loaded the same file

	// window A saves progress
	a := hero(t, "Hero", 2)
	if sumA, err = CommitD2S(path, a, sumA); err != nil {
		t.Fatal(err)
	}

	// window B (stale) must not overwrite A's save
	b := hero(t, "Hero", 3)
	_, err = CommitD2S(path, b, sumB)

	if !errors.Is(err, ErrConcurrentWrite) {
		t.Fatalf("stale writer: %v", err)
	}

	if !bytes.Equal(readFile(t, path), a) {
		t.Fatal("the stale writer replaced A's save")
	}

	if !bytes.Equal(readFile(t, path+ConflictSuffix), b) {
		t.Fatal("B's version was not kept as a side copy")
	}

	// A continues normally
	if _, err = CommitD2S(path, hero(t, "Hero", 4), sumA); err != nil {
		t.Fatal(err)
	}
}

func TestCommitD2SConcurrentWritersNeverBreakTheFile(t *testing.T) {
	noRotationDelay(t)

	dir := t.TempDir()
	path := filepath.Join(dir, "Hero.d2s")

	if _, err := CommitD2S(path, hero(t, "Hero", 1), ""); err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup

	errs := make(chan error, 64)

	for w := 0; w < 4; w++ {
		wg.Add(1)

		go func(w int) {
			defer wg.Done()

			for i := 0; i < 15; i++ {
				// unknown content: all four always get through
				if _, err := CommitD2S(path, hero(t, "Hero", uint32(100*w+i+10)), ""); err != nil {
					errs <- err
					return
				}
			}
		}(w)
	}

	// a reader never sees a damaged file
	stop := make(chan struct{})
	done := make(chan struct{})

	go func() {
		defer close(done)

		for {
			select {
			case <-stop:
				return
			default:
			}

			if b, err := os.ReadFile(path); err != nil || ValidateD2S(b) != nil {
				errs <- fmt.Errorf("reader saw a bad file: %v", err)
				return
			}
		}
	}()

	wg.Wait()
	close(stop)
	<-done
	close(errs)

	for err := range errs {
		t.Fatal(err)
	}

	for _, p := range append(BackupCandidates(path), path) {
		b, err := os.ReadFile(p)
		if err != nil {
			continue
		}

		if err := ValidateD2S(b); err != nil {
			t.Errorf("%s: %v", filepath.Base(p), err)
		}
	}

	for _, n := range listNames(t, dir) {
		if strings.Contains(n, ".tmp") {
			t.Errorf("temp file left: %s", n)
		}
	}
}

func withAnswer(t *testing.T, answer bool) *int {
	asked := 0
	old := AskUser
	AskUser = func(title, message, no, yes string) bool { asked++; return answer }

	t.Cleanup(func() { AskUser = old })

	return &asked
}

func damage(t *testing.T, path string, how string) []byte {
	t.Helper()

	b := readFile(t, path)

	switch how {
	case "flip":
		b[0x55] ^= 0xFF
	case "truncate":
		b = b[:len(b)-20]
	case "zero":
		b = nil
	}

	if err := os.WriteFile(path, b, 0o600); err != nil {
		t.Fatal(err)
	}

	return b
}

func TestReadD2SFileRecoversFromBackup(t *testing.T) {
	noRotationDelay(t)

	for _, how := range []string{"flip", "truncate", "zero"} {
		t.Run(how, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "Hero.d2s")
			v := [][]byte{hero(t, "Hero", 1), hero(t, "Hero", 2), hero(t, "Hero", 3)}
			known := ""

			for _, d := range v {
				sum, err := CommitD2S(path, d, known)
				if err != nil {
					t.Fatal(err)
				}

				known = sum
			}

			bad := damage(t, path, how)
			asked := withAnswer(t, true)
			f := &HeroStateFactory{}

			got, err := f.ReadD2SFile(path)
			if err != nil {
				t.Fatal(err)
			}

			if *asked != 1 {
				t.Fatalf("asked %d times, want once", *asked)
			}

			if !bytes.Equal(got, v[1]) || !bytes.Equal(readFile(t, path), v[1]) {
				t.Fatal("the newest valid backup (the version before the last) was not restored")
			}

			found := false

			for _, n := range listNames(t, dir) {
				if strings.HasPrefix(n, "Hero.d2s"+CorruptSuffix) {
					found = bytes.Equal(readFile(t, filepath.Join(dir, n)), bad)
				}
			}

			if !found {
				t.Fatal("the damaged file was not kept")
			}
		})
	}
}

func TestReadD2SFileDeclinedOrNoBackupLeavesFileAlone(t *testing.T) {
	noRotationDelay(t)

	dir := t.TempDir()
	path := filepath.Join(dir, "Hero.d2s")
	f := &HeroStateFactory{}

	// no backup at all
	if _, err := CommitD2S(path, hero(t, "Hero", 1), ""); err != nil {
		t.Fatal(err)
	}

	bad := damage(t, path, "flip")
	asked := withAnswer(t, true)

	if _, err := f.ReadD2SFile(path); !errors.Is(err, ErrCorruptNoBackup) {
		t.Fatalf("no backup: %v", err)
	}

	if *asked != 0 {
		t.Fatal("asked to restore although nothing can be restored")
	}

	if !bytes.Equal(readFile(t, path), bad) {
		t.Fatal("the damaged file was touched")
	}

	// a backup exists but the player says no
	if err := os.WriteFile(BackupPath(path, 1), hero(t, "Hero", 9), 0o600); err != nil {
		t.Fatal(err)
	}

	asked = withAnswer(t, false)

	if _, err := f.ReadD2SFile(path); err == nil {
		t.Fatal("declined restore returned data")
	}

	if *asked != 1 || !bytes.Equal(readFile(t, path), bad) {
		t.Fatal("declined restore changed the file")
	}

	for _, n := range listNames(t, dir) {
		if strings.Contains(n, "deleted") || strings.Contains(n, CorruptSuffix) {
			t.Fatalf("unexpected file %s", n)
		}
	}
}

func TestReadD2SFileValidFileIsReadOnly(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Hero.d2s")
	good := hero(t, "Hero", 1)

	if err := os.WriteFile(path, good, 0o600); err != nil {
		t.Fatal(err)
	}

	asked := withAnswer(t, true)

	got, err := (&HeroStateFactory{}).ReadD2SFile(path)
	if err != nil || !bytes.Equal(got, good) || *asked != 0 {
		t.Fatalf("got %v asked=%d", err, *asked)
	}

	if names := listNames(t, dir); len(names) != 1 {
		t.Fatalf("reading created files: %v", names)
	}
}

func TestSaveD2SRefusesIncompleteHero(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Hero.d2s")
	good := hero(t, "Hero", 1)

	if err := os.WriteFile(path, good, 0o600); err != nil {
		t.Fatal(err)
	}

	state := &HeroState{HeroName: "Hero", D2SBase: good, Imported: &ImportedInfo{Source: path},
		LoadProblem: "items unreadable: boom"}

	res, err := (&HeroStateFactory{}).SaveD2S(state)
	if !errors.Is(err, ErrIncompleteLoad) {
		t.Fatalf("got %v", err)
	}

	if res == nil || res.Path != path {
		t.Fatalf("result %+v", res)
	}

	if !bytes.Equal(readFile(t, path), good) || len(listNames(t, dir)) != 1 {
		t.Fatal("an incomplete hero touched the disk")
	}
}

// Every SaveD2S call of the game reaches the file through CommitD2S; no other code writes a .d2s.
func TestOnlyCommitD2SWritesSaves(t *testing.T) {
	root := filepath.Join("..", "..")
	allowed := map[string]bool{"d2s_integrity.go": true, "d2s_save.go": true, "d2s_newchar.go": true}

	err := filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go") {
			return err
		}

		if strings.Contains(p, "d2realm") { // the realm server keeps its own account files
			return nil
		}

		src, rerr := os.ReadFile(p)
		if rerr != nil {
			return rerr
		}

		text := string(src)
		if allowed[filepath.Base(p)] {
			return nil
		}

		for _, bad := range []string{"writeFileAtomic(", "backupOriginal(", "CommitD2S("} {
			if strings.Contains(text, bad) && !strings.Contains(p, "d2hero") {
				t.Errorf("%s calls %s: saves must go through HeroStateFactory.SaveD2S", p, bad)
			}
		}

		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestPlaceholderItemsRules(t *testing.T) {
	mk := func(level, exp uint64, items ...d2s.Item) *d2s.Character {
		return &d2s.Character{Body: &d2s.Body{Attributes: d2s.Attributes{Level: level, Experience: exp}}, Items: items}
	}

	plain := func(code string) d2s.Item {
		return d2s.Item{Code: code + " ", Quality: d2s.QualityNormal, Location: d2s.LocationEquipped}
	}
	magic := plain("rin")
	magic.Quality = d2s.QualityMagic
	socketed := plain("crn")
	socketed.Socketed = true
	stored := plain("kit")
	stored.Location = d2s.LocationStored

	tests := []struct {
		name string
		c    *d2s.Character
		want int
	}{
		{"fresh hero with the old gear", mk(1, 0, plain("crn"), plain("plt"), plain("vbt"), plain("vbl"), plain("lgl"),
			plain("rin"), plain("rin"), plain("amu"), stored), 9},
		{"level 2 is never touched", mk(2, 0, plain("crn")), 0},
		{"experience gained is never touched", mk(1, 1, plain("crn")), 0},
		{"a magic ring is the player's", mk(1, 0, magic), 0},
		{"a socketed item is the player's", mk(1, 0, socketed), 0},
		{"other codes stay", mk(1, 0, plain("clb"), plain("buc"), plain("crn")), 1},
		{"no body (new character)", &d2s.Character{}, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := len(PlaceholderItems(tt.c)); got != tt.want {
				t.Fatalf("got %d, want %d", got, tt.want)
			}
		})
	}
}

// With the real tables: the old first save of a hero is repaired, backups come first, an edited hero is not touched.
func TestRepairPlaceholdersRealTables(t *testing.T) {
	_, tables, _ := realSave(t)

	build := func(level, exp uint64) []byte {
		raw, err := d2s.NewCharacter("Old", d2s.Druid, d2s.NewCharacterFlags{Expansion: true, MapSeed: 7,
			Created: time.Unix(1700000000, 0)}, d2s.DefaultAppearance(d2s.Druid))
		if err != nil {
			t.Fatal(err)
		}

		c, err := d2s.Parse(raw, nil)
		if err != nil {
			t.Fatal(err)
		}

		c.Promote()
		c.Body.SetStat(d2s.StatLevel, level)
		c.Body.SetStat(d2s.StatExperience, exp)

		var id uint32

		mk := func(code string, slot uint8, loc uint8, x uint8) d2s.Item {
			id++

			it, err := d2s.NewItem(d2s.Item{Identified: true, Location: loc, Equipped: slot, X: x, Page: 1, Code: code, ID: id * 77,
				Level: 1, Quality: d2s.QualityNormal, Defense: 5, MaxDurability: 20, Durability: 20}, tables)
			if err != nil {
				t.Fatalf("%s: %v", code, err)
			}

			return it
		}

		c.Items = []d2s.Item{
			mk("crn", 1, d2s.LocationEquipped, 0), mk("plt", 3, d2s.LocationEquipped, 0), mk("rin", 6, d2s.LocationEquipped, 0),
			mk("amu", 2, d2s.LocationEquipped, 0), mk("cap", 0, d2s.LocationStored, 4), // cap is the player's
		}

		out, err := d2s.Write(c, tables)
		if err != nil {
			t.Fatal(err)
		}

		return out
	}

	fixed, codes, err := RepairPlaceholders(build(1, 0), tables)
	if err != nil {
		t.Fatal(err)
	}

	if len(codes) != 4 {
		t.Fatalf("removed %v", codes)
	}

	back, err := d2s.Parse(fixed, tables)
	if err != nil || len(back.Items) != 1 || strings.TrimSpace(back.Items[0].Code) != "cap" {
		t.Fatalf("after repair: %v %+v", err, back.Items)
	}

	if again, codes, _ := RepairPlaceholders(fixed, tables); again != nil || codes != nil {
		t.Fatal("a repaired hero offered a second repair")
	}

	if out, _, err := RepairPlaceholders(build(2, 0), tables); err != nil || out != nil {
		t.Fatalf("level 2 hero repaired: %v", err)
	}

	if out, _, err := RepairPlaceholders(build(1, 5), tables); err != nil || out != nil {
		t.Fatalf("hero with experience repaired: %v", err)
	}

	// through the loader: only when the player agrees, with a backup first
	dir := t.TempDir()
	path := filepath.Join(dir, "Old.d2s")
	orig := build(1, 0)

	if err := os.WriteFile(path, orig, 0o600); err != nil {
		t.Fatal(err)
	}

	f := &HeroStateFactory{d2sTables: tables}

	asked := withAnswer(t, false)
	if got, _ := f.ReadD2SFile(path); !bytes.Equal(got, orig) || *asked != 1 || !bytes.Equal(readFile(t, path), orig) {
		t.Fatal("declined repair changed the hero")
	}

	if _, err := os.Stat(path + NoRepairSuffix); err != nil {
		t.Fatal("the decline was not remembered")
	}

	withAnswer(t, true)
	asked = withAnswer(t, true)

	if got, _ := f.ReadD2SFile(path); !bytes.Equal(got, orig) || *asked != 0 {
		t.Fatal("a declined repair was offered again")
	}

	if err := os.Remove(path + NoRepairSuffix); err != nil {
		t.Fatal(err)
	}

	got, err := f.ReadD2SFile(path)
	if err != nil || bytes.Equal(got, orig) || !bytes.Equal(got, readFile(t, path)) {
		t.Fatalf("accepted repair: %v", err)
	}

	if !bytes.Equal(readFile(t, BackupPath(path, 1)), orig) || !bytes.Equal(readFile(t, path+BackupSuffix), orig) {
		t.Fatal("no backup of the hero before the repair")
	}
}

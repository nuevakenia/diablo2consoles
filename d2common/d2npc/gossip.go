package d2npc

// The NPC state array of Game.exe (0x725968, count at 0x724664 = 46, 0x16 bytes per entry) is the client's
// per-NPC speech pool: +0 class id, +4 act, +5 pointer to an array of 15 byte records, +9 record count, +0xd the
// index chosen for "gossip", +0x11 "spoke, roll again", +0x12 "return greeting pending", +0x14 "no introduction
// row" and +0x15 "greeting still allowed". A record is {u16 message id, u8 gated, u32 wanted flag, u32 quest,
// u32 hero class (7 = any)}; the message id is the string.tbl index of the line (and the key of the
// Sounds.txt voice, see d2quest.SoundForMessage). Record 0 is the introduction, record 1 the introduction
// for the hero class named in its class field, records 2.. the gossip pool.

// AnyClass is the class value of a record that fits every hero class.
const AnyClass = 7

// Record is one line of an NPC's pool.
type Record struct {
	Msg int
	// Gated: the line needs quest flag bit 0 of Quest to equal Want.
	Gated bool
	Want  bool
	Quest int
	// Class is the hero class (0 ama, 1 sor, 2 nec, 3 pal, 4 bar, 5 dru, 6 asn) the line is for, or AnyClass.
	Class int
}

// Entry is one NPC of the state array.
type Entry struct {
	Class, Act int
	// NoIntro is the +0x14 byte: the Talk submenu has no "introduction" row.
	NoIntro bool
	// Flag13 is the +0x13 byte (UNVERIFIED use; set for Warriv 1, Meshif 1, Tyrael 2 and 3).
	Flag13  bool
	Records []Record
}

// EntryFor finds the NPC's entry (first match by class, as UI_StartNpcInteraction does).
func EntryFor(class int) (*Entry, bool) {
	for i := range gossipEntries {
		if gossipEntries[i].Class == class {
			return &gossipEntries[i], true
		}
	}

	return nil, false
}

// EntryCount is the number of entries of the state array (DAT_00724664 = 46).
func EntryCount() int { return len(gossipEntries) }

// MessageKey is the string.tbl key of a record's message, "" if unknown.
func MessageKey(msg int) string { return messageKeys[msg] }

// Eligible is the test of UI_InitNpcMenuEntry for one record: the hero class fits and a gated line's quest
// flag has the wanted value.
func (r Record) Eligible(heroClass int, q QuestFlag) bool {
	if r.Class != heroClass && r.Class != AnyClass {
		return false
	}

	if !r.Gated {
		return true
	}

	return (q != nil && q(r.Quest, 0)) == r.Want
}

// gossipTries is the number of rolls before the picker gives up (a countdown from 10).
const gossipTries = 10

// gossipFallback is the index chosen when no roll fits (*(entry+0xd) = 2).
const gossipFallback = 2

// PickGossip is UI_InitNpcMenuEntry (0x4ade00): up to ten rolls of rng(count) (the hero's own seed, see Seed);
// a roll counts when its index is above 1 (records 0 and 1 are the introductions), the hero class fits and a
// gated line's quest bit has the wanted value. After ten failures the index is 2.
//
// Jerhyn's array (0x724dc8) has an extra test on the line with message id 0xff; both of its branches accept the
// roll (the quest bit only changes the returned value, which the caller ignores), so it is not modelled.
func PickGossip(e *Entry, heroClass int, q QuestFlag, rng func(n int) int) int {
	n := len(e.Records)

	for try := 0; try < gossipTries; try++ {
		r := 0
		if n > 0 {
			r = rng(n)
		}

		if r > 1 && e.Records[r].Eligible(heroClass, q) {
			return r
		}
	}

	return gossipFallback
}

// IntroIndex is the "introduction" callback 0x4b0a00: Malah shows record 15 to a barbarian, Nihlathak record 11
// to a necromancer, Qual-Kehk record 10 to a paladin (the three Act 5 lines that name the class); every other
// case shows record 1 when the hero is of the class of record 1, else record 0.
func IntroIndex(e *Entry, heroClass int) int {
	switch e.Class {
	case ClassMalah:
		if heroClass == 4 {
			return 15
		}
	case ClassNihlathak:
		if heroClass == 2 {
			return 11
		}
	case ClassQualKehk:
		if heroClass == 3 {
			return 10
		}
	}

	if len(e.Records) > 1 && heroClass == e.Records[1].Class {
		return 1
	}

	return 0
}

// Seed is the generator of the unit seed (qwSeed) the picker rolls with: seed = lo * 0x6ac690c5 + hi, the new
// low half is the roll (modulo n; for a power of two the exe masks, which is the same).
type Seed struct{ V uint64 }

// Below rolls a number in [0, n); 0 for n < 1.
func (s *Seed) Below(n int) int {
	if n < 1 {
		return 0
	}

	lo, hi := uint32(s.V), uint32(s.V>>32)
	s.V = uint64(lo)*0x6ac690c5 + uint64(hi)

	return int(uint32(s.V) % uint32(n))
}

// TalkRow is a row of the Talk submenu (UI_TalkTopicMenu 0x4b20f0).
type TalkRow int

// The rows of the Talk submenu in the order the exe adds them.
const (
	TalkIntroduction TalkRow = iota // string 0xd47, unless the entry's +0x14 byte is set
	TalkGossip                      // string 0xd43
	TalkQuestTopic                  // one per type 2 entry of the NPC's quest dialog (not listed by TalkRows)
	TalkMerchants                   // string 0xd40, Jerhyn only
	TalkCube                        // string 0x8b7 (the Horadric Cube), the five Cain rows when the hero carries a cube
)

// TalkRows lists the fixed rows of the Talk submenu for a class, in order (the quest topics go between Gossip
// and the class rows). hasCube is whether the hero carries a Horadric Cube ("box").
func TalkRows(class int, hasCube bool) []TalkRow {
	e, ok := EntryFor(class)
	if !ok {
		return nil
	}

	var rows []TalkRow

	if !e.NoIntro {
		rows = append(rows, TalkIntroduction)
	}

	rows = append(rows, TalkGossip)

	if class == ClassJerhyn {
		rows = append(rows, TalkMerchants)
	}

	switch class {
	case ClassCain2, ClassCain3, ClassCain4, ClassCain5, ClassCain6:
		if hasCube {
			rows = append(rows, TalkCube)
		}
	}

	return rows
}

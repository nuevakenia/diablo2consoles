package d2level

import "fmt"

// Act cinematics (Game.exe). VERIFIED from the exe:
//   - The movie table at 0x721270 has 8 rows of 12 bytes (index, name pattern, flags): 0 d2_promo, 1 d2intro (Act 1
//     start), 2 Act02start, 3 Act03start, 4 Act04start, 5 Act04end, 6 D2X_Intro, 7 D2X_Out. Rows 1 to 6 carry the
//     flags 0x0101 (localized folder, resolution suffix "640x292", or "640x146" in the small mode).
//   - The server asks the client to play one with the 2-byte packet 0x61 (SCMD_SendOp61 0x53b700), the second byte
//     is the table index. The senders: QUESTS_OnActChangeNpcTravel 0x5446e0 (Warriv class 155 sends 2, Meshif
//     class 210 sends 3, Tyrael class 367 sends 5), QUESTS_OnHellPortalActChange (Durance level 102 portal sends 4),
//     QUEST_A4_Act4ToAct5TravelHandler (sends 5), and the Baal kill code at 0x58c595 (sends 7).
//   - Each sender first sets the "act finished" slot (7, 15, 28 for the NPCs, 23 for the portal) and only sends the
//     packet when that slot was not done yet in the difficulty AND the player's play flag (PlayerData+0x4c) is not
//     already 1; it then sets the flag. So a cinematic plays once per game session and once per difficulty
//     (slot already done on a second trip: no movie). The Baal sender sets the flag without testing it.
//   - Index 5 is followed by index 6 in the expansion (MAIN_PlayPromoMovieByIndex 0x47ee50 plays the Lord of
//     Destruction intro right after the Act IV end movie when its LoD flag DAT_007b4a60 is set).
//   - Index 7 is played only when a file named d2game.dll exists in the install folder (the exe checks it;
//     the meaning of that check is UNVERIFIED).
// UNVERIFIED: the check that the 0x16f branch needs a non-zero game field at +0x? (the decompiler shows
// dwGameSeed); modelled as "the game is an expansion game".

// The movie table indices.
const (
	MoviePromo = iota
	MovieAct1Start
	MovieAct2Start
	MovieAct3Start
	MovieAct4Start
	MovieAct4End
	MovieLodIntro
	MovieLodOut
	NumMovies
)

var movieFile = [NumMovies]string{
	"d2_promo.bik", "d2intro%s.bik", "ACT02START%s.BIK", "ACT03START%s.BIK", "ACT04START%s.BIK", "ACT04END%s.BIK",
	"D2X_INTRO_%s.BIK", "D2X_OUT_%s.BIK",
}

// MovieFile returns the file name of table row i for a resolution suffix ("640x292"; "640x146" in the small mode).
// The promo row has no suffix.
func MovieFile(i int, suffix string) string {
	if i < 0 || i >= NumMovies {
		return ""
	}

	if i == MoviePromo {
		return movieFile[i]
	}

	return fmt.Sprintf(movieFile[i], suffix)
}

// ActMovie is what an act change sends.
type ActMovie struct {
	Movie int
	// Then is the movie that follows at once (-1: none): the LoD intro after the Act IV end movie.
	Then int
}

// ActChangeMovie answers which movie the exe sends for a trip, and whether it sends one at all. slotDone says the
// act's "finished" quest slot (7, 15, 23 or 28) was already done before this trip; played is the player's play
// flag (a movie was already sent in this session). from/to are acts (1..5); via is how the trip is made.
func ActChangeMovie(from, to int, via Via, slotDone, played, expansion bool) (ActMovie, bool) {
	if slotDone || played {
		return ActMovie{}, false
	}

	switch {
	case from == 1 && to == 2 && via == ViaNPC:
		return ActMovie{Movie: MovieAct2Start, Then: -1}, true
	case from == 2 && to == 3 && via == ViaNPC:
		return ActMovie{Movie: MovieAct3Start, Then: -1}, true
	case from == 3 && to == 4 && via == ViaPortal:
		return ActMovie{Movie: MovieAct4Start, Then: -1}, true
	case from == 4 && to == 5 && expansion:
		return ActMovie{Movie: MovieAct4End, Then: MovieLodIntro}, true
	}

	return ActMovie{}, false
}

// BaalMovie is the movie sent when Baal dies in the expansion: D2X_Out, regardless of the play flag.
func BaalMovie() ActMovie { return ActMovie{Movie: MovieLodOut, Then: -1} }

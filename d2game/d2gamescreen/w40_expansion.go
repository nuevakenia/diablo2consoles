package d2gamescreen

import (
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2resource"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2asset"
)

// expansionInstalled mirrors the exe's MAIN_CheckExpansionMpq choice for the front-end screens: the expansion
// credits background only exists in the Lord of Destruction archives.
func expansionInstalled(am *d2asset.AssetManager) bool {
	ok, err := am.FileExists(d2resource.CreditsBackground)

	return err == nil && ok
}

// cinematicRows says which LoD rows (Act V and the last cinematic) the cinematics menu shows. A classic install
// cannot play them (no d2xvideo). Whether the original hides them or greys them, and the row order itself, live in
// the menu library, which is not in the Game.exe project: UNVERIFIED (wave 40).
func cinematicRows(expansion bool) (showA5, showLastCine bool) { return expansion, expansion }

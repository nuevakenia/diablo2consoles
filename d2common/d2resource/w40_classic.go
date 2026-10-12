package d2resource

// Classic (no Lord of Destruction) variants of the credits and cinematics screens. VERIFIED in wave 40 in the exe:
// UI_LoadIntroLogoScreens 0x429ec0 loads creditsbckg (classic) or creditsbckgexpand, and CinematicsSelectionEXP
// (expansion only, next to CinematicsSelection); UI_LoadCreditsText 0x42c3d0 reads Credits.txt or ExpansionCredits.txt
// by the same expansion check (MAIN_CheckExpansionMpq).
const (
	CreditsBackgroundClassic    = "/data/global/ui/CharSelect/creditsbckg.dc6"
	CreditsTextClassic          = "/data/local/ui/" + LanguageTableToken + "/Credits.txt"
	CinematicsBackgroundClassic = "/data/global/ui/FrontEnd/CinematicsSelection.dc6"
)

// CreditsPaths returns the credits background and text file for an expansion or classic install.
func CreditsPaths(expansion bool) (background, text string) {
	if expansion {
		return CreditsBackground, CreditsText
	}

	return CreditsBackgroundClassic, CreditsTextClassic
}

// CinematicsBackgroundFor returns the cinematics menu background for an install.
func CinematicsBackgroundFor(expansion bool) string {
	if expansion {
		return CinematicsBackground
	}

	return CinematicsBackgroundClassic
}

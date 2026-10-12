// Package d2loadscreen holds the numbers and the order of the loading screen of Game.exe 1.14b (wave 24).
//
// VERIFIED = read in the decompile: GAME_DrawLoadingScreen 0x451c90, UI_LoadLoadingScreenPath 0x451c10,
// GAME_InitClientSession 0x449900, CLIENT_LoadActDungeon 0x449800, MSG_WaitForSaveSendProgress 0x473ea0,
// GFX_PreloadNearbyRoomUnitGraphics 0x46bb50.
package d2loadscreen

// File names of the animation: the English exe reads data\global\ui\Loading\Loadingscreen, every other language
// data\local\UI\LoadingScreen (the format "%s\UI\LoadingScreen"; the language folder comes from the file mapping).
const (
	PathGlobal = `data\global\ui\Loading\Loadingscreen`
	PathLocal  = `data\local\UI\LoadingScreen`
)

// PathFor returns the animation path; english is the string table's language id 0.
func PathFor(english bool) string {
	if english {
		return PathGlobal
	}

	return PathLocal
}

// The picture stands with its left edge 0x80 left of the screen centre and its BOTTOM edge 0x80 below the centre (the cel
// is drawn bottom-up), so a 256 x 256 frame is centred. VERIFIED (GFX_DrvSlot84 at w/2-0x80, h/2+0x80).
const (
	PictureHalfWidth = 0x80
	PictureBottom    = 0x80
)

// Rect is a screen rectangle (left, top, right, bottom; right and bottom exclusive as the exe passes them).
type Rect struct{ L, T, R, B int }

// Layout is where the parts stand on a screen of width w and height h.
type Layout struct {
	// PictureX, PictureBottomY: where the cel is drawn (left edge, bottom edge).
	PictureX, PictureBottomY int
	// Frame: the yellow outline of the bar and Fill the black box inside it (VERIFIED rectangles, colour indexes: yellow =
	// the palette entry nearest (255,255,0), fill 0).
	Frame, Fill Rect
	// BarHeight is the height of the progress bar drawn inside Fill (0x10, VERIFIED); its x/y origin was not read
	// (UNVERIFIED: the left/top of Fill).
	BarHeight int
}

// LayoutFor computes the layout.
func LayoutFor(w, h int) Layout {
	cx, cy := w/2, h/2

	return Layout{
		PictureX:       cx - PictureHalfWidth,
		PictureBottomY: cy + PictureBottom,
		Frame:          Rect{cx - 0x82, cy + 0x9e, cx + 0x82, cy + 0xb2},
		Fill:           Rect{cx - 0x80, cy + 0xa0, cx + 0x80, cy + 0xb0},
		BarHeight:      0x10,
	}
}

// BarMax is the width of the full bar in pixels: the ratio the bar draws goes up to 256 (the width of Fill).
const BarMax = 0x100

// ShowsBar tells whether the bar is drawn: only in the panel layout modes 6..9 (UI_GetPanelLayoutMode; the high resolution
// modes, UNVERIFIED which resolutions) and only while the "local session" value (DAT_0079a840, a copy of DAT_007986ac
// taken when the path was loaded) is 0.
func ShowsBar(panelMode int, sessionValue int) bool {
	return panelMode >= 6 && panelMode <= 9 && sessionValue == 0
}

// Frames is the animation state: every call of the draw function with its argument set moves one frame on, and a frame
// number past the end of the cel is pulled back to the last frame (the animation does not loop). VERIFIED.
type Frames struct {
	Count int
	Index int
}

// Draw returns the frame to draw now and then advances when advance is set.
func (f *Frames) Draw(advance bool) int {
	if f.Count > 0 && f.Index >= f.Count {
		f.Index = f.Count - 1
	}

	shown := f.Index
	if advance {
		f.Index++
	}

	return shown
}

// SaveSend is the progress of the hero upload that follows the first screen when the game mode is one of 6..9
// (MSG_SendLocalSaveToServer, called from GAME_InitClientSession): per chunk the exe draws the screen 8 times, 250 ms apart
// (a 2 s wait per call), with the bar ratio = (chunk << 11 + 0x100 * step) / (total * 8), i.e. it rises by 256 / total over
// the eight steps of one chunk and reaches 256 when the last chunk is done. step runs 0..7.
const (
	SaveSendSteps    = 8
	SaveSendStepWait = 250 // milliseconds
)

// SaveSendRatio returns the bar ratio (0..256) for chunk number chunk (of total) at sub-step step (0..7). total 0 gives 0.
func SaveSendRatio(chunk, total, step int) int {
	if total <= 0 {
		return 0
	}

	return (chunk<<11 + 0x100*step) / (total * 8)
}

// Stage is one stage of the load, in the exe's order.
type Stage struct {
	Name string
	// Draws is true when GAME_DrawLoadingScreen is called at this point.
	Draws bool
	Note  string
}

// Stages is the order of GAME_InitClientSession followed by CLIENT_LoadActDungeon (VERIFIED order). The screen is drawn
// right after the loading picture is loaded, again after the dungeon of the act is built, and during the preload of the
// graphics of the units in the rooms near the hero (every 32nd room unit while a preload is pending).
//
//nolint:gochecknoglobals // order of the exe
var Stages = []Stage{
	{"reset session counters", false, "ten session globals cleared"},
	{"client pool", false, `pool "Client Pool System", 0x40000 bytes in blocks of 0x1000`},
	{"reload panels for the player", false, ""},
	{"read debug flag file", false, ""},
	{"create view", false, "once"},
	{"load resolution options", false, ""},
	{"load loading screen cel and palette", false, "frame counter reset to 0"},
	{"draw loading screen", true, "first frame"},
	{"send local save to server", false, "only in game modes 6..9; shows the bar, 8 draws of 250 ms per chunk"},
	{"palette shift tables, item palettes, memory check", false, ""},
	{"client tables: player slots, environment, unit hash tables, light map, chat input", false, ""},
	{"load act DT1 tile set", false, "CLIENT_LoadActDungeon"},
	{"free previous act, create dungeon", false, "difficulty byte is the one of the game flags packet"},
	{"environment phase and tick for act 1 on the first load", false, "only the very first time"},
	{"set act palette index", false, ""},
	{"draw loading screen", true, "advance one frame"},
	{"automap marker colours, light callback, interface colours", false, ""},
	{"free and reload common COF cache", false, ""},
	{"preload room unit graphics", true, "draws every 32nd room unit"},
}

// MinTickAfterLoadMs: after a dungeon load the exe raises a tick floor (DAT_00798544) to now + 10000 ms if it is lower.
// What the floor throttles was not traced (UNVERIFIED).
const MinTickAfterLoadMs = 10000

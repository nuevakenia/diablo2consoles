package d2util

import "os"

// QuietEnabled reports OD2_QUIET (anything but "" or "0"): a scripted run that must not disturb the person who
// plays on the same machine. The window starts behind the other windows, never takes the focus, ignores the real
// mouse and keyboard (the scripts inject their input into the input manager, not into the window system) and,
// on macOS, has no Dock icon. Rendering, timing and screenshots are unchanged. See scripts/verify.sh.
func QuietEnabled() bool {
	v := os.Getenv("OD2_QUIET")

	return v != "" && v != "0"
}

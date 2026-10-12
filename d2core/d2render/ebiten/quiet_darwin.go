package ebiten

/*
#cgo LDFLAGS: -framework Cocoa
#include "quiet_darwin.h"
*/
import "C"

// quietWindow is called for the first frames and now and then later, because the first frames may run before
// the native window exists and the system may bring it forward once more while it opens.
func quietWindow() { C.quietApply() }

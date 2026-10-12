//go:build darwin

package main

import (
	"os"
	"path/filepath"
	"strings"
)

const (
	currentLogName  = "OpenDiablo2.log"
	previousLogName = "OpenDiablo2.previous.log"
	crashLogName    = "OpenDiablo2.crash.log"
)

// logHasCrash reports whether text holds one of the lines the runtime or main.go write when the engine dies.
func logHasCrash(text string) bool {
	for _, m := range crashMarkers {
		if strings.Contains(text, m) {
			return true
		}
	}

	return false
}

// rotateLogs moves the log of the last session aside before a new one starts: OpenDiablo2.log becomes
// OpenDiablo2.previous.log (the last session), and when that session crashed it is also kept as
// OpenDiablo2.crash.log, so a quick relaunch after a crash (which makes a clean "previous" log) does not lose the
// evidence. A missing or empty log is not rotated (an empty one would wipe the last real session).
func rotateLogs(dir string) {
	cur := filepath.Join(dir, currentLogName)

	data, err := os.ReadFile(cur) //nolint:gosec // user log
	if err != nil || len(data) == 0 {
		return
	}

	if logHasCrash(string(data)) {
		_ = os.WriteFile(filepath.Join(dir, crashLogName), data, 0o640) //nolint:gosec // user log
	}

	_ = os.Rename(cur, filepath.Join(dir, previousLogName))
}

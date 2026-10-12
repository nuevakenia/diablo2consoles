//go:build darwin

package main

import (
	"os"
	"path/filepath"
	"testing"
)

func writeLog(t *testing.T, dir, name, text string) {
	t.Helper()

	if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
}

func readLog(dir, name string) (string, bool) {
	b, err := os.ReadFile(filepath.Join(dir, name))
	return string(b), err == nil
}

func TestRotateLogsKeepsLastSession(t *testing.T) {
	dir := t.TempDir()
	writeLog(t, dir, currentLogName, "session one\n")
	rotateLogs(dir)

	if got, _ := readLog(dir, previousLogName); got != "session one\n" {
		t.Fatalf("previous = %q", got)
	}

	if _, ok := readLog(dir, currentLogName); ok {
		t.Fatal("current log should have moved")
	}

	if _, ok := readLog(dir, crashLogName); ok {
		t.Fatal("a clean session must not leave a crash log")
	}
}

func TestRotateLogsKeepsCrashAcrossCleanRelaunch(t *testing.T) {
	dir := t.TempDir()
	writeLog(t, dir, currentLogName, "starting\npanic: boom\n")
	rotateLogs(dir)
	writeLog(t, dir, currentLogName, "clean session\n") // the relaunch
	rotateLogs(dir)

	if got, _ := readLog(dir, previousLogName); got != "clean session\n" {
		t.Fatalf("previous = %q", got)
	}

	if got, _ := readLog(dir, crashLogName); got != "starting\npanic: boom\n" {
		t.Fatalf("crash log lost: %q", got)
	}
}

func TestRotateLogsIgnoresMissingAndEmpty(t *testing.T) {
	dir := t.TempDir()
	rotateLogs(dir) // nothing to do, no panic

	writeLog(t, dir, previousLogName, "real last session")
	writeLog(t, dir, currentLogName, "")
	rotateLogs(dir)

	if got, _ := readLog(dir, previousLogName); got != "real last session" {
		t.Fatalf("an empty log wiped the last session: %q", got)
	}
}

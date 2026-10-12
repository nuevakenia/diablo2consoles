//go:build windows
// +build windows

package d2hero

// lockD2S is a no-op on Windows: the last-writer check of CommitD2S still
// protects a save there.
func lockD2S(string) (func(), error) { return func() {}, nil }

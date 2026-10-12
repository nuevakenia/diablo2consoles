//go:build !windows

package d2setup

func nativeUI() UI { return OsaUI{} }

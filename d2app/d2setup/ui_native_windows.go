//go:build windows

package d2setup

import (
	"os"
	"os/exec"
	"strings"
	"syscall"
)

// WinUI implements UI with Windows PowerShell and System.Windows.Forms, which ship with Windows.
// UNVERIFIED: written on a Mac, never run on Windows.
type WinUI struct{}

func nativeUI() UI { return WinUI{} }

// psQuote escapes s as a PowerShell single-quoted string literal.
func psQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}

const psPrelude = "Add-Type -AssemblyName System.Windows.Forms;"

func powershell(script string) (string, error) {
	cmd := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-STA", "-Command", psPrelude+script)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	out, err := cmd.Output()

	return strings.TrimSpace(string(out)), err
}

// ChooseFolder implements UI.
func (WinUI) ChooseFolder(prompt, startDir string) (string, error) {
	script := "$d = New-Object System.Windows.Forms.FolderBrowserDialog;" +
		"$d.Description = " + psQuote(prompt) + ";"

	if st, err := os.Stat(startDir); err == nil && st.IsDir() {
		script += "$d.SelectedPath = " + psQuote(startDir) + ";"
	}

	script += "if ($d.ShowDialog() -eq 'OK') { Write-Output $d.SelectedPath }"

	out, err := powershell(script)
	if err != nil || out == "" {
		return "", ErrCancelled
	}

	return out, nil
}

// Ask implements UI. MessageBox only has stock buttons, so the Yes button means yes and No means no;
// the wanted button names are spelled out in the text.
func (WinUI) Ask(title, message, no, yes string) bool {
	out, err := powershell("[System.Windows.Forms.MessageBox]::Show(" + psQuote(message+"\n\nYes: "+yes+"\nNo: "+no) +
		", " + psQuote(title) + ", 'YesNo', 'Question')")

	return err == nil && out == "Yes"
}

// Alert implements UI.
func (WinUI) Alert(title, message string) {
	_, _ = powershell("[void][System.Windows.Forms.MessageBox]::Show(" + psQuote(message) + ", " + psQuote(title) + ", 'OK', 'Error')")
}

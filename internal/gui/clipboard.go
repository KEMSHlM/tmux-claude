package gui

import "os/exec"

// readClipboard reads the system clipboard via pbpaste (macOS).
func readClipboard() (string, error) {
	out, err := exec.Command("pbpaste").Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}

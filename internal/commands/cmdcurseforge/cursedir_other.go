//go:build !windows

package cmdcurseforge

import "errors"

// Stub version, so that getCurseDir exists
func getCurseDir() (string, error) {
	return "", errors.New("not compiled for windows")
}

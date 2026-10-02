//go:build !darwin

package coreaudio

import "fmt"

func HostInNames(device string) ([]string, error) {
	return nil, fmt.Errorf("CoreAudio is only available on macOS")
}

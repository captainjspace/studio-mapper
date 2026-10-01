//go:build !darwin

package main

import "fmt"

func hostInNames(device string) ([]string, error) {
	return nil, fmt.Errorf("CoreAudio is only available on macOS")
}

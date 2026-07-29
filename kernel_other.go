//go:build !arm64

package uyaml

// KernelName reports which newline-scan kernel Index uses on this platform.
func KernelName() string { return "swar-generic" }

func newlineMasks(data []byte, masks []uint64) {
	newlineMasksGeneric(data, masks)
}

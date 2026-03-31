package network

import "strings"

func parseAETitle(raw [16]byte) string {
	// AE titles are space padded; preserve internal spaces and trim padding only.
	return strings.TrimRight(string(raw[:]), " \x00")
}

func formatAETitle(dst *[16]byte, aeTitle string) {
	for i := range dst {
		dst[i] = 0x20
	}
	copy(dst[:], aeTitle)
}

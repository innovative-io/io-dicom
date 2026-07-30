package deflate

import (
	"bytes"
	"compress/zlib"
	"fmt"
	"io"
)

func DeflateFrame(in []byte) ([]byte, error) {
	var buf bytes.Buffer
	zw := zlib.NewWriter(&buf)
	if _, err := zw.Write(in); err != nil {
		_ = zw.Close()
		return nil, err
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// MaxInflatedBytes bounds how much data InflateFrame will materialise when the
// caller cannot state the expected size.
//
// Without a bound, a small crafted payload inflates without limit: 261 KB of
// input expands to 268 MB (about 1029:1), so a sub-megabyte hostile instance
// could exhaust memory on any receiver that parses untrusted objects. The
// dataset-level call in media's parser passes -1, and runs before validation,
// so that path was reachable straight off the wire. 512 MiB mirrors the
// pixel-data ceiling media already enforces.
const MaxInflatedBytes = 512 << 20

// InflateFrame decompresses a zlib stream. When expectedSize is non-negative it
// is treated as the exact expected output length; pass -1 when the size is not
// known ahead of time, in which case output is capped at MaxInflatedBytes.
func InflateFrame(in []byte, expectedSize int) ([]byte, error) {
	zr, err := zlib.NewReader(bytes.NewReader(in))
	if err != nil {
		return nil, err
	}
	defer zr.Close()

	if expectedSize >= 0 {
		// Exact size known: read precisely that much, then probe for trailing
		// bytes so an oversized stream is still rejected rather than silently
		// truncated. This also avoids io.ReadAll's growing buffer entirely.
		out := make([]byte, expectedSize)
		if _, err := io.ReadFull(zr, out); err != nil {
			return nil, fmt.Errorf("ERROR, invalid deflated frame size")
		}
		var probe [1]byte
		if n, _ := io.ReadFull(zr, probe[:]); n > 0 {
			return nil, fmt.Errorf("ERROR, invalid deflated frame size")
		}
		return out, nil
	}

	// Size unknown: read at most one byte past the cap so an over-long stream is
	// detected instead of being silently truncated at the limit.
	out, err := io.ReadAll(io.LimitReader(zr, MaxInflatedBytes+1))
	if err != nil {
		return nil, err
	}
	if len(out) > MaxInflatedBytes {
		return nil, fmt.Errorf("ERROR, deflated stream exceeds %d bytes", MaxInflatedBytes)
	}
	return out, nil
}

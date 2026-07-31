package network

import (
	"runtime"
	"testing"

	"github.com/innovative-io/io-dicom/media"
)

// buildOverlongUIDItem encodes a UID sub-item whose declared length far exceeds
// the bytes that follow it — the shape a hostile A-ASSOCIATE-RQ uses to make the
// parser allocate far more than it received.
// The item type is consumed by Read, so ReadDynamic starts at reserved1.
func buildOverlongUIDItem(declared uint16, actual int) []byte {
	out := []byte{
		0x00,                                // reserved1
		byte(declared >> 8), byte(declared), // declared length, big-endian
	}
	return append(out, make([]byte, actual)...)
}

// TestUIDItemRejectsOverlongLength pins the bounds check. The parser previously
// did make([]byte, u.length) before any bounds check and discarded ReadData's
// error, so a 4-byte item declaring 65535 bytes still allocated 64 KiB and
// retained it as the item's UID.
func TestUIDItemRejectsOverlongLength(t *testing.T) {
	// 4 header bytes, no payload, declaring the maximum a uint16 can hold.
	data := buildOverlongUIDItem(0xFFFF, 0)
	buf := media.NewDICOMBufferFromBytes(data)

	var u uidItem
	if err := u.ReadDynamic(buf); err == nil {
		t.Fatalf("expected an error for a declared length beyond the input, got uid of %d bytes", len(u.uid))
	}
	if u.uid != "" {
		t.Fatalf("no UID should be retained on failure, got %d bytes", len(u.uid))
	}
}

// TestUIDItemAllocationIsBoundedByInput measures the amplification directly:
// parsing many overlong items must not retain memory disproportionate to the
// bytes received.
func TestUIDItemAllocationIsBoundedByInput(t *testing.T) {
	const items = 4000
	// Each item is 4 bytes on the wire but declares 65535 bytes of UID.
	var wire []byte
	for i := 0; i < items; i++ {
		wire = append(wire, buildOverlongUIDItem(0xFFFF, 0)...)
	}

	var before, after runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&before)

	buf := media.NewDICOMBufferFromBytes(wire)
	parsed := 0
	for i := 0; i < items; i++ {
		var u uidItem
		if err := u.ReadDynamic(buf); err != nil {
			break // expected: the first item already runs past the input
		}
		parsed++
	}

	runtime.ReadMemStats(&after)
	allocated := after.TotalAlloc - before.TotalAlloc
	t.Logf("%d bytes of input, %d items parsed, %d bytes allocated (%.1fx)",
		len(wire), parsed, allocated, float64(allocated)/float64(len(wire)))

	// Before the fix each item allocated 64 KiB regardless of input, so 4000
	// items amplified ~16000x. A small multiple of the input is the contract.
	if allocated > uint64(len(wire))*8 {
		t.Fatalf("allocation %d is disproportionate to %d bytes of input", allocated, len(wire))
	}
}

// TestUIDItemValidRoundTrip guards against the bounds work rejecting good input.
func TestUIDItemValidRoundTrip(t *testing.T) {
	const uid = "1.2.840.10008.1.1"
	data := append(buildOverlongUIDItem(uint16(len(uid)), 0), []byte(uid)...)

	var u uidItem
	if err := u.ReadDynamic(media.NewDICOMBufferFromBytes(data)); err != nil {
		t.Fatalf("valid UID item should parse: %v", err)
	}
	if u.uid != uid {
		t.Fatalf("got %q, want %q", u.uid, uid)
	}
}

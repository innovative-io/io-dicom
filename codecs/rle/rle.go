package rle

import (
	"encoding/binary"
	"fmt"
	"math"
	"strings"
)

const rleMaxSegments = 15

// encodePackBits encodes data as PackBits (PS3.5 Annex G): a replicate run for
// three or more identical bytes, literal runs otherwise.
//
// Only literal runs were emitted before, so "RLE Lossless" never compressed
// anything -- output was always the input plus one byte per 128, meaning even a
// uniform frame expanded. The decoder already handled replicate runs, so this
// is encoder-only and needs no format change.
func encodePackBits(data []byte) []byte {
	if len(data) == 0 {
		return nil
	}

	out := make([]byte, 0, len(data)/2+16)
	i := 0
	for i < len(data) {
		// Measure the run of identical bytes starting at i.
		runEnd := i + 1
		for runEnd < len(data) && data[runEnd] == data[i] && runEnd-i < 128 {
			runEnd++
		}
		if runEnd-i >= 3 {
			// Replicate run: count byte is 257-n for n in [2,128].
			n := runEnd - i
			out = append(out, byte(257-n), data[i])
			i = runEnd
			continue
		}

		// Literal run: accumulate until a run of three or more appears, or the
		// 128-byte maximum is reached.
		litStart := i
		for i < len(data) && i-litStart < 128 {
			// Look ahead for a replicate run worth breaking the literal for.
			if i+2 < len(data) && data[i] == data[i+1] && data[i] == data[i+2] {
				break
			}
			i++
		}
		n := i - litStart
		out = append(out, byte(n-1))
		out = append(out, data[litStart:i]...)
	}
	return out
}

func RLEencode(in []byte, rows uint16, cols uint16, bitsAllocated uint16, samplesPerPixel uint16) ([]byte, error) {
	pixelCount := int(rows) * int(cols)
	if pixelCount <= 0 {
		return nil, fmt.Errorf("ERROR, format not supported")
	}

	if bitsAllocated != 8 && bitsAllocated != 16 {
		return nil, fmt.Errorf("ERROR, format not supported")
	}

	if samplesPerPixel != 1 && samplesPerPixel != 3 {
		return nil, fmt.Errorf("ERROR, format not supported")
	}

	bytesPerSample := int(bitsAllocated / 8)
	expectedSize := pixelCount * int(samplesPerPixel) * bytesPerSample
	if len(in) < expectedSize {
		return nil, fmt.Errorf("ERROR, overflow decoding RLE")
	}

	totalSegments := int(samplesPerPixel) * bytesPerSample
	if totalSegments > rleMaxSegments {
		return nil, fmt.Errorf("ERROR, format not supported")
	}

	rawSegments := make([][]byte, 0, totalSegments)
	for sample := 0; sample < int(samplesPerPixel); sample++ {
		for bytePlane := bytesPerSample - 1; bytePlane >= 0; bytePlane-- {
			segment := make([]byte, pixelCount)
			for pixel := 0; pixel < pixelCount; pixel++ {
				base := (pixel*int(samplesPerPixel) + sample) * bytesPerSample
				segment[pixel] = in[base+bytePlane]
			}
			rawSegments = append(rawSegments, segment)
		}
	}

	encodedSegments := make([][]byte, 0, totalSegments)
	for _, segment := range rawSegments {
		encodedSegments = append(encodedSegments, encodePackBits(segment))
	}

	headerSize := 64
	offsets := make([]uint32, rleMaxSegments)
	cursor := uint32(headerSize)
	for i, segment := range encodedSegments {
		offsets[i] = cursor
		cursor += uint32(len(segment))
	}

	out := make([]byte, 0, int(cursor))
	header := make([]byte, headerSize)
	binary.LittleEndian.PutUint32(header[0:4], uint32(totalSegments))
	for i := 0; i < rleMaxSegments; i++ {
		binary.LittleEndian.PutUint32(header[4+i*4:8+i*4], offsets[i])
	}
	out = append(out, header...)
	for _, segment := range encodedSegments {
		out = append(out, segment...)
	}

	return out, nil
}

func getUint32LE(in []byte) (uint32, error) {
	if len(in) < 4 {
		return 0, fmt.Errorf("ERROR, invalid RLE header")
	}
	return binary.LittleEndian.Uint32(in[:4]), nil
}

func readSegment(in []byte, out []byte, segmentOffset uint32, segmentSize uint32, segmentIndex uint32, rawSize uint32) error {
	outOffset := segmentIndex * rawSize
	inOffset := segmentOffset

	for (outOffset - segmentIndex*rawSize) < rawSize {
		if int(inOffset) >= len(in) || inOffset-segmentOffset >= segmentSize {
			return fmt.Errorf("ERROR, overflow decoding RLE")
		}

		count := int8(in[inOffset])
		inOffset++

		switch {
		case count >= 0:
			run := uint32(count) + 1
			if int(inOffset+run) > len(in) || inOffset+run-segmentOffset > segmentSize {
				return fmt.Errorf("ERROR, overflow decoding RLE")
			}
			if outOffset+run > uint32(len(out)) {
				return fmt.Errorf("ERROR, overflow decoding RLE")
			}
			copy(out[outOffset:outOffset+run], in[inOffset:inOffset+run])
			inOffset += run
			outOffset += run
		case count >= -127:
			// inOffset indexes the run's value byte, so it must lie strictly
			// inside the segment: >= not >. With > a segment ending in a bare
			// replicate-count byte was accepted and took its fill value from the
			// first byte of the NEXT segment, decoding malformed input to
			// plausible-looking garbage instead of erroring.
			if int(inOffset) >= len(in) || inOffset-segmentOffset >= segmentSize {
				return fmt.Errorf("ERROR, overflow decoding RLE")
			}
			run := uint32(-count) + 1
			if outOffset+run > uint32(len(out)) {
				return fmt.Errorf("ERROR, overflow decoding RLE")
			}
			value := in[inOffset]
			inOffset++
			for j := uint32(0); j < run; j++ {
				out[outOffset+j] = value
			}
			outOffset += run
		default:
			// count == -128 is a no-op in PackBits.
		}
	}

	return nil
}

func clampByte(v float32) byte {
	if v <= 0 {
		return 0
	}
	if v >= 255 {
		return 255
	}
	return byte(math.Round(float64(v)))
}

func RLEdecode(in []byte, out []byte, length uint32, size uint32, photoInt string) error {
	if len(in) < 64 || len(out) < int(size) || length == 0 || length > uint32(len(in)) {
		return fmt.Errorf("ERROR, overflow decoding RLE")
	}

	segmentCount, err := getUint32LE(in)
	if err != nil {
		return err
	}
	if segmentCount == 0 || segmentCount > rleMaxSegments {
		return fmt.Errorf("ERROR, format not supported")
	}
	if size%segmentCount != 0 {
		return fmt.Errorf("ERROR, format not supported")
	}

	segmentOffset := make([]uint32, rleMaxSegments+1)
	for i := uint32(0); i < rleMaxSegments; i++ {
		off, err := getUint32LE(in[4+i*4:])
		if err != nil {
			return err
		}
		segmentOffset[i] = off
	}
	segmentOffset[segmentCount] = length

	temp := make([]byte, size)
	decodedPlaneSize := size / segmentCount

	for i := uint32(0); i < segmentCount; i++ {
		if segmentOffset[i] > segmentOffset[i+1] || segmentOffset[i+1] > length {
			return fmt.Errorf("ERROR, overflow decoding RLE")
		}
		segmentLength := segmentOffset[i+1] - segmentOffset[i]
		if err := readSegment(in, temp, segmentOffset[i], segmentLength, i, decodedPlaneSize); err != nil {
			return err
		}
	}

	offset := decodedPlaneSize

	// YBR_FULL keeps its dedicated path because it also colour-converts to RGB.
	if photoInt == "YBR_FULL" && segmentCount == 3 {
		for i := uint32(0); i < decodedPlaneSize; i++ {
			y := float32(temp[i])
			cb := float32(temp[i+offset])
			cr := float32(temp[i+2*offset])
			out[3*i] = clampByte(y + 1.402*(cr-128.0))
			out[3*i+1] = clampByte(y - 0.344136*(cb-128.0) - 0.714136*(cr-128.0))
			out[3*i+2] = clampByte(y + 1.772*(cb-128.0))
		}
		return nil
	}

	// Everything else is a planar de-interleave. DICOM RLE (PS3.5 Annex G)
	// stores samplesPerPixel * bytesPerSample segments, ordered per sample from
	// the most significant byte down.
	//
	// This replaces a fixed set of (photometric, segmentCount) cases that
	// covered only 1, 2 and 3 segments. 16-bit colour legitimately produces 6,
	// so the encoder in this package emitted streams its own decoder rejected
	// with "format not supported", and any conforming 6-segment stream from
	// another vendor was refused on ingest. PALETTE COLOR was unreachable for
	// the same reason.
	samples := uint32(3)
	if strings.Contains(photoInt, "MONO") || strings.Contains(photoInt, "PALETTE") {
		samples = 1
	}
	if samples > segmentCount || segmentCount%samples != 0 {
		return fmt.Errorf("ERROR, %d RLE segments is not a multiple of %d samples for %q",
			segmentCount, samples, photoInt)
	}
	bytesPerSample := segmentCount / samples
	if uint64(decodedPlaneSize)*uint64(segmentCount) > uint64(len(out)) {
		return fmt.Errorf("ERROR, RLE output buffer too small")
	}
	for p := uint32(0); p < decodedPlaneSize; p++ {
		for s := uint32(0); s < samples; s++ {
			for b := uint32(0); b < bytesPerSample; b++ {
				// Segments run most-significant byte first within each sample,
				// while the output is little-endian.
				seg := s*bytesPerSample + (bytesPerSample - 1 - b)
				out[(p*samples+s)*bytesPerSample+b] = temp[seg*decodedPlaneSize+p]
			}
		}
	}
	return nil
}

package rle

import (
	"encoding/binary"
	"fmt"
	"math"
	"strings"
)

const rleMaxSegments = 15

func encodeLiteralRuns(data []byte) []byte {
	if len(data) == 0 {
		return nil
	}

	out := make([]byte, 0, len(data)+(len(data)/128)+1)
	for i := 0; i < len(data); {
		run := len(data) - i
		if run > 128 {
			run = 128
		}
		out = append(out, byte(run-1))
		out = append(out, data[i:i+run]...)
		i += run
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
		encodedSegments = append(encodedSegments, encodeLiteralRuns(segment))
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
			if int(inOffset) >= len(in) || inOffset-segmentOffset > segmentSize {
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
	switch {
	case strings.Contains(photoInt, "MONO") && segmentCount == 2:
		for i := uint32(0); i < decodedPlaneSize; i++ {
			out[2*i] = temp[i+offset]
			out[2*i+1] = temp[i]
		}
	case strings.Contains(photoInt, "MONO") && segmentCount == 1:
		copy(out[:size], temp)
	case photoInt == "YBR_FULL" && segmentCount == 3:
		for i := uint32(0); i < decodedPlaneSize; i++ {
			y := float32(temp[i])
			cb := float32(temp[i+offset])
			cr := float32(temp[i+2*offset])
			out[3*i] = clampByte(y + 1.402*(cr-128.0))
			out[3*i+1] = clampByte(y - 0.344136*(cb-128.0) - 0.714136*(cr-128.0))
			out[3*i+2] = clampByte(y + 1.772*(cb-128.0))
		}
	case photoInt == "RGB" && segmentCount == 3:
		for i := uint32(0); i < decodedPlaneSize; i++ {
			out[3*i] = temp[i]
			out[3*i+1] = temp[i+offset]
			out[3*i+2] = temp[i+2*offset]
		}
	default:
		return fmt.Errorf("ERROR, format not supported")
	}

	return nil
}

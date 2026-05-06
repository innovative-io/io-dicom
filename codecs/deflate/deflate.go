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

func InflateFrame(in []byte, expectedSize int) ([]byte, error) {
	zr, err := zlib.NewReader(bytes.NewReader(in))
	if err != nil {
		return nil, err
	}
	defer zr.Close()

	out, err := io.ReadAll(zr)
	if err != nil {
		return nil, err
	}
	if expectedSize >= 0 && len(out) != expectedSize {
		return nil, fmt.Errorf("ERROR, invalid deflated frame size")
	}
	return out, nil
}

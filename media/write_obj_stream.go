package media

import "io"

// WriteObjTo streams obj's tags to w, producing byte-for-byte what
// DICOMBuffer.WriteObj writes into a buffer — but without materialising the
// whole serialized object first. It emits no File Meta header (unlike WriteTo),
// so the output is a bare DIMSE stream suitable for the network P-DATA path.
//
// A small scratch buffer carries each tag's 8/12-byte header; the value bytes
// are handed to w directly so a multi-megabyte pixel-data tag is never copied.
// The scratch buffer is little-endian, matching the DICOMBuffer that WriteObj
// runs against on the send path (network P-DATA buffers are always constructed
// with the little-endian default and their tag values are stored pre-encoded).
func WriteObjTo(w io.Writer, obj DICOMObject) (int64, error) {
	scratch := &DICOMBuffer{data: make([]byte, 0, 32)}
	explicitVR := obj.IsExplicitVR()
	ts := obj.GetTransferSyntax()

	var written int64
	for i := 0; i < obj.TagCount(); i++ {
		tag := obj.GetTagAt(i)

		scratch.Clear()
		writeLen := scratch.writeTagHeader(tag, explicitVR, ts)
		if n, err := w.Write(scratch.GetData()); err != nil {
			return written + int64(n), err
		} else {
			written += int64(n)
		}

		if writeLen != 0 && writeLen != 0xFFFFFFFF {
			if n, err := w.Write(tag.Data[:writeLen]); err != nil {
				return written + int64(n), err
			} else {
				written += int64(n)
			}
		}
	}
	return written, nil
}

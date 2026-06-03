// Package output provides library-level output redirection for io-dicom.
//
// Call Redirect once at startup to route all log and slog output from the
// library to a single writer — for example an application console or test
// buffer — without resorting to process-wide stdout/stderr capture.
//
// media.DICOMObject.DumpTags accepts an io.Writer directly; pass the same
// writer there when you also want tag-dump output in the same destination.
package output

import (
	"io"
	"log"
	"log/slog"
)

// Redirect routes the default log and slog outputs to w. It is safe to call
// multiple times; each call replaces the previously configured destination.
func Redirect(w io.Writer) {
	log.SetOutput(w)
	slog.SetDefault(slog.New(slog.NewTextHandler(w, nil)))
}

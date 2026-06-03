package media

import (
	"github.com/innovative-io/io-dicom/dictionary"
)

// InitDict initializes the DICOM dictionary. It is safe to call from multiple
// goroutines.
//
// Deprecated: the dictionary is now initialized automatically when the
// dictionary package is imported. This function is a no-op kept for backward
// compatibility and can be removed from existing code.
func InitDict() { dictionary.InitDict() }

// FillTag populates tag fields from the dictionary. Name, Description, VR, and
// VM are only written when the tag's existing value is empty.
func FillTag(tag *DICOMTag) {
	dt := dictionary.GetDictionaryTag(tag.Group, tag.Element)
	if tag.Name == "" {
		tag.Name = dt.Name
	}
	if tag.Description == "" {
		tag.Description = dt.Description
	}
	if tag.VR == "" {
		tag.VR = dt.VR
	}
	if tag.VM == "" {
		tag.VM = dt.VM
	}
}

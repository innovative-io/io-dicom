package media

// JSONTag - JSON struct for a DICOM tag
type JSONTag struct {
	VR    string        `json:"vr"`
	Value []interface{} `json:"Value"`
}

// JSONPNValue - JSON struct for PN value
type JSONPNValue struct {
	Alphabetic  JSONAlphabeticValue `json:"Alphabetic"`
	Ideographic JSONAlphabeticValue `json:"Ideographic,omitempty"`
}

// JSONAlphabeticValue - JSON struct for alphabetic value
type JSONAlphabeticValue struct {
	Family []string `json:"Family,omitempty"`
	Given  []string `json:"Given,omitempty"`
	Suffix []string `json:"Suffix,omitempty"`
}

// DICOMJSONObject - JSON struct for DICOM data
type DICOMJSONObject interface {
}

type dicomJSONObject struct {
	Tags map[string]JSONTag
}

// NewJSONObj - Creates a new dicomJSONObject and returns an interface to it
func NewJSONObj() DICOMJSONObject {
	return &dicomJSONObject{
		Tags: make(map[string]JSONTag),
	}
}

// NewJSONObjFromDcmObj - Creates a new dicomJSONObject and parses DICOMObject into it
func NewJSONObjFromDcmObj(dcm DICOMObject) DICOMJSONObject {
	return &dicomJSONObject{}
}

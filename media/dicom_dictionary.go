package media

import (
	"encoding/xml"
	"os"
	"strconv"
	"sync"

	"github.com/innovative-io/io-dicom/dictionary/tags"
)

type privateDictionaryXML struct {
	XMLName xml.Name               `xml:"dictionary"`
	Tags    []privateDictionaryTag `xml:"tag"`
}

type privateDictionaryTag struct {
	Group       string `xml:"group,attr"`
	Element     string `xml:"element,attr"`
	Name        string `xml:"keyword,attr"`
	VR          string `xml:"vr,attr"`
	VM          string `xml:"vm,attr"`
	Description string `xml:",chardata"`
}

var codes []*tags.Tag
var codeByKey map[uint32]*tags.Tag
var codeVRByKey map[uint32]string
var initOnce sync.Once

// unknownTag is a package-level sentinel returned by GetDictionaryTag for
// unrecognised group/element pairs. Using a single shared instance avoids
// a heap allocation on every cache miss in the hot tag-parsing loop.
var unknownTag = &tags.Tag{
	Group:       0,
	Element:     0,
	VR:          "UN",
	VM:          "",
	Name:        "Unknown",
	Description: "Unknown",
}

func dictionaryKey(group uint16, element uint16) uint32 {
	return uint32(group)<<16 | uint32(element)
}

func buildDictionaryIndex() {
	codeByKey = make(map[uint32]*tags.Tag, len(codes))
	codeVRByKey = make(map[uint32]string, len(codes))
	for i := 0; i < len(codes); i++ {
		key := dictionaryKey(codes[i].Group, codes[i].Element)
		if _, exists := codeByKey[key]; !exists {
			codeByKey[key] = codes[i]
			codeVRByKey[key] = codes[i].VR
		}
	}
}

// FillTag - Populates with data from dictionary
func FillTag(tag *DICOMTag) {
	dt := GetDictionaryTag(tag.Group, tag.Element)
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

// GetDictionaryTag - get tag from Dictionary
func GetDictionaryTag(group uint16, element uint16) *tags.Tag {
	InitDict()
	if tag, ok := codeByKey[dictionaryKey(group, element)]; ok {
		return tag
	}
	return unknownTag
}

// GetDictionaryVR - get info from Dictionary
func GetDictionaryVR(group uint16, element uint16) string {
	InitDict()
	if vr, ok := codeVRByKey[dictionaryKey(group, element)]; ok {
		return vr
	}
	return "UN"
}

func loadPrivateDictionary() {
	privateDictionaryFile := "./private.xml"
	data, err := os.ReadFile(privateDictionaryFile)
	if err != nil {
		return
	}

	dict := new(privateDictionaryXML)
	err = xml.Unmarshal(data, dict)
	if err != nil {
		return
	}

	for _, t := range dict.Tags {
		g, err := strconv.Atoi(t.Group)
		if err != nil {
			continue
		}
		e, err := strconv.Atoi(t.Element)
		if err != nil {
			continue
		}

		codes = append(codes, &tags.Tag{
			Group:       uint16(g),
			Element:     uint16(e),
			Name:        t.Name,
			Description: t.Description,
			VR:          t.VR,
			VM:          t.VM,
		})
	}
}

// InitDict Initialize Dictionary
func InitDict() {
	initOnce.Do(func() {
		codes = tags.GetTags()
		loadPrivateDictionary()
		buildDictionaryIndex()
	})
}

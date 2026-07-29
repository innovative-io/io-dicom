package dictionary

import (
	"encoding/xml"
	"os"
	"strconv"
	"sync"

	_ "embed"

	"github.com/innovative-io/io-dicom/dictionary/tags"
)

//go:embed private.xml
var defaultPrivateXML []byte

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
var initOnce sync.Once
var mu sync.Mutex

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
	for i := 0; i < len(codes); i++ {
		key := dictionaryKey(codes[i].Group, codes[i].Element)
		if _, exists := codeByKey[key]; !exists {
			codeByKey[key] = codes[i]
		}
	}
}

func parseDictionaryXML(data []byte) {
	dict := new(privateDictionaryXML)
	if err := xml.Unmarshal(data, dict); err != nil {
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

// GetDictionaryTag returns the Tag for the given group/element pair, or a
// sentinel "Unknown" tag if the pair is not in the dictionary.
func GetDictionaryTag(group uint16, element uint16) *tags.Tag {
	InitDict()
	if tag, ok := codeByKey[dictionaryKey(group, element)]; ok {
		return tag
	}
	return unknownTag
}

// GetDictionaryVR returns the VR string for the given group/element pair, or
// "UN" if it is not in the dictionary.
func GetDictionaryVR(group uint16, element uint16) string {
	InitDict()
	if tag, ok := codeByKey[dictionaryKey(group, element)]; ok {
		return tag.VR
	}
	return "UN"
}

// LoadPrivateDictionary parses the private-tag XML file at path and merges its
// entries into the dictionary. InitDict must be called first. Duplicate
// group/element pairs are silently skipped.
func LoadPrivateDictionary(path string) {
	InitDict()
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	parseDictionaryXML(data)
	mu.Lock()
	buildDictionaryIndex()
	mu.Unlock()
}

// InitDict initializes the DICOM dictionary from the standard tag set and the
// bundled private.xml. It is safe to call from multiple goroutines; initialization
// happens at most once.
//
// Deprecated: the dictionary is now initialized automatically when this package
// is imported. Calling InitDict is a no-op and can be removed from existing code.
func InitDict() {
	initOnce.Do(func() {
		codes = tags.GetTags()
		parseDictionaryXML(defaultPrivateXML)
		buildDictionaryIndex()
	})
}

func init() { InitDict() }

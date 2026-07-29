package tags

import "sync"

// Tag Dictionary Structure definition
type Tag struct {
	Group       uint16
	Element     uint16
	VR          string
	VM          string
	Name        string
	Description string
}

// emptyTag is the shared sentinel returned when a lookup finds nothing. It must
// never be mutated by callers. Returning a shared value keeps misses
// allocation-free, matching the unknownTag sentinel in the dictionary package.
var emptyTag = &Tag{}

var (
	byNameOnce sync.Once
	byName     map[string]*Tag

	byKeyOnce sync.Once
	byKey     map[uint32]*Tag
)

func tagKey(group, element uint16) uint32 {
	return uint32(group)<<16 | uint32(element)
}

// GetTagFromName returns the tag with the given name, or an empty sentinel tag.
// The name index is built on first use, so callers that never look up by name
// do not pay for it.
func GetTagFromName(name string) *Tag {
	byNameOnce.Do(func() {
		byName = make(map[string]*Tag, len(tags))
		for _, tag := range tags {
			if _, exists := byName[tag.Name]; !exists {
				byName[tag.Name] = tag
			}
		}
	})
	if tag, ok := byName[name]; ok {
		return tag
	}
	return emptyTag
}

// GetTag returns the tag for the given group and element, or an empty sentinel
// tag. The group/element index is built on first use.
func GetTag(group uint16, element uint16) *Tag {
	byKeyOnce.Do(func() {
		byKey = make(map[uint32]*Tag, len(tags))
		for _, tag := range tags {
			k := tagKey(tag.Group, tag.Element)
			if _, exists := byKey[k]; !exists {
				byKey[k] = tag
			}
		}
	})
	if tag, ok := byKey[tagKey(group, element)]; ok {
		return tag
	}
	return emptyTag
}

// GetTags - Get all tags
func GetTags() []*Tag {
	return tags
}

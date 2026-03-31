package tags

// Tag Dictionary Structure definition
type Tag struct {
	Group       uint16
	Element     uint16
	VR          string
	VM          string
	Name        string
	Description string
}

func GetTagFromName(name string) *Tag {
	for _, tag := range tags {
		if tag.Name == name {
			return tag
		}
	}
	return &Tag{}
}

// GetTag - Get tag from group and element
func GetTag(group uint16, element uint16) *Tag {
	for _, tag := range tags {
		if tag.Group == group && tag.Element == element {
			return tag
		}
	}
	return &Tag{}
}

// GetTags - Get all tags
func GetTags() []*Tag {
	return tags
}

func GetGroupElement(Name string) (group uint16, element uint16) {
	for _, tag := range tags {
		if tag.Name == Name {
			return tag.Group, tag.Element
		}
	}
	return 0, 0
}

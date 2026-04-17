package sopclass

import "strings"

type SOPClass struct {
	UID         string
	Name        string
	Description string
	Type        string
}

func GetSOPClassFromName(name string) *SOPClass {
	for _, sop := range sopClasses {
		if sop.Name == name {
			return sop
		}
	}
	return nil
}

func GetSOPClassFromUID(uid string) *SOPClass {
	for _, sop := range sopClasses {
		if sop.UID == uid {
			return sop
		}
	}
	return nil
}

func GetStorageSOPClasses() []*SOPClass {
	storageClasses := make([]*SOPClass, 0)
	for _, sop := range sopClasses {
		if strings.HasPrefix(sop.UID, "1.2.840.10008.5.1.4.1.1.") {
			storageClasses = append(storageClasses, sop)
		}
	}
	return storageClasses
}

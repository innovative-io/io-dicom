package sopclass

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

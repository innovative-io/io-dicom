package network

var (
	uniqid = 1
)

// Resetuniq - Resetuniq
func Resetuniq() {
	uniqid = 1
}

// Uniq8 - Uniq8
func Uniq8() byte {
	uniqid++
	return byte(uniqid & 0xff)
}

// Uniq16 - Uniq16
func Uniq16() uint16 {
	uniqid++
	return uint16(uniqid & 0xffff)
}

// Uniq8odd - Uniq8odd
func Uniq8odd() byte {
	if uniqid&0x01 == 1 {
		uniqid++
		return byte(uniqid & 0xff)
	}
	uniqid++
	uniqid++
	return byte(uniqid & 0xff)
}

// Uniq16odd - Uniq16odd
func Uniq16odd() uint16 {
	if uniqid&0x01 == 1 {
		uniqid++
		return uint16(uniqid & 0xffff)
	}
	uniqid++
	uniqid++
	return uint16(uniqid & 0xffff)
}

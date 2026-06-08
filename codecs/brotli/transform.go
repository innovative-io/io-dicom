package brotli

// Dictionary word transform types (RFC 7932 Appendix B).
const (
	tIdentity = 0
	// 1..9: omit last n bytes.
	tOmitLast1  = 1
	tUpperFirst = 10
	tUpperAll   = 11
	// 12..20: omit first n bytes.
	tOmitFirst1 = 12
)

type wordTransform struct {
	prefix string
	kind   int
	suffix string
}

// kTransforms is the standard 121-entry transform table (RFC 7932 Appendix B).
var kTransforms = [121]wordTransform{
	{"", tIdentity, ""},
	{"", tIdentity, " "},
	{" ", tIdentity, " "},
	{"", tOmitFirst1, ""},
	{"", tUpperFirst, " "},
	{"", tIdentity, " the "},
	{" ", tIdentity, ""},
	{"s ", tIdentity, " "},
	{"", tIdentity, " of "},
	{"", tUpperFirst, ""},
	{"", tIdentity, " and "},
	{"", tOmitFirst1 + 1, ""}, // OMIT_FIRST_2
	{"", tOmitLast1, ""},
	{", ", tIdentity, " "},
	{"", tIdentity, ", "},
	{" ", tUpperFirst, " "},
	{"", tIdentity, " in "},
	{"", tIdentity, " to "},
	{"e ", tIdentity, " "},
	{"", tIdentity, "\""},
	{"", tIdentity, "."},
	{"", tIdentity, "\">"},
	{"", tIdentity, "\n"},
	{"", tOmitLast1 + 2, ""}, // OMIT_LAST_3
	{"", tIdentity, "]"},
	{"", tIdentity, " for "},
	{"", tOmitFirst1 + 2, ""}, // OMIT_FIRST_3
	{"", tOmitLast1 + 1, ""},  // OMIT_LAST_2
	{"", tIdentity, " a "},
	{"", tIdentity, " that "},
	{" ", tUpperFirst, ""},
	{"", tIdentity, ". "},
	{".", tIdentity, ""},
	{" ", tIdentity, ", "},
	{"", tOmitFirst1 + 3, ""}, // OMIT_FIRST_4
	{"", tIdentity, " with "},
	{"", tIdentity, "'"},
	{"", tIdentity, " from "},
	{"", tIdentity, " by "},
	{"", tOmitFirst1 + 4, ""}, // OMIT_FIRST_5
	{"", tOmitFirst1 + 5, ""}, // OMIT_FIRST_6
	{" the ", tIdentity, ""},
	{"", tOmitLast1 + 3, ""}, // OMIT_LAST_4
	{"", tIdentity, ". The "},
	{"", tUpperAll, ""},
	{"", tIdentity, " on "},
	{"", tIdentity, " as "},
	{"", tIdentity, " is "},
	{"", tOmitLast1 + 6, ""}, // OMIT_LAST_7
	{"", tOmitLast1, "ing "},
	{"", tIdentity, "\n\t"},
	{"", tIdentity, ":"},
	{" ", tIdentity, ". "},
	{"", tIdentity, "ed "},
	{"", tOmitFirst1 + 8, ""}, // OMIT_FIRST_9
	{"", tOmitFirst1 + 6, ""}, // OMIT_FIRST_7
	{"", tOmitLast1 + 5, ""},  // OMIT_LAST_6
	{"", tIdentity, "("},
	{"", tUpperFirst, ", "},
	{"", tOmitLast1 + 7, ""}, // OMIT_LAST_8
	{"", tIdentity, " at "},
	{"", tIdentity, "ly "},
	{" the ", tIdentity, " of "},
	{"", tOmitLast1 + 4, ""}, // OMIT_LAST_5
	{"", tOmitLast1 + 8, ""}, // OMIT_LAST_9
	{" ", tUpperFirst, ", "},
	{"", tUpperFirst, "\""},
	{".", tIdentity, "("},
	{"", tUpperAll, " "},
	{"", tUpperFirst, "\">"},
	{"", tIdentity, "=\""},
	{" ", tIdentity, "."},
	{".com/", tIdentity, ""},
	{" the ", tIdentity, " of the "},
	{"", tUpperFirst, "'"},
	{"", tIdentity, ". This "},
	{"", tIdentity, ","},
	{".", tIdentity, " "},
	{"", tUpperFirst, "("},
	{"", tUpperFirst, "."},
	{"", tIdentity, " not "},
	{" ", tIdentity, "=\""},
	{"", tIdentity, "er "},
	{" ", tUpperAll, " "},
	{"", tIdentity, "al "},
	{" ", tUpperAll, ""},
	{"", tIdentity, "='"},
	{"", tUpperAll, "\""},
	{"", tUpperFirst, ". "},
	{" ", tIdentity, "("},
	{"", tIdentity, "ful "},
	{" ", tUpperFirst, ". "},
	{"", tIdentity, "ive "},
	{"", tIdentity, "less "},
	{"", tUpperAll, "'"},
	{"", tIdentity, "est "},
	{" ", tUpperFirst, "."},
	{"", tUpperAll, "\">"},
	{" ", tIdentity, "='"},
	{"", tUpperFirst, ","},
	{"", tIdentity, "ize "},
	{"", tUpperAll, "."},
	{" ", tIdentity, ""},
	{" ", tIdentity, ","},
	{"", tUpperFirst, "=\""},
	{"", tUpperAll, "=\""},
	{"", tIdentity, "ous "},
	{"", tUpperAll, ", "},
	{"", tUpperFirst, "='"},
	{" ", tUpperFirst, ","},
	{" ", tUpperAll, "=\""},
	{" ", tUpperAll, ", "},
	{"", tUpperAll, ","},
	{"", tUpperAll, "("},
	{"", tUpperAll, ". "},
	{" ", tUpperAll, "."},
	{"", tUpperAll, "='"},
	{" ", tUpperAll, ". "},
	{" ", tUpperFirst, "=\""},
	{" ", tUpperAll, "='"},
	{" ", tUpperFirst, "='"},
}

// fermentChar upper-cases the (UTF-8) character at p[0:] in place and returns
// its byte length, matching Brotli's word transform.
func fermentChar(p []byte) int {
	switch {
	case p[0] < 0xC0:
		if p[0] >= 'a' && p[0] <= 'z' {
			p[0] ^= 32
		}
		return 1
	case p[0] < 0xE0:
		if len(p) >= 2 {
			p[1] ^= 32
		}
		return 2
	default:
		if len(p) >= 3 {
			p[2] ^= 32
		}
		return 3
	}
}

// applyTransform applies transform id to the dictionary word and appends the
// result to dst, returning the extended slice.
func applyTransform(dst []byte, id int, word []byte) ([]byte, error) {
	if id < 0 || id >= len(kTransforms) {
		return nil, errDictionary
	}
	t := kTransforms[id]
	dst = append(dst, t.prefix...)
	var core []byte
	switch {
	case t.kind == tIdentity:
		core = append(core, word...)
	case t.kind >= tOmitLast1 && t.kind <= tOmitLast1+8:
		n := t.kind - tOmitLast1 + 1
		if n < len(word) {
			core = append(core, word[:len(word)-n]...)
		}
	case t.kind >= tOmitFirst1 && t.kind <= tOmitFirst1+8:
		n := t.kind - tOmitFirst1 + 1
		if n < len(word) {
			core = append(core, word[n:]...)
		}
	case t.kind == tUpperFirst:
		core = append(core, word...)
		if len(core) > 0 {
			fermentChar(core)
		}
	case t.kind == tUpperAll:
		core = append(core, word...)
		for i := 0; i < len(core); {
			i += fermentChar(core[i:])
		}
	default:
		return nil, errDictionary
	}
	dst = append(dst, core...)
	dst = append(dst, t.suffix...)
	return dst, nil
}

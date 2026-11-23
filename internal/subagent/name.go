package subagent

import (
	"crypto/rand"
	"encoding/hex"
	"strings"
	"unicode"
)

// slugWords bounds how much of a title names a branch.
const slugWords = 4

// slug is title as a branch segment: its first words, lower case, letters
// and digits joined by dashes; task when nothing is left.
func slug(title string) string {
	words := strings.FieldsFunc(strings.ToLower(title), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r) || r > unicode.MaxASCII
	})
	if len(words) > slugWords {
		words = words[:slugWords]
	}
	if len(words) == 0 {
		return "task"
	}
	return strings.Join(words, "-")
}

// suffix is four hex digits, the tail that keeps two branches named alike
// apart.
func suffix() string {
	b := make([]byte, 2)
	if _, err := rand.Read(b); err != nil {
		return "0000"
	}
	return hex.EncodeToString(b)
}

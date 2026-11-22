package textutil

import (
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

func Fold(s string) string {
	var b strings.Builder
	for _, r := range norm.NFD.String(s) {
		if unicode.Is(unicode.Mn, r) {
			continue
		}
		b.WriteRune(unicode.ToLower(r))
	}
	return strings.Join(strings.Fields(b.String()), " ")
}

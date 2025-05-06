package util

import (
	"regexp"
	"strings"
)

func LetterNumberEscape(s string) string {
	m := regexp.MustCompile(`([A-Za-z0-9-_]+)`).FindAllString(s, -1)
	if m != nil {
		s = strings.Join(m, "")
	}
	return s
}

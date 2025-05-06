package gohttp

import (
	"os"
	"regexp"
	"strings"
)

func ReadCookieFile(cfile string) (cookies string) {
	if cfile == "" {
		return
	}
	bs, err := os.ReadFile(cfile)
	if err != nil {
		return ""
	}
	mCookie := strings.Split(string(bs), "\n")
	for _, line := range mCookie {
		if strings.HasPrefix(line, "#") {
			continue
		}
		text := regexp.MustCompile(`\\"`).ReplaceAllString(line, "\"")
		row := strings.Split(text, "\t")
		if len(row) < 8 {
			continue
		}
		k := strings.ReplaceAll(row[5], "\"", "")
		v := strings.ReplaceAll(row[6], "\"", "")
		s := k + "=" + v + "; "
		cookies += s
	}
	return cookies
}

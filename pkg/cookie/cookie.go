package cookie

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"
)

var cookieNameSanitizer = strings.NewReplacer("\n", "-", "\r", "-")

func sanitizeCookieName(n string) string {
	return cookieNameSanitizer.Replace(n)
}
func sanitizeCookieValue(v string, quoted bool) string {
	v = sanitizeOrWarn("Cookie.Value", validCookieValueByte, v)
	if len(v) == 0 {
		return v
	}
	if strings.ContainsAny(v, " ,") || quoted {
		return `"` + v + `"`
	}
	return v
}

func validCookieValueByte(b byte) bool {
	return 0x20 <= b && b < 0x7f && b != '"' && b != ';' && b != '\\'
}

func sanitizeOrWarn(fieldName string, valid func(byte) bool, v string) string {
	ok := true
	for i := 0; i < len(v); i++ {
		if valid(v[i]) {
			continue
		}
		log.Printf("net/http: invalid byte %q in %s; dropping invalid bytes", v[i], fieldName)
		ok = false
		break
	}
	if ok {
		return v
	}
	buf := make([]byte, 0, len(v))
	for i := 0; i < len(v); i++ {
		if b := v[i]; valid(b) {
			buf = append(buf, b)
		}
	}
	return string(buf)
}

func HttpCookieFromFile(cookieFile string) []http.Cookie {
	if cookieFile == "" {
		return nil
	}
	fp, err := os.Open(cookieFile)
	if err != nil {
		return nil
	}
	defer fp.Close()

	bsHeader, err := io.ReadAll(fp)
	if err != nil {
		return nil
	}
	mHeader := strings.Split(string(bsHeader), "\n")
	cookies := make([]http.Cookie, 0, len(mHeader)+1)
	for _, line := range mHeader {
		if strings.HasPrefix(line, "#") {
			continue
		}
		text := regexp.MustCompile(`\\"`).ReplaceAllString(line, "\"")
		row := strings.Split(text, "\t")
		if len(row) < 8 {
			continue
		}
		name := strings.ReplaceAll(row[5], "\"", "")
		value := strings.ReplaceAll(row[6], "\"", "")
		//expires := strings.ReplaceAll(row[4], "#HttpOnly_", "")
		cookies = append(cookies, http.Cookie{Name: name, Value: value})
	}
	return cookies
}

func HttpHeaderFromFile(cookieFile string) http.Header {
	if cookieFile == "" {
		return nil
	}
	fp, err := os.Open(cookieFile)
	if err != nil {
		return nil
	}
	defer fp.Close()

	bsHeader, err := io.ReadAll(fp)
	if err != nil {
		return nil
	}
	mHeader := strings.Split(string(bsHeader), "\n")
	cookies := make([]http.Cookie, 0, len(mHeader)+1)
	for _, line := range mHeader {
		if strings.HasPrefix(line, "#") {
			continue
		}
		text := regexp.MustCompile(`\\"`).ReplaceAllString(line, "\"")
		row := strings.Split(text, "\t")
		if len(row) < 8 {
			continue
		}
		name := strings.ReplaceAll(row[5], "\"", "")
		value := strings.ReplaceAll(row[6], "\"", "")
		//expires := strings.ReplaceAll(row[4], "#HttpOnly_", "")
		cookies = append(cookies, http.Cookie{Name: name, Value: value})
	}

	header := make(http.Header)
	for _, v := range cookies {
		s := fmt.Sprintf("%s=%s", sanitizeCookieName(v.Name), sanitizeCookieValue(v.Value, v.Quoted))
		if c := header.Get("Cookie"); c != "" {
			header.Set("Cookie", c+"; "+s)
		} else {
			header.Set("Cookie", s)
		}
	}

	return header
}

func CookiesFromFile(cfile string) (cookies string) {
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
		name := strings.ReplaceAll(row[5], "\"", "")
		value := strings.ReplaceAll(row[6], "\"", "")
		s := name + "=" + value + "; "
		cookies += s
	}
	return cookies
}

package util

import (
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

type UriMatch struct {
	Min  string
	Max  string
	IMin int
	IMax int
}

func SubText(text, from, to string) string {
	iPos := strings.Index(text, from)
	if iPos == -1 {
		return ""
	}
	subText := text[iPos:]
	iPos2 := strings.Index(subText, to)
	if iPos2 == -1 {
		return ""
	}
	return subText[:iPos2]
}

func GetUriMatch(uri string) (u UriMatch, ok bool) {
	m := regexp.MustCompile(`\((\d+)-(\d+)\)`).FindStringSubmatch(uri)
	if m == nil {
		return u, false
	}

	u.Min = m[1]
	u.Max = m[2]
	i, _ := strconv.Atoi(u.Min)
	u.IMin = i
	iMax, _ := strconv.Atoi(u.Max)
	u.IMax = iMax

	return u, true
}

func GetHostUrl(uri string) string {
	u, err := url.Parse(uri)
	if err != nil {
		return ""
	}
	var hostUrl = fmt.Sprintf("%s://%s/", u.Scheme, u.Host)
	return hostUrl
}

func RemoveDuplicate(source []string) []string {
	mTmp := map[string]string{}
	newArray := make([]string, len(source), 0)
	for e := range source {
		if value, ok := mTmp[source[e]]; !ok {
			newArray = append(newArray, value)
		}
	}
	return newArray
}

func ToInt(val interface{}) (int, error) {
	switch v := val.(type) {
	case int:
		return v, nil
	case float64:
		return int(v), nil
	case string:
		return strconv.Atoi(v)
	default:
		return 0, fmt.Errorf("unsupported type")
	}
}

func ToString(val interface{}) (string, error) {
	switch v := val.(type) {
	case string:
		return v, nil
	case int:
		return strconv.Itoa(v), nil
	case float64:
		return strconv.Itoa(int(v)), nil
	default:
		return "", fmt.Errorf("unsupported type")
	}
}

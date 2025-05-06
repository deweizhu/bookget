package crypt

import (
	"net/url"
	"strings"
)

// Javascript encodeURI 效率很低，应该从底层修改
func EncodeURI(path string) string {
	// var set1 = ";,/?:@&=+$"  // 保留字符
	// var set2 = "-_.!~*'()"   // 不转义字符
	// var set3 = "#"           // 数字标志
	// var set4 = "ABC abc 123" // 字母数字字符和空格
	s := url.PathEscape(path)
	s = strings.ReplaceAll(s, "%3B", ";")
	s = strings.ReplaceAll(s, "%2C", ",")
	s = strings.ReplaceAll(s, "%2F", "/")
	s = strings.ReplaceAll(s, "%3F", "?")
	s = strings.ReplaceAll(s, "%21", "!")
	s = strings.ReplaceAll(s, "%2A", "*")
	s = strings.ReplaceAll(s, "%27", "'")
	s = strings.ReplaceAll(s, "%28", "(")
	s = strings.ReplaceAll(s, "%29", ")")
	s = strings.ReplaceAll(s, "%23", "#")
	return s
}

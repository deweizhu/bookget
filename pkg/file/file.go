package file

import (
	"bookget/config"
	"strings"
)

func Ext(uri string) string {
	if config.Conf.FileExt != "" && config.Conf.FileExt[0] == '.' {
		return config.Conf.FileExt
	}
	return Extention(uri)
}
func Extention(uri string) string {
	ext := ""
	k := len(uri)
	for i := k - 1; i >= 0; i-- {
		if uri[i] == '?' {
			k = i
			continue
		}
		if uri[i] == '.' {
			ext = uri[i:k]
			break
		}
	}
	return ext
}

func Name(uri string) string {
	if strings.Contains(uri, "?") {
		pos := strings.Index(uri, "?")
		uri = uri[:pos]
	}
	if strings.Contains(uri, "&") {
		pos := strings.Index(uri, "&")
		uri = uri[:pos]
	}
	name := ""
	for i := len(uri) - 1; i >= 0; i-- {
		if uri[i] == '/' {
			name = uri[i+1:]
			break
		}
	}
	return name
}

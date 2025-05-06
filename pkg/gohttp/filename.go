package gohttp

import (
	"mime"
	"net/url"
	"path/filepath"
	"strings"
)

// DefaultFileName is the fallback name for GetFilename.
var DefaultFileName = "gohttp.output"

// GetFilename it returns default file name from a URL.
func getFilename(URL string) string {

	if u, err := url.Parse(URL); err == nil && filepath.Ext(u.Path) != "" {

		return filepath.Base(u.Path)
	}

	return DefaultFileName
}

func getNameFromHeader(val string) string {

	_, params, err := mime.ParseMediaType(val)

	// Prevent path traversal
	if err != nil || strings.Contains(params["filename"], "..") || strings.Contains(params["filename"], "/") || strings.Contains(params["filename"], "\\") {
		return ""
	}

	return params["filename"]
}

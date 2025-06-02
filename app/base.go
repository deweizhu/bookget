package app

import (
	"bookget/config"
	"bookget/pkg/chttp"
)

func BuildRequestHeader() map[string]string {
	httpHeaders := map[string]string{"User-Agent": config.Conf.UserAgent}
	cookies, _ := chttp.ReadCookiesFromFile(config.Conf.CookieFile)
	if cookies != "" {
		httpHeaders["Cookie"] = cookies
	}

	headers, err := chttp.ReadHeadersFromFile(config.Conf.HeaderFile)
	if err == nil {
		for key, value := range headers {
			httpHeaders[key] = value
		}
	}
	return httpHeaders
}

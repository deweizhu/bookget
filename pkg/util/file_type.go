package util

import (
	"bookget/config"
	"crypto/tls"
	"log"
	"net/http"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

var (
	contentTypeCache = sync.Map{}
	jsonExtensions   = map[string]bool{
		".json": true,
	}
	contentTypeMappings = map[string]string{
		"application/ld+json": "json",
		"application/json":    "json",
		"text/html":           "html",
		"image/jpeg":          "bookget",
		"image/png":           "bookget",
		"application/pdf":     "bookget",
	}
	rangeRegex = regexp.MustCompile(`$(\d+)-(\d+)$`)
)

func GetHeaderContentType(sUrl string) string {
	// 检查缓存
	if cached, ok := contentTypeCache.Load(sUrl); ok {
		return cached.(string)
	}

	// 1. 首先检查文件扩展名
	if hasJSONExtension(sUrl) {
		return cacheAndReturn(sUrl, "json")
	}

	// 2. 检查是否有范围格式
	if isRangeFormat(sUrl) {
		return cacheAndReturn(sUrl, "octet-stream")
	}

	// 3. 最后通过HTTP请求获取Content-Type
	return determineContentTypeByRequest(sUrl)
}

func hasJSONExtension(url string) bool {
	ext := filepath.Ext(url)
	return jsonExtensions[ext]
}

func isRangeFormat(url string) bool {
	return rangeRegex.MatchString(url)
}

func determineContentTypeByRequest(url string) string {
	// 创建一次性使用的HTTP客户端
	client := &http.Client{
		Timeout: config.Conf.Timeout * time.Second,
		Transport: &http.Transport{
			TLSClientConfig:   &tls.Config{InsecureSkipVerify: true},
			DisableKeepAlives: true,
		},
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Printf("创建请求失败: %v", err)
		return "bookget"
	}

	req.Header.Set("User-Agent", config.Conf.UserAgent)
	req.Header.Set("Range", "bytes=0-0") // 只请求头信息

	resp, err := client.Do(req)
	if err != nil {
		log.Printf("请求失败: %v", err)
		return "bookget"
	}
	defer resp.Body.Close()

	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		return cacheAndReturn(url, "bookget")
	}

	// 去除可能的参数部分（如charset=utf-8）
	contentType = strings.Split(contentType, ";")[0]
	contentType = strings.TrimSpace(contentType)

	if result, ok := contentTypeMappings[contentType]; ok {
		return cacheAndReturn(url, result)
	}

	// 默认返回bookget
	return cacheAndReturn(url, "bookget")
}

func cacheAndReturn(url, result string) string {
	contentTypeCache.Store(url, result)
	return result
}

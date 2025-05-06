package downloader

import (
	"fmt"
	"net/http"
	"strings"
)

const (
	maxConcurrent    = 16 // 最大并发下载数
	userAgent        = "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:139.0) Gecko/20100101 Firefox/139.0"
	minFileSize      = 1024 // 最小文件大小(1KB)
	progressBarWidth = 50
)

// 将curl风格的header参数转换为http.Header
func argsToHeaders(args []string) (http.Header, error) {
	headers := make(http.Header)

	for i := 0; i < len(args); i++ {
		if args[i] == "-H" {
			if i+1 >= len(args) {
				return nil, fmt.Errorf("缺少header值")
			}
			headerStr := args[i+1]
			i++ // 跳过下一个参数，因为已经处理了

			// 分割header键值对
			parts := strings.SplitN(headerStr, ":", 2)
			if len(parts) != 2 {
				return nil, fmt.Errorf("无效的header格式: %s", headerStr)
			}

			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			headers.Add(key, value)
		}
	}

	return headers, nil
}

// 使用header创建HTTP请求
func createRequestWithHeaders(url string, headers http.Header) (*http.Request, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	for key, values := range headers {
		for _, value := range values {
			req.Header.Add(key, value)
		}
	}

	return req, nil
}

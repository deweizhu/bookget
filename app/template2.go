package app

import (
	"bookget/config"
	"bookget/pkg/chttp"
	"bookget/pkg/sharedmemory"
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"time"
)

type Downloader interface {
	NewDownloader() *Downloader
	GetRouterInit(rawUrl string) (map[string]interface{}, error)
	getBookId(rawUrl string) (bookId string)
	Run() (err error)
	do(canvases []string) (err error)
	getVolumes(rawUrl string) (volumes []string, err error)
	getCanvases(rawUrl string) (canvases []string, err error)
	getBody(rawUrl string) ([]byte, error)
	postBody(rawUrl string, postData interface{}) ([]byte, error)
	getBodyByGui(apiUrl string) (bs []byte, err error)
	imageDownloader(imgUrl, targetFilePath string) (ok bool, err error)
}

type DownloaderImpl struct {
	ctx    context.Context
	cancel context.CancelFunc
	client *http.Client

	bufBuilder strings.Builder
	bufString  string
	bufBody    []byte
	canvases   []string
	urlsFile   string

	rawUrl    string
	parsedUrl *url.URL
	savePath  string
	bookId    string
}

// Implement the NewDownloader method to satisfy the interface
func (d *DownloaderImpl) NewDownloader() *DownloaderImpl {
	ctx, cancel := context.WithCancel(context.Background())

	// 创建自定义 Transport 忽略 SSL 验证
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
	}
	jar, _ := cookiejar.New(nil)
	return &DownloaderImpl{
		// 初始化字段
		client: &http.Client{Timeout: config.Conf.Timeout * time.Second, Jar: jar, Transport: tr},
		ctx:    ctx,
		cancel: cancel,
	}
}

func (d *DownloaderImpl) GetRouterInit(rawUrl string) (map[string]interface{}, error) {
	d.rawUrl = rawUrl
	d.parsedUrl, _ = url.Parse(rawUrl)
	err := d.Run()
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"type": "",
		"url":  rawUrl,
	}, nil
}

func (d *DownloaderImpl) Run() (err error) {

	return nil
}

func (d *DownloaderImpl) getBodyByGui(rawUrl string) (bs []byte, err error) {
	err = sharedmemory.WriteURLToSharedMemory(rawUrl)
	if err != nil {
		fmt.Println("Failed to write to shared memory:", err)
		return
	}
	for i := 0; i < 300; i++ {
		time.Sleep(time.Second * 1)
		d.bufString, err = sharedmemory.ReadHTMLFromSharedMemory()
		if err == nil && d.bufString != "" && !strings.Contains(d.bufString, "window.awsWafCookieDomainList") {
			break
		}
	}
	return []byte(d.bufString), nil
}

func (d *DownloaderImpl) imageDownloader(imgUrl, targetFilePath string) (ok bool, err error) {
	err = sharedmemory.WriteURLImagePathToSharedMemory(imgUrl, targetFilePath)
	if err != nil {
		fmt.Println("Failed to write to shared memory:", err)
		return
	}
	for i := 0; i < 300; i++ {
		time.Sleep(time.Second * 1)
		ok, err = sharedmemory.ReadImageReadyFromSharedMemory()
		if err != nil || !ok {
			continue
		}
		break
	}
	return ok, nil
}

func (d *DownloaderImpl) getBody(rawUrl string) ([]byte, error) {
	req, err := http.NewRequest("GET", rawUrl, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", config.Conf.UserAgent)
	cookies := chttp.CookiesFromFile(config.Conf.CookieFile)
	if cookies != "" {
		req.Header.Set("Cookie", cookies)
	}
	headers, err := chttp.ReadHeadersFromFile(config.Conf.HeaderFile)
	if err == nil {
		for key, value := range headers {
			req.Header.Set(key, value)
		}
	}
	resp, err := d.client.Do(req.WithContext(d.ctx))
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			log.Printf("close body err=%v", err)
		}
	}()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
		err = fmt.Errorf("服务器返回错误状态码: %d", resp.StatusCode)
		return nil, err
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return body, nil
}

func (d *DownloaderImpl) postBody(rawUrl string, postData interface{}) ([]byte, error) {
	var bodyData []byte
	var err error

	var isJSON = false
	// 根据传入的数据类型处理
	switch v := postData.(type) {
	case []byte:
		bodyData = v // 直接使用 []byte
	case string:
		bodyData = []byte(v) // 将 string 转为 []byte
	default:
		// 其他类型尝试转为 JSON
		bodyData, err = json.Marshal(v)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal JSON data: %v", err)
		}
		isJSON = true
	}
	req, err := http.NewRequest("POST", rawUrl, bytes.NewBuffer(bodyData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}
	// 设置请求头
	req.Header.Set("User-Agent", config.Conf.UserAgent)
	if isJSON {
		req.Header.Set("Content-Type", "application/json")
	} else {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	req.Header.Set("Origin", "https://"+d.parsedUrl.Host)
	req.Header.Set("Referer", d.rawUrl)

	cookies := chttp.CookiesFromFile(config.Conf.CookieFile)
	if cookies != "" {
		req.Header.Set("Cookie", cookies)
	}

	headers, err := chttp.ReadHeadersFromFile(config.Conf.HeaderFile)
	if err == nil {
		for key, value := range headers {
			req.Header.Set(key, value)
		}
	}
	resp, err := d.client.Do(req.WithContext(d.ctx))
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			log.Printf("close body err=%v", err)
		}
	}()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
		err = fmt.Errorf("服务器返回错误状态码: %d", resp.StatusCode)
		return nil, err
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return body, nil
}

func (d *DownloaderImpl) buildRequestHeader() map[string]string {
	httpHeaders := map[string]string{"User-Agent": config.Conf.UserAgent}
	cookies := chttp.CookiesFromFile(config.Conf.CookieFile)
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

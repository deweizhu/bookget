package app

import (
	"bookget/config"
	"bookget/pkg/chttp"
	"bookget/pkg/downloader"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"regexp"
	"time"
)

type Gzlib struct {
	dm     *downloader.DownloadManager
	ctx    context.Context
	cancel context.CancelFunc
	client *http.Client

	rawUrl    string
	parsedUrl *url.URL
	savePath  string
	bookId    string
}

func NewGzlib() *Gzlib {
	ctx, cancel := context.WithCancel(context.Background())
	dm := downloader.NewDownloadManager(ctx, cancel, config.Conf.MaxConcurrent)

	// 创建自定义 Transport 忽略 SSL 验证
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
	}
	jar, _ := cookiejar.New(nil)
	return &Gzlib{
		// 初始化字段
		dm:     dm,
		client: &http.Client{Timeout: config.Conf.Timeout * time.Second, Jar: jar, Transport: tr},
		ctx:    ctx,
		cancel: cancel,
	}
}

func (r *Gzlib) GetRouterInit(sUrl string) (map[string]interface{}, error) {
	r.rawUrl = sUrl
	r.parsedUrl, _ = url.Parse(sUrl)
	msg, err := r.Run()
	return map[string]interface{}{
		"url": sUrl,
		"msg": msg,
	}, err
}

func (r *Gzlib) getBookId() (bookId string) {
	m := regexp.MustCompile(`(?i)id=([A-z0-9_-]+)`).FindStringSubmatch(r.rawUrl)
	if m != nil {
		bookId = m[1]
	}
	return bookId
}

func (r *Gzlib) Run() (msg string, err error) {
	r.bookId = r.getBookId()
	if r.bookId == "" {
		return "[err=getBookId]", err
	}
	r.savePath = config.Conf.Directory

	apiUrl := fmt.Sprintf("https://%s/attach/GZDD/Attach/%s.pdf", r.parsedUrl.Hostname(), r.bookId)
	fileName := fmt.Sprintf("%s.pdf", r.bookId)

	headers := BuildRequestHeader()
	r.dm.UseSizeBar = true
	// 添加GET下载任务
	r.dm.AddTask(
		apiUrl,
		"GET",
		headers,
		nil,
		r.savePath,
		fileName,
		config.Conf.Threads,
	)
	r.dm.Start()

	return "", err
}

func (r *Gzlib) getBody(sUrl string) ([]byte, error) {
	req, err := http.NewRequest("GET", sUrl, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", config.Conf.UserAgent)
	req.Header.Set("Origin", "https://"+r.parsedUrl.Host)
	req.Header.Set("Referer", r.rawUrl)

	cookies, _ := chttp.ReadCookiesFromFile(config.Conf.CookieFile)
	if cookies != "" {
		req.Header.Set("Cookie", cookies)
	}

	headers, err := chttp.ReadHeadersFromFile(config.Conf.HeaderFile)
	if err == nil {
		for key, value := range headers {
			req.Header.Set(key, value)
		}
	}

	resp, err := r.client.Do(req.WithContext(r.ctx))
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

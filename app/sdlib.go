package app

import (
	"bookget/config"
	"bookget/model/sdlib"
	"bookget/pkg/chttp"
	"bookget/pkg/downloader"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

type Sdlib struct {
	dm     *downloader.DownloadManager
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

func NewSdlib() *Sdlib {
	ctx, cancel := context.WithCancel(context.Background())
	dm := downloader.NewDownloadManager(ctx, cancel, config.Conf.MaxConcurrent)

	// 创建自定义 Transport 忽略 SSL 验证
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
	}
	jar, _ := cookiejar.New(nil)
	return &Sdlib{
		// 初始化字段
		dm:     dm,
		client: &http.Client{Timeout: config.Conf.Timeout * time.Second, Jar: jar, Transport: tr},
		ctx:    ctx,
		cancel: cancel,
	}
}

func (r *Sdlib) GetRouterInit(rawUrl string) (map[string]interface{}, error) {
	r.rawUrl = rawUrl
	r.parsedUrl, _ = url.Parse(rawUrl)
	err := r.Run()
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"type": "",
		"url":  rawUrl,
	}, nil
}

func (r *Sdlib) getBookId(rawUrl string) (bookId string) {
	const (
		idPattern = `(?i)\?resId=([A-Za-z0-9_-]+)`
	)

	// 预编译正则表达式
	var (
		idRe = regexp.MustCompile(idPattern)
	)

	// 然后尝试匹配 id
	if matches := idRe.FindStringSubmatch(r.rawUrl); matches != nil && len(matches) > 1 {
		return matches[1]
	}

	return "" // 明确返回空字符串表示未找到
}

func (r *Sdlib) Run() (err error) {
	r.bookId = r.getBookId(r.rawUrl)
	if r.bookId == "" {
		return err
	}
	r.savePath = config.Conf.Directory
	r.urlsFile = path.Join(r.savePath, "urls.txt")

	r.canvases, err = r.getCanvases(r.rawUrl)
	if err != nil || len(r.canvases) == 0 {
		return err
	}

	err = os.WriteFile(r.urlsFile, []byte(r.bufBuilder.String()), os.ModePerm)
	if err != nil {
		return err
	}

	r.do(r.canvases)

	return nil
}

func (r *Sdlib) do(canvases []string) (err error) {
	sizeVol := len(canvases)
	if sizeVol <= 0 {
		return err
	}

	counter := 0
	headers := BuildRequestHeader()
	for i, imgUrl := range canvases {
		i++
		sortId := fmt.Sprintf("%04d", i)
		fileName := sortId + filepath.Ext(imgUrl)

		if imgUrl == "" || !config.PageRange(i, sizeVol) {
			continue
		}
		//跳过存在的文件
		if FileExist(path.Join(r.savePath, fileName)) {
			continue
		}
		// 添加GET下载任务
		r.dm.AddTask(
			imgUrl,
			"GET",
			headers,
			nil,
			r.savePath,
			fileName,
			config.Conf.Threads,
		)
		counter++
	}
	fmt.Println()
	r.dm.SetBar(counter)
	r.dm.Start()
	return nil
}

func (r *Sdlib) getVolumes(rawUrl string) (volumes []string, err error) {
	//TODO implement me
	panic("implement me")
}

func (r *Sdlib) getCanvases(rawUrl string) (canvases []string, err error) {
	apiUrl := fmt.Sprintf("http://%s/dev-api/ancientbooks/front/getFileContentPage/3/", r.parsedUrl.Host, r.bookId)
	r.bufBody, err = r.getBody(apiUrl)
	if err != nil {
		return nil, err
	}
	resp := sdlib.Response{}
	if err = json.Unmarshal(r.bufBody, &resp); err != nil {
		return nil, err
	}
	r.canvases = make([]string, 0, len(resp.Data))
	for _, d := range resp.Data {
		r.bufBuilder.WriteString(d.Url)
		r.bufBuilder.WriteString("\n")
		r.canvases = append(r.canvases, d.Url)
	}
	return r.canvases, err
}

func (r *Sdlib) getBody(rawUrl string) ([]byte, error) {
	req, err := http.NewRequest("GET", rawUrl, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", config.Conf.UserAgent)
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

func (r *Sdlib) postBody(rawUrl string, postData interface{}) ([]byte, error) {
	//TODO implement me
	panic("implement me")
}

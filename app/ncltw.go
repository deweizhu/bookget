package app

import (
	"bookget/config"
	"bookget/pkg/chttp"
	xhash "bookget/pkg/hash"
	"bookget/pkg/sharedmemory"
	"bookget/pkg/util"
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"path"
	"strings"
	"time"
)

type NlcTw struct {
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
	serverURL string
	savePath  string
	bookId    string
}

func (r *NlcTw) NewNlcTw() *NlcTw {
	ctx, cancel := context.WithCancel(context.Background())

	// 创建自定义 Transport 忽略 SSL 验证
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
	}
	jar, _ := cookiejar.New(nil)
	return &NlcTw{
		// 初始化字段
		client: &http.Client{Timeout: config.Conf.Timeout * time.Second, Jar: jar, Transport: tr},
		ctx:    ctx,
		cancel: cancel,
	}
}
func (d *NlcTw) GetRouterInit(rawUrl string) (map[string]interface{}, error) {
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

func (r *NlcTw) getBookId(rawUrl string) (bookId string) {
	mh := xhash.NewMultiHasher()
	_, _ = io.Copy(mh, bytes.NewBuffer([]byte(rawUrl)))
	bookId, _ = mh.SumString(xhash.QuickXorHash, false)
	return bookId
}

func (r *NlcTw) Run() (err error) {
	r.bookId = r.getBookId(r.rawUrl)
	if r.bookId == "" {
		return err
	}
	r.savePath = config.Conf.Directory
	r.urlsFile = path.Join(r.savePath, "urls.txt")
	//開始工作了
	if os.PathSeparator != '\\' {
		return errors.New("此网站只能在 Windows 操作系统下使用本软件。")
	}

	if util.OpenWebBrowser([]string{"-i", r.rawUrl}) {
		fmt.Println("已启动 bookget-gui 浏览器，请注意完成「真人验证」。")
		for i := 0; i < 10; i++ {
			fmt.Printf("等待 bookget-gui 加载完成，还有 %d 秒 \r", 10-i)
			time.Sleep(time.Second * 1)
		}
	}
	fmt.Println()

	r.serverURL = "https://" + r.parsedUrl.Host + "/NCLSearch/WaterMark/GetVideoImage"
	r.bufBody, err = r.getBodyByGui(r.rawUrl)

	//保存URLs
	err = os.WriteFile(r.urlsFile, []byte(r.bufBuilder.String()), os.ModePerm)
	if err != nil {
		return err
	}
	fmt.Printf("\n已生成图片URLs文件[%s]\n 可复制到 bookget-gui.exe 目录下，或使用其它软件下载。\n", r.urlsFile)

	return err
}

func (r *NlcTw) do(canvases []string) (err error) {
	//TODO implement me
	panic("implement me")
}

func (r *NlcTw) getVolumes(rawUrl string) (volumes []string, err error) {
	//TODO implement me
	panic("implement me")
}

func (r *NlcTw) getCanvases(rawUrl string) (canvases []string, err error) {
	//TODO implement me
	panic("implement me")
}

func (r *NlcTw) getBody(rawUrl string) ([]byte, error) {
	req, err := http.NewRequest("GET", rawUrl, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", config.Conf.UserAgent)
	cookies := chttp.CookiesFromFile(config.Conf.CookieFile)
	if cookies != "" {
		req.Header.Set("Cookie", cookies)
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

func (r *NlcTw) postBody(rawUrl string, postData interface{}) ([]byte, error) {
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
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.8,zh-TW;q=0.7,zh-HK;q=0.5,en-US;q=0.3,en;q=0.2")
	req.Header.Set("Origin", "https://"+r.parsedUrl.Host)
	req.Header.Set("Referer", r.rawUrl)

	cookies := chttp.CookiesFromFile(config.Conf.CookieFile)
	if cookies != "" {
		req.Header.Set("Cookie", cookies)
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

func (r *NlcTw) getBodyByGui(apiUrl string) (bs []byte, err error) {
	err = sharedmemory.WriteURLToSharedMemory(apiUrl)
	if err != nil {
		fmt.Println("Failed to write to shared memory:", err)
		return
	}
	for i := 0; i < 300; i++ {
		time.Sleep(time.Second * 1)
		r.bufString, err = sharedmemory.ReadHTMLFromSharedMemory()
		if err == nil && r.bufString != "" && !strings.Contains(r.bufString, "id=\"Identifier_BookNo\"") {
			break
		}
	}
	return []byte(r.bufString), nil
}

func (r *NlcTw) imageDownloader(imgUrl, targetFilePath string) (ok bool, err error) {
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

package app

import (
	"bookget/config"
	"bookget/model/family"
	"bookget/pkg/chttp"
	"bookget/pkg/downloader"
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
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

type Familysearch struct {
	ctx    context.Context
	cancel context.CancelFunc
	client *http.Client
	dm     *downloader.DownloadManager

	urlsFile   string
	bufBuilder strings.Builder
	bufBody    []byte
	bufString  string
	canvases   []string

	rawUrl    string
	parsedUrl *url.URL
	savePath  string
	bookId    string

	urlType     int
	dziTemplate string
	baseUrl     string
	sgBaseUrl   string
	apiUrl      string
}

func NewFamilysearch() *Familysearch {
	ctx, cancel := context.WithCancel(context.Background())
	dm := downloader.NewDownloadManager(ctx, cancel, config.Conf.MaxConcurrent)

	// 创建自定义 Transport 忽略 SSL 验证
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
	}
	jar, _ := cookiejar.New(nil)

	return &Familysearch{
		// 初始化字段
		dm:     dm,
		client: &http.Client{Timeout: config.Conf.Timeout * time.Second, Jar: jar, Transport: tr},
		ctx:    ctx,
		cancel: cancel,
	}
}

func (r *Familysearch) GetRouterInit(sUrl string) (map[string]interface{}, error) {
	r.rawUrl = sUrl
	r.parsedUrl, _ = url.Parse(r.rawUrl)
	r.apiUrl = "https://" + r.parsedUrl.Host + "/search/filmdatainfo/image-data"
	msg, err := r.Run()
	return map[string]interface{}{
		"type": "iiif",
		"url":  sUrl,
		"msg":  msg,
	}, err
}

func (r *Familysearch) getBookId(sUrl string) (bookId string) {
	m := regexp.MustCompile(`(?i)ark:/(?:[A-z0-9-_:]+)/([A-z\d-_:]+)`).FindStringSubmatch(sUrl)
	if m != nil {
		bookId = m[1]
	}
	return bookId
}

func (r *Familysearch) getBaseUrl(sUrl string) (string, string, error) {
	bs, err := r.getBody(sUrl)
	if err != nil {
		return "", "", err
	}

	//SERVER_DATA.baseUrl = "https://www.familysearch.org";
	m := regexp.MustCompile(`SERVER_DATA.baseUrl\s=\s"([^"]+)"`).FindSubmatch(bs)
	baseUrl := string(m[1])

	// SERVER_DATA.sgBaseUrl = "https://sg30p0.familysearch.org"
	m = regexp.MustCompile(`SERVER_DATA.sgBaseUrl\s=\s"([^"]+)"`).FindSubmatch(bs)
	sgBaseUrl := string(m[1])
	return baseUrl, sgBaseUrl, nil
}

func (r *Familysearch) Run() (msg string, err error) {
	r.bookId = r.getBookId(r.rawUrl)
	if r.bookId == "" {
		return "requested URL was not found.", err
	}
	if os.PathSeparator == '\\' {
		if util.OpenWebBrowser([]string{"-i", r.rawUrl}) {
			fmt.Println("已启动 bookget-gui 浏览器，请注意完成「真人验证」或「账号登录」。")
			for i := 0; i < 10; i++ {
				fmt.Printf("等待 bookget-gui 加载完成，还有 %d 秒 \r", 10-i)
				time.Sleep(time.Second * 1)
			}
		}
		fmt.Println()
	}
	r.baseUrl, r.sgBaseUrl, err = r.getBaseUrl(r.rawUrl)
	if err != nil || r.baseUrl == "" || r.sgBaseUrl == "" {
		return "", err
	}

	imageData, err := r.getImageData(r.apiUrl)
	if err != nil {
		return "", err
	}
	r.canvases, err = r.getCanvases(r.apiUrl, imageData)
	if err != nil || r.canvases == nil {
		return "", err
	}
	r.savePath = config.Conf.Directory
	r.urlsFile = path.Join(r.savePath, "urls.txt")
	err = os.WriteFile(r.urlsFile, []byte(r.bufBuilder.String()), os.ModePerm)
	if err != nil {
		return "", err
	}

	r.do(r.canvases)
	return "", nil
}

func (r *Familysearch) do(canvases []string) (err error) {
	sizeVol := len(canvases)
	if sizeVol <= 0 {
		return errors.New("[err=do]")
	}
	referer := url.QueryEscape(r.rawUrl)
	sid := r.getSessionId()
	args := []string{
		"-H", "authority:www.familysearch.org",
		"-H", "Authorization:" + sid,
		"-H", "referer:" + referer,
	}
	// 创建下载器实例
	iiifDownloader := downloader.NewIIIFDownloader(&config.Conf)
	iiifDownloader.SetDeepZoomTileFormat("{{.ServerBaseURL}}/{{.URL}}_files/{{.Level}}/{{.X}}_{{.Y}}.{{.Format}}")
	// 设置固定值
	iiifDownloader.DeepzoomTileFormat.FixedValues = map[string]interface{}{
		//"Level":     12,
		"Format":        "jpg",
		"ServerBaseURL": r.sgBaseUrl,
	}
	for i, uri := range canvases {
		if uri == "" || !config.PageRange(i, sizeVol) {
			continue
		}
		sortId := fmt.Sprintf("%04d", i+1)
		dest := filepath.Join(r.savePath, sortId+config.Conf.FileExt)
		if FileExist(dest) {
			continue
		}
		log.Printf("Get %d/%d  %s\n", i+1, sizeVol, uri)
		iiifDownloader.Dezoomify(r.ctx, uri, dest, args)
		util.PrintSleepTime(config.Conf.Sleep)
	}
	return err
}

func (r *Familysearch) getImageData(sUrl string) (imageData family.ImageData, err error) {

	var data = family.ReqData{}
	data.Type = "image-data"
	data.Args.ImageURL = r.rawUrl
	data.Args.State.ImageOrFilmUrl = ""
	data.Args.State.ViewMode = "i"
	data.Args.State.SelectedImageIndex = -1
	data.Args.Locale = "zh"

	bs, err := r.postBody(sUrl, data)
	if err != nil {
		fmt.Println("请求失败，cookie 可能已失效。")
		return
	}
	var resultError family.ResultError
	if err = json.Unmarshal(bs, &resultError); resultError.Error.StatusCode != 0 {
		msg := fmt.Sprintf("StatusCode: %d, Message: %s", resultError.Error.StatusCode, resultError.Error.Message)
		err = errors.New(msg)
		return
	}
	resp := family.Response{}
	if err = json.Unmarshal(bs, &resp); err != nil {
		return
	}
	imageData.DgsNum = resp.DgsNum
	imageData.ImageURL = resp.ImageURL
	for _, description := range resp.Meta.SourceDescriptions {
		if strings.Contains(description.About, "platform/records/waypoints") {
			imageData.WaypointURL = description.About
			break
		}
	}
	return imageData, nil
}

func (r *Familysearch) getCanvases(sUrl string, imageData family.ImageData) (canvases []string, err error) {

	u, err := url.Parse(imageData.ImageURL)
	if err != nil {
		return nil, err
	}
	q := u.Query()
	var data = family.FilmDataReqData{}
	data.Type = "film-data"
	data.Args.DgsNum = imageData.DgsNum
	data.Args.State.CatalogContext = q.Get("cat")
	data.Args.State.Cat = q.Get("cat")
	data.Args.State.ImageOrFilmUrl = u.Path
	data.Args.State.ViewMode = "i"
	data.Args.State.SelectedImageIndex = -1
	data.Args.Locale = "zh"
	data.Args.LoggedIn = true
	data.Args.SessionId = r.getSessionId()

	bs, err := r.postBody(sUrl, data)
	if err != nil {
		return
	}
	var resultError family.ResultError
	if err = json.Unmarshal(bs, &resultError); resultError.Error.StatusCode != 0 {
		msg := fmt.Sprintf("StatusCode: %d, Message: %s", resultError.Error.StatusCode, resultError.Error.Message)
		err = errors.New(msg)
		return
	}
	resp := family.FilmDataResponse{}
	if err = json.Unmarshal(bs, &resp); err != nil {
		return
	}
	//https://sg30p0.familysearch.org/service/records/storage/deepzoomcloud/dz/v1/{id}/{image}
	r.dziTemplate = regexp.MustCompile(`\{[A-z]+\}`).ReplaceAllString(resp.Templates.DzTemplate, "%s")
	r.dziTemplate = regexp.MustCompile(`https://([^/]+)`).ReplaceAllString(r.dziTemplate, r.baseUrl)
	for _, image := range resp.Images {
		//https://www.familysearch.org/service/records/storage/deepzoomcloud/dz/v1/3:1:3QSQ-G9DL-LLT2/image.xml
		//https://familysearch.org/ark:/61903/3:1:3QSQ-G9MC-ZSQ7-3/image.xml
		m := regexp.MustCompile(`(?i)ark:/(?:[A-z0-9-_:]+)/([A-z\d-_:]+)/image.xml`).FindStringSubmatch(image)
		if m == nil {
			continue
		}
		xmlUrl := fmt.Sprintf(r.dziTemplate, m[1], "image.xml")
		canvases = append(canvases, xmlUrl)

		r.bufBuilder.WriteString(xmlUrl)
		r.bufBuilder.WriteString("\n")
	}
	return canvases, err
}

func (r *Familysearch) getSessionId() string {
	cookies, err := chttp.ReadCookiesFromFile(config.Conf.CookieFile)
	if err != nil {
		return ""
	}
	//fssessionid=e10ce618-f7f7-45de-b2c3-d1a31d080d58-prod;
	m := regexp.MustCompile(`fssessionid=([^;]+);`).FindStringSubmatch(cookies)
	if m != nil {
		return "bearer " + m[1]
	}
	return ""
}

func (r *Familysearch) getBody(sUrl string) ([]byte, error) {
	req, err := http.NewRequest("GET", sUrl, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", config.Conf.UserAgent)
	req.Header.Set("accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7")
	req.Header.Set("authority", "www.familysearch.org")
	req.Header.Set("origin", r.baseUrl)
	req.Header.Set("referer", r.rawUrl)

	cookies, _ := chttp.ReadCookiesFromFile(config.Conf.CookieFile)
	if cookies != "" {
		req.Header.Set("Cookie", cookies)
		//sid := r.getSessionId(cookies)
		//req.Header.Set("authorization", sid)
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

func (r *Familysearch) postBody(sUrl string, postData interface{}) ([]byte, error) {
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
	req, err := http.NewRequest("POST", sUrl, bytes.NewBuffer(bodyData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}
	// 设置请求头
	req.Header.Set("User-Agent", config.Conf.UserAgent)
	if isJSON {
		req.Header.Set("accept", "application/json, text/plain, */*")
		req.Header.Set("Content-Type", "application/json")
	} else {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	req.Header.Set("authority", "www.familysearch.org")
	req.Header.Set("origin", r.baseUrl)
	req.Header.Set("referer", r.rawUrl)

	// 添加cookie
	cookies, err := chttp.ReadCookiesFromFile(config.Conf.CookieFile)
	if err == nil && cookies != "" {
		req.Header.Set("Cookie", cookies)
		sid := r.getSessionId()
		req.Header.Set("authorization", sid)
	}

	// 发送请求
	resp, err := r.client.Do(req.WithContext(r.ctx))
	if err != nil {
		return nil, fmt.Errorf("request failed: %v", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			log.Printf("warning: failed to close response body: %v", err)
		}
	}()

	// 检查响应状态码
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("server returned status %d: %s", resp.StatusCode, string(body))
	}

	// 读取响应体
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %v", err)
	}

	return body, nil
}

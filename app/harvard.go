package app

import (
	"bookget/config"
	"bookget/model/iiif"
	"bookget/pkg/chttp"
	"bookget/pkg/downloader"
	"bookget/pkg/progressbar"
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
	"regexp"
	"strings"
	"time"
)

type Harvard struct {
	ctx    context.Context
	cancel context.CancelFunc
	client *http.Client
	dm     *downloader.DownloadManager

	tmpFile    string
	urlsFile   string
	bufBuilder strings.Builder
	bufBody    []byte
	bufString  string
	canvases   []string

	rawUrl    string
	parsedUrl *url.URL
	savePath  string
	bookId    string
}

func NewHarvard() *Harvard {
	ctx, cancel := context.WithCancel(context.Background())
	dm := downloader.NewDownloadManager(ctx, cancel, config.Conf.MaxConcurrent)

	// 创建自定义 Transport 忽略 SSL 验证
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
	}
	jar, _ := cookiejar.New(nil)
	return &Harvard{
		// 初始化字段
		dm:     dm,
		client: &http.Client{Timeout: config.Conf.Timeout * time.Second, Jar: jar, Transport: tr},
		ctx:    ctx,
		cancel: cancel,
	}
}

func (r *Harvard) GetRouterInit(sUrl string) (map[string]interface{}, error) {
	r.rawUrl = sUrl
	r.parsedUrl, _ = url.Parse(r.rawUrl)
	msg, err := r.Run()
	return map[string]interface{}{
		"type": "iiif",
		"url":  sUrl,
		"msg":  msg,
	}, err
}

func (r *Harvard) getBookId(sUrl string) (bookId string) {
	m := regexp.MustCompile(`manifests/view/([A-z0-9-_:]+)`).FindStringSubmatch(sUrl)
	if m != nil {
		return m[1]
	}
	m = regexp.MustCompile(`/manifests/([A-z0-9-_:]+)`).FindStringSubmatch(sUrl)
	if m != nil {
		return m[1]
	}
	return ""
}

func (r *Harvard) Run() (msg string, err error) {
	r.bookId = r.getBookId(r.rawUrl)
	if r.bookId == "" {
		return "requested URL was not found.", err
	}

	if os.PathSeparator == '\\' {
		if util.OpenWebBrowser([]string{"-i", r.rawUrl}) {
			fmt.Println("已启动 bookget-gui 浏览器，请注意完成「真人验证」。")
			for i := 0; i < 10; i++ {
				fmt.Printf("等待 bookget-gui 加载完成，还有 %d 秒 \r", 10-i)
				time.Sleep(time.Second * 1)
			}
		}
		fmt.Println()
	}

	r.canvases, err = r.getCanvases()
	if err != nil || r.canvases == nil {
		return "", err
	}
	r.savePath = config.Conf.Directory
	r.urlsFile = path.Join(r.savePath, "urls.txt")
	err = os.WriteFile(r.urlsFile, []byte(r.bufBuilder.String()), os.ModePerm)
	if err != nil {
		return "", err
	}
	fmt.Printf("\n已生成图片URLs文件[%s]\n 可复制到 bookget-gui.exe 目录下，或使用其它软件下载。\n", r.urlsFile)

	r.do(r.canvases)
	return "", nil
}

func (r *Harvard) do(imgUrls []string) (err error) {
	if os.PathSeparator == '\\' {
		return r.doByGUI(imgUrls)
	}
	if config.Conf.UseDzi {
		return r.doDezoomify(imgUrls)
	}
	return r.doNormal(imgUrls)
}

func (r *Harvard) getCanvases() (canvases []string, err error) {
	var manifestUri = "https://" + r.parsedUrl.Host + "/manifests/" + r.bookId
	r.bufBody, err = r.tryGetBody(manifestUri)
	if err != nil {
		return
	}
	// 提取JSON部分
	start := bytes.Index(r.bufBody, []byte("<pre>")) + 5
	end := bytes.Index(r.bufBody, []byte("</pre>"))
	if start <= 0 || end <= 0 {
		return nil, err
	}
	r.bufBody = r.bufBody[start:end]

	var manifest = new(iiif.ManifestResponse)
	if err = json.Unmarshal(r.bufBody, manifest); err != nil {
		log.Printf("json.Unmarshal failed: %s\n", err)
		return
	}
	if len(manifest.Sequences) == 0 {
		return
	}
	size := len(manifest.Sequences[0].Canvases)
	canvases = make([]string, 0, size)
	for _, canvase := range manifest.Sequences[0].Canvases {
		for _, image := range canvase.Images {
			//JPEG URL
			imgUrl := image.Resource.Service.Id + "/" + config.Conf.Format
			//dezoomify-rs URL
			iiiInfo := fmt.Sprintf("%s/info.json", image.Resource.Service.Id)
			if config.Conf.UseDzi && os.PathSeparator != '\\' {
				canvases = append(canvases, iiiInfo)
			} else {
				canvases = append(canvases, imgUrl)
			}
			r.bufBuilder.WriteString(imgUrl)
			r.bufBuilder.WriteString("\n")
		}
	}
	return canvases, nil

}

func (r *Harvard) doDezoomify(canvases []string) (err error) {
	sizeVol := len(canvases)
	if sizeVol <= 0 {
		return errors.New("[err=doByGUI]")
	}
	referer := url.QueryEscape(r.rawUrl)
	iiifDownloader := downloader.NewIIIFDownloader(&config.Conf)
	for i, uri := range canvases {
		if uri == "" || !config.PageRange(i, sizeVol) {
			continue
		}
		sortId := fmt.Sprintf("%04d", i+1)
		filename := sortId + config.Conf.FileExt
		dest := path.Join(r.savePath, filename)
		if FileExist(dest) {
			continue
		}
		log.Printf("Get %d/%d  %s\n", i+1, sizeVol, uri)

		args := []string{
			"-H", "Origin:" + referer,
			"-H", "Referer:" + referer,
		}
		iiifDownloader.Dezoomify(r.ctx, uri, dest, args)
	}
	return nil
}

func (r *Harvard) doNormal(canvases []string) (err error) {
	sizeVol := len(canvases)
	if sizeVol <= 0 {
		return errors.New("[err=doNormal]")
	}
	fmt.Println()
	counter := 0
	for i, imgUrl := range canvases {
		if imgUrl == "" || !config.PageRange(i, sizeVol) {
			continue
		}
		ext := util.FileExt(imgUrl)
		sortId := fmt.Sprintf("%04d", i+1)
		fileName := sortId + ext
		dest := path.Join(r.savePath, fileName)
		if FileExist(dest) {
			continue
		}
		// 添加GET下载任务
		r.dm.AddTask(
			imgUrl,
			"GET",
			map[string]string{"User-Agent": config.Conf.UserAgent},
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

func (r *Harvard) doByGUI(canvases []string) (err error) {
	sizeVol := len(canvases)
	if sizeVol <= 0 {
		return errors.New("[err=doByGUI]")
	}
	fmt.Println()
	bar := progressbar.Default(int64(sizeVol), "downloading")
	for i, imgUrl := range canvases {
		i++
		sortId := fmt.Sprintf("%04d", i)
		fileName := sortId + config.Conf.FileExt

		if imgUrl == "" || !config.PageRange(i, sizeVol) {
			bar.Add(1)
			continue
		}
		//跳过存在的文件
		targetFilePath := path.Join(r.savePath, fileName)
		if FileExist(targetFilePath) {
			bar.Add(1)
			continue
		}

		ok, err := r.imageDownloader(imgUrl, targetFilePath)
		if err == nil && ok {
			bar.Add(1)
		}
	}
	fmt.Println()
	return nil
}

func (r *Harvard) tryGetBody(sUrl string) (bs []byte, err error) {
	if os.PathSeparator == '\\' {
		return r.getBodyByGui(sUrl)
	}
	return r.getBody(sUrl)
}

func (r *Harvard) getBodyByGui(apiUrl string) (bs []byte, err error) {
	err = sharedmemory.WriteURLToSharedMemory(apiUrl)
	if err != nil {
		fmt.Println("Failed to write to shared memory:", err)
		return
	}
	for i := 0; i < 300; i++ {
		time.Sleep(time.Second * 1)
		r.bufString, err = sharedmemory.ReadHTMLFromSharedMemory()
		if err == nil && r.bufString != "" && strings.Contains(r.bufString, "http://iiif.io/api/") {
			break
		}
	}
	r.bufBody = []byte(r.bufString)
	return r.bufBody, nil
}

func (r *Harvard) imageDownloader(imgUrl, targetFilePath string) (ok bool, err error) {
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

func (r *Harvard) getBody(sUrl string) ([]byte, error) {
	req, err := http.NewRequest("GET", sUrl, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", config.Conf.UserAgent)
	cookies, _ := chttp.ReadCookiesFromFile(config.Conf.CookieFile)
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

func (r *Harvard) postBody(sUrl string, postData interface{}) ([]byte, error) {
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
		req.Header.Set("Content-Type", "application/json")
	} else {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	// 添加cookie
	cookies, _ := chttp.ReadCookiesFromFile(config.Conf.CookieFile)
	if cookies != "" {
		req.Header.Set("Cookie", cookies)
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

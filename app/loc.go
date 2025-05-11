package app

import (
	"bookget/config"
	"bookget/model/loc"
	"bookget/pkg/downloader"
	"bookget/pkg/progressbar"
	"bookget/pkg/sharedmemory"
	"bookget/pkg/util"
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
	"regexp"
	"strings"
	"time"
)

type Loc struct {
	dm     *downloader.DownloadManager
	ctx    context.Context
	cancel context.CancelFunc
	client *http.Client

	responseBody []byte
	tmpFile      string
	urlsFile     string
	bufBuilder   strings.Builder
	bufBody      string
	canvases     []string

	rawUrl    string
	parsedUrl *url.URL
	savePath  string
	bookId    string
}

func NewLoc() *Loc {
	ctx, cancel := context.WithCancel(context.Background())
	dm := downloader.NewDownloadManager(ctx, cancel, config.Conf.MaxConcurrent)

	// 创建自定义 Transport 忽略 SSL 验证
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
	}
	jar, _ := cookiejar.New(nil)
	return &Loc{
		// 初始化字段
		dm:     dm,
		client: &http.Client{Timeout: config.Conf.Timeout * time.Second, Jar: jar, Transport: tr},
		ctx:    ctx,
		cancel: cancel,
	}
}

func (r *Loc) GetRouterInit(sUrl string) (map[string]interface{}, error) {
	r.rawUrl = sUrl
	r.parsedUrl, _ = url.Parse(sUrl)
	msg, err := r.Run()
	return map[string]interface{}{
		"url": sUrl,
		"msg": msg,
	}, err
}

func (r *Loc) getBookId() (bookId string) {
	m := regexp.MustCompile(`item/([A-Za-z0-9]+)`).FindStringSubmatch(r.rawUrl)
	if m != nil {
		bookId = m[1]
	}
	return bookId
}

func (r *Loc) Run() (msg string, err error) {

	r.bookId = r.getBookId()
	if r.bookId == "" {
		return "[err=getBookId]", err
	}
	r.savePath = CreateDirectory(r.parsedUrl.Host, r.bookId, "")

	apiUrl := fmt.Sprintf("https://www.loc.gov/item/%s/?fo=json", r.bookId)

	//windows 处理
	if os.PathSeparator == '\\' {
		running, err := util.IsBookgetGuiRunning()
		if err != nil {
			return "", err
		}
		if !running {
			go util.OpenWebBrowser([]string{"-i", apiUrl})
			fmt.Println("已启动 bookget-gui 浏览器，请注意完成「真人验证」。")
		}

		r.bufBody, err = r.getBodyByGui(apiUrl)
		// 提取JSON部分
		start := strings.Index(r.bufBody, "<pre>") + 5
		end := strings.Index(r.bufBody, "</pre>")
		if start <= 0 || end <= 0 || err != nil {
			return "[err=getBodyByGui]", err
		}
		r.bufBody = r.bufBody[start:end]
		r.responseBody = []byte(r.bufBody)
	} else {
		r.responseBody, err = r.getBody(apiUrl)
		r.bufBody = string(r.responseBody)
	}

	r.canvases, err = r.getCanvases()
	if err != nil || r.canvases == nil {
		return "", err
	}
	r.savePath = CreateDirectory(r.parsedUrl.Host, r.bookId, "")
	if os.PathSeparator == '\\' {
		r.urlsFile = r.savePath + "urls.txt"
		err = os.WriteFile(r.urlsFile, []byte(r.bufBuilder.String()), os.ModePerm)
		if err != nil {
			return "", err
		}
		fmt.Printf("\n已生成图片URLs文件[%s]\n 可复制到 bookget-gui.exe 目录下，或使用其它软件下载。\n", r.urlsFile)
		r.do(r.canvases)
	} else {
		r.letsGo(r.canvases)
	}
	return "", nil
}

func (r *Loc) do(canvases []string) (msg string, err error) {
	sizeVol := len(canvases)
	if sizeVol <= 0 {
		return "[err=letsGo]", err
	}

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
		targetFilePath := r.savePath + fileName
		if FileExist(r.savePath + fileName) {
			bar.Add(1)
			continue
		}

		ok, err := r.imageDownloader(imgUrl, targetFilePath)
		if err == nil && ok {
			bar.Add(1)
		}
	}
	fmt.Println()
	return "", err
}

func (r *Loc) letsGo(canvases []string) (msg string, err error) {
	sizeVol := len(canvases)
	if sizeVol <= 0 {
		return "[err=letsGo]", err
	}

	counter := 0
	for i, imgUrl := range canvases {
		i++
		sortId := fmt.Sprintf("%04d", i)
		fileName := sortId + config.Conf.FileExt

		if imgUrl == "" || !config.PageRange(i, sizeVol) {
			continue
		}
		//跳过存在的文件
		if FileExist(r.savePath + fileName) {
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
	return "", err
}

func (r *Loc) getCanvases() (canvases []string, err error) {
	var manifests = new(loc.ManifestsJson)
	if err = json.Unmarshal(r.responseBody, manifests); err != nil {
		return nil, err
	}

	for _, resource := range manifests.Resources {
		for _, file := range resource.Files {
			//每页有6种下载方式
			imgUrl, ok := r.getImagePage(file)
			if ok {
				r.bufBuilder.WriteString(imgUrl)
				r.bufBuilder.WriteString("\n")
				canvases = append(canvases, imgUrl)
			}
		}
	}
	return canvases, nil
}

//func (r *Loc) getVolumes() (volumes []string, err error) {
//	var manifests = new(loc.ManifestsJson)
//	if err = json.Unmarshal(r.responseBody, manifests); err != nil {
//		log.Printf("json.Unmarshal failed: %s\n", err)
//		return
//	}
//	//一本书有N卷
//	for _, resource := range manifests.Resources {
//		volumes = append(volumes, resource.Url)
//	}
//	return volumes, nil
//}

//func (r *Loc) getCanvases(sUrl string) ([]string, err error) {
//	var manifests = new(loc.ManifestsJson)
//	if err = json.Unmarshal(r.responseBody, manifests); err != nil {
//		return nil, err
//	}
//
//	for _, resource := range manifests.Resources {
//		if resource.Url != sUrl {
//			continue
//		}
//		for _, file := range resource.Files {
//			//每页有6种下载方式
//			imgUrl, ok := r.getImagePage(file)
//			if ok {
//				r.bufBuilder.WriteString(imgUrl)
//				r.bufBuilder.WriteString("\n")
//				r.canvases = append(r.canvases, imgUrl)
//			}
//		}
//	}
//	return r.canvases, nil
//}

func (r *Loc) getBody(sUrl string) (bs []byte, err error) {
	req, err := http.NewRequest("GET", sUrl, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", config.Conf.UserAgent)
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

func (r *Loc) getImagePage(fileUrls []loc.ImageFile) (downloadUrl string, ok bool) {
	for _, f := range fileUrls {
		if config.Conf.FileExt == ".jpg" && f.Mimetype == "image/jpeg" {
			if strings.Contains(f.Url, "full/pct:100/") {
				if config.Conf.Format != "" {
					downloadUrl = regexp.MustCompile(`full/pct:(.+)`).ReplaceAllString(f.Url, config.Conf.Format)
				} else {
					downloadUrl = f.Url
				}
				ok = true
				break
			}
		} else if f.Mimetype != "image/jpeg" {
			downloadUrl = f.Url
			ok = true
			break
		}
	}
	return
}

func (r *Loc) getBodyByGui(apiUrl string) (buf string, err error) {
	err = sharedmemory.WriteURLToSharedMemory(apiUrl)
	if err != nil {
		fmt.Println("Failed to write to shared memory:", err)
		return
	}
	for i := 0; i < 300; i++ {
		time.Sleep(time.Second * 1)
		r.bufBody, err = sharedmemory.ReadHTMLFromSharedMemory()
		if err == nil && r.bufBody != "" && strings.Contains(r.bufBody, "https://tile.loc.gov/image-services/iiif/") {
			break
		}
	}
	return r.bufBody, nil
}

func (r *Loc) imageDownloader(imgUrl, targetFilePath string) (ok bool, err error) {
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

package app

import (
	"bookget/config"
	"bookget/model/cuhk"
	"bookget/pkg/gohttp"
	"bookget/pkg/progressbar"
	"bookget/pkg/sharedmemory"
	"bookget/pkg/util"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
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

type Cuhk struct {
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

func NewCuhk() *Cuhk {
	ctx, cancel := context.WithCancel(context.Background())

	// 创建自定义 Transport 忽略 SSL 验证
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
	}
	jar, _ := cookiejar.New(nil)
	return &Cuhk{
		// 初始化字段
		client: &http.Client{Timeout: config.Conf.Timeout * time.Second, Jar: jar, Transport: tr},
		ctx:    ctx,
		cancel: cancel,
	}
}

func (r *Cuhk) GetRouterInit(sUrl string) (map[string]interface{}, error) {
	lastPos := strings.Index(sUrl, "#")
	if lastPos > 0 {
		r.rawUrl = strings.Replace(sUrl[:lastPos], "hk/sc/", "hk/en/", -1)
	} else {
		r.rawUrl = strings.Replace(sUrl, "hk/sc/", "hk/en/", -1)
	}
	r.parsedUrl, _ = url.Parse(r.rawUrl)
	msg, err := r.Run()
	return map[string]interface{}{
		"url": r.rawUrl,
		"msg": msg,
	}, err
}

func (r *Cuhk) getBookId() string {
	const (
		IdPattern  = `(?i)/item/([^#/]+)`
		IdPattern2 = `(?i)/object/([^#/]+)`
	)

	// 预编译正则表达式
	var (
		IdRe  = regexp.MustCompile(IdPattern)
		IdRe2 = regexp.MustCompile(IdPattern2)
	)

	// 优先尝试匹配 metadataId
	if matches := IdRe.FindStringSubmatch(r.rawUrl); matches != nil && len(matches) > 1 {
		return matches[1]
	}

	// 然后尝试匹配 id
	if matches := IdRe2.FindStringSubmatch(r.rawUrl); matches != nil && len(matches) > 1 {
		return matches[1]
	}

	return "" // 明确返回空字符串表示未找到
}

func (r *Cuhk) Run() (msg string, err error) {
	r.bookId = r.getBookId()
	if r.bookId == "" {
		return "[err=getBookId]", err
	}
	r.savePath = config.Conf.Directory

	if util.OpenWebBrowser([]string{"-i", r.rawUrl}) {
		fmt.Println("已启动 bookget-gui 浏览器，请注意完成「真人验证」。")
		for i := 0; i < 10; i++ {
			fmt.Printf("等待 bookget-gui 加载完成，还有 %d 秒 \r", 10-i)
			time.Sleep(time.Second * 1)
		}
	}
	fmt.Println()

	r.canvases, err = r.getCanvases(r.rawUrl)
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

func (r *Cuhk) do(canvases []string) (msg string, err error) {
	fmt.Println()
	sizeVol := len(canvases)
	bar := progressbar.Default(int64(sizeVol), "downloading")
	for i, uri := range canvases {
		if uri == "" || !config.PageRange(i, sizeVol) {
			bar.Add(1)
			continue
		}
		sortId := fmt.Sprintf("%04d", i+1)
		filename := sortId + config.Conf.FileExt
		targetFilePath := path.Join(r.savePath, filename)
		if FileExist(targetFilePath) {
			bar.Add(1)
			continue
		}
		ok, err := r.imageDownloader(uri, targetFilePath)
		if err == nil && ok {
			bar.Add(1)
		}
	}
	fmt.Println()
	return
}

func (r *Cuhk) getVolumes() (volumes []string, err error) {
	bs, err := r.getBodyByGui(r.rawUrl)
	subText := util.SubText(string(bs), "id=\"block-islandora-compound-object-compound-navigation-select-list\"", "id=\"book-viewer\">")
	matches := regexp.MustCompile(`value=['"]([A-z\d:_-]+)['"]`).FindAllStringSubmatch(subText, -1)
	if matches == nil {
		volumes = append(volumes, r.rawUrl)
		return
	}
	volumes = make([]string, 0, len(matches))
	for _, m := range matches {
		//value='ignore'
		if m[1] == "ignore" {
			continue
		}
		id := strings.Replace(m[1], ":", "-", 1)
		volumes = append(volumes, fmt.Sprintf("https://%s/en/item/%s", r.parsedUrl.Host, id))
	}
	return volumes, nil
}

func (r *Cuhk) getCanvases(sUrl string) (canvases []string, err error) {
	r.responseBody, err = r.getBodyByGui(sUrl)
	if err != nil {
		return
	}
	var resp cuhk.ResponsePage
	matches := regexp.MustCompile(`"pages":([^]]+)]`).FindSubmatch(r.responseBody)
	if matches == nil {
		return nil, errors.New("[err=getCanvases]")
	}
	data := []byte("{\"pages\":" + string(matches[1]) + "]}")
	if err = json.Unmarshal(data, &resp); err != nil {
		log.Printf("json.Unmarshal failed: %s\n", err)
	}
	for _, page := range resp.ImagePage {
		var imgUrl string
		//if config.Conf.UseDzi {
		//	//dezoomify-rs URL
		//	imgUrl = fmt.Sprintf("https://%s/iiif/2/%s/info.json", r.parsedUrl.Host, page.Identifier)
		//} else {
		imgUrl = fmt.Sprintf("https://%s/iiif/2/%s/%s", r.parsedUrl.Host, page.Identifier, config.Conf.Format)
		//}
		r.bufBuilder.WriteString(imgUrl)
		r.bufBuilder.WriteString("\n")
		r.canvases = append(r.canvases, imgUrl)
	}
	return r.canvases, err
}

func (r *Cuhk) getBodyByGui(apiUrl string) (bs []byte, err error) {
	err = sharedmemory.WriteURLToSharedMemory(apiUrl)
	if err != nil {
		fmt.Println("Failed to write to shared memory:", err)
		return
	}
	for i := 0; i < 300; i++ {
		time.Sleep(time.Second * 1)
		r.bufBody, err = sharedmemory.ReadHTMLFromSharedMemory()
		if err == nil && r.bufBody != "" && !strings.Contains(r.bufBody, "window.awsWafCookieDomainList") {
			break
		}
	}
	return []byte(r.bufBody), nil
}

func (r *Cuhk) imageDownloader(imgUrl, targetFilePath string) (ok bool, err error) {
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

func (r *Cuhk) getBody(apiUrl string, jar *cookiejar.Jar) ([]byte, error) {
	referer := url.QueryEscape(apiUrl)
	ctx := context.Background()
	cli := gohttp.NewClient(ctx, gohttp.Options{
		CookieFile: config.Conf.CookieFile,
		CookieJar:  jar,
		Headers: map[string]interface{}{
			"User-Agent": config.Conf.UserAgent,
			"Referer":    referer,
		},
	})
	resp, err := cli.Get(apiUrl)
	if err != nil {
		return nil, err
	}
	bs, _ := resp.GetBody()
	if resp.GetStatusCode() == 202 || bs == nil {
		return nil, errors.New(fmt.Sprintf("ErrCode:%d, %s", resp.GetStatusCode(), resp.GetReasonPhrase()))
	}
	return bs, nil
}

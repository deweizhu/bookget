package app

import (
	"bookget/config"
	"bookget/pkg/downloader"
	"bookget/pkg/sharedmemory"
	"bookget/pkg/util"
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type LodNLGoKr struct {
	dm     *downloader.DownloadManager
	ctx    context.Context
	cancel context.CancelFunc
	client *http.Client

	urlsFile   string
	bufBuilder strings.Builder
	bufBody    string
	canvases   []string

	rawUrl    string
	parsedUrl *url.URL
	savePath  string
	bookId    string
	ServerUrl string
	fileExt   string
}

func NewLodNLGoKr() *LodNLGoKr {
	ctx, cancel := context.WithCancel(context.Background())
	dm := downloader.NewDownloadManager(ctx, cancel, config.Conf.MaxConcurrent)

	// 创建自定义 Transport 忽略 SSL 验证
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
	}
	jar, _ := cookiejar.New(nil)

	return &LodNLGoKr{
		// 初始化字段
		dm:        dm,
		client:    &http.Client{Timeout: config.Conf.Timeout * time.Second, Jar: jar, Transport: tr},
		ctx:       ctx,
		cancel:    cancel,
		ServerUrl: "http://viewer.nl.go.kr:8080", //"https://viewer.nl.go.kr"
		fileExt:   ".jpg",
	}
}

func (r *LodNLGoKr) GetRouterInit(sUrl string) (map[string]interface{}, error) {
	r.rawUrl = sUrl
	r.parsedUrl, _ = url.Parse(sUrl)
	msg, err := r.Run()
	return map[string]interface{}{
		"url": sUrl,
		"msg": msg,
	}, err
}

func (r *LodNLGoKr) getBookId(sUrl string) (bookId string) {
	m := regexp.MustCompile(`/resource/([A-z0-9_-]+)`).FindStringSubmatch(sUrl)
	if m != nil {
		bookId = m[1]
		return
	}
	m = regexp.MustCompile(`/page/([A-z0-9_-]+)`).FindStringSubmatch(sUrl)
	if m != nil {
		bookId = m[1]
	}
	return bookId
}

func (r *LodNLGoKr) Run() (msg string, err error) {
	r.bookId = r.getBookId(r.rawUrl)
	if r.bookId == "" {
		return "[err=getBookId]", err
	}
	r.savePath = CreateDirectory(r.parsedUrl.Host, r.bookId, "")

	webPageUrl := r.ServerUrl + "/nlmivs/viewWonmun_js.jsp?card_class=L&cno=" + r.bookId
	if util.OpenWebBrowser([]string{"-i", webPageUrl}) {
		fmt.Println("已启动 bookget-gui 浏览器，请注意完成「真人验证」。")
	}
	r.bufBody, err = r.getBodyByGui(webPageUrl)
	if err != nil || r.bufBody == "" {
		return "[err=getBodyByGui]", err
	}

	r.savePath = CreateDirectory(r.parsedUrl.Host, r.bookId, "")

	//PDF
	if strings.Contains(r.bufBody, "extention = \"PDF\";") {
		r.fileExt = ".pdf"
		m := regexp.MustCompile(`DEFAULT_URL\s=\s["']([^;]+)["'];`).FindStringSubmatch(r.bufBody)
		if m == nil {
			return "requested URL was not found.", err
		}
		pdfUrl := r.ServerUrl + m[1]
		return r.do([]string{pdfUrl})
	}
	//file ext
	r.fileExt = ".png"
	if match := regexp.MustCompile(`ext = "([A-z]+)"`).FindStringSubmatch(r.bufBody); match != nil && match[1] != "TIF" {
		r.fileExt = "." + match[1]
	}

	respVolume, err := r.getVolumeUrls()
	if err != nil {
		fmt.Println(err)
		return "getVolumes", err
	}
	for i, vol := range respVolume {
		if !config.VolumeRange(i) {
			continue
		}
		vid := fmt.Sprintf("%04d", i+1)
		r.savePath = CreateDirectory(r.parsedUrl.Host, r.bookId, vid)
		r.canvases, err = r.getCanvasesByUrl(i, vol.Url)
		if err != nil || r.canvases == nil {
			fmt.Println(err)
			continue
		}
		r.do(r.canvases)
	}
	return "", err
}

func (r *LodNLGoKr) do(canvases []string) (msg string, err error) {
	sizeVol := len(canvases)
	if sizeVol <= 0 {
		return "[err=letsGo]", err
	}
	fmt.Println()
	if r.fileExt != ".pdf" {
		config.Conf.Threads = 1
	}
	counter := 0
	for i, imgUrl := range canvases {
		if imgUrl == "" || !config.PageRange(i, sizeVol) {
			continue
		}
		sortId := fmt.Sprintf("%04d", i+1)
		fileName := sortId + r.fileExt
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

func (r *LodNLGoKr) getVolumeUrls() (volumes []Volume, err error) {
	matches := regexp.MustCompile(`<h2 class="volTitle(?:[^"]*)">([^<]+)</h2>`).FindAllStringSubmatch(r.bufBody, -1)
	if matches == nil {
		return
	}
	for i, m := range matches {
		vol := Volume{
			Title: strings.TrimSpace(m[1]),
			Url:   "",
			Seq:   1,
		}
		vol.Url = fmt.Sprintf("%s/main.wviewer?card_class=L&cno=%s&vol=%d&page=1", r.ServerUrl, r.bookId, i+1)
		volumes = append(volumes, vol)
		//r.bookMark += m[1] + "......" + m[2] + "\r\n"
	}
	return volumes, err
}

func (r *LodNLGoKr) getCanvasesByUrl(volId int, sUrl string) ([]string, error) {
	//loadVol('CNTS-00047981911',1,'/wonmun5/data4/imagedb/ncldb7/KOL000021672',155,'26');
	matches := regexp.MustCompile(`loadVol\(([^,]+),([^,]+),([^,]+),([^,]+),([^,]+)\);`).FindAllStringSubmatch(r.bufBody, -1)
	if matches == nil || len(matches) == 0 {
		return nil, errors.New("[getCanvases] not found")
	}
	var maxPage = 0
	for i, m := range matches {
		vid, _ := strconv.Atoi(m[2])
		if i == 0 && vid == 1 {
			volId++
		}
		if vid == volId {
			maxPage, _ = strconv.Atoi(m[4])
			break
		}
	}
	for i := 1; i <= maxPage; i++ {
		//imgUrl := fmt.Sprintf("%s/nlmivs/view_image.jsp?cno=%s&vol=%d&page=%d&twoThreeYn=N", r.apiUrl, r.dt.BookId, volId, i)
		imgUrl := fmt.Sprintf("%s/nlmivs/download_image.jsp?cno=%s&vol=%d&page=%d&twoThreeYn=N&servPeriCd=&servTypeCd=",
			r.ServerUrl, r.bookId, volId, i)
		r.canvases = append(r.canvases, imgUrl)
	}
	return r.canvases, nil
}

func (r *LodNLGoKr) getBody(sUrl string) ([]byte, error) {
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

func (r *LodNLGoKr) postBody(sUrl string, postData []byte) ([]byte, error) {

	req, err := http.NewRequest("POST", sUrl, bytes.NewBuffer(postData))
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", config.Conf.UserAgent)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Referer", r.ServerUrl+"/main.wviewer")

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

func (r *LodNLGoKr) getBodyByGui(apiUrl string) (buf string, err error) {
	err = sharedmemory.WriteURLToSharedMemory(apiUrl)
	if err != nil {
		fmt.Println("Failed to write to shared memory:", err)
		return
	}
	for i := 0; i < 300; i++ {
		time.Sleep(time.Second * 1)
		r.bufBody, err = sharedmemory.ReadHTMLFromSharedMemory()
		if err == nil && r.bufBody != "" && strings.Contains(r.bufBody, "loadVol") {
			break
		}
	}
	return r.bufBody, nil
}

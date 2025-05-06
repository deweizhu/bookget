package app

import (
	"bookget/config"
	"bookget/pkg/gohttp"
	"bookget/pkg/util"
	"context"
	"errors"
	"fmt"
	"log"
	"net/http/cookiejar"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type LodNLGoKr struct {
	dt       *DownloadTask
	PageBody string
	bookMark string
	apiUrl   string
	fileExt  string
	tmpFile  string
}

func NewLodNLGoKr() *LodNLGoKr {
	return &LodNLGoKr{
		// 初始化字段
		dt: new(DownloadTask),
	}
}

func (r *LodNLGoKr) GetRouterInit(sUrl string) (map[string]interface{}, error) {
	msg, err := r.Run(sUrl)
	return map[string]interface{}{
		"url": sUrl,
		"msg": msg,
	}, err
}

func (r *LodNLGoKr) Run(sUrl string) (msg string, err error) {

	r.dt.UrlParsed, err = url.Parse(sUrl)
	r.dt.Url = sUrl

	r.dt.BookId = r.getBookId(r.dt.Url)
	if r.dt.BookId == "" {
		return "requested URL was not found.", err
	}
	r.dt.Jar, _ = cookiejar.New(nil)
	r.apiUrl = "http://viewer.nl.go.kr:8080"
	//r.apiUrl = "https://viewer.nl.go.kr"
	webUrl := r.apiUrl + "/nlmivs/viewWonmun_js.jsp?card_class=L&cno=" + r.dt.BookId
	//AppData\Roaming\BookGet\bookget\User Data\
	r.tmpFile = config.UserHomeDir() + "\\AppData\\Roaming\\BookGet\\bookget\\User Data\\tmp.html"
	OpenWebBrowser(webUrl, []string{"-o", "tmp.html"})
	return r.download()
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

func (r *LodNLGoKr) download() (msg string, err error) {
	for i := 0; i < 60; i++ {
		time.Sleep(time.Second * 5)
		bs, err := os.ReadFile(r.tmpFile)
		if bs == nil || err != nil {
			continue
		}
		r.PageBody = string(bs)
		break
	}
	name := fmt.Sprintf("%04d", r.dt.Index)
	log.Printf("Get %s  %s\n", name, r.dt.Url)
	//PDF
	if strings.Contains(r.PageBody, "extention = \"PDF\";") {
		r.fileExt = ".pdf"
		m := regexp.MustCompile(`DEFAULT_URL\s=\s["']([^;]+)["'];`).FindStringSubmatch(r.PageBody)
		if m == nil {
			return "requested URL was not found.", err
		}
		pdfUrl := r.apiUrl + m[1]
		r.dt.SavePath = CreateDirectory(r.dt.UrlParsed.Host, r.dt.BookId, "")
		return r.do([]string{pdfUrl})
	}
	//file ext
	r.fileExt = ".png"
	if match := regexp.MustCompile(`ext = "([A-z]+)"`).FindStringSubmatch(r.PageBody); match != nil && match[1] != "TIF" {
		r.fileExt = "." + match[1]
	}

	respVolume, err := r.getVolumeUrls(r.dt.Url)
	if err != nil {
		fmt.Println(err)
		return "getVolumes", err
	}
	for i, vol := range respVolume {
		if !config.VolumeRange(i) {
			continue
		}
		vid := fmt.Sprintf("%04d", i+1)
		r.dt.SavePath = CreateDirectory(r.dt.UrlParsed.Host, r.dt.BookId, vid)
		canvases, err := r.getCanvasesByUrl(i, vol.Url)
		if err != nil || canvases == nil {
			fmt.Println(err)
			continue
		}
		log.Printf(" %d/%d volume, %d pages, title=%s \n", i+1, len(respVolume), len(canvases), vol.Title)
		r.do(canvases)
	}
	_ = os.Remove(r.tmpFile)
	return "", err
}

func (r *LodNLGoKr) do(imgUrls []string) (msg string, err error) {
	if imgUrls == nil {
		return
	}
	fmt.Println()
	size := len(imgUrls)
	ctx := context.Background()
	if r.fileExt != ".pdf" {
		config.Conf.Threads = 1
	}
	for i, uri := range imgUrls {
		if uri == "" || !config.PageRange(i, size) {
			continue
		}
		sortId := fmt.Sprintf("%04d", i+1)
		filename := sortId + r.fileExt
		dest := r.dt.SavePath + filename
		if FileExist(dest) {
			continue
		}
		log.Printf("Get %d/%d, URL: %s\n", i+1, size, uri)
		opts := gohttp.Options{
			DestFile:    dest,
			Overwrite:   false,
			Concurrency: 1,
			CookieFile:  config.Conf.CookieFile,
			CookieJar:   r.dt.Jar,
			Headers: map[string]interface{}{
				"User-Agent": config.Conf.UserAgent,
			},
		}
		gohttp.FastGet(ctx, uri, opts)
		util.PrintSleepTime(config.Conf.Speed)
		fmt.Println()
	}
	fmt.Println()
	return "", err
}

func (r *LodNLGoKr) getVolumeUrls(sUrl string) (volumes []Volume, err error) {
	matches := regexp.MustCompile(`<h2 class="volTitle(?:[^"]*)">([^<]+)</h2>`).FindAllStringSubmatch(r.PageBody, -1)
	if matches == nil {
		return
	}
	for i, m := range matches {
		vol := Volume{
			Title: strings.TrimSpace(m[1]),
			Url:   "",
			Seq:   1,
		}
		vol.Url = fmt.Sprintf("%s/main.wviewer?card_class=L&cno=%s&vol=%d&page=1", r.apiUrl, r.dt.BookId, i+1)
		volumes = append(volumes, vol)
		//r.bookMark += m[1] + "......" + m[2] + "\r\n"
	}
	return volumes, err
}

func (r *LodNLGoKr) getCanvasesByUrl(volId int, sUrl string) (canvases []string, err error) {
	//loadVol('CNTS-00047981911',1,'/wonmun5/data4/imagedb/ncldb7/KOL000021672',155,'26');
	matches := regexp.MustCompile(`loadVol\(([^,]+),([^,]+),([^,]+),([^,]+),([^,]+)\);`).FindAllStringSubmatch(r.PageBody, -1)
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
		imgUrl := fmt.Sprintf("%s/nlmivs/download_image.jsp?cno=%s&vol=%d&page=%d&twoThreeYn=N&servPeriCd=&servTypeCd=", r.apiUrl, r.dt.BookId, volId, i)
		canvases = append(canvases, imgUrl)
	}
	return canvases, err
}

func (r *LodNLGoKr) getBody(apiUrl string, jar *cookiejar.Jar) ([]byte, error) {
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
	if resp.GetStatusCode() != 200 || bs == nil {
		return nil, errors.New(fmt.Sprintf("ErrCode:%d, %s", resp.GetStatusCode(), resp.GetReasonPhrase()))
	}
	return bs, nil
}

func (r *LodNLGoKr) postBody(sUrl string, d []byte) ([]byte, error) {
	ctx := context.Background()
	cli := gohttp.NewClient(ctx, gohttp.Options{
		CookieFile: config.Conf.CookieFile,
		CookieJar:  r.dt.Jar,
		Headers: map[string]interface{}{
			"User-Agent":   config.Conf.UserAgent,
			"Content-Type": "application/x-www-form-urlencoded",
			"Referer":      "http://viewer.nl.go.kr:8080/main.wviewer",
		},
		Body: d,
	})
	resp, err := cli.Post(sUrl)
	if err != nil {
		return nil, err
	}
	bs, _ := resp.GetBody()
	return bs, err
}

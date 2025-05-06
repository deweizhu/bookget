package app

import (
	"bookget/config"
	"bookget/model/sdutcm"
	"bookget/pkg/crypt"
	"bookget/pkg/gohttp"
	"bookget/pkg/util"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http/cookiejar"
	"net/url"
	"regexp"
	"strconv"
)

type Sdutcm struct {
	dt    *DownloadTask
	token string
	body  []byte
}

func NewSdutcm() *Sdutcm {
	return &Sdutcm{
		// 初始化字段
		dt: new(DownloadTask),
	}
}

func (r *Sdutcm) GetRouterInit(sUrl string) (map[string]interface{}, error) {
	msg, err := r.Run(sUrl)
	return map[string]interface{}{
		"url": sUrl,
		"msg": msg,
	}, err
}

func (r *Sdutcm) Run(sUrl string) (msg string, err error) {
	r.dt.UrlParsed, err = url.Parse(sUrl)
	r.dt.Url = sUrl
	r.dt.BookId = r.getBookId(r.dt.Url)
	if r.dt.BookId == "" {
		return "requested URL was not found.", err
	}
	r.dt.Jar, _ = cookiejar.New(nil)
	WaitNewCookie()
	return r.download()
}

func (r *Sdutcm) getBookId(sUrl string) (bookId string) {
	m := regexp.MustCompile(`(?i)id=([A-z0-9_-]+)`).FindStringSubmatch(sUrl)
	if m != nil {
		bookId = m[1]
	}
	return bookId
}

func (r *Sdutcm) download() (msg string, err error) {
	name := fmt.Sprintf("%04d", r.dt.Index)
	log.Printf("Get %s  %s\n", name, r.dt.Url)
	r.body, err = r.getPageContent(r.dt.Url)
	if err != nil {
		return "requested URL was not found.", err
	}
	respVolume, err := r.getVolumes(r.dt.Url, r.dt.Jar)
	if err != nil {
		fmt.Println(err)
		return "getVolumes", err
	}
	config.Conf.FileExt = ".pdf"
	for i, vol := range respVolume {
		if !config.VolumeRange(i) {
			continue
		}
		vid := fmt.Sprintf("%04d", i+1)
		r.dt.SavePath = CreateDirectory(r.dt.UrlParsed.Host, r.dt.BookId, vid)
		canvases, err := r.getCanvases(vol, r.dt.Jar)
		if err != nil || canvases == nil {
			fmt.Println(err)
			continue
		}
		log.Printf(" %d/%d volume, %d pages \n", i+1, len(respVolume), len(canvases))
		r.do(canvases)
	}
	return "", nil
}

func (r *Sdutcm) do(imgUrls []string) (msg string, err error) {
	fmt.Println()
	referer := r.dt.Url
	size := len(imgUrls)
	ctx := context.Background()
	for i, uri := range imgUrls {
		if uri == "" || !config.PageRange(i, size) {
			continue
		}
		sortId := fmt.Sprintf("%04d", i+1)
		filename := sortId + config.Conf.FileExt
		dest := r.dt.SavePath + filename
		if FileExist(dest) {
			continue
		}
		log.Printf("Get %d/%d,  URL: %s\n", i+1, size, uri)

		bs, err := getBody(uri, r.dt.Jar)
		var respBody sdutcm.PagePicTxt
		if err = json.Unmarshal(bs, &respBody); err != nil {
			break
		}
		csPath := crypt.EncodeURI(respBody.Url)
		pdfUrl := "https://" + r.dt.UrlParsed.Host + "/getencryptFtpPdf.jspx?fileName=" + csPath + r.token
		opts := gohttp.Options{
			DestFile:    dest,
			Overwrite:   false,
			Concurrency: 1,
			CookieFile:  config.Conf.CookieFile,
			CookieJar:   r.dt.Jar,
			Headers: map[string]interface{}{
				"User-Agent": config.Conf.UserAgent,
				"Referer":    referer,
			},
		}
		for k := 0; k < 10; k++ {
			resp, err := gohttp.FastGet(ctx, pdfUrl, opts)
			if err == nil && resp.GetStatusCode() == 200 {
				break
			}
			WaitNewCookieWithMsg(uri)
		}
		util.PrintSleepTime(config.Conf.Speed)
		fmt.Println()
	}
	fmt.Println()
	return "", err
}

func (r *Sdutcm) getVolumes(sUrl string, jar *cookiejar.Jar) (volumes []string, err error) {
	ancientVolume := r.getVolumeId(r.body)
	if err != nil {
		return nil, err
	}
	apiUrl := "https://" + r.dt.UrlParsed.Host + "/sdutcm/ancient/book/getVolume.jspx?lshh=" + ancientVolume
	bs, err := getBody(apiUrl, jar)
	var respBody sdutcm.VolumeList
	if err = json.Unmarshal(bs, &respBody); err != nil {
		return nil, err
	}
	for _, m := range respBody.List {
		volUrl := fmt.Sprintf("https://%s/sdutcm/ancient/book/read.jspx?id=%s&pageNum=1", r.dt.UrlParsed.Host, m.ContentId)
		volumes = append(volumes, volUrl)
	}
	return volumes, nil
}

func (r *Sdutcm) getCanvases(sUrl string, jar *cookiejar.Jar) (canvases []string, err error) {
	r.token = r.getToken(r.body)
	size := r.getPageCount(r.body)
	canvases = make([]string, 0, size)
	for i := 1; i <= size; i++ {
		imgUrl := fmt.Sprintf("https://%s/sdutcm/ancient/book/getPagePicTxt.jspx?pageNum=%d&contentId=%s", r.dt.UrlParsed.Host, i, r.dt.BookId)
		canvases = append(canvases, imgUrl)
	}
	return canvases, nil
}

func (r *Sdutcm) getToken(bs []byte) string {
	matches := regexp.MustCompile(`params\s*=\s*["'](\S+)["']`).FindSubmatch(bs)
	if matches != nil {
		return string(matches[1])
	}
	return ""
}

func (r *Sdutcm) getPageCount(bs []byte) int {
	matches := regexp.MustCompile(`pageCount\s+=\s+parseInt\(([0-9]+)\);`).FindSubmatch(bs)
	if matches != nil {
		pageCount, _ := strconv.Atoi(string(matches[1]))
		return pageCount
	}
	return 0
}

func (r *Sdutcm) getVolumeId(bs []byte) string {
	matches := regexp.MustCompile(`ancientVolume\s*=\s*["'](\S+)["'];`).FindSubmatch(bs)
	if matches != nil {
		return string(matches[1])
	}
	return ""
}

func (r *Sdutcm) getPageContent(sUrl string) (bs []byte, err error) {
	r.body, err = getBody(sUrl, r.dt.Jar)
	if err != nil {
		return
	}
	return r.body, err
}

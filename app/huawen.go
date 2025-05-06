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
	"regexp"
	"strings"
)

type Huawen struct {
	dt *DownloadTask
}

func NewHuawen() *Huawen {
	return &Huawen{
		// 初始化字段
		dt: new(DownloadTask),
	}
}

func (r *Huawen) GetRouterInit(sUrl string) (map[string]interface{}, error) {
	msg, err := r.Run(sUrl)
	return map[string]interface{}{
		"url": sUrl,
		"msg": msg,
	}, err
}

func (r *Huawen) Run(sUrl string) (msg string, err error) {
	if !strings.Contains(sUrl, "/reader") && strings.Contains(sUrl, "/zh-tw/book/") {
		sUrl += "/reader"
	}

	r.dt.UrlParsed, err = url.Parse(sUrl)
	r.dt.Url = sUrl

	r.dt.BookId = getBookId(r.dt.Url)
	if r.dt.BookId == "" {
		return "requested URL was not found.", err
	}
	r.dt.Jar, _ = cookiejar.New(nil)
	return r.download()
}

func (r *Huawen) getBookId(sUrl string) (bookId string) {
	//TODO implement me
	panic("implement me")
}

func (r *Huawen) download() (msg string, err error) {
	name := fmt.Sprintf("%04d", r.dt.Index)
	log.Printf("Get %s  %s\n", name, r.dt.Url)

	respVolume, err := r.getVolumes(r.dt.Url, r.dt.Jar)
	if err != nil {
		fmt.Println(err)
		return "getVolumes", err
	}
	for i, vol := range respVolume {
		if !config.VolumeRange(i) {
			continue
		}
		r.dt.SavePath = CreateDirectory(r.dt.UrlParsed.Host, r.dt.BookId, "")
		log.Printf(" %d/%d PDFs \n", i+1, len(respVolume))
		r.do(vol)
	}
	return "", nil
}

func (r *Huawen) do(pdfUrl string) (msg string, err error) {
	filename := util.FileName(pdfUrl)
	dest := r.dt.SavePath + filename
	if FileExist(dest) {
		return "", nil
	}
	u, err := url.Parse(pdfUrl)
	ctx := context.Background()
	opts := gohttp.Options{
		DestFile:    dest,
		Overwrite:   false,
		Concurrency: 1,
		CookieFile:  config.Conf.CookieFile,
		CookieJar:   r.dt.Jar,
		Headers: map[string]interface{}{
			"User-Agent": config.Conf.UserAgent,
			"Referer":    "https://" + r.dt.UrlParsed.Host + "/pdfjs/web/viewer.html?file=" + u.Path,
		},
	}
	_, err = gohttp.FastGet(ctx, pdfUrl, opts)
	if err != nil {
		fmt.Println(err)
	}
	util.PrintSleepTime(config.Conf.Speed)
	fmt.Println()
	return "", nil
}

func (r *Huawen) getVolumes(sUrl string, jar *cookiejar.Jar) (volumes []string, err error) {
	bs, err := r.getBody(sUrl, jar)
	if err != nil {
		return
	}
	matches := regexp.MustCompile(`(?i)viewer.html\?file=([^"]+)"`).FindAllSubmatch(bs, -1)
	if matches == nil {
		return
	}
	for _, match := range matches {
		sPath := strings.TrimSpace(string(match[1]))
		if pos := strings.Index(sPath, "&"); pos > 0 {
			sPath = sPath[:pos]
		}
		pdfUrl := "https://" + r.dt.UrlParsed.Host + sPath
		volumes = append(volumes, pdfUrl)
	}
	return volumes, nil
}

func (r *Huawen) getCanvases(sUrl string, jar *cookiejar.Jar) (canvases []string, err error) {
	//TODO implement me
	panic("implement me")
}

func (r *Huawen) getBody(apiUrl string, jar *cookiejar.Jar) ([]byte, error) {
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

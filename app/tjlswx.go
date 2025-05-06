package app

import (
	"bookget/config"
	"bookget/pkg/gohttp"
	"bookget/pkg/util"
	"context"
	"fmt"
	"log"
	"net/http/cookiejar"
	"net/url"
	"regexp"
	"strings"
)

type Tjlswx struct {
	dt *DownloadTask
}

func NewTjlswx() *Tjlswx {
	return &Tjlswx{
		// 初始化字段
		dt: new(DownloadTask),
	}
}

func (r *Tjlswx) GetRouterInit(sUrl string) (map[string]interface{}, error) {
	msg, err := r.Run(sUrl)
	return map[string]interface{}{
		"url": sUrl,
		"msg": msg,
	}, err
}

func (r Tjlswx) Run(sUrl string) (msg string, err error) {

	r.dt.UrlParsed, err = url.Parse(sUrl)
	r.dt.Url = sUrl

	r.dt.BookId = r.getBookId(r.dt.Url)
	if r.dt.BookId == "" {
		return "requested URL was not found.", err
	}
	r.dt.Jar, _ = cookiejar.New(nil)
	return r.download()
}

func (r Tjlswx) getBookId(sUrl string) (bookId string) {
	m := regexp.MustCompile(`drid=([A-Za-z0-9]+)`).FindStringSubmatch(sUrl)
	if m != nil {
		bookId = m[1]
	}
	return bookId
}

func (r Tjlswx) download() (msg string, err error) {
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

func (r Tjlswx) do(imgUrls []string) (msg string, err error) {
	if imgUrls == nil {
		return
	}
	fmt.Println()
	referer := url.QueryEscape(r.dt.Url)
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
		log.Printf("Get %d/%d page, URL: %s\n", i+1, size, uri)
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
		_, err = gohttp.FastGet(ctx, uri, opts)
		if err != nil {
			fmt.Println(err)
			util.PrintSleepTime(config.Conf.Speed)
		}
		fmt.Println()
	}
	fmt.Println()
	return "", err
}

func (r Tjlswx) getVolumes(sUrl string, jar *cookiejar.Jar) (volumes []string, err error) {
	apiUrl := fmt.Sprintf("%s://%s/Ashx/GetPageImage.ashx?volume=1&readType=photo&%s",
		r.dt.UrlParsed.Scheme, r.dt.UrlParsed.Host, r.dt.UrlParsed.RawQuery)
	bs, err := r.getBody(apiUrl, jar)
	if err != nil {
		return
	}
	//<volumesNum>1,2,3,4,5,6,7,8,9,10,11,12,13</volumesNum>
	match := regexp.MustCompile(`<volumesNum>([0-9,]+)</volumesNum>`).FindSubmatch(bs)
	if match == nil {
		return
	}
	m := strings.Split(string(match[1]), ",")
	for _, v := range m {
		dUrl := fmt.Sprintf("%s://%s/Ashx/GetPageImage.ashx?volume=%s&readType=photo&%s",
			r.dt.UrlParsed.Scheme, r.dt.UrlParsed.Host, v, r.dt.UrlParsed.RawQuery)
		volumes = append(volumes, dUrl)
	}
	return volumes, nil
}

func (r Tjlswx) getCanvases(sUrl string, jar *cookiejar.Jar) (canvases []string, err error) {
	bs, err := r.getBody(sUrl, jar)
	if err != nil {
		return
	}
	matches := regexp.MustCompile(`<image\ssource=["']([^"']+)["']`).FindAllSubmatch(bs, -1)
	if matches == nil {
		return
	}
	for _, v := range matches {
		imgUrl := fmt.Sprintf("%s://%s%s", r.dt.UrlParsed.Scheme, r.dt.UrlParsed.Host, string(v[1]))
		canvases = append(canvases, imgUrl)
	}
	return canvases, nil
}

func (r Tjlswx) getBody(apiUrl string, jar *cookiejar.Jar) ([]byte, error) {
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
	return bs, nil
}

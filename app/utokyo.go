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
)

type Utokyo struct {
	dt *DownloadTask
}

func NewUtokyo() *Utokyo {
	return &Utokyo{
		// 初始化字段
		dt: new(DownloadTask),
	}
}

func (r *Utokyo) GetRouterInit(sUrl string) (map[string]interface{}, error) {
	msg, err := r.Run(sUrl)
	return map[string]interface{}{
		"url": sUrl,
		"msg": msg,
	}, err
}

func (p *Utokyo) Run(sUrl string) (msg string, err error) {
	p.dt.UrlParsed, err = url.Parse(sUrl)
	p.dt.Url = sUrl
	p.dt.BookId = p.getBookId(p.dt.Url)
	if p.dt.BookId == "" {
		return "requested URL was not found.", err
	}
	p.dt.Jar, _ = cookiejar.New(nil)
	return p.download()
}

func (p *Utokyo) getBookId(sUrl string) (bookId string) {
	if m := regexp.MustCompile(`nu=([A-Za-z0-9]+)`).FindStringSubmatch(sUrl); m != nil {
		bookId = m[1]
	}
	return bookId
}

func (p *Utokyo) download() (msg string, err error) {
	name := fmt.Sprintf("%04d", p.dt.Index)
	log.Printf("Get %s  %s\n", name, p.dt.Url)
	respVolume, err := p.getVolumes(p.dt.Url, p.dt.Jar)
	if err != nil {
		fmt.Println(err)
		return "getVolumes", err
	}
	p.dt.SavePath = CreateDirectory(p.dt.UrlParsed.Host, p.dt.BookId, "")
	for i, vol := range respVolume {
		if !config.VolumeRange(i) {
			continue
		}
		log.Printf(" %d/%d volume, %s \n", i+1, len(respVolume), vol)
		fName := util.FileName(vol)
		sortId := fmt.Sprintf("%04d", i+1)
		dest := p.dt.SavePath + sortId + fName
		p.do(dest, vol)
		util.PrintSleepTime(config.Conf.Speed)
	}
	return msg, err
}

func (p *Utokyo) do(dest, pdfUrl string) (msg string, err error) {
	ctx := context.Background()
	opts := gohttp.Options{
		DestFile:    dest,
		Overwrite:   false,
		Concurrency: config.Conf.Threads,
		CookieFile:  config.Conf.CookieFile,
		CookieJar:   p.dt.Jar,
		Headers: map[string]interface{}{
			"User-Agent": config.Conf.UserAgent,
		},
	}
	resp, err := gohttp.FastGet(ctx, pdfUrl, opts)
	if err != nil || resp.GetStatusCode() != 200 {
		fmt.Println(err)
	}
	return "", err
}

func (p *Utokyo) getVolumes(sUrl string, jar *cookiejar.Jar) (volumes []string, err error) {
	bs, err := p.getBody(sUrl, jar)
	if err != nil {
		return
	}
	//取册数
	matches := regexp.MustCompile(`<a href="pdf/([^"]+)"`).FindAllStringSubmatch(string(bs), -1)
	if matches == nil {
		return
	}
	volumes = make([]string, 0, len(matches))
	for _, v := range matches {
		uri := fmt.Sprintf("http://%s/pdf/%s", p.dt.UrlParsed.Host, v[1])
		volumes = append(volumes, uri)
	}
	return volumes, nil
}

func (p *Utokyo) getBody(sUrl string, jar *cookiejar.Jar) ([]byte, error) {
	referer := url.QueryEscape(sUrl)
	ctx := context.Background()
	cli := gohttp.NewClient(ctx, gohttp.Options{
		CookieFile: config.Conf.CookieFile,
		CookieJar:  jar,
		Headers: map[string]interface{}{
			"User-Agent": config.Conf.UserAgent,
			"Referer":    referer,
		},
	})
	resp, err := cli.Get(sUrl)
	if err != nil {
		return nil, err
	}
	bs, _ := resp.GetBody()
	if bs == nil {
		return nil, errors.New(fmt.Sprintf("ErrCode:%d, %s", resp.GetStatusCode(), resp.GetReasonPhrase()))
	}
	return bs, nil
}

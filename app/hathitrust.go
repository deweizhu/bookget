package app

import (
	"bookget/config"
	"bookget/pkg/gohttp"
	"bookget/pkg/util"
	"bytes"
	"context"
	"errors"
	"fmt"
	"log"
	"net/http/cookiejar"
	"net/url"
	"regexp"
	"strconv"
)

type Hathitrust struct {
	dt *DownloadTask
}

func NewHathitrust() *Hathitrust {
	return &Hathitrust{
		// 初始化字段
		dt: new(DownloadTask),
	}
}

func (r *Hathitrust) GetRouterInit(sUrl string) (map[string]interface{}, error) {
	msg, err := r.Run(sUrl)
	return map[string]interface{}{
		"url": sUrl,
		"msg": msg,
	}, err
}

func (r Hathitrust) Run(sUrl string) (msg string, err error) {
	r.dt.UrlParsed, err = url.Parse(sUrl)
	r.dt.Url = sUrl
	r.dt.BookId = r.getBookId(r.dt.Url)
	if r.dt.BookId == "" {
		return "requested URL was not found.", err
	}
	r.dt.Jar, _ = cookiejar.New(nil)
	return r.download()
}

func (r Hathitrust) getBookId(sUrl string) (bookId string) {
	m := regexp.MustCompile(`id=([^&]+)`).FindStringSubmatch(sUrl)
	if m != nil {
		bookId = m[1]
	}
	return bookId
}

func (r Hathitrust) download() (msg string, err error) {
	name := fmt.Sprintf("%04d", r.dt.Index)
	log.Printf("Get %s  %s\n", name, r.dt.Url)
	canvases, err := r.getCanvases(r.dt.Url, r.dt.Jar)
	if err != nil {
		fmt.Println(err.Error())
		return "requested URL was not found.", err
	}
	r.dt.SavePath = CreateDirectory(r.dt.UrlParsed.Host, r.dt.BookId, "")
	msg, err = r.do(canvases)
	return msg, err
}

func (r Hathitrust) do(imgUrls []string) (msg string, err error) {
	if imgUrls == nil {
		return
	}
	fmt.Println()
	referer := url.QueryEscape(r.dt.Url)
	size := len(imgUrls)
	for i, uri := range imgUrls {
		if !config.PageRange(i, size) {
			continue
		}
		if uri == "" {
			continue
		}
		sortId := fmt.Sprintf("%04d", i+1)
		filename := sortId + config.Conf.FileExt
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
				"Referer":    referer,
			},
		}
		ctx := context.Background()
		for {
			_, err := gohttp.FastGet(ctx, uri, opts)
			if err != nil {
				fmt.Println(err)
				//log.Println("images (1 file per page, watermarked,  max. 20 MB / 1 min), image quality:Full")
				util.PrintSleepTime(60)
				continue
			}
			break
		}
	}
	fmt.Println()
	return "", err
}

func (r Hathitrust) getVolumes(sUrl string, jar *cookiejar.Jar) (volumes []string, err error) {
	//TODO implement me
	panic("implement me")
}

func (r Hathitrust) getCanvases(sUrl string, jar *cookiejar.Jar) (canvases []string, err error) {
	bs, err := r.getBody(r.dt.Url, r.dt.Jar)
	if err != nil || bs == nil {
		return nil, err
	}
	//
	if !bytes.Contains(bs, []byte("HT.params.allowSinglePageDownload = true;")) {
		return nil, errors.New("This item is not available online —  Limited - search only")
	}
	// HT.params.totalSeq = 1220;
	matches := regexp.MustCompile(`HT.params.totalSeq = ([0-9]+);`).FindStringSubmatch(string(bs))
	if matches == nil {
		return
	}
	size, _ := strconv.Atoi(matches[1])

	canvases = make([]string, 0, size)
	ext := config.Conf.FileExt
	format := "jpeg"
	if ext == ".png" {
		format = "png"
	} else if ext == ".tif" {
		format = "tiff"
	}
	for i := 0; i < size; i++ {
		imgurl := fmt.Sprintf("https://babel.hathitrust.org/cgi/imgsrv/image?id=%s&attachment=1&size=ppi%%3A300&format=image/%s&seq=%d", r.dt.BookId, format, i+1)
		canvases = append(canvases, imgurl)
	}
	return canvases, err
}

func (r Hathitrust) getBody(apiUrl string, jar *cookiejar.Jar) ([]byte, error) {
	ctx := context.Background()
	cli := gohttp.NewClient(ctx, gohttp.Options{
		CookieFile: config.Conf.CookieFile,
		CookieJar:  jar,
		Headers: map[string]interface{}{
			"User-Agent": config.Conf.UserAgent,
		},
	})
	resp, err := cli.Get(apiUrl)
	if err != nil {
		return nil, err
	}
	bs, _ := resp.GetBody()
	if bs == nil {
		err = errors.New(resp.GetReasonPhrase())
		return nil, err
	}
	return bs, nil
}

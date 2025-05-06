package app

import (
	"bookget/config"
	"bookget/pkg/gohttp"
	"bookget/pkg/util"
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
)

type Nomfoundation struct {
	dt *DownloadTask
}

func NewNomfoundation() *Nomfoundation {
	return &Nomfoundation{
		// 初始化字段
		dt: new(DownloadTask),
	}
}

func (r *Nomfoundation) GetRouterInit(sUrl string) (map[string]interface{}, error) {
	msg, err := r.Run(sUrl)
	return map[string]interface{}{
		"url": sUrl,
		"msg": msg,
	}, err
}

func (r *Nomfoundation) Run(sUrl string) (msg string, err error) {

	r.dt.UrlParsed, err = url.Parse(sUrl)
	r.dt.Url = sUrl

	r.dt.BookId = r.getBookId(r.dt.Url)
	if r.dt.BookId == "" {
		return "requested URL was not found.", err
	}
	r.dt.Jar, _ = cookiejar.New(nil)
	return r.download()
}

func (r *Nomfoundation) getBookId(sUrl string) (bookId string) {
	m := regexp.MustCompile(`/collection/([A-z\d]+)/volume/([A-z\d]+)/`).FindStringSubmatch(sUrl)
	if m == nil {
		m = regexp.MustCompile(`/collection/([A-z\d]+)/volume/([A-z\d]+)/page/(\d+)`).FindStringSubmatch(sUrl)
	}
	if m != nil {
		bookId = m[1] + "-" + m[2]
	}
	return bookId
}

func (r *Nomfoundation) download() (msg string, err error) {
	name := fmt.Sprintf("%04d", r.dt.Index)
	log.Printf("Get %s  %s\n", name, r.dt.Url)
	r.dt.SavePath = CreateDirectory(r.dt.UrlParsed.Host, r.dt.BookId, "")
	canvases, err := r.getCanvases(r.dt.Url, r.dt.Jar)
	if err != nil || canvases == nil {
		return "requested URL was not found.", err
	}
	log.Printf(" %d pages \n", len(canvases))
	return r.do(canvases)
}

func (r *Nomfoundation) do(canvases []string) (msg string, err error) {
	if canvases == nil {
		return
	}
	fmt.Println()
	referer := r.dt.Url
	size := len(canvases)
	var wg sync.WaitGroup
	q := QueueNew(int(config.Conf.Threads))
	for i, uri := range canvases {
		if uri == "" || !config.PageRange(i, size) {
			continue
		}
		sortId := fmt.Sprintf("%04d", i+1)
		filename := sortId + config.Conf.FileExt
		dest := r.dt.SavePath + filename
		if FileExist(dest) {
			continue
		}
		imgUrl := uri
		log.Printf("Get %d/%d, URL: %s\n", i+1, size, imgUrl)
		wg.Add(1)
		q.Go(func() {
			defer wg.Done()
			ctx := context.Background()
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
			gohttp.FastGet(ctx, imgUrl, opts)
			util.PrintSleepTime(config.Conf.Speed)
			fmt.Println()
		})
	}
	wg.Wait()
	fmt.Println()
	return "", err
}

func (r *Nomfoundation) getVolumes(sUrl string, jar *cookiejar.Jar) (volumes []string, err error) {
	//TODO implement me
	panic("implement me")
}

func (r *Nomfoundation) getCanvases(sUrl string, jar *cookiejar.Jar) (canvases []string, err error) {
	if !strings.Contains(sUrl, "/page/") {
		sUrl += "/page/1"
	}
	bs, err := r.getBody(sUrl, r.dt.Jar)
	if err != nil {
		return
	}
	match := regexp.MustCompile(`Page (\d+) of (\d+)`).FindSubmatch(bs)
	if match == nil {
		return nil, errors.New("Error: Page No. ")
	}
	size, err := strconv.Atoi(string(match[2]))
	if err != nil {
		return nil, err
	}
	// <img src="/site_media/nom/nlvnpf-0889-05/jpeg/nlvnpf-0889-05-001.jpg" usemap="#pagemap" ismap="ismap"/>
	m := regexp.MustCompile(`<img\s+src="([^"]+)"\s+usemap="#pagemap"`).FindSubmatch(bs)
	if m == nil {
		return nil, errors.New("Error: image URL. ")
	}
	path := regexp.MustCompile(`-(\d+)\.jpg`).ReplaceAll(m[1], []byte("-%s.jpg"))
	imgUrlTemplate := "https://lib.nomfoundation.org" + strings.Replace(string(path), "/jpeg/", "/large/", 1)
	for i := 1; i <= size; i++ {
		sid := fmt.Sprintf("%03d", i)
		imgUrl := fmt.Sprintf(imgUrlTemplate, sid)
		canvases = append(canvases, imgUrl)
	}
	return canvases, nil
}

func (r *Nomfoundation) getBody(apiUrl string, jar *cookiejar.Jar) ([]byte, error) {
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
	if resp.GetStatusCode() != http.StatusOK {
		return nil, errors.New(fmt.Sprintf("ErrCode:%d, %s", resp.GetStatusCode(), resp.GetReasonPhrase()))
	}
	return bs, nil
}

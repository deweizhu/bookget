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
	"strconv"
	"strings"
	"sync"
)

type Yonezawa struct {
	dt *DownloadTask
}

func NewYonezawa() *Yonezawa {
	return &Yonezawa{
		// 初始化字段
		dt: new(DownloadTask),
	}
}

func (r *Yonezawa) GetRouterInit(sUrl string) (map[string]interface{}, error) {
	msg, err := r.Run(sUrl)
	return map[string]interface{}{
		"url": sUrl,
		"msg": msg,
	}, err
}

func (p *Yonezawa) Run(sUrl string) (msg string, err error) {
	p.dt.UrlParsed, err = url.Parse(sUrl)
	p.dt.Url = sUrl
	p.dt.BookId = p.getBookId(p.dt.Url)
	if p.dt.BookId == "" {
		return "requested URL was not found.", err
	}
	p.dt.Jar, _ = cookiejar.New(nil)
	return p.download()
}

func (p *Yonezawa) getBookId(sUrl string) (bookId string) {
	if m := regexp.MustCompile(`/([A-z\d_-]+)_view.html`).FindStringSubmatch(sUrl); m != nil {
		bookId = m[1]
	}
	return bookId
}

func (p *Yonezawa) download() (msg string, err error) {
	name := fmt.Sprintf("%04d", p.dt.Index)
	log.Printf("Get %s  %s\n", name, p.dt.Url)
	respVolume, err := p.getVolumes(p.dt.Url, p.dt.Jar)
	if err != nil {
		fmt.Println(err)
		return "getVolumes", err
	}
	sizeVol := len(respVolume)
	for i, vol := range respVolume {
		if !config.VolumeRange(i) {
			continue
		}
		if sizeVol == 1 {
			p.dt.SavePath = CreateDirectory(p.dt.UrlParsed.Host, p.dt.BookId, "")
		} else {
			vid := fmt.Sprintf("%04d", i+1)
			p.dt.SavePath = CreateDirectory(p.dt.UrlParsed.Host, p.dt.BookId, vid)
		}

		canvases, err := p.getCanvases(vol, p.dt.Jar)
		if err != nil || canvases == nil {
			fmt.Println(err)
			continue
		}
		log.Printf(" %d/%d volume, %d pages \n", i+1, sizeVol, len(canvases))
		p.do(canvases)
	}
	return msg, err
}

func (p *Yonezawa) do(imgUrls []string) (msg string, err error) {
	if imgUrls == nil {
		return
	}
	size := len(imgUrls)
	fmt.Println()
	var wg sync.WaitGroup
	q := QueueNew(int(config.Conf.Threads))
	for i, uri := range imgUrls {
		if uri == "" || !config.PageRange(i, size) {
			continue
		}
		ext := util.FileExt(uri)
		sortId := fmt.Sprintf("%04d", i+1)
		filename := sortId + ext
		dest := p.dt.SavePath + filename
		if FileExist(dest) {
			continue
		}
		imgUrl := uri
		fmt.Println()
		log.Printf("Get %d/%d  %s\n", i+1, size, imgUrl)
		wg.Add(1)
		q.Go(func() {
			defer wg.Done()
			ctx := context.Background()
			opts := gohttp.Options{
				DestFile:    dest,
				Overwrite:   false,
				Concurrency: 1,
				CookieFile:  config.Conf.CookieFile,
				CookieJar:   p.dt.Jar,
				Headers: map[string]interface{}{
					"User-Agent": config.Conf.UserAgent,
				},
			}
			gohttp.FastGet(ctx, imgUrl, opts)
			fmt.Println()
		})
	}
	wg.Wait()
	fmt.Println()
	return "", err
}

func (p *Yonezawa) getVolumes(sUrl string, jar *cookiejar.Jar) (volumes []string, err error) {
	volumes = append(volumes, sUrl)
	return volumes, nil
}

func (p *Yonezawa) getCanvases(sUrl string, jar *cookiejar.Jar) (canvases []string, err error) {
	bs, err := p.getBody(sUrl, jar)
	if err != nil {
		return
	}
	matches := regexp.MustCompile(`<option\s+value=["']?([A-z\d,_-]+)["']?`).FindAllSubmatch(bs, -1)
	if matches == nil {
		return
	}
	//var dir='data/AA003/';
	imageDir := regexp.MustCompile(`var\s+dir\s?=\s?["'](\S+)["']`).FindSubmatch(bs)
	if imageDir == nil {
		return
	}
	pos := strings.LastIndex(sUrl, "/")
	if pos == -1 {
		return
	}
	host := sUrl[:pos+1]

	for _, val := range matches {
		imgUrls := p.getImageUrls(host, string(imageDir[1]), string(val[1]))
		canvases = append(canvases, imgUrls...)
	}
	return canvases, err
}

func (p *Yonezawa) getBody(sUrl string, jar *cookiejar.Jar) ([]byte, error) {
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

func (p *Yonezawa) postBody(sUrl string, d []byte) ([]byte, error) {
	//TODO implement me
	panic("implement me")
}

func (p *Yonezawa) getImageUrls(host, imageDir, val string) (imgUrls []string) {
	m := strings.Split(val, ",")
	if m == nil {
		return
	}
	id := m[0]
	maxSize, _ := strconv.Atoi(m[1])
	imgUrls = make([]string, 0, maxSize)
	for i := 1; i <= maxSize; i++ {
		imgUrl := host + p.makeUri(imageDir, id, i)
		imgUrls = append(imgUrls, imgUrl)
	}
	return
}

func (p *Yonezawa) makeUri(imageDir, val string, i int) string {
	dir2 := val[5:8]
	book := val[0:8]
	page := val[len(val)-3:]
	page = regexp.MustCompile(`^0+0?`).ReplaceAllString(page, "")
	sortId := fmt.Sprintf("%03d", i)
	s := fmt.Sprintf("%s%s/%s_%s.jpg", imageDir, dir2, book, sortId)
	return s
}

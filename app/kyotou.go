package app

import (
	"bookget/config"
	"bookget/pkg/gohttp"
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

type Kyotou struct {
	dt *DownloadTask
}

func NewKyotou() *Kyotou {
	return &Kyotou{
		// 初始化字段
		dt: new(DownloadTask),
	}
}

func (r *Kyotou) GetRouterInit(sUrl string) (map[string]interface{}, error) {
	msg, err := r.Run(sUrl)
	return map[string]interface{}{
		"url": sUrl,
		"msg": msg,
	}, err
}

func (r *Kyotou) Run(sUrl string) (msg string, err error) {
	r.dt.UrlParsed, err = url.Parse(sUrl)
	r.dt.Url = sUrl
	r.dt.Jar, _ = cookiejar.New(nil)
	r.dt.BookId = r.getBookId(r.dt.Url)
	if r.dt.BookId == "" {
		return "requested URL was not found.", err
	}
	return r.download()
}

func (r *Kyotou) getBookId(sUrl string) (bookId string) {
	if strings.Contains(sUrl, "menu") {
		return getBookId(sUrl)
	}
	return ""
}

func (r *Kyotou) download() (msg string, err error) {
	name := fmt.Sprintf("%04d", r.dt.Index)
	log.Printf("Get %s  %s\n", name, r.dt.Url)

	respVolume, err := r.getVolumes(r.dt.Url, r.dt.Jar)
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
			r.dt.SavePath = CreateDirectory(r.dt.UrlParsed.Host, r.dt.BookId, "")
		} else {
			vid := fmt.Sprintf("%04d", i+1)
			r.dt.SavePath = CreateDirectory(r.dt.UrlParsed.Host, r.dt.BookId, vid)
		}
		canvases, err := r.getCanvases(vol, r.dt.Jar)
		if err != nil || canvases == nil {
			fmt.Println(err)
			continue
		}
		log.Printf(" %d/%d volume, %d pages \n", i+1, sizeVol, len(canvases))
		r.do(canvases)
	}
	return "", nil
}

func (r *Kyotou) do(imgUrls []string) (msg string, err error) {
	if imgUrls == nil {
		return "", nil
	}
	size := len(imgUrls)
	fmt.Println()
	var wg sync.WaitGroup
	q := QueueNew(int(config.Conf.Threads))
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
				CookieJar:   r.dt.Jar,
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

func (r *Kyotou) getVolumes(sUrl string, jar *cookiejar.Jar) (volumes []string, err error) {
	bs, err := r.getBody(sUrl, jar)
	if err != nil {
		return
	}
	//取册数
	matches := regexp.MustCompile(`href=["']?(.+?)\.html["']?`).FindAllSubmatch(bs, -1)
	if matches == nil {
		return
	}
	pos := strings.LastIndex(sUrl, "/")
	hostUrl := sUrl[:pos]
	volumes = make([]string, 0, len(matches))
	for _, v := range matches {
		text := string(v[1])
		if strings.Contains(text, "top") {
			continue
		}
		linkUrl := fmt.Sprintf("%s/%s.html", hostUrl, text)
		volumes = append(volumes, linkUrl)
	}
	return volumes, err
}

func (r *Kyotou) getCanvases(sUrl string, jar *cookiejar.Jar) (canvases []string, err error) {
	bs, err := r.getBody(sUrl, jar)
	if err != nil {
		return
	}
	startPos, ok := r.getVolStartPos(bs)
	if !ok {
		return
	}
	maxPage, ok := r.getVolMaxPage(bs)
	if !ok {
		return
	}
	bookNumber, ok := r.getBookNumber(bs)
	if !ok {
		return
	}
	pos := strings.LastIndex(sUrl, "/")
	pos1 := strings.LastIndex(sUrl[:pos], "/")
	hostUrl := sUrl[:pos1]
	maxPos := startPos + maxPage
	for i := 1; i < maxPos; i++ {
		sortId := fmt.Sprintf("%04d", i)
		imgUrl := fmt.Sprintf("%s/L/%s%s.jpg", hostUrl, bookNumber, sortId)
		canvases = append(canvases, imgUrl)
	}
	return canvases, err
}

func (r *Kyotou) getBody(sUrl string, jar *cookiejar.Jar) ([]byte, error) {
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
	if resp.GetStatusCode() != 200 || bs == nil {
		return nil, errors.New(fmt.Sprintf("ErrCode:%d, %s", resp.GetStatusCode(), resp.GetReasonPhrase()))
	}
	return bs, nil
}

func (r *Kyotou) postBody(sUrl string, d []byte) ([]byte, error) {
	//TODO implement me
	panic("implement me")
}

func (r *Kyotou) getBookNumber(bs []byte) (bookNumber string, ok bool) {
	//当前开始位置
	match := regexp.MustCompile(`var[\s]+bookNum[\s]+=["'\s]*([A-z0-9]+)["'\s]*;`).FindStringSubmatch(string(bs))
	if match == nil {
		return "", false
	}
	return match[1], true
}

func (r *Kyotou) getVolStartPos(bs []byte) (startPos int, ok bool) {
	//当前开始位置
	match := regexp.MustCompile(`var[\s]+volStartPos[\s]*=[\s]*([0-9]+)[\s]*;`).FindStringSubmatch(string(bs))
	if match == nil {
		return 0, false
	}
	startPos, _ = strconv.Atoi(match[1])
	return startPos, true
}

func (r *Kyotou) getVolCurPage(bs []byte) (curPage int, ok bool) {
	//当前开始位置
	match := regexp.MustCompile(`var[\s]+curPage[\s]*=[\s]*([0-9]+)[\s]*;`).FindStringSubmatch(string(bs))
	if match == nil {
		return 0, false
	}
	curPage, _ = strconv.Atoi(match[1])
	return curPage, true
}

func (r *Kyotou) getVolMaxPage(bs []byte) (maxPage int, ok bool) {
	//当前开始位置
	match := regexp.MustCompile(`var[\s]+volMaxPage[\s]*=[\s]*([0-9]+)[\s]*;`).FindStringSubmatch(string(bs))
	if match == nil {
		return 0, false
	}
	maxPage, _ = strconv.Atoi(match[1])
	return maxPage, true
}

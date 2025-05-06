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
	"path"
	"regexp"
	"sort"
	"sync"
)

type Waseda struct {
	dt *DownloadTask
}

func NewWaseda() *Waseda {
	return &Waseda{
		// 初始化字段
		dt: new(DownloadTask),
	}
}

func (r *Waseda) GetRouterInit(sUrl string) (map[string]interface{}, error) {
	msg, err := r.Run(sUrl)
	return map[string]interface{}{
		"url": sUrl,
		"msg": msg,
	}, err
}
func (r Waseda) Run(sUrl string) (msg string, err error) {

	r.dt.UrlParsed, err = url.Parse(sUrl)
	r.dt.Url = sUrl

	r.dt.BookId = getBookId(r.dt.Url)
	r.dt.Jar, _ = cookiejar.New(nil)
	return r.download()
}

func (r Waseda) download() (msg string, err error) {
	respVolume, err := r.getVolumes(r.dt.Url, r.dt.Jar)
	if err != nil {
		fmt.Println(err)
		return "getVolumes", err
	}
	if config.Conf.FileExt == ".pdf" {
		for i, vol := range respVolume {
			if !config.VolumeRange(i) {
				continue
			}
			sortId := fmt.Sprintf("%04d", i+1)
			r.dt.SavePath = CreateDirectory(r.dt.UrlParsed.Host, r.dt.BookId, "")
			log.Printf(" %d/%d volume, URL:%s \n", i+1, len(respVolume), vol)
			filename := sortId + config.Conf.FileExt
			dest := r.dt.SavePath + filename
			r.doDownload(vol, dest)
		}
	} else {
		for i, vol := range respVolume {
			if !config.VolumeRange(i) {
				continue
			}
			if len(respVolume) == 1 {
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

			log.Printf(" %d/%d volume, %d pages \n", i+1, len(respVolume), len(canvases))
			r.do(canvases)
		}
	}

	return "", nil
}

func (r Waseda) do(imgUrls []string) (msg string, err error) {
	if imgUrls == nil {
		return
	}
	fmt.Println()
	size := len(imgUrls)
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
		log.Printf("Get %d/%d page, URL: %s\n", i+1, size, uri)
		imgUrl := uri
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

func (r Waseda) getVolumes(sUrl string, jar *cookiejar.Jar) (volumes []string, err error) {
	bs, err := r.getBody(sUrl, jar)
	if err != nil {
		return
	}
	text := string(bs)
	//取册数
	matches := regexp.MustCompile(`href=["'](.+?)\.html["']`).FindAllStringSubmatch(text, -1)
	if matches == nil {
		return
	}
	ids := make([]string, 0, len(matches))
	for _, match := range matches {
		ids = append(ids, match[1])
	}
	sort.Sort(util.SortByStr(ids))
	volumes = make([]string, 0, len(ids))
	for _, v := range ids {
		var htmlUrl string
		if config.Conf.FileExt == ".pdf" {
			htmlUrl = sUrl + v + ".pdf"
		} else {
			htmlUrl = sUrl + v + ".html"
		}
		volumes = append(volumes, htmlUrl)
	}
	return volumes, nil
}

func (r Waseda) getCanvases(sUrl string, jar *cookiejar.Jar) (canvases []string, err error) {
	bs, err := r.getBody(sUrl, jar)
	if err != nil {
		return
	}
	text := string(bs)
	//取册数
	matches := regexp.MustCompile(`(?i)href=["'](.+?)\.jpg["']\s+target="_blank">\d+</A>`).FindAllStringSubmatch(text, -1)
	if matches == nil {
		return
	}
	ids := make([]string, 0, len(matches))
	for _, match := range matches {
		ids = append(ids, match[1])
	}
	sort.Sort(util.SortByStr(ids))
	canvases = make([]string, 0, len(ids))
	dir, _ := path.Split(sUrl)
	for _, v := range ids {
		imgUrl := dir + v + ".jpg"
		canvases = append(canvases, imgUrl)
	}
	return canvases, nil
}

func (r Waseda) getBody(apiUrl string, jar *cookiejar.Jar) ([]byte, error) {
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
	if bs == nil {
		return nil, errors.New(fmt.Sprintf("ErrCode:%d, %s", resp.GetStatusCode(), resp.GetReasonPhrase()))
	}
	return bs, nil
}

func (r Waseda) doDownload(dUrl, dest string) bool {
	if FileExist(dest) {
		return false
	}
	referer := url.QueryEscape(r.dt.Url)
	opts := gohttp.Options{
		DestFile:    dest,
		Overwrite:   false,
		Concurrency: config.Conf.Threads,
		CookieFile:  config.Conf.CookieFile,
		CookieJar:   r.dt.Jar,
		Headers: map[string]interface{}{
			"User-Agent": config.Conf.UserAgent,
			"Referer":    referer,
		},
	}
	ctx := context.Background()
	_, err := gohttp.FastGet(ctx, dUrl, opts)
	if err == nil {
		fmt.Println()
		return true
	}
	fmt.Println(err)
	fmt.Println()
	return false
}

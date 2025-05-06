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
	"sync"
)

type HannomNlv struct {
	dt   *DownloadTask
	body []byte
}

func NewHannomNlv() *HannomNlv {
	return &HannomNlv{
		// 初始化字段
		dt: new(DownloadTask),
	}
}

func (r *HannomNlv) GetRouterInit(sUrl string) (map[string]interface{}, error) {
	msg, err := r.Run(sUrl)
	return map[string]interface{}{
		"url": sUrl,
		"msg": msg,
	}, err
}

func (r *HannomNlv) Run(sUrl string) (msg string, err error) {
	r.dt.UrlParsed, err = url.Parse(sUrl)
	r.dt.Url = sUrl
	r.dt.Jar, _ = cookiejar.New(nil)
	r.dt.BookId = r.getBookId(r.dt.Url)
	if r.dt.BookId == "" {
		return "requested URL was not found.", err
	}
	return r.download()
}

func (r *HannomNlv) getBookId(sUrl string) (bookId string) {
	var err error
	r.body, err = getBody(sUrl, r.dt.Jar)
	if err != nil {
		return ""
	}
	m := regexp.MustCompile(`var[\s+]documentOID[\s+]=[\s+]['"]([^“]+?)['"];`).FindSubmatch(r.body)
	if m != nil {
		return string(m[1])
	}
	return ""
}

func (r *HannomNlv) download() (msg string, err error) {
	name := fmt.Sprintf("%04d", r.dt.Index)
	log.Printf("Get %s  %s\n", name, r.dt.Url)
	r.dt.SavePath = CreateDirectory(r.dt.UrlParsed.Host, r.dt.BookId, "")
	canvases, err := r.getCanvases(r.dt.Url, r.dt.Jar)
	if err != nil || canvases == nil {
		fmt.Println(err)
	}
	log.Printf(" %d pages \n", len(canvases))
	r.do(canvases)
	return "", nil
}

func (r *HannomNlv) do(imgUrls []string) (msg string, err error) {
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
	return "", nil
}

func (r *HannomNlv) getVolumes(sUrl string, jar *cookiejar.Jar) (volumes []string, err error) {
	//TODO implement me
	panic("implement me")
}

func (r *HannomNlv) getCanvases(sUrl string, jar *cookiejar.Jar) (canvases []string, err error) {
	matches := regexp.MustCompile(`'([^']+)':\{'w':([0-9]+),'h':([0-9]+)\}`).FindAllSubmatch(r.body, -1)
	if matches == nil {
		return nil, errors.New("No image")
	}
	apiUrl := r.dt.UrlParsed.Scheme + "://" + r.dt.UrlParsed.Host
	match := regexp.MustCompile(`imageserverPageTileImageRequest[\s+]=[\s+]['"]([^;]+)['"];`).FindSubmatch(r.body)
	if match != nil {
		apiUrl += string(match[1])
	} else {
		apiUrl += "/hannom/cgi-bin/imageserver/imageserver.pl?color=all&ext=jpg"
	}
	for _, m := range matches {
		imgUrl := apiUrl + fmt.Sprintf("&oid=%s.%s&key=&width=%s&crop=0,0,%s,%s", r.dt.BookId, m[1], m[2], m[2], m[3])
		canvases = append(canvases, imgUrl)
	}
	return canvases, err
}

func (r *HannomNlv) getBody(sUrl string, jar *cookiejar.Jar) ([]byte, error) {
	//TODO implement me
	panic("implement me")
}

func (r *HannomNlv) postBody(sUrl string, d []byte) ([]byte, error) {
	//TODO implement me
	panic("implement me")
}

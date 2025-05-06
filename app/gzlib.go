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
)

type Gzlib struct {
	dt *DownloadTask
}

func NewGzlib() *Gzlib {
	return &Gzlib{
		// 初始化字段
		dt: new(DownloadTask),
	}
}

func (r *Gzlib) GetRouterInit(sUrl string) (map[string]interface{}, error) {
	msg, err := r.Run(sUrl)
	return map[string]interface{}{
		"url": sUrl,
		"msg": msg,
	}, err
}

func (r Gzlib) Run(sUrl string) (msg string, err error) {

	r.dt.UrlParsed, err = url.Parse(sUrl)
	r.dt.Url = sUrl

	r.dt.BookId = r.getBookId(r.dt.Url)
	if r.dt.BookId == "" {
		return "requested URL was not found.", err
	}
	r.dt.Jar, _ = cookiejar.New(nil)
	return r.download()
}

func (r Gzlib) getBookId(sUrl string) (bookId string) {
	m := regexp.MustCompile(`(?i)bookid=([A-z0-9_-]+)`).FindStringSubmatch(sUrl)
	if m != nil {
		bookId = m[1]
	}
	m = regexp.MustCompile(`(?i)filename=([A-z0-9_-]+)`).FindStringSubmatch(sUrl)
	if m != nil {
		bookId = m[1]
	}
	return bookId
}

func (r Gzlib) download() (msg string, err error) {
	name := fmt.Sprintf("%04d", r.dt.Index)
	log.Printf("Get %s  %s\n", name, r.dt.Url)
	r.dt.SavePath = CreateDirectory(r.dt.UrlParsed.Host, r.dt.BookId, "")
	canvases, err := r.getCanvases(r.dt.Url, r.dt.Jar)
	if err != nil || canvases == nil {
		fmt.Println(err)
	}
	return r.do(canvases)
}

func (r Gzlib) do(dUrls []string) (msg string, err error) {
	if dUrls == nil {
		return
	}
	fmt.Println()
	size := len(dUrls)
	ctx := context.Background()
	requestCookie := r.dt.Jar.Cookies(r.dt.UrlParsed)
	for i, uri := range dUrls {
		if !config.PageRange(i, size) {
			continue
		}
		if uri == "" {
			continue
		}
		ext := util.FileExt(uri)
		dest := r.dt.SavePath + r.dt.BookId + ext
		opts := gohttp.Options{
			DestFile:    dest,
			Overwrite:   false,
			Concurrency: 1,
			CookieFile:  config.Conf.CookieFile,
			CookieJar:   r.dt.Jar,
			Headers: map[string]interface{}{
				"User-Agent":     "ReaderEx 2.3",
				"Accept-Range":   "bytes=0-",
				"Range":          "bytes=0-",
				"Request-Cookie": requestCookie,
			},
		}
		_, err = gohttp.FastGet(ctx, uri, opts)
		if err != nil {
			fmt.Println(err)
			continue
		}
	}

	return "", err
}

func (r Gzlib) getVolumes(sUrl string, jar *cookiejar.Jar) (volumes []string, err error) {
	//TODO implement me
	panic("implement me")
}

func (r Gzlib) getCanvases(sUrl string, jar *cookiejar.Jar) (canvases []string, err error) {
	apiUrl := fmt.Sprintf("%s://%s/Hrcanton/Search/ResultDetail?BookId=%s", r.dt.UrlParsed.Scheme,
		r.dt.UrlParsed.Host, r.dt.BookId)
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
	text := string(bs)
	pdfUrl := ""
	//var fileUrl = "http://113.108.173.156" + subStr;
	m := regexp.MustCompile(`fileUrl[\s]+=[\s]+["'](\S+)["']`).FindStringSubmatch(text)
	if m != nil {
		pdfUrl = m[1]
	}
	//var subStr = "/OnlineViewServer/onlineview.aspx?filename=GZDD034601001.pdf"
	m = regexp.MustCompile(`subStr[\s]+=[\s]+["'](\S+)["']`).FindStringSubmatch(text)
	if m != nil {
		pdfUrl += m[1]
	}
	if pdfUrl == "" {
		pdfUrl = fmt.Sprintf("http://113.108.173.156/OnlineViewServer/onlineview.aspx?filename=%s.pdf", r.dt.BookId)
	}
	canvases = append(canvases, pdfUrl)
	return canvases, nil
}

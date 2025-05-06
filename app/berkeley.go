package app

import (
	"bookget/config"
	"bookget/pkg/gohttp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http/cookiejar"
	"net/url"
	"path/filepath"
	"regexp"
	"time"
)

type Berkeley struct {
	dt *DownloadTask
}

func NewBerkeley() *Berkeley {
	return &Berkeley{
		// 初始化字段
		dt: new(DownloadTask),
	}
}

func (r *Berkeley) GetRouterInit(sUrl string) (map[string]interface{}, error) {
	msg, err := r.Run(sUrl)
	return map[string]interface{}{
		"url": sUrl,
		"msg": msg,
	}, err
}

type BerkeleyResponse struct {
	Name        string `json:"name"`
	Url         string `json:"url"`
	Size        int    `json:"size"`
	Description string `json:"description"`
}

func (r *Berkeley) Run(sUrl string) (msg string, err error) {
	r.dt.UrlParsed, err = url.Parse(sUrl)
	r.dt.Url = sUrl
	r.dt.BookId = r.getBookId(r.dt.Url)
	if r.dt.BookId == "" {
		return "requested URL was not found.", err
	}
	r.dt.Jar, _ = cookiejar.New(nil)
	return r.download()
}

func (r *Berkeley) getBookId(sUrl string) (bookId string) {
	m := regexp.MustCompile(`(?i)record/([A-z0-9_-]+)`).FindStringSubmatch(sUrl)
	if m != nil {
		bookId = m[1]
	}
	return bookId
}

func (r *Berkeley) download() (msg string, err error) {
	name := fmt.Sprintf("%04d", r.dt.Index)
	log.Printf("Get %s  %s\n", name, r.dt.Url)

	r.dt.SavePath = CreateDirectory(r.dt.UrlParsed.Host, r.dt.BookId, "")
	canvases, err := r.getCanvases(r.dt.Url, r.dt.Jar)
	if err != nil || canvases == nil {
		return "requested URL was not found.", err
	}
	log.Printf(" %d files \n", len(canvases))
	r.do(canvases)
	return "", nil
}

func (r *Berkeley) do(canvases []string) (msg string, err error) {
	if canvases == nil {
		return
	}
	fmt.Println()
	referer := r.dt.Url
	size := len(canvases)
	for i, dUrl := range canvases {
		if dUrl == "" || !config.PageRange(i, size) {
			continue
		}
		sortId := fmt.Sprintf("%04d", i+1)
		ext := filepath.Ext(dUrl)
		filename := sortId + ext
		dest := r.dt.SavePath + filename
		if FileExist(dest) {
			continue
		}
		log.Printf("Get %d/%d,  URL: %s\n", i+1, size, dUrl)
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
		gohttp.FastGet(ctx, dUrl, opts)
		fmt.Println()
	}
	return "", err
}

func (r *Berkeley) getVolumes(sUrl string, jar *cookiejar.Jar) (volumes []string, err error) {
	//TODO implement me
	panic("implement me")
}

func (r *Berkeley) getCanvases(sUrl string, jar *cookiejar.Jar) (canvases []string, err error) {

	apiUrl := "https://" + r.dt.UrlParsed.Host + "/api/v1/file?recid=" + r.dt.BookId +
		"&file_types=%5B%5D&hidden_types=%5B%22pdf%3Bpdfa%22%2C%22hocr%22%5D&ln=en&hr=1&_=" + string(time.Now().Unix())
	bs, err := r.getBody(apiUrl, jar)
	if err != nil {
		return
	}

	var resT = make([]BerkeleyResponse, 0, 64)
	if err = json.Unmarshal(bs, &resT); err != nil {
		log.Printf("json.Unmarshal failed: %s\n", err)
		return
	}
	for _, ret := range resT {
		canvases = append(canvases, ret.Url)
	}
	return
}

func (r *Berkeley) getBody(apiUrl string, jar *cookiejar.Jar) ([]byte, error) {
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

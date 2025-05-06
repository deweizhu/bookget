package app

import (
	"bookget/config"
	"bookget/model/yndfz"
	"bookget/pkg/gohttp"
	"bookget/pkg/util"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http/cookiejar"
	"net/url"
	"regexp"
)

type Yndfz struct {
	dt *DownloadTask
}

func NewYndfz() *Yndfz {
	return &Yndfz{
		// 初始化字段
		dt: new(DownloadTask),
	}
}

func (r *Yndfz) GetRouterInit(sUrl string) (map[string]interface{}, error) {
	msg, err := r.Run(sUrl)
	return map[string]interface{}{
		"url": sUrl,
		"msg": msg,
	}, err
}

func (r *Yndfz) Run(sUrl string) (msg string, err error) {

	r.dt.UrlParsed, err = url.Parse(sUrl)
	r.dt.Url = sUrl

	r.dt.BookId = r.getBookId(r.dt.Url)
	if r.dt.BookId == "" {
		return "requested URL was not found.", err
	}
	r.dt.Jar, _ = cookiejar.New(nil)
	return r.download()
}

func (r *Yndfz) getBookId(sUrl string) (bookId string) {
	m := regexp.MustCompile(`id=([A-Za-z0-9]+)`).FindStringSubmatch(sUrl)
	if m != nil {
		bookId = m[1]
	}
	return bookId
}

func (r *Yndfz) download() (msg string, err error) {
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

func (r *Yndfz) do(imgUrls []string) (msg string, err error) {
	if imgUrls == nil {
		return
	}
	fmt.Println()
	referer := url.QueryEscape(r.dt.Url)
	size := len(imgUrls)
	ctx := context.Background()
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

		imgUrl, err := r.getDownloadUrl(uri)
		if err != nil {
			fmt.Println(err)
			break
		}
		_, err = gohttp.FastGet(ctx, imgUrl, opts)
		if err != nil {
			fmt.Println(err)
			util.PrintSleepTime(config.Conf.Speed)
		}
		fmt.Println()
	}
	fmt.Println()
	return "", err
}

func (r *Yndfz) getVolumes(sUrl string, jar *cookiejar.Jar) (volumes []string, err error) {
	volumes = append(volumes, sUrl)
	return volumes, nil
}

func (r *Yndfz) getCanvases(sUrl string, jar *cookiejar.Jar) (canvases []string, err error) {
	type ResponseBody struct {
		BookId           string        `json:"bookId"`
		BookName         string        `json:"bookName"`
		TotalPage        interface{}   `json:"totalPage"`
		ChildCatalogList []interface{} `json:"childCatalogList"`
		PageInfoList     []struct {
			PageId  string  `json:"pageId"`
			PageNum int     `json:"pageNum"`
			Height  float64 `json:"height"`
			Width   float64 `json:"width"`
			PdfUrl  string  `json:"pdfUrl"`
			ImgUrl  string  `json:"imgUrl"`
			Content string  `json:"content"`
		} `json:"pageInfoList"`
	}
	apiUrl := fmt.Sprintf("%s://%s/api/record/pageAndCatalogInfo/getBookInfoByBookId?bookId=%s",
		r.dt.UrlParsed.Scheme, r.dt.UrlParsed.Host, r.dt.BookId)
	bs, err := r.getBody(apiUrl, jar)
	if err != nil {
		return
	}
	var result ResponseBody
	if err = json.Unmarshal(bs, &result); err != nil {
		return nil, err
	}
	for _, v := range result.PageInfoList {
		canvases = append(canvases, v.ImgUrl)
	}
	return canvases, nil
}

func (r *Yndfz) getBody(apiUrl string, jar *cookiejar.Jar) ([]byte, error) {
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

func (r *Yndfz) getDownloadUrl(sUrl string) (string, error) {
	apiUrl := "http://" + r.dt.UrlParsed.Host + "/api/readRight/path/old040001?key=" + url.QueryEscape(sUrl)
	bs, err := r.getBody(apiUrl, r.dt.Jar)
	if err != nil {
		return "", err
	}
	var result yndfz.Response
	if err = json.Unmarshal(bs, &result); err != nil {
		return "", err
	}
	return result.Url, nil
}

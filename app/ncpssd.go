package app

import (
	"bookget/config"
	"bookget/pkg/gohttp"
	"bookget/pkg/util"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http/cookiejar"
	"net/url"
	"regexp"
	"strings"
)

type Ncpssd struct {
	dt *DownloadTask
}

func NewNcpssd() *Ncpssd {
	return &Ncpssd{
		// 初始化字段
		dt: new(DownloadTask),
	}
}

func (r *Ncpssd) GetRouterInit(sUrl string) (map[string]interface{}, error) {
	msg, err := r.Run(sUrl)
	return map[string]interface{}{
		"url": sUrl,
		"msg": msg,
	}, err
}

func (r *Ncpssd) Run(sUrl string) (msg string, err error) {
	r.dt.UrlParsed, err = url.Parse(sUrl)
	r.dt.Url = sUrl
	r.dt.Jar, _ = cookiejar.New(nil)
	WaitNewCookie()
	return r.download()
}

func (r *Ncpssd) getBookId(sUrl string) (bookId string) {
	m := regexp.MustCompile(`(?i)barcodenum=([A-z0-9_-]+)`).FindStringSubmatch(sUrl)
	if m != nil {
		return m[1]
	}
	m = regexp.MustCompile(`(?i)pdf/([A-z0-9_-]+)\.pdf`).FindStringSubmatch(sUrl)
	if m != nil {
		return m[1]
	}
	return bookId
}

func (r *Ncpssd) download() (msg string, err error) {
	respVolume, err := r.getVolumes(r.dt.Url, r.dt.Jar)
	if r.dt.BookId == "" || err != nil {
		fmt.Println(err)
		return "requested URL was not found.", err
	}
	name := fmt.Sprintf("%04d", r.dt.Index)
	log.Printf("Get %s  %s\n", name, r.dt.Url)
	bookId := r.dt.UrlParsed.Query().Get("type")
	if bookId == "" {
		bookId = "ncpssd"
	}
	r.dt.SavePath = CreateDirectory(r.dt.UrlParsed.Host, bookId, "")
	for i, vol := range respVolume {
		if !config.VolumeRange(i) {
			continue
		}
		log.Printf(" %d/%d volume, %s \n", i+1, len(respVolume), vol)
		r.do(vol)
		util.PrintSleepTime(config.Conf.Speed)
		fmt.Println()
	}
	return msg, err
}

func (r *Ncpssd) do(pdfUrl string) (msg string, err error) {
	token, _ := r.getToken()
	ext := util.FileExt(pdfUrl)
	dest := r.dt.SavePath + r.dt.BookId + ext
	jar, _ := cookiejar.New(nil)
	ctx := context.Background()
	referer := "https://" + r.dt.UrlParsed.Host
	gohttp.FastGet(ctx, pdfUrl, gohttp.Options{
		DestFile:    dest,
		Overwrite:   false,
		Concurrency: 1,
		CookieJar:   jar,
		CookieFile:  config.Conf.CookieFile,
		Headers: map[string]interface{}{
			"user-agent": config.Conf.UserAgent,
			"Referer":    referer,
			"Origin":     referer,
			"site":       "npssd",
			"sign":       token,
		},
	})
	return "", err
}

func (r *Ncpssd) getVolumes(sUrl string, jar *cookiejar.Jar) (volumes []string, err error) {
	if strings.Contains(sUrl, "fullTextRead?filePath=") {
		dUrl := r.getPdfUrl(sUrl)
		r.dt.BookId = r.getBookId(dUrl)
		volumes = append(volumes, dUrl)
	} else {
		r.dt.BookId = r.getBookId(sUrl)
		name := fmt.Sprintf("%04d", r.dt.Index)
		log.Printf("Get %s  %s\n", name, sUrl)
		dUrl, err := r.getReadUrl(r.dt.BookId)
		if err != nil {
			return nil, err
		}
		volumes = append(volumes, dUrl)
	}
	return volumes, err
}

func (r *Ncpssd) getCanvases(sUrl string, jar *cookiejar.Jar) (canvases []string, err error) {
	//TODO implement me
	panic("implement me")
}

func (r *Ncpssd) getBody(sUrl string, jar *cookiejar.Jar) ([]byte, error) {
	referer := url.QueryEscape(r.dt.Url)
	ctx := context.Background()
	cli := gohttp.NewClient(ctx, gohttp.Options{
		CookieFile: config.Conf.CookieFile,
		CookieJar:  jar,
		Headers: map[string]interface{}{
			"User-Agent":       config.Conf.UserAgent,
			"Referer":          referer,
			"X-Requested-With": "XMLHttpRequest",
			"Content-Type":     "application/json; charset=utf-8",
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

func (r *Ncpssd) postBody(sUrl string, d []byte) ([]byte, error) {
	ctx := context.Background()
	cli := gohttp.NewClient(ctx, gohttp.Options{
		CookieFile: config.Conf.CookieFile,
		CookieJar:  r.dt.Jar,
		Headers: map[string]interface{}{
			"User-Agent":   config.Conf.UserAgent,
			"Content-Type": "application/json; charset=utf-8",
		},
		Body: d,
	})
	resp, err := cli.Post(sUrl)
	if err != nil {
		return nil, err
	}
	bs, _ := resp.GetBody()
	return bs, err
}

func (r *Ncpssd) getReadUrl(bookId string) (string, error) {
	apiUrl := fmt.Sprintf("https://%s/Literature/readurl?id=%s&type=3", r.dt.UrlParsed.Host, bookId)
	bs, err := r.getBody(apiUrl, r.dt.Jar)
	if err != nil {
		return "", err
	}
	type ResponseReadUrl struct {
		Url string `json:"url"`
	}
	var respReadUrl ResponseReadUrl
	if err = json.Unmarshal(bs, &respReadUrl); err != nil {
		return "", err
	}
	return respReadUrl.Url, nil
}

func (r *Ncpssd) getPdfUrl(sUrl string) string {
	var pdfUrl string
	m := regexp.MustCompile(`(?i)filePath=(.+)\.pdf`).FindStringSubmatch(sUrl)
	if m != nil {
		s, _ := url.QueryUnescape(m[1])
		pdfUrl = s + ".pdf"
	}
	return pdfUrl
}

func (r *Ncpssd) getToken() (string, error) {
	apiUrl := "https://" + r.dt.UrlParsed.Host + "/common/getMinioSign"
	bs, err := r.postBody(apiUrl, nil)
	if err != nil {
		return "", err
	}

	type MinioSign struct {
		Result bool   `json:"result"`
		Code   int    `json:"code"`
		Data   string `json:"data"`
		Succee bool   `json:"succee"`
	}
	var minioSign MinioSign
	if err = json.Unmarshal(bs, &minioSign); err != nil {
		return "", err
	}
	//h := md5.New()
	//h.Write([]byte("L!N45S26y1SGzq9^" + minioSign.Data))
	//token := fmt.Sprintf("%x", h.Sum(nil))
	return minioSign.Data, nil
}

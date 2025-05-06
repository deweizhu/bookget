package app

import (
	"bookget/config"
	"bookget/model/iiif"
	"bookget/pkg/gohttp"
	"bookget/pkg/util"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http/cookiejar"
	"net/url"
	"os"
	"regexp"
	"strings"
)

type Khirin struct {
	dt     *DownloadTask
	apiUrl string
}

func NewKhirin() *Khirin {
	return &Khirin{
		// 初始化字段
		dt: new(DownloadTask),
	}
}

func (r *Khirin) GetRouterInit(sUrl string) (map[string]interface{}, error) {
	msg, err := r.Run(sUrl)
	return map[string]interface{}{
		"type": "iiif",
		"url":  sUrl,
		"msg":  msg,
	}, err
}

func (r *Khirin) Run(sUrl string) (msg string, err error) {

	r.dt.UrlParsed, err = url.Parse(sUrl)
	r.dt.Url = sUrl

	r.dt.BookId = r.getBookId(r.dt.Url)
	if r.dt.BookId == "" {
		return "requested URL was not found.", err
	}
	r.dt.Jar, _ = cookiejar.New(nil)
	r.apiUrl = r.dt.UrlParsed.Scheme + "://" + r.dt.UrlParsed.Host
	return r.download()
}

func (r *Khirin) getBookId(sUrl string) (bookId string) {
	m := regexp.MustCompile(`ac.jp/([A-z\d_-]+)/([A-z\d_-]+)`).FindStringSubmatch(sUrl)
	if m != nil {
		bookId = fmt.Sprintf("%s.%s", m[1], m[2])
	}
	return bookId
}

func (r *Khirin) download() (msg string, err error) {
	name := fmt.Sprintf("%04d", r.dt.Index)
	log.Printf("Get %s  %s\n", name, r.dt.Url)
	r.dt.SavePath = CreateDirectory(r.dt.Url, r.dt.BookId, "")
	manifestUrl, err := r.getManifestUrl(r.dt.Url)
	if err != nil {
		return "requested URL was not found.", err
	}
	canvases, err := r.getCanvases(manifestUrl, r.dt.Jar)
	if err != nil || canvases == nil {
		return "requested URL was not found.", err
	}
	log.Printf(" %d pages \n", len(canvases))
	return r.do(canvases)
}

func (r *Khirin) do(canvases []string) (msg string, err error) {
	if config.Conf.UseDziRs {
		r.doDezoomifyRs(canvases)
	} else {
		r.doNormal(canvases)
	}
	return "", err
}

func (r *Khirin) doDezoomifyRs(canvases []string) bool {
	if canvases == nil {
		return false
	}
	referer := url.QueryEscape(r.dt.Url)
	args := []string{"--dezoomer=iiif",
		"-H", "Origin:" + referer,
		"-H", "Referer:" + referer,
		"-H", "User-Agent:" + config.Conf.UserAgent,
	}
	size := len(canvases)
	for i, uri := range canvases {
		if uri == "" || !config.PageRange(i, size) {
			continue
		}
		sortId := fmt.Sprintf("%04d", i+1)
		filename := sortId + config.Conf.FileExt
		inputUri := r.dt.SavePath + sortId + "_info.json"
		bs, err := r.getBody(uri, r.dt.Jar)
		if err != nil {
			continue
		}
		bsNew := regexp.MustCompile(`profile":\[([^{]+)\{"formats":([^\]]+)\],`).ReplaceAll(bs, []byte(`profile":[{"formats":["jpg"],`))
		os.WriteFile(inputUri, bsNew, os.ModePerm)
		dest := r.dt.SavePath + filename
		if FileExist(dest) {
			continue
		}
		log.Printf("Get %s  %s\n", sortId, uri)
		if ret := util.StartProcess(inputUri, dest, args); ret == true {
			os.Remove(inputUri)
		}
	}
	return true
}

func (r *Khirin) doNormal(canvases []string) bool {
	if canvases == nil {
		return false
	}
	fmt.Println()
	size := len(canvases)
	ctx := context.Background()
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
		log.Printf("Get %d/%d, URL: %s\n", i+1, size, uri)
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
		gohttp.FastGet(ctx, uri, opts)
		fmt.Println()
		//util.PrintSleepTime(config.Conf.Speed)
	}
	fmt.Println()
	return true
}

func (r *Khirin) getVolumes(sUrl string, jar *cookiejar.Jar) (volumes []string, err error) {
	//TODO implement me
	panic("implement me")
}

func (r *Khirin) getCanvases(sUrl string, jar *cookiejar.Jar) (canvases []string, err error) {
	bs, err := r.getBody(sUrl, jar)
	var manifest = new(iiif.ManifestResponse)
	if err != nil {
		return nil, err
	}
	if err = json.Unmarshal(bs, manifest); err != nil {
		log.Printf("json.Unmarshal failed: %s\n", err)
		return
	}
	if len(manifest.Sequences) == 0 {
		return
	}
	size := len(manifest.Sequences[0].Canvases)
	canvases = make([]string, 0, size)
	for _, canvase := range manifest.Sequences[0].Canvases {
		for _, image := range canvase.Images {
			if config.Conf.UseDziRs {
				//dezoomify-rs URL
				iiiInfo := fmt.Sprintf("%s/info.json", image.Resource.Service.Id)
				canvases = append(canvases, iiiInfo)
			} else {
				imgUrl := image.Resource.Service.Id + "/" + config.Conf.Format
				canvases = append(canvases, imgUrl)
			}
		}
	}
	return canvases, nil
}
func (r *Khirin) getManifestUrl(sUrl string) (uri string, err error) {
	bs, err := r.getBody(sUrl, r.dt.Jar)
	if err != nil {
		return
	}
	text := string(bs)
	//<iframe id="uv-iframe" class="uv-iframe" src="/libraries/uv/uv.html#?manifest=/iiif/rekihaku/H-173-1/manifest.json"></iframe>
	m := regexp.MustCompile(`manifest=(.+?)["']`).FindStringSubmatch(text)
	if m == nil {
		return
	}
	if !strings.HasPrefix(m[1], "https://") {
		uri = r.apiUrl + "/" + m[1]
	} else {
		uri = m[1]
	}
	return
}

func (r *Khirin) getBody(apiUrl string, jar *cookiejar.Jar) ([]byte, error) {
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

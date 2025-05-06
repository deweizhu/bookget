package app

import (
	"bookget/config"
	"bookget/model/cuhk"
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

type Cuhk struct {
	dt *DownloadTask
}

func NewCuhk() *Cuhk {
	return &Cuhk{
		// 初始化字段
		dt: new(DownloadTask),
	}
}

func (r *Cuhk) GetRouterInit(sUrl string) (map[string]interface{}, error) {
	msg, err := r.Run(sUrl)
	return map[string]interface{}{
		"url": sUrl,
		"msg": msg,
	}, err
}

func (r *Cuhk) Run(sUrl string) (msg string, err error) {
	r.dt.UrlParsed, err = url.Parse(sUrl)
	r.dt.Url = sUrl
	r.dt.BookId = getBookId(r.dt.Url)
	if r.dt.BookId == "" {
		return "requested URL was not found.", err
	}
	r.dt.Jar, _ = cookiejar.New(nil)
	//OpenWebBrowser(sUrl, []string{})
	WaitNewCookie()
	return r.download()
}

func (r *Cuhk) download() (msg string, err error) {
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

func (r *Cuhk) do(imgUrls []string) (msg string, err error) {
	if imgUrls == nil {
		return
	}
	if config.Conf.UseDziRs {
		r.doDezoomifyRs(imgUrls)
	} else {
		r.doNormal(imgUrls)
	}
	return "", err
}

func (r *Cuhk) doDezoomifyRs(iiifUrls []string) bool {
	if iiifUrls == nil {
		return false
	}
	referer := url.QueryEscape(r.dt.Url)
	size := len(iiifUrls)
	for i, uri := range iiifUrls {
		if uri == "" || !config.PageRange(i, size) {
			continue
		}
		sortId := fmt.Sprintf("%04d", i+1)
		filename := sortId + config.Conf.FileExt
		dest := r.dt.SavePath + filename
		if FileExist(dest) {
			continue
		}
		log.Printf("Get %d/%d  %s\n", i+1, size, uri)
		cookies := gohttp.ReadCookieFile(config.Conf.CookieFile)
		args := []string{"--dezoomer=deepzoom",
			"-H", "Origin:" + referer,
			"-H", "Referer:" + referer,
			"-H", "User-Agent:" + config.Conf.UserAgent,
			"-H", "cookie:" + cookies,
		}
		util.StartProcess(uri, dest, args)
	}
	return true
}

func (r *Cuhk) doNormal(imgUrls []string) {
	fmt.Println()
	referer := r.dt.Url
	size := len(imgUrls)
	ctx := context.Background()
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
		log.Printf("Get %d/%d,  URL: %s\n", i+1, size, uri)
		opts := gohttp.Options{
			DestFile:    dest,
			Overwrite:   false,
			Concurrency: 1,
			CookieFile:  config.Conf.CookieFile,
			CookieJar:   r.dt.Jar,
			Headers: map[string]interface{}{
				"User-Agent": config.Conf.UserAgent,
				"Referer":    referer,
				//"X-ISLANDORA-TOKEN": v.Token,
			},
		}
		for k := 0; k < 10; k++ {
			resp, err := gohttp.FastGet(ctx, uri, opts)
			if err == nil && resp.GetStatusCode() == 200 {
				break
			}
			WaitNewCookieWithMsg(uri)
		}
		util.PrintSleepTime(config.Conf.Speed)
		fmt.Println()
	}
	fmt.Println()

}

func (r *Cuhk) getVolumes(sUrl string, jar *cookiejar.Jar) (volumes []string, err error) {
	bs, err := r.getBodyWithLoop(sUrl, jar)
	subText := util.SubText(string(bs), "id=\"block-islandora-compound-object-compound-navigation-select-list\"", "id=\"book-viewer\">")
	matches := regexp.MustCompile(`value=['"]([A-z\d:_-]+)['"]`).FindAllStringSubmatch(subText, -1)
	if matches == nil {
		volumes = append(volumes, sUrl)
		return
	}
	volumes = make([]string, 0, len(matches))
	for _, m := range matches {
		//value='ignore'
		if m[1] == "ignore" {
			continue
		}
		id := strings.Replace(m[1], ":", "-", 1)
		volumes = append(volumes, fmt.Sprintf("https://repository.lib.cuhk.edu.hk/sc/item/%s#page/1/mode/2up", id))
	}
	return volumes, nil
}

func (r *Cuhk) getCanvases(sUrl string, jar *cookiejar.Jar) (canvases []string, err error) {
	bs, err := r.getBodyWithLoop(sUrl, jar)
	var resp cuhk.ResponsePage
	matches := regexp.MustCompile(`"pages":([^]]+)]`).FindSubmatch(bs)
	if matches == nil {
		return nil, err
	}
	data := []byte("{\"pages\":" + string(matches[1]) + "]}")
	if err = json.Unmarshal(data, &resp); err != nil {
		log.Printf("json.Unmarshal failed: %s\n", err)
	}
	for _, page := range resp.ImagePage {
		var imgUrl string
		if config.Conf.UseDziRs {
			//dezoomify-rs URL
			imgUrl = fmt.Sprintf("https://%s/iiif/2/%s/info.json", r.dt.UrlParsed.Host, page.Identifier)
		} else {
			if config.Conf.FileExt == ".jpg" {
				imgUrl = fmt.Sprintf("https://%s/iiif/2/%s/%s", r.dt.UrlParsed.Host, page.Identifier, config.Conf.Format)
			} else {
				imgUrl = fmt.Sprintf("https://%s/islandora/object/%s/datastream/JP2", r.dt.UrlParsed.Host, page.Pid)
			}
		}
		canvases = append(canvases, imgUrl)
	}
	return canvases, err
}

func (r *Cuhk) getBodyWithLoop(sUrl string, jar *cookiejar.Jar) (bs []byte, err error) {
	for i := 0; i < 1000; i++ {
		bs, err = r.getBody(sUrl, jar)
		if err != nil {
			WaitNewCookie()
			continue
		}
		break
	}
	return bs, nil
}

func (r *Cuhk) getBody(apiUrl string, jar *cookiejar.Jar) ([]byte, error) {
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
	if resp.GetStatusCode() == 202 || bs == nil {
		return nil, errors.New(fmt.Sprintf("ErrCode:%d, %s", resp.GetStatusCode(), resp.GetReasonPhrase()))
	}
	return bs, nil
}

func (r *Cuhk) getCanvasesJPEG2000(sUrl string, jar *cookiejar.Jar) (imagePage []cuhk.ImagePage) {
	bs, err := r.getBodyWithLoop(sUrl, jar)
	var resp cuhk.ResponsePage
	matches := regexp.MustCompile(`"pages":([^]]+)]`).FindSubmatch(bs)
	if matches != nil {
		data := []byte("{\"pages\":" + string(matches[1]) + "]}")
		if err = json.Unmarshal(data, &resp); err != nil {
			log.Printf("json.Unmarshal failed: %s\n", err)
		}
		imagePage = make([]cuhk.ImagePage, len(resp.ImagePage))
		copy(imagePage, resp.ImagePage)
	}
	return imagePage
}

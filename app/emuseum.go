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
	"regexp"
	"strings"
	"sync"
)

type Emuseum struct {
	dt *DownloadTask
}

func NewEmuseum() *Emuseum {
	return &Emuseum{
		// 初始化字段
		dt: new(DownloadTask),
	}
}

func (d *Emuseum) GetRouterInit(sUrl string) (map[string]interface{}, error) {
	msg, err := d.Run(sUrl)
	return map[string]interface{}{
		"url": sUrl,
		"msg": msg,
	}, err
}

func (d *Emuseum) Run(sUrl string) (msg string, err error) {
	d.dt.UrlParsed, err = url.Parse(sUrl)
	d.dt.Url = sUrl
	d.dt.BookId = d.getBookId(d.dt.Url)
	if d.dt.BookId == "" {
		return "requested URL was not found.", err
	}
	d.dt.Jar, _ = cookiejar.New(nil)
	return d.download()
}

func (d *Emuseum) getBookId(sUrl string) (bookId string) {
	m := regexp.MustCompile(`content_base_id=([A-z0-9]+)&content_part_id=([A-z0-9]+)`).FindStringSubmatch(sUrl)
	if m != nil {
		if len(m[2]) < 3 {
			m[2] = "00" + m[2]
		}
		bookId = fmt.Sprintf("%s%s", m[1], m[2])
	}
	return bookId
}

func (d *Emuseum) download() (msg string, err error) {
	name := fmt.Sprintf("%04d", d.dt.Index)
	log.Printf("Get %s  %s\n", name, d.dt.Url)

	respVolume, err := d.getVolumes(d.dt.Url, d.dt.Jar)
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
			d.dt.SavePath = CreateDirectory(d.dt.UrlParsed.Host, d.dt.BookId, "")
		} else {
			vid := fmt.Sprintf("%04d", i+1)
			d.dt.SavePath = CreateDirectory(d.dt.UrlParsed.Host, d.dt.BookId, vid)
		}

		canvases, err := d.getCanvases(vol, d.dt.Jar)
		if err != nil || canvases == nil {
			fmt.Println(err)
			continue
		}
		log.Printf(" %d/%d volume, %d pages \n", i+1, sizeVol, len(canvases))
		d.do(canvases)
	}
	return "", nil
}

func (d *Emuseum) do(imgUrls []string) (msg string, err error) {
	if config.Conf.UseDziRs {
		d.doDezoomifyRs(imgUrls)
	} else {
		d.doNormal(imgUrls)
	}
	return "", err
}

func (d *Emuseum) getVolumes(sUrl string, jar *cookiejar.Jar) (volumes []string, err error) {
	bs, err := d.getBody(sUrl, jar)
	if err != nil {
		return
	}
	match := regexp.MustCompile(`https://(.+)/manifest.json`).FindSubmatch(bs)
	if match == nil {
		return
	}
	jsonUrl := "https://" + string(match[1]) + "/manifest.json"
	volumes = append(volumes, jsonUrl)
	return volumes, nil
}

func (d *Emuseum) getCanvases(sUrl string, jar *cookiejar.Jar) (canvases []string, err error) {
	bs, err := d.getBody(sUrl, jar)
	if err != nil {
		return
	}
	var manifest = new(iiif.ManifestResponse)
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
			if strings.Contains(image.Resource.Service.Id, "/100001001002.tif") {
				image.Resource.Service.Id = strings.Replace(image.Resource.Service.Id, "/100001001002.tif", "/100001001001.tif", 1)
				image.Resource.Id = strings.Replace(image.Resource.Id, "/100001001002.tif", "/100001001001.tif", 1)
			}
			if config.Conf.UseDziRs {
				//dezoomify-rs URL
				iiiInfo := fmt.Sprintf("%s/info.json", image.Resource.Service.Id)
				canvases = append(canvases, iiiInfo)
			} else {
				//JPEG URL
				imgUrl := image.Resource.Service.Id + "/" + config.Conf.Format
				canvases = append(canvases, imgUrl)
			}
		}
	}
	return canvases, nil

}

func (d *Emuseum) getBody(sUrl string, jar *cookiejar.Jar) ([]byte, error) {
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

func (d *Emuseum) doDezoomifyRs(iiifUrls []string) bool {
	if iiifUrls == nil {
		return false
	}
	referer := url.QueryEscape(d.dt.Url)
	args := []string{
		"-H", "Origin:" + referer,
		"-H", "Referer:" + referer,
		"-H", "User-Agent:" + config.Conf.UserAgent,
	}
	size := len(iiifUrls)
	for i, uri := range iiifUrls {
		if uri == "" || !config.PageRange(i, size) {
			continue
		}
		sortId := fmt.Sprintf("%04d", i+1)
		filename := sortId + config.Conf.FileExt
		dest := d.dt.SavePath + filename
		if FileExist(dest) {
			continue
		}
		log.Printf("Get %d/%d  %s\n", i+1, size, uri)
		util.StartProcess(uri, dest, args)
	}
	return true
}

func (d *Emuseum) doNormal(imgUrls []string) bool {
	if imgUrls == nil {
		return false
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
		dest := d.dt.SavePath + filename
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
				CookieJar:   d.dt.Jar,
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
	return true
}

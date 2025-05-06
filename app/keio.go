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

type Keio struct {
	dt *DownloadTask
}

func NewKeio() *Keio {
	return &Keio{
		// 初始化字段
		dt: new(DownloadTask),
	}
}

func (r *Keio) GetRouterInit(sUrl string) (map[string]interface{}, error) {
	msg, err := r.Run(sUrl)
	return map[string]interface{}{
		"type": "iiif",
		"url":  sUrl,
		"msg":  msg,
	}, err
}

func (r *Keio) Run(sUrl string) (msg string, err error) {

	r.dt.UrlParsed, err = url.Parse(sUrl)
	r.dt.Url = sUrl

	r.dt.BookId, r.dt.VolumeId = r.getBookId(r.dt.Url)
	if r.dt.BookId == "" {
		return "requested URL was not found.", err
	}
	r.dt.Jar, _ = cookiejar.New(nil)
	return r.download()
}

func (r *Keio) getBookId(sUrl string) (bookId, volumeId string) {
	m := regexp.MustCompile(`bib_frame\?id=([A-z0-9_-]+)`).FindStringSubmatch(sUrl)
	if m != nil {
		m1 := strings.Split(m[1], "-")
		bookId = m1[0]
		if len(m1) == 2 {
			volumeId = m1[1]
		}
	}
	return bookId, volumeId
}

func (r *Keio) getManifestUrl(sUrl string) (uri string, err error) {
	//https://db2.sido.keio.ac.jp/kanseki/bib_image?id=
	apiUrl := "https://db2.sido.keio.ac.jp/kanseki/bib_image?id=" + r.dt.BookId
	bs, err := r.getBody(apiUrl, r.dt.Jar)
	if err != nil {
		return
	}
	text := string(bs)
	//"manifestUri": "https://db2.sido.keio.ac.jp/iiif/manifests/kanseki/007387/007387-001/manifest.json",
	m := regexp.MustCompile(`https://([^"]+)manifest.json`).FindStringSubmatch(text)
	if m == nil {
		return
	}
	uri = "https://" + m[1] + "manifest.json"
	return
}

func (r *Keio) download() (msg string, err error) {
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

func (r *Keio) do(imgUrls []string) (msg string, err error) {
	if config.Conf.UseDziRs {
		r.doDezoomifyRs(imgUrls)
	} else {
		r.doNormal(imgUrls)
	}
	return "", err
}

func (r *Keio) getVolumes(sUrl string, jar *cookiejar.Jar) (volumes []string, err error) {
	bs, err := r.getBody(sUrl, r.dt.Jar)
	matches := regexp.MustCompile(`data-folid=['|"]([A-z0-9]+)['|"]`).FindAllSubmatch(bs, -1)
	if matches == nil {
		return
	}
	var isFolid4Digit bool
	m := regexp.MustCompile(`id="isFolid4Digit"\s+value=['|"]([A-z0-9]+)['|"]`).FindSubmatch(bs)
	if m != nil {
		isFolid4Digit = string(m[1]) == "1"
	}
	//<input type="hidden" id="isFolid4Digit" value="0"><input type="hidden" id="bibid" value="006845">
	for _, v := range matches {
		childId := r.makeId(string(v[1]), r.dt.BookId, isFolid4Digit)
		uri := fmt.Sprintf("https://db2.sido.keio.ac.jp/iiif/manifests/kanseki/%s/%s/manifest.json", r.dt.BookId, childId)
		volumes = append(volumes, uri)
	}
	return volumes, nil
}

func (r *Keio) getCanvases(sUrl string, jar *cookiejar.Jar) (canvases []string, err error) {
	bs, err := r.getBody(sUrl, jar)
	if err != nil {
		return nil, err
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

func (r *Keio) getBody(apiUrl string, jar *cookiejar.Jar) ([]byte, error) {
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

func (r *Keio) makeId(childId string, bookId string, isFolid4Digit bool) string {
	childIDfmt := ""
	iLen := 3
	if isFolid4Digit {
		iLen = 4
	}
	for k := iLen - len(childId); k > 0; k-- {
		childIDfmt += "0"
	}
	childIDfmt += childId
	return bookId + "-" + childIDfmt
}

func (r *Keio) doNormal(imgUrls []string) bool {
	if imgUrls == nil {
		return false
	}
	fmt.Println()
	size := len(imgUrls)
	var wg sync.WaitGroup
	q := QueueNew(int(config.Conf.Threads))
	for i, dUrl := range imgUrls {
		if dUrl == "" || !config.PageRange(i, size) {
			continue
		}
		ext := util.FileExt(dUrl)
		sortId := fmt.Sprintf("%04d", i+1)
		filename := sortId + ext
		dest := r.dt.SavePath + filename
		if FileExist(dest) {
			continue
		}
		imgUrl := dUrl
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
		})
	}
	wg.Wait()
	return true
}

func (r *Keio) doDezoomifyRs(iiifUrls []string) bool {
	if iiifUrls == nil {
		return false
	}
	referer := url.QueryEscape(r.dt.Url)
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
		dest := r.dt.SavePath + filename
		if FileExist(dest) {
			continue
		}
		log.Printf("Get %d/%d  %s\n", i+1, size, uri)
		util.StartProcess(uri, dest, args)
	}
	return true
}

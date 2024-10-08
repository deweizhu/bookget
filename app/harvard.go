package app

import (
	"bookget/config"
	"bookget/lib/gohttp"
	"bookget/lib/util"
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

type Harvard struct {
	dt *DownloadTask
}

func (p *Harvard) Init(iTask int, sUrl string) (msg string, err error) {
	if strings.Contains(sUrl, "curiosity.lib.harvard.edu") {
		bs, err := p.getBodyLoop(sUrl, nil)
		if err != nil {
			return "", err
		}
		m := regexp.MustCompile(`manifestId=([^“]+?)"`).FindSubmatch(bs)
		if m != nil {
			sUrl = string(m[1])
		}
	}

	p.dt = new(DownloadTask)
	p.dt.UrlParsed, err = url.Parse(sUrl)
	p.dt.Url = sUrl
	p.dt.Index = iTask
	p.dt.BookId = p.getBookId(p.dt.Url)
	if p.dt.BookId == "" {
		return "requested URL was not found.", err
	}
	p.dt.Jar, _ = cookiejar.New(nil)
	WaitNewCookie()
	return p.download()
}

func (p *Harvard) getBookId(sUrl string) (bookId string) {
	m := regexp.MustCompile(`manifests/view/([A-z0-9-_:]+)`).FindStringSubmatch(sUrl)
	if m != nil {
		return m[1]
	}
	m = regexp.MustCompile(`/manifests/([A-z0-9-_:]+)`).FindStringSubmatch(sUrl)
	if m != nil {
		return m[1]
	}
	//https://listview.lib.harvard.edu/lists/drs-54194370
	m = regexp.MustCompile(`/lists/([A-z0-9-_:]+)`).FindStringSubmatch(sUrl)
	if m != nil {
		return m[1]
	}
	return ""
}

func (p *Harvard) download() (msg string, err error) {
	name := util.GenNumberSorted(p.dt.Index)
	log.Printf("Get %s  %s\n", name, p.dt.Url)

	respVolume, err := p.getVolumes(p.dt.Url, p.dt.Jar)
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
			p.dt.SavePath = CreateDirectory(p.dt.UrlParsed.Host, p.dt.BookId, "")
		} else {
			vid := util.GenNumberSorted(i + 1)
			p.dt.SavePath = CreateDirectory(p.dt.UrlParsed.Host, p.dt.BookId, vid)
		}

		canvases, err := p.getCanvases(vol, p.dt.Jar)
		if err != nil || canvases == nil {
			fmt.Println(err)
			continue
		}
		log.Printf(" %d/%d volume, %d pages \n", i+1, sizeVol, len(canvases))
		p.do(canvases)
	}
	return "", nil
}

func (p *Harvard) do(imgUrls []string) (msg string, err error) {
	if config.Conf.UseDziRs {
		p.doDezoomifyRs(imgUrls)
	} else {
		p.doNormal(imgUrls)
	}
	return "", err
}

func (p *Harvard) getVolumes(sUrl string, jar *cookiejar.Jar) (volumes []string, err error) {
	if strings.Contains(sUrl, "listview.lib.harvard.edu") {
		bs, err := p.getBodyLoop(sUrl, nil)
		if err != nil {
			return nil, err
		}
		matches := regexp.MustCompile(`target="_blank" href="https://nrs.harvard.edu([^"]+)"`).FindAllSubmatch(bs, -1)
		if matches == nil {
			return nil, err
		}
		for _, m := range matches {
			volUrl := "https://nrs.harvard.edu" + strings.Replace(string(m[1]), "//", "/", -1)
			volumes = append(volumes, volUrl)
		}
	} else if strings.Contains(sUrl, "iiif.lib.harvard.edu") {
		volumes = append(volumes, sUrl)
	}
	return volumes, nil
}

func (p *Harvard) getCanvases(sUrl string, jar *cookiejar.Jar) (canvases []string, err error) {
	var manifestUri = sUrl
	if strings.Contains(sUrl, "iiif.lib.harvard.edu/manifests/view/") ||
		strings.Contains(sUrl, "nrs.harvard.edu") {
		bs, err := p.getBodyLoop(sUrl, jar)
		if err != nil {
			return nil, err
		}
		//"manifestUri": "https://iiif.lib.harvard.edu/manifests/drs:428501920"
		match := regexp.MustCompile(`"manifestUri":[\s+]"([^"]+?)"`).FindSubmatch(bs)
		if match != nil {
			manifestUri = string(match[1])
		} else {
			return nil, errors.New("requested URL was not found.")
		}
	}
	bs, err := p.getBodyLoop(manifestUri, jar)
	if err != nil {
		return
	}
	var manifest = new(ResponseManifest)
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

func (p *Harvard) getBody(sUrl string, jar *cookiejar.Jar) ([]byte, error) {
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

func (p *Harvard) getBodyLoop(sUrl string, jar *cookiejar.Jar) (bs []byte, err error) {
	for i := 0; i < 1000; i++ {
		bs, err = p.getBody(sUrl, jar)
		if err != nil {
			WaitNewCookie()
			continue
		}
		break
	}
	return bs, nil
}

func (p *Harvard) postBody(sUrl string, d []byte) ([]byte, error) {
	//TODO implement me
	panic("implement me")
}

func (p *Harvard) doDezoomifyRs(iiifUrls []string) bool {
	if iiifUrls == nil {
		return false
	}
	referer := url.QueryEscape(p.dt.Url)
	size := len(iiifUrls)
	for i, uri := range iiifUrls {
		if uri == "" || !config.PageRange(i, size) {
			continue
		}
		sortId := util.GenNumberSorted(i + 1)
		filename := sortId + config.Conf.FileExt
		dest := p.dt.SavePath + filename
		if FileExist(dest) {
			continue
		}
		log.Printf("Get %d/%d  %s\n", i+1, size, uri)
		cookies := gohttp.ReadCookieFile(config.Conf.CookieFile)
		args := []string{
			"-H", "Origin:" + referer,
			"-H", "Referer:" + referer,
			"-H", "User-Agent:" + config.Conf.UserAgent,
			"-H", "cookie:" + cookies,
		}
		util.StartProcess(uri, dest, args)
	}
	return true
}

func (p *Harvard) doNormal(imgUrls []string) bool {
	if imgUrls == nil {
		return false
	}
	size := len(imgUrls)
	fmt.Println()
	ctx := context.Background()
	for i, uri := range imgUrls {
		if uri == "" || !config.PageRange(i, size) {
			continue
		}
		ext := util.FileExt(uri)
		sortId := util.GenNumberSorted(i + 1)
		filename := sortId + ext
		dest := p.dt.SavePath + filename
		if FileExist(dest) {
			continue
		}
		fmt.Println()
		log.Printf("Get %d/%d  %s\n", i+1, size, uri)
		opts := gohttp.Options{
			DestFile:    dest,
			Overwrite:   false,
			Concurrency: 1,
			CookieFile:  config.Conf.CookieFile,
			CookieJar:   p.dt.Jar,
			Headers: map[string]interface{}{
				"User-Agent": config.Conf.UserAgent,
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
	return true
}

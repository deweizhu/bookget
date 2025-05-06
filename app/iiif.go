package app

import (
	"bookget/config"
	"bookget/model/iiif"
	"bookget/pkg/downloader"
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

type IIIF struct {
	dt         *DownloadTask
	xmlContent []byte
	ctx        context.Context
	bookId     string
}

func NewIiifRouter() *IIIF {
	return &IIIF{
		// 初始化字段
		dt:  new(DownloadTask),
		ctx: context.Background(),
	}
}

func (i *IIIF) GetRouterInit(sUrl string) (map[string]interface{}, error) {
	msg, err := i.Run(sUrl)
	return map[string]interface{}{
		"type": "iiif",
		"url":  sUrl,
		"msg":  msg,
	}, err
}

func (i *IIIF) Run(sUrl string) (msg string, err error) {
	i.dt.UrlParsed, err = url.Parse(sUrl)
	i.dt.Url = sUrl
	i.dt.Jar, _ = cookiejar.New(nil)
	i.dt.BookId = i.getBookId(i.dt.Url)
	if i.dt.BookId == "" {
		return "requested URL was not found.", err
	}
	return i.download()
}

func (i *IIIF) InitWithId(iTask int, sUrl string, id string) (msg string, err error) {
	i.dt = new(DownloadTask)
	i.dt.UrlParsed, err = url.Parse(sUrl)
	i.dt.Url = sUrl
	i.dt.Index = iTask
	i.dt.Jar, _ = cookiejar.New(nil)
	i.dt.BookId = id
	return i.download()
}

func (i *IIIF) getBookId(sUrl string) (bookId string) {
	m := regexp.MustCompile(`/([^/]+)/manifest.json`).FindStringSubmatch(sUrl)
	if m != nil {
		bookId = m[1]
		return
	}
	return getBookId(sUrl)
}

func (i *IIIF) download() (msg string, err error) {
	i.xmlContent, err = i.getBody(i.dt.Url, i.dt.Jar)
	if err != nil || i.xmlContent == nil {
		return "requested URL was not found.", err
	}

	ver, err := i.checkVersion(i.xmlContent)
	var canvases = make([]string, 0, 1000)
	if ver == 3 {
		//https://catalog.lib.kyushu-u.ac.jp/image/manifest/1/820/1446033.json
		canvases, err = i.getCanvasesV3(i.dt.Url, i.dt.Jar)
	} else {
		//https://dcollections.lib.keio.ac.jp/sites/default/files/iiif/KAN/110X-24-1/manifest.json
		canvases, err = i.getCanvases(i.dt.Url, i.dt.Jar)
	}
	if err != nil || canvases == nil {
		return
	}
	i.dt.SavePath = CreateDirectory(i.dt.UrlParsed.Host, i.dt.BookId, "")
	return i.do(canvases)
}

func (i *IIIF) do(imgUrls []string) (msg string, err error) {
	if config.Conf.UseDzi {
		i.doDezoomifyRs(imgUrls)
	} else {
		i.doNormal(imgUrls)
	}
	return "", nil
}

func (i *IIIF) getCanvases(sUrl string, jar *cookiejar.Jar) (canvases []string, err error) {
	var manifest = new(iiif.ManifestResponse)
	if err = json.Unmarshal(i.xmlContent, manifest); err != nil {
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
			if config.Conf.UseDzi {
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

func (i *IIIF) getCanvasesV3(sUrl string, jar *cookiejar.Jar) (canvases []string, err error) {
	var manifest = new(iiif.ManifestV3Response)
	if err = json.Unmarshal(i.xmlContent, manifest); err != nil {
		log.Printf("json.Unmarshal failed: %s\n", err)
		return
	}
	if len(manifest.Canvases) == 0 {
		return
	}
	size := len(manifest.Canvases)
	canvases = make([]string, 0, size)
	//config.Conf.Format = strings.ReplaceAll(config.Conf.Format, "full/full", "full/max")
	for _, canvase := range manifest.Canvases {
		image := canvase.Items[0].Items[0]
		id := image.Body.Service[0].Id
		if id == "" && image.Body.Service[0].Id_ != "" {
			id = image.Body.Service[0].Id_
		}
		if config.Conf.UseDzi {
			//dezoomify-rs URL
			iiiInfo := fmt.Sprintf("%s/info.json", id)
			canvases = append(canvases, iiiInfo)
		} else {
			//JPEG URL
			imgUrl := id + "/" + config.Conf.Format
			canvases = append(canvases, imgUrl)
		}
	}
	return canvases, nil
}

func (i *IIIF) getBody(sUrl string, jar *cookiejar.Jar) ([]byte, error) {
	ctx := context.Background()
	cli := gohttp.NewClient(ctx, gohttp.Options{
		CookieFile: config.Conf.CookieFile,
		CookieJar:  jar,
		Headers: map[string]interface{}{
			"User-Agent": config.Conf.UserAgent,
		},
	})
	resp, err := cli.Get(sUrl)
	if err != nil {
		return nil, err
	}
	bs, _ := resp.GetBody()
	if bs == nil {
		err = errors.New(resp.GetReasonPhrase())
		return nil, err
	}
	//fix bug https://www.dh-jac.net/db1/books/results-iiif.php?f1==nar-h13-01-01&f12=1&enter=portal
	//delete '?'
	if bs[0] != 123 {
		for i := 0; i < len(bs); i++ {
			if bs[i] == 123 {
				bs = bs[i:]
				break
			}
		}
	}
	return bs, nil
}

func (i *IIIF) doDezoomifyRs(iiifUrls []string) bool {
	if iiifUrls == nil {
		return false
	}
	referer := url.QueryEscape(i.dt.Url)
	args := []string{
		"-H", "Origin:" + referer,
		"-H", "Referer:" + referer,
		"-H", "User-Agent:" + config.Conf.UserAgent,
	}
	size := len(iiifUrls)
	for k, uri := range iiifUrls {
		if uri == "" || !config.PageRange(k, size) {
			continue
		}
		sortId := fmt.Sprintf("%04d", k+1)

		filename := sortId + config.Conf.FileExt
		dest := i.dt.SavePath + filename
		if FileExist(dest) {
			continue
		}
		log.Printf("Get %d/%d  %s\n", k+1, size, uri)
		downloader.DezoomifyGo(i.ctx, uri, dest, args)
	}
	return true
}

func (i *IIIF) doNormal(imgUrls []string) bool {
	if imgUrls == nil {
		return false
	}
	size := len(imgUrls)
	fmt.Println()
	ctx := context.Background()
	for k, uri := range imgUrls {
		if uri == "" || !config.PageRange(k, size) {
			continue
		}
		ext := util.FileExt(uri)
		sortId := fmt.Sprintf("%04d", k+1)
		filename := sortId + ext
		dest := i.dt.SavePath + filename
		if FileExist(dest) {
			continue
		}
		log.Printf("Get %d/%d  %s\n", k+1, size, uri)
		opts := gohttp.Options{
			DestFile:    dest,
			Overwrite:   false,
			Concurrency: 1,
			CookieFile:  config.Conf.CookieFile,
			CookieJar:   i.dt.Jar,
			Headers: map[string]interface{}{
				"User-Agent": config.Conf.UserAgent,
			},
		}
		_, err := gohttp.FastGet(ctx, uri, opts)
		if err != nil {
			fmt.Println(err)
		}
		fmt.Println()
	}
	return true
}

func (i *IIIF) checkVersion(bs []byte) (int, error) {
	var presentation iiif.ManifestPresentation
	if err := json.Unmarshal(bs, &presentation); err != nil {
		return 0, err
	}
	if strings.Contains(presentation.Context, "presentation/3/") {
		return 3, nil
	}
	return 2, nil
}

//func (i *IIIF) getManifestUrl(pageUrl, text string) string {
//	//最后是，相对URI
//	u, err := url.Parse(pageUrl)
//	if err != nil {
//		return ""
//	}
//	host := fmt.Sprintf("%s://%s/", u.Scheme, u.Host)
//	//url包含manifest json
//	if strings.Contains(pageUrl, ".json") {
//		m := regexp.MustCompile(`manifest=([^&]+)`).FindStringSubmatch(pageUrl)
//		if m != nil {
//			return i.padUri(host, m[1])
//		}
//		return pageUrl
//	}
//	//网页内含manifest URL
//	m := regexp.MustCompile(`manifest=(\S+).json["']`).FindStringSubmatch(text)
//	if m != nil {
//		return i.padUri(host, m[1]+".json")
//	}
//	m = regexp.MustCompile(`manifest=(\S+)["']`).FindStringSubmatch(text)
//	if m != nil {
//		return i.padUri(host, m[1])
//	}
//	m = regexp.MustCompile(`data-uri=["'](\S+)manifest(\S+).json["']`).FindStringSubmatch(text)
//	if m != nil {
//		return m[1] + "manifest" + m[2] + ".json"
//	}
//	m = regexp.MustCompile(`href=["'](\S+)/manifest.json["']`).FindStringSubmatch(text)
//	if m == nil {
//		return ""
//	}
//	return i.padUri(host, m[1]+"/manifest.json")
//}

//func (i *IIIF) padUri(host, uri string) string {
//	//https:// 或 http:// 绝对URL
//	if strings.HasPrefix(uri, "https://") || strings.HasPrefix(uri, "http://") {
//		return uri
//	}
//	manifestUri := ""
//	if uri[0] == '/' {
//		manifestUri = uri[1:]
//	} else {
//		manifestUri = uri
//	}
//	return host + manifestUri
//}

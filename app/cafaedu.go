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
	"sort"
	"sync"
)

type CafaEduResponse struct {
	Links struct {
		Self struct {
			Href string `json:"href"`
		} `json:"self"`
	} `json:"_links"`
	Meta struct {
		Took float64 `json:"took"`
	} `json:"_meta"`
	Item struct {
		Tiles map[string]CafaEduItem `json:"tiles"`
	} `json:"item"`
}

type CafaEduItem struct {
	Context  string `json:"@context"`
	Id       string `json:"@id"`
	Height   int    `json:"height"`
	Width    int    `json:"width"`
	Profile  string `json:"profile"`
	Protocol string `json:"protocol"`
	Tiles    struct {
		ScaleFactors []int `json:"scaleFactors"`
		Width        int   `json:"width"`
	} `json:"tiles"`
	TileSize struct {
		W int `json:"w"`
		H int `json:"h"`
	} `json:"tile_size"`
}

type CafaEdu struct {
	dt        *DownloadTask
	ServerUrl string
}

func NewCafaEdu() *CafaEdu {
	return &CafaEdu{
		// 初始化字段
		dt: new(DownloadTask),
	}
}

func (r *CafaEdu) GetRouterInit(sUrl string) (map[string]interface{}, error) {
	msg, err := r.Run(sUrl)
	return map[string]interface{}{
		"url": sUrl,
		"msg": msg,
	}, err
}

func (r *CafaEdu) Run(sUrl string) (msg string, err error) {
	r.dt.UrlParsed, err = url.Parse(sUrl)
	r.dt.Url = sUrl
	r.dt.BookId = r.getBookId(r.dt.Url)
	if r.dt.BookId == "" {
		return "requested URL was not found.", err
	}
	r.dt.Jar, _ = cookiejar.New(nil)
	r.ServerUrl = "dlibgate.cafa.edu.cn"
	return r.download()
}

func (r *CafaEdu) getBookId(sUrl string) (bookId string) {
	m := regexp.MustCompile(`ebook/item/([A-z0-9]+)`).FindStringSubmatch(sUrl)
	if m != nil {
		bookId = m[1]
	}
	return bookId
}

func (r *CafaEdu) download() (msg string, err error) {
	name := fmt.Sprintf("%04d", r.dt.Index)
	log.Printf("Get %s  %s\n", name, r.dt.Url)

	respVolume, err := r.getVolumes(r.dt.Url, r.dt.Jar)
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
			r.dt.SavePath = CreateDirectory(r.dt.UrlParsed.Host, r.dt.BookId, "")
		} else {
			vid := fmt.Sprintf("%04d", i+1)
			r.dt.SavePath = CreateDirectory(r.dt.UrlParsed.Host, r.dt.BookId, vid)
		}

		canvases, err := r.getCanvases(vol, r.dt.Jar)
		if err != nil || canvases == nil {
			fmt.Println(err)
			continue
		}
		log.Printf(" %d/%d volume, %d pages \n", i+1, sizeVol, len(canvases))
		r.do(canvases)
	}
	return "", nil
}

func (r *CafaEdu) do(imgUrls []string) (msg string, err error) {
	if config.Conf.UseDziRs {
		r.doDezoomifyRs(imgUrls)
	} else {
		r.doNormal(imgUrls)
	}
	return "", err
}

func (r *CafaEdu) getVolumes(sUrl string, jar *cookiejar.Jar) (volumes []string, err error) {
	iiifId, err := r.getMediaImageId(sUrl, jar)
	if err != nil {
		return nil, err
	}
	jsonUrl := fmt.Sprintf("https://%s/api/viewer/lgiiif?url=/srv/www/limbgallery/medias/%s/&max=%d", r.ServerUrl, iiifId, 10000)
	volumes = append(volumes, jsonUrl)
	return volumes, err
}

func (r *CafaEdu) getCanvases(apiUrl string, jar *cookiejar.Jar) (canvases []string, err error) {
	bs, err := r.getBody(apiUrl, jar)
	if err != nil {
		return
	}
	var manifest = new(CafaEduResponse)
	if err = json.Unmarshal(bs, manifest); err != nil {
		log.Printf("json.Unmarshal failed: %s\n", err)
		return
	}
	canvases = make([]string, 0, len(manifest.Item.Tiles))
	for _, canvase := range manifest.Item.Tiles {
		if config.Conf.UseDziRs {
			//dezoomify-rs URL
			iiiInfo := "https://" + r.dt.UrlParsed.Host + canvase.Id + "/info.json"
			canvases = append(canvases, iiiInfo)
		} else {
			//JPEG URL
			//https://dlibgate.cafa.edu.cn/i/?IIIF=/1b/86/7e/68/1b867e68-807a-44e1-b16b-a86775dc0b16/iiif/GJ05685_000001.tif/full/full/0/default.jpg
			imgUrl := "https://" + r.ServerUrl + canvase.Id + "/" + config.Conf.Format
			canvases = append(canvases, imgUrl)
		}
	}
	sort.Sort(util.SortByStr(canvases))
	return canvases, nil
}

func (r *CafaEdu) getBody(sUrl string, jar *cookiejar.Jar) ([]byte, error) {
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

func (r *CafaEdu) postBody(sUrl string, d []byte) ([]byte, error) {
	//TODO implement me
	panic("implement me")
}

func (r *CafaEdu) getMediaImageId(sUrl string, jar *cookiejar.Jar) (iiifId string, err error) {
	bs, err := r.getBody(sUrl, jar)
	if err != nil {
		return "", err
	}
	//match := regexp.MustCompile(`src="/i/?IIIF=([^"])"`).FindSubmatch(bs)
	match := regexp.MustCompile(`IIIF=/([A-z0-9/_-]+)/iiif/`).FindSubmatch(bs)
	if match != nil {
		iiifId = string(match[1])
	}
	return iiifId, err
}

func (r *CafaEdu) doDezoomifyRs(iiifUrls []string) bool {
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

func (r *CafaEdu) doNormal(imgUrls []string) bool {
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
		dest := r.dt.SavePath + filename
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
				CookieJar:   r.dt.Jar,
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

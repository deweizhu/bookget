package app

import (
	"bookget/config"
	"bookget/model/iiif"
	"bookget/pkg/gohttp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http/cookiejar"
	"net/url"
	"regexp"
	"sync"
)

type NdlJP struct {
	dt *DownloadTask
}

func NewNdlJP() *NdlJP {
	return &NdlJP{
		// 初始化字段
		dt: new(DownloadTask),
	}
}

func (r *NdlJP) GetRouterInit(sUrl string) (map[string]interface{}, error) {
	msg, err := r.Run(sUrl)
	return map[string]interface{}{
		"url": sUrl,
		"msg": msg,
	}, err
}

func (r *NdlJP) Run(sUrl string) (msg string, err error) {
	r.dt.UrlParsed, err = url.Parse(sUrl)
	r.dt.Url = sUrl
	r.dt.BookId = r.getBookId(r.dt.Url)
	if r.dt.BookId == "" {
		return "requested URL was not found.", err
	}
	r.dt.Jar, _ = cookiejar.New(nil)
	return r.download()
}

func (r *NdlJP) getBookId(sUrl string) (bookId string) {
	if m := regexp.MustCompile(`/pid/([A-Za-z0-9]+)`).FindStringSubmatch(sUrl); m != nil {
		bookId = m[1]
	}
	return bookId
}

func (r *NdlJP) download() (msg string, err error) {
	respVolume, err := r.getVolumes(r.dt.Url, r.dt.Jar)
	if err != nil {
		fmt.Println(err)
		return "getVolumes", err
	}
	for i, vol := range respVolume {
		if !config.VolumeRange(i) {
			continue
		}
		iiifUrl, _ := r.getManifestUrl(vol)
		if iiifUrl == "" {
			continue
		}
		canvases, err := r.getCanvases(iiifUrl, r.dt.Jar)
		if err != nil || canvases == nil {
			fmt.Println(err)
			continue
		}
		vid := fmt.Sprintf("%04d", i+1)
		r.dt.SavePath = CreateDirectory(r.dt.UrlParsed.Host, r.dt.BookId, vid)

		log.Printf(" %d/%d volume, %d pages \n", i+1, len(respVolume), len(canvases))
		r.do(canvases)
	}
	return msg, err
}

func (r *NdlJP) do(imgUrls []string) (msg string, err error) {
	if imgUrls == nil {
		return "", nil
	}
	size := len(imgUrls)
	fmt.Println()
	var wg sync.WaitGroup
	q := QueueNew(int(config.Conf.Threads))
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
	return "", err
}

func (r *NdlJP) getVolumes(sUrl string, jar *cookiejar.Jar) (volumes []string, err error) {
	apiUrl := "https://" + r.dt.UrlParsed.Host + "/api/meta/search/toc/facet/" + r.dt.BookId
	bs, err := r.getBody(apiUrl, jar)
	if err != nil {
		return
	}
	type ResponseBody struct {
		Pid      string `json:"pid"`
		Id       string `json:"id"`
		Title    string `json:"title"`
		Children []struct {
			Pid     string `json:"pid"`
			Id      string `json:"id"`
			Title   string `json:"title"`
			SortKey string `json:"sortKey"`
			Parent  string `json:"parent"`
			Level   string `json:"level"`
		} `json:"children"`
	}
	var result = new(ResponseBody)
	if err = json.Unmarshal(bs, result); err != nil {
		log.Printf("json.Unmarshal failed: %s\n", err)
		return
	}
	if result.Children == nil {
		volumes = append(volumes, r.dt.BookId)
		return volumes, nil
	}
	volumes = make([]string, 0, len(result.Children))
	for _, v := range result.Children {
		volumes = append(volumes, v.Id)
	}
	return volumes, nil
}

func (r *NdlJP) getCanvases(sUrl string, jar *cookiejar.Jar) (canvases []string, err error) {
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
			//JPEG URL
			imgUrl := image.Resource.Service.Id + "/" + config.Conf.Format
			canvases = append(canvases, imgUrl)
		}
	}
	return canvases, nil
}

func (r *NdlJP) getBody(apiUrl string, jar *cookiejar.Jar) ([]byte, error) {
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
	if bs == nil {
		return nil, errors.New(fmt.Sprintf("ErrCode:%d, %s", resp.GetStatusCode(), resp.GetReasonPhrase()))
	}
	return bs, nil
}

func (r *NdlJP) getManifestUrl(id string) (iiifUrl string, err error) {
	type ResponseBody struct {
		Item struct {
			IiifManifestUrl string `json:"iiifManifestUrl"`
		} `json:"item"`
	}
	apiUrl := "https://" + r.dt.UrlParsed.Host + "/api/item/search/info:ndljp/pid/" + id
	bs, err := r.getBody(apiUrl, r.dt.Jar)
	if err != nil {
		return "", err
	}
	var result ResponseBody
	if err = json.Unmarshal(bs, &result); err != nil {
		log.Printf("json.Unmarshal failed: %s\n", err)
		return
	}
	return result.Item.IiifManifestUrl, nil
}
